package gira

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	PMWorkGraphSourceSchemaVersion  = "pm-work-graph-source/v1"
	PMWorkGraphReportSchemaVersion  = "pm-work-graph-report/v1"
	PMWorkGraphCompactSchemaVersion = "pm-work-graph-compact/v1"
)

const (
	PMWorkGraphMissingSource      = "PWG001_MISSING_SOURCE"
	PMWorkGraphInvalidNode        = "PWG002_INVALID_NODE"
	PMWorkGraphMissingOutcome     = "PWG003_MISSING_OUTCOME"
	PMWorkGraphUnverifiable       = "PWG004_UNVERIFIABLE_NODE"
	PMWorkGraphCycle              = "PWG005_DEPENDENCY_CYCLE"
	PMWorkGraphFalseDependency    = "PWG006_FALSE_DEPENDENCY"
	PMWorkGraphMissingResume      = "PWG007_MISSING_RESUME"
	PMWorkGraphOversized          = "PWG008_OVERSIZED_SLICE"
	PMWorkGraphUnresolvedJudgment = "PWG009_UNRESOLVED_JUDGMENT"
	PMWorkGraphUnknownOutcome     = "PWG010_UNKNOWN_OUTCOME"
	PMWorkGraphPlanChanged        = "PWG011_PLAN_CHANGED"
	PMWorkGraphInvalidPMIR        = "PWG012_INVALID_PM_IR"
	PMWorkGraphInvalidDiscovery   = "PWG013_INVALID_DISCOVERY_STATE"
)

type PMWorkGraphSource struct {
	SchemaVersion string            `json:"schema_version"`
	Nodes         []PMWorkGraphNode `json:"nodes"`
}

type PMWorkGraphNode struct {
	ID              string                    `json:"id"`
	Title           string                    `json:"title"`
	Purpose         string                    `json:"purpose"`
	Profile         string                    `json:"profile"`
	ParentOutcome   string                    `json:"parent_outcome"`
	Verification    []PMWorkGraphVerification `json:"verification"`
	Dependencies    []PMWorkGraphDependency   `json:"dependencies,omitempty"`
	ResumeCondition string                    `json:"resume_condition,omitempty"`
	Size            string                    `json:"size,omitempty"`
	Uncertainty     string                    `json:"uncertainty,omitempty"`
	TargetRepo      string                    `json:"target_repo,omitempty"`
	SupersedesIssue int                       `json:"supersedes_issue,omitempty"`
	DeferUntil      string                    `json:"defer_until,omitempty"`
	SplitInto       []string                  `json:"split_into,omitempty"`
}

type PMWorkGraphVerification struct {
	Method   string `json:"method"`
	Evidence string `json:"evidence"`
}
type PMWorkGraphDependency struct {
	NodeID string `json:"node_id"`
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}
type PMWorkGraphDiagnostic struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	NodeID   string `json:"node_id,omitempty"`
	Reason   string `json:"reason"`
	Repair   string `json:"repair"`
}
type PMWorkGraphAction struct {
	NodeID        string `json:"node_id"`
	Action        string `json:"action"`
	Status        string `json:"status"`
	ExistingIssue int    `json:"existing_issue,omitempty"`
	CreatedIssue  int    `json:"created_issue,omitempty"`
	Reason        string `json:"reason"`
}

type PMWorkGraphInput struct {
	Repo           RepoRef
	Goal           int
	DryRun         bool
	Apply          bool
	ExpectedPlanID string
}

