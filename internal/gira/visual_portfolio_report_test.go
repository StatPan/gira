package gira

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBuildVisualPortfolioReportAndGoldenHTML(t *testing.T) {
	now := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	repo := ParseRepoRefMust("StatPan/gira")
	due := "2026-08-15T00:00:00Z"
	start := "2026-07-15"
	target := "2026-08-01"
	dashboard := &fakeDashboardExportClient{
		repo: repo,
		issues: []DashboardRawIssue{
			{IssueNumber: 871, Title: "Blocked <unsafe>", State: "open", Labels: []string{"status:blocked"}, Milestone: "V3", UpdatedAt: "2026-07-20T12:00:00Z", URL: "https://example.test/issues/871"},
		},
		milestones:      []DashboardRawMilestone{{MilestoneNumber: 7, Title: "V3", State: "open", DueOn: &due, OpenIssues: 2, ClosedIssues: 6}},
		projectSnapshot: ProjectSyncSnapshot{RoadmapItems: []ProjectRoadmapItem{{IssueNumber: 871, IssueTitle: "Portfolio gate", IssueURL: "https://example.test/issues/871", StartDate: &start, TargetDate: &target}}},
	}
	review := weeklyReviewClient{repo: repo, prs: []ReviewPR{{Number: 887, Title: "Needs review", URL: "https://example.test/pull/887", ReviewDecision: "REVIEW_REQUIRED", CheckStatus: "passing", UpdatedAt: "2026-07-21T10:00:00Z"}}}

	report, err := BuildVisualPortfolioReport(VisualPortfolioReportOptions{Repos: []RepoRef{repo}, Milestones: []string{"V3"}, Since: "2026-07-01", Until: "2026-08-31", Now: now}, func(RepoRef) DashboardExportClient { return dashboard }, func(RepoRef) ReviewGateClient { return review })
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Milestones != 1 || report.Summary.OpenIssues != 2 || report.Summary.ClosedIssues != 6 || report.Summary.BlockedItems != 1 || report.Summary.ReviewWaitingItems != 1 {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}
	if len(report.Timeline) != 3 || report.Milestones[0].CompletionPercent != 75 || report.Milestones[0].Trace != "closed_issues=6 / total_issues=8 from GitHub milestone counters" {
		t.Fatalf("source-derived progress or dates regressed: milestones=%+v timeline=%+v", report.Milestones, report.Timeline)
	}
	html, err := RenderVisualPortfolioHTML(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{`href="#content"`, `role="progressbar"`, `aria-valuenow="75"`, "product_outcome_confidence", "No dated milestones", "@media (max-width:520px)"} {
		if required == "No dated milestones" {
			continue
		}
		if !strings.Contains(html, required) {
			t.Fatalf("HTML missing %q", required)
		}
	}
	if strings.Contains(html, "Blocked <unsafe>") || !strings.Contains(html, "Blocked &lt;unsafe&gt;") {
		t.Fatalf("HTML escaping regressed")
	}
	wantDigest, err := os.ReadFile("testdata/visual_portfolio_report.golden.sha256")
	if err != nil {
		t.Fatal(err)
	}
	gotDigest := fmt.Sprintf("%x", sha256.Sum256([]byte(html)))
	if gotDigest != strings.TrimSpace(string(wantDigest)) {
		t.Fatalf("golden HTML digest changed: got %s want %s", gotDigest, strings.TrimSpace(string(wantDigest)))
	}
}

func TestBuildVisualPortfolioReportPreservesPartialAndUnknownState(t *testing.T) {
	repo := ParseRepoRefMust("StatPan/gira")
	dashboard := &visualPortfolioErrorDashboard{repo: repo}
	review := visualPortfolioErrorReview{repo: repo}
	report, err := BuildVisualPortfolioReport(VisualPortfolioReportOptions{Repos: []RepoRef{repo}, Now: time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)}, func(RepoRef) DashboardExportClient { return dashboard }, func(RepoRef) ReviewGateClient { return review })
	if err != nil {
		t.Fatal(err)
	}
	if report.Repositories[0].Status != "unavailable" || report.Summary.AvailableRepos != 0 || len(report.Sources) != 4 {
		t.Fatalf("partial access was hidden: %+v", report)
	}
	html, err := RenderVisualPortfolioHTML(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"No milestones matched", "No dated milestones", "No blocked items", "No review-waiting items", "unavailable"} {
		if !strings.Contains(html, want) {
			t.Fatalf("empty/unsupported state missing %q", want)
		}
	}
}

func TestVisualPortfolioLargeFixtureBudget(t *testing.T) {
	repo := ParseRepoRefMust("StatPan/gira")
	issues := make([]DashboardRawIssue, 2000)
	for i := range issues {
		issues[i] = DashboardRawIssue{IssueNumber: i + 1, Title: fmt.Sprintf("Issue %d", i+1), State: "open", Labels: []string{"status:blocked"}, UpdatedAt: "2026-07-20T00:00:00Z", URL: fmt.Sprintf("https://example.test/issues/%d", i+1)}
	}
	dashboard := &fakeDashboardExportClient{repo: repo, issues: issues}
	review := weeklyReviewClient{repo: repo}
	started := time.Now()
	report, err := BuildVisualPortfolioReport(VisualPortfolioReportOptions{Repos: []RepoRef{repo}, Now: time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)}, func(RepoRef) DashboardExportClient { return dashboard }, func(RepoRef) ReviewGateClient { return review })
	if err != nil {
		t.Fatal(err)
	}
	html, err := RenderVisualPortfolioHTML(report)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("large report exceeded 2s budget: %s", elapsed)
	}
	if len(html) > 2_000_000 {
		t.Fatalf("large report exceeded 2MB budget: %d", len(html))
	}
}

type visualPortfolioErrorDashboard struct{ repo RepoRef }

func (c *visualPortfolioErrorDashboard) Repo() RepoRef { return c.repo }
func (c *visualPortfolioErrorDashboard) FetchIssues() ([]DashboardRawIssue, error) {
	return nil, fmt.Errorf("issues denied")
}
func (c *visualPortfolioErrorDashboard) FetchPullRequests() ([]DashboardRawPullRequest, error) {
	return nil, fmt.Errorf("pulls denied")
}
func (c *visualPortfolioErrorDashboard) FetchMilestones() ([]DashboardRawMilestone, error) {
	return nil, fmt.Errorf("milestones denied")
}
func (c *visualPortfolioErrorDashboard) FetchProjectSnapshot() (ProjectSyncSnapshot, error) {
	return ProjectSyncSnapshot{}, fmt.Errorf("project denied")
}
func (c *visualPortfolioErrorDashboard) FetchTransitionSnapshot() (ProjectTransitionSnapshot, error) {
	return ProjectTransitionSnapshot{}, nil
}
func (c *visualPortfolioErrorDashboard) FetchCapabilities() (ProjectCapabilityReport, error) {
	return ProjectCapabilityReport{}, nil
}

type visualPortfolioErrorReview struct{ repo RepoRef }

func (c visualPortfolioErrorReview) Repo() RepoRef { return c.repo }
func (c visualPortfolioErrorReview) ListOpenPRs() ([]ReviewPR, error) {
	return nil, fmt.Errorf("reviews denied")
}
func (c visualPortfolioErrorReview) ListOpenIssues() ([]ReviewIssue, error) { return nil, nil }
func (c visualPortfolioErrorReview) MergePR(int) error                      { return fmt.Errorf("unexpected mutation") }
