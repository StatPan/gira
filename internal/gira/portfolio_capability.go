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

	portfolioCapability, err := buildPortfolioRepoCapability(portfolioRepo, "portfolio", runner, *host)
	if err != nil {
		portfolioCapability = blockedPortfolioRepoCapability(portfolioRepo, "portfolio")
	}
	report.addRepoCapability(portfolioCapability, []string{"issues:read"})

	orderedRepos := append([]RepoRef(nil), repos...)
	sort.Slice(orderedRepos, func(i, j int) bool { return orderedRepos[i].FullName() < orderedRepos[j].FullName() })
	for _, repo := range orderedRepos {
		capability, err := buildPortfolioRepoCapability(repo, "execution", runner, *host)
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

func buildPortfolioRepoCapability(repo RepoRef, role string, runner CommandRunner, host ghAuthHost) (PortfolioRepoCapability, error) {
	payload, err := fetchRepoPermissions(repo, runner)
	if err != nil {
		return PortfolioRepoCapability{}, err
	}
	canRead := probeIssueRead(repo, runner)
	canIssueRoleWrite := payload.Permissions.Triage || payload.Permissions.Push || payload.Permissions.Maintain || payload.Permissions.Admin
	writeStatus := capabilityFromIssueWriteEvidence(canRead, canIssueRoleWrite, host)
	mode := "inspect-only"
	if writeStatus == ProjectCapabilityAllowed {
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
			"issues:write": writeStatus,
		},
	}, nil
}

func probeIssueRead(repo RepoRef, runner CommandRunner) bool {
	_, err := runner.Run("gh", "api", "repos/"+repo.FullName()+"/issues", "-X", "GET", "-f", "state=all", "-f", "per_page=1")
	return err == nil
}

func capabilityFromIssueWriteEvidence(canRead bool, canIssueRoleWrite bool, host ghAuthHost) ProjectCapabilityStatus {
	if !canRead || !canIssueRoleWrite {
		return ProjectCapabilityDeniedScope
	}
	if tokenScopesAllowIssueWrite(host.Scopes) {
		return ProjectCapabilityAllowed
	}
	return ProjectCapabilityUnsupported
}

func tokenScopesAllowIssueWrite(scopes string) bool {
	for _, raw := range strings.FieldsFunc(scopes, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' }) {
		scope := strings.TrimSpace(raw)
		if scope == "repo" || scope == "public_repo" || scope == "issues" || scope == "issues:write" {
			return true
		}
	}
	return false
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
		} else if status == ProjectCapabilityUnsupported {
			reason = "issue write capability cannot be proven non-destructively with this token"
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

func PortfolioCapabilityBlocksForActions(report PortfolioCapabilityReport, actions []PortfolioPlanAction) []PortfolioCapabilityBlock {
	needed := map[string]map[string]struct{}{}
	for _, action := range actions {
		if action.Repo == "" {
			continue
		}
		required := "issues:read"
		if action.Action == "execution_issue:create" {
			required = "issues:write"
		}
		if _, ok := needed[action.Repo]; !ok {
			needed[action.Repo] = map[string]struct{}{}
		}
		needed[action.Repo][required] = struct{}{}
	}
	blocks := make([]PortfolioCapabilityBlock, 0)
	for _, block := range report.BlockedActions {
		if byCapability, ok := needed[block.Repo]; ok {
			if _, ok := byCapability[block.Required]; ok {
				blocks = append(blocks, block)
			}
		}
	}
	return blocks
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
		lines = append(lines, "next step: fix blocked repo permissions before implementing portfolio lower --apply")
	} else {
		lines = append(lines, "")
		lines = append(lines, "all portfolio capability checks passed")
		lines = append(lines, "next step: gira portfolio plan --dry-run --config .gira/config.yaml")
	}
	lines = append(lines, "fetched_at: "+report.FetchedAt)
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}