type PMWorkGraphReport struct {
	Command         string                  `json:"command"`
	SchemaVersion   string                  `json:"schema_version"`
	Mode            string                  `json:"mode"`
	ReadOnly        bool                    `json:"read_only"`
	Repo            string                  `json:"repo"`
	Goal            GoalStatusIssue         `json:"goal"`
	PMIRDigest      string                  `json:"pm_ir_digest"`
	PMIRDiagnostics []PMCompileDiagnostic   `json:"pm_ir_diagnostics,omitempty"`
	CandidateWork   []string                `json:"candidate_work"`
	DiscoverySchema string                  `json:"discovery_schema"`
	OutcomeRefs     []string                `json:"outcome_refs"`
	Nodes           []PMWorkGraphNode       `json:"nodes"`
	Order           []string                `json:"order"`
	Actions         []PMWorkGraphAction     `json:"actions"`
	Diagnostics     []PMWorkGraphDiagnostic `json:"diagnostics"`
	PlanID          string                  `json:"plan_id"`
	ExpectedPlanID  string                  `json:"expected_plan_id,omitempty"`
	Matched         bool                    `json:"matched"`
	Idempotent      bool                    `json:"idempotent"`
	Created         []GoalPlanChild         `json:"created,omitempty"`
	NextStep        string                  `json:"next_step"`
}

type PMWorkGraphCompactNode struct {
	ID                string   `json:"id"`
	Profile           string   `json:"profile"`
	Action            string   `json:"action"`
	DependsOn         []string `json:"depends_on,omitempty"`
	VerificationCount int      `json:"verification_count"`
	PayloadSHA256     string   `json:"payload_sha256,omitempty"`
}
type PMWorkGraphCompactReport struct {
	Command        string                   `json:"command"`
	SchemaVersion  string                   `json:"schema_version"`
	Mode           string                   `json:"mode"`
	Repo           string                   `json:"repo"`
	Goal           int                      `json:"goal"`
	PlanID         string                   `json:"plan_id"`
	ExpectedPlanID string                   `json:"expected_plan_id,omitempty"`
	Matched        bool                     `json:"matched"`
	Idempotent     bool                     `json:"idempotent"`
	Nodes          []PMWorkGraphCompactNode `json:"nodes"`
	Diagnostics    []PMWorkGraphDiagnostic  `json:"diagnostics"`
	Created        []GoalPlanChild          `json:"created,omitempty"`
	DetailCommand  string                   `json:"detail_command"`
}

