package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildWorkspaceInitReportDryRunPersonalRepoBound(t *testing.T) {
	report, err := BuildWorkspaceInitReport(WorkspaceInitInput{
		InboxRepo: "StatPan/gira",
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("BuildWorkspaceInitReport error: %v", err)
	}
	if report.Workspace.Owner != "StatPan" || report.Workspace.Name != "gira" || report.InboxRepo != "StatPan/gira" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if len(report.Repos) != 1 || report.Repos[0] != "StatPan/gira" {
		t.Fatalf("repos = %+v, want repo-bound default", report.Repos)
	}
	if report.Project.Owner != "StatPan" || report.Project.Title != "gira" || report.Project.Number != 0 {
		t.Fatalf("project = %+v, want workspace owner/name defaults", report.Project)
	}
	for _, want := range []string{"workspace:", "name: \"gira\"", "inbox_repo: StatPan/gira", "project:", "owner: StatPan", "title: \"gira\""} {
		if !strings.Contains(report.Content, want) {
			t.Fatalf("content missing %q:\n%s", want, report.Content)
		}
	}
	if strings.Contains(report.Content, "number:") {
		t.Fatalf("content should omit project number by default:\n%s", report.Content)
	}
}

func TestBuildWorkspaceInitReportProjectOverrides(t *testing.T) {
	report, err := BuildWorkspaceInitReport(WorkspaceInitInput{
		Name:          "personal",
		Owner:         "StatPan",
		InboxRepo:     "StatPan/backlog",
		ProjectOwner:  "GiraOrg",
		ProjectTitle:  "Roadmap",
		ProjectNumber: 12,
		DryRun:        true,
	})
	if err != nil {
		t.Fatalf("BuildWorkspaceInitReport error: %v", err)
	}
	if report.Project.Owner != "GiraOrg" || report.Project.Title != "Roadmap" || report.Project.Number != 12 {
		t.Fatalf("project = %+v, want overrides", report.Project)
	}
	for _, want := range []string{"owner: GiraOrg", "title: \"Roadmap\"", "number: 12"} {
		if !strings.Contains(report.Content, want) {
			t.Fatalf("content missing %q:\n%s", want, report.Content)
		}
	}
}

func TestBuildWorkspaceInitReportQuotesFreeFormYAMLScalars(t *testing.T) {
	dir := t.TempDir()
	report, err := BuildWorkspaceInitReport(WorkspaceInitInput{
		Name:         `Personal: Q2 "Focus"`,
		Owner:        "StatPan",
		InboxRepo:    "StatPan/backlog",
		ProjectTitle: `Roadmap: Q2 "Focus"`,
		Path:         dir,
		Apply:        true,
	})
	if err != nil {
		t.Fatalf("BuildWorkspaceInitReport error: %v", err)
	}
	resolved, err := ResolveWorkspaceConfig(report.ConfigPath)
	if err != nil {
		t.Fatalf("ResolveWorkspaceConfig error: %v", err)
	}
	if resolved.Name != `Personal: Q2 "Focus"` || resolved.Project.Title != `Roadmap: Q2 "Focus"` {
		t.Fatalf("resolved = %+v, want quoted free-form scalar values", resolved)
	}
}

func TestBuildWorkspaceInitReportRejectsNegativeProjectNumber(t *testing.T) {
	_, err := BuildWorkspaceInitReport(WorkspaceInitInput{
		InboxRepo:     "StatPan/gira",
		ProjectNumber: -1,
		DryRun:        true,
	})
	if err == nil || !strings.Contains(err.Error(), "--project-number must be >= 0") {
		t.Fatalf("expected project number error, got %v", err)
	}
}

func TestFormatWorkspaceInitReportIncludesProject(t *testing.T) {
	report, err := BuildWorkspaceInitReport(WorkspaceInitInput{
		InboxRepo:     "StatPan/gira",
		ProjectNumber: 9,
		DryRun:        true,
	})
	if err != nil {
		t.Fatalf("BuildWorkspaceInitReport error: %v", err)
	}
	output := FormatWorkspaceInitReport(report)
	if !strings.Contains(output, "project: StatPan/gira #9") {
		t.Fatalf("formatted output missing project:\n%s", output)
	}
}

func TestBuildWorkspaceInitReportApplyWritesConfig(t *testing.T) {
	dir := t.TempDir()
	input := WorkspaceInitInput{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: "StatPan/backlog",
		Repos:     []string{"StatPan/gira", "StatPan/docs"},
		Path:      dir,
		Apply:     true,
	}
	report, err := BuildWorkspaceInitReport(input)
	if err != nil {
		t.Fatalf("BuildWorkspaceInitReport error: %v", err)
	}
	if !report.Created || !report.Applied {
		t.Fatalf("expected created/applied report: %+v", report)
	}
	resolved, err := ResolveWorkspaceConfig(filepath.Join(dir, ".gira", "config.yaml"))
	if err != nil {
		t.Fatalf("ResolveWorkspaceConfig error: %v", err)
	}
	if resolved.InboxRepo.FullName() != "StatPan/backlog" || len(resolved.Repos) != 2 {
		t.Fatalf("resolved = %+v", resolved)
	}
	if resolved.Project.Owner != "StatPan" || resolved.Project.Title != "personal" {
		t.Fatalf("resolved project = %+v, want workspace project defaults", resolved.Project)
	}
	if report.Project != resolved.Project {
		t.Fatalf("report project = %+v, resolved project = %+v", report.Project, resolved.Project)
	}
	if _, err := os.Stat(report.ConfigPath); err != nil {
		t.Fatalf("config path not written: %v", err)
	}

	second, err := BuildWorkspaceInitReport(input)
	if err != nil {
		t.Fatalf("second BuildWorkspaceInitReport error: %v", err)
	}
	if !second.Skipped || second.Applied || second.Created || second.Overwritten {
		t.Fatalf("expected idempotent repo skip: %+v", second)
	}
}

func TestBuildWorkspaceInitReportMergeDryRunShowsWorkspaceBlock(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".gira", "config.yaml")
	writeTestFile(t, configPath, "repo: StatPan/gira\nprofiles:\n  default:\n    labels:\n      - type:task\n")

	report, err := BuildWorkspaceInitReport(WorkspaceInitInput{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: "StatPan/backlog",
		Repos:     []string{"StatPan/gira"},
		Path:      dir,
		Merge:     true,
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("BuildWorkspaceInitReport error: %v", err)
	}
	if !report.Merge || report.Merged || report.Applied {
		t.Fatalf("unexpected merge dry-run report: %+v", report)
	}
	for _, want := range []string{"workspace:", "inbox_repo: StatPan/backlog", "profiles:", "- type:task"} {
		if !strings.Contains(report.Content, want) {
			t.Fatalf("merged content missing %q:\n%s", want, report.Content)
		}
	}
	output := FormatWorkspaceInitReport(report)
	if !strings.Contains(output, "workspace block:\nworkspace:") || !strings.Contains(output, "config:\nrepo: StatPan/gira") {
		t.Fatalf("formatted merge dry-run missing block/config:\n%s", output)
	}
	if got := readText(t, configPath); strings.Contains(got, "workspace:") {
		t.Fatalf("dry-run changed config:\n%s", got)
	}
}

func TestBuildWorkspaceInitReportMergeApplyPreservesRepoContract(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".gira", "config.yaml")
	writeTestFile(t, configPath, "repo: StatPan/gira\nprofiles:\n  default:\n    labels:\n      - type:task\n")

	report, err := BuildWorkspaceInitReport(WorkspaceInitInput{
		InboxRepo: "StatPan/backlog",
		Repos:     []string{"StatPan/gira"},
		Path:      dir,
		Merge:     true,
		Apply:     true,
	})
	if err != nil {
		t.Fatalf("BuildWorkspaceInitReport error: %v", err)
	}
	if !report.Merged || !report.Applied || report.Created || report.Overwritten {
		t.Fatalf("unexpected merge apply report: %+v", report)
	}
	config := readText(t, configPath)
	for _, want := range []string{"repo: StatPan/gira", "workspace:", "inbox_repo: StatPan/backlog", "profiles:", "- type:task"} {
		if !strings.Contains(config, want) {
			t.Fatalf("merged config missing %q:\n%s", want, config)
		}
	}
	resolved, err := ResolveWorkspaceConfig(configPath)
	if err != nil {
		t.Fatalf("ResolveWorkspaceConfig error: %v", err)
	}
	if resolved.InboxRepo.FullName() != "StatPan/backlog" || len(resolved.Repos) != 1 {
		t.Fatalf("resolved = %+v", resolved)
	}
}

