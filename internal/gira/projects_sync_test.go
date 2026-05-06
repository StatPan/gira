package gira

import (
	"strings"
	"testing"
	"time"
)

func TestBuildProjectsSyncReportPlansMissingItemsAndStatusUpdates(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
		Project:   ProjectConfig{Owner: "StatPan", Number: 7, Title: "Gira"},
	}
	client := &fakeProjectsSyncClient{
		project:     ProjectsSyncProject{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"},
		statusField: ProjectsSyncStatusField{ID: "status-field", Options: map[string]string{"Todo": "todo", "In Progress": "progress", "Done": "done"}},
		linked:      map[string]bool{"StatPan/gira": false},
		issues: map[string][]ProjectsSyncIssue{
			"StatPan/gira": {
				{Repo: "StatPan/gira", Number: 180, Title: "Ready", URL: "https://github.com/StatPan/gira/issues/180", Labels: []string{"status:ready"}},
				{Repo: "StatPan/gira", Number: 181, Title: "Started", URL: "https://github.com/StatPan/gira/issues/181", Labels: []string{"status:in-progress"}},
			},
		},
		items: []ProjectsSyncItem{{ID: "item-181", Repo: "StatPan/gira", Number: 181, Status: "Todo"}},
	}

	report, err := BuildProjectsSyncReport(config, client, true, time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildProjectsSyncReport error: %v", err)
	}
	if report.Counts.ProjectLinksAdd != 1 || report.Counts.ProjectItemsAdd != 1 || report.Counts.ProjectItemsSkip != 1 || report.Counts.StatusUpdates != 1 {
		t.Fatalf("counts = %+v", report.Counts)
	}
	for _, want := range []string{"project_repo:link", "project_item:add", "project_status:update"} {
		if !strings.Contains(FormatProjectsSyncReport(report), want) {
			t.Fatalf("formatted report missing %q:\n%s", want, FormatProjectsSyncReport(report))
		}
	}
	if len(client.linkedApplied) != 0 || len(client.added) != 0 || len(client.updated) != 0 {
		t.Fatalf("dry-run mutated fake client: %+v", client)
	}
}

func TestBuildProjectsSyncReportApplyIsIdempotentForExistingItems(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
		Project:   ProjectConfig{Owner: "StatPan", Number: 7, Title: "Gira"},
	}
	client := &fakeProjectsSyncClient{
		project:     ProjectsSyncProject{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"},
		statusField: ProjectsSyncStatusField{ID: "status-field", Options: map[string]string{"Todo": "todo", "In Progress": "progress", "Done": "done"}},
		linked:      map[string]bool{"StatPan/gira": true},
		issues: map[string][]ProjectsSyncIssue{
			"StatPan/gira": {{Repo: "StatPan/gira", Number: 180, Title: "Ready", URL: "https://github.com/StatPan/gira/issues/180", Labels: []string{"status:ready"}}},
		},
		items: []ProjectsSyncItem{{ID: "item-180", Repo: "StatPan/gira", Number: 180, Status: "Todo"}},
	}

	report, err := BuildProjectsSyncReport(config, client, false, time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildProjectsSyncReport error: %v", err)
	}
	if len(report.Actions) != 0 || report.Counts.ProjectItemsSkip != 1 {
		t.Fatalf("report = %+v", report)
	}
	if len(client.linkedApplied) != 0 || len(client.added) != 0 || len(client.updated) != 0 {
		t.Fatalf("apply should not mutate existing synced item: %+v", client)
	}
}

func TestBuildProjectsSyncReportApplyAddsAndUpdates(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
		Project:   ProjectConfig{Owner: "StatPan", Number: 7, Title: "Gira"},
	}
	client := &fakeProjectsSyncClient{
		project:     ProjectsSyncProject{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"},
		statusField: ProjectsSyncStatusField{ID: "status-field", Options: map[string]string{"Todo": "todo", "In Progress": "progress", "Done": "done"}},
		linked:      map[string]bool{"StatPan/gira": false},
		issues: map[string][]ProjectsSyncIssue{
			"StatPan/gira": {{Repo: "StatPan/gira", Number: 181, Title: "Started", URL: "https://github.com/StatPan/gira/issues/181", Labels: []string{"status:in-progress"}}},
		},
	}

	report, err := BuildProjectsSyncReport(config, client, false, time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildProjectsSyncReport error: %v", err)
	}
	if report.Counts.ProjectLinksAdd != 1 || report.Counts.ProjectItemsAdd != 1 || report.Counts.StatusUpdates != 1 {
		t.Fatalf("counts = %+v", report.Counts)
	}
	if len(client.linkedApplied) != 1 || len(client.added) != 1 || len(client.updated) != 1 {
		t.Fatalf("apply did not call expected mutations: %+v", client)
	}
}

type fakeProjectsSyncClient struct {
	project       ProjectsSyncProject
	statusField   ProjectsSyncStatusField
	linked        map[string]bool
	issues        map[string][]ProjectsSyncIssue
	items         []ProjectsSyncItem
	linkedApplied []string
	added         []ProjectsSyncIssue
	updated       []string
}

func (c *fakeProjectsSyncClient) Project(owner string, number int) (ProjectsSyncProject, error) {
	return c.project, nil
}

func (c *fakeProjectsSyncClient) StatusField(owner string, number int) (ProjectsSyncStatusField, error) {
	return c.statusField, nil
}

func (c *fakeProjectsSyncClient) RepoLinked(owner string, number int, repo RepoRef) (bool, error) {
	return c.linked[repo.FullName()], nil
}

func (c *fakeProjectsSyncClient) OpenIssues(repo RepoRef) ([]ProjectsSyncIssue, error) {
	return c.issues[repo.FullName()], nil
}

func (c *fakeProjectsSyncClient) ProjectItems(owner string, number int) ([]ProjectsSyncItem, error) {
	return c.items, nil
}

func (c *fakeProjectsSyncClient) LinkRepo(owner string, number int, repo RepoRef) error {
	c.linkedApplied = append(c.linkedApplied, repo.FullName())
	return nil
}

func (c *fakeProjectsSyncClient) AddItem(owner string, number int, issue ProjectsSyncIssue) (string, error) {
	c.added = append(c.added, issue)
	return "item-added", nil
}

func (c *fakeProjectsSyncClient) UpdateItemStatus(projectID string, itemID string, fieldID string, optionID string) error {
	c.updated = append(c.updated, itemID+":"+optionID)
	return nil
}
