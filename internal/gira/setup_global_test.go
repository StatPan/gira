package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildSetupGlobalReportDryRunGlobalOnly(t *testing.T) {
	checkout := t.TempDir()
	writeTestFile(t, filepath.Join(checkout, ".gira", "config.yaml"), "repo: StatPan/gira\n")
	root := t.TempDir()

	report, err := BuildSetupGlobalReport(SetupGlobalInput{
		Repo:          ParseRepoRefMust("StatPan/gira"),
		Path:          checkout,
		ConfigRoot:    root,
		WorkspaceName: "personal",
		InboxRepo:     "StatPan/gira",
		Agent:         "codex",
		Assignee:      "ilgukim",
		DryRun:        true,
	}, fakeRegisterRunner{origin: "git@github.com:StatPan/gira.git"})
	if err != nil {
		t.Fatalf("BuildSetupGlobalReport returned error: %v", err)
	}
	if report.Status != "planned" || report.Mode != SetupGlobalModeGlobalOnly {
		t.Fatalf("unexpected report status/mode: %+v", report)
	}
	if !report.RepoContract.Exists || report.GlobalRepo.Contract != "" {
		t.Fatalf("expected detected but unreferenced contract: %+v", report)
	}
	if len(report.Files) != 3 {
		t.Fatalf("files = %d, want 3", len(report.Files))
	}
	for _, plan := range report.Files {
		if plan.Action != "create" {
			t.Fatalf("plan action = %q, want create: %+v", plan.Action, plan)
		}
		if _, err := os.Stat(plan.Path); !os.IsNotExist(err) {
			t.Fatalf("dry-run wrote %s or unexpected stat error: %v", plan.Path, err)
		}
	}
	if !strings.Contains(strings.Join(report.Notes, "\n"), "global-only mode does not reference") {
		t.Fatalf("notes should explain ignored repo-local contract: %+v", report.Notes)
	}
}

func TestBuildSetupGlobalReportApplyHybrid(t *testing.T) {
	checkout := t.TempDir()
	writeTestFile(t, filepath.Join(checkout, ".gira", "config.yaml"), "repo: StatPan/gira\n")
	root := t.TempDir()

	report, err := BuildSetupGlobalReport(SetupGlobalInput{
		Repo:          ParseRepoRefMust("StatPan/gira"),
		Path:          checkout,
		ConfigRoot:    root,
		WorkspaceName: "personal",
		InboxRepo:     "StatPan/gira",
		Mode:          "hybrid",
		Agent:         "codex",
		Assignee:      "ilgukim",
		Apply:         true,
	}, fakeRegisterRunner{origin: "https://github.com/StatPan/gira.git"})
	if err != nil {
		t.Fatalf("BuildSetupGlobalReport apply returned error: %v", err)
	}
	if !report.Applied || report.Status != "applied" {
		t.Fatalf("unexpected apply report: %+v", report)
	}
	cfg, err := LoadGlobalConfig(root)
	if err != nil {
		t.Fatalf("LoadGlobalConfig returned error: %v", err)
	}
	if cfg.DefaultWorkspace != "personal" || cfg.InboxRepo != "StatPan/gira" || cfg.Defaults.Agent != "codex" {
		t.Fatalf("unexpected global config: %+v", cfg)
	}
	repoEntry, err := LoadGlobalRepoRegistryEntry(root, ParseRepoRefMust("StatPan/gira"))
	if err != nil {
		t.Fatalf("LoadGlobalRepoRegistryEntry returned error: %v", err)
	}
	if repoEntry.Contract != ".gira/config.yaml" || repoEntry.Workspace.Name != "personal" {
		t.Fatalf("unexpected repo entry: %+v", repoEntry)
	}
	workspace, err := LoadGlobalWorkspaceRegistryEntry(root, "personal")
	if err != nil {
		t.Fatalf("LoadGlobalWorkspaceRegistryEntry returned error: %v", err)
	}
	if workspace.Workspace.Owner != "StatPan" || workspace.Workspace.Repos[0] != "StatPan/gira" {
		t.Fatalf("unexpected workspace entry: %+v", workspace)
	}
}

func TestBuildSetupGlobalReportConflictAndOverwrite(t *testing.T) {
	checkout := t.TempDir()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "config.yaml"), "default_owner: Other\n")
	input := SetupGlobalInput{
		Repo:          ParseRepoRefMust("StatPan/gira"),
		Path:          checkout,
		ConfigRoot:    root,
		WorkspaceName: "personal",
		DryRun:        true,
	}
	runner := fakeRegisterRunner{origin: "https://github.com/StatPan/gira.git"}

	report, err := BuildSetupGlobalReport(input, runner)
	if err != nil {
		t.Fatalf("dry-run conflict should not error: %v", err)
	}
	if report.Status != "blocked" || report.Files[0].Action != "conflict" {
		t.Fatalf("expected blocked conflict report: %+v", report)
	}

	input.DryRun = false
	input.Apply = true
	_, err = BuildSetupGlobalReport(input, runner)
	if err == nil || !strings.Contains(err.Error(), "would overwrite existing files") {
		t.Fatalf("error = %v, want overwrite conflict", err)
	}

	input.Overwrite = true
	report, err = BuildSetupGlobalReport(input, runner)
	if err != nil {
		t.Fatalf("overwrite apply returned error: %v", err)
	}
	if report.Status != "applied" || report.Files[0].Action != "overwrite" {
		t.Fatalf("unexpected overwrite report: %+v", report)
	}
}
