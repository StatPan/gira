package gira

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildDoctorReportReady(t *testing.T) {
	report := BuildDoctorReport("StatPan/gira", readyDoctorRunner(), time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))

	if !report.Ready {
		t.Fatalf("ready = false, want true: %+v", report.Checks)
	}
	for _, id := range []string{"gira_cli_visible", "gh_available", "repo_context", "gh_auth", "repo_access", "metadata_drift", "workflow_policy_labels", "closed_issue_status_labels", "workflow_nonconformance", "onboard_readiness", "companion_doctors", "local_git_state"} {
		check := doctorCheckByID(report, id)
		if check == nil {
			t.Fatalf("missing check %s: %+v", id, report.Checks)
		}
		if check.Status != DoctorCheckPass {
			t.Fatalf("check %s status = %s, want pass: %+v", id, check.Status, *check)
		}
	}
	cliCheck := doctorCheckByID(report, "gira_cli_visible")
	if !strings.Contains(cliCheck.Detail, "executable=") {
		t.Fatalf("gira_cli_visible detail = %q, want executable detail", cliCheck.Detail)
	}
	if got := doctorCheckByID(report, "branch_policy"); got == nil || got.Status != DoctorCheckWarn || !strings.Contains(got.Detail, "github-flow defaults") {
		t.Fatalf("branch_policy = %+v, want default-policy warning", got)
	}
}

func TestBuildDoctorReportObservationClassifiesGiraMetadataAsAdvisory(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	runner := readyDoctorRunner()
	runner.responses["gh label list --repo StatPan/gira --json name,color,description --limit 1000"] = `[]`
	runner.responses["gh label list --repo StatPan/gira --json name --limit 1000"] = `[]`
	runner.responses["gh api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100"] = `[[ ]]`

	report := BuildDoctorReport("StatPan/gira", runner, time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))
	if report.Policy.OperationMode != OperationModeObservation || report.Policy.Source != OperationPolicySourceUnconfigured {
		t.Fatalf("policy = %+v, want unenrolled observation policy", report.Policy)
	}
	if !report.Ready {
		t.Fatalf("ready = false, want true with observation advisories: %+v", report.Checks)
	}
	for _, id := range []string{"metadata_drift", "workflow_policy_labels", "onboard_readiness"} {
		check := doctorCheckByID(report, id)
		if check == nil {
			t.Fatalf("missing check %s", id)
		}
		if check.Status != DoctorCheckWarn || check.FindingClass != DoctorFindingProviderObservation || check.Enforced {
			t.Fatalf("check %s = %+v, want provider observation warning", id, *check)
		}
		if check.Remediation != "" || !strings.Contains(check.Detail, "no managed mutation is prescribed") {
			t.Fatalf("check %s should not prescribe managed mutation: %+v", id, *check)
		}
	}
	policyCheck := doctorCheckByID(report, "operation_policy")
	if policyCheck == nil || policyCheck.Status != DoctorCheckPass || policyCheck.FindingClass != DoctorFindingProviderObservation || policyCheck.Enforced {
		t.Fatalf("operation_policy = %+v, want observation provenance", policyCheck)
	}
}

func TestPolicyFindingDoctorFixtures(t *testing.T) {
	cases := []struct {
		name        string
		policy      ResolvedOperationPolicy
		status      DoctorCheckStatus
		class       DoctorFindingClass
		enforced    bool
		remediation bool
	}{
		{
			name:        "unenrolled observation",
			policy:      ResolvedOperationPolicy{OperationMode: OperationModeObservation, DeliveryPolicy: DeliveryPolicyNone},
			status:      DoctorCheckWarn,
			class:       DoctorFindingProviderObservation,
			enforced:    false,
			remediation: false,
		},
		{
			name:        "explicit observation",
			policy:      ResolvedOperationPolicy{OperationMode: OperationModeObservation, DeliveryPolicy: DeliveryPolicyNone, Source: "repo_local_contract"},
			status:      DoctorCheckWarn,
			class:       DoctorFindingProviderObservation,
			enforced:    false,
			remediation: false,
		},
		{
			name:        "managed advisory",
			policy:      ResolvedOperationPolicy{OperationMode: OperationModeManaged, DeliveryPolicy: DeliveryPolicyAdvisory},
			status:      DoctorCheckWarn,
			class:       DoctorFindingManagedPolicy,
			enforced:    false,
			remediation: true,
		},
		{
			name:        "managed compatibility",
			policy:      ResolvedOperationPolicy{OperationMode: OperationModeManaged, DeliveryPolicy: DeliveryPolicyRequired, CompatibilityFallback: OperationPolicyFallbackConfiguredRepository},
			status:      DoctorCheckFail,
			class:       DoctorFindingManagedPolicy,
			enforced:    true,
			remediation: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			check := policyFindingDoctorCheck(tc.policy, "metadata_drift", "missing labels", "run sync")
			if check.Status != tc.status || check.FindingClass != tc.class || check.Enforced != tc.enforced {
				t.Fatalf("check = %+v, want status=%s class=%s enforced=%t", check, tc.status, tc.class, tc.enforced)
			}
			if (check.Remediation != "") != tc.remediation {
				t.Fatalf("remediation = %q, want present=%t", check.Remediation, tc.remediation)
			}
		})
	}
}

