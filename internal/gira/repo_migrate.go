package gira

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type RepoMigrateInput struct {
	Repo       RepoRef `json:"repo"`
	Path       string  `json:"path,omitempty"`
	ConfigRoot string  `json:"config_root,omitempty"`
	Overwrite  bool    `json:"overwrite"`
	DryRun     bool    `json:"dry_run"`
	Apply      bool    `json:"apply"`
}

type RepoMigrateReport struct {
	Command      string                  `json:"command"`
	Repo         string                  `json:"repo,omitempty"`
	ConfigRoot   string                  `json:"config_root"`
	Path         string                  `json:"path"`
	Contract     string                  `json:"contract"`
	ContractFile string                  `json:"contract_file"`
	File         string                  `json:"file,omitempty"`
	DryRun       bool                    `json:"dry_run"`
	Applied      bool                    `json:"applied"`
	Status       string                  `json:"status"`
	Action       string                  `json:"action"`
	Entry        GlobalRepoRegistryEntry `json:"entry,omitempty"`
	Register     *RepoRegisterReport     `json:"register,omitempty"`
	Notes        []string                `json:"notes,omitempty"`
	NextStep     string                  `json:"next_step,omitempty"`
}

func BuildRepoMigrateReport(input RepoMigrateInput, runner CommandRunner) (RepoMigrateReport, error) {
	if input.DryRun == input.Apply {
		return RepoMigrateReport{}, fmt.Errorf("exactly one of --dry-run or --apply is required")
	}
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	root, err := globalConfigRoot(input.ConfigRoot)
	if err != nil {
		return RepoMigrateReport{}, err
	}
	checkoutPath, err := normalizeRepoMigratePath(input.Path)
	if err != nil {
		return RepoMigrateReport{}, err
	}
	contract := filepath.ToSlash(filepath.Join(".gira", "config.yaml"))
	contractFile := filepath.Join(checkoutPath, filepath.FromSlash(contract))
	report := RepoMigrateReport{
		Command:      "repo migrate",
		ConfigRoot:   root,
		Path:         checkoutPath,
		Contract:     contract,
		ContractFile: contractFile,
		DryRun:       input.DryRun,
		Status:       actionStatus(input.DryRun),
		Action:       "plan",
		Notes: []string{
			"repo-local .gira/config.yaml is preserved as the shared contract",
			"global registry imports personal metadata and references the repo-local contract",
			"symlink migration is not the default; use explicit advanced migration if it is added later",
		},
	}
	if _, err := os.Stat(contractFile); err != nil {
		report.Status = "blocked"
		report.Action = "none"
		if os.IsNotExist(err) {
			return report, fmt.Errorf("repo-local contract %s was not found", contractFile)
		}
		return report, fmt.Errorf("stat repo-local contract %s: %w", contractFile, err)
	}
	repo, err := resolveRepoMigrateRepo(input.Repo, contractFile, checkoutPath, runner)
	if err != nil {
		report.Status = "blocked"
		report.Action = "none"
		return report, err
	}
	report.Repo = repo.FullName()
	file, err := GlobalRepoRegistryPath(root, repo)
	if err != nil {
		return report, err
	}
	report.File = file
	workspaceName := repoMigrateWorkspaceName(contractFile)
	register, err := BuildRepoRegisterReport(RepoRegisterInput{
		Repo:          repo,
		Path:          checkoutPath,
		ConfigRoot:    root,
		Contract:      contract,
		WorkspaceName: workspaceName,
		Overwrite:     input.Overwrite,
		DryRun:        input.DryRun,
		Apply:         input.Apply,
	}, runner)
	if register.Command != "" {
		report.Register = &register
		report.Entry = register.Entry
		report.Action = register.Action
		report.Status = register.Status
		report.NextStep = register.NextStep
		report.Applied = register.Applied
		if input.DryRun && (register.Action == "create" || register.Action == "overwrite") {
			next := fmt.Sprintf("gira repo migrate --repo %s --path %s --config-root %s --apply", repo.FullName(), checkoutPath, root)
			if input.Overwrite || register.Action == "overwrite" {
				next += " --overwrite"
			}
			report.NextStep = next
		}
	}
	if err != nil {
		if report.Status == "" || report.Status == actionStatus(input.DryRun) {
			report.Status = "blocked"
		}
		if report.Action == "" || report.Action == "plan" {
			report.Action = "none"
		}
		return report, err
	}
	if input.Apply && register.Applied {
		report.Status = "applied"
		report.NextStep = fmt.Sprintf("gira config repo --repo %s --config-root %s", repo.FullName(), root)
	}
	return report, nil
}

func FormatRepoMigrateReport(report RepoMigrateReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "repo migrate: %s %s\n", report.Status, valueOrNone(report.Repo))
	fmt.Fprintf(&b, "contract: %s\n", report.ContractFile)
	if strings.TrimSpace(report.File) != "" {
		fmt.Fprintf(&b, "global repo: %s\n", report.File)
	}
	fmt.Fprintf(&b, "action: %s\n", report.Action)
	for _, note := range report.Notes {
		fmt.Fprintf(&b, "note: %s\n", note)
	}
	if strings.TrimSpace(report.NextStep) != "" {
		fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	}
	return b.String()
}

func normalizeRepoMigratePath(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		trimmed = "."
	}
	if strings.ContainsRune(trimmed, 0) {
		return "", fmt.Errorf("--path must not contain NUL bytes")
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve --path: %w", err)
	}
	return filepath.Clean(abs), nil
}

func resolveRepoMigrateRepo(explicit RepoRef, contractFile string, checkoutPath string, runner CommandRunner) (RepoRef, error) {
	configRepo, hasConfigRepo, err := repoContextFromConfig(contractFile)
	if err != nil {
		return RepoRef{}, err
	}
	if repoRefIsSet(explicit) {
		if hasConfigRepo && !sameRepoRef(explicit, configRepo) {
			return RepoRef{}, fmt.Errorf("explicit repo %s does not match repo-local contract %s", explicit.FullName(), configRepo.FullName())
		}
		return explicit, nil
	}
	if hasConfigRepo {
		return configRepo, nil
	}
	out, err := runner.Run("git", "-C", checkoutPath, "remote", "get-url", "origin")
	if err != nil {
		return RepoRef{}, fmt.Errorf("repo could not be inferred from %s; pass --repo OWNER/REPO", contractFile)
	}
	repo, err := ParseGitHubRemoteRepo(strings.TrimSpace(string(out)))
	if err != nil {
		return RepoRef{}, fmt.Errorf("git origin is not a GitHub OWNER/REPO URL; pass --repo OWNER/REPO")
	}
	return repo, nil
}

func repoMigrateWorkspaceName(contractFile string) string {
	cfg, err := loadWorkspaceConfig(contractFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cfg.Workspace.Name)
}
