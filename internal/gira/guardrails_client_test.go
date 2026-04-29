package gira

import (
	"errors"
	"strings"
	"testing"
)

type mockGuardrailsRunner struct {
	responses map[string][]byte
	errs      map[string]error
	calls     []string
}

func (m *mockGuardrailsRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	m.calls = append(m.calls, key)
	if err, ok := m.errs[key]; ok {
		return nil, err
	}
	if out, ok := m.responses[key]; ok {
		return out, nil
	}
	return nil, errors.New("unexpected call: " + key)
}

func TestGHGuardrailsClientFetchCurrentGuardrailsSuccess(t *testing.T) {
	repo := ParseRepoRefMust("StatPan/gira")
	r := &mockGuardrailsRunner{
		responses: map[string][]byte{
			"gh api repos/StatPan/gira/branches": []byte(`[
				{"name":"main","protected":true},
				{"name":"dev","protected":false}
			]`),
			"gh api repos/StatPan/gira/branches/main/protection": []byte(`{
				"required_pull_request_reviews":{"required_approving_review_count":2,"require_code_owner_reviews":true},
				"required_status_checks":{"strict":true},
				"allow_force_pushes":{"enabled":false},
				"allow_deletions":{"enabled":false}
			}`),
			"gh api repos/StatPan/gira/rulesets": []byte(`[
				{"name":"baseline","target":"branch","enforcement":"active"}
			]`),
		},
		errs: map[string]error{},
	}
	client := NewGHGuardrailsClient(repo, r)
	state, err := client.FetchCurrentGuardrails()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.BranchProtection["main"].RequiredApprovingReviewCount != 2 {
		t.Fatalf("unexpected branch protection parse: %+v", state.BranchProtection["main"])
	}
	if len(state.Rulesets) != 1 || state.Rulesets[0].Name != "baseline" {
		t.Fatalf("unexpected rulesets parse: %+v", state.Rulesets)
	}
}

func TestGHGuardrailsClientApplyBranchProtectionFailure(t *testing.T) {
	repo := ParseRepoRefMust("StatPan/gira")
	key := "gh api repos/StatPan/gira/branches/main/protection -X PUT -H Accept: application/vnd.github+json -f required_pull_request_reviews[required_approving_review_count]=2 -f required_pull_request_reviews[require_code_owner_reviews]=true -f required_status_checks[strict]=true -f allow_force_pushes=false -f allow_deletions=false"
	r := &mockGuardrailsRunner{
		responses: map[string][]byte{},
		errs:      map[string]error{key: errors.New("403 forbidden")},
	}
	client := NewGHGuardrailsClient(repo, r)
	err := client.ApplyBranchProtection("main", GuardrailsBranchProtection{
		RequiredApprovingReviewCount: 2,
		RequireCodeOwnerReviews:      true,
		RequiredStatusChecksStrict:   true,
		AllowForcePushes:             false,
		AllowDeletions:               false,
	})
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("expected 403 error, got %v", err)
	}
}

func TestGHGuardrailsClientApplyRulesetPaths(t *testing.T) {
	repo := ParseRepoRefMust("StatPan/gira")
	key := "gh api repos/StatPan/gira/rulesets -X POST -f name=baseline -f target=branch -f enforcement=active"
	clientOK := NewGHGuardrailsClient(repo, &mockGuardrailsRunner{
		responses: map[string][]byte{key: []byte(`{"id":1}`)},
		errs:      map[string]error{},
	})
	if err := clientOK.ApplyRuleset(GuardrailsRulesetPolicy{Name: "baseline", Target: "branch", Enforcement: "active"}); err != nil {
		t.Fatalf("unexpected success-path error: %v", err)
	}

	clientAlreadyExists := NewGHGuardrailsClient(repo, &mockGuardrailsRunner{
		responses: map[string][]byte{},
		errs:      map[string]error{key: errors.New("already_exists")},
	})
	if err := clientAlreadyExists.ApplyRuleset(GuardrailsRulesetPolicy{Name: "baseline", Target: "branch", Enforcement: "active"}); err != nil {
		t.Fatalf("already_exists should be tolerated, got: %v", err)
	}

	clientFail := NewGHGuardrailsClient(repo, &mockGuardrailsRunner{
		responses: map[string][]byte{},
		errs:      map[string]error{key: errors.New("500 internal")},
	})
	if err := clientFail.ApplyRuleset(GuardrailsRulesetPolicy{Name: "baseline", Target: "branch", Enforcement: "active"}); err == nil {
		t.Fatal("expected failure for non-already_exists error")
	}
}
