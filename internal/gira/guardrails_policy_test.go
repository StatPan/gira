package gira

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadGuardrailsPolicyValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "guardrails.yaml")
	if err := os.WriteFile(path, []byte(`branch_protection:
  main:
    required_approving_review_count: 2
    require_code_owner_reviews: true
    required_status_checks_strict: true
    allow_force_pushes: false
    allow_deletions: false
rulesets:
  - name: baseline
    target: branch
    enforcement: active
`), 0o644); err != nil {
		t.Fatal(err)
	}
	policy, err := LoadGuardrailsPolicy(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if policy.BranchProtection["main"].RequiredApprovingReviewCount != 2 {
		t.Fatalf("unexpected review count")
	}
}

func TestLoadGuardrailsPolicyInvalidPattern(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "guardrails.yaml")
	_ = os.WriteFile(path, []byte("branch_protection:\n  '': {}\n"), 0o644)
	_, err := LoadGuardrailsPolicy(path)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGuardrailsBlocksForcePushAndDeletionRelaxation(t *testing.T) {
	policy := GuardrailsPolicy{BranchProtection: map[string]GuardrailsBranchProtection{
		"main": {
			RequiredApprovingReviewCount: 1,
			RequireCodeOwnerReviews:      true,
			RequiredStatusChecksStrict:   true,
			AllowForcePushes:             true,
			AllowDeletions:               true,
		},
	}}
	current := GuardrailsState{BranchProtection: map[string]GuardrailsBranchProtection{
		"main": {
			RequiredApprovingReviewCount: 1,
			RequireCodeOwnerReviews:      true,
			RequiredStatusChecksStrict:   true,
			AllowForcePushes:             false,
			AllowDeletions:               false,
		},
	}}

	report := BuildGuardrailsReport(policy, current, false, false)
	if !guardrailDiffBlocked(report, "allow_force_pushes") || !guardrailDiffBlocked(report, "allow_deletions") {
		t.Fatalf("expected force-push/deletion relaxation to be blocked: %+v", report.Diff)
	}
}

func guardrailDiffBlocked(report GuardrailsSyncReport, field string) bool {
	for _, item := range report.Diff {
		if item.Field == field && item.Blocked {
			return true
		}
	}
	return false
}
