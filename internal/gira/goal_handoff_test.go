package gira

import (
	"strconv"
	"strings"
	"testing"
)

func TestBuildGoalHandoffReportEmbedsSelectedWorkerHandoff(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	goalBody := "## Goal\nShip goal-level LLM delegation\n\n## Direction\nKeep execution ticket-bounded.\n\n## Scope\nGoal and handoff commands.\n\n## Autonomy\nlane:agent for child implementation only.\n\n## Stop Conditions\n- unclear child acceptance\n\n## Child tickets\n- #201\n<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=201 -->\n"
	childBody := "## Goal\nAdd goal handoff\n\nParent: #100\n\n## Scope\nCLI and JSON report.\n\n## Acceptance Criteria\n- emits goal-handoff/v1\n- embeds worker-handoff/v1\n\n## Expected Evidence\n- go test ./internal/gira\n\n## Expected Delivery\nOpen a PR for review.\n\n" + RenderTicketLifecycleBlock(TicketLifecycleState{BaseBranch: "main", BaseSource: "branch_policy.default", BranchPolicyMode: BranchPolicyModeGitHubFlow, WorkBranch: "issue-201-goal-handoff"})
	runner := onboardFakeRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/100":                  `{"number":100,"title":"LLM delegation goal","state":"open","body":` + strconv.Quote(goalBody) + `,"labels":[{"name":"type:epic"},{"name":"status:ready"},{"name":"lane:agent"}]}`,
		"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[]}`,
		"gh api repos/StatPan/gira/issues/201":                  `{"number":201,"title":"Add goal handoff","state":"open","body":` + strconv.Quote(childBody) + `,"labels":[{"name":"type:task"},{"name":"status:ready"}]}`,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 201 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": `[]`,
	}}

	report, err := BuildGoalHandoffReport(GoalHandoffInput{Repo: repo, Goal: 100, Role: AgentPromptRoleImplementer, Profile: AgentPromptProfileDefault}, runner)
	if err != nil {
		t.Fatalf("BuildGoalHandoffReport error: %v", err)
	}
	if report.SchemaVersion != GoalHandoffSchemaVersion || report.Command != "goal handoff" || report.Goal.Number != 100 {
		t.Fatalf("unexpected report metadata: %+v", report)
	}
	if report.SelectedTicket == nil || report.SelectedTicket.Number != 201 || report.SelectedTicket.Repo != "StatPan/gira" {
		t.Fatalf("unexpected selected ticket: %+v", report.SelectedTicket)
	}
	if report.WorkerHandoff == nil || report.WorkerHandoff.SchemaVersion != WorkerHandoffSchemaVersion || report.WorkerHandoff.Issue != 201 {
		t.Fatalf("missing embedded worker handoff: %+v", report.WorkerHandoff)
	}
	if report.GoalContext.Objective != "Ship goal-level LLM delegation" {
		t.Fatalf("missing goal context: %+v", report.GoalContext)
	}
	if len(report.WorkerHandoff.OperatorContext) < 3 || !strings.Contains(report.WorkerHandoff.OperatorContext[1].Content, "selected child ticket") {
		t.Fatalf("worker handoff missing goal operator context: %+v", report.WorkerHandoff.OperatorContext)
	}
	if report.NextAction != "handoff_child" || !strings.Contains(report.NextSafeCommand, "gira ticket start") {
		t.Fatalf("unexpected next action: %s %s", report.NextAction, report.NextSafeCommand)
	}
	text := FormatGoalHandoff(report)
	for _, want := range []string{"goal handoff: #100 selected=StatPan/gira#201", "worker handoff: schema=worker-handoff/v1", "next safe command:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted goal handoff missing %q:\n%s", want, text)
		}
	}
}

func TestBuildGoalHandoffReportStopsWithoutSelectedChild(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := onboardFakeRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/100":                  `{"number":100,"title":"Empty goal","state":"open","body":"## Goal\nEmpty","labels":[{"name":"type:epic"},{"name":"status:ready"}]}`,
		"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[]}`,
	}}

	report, err := BuildGoalHandoffReport(GoalHandoffInput{Repo: repo, Goal: 100, Role: AgentPromptRoleImplementer, Profile: AgentPromptProfileDefault}, runner)
	if err != nil {
		t.Fatalf("BuildGoalHandoffReport error: %v", err)
	}
	if report.SelectedTicket != nil || report.WorkerHandoff != nil || report.NextAction != "plan_children" {
		t.Fatalf("unexpected stop report: %+v", report)
	}
	if !containsString(report.StopReasons, "no_child_tickets") {
		t.Fatalf("stop reasons missing no_child_tickets: %+v", report.StopReasons)
	}
}
