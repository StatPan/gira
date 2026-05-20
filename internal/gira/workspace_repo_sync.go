package gira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type WorkspaceRepoSyncInput struct {
	WorkspaceName   string `json:"workspace,omitempty"`
	Owner           string `json:"owner,omitempty"`
	ConfigRoot      string `json:"config_root,omitempty"`
	Limit           int    `json:"limit,omitempty"`
	IncludeArchived bool   `json:"include_archived,omitempty"`
	DryRun          bool   `json:"dry_run"`
	Apply           bool   `json:"apply"`
}

type WorkspaceRepoSyncReport struct {
	Command         string                       `json:"command"`
	ConfigRoot      string                       `json:"config_root"`
	ConfigPath      string                       `json:"config_path"`
	Owner           string                       `json:"owner"`
	Workspace       WorkspaceSummary             `json:"workspace"`
	InboxRepo       string                       `json:"inbox_repo"`
	ExistingRepos   []string                     `json:"existing_repos"`
	DiscoveredRepos []string                     `json:"discovered_repos"`
	TargetRepos     []string                     `json:"target_repos"`
	AddedRepos      []string                     `json:"added_repos"`
	RemovedRepos    []string                     `json:"removed_repos,omitempty"`
	SkippedRepos    []string                     `json:"skipped_repos,omitempty"`
	File            SetupGlobalFilePlan          `json:"file"`
	DryRun          bool                         `json:"dry_run"`
	Applied         bool                         `json:"applied"`
	Status          string                       `json:"status"`
	Notes           []string                     `json:"notes,omitempty"`
	NextStep        string                       `json:"next_step,omitempty"`
	GlobalWorkspace GlobalWorkspaceRegistryEntry `json:"global_workspace"`
}

func BuildWorkspaceRepoSyncReport(input WorkspaceRepoSyncInput, runner CommandRunner) (WorkspaceRepoSyncReport, error) {
	if input.DryRun == input.Apply {
		return WorkspaceRepoSyncReport{}, fmt.Errorf("exactly one of --dry-run or --apply is required")
	}
	if input.Limit < 1 {
		input.Limit = 100
	}
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	root, err := globalConfigRoot(input.ConfigRoot)
	if err != nil {
		return WorkspaceRepoSyncReport{}, err
	}
	workspaceName, err := workspaceRepoSyncName(root, input.WorkspaceName)
	if err != nil {
		return WorkspaceRepoSyncReport{}, err
	}
	entry, err := LoadGlobalWorkspaceRegistryEntry(root, workspaceName)
	if err != nil {
		return WorkspaceRepoSyncReport{}, err
	}
	path, err := GlobalWorkspaceRegistryPath(root, workspaceName)
	if err != nil {
		return WorkspaceRepoSyncReport{}, err
	}
	owner := strings.TrimSpace(input.Owner)
	if owner == "" {
		owner = entry.Workspace.Owner
	}
	if owner == "" {
		return WorkspaceRepoSyncReport{}, fmt.Errorf("--owner is required when workspace.owner is empty")
	}
	discovered, err := DiscoverOwnerRepos(owner, input.Limit, input.IncludeArchived, runner)
	if err != nil {
		return WorkspaceRepoSyncReport{}, err
	}
	inboxRepo, err := ParseRepoRef(entry.Workspace.InboxRepo)
	if err != nil {
		return WorkspaceRepoSyncReport{}, fmt.Errorf("workspace inbox repo is invalid: %w", err)
	}
	existing := parseWorkspaceRepos(entry.Workspace.Repos)
	target := make([]RepoRef, 0, len(discovered))
	var skipped []string
	for _, repo := range discovered {
		if sameRepoRef(repo, inboxRepo) {
			skipped = append(skipped, repo.FullName())
			continue
		}
		target = append(target, repo)
	}
	target = uniqueSortedRepoRefs(target, 0)
	targetStrings := repoRefsToStrings(target)
	existingStrings := repoRefsToStrings(existing)
	added, removed := diffRepoStrings(existingStrings, targetStrings)
	nextEntry := entry
	nextEntry.Workspace.Repos = targetStrings
	if err := ValidateGlobalWorkspaceRegistryEntry(nextEntry, workspaceName, path); err != nil {
		return WorkspaceRepoSyncReport{}, err
	}
	content := renderGlobalWorkspaceRegistryEntry(nextEntry)
	plan := buildWorkspaceRepoSyncFilePlan(path, content)
	status := setupGlobalStatus(input.DryRun, []SetupGlobalFilePlan{plan})
	report := WorkspaceRepoSyncReport{
		Command:         "workspace repos sync",
		ConfigRoot:      root,
		ConfigPath:      path,
		Owner:           owner,
		Workspace:       WorkspaceSummary{Name: nextEntry.Workspace.Name, Owner: nextEntry.Workspace.Owner},
		InboxRepo:       inboxRepo.FullName(),
		ExistingRepos:   existingStrings,
		DiscoveredRepos: repoRefsToStrings(discovered),
		TargetRepos:     targetStrings,
		AddedRepos:      added,
		RemovedRepos:    removed,
		SkippedRepos:    skipped,
		File:            plan,
		DryRun:          input.DryRun,
		Status:          status,
		Notes: []string{
			"owner repo discovery is opt-in; global workspaces do not automatically scan every GitHub repo",
			"workspace inbox repo is treated as backlog/intake and is not added as an execution repo",
		},
		NextStep:        "gira workspace status --config " + QuoteShellArg(path),
		GlobalWorkspace: nextEntry,
	}
	if input.DryRun {
		if plan.Action == "conflict" {
			report.NextStep = "inspect workspace registry file"
		}
		return report, nil
	}
	if plan.Action == "conflict" {
		report.Status = "blocked"
		return report, fmt.Errorf("workspace repo sync could not update %s", path)
	}
	if plan.Action != "skip" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return report, fmt.Errorf("create workspace registry directory %q: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return report, fmt.Errorf("write workspace registry %q: %w", path, err)
		}
		report.Applied = true
	}
	if plan.Action == "skip" {
		report.Status = "skipped"
	}
	return report, nil
}

