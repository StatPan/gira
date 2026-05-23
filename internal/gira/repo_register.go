package gira

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

const RepoRegisterReportSchemaVersion = "repo-register-report/v1"

type RepoRegisterInput struct {
	Repo          RepoRef `json:"repo"`
	Path          string  `json:"path,omitempty"`
	ConfigRoot    string  `json:"config_root,omitempty"`
	Contract      string  `json:"contract,omitempty"`
	WorkspaceName string  `json:"workspace_name,omitempty"`
	Overwrite     bool    `json:"overwrite"`
	DryRun        bool    `json:"dry_run"`
	Apply         bool    `json:"apply"`
}

type RepoRegisterReport struct {
	SchemaVersion string                  `json:"schema_version,omitempty"`
	Command       string                  `json:"command"`
	Repo          string                  `json:"repo"`
	ConfigRoot    string                  `json:"config_root"`
	Path          string                  `json:"path,omitempty"`
	File          string                  `json:"file"`
	DryRun        bool                    `json:"dry_run"`
	Applied       bool                    `json:"applied"`
	Status        string                  `json:"status"`
	Action        string                  `json:"action"`
	Entry         GlobalRepoRegistryEntry `json:"entry"`
	NextStep      string                  `json:"next_step,omitempty"`
	Approval      *ApprovalEvidence       `json:"approval,omitempty"`
}

func EnsureRepoRegisterReportSchema(report *RepoRegisterReport) {
	if report != nil && strings.TrimSpace(report.SchemaVersion) == "" {
		report.SchemaVersion = RepoRegisterReportSchemaVersion
	}
}

func BuildRepoRegisterReport(input RepoRegisterInput, runner CommandRunner) (RepoRegisterReport, error) {
	if input.DryRun == input.Apply {
		return RepoRegisterReport{}, fmt.Errorf("exactly one of --dry-run or --apply is required")
	}
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	root, err := globalConfigRoot(input.ConfigRoot)
	if err != nil {
		return RepoRegisterReport{}, err
	}
	file, err := GlobalRepoRegistryPath(root, input.Repo)
	if err != nil {
		return RepoRegisterReport{}, err
	}
	storedPath, checkoutPath, err := normalizeRepoRegisterPath(input.Path)
	if err != nil {
		return RepoRegisterReport{}, err
	}
	if checkoutPath != "" {
		if err := validateRepoRegisterCheckout(input.Repo, checkoutPath, runner); err != nil {
			return RepoRegisterReport{}, err
		}
	}
	entry := GlobalRepoRegistryEntry{Repo: input.Repo.FullName(), Path: storedPath, Contract: strings.TrimSpace(input.Contract)}
	if strings.TrimSpace(input.WorkspaceName) != "" {
		entry.Workspace = GlobalRepoWorkspaceRef{Name: strings.TrimSpace(input.WorkspaceName)}
	}
	if err := ValidateGlobalRepoRegistryEntry(entry, input.Repo, file); err != nil {
		return RepoRegisterReport{}, err
	}
	content, err := marshalRepoRegistryEntry(entry)
	if err != nil {
		return RepoRegisterReport{}, err
	}
	report := RepoRegisterReport{
		SchemaVersion: RepoRegisterReportSchemaVersion,
		Command:       "repo register",
		Repo:          input.Repo.FullName(),
		ConfigRoot:    root,
		Path:          storedPath,
		File:          file,
		DryRun:        input.DryRun,
		Status:        actionStatus(input.DryRun),
		Action:        "create",
		Entry:         entry,
		NextStep:      fmt.Sprintf("gira repo register %s --apply", QuoteShellArg(input.Repo.FullName())),
	}
	existing, err := os.ReadFile(file)
	if err == nil {
		existingEntry, loadErr := LoadGlobalRepoRegistryEntry(root, input.Repo)
		if loadErr != nil && !input.Overwrite {
			report.Action = "conflict"
			report.Status = "blocked"
			if input.DryRun {
				report.Approval = RepoRegisterApprovalEvidence(report)
			}
			return report, loadErr
		}
		if loadErr == nil && repoRegistryEntriesEqual(existingEntry, entry) {
			report.Action = "skip"
			report.Status = "skipped"
			report.NextStep = "repo already registered"
			if input.DryRun {
				report.Approval = RepoRegisterApprovalEvidence(report)
			}
			return report, nil
		}
		if !input.Overwrite {
			report.Action = "conflict"
			report.Status = "blocked"
			if input.DryRun {
				report.Approval = RepoRegisterApprovalEvidence(report)
			}
			return report, fmt.Errorf("%s already exists with different content; pass --overwrite to replace it", file)
		}
		if bytes.Equal(bytes.TrimSpace(existing), bytes.TrimSpace(content)) {
			report.Action = "skip"
			report.Status = "skipped"
			report.NextStep = "repo already registered"
			if input.DryRun {
				report.Approval = RepoRegisterApprovalEvidence(report)
			}
			return report, nil
		}
		report.Action = "overwrite"
	} else if !os.IsNotExist(err) {
		return report, fmt.Errorf("read repo registry %q: %w", file, err)
	}
	if input.DryRun {
		report.Approval = RepoRegisterApprovalEvidence(report)
		return report, nil
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return report, fmt.Errorf("create repo registry directory %q: %w", filepath.Dir(file), err)
	}
	if err := os.WriteFile(file, content, 0o644); err != nil {
		return report, fmt.Errorf("write repo registry %q: %w", file, err)
	}
	report.Applied = true
	report.Status = "applied"
	report.NextStep = fmt.Sprintf("gira config repo --repo %s --config-root %s", QuoteShellArg(input.Repo.FullName()), QuoteShellArg(root))
	return report, nil
}

