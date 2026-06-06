package gira

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ConfigGlobalReport struct {
	Command        string           `json:"command"`
	ConfigRoot     string           `json:"config_root"`
	Config         ConfigFileStatus `json:"config"`
	ReposRoot      ConfigPathStatus `json:"repos_root"`
	WorkspacesRoot ConfigPathStatus `json:"workspaces_root"`
}

type ConfigRepoReport struct {
	Command       string             `json:"command"`
	Repo          string             `json:"repo,omitempty"`
	Source        string             `json:"source"`
	Detail        string             `json:"detail,omitempty"`
	ConfigRoot    string             `json:"config_root"`
	GlobalRepo    ConfigFileStatus   `json:"global_repo"`
	RepoContracts []ConfigFileStatus `json:"repo_contracts"`
}

type ConfigDoctorReport struct {
	Command       string             `json:"command"`
	Repo          string             `json:"repo,omitempty"`
	Source        string             `json:"source"`
	Detail        string             `json:"detail,omitempty"`
	ConfigRoot    string             `json:"config_root"`
	GlobalConfig  ConfigFileStatus   `json:"global_config"`
	GlobalRepo    ConfigFileStatus   `json:"global_repo"`
	RepoContracts []ConfigFileStatus `json:"repo_contracts"`
	NextSteps     []string           `json:"next_steps,omitempty"`
}

const ConfigStorageReportSchemaVersion = "config-storage-report/v1"

type ConfigStorageReport struct {
	SchemaVersion string                 `json:"schema_version"`
	Command       string                 `json:"command"`
	Repo          string                 `json:"repo,omitempty"`
	Source        string                 `json:"source"`
	Detail        string                 `json:"detail,omitempty"`
	ConfigRoot    string                 `json:"config_root"`
	Surfaces      []ConfigStorageSurface `json:"surfaces"`
	Warnings      []ConfigStorageWarning `json:"warnings,omitempty"`
	NextSteps     []string               `json:"next_steps,omitempty"`
}

type ConfigStorageSurface struct {
	Name          string   `json:"name"`
	Kind          string   `json:"kind"`
	Path          string   `json:"path"`
	PathSource    string   `json:"path_source"`
	Exists        bool     `json:"exists"`
	Owner         string   `json:"owner"`
	Durability    string   `json:"durability"`
	Visibility    string   `json:"visibility"`
	SourceOfTruth string   `json:"source_of_truth"`
	Rebuild       string   `json:"rebuild"`
	Retention     string   `json:"retention"`
	Notes         []string `json:"notes,omitempty"`
}

type ConfigStorageWarning struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type ConfigPathStatus struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
}

type ConfigFileStatus struct {
	Path   string `json:"path,omitempty"`
	Exists bool   `json:"exists"`
	Valid  bool   `json:"valid,omitempty"`
	Error  string `json:"error,omitempty"`
}

func BuildConfigGlobalReport(configRoot string) (ConfigGlobalReport, error) {
	root, err := globalConfigRoot(configRoot)
	if err != nil {
		return ConfigGlobalReport{}, err
	}
	configPath, err := GlobalConfigPath(root)
	if err != nil {
		return ConfigGlobalReport{}, err
	}
	reposRoot, err := GlobalReposRoot(root)
	if err != nil {
		return ConfigGlobalReport{}, err
	}
	workspacesRoot, err := GlobalWorkspacesRoot(root)
	if err != nil {
		return ConfigGlobalReport{}, err
	}
	return ConfigGlobalReport{
		Command:        "config global",
		ConfigRoot:     root,
		Config:         inspectConfigFile(configPath, func() error { _, err := LoadGlobalConfig(root); return err }),
		ReposRoot:      ConfigPathStatus{Path: reposRoot, Exists: pathExists(reposRoot)},
		WorkspacesRoot: ConfigPathStatus{Path: workspacesRoot, Exists: pathExists(workspacesRoot)},
	}, nil
}

