package gira

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type jiraRunner struct {
	outputs map[string][]byte
	errs    map[string]error
	calls   []string
}

func (r *jiraRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, key)
	if err, ok := r.errs[key]; ok {
		return nil, err
	}
	if out, ok := r.outputs[key]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected call: %s", key)
}

func TestParseJiraImportCSVMapsFieldsAndLabels(t *testing.T) {
	items, err := ParseJiraImportCSV([]byte("key,summary,description,status,priority,assignee,labels\nGIRA-7,Add importer,Body,In Progress,High,alice,\"backend, migration\"\n"))
	if err != nil {
		t.Fatalf("ParseJiraImportCSV error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	item := items[0]
	if item.Key != "GIRA-7" || item.Summary != "Add importer" || item.Description != "Body" || item.Status != "In Progress" || item.Priority != "High" || item.Assignee != "alice" {
		t.Fatalf("unexpected item: %+v", item)
	}
	labels := JiraGitHubLabels(item)
	for _, want := range []string{"jira:GIRA-7", "status:in-progress", "priority:high", "backend", "migration"} {
		if !containsCall(labels, want) {
			t.Fatalf("labels missing %q: %v", want, labels)
		}
	}
	if body := JiraIssueBody(item); !strings.Contains(body, "Jira-Key: GIRA-7") || !strings.Contains(body, "Jira-Status: In Progress") {
		t.Fatalf("body missing Jira metadata:\n%s", body)
	}
}

func TestImportJiraItemsDetectsExistingAndSourceDuplicates(t *testing.T) {
	repo := ParseRepoRefMust("StatPan/gira")
	runner := &jiraRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --limit 1000 --json number,title,body": []byte(`[{"number":4,"title":"Existing","body":"Jira-Key: GIRA-1\n"}]`),
	}}
	report, err := ImportJiraItems(repo, "jira.csv", []JiraWorkItem{
		{Key: "GIRA-1", Summary: "Existing"},
		{Key: "GIRA-2", Summary: "New"},
		{Key: "GIRA-2", Summary: "New again"},
	}, true, false, runner)
	if err != nil {
		t.Fatalf("ImportJiraItems error: %v", err)
	}
	if report.Counts.Create != 1 || report.Counts.Duplicate != 2 || report.Counts.Applied != 0 {
		t.Fatalf("counts = %+v, want create=1 duplicate=2 applied=0", report.Counts)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("dry-run should only list existing issues, calls=%v", runner.calls)
	}
}

func TestFetchJiraAPIItemsUsesReadOnlyHTTPTransport(t *testing.T) {
	t.Setenv("JIRA_EMAIL", "alice@example.com")
	t.Setenv("JIRA_API_TOKEN", "token")
	restore := jiraAPISearch
	t.Cleanup(func() { jiraAPISearch = restore })
	var calls []string
	jiraAPISearch = func(apiBase string, project string, email string, token string, startAt int, maxResults int) ([]byte, error) {
		calls = append(calls, fmt.Sprintf("%s %s %s %d %d", apiBase, project, email, startAt, maxResults))
		if token != "token" {
			t.Fatalf("unexpected token passed to HTTP transport")
		}
		return []byte(`{"startAt":0,"maxResults":100,"total":1,"issues":[{"key":"GIRA-9","fields":{"summary":"API item","description":"Body","status":{"name":"To Do"},"priority":{"name":"Medium"},"assignee":{"name":"alice"},"labels":["api"]}}]}`), nil
	}
	runner := &jiraRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --limit 1000 --json number,title,body": []byte(`[]`),
	}}
	report, err := ImportJiraFromAPI(ParseRepoRefMust("StatPan/gira"), "https://jira.example", "GIRA", true, false, runner)
	if err != nil {
		t.Fatalf("ImportJiraFromAPI error: %v", err)
	}
	if report.APIBase != "https://jira.example" || report.Project != "GIRA" || report.Counts.Create != 1 {
		t.Fatalf("unexpected API report: %+v", report)
	}
	if len(calls) != 1 || calls[0] != "https://jira.example GIRA alice@example.com 0 100" {
		t.Fatalf("unexpected Jira API calls: %v", calls)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "token") {
			t.Fatalf("token leaked through command runner call: %v", runner.calls)
		}
	}
}

