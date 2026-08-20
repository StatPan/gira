package gira

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func configureManagedRequiredPolicyTest(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".gira"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := "repo: StatPan/gira\noperation_mode: managed\ndelivery_policy: required\nprofiles:\n  default:\n    labels: []\n    milestones: []\n    issue_templates: []\n"
	if err := os.WriteFile(filepath.Join(root, ".gira", "config.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
}

func TestTicketReadinessPolicyModesClassifyManagedFindings(t *testing.T) {
	body := "## Goal\nShip the change\n\n## Scope\n_No response_\n\n## Acceptance Criteria\n_No response_\n"
	tests := []struct {
		name      string
		policy    ResolvedOperationPolicy
		readiness string
		enforced  bool
		severity  string
	}{
		{name: "observation", policy: ResolvedOperationPolicy{OperationMode: OperationModeObservation, DeliveryPolicy: DeliveryPolicyNone, Source: "repo"}, readiness: "ready", enforced: false, severity: "warning"},
		{name: "advisory", policy: ResolvedOperationPolicy{OperationMode: OperationModeManaged, DeliveryPolicy: DeliveryPolicyAdvisory, Source: "repo"}, readiness: "ready", enforced: false, severity: "warning"},
		{name: "required", policy: ResolvedOperationPolicy{OperationMode: OperationModeManaged, DeliveryPolicy: DeliveryPolicyRequired, Source: "repo"}, readiness: "needs_refinement", enforced: true, severity: "error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := EvaluateTicketReadinessWithPolicy(body, []string{"type:task", "status:ready"}, "open", tt.policy)
			if report.Policy == nil || report.Policy.Source != "repo" {
				t.Fatalf("policy provenance missing: %+v", report)
			}
			if report.Readiness != tt.readiness {
				t.Fatalf("readiness = %q, want %q: %+v", report.Readiness, tt.readiness, report)
			}
			finding := report.Findings[0]
			if finding.FindingClass != ReviewFindingClassPolicy || finding.Enforced != tt.enforced || finding.Severity != tt.severity {
				t.Fatalf("managed finding provenance = %+v", finding)
			}
		})
	}
}

func TestTicketReadinessProviderFactsRemainBlockingAcrossPolicyModes(t *testing.T) {
	policies := []ResolvedOperationPolicy{
		{OperationMode: OperationModeObservation, DeliveryPolicy: DeliveryPolicyNone, Source: "observation"},
		{OperationMode: OperationModeManaged, DeliveryPolicy: DeliveryPolicyAdvisory, Source: "advisory"},
		{OperationMode: OperationModeManaged, DeliveryPolicy: DeliveryPolicyRequired, Source: "required"},
	}
	for _, policy := range policies {
		report := EvaluateTicketReadinessWithPolicy("", []string{"type:task", "status:ready"}, "closed", policy)
		if report.Readiness != "unknown" || len(report.Findings) != 1 {
			t.Fatalf("closed ticket should remain provider-blocked for %+v: %+v", policy, report)
		}
		finding := report.Findings[0]
		if finding.Kind != "closed_ticket" || finding.FindingClass != ReviewFindingClassProvider || !finding.Enforced {
			t.Fatalf("closed ticket provenance = %+v", finding)
		}
	}
}

func TestPRReadinessManagedFindingsRespectOperationPolicy(t *testing.T) {
	base := prReadinessInput{
		Repo:             "StatPan/gira",
		Issue:            939,
		PullRequest:      940,
		PRAvailable:      true,
		ChecksStatus:     "passed",
		ClosingReference: false,
		ReviewPolicy:     &FinishReviewPolicy{Value: FinishReviewPolicyNone, Source: "explicit"},
		Acceptance:       &TicketStatusAcceptance{Status: "complete", Total: 1, Complete: 1},
	}
	tests := []struct {
		name      string
		policy    ResolvedOperationPolicy
		readiness string
		enforced  bool
		severity  string
	}{
		{name: "observation", policy: ResolvedOperationPolicy{OperationMode: OperationModeObservation, DeliveryPolicy: DeliveryPolicyNone, Source: "repo"}, readiness: "ready_for_review", enforced: false, severity: "warning"},
		{name: "advisory", policy: ResolvedOperationPolicy{OperationMode: OperationModeManaged, DeliveryPolicy: DeliveryPolicyAdvisory, Source: "repo"}, readiness: "ready_for_review", enforced: false, severity: "warning"},
		{name: "required", policy: ResolvedOperationPolicy{OperationMode: OperationModeManaged, DeliveryPolicy: DeliveryPolicyRequired, Source: "repo"}, readiness: "needs_revision", enforced: true, severity: "error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base.Policy = tt.policy
			report := evaluatePRReadinessWithPolicy(base, tt.policy)
			if report.Policy.Source != "repo" || report.Readiness != tt.readiness {
				t.Fatalf("policy/readiness = %+v, want %q", report, tt.readiness)
			}
			finding := report.Findings[0]
			if finding.Kind != "missing_closing_link" || finding.FindingClass != ReviewFindingClassPolicy || finding.Enforced != tt.enforced || finding.Severity != tt.severity {
				t.Fatalf("managed PR finding = %+v", finding)
			}
		})
	}
}