func TestBuildDoctorReportFailsClosedWhenOperationPolicyCannotResolve(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Join(root, ".gira"), 0o755); err != nil {
		t.Fatalf("mkdir .gira: %v", err)
	}
	config := `repo: StatPan/gira
operation_mode: invalid
profiles:
  default:
    labels: []
    milestones: []
    issue_templates: []
`
	if err := os.WriteFile(filepath.Join(root, ".gira", "config.yaml"), []byte(config), 0o644); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}

	report := BuildDoctorReport("StatPan/gira", readyDoctorRunner(), time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))
	if report.Ready {
		t.Fatal("ready = true, want false for policy resolution error")
	}
	check := doctorCheckByID(report, "operation_policy")
	if check == nil || check.Status != DoctorCheckFail || check.FindingClass != DoctorFindingManagedPolicy || !check.Enforced {
		t.Fatalf("operation_policy = %+v, want enforced fail", check)
	}
	if report.PolicyError == "" {
		t.Fatal("policy_error is empty, want stable failure detail")
	}
	if got := doctorCheckByID(report, "metadata_drift"); got == nil || got.Status != DoctorCheckSkip {
		t.Fatalf("metadata_drift = %+v, want skipped after policy resolution failure", got)
	}
}

func TestBuildDoctorReportMissingGhFailsAndSkipsGhDependentChecks(t *testing.T) {
	runner := onboardFakeRunner{
		errors: map[string]error{
			"gh --version": fmt.Errorf("executable file not found"),
		},
		responses: map[string]string{
			"git rev-parse --is-inside-work-tree": "true",
			"git branch --show-current":           "main",
			"git status --porcelain":              "",
		},
	}
	report := BuildDoctorReport("StatPan/gira", runner, time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))

	if report.Ready {
		t.Fatal("ready = true, want false")
	}
	if got := doctorCheckByID(report, "gh_available"); got == nil || got.Status != DoctorCheckFail {
		t.Fatalf("gh_available = %+v, want fail", got)
	}
	if got := doctorCheckByID(report, "metadata_drift"); got == nil || got.Status != DoctorCheckSkip {
		t.Fatalf("metadata_drift = %+v, want skip", got)
	}
}

func TestBuildDoctorReportAuthAndRepoFailure(t *testing.T) {
	runner := readyDoctorRunner()
	runner.errors["gh auth status"] = fmt.Errorf("not logged in")
	runner.errors["gh repo view StatPan/gira --json nameWithOwner,viewerPermission,defaultBranchRef"] = fmt.Errorf("HTTP 404")
	report := BuildDoctorReport("StatPan/gira", runner, time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))

	if report.Ready {
		t.Fatal("ready = true, want false")
	}
	if got := doctorCheckByID(report, "gh_auth"); got == nil || got.Status != DoctorCheckFail {
		t.Fatalf("gh_auth = %+v, want fail", got)
	}
	if got := doctorCheckByID(report, "repo_access"); got == nil || got.Status != DoctorCheckFail {
		t.Fatalf("repo_access = %+v, want fail", got)
	}
}

func TestBuildDoctorReportDriftFailureHasSyncRemediation(t *testing.T) {
	runner := readyDoctorRunner()
	runner.responses["gh label list --repo StatPan/gira --json name,color,description --limit 1000"] = `[]`
	report := BuildDoctorReport("StatPan/gira", runner, time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))

	if report.Ready {
		t.Fatal("ready = true, want false")
	}
	check := doctorCheckByID(report, "metadata_drift")
	if check == nil {
		t.Fatal("metadata_drift check missing")
	}
	if check.Status != DoctorCheckFail {
		t.Fatalf("metadata_drift status = %s, want fail", check.Status)
	}
	for _, want := range []string{
		"`gira ops sync --repo StatPan/gira --dry-run`",
		"`gira ops sync --repo StatPan/gira`",
		"labels create=",
	} {
		if !strings.Contains(check.Remediation+check.Detail, want) {
			t.Fatalf("metadata_drift missing %q: %+v", want, *check)
		}
	}
}

