package gira

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

const (
	qaRecordedReviewHead1 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	qaRecordedReviewHead2 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	qaRecordedReviewHead3 = "cccccccccccccccccccccccccccccccccccccccc"
	qaRecordedReviewBase1 = "dddddddddddddddddddddddddddddddddddddddd"
)

// qaRecordedReviewRunner is intentionally limited to one read-only review
// endpoint. Its synthetic identities and evidence URLs are test fixtures, not
// human reviewers or production review evidence.
type qaRecordedReviewRunner struct {
	reviews []finishReview
	err     error
	calls   int
}

func (r *qaRecordedReviewRunner) Run(name string, args ...string) ([]byte, error) {
	r.calls++
	if name != "gh" || len(args) != 4 || args[0] != "api" || args[1] != "repos/StatPan/gira/pulls/220/reviews" || args[2] != "--paginate" || args[3] != "--slurp" {
		return nil, fmt.Errorf("unexpected QA review fixture request")
	}
	if r.err != nil {
		return nil, r.err
	}
	return json.Marshal([][]finishReview{r.reviews})
}

func TestQARecordedReviewKeepsNativeCurrentHeadApprovalOnMain(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	status := qaRecordedReviewStatus(qaRecordedReviewHead1, qaRecordedReviewBase1, "main", "APPROVED")
	runner := &qaRecordedReviewRunner{reviews: []finishReview{
		qaNativeReview(11, "native-reviewer-fixture", "APPROVED", qaRecordedReviewHead1),
	}}

	evidence := finishReviewEvidence(repo, status, FinishReviewPolicy{Value: FinishReviewPolicyRequired}, runner)
	if evidence.Status != "approved" || evidence.Source != "github_approval" || evidence.ApprovalSHA != qaRecordedReviewHead1 || evidence.Blocker != "" {
		t.Fatalf("required native review on main must retain its existing current-head behavior: %+v", evidence)
	}
	if runner.calls != 1 {
		t.Fatalf("native lane review reads = %d, want 1", runner.calls)
	}
}

func TestQARecordedReviewAcceptsTrustedTypedExactHeadAndBaseGO(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	status := qaRecordedReviewStatus(qaRecordedReviewHead1, qaRecordedReviewBase1, "dev", "REVIEW_REQUIRED")
	receipt := qaIndependentReceipt(status, "qa-recorder")
	runner := &qaRecordedReviewRunner{reviews: []finishReview{qaReceiptReview(t, 21, "qa-recorder", receipt)}}

	evidence := finishReviewEvidence(repo, status, qaRecordedPolicy("qa-recorder"), runner)
	if evidence.Status != "independent_recorded" || evidence.Blocker != "" || evidence.ReviewID != 21 || evidence.Recorder != "qa-recorder" || evidence.ReviewerID != receipt.Reviewer.ID {
		t.Fatalf("trusted exact-head/base independent GO should qualify on dev: %+v", evidence)
	}
	if evidence.ApprovalSHA != "" || evidence.Source == "github_approval" {
		t.Fatalf("recorded independent review must not be mislabeled as native approval: %+v", evidence)
	}

	t.Run("recorded lane is unavailable on main", func(t *testing.T) {
		mainStatus := qaRecordedReviewStatus(qaRecordedReviewHead1, qaRecordedReviewBase1, "main", "REVIEW_REQUIRED")
		mainReceipt := qaIndependentReceipt(mainStatus, "qa-recorder")
		mainRunner := &qaRecordedReviewRunner{reviews: []finishReview{qaReceiptReview(t, 22, "qa-recorder", mainReceipt)}}
		mainEvidence := finishReviewEvidence(repo, mainStatus, qaRecordedPolicy("qa-recorder"), mainRunner)
		qaRequireBlocked(t, mainEvidence, "review_required_but_absent")
	})
}

func TestQARecordedReviewRejectsGenericSelfReviewAndUntrustedRecorder(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	status := qaRecordedReviewStatus(qaRecordedReviewHead1, qaRecordedReviewBase1, "dev", "REVIEW_REQUIRED")

	t.Run("generic prose is not a receipt", func(t *testing.T) {
		review := qaNativeReview(31, "qa-recorder", "COMMENTED", qaRecordedReviewHead1)
		review.Body = "I reviewed this change and it looks good."
		runner := &qaRecordedReviewRunner{reviews: []finishReview{review}}
		evidence := finishReviewEvidence(repo, status, qaRecordedPolicy("qa-recorder"), runner)
		qaRequireBlocked(t, evidence, "review_required_but_absent")
	})

	t.Run("untrusted account cannot record a valid-looking receipt", func(t *testing.T) {
		receipt := qaIndependentReceipt(status, "untrusted-recorder")
		runner := &qaRecordedReviewRunner{reviews: []finishReview{qaReceiptReview(t, 32, "untrusted-recorder", receipt)}}
		evidence := finishReviewEvidence(repo, status, qaRecordedPolicy("qa-recorder"), runner)
		qaRequireBlocked(t, evidence, "review_required_but_absent")
	})
}

