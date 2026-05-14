package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	if strings.Contains(stdout.String(), "portfolio   ") || strings.Contains(stdout.String(), "jira        ") {
		t.Fatalf("help output should not frontload advanced commands:\n%s", stdout.String())
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
			DryRun:     true,
			Status:     "planned",
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
	newUpgradeReport = func(channel string) (gira.UpgradeReport, error) {
		if channel != "pipx" {
			t.Fatalf("channel = %q, want pipx", channel)
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
	newUpgradeReport = func(channel string) (gira.UpgradeReport, error) {
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

func TestGuideQuickstartDefault(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"guide"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"Gira quickstart", "gira ticket new", "gira ticket checks", "gira ticket finish --apply"} {
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
	if !strings.Contains(stderr.String(), "unknown guide topic: missing") || !strings.Contains(stderr.String(), "gira guide [quickstart|ticket|stats|jira|agent|skill|concepts]") {
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
	for _, want := range []string{"owner: GiraOrg", "title: \"Roadmap\"", "number: 12"} {
		if !strings.Contains(report.Content, want) {
			t.Fatalf("content missing %q:\n%s", want, report.Content)
		}
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
	var stdout, stderr bytes.Buffer
	code := Run([]string{"jira", "import", "--repo", "StatPan/gira", "--source", "jira.csv"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "exactly one of --dry-run or --apply is required") {
		t.Fatalf("stderr missing mode guidance:\n%s", stderr.String())
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
	for _, want := range []string{`"command": "jira transition"`, `"decision": "direct_transition"`, `"key": "ABC-123"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("jira transition JSON missing %q:\n%s", want, stdout.String())
		}
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
	for _, want := range []string{`"workspace"`, `"repo": "StatPan/backlog"`, `"needs_routing": true`} {
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
	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "sync"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "exactly one of --dry-run or --apply is required for workspace sync") {
		t.Fatalf("stderr missing mode guidance:\n%s", stderr.String())
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
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"workspace", "ticket", "route", "--config", "testdata/workspace.yaml", "--ticket", "8", "--repo", "StatPan/gira", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "workspace ticket route"`, `"dry_run": true`, `"action": "execution_issue:create"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("workspace ticket route JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestProjectsSyncRequiresMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"projects", "sync"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "exactly one of --dry-run or --apply is required for projects sync") {
		t.Fatalf("stderr missing mode guidance:\n%s", stderr.String())
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
			Command: "projects sync",
			DryRun:  true,
			Project: gira.ProjectsSyncProject{Owner: "StatPan", Number: 7, Title: "Gira"},
			Counts:  gira.ProjectsSyncCounts{Issues: 1, ProjectItemsAdd: 1},
			Actions: []gira.ProjectsSyncAction{{Action: "project_item:add", Repo: "StatPan/gira", Issue: 180, Status: "planned"}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"projects", "sync", "--config", "testdata/workspace.yaml", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "projects sync"`, `"project_items_add": 1`, `"action": "project_item:add"`} {
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
		return gira.WorkStartResult{Repo: repo.FullName(), Issue: issue, Branch: "issue-126-work-command", DryRun: true, NextStatus: "In progress"}, nil
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
		return gira.TicketNewReport{Repo: input.Repo.FullName(), Title: input.Title, DryRun: true, Labels: []string{"type:bug", "status:ready", "priority:p1"}, Body: "## Goal\nRetry\n", NextStep: "gira ticket new --apply"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "new", "Add retry", "--goal", "Retry", "--acceptance", "works;has tests", "--type", "bug", "--priority", "p1", "--label", "area:backend", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"title": "Add retry"`) || !strings.Contains(stdout.String(), `"type:bug"`) {
		t.Fatalf("ticket new JSON missing expected fields:\n%s", stdout.String())
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
	for _, want := range []string{"gira ticket list", "--state open|closed|all", "--label LABEL", "--assignee LOGIN", "--milestone TITLE"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("ticket help missing %q:\n%s", want, stdout.String())
		}
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
		}, fmt.Errorf("issue #33 is not ready (missing status:ready)")
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
		"issue #33 is not ready (missing status:ready)",
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
		}, fmt.Errorf("issue #33 is not ready (missing status:ready)")
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
		return gira.TicketNoteReport{Command: "ticket note", Repo: input.Repo.FullName(), Ticket: input.Ticket, Kind: input.Kind, Target: input.Target, DryRun: true, Targets: []gira.TicketNoteSink{{Type: "issue", Number: 126, Status: "planned"}, {Type: "pr", Number: 127, Status: "planned"}}, RenderedBody: "## Decision\n\nParser path works\n", NextStep: "gira ticket note --apply"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "note", "126", "Parser path works", "--repo", "StatPan/gira", "--kind", "decision", "--target", "both", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"command": "ticket note"`) || !strings.Contains(stdout.String(), `"rendered_body": "## Decision`) {
		t.Fatalf("ticket note JSON missing rendered body:\n%s", stdout.String())
	}
}

func TestTicketSupersedeParsesReplacementBodyAndJSON(t *testing.T) {
	restore := newTicketSupersedeReport
	t.Cleanup(func() { newTicketSupersedeReport = restore })
	newTicketSupersedeReport = func(input gira.TicketSupersedeInput) (gira.TicketSupersedeReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Ticket != 64 || input.ReplacementTitle != "New gate" || input.Body != "## Goal\nBody" || !input.DryRun || input.CloseDraftPR {
			t.Fatalf("unexpected input: %+v repo=%s", input, input.Repo.FullName())
		}
		return gira.TicketSupersedeReport{
			Command:  "ticket supersede",
			Repo:     input.Repo.FullName(),
			DryRun:   true,
			Original: gira.TicketSupersedeIssue{Number: input.Ticket, Title: "Old gate"},
			Replacement: gira.TicketSupersedeIssue{
				Title: input.ReplacementTitle,
				Body:  input.Body + "\n\n## Supersedes\n- Supersedes #64\n",
			},
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
	code := Run([]string{"ticket", "supersede", "64", "--repo", "StatPan/gira", "--replacement-title", "New gate", "--body-file", bodyPath, "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"command": "ticket supersede"`) || !strings.Contains(stdout.String(), `"replacement"`) {
		t.Fatalf("ticket supersede JSON missing fields:\n%s", stdout.String())
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
	newWorkFinishResult = func(repo gira.RepoRef, issue int, dryRun bool, wait time.Duration) (gira.WorkFinishResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 219 || !dryRun || wait != 0 {
			t.Fatalf("unexpected args repo=%s issue=%d dryRun=%t wait=%s", repo.FullName(), issue, dryRun, wait)
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
	newWorkFinishResult = func(repo gira.RepoRef, issue int, dryRun bool, wait time.Duration) (gira.WorkFinishResult, error) {
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
	if !strings.Contains(stdout.String(), "Advanced Gira controls") || !strings.Contains(stdout.String(), "sync") {
		t.Fatalf("stdout missing ops help:\n%s", stdout.String())
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
		"gh label list --repo StatPan/gira --json name --limit 1000": []byte(`[{"name":"type:task"},{"name":"status:ready"}]`),
	}}
	newTicketNewReport = func(input gira.TicketNewInput) (gira.TicketNewReport, error) {
		return gira.BuildTicketNewReport(input, devCommandRunner)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "new", "Add CLI", "--repo", "StatPan/gira", "--label", "area:cli", "--dry-run"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout: %s stderr: %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "missing repo labels: area:cli") {
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

func TestExportDashboardCommandRequiresRepo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"export", "dashboard", "--dry-run"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "--repo is required") {
		t.Fatalf("stderr missing repo requirement:\n%s", stderr.String())
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
		"api repos/" + repo + "/milestones --paginate --slurp -X GET -f state=all -f per_page=100":                         `[[{"number":1,"title":"MVP","state":"open","description":"","due_on":null,"open_issues":1,"closed_issues":1}]]`,
		"issue list --repo " + repo + " --state all --limit 1000 --json number,title,state,labels,milestone,updatedAt,url": `[{"number":1,"title":"Issue 1","state":"OPEN","labels":[],"milestone":{"title":"MVP"},"updatedAt":"2026-04-25T12:00:00Z","url":"https://github.com/` + repo + `/issues/1"}]`,
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
	for _, want := range []string{`"repo": "StatPan/gira"`, `"command": "audit readiness"`, `"ready": false`, `"checked_at": "2026-05-08T12:00:00Z"`, `"audit": {`, `"next_step": "fix audit ledger corruption`} {
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
	for _, want := range []string{"readiness/doctor checks:", "audit ledger health:", "[warn] audit_ledger"} {
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
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 60 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": []byte(`[{"number":99,"title":"x","body":"Closes #60","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
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

func TestAdoptRepoJSONUsesInjectedReport(t *testing.T) {
	original := newAdoptRepoReport
	t.Cleanup(func() { newAdoptRepoReport = original })
	newAdoptRepoReport = func(input gira.AdoptRepoInput) (gira.AdoptRepoReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Path != "." || !input.DryRun || input.Apply {
			t.Fatalf("unexpected input: %+v", input)
		}
		return gira.AdoptRepoReport{Repo: input.Repo.FullName(), Path: "/repo", DryRun: true, Strategy: "merge", Recommendation: "merge", NextStep: "gira adopt repo --apply"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"adopt", "repo", "--repo", "StatPan/gira", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"strategy": "merge"`) || !strings.Contains(stdout.String(), `"next_step": "gira adopt repo --apply"`) {
		t.Fatalf("unexpected JSON: %s", stdout.String())
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

func TestReviewGateFailsWhenChecksFail(t *testing.T) {
	original := reviewGateRunner
	t.Cleanup(func() { reviewGateRunner = original })
	reviewGateRunner = devCLIRunner{outputs: map[string][]byte{
		"gofmt -l .":          []byte(""),
		"go test ./...":       []byte(""),
		"go test -race ./...": []byte(""),
	}, errs: map[string]error{"go vet ./...": fmt.Errorf("vet failed")}}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"review", "gate", "--json"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero when checks fail")
	}
	if !strings.Contains(stdout.String(), `"ready": false`) {
		t.Fatalf("expected readiness false output: %s", stdout.String())
	}
}

type weeklyDashClient struct {
	repo   gira.RepoRef
	issues []gira.DashboardRawIssue
}

func (c weeklyDashClient) Repo() gira.RepoRef                             { return c.repo }
func (c weeklyDashClient) FetchIssues() ([]gira.DashboardRawIssue, error) { return c.issues, nil }
func (c weeklyDashClient) FetchPullRequests() ([]gira.DashboardRawPullRequest, error) {
	return nil, nil
}
func (c weeklyDashClient) FetchMilestones() ([]gira.DashboardRawMilestone, error) { return nil, nil }
func (c weeklyDashClient) FetchProjectSnapshot() (gira.ProjectSyncSnapshot, error) {
	return gira.ProjectSyncSnapshot{}, nil
}
func (c weeklyDashClient) FetchTransitionSnapshot() (gira.ProjectTransitionSnapshot, error) {
	return gira.ProjectTransitionSnapshot{}, nil
}
func (c weeklyDashClient) FetchCapabilities() (gira.ProjectCapabilityReport, error) {
	return gira.ProjectCapabilityReport{}, nil
}

type weeklyReviewClient struct {
	repo   gira.RepoRef
	prs    []gira.ReviewPR
	issues []gira.ReviewIssue
}

func (c weeklyReviewClient) Repo() gira.RepoRef                          { return c.repo }
func (c weeklyReviewClient) ListOpenPRs() ([]gira.ReviewPR, error)       { return c.prs, nil }
func (c weeklyReviewClient) ListOpenIssues() ([]gira.ReviewIssue, error) { return c.issues, nil }
func (c weeklyReviewClient) MergePR(number int) error                    { return nil }