func TestBuildDoctorReportBootstrapIssueDriftWarnsOnly(t *testing.T) {
	runner := readyDoctorRunner()
	runner.responses["gh issue list --repo StatPan/gira --state all --label gira:bootstrap --json number,title,labels --limit 1000"] = `[]`
	report := BuildDoctorReport("StatPan/gira", runner, time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))

	if !report.Ready {
		t.Fatalf("ready = false, want true for optional bootstrap issue drift: %+v", report.Checks)
	}
	check := doctorCheckByID(report, "metadata_drift")
	if check == nil {
		t.Fatal("metadata_drift check missing")
	}
	if check.Status != DoctorCheckWarn {
		t.Fatalf("metadata_drift status = %s, want warn: %+v", check.Status, *check)
	}
	if !strings.Contains(check.Detail, "optional bootstrap issues create=5") {
		t.Fatalf("metadata_drift detail = %q, want optional bootstrap count", check.Detail)
	}
	if !strings.Contains(check.Remediation, "only when you want Gira sample bootstrap issues") {
		t.Fatalf("metadata_drift remediation = %q, want opt-in guidance", check.Remediation)
	}
}

func TestBuildDoctorReportWorkflowPolicyLabelFailure(t *testing.T) {
	runner := readyDoctorRunner()
	runner.responses["gh label list --repo StatPan/gira --json name --limit 1000"] = `[{"name":"type:task"},{"name":"status:ready"}]`
	report := BuildDoctorReport("StatPan/gira", runner, time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))

	if report.Ready {
		t.Fatal("ready = true, want false")
	}
	check := doctorCheckByID(report, "workflow_policy_labels")
	if check == nil {
		t.Fatal("workflow_policy_labels check missing")
	}
	if check.Status != DoctorCheckFail {
		t.Fatalf("workflow_policy_labels status = %s, want fail: %+v", check.Status, *check)
	}
	if !strings.Contains(check.Detail, "priority:p1") || !strings.Contains(check.Detail, "area:ai") {
		t.Fatalf("workflow_policy_labels detail = %q, want missing policy labels", check.Detail)
	}
}

func TestBuildDoctorReportClosedIssueStatusLabelsFail(t *testing.T) {
	runner := readyDoctorRunner()
	runner.responses["gh issue list --repo StatPan/gira --state closed --limit 1000 --json number,title,labels"] = `[
		{"number":420,"title":"Template work","labels":[{"name":"status:in-progress"},{"name":"type:story"}]},
		{"number":422,"title":"Coverage","labels":[{"name":"status:in-review"}]}
	]`
	report := BuildDoctorReport("StatPan/gira", runner, time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))

	if report.Ready {
		t.Fatal("ready = true, want false")
	}
	check := doctorCheckByID(report, "closed_issue_status_labels")
	if check == nil {
		t.Fatal("closed_issue_status_labels check missing")
	}
	if check.Status != DoctorCheckFail {
		t.Fatalf("closed_issue_status_labels status = %s, want fail: %+v", check.Status, *check)
	}
	for _, want := range []string{"closed issues with active status labels=2", "#420 status:in-progress", "#422 status:in-review", "gira adopt issues --repo StatPan/gira --state all --issues 420,422 --normalize-status --dry-run"} {
		if !strings.Contains(check.Detail+check.Remediation, want) {
			t.Fatalf("closed_issue_status_labels missing %q: %+v", want, *check)
		}
	}
}

func TestBuildDoctorReportDetachedHeadFailsReadiness(t *testing.T) {
	runner := readyDoctorRunner()
	runner.responses["git branch --show-current"] = ""
	report := BuildDoctorReport("StatPan/gira", runner, time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))

	if report.Ready {
		t.Fatal("ready = true, want false")
	}
	check := doctorCheckByID(report, "local_git_state")
	if check == nil {
		t.Fatal("local_git_state check missing")
	}
	if check.Status != DoctorCheckFail {
		t.Fatalf("local_git_state status = %s, want fail: %+v", check.Status, *check)
	}
	if !strings.Contains(check.Detail, "detached HEAD") {
		t.Fatalf("local_git_state detail = %q, want detached HEAD", check.Detail)
	}
}

func TestBuildDoctorReportDirtyWorktreeFailsReadiness(t *testing.T) {
	runner := readyDoctorRunner()
	runner.responses["git status --porcelain"] = " M internal/gira/doctor.go\n M .gira/audit/StatPan_gira.jsonl\n?? scratch.txt\n"
	report := BuildDoctorReport("StatPan/gira", runner, time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))

	if report.Ready {
		t.Fatal("ready = true, want false")
	}
	check := doctorCheckByID(report, "local_git_state")
	if check == nil {
		t.Fatal("local_git_state check missing")
	}
	if check.Status != DoctorCheckFail {
		t.Fatalf("local_git_state status = %s, want fail: %+v", check.Status, *check)
	}
	if !strings.Contains(check.Detail, "uncommitted changes=3") || !strings.Contains(check.Detail, "Gira audit ledger changes=1") || !strings.Contains(check.Detail, "user changes=2") {
		t.Fatalf("local_git_state detail = %q, want dirty count", check.Detail)
	}
}

