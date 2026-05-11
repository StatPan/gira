package gira

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
)

var jiraAPIGet = fetchJiraAPIGetHTTP

type JiraProviderInitInput struct {
	Repo    RepoRef `json:"-"`
	APIBase string  `json:"api_base"`
	Project string  `json:"project"`
	Email   string  `json:"-"`
	Token   string  `json:"-"`
	DryRun  bool    `json:"dry_run"`
	Apply   bool    `json:"apply"`
}

type JiraProviderInitReport struct {
	Command        string                     `json:"command"`
	Repo           string                     `json:"repo"`
	APIBase        string                     `json:"api_base"`
	Project        JiraProviderProject        `json:"project"`
	DryRun         bool                       `json:"dry_run"`
	Apply          bool                       `json:"apply"`
	ReadOnly       bool                       `json:"read_only"`
	IssueTypes     []JiraProviderIssueType    `json:"issue_types"`
	Statuses       []JiraProviderStatus       `json:"statuses"`
	Priorities     []JiraProviderPriority     `json:"priorities"`
	Capabilities   []JiraProviderCapability   `json:"capabilities"`
	ConfigProposal JiraProviderConfigProposal `json:"config_proposal"`
	Warnings       []string                   `json:"warnings,omitempty"`
	NextSteps      []string                   `json:"next_steps"`
}

type JiraProviderProject struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	ProjectType string `json:"project_type,omitempty"`
	Simplified  bool   `json:"simplified,omitempty"`
}

type JiraProviderIssueType struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Subtask        bool   `json:"subtask"`
	HierarchyLevel int    `json:"hierarchy_level,omitempty"`
}

type JiraProviderStatus struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category,omitempty"`
}

type JiraProviderPriority struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type JiraProviderCapability struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type JiraProviderConfigProposal struct {
	Provider JiraProviderConfig `json:"provider"`
	GitHub   JiraProviderGitHub `json:"github"`
}

type JiraProviderConfig struct {
	Enabled       bool                    `json:"enabled"`
	Mode          string                  `json:"mode"`
	BaseURL       string                  `json:"base_url"`
	ProjectKey    string                  `json:"project_key"`
	SourceOfTruth JiraProviderSourceTruth `json:"source_of_truth"`
	StatusMap     []JiraProviderStatusMap `json:"status_map"`
}

type JiraProviderSourceTruth struct {
	Planning  string `json:"planning"`
	Status    string `json:"status"`
	Execution string `json:"execution"`
}

type JiraProviderStatusMap struct {
	GiraStatus   string   `json:"gira_status"`
	JiraStatuses []string `json:"jira_statuses"`
}

type JiraProviderGitHub struct {
	Repo         string `json:"repo"`
	MirrorIssue  bool   `json:"mirror_issue"`
	MirrorLabels bool   `json:"mirror_labels"`
}

