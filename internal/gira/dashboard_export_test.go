package gira

import (
	"encoding/json"
	"errors"
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
