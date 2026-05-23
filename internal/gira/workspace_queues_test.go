package gira

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildWorkspaceQueuesClassifiesCoreQueues(t *testing.T) {
	report := BuildWorkspaceQueues(WorkspaceSummary{Name: "personal", Owner: "StatPan"}, []WorkStatusResult{
		{
			Repo:            "StatPan/gira",
			Issue:           10,
			Title:           "Ready work",
			State:           "open",
			Status:          "Ready",
			Labels:          []string{"type:task", "status:ready"},
			TicketReadiness: &TicketReadinessReport{SchemaVersion: TicketReadinessSchemaVersion, Readiness: "ready", NextAction: "start_ticket"},
		},
		{
			Repo:         "StatPan/gira",
			Issue:        11,
			Title:        "Needs review",
			State:        "open",
			Status:       "In review",
			ReviewStatus: "missing",
			PullRequest:  &TicketStatusPullRequest{Available: true, Number: 111, State: "OPEN", ReviewDecision: "REVIEW_REQUIRED"},
			PRReadiness:  &PRReadinessReport{SchemaVersion: PRReadinessSchemaVersion, PullRequest: 111, Readiness: "needs_review", NextAction: "request_review"},
		},
		{
			Repo:         "StatPan/gira",
			Issue:        12,
			Title:        "Finishable",
			State:        "open",
			Status:       "In review",
			NextAction:   "merge_when_policy_allows",
			ChecksStatus: "passed",
			ReviewStatus: "approved",
			PullRequest:  &TicketStatusPullRequest{Available: true, Number: 112, State: "OPEN", ReviewDecision: "APPROVED"},
			PRReadiness:  &PRReadinessReport{SchemaVersion: PRReadinessSchemaVersion, PullRequest: 112, Readiness: "ready_for_finish", NextAction: "finish_ticket"},
			Evidence:     &TicketStatusEvidence{FinishReady: true},
		},
		{
			Repo:     "StatPan/gira",
			Issue:    13,
			Title:    "Blocked",
			State:    "open",
			Status:   "Blocked",
			Blockers: []string{"missing_linked_pr"},
			TicketReadiness: &TicketReadinessReport{
				SchemaVersion: TicketReadinessSchemaVersion,
				Readiness:     "needs_refinement",
				Findings:      []TicketReadinessFinding{{Severity: "error", Kind: "missing_goal"}},
				NextAction:    "refine_ticket",
			},
		},
		{
			Repo:         "StatPan/gira",
			Issue:        14,
			Title:        "Failed checks",
			State:        "open",
			Status:       "In review",
			ChecksStatus: "failed",
			Blockers:     []string{"checks"},
			PullRequest:  &TicketStatusPullRequest{Available: true, Number: 114, State: "OPEN"},
			PRReadiness: &PRReadinessReport{
				SchemaVersion: PRReadinessSchemaVersion,
				PullRequest:   114,
				Readiness:     "needs_revision",
				Findings:      []PRReadinessFinding{{Severity: "error", Kind: "checks_failed"}},
				NextAction:    "fix_checks",
			},
		},
		{
			Repo:   "StatPan/gira",
			Issue:  15,
			Title:  "Human decision",
			State:  "open",
			Status: "Ready",
			Labels: []string{"type:decision", "needs:human"},
		},
	})

	if report.SchemaVersion != WorkspaceQueuesSchemaVersion {
		t.Fatalf("schema version = %q", report.SchemaVersion)
	}
	if report.Counts.AgentReady != 1 || report.Counts.ReviewNeeded != 1 || report.Counts.FinishReady != 1 || report.Counts.Blocked != 2 || report.Counts.FailedCheck != 1 || report.Counts.HumanDecision != 1 {
		t.Fatalf("unexpected counts: %+v", report.Counts)
	}
	if got := report.Queues.AgentReady[0]; got.Issue != 10 || got.NextSafeCommand != "gira ticket start --repo StatPan/gira --ticket 10 --apply" || !hasWorkspaceQueueReason(got, "ticket_readiness_ready") {
		t.Fatalf("agent_ready item = %+v", got)
	}
	if got := report.Queues.ReviewNeeded[0]; got.Issue != 11 || got.PullRequest == nil || got.PullRequest.Number != 111 || !hasWorkspaceQueueReason(got, "review_required") {
		t.Fatalf("review_needed item = %+v", got)
	}
	if got := report.Queues.FinishReady[0]; got.Issue != 12 || got.NextSafeCommand != "gira ticket finish --repo StatPan/gira --ticket 12 --dry-run" || got.Evidence.PRReadiness != "ready_for_finish" {
		t.Fatalf("finish_ready item = %+v", got)
	}
	if got := report.Queues.FailedCheck[0]; got.Issue != 14 || !hasWorkspaceQueueReason(got, "checks_failed") || !hasWorkspaceQueueReason(got, "blocker_checks") {
		t.Fatalf("failed_check item = %+v", got)
	}
	if got := report.Queues.HumanDecision[0]; got.Issue != 15 || !strings.Contains(got.NextSafeCommand, "ticket handoff") || !hasWorkspaceQueueReason(got, "label_needs_human") {
		t.Fatalf("human_decision item = %+v", got)
	}
}

func TestBuildWorkspaceQueuesSkipsClosedAndKeepsPrivacyBoundary(t *testing.T) {
	report := BuildWorkspaceQueues(WorkspaceSummary{Name: "personal", Owner: "StatPan"}, []WorkStatusResult{
		{Repo: "StatPan/gira", Issue: 20, Title: "Closed", State: "closed", Status: "Ready"},
	})
	if report.Counts != (WorkspaceQueueCounts{}) {
		t.Fatalf("closed work should not enter queues: %+v", report.Counts)
	}
	if report.PrivacyBoundary.Scope != "work_item_state_only" || !stringSliceContains(report.PrivacyBoundary.Prohibited, "personal_productivity_ranking") {
		t.Fatalf("privacy boundary = %+v", report.PrivacyBoundary)
	}
	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if strings.Contains(string(payload), `"agent_ready":null`) {
		t.Fatalf("empty queues should encode as arrays: %s", payload)
	}
	for _, forbiddenField := range []string{`"score"`, `"rank"`, `"velocity"`, `"assignee_score"`, `"agent_score"`} {
		if strings.Contains(string(payload), forbiddenField) {
			t.Fatalf("queue contract should not expose ranking field %q: %s", forbiddenField, payload)
		}
	}
}

func hasWorkspaceQueueReason(item WorkspaceQueueItem, reason string) bool {
	return stringSliceContains(item.ReasonCodes, reason)
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
