package gira

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const GoalFinishReadinessSchemaVersion = "goal-finish-readiness/v1"
const GoalFinishReceiptSchemaVersion = "goal-finish-receipt/v1"

type GoalFinishInput struct {
	Repo     RepoRef `json:"repo"`
	Goal     int     `json:"goal"`
	DryRun   bool    `json:"dry_run"`
	Apply    bool    `json:"apply,omitempty"`
	Terminal string  `json:"terminal,omitempty"`
}

type GoalFinishReport struct {
	Command    string              `json:"command"`
	Repo       string              `json:"repo"`
	Goal       int                 `json:"goal"`
	DryRun     bool                `json:"dry_run"`
	Apply      bool                `json:"apply,omitempty"`
	Readiness  GoalFinishReadiness `json:"readiness"`
	Receipt    GoalFinishReceipt   `json:"receipt"`
	Actions    []GoalFinishAction  `json:"actions,omitempty"`
	NextAction string              `json:"next_action"`
	NextStep   string              `json:"next_step"`
}

type GoalFinishAction struct {
	Action string `json:"action"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type GoalFinishReadiness struct {
	SchemaVersion          string                    `json:"schema_version"`
	Repository             string                    `json:"repository"`
	Goal                   GoalStatusIssue           `json:"goal"`
	Ready                  bool                      `json:"ready"`
	TerminalRecommendation string                    `json:"terminal_recommendation"`
	Counts                 map[string]int            `json:"counts"`
	RemainingOpenWork      int                       `json:"remaining_open_work"`
	Children               []GoalFinishChildEvidence `json:"children"`
	Blockers               []string                  `json:"blockers"`
	Warnings               []string                  `json:"warnings,omitempty"`
	NextAction             string                    `json:"next_action"`
	NextStep               string                    `json:"next_step"`
}

type GoalFinishChildEvidence struct {
	Number         int      `json:"number"`
	Title          string   `json:"title"`
	State          string   `json:"state"`
	Status         string   `json:"status"`
	Category       string   `json:"category"`
	PRNumber       int      `json:"pr_number,omitempty"`
	PRURL          string   `json:"pr_url,omitempty"`
	PRState        string   `json:"pr_state,omitempty"`
	ChecksStatus   string   `json:"checks_status,omitempty"`
	ReviewStatus   string   `json:"review_status,omitempty"`
	ReceiptPresent bool     `json:"receipt_present"`
	Evidence       []string `json:"evidence,omitempty"`
	Blockers       []string `json:"blockers,omitempty"`
	URL            string   `json:"url,omitempty"`
}

type GoalFinishReceipt struct {
	SchemaVersion          string          `json:"schema_version"`
	FinishedAt             string          `json:"finished_at"`
	Repository             string          `json:"repository"`
	Goal                   GoalStatusIssue `json:"goal"`
	Ready                  bool            `json:"ready"`
	TerminalRecommendation string          `json:"terminal_recommendation"`
	ChildSummary           map[string]int  `json:"child_summary"`
	Blockers               []string        `json:"blockers,omitempty"`
	Warnings               []string        `json:"warnings,omitempty"`
	FinalState             string          `json:"final_state"`
	Target                 string          `json:"target"`
	RenderedBody           string          `json:"rendered_body"`
}

var goalFinishReceiptNow = func() time.Time { return time.Now().UTC() }

func BuildGoalFinishReport(input GoalFinishInput, runner CommandRunner) (GoalFinishReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if input.Goal <= 0 {
		return GoalFinishReport{}, fmt.Errorf("goal must be > 0")
	}
	if input.DryRun == input.Apply {
		return GoalFinishReport{}, fmt.Errorf("exactly one of --dry-run or --apply is required")
	}
	terminal := strings.TrimSpace(input.Terminal)
	if terminal == "" {
		terminal = "auto"
	}
	if !validGoalTerminalRecommendation(terminal) {
		return GoalFinishReport{}, fmt.Errorf("--terminal must be one of done, human_review, blocked, superseded, abandoned")
	}
	status, err := BuildGoalStatusReport(GoalStatusInput{Repo: input.Repo, Goal: input.Goal}, runner)
	if err != nil {
		return GoalFinishReport{}, err
	}
	readiness := buildGoalFinishReadiness(input.Repo, status, terminal, runner)
	receipt := buildGoalFinishReceipt(readiness)
	report := GoalFinishReport{
		Command:    "goal finish",
		Repo:       input.Repo.FullName(),
		Goal:       input.Goal,
		DryRun:     input.DryRun,
		Apply:      input.Apply,
		Readiness:  readiness,
		Receipt:    receipt,
		Actions:    goalFinishActions(input, readiness),
		NextAction: readiness.NextAction,
		NextStep:   readiness.NextStep,
	}
	if input.Apply {
		if err := applyGoalFinishHandoff(input, readiness, receipt, runner); err != nil {
			return report, err
		}
		for i := range report.Actions {
			if report.Actions[i].Action == "goal:comment" {
				report.Actions[i].Status = "applied"
			}
		}
		report.NextStep = "human review handoff receipt posted"
	}
	return report, nil
}

func validGoalTerminalRecommendation(value string) bool {
	switch value {
	case "auto", "done", "human_review", "blocked", "superseded", "abandoned":
		return true
	default:
		return false
	}
}

func buildGoalFinishReadiness(repo RepoRef, status GoalStatusReport, terminal string, runner CommandRunner) GoalFinishReadiness {
	readiness := GoalFinishReadiness{
		SchemaVersion:     GoalFinishReadinessSchemaVersion,
		Repository:        repo.FullName(),
		Goal:              status.Goal,
		Counts:            copyStringIntMap(status.Counts),
		RemainingOpenWork: status.RemainingAutonomousWork,
		Children:          []GoalFinishChildEvidence{},
		Blockers:          []string{},
		Warnings:          []string{},
	}
	for _, child := range status.Children {
		evidence := goalFinishChildEvidence(repo, child, runner)
		readiness.Children = append(readiness.Children, evidence)
		readiness.Blockers = appendUniqueStrings(readiness.Blockers, evidence.Blockers...)
	}
	if len(status.Children) == 0 {
		readiness.Blockers = appendUniqueStrings(readiness.Blockers, "no_child_tickets")
	}
	readiness.TerminalRecommendation = goalTerminalRecommendation(terminal, readiness)
	readiness.Ready = len(readiness.Blockers) == 0 && readiness.TerminalRecommendation == "done"
	readiness.NextAction, readiness.NextStep = goalFinishNextStep(repo, readiness)
	return readiness
}

func goalFinishChildEvidence(repo RepoRef, child GoalStatusChild, runner CommandRunner) GoalFinishChildEvidence {
	evidence := GoalFinishChildEvidence{
		Number:         child.Number,
		Title:          child.Title,
		State:          child.State,
		Status:         child.Status,
		Category:       child.Category,
		PRNumber:       child.PRNumber,
		PRURL:          child.PRURL,
		PRState:        child.PRState,
		ChecksStatus:   child.ChecksStatus,
		ReviewStatus:   child.ReviewStatus,
		ReceiptPresent: goalFinishReceiptPresent(repo, child.Number, runner),
		URL:            child.URL,
	}
	if child.PRNumber > 0 {
		evidence.Evidence = append(evidence.Evidence, "linked_pr")
	}
	if strings.EqualFold(child.PRState, "MERGED") {
		evidence.Evidence = append(evidence.Evidence, "merged_pr")
	}
	if child.ChecksStatus == "passed" {
		evidence.Evidence = append(evidence.Evidence, "checks_passed")
	}
	if evidence.ReceiptPresent {
		evidence.Evidence = append(evidence.Evidence, "finish_receipt")
	}
	evidence.Blockers = goalFinishChildBlockers(child, evidence)
	return evidence
}

func goalFinishChildBlockers(child GoalStatusChild, evidence GoalFinishChildEvidence) []string {
	blockers := []string{}
	if child.Category != "done" {
		blockers = append(blockers, fmt.Sprintf("child_%d_open", child.Number))
	}
	if child.Category == "blocked" {
		blockers = append(blockers, fmt.Sprintf("child_%d_blocked", child.Number))
	}
	if evidence.PRNumber == 0 {
		blockers = append(blockers, fmt.Sprintf("child_%d_missing_pr", child.Number))
	}
	if evidence.PRNumber > 0 && !strings.EqualFold(evidence.PRState, "MERGED") {
		blockers = append(blockers, fmt.Sprintf("child_%d_unmerged_pr", child.Number))
	}
	switch evidence.ChecksStatus {
	case "failed":
		blockers = append(blockers, fmt.Sprintf("child_%d_checks_failed", child.Number))
	case "pending":
		blockers = append(blockers, fmt.Sprintf("child_%d_checks_pending", child.Number))
	case "missing":
		blockers = append(blockers, fmt.Sprintf("child_%d_checks_missing", child.Number))
	}
	if child.Category == "done" && !evidence.ReceiptPresent {
		blockers = append(blockers, fmt.Sprintf("child_%d_missing_finish_receipt", child.Number))
	}
	return blockers
}

func goalFinishReceiptPresent(repo RepoRef, issue int, runner CommandRunner) bool {
	out, err := runner.Run("gh", "issue", "view", strconv.Itoa(issue), "--repo", repo.FullName(), "--json", "comments")
	if err != nil {
		return false
	}
	var raw struct {
		Comments []struct {
			Body string `json:"body"`
		} `json:"comments"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return false
	}
	for _, comment := range raw.Comments {
		lower := strings.ToLower(comment.Body)
		if strings.Contains(lower, "finish receipt") || strings.Contains(lower, "finish-receipt/v1") {
			return true
		}
	}
	return false
}

