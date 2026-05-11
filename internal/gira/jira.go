package gira

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const JiraExportSchemaVersion = "v1alpha1"

var jiraImportCSVHeaders = []string{"key", "summary", "description", "status", "priority", "assignee", "labels"}
var jiraExportCSVHeaders = []string{"number", "title", "body", "state", "status", "priority", "assignee", "labels", "jira_key", "url"}
var jiraKeyLinePattern = regexp.MustCompile(`(?im)^Jira-Key:\s*([A-Z][A-Z0-9]+-\d+)\s*$`)

var jiraAPISearch = fetchJiraAPISearchHTTP

type JiraWorkItem struct {
	Key         string   `json:"key"`
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	Assignee    string   `json:"assignee,omitempty"`
	Labels      []string `json:"labels"`
	URL         string   `json:"url,omitempty"`
}

type JiraImportCounts struct {
	SourceItems int `json:"source_items"`
	Create      int `json:"create"`
	Duplicate   int `json:"duplicate"`
	Applied     int `json:"applied"`
}

type JiraImportAction struct {
	Key         string   `json:"key"`
	Action      string   `json:"action"`
	Reason      string   `json:"reason,omitempty"`
	IssueNumber int      `json:"issue_number,omitempty"`
	Title       string   `json:"title"`
	Labels      []string `json:"labels"`
}

type JiraImportReport struct {
	Command string             `json:"command"`
	Repo    string             `json:"repo"`
	Source  string             `json:"source,omitempty"`
	APIBase string             `json:"api_base,omitempty"`
	Project string             `json:"project,omitempty"`
	DryRun  bool               `json:"dry_run"`
	Apply   bool               `json:"apply"`
	Counts  JiraImportCounts   `json:"counts"`
	Actions []JiraImportAction `json:"actions"`
}

type JiraMirrorInput struct {
	Repo       RepoRef `json:"-"`
	Key        string  `json:"key"`
	APIBase    string  `json:"api_base,omitempty"`
	ConfigRoot string  `json:"config_root,omitempty"`
	Email      string  `json:"-"`
	Token      string  `json:"-"`
	DryRun     bool    `json:"dry_run"`
	Apply      bool    `json:"apply"`
}

type JiraMirrorReport struct {
	Command    string            `json:"command"`
	Repo       string            `json:"repo"`
	Key        string            `json:"key"`
	APIBase    string            `json:"api_base"`
	DryRun     bool              `json:"dry_run"`
	Apply      bool              `json:"apply"`
	Status     string            `json:"status"`
	Action     string            `json:"action"`
	Reason     string            `json:"reason,omitempty"`
	Issue      JiraMirrorIssue   `json:"issue,omitempty"`
	Duplicates []JiraMirrorIssue `json:"duplicates,omitempty"`
	Item       JiraWorkItem      `json:"item"`
	Labels     []string          `json:"labels"`
	NextStep   string            `json:"next_step,omitempty"`
}

type JiraMirrorIssue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url,omitempty"`
}

type JiraExportIssue struct {
	Number   int      `json:"number"`
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	State    string   `json:"state"`
	Status   string   `json:"status,omitempty"`
	Priority string   `json:"priority,omitempty"`
	Assignee string   `json:"assignee,omitempty"`
	Labels   []string `json:"labels"`
	JiraKey  string   `json:"jira_key,omitempty"`
	URL      string   `json:"url"`
}

type JiraExportArtifact struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

type JiraExportReport struct {
	Command       string               `json:"command"`
	Repo          string               `json:"repo"`
	OutputRoot    string               `json:"output_root"`
	SchemaVersion string               `json:"schema_version"`
	Artifacts     []JiraExportArtifact `json:"artifacts"`
	Counts        struct {
		Issues int `json:"issues"`
	} `json:"counts"`
}

func LoadJiraImportFile(path string) ([]JiraWorkItem, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv":
		return ParseJiraImportCSV(content)
	case ".json":
		return ParseJiraImportJSON(content)
	default:
		return nil, fmt.Errorf("unsupported Jira import source %q: expected .csv or .json", path)
	}
}

