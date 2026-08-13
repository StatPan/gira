package gira

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
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
	Repo           string                 `yaml:"repo" toml:"repo" json:"repo"`
	Path           string                 `yaml:"path" toml:"path" json:"path,omitempty"`
	Aliases        []string               `yaml:"aliases" toml:"aliases" json:"aliases,omitempty"`
	Contract       string                 `yaml:"contract" toml:"contract" json:"contract,omitempty"`
	Defaults       GlobalDefaults         `yaml:"defaults" toml:"defaults" json:"defaults,omitempty"`
	Workspace      GlobalRepoWorkspaceRef `yaml:"workspace" toml:"workspace" json:"workspace,omitempty"`
	OperationMode  string                 `yaml:"operation_mode,omitempty" toml:"operation_mode" json:"operation_mode,omitempty"`
	DeliveryPolicy string                 `yaml:"delivery_policy,omitempty" toml:"delivery_policy" json:"delivery_policy,omitempty"`
	BranchPolicy   *BranchPolicyConfig    `yaml:"branch_policy,omitempty" toml:"branch_policy,omitempty" json:"branch_policy,omitempty"`
	Providers      *GlobalProvidersConfig `yaml:"providers,omitempty" toml:"providers,omitempty" json:"providers,omitempty"`
}

type GlobalProvidersConfig struct {
	Jira *JiraProviderConfig `yaml:"jira,omitempty" toml:"jira,omitempty" json:"jira,omitempty"`
}

type GlobalRepoWorkspaceRef struct {
	Name string `yaml:"name" toml:"name" json:"name,omitempty"`
}

