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

func TestBuildQueueHandoffReportFromNextEmbedsWorkerHandoff(t *testing.T) {
	workspace := WorkspaceSummary{Name: "personal", Owner: "StatPan"}
	queues := BuildWorkspaceQueues(workspace, []WorkStatusResult{
		{Repo: "StatPan/app", Issue: 42, Title: "Implement queue handoff", State: "open", Status: "Ready", Labels: []string{"status:ready"}},
	})
	next, err := BuildQueueNextReport(WorkspaceReport{
		Workspace:  workspace,
		ConfigPath: ".gira/config.yaml",
		Queues:     queues,
	}, QueueNextOptions{Role: AgentPromptRoleReviewer, Profile: AgentPromptProfilePython})
	if err != nil {
		t.Fatalf("BuildQueueNextReport error: %v", err)
	}
	handoff := TicketHandoffReport{
		Command:       "ticket handoff",
		SchemaVersion: WorkerHandoffSchemaVersion,
		Repo:          "StatPan/app",
		Issue:         42,
		Role:          AgentPromptRoleReviewer,
		Profile:       AgentPromptProfilePython,
		Readiness:     TicketReadinessReport{SchemaVersion: TicketReadinessSchemaVersion, Readiness: "ready"},
		NextAction:    "request_review",
	}
	report := BuildQueueHandoffReportFromNext(next, &handoff, AgentPromptRoleReviewer, AgentPromptProfilePython)
	if report.SchemaVersion != QueueHandoffSchemaVersion || report.WorkerHandoff == nil || report.WorkerHandoff.SchemaVersion != WorkerHandoffSchemaVersion {
		t.Fatalf("unexpected handoff report: %+v", report)
	}
	if report.Selected == nil || report.Selected.Issue != 42 || report.NextAction != "start_run" {
		t.Fatalf("selected/action = %+v next=%s", report.Selected, report.NextAction)
	}
	if !strings.Contains(report.RunCommand, "--profile python") || report.NextStep != report.RunCommand {
		t.Fatalf("run command/next step = %q %q", report.RunCommand, report.NextStep)
	}
}

func TestBuildQueueHandoffReportStopsWhenWorkerHandoffNeedsRefinement(t *testing.T) {
	workspace := WorkspaceSummary{Name: "personal", Owner: "StatPan"}
	queues := BuildWorkspaceQueues(workspace, []WorkStatusResult{
		{Repo: "StatPan/app", Issue: 42, Title: "Refine before handoff", State: "open", Status: "Ready", Labels: []string{"status:ready"}},
	})
	next, err := BuildQueueNextReport(WorkspaceReport{Workspace: workspace, Queues: queues}, QueueNextOptions{})
	if err != nil {
		t.Fatalf("BuildQueueNextReport error: %v", err)
	}
	handoff := TicketHandoffReport{
		Command:       "ticket handoff",
		SchemaVersion: WorkerHandoffSchemaVersion,
		Repo:          "StatPan/app",
		Issue:         42,
		Role:          AgentPromptRoleImplementer,
		Profile:       AgentPromptProfileDefault,
		Readiness: TicketReadinessReport{
			SchemaVersion: TicketReadinessSchemaVersion,
			Readiness:     "needs_refinement",
			Findings:      []TicketReadinessFinding{{Severity: "error", Kind: "missing_scope"}},
		},
		NextAction:      "refine_ticket",
		NextSafeCommand: "gira ticket view --repo StatPan/app --ticket 42",
	}
	report := BuildQueueHandoffReportFromNext(next, &handoff, AgentPromptRoleImplementer, AgentPromptProfileDefault)
	if report.NextAction != "refine_ticket" || report.NextStep != handoff.NextSafeCommand {
		t.Fatalf("next action/step = %s %s", report.NextAction, report.NextStep)
	}
	for _, want := range []string{"worker_handoff_not_ready", "readiness_needs_refinement", "finding_missing_scope"} {
		if !containsString(report.StopReasons, want) {
			t.Fatalf("stop reasons missing %q: %+v", want, report.StopReasons)
		}
	}
	if !strings.Contains(FormatQueueHandoff(report, false), "worker handoff: schema=worker-handoff/v1 readiness=needs_refinement") {
		t.Fatalf("format should include worker handoff readiness")
	}
}