func buildWorkspaceRepoSyncFilePlan(path string, content string) SetupGlobalFilePlan {
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
	plan.Action = "update"
	return plan
}

func workspaceRepoSyncName(configRoot string, name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed != "" {
		if !isSafeRegistryName(trimmed) {
			return "", fmt.Errorf("--workspace must not contain path separators")
		}
		return trimmed, nil
	}
	cfg, err := LoadGlobalConfig(configRoot)
	if err != nil {
		return "", fmt.Errorf("--workspace is required when global config default_workspace is unavailable: %w", err)
	}
	if strings.TrimSpace(cfg.DefaultWorkspace) == "" {
		return "", fmt.Errorf("--workspace is required when global config default_workspace is empty")
	}
	if !isSafeRegistryName(cfg.DefaultWorkspace) {
		return "", fmt.Errorf("global config default_workspace must not contain path separators")
	}
	return strings.TrimSpace(cfg.DefaultWorkspace), nil
}

func DiscoverOwnerRepos(owner string, limit int, includeArchived bool, runner CommandRunner) ([]RepoRef, error) {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return nil, fmt.Errorf("--owner is required")
	}
	if limit < 1 {
		limit = 100
	}
	repos, err := discoverOwnerRepos(owner, limit, false, runner)
	if err != nil {
		return nil, err
	}
	if includeArchived {
		archived, err := discoverOwnerRepos(owner, limit, true, runner)
		if err != nil {
			return nil, err
		}
		repos = append(repos, archived...)
	}
	return uniqueSortedRepoRefs(repos, limit), nil
}

func discoverOwnerRepos(owner string, limit int, archived bool, runner CommandRunner) ([]RepoRef, error) {
	args := []string{"repo", "list", owner, "--limit", strconv.Itoa(limit), "--json", "nameWithOwner,isArchived"}
	if archived {
		args = append(args, "--archived")
	} else {
		args = append(args, "--no-archived")
	}
	out, err := runner.Run("gh", args...)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		NameWithOwner string `json:"nameWithOwner"`
		IsArchived    bool   `json:"isArchived"`
	}
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("parse gh repo list JSON: %w", err)
	}
	repos := make([]RepoRef, 0, len(rows))
	for _, row := range rows {
		if archived && !row.IsArchived {
			continue
		}
		if !archived && row.IsArchived {
			continue
		}
		repo, err := ParseRepoRef(row.NameWithOwner)
		if err != nil {
			return nil, fmt.Errorf("parse repo %q: %w", row.NameWithOwner, err)
		}
		repos = append(repos, repo)
	}
	return repos, nil
}