func BuildConfigRepoReport(repoValue string, configRoot string, runner CommandRunner) (ConfigRepoReport, error) {
	root, err := globalConfigRoot(configRoot)
	if err != nil {
		return ConfigRepoReport{}, err
	}
	repo, source, detail, err := resolveConfigDiagnosticRepo(repoValue, root, runner)
	if err != nil {
		return ConfigRepoReport{}, err
	}
	report := ConfigRepoReport{
		Command:       "config repo",
		Source:        source,
		Detail:        detail,
		ConfigRoot:    root,
		RepoContracts: inspectRepoContracts(),
	}
	if repoRefIsSet(repo) {
		report.Repo = repo.FullName()
		path, err := GlobalRepoRegistryPath(root, repo)
		if err != nil {
			return ConfigRepoReport{}, err
		}
		report.GlobalRepo = inspectConfigFile(path, func() error { _, err := LoadGlobalRepoRegistryEntry(root, repo); return err })
	}
	return report, nil
}

func BuildConfigDoctorReport(repoValue string, configRoot string, runner CommandRunner) (ConfigDoctorReport, error) {
	global, err := BuildConfigGlobalReport(configRoot)
	if err != nil {
		return ConfigDoctorReport{}, err
	}
	repo, err := BuildConfigRepoReport(repoValue, global.ConfigRoot, runner)
	if err != nil {
		return ConfigDoctorReport{}, err
	}
	report := ConfigDoctorReport{
		Command:       "config doctor",
		Repo:          repo.Repo,
		Source:        repo.Source,
		Detail:        repo.Detail,
		ConfigRoot:    global.ConfigRoot,
		GlobalConfig:  global.Config,
		GlobalRepo:    repo.GlobalRepo,
		RepoContracts: repo.RepoContracts,
	}
	if report.Source == "defaults" {
		report.NextSteps = append(report.NextSteps, "pass --repo OWNER/REPO, run from a GitHub checkout, or register the repo in the global registry")
	}
	if report.GlobalConfig.Exists && !report.GlobalConfig.Valid {
		report.NextSteps = append(report.NextSteps, "fix global config validation errors")
	}
	if report.GlobalRepo.Exists && !report.GlobalRepo.Valid {
		report.NextSteps = append(report.NextSteps, "fix global repo registry validation errors")
	}
	return report, nil
}