func TestBuildWorkspaceInitReportMergeRejectsExistingWorkspace(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, ".gira", "config.yaml"), "repo: StatPan/gira\nworkspace:\n  inbox_repo: StatPan/backlog\n  repos:\n    - StatPan/gira\nprofiles:\n  default:\n    labels: []\n")

	_, err := BuildWorkspaceInitReport(WorkspaceInitInput{
		InboxRepo: "StatPan/other",
		Repos:     []string{"StatPan/other"},
		Path:      dir,
		Merge:     true,
		Apply:     true,
	})
	if err == nil || !strings.Contains(err.Error(), "already has workspace fields") {
		t.Fatalf("expected existing workspace conflict, got %v", err)
	}
}

func TestBuildWorkspaceInitReportGlobalDryRun(t *testing.T) {
	root := t.TempDir()
	report, err := BuildWorkspaceInitReport(WorkspaceInitInput{
		Name:       "personal",
		Owner:      "StatPan",
		InboxRepo:  "StatPan/backlog",
		Repos:      []string{"StatPan/gira"},
		Scope:      "global",
		ConfigRoot: root,
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("BuildWorkspaceInitReport error: %v", err)
	}
	wantPath := filepath.Join(root, "workspaces", "personal.yaml")
	if report.Scope != "global" || report.ConfigRoot != root || report.ConfigPath != wantPath {
		t.Fatalf("unexpected global report: %+v", report)
	}
	if _, err := os.Stat(wantPath); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote global workspace or stat failed: %v", err)
	}
	if strings.Contains(report.Content, "\nrepo:") || strings.Contains(report.Content, "profiles:") {
		t.Fatalf("global workspace content should not include repo contract fields:\n%s", report.Content)
	}
}

