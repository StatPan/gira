package gira

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	SetupGlobalModeGlobalOnly = "global-only"
	SetupGlobalModeHybrid     = "hybrid"
)

type SetupGlobalInput struct {
	Repo          RepoRef  `json:"repo,omitempty"`
	Path          string   `json:"path,omitempty"`
	ConfigRoot    string   `json:"config_root,omitempty"`
	WorkspaceName string   `json:"workspace,omitempty"`
	Owner         string   `json:"owner,omitempty"`
	InboxRepo     string   `json:"inbox_repo,omitempty"`
	ProjectOwner  string   `json:"project_owner,omitempty"`
	ProjectTitle  string   `json:"project_title,omitempty"`
	ProjectNumber int      `json:"project_number,omitempty"`
	Mode          string   `json:"mode,omitempty"`
	Agent         string   `json:"agent,omitempty"`
	Assignee      string   `json:"assignee,omitempty"`
	AgentLabels   []string `json:"agent_labels,omitempty"`
	Overwrite     bool     `json:"overwrite"`
	DryRun        bool     `json:"dry_run"`
	Apply         bool     `json:"apply"`
}

type SetupGlobalFilePlan struct {
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	Action  string `json:"action"`
	Content string `json:"content,omitempty"`
}

type SetupGlobalReport struct {
	Command         string                       `json:"command"`
	Mode            string                       `json:"mode"`
	ConfigRoot      string                       `json:"config_root"`
	Repo            string                       `json:"repo"`
	Path            string                       `json:"path"`
	Workspace       WorkspaceSummary             `json:"workspace"`
	InboxRepo       string                       `json:"inbox_repo"`
	InboxExplicit   bool                         `json:"inbox_explicit"`
	Defaults        GlobalDefaults               `json:"defaults,omitempty"`
	RepoContract    ConfigFileStatus             `json:"repo_contract"`
	Files           []SetupGlobalFilePlan        `json:"files"`
	GlobalConfig    GlobalConfig                 `json:"global_config"`
	GlobalWorkspace GlobalWorkspaceRegistryEntry `json:"global_workspace"`
	GlobalRepo      GlobalRepoRegistryEntry      `json:"global_repo"`
	DryRun          bool                         `json:"dry_run"`
	Applied         bool                         `json:"applied"`
	Status          string                       `json:"status"`
	Notes           []string                     `json:"notes,omitempty"`
	NextStep        string                       `json:"next_step,omitempty"`
}