func BuildConfigStorageReport(repoValue string, configRoot string, runner CommandRunner) (ConfigStorageReport, error) {
	global, err := BuildConfigGlobalReport(configRoot)
	if err != nil {
		return ConfigStorageReport{}, err
	}
	repo, err := BuildConfigRepoReport(repoValue, global.ConfigRoot, runner)
	if err != nil {
		return ConfigStorageReport{}, err
	}

	report := ConfigStorageReport{
		SchemaVersion: ConfigStorageReportSchemaVersion,
		Command:       "config storage",
		Repo:          repo.Repo,
		Source:        repo.Source,
		Detail:        repo.Detail,
		ConfigRoot:    global.ConfigRoot,
	}

	var cfg GlobalConfig
	if global.Config.Exists && global.Config.Valid {
		cfg, _ = LoadGlobalConfig(global.ConfigRoot)
	}
	if global.Config.Exists && !global.Config.Valid {
		report.Warnings = append(report.Warnings, ConfigStorageWarning{
			Code:     "invalid_global_config",
			Severity: "warning",
			Message:  "Global config exists but is invalid, so paths.cache_root and paths.state_root overrides are ignored.",
		})
	}

	cacheRoot, cacheSource, err := defaultGiraCacheRootForConfig(cfg)
	if err != nil {
		return ConfigStorageReport{}, err
	}
	stateRoot, stateSource, err := DefaultGiraStateRoot(global.ConfigRoot)
	if err != nil {
		return ConfigStorageReport{}, err
	}
	wrapperCacheRoot, err := DefaultCachePruneRoot()
	if err != nil {
		return ConfigStorageReport{}, err
	}

	report.Surfaces = append(report.Surfaces,
		ConfigStorageSurface{
			Name:          "global_config",
			Kind:          "config",
			Path:          global.Config.Path,
			PathSource:    "global_config_root",
			Exists:        global.Config.Exists,
			Owner:         "operator",
			Durability:    "durable_config",
			Visibility:    "private_config",
			SourceOfTruth: "local_file",
			Rebuild:       "not_reconstructible_from_github",
			Retention:     "keep_until_operator_removes_or_migrates",
			Notes:         []string{"Stores OS-user defaults and optional cache/state path overrides."},
		},
		ConfigStorageSurface{
			Name:          "global_repo_registry",
			Kind:          "config",
			Path:          global.ReposRoot.Path,
			PathSource:    "global_config_root",
			Exists:        global.ReposRoot.Exists,
			Owner:         "operator",
			Durability:    "durable_config",
			Visibility:    "private_config",
			SourceOfTruth: "local_file",
			Rebuild:       "partially_reconstructible_from_git_origin_and_operator_choices",
			Retention:     "keep_until_repo_is_unregistered",
		},
		ConfigStorageSurface{
			Name:          "global_workspace_registry",
			Kind:          "config",
			Path:          global.WorkspacesRoot.Path,
			PathSource:    "global_config_root",
			Exists:        global.WorkspacesRoot.Exists,
			Owner:         "operator",
			Durability:    "durable_config",
			Visibility:    "private_config",
			SourceOfTruth: "local_file",
			Rebuild:       "not_reconstructible_from_github_without_operator_workspace_choices",
			Retention:     "keep_until_workspace_is_removed",
		},
	)

	if strings.TrimSpace(repo.GlobalRepo.Path) != "" {
		report.Surfaces = append(report.Surfaces, ConfigStorageSurface{
			Name:          "selected_repo_registry",
			Kind:          "config",
			Path:          repo.GlobalRepo.Path,
			PathSource:    "selected_repo",
			Exists:        repo.GlobalRepo.Exists,
			Owner:         "operator",
			Durability:    "durable_config",
			Visibility:    "private_config",
			SourceOfTruth: "local_file",
			Rebuild:       "partially_reconstructible_from_git_origin_and_operator_choices",
			Retention:     "keep_until_repo_is_unregistered",
		})
	}

	for _, contract := range repo.RepoContracts {
		report.Surfaces = append(report.Surfaces, ConfigStorageSurface{
			Name:          "repo_local_contract",
			Kind:          "config",
			Path:          contract.Path,
			PathSource:    "current_checkout",
			Exists:        contract.Exists,
			Owner:         "repo",
			Durability:    "shared_repo_contract",
			Visibility:    "repo_visible_when_committed",
			SourceOfTruth: "repo_file",
			Rebuild:       "recovered_by_git_checkout_when_committed",
			Retention:     "keep_while_repo_uses_gira_contract_mode",
		})
	}

	report.Surfaces = append(report.Surfaces,
		ConfigStorageSurface{
			Name:          "runtime_state_root",
			Kind:          "state",
			Path:          stateRoot,
			PathSource:    stateSource,
			Exists:        pathExists(stateRoot),
			Owner:         "gira_runtime",
			Durability:    "private_runtime_state",
			Visibility:    "private_local",
			SourceOfTruth: "local_runtime_evidence_only",
			Rebuild:       "workflow_state_rebuilds_from_github_but_private_prompts_and_logs_do_not",
			Retention:     "operator_managed",
			Notes:         []string{"Must not become the source of truth for ticket completion."},
		},
		ConfigStorageSurface{
			Name:          "run_manifests",
			Kind:          "runtime_evidence",
			Path:          filepath.Join(stateRoot, "runs"),
			PathSource:    "runtime_state_root",
			Exists:        pathExists(filepath.Join(stateRoot, "runs")),
			Owner:         "worker_runtime",
			Durability:    "private_runtime_state",
			Visibility:    "private_local",
			SourceOfTruth: "optional_worker_evidence",
			Rebuild:       "not_reconstructible_if_deleted",
			Retention:     "operator_managed",
			Notes:         []string{"Contains prompts, event logs, stderr, results, and manifests when gira run is used."},
		},
		ConfigStorageSurface{
			Name:          "gira_cache_root",
			Kind:          "cache",
			Path:          cacheRoot,
			PathSource:    cacheSource,
			Exists:        pathExists(cacheRoot),
			Owner:         "gira",
			Durability:    "disposable_cache",
			Visibility:    "private_local",
			SourceOfTruth: "github_and_config",
			Rebuild:       "regenerated_by_refreshing_commands",
			Retention:     "safe_to_prune_when_not_in_use",
		},
		ConfigStorageSurface{
			Name:          "workspace_status_cache",
			Kind:          "cache",
			Path:          filepath.Join(cacheRoot, "workspace-status"),
			PathSource:    "gira_cache_root",
			Exists:        pathExists(filepath.Join(cacheRoot, "workspace-status")),
			Owner:         "gira",
			Durability:    "disposable_cache",
			Visibility:    "private_local",
			SourceOfTruth: "github_issue_pr_milestone_state",
			Rebuild:       "regenerated_by_workspace_status_refresh",
			Retention:     "bounded_by_cache_ttl_and_operator_prune",
		},
		ConfigStorageSurface{
			Name:          "audit_ledger",
			Kind:          "audit",
			Path:          filepath.Join(".gira", "audit"),
			PathSource:    "current_checkout",
			Exists:        pathExists(filepath.Join(".gira", "audit")),
			Owner:         "repo_operator",
			Durability:    "local_receipt",
			Visibility:    "repo_local_private_unless_committed",
			SourceOfTruth: "local_append_only_evidence",
			Rebuild:       "not_reconstructible_if_deleted",
			Retention:     "keep_when_audit_trace_is_required",
		},
		ConfigStorageSurface{
			Name:          "dashboard_export_bundle",
			Kind:          "export",
			Path:          "operator-selected --output path",
			PathSource:    "export_dashboard_flag",
			Exists:        false,
			Owner:         "operator",
			Durability:    "regenerable_export",
			Visibility:    "operator_selected",
			SourceOfTruth: "github_and_gira_computed_state",
			Rebuild:       "regenerated_by_gira_export_dashboard",
			Retention:     "safe_to_delete_when_downstream_consumers_do_not_need_snapshot",
		},
		ConfigStorageSurface{
			Name:          "wrapper_binary_cache",
			Kind:          "cache",
			Path:          wrapperCacheRoot,
			PathSource:    "GIRA_PYPI_CACHE_DIR_or_user_cache_dir",
			Exists:        pathExists(wrapperCacheRoot),
			Owner:         "distribution_wrapper",
			Durability:    "disposable_cache",
			Visibility:    "private_local",
			SourceOfTruth: "published_release_artifacts",
			Rebuild:       "redownloaded_by_package_wrapper",
			Retention:     "safe_to_prune_with_gira_cache_prune",
		},
	)

	if report.Source == "defaults" {
		report.NextSteps = append(report.NextSteps, "pass --repo OWNER/REPO or run from a GitHub checkout to include selected repo registry paths")
	}
	if stateSource == "config_root_state_default" {
		report.NextSteps = append(report.NextSteps, "set paths.state_root in global config when runtime evidence should live outside the config root")
	}
	return report, nil
}

