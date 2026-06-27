package gira

import (
	"testing"
	"time"
)

func TestBuildTicketChecksReportShowsPendingChecks(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh api repos/StatPan/gira/issues/227/timeline --paginate": {[]byte(`[{"event":"cross-referenced","source":{"issue":{"number":228,"body":"Closes #227","pull_request":{"url":"https://api.github.com/repos/StatPan/gira/pulls/228"}}}}]`)},
		"gh api repos/StatPan/gira/pulls/228":                      {[]byte(`{"number":228,"title":"x","body":"Closes #227","state":"open","html_url":"u","draft":false,"mergeable_state":"unstable","head":{"ref":"issue-227-checks","sha":"abc123"},"base":{"ref":"main"}}`)},
		"gh api repos/StatPan/gira/pulls/228/reviews --paginate":   {[]byte(`[{"state":"APPROVED","submitted_at":"2026-06-18T09:00:00Z"}]`)},
		"gh api repos/StatPan/gira/commits/abc123/check-runs -X GET -f per_page=100": {
			[]byte(`{"check_runs":[{"name":"Build linux","status":"in_progress","conclusion":"","html_url":"https://example.test/check","app":{"name":"Go release"}}]}`),
		},
		"gh api repos/StatPan/gira/commits/abc123/status": {[]byte(`{"statuses":[]}`)},
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
		"gh api repos/StatPan/gira/issues/227/timeline --paginate": {
			[]byte(`[{"event":"cross-referenced","source":{"issue":{"number":228,"body":"Closes #227","pull_request":{"url":"https://api.github.com/repos/StatPan/gira/pulls/228"}}}}]`),
			[]byte(`[{"event":"cross-referenced","source":{"issue":{"number":228,"body":"Closes #227","pull_request":{"url":"https://api.github.com/repos/StatPan/gira/pulls/228"}}}}]`),
		},
		"gh api repos/StatPan/gira/pulls/228": {
			[]byte(`{"number":228,"title":"x","body":"Closes #227","state":"open","html_url":"u","draft":false,"mergeable_state":"unstable","head":{"ref":"issue-227-checks","sha":"abc123"},"base":{"ref":"main"}}`),
			[]byte(`{"number":228,"title":"x","body":"Closes #227","state":"open","html_url":"u","draft":false,"mergeable_state":"clean","head":{"ref":"issue-227-checks","sha":"abc123"},"base":{"ref":"main"}}`),
		},
		"gh api repos/StatPan/gira/pulls/228/reviews --paginate": {
			[]byte(`[{"state":"APPROVED","submitted_at":"2026-06-18T09:00:00Z"}]`),
			[]byte(`[{"state":"APPROVED","submitted_at":"2026-06-18T09:00:00Z"}]`),
		},
		"gh api repos/StatPan/gira/commits/abc123/check-runs -X GET -f per_page=100": {
			[]byte(`{"check_runs":[{"name":"Build linux","status":"in_progress","conclusion":"","app":{"name":"Go release"}}]}`),
			[]byte(`{"check_runs":[{"name":"Build linux","status":"completed","conclusion":"success","app":{"name":"Go release"}}]}`),
		},
		"gh api repos/StatPan/gira/commits/abc123/status": {
			[]byte(`{"statuses":[]}`),
			[]byte(`{"statuses":[]}`),
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
