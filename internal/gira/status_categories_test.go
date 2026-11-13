package gira

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultStatusCategoryAggregation(t *testing.T) {
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	var issues []normalizedIssue
	for i, label := range []string{"status:ready", "status:in-progress", "status:in-review", "status:blocked", ""} {
		labels := []string{}
		if label != "" {
			labels = append(labels, label)
		}
		issues = append(issues, normalizedIssue{Number: i + 1, State: "open", Labels: labels, UpdatedAt: now.Format(time.RFC3339)})
	}
	issues = append(issues, normalizedIssue{Number: 6, State: "closed", Labels: []string{"status:ready"}})
	summary, err := SummarizeStatus("other/repo", nil, issues, now, 30)
	if err != nil {
		t.Fatal(err)
	}
	want := (CategoryCounts{ToDo: 2, InProgress: 3, Done: 1})
	if summary.Counts.Issues.Categories != want {
		t.Fatalf("categories = %+v, want %+v", summary.Counts.Issues.Categories, want)
	}
	data, err := json.Marshal(summary)
	if err != nil || !strings.Contains(string(data), `"categories":{"to_do":2,"in_progress":3,"done":1}`) {
		t.Fatalf("JSON %s, error %v", data, err)
	}
	if got := statusCategoryOf(defaultWorkflow(), []string{"status:ready", "status:in-review"}, "OPEN"); got != statusCategoryInProgress {
		t.Fatalf("priority category = %s", got)
	}
}

func TestDeclaredCategoryAcrossReports(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Mkdir(filepath.Join(dir, ".gira"), 0755); err != nil {
		t.Fatal(err)
	}
	config := "repo: owner/repo\nworkflow:\n  statuses:\n    - {name: ready-for-release, label: 'status:ready-for-release', category: in_progress}\n  finish_target: ready-for-release\n"
	if err := os.WriteFile(filepath.Join(dir, ".gira", "config.yaml"), []byte(config), 0644); err != nil {
		t.Fatal(err)
	}
	repo := RepoRef{Owner: "owner", Name: "repo"}
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	labels := []string{"status:ready-for-release"}
	issues := []normalizedIssue{{Number: 1, State: "open", Labels: labels, UpdatedAt: now.Format(time.RFC3339)}}
	summary, err := SummarizeStatus(repo.FullName(), nil, issues, now, 30)
	if err != nil {
		t.Fatal(err)
	}
	want := (CategoryCounts{InProgress: 1})
	if summary.Counts.Issues.Categories != want {
		t.Fatalf("status: %+v", summary.Counts.Issues.Categories)
	}
	triage, err := BuildTriageQueue(&fakeTriageClient{repo: repo, issues: []TriageIssue{{Number: 1, State: "open", Labels: labels}}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if triage.Categories != want {
		t.Fatalf("triage: %+v", triage.Categories)
	}
	queues := BuildWorkspaceQueues(WorkspaceSummary{}, []WorkStatusResult{{Repo: repo.FullName(), Issue: 1, State: "open", Labels: labels}})
	if queues.Counts.Categories != want {
		t.Fatalf("queues: %+v", queues.Counts.Categories)
	}
	view := workspaceRepoFromStatus(summary)
	report := WorkspaceReport{}
	addWorkspaceStatusSummary(&report, summary, view)
	if view.Categories != want || report.Counts.Categories != want {
		t.Fatalf("workspace: %+v %+v", view.Categories, report.Counts.Categories)
	}
	client := sprintFakeStatusClient{repo: repo, pages: map[string]string{
		"api repos/owner/repo/milestones --paginate --slurp -X GET -f state=all -f per_page=100": `[[{"number":1,"title":"old","state":"closed","open_issues":1,"closed_issues":0}]]`,
		"api repos/owner/repo/issues --paginate --slurp -X GET -f state=all -f per_page=100":     `[[{"number":1,"title":"work","state":"open","labels":[{"name":"status:ready-for-release"}],"milestone":{"title":"old"},"updated_at":"2026-05-01T00:00:00Z"}]]`,
	}}
	sprint, err := SprintRolloverForClient(client, &sprintFakeRunner{}, "", false, now)
	if err != nil {
		t.Fatal(err)
	}
	if sprint.Summary.Categories != want {
		t.Fatalf("sprint: %+v", sprint.Summary.Categories)
	}
}