func FormatConfigGlobalReport(report ConfigGlobalReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "config global: %s\n", report.ConfigRoot)
	fmt.Fprintf(&b, "config: %s\n", formatConfigFileStatus(report.Config))
	fmt.Fprintf(&b, "repos: %s exists=%t\n", report.ReposRoot.Path, report.ReposRoot.Exists)
	fmt.Fprintf(&b, "workspaces: %s exists=%t\n", report.WorkspacesRoot.Path, report.WorkspacesRoot.Exists)
	return b.String()
}

func FormatConfigRepoReport(report ConfigRepoReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "config repo: %s\n", valueOrNone(report.Repo))
	fmt.Fprintf(&b, "source: %s\n", report.Source)
	if strings.TrimSpace(report.Detail) != "" {
		fmt.Fprintf(&b, "detail: %s\n", report.Detail)
	}
	if strings.TrimSpace(report.GlobalRepo.Path) != "" {
		fmt.Fprintf(&b, "global repo: %s\n", formatConfigFileStatus(report.GlobalRepo))
	}
	for _, contract := range report.RepoContracts {
		fmt.Fprintf(&b, "repo contract: %s\n", formatConfigFileStatus(contract))
	}
	return b.String()
}

func FormatConfigDoctorReport(report ConfigDoctorReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "config doctor: %s\n", report.ConfigRoot)
	fmt.Fprintf(&b, "source: %s\n", report.Source)
	if strings.TrimSpace(report.Detail) != "" {
		fmt.Fprintf(&b, "detail: %s\n", report.Detail)
	}
	fmt.Fprintf(&b, "global config: %s\n", formatConfigFileStatus(report.GlobalConfig))
	if strings.TrimSpace(report.GlobalRepo.Path) != "" {
		fmt.Fprintf(&b, "global repo: %s\n", formatConfigFileStatus(report.GlobalRepo))
	}
	for _, contract := range report.RepoContracts {
		fmt.Fprintf(&b, "repo contract: %s\n", formatConfigFileStatus(contract))
	}
	for _, next := range report.NextSteps {
		fmt.Fprintf(&b, "next step: %s\n", next)
	}
	return b.String()
}

