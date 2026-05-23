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
	if report.SchemaVersion != RepoRegisterReportSchemaVersion || report.Approval == nil {
		t.Fatalf("expected repo register schema and approval evidence: %+v", report)
	}
	expectedApply := "gira repo register StatPan/gira --path " + QuoteShellArg(filepath.Clean(checkout)) + " --config-root " + QuoteShellArg(report.ConfigRoot) + " --apply"
	if report.Approval.SchemaVersion != ApprovalPlanSchemaVersion || report.Approval.CanonicalCommand != "gira repo register" || report.Approval.OutputSchema != RepoRegisterReportSchemaVersion {
		t.Fatalf("unexpected repo register approval identity: %+v", report.Approval)
	}
	if report.Approval.ApplyCommand != expectedApply || report.Approval.PostApplyVerification != "gira config repo --repo StatPan/gira --config-root "+QuoteShellArg(report.ConfigRoot)+" --json" {
		t.Fatalf("unexpected repo register approval commands: %+v", report.Approval)
	}
	if report.Approval.Blockers == nil || report.Approval.Warnings == nil || !approvalHasAction(report.Approval.PlannedActions, "registry:create") {
		t.Fatalf("unexpected repo register approval plan: %+v", report.Approval)
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
	if report.SchemaVersion != RepoRegisterReportSchemaVersion || report.Approval != nil {
		t.Fatalf("apply report should have schema and omit dry-run approval: %+v", report)
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

func TestBuildRepoRegisterReportRejectsUnsafeContractPath(t *testing.T) {
	_, err := BuildRepoRegisterReport(RepoRegisterInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		Contract:   "../config.yaml",
		ConfigRoot: t.TempDir(),
		DryRun:     true,
	}, fakeRegisterRunner{})
	if err == nil || !strings.Contains(err.Error(), "contract must be a relative path inside the repo") {
		t.Fatalf("error = %v, want unsafe contract path", err)
	}
}

func TestBuildRepoRegisterReportInvalidExistingRegistryDeterministic(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "repos", "StatPan", "gira.yaml"), "repo: [")

	report, err := BuildRepoRegisterReport(RepoRegisterInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		ConfigRoot: root,
		DryRun:     true,
	}, fakeRegisterRunner{})
	if err == nil || !strings.Contains(err.Error(), "parse global config") {
		t.Fatalf("error = %v, want invalid registry parse error", err)
	}
	if report.Action != "conflict" || report.Status != "blocked" {
		t.Fatalf("report = %+v, want deterministic blocked conflict", report)
	}
	if report.Approval == nil || !containsCall(report.Approval.Blockers, "repo_registry_conflict") {
		t.Fatalf("blocked dry-run should include approval blockers: %+v", report.Approval)
	}
}

func TestFormatRepoRegisterReportTextContract(t *testing.T) {
	text := FormatRepoRegisterReport(RepoRegisterReport{
		Repo:     "StatPan/gira",
		File:     "/tmp/gira/repos/StatPan/gira.yaml",
		Status:   "planned",
		Action:   "create",
		Path:     "/repo",
		NextStep: "gira repo register StatPan/gira --apply",
	})
	for _, want := range []string{"repo register: planned StatPan/gira", "file: /tmp/gira/repos/StatPan/gira.yaml", "path: /repo", "next step: gira repo register StatPan/gira --apply"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q:\n%s", want, text)
		}
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