func TestFetchJiraAPIItemsPaginates(t *testing.T) {
	restore := jiraAPISearch
	t.Cleanup(func() { jiraAPISearch = restore })
	var starts []int
	jiraAPISearch = func(apiBase string, project string, email string, token string, startAt int, maxResults int) ([]byte, error) {
		starts = append(starts, startAt)
		switch startAt {
		case 0:
			return []byte(`{"startAt":0,"maxResults":1,"total":2,"issues":[{"key":"GIRA-1","fields":{"summary":"One"}}]}`), nil
		case 1:
			return []byte(`{"startAt":1,"maxResults":1,"total":2,"issues":[{"key":"GIRA-2","fields":{"summary":"Two"}}]}`), nil
		default:
			return nil, fmt.Errorf("unexpected startAt %d", startAt)
		}
	}
	items, err := FetchJiraAPIItems("https://jira.example", "GIRA", "alice@example.com", "token", nil)
	if err != nil {
		t.Fatalf("FetchJiraAPIItems error: %v", err)
	}
	if len(items) != 2 || len(starts) != 2 || starts[0] != 0 || starts[1] != 1 {
		t.Fatalf("items=%+v starts=%v, want two paged items", items, starts)
	}
}

func TestImportJiraItemsApplyCreatesGitHubIssue(t *testing.T) {
	repo := ParseRepoRefMust("StatPan/gira")
	runner := &jiraRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --limit 1000 --json number,title,body":                                                                                    []byte(`[]`),
		"gh issue create --repo StatPan/gira --title Create me --body Imported from Jira.\n\nJira-Key: GIRA-11\nJira-Status: To Do\nJira-Priority: High\nJira-Assignee: alice\n": []byte("https://github.com/StatPan/gira/issues/42\n"),
	}}
	report, err := ImportJiraItems(repo, "jira.json", []JiraWorkItem{{Key: "GIRA-11", Summary: "Create me", Status: "To Do", Priority: "High", Assignee: "alice"}}, false, true, runner)
	if err != nil {
		t.Fatalf("ImportJiraItems apply error: %v", err)
	}
	if report.Counts.Create != 1 || report.Counts.Applied != 1 || report.Actions[0].IssueNumber != 42 {
		t.Fatalf("unexpected apply report: %+v", report)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "--label") || strings.Contains(call, "--assignee") {
			t.Fatalf("Jira import apply should not require pre-existing GitHub labels or assignee mapping, calls=%v", runner.calls)
		}
	}
}

func TestExportJiraIssuesWritesStableJSONAndCSV(t *testing.T) {
	repo := ParseRepoRefMust("StatPan/gira")
	runner := &jiraRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --limit 1000 --json number,title,body,state,labels,assignees,url": []byte(`[{"number":2,"title":"Second","body":"Jira-Key: GIRA-2\n","state":"OPEN","url":"https://github.com/StatPan/gira/issues/2","labels":[{"name":"priority:high"},{"name":"status:ready"}],"assignees":[{"login":"alice"}]},{"number":1,"title":"First","body":"","state":"CLOSED","url":"https://github.com/StatPan/gira/issues/1","labels":[],"assignees":[]}]`),
	}}
	outputRoot := filepath.Join(t.TempDir(), "jira-export")
	report, err := ExportJiraIssues(repo, outputRoot, runner)
	if err != nil {
		t.Fatalf("ExportJiraIssues error: %v", err)
	}
	if report.Counts.Issues != 2 {
		t.Fatalf("issues = %d, want 2", report.Counts.Issues)
	}
	jsonContent := readFile(t, filepath.Join(outputRoot, "issues.json"))
	if !strings.Contains(jsonContent, `"number": 1`) || !strings.Contains(jsonContent, `"jira_key": "GIRA-2"`) {
		t.Fatalf("unexpected JSON artifact:\n%s", jsonContent)
	}
	csvContent := readFile(t, filepath.Join(outputRoot, "issues.csv"))
	if !strings.HasPrefix(csvContent, "number,title,body,state,status,priority,assignee,labels,jira_key,url\n") {
		t.Fatalf("unexpected CSV header:\n%s", csvContent)
	}
	if strings.Index(csvContent, "1,First") > strings.Index(csvContent, "2,Second") {
		t.Fatalf("CSV is not sorted by issue number:\n%s", csvContent)
	}
	if _, err := os.Stat(filepath.Join(outputRoot, "issues.json")); err != nil {
		t.Fatal(err)
	}
}
