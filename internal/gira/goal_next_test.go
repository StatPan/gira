package gira

import (
	"strings"
	"testing"
)

func TestBuildGoalNextReportSelectsReadyChild(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	status := goalNextTestStatus([]GoalStatusChild{
		{Number: 201, Title: "Done", State: "closed", Status: "Done", Category: "done", NextAction: "done"},
		{Number: 202, Title: "Ready", State: "open", Status: "Ready", Category: "ready", NextAction: "start_work"},
	})

	report := BuildGoalNextReportFromStatus(repo, status)
	if report.SchemaVersion != GoalNextSchemaVersion || report.SelectedTicket == nil || report.SelectedTicket.Number != 202 {
		t.Fatalf("unexpected ready selection: %+v", report)
	}
	if report.NextAction != "start_child" || !strings.Contains(report.NextStep, "gira ticket start") {
		t.Fatalf("unexpected next command: %+v", report)
	}
}

func TestBuildGoalNextReportPreservesCrossRepoChild(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "backlog"}
	status := goalNextTestStatus([]GoalStatusChild{
		{Repo: "StatPan/gira", Number: 202, Title: "Ready", State: "open", Status: "Ready", Category: "ready", NextAction: "start_work"},
	})

	report := BuildGoalNextReportFromStatus(repo, status)
	if report.SelectedTicket == nil || report.SelectedTicket.Repo != "StatPan/gira" {
		t.Fatalf("unexpected selected ticket: %+v", report.SelectedTicket)
	}
	if !strings.Contains(report.NextStep, "--repo StatPan/gira") || strings.Contains(report.NextStep, "--repo StatPan/backlog") {
		t.Fatalf("cross-repo next step used wrong repo: %q", report.NextStep)
	}
}

func TestBuildGoalNextReportSelectsOnlyTypedOrNativeChildrenWithProvenance(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := onboardFakeRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/100": `{"number":100,"title":"Goal","state":"open","body":"## Goal\nShip\n\nGoal planning notes mention #203, but it is not a child.\n<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=202 -->","labels":[{"name":"type:epic"},{"name":"status:ready"}]}`,
		"gh api repos/StatPan/gira/issues/100/sub_issues -X GET -H Accept: application/vnd.github+json -H X-GitHub-Api-Version: 2026-03-10 -f per_page=100": `[{
"number":201,"title":"Historical native child","state":"closed"}]`,
		"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[{"body":"Grounding-gap parent: #203"}]}`,
		"gh api repos/StatPan/gira/issues/201":                  `{"number":201,"title":"Historical native child","state":"closed","body":"## Goal\nDone","labels":[{"name":"type:task"},{"name":"status:done"}]}`,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 201 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": `[]`,
		"gh api repos/StatPan/gira/issues/202": `{"number":202,"title":"Typed ready child","state":"open","body":"## Goal\nReady","labels":[{"name":"type:task"},{"name":"status:ready"}]}`,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 202 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": `[]`,
	}}

	report, err := BuildGoalNextReport(GoalNextInput{Repo: repo, Goal: 100}, runner)
	if err != nil {
		t.Fatalf("BuildGoalNextReport error: %v", err)
	}
	if report.SelectedTicket == nil || report.SelectedTicket.Number != 202 || report.SelectedTicket.RelationSource != GoalChildRelationSourceGiraGoalChildLink {
		t.Fatalf("expected typed ready child selection with provenance: %+v", report)
	}
	if len(report.SkippedCandidates) != 1 || report.SkippedCandidates[0].Number != 201 || report.SkippedCandidates[0].RelationSource != GoalChildRelationSourceGitHubSubIssue {
		t.Fatalf("historical native child/provenance missing from skipped candidates: %+v", report.SkippedCandidates)
	}
}

func TestBuildGoalNextReportStopsForBlockedChild(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	status := goalNextTestStatus([]GoalStatusChild{
		{Number: 201, Title: "Blocked", State: "open", Status: "Blocked", Category: "blocked", NextAction: "resolve_blockers"},
		{Number: 202, Title: "Ready", State: "open", Status: "Ready", Category: "ready", NextAction: "start_work"},
	})
	status.Counts["blocked"] = 1
	status.Blockers = []string{"child_201:blocked"}

	report := BuildGoalNextReportFromStatus(repo, status)
	if report.SelectedTicket != nil || report.NextAction != "resolve_blockers" || !containsString(report.StopReasons, "goal_blockers_present") {
		t.Fatalf("unexpected blocked report: %+v", report)
	}
}

