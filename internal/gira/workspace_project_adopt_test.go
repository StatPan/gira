package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceProjectAdoptDryRunDoesNotWriteConfig(t *testing.T) {
	path := writeWorkspaceProjectAdoptConfig(t, `profiles:
  default:
    labels: ["type:task"]
workspace:
  name: personal
  owner: StatPan
  inbox_repo: StatPan/backlog
  repos:
    - StatPan/gira
`)
	before := readFileString(t, path)
	client := &fakeProjectsSyncClient{projects: []ProjectsSyncProject{{ID: "p1", Owner: "StatPan", Number: 7, Title: "Gira Board", URL: "https://github.com/users/StatPan/projects/7"}}}

	report, err := BuildWorkspaceProjectAdoptReport(WorkspaceProjectAdoptInput{ConfigPath: path, Owner: "StatPan", Title: "Gira Board", DryRun: true}, client)
	if err != nil {
		t.Fatalf("BuildWorkspaceProjectAdoptReport error: %v", err)
	}
	if report.Action.Status != "planned" || report.Project.Number != 7 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if got := readFileString(t, path); got != before {
		t.Fatalf("dry-run changed config:\nbefore:\n%s\nafter:\n%s", before, got)
	}
	if !strings.Contains(report.NextSteps[0], "gira projects sync --config "+path+" --dry-run") {
		t.Fatalf("next step missing projects sync dry-run: %+v", report.NextSteps)
	}
}

func TestWorkspaceProjectAdoptApplyWritesWorkspaceProjectYAML(t *testing.T) {
	path := writeWorkspaceProjectAdoptConfig(t, `repo: StatPan/gira
profiles:
  default:
    labels: ["type:task"]
workspace:
  name: personal
  owner: StatPan
  inbox_repo: StatPan/backlog
  repos:
    - StatPan/gira
`)
	client := &fakeProjectsSyncClient{projects: []ProjectsSyncProject{{ID: "p1", Owner: "StatPan", Number: 7, Title: "Gira Board"}}}

	report, err := BuildWorkspaceProjectAdoptReport(WorkspaceProjectAdoptInput{ConfigPath: path, Owner: "StatPan", Title: "Gira Board", Apply: true}, client)
	if err != nil {
		t.Fatalf("BuildWorkspaceProjectAdoptReport error: %v", err)
	}
	if report.Action.Status != "applied" {
		t.Fatalf("action status = %q, want applied", report.Action.Status)
	}
	content := readFileString(t, path)
	for _, want := range []string{"repo: StatPan/gira", "project:", "owner: StatPan", "title: Gira Board"} {
		if !strings.Contains(content, want) {
			t.Fatalf("config missing %q:\n%s", want, content)
		}
	}
	cfg, err := LoadInitConfig(path)
	if err != nil {
		t.Fatalf("LoadInitConfig after apply: %v", err)
	}
	if cfg.Workspace.Project.Owner != "StatPan" || cfg.Workspace.Project.Title != "Gira Board" || cfg.Workspace.Project.Number != 0 {
		t.Fatalf("workspace.project = %+v", cfg.Workspace.Project)
	}
}

func TestWorkspaceProjectAdoptApplyWritesNumberWhenResolvedByNumber(t *testing.T) {
	path := writeWorkspaceProjectAdoptConfig(t, `profiles:
  default:
    labels: ["type:task"]
workspace:
  inbox_repo: StatPan/backlog
  repos:
    - StatPan/gira
`)
	client := &fakeProjectsSyncClient{project: ProjectsSyncProject{ID: "p1", Owner: "StatPan", Number: 7, Title: "Gira Board"}}

	_, err := BuildWorkspaceProjectAdoptReport(WorkspaceProjectAdoptInput{ConfigPath: path, Owner: "StatPan", Number: 7, Apply: true}, client)
	if err != nil {
		t.Fatalf("BuildWorkspaceProjectAdoptReport error: %v", err)
	}
	cfg, err := LoadInitConfig(path)
	if err != nil {
		t.Fatalf("LoadInitConfig after apply: %v", err)
	}
	if cfg.Workspace.Project.Owner != "StatPan" || cfg.Workspace.Project.Number != 7 || cfg.Workspace.Project.Title != "" {
		t.Fatalf("workspace.project = %+v", cfg.Workspace.Project)
	}
}

