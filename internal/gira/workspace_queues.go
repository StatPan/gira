package gira

import (
	"fmt"
	"sort"
	"strings"
)

const WorkspaceQueuesSchemaVersion = "workspace-queues/v1"

type WorkspaceQueuesReport struct {
	SchemaVersion   string                `json:"schema_version"`
	Workspace       WorkspaceSummary      `json:"workspace,omitempty"`
	Queues          WorkspaceQueues       `json:"queues"`
	Counts          WorkspaceQueueCounts  `json:"counts"`
	PrivacyBoundary WorkspaceQueuePrivacy `json:"privacy_boundary"`
}

type WorkspaceQueues struct {
	AgentReady    []WorkspaceQueueItem `json:"agent_ready"`
	ReviewNeeded  []WorkspaceQueueItem `json:"review_needed"`
	FinishReady   []WorkspaceQueueItem `json:"finish_ready"`
	Blocked       []WorkspaceQueueItem `json:"blocked"`
	FailedCheck   []WorkspaceQueueItem `json:"failed_check"`
	HumanDecision []WorkspaceQueueItem `json:"human_decision"`
}

type WorkspaceQueueCounts struct {
	AgentReady    int `json:"agent_ready"`
	ReviewNeeded  int `json:"review_needed"`
	FinishReady   int `json:"finish_ready"`
	Blocked       int `json:"blocked"`
	FailedCheck   int `json:"failed_check"`
	HumanDecision int `json:"human_decision"`
}

type WorkspaceQueueItem struct {
	Queue           string                 `json:"queue"`
	Repo            string                 `json:"repo"`
	Issue           int                    `json:"issue"`
	Title           string                 `json:"title"`
	State           string                 `json:"state"`
	Status          string                 `json:"status"`
	Labels          []string               `json:"labels,omitempty"`
	Milestone       string                 `json:"milestone,omitempty"`
	PullRequest     *WorkspaceQueuePR      `json:"pull_request,omitempty"`
	Evidence        WorkspaceQueueEvidence `json:"evidence"`
	ReasonCodes     []string               `json:"reason_codes"`
	NextSafeCommand string                 `json:"next_safe_command"`
}

type WorkspaceQueuePR struct {
	Number         int    `json:"number"`
	URL            string `json:"url,omitempty"`
	State          string `json:"state,omitempty"`
	Draft          bool   `json:"draft,omitempty"`
	ReviewDecision string `json:"review_decision,omitempty"`
}

type WorkspaceQueueEvidence struct {
	TicketReadiness string   `json:"ticket_readiness,omitempty"`
	PRReadiness     string   `json:"pr_readiness,omitempty"`
	ChecksStatus    string   `json:"checks_status,omitempty"`
	ReviewStatus    string   `json:"review_status,omitempty"`
	NextAction      string   `json:"next_action,omitempty"`
	Blockers        []string `json:"blockers,omitempty"`
}

type WorkspaceQueuePrivacy struct {
	Scope      string   `json:"scope"`
	Prohibited []string `json:"prohibited"`
}

func BuildWorkspaceQueues(workspace WorkspaceSummary, statuses []WorkStatusResult) WorkspaceQueuesReport {
	report := WorkspaceQueuesReport{
		SchemaVersion: WorkspaceQueuesSchemaVersion,
		Workspace:     workspace,
		PrivacyBoundary: WorkspaceQueuePrivacy{
			Scope: "work_item_state_only",
			Prohibited: []string{
				"personal_productivity_ranking",
				"agent_productivity_ranking",
				"time_online_scoring",
				"token_spend_scoring",
			},
		},
	}
	items := append([]WorkStatusResult(nil), statuses...)
	sort.Slice(items, func(i, j int) bool {
		if !strings.EqualFold(items[i].Repo, items[j].Repo) {
			return strings.ToLower(items[i].Repo) < strings.ToLower(items[j].Repo)
		}
		return items[i].Issue < items[j].Issue
	})
	for _, status := range items {
		if isClosedWorkspaceQueueIssue(status) {
			continue
		}
		if reasonCodes := workspaceAgentReadyReasons(status); len(reasonCodes) > 0 {
			item := workspaceQueueItem("agent_ready", status, reasonCodes, fmt.Sprintf("gira ticket start --repo %s --ticket %d --apply", QuoteShellArg(status.Repo), status.Issue))
			report.Queues.AgentReady = append(report.Queues.AgentReady, item)
		}
		if reasonCodes := workspaceReviewNeededReasons(status); len(reasonCodes) > 0 {
			item := workspaceQueueItem("review_needed", status, reasonCodes, fmt.Sprintf("gira ticket review --repo %s --ticket %d --json", QuoteShellArg(status.Repo), status.Issue))
			report.Queues.ReviewNeeded = append(report.Queues.ReviewNeeded, item)
		}
		if reasonCodes := workspaceFinishReadyReasons(status); len(reasonCodes) > 0 {
			item := workspaceQueueItem("finish_ready", status, reasonCodes, fmt.Sprintf("gira ticket finish --repo %s --ticket %d --dry-run", QuoteShellArg(status.Repo), status.Issue))
			report.Queues.FinishReady = append(report.Queues.FinishReady, item)
		}
		if reasonCodes := workspaceBlockedReasons(status); len(reasonCodes) > 0 {
			item := workspaceQueueItem("blocked", status, reasonCodes, fmt.Sprintf("gira ticket status --repo %s --ticket %d --json", QuoteShellArg(status.Repo), status.Issue))
			report.Queues.Blocked = append(report.Queues.Blocked, item)
		}
		if reasonCodes := workspaceFailedCheckReasons(status); len(reasonCodes) > 0 {
			item := workspaceQueueItem("failed_check", status, reasonCodes, fmt.Sprintf("gira ticket status --repo %s --ticket %d --json", QuoteShellArg(status.Repo), status.Issue))
			report.Queues.FailedCheck = append(report.Queues.FailedCheck, item)
		}
		if reasonCodes := workspaceHumanDecisionReasons(status); len(reasonCodes) > 0 {
			item := workspaceQueueItem("human_decision", status, reasonCodes, fmt.Sprintf("gira ticket handoff --repo %s --ticket %d planner --json", QuoteShellArg(status.Repo), status.Issue))
			report.Queues.HumanDecision = append(report.Queues.HumanDecision, item)
		}
	}
	report.Counts = WorkspaceQueueCounts{
		AgentReady:    len(report.Queues.AgentReady),
		ReviewNeeded:  len(report.Queues.ReviewNeeded),
		FinishReady:   len(report.Queues.FinishReady),
		Blocked:       len(report.Queues.Blocked),
		FailedCheck:   len(report.Queues.FailedCheck),
		HumanDecision: len(report.Queues.HumanDecision),
	}
	return report
}