func ParseJiraImportCSV(content []byte) ([]JiraWorkItem, error) {
	reader := csv.NewReader(bytes.NewReader(content))
	reader.TrimLeadingSpace = true
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	headers := map[string]int{}
	for i, header := range rows[0] {
		headers[strings.ToLower(strings.TrimSpace(header))] = i
	}
	items := make([]JiraWorkItem, 0, len(rows)-1)
	for _, row := range rows[1:] {
		item := JiraWorkItem{
			Key:         csvValue(row, headers, "key"),
			Summary:     csvValue(row, headers, "summary"),
			Description: csvValue(row, headers, "description"),
			Status:      csvValue(row, headers, "status"),
			Priority:    csvValue(row, headers, "priority"),
			Assignee:    csvValue(row, headers, "assignee"),
			Labels:      splitJiraLabels(csvValue(row, headers, "labels")),
		}
		normalized, err := NormalizeJiraWorkItem(item)
		if err != nil {
			return nil, err
		}
		items = append(items, normalized)
	}
	return items, nil
}

func ParseJiraImportJSON(content []byte) ([]JiraWorkItem, error) {
	var direct []JiraWorkItem
	if err := json.Unmarshal(content, &direct); err == nil {
		return normalizeJiraItems(direct)
	}
	var wrapped struct {
		Issues []JiraWorkItem `json:"issues"`
	}
	if err := json.Unmarshal(content, &wrapped); err != nil {
		return nil, fmt.Errorf("parse Jira import JSON: %w", err)
	}
	return normalizeJiraItems(wrapped.Issues)
}

func NormalizeJiraWorkItem(item JiraWorkItem) (JiraWorkItem, error) {
	item.Key = strings.ToUpper(strings.TrimSpace(item.Key))
	item.Summary = strings.TrimSpace(item.Summary)
	item.Description = strings.TrimSpace(item.Description)
	item.Status = strings.TrimSpace(item.Status)
	item.Priority = strings.TrimSpace(item.Priority)
	item.Assignee = strings.TrimSpace(item.Assignee)
	item.Labels = normalizeJiraLabels(item.Labels)
	item.URL = strings.TrimSpace(item.URL)
	if item.Key == "" {
		return JiraWorkItem{}, fmt.Errorf("Jira item key is required")
	}
	if item.Summary == "" {
		return JiraWorkItem{}, fmt.Errorf("Jira item %s summary is required", item.Key)
	}
	return item, nil
}

func ImportJiraItems(repo RepoRef, source string, items []JiraWorkItem, dryRun bool, apply bool, runner CommandRunner) (JiraImportReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if dryRun == apply {
		return JiraImportReport{}, fmt.Errorf("exactly one of --dry-run or --apply is required")
	}
	normalized, err := normalizeJiraItems(items)
	if err != nil {
		return JiraImportReport{}, err
	}
	existing, err := fetchJiraExistingIssues(repo, runner)
	if err != nil {
		return JiraImportReport{}, err
	}
	report := JiraImportReport{
		Command: "jira import",
		Repo:    repo.FullName(),
		Source:  source,
		DryRun:  dryRun,
		Apply:   apply,
		Counts:  JiraImportCounts{SourceItems: len(normalized)},
	}
	seen := map[string]bool{}
	for _, item := range normalized {
		labels := JiraGitHubLabels(item)
		action := JiraImportAction{Key: item.Key, Title: item.Summary, Labels: labels}
		if seen[item.Key] {
			action.Action = "duplicate"
			action.Reason = "duplicate_key_in_source"
			report.Counts.Duplicate++
			report.Actions = append(report.Actions, action)
			continue
		}
		seen[item.Key] = true
		if found, ok := existing[item.Key]; ok {
			action.Action = "duplicate"
			action.Reason = "jira_key_exists"
			action.IssueNumber = found.Number
			report.Counts.Duplicate++
			report.Actions = append(report.Actions, action)
			continue
		}
		if found, ok := existing[strings.ToLower(item.Summary)]; ok {
			action.Action = "duplicate"
			action.Reason = "title_exists"
			action.IssueNumber = found.Number
			report.Counts.Duplicate++
			report.Actions = append(report.Actions, action)
			continue
		}
		action.Action = "create"
		report.Counts.Create++
		if apply {
			number, err := createJiraGitHubIssue(repo, item, labels, runner)
			if err != nil {
				return report, err
			}
			action.IssueNumber = number
			report.Counts.Applied++
		}
		report.Actions = append(report.Actions, action)
	}
	return report, nil
}