func TestPRReadinessProviderFactsRemainBlockingInAdvisoryMode(t *testing.T) {
	policy := ResolvedOperationPolicy{OperationMode: OperationModeManaged, DeliveryPolicy: DeliveryPolicyAdvisory, Source: "repo"}
	for _, tc := range []struct {
		name string
		set  func(*prReadinessInput)
		kind string
	}{
		{name: "draft", set: func(input *prReadinessInput) { input.IsDraft = true }, kind: "draft_pr"},
		{name: "failing checks", set: func(input *prReadinessInput) { input.ChecksStatus = "failed" }, kind: "checks_failing"},
		{name: "base mismatch", set: func(input *prReadinessInput) { input.BaseMismatch = true }, kind: "base_mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := prReadinessInput{Repo: "StatPan/gira", Issue: 939, PullRequest: 940, PRAvailable: true, ClosingReference: true, ChecksStatus: "passed", ReviewPolicy: &FinishReviewPolicy{Value: FinishReviewPolicyNone}, Acceptance: &TicketStatusAcceptance{Status: "complete"}, Policy: policy}
			tc.set(&input)
			report := evaluatePRReadinessWithPolicy(input, policy)
			if report.Readiness != "needs_revision" || !prReadinessHasFinding(report, tc.kind) {
				t.Fatalf("provider fact must block: %+v", report)
			}
			for _, finding := range report.Findings {
				if finding.Kind == tc.kind && (finding.FindingClass != ReviewFindingClassProvider || !finding.Enforced) {
					t.Fatalf("provider finding provenance = %+v", finding)
				}
			}
		})
	}
}

func TestPRReadinessPreservesExplicitRequiredFinishReviewPolicy(t *testing.T) {
	policy := ResolvedOperationPolicy{OperationMode: OperationModeManaged, DeliveryPolicy: DeliveryPolicyAdvisory, Source: "repo"}
	report := evaluatePRReadinessWithPolicy(prReadinessInput{
		Repo:             "StatPan/gira",
		Issue:            939,
		PullRequest:      940,
		PRAvailable:      true,
		ClosingReference: true,
		ChecksStatus:     "passed",
		ReviewStatus:     "unknown",
		ReviewPolicy:     &FinishReviewPolicy{Value: FinishReviewPolicyRequired, Source: "explicit"},
		Policy:           policy,
		Acceptance:       &TicketStatusAcceptance{Status: "complete"},
	}, policy)
	if report.Readiness != "needs_revision" || !prReadinessHasFinding(report, "review_required_but_absent") {
		t.Fatalf("explicit finish review policy must remain blocking: %+v", report)
	}
	for _, finding := range report.Findings {
		if finding.Kind == "review_required_but_absent" && (!finding.Enforced || finding.FindingClass != ReviewFindingClassPolicy) {
			t.Fatalf("explicit review policy provenance = %+v", finding)
		}
	}
}

