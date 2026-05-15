package gira

import (
	"strings"
	"testing"
)

func TestBuildTicketViewReportUsesWorkStatus(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[{"number":127,"title":"feat: Work command","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/127","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`),
	}}

	report, err := BuildTicketViewReport(repo, 126, runner)
	if err != nil {
		t.Fatalf("BuildTicketViewReport error: %v", err)
	}
	if report.Command != "ticket view" || report.Status.PRNumber != 127 || !strings.Contains(FormatTicketView(report), "linked pr: #127 OPEN") {
		t.Fatalf("unexpected report: %+v\n%s", report, FormatTicketView(report))
	}
}

func TestTicketNoteDryRunRendersContextWithoutComment(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[]`),
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
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20":             []byte(`[{"number":127,"title":"feat: Work command","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/127","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`),
		"gh pr comment 127 --repo StatPan/gira --body ## Decision\n\nUse ticket-level note templates.\n\nContext:\n- Ticket: #126\n- Status: In review\n- Linked PR: #127\n- Blockers: none\n- Next: merge when policy checks pass\n": []byte(""),
	}}

	report, err := BuildTicketNoteReport(TicketNoteInput{Repo: repo, Ticket: 126, Body: "Use ticket-level note templates.", Kind: "decision", Target: "auto", Apply: true}, runner)
	if err != nil {
		t.Fatalf("BuildTicketNoteReport error: %v", err)
	}
	if len(report.Targets) != 1 || report.Targets[0].Type != "pr" || report.Targets[0].Status != "applied" {
		t.Fatalf("unexpected targets: %+v", report.Targets)
	}
}
