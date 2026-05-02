package gira

import (
	"testing"
	"time"
)

type fakeTriageClient struct {
	repo   RepoRef
	issues []TriageIssue
	edits  map[int][]string
}

func (f *fakeTriageClient) Repo() RepoRef                          { return f.repo }
func (f *fakeTriageClient) ListOpenIssues() ([]TriageIssue, error) { return f.issues, nil }
func (f *fakeTriageClient) AddLabels(issue int, labels []string) error {
	if f.edits == nil {
		f.edits = map[int][]string{}
	}
	f.edits[issue] = append(f.edits[issue], labels...)
	return nil
}

func TestBuildTriageQueueBuckets(t *testing.T) {
	repo, _ := ParseRepoRef("StatPan/gira")
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	client := &fakeTriageClient{repo: repo, issues: []TriageIssue{
		{Number: 65, Title: "a", State: "open", Labels: []string{"priority:p1", "due:2026-04-28"}, CreatedAt: "2026-04-20T00:00:00Z"},
		{Number: 66, Title: "b", State: "open", Labels: []string{"due:2026-05-10"}, Assignees: []string{"alice"}, CreatedAt: "2026-04-30T00:00:00Z"},
	}}
	report, err := BuildTriageQueue(client, now)
	if err != nil {
		t.Fatalf("BuildTriageQueue err=%v", err)
	}
	if len(report.Buckets["unowned"]) != 1 || report.Buckets["unowned"][0].Issue.Number != 65 {
		t.Fatalf("unexpected unowned bucket: %+v", report.Buckets["unowned"])
	}
	if len(report.Buckets["no-priority"]) != 1 || report.Buckets["no-priority"][0].Issue.Number != 66 {
		t.Fatalf("unexpected no-priority bucket: %+v", report.Buckets["no-priority"])
	}
	if len(report.Buckets["overdue"]) != 1 || report.Buckets["overdue"][0].Issue.Number != 65 {
		t.Fatalf("unexpected overdue bucket: %+v", report.Buckets["overdue"])
	}
}

func TestApplyTriagePolicyDryRunAndApply(t *testing.T) {
	repo, _ := ParseRepoRef("StatPan/gira")
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	client := &fakeTriageClient{repo: repo, issues: []TriageIssue{{Number: 70, Title: "x", State: "open", Labels: []string{}, CreatedAt: "2026-04-20T00:00:00Z"}}}
	policy := TriagePolicy{EscalationLabels: map[string][]string{"missing_assignee": {"triage:unowned"}, "missing_priority": {"triage:no-priority"}}}
	report, err := ApplyTriagePolicy(client, policy, false, now)
	if err != nil {
		t.Fatalf("ApplyTriagePolicy dry-run err=%v", err)
	}
	if report.Mode != "dry-run" || len(report.Actions) == 0 {
		t.Fatalf("unexpected dry-run report: %+v", report)
	}
	if len(client.edits) != 0 {
		t.Fatalf("dry-run should not edit")
	}
	_, err = ApplyTriagePolicy(client, policy, true, now)
	if err != nil {
		t.Fatalf("ApplyTriagePolicy apply err=%v", err)
	}
	if len(client.edits[70]) == 0 {
		t.Fatalf("expected edits for issue 70")
	}
}

func TestValidateTriagePolicyFailsClosed(t *testing.T) {
	policy := TriagePolicy{SLAWindowsByPriority: map[string]string{"p0": "not-duration"}}
	if err := ValidateTriagePolicy(policy); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestNormalizeOpenIssueTriageAddsMissingAxes(t *testing.T) {
	repo, _ := ParseRepoRef("StatPan/gira")
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	client := &fakeTriageClient{repo: repo, issues: []TriageIssue{{Number: 80, Title: "CLI command cleanup", Labels: []string{"status:ready"}}}}
	report, err := NormalizeOpenIssueTriage(client, false, now)
	if err != nil {
		t.Fatalf("NormalizeOpenIssueTriage err=%v", err)
	}
	if report.Planned != 1 || report.Applied != 0 || report.Skipped != 0 {
		t.Fatalf("unexpected summary: %+v", report)
	}
	if len(report.Issues) != 1 || len(report.Issues[0].Add) != 3 {
		t.Fatalf("unexpected planned labels: %+v", report.Issues)
	}
}

func TestNormalizeOpenIssueTriageSkipsConflicts(t *testing.T) {
	repo, _ := ParseRepoRef("StatPan/gira")
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	client := &fakeTriageClient{repo: repo, issues: []TriageIssue{{Number: 81, Title: "conflict", Labels: []string{"type:task", "type:bug", "priority:p2"}}}}
	report, err := NormalizeOpenIssueTriage(client, false, now)
	if err != nil {
		t.Fatalf("NormalizeOpenIssueTriage err=%v", err)
	}
	if report.Skipped != 1 || report.Issues[0].Skip != "needs-decision" {
		t.Fatalf("expected needs-decision skip: %+v", report)
	}
}

func TestNormalizeOpenIssueTriageApplyIdempotent(t *testing.T) {
	repo, _ := ParseRepoRef("StatPan/gira")
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	client := &fakeTriageClient{repo: repo, issues: []TriageIssue{{Number: 82, Title: "docs", Labels: []string{}}}}
	first, err := NormalizeOpenIssueTriage(client, true, now)
	if err != nil {
		t.Fatalf("NormalizeOpenIssueTriage first err=%v", err)
	}
	if first.Applied != 1 {
		t.Fatalf("expected first apply mutation: %+v", first)
	}
	client.issues[0].Labels = append(client.issues[0].Labels, client.edits[82]...)
	second, err := NormalizeOpenIssueTriage(client, true, now)
	if err != nil {
		t.Fatalf("NormalizeOpenIssueTriage second err=%v", err)
	}
	if second.Applied != 0 || second.Planned != 0 || second.Unchanged != 1 {
		t.Fatalf("expected idempotent second apply: %+v", second)
	}
}
