package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultGlobalConfigRootUsesXDGConfigHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	root, err := DefaultGlobalConfigRoot()
	if err != nil {
		t.Fatalf("DefaultGlobalConfigRoot returned error: %v", err)
	}
	want := filepath.Join(dir, "gira")
	if root != want {
		t.Fatalf("root = %q, want %q", root, want)
	}
}

func TestGlobalRegistryPathsUseConfigRoot(t *testing.T) {
	root := t.TempDir()
	repo := ParseRepoRefMust("StatPan/gira")

	configPath, err := GlobalConfigPath(root)
	if err != nil {
		t.Fatalf("GlobalConfigPath returned error: %v", err)
	}
	if configPath != filepath.Join(root, "config.yaml") {
		t.Fatalf("config path = %q", configPath)
	}

	repoPath, err := GlobalRepoRegistryPath(root, repo)
	if err != nil {
		t.Fatalf("GlobalRepoRegistryPath returned error: %v", err)
	}
	if repoPath != filepath.Join(root, "repos", "StatPan", "gira.yaml") {
		t.Fatalf("repo path = %q", repoPath)
	}

	workspacePath, err := GlobalWorkspaceRegistryPath(root, "personal")
	if err != nil {
		t.Fatalf("GlobalWorkspaceRegistryPath returned error: %v", err)
	}
	if workspacePath != filepath.Join(root, "workspaces", "personal.yaml") {
		t.Fatalf("workspace path = %q", workspacePath)
	}
}

func TestGlobalWorkspaceRegistryPathRejectsUnsafeNames(t *testing.T) {
	for _, name := range []string{"", " ", ".", "..", "team/a", `team\a`} {
		if _, err := GlobalWorkspaceRegistryPath(t.TempDir(), name); err == nil {
			t.Fatalf("GlobalWorkspaceRegistryPath(%q) returned nil error", name)
		}
	}
}

func TestGlobalRegistrySchemaTypes(t *testing.T) {
	repo := GlobalRepoRegistryEntry{
		Repo:     "StatPan/gira",
		Path:     "~/workspace/apps/gira",
		Aliases:  []string{"gira"},
		Contract: ".gira/config.yaml",
		Defaults: GlobalDefaults{Agent: "codex", Assignee: "ilgukim", AgentLabels: []string{"agent:codex"}},
		Workspace: GlobalRepoWorkspaceRef{
			Name: "personal",
		},
	}
	if repo.Repo != "StatPan/gira" || repo.Workspace.Name != "personal" || repo.Contract == "" {
		t.Fatalf("unexpected repo registry schema fixture: %+v", repo)
	}

	workspace := GlobalWorkspaceRegistryEntry{
		Workspace: WorkspaceConfig{
			Name:      "personal",
			Owner:     "StatPan",
			InboxRepo: "StatPan/backlog",
			Repos:     []string{"StatPan/gira"},
		},
		Defaults: GlobalDefaults{Agent: "codex"},
	}
	if workspace.Workspace.InboxRepo != "StatPan/backlog" || workspace.Defaults.Agent != "codex" {
		t.Fatalf("unexpected workspace registry schema fixture: %+v", workspace)
	}
}

func TestLoadGlobalConfig(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "config.yaml"), `default_owner: StatPan
default_workspace: personal
inbox_repo: StatPan/backlog
defaults:
  agent: codex
  assignee: ilgukim
  agent_labels:
    - agent:codex
paths:
  cache_root: /tmp/gira-cache
  state_root: /tmp/gira-state
`)

	cfg, err := LoadGlobalConfig(root)
	if err != nil {
		t.Fatalf("LoadGlobalConfig returned error: %v", err)
	}
	if cfg.DefaultOwner != "StatPan" || cfg.DefaultWorkspace != "personal" || cfg.Defaults.Agent != "codex" {
		t.Fatalf("unexpected global config: %+v", cfg)
	}
}

