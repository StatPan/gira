package gira

import (
	"strings"
	"testing"
)

func TestBuildTicketViewReportUsesWorkStatus(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[{"number":127,"title":"feat: Work command","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/127","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`),
	}}

	report, err := BuildTicketViewReport(repo, 126, runner)
	if err != nil {
		t.Fatalf("BuildTicketViewReport error: %v", err)
	}
	if report.Command != "ticket view" || report.Status.PRNumber != 127 || !strings.Contains(FormatTicketView(report), "linked pr: #127 OPEN") {
		t.Fatalf("unexpected report: %+v\n%s", report, FormatTicketView(report))
	}
}

func TestTicketLifecycleNextStepPreservesAdoptIssueFlag(t *testing.T) {
	result := WorkStatusResult{
		Repo:       "StatPan/gira",
		Issue:      760,
		State:      "open",
		Status:     "null",
		NextAction: "start_work",
	}
	got := ticketLifecycleNextStep(result)
	want := "gira adopt issues --repo StatPan/gira --issue 760 --label status:ready --apply"
	if got != want {
		t.Fatalf("next step = %q, want %q", got, want)
	}
}

func TestTicketLifecycleNextStepConvertsWorkIssueFlagOnlyForWorkAliases(t *testing.T) {
	result := WorkStatusResult{
		NextStep: "gira work start --repo StatPan/gira --issue 760 --apply",
	}
	got := ticketLifecycleNextStep(result)
	want := "gira ticket start --repo StatPan/gira --ticket 760 --apply"
	if got != want {
		t.Fatalf("next step = %q, want %q", got, want)
	}
}

func TestTicketNoteDryRunRendersContextWithoutComment(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[]`),
	}}

	report, err := BuildTicketNoteReport(TicketNoteInput{Repo: repo, Ticket: 126, Body: "Implemented parser path.", Kind: "progress", Target: "auto", DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildTicketNoteReport error: %v", err)
	}
	if len(report.Targets) != 1 || report.Targets[0].Type != "issue" || report.Targets[0].Number != 126 {
		t.Fatalf("unexpected targets: %+v", report.Targets)
	}
	if !strings.Contains(report.RenderedBody, "## Progress Update") || !strings.Contains(report.RenderedBody, "- Ticket: #126") {
		t.Fatalf("rendered body missing context:\n%s", report.RenderedBody)
	}
	if report.SchemaVersion != TicketNoteReportSchemaVersion || report.Body != "Implemented parser path." {
		t.Fatalf("unexpected note schema/body: %+v", report)
	}
	if report.Approval == nil {
		t.Fatalf("expected approval evidence: %+v", report)
	}
	if report.Approval.SchemaVersion != ApprovalPlanSchemaVersion || report.Approval.CanonicalCommand != "gira ticket note" || report.Approval.OutputSchema != TicketNoteReportSchemaVersion {
		t.Fatalf("unexpected approval evidence: %+v", report.Approval)
	}
	if report.Approval.ApplyCommand != "gira ticket note 126 --repo StatPan/gira --kind progress --target auto --body 'Implemented parser path.' --apply" {
		t.Fatalf("unexpected approval command: %+v", report.Approval)
	}
	if len(report.Approval.PlannedActions) != 1 || report.Approval.PlannedActions[0].Action != "issue:comment" || report.Approval.PostApplyVerification != "gira ticket view 126 --repo StatPan/gira --json" {
		t.Fatalf("unexpected approval planned actions: %+v", report.Approval)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "comment") {
			t.Fatalf("dry-run should not comment, calls=%v", runner.calls)
		}
	}
}

func TestTicketNoteApplyPostsToLinkedPR(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-review"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[{"number":127,"title":"feat: Work command","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/127","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefOid":"head220","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS"}]}]`),
		"gh api repos/StatPan/gira/pulls/127/reviews --paginate --slurp": []byte(`[[{"state":"APPROVED","commit_id":"head220"}]]`),
		"gh pr comment 127 --repo StatPan/gira --body ## Decision\n\nUse ticket-level note templates.\n\nContext:\n- Ticket: #126\n- Status: In review\n- Linked PR: #127\n- Blockers: none\n- Next: merge when policy checks pass\n": []byte(""),
	}}

	report, err := BuildTicketNoteReport(TicketNoteInput{Repo: repo, Ticket: 126, Body: "Use ticket-level note templates.", Kind: "decision", Target: "auto", Apply: true}, runner)
	if err != nil {
		t.Fatalf("BuildTicketNoteReport error: %v", err)
	}
	if len(report.Targets) != 1 || report.Targets[0].Type != "pr" || report.Targets[0].Status != "applied" {
		t.Fatalf("unexpected targets: %+v", report.Targets)
	}
	if report.Approval != nil {
		t.Fatalf("apply output should not include dry-run approval evidence: %+v", report.Approval)
	}
}
