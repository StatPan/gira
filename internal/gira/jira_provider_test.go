package gira

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildJiraProviderInitReportDiscoversReadOnlyConfigPlan(t *testing.T) {
	calls := fakeJiraProviderDiscovery(t)
	root := t.TempDir()

	report, err := BuildJiraProviderInitReport(JiraProviderInitInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		APIBase:    "https://jira.example/",
		Project:    "abc",
		Email:      "alice@example.com",
		Token:      "secret-token",
		ConfigRoot: root,
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("BuildJiraProviderInitReport error: %v", err)
	}
	if report.Command != "jira init" || !report.ReadOnly || report.Project.Key != "ABC" || report.APIBase != "https://jira.example" || report.Status != "planned" {
		t.Fatalf("unexpected report metadata: %+v", report)
	}
	if report.File.Path != filepath.Join(root, "repos", "StatPan", "gira.yaml") || report.File.Action != "create" {
		t.Fatalf("unexpected config file plan: %+v", report.File)
	}
	if len(report.IssueTypes) != 2 || report.IssueTypes[0].Name != "Bug" {
		t.Fatalf("issue types should be sorted and complete: %+v", report.IssueTypes)
	}
	if len(report.Statuses) != 4 || len(report.Priorities) != 2 {
		t.Fatalf("unexpected discovered values: statuses=%+v priorities=%+v", report.Statuses, report.Priorities)
	}
	if report.ConfigProposal.Provider.Mode != "primary" || report.ConfigProposal.Provider.SourceOfTruth.Status != "jira" || report.ConfigProposal.GitHub.Repo != "StatPan/gira" {
		t.Fatalf("unexpected config proposal: %+v", report.ConfigProposal)
	}
	if !containsJiraStatusMap(report.ConfigProposal.Provider.StatusMap, "ready", "To Do") || !containsJiraStatusMap(report.ConfigProposal.Provider.StatusMap, "review", "Code Review") {
		t.Fatalf("status map did not infer expected values: %+v", report.ConfigProposal.Provider.StatusMap)
	}
	for _, want := range []string{"providers:", "jira:", "base_url: https://jira.example", "project_key: ABC"} {
		if !strings.Contains(report.ConfigPayload, want) {
			t.Fatalf("config payload missing %q:\n%s", want, report.ConfigPayload)
		}
	}
	out := FormatJiraProviderInitReport(report)
	for _, want := range []string{"jira init: planned provider discovery", "workflow_mutation: manual_admin_required", "providers.jira.mode: primary", "provider config payload:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("formatted report missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "secret-token") || strings.Contains(strings.Join(*calls, " "), "secret-token") {
		t.Fatalf("credential leaked: calls=%v output=%s", calls, out)
	}
}

func TestBuildJiraProviderInitReportRequiresCredentials(t *testing.T) {
	t.Setenv("JIRA_EMAIL", "")
	t.Setenv("JIRA_API_TOKEN", "")
	_, err := BuildJiraProviderInitReport(JiraProviderInitInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		APIBase:    "https://jira.example",
		Project:    "ABC",
		ConfigRoot: t.TempDir(),
		DryRun:     true,
	})
	if err == nil || !strings.Contains(err.Error(), "JIRA_EMAIL and JIRA_API_TOKEN") {
		t.Fatalf("expected credential error, got %v", err)
	}
}

func TestBuildJiraProviderInitRejectsCredentialBearingAPIBase(t *testing.T) {
	restore := jiraAPIGet
	t.Cleanup(func() { jiraAPIGet = restore })
	jiraAPIGet = func(apiBase string, path string, query map[string]string, email string, token string) ([]byte, error) {
		t.Fatalf("Jira API should not be called for credential-bearing API base")
		return nil, nil
	}

	_, err := BuildJiraProviderInitReport(JiraProviderInitInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		APIBase:    "https://alice:secret-token@jira.example",
		Project:    "ABC",
		Email:      "alice@example.com",
		Token:      "secret-token",
		ConfigRoot: t.TempDir(),
		DryRun:     true,
	})
	if err == nil || !strings.Contains(err.Error(), "must not contain credentials") {
		t.Fatalf("expected credential-bearing api-base error, got %v", err)
	}
}