func BuildSetupGlobalReport(input SetupGlobalInput, runner CommandRunner) (SetupGlobalReport, error) {
	if input.DryRun == input.Apply {
		return SetupGlobalReport{}, fmt.Errorf("exactly one of --dry-run or --apply is required")
	}
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	mode, err := normalizeSetupGlobalMode(input.Mode)
	if err != nil {
		return SetupGlobalReport{}, err
	}
	root, err := globalConfigRoot(input.ConfigRoot)
	if err != nil {
		return SetupGlobalReport{}, err
	}
	storedPath, checkoutPath, err := normalizeRepoRegisterPath(defaultSetupPath(input.Path))
	if err != nil {
		return SetupGlobalReport{}, err
	}
	repo, err := resolveSetupGlobalRepo(input.Repo, checkoutPath, root, runner)
	if err != nil {
		return SetupGlobalReport{}, err
	}
	if checkoutPath != "" {
		if err := validateRepoRegisterCheckout(repo, checkoutPath, runner); err != nil {
			return SetupGlobalReport{}, err
		}
	}
	inboxExplicit := strings.TrimSpace(input.InboxRepo) != ""
	inbox, err := setupGlobalInboxRepo(input.InboxRepo, repo)
	if err != nil {
		return SetupGlobalReport{}, err
	}
	workspaceName := strings.TrimSpace(input.WorkspaceName)
	if workspaceName == "" {
		workspaceName = "personal"
	}
	if !isSafeRegistryName(workspaceName) {
		return SetupGlobalReport{}, fmt.Errorf("--workspace must be non-empty and must not contain path separators")
	}
	owner := strings.TrimSpace(input.Owner)
	if owner == "" {
		owner = inbox.Owner
	}
	projectOwner := strings.TrimSpace(input.ProjectOwner)
	if projectOwner == "" {
		projectOwner = owner
	}
	projectTitle := strings.TrimSpace(input.ProjectTitle)
	if projectTitle == "" {
		projectTitle = workspaceName
	}
	if input.ProjectNumber < 0 {
		return SetupGlobalReport{}, fmt.Errorf("--project-number must be >= 0")
	}
	defaults := GlobalDefaults{
		Agent:       strings.TrimSpace(input.Agent),
		Assignee:    strings.TrimSpace(input.Assignee),
		AgentLabels: cleanSetupLabels(input.AgentLabels),
	}
	globalConfig := GlobalConfig{
		DefaultOwner:     owner,
		DefaultWorkspace: workspaceName,
		InboxRepo:        inbox.FullName(),
		Defaults:         defaults,
		Output:           GlobalOutputConfig{Format: "text", Color: "auto"},
	}
	if err := ValidateGlobalConfig(globalConfig, "setup global"); err != nil {
		return SetupGlobalReport{}, err
	}
	workspaceEntry := GlobalWorkspaceRegistryEntry{
		Workspace: WorkspaceConfig{
			Name:      workspaceName,
			Owner:     owner,
			InboxRepo: inbox.FullName(),
			Repos:     []string{repo.FullName()},
			Project: ProjectConfig{
				Owner:  projectOwner,
				Title:  projectTitle,
				Number: input.ProjectNumber,
			},
		},
	}
	if err := ValidateGlobalWorkspaceRegistryEntry(workspaceEntry, workspaceName, "setup global"); err != nil {
		return SetupGlobalReport{}, err
	}
	contract := ""
	contractPath := filepath.Join(checkoutPath, ".gira", "config.yaml")
	contractStatus := inspectConfigFile(contractPath, func() error {
		_, _, err := repoContextFromConfig(contractPath)
		return err
	})
	if mode == SetupGlobalModeHybrid && contractStatus.Exists {
		contract = ".gira/config.yaml"
	}
	repoEntry := GlobalRepoRegistryEntry{
		Repo:     repo.FullName(),
		Path:     storedPath,
		Aliases:  []string{},
		Contract: contract,
		Defaults: defaults,
		Workspace: GlobalRepoWorkspaceRef{
			Name: workspaceName,
		},
	}
	repoFile, err := GlobalRepoRegistryPath(root, repo)
	if err != nil {
		return SetupGlobalReport{}, err
	}
	if err := ValidateGlobalRepoRegistryEntry(repoEntry, repo, repoFile); err != nil {
		return SetupGlobalReport{}, err
	}
	configFile, err := GlobalConfigPath(root)
	if err != nil {
		return SetupGlobalReport{}, err
	}
	workspaceFile, err := GlobalWorkspaceRegistryPath(root, workspaceName)
	if err != nil {
		return SetupGlobalReport{}, err
	}
	plans := []SetupGlobalFilePlan{
		buildSetupFilePlan(configFile, renderSetupGlobalConfig(globalConfig), input.Overwrite),
		buildSetupFilePlan(workspaceFile, renderWorkspaceGlobalConfig(workspaceName, owner, inbox.FullName(), []string{repo.FullName()}, workspaceEntry.Workspace.Project), input.Overwrite),
	}
	plans = append(plans, buildSetupFilePlan(repoFile, renderSetupGlobalRepoEntry(repoEntry), input.Overwrite))
	report := SetupGlobalReport{
		Command:         "setup global",
		Mode:            mode,
		ConfigRoot:      root,
		Repo:            repo.FullName(),
		Path:            storedPath,
		Workspace:       WorkspaceSummary{Name: workspaceName, Owner: owner},
		InboxRepo:       inbox.FullName(),
		InboxExplicit:   inboxExplicit,
		Defaults:        defaults,
		RepoContract:    contractStatus,
		Files:           plans,
		GlobalConfig:    globalConfig,
		GlobalWorkspace: workspaceEntry,
		GlobalRepo:      repoEntry,
		DryRun:          input.DryRun,
		Status:          setupGlobalStatus(input.DryRun, plans),
		Notes:           setupGlobalNotes(mode, contractStatus, inboxExplicit, sameRepoRef(inbox, repo)),
		NextStep:        fmt.Sprintf("gira workspace status --config %s", QuoteShellArg(workspaceFile)),
	}
	if input.DryRun {
		if setupPlansHaveConflict(plans) {
			report.NextStep = "review conflicts or rerun with --overwrite"
		}
		return report, nil
	}
	if setupPlansHaveConflict(plans) {
		report.Status = "blocked"
		return report, fmt.Errorf("global setup would overwrite existing files; pass --overwrite to replace them")
	}
	for _, plan := range plans {
		if plan.Action == "skip" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(plan.Path), 0o755); err != nil {
			return report, fmt.Errorf("create config directory %q: %w", filepath.Dir(plan.Path), err)
		}
		if err := os.WriteFile(plan.Path, []byte(plan.Content), 0o644); err != nil {
			return report, fmt.Errorf("write setup file %q: %w", plan.Path, err)
		}
	}
	report.Applied = true
	if setupPlansAllSkipped(plans) {
		report.Status = "skipped"
		report.Applied = false
	}
	return report, nil
}

