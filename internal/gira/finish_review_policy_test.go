package gira

import "testing"

func TestFinishReviewEvidenceRequiresApprovalForCurrentHead(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	status := DevPRStatusResult{PRNumber: 220, ReviewDecision: "APPROVED", HeadSHA: "current-head"}
	policy := FinishReviewPolicy{Value: FinishReviewPolicyRequired, Source: "repo_config"}

	t.Run("stale approval blocks", func(t *testing.T) {
		runner := &finishRunner{outputs: map[string][][]byte{
			"gh api repos/StatPan/gira/pulls/220/reviews --paginate": {[]byte(`[{"state":"APPROVED","commit_id":"old-head"}]`)},
		}}
		evidence := finishReviewEvidence(repo, status, policy, runner)
		if evidence.Blocker != "review_approval_stale" || evidence.Status != "blocked" {
			t.Fatalf("unexpected stale review evidence: %+v", evidence)
		}
	})

	t.Run("current head approval passes", func(t *testing.T) {
		runner := &finishRunner{outputs: map[string][][]byte{
			"gh api repos/StatPan/gira/pulls/220/reviews --paginate": {[]byte(`[{"state":"APPROVED","commit_id":"current-head"}]`)},
		}}
		evidence := finishReviewEvidence(repo, status, policy, runner)
		if evidence.Status != "approved" || evidence.ApprovalSHA != "current-head" || evidence.Blocker != "" {
			t.Fatalf("unexpected current-head review evidence: %+v", evidence)
		}
	})
}

func TestFinishReviewEvidenceReportsPolicyAndMissingApprovalSeparately(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	status := DevPRStatusResult{PRNumber: 220, ReviewDecision: ""}
	if evidence := finishReviewEvidence(repo, status, FinishReviewPolicy{Value: FinishReviewPolicyMissing}, &finishRunner{}); evidence.Blocker != "review_policy_not_configured" {
		t.Fatalf("policy gap should be distinct: %+v", evidence)
	}
	if evidence := finishReviewEvidence(repo, status, FinishReviewPolicy{Value: FinishReviewPolicyRequired}, &finishRunner{}); evidence.Blocker != "review_required_but_absent" {
		t.Fatalf("missing approval should be distinct: %+v", evidence)
	}
	if evidence := finishReviewEvidence(repo, status, FinishReviewPolicy{Value: FinishReviewPolicyNone}, &finishRunner{}); evidence.Status != "not_required" || evidence.Blocker != "" {
		t.Fatalf("explicit no-review policy should remain supported: %+v", evidence)
	}
}