func TestBuildGoalNextReportSelectsInReviewBeforeReady(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	status := goalNextTestStatus([]GoalStatusChild{
		{Number: 201, Title: "Review", State: "open", Status: "In review", Category: "in_review", NextAction: "merge_when_policy_allows", NextStep: "gira work finish --repo StatPan/gira --issue 201 --dry-run"},
		{Number: 202, Title: "Ready", State: "open", Status: "Ready", Category: "ready", NextAction: "start_work"},
	})
	status.Counts["in_review"] = 1

	report := BuildGoalNextReportFromStatus(repo, status)
	if report.SelectedTicket == nil || report.SelectedTicket.Number != 201 || report.NextAction != "review_child" {
		t.Fatalf("unexpected in-review selection: %+v", report)
	}
	if !strings.Contains(report.NextStep, "gira ticket finish") || strings.Contains(report.NextStep, "--issue") {
		t.Fatalf("next step was not normalized: %q", report.NextStep)
	}
	if len(report.SkippedCandidates) != 1 || report.SkippedCandidates[0].Reason != "wait_for_selected_child" {
		t.Fatalf("unexpected skipped candidates: %+v", report.SkippedCandidates)
	}
}

func TestBuildGoalNextReportStopsForFailedCheckChild(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	status := goalNextTestStatus([]GoalStatusChild{
		{Number: 201, Title: "Failing", State: "open", Status: "In review", Category: "in_review", Blockers: []string{"checks"}, NextAction: "wait_for_checks"},
		{Number: 202, Title: "Ready", State: "open", Status: "Ready", Category: "ready", NextAction: "start_work"},
	})
	status.Blockers = []string{"child_201:checks"}

	report := BuildGoalNextReportFromStatus(repo, status)
	if report.SelectedTicket != nil || report.NextAction != "resolve_blockers" || !containsString(report.Blockers, "child_201:checks") {
		t.Fatalf("unexpected failed-check report: %+v", report)
	}
}

func TestBuildGoalNextReportNoEligibleChild(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	status := goalNextTestStatus([]GoalStatusChild{
		{Number: 201, Title: "Human approval", State: "open", Status: "Ready", Category: "ready", Labels: []string{"agent:human"}, NextAction: "start_work"},
	})

	report := BuildGoalNextReportFromStatus(repo, status)
	if report.SelectedTicket != nil || report.NextAction != "inspect_goal" || !containsString(report.StopReasons, "no_eligible_child_ticket") {
		t.Fatalf("unexpected no-eligible report: %+v", report)
	}
	if len(report.SkippedCandidates) != 1 || report.SkippedCandidates[0].Reason != "human_approval_required" {
		t.Fatalf("unexpected skipped candidates: %+v", report.SkippedCandidates)
	}
}

func TestBuildGoalNextReportNoRemainingWorkFinishesGoal(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	status := goalNextTestStatus([]GoalStatusChild{
		{Number: 201, Title: "Done", State: "closed", Status: "Done", Category: "done", NextAction: "done"},
	})
	status.RemainingAutonomousWork = 0

	report := BuildGoalNextReportFromStatus(repo, status)
	if report.SelectedTicket != nil || report.NextAction != "finish_goal" || !containsString(report.StopReasons, "no_remaining_child_work") {
		t.Fatalf("unexpected all-done report: %+v", report)
	}
}

func TestBuildGoalNextReportNoRemainingWorkWithHandoffStopsForHumanReview(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	status := goalNextTestStatus([]GoalStatusChild{
		{Number: 201, Title: "Done", State: "closed", Status: "Done", Category: "done", NextAction: "done"},
	})
	status.RemainingAutonomousWork = 0
	status.HandoffReceiptPresent = true

	report := BuildGoalNextReportFromStatus(repo, status)
	if report.SelectedTicket != nil || report.NextAction != "human_review" || !containsString(report.StopReasons, "human_review_handoff_present") {
		t.Fatalf("unexpected handoff report: %+v", report)
	}
	if !strings.Contains(report.NextStep, "goal-finish-receipt/v1") {
		t.Fatalf("next step should point to handoff receipt: %q", report.NextStep)
	}
}

func TestBuildGoalNextReportClosedDoneGoalStopsAsDone(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	status := goalNextTestStatus([]GoalStatusChild{
		{Number: 201, Title: "Done", State: "closed", Status: "Done", Category: "done", NextAction: "done"},
	})
	status.Goal.State = "closed"
	status.Goal.Status = "Done"
	status.RemainingAutonomousWork = 0
	status.HandoffReceiptPresent = true

	report := BuildGoalNextReportFromStatus(repo, status)
	if report.SelectedTicket != nil || report.NextAction != "done" || report.NextStep != "goal is done" || !containsString(report.StopReasons, "goal_done") {
		t.Fatalf("closed done goal should stop as done: %+v", report)
	}
}

func goalNextTestStatus(children []GoalStatusChild) GoalStatusReport {
	counts := map[string]int{"total": len(children)}
	remaining := 0
	for _, child := range children {
		counts[child.Category]++
		if child.Category != "done" && child.Category != "closed_other" {
			remaining++
		}
	}
	return GoalStatusReport{
		Command:                 "goal status",
		SchemaVersion:           GoalStatusSchemaVersion,
		Repo:                    "StatPan/gira",
		Goal:                    GoalStatusIssue{Number: 100, Title: "Goal", State: "open", Status: "Ready"},
		Children:                children,
		Counts:                  counts,
		NextAction:              "start_next_child",
		NextStep:                "gira goal next --repo StatPan/gira --goal 100 --json",
		RemainingAutonomousWork: remaining,
	}
}
