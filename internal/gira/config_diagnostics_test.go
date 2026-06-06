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

func TestBuildConfigStorageReportClassifiesLocalSurfaces(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestFile(t, filepath.Join(".gira", "config.yaml"), "repo: StatPan/gira\n")
	root := t.TempDir()
	cacheRoot := filepath.Join(t.TempDir(), "cache")
	stateRoot := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(filepath.Join(cacheRoot, "workspace-status"), 0o755); err != nil {
		t.Fatalf("mkdir cache: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(stateRoot, "runs"), 0o755); err != nil {
		t.Fatalf("mkdir state: %v", err)
	}
	writeTestFile(t, filepath.Join(root, "config.yaml"), "paths:\n  cache_root: "+filepath.ToSlash(cacheRoot)+"\n  state_root: "+filepath.ToSlash(stateRoot)+"\n")
	writeTestFile(t, filepath.Join(root, "repos", "StatPan", "gira.yaml"), "repo: StatPan/gira\npath: /workspace/gira\n")

	report, err := BuildConfigStorageReport("StatPan/gira", root, fakeRegisterRunner{})
	if err != nil {
		t.Fatalf("BuildConfigStorageReport error: %v", err)
	}
	if report.SchemaVersion != ConfigStorageReportSchemaVersion || report.Command != "config storage" || report.Repo != "StatPan/gira" {
		t.Fatalf("unexpected storage report header: %+v", report)
	}
	state := findStorageSurface(t, report, "runtime_state_root")
	if state.Path != stateRoot || state.PathSource != "global_config.paths.state_root" || state.Durability != "private_runtime_state" || state.SourceOfTruth == "github" {
		t.Fatalf("unexpected state surface: %+v", state)
	}
	cache := findStorageSurface(t, report, "workspace_status_cache")
	if cache.Path != filepath.Join(cacheRoot, "workspace-status") || !cache.Exists || cache.Durability != "disposable_cache" {
		t.Fatalf("unexpected workspace cache surface: %+v", cache)
	}
	contract := findStorageSurface(t, report, "repo_local_contract")
	if contract.Path != ".gira/config.yaml" || !contract.Exists || contract.Durability != "shared_repo_contract" {
		t.Fatalf("unexpected repo contract surface: %+v", contract)
	}
	export := findStorageSurface(t, report, "dashboard_export_bundle")
	if export.Durability != "regenerable_export" || export.SourceOfTruth != "github_and_gira_computed_state" {
		t.Fatalf("unexpected export surface: %+v", export)
	}
	text := FormatConfigStorageReport(report)
	for _, want := range []string{"config storage:", "runtime_state_root", "workspace_status_cache", "durability=private_runtime_state", "dashboard_export_bundle"} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted storage report missing %q:\n%s", want, text)
		}
	}
}

func TestBuildConfigStorageReportWarnsWhenGlobalConfigInvalid(t *testing.T) {
	t.Chdir(t.TempDir())
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "config.yaml"), "default_owner: [")

	report, err := BuildConfigStorageReport("", root, fakeRegisterRunner{})
	if err != nil {
		t.Fatalf("BuildConfigStorageReport error: %v", err)
	}
	if len(report.Warnings) != 1 || report.Warnings[0].Code != "invalid_global_config" {
		t.Fatalf("warnings = %+v, want invalid_global_config", report.Warnings)
	}
	state := findStorageSurface(t, report, "runtime_state_root")
	if state.Path != filepath.Join(root, "state") || state.PathSource != "config_root_state_default" {
		t.Fatalf("state root should fall back under config root when config is invalid: %+v", state)
	}
}

func findStorageSurface(t *testing.T, report ConfigStorageReport, name string) ConfigStorageSurface {
	t.Helper()
	for _, surface := range report.Surfaces {
		if surface.Name == name {
			return surface
		}
	}
	t.Fatalf("missing storage surface %q in %+v", name, report.Surfaces)
	return ConfigStorageSurface{}
}
