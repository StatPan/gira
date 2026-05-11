package gira

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildJiraTransitionPlanDirectTransition(t *testing.T) {
	root := writeJiraTransitionConfig(t)
	fakeJiraTransitionAPI(t, "ABC-123", `{"transitions":[{"id":"21","name":"Start Progress","to":{"name":"In Progress"},"fields":{}}]}`)

	report, err := BuildJiraTransitionPlan(JiraTransitionPlanInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		Key:        "abc-123",
		Target:     "in_progress",
		ConfigRoot: root,
		Email:      "alice@example.com",
		Token:      "secret-token",
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("BuildJiraTransitionPlan error: %v", err)
	}
	if report.Decision != "direct_transition" || report.CurrentStatus != "To Do" || report.Candidate.ID != "21" || report.Candidate.ToStatus != "In Progress" {
		t.Fatalf("unexpected direct transition report: %+v", report)
	}
	if len(report.TargetStatuses) != 1 || report.TargetStatuses[0] != "In Progress" {
		t.Fatalf("target statuses = %+v, want In Progress", report.TargetStatuses)
	}
	if !report.DryRun || !report.ReadOnly {
		t.Fatalf("transition planner must be dry-run/read-only: %+v", report)
	}
	if text := FormatJiraTransitionPlan(report); !strings.Contains(text, "candidate: 21 Start Progress -> In Progress") {
		t.Fatalf("formatted report missing candidate:\n%s", text)
	}
}

func TestBuildJiraTransitionPlanMissingTransition(t *testing.T) {
	root := writeJiraTransitionConfig(t)
	fakeJiraTransitionAPI(t, "ABC-123", `{"transitions":[{"id":"21","name":"Start Progress","to":{"name":"In Progress"},"fields":{}}]}`)

	report, err := BuildJiraTransitionPlan(JiraTransitionPlanInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		Key:        "ABC-123",
		Target:     "done",
		ConfigRoot: root,
		Email:      "alice@example.com",
		Token:      "secret-token",
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("BuildJiraTransitionPlan error: %v", err)
	}
	if report.Decision != "missing_transition" || !strings.Contains(report.Reason, "no allowed transition") {
		t.Fatalf("unexpected missing transition report: %+v", report)
	}
}

func TestBuildJiraTransitionPlanRequiredFieldNeedsManualAdmin(t *testing.T) {
	root := writeJiraTransitionConfig(t)
	fakeJiraTransitionAPI(t, "ABC-123", `{"transitions":[{"id":"31","name":"Resolve","to":{"name":"Done"},"fields":{"resolution":{"required":true},"comment":{"required":false}}}]}`)

	report, err := BuildJiraTransitionPlan(JiraTransitionPlanInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		Key:        "ABC-123",
		Target:     "done",
		ConfigRoot: root,
		Email:      "alice@example.com",
		Token:      "secret-token",
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("BuildJiraTransitionPlan error: %v", err)
	}
	if report.Decision != "manual_admin_required" || len(report.Candidate.RequiredFields) != 1 || report.Candidate.RequiredFields[0] != "resolution" {
		t.Fatalf("unexpected required-field report: %+v", report)
	}
}

func TestBuildJiraTransitionPlanPrefersCandidateWithoutRequiredFields(t *testing.T) {
	root := writeJiraTransitionConfig(t)
	fakeJiraTransitionAPI(t, "ABC-123", `{"transitions":[{"id":"31","name":"Resolve With Fields","to":{"name":"Done"},"fields":{"resolution":{"required":true}}},{"id":"32","name":"Done","to":{"name":"Done"},"fields":{}}]}`)

	report, err := BuildJiraTransitionPlan(JiraTransitionPlanInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		Key:        "ABC-123",
		Target:     "done",
		ConfigRoot: root,
		Email:      "alice@example.com",
		Token:      "secret-token",
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("BuildJiraTransitionPlan error: %v", err)
	}
	if report.Decision != "direct_transition" || report.Candidate.ID != "32" {
		t.Fatalf("planner should prefer candidate without required fields: %+v", report)
	}
}

func TestBuildJiraTransitionPlanUnmappedStatus(t *testing.T) {
	root := writeJiraTransitionConfig(t)
	fakeJiraTransitionAPI(t, "ABC-123", `{"transitions":[{"id":"21","name":"Start Progress","to":{"name":"In Progress"},"fields":{}}]}`)

	report, err := BuildJiraTransitionPlan(JiraTransitionPlanInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		Key:        "ABC-123",
		Target:     "blocked",
		ConfigRoot: root,
		Email:      "alice@example.com",
		Token:      "secret-token",
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("BuildJiraTransitionPlan error: %v", err)
	}
	if report.Decision != "unmapped_status" || !strings.Contains(report.Reason, "status_map") {
		t.Fatalf("unexpected unmapped status report: %+v", report)
	}
}