func TestQARecordedReviewRejectsStaleAndForeignBindings(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	status := qaRecordedReviewStatus(qaRecordedReviewHead2, qaRecordedReviewBase1, "dev", "REVIEW_REQUIRED")
	policy := qaRecordedPolicy("qa-recorder")

	tests := []struct {
		name      string
		mutate    func(*IndependentReviewReceipt)
		wantBlock string
		commitSHA string
	}{
		{name: "old head", mutate: func(r *IndependentReviewReceipt) { r.HeadSHA = qaRecordedReviewHead1 }, wantBlock: "recorded_review_stale", commitSHA: qaRecordedReviewHead1},
		{name: "review metadata commit differs from receipt head", wantBlock: "recorded_review_invalid", commitSHA: qaRecordedReviewHead1},
		{name: "wrong repository", mutate: func(r *IndependentReviewReceipt) { r.Repository = "Elsewhere/gira" }, wantBlock: "recorded_review_invalid"},
		{name: "wrong pull request", mutate: func(r *IndependentReviewReceipt) { r.PullRequest = 221 }, wantBlock: "recorded_review_invalid"},
		{name: "wrong base ref", mutate: func(r *IndependentReviewReceipt) { r.BaseRef = "main" }, wantBlock: "recorded_review_stale"},
		{name: "wrong base sha", mutate: func(r *IndependentReviewReceipt) { r.BaseSHA = qaRecordedReviewHead3 }, wantBlock: "recorded_review_stale"},
	}
	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			receipt := qaIndependentReceipt(status, "qa-recorder")
			if test.mutate != nil {
				test.mutate(&receipt)
			}
			commit := test.commitSHA
			if commit == "" {
				commit = status.HeadSHA
			}
			runner := &qaRecordedReviewRunner{reviews: []finishReview{qaReceiptReview(t, int64(40+i), "qa-recorder", receipt, commit)}}
			evidence := finishReviewEvidence(repo, status, policy, runner)
			qaRequireBlocked(t, evidence, test.wantBlock)
		})
	}
}

func TestQARecordedReviewRejectsMalformedOrIncompleteReceipts(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	status := qaRecordedReviewStatus(qaRecordedReviewHead1, qaRecordedReviewBase1, "dev", "REVIEW_REQUIRED")
	policy := qaRecordedPolicy("qa-recorder")

	tests := []struct {
		name    string
		mutate  func(*IndependentReviewReceipt)
		rawBody string
	}{
		{name: "unsupported schema", mutate: func(r *IndependentReviewReceipt) { r.SchemaVersion = "gira-independent-review/v99" }},
		{name: "missing reviewer kind", mutate: func(r *IndependentReviewReceipt) { r.Reviewer.Kind = "" }},
		{name: "missing limitation", mutate: func(r *IndependentReviewReceipt) { r.Limitations = nil }},
		{name: "same reviewer and implementer", mutate: func(r *IndependentReviewReceipt) { r.Implementer.ID = r.Reviewer.ID }},
		{name: "non-inspectable evidence reference", mutate: func(r *IndependentReviewReceipt) { r.EvidenceRefs = []string{"file:///tmp/private-review"} }},
		{name: "BLOCKED without an open blocker", mutate: func(r *IndependentReviewReceipt) { r.Verdict = "BLOCKED" }},
		{name: "BLOCKED with trailing whitespace is not canonical", mutate: func(r *IndependentReviewReceipt) { r.Verdict = "BLOCKED " }},
		{name: "BLOCKED with leading whitespace is not canonical", mutate: func(r *IndependentReviewReceipt) { r.Verdict = " BLOCKED" }},
		{name: "GO with trailing whitespace is not canonical", mutate: func(r *IndependentReviewReceipt) { r.Verdict = "GO " }},
		{name: "GO with open blocking finding", mutate: func(r *IndependentReviewReceipt) {
			r.Findings = []IndependentReviewFinding{{ID: "finding-open", Severity: "blocking", Status: "open", Summary: "A blocking issue remains.", EvidenceRefs: []string{"https://evidence.example.invalid/findings/open"}}}
		}},
		{name: "unknown schema field", rawBody: IndependentReviewReceiptMarker + `{"schema_version":"gira-independent-review/v1","unexpected":"must fail"}`},
		{name: "trailing JSON", rawBody: IndependentReviewReceiptMarker + `{} {}`},
	}
	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var review finishReview
			if test.rawBody != "" {
				review = qaRawRecordedReview(int64(60+i), "qa-recorder", status.HeadSHA, test.rawBody)
			} else {
				receipt := qaIndependentReceipt(status, "qa-recorder")
				test.mutate(&receipt)
				review = qaReceiptReview(t, int64(60+i), "qa-recorder", receipt)
			}
			runner := &qaRecordedReviewRunner{reviews: []finishReview{review}}
			evidence := finishReviewEvidence(repo, status, policy, runner)
			qaRequireBlocked(t, evidence, "recorded_review_invalid")
		})
	}
}

