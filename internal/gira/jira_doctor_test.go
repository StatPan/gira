package gira

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildJiraDoctorReportReadyWithSampleTransition(t *testing.T) {
	root := writeJiraDoctorConfig(t, "")
	fakeJiraDoctorAPI(t, "statuses_simple.json", "transitions_done.json", nil)
	runner := &jiraRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --limit 1000 --json number,title,body,url,labels": []byte(`[{"number":77,"title":"Mirror","body":"Jira-Key: ABC-123\n","url":"https://github.com/StatPan/gira/issues/77","labels":[{"name":"jira:ABC-123"}]}]`),
	}}

	report, err := BuildJiraDoctorReport(JiraDoctorInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		ConfigRoot: root,
		SampleKey:  "abc-123",
		Email:      "alice@example.com",
		Token:      "secret-token",
	}, runner)
	if err != nil {
		t.Fatalf("BuildJiraDoctorReport error: %v", err)
	}
	if report.Command != "jira doctor" || report.Status != "ready" || report.Compatibility != "supported" || !report.ReadOnly {
		t.Fatalf("unexpected report metadata: %+v", report)
	}
	if report.Project.Key != "ABC" || len(report.IssueTypes) != 3 || len(report.Statuses) != 4 || report.Mirror.MirrorCount != 1 {
		t.Fatalf("unexpected discovered diagnostics: project=%+v issueTypes=%d statuses=%d mirror=%+v", report.Project, len(report.IssueTypes), len(report.Statuses), report.Mirror)
	}
	if report.Transitions.Status != "ready" || report.Transitions.Candidate.ID != "31" || report.Transitions.SampleKey != "ABC-123" {
		t.Fatalf("unexpected transition diagnostics: %+v", report.Transitions)
	}
	if !hasJiraDoctorCheck(report, "status_map_conflicts", "ready") || !hasJiraDoctorCheck(report, "mirror_issue_health", "ready") {
		t.Fatalf("missing ready checks: %+v", report.Checks)
	}
	var encoded map[string]any
	content, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal doctor report: %v", err)
	}
	if err := json.Unmarshal(content, &encoded); err != nil {
		t.Fatalf("decode doctor report JSON: %v", err)
	}
	if _, ok := encoded["read_only"].(bool); !ok {
		t.Fatalf("doctor JSON should include read_only: %s", content)
	}
	out := FormatJiraDoctorReport(report)
	for _, want := range []string{"jira doctor: ready (supported)", "status_map_conflicts: ready", "transition_sample: ready ABC-123"} {
		if !strings.Contains(out, want) {
			t.Fatalf("formatted doctor report missing %q:\n%s", want, out)
		}
	}
}

