package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildRepoMigrateReportDryRunPreservesContract(t *testing.T) {
	repo := testRepoWithContract(t, "repo: StatPan/gira\nworkspace:\n  name: personal\n")
	root := t.TempDir()

	report, err := BuildRepoMigrateReport(RepoMigrateInput{Path: repo, ConfigRoot: root, DryRun: true}, fakeRegisterRunner{origin: "https://github.com/StatPan/gira.git"})
	if err != nil {
		t.Fatalf("BuildRepoMigrateReport returned error: %v", err)
	}
	if report.Repo != "StatPan/gira" || report.Entry.Contract != ".gira/config.yaml" || report.Entry.Workspace.Name != "personal" {
		t.Fatalf("unexpected migrate report: %+v", report)
	}
	if _, err := os.Stat(report.File); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote registry file or unexpected stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".gira", "config.yaml")); err != nil {
		t.Fatalf("repo-local contract was not preserved: %v", err)
	}
}

func TestBuildRepoMigrateReportApplyAndAlreadyRegistered(t *testing.T) {
	repo := testRepoWithContract(t, "repo: StatPan/gira\n")
	root := t.TempDir()
	input := RepoMigrateInput{Path: repo, ConfigRoot: root, Apply: true}

	report, err := BuildRepoMigrateReport(input, fakeRegisterRunner{origin: "https://github.com/StatPan/gira.git"})
	if err != nil {
		t.Fatalf("BuildRepoMigrateReport apply returned error: %v", err)
	}
	if !report.Applied || report.Entry.Contract != ".gira/config.yaml" {
		t.Fatalf("unexpected apply report: %+v", report)
	}

	second, err := BuildRepoMigrateReport(input, fakeRegisterRunner{origin: "https://github.com/StatPan/gira.git"})
	if err != nil {
		t.Fatalf("second BuildRepoMigrateReport returned error: %v", err)
	}
	if second.Action != "skip" || second.Status != "skipped" {
		t.Fatalf("expected already-registered skip, got %+v", second)
	}
}

func TestBuildRepoMigrateReportNoRepoLocalConfig(t *testing.T) {
	_, err := BuildRepoMigrateReport(RepoMigrateInput{Path: t.TempDir(), ConfigRoot: t.TempDir(), DryRun: true}, fakeRegisterRunner{})
	if err == nil || !strings.Contains(err.Error(), "repo-local contract") {
		t.Fatalf("error = %v, want missing repo-local contract", err)
	}
}

func TestBuildRepoMigrateReportAlreadyRegisteredConflict(t *testing.T) {
	repo := testRepoWithContract(t, "repo: StatPan/gira\n")
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "repos", "StatPan", "gira.yaml"), "repo: StatPan/gira\npath: /other\ncontract: .gira/config.yaml\n")

	_, err := BuildRepoMigrateReport(RepoMigrateInput{Path: repo, ConfigRoot: root, Apply: true}, fakeRegisterRunner{origin: "https://github.com/StatPan/gira.git"})
	if err == nil || !strings.Contains(err.Error(), "already exists with different content") {
		t.Fatalf("error = %v, want conflict", err)
	}
}

func testRepoWithContract(t *testing.T, content string) string {
	t.Helper()
	repo := t.TempDir()
	writeTestFile(t, filepath.Join(repo, ".gira", "config.yaml"), content)
	return repo
}
