package gira

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type PMReplanInput struct {
	Repo              RepoRef
	Ticket            int
	DryRun            bool
	Apply             bool
	ExpectedPlanID    string
	Override          string
	OverrideRationale string
}

type PMReplanMutation struct {
	NodeID        string `json:"node_id,omitempty"`
	Action        string `json:"action"`
	Target        string `json:"target"`
	Reason        string `json:"reason"`
	Capability    string `json:"capability"`
	Status        string `json:"status"`
	ExistingIssue int    `json:"existing_issue,omitempty"`
	CreatedIssue  int    `json:"created_issue,omitempty"`
}

type PMReplanOverride struct {
	Action    string `json:"action"`
	Rationale string `json:"rationale"`
	DurableID string `json:"durable_id"`
}

type PMReplanReport struct {
	Command              string             `json:"command"`
	SchemaVersion        string             `json:"schema_version"`
	Mode                 string             `json:"mode"`
	Repo                 string             `json:"repo"`
	Ticket               int                `json:"ticket"`
	PlanID               string             `json:"plan_id"`
	ExpectedPlanID       string             `json:"expected_plan_id,omitempty"`
	Matched              bool               `json:"matched"`
	Idempotent           bool               `json:"idempotent"`
	RecommendationDigest string             `json:"recommendation_digest"`
	Changed              bool               `json:"changed"`
	ChangeReason         string             `json:"change_reason"`
	Mutations            []PMReplanMutation `json:"mutations"`
	ResidualActions      []PMObserveAction  `json:"residual_actions"`
	Override             *PMReplanOverride  `json:"override,omitempty"`
	Observe              *PMObserveReport   `json:"observe,omitempty"`
	NextStep             string             `json:"next_step"`
}

func BuildPMReplanReport(input PMReplanInput, runner CommandRunner) (PMReplanReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	mode := "dry_run"
	if input.Apply {
		mode = "apply"
	}
	report := PMReplanReport{Command: "pm replan", SchemaVersion: PMReplanSchemaVersion, Mode: mode, Repo: input.Repo.FullName(), Ticket: input.Ticket, ExpectedPlanID: strings.TrimSpace(input.ExpectedPlanID), Matched: true, Mutations: []PMReplanMutation{}, ResidualActions: []PMObserveAction{}}
	if input.DryRun == input.Apply {
		return report, fmt.Errorf("exactly one of dry_run/apply is required")
	}
	if input.Apply && report.ExpectedPlanID == "" {
		return report, fmt.Errorf("apply requires expected plan fingerprint")
	}
	override, rationale := strings.TrimSpace(input.Override), strings.TrimSpace(input.OverrideRationale)
	if (override == "") != (rationale == "") {
		return report, fmt.Errorf("override and override rationale must be supplied together")
	}
	if override != "" && len(rationale) < 12 {
		return report, fmt.Errorf("override rationale must explain the product judgment")
	}
	if input.Apply && pmReplanReceiptPresent(input.Repo, input.Ticket, report.ExpectedPlanID, runner) {
		report.PlanID, report.Idempotent = report.ExpectedPlanID, true
		report.NextStep = fmt.Sprintf("gira pm observe --repo %s --ticket %d --json", report.Repo, report.Ticket)
		return report, nil
	}
	observe, err := BuildPMObserveReport(PMObserveInput{Repo: input.Repo, Ticket: input.Ticket}, runner)
	if err != nil {
		return report, err
	}
	report.Observe = &observe
	report.RecommendationDigest = observe.Change.CurrentDigest
	report.Changed, report.ChangeReason = observe.Change.Changed, observe.Change.Reason
	if observe.WorkGraph != nil {
		for _, action := range observe.WorkGraph.Actions {
			capability := "issue:create"
			status := "planned"
			switch action.Action {
			case "reuse":
				capability = "issue:read"
			case "defer":
				capability = "plan:write"
			case "split":
				capability = "plan:write"
			case "supersede":
				capability, status = "issue:close", "residual"
			}
			report.Mutations = append(report.Mutations, PMReplanMutation{NodeID: action.NodeID, Action: action.Action, Target: action.NodeID, Reason: action.Reason, Capability: capability, Status: status, ExistingIssue: action.ExistingIssue})
		}
	}
	for _, action := range observe.Actions {
		if action.Residual {
			report.ResidualActions = append(report.ResidualActions, action)
		}
	}
	if override != "" {
		sum := sha256.Sum256([]byte(override + "\x00" + rationale))
		report.Override = &PMReplanOverride{Action: override, Rationale: rationale, DurableID: "replan.override." + hex.EncodeToString(sum[:6])}
		if issue := pmReplanOverrideIssue(override); issue > 0 {
			if !pmReplanBlockedChild(observe, issue) {
				return report, fmt.Errorf("unblock override target must be a blocked child of Goal #%d", input.Ticket)
			}
			report.Mutations = append(report.Mutations, PMReplanMutation{Action: "unblock", Target: "#" + strconv.Itoa(issue), Reason: rationale, Capability: "issue:label", Status: "planned", ExistingIssue: issue})
		}
	}
	report.PlanID = pmReplanFingerprint(report)
	if report.ExpectedPlanID != "" && report.ExpectedPlanID != report.PlanID {
		report.Matched = false
	}
	report.NextStep = fmt.Sprintf("gira pm replan --repo %s --ticket %d --apply --expect-plan %s", report.Repo, report.Ticket, report.PlanID)
	if input.Apply {
		if !report.Matched {
			return report, fmt.Errorf("replan fingerprint changed; rerun dry-run")
		}
		if err := applyPMReplan(&report, input.Repo, runner); err != nil {
			return report, err
		}
		report.NextStep = fmt.Sprintf("gira pm observe --repo %s --ticket %d --json", report.Repo, report.Ticket)
	}
	return report, nil
}