func TestStartWorkFailsClosedOnOperationPolicyResolutionError(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".gira"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gira", "config.yaml"), []byte("repo: StatPan/gira\noperation_mode: managed\ndelivery_policy: invalid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/939": []byte(`{"number":939,"title":"Policy","state":"open","labels":[{"name":"status:ready"}]}`),
	}}
	_, err := StartWorkWithOptions(RepoRef{Owner: "StatPan", Name: "gira"}, 939, WorkStartOptions{DryRun: true}, runner)
	if err == nil || !strings.Contains(err.Error(), "resolve operation policy") {
		t.Fatalf("expected policy resolution error, got %v", err)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "checkout") || strings.Contains(call, "labels -X POST") || strings.Contains(call, "-X PATCH") {
			t.Fatalf("policy error must precede mutation: %v", runner.calls)
		}
	}
}

func TestStartWorkAdvisoryAllowsOpenIssueWithoutReadyLabel(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".gira"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gira", "config.yaml"), []byte("repo: StatPan/gira\noperation_mode: managed\ndelivery_policy: advisory\nprofiles:\n  default:\n    labels: []\nbranch_policy:\n  mode: github-flow\n  start_mode: auto\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	runner := autoStartRunner(956, "Advisory start", "main", false)
	runner.outputs["gh api repos/StatPan/gira/issues/956"] = []byte(`{"number":956,"title":"Advisory start","state":"open","labels":[{"name":"type:task"}]}`)
	runner.errs["git show-ref --verify --quiet refs/heads/issue-956-advisory-start"] = fmt.Errorf("exit status 1")
	runner.errs["git ls-remote --exit-code --heads origin issue-956-advisory-start"] = fmt.Errorf("exit status 2")
	runner.errs["git show-ref --verify --quiet refs/heads/issue/956-advisory-start"] = fmt.Errorf("exit status 1")
	runner.errs["git ls-remote --exit-code --heads origin issue/956-advisory-start"] = fmt.Errorf("exit status 2")
	result, err := StartWorkWithOptions(RepoRef{Owner: "StatPan", Name: "gira"}, 956, WorkStartOptions{DryRun: true, Branch: "auto"}, runner)
	if err != nil {
		t.Fatalf("advisory start should be previewable: %v", err)
	}
	if result.Policy == nil || result.Policy.DeliveryPolicy != DeliveryPolicyAdvisory || result.TicketReadiness == nil || result.TicketReadiness.Readiness != "ready" {
		t.Fatalf("missing advisory provenance: %+v", result)
	}
	if result.Checks["ready_label"] {
		t.Fatalf("readiness check must reflect missing label: %+v", result.Checks)
	}
	if result.TicketReadiness.Findings[0].Enforced {
		t.Fatalf("readiness finding should be advisory: %+v", result.TicketReadiness.Findings[0])
	}
}

func TestTicketReadinessJSONPolicyFieldsAreAdditive(t *testing.T) {
	policy := ResolvedOperationPolicy{OperationMode: OperationModeManaged, DeliveryPolicy: DeliveryPolicyAdvisory, Source: "repo"}
	report := EvaluateTicketReadinessWithPolicy("## Goal\nGoal\n## Scope\n_No response_\n## Acceptance Criteria\n- a\n", []string{"type:task", "status:ready"}, "open", policy)
	if report.Policy == nil || report.Policy.Source != "repo" {
		t.Fatal("policy source missing")
	}
	if len(report.Findings) == 0 || report.Findings[0].FindingClass == "" {
		t.Fatalf("finding provenance missing: %+v", report.Findings)
	}
}

func TestReadOnlyMutationNextStepsUseDryRun(t *testing.T) {
	repo := ParseRepoRefMust("StatPan/gira")
	checks := ticketChecksNextStep(repo, 939, DevPRStatusResult{})
	if !strings.Contains(checks, "gira ticket pr") || !strings.Contains(checks, "--dry-run") {
		t.Fatalf("ticket checks next step must preview PR creation: %q", checks)
	}
	for _, blockers := range [][]string{{"checks_pending"}, {"checks"}, {"review"}, {"draft"}} {
		next := finishBlockedNextStep(repo, 939, blockers)
		if !strings.Contains(next, "--dry-run") || strings.Contains(next, "--apply") {
			t.Fatalf("finish blocker next step must preview mutation: %q", next)
		}
	}
	if next := goalNextSafeCommand(repo, GoalStatusChild{Number: 940, Category: "ready"}); !strings.Contains(next, "--dry-run") {
		t.Fatalf("goal ready child next step must preview start: %q", next)
	}
}