func BuildJiraProviderInitReport(input JiraProviderInitInput) (JiraProviderInitReport, error) {
	if input.DryRun == input.Apply {
		return JiraProviderInitReport{}, fmt.Errorf("exactly one of --dry-run or --apply is required")
	}
	if input.Apply {
		return JiraProviderInitReport{}, fmt.Errorf("jira init --apply is not supported in this slice; use --dry-run to review the provider plan")
	}
	apiBase := strings.TrimRight(strings.TrimSpace(input.APIBase), "/")
	projectKey := strings.ToUpper(strings.TrimSpace(input.Project))
	if apiBase == "" || projectKey == "" {
		return JiraProviderInitReport{}, fmt.Errorf("--api-base and --project are required for jira init")
	}
	email := strings.TrimSpace(input.Email)
	if email == "" {
		email = strings.TrimSpace(os.Getenv("JIRA_EMAIL"))
	}
	token := strings.TrimSpace(input.Token)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("JIRA_API_TOKEN"))
	}
	if email == "" || token == "" {
		return JiraProviderInitReport{}, fmt.Errorf("JIRA_EMAIL and JIRA_API_TOKEN are required for jira init discovery")
	}

	project, err := fetchJiraProviderProject(apiBase, projectKey, email, token)
	if err != nil {
		return JiraProviderInitReport{}, err
	}
	issueTypes, err := fetchJiraProviderIssueTypes(apiBase, project.ID, email, token)
	if err != nil {
		return JiraProviderInitReport{}, err
	}
	statuses, err := fetchJiraProviderStatuses(apiBase, project.ID, email, token)
	if err != nil {
		return JiraProviderInitReport{}, err
	}
	priorities, err := fetchJiraProviderPriorities(apiBase, project.ID, email, token)
	if err != nil {
		return JiraProviderInitReport{}, err
	}

	report := JiraProviderInitReport{
		Command:      "jira init",
		Repo:         input.Repo.FullName(),
		APIBase:      apiBase,
		Project:      project,
		DryRun:       input.DryRun,
		Apply:        input.Apply,
		ReadOnly:     true,
		IssueTypes:   issueTypes,
		Statuses:     statuses,
		Priorities:   priorities,
		Capabilities: jiraProviderCapabilities(issueTypes, statuses, priorities),
		ConfigProposal: JiraProviderConfigProposal{
			Provider: JiraProviderConfig{
				Enabled:    true,
				Mode:       "primary",
				BaseURL:    apiBase,
				ProjectKey: project.Key,
				SourceOfTruth: JiraProviderSourceTruth{
					Planning:  "jira",
					Status:    "jira",
					Execution: "github",
				},
				StatusMap: inferJiraProviderStatusMap(statuses),
			},
			GitHub: JiraProviderGitHub{
				Repo:         input.Repo.FullName(),
				MirrorIssue:  true,
				MirrorLabels: true,
			},
		},
		NextSteps: []string{
			"Review the provider config proposal.",
			"Do not write credentials into repo-local config.",
			"Next implementation slice: jira init --apply writes reviewed non-secret provider config.",
		},
	}
	if len(issueTypes) == 0 {
		report.Warnings = append(report.Warnings, "no issue types discovered")
	}
	if len(statuses) == 0 {
		report.Warnings = append(report.Warnings, "no statuses discovered")
	}
	return report, nil
}

func fetchJiraProviderProject(apiBase string, projectKey string, email string, token string) (JiraProviderProject, error) {
	content, err := jiraAPIGet(apiBase, "/rest/api/3/project/"+url.PathEscape(projectKey), nil, email, token)
	if err != nil {
		return JiraProviderProject{}, fmt.Errorf("fetch Jira project: %w", err)
	}
	var raw struct {
		ID             string `json:"id"`
		Key            string `json:"key"`
		Name           string `json:"name"`
		ProjectTypeKey string `json:"projectTypeKey"`
		Simplified     bool   `json:"simplified"`
	}
	if err := json.Unmarshal(content, &raw); err != nil {
		return JiraProviderProject{}, fmt.Errorf("parse Jira project JSON: %w", err)
	}
	if strings.TrimSpace(raw.ID) == "" {
		return JiraProviderProject{}, fmt.Errorf("parse Jira project JSON: missing project id")
	}
	return JiraProviderProject{
		ID:          raw.ID,
		Key:         strings.ToUpper(strings.TrimSpace(raw.Key)),
		Name:        strings.TrimSpace(raw.Name),
		ProjectType: strings.TrimSpace(raw.ProjectTypeKey),
		Simplified:  raw.Simplified,
	}, nil
}