func pmReplanFingerprint(report PMReplanReport) string {
	type mutationSignature struct{ NodeID, Action, Target, Reason, Capability string }
	mutations := make([]mutationSignature, 0, len(report.Mutations))
	for _, mutation := range report.Mutations {
		action := mutation.Action
		reason, capability := mutation.Reason, mutation.Capability
		if action == "create" || action == "reuse" {
			action, reason, capability = "ensure", "ensure equivalent child exists", "issue:ensure"
		}
		mutations = append(mutations, mutationSignature{mutation.NodeID, action, mutation.Target, reason, capability})
	}
	value := struct {
		Repo           string
		Ticket         int
		Recommendation string
		Graph          string
		Mutations      []mutationSignature
		Residual       []PMObserveAction
		Override       *PMReplanOverride
	}{Repo: report.Repo, Ticket: report.Ticket, Recommendation: report.RecommendationDigest, Mutations: mutations, Residual: report.ResidualActions, Override: report.Override}
	if report.Observe != nil && report.Observe.WorkGraph != nil {
		value.Graph = report.Observe.WorkGraph.PlanID
	}
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return "pmr-" + hex.EncodeToString(sum[:16])
}

func applyPMReplan(report *PMReplanReport, repo RepoRef, runner CommandRunner) error {
	if report.Observe != nil && report.Observe.WorkGraph != nil && !hasPMWorkGraphErrors(report.Observe.WorkGraph.Diagnostics) {
		graph := *report.Observe.WorkGraph
		graph.Actions = append([]PMWorkGraphAction(nil), graph.Actions...)
		for i := range graph.Actions {
			if graph.Actions[i].Action == "supersede" {
				graph.Actions[i].Action, graph.Actions[i].Status = "defer", "planned"
				graph.Actions[i].Reason = "authority required before closing existing work"
			}
		}
		if err := applyPMWorkGraph(&graph, repo, runner); err != nil {
			return fmt.Errorf("apply safe graph mutations: %w", err)
		}
		created := map[string]int{}
		for _, action := range graph.Actions {
			created[action.NodeID] = action.CreatedIssue
		}
		for i := range report.Mutations {
			if report.Mutations[i].Action == "supersede" {
				continue
			}
			report.Mutations[i].Status = "applied"
			report.Mutations[i].CreatedIssue = created[report.Mutations[i].NodeID]
		}
	}
	if len(report.ResidualActions) > 0 || pmReplanHasSupersede(report.Mutations) {
		created, err := createPMReplanDecisionPacket(*report, repo, runner)
		if err != nil {
			return err
		}
		for i := range report.Mutations {
			if report.Mutations[i].Action == "supersede" {
				report.Mutations[i].CreatedIssue, report.Mutations[i].Status = created, "residual"
			}
		}
	}
	if report.Override != nil {
		record, err := BuildPMRecordReport(PMRecordInput{Repo: repo, Ticket: report.Ticket, ID: report.Override.DurableID, Kind: "decision", Text: report.Override.Rationale, SourceRefs: []string{fmt.Sprintf("issue:%s#%d", repo.FullName(), report.Ticket)}, ActorKind: "human", Status: "accepted", RecordedAt: time.Now().UTC(), Apply: true}, runner)
		if err != nil {
			return fmt.Errorf("record human override: %w", err)
		}
		_ = record
		if issue := pmReplanOverrideIssue(report.Override.Action); issue > 0 {
			if _, err := runner.Run("gh", "issue", "edit", strconv.Itoa(issue), "--repo", repo.FullName(), "--remove-label", "status:blocked", "--add-label", "status:ready"); err != nil {
				return fmt.Errorf("apply explicit unblock override: %w", err)
			}
			for i := range report.Mutations {
				if report.Mutations[i].Action == "unblock" {
					report.Mutations[i].Status = "applied"
				}
			}
		}
	}
	body := renderPMReplanReceipt(*report)
	if _, err := runner.Run("gh", "issue", "comment", strconv.Itoa(report.Ticket), "--repo", repo.FullName(), "--body", body); err != nil {
		return fmt.Errorf("post replan receipt: %w", err)
	}
	return nil
}

