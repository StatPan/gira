package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildReleaseNotesReportLinksMergedPRsAndRendersMarkdown(t *testing.T) {
	client := &fakeReleaseNotesClient{
		issues: []ReleaseNotesIssue{
			{Number: 10, Title: "Add export button", State: "closed", Labels: []string{"type:story"}, Milestone: "v2.1.0", URL: "u10"},
			{Number: 11, Title: "Fix install bug", State: "closed", Labels: []string{"type:bug"}, Milestone: "v2.1.0", URL: "u11"},
			{Number: 12, Title: "Still open", State: "open", Labels: []string{"type:task"}, Milestone: "v2.1.0", URL: "u12"},
			{Number: 13, Title: "Other milestone", State: "closed", Labels: []string{"type:task"}, Milestone: "v2.0.0", URL: "u13"},
		},
		prs: []ReleaseNotesPullRequest{
			{Number: 100, Title: "Export button", Body: "Closes #10", URL: "p100", MergedAt: "2026-06-01T00:00:00Z"},
		},
		milestones: []DashboardRawMilestone{{MilestoneNumber: 1, Title: "v2.1.0", State: "closed"}},
	}
	report, err := BuildReleaseNotesReport(ParseRepoRefMust("StatPan/gira"), "v2.1.0", client, time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildReleaseNotesReport returned error: %v", err)
	}
	if report.SchemaVersion != ReleaseNotesReportSchemaVersion || report.Counts.Issues != 2 || report.Counts.PullRequests != 1 || report.Counts.Features != 1 || report.Counts.Fixes != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Confidence != "review_required" {
		t.Fatalf("confidence = %s, want review_required", report.Confidence)
	}
	if !containsString(report.Warnings, "open_issue_in_milestone:#12") || !containsString(report.Warnings, "issue#11:missing_linked_pr") {
		t.Fatalf("warnings missing expected gaps: %+v", report.Warnings)
	}
	markdown := RenderReleaseNotesMarkdown(report)
	if !strings.Contains(markdown, "# Release Notes: v2.1.0") || !strings.Contains(markdown, "Add export button (#10 via PR #100)") {
		t.Fatalf("markdown missing release content:\n%s", markdown)
	}
	csvBytes, err := RenderReleaseNotesCSV(report)
	if err != nil {
		t.Fatalf("RenderReleaseNotesCSV returned error: %v", err)
	}
	if !strings.HasPrefix(string(csvBytes), "kind,repo,issue,pr,title,group,status,milestone,evidence_level,evidence,warnings,url\n") {
		t.Fatalf("unexpected CSV:\n%s", csvBytes)
	}
}

func TestWriteReleaseNotesBundleWritesHumanAndMachineArtifacts(t *testing.T) {
	report := ReleaseNotesReport{
		Command:       "report release-notes",
		SchemaVersion: ReleaseNotesReportSchemaVersion,
		Repo:          "StatPan/gira",
		Milestone:     "v2.1.0",
		GeneratedAt:   "2026-06-18T09:00:00Z",
		Confidence:    "ready",
		Items: []ReleaseNotesItem{
			{Kind: "issue", Repo: "StatPan/gira", Issue: 10, PullRequest: 100, Title: "Add export button", Group: "features", Status: "included", Milestone: "v2.1.0", EvidenceLevel: "guaranteed", Evidence: []string{"github_issue", "merged_pr"}},
		},
		Sections: []ReleaseNotesSection{{Group: "features", Title: "Features", Count: 1}},
	}
	report.PublishableDraft = renderReleaseNotesPublishableDraft(report)
	outputRoot := filepath.Join(t.TempDir(), "release-notes")
	if err := WriteReleaseNotesBundle(outputRoot, report); err != nil {
		t.Fatalf("WriteReleaseNotesBundle returned error: %v", err)
	}
	for _, rel := range []string{"index.html", "release-notes.md", "derived/release_notes.json", "csv/release_items.csv"} {
		if _, err := os.Stat(filepath.Join(outputRoot, rel)); err != nil {
			t.Fatalf("expected artifact %s: %v", rel, err)
		}
	}
}

type fakeReleaseNotesClient struct {
	issues     []ReleaseNotesIssue
	prs        []ReleaseNotesPullRequest
	milestones []DashboardRawMilestone
}

func (c *fakeReleaseNotesClient) FetchIssues() ([]ReleaseNotesIssue, error) {
	return c.issues, nil
}

func (c *fakeReleaseNotesClient) FetchMergedPRs() ([]ReleaseNotesPullRequest, error) {
	return c.prs, nil
}

func (c *fakeReleaseNotesClient) FetchMilestones() ([]DashboardRawMilestone, error) {
	return c.milestones, nil
}