func TestQARecordedReviewRequiresBlockingFindingsToStayBlockedUntilResolved(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	status := qaRecordedReviewStatus(qaRecordedReviewHead1, qaRecordedReviewBase1, "dev", "REVIEW_REQUIRED")
	policy := qaRecordedPolicy("qa-recorder")
	blocked := qaIndependentReceipt(status, "qa-recorder")
	blocked.Verdict = "BLOCKED"
	blocked.Findings = []IndependentReviewFinding{{ID: "binding-race", Severity: "blocking", Status: "open", Summary: "Head can change before merge.", EvidenceRefs: []string{"https://evidence.example.invalid/findings/binding-race"}}}

	t.Run("new empty GO cannot erase unresolved history", func(t *testing.T) {
		newHeadStatus := qaRecordedReviewStatus(qaRecordedReviewHead2, qaRecordedReviewBase1, "dev", "REVIEW_REQUIRED")
		currentGO := qaIndependentReceipt(newHeadStatus, "qa-recorder")
		runner := &qaRecordedReviewRunner{reviews: []finishReview{
			qaReceiptReview(t, 71, "qa-recorder", blocked),
			qaReceiptReview(t, 72, "qa-recorder", currentGO),
		}}
		evidence := finishReviewEvidence(repo, newHeadStatus, policy, runner)
		qaRequireBlocked(t, evidence, "recorded_review_blocked")
	})

	t.Run("explicit evidenced resolution permits a later-head GO", func(t *testing.T) {
		middleStatus := qaRecordedReviewStatus(qaRecordedReviewHead2, qaRecordedReviewBase1, "dev", "REVIEW_REQUIRED")
		finalStatus := qaRecordedReviewStatus(qaRecordedReviewHead3, qaRecordedReviewBase1, "dev", "REVIEW_REQUIRED")
		resolved := qaIndependentReceipt(middleStatus, "qa-recorder")
		resolved.Findings = []IndependentReviewFinding{{ID: "binding-race", Severity: "blocking", Status: "resolved", Summary: "The exact-head merge guard is now verified.", EvidenceRefs: []string{"https://evidence.example.invalid/findings/binding-race-resolution"}}}
		finalGO := qaIndependentReceipt(finalStatus, "qa-recorder")
		runner := &qaRecordedReviewRunner{reviews: []finishReview{
			qaReceiptReview(t, 73, "qa-recorder", blocked),
			qaReceiptReview(t, 74, "qa-recorder", resolved),
			qaReceiptReview(t, 75, "qa-recorder", finalGO),
		}}
		evidence := finishReviewEvidence(repo, finalStatus, policy, runner)
		if evidence.Status != "independent_recorded" || evidence.Blocker != "" || evidence.ReviewID != 75 {
			t.Fatalf("evidenced resolution followed by a fresh-head GO should qualify: %+v", evidence)
		}
	})
}

