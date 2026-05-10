package gira

import (
	"path/filepath"
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
