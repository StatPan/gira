package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildConfigGlobalReportInvalidYAML(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "config.yaml"), "default_owner: [")
	if err := os.MkdirAll(filepath.Join(root, "repos"), 0o755); err != nil {
		t.Fatalf("mkdir repos: %v", err)
	}

	report, err := BuildConfigGlobalReport(root)
	if err != nil {
		t.Fatalf("BuildConfigGlobalReport error: %v", err)
	}
	if report.Command != "config global" || !report.Config.Exists || report.Config.Valid {
		t.Fatalf("unexpected config status: %+v", report)
	}
	if !strings.Contains(report.Config.Error, "parse global config") || !report.ReposRoot.Exists {
		t.Fatalf("report missing deterministic invalid YAML/root evidence: %+v", report)
	}
}

func TestBuildConfigRepoReportFromRepoLocalContract(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestFile(t, filepath.Join(".gira", "config.yaml"), "repo: StatPan/gira\n")
	root := t.TempDir()

	report, err := BuildConfigRepoReport("", root, fakeRegisterRunner{})
	if err != nil {
		t.Fatalf("BuildConfigRepoReport error: %v", err)
	}
	if report.Repo != "StatPan/gira" || report.Source != "repo_local_contract" || report.Detail != ".gira/config.yaml" {
		t.Fatalf("unexpected repo report: %+v", report)
	}
	if len(report.RepoContracts) != 2 || !report.RepoContracts[0].Exists || !report.RepoContracts[0].Valid {
		t.Fatalf("repo contracts = %+v, want valid yaml contract plus missing toml", report.RepoContracts)
	}
}

func TestBuildConfigRepoReportInvalidRegistryIsStatusNotError(t *testing.T) {
	t.Chdir(t.TempDir())
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "repos", "StatPan", "gira.yaml"), "repo: StatPan/gira\ncontract: ../config.yaml\n")

	report, err := BuildConfigRepoReport("StatPan/gira", root, fakeRegisterRunner{})
	if err != nil {
		t.Fatalf("BuildConfigRepoReport error: %v", err)
	}
	if report.Source != "explicit" || !report.GlobalRepo.Exists || report.GlobalRepo.Valid {
		t.Fatalf("unexpected repo report: %+v", report)
	}
	if !strings.Contains(report.GlobalRepo.Error, "contract must be a relative path inside the repo") {
		t.Fatalf("global repo error = %q", report.GlobalRepo.Error)
	}
}

func TestBuildConfigDoctorReportNextStepsForInvalidGlobalAndRepo(t *testing.T) {
	t.Chdir(t.TempDir())
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "config.yaml"), "default_owner: [")
	writeTestFile(t, filepath.Join(root, "repos", "StatPan", "gira.yaml"), "repo: StatPan/gira\ncontract: ../config.yaml\n")

	report, err := BuildConfigDoctorReport("StatPan/gira", root, fakeRegisterRunner{})
	if err != nil {
		t.Fatalf("BuildConfigDoctorReport error: %v", err)
	}
	joined := strings.Join(report.NextSteps, "\n")
	for _, want := range []string{"fix global config validation errors", "fix global repo registry validation errors"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("next steps missing %q: %+v", want, report.NextSteps)
		}
	}
	text := FormatConfigDoctorReport(report)
	if !strings.Contains(text, "config doctor:") || !strings.Contains(text, "next step: fix global repo registry validation errors") {
		t.Fatalf("formatted doctor report missing evidence:\n%s", text)
	}
}

func TestBuildConfigDoctorReportDefaultsNextStep(t *testing.T) {
	t.Chdir(t.TempDir())
	root := t.TempDir()

	report, err := BuildConfigDoctorReport("", root, fakeRegisterRunner{})
	if err != nil {
		t.Fatalf("BuildConfigDoctorReport error: %v", err)
	}
	if report.Source != "defaults" || len(report.NextSteps) != 1 {
		t.Fatalf("unexpected defaults doctor report: %+v", report)
	}
	if !strings.Contains(report.NextSteps[0], "pass --repo OWNER/REPO") {
		t.Fatalf("next steps = %+v", report.NextSteps)
	}
}
