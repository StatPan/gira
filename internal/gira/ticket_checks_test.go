package gira

import (
	"testing"
	"time"
)

func TestBuildTicketChecksReportShowsPendingChecks(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 227 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[{"number":228,"title":"x","body":"Closes #227","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"UNSTABLE","statusCheckRollup":[{"name":"Build linux","workflowName":"Go release","conclusion":"","status":"IN_PROGRESS","detailsUrl":"https://example.test/check"}]}]`),
		},
	}}

	report, err := BuildTicketChecksReport(repo, 227, 0, 0, runner)
	if err != nil {
		t.Fatalf("BuildTicketChecksReport error: %v", err)
	}
	if report.Ready || !containsString(report.Blockers, "checks_pending") {
		t.Fatalf("expected pending blocker, got %+v", report)
	}
	if len(report.Checks) != 1 || report.Checks[0].State != "pending" || report.Checks[0].Name != "Build linux" {
		t.Fatalf("unexpected checks: %+v", report.Checks)
	}
}

func TestBuildTicketChecksReportWaitsUntilChecksPass(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 227 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[{"number":228,"title":"x","body":"Closes #227","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"UNSTABLE","statusCheckRollup":[{"name":"Build linux","workflowName":"Go release","conclusion":"","status":"IN_PROGRESS"}]}]`),
			[]byte(`[{"number":228,"title":"x","body":"Closes #227","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"name":"Build linux","workflowName":"Go release","conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
	}}

	report, err := BuildTicketChecksReport(repo, 227, time.Second, 0, runner)
	if err != nil {
		t.Fatalf("BuildTicketChecksReport error: %v", err)
	}
	if !report.Ready || len(report.Blockers) != 0 {
		t.Fatalf("expected ready after wait, got %+v", report)
	}
	if len(report.Checks) != 1 || report.Checks[0].State != "passing" {
		t.Fatalf("unexpected checks: %+v", report.Checks)
	}
}
