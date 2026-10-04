package gira

import (
	"encoding/json"
	"testing"
)

func TestQARecordedFinishLifecycleRejectsUnsafePreflight(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*testing.T, *recordedFinishLifecycleRunner)
		wantBlock string
	}{
		{
			name: "failed checks",
			mutate: func(_ *testing.T, runner *recordedFinishLifecycleRunner) {
				runner.initial.CheckRunStatus = "completed"
				runner.initial.CheckConclusion = "failure"
			},
			wantBlock: "checks",
		},
		{
			name: "pending checks",
			mutate: func(_ *testing.T, runner *recordedFinishLifecycleRunner) {
				runner.initial.CheckRunStatus = "in_progress"
				runner.initial.CheckConclusion = ""
			},
			wantBlock: "checks_pending",
		},
		{
			name: "unavailable checks",
			mutate: func(_ *testing.T, runner *recordedFinishLifecycleRunner) {
				runner.initial.ChecksUnavailable = true
			},
			wantBlock: "checks",
		},
		{
			name: "unrecognized base branch cannot use the recorded lane",
			mutate: func(_ *testing.T, runner *recordedFinishLifecycleRunner) {
				runner.initial.BaseRef = "release/2"
			},
			wantBlock: "review_required_but_absent",
		},
		{
			name: "untrusted branch binding fails before merge",
			mutate: func(_ *testing.T, runner *recordedFinishLifecycleRunner) {
				runner.initial.HeadRef = ""
				runner.revalidation.HeadRef = ""
			},
			wantBlock: "review_evidence_unavailable",
		},
		{
			name: "stale recorded review",
			mutate: func(t *testing.T, runner *recordedFinishLifecycleRunner) {
				stale := runner.initial.Reviews[0]
				stale = qaRewriteLifecycleReceipt(t, stale, func(receipt *IndependentReviewReceipt) {
					receipt.HeadSHA = qaRecordedReviewHead2
				})
				runner.initial.Reviews = []finishReview{stale}
			},
			wantBlock: "recorded_review_stale",
		},
		{
			name: "current blocked recorded review",
			mutate: func(t *testing.T, runner *recordedFinishLifecycleRunner) {
				blocked := runner.initial.Reviews[0]
				blocked = qaRewriteLifecycleReceipt(t, blocked, func(receipt *IndependentReviewReceipt) {
					receipt.Verdict = "BLOCKED"
					receipt.Findings = []IndependentReviewFinding{{
						ID: "lifecycle-blocker", Severity: "blocking", Status: "open",
						Summary:      "A lifecycle blocker remains open.",
						EvidenceRefs: []string{"https://evidence.example.invalid/lifecycle/blocker"},
					}}
				})
				runner.initial.Reviews = []finishReview{blocked}
			},
			wantBlock: "recorded_review_blocked",
		},
		{
			name: "main cannot use the recorded-review lane",
			mutate: func(t *testing.T, runner *recordedFinishLifecycleRunner) {
				runner.initial.BaseRef = "main"
				mainReceipt := runner.initial.Reviews[0]
				mainReceipt = qaRewriteLifecycleReceipt(t, mainReceipt, func(receipt *IndependentReviewReceipt) {
					receipt.BaseRef = "main"
				})
				runner.initial.Reviews = []finishReview{mainReceipt}
			},
			wantBlock: "review_required_but_absent",
		},
	}

	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := newRecordedFinishLifecycleRunner(t)
			tt.mutate(t, runner)

			result, err := FinishWorkWithOptions(repo, 219, false, 0, WorkFinishOptions{}, runner)
			if err == nil {
				t.Fatal("unsafe finish preflight should block apply")
			}
			if !containsString(result.Blockers, tt.wantBlock) {
				t.Fatalf("blockers = %v, want %q (error %v)", result.Blockers, tt.wantBlock, err)
			}
			if result.Merged || len(runner.mergeCalls) != 0 {
				t.Fatalf("rejected finish must never merge: merged=%t merge calls=%v", result.Merged, runner.mergeCalls)
			}
			if len(runner.mutations) != 0 {
				t.Fatalf("rejected finish must not mutate GitHub state: %v", runner.mutations)
			}
		})
	}
}

