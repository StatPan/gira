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