func parseWorkspaceRepos(values []string) []RepoRef {
	repos := make([]RepoRef, 0, len(values))
	for _, value := range values {
		repo, err := ParseRepoRef(value)
		if err == nil {
			repos = append(repos, repo)
		}
	}
	return uniqueSortedRepoRefs(repos, 0)
}

func uniqueSortedRepoRefs(repos []RepoRef, limit int) []RepoRef {
	seen := map[string]RepoRef{}
	for _, repo := range repos {
		seen[strings.ToLower(repo.FullName())] = repo
	}
	out := make([]RepoRef, 0, len(seen))
	for _, repo := range seen {
		out = append(out, repo)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].FullName()) < strings.ToLower(out[j].FullName())
	})
	if limit > 0 && len(out) > limit {
		return out[:limit]
	}
	return out
}

func repoRefsToStrings(repos []RepoRef) []string {
	out := make([]string, 0, len(repos))
	for _, repo := range repos {
		out = append(out, repo.FullName())
	}
	return out
}

func diffRepoStrings(oldRepos []string, newRepos []string) ([]string, []string) {
	oldSet := map[string]string{}
	newSet := map[string]string{}
	for _, repo := range oldRepos {
		oldSet[strings.ToLower(repo)] = repo
	}
	for _, repo := range newRepos {
		newSet[strings.ToLower(repo)] = repo
	}
	var added, removed []string
	for key, repo := range newSet {
		if _, ok := oldSet[key]; !ok {
			added = append(added, repo)
		}
	}
	for key, repo := range oldSet {
		if _, ok := newSet[key]; !ok {
			removed = append(removed, repo)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

func renderGlobalWorkspaceRegistryEntry(entry GlobalWorkspaceRegistryEntry) string {
	content := renderWorkspaceGlobalConfig(entry.Workspace.Name, entry.Workspace.Owner, entry.Workspace.InboxRepo, entry.Workspace.Repos, entry.Workspace.Project)
	branchPolicy := renderBranchPolicyConfig(entry.BranchPolicy)
	if entry.Defaults.Agent == "" && entry.Defaults.Assignee == "" && len(entry.Defaults.AgentLabels) == 0 && branchPolicy == "" {
		return content
	}
	var b strings.Builder
	b.WriteString(content)
	if branchPolicy != "" {
		b.WriteString(branchPolicy)
	}
	if entry.Defaults.Agent == "" && entry.Defaults.Assignee == "" && len(entry.Defaults.AgentLabels) == 0 {
		return b.String()
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
	return b.String()
}

func FormatWorkspaceRepoSyncReport(report WorkspaceRepoSyncReport) string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "workspace repos sync: %s %s\n", report.Status, report.Workspace.Name)
	fmt.Fprintf(&b, "owner: %s\n", report.Owner)
	fmt.Fprintf(&b, "workspace: %s\n", report.ConfigPath)
	fmt.Fprintf(&b, "inbox: %s\n", report.InboxRepo)
	fmt.Fprintf(&b, "discovered: %d\n", len(report.DiscoveredRepos))
	fmt.Fprintf(&b, "execution repos: %d\n", len(report.TargetRepos))
	if len(report.AddedRepos) > 0 {
		fmt.Fprintf(&b, "added: %s\n", strings.Join(report.AddedRepos, ","))
	}
	if len(report.RemovedRepos) > 0 {
		fmt.Fprintf(&b, "removed: %s\n", strings.Join(report.RemovedRepos, ","))
	}
	if len(report.SkippedRepos) > 0 {
		fmt.Fprintf(&b, "skipped: %s\n", strings.Join(report.SkippedRepos, ","))
	}
	fmt.Fprintf(&b, "file: %s action=%s\n", report.File.Path, report.File.Action)
	for _, note := range report.Notes {
		fmt.Fprintf(&b, "note: %s\n", note)
	}
	if strings.TrimSpace(report.NextStep) != "" {
		fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	}
	return b.String()
}
