package gira

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

type fakeReviewGateClient struct {
	repo      RepoRef
	prs       []ReviewPR
	issues    []ReviewIssue
	merged    []int
	policy    *ResolvedOperationPolicy
	policyErr error
	prsErr    error
	issuesErr error
}

func (f *fakeReviewGateClient) Repo() RepoRef { return f.repo }
func (f *fakeReviewGateClient) ResolveOperationPolicy() (ResolvedOperationPolicy, error) {
	if f.policyErr != nil {
		return ResolvedOperationPolicy{}, f.policyErr
	}
	if f.policy != nil {
		return *f.policy, nil
	}
	return ResolvedOperationPolicy{OperationMode: OperationModeManaged, DeliveryPolicy: DeliveryPolicyRequired, Source: "test"}, nil
}
func (f *fakeReviewGateClient) ListOpenPRs() ([]ReviewPR, error) {
	if f.prsErr != nil {
		return nil, f.prsErr
	}
	return f.prs, nil
}
func (f *fakeReviewGateClient) ListOpenIssues() ([]ReviewIssue, error) {
	if f.issuesErr != nil {
		return nil, f.issuesErr
	}
	return f.issues, nil
}
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
	text := FormatReviewQueueText(report)
	if !strings.HasSuffix(text, "next step: review PR #10\n") {
		t.Fatalf("review queue text missing final next step:\n%s", text)
	}
}

