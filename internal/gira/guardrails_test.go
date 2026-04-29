package gira

import "testing"

type fakeGuardrailsClient struct {
	current      GuardrailsState
	branchApply  []string
	rulesetApply []string
}

func (f *fakeGuardrailsClient) FetchCurrentGuardrails() (GuardrailsState, error) {
	return f.current, nil
}
func (f *fakeGuardrailsClient) ApplyBranchProtection(pattern string, cfg GuardrailsBranchProtection) error {
	f.branchApply = append(f.branchApply, pattern)
	f.current.BranchProtection[pattern] = cfg
	return nil
}
func (f *fakeGuardrailsClient) ApplyRuleset(rs GuardrailsRulesetPolicy) error {
	f.rulesetApply = append(f.rulesetApply, rs.Name)
	return nil
}

func TestSyncGuardrailsRelaxationBlockedWithoutFlag(t *testing.T) {
	policy := GuardrailsPolicy{BranchProtection: map[string]GuardrailsBranchProtection{"main": {RequiredApprovingReviewCount: 1}}}
	client := &fakeGuardrailsClient{current: GuardrailsState{BranchProtection: map[string]GuardrailsBranchProtection{"main": {RequiredApprovingReviewCount: 2}}}}
	report, err := SyncGuardrailsForClient(ParseRepoRefMust("StatPan/gira"), policy, client, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Diff) == 0 || !report.Diff[0].Blocked {
		t.Fatalf("expected blocked relaxation diff")
	}
}

func TestSyncGuardrailsUnknownPatternBlocked(t *testing.T) {
	policy := GuardrailsPolicy{BranchProtection: map[string]GuardrailsBranchProtection{"main": {RequiredApprovingReviewCount: 2}}}
	client := &fakeGuardrailsClient{current: GuardrailsState{BranchProtection: map[string]GuardrailsBranchProtection{"release/*": {RequiredApprovingReviewCount: 2}}}}
	report, err := SyncGuardrailsForClient(ParseRepoRefMust("StatPan/gira"), policy, client, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if report.BlockedCount != 1 {
		t.Fatalf("blocked_count=%d want 1", report.BlockedCount)
	}
}

func TestSyncGuardrailsIdempotentSecondApplyZeroDiff(t *testing.T) {
	policy := GuardrailsPolicy{BranchProtection: map[string]GuardrailsBranchProtection{"main": {RequiredApprovingReviewCount: 2}}}
	client := &fakeGuardrailsClient{current: GuardrailsState{BranchProtection: map[string]GuardrailsBranchProtection{"main": {RequiredApprovingReviewCount: 1}}}}
	first, err := SyncGuardrailsForClient(ParseRepoRefMust("StatPan/gira"), policy, client, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Applied) == 0 {
		t.Fatal("expected apply actions")
	}
	second, err := SyncGuardrailsForClient(ParseRepoRefMust("StatPan/gira"), policy, client, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Diff) != 0 {
		t.Fatalf("expected zero diff on second apply, got %d", len(second.Diff))
	}
}
