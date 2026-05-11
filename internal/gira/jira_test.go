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

func TestFetchJiraAPIItemsRejectsCredentialBearingAPIBase(t *testing.T) {
	restore := jiraAPISearch
	t.Cleanup(func() { jiraAPISearch = restore })
	jiraAPISearch = func(apiBase string, project string, email string, token string, startAt int, maxResults int) ([]byte, error) {
		t.Fatalf("Jira API search should not be called for credential-bearing API base")
		return nil, nil
	}

	_, err := FetchJiraAPIItems("https://alice:secret-token@jira.example", "GIRA", "alice@example.com", "secret-token", nil)
	if err == nil || !strings.Contains(err.Error(), "must not contain credentials") {
		t.Fatalf("expected credential-bearing api-base error, got %v", err)
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
	item := JiraWorkItem{Key: "GIRA-11", Summary: "Create me", Status: "To Do", Priority: "High", Assignee: "alice"}
	labels := JiraGitHubLabels(item)
	runner := &jiraRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --limit 1000 --json number,title,body":                             []byte(`[]`),
		"gh label list --repo StatPan/gira --json name --limit 1000":                                                      []byte(`[{"name":"status:to-do"}]`),
		"gh label create jira:GIRA-11 --repo StatPan/gira --color 5319E7 --description Mirrored Jira issue key.":          []byte(""),
		"gh label create priority:high --repo StatPan/gira --color D93F0B --description Mirrored Jira priority evidence.": []byte(""),
		jiraIssueCreateCommand(repo, item, labels):                                                                        []byte("https://github.com/StatPan/gira/issues/42\n"),
	}}
	report, err := ImportJiraItems(repo, "jira.json", []JiraWorkItem{item}, false, true, runner)
	if err != nil {
		t.Fatalf("ImportJiraItems apply error: %v", err)
	}
	if report.Counts.Create != 1 || report.Counts.Applied != 1 || report.Actions[0].IssueNumber != 42 {
		t.Fatalf("unexpected apply report: %+v", report)
	}
	joinedCalls := strings.Join(runner.calls, "\n")
	if !strings.Contains(joinedCalls, "--label jira:GIRA-11") || !strings.Contains(joinedCalls, "--label status:to-do") || !strings.Contains(joinedCalls, "--label priority:high") {
		t.Fatalf("Jira import apply did not pass computed labels: %v", runner.calls)
	}
}

func TestMirrorJiraIssueDryRunCreatesWhenMissing(t *testing.T) {
	fakeJiraIssueByKey(t, "ABC-123")
	repo := ParseRepoRefMust("StatPan/gira")
	runner := &jiraRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --search ABC-123 in:body --limit 1000 --json number,title,body,url": []byte(`[]`),
	}}

	report, err := MirrorJiraIssue(JiraMirrorInput{
		Repo:    repo,
		Key:     "abc-123",
		APIBase: "https://jira.example",
		Email:   "alice@example.com",
		Token:   "secret-token",
		DryRun:  true,
	}, runner)
	if err != nil {
		t.Fatalf("MirrorJiraIssue dry-run error: %v", err)
	}
	if report.Action != "create" || report.Status != "planned" || report.Item.URL != "https://jira.example/browse/ABC-123" {
		t.Fatalf("unexpected mirror dry-run report: %+v", report)
	}
	if !strings.Contains(report.NextStep, "--api-base https://jira.example") {
		t.Fatalf("next step should preserve api-base: %+v", report)
	}
	for _, want := range []string{"jira:ABC-123", "status:to-do", "priority:high"} {
		if !containsCall(report.Labels, want) {
			t.Fatalf("mirror labels missing %q: %+v", want, report.Labels)
		}
	}
	if len(runner.calls) != 1 {
		t.Fatalf("dry-run should only inspect GitHub mirrors, calls=%v", runner.calls)
	}
}

func TestMirrorJiraIssueApplyCreatesLabeledGitHubIssue(t *testing.T) {
	fakeJiraIssueByKey(t, "ABC-123")
	repo := ParseRepoRefMust("StatPan/gira")
	item := JiraWorkItem{Key: "ABC-123", Summary: "Mirror me", Description: "Body", Status: "To Do", Priority: "High", Assignee: "Alice", Labels: []string{"team-a"}, URL: "https://jira.example/browse/ABC-123"}
	labels := JiraGitHubLabels(item)
	runner := &jiraRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --search ABC-123 in:body --limit 1000 --json number,title,body,url": []byte(`[]`),
		"gh label list --repo StatPan/gira --json name --limit 1000":                                                       []byte(`[{"name":"jira:ABC-123"},{"name":"status:to-do"}]`),
		"gh label create priority:high --repo StatPan/gira --color D93F0B --description Mirrored Jira priority evidence.":  []byte(""),
		"gh label create team-a --repo StatPan/gira --color C5DEF5 --description Mirrored Jira label evidence.":            []byte(""),
		jiraIssueCreateCommand(repo, item, labels):                                                                         []byte("https://github.com/StatPan/gira/issues/77\n"),
	}}

	report, err := MirrorJiraIssue(JiraMirrorInput{
		Repo:    repo,
		Key:     "ABC-123",
		APIBase: "https://jira.example",
		Email:   "alice@example.com",
		Token:   "secret-token",
		Apply:   true,
	}, runner)
	if err != nil {
		t.Fatalf("MirrorJiraIssue apply error: %v", err)
	}
	if report.Action != "create" || report.Status != "applied" || report.Issue.Number != 77 {
		t.Fatalf("unexpected mirror apply report: %+v", report)
	}
	joinedCalls := strings.Join(runner.calls, "\n")
	if !strings.Contains(joinedCalls, "--label jira:ABC-123") || !strings.Contains(joinedCalls, "--label team-a") {
		t.Fatalf("mirror apply did not pass labels: %v", runner.calls)
	}
}