func TestWorkspaceProjectAdoptExistingMatchingProjectSkips(t *testing.T) {
	path := writeWorkspaceProjectAdoptConfig(t, `profiles:
  default:
    labels: ["type:task"]
workspace:
  inbox_repo: StatPan/backlog
  repos:
    - StatPan/gira
  project:
    owner: StatPan
    title: Gira Board
`)
	client := &fakeProjectsSyncClient{projects: []ProjectsSyncProject{{ID: "p1", Owner: "StatPan", Number: 7, Title: "Gira Board"}}}

	report, err := BuildWorkspaceProjectAdoptReport(WorkspaceProjectAdoptInput{ConfigPath: path, Owner: "StatPan", Title: "Gira Board", Apply: true}, client)
	if err != nil {
		t.Fatalf("BuildWorkspaceProjectAdoptReport error: %v", err)
	}
	if report.Action.Action != "workspace.project:skip" || report.Action.Status != "skipped" {
		t.Fatalf("unexpected skip report: %+v", report.Action)
	}
}

func TestWorkspaceProjectAdoptNumberDisambiguatesMatchingTitleConfig(t *testing.T) {
	path := writeWorkspaceProjectAdoptConfig(t, `profiles:
  default:
    labels: ["type:task"]
workspace:
  inbox_repo: StatPan/backlog
  repos:
    - StatPan/gira
  project:
    owner: StatPan
    title: Gira Board
`)
	client := &fakeProjectsSyncClient{project: ProjectsSyncProject{ID: "p1", Owner: "StatPan", Number: 7, Title: "Gira Board"}}

	report, err := BuildWorkspaceProjectAdoptReport(WorkspaceProjectAdoptInput{ConfigPath: path, Owner: "StatPan", Number: 7, Apply: true}, client)
	if err != nil {
		t.Fatalf("BuildWorkspaceProjectAdoptReport error: %v", err)
	}
	if report.Action.Action != "workspace.project:update" || report.Action.Status != "applied" {
		t.Fatalf("unexpected update report: %+v", report.Action)
	}
	cfg, err := LoadInitConfig(path)
	if err != nil {
		t.Fatalf("LoadInitConfig after apply: %v", err)
	}
	if cfg.Workspace.Project.Number != 7 || cfg.Workspace.Project.Title != "Gira Board" {
		t.Fatalf("workspace.project = %+v, want title preserved with number disambiguation", cfg.Workspace.Project)
	}
}

func TestWorkspaceProjectAdoptExistingDifferentProjectFails(t *testing.T) {
	path := writeWorkspaceProjectAdoptConfig(t, `profiles:
  default:
    labels: ["type:task"]
workspace:
  inbox_repo: StatPan/backlog
  repos:
    - StatPan/gira
  project:
    owner: StatPan
    title: Other Board
`)
	client := &fakeProjectsSyncClient{projects: []ProjectsSyncProject{{ID: "p1", Owner: "StatPan", Number: 7, Title: "Gira Board"}}}

	_, err := BuildWorkspaceProjectAdoptReport(WorkspaceProjectAdoptInput{ConfigPath: path, Owner: "StatPan", Title: "Gira Board", Apply: true}, client)
	if err == nil {
		t.Fatal("expected different project error")
	}
	if !strings.Contains(err.Error(), "replace is not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkspaceProjectAdoptApplyRequiresValidWorkspaceConfig(t *testing.T) {
	path := writeWorkspaceProjectAdoptConfig(t, `profiles:
  default:
    labels: ["type:task"]
`)
	before := readFileString(t, path)
	client := &fakeProjectsSyncClient{projects: []ProjectsSyncProject{{ID: "p1", Owner: "StatPan", Number: 7, Title: "Gira Board"}}}

	_, err := BuildWorkspaceProjectAdoptReport(WorkspaceProjectAdoptInput{ConfigPath: path, Owner: "StatPan", Title: "Gira Board", Apply: true}, client)
	if err == nil {
		t.Fatal("expected valid workspace config error")
	}
	if !strings.Contains(err.Error(), "existing valid workspace config") {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := readFileString(t, path); got != before {
		t.Fatalf("invalid workspace config was mutated:\nbefore:\n%s\nafter:\n%s", before, got)
	}
}

func TestWorkspaceProjectAdoptApplyTOMLUnsupported(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `[profiles.default]
labels = ["type:task"]

[workspace]
inbox_repo = "StatPan/backlog"
repos = ["StatPan/gira"]
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	client := &fakeProjectsSyncClient{projects: []ProjectsSyncProject{{ID: "p1", Owner: "StatPan", Number: 7, Title: "Gira Board"}}}

	_, err := BuildWorkspaceProjectAdoptReport(WorkspaceProjectAdoptInput{ConfigPath: path, Owner: "StatPan", Title: "Gira Board", Apply: true}, client)
	if err == nil {
		t.Fatal("expected TOML unsupported error")
	}
	if !strings.Contains(err.Error(), "does not support TOML") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeWorkspaceProjectAdoptConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	return string(content)
}
