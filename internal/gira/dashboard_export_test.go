package gira

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBuildDashboardExportPlanDeterministicTextAndJSON(t *testing.T) {
	client := &fakeDashboardExportClient{
		repo: ParseRepoRefMust("StatPan/gira"),
		issues: []DashboardRawIssue{
			{IssueNumber: 20, Title: "Issue B", State: "open", Labels: []string{"status:ready"}, UpdatedAt: "2026-04-20T08:00:00Z"},
			{IssueNumber: 10, Title: "Issue A", State: "open", Labels: []string{"status:blocked"}, UpdatedAt: "2026-04-25T08:00:00Z"},
		},
		pulls: []DashboardRawPullRequest{
			{PullRequestNumber: 3, Title: "PR B", State: "open", Labels: []string{"status:ready"}},
			{PullRequestNumber: 1, Title: "PR A", State: "closed", Labels: []string{"status:closed"}},
		},
		milestones: []DashboardRawMilestone{
			{MilestoneNumber: 2, Title: "Second", State: "closed"},
			{MilestoneNumber: 1, Title: "First", State: "open"},
		},
		projectSnapshot: ProjectSyncSnapshot{
			ProjectName: "Product OS",
			RoadmapItems: []ProjectRoadmapItem{
				{IssueNumber: 8, IssueTitle: "Roadmap 2", IssueURL: "https://github.com/StatPan/gira/issues/8", TypeLabel: "type:task", Roadmapable: true, TargetDate: ptrString("2026-04-22")},
				{IssueNumber: 3, IssueTitle: "Roadmap 1", IssueURL: "https://github.com/StatPan/gira/issues/3", TypeLabel: "type:epic", Roadmapable: true, TargetDate: ptrString("2026-04-18")},
			},
		},
		capabilities: ProjectCapabilityReport{
			Capabilities: map[string]ProjectCapabilityStatus{
				"issues:read":  "allowed",
				"issues:write": "allowed",
			},
			BlockedActions: []ProjectCapabilityBlock{
				{Action: "projectsv2:write", Reason: "mock"},
				{Action: "repo:settings:write", Reason: "mock"},
			},
		},
	}
	snapshotAt := time.Date(2026, 4, 26, 12, 0, 34, 1200, time.UTC)

	planA, bundleA, err := BuildDashboardExportPlan(client.repo, "./out/dashboard", snapshotAt, true, client)
	if err != nil {
		t.Fatalf("BuildDashboardExportPlan returned error: %v", err)
	}
	planB, bundleB, err := BuildDashboardExportPlan(client.repo, "./out/dashboard", snapshotAt, true, client)
	if err != nil {
		t.Fatalf("BuildDashboardExportPlan returned error: %v", err)
	}

	textA := FormatDashboardExportPlan(planA)
	textB := FormatDashboardExportPlan(planB)
	if textA != textB {
		t.Fatalf("format output changed:\n%s\n---\n%s", textA, textB)
	}
	jsonA, err := json.MarshalIndent(planA, "", "  ")
	if err != nil {
		t.Fatalf("json A marshal failed: %v", err)
	}
	jsonB, err := json.MarshalIndent(planB, "", "  ")
	if err != nil {
		t.Fatalf("json B marshal failed: %v", err)
	}
	if string(jsonA) != string(jsonB) {
		t.Fatalf("json output changed:\n%s\n---\n%s", jsonA, jsonB)
	}
	if planA.SnapshotAt != formatGitHubTime(snapshotAt) {
		t.Fatalf("plan snapshot_at = %s, want %s", planA.SnapshotAt, formatGitHubTime(snapshotAt))
	}
	if bundleA.Manifest.SnapshotAt != bundleB.Manifest.SnapshotAt {
		t.Fatalf("manifest snapshot changed")
	}
}

