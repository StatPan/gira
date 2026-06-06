package gira

import (
	"strings"
	"testing"
)

func TestBuildQueueListReportFiltersAliasesAndLimit(t *testing.T) {
	workspace := WorkspaceSummary{Name: "personal", Owner: "StatPan"}
	queues := BuildWorkspaceQueues(workspace, []WorkStatusResult{
		{Repo: "StatPan/app-a", Issue: 10, Title: "Ready A", State: "open", Status: "Ready", Labels: []string{"status:ready"}},
		{Repo: "StatPan/app-b", Issue: 11, Title: "Ready B", State: "open", Status: "Ready", Labels: []string{"status:ready"}},
		{Repo: "StatPan/app-c", Issue: 12, Title: "Blocked C", State: "open", Status: "Blocked", Labels: []string{"status:blocked"}},
	})
	report, err := BuildQueueListReport(WorkspaceReport{
		Workspace:  workspace,
		ConfigPath: ".gira/config.yaml",
		Queues:     queues,
		FetchedAt:  "2026-06-06T00:00:00Z",
	}, QueueListOptions{QueueNames: []string{"ready"}, RepoFilters: []string{"StatPan/app-b"}, Limit: 1})
	if err != nil {
		t.Fatalf("BuildQueueListReport error: %v", err)
	}
	if report.SchemaVersion != QueueListSchemaVersion || report.SourceContract != WorkspaceQueuesSchemaVersion {
		t.Fatalf("unexpected queue list schema/source: %+v", report)
	}
	if len(report.Items) != 1 || report.Items[0].Queue != "agent_ready" || report.Items[0].Issue != 11 {
		t.Fatalf("items = %+v", report.Items)
	}
	if report.Counts.AgentReady != 1 || report.Counts.Blocked != 0 {
		t.Fatalf("filtered counts = %+v", report.Counts)
	}
	if report.Filters.Queues[0] != "agent_ready" || report.Filters.Repos[0] != "StatPan/app-b" || report.Filters.Limit != 1 {
		t.Fatalf("filters = %+v", report.Filters)
	}
	text := FormatQueueList(report, false)
	if !strings.Contains(text, "ready") || !strings.Contains(text, "gira ticket start") {
		t.Fatalf("queue list text missing short queue or next command:\n%s", text)
	}
}

func TestBuildQueueNextReportSelectsAgentReadyItem(t *testing.T) {
	workspace := WorkspaceSummary{Name: "personal", Owner: "StatPan"}
	queues := BuildWorkspaceQueues(workspace, []WorkStatusResult{
		{Repo: "StatPan/app", Issue: 42, Title: "Implement queue next", State: "open", Status: "Ready", Labels: []string{"status:ready"}},
		{Repo: "StatPan/app", Issue: 43, Title: "Needs human", State: "open", Status: "Ready", Labels: []string{"status:ready", "needs:human"}},
	})
	report, err := BuildQueueNextReport(WorkspaceReport{
		Workspace:  workspace,
		ConfigPath: ".gira/config.yaml",
		Queues:     queues,
	}, QueueNextOptions{Role: AgentPromptRoleImplementer})
	if err != nil {
		t.Fatalf("BuildQueueNextReport error: %v", err)
	}
	if report.SchemaVersion != QueueNextSchemaVersion || report.Selected == nil {
		t.Fatalf("unexpected queue next report: %+v", report)
	}
	if report.Selected.Issue != 42 || report.NextAction != "handoff_ticket" {
		t.Fatalf("selected issue/action = #%d %s", report.Selected.Issue, report.NextAction)
	}
	if !strings.Contains(report.Selected.SelectionReason, "ticket_ready") {
		t.Fatalf("selection reason = %q", report.Selected.SelectionReason)
	}
	if report.Selected.HandoffCommand != "gira ticket handoff --repo StatPan/app --ticket 42 implementer --json" {
		t.Fatalf("handoff command = %q", report.Selected.HandoffCommand)
	}
	if report.Selected.RunCommand != "gira run start 42 --repo StatPan/app --role implementer --dry-run" {
		t.Fatalf("run command = %q", report.Selected.RunCommand)
	}
	text := FormatQueueNext(report, false)
	for _, want := range []string{"next safe command:", "handoff command:", "run command:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("queue next text missing %q:\n%s", want, text)
		}
	}
}

func TestBuildQueueNextReportStopsWhenNoAgentReadyItem(t *testing.T) {
	workspace := WorkspaceSummary{Name: "personal", Owner: "StatPan"}
	queues := BuildWorkspaceQueues(workspace, []WorkStatusResult{
		{Repo: "StatPan/app", Issue: 44, Title: "Blocked work", State: "open", Status: "Blocked", Labels: []string{"status:blocked"}},
	})
	report, err := BuildQueueNextReport(WorkspaceReport{
		Workspace:  workspace,
		ConfigPath: ".gira/config.yaml",
		Queues:     queues,
	}, QueueNextOptions{})
	if err != nil {
		t.Fatalf("BuildQueueNextReport error: %v", err)
	}
	if report.Selected != nil || report.NextAction != "inspect_queues" {
		t.Fatalf("unexpected selected/action: %+v", report)
	}
	if !containsString(report.StopReasons, "no_agent_ready_item") || !containsString(report.StopReasons, "blocked_present") {
		t.Fatalf("stop reasons = %+v", report.StopReasons)
	}
	if report.NextStep != "gira queue list --config .gira/config.yaml" {
		t.Fatalf("next step = %q", report.NextStep)
	}
}

func TestNormalizeWorkspaceQueueNameRejectsUnknownQueue(t *testing.T) {
	if _, err := NormalizeWorkspaceQueueName("later"); err == nil {
		t.Fatalf("expected unknown queue error")
	}
}