type GlobalWorkspaceRegistryEntry struct {
	Workspace      WorkspaceConfig     `yaml:"workspace" toml:"workspace" json:"workspace"`
	Defaults       GlobalDefaults      `yaml:"defaults" toml:"defaults" json:"defaults,omitempty"`
	OperationMode  string              `yaml:"operation_mode" toml:"operation_mode" json:"operation_mode,omitempty"`
	DeliveryPolicy string              `yaml:"delivery_policy" toml:"delivery_policy" json:"delivery_policy,omitempty"`
	BranchPolicy   *BranchPolicyConfig `yaml:"branch_policy,omitempty" toml:"branch_policy,omitempty" json:"branch_policy,omitempty"`
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

func LoadGlobalConfig(configRoot string) (GlobalConfig, error) {
	path, err := GlobalConfigPath(configRoot)
	if err != nil {
		return GlobalConfig{}, err
	}
	var cfg GlobalConfig
	if err := readGlobalYAML(path, &cfg); err != nil {
		return GlobalConfig{}, err
	}
	if err := ValidateGlobalConfig(cfg, path); err != nil {
		return GlobalConfig{}, err
	}
	return cfg, nil
}

func LoadGlobalRepoRegistryEntry(configRoot string, repo RepoRef) (GlobalRepoRegistryEntry, error) {
	path, err := GlobalRepoRegistryPath(configRoot, repo)
	if err != nil {
		return GlobalRepoRegistryEntry{}, err
	}
	var entry GlobalRepoRegistryEntry
	if err := readGlobalYAML(path, &entry); err != nil {
		return GlobalRepoRegistryEntry{}, err
	}
	if err := ValidateGlobalRepoRegistryEntry(entry, repo, path); err != nil {
		return GlobalRepoRegistryEntry{}, err
	}
	return entry, nil
}

func LoadGlobalWorkspaceRegistryEntry(configRoot string, name string) (GlobalWorkspaceRegistryEntry, error) {
	path, err := GlobalWorkspaceRegistryPath(configRoot, name)
	if err != nil {
		return GlobalWorkspaceRegistryEntry{}, err
	}
	var entry GlobalWorkspaceRegistryEntry
	if err := readGlobalYAML(path, &entry); err != nil {
		return GlobalWorkspaceRegistryEntry{}, err
	}
	if strings.TrimSpace(entry.Workspace.Name) == "" {
		entry.Workspace.Name = strings.TrimSpace(name)
	}
	if err := ValidateGlobalWorkspaceRegistryEntry(entry, strings.TrimSpace(name), path); err != nil {
		return GlobalWorkspaceRegistryEntry{}, err
	}
	return entry, nil
}

func ValidateGlobalConfig(cfg GlobalConfig, source string) error {
	if strings.TrimSpace(cfg.DefaultWorkspace) != "" && !isSafeRegistryName(cfg.DefaultWorkspace) {
		return fmt.Errorf("invalid global config %q: default_workspace must not contain path separators", source)
	}
	if strings.TrimSpace(cfg.InboxRepo) != "" {
		if _, err := ParseRepoRef(cfg.InboxRepo); err != nil {
			return fmt.Errorf("invalid global config %q: inbox_repo must be in OWNER/REPO format", source)
		}
	}
	if err := validateGlobalPathValue(source, "paths.cache_root", cfg.Paths.CacheRoot); err != nil {
		return err
	}
	if err := validateGlobalPathValue(source, "paths.state_root", cfg.Paths.StateRoot); err != nil {
		return err
	}
	return nil
}

func ValidateGlobalRepoRegistryEntry(entry GlobalRepoRegistryEntry, expected RepoRef, source string) error {
	if strings.TrimSpace(entry.Repo) == "" {
		return fmt.Errorf("invalid global repo registry %q: repo is required", source)
	}
	repo, err := ParseRepoRef(entry.Repo)
	if err != nil {
		return fmt.Errorf("invalid global repo registry %q: repo must be in OWNER/REPO format", source)
	}
	if repoRefIsSet(expected) && !sameRepoRef(repo, expected) {
		return fmt.Errorf("invalid global repo registry %q: repo %s does not match registry path %s", source, repo.FullName(), expected.FullName())
	}
	if err := validateGlobalPathValue(source, "path", entry.Path); err != nil {
		return err
	}
	if strings.TrimSpace(entry.Contract) != "" {
		if err := validateRelativeGlobalPath(source, "contract", entry.Contract); err != nil {
			return err
		}
	}
	seenAliases := map[string]struct{}{}
	for i, alias := range entry.Aliases {
		trimmed := strings.TrimSpace(alias)
		if !isSafeRegistryName(trimmed) {
			return fmt.Errorf("invalid global repo registry %q: aliases[%d] must be non-empty and must not contain path separators", source, i)
		}
		key := strings.ToLower(trimmed)
		if _, ok := seenAliases[key]; ok {
			return fmt.Errorf("invalid global repo registry %q: aliases contains duplicate %q", source, trimmed)
		}
		seenAliases[key] = struct{}{}
	}
	if strings.TrimSpace(entry.Workspace.Name) != "" && !isSafeRegistryName(entry.Workspace.Name) {
		return fmt.Errorf("invalid global repo registry %q: workspace.name must not contain path separators", source)
	}
	if entry.Providers != nil && entry.Providers.Jira != nil {
		if err := validateJiraProviderConfig(source, "providers.jira", *entry.Providers.Jira); err != nil {
			return err
		}
	}
	if err := validateOperationPolicyConfig(source, "operation_mode", "delivery_policy", OperationPolicyConfig{OperationMode: entry.OperationMode, DeliveryPolicy: entry.DeliveryPolicy}); err != nil {
		return err
	}
	if err := validateBranchPolicyConfig(source, "branch_policy", entry.BranchPolicy); err != nil {
		return err
	}
	return nil
}

func ValidateGlobalWorkspaceRegistryEntry(entry GlobalWorkspaceRegistryEntry, expectedName string, source string) error {
	workspace := entry.Workspace
	if strings.TrimSpace(workspace.Name) == "" {
		return fmt.Errorf("invalid global workspace registry %q: workspace.name is required", source)
	}
	if !isSafeRegistryName(workspace.Name) {
		return fmt.Errorf("invalid global workspace registry %q: workspace.name must not contain path separators", source)
	}
	if strings.TrimSpace(expectedName) != "" && !strings.EqualFold(workspace.Name, expectedName) {
		return fmt.Errorf("invalid global workspace registry %q: workspace.name %q does not match registry path %q", source, workspace.Name, expectedName)
	}
	if strings.TrimSpace(workspace.InboxRepo) == "" {
		return fmt.Errorf("invalid global workspace registry %q: workspace.inbox_repo is required", source)
	}
	if _, err := ParseRepoRef(workspace.InboxRepo); err != nil {
		return fmt.Errorf("invalid global workspace registry %q: workspace.inbox_repo must be in OWNER/REPO format", source)
	}
	if len(workspace.Repos) == 0 {
		return fmt.Errorf("invalid global workspace registry %q: workspace.repos must include at least one execution repo", source)
	}
	seen := map[string]struct{}{}
	for i, value := range workspace.Repos {
		repo, err := ParseRepoRef(value)
		if err != nil {
			return fmt.Errorf("invalid global workspace registry %q: workspace.repos[%d] must be in OWNER/REPO format", source, i)
		}
		key := strings.ToLower(repo.FullName())
		if _, ok := seen[key]; ok {
			return fmt.Errorf("invalid global workspace registry %q: workspace.repos[%d] duplicates %s", source, i, repo.FullName())
		}
		seen[key] = struct{}{}
	}
	if workspace.Project.Number < 0 {
		return fmt.Errorf("invalid global workspace registry %q: workspace.project.number must be >= 0 when workspace.project is set", source)
	}
	if err := validateOperationPolicyConfig(source, "operation_mode", "delivery_policy", OperationPolicyConfig{OperationMode: entry.OperationMode, DeliveryPolicy: entry.DeliveryPolicy}); err != nil {
		return err
	}
	if err := validateBranchPolicyConfig(source, "branch_policy", entry.BranchPolicy); err != nil {
		return err
	}
	return nil
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

func readGlobalYAML(path string, out any) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read global config %q: %w", path, err)
	}
	if err := yaml.Unmarshal(content, out); err != nil {
		return fmt.Errorf("parse global config %q: %w", path, err)
	}
	return nil
}

func validateGlobalPathValue(source string, field string, value string) error {
	if strings.ContainsRune(value, 0) {
		return fmt.Errorf("invalid global config %q: %s must not contain NUL bytes", source, field)
	}
	return nil
}

func validateRelativeGlobalPath(source string, field string, value string) error {
	if err := validateGlobalPathValue(source, field, value); err != nil {
		return err
	}
	cleaned := filepath.Clean(value)
	if filepath.IsAbs(value) || cleaned == "." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) || cleaned == ".." {
		return fmt.Errorf("invalid global config %q: %s must be a relative path inside the repo", source, field)
	}
	return nil
}

func sameRepoRef(a RepoRef, b RepoRef) bool {
	return strings.EqualFold(a.Owner, b.Owner) && strings.EqualFold(a.Name, b.Name)
}

func repoRefIsSet(repo RepoRef) bool {
	return strings.TrimSpace(repo.Owner) != "" || strings.TrimSpace(repo.Name) != ""
}