func MirrorJiraIssue(input JiraMirrorInput, runner CommandRunner) (JiraMirrorReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if input.DryRun == input.Apply {
		return JiraMirrorReport{}, fmt.Errorf("exactly one of --dry-run or --apply is required")
	}
	key, err := normalizeJiraKey(input.Key)
	if err != nil {
		return JiraMirrorReport{}, err
	}
	apiBase, err := resolveJiraMirrorAPIBase(input)
	if err != nil {
		return JiraMirrorReport{}, err
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
		return JiraMirrorReport{}, fmt.Errorf("JIRA_EMAIL and JIRA_API_TOKEN are required for jira mirror")
	}
	item, err := FetchJiraIssueByKey(apiBase, key, email, token)
	if err != nil {
		return JiraMirrorReport{}, err
	}
	matches, err := findJiraMirrorIssues(input.Repo, key, runner)
	if err != nil {
		return JiraMirrorReport{}, err
	}
	labels := JiraGitHubLabels(item)
	report := JiraMirrorReport{
		Command: "jira mirror",
		Repo:    input.Repo.FullName(),
		Key:     key,
		APIBase: apiBase,
		DryRun:  input.DryRun,
		Apply:   input.Apply,
		Item:    item,
		Labels:  labels,
	}
	switch len(matches) {
	case 0:
		report.Action = "create"
		report.Status = actionStatus(input.DryRun)
		report.NextStep = jiraMirrorNextStep(input, key)
		if input.Apply {
			number, err := createJiraGitHubIssue(input.Repo, item, labels, runner)
			if err != nil {
				return report, err
			}
			report.Issue = JiraMirrorIssue{Number: number, Title: item.Summary}
			report.Status = "applied"
			report.NextStep = fmt.Sprintf("gira ticket view %d --repo %s", number, input.Repo.FullName())
		}
	case 1:
		report.Action = "reuse"
		report.Status = "skipped"
		report.Issue = matches[0]
		report.Reason = "jira_key_exists"
		report.NextStep = fmt.Sprintf("gira ticket view %d --repo %s", matches[0].Number, input.Repo.FullName())
	default:
		report.Action = "conflict"
		report.Status = "blocked"
		report.Reason = "duplicate_jira_key"
		report.Duplicates = matches
	}
	return report, nil
}

func ImportJiraFromAPI(repo RepoRef, apiBase string, project string, dryRun bool, apply bool, runner CommandRunner) (JiraImportReport, error) {
	normalizedAPIBase, err := normalizeJiraAPIBase(apiBase)
	if err != nil {
		return JiraImportReport{}, err
	}
	items, err := FetchJiraAPIItems(normalizedAPIBase, project, os.Getenv("JIRA_EMAIL"), os.Getenv("JIRA_API_TOKEN"), runner)
	if err != nil {
		return JiraImportReport{}, err
	}
	report, err := ImportJiraItems(repo, "", items, dryRun, apply, runner)
	report.APIBase = normalizedAPIBase
	report.Project = strings.ToUpper(strings.TrimSpace(project))
	return report, err
}

