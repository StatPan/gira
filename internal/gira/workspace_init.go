package gira

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type WorkspaceInitInput struct {
	Name          string   `json:"name,omitempty"`
	Owner         string   `json:"owner,omitempty"`
	InboxRepo     string   `json:"inbox_repo"`
	Repos         []string `json:"repos"`
	ProjectOwner  string   `json:"project_owner,omitempty"`
	ProjectTitle  string   `json:"project_title,omitempty"`
	ProjectNumber int      `json:"project_number,omitempty"`
	Scope         string   `json:"scope,omitempty"`
	ConfigRoot    string   `json:"config_root,omitempty"`
	Path          string   `json:"path"`
	Overwrite     bool     `json:"overwrite"`
	DryRun        bool     `json:"dry_run"`
	Apply         bool     `json:"apply"`
}

type WorkspaceInitReport struct {
	Command     string           `json:"command"`
	Scope       string           `json:"scope"`
	ConfigRoot  string           `json:"config_root,omitempty"`
	ConfigPath  string           `json:"config_path"`
	DryRun      bool             `json:"dry_run"`
	Applied     bool             `json:"applied"`
	Created     bool             `json:"created"`
	Overwritten bool             `json:"overwritten"`
	Skipped     bool             `json:"skipped"`
	Workspace   WorkspaceSummary `json:"workspace"`
	Project     ProjectConfig    `json:"project"`
	InboxRepo   string           `json:"inbox_repo"`
	Repos       []string         `json:"repos"`
	Content     string           `json:"content"`
	NextStep    string           `json:"next_step"`
}

func BuildWorkspaceInitReport(input WorkspaceInitInput) (WorkspaceInitReport, error) {
	if input.DryRun == input.Apply {
		return WorkspaceInitReport{}, fmt.Errorf("exactly one of --dry-run or --apply is required")
	}
	scope, err := normalizeWorkspaceInitScope(input.Scope)
	if err != nil {
		return WorkspaceInitReport{}, err
	}
	if strings.TrimSpace(input.Path) == "" {
		input.Path = "."
	}
	inbox, err := ParseRepoRef(input.InboxRepo)
	if err != nil {
		return WorkspaceInitReport{}, fmt.Errorf("--inbox-repo must be in OWNER/REPO format")
	}
	repos := append([]string{}, input.Repos...)
	if len(repos) == 0 {
		repos = []string{inbox.FullName()}
	}
	seen := map[string]struct{}{}
	normalizedRepos := make([]string, 0, len(repos))
	for i, value := range repos {
		repo, err := ParseRepoRef(value)
		if err != nil {
			return WorkspaceInitReport{}, fmt.Errorf("--repo[%d] must be in OWNER/REPO format", i)
		}
		key := strings.ToLower(repo.FullName())
		if _, ok := seen[key]; ok {
			return WorkspaceInitReport{}, fmt.Errorf("--repo contains duplicate repo %s", repo.FullName())
		}
		seen[key] = struct{}{}
		normalizedRepos = append(normalizedRepos, repo.FullName())
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = inbox.Name
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
		projectTitle = name
	}
	if input.ProjectNumber < 0 {
		return WorkspaceInitReport{}, fmt.Errorf("--project-number must be >= 0")
	}
	project := ProjectConfig{Owner: projectOwner, Title: projectTitle, Number: input.ProjectNumber}
	configPath := DefaultInitConfigPath(input.Path)
	configRoot := ""
	content := renderWorkspaceInitConfig(name, owner, inbox.FullName(), normalizedRepos, project)
	if scope == "global" {
		root, err := globalConfigRoot(input.ConfigRoot)
		if err != nil {
			return WorkspaceInitReport{}, err
		}
		configRoot = root
		configPath, err = GlobalWorkspaceRegistryPath(root, name)
		if err != nil {
			return WorkspaceInitReport{}, err
		}
		content = renderWorkspaceGlobalConfig(name, owner, inbox.FullName(), normalizedRepos, project)
	}
	report := WorkspaceInitReport{
		Command:    "workspace init",
		Scope:      scope,
		ConfigRoot: configRoot,
		ConfigPath: configPath,
		DryRun:     input.DryRun,
		Workspace:  WorkspaceSummary{Name: name, Owner: owner},
		Project:    project,
		InboxRepo:  inbox.FullName(),
		Repos:      normalizedRepos,
		Content:    content,
		NextStep:   "gira workspace status --config " + QuoteShellArg(configPath),
	}
	if input.DryRun {
		return report, nil
	}
	if existing, err := os.ReadFile(configPath); err == nil {
		if bytes.Equal(bytes.TrimSpace(existing), bytes.TrimSpace([]byte(content))) {
			report.Skipped = true
			return report, nil
		}
		if !input.Overwrite {
			return report, fmt.Errorf("%s already exists; pass --overwrite to replace it", configPath)
		}
		report.Overwritten = true
	} else if os.IsNotExist(err) {
		report.Created = true
	} else {
		return report, err
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return report, err
	}
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		return report, err
	}
	if scope == "global" {
		entry, err := LoadGlobalWorkspaceRegistryEntry(configRoot, name)
		if err != nil {
			return report, err
		}
		report.Project = entry.Workspace.Project
	} else {
		resolved, err := ResolveWorkspaceConfig(configPath)
		if err != nil {
			return report, err
		}
		report.Project = resolved.Project
	}
	report.Applied = true
	return report, nil
}