func TestMirrorJiraIssueReusesExistingMirror(t *testing.T) {
	fakeJiraIssueByKey(t, "ABC-123")
	repo := ParseRepoRefMust("StatPan/gira")
	runner := &jiraRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --search ABC-123 in:body --limit 1000 --json number,title,body,url": []byte(`[{"number":12,"title":"Existing mirror","body":"Jira-Key: ABC-123\n","url":"https://github.com/StatPan/gira/issues/12"}]`),
	}}

	report, err := MirrorJiraIssue(JiraMirrorInput{
		Repo:    repo,
		Key:     "ABC-123",
		APIBase: "https://jira.example",
		Email:   "alice@example.com",
		Token:   "secret-token",
		Apply:   true,
	}, runner)
	if err != nil {
		t.Fatalf("MirrorJiraIssue reuse error: %v", err)
	}
	if report.Action != "reuse" || report.Status != "skipped" || report.Issue.Number != 12 {
		t.Fatalf("unexpected reuse report: %+v", report)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("reuse should not create issue, calls=%v", runner.calls)
	}
}

func TestMirrorJiraIssueReportsDuplicateMirrors(t *testing.T) {
	fakeJiraIssueByKey(t, "ABC-123")
	repo := ParseRepoRefMust("StatPan/gira")
	runner := &jiraRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --search ABC-123 in:body --limit 1000 --json number,title,body,url": []byte(`[{"number":12,"title":"One","body":"Jira-Key: ABC-123\n"},{"number":13,"title":"Two","body":"Jira-Key: ABC-123\n"}]`),
	}}

	report, err := MirrorJiraIssue(JiraMirrorInput{
		Repo:    repo,
		Key:     "ABC-123",
		APIBase: "https://jira.example",
		Email:   "alice@example.com",
		Token:   "secret-token",
		DryRun:  true,
	}, runner)
	if err != nil {
		t.Fatalf("MirrorJiraIssue duplicate error: %v", err)
	}
	if report.Action != "conflict" || report.Status != "blocked" || len(report.Duplicates) != 2 {
		t.Fatalf("unexpected duplicate report: %+v", report)
	}
}

func TestResolveJiraMirrorIssue(t *testing.T) {
	repo := ParseRepoRefMust("StatPan/gira")
	runner := &jiraRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --search ABC-123 in:body --limit 1000 --json number,title,body,url": []byte(`[{"number":77,"title":"Mirror","body":"Jira-Key: ABC-123\n","url":"https://github.com/StatPan/gira/issues/77"}]`),
	}}
	issue, err := ResolveJiraMirrorIssue(repo, "abc-123", runner)
	if err != nil {
		t.Fatalf("ResolveJiraMirrorIssue error: %v", err)
	}
	if issue.Number != 77 || issue.Title != "Mirror" {
		t.Fatalf("unexpected mirror issue: %+v", issue)
	}
}

func TestResolveJiraMirrorIssueMissingAndDuplicate(t *testing.T) {
	repo := ParseRepoRefMust("StatPan/gira")
	missingRunner := &jiraRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --search ABC-123 in:body --limit 1000 --json number,title,body,url": []byte(`[]`),
	}}
	if _, err := ResolveJiraMirrorIssue(repo, "ABC-123", missingRunner); err == nil || !strings.Contains(err.Error(), "no GitHub mirror issue found") {
		t.Fatalf("expected missing mirror error, got %v", err)
	}

	duplicateRunner := &jiraRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --search ABC-123 in:body --limit 1000 --json number,title,body,url": []byte(`[{"number":77,"title":"One","body":"Jira-Key: ABC-123\n"},{"number":78,"title":"Two","body":"Jira-Key: ABC-123\n"}]`),
	}}
	if _, err := ResolveJiraMirrorIssue(repo, "ABC-123", duplicateRunner); err == nil || !strings.Contains(err.Error(), "multiple GitHub mirror issues") {
		t.Fatalf("expected duplicate mirror error, got %v", err)
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

func fakeJiraIssueByKey(t *testing.T, key string) {
	t.Helper()
	restore := jiraAPIGet
	t.Cleanup(func() { jiraAPIGet = restore })
	jiraAPIGet = func(apiBase string, path string, query map[string]string, email string, token string) ([]byte, error) {
		if apiBase != "https://jira.example" || path != "/rest/api/3/issue/"+key || query["fields"] == "" || email != "alice@example.com" || token != "secret-token" {
			t.Fatalf("unexpected Jira issue fetch apiBase=%s path=%s query=%v email=%s token=%s", apiBase, path, query, email, token)
		}
		return []byte(`{"key":"` + key + `","fields":{"summary":"Mirror me","description":"Body","status":{"name":"To Do"},"priority":{"name":"High"},"assignee":{"name":"Alice"},"labels":["team-a"]}}`), nil
	}
}

func jiraIssueCreateCommand(repo RepoRef, item JiraWorkItem, labels []string) string {
	args := []string{"issue", "create", "--repo", repo.FullName(), "--title", item.Summary, "--body", JiraIssueBody(item)}
	for _, label := range labels {
		args = append(args, "--label", label)
	}
	return "gh " + strings.Join(args, " ")
}
