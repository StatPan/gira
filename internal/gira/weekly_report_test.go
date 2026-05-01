package gira

import (
	"strings"
	"testing"
	"time"
)

type weeklyDashClient struct {
	repo   RepoRef
	issues []DashboardRawIssue
}

func (c weeklyDashClient) Repo() RepoRef                                         { return c.repo }
func (c weeklyDashClient) FetchIssues() ([]DashboardRawIssue, error)             { return c.issues, nil }
func (c weeklyDashClient) FetchPullRequests() ([]DashboardRawPullRequest, error) { return nil, nil }
func (c weeklyDashClient) FetchMilestones() ([]DashboardRawMilestone, error)     { return nil, nil }
func (c weeklyDashClient) FetchProjectSnapshot() (ProjectSyncSnapshot, error) {
	return ProjectSyncSnapshot{}, nil
}
func (c weeklyDashClient) FetchTransitionSnapshot() (ProjectTransitionSnapshot, error) {
	return ProjectTransitionSnapshot{}, nil
}
func (c weeklyDashClient) FetchCapabilities() (ProjectCapabilityReport, error) {
	return ProjectCapabilityReport{}, nil
}

type weeklyReviewClient struct {
	repo   RepoRef
	prs    []ReviewPR
	issues []ReviewIssue
}

func (c weeklyReviewClient) Repo() RepoRef                          { return c.repo }
func (c weeklyReviewClient) ListOpenPRs() ([]ReviewPR, error)       { return c.prs, nil }
func (c weeklyReviewClient) ListOpenIssues() ([]ReviewIssue, error) { return c.issues, nil }
func (c weeklyReviewClient) MergePR(number int) error               { return nil }

func TestBuildWeeklyReportDeterministic(t *testing.T) {
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	repo := ParseRepoRefMust("StatPan/gira")
	dash := weeklyDashClient{repo: repo, issues: []DashboardRawIssue{
		{IssueNumber: 70, Title: "Blocked item", State: "open", Labels: []string{"blocked"}, UpdatedAt: now.Add(-20 * 24 * time.Hour).Format(time.RFC3339), URL: "https://example/issues/70"},
		{IssueNumber: 71, Title: "Fresh item", State: "open", Labels: nil, UpdatedAt: now.Add(-2 * 24 * time.Hour).Format(time.RFC3339), URL: "https://example/issues/71"},
	}}
	review := weeklyReviewClient{repo: repo, prs: []ReviewPR{{Number: 77, Title: "Needs review", URL: "https://example/pr/77", ReviewDecision: "", UpdatedAt: now.Add(-96 * time.Hour).Format(time.RFC3339), RequestedReviewers: []string{"alice"}}}, issues: []ReviewIssue{{Number: 70, Labels: []string{"blocker"}}}}

	a, err := BuildWeeklyReport(repo, now, dash, review)
	if err != nil {
		t.Fatalf("build report: %v", err)
	}
	b, err := BuildWeeklyReport(repo, now, dash, review)
	if err != nil {
		t.Fatalf("build report: %v", err)
	}
	if a.KPIs.SLABreaches != 1 || a.KPIs.BlockedIssues != 1 || a.KPIs.ReleaseBlockers == 0 {
		t.Fatalf("unexpected kpis: %+v", a.KPIs)
	}
	if len(a.Exceptions) != len(b.Exceptions) || a.Exceptions[0].Title != b.Exceptions[0].Title {
		t.Fatalf("non-deterministic exceptions ordering")
	}
	md := FormatWeeklyReportMarkdown(a)
	if !strings.Contains(md, "Top exceptions") || !strings.Contains(md, "owner:alice") {
		t.Fatalf("missing markdown fields: %s", md)
	}
}