func BuildPMWorkGraphReport(input PMWorkGraphInput, runner CommandRunner) (PMWorkGraphReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	mode := "compile"
	if input.DryRun {
		mode = "dry_run"
	}
	if input.Apply {
		mode = "apply"
	}
	report := PMWorkGraphReport{Command: "goal graph", SchemaVersion: PMWorkGraphReportSchemaVersion, Mode: mode, ReadOnly: !input.Apply, Repo: input.Repo.FullName(), ExpectedPlanID: strings.TrimSpace(input.ExpectedPlanID), Matched: true, Nodes: []PMWorkGraphNode{}, Order: []string{}, Actions: []PMWorkGraphAction{}, Diagnostics: []PMWorkGraphDiagnostic{}, Created: []GoalPlanChild{}}
	if input.DryRun && input.Apply {
		return report, fmt.Errorf("dry_run and apply are mutually exclusive")
	}
	if input.Apply && report.ExpectedPlanID == "" {
		return report, fmt.Errorf("apply requires expected plan fingerprint")
	}
	goalNumber, _, err := ResolveGoalNumber(input.Repo, input.Goal, runner)
	if err != nil {
		return report, err
	}
	goal, err := fetchDevIssue(input.Repo, goalNumber, runner)
	if err != nil {
		return report, err
	}
	status, err := BuildGoalStatusReport(GoalStatusInput{Repo: input.Repo, Goal: goalNumber}, runner)
	if err != nil {
		return report, err
	}
	report.Goal = status.Goal
	if input.Apply && pmWorkGraphReceiptPresent(input.Repo, goalNumber, report.ExpectedPlanID, runner) {
		report.PlanID = report.ExpectedPlanID
		report.Idempotent = true
		report.Matched = true
		report.NextStep = fmt.Sprintf("gira goal status %d --repo %s --json", goalNumber, input.Repo.FullName())
		return report, nil
	}
	compile, err := BuildPMCompileReport(PMCompileInput{RawIntent: goal.Body, Repo: input.Repo.FullName(), Goal: &PMCompileGoal{Number: goal.Number, Title: goal.Title, Body: goal.Body, URL: githubIssueURL(input.Repo, goal.Number)}})
	if err != nil {
		return report, err
	}
	report.PMIRDigest = compile.IR.SourceDigest
	report.PMIRDiagnostics = compile.Diagnostics
	if compile.Summary.Errors > 0 {
		report.Diagnostics = append(report.Diagnostics, workGraphDiagnostic("error", PMWorkGraphInvalidPMIR, "", fmt.Sprintf("pm-ir/v1 contains %d compiler errors", compile.Summary.Errors), "repair the Goal source fields before lowering work"))
	}
	for _, item := range compile.IR.CandidateWork.Items {
		report.CandidateWork = append(report.CandidateWork, item.Value)
	}
	context, contextErr := BuildPMContextReport(PMContextInput{Repo: input.Repo, Ticket: goalNumber}, runner)
	outcomeSet := map[string]bool{"goal:" + strconv.Itoa(goalNumber): true}
	if contextErr == nil {
		report.DiscoverySchema = PMDiscoveryReportSchemaVersion
		if hasPMLedgerErrors(context.Diagnostics) {
			report.Diagnostics = append(report.Diagnostics, workGraphDiagnostic("error", PMWorkGraphInvalidDiscovery, "", "current typed PM discovery state contains errors", "repair or supersede invalid PM ledger records before lowering work"))
		}
		for _, item := range context.Records {
			if item.Current && item.Record.Kind == "outcome" {
				outcomeSet[item.Record.ID] = true
			}
		}
	}
	for ref := range outcomeSet {
		report.OutcomeRefs = append(report.OutcomeRefs, ref)
	}
	sort.Strings(report.OutcomeRefs)
	source, parseErr := parsePMWorkGraphSource(goal.Body)
	if parseErr != nil {
		report.Diagnostics = append(report.Diagnostics, workGraphDiagnostic("error", PMWorkGraphMissingSource, "", parseErr.Error(), "add a valid pm-work-graph-source/v1 JSON block under Work Graph"))
	} else {
		report.Nodes = normalizePMWorkGraphNodes(source.Nodes)
		report.Diagnostics = append(report.Diagnostics, validatePMWorkGraph(report.Nodes, outcomeSet)...)
		report.Order, err = pmWorkGraphOrder(report.Nodes)
		if err != nil {
			report.Diagnostics = append(report.Diagnostics, workGraphDiagnostic("error", PMWorkGraphCycle, "", err.Error(), "remove the dependency cycle"))
		}
	}
	report.Actions = pmWorkGraphActions(report.Nodes, status.Children, input.Repo)
	for _, action := range report.Actions {
		if action.Action == "supersede" && action.Status == "blocked" {
			report.Diagnostics = append(report.Diagnostics, workGraphDiagnostic("error", PMWorkGraphInvalidNode, action.NodeID, "supersede target is not an existing Goal child", "reference an existing child issue"))
		}
	}
	report.PlanID = pmWorkGraphFingerprint(report)
	if report.ExpectedPlanID != "" && report.ExpectedPlanID != report.PlanID {
		report.Matched = false
		report.Diagnostics = append(report.Diagnostics, workGraphDiagnostic("error", PMWorkGraphPlanChanged, "", "work graph fingerprint changed", "rerun dry-run and approve the new plan"))
	}
	sortPMWorkGraphDiagnostics(report.Diagnostics)
	report.NextStep = fmt.Sprintf("gira goal graph %d --repo %s --dry-run --compact-json", goalNumber, input.Repo.FullName())
	if input.Apply {
		if !report.Matched || hasPMWorkGraphErrors(report.Diagnostics) {
			return report, fmt.Errorf("work graph is not safe to apply")
		}
		if err := applyPMWorkGraph(&report, input.Repo, runner); err != nil {
			return report, err
		}
		report.NextStep = fmt.Sprintf("gira goal status %d --repo %s --json", goalNumber, input.Repo.FullName())
	}
	return report, nil
}