func normalizeWorkspaceInitScope(value string) (string, error) {
	scope := strings.ToLower(strings.TrimSpace(value))
	if scope == "" {
		return "repo", nil
	}
	switch scope {
	case "repo", "global":
		return scope, nil
	default:
		return "", fmt.Errorf("--scope must be repo or global")
	}
}

func renderWorkspaceInitConfig(name string, owner string, inboxRepo string, repos []string, project ProjectConfig) string {
	var b strings.Builder
	fmt.Fprintf(&b, "repo: %s\n\n", inboxRepo)
	b.WriteString("workspace:\n")
	fmt.Fprintf(&b, "  name: %s\n", yamlQuotedString(name))
	fmt.Fprintf(&b, "  owner: %s\n", owner)
	fmt.Fprintf(&b, "  inbox_repo: %s\n", inboxRepo)
	b.WriteString("  repos:\n")
	for _, repo := range repos {
		fmt.Fprintf(&b, "    - %s\n", repo)
	}
	b.WriteString("  project:\n")
	fmt.Fprintf(&b, "    owner: %s\n", project.Owner)
	fmt.Fprintf(&b, "    title: %s\n", yamlQuotedString(project.Title))
	if project.Number > 0 {
		fmt.Fprintf(&b, "    number: %d\n", project.Number)
	}
	b.WriteString("\nprofiles:\n")
	b.WriteString("  default:\n")
	b.WriteString("    labels: [\"type:epic\", \"type:story\", \"type:task\", \"type:bug\", \"status:ready\", \"status:in-progress\", \"status:done\"]\n")
	b.WriteString("    milestones: [\"MVP\", \"Beta\", \"v1\"]\n")
	b.WriteString("    issue_templates: [\"epic\", \"story\", \"task\", \"bug\"]\n")
	b.WriteString("    review_policy:\n")
	b.WriteString("      required_approvals: 0\n")
	b.WriteString("      require_code_owners: false\n")
	return b.String()
}

func renderWorkspaceGlobalConfig(name string, owner string, inboxRepo string, repos []string, project ProjectConfig) string {
	var b strings.Builder
	b.WriteString("workspace:\n")
	fmt.Fprintf(&b, "  name: %s\n", yamlQuotedString(name))
	fmt.Fprintf(&b, "  owner: %s\n", owner)
	fmt.Fprintf(&b, "  inbox_repo: %s\n", inboxRepo)
	b.WriteString("  repos:\n")
	for _, repo := range repos {
		fmt.Fprintf(&b, "    - %s\n", repo)
	}
	b.WriteString("  project:\n")
	fmt.Fprintf(&b, "    owner: %s\n", project.Owner)
	fmt.Fprintf(&b, "    title: %s\n", yamlQuotedString(project.Title))
	if project.Number > 0 {
		fmt.Fprintf(&b, "    number: %d\n", project.Number)
	}
	return b.String()
}

func yamlQuotedString(value string) string {
	return strconv.Quote(value)
}

func FormatWorkspaceInitReport(report WorkspaceInitReport) string {
	mode := "dry-run"
	if report.Applied {
		mode = "applied"
	} else if report.Skipped {
		mode = "skipped"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "workspace init: %s %s %s\n", mode, report.Scope, report.ConfigPath)
	fmt.Fprintf(&b, "workspace: %s (%s)\n", report.Workspace.Name, report.Workspace.Owner)
	if report.Project.Number > 0 {
		fmt.Fprintf(&b, "project: %s/%s #%d\n", report.Project.Owner, report.Project.Title, report.Project.Number)
	} else {
		fmt.Fprintf(&b, "project: %s/%s\n", report.Project.Owner, report.Project.Title)
	}
	fmt.Fprintf(&b, "inbox: %s\n", report.InboxRepo)
	fmt.Fprintf(&b, "repos: %s\n", strings.Join(report.Repos, ","))
	if report.DryRun {
		b.WriteString("config:\n")
		b.WriteString(strings.TrimRight(report.Content, "\n"))
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}
