package gira

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildGoalPlanCompactReportOmitsBodiesAndIsDeterministic(t *testing.T) {
	report := GoalPlanReport{Repo: "StatPan/gira", Goal: GoalStatusIssue{Number: 100, Title: "Goal"}, ProposedTickets: []GoalPlanTicket{{Title: "[Task] Add API", TargetRepo: "StatPan/gira", Type: "task", Priority: "p1", Labels: []string{"type:task", "status:ready"}, Scope: "CLI", Goal: "Add API", Acceptance: []string{"tested"}, ExpectedEvidence: []string{"go test"}, Body: "secret rendered issue body"}}}
	first := BuildGoalPlanCompactReport(report, "dry_run", "")
	second := BuildGoalPlanCompactReport(report, "dry_run", "")
	if first.PlanID == "" || first.PlanID != second.PlanID || len(first.Proposals) != 1 || first.Proposals[0].PayloadSHA256 == "" {
		t.Fatalf("unexpected compact plan: %+v", first)
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "secret rendered issue body") || strings.Contains(string(encoded), "ticket_readiness") {
		t.Fatalf("compact output leaked verbose payload: %s", encoded)
	}
}

func TestBuildGoalPlanCompactApplyReceiptDoesNotRepeatProposals(t *testing.T) {
	report := GoalPlanReport{Repo: "StatPan/gira", Goal: GoalStatusIssue{Number: 100}, CreatedChildren: []GoalPlanChild{{Number: 101, Title: "Created"}}, Actions: []GoalPlanAction{{Action: "child_ticket:create", Status: "applied"}}}
	compact := BuildGoalPlanCompactReport(report, "apply", BuildGoalPlanCompactReport(report, "dry_run", "").PlanID)
	if compact.Receipt == nil || len(compact.Proposals) != 0 || !compact.Matched {
		t.Fatalf("unexpected compact receipt: %+v", compact)
	}
}

func TestBuildGoalPlanCompactMismatchDoesNotClaimReceipt(t *testing.T) {
	report := GoalPlanReport{Repo: "StatPan/gira", Goal: GoalStatusIssue{Number: 100}, CreatedChildren: []GoalPlanChild{{Number: 101, Title: "Created"}}}
	compact := BuildGoalPlanCompactReport(report, "apply", "gpp-stale")
	if compact.Matched || compact.Receipt != nil || len(compact.Proposals) != 0 {
		t.Fatalf("mismatched compact apply must not claim mutation: %+v", compact)
	}
}
