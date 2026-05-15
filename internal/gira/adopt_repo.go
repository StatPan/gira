package gira

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type AdoptRepoStrategy string

const (
	AdoptRepoStrategyObserve   AdoptRepoStrategy = "observe"
	AdoptRepoStrategyMerge     AdoptRepoStrategy = "merge"
	AdoptRepoStrategyNormalize AdoptRepoStrategy = "normalize"
)

type AdoptRepoInput struct {
	Repo     RepoRef `json:"repo"`
	Path     string  `json:"path"`
	Strategy string  `json:"strategy,omitempty"`
	Yes      bool    `json:"yes,omitempty"`
	DryRun   bool    `json:"dry_run"`
	Apply    bool    `json:"apply"`
}

type AdoptRepoReport struct {
	Repo           string               `json:"repo"`
	Path           string               `json:"path"`
	DryRun         bool                 `json:"dry_run"`
	Apply          bool                 `json:"apply"`
	Strategy       string               `json:"strategy"`
	Recommendation string               `json:"recommendation"`
	Counts         AdoptRepoCounts      `json:"counts"`
	Local          AdoptRepoLocalState  `json:"local"`
	GitHub         AdoptRepoGitHubState `json:"github"`
	Actions        []AdoptRepoAction    `json:"actions,omitempty"`
	Warnings       []string             `json:"warnings,omitempty"`
	NextStep       string               `json:"next_step"`
}

type AdoptRepoCounts struct {
	Labels          int `json:"labels"`
	Milestones      int `json:"milestones"`
	OpenIssues      int `json:"open_issues"`
	UnmappedIssues  int `json:"unmapped_issues"`
	Projects        int `json:"projects"`
	PlannedActions  int `json:"planned_actions"`
	AppliedActions  int `json:"applied_actions"`
	ConflictActions int `json:"conflict_actions"`
}

type AdoptRepoLocalState struct {
	ConfigPath             string `json:"config_path"`
	ConfigExists           bool   `json:"config_exists"`
	AgentsPath             string `json:"agents_path"`
	AgentsExists           bool   `json:"agents_exists"`
	AgentsManagedBlock     string `json:"agents_managed_block"`
	PRTemplateExists       bool   `json:"pr_template_exists"`
	IssueTemplateDirExists bool   `json:"issue_template_dir_exists"`
	IssueTemplateCount     int    `json:"issue_template_count"`
}

type AdoptRepoGitHubState struct {
	ExistingLabels     []string         `json:"existing_labels,omitempty"`
	ExistingMilestones []string         `json:"existing_milestones,omitempty"`
	UnmappedIssues     []AdoptIssueItem `json:"unmapped_issues,omitempty"`
	Projects           []string         `json:"projects,omitempty"`
	ProjectsWarning    string           `json:"projects_warning,omitempty"`
}

