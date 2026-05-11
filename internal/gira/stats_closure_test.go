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