func parsePMWorkGraphSource(body string) (PMWorkGraphSource, error) {
	section := strings.TrimSpace(markdownSection(body, "Work Graph"))
	if section == "" {
		return PMWorkGraphSource{}, fmt.Errorf("Work Graph section is missing")
	}
	raw := section
	if start := strings.Index(section, "```"); start >= 0 {
		lineEnd := strings.Index(section[start:], "\n")
		if lineEnd < 0 {
			return PMWorkGraphSource{}, fmt.Errorf("Work Graph JSON fence is malformed")
		}
		contentStart := start + lineEnd + 1
		end := strings.Index(section[contentStart:], "```")
		if end < 0 {
			return PMWorkGraphSource{}, fmt.Errorf("Work Graph JSON fence is unterminated")
		}
		raw = section[contentStart : contentStart+end]
	}
	var source PMWorkGraphSource
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &source); err != nil {
		return source, fmt.Errorf("parse Work Graph JSON: %w", err)
	}
	if source.SchemaVersion != PMWorkGraphSourceSchemaVersion {
		return source, fmt.Errorf("Work Graph schema_version must be %q", PMWorkGraphSourceSchemaVersion)
	}
	if len(source.Nodes) == 0 {
		return source, fmt.Errorf("Work Graph requires nodes")
	}
	return source, nil
}