func TestBuildWorkspaceInitReportGlobalApplyWritesRegistry(t *testing.T) {
	root := t.TempDir()
	input := WorkspaceInitInput{
		Name:          "personal",
		Owner:         "StatPan",
		InboxRepo:     "StatPan/backlog",
		Repos:         []string{"StatPan/gira"},
		ProjectNumber: 7,
		Scope:         "global",
		ConfigRoot:    root,
		Apply:         true,
	}
	report, err := BuildWorkspaceInitReport(input)
	if err != nil {
		t.Fatalf("BuildWorkspaceInitReport error: %v", err)
	}
	if !report.Created || !report.Applied || report.Scope != "global" {
		t.Fatalf("expected global created/applied report: %+v", report)
	}
	entry, err := LoadGlobalWorkspaceRegistryEntry(root, "personal")
	if err != nil {
		t.Fatalf("LoadGlobalWorkspaceRegistryEntry error: %v", err)
	}
	if entry.Workspace.InboxRepo != "StatPan/backlog" || len(entry.Workspace.Repos) != 1 || entry.Workspace.Project.Number != 7 {
		t.Fatalf("unexpected global workspace entry: %+v", entry)
	}

	second, err := BuildWorkspaceInitReport(input)
	if err != nil {
		t.Fatalf("second BuildWorkspaceInitReport error: %v", err)
	}
	if !second.Skipped || second.Applied || second.Created || second.Overwritten {
		t.Fatalf("expected idempotent global skip: %+v", second)
	}
}

func TestBuildWorkspaceInitReportRejectsInvalidScope(t *testing.T) {
	_, err := BuildWorkspaceInitReport(WorkspaceInitInput{InboxRepo: "StatPan/gira", Scope: "user", DryRun: true})
	if err == nil || !strings.Contains(err.Error(), "--scope must be repo or global") {
		t.Fatalf("expected scope error, got %v", err)
	}
}

func TestBuildWorkspaceInitReportValidatesRepoAllowlist(t *testing.T) {
	_, err := BuildWorkspaceInitReport(WorkspaceInitInput{InboxRepo: "StatPan/gira", Repos: []string{"StatPan/gira", "StatPan/gira"}, DryRun: true})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate repo error, got %v", err)
	}
}
