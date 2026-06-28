package gira

import (
	"strings"
	"testing"
	"time"
)

func TestParseAPILimitReport(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	raw := []byte(`{
		"resources": {
			"core": {"limit": 5000, "used": 125, "remaining": 4875, "reset": 1782530958},
			"graphql": {"limit": 5000, "used": 4, "remaining": 4996, "reset": 1782533345},
			"search": {"limit": 30, "used": 1, "remaining": 29, "reset": 1782530525}
		},
		"rate": {"limit": 5000, "used": 125, "remaining": 4875, "reset": 1782530958}
	}`)

	report, err := ParseAPILimitReport(repo, raw, time.Date(2026, 6, 27, 3, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ParseAPILimitReport error: %v", err)
	}
	if report.SchemaVersion != APILimitReportSchemaVersion || report.Command != "ops limit" || report.Repo != "StatPan/gira" {
		t.Fatalf("unexpected identity: %+v", report)
	}
	if report.Core.Remaining != 4875 || report.GraphQL.Remaining != 4996 || report.Search.Remaining != 29 {
		t.Fatalf("unexpected buckets: %+v", report)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("healthy budget should not warn: %+v", report.Warnings)
	}
	if report.Secondary.Status != "unobservable" || len(report.Secondary.Signals) == 0 {
		t.Fatalf("secondary limit should be unobservable with signals: %+v", report.Secondary)
	}
	text := FormatAPILimitReport(report)
	for _, want := range []string{"ops limit: StatPan/gira", "core: remaining=4875/5000", "graphql: remaining=4996/5000", "search: remaining=29/30", "secondary: unobservable"} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted report missing %q:\n%s", want, text)
		}
	}
}

func TestParseAPILimitReportWarnsOnExhaustedBuckets(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	raw := []byte(`{"resources":{"core":{"limit":5000,"remaining":0},"graphql":{"limit":5000,"remaining":0},"search":{"limit":30,"remaining":0}}}`)

	report, err := ParseAPILimitReport(repo, raw, time.Time{})
	if err != nil {
		t.Fatalf("ParseAPILimitReport error: %v", err)
	}
	if len(report.Warnings) != 3 {
		t.Fatalf("warnings = %+v, want 3 exhausted bucket warnings", report.Warnings)
	}
	if !strings.Contains(report.Warnings[1], "GitHub GraphQL budget exhausted") || !strings.Contains(report.Warnings[1], "gira ops limit") {
		t.Fatalf("warning should be short and actionable: %+v", report.Warnings)
	}
}

func TestParseAPILimitReportWarnsOnLowBuckets(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	raw := []byte(`{"resources":{"core":{"limit":5000,"remaining":500},"graphql":{"limit":5000,"remaining":501},"search":{"limit":30,"remaining":3}}}`)

	report, err := ParseAPILimitReport(repo, raw, time.Time{})
	if err != nil {
		t.Fatalf("ParseAPILimitReport error: %v", err)
	}
	if len(report.Warnings) != 2 {
		t.Fatalf("warnings = %+v, want low core/search warnings only", report.Warnings)
	}
	for _, warning := range report.Warnings {
		if !strings.Contains(warning, "GitHub API budget low") || !strings.Contains(warning, "gira ops limit") {
			t.Fatalf("warning should be concise and point to ops limit: %q", warning)
		}
	}
}

func TestWithAPILimitWorkflowTicketLifecycle(t *testing.T) {
	report := APILimitReport{
		Repo:    "StatPan/gira",
		Core:    APILimitBucket{Limit: 5000, Remaining: 1000},
		GraphQL: APILimitBucket{Limit: 5000, Remaining: 500},
		Search:  APILimitBucket{Limit: 30, Remaining: 25},
	}

	report, err := WithAPILimitWorkflow(report, WorkflowCostProfileTicketLifecycle)
	if err != nil {
		t.Fatalf("WithAPILimitWorkflow error: %v", err)
	}
	if report.Workflow == nil {
		t.Fatal("workflow estimate missing")
	}
	if report.Workflow.SafeRuns != 7 || report.Workflow.LimitingBucket != "rest_core" {
		t.Fatalf("workflow = %+v, want safe_runs=7 limiting_bucket=rest_core", report.Workflow)
	}
	if report.Workflow.Cost != (WorkflowCostBucketEstimate{RESTCore: 110, GraphQL: 8, Search: 1, WriteContent: 12}) {
		t.Fatalf("cost = %+v", report.Workflow.Cost)
	}
	if report.Workflow.BucketRuns.WriteContent != -1 || report.Workflow.WriteContentMeasurable {
		t.Fatalf("write/content should be unobservable: %+v", report.Workflow)
	}
	text := FormatAPILimitReport(report)
	for _, want := range []string{"workflow: ticket-lifecycle", "safe runs: 7 limiting_bucket=rest_core", "write_content=unobservable", "secondary note:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted report missing %q:\n%s", want, text)
		}
	}
}

func TestAPILimitWorkflowSelectsGraphQLLimitingBucket(t *testing.T) {
	estimate := BuildAPILimitWorkflowEstimate(
		APILimitReport{
			Core:    APILimitBucket{Remaining: 5000},
			GraphQL: APILimitBucket{Remaining: 24},
			Search:  APILimitBucket{Remaining: 30},
		},
		WorkflowCostProfileTicketLifecycle,
		WorkflowCostModeConservative,
		WorkflowCostBucketEstimate{RESTCore: 100, GraphQL: 24, Search: 1, WriteContent: 1},
	)
	if estimate.SafeRuns != 0 || estimate.LimitingBucket != "graphql" {
		t.Fatalf("estimate = %+v, want graphql limit at 0 safe runs", estimate)
	}
}