func TestBuildJiraProviderInitApplyWritesNonSecretRepoRegistry(t *testing.T) {
	fakeJiraProviderDiscovery(t)
	root := t.TempDir()
	report, err := BuildJiraProviderInitReport(JiraProviderInitInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		APIBase:    "https://jira.example",
		Project:    "ABC",
		Email:      "alice@example.com",
		Token:      "secret-token",
		ConfigRoot: root,
		Apply:      true,
	})
	if err != nil {
		t.Fatalf("BuildJiraProviderInitReport apply error: %v", err)
	}
	if !report.Applied || report.Status != "applied" {
		t.Fatalf("unexpected apply report: %+v", report)
	}
	path := filepath.Join(root, "repos", "StatPan", "gira.yaml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read repo registry: %v", err)
	}
	text := string(content)
	for _, want := range []string{"repo: StatPan/gira", "providers:", "jira:", "base_url: https://jira.example", "project_key: ABC"} {
		if !strings.Contains(text, want) {
			t.Fatalf("repo registry missing %q:\n%s", want, text)
		}
	}
	for _, forbidden := range []string{"secret-token", "alice@example.com"} {
		if strings.Contains(text, forbidden) || strings.Contains(report.ConfigPayload, forbidden) {
			t.Fatalf("credential leaked %q:\nregistry=%s\npayload=%s", forbidden, text, report.ConfigPayload)
		}
	}
}

func TestBuildJiraProviderInitApplyMergesExistingRegistry(t *testing.T) {
	fakeJiraProviderDiscovery(t)
	root := t.TempDir()
	path := filepath.Join(root, "repos", "StatPan", "gira.yaml")
	writeTestFile(t, path, `repo: StatPan/gira
path: ~/workspace/apps/gira
aliases:
  - gira
workspace:
  name: personal
`)

	report, err := BuildJiraProviderInitReport(JiraProviderInitInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		APIBase:    "https://jira.example",
		Project:    "ABC",
		Email:      "alice@example.com",
		Token:      "secret-token",
		ConfigRoot: root,
		Apply:      true,
	})
	if err != nil {
		t.Fatalf("BuildJiraProviderInitReport apply error: %v", err)
	}
	if report.File.Action != "update" {
		t.Fatalf("expected update action, got %+v", report.File)
	}
	loaded, err := LoadGlobalRepoRegistryEntry(root, ParseRepoRefMust("StatPan/gira"))
	if err != nil {
		t.Fatalf("load merged repo registry: %v", err)
	}
	if loaded.Path != "~/workspace/apps/gira" || loaded.Workspace.Name != "personal" || len(loaded.Aliases) != 1 || loaded.Providers == nil || loaded.Providers.Jira == nil {
		t.Fatalf("existing fields were not preserved: %+v", loaded)
	}
}

func TestBuildJiraProviderInitDryRunReportsProviderConflict(t *testing.T) {
	fakeJiraProviderDiscovery(t)
	root := t.TempDir()
	path := filepath.Join(root, "repos", "StatPan", "gira.yaml")
	writeTestFile(t, path, `repo: StatPan/gira
providers:
  jira:
    enabled: true
    mode: primary
    base_url: https://old-jira.example
    project_key: OLD
    source_of_truth:
      planning: jira
      status: jira
      execution: github
`)

	report, err := BuildJiraProviderInitReport(JiraProviderInitInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		APIBase:    "https://jira.example",
		Project:    "ABC",
		Email:      "alice@example.com",
		Token:      "secret-token",
		ConfigRoot: root,
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("dry-run conflict should return blocked report without error: %v", err)
	}
	if report.Status != "blocked" || report.File.Action != "conflict" {
		t.Fatalf("expected blocked conflict report: %+v", report)
	}
}

