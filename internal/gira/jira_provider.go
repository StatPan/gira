package gira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var jiraAPIGet = fetchJiraAPIGetHTTP
var jiraAPIPost = fetchJiraAPIPostHTTP

type JiraProviderInitInput struct {
	Repo       RepoRef `json:"-"`
	APIBase    string  `json:"api_base"`
	Project    string  `json:"project"`
	Email      string  `json:"-"`
	Token      string  `json:"-"`
	ConfigRoot string  `json:"config_root,omitempty"`
	Overwrite  bool    `json:"overwrite"`
	DryRun     bool    `json:"dry_run"`
	Apply      bool    `json:"apply"`
}

type JiraProviderInitReport struct {
	Command        string                     `json:"command"`
	Repo           string                     `json:"repo"`
	APIBase        string                     `json:"api_base"`
	ConfigRoot     string                     `json:"config_root"`
	Project        JiraProviderProject        `json:"project"`
	DryRun         bool                       `json:"dry_run"`
	Apply          bool                       `json:"apply"`
	Applied        bool                       `json:"applied"`
	Status         string                     `json:"status"`
	File           SetupGlobalFilePlan        `json:"file"`
	ReadOnly       bool                       `json:"read_only"`
	IssueTypes     []JiraProviderIssueType    `json:"issue_types"`
	Statuses       []JiraProviderStatus       `json:"statuses"`
	Priorities     []JiraProviderPriority     `json:"priorities"`
	Capabilities   []JiraProviderCapability   `json:"capabilities"`
	ConfigProposal JiraProviderConfigProposal `json:"config_proposal"`
	ConfigPayload  string                     `json:"config_payload,omitempty"`
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
	Enabled       bool                    `yaml:"enabled" toml:"enabled" json:"enabled"`
	Mode          string                  `yaml:"mode" toml:"mode" json:"mode"`
	BaseURL       string                  `yaml:"base_url" toml:"base_url" json:"base_url"`
	ProjectKey    string                  `yaml:"project_key" toml:"project_key" json:"project_key"`
	SourceOfTruth JiraProviderSourceTruth `yaml:"source_of_truth" toml:"source_of_truth" json:"source_of_truth"`
	StatusMap     []JiraProviderStatusMap `yaml:"status_map" toml:"status_map" json:"status_map"`
}

type JiraProviderSourceTruth struct {
	Planning  string `yaml:"planning" toml:"planning" json:"planning"`
	Status    string `yaml:"status" toml:"status" json:"status"`
	Execution string `yaml:"execution" toml:"execution" json:"execution"`
}

type JiraProviderStatusMap struct {
	GiraStatus   string   `yaml:"gira_status" toml:"gira_status" json:"gira_status"`
	JiraStatuses []string `yaml:"jira_statuses" toml:"jira_statuses" json:"jira_statuses"`
}

type JiraProviderGitHub struct {
	Repo         string `json:"repo"`
	MirrorIssue  bool   `json:"mirror_issue"`
	MirrorLabels bool   `json:"mirror_labels"`
}

func validateJiraProviderConfig(source string, field string, cfg JiraProviderConfig) error {
	if !cfg.Enabled {
		return fmt.Errorf("invalid Jira provider config %q: %s.enabled must be true for configured providers", source, field)
	}
	if strings.TrimSpace(cfg.Mode) != "primary" {
		return fmt.Errorf("invalid Jira provider config %q: %s.mode must be primary", source, field)
	}
	if _, err := normalizeJiraAPIBase(cfg.BaseURL); err != nil {
		return fmt.Errorf("invalid Jira provider config %q: %s.base_url %v", source, field, err)
	}
	if strings.TrimSpace(cfg.ProjectKey) == "" {
		return fmt.Errorf("invalid Jira provider config %q: %s.project_key is required", source, field)
	}
	if err := validateJiraProviderSourceOfTruth(source, field, cfg.SourceOfTruth); err != nil {
		return err
	}
	seen := map[string]struct{}{}
	for i, mapping := range cfg.StatusMap {
		status := strings.TrimSpace(mapping.GiraStatus)
		if status == "" {
			return fmt.Errorf("invalid Jira provider config %q: %s.status_map[%d].gira_status is required", source, field, i)
		}
		key := strings.ToLower(status)
		if _, ok := seen[key]; ok {
			return fmt.Errorf("invalid Jira provider config %q: %s.status_map duplicates %q", source, field, status)
		}
		seen[key] = struct{}{}
		for j, jiraStatus := range mapping.JiraStatuses {
			if strings.TrimSpace(jiraStatus) == "" {
				return fmt.Errorf("invalid Jira provider config %q: %s.status_map[%d].jira_statuses[%d] is required", source, field, i, j)
			}
		}
	}
	return nil
}

