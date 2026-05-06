package gira

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type WorkspaceCapabilityReport struct {
	Command        string                        `json:"command"`
	Workspace      WorkspaceSummary              `json:"workspace"`
	Token          ProjectCapabilityTokenSummary `json:"token"`
	Repos          []PortfolioRepoCapability     `json:"repos"`
	BlockedActions []PortfolioCapabilityBlock    `json:"blocked_actions"`
	FetchedAt      string                        `json:"fetched_at"`
}

func BuildWorkspaceCapabilityReport(config WorkspaceConfigResolved, runner CommandRunner, now time.Time) (WorkspaceCapabilityReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	authStatus, err := fetchAuthStatus(runner)
	if err != nil {
		return WorkspaceCapabilityReport{}, err
	}
	host := authStatus.githubHost("github.com")
	if host == nil {
		return WorkspaceCapabilityReport{}, fmt.Errorf("no active github.com authentication found")
	}
	report := WorkspaceCapabilityReport{
		Command:   "workspace capability",
		Workspace: WorkspaceSummary{Name: config.Name, Owner: config.Owner},
		Token: ProjectCapabilityTokenSummary{
			Kind:     inferTokenKind(*host),
			Identity: host.Login,
		},
		Repos:          []PortfolioRepoCapability{},
		BlockedActions: []PortfolioCapabilityBlock{},
		FetchedAt:      now.UTC().Format(time.RFC3339),
	}
	inboxCapability, err := buildPortfolioRepoCapability(config.InboxRepo, "inbox", runner, *host)
	if err != nil {
		inboxCapability = blockedPortfolioRepoCapability(config.InboxRepo, "inbox")
	}
	addWorkspaceRepoCapability(&report, inboxCapability, []string{"issues:read", "issues:write"})
	for _, repo := range uniqueWorkspaceExecutionRepos(config) {
		capability, err := buildPortfolioRepoCapability(repo, "execution", runner, *host)
		if err != nil {
			capability = blockedPortfolioRepoCapability(repo, "execution")
		}
		addWorkspaceRepoCapability(&report, capability, []string{"issues:read", "issues:write"})
	}
	sort.SliceStable(report.BlockedActions, func(i, j int) bool {
		if report.BlockedActions[i].Repo == report.BlockedActions[j].Repo {
			return report.BlockedActions[i].CheckID < report.BlockedActions[j].CheckID
		}
		return report.BlockedActions[i].Repo < report.BlockedActions[j].Repo
	})
	return report, nil
}

func addWorkspaceRepoCapability(report *WorkspaceCapabilityReport, capability PortfolioRepoCapability, required []string) {
	report.Repos = append(report.Repos, capability)
	for _, name := range required {
		status := capability.Capabilities[name]
		if status == ProjectCapabilityAllowed {
			continue
		}
		reason := "permission denied"
		if status == ProjectCapabilityDeniedScope {
			reason = "token scope or repository permission is insufficient"
		} else if status == ProjectCapabilityUnsupported {
			reason = "issue write capability cannot be proven non-destructively with this token"
		}
		report.BlockedActions = append(report.BlockedActions, PortfolioCapabilityBlock{
			CheckID:  capability.Role + ":" + capability.Repo + ":" + name,
			Repo:     capability.Repo,
			Role:     capability.Role,
			Required: name,
			Reason:   reason,
		})
	}
}

func uniqueWorkspaceExecutionRepos(config WorkspaceConfigResolved) []RepoRef {
	seen := map[string]struct{}{}
	repos := []RepoRef{}
	for _, repo := range config.Repos {
		key := strings.ToLower(repo.FullName())
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		repos = append(repos, repo)
	}
	sort.Slice(repos, func(i, j int) bool { return repos[i].FullName() < repos[j].FullName() })
	return repos
}

func FormatWorkspaceCapabilityReport(report WorkspaceCapabilityReport) string {
	var lines []string
	lines = append(lines, "workspace capability")
	lines = append(lines, "workspace: "+report.Workspace.Name+" ("+report.Workspace.Owner+")")
	lines = append(lines, "token: "+report.Token.Identity+" ("+report.Token.Kind+")")
	lines = append(lines, "")
	lines = append(lines, "repos:")
	for _, repo := range report.Repos {
		lines = append(lines, "  "+repo.Repo+" ["+repo.Role+"] mode="+repo.Mode)
		keys := make([]string, 0, len(repo.Capabilities))
		for key := range repo.Capabilities {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			lines = append(lines, "    "+key+": "+string(repo.Capabilities[key]))
		}
	}
	if len(report.BlockedActions) > 0 {
		lines = append(lines, "")
		lines = append(lines, "blocked actions:")
		for _, block := range report.BlockedActions {
			lines = append(lines, "  - "+block.CheckID+" requires "+block.Required+" ("+block.Reason+")")
		}
		lines = append(lines, "next step: fix blocked repo permissions before workspace sync or route --apply")
	} else {
		lines = append(lines, "")
		lines = append(lines, "all workspace capability checks passed")
		lines = append(lines, "next step: gira workspace sync --dry-run --config .gira/config.yaml")
	}
	lines = append(lines, "fetched_at: "+report.FetchedAt)
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}
