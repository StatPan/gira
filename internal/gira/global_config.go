package gira

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	GlobalConfigFileName    = "config.yaml"
	GlobalReposDirName      = "repos"
	GlobalWorkspacesDirName = "workspaces"
)

type GlobalConfig struct {
	DefaultOwner     string             `yaml:"default_owner" toml:"default_owner" json:"default_owner,omitempty"`
	DefaultWorkspace string             `yaml:"default_workspace" toml:"default_workspace" json:"default_workspace,omitempty"`
	InboxRepo        string             `yaml:"inbox_repo" toml:"inbox_repo" json:"inbox_repo,omitempty"`
	Defaults         GlobalDefaults     `yaml:"defaults" toml:"defaults" json:"defaults,omitempty"`
	Output           GlobalOutputConfig `yaml:"output" toml:"output" json:"output,omitempty"`
	Paths            GlobalPathsConfig  `yaml:"paths" toml:"paths" json:"paths,omitempty"`
}

type GlobalDefaults struct {
	Agent       string   `yaml:"agent" toml:"agent" json:"agent,omitempty"`
	Assignee    string   `yaml:"assignee" toml:"assignee" json:"assignee,omitempty"`
	AgentLabels []string `yaml:"agent_labels" toml:"agent_labels" json:"agent_labels,omitempty"`
}

type GlobalOutputConfig struct {
	Format string `yaml:"format" toml:"format" json:"format,omitempty"`
	Color  string `yaml:"color" toml:"color" json:"color,omitempty"`
}

type GlobalPathsConfig struct {
	CacheRoot string `yaml:"cache_root" toml:"cache_root" json:"cache_root,omitempty"`
	StateRoot string `yaml:"state_root" toml:"state_root" json:"state_root,omitempty"`
}

type GlobalRepoRegistryEntry struct {
	Repo      string                 `yaml:"repo" toml:"repo" json:"repo"`
	Path      string                 `yaml:"path" toml:"path" json:"path,omitempty"`
	Aliases   []string               `yaml:"aliases" toml:"aliases" json:"aliases,omitempty"`
	Contract  string                 `yaml:"contract" toml:"contract" json:"contract,omitempty"`
	Defaults  GlobalDefaults         `yaml:"defaults" toml:"defaults" json:"defaults,omitempty"`
	Workspace GlobalRepoWorkspaceRef `yaml:"workspace" toml:"workspace" json:"workspace,omitempty"`
}

type GlobalRepoWorkspaceRef struct {
	Name string `yaml:"name" toml:"name" json:"name,omitempty"`
}

type GlobalWorkspaceRegistryEntry struct {
	Workspace WorkspaceConfig `yaml:"workspace" toml:"workspace" json:"workspace"`
	Defaults  GlobalDefaults  `yaml:"defaults" toml:"defaults" json:"defaults,omitempty"`
}

func DefaultGlobalConfigRoot() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(root, "gira"), nil
}

func GlobalConfigPath(configRoot string) (string, error) {
	root, err := globalConfigRoot(configRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, GlobalConfigFileName), nil
}

func GlobalReposRoot(configRoot string) (string, error) {
	root, err := globalConfigRoot(configRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, GlobalReposDirName), nil
}

func GlobalRepoRegistryPath(configRoot string, repo RepoRef) (string, error) {
	root, err := GlobalReposRoot(configRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, repo.Owner, repo.Name+".yaml"), nil
}

func GlobalWorkspacesRoot(configRoot string) (string, error) {
	root, err := globalConfigRoot(configRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, GlobalWorkspacesDirName), nil
}

func GlobalWorkspaceRegistryPath(configRoot string, name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if !isSafeRegistryName(trimmed) {
		return "", fmt.Errorf("workspace name must be non-empty and must not contain path separators")
	}
	root, err := GlobalWorkspacesRoot(configRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, trimmed+".yaml"), nil
}

func globalConfigRoot(configRoot string) (string, error) {
	if strings.TrimSpace(configRoot) != "" {
		return filepath.Clean(configRoot), nil
	}
	return DefaultGlobalConfigRoot()
}

func isSafeRegistryName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	return !strings.ContainsAny(name, `/\`)
}