func TestBuildQueueTakeReportDryRunPlansTicketStart(t *testing.T) {
	workspace := WorkspaceSummary{Name: "personal", Owner: "StatPan"}
	queues := BuildWorkspaceQueues(workspace, []WorkStatusResult{
		{Repo: "StatPan/app", Issue: 42, Title: "Implement queue take", State: "open", Status: "Ready", Labels: []string{"status:ready"}},
	})
	next, err := BuildQueueNextReport(WorkspaceReport{Workspace: workspace, Queues: queues}, QueueNextOptions{})
	if err != nil {
		t.Fatalf("BuildQueueNextReport error: %v", err)
	}
	handoff := TicketHandoffReport{
		Command:       "ticket handoff",
		SchemaVersion: WorkerHandoffSchemaVersion,
		Repo:          "StatPan/app",
		Issue:         42,
		Role:          AgentPromptRoleImplementer,
		Profile:       AgentPromptProfileDefault,
		Readiness:     TicketReadinessReport{SchemaVersion: TicketReadinessSchemaVersion, Readiness: "ready"},
		NextAction:    "implement",
	}
	queueHandoff := BuildQueueHandoffReportFromNext(next, &handoff, AgentPromptRoleImplementer, AgentPromptProfileDefault)
	start := WorkStartResult{
		Repo:       "StatPan/app",
		Issue:      42,
		Title:      "Implement queue take",
		Branch:     "issue-42-implement-queue-take",
		DryRun:     true,
		Status:     "Ready",
		NextStatus: "In progress",
		NextStep:   "gira ticket start 42 --apply",
	}
	report := BuildQueueTakeReport(queueHandoff, &start, true, false)
	if report.SchemaVersion != QueueTakeSchemaVersion || report.StartResult == nil || report.StartResult.SchemaVersion != WorkStartResultSchemaVersion {
		t.Fatalf("unexpected take report: %+v", report)
	}
	if report.NextAction != "apply_ticket_start" || report.NextStep != "gira queue take --repo StatPan/app --ticket 42 --apply" {
		t.Fatalf("next action/step = %s %s", report.NextAction, report.NextStep)
	}
	if report.Approval == nil || report.Approval.OutputSchema != QueueTakeSchemaVersion || report.StartResult.Approval == nil {
		t.Fatalf("missing approval evidence: %+v start=%+v", report.Approval, report.StartResult)
	}
	if !strings.Contains(FormatQueueTake(report, false), "ticket start: branch=issue-42-implement-queue-take") {
		t.Fatalf("queue take text missing ticket start summary")
	}
}

func TestBuildQueueTakeReportStopsBeforeTicketStart(t *testing.T) {
	handoff := QueueHandoffReport{
		SchemaVersion: QueueHandoffSchemaVersion,
		Command:       "queue handoff",
		StopReasons:   []string{"queue_not_handoff_safe", "queue_blocked"},
		NextAction:    "inspect_queues",
		NextStep:      "gira queue list",
	}
	report := BuildQueueTakeReport(handoff, nil, false, true)
	if report.StartResult != nil || report.NextAction != "inspect_queues" {
		t.Fatalf("unexpected stopped report: %+v", report)
	}
	for _, want := range []string{"queue_not_handoff_safe", "queue_blocked"} {
		if !containsString(report.StopReasons, want) {
			t.Fatalf("stop reasons missing %q: %+v", want, report.StopReasons)
		}
	}
}

func TestQueueHandoffStopReasonsForBlockedItem(t *testing.T) {
	item := WorkspaceQueueItem{Queue: "blocked", ReasonCodes: []string{"status_blocked"}}
	reasons := QueueHandoffStopReasonsForItem(item)
	for _, want := range []string{"queue_not_handoff_safe", "queue_blocked", "reason_status_blocked"} {
		if !containsString(reasons, want) {
			t.Fatalf("stop reasons missing %q: %+v", want, reasons)
		}
	}
	if WorkspaceQueueItemHandoffSafe(item) {
		t.Fatalf("blocked item should not be handoff-safe")
	}
}

func TestNormalizeWorkspaceQueueNameRejectsUnknownQueue(t *testing.T) {
	if _, err := NormalizeWorkspaceQueueName("later"); err == nil {
		t.Fatalf("expected unknown queue error")
	}
}