func createPMReplanDecisionPacket(report PMReplanReport, repo RepoRef, runner CommandRunner) (int, error) {
	title := "Authorize residual PM replan decisions"
	if report.Observe != nil {
		for _, child := range report.Observe.GoalStatus.Children {
			if goalPlanTicketTitle(title) == child.Title {
				return child.Number, nil
			}
		}
	}
	node := PMWorkGraphNode{ID: "replan.residual", Title: title, Purpose: "Resolve authority-bound or irreversible work left by the active PM replan", Profile: "decision", ParentOutcome: "goal:" + strconv.Itoa(report.Ticket), Size: "small", Verification: []PMWorkGraphVerification{{Method: "decision review", Evidence: "accepted rationale and explicit disposition"}}}
	body := renderPMWorkGraphTicket(report.Ticket, repo, repo, node)
	labels := goalPlanLabels(report.Observe.GoalStatus.Goal.Labels)
	if err := preflightTicketNewLabels(repo, labels, runner); err != nil {
		return 0, err
	}
	created, err := createRepoTicket(repo, goalPlanTicketTitle(title), body, labels, report.Observe.GoalStatus.Goal.Milestone, runner)
	if err != nil {
		return 0, err
	}
	child, err := fetchGitHubIssue(repo, created.Number, runner)
	if err != nil {
		return 0, err
	}
	if err := addGitHubSubIssue(repo, report.Ticket, child.ID, false, runner); err != nil {
		return 0, err
	}
	return created.Number, nil
}

func pmReplanOverrideIssue(value string) int {
	value = strings.TrimSpace(strings.ToLower(value))
	if !strings.HasPrefix(value, "unblock:") {
		return 0
	}
	value = strings.TrimPrefix(strings.TrimPrefix(value, "unblock:"), "#")
	n, _ := strconv.Atoi(value)
	return n
}
func pmReplanHasSupersede(values []PMReplanMutation) bool {
	for _, value := range values {
		if value.Action == "supersede" {
			return true
		}
	}
	return false
}
func pmReplanBlockedChild(observe PMObserveReport, issue int) bool {
	if observe.GoalStatus == nil {
		return false
	}
	for _, child := range observe.GoalStatus.Children {
		if child.Number == issue && (child.Repo == "" || child.Repo == observe.Repo) && strings.EqualFold(child.Status, "Blocked") {
			return true
		}
	}
	return false
}
func pmReplanReceiptPresent(repo RepoRef, ticket int, plan string, runner CommandRunner) bool {
	raw, err := runner.Run("gh", "issue", "view", strconv.Itoa(ticket), "--repo", repo.FullName(), "--json", "comments")
	if err != nil {
		return false
	}
	var payload struct {
		Comments []struct {
			Body string `json:"body"`
		} `json:"comments"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return false
	}
	needle := `"plan_id": "` + plan + `"`
	for _, comment := range payload.Comments {
		if strings.Contains(comment.Body, pmReplanReceiptMarker) && strings.Contains(comment.Body, needle) {
			return true
		}
	}
	return false
}
func renderPMReplanReceipt(report PMReplanReport) string {
	receipt := struct {
		PlanID               string             `json:"plan_id"`
		RecommendationDigest string             `json:"recommendation_digest"`
		Mutations            []PMReplanMutation `json:"mutations"`
		Override             *PMReplanOverride  `json:"override,omitempty"`
	}{report.PlanID, report.RecommendationDigest, report.Mutations, report.Override}
	encoded, _ := json.MarshalIndent(receipt, "", "  ")
	return pmReplanReceiptMarker + "\n\n```json\n" + string(encoded) + "\n```\n"
}

func FormatPMReplan(report PMReplanReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "pm replan: #%d mode=%s plan=%s matched=%t mutations=%d residual=%d\n", report.Ticket, report.Mode, report.PlanID, report.Matched, len(report.Mutations), len(report.ResidualActions))
	for _, a := range report.Mutations {
		fmt.Fprintf(&b, "- %s %s status=%s reason=%s\n", a.Action, a.Target, a.Status, a.Reason)
	}
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}
