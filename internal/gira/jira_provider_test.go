package gira

import (
	"fmt"
	"strings"
	"testing"
)

func TestBuildJiraProviderInitReportDiscoversReadOnlyConfigPlan(t *testing.T) {
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

	report, err := BuildJiraProviderInitReport(JiraProviderInitInput{
		Repo:    ParseRepoRefMust("StatPan/gira"),
		APIBase: "https://jira.example/",
		Project: "abc",
		Email:   "alice@example.com",
		Token:   "secret-token",
		DryRun:  true,
	})
	if err != nil {
		t.Fatalf("BuildJiraProviderInitReport error: %v", err)
	}
	if report.Command != "jira init" || !report.ReadOnly || report.Project.Key != "ABC" || report.APIBase != "https://jira.example" {
		t.Fatalf("unexpected report metadata: %+v", report)
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
	out := FormatJiraProviderInitReport(report)
	for _, want := range []string{"jira init: dry-run provider discovery", "workflow_mutation: manual_admin_required", "providers.jira.mode: primary"} {
		if !strings.Contains(out, want) {
			t.Fatalf("formatted report missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "secret-token") || strings.Contains(strings.Join(calls, " "), "secret-token") {
		t.Fatalf("credential leaked: calls=%v output=%s", calls, out)
	}
}

func TestBuildJiraProviderInitReportRequiresCredentials(t *testing.T) {
	t.Setenv("JIRA_EMAIL", "")
	t.Setenv("JIRA_API_TOKEN", "")
	_, err := BuildJiraProviderInitReport(JiraProviderInitInput{
		Repo:    ParseRepoRefMust("StatPan/gira"),
		APIBase: "https://jira.example",
		Project: "ABC",
		DryRun:  true,
	})
	if err == nil || !strings.Contains(err.Error(), "JIRA_EMAIL and JIRA_API_TOKEN") {
		t.Fatalf("expected credential error, got %v", err)
	}
}

func TestBuildJiraProviderInitApplyIsDeferred(t *testing.T) {
	_, err := BuildJiraProviderInitReport(JiraProviderInitInput{
		Repo:    ParseRepoRefMust("StatPan/gira"),
		APIBase: "https://jira.example",
		Project: "ABC",
		Email:   "alice@example.com",
		Token:   "secret-token",
		Apply:   true,
	})
	if err == nil || !strings.Contains(err.Error(), "jira init --apply is not supported") {
		t.Fatalf("expected deferred apply error, got %v", err)
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
