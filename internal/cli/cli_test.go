package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/StatPan/gira/internal/gira"
)

func TestHelpOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Jira-style project flow on GitHub") {
		t.Fatalf("help output missing product description:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "ticket") {
		t.Fatalf("help output missing ticket command:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "milestone") {
		t.Fatalf("help output missing milestone command:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "guide") {
		t.Fatalf("help output missing guide command:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "setup") {
		t.Fatalf("help output missing setup command:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "ops") {
		t.Fatalf("help output missing ops command:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "version") {
		t.Fatalf("help output missing version command:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "upgrade") {
		t.Fatalf("help output missing upgrade command:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "cache") {
		t.Fatalf("help output missing cache command:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "config") {
		t.Fatalf("help output missing config command:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "repo") {
		t.Fatalf("help output missing repo command:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "stats") {
		t.Fatalf("help output missing stats command:\n%s", stdout.String())
	}
	for _, want := range []string{"Assist (read GitHub state", "Managed delivery (one GitHub issue", "Advanced orchestration (explicit multi-ticket", "Canonical ticket lifecycle commands", "Canonical Goal-level agent entry"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help output missing tiered discovery %q:\n%s", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "portfolio   ") || strings.Contains(stdout.String(), "jira        ") {
		t.Fatalf("help output should not frontload advanced commands:\n%s", stdout.String())
	}
}

func TestReportPortfolioWritesOnlyExplicitLocalHTML(t *testing.T) {
	restoreDashboard := newDashboardExportClient
	restoreReview := newReviewGateClient
	restoreNow := reportNow
	t.Cleanup(func() {
		newDashboardExportClient = restoreDashboard
		newReviewGateClient = restoreReview
		reportNow = restoreNow
	})
	reportNow = func() time.Time { return time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC) }
	due := "2026-08-15T00:00:00Z"
	newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient {
		return &cliFakeDashboardExportClient{repo: repo, issues: []gira.DashboardRawIssue{{IssueNumber: 9, Title: "Blocked", State: "open", Labels: []string{"status:blocked"}, Milestone: "V3", UpdatedAt: "2026-07-20T00:00:00Z", URL: "https://example/issues/9"}}, milestones: []gira.DashboardRawMilestone{{MilestoneNumber: 3, Title: "V3", State: "open", DueOn: &due, OpenIssues: 1, ClosedIssues: 3}}}
	}
	newReviewGateClient = func(repo gira.RepoRef) gira.ReviewGateClient {
		return weeklyReviewClient{repo: repo, prs: []gira.ReviewPR{{Number: 10, Title: "Review", URL: "https://example/pull/10", UpdatedAt: "2026-07-21T00:00:00Z"}}}
	}
	output := filepath.Join(t.TempDir(), "reports", "portfolio.html")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"report", "portfolio", "--repo", "StatPan/gira", "--milestone", "V3", "--since", "2026-07-01", "--until", "2026-08-31", "--output", output}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	content, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Gira portfolio overview", "Milestone delivery", "Timeline and named gates", "Work queues", "Static local artifact"} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("portfolio HTML missing %q", want)
		}
	}
	if !strings.Contains(stdout.String(), "portfolio html written to "+output) {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestReportPortfolioRequiresOutputAndValidWindow(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"report", "portfolio", "--repo", "StatPan/gira"}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "--output is required") {
		t.Fatalf("missing output was not rejected: code=%d stderr=%s", code, stderr.String())
	}

	restoreDashboard := newDashboardExportClient
	restoreReview := newReviewGateClient
	t.Cleanup(func() {
		newDashboardExportClient = restoreDashboard
		newReviewGateClient = restoreReview
	})
	newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient { return &cliFakeDashboardExportClient{repo: repo} }
	newReviewGateClient = func(repo gira.RepoRef) gira.ReviewGateClient { return weeklyReviewClient{repo: repo} }
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"report", "portfolio", "--repo", "StatPan/gira", "--since", "2026-08-01", "--until", "2026-07-01", "--output", filepath.Join(t.TempDir(), "portfolio.html")}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "--since must be on or before --until") {
		t.Fatalf("invalid window was not rejected: code=%d stderr=%s", code, stderr.String())
	}
}

func TestConfigGlobalCommandJSON(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte("default_owner: StatPan\n"), 0o644); err != nil {
		t.Fatalf("write global config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"config", "global", "--config-root", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.ConfigGlobalReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode config global JSON: %v\n%s", err, stdout.String())
	}
	if report.ConfigRoot != root || !report.Config.Exists || !report.Config.Valid {
		t.Fatalf("unexpected config global report: %+v", report)
	}
}

func TestConfigRepoCommandText(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "repos", "StatPan", "gira.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir repo registry: %v", err)
	}
	if err := os.WriteFile(path, []byte("repo: StatPan/gira\npath: ~/workspace/apps/gira\n"), 0o644); err != nil {
		t.Fatalf("write repo registry: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"config", "repo", "--repo", "StatPan/gira", "--config-root", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"config repo: StatPan/gira", "source: explicit", "global repo:", "valid=true", ".gira/config.yaml"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("config repo output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestConfigDoctorCommandJSON(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte("default_workspace: personal\n"), 0o644); err != nil {
		t.Fatalf("write global config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"config", "doctor", "--repo", "StatPan/gira", "--config-root", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.ConfigDoctorReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode config doctor JSON: %v\n%s", err, stdout.String())
	}
	if report.Source != "explicit" || report.Repo != "StatPan/gira" || !report.GlobalConfig.Exists {
		t.Fatalf("unexpected config doctor report: %+v", report)
	}
}

func TestConfigStorageCommandJSON(t *testing.T) {
	t.Chdir(t.TempDir())
	root := t.TempDir()
	stateRoot := filepath.Join(t.TempDir(), "state")
	if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte("paths:\n  state_root: "+filepath.ToSlash(stateRoot)+"\n"), 0o644); err != nil {
		t.Fatalf("write global config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"config", "storage", "--repo", "StatPan/gira", "--config-root", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.ConfigStorageReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode config storage JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.ConfigStorageReportSchemaVersion || report.Command != "config storage" || report.Repo != "StatPan/gira" {
		t.Fatalf("unexpected config storage report: %+v", report)
	}
	if !strings.Contains(stdout.String(), `"runtime_state_root"`) || !strings.Contains(stdout.String(), filepath.ToSlash(stateRoot)) {
		t.Fatalf("config storage JSON missing state root evidence:\n%s", stdout.String())
	}
}

func TestConfigUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"config", "missing"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown config command: missing") {
		t.Fatalf("stderr missing unknown command:\n%s", stderr.String())
	}
}

func TestFeatureAliasCheckCommand(t *testing.T) {
	original := newFeatureMapCheckReport
	t.Cleanup(func() { newFeatureMapCheckReport = original })
	newFeatureMapCheckReport = func(options gira.FeatureMapOptions) (gira.FeatureMapCheckReport, error) {
		if options.Repo.FullName() != "StatPan/backlog" || options.Limit != 25 {
			t.Fatalf("unexpected feature check options: %+v", options)
		}
		return gira.FeatureMapCheckReport{
			SchemaVersion: gira.FeatureMapCheckSchemaVersion,
			Command:       "feature check",
			Repo:          options.Repo.FullName(),
			Source:        "github_issues",
			Mode:          "none",
			Limit:         options.Limit,
			Diagnostics: []gira.FeatureMapDiagnostic{
				{Severity: "info", Code: "feature_map_not_configured", Message: "no issue-backed feature records found"},
			},
			NextStep: "feature map is optional; no action required",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"feat", "check", "--repo", "StatPan/backlog", "--limit", "25"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"feature map check: StatPan/backlog", "feature_map_not_configured", "feature map is optional"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("feature check output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestFeatureForCommandJSON(t *testing.T) {
	original := newFeatureMapForReport
	t.Cleanup(func() { newFeatureMapForReport = original })
	newFeatureMapForReport = func(options gira.FeatureForOptions) (gira.FeatureMapForReport, error) {
		if options.Repo.FullName() != "StatPan/backlog" || options.Issue != 41 {
			t.Fatalf("unexpected feature for options: %+v", options)
		}
		feature := gira.FeatureMapFeature{Number: 31, Title: "Capability: Ticket lifecycle", Key: "tl", Maturity: "stable"}
		return gira.FeatureMapForReport{
			SchemaVersion: gira.FeatureMapForSchemaVersion,
			Command:       "feature for",
			Repo:          options.Repo.FullName(),
			Issue:         gira.FeatureMapWorkIssue{Number: options.Issue, Title: "Add finish receipt validation", LinkedFeature: 31},
			Feature:       &feature,
			NextStep:      "gira ticket status 41 --repo StatPan/backlog",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"feature", "for", "41", "--repo", "StatPan/backlog", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.FeatureMapForReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode feature for JSON: %v\n%s", err, stdout.String())
	}
	if report.Feature == nil || report.Feature.Key != "tl" || report.Issue.LinkedFeature != 31 {
		t.Fatalf("unexpected feature for JSON: %+v", report)
	}
}

func TestSetupGlobalCommandJSON(t *testing.T) {
	original := newSetupGlobalReport
	t.Cleanup(func() { newSetupGlobalReport = original })
	newSetupGlobalReport = func(input gira.SetupGlobalInput) (gira.SetupGlobalReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Path != "/repo" || input.ConfigRoot != "/tmp/gira" || input.WorkspaceName != "personal" || input.Mode != "global-only" || !input.DryRun || input.Apply {
			t.Fatalf("unexpected setup global input: %+v", input)
		}
		return gira.SetupGlobalReport{
			Command:    "setup global",
			Mode:       input.Mode,
			ConfigRoot: input.ConfigRoot,
			Repo:       input.Repo.FullName(),
			Path:       input.Path,
			Workspace:  gira.WorkspaceSummary{Name: input.WorkspaceName, Owner: "StatPan"},
			InboxRepo:  "StatPan/gira",
			GlobalWorkspace: gira.GlobalWorkspaceRegistryEntry{Workspace: gira.WorkspaceConfig{
				Name:      input.WorkspaceName,
				Owner:     "StatPan",
				InboxRepo: "StatPan/gira",
				Project:   gira.ProjectConfig{Owner: "StatPan", Title: input.WorkspaceName},
			}},
			DryRun: true,
			Status: "planned",
			Files: []gira.SetupGlobalFilePlan{
				{Path: "/tmp/gira/config.yaml", Action: "create"},
			},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"setup", "global", "--repo", "StatPan/gira", "--path", "/repo", "--config-root", "/tmp/gira", "--workspace", "personal", "--mode", "global-only", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.SetupGlobalReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode setup global JSON: %v\n%s", err, stdout.String())
	}
	if report.Status != "planned" || report.Mode != "global-only" || len(report.Files) != 1 {
		t.Fatalf("unexpected setup global report: %+v", report)
	}
	if report.SchemaVersion != gira.SetupGlobalReportSchemaVersion || report.Approval == nil {
		t.Fatalf("setup global dry-run JSON missing schema or approval:\n%s", stdout.String())
	}
	if report.Approval.SchemaVersion != gira.ApprovalPlanSchemaVersion || report.Approval.CanonicalCommand != "gira setup global" || report.Approval.OutputSchema != gira.SetupGlobalReportSchemaVersion {
		t.Fatalf("unexpected setup global approval evidence: %+v", report.Approval)
	}
	if report.Approval.ApplyCommand != "gira setup global --repo StatPan/gira --path /repo --config-root /tmp/gira --workspace personal --owner StatPan --inbox-repo StatPan/gira --mode global-only --project-owner StatPan --project-title personal --apply" || report.Approval.PostApplyVerification != "gira config doctor --repo StatPan/gira --config-root /tmp/gira --json" {
		t.Fatalf("unexpected setup global approval commands: %+v", report.Approval)
	}
	if report.Approval.Blockers == nil || report.Approval.Warnings == nil {
		t.Fatalf("approval blockers and warnings must be stable arrays: %+v", report.Approval)
	}
}

func TestSetupGlobalApplyJSONOmitsApprovalEvidence(t *testing.T) {
	original := newSetupGlobalReport
	t.Cleanup(func() { newSetupGlobalReport = original })
	newSetupGlobalReport = func(input gira.SetupGlobalInput) (gira.SetupGlobalReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Path != "/repo" || input.ConfigRoot != "/tmp/gira" || input.WorkspaceName != "personal" || input.Mode != "global-only" || !input.Apply || input.DryRun {
			t.Fatalf("unexpected setup global input: %+v", input)
		}
		return gira.SetupGlobalReport{
			Command:    "setup global",
			Mode:       input.Mode,
			ConfigRoot: input.ConfigRoot,
			Repo:       input.Repo.FullName(),
			Path:       input.Path,
			Workspace:  gira.WorkspaceSummary{Name: input.WorkspaceName, Owner: "StatPan"},
			InboxRepo:  "StatPan/gira",
			Applied:    true,
			Status:     "applied",
			Files: []gira.SetupGlobalFilePlan{
				{Path: "/tmp/gira/config.yaml", Action: "create"},
			},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"setup", "global", "--repo", "StatPan/gira", "--path", "/repo", "--config-root", "/tmp/gira", "--workspace", "personal", "--mode", "global-only", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.SetupGlobalReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode setup global JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.SetupGlobalReportSchemaVersion || !report.Applied {
		t.Fatalf("unexpected setup global apply report: %+v", report)
	}
	if report.Approval != nil {
		t.Fatalf("apply output should not include dry-run approval evidence: %+v", report.Approval)
	}
}

func TestSetupGlobalRequiresDryRunOrApply(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"setup", "global", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "exactly one of --dry-run/--apply is required") {
		t.Fatalf("stderr missing dry-run/apply message:\n%s", stderr.String())
	}
}

func TestRepoRegisterCommandJSON(t *testing.T) {
	original := newRepoRegisterReport
	t.Cleanup(func() { newRepoRegisterReport = original })
	newRepoRegisterReport = func(input gira.RepoRegisterInput) (gira.RepoRegisterReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Path != "/repo" || input.ConfigRoot != "/tmp/gira" || !input.DryRun || input.Apply {
			t.Fatalf("unexpected repo register input: %+v", input)
		}
		return gira.RepoRegisterReport{
			Command:    "repo register",
			Repo:       input.Repo.FullName(),
			ConfigRoot: input.ConfigRoot,
			Path:       input.Path,
			File:       "/tmp/gira/repos/StatPan/gira.yaml",
			DryRun:     true,
			Status:     "planned",
			Action:     "create",
			Entry:      gira.GlobalRepoRegistryEntry{Repo: input.Repo.FullName(), Path: input.Path},
			NextStep:   "gira repo register StatPan/gira --apply",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"repo", "register", "StatPan/gira", "--path", "/repo", "--config-root", "/tmp/gira", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.RepoRegisterReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode repo register JSON: %v\n%s", err, stdout.String())
	}
	if report.Action != "create" || report.Status != "planned" {
		t.Fatalf("unexpected repo register report: %+v", report)
	}
	if report.SchemaVersion != gira.RepoRegisterReportSchemaVersion || report.Approval == nil {
		t.Fatalf("repo register dry-run JSON missing schema or approval:\n%s", stdout.String())
	}
	if report.Approval.SchemaVersion != gira.ApprovalPlanSchemaVersion || report.Approval.CanonicalCommand != "gira repo register" || report.Approval.OutputSchema != gira.RepoRegisterReportSchemaVersion {
		t.Fatalf("unexpected repo register approval evidence: %+v", report.Approval)
	}
	if report.Approval.ApplyCommand != "gira repo register StatPan/gira --path /repo --config-root /tmp/gira --apply" || report.Approval.PostApplyVerification != "gira config repo --repo StatPan/gira --config-root /tmp/gira --json" {
		t.Fatalf("unexpected repo register approval commands: %+v", report.Approval)
	}
	if report.Approval.Blockers == nil || report.Approval.Warnings == nil {
		t.Fatalf("approval blockers and warnings must be stable arrays: %+v", report.Approval)
	}
}

func TestRepoRegisterApplyJSONOmitsApprovalEvidence(t *testing.T) {
	original := newRepoRegisterReport
	t.Cleanup(func() { newRepoRegisterReport = original })
	newRepoRegisterReport = func(input gira.RepoRegisterInput) (gira.RepoRegisterReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Path != "/repo" || input.ConfigRoot != "/tmp/gira" || !input.Apply || input.DryRun {
			t.Fatalf("unexpected repo register input: %+v", input)
		}
		return gira.RepoRegisterReport{
			Command:    "repo register",
			Repo:       input.Repo.FullName(),
			ConfigRoot: input.ConfigRoot,
			Path:       input.Path,
			File:       "/tmp/gira/repos/StatPan/gira.yaml",
			Applied:    true,
			Status:     "applied",
			Action:     "create",
			Entry:      gira.GlobalRepoRegistryEntry{Repo: input.Repo.FullName(), Path: input.Path},
			NextStep:   "gira config repo --repo StatPan/gira --config-root /tmp/gira",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"repo", "register", "StatPan/gira", "--path", "/repo", "--config-root", "/tmp/gira", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.RepoRegisterReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode repo register JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.RepoRegisterReportSchemaVersion || !report.Applied {
		t.Fatalf("unexpected repo register apply report: %+v", report)
	}
	if report.Approval != nil {
		t.Fatalf("apply output should not include dry-run approval evidence: %+v", report.Approval)
	}
}

func TestRepoRegisterRequiresRepo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"repo", "register", "--dry-run"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "repo register requires OWNER/REPO") {
		t.Fatalf("stderr missing required repo message:\n%s", stderr.String())
	}
}

func TestRepoMigrateCommandJSON(t *testing.T) {
	original := newRepoMigrateReport
	t.Cleanup(func() { newRepoMigrateReport = original })
	newRepoMigrateReport = func(input gira.RepoMigrateInput) (gira.RepoMigrateReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Path != "/repo" || input.ConfigRoot != "/tmp/gira" || !input.DryRun || input.Apply {
			t.Fatalf("unexpected repo migrate input: %+v", input)
		}
		return gira.RepoMigrateReport{
			Command:      "repo migrate",
			Repo:         input.Repo.FullName(),
			ConfigRoot:   input.ConfigRoot,
			Path:         input.Path,
			Contract:     ".gira/config.yaml",
			ContractFile: "/repo/.gira/config.yaml",
			File:         "/tmp/gira/repos/StatPan/gira.yaml",
			DryRun:       true,
			Status:       "planned",
			Action:       "create",
			Entry:        gira.GlobalRepoRegistryEntry{Repo: input.Repo.FullName(), Path: input.Path, Contract: ".gira/config.yaml"},
			NextStep:     "gira repo migrate --apply",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"repo", "migrate", "--repo", "StatPan/gira", "--path", "/repo", "--config-root", "/tmp/gira", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.RepoMigrateReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode repo migrate JSON: %v\n%s", err, stdout.String())
	}
	if report.Entry.Contract != ".gira/config.yaml" || report.Action != "create" {
		t.Fatalf("unexpected repo migrate report: %+v", report)
	}
	if report.SchemaVersion != gira.RepoMigrateReportSchemaVersion || report.Approval == nil {
		t.Fatalf("repo migrate dry-run JSON missing schema or approval:\n%s", stdout.String())
	}
	if report.Approval.SchemaVersion != gira.ApprovalPlanSchemaVersion || report.Approval.CanonicalCommand != "gira repo migrate" || report.Approval.OutputSchema != gira.RepoMigrateReportSchemaVersion {
		t.Fatalf("unexpected repo migrate approval evidence: %+v", report.Approval)
	}
	if report.Approval.ApplyCommand != "gira repo migrate --repo StatPan/gira --path /repo --config-root /tmp/gira --apply" || report.Approval.PostApplyVerification != "gira config repo --repo StatPan/gira --config-root /tmp/gira --json" {
		t.Fatalf("unexpected repo migrate approval commands: %+v", report.Approval)
	}
	if report.Approval.Blockers == nil || report.Approval.Warnings == nil {
		t.Fatalf("approval blockers and warnings must be stable arrays: %+v", report.Approval)
	}
}

func TestRepoMigrateApplyJSONOmitsApprovalEvidence(t *testing.T) {
	original := newRepoMigrateReport
	t.Cleanup(func() { newRepoMigrateReport = original })
	newRepoMigrateReport = func(input gira.RepoMigrateInput) (gira.RepoMigrateReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Path != "/repo" || input.ConfigRoot != "/tmp/gira" || !input.Apply || input.DryRun {
			t.Fatalf("unexpected repo migrate input: %+v", input)
		}
		return gira.RepoMigrateReport{
			Command:      "repo migrate",
			Repo:         input.Repo.FullName(),
			ConfigRoot:   input.ConfigRoot,
			Path:         input.Path,
			Contract:     ".gira/config.yaml",
			ContractFile: "/repo/.gira/config.yaml",
			File:         "/tmp/gira/repos/StatPan/gira.yaml",
			Applied:      true,
			Status:       "applied",
			Action:       "create",
			Entry:        gira.GlobalRepoRegistryEntry{Repo: input.Repo.FullName(), Path: input.Path, Contract: ".gira/config.yaml"},
			NextStep:     "gira config repo --repo StatPan/gira --config-root /tmp/gira",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"repo", "migrate", "--repo", "StatPan/gira", "--path", "/repo", "--config-root", "/tmp/gira", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.RepoMigrateReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode repo migrate JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.RepoMigrateReportSchemaVersion || !report.Applied {
		t.Fatalf("unexpected repo migrate apply report: %+v", report)
	}
	if report.Approval != nil {
		t.Fatalf("apply output should not include dry-run approval evidence: %+v", report.Approval)
	}
}

func TestRepoMigrateRejectsUnexpectedArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"repo", "migrate", "StatPan/gira", "--dry-run"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unexpected argument") {
		t.Fatalf("stderr missing unexpected argument message:\n%s", stderr.String())
	}
}

func TestUpgradeCommandHumanOutput(t *testing.T) {
	original := newUpgradeReport
	t.Cleanup(func() { newUpgradeReport = original })
	newUpgradeReport = func(options gira.UpgradeOptions) (gira.UpgradeReport, error) {
		if options.ChannelOverride != "pipx" {
			t.Fatalf("channel = %q, want pipx", options.ChannelOverride)
		}
		if options.NotifyOnce {
			t.Fatal("NotifyOnce = true, want false")
		}
		return gira.UpgradeReport{
			Current:  "v1.1.1",
			Latest:   "v1.2.0",
			Status:   "update_available",
			Channel:  "pipx",
			NextStep: "pipx upgrade gira-cli",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"upgrade", "--channel", "pipx"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"upgrade: gira", "current: v1.1.1", "latest:  v1.2.0", "pipx upgrade gira-cli"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("upgrade output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestUpdateAliasJSON(t *testing.T) {
	original := newUpgradeReport
	t.Cleanup(func() { newUpgradeReport = original })
	newUpgradeReport = func(options gira.UpgradeOptions) (gira.UpgradeReport, error) {
		return gira.UpgradeReport{
			Current:  "v1.2.0",
			Latest:   "v1.2.0",
			Status:   "up_to_date",
			Channel:  "homebrew",
			NextStep: "brew update && brew upgrade gira",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"update", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.UpgradeReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode upgrade JSON: %v\n%s", err, stdout.String())
	}
	if report.Status != "up_to_date" || report.Channel != "homebrew" {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func TestUpgradeNotifyOnceJSON(t *testing.T) {
	original := newUpgradeReport
	t.Cleanup(func() { newUpgradeReport = original })
	newUpgradeReport = func(options gira.UpgradeOptions) (gira.UpgradeReport, error) {
		if !options.NotifyOnce {
			t.Fatal("NotifyOnce = false, want true")
		}
		return gira.UpgradeReport{
			Current:  "v1.1.1",
			Latest:   "v1.2.0",
			Status:   "update_available",
			Channel:  "npm",
			NextStep: "npm update -g @statpan/gira",
			Notice: &gira.UpgradeNotice{
				Kind:    "new_version",
				Version: "v1.2.0",
				Status:  "emitted",
				Message: "new Gira release v1.2.0 is available; inspect next_step before upgrading",
			},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"update", "--notify-once", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.UpgradeReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode upgrade JSON: %v\n%s", err, stdout.String())
	}
	if report.Notice == nil || report.Notice.Kind != "new_version" || report.Notice.Status != "emitted" {
		t.Fatalf("unexpected notice: %#v", report.Notice)
	}
}

type cliFakeLatestReleaseFetcher struct {
	tag string
	err error
}

func (f cliFakeLatestReleaseFetcher) LatestReleaseTag() (string, error) {
	return f.tag, f.err
}

func TestPassiveUpgradeNoticeEmitsOnceToStderr(t *testing.T) {
	originalVersion := gira.Version
	t.Cleanup(func() { gira.Version = originalVersion })
	gira.Version = "v1.1.1"
	root := t.TempDir()
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	options := passiveUpgradeNoticeOptions{
		Env:           []string{"GIRA_INSTALL_CHANNEL=npm"},
		Now:           func() time.Time { return now },
		CheckInterval: 24 * time.Hour,
		NoticeRoot:    root,
		NewReport: func(input gira.UpgradeOptions) (gira.UpgradeReport, error) {
			input.ChannelOverride = "npm"
			input.ExecutablePath = "/tmp/gira"
			input.Fetcher = cliFakeLatestReleaseFetcher{tag: "v1.2.0"}
			return gira.BuildUpgradeReportWithOptions(input)
		},
	}

	var stderr bytes.Buffer
	emitPassiveUpgradeNotice([]string{"status"}, 0, &stderr, options)
	if got := stderr.String(); !strings.Contains(got, "gira: new release v1.2.0 available") || !strings.Contains(got, "npm update -g @statpan/gira") {
		t.Fatalf("passive notice stderr unexpected:\n%s", got)
	}

	stderr.Reset()
	emitPassiveUpgradeNotice([]string{"status"}, 0, &stderr, options)
	if stderr.String() != "" {
		t.Fatalf("repeated passive notice should be suppressed, got:\n%s", stderr.String())
	}
}

func TestPassiveUpgradeNoticeKeepsJSONStdoutClean(t *testing.T) {
	originalVersion := gira.Version
	t.Cleanup(func() { gira.Version = originalVersion })
	gira.Version = "v1.1.1"
	root := t.TempDir()
	original := passiveUpgradeNotice
	t.Cleanup(func() { passiveUpgradeNotice = original })
	passiveUpgradeNotice = func(args []string, exitCode int, stderr io.Writer) {
		emitPassiveUpgradeNotice(args, exitCode, stderr, passiveUpgradeNoticeOptions{
			Env:           []string{"GIRA_INSTALL_CHANNEL=npm"},
			Now:           func() time.Time { return time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC) },
			CheckInterval: 24 * time.Hour,
			NoticeRoot:    root,
			NewReport: func(input gira.UpgradeOptions) (gira.UpgradeReport, error) {
				input.ChannelOverride = "npm"
				input.ExecutablePath = "/tmp/gira"
				input.Fetcher = cliFakeLatestReleaseFetcher{tag: "v1.2.0"}
				return gira.BuildUpgradeReportWithOptions(input)
			},
		})
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"version", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var info gira.VersionInfo
	if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
		t.Fatalf("version JSON was contaminated: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if stderr.String() != "" {
		t.Fatalf("version command should skip passive notice, got stderr:\n%s", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"guide", "quickstart"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "gira: new release v1.2.0 available") {
		t.Fatalf("passive notice missing from stderr:\n%s", stderr.String())
	}
	if strings.Contains(stdout.String(), "new release") {
		t.Fatalf("passive notice contaminated stdout:\n%s", stdout.String())
	}
}

func TestPassiveUpgradeNoticeOptOutAndRecentCheck(t *testing.T) {
	if shouldRunPassiveUpgradeNotice([]string{"status"}, 0, []string{"GIRA_UPDATE_NOTICE=off"}, false) {
		t.Fatal("GIRA_UPDATE_NOTICE=off should disable passive notices")
	}
	if shouldRunPassiveUpgradeNotice([]string{"status"}, 0, []string{"GIRA_DISABLE_UPDATE_NOTICE=1"}, false) {
		t.Fatal("GIRA_DISABLE_UPDATE_NOTICE=1 should disable passive notices")
	}
	if shouldRunPassiveUpgradeNotice([]string{"status"}, 0, []string{"GIRA_UPDATE_NOTICE=on", "GIRA_DISABLE_UPDATE_NOTICE=1"}, false) {
		t.Fatal("GIRA_DISABLE_UPDATE_NOTICE=1 should win over GIRA_UPDATE_NOTICE=on")
	}
	if shouldRunPassiveUpgradeNotice([]string{"status"}, 1, nil, false) {
		t.Fatal("non-zero exit should skip passive notices")
	}

	originalVersion := gira.Version
	t.Cleanup(func() { gira.Version = originalVersion })
	gira.Version = "v1.2.0"
	root := t.TempDir()
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	if err := gira.MarkUpgradeNoticeChecked(root, now); err != nil {
		t.Fatalf("MarkUpgradeNoticeChecked error: %v", err)
	}
	var called bool
	var stderr bytes.Buffer
	emitPassiveUpgradeNotice([]string{"status"}, 0, &stderr, passiveUpgradeNoticeOptions{
		Now:           func() time.Time { return now.Add(time.Hour) },
		CheckInterval: 24 * time.Hour,
		NoticeRoot:    root,
		NewReport: func(input gira.UpgradeOptions) (gira.UpgradeReport, error) {
			called = true
			return gira.UpgradeReport{}, nil
		},
	})
	if called {
		t.Fatal("recent check should skip latest release fetch")
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestUpgradeCommandHelpAndError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"upgrade", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("help exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "gira update") {
		t.Fatalf("help output missing update alias:\n%s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"upgrade", "extra"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("unexpected argument exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unexpected argument: extra") {
		t.Fatalf("stderr missing unexpected argument:\n%s", stderr.String())
	}
}

func TestCachePruneRequiresMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"cache", "prune"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "exactly one of --dry-run or --apply is required") {
		t.Fatalf("stderr missing mode guidance:\n%s", stderr.String())
	}
}

func TestCacheHelpAndUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"cache", "help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "gira cache prune") {
		t.Fatalf("cache help missing prune command:\n%s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"cache", "missing"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown cache command: missing") || !strings.Contains(stderr.String(), "gira cache prune") {
		t.Fatalf("stderr missing cache remediation:\n%s", stderr.String())
	}
}

func TestCachePruneRejectsUnexpectedArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"cache", "prune", "--dry-run", "extra"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unexpected argument: extra") {
		t.Fatalf("stderr missing unexpected argument:\n%s", stderr.String())
	}
}

func TestCachePruneJSON(t *testing.T) {
	restore := newCachePruneReport
	t.Cleanup(func() { newCachePruneReport = restore })
	newCachePruneReport = func(options gira.CachePruneOptions) (gira.CachePruneReport, error) {
		if options.Root != "/tmp/gira-cache" || !options.DryRun || options.Apply {
			t.Fatalf("unexpected cache prune options: %#v", options)
		}
		return gira.CachePruneReport{
			Command:          "cache prune",
			Root:             "/tmp/gira-cache",
			ActiveVersion:    "v1.2.0",
			ActiveComparable: true,
			DryRun:           true,
			Counts:           gira.CachePruneCounts{Planned: 1, Skipped: 1},
			Actions: []gira.CachePruneAction{
				{Action: "prune", Status: "planned", Name: "v1.1.0", Path: "/tmp/gira-cache/v1.1.0", Reason: "would remove stale version directory"},
			},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"cache", "prune", "--root", "/tmp/gira-cache", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "cache prune"`, `"dry_run": true`, `"planned": 1`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("cache prune JSON missing %q:\n%s", want, stdout.String())
		}
	}
	var report gira.CachePruneReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode cache prune JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.CachePruneReportSchemaVersion || report.Approval == nil {
		t.Fatalf("cache prune dry-run JSON missing schema or approval:\n%s", stdout.String())
	}
	if report.Approval.SchemaVersion != gira.ApprovalPlanSchemaVersion || report.Approval.CanonicalCommand != "gira cache prune" || report.Approval.OutputSchema != gira.CachePruneReportSchemaVersion {
		t.Fatalf("unexpected cache prune approval evidence: %+v", report.Approval)
	}
	if report.Approval.ApplyCommand != "gira cache prune --root /tmp/gira-cache --apply" || report.Approval.PostApplyVerification != "gira cache prune --root /tmp/gira-cache --dry-run --json" {
		t.Fatalf("unexpected cache prune approval commands: %+v", report.Approval)
	}
	if report.Approval.Blockers == nil || report.Approval.Warnings == nil {
		t.Fatalf("approval blockers and warnings must be stable arrays: %+v", report.Approval)
	}
}

func TestCachePruneApplyBuilderWiring(t *testing.T) {
	restore := newCachePruneReport
	t.Cleanup(func() { newCachePruneReport = restore })
	newCachePruneReport = func(options gira.CachePruneOptions) (gira.CachePruneReport, error) {
		if options.Root != "" || options.DryRun || !options.Apply {
			t.Fatalf("unexpected cache prune options: %#v", options)
		}
		return gira.CachePruneReport{
			Command:       "cache prune",
			Root:          "/home/me/.cache/gira-cli",
			ActiveVersion: "v1.2.0",
			DryRun:        false,
			Counts:        gira.CachePruneCounts{Applied: 1},
			Actions: []gira.CachePruneAction{
				{Action: "prune", Status: "applied", Name: "v1.1.0", Path: "/home/me/.cache/gira-cli/v1.1.0", Reason: "removed stale version directory"},
			},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"cache", "prune", "--apply"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "mode: apply") || !strings.Contains(stdout.String(), "applied prune: v1.1.0") {
		t.Fatalf("cache prune text missing apply summary:\n%s", stdout.String())
	}
}

func TestCachePruneApplyJSONOmitsApprovalEvidence(t *testing.T) {
	restore := newCachePruneReport
	t.Cleanup(func() { newCachePruneReport = restore })
	newCachePruneReport = func(options gira.CachePruneOptions) (gira.CachePruneReport, error) {
		if options.Root != "/tmp/gira-cache" || options.DryRun || !options.Apply {
			t.Fatalf("unexpected cache prune options: %#v", options)
		}
		return gira.CachePruneReport{
			Command:       "cache prune",
			Root:          "/tmp/gira-cache",
			ActiveVersion: "v1.2.0",
			Apply:         true,
			Counts:        gira.CachePruneCounts{Applied: 1},
			Actions: []gira.CachePruneAction{
				{Action: "prune", Status: "applied", Name: "v1.1.0", Path: "/tmp/gira-cache/v1.1.0", Reason: "removed stale version directory"},
			},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"cache", "prune", "--root", "/tmp/gira-cache", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.CachePruneReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode cache prune JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.CachePruneReportSchemaVersion || !report.Apply {
		t.Fatalf("unexpected cache prune apply report: %+v", report)
	}
	if report.Approval != nil {
		t.Fatalf("apply output should not include dry-run approval evidence: %+v", report.Approval)
	}
}

func TestGuideQuickstartDefault(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"guide"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"Gira quickstart", "gira new", "gira ticket checks", "gira ticket finish --apply", "gira ticket handoff TICKET", "gira dispatch goal GOAL"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("guide output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestDocsAliasAndGuideTopics(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"docs", "agent"}, "docs/skills/gira-agent-operator.md"},
		{[]string{"guide", "skill"}, "Use --dry-run before --apply"},
		{[]string{"guide", "ticket"}, "Registry-backed commands:"},
		{[]string{"guide", "stats"}, "Closure Funnel reports"},
		{[]string{"guide", "jira"}, "gira jira export"},
		{[]string{"guide", "concepts"}, "Jira terms on GitHub"},
		{[]string{"guide", "capabilities"}, "gira-command-capabilities/v1"},
		{[]string{"guide", "--help"}, "Topics:"},
	}
	for _, tc := range cases {
		var stdout, stderr bytes.Buffer
		code := Run(tc.args, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("%v exit code = %d, want 0; stderr: %s", tc.args, code, stderr.String())
		}
		if !strings.Contains(stdout.String(), tc.want) {
			t.Fatalf("%v output missing %q:\n%s", tc.args, tc.want, stdout.String())
		}
	}
}

func TestGuideCapabilitiesJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"guide", "capabilities", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.CommandCapabilityReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode guide capabilities JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.CommandCapabilitySchemaVersion {
		t.Fatalf("schema version = %q, want %q", report.SchemaVersion, gira.CommandCapabilitySchemaVersion)
	}
	foundTicketStart := false
	foundCanonicalEntrypoint := false
	for _, command := range report.Commands {
		if command.Canonical == "gira ticket start" {
			foundTicketStart = true
			if command.Capability != gira.AdapterCapabilityApplyMutation || command.JSONSupport != gira.JSONSupportStable {
				t.Fatalf("unexpected ticket start capability: %+v", command)
			}
		}
		if command.Canonical == "gira ticket handoff" {
			foundCanonicalEntrypoint = command.Tier == gira.CommandTierManagedDelivery && command.WorkflowRole == "canonical_single_issue_agent_entry_point"
		}
	}
	if !foundTicketStart {
		t.Fatalf("capability report missing gira ticket start: %+v", report.Commands)
	}
	if !foundCanonicalEntrypoint {
		t.Fatalf("capability report missing canonical single-issue entry point metadata: %+v", report.Commands)
	}
}

func TestTicketGuideUsesCommandRegistry(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"guide", "ticket"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{
		`gira ticket new "Title" --dry-run|--apply`,
		"Create a repo-bound executable GitHub issue",
		"Example: gira ticket new --title \"TITLE\" --body-file issue.md --dry-run",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("ticket guide missing registry-backed %q:\n%s", want, stdout.String())
		}
	}
}

func TestGuideUnknownTopic(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"guide", "missing"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown guide topic: missing") || !strings.Contains(stderr.String(), "gira guide [quickstart|ticket|stats|jira|agent|skill|concepts|capabilities]") {
		t.Fatalf("stderr missing guide remediation:\n%s", stderr.String())
	}
}

func TestStatsRepoCommandText(t *testing.T) {
	restore := newStatsRepoReport
	t.Cleanup(func() { newStatsRepoReport = restore })
	newStatsRepoReport = func(input gira.StatsRepoOptions) (gira.StatsRepoReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Since != "30d" || input.StaleDays != 10 || input.Limit != 50 {
			t.Fatalf("unexpected stats input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.StatsRepoReport{
			Command:    "stats repo",
			Repo:       input.Repo.FullName(),
			Window:     gira.StatsWindow{Since: input.Since, StaleDays: input.StaleDays, Limit: input.Limit},
			Source:     gira.StatsSource{Backend: "github", ReadOnly: true},
			Metrics:    gira.StatsRepoMetrics{OpenedIssues: 4, MergedPRsWithLinkedIssues: 2, ClosureRate: 0.5},
			Confidence: gira.StatsConfidence{Level: "medium", Signals: []string{"status labels"}},
			NonGoals:   []string{"personal productivity score"},
			NextSteps:  []string{"Improve closing links."},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"stats", "repo", "--repo", "StatPan/gira", "--since", "30d", "--stale-days", "10", "--limit", "50"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"Gira Closure Funnel", "opened issues: 4", "closure rate: 50.0%"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stats output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestStatsRepoCommandJSON(t *testing.T) {
	restore := newStatsRepoReport
	t.Cleanup(func() { newStatsRepoReport = restore })
	newStatsRepoReport = func(input gira.StatsRepoOptions) (gira.StatsRepoReport, error) {
		return gira.StatsRepoReport{
			Command: "stats repo",
			Repo:    input.Repo.FullName(),
			Window:  gira.StatsWindow{Since: input.Since},
			Metrics: gira.StatsRepoMetrics{ClosureRate: 1},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"stats", "repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.StatsRepoReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode stats JSON: %v\n%s", err, stdout.String())
	}
	if report.Command != "stats repo" || report.Repo != "StatPan/gira" {
		t.Fatalf("unexpected stats report: %+v", report)
	}
}

func TestStatsPulseCommandText(t *testing.T) {
	restore := newStatsPulseReport
	t.Cleanup(func() { newStatsPulseReport = restore })
	newStatsPulseReport = func(input gira.StatsPulseOptions) (gira.PulseReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Since != "14d" || input.Limit != 25 {
			t.Fatalf("unexpected pulse input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.PulseReport{
			SchemaVersion: "pulse-report/v1alpha1",
			Command:       "stats pulse",
			Scope:         gira.PulseScope{Kind: "repo", Repo: input.Repo.FullName()},
			Window:        gira.PulseWindow{Since: input.Since, Until: "2026-05-11T00:00:00Z"},
			Source:        gira.StatsSource{Backend: "github", ReadOnly: true},
			Summary:       gira.PulseSummary{Finished: 1, Reviewed: 1},
			Health:        gira.PulseHealth{ReviewNeeded: 2},
			Items: []gira.PulseItem{{
				Kind: "finished", Repo: input.Repo.FullName(), Issue: 7, PR: 10, Title: "Finish pulse", Evidence: []string{"merged_pr", "closing_reference"},
			}},
			PrivacyBoundary: gira.PulsePrivacyBoundary{Scope: "work_item_state_only"},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"stats", "pulse", "--repo", "StatPan/gira", "--since", "14d", "--limit", "25"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"Gira Pulse", "finished: 1", "review needed: 2", "Finish pulse"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("pulse output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestStatsPulseCommandJSON(t *testing.T) {
	restore := newStatsPulseReport
	t.Cleanup(func() { newStatsPulseReport = restore })
	newStatsPulseReport = func(input gira.StatsPulseOptions) (gira.PulseReport, error) {
		return gira.PulseReport{
			SchemaVersion: "pulse-report/v1alpha1",
			Command:       "stats pulse",
			Scope:         gira.PulseScope{Kind: "repo", Repo: input.Repo.FullName()},
			Window:        gira.PulseWindow{Since: input.Since},
			Summary:       gira.PulseSummary{Finished: 1},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"stats", "pulse", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.PulseReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode pulse JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != "pulse-report/v1alpha1" || report.Command != "stats pulse" || report.Scope.Repo != "StatPan/gira" {
		t.Fatalf("unexpected pulse report: %+v", report)
	}
}

func TestStatsWorkspacePlannedAndUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"stats", "workspace", "--since", "30d"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "gira stats workspace is planned") || !strings.Contains(stdout.String(), "gira stats repo --repo OWNER/REPO") {
		t.Fatalf("workspace planned output missing guidance:\n%s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"stats", "missing"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown stats command: missing") || !strings.Contains(stderr.String(), "gira stats repo") {
		t.Fatalf("stderr missing stats remediation:\n%s", stderr.String())
	}
}

func TestStatsRepoRejectsDuplicateRepoAndReportsBuilderError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"stats", "repo", "StatPan/gira", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "repo specified both positionally and with --repo") {
		t.Fatalf("stderr missing duplicate repo guidance:\n%s", stderr.String())
	}

	restore := newStatsRepoReport
	t.Cleanup(func() { newStatsRepoReport = restore })
	newStatsRepoReport = func(input gira.StatsRepoOptions) (gira.StatsRepoReport, error) {
		return gira.StatsRepoReport{}, fmt.Errorf("--limit must be greater than 0")
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"stats", "repo", "--repo", "StatPan/gira", "--limit", "-1"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--limit must be greater than 0") {
		t.Fatalf("stderr missing builder error:\n%s", stderr.String())
	}

	restorePulse := newStatsPulseReport
	t.Cleanup(func() { newStatsPulseReport = restorePulse })
	newStatsPulseReport = func(input gira.StatsPulseOptions) (gira.PulseReport, error) {
		return gira.PulseReport{}, fmt.Errorf("--since must be a positive day window like 90d or YYYY-MM-DD")
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"stats", "pulse", "--repo", "StatPan/gira", "--since", "0d"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--since must be a positive day window") {
		t.Fatalf("stderr missing pulse builder error:\n%s", stderr.String())
	}
}

func TestWorkspaceInitProjectFlagsJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"workspace",
		"init",
		"--inbox-repo",
		"StatPan/backlog",
		"--name",
		"personal",
		"--owner",
		"StatPan",
		"--project-owner",
		"GiraOrg",
		"--project-title",
		"Roadmap",
		"--project-number",
		"12",
		"--dry-run",
		"--json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.WorkspaceInitReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode workspace init JSON: %v\n%s", err, stdout.String())
	}
	if report.Project.Owner != "GiraOrg" || report.Project.Title != "Roadmap" || report.Project.Number != 12 {
		t.Fatalf("project = %+v, want CLI overrides", report.Project)
	}
	for _, want := range []string{"finish_review_policy: required", "owner: GiraOrg", "title: \"Roadmap\"", "number: 12"} {
		if !strings.Contains(report.Content, want) {
			t.Fatalf("content missing %q:\n%s", want, report.Content)
		}
	}
}

func TestWorkspaceInitMergeFlagJSON(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".gira", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("repo: StatPan/gira\nprofiles:\n  default:\n    labels: []\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"workspace",
		"init",
		"--inbox-repo",
		"StatPan/backlog",
		"--repo",
		"StatPan/gira",
		"--path",
		dir,
		"--merge",
		"--dry-run",
		"--json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.WorkspaceInitReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode workspace init JSON: %v\n%s", err, stdout.String())
	}
	if !report.Merge || report.MergeBlock == "" || !strings.Contains(report.Content, "workspace:") {
		t.Fatalf("unexpected merge report: %+v", report)
	}
}

func TestWorkspaceInitGlobalScopeJSON(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"workspace",
		"init",
		"--inbox-repo",
		"StatPan/backlog",
		"--name",
		"personal",
		"--scope",
		"global",
		"--config-root",
		root,
		"--dry-run",
		"--json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.WorkspaceInitReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode workspace init JSON: %v\n%s", err, stdout.String())
	}
	if report.Scope != "global" || report.ConfigRoot != root || report.ConfigPath != filepath.Join(root, "workspaces", "personal.yaml") {
		t.Fatalf("unexpected global workspace init report: %+v", report)
	}
}

func TestVersionCommandHumanOutput(t *testing.T) {
	originalVersion, originalCommit, originalDate := gira.Version, gira.Commit, gira.Date
	t.Cleanup(func() {
		gira.Version, gira.Commit, gira.Date = originalVersion, originalCommit, originalDate
	})
	gira.Version = "v1.2.3"
	gira.Commit = "abc123"
	gira.Date = "2026-05-05T12:00:00Z"

	var stdout, stderr bytes.Buffer
	code := Run([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if got, want := stdout.String(), "gira v1.2.3 (abc123, 2026-05-05T12:00:00Z)\n"; got != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
}

func TestVersionCommandJSON(t *testing.T) {
	originalVersion, originalCommit, originalDate := gira.Version, gira.Commit, gira.Date
	t.Cleanup(func() {
		gira.Version, gira.Commit, gira.Date = originalVersion, originalCommit, originalDate
	})
	gira.Version = "v1.2.3"
	gira.Commit = "abc123"
	gira.Date = "2026-05-05T12:00:00Z"

	var stdout, stderr bytes.Buffer
	code := Run([]string{"version", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"version": "v1.2.3"`, `"commit": "abc123"`, `"date": "2026-05-05T12:00:00Z"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("version JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRootVersionFlag(t *testing.T) {
	originalVersion, originalCommit, originalDate := gira.Version, gira.Commit, gira.Date
	t.Cleanup(func() {
		gira.Version, gira.Commit, gira.Date = originalVersion, originalCommit, originalDate
	})
	gira.Version = "v9.9.9"
	gira.Commit = "abc123"
	gira.Date = "2026-05-05T12:00:00Z"

	var stdout, stderr bytes.Buffer
	code := Run([]string{"--version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "gira v9.9.9 ") {
		t.Fatalf("--version output unexpected:\n%s", stdout.String())
	}
}

func TestVersionCommandFallsBackForEmptyBuildValues(t *testing.T) {
	originalVersion, originalCommit, originalDate := gira.Version, gira.Commit, gira.Date
	t.Cleanup(func() {
		gira.Version, gira.Commit, gira.Date = originalVersion, originalCommit, originalDate
	})
	gira.Version = ""
	gira.Commit = ""
	gira.Date = ""

	var stdout, stderr bytes.Buffer
	code := Run([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if got, want := stdout.String(), "gira dev (unknown, unknown)\n"; got != want {
		t.Fatalf("version fallback output = %q, want %q", got, want)
	}
}

func TestJiraImportRequiresMode(t *testing.T) {
	for _, args := range [][]string{
		{"jira", "import", "--repo", "StatPan/gira", "--source", "jira.csv"},
		{"jira", "import", "--repo", "StatPan/gira", "--source", "jira.csv", "--dry-run", "--apply"},
	} {
		var stdout, stderr bytes.Buffer
		code := Run(args, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("Run(%v) exit code = %d, want 2", args, code)
		}
		if !strings.Contains(stderr.String(), "exactly one of --dry-run or --apply is required") {
			t.Fatalf("Run(%v) stderr missing mode guidance:\n%s", args, stderr.String())
		}
	}
}

func TestJiraImportRejectsMixedSourceAndAPI(t *testing.T) {
	for _, args := range [][]string{
		{"jira", "import", "--repo", "StatPan/gira", "--dry-run"},
		{"jira", "import", "--repo", "StatPan/gira", "--source", "jira.csv", "--api-base", "https://jira.example", "--project", "GIRA", "--dry-run"},
		{"jira", "import", "--repo", "StatPan/gira", "--api-base", "https://jira.example", "--dry-run"},
	} {
		var stdout, stderr bytes.Buffer
		code := Run(args, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("Run(%v) exit code = %d, want 2", args, code)
		}
		if !strings.Contains(stderr.String(), "--source") && !strings.Contains(stderr.String(), "--api-base and --project") {
			t.Fatalf("Run(%v) stderr missing source/API guidance:\n%s", args, stderr.String())
		}
	}
}

func TestJiraInitJSON(t *testing.T) {
	restore := newJiraProviderInitReport
	t.Cleanup(func() { newJiraProviderInitReport = restore })
	newJiraProviderInitReport = func(input gira.JiraProviderInitInput) (gira.JiraProviderInitReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.APIBase != "https://jira.example" || input.Project != "ABC" || input.ConfigRoot != "/tmp/gira" || !input.Overwrite || !input.DryRun || input.Apply {
			t.Fatalf("unexpected jira init input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.JiraProviderInitReport{
			Command:    "jira init",
			Repo:       input.Repo.FullName(),
			APIBase:    input.APIBase,
			ConfigRoot: input.ConfigRoot,
			DryRun:     true,
			Status:     "planned",
			ReadOnly:   true,
			Project:    gira.JiraProviderProject{ID: "10000", Key: "ABC", Name: "Alpha"},
			Statuses:   []gira.JiraProviderStatus{{ID: "1", Name: "To Do"}},
			ConfigProposal: gira.JiraProviderConfigProposal{
				Provider: gira.JiraProviderConfig{Enabled: true, Mode: "primary", ProjectKey: "ABC"},
				GitHub:   gira.JiraProviderGitHub{Repo: input.Repo.FullName(), MirrorIssue: true, MirrorLabels: true},
			},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"jira", "init", "--repo", "StatPan/gira", "--api-base", "https://jira.example", "--project", "ABC", "--config-root", "/tmp/gira", "--overwrite", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "jira init"`, `"read_only": true`, `"project_key": "ABC"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("jira init JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestJiraInitApplyText(t *testing.T) {
	restore := newJiraProviderInitReport
	t.Cleanup(func() { newJiraProviderInitReport = restore })
	newJiraProviderInitReport = func(input gira.JiraProviderInitInput) (gira.JiraProviderInitReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.ConfigRoot != "/tmp/gira" || !input.Apply || input.DryRun {
			t.Fatalf("unexpected jira init input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.JiraProviderInitReport{
			Command:    "jira init",
			Repo:       input.Repo.FullName(),
			APIBase:    input.APIBase,
			ConfigRoot: input.ConfigRoot,
			Apply:      true,
			Applied:    true,
			Status:     "applied",
			File:       gira.SetupGlobalFilePlan{Path: "/tmp/gira/repos/StatPan/gira.yaml", Action: "create"},
			ReadOnly:   true,
			Project:    gira.JiraProviderProject{ID: "10000", Key: "ABC", Name: "Alpha"},
			ConfigProposal: gira.JiraProviderConfigProposal{
				Provider: gira.JiraProviderConfig{Enabled: true, Mode: "primary", ProjectKey: "ABC"},
				GitHub:   gira.JiraProviderGitHub{Repo: input.Repo.FullName(), MirrorIssue: true, MirrorLabels: true},
			},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"jira", "init", "--repo", "StatPan/gira", "--api-base", "https://jira.example", "--project", "ABC", "--config-root", "/tmp/gira", "--apply"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "jira init: applied provider discovery") || !strings.Contains(stdout.String(), "config_action: create") {
		t.Fatalf("stdout missing apply report:\n%s", stdout.String())
	}
}

func TestJiraDoctorJSON(t *testing.T) {
	restore := newJiraDoctorReport
	t.Cleanup(func() { newJiraDoctorReport = restore })
	newJiraDoctorReport = func(input gira.JiraDoctorInput) (gira.JiraDoctorReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.APIBase != "https://jira.example" || input.Project != "ABC" || input.SampleKey != "ABC-123" || input.ConfigRoot != "/tmp/gira" {
			t.Fatalf("unexpected jira doctor input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.JiraDoctorReport{
			Command:       "jira doctor",
			Repo:          input.Repo.FullName(),
			APIBase:       input.APIBase,
			ProjectKey:    input.Project,
			ConfigRoot:    input.ConfigRoot,
			Status:        "warning",
			Compatibility: "partially_supported",
			ReadOnly:      true,
			Checks: []gira.JiraDoctorCheck{
				{Name: "transition_reachability", Status: "warning", Detail: "sample issue required"},
			},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"jira", "doctor", "--repo", "StatPan/gira", "--api-base", "https://jira.example", "--project", "ABC", "--sample-key", "ABC-123", "--config-root", "/tmp/gira", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "jira doctor"`, `"read_only": true`, `"id": "transition_reachability"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("jira doctor JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestJiraDoctorText(t *testing.T) {
	restore := newJiraDoctorReport
	t.Cleanup(func() { newJiraDoctorReport = restore })
	newJiraDoctorReport = func(input gira.JiraDoctorInput) (gira.JiraDoctorReport, error) {
		return gira.JiraDoctorReport{
			Command:       "jira doctor",
			Repo:          input.Repo.FullName(),
			ProjectKey:    "ABC",
			Status:        "ready",
			Compatibility: "supported",
			ReadOnly:      true,
			Checks:        []gira.JiraDoctorCheck{{Name: "provider_config", Status: "ready", Detail: "loaded"}},
			Mirror:        gira.JiraDoctorMirrorDiagnostic{Status: "ready", IssueCount: 1, MirrorCount: 1},
			Transitions:   gira.JiraDoctorTransitionCheck{Status: "ready", SampleKey: "ABC-123", Detail: "direct transition"},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"jira", "doctor", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"jira doctor: ready (supported)", "provider_config: ready", "transition_sample: ready ABC-123"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("jira doctor text missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestJiraMirrorJSON(t *testing.T) {
	restore := newJiraMirrorReport
	t.Cleanup(func() { newJiraMirrorReport = restore })
	newJiraMirrorReport = func(input gira.JiraMirrorInput) (gira.JiraMirrorReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Key != "ABC-123" || input.APIBase != "https://jira.example" || input.ConfigRoot != "/tmp/gira" || !input.DryRun || input.Apply {
			t.Fatalf("unexpected jira mirror input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.JiraMirrorReport{
			Command:  "jira mirror",
			Repo:     input.Repo.FullName(),
			Key:      input.Key,
			APIBase:  input.APIBase,
			DryRun:   true,
			Status:   "planned",
			Action:   "create",
			Labels:   []string{"jira:ABC-123", "status:ready"},
			NextStep: "gira jira mirror ABC-123 --repo StatPan/gira --apply",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"jira", "mirror", "ABC-123", "--repo", "StatPan/gira", "--api-base", "https://jira.example", "--config-root", "/tmp/gira", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "jira mirror"`, `"action": "create"`, `"key": "ABC-123"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("jira mirror JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestJiraMirrorRejectsDuplicatePositionalKeys(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"jira", "mirror", "ABC-123", "abc-123", "--repo", "StatPan/gira", "--api-base", "https://jira.example", "--dry-run"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "only one Jira key can be provided") {
		t.Fatalf("stderr missing duplicate key guidance:\n%s", stderr.String())
	}
}

func TestJiraTransitionJSON(t *testing.T) {
	restore := newJiraTransitionPlanReport
	t.Cleanup(func() { newJiraTransitionPlanReport = restore })
	newJiraTransitionPlanReport = func(input gira.JiraTransitionPlanInput) (gira.JiraTransitionPlanReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Key != "ABC-123" || input.Target != "in_progress" || input.APIBase != "https://jira.example" || input.ConfigRoot != "/tmp/gira" || !input.DryRun {
			t.Fatalf("unexpected jira transition input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.JiraTransitionPlanReport{
			Command:       "jira transition",
			Repo:          input.Repo.FullName(),
			Key:           input.Key,
			APIBase:       input.APIBase,
			CurrentStatus: "To Do",
			Target:        input.Target,
			TargetStatuses: []string{
				"In Progress",
			},
			Candidate: gira.JiraTransitionCandidate{ID: "21", Name: "Start Progress", ToStatus: "In Progress"},
			Decision:  "direct_transition",
			DryRun:    true,
			ReadOnly:  true,
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"jira", "transition", "ABC-123", "--repo", "StatPan/gira", "--to", "in_progress", "--api-base", "https://jira.example", "--config-root", "/tmp/gira", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"schema_version": "jira-transition-plan/v1"`, `"command": "jira transition"`, `"decision": "direct_transition"`, `"key": "ABC-123"`, `"read_only": true`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("jira transition JSON missing %q:\n%s", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), `"approval"`) {
		t.Fatalf("jira transition JSON must not emit approval evidence:\n%s", stdout.String())
	}
}

func TestJiraImportJSON(t *testing.T) {
	restore := newJiraImportReport
	t.Cleanup(func() { newJiraImportReport = restore })
	newJiraImportReport = func(repo gira.RepoRef, source string, apiBase string, project string, dryRun bool, apply bool) (gira.JiraImportReport, error) {
		if repo.FullName() != "StatPan/gira" || source != "jira.csv" || apiBase != "" || project != "" || !dryRun || apply {
			t.Fatalf("unexpected jira import args repo=%s source=%s apiBase=%s project=%s dryRun=%t apply=%t", repo.FullName(), source, apiBase, project, dryRun, apply)
		}
		return gira.JiraImportReport{
			Command: "jira import",
			Repo:    "StatPan/gira",
			Source:  "jira.csv",
			DryRun:  true,
			Counts:  gira.JiraImportCounts{SourceItems: 1, Create: 1},
			Actions: []gira.JiraImportAction{{Key: "GIRA-1", Action: "create", Title: "Import me"}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"jira", "import", "--repo", "StatPan/gira", "--source", "jira.csv", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "jira import"`, `"key": "GIRA-1"`, `"create": 1`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("jira import JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestJiraImportAPIJSON(t *testing.T) {
	restore := newJiraImportReport
	t.Cleanup(func() { newJiraImportReport = restore })
	newJiraImportReport = func(repo gira.RepoRef, source string, apiBase string, project string, dryRun bool, apply bool) (gira.JiraImportReport, error) {
		if repo.FullName() != "StatPan/gira" || source != "" || apiBase != "https://jira.example" || project != "GIRA" || dryRun || !apply {
			t.Fatalf("unexpected jira API import args repo=%s source=%s apiBase=%s project=%s dryRun=%t apply=%t", repo.FullName(), source, apiBase, project, dryRun, apply)
		}
		return gira.JiraImportReport{
			Command: "jira import",
			Repo:    "StatPan/gira",
			APIBase: "https://jira.example",
			Project: "GIRA",
			Apply:   true,
			Counts:  gira.JiraImportCounts{SourceItems: 1, Create: 1, Applied: 1},
			Actions: []gira.JiraImportAction{{Key: "GIRA-9", Action: "create", Title: "API import", IssueNumber: 91}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"jira", "import", "--repo", "StatPan/gira", "--api-base", "https://jira.example", "--project", "GIRA", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"api_base": "https://jira.example"`, `"project": "GIRA"`, `"applied": 1`, `"issue_number": 91`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("jira API import JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestJiraExportJSON(t *testing.T) {
	restore := newJiraExportReport
	t.Cleanup(func() { newJiraExportReport = restore })
	newJiraExportReport = func(repo gira.RepoRef, outputRoot string) (gira.JiraExportReport, error) {
		if repo.FullName() != "StatPan/gira" || outputRoot != "out/jira" {
			t.Fatalf("unexpected jira export args repo=%s output=%s", repo.FullName(), outputRoot)
		}
		report := gira.JiraExportReport{
			Command:       "jira export",
			Repo:          "StatPan/gira",
			OutputRoot:    "out/jira",
			SchemaVersion: gira.JiraExportSchemaVersion,
			Artifacts:     []gira.JiraExportArtifact{{Path: "out/jira/issues.json", Kind: "json"}},
		}
		report.Counts.Issues = 2
		return report, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"jira", "export", "--repo", "StatPan/gira", "--output", "out/jira", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "jira export"`, `"output_root": "out/jira"`, `"issues": 2`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("jira export JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestPortfolioPlanRequiresDryRun(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"portfolio", "plan"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--dry-run is required for portfolio plan") {
		t.Fatalf("stderr missing dry-run guidance:\n%s", stderr.String())
	}
}

func TestPortfolioLowerRequiresMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"portfolio", "lower"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "exactly one of --dry-run or --apply is required") {
		t.Fatalf("stderr missing mode guidance:\n%s", stderr.String())
	}
}

func TestDetachRequiresMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"detach", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "exactly one of --dry-run or --apply is required") {
		t.Fatalf("stderr missing mode guidance:\n%s", stderr.String())
	}
}

func TestDetachDryRunJSON(t *testing.T) {
	restore := newDetachReport
	t.Cleanup(func() { newDetachReport = restore })
	newDetachReport = func(repo gira.RepoRef, dryRun bool, apply bool) (gira.DetachReport, error) {
		if repo.FullName() != "StatPan/gira" || !dryRun || apply {
			t.Fatalf("unexpected detach args repo=%s dryRun=%t apply=%t", repo.FullName(), dryRun, apply)
		}
		return gira.DetachReport{
			Repo:         "StatPan/gira",
			Command:      "detach",
			DryRun:       true,
			Counts:       gira.DetachCounts{CloseBootstrapIssues: 1, DeleteLabels: 1, ManualFiles: 1},
			Actions:      []gira.DetachAction{{Kind: "bootstrap_issue", Action: "close", Target: "Slice", Number: 10, Reason: "bootstrap issues are closed instead of deleted", Status: "planned"}},
			ManagedFiles: []gira.DetachManagedFile{{Path: "AGENTS.md", Action: "manual_remove", Reason: "detach has no local path flag in this slice"}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"detach", "--repo", "StatPan/gira", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "detach"`, `"dry_run": true`, `"managed_files"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("detach JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestDetachApplyText(t *testing.T) {
	restore := newDetachReport
	t.Cleanup(func() { newDetachReport = restore })
	newDetachReport = func(repo gira.RepoRef, dryRun bool, apply bool) (gira.DetachReport, error) {
		if repo.FullName() != "StatPan/gira" || dryRun || !apply {
			t.Fatalf("unexpected detach args repo=%s dryRun=%t apply=%t", repo.FullName(), dryRun, apply)
		}
		return gira.DetachReport{
			Repo:    "StatPan/gira",
			Command: "detach",
			DryRun:  false,
			Counts:  gira.DetachCounts{CloseBootstrapIssues: 1},
			Actions: []gira.DetachAction{{Kind: "bootstrap_issue", Action: "close", Target: "Slice", Number: 10, Status: "applied"}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"detach", "--repo", "StatPan/gira", "--apply"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "mode: apply") || !strings.Contains(stdout.String(), "close bootstrap_issue: Slice") {
		t.Fatalf("detach text missing apply summary:\n%s", stdout.String())
	}
}

func TestPortfolioLowerJSON(t *testing.T) {
	restore := newPortfolioLowerReport
	t.Cleanup(func() { newPortfolioLowerReport = restore })
	newPortfolioLowerReport = func(configPath string, apply bool) (gira.PortfolioLowerReport, error) {
		if configPath != "testdata/portfolio.yaml" || apply {
			t.Fatalf("unexpected lower args config=%s apply=%t", configPath, apply)
		}
		return gira.PortfolioLowerReport{
			Command:       "portfolio lower",
			PortfolioRepo: "StatPan/portfolio",
			Repos:         []string{"StatPan/gira"},
			DryRun:        true,
			Counts:        gira.PortfolioLowerCounts{Tickets: 1, OpenTickets: 1, Actions: 1},
			Actions:       []gira.PortfolioLowerAction{{Ticket: 144, Action: "execution_issue:create", Repo: "StatPan/gira", Reason: "no lowered issue found for target repo"}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"portfolio", "lower", "--dry-run", "--config", "testdata/portfolio.yaml", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "portfolio lower"`, `"action": "execution_issue:create"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("portfolio lower JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestPortfolioCapabilityJSON(t *testing.T) {
	restore := newPortfolioCapabilityReport
	t.Cleanup(func() { newPortfolioCapabilityReport = restore })
	newPortfolioCapabilityReport = func(configPath string) (gira.PortfolioCapabilityReport, error) {
		if configPath != "testdata/portfolio.yaml" {
			t.Fatalf("unexpected config path: %s", configPath)
		}
		return gira.PortfolioCapabilityReport{
			Command:       "portfolio capability",
			PortfolioRepo: "StatPan/portfolio",
			Token:         gira.ProjectCapabilityTokenSummary{Kind: "pat", Identity: "alice"},
			Repos: []gira.PortfolioRepoCapability{{
				Repo: "StatPan/gira",
				Role: "execution",
				Mode: "write",
				Capabilities: map[string]gira.ProjectCapabilityStatus{
					"issues:read":  gira.ProjectCapabilityAllowed,
					"issues:write": gira.ProjectCapabilityAllowed,
				},
			}},
			FetchedAt: "2026-05-05T12:00:00Z",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"portfolio", "capability", "--config", "testdata/portfolio.yaml", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "portfolio capability"`, `"portfolio_repo": "StatPan/portfolio"`, `"issues:write": "allowed"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("portfolio capability JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestPortfolioCapabilityBlockedExitOne(t *testing.T) {
	restore := newPortfolioCapabilityReport
	t.Cleanup(func() { newPortfolioCapabilityReport = restore })
	newPortfolioCapabilityReport = func(configPath string) (gira.PortfolioCapabilityReport, error) {
		return gira.PortfolioCapabilityReport{
			Command:       "portfolio capability",
			PortfolioRepo: "StatPan/portfolio",
			Token:         gira.ProjectCapabilityTokenSummary{Kind: "pat", Identity: "alice"},
			Repos: []gira.PortfolioRepoCapability{{
				Repo: "StatPan/gira",
				Role: "execution",
				Mode: "read-only",
				Capabilities: map[string]gira.ProjectCapabilityStatus{
					"issues:read":  gira.ProjectCapabilityAllowed,
					"issues:write": gira.ProjectCapabilityDeniedScope,
				},
			}},
			BlockedActions: []gira.PortfolioCapabilityBlock{{CheckID: "execution:StatPan/gira:issues:write", Repo: "StatPan/gira", Role: "execution", Required: "issues:write", Reason: "token scope or repository permission is insufficient"}},
			FetchedAt:      "2026-05-05T12:00:00Z",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"portfolio", "capability"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "blocked actions") {
		t.Fatalf("stdout missing blocked actions:\n%s", stdout.String())
	}
}

func TestPortfolioPlanJSON(t *testing.T) {
	restore := newPortfolioReport
	t.Cleanup(func() { newPortfolioReport = restore })
	newPortfolioReport = func(command string, configPath string, dryRun bool) (gira.PortfolioReport, error) {
		if command != "plan" || configPath != "testdata/portfolio.yaml" || !dryRun {
			t.Fatalf("unexpected portfolio args command=%s config=%s dryRun=%t", command, configPath, dryRun)
		}
		return gira.PortfolioReport{
			Command:       "portfolio plan",
			PortfolioRepo: "StatPan/portfolio",
			Repos:         []string{"StatPan/gira"},
			DryRun:        true,
			Counts:        gira.PortfolioCounts{Tickets: 1, OpenTickets: 1, Actions: 1},
			Actions:       []gira.PortfolioPlanAction{{Ticket: 140, Action: "execution_issue:create", Repo: "StatPan/gira", Reason: "no child issue linked for target repo"}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"portfolio", "plan", "--dry-run", "--config", "testdata/portfolio.yaml", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"portfolio_repo": "StatPan/portfolio"`, `"action": "execution_issue:create"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("portfolio JSON missing %q:\n%s", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "next step:") {
		t.Fatalf("portfolio JSON contains human prose:\n%s", stdout.String())
	}
}

func TestPortfolioPlanPermissionBlocksExitOne(t *testing.T) {
	restore := newPortfolioReport
	t.Cleanup(func() { newPortfolioReport = restore })
	newPortfolioReport = func(command string, configPath string, dryRun bool) (gira.PortfolioReport, error) {
		return gira.PortfolioReport{
			Command:          "portfolio plan",
			PortfolioRepo:    "StatPan/portfolio",
			Repos:            []string{"StatPan/gira"},
			DryRun:           true,
			Counts:           gira.PortfolioCounts{Tickets: 1, OpenTickets: 1, Actions: 1},
			Actions:          []gira.PortfolioPlanAction{{Ticket: 145, Action: "execution_issue:create", Repo: "StatPan/gira", Reason: "no child issue linked for target repo"}},
			PermissionBlocks: []gira.PortfolioCapabilityBlock{{CheckID: "execution:StatPan/gira:issues:write", Repo: "StatPan/gira", Role: "execution", Required: "issues:write", Reason: "issue write capability cannot be proven non-destructively with this token"}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"portfolio", "plan", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"permission_blocks"`) {
		t.Fatalf("stdout missing permission blocks:\n%s", stdout.String())
	}
}

func TestPortfolioValidateDiagnosticsExitOne(t *testing.T) {
	restore := newPortfolioReport
	t.Cleanup(func() { newPortfolioReport = restore })
	newPortfolioReport = func(command string, configPath string, dryRun bool) (gira.PortfolioReport, error) {
		if command != "validate" || dryRun {
			t.Fatalf("unexpected portfolio args command=%s dryRun=%t", command, dryRun)
		}
		return gira.PortfolioReport{
			Command:       "portfolio validate",
			PortfolioRepo: "StatPan/portfolio",
			Repos:         []string{"StatPan/gira"},
			Counts:        gira.PortfolioCounts{Tickets: 1, OpenTickets: 1, Diagnostics: 1},
			Diagnostics:   []gira.PortfolioDiagnostic{{Ticket: 140, RuleID: "missing_required_field", Detail: "goal is required"}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"portfolio", "validate"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ticket #140 missing_required_field") {
		t.Fatalf("stdout missing diagnostic:\n%s", stdout.String())
	}
}

func TestPortfolioValidateJSONDiagnosticsExitOne(t *testing.T) {
	restore := newPortfolioReport
	t.Cleanup(func() { newPortfolioReport = restore })
	newPortfolioReport = func(command string, configPath string, dryRun bool) (gira.PortfolioReport, error) {
		if command != "validate" || dryRun {
			t.Fatalf("unexpected portfolio args command=%s dryRun=%t", command, dryRun)
		}
		return gira.PortfolioReport{
			Command:       "portfolio validate",
			PortfolioRepo: "StatPan/portfolio",
			Repos:         []string{"StatPan/gira"},
			Counts:        gira.PortfolioCounts{Tickets: 1, OpenTickets: 1, Diagnostics: 1},
			Diagnostics:   []gira.PortfolioDiagnostic{{Ticket: 140, RuleID: "invalid_child_issue", Detail: "bad must be OWNER/REPO#N"}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"portfolio", "validate", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"rule_id": "invalid_child_issue"`) {
		t.Fatalf("stdout missing JSON diagnostic:\n%s", stdout.String())
	}
}

func TestWorkspaceStatusJSON(t *testing.T) {
	restore := newWorkspaceStatusReportWithOptions
	t.Cleanup(func() { newWorkspaceStatusReportWithOptions = restore })
	newWorkspaceStatusReportWithOptions = func(configPath string, options gira.WorkspaceStatusOptions) (gira.WorkspaceReport, error) {
		if configPath != "testdata/workspace.yaml" {
			t.Fatalf("unexpected config path: %s", configPath)
		}
		if options.CacheTTL != 5*time.Minute || options.MaxConcurrency != 4 || options.Refresh {
			t.Fatalf("unexpected workspace status options: %+v", options)
		}
		return gira.WorkspaceReport{
			Workspace: gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"},
			Inbox:     gira.WorkspaceInbox{Repo: "StatPan/backlog", Open: 1, NeedsRouting: 1},
			Repos:     []gira.WorkspaceRepo{{Repo: "StatPan/gira", Open: 2, Ready: 1}},
			Counts:    gira.WorkspaceCounts{Backlog: 3, InboxOpen: 1, RepoOpen: 2, Ready: 1},
			Queues:    gira.BuildWorkspaceQueues(gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"}, []gira.WorkStatusResult{{Repo: "StatPan/gira", Issue: 10, Title: "Ready issue", State: "open", Status: "Ready"}}),
			Backlog:   []gira.WorkspaceBacklogItem{{Source: "inbox", Repo: "StatPan/backlog", Number: 7, Title: "Route later", State: "open", NeedsRouting: true}},
			NextSteps: []string{"gira workspace ticket route --ticket 7 --repo OWNER/REPO --dry-run"},
			FetchedAt: "2026-05-06T00:00:00Z",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "status", "--config", "testdata/workspace.yaml", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"workspace"`, `"repo": "StatPan/backlog"`, `"needs_routing": true`, `"workspace_queues"`, `"agent_ready": 1`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("workspace status JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestWorkspaceStatusPassesOptimizationOptions(t *testing.T) {
	restore := newWorkspaceStatusReportWithOptions
	t.Cleanup(func() { newWorkspaceStatusReportWithOptions = restore })
	newWorkspaceStatusReportWithOptions = func(configPath string, options gira.WorkspaceStatusOptions) (gira.WorkspaceReport, error) {
		if configPath != "testdata/workspace.yaml" {
			t.Fatalf("unexpected config path: %s", configPath)
		}
		if len(options.Repos) != 2 || options.Repos[0].FullName() != "StatPan/app-a" || options.Repos[1].FullName() != "StatPan/app-b" {
			t.Fatalf("unexpected repo filters: %+v", options.Repos)
		}
		if options.Limit != 5 || !options.ActiveOnly || options.MaxConcurrency != 2 || options.CacheTTL != time.Minute || !options.Refresh || options.CacheRoot != "/tmp/gira-cache" {
			t.Fatalf("unexpected options: %+v", options)
		}
		return gira.WorkspaceReport{
			Workspace: gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"},
			Inbox:     gira.WorkspaceInbox{Repo: "StatPan/backlog"},
			FetchedAt: "2026-05-06T00:00:00Z",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "status", "--config", "testdata/workspace.yaml", "--repo", "StatPan/app-a,StatPan/app-b", "--limit", "5", "--active-only", "--max-concurrency", "2", "--cache-ttl", "1m", "--refresh", "--cache-root", "/tmp/gira-cache"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
}

func TestQueueListJSONUsesWorkspaceQueuesContract(t *testing.T) {
	restore := newWorkspaceStatusReportWithOptions
	t.Cleanup(func() { newWorkspaceStatusReportWithOptions = restore })
	newWorkspaceStatusReportWithOptions = func(configPath string, options gira.WorkspaceStatusOptions) (gira.WorkspaceReport, error) {
		if configPath != "testdata/workspace.yaml" {
			t.Fatalf("unexpected config path: %s", configPath)
		}
		if len(options.Repos) != 1 || options.Repos[0].FullName() != "StatPan/gira" || options.CacheTTL != time.Minute || options.MaxConcurrency != 2 || !options.Refresh {
			t.Fatalf("unexpected workspace options: %+v", options)
		}
		workspace := gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"}
		return gira.WorkspaceReport{
			Workspace:  workspace,
			ConfigPath: configPath,
			Queues: gira.BuildWorkspaceQueues(workspace, []gira.WorkStatusResult{
				{Repo: "StatPan/gira", Issue: 10, Title: "Ready issue", State: "open", Status: "Ready", Labels: []string{"status:ready"}},
				{Repo: "StatPan/gira", Issue: 11, Title: "Blocked issue", State: "open", Status: "Blocked", Labels: []string{"status:blocked"}},
			}),
			FetchedAt: "2026-06-06T00:00:00Z",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"queue", "list",
		"--config", "testdata/workspace.yaml",
		"--repo", "StatPan/gira",
		"--queue", "ready",
		"--limit", "1",
		"--max-concurrency", "2",
		"--cache-ttl", "1m",
		"--refresh",
		"--json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"schema_version": "queue-list/v1"`, `"source_contract": "workspace-queues/v1"`, `"agent_ready"`, `"queue": "agent_ready"`, `"limit": 1`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("queue list JSON missing %q:\n%s", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "Blocked issue") {
		t.Fatalf("queue filter leaked blocked item:\n%s", stdout.String())
	}
}

func TestQueueListTextUsesShortQueueNames(t *testing.T) {
	restore := newWorkspaceStatusReportWithOptions
	t.Cleanup(func() { newWorkspaceStatusReportWithOptions = restore })
	newWorkspaceStatusReportWithOptions = func(configPath string, options gira.WorkspaceStatusOptions) (gira.WorkspaceReport, error) {
		workspace := gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"}
		return gira.WorkspaceReport{
			Workspace:  workspace,
			ConfigPath: ".gira/config.yaml",
			Queues: gira.BuildWorkspaceQueues(workspace, []gira.WorkStatusResult{
				{Repo: "StatPan/gira", Issue: 10, Title: "Ready issue", State: "open", Status: "Ready", Labels: []string{"status:ready"}},
			}),
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"queue", "list", "--compact"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ready   ") || !strings.Contains(stdout.String(), "StatPan/gira#10") {
		t.Fatalf("queue list text missing short queue name:\n%s", stdout.String())
	}
}

func TestQueueNextJSONSelectsHandoffAndRunCommands(t *testing.T) {
	restore := newWorkspaceStatusReportWithOptions
	t.Cleanup(func() { newWorkspaceStatusReportWithOptions = restore })
	newWorkspaceStatusReportWithOptions = func(configPath string, options gira.WorkspaceStatusOptions) (gira.WorkspaceReport, error) {
		workspace := gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"}
		return gira.WorkspaceReport{
			Workspace:  workspace,
			ConfigPath: ".gira/config.yaml",
			Queues: gira.BuildWorkspaceQueues(workspace, []gira.WorkStatusResult{
				{Repo: "StatPan/gira", Issue: 10, Title: "Ready issue", State: "open", Status: "Ready", Labels: []string{"status:ready"}},
			}),
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"queue", "next", "--role", "planner", "--profile", "python", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{
		`"schema_version": "queue-next/v1"`,
		`"next_action": "handoff_ticket"`,
		`"selection_reason": "first_agent_ready:ticket_ready"`,
		`"handoff_command": "gira ticket handoff --repo StatPan/gira --ticket 10 planner --profile python --json"`,
		`"run_command": "gira run start 10 --repo StatPan/gira --role planner --profile python --dry-run"`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("queue next JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestQueueNextJSONReportsStopReasonsWithoutSelection(t *testing.T) {
	restore := newWorkspaceStatusReportWithOptions
	t.Cleanup(func() { newWorkspaceStatusReportWithOptions = restore })
	newWorkspaceStatusReportWithOptions = func(configPath string, options gira.WorkspaceStatusOptions) (gira.WorkspaceReport, error) {
		workspace := gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"}
		return gira.WorkspaceReport{
			Workspace:  workspace,
			ConfigPath: ".gira/config.yaml",
			Queues: gira.BuildWorkspaceQueues(workspace, []gira.WorkStatusResult{
				{Repo: "StatPan/gira", Issue: 11, Title: "Blocked issue", State: "open", Status: "Blocked", Labels: []string{"status:blocked"}},
			}),
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"queue", "next", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"selected": null`, `"no_agent_ready_item"`, `"blocked_present"`, `"next_action": "inspect_queues"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("queue next stop JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestQueueHandoffJSONEmbedsWorkerHandoff(t *testing.T) {
	restoreWorkspace := newWorkspaceStatusReportWithOptions
	restoreHandoff := newTicketHandoffReport
	t.Cleanup(func() {
		newWorkspaceStatusReportWithOptions = restoreWorkspace
		newTicketHandoffReport = restoreHandoff
	})
	newWorkspaceStatusReportWithOptions = func(configPath string, options gira.WorkspaceStatusOptions) (gira.WorkspaceReport, error) {
		workspace := gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"}
		return gira.WorkspaceReport{
			Workspace:  workspace,
			ConfigPath: ".gira/config.yaml",
			Queues: gira.BuildWorkspaceQueues(workspace, []gira.WorkStatusResult{
				{Repo: "StatPan/gira", Issue: 10, Title: "Ready issue", State: "open", Status: "Ready", Labels: []string{"status:ready"}},
			}),
		}, nil
	}
	newTicketHandoffReport = func(input gira.TicketHandoffInput) (gira.TicketHandoffReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Ticket != 10 || input.Role != "reviewer" || input.Profile != "python" {
			t.Fatalf("unexpected handoff input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.TicketHandoffReport{
			Command:       "ticket handoff",
			SchemaVersion: gira.WorkerHandoffSchemaVersion,
			Repo:          input.Repo.FullName(),
			Issue:         input.Ticket,
			Role:          input.Role,
			Profile:       input.Profile,
			Readiness:     gira.TicketReadinessReport{SchemaVersion: gira.TicketReadinessSchemaVersion, Readiness: "ready"},
			NextAction:    "request_review",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"queue", "handoff", "--role", "reviewer", "--profile", "python", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{
		`"schema_version": "queue-handoff/v1"`,
		`"worker_handoff"`,
		`"schema_version": "worker-handoff/v1"`,
		`"next_action": "start_run"`,
		`"run_command": "gira run start 10 --repo StatPan/gira --role reviewer --profile python --dry-run"`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("queue handoff JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestQueueHandoffJSONReportsNoWorkWithoutCallingHandoff(t *testing.T) {
	restoreWorkspace := newWorkspaceStatusReportWithOptions
	restoreHandoff := newTicketHandoffReport
	t.Cleanup(func() {
		newWorkspaceStatusReportWithOptions = restoreWorkspace
		newTicketHandoffReport = restoreHandoff
	})
	newWorkspaceStatusReportWithOptions = func(configPath string, options gira.WorkspaceStatusOptions) (gira.WorkspaceReport, error) {
		workspace := gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"}
		return gira.WorkspaceReport{Workspace: workspace, ConfigPath: ".gira/config.yaml", Queues: gira.BuildWorkspaceQueues(workspace, nil)}, nil
	}
	newTicketHandoffReport = func(input gira.TicketHandoffInput) (gira.TicketHandoffReport, error) {
		t.Fatalf("handoff builder should not be called for no-work report")
		return gira.TicketHandoffReport{}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"queue", "handoff", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"selected": null`, `"worker_handoff": null`, `"no_agent_ready_item"`, `"no_queue_items"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("queue handoff no-work JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestQueueHandoffExplicitBlockedTicketStopsWithoutCallingHandoff(t *testing.T) {
	restoreWorkspace := newWorkspaceStatusReportWithOptions
	restoreHandoff := newTicketHandoffReport
	t.Cleanup(func() {
		newWorkspaceStatusReportWithOptions = restoreWorkspace
		newTicketHandoffReport = restoreHandoff
	})
	newWorkspaceStatusReportWithOptions = func(configPath string, options gira.WorkspaceStatusOptions) (gira.WorkspaceReport, error) {
		if len(options.Repos) != 1 || options.Repos[0].FullName() != "StatPan/gira" {
			t.Fatalf("unexpected repo filter: %+v", options.Repos)
		}
		workspace := gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"}
		return gira.WorkspaceReport{
			Workspace:  workspace,
			ConfigPath: ".gira/config.yaml",
			Queues: gira.BuildWorkspaceQueues(workspace, []gira.WorkStatusResult{
				{Repo: "StatPan/gira", Issue: 11, Title: "Blocked issue", State: "open", Status: "Blocked", Labels: []string{"status:blocked"}},
			}),
		}, nil
	}
	newTicketHandoffReport = func(input gira.TicketHandoffInput) (gira.TicketHandoffReport, error) {
		t.Fatalf("handoff builder should not be called for blocked queue item")
		return gira.TicketHandoffReport{}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"queue", "handoff", "--repo", "StatPan/gira", "--ticket", "11", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"queue_not_handoff_safe"`, `"queue_blocked"`, `"worker_handoff": null`, `"next_action": "inspect_queues"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("queue handoff blocked JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestQueueHandoffExplicitHumanTicketStopsWithoutCallingHandoff(t *testing.T) {
	restoreWorkspace := newWorkspaceStatusReportWithOptions
	restoreHandoff := newTicketHandoffReport
	t.Cleanup(func() {
		newWorkspaceStatusReportWithOptions = restoreWorkspace
		newTicketHandoffReport = restoreHandoff
	})
	newWorkspaceStatusReportWithOptions = func(configPath string, options gira.WorkspaceStatusOptions) (gira.WorkspaceReport, error) {
		workspace := gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"}
		return gira.WorkspaceReport{
			Workspace:  workspace,
			ConfigPath: ".gira/config.yaml",
			Queues: gira.BuildWorkspaceQueues(workspace, []gira.WorkStatusResult{
				{Repo: "StatPan/gira", Issue: 12, Title: "Human decision", State: "open", Status: "Ready", Labels: []string{"status:ready", "needs:human"}},
			}),
		}, nil
	}
	newTicketHandoffReport = func(input gira.TicketHandoffInput) (gira.TicketHandoffReport, error) {
		t.Fatalf("handoff builder should not be called for human-decision queue item")
		return gira.TicketHandoffReport{}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"queue", "handoff", "--repo", "StatPan/gira", "--ticket", "12", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"queue_not_handoff_safe"`, `"queue_human_decision"`, `"reason_label_needs_human"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("queue handoff human JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestQueueTakeDryRunJSONDelegatesTicketStart(t *testing.T) {
	restoreWorkspace := newWorkspaceStatusReportWithOptions
	restoreHandoff := newTicketHandoffReport
	restoreStart := newWorkStartResultWithOptions
	t.Cleanup(func() {
		newWorkspaceStatusReportWithOptions = restoreWorkspace
		newTicketHandoffReport = restoreHandoff
		newWorkStartResultWithOptions = restoreStart
	})
	newWorkspaceStatusReportWithOptions = func(configPath string, options gira.WorkspaceStatusOptions) (gira.WorkspaceReport, error) {
		workspace := gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"}
		return gira.WorkspaceReport{
			Workspace:  workspace,
			ConfigPath: ".gira/config.yaml",
			Queues: gira.BuildWorkspaceQueues(workspace, []gira.WorkStatusResult{
				{Repo: "StatPan/gira", Issue: 10, Title: "Ready issue", State: "open", Status: "Ready", Labels: []string{"status:ready"}},
			}),
		}, nil
	}
	newTicketHandoffReport = func(input gira.TicketHandoffInput) (gira.TicketHandoffReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Ticket != 10 || input.Role != "implementer" || input.Profile != "default" {
			t.Fatalf("unexpected handoff input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.TicketHandoffReport{
			Command:       "ticket handoff",
			SchemaVersion: gira.WorkerHandoffSchemaVersion,
			Repo:          input.Repo.FullName(),
			Issue:         input.Ticket,
			Role:          input.Role,
			Profile:       input.Profile,
			Readiness:     gira.TicketReadinessReport{SchemaVersion: gira.TicketReadinessSchemaVersion, Readiness: "ready"},
			NextAction:    "implement",
		}, nil
	}
	startCalled := false
	newWorkStartResultWithOptions = func(repo gira.RepoRef, issue int, options gira.WorkStartOptions) (gira.WorkStartResult, error) {
		startCalled = true
		if repo.FullName() != "StatPan/gira" || issue != 10 || !options.DryRun {
			t.Fatalf("unexpected start input: repo=%s issue=%d options=%+v", repo.FullName(), issue, options)
		}
		return gira.WorkStartResult{
			Repo:       repo.FullName(),
			Issue:      issue,
			Title:      "Ready issue",
			Branch:     "issue-10-ready-issue",
			DryRun:     true,
			Status:     "Ready",
			NextStatus: "In progress",
			NextStep:   "gira ticket start 10 --apply",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"queue", "take", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !startCalled {
		t.Fatalf("ticket start builder was not called")
	}
	for _, want := range []string{
		`"schema_version": "queue-take/v1"`,
		`"dry_run": true`,
		`"apply": false`,
		`"worker_handoff"`,
		`"start_result"`,
		`"schema_version": "work-start-result/v1"`,
		`"branch": "issue-10-ready-issue"`,
		`"next_action": "apply_ticket_start"`,
		`"next_step": "gira queue take --repo StatPan/gira --ticket 10 --apply"`,
		`"canonical_command": "gira queue take"`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("queue take dry-run JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestQueueTakeApplyJSONDelegatesTicketStart(t *testing.T) {
	restoreWorkspace := newWorkspaceStatusReportWithOptions
	restoreHandoff := newTicketHandoffReport
	restoreStart := newWorkStartResultWithOptions
	t.Cleanup(func() {
		newWorkspaceStatusReportWithOptions = restoreWorkspace
		newTicketHandoffReport = restoreHandoff
		newWorkStartResultWithOptions = restoreStart
	})
	newWorkspaceStatusReportWithOptions = func(configPath string, options gira.WorkspaceStatusOptions) (gira.WorkspaceReport, error) {
		workspace := gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"}
		return gira.WorkspaceReport{
			Workspace:  workspace,
			ConfigPath: ".gira/config.yaml",
			Queues: gira.BuildWorkspaceQueues(workspace, []gira.WorkStatusResult{
				{Repo: "StatPan/gira", Issue: 10, Title: "Ready issue", State: "open", Status: "Ready", Labels: []string{"status:ready"}},
			}),
		}, nil
	}
	newTicketHandoffReport = func(input gira.TicketHandoffInput) (gira.TicketHandoffReport, error) {
		return gira.TicketHandoffReport{
			Command:       "ticket handoff",
			SchemaVersion: gira.WorkerHandoffSchemaVersion,
			Repo:          input.Repo.FullName(),
			Issue:         input.Ticket,
			Role:          input.Role,
			Profile:       input.Profile,
			Readiness:     gira.TicketReadinessReport{SchemaVersion: gira.TicketReadinessSchemaVersion, Readiness: "ready"},
			NextAction:    "implement",
		}, nil
	}
	startCalled := false
	newWorkStartResultWithOptions = func(repo gira.RepoRef, issue int, options gira.WorkStartOptions) (gira.WorkStartResult, error) {
		startCalled = true
		if repo.FullName() != "StatPan/gira" || issue != 10 || options.DryRun {
			t.Fatalf("unexpected start input: repo=%s issue=%d options=%+v", repo.FullName(), issue, options)
		}
		return gira.WorkStartResult{
			Repo:       repo.FullName(),
			Issue:      issue,
			Title:      "Ready issue",
			Branch:     "issue-10-ready-issue",
			DryRun:     false,
			Status:     "Ready",
			NextStatus: "In progress",
			NextStep:   "gira ticket pr --dry-run",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"queue", "take", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !startCalled {
		t.Fatalf("ticket start builder was not called")
	}
	for _, want := range []string{
		`"schema_version": "queue-take/v1"`,
		`"dry_run": false`,
		`"apply": true`,
		`"next_action": "handoff_ticket"`,
		`"next_step": "gira ticket handoff --repo StatPan/gira --ticket 10 implementer --json"`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("queue take apply JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestQueueTakeApplyRefusesUnsafeQueuesWithoutCallingStart(t *testing.T) {
	restoreWorkspace := newWorkspaceStatusReportWithOptions
	restoreHandoff := newTicketHandoffReport
	restoreStart := newWorkStartResultWithOptions
	t.Cleanup(func() {
		newWorkspaceStatusReportWithOptions = restoreWorkspace
		newTicketHandoffReport = restoreHandoff
		newWorkStartResultWithOptions = restoreStart
	})
	newWorkspaceStatusReportWithOptions = func(configPath string, options gira.WorkspaceStatusOptions) (gira.WorkspaceReport, error) {
		if len(options.Repos) != 1 || options.Repos[0].FullName() != "StatPan/gira" {
			t.Fatalf("unexpected repo filter: %+v", options.Repos)
		}
		workspace := gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"}
		return gira.WorkspaceReport{
			Workspace:  workspace,
			ConfigPath: ".gira/config.yaml",
			Queues: gira.BuildWorkspaceQueues(workspace, []gira.WorkStatusResult{
				{Repo: "StatPan/gira", Issue: 11, Title: "Blocked issue", State: "open", Status: "Blocked", Labels: []string{"status:blocked"}},
			}),
		}, nil
	}
	newTicketHandoffReport = func(input gira.TicketHandoffInput) (gira.TicketHandoffReport, error) {
		t.Fatalf("handoff builder should not be called for blocked queue item")
		return gira.TicketHandoffReport{}, nil
	}
	newWorkStartResultWithOptions = func(repo gira.RepoRef, issue int, options gira.WorkStartOptions) (gira.WorkStartResult, error) {
		t.Fatalf("ticket start builder should not be called for blocked queue item")
		return gira.WorkStartResult{}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"queue", "take", "--repo", "StatPan/gira", "--ticket", "11", "--apply", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr: %s stdout: %s", code, stderr.String(), stdout.String())
	}
	for _, want := range []string{`"schema_version": "queue-take/v1"`, `"queue_not_handoff_safe"`, `"queue_blocked"`, `"start_result": null`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("queue take blocked JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestQueueTakeApplyRefusesWorkerHandoffNotReadyWithoutCallingStart(t *testing.T) {
	restoreWorkspace := newWorkspaceStatusReportWithOptions
	restoreHandoff := newTicketHandoffReport
	restoreStart := newWorkStartResultWithOptions
	t.Cleanup(func() {
		newWorkspaceStatusReportWithOptions = restoreWorkspace
		newTicketHandoffReport = restoreHandoff
		newWorkStartResultWithOptions = restoreStart
	})
	newWorkspaceStatusReportWithOptions = func(configPath string, options gira.WorkspaceStatusOptions) (gira.WorkspaceReport, error) {
		workspace := gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"}
		return gira.WorkspaceReport{
			Workspace:  workspace,
			ConfigPath: ".gira/config.yaml",
			Queues: gira.BuildWorkspaceQueues(workspace, []gira.WorkStatusResult{
				{Repo: "StatPan/gira", Issue: 10, Title: "Weak ticket", State: "open", Status: "Ready", Labels: []string{"status:ready"}},
			}),
		}, nil
	}
	newTicketHandoffReport = func(input gira.TicketHandoffInput) (gira.TicketHandoffReport, error) {
		return gira.TicketHandoffReport{
			Command:       "ticket handoff",
			SchemaVersion: gira.WorkerHandoffSchemaVersion,
			Repo:          input.Repo.FullName(),
			Issue:         input.Ticket,
			Role:          input.Role,
			Profile:       input.Profile,
			Readiness: gira.TicketReadinessReport{
				SchemaVersion: gira.TicketReadinessSchemaVersion,
				Readiness:     "needs_refinement",
				Findings:      []gira.TicketReadinessFinding{{Severity: "error", Kind: "missing_scope"}},
			},
			NextAction:      "refine_ticket",
			NextSafeCommand: "gira ticket view --repo StatPan/gira --ticket 10",
		}, nil
	}
	newWorkStartResultWithOptions = func(repo gira.RepoRef, issue int, options gira.WorkStartOptions) (gira.WorkStartResult, error) {
		t.Fatalf("ticket start builder should not be called when worker handoff is not ready")
		return gira.WorkStartResult{}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"queue", "take", "--apply", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr: %s stdout: %s", code, stderr.String(), stdout.String())
	}
	for _, want := range []string{`"worker_handoff_not_ready"`, `"readiness_needs_refinement"`, `"finding_missing_scope"`, `"next_action": "refine_ticket"`, `"start_result": null`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("queue take worker-not-ready JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestWorkspaceListAliasesBacklog(t *testing.T) {
	restore := newWorkspaceStatusReportWithOptions
	t.Cleanup(func() { newWorkspaceStatusReportWithOptions = restore })
	newWorkspaceStatusReportWithOptions = func(configPath string, options gira.WorkspaceStatusOptions) (gira.WorkspaceReport, error) {
		if configPath != "testdata/workspace.yaml" {
			t.Fatalf("unexpected config path: %s", configPath)
		}
		return gira.WorkspaceReport{
			Workspace: gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"},
			Inbox:     gira.WorkspaceInbox{Repo: "StatPan/backlog", Open: 1, NeedsRouting: 1},
			Backlog:   []gira.WorkspaceBacklogItem{{Source: "inbox", Repo: "StatPan/backlog", Number: 7, Title: "Route later", State: "open", Status: "needs-routing"}},
			NextSteps: []string{"should be replaced by list alias"},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "list", "--config", "testdata/workspace.yaml"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"backlog:", "StatPan/backlog#7", "next step: gira workspace status --config .gira/config.yaml"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("workspace list output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestWorkspaceCapabilityCommandTextUsesInjectedReport(t *testing.T) {
	restore := newWorkspaceCapabilityReport
	t.Cleanup(func() { newWorkspaceCapabilityReport = restore })
	newWorkspaceCapabilityReport = func(configPath string) (gira.WorkspaceCapabilityReport, error) {
		if configPath != "testdata/workspace.yaml" {
			t.Fatalf("unexpected config path: %s", configPath)
		}
		return gira.WorkspaceCapabilityReport{
			Command:   "workspace capability",
			Workspace: gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"},
			Token:     gira.ProjectCapabilityTokenSummary{Kind: "pat", Identity: "alice"},
			Repos: []gira.PortfolioRepoCapability{
				{Repo: "StatPan/backlog", Role: "inbox", Mode: "write", Capabilities: map[string]gira.ProjectCapabilityStatus{"issues:read": gira.ProjectCapabilityAllowed, "issues:write": gira.ProjectCapabilityAllowed}},
				{Repo: "StatPan/gira", Role: "execution", Mode: "read-only", Capabilities: map[string]gira.ProjectCapabilityStatus{"issues:read": gira.ProjectCapabilityAllowed, "issues:write": gira.ProjectCapabilityDeniedScope}},
			},
			BlockedActions: []gira.PortfolioCapabilityBlock{{CheckID: "execution:StatPan/gira:issues:write", Repo: "StatPan/gira", Role: "execution", Required: "issues:write", Reason: "token scope or repository permission is insufficient"}},
			FetchedAt:      "2026-05-13T00:00:00Z",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "capability", "--config", "testdata/workspace.yaml"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"workspace capability", "workspace: personal (StatPan)", "token: alice (pat)", "StatPan/gira [execution] mode=read-only", "blocked actions:", "requires issues:write"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("workspace capability output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestWorkspaceValidateCommandJSONUsesInjectedReport(t *testing.T) {
	restore := newWorkspaceValidateReport
	t.Cleanup(func() { newWorkspaceValidateReport = restore })
	newWorkspaceValidateReport = func(configPath string) (gira.WorkspaceValidateReport, error) {
		if configPath != "testdata/workspace.yaml" {
			t.Fatalf("unexpected config path: %s", configPath)
		}
		return gira.WorkspaceValidateReport{
			Command:   "workspace validate",
			Workspace: gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"},
			InboxRepo: "StatPan/backlog",
			Items: []gira.WorkspaceValidateItem{
				{Ticket: 7, Title: "Route work", State: "open", Status: "routeable", Reason: "ticket can be routed into execution repo", TargetRepos: []string{"StatPan/gira"}, NextStep: "gira workspace ticket route --ticket 7 --repo StatPan/gira --dry-run"},
			},
			Counts:    gira.WorkspaceValidateCounts{Total: 1, Routeable: 1},
			NextSteps: []string{"gira workspace ticket route --ticket 7 --repo StatPan/gira --dry-run"},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "validate", "--config", "testdata/workspace.yaml", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.WorkspaceValidateReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode workspace validate JSON: %v\n%s", err, stdout.String())
	}
	if report.Counts.Routeable != 1 || report.Items[0].Status != "routeable" {
		t.Fatalf("unexpected workspace validate report: %+v", report)
	}
}

func TestWorkspaceSyncRequiresMode(t *testing.T) {
	for _, args := range [][]string{
		{"workspace", "sync"},
		{"workspace", "sync", "--dry-run", "--apply"},
	} {
		var stdout, stderr bytes.Buffer
		code := Run(args, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("Run(%v) exit code = %d, want 2", args, code)
		}
		if !strings.Contains(stderr.String(), "exactly one of --dry-run or --apply is required for workspace sync") {
			t.Fatalf("Run(%v) stderr missing mode guidance:\n%s", args, stderr.String())
		}
	}
}

func TestWorkspaceSyncJSON(t *testing.T) {
	restore := newWorkspaceSyncReport
	t.Cleanup(func() { newWorkspaceSyncReport = restore })
	newWorkspaceSyncReport = func(configPath string, dryRun bool, bootstrapIssues bool) (gira.WorkspaceSyncReport, error) {
		if configPath != "testdata/workspace.yaml" || !dryRun || !bootstrapIssues {
			t.Fatalf("unexpected workspace sync args config=%s dryRun=%t bootstrapIssues=%t", configPath, dryRun, bootstrapIssues)
		}
		return gira.WorkspaceSyncReport{
			Command:   "workspace sync",
			Workspace: gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"},
			DryRun:    true,
			Repos: []gira.WorkspaceSyncRepoReport{
				{Repo: "StatPan/backlog", Role: "inbox", LabelsCreate: 1, MilestonesUpdate: 2},
				{Repo: "StatPan/gira", Role: "execution", LabelsUpdate: 3, BootstrapIssuesCreate: 4},
			},
			Counts:    gira.WorkspaceSyncCounts{Repos: 2, LabelsCreate: 1, LabelsUpdate: 3, MilestonesUpdate: 2, BootstrapIssuesCreate: 4},
			NextSteps: []string{"gira workspace sync --apply --config testdata/workspace.yaml"},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "sync", "--config", "testdata/workspace.yaml", "--dry-run", "--bootstrap-issues", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.WorkspaceSyncReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode workspace sync JSON: %v\n%s", err, stdout.String())
	}
	if !report.DryRun || report.Counts.Repos != 2 || report.Repos[1].BootstrapIssuesCreate != 4 || report.NextSteps[0] == "" {
		t.Fatalf("unexpected workspace sync report: %+v", report)
	}
}

func TestWorkspaceSyncTextIncludesNextStep(t *testing.T) {
	restore := newWorkspaceSyncReport
	t.Cleanup(func() { newWorkspaceSyncReport = restore })
	newWorkspaceSyncReport = func(configPath string, dryRun bool, bootstrapIssues bool) (gira.WorkspaceSyncReport, error) {
		if configPath != "" || dryRun || bootstrapIssues {
			t.Fatalf("unexpected workspace sync args config=%s dryRun=%t bootstrapIssues=%t", configPath, dryRun, bootstrapIssues)
		}
		return gira.WorkspaceSyncReport{
			Command:   "workspace sync",
			Workspace: gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"},
			DryRun:    false,
			Repos:     []gira.WorkspaceSyncRepoReport{{Repo: "StatPan/gira", Role: "execution", LabelsCreate: 2}},
			Counts:    gira.WorkspaceSyncCounts{Repos: 1, LabelsCreate: 2},
			NextSteps: []string{"gira workspace status --config .gira/config.yaml"},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "sync", "--apply"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"workspace sync: apply", "execution StatPan/gira labels create=2", "next step: gira workspace status --config .gira/config.yaml"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("workspace sync text missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestWorkspaceReposSyncJSON(t *testing.T) {
	restore := newWorkspaceRepoSyncReport
	t.Cleanup(func() { newWorkspaceRepoSyncReport = restore })
	newWorkspaceRepoSyncReport = func(input gira.WorkspaceRepoSyncInput) (gira.WorkspaceRepoSyncReport, error) {
		if input.Owner != "StatPan" || input.WorkspaceName != "personal" || input.ConfigRoot != "/tmp/gira" || input.Limit != 50 || !input.IncludeArchived || !input.DryRun || input.Apply {
			t.Fatalf("unexpected workspace repos sync input: %+v", input)
		}
		return gira.WorkspaceRepoSyncReport{
			Command:         "workspace repos sync",
			ConfigRoot:      input.ConfigRoot,
			ConfigPath:      "/tmp/gira/workspaces/personal.yaml",
			Owner:           input.Owner,
			Limit:           input.Limit,
			IncludeArchived: input.IncludeArchived,
			Workspace:       gira.WorkspaceSummary{Name: input.WorkspaceName, Owner: input.Owner},
			InboxRepo:       "StatPan/backlog",
			DiscoveredRepos: []string{"StatPan/backlog", "StatPan/gira", "StatPan/statpan-infra"},
			TargetRepos:     []string{"StatPan/gira", "StatPan/statpan-infra"},
			AddedRepos:      []string{"StatPan/statpan-infra"},
			SkippedRepos:    []string{"StatPan/backlog"},
			File:            gira.SetupGlobalFilePlan{Path: "/tmp/gira/workspaces/personal.yaml", Action: "update"},
			DryRun:          true,
			Status:          "planned",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "repos", "sync", "--owner", "StatPan", "--workspace", "personal", "--config-root", "/tmp/gira", "--limit", "50", "--include-archived", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.WorkspaceRepoSyncReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode workspace repos sync JSON: %v\n%s", err, stdout.String())
	}
	if report.Status != "planned" || strings.Join(report.AddedRepos, ",") != "StatPan/statpan-infra" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.SchemaVersion != gira.WorkspaceReposSyncReportSchemaVersion || report.Approval == nil {
		t.Fatalf("workspace repos sync dry-run JSON missing schema or approval:\n%s", stdout.String())
	}
	if report.Approval.SchemaVersion != gira.ApprovalPlanSchemaVersion || report.Approval.CanonicalCommand != "gira workspace repos sync" || report.Approval.OutputSchema != gira.WorkspaceReposSyncReportSchemaVersion {
		t.Fatalf("unexpected workspace repos sync approval evidence: %+v", report.Approval)
	}
	if report.Approval.ApplyCommand != "gira workspace repos sync --owner StatPan --workspace personal --config-root /tmp/gira --limit 50 --include-archived --apply" || report.Approval.PostApplyVerification != "gira workspace status --config /tmp/gira/workspaces/personal.yaml --json" {
		t.Fatalf("unexpected workspace repos sync approval commands: %+v", report.Approval)
	}
	if report.Approval.Blockers == nil || report.Approval.Warnings == nil {
		t.Fatalf("approval blockers and warnings must be stable arrays: %+v", report.Approval)
	}
}

func TestWorkspaceReposSyncApplyJSONOmitsApprovalEvidence(t *testing.T) {
	restore := newWorkspaceRepoSyncReport
	t.Cleanup(func() { newWorkspaceRepoSyncReport = restore })
	newWorkspaceRepoSyncReport = func(input gira.WorkspaceRepoSyncInput) (gira.WorkspaceRepoSyncReport, error) {
		if input.Owner != "StatPan" || input.WorkspaceName != "personal" || input.ConfigRoot != "/tmp/gira" || input.Limit != 50 || !input.Apply || input.DryRun {
			t.Fatalf("unexpected workspace repos sync input: %+v", input)
		}
		return gira.WorkspaceRepoSyncReport{
			Command:    "workspace repos sync",
			ConfigRoot: input.ConfigRoot,
			ConfigPath: "/tmp/gira/workspaces/personal.yaml",
			Owner:      input.Owner,
			Limit:      input.Limit,
			Workspace:  gira.WorkspaceSummary{Name: input.WorkspaceName, Owner: input.Owner},
			InboxRepo:  "StatPan/backlog",
			File:       gira.SetupGlobalFilePlan{Path: "/tmp/gira/workspaces/personal.yaml", Action: "update"},
			Applied:    true,
			Status:     "applied",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "repos", "sync", "--owner", "StatPan", "--workspace", "personal", "--config-root", "/tmp/gira", "--limit", "50", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.WorkspaceRepoSyncReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode workspace repos sync JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.WorkspaceReposSyncReportSchemaVersion || !report.Applied {
		t.Fatalf("unexpected workspace repos sync apply report: %+v", report)
	}
	if report.Approval != nil {
		t.Fatalf("apply output should not include dry-run approval evidence: %+v", report.Approval)
	}
}

func TestWorkspaceReposSyncOwnerOptional(t *testing.T) {
	restore := newWorkspaceRepoSyncReport
	t.Cleanup(func() { newWorkspaceRepoSyncReport = restore })
	newWorkspaceRepoSyncReport = func(input gira.WorkspaceRepoSyncInput) (gira.WorkspaceRepoSyncReport, error) {
		if input.Owner != "" || input.WorkspaceName != "personal" || !input.DryRun {
			t.Fatalf("unexpected workspace repos sync input: %+v", input)
		}
		return gira.WorkspaceRepoSyncReport{
			Command:   "workspace repos sync",
			Workspace: gira.WorkspaceSummary{Name: input.WorkspaceName, Owner: "StatPan"},
			Owner:     "StatPan",
			DryRun:    true,
			Status:    "planned",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "repos", "sync", "--workspace", "personal", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
}

func TestWorkspaceReposSyncRequiresMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "repos", "sync", "--owner", "StatPan"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "exactly one of --dry-run or --apply is required") {
		t.Fatalf("stderr missing dry-run/apply guidance:\n%s", stderr.String())
	}
}

func TestWorkspaceTicketNewJSON(t *testing.T) {
	restore := newWorkspaceTicketNewReport
	t.Cleanup(func() { newWorkspaceTicketNewReport = restore })
	newWorkspaceTicketNewReport = func(configPath string, title string, body string, targetRepo gira.RepoRef, route bool, dryRun bool) (gira.WorkspaceTicketNewReport, error) {
		if configPath != "testdata/workspace.yaml" || title != "Capture product idea" || body != "Needs routing" || route || dryRun {
			t.Fatalf("unexpected ticket new args config=%s title=%s body=%s repo=%s route=%t dryRun=%t", configPath, title, body, targetRepo.FullName(), route, dryRun)
		}
		created := gira.WorkspaceTicketRef{Repo: "StatPan/backlog", Number: 8, URL: "https://github.com/StatPan/backlog/issues/8"}
		return gira.WorkspaceTicketNewReport{
			Command:   "workspace ticket new",
			Workspace: gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"},
			InboxRepo: "StatPan/backlog",
			Title:     title,
			Created:   &created,
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "ticket", "new", "--config", "testdata/workspace.yaml", "--title", "Capture product idea", "--body", "Needs routing", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "workspace ticket new"`, `"number": 8`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("workspace ticket new JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestWorkspaceTicketNewBodyFileJSON(t *testing.T) {
	restore := newWorkspaceTicketNewReport
	t.Cleanup(func() { newWorkspaceTicketNewReport = restore })
	newWorkspaceTicketNewReport = func(configPath string, title string, body string, targetRepo gira.RepoRef, route bool, dryRun bool) (gira.WorkspaceTicketNewReport, error) {
		if title != "Capture product idea" || body != "## Goal\nFrom file" || targetRepo.FullName() != "StatPan/gira" || !route || !dryRun {
			t.Fatalf("unexpected ticket new body-file args title=%s body=%q repo=%s route=%t dryRun=%t", title, body, targetRepo.FullName(), route, dryRun)
		}
		return gira.WorkspaceTicketNewReport{
			Command:    "workspace ticket new",
			Workspace:  gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"},
			InboxRepo:  "StatPan/backlog",
			Title:      title,
			TargetRepo: targetRepo.FullName(),
			DryRun:     true,
			NextSteps:  []string{`gira workspace ticket new "Capture product idea" --repo StatPan/gira --apply`},
		}, nil
	}
	bodyPath := filepath.Join(t.TempDir(), "issue.md")
	if err := os.WriteFile(bodyPath, []byte("## Goal\nFrom file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "ticket", "new", "Capture", "product", "idea", "--body-file", bodyPath, "--repo", "StatPan/gira", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"target_repo": "StatPan/gira"`) {
		t.Fatalf("workspace ticket new body-file JSON missing route output:\n%s", stdout.String())
	}
}

func TestWorkspaceTicketNewRejectsBodyAndBodyFile(t *testing.T) {
	bodyPath := filepath.Join(t.TempDir(), "issue.md")
	if err := os.WriteFile(bodyPath, []byte("## Goal\nFrom file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "ticket", "new", "Capture product idea", "--body", "inline", "--body-file", bodyPath}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "either --body or --body-file") {
		t.Fatalf("stderr missing body conflict guidance:\n%s", stderr.String())
	}
}

func TestWorkspaceTicketNewAcceptsPositionalTitle(t *testing.T) {
	restore := newWorkspaceTicketNewReport
	t.Cleanup(func() { newWorkspaceTicketNewReport = restore })
	newWorkspaceTicketNewReport = func(configPath string, title string, body string, targetRepo gira.RepoRef, route bool, dryRun bool) (gira.WorkspaceTicketNewReport, error) {
		if title != "Capture product idea" {
			t.Fatalf("title = %q, want positional title", title)
		}
		created := gira.WorkspaceTicketRef{Repo: "StatPan/backlog", Number: 8}
		return gira.WorkspaceTicketNewReport{
			Command:   "workspace ticket new",
			Workspace: gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"},
			InboxRepo: "StatPan/backlog",
			Title:     title,
			Created:   &created,
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "ticket", "new", "Capture", "product", "idea"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "workspace ticket new") {
		t.Fatalf("stdout missing ticket new summary:\n%s", stdout.String())
	}
}

func TestWorkspaceTicketNewRouteRequiresMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "ticket", "new", "Capture product idea", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--repo requires exactly one of --dry-run or --apply") {
		t.Fatalf("stderr missing route mode guidance:\n%s", stderr.String())
	}
}

func TestWorkspaceTicketNewRouteJSON(t *testing.T) {
	restore := newWorkspaceTicketNewReport
	t.Cleanup(func() { newWorkspaceTicketNewReport = restore })
	newWorkspaceTicketNewReport = func(configPath string, title string, body string, targetRepo gira.RepoRef, route bool, dryRun bool) (gira.WorkspaceTicketNewReport, error) {
		if configPath != "testdata/workspace.yaml" || title != "Capture product idea" || body != "" || targetRepo.FullName() != "StatPan/gira" || !route || !dryRun {
			t.Fatalf("unexpected ticket new route args config=%s title=%s body=%s repo=%s route=%t dryRun=%t", configPath, title, body, targetRepo.FullName(), route, dryRun)
		}
		return gira.WorkspaceTicketNewReport{
			Command:    "workspace ticket new",
			Workspace:  gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"},
			InboxRepo:  "StatPan/backlog",
			Title:      title,
			TargetRepo: "StatPan/gira",
			DryRun:     true,
			Actions:    []gira.WorkspaceRouteAction{{Action: "inbox_ticket:create", Repo: "StatPan/backlog", Reason: "capture workspace ticket before routing"}},
			NextSteps:  []string{`gira workspace ticket new "Capture product idea" --repo StatPan/gira --apply`},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "ticket", "new", "Capture", "product", "idea", "--config", "testdata/workspace.yaml", "--repo", "StatPan/gira", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"target_repo": "StatPan/gira"`, `"dry_run": true`, `"action": "inbox_ticket:create"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("workspace ticket new route JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestWorkspaceTicketRouteRequiresModeRepoAndTicket(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "ticket", "route", "--ticket", "8", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--ticket, --repo, and exactly one of --dry-run or --apply are required") {
		t.Fatalf("stderr missing route guidance:\n%s", stderr.String())
	}
}

func TestWorkspaceTicketRouteJSON(t *testing.T) {
	restore := newWorkspaceTicketRouteReport
	t.Cleanup(func() { newWorkspaceTicketRouteReport = restore })
	newWorkspaceTicketRouteReport = func(configPath string, ticketValue string, repo gira.RepoRef, dryRun bool) (gira.WorkspaceTicketRouteReport, error) {
		if configPath != "testdata/workspace.yaml" || ticketValue != "8" || repo.FullName() != "StatPan/gira" || !dryRun {
			t.Fatalf("unexpected route args config=%s ticket=%s repo=%s dryRun=%t", configPath, ticketValue, repo.FullName(), dryRun)
		}
		return gira.WorkspaceTicketRouteReport{
			Command:    "workspace ticket route",
			Workspace:  gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"},
			InboxRepo:  "StatPan/backlog",
			Ticket:     8,
			TargetRepo: "StatPan/gira",
			DryRun:     true,
			Actions:    []gira.WorkspaceRouteAction{{Action: "execution_issue:create", Repo: "StatPan/gira", Reason: "ticket needs repository routing"}},
			NextSteps:  []string{"gira workspace ticket route --ticket 8 --repo StatPan/gira --apply"},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "ticket", "route", "--config", "testdata/workspace.yaml", "--ticket", "8", "--repo", "StatPan/gira", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "workspace ticket route"`, `"dry_run": true`, `"action": "execution_issue:create"`, `"gira workspace ticket route --ticket 8 --repo StatPan/gira --apply"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("workspace ticket route JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestWorkspaceTicketNewRouteTextIncludesNextStep(t *testing.T) {
	restore := newWorkspaceTicketNewReport
	t.Cleanup(func() { newWorkspaceTicketNewReport = restore })
	newWorkspaceTicketNewReport = func(configPath string, title string, body string, targetRepo gira.RepoRef, route bool, dryRun bool) (gira.WorkspaceTicketNewReport, error) {
		if configPath != "testdata/workspace.yaml" || title != "Capture product idea" || body != "Needs route" || targetRepo.FullName() != "StatPan/gira" || !route || dryRun {
			t.Fatalf("unexpected ticket new route args config=%s title=%s body=%s repo=%s route=%t dryRun=%t", configPath, title, body, targetRepo.FullName(), route, dryRun)
		}
		return gira.WorkspaceTicketNewReport{
			Command:    "workspace ticket new",
			Workspace:  gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"},
			InboxRepo:  "StatPan/backlog",
			Title:      title,
			TargetRepo: "StatPan/gira",
			Actions:    []gira.WorkspaceRouteAction{{Action: "execution_issue:create", Repo: "StatPan/gira", Reason: "route directly"}},
			NextSteps:  []string{"gira workspace status --config testdata/workspace.yaml"},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "ticket", "new", "--config", "testdata/workspace.yaml", "--title", "Capture product idea", "--body", "Needs route", "--repo", "StatPan/gira", "--apply"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"workspace ticket new: apply Capture product idea -> StatPan/gira", "execution_issue:create StatPan/gira", "next step: gira workspace status --config testdata/workspace.yaml"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("workspace ticket new route text missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestWorkspaceTicketRouteTextIncludesNextStep(t *testing.T) {
	restore := newWorkspaceTicketRouteReport
	t.Cleanup(func() { newWorkspaceTicketRouteReport = restore })
	newWorkspaceTicketRouteReport = func(configPath string, ticketValue string, repo gira.RepoRef, dryRun bool) (gira.WorkspaceTicketRouteReport, error) {
		if configPath != "testdata/workspace.yaml" || ticketValue != "8" || repo.FullName() != "StatPan/gira" || dryRun {
			t.Fatalf("unexpected route args config=%s ticket=%s repo=%s dryRun=%t", configPath, ticketValue, repo.FullName(), dryRun)
		}
		return gira.WorkspaceTicketRouteReport{
			Command:    "workspace ticket route",
			Workspace:  gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"},
			InboxRepo:  "StatPan/backlog",
			Ticket:     8,
			TargetRepo: "StatPan/gira",
			Actions:    []gira.WorkspaceRouteAction{{Action: "execution_issue:create", Repo: "StatPan/gira", Reason: "ticket needs repository routing", Issue: 44}},
			Created:    &gira.PortfolioLoweredIssue{Repo: "StatPan/gira", Number: 44, URL: "https://github.com/StatPan/gira/issues/44"},
			NextSteps:  []string{"gira ticket start 44 --repo StatPan/gira --dry-run"},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "ticket", "route", "--config", "testdata/workspace.yaml", "--ticket", "8", "--repo", "StatPan/gira", "--apply"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"workspace ticket route: apply inbox#8 -> StatPan/gira", "execution_issue:create StatPan/gira #44", "created: StatPan/gira#44", "next step: gira ticket start 44 --repo StatPan/gira --dry-run"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("workspace ticket route text missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestProjectsSyncRequiresMode(t *testing.T) {
	for _, args := range [][]string{
		{"projects", "sync"},
		{"projects", "sync", "--dry-run", "--apply"},
	} {
		var stdout, stderr bytes.Buffer
		code := Run(args, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("Run(%v) exit code = %d, want 2", args, code)
		}
		if !strings.Contains(stderr.String(), "exactly one of --dry-run or --apply is required for projects sync") {
			t.Fatalf("Run(%v) stderr missing mode guidance:\n%s", args, stderr.String())
		}
	}
}

func TestWorkspaceProjectAdoptRequiresBoundedInputs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "project", "adopt", "--owner", "StatPan", "--title", "Gira"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--owner, exactly one of --title or --number") {
		t.Fatalf("stderr missing bounded input guidance:\n%s", stderr.String())
	}
}

func TestWorkspaceProjectHelpDescribesPortfolioProjectModel(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "project", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"Manage workspace GitHub Projects v2 visibility", "profile or org Project", "Repository issues remain the execution source of truth"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("workspace project help missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestWorkspaceProjectAdoptHelpDescribesExistingProjectAdoption(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "project", "adopt", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"Register an existing GitHub Project", "Does not create Projects", "gira projects sync --config .gira/config.yaml --dry-run"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("workspace project adopt help missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestWorkspaceProjectAdoptJSON(t *testing.T) {
	restore := newWorkspaceProjectAdoptReport
	t.Cleanup(func() { newWorkspaceProjectAdoptReport = restore })
	newWorkspaceProjectAdoptReport = func(input gira.WorkspaceProjectAdoptInput) (gira.WorkspaceProjectAdoptReport, error) {
		if input.ConfigPath != "testdata/workspace.yaml" || input.Owner != "StatPan" || input.Title != "Gira" || !input.DryRun || input.Apply {
			t.Fatalf("unexpected workspace project adopt input: %+v", input)
		}
		return gira.WorkspaceProjectAdoptReport{
			Command:    "workspace project adopt",
			DryRun:     true,
			ConfigPath: "testdata/workspace.yaml",
			Project:    gira.ProjectsSyncProject{Owner: "StatPan", Number: 7, Title: "Gira"},
			Action:     gira.WorkspaceAdoptAction{Action: "workspace.project:set", Status: "planned"},
			NextSteps:  []string{"gira projects sync --config testdata/workspace.yaml --dry-run"},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "project", "adopt", "--config", "testdata/workspace.yaml", "--owner", "StatPan", "--title", "Gira", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "workspace project adopt"`, `"dry_run": true`, `"action": "workspace.project:set"`, `"next_steps"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("workspace project adopt JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestWorkspaceProjectAdoptTextMentionsIssueSourceOfTruth(t *testing.T) {
	restore := newWorkspaceProjectAdoptReport
	t.Cleanup(func() { newWorkspaceProjectAdoptReport = restore })
	newWorkspaceProjectAdoptReport = func(input gira.WorkspaceProjectAdoptInput) (gira.WorkspaceProjectAdoptReport, error) {
		if input.Number != 7 || !input.Apply {
			t.Fatalf("unexpected workspace project adopt input: %+v", input)
		}
		return gira.WorkspaceProjectAdoptReport{
			Command:    "workspace project adopt",
			ConfigPath: ".gira/config.yaml",
			Project:    gira.ProjectsSyncProject{Owner: "StatPan", Number: 7, Title: "Gira"},
			Action:     gira.WorkspaceAdoptAction{Action: "workspace.project:set", Status: "applied"},
			NextSteps:  []string{"gira projects sync --config .gira/config.yaml --dry-run"},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "project", "adopt", "--owner", "StatPan", "--number", "7", "--apply"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"workspace project adopt: apply", "next step: gira projects sync --config .gira/config.yaml --dry-run", "repo issues remain the execution source of truth"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("workspace project adopt output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestProjectsSyncJSON(t *testing.T) {
	restore := newProjectsSyncReport
	t.Cleanup(func() { newProjectsSyncReport = restore })
	newProjectsSyncReport = func(configPath string, dryRun bool, archiveClosed bool) (gira.ProjectsSyncReport, error) {
		if configPath != "testdata/workspace.yaml" || !dryRun || archiveClosed {
			t.Fatalf("unexpected projects sync args config=%s dryRun=%t archiveClosed=%t", configPath, dryRun, archiveClosed)
		}
		return gira.ProjectsSyncReport{
			Command:              "projects sync",
			DryRun:               true,
			Project:              gira.ProjectsSyncProject{Owner: "StatPan", Number: 7, Title: "Gira"},
			Counts:               gira.ProjectsSyncCounts{Issues: 1, ProjectItemsAdd: 1, ViewSetupRequired: true},
			Actions:              []gira.ProjectsSyncAction{{Action: "project_item:add", Repo: "StatPan/gira", Issue: 180, Status: "planned"}},
			ManualActionRequired: true,
			ManualActions:        []string{"In GitHub Project, create Board grouped by Status"},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"projects", "sync", "--config", "testdata/workspace.yaml", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "projects sync"`, `"project_items_add": 1`, `"manual_action_required": true`, `"action": "project_item:add"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("projects sync JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestProjectsSyncArchiveClosedFlag(t *testing.T) {
	restore := newProjectsSyncReport
	t.Cleanup(func() { newProjectsSyncReport = restore })
	newProjectsSyncReport = func(configPath string, dryRun bool, archiveClosed bool) (gira.ProjectsSyncReport, error) {
		if configPath != "testdata/workspace.yaml" || !dryRun || !archiveClosed {
			t.Fatalf("unexpected projects sync args config=%s dryRun=%t archiveClosed=%t", configPath, dryRun, archiveClosed)
		}
		return gira.ProjectsSyncReport{
			Command: "projects sync",
			DryRun:  true,
			Project: gira.ProjectsSyncProject{Owner: "StatPan", Number: 7, Title: "Gira"},
			Counts:  gira.ProjectsSyncCounts{ProjectItemsArchive: 1},
			Actions: []gira.ProjectsSyncAction{{Action: "project_item:archive", Repo: "StatPan/gira", Issue: 199, Status: "planned"}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"projects", "sync", "--config", "testdata/workspace.yaml", "--dry-run", "--archive-closed", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"project_items_archive": 1`, `"action": "project_item:archive"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("projects sync JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestTriageHelpFlagPrintsHelpAndExitsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"triage", "-h"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Backlog triage normalization helpers") {
		t.Fatalf("stdout missing triage help:\n%s", stdout.String())
	}
}

func TestWorkStartRequiresRepoIssueAndMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"work", "start", "--repo", "StatPan/gira", "--issue", "126"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "exactly one of --dry-run/--apply") {
		t.Fatalf("stderr missing mode guidance:\n%s", stderr.String())
	}
}

func TestTicketStartDryRunJSON(t *testing.T) {
	restore := newWorkStartResult
	t.Cleanup(func() { newWorkStartResult = restore })
	newWorkStartResult = func(repo gira.RepoRef, issue int, dryRun bool) (gira.WorkStartResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 126 || !dryRun {
			t.Fatalf("unexpected args repo=%s issue=%d dryRun=%t", repo.FullName(), issue, dryRun)
		}
		return gira.WorkStartResult{Repo: repo.FullName(), Issue: issue, Branch: "issue-126-work-command", DryRun: true, NextStatus: "In progress", NextStep: "gira work start --repo StatPan/gira --issue 126 --apply"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "start", "--repo", "StatPan/gira", "--ticket", "126", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"branch": "issue-126-work-command"`) {
		t.Fatalf("stdout missing branch JSON:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "next step:") {
		t.Fatalf("JSON stdout contains human prose:\n%s", stdout.String())
	}
	var report gira.WorkStartResult
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode ticket start JSON: %v\n%s", err, stdout.String())
	}
	if report.Approval == nil {
		t.Fatalf("ticket start dry-run JSON missing approval evidence:\n%s", stdout.String())
	}
	if report.SchemaVersion != gira.WorkStartResultSchemaVersion {
		t.Fatalf("schema_version = %q, want %q", report.SchemaVersion, gira.WorkStartResultSchemaVersion)
	}
	if report.Approval.SchemaVersion != gira.ApprovalPlanSchemaVersion || report.Approval.CanonicalCommand != "gira ticket start" {
		t.Fatalf("unexpected approval evidence: %+v", report.Approval)
	}
	if report.Approval.ApplyCommand != "gira ticket start 126 --repo StatPan/gira --apply" || report.Approval.PostApplyVerification != "gira ticket status 126 --repo StatPan/gira --json" {
		t.Fatalf("unexpected approval commands: %+v", report.Approval)
	}
}

func TestTicketStartTextShowsSafeBranchReuseDiagnostics(t *testing.T) {
	restore := newWorkStartResult
	t.Cleanup(func() { newWorkStartResult = restore })
	newWorkStartResult = func(repo gira.RepoRef, issue int, dryRun bool) (gira.WorkStartResult, error) {
		return gira.WorkStartResult{
			Repo: repo.FullName(), Issue: issue, Branch: "issue-126-work-command", BaseBranch: "main", DryRun: dryRun, NextStatus: "In progress",
			BranchReuse: &gira.DevStartBranchReuseCheck{Safe: true, BaseRef: "origin/main", MergeBase: "abc123", Ahead: 1, Behind: 0, Diagnostics: []string{"ahead_base=1", "safe_reuse"}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "start", "--repo", "StatPan/gira", "--ticket", "126", "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{
		"branch reuse: safe base=origin/main merge_base=abc123 ahead=1 behind=0 duplicate_patches=0",
		"branch reuse diagnostics: ahead_base=1, safe_reuse",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("ticket start output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestTicketStartTextUnsafeReuseShowsDiagnosticsAndRecovery(t *testing.T) {
	restore := newWorkStartResult
	t.Cleanup(func() { newWorkStartResult = restore })
	newWorkStartResult = func(repo gira.RepoRef, issue int, dryRun bool) (gira.WorkStartResult, error) {
		return gira.WorkStartResult{Repo: repo.FullName(), Issue: issue, Branch: "issue-126-work-command", NextStatus: "In progress"}, fmt.Errorf("unsafe branch reuse for %q: behind_base=1; duplicate_patch_candidates=1; rebase the branch onto origin/main; recreate the work branch from origin/main", "issue-126-work-command")
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "start", "--repo", "StatPan/gira", "--ticket", "126", "--dry-run"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"behind_base=1", "duplicate_patch_candidates=1", "rebase the branch onto origin/main", "recreate the work branch from origin/main"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("ticket start error missing %q:\n%s", want, stderr.String())
		}
	}
}

func TestTicketStartDirtyWorktreeJSONAndTextAreBlocked(t *testing.T) {
	restore := newWorkStartResult
	t.Cleanup(func() { newWorkStartResult = restore })
	newWorkStartResult = func(repo gira.RepoRef, issue int, dryRun bool) (gira.WorkStartResult, error) {
		return gira.WorkStartResult{
			Repo: repo.FullName(), Issue: issue, Branch: "issue-126-work-command", DryRun: dryRun,
			Status: "Ready", NextStatus: "Ready", Started: false, ExecutionState: "blocked_before_mutation",
			NextStep:  "cd /workspace/ticket-126 && gira work start --repo StatPan/gira --issue 126 --apply",
			Preflight: &gira.DevStartWorktreePreflight{CurrentWorktree: "/workspace/current", Dirty: true, ExpectedBranch: "issue-126-work-command", ReusableBranch: true, SuggestedWorktree: "/workspace/ticket-126"},
		}, fmt.Errorf("dirty worktree before branch mutation")
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "start", "126", "--repo", "StatPan/gira", "--apply", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var report gira.WorkStartResult
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode blocked ticket start JSON: %v\n%s", err, stdout.String())
	}
	if report.Started || report.ExecutionState != "blocked_before_mutation" || report.NextStatus != "Ready" || report.Preflight == nil || report.Preflight.SuggestedWorktree != "/workspace/ticket-126" {
		t.Fatalf("blocked JSON must not look successful: %+v", report)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"ticket", "start", "126", "--repo", "StatPan/gira", "--apply"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("text exit code = %d, want 1; stderr=%s", code, stderr.String())
	}
	for _, want := range []string{"status=Ready", "execution_state=blocked_before_mutation", "started=false", "dirty=true", "suggested worktree: /workspace/ticket-126"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("blocked ticket start text missing %q:\n%s", want, stderr.String())
		}
	}
}

func TestTicketStartDryRunJSONPreservesApplyTicketNumber(t *testing.T) {
	restore := newWorkStartResult
	t.Cleanup(func() { newWorkStartResult = restore })
	newWorkStartResult = func(repo gira.RepoRef, issue int, dryRun bool) (gira.WorkStartResult, error) {
		return gira.WorkStartResult{
			Repo:       repo.FullName(),
			Issue:      issue,
			Branch:     "issue-33-rag-docling",
			DryRun:     true,
			Status:     "Ready",
			NextStatus: "In progress",
			NextStep:   "gira work start --repo " + repo.FullName() + " --issue 33 --apply",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "start", "33", "--repo", "StatPan/statpan-infra", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	want := `"next_step": "gira ticket start 33 --apply"`
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("ticket start dry-run JSON missing apply next step:\n%s", stdout.String())
	}
}

func TestTicketStartPositionalAcceptsSplitBaseFlag(t *testing.T) {
	restore := newWorkStartResultWithOptions
	t.Cleanup(func() { newWorkStartResultWithOptions = restore })
	newWorkStartResultWithOptions = func(repo gira.RepoRef, issue int, options gira.WorkStartOptions) (gira.WorkStartResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 126 || !options.DryRun || options.BaseOverride != "dev" {
			t.Fatalf("unexpected args repo=%s issue=%d options=%+v", repo.FullName(), issue, options)
		}
		return gira.WorkStartResult{Repo: repo.FullName(), Issue: issue, Branch: "issue-126-base", BaseBranch: options.BaseOverride, BaseSource: "explicit --base", DryRun: true, NextStatus: "In progress"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "start", "126", "--repo", "StatPan/gira", "--base", "dev", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.WorkStartResult
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode ticket start JSON: %v\n%s", err, stdout.String())
	}
	if report.BaseBranch != "dev" {
		t.Fatalf("base_branch = %q, want dev; stdout=%s", report.BaseBranch, stdout.String())
	}
}

func TestTicketStartPositionalAcceptsEqualsBaseFlag(t *testing.T) {
	restore := newWorkStartResultWithOptions
	t.Cleanup(func() { newWorkStartResultWithOptions = restore })
	newWorkStartResultWithOptions = func(repo gira.RepoRef, issue int, options gira.WorkStartOptions) (gira.WorkStartResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 126 || !options.DryRun || options.BaseOverride != "dev" {
			t.Fatalf("unexpected args repo=%s issue=%d options=%+v", repo.FullName(), issue, options)
		}
		return gira.WorkStartResult{Repo: repo.FullName(), Issue: issue, Branch: "issue-126-base", BaseBranch: options.BaseOverride, BaseSource: "explicit --base", DryRun: true, NextStatus: "In progress"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "start", "126", "--repo", "StatPan/gira", "--base=dev", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.WorkStartResult
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode ticket start JSON: %v\n%s", err, stdout.String())
	}
	if report.BaseBranch != "dev" {
		t.Fatalf("base_branch = %q, want dev; stdout=%s", report.BaseBranch, stdout.String())
	}
}

func TestTicketStartApplyJSONIncludesSchemaVersion(t *testing.T) {
	restore := newWorkStartResult
	t.Cleanup(func() { newWorkStartResult = restore })
	newWorkStartResult = func(repo gira.RepoRef, issue int, dryRun bool) (gira.WorkStartResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 126 || dryRun {
			t.Fatalf("unexpected args repo=%s issue=%d dryRun=%t", repo.FullName(), issue, dryRun)
		}
		return gira.WorkStartResult{Repo: repo.FullName(), Issue: issue, Branch: "issue-126-work-command", DryRun: false, NextStatus: "In progress"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "start", "--repo", "StatPan/gira", "--ticket", "126", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.WorkStartResult
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode ticket start apply JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.WorkStartResultSchemaVersion {
		t.Fatalf("schema_version = %q, want %q", report.SchemaVersion, gira.WorkStartResultSchemaVersion)
	}
	if report.Approval != nil {
		t.Fatalf("apply output should not include dry-run approval evidence: %+v", report.Approval)
	}
}

func TestTicketStartResolvesJiraKey(t *testing.T) {
	restoreStart := newWorkStartResult
	restoreResolve := newJiraMirrorIssueResolver
	t.Cleanup(func() {
		newWorkStartResult = restoreStart
		newJiraMirrorIssueResolver = restoreResolve
	})
	newJiraMirrorIssueResolver = func(repo gira.RepoRef, key string) (gira.JiraMirrorIssue, error) {
		if repo.FullName() != "StatPan/gira" || key != "ABC-123" {
			t.Fatalf("unexpected mirror resolve repo=%s key=%s", repo.FullName(), key)
		}
		return gira.JiraMirrorIssue{Number: 77, Title: "Mirror"}, nil
	}
	newWorkStartResult = func(repo gira.RepoRef, issue int, dryRun bool) (gira.WorkStartResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 77 || !dryRun {
			t.Fatalf("unexpected start args repo=%s issue=%d dryRun=%t", repo.FullName(), issue, dryRun)
		}
		return gira.WorkStartResult{Repo: repo.FullName(), Issue: issue, Branch: "issue-77-mirror", DryRun: true, NextStatus: "In progress"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "start", "ABC-123", "--repo", "StatPan/gira", "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"ticket #77", "jira key: ABC-123", "mirror issue: #77"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("ticket start output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestTicketStartJiraKeyMissingMirror(t *testing.T) {
	restoreResolve := newJiraMirrorIssueResolver
	t.Cleanup(func() { newJiraMirrorIssueResolver = restoreResolve })
	newJiraMirrorIssueResolver = func(repo gira.RepoRef, key string) (gira.JiraMirrorIssue, error) {
		return gira.JiraMirrorIssue{}, fmt.Errorf("multiple GitHub mirror issues found for Jira key %s", key)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "start", "ABC-123", "--repo", "StatPan/gira", "--dry-run"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "multiple GitHub mirror issues") || strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("stderr should contain runtime mirror error without help:\n%s", stderr.String())
	}
}

func TestTicketNewDryRunJSON(t *testing.T) {
	restore := newTicketNewReport
	restoreRepo := repoContextRunner
	t.Cleanup(func() {
		newTicketNewReport = restore
		repoContextRunner = restoreRepo
	})
	repoContextRunner = devCLIRunner{outputs: map[string][]byte{
		"git remote get-url origin": []byte("https://github.com/StatPan/gira.git\n"),
	}}
	newTicketNewReport = func(input gira.TicketNewInput) (gira.TicketNewReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Title != "Add retry" || !input.DryRun || input.Start {
			t.Fatalf("unexpected input: %+v", input)
		}
		if input.Type != "bug" || input.Priority != "p1" || len(input.Acceptance) != 2 || len(input.Labels) != 1 {
			t.Fatalf("unexpected structured input: %+v", input)
		}
		return gira.TicketNewReport{Repo: input.Repo.FullName(), Title: input.Title, DryRun: true, Type: input.Type, Priority: input.Priority, Labels: []string{"type:bug", "status:ready", "priority:p1", "area:backend"}, Body: "## Goal\nRetry\n", NextStep: "gira ticket new --apply"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "new", "Add retry", "--goal", "Retry", "--acceptance", "works;has tests", "--type", "bug", "--priority", "p1", "--label", "area:backend", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"title": "Add retry"`) || !strings.Contains(stdout.String(), `"type:bug"`) {
		t.Fatalf("ticket new JSON missing expected fields:\n%s", stdout.String())
	}
	var report gira.TicketNewReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode ticket new dry-run JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.TicketNewReportSchemaVersion || report.Approval == nil {
		t.Fatalf("ticket new dry-run JSON missing schema or approval:\n%s", stdout.String())
	}
	if report.Approval.SchemaVersion != gira.ApprovalPlanSchemaVersion || report.Approval.OutputSchema != gira.TicketNewReportSchemaVersion {
		t.Fatalf("unexpected approval evidence: %+v", report.Approval)
	}
	for _, want := range []string{"gira ticket new 'Add retry'", "--body '## Goal\nRetry\n'", "--type bug", "--priority p1", "--label area:backend", "--apply"} {
		if !strings.Contains(report.Approval.ApplyCommand, want) {
			t.Fatalf("ticket new approval command missing %q: %+v", want, report.Approval)
		}
	}
	if report.Approval.Blockers == nil || report.Approval.Warnings == nil {
		t.Fatalf("approval blockers and warnings must be stable arrays: %+v", report.Approval)
	}
}

func TestTicketNewAcceptsSpaceSeparatedReleaseImpactAndAliases(t *testing.T) {
	restore := newTicketNewReport
	t.Cleanup(func() { newTicketNewReport = restore })

	tests := []struct {
		name           string
		args           []string
		expectedTitle  string
		expectedImpact string
	}{
		{
			name:           "canonical positional title",
			args:           []string{"ticket", "new", "Create packet", "--repo", "StatPan/gira", "--body", "body", "--release-impact", "internal", "--dry-run"},
			expectedTitle:  "Create packet",
			expectedImpact: "internal",
		},
		{
			name:           "explicit title",
			args:           []string{"ticket", "new", "--title", "Create packet", "--repo", "StatPan/gira", "--body", "body", "--release-impact", "internal", "--dry-run"},
			expectedTitle:  "Create packet",
			expectedImpact: "internal",
		},
		{
			name:           "release impact reason",
			args:           []string{"ticket", "new", "Create packet", "--repo", "StatPan/gira", "--body", "body", "--release-impact", "exempt", "--release-impact-reason", "No release note needed", "--dry-run"},
			expectedTitle:  "Create packet",
			expectedImpact: "exempt",
		},
		{
			name:          "root shortcut",
			args:          []string{"new", "Create packet", "--repo", "StatPan/gira", "--body", "body", "--dry-run"},
			expectedTitle: "Create packet",
		},
		{
			name:          "compact ticket shortcut",
			args:          []string{"t", "n", "Create packet", "--repo", "StatPan/gira", "--body", "body", "--dry-run"},
			expectedTitle: "Create packet",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			newTicketNewReport = func(input gira.TicketNewInput) (gira.TicketNewReport, error) {
				if input.Title != test.expectedTitle || input.ReleaseImpact != test.expectedImpact || !input.DryRun {
					t.Fatalf("unexpected ticket input: %+v", input)
				}
				return gira.TicketNewReport{Repo: input.Repo.FullName(), Title: input.Title, DryRun: true, Labels: []string{"type:task", "status:ready"}, Body: input.Body, NextStep: "gira ticket new --apply"}, nil
			}
			var stdout, stderr bytes.Buffer
			if code := Run(test.args, &stdout, &stderr); code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
			}
		})
	}
}

func TestTicketNewReadsStdinBodyWithSpaceSeparatedReleaseImpact(t *testing.T) {
	restore := newTicketNewReport
	originalStdin := os.Stdin
	t.Cleanup(func() {
		newTicketNewReport = restore
		os.Stdin = originalStdin
	})

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdin pipe: %v", err)
	}
	if _, err := writer.WriteString("## Goal\nFrom stdin\n"); err != nil {
		t.Fatalf("write stdin pipe: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}
	os.Stdin = reader
	t.Cleanup(func() { _ = reader.Close() })

	newTicketNewReport = func(input gira.TicketNewInput) (gira.TicketNewReport, error) {
		if input.Title != "Create packet" || input.Body != "## Goal\nFrom stdin" || input.ReleaseImpact != "internal" || !input.DryRun {
			t.Fatalf("unexpected ticket input: %+v", input)
		}
		return gira.TicketNewReport{Repo: input.Repo.FullName(), Title: input.Title, DryRun: true, Labels: []string{"type:task", "status:ready"}, Body: input.Body, NextStep: "gira ticket new --apply"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "new", "Create packet", "--repo", "StatPan/gira", "--body-file", "-", "--release-impact", "internal", "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
}

func TestTicketNewParentFlagJSON(t *testing.T) {
	restore := newTicketNewReport
	t.Cleanup(func() { newTicketNewReport = restore })
	newTicketNewReport = func(input gira.TicketNewInput) (gira.TicketNewReport, error) {
		if input.Parent != 10 {
			t.Fatalf("parent = %d, want 10; input=%+v", input.Parent, input)
		}
		return gira.TicketNewReport{Repo: input.Repo.FullName(), Title: input.Title, Parent: input.Parent, DryRun: true, Labels: []string{"type:task", "status:ready"}, Body: "## Goal\nChild\n", NextStep: "gira ticket new --apply"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "new", "Child", "--repo", "StatPan/gira", "--parent", "10", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"parent": 10`) {
		t.Fatalf("ticket new parent JSON missing parent:\n%s", stdout.String())
	}
}

func TestTicketParentSetDryRunJSON(t *testing.T) {
	restore := newTicketParentReport
	t.Cleanup(func() { newTicketParentReport = restore })
	newTicketParentReport = func(input gira.TicketParentInput) (gira.TicketParentReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Ticket != 42 || input.Set != 10 || !input.DryRun || input.Apply {
			t.Fatalf("unexpected input: %+v", input)
		}
		return gira.TicketParentReport{
			SchemaVersion: gira.TicketParentReportSchemaVersion,
			Repo:          input.Repo.FullName(),
			Ticket:        input.Ticket,
			TargetParent:  &gira.TicketParentIssue{Number: input.Set, Title: "Parent"},
			DryRun:        true,
			Actions:       []gira.TicketParentAction{{Action: "parent:set", Status: "planned", Detail: "link #42 under #10"}},
			NextStep:      "gira ticket parent 42 --repo StatPan/gira",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "parent", "42", "--repo", "StatPan/gira", "--set", "10", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"schema_version": "ticket-parent-report/v1"`, `"ticket": 42`, `"number": 10`, `"action": "parent:set"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("ticket parent JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestTicketNewBodyFlagJSON(t *testing.T) {
	restore := newTicketNewReport
	t.Cleanup(func() { newTicketNewReport = restore })
	newTicketNewReport = func(input gira.TicketNewInput) (gira.TicketNewReport, error) {
		if input.Body != "## Goal\nExact packet" || input.BodyFile != "" {
			t.Fatalf("unexpected body input: %+v", input)
		}
		return gira.TicketNewReport{Repo: input.Repo.FullName(), Title: input.Title, DryRun: true, Labels: []string{"type:task", "status:ready"}, Body: input.Body, NextStep: "gira ticket new --apply"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "new", "--repo", "StatPan/gira", "--title", "Add packet", "--body", "## Goal\nExact packet", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Exact packet") {
		t.Fatalf("ticket new JSON missing body:\n%s", stdout.String())
	}
}

func TestTicketNewBodyFileJSON(t *testing.T) {
	restore := newTicketNewReport
	t.Cleanup(func() { newTicketNewReport = restore })
	dir := t.TempDir()
	bodyPath := filepath.Join(dir, "issue.md")
	if err := os.WriteFile(bodyPath, []byte("## Goal\nFrom file\n"), 0o644); err != nil {
		t.Fatalf("write body file: %v", err)
	}
	newTicketNewReport = func(input gira.TicketNewInput) (gira.TicketNewReport, error) {
		if input.Body != "## Goal\nFrom file" || input.BodyFile != "" {
			t.Fatalf("unexpected body input: %+v", input)
		}
		return gira.TicketNewReport{Repo: input.Repo.FullName(), Title: input.Title, DryRun: true, Labels: []string{"type:task", "status:ready"}, Body: input.Body, NextStep: "gira ticket new --apply"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "new", "--repo", "StatPan/gira", "--title", "Add packet", "--body-file", bodyPath, "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "From file") {
		t.Fatalf("ticket new JSON missing body:\n%s", stdout.String())
	}
}

func TestReadTicketNewBodyStdin(t *testing.T) {
	body, err := readTicketNewBody("", "-", strings.NewReader("## Goal\nFrom stdin\n"))
	if err != nil {
		t.Fatalf("readTicketNewBody returned error: %v", err)
	}
	if body != "## Goal\nFrom stdin" {
		t.Fatalf("body = %q", body)
	}
	if _, err := readTicketNewBody("body", "-", strings.NewReader("ignored")); err == nil || !strings.Contains(err.Error(), "either --body or --body-file") {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestTicketNewApplyStart(t *testing.T) {
	restore := newTicketNewReport
	t.Cleanup(func() { newTicketNewReport = restore })
	newTicketNewReport = func(input gira.TicketNewInput) (gira.TicketNewReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Title != "Add retry" || input.DryRun || !input.Start {
			t.Fatalf("unexpected input: %+v", input)
		}
		return gira.TicketNewReport{Repo: input.Repo.FullName(), Title: input.Title, Start: true, Created: gira.TicketCreatedIssue{Repo: input.Repo.FullName(), Number: 224}, NextStep: "gira ticket pr --dry-run"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "new", "--repo", "StatPan/gira", "--title", "Add retry", "--apply", "--start"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ticket #224") || !strings.Contains(stdout.String(), "gira ticket pr --dry-run") {
		t.Fatalf("ticket new output missing created ticket:\n%s", stdout.String())
	}
}

func TestTicketNewApplyJSONOmitsApprovalEvidence(t *testing.T) {
	restore := newTicketNewReport
	t.Cleanup(func() { newTicketNewReport = restore })
	newTicketNewReport = func(input gira.TicketNewInput) (gira.TicketNewReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Title != "Add retry" || input.DryRun || !input.Start {
			t.Fatalf("unexpected input: %+v", input)
		}
		return gira.TicketNewReport{Repo: input.Repo.FullName(), Title: input.Title, Start: true, Created: gira.TicketCreatedIssue{Repo: input.Repo.FullName(), Number: 224}, NextStep: "gira ticket pr --dry-run"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "new", "--repo", "StatPan/gira", "--title", "Add retry", "--apply", "--start", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.TicketNewReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode ticket new apply JSON: %v\n%s", err, stdout.String())
	}
	if report.Approval != nil {
		t.Fatalf("apply output should not include dry-run approval evidence: %+v", report.Approval)
	}
}

func TestTicketNewApplyJSONUsesVerifiedLabelsOnLabelWarning(t *testing.T) {
	restore := newTicketNewReport
	t.Cleanup(func() { newTicketNewReport = restore })
	newTicketNewReport = func(input gira.TicketNewInput) (gira.TicketNewReport, error) {
		return gira.TicketNewReport{
			Repo: input.Repo.FullName(), Title: input.Title, Created: gira.TicketCreatedIssue{Repo: input.Repo.FullName(), Number: 224},
			Labels: []string{}, RequestedLabels: []string{"type:task", "status:ready"}, AppliedLabels: []string{},
			LabelOutcome: gira.TicketLabelOutcome{Status: "warning", RequestedLabels: []string{"type:task", "status:ready"}, AppliedLabels: []string{}, MissingLabels: []string{"type:task", "status:ready"}},
			NextStep:     "gira adopt issues --repo StatPan/gira --issues 224 --dry-run",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "new", "--repo", "StatPan/gira", "--title", "Add retry", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.TicketNewReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode ticket new apply JSON: %v\n%s", err, stdout.String())
	}
	if len(report.Labels) != 0 || len(report.AppliedLabels) != 0 || len(report.RequestedLabels) != 2 || report.LabelOutcome.Status != "warning" {
		t.Fatalf("apply JSON must separate verified and requested labels, got %+v", report)
	}
}

func TestTicketNewRequiresModeAndTitle(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "new", "Add retry", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "exactly one of --dry-run/--apply") {
		t.Fatalf("expected mode error, code=%d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"ticket", "new", "--repo", "StatPan/gira", "--dry-run"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "ticket title is required") {
		t.Fatalf("expected title error, code=%d stderr=%s", code, stderr.String())
	}
}

func TestTicketHelpIncludesListFilters(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"gira ticket list", "gira ticket view|show", "Alias: show", "--state open|closed|all", "--label LABEL", "--assignee LOGIN", "--milestone TITLE"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("ticket help missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestMilestoneHelpIncludesLifecycleCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"milestone", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"gira milestone new", "gira milestone list", "gira milestone status", "gira milestone assign", "gira milestone plan", "--dry-run|--apply"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("milestone help missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestMilestonePlanJSON(t *testing.T) {
	restore := newMilestonePlanReport
	t.Cleanup(func() { newMilestonePlanReport = restore })
	newMilestonePlanReport = func(input gira.MilestonePlanInput) (gira.MilestoneReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Milestone != "2.0 Alpha" || !input.DryRun || input.Apply || input.State != "open" || input.Limit != 5 {
			t.Fatalf("unexpected input: %+v", input)
		}
		if strings.Join(input.Labels, "|") != "status:ready|area:backend" {
			t.Fatalf("unexpected labels: %+v", input.Labels)
		}
		return gira.MilestoneReport{
			Command: "milestone plan",
			Repo:    input.Repo.FullName(),
			DryRun:  input.DryRun,
			Milestone: &gira.MilestoneItem{
				Number: 1,
				Title:  input.Milestone,
				State:  "open",
			},
			Filters: gira.MilestoneFilters{Milestone: input.Milestone, State: input.State, Labels: input.Labels, Limit: input.Limit},
			Counts:  gira.MilestoneWorkCounts{Tickets: 1, WouldAssign: 1},
			Actions: []gira.MilestoneAction{{Action: "issue:assign-milestone", Status: "planned", Issue: 42, Milestone: input.Milestone}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"milestone", "plan", "2.0 Alpha", "--repo", "StatPan/gira", "--label", "status:ready", "--label", "area:backend", "--limit", "5", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "milestone plan"`, `"repo": "StatPan/gira"`, `"milestone": "2.0 Alpha"`, `"would_assign": 1`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("milestone plan JSON missing %q:\n%s", want, stdout.String())
		}
	}
	var report gira.MilestoneReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode milestone plan JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.MilestoneReportSchemaVersion || report.Approval == nil {
		t.Fatalf("milestone plan dry-run JSON missing schema or approval:\n%s", stdout.String())
	}
	if report.Approval.SchemaVersion != gira.ApprovalPlanSchemaVersion || report.Approval.CanonicalCommand != "gira milestone plan" || report.Approval.OutputSchema != gira.MilestoneReportSchemaVersion {
		t.Fatalf("unexpected milestone approval evidence: %+v", report.Approval)
	}
	if report.Approval.ApplyCommand != "gira milestone plan '2.0 Alpha' --repo StatPan/gira --state open --label status:ready --label area:backend --limit 5 --apply" || report.Approval.PostApplyVerification != "gira milestone status '2.0 Alpha' --repo StatPan/gira --json" {
		t.Fatalf("unexpected milestone approval commands: %+v", report.Approval)
	}
	if report.Approval.Blockers == nil || report.Approval.Warnings == nil {
		t.Fatalf("approval blockers and warnings must be stable arrays: %+v", report.Approval)
	}
}

func TestMilestoneNewApplyJSONOmitsApprovalEvidence(t *testing.T) {
	restore := newMilestoneNewReport
	t.Cleanup(func() { newMilestoneNewReport = restore })
	newMilestoneNewReport = func(input gira.MilestoneNewInput) (gira.MilestoneReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Title != "2.0 Alpha" || !input.Apply || input.DryRun {
			t.Fatalf("unexpected input: %+v", input)
		}
		return gira.MilestoneReport{
			Command: "milestone new",
			Repo:    input.Repo.FullName(),
			Apply:   true,
			Milestone: &gira.MilestoneItem{
				Number: 1,
				Title:  input.Title,
				State:  "open",
			},
			Actions: []gira.MilestoneAction{{Action: "milestone:create", Status: "applied", Milestone: input.Title}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"milestone", "new", "2.0 Alpha", "--repo", "StatPan/gira", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.MilestoneReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode milestone new JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.MilestoneReportSchemaVersion || !report.Apply {
		t.Fatalf("unexpected milestone apply report: %+v", report)
	}
	if report.Approval != nil {
		t.Fatalf("apply output should not include dry-run approval evidence: %+v", report.Approval)
	}
}

func TestTicketListHumanOutput(t *testing.T) {
	restore := newTicketListReport
	t.Cleanup(func() { newTicketListReport = restore })
	newTicketListReport = func(options gira.TicketListOptions) (gira.TicketListReport, error) {
		if options.Repo.FullName() != "StatPan/gira" || options.State != "closed" || options.Assignee != "alice" || options.Milestone != "MVP" || options.Limit != 20 {
			t.Fatalf("unexpected options: %+v", options)
		}
		if strings.Join(options.Labels, "|") != "status:ready,priority:p1|area:backend" {
			t.Fatalf("unexpected labels: %+v", options.Labels)
		}
		return gira.TicketListReport{
			Command: "ticket list",
			Repo:    options.Repo.FullName(),
			Filters: gira.TicketListFilters{State: "closed", Labels: []string{"status:ready", "priority:p1", "area:backend"}, Assignee: "alice", Milestone: "MVP", Limit: 20},
			Counts:  gira.TicketListCounts{Tickets: 1},
			Tickets: []gira.TicketListItem{{Number: 42, State: "closed", Title: "Ship list UX", Labels: []string{"status:done", "type:story"}, Assignees: []string{"alice"}, Milestone: "MVP"}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "list", "--repo", "StatPan/gira", "--state", "closed", "--label", "status:ready,priority:p1", "--label", "area:backend", "--assignee", "alice", "--milestone", "MVP", "--limit", "20"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"ticket list: StatPan/gira state=closed count=1", "#42 closed", "labels=status:done,type:story", "assignees=alice", "milestone=MVP"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("ticket list output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestTicketListJSONInfersRepo(t *testing.T) {
	restoreList := newTicketListReport
	restoreRepo := repoContextRunner
	t.Cleanup(func() {
		newTicketListReport = restoreList
		repoContextRunner = restoreRepo
	})
	repoContextRunner = devCLIRunner{outputs: map[string][]byte{
		"git remote get-url origin": []byte("https://github.com/StatPan/gira.git\n"),
	}}
	newTicketListReport = func(options gira.TicketListOptions) (gira.TicketListReport, error) {
		if options.Repo.FullName() != "StatPan/gira" || options.State != "open" || options.Limit != 30 {
			t.Fatalf("unexpected options: %+v", options)
		}
		return gira.TicketListReport{
			Command: "ticket list",
			Repo:    options.Repo.FullName(),
			Filters: gira.TicketListFilters{State: "open", Limit: 30},
			Counts:  gira.TicketListCounts{Tickets: 1},
			Tickets: []gira.TicketListItem{{Number: 7, State: "open", Title: "Ready ticket", Status: "ready", Labels: []string{"status:ready"}, URL: "https://github.com/StatPan/gira/issues/7"}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "list", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "ticket list"`, `"repo": "StatPan/gira"`, `"number": 7`, `"status": "ready"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("ticket list JSON missing %q:\n%s", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "next step:") {
		t.Fatalf("JSON stdout contains human prose:\n%s", stdout.String())
	}
}

func TestTicketListUnexpectedArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "list", "extra", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unexpected argument: extra") || !strings.Contains(stderr.String(), "gira ticket list") {
		t.Fatalf("stderr missing list guidance:\n%s", stderr.String())
	}
}

func TestEpicHelpIncludesList(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"epic", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"gira epic list", "--state open|closed|all", "--label LABEL", "--assignee LOGIN"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("epic help missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestEpicListAddsTypeEpicFilter(t *testing.T) {
	restore := newTicketListReport
	t.Cleanup(func() { newTicketListReport = restore })
	newTicketListReport = func(options gira.TicketListOptions) (gira.TicketListReport, error) {
		if options.Repo.FullName() != "StatPan/gira" || options.State != "all" || options.Limit != 10 {
			t.Fatalf("unexpected options: %+v", options)
		}
		if strings.Join(options.Labels, "|") != "type:epic|status:ready" {
			t.Fatalf("unexpected epic labels: %+v", options.Labels)
		}
		return gira.TicketListReport{
			Command: "ticket list",
			Repo:    options.Repo.FullName(),
			Filters: gira.TicketListFilters{State: "all", Labels: []string{"type:epic", "status:ready"}, Limit: 10},
			Counts:  gira.TicketListCounts{Tickets: 1},
			Tickets: []gira.TicketListItem{{Number: 88, State: "open", Title: "Platform epic", Labels: []string{"type:epic", "status:ready"}}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"epic", "list", "--repo", "StatPan/gira", "--state", "all", "--label", "status:ready", "--limit", "10"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"epic list: StatPan/gira state=all count=1", "#88 open", "labels=type:epic,status:ready"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("epic list output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestEpicListJSONUsesEpicCommandAndLabels(t *testing.T) {
	restore := newTicketListReport
	t.Cleanup(func() { newTicketListReport = restore })
	newTicketListReport = func(options gira.TicketListOptions) (gira.TicketListReport, error) {
		if strings.Join(options.Labels, "|") != "type:epic|area:docs" {
			t.Fatalf("unexpected epic labels: %+v", options.Labels)
		}
		return gira.TicketListReport{
			Command: "ticket list",
			Repo:    options.Repo.FullName(),
			Filters: gira.TicketListFilters{State: options.State, Labels: options.Labels, Limit: options.Limit},
			Counts:  gira.TicketListCounts{Tickets: 1},
			Tickets: []gira.TicketListItem{{Number: 88, State: "open", Title: "Platform epic", Labels: []string{"type:epic", "area:docs"}}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"epic", "list", "--repo", "StatPan/gira", "--label", "area:docs", "--limit", "5", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.TicketListReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode epic list JSON: %v\n%s", err, stdout.String())
	}
	if report.Command != "epic list" || strings.Join(report.Filters.Labels, "|") != "type:epic|area:docs" || report.Counts.Tickets != 1 {
		t.Fatalf("unexpected epic list report: %+v", report)
	}
}

func TestEpicStatusHumanOutput(t *testing.T) {
	restore := newEpicStatusReport
	t.Cleanup(func() { newEpicStatusReport = restore })
	newEpicStatusReport = func(input gira.EpicInput) (gira.EpicReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Ticket != 0 || input.Milestone != "v1.4" {
			t.Fatalf("unexpected input: %+v", input)
		}
		return gira.EpicReport{
			Repo:       input.Repo.FullName(),
			Epic:       gira.EpicIssue{Number: 207, Title: "[Epic] Public docs", State: "open", Slug: "epic-public-docs", Milestone: "v1.4"},
			ChildCount: gira.EpicChildCount{Total: 2, Open: 0, Closed: 2},
			NextStep:   "gira epic finish --repo StatPan/gira --ticket 207 --dry-run",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"epic", "status", "--repo", "StatPan/gira", "--milestone", "v1.4"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"epic status: epic #207", "children=2 open=0 closed=2", "next step: gira epic finish --dry-run"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("epic status output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestEpicStatusJSONShortensNextStep(t *testing.T) {
	restore := newEpicStatusReport
	t.Cleanup(func() { newEpicStatusReport = restore })
	newEpicStatusReport = func(input gira.EpicInput) (gira.EpicReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Ticket != 207 {
			t.Fatalf("unexpected input: %+v", input)
		}
		return gira.EpicReport{
			Repo:       input.Repo.FullName(),
			Epic:       gira.EpicIssue{Number: 207, Title: "[Epic] Public docs", State: "open", Slug: "epic-public-docs"},
			ChildCount: gira.EpicChildCount{Total: 1, Closed: 1},
			NextStep:   "gira epic finish --repo StatPan/gira --ticket 207 --dry-run",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"epic", "status", "--ticket", "207", "--repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.EpicReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode epic status JSON: %v\n%s", err, stdout.String())
	}
	if report.NextStep != "gira epic finish --dry-run" {
		t.Fatalf("next step = %q, want shortened epic finish", report.NextStep)
	}
}

func TestEpicFinishJSON(t *testing.T) {
	restore := newEpicFinishReport
	t.Cleanup(func() { newEpicFinishReport = restore })
	newEpicFinishReport = func(input gira.EpicInput) (gira.EpicReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Ticket != 207 || !input.Apply {
			t.Fatalf("unexpected input: %+v", input)
		}
		return gira.EpicReport{
			Repo:  input.Repo.FullName(),
			Epic:  gira.EpicIssue{Number: 207, Title: "[Epic] Public docs", State: "open"},
			Apply: true,
			Actions: []gira.EpicAction{
				{Action: "epic:close", Status: "applied", Detail: "close epic #207"},
			},
			NextStep: "epic is closed",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"epic", "finish", "--repo", "StatPan/gira", "--ticket", "207", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"number": 207`) || !strings.Contains(stdout.String(), `"action": "epic:close"`) {
		t.Fatalf("epic finish JSON missing expected fields:\n%s", stdout.String())
	}
}

func TestEpicFinishHumanOutputShortensNextStep(t *testing.T) {
	restore := newEpicFinishReport
	t.Cleanup(func() { newEpicFinishReport = restore })
	newEpicFinishReport = func(input gira.EpicInput) (gira.EpicReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Ticket != 207 || !input.DryRun {
			t.Fatalf("unexpected input: %+v", input)
		}
		return gira.EpicReport{
			Repo:       input.Repo.FullName(),
			Epic:       gira.EpicIssue{Number: 207, Title: "[Epic] Public docs", State: "open"},
			DryRun:     true,
			ChildCount: gira.EpicChildCount{Total: 2, Closed: 2},
			Actions: []gira.EpicAction{
				{Action: "epic:close", Status: "planned", Detail: "close epic #207"},
			},
			NextStep: "gira epic finish --repo StatPan/gira --ticket 207 --apply",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"epic", "finish", "--ticket", "207", "--repo", "StatPan/gira", "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{
		"epic status: epic #207",
		"actions:",
		"epic:close:planned",
		"next step: gira epic finish --apply",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("epic finish output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestEpicFinishRequiresMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"epic", "finish", "--repo", "StatPan/gira", "--ticket", "207"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "exactly one of --dry-run/--apply") {
		t.Fatalf("stderr missing mode guidance:\n%s", stderr.String())
	}
}

func TestTicketStartIssueAlias(t *testing.T) {
	restore := newWorkStartResult
	t.Cleanup(func() { newWorkStartResult = restore })
	newWorkStartResult = func(repo gira.RepoRef, issue int, dryRun bool) (gira.WorkStartResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 127 || dryRun {
			t.Fatalf("unexpected args repo=%s issue=%d dryRun=%t", repo.FullName(), issue, dryRun)
		}
		return gira.WorkStartResult{Repo: repo.FullName(), Issue: issue, Branch: "issue-127-alias", NextStatus: "In progress"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "start", "--repo", "StatPan/gira", "--issue", "127", "--apply"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{
		"ticket start: ticket #127",
		"next step: gira ticket pr --dry-run",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("ticket start output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestTicketStartPositionalInfersRepo(t *testing.T) {
	restoreWork := newWorkStartResult
	restoreRepo := repoContextRunner
	t.Cleanup(func() {
		newWorkStartResult = restoreWork
		repoContextRunner = restoreRepo
	})
	repoContextRunner = devCLIRunner{outputs: map[string][]byte{
		"git remote get-url origin": []byte("https://github.com/StatPan/gira.git\n"),
	}}
	newWorkStartResult = func(repo gira.RepoRef, issue int, dryRun bool) (gira.WorkStartResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 126 || !dryRun {
			t.Fatalf("unexpected args repo=%s issue=%d dryRun=%t", repo.FullName(), issue, dryRun)
		}
		return gira.WorkStartResult{Repo: repo.FullName(), Issue: issue, Branch: "issue-126-work-command", DryRun: true, NextStatus: "In progress"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "start", "126", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"issue": 126`) {
		t.Fatalf("stdout missing positional issue:\n%s", stdout.String())
	}
}

func TestTicketStartRejectsConflictingTicketAndIssue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "start", "--repo", "StatPan/gira", "--ticket", "126", "--issue", "127", "--dry-run"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "must refer to the same number") {
		t.Fatalf("stderr missing conflict guidance:\n%s", stderr.String())
	}
}

func TestTicketStartWithoutTicketDoesNotAutoSelect(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "start", "--repo", "StatPan/gira", "--dry-run"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--ticket or positional ticket is required") {
		t.Fatalf("stderr missing explicit ticket guidance:\n%s", stderr.String())
	}
}

func TestTicketStartMissingReadyHumanShowsNextStep(t *testing.T) {
	restore := newWorkStartResult
	t.Cleanup(func() { newWorkStartResult = restore })
	newWorkStartResult = func(repo gira.RepoRef, issue int, dryRun bool) (gira.WorkStartResult, error) {
		return gira.WorkStartResult{
			Repo:       repo.FullName(),
			Issue:      issue,
			Title:      "RAG Docling",
			Status:     "null",
			NextStatus: "In progress",
			NextStep:   "gira adopt issues --repo " + repo.FullName() + " --issue 33 --label status:ready --apply",
		}, fmt.Errorf("issue #33 is not ready for start: missing label status:ready; try `gira adopt issues --repo StatPan/statpan-infra --issue 33 --label status:ready --apply` after confirming the issue is executable")
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "start", "33", "--repo", "StatPan/statpan-infra", "--apply"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	for _, want := range []string{
		"issue #33 is not ready for start",
		"missing label status:ready",
		"next step: gira adopt issues --repo StatPan/statpan-infra --issue 33 --label status:ready --apply",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}

func TestTicketStartMissingReadyJSONUsesTicketNextStep(t *testing.T) {
	restore := newWorkStartResult
	t.Cleanup(func() { newWorkStartResult = restore })
	newWorkStartResult = func(repo gira.RepoRef, issue int, dryRun bool) (gira.WorkStartResult, error) {
		return gira.WorkStartResult{
			Repo:       repo.FullName(),
			Issue:      issue,
			Status:     "null",
			NextStatus: "In progress",
			NextStep:   "gira adopt issues --repo " + repo.FullName() + " --issue 33 --label status:ready --apply",
		}, fmt.Errorf("issue #33 is not ready for start: missing label status:ready; try `gira adopt issues --repo StatPan/statpan-infra --issue 33 --label status:ready --apply` after confirming the issue is executable")
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "start", "33", "--repo", "StatPan/statpan-infra", "--apply", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), `"next_step": "gira adopt issues --repo StatPan/statpan-infra --issue 33 --label status:ready --apply"`) {
		t.Fatalf("stdout missing actionable next step:\n%s", stdout.String())
	}
}

func TestWorkStartDryRunJSON(t *testing.T) {
	restore := newWorkStartResult
	t.Cleanup(func() { newWorkStartResult = restore })
	newWorkStartResult = func(repo gira.RepoRef, issue int, dryRun bool) (gira.WorkStartResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 126 || !dryRun {
			t.Fatalf("unexpected args repo=%s issue=%d dryRun=%t", repo.FullName(), issue, dryRun)
		}
		return gira.WorkStartResult{Repo: repo.FullName(), Issue: issue, Branch: "issue-126-work-command", DryRun: true, NextStatus: "In progress"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"work", "start", "--repo", "StatPan/gira", "--issue", "126", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"branch": "issue-126-work-command"`) {
		t.Fatalf("stdout missing branch JSON:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "next step:") {
		t.Fatalf("JSON stdout contains human prose:\n%s", stdout.String())
	}
	var report gira.WorkStartResult
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode work start JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.WorkStartResultSchemaVersion {
		t.Fatalf("schema_version = %q, want %q", report.SchemaVersion, gira.WorkStartResultSchemaVersion)
	}
}

func TestWorkStartApplyJSONIncludesSchemaVersion(t *testing.T) {
	restore := newWorkStartResult
	t.Cleanup(func() { newWorkStartResult = restore })
	newWorkStartResult = func(repo gira.RepoRef, issue int, dryRun bool) (gira.WorkStartResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 126 || dryRun {
			t.Fatalf("unexpected args repo=%s issue=%d dryRun=%t", repo.FullName(), issue, dryRun)
		}
		return gira.WorkStartResult{Repo: repo.FullName(), Issue: issue, Branch: "issue-126-work-command", DryRun: false, NextStatus: "In progress"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"work", "start", "--repo", "StatPan/gira", "--issue", "126", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.WorkStartResult
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode work start apply JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.WorkStartResultSchemaVersion {
		t.Fatalf("schema_version = %q, want %q", report.SchemaVersion, gira.WorkStartResultSchemaVersion)
	}
	if report.Approval != nil {
		t.Fatalf("apply output should not include dry-run approval evidence: %+v", report.Approval)
	}
}

func TestWorkStartDryRunJSONPreservesWorkNextStep(t *testing.T) {
	restore := newWorkStartResult
	t.Cleanup(func() { newWorkStartResult = restore })
	newWorkStartResult = func(repo gira.RepoRef, issue int, dryRun bool) (gira.WorkStartResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 126 || !dryRun {
			t.Fatalf("unexpected args repo=%s issue=%d dryRun=%t", repo.FullName(), issue, dryRun)
		}
		return gira.WorkStartResult{
			Repo:       repo.FullName(),
			Issue:      issue,
			Branch:     "issue-126-work-command",
			DryRun:     true,
			NextStatus: "In progress",
			NextStep:   "gira work start --repo StatPan/gira --issue 126 --apply",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"work", "start", "--repo", "StatPan/gira", "--issue", "126", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"next_step": "gira work start --repo StatPan/gira --issue 126 --apply"`) {
		t.Fatalf("work start JSON should preserve legacy work next step:\n%s", stdout.String())
	}
}

func TestStartAliasWrapsWorkStart(t *testing.T) {
	restore := newWorkStartResult
	t.Cleanup(func() { newWorkStartResult = restore })
	newWorkStartResult = func(repo gira.RepoRef, issue int, dryRun bool) (gira.WorkStartResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 130 || dryRun {
			t.Fatalf("unexpected args repo=%s issue=%d dryRun=%t", repo.FullName(), issue, dryRun)
		}
		return gira.WorkStartResult{Repo: repo.FullName(), Issue: issue, Branch: "issue-130-next-step-aliases", NextStatus: "In progress"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"start", "--repo", "StatPan/gira", "--issue", "130", "--apply"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{
		"ticket start: ticket #130",
		"next step: gira ticket pr --dry-run",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("start alias output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestStartAliasHelpMentionsAlias(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"start", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Alias: gira start") {
		t.Fatalf("help output missing start alias:\n%s", stdout.String())
	}
}

func TestSyncNextStepKeepsPolicyAndBootstrapFlags(t *testing.T) {
	repo := gira.RepoRef{Owner: "StatPan", Name: "gira"}
	got := syncNextStep("gira ops sync", repo, true, true, gira.SyncPolicyAdopt, false)
	want := "next step: gira ops sync --repo StatPan/gira --policy-mode adopt --bootstrap-issues"
	if got != want {
		t.Fatalf("syncNextStep = %q, want %q", got, want)
	}
}

func TestParseRepeatedIssueNumbersSupportsListsAndRanges(t *testing.T) {
	got, err := parseRepeatedIssueNumbers([]string{"1,3-5", "2", "4"})
	if err != nil {
		t.Fatalf("parseRepeatedIssueNumbers error: %v", err)
	}
	want := []int{1, 3, 4, 5, 2}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("parseRepeatedIssueNumbers = %v, want %v", got, want)
	}
}

func TestOpsSyncNextStepUsesOpsCommand(t *testing.T) {
	repo := gira.RepoRef{Owner: "StatPan", Name: "gira"}
	got := syncNextStep("gira ops sync", repo, true, true, gira.SyncPolicyMerge, false)
	want := "next step: gira ops sync --repo StatPan/gira --bootstrap-issues"
	if got != want {
		t.Fatalf("syncNextStep = %q, want %q", got, want)
	}
}

func TestWorkPRApplyDraftJSON(t *testing.T) {
	restore := newWorkPRResult
	t.Cleanup(func() { newWorkPRResult = restore })
	newWorkPRResult = func(repo gira.RepoRef, issue int, dryRun bool, draft bool) (gira.WorkPRResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 126 || dryRun || !draft {
			t.Fatalf("unexpected args repo=%s issue=%d dryRun=%t draft=%t", repo.FullName(), issue, dryRun, draft)
		}
		return gira.WorkPRResult{Repo: repo.FullName(), Issue: issue, Draft: true, PRNumber: 204, NextStatus: "In progress"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"work", "pr", "--repo", "StatPan/gira", "--issue", "126", "--apply", "--draft", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"draft": true`) {
		t.Fatalf("stdout missing draft JSON:\n%s", stdout.String())
	}
	var report gira.WorkPRResult
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode work PR apply JSON: %v\n%s", err, stdout.String())
	}
	if report.Approval != nil {
		t.Fatalf("apply output should not include dry-run approval evidence: %+v", report.Approval)
	}
}

func TestTicketPRApplyDraftJSON(t *testing.T) {
	restore := newWorkPRResult
	t.Cleanup(func() { newWorkPRResult = restore })
	newWorkPRResult = func(repo gira.RepoRef, issue int, dryRun bool, draft bool) (gira.WorkPRResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 126 || dryRun || !draft {
			t.Fatalf("unexpected args repo=%s issue=%d dryRun=%t draft=%t", repo.FullName(), issue, dryRun, draft)
		}
		return gira.WorkPRResult{Repo: repo.FullName(), Issue: issue, Draft: true, PRNumber: 204, NextStatus: "In progress"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "pr", "--repo", "StatPan/gira", "--ticket", "126", "--apply", "--draft", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"draft": true`) {
		t.Fatalf("stdout missing draft JSON:\n%s", stdout.String())
	}
	var report gira.WorkPRResult
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode ticket PR apply JSON: %v\n%s", err, stdout.String())
	}
	if report.Approval != nil {
		t.Fatalf("apply output should not include dry-run approval evidence: %+v", report.Approval)
	}
}

func TestTicketPRDryRunJSONIncludesApprovalEvidence(t *testing.T) {
	restore := newWorkPRResult
	t.Cleanup(func() { newWorkPRResult = restore })
	newWorkPRResult = func(repo gira.RepoRef, issue int, dryRun bool, draft bool) (gira.WorkPRResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 126 || !dryRun || draft {
			t.Fatalf("unexpected args repo=%s issue=%d dryRun=%t draft=%t", repo.FullName(), issue, dryRun, draft)
		}
		return gira.WorkPRResult{Repo: repo.FullName(), Issue: issue, DryRun: true, Branch: "issue-126-work-command", BranchPush: "planned", LocalGit: "git push -u origin <validated-ticket-branch>", Blockers: []string{"missing_linked_pr", "branch_push_required"}, ClosingBody: "Closes #126", NextStatus: "In review"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "pr", "--repo", "StatPan/gira", "--ticket", "126", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.WorkPRResult
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode ticket PR dry-run JSON: %v\n%s", err, stdout.String())
	}
	if report.Approval == nil {
		t.Fatalf("ticket PR dry-run JSON missing approval evidence:\n%s", stdout.String())
	}
	if report.Approval.SchemaVersion != gira.ApprovalPlanSchemaVersion || report.Approval.CanonicalCommand != "gira ticket pr" || report.Approval.OutputSchema != gira.WorkPRResultSchemaVersion {
		t.Fatalf("unexpected approval evidence: %+v", report.Approval)
	}
	if report.Approval.ApplyCommand != "gira ticket pr 126 --repo StatPan/gira --apply" || report.Approval.PostApplyVerification != "gira ticket status 126 --repo StatPan/gira --json" {
		t.Fatalf("unexpected approval commands: %+v", report.Approval)
	}
}

func TestTicketPRInfersTicketFromBranch(t *testing.T) {
	restoreWork := newWorkPRResult
	restoreRepo := repoContextRunner
	restoreDev := devCommandRunner
	t.Cleanup(func() {
		newWorkPRResult = restoreWork
		repoContextRunner = restoreRepo
		devCommandRunner = restoreDev
	})
	repoContextRunner = devCLIRunner{outputs: map[string][]byte{
		"git remote get-url origin": []byte("git@github.com:StatPan/gira.git\n"),
	}}
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"git branch --show-current": []byte("feat/issue-219-finish-flow\n"),
	}}
	newWorkPRResult = func(repo gira.RepoRef, issue int, dryRun bool, draft bool) (gira.WorkPRResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 219 || !dryRun || draft {
			t.Fatalf("unexpected args repo=%s issue=%d dryRun=%t draft=%t", repo.FullName(), issue, dryRun, draft)
		}
		return gira.WorkPRResult{Repo: repo.FullName(), Issue: issue, DryRun: true, PRNumber: 220, NextStatus: "In review"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "pr", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"issue": 219`) {
		t.Fatalf("stdout missing inferred issue:\n%s", stdout.String())
	}
}

func TestWorkPRDryRunJSONIncludesApprovalEvidence(t *testing.T) {
	restore := newWorkPRResult
	t.Cleanup(func() { newWorkPRResult = restore })
	newWorkPRResult = func(repo gira.RepoRef, issue int, dryRun bool, draft bool) (gira.WorkPRResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 126 || !dryRun || !draft {
			t.Fatalf("unexpected args repo=%s issue=%d dryRun=%t draft=%t", repo.FullName(), issue, dryRun, draft)
		}
		return gira.WorkPRResult{Repo: repo.FullName(), Issue: issue, DryRun: true, Draft: true, Branch: "issue-126-work-command", BranchPush: "planned", LocalGit: "git push -u origin <validated-ticket-branch>", Blockers: []string{"missing_linked_pr", "branch_push_required"}, ClosingBody: "Closes #126", NextStatus: "In progress"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"work", "pr", "--repo", "StatPan/gira", "--issue", "126", "--dry-run", "--draft", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.WorkPRResult
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode work PR dry-run JSON: %v\n%s", err, stdout.String())
	}
	if report.Approval == nil {
		t.Fatalf("work PR dry-run JSON missing approval evidence:\n%s", stdout.String())
	}
	if report.Approval.SchemaVersion != gira.ApprovalPlanSchemaVersion || report.Approval.CanonicalCommand != "gira work pr" || report.Approval.OutputSchema != gira.WorkPRResultSchemaVersion {
		t.Fatalf("unexpected approval evidence: %+v", report.Approval)
	}
	if report.Approval.ApplyCommand != "gira work pr --repo StatPan/gira --issue 126 --draft --apply" {
		t.Fatalf("unexpected approval apply command: %+v", report.Approval)
	}
}

func TestResolveTicketContextExplicitSkipsInference(t *testing.T) {
	restoreDev := devCommandRunner
	t.Cleanup(func() { devCommandRunner = restoreDev })
	devCommandRunner = devCLIRunner{errs: map[string]error{
		"git branch --show-current":                                    fmt.Errorf("should not infer"),
		"gh pr view --repo StatPan/gira --json body,headRefName,title": fmt.Errorf("should not infer"),
	}}

	var stderr bytes.Buffer
	ticket, ok := resolveTicketContext(gira.RepoRef{Owner: "StatPan", Name: "gira"}, 225, 0, 0, true, &stderr)
	if !ok || ticket != 225 || stderr.Len() != 0 {
		t.Fatalf("ticket=%d ok=%t stderr=%q", ticket, ok, stderr.String())
	}
}

func TestResolveTicketContextMissingWhenInferenceDisabled(t *testing.T) {
	var stderr bytes.Buffer
	ticket, ok := resolveTicketContext(gira.RepoRef{Owner: "StatPan", Name: "gira"}, 0, 0, 0, false, &stderr)
	if ok || ticket != 0 || !strings.Contains(stderr.String(), "--ticket or positional ticket is required") {
		t.Fatalf("ticket=%d ok=%t stderr=%q", ticket, ok, stderr.String())
	}
}

func TestInferTicketFromCurrentContextRejectsAmbiguousPRBody(t *testing.T) {
	runner := devCLIRunner{outputs: map[string][]byte{
		"git branch --show-current":                                    []byte("main\n"),
		"gh pr view --repo StatPan/gira --json body,headRefName,title": []byte(`{"body":"Closes #10\nFixes #11","headRefName":"feature/context","title":"Context"}`),
	}}

	_, err := inferTicketFromCurrentContext(gira.RepoRef{Owner: "StatPan", Name: "gira"}, runner)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") || !strings.Contains(err.Error(), "#10") || !strings.Contains(err.Error(), "#11") || !strings.Contains(err.Error(), "--ticket N") {
		t.Fatalf("expected ambiguous PR body error, got %v", err)
	}
}

func TestInferTicketFromCurrentContextUsesBranchPRClosingReference(t *testing.T) {
	runner := devCLIRunner{outputs: map[string][]byte{
		"git branch --show-current": []byte("feature/context-runtime\n"),
		"gh pr list --repo StatPan/gira --head feature/context-runtime --state all --json number,body,headRefName,title,url --limit 20": []byte(`[
			{"number":77,"body":"Closes #527","headRefName":"feature/context-runtime","title":"feat: context runtime","url":"https://github.com/StatPan/gira/pull/77"}
		]`),
	}}

	ticket, err := inferTicketFromCurrentContext(gira.RepoRef{Owner: "StatPan", Name: "gira"}, runner)
	if err != nil {
		t.Fatalf("inferTicketFromCurrentContext error: %v", err)
	}
	if ticket != 527 {
		t.Fatalf("ticket = %d, want 527", ticket)
	}
}

func TestInferTicketFromCurrentContextRejectsAmbiguousBranchPRs(t *testing.T) {
	runner := devCLIRunner{outputs: map[string][]byte{
		"git branch --show-current": []byte("feature/context-runtime\n"),
		"gh pr list --repo StatPan/gira --head feature/context-runtime --state all --json number,body,headRefName,title,url --limit 20": []byte(`[
			{"number":77,"body":"Closes #527","headRefName":"feature/context-runtime","title":"feat: context runtime"},
			{"number":78,"body":"Fixes #528","headRefName":"feature/context-runtime","title":"fix: context runtime"}
		]`),
	}}

	_, err := inferTicketFromCurrentContext(gira.RepoRef{Owner: "StatPan", Name: "gira"}, runner)
	if err == nil || !strings.Contains(err.Error(), "Candidates: #527 via PR #77") || !strings.Contains(err.Error(), "#528 via PR #78") || !strings.Contains(err.Error(), "Re-run with: --ticket N") {
		t.Fatalf("expected candidate-rich ambiguity error, got %v", err)
	}
}

func TestTicketPRApplyStopsOnAmbiguousContextBeforeMutation(t *testing.T) {
	restoreWork := newWorkPRResult
	restoreRunner := devCommandRunner
	t.Cleanup(func() {
		newWorkPRResult = restoreWork
		devCommandRunner = restoreRunner
	})
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"git branch --show-current": []byte("feature/context-runtime\n"),
		"gh pr list --repo StatPan/gira --head feature/context-runtime --state all --json number,body,headRefName,title,url --limit 20": []byte(`[
			{"number":77,"body":"Closes #527","headRefName":"feature/context-runtime","title":"feat: context runtime"},
			{"number":78,"body":"Fixes #528","headRefName":"feature/context-runtime","title":"fix: context runtime"}
		]`),
	}}
	newWorkPRResult = func(repo gira.RepoRef, issue int, dryRun bool, draft bool) (gira.WorkPRResult, error) {
		t.Fatalf("newWorkPRResult should not be called when context is ambiguous")
		return gira.WorkPRResult{}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "pr", "--repo", "StatPan/gira", "--apply"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "Candidates: #527 via PR #77") || !strings.Contains(stderr.String(), "Re-run with: --ticket N") {
		t.Fatalf("stderr missing ambiguity guidance:\n%s", stderr.String())
	}
}

func TestInferTicketFromCurrentContextMissingIncludesSafeActions(t *testing.T) {
	runner := devCLIRunner{
		outputs: map[string][]byte{
			"git branch --show-current": []byte("feature/no-ticket\n"),
		},
		errs: map[string]error{
			"gh pr list --repo StatPan/gira --head feature/no-ticket --state all --json number,body,headRefName,title,url --limit 20": fmt.Errorf("no PR"),
			"gh pr view --repo StatPan/gira --json body,headRefName,title":                                                            fmt.Errorf("no current PR"),
		},
	}

	_, err := inferTicketFromCurrentContext(gira.RepoRef{Owner: "StatPan", Name: "gira"}, runner)
	if err == nil || !strings.Contains(err.Error(), "feature/no-ticket") || !strings.Contains(err.Error(), "Try: gira ticket status --repo StatPan/gira --ticket N") || !strings.Contains(err.Error(), "Closes #N") {
		t.Fatalf("expected action-oriented missing context error, got %v", err)
	}
}

func TestTicketLifecycleNextStepsShortenWorkCommands(t *testing.T) {
	start := ticketWorkStartNextStep(gira.WorkStartResult{
		Repo:     "StatPan/gira",
		Issue:    126,
		NextStep: "gira work start --repo StatPan/gira --issue 126 --apply",
	})
	if start != "gira ticket start 126 --apply" {
		t.Fatalf("start next step = %q", start)
	}

	finish := ticketFinishNextStep(gira.WorkFinishResult{
		Repo:     "StatPan/gira",
		Issue:    219,
		NextStep: "resolve review requirements, then gira work pr --repo StatPan/gira --issue 219 --apply",
	})
	if finish != "resolve review requirements, then gira ticket pr --apply" {
		t.Fatalf("finish next step = %q", finish)
	}
}

func TestTicketChecksJSON(t *testing.T) {
	restoreChecks := newTicketChecksReport
	restoreRepo := repoContextRunner
	restoreDev := devCommandRunner
	t.Cleanup(func() {
		newTicketChecksReport = restoreChecks
		repoContextRunner = restoreRepo
		devCommandRunner = restoreDev
	})
	repoContextRunner = devCLIRunner{outputs: map[string][]byte{
		"git remote get-url origin": []byte("https://github.com/StatPan/gira.git\n"),
	}}
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"git branch --show-current": []byte("issue-227-checks\n"),
	}}
	newTicketChecksReport = func(repo gira.RepoRef, issue int, wait time.Duration, pollInterval time.Duration) (gira.TicketChecksReport, error) {
		if repo.FullName() != "StatPan/gira" || issue != 227 || wait != 0 || pollInterval != 0 {
			t.Fatalf("unexpected args repo=%s issue=%d wait=%s poll=%s", repo.FullName(), issue, wait, pollInterval)
		}
		return gira.TicketChecksReport{Repo: repo.FullName(), Issue: issue, PRNumber: 228, Blockers: []string{"checks_pending"}, Checks: []gira.DevPRCheck{{Name: "Build", State: "pending"}}, NextStep: "gira ticket wait --repo StatPan/gira --ticket 227"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "checks", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"checks_pending"`) || strings.Contains(stdout.String(), "--repo StatPan/gira") {
		t.Fatalf("ticket checks JSON did not normalize output:\n%s", stdout.String())
	}
}

func TestTicketViewInfersTicketAndPrintsCard(t *testing.T) {
	restoreView := newTicketViewReport
	restoreRepo := repoContextRunner
	restoreDev := devCommandRunner
	t.Cleanup(func() {
		newTicketViewReport = restoreView
		repoContextRunner = restoreRepo
		devCommandRunner = restoreDev
	})
	repoContextRunner = devCLIRunner{outputs: map[string][]byte{
		"git remote get-url origin": []byte("https://github.com/StatPan/gira.git\n"),
	}}
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"git branch --show-current": []byte("issue-126-work-command\n"),
	}}
	newTicketViewReport = func(repo gira.RepoRef, issue int) (gira.TicketViewReport, error) {
		if repo.FullName() != "StatPan/gira" || issue != 126 {
			t.Fatalf("unexpected args repo=%s issue=%d", repo.FullName(), issue)
		}
		return gira.TicketViewReport{Command: "ticket view", Repo: repo.FullName(), Ticket: issue, Status: gira.WorkStatusResult{Repo: repo.FullName(), Issue: issue, Title: "Work command", State: "open", Status: "In progress", PRNumber: 127, PRState: "OPEN", NextAction: "wait_for_checks", NextStep: "gira ticket wait --repo StatPan/gira --ticket 126"}}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "view"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ticket view: #126 Work command") || !strings.Contains(stdout.String(), "next step: gira ticket wait") {
		t.Fatalf("ticket view output missing context:\n%s", stdout.String())
	}
}

func TestTicketViewResolvesJiraKey(t *testing.T) {
	restoreView := newTicketViewReport
	restoreResolve := newJiraMirrorIssueResolver
	t.Cleanup(func() {
		newTicketViewReport = restoreView
		newJiraMirrorIssueResolver = restoreResolve
	})
	newJiraMirrorIssueResolver = func(repo gira.RepoRef, key string) (gira.JiraMirrorIssue, error) {
		if repo.FullName() != "StatPan/gira" || key != "ABC-123" {
			t.Fatalf("unexpected mirror resolve repo=%s key=%s", repo.FullName(), key)
		}
		return gira.JiraMirrorIssue{Number: 77, Title: "Mirror"}, nil
	}
	newTicketViewReport = func(repo gira.RepoRef, issue int) (gira.TicketViewReport, error) {
		if repo.FullName() != "StatPan/gira" || issue != 77 {
			t.Fatalf("unexpected ticket view args repo=%s issue=%d", repo.FullName(), issue)
		}
		return gira.TicketViewReport{Command: "ticket view", Repo: repo.FullName(), Ticket: issue, Status: gira.WorkStatusResult{Repo: repo.FullName(), Issue: issue, Title: "Mirror", State: "open", Status: "Ready", NextAction: "start_work", NextStep: "gira ticket start --repo StatPan/gira --ticket 77 --dry-run"}}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "view", "ABC-123", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"ticket view: #77 Mirror", "jira key: ABC-123", "mirror issue: #77"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("ticket view output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestTicketShowAliasesTicketView(t *testing.T) {
	restoreView := newTicketViewReport
	t.Cleanup(func() { newTicketViewReport = restoreView })
	newTicketViewReport = func(repo gira.RepoRef, issue int) (gira.TicketViewReport, error) {
		if repo.FullName() != "StatPan/gira" || issue != 77 {
			t.Fatalf("unexpected ticket show args repo=%s issue=%d", repo.FullName(), issue)
		}
		return gira.TicketViewReport{Command: "ticket view", Repo: repo.FullName(), Ticket: issue, Status: gira.WorkStatusResult{Repo: repo.FullName(), Issue: issue, Title: "Alias", State: "open", Status: "Ready", NextAction: "start_work", NextStep: "gira ticket start --repo StatPan/gira --ticket 77 --dry-run"}}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "show", "77", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ticket view: #77 Alias") {
		t.Fatalf("ticket show output should use view formatter:\n%s", stdout.String())
	}
}

func TestTicketViewJiraKeyMissingMirror(t *testing.T) {
	restoreResolve := newJiraMirrorIssueResolver
	t.Cleanup(func() { newJiraMirrorIssueResolver = restoreResolve })
	newJiraMirrorIssueResolver = func(repo gira.RepoRef, key string) (gira.JiraMirrorIssue, error) {
		return gira.JiraMirrorIssue{}, fmt.Errorf("no GitHub mirror issue found for Jira key %s", key)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "view", "ABC-123", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "no GitHub mirror issue found") || strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("stderr should contain runtime mirror error without help:\n%s", stderr.String())
	}
}

func TestTicketStartRepeatedSameNumericPositionalRemainsCompatible(t *testing.T) {
	restoreStart := newWorkStartResult
	t.Cleanup(func() { newWorkStartResult = restoreStart })
	newWorkStartResult = func(repo gira.RepoRef, issue int, dryRun bool) (gira.WorkStartResult, error) {
		if issue != 33 {
			t.Fatalf("unexpected issue: %d", issue)
		}
		return gira.WorkStartResult{Repo: repo.FullName(), Issue: issue, Branch: "issue-33-task", DryRun: true, NextStatus: "In progress"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "start", "33", "33", "--repo", "StatPan/gira", "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ticket #33") {
		t.Fatalf("stdout missing ticket number:\n%s", stdout.String())
	}
}

func TestTicketNoteParsesBodyAndDryRunJSON(t *testing.T) {
	restoreNote := newTicketNoteReport
	t.Cleanup(func() { newTicketNoteReport = restoreNote })
	newTicketNoteReport = func(input gira.TicketNoteInput) (gira.TicketNoteReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Ticket != 126 || input.Body != "Parser path works" || input.Kind != "decision" || input.Target != "both" || !input.DryRun {
			t.Fatalf("unexpected input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.TicketNoteReport{Command: "ticket note", Repo: input.Repo.FullName(), Ticket: input.Ticket, Body: input.Body, Kind: input.Kind, Target: input.Target, DryRun: true, Targets: []gira.TicketNoteSink{{Type: "issue", Number: 126, Status: "planned"}, {Type: "pr", Number: 127, Status: "planned"}}, RenderedBody: "## Decision\n\nParser path works\n", NextStep: "gira ticket note --apply"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "note", "126", "Parser path works", "--repo", "StatPan/gira", "--kind", "decision", "--target", "both", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"command": "ticket note"`) || !strings.Contains(stdout.String(), `"rendered_body": "## Decision`) {
		t.Fatalf("ticket note JSON missing rendered body:\n%s", stdout.String())
	}
	var report gira.TicketNoteReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode ticket note dry-run JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.TicketNoteReportSchemaVersion || report.Approval == nil {
		t.Fatalf("ticket note dry-run JSON missing schema or approval:\n%s", stdout.String())
	}
	if report.Approval.SchemaVersion != gira.ApprovalPlanSchemaVersion || report.Approval.OutputSchema != gira.TicketNoteReportSchemaVersion {
		t.Fatalf("unexpected approval evidence: %+v", report.Approval)
	}
	if report.Approval.ApplyCommand != "gira ticket note 126 --repo StatPan/gira --kind decision --target both --body 'Parser path works' --apply" {
		t.Fatalf("unexpected approval apply command: %+v", report.Approval)
	}
	if len(report.Approval.PlannedActions) != 2 || report.Approval.PlannedActions[0].Action != "issue:comment" || report.Approval.PlannedActions[1].Action != "pr:comment" {
		t.Fatalf("unexpected approval planned actions: %+v", report.Approval.PlannedActions)
	}
}

func TestTicketNoteApplyJSONOmitsApprovalEvidence(t *testing.T) {
	restoreNote := newTicketNoteReport
	t.Cleanup(func() { newTicketNoteReport = restoreNote })
	newTicketNoteReport = func(input gira.TicketNoteInput) (gira.TicketNoteReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Ticket != 126 || input.Body != "Applied note" || !input.Apply {
			t.Fatalf("unexpected input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.TicketNoteReport{Command: "ticket note", Repo: input.Repo.FullName(), Ticket: input.Ticket, Body: input.Body, Kind: input.Kind, Target: input.Target, Targets: []gira.TicketNoteSink{{Type: "issue", Number: 126, Status: "applied"}}, RenderedBody: "## Progress Update\n\nApplied note\n", NextStep: "gira ticket view"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "note", "126", "Applied note", "--repo", "StatPan/gira", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.TicketNoteReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode ticket note apply JSON: %v\n%s", err, stdout.String())
	}
	if report.Approval != nil {
		t.Fatalf("apply output should not include dry-run approval evidence: %+v", report.Approval)
	}
}

func TestTicketSupersedeParsesReplacementBodyAndJSON(t *testing.T) {
	restore := newTicketSupersedeReport
	t.Cleanup(func() { newTicketSupersedeReport = restore })
	newTicketSupersedeReport = func(input gira.TicketSupersedeInput) (gira.TicketSupersedeReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Ticket != 64 || input.ReplacementTitle != "New gate" || input.Body != "## Goal\nBody" || !input.DryRun || !input.CloseDraftPR || input.Milestone != "2.0" || len(input.Labels) != 1 || input.Labels[0] != "area:ai" {
			t.Fatalf("unexpected input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.TicketSupersedeReport{
			Command:   "ticket supersede",
			Repo:      input.Repo.FullName(),
			DryRun:    true,
			Body:      input.Body,
			Labels:    []string{"area:ai"},
			Milestone: "2.0",
			Original:  gira.TicketSupersedeIssue{Number: input.Ticket, Title: "Old gate"},
			Replacement: gira.TicketSupersedeIssue{
				Title: input.ReplacementTitle,
				Body:  input.Body + "\n\n## Supersedes\n- Supersedes #64\n",
			},
			DraftPR:  gira.TicketSupersedeDraftPR{Number: 65, Draft: true, Action: "close"},
			Actions:  []gira.TicketSupersedeAction{{Action: "replacement:create", Status: "planned"}},
			NextStep: "gira ticket supersede --apply",
		}, nil
	}

	tmp := t.TempDir()
	bodyPath := filepath.Join(tmp, "replacement.md")
	if err := os.WriteFile(bodyPath, []byte("## Goal\nBody\n"), 0o644); err != nil {
		t.Fatalf("write body file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "supersede", "64", "--repo", "StatPan/gira", "--replacement-title", "New gate", "--body-file", bodyPath, "--label", "area:ai", "--milestone", "2.0", "--close-draft-pr", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"command": "ticket supersede"`) || !strings.Contains(stdout.String(), `"replacement"`) {
		t.Fatalf("ticket supersede JSON missing fields:\n%s", stdout.String())
	}
	var report gira.TicketSupersedeReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode ticket supersede dry-run JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.TicketSupersedeReportSchemaVersion || report.Approval == nil {
		t.Fatalf("ticket supersede dry-run JSON missing schema or approval:\n%s", stdout.String())
	}
	if report.Approval.SchemaVersion != gira.ApprovalPlanSchemaVersion || report.Approval.OutputSchema != gira.TicketSupersedeReportSchemaVersion {
		t.Fatalf("unexpected approval evidence: %+v", report.Approval)
	}
	if report.Approval.ApplyCommand != "gira ticket supersede 64 --repo StatPan/gira --replacement-title 'New gate' --body '## Goal\nBody' --label area:ai --milestone 2.0 --close-draft-pr --apply" {
		t.Fatalf("unexpected approval apply command: %+v", report.Approval)
	}
	if report.Approval.Blockers == nil || report.Approval.Warnings == nil {
		t.Fatalf("approval blockers and warnings must be stable arrays: %+v", report.Approval)
	}
}

func TestTicketSupersedeReadsReplacementBodyFromStdin(t *testing.T) {
	restore := newTicketSupersedeReport
	t.Cleanup(func() { newTicketSupersedeReport = restore })
	newTicketSupersedeReport = func(input gira.TicketSupersedeInput) (gira.TicketSupersedeReport, error) {
		if input.Body != "## Goal\nBody from stdin" || !input.DryRun {
			t.Fatalf("unexpected input: %+v", input)
		}
		return gira.TicketSupersedeReport{
			Command:     "ticket supersede",
			Repo:        input.Repo.FullName(),
			DryRun:      true,
			Body:        input.Body,
			Original:    gira.TicketSupersedeIssue{Number: input.Ticket, Title: "Old gate"},
			Replacement: gira.TicketSupersedeIssue{Title: input.ReplacementTitle, Body: input.Body},
			Actions:     []gira.TicketSupersedeAction{{Action: "replacement:create", Status: "planned"}},
			NextStep:    "gira ticket supersede --apply",
		}, nil
	}

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if _, err := writer.WriteString("## Goal\nBody from stdin\n"); err != nil {
		t.Fatalf("write stdin pipe: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdin pipe writer: %v", err)
	}
	originalStdin := os.Stdin
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = originalStdin
		_ = reader.Close()
	})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "supersede", "64", "--repo", "StatPan/gira", "--replacement-title", "New gate", "--body-file", "-", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.TicketSupersedeReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode ticket supersede stdin JSON: %v\n%s", err, stdout.String())
	}
	if report.Body != "## Goal\nBody from stdin" || report.Approval == nil {
		t.Fatalf("unexpected stdin supersede report: %+v", report)
	}
}

func TestTicketSupersedeApplyJSONOmitsApprovalEvidence(t *testing.T) {
	restore := newTicketSupersedeReport
	t.Cleanup(func() { newTicketSupersedeReport = restore })
	newTicketSupersedeReport = func(input gira.TicketSupersedeInput) (gira.TicketSupersedeReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Ticket != 64 || input.ReplacementTitle != "New gate" || !input.Apply {
			t.Fatalf("unexpected input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.TicketSupersedeReport{
			Command:     "ticket supersede",
			Repo:        input.Repo.FullName(),
			DryRun:      false,
			Original:    gira.TicketSupersedeIssue{Number: input.Ticket, Title: "Old gate", State: "closed"},
			Replacement: gira.TicketSupersedeIssue{Number: 94, Title: input.ReplacementTitle},
			Actions:     []gira.TicketSupersedeAction{{Action: "replacement:create", Status: "applied"}},
			NextStep:    "gira ticket start 94 --apply",
		}, nil
	}

	tmp := t.TempDir()
	bodyPath := filepath.Join(tmp, "replacement.md")
	if err := os.WriteFile(bodyPath, []byte("## Goal\nBody\n"), 0o644); err != nil {
		t.Fatalf("write body file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "supersede", "64", "--repo", "StatPan/gira", "--replacement-title", "New gate", "--body-file", bodyPath, "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.TicketSupersedeReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode ticket supersede apply JSON: %v\n%s", err, stdout.String())
	}
	if report.Approval != nil {
		t.Fatalf("apply output should not include dry-run approval evidence: %+v", report.Approval)
	}
}

func TestTicketWaitUsesTimeoutAndInterval(t *testing.T) {
	restoreChecks := newTicketChecksReport
	t.Cleanup(func() { newTicketChecksReport = restoreChecks })
	newTicketChecksReport = func(repo gira.RepoRef, issue int, wait time.Duration, pollInterval time.Duration) (gira.TicketChecksReport, error) {
		if repo.FullName() != "StatPan/gira" || issue != 227 || wait != 2*time.Minute || pollInterval != time.Second {
			t.Fatalf("unexpected args repo=%s issue=%d wait=%s poll=%s", repo.FullName(), issue, wait, pollInterval)
		}
		return gira.TicketChecksReport{Repo: repo.FullName(), Issue: issue, PRNumber: 228, Ready: true, Checks: []gira.DevPRCheck{{Name: "Build", State: "passing"}}, NextStep: "gira ticket finish --repo StatPan/gira --ticket 227 --apply"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "wait", "227", "--repo", "StatPan/gira", "--timeout", "2m", "--interval", "1s"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ready=true") || !strings.Contains(stdout.String(), "next step: gira ticket finish --apply") {
		t.Fatalf("ticket wait output missing ready state:\n%s", stdout.String())
	}
}

func TestTicketFinishDryRunJSON(t *testing.T) {
	restore := newWorkFinishResult
	t.Cleanup(func() { newWorkFinishResult = restore })
	newWorkFinishResult = func(repo gira.RepoRef, issue int, dryRun bool, wait time.Duration, options gira.WorkFinishOptions) (gira.WorkFinishResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 219 || !dryRun || wait != 0 || options.SyncLocal {
			t.Fatalf("unexpected args repo=%s issue=%d dryRun=%t wait=%s options=%+v", repo.FullName(), issue, dryRun, wait, options)
		}
		return gira.WorkFinishResult{Repo: repo.FullName(), Issue: issue, DryRun: true, PRNumber: 220, Blockers: []string{"checks_pending"}, FinalStatus: gira.WorkStatusResult{Repo: repo.FullName(), Issue: issue, NextAction: "open_pr", NextStep: "gira work pr --repo StatPan/gira --issue 219 --apply"}, NextStep: "wait for required checks, then gira ticket finish --repo StatPan/gira --ticket 219 --apply"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "finish", "--repo", "StatPan/gira", "--ticket", "219", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"pr_number": 220`) || !strings.Contains(stdout.String(), `"checks_pending"`) {
		t.Fatalf("ticket finish JSON missing expected fields:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "gira work pr") {
		t.Fatalf("ticket finish JSON leaked work next step:\n%s", stdout.String())
	}
	var report gira.WorkFinishResult
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode ticket finish dry-run JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.WorkFinishResultSchemaVersion || report.Approval == nil {
		t.Fatalf("ticket finish dry-run JSON missing schema or approval:\n%s", stdout.String())
	}
	if report.Approval.SchemaVersion != gira.ApprovalPlanSchemaVersion || report.Approval.CanonicalCommand != "gira ticket finish" || report.Approval.OutputSchema != gira.WorkFinishResultSchemaVersion {
		t.Fatalf("unexpected approval evidence: %+v", report.Approval)
	}
	if report.Approval.ApplyCommand != "gira ticket finish 219 --repo StatPan/gira --apply" || report.Approval.PostApplyVerification != "gira ticket status 219 --repo StatPan/gira --json" {
		t.Fatalf("unexpected approval commands: %+v", report.Approval)
	}
	if report.Approval.Blockers == nil || report.Approval.Warnings == nil {
		t.Fatalf("approval blockers and warnings must be stable arrays: %+v", report.Approval)
	}
}

func TestTicketFinishSyncLocalFlagPassesOption(t *testing.T) {
	restore := newWorkFinishResult
	t.Cleanup(func() { newWorkFinishResult = restore })
	newWorkFinishResult = func(repo gira.RepoRef, issue int, dryRun bool, wait time.Duration, options gira.WorkFinishOptions) (gira.WorkFinishResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 219 || !dryRun || !options.SyncLocal {
			t.Fatalf("unexpected args repo=%s issue=%d dryRun=%t wait=%s options=%+v", repo.FullName(), issue, dryRun, wait, options)
		}
		return gira.WorkFinishResult{Repo: repo.FullName(), Issue: issue, DryRun: true, LocalSync: gira.WorkFinishLocalSync{Skipped: true, Reason: "dirty_worktree", TargetBranch: "release/2.0"}, NextStep: "gira ticket finish --repo StatPan/gira --ticket 219 --apply"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "finish", "219", "--repo", "StatPan/gira", "--dry-run", "--sync-local", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"target_branch": "release/2.0"`) {
		t.Fatalf("ticket finish JSON missing local sync target:\n%s", stdout.String())
	}
	var report gira.WorkFinishResult
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode ticket finish sync-local JSON: %v\n%s", err, stdout.String())
	}
	if report.Approval == nil || report.Approval.ApplyCommand != "gira ticket finish 219 --repo StatPan/gira --sync-local --apply" {
		t.Fatalf("ticket finish approval should preserve sync-local: %+v", report.Approval)
	}
}

func TestTicketFinishDryRunApprovalPreservesWait(t *testing.T) {
	restore := newWorkFinishResult
	t.Cleanup(func() { newWorkFinishResult = restore })
	newWorkFinishResult = func(repo gira.RepoRef, issue int, dryRun bool, wait time.Duration, options gira.WorkFinishOptions) (gira.WorkFinishResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 219 || !dryRun || wait != 2*time.Minute {
			t.Fatalf("unexpected args repo=%s issue=%d dryRun=%t wait=%s", repo.FullName(), issue, dryRun, wait)
		}
		return gira.WorkFinishResult{Repo: repo.FullName(), Issue: issue, DryRun: true, Actions: []gira.WorkFinishAction{{Action: "checks:wait", Status: "planned", Detail: "2m0s"}}, NextStep: "gira ticket finish --repo StatPan/gira --ticket 219 --apply"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "finish", "219", "--repo", "StatPan/gira", "--dry-run", "--wait", "2m", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.WorkFinishResult
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode ticket finish wait JSON: %v\n%s", err, stdout.String())
	}
	if report.Approval == nil || report.Approval.ApplyCommand != "gira ticket finish 219 --repo StatPan/gira --wait 2m0s --apply" {
		t.Fatalf("ticket finish approval should preserve wait: %+v", report.Approval)
	}
}

func TestFormatTicketPRCoversPlannedCreatedReusedAndDraft(t *testing.T) {
	cases := []struct {
		name   string
		result gira.WorkPRResult
		wants  []string
	}{
		{
			name:   "planned draft dry-run",
			result: gira.WorkPRResult{Issue: 10, DryRun: true, Draft: true, NextStatus: "In review", BranchPush: "planned"},
			wants:  []string{"ticket #10", "pr=(planned)", "planned", "branch_push=planned", "next step: gira ticket pr --apply --draft"},
		},
		{
			name:   "created",
			result: gira.WorkPRResult{Issue: 11, PRURL: "https://github.com/StatPan/gira/pull/12", Created: true, NextStatus: "In review", BranchPush: "applied"},
			wants:  []string{"pr=https://github.com/StatPan/gira/pull/12", "created", "branch_push=applied", "next step: gira ticket status"},
		},
		{
			name:   "reused draft",
			result: gira.WorkPRResult{Issue: 12, PRURL: "https://github.com/StatPan/gira/pull/13", Draft: true, NextStatus: "In progress", BranchPush: "skipped"},
			wants:  []string{"reused", "next step: mark the PR ready, then gira ticket status"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			output := formatTicketPR(tc.result)
			for _, want := range tc.wants {
				if !strings.Contains(output, want) {
					t.Fatalf("formatTicketPR missing %q:\n%s", want, output)
				}
			}
			if tc.result.BranchPush == "skipped" && strings.Contains(output, "branch_push=") {
				t.Fatalf("formatTicketPR should hide skipped branch push:\n%s", output)
			}
		})
	}
}

func TestFormatTicketFinishCoversBlockersActionsAndFallbacks(t *testing.T) {
	cases := []struct {
		name   string
		result gira.WorkFinishResult
		wants  []string
	}{
		{
			name:   "blocked with actions",
			result: gira.WorkFinishResult{Issue: 21, PRNumber: 22, Readiness: gira.WorkFinishReadinessReport{SchemaVersion: "finish-readiness/v1", Ready: false}, Blockers: []string{"checks", "review"}, Actions: []gira.WorkFinishAction{{Action: "linked_pr:inspect", Status: "done"}, {Action: "pr:merge", Status: "blocked"}}, NextStep: "resolve blockers"},
			wants:  []string{"ticket #21", "pr=22", "merged=false", "readiness=blocked", "blockers=checks,review", "actions=linked_pr:inspect:done,pr:merge:blocked", "next step: resolve blockers"},
		},
		{
			name:   "done without blockers or actions",
			result: gira.WorkFinishResult{Issue: 23, PRNumber: 24, Merged: true, Readiness: gira.WorkFinishReadinessReport{SchemaVersion: "finish-readiness/v1", Ready: true}, Warnings: []string{"IRREVERSIBLE: merge and remote branch deletion"}, NextStep: "ticket is done"},
			wants:  []string{"WARNING: IRREVERSIBLE", "merged=true", "readiness=ready", "blockers=none", "actions=none", "next step: ticket is done"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			output := formatTicketFinish(tc.result)
			for _, want := range tc.wants {
				if !strings.Contains(output, want) {
					t.Fatalf("formatTicketFinish missing %q:\n%s", want, output)
				}
			}
		})
	}
}

func TestTicketStatusInfersTicketFromCurrentPR(t *testing.T) {
	restoreWork := newWorkStatusResult
	restoreRepo := repoContextRunner
	restoreDev := devCommandRunner
	t.Cleanup(func() {
		newWorkStatusResult = restoreWork
		repoContextRunner = restoreRepo
		devCommandRunner = restoreDev
	})
	repoContextRunner = devCLIRunner{outputs: map[string][]byte{
		"git remote get-url origin": []byte("https://github.com/StatPan/gira.git\n"),
	}}
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"git branch --show-current":                                    []byte("main\n"),
		"gh pr view --repo StatPan/gira --json body,headRefName,title": []byte(`{"body":"Closes #221","headRefName":"feature/context","title":"Short context"}`),
	}}
	newWorkStatusResult = func(repo gira.RepoRef, issue int) (gira.WorkStatusResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 221 {
			t.Fatalf("unexpected args repo=%s issue=%d", repo.FullName(), issue)
		}
		return gira.WorkStatusResult{Repo: repo.FullName(), Issue: issue, Status: "In progress", NextAction: "open_pr"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "status", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"issue": 221`) {
		t.Fatalf("stdout missing PR-inferred issue:\n%s", stdout.String())
	}
}

func TestTicketFinishApplyBlockedReturnsJSONAndError(t *testing.T) {
	restore := newWorkFinishResult
	t.Cleanup(func() { newWorkFinishResult = restore })
	newWorkFinishResult = func(repo gira.RepoRef, issue int, dryRun bool, wait time.Duration, options gira.WorkFinishOptions) (gira.WorkFinishResult, error) {
		return gira.WorkFinishResult{Repo: repo.FullName(), Issue: issue, PRNumber: 220, Blockers: []string{"review"}, NextStep: "resolve review requirements, then gira ticket finish --repo StatPan/gira --ticket 219 --apply"}, fmt.Errorf("ticket finish blocked: review")
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "finish", "--repo", "StatPan/gira", "--ticket", "219", "--apply", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), `"review"`) || !strings.Contains(stderr.String(), "ticket finish blocked") {
		t.Fatalf("expected JSON result and error; stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	var report gira.WorkFinishResult
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode ticket finish blocked apply JSON: %v\n%s", err, stdout.String())
	}
	if report.Approval != nil {
		t.Fatalf("apply output should not include dry-run approval evidence: %+v", report.Approval)
	}
}

func TestWorkStatusJSON(t *testing.T) {
	restore := newWorkStatusResult
	t.Cleanup(func() { newWorkStatusResult = restore })
	newWorkStatusResult = func(repo gira.RepoRef, issue int) (gira.WorkStatusResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 126 {
			t.Fatalf("unexpected args repo=%s issue=%d", repo.FullName(), issue)
		}
		return gira.WorkStatusResult{Repo: repo.FullName(), Issue: issue, Status: "In progress", Blockers: []string{"draft"}, NextAction: "mark_pr_ready"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"work", "status", "--repo", "StatPan/gira", "--issue", "126", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"next_action": "mark_pr_ready"`) {
		t.Fatalf("stdout missing next action JSON:\n%s", stdout.String())
	}
}

func TestWorkStatusHumanOutputUsesWorkNextStep(t *testing.T) {
	restore := newWorkStatusResult
	t.Cleanup(func() { newWorkStatusResult = restore })
	newWorkStatusResult = func(repo gira.RepoRef, issue int) (gira.WorkStatusResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 126 {
			t.Fatalf("unexpected args repo=%s issue=%d", repo.FullName(), issue)
		}
		return gira.WorkStatusResult{Repo: repo.FullName(), Issue: issue, Status: "Ready", NextAction: "start_work"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"work", "status", "--repo", "StatPan/gira", "--issue", "126"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{
		"work status: issue #126",
		"next step: gira work start --repo StatPan/gira --issue 126 --apply",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("work status output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestTicketStatusHumanOutputUsesTicketNextStep(t *testing.T) {
	restore := newWorkStatusResult
	t.Cleanup(func() { newWorkStatusResult = restore })
	newWorkStatusResult = func(repo gira.RepoRef, issue int) (gira.WorkStatusResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 126 {
			t.Fatalf("unexpected args repo=%s issue=%d", repo.FullName(), issue)
		}
		return gira.WorkStatusResult{Repo: repo.FullName(), Issue: issue, Status: "In progress", NextAction: "open_pr"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "status", "--repo", "StatPan/gira", "--ticket", "126"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{
		"ticket status: ticket #126",
		"next step: gira ticket pr --apply",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("ticket status output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestTicketStatusHTMLWritesOutput(t *testing.T) {
	restore := newWorkStatusResult
	t.Cleanup(func() { newWorkStatusResult = restore })
	newWorkStatusResult = func(repo gira.RepoRef, issue int) (gira.WorkStatusResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 126 {
			t.Fatalf("unexpected args repo=%s issue=%d", repo.FullName(), issue)
		}
		return gira.WorkStatusResult{
			Command:       "ticket status",
			SchemaVersion: gira.TicketStatusSchemaVersion,
			Repo:          repo.FullName(),
			Issue:         issue,
			Title:         "Ticket <detail>",
			State:         "open",
			Status:        "In review",
			NextAction:    "address_review",
			ReviewStatus:  "blocked",
			PullRequest:   &gira.TicketStatusPullRequest{Available: true, Number: 127, URL: "https://github.com/StatPan/gira/pull/127", State: "OPEN", ReviewDecision: "CHANGES_REQUESTED"},
			PRReadiness:   &gira.PRReadinessReport{SchemaVersion: gira.PRReadinessSchemaVersion, Repo: repo.FullName(), Issue: issue, PullRequest: 127, Readiness: "needs_revision", NextAction: "revise_pr"},
		}, nil
	}

	outputPath := filepath.Join(t.TempDir(), "ticket-126.html")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "status", "126", "--repo", "StatPan/gira", "--html", "--output", outputPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ticket status html:") || !strings.Contains(stdout.String(), "next step: open") {
		t.Fatalf("ticket status HTML output missing next-step summary:\n%s", stdout.String())
	}
	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read ticket status HTML: %v", err)
	}
	for _, want := range []string{"Gira ticket report", "Ticket &lt;detail&gt;", "review: blocked", "CHANGES_REQUESTED", gira.TicketStatusSchemaVersion} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("ticket status HTML missing %q:\n%s", want, string(got))
		}
	}
}

func TestTicketStatusRejectsInvalidOutputFlags(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{
			name: "json and html",
			args: []string{"ticket", "status", "126", "--repo", "StatPan/gira", "--json", "--html", "--output", "ticket.html"},
			want: "choose exactly one output format",
		},
		{
			name: "html needs output",
			args: []string{"ticket", "status", "126", "--repo", "StatPan/gira", "--html"},
			want: "--output is required when using --html",
		},
		{
			name: "output needs html",
			args: []string{"ticket", "status", "126", "--repo", "StatPan/gira", "--output", "ticket.html"},
			want: "--output requires --html",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tc.args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("exit code = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("stderr missing %q:\n%s", tc.want, stderr.String())
			}
		})
	}
}

func TestTicketStatusJSONUsesTicketNextStep(t *testing.T) {
	restore := newWorkStatusResult
	t.Cleanup(func() { newWorkStatusResult = restore })
	newWorkStatusResult = func(repo gira.RepoRef, issue int) (gira.WorkStatusResult, error) {
		return gira.WorkStatusResult{Repo: repo.FullName(), Issue: issue, Status: "Ready", NextAction: "start_work", NextStep: "gira work start --repo StatPan/gira --issue 126 --apply"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "status", "--repo", "StatPan/gira", "--ticket", "126", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"next_step": "gira ticket start 126 --apply"`) {
		t.Fatalf("ticket status JSON should use ticket next step:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "gira work start") {
		t.Fatalf("ticket status JSON leaked work next step:\n%s", stdout.String())
	}
}

func TestTicketStatusJSONGuidesMissingStatusToAdoptReady(t *testing.T) {
	restore := newWorkStatusResult
	t.Cleanup(func() { newWorkStatusResult = restore })
	newWorkStatusResult = func(repo gira.RepoRef, issue int) (gira.WorkStatusResult, error) {
		return gira.WorkStatusResult{Repo: repo.FullName(), Issue: issue, Status: "null", NextAction: "start_work"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "status", "--repo", "StatPan/statpan-infra", "--ticket", "33", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	want := `"next_step": "gira adopt issues --repo StatPan/statpan-infra --issue 33 --label status:ready --apply"`
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("ticket status JSON missing adopt next step:\n%s", stdout.String())
	}
}

func TestTicketStatusJSONGuidesBlockedIssueToResolveBlockers(t *testing.T) {
	restore := newWorkStatusResult
	t.Cleanup(func() { newWorkStatusResult = restore })
	newWorkStatusResult = func(repo gira.RepoRef, issue int) (gira.WorkStatusResult, error) {
		return gira.WorkStatusResult{Repo: repo.FullName(), Issue: issue, Status: "Blocked", NextAction: "resolve_blockers"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "status", "--repo", "StatPan/statpan-infra", "--ticket", "33", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	want := `"next_step": "resolve blockers, then set status:ready before starting work"`
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("ticket status JSON missing blocker next step:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "gira ticket start 33 --apply") {
		t.Fatalf("blocked ticket status should not point to start:\n%s", stdout.String())
	}
}

func TestOpsHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"ops", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Advanced Gira controls") || !strings.Contains(stdout.String(), "sync") || !strings.Contains(stdout.String(), "limit") {
		t.Fatalf("stdout missing ops help:\n%s", stdout.String())
	}
}

func TestOpsLimitCommandJSON(t *testing.T) {
	restore := newOpsLimitReport
	t.Cleanup(func() { newOpsLimitReport = restore })
	newOpsLimitReport = func(repo gira.RepoRef) (gira.APILimitReport, error) {
		if repo.FullName() != "StatPan/gira" {
			t.Fatalf("unexpected repo=%s", repo.FullName())
		}
		return gira.APILimitReport{
			SchemaVersion: gira.APILimitReportSchemaVersion,
			Command:       "ops limit",
			Repo:          repo.FullName(),
			FetchedAt:     "2026-06-27T03:30:00Z",
			Core:          gira.APILimitBucket{Limit: 5000, Remaining: 4800},
			GraphQL:       gira.APILimitBucket{Limit: 5000, Remaining: 4990},
			Search:        gira.APILimitBucket{Limit: 30, Remaining: 30},
			Secondary:     gira.SecondaryLimitInfo{Status: "unobservable", Signals: []string{"http_403"}, Guidance: "Back off."},
			NextStep:      "gira ops limit --repo StatPan/gira --json",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ops", "limit", "--repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.APILimitReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode ops limit JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.APILimitReportSchemaVersion || report.Core.Remaining != 4800 || report.Secondary.Status != "unobservable" {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestOpsLimitCommandText(t *testing.T) {
	restore := newOpsLimitReport
	t.Cleanup(func() { newOpsLimitReport = restore })
	newOpsLimitReport = func(repo gira.RepoRef) (gira.APILimitReport, error) {
		return gira.APILimitReport{
			SchemaVersion: gira.APILimitReportSchemaVersion,
			Command:       "ops limit",
			Repo:          repo.FullName(),
			FetchedAt:     "2026-06-27T03:30:00Z",
			Core:          gira.APILimitBucket{Limit: 5000, Remaining: 4800},
			GraphQL:       gira.APILimitBucket{Limit: 5000, Remaining: 4990},
			Search:        gira.APILimitBucket{Limit: 30, Remaining: 30},
			Secondary:     gira.SecondaryLimitInfo{Status: "unobservable", Signals: []string{"http_403"}, Guidance: "Back off."},
			NextStep:      "gira ops limit --repo StatPan/gira --json",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ops", "limit", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"ops limit: StatPan/gira", "core: remaining=4800/5000", "graphql: remaining=4990/5000", "search: remaining=30/30", "secondary: unobservable"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestOpsLimitCommandWorkflow(t *testing.T) {
	restore := newOpsLimitReport
	t.Cleanup(func() { newOpsLimitReport = restore })
	newOpsLimitReport = func(repo gira.RepoRef) (gira.APILimitReport, error) {
		return gira.APILimitReport{
			SchemaVersion: gira.APILimitReportSchemaVersion,
			Command:       "ops limit",
			Repo:          repo.FullName(),
			FetchedAt:     "2026-06-27T03:30:00Z",
			Core:          gira.APILimitBucket{Limit: 5000, Remaining: 1000},
			GraphQL:       gira.APILimitBucket{Limit: 5000, Remaining: 500},
			Search:        gira.APILimitBucket{Limit: 30, Remaining: 25},
			Secondary:     gira.SecondaryLimitInfo{Status: "unobservable", Signals: []string{"http_403"}, Guidance: "Back off."},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ops", "limit", "--repo", "StatPan/gira", "--workflow", "ticket-lifecycle", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.APILimitReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode ops limit JSON: %v\n%s", err, stdout.String())
	}
	if report.Workflow == nil || report.Workflow.Name != gira.WorkflowCostProfileTicketLifecycle {
		t.Fatalf("workflow estimate missing: %+v", report.Workflow)
	}
	if report.Workflow.SafeRuns != 7 || report.Workflow.LimitingBucket != "rest_core" {
		t.Fatalf("workflow = %+v, want safe_runs=7 limiting_bucket=rest_core", report.Workflow)
	}
	if report.Workflow.Cost.WriteContent != 12 || report.Workflow.WriteContentMeasurable {
		t.Fatalf("unexpected write/content estimate: %+v", report.Workflow)
	}
}

func TestOpsParityDelegatesToExistingCommand(t *testing.T) {
	restore := newJiraParityReport
	t.Cleanup(func() { newJiraParityReport = restore })
	newJiraParityReport = func(repo gira.RepoRef) (gira.JiraParityReport, error) {
		if repo.FullName() != "StatPan/gira" {
			t.Fatalf("unexpected repo=%s", repo.FullName())
		}
		return gira.JiraParityReport{Command: "parity jira", Repo: repo.FullName(), Scores: gira.JiraParityScores{Earned: 100, Total: 100, Pct: 100}, Ready: true}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ops", "parity", "jira", "--repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"command": "parity jira"`) {
		t.Fatalf("stdout missing delegated parity JSON:\n%s", stdout.String())
	}
}

func TestSprintRolloverMissingRepoReturnsTwo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"sprint", "rollover", "--dry-run"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--repo and exactly one of --dry-run/--apply are required") {
		t.Fatalf("stderr missing required-args message:\n%s", stderr.String())
	}
}

func TestSprintCommandsRequireBoundedModeAndArgs(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"sprint", "plan", "--repo", "StatPan/gira", "--iteration", "2026-W18", "--capacity", "2"}, "--repo, --iteration, --capacity and exactly one of --dry-run/--apply are required"},
		{[]string{"sprint", "plan", "--repo", "StatPan/gira", "--iteration", "2026-W18", "--capacity", "2", "--dry-run", "--apply"}, "--repo, --iteration, --capacity and exactly one of --dry-run/--apply are required"},
		{[]string{"sprint", "start", "--repo", "StatPan/gira", "--iteration", "2026-W18"}, "--repo, --iteration and exactly one of --dry-run/--apply are required"},
		{[]string{"sprint", "close", "--repo", "StatPan/gira", "--iteration", "2026-W18", "--spillover-disposition", "carry", "--dry-run"}, "--repo, --iteration, --spillover-disposition, --rollover-reason and exactly one of --dry-run/--apply are required"},
		{[]string{"sprint", "rollover", "--repo", "StatPan/gira", "--dry-run", "--apply"}, "--repo and exactly one of --dry-run/--apply are required"},
	}
	for _, tc := range cases {
		var stdout, stderr bytes.Buffer
		code := Run(tc.args, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("Run(%v) exit code=%d, want 2", tc.args, code)
		}
		if !strings.Contains(stderr.String(), tc.want) {
			t.Fatalf("Run(%v) stderr missing %q:\n%s", tc.args, tc.want, stderr.String())
		}
	}
}

func TestSprintPlanStartCloseJSONLifecycle(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	var stdout, stderr bytes.Buffer
	code := Run([]string{"sprint", "plan", "--repo", "StatPan/gira", "--iteration", "2026-W18", "--capacity", "2", "--issues", "3,1,2", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("plan dry-run exit code=%d stderr=%s", code, stderr.String())
	}
	var dryPlan gira.SprintPlanReport
	if err := json.Unmarshal(stdout.Bytes(), &dryPlan); err != nil {
		t.Fatalf("decode sprint plan dry-run JSON: %v\n%s", err, stdout.String())
	}
	if dryPlan.SchemaVersion != gira.SprintPlanReportSchemaVersion || dryPlan.Approval == nil {
		t.Fatalf("sprint plan dry-run JSON missing schema or approval:\n%s", stdout.String())
	}
	if dryPlan.Approval.ApplyCommand != "gira sprint plan --repo StatPan/gira --iteration 2026-W18 --capacity 2 --issues 1,2,3 --apply" || dryPlan.Approval.OutputSchema != gira.SprintPlanReportSchemaVersion {
		t.Fatalf("unexpected sprint plan approval evidence: %+v", dryPlan.Approval)
	}
	if dryPlan.Approval.Blockers == nil || dryPlan.Approval.Warnings == nil {
		t.Fatalf("approval blockers and warnings must be stable arrays: %+v", dryPlan.Approval)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"sprint", "plan", "--repo", "StatPan/gira", "--iteration", "2026-W18", "--capacity", "2", "--issues", "3,1,2", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("plan exit code=%d stderr=%s", code, stderr.String())
	}
	var applyPlan gira.SprintPlanReport
	if err := json.Unmarshal(stdout.Bytes(), &applyPlan); err != nil {
		t.Fatalf("decode sprint plan apply JSON: %v\n%s", err, stdout.String())
	}
	if applyPlan.SchemaVersion != gira.SprintPlanReportSchemaVersion || applyPlan.Approval != nil {
		t.Fatalf("sprint plan apply JSON should have schema and omit approval: %+v", applyPlan)
	}
	for _, want := range []string{`"mode": "apply"`, `"capacity_target": 2`, `"commit_count": 3`, `"capacity_breach": true`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("sprint plan JSON missing %q:\n%s", want, stdout.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"sprint", "start", "--repo", "StatPan/gira", "--iteration", "2026-W18", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("start dry-run exit code=%d stderr=%s", code, stderr.String())
	}
	var dryStart gira.SprintStartReport
	if err := json.Unmarshal(stdout.Bytes(), &dryStart); err != nil {
		t.Fatalf("decode sprint start dry-run JSON: %v\n%s", err, stdout.String())
	}
	if dryStart.SchemaVersion != gira.SprintStartReportSchemaVersion || dryStart.Approval == nil || dryStart.Approval.ApplyCommand != "gira sprint start --repo StatPan/gira --iteration 2026-W18 --apply" {
		t.Fatalf("unexpected sprint start approval evidence: %+v", dryStart)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"sprint", "start", "--repo", "StatPan/gira", "--iteration", "2026-W18", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("start exit code=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{`"mode": "apply"`, `"commitment_frozen": true`, `"started_at"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("sprint start JSON missing %q:\n%s", want, stdout.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"sprint", "close", "--repo", "StatPan/gira", "--iteration", "2026-W18", "--completed", "1,3", "--spillover-disposition", "carry", "--rollover-reason", "dependency blocked", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("close dry-run exit code=%d stderr=%s", code, stderr.String())
	}
	var dryClose gira.SprintCloseReport
	if err := json.Unmarshal(stdout.Bytes(), &dryClose); err != nil {
		t.Fatalf("decode sprint close dry-run JSON: %v\n%s", err, stdout.String())
	}
	if dryClose.SchemaVersion != gira.SprintCloseReportSchemaVersion || dryClose.Approval == nil {
		t.Fatalf("sprint close dry-run JSON missing schema or approval: %+v", dryClose)
	}
	if dryClose.Approval.ApplyCommand != "gira sprint close --repo StatPan/gira --iteration 2026-W18 --completed 1,3 --spillover-disposition carry --rollover-reason 'dependency blocked' --apply" {
		t.Fatalf("unexpected sprint close approval command: %+v", dryClose.Approval)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"sprint", "close", "--repo", "StatPan/gira", "--iteration", "2026-W18", "--completed", "1,3", "--spillover-disposition", "carry", "--rollover-reason", "dependency blocked", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("close exit code=%d stderr=%s", code, stderr.String())
	}
	var closeReport gira.SprintCloseReport
	if err := json.Unmarshal(stdout.Bytes(), &closeReport); err != nil {
		t.Fatalf("decode sprint close JSON: %v\n%s", err, stdout.String())
	}
	if closeReport.Mode != "apply" || fmt.Sprint(closeReport.Summary.CompletedItems) != "[1 3]" || fmt.Sprint(closeReport.Summary.SpilloverItems) != "[2]" || closeReport.Summary.SpilloverDisposition != "carry" || closeReport.Summary.RolloverReason != "dependency blocked" {
		t.Fatalf("unexpected sprint close report: %+v", closeReport)
	}
	if closeReport.SchemaVersion != gira.SprintCloseReportSchemaVersion || closeReport.Approval != nil {
		t.Fatalf("sprint close apply JSON should have schema and omit approval: %+v", closeReport)
	}
}

func TestSprintRolloverJSONUsesInjectedReport(t *testing.T) {
	restore := newSprintRolloverReport
	t.Cleanup(func() { newSprintRolloverReport = restore })
	newSprintRolloverReport = func(repo gira.RepoRef, toMilestone string, apply bool) (gira.SprintRolloverReport, error) {
		if repo.FullName() != "StatPan/gira" || toMilestone != "W18" || !apply {
			t.Fatalf("unexpected rollover args repo=%s to=%s apply=%t", repo.FullName(), toMilestone, apply)
		}
		return gira.SprintRolloverReport{
			Repo:             repo.FullName(),
			Mode:             "apply",
			TargetMilestone:  &gira.SprintRolloverTarget{Number: 2, Title: "W18"},
			TargetResolution: "explicit --to",
			Summary:          gira.SprintRolloverSummary{Candidates: 1, Applied: 1},
			Items:            []gira.SprintRolloverItem{{IssueNumber: 10, IssueTitle: "Carry me", FromMilestone: "W17", Action: "applied", TargetMilestone: "W18"}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"sprint", "rollover", "--repo", "StatPan/gira", "--to", "W18", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("rollover exit code=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{`"mode": "apply"`, `"target_resolution": "explicit --to"`, `"applied": 1`, `"issue_number": 10`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("sprint rollover JSON missing %q:\n%s", want, stdout.String())
		}
	}
	var report gira.SprintRolloverReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode sprint rollover JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.SprintRolloverReportSchemaVersion || report.Approval != nil {
		t.Fatalf("sprint rollover apply JSON should have schema and omit approval: %+v", report)
	}
}

func TestSprintRolloverDryRunJSONUsesInjectedApproval(t *testing.T) {
	restore := newSprintRolloverReport
	t.Cleanup(func() { newSprintRolloverReport = restore })
	newSprintRolloverReport = func(repo gira.RepoRef, toMilestone string, apply bool) (gira.SprintRolloverReport, error) {
		if repo.FullName() != "StatPan/gira" || toMilestone != "W18" || apply {
			t.Fatalf("unexpected rollover args repo=%s to=%s apply=%t", repo.FullName(), toMilestone, apply)
		}
		return gira.SprintRolloverReport{
			Repo:             repo.FullName(),
			Mode:             "dry-run",
			TargetMilestone:  &gira.SprintRolloverTarget{Number: 2, Title: "W18"},
			TargetResolution: "explicit --to",
			Summary:          gira.SprintRolloverSummary{Candidates: 1, Applied: 1},
			Items:            []gira.SprintRolloverItem{{IssueNumber: 10, IssueTitle: "Carry me", FromMilestone: "W17", CandidateReason: "source milestone due date passed", Action: "would-apply", TargetMilestone: "W18"}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"sprint", "rollover", "--repo", "StatPan/gira", "--to", "W18", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("rollover exit code=%d stderr=%s", code, stderr.String())
	}
	var report gira.SprintRolloverReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode sprint rollover JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.SprintRolloverReportSchemaVersion || report.Approval == nil {
		t.Fatalf("sprint rollover dry-run JSON missing schema or approval:\n%s", stdout.String())
	}
	if report.Approval.ApplyCommand != "gira sprint rollover --repo StatPan/gira --to W18 --apply" || report.Approval.OutputSchema != gira.SprintRolloverReportSchemaVersion {
		t.Fatalf("unexpected sprint rollover approval evidence: %+v", report.Approval)
	}
	if report.Approval.Blockers == nil || report.Approval.Warnings == nil {
		t.Fatalf("approval blockers and warnings must be stable arrays: %+v", report.Approval)
	}
}

func TestTriageMissingRepoReturnsTwo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"triage", "--dry-run"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--repo and exactly one of --dry-run/--apply are required") {
		t.Fatalf("stderr missing required-args message:\n%s", stderr.String())
	}
}

func TestTriageDryRunApplyExclusivityReturnsTwo(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "neither", args: []string{"triage", "--repo", "StatPan/gira"}},
		{name: "both", args: []string{"triage", "--repo", "StatPan/gira", "--dry-run", "--apply"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tc.args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("exit code = %d, want 2", code)
			}
			if !strings.Contains(stderr.String(), "exactly one of --dry-run/--apply") {
				t.Fatalf("stderr missing exclusivity guidance:\n%s", stderr.String())
			}
		})
	}
}

func TestTriageDryRunValidInvocationReturnsZero(t *testing.T) {
	restore := newTriageReport
	t.Cleanup(func() { newTriageReport = restore })
	newTriageReport = func(repo gira.RepoRef, apply bool) (gira.TriageNormalizeReport, error) {
		if repo.FullName() != "StatPan/gira" {
			t.Fatalf("repo = %s, want StatPan/gira", repo.FullName())
		}
		if apply {
			t.Fatalf("apply = true, want false for dry-run")
		}
		return gira.TriageNormalizeReport{Repo: repo.FullName(), Mode: "dry-run"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"triage", "--repo", "StatPan/gira", "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"repo\": \"StatPan/gira\"") {
		t.Fatalf("stdout missing triage JSON payload:\n%s", stdout.String())
	}
}

func TestContractCRUDMatrix(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"contract", "crud"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"CRUD capability matrix (MVP contract)",
		"surface               create                                           read                                                                                     update                                                            delete",
		"labels                gira ops sync --repo OWNER/REPO                  gira ops sync --repo OWNER/REPO --dry-run                                                gira ops sync --repo OWNER/REPO                                   unsupported (intentional in MVP)",
		"milestones            gira ops sync --repo OWNER/REPO                  gira ops sync --repo OWNER/REPO --dry-run                                                gira ops sync --repo OWNER/REPO                                   unsupported (intentional in MVP)",
		"issues                gira ops sync --repo OWNER/REPO --bootstrap-issues gira status --repo OWNER/REPO                                                          gira triage apply --apply / gira worker claim|handoff|release     unsupported direct delete in MVP",
		"pr_loop               gira dev pr open --repo OWNER/REPO --issue N     gira dev pr status --repo OWNER/REPO --issue N / gira review queue                        gira merge queue --apply (opt-in destructive)                     unsupported direct delete; close via GitHub UI/API",
		"project_fields_views  unsupported (MVP non-goal)                       gira project capability / gira project sync --dry-run / gira project transitions --dry-run unsupported in MVP (dry-run inspection only)                      unsupported (MVP non-goal)",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("contract output missing %q:\n%s", want, output)
		}
	}
}

func TestContractUnsupportedOperationGuidance(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"contract", "delete"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "unsupported contract operation") {
		t.Fatalf("stderr missing unsupported guidance:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "supported operation: gira contract crud") {
		t.Fatalf("stderr missing supported operation guidance:\n%s", stderr.String())
	}
}

func TestBootstrapRequiresRepo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"bootstrap", "--dry-run"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "--repo is required") {
		t.Fatalf("stderr missing repo requirement:\n%s", stderr.String())
	}
}

func TestBootstrapNonDryRunRequiresPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"bootstrap", "--repo", "StatPan/example"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "--path is required") {
		t.Fatalf("stderr missing path requirement:\n%s", stderr.String())
	}
}

func TestBootstrapNonDryRunRequiresGitRepo(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"bootstrap",
		"--repo",
		"StatPan/example",
		"--path",
		dir,
		"--created-at",
		"2026-04-26",
	}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "not a git repository") {
		t.Fatalf("stderr missing git repo requirement:\n%s", stderr.String())
	}
}

func TestBootstrapNonDryRunCanInstallWithoutBranch(t *testing.T) {
	repo := t.TempDir()
	runCLIGit(t, repo, "init", "-q", "-b", "main")

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"bootstrap",
		"--repo",
		"StatPan/example",
		"--path",
		repo,
		"--no-branch",
		"--created-at",
		"2026-04-26",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "branch:") {
		t.Fatalf("stdout unexpectedly included branch line:\n%s", stdout.String())
	}
	if _, err := os.Stat(repo + "/AGENTS.md"); err != nil {
		t.Fatalf("AGENTS.md was not created: %v", err)
	}
}

func TestBootstrapDryRunIsDeterministic(t *testing.T) {
	args := []string{
		"bootstrap",
		"--repo",
		"StatPan/example",
		"--template",
		"default",
		"--dry-run",
		"--created-at",
		"2026-04-26",
	}

	var firstOut, firstErr, secondOut, secondErr bytes.Buffer
	firstCode := Run(args, &firstOut, &firstErr)
	secondCode := Run(args, &secondOut, &secondErr)

	if firstCode != 0 || secondCode != 0 {
		t.Fatalf("exit codes = %d/%d, want 0/0; stderr: %s%s", firstCode, secondCode, firstErr.String(), secondErr.String())
	}
	if firstOut.String() != secondOut.String() {
		t.Fatal("dry-run output differed between identical runs")
	}
	output := firstOut.String()
	for _, want := range []string{"--- AGENTS.md", "StatPan/example", "example", "2026-04-26", "--- .github/PULL_REQUEST_TEMPLATE.md", "--- .github/ISSUE_TEMPLATE/config.yml", "--- .github/ISSUE_TEMPLATE/portfolio.yml"} {
		if !strings.Contains(output, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, output)
		}
	}
}

func TestStatusJSONUsesInjectedClient(t *testing.T) {
	restoreClient, restoreNow := newStatusClient, statusNow
	t.Cleanup(func() {
		newStatusClient = restoreClient
		statusNow = restoreNow
	})
	newStatusClient = func(repo gira.RepoRef) gira.StatusClient {
		return cliFakeStatusClient{
			repo: repo,
			responses: map[string]string{
				"api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100": `[[{"number":1,"title":"MVP","state":"open","description":"","due_on":null,"open_issues":1,"closed_issues":1}]]`,
				"api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100":     `[[{"number":1,"title":"Issue 1","state":"open","labels":[],"milestone":null,"updated_at":"2026-04-25T12:00:00Z","html_url":"https://github.com/StatPan/gira/issues/1"}]]`,
			},
		}
	}
	statusNow = func() time.Time {
		return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"status", "--repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("status JSON did not parse: %v\n%s", err, stdout.String())
	}
	if payload["repo"] != "StatPan/gira" {
		t.Fatalf("repo = %v, want StatPan/gira", payload["repo"])
	}
}

func TestOnboardVerifyRequiresRepoAndStage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"onboard", "verify", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "--stage is required") {
		t.Fatalf("stderr missing stage requirement:\n%s", stderr.String())
	}
}

func TestOnboardVerifyJSONUsesInjectedBuilder(t *testing.T) {
	restore := newOnboardVerifyReport
	t.Cleanup(func() { newOnboardVerifyReport = restore })
	newOnboardVerifyReport = func(repo gira.RepoRef, stage gira.OnboardStage) (gira.OnboardVerifyReport, error) {
		return gira.OnboardVerifyReport{
			Repo:      repo.FullName(),
			Command:   "onboard verify",
			Stage:     string(stage),
			CheckedAt: "2026-04-26T12:00:00Z",
			Ready:     false,
			BlockingChecklist: []gira.OnboardCheck{{
				ID:          "bootstrap_pr_template",
				Description: "bootstrap PR template is committed",
				Status:      gira.OnboardCheckFail,
				Remediation: "run gira bootstrap",
			}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"onboard", "verify", "--repo", "StatPan/gira", "--stage", "bootstrap", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"stage\": \"bootstrap\"") {
		t.Fatalf("onboard JSON missing stage: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "\"ready\": false") {
		t.Fatalf("onboard JSON missing ready=false: %s", stdout.String())
	}
}

func TestDoctorJSONUsesInjectedReportAndExitCode(t *testing.T) {
	restore := newDoctorReport
	t.Cleanup(func() { newDoctorReport = restore })
	newDoctorReport = func(repoValue string) gira.DoctorReport {
		if repoValue != "StatPan/gira" {
			t.Fatalf("repoValue = %q, want StatPan/gira", repoValue)
		}
		return gira.DoctorReport{
			Repo:      "StatPan/gira",
			Command:   "doctor",
			CheckedAt: "2026-05-05T12:00:00Z",
			Ready:     false,
			Checks: []gira.DoctorCheck{{
				ID:          "metadata_drift",
				Status:      gira.DoctorCheckFail,
				Detail:      "labels create=1 update=0; milestones create=0 update=0; bootstrap issues create=0",
				Remediation: "run `gira ops sync --repo StatPan/gira --dry-run`, then apply with `gira ops sync --repo StatPan/gira`",
			}},
		}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"doctor", "--repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"id\": \"metadata_drift\"") {
		t.Fatalf("doctor JSON missing check id:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "\"remediation\": \"run `gira ops sync --repo StatPan/gira --dry-run`") {
		t.Fatalf("doctor JSON missing remediation:\n%s", stdout.String())
	}
}

func TestDoctorHumanReadyAndUnexpectedArgument(t *testing.T) {
	restore := newDoctorReport
	t.Cleanup(func() { newDoctorReport = restore })
	newDoctorReport = func(repoValue string) gira.DoctorReport {
		return gira.DoctorReport{
			Repo:      repoValue,
			Command:   "doctor",
			CheckedAt: "2026-05-05T12:00:00Z",
			Ready:     true,
			Checks: []gira.DoctorCheck{{
				ID:     "repo_context",
				Status: gira.DoctorCheckPass,
				Detail: "using --repo " + repoValue,
			}},
		}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"doctor", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"doctor: READY", "repo: StatPan/gira", "next step: gira status --repo StatPan/gira"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("doctor output missing %q:\n%s", want, stdout.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"doctor", "extra"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unexpected argument: extra") {
		t.Fatalf("stderr missing unexpected argument:\n%s", stderr.String())
	}
}

func TestDoctorHelpFlagPrintsHelpAndExitsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"doctor", "-h"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "gira doctor [--repo OWNER/REPO] [--json]") {
		t.Fatalf("stdout missing doctor help:\n%s", stdout.String())
	}
}

func TestTicketNewDryRunReportsMissingRepoLabel(t *testing.T) {
	restoreBuilder := newTicketNewReport
	restoreRunner := devCommandRunner
	t.Cleanup(func() {
		newTicketNewReport = restoreBuilder
		devCommandRunner = restoreRunner
	})
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"gh label list --repo StatPan/gira --json name --limit 1000": []byte(`[{"name":"type:task"},{"name":"status:ready"},{"name":"area:backend"},{"name":"area:docs"}]`),
	}}
	newTicketNewReport = func(input gira.TicketNewInput) (gira.TicketNewReport, error) {
		return gira.BuildTicketNewReport(input, devCommandRunner)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "new", "Add CLI", "--repo", "StatPan/gira", "--label", "area:cli", "--dry-run"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout: %s stderr: %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "missing repo labels: area:cli") || !strings.Contains(stderr.String(), "candidates: area:backend,area:docs") {
		t.Fatalf("stderr missing label preflight error:\n%s", stderr.String())
	}
}

func TestTicketNewJSONIncludesPartialReportOnMissingRepoLabel(t *testing.T) {
	restoreBuilder := newTicketNewReport
	restoreRunner := devCommandRunner
	t.Cleanup(func() {
		newTicketNewReport = restoreBuilder
		devCommandRunner = restoreRunner
	})
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"gh label list --repo StatPan/gira --json name --limit 1000": []byte(`[{"name":"type:task"},{"name":"status:ready"}]`),
	}}
	newTicketNewReport = func(input gira.TicketNewInput) (gira.TicketNewReport, error) {
		return gira.BuildTicketNewReport(input, devCommandRunner)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "new", "Add CLI", "--repo", "StatPan/gira", "--label", "area:cli", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout: %s stderr: %s", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{`"title": "Add CLI"`, `"area:cli"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestTicketPromptTextUsesInjectedBuilder(t *testing.T) {
	restore := newTicketPromptReport
	t.Cleanup(func() { newTicketPromptReport = restore })
	newTicketPromptReport = func(input gira.AgentPromptInput) (gira.AgentPromptReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Ticket != 436 || input.Role != "implementer" || input.Profile != "python" {
			t.Fatalf("unexpected prompt input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.AgentPromptReport{
			Command:  "ticket prompt",
			Repo:     input.Repo.FullName(),
			Ticket:   input.Ticket,
			Role:     input.Role,
			Profile:  input.Profile,
			Prompt:   "# Gira implementer prompt\n\nPython profile: run pytest when configured.\n",
			NextStep: "gira ticket pr --repo StatPan/gira --ticket 436 --dry-run",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "prompt", "436", "--repo", "StatPan/gira", "--role", "implementer", "--profile", "python"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"# Gira implementer prompt", "pytest"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("ticket prompt output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestTicketPromptJSONUsesInjectedBuilder(t *testing.T) {
	restore := newTicketPromptReport
	t.Cleanup(func() { newTicketPromptReport = restore })
	newTicketPromptReport = func(input gira.AgentPromptInput) (gira.AgentPromptReport, error) {
		if input.PRNumber != 77 || input.Role != "reviewer" {
			t.Fatalf("unexpected prompt input: %+v", input)
		}
		return gira.AgentPromptReport{
			Command: "ticket prompt",
			Repo:    input.Repo.FullName(),
			Ticket:  input.Ticket,
			Role:    input.Role,
			Profile: input.Profile,
			PR:      &gira.AgentPromptPR{Number: input.PRNumber, Title: "feat: prompts"},
			Prompt:  "# Gira reviewer prompt\n",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "prompt", "436", "--repo", "StatPan/gira", "--role", "reviewer", "--pr", "77", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "ticket prompt"`, `"role": "reviewer"`, `"number": 77`, `"prompt": "# Gira reviewer prompt\n"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("ticket prompt JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestTicketPromptAcceptsPositionalRole(t *testing.T) {
	restore := newTicketPromptReport
	t.Cleanup(func() { newTicketPromptReport = restore })
	newTicketPromptReport = func(input gira.AgentPromptInput) (gira.AgentPromptReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Ticket != 436 || input.Role != "planner" {
			t.Fatalf("unexpected prompt input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.AgentPromptReport{Command: "ticket prompt", Repo: input.Repo.FullName(), Ticket: input.Ticket, Role: input.Role, Profile: input.Profile, Prompt: "# Gira planner prompt\n"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "prompt", "436", "planner", "--repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"role": "planner"`) {
		t.Fatalf("ticket prompt JSON missing positional role:\n%s", stdout.String())
	}
}

func TestTicketPromptRejectsConflictingPositionalRole(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "prompt", "436", "planner", "--repo", "StatPan/gira", "--role", "implementer"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "positional role and --role must match") {
		t.Fatalf("stderr missing role conflict:\n%s", stderr.String())
	}
}

func TestTicketHandoffJSONUsesInjectedBuilder(t *testing.T) {
	restore := newTicketHandoffReport
	t.Cleanup(func() { newTicketHandoffReport = restore })
	newTicketHandoffReport = func(input gira.TicketHandoffInput) (gira.TicketHandoffReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Ticket != 436 || input.Role != "planner" || input.Profile != "python" {
			t.Fatalf("unexpected handoff input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.TicketHandoffReport{
			Command:         "ticket handoff",
			SchemaVersion:   gira.WorkerHandoffSchemaVersion,
			Repo:            input.Repo.FullName(),
			Issue:           input.Ticket,
			Role:            input.Role,
			Profile:         input.Profile,
			Readiness:       gira.TicketReadinessReport{SchemaVersion: gira.TicketReadinessSchemaVersion, Readiness: "ready", NextAction: "start_ticket"},
			NextAction:      "plan",
			NextSafeCommand: "gira ticket prompt --repo StatPan/gira --ticket 436 --role planner --json",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "handoff", "436", "planner", "--repo", "StatPan/gira", "--profile", "python", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "ticket handoff"`, `"schema_version": "worker-handoff/v1"`, `"role": "planner"`, `"next_action": "plan"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("ticket handoff JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestTicketHandoffTextDefaultsImplementerRole(t *testing.T) {
	restore := newTicketHandoffReport
	t.Cleanup(func() { newTicketHandoffReport = restore })
	newTicketHandoffReport = func(input gira.TicketHandoffInput) (gira.TicketHandoffReport, error) {
		if input.Role != "implementer" {
			t.Fatalf("role = %q, want implementer", input.Role)
		}
		return gira.TicketHandoffReport{
			Command:         "ticket handoff",
			SchemaVersion:   gira.WorkerHandoffSchemaVersion,
			Repo:            input.Repo.FullName(),
			Issue:           input.Ticket,
			IssueURL:        "https://github.com/StatPan/gira/issues/436",
			Role:            input.Role,
			Profile:         input.Profile,
			Readiness:       gira.TicketReadinessReport{SchemaVersion: gira.TicketReadinessSchemaVersion, Readiness: "ready", NextAction: "start_ticket"},
			BranchPolicy:    gira.TicketHandoffBranchPolicy{Base: "main", Source: "branch_policy.default", WorkBranch: "issue-436-add-prompts"},
			NextAction:      "implement",
			NextSafeCommand: "gira ticket pr --repo StatPan/gira --ticket 436 --dry-run",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "handoff", "436", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"ticket handoff: #436 role=implementer", "branch: base=main", "next safe command:"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("ticket handoff output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunHelpIncludesStartStatusCollect(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"run", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"gira run start", "gira run status", "gira run collect"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("run help missing %q:\n%s", want, stdout.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"run", "status", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run status help exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "run status: no matching") {
		t.Fatalf("run status help should not print status output:\n%s", stdout.String())
	}
}

func TestRunStartStatusAndCollectUseLocalState(t *testing.T) {
	restore := newTicketHandoffReport
	t.Cleanup(func() { newTicketHandoffReport = restore })
	newTicketHandoffReport = func(input gira.TicketHandoffInput) (gira.TicketHandoffReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Ticket != 688 || input.Role != "implementer" {
			t.Fatalf("unexpected handoff input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.TicketHandoffReport{
			Command:       "ticket handoff",
			SchemaVersion: gira.WorkerHandoffSchemaVersion,
			Repo:          input.Repo.FullName(),
			Issue:         input.Ticket,
			Role:          input.Role,
			Profile:       input.Profile,
			Readiness:     gira.TicketReadinessReport{SchemaVersion: gira.TicketReadinessSchemaVersion, Readiness: "ready"},
			NextAction:    "implement",
		}, nil
	}

	stateRoot := t.TempDir()
	workDir := t.TempDir()
	var startOut, startErr bytes.Buffer
	code := Run([]string{"run", "start", "688", "--repo", "StatPan/gira", "--state-root", stateRoot, "--workdir", workDir, "--apply", "--json"}, &startOut, &startErr)
	if code != 0 {
		t.Fatalf("run start exit code = %d, want 0; stderr: %s", code, startErr.String())
	}
	var start gira.RunStartReport
	if err := json.Unmarshal(startOut.Bytes(), &start); err != nil {
		t.Fatalf("decode run start JSON: %v\n%s", err, startOut.String())
	}
	if start.Manifest.RunID == "" || start.Manifest.Status != "prepared" {
		t.Fatalf("unexpected start manifest: %+v", start.Manifest)
	}
	if _, err := os.Stat(start.Manifest.ManifestPath); err != nil {
		t.Fatalf("manifest was not written: %v", err)
	}
	if _, err := os.Stat(start.Manifest.PromptPath); err != nil {
		t.Fatalf("prompt was not written: %v", err)
	}

	var statusOut, statusErr bytes.Buffer
	code = Run([]string{"run", "status", "--latest", "--repo", "StatPan/gira", "--ticket", "688", "--state-root", stateRoot, "--json"}, &statusOut, &statusErr)
	if code != 0 {
		t.Fatalf("run status exit code = %d, want 0; stderr: %s", code, statusErr.String())
	}
	var status gira.RunStatusReport
	if err := json.Unmarshal(statusOut.Bytes(), &status); err != nil {
		t.Fatalf("decode run status JSON: %v\n%s", err, statusOut.String())
	}
	if status.Manifest == nil || status.Manifest.RunID != start.Manifest.RunID {
		t.Fatalf("status manifest = %+v, want %s", status.Manifest, start.Manifest.RunID)
	}

	if err := os.WriteFile(start.Manifest.ResultPath, []byte("collected\n"), 0o600); err != nil {
		t.Fatalf("write result: %v", err)
	}
	var collectOut, collectErr bytes.Buffer
	code = Run([]string{"run", "collect", "--id", start.Manifest.RunID, "--state-root", stateRoot}, &collectOut, &collectErr)
	if code != 0 {
		t.Fatalf("run collect exit code = %d, want 0; stderr: %s", code, collectErr.String())
	}
	if collectOut.String() != "collected\n" {
		t.Fatalf("collect output = %q", collectOut.String())
	}
}

func TestRunStartDryRunShowsIncludedPromptContext(t *testing.T) {
	restore := newTicketHandoffReport
	t.Cleanup(func() { newTicketHandoffReport = restore })
	newTicketHandoffReport = func(input gira.TicketHandoffInput) (gira.TicketHandoffReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Ticket != 690 || input.Role != "reviewer" {
			t.Fatalf("unexpected handoff input: %+v repo=%s", input, input.Repo.FullName())
		}
		if len(input.ContextNotes) != 1 || input.ContextNotes[0] != "Use the shared handoff path." {
			t.Fatalf("unexpected context notes: %+v", input.ContextNotes)
		}
		return gira.TicketHandoffReport{
			Command:       "ticket handoff",
			SchemaVersion: gira.WorkerHandoffSchemaVersion,
			Repo:          input.Repo.FullName(),
			Issue:         input.Ticket,
			Role:          input.Role,
			Profile:       input.Profile,
			Readiness:     gira.TicketReadinessReport{SchemaVersion: gira.TicketReadinessSchemaVersion, Readiness: "ready"},
			WorkOrder: gira.TicketHandoffWorkOrder{
				Goal:             "Reduce manual context",
				Acceptance:       []string{"run prompt includes ticket context"},
				ExpectedDelivery: "Document verification.",
				ReviewGuidance:   "Review prompt content.",
				TicketBody:       "## Goal\nReduce manual context",
			},
			Guidance:        []gira.AgentPromptGuidance{{Path: "AGENTS.md", Status: "found", Note: "content intentionally not expanded"}},
			RolePacket:      &gira.AgentPromptRolePacket{Role: input.Role, WorkOrder: []string{"review the PR diff"}},
			OperatorContext: []gira.TicketHandoffContext{{Source: "operator_note_1", Content: input.ContextNotes[0]}},
			NextAction:      "request_review",
			NextSafeCommand: "gira ticket review --repo StatPan/gira --ticket 690 --json",
		}, nil
	}

	stateRoot := t.TempDir()
	workDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"run", "start", "690", "--repo", "StatPan/gira", "--role", "reviewer", "--context", "Use the shared handoff path.", "--state-root", stateRoot, "--workdir", workDir, "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run start exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"handoff: schema=worker-handoff/v1 role=reviewer readiness=ready next=request_review", "context included:", "ticket body", "expected delivery", "review guidance", "policy pointers: AGENTS.md (found)", "extra context notes: 1", "storage: prompt is written to private local Gira state"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("run start dry-run output missing %q:\n%s", want, stdout.String())
		}
	}
	if _, err := os.Stat(filepath.Join(stateRoot, "runs")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not create run directory, stat err=%v", err)
	}
}

func TestGoalStatusJSONUsesInjectedBuilder(t *testing.T) {
	restore := newGoalStatusReport
	t.Cleanup(func() { newGoalStatusReport = restore })
	newGoalStatusReport = func(input gira.GoalStatusInput) (gira.GoalStatusReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Goal != 521 {
			t.Fatalf("unexpected goal status input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.GoalStatusReport{
			Command:       "goal status",
			SchemaVersion: gira.GoalStatusSchemaVersion,
			Repo:          input.Repo.FullName(),
			Goal:          gira.GoalStatusIssue{Number: input.Goal, Title: "Gira 2.0", State: "open", Status: "Ready"},
			Counts:        map[string]int{"total": 0},
			NextAction:    "plan_children",
			NextStep:      "gira goal plan --repo StatPan/gira --goal 521 --dry-run",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"goal", "status", "521", "--repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "goal status"`, `"schema_version": "goal-status/v1"`, `"next_action": "plan_children"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("goal status JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestGoalStatusTextAllowsInferredGoal(t *testing.T) {
	restore := newGoalStatusReport
	t.Cleanup(func() { newGoalStatusReport = restore })
	newGoalStatusReport = func(input gira.GoalStatusInput) (gira.GoalStatusReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Goal != 0 {
			t.Fatalf("unexpected inferred goal status input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.GoalStatusReport{
			Command:       "goal status",
			SchemaVersion: gira.GoalStatusSchemaVersion,
			Repo:          input.Repo.FullName(),
			Goal:          gira.GoalStatusIssue{Number: 521, Title: "Inferred", State: "open", Status: "Ready"},
			Counts:        map[string]int{"total": 0},
			NextAction:    "plan_children",
			NextStep:      "gira goal plan --repo StatPan/gira --goal 521 --dry-run",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"goal", "status", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "goal status: #521") {
		t.Fatalf("stdout missing inferred goal status:\n%s", stdout.String())
	}
}

func TestGoalNewJSONUsesInjectedBuilder(t *testing.T) {
	restore := newGoalNewReport
	t.Cleanup(func() { newGoalNewReport = restore })
	newGoalNewReport = func(input gira.GoalNewInput) (gira.GoalNewReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Title != "Ship goal mode" || !input.DryRun {
			t.Fatalf("unexpected goal new input: %+v repo=%s", input, input.Repo.FullName())
		}
		if input.Objective != "Make goal mode executable" || input.Scope != "CLI" || input.Type != "epic" || input.Priority != "p1" {
			t.Fatalf("unexpected goal new fields: %+v", input)
		}
		return gira.GoalNewReport{
			Command:       "goal new",
			SchemaVersion: gira.GoalNewReportSchemaVersion,
			Repo:          input.Repo.FullName(),
			Title:         input.Title,
			DryRun:        true,
			Type:          input.Type,
			Priority:      input.Priority,
			Labels:        []string{"type:epic", "status:ready", "priority:p1"},
			Body:          "## Goal\nMake goal mode executable\n",
			NextStep:      "gira goal new --apply",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"goal", "new", "Ship goal mode", "--repo", "StatPan/gira", "--objective", "Make goal mode executable", "--scope", "CLI", "--priority", "p1", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "goal new"`, `"schema_version": "goal-new-report/v1"`, `"approval"`, `"canonical_command": "gira goal new"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("goal new JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestGoalNewApplyTextUsesInjectedBuilder(t *testing.T) {
	restore := newGoalNewReport
	t.Cleanup(func() { newGoalNewReport = restore })
	newGoalNewReport = func(input gira.GoalNewInput) (gira.GoalNewReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Title != "Ship goal mode" || input.DryRun {
			t.Fatalf("unexpected goal new apply input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.GoalNewReport{
			Command:       "goal new",
			SchemaVersion: gira.GoalNewReportSchemaVersion,
			Repo:          input.Repo.FullName(),
			Title:         input.Title,
			Type:          "epic",
			Labels:        []string{"type:epic", "status:ready"},
			Created:       gira.TicketCreatedIssue{Repo: input.Repo.FullName(), Number: 521},
			NextStep:      "gira goal status 521 --repo StatPan/gira --json",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"goal", "new", "--title", "Ship goal mode", "--repo", "StatPan/gira", "--apply"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"goal new: goal #521 Ship goal mode", "next step: gira goal status 521 --repo StatPan/gira --json"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("goal new text missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestGoalNewRequiresExactlyOneMode(t *testing.T) {
	for _, args := range [][]string{
		{"goal", "new", "Ship goal mode", "--repo", "StatPan/gira"},
		{"goal", "new", "Ship goal mode", "--repo", "StatPan/gira", "--dry-run", "--apply"},
	} {
		var stdout, stderr bytes.Buffer
		code := Run(args, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("exit code = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stderr.String(), "exactly one of --dry-run or --apply is required for goal new") {
			t.Fatalf("stderr missing goal new mode requirement:\n%s", stderr.String())
		}
	}
}

func TestGoalDossierJSONUsesInjectedBuilder(t *testing.T) {
	restore := newGoalDossierReport
	t.Cleanup(func() { newGoalDossierReport = restore })
	newGoalDossierReport = func(input gira.GoalDossierInput) (gira.GoalDossierReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Goal != 521 || input.View != "operator" {
			t.Fatalf("unexpected goal dossier input: %+v repo=%s", input, input.Repo.FullName())
		}
		selected := gira.GoalNextCandidate{Number: 573, Title: "Next", Category: "ready", Reason: "next_ready_child", NextStep: "gira ticket start --repo StatPan/gira --ticket 573 --apply"}
		return gira.GoalDossierReport{
			Command:                 "goal dossier",
			SchemaVersion:           gira.GoalDossierSchemaVersion,
			Repo:                    input.Repo.FullName(),
			GeneratedAt:             "2026-05-31T00:00:00Z",
			Goal:                    gira.GoalStatusIssue{Number: input.Goal, Title: "Gira 3.0", State: "open", Status: "Ready"},
			Counts:                  map[string]int{"total": 1, "ready": 1},
			ChildGroups:             []gira.GoalDossierChildGroup{{Category: "ready", Count: 1, Children: []gira.GoalStatusChild{{Number: 573, Title: "Next", Category: "ready", Status: "Ready"}}}},
			SelectedTicket:          &selected,
			NextAction:              "start_child",
			NextStep:                selected.NextStep,
			RemainingAutonomousWork: 1,
			Evidence:                gira.GoalDossierEvidenceSummary{Sources: []string{"goal_status", "goal_next"}, ChildCount: 1, RemainingAutonomousWork: 1},
			Sources:                 []gira.GoalDossierSource{{Name: "goal_status", SchemaVersion: gira.GoalStatusSchemaVersion}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"goal", "dossier", "521", "--repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "goal dossier"`, `"schema_version": "goal-dossier/v1"`, `"child_groups"`, `"selected_ticket"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("goal dossier JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestGoalReportJSONUsesInjectedBuilder(t *testing.T) {
	restore := newGoalDossierReport
	t.Cleanup(func() { newGoalDossierReport = restore })
	newGoalDossierReport = func(input gira.GoalDossierInput) (gira.GoalDossierReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Goal != 521 || input.View != "ai" {
			t.Fatalf("unexpected goal report input: %+v repo=%s", input, input.Repo.FullName())
		}
		selected := gira.GoalNextCandidate{Number: 573, Title: "Next", Category: "ready", Reason: "next_ready_child", NextStep: "gira ticket start --repo StatPan/gira --ticket 573 --apply"}
		return gira.GoalDossierReport{
			Command:                 "goal dossier",
			SchemaVersion:           gira.GoalDossierSchemaVersion,
			Repo:                    input.Repo.FullName(),
			GeneratedAt:             "2026-05-31T00:00:00Z",
			Goal:                    gira.GoalStatusIssue{Number: input.Goal, Title: "Gira 3.0", State: "open", Status: "Ready"},
			Counts:                  map[string]int{"total": 1, "ready": 1},
			ChildGroups:             []gira.GoalDossierChildGroup{{Category: "ready", Count: 1, Children: []gira.GoalStatusChild{{Number: 573, Title: "Next", Category: "ready", Status: "Ready"}}}},
			SelectedTicket:          &selected,
			NextAction:              "start_child",
			NextStep:                selected.NextStep,
			RemainingAutonomousWork: 1,
			Evidence:                gira.GoalDossierEvidenceSummary{Sources: []string{"goal_status", "goal_next"}, ChildCount: 1, RemainingAutonomousWork: 1},
			Sources:                 []gira.GoalDossierSource{{Name: "goal_status", SchemaVersion: gira.GoalStatusSchemaVersion}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"goal", "report", "521", "--repo", "StatPan/gira", "--view", "ai", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "goal report"`, `"schema_version": "goal-dossier/v1"`, `"child_groups"`, `"selected_ticket"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("goal report JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestGoalReportHTMLWritesOutput(t *testing.T) {
	restore := newGoalDossierReport
	t.Cleanup(func() { newGoalDossierReport = restore })
	newGoalDossierReport = func(input gira.GoalDossierInput) (gira.GoalDossierReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Goal != 521 {
			t.Fatalf("unexpected goal report input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.GoalDossierReport{
			Command:       "goal dossier",
			SchemaVersion: gira.GoalDossierSchemaVersion,
			Repo:          input.Repo.FullName(),
			GeneratedAt:   "2026-05-31T00:00:00Z",
			Goal:          gira.GoalStatusIssue{Number: input.Goal, Title: "Gira <3", State: "open", Status: "Ready"},
			Counts:        map[string]int{"total": 0},
			NextAction:    "plan_children",
			NextStep:      "gira goal plan --repo StatPan/gira --goal 521 --dry-run",
			Evidence:      gira.GoalDossierEvidenceSummary{Sources: []string{"goal_status", "goal_next"}},
		}, nil
	}

	outputPath := filepath.Join(t.TempDir(), "goal-521.html")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"goal", "report", "521", "--repo", "StatPan/gira", "--html", "--output", outputPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "goal report html:") || !strings.Contains(stdout.String(), "next step: open") {
		t.Fatalf("goal report HTML output missing next-step summary:\n%s", stdout.String())
	}
	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read report HTML: %v", err)
	}
	for _, want := range []string{"Gira goal report", "Gira &lt;3", gira.GoalDossierSchemaVersion} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("report HTML missing %q:\n%s", want, string(got))
		}
	}
}

func TestGoalReportRejectsInvalidOutputFlags(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{
			name: "json and html",
			args: []string{"goal", "report", "521", "--repo", "StatPan/gira", "--json", "--html", "--output", "goal.html"},
			want: "choose exactly one output format",
		},
		{
			name: "html needs output",
			args: []string{"goal", "report", "521", "--repo", "StatPan/gira", "--html"},
			want: "--output is required when using --html",
		},
		{
			name: "output needs html",
			args: []string{"goal", "report", "521", "--repo", "StatPan/gira", "--output", "goal.html"},
			want: "--output requires --html",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tc.args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("exit code = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("stderr missing %q:\n%s", tc.want, stderr.String())
			}
		})
	}
}

func TestGoalNextJSONUsesInjectedBuilder(t *testing.T) {
	restore := newGoalNextReport
	t.Cleanup(func() { newGoalNextReport = restore })
	newGoalNextReport = func(input gira.GoalNextInput) (gira.GoalNextReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Goal != 521 {
			t.Fatalf("unexpected goal next input: %+v repo=%s", input, input.Repo.FullName())
		}
		selected := gira.GoalNextCandidate{Number: 573, Title: "Next", Category: "ready", Reason: "next_ready_child", NextStep: "gira ticket start --repo StatPan/gira --ticket 573 --apply"}
		return gira.GoalNextReport{
			Command:        "goal next",
			SchemaVersion:  gira.GoalNextSchemaVersion,
			Repo:           input.Repo.FullName(),
			Goal:           gira.GoalStatusIssue{Number: input.Goal, Title: "Gira 2.0", State: "open", Status: "Ready"},
			SelectedTicket: &selected,
			Counts:         map[string]int{"total": 1, "ready": 1},
			NextAction:     "start_child",
			NextStep:       selected.NextStep,
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"goal", "next", "521", "--repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "goal next"`, `"schema_version": "goal-next/v1"`, `"selected_ticket"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("goal next JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestGoalHandoffJSONUsesInjectedBuilder(t *testing.T) {
	restore := newGoalHandoffReport
	t.Cleanup(func() { newGoalHandoffReport = restore })
	newGoalHandoffReport = func(input gira.GoalHandoffInput) (gira.GoalHandoffReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Goal != 521 || input.Role != "reviewer" || input.Profile != "python" {
			t.Fatalf("unexpected goal handoff input: %+v repo=%s", input, input.Repo.FullName())
		}
		selected := gira.GoalNextCandidate{Repo: "StatPan/gira", Number: 573, Title: "Next", Category: "in_review", Reason: "review_or_finish_before_new_work", NextStep: "gira ticket review --repo StatPan/gira --ticket 573 --json"}
		worker := gira.TicketHandoffReport{
			Command:         "ticket handoff",
			SchemaVersion:   gira.WorkerHandoffSchemaVersion,
			Role:            "reviewer",
			Profile:         "python",
			Repo:            input.Repo.FullName(),
			Issue:           573,
			Title:           "Next",
			Readiness:       gira.TicketReadinessReport{SchemaVersion: gira.TicketReadinessSchemaVersion, Readiness: "ready"},
			NextAction:      "request_review",
			NextSafeCommand: "gira ticket review --repo StatPan/gira --ticket 573 --json",
			PrivateStorage:  true,
			StorageNotice:   "private",
		}
		return gira.GoalHandoffReport{
			Command:         "goal handoff",
			SchemaVersion:   gira.GoalHandoffSchemaVersion,
			Repo:            input.Repo.FullName(),
			Role:            input.Role,
			Profile:         input.Profile,
			Goal:            gira.GoalStatusIssue{Number: input.Goal, Title: "Gira 3.0", State: "open", Status: "Ready"},
			GoalContext:     gira.GoalHandoffContext{Objective: "Ship goal delegation"},
			SelectedTicket:  &selected,
			WorkerHandoff:   &worker,
			NextAction:      "handoff_child",
			NextSafeCommand: worker.NextSafeCommand,
			PrivateStorage:  true,
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"goal", "handoff", "521", "--repo", "StatPan/gira", "--role", "reviewer", "--profile", "python", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "goal handoff"`, `"schema_version": "goal-handoff/v1"`, `"worker_handoff"`, `"schema_version": "worker-handoff/v1"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("goal handoff JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestDispatchGoalJSONAllowsInferredGoal(t *testing.T) {
	restore := newDispatchGoalPacket
	t.Cleanup(func() { newDispatchGoalPacket = restore })
	newDispatchGoalPacket = func(input gira.DispatchGoalInput) (gira.DispatchPacket, error) {
		if input.Repo.FullName() != "StatPan/backlog" || input.Goal != 0 || input.Role != "implementer" || input.Profile != "python" {
			t.Fatalf("unexpected dispatch goal input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.DispatchPacket{
			Command:       "dispatch goal",
			SchemaVersion: gira.DispatchPacketSchemaVersion,
			Source:        gira.DispatchSource{Kind: "goal", Repo: input.Repo.FullName(), Number: 521},
			Role:          input.Role,
			Profile:       input.Profile,
			Authority: []gira.DispatchReference{
				{Kind: "goal", Repo: input.Repo.FullName(), Number: 521, Title: "Dispatch goal"},
			},
			Instruction:     gira.DispatchInstruction{Objective: "Issue official AI work orders", SelectedWork: "StatPan/gira#573 Add dispatch"},
			NextAction:      "handoff_child",
			NextSafeCommand: "gira ticket start --repo StatPan/gira --ticket 573 --apply",
			PrivateStorage:  true,
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"dispatch", "goal", "--repo", "StatPan/backlog", "--role", "implementer", "--profile", "python", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "dispatch goal"`, `"schema_version": "dispatch-packet/v1"`, `"source"`, `"authority"`, `"selected_work": "StatPan/gira#573 Add dispatch"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("dispatch goal JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestDispatchGoalCompactJSONUsesInjectedBuilder(t *testing.T) {
	restore := newDispatchGoalPacket
	t.Cleanup(func() { newDispatchGoalPacket = restore })
	newDispatchGoalPacket = func(input gira.DispatchGoalInput) (gira.DispatchPacket, error) {
		if input.Repo.FullName() != "StatPan/backlog" || input.Goal != 0 {
			t.Fatalf("unexpected dispatch compact input: %+v repo=%s", input, input.Repo.FullName())
		}
		return dispatchGoalCLITestPacket(input), nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"dispatch", "goal", "--repo", "StatPan/backlog", "--compact-json", "--context-budget", "1200"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"schema_version": "dispatch-compact/v1"`, `"selected_ticket"`, `"acceptance"`, `"next_safe_command"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("compact dispatch JSON missing %q:\n%s", want, stdout.String())
		}
	}
	for _, leaked := range []string{"FULL TICKET BODY SHOULD NOT APPEAR", "ticket_body", "role_packet"} {
		if strings.Contains(stdout.String(), leaked) {
			t.Fatalf("compact dispatch JSON leaked %q:\n%s", leaked, stdout.String())
		}
	}
}

func TestDispatchGoalPromptUsesInjectedBuilder(t *testing.T) {
	restore := newDispatchGoalPacket
	t.Cleanup(func() { newDispatchGoalPacket = restore })
	newDispatchGoalPacket = func(input gira.DispatchGoalInput) (gira.DispatchPacket, error) {
		return dispatchGoalCLITestPacket(input), nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"dispatch", "goal", "--repo", "StatPan/backlog", "--prompt", "--context-budget", "1200"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"# Gira Dispatch", "## Selected Work", "## Acceptance", "## Next Safe Command"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("dispatch prompt missing %q:\n%s", want, stdout.String())
		}
	}
	if len(stdout.String()) > 1200 || strings.Contains(stdout.String(), "FULL TICKET BODY SHOULD NOT APPEAR") {
		t.Fatalf("dispatch prompt not compact enough len=%d:\n%s", len(stdout.String()), stdout.String())
	}
}

func TestDispatchGoalRejectsMultipleOutputFormats(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"dispatch", "goal", "--repo", "StatPan/backlog", "--json", "--prompt"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "choose at most one output format") {
		t.Fatalf("stderr missing output conflict:\n%s", stderr.String())
	}
}

func dispatchGoalCLITestPacket(input gira.DispatchGoalInput) gira.DispatchPacket {
	selected := gira.GoalNextCandidate{Repo: "StatPan/gira", Number: 573, Title: "Compact dispatch", State: "open", Status: "Ready", Category: "ready", URL: "https://github.com/StatPan/gira/issues/573"}
	worker := gira.TicketHandoffReport{
		SchemaVersion: gira.WorkerHandoffSchemaVersion,
		Role:          input.Role,
		Profile:       input.Profile,
		Repo:          "StatPan/gira",
		Issue:         573,
		Title:         "Compact dispatch",
		Readiness:     gira.TicketReadinessReport{SchemaVersion: gira.TicketReadinessSchemaVersion, Readiness: "ready"},
		WorkOrder: gira.TicketHandoffWorkOrder{
			Goal:       "Reduce context waste",
			Scope:      strings.Repeat("compact scope ", 200),
			Acceptance: []string{"compact output omits full ticket body", "prompt fits budget"},
			TicketBody: "FULL TICKET BODY SHOULD NOT APPEAR",
		},
		RequiredChecks:       []string{"go test ./..."},
		EvidenceExpectations: []string{"compact output includes selected work"},
		NextSafeCommand:      "gira ticket start --repo StatPan/gira --ticket 573 --apply",
	}
	handoff := gira.GoalHandoffReport{
		Repo:            input.Repo.FullName(),
		Role:            input.Role,
		Profile:         input.Profile,
		Goal:            gira.GoalStatusIssue{Number: 521, Title: "Dispatch goal", State: "open", Status: "Ready"},
		GoalContext:     gira.GoalHandoffContext{Objective: "Reduce token waste", StopConditions: []string{"unclear selected work"}},
		GoalStatus:      gira.GoalStatusReport{Counts: map[string]int{"ready": 1, "total": 1}, RemainingAutonomousWork: 1},
		SelectedTicket:  &selected,
		WorkerHandoff:   &worker,
		NextAction:      "handoff_child",
		NextSafeCommand: worker.NextSafeCommand,
	}
	return gira.DispatchPacket{
		Command:         "dispatch goal",
		SchemaVersion:   gira.DispatchPacketSchemaVersion,
		Source:          gira.DispatchSource{Kind: "goal", Repo: input.Repo.FullName(), Number: 521},
		Role:            input.Role,
		Profile:         input.Profile,
		Authority:       []gira.DispatchReference{{Kind: "goal", Repo: input.Repo.FullName(), Number: 521}},
		References:      []gira.DispatchReference{{Kind: "selected_ticket_issue", Repo: "StatPan/gira", Number: 573}},
		Instruction:     gira.DispatchInstruction{Objective: "Reduce token waste", SelectedWork: "StatPan/gira#573 Compact dispatch", AllowedActions: []string{"Execute only selected child ticket."}, StopConditions: []string{"unclear selected work"}, EvidenceRequired: []string{"go test ./..."}},
		GoalHandoff:     &handoff,
		WorkerHandoff:   &worker,
		NextAction:      "handoff_child",
		NextSafeCommand: worker.NextSafeCommand,
		PrivateStorage:  true,
	}
}

func TestGoalPlanJSONUsesInjectedBuilder(t *testing.T) {
	restore := newGoalPlanReport
	t.Cleanup(func() { newGoalPlanReport = restore })
	newGoalPlanReport = func(input gira.GoalPlanInput) (gira.GoalPlanReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Goal != 521 || !input.DryRun || input.Apply {
			t.Fatalf("unexpected goal plan input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.GoalPlanReport{
			Command:       "goal plan",
			SchemaVersion: gira.GoalPlanSchemaVersion,
			Repo:          input.Repo.FullName(),
			DryRun:        input.DryRun,
			Goal:          gira.GoalStatusIssue{Number: input.Goal, Title: "Gira 2.0", State: "open", Status: "Ready"},
			ProposedTickets: []gira.GoalPlanTicket{
				{Title: "[Task] Add plan", ParentGoal: input.Goal, Goal: "Add plan", Scope: "CLI", Acceptance: []string{"works"}, ExpectedEvidence: []string{"go test ./..."}},
			},
			NextAction: "create_child_tickets",
			NextStep:   "review proposed tickets",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"goal", "plan", "521", "--repo", "StatPan/gira", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "goal plan"`, `"schema_version": "goal-plan/v1"`, `"proposed_tickets"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("goal plan JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestGoalPlanApplyJSONUsesInjectedBuilder(t *testing.T) {
	restore := newGoalPlanReport
	t.Cleanup(func() { newGoalPlanReport = restore })
	newGoalPlanReport = func(input gira.GoalPlanInput) (gira.GoalPlanReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Goal != 521 || input.DryRun || !input.Apply {
			t.Fatalf("unexpected goal plan apply input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.GoalPlanReport{
			Command:       "goal plan",
			SchemaVersion: gira.GoalPlanSchemaVersion,
			Repo:          input.Repo.FullName(),
			Apply:         true,
			Goal:          gira.GoalStatusIssue{Number: input.Goal, Title: "Gira 2.0", State: "open", Status: "Ready"},
			CreatedChildren: []gira.GoalPlanChild{
				{Number: 700, Title: "[Task] Add plan", Category: "ready", Status: "Ready"},
			},
			Actions:    []gira.GoalPlanAction{{Action: "child_ticket:create", Title: "[Task] Add plan", Status: "applied", Issue: 700}},
			NextAction: "inspect_goal",
			NextStep:   "gira goal status --repo StatPan/gira --goal 521 --json",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"goal", "plan", "521", "--repo", "StatPan/gira", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"apply": true`, `"created_children"`, `"issue": 700`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("goal plan apply JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestGoalPlanCompactApplyStopsBeforeMutationWhenPlanChanges(t *testing.T) {
	restore := newGoalPlanReport
	t.Cleanup(func() { newGoalPlanReport = restore })
	calls := 0
	newGoalPlanReport = func(input gira.GoalPlanInput) (gira.GoalPlanReport, error) {
		calls++
		if !input.DryRun || input.Apply {
			t.Fatalf("compact apply must not invoke mutation after a plan mismatch: %+v", input)
		}
		return goalPlanCLITestReport(input), nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"goal", "plan", "521", "--repo", "StatPan/gira", "--apply", "--compact-json", "--expect-plan", "gpp-stale"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if calls != 1 {
		t.Fatalf("goal plan builder calls = %d, want only dry-run preview", calls)
	}
	if !strings.Contains(stdout.String(), `"plan_changed"`) || !strings.Contains(stdout.String(), `"next_action": "rerun_dry_run"`) {
		t.Fatalf("compact mismatch response missing stop contract:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), `"receipt"`) {
		t.Fatalf("compact mismatch must not claim an apply receipt:\n%s", stdout.String())
	}
}

func TestGoalPlanCompactApplyEmitsReceiptAfterMatchingPreview(t *testing.T) {
	restore := newGoalPlanReport
	t.Cleanup(func() { newGoalPlanReport = restore })
	var inputs []gira.GoalPlanInput
	newGoalPlanReport = func(input gira.GoalPlanInput) (gira.GoalPlanReport, error) {
		inputs = append(inputs, input)
		report := goalPlanCLITestReport(input)
		if input.Apply {
			report.CreatedChildren = []gira.GoalPlanChild{{Number: 700, Title: "[Task] Add plan"}}
			report.Actions = []gira.GoalPlanAction{{Action: "child_ticket:create", Status: "applied", Issue: 700}}
		}
		return report, nil
	}
	expected := gira.BuildGoalPlanCompactReport(goalPlanCLITestReport(gira.GoalPlanInput{Repo: gira.RepoRef{Owner: "StatPan", Name: "gira"}, Goal: 521, DryRun: true}), "dry_run", "").PlanID

	var stdout, stderr bytes.Buffer
	code := Run([]string{"goal", "plan", "521", "--repo", "StatPan/gira", "--apply", "--compact-json", "--expect-plan", expected}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if len(inputs) != 2 || !inputs[0].DryRun || inputs[0].Apply || inputs[1].DryRun || !inputs[1].Apply {
		t.Fatalf("compact apply inputs = %+v, want preview then apply", inputs)
	}
	if !strings.Contains(stdout.String(), `"receipt"`) || strings.Contains(stdout.String(), `"proposals"`) || !strings.Contains(stdout.String(), `"issue": 700`) {
		t.Fatalf("compact apply receipt contract invalid:\n%s", stdout.String())
	}
}

func TestGoalPlanCompactApplyRequiresExpectedPlan(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"goal", "plan", "521", "--repo", "StatPan/gira", "--apply", "--compact-json"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "--compact-json --apply requires --expect-plan from a dry-run") {
		t.Fatalf("stderr missing compact apply guard:\n%s", stderr.String())
	}
}

func goalPlanCLITestReport(input gira.GoalPlanInput) gira.GoalPlanReport {
	return gira.GoalPlanReport{
		Command:       "goal plan",
		SchemaVersion: gira.GoalPlanSchemaVersion,
		Repo:          input.Repo.FullName(),
		DryRun:        input.DryRun,
		Apply:         input.Apply,
		Goal:          gira.GoalStatusIssue{Number: input.Goal, Title: "Gira 2.0", State: "open", Status: "Ready"},
		ProposedTickets: []gira.GoalPlanTicket{
			{Title: "[Task] Add plan", TargetRepo: input.Repo.FullName(), Type: "task", Goal: "Add plan", Scope: "CLI", Acceptance: []string{"works"}, ExpectedEvidence: []string{"go test ./..."}, Body: "compact fixture"},
		},
		NextAction: "create_child_tickets",
		NextStep:   "review proposed tickets",
	}
}

func TestGoalPlanRequiresExactlyOneMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"goal", "plan", "521", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "exactly one of --dry-run or --apply is required for goal plan") {
		t.Fatalf("stderr missing mode requirement:\n%s", stderr.String())
	}
	stderr.Reset()
	stdout.Reset()
	code = Run([]string{"goal", "plan", "521", "--repo", "StatPan/gira", "--dry-run", "--apply"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "exactly one of --dry-run or --apply is required for goal plan") {
		t.Fatalf("expected dry-run/apply conflict; code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

func TestGoalFinishJSONUsesInjectedBuilder(t *testing.T) {
	restore := newGoalFinishReport
	t.Cleanup(func() { newGoalFinishReport = restore })
	newGoalFinishReport = func(input gira.GoalFinishInput) (gira.GoalFinishReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Goal != 521 || !input.DryRun || input.Apply || input.Terminal != "human_review" {
			t.Fatalf("unexpected goal finish input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.GoalFinishReport{
			Command: "goal finish",
			Repo:    input.Repo.FullName(),
			Goal:    input.Goal,
			DryRun:  input.DryRun,
			Readiness: gira.GoalFinishReadiness{
				SchemaVersion:          gira.GoalFinishReadinessSchemaVersion,
				Repository:             input.Repo.FullName(),
				Goal:                   gira.GoalStatusIssue{Number: input.Goal, Title: "Gira 2.0", State: "open", Status: "Ready"},
				TerminalRecommendation: "human_review",
				Blockers:               []string{"child_575_open"},
				NextAction:             "human_review",
				NextStep:               "review blockers",
			},
			NextAction: "human_review",
			NextStep:   "review blockers",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"goal", "finish", "521", "--repo", "StatPan/gira", "--dry-run", "--terminal", "human_review", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "goal finish"`, `"schema_version": "goal-finish-readiness/v1"`, `"terminal_recommendation": "human_review"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("goal finish JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestGoalFinishApplyUsesInjectedBuilder(t *testing.T) {
	restore := newGoalFinishReport
	t.Cleanup(func() { newGoalFinishReport = restore })
	newGoalFinishReport = func(input gira.GoalFinishInput) (gira.GoalFinishReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Goal != 521 || input.DryRun || !input.Apply || input.Terminal != "human_review" {
			t.Fatalf("unexpected goal finish input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.GoalFinishReport{
			Command: "goal finish",
			Repo:    input.Repo.FullName(),
			Goal:    input.Goal,
			Apply:   input.Apply,
			Readiness: gira.GoalFinishReadiness{
				SchemaVersion:          gira.GoalFinishReadinessSchemaVersion,
				Repository:             input.Repo.FullName(),
				Goal:                   gira.GoalStatusIssue{Number: input.Goal, Title: "Gira 2.0", State: "open", Status: "Ready"},
				TerminalRecommendation: "human_review",
				Blockers:               []string{"child_575_missing_finish_receipt"},
				NextAction:             "human_review",
				NextStep:               "review blockers",
			},
			Actions:    []gira.GoalFinishAction{{Action: "goal:comment", Status: "applied", Detail: "posted receipt"}},
			NextAction: "human_review",
			NextStep:   "human review handoff receipt posted",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"goal", "finish", "521", "--repo", "StatPan/gira", "--apply", "--terminal", "human_review", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"apply": true`, `"action": "goal:comment"`, `"status": "applied"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("goal finish apply JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestGoalFinishRequiresDryRunOrApply(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"goal", "finish", "521", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "exactly one of --dry-run or --apply") {
		t.Fatalf("stderr missing dry-run/apply requirement:\n%s", stderr.String())
	}
}

func TestTicketReviewInfersTicketAndDefaultsReviewerRole(t *testing.T) {
	restorePrompt := newTicketPromptReport
	restoreRunner := devCommandRunner
	t.Cleanup(func() {
		newTicketPromptReport = restorePrompt
		devCommandRunner = restoreRunner
	})
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"git branch --show-current": []byte("issue-436-add-prompts\n"),
	}}
	newTicketPromptReport = func(input gira.AgentPromptInput) (gira.AgentPromptReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Ticket != 436 || input.Role != "reviewer" || input.PRNumber != 77 {
			t.Fatalf("unexpected review input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.AgentPromptReport{
			Command: "ticket prompt",
			Repo:    input.Repo.FullName(),
			Ticket:  input.Ticket,
			Role:    input.Role,
			Profile: input.Profile,
			PR:      &gira.AgentPromptPR{Number: input.PRNumber, Title: "feat: prompts", FinishReady: true},
			Prompt:  "# Gira reviewer prompt\n\nFinish Ready: `true`\n",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "review", "--repo", "StatPan/gira", "--pr", "77", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "ticket review"`, `"role": "reviewer"`, `"ticket": 436`, `"finish_ready": true`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("ticket review JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestTicketReviewDiffSummaryUsesCurrentBranchContext(t *testing.T) {
	restorePrompt := newTicketPromptReport
	restoreRunner := devCommandRunner
	t.Cleanup(func() {
		newTicketPromptReport = restorePrompt
		devCommandRunner = restoreRunner
	})
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"git branch --show-current": []byte("issue-646-review-ux\n"),
	}}
	newTicketPromptReport = func(input gira.AgentPromptInput) (gira.AgentPromptReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Ticket != 646 || input.Role != "reviewer" || !input.IncludeDiffSummary || input.IncludeDiff {
			t.Fatalf("unexpected review input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.AgentPromptReport{
			Command: "ticket review",
			Repo:    input.Repo.FullName(),
			Ticket:  input.Ticket,
			Role:    input.Role,
			Profile: input.Profile,
			Issue:   gira.AgentPromptIssue{Number: input.Ticket, Title: "Review UX"},
			PR:      &gira.AgentPromptPR{Number: 647, Title: "review ux"},
			Review: &gira.AgentReviewContract{
				DiffSummary: &gira.AgentReviewDiffSummary{
					ChangedFiles:    []string{"internal/cli/cli.go"},
					Files:           []gira.AgentReviewDiffFile{{Path: "internal/cli/cli.go", Additions: 3, Deletions: 1}},
					TotalAdditions:  3,
					TotalDeletions:  1,
					FullDiffCommand: "gh pr diff 647 --repo StatPan/gira",
				},
			},
			Prompt: "# Gira reviewer prompt\n\n- Diff Summary:\n  - files: 1 changed, +3/-1\n",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "review", "--repo", "StatPan/gira", "--diff-summary", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"ticket": 646`, `"diff_summary"`, `"total_additions": 3`, `"full_diff_command": "gh pr diff 647 --repo StatPan/gira"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("ticket review diff summary JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestTicketReviewHTMLWritesOutput(t *testing.T) {
	restorePrompt := newTicketPromptReport
	t.Cleanup(func() { newTicketPromptReport = restorePrompt })
	newTicketPromptReport = func(input gira.AgentPromptInput) (gira.AgentPromptReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Ticket != 646 || input.Role != "reviewer" || !input.IncludeDiffSummary {
			t.Fatalf("unexpected review input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.AgentPromptReport{
			Command: "ticket review",
			Repo:    input.Repo.FullName(),
			Ticket:  input.Ticket,
			Role:    input.Role,
			Profile: input.Profile,
			Issue:   gira.AgentPromptIssue{Number: input.Ticket, Title: "Review <UX>", State: "open"},
			PR:      &gira.AgentPromptPR{Number: 647, URL: "https://github.com/StatPan/gira/pull/647", Title: "review ux", State: "OPEN", ReviewDecision: "APPROVED", FinishReady: true},
			PRReady: &gira.PRReadinessReport{SchemaVersion: gira.PRReadinessSchemaVersion, Repo: input.Repo.FullName(), Issue: input.Ticket, PullRequest: 647, Readiness: "ready_for_finish", NextAction: "finish_ticket"},
			Review: &gira.AgentReviewContract{
				DiffSummary: &gira.AgentReviewDiffSummary{
					ChangedFiles:    []string{"internal/cli/cli.go"},
					Files:           []gira.AgentReviewDiffFile{{Path: "internal/cli/cli.go", Additions: 3, Deletions: 1}},
					TotalAdditions:  3,
					TotalDeletions:  1,
					FullDiffCommand: "gh pr diff 647 --repo StatPan/gira",
				},
				VerdictSchema: gira.AgentReviewVerdictSchema{RecommendedAction: []string{"approve", "request_changes"}},
			},
			Prompt:   "# Gira reviewer prompt\n",
			NextStep: "gira ticket finish --apply",
		}, nil
	}

	outputPath := filepath.Join(t.TempDir(), "review-646.html")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "review", "646", "--repo", "StatPan/gira", "--diff-summary", "--html", "--output", outputPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ticket review html:") || !strings.Contains(stdout.String(), "next step: open") {
		t.Fatalf("ticket review HTML output missing next-step summary:\n%s", stdout.String())
	}
	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read ticket review HTML: %v", err)
	}
	for _, want := range []string{"Gira review packet", "Review &lt;UX&gt;", "ready_for_finish", "gh pr diff 647 --repo StatPan/gira"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("ticket review HTML missing %q:\n%s", want, string(got))
		}
	}
}

func TestTicketReviewRejectsInvalidOutputFlags(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{
			name: "json and html",
			args: []string{"ticket", "review", "646", "--repo", "StatPan/gira", "--json", "--html", "--output", "review.html"},
			want: "choose exactly one output format",
		},
		{
			name: "html needs output",
			args: []string{"ticket", "review", "646", "--repo", "StatPan/gira", "--html"},
			want: "--output is required when using --html",
		},
		{
			name: "output needs html",
			args: []string{"ticket", "review", "646", "--repo", "StatPan/gira", "--output", "review.html"},
			want: "--output requires --html",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tc.args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("exit code = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("stderr missing %q:\n%s", tc.want, stderr.String())
			}
		})
	}
}

func TestTicketReviewIncludeDiffRequiresExplicitFlag(t *testing.T) {
	restorePrompt := newTicketPromptReport
	restoreRunner := devCommandRunner
	t.Cleanup(func() {
		newTicketPromptReport = restorePrompt
		devCommandRunner = restoreRunner
	})
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"git branch --show-current": []byte("issue-646-review-ux\n"),
	}}
	newTicketPromptReport = func(input gira.AgentPromptInput) (gira.AgentPromptReport, error) {
		if !input.IncludeDiffSummary || !input.IncludeDiff {
			t.Fatalf("expected include diff flags, got %+v", input)
		}
		return gira.AgentPromptReport{Command: "ticket review", Repo: input.Repo.FullName(), Ticket: input.Ticket, Role: input.Role, Profile: input.Profile, Prompt: "prompt\n"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "review", "--repo", "StatPan/gira", "--include-diff"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
}

func TestTicketReviewMissingContextGivesActionableError(t *testing.T) {
	restoreRunner := devCommandRunner
	t.Cleanup(func() { devCommandRunner = restoreRunner })
	devCommandRunner = devCLIRunner{errs: map[string]error{
		"git branch --show-current":                                    fmt.Errorf("not a branch"),
		"gh pr view --repo StatPan/gira --json body,headRefName,title": fmt.Errorf("no pr"),
	}}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "review", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stdout: %s stderr: %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "cannot determine current ticket") || !strings.Contains(stderr.String(), "Try: gira ticket status --repo StatPan/gira --ticket N") {
		t.Fatalf("stderr missing context guidance:\n%s", stderr.String())
	}
}

func TestTicketSelfReviewInfersTicketAndPreviewsPRNote(t *testing.T) {
	restoreSelfReview := newTicketSelfReviewReport
	restoreRunner := devCommandRunner
	t.Cleanup(func() {
		newTicketSelfReviewReport = restoreSelfReview
		devCommandRunner = restoreRunner
	})
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"git branch --show-current": []byte("issue-646-review-ux\n"),
	}}
	newTicketSelfReviewReport = func(input gira.TicketSelfReviewInput) (gira.TicketSelfReviewReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Ticket != 646 || !input.DryRun || input.Apply || !input.DiffSummary {
			t.Fatalf("unexpected self-review input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.TicketSelfReviewReport{
			SchemaVersion: gira.TicketSelfReviewReportSchemaVersion,
			Command:       "ticket self-review",
			Repo:          input.Repo.FullName(),
			Ticket:        input.Ticket,
			PullRequest:   647,
			DiffSummary:   true,
			DryRun:        true,
			RenderedBody:  "## Check\n\nSelf-review packet for ticket #646 and PR #647.",
			NextStep:      "gira ticket note --apply",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "self-review", "--repo", "StatPan/gira", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"schema_version": "ticket-self-review-report/v1"`, `"ticket": 646`, `"pull_request": 647`, `"dry_run": true`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("self-review JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestMilestoneStatusMissingTitleGivesActionableError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"milestone", "status", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{"cannot determine milestone title", "Try: gira milestone list --repo OWNER/REPO", "Then: gira milestone status \"MILESTONE\" --repo OWNER/REPO"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}

func TestSyncDryRunUsesInjectedClientWithoutApplying(t *testing.T) {
	restoreClient := newSyncClient
	t.Cleanup(func() {
		newSyncClient = restoreClient
	})
	client := &cliFakeSyncClient{
		repo: mustCLIRepo(t, "StatPan/gira"),
		labels: []gira.ExistingLabel{
			{Name: gira.BootstrapLabel, Color: "5319E7", Description: "Created or managed by Gira bootstrap metadata sync."},
		},
	}
	newSyncClient = func(repo gira.RepoRef) gira.SyncClient {
		client.repo = repo
		return client
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"sync", "--repo", "StatPan/gira", "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"sync plan:", "would create", "bootstrap issues: 0 would create", "create label:"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("sync dry-run output missing %q:\n%s", want, stdout.String())
		}
	}
	if len(client.calls) != 0 {
		t.Fatalf("dry-run applied calls: %v", client.calls)
	}
}

func TestSyncDryRunWithBootstrapIssuesFlagIncludesIssueCreates(t *testing.T) {
	restoreClient := newSyncClient
	t.Cleanup(func() {
		newSyncClient = restoreClient
	})
	client := &cliFakeSyncClient{
		repo: mustCLIRepo(t, "StatPan/gira"),
		labels: []gira.ExistingLabel{
			{Name: gira.BootstrapLabel, Color: "5319E7", Description: "Created or managed by Gira bootstrap metadata sync."},
		},
	}
	newSyncClient = func(repo gira.RepoRef) gira.SyncClient {
		client.repo = repo
		return client
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"sync", "--repo", "StatPan/gira", "--dry-run", "--bootstrap-issues"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"sync plan:", "bootstrap issues:", "create issue:"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("sync dry-run output missing %q:\n%s", want, stdout.String())
		}
	}
	if len(client.calls) != 0 {
		t.Fatalf("dry-run applied calls: %v", client.calls)
	}
}

func TestSyncPolicyModeAdoptApplyPerformsNoMutations(t *testing.T) {
	restoreClient := newSyncClient
	t.Cleanup(func() {
		newSyncClient = restoreClient
	})
	client := &cliFakeSyncClient{repo: mustCLIRepo(t, "StatPan/gira")}
	newSyncClient = func(repo gira.RepoRef) gira.SyncClient {
		client.repo = repo
		return client
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"sync", "--repo", "StatPan/gira", "--policy-mode", "adopt"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "policy mode:      adopt") {
		t.Fatalf("sync output missing adopt policy mode:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "sync complete") {
		t.Fatalf("sync apply output missing completion:\n%s", stdout.String())
	}
	if len(client.calls) != 0 {
		t.Fatalf("adopt mode should not mutate, calls=%v", client.calls)
	}
}

func TestSyncDryRunNextStepPinsExplicitMergeWhenEnvEnforce(t *testing.T) {
	t.Setenv("GIRA_SYNC_POLICY_MODE", "enforce")
	restoreClient := newSyncClient
	t.Cleanup(func() {
		newSyncClient = restoreClient
	})
	client := &cliFakeSyncClient{repo: mustCLIRepo(t, "StatPan/gira")}
	newSyncClient = func(repo gira.RepoRef) gira.SyncClient {
		client.repo = repo
		return client
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"sync", "--repo", "StatPan/gira", "--dry-run", "--policy-mode", "merge"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "policy mode:      merge") {
		t.Fatalf("sync output missing reviewed merge policy mode:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "next step: gira ops sync --repo StatPan/gira --policy-mode merge") {
		t.Fatalf("next step must pin explicit merge policy mode:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "next step: gira ops sync --repo StatPan/gira --policy-mode enforce") {
		t.Fatalf("next step would apply env enforce policy:\n%s", stdout.String())
	}
	if len(client.calls) != 0 {
		t.Fatalf("dry-run applied calls: %v", client.calls)
	}
}

func TestSyncPolicyModeRejectsInvalidValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"sync", "--repo", "StatPan/gira", "--dry-run", "--policy-mode", "bad"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "invalid sync policy mode") {
		t.Fatalf("stderr missing invalid policy mode error:\n%s", stderr.String())
	}
}

func TestSyncApplyUsesInjectedClientAndPrintsComplete(t *testing.T) {
	restoreClient := newSyncClient
	t.Cleanup(func() {
		newSyncClient = restoreClient
	})
	client := &cliFakeSyncClient{
		repo: mustCLIRepo(t, "StatPan/gira"),
		labels: []gira.ExistingLabel{
			{Name: gira.BootstrapLabel, Color: "000000", Description: "Old description."},
		},
		milestones: []gira.ExistingMilestone{
			{Number: 1, Title: "MVP", Description: "CLI-first Gira bootstrapper with templates and GitHub metadata sync."},
			{Number: 2, Title: "Beta", Description: "Broader validation and hardening after the MVP workflow is usable."},
			{Number: 3, Title: "v1", Description: "Stable first release of the GitHub-native project OS workflow."},
		},
		issues: []gira.ExistingIssue{
			{Number: 1, Title: "[Epic] Gira MVP: GitHub-as-OS bootstrap", Labels: []string{gira.BootstrapLabel}},
			{Number: 2, Title: "[Task] Slice 1: CLI skeleton + template dry-run", Labels: []string{gira.BootstrapLabel}},
			{Number: 3, Title: "[Task] Slice 2: idempotent repo file install", Labels: []string{gira.BootstrapLabel}},
			{Number: 4, Title: "[Task] Slice 3: labels/milestones/bootstrap-issues sync", Labels: []string{gira.BootstrapLabel}},
			{Number: 5, Title: "[Task] Slice 4: gira status (text + --json)", Labels: []string{gira.BootstrapLabel}},
		},
	}
	newSyncClient = func(repo gira.RepoRef) gira.SyncClient {
		client.repo = repo
		return client
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"sync", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "sync complete") {
		t.Fatalf("sync apply output missing completion:\n%s", stdout.String())
	}
	if len(client.calls) == 0 || client.calls[0] != "update label "+gira.BootstrapLabel {
		t.Fatalf("apply calls = %v, want first call to update bootstrap label", client.calls)
	}
}

func TestExportDashboardCommandRequiresRepoOrConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"export", "dashboard", "--dry-run"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "--repo or --config is required") {
		t.Fatalf("stderr missing repo/config requirement:\n%s", stderr.String())
	}
}

func TestReportWBSCommandRendersCSV(t *testing.T) {
	restoreClient, restoreNow := newWBSReportClient, reportNow
	t.Cleanup(func() {
		newWBSReportClient = restoreClient
		reportNow = restoreNow
	})
	newWBSReportClient = func(repo gira.RepoRef) gira.WBSReportClient {
		return &cliFakeWBSReportClient{
			issues: []gira.WBSRawIssue{
				{IssueNumber: 10, Title: "Planning epic", State: "open", Body: "Tracks #11", Labels: []string{"type:epic", "status:ready"}, Milestone: "M1"},
				{IssueNumber: 11, Title: "Schedule table", State: "open", Labels: []string{"type:task", "status:ready"}, Milestone: "M1"},
			},
			milestones: []gira.DashboardRawMilestone{{MilestoneNumber: 1, Title: "M1", State: "open", DueOn: strPtr("2026-07-01T00:00:00Z")}},
		}
	}
	reportNow = func() time.Time {
		return time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"report", "wbs", "--repo", "StatPan/gira", "--csv"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "wbs_id,parent_id,level,kind,repo,issue,title,state,status,priority,owner,milestone,start_date,target_date,progress,children,source,url\n") {
		t.Fatalf("stdout missing CSV header:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "1.1,1,2,task,StatPan/gira,11,Schedule table") {
		t.Fatalf("stdout missing child row:\n%s", stdout.String())
	}
}

func TestReportWBSCommandRendersExecutionCSV(t *testing.T) {
	restoreClient, restoreNow := newWBSReportClient, reportNow
	t.Cleanup(func() {
		newWBSReportClient = restoreClient
		reportNow = restoreNow
	})
	newWBSReportClient = func(repo gira.RepoRef) gira.WBSReportClient {
		return &cliFakeWBSReportClient{
			issues: []gira.WBSRawIssue{
				{IssueNumber: 10, Title: "Planning epic", State: "open", Body: "Tracks #11", Labels: []string{"type:epic", "status:ready"}, Milestone: "M1"},
				{IssueNumber: 11, Title: "Schedule table", State: "open", Body: "Depends on #9", Labels: []string{"type:task", "status:ready", "area:backend", "owner:kim"}, Milestone: "M1"},
			},
			milestones: []gira.DashboardRawMilestone{{MilestoneNumber: 1, Title: "M1", State: "open", DueOn: strPtr("2026-07-01T00:00:00Z")}},
		}
	}
	reportNow = func() time.Time {
		return time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"report", "wbs", "--repo", "StatPan/gira", "--mode", "execution", "--format", "csv"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "phase,workstream,task,owner,start_date,due_date,status,priority,dependency,milestone,issue_url") {
		t.Fatalf("stdout missing execution CSV header:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "M1,backend,Schedule table,kim,,2026-07-01,ready,,#9,M1") {
		t.Fatalf("stdout missing execution row:\n%s", stdout.String())
	}
}

func TestReportScheduleCommandRendersScenarioJSON(t *testing.T) {
	restoreClient, restoreNow := newWBSReportClient, reportNow
	t.Cleanup(func() {
		newWBSReportClient = restoreClient
		reportNow = restoreNow
	})
	newWBSReportClient = func(repo gira.RepoRef) gira.WBSReportClient {
		return &cliFakeWBSReportClient{
			issues: []gira.WBSRawIssue{
				{IssueNumber: 11, Title: "Schedule table", State: "open", Labels: []string{"type:task", "status:ready"}, Milestone: "M1"},
			},
			milestones: []gira.DashboardRawMilestone{{MilestoneNumber: 1, Title: "M1", State: "open", DueOn: strPtr("2026-07-01T00:00:00Z")}},
		}
	}
	reportNow = func() time.Time {
		return time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"report", "schedule", "--repo", "StatPan/gira", "--scenario", "one-month", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"command": "report schedule"`) || !strings.Contains(stdout.String(), `"scenario": "one-month"`) || !strings.Contains(stdout.String(), `"scenario_due_date": "2026-06-18"`) {
		t.Fatalf("stdout missing schedule scenario JSON:\n%s", stdout.String())
	}
}

func TestReportWBSCommandWritesBundle(t *testing.T) {
	restoreClient, restoreNow := newWBSReportClient, reportNow
	t.Cleanup(func() {
		newWBSReportClient = restoreClient
		reportNow = restoreNow
	})
	newWBSReportClient = func(repo gira.RepoRef) gira.WBSReportClient {
		return &cliFakeWBSReportClient{
			issues: []gira.WBSRawIssue{
				{IssueNumber: 10, Title: "Planning epic", State: "open", Labels: []string{"type:epic", "status:ready"}},
			},
		}
	}
	reportNow = func() time.Time {
		return time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	}

	outputRoot := filepath.Join(t.TempDir(), "wbs")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"report", "wbs", "--repo", "StatPan/gira", "--format", "bundle", "--output", outputRoot}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "wbs report bundle written") {
		t.Fatalf("stdout missing bundle confirmation:\n%s", stdout.String())
	}
	for _, rel := range []string{"index.html", "csv/wbs_items.csv", "derived/wbs_tree.json"} {
		if _, err := os.Stat(filepath.Join(outputRoot, rel)); err != nil {
			t.Fatalf("expected artifact %s: %v", rel, err)
		}
	}
}

func TestExportDashboardCommandRejectsInvalidRepoFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"export", "dashboard", "--repo", "bad-format", "--dry-run"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "OWNER/REPO") {
		t.Fatalf("stderr missing parse error for invalid repo:\n%s", stderr.String())
	}
}

func TestExportDashboardCommandRejectsPositionalArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"export", "dashboard", "--repo", "StatPan/gira", "--dry-run", "unexpected"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "unexpected argument") {
		t.Fatalf("stderr missing unexpected argument error:\n%s", stderr.String())
	}
}

func TestExportDashboardDryRunRequiresDeterministicOutput(t *testing.T) {
	restoreClient, restoreNow := newDashboardExportClient, dashboardExportNow
	t.Cleanup(func() {
		newDashboardExportClient = restoreClient
		dashboardExportNow = restoreNow
	})
	client := &cliFakeDashboardExportClient{
		repo: mustCLIRepo(t, "StatPan/gira"),
		issues: []gira.DashboardRawIssue{
			{IssueNumber: 2, Title: "Second", State: "open", Labels: []string{"status:ready"}},
			{IssueNumber: 1, Title: "First", State: "open", Labels: []string{"status:blocked"}},
		},
		pulls: []gira.DashboardRawPullRequest{
			{PullRequestNumber: 11, Title: "PR", State: "closed", Labels: []string{"priority:p1"}},
		},
		milestones: []gira.DashboardRawMilestone{
			{MilestoneNumber: 3, Title: "Later", State: "closed"},
			{MilestoneNumber: 1, Title: "First", State: "open"},
		},
		projectSync: gira.ProjectSyncSnapshot{
			RoadmapItems: []gira.ProjectRoadmapItem{
				{IssueNumber: 10, IssueTitle: "Roadmap", TypeLabel: "type:epic", IssueURL: "https://github.com/StatPan/gira/issues/10", Roadmapable: true, StartDate: strPtr("2026-04-10"), TargetDate: strPtr("2026-04-20")},
				{IssueNumber: 9, IssueTitle: "Another", TypeLabel: "type:task", IssueURL: "https://github.com/StatPan/gira/issues/9", Roadmapable: false},
			},
		},
		capabilities: gira.ProjectCapabilityReport{
			Capabilities:   map[string]gira.ProjectCapabilityStatus{},
			BlockedActions: []gira.ProjectCapabilityBlock{},
		},
	}
	newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient {
		client.repo = repo
		return client
	}
	dashboardExportNow = func() time.Time {
		return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	}

	var firstOut, firstErr, secondOut, secondErr bytes.Buffer
	firstCode := Run([]string{"export", "dashboard", "--repo", "StatPan/gira", "--dry-run"}, &firstOut, &firstErr)
	secondCode := Run([]string{"export", "dashboard", "--repo", "StatPan/gira", "--dry-run"}, &secondOut, &secondErr)

	if firstCode != 0 || secondCode != 0 {
		t.Fatalf("exit codes = %d/%d, want 0/0; stderr: %s%s", firstCode, secondCode, firstErr.String(), secondErr.String())
	}
	if firstOut.String() != secondOut.String() {
		t.Fatalf("dry-run output changed between identical runs")
	}
	if !strings.Contains(firstOut.String(), "export dashboard plan:") {
		t.Fatalf("dry-run output missing plan header:\n%s", firstOut.String())
	}
}

func TestExportDashboardJSONOnlyStdout(t *testing.T) {
	restoreClient, restoreNow := newDashboardExportClient, dashboardExportNow
	t.Cleanup(func() {
		newDashboardExportClient = restoreClient
		dashboardExportNow = restoreNow
	})
	client := &cliFakeDashboardExportClient{
		repo: mustCLIRepo(t, "StatPan/gira"),
		capabilities: gira.ProjectCapabilityReport{
			Capabilities:   map[string]gira.ProjectCapabilityStatus{},
			BlockedActions: []gira.ProjectCapabilityBlock{},
		},
	}
	newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient {
		client.repo = repo
		return client
	}
	dashboardExportNow = func() time.Time {
		return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"export",
		"dashboard",
		"--repo",
		"StatPan/gira",
		"--dry-run",
		"--json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr not empty in json mode:\n%s", stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("JSON output parse failure: %v\n%s", err, stdout.String())
	}
	if payload["command"] != "export dashboard" {
		t.Fatalf("payload missing command field: %v", payload["command"])
	}
}

func TestExportDashboardApplyWritesArtifactsAndJsonOnlyStdout(t *testing.T) {
	restoreClient, restoreNow := newDashboardExportClient, dashboardExportNow
	t.Cleanup(func() {
		newDashboardExportClient = restoreClient
		dashboardExportNow = restoreNow
	})
	client := &cliFakeDashboardExportClient{
		repo: mustCLIRepo(t, "StatPan/gira"),
		milestones: []gira.DashboardRawMilestone{
			{MilestoneNumber: 1, Title: "MVP", State: "open"},
		},
		capabilities: gira.ProjectCapabilityReport{
			Capabilities: map[string]gira.ProjectCapabilityStatus{
				"issues:read": "allowed",
			},
		},
	}
	newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient {
		client.repo = repo
		return client
	}
	dashboardExportNow = func() time.Time {
		return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	}

	outputRoot := filepath.Join(t.TempDir(), "dashboard-output")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"export",
		"dashboard",
		"--repo",
		"StatPan/gira",
		"--output",
		outputRoot,
		"--json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr not empty in json mode:\n%s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"command\": \"export dashboard\"") {
		t.Fatalf("stdout missing plan JSON:\n%s", stdout.String())
	}

	expected := []string{
		"manifest.json",
		"raw/github.json",
		"raw/transitions.json",
		"raw/capabilities.json",
		"derived/execution_board.json",
		"derived/roadmap_timeline.json",
		"derived/warnings.json",
		"csv/execution_items.csv",
		"csv/roadmap_items.csv",
	}
	for _, relativePath := range expected {
		if _, err := os.Stat(filepath.Join(outputRoot, relativePath)); err != nil {
			t.Fatalf("expected exported file %q: %v", relativePath, err)
		}
	}
}

func TestReportReleaseNotesCommandRendersMarkdown(t *testing.T) {
	restoreClient, restoreNow := newReleaseNotesClient, reportNow
	t.Cleanup(func() {
		newReleaseNotesClient = restoreClient
		reportNow = restoreNow
	})
	newReleaseNotesClient = func(repo gira.RepoRef) gira.ReleaseNotesClient {
		return &cliFakeReleaseNotesClient{
			issues: []gira.ReleaseNotesIssue{
				{Number: 10, Title: "Add export button", State: "closed", Labels: []string{"type:story"}, Milestone: "v2.1.0", URL: "u10"},
			},
			prs:        []gira.ReleaseNotesPullRequest{{Number: 100, Title: "Export button", Body: "Closes #10", URL: "p100"}},
			milestones: []gira.DashboardRawMilestone{{MilestoneNumber: 1, Title: "v2.1.0", State: "closed"}},
		}
	}
	reportNow = func() time.Time {
		return time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"report", "release-notes", "--repo", "StatPan/gira", "--milestone", "v2.1.0", "--md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "# Release Notes: v2.1.0") || !strings.Contains(stdout.String(), "Add export button (#10 via PR #100)") {
		t.Fatalf("stdout missing markdown release notes:\n%s", stdout.String())
	}
}

func TestReportReleaseNotesCommandRequiresMilestone(t *testing.T) {
	restoreClient := newReleaseNotesClient
	t.Cleanup(func() {
		newReleaseNotesClient = restoreClient
	})
	newReleaseNotesClient = func(repo gira.RepoRef) gira.ReleaseNotesClient {
		return &cliFakeReleaseNotesClient{}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"report", "release-notes", "--repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "--milestone is required") {
		t.Fatalf("stderr missing milestone requirement:\n%s", stderr.String())
	}
}

func TestReportReleaseNotesCommandWritesBundle(t *testing.T) {
	restoreClient, restoreNow := newReleaseNotesClient, reportNow
	t.Cleanup(func() {
		newReleaseNotesClient = restoreClient
		reportNow = restoreNow
	})
	newReleaseNotesClient = func(repo gira.RepoRef) gira.ReleaseNotesClient {
		return &cliFakeReleaseNotesClient{
			issues:     []gira.ReleaseNotesIssue{{Number: 10, Title: "Add export button", State: "closed", Labels: []string{"type:story"}, Milestone: "v2.1.0"}},
			prs:        []gira.ReleaseNotesPullRequest{{Number: 100, Body: "Closes #10"}},
			milestones: []gira.DashboardRawMilestone{{MilestoneNumber: 1, Title: "v2.1.0", State: "closed"}},
		}
	}
	reportNow = func() time.Time {
		return time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	}

	outputRoot := filepath.Join(t.TempDir(), "release-notes")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"report", "release-notes", "--repo", "StatPan/gira", "--milestone", "v2.1.0", "--format", "bundle", "--output", outputRoot}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "release notes bundle written") {
		t.Fatalf("stdout missing bundle confirmation:\n%s", stdout.String())
	}
	for _, rel := range []string{"index.html", "release-notes.md", "derived/release_notes.json", "csv/release_items.csv"} {
		if _, err := os.Stat(filepath.Join(outputRoot, rel)); err != nil {
			t.Fatalf("expected artifact %s: %v", rel, err)
		}
	}
}

func TestExportDashboardWorkspaceDryRunJSON(t *testing.T) {
	restoreExport, restoreNow := newWorkspaceDashboardExportBundle, dashboardExportNow
	t.Cleanup(func() {
		newWorkspaceDashboardExportBundle = restoreExport
		dashboardExportNow = restoreNow
	})
	dashboardExportNow = func() time.Time {
		return time.Date(2026, 5, 31, 9, 0, 0, 0, time.UTC)
	}
	var capturedConfig string
	var capturedDryRun bool
	var capturedOptions gira.WorkspaceStatusOptions
	newWorkspaceDashboardExportBundle = func(configPath string, outputRoot string, snapshotAt time.Time, dryRun bool, options gira.WorkspaceStatusOptions) (gira.DashboardExportPlan, gira.DashboardExportBundle, error) {
		capturedConfig = configPath
		capturedDryRun = dryRun
		capturedOptions = options
		workspace := gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"}
		return gira.DashboardExportPlan{
			Command:       "export dashboard",
			DryRun:        dryRun,
			Workspace:     &workspace,
			OutputRoot:    outputRoot,
			SchemaVersion: gira.DashboardExportSchemaVersion,
			SnapshotAt:    "2026-05-31T09:00:00Z",
			Sources:       []gira.DashboardExportSource{{Name: "workspace_status", Included: true}},
			Artifacts:     gira.DashboardExportWorkspaceArtifacts(),
			Counts:        gira.DashboardExportCounts{WorkspaceRepos: 1, WorkspaceQueueItems: 1},
		}, gira.DashboardExportBundle{}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"export",
		"dashboard",
		"--config",
		".gira/config.yaml",
		"--repo",
		"StatPan/gira",
		"--limit",
		"1",
		"--active-only",
		"--cache-ttl",
		"0s",
		"--dry-run",
		"--json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr not empty in json mode:\n%s", stderr.String())
	}
	if capturedConfig != ".gira/config.yaml" || !capturedDryRun {
		t.Fatalf("workspace export call config=%q dry=%t", capturedConfig, capturedDryRun)
	}
	if len(capturedOptions.Repos) != 1 || capturedOptions.Repos[0].FullName() != "StatPan/gira" || capturedOptions.Limit != 1 || !capturedOptions.ActiveOnly || capturedOptions.CacheTTL != 0 {
		t.Fatalf("workspace options = %+v", capturedOptions)
	}
	var payload struct {
		Workspace *gira.WorkspaceSummary     `json:"workspace"`
		Counts    gira.DashboardExportCounts `json:"counts"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("JSON output parse failure: %v\n%s", err, stdout.String())
	}
	if payload.Workspace == nil || payload.Workspace.Name != "personal" || payload.Counts.WorkspaceQueueItems != 1 {
		t.Fatalf("workspace payload mismatch: %+v", payload)
	}
}

func TestExportDashboardWorkspaceApplyWritesArtifacts(t *testing.T) {
	restoreExport, restoreNow := newWorkspaceDashboardExportBundle, dashboardExportNow
	t.Cleanup(func() {
		newWorkspaceDashboardExportBundle = restoreExport
		dashboardExportNow = restoreNow
	})
	dashboardExportNow = func() time.Time {
		return time.Date(2026, 5, 31, 9, 0, 0, 0, time.UTC)
	}
	newWorkspaceDashboardExportBundle = func(configPath string, outputRoot string, snapshotAt time.Time, dryRun bool, options gira.WorkspaceStatusOptions) (gira.DashboardExportPlan, gira.DashboardExportBundle, error) {
		workspace := gira.WorkspaceSummary{Name: "personal", Owner: "StatPan"}
		queues := gira.BuildWorkspaceQueues(workspace, []gira.WorkStatusResult{
			{Repo: "StatPan/gira", Issue: 10, Title: "Ready issue", State: "open", Status: "Ready", Labels: []string{"status:ready"}},
		})
		dashboard := gira.DashboardWorkspaceDashboard{
			SchemaVersion: gira.WorkspaceDashboardSchemaVersion,
			SnapshotAt:    "2026-05-31T09:00:00Z",
			Workspace:     workspace,
			Source:        gira.DashboardWorkspaceSource{Contract: gira.WorkspaceStatusSourceContract, Path: "raw/workspace_status.json"},
			QueueCounts:   queues.Counts,
			TopActions: []gira.DashboardWorkspaceTopAction{
				{Queue: "agent_ready", Repo: "StatPan/gira", Issue: 10, Title: "Ready issue", URL: "https://github.com/StatPan/gira/issues/10", ReasonCodes: []string{"ticket_ready"}, NextSafeCommand: "gira ticket start --repo StatPan/gira --ticket 10 --apply"},
			},
			Artifacts: gira.DashboardWorkspaceArtifacts{WorkspaceStatus: "raw/workspace_status.json", WorkspaceQueues: "derived/workspace_queues.json", QueueItemsCSV: "csv/workspace_queue_items.csv"},
		}
		report := gira.WorkspaceReport{Workspace: workspace, Queues: queues, FetchedAt: "2026-05-31T09:00:00Z"}
		return gira.DashboardExportPlan{
				Command:       "export dashboard",
				DryRun:        dryRun,
				Workspace:     &workspace,
				OutputRoot:    outputRoot,
				SchemaVersion: gira.DashboardExportSchemaVersion,
				SnapshotAt:    "2026-05-31T09:00:00Z",
				Artifacts:     gira.DashboardExportWorkspaceArtifacts(),
				Counts:        gira.DashboardExportCounts{WorkspaceRepos: 1, WorkspaceQueueItems: 1},
			}, gira.DashboardExportBundle{
				Manifest:           gira.DashboardExportManifest{SchemaVersion: gira.DashboardExportSchemaVersion, SnapshotAt: "2026-05-31T09:00:00Z", Workspace: &workspace, Artifacts: gira.DashboardExportWorkspaceArtifacts()},
				WorkspaceStatus:    &report,
				WorkspaceQueues:    &queues,
				WorkspaceDashboard: &dashboard,
			}, nil
	}

	outputRoot := filepath.Join(t.TempDir(), "workspace-dashboard")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"export",
		"dashboard",
		"--config",
		".gira/config.yaml",
		"--output",
		outputRoot,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr not empty:\n%s", stderr.String())
	}
	for _, relativePath := range []string{"raw/workspace_status.json", "derived/workspace_queues.json", "derived/workspace_dashboard.json", "csv/workspace_queue_items.csv", "index.html"} {
		if _, err := os.Stat(filepath.Join(outputRoot, relativePath)); err != nil {
			t.Fatalf("expected workspace artifact %q: %v", relativePath, err)
		}
	}
	if !strings.Contains(stdout.String(), "export dashboard artifacts written") {
		t.Fatalf("stdout missing write summary:\n%s", stdout.String())
	}
}

func TestExportDashboardApplyRejectsOutputFilePath(t *testing.T) {
	restoreClient := newDashboardExportClient
	t.Cleanup(func() { newDashboardExportClient = restoreClient })
	file, err := os.CreateTemp(t.TempDir(), "dashboard-output")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient {
		return &cliFakeDashboardExportClient{repo: repo}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"export",
		"dashboard",
		"--repo",
		"StatPan/gira",
		"--output",
		file.Name(),
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "output path exists but is not a directory") {
		t.Fatalf("stderr missing output path type error:\n%s", stderr.String())
	}
}

func TestStatusTextUsesInjectedClient(t *testing.T) {
	restoreClient, restoreNow := newStatusClient, statusNow
	t.Cleanup(func() {
		newStatusClient = restoreClient
		statusNow = restoreNow
	})
	newStatusClient = func(repo gira.RepoRef) gira.StatusClient {
		return cliFakeStatusClient{
			repo: repo,
			responses: map[string]string{
				"api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100": `[]`,
				"api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100":     `[[{"number":1,"title":"Issue 1","state":"open","labels":[{"name":"status:blocked"}],"milestone":null,"updated_at":"2026-04-01T12:00:00Z","html_url":"https://github.com/StatPan/gira/issues/1"}]]`,
			},
		}
	}
	statusNow = func() time.Time {
		return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"status", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"status: StatPan/gira", "milestone progress: none", "stale open issues: 1", "blocked issues: 1"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("status output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestStatusAllUsesWorkspaceRepos(t *testing.T) {
	restoreClient, restoreNow := newStatusClient, statusNow
	t.Cleanup(func() {
		newStatusClient = restoreClient
		statusNow = restoreNow
	})
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("workspace:\n  name: personal\n  inbox_repo: StatPan/inbox\n  repos:\n    - StatPan/app-a\n    - StatPan/app-b\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	newStatusClient = func(repo gira.RepoRef) gira.StatusClient {
		return cliFakeStatusClient{repo: repo, responses: cliStatusResponses(repo.FullName())}
	}
	statusNow = func() time.Time { return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC) }

	var stdout, stderr bytes.Buffer
	code := Run([]string{"status", "--all", "--config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"status: workspace personal", "repos: 2 checked, 0 failed", "StatPan/app-a", "StatPan/app-b", "Repository"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("status --all output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestStatusOwnerJSONUsesRepoDiscovery(t *testing.T) {
	restoreClient, restoreList, restoreNow := newStatusClient, listStatusReposForOwner, statusNow
	t.Cleanup(func() {
		newStatusClient = restoreClient
		listStatusReposForOwner = restoreList
		statusNow = restoreNow
	})
	listStatusReposForOwner = func(owner string, limit int, includeArchived bool) ([]gira.RepoRef, error) {
		if owner != "StatPan" || limit != 25 || !includeArchived {
			t.Fatalf("unexpected discovery args owner=%q limit=%d includeArchived=%v", owner, limit, includeArchived)
		}
		return []gira.RepoRef{mustCLIRepo(t, "StatPan/app-b"), mustCLIRepo(t, "StatPan/app-a")}, nil
	}
	newStatusClient = func(repo gira.RepoRef) gira.StatusClient {
		return cliFakeStatusClient{repo: repo, responses: cliStatusResponses(repo.FullName())}
	}
	statusNow = func() time.Time { return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC) }

	var stdout, stderr bytes.Buffer
	code := Run([]string{"status", "--owner", "StatPan", "--limit", "25", "--include-archived", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.GlobalStatusReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode global status JSON: %v\n%s", err, stdout.String())
	}
	if report.Scope != "owner StatPan" || report.Counts.Repos != 2 || report.Counts.OpenIssues != 2 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Repos[0].Repo != "StatPan/app-a" {
		t.Fatalf("repos should be sorted deterministically: %+v", report.Repos)
	}
}

func TestStatusOwnerDiscoveryUsesArchivedFlags(t *testing.T) {
	runner := devCLIRunner{outputs: map[string][]byte{
		"gh repo list StatPan --limit 2 --json nameWithOwner,isArchived --no-archived": []byte(`[
			{"nameWithOwner":"StatPan/app","isArchived":false},
			{"nameWithOwner":"StatPan/old","isArchived":true}
		]`),
		"gh repo list StatPan --limit 2 --json nameWithOwner,isArchived --archived": []byte(`[
			{"nameWithOwner":"StatPan/archive","isArchived":true},
			{"nameWithOwner":"StatPan/live","isArchived":false}
		]`),
	}}

	repos, err := ghStatusReposForOwner("StatPan", 2, false, runner)
	if err != nil {
		t.Fatalf("ghStatusReposForOwner error: %v", err)
	}
	if len(repos) != 1 || repos[0].FullName() != "StatPan/app" {
		t.Fatalf("non-archived discovery should use --no-archived and ignore archived rows: %+v", repos)
	}

	repos, err = ghStatusReposForOwner("StatPan", 2, true, runner)
	if err != nil {
		t.Fatalf("ghStatusReposForOwner include archived error: %v", err)
	}
	got := make([]string, 0, len(repos))
	for _, repo := range repos {
		got = append(got, repo.FullName())
	}
	if strings.Join(got, ",") != "StatPan/app,StatPan/archive" {
		t.Fatalf("include archived discovery should merge non-archived and archived rows within limit, got %v", got)
	}
}

func TestStatusRejectsMultipleRepoModes(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"status", "--repo", "StatPan/gira", "--all"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "choose only one of --repo, --all, or --owner") {
		t.Fatalf("stderr missing mode error:\n%s", stderr.String())
	}
}

func TestStatusInfersRepoFromGitOrigin(t *testing.T) {
	restoreClient, restoreRunner, restoreNow := newStatusClient, repoContextRunner, statusNow
	t.Cleanup(func() {
		newStatusClient = restoreClient
		repoContextRunner = restoreRunner
		statusNow = restoreNow
	})
	repoContextRunner = devCLIRunner{outputs: map[string][]byte{
		"git remote get-url origin": []byte("git@github.com:StatPan/gira.git\n"),
	}}
	newStatusClient = func(repo gira.RepoRef) gira.StatusClient {
		if repo.FullName() != "StatPan/gira" {
			t.Fatalf("repo = %s, want StatPan/gira", repo.FullName())
		}
		return cliFakeStatusClient{
			repo: repo,
			responses: map[string]string{
				"api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100": `[]`,
				"api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100":     `[]`,
			},
		}
	}
	statusNow = func() time.Time { return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC) }

	var stdout, stderr bytes.Buffer
	code := Run([]string{"status", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"repo": "StatPan/gira"`) {
		t.Fatalf("stdout missing inferred repo:\n%s", stdout.String())
	}
}

func TestStatusRepoOverrideWinsOverContext(t *testing.T) {
	restoreClient, restoreRunner, restoreNow := newStatusClient, repoContextRunner, statusNow
	t.Cleanup(func() {
		newStatusClient = restoreClient
		repoContextRunner = restoreRunner
		statusNow = restoreNow
	})
	repoContextRunner = devCLIRunner{errs: map[string]error{
		"git remote get-url origin": fmt.Errorf("repo context runner should not be called"),
	}}
	newStatusClient = func(repo gira.RepoRef) gira.StatusClient {
		if repo.FullName() != "StatPan/override" {
			t.Fatalf("repo = %s, want StatPan/override", repo.FullName())
		}
		return cliFakeStatusClient{
			repo: repo,
			responses: map[string]string{
				"api repos/StatPan/override/milestones --paginate --slurp -X GET -f state=all -f per_page=100": `[]`,
				"api repos/StatPan/override/issues --paginate --slurp -X GET -f state=all -f per_page=100":     `[]`,
			},
		}
	}
	statusNow = func() time.Time { return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC) }

	var stdout, stderr bytes.Buffer
	code := Run([]string{"status", "--repo", "StatPan/override", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"repo": "StatPan/override"`) {
		t.Fatalf("stdout missing override repo:\n%s", stdout.String())
	}
}

func TestStatusMissingRepoContextReturnsRemediation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	restoreRunner := repoContextRunner
	t.Cleanup(func() { repoContextRunner = restoreRunner })
	repoContextRunner = devCLIRunner{errs: map[string]error{
		"git remote get-url origin": fmt.Errorf("exit status 2"),
	}}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"status"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "pass --repo OWNER/REPO") {
		t.Fatalf("stderr missing repo context remediation:\n%s", stderr.String())
	}
}

func TestOnboardVerifyInfersRepoFromConfig(t *testing.T) {
	restoreBuilder, restoreRunner := newOnboardVerifyReport, repoContextRunner
	t.Cleanup(func() {
		newOnboardVerifyReport = restoreBuilder
		repoContextRunner = restoreRunner
	})
	withCLIRepoConfig(t, "StatPan/configured")
	repoContextRunner = devCLIRunner{errs: map[string]error{
		"git remote get-url origin": fmt.Errorf("config should win before origin"),
	}}
	newOnboardVerifyReport = func(repo gira.RepoRef, stage gira.OnboardStage) (gira.OnboardVerifyReport, error) {
		if repo.FullName() != "StatPan/configured" {
			t.Fatalf("repo = %s, want StatPan/configured", repo.FullName())
		}
		return gira.OnboardVerifyReport{Repo: repo.FullName(), Command: "onboard verify", Stage: string(stage), Ready: true}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"onboard", "verify", "--stage", "init", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"repo": "StatPan/configured"`) {
		t.Fatalf("stdout missing configured repo:\n%s", stdout.String())
	}
}

func TestOnboardVerifyRepoOverrideWinsOverConfig(t *testing.T) {
	restoreBuilder, restoreRunner := newOnboardVerifyReport, repoContextRunner
	t.Cleanup(func() {
		newOnboardVerifyReport = restoreBuilder
		repoContextRunner = restoreRunner
	})
	withCLIRepoConfig(t, "StatPan/configured")
	repoContextRunner = devCLIRunner{errs: map[string]error{
		"git remote get-url origin": fmt.Errorf("override should not call origin"),
	}}
	newOnboardVerifyReport = func(repo gira.RepoRef, stage gira.OnboardStage) (gira.OnboardVerifyReport, error) {
		if repo.FullName() != "StatPan/override" {
			t.Fatalf("repo = %s, want StatPan/override", repo.FullName())
		}
		return gira.OnboardVerifyReport{Repo: repo.FullName(), Command: "onboard verify", Stage: string(stage), Ready: true}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"onboard", "verify", "--repo", "StatPan/override", "--stage", "init", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"repo": "StatPan/override"`) {
		t.Fatalf("stdout missing override repo:\n%s", stdout.String())
	}
}

func TestReportWeeklyInfersRepoFromConfig(t *testing.T) {
	restoreDash, restoreReview, restoreRunner, restoreNow := newDashboardExportClient, newReviewGateClient, repoContextRunner, reportNow
	t.Cleanup(func() {
		newDashboardExportClient = restoreDash
		newReviewGateClient = restoreReview
		repoContextRunner = restoreRunner
		reportNow = restoreNow
	})
	withCLIRepoConfig(t, "StatPan/configured")
	repoContextRunner = devCLIRunner{errs: map[string]error{
		"git remote get-url origin": fmt.Errorf("config should win before origin"),
	}}
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	reportNow = func() time.Time { return now }
	newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient {
		if repo.FullName() != "StatPan/configured" {
			t.Fatalf("dashboard repo = %s, want StatPan/configured", repo.FullName())
		}
		return weeklyDashClient{repo: repo}
	}
	newReviewGateClient = func(repo gira.RepoRef) gira.ReviewGateClient {
		if repo.FullName() != "StatPan/configured" {
			t.Fatalf("review repo = %s, want StatPan/configured", repo.FullName())
		}
		return weeklyReviewClient{repo: repo}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"report", "weekly", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"repo": "StatPan/configured"`) {
		t.Fatalf("stdout missing configured repo:\n%s", stdout.String())
	}
}

func TestReportWeeklyRepoOverrideWinsOverConfig(t *testing.T) {
	restoreDash, restoreReview, restoreRunner, restoreNow := newDashboardExportClient, newReviewGateClient, repoContextRunner, reportNow
	t.Cleanup(func() {
		newDashboardExportClient = restoreDash
		newReviewGateClient = restoreReview
		repoContextRunner = restoreRunner
		reportNow = restoreNow
	})
	withCLIRepoConfig(t, "StatPan/configured")
	repoContextRunner = devCLIRunner{errs: map[string]error{
		"git remote get-url origin": fmt.Errorf("override should not call origin"),
	}}
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	reportNow = func() time.Time { return now }
	newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient {
		if repo.FullName() != "StatPan/override" {
			t.Fatalf("dashboard repo = %s, want StatPan/override", repo.FullName())
		}
		return weeklyDashClient{repo: repo}
	}
	newReviewGateClient = func(repo gira.RepoRef) gira.ReviewGateClient {
		if repo.FullName() != "StatPan/override" {
			t.Fatalf("review repo = %s, want StatPan/override", repo.FullName())
		}
		return weeklyReviewClient{repo: repo}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"report", "weekly", "--repo", "StatPan/override", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"repo": "StatPan/override"`) {
		t.Fatalf("stdout missing override repo:\n%s", stdout.String())
	}
}

func withCLIRepoConfig(t *testing.T, repo string) {
	t.Helper()
	dir := t.TempDir()
	giraDir := filepath.Join(dir, ".gira")
	if err := os.MkdirAll(giraDir, 0o755); err != nil {
		t.Fatalf("mkdir .gira: %v", err)
	}
	if err := os.WriteFile(filepath.Join(giraDir, "config.yaml"), []byte("repo: "+repo+"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
}

type cliFakeStatusClient struct {
	repo      gira.RepoRef
	responses map[string]string
}

type cliFakeSyncClient struct {
	repo       gira.RepoRef
	labels     []gira.ExistingLabel
	milestones []gira.ExistingMilestone
	issues     []gira.ExistingIssue
	calls      []string
}

type cliFakeDashboardExportClient struct {
	repo         gira.RepoRef
	issues       []gira.DashboardRawIssue
	pulls        []gira.DashboardRawPullRequest
	milestones   []gira.DashboardRawMilestone
	projectSync  gira.ProjectSyncSnapshot
	transitions  gira.ProjectTransitionSnapshot
	capabilities gira.ProjectCapabilityReport
}

func (c *cliFakeDashboardExportClient) Repo() gira.RepoRef { return c.repo }

func (c *cliFakeDashboardExportClient) FetchIssues() ([]gira.DashboardRawIssue, error) {
	return c.issues, nil
}

func (c *cliFakeDashboardExportClient) FetchPullRequests() ([]gira.DashboardRawPullRequest, error) {
	return c.pulls, nil
}

func (c *cliFakeDashboardExportClient) FetchMilestones() ([]gira.DashboardRawMilestone, error) {
	return c.milestones, nil
}

func (c *cliFakeDashboardExportClient) FetchProjectSnapshot() (gira.ProjectSyncSnapshot, error) {
	return c.projectSync, nil
}

func (c *cliFakeDashboardExportClient) FetchTransitionSnapshot() (gira.ProjectTransitionSnapshot, error) {
	return c.transitions, nil
}

func (c *cliFakeDashboardExportClient) FetchCapabilities() (gira.ProjectCapabilityReport, error) {
	return c.capabilities, nil
}

func strPtr(value string) *string {
	return &value
}

func (c *cliFakeSyncClient) Repo() gira.RepoRef { return c.repo }

func (c *cliFakeSyncClient) ListLabels() ([]gira.ExistingLabel, error) {
	return c.labels, nil
}

func (c *cliFakeSyncClient) CreateLabel(label gira.LabelDef) error {
	c.calls = append(c.calls, "create label "+label.Name)
	return nil
}

func (c *cliFakeSyncClient) UpdateLabel(label gira.LabelDef) error {
	c.calls = append(c.calls, "update label "+label.Name)
	return nil
}

func (c *cliFakeSyncClient) ListMilestones() ([]gira.ExistingMilestone, error) {
	return c.milestones, nil
}

func (c *cliFakeSyncClient) CreateMilestone(milestone gira.MilestoneDef) error {
	c.calls = append(c.calls, "create milestone "+milestone.Title)
	return nil
}

func (c *cliFakeSyncClient) UpdateMilestone(number int, milestone gira.MilestoneDef) error {
	c.calls = append(c.calls, "update milestone "+milestone.Title)
	return nil
}

func (c *cliFakeSyncClient) ListBootstrapIssues() ([]gira.ExistingIssue, error) {
	return c.issues, nil
}

func (c *cliFakeSyncClient) CreateIssue(issue gira.BootstrapIssueDef) error {
	c.calls = append(c.calls, "create issue "+issue.Title)
	return nil
}

func mustCLIRepo(t *testing.T, value string) gira.RepoRef {
	t.Helper()
	repo, err := gira.ParseRepoRef(value)
	if err != nil {
		t.Fatalf("ParseRepoRef returned error: %v", err)
	}
	return repo
}

func runCLIGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	if _, err := (gira.ExecCommandRunner{}).Run("git", append([]string{"-C", repo}, args...)...); err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
}

func (c cliFakeStatusClient) Repo() gira.RepoRef {
	return c.repo
}

func (c cliFakeStatusClient) JSON(args []string, target any) error {
	key := strings.Join(args, " ")
	return json.Unmarshal([]byte(c.responses[key]), target)
}

func cliStatusResponses(repo string) map[string]string {
	return map[string]string{
		"api repos/" + repo + "/milestones --paginate --slurp -X GET -f state=all -f per_page=100":                              `[[{"number":1,"title":"MVP","state":"open","description":"","due_on":null,"open_issues":1,"closed_issues":1}]]`,
		"issue list --repo " + repo + " --state all --limit 1000 --json number,title,state,labels,milestone,updatedAt,url,body": `[{"number":1,"title":"Issue 1","state":"OPEN","labels":[],"milestone":{"title":"MVP"},"updatedAt":"2026-04-25T12:00:00Z","url":"https://github.com/` + repo + `/issues/1"}]`,
	}
}

func TestProjectCapabilityCommandRequiresRepo(t *testing.T) {
	restore := newProjectCapabilityReport
	t.Cleanup(func() { newProjectCapabilityReport = restore })
	newProjectCapabilityReport = func(repo gira.RepoRef) (gira.ProjectCapabilityReport, error) {
		report := gira.ProjectCapabilityReport{
			Repo:    repo.FullName(),
			Command: "project capability",
			Mode:    "write",
			Token:   gira.ProjectCapabilityTokenSummary{Kind: "pat", Identity: "alice"},
			Capabilities: map[string]gira.ProjectCapabilityStatus{
				"issues:read": "allowed",
			},
			BlockedActions: []gira.ProjectCapabilityBlock{},
		}
		return report, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"project", "capability", "--repo", "StatPan/example", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"repo\": \"StatPan/example\"") {
		t.Fatalf("project capability JSON missing repo: %s", stdout.String())
	}
}

func TestProjectCapabilityCommandNeedsSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"project", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "project capability") {
		t.Fatalf("project help output unexpected: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "project sync") {
		t.Fatalf("project help output missing sync command: %s", stdout.String())
	}
}

func TestParityJiraCommandJSONUsesInjectedBuilder(t *testing.T) {
	restore := newJiraParityReport
	t.Cleanup(func() { newJiraParityReport = restore })
	newJiraParityReport = func(repo gira.RepoRef) (gira.JiraParityReport, error) {
		return gira.JiraParityReport{
			Repo:    repo.FullName(),
			Command: "parity jira",
			Scores:  gira.JiraParityScores{Earned: 80, Total: 100, Pct: 80},
			Domains: []gira.JiraParityDomain{{Name: "visibility", Weight: 15, Pass: true, Evidence: []string{"gira status"}}},
			Ready:   false,
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"parity", "jira", "--repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"command\": \"parity jira\"") {
		t.Fatalf("parity jira JSON missing command: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "\"percent\": 80") {
		t.Fatalf("parity jira JSON missing percent score: %s", stdout.String())
	}
}

func TestProjectSyncCommandRequiresDryRunOrApply(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"project", "sync", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "exactly one of --dry-run or --apply is required") {
		t.Fatalf("stderr missing dry-run/apply requirement:\n%s", stderr.String())
	}
}

func TestProjectSyncCommandJSONUsesInjectedBuilder(t *testing.T) {
	restore := newProjectSyncReport
	t.Cleanup(func() { newProjectSyncReport = restore })
	newProjectSyncReport = func(repo gira.RepoRef, dryRun bool) (gira.ProjectSyncReport, error) {
		report := gira.ProjectSyncReport{
			Repo:           repo.FullName(),
			Command:        "project sync",
			Project:        "Product OS",
			DryRun:         dryRun,
			MissingProject: true,
			Counts: gira.ProjectSyncCounts{
				FieldsMissing: 6,
			},
		}
		return report, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"project", "sync", "--repo", "StatPan/gira", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"command\": \"project sync\"") {
		t.Fatalf("project sync JSON missing command: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "\"dry_run\": true") {
		t.Fatalf("project sync JSON missing dry_run true: %s", stdout.String())
	}
}

func TestProjectTransitionsCommandRequiresDryRunOrApply(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"project", "transitions", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "exactly one of --dry-run or --apply is required") {
		t.Fatalf("stderr missing dry-run/apply requirement:\n%s", stderr.String())
	}
}

func TestProjectTransitionsCommandJSONUsesInjectedBuilder(t *testing.T) {
	restore := newProjectTransitionsReport
	t.Cleanup(func() { newProjectTransitionsReport = restore })
	newProjectTransitionsReport = func(repo gira.RepoRef, dryRun bool) (gira.ProjectTransitionsReport, error) {
		return gira.ProjectTransitionsReport{
			Repo:    repo.FullName(),
			Command: "project transitions",
			DryRun:  dryRun,
			Counts: gira.ProjectTransitionCounts{
				Applied: 1,
			},
			Transitions: []gira.ProjectTransitionPlanItem{
				{RuleID: "issue_open_default", TargetType: "issue", TargetID: "10", Decision: "apply", From: "null", To: "Backlog"},
			},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"project", "transitions", "--repo", "StatPan/gira", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"command\": \"project transitions\"") {
		t.Fatalf("project transitions JSON missing command: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "\"dry_run\": true") {
		t.Fatalf("project transitions JSON missing dry_run true: %s", stdout.String())
	}
}

func TestProjectSyncApplyCommandJSONUsesInjectedBuilder(t *testing.T) {
	restore := newProjectSyncApplyReport
	t.Cleanup(func() { newProjectSyncApplyReport = restore })
	newProjectSyncApplyReport = func(repo gira.RepoRef) (gira.ProjectSyncApplyReport, error) {
		return gira.ProjectSyncApplyReport{
			Repo:    repo.FullName(),
			Command: "project sync",
			DryRun:  false,
			Applied: []gira.ProjectSyncApplyAction{{Action: "project_status_field:update", Required: "projectsv2:write", Result: "planned"}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"project", "sync", "--repo", "StatPan/gira", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"dry_run\": false") {
		t.Fatalf("project sync apply JSON missing dry_run false: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "\"action\": \"project_status_field:update\"") {
		t.Fatalf("project sync apply JSON missing action: %s", stdout.String())
	}
}

func TestAuditVerifyScopesToRepoInCLI(t *testing.T) {
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "StatPan_gira.jsonl")
	otherPath := filepath.Join(dir, "OtherOrg_other.jsonl")
	if err := gira.AppendAuditRecords(repoPath, []gira.AuditRecord{gira.NewAuditRecord("sync", "sha256:a", "label:create", "issue#1", "ok", "", "allowed", time.Now())}); err != nil {
		t.Fatalf("append repo audit: %v", err)
	}
	if err := gira.AppendAuditRecords(otherPath, []gira.AuditRecord{gira.NewAuditRecord("sync", "sha256:b", "label:create", "issue#2", "ok", "", "allowed", time.Now())}); err != nil {
		t.Fatalf("append other audit: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"audit", "verify", "--repo", "StatPan/gira", "--path", filepath.Join(dir, "*.jsonl")}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "audit verify: ok (1 records)") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestAuditCommandBranchesAndVerifyJSONFailure(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"audit", "missing"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown audit command: missing") || !strings.Contains(stderr.String(), "gira audit verify") {
		t.Fatalf("stderr missing audit remediation:\n%s", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"audit", "readiness"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--repo is required") {
		t.Fatalf("stderr missing readiness repo requirement:\n%s", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"audit", "verify", "--repo", "StatPan/gira", "--path", filepath.Join(t.TempDir(), "*.jsonl"), "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"valid": false`) || !strings.Contains(stdout.String(), `"failure": "no_audit_files_found"`) {
		t.Fatalf("audit verify JSON missing failure evidence:\n%s", stdout.String())
	}
}

func TestAuditWorkflowJSONUsesInjectedReportAndExitCode(t *testing.T) {
	restore := newAuditWorkflowReport
	t.Cleanup(func() { newAuditWorkflowReport = restore })
	newAuditWorkflowReport = func(repo gira.RepoRef) (gira.WorkflowAuditReport, error) {
		if repo.FullName() != "StatPan/gira" {
			t.Fatalf("repo = %q, want StatPan/gira", repo.FullName())
		}
		return gira.WorkflowAuditReport{
			Repo:      repo.FullName(),
			Command:   "audit workflow",
			CheckedAt: "2026-05-18T12:00:00Z",
			Ready:     false,
			Counts:    gira.WorkflowAuditCounts{IssuesScanned: 2, PRsScanned: 1, Findings: 1},
			Findings: []gira.WorkflowAuditFinding{{
				ID:          "open_issue_done_status",
				Severity:    "fail",
				IssueNumber: 7,
				Detail:      "open issue has terminal status:done",
				Remediation: "normalize status",
			}},
			NextStep: "gira adopt issues --repo StatPan/gira --state all --issues 7 --normalize-status --dry-run",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"audit", "workflow", "--repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "audit workflow"`, `"ready": false`, `"issues_scanned": 2`, `"id": "open_issue_done_status"`, `"next_step": "gira adopt issues`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("audit workflow JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestAuditWorkflowTextReady(t *testing.T) {
	restore := newAuditWorkflowReport
	t.Cleanup(func() { newAuditWorkflowReport = restore })
	newAuditWorkflowReport = func(repo gira.RepoRef) (gira.WorkflowAuditReport, error) {
		return gira.WorkflowAuditReport{
			Repo:     repo.FullName(),
			Command:  "audit workflow",
			Ready:    true,
			Counts:   gira.WorkflowAuditCounts{IssuesScanned: 2, PRsScanned: 1},
			NextStep: "workflow contract is converged",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"audit", "workflow", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"audit workflow: READY", "repo: StatPan/gira", "next step: workflow contract is converged"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("audit workflow text missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestAuditDriftAliasUsesWorkflowAuditReport(t *testing.T) {
	restore := newAuditWorkflowReport
	t.Cleanup(func() { newAuditWorkflowReport = restore })
	newAuditWorkflowReport = func(repo gira.RepoRef) (gira.WorkflowAuditReport, error) {
		return gira.WorkflowAuditReport{
			Repo:     repo.FullName(),
			Command:  "audit drift",
			Ready:    true,
			Counts:   gira.WorkflowAuditCounts{IssuesScanned: 1},
			NextStep: "workflow contract is converged",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"audit", "drift", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "audit drift: READY") {
		t.Fatalf("audit drift alias output unexpected:\n%s", stdout.String())
	}
}

func TestAuditReadinessJSONUsesInjectedReportAndExitCode(t *testing.T) {
	restore := newAuditReadinessReport
	t.Cleanup(func() { newAuditReadinessReport = restore })
	newAuditReadinessReport = func(repo gira.RepoRef, ledgerPath string) gira.AuditReadinessReport {
		if repo.FullName() != "StatPan/gira" {
			t.Fatalf("repo = %q, want StatPan/gira", repo.FullName())
		}
		if ledgerPath != ".gira/audit/*.jsonl" {
			t.Fatalf("ledgerPath = %q, want default path", ledgerPath)
		}
		return gira.AuditReadinessReport{
			Repo:      "StatPan/gira",
			Command:   "audit readiness",
			Mode:      gira.AuditReadinessModeDailyOperation,
			Ready:     false,
			CheckedAt: "2026-05-08T12:00:00Z",
			Doctor: gira.DoctorReport{
				Repo:      "StatPan/gira",
				Command:   "doctor",
				CheckedAt: "2026-05-08T12:00:00Z",
				Ready:     true,
			},
			Audit: gira.AuditReadinessHealth{
				Status:      gira.AuditReadinessStatusFailed,
				Detail:      "malformed_json in .gira/audit/StatPan_gira.jsonl line 1",
				Remediation: "fix the audit ledger, then run `gira audit verify --repo StatPan/gira --path .gira/audit/*.jsonl`",
				Verify: gira.AuditVerifyReport{
					Valid:       false,
					Failure:     "malformed_json",
					FailureFile: ".gira/audit/StatPan_gira.jsonl",
					FailureLine: 1,
				},
			},
			NextStep: "fix audit ledger corruption, then run `gira audit verify --repo StatPan/gira --path .gira/audit/*.jsonl`",
		}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"audit", "readiness", "--repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"repo": "StatPan/gira"`, `"command": "audit readiness"`, `"mode": "daily_operation"`, `"ready": false`, `"checked_at": "2026-05-08T12:00:00Z"`, `"audit": {`, `"next_step": "fix audit ledger corruption`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("audit readiness JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestAuditReadinessHumanUsesInjectedReport(t *testing.T) {
	restore := newAuditReadinessReport
	t.Cleanup(func() { newAuditReadinessReport = restore })
	newAuditReadinessReport = func(repo gira.RepoRef, ledgerPath string) gira.AuditReadinessReport {
		return gira.AuditReadinessReport{
			Repo:      repo.FullName(),
			Command:   "audit readiness",
			Mode:      gira.AuditReadinessModeNoOpenWork,
			Ready:     true,
			CheckedAt: "2026-05-08T12:00:00Z",
			Doctor: gira.DoctorReport{
				Repo:      repo.FullName(),
				Command:   "doctor",
				CheckedAt: "2026-05-08T12:00:00Z",
				Ready:     true,
				Checks: []gira.DoctorCheck{{
					ID:     "repo_context",
					Status: gira.DoctorCheckPass,
					Detail: "using --repo " + repo.FullName(),
				}},
			},
			Audit: gira.AuditReadinessHealth{
				Status: gira.AuditReadinessStatusMissing,
				Detail: "no audit ledger found for " + repo.FullName() + " at " + ledgerPath,
				Verify: gira.AuditVerifyReport{Valid: false, Failure: "no_audit_files_found"},
			},
			NextStep: "run `gira status --repo " + repo.FullName() + "`",
		}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"audit", "readiness", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"mode: no_open_work", "readiness/doctor checks:", "audit ledger health:", "[warn] audit_ledger"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("audit readiness output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestAuditVerifyFailsForMismatchedRepoPath(t *testing.T) {
	dir := t.TempDir()
	otherPath := filepath.Join(dir, "OtherOrg_other.jsonl")
	if err := gira.AppendAuditRecords(otherPath, []gira.AuditRecord{gira.NewAuditRecord("sync", "sha256:b", "label:create", "issue#2", "ok", "", "allowed", time.Now())}); err != nil {
		t.Fatalf("append other audit: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"audit", "verify", "--repo", "StatPan/gira", "--path", otherPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout: %s stderr: %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "audit verify: failed (no_audit_files_found)") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestApplyPathsFailClosedWhenAuditWriteFails(t *testing.T) {
	restoreSyncClient := newSyncClient
	restoreProjectSync := newProjectSyncApplyReport
	restoreTransitions := newProjectTransitionsApplyReport
	t.Cleanup(func() {
		newSyncClient = restoreSyncClient
		newProjectSyncApplyReport = restoreProjectSync
		newProjectTransitionsApplyReport = restoreTransitions
	})

	blockAuditWrites(t)

	syncClient := &cliFakeSyncClient{repo: mustCLIRepo(t, "StatPan/gira")}
	newSyncClient = func(repo gira.RepoRef) gira.SyncClient {
		syncClient.repo = repo
		return syncClient
	}
	newProjectSyncApplyReport = func(repo gira.RepoRef) (gira.ProjectSyncApplyReport, error) {
		return gira.ProjectSyncApplyReport{Repo: repo.FullName(), Command: "project sync", DryRun: false}, nil
	}
	newProjectTransitionsApplyReport = func(repo gira.RepoRef) (gira.ProjectTransitionsApplyReport, error) {
		return gira.ProjectTransitionsApplyReport{Repo: repo.FullName(), Command: "project transitions", DryRun: false}, nil
	}

	cases := [][]string{
		{"sync", "--repo", "StatPan/gira"},
		{"project", "sync", "--repo", "StatPan/gira", "--apply"},
		{"project", "transitions", "--repo", "StatPan/gira", "--apply"},
	}
	for _, args := range cases {
		var stdout, stderr bytes.Buffer
		code := Run(args, &stdout, &stderr)
		if code == 0 {
			t.Fatalf("expected non-zero for %v", args)
		}
		if !strings.Contains(stderr.String(), "audit write failed") {
			t.Fatalf("expected audit failure for %v; stderr: %s", args, stderr.String())
		}
	}
}

func blockAuditWrites(t *testing.T) {
	t.Helper()
	base := filepath.Join(os.TempDir(), "gira-audit-test")
	_ = os.RemoveAll(base)
	if err := os.WriteFile(base, []byte("block"), 0o644); err != nil {
		t.Fatalf("prepare audit block file: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Remove(base)
	})
}

func TestGuardrailsSyncCommandJSONUsesInjectedBuilder(t *testing.T) {
	restore := newGuardrailsSyncReport
	t.Cleanup(func() { newGuardrailsSyncReport = restore })
	newGuardrailsSyncReport = func(repo gira.RepoRef, policyPath string, apply bool, allowRelaxation bool) (gira.GuardrailsSyncReport, error) {
		return gira.GuardrailsSyncReport{Repo: repo.FullName(), BlockedCount: 1}, nil
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"guardrails", "sync", "--repo", "StatPan/gira", "--policy", ".gira/guardrails.yaml", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"blocked_count\": 1") {
		t.Fatalf("missing blocked_count: %s", stdout.String())
	}
}

func TestGraphValidateCommandUsesInjectedClient(t *testing.T) {
	restore := newGraphClient
	t.Cleanup(func() { newGraphClient = restore })
	newGraphClient = func(repo gira.RepoRef) gira.GraphClient {
		if repo.FullName() != "StatPan/gira" {
			t.Fatalf("unexpected repo: %s", repo.FullName())
		}
		return cliFakeGraphClient{
			repo: repo,
			issues: []gira.GraphIssue{
				{Number: 1, State: "open", Labels: []string{"type:epic"}, Body: ""},
				{Number: 2, State: "open", Labels: []string{"type:task"}, Body: "Parent: #1"},
			},
		}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"graph", "validate", "--repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var report gira.GraphValidationReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode graph JSON: %v\n%s", err, stdout.String())
	}
	if report.Repo != "StatPan/gira" || report.Counts.Issues != 2 || report.Counts.Diagnostics != 0 {
		t.Fatalf("unexpected graph report: %+v", report)
	}
}

func TestParseCSVIntsRejectsInvalidValues(t *testing.T) {
	got, err := parseCSVInts("1, 2,3")
	if err != nil {
		t.Fatalf("parseCSVInts returned error: %v", err)
	}
	if fmt.Sprint(got) != "[1 2 3]" {
		t.Fatalf("parseCSVInts = %v, want [1 2 3]", got)
	}
	if _, err := parseCSVInts("1,nope"); err == nil || !strings.Contains(err.Error(), "invalid integer") {
		t.Fatalf("parseCSVInts invalid error = %v, want invalid integer", err)
	}
}

type cliFakeGraphClient struct {
	repo   gira.RepoRef
	issues []gira.GraphIssue
	err    error
}

func (c cliFakeGraphClient) Repo() gira.RepoRef {
	return c.repo
}

func (c cliFakeGraphClient) Issues() ([]gira.GraphIssue, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.issues, nil
}

func TestGuardrailsSyncRequiresMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"guardrails", "sync", "--repo", "StatPan/gira", "--policy", ".gira/guardrails.yaml"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero")
	}
	if !strings.Contains(stderr.String(), "exactly one of --dry-run or --apply is required") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestGuardrailsSyncApplyDeniedPermissionFailsClosed(t *testing.T) {
	restoreCapability := newProjectCapabilityReport
	t.Cleanup(func() { newProjectCapabilityReport = restoreCapability })
	newProjectCapabilityReport = func(repo gira.RepoRef) (gira.ProjectCapabilityReport, error) {
		return gira.ProjectCapabilityReport{
			Repo: repo.FullName(),
			Capabilities: map[string]gira.ProjectCapabilityStatus{
				"repo:settings:write": gira.ProjectCapabilityDeniedScope,
			},
			BlockedActions: []gira.ProjectCapabilityBlock{{
				Action: "repo:settings:write",
				Reason: "token scope or repository permission is insufficient",
			}},
		}, nil
	}

	dir := t.TempDir()
	policyPath := filepath.Join(dir, "guardrails.yaml")
	if err := os.WriteFile(policyPath, []byte(`branch_protection:
  main:
    required_approving_review_count: 2
    require_code_owner_reviews: true
    required_status_checks_strict: true
    allow_force_pushes: false
    allow_deletions: false
`), 0o644); err != nil {
		t.Fatalf("write policy: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"guardrails", "sync", "--repo", "StatPan/gira", "--policy", policyPath, "--apply", "--json"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit for denied permission")
	}
	if !strings.Contains(stderr.String(), "repo:settings:write denied") {
		t.Fatalf("missing explicit denied reason: %s", stderr.String())
	}
}

type devCLIRunner struct {
	outputs map[string][]byte
	errs    map[string]error
}

func (r devCLIRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	if err, ok := r.errs[key]; ok {
		return nil, err
	}
	if out, ok := r.outputs[key]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected call: %s", key)
}

func TestDevStartJSONDryRun(t *testing.T) {
	original := devCommandRunner
	t.Cleanup(func() { devCommandRunner = original })
	devCommandRunner = devCLIRunner{
		outputs: map[string][]byte{
			"gh api repos/StatPan/gira/issues/59": []byte(`{"number":59,"title":"Start branch","state":"open","labels":[{"name":"status:ready"}]}`),
		},
		errs: map[string]error{
			"git show-ref --verify --quiet refs/heads/issue-59-start-branch": fmt.Errorf("exit status 1"),
			"git ls-remote --exit-code --heads origin issue-59-start-branch": fmt.Errorf("exit status 2"),
		},
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"dev", "start", "--repo", "StatPan/gira", "--issue", "59", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"branch": "issue-59-start-branch"`) {
		t.Fatalf("missing branch in output: %s", stdout.String())
	}
}

func TestDevStartJSONReusesLocalBranch(t *testing.T) {
	original := devCommandRunner
	t.Cleanup(func() { devCommandRunner = original })
	devCommandRunner = devCLIRunner{
		outputs: map[string][]byte{
			"gh api repos/StatPan/gira/issues/59":                            []byte(`{"number":59,"title":"Start branch","state":"open","labels":[{"name":"status:ready"}]}`),
			"git show-ref --verify --quiet refs/heads/issue-59-start-branch": nil,
			"git ls-remote --exit-code --heads origin issue-59-start-branch": []byte("abc\trefs/heads/issue-59-start-branch"),
			"git checkout issue-59-start-branch":                             nil,
		},
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"dev", "start", "--repo", "StatPan/gira", "--issue", "59", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"created": false`) {
		t.Fatalf("expected created=false for local branch reuse: %s", stdout.String())
	}
}

func TestDevPROpenJSON(t *testing.T) {
	original := devCommandRunner
	t.Cleanup(func() { devCommandRunner = original })
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/60":                                          []byte(`{"number":60,"title":"Add PR loop","state":"open","labels":[{"name":"status:ready"}]}`),
		"gh pr create --repo StatPan/gira --title feat: Add PR loop --body Closes #60": []byte("https://github.com/StatPan/gira/pull/99\n"),
	}}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"dev", "pr", "open", "--repo", "StatPan/gira", "--issue", "60", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"pr_number": 99`) {
		t.Fatalf("missing pr_number: %s", stdout.String())
	}
}

func TestDevPRStatusJSON(t *testing.T) {
	original := devCommandRunner
	t.Cleanup(func() { devCommandRunner = original })
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 60 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[{"number":99,"title":"x","body":"Closes #60","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
	}}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"dev", "pr", "status", "--repo", "StatPan/gira", "--issue", "60", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"ready": true`) {
		t.Fatalf("expected ready true: %s", stdout.String())
	}
}

func TestInitRequiresRepo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"init", "--json"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero without --repo")
	}
	if !strings.Contains(stderr.String(), "--repo is required") {
		t.Fatalf("missing repo error: %s", stderr.String())
	}
}

func TestInitJSONReady(t *testing.T) {
	original := devCommandRunner
	t.Cleanup(func() { devCommandRunner = original })
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"gh --version":                                 []byte("gh version 2"),
		"git --version":                                []byte("git version 2"),
		"gh auth status":                               []byte("ok"),
		"gh repo view StatPan/gira --json name":        []byte(`{"name":"gira"}`),
		"git -C /repo rev-parse --is-inside-work-tree": []byte("true"),
		"git -C /repo diff --quiet":                    nil,
		"git -C /repo diff --cached --quiet":           nil,
	}}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"init", "--repo", "StatPan/gira", "--path", "/repo", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"ready": true`) {
		t.Fatalf("expected ready true: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `gira adopt repo --repo StatPan/gira --path /repo --dry-run`) {
		t.Fatalf("expected init to point at repo adoption: %s", stdout.String())
	}
}

func TestInitReadsConfig(t *testing.T) {
	original := devCommandRunner
	t.Cleanup(func() { devCommandRunner = original })
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"gh --version":                                 []byte("gh version 2"),
		"git --version":                                []byte("git version 2"),
		"gh auth status":                               []byte("ok"),
		"gh repo view StatPan/gira --json name":        []byte(`{"name":"gira"}`),
		"git -C /repo rev-parse --is-inside-work-tree": []byte("true"),
		"git -C /repo diff --quiet":                    nil,
		"git -C /repo diff --cached --quiet":           nil,
	}}
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`profiles:
  default:
    labels: ["type:task"]
    review_policy:
      required_approvals: 1
`), 0o644)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"init", "--repo", "StatPan/gira", "--path", "/repo", "--config", configPath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"ready": true`) {
		t.Fatalf("expected ready true: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"config_path":`) {
		t.Fatalf("expected config path in output: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"config_profile_count": 1`) {
		t.Fatalf("expected config profile count in output: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `config profile \"default\": labels=1 milestones=0 issue_templates=0 approvals=1 codeowners=false`) {
		t.Fatalf("expected config profile plan details in output: %s", stdout.String())
	}
}

func TestInitUsesDefaultConfigPathWhenPresent(t *testing.T) {
	original := devCommandRunner
	t.Cleanup(func() { devCommandRunner = original })
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"gh --version":                                 []byte("gh version 2"),
		"git --version":                                []byte("git version 2"),
		"gh auth status":                               []byte("ok"),
		"gh repo view StatPan/gira --json name":        []byte(`{"name":"gira"}`),
		"git -C /repo rev-parse --is-inside-work-tree": []byte("true"),
		"git -C /repo diff --quiet":                    nil,
		"git -C /repo diff --cached --quiet":           nil,
	}}

	dir := t.TempDir()
	giraDir := filepath.Join(dir, ".gira")
	if err := os.MkdirAll(giraDir, 0o755); err != nil {
		t.Fatalf("mkdir .gira: %v", err)
	}
	configPath := filepath.Join(giraDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`profiles:
  default:
    labels: ["type:task"]
`), 0o644)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir tempdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	var stdout, stderr bytes.Buffer
	code := Run([]string{"init", "--repo", "StatPan/gira", "--path", "/repo", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"config_path": ".gira/config.yaml"`) {
		t.Fatalf("expected default config path in output: %s", stdout.String())
	}
}

func TestInitUsesWorkspaceDefaultConfigPathWhenPresent(t *testing.T) {
	original := devCommandRunner
	t.Cleanup(func() { devCommandRunner = original })

	workspace := t.TempDir()
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"gh --version":                          []byte("gh version 2"),
		"git --version":                         []byte("git version 2"),
		"gh auth status":                        []byte("ok"),
		"gh repo view StatPan/gira --json name": []byte(`{"name":"gira"}`),
		"git -C " + workspace + " rev-parse --is-inside-work-tree": []byte("true"),
		"git -C " + workspace + " diff --quiet":                    nil,
		"git -C " + workspace + " diff --cached --quiet":           nil,
	}}
	giraDir := filepath.Join(workspace, ".gira")
	if err := os.MkdirAll(giraDir, 0o755); err != nil {
		t.Fatalf("mkdir .gira: %v", err)
	}
	configPath := filepath.Join(giraDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`profiles:
  default:
    labels: ["type:task"]
`), 0o644)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"init", "--repo", "StatPan/gira", "--path", workspace, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"config_path": `) || !strings.Contains(stdout.String(), filepath.ToSlash(configPath)) {
		t.Fatalf("expected workspace config path in output: %s", stdout.String())
	}
}

func TestInitInvalidConfigFails(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`profiles:
  default:
    review_policy:
      required_approvals: -1
`), 0o644)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"init", "--repo", "StatPan/gira", "--config", configPath}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero for invalid config")
	}
	if !strings.Contains(stderr.String(), "required_approvals") {
		t.Fatalf("expected actionable config error: %s", stderr.String())
	}
}

func TestAdoptIssuesJSONUsesInjectedReport(t *testing.T) {
	original := newAdoptIssuesReport
	t.Cleanup(func() { newAdoptIssuesReport = original })
	newAdoptIssuesReport = func(input gira.AdoptIssueInput) (gira.AdoptIssuesReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || !input.DryRun || input.Apply || input.State != "all" || !input.NormalizeStatus {
			t.Fatalf("unexpected input: %+v", input)
		}
		if len(input.Issues) != 1 || input.Issues[0] != 7 || input.Milestone != "MVP" {
			t.Fatalf("unexpected issue mapping input: %+v", input)
		}
		return gira.AdoptIssuesReport{
			Repo:            input.Repo.FullName(),
			DryRun:          true,
			State:           input.State,
			Issues:          append([]int(nil), input.Issues...),
			Milestone:       input.Milestone,
			Labels:          append([]string(nil), input.Labels...),
			NormalizeStatus: input.NormalizeStatus,
			Actions: []gira.AdoptIssuesAction{{
				Issue:     7,
				Title:     "Normalize me",
				Action:    "issue:update",
				Status:    "planned",
				Milestone: "MVP",
				Labels:    []string{"type:task"},
				Reason:    "explicit issue adoption mapping",
			}},
			NextStep: "gira adopt issues --repo StatPan/gira --issues 7 --apply",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"adopt", "issues", "--repo", "StatPan/gira", "--state", "all", "--issues", "7", "--milestone", "MVP", "--label", "type:task", "--normalize-status", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	var report gira.AdoptIssuesReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode adopt issues JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.AdoptIssuesReportSchemaVersion || report.Approval == nil {
		t.Fatalf("adopt issues dry-run JSON missing schema or approval:\n%s", stdout.String())
	}
	if report.Approval.SchemaVersion != gira.ApprovalPlanSchemaVersion || report.Approval.CanonicalCommand != "gira adopt issues" || report.Approval.OutputSchema != gira.AdoptIssuesReportSchemaVersion {
		t.Fatalf("unexpected adopt issues approval evidence: %+v", report.Approval)
	}
	if report.Approval.ApplyCommand != "gira adopt issues --repo StatPan/gira --state all --issues 7 --milestone MVP --label type:task --normalize-status --apply" || report.Approval.PostApplyVerification != "gira status --repo StatPan/gira --json" {
		t.Fatalf("unexpected adopt issues approval commands: %+v", report.Approval)
	}
	if report.Approval.Blockers == nil || report.Approval.Warnings == nil {
		t.Fatalf("approval blockers and warnings must be stable arrays: %+v", report.Approval)
	}
}

func TestAdoptIssuesApplyJSONOmitsApprovalEvidence(t *testing.T) {
	original := newAdoptIssuesReport
	t.Cleanup(func() { newAdoptIssuesReport = original })
	newAdoptIssuesReport = func(input gira.AdoptIssueInput) (gira.AdoptIssuesReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || !input.Apply || input.DryRun {
			t.Fatalf("unexpected input: %+v", input)
		}
		return gira.AdoptIssuesReport{
			Repo:  input.Repo.FullName(),
			Apply: true,
			State: "open",
			Actions: []gira.AdoptIssuesAction{{
				Issue:  7,
				Action: "issue:update",
				Status: "applied",
				Reason: "explicit issue adoption mapping",
			}},
			NextStep: "gira status --repo StatPan/gira",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"adopt", "issues", "--repo", "StatPan/gira", "--issues", "7", "--label", "type:task", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	var report gira.AdoptIssuesReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode adopt issues JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.AdoptIssuesReportSchemaVersion || !report.Apply {
		t.Fatalf("unexpected adopt issues apply report: %+v", report)
	}
	if report.Approval != nil {
		t.Fatalf("apply output should not include dry-run approval evidence: %+v", report.Approval)
	}
}

func TestAdoptRepoJSONUsesInjectedReport(t *testing.T) {
	original := newAdoptRepoReport
	t.Cleanup(func() { newAdoptRepoReport = original })
	newAdoptRepoReport = func(input gira.AdoptRepoInput) (gira.AdoptRepoReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Path != "." || !input.DryRun || input.Apply {
			t.Fatalf("unexpected input: %+v", input)
		}
		return gira.AdoptRepoReport{
			Repo:           input.Repo.FullName(),
			Path:           "/repo",
			DryRun:         true,
			Strategy:       "merge",
			Recommendation: "merge",
			Actions:        []gira.AdoptRepoAction{{Action: "config:create", Target: "/repo/.gira/config.yaml", Status: "planned", Reason: "minimal contract"}},
			NextStep:       "gira adopt repo --apply",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"adopt", "repo", "--repo", "StatPan/gira", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"strategy": "merge"`) || !strings.Contains(stdout.String(), `"next_step": "gira adopt repo --apply"`) {
		t.Fatalf("unexpected JSON: %s", stdout.String())
	}
	var report gira.AdoptRepoReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode adopt repo JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.AdoptRepoReportSchemaVersion || report.Approval == nil {
		t.Fatalf("adopt repo dry-run JSON missing schema or approval:\n%s", stdout.String())
	}
	if report.Approval.SchemaVersion != gira.ApprovalPlanSchemaVersion || report.Approval.CanonicalCommand != "gira adopt repo" || report.Approval.OutputSchema != gira.AdoptRepoReportSchemaVersion {
		t.Fatalf("unexpected adopt repo approval evidence: %+v", report.Approval)
	}
	if report.Approval.ApplyCommand != "gira adopt repo --repo StatPan/gira --path /repo --strategy merge --apply" || report.Approval.PostApplyVerification != "gira config repo --repo StatPan/gira --json" {
		t.Fatalf("unexpected adopt repo approval commands: %+v", report.Approval)
	}
	if report.Approval.Blockers == nil || report.Approval.Warnings == nil {
		t.Fatalf("approval blockers and warnings must be stable arrays: %+v", report.Approval)
	}
}

func TestAdoptRepoApplyJSONOmitsApprovalEvidence(t *testing.T) {
	original := newAdoptRepoReport
	t.Cleanup(func() { newAdoptRepoReport = original })
	newAdoptRepoReport = func(input gira.AdoptRepoInput) (gira.AdoptRepoReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Path != "." || input.Strategy != "merge" || !input.Apply || input.DryRun {
			t.Fatalf("unexpected input: %+v", input)
		}
		return gira.AdoptRepoReport{Repo: input.Repo.FullName(), Path: "/repo", Apply: true, Strategy: "merge", Recommendation: "merge", NextStep: "gira ops sync --repo StatPan/gira --dry-run"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"adopt", "repo", "--repo", "StatPan/gira", "--strategy", "merge", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	var report gira.AdoptRepoReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode adopt repo JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.AdoptRepoReportSchemaVersion || !report.Apply {
		t.Fatalf("unexpected adopt repo apply report: %+v", report)
	}
	if report.Approval != nil {
		t.Fatalf("apply output should not include dry-run approval evidence: %+v", report.Approval)
	}
}

func TestAdoptRepoApplyRequiresExplicitStrategy(t *testing.T) {
	original := newAdoptRepoReport
	t.Cleanup(func() { newAdoptRepoReport = original })
	newAdoptRepoReport = func(input gira.AdoptRepoInput) (gira.AdoptRepoReport, error) {
		return gira.AdoptRepoReport{Repo: input.Repo.FullName(), Apply: input.Apply}, fmt.Errorf("--apply requires --strategy observe|merge|normalize or --yes")
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"adopt", "repo", "--repo", "StatPan/gira", "--apply"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	if !strings.Contains(stderr.String(), "--apply requires --strategy") {
		t.Fatalf("stderr missing explicit strategy guidance: %s", stderr.String())
	}
}

func TestReportWeeklyJSON(t *testing.T) {
	restoreDash := newDashboardExportClient
	restoreReview := newReviewGateClient
	restoreNow := reportNow
	t.Cleanup(func() {
		newDashboardExportClient = restoreDash
		newReviewGateClient = restoreReview
		reportNow = restoreNow
	})
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	reportNow = func() time.Time { return now }
	newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient {
		return weeklyDashClient{repo: repo, issues: []gira.DashboardRawIssue{{IssueNumber: 70, Title: "Blocked", State: "open", Labels: []string{"blocked"}, UpdatedAt: now.Add(-15 * 24 * time.Hour).Format(time.RFC3339), URL: "https://example/issues/70"}}}
	}
	newReviewGateClient = func(repo gira.RepoRef) gira.ReviewGateClient {
		return weeklyReviewClient{repo: repo, prs: []gira.ReviewPR{{Number: 77, Title: "Wait", URL: "https://example/pr/77", UpdatedAt: now.Add(-72 * time.Hour).Format(time.RFC3339), RequestedReviewers: []string{"alice"}}}, issues: []gira.ReviewIssue{{Number: 70, Labels: []string{"blocker"}}}}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"report", "weekly", "--repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"backlog_health": "amber"`) {
		t.Fatalf("missing backlog health in output: %s", stdout.String())
	}
}

func TestReportWeeklyBundle(t *testing.T) {
	restoreDash := newDashboardExportClient
	restoreReview := newReviewGateClient
	restoreNow := reportNow
	t.Cleanup(func() {
		newDashboardExportClient = restoreDash
		newReviewGateClient = restoreReview
		reportNow = restoreNow
	})
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	reportNow = func() time.Time { return now }
	newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient {
		return weeklyDashClient{repo: repo}
	}
	newReviewGateClient = func(repo gira.RepoRef) gira.ReviewGateClient {
		return weeklyReviewClient{repo: repo}
	}

	outputRoot := filepath.Join(t.TempDir(), "weekly")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"report", "weekly", "--repo", "StatPan/gira", "--format", "bundle", "--output", outputRoot}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	for _, rel := range []string{"index.html", "weekly.md", "derived/weekly_report.json", "csv/weekly_exceptions.csv"} {
		if _, err := os.Stat(filepath.Join(outputRoot, rel)); err != nil {
			t.Fatalf("missing weekly bundle artifact %s", rel)
		}
	}
}

func TestReportProjectDocuments(t *testing.T) {
	restoreDash := newDashboardExportClient
	restoreReview := newReviewGateClient
	restoreNow := reportNow
	t.Cleanup(func() {
		newDashboardExportClient = restoreDash
		newReviewGateClient = restoreReview
		reportNow = restoreNow
	})
	now := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	reportNow = func() time.Time { return now }
	newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient {
		return weeklyDashClient{
			repo: repo,
			issues: []gira.DashboardRawIssue{
				{IssueNumber: 10, Title: "Closed feature", State: "closed", Labels: []string{"type:feature", "qa"}, UpdatedAt: now.Add(-24 * time.Hour).Format(time.RFC3339), Milestone: "v2.5.0", URL: "https://example/issues/10"},
				{IssueNumber: 11, Title: "Blocked task", State: "open", Labels: []string{"blocked"}, UpdatedAt: now.Add(-20 * 24 * time.Hour).Format(time.RFC3339), Milestone: "v2.5.0", URL: "https://example/issues/11"},
			},
			milestones: []gira.DashboardRawMilestone{{Title: "v2.5.0", State: "open", OpenIssues: 1, ClosedIssues: 1}},
		}
	}
	newReviewGateClient = func(repo gira.RepoRef) gira.ReviewGateClient {
		return weeklyReviewClient{repo: repo, prs: []gira.ReviewPR{{Number: 20, Title: "Open PR", Body: "Fixes #11", URL: "https://example/pr/20", CheckStatus: "pending", UpdatedAt: now.Add(-48 * time.Hour).Format(time.RFC3339)}}}
	}

	cases := [][]string{
		{"report", "milestone", "--repo", "StatPan/gira", "--milestone", "v2.5.0", "--json"},
		{"report", "backlog-health", "--repo", "StatPan/gira", "--csv"},
		{"report", "delivery-status", "--repo", "StatPan/gira", "--format", "md"},
		{"report", "qa-checklist", "--repo", "StatPan/gira", "--milestone", "v2.5.0", "--format", "html"},
	}
	for _, args := range cases {
		var stdout, stderr bytes.Buffer
		code := Run(args, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("Run(%v) exit code=%d stderr=%s", args, code, stderr.String())
		}
		if stdout.Len() == 0 {
			t.Fatalf("Run(%v) produced no output", args)
		}
	}
}

func TestReviewGateFailsWhenChecksFail(t *testing.T) {
	original := reviewGateRunner
	t.Cleanup(func() { reviewGateRunner = original })
	reviewGateRunner = devCLIRunner{outputs: map[string][]byte{
		"gofmt -l .":          []byte(""),
		"go test ./...":       []byte(""),
		"go test -race ./...": []byte(""),
	}, errs: map[string]error{"go vet ./...": fmt.Errorf("vet failed")}}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"review", "gate", "--json", "--local-exec"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero when checks fail")
	}
	if !strings.Contains(stdout.String(), `"ready": false`) {
		t.Fatalf("expected readiness false output: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"evidence_source": "local_execution"`) {
		t.Fatalf("expected local execution evidence source: %s", stdout.String())
	}
}

func TestReviewGateDefaultDoesNotRunLocalChecks(t *testing.T) {
	original := reviewGateRunner
	t.Cleanup(func() { reviewGateRunner = original })
	reviewGateRunner = devCLIRunner{errs: map[string]error{
		"gofmt -l .":    fmt.Errorf("should not run"),
		"go vet ./...":  fmt.Errorf("should not run"),
		"go test ./...": fmt.Errorf("should not run"),
	}}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"review", "gate", "--json"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected blocked default review gate")
	}
	for _, want := range []string{`"ready": false`, `"evidence_source": "static_policy"`, `"execution_mode": "no_local_execution"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("review gate output missing %q: %s", want, stdout.String())
		}
	}
}

func TestReviewQueueJSONUsesInjectedClient(t *testing.T) {
	restore := newReviewGateClient
	t.Cleanup(func() { newReviewGateClient = restore })
	newReviewGateClient = func(repo gira.RepoRef) gira.ReviewGateClient {
		if repo.FullName() != "StatPan/gira" {
			t.Fatalf("review queue repo = %s", repo.FullName())
		}
		return weeklyReviewClient{repo: repo, prs: []gira.ReviewPR{
			{Number: 31, Title: "Needs review", Body: "Fixes #30", ReviewDecision: "", CheckStatus: "passing", RequestedReviewers: []string{"alice"}, UpdatedAt: "2026-04-01T00:00:00Z"},
		}}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"review", "queue", "--repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{`"repo": "StatPan/gira"`, `"number": 31`, `"route_to": "alice"`, `"missing_approval"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("review queue JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestMergeQueueRequiresMode(t *testing.T) {
	for _, args := range [][]string{
		{"merge", "queue", "--repo", "StatPan/gira"},
		{"merge", "queue", "--repo", "StatPan/gira", "--dry-run", "--apply"},
		{"merge", "queue", "--dry-run"},
	} {
		var stdout, stderr bytes.Buffer
		code := Run(args, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("Run(%v) exit code=%d, want 2", args, code)
		}
		if !strings.Contains(stderr.String(), "--repo and exactly one of --dry-run/--apply are required") {
			t.Fatalf("Run(%v) stderr missing mode guidance:\n%s", args, stderr.String())
		}
	}
}

func TestMergeQueueDryRunJSONDoesNotMerge(t *testing.T) {
	restore := newReviewGateClient
	t.Cleanup(func() { newReviewGateClient = restore })
	merged := []int{}
	newReviewGateClient = func(repo gira.RepoRef) gira.ReviewGateClient {
		return weeklyReviewClient{repo: repo, merged: &merged, prs: []gira.ReviewPR{
			{Number: 40, Title: "Ready", Body: "Closes #40", ReviewDecision: "APPROVED", CheckStatus: "passing", UpdatedAt: "2026-05-01T00:00:00Z"},
			{Number: 41, Title: "Blocked", Body: "", ReviewDecision: "", CheckStatus: "failing", UpdatedAt: "2026-05-01T00:00:00Z"},
		}}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"merge", "queue", "--repo", "StatPan/gira", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	if len(merged) != 0 {
		t.Fatalf("dry-run should not merge, got %v", merged)
	}
	for _, want := range []string{`"mode": "dry-run"`, `"number": 40`, `"candidates"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("merge queue dry-run JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestMergeQueueApplyJSONMergesCandidates(t *testing.T) {
	restore := newReviewGateClient
	t.Cleanup(func() { newReviewGateClient = restore })
	merged := []int{}
	newReviewGateClient = func(repo gira.RepoRef) gira.ReviewGateClient {
		return weeklyReviewClient{repo: repo, merged: &merged, prs: []gira.ReviewPR{
			{Number: 42, Title: "Ready", Body: "Resolves #42", ReviewDecision: "APPROVED", CheckStatus: "passing", UpdatedAt: "2026-05-01T00:00:00Z"},
		}}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"merge", "queue", "--repo", "StatPan/gira", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	if len(merged) != 1 || merged[0] != 42 {
		t.Fatalf("apply should merge PR 42, got %v", merged)
	}
	if !strings.Contains(stdout.String(), `"mode": "apply"`) || !strings.Contains(stdout.String(), `"merged"`) || !strings.Contains(stdout.String(), "42") {
		t.Fatalf("merge queue apply JSON missing merge evidence:\n%s", stdout.String())
	}
}

func TestReleaseReadinessJSON(t *testing.T) {
	restore := newReviewGateClient
	t.Cleanup(func() { newReviewGateClient = restore })
	newReviewGateClient = func(repo gira.RepoRef) gira.ReviewGateClient {
		return weeklyReviewClient{
			repo:   repo,
			prs:    []gira.ReviewPR{{Number: 50, Title: "Needs approval", Body: "Fixes #60", ReviewDecision: "", CheckStatus: "passing", UpdatedAt: "2026-05-01T00:00:00Z"}},
			issues: []gira.ReviewIssue{{Number: 60, Labels: []string{"blocker"}}, {Number: 61, Labels: []string{"must-fix"}}},
		}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"release", "readiness", "--repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{`"ready": false`, `"open_blocker_issues"`, `"open_must_fix_issues"`, `"missing_approval"`, `"blocker_taxonomy"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("release readiness JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

type weeklyDashClient struct {
	repo       gira.RepoRef
	issues     []gira.DashboardRawIssue
	milestones []gira.DashboardRawMilestone
}

func (c weeklyDashClient) Repo() gira.RepoRef                             { return c.repo }
func (c weeklyDashClient) FetchIssues() ([]gira.DashboardRawIssue, error) { return c.issues, nil }
func (c weeklyDashClient) FetchPullRequests() ([]gira.DashboardRawPullRequest, error) {
	return nil, nil
}
func (c weeklyDashClient) FetchMilestones() ([]gira.DashboardRawMilestone, error) {
	return c.milestones, nil
}
func (c weeklyDashClient) FetchProjectSnapshot() (gira.ProjectSyncSnapshot, error) {
	return gira.ProjectSyncSnapshot{}, nil
}
func (c weeklyDashClient) FetchTransitionSnapshot() (gira.ProjectTransitionSnapshot, error) {
	return gira.ProjectTransitionSnapshot{}, nil
}
func (c weeklyDashClient) FetchCapabilities() (gira.ProjectCapabilityReport, error) {
	return gira.ProjectCapabilityReport{}, nil
}

type cliFakeWBSReportClient struct {
	issues          []gira.WBSRawIssue
	milestones      []gira.DashboardRawMilestone
	projectSnapshot gira.ProjectSyncSnapshot
}

func (c *cliFakeWBSReportClient) FetchIssues() ([]gira.WBSRawIssue, error) {
	return c.issues, nil
}

func (c *cliFakeWBSReportClient) FetchMilestones() ([]gira.DashboardRawMilestone, error) {
	return c.milestones, nil
}

func (c *cliFakeWBSReportClient) FetchProjectSnapshot() (gira.ProjectSyncSnapshot, error) {
	return c.projectSnapshot, nil
}

type cliFakeReleaseNotesClient struct {
	issues     []gira.ReleaseNotesIssue
	prs        []gira.ReleaseNotesPullRequest
	milestones []gira.DashboardRawMilestone
}

func (c *cliFakeReleaseNotesClient) FetchIssues() ([]gira.ReleaseNotesIssue, error) {
	return c.issues, nil
}

func (c *cliFakeReleaseNotesClient) FetchMergedPRs() ([]gira.ReleaseNotesPullRequest, error) {
	return c.prs, nil
}

func (c *cliFakeReleaseNotesClient) FetchMilestones() ([]gira.DashboardRawMilestone, error) {
	return c.milestones, nil
}

type weeklyReviewClient struct {
	repo   gira.RepoRef
	prs    []gira.ReviewPR
	issues []gira.ReviewIssue
	merged *[]int
}

func (c weeklyReviewClient) Repo() gira.RepoRef                          { return c.repo }
func (c weeklyReviewClient) ListOpenPRs() ([]gira.ReviewPR, error)       { return c.prs, nil }
func (c weeklyReviewClient) ListOpenIssues() ([]gira.ReviewIssue, error) { return c.issues, nil }
func (c weeklyReviewClient) MergePR(number int) error {
	if c.merged != nil {
		*c.merged = append(*c.merged, number)
	}
	return nil
}