func normalizeJiraAPIBase(value string) (string, error) {
	apiBase := strings.TrimRight(strings.TrimSpace(value), "/")
	parsed, err := url.Parse(apiBase)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Hostname() == "" {
		return "", fmt.Errorf("--api-base must be an absolute HTTPS URL")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("--api-base must not contain credentials")
	}
	if parsed.Scheme != "https" {
		return "", fmt.Errorf("--api-base must use https")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("--api-base must not contain query strings or fragments")
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return "", fmt.Errorf("--api-base must not target localhost")
	}
	if ip := net.ParseIP(host); ip != nil && !isPublicJiraAPIBaseIP(ip) {
		return "", fmt.Errorf("--api-base must not target local or private network addresses")
	}
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), nil
}

func isPublicJiraAPIBaseIP(ip net.IP) bool {
	return !(ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast())
}

func validateJiraProviderSourceOfTruth(source string, field string, truth JiraProviderSourceTruth) error {
	allowed := map[string]map[string]struct{}{
		"planning":  {"jira": {}},
		"status":    {"jira": {}},
		"execution": {"github": {}},
	}
	values := map[string]string{
		"planning":  truth.Planning,
		"status":    truth.Status,
		"execution": truth.Execution,
	}
	for name, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if _, ok := allowed[name][normalized]; !ok {
			return fmt.Errorf("invalid Jira provider config %q: %s.source_of_truth.%s must be %s", source, field, name, firstAllowedValue(allowed[name]))
		}
	}
	return nil
}

func firstAllowedValue(values map[string]struct{}) string {
	for value := range values {
		return value
	}
	return ""
}

func BuildJiraProviderInitReport(input JiraProviderInitInput) (JiraProviderInitReport, error) {
	if input.DryRun == input.Apply {
		return JiraProviderInitReport{}, fmt.Errorf("exactly one of --dry-run or --apply is required")
	}
	projectKey := strings.ToUpper(strings.TrimSpace(input.Project))
	if strings.TrimSpace(input.APIBase) == "" || projectKey == "" {
		return JiraProviderInitReport{}, fmt.Errorf("--api-base and --project are required for jira init")
	}
	apiBase, err := normalizeJiraAPIBase(input.APIBase)
	if err != nil {
		return JiraProviderInitReport{}, err
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
	root, err := globalConfigRoot(input.ConfigRoot)
	if err != nil {
		return JiraProviderInitReport{}, err
	}
	file, err := GlobalRepoRegistryPath(root, input.Repo)
	if err != nil {
		return JiraProviderInitReport{}, err
	}
	providerConfig := JiraProviderConfig{
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
	}
	if err := validateJiraProviderConfig(file, "providers.jira", providerConfig); err != nil {
		return JiraProviderInitReport{}, err
	}
	filePlan, payload, entry, err := buildJiraProviderConfigFilePlan(file, input.Repo, providerConfig, input.Overwrite)
	if err != nil {
		return JiraProviderInitReport{}, err
	}

	report := JiraProviderInitReport{
		Command:      "jira init",
		Repo:         input.Repo.FullName(),
		APIBase:      apiBase,
		ConfigRoot:   root,
		Project:      project,
		DryRun:       input.DryRun,
		Apply:        input.Apply,
		Status:       setupGlobalStatus(input.DryRun, []SetupGlobalFilePlan{filePlan}),
		File:         filePlan,
		ReadOnly:     true,
		IssueTypes:   issueTypes,
		Statuses:     statuses,
		Priorities:   priorities,
		Capabilities: jiraProviderCapabilities(issueTypes, statuses, priorities),
		ConfigProposal: JiraProviderConfigProposal{
			Provider: providerConfig,
			GitHub: JiraProviderGitHub{
				Repo:         input.Repo.FullName(),
				MirrorIssue:  true,
				MirrorLabels: true,
			},
		},
		ConfigPayload: payload,
		NextSteps: []string{
			"Review the provider config proposal and generated repo registry payload.",
			"Run jira init --apply only after confirming no credentials are present.",
			fmt.Sprintf("Inspect the registry with gira config repo --repo %s --config-root %s.", input.Repo.FullName(), root),
		},
	}
	if entry.Providers != nil && entry.Providers.Jira != nil {
		report.ConfigProposal.Provider = *entry.Providers.Jira
	}
	if len(issueTypes) == 0 {
		report.Warnings = append(report.Warnings, "no issue types discovered")
	}
	if len(statuses) == 0 {
		report.Warnings = append(report.Warnings, "no statuses discovered")
	}
	if filePlan.Action == "conflict" && input.Apply {
		return report, fmt.Errorf("%s already has a different providers.jira config; pass --overwrite to replace only providers.jira", file)
	}
	if input.Apply && filePlan.Action != "skip" {
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return report, fmt.Errorf("create repo registry directory %q: %w", filepath.Dir(file), err)
		}
		if err := os.WriteFile(file, []byte(filePlan.Content), 0o644); err != nil {
			return report, fmt.Errorf("write repo registry %q: %w", file, err)
		}
		report.Applied = true
		report.Status = "applied"
	}
	return report, nil
}

