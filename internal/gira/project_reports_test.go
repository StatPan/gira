package gira

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type projectReportDashClient struct {
	repo       RepoRef
	issues     []DashboardRawIssue
	milestones []DashboardRawMilestone
}

func (c projectReportDashClient) Repo() RepoRef { return c.repo }
func (c projectReportDashClient) FetchIssues() ([]DashboardRawIssue, error) {
	return c.issues, nil
}
func (c projectReportDashClient) FetchPullRequests() ([]DashboardRawPullRequest, error) {
	return nil, nil
}
func (c projectReportDashClient) FetchMilestones() ([]DashboardRawMilestone, error) {
	return c.milestones, nil
}
func (c projectReportDashClient) FetchProjectSnapshot() (ProjectSyncSnapshot, error) {
	return ProjectSyncSnapshot{}, nil
}
func (c projectReportDashClient) FetchTransitionSnapshot() (ProjectTransitionSnapshot, error) {
	return ProjectTransitionSnapshot{}, nil
}
func (c projectReportDashClient) FetchCapabilities() (ProjectCapabilityReport, error) {
	return ProjectCapabilityReport{}, nil
}

func TestBuildProjectReportsFromGitHubEvidence(t *testing.T) {
	now := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	repo := ParseRepoRefMust("StatPan/gira")
	dash := projectReportDashClient{
		repo: repo,
		issues: []DashboardRawIssue{
			{IssueNumber: 10, Title: "Ship report bundle", State: "closed", Labels: []string{"type:feature", "qa"}, UpdatedAt: now.Add(-2 * 24 * time.Hour).Format(time.RFC3339), Milestone: "v2.5.0", URL: "https://example/issues/10"},
			{IssueNumber: 11, Title: "Blocked rollout", State: "open", Labels: []string{"blocked"}, UpdatedAt: now.Add(-20 * 24 * time.Hour).Format(time.RFC3339), Milestone: "v2.5.0", URL: "https://example/issues/11"},
			{IssueNumber: 12, Title: "Unplanned work", State: "open", Labels: []string{"type:task"}, UpdatedAt: now.Add(-1 * 24 * time.Hour).Format(time.RFC3339), URL: "https://example/issues/12"},
		},
		milestones: []DashboardRawMilestone{{Title: "v2.5.0", State: "open", OpenIssues: 1, ClosedIssues: 1}},
	}
	review := weeklyReviewClient{repo: repo, prs: []ReviewPR{{Number: 20, Title: "Needs checks", Body: "Fixes #11", URL: "https://example/pull/20", CheckStatus: "failing", UpdatedAt: now.Add(-3 * 24 * time.Hour).Format(time.RFC3339)}}}

	milestone, err := BuildProjectReport(repo, dash, review, now, ProjectReportOptions{Kind: "milestone", Milestone: "v2.5.0"})
	if err != nil {
		t.Fatalf("milestone report: %v", err)
	}
	if milestone.Counts.CompletionPct != 50 || milestone.Confidence != "review_required" {
		t.Fatalf("unexpected milestone report: %+v", milestone)
	}

	backlog, err := BuildProjectReport(repo, dash, review, now, ProjectReportOptions{Kind: "backlog-health"})
	if err != nil {
		t.Fatalf("backlog report: %v", err)
	}
	if backlog.Counts.Blocked != 1 || backlog.Counts.Unplanned != 1 || backlog.Counts.Stale != 1 {
		t.Fatalf("unexpected backlog counts: %+v", backlog.Counts)
	}

	qa, err := BuildProjectReport(repo, dash, review, now, ProjectReportOptions{Kind: "qa-checklist", Milestone: "v2.5.0"})
	if err != nil {
		t.Fatalf("qa report: %v", err)
	}
	if qa.Counts.ChecklistItems == 0 || qa.Counts.ChecklistComplete == qa.Counts.ChecklistItems {
		t.Fatalf("qa report should include review items: %+v", qa.Counts)
	}
	csvBytes, err := RenderProjectReportCSV(qa)
	if err != nil {
		t.Fatalf("render csv: %v", err)
	}
	if !strings.HasPrefix(string(csvBytes), "kind,repo,issue,pr,title,group,status,priority,milestone,age_days,evidence,warnings,url\n") {
		t.Fatalf("unexpected csv header:\n%s", string(csvBytes))
	}
}

func TestWriteProjectReportBundle(t *testing.T) {
	report := ProjectReport{
		Command:       "report backlog-health",
		SchemaVersion: ProjectReportSchemaVersion,
		Repo:          "StatPan/gira",
		Kind:          "backlog-health",
		Title:         "Backlog Health Report",
		GeneratedAt:   "2026-06-18T00:00:00Z",
		Confidence:    "ready",
		Items:         []ProjectReportItem{{Kind: "issue", Repo: "StatPan/gira", Issue: 1, Title: "Ready item", Group: "ready", Status: "open", Evidence: []string{"github_issue"}}},
		Sections:      []ProjectReportSection{{Group: "ready", Title: "Ready", Count: 1}},
	}
	outputRoot := filepath.Join(t.TempDir(), "backlog")
	if err := WriteProjectReportBundle(outputRoot, report); err != nil {
		t.Fatalf("write bundle: %v", err)
	}
	for _, rel := range []string{"index.html", "report.md", "derived/report.json", "csv/report_items.csv"} {
		if !fileExists(filepath.Join(outputRoot, rel)) {
			t.Fatalf("missing bundle artifact %s", rel)
		}
	}
}