func TestBuildDoctorReportAuditOnlyDirtyWorktreeWarns(t *testing.T) {
	runner := readyDoctorRunner()
	runner.responses["git status --porcelain"] = " M .gira/audit/StatPan_gira.jsonl\n M .gira/audit/StatPan_gira.jsonl.lasthash\n"
	report := BuildDoctorReport("StatPan/gira", runner, time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))

	if !report.Ready {
		t.Fatalf("ready = false, want true for audit-only changes: %+v", report.Checks)
	}
	check := doctorCheckByID(report, "local_git_state")
	if check == nil {
		t.Fatal("local_git_state check missing")
	}
	if check.Status != DoctorCheckWarn {
		t.Fatalf("local_git_state status = %s, want warn: %+v", check.Status, *check)
	}
	if !strings.Contains(check.Detail, "Gira audit ledger changes=2") || !strings.Contains(check.Detail, "user changes=0") {
		t.Fatalf("local_git_state detail = %q, want audit-owned dirty detail", check.Detail)
	}
}

func readyDoctorRunner() onboardFakeRunner {
	return onboardFakeRunner{
		responses: map[string]string{
			"gh --version":   "gh version 2.0.0",
			"gh auth status": "Logged in to github.com",
			"gh repo view StatPan/gira --json nameWithOwner,viewerPermission,defaultBranchRef":                             `{"nameWithOwner":"StatPan/gira","viewerPermission":"WRITE","defaultBranchRef":{"name":"main"}}`,
			"gh label list --repo StatPan/gira --json name,color,description --limit 1000":                                 doctorReadyLabelsJSON(),
			"gh label list --repo StatPan/gira --json name --limit 1000":                                                   doctorReadyLabelsJSON(),
			"gh api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100":                  doctorDesiredMilestonesJSON(),
			"gh api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100":                      `[[{"number":129,"title":"Doctor","state":"open","labels":[{"name":"type:task"}],"milestone":{"title":"MVP"},"updated_at":"2026-05-05T12:00:00Z","html_url":"https://github.com/StatPan/gira/issues/129"}]]`,
			"gh issue list --repo StatPan/gira --state closed --limit 1000 --json number,title,labels":                     `[]`,
			"gh issue list --repo StatPan/gira --state all --limit 1000 --json number,title,state,labels,body":             `[{"number":129,"title":"Doctor","state":"OPEN","body":"` + ProvenanceBlockStart + `\nplanning: human\nimplementation:\nreview:\n` + ProvenanceBlockEnd + `","labels":[{"name":"type:task"},{"name":"status:ready"}]}]`,
			"gh pr list --repo StatPan/gira --state all --limit 1000 --json number,title,body,state,mergedAt":              `[]`,
			"gh issue list --repo StatPan/gira --state all --label gira:bootstrap --json number,title,labels --limit 1000": desiredBootstrapIssuesJSON(),
			"git rev-parse --is-inside-work-tree":                                                                          "true",
			"git ls-remote --exit-code --heads origin main":                                                                "abc\trefs/heads/main",
			"git branch --show-current": "main",
			"git status --porcelain":    "",
		},
		errors: map[string]error{},
	}
}

func doctorReadyLabelsJSON() string {
	labels := append([]LabelDef{}, DesiredLabels...)
	parts := make([]string, 0, len(labels))
	for _, label := range labels {
		parts = append(parts, fmt.Sprintf(`{"name":%q,"color":%q,"description":%q}`, label.Name, label.Color, label.Description))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func doctorDesiredMilestonesJSON() string {
	parts := make([]string, 0, len(DesiredMilestones))
	for idx, milestone := range DesiredMilestones {
		dueOn := "null"
		if milestone.DueOn != nil {
			dueOn = fmt.Sprintf("%q", *milestone.DueOn)
		}
		parts = append(parts, fmt.Sprintf(`{"number":%d,"title":%q,"description":%q,"due_on":%s}`, idx+1, milestone.Title, milestone.Description, dueOn))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func doctorCheckByID(report DoctorReport, id string) *DoctorCheck {
	for i := range report.Checks {
		if report.Checks[i].ID == id {
			return &report.Checks[i]
		}
	}
	return nil
}