func normalizeSetupGlobalMode(value string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(value))
	if mode == "" {
		return SetupGlobalModeGlobalOnly, nil
	}
	switch mode {
	case SetupGlobalModeGlobalOnly:
		return mode, nil
	case "hybrid", "contract", "repo-contract":
		return SetupGlobalModeHybrid, nil
	default:
		return "", fmt.Errorf("--mode must be global-only or hybrid")
	}
}

func defaultSetupPath(value string) string {
	if strings.TrimSpace(value) == "" {
		return "."
	}
	return value
}

func resolveSetupGlobalRepo(repo RepoRef, checkoutPath string, configRoot string, runner CommandRunner) (RepoRef, error) {
	if repoRefIsSet(repo) {
		return repo, nil
	}
	ctx, err := ResolveRepoContextDetails(RepoContextOptions{WorkDir: checkoutPath, ConfigRoot: configRoot, Runner: runner})
	if err != nil {
		return RepoRef{}, fmt.Errorf("repo could not be inferred; pass --repo OWNER/REPO: %w", err)
	}
	return ctx.Repo, nil
}

func setupGlobalInboxRepo(value string, fallback RepoRef) (RepoRef, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	repo, err := ParseRepoRef(value)
	if err != nil {
		return RepoRef{}, fmt.Errorf("--inbox-repo must be in OWNER/REPO format")
	}
	return repo, nil
}

func cleanSetupLabels(values []string) []string {
	seen := map[string]struct{}{}
	labels := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			key := strings.ToLower(trimmed)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			labels = append(labels, trimmed)
		}
	}
	return labels
}

func buildSetupFilePlan(path string, content string, overwrite bool) SetupGlobalFilePlan {
	plan := SetupGlobalFilePlan{Path: path, Action: "create", Content: content}
	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return plan
		}
		plan.Exists = true
		plan.Action = "conflict"
		return plan
	}
	plan.Exists = true
	if bytes.Equal(bytes.TrimSpace(existing), bytes.TrimSpace([]byte(content))) {
		plan.Action = "skip"
		return plan
	}
	if overwrite {
		plan.Action = "overwrite"
		return plan
	}
	plan.Action = "conflict"
	return plan
}

func setupGlobalStatus(dryRun bool, plans []SetupGlobalFilePlan) string {
	if setupPlansHaveConflict(plans) {
		return "blocked"
	}
	if setupPlansAllSkipped(plans) {
		return "skipped"
	}
	if dryRun {
		return "planned"
	}
	return "applied"
}

func setupPlansHaveConflict(plans []SetupGlobalFilePlan) bool {
	for _, plan := range plans {
		if plan.Action == "conflict" {
			return true
		}
	}
	return false
}

func setupPlansAllSkipped(plans []SetupGlobalFilePlan) bool {
	if len(plans) == 0 {
		return false
	}
	for _, plan := range plans {
		if plan.Action != "skip" {
			return false
		}
	}
	return true
}

func setupGlobalNotes(mode string, contract ConfigFileStatus, inboxExplicit bool, inboxMatchesRepo bool) []string {
	var notes []string
	if !inboxExplicit {
		notes = append(notes, "--inbox-repo was not provided; using the target repo as a single-repo inbox. For multi-repo global operation, prefer a dedicated backlog repo such as OWNER/backlog.")
	} else if inboxMatchesRepo {
		notes = append(notes, "inbox repo matches the execution repo; this is acceptable for single-repo operation, but a dedicated backlog repo is clearer for multi-repo workspaces.")
	} else {
		notes = append(notes, "inbox repo is separate from the execution repo and will act as the workspace backlog/intake queue.")
	}
	if contract.Exists && mode == SetupGlobalModeGlobalOnly {
		notes = append(notes, "repo-local .gira/config.yaml exists but global-only mode does not reference it")
	}
	if contract.Exists && mode == SetupGlobalModeHybrid {
		notes = append(notes, "repo-local .gira/config.yaml exists and will be referenced as the optional shared contract")
	}
	if !contract.Exists {
		notes = append(notes, "no repo-local .gira/config.yaml detected; setup uses global registry only")
	}
	notes = append(notes, "interactive prompts can be layered later; this command keeps a deterministic dry-run/apply contract")
	return notes
}