func goalTerminalRecommendation(requested string, readiness GoalFinishReadiness) string {
	if requested != "auto" {
		return requested
	}
	if len(readiness.Blockers) == 0 {
		return "done"
	}
	for _, blocker := range readiness.Blockers {
		if strings.Contains(blocker, "blocked") {
			return "blocked"
		}
	}
	return "human_review"
}

func goalFinishNextStep(repo RepoRef, readiness GoalFinishReadiness) (string, string) {
	if readiness.Ready {
		return "finish_goal", fmt.Sprintf("post goal receipt and close #%d when apply support is enabled", readiness.Goal.Number)
	}
	switch readiness.TerminalRecommendation {
	case "superseded", "abandoned":
		return "human_review", fmt.Sprintf("confirm terminal recommendation %s for #%d with a maintainer", readiness.TerminalRecommendation, readiness.Goal.Number)
	case "blocked":
		return "resolve_blockers", fmt.Sprintf("gira goal status --repo %s --goal %d --json", repo.FullName(), readiness.Goal.Number)
	default:
		return "human_review", fmt.Sprintf("review blockers, then rerun gira goal finish --repo %s --goal %d --dry-run --json", repo.FullName(), readiness.Goal.Number)
	}
}

func goalFinishActions(input GoalFinishInput, readiness GoalFinishReadiness) []GoalFinishAction {
	if input.DryRun && readiness.TerminalRecommendation == "human_review" && len(readiness.Blockers) > 0 {
		return []GoalFinishAction{{
			Action: "goal:comment",
			Status: "planned",
			Detail: fmt.Sprintf("post goal-finish-receipt/v1 human review handoff to issue #%d", readiness.Goal.Number),
		}}
	}
	if input.Apply && readiness.TerminalRecommendation == "human_review" && len(readiness.Blockers) > 0 {
		return []GoalFinishAction{{
			Action: "goal:comment",
			Status: "planned",
			Detail: fmt.Sprintf("post goal-finish-receipt/v1 human review handoff to issue #%d", readiness.Goal.Number),
		}}
	}
	return nil
}

