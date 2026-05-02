package gira

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type InitReport struct {
	Command            string            `json:"command"`
	Repo               string            `json:"repo"`
	Path               string            `json:"path"`
	DryRun             bool              `json:"dry_run"`
	ConfigPath         string            `json:"config_path,omitempty"`
	ConfigProfileCount int               `json:"config_profile_count,omitempty"`
	Ready              bool              `json:"ready"`
	Checks             map[string]bool   `json:"checks"`
	Failures           map[string]string `json:"failures,omitempty"`
	Remediations       map[string]string `json:"remediations,omitempty"`
	PlannedSteps       []string          `json:"planned_steps"`
	NextStep           string            `json:"next_step"`
}

func BuildInitReport(repo RepoRef, path string, dryRun bool, runner CommandRunner) (InitReport, error) {
	return BuildInitReportWithConfig(repo, path, dryRun, "", nil, runner)
}

func BuildInitReportWithConfig(repo RepoRef, path string, dryRun bool, configPath string, config *InitConfig, runner CommandRunner) (InitReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if strings.TrimSpace(path) == "" {
		path = "."
	}
	absPath, _ := filepath.Abs(path)
	report := InitReport{
		Command:      "init",
		Repo:         repo.FullName(),
		Path:         absPath,
		DryRun:       dryRun,
		ConfigPath:   configPath,
		Checks:       map[string]bool{},
		Failures:     map[string]string{},
		Remediations: map[string]string{},
		PlannedSteps: []string{
			"gira bootstrap --repo " + repo.FullName() + " --path " + absPath,
			"gira sync --repo " + repo.FullName(),
			"gira status --repo " + repo.FullName() + " --json",
		},
	}
	if config != nil {
		report.ConfigProfileCount = len(config.Profiles)
		profileNames := make([]string, 0, len(config.Profiles))
		for name := range config.Profiles {
			profileNames = append(profileNames, name)
		}
		sort.Strings(profileNames)
		for _, name := range profileNames {
			profile := config.Profiles[name]
			report.PlannedSteps = append(report.PlannedSteps,
				fmt.Sprintf("config profile %q: labels=%d milestones=%d issue_templates=%d approvals=%d codeowners=%t",
					name,
					len(profile.Labels),
					len(profile.Milestones),
					len(profile.IssueTemplates),
					profile.ReviewPolicy.RequiredApprovals,
					profile.ReviewPolicy.RequireCodeOwners,
				),
			)
		}
	}

	report.Checks["gh_installed"] = probeCommand(runner, "gh", "--version") == nil
	if !report.Checks["gh_installed"] {
		report.Failures["gh_installed"] = "gh cli not found"
		report.Remediations["gh_installed"] = "install GitHub CLI and ensure `gh` is on PATH"
	}

	report.Checks["git_installed"] = probeCommand(runner, "git", "--version") == nil
	if !report.Checks["git_installed"] {
		report.Failures["git_installed"] = "git not found"
		report.Remediations["git_installed"] = "install git and ensure `git` is on PATH"
	}

	report.Checks["gh_auth"] = probeCommand(runner, "gh", "auth", "status") == nil
	if !report.Checks["gh_auth"] {
		report.Failures["gh_auth"] = "gh auth status failed"
		report.Remediations["gh_auth"] = "run `gh auth login` and confirm token scopes"
	}

	report.Checks["repo_access"] = probeCommand(runner, "gh", "repo", "view", repo.FullName(), "--json", "name") == nil
	if !report.Checks["repo_access"] {
		report.Failures["repo_access"] = "cannot access repository"
		report.Remediations["repo_access"] = "verify repository name and token access rights"
	}

	report.Checks["path_is_git_repo"] = probeCommand(runner, "git", "-C", absPath, "rev-parse", "--is-inside-work-tree") == nil
	if !report.Checks["path_is_git_repo"] {
		report.Failures["path_is_git_repo"] = "path is not a git repository"
		report.Remediations["path_is_git_repo"] = "clone the target repository or pass --path to a git working tree"
	}

	report.Checks["git_clean"] = probeCommand(runner, "git", "-C", absPath, "diff", "--quiet") == nil && probeCommand(runner, "git", "-C", absPath, "diff", "--cached", "--quiet") == nil
	if !report.Checks["git_clean"] {
		report.Failures["git_clean"] = "working tree has uncommitted changes"
		report.Remediations["git_clean"] = "commit or stash changes before running bootstrap/apply steps"
	}

	report.Ready = len(report.Failures) == 0
	if report.Ready {
		report.NextStep = report.PlannedSteps[0]
		return report, nil
	}
	report.NextStep = "resolve prerequisites and re-run `gira init --repo " + repo.FullName() + " --path " + absPath + " --json`"
	return report, fmt.Errorf("init prerequisites failed")
}

func probeCommand(runner CommandRunner, name string, args ...string) error {
	_, err := runner.Run(name, args...)
	return err
}

func FormatInitReport(report InitReport) string {
	var b strings.Builder
	status := "blocked"
	if report.Ready {
		status = "ready"
	}
	fmt.Fprintf(&b, "init %s: %s\n", status, report.Repo)
	if strings.TrimSpace(report.ConfigPath) != "" {
		fmt.Fprintf(&b, "- config: %s (profiles=%d)\n", report.ConfigPath, report.ConfigProfileCount)
	}
	for check, ok := range report.Checks {
		state := "FAIL"
		if ok {
			state = "OK"
		}
		fmt.Fprintf(&b, "- %s: %s\n", check, state)
	}
	if len(report.Failures) > 0 {
		for check, reason := range report.Failures {
			fmt.Fprintf(&b, "- remediation[%s]: %s (%s)\n", check, report.Remediations[check], reason)
		}
	}
	if len(report.PlannedSteps) > 0 {
		fmt.Fprintf(&b, "next:\n")
		for _, step := range report.PlannedSteps {
			fmt.Fprintf(&b, "- %s\n", step)
		}
	}
	return b.String()
}