type AdoptRepoAction struct {
	Action string `json:"action"`
	Target string `json:"target"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func BuildAdoptRepoReport(input AdoptRepoInput, runner CommandRunner) (AdoptRepoReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if input.DryRun == input.Apply {
		return AdoptRepoReport{}, fmt.Errorf("exactly one of --dry-run or --apply is required")
	}
	strategy, err := parseAdoptRepoStrategy(input.Strategy)
	if err != nil {
		return AdoptRepoReport{}, err
	}
	if strategy == "" {
		strategy = AdoptRepoStrategyMerge
	}
	if input.Apply && strings.TrimSpace(input.Strategy) == "" && !input.Yes {
		return AdoptRepoReport{}, fmt.Errorf("--apply requires --strategy observe|merge|normalize or --yes")
	}
	path := strings.TrimSpace(input.Path)
	if path == "" {
		path = "."
	}
	absPath, _ := filepath.Abs(path)
	report := AdoptRepoReport{
		Repo:           input.Repo.FullName(),
		Path:           absPath,
		DryRun:         input.DryRun,
		Apply:          input.Apply,
		Strategy:       string(strategy),
		Recommendation: string(AdoptRepoStrategyMerge),
		Local:          inspectAdoptRepoLocal(absPath),
	}

	labels, err := NewGHSyncClient(input.Repo, runner).ListLabels()
	if err != nil {
		return report, err
	}
	for _, label := range labels {
		report.GitHub.ExistingLabels = append(report.GitHub.ExistingLabels, label.Name)
	}
	sort.Strings(report.GitHub.ExistingLabels)
	report.Counts.Labels = len(report.GitHub.ExistingLabels)

	milestones, err := NewGHSyncClient(input.Repo, runner).ListMilestones()
	if err != nil {
		return report, err
	}
	for _, milestone := range milestones {
		report.GitHub.ExistingMilestones = append(report.GitHub.ExistingMilestones, milestone.Title)
	}
	sort.Strings(report.GitHub.ExistingMilestones)
	report.Counts.Milestones = len(report.GitHub.ExistingMilestones)

	issues, err := fetchAdoptIssues(input.Repo, "open", runner)
	if err != nil {
		return report, err
	}
	report.Counts.OpenIssues = len(issues)
	for _, issue := range issues {
		item, unmapped := adoptIssueItem(issue)
		if unmapped {
			report.GitHub.UnmappedIssues = append(report.GitHub.UnmappedIssues, item)
		}
	}
	report.Counts.UnmappedIssues = len(report.GitHub.UnmappedIssues)

	projects, projectsWarning := fetchAdoptRepoProjects(input.Repo, runner)
	report.GitHub.Projects = projects
	report.GitHub.ProjectsWarning = projectsWarning
	report.Counts.Projects = len(projects)
	if projectsWarning != "" {
		report.Warnings = append(report.Warnings, projectsWarning)
	}

	report.Actions = planAdoptRepoActions(report, strategy)
	for i := range report.Actions {
		switch report.Actions[i].Status {
		case "planned":
			report.Counts.PlannedActions++
		case "conflict":
			report.Counts.ConflictActions++
		}
	}
	if input.Apply {
		if err := applyAdoptRepoActions(absPath, &report); err != nil {
			return report, err
		}
	}
	report.NextStep = adoptRepoNextStep(report)
	return report, nil
}

func parseAdoptRepoStrategy(value string) (AdoptRepoStrategy, error) {
	trimmed := AdoptRepoStrategy(strings.ToLower(strings.TrimSpace(value)))
	switch trimmed {
	case "", AdoptRepoStrategyObserve, AdoptRepoStrategyMerge, AdoptRepoStrategyNormalize:
		return trimmed, nil
	default:
		return "", fmt.Errorf("--strategy must be one of observe, merge, normalize")
	}
}

func inspectAdoptRepoLocal(path string) AdoptRepoLocalState {
	state := AdoptRepoLocalState{
		ConfigPath: filepath.Join(path, ".gira", "config.yaml"),
		AgentsPath: "AGENTS.md",
	}
	state.ConfigExists = fileExists(state.ConfigPath)
	agentsPath := filepath.Join(path, "AGENTS.md")
	state.AgentsExists = fileExists(agentsPath)
	if state.AgentsExists {
		state.AgentsManagedBlock = "missing"
		content, err := os.ReadFile(agentsPath)
		if err == nil {
			text := string(content)
			if strings.Contains(text, AgentsManagedBlockStart) && strings.Contains(text, AgentsManagedBlockEnd) {
				state.AgentsManagedBlock = "present"
			}
		}
	} else {
		state.AgentsManagedBlock = "absent"
	}
	state.PRTemplateExists = fileExists(filepath.Join(path, ".github", "PULL_REQUEST_TEMPLATE.md"))
	issueTemplateDir := filepath.Join(path, ".github", "ISSUE_TEMPLATE")
	if info, err := os.Stat(issueTemplateDir); err == nil && info.IsDir() {
		state.IssueTemplateDirExists = true
		entries, _ := os.ReadDir(issueTemplateDir)
		for _, entry := range entries {
			if !entry.IsDir() {
				state.IssueTemplateCount++
			}
		}
	}
	return state
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func fetchAdoptRepoProjects(repo RepoRef, runner CommandRunner) ([]string, string) {
	output, err := runner.Run("gh", "project", "list", "--owner", repo.Owner, "--format", "json", "--limit", "100")
	if err != nil {
		return nil, "projects_v2_detection_skipped: " + err.Error()
	}
	var raw struct {
		Projects []struct {
			Title  string `json:"title"`
			Number int    `json:"number"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, "projects_v2_detection_skipped: parse gh JSON: " + err.Error()
	}
	projects := make([]string, 0, len(raw.Projects))
	for _, project := range raw.Projects {
		if strings.TrimSpace(project.Title) == "" {
			continue
		}
		projects = append(projects, fmt.Sprintf("%s (#%d)", project.Title, project.Number))
	}
	sort.Strings(projects)
	return projects, ""
}