func TestQARecordedFinishLifecycleRechecksEvidenceBeforeMerge(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *recordedFinishLifecycleRunner)
	}{
		{
			name: "changed head",
			mutate: func(_ *testing.T, runner *recordedFinishLifecycleRunner) {
				runner.revalidation.HeadSHA = qaRecordedReviewHead2
			},
		},
		{
			name: "changed base",
			mutate: func(_ *testing.T, runner *recordedFinishLifecycleRunner) {
				runner.revalidation.BaseSHA = qaRecordedReviewHead3
			},
		},
		{
			name: "new changes requested review",
			mutate: func(_ *testing.T, runner *recordedFinishLifecycleRunner) {
				login := runner.initial.Reviews[0].User.Login
				runner.revalidation.Reviews = []finishReview{qaNativeReview(901, login, "CHANGES_REQUESTED", runner.revalidation.HeadSHA)}
			},
		},
		{
			name: "fresh receipt is blocked",
			mutate: func(t *testing.T, runner *recordedFinishLifecycleRunner) {
				blocked := runner.revalidation.Reviews[0]
				blocked = qaRewriteLifecycleReceipt(t, blocked, func(receipt *IndependentReviewReceipt) {
					receipt.Verdict = "BLOCKED"
					receipt.Findings = []IndependentReviewFinding{{
						ID: "fresh-lifecycle-blocker", Severity: "blocking", Status: "open",
						Summary:      "Fresh review found a blocker.",
						EvidenceRefs: []string{"https://evidence.example.invalid/lifecycle/fresh-blocker"},
					}}
				})
				runner.revalidation.Reviews = []finishReview{blocked}
			},
		},
	}

	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := newRecordedFinishLifecycleRunner(t)
			tt.mutate(t, runner)

			result, err := FinishWorkWithOptions(repo, 219, false, 0, WorkFinishOptions{}, runner)
			if err == nil {
				t.Fatal("changed pre-merge review evidence should block apply")
			}
			if !containsString(result.Blockers, "review_evidence_unavailable") {
				t.Fatalf("blockers = %v, want review_evidence_unavailable (error %v)", result.Blockers, err)
			}
			if runner.pullReads < 2 {
				t.Fatalf("expected initial status and fresh pre-merge status reads, got %d", runner.pullReads)
			}
			if result.Merged || len(runner.mergeCalls) != 0 || len(runner.mutations) != 0 {
				t.Fatalf("failed fresh evidence check must prevent merge and mutations: merged=%t merge=%v mutations=%v", result.Merged, runner.mergeCalls, runner.mutations)
			}
		})
	}
}

func TestQARecordedFinishLifecycleDraftApplyStopsAfterReady(t *testing.T) {
	runner := newRecordedFinishLifecycleRunner(t)
	runner.initial.Draft = true
	runner.revalidation.Draft = true

	result, err := FinishWorkWithOptions(RepoRef{Owner: "StatPan", Name: "gira"}, 219, false, 0, WorkFinishOptions{}, runner)
	if err != nil {
		t.Fatalf("Draft apply should perform only the ready transition: %v", err)
	}
	if !containsCall(runner.mutations, "gh pr ready 220 --repo StatPan/gira") {
		t.Fatalf("missing the bounded ready transition: %v", runner.mutations)
	}
	if len(runner.mutations) != 1 || !finishActionStatus(result.Actions, "pr:ready", "applied") {
		t.Fatalf("Draft apply should record only the ready transition: mutations=%v actions=%+v", runner.mutations, result.Actions)
	}
	if result.Merged || len(runner.mergeCalls) != 0 {
		t.Fatalf("Draft apply must stop before merge: merged=%t merge=%v", result.Merged, runner.mergeCalls)
	}
	if !result.LocalSync.Skipped || result.LocalSync.Reason != "ready_transition_only" {
		t.Fatalf("Draft transition should stop local sync too: %+v", result.LocalSync)
	}
}

func qaRewriteLifecycleReceipt(t *testing.T, review finishReview, mutate func(*IndependentReviewReceipt)) finishReview {
	t.Helper()
	receipt, marked, err := decodeIndependentReviewReceipt(review.Body)
	if err != nil || !marked {
		t.Fatalf("default lifecycle fixture must contain a valid marked receipt (marked=%t err=%v)", marked, err)
	}
	mutate(&receipt)
	body, err := json.Marshal(receipt)
	if err != nil {
		t.Fatalf("encode changed fixture receipt: %v", err)
	}
	review.Body = IndependentReviewReceiptMarker + "\n" + string(body)
	review.CommitID = receipt.HeadSHA
	return review
}
