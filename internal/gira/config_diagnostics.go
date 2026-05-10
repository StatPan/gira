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
