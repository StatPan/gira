package gira

import (
	"fmt"
	"strings"
)

const GoalNextSchemaVersion = "goal-next/v1"

type GoalNextInput struct {
	Repo RepoRef `json:"repo"`
	Goal int     `json:"goal"`
}

type GoalNextReport struct {
	Command                 string              `json:"command"`
	SchemaVersion           string              `json:"schema_version"`
	Repo                    string              `json:"repo"`
	Goal                    GoalStatusIssue     `json:"goal"`
	Counts                  map[string]int      `json:"counts"`
	SelectedTicket          *GoalNextCandidate  `json:"selected_ticket,omitempty"`
	SkippedCandidates       []GoalNextCandidate `json:"skipped_candidates,omitempty"`
	Blockers                []string            `json:"blockers,omitempty"`
	StopReasons             []string            `json:"stop_reasons,omitempty"`
	NextAction              string              `json:"next_action"`
	NextStep                string              `json:"next_step"`
	RemainingAutonomousWork int                 `json:"remaining_autonomous_work"`
}

type GoalNextCandidate struct {
	Repo       string   `json:"repo,omitempty"`
	Number     int      `json:"number"`
	Title      string   `json:"title"`
	State      string   `json:"state"`
	Status     string   `json:"status"`
	Category   string   `json:"category"`
	Labels     []string `json:"labels,omitempty"`
	Blockers   []string `json:"blockers,omitempty"`
	Reason     string   `json:"reason"`
	NextAction string   `json:"next_action"`
	NextStep   string   `json:"next_step"`
	URL        string   `json:"url,omitempty"`
}

func BuildGoalNextReport(input GoalNextInput, runner CommandRunner) (GoalNextReport, error) {
	status, err := BuildGoalStatusReport(GoalStatusInput{Repo: input.Repo, Goal: input.Goal}, runner)
	if err != nil {
		return GoalNextReport{}, err
	}
	return BuildGoalNextReportFromStatus(input.Repo, status), nil
}

func BuildGoalNextReportFromStatus(repo RepoRef, status GoalStatusReport) GoalNextReport {
	report := GoalNextReport{
		Command:                 "goal next",
		SchemaVersion:           GoalNextSchemaVersion,
		Repo:                    repo.FullName(),
		Goal:                    status.Goal,
		Counts:                  copyStringIntMap(status.Counts),
		Blockers:                append([]string(nil), status.Blockers...),
		RemainingAutonomousWork: status.RemainingAutonomousWork,
	}
	if goalStatusIssueDone(status.Goal) {
		report.StopReasons = []string{"goal_done"}
		report.NextAction = "done"
		report.NextStep = "goal is done"
		return report
	}
	if len(status.Children) == 0 {
		report.StopReasons = []string{"no_child_tickets"}
		report.NextAction = "plan_children"
		report.NextStep = fmt.Sprintf("gira goal plan --repo %s --goal %d --dry-run", repo.FullName(), status.Goal.Number)
		return report
	}
	if len(status.Blockers) > 0 || status.Counts["blocked"] > 0 {
		report.StopReasons = appendUniqueStrings(report.StopReasons, "goal_blockers_present")
		report.NextAction = "resolve_blockers"
		report.NextStep = fmt.Sprintf("gira goal status --repo %s --goal %d --json", repo.FullName(), status.Goal.Number)
		report.SkippedCandidates = goalNextSkippedCandidates(status.Children, nil)
		return report
	}
	selected := goalNextSelectChild(status.Children)
	if selected != nil {
		candidate := goalNextCandidateFromChild(*selected, goalNextSelectedReason(*selected), goalNextSafeCommand(repo, *selected))
		report.SelectedTicket = &candidate
		report.NextAction = goalNextActionForChild(*selected)
		report.NextStep = candidate.NextStep
		report.SkippedCandidates = goalNextSkippedCandidates(status.Children, selected)
		return report
	}
	report.SkippedCandidates = goalNextSkippedCandidates(status.Children, nil)
	if status.RemainingAutonomousWork == 0 {
		if status.HandoffReceiptPresent {
			report.StopReasons = []string{"human_review_handoff_present"}
			report.NextAction = "human_review"
			report.NextStep = fmt.Sprintf("review goal-finish-receipt/v1 handoff on #%d", status.Goal.Number)
			return report
		}
		report.StopReasons = []string{"no_remaining_child_work"}
		report.NextAction = "finish_goal"
		report.NextStep = fmt.Sprintf("gira goal finish --repo %s --goal %d --dry-run", repo.FullName(), status.Goal.Number)
		return report
	}
	report.StopReasons = []string{"no_eligible_child_ticket"}
	report.NextAction = "inspect_goal"
	report.NextStep = fmt.Sprintf("gira goal status --repo %s --goal %d --json", repo.FullName(), status.Goal.Number)
	return report
}

func goalNextSelectChild(children []GoalStatusChild) *GoalStatusChild {
	for _, category := range []string{"in_review", "in_progress", "ready"} {
		for i := range children {
			child := &children[i]
			if child.Category == category && !goalChildRequiresHuman(child.Labels) {
				return child
			}
		}
	}
	return nil
}