func workspaceQueueItem(queue string, status WorkStatusResult, reasonCodes []string, nextSafeCommand string) WorkspaceQueueItem {
	return WorkspaceQueueItem{
		Queue:           queue,
		Repo:            status.Repo,
		Issue:           status.Issue,
		Title:           status.Title,
		State:           status.State,
		Status:          status.Status,
		Labels:          append([]string(nil), status.Labels...),
		Milestone:       status.Milestone,
		PullRequest:     workspaceQueuePR(status),
		Evidence:        workspaceQueueEvidence(status),
		ReasonCodes:     uniqueWorkspaceQueueReasons(reasonCodes),
		NextSafeCommand: nextSafeCommand,
	}
}

func workspaceQueuePR(status WorkStatusResult) *WorkspaceQueuePR {
	if status.PullRequest != nil && status.PullRequest.Number > 0 {
		return &WorkspaceQueuePR{
			Number:         status.PullRequest.Number,
			URL:            status.PullRequest.URL,
			State:          status.PullRequest.State,
			Draft:          status.PullRequest.IsDraft,
			ReviewDecision: status.PullRequest.ReviewDecision,
		}
	}
	if status.PRNumber > 0 {
		return &WorkspaceQueuePR{Number: status.PRNumber, URL: status.PRURL, State: status.PRState}
	}
	return nil
}

func workspaceQueueEvidence(status WorkStatusResult) WorkspaceQueueEvidence {
	evidence := WorkspaceQueueEvidence{
		ChecksStatus: status.ChecksStatus,
		ReviewStatus: status.ReviewStatus,
		NextAction:   status.NextAction,
		Blockers:     append([]string(nil), status.Blockers...),
	}
	if status.TicketReadiness != nil {
		evidence.TicketReadiness = status.TicketReadiness.Readiness
	}
	if status.PRReadiness != nil {
		evidence.PRReadiness = status.PRReadiness.Readiness
	}
	return evidence
}

func workspaceAgentReadyReasons(status WorkStatusResult) []string {
	if hasWorkspaceQueuePR(status) || !workspaceQueueStatusIs(status, "ready") || len(status.Blockers) > 0 {
		return nil
	}
	if len(workspaceHumanDecisionReasons(status)) > 0 {
		return nil
	}
	if status.TicketReadiness != nil && status.TicketReadiness.Readiness != "ready" {
		return nil
	}
	reasons := []string{"ticket_ready"}
	if status.TicketReadiness != nil {
		reasons = append(reasons, "ticket_readiness_ready")
	}
	return reasons
}

func workspaceReviewNeededReasons(status WorkStatusResult) []string {
	if !hasWorkspaceQueuePR(status) || workspaceQueuePRIsDraft(status) {
		return nil
	}
	if len(status.Blockers) > 0 || status.ChecksStatus == "failed" || status.ChecksStatus == "failing" || status.ReviewStatus == "approved" {
		return nil
	}
	if status.PullRequest != nil && strings.EqualFold(status.PullRequest.ReviewDecision, "APPROVED") {
		return nil
	}
	reasons := []string{}
	if workspaceQueueStatusIs(status, "in-review") || workspaceQueueStatusIs(status, "in review") {
		reasons = append(reasons, "status_in_review")
	}
	if status.ReviewStatus == "" || status.ReviewStatus == "missing" || status.ReviewStatus == "unknown" || status.ReviewStatus == "review_required" {
		reasons = append(reasons, "review_required")
	}
	if status.PullRequest != nil && strings.EqualFold(status.PullRequest.ReviewDecision, "REVIEW_REQUIRED") {
		reasons = append(reasons, "review_required")
	}
	if status.PRReadiness != nil && status.PRReadiness.NextAction == "request_review" {
		reasons = append(reasons, "pr_readiness_request_review")
	}
	return reasons
}

