package gira

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var statusNowFixture = time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

func TestSummarizeStatusCountsIssuesAndMilestones(t *testing.T) {
	milestoneTitle := "MVP"
	summary, err := SummarizeStatus(
		"StatPan/gira",
		[]normalizedMilestone{{Number: 1, Title: milestoneTitle, State: "open", OpenIssues: 1, ClosedIssues: 1}},
		[]normalizedIssue{
			issueFixture(1, "open", "2026-04-01T12:00:00Z", nil, nil),
			issueFixture(2, "open", "2026-04-25T12:00:00Z", []string{"status:blocked"}, &milestoneTitle),
			issueFixture(3, "closed", "2026-04-20T12:00:00Z", nil, nil),
		},
		statusNowFixture,
		14,
	)
	if err != nil {
		t.Fatalf("SummarizeStatus returned error: %v", err)
	}

	if summary.Counts.Issues.Total != 3 {
		t.Fatalf("total issues = %d, want 3", summary.Counts.Issues.Total)
	}
	if summary.Counts.Issues.Open != 2 || summary.Counts.Issues.Closed != 1 {
		t.Fatalf("open/closed issues = %d/%d, want 2/1", summary.Counts.Issues.Open, summary.Counts.Issues.Closed)
	}
	if summary.Counts.Issues.StaleOpen != 1 || summary.Counts.Issues.BlockedOpen != 1 {
		t.Fatalf("stale/blocked issues = %d/%d, want 1/1", summary.Counts.Issues.StaleOpen, summary.Counts.Issues.BlockedOpen)
	}
	if summary.Milestones[0].ProgressPercent != 50 {
		t.Fatalf("progress percent = %d, want 50", summary.Milestones[0].ProgressPercent)
	}
}

func TestFormatStatusTextIsCompact(t *testing.T) {
	summary, err := SummarizeStatus(
		"StatPan/gira",
		[]normalizedMilestone{{Number: 1, Title: "MVP", State: "open", OpenIssues: 1, ClosedIssues: 4}},
		[]normalizedIssue{
			issueFixture(5, "open", "2026-04-01T12:00:00Z", nil, nil),
			issueFixture(6, "open", "2026-04-25T12:00:00Z", []string{"status:blocked"}, nil),
		},
		statusNowFixture,
		14,
	)
	if err != nil {
		t.Fatalf("SummarizeStatus returned error: %v", err)
	}

	text := FormatStatusText(summary)
	for _, want := range []string{"status: StatPan/gira", "issues:", "milestone progress:", "stale open issues:", "blocked issues:", "open issues:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("status text missing %q:\n%s", want, text)
		}
	}
	if lines := strings.Count(strings.TrimSpace(text), "\n") + 1; lines > 14 {
		t.Fatalf("status text has %d lines, want <= 14:\n%s", lines, text)
	}
}

func TestBuildStatusSummaryFetchesWithGhShape(t *testing.T) {
	client := fakeStatusClient{
		repo: mustRepo(t, "StatPan/gira"),
		responses: map[string]string{
			"api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100": `[[{"number":1,"title":"MVP","state":"open","description":null,"due_on":null,"open_issues":2,"closed_issues":3}]]`,
			"api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100":     `[[{"number":1,"title":"Issue 1","state":"open","labels":[{"name":"status:blocked"}],"milestone":{"title":"MVP"},"updated_at":"2026-04-25T12:00:00Z","html_url":"https://github.com/StatPan/gira/issues/1"},{"number":2,"title":"PR 2","state":"open","pull_request":{},"labels":[],"updated_at":"2026-04-25T12:00:00Z","html_url":"https://github.com/StatPan/gira/pull/2"}]]`,
			"api repos/StatPan/gira/pulls --paginate --slurp -X GET -f state=open -f per_page=100":     `[[{"body":"Implements changes","draft":false},{"body":"Fixes #1","draft":false}]]`,
		},
	}

	summary, err := BuildStatusSummary(client, statusNowFixture, 14)
	if err != nil {
		t.Fatalf("BuildStatusSummary returned error: %v", err)
	}
	if summary.Counts.Issues.Total != 1 {
		t.Fatalf("total issues = %d, want 1", summary.Counts.Issues.Total)
	}
	if summary.Counts.Issues.PRsMissingClosureLink != 1 || summary.Counts.Issues.ClosureLinkMissingOpenIssues != 1 {
		t.Fatalf("unexpected closure-link metrics: %+v", summary.Counts.Issues)
	}
	if summary.Counts.Milestones.Total != 1 || summary.Milestones[0].ProgressPercent != 60 {
		t.Fatalf("unexpected milestone summary: %+v", summary.Milestones)
	}
}

func TestStatusJSONShapeMatchesAutomationContract(t *testing.T) {
	summary, err := SummarizeStatus(
		"StatPan/gira",
		nil,
		[]normalizedIssue{issueFixture(1, "open", "2026-04-25T12:00:00Z", nil, nil)},
		statusNowFixture,
		14,
	)
	if err != nil {
		t.Fatalf("SummarizeStatus returned error: %v", err)
	}

	output, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(output, &payload); err != nil {
		t.Fatalf("status JSON did not parse: %v\n%s", err, output)
	}
	for _, key := range []string{"repo", "fetched_at", "stale_days", "counts", "milestones", "issues"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("status JSON missing key %q:\n%s", key, output)
		}
	}
}

type fakeStatusClient struct {
	repo      RepoRef
	responses map[string]string
}

func (c fakeStatusClient) Repo() RepoRef {
	return c.repo
}

func (c fakeStatusClient) JSON(args []string, target any) error {
	key := strings.Join(args, " ")
	response := c.responses[key]
	return json.Unmarshal([]byte(response), target)
}

func issueFixture(number int, state string, updatedAt string, labels []string, milestone *string) normalizedIssue {
	return normalizedIssue{
		Number:    number,
		Title:     "Issue",
		State:     state,
		Labels:    labels,
		Milestone: milestone,
		UpdatedAt: updatedAt,
		URL:       "https://github.com/StatPan/gira/issues/1",
	}
}

func mustRepo(t *testing.T, value string) RepoRef {
	t.Helper()
	repo, err := ParseRepoRef(value)
	if err != nil {
		t.Fatalf("ParseRepoRef returned error: %v", err)
	}
	return repo
}