func renderSetupGlobalConfig(cfg GlobalConfig) string {
	var b strings.Builder
	if strings.TrimSpace(cfg.DefaultOwner) != "" {
		fmt.Fprintf(&b, "default_owner: %s\n", cfg.DefaultOwner)
	}
	if strings.TrimSpace(cfg.DefaultWorkspace) != "" {
		fmt.Fprintf(&b, "default_workspace: %s\n", cfg.DefaultWorkspace)
	}
	if strings.TrimSpace(cfg.InboxRepo) != "" {
		fmt.Fprintf(&b, "inbox_repo: %s\n", cfg.InboxRepo)
	}
	b.WriteString("\n")
	b.WriteString("defaults:\n")
	fmt.Fprintf(&b, "  agent: %s\n", yamlQuotedString(cfg.Defaults.Agent))
	fmt.Fprintf(&b, "  assignee: %s\n", yamlQuotedString(cfg.Defaults.Assignee))
	if len(cfg.Defaults.AgentLabels) == 0 {
		b.WriteString("  agent_labels: []\n")
	} else {
		b.WriteString("  agent_labels:\n")
		for _, label := range cfg.Defaults.AgentLabels {
			fmt.Fprintf(&b, "    - %s\n", yamlQuotedString(label))
		}
	}
	b.WriteString("\n")
	b.WriteString("output:\n")
	fmt.Fprintf(&b, "  format: %s\n", yamlQuotedString(cfg.Output.Format))
	fmt.Fprintf(&b, "  color: %s\n", yamlQuotedString(cfg.Output.Color))
	return b.String()
}

func renderSetupGlobalRepoEntry(entry GlobalRepoRegistryEntry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "repo: %s\n", entry.Repo)
	if strings.TrimSpace(entry.Path) != "" {
		fmt.Fprintf(&b, "path: %s\n", entry.Path)
	}
	if len(entry.Aliases) > 0 {
		b.WriteString("aliases:\n")
		for _, alias := range entry.Aliases {
			fmt.Fprintf(&b, "  - %s\n", yamlQuotedString(alias))
		}
	}
	if strings.TrimSpace(entry.Contract) != "" {
		fmt.Fprintf(&b, "contract: %s\n", entry.Contract)
	}
	b.WriteString("defaults:\n")
	fmt.Fprintf(&b, "  agent: %s\n", yamlQuotedString(entry.Defaults.Agent))
	fmt.Fprintf(&b, "  assignee: %s\n", yamlQuotedString(entry.Defaults.Assignee))
	if len(entry.Defaults.AgentLabels) == 0 {
		b.WriteString("  agent_labels: []\n")
	} else {
		b.WriteString("  agent_labels:\n")
		for _, label := range entry.Defaults.AgentLabels {
			fmt.Fprintf(&b, "    - %s\n", yamlQuotedString(label))
		}
	}
	if strings.TrimSpace(entry.Workspace.Name) != "" {
		b.WriteString("workspace:\n")
		fmt.Fprintf(&b, "  name: %s\n", yamlQuotedString(entry.Workspace.Name))
	}
	return b.String()
}

func FormatSetupGlobalReport(report SetupGlobalReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "setup global: %s %s\n", report.Status, report.Mode)
	fmt.Fprintf(&b, "config root: %s\n", report.ConfigRoot)
	fmt.Fprintf(&b, "repo: %s\n", report.Repo)
	fmt.Fprintf(&b, "workspace: %s (%s)\n", report.Workspace.Name, report.Workspace.Owner)
	fmt.Fprintf(&b, "inbox: %s\n", report.InboxRepo)
	if report.RepoContract.Exists {
		fmt.Fprintf(&b, "repo-local contract: %s valid=%t\n", report.RepoContract.Path, report.RepoContract.Valid)
	}
	for _, plan := range report.Files {
		fmt.Fprintf(&b, "file: %s action=%s\n", plan.Path, plan.Action)
		if report.DryRun && strings.TrimSpace(plan.Content) != "" {
			b.WriteString(strings.TrimRight(plan.Content, "\n"))
			b.WriteString("\n")
		}
	}
	for _, note := range report.Notes {
		fmt.Fprintf(&b, "note: %s\n", note)
	}
	if strings.TrimSpace(report.NextStep) != "" {
		fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	}
	return b.String()
}