func TestDashboardExportArtifactsOrder(t *testing.T) {
	got := DashboardExportArtifacts()
	want := []DashboardExportArtifact{
		{Path: "manifest.json", Kind: "manifest_json", WillWrite: true},
		{Path: "raw/github.json", Kind: "raw_json", WillWrite: true},
		{Path: "raw/transitions.json", Kind: "raw_json", WillWrite: true},
		{Path: "raw/capabilities.json", Kind: "raw_json", WillWrite: true},
		{Path: "derived/execution_board.json", Kind: "derived_json", WillWrite: true},
		{Path: "derived/roadmap_timeline.json", Kind: "derived_json", WillWrite: true},
		{Path: "derived/warnings.json", Kind: "derived_json", WillWrite: true},
		{Path: "csv/execution_items.csv", Kind: "csv", WillWrite: true},
		{Path: "csv/roadmap_items.csv", Kind: "csv", WillWrite: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("artifacts mismatch: %v", got)
	}
}

func TestDashboardExportWorkspaceArtifactsOrder(t *testing.T) {
	got := DashboardExportWorkspaceArtifacts()
	want := []DashboardExportArtifact{
		{Path: "manifest.json", Kind: "manifest_json", WillWrite: true},
		{Path: "raw/workspace_status.json", Kind: "raw_json", WillWrite: true},
		{Path: "derived/workspace_queues.json", Kind: "derived_json", WillWrite: true},
		{Path: "derived/workspace_dashboard.json", Kind: "derived_json", WillWrite: true},
		{Path: "csv/workspace_queue_items.csv", Kind: "csv", WillWrite: true},
		{Path: "index.html", Kind: "html", WillWrite: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("workspace artifacts mismatch: %v", got)
	}
}

func TestBuildWorkspaceDashboardExportPlanUsesWorkspaceQueues(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:       "personal",
		Owner:      "StatPan",
		InboxRepo:  ParseRepoRefMust("StatPan/gira"),
		Repos:      []RepoRef{ParseRepoRefMust("StatPan/gira")},
		ConfigPath: ".gira/config.yaml",
	}
	client := queueWorkspaceClient{
		fakeWorkspaceClient: fakeWorkspaceClient{
			status: map[string]StatusSummary{
				"StatPan/gira": {
					Repo:   "StatPan/gira",
					Counts: StatusCounts{Issues: IssueCounts{Open: 2}},
					Issues: StatusIssueLists{Open: []IssueStats{
						{Number: 10, Title: "Ready issue", State: "open", Labels: []string{"status:ready"}},
						{Number: 11, Title: "Finishable", State: "open", Labels: []string{"status:in-review"}},
					}},
				},
			},
		},
		queues: map[string][]WorkStatusResult{
			"StatPan/gira": {
				{Repo: "StatPan/gira", Issue: 10, Title: "Ready issue", State: "open", Status: "Ready", Labels: []string{"status:ready"}},
				{
					Repo:         "StatPan/gira",
					Issue:        11,
					Title:        "Finishable",
					State:        "open",
					Status:       "In review",
					NextAction:   "merge_when_policy_allows",
					ChecksStatus: "passed",
					ReviewStatus: "approved",
					PullRequest:  &TicketStatusPullRequest{Available: true, Number: 101, State: "OPEN", ReviewDecision: "APPROVED"},
					PRReadiness:  &PRReadinessReport{SchemaVersion: PRReadinessSchemaVersion, PullRequest: 101, Readiness: "ready_for_finish", NextAction: "finish_ticket"},
				},
			},
		},
	}

	plan, bundle, err := BuildWorkspaceDashboardExportPlanWithSignals(config, "./out/dashboard", time.Date(2026, 5, 31, 9, 0, 0, 0, time.UTC), true, client, 14, WorkspaceStatusOptions{}, fakeDashboardSignalBuilder{signals: dashboardTestSignals()})
	if err != nil {
		t.Fatalf("BuildWorkspaceDashboardExportPlan returned error: %v", err)
	}
	if plan.Workspace == nil || plan.Workspace.Name != "personal" || plan.Repo != "" {
		t.Fatalf("plan workspace/repo mismatch: %+v", plan)
	}
	if plan.Counts.WorkspaceRepos != 1 || plan.Counts.WorkspaceQueueItems != 2 || plan.Counts.PulseItems != 1 || plan.Counts.StorageSurfaces != 2 {
		t.Fatalf("plan counts = %+v", plan.Counts)
	}
	if bundle.WorkspaceStatus == nil || bundle.WorkspaceQueues == nil || bundle.WorkspaceDashboard == nil {
		t.Fatalf("workspace bundle artifacts missing: %+v", bundle)
	}
	if bundle.WorkspacePulse == nil || bundle.StorageDiagnostics == nil {
		t.Fatalf("workspace signal artifacts missing: %+v", bundle)
	}
	if bundle.WorkspaceDashboard.SchemaVersion != WorkspaceDashboardSchemaVersion {
		t.Fatalf("workspace dashboard schema = %s", bundle.WorkspaceDashboard.SchemaVersion)
	}
	if bundle.WorkspaceDashboard.Source.Contract != WorkspaceStatusSourceContract {
		t.Fatalf("workspace source = %+v", bundle.WorkspaceDashboard.Source)
	}
	if artifacts := bundle.WorkspaceDashboard.Artifacts; artifacts.Manifest != "manifest.json" ||
		artifacts.WorkspaceStatus != "raw/workspace_status.json" ||
		artifacts.WorkspaceQueues != "derived/workspace_queues.json" ||
		artifacts.WorkspaceDashboard != "derived/workspace_dashboard.json" ||
		artifacts.QueueItemsCSV != "csv/workspace_queue_items.csv" ||
		artifacts.WorkspacePulse != "derived/workspace_pulse.json" ||
		artifacts.PulseItemsCSV != "csv/workspace_pulse_items.csv" ||
		artifacts.StorageDiagnostics != "derived/storage_diagnostics.json" ||
		artifacts.IndexHTML != "index.html" {
		t.Fatalf("workspace dashboard artifact index = %+v", artifacts)
	}
	if len(bundle.WorkspaceDashboard.TopActions) != 2 || bundle.WorkspaceDashboard.TopActions[0].Issue != 10 {
		t.Fatalf("top actions = %+v", bundle.WorkspaceDashboard.TopActions)
	}
	if bundle.WorkspaceDashboard.TopActions[0].LocalTicketHTML != "tickets/statpan-gira-ticket-10.html" {
		t.Fatalf("top action ticket link = %+v", bundle.WorkspaceDashboard.TopActions[0])
	}
	if bundle.WorkspaceDashboard.TopActions[1].LocalTicketHTML != "tickets/statpan-gira-ticket-11.html" || bundle.WorkspaceDashboard.TopActions[1].LocalReviewHTML != "reviews/statpan-gira-pr-101.html" {
		t.Fatalf("top action deep links = %+v", bundle.WorkspaceDashboard.TopActions[1])
	}
	for _, want := range []DashboardExportArtifact{
		{Path: "tickets/statpan-gira-ticket-10.html", Kind: "html", WillWrite: true},
		{Path: "tickets/statpan-gira-ticket-11.html", Kind: "html", WillWrite: true},
		{Path: "reviews/statpan-gira-pr-101.html", Kind: "html", WillWrite: true},
		{Path: "derived/workspace_pulse.json", Kind: "derived_json", WillWrite: true},
		{Path: "csv/workspace_pulse_items.csv", Kind: "csv", WillWrite: true},
		{Path: "derived/storage_diagnostics.json", Kind: "derived_json", WillWrite: true},
	} {
		if !containsDashboardArtifact(plan.Artifacts, want) || !containsDashboardArtifact(bundle.Manifest.Artifacts, want) {
			t.Fatalf("missing deep-link artifact %+v\nplan=%+v\nmanifest=%+v", want, plan.Artifacts, bundle.Manifest.Artifacts)
		}
	}
	if bundle.WorkspaceDashboard.Pulse == nil || bundle.WorkspaceDashboard.Pulse.Summary.Finished != 1 || bundle.WorkspaceDashboard.Pulse.Items != 1 {
		t.Fatalf("pulse summary missing: %+v", bundle.WorkspaceDashboard.Pulse)
	}
	if bundle.WorkspaceDashboard.Storage == nil || bundle.WorkspaceDashboard.Storage.Surfaces != 2 || bundle.WorkspaceDashboard.Storage.PrivateRunEvidenceIncluded {
		t.Fatalf("storage summary missing or unsafe: %+v", bundle.WorkspaceDashboard.Storage)
	}
	text := FormatDashboardExportPlan(plan)
	for _, want := range []string{"workspace: personal (StatPan)", "workspace_repos: 1", "workspace_queue_items: 2", "pulse_items: 1", "storage_surfaces: 2"} {
		if !strings.Contains(text, want) {
			t.Fatalf("plan text missing %q:\n%s", want, text)
		}
	}
}

func TestDashboardExportSortingAndCSVHeaders(t *testing.T) {
	execution := buildDashboardExecutionItems(
		[]DashboardRawIssue{
			{IssueNumber: 11, Title: "Second", Labels: []string{"status:ready"}},
			{IssueNumber: 3, Title: "Alpha", Labels: []string{"status:blocked"}},
			{IssueNumber: 11, Title: "First", Labels: []string{"status:backlog"}},
		},
		[]DashboardRawPullRequest{
			{PullRequestNumber: 7, Title: "Fix", Labels: []string{"status:open"}},
			{PullRequestNumber: 2, Title: "Hotfix", Labels: []string{"status:open"}},
		},
	)
	wantExecution := []string{"issue:3", "issue:11", "issue:11", "pr:2", "pr:7"}
	for i, item := range execution {
		if item.ID != wantExecution[i] {
			t.Fatalf("execution item[%d].ID = %s, want %s", i, item.ID, wantExecution[i])
		}
	}

	roadmap := buildDashboardRoadmapItems(
		[]DashboardRawMilestone{
			{MilestoneNumber: 9, Title: "Later", State: "closed", DueOn: ptrString("2026-04-30")},
			{MilestoneNumber: 1, Title: "Now", State: "open", DueOn: ptrString("2026-04-20")},
		},
		[]DashboardRawProjectItem{
			{IssueNumber: 20, IssueTitle: "Later roadmap", TypeLabel: "type:task", TargetDate: ptrString("2026-05-01"), Roadmapable: true},
			{IssueNumber: 10, IssueTitle: "No target", TypeLabel: "type:epic", Roadmapable: true},
		},
	)
	wantRoadmap := []string{"milestone:1", "milestone:9", "issue:20", "issue:10"}
	for i, item := range roadmap {
		if item.ID != wantRoadmap[i] {
			t.Fatalf("roadmap item[%d].ID = %s, want %s", i, item.ID, wantRoadmap[i])
		}
	}

	executionCSV, err := renderDashboardExecutionCSV(execution)
	if err != nil {
		t.Fatalf("renderDashboardExecutionCSV returned error: %v", err)
	}
	roadmapCSV, err := renderDashboardRoadmapCSV(roadmap)
	if err != nil {
		t.Fatalf("renderDashboardRoadmapCSV returned error: %v", err)
	}
	executionHeader := strings.Split(strings.TrimSpace(string(executionCSV)), "\n")[0]
	roadmapHeader := strings.Split(strings.TrimSpace(string(roadmapCSV)), "\n")[0]
	if executionHeader != "id,kind,title,status,priority,owner,milestone,target_date,source_refs" {
		t.Fatalf("unexpected execution header: %s", executionHeader)
	}
	if roadmapHeader != "id,title,start_date,target_date,status,phase,source_refs" {
		t.Fatalf("unexpected roadmap header: %s", roadmapHeader)
	}
}

func TestWriteWorkspaceDashboardExportBundleWritesArtifactsAndEscapesHTML(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
	}
	client := queueWorkspaceClient{
		fakeWorkspaceClient: fakeWorkspaceClient{
			status: map[string]StatusSummary{
				"StatPan/gira": {
					Repo:   "StatPan/gira",
					Counts: StatusCounts{Issues: IssueCounts{Open: 1}},
					Issues: StatusIssueLists{Open: []IssueStats{
						{Number: 10, Title: "<script>alert(1)</script>", State: "open", Labels: []string{"status:ready"}},
						{Number: 11, Title: "Review ready", State: "open", Labels: []string{"status:in-review"}},
					}},
				},
			},
		},
		queues: map[string][]WorkStatusResult{
			"StatPan/gira": {
				{Repo: "StatPan/gira", Issue: 10, Title: "<script>alert(1)</script>", State: "open", Status: "Ready", Labels: []string{"status:ready"}},
				{
					Repo:         "StatPan/gira",
					Issue:        11,
					Title:        "Review ready",
					State:        "open",
					Status:       "In review",
					Labels:       []string{"status:in-review"},
					NextAction:   "merge_when_policy_allows",
					ChecksStatus: "passed",
					ReviewStatus: "approved",
					PullRequest:  &TicketStatusPullRequest{Available: true, Number: 101, State: "OPEN", ReviewDecision: "APPROVED"},
					PRReadiness:  &PRReadinessReport{SchemaVersion: PRReadinessSchemaVersion, PullRequest: 101, Readiness: "ready_for_finish", NextAction: "finish_ticket"},
				},
			},
		},
	}
	_, bundle, err := BuildWorkspaceDashboardExportPlanWithSignals(config, "./out/dashboard", time.Date(2026, 5, 31, 9, 0, 0, 0, time.UTC), false, client, 14, WorkspaceStatusOptions{}, fakeDashboardSignalBuilder{signals: dashboardTestSignals()})
	if err != nil {
		t.Fatalf("BuildWorkspaceDashboardExportPlan returned error: %v", err)
	}
	outputRoot := filepath.Join(t.TempDir(), "dashboard")
	if err := WriteDashboardExportBundle(outputRoot, bundle); err != nil {
		t.Fatalf("WriteDashboardExportBundle returned error: %v", err)
	}
	expected := []string{
		"manifest.json",
		"raw/workspace_status.json",
		"derived/workspace_queues.json",
		"derived/workspace_dashboard.json",
		"derived/workspace_pulse.json",
		"derived/storage_diagnostics.json",
		"csv/workspace_queue_items.csv",
		"csv/workspace_pulse_items.csv",
		"index.html",
		"tickets/statpan-gira-ticket-10.html",
		"tickets/statpan-gira-ticket-11.html",
		"reviews/statpan-gira-pr-101.html",
	}
	for _, relativePath := range expected {
		if _, err := os.Stat(filepath.Join(outputRoot, relativePath)); err != nil {
			t.Fatalf("expected workspace exported file %q: %v", relativePath, err)
		}
	}
	csvContent, err := os.ReadFile(filepath.Join(outputRoot, "csv/workspace_queue_items.csv"))
	if err != nil {
		t.Fatalf("read workspace queue csv: %v", err)
	}
	if !strings.HasPrefix(string(csvContent), "queue,repo,issue,title,state,status,pr_number,pr_state,reason_codes,next_safe_command,url\n") {
		t.Fatalf("unexpected workspace queue csv:\n%s", csvContent)
	}
	manifestContent, err := os.ReadFile(filepath.Join(outputRoot, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest json: %v", err)
	}
	var manifest DashboardExportManifest
	if err := json.Unmarshal(manifestContent, &manifest); err != nil {
		t.Fatalf("parse manifest json: %v\n%s", err, manifestContent)
	}
	for _, want := range []DashboardExportArtifact{
		{Path: "raw/workspace_status.json", Kind: "raw_json", WillWrite: true},
		{Path: "derived/workspace_queues.json", Kind: "derived_json", WillWrite: true},
		{Path: "derived/workspace_dashboard.json", Kind: "derived_json", WillWrite: true},
		{Path: "derived/workspace_pulse.json", Kind: "derived_json", WillWrite: true},
		{Path: "csv/workspace_pulse_items.csv", Kind: "csv", WillWrite: true},
		{Path: "derived/storage_diagnostics.json", Kind: "derived_json", WillWrite: true},
		{Path: "csv/workspace_queue_items.csv", Kind: "csv", WillWrite: true},
		{Path: "index.html", Kind: "html", WillWrite: true},
		{Path: "tickets/statpan-gira-ticket-10.html", Kind: "html", WillWrite: true},
		{Path: "reviews/statpan-gira-pr-101.html", Kind: "html", WillWrite: true},
	} {
		if !containsDashboardArtifact(manifest.Artifacts, want) {
			t.Fatalf("manifest missing artifact %+v\nmanifest=%+v", want, manifest.Artifacts)
		}
	}
	dashboardContent, err := os.ReadFile(filepath.Join(outputRoot, "derived/workspace_dashboard.json"))
	if err != nil {
		t.Fatalf("read workspace dashboard json: %v", err)
	}
	var dashboard DashboardWorkspaceDashboard
	if err := json.Unmarshal(dashboardContent, &dashboard); err != nil {
		t.Fatalf("parse workspace dashboard json: %v\n%s", err, dashboardContent)
	}
	if dashboard.SchemaVersion != WorkspaceDashboardSchemaVersion || dashboard.Source.Contract != WorkspaceStatusSourceContract || dashboard.Source.Path != "raw/workspace_status.json" {
		t.Fatalf("workspace dashboard contract mismatch: %+v", dashboard)
	}
	if dashboard.Artifacts.WorkspaceDashboard != "derived/workspace_dashboard.json" || dashboard.Artifacts.IndexHTML != "index.html" || dashboard.Artifacts.Manifest != "manifest.json" {
		t.Fatalf("workspace dashboard artifact index incomplete: %+v", dashboard.Artifacts)
	}
	if dashboard.Pulse == nil || dashboard.Pulse.Path != "derived/workspace_pulse.json" || dashboard.Pulse.Summary.Finished != 1 {
		t.Fatalf("workspace dashboard pulse summary incomplete: %+v", dashboard.Pulse)
	}
	if dashboard.Storage == nil || dashboard.Storage.Path != "derived/storage_diagnostics.json" || dashboard.Storage.PrivateRunEvidenceIncluded {
		t.Fatalf("workspace dashboard storage summary incomplete: %+v", dashboard.Storage)
	}
	if len(dashboard.TopActions) != 2 || dashboard.TopActions[0].LocalTicketHTML == "" || dashboard.TopActions[1].LocalReviewHTML == "" {
		t.Fatalf("workspace dashboard top actions missing local links: %+v", dashboard.TopActions)
	}
	pulseCSV, err := os.ReadFile(filepath.Join(outputRoot, "csv/workspace_pulse_items.csv"))
	if err != nil {
		t.Fatalf("read workspace pulse csv: %v", err)
	}
	if !strings.HasPrefix(string(pulseCSV), "kind,repo,issue,pr,title,confidence,occurred_at,evidence,source_refs\n") {
		t.Fatalf("unexpected workspace pulse csv:\n%s", pulseCSV)
	}
	htmlContent, err := os.ReadFile(filepath.Join(outputRoot, "index.html"))
	if err != nil {
		t.Fatalf("read index html: %v", err)
	}
	if strings.Contains(string(htmlContent), "<script>alert(1)</script>") || !strings.Contains(string(htmlContent), "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("HTML did not escape queue title:\n%s", htmlContent)
	}
	if !strings.Contains(string(htmlContent), `tickets/statpan-gira-ticket-10.html`) {
		t.Fatalf("HTML missing local ticket report link:\n%s", htmlContent)
	}
	if !strings.Contains(string(htmlContent), `reviews/statpan-gira-pr-101.html`) {
		t.Fatalf("HTML missing local review report link:\n%s", htmlContent)
	}
	if !strings.Contains(string(htmlContent), "Pulse") || !strings.Contains(string(htmlContent), "Storage") {
		t.Fatalf("HTML missing pulse/storage summaries:\n%s", htmlContent)
	}
	if strings.Contains(string(htmlContent), "secret prompt text") {
		t.Fatalf("HTML leaked private run detail:\n%s", htmlContent)
	}
	ticketHTML, err := os.ReadFile(filepath.Join(outputRoot, "tickets/statpan-gira-ticket-10.html"))
	if err != nil {
		t.Fatalf("read ticket report html: %v", err)
	}
	if strings.Contains(string(ticketHTML), "<script>alert(1)</script>") || !strings.Contains(string(ticketHTML), "Gira ticket report") {
		t.Fatalf("ticket report HTML unsafe or incomplete:\n%s", ticketHTML)
	}
	reviewHTML, err := os.ReadFile(filepath.Join(outputRoot, "reviews/statpan-gira-pr-101.html"))
	if err != nil {
		t.Fatalf("read review packet html: %v", err)
	}
	if !strings.Contains(string(reviewHTML), "Gira review packet") || !strings.Contains(string(reviewHTML), PRReadinessSchemaVersion) {
		t.Fatalf("review packet HTML incomplete:\n%s", reviewHTML)
	}
	if _, err := os.Stat(filepath.Join(outputRoot, "raw/github.json")); !os.IsNotExist(err) {
		t.Fatalf("workspace-only export should not write repo raw github artifact, stat err=%v", err)
	}
}

func TestRenderWorkspaceDashboardHTMLShowsEmptyStateAndWarnings(t *testing.T) {
	html, err := renderWorkspaceDashboardHTML(DashboardWorkspaceDashboard{
		SchemaVersion: WorkspaceDashboardSchemaVersion,
		SnapshotAt:    "2026-05-31T09:00:00Z",
		Workspace:     WorkspaceSummary{Name: "personal", Owner: "StatPan"},
		Source:        DashboardWorkspaceSource{Contract: WorkspaceStatusSourceContract, Path: "raw/workspace_status.json"},
		Artifacts: DashboardWorkspaceArtifacts{
			Manifest:           "manifest.json",
			WorkspaceStatus:    "raw/workspace_status.json",
			WorkspaceQueues:    "derived/workspace_queues.json",
			WorkspaceDashboard: "derived/workspace_dashboard.json",
			QueueItemsCSV:      "csv/workspace_queue_items.csv",
			IndexHTML:          "index.html",
		},
		Warnings: []DashboardWorkspaceWarning{
			{Code: "workspace_cache_stale", Severity: "warning", Message: "1 workspace status cache entry was stale."},
		},
	})
	if err != nil {
		t.Fatalf("renderWorkspaceDashboardHTML returned error: %v", err)
	}
	output := string(html)
	for _, want := range []string{
		"Snapshot: 2026-05-31T09:00:00Z",
		"Source: workspace-status/v1 at raw/workspace_status.json",
		"No queue actions in this snapshot.",
		"workspace_cache_stale",
		"1 workspace status cache entry was stale.",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("workspace dashboard html missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "No warnings.") {
		t.Fatalf("workspace dashboard html showed no-warning empty state despite warning:\n%s", output)
	}
}

type fakeDashboardSignalBuilder struct {
	signals DashboardWorkspaceSignals
}

func (b fakeDashboardSignalBuilder) BuildWorkspaceDashboardSignals(config WorkspaceConfigResolved, report WorkspaceReport, snapshotAt time.Time) (DashboardWorkspaceSignals, error) {
	return b.signals, nil
}

func dashboardTestSignals() DashboardWorkspaceSignals {
	pulse := PulseReport{
		SchemaVersion: "pulse-report/v1alpha1",
		Command:       "workspace pulse",
		Scope:         PulseScope{Kind: "workspace", Workspace: "StatPan/personal"},
		Window:        PulseWindow{Since: "7d", SinceAt: "2026-05-24T09:00:00Z", Until: "2026-05-31T09:00:00Z", Label: "7d", Limit: 100},
		Source:        StatsSource{Backend: "github", ReadOnly: true},
		Summary:       PulseSummary{Finished: 1, Reviewed: 1},
		Health:        PulseHealth{Ready: 1},
		Items: []PulseItem{{
			Kind:       "finished",
			Repo:       "StatPan/gira",
			Issue:      10,
			PR:         101,
			Title:      "Finish dashboard signal",
			Confidence: "high",
			OccurredAt: "2026-05-31T08:00:00Z",
			Evidence:   []string{"merged_pr", "closing_reference"},
			SourceRefs: []string{"issue:StatPan/gira#10", "pr:StatPan/gira#101"},
		}},
		PrivacyBoundary: PulsePrivacyBoundary{
			Scope:      "work_item_state_only",
			Prohibited: []string{"people_ranking", "private_run_artifact_contents"},
		},
	}
	storage := ConfigStorageReport{
		SchemaVersion: ConfigStorageReportSchemaVersion,
		Command:       "config storage",
		Repo:          "StatPan/gira",
		Source:        "flag",
		ConfigRoot:    "/home/test/.config/gira",
		Surfaces: []ConfigStorageSurface{
			{Name: "workspace_status_cache", Kind: "cache", Path: "/cache/workspace-status", Visibility: "private_local", SourceOfTruth: "github_issue_pr_milestone_state"},
			{Name: "run_manifests", Kind: "runtime_evidence", Path: "/state/runs", Visibility: "private_local", SourceOfTruth: "optional_worker_evidence", Notes: []string{"secret prompt text should stay out of HTML"}},
		},
	}
	return DashboardWorkspaceSignals{Pulse: &pulse, Storage: &storage}
}

func containsDashboardArtifact(values []DashboardExportArtifact, want DashboardExportArtifact) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestDashboardExportBundleMetadataAndSnapshotReuse(t *testing.T) {
	client := &fakeDashboardExportClient{
		repo: ParseRepoRefMust("StatPan/gira"),
		capabilities: ProjectCapabilityReport{
			Capabilities: map[string]ProjectCapabilityStatus{},
		},
	}
	snapshotAt := time.Date(2026, 4, 26, 12, 0, 0, 1200, time.UTC)
	plan, bundle, err := BuildDashboardExportPlan(client.repo, "./out/dashboard", snapshotAt, true, client)
	if err != nil {
		t.Fatalf("BuildDashboardExportPlan returned error: %v", err)
	}
	wantSnapshot := "2026-04-26T12:00:00Z"
	if plan.SnapshotAt != wantSnapshot {
		t.Fatalf("plan snapshot_at = %s, want %s", plan.SnapshotAt, wantSnapshot)
	}
	if bundle.Manifest.SchemaVersion != DashboardExportSchemaVersion {
		t.Fatalf("manifest schema_version = %s, want %s", bundle.Manifest.SchemaVersion, DashboardExportSchemaVersion)
	}
	if bundle.Manifest.Generator.Name != "gira" || bundle.Manifest.Generator.Mode != "dashboard_export" {
		t.Fatalf("manifest generator metadata unexpected: %#v", bundle.Manifest.Generator)
	}
	if bundle.Manifest.Repo != "StatPan/gira" {
		t.Fatalf("manifest repo = %s, want StatPan/gira", bundle.Manifest.Repo)
	}
	if bundle.Manifest.SnapshotAt != plan.SnapshotAt {
		t.Fatalf("manifest snapshot_at mismatch: %s != %s", bundle.Manifest.SnapshotAt, plan.SnapshotAt)
	}
	if bundle.RawGitHub.SnapshotAt != plan.SnapshotAt ||
		bundle.RawTransitions.SnapshotAt != plan.SnapshotAt ||
		bundle.RawCapabilities.SnapshotAt != plan.SnapshotAt ||
		bundle.ExecutionBoard.SnapshotAt != plan.SnapshotAt ||
		bundle.RoadmapTimeline.SnapshotAt != plan.SnapshotAt {
		t.Fatalf("snapshot_at mismatch across bundle artifacts")
	}
}

func TestWriteDashboardExportBundleWritesSafeOutputRoot(t *testing.T) {
	outputRoot := filepath.Join(t.TempDir(), "dashboard")
	bundle := DashboardExportBundle{
		Manifest: DashboardExportManifest{
			SchemaVersion: DashboardExportSchemaVersion,
			Repo:          "StatPan/gira",
		},
	}
	if err := WriteDashboardExportBundle(outputRoot, bundle); err != nil {
		t.Fatalf("WriteDashboardExportBundle returned error: %v", err)
	}
	for _, rel := range []string{"manifest.json", "raw/github.json", "derived/warnings.json", "csv/execution_items.csv", "csv/roadmap_items.csv"} {
		if _, err := os.Stat(filepath.Join(outputRoot, rel)); err != nil {
			t.Fatalf("expected artifact %s: %v", rel, err)
		}
	}
}

func TestWriteDashboardExportBundleRejectsSymlinkOutputRoot(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	outputRoot := filepath.Join(dir, "dashboard")
	if err := os.Symlink(outside, outputRoot); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	err := WriteDashboardExportBundle(outputRoot, DashboardExportBundle{})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("unexpected write through symlink, stat err=%v", err)
	}
}

func TestGHDashboardExportClientUsesReadOnlyCommands(t *testing.T) {
	runner := &readonlyDashboardCommandRunner{}
	client := NewGHDashboardExportClient(ParseRepoRefMust("StatPan/gira"), runner)
	if _, err := client.FetchIssues(); err != nil {
		t.Fatalf("FetchIssues returned error: %v", err)
	}
	if _, err := client.FetchPullRequests(); err != nil {
		t.Fatalf("FetchPullRequests returned error: %v", err)
	}
	if _, err := client.FetchMilestones(); err != nil {
		t.Fatalf("FetchMilestones returned error: %v", err)
	}
	if _, err := client.FetchProjectSnapshot(); err != nil {
		t.Fatalf("FetchProjectSnapshot returned error: %v", err)
	}
	if _, err := client.FetchTransitionSnapshot(); err != nil {
		t.Fatalf("FetchTransitionSnapshot returned error: %v", err)
	}
	if _, err := client.FetchCapabilities(); err != nil {
		t.Fatalf("FetchCapabilities returned error: %v", err)
	}
	if len(runner.mutatingCalls) != 0 {
		t.Fatalf("mutating calls observed: %v", runner.mutatingCalls)
	}
}

type fakeDashboardExportClient struct {
	repo               RepoRef
	issues             []DashboardRawIssue
	pulls              []DashboardRawPullRequest
	milestones         []DashboardRawMilestone
	projectSnapshot    ProjectSyncSnapshot
	transitionSnapshot ProjectTransitionSnapshot
	capabilities       ProjectCapabilityReport
}

func (c *fakeDashboardExportClient) Repo() RepoRef { return c.repo }
func (c *fakeDashboardExportClient) FetchIssues() ([]DashboardRawIssue, error) {
	return c.issues, nil
}
func (c *fakeDashboardExportClient) FetchPullRequests() ([]DashboardRawPullRequest, error) {
	return c.pulls, nil
}
func (c *fakeDashboardExportClient) FetchMilestones() ([]DashboardRawMilestone, error) {
	return c.milestones, nil
}
func (c *fakeDashboardExportClient) FetchProjectSnapshot() (ProjectSyncSnapshot, error) {
	return c.projectSnapshot, nil
}
func (c *fakeDashboardExportClient) FetchTransitionSnapshot() (ProjectTransitionSnapshot, error) {
	return c.transitionSnapshot, nil
}
func (c *fakeDashboardExportClient) FetchCapabilities() (ProjectCapabilityReport, error) {
	return c.capabilities, nil
}

type readonlyDashboardCommandRunner struct {
	mutatingCalls []string
}

func (r *readonlyDashboardCommandRunner) Run(name string, args ...string) ([]byte, error) {
	for i := 0; i < len(args)-1; i++ {
		if args[i] != "-X" {
			continue
		}
		method := strings.ToUpper(args[i+1])
		switch method {
		case "POST", "PATCH", "PUT", "DELETE":
			r.mutatingCalls = append(r.mutatingCalls, method)
			return nil, errors.New("unexpected mutating command")
		}
	}

	if len(args) >= 2 && args[0] == "auth" && args[1] == "status" {
		return []byte(`{"hosts":{"github.com":[{"state":"success","active":true,"host":"github.com","login":"alice","tokenSource":"/home/user/.config/gh/hosts.yml","scopes":"repo"}]}}`), nil
	}
	if len(args) >= 2 && args[0] == "api" && args[1] == "repos/StatPan/gira" {
		return []byte(`{"permissions":{"admin":true,"maintain":true,"pull":true,"push":true,"triage":true}}`), nil
	}
	if len(args) >= 2 && args[0] == "api" && args[1] == "repos/StatPan/gira/issues" {
		return []byte(`[[{"number":1,"title":"Issue","state":"open","labels":[{"name":"status:ready"}],"milestone":null,"updated_at":"2026-04-26T12:00:00Z","html_url":"https://github.com/StatPan/gira/issues/1"}]]`), nil
	}
	if len(args) >= 2 && args[0] == "api" && args[1] == "repos/StatPan/gira/pulls" {
		return []byte(`[[{"number":2,"title":"PR","state":"open","draft":false,"labels":[{"name":"status:ready"}],"html_url":"https://github.com/StatPan/gira/pull/2"}]]`), nil
	}
	if len(args) >= 2 && args[0] == "api" && args[1] == "repos/StatPan/gira/milestones" {
		return []byte(`[[{"number":1,"title":"MVP","state":"open","description":"","open_issues":1,"closed_issues":0}]]`), nil
	}
	if len(args) >= 2 && args[0] == "api" && args[1] == "repos/StatPan/gira/branches" {
		return []byte(`[[{"name":"main"}]]`), nil
	}
	if len(args) >= 2 && args[0] == "api" && args[1] == "graphql" {
		return []byte(`{"data":{"repository":{"viewerPermission":"ADMIN","viewerCanAdminister":true,"projectsV2":{"nodes":[{"title":"Product OS","fields":{"nodes":[]},"items":{"nodes":[]}}]}}}}`), nil
	}
	return []byte("[]"), nil
}

func ptrString(value string) *string {
	return &value
}
