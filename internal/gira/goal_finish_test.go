package gira

import (
	"strings"
	"testing"
)

func TestBuildGoalFinishReportAllDoneReady(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalFinishRunner(`{"comments":[{"body":"## Finish Receipt\nDone"}]}`, goalFinishMergedPR("SUCCESS"), goalFinishChildIssue("closed", "status:done"))

	report, err := BuildGoalFinishReport(GoalFinishInput{Repo: repo, Goal: 100, DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildGoalFinishReport error: %v", err)
	}
	if report.Readiness.SchemaVersion != GoalFinishReadinessSchemaVersion || !report.Readiness.Ready || report.Readiness.TerminalRecommendation != "done" {
		t.Fatalf("unexpected all-done readiness: %+v", report.Readiness)
	}
	if report.Receipt.SchemaVersion != GoalFinishReceiptSchemaVersion || !strings.Contains(report.Receipt.RenderedBody, "## Goal Finish Receipt") {
		t.Fatalf("unexpected receipt: %+v", report.Receipt)
	}
}

func TestBuildGoalFinishReportBlocksOpenChild(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalFinishRunner(`{"comments":[]}`, `[]`, goalFinishChildIssue("open", "status:ready"))

	report, err := BuildGoalFinishReport(GoalFinishInput{Repo: repo, Goal: 100, DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildGoalFinishReport error: %v", err)
	}
	if report.Readiness.Ready || !containsString(report.Readiness.Blockers, "child_101_open") || !containsString(report.Readiness.Blockers, "child_101_missing_pr") {
		t.Fatalf("expected open child blockers: %+v", report.Readiness)
	}
}

func TestBuildGoalFinishReportBlocksFailedChecks(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalFinishRunner(`{"comments":[{"body":"finish-receipt/v1"}]}`, goalFinishMergedPR("FAILURE"), goalFinishChildIssue("closed", "status:done"))

	report, err := BuildGoalFinishReport(GoalFinishInput{Repo: repo, Goal: 100, DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildGoalFinishReport error: %v", err)
	}
	if report.Readiness.Ready || !containsString(report.Readiness.Blockers, "child_101_checks_failed") {
		t.Fatalf("expected failed checks blocker: %+v", report.Readiness)
	}
}

func TestBuildGoalFinishReportBlocksMissingEvidence(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalFinishRunner(`{"comments":[]}`, `[]`, goalFinishChildIssue("closed", "status:done"))

	report, err := BuildGoalFinishReport(GoalFinishInput{Repo: repo, Goal: 100, DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildGoalFinishReport error: %v", err)
	}
	for _, want := range []string{"child_101_missing_pr", "child_101_checks_missing", "child_101_missing_finish_receipt"} {
		if !containsString(report.Readiness.Blockers, want) {
			t.Fatalf("blockers missing %s: %+v", want, report.Readiness.Blockers)
		}
	}
}

func TestBuildGoalFinishReportHumanReviewTerminalState(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalFinishRunner(`{"comments":[{"body":"## Finish Receipt\nDone"}]}`, goalFinishMergedPR("SUCCESS"), goalFinishChildIssue("closed", "status:done"))

	report, err := BuildGoalFinishReport(GoalFinishInput{Repo: repo, Goal: 100, DryRun: true, Terminal: "human_review"}, runner)
	if err != nil {
		t.Fatalf("BuildGoalFinishReport error: %v", err)
	}
	if report.Readiness.Ready || report.Readiness.TerminalRecommendation != "human_review" || report.NextAction != "human_review" {
		t.Fatalf("unexpected human-review report: %+v", report)
	}
}

func goalFinishRunner(childComments string, childPRs string, childIssue string) onboardFakeRunner {
	return onboardFakeRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/100": `{"number":100,"title":"Goal","state":"open","body":"## Goal\nShip\n\n## Scope\nGoal finish\n\n## Goal Plan\n- finish","labels":[{"name":"type:epic"},{"name":"status:ready"}]}`,
		`gh issue list --repo StatPan/gira --state all --search repo:StatPan/gira is:issue "Parent: #100" --json number,title,state,url --limit 100`: `[{"number":101}]`,
		"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[]}`,
		"gh api repos/StatPan/gira/issues/101":                  childIssue,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 101 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": childPRs,
		"gh issue view 101 --repo StatPan/gira --json comments": childComments,
	}}
}

func goalFinishChildIssue(state string, statusLabel string) string {
	return `{"number":101,"title":"Child","state":"` + state + `","body":"## Goal\nChild\n\n## Scope\nWork\n\n## Acceptance Criteria\n- done","labels":[{"name":"type:task"},{"name":"` + statusLabel + `"}]}`
}

func goalFinishMergedPR(conclusion string) string {
	return `[{"number":202,"title":"Child PR","body":"Closes #101","state":"MERGED","url":"https://github.com/StatPan/gira/pull/202","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-101-child","baseRefName":"main","statusCheckRollup":[{"conclusion":"` + conclusion + `","status":"COMPLETED"}]}]`
}