func goalNextSkippedCandidates(children []GoalStatusChild, selected *GoalStatusChild) []GoalNextCandidate {
	out := []GoalNextCandidate{}
	for _, child := range children {
		if selected != nil && child.Number == selected.Number {
			continue
		}
		reason := goalNextSkipReason(child, selected)
		out = append(out, goalNextCandidateFromChild(child, reason, child.NextStep))
	}
	return out
}

func goalNextSkipReason(child GoalStatusChild, selected *GoalStatusChild) string {
	if goalChildRequiresHuman(child.Labels) {
		return "human_approval_required"
	}
	switch child.Category {
	case "done":
		return "done"
	case "closed_other":
		return "closed_other"
	case "blocked":
		return "blocked"
	case "unknown":
		return "unknown_status"
	}
	if len(child.Blockers) > 0 {
		return "blocked"
	}
	if selected != nil {
		return "wait_for_selected_child"
	}
	return "not_eligible"
}

func goalNextSelectedReason(child GoalStatusChild) string {
	switch child.Category {
	case "in_review":
		return "review_or_finish_before_new_work"
	case "in_progress":
		return "continue_active_child_before_new_work"
	case "ready":
		return "next_ready_child"
	default:
		return "selected"
	}
}

func goalNextActionForChild(child GoalStatusChild) string {
	switch child.Category {
	case "in_review":
		return "review_child"
	case "in_progress":
		return "continue_child"
	case "ready":
		return "start_child"
	default:
		return "inspect_child"
	}
}

func goalNextSafeCommand(repo RepoRef, child GoalStatusChild) string {
	childRepo := goalNextChildRepo(repo, child)
	switch child.Category {
	case "ready":
		return fmt.Sprintf("gira ticket start --repo %s --ticket %d --apply", childRepo.FullName(), child.Number)
	case "in_progress":
		if strings.TrimSpace(child.NextStep) != "" {
			return normalizeGoalNextCommand(child.NextStep)
		}
		return fmt.Sprintf("gira ticket pr --repo %s --ticket %d --dry-run", childRepo.FullName(), child.Number)
	case "in_review":
		if strings.TrimSpace(child.NextStep) != "" {
			return normalizeGoalNextCommand(child.NextStep)
		}
		return fmt.Sprintf("gira ticket finish --repo %s --ticket %d --dry-run", childRepo.FullName(), child.Number)
	default:
		if strings.TrimSpace(child.NextStep) != "" {
			return normalizeGoalNextCommand(child.NextStep)
		}
		return fmt.Sprintf("gira ticket status --repo %s --ticket %d --json", childRepo.FullName(), child.Number)
	}
}

func goalNextChildRepo(defaultRepo RepoRef, child GoalStatusChild) RepoRef {
	if repo, err := ParseRepoRef(child.Repo); err == nil {
		return repo
	}
	return defaultRepo
}

func goalNextCandidateFromChild(child GoalStatusChild, reason string, nextStep string) GoalNextCandidate {
	return GoalNextCandidate{
		Repo:       child.Repo,
		Number:     child.Number,
		Title:      child.Title,
		State:      child.State,
		Status:     child.Status,
		Category:   child.Category,
		Labels:     append([]string(nil), child.Labels...),
		Blockers:   append([]string(nil), child.Blockers...),
		Reason:     reason,
		NextAction: child.NextAction,
		NextStep:   normalizeGoalNextCommand(nextStep),
		URL:        child.URL,
	}
}

func normalizeGoalNextCommand(value string) string {
	normalized := strings.TrimSpace(value)
	normalized = strings.Replace(normalized, "gira work ", "gira ticket ", 1)
	normalized = strings.Replace(normalized, " --issue ", " --ticket ", 1)
	return normalized
}

func goalChildRequiresHuman(labels []string) bool {
	for _, label := range labels {
		lower := strings.ToLower(strings.TrimSpace(label))
		switch lower {
		case "agent:human", "needs:human", "human:required", "type:decision":
			return true
		}
	}
	return false
}

func copyStringIntMap(in map[string]int) map[string]int {
	out := map[string]int{}
	for key, value := range in {
		out[key] = value
	}
	return out
}

func FormatGoalNext(report GoalNextReport) string {
	var b strings.Builder
	if report.SelectedTicket != nil {
		fmt.Fprintf(&b, "goal next: #%d selected=#%d reason=%s next=%s\n", report.Goal.Number, report.SelectedTicket.Number, report.SelectedTicket.Reason, report.NextAction)
	} else {
		fmt.Fprintf(&b, "goal next: #%d stop=%s next=%s\n", report.Goal.Number, strings.Join(report.StopReasons, ","), report.NextAction)
	}
	if len(report.Blockers) > 0 {
		fmt.Fprintf(&b, "blockers: %s\n", strings.Join(report.Blockers, ","))
	}
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}
