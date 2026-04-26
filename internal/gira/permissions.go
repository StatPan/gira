package gira

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	ProjectCapabilityAllowed     = "allowed"
	ProjectCapabilityDeniedScope = "denied:token_scope"
	ProjectCapabilityDeniedAuth  = "denied:unauthenticated"
	ProjectCapabilityUnsupported = "unknown:unsupported"
)

type ProjectCapabilityStatus string

type ProjectCapabilityReport struct {
	Repo           string                             `json:"repo"`
	Command        string                             `json:"command"`
	Token          ProjectCapabilityTokenSummary      `json:"token"`
	Mode           string                             `json:"mode"`
	Capabilities   map[string]ProjectCapabilityStatus `json:"capabilities"`
	BlockedActions []ProjectCapabilityBlock           `json:"blocked_actions"`
	FetchedAt      string                             `json:"fetched_at"`
}

type ProjectCapabilityTokenSummary struct {
	Kind     string `json:"kind"`
	Identity string `json:"identity"`
}

type ProjectCapabilityBlock struct {
	Action   string `json:"action"`
	Required string `json:"required"`
	Reason   string `json:"reason"`
}

type ghAuthHost struct {
	State       string `json:"state"`
	Active      bool   `json:"active"`
	Host        string `json:"host"`
	Login       string `json:"login"`
	TokenSource string `json:"tokenSource"`
	Scopes      string `json:"scopes"`
}

type ghAuthStatus struct {
	Hosts map[string][]ghAuthHost `json:"hosts"`
}

type githubRepoPermissions struct {
	Admin    bool `json:"admin"`
	Maintain bool `json:"maintain"`
	Pull     bool `json:"pull"`
	Push     bool `json:"push"`
	Triage   bool `json:"triage"`
}

type ghRepoPayload struct {
	Permissions githubRepoPermissions `json:"permissions"`
}

type ghProjectQuery struct {
	Data struct {
		Repository struct {
			ViewerPermission    string `json:"viewerPermission"`
			ViewerCanAdminister bool   `json:"viewerCanAdminister"`
			ProjectsV2          struct {
				Nodes []struct {
					Title string `json:"title"`
				} `json:"nodes"`
			} `json:"projectsV2"`
		} `json:"repository"`
	} `json:"data"`
}

func BuildProjectCapabilityReport(repo RepoRef, runner CommandRunner) (ProjectCapabilityReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}

	authStatus, err := fetchAuthStatus(runner)
	if err != nil {
		return ProjectCapabilityReport{}, err
	}
	host := authStatus.githubHost("github.com")
	if host == nil {
		return ProjectCapabilityReport{}, fmt.Errorf("no active github.com authentication found")
	}

	repoPayload, err := fetchRepoPermissions(repo, runner)
	if err != nil {
		return ProjectCapabilityReport{}, err
	}

	projectRead, projectWrite := probeProjectCapability(repo, repoPayload.Permissions, runner)

	report := ProjectCapabilityReport{
		Repo: repo.FullName(),
		Token: ProjectCapabilityTokenSummary{
			Kind:     inferTokenKind(*host),
			Identity: host.Login,
		},
		Capabilities:   map[string]ProjectCapabilityStatus{},
		BlockedActions: []ProjectCapabilityBlock{},
		FetchedAt:      time.Now().Format(time.RFC3339),
	}

	canRead := repoPayload.Permissions.Pull || repoPayload.Permissions.Push || repoPayload.Permissions.Triage || repoPayload.Permissions.Maintain || repoPayload.Permissions.Admin
	canWrite := repoPayload.Permissions.Push || repoPayload.Permissions.Maintain || repoPayload.Permissions.Admin
	if canWrite {
		report.Mode = "write"
	} else if canRead {
		report.Mode = "read-only"
	} else {
		report.Mode = "inspect-only"
	}

	setCapability := func(name string, state ProjectCapabilityStatus) {
		report.Capabilities[name] = state
	}

	setCapability("issues:read", capabilityFromBool(canRead))
	setCapability("issues:write", capabilityFromBool(canWrite))
	setCapability("pullrequests:read", capabilityFromBool(canRead))
	setCapability("pullrequests:write", capabilityFromBool(canWrite))
	setCapability("projectsv2:read", capabilityFromBool(projectRead))
	setCapability("projectsv2:write", capabilityFromBool(projectWrite))
	setCapability("repo:settings:write", capabilityFromBool(repoPayload.Permissions.Admin))
	setCapability("repo:milestone:close", capabilityFromBool(canWrite))

	for action, st := range report.Capabilities {
		if st != ProjectCapabilityAllowed {
			reason := "permission denied"
			if st == ProjectCapabilityDeniedScope {
				reason = "token scope or repository permission is insufficient"
			}
			report.BlockedActions = append(report.BlockedActions, ProjectCapabilityBlock{
				Action:   action,
				Required: action,
				Reason:   reason,
			})
		}
	}
	sort.SliceStable(report.BlockedActions, func(i, j int) bool {
		return report.BlockedActions[i].Action < report.BlockedActions[j].Action
	})

	return report, nil
}