func TestQARecordedReviewHistoryMutationIsAtomic(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	status := qaRecordedReviewStatus(qaRecordedReviewHead3, qaRecordedReviewBase1, "dev", "REVIEW_REQUIRED")
	policy := qaRecordedPolicy("qa-recorder")
	priorBlocked := qaIndependentReceipt(qaRecordedReviewStatus(qaRecordedReviewHead1, qaRecordedReviewBase1, "dev", "REVIEW_REQUIRED"), "qa-recorder")
	priorBlocked.Verdict = "BLOCKED"
	priorBlocked.Findings = []IndependentReviewFinding{{ID: "finding-a", Severity: "blocking", Status: "open", Summary: "A blocker is still open.", EvidenceRefs: []string{"https://evidence.example.invalid/findings/a-open"}}}

	malformed := qaIndependentReceipt(status, "qa-recorder")
	malformed.Findings = []IndependentReviewFinding{
		{ID: "finding-a", Severity: "blocking", Status: "resolved", Summary: "Tentative resolution of A.", EvidenceRefs: []string{"https://evidence.example.invalid/findings/a-tentative-resolution"}},
		{ID: "finding-b", Severity: "blocking", Status: "resolved", Summary: "B has no open history to resolve.", EvidenceRefs: []string{"https://evidence.example.invalid/findings/b-nonexistent-resolution"}},
	}
	finalGO := qaIndependentReceipt(status, "qa-recorder")
	finalGO.SupersedesReviewIDs = []int64{122}
	runner := &qaRecordedReviewRunner{reviews: []finishReview{
		qaReceiptReview(t, 121, "qa-recorder", priorBlocked),
		qaReceiptReview(t, 122, "qa-recorder", malformed),
		qaReceiptReview(t, 123, "qa-recorder", finalGO),
	}}

	evidence := finishReviewEvidence(repo, status, policy, runner)
	qaRequireBlocked(t, evidence, "recorded_review_blocked")
}

func TestQARecordedReviewChangesRequestedIsTrackedPerReviewer(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	status := qaRecordedReviewStatus(qaRecordedReviewHead1, qaRecordedReviewBase1, "dev", "APPROVED")
	policy := qaRecordedPolicy("qa-recorder")
	otherActorApproval := qaNativeReview(82, "reviewer-b-fixture", "APPROVED", status.HeadSHA)

	t.Run("another actor cannot clear an active request", func(t *testing.T) {
		changeRequest := qaNativeReview(81, "reviewer-a-fixture", "CHANGES_REQUESTED", qaRecordedReviewHead1)
		runner := &qaRecordedReviewRunner{reviews: []finishReview{changeRequest, otherActorApproval}}
		evidence := finishReviewEvidence(repo, status, policy, runner)
		qaRequireBlocked(t, evidence, "review_changes_requested")
	})

	t.Run("the requesting reviewer can clear their own prior request", func(t *testing.T) {
		changeRequest := qaNativeReview(83, "reviewer-a-fixture", "CHANGES_REQUESTED", qaRecordedReviewHead1)
		clearedBySameReviewer := qaNativeReview(84, "reviewer-a-fixture", "APPROVED", qaRecordedReviewHead1)
		runner := &qaRecordedReviewRunner{reviews: []finishReview{changeRequest, clearedBySameReviewer}}
		evidence := finishReviewEvidence(repo, status, policy, runner)
		if evidence.Status != "approved" || evidence.Source != "github_approval" || evidence.Blocker != "" {
			t.Fatalf("a later native approval from the same reviewer should clear their prior request: %+v", evidence)
		}
	})
}

func TestQARecordedReviewRejectsMalformedOrDuplicateOrderingMetadata(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	status := qaRecordedReviewStatus(qaRecordedReviewHead1, qaRecordedReviewBase1, "dev", "APPROVED")
	nativeApproval := qaNativeReview(110, "native-reviewer-fixture", "APPROVED", status.HeadSHA)
	receipt := qaIndependentReceipt(status, "qa-recorder")
	duplicateReceipt := qaReceiptReview(t, 110, "qa-recorder", receipt)
	duplicateReceipt.SubmittedAt = "not-an-rfc3339-timestamp"
	runner := &qaRecordedReviewRunner{reviews: []finishReview{nativeApproval, duplicateReceipt}}

	evidence := finishReviewEvidence(repo, status, qaRecordedPolicy("qa-recorder"), runner)
	qaRequireBlocked(t, evidence, "review_evidence_unavailable")
}