func planAdoptRepoActions(report AdoptRepoReport, strategy AdoptRepoStrategy) []AdoptRepoAction {
	actions := []AdoptRepoAction{}
	if strategy == AdoptRepoStrategyObserve {
		actions = append(actions, AdoptRepoAction{Action: "config:observe", Target: report.Local.ConfigPath, Status: "planned", Reason: "observe strategy does not write local config"})
		return actions
	}
	if report.Local.ConfigExists {
		actions = append(actions, AdoptRepoAction{Action: "config:skip", Target: report.Local.ConfigPath, Status: "skipped", Reason: "existing Gira config is preserved"})
	} else {
		actions = append(actions, AdoptRepoAction{Action: "config:create", Target: report.Local.ConfigPath, Status: "planned", Reason: "minimal contract for Gira repo context and mappings"})
	}
	if report.Local.AgentsExists {
		if report.Local.AgentsManagedBlock == "present" {
			actions = append(actions, AdoptRepoAction{Action: "agents:managed-block:skip", Target: "AGENTS.md", Status: "skipped", Reason: "Gira managed block already exists"})
		} else {
			actions = append(actions, AdoptRepoAction{Action: "agents:managed-block:insert", Target: "AGENTS.md", Status: "planned", Reason: "preserve existing instructions and add only the Gira contract block"})
		}
	} else {
		actions = append(actions, AdoptRepoAction{Action: "agents:create", Target: "AGENTS.md", Status: "planned", Reason: "create minimal Gira agent contract"})
	}
	if report.Local.PRTemplateExists {
		actions = append(actions, AdoptRepoAction{Action: "pr-template:preserve", Target: ".github/PULL_REQUEST_TEMPLATE.md", Status: "skipped", Reason: "existing PR template is user-owned"})
	} else {
		actions = append(actions, AdoptRepoAction{Action: "pr-template:recommend", Target: ".github/PULL_REQUEST_TEMPLATE.md", Status: "planned", Reason: "optional; use full bootstrap only for fresh repos"})
	}
	if report.Local.IssueTemplateDirExists {
		actions = append(actions, AdoptRepoAction{Action: "issue-templates:preserve", Target: ".github/ISSUE_TEMPLATE", Status: "skipped", Reason: "existing issue templates are user-owned"})
	}
	if report.Counts.UnmappedIssues > 0 {
		actions = append(actions, AdoptRepoAction{Action: "issues:map", Target: "existing GitHub issues", Status: "planned", Reason: "run targeted gira adopt issues mappings for missing type/status/milestone"})
	}
	if report.Counts.Projects > 0 {
		actions = append(actions, AdoptRepoAction{Action: "projects:link", Target: "existing GitHub Projects", Status: "planned", Reason: "record selected project in .gira/config.yaml before projects sync"})
	}
	if strategy == AdoptRepoStrategyNormalize {
		actions = append(actions, AdoptRepoAction{Action: "metadata:normalize", Target: "labels/milestones/issues", Status: "planned", Reason: "normalize strategy should be followed by explicit sync/adopt issue apply commands"})
	}
	return actions
}

func applyAdoptRepoActions(path string, report *AdoptRepoReport) error {
	for i := range report.Actions {
		action := &report.Actions[i]
		if action.Status != "planned" {
			continue
		}
		switch action.Action {
		case "config:create":
			if err := writeAdoptRepoConfig(path, report.Repo); err != nil {
				return err
			}
			action.Status = "applied"
			report.Counts.AppliedActions++
		case "agents:managed-block:insert", "agents:create":
			if err := upsertAgentsManagedBlock(filepath.Join(path, "AGENTS.md")); err != nil {
				return err
			}
			action.Status = "applied"
			report.Counts.AppliedActions++
		default:
			action.Status = "planned"
		}
	}
	return nil
}