func FetchJiraAPIItems(apiBase string, project string, email string, token string, runner CommandRunner) ([]JiraWorkItem, error) {
	var err error
	apiBase, err = normalizeJiraAPIBase(apiBase)
	if err != nil {
		return nil, err
	}
	project = strings.ToUpper(strings.TrimSpace(project))
	if project == "" {
		return nil, fmt.Errorf("--api-base and --project are required for Jira API import")
	}
	if strings.TrimSpace(email) == "" || strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("JIRA_EMAIL and JIRA_API_TOKEN are required for Jira API import")
	}
	var items []JiraWorkItem
	startAt := 0
	maxResults := 100
	for {
		output, err := jiraAPISearch(apiBase, project, email, token, startAt, maxResults)
		if err != nil {
			return nil, err
		}
		pageItems, page, err := parseJiraAPISearchPage(output)
		if err != nil {
			return nil, err
		}
		items = append(items, pageItems...)
		nextStart := page.StartAt + page.MaxResults
		if page.MaxResults <= 0 || nextStart >= page.Total || len(pageItems) == 0 {
			break
		}
		startAt = nextStart
	}
	return normalizeJiraItems(items)
}

func FetchJiraIssueByKey(apiBase string, key string, email string, token string) (JiraWorkItem, error) {
	apiBase, err := normalizeJiraAPIBase(apiBase)
	if err != nil {
		return JiraWorkItem{}, err
	}
	key, err = normalizeJiraKey(key)
	if err != nil {
		return JiraWorkItem{}, err
	}
	if strings.TrimSpace(email) == "" || strings.TrimSpace(token) == "" {
		return JiraWorkItem{}, fmt.Errorf("JIRA_EMAIL and JIRA_API_TOKEN are required for jira issue fetch")
	}
	content, err := jiraAPIGet(apiBase, "/rest/api/3/issue/"+url.PathEscape(key), map[string]string{"fields": "summary,description,status,priority,assignee,labels"}, email, token)
	if err != nil {
		return JiraWorkItem{}, fmt.Errorf("fetch Jira issue %s: %w", key, err)
	}
	var issue jiraAPIIssue
	if err := json.Unmarshal(content, &issue); err != nil {
		return JiraWorkItem{}, fmt.Errorf("parse Jira issue JSON: %w", err)
	}
	item, err := jiraWorkItemFromAPIIssue(issue)
	if err != nil {
		return JiraWorkItem{}, err
	}
	item.URL = apiBase + "/browse/" + item.Key
	return item, nil
}

func JiraGitHubLabels(item JiraWorkItem) []string {
	labels := []string{"jira:" + item.Key}
	if status := jiraAxisLabel("status", item.Status); status != "" {
		labels = append(labels, status)
	}
	if priority := jiraAxisLabel("priority", item.Priority); priority != "" {
		labels = append(labels, priority)
	}
	labels = append(labels, item.Labels...)
	return normalizeJiraLabels(labels)
}

func JiraIssueBody(item JiraWorkItem) string {
	var b strings.Builder
	b.WriteString(item.Description)
	if strings.TrimSpace(item.Description) != "" {
		b.WriteString("\n\n")
	}
	b.WriteString("Imported from Jira.\n")
	b.WriteString("Jira-owned fields are mirrored/evidence-only in GitHub; Jira remains the planning/status source of truth.\n\n")
	b.WriteString("Jira-Key: " + item.Key + "\n")
	if item.URL != "" {
		b.WriteString("Jira-URL: " + item.URL + "\n")
	}
	if item.Status != "" {
		b.WriteString("Jira-Status: " + item.Status + "\n")
	}
	if item.Priority != "" {
		b.WriteString("Jira-Priority: " + item.Priority + "\n")
	}
	if item.Assignee != "" {
		b.WriteString("Jira-Assignee: " + item.Assignee + "\n")
	}
	if len(item.Labels) > 0 {
		b.WriteString("Jira-Labels: " + strings.Join(item.Labels, ", ") + "\n")
	}
	return b.String()
}

