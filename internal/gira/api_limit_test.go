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
}