func FormatConfigStorageReport(report ConfigStorageReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "config storage: %s\n", report.ConfigRoot)
	fmt.Fprintf(&b, "source: %s\n", report.Source)
	if strings.TrimSpace(report.Repo) != "" {
		fmt.Fprintf(&b, "repo: %s\n", report.Repo)
	}
	if strings.TrimSpace(report.Detail) != "" {
		fmt.Fprintf(&b, "detail: %s\n", report.Detail)
	}
	for _, surface := range report.Surfaces {
		fmt.Fprintf(&b, "- %s [%s]: %s exists=%t durability=%s visibility=%s source=%s rebuild=%s\n", surface.Name, surface.Kind, surface.Path, surface.Exists, surface.Durability, surface.Visibility, surface.SourceOfTruth, surface.Rebuild)
	}
	for _, warning := range report.Warnings {
		fmt.Fprintf(&b, "warning: %s %s: %s\n", warning.Severity, warning.Code, warning.Message)
	}
	for _, next := range report.NextSteps {
		fmt.Fprintf(&b, "next step: %s\n", next)
	}
	return b.String()
}

func resolveConfigDiagnosticRepo(repoValue string, configRoot string, runner CommandRunner) (RepoRef, string, string, error) {
	ctx, err := ResolveRepoContextDetails(RepoContextOptions{RepoValue: repoValue, ConfigRoot: configRoot, Runner: runner})
	if err != nil {
		if strings.TrimSpace(repoValue) == "" && strings.Contains(err.Error(), "repo context unavailable") {
			return RepoRef{}, "defaults", "", nil
		}
		return RepoRef{}, "", "", err
	}
	source := ctx.Source
	detail := ctx.Detail
	if source == "repo_config" {
		source = "repo_local_contract"
		if strings.HasPrefix(detail, "."+string(filepath.Separator)) {
			detail = strings.TrimPrefix(detail, "."+string(filepath.Separator))
		}
	}
	if source == "explicit" {
		detail = "--repo"
	}
	return ctx.Repo, source, detail, nil
}

func inspectRepoContracts() []ConfigFileStatus {
	return []ConfigFileStatus{
		inspectConfigFile(DefaultInitConfigPath("."), func() error {
			_, _, err := repoContextFromConfig(DefaultInitConfigPath("."))
			return err
		}),
		inspectConfigFile(filepath.Join(".", ".gira", "config.toml"), func() error {
			_, _, err := repoContextFromConfig(filepath.Join(".", ".gira", "config.toml"))
			return err
		}),
	}
}

func inspectConfigFile(path string, validate func() error) ConfigFileStatus {
	status := ConfigFileStatus{Path: path, Exists: pathExists(path)}
	if !status.Exists {
		return status
	}
	if err := validate(); err != nil {
		status.Error = err.Error()
		return status
	}
	status.Valid = true
	return status
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func formatConfigFileStatus(status ConfigFileStatus) string {
	if !status.Exists {
		return fmt.Sprintf("%s exists=false", status.Path)
	}
	if !status.Valid {
		return fmt.Sprintf("%s exists=true valid=false error=%s", status.Path, status.Error)
	}
	return fmt.Sprintf("%s exists=true valid=true", status.Path)
}

func valueOrNone(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(none)"
	}
	return value
}

func defaultGiraCacheRootForConfig(cfg GlobalConfig) (string, string, error) {
	if strings.TrimSpace(cfg.Paths.CacheRoot) != "" {
		path, err := filepath.Abs(expandUserPath(cfg.Paths.CacheRoot))
		if err != nil {
			return "", "", fmt.Errorf("resolve paths.cache_root: %w", err)
		}
		return path, "global_config.paths.cache_root", nil
	}
	root, err := os.UserCacheDir()
	if err != nil {
		return "", "", fmt.Errorf("resolve user cache directory: %w", err)
	}
	return filepath.Join(root, "gira"), "os_user_cache_dir", nil
}

func DefaultGiraStateRoot(configRoot string) (string, string, error) {
	root, err := globalConfigRoot(configRoot)
	if err != nil {
		return "", "", err
	}
	cfg, err := LoadGlobalConfig(root)
	if err == nil && strings.TrimSpace(cfg.Paths.StateRoot) != "" {
		path, err := filepath.Abs(expandUserPath(cfg.Paths.StateRoot))
		if err != nil {
			return "", "", fmt.Errorf("resolve paths.state_root: %w", err)
		}
		return path, "global_config.paths.state_root", nil
	}
	return filepath.Join(root, "state"), "config_root_state_default", nil
}
