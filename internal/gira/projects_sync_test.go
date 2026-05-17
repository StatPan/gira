package gira

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBuildProjectsSyncReportPlansMissingItemsAndStatusUpdates(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
		Project:   ProjectConfig{Owner: "StatPan", Title: "Gira"},
	}
	client := &fakeProjectsSyncClient{
		project:     ProjectsSyncProject{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"},
		projects:    []ProjectsSyncProject{{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"}},
		fields:      allProjectsSyncCanonicalFields(),
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
	if report.Counts.ProjectItemsSkipReasons.AlreadyPresent != 1 {
		t.Fatalf("skip reasons = %+v", report.Counts.ProjectItemsSkipReasons)
	}
	for _, want := range []string{"project_repo:link", "project_item:add", "project_status:update", "already_present=1"} {
		if !strings.Contains(FormatProjectsSyncReport(report), want) {
			t.Fatalf("formatted report missing %q:\n%s", want, FormatProjectsSyncReport(report))
		}
	}
	if len(client.linkedApplied) != 0 || len(client.added) != 0 || len(client.updated) != 0 {
		t.Fatalf("dry-run mutated fake client: %+v", client)
	}
	if client.callCount("StatusField") != 0 {
		t.Fatalf("StatusField should be derived from ProjectFields, calls=%d", client.callCount("StatusField"))
	}
}

func TestBuildProjectsSyncReportFetchesIndependentReadsConcurrently(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
		Project:   ProjectConfig{Owner: "StatPan", Title: "Gira"},
	}
	client := &fakeProjectsSyncClient{
		project:     ProjectsSyncProject{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"},
		projects:    []ProjectsSyncProject{{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"}},
		fields:      allProjectsSyncCanonicalFields(),
		statusField: ProjectsSyncStatusField{ID: "status-field", Options: map[string]string{"Todo": "todo", "In Progress": "progress", "Done": "done"}},
		linked:      map[string]bool{"StatPan/gira": true},
		issues: map[string][]ProjectsSyncIssue{
			"StatPan/gira": {{Repo: "StatPan/gira", Number: 180, Title: "Ready", URL: "https://github.com/StatPan/gira/issues/180", Labels: []string{"status:ready"}}},
		},
		items: []ProjectsSyncItem{{ID: "item-180", Repo: "StatPan/gira", Number: 180, Status: "Todo"}},
		delays: map[string]time.Duration{
			"ProjectFields":       80 * time.Millisecond,
			"ProjectItemsGraphQL": 80 * time.Millisecond,
			"RepoLinked":          80 * time.Millisecond,
			"OpenIssues":          80 * time.Millisecond,
		},
	}

	start := time.Now()
	report, err := BuildProjectsSyncReport(config, client, true, time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC))
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("BuildProjectsSyncReport error: %v", err)
	}
	if report.Counts.Issues != 1 || report.Counts.ProjectItemsSkip != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Counts.ProjectItemsSkipReasons.AlreadyPresent != 1 {
		t.Fatalf("skip reasons = %+v", report.Counts.ProjectItemsSkipReasons)
	}
	if elapsed > 260*time.Millisecond {
		t.Fatalf("BuildProjectsSyncReport took %s, want independent reads under 260ms", elapsed)
	}
	if client.callCount("ProjectFields") != 1 || client.callCount("ProjectItemsGraphQL") != 1 || client.callCount("RepoLinked") != 1 || client.callCount("OpenIssues") != 1 {
		t.Fatalf("unexpected read call counts: %+v", client.calls)
	}
}

func TestBuildProjectsSyncReportApplyIsIdempotentForExistingItems(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
		Project:   ProjectConfig{Owner: "StatPan", Title: "Gira"},
	}
	client := &fakeProjectsSyncClient{
		project:     ProjectsSyncProject{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"},
		projects:    []ProjectsSyncProject{{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"}},
		fields:      allProjectsSyncCanonicalFields(),
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
	if report.Counts.ProjectItemsSkipReasons.AlreadyPresent != 1 {
		t.Fatalf("skip reasons = %+v", report.Counts.ProjectItemsSkipReasons)
	}
	if !report.ManualActionRequired || len(report.ManualActions) != 1 || !report.Counts.ViewSetupRequired {
		t.Fatalf("manual view setup should be machine-readable: %+v", report)
	}
	if !strings.Contains(FormatProjectsSyncReport(report), "data sync: complete; manual action required") {
		t.Fatalf("formatted report should separate sync completion from manual view setup:\n%s", FormatProjectsSyncReport(report))
	}
	if len(client.linkedApplied) != 0 || len(client.added) != 0 || len(client.updated) != 0 {
		t.Fatalf("apply should not mutate existing synced item: %+v", client)
	}
}

func TestBuildProjectsSyncReportBreaksDownProjectItemSkipReasons(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
		Project:   ProjectConfig{Owner: "StatPan", Title: "Gira"},
	}
	client := &fakeProjectsSyncClient{
		project:     ProjectsSyncProject{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"},
		projects:    []ProjectsSyncProject{{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"}},
		fields:      allProjectsSyncCanonicalFields(),
		statusField: ProjectsSyncStatusField{ID: "status-field", Options: map[string]string{"Todo": "todo", "Done": "done"}},
		linked:      map[string]bool{"StatPan/gira": true},
		issues: map[string][]ProjectsSyncIssue{
			"StatPan/gira": {{Repo: "StatPan/gira", Number: 180, Title: "Ready", URL: "https://github.com/StatPan/gira/issues/180", Labels: []string{"status:ready"}}},
		},
		items: []ProjectsSyncItem{
			{ID: "item-180", Repo: "StatPan/gira", Number: 180, Status: "Todo", IssueState: "open"},
			{ID: "item-180-duplicate", Repo: "StatPan/gira", Number: 180, Status: "Todo", IssueState: "open"},
			{ID: "item-closed", Repo: "StatPan/gira", Number: 199, Status: "Done", IssueState: "closed"},
			{ID: "item-draft"},
		},
	}

	report, err := BuildProjectsSyncReport(config, client, true, time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildProjectsSyncReport error: %v", err)
	}
	reasons := report.Counts.ProjectItemsSkipReasons
	if report.Counts.ProjectItemsSkip != 4 || reasons.AlreadyPresent != 1 || reasons.DuplicateCandidate != 1 || reasons.ClosedDone != 1 || reasons.UnsupportedItemShape != 1 {
		t.Fatalf("skip counts=%+v reasons=%+v", report.Counts, reasons)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if !strings.Contains(string(encoded), `"project_items_skip_reasons"`) || !strings.Contains(string(encoded), `"duplicate_candidate":1`) {
		t.Fatalf("JSON output missing skip reasons:\n%s", encoded)
	}
	text := FormatProjectsSyncReport(report)
	for _, want := range []string{"project-items-skip: total=4", "already_present=1", "closed_done=1", "duplicate_candidate=1", "unsupported_item_shape=1"} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted output missing %q:\n%s", want, text)
		}
	}
}

func TestBuildProjectsSyncReportManualViewOnlyNextStepSkipsApply(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
		Project:   ProjectConfig{Owner: "StatPan", Title: "Gira"},
	}
	client := &fakeProjectsSyncClient{
		project:     ProjectsSyncProject{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"},
		projects:    []ProjectsSyncProject{{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"}},
		fields:      allProjectsSyncCanonicalFields(),
		statusField: ProjectsSyncStatusField{ID: "status-field", Options: map[string]string{"Todo": "todo", "In Progress": "progress", "Done": "done"}},
		linked:      map[string]bool{"StatPan/gira": true},
		issues: map[string][]ProjectsSyncIssue{
			"StatPan/gira": {{Repo: "StatPan/gira", Number: 180, Title: "Ready", URL: "https://github.com/StatPan/gira/issues/180", Labels: []string{"status:ready"}}},
		},
		items: []ProjectsSyncItem{{ID: "item-180", Repo: "StatPan/gira", Number: 180, Status: "Todo"}},
	}

	report, err := BuildProjectsSyncReport(config, client, true, time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildProjectsSyncReport error: %v", err)
	}
	if len(report.Actions) != 0 || len(report.NextSteps) != 1 || strings.Contains(report.NextSteps[0], "projects sync") {
		t.Fatalf("manual-only sync should not ask for another apply: %+v", report)
	}
	if !report.ManualActionRequired || len(report.ManualActions) != 1 {
		t.Fatalf("manual-only sync should expose manual action fields: %+v", report)
	}
}

func TestBuildProjectsSyncReportApplyAddsAndUpdates(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
		Project:   ProjectConfig{Owner: "StatPan", Title: "Gira"},
	}
	client := &fakeProjectsSyncClient{
		project:     ProjectsSyncProject{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"},
		projects:    []ProjectsSyncProject{{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"}},
		fields:      allProjectsSyncCanonicalFields(),
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

func TestBuildProjectsSyncReportInfersSingleLinkedProject(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
	}
	client := &fakeProjectsSyncClient{
		project:        ProjectsSyncProject{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"},
		fields:         allProjectsSyncCanonicalFields(),
		statusField:    ProjectsSyncStatusField{ID: "status-field", Options: map[string]string{"Todo": "todo"}},
		linked:         map[string]bool{"StatPan/gira": true},
		linkedProjects: map[string][]ProjectsSyncProject{"StatPan/gira": []ProjectsSyncProject{{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"}}},
		issues:         map[string][]ProjectsSyncIssue{"StatPan/gira": {}},
	}

	report, err := BuildProjectsSyncReport(config, client, true, time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildProjectsSyncReport error: %v", err)
	}
	if report.Project.Title != "Gira" || report.Project.Number != 7 {
		t.Fatalf("project = %+v", report.Project)
	}
}

func TestBuildProjectsSyncReportRequiresTitleWhenLinkedProjectsAmbiguous(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
	}
	client := &fakeProjectsSyncClient{
		linkedProjects: map[string][]ProjectsSyncProject{"StatPan/gira": {
			{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"},
			{ID: "PVT_2", Owner: "StatPan", Number: 8, Title: "Other"},
		}},
	}

	_, err := BuildProjectsSyncReport(config, client, true, time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "workspace.project.title is required") {
		t.Fatalf("expected ambiguous linked project error, got %v", err)
	}
}

func TestBuildProjectsSyncReportPlansMissingFieldsAndDateUpdates(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
		Project:   ProjectConfig{Owner: "StatPan", Title: "Gira"},
	}
	client := &fakeProjectsSyncClient{
		project:     ProjectsSyncProject{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"},
		projects:    []ProjectsSyncProject{{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"}},
		fields:      []ProjectsSyncField{{ID: "status-field", Name: "Status", Type: "SINGLE_SELECT", Options: map[string]string{"Todo": "todo"}}},
		statusField: ProjectsSyncStatusField{ID: "status-field", Options: map[string]string{"Todo": "todo", "Done": "done"}},
		linked:      map[string]bool{"StatPan/gira": true},
		issues: map[string][]ProjectsSyncIssue{
			"StatPan/gira": {{Repo: "StatPan/gira", Number: 199, Title: "Schedule", URL: "https://github.com/StatPan/gira/issues/199", Labels: []string{"status:ready"}, Milestone: "v1.2", MilestoneDueDate: "2026-06-01"}},
		},
		items: []ProjectsSyncItem{{ID: "item-199", Repo: "StatPan/gira", Number: 199, Status: "Todo"}},
	}

	report, err := BuildProjectsSyncReport(config, client, true, time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildProjectsSyncReport error: %v", err)
	}
	if report.Counts.FieldsCreate != 5 || report.Counts.DateUpdateSkips != 1 || !report.Counts.ViewSetupRequired {
		t.Fatalf("counts = %+v", report.Counts)
	}
	text := FormatProjectsSyncReport(report)
	for _, want := range []string{"project_field:create", "Target date", "view setup"} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted report missing %q:\n%s", want, text)
		}
	}
}

func TestBuildProjectsSyncReportApplyCreatesFieldsAndUpdatesTargetDate(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
		Project:   ProjectConfig{Owner: "StatPan", Title: "Gira"},
	}
	client := &fakeProjectsSyncClient{
		project:     ProjectsSyncProject{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"},
		projects:    []ProjectsSyncProject{{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"}},
		fields:      []ProjectsSyncField{{ID: "status-field", Name: "Status", Type: "SINGLE_SELECT", Options: map[string]string{"Todo": "todo"}}},
		statusField: ProjectsSyncStatusField{ID: "status-field", Options: map[string]string{"Todo": "todo", "Done": "done"}},
		linked:      map[string]bool{"StatPan/gira": true},
		issues: map[string][]ProjectsSyncIssue{
			"StatPan/gira": {{Repo: "StatPan/gira", Number: 199, Title: "Schedule", URL: "https://github.com/StatPan/gira/issues/199", Labels: []string{"status:ready"}, Milestone: "v1.2", MilestoneDueDate: "2026-06-01"}},
		},
		items: []ProjectsSyncItem{{ID: "item-199", Repo: "StatPan/gira", Number: 199, Status: "Todo"}},
	}

	report, err := BuildProjectsSyncReport(config, client, false, time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildProjectsSyncReport error: %v", err)
	}
	if report.Counts.FieldsCreate != 5 || report.Counts.DateUpdates != 1 {
		t.Fatalf("counts = %+v", report.Counts)
	}
	if len(client.createdFields) != 5 || len(client.updatedDates) != 1 {
		t.Fatalf("apply mutations fields=%v dates=%v", client.createdFields, client.updatedDates)
	}
}

func TestBuildProjectsSyncReportPlansPlanningFieldUpdates(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
		Project:   ProjectConfig{Owner: "StatPan", Title: "Gira"},
	}
	client := &fakeProjectsSyncClient{
		project:     ProjectsSyncProject{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"},
		projects:    []ProjectsSyncProject{{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"}},
		fields:      allProjectsSyncCanonicalFields(),
		statusField: ProjectsSyncStatusField{ID: "status-field", Options: map[string]string{"Todo": "todo"}},
		linked:      map[string]bool{"StatPan/gira": true},
		issues: map[string][]ProjectsSyncIssue{
			"StatPan/gira": {{Repo: "StatPan/gira", Number: 208, Title: "Planning", URL: "https://github.com/StatPan/gira/issues/208", Labels: []string{"status:ready", "priority:p1", "area:infra", "agent:worker"}}},
		},
		items: []ProjectsSyncItem{{ID: "item-208", Repo: "StatPan/gira", Number: 208, Status: "Todo"}},
	}

	report, err := BuildProjectsSyncReport(config, client, true, time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildProjectsSyncReport error: %v", err)
	}
	if report.Counts.FieldUpdates != 3 || report.Counts.FieldUpdateSkips != 0 {
		t.Fatalf("planning field counts = %+v", report.Counts)
	}
	text := FormatProjectsSyncReport(report)
	for _, want := range []string{"project_field:update", "Priority -> P1", "Layer / workstream -> Infra", "Owner / agent -> worker"} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted report missing %q:\n%s", want, text)
		}
	}
	if len(client.updatedFields) != 0 || len(client.updatedTexts) != 0 {
		t.Fatalf("dry-run mutated planning fields: fields=%v texts=%v", client.updatedFields, client.updatedTexts)
	}
}

func TestBuildProjectsSyncReportApplyUpdatesPlanningFieldsIdempotently(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
		Project:   ProjectConfig{Owner: "StatPan", Title: "Gira"},
	}
	client := &fakeProjectsSyncClient{
		project:     ProjectsSyncProject{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"},
		projects:    []ProjectsSyncProject{{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"}},
		fields:      allProjectsSyncCanonicalFields(),
		statusField: ProjectsSyncStatusField{ID: "status-field", Options: map[string]string{"Todo": "todo"}},
		linked:      map[string]bool{"StatPan/gira": true},
		issues: map[string][]ProjectsSyncIssue{
			"StatPan/gira": {{Repo: "StatPan/gira", Number: 208, Title: "Planning", URL: "https://github.com/StatPan/gira/issues/208", Labels: []string{"status:ready", "priority:p1", "area:infra", "agent:worker"}}},
		},
		items: []ProjectsSyncItem{{ID: "item-208", Repo: "StatPan/gira", Number: 208, Status: "Todo", Priority: "P2", Layer: "Docs", OwnerAgent: "human"}},
	}

	report, err := BuildProjectsSyncReport(config, client, false, time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildProjectsSyncReport error: %v", err)
	}
	if report.Counts.FieldUpdates != 3 || len(client.updatedFields) != 2 || len(client.updatedTexts) != 1 {
		t.Fatalf("apply planning updates wrong: counts=%+v fields=%v texts=%v", report.Counts, client.updatedFields, client.updatedTexts)
	}

	client.items = []ProjectsSyncItem{{ID: "item-208", Repo: "StatPan/gira", Number: 208, Status: "Todo", Priority: "P1", Layer: "Infra", OwnerAgent: "worker"}}
	client.updatedFields = nil
	client.updatedTexts = nil
	report, err = BuildProjectsSyncReport(config, client, false, time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildProjectsSyncReport error: %v", err)
	}
	if report.Counts.FieldUpdates != 0 || len(client.updatedFields) != 0 || len(client.updatedTexts) != 0 {
		t.Fatalf("planning fields should be idempotent: counts=%+v fields=%v texts=%v", report.Counts, client.updatedFields, client.updatedTexts)
	}
}

func TestBuildProjectsSyncReportSkipsPlanningFieldWhenOptionMissing(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
		Project:   ProjectConfig{Owner: "StatPan", Title: "Gira"},
	}
	fields := allProjectsSyncCanonicalFields()
	for i := range fields {
		if fields[i].Name == "Priority" {
			fields[i].Options = map[string]string{"P0": "P0"}
		}
	}
	client := &fakeProjectsSyncClient{
		project:     ProjectsSyncProject{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"},
		projects:    []ProjectsSyncProject{{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"}},
		fields:      fields,
		statusField: ProjectsSyncStatusField{ID: "status-field", Options: map[string]string{"Todo": "todo"}},
		linked:      map[string]bool{"StatPan/gira": true},
		issues: map[string][]ProjectsSyncIssue{
			"StatPan/gira": {{Repo: "StatPan/gira", Number: 208, Title: "Planning", URL: "https://github.com/StatPan/gira/issues/208", Labels: []string{"status:ready", "priority:p1"}}},
		},
		items: []ProjectsSyncItem{{ID: "item-208", Repo: "StatPan/gira", Number: 208, Status: "Todo"}},
	}

	report, err := BuildProjectsSyncReport(config, client, false, time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildProjectsSyncReport error: %v", err)
	}
	if report.Counts.FieldUpdates != 0 || report.Counts.FieldUpdateSkips != 1 || len(client.updatedFields) != 0 {
		t.Fatalf("missing option should skip planning update: counts=%+v fields=%v", report.Counts, client.updatedFields)
	}
	if len(report.Actions) != 1 || !strings.Contains(report.Actions[0].Reason, "single-select option") {
		t.Fatalf("skip action = %+v", report.Actions)
	}
}

func TestBuildProjectsSyncReportSkipsBlockedFieldTypes(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
		Project:   ProjectConfig{Owner: "StatPan", Title: "Gira"},
	}
	fields := allProjectsSyncCanonicalFields()
	for i := range fields {
		switch fields[i].Name {
		case "Status", "Priority", "Target date":
			fields[i].Type = "TEXT"
		}
	}
	client := &fakeProjectsSyncClient{
		project:  ProjectsSyncProject{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"},
		projects: []ProjectsSyncProject{{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"}},
		fields:   fields,
		linked:   map[string]bool{"StatPan/gira": true},
		issues: map[string][]ProjectsSyncIssue{
			"StatPan/gira": {{Repo: "StatPan/gira", Number: 208, Title: "Planning", URL: "https://github.com/StatPan/gira/issues/208", Labels: []string{"status:in-progress", "priority:p1"}, Milestone: "v1.2", MilestoneDueDate: "2026-06-01"}},
		},
		items: []ProjectsSyncItem{{ID: "item-208", Repo: "StatPan/gira", Number: 208, Status: "Todo"}},
	}

	report, err := BuildProjectsSyncReport(config, client, true, time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildProjectsSyncReport error: %v", err)
	}
	if report.Counts.StatusUpdates != 0 || report.Counts.StatusUpdateSkips != 1 || report.Counts.FieldUpdates != 0 || report.Counts.FieldUpdateSkips != 1 || report.Counts.DateUpdates != 0 || report.Counts.DateUpdateSkips != 1 {
		t.Fatalf("blocked fields should skip downstream updates: %+v", report.Counts)
	}
	text := FormatProjectsSyncReport(report)
	for _, want := range []string{
		"project_field:skip",
		"Status",
		"Priority",
		"Target date",
		"project Status field or option is unavailable",
		"project field or item id is unavailable",
		"project Target date field or item id is unavailable",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("blocked field report missing %q:\n%s", want, text)
		}
	}
	if len(client.updated) != 0 || len(client.updatedFields) != 0 || len(client.updatedDates) != 0 {
		t.Fatalf("blocked fields should not mutate fake client: status=%v fields=%v dates=%v", client.updated, client.updatedFields, client.updatedDates)
	}
}

func TestBuildProjectsSyncReportSkipsMatchingTargetDate(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
		Project:   ProjectConfig{Owner: "StatPan", Title: "Gira"},
	}
	client := &fakeProjectsSyncClient{
		project:     ProjectsSyncProject{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"},
		projects:    []ProjectsSyncProject{{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"}},
		fields:      allProjectsSyncCanonicalFields(),
		statusField: ProjectsSyncStatusField{ID: "status-field", Options: map[string]string{"Todo": "todo"}},
		linked:      map[string]bool{"StatPan/gira": true},
		issues: map[string][]ProjectsSyncIssue{
			"StatPan/gira": {{Repo: "StatPan/gira", Number: 199, Title: "Schedule", URL: "https://github.com/StatPan/gira/issues/199", Labels: []string{"status:ready"}, Milestone: "v1.2", MilestoneDueDate: "2026-06-01"}},
		},
		items: []ProjectsSyncItem{{ID: "item-199", Repo: "StatPan/gira", Number: 199, Status: "Todo", TargetDate: "2026-06-01"}},
	}

	report, err := BuildProjectsSyncReport(config, client, false, time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildProjectsSyncReport error: %v", err)
	}
	if report.Counts.DateUpdates != 0 || len(client.updatedDates) != 0 {
		t.Fatalf("date update should be idempotent: %+v dates=%v", report.Counts, client.updatedDates)
	}
}

func TestBuildProjectsSyncReportKeepsClosedProjectItemsAsDoneByDefault(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
		Project:   ProjectConfig{Owner: "StatPan", Title: "Gira"},
	}
	client := &fakeProjectsSyncClient{
		project:     ProjectsSyncProject{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"},
		projects:    []ProjectsSyncProject{{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"}},
		fields:      allProjectsSyncCanonicalFields(),
		statusField: ProjectsSyncStatusField{ID: "status-field", Options: map[string]string{"Todo": "todo", "Done": "done"}},
		linked:      map[string]bool{"StatPan/gira": true},
		issues: map[string][]ProjectsSyncIssue{
			"StatPan/gira": {{Repo: "StatPan/gira", Number: 180, Title: "Open", URL: "https://github.com/StatPan/gira/issues/180", Labels: []string{"status:ready"}}},
		},
		items: []ProjectsSyncItem{
			{ID: "item-180", Repo: "StatPan/gira", Number: 180, IssueState: "open", Status: "Todo"},
			{ID: "item-199", Repo: "StatPan/gira", Number: 199, IssueState: "closed", Status: "In Progress"},
		},
	}

	report, err := BuildProjectsSyncReport(config, client, true, time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildProjectsSyncReport error: %v", err)
	}
	if report.Counts.ProjectItemsArchive != 0 || report.Counts.StatusUpdates != 1 || len(client.archived) != 0 {
		t.Fatalf("default closed item handling wrong: counts=%+v archived=%v", report.Counts, client.archived)
	}
	text := FormatProjectsSyncReport(report)
	if strings.Contains(text, "project_item:archive") || !strings.Contains(text, "project_status:update") || !strings.Contains(text, "-> Done") {
		t.Fatalf("formatted report should sync Done without archive:\n%s", text)
	}
}

func TestBuildProjectsSyncReportArchiveClosedOptIn(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
		Project:   ProjectConfig{Owner: "StatPan", Title: "Gira"},
	}
	client := &fakeProjectsSyncClient{
		project:     ProjectsSyncProject{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"},
		projects:    []ProjectsSyncProject{{ID: "PVT_1", Owner: "StatPan", Number: 7, Title: "Gira"}},
		fields:      allProjectsSyncCanonicalFields(),
		statusField: ProjectsSyncStatusField{ID: "status-field", Options: map[string]string{"Todo": "todo"}},
		linked:      map[string]bool{"StatPan/gira": true},
		issues:      map[string][]ProjectsSyncIssue{"StatPan/gira": {}},
		items:       []ProjectsSyncItem{{ID: "item-199", Repo: "StatPan/gira", Number: 199, IssueState: "closed", Status: "Done"}},
	}

	report, err := BuildProjectsSyncReportWithOptions(config, client, ProjectsSyncOptions{DryRun: false, ArchiveClosed: true, FetchedAt: time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("BuildProjectsSyncReport error: %v", err)
	}
	if report.Counts.ProjectItemsArchive != 1 || len(client.archived) != 1 || client.archived[0] != "item-199" {
		t.Fatalf("apply archive wrong: counts=%+v archived=%v", report.Counts, client.archived)
	}
}

func TestGHProjectsSyncClientProjectItemsGraphQLUsesSupportedPageSize(t *testing.T) {
	runner := &recordingProjectsSyncRunner{output: []byte(`{"data":{"node":{"items":{"nodes":[]}}}}`)}
	client := NewGHProjectsSyncClient(runner)

	if _, err := client.ProjectItemsGraphQL("PVT_1"); err != nil {
		t.Fatalf("ProjectItemsGraphQL error: %v", err)
	}
	if !strings.Contains(runner.args, "items(first:100)") {
		t.Fatalf("query should use GitHub-supported page size, args=%s", runner.args)
	}
	if strings.Contains(runner.args, "items(first:500)") {
		t.Fatalf("query should not use unsupported page size, args=%s", runner.args)
	}
}

type fakeProjectsSyncClient struct {
	mu             sync.Mutex
	calls          map[string]int
	delays         map[string]time.Duration
	project        ProjectsSyncProject
	projects       []ProjectsSyncProject
	fields         []ProjectsSyncField
	statusField    ProjectsSyncStatusField
	linked         map[string]bool
	linkedProjects map[string][]ProjectsSyncProject
	issues         map[string][]ProjectsSyncIssue
	items          []ProjectsSyncItem
	linkedApplied  []string
	createdFields  []ProjectsSyncFieldDef
	added          []ProjectsSyncIssue
	archived       []string
	updated        []string
	updatedFields  []string
	updatedTexts   []string
	updatedDates   []string
}

type recordingProjectsSyncRunner struct {
	args   string
	output []byte
}

func (r *recordingProjectsSyncRunner) Run(name string, args ...string) ([]byte, error) {
	r.args = name + " " + strings.Join(args, " ")
	return r.output, nil
}

func (c *fakeProjectsSyncClient) Project(owner string, number int) (ProjectsSyncProject, error) {
	if delay := c.recordCall("Project"); delay > 0 {
		time.Sleep(delay)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.project, nil
}

func (c *fakeProjectsSyncClient) Projects(owner string) ([]ProjectsSyncProject, error) {
	if delay := c.recordCall("Projects"); delay > 0 {
		time.Sleep(delay)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.projects, nil
}

func (c *fakeProjectsSyncClient) LinkedProjects(repo RepoRef) ([]ProjectsSyncProject, error) {
	if delay := c.recordCall("LinkedProjects"); delay > 0 {
		time.Sleep(delay)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.linkedProjects[repo.FullName()], nil
}

func (c *fakeProjectsSyncClient) StatusField(owner string, number int) (ProjectsSyncStatusField, error) {
	if delay := c.recordCall("StatusField"); delay > 0 {
		time.Sleep(delay)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.statusField, nil
}

func (c *fakeProjectsSyncClient) ProjectFields(projectID string) ([]ProjectsSyncField, error) {
	if delay := c.recordCall("ProjectFields"); delay > 0 {
		time.Sleep(delay)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.fields, nil
}

func (c *fakeProjectsSyncClient) RepoLinked(owner string, number int, repo RepoRef) (bool, error) {
	if delay := c.recordCall("RepoLinked"); delay > 0 {
		time.Sleep(delay)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.linked[repo.FullName()], nil
}

func (c *fakeProjectsSyncClient) OpenIssues(repo RepoRef) ([]ProjectsSyncIssue, error) {
	if delay := c.recordCall("OpenIssues"); delay > 0 {
		time.Sleep(delay)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.issues[repo.FullName()], nil
}

func (c *fakeProjectsSyncClient) ProjectItems(owner string, number int) ([]ProjectsSyncItem, error) {
	if delay := c.recordCall("ProjectItems"); delay > 0 {
		time.Sleep(delay)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.items, nil
}

func (c *fakeProjectsSyncClient) ProjectItemsGraphQL(projectID string) ([]ProjectsSyncItem, error) {
	if delay := c.recordCall("ProjectItemsGraphQL"); delay > 0 {
		time.Sleep(delay)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.items, nil
}

func (c *fakeProjectsSyncClient) LinkRepo(owner string, number int, repo RepoRef) error {
	if delay := c.recordCall("LinkRepo"); delay > 0 {
		time.Sleep(delay)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.linkedApplied = append(c.linkedApplied, repo.FullName())
	return nil
}

func (c *fakeProjectsSyncClient) AddItem(owner string, number int, issue ProjectsSyncIssue) (string, error) {
	if delay := c.recordCall("AddItem"); delay > 0 {
		time.Sleep(delay)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.added = append(c.added, issue)
	return "item-added", nil
}

func (c *fakeProjectsSyncClient) CreateProjectField(owner string, number int, field ProjectsSyncFieldDef) (string, error) {
	if delay := c.recordCall("CreateProjectField"); delay > 0 {
		time.Sleep(delay)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.createdFields = append(c.createdFields, field)
	id := "field-" + strings.ToLower(strings.ReplaceAll(field.Name, " ", "-"))
	c.fields = append(c.fields, ProjectsSyncField{ID: id, Name: field.Name, Type: field.Type})
	return id, nil
}

func (c *fakeProjectsSyncClient) ArchiveItem(owner string, number int, itemID string) error {
	if delay := c.recordCall("ArchiveItem"); delay > 0 {
		time.Sleep(delay)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.archived = append(c.archived, itemID)
	return nil
}

func (c *fakeProjectsSyncClient) UpdateItemStatus(projectID string, itemID string, fieldID string, optionID string) error {
	if delay := c.recordCall("UpdateItemStatus"); delay > 0 {
		time.Sleep(delay)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updated = append(c.updated, itemID+":"+optionID)
	return nil
}

func (c *fakeProjectsSyncClient) UpdateItemSingleSelect(projectID string, itemID string, fieldID string, optionID string) error {
	if delay := c.recordCall("UpdateItemSingleSelect"); delay > 0 {
		time.Sleep(delay)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updatedFields = append(c.updatedFields, itemID+":"+fieldID+":"+optionID)
	return nil
}

func (c *fakeProjectsSyncClient) UpdateItemText(projectID string, itemID string, fieldID string, text string) error {
	if delay := c.recordCall("UpdateItemText"); delay > 0 {
		time.Sleep(delay)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updatedTexts = append(c.updatedTexts, itemID+":"+fieldID+":"+text)
	return nil
}

func (c *fakeProjectsSyncClient) UpdateItemDate(projectID string, itemID string, fieldID string, date string) error {
	if delay := c.recordCall("UpdateItemDate"); delay > 0 {
		time.Sleep(delay)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updatedDates = append(c.updatedDates, itemID+":"+fieldID+":"+date)
	return nil
}

func (c *fakeProjectsSyncClient) recordCall(name string) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.calls == nil {
		c.calls = map[string]int{}
	}
	c.calls[name]++
	return c.delays[name]
}

func (c *fakeProjectsSyncClient) callCount(name string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls[name]
}

func allProjectsSyncCanonicalFields() []ProjectsSyncField {
	fields := make([]ProjectsSyncField, 0, len(projectsSyncCanonicalFields))
	for _, field := range projectsSyncCanonicalFields {
		id := "field-" + strings.ToLower(strings.ReplaceAll(field.Name, " ", "-"))
		if field.Name == "Status" {
			id = "status-field"
		}
		fields = append(fields, ProjectsSyncField{ID: id, Name: field.Name, Type: field.Type, Options: projectsSyncOptionsByName(field.Options)})
	}
	return fields
}
