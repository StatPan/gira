package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadInitConfigValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `repo: StatPan/gira
profiles:
  default:
    labels: ["type:task"]
    milestones: ["MVP"]
    issue_templates: ["feature"]
    review_policy:
      required_approvals: 1
      require_code_owners: true
portfolio:
  repo: StatPan/portfolio
  repos:
    - StatPan/gira
    - StatPan/docs
workspace:
  name: personal
  owner: StatPan
  inbox_repo: StatPan/backlog
  repos:
    - StatPan/gira
  project:
    owner: StatPan
    number: 7
    title: Gira
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := LoadInitConfig(path)
	if err != nil {
		t.Fatalf("LoadInitConfig error: %v", err)
	}
	if _, ok := cfg.Profiles["default"]; !ok {
		t.Fatalf("missing default profile: %+v", cfg)
	}
	if cfg.Repo != "StatPan/gira" {
		t.Fatalf("repo = %q, want StatPan/gira", cfg.Repo)
	}
	if cfg.Portfolio.Repo != "StatPan/portfolio" || len(cfg.Portfolio.Repos) != 2 {
		t.Fatalf("portfolio config = %+v, want repo and two execution repos", cfg.Portfolio)
	}
	if cfg.Workspace.InboxRepo != "StatPan/backlog" || len(cfg.Workspace.Repos) != 1 {
		t.Fatalf("workspace config = %+v, want inbox repo and one execution repo", cfg.Workspace)
	}
	if cfg.Workspace.Project.Owner != "StatPan" || cfg.Workspace.Project.Number != 7 {
		t.Fatalf("workspace project config = %+v", cfg.Workspace.Project)
	}
}

func TestLoadInitConfigInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `profiles:
  default:
    review_policy:
      required_approvals: -1
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, err := LoadInitConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
	if !strings.Contains(err.Error(), "required_approvals") {
		t.Fatalf("expected actionable error, got: %v", err)
	}
}

func TestLoadInitConfigInvalidRepo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `repo: bad-format
profiles:
  default:
    labels: ["type:task"]
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, err := LoadInitConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid repo")
	}
	if !strings.Contains(err.Error(), "repo must be in OWNER/REPO format") {
		t.Fatalf("expected actionable repo error, got: %v", err)
	}
}

func TestLoadInitConfigInvalidPortfolioRepo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `profiles:
  default:
    labels: ["type:task"]
portfolio:
  repo: bad-format
  repos:
    - StatPan/gira
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, err := LoadInitConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid portfolio repo")
	}
	if !strings.Contains(err.Error(), "portfolio.repo must be in OWNER/REPO format") {
		t.Fatalf("expected actionable portfolio repo error, got: %v", err)
	}
}

func TestLoadInitConfigInvalidWorkspaceRepo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `profiles:
  default:
    labels: ["type:task"]
workspace:
  inbox_repo: bad-format
  repos:
    - StatPan/gira
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, err := LoadInitConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid workspace inbox repo")
	}
	if !strings.Contains(err.Error(), "workspace.inbox_repo must be in OWNER/REPO format") {
		t.Fatalf("expected actionable workspace repo error, got: %v", err)
	}
}

func TestLoadInitConfigInvalidWorkspaceRepos(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `profiles:
  default:
    labels: ["type:task"]
workspace:
  inbox_repo: StatPan/backlog
  repos:
    - StatPan/gira
    - bad-format
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, err := LoadInitConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid workspace execution repo")
	}
	if !strings.Contains(err.Error(), "workspace.repos[1] must be in OWNER/REPO format") {
		t.Fatalf("expected actionable workspace repos error, got: %v", err)
	}
}

func TestLoadInitConfigInvalidWorkspaceProject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `profiles:
  default:
    labels: ["type:task"]
workspace:
  inbox_repo: StatPan/backlog
  repos:
    - StatPan/gira
  project:
    owner: StatPan
    number: 0
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, err := LoadInitConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid workspace project")
	}
	if !strings.Contains(err.Error(), "workspace.project.number must be > 0") {
		t.Fatalf("expected actionable workspace project error, got: %v", err)
	}
}

func TestLoadInitConfigInvalidPortfolioRepos(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `profiles:
  default:
    labels: ["type:task"]
portfolio:
  repo: StatPan/portfolio
  repos:
    - StatPan/gira
    - bad-format
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, err := LoadInitConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid portfolio execution repo")
	}
	if !strings.Contains(err.Error(), "portfolio.repos[1] must be in OWNER/REPO format") {
		t.Fatalf("expected actionable portfolio repos error, got: %v", err)
	}
}

func TestLoadInitConfigValidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[profiles.default]
labels = ["type:task"]
milestones = ["MVP"]
issue_templates = ["feature"]

[profiles.default.review_policy]
required_approvals = 1
require_code_owners = true
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := LoadInitConfig(path)
	if err != nil {
		t.Fatalf("LoadInitConfig error: %v", err)
	}
	if _, ok := cfg.Profiles["default"]; !ok {
		t.Fatalf("missing default profile: %+v", cfg)
	}
}

func TestLoadInitConfigInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[profiles.default.review_policy]
required_approvals = -1
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, err := LoadInitConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
	if !strings.Contains(err.Error(), "required_approvals") {
		t.Fatalf("expected actionable error, got: %v", err)
	}
}