func TestQARecordedReviewRequiresSameReviewerFindingScope(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	status := qaRecordedReviewStatus(qaRecordedReviewHead2, qaRecordedReviewBase1, "dev", "REVIEW_REQUIRED")
	policy := qaRecordedPolicy("qa-recorder")
	blockedByReviewerA := qaIndependentReceipt(qaRecordedReviewStatus(qaRecordedReviewHead1, qaRecordedReviewBase1, "dev", "REVIEW_REQUIRED"), "qa-recorder")
	blockedByReviewerA.Reviewer.ID = "ai-reviewer-fixture-a"
	blockedByReviewerA.Verdict = "BLOCKED"
	blockedByReviewerA.Findings = []IndependentReviewFinding{{ID: "same-finding", Severity: "blocking", Status: "open", Summary: "A blocker remains.", EvidenceRefs: []string{"https://evidence.example.invalid/findings/shared-id"}}}
	resolvedByReviewerB := qaIndependentReceipt(status, "qa-recorder")
	resolvedByReviewerB.Reviewer.ID = "ai-reviewer-fixture-b"
	resolvedByReviewerB.Findings = []IndependentReviewFinding{{ID: "same-finding", Severity: "blocking", Status: "resolved", Summary: "A different reviewer cannot resolve another reviewer's finding.", EvidenceRefs: []string{"https://evidence.example.invalid/findings/unauthorized-resolution"}}}

	t.Run("same ID from a different reviewer is not resolution", func(t *testing.T) {
		runner := &qaRecordedReviewRunner{reviews: []finishReview{
			qaReceiptReview(t, 91, "qa-recorder", blockedByReviewerA),
			qaReceiptReview(t, 92, "qa-recorder", resolvedByReviewerB),
		}}
		evidence := finishReviewEvidence(repo, status, policy, runner)
		qaRequireBlocked(t, evidence, "recorded_review_invalid")
	})

	t.Run("delimiter-bearing IDs cannot collide across reviewer scopes", func(t *testing.T) {
		collisionBlocked := qaIndependentReceipt(qaRecordedReviewStatus(qaRecordedReviewHead1, qaRecordedReviewBase1, "dev", "REVIEW_REQUIRED"), "qa-recorder")
		collisionBlocked.Reviewer.ID = "reviewer-a|reviewer-b"
		collisionBlocked.Verdict = "BLOCKED"
		collisionBlocked.Findings = []IndependentReviewFinding{{ID: "finding-c", Severity: "blocking", Status: "open", Summary: "Open under the first reviewer identity.", EvidenceRefs: []string{"https://evidence.example.invalid/findings/collision-open"}}}
		collisionResolution := qaIndependentReceipt(status, "qa-recorder")
		collisionResolution.Reviewer.ID = "reviewer-a"
		collisionResolution.Findings = []IndependentReviewFinding{{ID: "reviewer-b|finding-c", Severity: "blocking", Status: "resolved", Summary: "This distinct scoped finding must not collide.", EvidenceRefs: []string{"https://evidence.example.invalid/findings/collision-resolution"}}}
		runner := &qaRecordedReviewRunner{reviews: []finishReview{
			qaReceiptReview(t, 93, "qa-recorder", collisionBlocked),
			qaReceiptReview(t, 94, "qa-recorder", collisionResolution),
		}}
		evidence := finishReviewEvidence(repo, status, policy, runner)
		if evidence.Status == "independent_recorded" || evidence.Blocker == "" {
			t.Fatalf("finding-scope delimiter collision cleared a different reviewer's blocker: %+v", evidence)
		}
	})
}

func TestQARecordedReviewMalformedReceiptNeedsSameRecorderReplacement(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	status := qaRecordedReviewStatus(qaRecordedReviewHead1, qaRecordedReviewBase1, "dev", "REVIEW_REQUIRED")
	malformed := qaRawRecordedReview(101, "qa-recorder", status.HeadSHA, IndependentReviewReceiptMarker+`{"schema_version":"unsupported"}`)

	t.Run("unreplaced malformed history blocks", func(t *testing.T) {
		runner := &qaRecordedReviewRunner{reviews: []finishReview{malformed}}
		evidence := finishReviewEvidence(repo, status, qaRecordedPolicy("qa-recorder"), runner)
		qaRequireBlocked(t, evidence, "recorded_review_invalid")
	})

	t.Run("explicit same-recorder supersession repairs the malformed record", func(t *testing.T) {
		replacement := qaIndependentReceipt(status, "qa-recorder")
		replacement.SupersedesReviewIDs = []int64{101}
		runner := &qaRecordedReviewRunner{reviews: []finishReview{
			malformed,
			qaReceiptReview(t, 102, "qa-recorder", replacement),
		}}
		evidence := finishReviewEvidence(repo, status, qaRecordedPolicy("qa-recorder"), runner)
		if evidence.Status != "independent_recorded" || evidence.ReviewID != 102 || evidence.Blocker != "" {
			t.Fatalf("valid same-recorder explicit replacement should clear malformed receipt: %+v", evidence)
		}
	})

	t.Run("another trusted recorder cannot supersede it", func(t *testing.T) {
		replacement := qaIndependentReceipt(status, "qa-recorder-b")
		replacement.SupersedesReviewIDs = []int64{101}
		runner := &qaRecordedReviewRunner{reviews: []finishReview{
			malformed,
			qaReceiptReview(t, 103, "qa-recorder-b", replacement),
		}}
		evidence := finishReviewEvidence(repo, status, qaRecordedPolicy("qa-recorder", "qa-recorder-b"), runner)
		qaRequireBlocked(t, evidence, "recorded_review_invalid")
	})
}