func buildJiraProviderConfigFilePlan(file string, repo RepoRef, provider JiraProviderConfig, overwrite bool) (SetupGlobalFilePlan, string, GlobalRepoRegistryEntry, error) {
	entry := GlobalRepoRegistryEntry{Repo: repo.FullName()}
	existing, err := os.ReadFile(file)
	exists := false
	if err == nil {
		exists = true
		if err := readGlobalYAML(file, &entry); err != nil {
			return SetupGlobalFilePlan{Path: file, Exists: true, Action: "conflict"}, "", entry, err
		}
		if strings.TrimSpace(entry.Repo) == "" {
			entry.Repo = repo.FullName()
		}
		if err := validateJiraProviderExistingRepoEntry(entry, repo, file, overwrite); err != nil {
			return SetupGlobalFilePlan{Path: file, Exists: true, Action: "conflict"}, "", entry, err
		}
	} else if !os.IsNotExist(err) {
		return SetupGlobalFilePlan{Path: file, Exists: true, Action: "conflict"}, "", entry, fmt.Errorf("read repo registry %q: %w", file, err)
	}

	proposed := entry
	if entry.Providers != nil {
		proposed.Providers = &GlobalProvidersConfig{}
		if entry.Providers.Jira != nil {
			proposed.Providers.Jira = cloneJiraProviderConfig(*entry.Providers.Jira)
		}
	}
	if proposed.Providers == nil {
		proposed.Providers = &GlobalProvidersConfig{}
	}
	proposed.Providers.Jira = cloneJiraProviderConfig(provider)
	if err := ValidateGlobalRepoRegistryEntry(proposed, repo, file); err != nil {
		return SetupGlobalFilePlan{Path: file, Exists: exists, Action: "conflict"}, "", proposed, err
	}
	content, err := marshalRepoRegistryEntry(proposed)
	if err != nil {
		return SetupGlobalFilePlan{Path: file, Exists: exists, Action: "conflict"}, "", proposed, err
	}
	payload, err := marshalJiraProviderPayload(provider)
	if err != nil {
		return SetupGlobalFilePlan{Path: file, Exists: exists, Action: "conflict"}, "", proposed, err
	}

	plan := SetupGlobalFilePlan{Path: file, Exists: exists, Action: "create", Content: string(content)}
	if !exists {
		return plan, payload, proposed, nil
	}
	if bytesEqualTrimmed(existing, content) {
		plan.Action = "skip"
		return plan, payload, proposed, nil
	}
	if entry.Providers != nil && entry.Providers.Jira != nil && !reflect.DeepEqual(entry.Providers.Jira, &provider) && !overwrite {
		plan.Action = "conflict"
		return plan, payload, proposed, nil
	}
	if entry.Providers == nil || entry.Providers.Jira == nil {
		plan.Action = "update"
		return plan, payload, proposed, nil
	}
	plan.Action = "overwrite"
	return plan, payload, proposed, nil
}

func validateJiraProviderExistingRepoEntry(entry GlobalRepoRegistryEntry, repo RepoRef, file string, overwrite bool) error {
	if !overwrite {
		return ValidateGlobalRepoRegistryEntry(entry, repo, file)
	}
	preserved := entry
	preserved.Providers = nil
	return ValidateGlobalRepoRegistryEntry(preserved, repo, file)
}

func cloneJiraProviderConfig(cfg JiraProviderConfig) *JiraProviderConfig {
	out := cfg
	if cfg.StatusMap != nil {
		out.StatusMap = append([]JiraProviderStatusMap(nil), cfg.StatusMap...)
		for i := range out.StatusMap {
			out.StatusMap[i].JiraStatuses = append([]string(nil), cfg.StatusMap[i].JiraStatuses...)
		}
	}
	return &out
}

func marshalJiraProviderPayload(provider JiraProviderConfig) (string, error) {
	content, err := yaml.Marshal(map[string]GlobalProvidersConfig{
		"providers": {Jira: cloneJiraProviderConfig(provider)},
	})
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func bytesEqualTrimmed(a []byte, b []byte) bool {
	return strings.TrimSpace(string(a)) == strings.TrimSpace(string(b))
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

func fetchJiraAPIPostHTTP(apiBase string, path string, body []byte, email string, token string) ([]byte, error) {
	endpoint := strings.TrimRight(apiBase, "/") + path
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
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
		return nil, fmt.Errorf("Jira API POST %s failed: %s", path, resp.Status)
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
	fmt.Fprintf(&b, "jira init: %s provider discovery\n", report.Status)
	fmt.Fprintf(&b, "repo: %s\n", report.Repo)
	fmt.Fprintf(&b, "api_base: %s\n", report.APIBase)
	fmt.Fprintf(&b, "config_root: %s\n", report.ConfigRoot)
	fmt.Fprintf(&b, "config_file: %s\n", report.File.Path)
	fmt.Fprintf(&b, "config_action: %s\n", report.File.Action)
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
	if strings.TrimSpace(report.ConfigPayload) != "" {
		b.WriteString("provider config payload:\n")
		for _, line := range strings.Split(strings.TrimRight(report.ConfigPayload, "\n"), "\n") {
			fmt.Fprintf(&b, "  %s\n", line)
		}
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
