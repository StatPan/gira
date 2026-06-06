package gira

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

type statsRunner struct {
	calls []string
}

func (r *statsRunner) Run(name string, args ...string) ([]byte, error) {
	call := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, call)
	switch call {
	case "gh issue list --repo StatPan/gira --state all --limit 100 --search created:>=2026-02-10 --json number,title,state,createdAt,closedAt,updatedAt,labels,url":
		return []byte(`[
			{"number":1,"title":"Ready task","state":"OPEN","createdAt":"2026-03-01T00:00:00Z","updatedAt":"2026-03-02T00:00:00Z","labels":[{"name":"type:task"},{"name":"status:ready"}],"url":"u1"},
			{"number":2,"title":"Done bug","state":"CLOSED","createdAt":"2026-03-03T00:00:00Z","closedAt":"2026-03-04T00:00:00Z","updatedAt":"2026-03-04T00:00:00Z","labels":[{"name":"type:bug"},{"name":"status:done"}],"url":"u2"}
		]`), nil
	case "gh issue list --repo StatPan/gira --state closed --limit 100 --search closed:>=2026-02-10 --json number,title,state,createdAt,closedAt,updatedAt,labels,url":
		return []byte(`[
			{"number":2,"title":"Done bug","state":"CLOSED","closedAt":"2026-03-04T00:00:00Z","labels":[{"name":"status:done"}],"url":"u2"},
			{"number":3,"title":"Old packet","state":"CLOSED","closedAt":"2026-03-05T00:00:00Z","labels":[{"name":"resolution:superseded"}],"url":"u3"}
		]`), nil
	case "gh issue list --repo StatPan/gira --state open --limit 100 --search updated:<2026-04-27 --json number,title,state,createdAt,closedAt,updatedAt,labels,url":
		return []byte(`[{"number":4,"title":"Stale issue","state":"OPEN","updatedAt":"2026-04-01T00:00:00Z","labels":[],"url":"u4"}]`), nil
	case "gh pr list --repo StatPan/gira --state all --limit 100 --search created:>=2026-02-10 --json number,title,body,state,createdAt,mergedAt,updatedAt,url,statusCheckRollup":
		return []byte(`[
			{"number":10,"title":"Fix bug","body":"Closes #2","state":"MERGED","mergedAt":"2026-03-04T00:00:00Z","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS"}],"url":"p10"},
			{"number":11,"title":"Refactor","body":"","state":"OPEN","statusCheckRollup":[{"status":"IN_PROGRESS","conclusion":""},{"status":"COMPLETED","conclusion":"FAILURE"}],"url":"p11"}
		]`), nil
	case "gh pr list --repo StatPan/gira --state open --limit 100 --search updated:<2026-04-27 --json number,title,body,state,createdAt,mergedAt,updatedAt,url,statusCheckRollup":
		return []byte(`[{"number":12,"title":"Stale PR","body":"","state":"OPEN","updatedAt":"2026-04-01T00:00:00Z","statusCheckRollup":[],"url":"p12"}]`), nil
	default:
		return nil, fmt.Errorf("unexpected call: %s", call)
	}
}

type pulseStatsRunner struct {
	calls []string
}