func capabilityFromBool(allowed bool) ProjectCapabilityStatus {
	if allowed {
		return ProjectCapabilityAllowed
	}
	return ProjectCapabilityDeniedScope
}

func (s ghAuthStatus) githubHost(host string) *ghAuthHost {
	hosts, ok := s.Hosts[host]
	if !ok {
		return nil
	}
	for i := range hosts {
		if hosts[i].Active && hosts[i].State == "success" {
			return &hosts[i]
		}
	}
	for i := range hosts {
		if hosts[i].State == "success" {
			return &hosts[i]
		}
	}
	if len(hosts) > 0 {
		return &hosts[0]
	}
	return nil
}

func inferTokenKind(host ghAuthHost) string {
	if host.TokenSource != "" {
		if strings.HasPrefix(host.TokenSource, "env://") {
			return "actions_secret"
		}
		if host.Scopes != "" {
			return "pat"
		}
	}
	return "github_app"
}

func fetchAuthStatus(runner CommandRunner) (ghAuthStatus, error) {
	var status ghAuthStatus
	output, err := runner.Run("gh", "auth", "status", "--json", "hosts")
	if err != nil {
		return ghAuthStatus{}, err
	}
	if err := json.Unmarshal(output, &status); err != nil {
		return ghAuthStatus{}, fmt.Errorf("parse gh auth status: %w", err)
	}
	return status, nil
}

func fetchRepoPermissions(repo RepoRef, runner CommandRunner) (ghRepoPayload, error) {
	var payload ghRepoPayload
	args := []string{"api", "repos/" + repo.FullName()}
	output, err := runner.Run("gh", args...)
	if err != nil {
		return ghRepoPayload{}, err
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return ghRepoPayload{}, fmt.Errorf("parse gh repo payload: %w", err)
	}
	return payload, nil
}

func probeProjectCapability(repo RepoRef, perms githubRepoPermissions, runner CommandRunner) (read bool, write bool) {
	var probe ghProjectQuery
	query := `query($o: String!, $n: String!){ repository(owner: $o, name: $n){ viewerPermission viewerCanAdminister projectsV2(first:1){ nodes{ title } } } }`
	output, err := runner.Run("gh", "api", "graphql", "-f", "query="+query, "-f", "o="+repo.Owner, "-f", "n="+repo.Name)
	if err != nil {
		read = false
	} else if err := json.Unmarshal(output, &probe); err == nil {
		if probe.Data.Repository.ProjectsV2.Nodes != nil {
			read = true
		}
	}
	write = read && (perms.Admin || perms.Maintain)
	return
}

func FormatProjectCapabilitySummary(report ProjectCapabilityReport) string {
	var lines []string
	lines = append(lines, "project capability\n")
	lines = append(lines, "repo: "+report.Repo)
	lines = append(lines, "command: "+report.Command)
	lines = append(lines, "token: "+report.Token.Identity+" ("+report.Token.Kind+")")
	lines = append(lines, "mode: "+report.Mode)
	lines = append(lines, "")
	lines = append(lines, "capabilities:")

	keys := make([]string, 0, len(report.Capabilities))
	for k := range report.Capabilities {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		lines = append(lines, "  "+key+": "+string(report.Capabilities[key]))
	}

	if len(report.BlockedActions) > 0 {
		lines = append(lines, "")
		lines = append(lines, "blocked actions:")
		for _, block := range report.BlockedActions {
			lines = append(lines, "  - "+block.Action+" requires "+block.Required+" ("+block.Reason+")")
		}
	} else {
		lines = append(lines, "")
		lines = append(lines, "all capability checks passed")
	}

	lines = append(lines, "")
	lines = append(lines, "fetched_at: "+report.FetchedAt+"\n")
	return strings.Join(lines, "\n")
}