func TestBuildMergeQueueDryRunAndApply(t *testing.T) {
	repo, _ := ParseRepoRef("StatPan/gira")
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	client := &fakeReviewGateClient{repo: repo, prs: []ReviewPR{
		{Number: 20, Body: "Closes #20", ReviewDecision: "APPROVED", CheckStatus: "passing", UpdatedAt: "2026-04-30T00:00:00Z"},
		{Number: 21, Body: "", ReviewDecision: "", CheckStatus: "passing", UpdatedAt: "2026-04-30T00:00:00Z"},
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
		prs:    []ReviewPR{{Number: 30, Body: "Fixes #40", ReviewDecision: "", CheckStatus: "failing", UpdatedAt: "2026-04-30T00:00:00Z"}},
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

func TestExtractClosureIssueNumbers(t *testing.T) {
	issues := ExtractClosureIssueNumbers("Implements feature. Fixes #12 and resolves #13; closes #12")
	if len(issues) != 2 || issues[0] != 12 || issues[1] != 13 {
		t.Fatalf("unexpected closure issues: %+v", issues)
	}
}

func TestClassifyPRBlockersIncludesMissingClosureLink(t *testing.T) {
	blockers := classifyPRBlockers(ReviewPR{ReviewDecision: "APPROVED", CheckStatus: "passing", Body: "no closure keyword"})
	if len(blockers) != 1 || blockers[0] != BlockerPolicyViolation {
		t.Fatalf("unexpected blockers: %+v", blockers)
	}
}

func TestReviewQueuePolicyModesSeparateProviderFactsFromManagedEnforcement(t *testing.T) {
	repo, _ := ParseRepoRef("external/example")
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	pr := ReviewPR{Number: 70, Title: "Conventional external PR", CheckStatus: "passing", Body: "no Gira closure", UpdatedAt: "2026-04-30T00:00:00Z"}
	tests := []struct {
		name         string
		policy       ResolvedOperationPolicy
		wantBlockers []string
		wantMode     string
		wantEnforced bool
	}{
		{name: "observation is neutral", policy: ResolvedOperationPolicy{OperationMode: OperationModeObservation, DeliveryPolicy: DeliveryPolicyNone, Source: OperationPolicySourceUnconfigured}, wantBlockers: nil, wantMode: OperationModeObservation, wantEnforced: false},
		{name: "managed advisory reports policy findings without blocking", policy: ResolvedOperationPolicy{OperationMode: OperationModeManaged, DeliveryPolicy: DeliveryPolicyAdvisory, Source: "repo_local_contract"}, wantBlockers: nil, wantMode: OperationModeManaged, wantEnforced: false},
		{name: "managed required preserves strict behavior", policy: ResolvedOperationPolicy{OperationMode: OperationModeManaged, DeliveryPolicy: DeliveryPolicyRequired, Source: "repo_local_contract"}, wantBlockers: []string{BlockerMissingApproval, BlockerPolicyViolation}, wantMode: OperationModeManaged, wantEnforced: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeReviewGateClient{repo: repo, prs: []ReviewPR{pr}, policy: &tt.policy}
			report, err := BuildReviewQueue(client, now)
			if err != nil {
				t.Fatal(err)
			}
			if report.Policy != tt.policy || len(report.Items) != 1 {
				t.Fatalf("policy/report = %+v", report)
			}
			item := report.Items[0]
			if strings.Join(item.Blockers, ",") != strings.Join(tt.wantBlockers, ",") {
				t.Fatalf("blockers=%v want=%v findings=%+v", item.Blockers, tt.wantBlockers, item.Findings)
			}
			if len(item.Findings) != 2 {
				t.Fatalf("findings=%+v", item.Findings)
			}
			for _, finding := range item.Findings {
				if finding.FindingClass != ReviewFindingClassPolicy || finding.Enforced != tt.wantEnforced {
					t.Fatalf("finding=%+v want mode=%s enforced=%t", finding, tt.wantMode, tt.wantEnforced)
				}
			}
			text := FormatReviewQueueText(report)
			if !strings.Contains(text, "source="+tt.policy.Source) || !strings.Contains(text, "finding_class="+ReviewFindingClassPolicy) {
				t.Fatalf("human output omitted policy provenance/finding class:\n%s", text)
			}
		})
	}
}

func TestReviewQueueProviderFactsRemainBlockingAcrossModes(t *testing.T) {
	repo, _ := ParseRepoRef("external/example")
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	policy := ResolvedOperationPolicy{OperationMode: OperationModeObservation, DeliveryPolicy: DeliveryPolicyNone, Source: OperationPolicySourceUnconfigured}
	client := &fakeReviewGateClient{repo: repo, policy: &policy, prs: []ReviewPR{{Number: 71, CheckStatus: "failing", Body: "external", UpdatedAt: "2026-04-30T00:00:00Z"}}}
	report, err := BuildReleaseReadiness(client, now)
	if err != nil {
		t.Fatal(err)
	}
	if report.Ready || len(report.BlockingPRs) != 1 || !strings.Contains(strings.Join(report.BlockingPRs[0].Blockers, ","), BlockerFailingChecks) {
		t.Fatalf("provider fact was not blocking: %+v", report)
	}
	found := false
	for _, finding := range report.Findings {
		if finding.Code == BlockerFailingChecks && finding.FindingClass == ReviewFindingClassProvider && !finding.Enforced {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing provider finding: %+v", report.Findings)
	}
	if !strings.Contains(FormatReleaseReadinessText(report), "policy: mode=observation") {
		t.Fatalf("release human output omitted policy: %s", FormatReleaseReadinessText(report))
	}
}

func TestManagedLabelPolicyAcrossOperationModes(t *testing.T) {
	repo, _ := ParseRepoRef("external/example")
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name          string
		policy        ResolvedOperationPolicy
		wantPRBlocked bool
		wantReleaseOK bool
		wantEnforced  bool
	}{
		{name: "observation reports labels", policy: ResolvedOperationPolicy{OperationMode: OperationModeObservation, DeliveryPolicy: DeliveryPolicyNone, Source: OperationPolicySourceUnconfigured}, wantReleaseOK: true, wantEnforced: false},
		{name: "advisory reports labels", policy: ResolvedOperationPolicy{OperationMode: OperationModeManaged, DeliveryPolicy: DeliveryPolicyAdvisory, Source: "repo_local_contract"}, wantReleaseOK: true, wantEnforced: false},
		{name: "required enforces labels", policy: ResolvedOperationPolicy{OperationMode: OperationModeManaged, DeliveryPolicy: DeliveryPolicyRequired, Source: "repo_local_contract"}, wantPRBlocked: true, wantReleaseOK: false, wantEnforced: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeReviewGateClient{repo: repo, policy: &tt.policy, prs: []ReviewPR{{Number: 72, Body: "Closes #72", ReviewDecision: "APPROVED", CheckStatus: "passing", Labels: []string{"blocker"}, UpdatedAt: "2026-04-30T00:00:00Z"}}, issues: []ReviewIssue{{Number: 80, Labels: []string{"blocker"}}, {Number: 81, Labels: []string{"must-fix"}}}}
			queue, err := BuildReviewQueue(client, now)
			if err != nil {
				t.Fatal(err)
			}
			if (len(queue.Items[0].Blockers) > 0) != tt.wantPRBlocked {
				t.Fatalf("PR label blockers=%v want blocked=%t", queue.Items[0].Blockers, tt.wantPRBlocked)
			}
			prLabelFindings := 0
			for _, finding := range queue.Items[0].Findings {
				if finding.Code == BlockerUnresolvedBlocker {
					prLabelFindings++
					if finding.FindingClass != ReviewFindingClassPolicy || finding.Enforced != tt.wantEnforced {
						t.Fatalf("PR label finding=%+v want enforced=%t", finding, tt.wantEnforced)
					}
				}
			}
			if prLabelFindings != 1 {
				t.Fatalf("PR label finding count=%d want exactly one: %+v", prLabelFindings, queue.Items[0].Findings)
			}
			readiness, err := BuildReleaseReadiness(client, now)
			if err != nil {
				t.Fatal(err)
			}
			if readiness.Ready != tt.wantReleaseOK {
				t.Fatalf("release ready=%t want=%t report=%+v", readiness.Ready, tt.wantReleaseOK, readiness)
			}
			for _, finding := range readiness.Findings {
				if finding.Code == BlockerUnresolvedBlocker || finding.Code == "must_fix_issue" {
					if finding.FindingClass != ReviewFindingClassPolicy || finding.Enforced != tt.wantEnforced {
						t.Fatalf("issue label finding=%+v want enforced=%t", finding, tt.wantEnforced)
					}
				}
			}
		})
	}
}

func TestReviewGateFailsClosedOnPolicyAndProviderErrors(t *testing.T) {
	repo, _ := ParseRepoRef("external/example")
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if _, err := BuildReviewQueue(&fakeReviewGateClient{repo: repo, policyErr: fmt.Errorf("invalid policy")}, now); err == nil {
		t.Fatal("policy resolution error was not returned")
	}
	policy := ResolvedOperationPolicy{OperationMode: OperationModeObservation, DeliveryPolicy: DeliveryPolicyNone, Source: OperationPolicySourceUnconfigured}
	if _, err := BuildReviewQueue(&fakeReviewGateClient{repo: repo, policy: &policy, prsErr: fmt.Errorf("provider unavailable")}, now); err == nil {
		t.Fatal("provider read error was not returned")
	}
	if _, err := BuildReleaseReadiness(&fakeReviewGateClient{repo: repo, policy: &policy, prsErr: fmt.Errorf("provider unavailable")}, now); err == nil {
		t.Fatal("release provider read error was not returned")
	}
}