func (r *pulseStatsRunner) Run(name string, args ...string) ([]byte, error) {
	call := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, call)
	switch call {
	case "gh issue list --repo StatPan/gira --state all --limit 100 --search updated:>=2026-05-04 --json number,title,body,state,createdAt,closedAt,updatedAt,labels,url":
		return []byte(`[
			{"number":1,"title":"Refine old ticket","body":"## Goal\nMove safely.\n\n## Scope\nTighten the handoff.\n\n## Acceptance Criteria\n- [ ] ready","state":"OPEN","createdAt":"2026-04-01T00:00:00Z","updatedAt":"2026-05-10T00:00:00Z","labels":[{"name":"status:ready"}],"url":"u1"},
			{"number":2,"title":"Start old ticket","body":"## Goal\nBuild it.","state":"OPEN","createdAt":"2026-04-02T00:00:00Z","updatedAt":"2026-05-10T01:00:00Z","labels":[{"name":"status:in-progress"}],"url":"u2"},
			{"number":3,"title":"Unblock old ticket","body":"Blocker resolved after decision.","state":"OPEN","createdAt":"2026-04-03T00:00:00Z","updatedAt":"2026-05-10T02:00:00Z","labels":[{"name":"status:ready"},{"name":"resolution:unblocked"}],"url":"u3"},
			{"number":8,"title":"Label-only ready","body":"thin","state":"OPEN","createdAt":"2026-04-04T00:00:00Z","updatedAt":"2026-05-10T03:00:00Z","labels":[{"name":"status:ready"}],"url":"u8"},
			{"number":9,"title":"New ready ticket","body":"## Goal\nNew.\n\n## Scope\nCreated ready.\n\n## Acceptance Criteria\n- [ ] ready","state":"OPEN","createdAt":"2026-05-10T00:00:00Z","updatedAt":"2026-05-10T00:00:00Z","labels":[{"name":"status:ready"}],"url":"u9"}
		]`), nil
	case "gh issue list --repo StatPan/gira --state closed --limit 100 --search closed:>=2026-05-04 --json number,title,body,state,createdAt,closedAt,updatedAt,labels,url":
		return []byte(`[
			{"number":4,"title":"Superseded old plan","body":"","state":"CLOSED","createdAt":"2026-04-01T00:00:00Z","closedAt":"2026-05-09T00:00:00Z","updatedAt":"2026-05-09T00:00:00Z","labels":[{"name":"resolution:superseded"}],"url":"u4"},
			{"number":7,"title":"Finished ticket","body":"","state":"CLOSED","createdAt":"2026-04-01T00:00:00Z","closedAt":"2026-05-08T00:00:00Z","updatedAt":"2026-05-08T00:00:00Z","labels":[{"name":"status:done"}],"url":"u7"}
		]`), nil
	case "gh issue list --repo StatPan/gira --state open --limit 100 --json number,title,body,state,createdAt,closedAt,updatedAt,labels,url":
		return []byte(`[
			{"number":1,"title":"Refine old ticket","state":"OPEN","labels":[{"name":"status:ready"}],"url":"u1"},
			{"number":3,"title":"Unblock old ticket","state":"OPEN","labels":[{"name":"status:ready"}],"url":"u3"},
			{"number":5,"title":"Blocked ticket","state":"OPEN","labels":[{"name":"status:blocked"}],"url":"u5"},
			{"number":6,"title":"Decision ticket","state":"OPEN","labels":[{"name":"needs:decision"}],"url":"u6"},
			{"number":9,"title":"New ready ticket","state":"OPEN","labels":[{"name":"status:ready"}],"url":"u9"}
		]`), nil
	case "gh pr list --repo StatPan/gira --state all --limit 100 --search updated:>=2026-05-04 --json number,title,body,state,reviewDecision,isDraft,createdAt,mergedAt,updatedAt,url,labels,statusCheckRollup":
		return []byte(`[
			{"number":10,"title":"Finish pulse command","body":"Closes #7","state":"MERGED","reviewDecision":"APPROVED","createdAt":"2026-05-06T00:00:00Z","mergedAt":"2026-05-08T00:00:00Z","updatedAt":"2026-05-08T00:00:00Z","url":"p10","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS"}]},
			{"number":11,"title":"Reviewed open PR","body":"","state":"OPEN","reviewDecision":"APPROVED","createdAt":"2026-05-06T00:00:00Z","updatedAt":"2026-05-10T00:00:00Z","url":"p11","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS"}]},
			{"number":12,"title":"Failing open PR","body":"","state":"OPEN","reviewDecision":"REVIEW_REQUIRED","createdAt":"2026-05-06T00:00:00Z","updatedAt":"2026-05-10T00:00:00Z","url":"p12","statusCheckRollup":[{"status":"COMPLETED","conclusion":"FAILURE"}]}
		]`), nil
	case "gh pr list --repo StatPan/gira --state open --limit 100 --json number,title,body,state,reviewDecision,isDraft,createdAt,mergedAt,updatedAt,url,labels,statusCheckRollup":
		return []byte(`[
			{"number":11,"title":"Reviewed open PR","state":"OPEN","reviewDecision":"APPROVED","url":"p11","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS"}]},
			{"number":12,"title":"Failing open PR","state":"OPEN","reviewDecision":"REVIEW_REQUIRED","url":"p12","statusCheckRollup":[{"status":"COMPLETED","conclusion":"FAILURE"}]},
			{"number":13,"title":"Pending review PR","state":"OPEN","reviewDecision":"REVIEW_REQUIRED","url":"p13","statusCheckRollup":[{"status":"IN_PROGRESS","conclusion":""}]}
		]`), nil
	default:
		return nil, fmt.Errorf("unexpected call: %s", call)
	}
}

