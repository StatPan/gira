package gira

import (
	"fmt"
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
	if !goalFinishActionStatus(report.Actions, "goal:comment", "planned") || !goalFinishActionStatus(report.Actions, "goal:normalize-status", "planned") || !goalFinishActionStatus(report.Actions, "goal:close", "planned") {
		t.Fatalf("expected done terminal planned actions: %+v", report.Actions)
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

func TestBuildGoalFinishReportAcceptsPlanningOnlyChildReceipt(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	childComments := `{"comments":[{"body":"## Finish Receipt\n\n- Schema: ticket-finish-receipt/v1\n- Evidence: planning_done\n- PR: not_required_planning_only\n- Checks: not_required_no_code_change\n- Decision: synthetic PR not created for planning-only work"}]}`
	runner := goalFinishRunner(childComments, `[]`, goalFinishChildIssue("closed", "status:done"))

	report, err := BuildGoalFinishReport(GoalFinishInput{Repo: repo, Goal: 100, DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildGoalFinishReport error: %v", err)
	}
	if !report.Readiness.Ready || report.Readiness.TerminalRecommendation != "done" {
		t.Fatalf("expected planning-only child to be ready for done: %+v", report.Readiness)
	}
	for _, blocker := range []string{"child_101_missing_pr", "child_101_checks_missing", "child_101_missing_finish_receipt"} {
		if containsString(report.Readiness.Blockers, blocker) {
			t.Fatalf("planning-only receipt should suppress %s: %+v", blocker, report.Readiness.Blockers)
		}
	}
	if len(report.Readiness.Children) != 1 {
		t.Fatalf("expected one child evidence item: %+v", report.Readiness.Children)
	}
	child := report.Readiness.Children[0]
	if !child.PlanningOnly || !child.PRNotRequired || !child.ChecksNotRequired {
		t.Fatalf("planning-only exception not exposed in child evidence: %+v", child)
	}
	for _, want := range []string{"finish_receipt", "planning_only_completion", "pr_not_required", "checks_not_required"} {
		if !containsString(child.Evidence, want) {
			t.Fatalf("child evidence missing %s: %+v", want, child.Evidence)
		}
	}
}

func TestBuildGoalFinishReportGenericReceiptStillBlocksMissingPRChecks(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalFinishRunner(`{"comments":[{"body":"## Finish Receipt\n\n- Schema: finish-receipt/v1\n- Evidence: done"}]}`, `[]`, goalFinishChildIssue("closed", "status:done"))

	report, err := BuildGoalFinishReport(GoalFinishInput{Repo: repo, Goal: 100, DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildGoalFinishReport error: %v", err)
	}
	for _, want := range []string{"child_101_missing_pr", "child_101_checks_missing"} {
		if !containsString(report.Readiness.Blockers, want) {
			t.Fatalf("generic receipt should still require %s: %+v", want, report.Readiness.Blockers)
		}
	}
	if containsString(report.Readiness.Blockers, "child_101_missing_finish_receipt") {
		t.Fatalf("generic finish receipt should satisfy receipt presence: %+v", report.Readiness.Blockers)
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

func TestBuildGoalFinishReportHumanReviewDryRunPlansHandoff(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalFinishRunner(`{"comments":[]}`, `[]`, goalFinishChildIssue("closed", "status:done"))

	report, err := BuildGoalFinishReport(GoalFinishInput{Repo: repo, Goal: 100, DryRun: true, Terminal: "human_review"}, runner)
	if err != nil {
		t.Fatalf("BuildGoalFinishReport error: %v", err)
	}
	if len(report.Actions) != 1 || report.Actions[0].Action != "goal:comment" || report.Actions[0].Status != "planned" {
		t.Fatalf("unexpected actions: %+v", report.Actions)
	}
	if !strings.Contains(report.Receipt.RenderedBody, GoalFinishReceiptSchemaVersion) || !strings.Contains(report.Receipt.RenderedBody, "child_101_missing_finish_receipt") {
		t.Fatalf("receipt does not preserve schema/blockers:\n%s", report.Receipt.RenderedBody)
	}
}

func TestBuildGoalFinishReportHumanReviewDryRunSkipsExistingHandoff(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalFinishRunnerWithGoalComments(`{"comments":[{"body":"## Goal Finish Receipt\n\n- Schema: goal-finish-receipt/v1"}]}`, `{"comments":[]}`, `[]`, goalFinishChildIssue("closed", "status:done"))

	report, err := BuildGoalFinishReport(GoalFinishInput{Repo: repo, Goal: 100, DryRun: true, Terminal: "human_review"}, runner)
	if err != nil {
		t.Fatalf("BuildGoalFinishReport error: %v", err)
	}
	if !report.Readiness.HandoffReceiptPresent {
		t.Fatalf("expected handoff receipt present: %+v", report.Readiness)
	}
	if len(report.Actions) != 1 || report.Actions[0].Status != "skipped" || !strings.Contains(report.Actions[0].Detail, "already exists") {
		t.Fatalf("unexpected actions: %+v", report.Actions)
	}
}

func TestBuildGoalFinishReportHumanReviewApplyPostsReceipt(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalFinishApplyRunner{responses: goalFinishRunner(`{"comments":[]}`, `[]`, goalFinishChildIssue("closed", "status:done")).responses}

	report, err := BuildGoalFinishReport(GoalFinishInput{Repo: repo, Goal: 100, Apply: true, Terminal: "human_review"}, &runner)
	if err != nil {
		t.Fatalf("BuildGoalFinishReport error: %v", err)
	}
	if len(runner.comments) != 1 || !strings.Contains(runner.comments[0], GoalFinishReceiptSchemaVersion) || !strings.Contains(runner.comments[0], "child_101_missing_pr") {
		t.Fatalf("unexpected comments: %+v", runner.comments)
	}
	if !report.Apply || len(report.Actions) != 1 || report.Actions[0].Status != "applied" || report.NextStep != "human review handoff receipt posted" {
		t.Fatalf("unexpected apply report: %+v", report)
	}
}

func TestBuildGoalFinishReportHumanReviewApplySkipsExistingReceipt(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	responses := goalFinishRunnerWithGoalComments(`{"comments":[{"body":"## Goal Finish Receipt\n\n- Schema: goal-finish-receipt/v1"}]}`, `{"comments":[]}`, `[]`, goalFinishChildIssue("closed", "status:done")).responses
	runner := goalFinishApplyRunner{responses: responses}

	report, err := BuildGoalFinishReport(GoalFinishInput{Repo: repo, Goal: 100, Apply: true, Terminal: "human_review"}, &runner)
	if err != nil {
		t.Fatalf("BuildGoalFinishReport error: %v", err)
	}
	if len(runner.comments) != 0 {
		t.Fatalf("apply should not duplicate existing receipt: %+v", runner.comments)
	}
	if !report.Readiness.HandoffReceiptPresent || len(report.Actions) != 1 || report.Actions[0].Status != "skipped" || report.NextStep != "human review handoff receipt already present" {
		t.Fatalf("unexpected no-op report: %+v", report)
	}
}

func TestBuildGoalFinishReportDoneDryRunPlansReceiptLabelsAndClose(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalFinishRunner(`{"comments":[{"body":"## Finish Receipt\nDone"}]}`, goalFinishMergedPR("SUCCESS"), goalFinishChildIssue("closed", "status:done"))

	report, err := BuildGoalFinishReport(GoalFinishInput{Repo: repo, Goal: 100, DryRun: true, Terminal: "done"}, runner)
	if err != nil {
		t.Fatalf("BuildGoalFinishReport error: %v", err)
	}
	if !report.Readiness.Ready || report.NextAction != "finish_goal" {
		t.Fatalf("unexpected readiness: %+v", report)
	}
	if !goalFinishActionStatus(report.Actions, "goal:comment", "planned") || !goalFinishActionStatus(report.Actions, "goal:normalize-status", "planned") || !goalFinishActionStatus(report.Actions, "goal:close", "planned") {
		t.Fatalf("unexpected actions: %+v", report.Actions)
	}
	if detail := goalFinishActionDetail(report.Actions, "goal:normalize-status"); !strings.Contains(detail, "add=status:done") || !strings.Contains(detail, "remove=status:ready") {
		t.Fatalf("unexpected normalize detail: %q", detail)
	}
	if !strings.Contains(report.NextStep, "--terminal done --apply") {
		t.Fatalf("next step should include explicit done apply: %q", report.NextStep)
	}
}

func TestBuildGoalFinishReportDoneApplyPostsReceiptNormalizesAndCloses(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalFinishApplyRunner{responses: goalFinishRunner(`{"comments":[{"body":"## Finish Receipt\nDone"}]}`, goalFinishMergedPR("SUCCESS"), goalFinishChildIssue("closed", "status:done")).responses}

	report, err := BuildGoalFinishReport(GoalFinishInput{Repo: repo, Goal: 100, Apply: true, Terminal: "done"}, &runner)
	if err != nil {
		t.Fatalf("BuildGoalFinishReport error: %v", err)
	}
	if len(runner.comments) != 1 || !strings.Contains(runner.comments[0], "Terminal recommendation: done") {
		t.Fatalf("unexpected comments: %+v", runner.comments)
	}
	if !containsCall(runner.calls, "gh issue edit 100 --repo StatPan/gira --add-label status:done --remove-label status:ready") {
		t.Fatalf("missing label normalization call: %+v", runner.calls)
	}
	if !containsCall(runner.calls, "gh issue close 100 --repo StatPan/gira --comment Closed by gira goal finish") {
		t.Fatalf("missing close call: %+v", runner.calls)
	}
	for _, action := range []string{"goal:comment", "goal:normalize-status", "goal:close"} {
		if !goalFinishActionStatus(report.Actions, action, "applied") {
			t.Fatalf("expected %s applied: %+v", action, report.Actions)
		}
	}
	if report.NextAction != "done" || report.NextStep != "goal is done" {
		t.Fatalf("unexpected next state: %+v", report)
	}
}

func TestBuildGoalFinishReportDoneApplyRejectsBlockers(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalFinishApplyRunner{responses: goalFinishRunner(`{"comments":[]}`, `[]`, goalFinishChildIssue("open", "status:ready")).responses}

	_, err := BuildGoalFinishReport(GoalFinishInput{Repo: repo, Goal: 100, Apply: true, Terminal: "done"}, &runner)
	if err == nil || !strings.Contains(err.Error(), "requires ready=true with no blockers") {
		t.Fatalf("error = %v, want readiness rejection", err)
	}
	if len(runner.comments) != 0 || len(runner.calls) != 0 {
		t.Fatalf("apply should not mutate on rejection: comments=%+v calls=%+v", runner.comments, runner.calls)
	}
}

func TestBuildGoalFinishReportApplyRejectsImplicitTerminal(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalFinishApplyRunner{responses: goalFinishRunner(`{"comments":[{"body":"## Finish Receipt\nDone"}]}`, goalFinishMergedPR("SUCCESS"), goalFinishChildIssue("closed", "status:done")).responses}

	_, err := BuildGoalFinishReport(GoalFinishInput{Repo: repo, Goal: 100, Apply: true}, &runner)
	if err == nil || !strings.Contains(err.Error(), "explicit --terminal done") {
		t.Fatalf("error = %v, want explicit terminal rejection", err)
	}
	if len(runner.comments) != 0 || len(runner.calls) != 0 {
		t.Fatalf("apply should not mutate on rejection: comments=%+v calls=%+v", runner.comments, runner.calls)
	}
}

func goalFinishRunner(childComments string, childPRs string, childIssue string) onboardFakeRunner {
	return goalFinishRunnerWithGoalComments(`{"comments":[]}`, childComments, childPRs, childIssue)
}

func goalFinishRunnerWithGoalComments(goalComments string, childComments string, childPRs string, childIssue string) onboardFakeRunner {
	return onboardFakeRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/100": `{"number":100,"title":"Goal","state":"open","body":"## Goal\nShip\n\n## Scope\nGoal finish\n\n## Goal Plan\n- finish","labels":[{"name":"type:epic"},{"name":"status:ready"}]}`,
		`gh issue list --repo StatPan/gira --state all --search repo:StatPan/gira is:issue "Parent: #100" --json number,title,state,url --limit 100`: `[{"number":101}]`,
		"gh issue view 100 --repo StatPan/gira --json comments": goalComments,
		"gh api repos/StatPan/gira/issues/101":                  childIssue,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 101 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": childPRs,
		"gh issue view 101 --repo StatPan/gira --json comments":      childComments,
		"gh label list --repo StatPan/gira --json name --limit 1000": `[{"name":"status:done"}]`,
	}}
}

type goalFinishApplyRunner struct {
	responses map[string]string
	comments  []string
	calls     []string
}

func (r *goalFinishApplyRunner) Run(name string, args ...string) ([]byte, error) {
	if name == "gh" && len(args) == 7 && args[0] == "issue" && args[1] == "comment" && args[2] == "100" && args[3] == "--repo" && args[4] == "StatPan/gira" && args[5] == "--body" {
		r.comments = append(r.comments, args[6])
		return []byte("https://github.com/StatPan/gira/issues/100#issuecomment-1\n"), nil
	}
	if name == "gh" && len(args) >= 2 && args[0] == "issue" && (args[1] == "edit" || args[1] == "close") {
		r.calls = append(r.calls, strings.TrimSpace(name+" "+strings.Join(args, " ")))
		return []byte("{}"), nil
	}
	key := strings.TrimSpace(name + " " + strings.Join(args, " "))
	response, ok := r.responses[key]
	if !ok {
		return nil, fmt.Errorf("unexpected command: %s", key)
	}
	return []byte(response), nil
}

func goalFinishChildIssue(state string, statusLabel string) string {
	return `{"number":101,"title":"Child","state":"` + state + `","body":"## Goal\nChild\n\n## Scope\nWork\n\n## Acceptance Criteria\n- done","labels":[{"name":"type:task"},{"name":"` + statusLabel + `"}]}`
}

func goalFinishMergedPR(conclusion string) string {
	return `[{"number":202,"title":"Child PR","body":"Closes #101","state":"MERGED","url":"https://github.com/StatPan/gira/pull/202","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-101-child","baseRefName":"main","headRefOid":"head220","statusCheckRollup":[{"conclusion":"` + conclusion + `","status":"COMPLETED"}]}]`
}

func goalFinishActionStatus(actions []GoalFinishAction, action string, status string) bool {
	for _, item := range actions {
		if item.Action == action && item.Status == status {
			return true
		}
	}
	return false
}

func goalFinishActionDetail(actions []GoalFinishAction, action string) string {
	for _, item := range actions {
		if item.Action == action {
			return item.Detail
		}
	}
	return ""
}
