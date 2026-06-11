package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type agentWorkflowBenchmarkFixture struct {
	SchemaVersion string `yaml:"schema_version"`
	Name          string `yaml:"name"`
	Repo          string `yaml:"repo"`
	Issue         struct {
		Number int      `yaml:"number"`
		Title  string   `yaml:"title"`
		State  string   `yaml:"state"`
		Labels []string `yaml:"labels"`
		Body   string   `yaml:"body"`
	} `yaml:"issue"`
	Status struct {
		Status       string   `yaml:"status"`
		PRNumber     int      `yaml:"pr_number"`
		PRState      string   `yaml:"pr_state"`
		ChecksStatus string   `yaml:"checks_status"`
		ReviewStatus string   `yaml:"review_status"`
		NextAction   string   `yaml:"next_action"`
		FinishReady  bool     `yaml:"finish_ready"`
		Blockers     []string `yaml:"blockers"`
	} `yaml:"status"`
	Expected struct {
		TicketReadiness         string   `yaml:"ticket_readiness"`
		Queue                   string   `yaml:"queue"`
		ReasonCode              string   `yaml:"reason_code"`
		NextAction              string   `yaml:"next_action"`
		NextSafeCommandContains string   `yaml:"next_safe_command_contains"`
		Blockers                []string `yaml:"blockers"`
	} `yaml:"expected"`
}

func TestAgentWorkflowCompletionBenchmarkFixtures(t *testing.T) {
	paths, err := filepath.Glob("testdata/agent_workflow_benchmark/*.yaml")
	if err != nil {
		t.Fatalf("glob benchmark fixtures: %v", err)
	}
	if len(paths) < 5 {
		t.Fatalf("benchmark fixtures = %d, want at least 5", len(paths))
	}
	for _, path := range paths {
		path := path
		t.Run(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), func(t *testing.T) {
			fixture := readAgentWorkflowBenchmarkFixture(t, path)
			readiness := EvaluateTicketReadiness(fixture.Issue.Body, fixture.Issue.Labels, fixture.Issue.State)
			if readiness.Readiness != fixture.Expected.TicketReadiness {
				t.Fatalf("ticket readiness = %s, want %s; findings=%+v", readiness.Readiness, fixture.Expected.TicketReadiness, readiness.Findings)
			}
			status := benchmarkStatusFromFixture(fixture, readiness)
			queues := BuildWorkspaceQueues(WorkspaceSummary{Name: "benchmark", Owner: "StatPan"}, []WorkStatusResult{status})
			item, ok := benchmarkQueueItem(queues, fixture.Expected.Queue)
			if !ok {
				t.Fatalf("queue %q not populated by fixture; queues=%+v", fixture.Expected.Queue, queues.Counts)
			}
			if !containsString(item.ReasonCodes, fixture.Expected.ReasonCode) {
				t.Fatalf("reason codes = %+v, want %q", item.ReasonCodes, fixture.Expected.ReasonCode)
			}
			if !strings.Contains(item.NextSafeCommand, fixture.Expected.NextSafeCommandContains) {
				t.Fatalf("next safe command = %q, want contains %q", item.NextSafeCommand, fixture.Expected.NextSafeCommandContains)
			}
			if strings.Join(item.Evidence.Blockers, ",") != strings.Join(fixture.Expected.Blockers, ",") {
				t.Fatalf("blockers = %+v, want %+v", item.Evidence.Blockers, fixture.Expected.Blockers)
			}
			if item.Evidence.NextAction != fixture.Expected.NextAction {
				t.Fatalf("next action = %q, want %q", item.Evidence.NextAction, fixture.Expected.NextAction)
			}
		})
	}
}

func readAgentWorkflowBenchmarkFixture(t *testing.T, path string) agentWorkflowBenchmarkFixture {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture agentWorkflowBenchmarkFixture
	if err := yaml.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if fixture.SchemaVersion != "agent-workflow-benchmark/v1" {
		t.Fatalf("schema_version = %q", fixture.SchemaVersion)
	}
	return fixture
}

func benchmarkStatusFromFixture(fixture agentWorkflowBenchmarkFixture, readiness TicketReadinessReport) WorkStatusResult {
	status := WorkStatusResult{
		Repo:            fixture.Repo,
		Issue:           fixture.Issue.Number,
		Title:           fixture.Issue.Title,
		State:           fixture.Issue.State,
		Status:          fixture.Status.Status,
		Labels:          append([]string(nil), fixture.Issue.Labels...),
		Blockers:        append([]string(nil), fixture.Status.Blockers...),
		NextAction:      fixture.Expected.NextAction,
		ChecksStatus:    fixture.Status.ChecksStatus,
		ReviewStatus:    fixture.Status.ReviewStatus,
		TicketReadiness: &readiness,
	}
	if fixture.Status.NextAction != "" {
		status.NextAction = fixture.Status.NextAction
	}
	if fixture.Status.PRNumber > 0 {
		status.PRNumber = fixture.Status.PRNumber
		status.PRState = fixture.Status.PRState
		status.PullRequest = &TicketStatusPullRequest{Available: true, Number: fixture.Status.PRNumber, State: fixture.Status.PRState}
		status.PRReadiness = &PRReadinessReport{SchemaVersion: PRReadinessSchemaVersion, Repo: fixture.Repo, Issue: fixture.Issue.Number, PullRequest: fixture.Status.PRNumber, Readiness: "needs_review", NextAction: "request_review"}
	}
	if fixture.Status.FinishReady {
		status.Evidence = &TicketStatusEvidence{ClosingReference: true, BranchTrusted: true, FinishReady: true, Sources: []string{"benchmark_fixture"}}
		status.PRReadiness = &PRReadinessReport{SchemaVersion: PRReadinessSchemaVersion, Repo: fixture.Repo, Issue: fixture.Issue.Number, PullRequest: fixture.Status.PRNumber, Readiness: "ready_for_finish", NextAction: "finish_ticket"}
	}
	return status
}

func benchmarkQueueItem(queues WorkspaceQueuesReport, queue string) (WorkspaceQueueItem, bool) {
	var items []WorkspaceQueueItem
	switch queue {
	case "agent_ready":
		items = queues.Queues.AgentReady
	case "review_needed":
		items = queues.Queues.ReviewNeeded
	case "finish_ready":
		items = queues.Queues.FinishReady
	case "blocked":
		items = queues.Queues.Blocked
	case "failed_check":
		items = queues.Queues.FailedCheck
	case "human_decision":
		items = queues.Queues.HumanDecision
	}
	if len(items) == 0 {
		return WorkspaceQueueItem{}, false
	}
	return items[0], true
}