func TestBuildJiraTransitionPlanNoAllowedTransitionsNeedsManualAdmin(t *testing.T) {
	root := writeJiraTransitionConfig(t)
	fakeJiraTransitionAPI(t, "ABC-123", `{"transitions":[]}`)

	report, err := BuildJiraTransitionPlan(JiraTransitionPlanInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		Key:        "ABC-123",
		Target:     "done",
		ConfigRoot: root,
		Email:      "alice@example.com",
		Token:      "secret-token",
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("BuildJiraTransitionPlan error: %v", err)
	}
	if report.Decision != "manual_admin_required" || !strings.Contains(report.Reason, "no allowed transitions") {
		t.Fatalf("unexpected no-transition report: %+v", report)
	}
}

func TestBuildJiraTransitionPlanPermissionLimitationNeedsManualAdmin(t *testing.T) {
	root := writeJiraTransitionConfig(t)
	fakeJiraTransitionAPIError(t, "ABC-123", fmt.Errorf("Jira API GET /rest/api/3/issue/ABC-123/transitions failed: 403 Forbidden"))

	report, err := BuildJiraTransitionPlan(JiraTransitionPlanInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		Key:        "ABC-123",
		Target:     "done",
		ConfigRoot: root,
		Email:      "alice@example.com",
		Token:      "secret-token",
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("BuildJiraTransitionPlan error: %v", err)
	}
	if report.Decision != "manual_admin_required" || !strings.Contains(report.Reason, "403 Forbidden") {
		t.Fatalf("unexpected permission limitation report: %+v", report)
	}
}

func TestBuildJiraTransitionPlanRequiresDryRun(t *testing.T) {
	_, err := BuildJiraTransitionPlan(JiraTransitionPlanInput{
		Repo:   ParseRepoRefMust("StatPan/gira"),
		Key:    "ABC-123",
		Target: "done",
	})
	if err == nil || !strings.Contains(err.Error(), "only supports --dry-run") {
		t.Fatalf("expected dry-run-only error, got %v", err)
	}
}

func fakeJiraTransitionAPIError(t *testing.T, key string, transitionErr error) {
	t.Helper()
	restore := jiraAPIGet
	t.Cleanup(func() { jiraAPIGet = restore })
	jiraAPIGet = func(apiBase string, path string, query map[string]string, email string, token string) ([]byte, error) {
		if apiBase != "https://jira.example" || email != "alice@example.com" || token != "secret-token" {
			t.Fatalf("unexpected Jira API call apiBase=%s path=%s query=%v email=%s token=%s", apiBase, path, query, email, token)
		}
		switch path {
		case "/rest/api/3/issue/" + key:
			return []byte(fmt.Sprintf(`{"key":"%s","fields":{"summary":"Plan transition","status":{"name":"To Do"}}}`, key)), nil
		case "/rest/api/3/issue/" + key + "/transitions":
			return nil, transitionErr
		default:
			t.Fatalf("unexpected Jira API path: %s", path)
			return nil, nil
		}
	}
}

func writeJiraTransitionConfig(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "repos", "StatPan", "gira.yaml"), `repo: StatPan/gira
providers:
  jira:
    enabled: true
    mode: primary
    base_url: https://jira.example
    project_key: ABC
    source_of_truth:
      planning: jira
      status: jira
      execution: github
    status_map:
      - gira_status: ready
        jira_statuses:
          - To Do
      - gira_status: in_progress
        jira_statuses:
          - In Progress
      - gira_status: review
        jira_statuses:
          - Code Review
      - gira_status: done
        jira_statuses:
          - Done
`)
	return root
}

func fakeJiraTransitionAPI(t *testing.T, key string, transitions string) {
	t.Helper()
	restore := jiraAPIGet
	t.Cleanup(func() { jiraAPIGet = restore })
	jiraAPIGet = func(apiBase string, path string, query map[string]string, email string, token string) ([]byte, error) {
		if apiBase != "https://jira.example" || email != "alice@example.com" || token != "secret-token" {
			t.Fatalf("unexpected Jira API call apiBase=%s path=%s query=%v email=%s token=%s", apiBase, path, query, email, token)
		}
		switch path {
		case "/rest/api/3/issue/" + key:
			if query["fields"] == "" {
				t.Fatalf("issue fetch missing fields query: %v", query)
			}
			return []byte(fmt.Sprintf(`{"key":"%s","fields":{"summary":"Plan transition","status":{"name":"To Do"}}}`, key)), nil
		case "/rest/api/3/issue/" + key + "/transitions":
			if query["expand"] != "transitions.fields" {
				t.Fatalf("transition fetch missing expand query: %v", query)
			}
			return []byte(transitions), nil
		default:
			t.Fatalf("unexpected Jira API path: %s", path)
			return nil, nil
		}
	}
}
