package gira

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type PortfolioCapabilityReport struct {
	Command        string                        `json:"command"`
	PortfolioRepo  string                        `json:"portfolio_repo"`
	Token          ProjectCapabilityTokenSummary `json:"token"`
	Repos          []PortfolioRepoCapability     `json:"repos"`
	BlockedActions []PortfolioCapabilityBlock    `json:"blocked_actions"`
	FetchedAt      string                        `json:"fetched_at"`
}

type PortfolioRepoCapability struct {
	Repo         string                             `json:"repo"`
	Role         string                             `json:"role"`
	Mode         string                             `json:"mode"`
	Capabilities map[string]ProjectCapabilityStatus `json:"capabilities"`
}

type PortfolioCapabilityBlock struct {
	CheckID  string `json:"check_id"`
	Repo     string `json:"repo"`
	Role     string `json:"role"`
	Required string `json:"required"`
	Reason   string `json:"reason"`
}

func BuildPortfolioCapabilityReport(portfolioRepo RepoRef, repos []RepoRef, runner CommandRunner, now time.Time) (PortfolioCapabilityReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	authStatus, err := fetchAuthStatus(runner)
	if err != nil {
		return PortfolioCapabilityReport{}, err
	}
	host := authStatus.githubHost("github.com")
	if host == nil {
		return PortfolioCapabilityReport{}, fmt.Errorf("no active github.com authentication found")
	}

	report := PortfolioCapabilityReport{
		Command:       "portfolio capability",
		PortfolioRepo: portfolioRepo.FullName(),
		Token: ProjectCapabilityTokenSummary{
			Kind:     inferTokenKind(*host),
			Identity: host.Login,
		},
		Repos:          []PortfolioRepoCapability{},
		BlockedActions: []PortfolioCapabilityBlock{},
		FetchedAt:      now.UTC().Format(time.RFC3339),
	}

	portfolioCapability, err := buildPortfolioRepoCapability(portfolioRepo, "portfolio", runner)
	if err != nil {
		portfolioCapability = blockedPortfolioRepoCapability(portfolioRepo, "portfolio")
	}
	report.addRepoCapability(portfolioCapability, []string{"issues:read"})

	orderedRepos := append([]RepoRef(nil), repos...)
	sort.Slice(orderedRepos, func(i, j int) bool { return orderedRepos[i].FullName() < orderedRepos[j].FullName() })
	for _, repo := range orderedRepos {
		capability, err := buildPortfolioRepoCapability(repo, "execution", runner)
		if err != nil {
			capability = blockedPortfolioRepoCapability(repo, "execution")
		}
		report.addRepoCapability(capability, []string{"issues:read", "issues:write"})
	}

	sort.SliceStable(report.BlockedActions, func(i, j int) bool {
		if report.BlockedActions[i].Repo == report.BlockedActions[j].Repo {
			return report.BlockedActions[i].CheckID < report.BlockedActions[j].CheckID
		}
		return report.BlockedActions[i].Repo < report.BlockedActions[j].Repo
	})
	return report, nil
}

func buildPortfolioRepoCapability(repo RepoRef, role string, runner CommandRunner) (PortfolioRepoCapability, error) {
	payload, err := fetchRepoPermissions(repo, runner)
	if err != nil {
		return PortfolioRepoCapability{}, err
	}
	canRead := payload.Permissions.Pull || payload.Permissions.Push || payload.Permissions.Triage || payload.Permissions.Maintain || payload.Permissions.Admin
	canWrite := payload.Permissions.Push || payload.Permissions.Maintain || payload.Permissions.Admin
	mode := "inspect-only"
	if canWrite {
		mode = "write"
	} else if canRead {
		mode = "read-only"
	}
	return PortfolioRepoCapability{
		Repo: repo.FullName(),
		Role: role,
		Mode: mode,
		Capabilities: map[string]ProjectCapabilityStatus{
			"issues:read":  capabilityFromBool(canRead),
			"issues:write": capabilityFromBool(canWrite),
		},
	}, nil
}

func blockedPortfolioRepoCapability(repo RepoRef, role string) PortfolioRepoCapability {
	return PortfolioRepoCapability{
		Repo: repo.FullName(),
		Role: role,
		Mode: "inspect-only",
		Capabilities: map[string]ProjectCapabilityStatus{
			"issues:read":  ProjectCapabilityDeniedScope,
			"issues:write": ProjectCapabilityDeniedScope,
		},
	}
}

func (r *PortfolioCapabilityReport) addRepoCapability(capability PortfolioRepoCapability, required []string) {
	r.Repos = append(r.Repos, capability)
	for _, name := range required {
		status := capability.Capabilities[name]
		if status == ProjectCapabilityAllowed {
			continue
		}
		reason := "permission denied"
		if status == ProjectCapabilityDeniedScope {
			reason = "token scope or repository permission is insufficient"
		}
		r.BlockedActions = append(r.BlockedActions, PortfolioCapabilityBlock{
			CheckID:  capability.Role + ":" + capability.Repo + ":" + name,
			Repo:     capability.Repo,
			Role:     capability.Role,
			Required: name,
			Reason:   reason,
		})
	}
}

func FormatPortfolioCapabilityReport(report PortfolioCapabilityReport) string {
	var lines []string
	lines = append(lines, "portfolio capability")
	lines = append(lines, "portfolio repo: "+report.PortfolioRepo)
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
		lines = append(lines, "next step: fix blocked repo permissions before portfolio lower --apply")
	} else {
		lines = append(lines, "")
		lines = append(lines, "all portfolio capability checks passed")
		lines = append(lines, "next step: gira portfolio lower --dry-run --config .gira/config.yaml")
	}
	lines = append(lines, "fetched_at: "+report.FetchedAt)
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}