func TestBuildJiraProviderInitConflictsOnDifferentProvider(t *testing.T) {
	fakeJiraProviderDiscovery(t)
	root := t.TempDir()
	path := filepath.Join(root, "repos", "StatPan", "gira.yaml")
	writeTestFile(t, path, `repo: StatPan/gira
providers:
  jira:
    enabled: true
    mode: primary
    base_url: https://old-jira.example
    project_key: OLD
    source_of_truth:
      planning: jira
      status: jira
      execution: github
`)

	report, err := BuildJiraProviderInitReport(JiraProviderInitInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		APIBase:    "https://jira.example",
		Project:    "ABC",
		Email:      "alice@example.com",
		Token:      "secret-token",
		ConfigRoot: root,
		Apply:      true,
	})
	if err == nil || !strings.Contains(err.Error(), "different providers.jira config") {
		t.Fatalf("expected provider conflict, got report=%+v err=%v", report, err)
	}
	if report.Status != "blocked" || report.File.Action != "conflict" {
		t.Fatalf("expected blocked conflict report: %+v", report)
	}
}

func TestBuildJiraProviderInitOverwriteReplacesInvalidProvider(t *testing.T) {
	fakeJiraProviderDiscovery(t)
	root := t.TempDir()
	path := filepath.Join(root, "repos", "StatPan", "gira.yaml")
	writeTestFile(t, path, `repo: StatPan/gira
path: ~/workspace/apps/gira
providers:
  jira:
    enabled: true
    mode: primary
    base_url: not-a-url
    project_key: OLD
    source_of_truth:
      planning: jira
      status: jira
      execution: github
`)

	report, err := BuildJiraProviderInitReport(JiraProviderInitInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		APIBase:    "https://jira.example",
		Project:    "ABC",
		Email:      "alice@example.com",
		Token:      "secret-token",
		ConfigRoot: root,
		Overwrite:  true,
		Apply:      true,
	})
	if err != nil {
		t.Fatalf("overwrite should replace invalid provider block: %v", err)
	}
	if !report.Applied || report.File.Action != "overwrite" {
		t.Fatalf("unexpected overwrite report: %+v", report)
	}
	loaded, err := LoadGlobalRepoRegistryEntry(root, ParseRepoRefMust("StatPan/gira"))
	if err != nil {
		t.Fatalf("load overwritten repo registry: %v", err)
	}
	if loaded.Path != "~/workspace/apps/gira" || loaded.Providers == nil || loaded.Providers.Jira == nil || loaded.Providers.Jira.BaseURL != "https://jira.example" {
		t.Fatalf("provider overwrite did not preserve/replace expected fields: %+v", loaded)
	}
}

func containsJiraStatusMap(items []JiraProviderStatusMap, status string, jiraStatus string) bool {
	for _, item := range items {
		if item.GiraStatus != status {
			continue
		}
		for _, value := range item.JiraStatuses {
			if value == jiraStatus {
				return true
			}
		}
	}
	return false
}

func fakeJiraProviderDiscovery(t *testing.T) *[]string {
	t.Helper()
	restore := jiraAPIGet
	t.Cleanup(func() { jiraAPIGet = restore })
	var calls []string
	jiraAPIGet = func(apiBase string, path string, query map[string]string, email string, token string) ([]byte, error) {
		calls = append(calls, fmt.Sprintf("%s %s %s %s", apiBase, path, query["projectId"], email))
		if token != "secret-token" {
			t.Fatalf("unexpected token")
		}
		switch path {
		case "/rest/api/3/project/ABC":
			return []byte(`{"id":"10000","key":"ABC","name":"Alpha Board","projectTypeKey":"software","simplified":true}`), nil
		case "/rest/api/3/issuetype/project":
			return []byte(`[{"id":"10001","name":"Task","subtask":false,"hierarchyLevel":0},{"id":"10002","name":"Bug","subtask":false,"hierarchyLevel":0}]`), nil
		case "/rest/api/3/statuses/search":
			return []byte(`{"values":[{"id":"1","name":"To Do","statusCategory":{"name":"To Do"}},{"id":"2","name":"In Progress","statusCategory":{"name":"In Progress"}},{"id":"3","name":"Code Review","statusCategory":{"name":"In Progress"}},{"id":"4","name":"Done","statusCategory":{"name":"Done"}}]}`), nil
		case "/rest/api/3/priority/search":
			return []byte(`{"values":[{"id":"1","name":"High"},{"id":"2","name":"Medium"}]}`), nil
		default:
			return nil, fmt.Errorf("unexpected path %s", path)
		}
	}
	return &calls
}
