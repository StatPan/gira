package gira

import (
	"testing"
	"time"
)

type fakeReviewGateClient struct {
	repo   RepoRef
	prs    []ReviewPR
	issues []ReviewIssue
	merged []int
}

func (f *fakeReviewGateClient) Repo() RepoRef                          { return f.repo }
func (f *fakeReviewGateClient) ListOpenPRs() ([]ReviewPR, error)       { return f.prs, nil }
func (f *fakeReviewGateClient) ListOpenIssues() ([]ReviewIssue, error) { return f.issues, nil }
func (f *fakeReviewGateClient) MergePR(number int) error {
	f.merged = append(f.merged, number)
	return nil
}

func TestBuildReviewQueueDeterministicOrderAndStale(t *testing.T) {
	repo, _ := ParseRepoRef("StatPan/gira")
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	client := &fakeReviewGateClient{repo: repo, prs: []ReviewPR{
		{Number: 10, ReviewDecision: "", CheckStatus: "passing", UpdatedAt: "2026-04-20T00:00:00Z", RequestedReviewers: []string{"alice"}},
		{Number: 11, ReviewDecision: "APPROVED", CheckStatus: "passing", UpdatedAt: "2026-04-30T00:00:00Z", RequestedReviewers: []string{"bob"}},
	}}
	report, err := BuildReviewQueue(client, now)
	if err != nil {
		t.Fatalf("BuildReviewQueue err=%v", err)
	}
	if len(report.Items) != 2 || report.Items[0].PR.Number != 10 {
		t.Fatalf("unexpected order: %+v", report.Items)
	}
	if !report.Items[0].StaleReview {
		t.Fatalf("expected stale review for PR 10")
	}
}

func TestBuildMergeQueueDryRunAndApply(t *testing.T) {
	repo, _ := ParseRepoRef("StatPan/gira")
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	client := &fakeReviewGateClient{repo: repo, prs: []ReviewPR{
		{Number: 20, ReviewDecision: "APPROVED", CheckStatus: "passing", UpdatedAt: "2026-04-30T00:00:00Z"},
		{Number: 21, ReviewDecision: "", CheckStatus: "passing", UpdatedAt: "2026-04-30T00:00:00Z"},
	}}
	report, err := BuildMergeQueue(client, now, false)
	if err != nil {
		t.Fatalf("BuildMergeQueue dry-run err=%v", err)
	}
	if report.Mode != "dry-run" || len(report.Candidates) != 1 || report.Candidates[0].PR.Number != 20 {
		t.Fatalf("unexpected dry-run report: %+v", report)
	}
	report, err = BuildMergeQueue(client, now, true)
	if err != nil {
		t.Fatalf("BuildMergeQueue apply err=%v", err)
	}
	if len(report.Merged) != 1 || report.Merged[0] != 20 {
		t.Fatalf("unexpected merged: %+v", report.Merged)
	}
}

func TestBuildReleaseReadiness(t *testing.T) {
	repo, _ := ParseRepoRef("StatPan/gira")
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	client := &fakeReviewGateClient{repo: repo,
		prs:    []ReviewPR{{Number: 30, ReviewDecision: "", CheckStatus: "failing", UpdatedAt: "2026-04-30T00:00:00Z"}},
		issues: []ReviewIssue{{Number: 40, Labels: []string{"blocker"}}, {Number: 41, Labels: []string{"must-fix"}}},
	}
	report, err := BuildReleaseReadiness(client, now)
	if err != nil {
		t.Fatalf("BuildReleaseReadiness err=%v", err)
	}
	if report.Ready {
		t.Fatalf("expected not ready")
	}
	if len(report.BlockingPRs) != 1 || len(report.OpenBlockers) != 1 || len(report.OpenMustFix) != 1 {
		t.Fatalf("unexpected readiness report: %+v", report)
	}
}