func applyGoalFinishHandoff(input GoalFinishInput, readiness GoalFinishReadiness, receipt GoalFinishReceipt, runner CommandRunner) error {
	if strings.TrimSpace(input.Terminal) != "human_review" {
		return fmt.Errorf("goal finish --apply currently supports only explicit --terminal human_review")
	}
	if readiness.TerminalRecommendation != "human_review" || len(readiness.Blockers) == 0 {
		return fmt.Errorf("goal finish --apply requires terminal human_review with blockers to preserve a handoff instead of closing the goal")
	}
	if _, err := runner.Run("gh", "issue", "comment", strconv.Itoa(input.Goal), "--repo", input.Repo.FullName(), "--body", receipt.RenderedBody); err != nil {
		return fmt.Errorf("post goal finish receipt: %w", err)
	}
	return nil
}

func buildGoalFinishReceipt(readiness GoalFinishReadiness) GoalFinishReceipt {
	receipt := GoalFinishReceipt{
		SchemaVersion:          GoalFinishReceiptSchemaVersion,
		FinishedAt:             goalFinishReceiptNow().Format(time.RFC3339),
		Repository:             readiness.Repository,
		Goal:                   readiness.Goal,
		Ready:                  readiness.Ready,
		TerminalRecommendation: readiness.TerminalRecommendation,
		ChildSummary:           copyStringIntMap(readiness.Counts),
		Blockers:               append([]string(nil), readiness.Blockers...),
		Warnings:               append([]string(nil), readiness.Warnings...),
		FinalState:             readiness.NextAction,
		Target:                 fmt.Sprintf("issue#%d", readiness.Goal.Number),
	}
	receipt.RenderedBody = renderGoalFinishReceipt(receipt)
	return receipt
}

func renderGoalFinishReceipt(receipt GoalFinishReceipt) string {
	blockers := strings.Join(receipt.Blockers, ",")
	if blockers == "" {
		blockers = "none"
	}
	return fmt.Sprintf("## Goal Finish Receipt\n\n- Schema: %s\n- Finished at: %s\n- Goal: #%d status=%s state=%s\n- Ready: %t\n- Terminal recommendation: %s\n- Children: total=%d done=%d ready=%d in_progress=%d in_review=%d blocked=%d\n- Blockers: %s\n- Next: %s\n", receipt.SchemaVersion, receipt.FinishedAt, receipt.Goal.Number, receipt.Goal.Status, receipt.Goal.State, receipt.Ready, receipt.TerminalRecommendation, receipt.ChildSummary["total"], receipt.ChildSummary["done"], receipt.ChildSummary["ready"], receipt.ChildSummary["in_progress"], receipt.ChildSummary["in_review"], receipt.ChildSummary["blocked"], blockers, receipt.FinalState)
}

func FormatGoalFinish(report GoalFinishReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "goal finish: #%d ready=%t terminal=%s blockers=%d\n", report.Goal, report.Readiness.Ready, report.Readiness.TerminalRecommendation, len(report.Readiness.Blockers))
	if len(report.Readiness.Blockers) > 0 {
		fmt.Fprintf(&b, "blockers: %s\n", strings.Join(report.Readiness.Blockers, ","))
	}
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}