func ExportJiraIssues(repo RepoRef, outputRoot string, runner CommandRunner) (JiraExportReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	issues, err := fetchJiraExportIssues(repo, runner)
	if err != nil {
		return JiraExportReport{}, err
	}
	if err := WriteJiraExport(outputRoot, issues); err != nil {
		return JiraExportReport{}, err
	}
	report := JiraExportReport{
		Command:       "jira export",
		Repo:          repo.FullName(),
		OutputRoot:    outputRoot,
		SchemaVersion: JiraExportSchemaVersion,
		Artifacts: []JiraExportArtifact{
			{Path: filepath.Join(outputRoot, "issues.json"), Kind: "json"},
			{Path: filepath.Join(outputRoot, "issues.csv"), Kind: "csv"},
		},
	}
	report.Counts.Issues = len(issues)
	return report, nil
}

func WriteJiraExport(outputRoot string, issues []JiraExportIssue) error {
	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(issues, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(filepath.Join(outputRoot, "issues.json"), encoded, 0o644); err != nil {
		return err
	}
	csvBytes, err := RenderJiraExportCSV(issues)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputRoot, "issues.csv"), csvBytes, 0o644)
}

func RenderJiraExportCSV(issues []JiraExportIssue) ([]byte, error) {
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	if err := writer.Write(jiraExportCSVHeaders); err != nil {
		return nil, err
	}
	for _, issue := range issues {
		row := []string{
			strconv.Itoa(issue.Number),
			issue.Title,
			issue.Body,
			issue.State,
			issue.Status,
			issue.Priority,
			issue.Assignee,
			strings.Join(issue.Labels, ","),
			issue.JiraKey,
			issue.URL,
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return buffer.Bytes(), writer.Error()
}

func FormatJiraImportReport(report JiraImportReport) string {
	lines := []string{
		"jira import:",
		"repo: " + report.Repo,
		fmt.Sprintf("source_items: %d", report.Counts.SourceItems),
		fmt.Sprintf("create: %d", report.Counts.Create),
		fmt.Sprintf("duplicate: %d", report.Counts.Duplicate),
		fmt.Sprintf("applied: %d", report.Counts.Applied),
	}
	return strings.Join(lines, "\n") + "\n"
}

func FormatJiraMirrorReport(report JiraMirrorReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "jira mirror: %s %s\n", report.Status, report.Key)
	fmt.Fprintf(&b, "repo: %s\n", report.Repo)
	fmt.Fprintf(&b, "action: %s\n", report.Action)
	if report.Issue.Number > 0 {
		fmt.Fprintf(&b, "issue: #%d %s\n", report.Issue.Number, report.Issue.Title)
	}
	if len(report.Duplicates) > 0 {
		b.WriteString("duplicates:\n")
		for _, issue := range report.Duplicates {
			fmt.Fprintf(&b, "  - #%d %s\n", issue.Number, issue.Title)
		}
	}
	if len(report.Labels) > 0 {
		fmt.Fprintf(&b, "labels: %s\n", strings.Join(report.Labels, ","))
	}
	if strings.TrimSpace(report.NextStep) != "" {
		fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	}
	return b.String()
}

func jiraMirrorNextStep(input JiraMirrorInput, key string) string {
	parts := []string{"gira", "jira", "mirror", key, "--repo", input.Repo.FullName()}
	if strings.TrimSpace(input.APIBase) != "" {
		parts = append(parts, "--api-base", strings.TrimRight(strings.TrimSpace(input.APIBase), "/"))
	}
	if strings.TrimSpace(input.ConfigRoot) != "" {
		parts = append(parts, "--config-root", strings.TrimSpace(input.ConfigRoot))
	}
	parts = append(parts, "--apply")
	return strings.Join(parts, " ")
}

func FormatJiraExportReport(report JiraExportReport) string {
	return fmt.Sprintf("jira export artifacts written to %s\nissues: %d\n", report.OutputRoot, report.Counts.Issues)
}

type jiraExistingIssue struct {
	Number int
	Title  string
}

func findJiraMirrorIssues(repo RepoRef, key string, runner CommandRunner) ([]JiraMirrorIssue, error) {
	output, err := runner.Run("gh", "issue", "list", "--repo", repo.FullName(), "--state", "all", "--search", key+" in:body", "--limit", "1000", "--json", "number,title,body,url")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(output, &rows); err != nil {
		return nil, fmt.Errorf("parse GitHub issues: %w", err)
	}
	var matches []JiraMirrorIssue
	for _, row := range rows {
		if JiraKeyFromBody(row.Body) == key {
			matches = append(matches, JiraMirrorIssue{Number: row.Number, Title: row.Title, URL: row.URL})
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Number < matches[j].Number })
	return matches, nil
}

func fetchJiraExistingIssues(repo RepoRef, runner CommandRunner) (map[string]jiraExistingIssue, error) {
	output, err := runner.Run("gh", "issue", "list", "--repo", repo.FullName(), "--state", "all", "--limit", "1000", "--json", "number,title,body")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
	}
	if err := json.Unmarshal(output, &rows); err != nil {
		return nil, fmt.Errorf("parse GitHub issues: %w", err)
	}
	existing := map[string]jiraExistingIssue{}
	for _, row := range rows {
		issue := jiraExistingIssue{Number: row.Number, Title: row.Title}
		if key := JiraKeyFromBody(row.Body); key != "" {
			existing[key] = issue
		}
		existing[strings.ToLower(strings.TrimSpace(row.Title))] = issue
	}
	return existing, nil
}