func normalizePMWorkGraphNodes(nodes []PMWorkGraphNode) []PMWorkGraphNode {
	out := append([]PMWorkGraphNode(nil), nodes...)
	for i := range out {
		n := &out[i]
		n.ID = strings.TrimSpace(n.ID)
		n.Title = strings.TrimSpace(n.Title)
		n.Purpose = strings.TrimSpace(n.Purpose)
		n.Profile = normalizePMTaskProfile(n.Profile)
		n.ParentOutcome = strings.TrimSpace(n.ParentOutcome)
		n.ResumeCondition = strings.TrimSpace(n.ResumeCondition)
		n.Size = strings.ToLower(strings.TrimSpace(n.Size))
		n.Uncertainty = strings.ToLower(strings.TrimSpace(n.Uncertainty))
		n.TargetRepo = strings.TrimSpace(n.TargetRepo)
		n.DeferUntil = strings.TrimSpace(n.DeferUntil)
		n.SplitInto = decompositionUniqueSorted(n.SplitInto)
		for j := range n.Dependencies {
			n.Dependencies[j].NodeID = strings.TrimSpace(n.Dependencies[j].NodeID)
			n.Dependencies[j].Kind = strings.ToLower(strings.TrimSpace(n.Dependencies[j].Kind))
			n.Dependencies[j].Reason = strings.TrimSpace(n.Dependencies[j].Reason)
		}
		sort.Slice(n.Dependencies, func(a, b int) bool { return n.Dependencies[a].NodeID < n.Dependencies[b].NodeID })
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func validatePMWorkGraph(nodes []PMWorkGraphNode, outcomes map[string]bool) []PMWorkGraphDiagnostic {
	out := []PMWorkGraphDiagnostic{}
	byID := map[string]PMWorkGraphNode{}
	for _, n := range nodes {
		if n.ID == "" || n.Title == "" || n.Purpose == "" {
			out = append(out, workGraphDiagnostic("error", PMWorkGraphInvalidNode, n.ID, "id, title, and purpose are required", "complete the typed node"))
		}
		if _, exists := byID[n.ID]; exists {
			out = append(out, workGraphDiagnostic("error", PMWorkGraphInvalidNode, n.ID, "duplicate node id", "use stable unique IDs"))
		}
		byID[n.ID] = n
		if _, ok := FindPMTaskProfile(n.Profile); !ok {
			out = append(out, workGraphDiagnostic("error", PMWorkGraphInvalidNode, n.ID, "unsupported task profile", "use a documented PM task profile"))
		}
		if n.ParentOutcome == "" {
			out = append(out, workGraphDiagnostic("error", PMWorkGraphMissingOutcome, n.ID, "parent outcome is missing", "reference a typed outcome or goal:N"))
		} else if !outcomes[n.ParentOutcome] {
			out = append(out, workGraphDiagnostic("error", PMWorkGraphUnknownOutcome, n.ID, "parent outcome does not exist in current discovery state", "record the outcome or correct the reference"))
		}
		if len(n.Verification) == 0 {
			out = append(out, workGraphDiagnostic("error", PMWorkGraphUnverifiable, n.ID, "verification contract is missing", "add method and expected evidence"))
		}
		for _, v := range n.Verification {
			if strings.TrimSpace(v.Method) == "" || strings.TrimSpace(v.Evidence) == "" {
				out = append(out, workGraphDiagnostic("error", PMWorkGraphUnverifiable, n.ID, "verification contract is incomplete", "add method and expected evidence"))
			}
		}
		if (n.Size == "large" || n.Size == "oversized") && len(n.SplitInto) == 0 {
			out = append(out, workGraphDiagnostic("error", PMWorkGraphOversized, n.ID, "oversized node has no split plan", "list independently verifiable split_into node IDs"))
		}
		if !containsPMValue([]string{"small", "medium", "large", "oversized"}, n.Size) {
			out = append(out, workGraphDiagnostic("error", PMWorkGraphInvalidNode, n.ID, "size is missing or invalid", "use small, medium, large, or oversized"))
		}
		if n.DeferUntil != "" && n.ResumeCondition == "" {
			out = append(out, workGraphDiagnostic("error", PMWorkGraphMissingResume, n.ID, "deferred node lacks a resume condition", "add an observable resume condition"))
		}
	}
	for _, n := range nodes {
		hasProductPredecessor := false
		dependencyIDs := map[string]bool{}
		for _, d := range n.Dependencies {
			dependencyIDs[d.NodeID] = true
			target, ok := byID[d.NodeID]
			if !ok {
				out = append(out, workGraphDiagnostic("error", PMWorkGraphFalseDependency, n.ID, "dependency target is missing", "reference an existing node"))
				continue
			}
			if !containsPMValue([]string{"ordering", "information"}, d.Kind) || d.Reason == "" {
				out = append(out, workGraphDiagnostic("error", PMWorkGraphFalseDependency, n.ID, "dependency lacks ordering/information semantics and reason", "state the actual ordering or information need"))
			}
			if containsPMValue([]string{"discovery", "decision", "experiment"}, target.Profile) {
				hasProductPredecessor = true
			}
		}
		if n.DeferUntil != "" {
			if _, ok := byID[n.DeferUntil]; !ok {
				out = append(out, workGraphDiagnostic("error", PMWorkGraphMissingResume, n.ID, "defer_until target does not exist", "reference a resolving node in the graph"))
			} else if !dependencyIDs[n.DeferUntil] {
				out = append(out, workGraphDiagnostic("error", PMWorkGraphFalseDependency, n.ID, "defer_until target is not a declared dependency", "add an ordering or information dependency on the resolving node"))
			}
		}
		if n.Profile == "delivery" && n.Uncertainty != "resolved" {
			if !hasProductPredecessor {
				out = append(out, workGraphDiagnostic("error", PMWorkGraphUnresolvedJudgment, n.ID, "delivery has unresolved product judgment without discovery/decision predecessor", "add discovery, decision, or experiment work before delivery"))
			}
			if n.DeferUntil == "" || n.ResumeCondition == "" {
				out = append(out, workGraphDiagnostic("error", PMWorkGraphMissingResume, n.ID, "unresolved delivery is not deferred behind an observable resume condition", "set defer_until and resume_condition for the resolving work"))
			}
		}
		for _, child := range n.SplitInto {
			if _, ok := byID[child]; !ok {
				out = append(out, workGraphDiagnostic("error", PMWorkGraphOversized, n.ID, "split target does not exist", "add every split_into node to the graph"))
			}
		}
	}
	return out
}

func pmWorkGraphOrder(nodes []PMWorkGraphNode) ([]string, error) {
	units := map[string]DecomposedWorkUnit{}
	for _, n := range nodes {
		deps := []string{}
		for _, d := range n.Dependencies {
			deps = append(deps, d.NodeID)
		}
		units[n.ID] = DecomposedWorkUnit{ID: n.ID, Dependencies: decompositionUniqueSorted(deps)}
	}
	return decompositionTopologicalOrder(units)
}

func pmWorkGraphActions(nodes []PMWorkGraphNode, children []GoalStatusChild, repo RepoRef) []PMWorkGraphAction {
	actions := []PMWorkGraphAction{}
	for _, n := range nodes {
		action := PMWorkGraphAction{NodeID: n.ID, Action: "create", Status: "planned", Reason: "new independently verifiable node"}
		target := repo
		if n.TargetRepo != "" {
			if parsed, err := ParseRepoRef(n.TargetRepo); err == nil {
				target = parsed
			}
		}
		if issue := goalPlanDuplicateChildNumber(goalPlanTicketTitle(n.Title), target, children); issue > 0 {
			action.Action = "reuse"
			action.ExistingIssue = issue
			action.Reason = "equivalent existing child"
		} else if n.SupersedesIssue > 0 {
			action.Action = "supersede"
			action.ExistingIssue = n.SupersedesIssue
			action.Reason = "explicit replacement"
			found := false
			for _, child := range children {
				if child.Number == n.SupersedesIssue && (child.Repo == "" || child.Repo == target.FullName()) {
					found = true
				}
			}
			if !found {
				action.Status = "blocked"
			}
		} else if len(n.SplitInto) > 0 {
			action.Action = "split"
			action.Reason = "oversized slice decomposed"
		} else if n.DeferUntil != "" {
			action.Action = "defer"
			action.Reason = n.DeferUntil
		}
		actions = append(actions, action)
	}
	return actions
}

func pmWorkGraphFingerprint(report PMWorkGraphReport) string {
	value := struct {
		Repo          string
		Goal          GoalStatusIssue
		Digest        string
		CandidateWork []string
		Outcomes      []string
		Nodes         []PMWorkGraphNode
	}{report.Repo, report.Goal, report.PMIRDigest, report.CandidateWork, report.OutcomeRefs, report.Nodes}
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return "pwg-" + hex.EncodeToString(sum[:16])
}

func applyPMWorkGraph(report *PMWorkGraphReport, repo RepoRef, runner CommandRunner) error {
	nodes := map[string]PMWorkGraphNode{}
	for _, n := range report.Nodes {
		nodes[n.ID] = n
	}
	labels := goalPlanLabels(report.Goal.Labels)
	preflighted := map[string]bool{}
	for _, action := range report.Actions {
		if action.Action != "create" && action.Action != "supersede" {
			continue
		}
		target, err := pmWorkGraphTargetRepo(repo, nodes[action.NodeID])
		if err != nil {
			return err
		}
		if !preflighted[target.FullName()] {
			if err := preflightTicketNewLabels(target, labels, runner); err != nil {
				return fmt.Errorf("preflight %s: %w", target.FullName(), err)
			}
			preflighted[target.FullName()] = true
		}
	}
	for i := range report.Actions {
		a := &report.Actions[i]
		n := nodes[a.NodeID]
		switch a.Action {
		case "create", "supersede":
			target, err := pmWorkGraphTargetRepo(repo, n)
			if err != nil {
				return err
			}
			body := renderPMWorkGraphTicket(report.Goal.Number, repo, target, n)
			milestone := report.Goal.Milestone
			if target.FullName() != repo.FullName() {
				milestone = ""
			}
			created, err := createRepoTicket(target, goalPlanTicketTitle(n.Title), body, labels, milestone, runner)
			if err != nil {
				return err
			}
			a.CreatedIssue = created.Number
			a.Status = "applied"
			report.Created = append(report.Created, GoalPlanChild{Repo: target.FullName(), Number: created.Number, Title: n.Title, Category: "ready", Status: "Ready", URL: created.URL})
			if target.FullName() == repo.FullName() {
				child, fetchErr := fetchGitHubIssue(target, created.Number, runner)
				if fetchErr != nil {
					return fmt.Errorf("fetch created work graph issue for parent link: %w", fetchErr)
				}
				if linkErr := addGitHubSubIssue(repo, report.Goal.Number, child.ID, false, runner); linkErr != nil {
					return fmt.Errorf("link work graph issue to Goal: %w", linkErr)
				}
			}
			if a.Action == "supersede" {
				message := fmt.Sprintf("Superseded by #%d through typed Goal work graph %s.", created.Number, report.PlanID)
				if _, err = runner.Run("gh", "issue", "comment", strconv.Itoa(a.ExistingIssue), "--repo", target.FullName(), "--body", message); err != nil {
					return err
				}
				if _, err = runner.Run("gh", "issue", "close", strconv.Itoa(a.ExistingIssue), "--repo", target.FullName(), "--reason", "not planned"); err != nil {
					return err
				}
			}
		case "reuse", "split", "defer":
			a.Status = "applied"
		}
	}
	body := renderPMWorkGraphReceipt(*report)
	if _, err := runner.Run("gh", "issue", "comment", strconv.Itoa(report.Goal.Number), "--repo", repo.FullName(), "--body", body); err != nil {
		return err
	}
	return nil
}

func pmWorkGraphTargetRepo(parent RepoRef, node PMWorkGraphNode) (RepoRef, error) {
	if node.TargetRepo == "" {
		return parent, nil
	}
	return ParseRepoRef(node.TargetRepo)
}

func renderPMWorkGraphTicket(goal int, parentRepo, targetRepo RepoRef, n PMWorkGraphNode) string {
	profile, _ := FindPMTaskProfile(n.Profile)
	var b strings.Builder
	b.WriteString(PMStateMarker + "\n")
	fmt.Fprintf(&b, "<!-- gira:task-packet schema=%s -->\n<!-- gira:task-profile/v1 profile=%s -->\n\n# %s\n\n", PMTaskPacketV2SchemaVersion, n.Profile, n.Title)
	parentRef := fmt.Sprintf("#%d", goal)
	if parentRepo.FullName() != targetRepo.FullName() {
		parentRef = fmt.Sprintf("%s#%d", parentRepo.FullName(), goal)
	}
	fmt.Fprintf(&b, "Parent: %s\n\n", parentRef)
	for _, section := range append(append([]string{}, profile.RequiredSections...), profile.VerificationSections...) {
		fmt.Fprintf(&b, "## %s\n\n", section)
		switch section {
		case "Actor":
			b.WriteString("PM-selected worker for this profile.\n\n")
		case "Problem":
			b.WriteString(n.Purpose + "\n\n")
		case "Desired Outcome":
			b.WriteString(n.ParentOutcome + "\n\n")
		case "Goal Alignment":
			fmt.Fprintf(&b, "Parent Goal: %s#%d\n\n", parentRepo.FullName(), goal)
		case "Parent Context":
			fmt.Fprintf(&b, "- goal:%d\n- outcome:%s\n\n", goal, n.ParentOutcome)
		case "Source References":
			fmt.Fprintf(&b, "- issue:%s#%d\n\n", parentRepo.FullName(), goal)
		case "Non-goals":
			b.WriteString("- Work outside this typed node.\n\n")
		case "Product Uncertainty":
			b.WriteString(n.Uncertainty + "\n\n")
		case "Acceptance Criteria":
			for _, v := range n.Verification {
				fmt.Fprintf(&b, "- %s produces %s\n", v.Method, v.Evidence)
			}
			b.WriteString("\n")
		case "Dependencies":
			for _, d := range n.Dependencies {
				fmt.Fprintf(&b, "- %s (%s): %s\n", d.NodeID, d.Kind, d.Reason)
			}
			if len(n.Dependencies) == 0 {
				b.WriteString("None.\n")
			}
			b.WriteString("\n")
		default:
			if strings.Contains(strings.ToLower(section), "verification") || strings.Contains(strings.ToLower(section), "evidence") || strings.Contains(strings.ToLower(section), "receipt") {
				for _, v := range n.Verification {
					fmt.Fprintf(&b, "- %s => %s\n", v.Method, v.Evidence)
				}
				b.WriteString("\n")
			} else {
				b.WriteString(n.Purpose + "\n\n")
			}
		}
	}
	b.WriteString("## Doctor Impact\n\nNo-op unless this node changes repository diagnostics.\n\n## Resume Condition\n\n" + emptyPMWorkGraphValue(n.ResumeCondition, "Ready when dependencies and required information are satisfied.") + "\n")
	return b.String()
}

func renderPMWorkGraphReceipt(report PMWorkGraphReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gira:work-graph-receipt plan=%s -->\n## Typed Work Graph Receipt\n\n", report.PlanID)
	for _, a := range report.Actions {
		fmt.Fprintf(&b, "- %s: %s status=%s", a.NodeID, a.Action, a.Status)
		if a.CreatedIssue > 0 {
			fmt.Fprintf(&b, " issue=#%d", a.CreatedIssue)
		}
		if a.ExistingIssue > 0 {
			fmt.Fprintf(&b, " existing=#%d", a.ExistingIssue)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func pmWorkGraphReceiptPresent(repo RepoRef, goal int, plan string, runner CommandRunner) bool {
	raw, err := runner.Run("gh", "issue", "view", strconv.Itoa(goal), "--repo", repo.FullName(), "--json", "comments")
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
	marker := "<!-- gira:work-graph-receipt plan=" + plan + " -->"
	for _, c := range payload.Comments {
		if strings.Contains(c.Body, marker) {
			return true
		}
	}
	return false
}

func BuildPMWorkGraphCompact(report PMWorkGraphReport) PMWorkGraphCompactReport {
	compact := PMWorkGraphCompactReport{Command: report.Command, SchemaVersion: PMWorkGraphCompactSchemaVersion, Mode: report.Mode, Repo: report.Repo, Goal: report.Goal.Number, PlanID: report.PlanID, ExpectedPlanID: report.ExpectedPlanID, Matched: report.Matched, Idempotent: report.Idempotent, Nodes: []PMWorkGraphCompactNode{}, Diagnostics: report.Diagnostics, Created: report.Created, DetailCommand: fmt.Sprintf("gira goal graph %d --repo %s --json", report.Goal.Number, report.Repo)}
	actionByID := map[string]string{}
	for _, a := range report.Actions {
		actionByID[a.NodeID] = a.Action
	}
	parentRepo, _ := ParseRepoRef(report.Repo)
	for _, n := range report.Nodes {
		deps := []string{}
		for _, d := range n.Dependencies {
			deps = append(deps, d.NodeID)
		}
		targetRepo := parentRepo
		if n.TargetRepo != "" {
			if parsed, err := ParseRepoRef(n.TargetRepo); err == nil {
				targetRepo = parsed
			}
		}
		body := renderPMWorkGraphTicket(report.Goal.Number, parentRepo, targetRepo, n)
		sum := sha256.Sum256([]byte(body))
		compact.Nodes = append(compact.Nodes, PMWorkGraphCompactNode{ID: n.ID, Profile: n.Profile, Action: actionByID[n.ID], DependsOn: deps, VerificationCount: len(n.Verification), PayloadSHA256: hex.EncodeToString(sum[:])})
	}
	return compact
}

func FormatPMWorkGraph(report PMWorkGraphReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "goal graph: #%d mode=%s nodes=%d plan=%s matched=%t diagnostics=%d\n", report.Goal.Number, report.Mode, len(report.Nodes), report.PlanID, report.Matched, len(report.Diagnostics))
	for _, d := range report.Diagnostics {
		fmt.Fprintf(&b, "- %s %s %s: %s\n", d.Severity, d.Code, d.NodeID, d.Reason)
	}
	for _, a := range report.Actions {
		fmt.Fprintf(&b, "- %s %s %s\n", a.Action, a.Status, a.NodeID)
	}
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}
func workGraphDiagnostic(severity, code, node, reason, repair string) PMWorkGraphDiagnostic {
	return PMWorkGraphDiagnostic{Severity: severity, Code: code, NodeID: node, Reason: reason, Repair: repair}
}
func sortPMWorkGraphDiagnostics(values []PMWorkGraphDiagnostic) {
	sort.Slice(values, func(i, j int) bool {
		if values[i].Code != values[j].Code {
			return values[i].Code < values[j].Code
		}
		return values[i].NodeID < values[j].NodeID
	})
}
func hasPMWorkGraphErrors(values []PMWorkGraphDiagnostic) bool {
	for _, v := range values {
		if v.Severity == "error" {
			return true
		}
	}
	return false
}
func emptyPMWorkGraphValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
