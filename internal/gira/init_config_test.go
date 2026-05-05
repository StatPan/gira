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