func fetchJiraProviderIssueTypes(apiBase string, projectID string, email string, token string) ([]JiraProviderIssueType, error) {
	content, err := jiraAPIGet(apiBase, "/rest/api/3/issuetype/project", map[string]string{"projectId": projectID}, email, token)
	if err != nil {
		return nil, fmt.Errorf("fetch Jira issue types: %w", err)
	}
	var raw []struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		Subtask        bool   `json:"subtask"`
		HierarchyLevel int    `json:"hierarchyLevel"`
	}
	if err := json.Unmarshal(content, &raw); err != nil {
		return nil, fmt.Errorf("parse Jira issue types JSON: %w", err)
	}
	items := make([]JiraProviderIssueType, 0, len(raw))
	for _, row := range raw {
		items = append(items, JiraProviderIssueType{ID: row.ID, Name: row.Name, Subtask: row.Subtask, HierarchyLevel: row.HierarchyLevel})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

func fetchJiraProviderStatuses(apiBase string, projectID string, email string, token string) ([]JiraProviderStatus, error) {
	content, err := jiraAPIGet(apiBase, "/rest/api/3/statuses/search", map[string]string{"projectId": projectID, "maxResults": "1000"}, email, token)
	if err != nil {
		return nil, fmt.Errorf("fetch Jira statuses: %w", err)
	}
	var wrapped struct {
		Values []jiraProviderStatusJSON `json:"values"`
	}
	if err := json.Unmarshal(content, &wrapped); err == nil && wrapped.Values != nil {
		return normalizeJiraProviderStatuses(wrapped.Values), nil
	}
	var direct []jiraProviderStatusJSON
	if err := json.Unmarshal(content, &direct); err != nil {
		return nil, fmt.Errorf("parse Jira statuses JSON: %w", err)
	}
	return normalizeJiraProviderStatuses(direct), nil
}

type jiraProviderStatusJSON struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	StatusCategory *struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"statusCategory"`
}

func normalizeJiraProviderStatuses(raw []jiraProviderStatusJSON) []JiraProviderStatus {
	items := make([]JiraProviderStatus, 0, len(raw))
	for _, row := range raw {
		category := ""
		if row.StatusCategory != nil {
			category = strings.TrimSpace(row.StatusCategory.Name)
			if category == "" {
				category = strings.TrimSpace(row.StatusCategory.Key)
			}
		}
		items = append(items, JiraProviderStatus{ID: row.ID, Name: strings.TrimSpace(row.Name), Category: category})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

func fetchJiraProviderPriorities(apiBase string, projectID string, email string, token string) ([]JiraProviderPriority, error) {
	content, err := jiraAPIGet(apiBase, "/rest/api/3/priority/search", map[string]string{"projectId": projectID, "maxResults": "1000"}, email, token)
	if err != nil {
		return nil, fmt.Errorf("fetch Jira priorities: %w", err)
	}
	var wrapped struct {
		Values []JiraProviderPriority `json:"values"`
	}
	if err := json.Unmarshal(content, &wrapped); err == nil && wrapped.Values != nil {
		return normalizeJiraProviderPriorities(wrapped.Values), nil
	}
	var direct []JiraProviderPriority
	if err := json.Unmarshal(content, &direct); err != nil {
		return nil, fmt.Errorf("parse Jira priorities JSON: %w", err)
	}
	return normalizeJiraProviderPriorities(direct), nil
}

func normalizeJiraProviderPriorities(items []JiraProviderPriority) []JiraProviderPriority {
	out := make([]JiraProviderPriority, 0, len(items))
	for _, item := range items {
		out = append(out, JiraProviderPriority{ID: strings.TrimSpace(item.ID), Name: strings.TrimSpace(item.Name)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func fetchJiraAPIGetHTTP(apiBase string, path string, query map[string]string, email string, token string) ([]byte, error) {
	endpoint := strings.TrimRight(apiBase, "/") + path
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	values := req.URL.Query()
	for key, value := range query {
		values.Set(key, value)
	}
	req.URL.RawQuery = values.Encode()
	req.SetBasicAuth(email, token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Jira API GET %s failed: %s", path, resp.Status)
	}
	return content, nil
}

func jiraProviderCapabilities(issueTypes []JiraProviderIssueType, statuses []JiraProviderStatus, priorities []JiraProviderPriority) []JiraProviderCapability {
	return []JiraProviderCapability{
		{Name: "project_metadata", Status: "supported", Detail: "read project identity from Jira"},
		{Name: "issue_types", Status: capabilityStatus(len(issueTypes) > 0), Detail: "read issue types for provider mapping"},
		{Name: "statuses", Status: capabilityStatus(len(statuses) > 0), Detail: "read statuses for status_map proposal"},
		{Name: "priorities", Status: capabilityStatus(len(priorities) > 0), Detail: "read priority values for mirror label planning"},
		{Name: "transitions", Status: "planned", Detail: "issue-specific transition discovery is deferred to the transition planner slice"},
		{Name: "workflow_mutation", Status: "manual_admin_required", Detail: "workflow status/transition mutation is intentionally out of scope"},
	}
}

func capabilityStatus(ok bool) string {
	if ok {
		return "supported"
	}
	return "not_discovered"
}

func inferJiraProviderStatusMap(statuses []JiraProviderStatus) []JiraProviderStatusMap {
	groups := []JiraProviderStatusMap{
		{GiraStatus: "backlog"},
		{GiraStatus: "ready"},
		{GiraStatus: "in_progress"},
		{GiraStatus: "review"},
		{GiraStatus: "done"},
	}
	for _, status := range statuses {
		name := strings.TrimSpace(status.Name)
		lower := strings.ToLower(name)
		switch {
		case strings.Contains(lower, "backlog"):
			groups[0].JiraStatuses = append(groups[0].JiraStatuses, name)
		case strings.Contains(lower, "to do") || strings.Contains(lower, "selected"):
			groups[1].JiraStatuses = append(groups[1].JiraStatuses, name)
		case strings.Contains(lower, "progress"):
			groups[2].JiraStatuses = append(groups[2].JiraStatuses, name)
		case strings.Contains(lower, "review") || strings.Contains(lower, "qa"):
			groups[3].JiraStatuses = append(groups[3].JiraStatuses, name)
		case strings.Contains(lower, "done") || strings.Contains(lower, "closed") || strings.Contains(lower, "resolved"):
			groups[4].JiraStatuses = append(groups[4].JiraStatuses, name)
		}
	}
	return groups
}

func FormatJiraProviderInitReport(report JiraProviderInitReport) string {
	var b strings.Builder
	b.WriteString("jira init: dry-run provider discovery\n")
	fmt.Fprintf(&b, "repo: %s\n", report.Repo)
	fmt.Fprintf(&b, "api_base: %s\n", report.APIBase)
	fmt.Fprintf(&b, "project: %s %s (%s)\n", report.Project.Key, report.Project.Name, report.Project.ID)
	fmt.Fprintf(&b, "read_only: %t\n", report.ReadOnly)
	fmt.Fprintf(&b, "issue_types: %d\n", len(report.IssueTypes))
	for _, item := range report.IssueTypes {
		fmt.Fprintf(&b, "  - %s (%s)\n", item.Name, item.ID)
	}
	fmt.Fprintf(&b, "statuses: %d\n", len(report.Statuses))
	for _, status := range report.Statuses {
		if status.Category != "" {
			fmt.Fprintf(&b, "  - %s (%s)\n", status.Name, status.Category)
		} else {
			fmt.Fprintf(&b, "  - %s\n", status.Name)
		}
	}
	fmt.Fprintf(&b, "priorities: %d\n", len(report.Priorities))
	for _, priority := range report.Priorities {
		fmt.Fprintf(&b, "  - %s\n", priority.Name)
	}
	b.WriteString("capabilities:\n")
	for _, capability := range report.Capabilities {
		fmt.Fprintf(&b, "  - %s: %s (%s)\n", capability.Name, capability.Status, capability.Detail)
	}
	b.WriteString("config proposal:\n")
	fmt.Fprintf(&b, "  providers.jira.enabled: %t\n", report.ConfigProposal.Provider.Enabled)
	fmt.Fprintf(&b, "  providers.jira.mode: %s\n", report.ConfigProposal.Provider.Mode)
	fmt.Fprintf(&b, "  providers.jira.project_key: %s\n", report.ConfigProposal.Provider.ProjectKey)
	fmt.Fprintf(&b, "  github.repo: %s\n", report.ConfigProposal.GitHub.Repo)
	b.WriteString("status_map:\n")
	for _, mapping := range report.ConfigProposal.Provider.StatusMap {
		fmt.Fprintf(&b, "  - %s: [%s]\n", mapping.GiraStatus, strings.Join(mapping.JiraStatuses, ", "))
	}
	if len(report.Warnings) > 0 {
		b.WriteString("warnings:\n")
		for _, warning := range report.Warnings {
			fmt.Fprintf(&b, "  - %s\n", warning)
		}
	}
	b.WriteString("next steps:\n")
	for i, step := range report.NextSteps {
		fmt.Fprintf(&b, "%d. %s\n", i+1, step)
	}
	return b.String()
}
