package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildRepoRegisterReportDryRun(t *testing.T) {
	checkout := t.TempDir()
	runner := fakeRegisterRunner{origin: "git@github.com:StatPan/gira.git"}

	report, err := BuildRepoRegisterReport(RepoRegisterInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		Path:       checkout,
		ConfigRoot: t.TempDir(),
		DryRun:     true,
	}, runner)
	if err != nil {
		t.Fatalf("BuildRepoRegisterReport returned error: %v", err)
	}
	if report.Action != "create" || report.Status != "planned" || report.File == "" {
		t.Fatalf("unexpected dry-run report: %+v", report)
	}
	if _, err := os.Stat(report.File); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote file or unexpected stat error: %v", err)
	}
}

func TestBuildRepoRegisterReportApplyAndIdempotent(t *testing.T) {
	checkout := t.TempDir()
	root := t.TempDir()
	runner := fakeRegisterRunner{origin: "https://github.com/StatPan/gira.git"}
	input := RepoRegisterInput{Repo: ParseRepoRefMust("StatPan/gira"), Path: checkout, ConfigRoot: root, Apply: true}

	report, err := BuildRepoRegisterReport(input, runner)
	if err != nil {
		t.Fatalf("BuildRepoRegisterReport apply returned error: %v", err)
	}
	if !report.Applied || report.Status != "applied" {
		t.Fatalf("unexpected apply report: %+v", report)
	}
	loaded, err := LoadGlobalRepoRegistryEntry(root, ParseRepoRefMust("StatPan/gira"))
	if err != nil {
		t.Fatalf("LoadGlobalRepoRegistryEntry returned error: %v", err)
	}
	if loaded.Path != filepath.Clean(checkout) {
		t.Fatalf("loaded path = %q, want %q", loaded.Path, filepath.Clean(checkout))
	}

	second, err := BuildRepoRegisterReport(input, runner)
	if err != nil {
		t.Fatalf("second BuildRepoRegisterReport returned error: %v", err)
	}
	if second.Action != "skip" || second.Status != "skipped" {
		t.Fatalf("expected idempotent skip, got %+v", second)
	}
}

func TestBuildRepoRegisterReportConflictAndOverwrite(t *testing.T) {
	checkout := t.TempDir()
	root := t.TempDir()
	path := filepath.Join(root, "repos", "StatPan", "gira.yaml")
	writeTestFile(t, path, "repo: StatPan/gira\npath: /other\n")
	runner := fakeRegisterRunner{origin: "https://github.com/StatPan/gira.git"}

	_, err := BuildRepoRegisterReport(RepoRegisterInput{Repo: ParseRepoRefMust("StatPan/gira"), Path: checkout, ConfigRoot: root, Apply: true}, runner)
	if err == nil || !strings.Contains(err.Error(), "already exists with different content") {
		t.Fatalf("error = %v, want conflict", err)
	}

	report, err := BuildRepoRegisterReport(RepoRegisterInput{Repo: ParseRepoRefMust("StatPan/gira"), Path: checkout, ConfigRoot: root, Overwrite: true, Apply: true}, runner)
	if err != nil {
		t.Fatalf("overwrite returned error: %v", err)
	}
	if report.Action != "overwrite" || !report.Applied {
		t.Fatalf("unexpected overwrite report: %+v", report)
	}
}

func TestBuildRepoRegisterReportRejectsMismatchedCheckout(t *testing.T) {
	_, err := BuildRepoRegisterReport(RepoRegisterInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		Path:       t.TempDir(),
		ConfigRoot: t.TempDir(),
		DryRun:     true,
	}, fakeRegisterRunner{origin: "https://github.com/Other/repo.git"})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("error = %v, want origin mismatch", err)
	}
}

type fakeRegisterRunner struct {
	origin string
}

func (f fakeRegisterRunner) Run(name string, args ...string) ([]byte, error) {
	if name == "git" && len(args) == 5 && args[0] == "-C" && args[2] == "remote" && args[3] == "get-url" && args[4] == "origin" {
		return []byte(f.origin + "\n"), nil
	}
	return nil, os.ErrNotExist
}