func createJiraGitHubIssue(repo RepoRef, item JiraWorkItem, labels []string, runner CommandRunner) (int, error) {
	if err := ensureJiraGitHubLabels(repo, labels, runner); err != nil {
		return 0, err
	}
	args := []string{"issue", "create", "--repo", repo.FullName(), "--title", item.Summary, "--body", JiraIssueBody(item)}
	for _, label := range labels {
		args = append(args, "--label", label)
	}
	output, err := runner.Run("gh", args...)
	if err != nil {
		return 0, err
	}
	return issueNumberFromURL(strings.TrimSpace(string(output))), nil
}

func ensureJiraGitHubLabels(repo RepoRef, labels []string, runner CommandRunner) error {
	if len(labels) == 0 {
		return nil
	}
	output, err := runner.Run("gh", "label", "list", "--repo", repo.FullName(), "--json", "name", "--limit", "1000")
	if err != nil {
		return err
	}
	var rows []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(output, &rows); err != nil {
		return fmt.Errorf("parse label list: %w", err)
	}
	existing := map[string]struct{}{}
	for _, row := range rows {
		existing[strings.ToLower(strings.TrimSpace(row.Name))] = struct{}{}
	}
	for _, label := range labels {
		trimmed := strings.TrimSpace(label)
		if trimmed == "" {
			continue
		}
		if _, ok := existing[strings.ToLower(trimmed)]; ok {
			continue
		}
		if _, err := runner.Run("gh", "label", "create", trimmed, "--repo", repo.FullName(), "--color", jiraGitHubLabelColor(trimmed), "--description", jiraGitHubLabelDescription(trimmed)); err != nil {
			return err
		}
		existing[strings.ToLower(trimmed)] = struct{}{}
	}
	return nil
}

func jiraGitHubLabelColor(label string) string {
	switch {
	case strings.HasPrefix(label, "jira:"):
		return "5319E7"
	case strings.HasPrefix(label, "status:"):
		return "0E8A16"
	case strings.HasPrefix(label, "priority:"):
		return "D93F0B"
	default:
		return "C5DEF5"
	}
}

func jiraGitHubLabelDescription(label string) string {
	switch {
	case strings.HasPrefix(label, "jira:"):
		return "Mirrored Jira issue key."
	case strings.HasPrefix(label, "status:"):
		return "Mirrored Jira status evidence."
	case strings.HasPrefix(label, "priority:"):
		return "Mirrored Jira priority evidence."
	default:
		return "Mirrored Jira label evidence."
	}
}