func workspaceFinishReadyReasons(status WorkStatusResult) []string {
	if !hasWorkspaceQueuePR(status) || workspaceQueuePRIsDraft(status) || len(status.Blockers) > 0 {
		return nil
	}
	reasons := []string{}
	if status.PRReadiness != nil && status.PRReadiness.Readiness == "ready_for_finish" {
		reasons = append(reasons, "pr_readiness_ready_for_finish")
	}
	if status.NextAction == "merge_when_policy_allows" || status.NextAction == "finish_ticket" {
		reasons = append(reasons, "next_action_finish")
	}
	if status.Evidence != nil && status.Evidence.FinishReady {
		reasons = append(reasons, "finish_ready_evidence")
	}
	if len(reasons) == 0 {
		return nil
	}
	if status.ChecksStatus != "" && status.ChecksStatus != "passed" {
		return nil
	}
	if status.ReviewStatus != "" && status.ReviewStatus != "approved" {
		return nil
	}
	return reasons
}

func workspaceBlockedReasons(status WorkStatusResult) []string {
	reasons := []string{}
	if workspaceQueueStatusIs(status, "blocked") {
		reasons = append(reasons, "status_blocked")
	}
	for _, blocker := range status.Blockers {
		reasons = append(reasons, "blocker_"+workspaceQueueReasonToken(blocker))
	}
	if status.TicketReadiness != nil {
		for _, finding := range status.TicketReadiness.Findings {
			if finding.Severity == "error" {
				reasons = append(reasons, "ticket_readiness_"+workspaceQueueReasonToken(finding.Kind))
			}
		}
	}
	if status.PRReadiness != nil {
		for _, finding := range status.PRReadiness.Findings {
			if finding.Severity == "error" {
				reasons = append(reasons, "pr_readiness_"+workspaceQueueReasonToken(finding.Kind))
			}
		}
	}
	return reasons
}

func workspaceFailedCheckReasons(status WorkStatusResult) []string {
	reasons := []string{}
	if status.ChecksStatus == "failed" || status.ChecksStatus == "failing" {
		reasons = append(reasons, "checks_failed")
	}
	for _, blocker := range status.Blockers {
		if strings.Contains(strings.ToLower(blocker), "check") {
			reasons = append(reasons, "blocker_checks")
		}
	}
	if status.PRReadiness != nil {
		for _, finding := range status.PRReadiness.Findings {
			if finding.Kind == "checks_failed" || finding.Kind == "checks_failing" {
				reasons = append(reasons, "pr_readiness_checks_failed")
			}
		}
	}
	return reasons
}

func workspaceHumanDecisionReasons(status WorkStatusResult) []string {
	reasons := []string{}
	for _, label := range status.Labels {
		switch strings.ToLower(strings.TrimSpace(label)) {
		case "agent:human", "needs:human", "needs:decision", "type:decision":
			reasons = append(reasons, "label_"+workspaceQueueReasonToken(label))
		}
	}
	if status.TicketReadiness != nil && status.TicketReadiness.NextAction == "ask_human" {
		reasons = append(reasons, "ticket_readiness_ask_human")
	}
	if status.PRReadiness != nil && status.PRReadiness.NextAction == "ask_human" {
		reasons = append(reasons, "pr_readiness_ask_human")
	}
	return reasons
}

func isClosedWorkspaceQueueIssue(status WorkStatusResult) bool {
	return strings.EqualFold(status.State, "closed")
}

func workspaceQueueStatusIs(status WorkStatusResult, want string) bool {
	normalized := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(status.Status)), " ", "-")
	return normalized == strings.ReplaceAll(strings.ToLower(strings.TrimSpace(want)), " ", "-")
}

func hasWorkspaceQueuePR(status WorkStatusResult) bool {
	if status.PullRequest != nil && status.PullRequest.Number > 0 {
		return true
	}
	return status.PRNumber > 0
}

func workspaceQueuePRIsDraft(status WorkStatusResult) bool {
	return status.PullRequest != nil && status.PullRequest.IsDraft
}

func uniqueWorkspaceQueueReasons(reasons []string) []string {
	seen := map[string]struct{}{}
	unique := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		reason = workspaceQueueReasonToken(reason)
		if reason == "" {
			continue
		}
		if _, ok := seen[reason]; ok {
			continue
		}
		seen[reason] = struct{}{}
		unique = append(unique, reason)
	}
	sort.Strings(unique)
	return unique
}

func workspaceQueueReasonToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(":", "_", " ", "_", "-", "_", "/", "_")
	value = replacer.Replace(value)
	value = strings.Trim(value, "_")
	for strings.Contains(value, "__") {
		value = strings.ReplaceAll(value, "__", "_")
	}
	return value
}