func FormatRepoRegisterReport(report RepoRegisterReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "repo register: %s %s\n", report.Status, report.Repo)
	fmt.Fprintf(&b, "file: %s\n", report.File)
	fmt.Fprintf(&b, "action: %s\n", report.Action)
	if strings.TrimSpace(report.Path) != "" {
		fmt.Fprintf(&b, "path: %s\n", report.Path)
	}
	if strings.TrimSpace(report.NextStep) != "" {
		fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	}
	return b.String()
}

func normalizeRepoRegisterPath(value string) (string, string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", "", nil
	}
	if strings.ContainsRune(trimmed, 0) {
		return "", "", fmt.Errorf("--path must not contain NUL bytes")
	}
	if trimmed == "~" || strings.HasPrefix(trimmed, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", "", fmt.Errorf("resolve home directory for --path: %w", err)
		}
		suffix := strings.TrimPrefix(trimmed, "~")
		return filepath.ToSlash(filepath.Clean(trimmed)), filepath.Join(home, filepath.FromSlash(suffix)), nil
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return "", "", fmt.Errorf("resolve --path: %w", err)
	}
	return filepath.Clean(abs), filepath.Clean(abs), nil
}

func validateRepoRegisterCheckout(repo RepoRef, checkoutPath string, runner CommandRunner) error {
	info, err := os.Stat(checkoutPath)
	if err != nil {
		return fmt.Errorf("validate checkout path %q: %w", checkoutPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("validate checkout path %q: not a directory", checkoutPath)
	}
	output, err := runner.Run("git", "-C", checkoutPath, "remote", "get-url", "origin")
	if err != nil {
		return fmt.Errorf("validate checkout path %q: git origin is required: %w", checkoutPath, err)
	}
	originRepo, err := ParseGitHubRemoteRepo(strings.TrimSpace(string(output)))
	if err != nil {
		return fmt.Errorf("validate checkout path %q: origin remote is not a GitHub OWNER/REPO URL", checkoutPath)
	}
	if !sameRepoRef(originRepo, repo) {
		return fmt.Errorf("validate checkout path %q: origin %s does not match %s", checkoutPath, originRepo.FullName(), repo.FullName())
	}
	return nil
}

func marshalRepoRegistryEntry(entry GlobalRepoRegistryEntry) ([]byte, error) {
	content, err := yaml.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("encode repo registry: %w", err)
	}
	return content, nil
}

func repoRegistryEntriesEqual(a GlobalRepoRegistryEntry, b GlobalRepoRegistryEntry) bool {
	return strings.EqualFold(a.Repo, b.Repo) &&
		filepath.Clean(a.Path) == filepath.Clean(b.Path) &&
		a.Contract == b.Contract &&
		strings.EqualFold(a.Workspace.Name, b.Workspace.Name) &&
		stringSlicesEqual(a.Aliases, b.Aliases) &&
		globalDefaultsEqual(a.Defaults, b.Defaults) &&
		reflect.DeepEqual(a.BranchPolicy, b.BranchPolicy) &&
		reflect.DeepEqual(a.Providers, b.Providers)
}

func globalDefaultsEqual(a GlobalDefaults, b GlobalDefaults) bool {
	return a.Agent == b.Agent && a.Assignee == b.Assignee && stringSlicesEqual(a.AgentLabels, b.AgentLabels)
}

func stringSlicesEqual(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