func fetchJiraExportIssues(repo RepoRef, runner CommandRunner) ([]JiraExportIssue, error) {
	output, err := runner.Run("gh", "issue", "list", "--repo", repo.FullName(), "--state", "all", "--limit", "1000", "--json", "number,title,body,state,labels,assignees,url")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		State  string `json:"state"`
		URL    string `json:"url"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
	}
	if err := json.Unmarshal(output, &rows); err != nil {
		return nil, fmt.Errorf("parse GitHub issues: %w", err)
	}
	issues := make([]JiraExportIssue, 0, len(rows))
	for _, row := range rows {
		labels := make([]string, 0, len(row.Labels))
		for _, label := range row.Labels {
			if strings.TrimSpace(label.Name) != "" {
				labels = append(labels, label.Name)
			}
		}
		sort.Strings(labels)
		assignee := ""
		if len(row.Assignees) > 0 {
			assignee = row.Assignees[0].Login
		}
		issues = append(issues, JiraExportIssue{
			Number:   row.Number,
			Title:    row.Title,
			Body:     row.Body,
			State:    row.State,
			Status:   jiraLabelAxis(labels, "status"),
			Priority: jiraLabelAxis(labels, "priority"),
			Assignee: assignee,
			Labels:   labels,
			JiraKey:  JiraKeyFromBody(row.Body),
			URL:      row.URL,
		})
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].Number < issues[j].Number })
	return issues, nil
}

type jiraAPIPage struct {
	StartAt    int
	MaxResults int
	Total      int
}

type jiraAPIIssue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary     string          `json:"summary"`
		Description json.RawMessage `json:"description"`
		Status      *struct {
			Name string `json:"name"`
		} `json:"status"`
		Priority *struct {
			Name string `json:"name"`
		} `json:"priority"`
		Assignee *struct {
			Name         string `json:"name"`
			AccountID    string `json:"accountId"`
			EmailAddress string `json:"emailAddress"`
		} `json:"assignee"`
		Labels []string `json:"labels"`
	} `json:"fields"`
}

func parseJiraAPISearch(content []byte) ([]JiraWorkItem, error) {
	items, _, err := parseJiraAPISearchPage(content)
	return items, err
}

func parseJiraAPISearchPage(content []byte) ([]JiraWorkItem, jiraAPIPage, error) {
	var payload jiraAPISearchPayload
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, jiraAPIPage{}, fmt.Errorf("parse Jira API response: %w", err)
	}
	items := make([]JiraWorkItem, 0, len(payload.Issues))
	for _, issue := range payload.Issues {
		item, err := jiraWorkItemFromAPIIssue(issue)
		if err != nil {
			return nil, jiraAPIPage{}, err
		}
		items = append(items, item)
	}
	normalized, err := normalizeJiraItems(items)
	return normalized, jiraAPIPage{StartAt: payload.StartAt, MaxResults: payload.MaxResults, Total: payload.Total}, err
}

type jiraAPISearchPayload struct {
	StartAt    int            `json:"startAt"`
	MaxResults int            `json:"maxResults"`
	Total      int            `json:"total"`
	Issues     []jiraAPIIssue `json:"issues"`
}

func fetchJiraAPISearchHTTP(apiBase string, project string, email string, token string, startAt int, maxResults int) ([]byte, error) {
	endpoint := apiBase + "/rest/api/3/search"
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	query := req.URL.Query()
	query.Set("jql", "project = "+project+" ORDER BY key ASC")
	query.Set("fields", "summary,description,status,priority,assignee,labels")
	query.Set("startAt", strconv.Itoa(startAt))
	query.Set("maxResults", strconv.Itoa(maxResults))
	req.URL.RawQuery = query.Encode()
	req.SetBasicAuth(email, token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Jira API search failed: %s", resp.Status)
	}
	return body, nil
}

func jiraDescriptionText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var node any
	if err := json.Unmarshal(raw, &node); err != nil {
		return ""
	}
	var parts []string
	collectJiraADFText(node, &parts)
	return strings.TrimSpace(strings.Join(parts, " "))
}

func collectJiraADFText(node any, parts *[]string) {
	switch value := node.(type) {
	case map[string]any:
		if text, ok := value["text"].(string); ok && strings.TrimSpace(text) != "" {
			*parts = append(*parts, strings.TrimSpace(text))
		}
		if children, ok := value["content"].([]any); ok {
			for _, child := range children {
				collectJiraADFText(child, parts)
			}
		}
	case []any:
		for _, child := range value {
			collectJiraADFText(child, parts)
		}
	}
}

func normalizeJiraItems(items []JiraWorkItem) ([]JiraWorkItem, error) {
	normalized := make([]JiraWorkItem, 0, len(items))
	for _, item := range items {
		next, err := NormalizeJiraWorkItem(item)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, next)
	}
	sort.SliceStable(normalized, func(i, j int) bool { return normalized[i].Key < normalized[j].Key })
	return normalized, nil
}

func jiraWorkItemFromAPIIssue(issue jiraAPIIssue) (JiraWorkItem, error) {
	status := ""
	if issue.Fields.Status != nil {
		status = issue.Fields.Status.Name
	}
	priority := ""
	if issue.Fields.Priority != nil {
		priority = issue.Fields.Priority.Name
	}
	assignee := ""
	if issue.Fields.Assignee != nil {
		assignee = issue.Fields.Assignee.Name
		if assignee == "" {
			assignee = issue.Fields.Assignee.EmailAddress
		}
		if assignee == "" {
			assignee = issue.Fields.Assignee.AccountID
		}
	}
	return NormalizeJiraWorkItem(JiraWorkItem{
		Key:         issue.Key,
		Summary:     issue.Fields.Summary,
		Description: jiraDescriptionText(issue.Fields.Description),
		Status:      status,
		Priority:    priority,
		Assignee:    assignee,
		Labels:      issue.Fields.Labels,
	})
}

func normalizeJiraKey(value string) (string, error) {
	key := strings.ToUpper(strings.TrimSpace(value))
	if !jiraKeyLinePattern.MatchString("Jira-Key: " + key) {
		return "", fmt.Errorf("Jira key must look like ABC-123")
	}
	return key, nil
}

func resolveJiraMirrorAPIBase(input JiraMirrorInput) (string, error) {
	if strings.TrimSpace(input.APIBase) != "" {
		return normalizeJiraAPIBase(input.APIBase)
	}
	entry, err := LoadGlobalRepoRegistryEntry(input.ConfigRoot, input.Repo)
	if err != nil {
		return "", fmt.Errorf("--api-base is required when no Jira provider config is registered: %w", err)
	}
	if entry.Providers == nil || entry.Providers.Jira == nil || strings.TrimSpace(entry.Providers.Jira.BaseURL) == "" {
		return "", fmt.Errorf("--api-base is required when repo registry has no providers.jira.base_url")
	}
	return normalizeJiraAPIBase(entry.Providers.Jira.BaseURL)
}

func csvValue(row []string, headers map[string]int, name string) string {
	idx, ok := headers[name]
	if !ok || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func splitJiraLabels(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' })
}

func normalizeJiraLabels(labels []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		value := strings.TrimSpace(label)
		if value == "" {
			continue
		}
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func jiraAxisLabel(axis string, value string) string {
	slug := jiraSlug(value)
	if slug == "" {
		return ""
	}
	return axis + ":" + slug
}

func jiraSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return -1
	}, value)
	value = strings.Trim(value, "-")
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	return value
}

func JiraKeyFromBody(body string) string {
	match := jiraKeyLinePattern.FindStringSubmatch(body)
	if len(match) == 2 {
		return strings.ToUpper(match[1])
	}
	return ""
}

func jiraLabelAxis(labels []string, axis string) string {
	prefix := axis + ":"
	for _, label := range labels {
		if strings.HasPrefix(label, prefix) {
			return strings.TrimPrefix(label, prefix)
		}
	}
	return ""
}

func issueNumberFromURL(value string) int {
	if value == "" {
		return 0
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return 0
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) == 0 {
		return 0
	}
	number, _ := strconv.Atoi(parts[len(parts)-1])
	return number
}