func TestBuildJiraDoctorReportBlocksOnDuplicateStatusMapMirrorAndRequiredFields(t *testing.T) {
	root := writeJiraDoctorConfig(t, `
      - gira_status: qa
        jira_statuses:
          - Done
`)
	fakeJiraDoctorAPI(t, "statuses_qa.json", "transitions_required_fields.json", nil)
	runner := &jiraRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --limit 1000 --json number,title,body,url,labels": []byte(`[
			{"number":77,"title":"Mirror one","body":"Jira-Key: ABC-123\n","url":"https://github.com/StatPan/gira/issues/77","labels":[{"name":"jira:ABC-123"}]},
			{"number":78,"title":"Mirror two","body":"Jira-Key: ABC-123\n","url":"https://github.com/StatPan/gira/issues/78","labels":[]}
		]`),
	}}

	report, err := BuildJiraDoctorReport(JiraDoctorInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		ConfigRoot: root,
		SampleKey:  "ABC-123",
		Email:      "alice@example.com",
		Token:      "secret-token",
	}, runner)
	if err != nil {
		t.Fatalf("BuildJiraDoctorReport error: %v", err)
	}
	if report.Status != "blocked" || report.Compatibility != "blocked" {
		t.Fatalf("expected blocked compatibility, got %+v", report)
	}
	for _, want := range []string{"status_map_conflicts", "mirror_issue_health", "transition_reachability"} {
		if !hasJiraDoctorCheck(report, want, "blocked") {
			t.Fatalf("missing blocked check %q: %+v", want, report.Checks)
		}
	}
	if len(report.Mirror.DuplicateKeys) != 1 || report.Mirror.DuplicateKeys[0].Key != "ABC-123" {
		t.Fatalf("expected duplicate mirror diagnostics: %+v", report.Mirror)
	}
	if len(report.Transitions.RequiredFields) != 1 || report.Transitions.RequiredFields[0] != "resolution" {
		t.Fatalf("expected required transition field diagnostic: %+v", report.Transitions)
	}
}

func TestBuildJiraDoctorReportWarnsWhenTransitionSampleMissing(t *testing.T) {
	root := writeJiraDoctorConfig(t, "")
	fakeJiraDoctorAPI(t, "statuses_qa.json", "transitions_done.json", nil)
	runner := &jiraRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --limit 1000 --json number,title,body,url,labels": []byte(`[]`),
	}}

	report, err := BuildJiraDoctorReport(JiraDoctorInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		ConfigRoot: root,
		Email:      "alice@example.com",
		Token:      "secret-token",
	}, runner)
	if err != nil {
		t.Fatalf("BuildJiraDoctorReport error: %v", err)
	}
	if report.Status != "warning" || report.Compatibility != "partially_supported" {
		t.Fatalf("expected warning compatibility, got %+v", report)
	}
	if !hasJiraDoctorCheck(report, "unmapped_statuses", "warning") || !hasJiraDoctorCheck(report, "transition_reachability", "warning") {
		t.Fatalf("expected warning checks: %+v", report.Checks)
	}
	if !strings.Contains(strings.Join(report.NextSteps, "\n"), "--sample-key") {
		t.Fatalf("expected sample-key remediation: %+v", report.NextSteps)
	}
}

func TestBuildJiraDoctorReportWarnsWhenDoneTransitionUnreachable(t *testing.T) {
	root := writeJiraDoctorConfig(t, "")
	fakeJiraDoctorAPI(t, "statuses_simple.json", "transitions_in_progress.json", nil)
	runner := &jiraRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --limit 1000 --json number,title,body,url,labels": []byte(`[{"number":77,"title":"Mirror","body":"Jira-Key: ABC-123\n","url":"https://github.com/StatPan/gira/issues/77","labels":[{"name":"jira:ABC-123"}]}]`),
	}}

	report, err := BuildJiraDoctorReport(JiraDoctorInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		ConfigRoot: root,
		SampleKey:  "ABC-123",
		Email:      "alice@example.com",
		Token:      "secret-token",
	}, runner)
	if err != nil {
		t.Fatalf("BuildJiraDoctorReport error: %v", err)
	}
	if report.Status != "warning" || report.Transitions.Status != "warning" || !strings.Contains(report.Transitions.Detail, "no allowed transition") {
		t.Fatalf("expected unreachable Done transition warning: %+v", report.Transitions)
	}
	if !hasJiraDoctorCheck(report, "transition_reachability", "warning") {
		t.Fatalf("missing transition warning check: %+v", report.Checks)
	}
	if text := FormatJiraDoctorReport(report); !strings.Contains(text, "transition_sample: warning ABC-123") || !strings.Contains(text, "Use Jira admin workflow settings") {
		t.Fatalf("formatted report missing unreachable transition guidance:\n%s", text)
	}
}

func TestBuildJiraDoctorReportWarnsOnMissingMirrorLabels(t *testing.T) {
	root := writeJiraDoctorConfig(t, "")
	fakeJiraDoctorAPI(t, "statuses_simple.json", "transitions_done.json", nil)
	runner := &jiraRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --limit 1000 --json number,title,body,url,labels": []byte(`[{"number":77,"title":"Mirror","body":"Jira-Key: ABC-123\n","url":"https://github.com/StatPan/gira/issues/77","labels":[]}]`),
	}}

	report, err := BuildJiraDoctorReport(JiraDoctorInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		ConfigRoot: root,
		Email:      "alice@example.com",
		Token:      "secret-token",
	}, runner)
	if err != nil {
		t.Fatalf("BuildJiraDoctorReport error: %v", err)
	}
	if report.Status != "warning" || report.Mirror.Status != "warning" || len(report.Mirror.MissingKeyLabels) != 1 {
		t.Fatalf("expected missing mirror label warning: %+v", report.Mirror)
	}
	if report.Mirror.MissingKeyLabels[0].Key != "ABC-123" || report.Mirror.MissingKeyLabels[0].Issue.Number != 77 {
		t.Fatalf("unexpected missing key label diagnostic: %+v", report.Mirror.MissingKeyLabels)
	}
	if !hasJiraDoctorCheck(report, "mirror_issue_health", "warning") {
		t.Fatalf("missing mirror warning check: %+v", report.Checks)
	}
}

func TestBuildJiraDoctorReportBlocksWhenSampleMirrorIsMissing(t *testing.T) {
	root := writeJiraDoctorConfig(t, "")
	fakeJiraDoctorAPI(t, "statuses_simple.json", "transitions_done.json", nil)
	runner := &jiraRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --limit 1000 --json number,title,body,url,labels": []byte(`[{"number":88,"title":"Other mirror","body":"Jira-Key: ABC-999\n","url":"https://github.com/StatPan/gira/issues/88","labels":[{"name":"jira:ABC-999"}]}]`),
	}}

	report, err := BuildJiraDoctorReport(JiraDoctorInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		ConfigRoot: root,
		SampleKey:  "ABC-123",
		Email:      "alice@example.com",
		Token:      "secret-token",
	}, runner)
	if err != nil {
		t.Fatalf("BuildJiraDoctorReport error: %v", err)
	}
	if report.Status != "blocked" || report.Mirror.Status != "blocked" || report.Mirror.SampleKey != "ABC-123" {
		t.Fatalf("expected missing sample mirror to block: %+v", report.Mirror)
	}
	if !strings.Contains(report.Mirror.Detail, "no GitHub mirror") {
		t.Fatalf("missing mirror detail should be actionable: %+v", report.Mirror)
	}
}

func TestBuildJiraDoctorReportReturnsBlockedDiagnosticsForMissingCredentials(t *testing.T) {
	t.Setenv("JIRA_EMAIL", "")
	t.Setenv("JIRA_API_TOKEN", "")
	root := writeJiraDoctorConfig(t, "")
	runner := &jiraRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --limit 1000 --json number,title,body,url,labels": []byte(`[]`),
	}}

	report, err := BuildJiraDoctorReport(JiraDoctorInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		ConfigRoot: root,
	}, runner)
	if err != nil {
		t.Fatalf("BuildJiraDoctorReport error: %v", err)
	}
	if report.Status != "blocked" || !hasJiraDoctorCheck(report, "credentials", "blocked") {
		t.Fatalf("expected blocked credential diagnostics: %+v", report)
	}
}

func TestBuildJiraDoctorReportReflectsCLIOverridesInProviderJSON(t *testing.T) {
	t.Setenv("JIRA_EMAIL", "")
	t.Setenv("JIRA_API_TOKEN", "")
	root := writeJiraDoctorConfigWithProvider(t, "https://old-jira.example", "OLD", "")
	runner := &jiraRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --limit 1000 --json number,title,body,url,labels": []byte(`[]`),
	}}

	report, err := BuildJiraDoctorReport(JiraDoctorInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		APIBase:    "https://jira.example",
		Project:    "ABC",
		ConfigRoot: root,
	}, runner)
	if err != nil {
		t.Fatalf("BuildJiraDoctorReport error: %v", err)
	}
	if report.APIBase != "https://jira.example" || report.ProjectKey != "ABC" || report.Provider.BaseURL != "https://jira.example" || report.Provider.ProjectKey != "ABC" {
		t.Fatalf("provider JSON should reflect CLI overrides: %+v", report)
	}
}

func TestBuildJiraDoctorReportDoesNotFallbackAfterInvalidAPIBaseOverride(t *testing.T) {
	t.Setenv("JIRA_EMAIL", "")
	t.Setenv("JIRA_API_TOKEN", "")
	root := writeJiraDoctorConfig(t, "")
	runner := &jiraRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --limit 1000 --json number,title,body,url,labels": []byte(`[]`),
	}}

	report, err := BuildJiraDoctorReport(JiraDoctorInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		APIBase:    "https://alice:secret-token@jira.example",
		ConfigRoot: root,
	}, runner)
	if err != nil {
		t.Fatalf("BuildJiraDoctorReport error: %v", err)
	}
	if report.Status != "blocked" || report.APIBase != "" || report.Provider.BaseURL != "" || !hasJiraDoctorCheck(report, "api_base", "blocked") {
		t.Fatalf("invalid override should block without falling back to configured API base: %+v", report)
	}
}

func TestBuildJiraDoctorReportConvertsJiraPermissionErrorToBlockedCheck(t *testing.T) {
	root := writeJiraDoctorConfig(t, "")
	fakeJiraDoctorAPI(t, "statuses_simple.json", "transitions_done.json", map[string]error{
		"/rest/api/3/project/ABC": fmt.Errorf("Jira API GET /rest/api/3/project/ABC failed: 403 Forbidden"),
	})
	runner := &jiraRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --limit 1000 --json number,title,body,url,labels": []byte(`[]`),
	}}

	report, err := BuildJiraDoctorReport(JiraDoctorInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		ConfigRoot: root,
		Email:      "alice@example.com",
		Token:      "secret-token",
	}, runner)
	if err != nil {
		t.Fatalf("BuildJiraDoctorReport error: %v", err)
	}
	if report.Status != "blocked" || !hasJiraDoctorCheck(report, "project_reachability", "blocked") {
		t.Fatalf("expected blocked Jira permission diagnostics: %+v", report)
	}
}

func TestBuildJiraDoctorReportConvertsGitHubPermissionErrorToBlockedCheck(t *testing.T) {
	root := writeJiraDoctorConfig(t, "")
	fakeJiraDoctorAPI(t, "statuses_simple.json", "transitions_done.json", nil)
	runner := &jiraRunner{errs: map[string]error{
		"gh issue list --repo StatPan/gira --state all --limit 1000 --json number,title,body,url,labels": fmt.Errorf("gh: 403 Forbidden"),
	}}

	report, err := BuildJiraDoctorReport(JiraDoctorInput{
		Repo:       ParseRepoRefMust("StatPan/gira"),
		ConfigRoot: root,
		Email:      "alice@example.com",
		Token:      "secret-token",
	}, runner)
	if err != nil {
		t.Fatalf("BuildJiraDoctorReport error: %v", err)
	}
	if report.Status != "blocked" || report.Mirror.PermissionProblem == "" || !hasJiraDoctorCheck(report, "mirror_issue_health", "blocked") {
		t.Fatalf("expected blocked GitHub permission diagnostics: %+v", report)
	}
}

func hasJiraDoctorCheck(report JiraDoctorReport, name string, status string) bool {
	for _, check := range report.Checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}

func writeJiraDoctorConfig(t *testing.T, extraStatusMap string) string {
	return writeJiraDoctorConfigWithProvider(t, "https://jira.example", "ABC", extraStatusMap)
}

func writeJiraDoctorConfigWithProvider(t *testing.T, apiBase string, project string, extraStatusMap string) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "repos", "StatPan", "gira.yaml"), `repo: StatPan/gira
providers:
  jira:
    enabled: true
    mode: primary
    base_url: `+apiBase+`
    project_key: `+project+`
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
`+extraStatusMap)
	return root
}

func fakeJiraDoctorAPI(t *testing.T, statusesFixture string, transitionsFixture string, pathErr map[string]error) {
	t.Helper()
	restore := jiraAPIGet
	t.Cleanup(func() { jiraAPIGet = restore })
	jiraAPIGet = func(apiBase string, path string, query map[string]string, email string, token string) ([]byte, error) {
		if apiBase != "https://jira.example" || email != "alice@example.com" || token != "secret-token" {
			t.Fatalf("unexpected Jira API call apiBase=%s path=%s query=%v email=%s token=%s", apiBase, path, query, email, token)
		}
		if err := pathErr[path]; err != nil {
			return nil, err
		}
		switch path {
		case "/rest/api/3/project/ABC":
			return jiraDoctorFixture(t, "project.json"), nil
		case "/rest/api/3/issuetype/project":
			if query["projectId"] != "10000" {
				t.Fatalf("issue type fetch missing project id: %v", query)
			}
			return jiraDoctorFixture(t, "issue_types.json"), nil
		case "/rest/api/3/statuses/search":
			if query["projectId"] != "10000" || query["maxResults"] != "1000" {
				t.Fatalf("status fetch missing query: %v", query)
			}
			return jiraDoctorFixture(t, statusesFixture), nil
		case "/rest/api/3/priority/search":
			if query["projectId"] != "10000" || query["maxResults"] != "1000" {
				t.Fatalf("priority fetch missing query: %v", query)
			}
			return jiraDoctorFixture(t, "priorities.json"), nil
		case "/rest/api/3/issue/ABC-123":
			if query["fields"] == "" {
				t.Fatalf("issue fetch missing fields query: %v", query)
			}
			return jiraDoctorFixture(t, "issue_abc_123.json"), nil
		case "/rest/api/3/issue/ABC-123/transitions":
			if query["expand"] != "transitions.fields" {
				t.Fatalf("transition fetch missing expand query: %v", query)
			}
			return jiraDoctorFixture(t, transitionsFixture), nil
		default:
			return nil, fmt.Errorf("unexpected path %s", path)
		}
	}
}

func jiraDoctorFixture(t *testing.T, name string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", "jira_doctor", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return content
}