func writeAdoptRepoConfig(path string, repo string) error {
	configPath := filepath.Join(path, ".gira", "config.yaml")
	if fileExists(configPath) {
		return nil
	}
	content := fmt.Sprintf(`repo: %s
profiles:
  default:
    labels:
      - type:epic
      - type:story
      - type:task
      - type:bug
      - status:ready
      - status:in-progress
      - status:in-review
      - status:done
    milestones:
      - MVP
      - Beta
      - v1
    issue_templates: []
    review_policy:
      required_approvals: 0
      require_code_owners: false
`, repo)
	return writeSafeLocalFile(configPath, []byte(content), 0o644)
}

func upsertAgentsManagedBlock(path string) error {
	if err := prepareSafeLocalFile(path); err != nil {
		return err
	}
	block := RenderAgentsManagedBlock(CoreAgentGuidanceSpec(), CoreCommandSpecs())
	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return writeSafeLocalFile(path, []byte("# Repository Instructions\n\n"+block), 0o644)
		}
		return err
	}
	text := string(existing)
	startAt := strings.Index(text, AgentsManagedBlockStart)
	endAt := strings.Index(text, AgentsManagedBlockEnd)
	if startAt >= 0 && endAt > startAt {
		endAt += len(AgentsManagedBlockEnd)
		updated := strings.TrimRight(text[:startAt], "\n") + "\n\n" + strings.TrimRight(block, "\n") + "\n" + text[endAt:]
		return writeSafeLocalFile(path, []byte(updated), 0o644)
	}
	separator := "\n\n"
	if strings.HasSuffix(text, "\n\n") {
		separator = ""
	} else if strings.HasSuffix(text, "\n") {
		separator = "\n"
	}
	return writeSafeLocalFile(path, []byte(text+separator+block), 0o644)
}

func adoptRepoNextStep(report AdoptRepoReport) string {
	if report.DryRun {
		return fmt.Sprintf("gira adopt repo --repo %s --path %s --strategy %s --apply", QuoteShellArg(report.Repo), QuoteShellArg(report.Path), QuoteShellArg(report.Strategy))
	}
	if report.Counts.UnmappedIssues > 0 {
		return fmt.Sprintf("gira adopt issues --repo %s --dry-run", QuoteShellArg(report.Repo))
	}
	return fmt.Sprintf("gira ops sync --repo %s --dry-run", QuoteShellArg(report.Repo))
}

func FormatAdoptRepoReport(report AdoptRepoReport) string {
	var b strings.Builder
	mode := "dry-run"
	if report.Apply {
		mode = "applied"
	}
	fmt.Fprintf(&b, "adopt repo: %s repo=%s strategy=%s recommendation=%s\n", mode, report.Repo, report.Strategy, report.Recommendation)
	fmt.Fprintf(&b, "local: config=%t agents=%t agents_block=%s pr_template=%t issue_templates=%d\n", report.Local.ConfigExists, report.Local.AgentsExists, report.Local.AgentsManagedBlock, report.Local.PRTemplateExists, report.Local.IssueTemplateCount)
	fmt.Fprintf(&b, "github: labels=%d milestones=%d open_issues=%d unmapped_issues=%d projects=%d\n", report.Counts.Labels, report.Counts.Milestones, report.Counts.OpenIssues, report.Counts.UnmappedIssues, report.Counts.Projects)
	for _, warning := range report.Warnings {
		fmt.Fprintf(&b, "warning: %s\n", warning)
	}
	if len(report.GitHub.UnmappedIssues) > 0 {
		b.WriteString("unmapped issues:\n")
		for _, item := range report.GitHub.UnmappedIssues {
			fmt.Fprintf(&b, "  #%d %s reasons=%s\n", item.Number, item.Title, strings.Join(item.Reasons, ","))
		}
	}
	if len(report.Actions) > 0 {
		b.WriteString("actions:\n")
		for _, action := range report.Actions {
			fmt.Fprintf(&b, "  %s %s target=%s reason=%s\n", action.Status, action.Action, action.Target, action.Reason)
		}
	}
	if report.NextStep != "" {
		fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	}
	return b.String()
}