func TestLoadGlobalRepoRegistryEntry(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "repos", "StatPan", "gira.yaml")
	writeTestFile(t, path, `repo: StatPan/gira
path: ~/workspace/apps/gira
aliases:
  - gira
contract: .gira/config.yaml
defaults:
  agent: codex
workspace:
  name: personal
`)

	entry, err := LoadGlobalRepoRegistryEntry(root, ParseRepoRefMust("StatPan/gira"))
	if err != nil {
		t.Fatalf("LoadGlobalRepoRegistryEntry returned error: %v", err)
	}
	if entry.Repo != "StatPan/gira" || entry.Path == "" || entry.Contract != ".gira/config.yaml" {
		t.Fatalf("unexpected repo registry entry: %+v", entry)
	}
}

func TestLoadGlobalWorkspaceRegistryEntry(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "workspaces", "personal.yaml")
	writeTestFile(t, path, `workspace:
  name: personal
  owner: StatPan
  inbox_repo: StatPan/backlog
  repos:
    - StatPan/gira
    - StatPan/docs
defaults:
  agent: codex
`)

	entry, err := LoadGlobalWorkspaceRegistryEntry(root, "personal")
	if err != nil {
		t.Fatalf("LoadGlobalWorkspaceRegistryEntry returned error: %v", err)
	}
	if entry.Workspace.Name != "personal" || len(entry.Workspace.Repos) != 2 || entry.Defaults.Agent != "codex" {
		t.Fatalf("unexpected workspace registry entry: %+v", entry)
	}
}

func TestLoadGlobalConfigMissingFile(t *testing.T) {
	_, err := LoadGlobalConfig(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "read global config") {
		t.Fatalf("error = %v, want missing file read error", err)
	}
}

func TestLoadGlobalConfigInvalidYAML(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "config.yaml"), "default_owner: [")

	_, err := LoadGlobalConfig(root)
	if err == nil || !strings.Contains(err.Error(), "parse global config") {
		t.Fatalf("error = %v, want parse error", err)
	}
}

func TestValidateGlobalRepoRegistryEntryInvalidRepo(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "repos", "StatPan", "gira.yaml"), `repo: StatPan
`)

	_, err := LoadGlobalRepoRegistryEntry(root, ParseRepoRefMust("StatPan/gira"))
	if err == nil || !strings.Contains(err.Error(), "repo must be in OWNER/REPO format") {
		t.Fatalf("error = %v, want invalid repo error", err)
	}
}

func TestValidateGlobalRepoRegistryEntryMalformedContractPath(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "repos", "StatPan", "gira.yaml"), `repo: StatPan/gira
contract: ../config.yaml
`)

	_, err := LoadGlobalRepoRegistryEntry(root, ParseRepoRefMust("StatPan/gira"))
	if err == nil || !strings.Contains(err.Error(), "contract must be a relative path inside the repo") {
		t.Fatalf("error = %v, want malformed contract path error", err)
	}
}

func TestValidateGlobalWorkspaceRegistryEntryDuplicateRepos(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "workspaces", "personal.yaml"), `workspace:
  name: personal
  inbox_repo: StatPan/backlog
  repos:
    - StatPan/gira
    - statpan/gira
`)

	_, err := LoadGlobalWorkspaceRegistryEntry(root, "personal")
	if err == nil || !strings.Contains(err.Error(), "duplicates") {
		t.Fatalf("error = %v, want duplicate repo error", err)
	}
}

func TestLoadGlobalWorkspaceRegistryEntryDefaultsNameFromPath(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "workspaces", "personal.yaml"), `workspace:
  inbox_repo: StatPan/backlog
  repos:
    - StatPan/gira
`)

	entry, err := LoadGlobalWorkspaceRegistryEntry(root, "personal")
	if err != nil {
		t.Fatalf("LoadGlobalWorkspaceRegistryEntry returned error: %v", err)
	}
	if entry.Workspace.Name != "personal" {
		t.Fatalf("workspace name = %q, want personal", entry.Workspace.Name)
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