func qaRecordedReviewStatus(headSHA, baseSHA, baseRef, decision string) DevPRStatusResult {
	return DevPRStatusResult{
		PRNumber:       220,
		ReviewDecision: decision,
		HeadSHA:        headSHA,
		BaseSHA:        baseSHA,
		Binding:        DevPRBinding{BaseRef: baseRef},
	}
}

func qaRecordedPolicy(recorders ...string) FinishReviewPolicy {
	return FinishReviewPolicy{
		Value:  FinishReviewPolicyRecordedIndependent,
		Source: "qa_authoritative_base_fixture",
		RecordedReview: &RecordedReviewConfig{
			AllowedRecorders:    append([]string(nil), recorders...),
			AllowedBaseBranches: []string{"dev"},
		},
	}
}

func qaIndependentReceipt(status DevPRStatusResult, recorder string) IndependentReviewReceipt {
	return IndependentReviewReceipt{
		SchemaVersion: IndependentReviewReceiptSchema,
		Repository:    "StatPan/gira",
		PullRequest:   status.PRNumber,
		HeadSHA:       status.HeadSHA,
		BaseRef:       status.Binding.BaseRef,
		BaseSHA:       status.BaseSHA,
		Verdict:       "GO",
		Reviewer: IndependentReviewIdentity{
			ID:     "ai-reviewer-fixture-session-01",
			Kind:   "ai",
			RunRef: "https://evidence.example.invalid/runs/reviewer-session-01",
		},
		Implementer: IndependentReviewIdentity{
			ID:     "ai-implementer-fixture-session-02",
			Kind:   "ai",
			RunRef: "https://evidence.example.invalid/runs/implementer-session-02",
		},
		Recorder:    recorder,
		Independent: true,
		EvidenceRefs: []string{
			"https://evidence.example.invalid/reviews/fixture-diff-and-tests",
		},
		Findings: []IndependentReviewFinding{},
		Limitations: []string{
			"Synthetic adversarial test fixture; not a human review or production approval.",
		},
	}
}

func qaReceiptReview(t *testing.T, id int64, login string, receipt IndependentReviewReceipt, commitOverride ...string) finishReview {
	t.Helper()
	body, err := json.Marshal(receipt)
	if err != nil {
		t.Fatalf("marshal synthetic receipt: %v", err)
	}
	commitSHA := receipt.HeadSHA
	if len(commitOverride) > 0 {
		commitSHA = commitOverride[0]
	}
	review := qaNativeReview(id, login, "COMMENTED", commitSHA)
	review.Body = IndependentReviewReceiptMarker + "\n" + string(body)
	return review
}

func qaRawRecordedReview(id int64, login, commitSHA, body string) finishReview {
	review := qaNativeReview(id, login, "COMMENTED", commitSHA)
	review.Body = body
	return review
}

func qaNativeReview(id int64, login, state, commitSHA string) finishReview {
	review := finishReview{
		ID:          id,
		State:       state,
		CommitID:    commitSHA,
		SubmittedAt: time.Date(2026, time.October, 4, 0, 0, 0, 0, time.UTC).Add(time.Duration(id) * time.Second).Format(time.RFC3339Nano),
	}
	review.User.Login = login
	return review
}

func qaRequireBlocked(t *testing.T, evidence FinishReviewEvidence, blocker string) {
	t.Helper()
	if evidence.Status != "blocked" || evidence.Blocker != blocker {
		t.Fatalf("expected fail-closed blocker %q, got %+v", blocker, evidence)
	}
	if evidence.ApprovalSHA != "" {
		t.Fatalf("blocked independent-review evidence must not expose approval SHA: %+v", evidence)
	}
}