func TestBuildStatsRepoReportComputesClosureFunnel(t *testing.T) {
	runner := &statsRunner{}
	report, err := BuildStatsRepoReport(StatsRepoOptions{
		Repo: RepoRef{Owner: "StatPan", Name: "gira"},
		Now:  time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC),
	}, runner)
	if err != nil {
		t.Fatalf("BuildStatsRepoReport error: %v", err)
	}
	if report.Command != "stats repo" || !report.Source.ReadOnly || report.Window.Since != "90d" {
		t.Fatalf("unexpected report metadata: %+v", report)
	}
	if report.Metrics.OpenedIssues != 2 || report.Metrics.CompletedIssues != 1 || report.Metrics.SupersededIssues != 1 {
		t.Fatalf("unexpected issue metrics: %+v", report.Metrics)
	}
	if report.Metrics.MergedPRs != 1 || report.Metrics.MergedPRsWithLinkedIssues != 1 || report.Metrics.ChecksPendingPRs != 1 || report.Metrics.ChecksFailingPRs != 1 {
		t.Fatalf("unexpected PR metrics: %+v", report.Metrics)
	}
	if report.Metrics.StaleOpenIssues != 1 || report.Metrics.StaleOpenPRs != 1 || report.Metrics.ClosureRate != 0.5 {
		t.Fatalf("unexpected funnel metrics: %+v", report.Metrics)
	}
	out := FormatStatsRepoReport(report)
	for _, want := range []string{"Gira Closure Funnel", "closure rate: 50.0%", "personal productivity score"} {
		if !strings.Contains(out, want) {
			t.Fatalf("formatted stats missing %q:\n%s", want, out)
		}
	}
}

func TestParseStatsSince(t *testing.T) {
	now := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	parsed, err := parseStatsSince("30d", now)
	if err != nil {
		t.Fatalf("parseStatsSince error: %v", err)
	}
	if got := parsed.Format("2006-01-02"); got != "2026-04-11" {
		t.Fatalf("30d parsed to %s", got)
	}
	parsed, err = parseStatsSince("2026-01-15", now)
	if err != nil {
		t.Fatalf("parseStatsSince date error: %v", err)
	}
	if got := parsed.Format("2006-01-02"); got != "2026-01-15" {
		t.Fatalf("date parsed to %s", got)
	}
}

func TestBuildStatsPulseReportComputesEvidenceSignals(t *testing.T) {
	runner := &pulseStatsRunner{}
	report, err := BuildStatsPulseReport(StatsPulseOptions{
		Repo:  RepoRef{Owner: "StatPan", Name: "gira"},
		Since: "7d",
		Now:   time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC),
	}, runner)
	if err != nil {
		t.Fatalf("BuildStatsPulseReport error: %v", err)
	}
	if report.SchemaVersion != "pulse-report/v1alpha1" || report.Command != "stats pulse" || report.Scope.Repo != "StatPan/gira" {
		t.Fatalf("unexpected pulse metadata: %+v", report)
	}
	if report.Summary.Finished != 1 || report.Summary.Reviewed != 1 || report.Summary.Refined != 1 || report.Summary.Unblocked != 1 || report.Summary.Superseded != 1 || report.Summary.Started != 1 || report.Summary.Checked != 0 {
		t.Fatalf("unexpected pulse summary: %+v", report.Summary)
	}
	if report.Health.Ready != 3 || report.Health.Blocked != 1 || report.Health.HumanDecision != 1 || report.Health.FinishReady != 1 || report.Health.FailedCheck != 1 || report.Health.ReviewNeeded != 2 {
		t.Fatalf("unexpected pulse health: %+v", report.Health)
	}
	for _, item := range report.Items {
		if item.Issue == 8 || item.Issue == 9 {
			t.Fatalf("anti-gaming issue was counted: %+v", item)
		}
	}
	out := FormatPulseReport(report)
	for _, want := range []string{"Gira Pulse", "finished: 1", "review needed: 2", "no people or agent ranking"} {
		if !strings.Contains(out, want) {
			t.Fatalf("formatted pulse missing %q:\n%s", want, out)
		}
	}
	for _, forbidden := range []string{"token", "streak", "leaderboard", "productivity score"} {
		if strings.Contains(strings.ToLower(out), forbidden) {
			t.Fatalf("formatted pulse should not contain %q:\n%s", forbidden, out)
		}
	}
}

func TestBuildStatsPulseReportRejectsInvalidWindow(t *testing.T) {
	_, err := BuildStatsPulseReport(StatsPulseOptions{
		Repo:  RepoRef{Owner: "StatPan", Name: "gira"},
		Since: "0d",
		Now:   time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC),
	}, &pulseStatsRunner{})
	if err == nil || !strings.Contains(err.Error(), "--since must be a positive day window") {
		t.Fatalf("expected invalid since error, got %v", err)
	}
}
