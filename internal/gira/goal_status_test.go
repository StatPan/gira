package gira

import (
	"strings"
	"testing"
)

func TestBuildGoalStatusReportNoChildrenPlansChildren(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalStatusFixtureRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/100":                  `{"number":100,"title":"Goal mode","state":"open","body":"## Goal\nShip goal mode","labels":[{"name":"type:epic"},{"name":"status:ready"}]}`,
		"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[]}`,
	}}

	report, err := BuildGoalStatusReport(GoalStatusInput{Repo: repo, Goal: 100}, runner)
	if err != nil {
		t.Fatalf("BuildGoalStatusReport error: %v", err)
	}
	if report.SchemaVersion != GoalStatusSchemaVersion || report.Goal.Number != 100 || report.PlanningEngine != "unconfigured" {
		t.Fatalf("unexpected report metadata: %+v", report)
	}
	if len(report.Children) != 0 || report.NextAction != "plan_children" || report.RemainingAutonomousWork != 0 {
		t.Fatalf("unexpected no-child report: %+v", report)
	}
	if !strings.Contains(FormatGoalStatus(report), "next=plan_children") {
		t.Fatalf("formatted report missing next action:\n%s", FormatGoalStatus(report))
	}
}

func TestGoalPlanningEngineMakesMixedSourcesVisible(t *testing.T) {
	validGraph := "## Work Graph\n```json\n{\"schema_version\":\"pm-work-graph-source/v1\",\"nodes\":[{\"id\":\"delivery\",\"title\":\"Deliver\",\"purpose\":\"Ship the slice\",\"profile\":\"delivery\",\"parent_outcome\":\"goal:100\",\"verification\":[{\"method\":\"go test ./...\",\"evidence\":\"passing tests\"}]}]}\n```"
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{name: "default placeholders", body: "## Decomposition\n_No response_\n\n## Work Graph\n_No response_", want: "unconfigured"},
		{name: "placeholder list", body: "## Goal Plan\n- _No response_", want: "unconfigured"},
		{name: "malformed graph", body: "## Work Graph\n```json\n{}\n```", want: "invalid_typed_work_graph"},
		{name: "empty graph section", body: "## Work Graph", want: "invalid_typed_work_graph"},
		{name: "valid bullet plan", body: "## Goal Plan\n- Deliver the bounded slice", want: "bullet_goal_plan"},
		{name: "valid typed", body: validGraph, want: "typed_work_graph"},
		{name: "mixed", body: "## Goal Plan\n- Deliver the bounded slice\n\n" + validGraph, want: "mixed"},
		{name: "mixed malformed graph", body: "## Goal Plan\n- Deliver the bounded slice\n\n## Work Graph\n```json\n{}\n```", want: "mixed_invalid_typed_work_graph"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := goalPlanningEngine(tc.body); got != tc.want {
				t.Fatalf("planning engine = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildGoalStatusReportIncludesNativeChildren(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalStatusFixtureRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/100": `{"number":100,"title":"Goal mode","state":"open","body":"## Goal\nShip goal mode","labels":[{"name":"type:epic"},{"name":"status:ready"}]}`,
		"gh api repos/StatPan/gira/issues/100/sub_issues -X GET -H Accept: application/vnd.github+json -H X-GitHub-Api-Version: 2026-03-10 -f per_page=100": `[{"number":101,"title":"Native child","state":"open"}]`,
		"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[]}`,
		"gh api repos/StatPan/gira/issues/101":                  `{"number":101,"title":"Native child","state":"open","body":"## Goal\nReady\n\n## Acceptance Criteria\n- done","labels":[{"name":"type:task"},{"name":"status:ready"}]}`,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 101 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": `[]`,
	}}

	report, err := BuildGoalStatusReport(GoalStatusInput{Repo: repo, Goal: 100}, runner)
	if err != nil {
		t.Fatalf("BuildGoalStatusReport error: %v", err)
	}
	if len(report.Children) != 1 || report.Children[0].Number != 101 || report.Children[0].RelationSource != GoalChildRelationSourceGitHubSubIssue || report.NextAction != "start_next_child" {
		t.Fatalf("native child was not reflected in goal status: %+v", report)
	}
}

func TestDiscoverGoalChildRefsMergesNativeAndTypedEvidence(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalStatusFixtureRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/100/sub_issues -X GET -H Accept: application/vnd.github+json -H X-GitHub-Api-Version: 2026-03-10 -f per_page=100": `[{"number":101,"title":"Duplicate native","state":"open"},{"number":102,"title":"Native only","state":"open"}]`,
		"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[{"body":"Created child tickets: #102 and #103\n<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=101 -->\n<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=103 -->"}]}`,
	}}

	refs, err := discoverGoalChildRefs(repo, devStartIssue{Number: 100, Body: "## Child tickets\n- #101\n<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=103 -->"}, runner)
	if err != nil {
		t.Fatalf("discoverGoalChildRefs error: %v", err)
	}
	if len(refs) != 3 {
		t.Fatalf("child refs = %+v, want 3 unique refs", refs)
	}
	for i, want := range []int{101, 102, 103} {
		if refs[i].Repo != repo || refs[i].Number != want {
			t.Fatalf("child refs = %+v, want ordered native/typed union", refs)
		}
	}
	if refs[0].RelationSource != GoalChildRelationSourceGitHubSubIssue || refs[1].RelationSource != GoalChildRelationSourceGitHubSubIssue || refs[2].RelationSource != GoalChildRelationSourceGiraGoalChildLink {
		t.Fatalf("unexpected relation sources: %+v", refs)
	}
}

func TestBuildGoalStatusReportSummarizesMixedChildren(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalStatusFixtureRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/100":                  `{"number":100,"title":"Goal mode","state":"open","body":"## Goal\nShip goal mode","labels":[{"name":"type:epic"},{"name":"status:ready"}]}`,
		"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[{"body":"<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=101 -->\n<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=102 -->"}]}`,
		"gh api repos/StatPan/gira/issues/101":                  `{"number":101,"title":"Ready child","state":"open","body":"## Goal\nReady\n\n## Acceptance Criteria\n- done","labels":[{"name":"type:task"},{"name":"status:ready"}]}`,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 101 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": `[]`,
		"gh api repos/StatPan/gira/issues/102": `{"number":102,"title":"Done child","state":"closed","body":"## Goal\nDone\n\n## Acceptance Criteria\n- done","labels":[{"name":"type:task"},{"name":"status:done"}]}`,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 102 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": `[]`,
	}}

	report, err := BuildGoalStatusReport(GoalStatusInput{Repo: repo, Goal: 100}, runner)
	if err != nil {
		t.Fatalf("BuildGoalStatusReport error: %v", err)
	}
	if len(report.Children) != 2 || report.Counts["ready"] != 1 || report.Counts["done"] != 1 || report.RemainingAutonomousWork != 1 {
		t.Fatalf("unexpected child summary: %+v", report)
	}
	if len(report.Blockers) != 0 || len(report.Children[0].Blockers) != 0 || len(report.Children[1].Blockers) != 0 {
		t.Fatalf("ready/done child reports should not promote missing PR warnings to goal blockers: %+v", report)
	}
	if report.NextAction != "start_next_child" {
		t.Fatalf("next action = %q, want start_next_child", report.NextAction)
	}
	text := FormatGoalStatus(report)
	for _, want := range []string{"children=2", "ready=1", "done=1", "next step: gira goal next"} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted report missing %q:\n%s", want, text)
		}
	}
}

func TestBuildGoalStatusReportSummarizesBlockedChild(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalStatusFixtureRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/100":                  `{"number":100,"title":"Goal mode","state":"open","body":"## Goal\nShip goal mode","labels":[{"name":"type:epic"},{"name":"status:ready"}]}`,
		"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[{"body":"<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=103 -->"}]}`,
		"gh api repos/StatPan/gira/issues/103":                  `{"number":103,"title":"Blocked child","state":"open","body":"## Goal\nBlocked\n\n## Acceptance Criteria\n- done","labels":[{"name":"type:task"},{"name":"status:blocked"}]}`,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 103 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": `[]`,
	}}

	report, err := BuildGoalStatusReport(GoalStatusInput{Repo: repo, Goal: 100}, runner)
	if err != nil {
		t.Fatalf("BuildGoalStatusReport error: %v", err)
	}
	if report.Counts["blocked"] != 1 || report.NextAction != "resolve_blockers" || !containsString(report.Blockers, "child_103:blocked") {
		t.Fatalf("unexpected blocked summary: %+v", report)
	}
}

func TestBuildGoalStatusReportAllDoneFromGoalBody(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalStatusFixtureRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/100":                  `{"number":100,"title":"Goal mode","state":"open","body":"## Goal\nShip goal mode\n\n## Child tickets\n- #201\n- #202\n<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=201 -->\n<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=202 -->","labels":[{"name":"type:epic"},{"name":"status:ready"}]}`,
		"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[]}`,
		"gh api repos/StatPan/gira/issues/201":                  `{"number":201,"title":"Done one","state":"closed","body":"## Goal\nDone\n\n## Acceptance Criteria\n- done","labels":[{"name":"type:task"},{"name":"status:done"}]}`,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 201 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": `[]`,
		"gh api repos/StatPan/gira/issues/202": `{"number":202,"title":"Done two","state":"closed","body":"## Goal\nDone\n\n## Acceptance Criteria\n- done","labels":[{"name":"type:task"},{"name":"status:done"}]}`,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 202 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": `[]`,
	}}

	report, err := BuildGoalStatusReport(GoalStatusInput{Repo: repo, Goal: 100}, runner)
	if err != nil {
		t.Fatalf("BuildGoalStatusReport error: %v", err)
	}
	if report.Counts["done"] != 2 || report.RemainingAutonomousWork != 0 || report.NextAction != "finish_goal" {
		t.Fatalf("unexpected all-done summary: %+v", report)
	}
}

func TestBuildGoalStatusReportAllDoneWithHandoffStopsForHumanReview(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalStatusFixtureRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/100":                  `{"number":100,"title":"Goal mode","state":"open","body":"## Goal\nShip goal mode\n\n## Child tickets\n- #201\n<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=201 -->","labels":[{"name":"type:epic"},{"name":"status:ready"}]}`,
		"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[{"body":"## Goal Finish Receipt\n\n- Schema: goal-finish-receipt/v1"}]}`,
		"gh api repos/StatPan/gira/issues/201":                  `{"number":201,"title":"Done one","state":"closed","body":"## Goal\nDone\n\n## Acceptance Criteria\n- done","labels":[{"name":"type:task"},{"name":"status:done"}]}`,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 201 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": `[]`,
	}}

	report, err := BuildGoalStatusReport(GoalStatusInput{Repo: repo, Goal: 100}, runner)
	if err != nil {
		t.Fatalf("BuildGoalStatusReport error: %v", err)
	}
	if !report.HandoffReceiptPresent || report.RemainingAutonomousWork != 0 || report.NextAction != "human_review" {
		t.Fatalf("unexpected handoff summary: %+v", report)
	}
	if !strings.Contains(report.NextStep, "goal-finish-receipt/v1") {
		t.Fatalf("next step should point to handoff receipt: %q", report.NextStep)
	}
}

func TestBuildGoalStatusReportClosedDoneGoalIsDone(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalStatusFixtureRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/100":                  `{"number":100,"title":"Goal mode","state":"closed","body":"## Goal\nShip goal mode\n\n## Child tickets\n- #201\n<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=201 -->","labels":[{"name":"type:epic"},{"name":"status:done"}]}`,
		"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[{"body":"## Goal Finish Receipt\n\n- Schema: goal-finish-receipt/v1\n- Terminal recommendation: done"}]}`,
		"gh api repos/StatPan/gira/issues/201":                  `{"number":201,"title":"Done one","state":"closed","body":"## Goal\nDone\n\n## Acceptance Criteria\n- done","labels":[{"name":"type:task"},{"name":"status:done"}]}`,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 201 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": `[]`,
	}}

	report, err := BuildGoalStatusReport(GoalStatusInput{Repo: repo, Goal: 100}, runner)
	if err != nil {
		t.Fatalf("BuildGoalStatusReport error: %v", err)
	}
	if !report.HandoffReceiptPresent || report.RemainingAutonomousWork != 0 || report.NextAction != "done" || report.NextStep != "goal is done" {
		t.Fatalf("closed done goal should be terminal done: %+v", report)
	}
}

func TestBuildGoalStatusReportIncludesCrossRepoChildren(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "backlog"}
	runner := goalStatusFixtureRunner{responses: map[string]string{
		"gh api repos/StatPan/backlog/issues/100":                  `{"number":100,"title":"Goal mode","state":"open","body":"## Goal\nShip cross repo goal\n\n## Child tickets\n- StatPan/gira#201\n<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=201 -->","labels":[{"name":"type:epic"},{"name":"status:ready"}]}`,
		"gh issue view 100 --repo StatPan/backlog --json comments": `{"comments":[{"body":"Created child tickets:\n- StatPan/agentree#202\n<!-- gira:goal-child-link/v1 repo=StatPan/agentree issue=202 -->"}]}`,
		"gh api repos/StatPan/gira/issues/201":                     `{"number":201,"title":"Gira child","state":"open","body":"## Goal\nGira\n\n## Acceptance Criteria\n- done","labels":[{"name":"type:task"},{"name":"status:ready"}]}`,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 201 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": `[]`,
		"gh api repos/StatPan/agentree/issues/202": `{"number":202,"title":"Agentree child","state":"closed","body":"## Goal\nAgentree\n\n## Acceptance Criteria\n- done","labels":[{"name":"type:task"},{"name":"status:done"}]}`,
		"gh pr list --repo StatPan/agentree --state all --search repo:StatPan/agentree is:pr 202 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": `[]`,
	}}

	report, err := BuildGoalStatusReport(GoalStatusInput{Repo: repo, Goal: 100}, runner)
	if err != nil {
		t.Fatalf("BuildGoalStatusReport error: %v", err)
	}
	if len(report.Children) != 2 || report.Children[0].Repo != "StatPan/agentree" || report.Children[1].Repo != "StatPan/gira" {
		t.Fatalf("unexpected cross-repo children: %+v", report.Children)
	}
	if report.Children[0].RelationSource != GoalChildRelationSourceGiraGoalChildLink || report.Children[1].RelationSource != GoalChildRelationSourceGiraGoalChildLink {
		t.Fatalf("typed child links must expose provenance: %+v", report.Children)
	}
	if report.Counts["ready"] != 1 || report.Counts["done"] != 1 {
		t.Fatalf("unexpected cross-repo counts: %+v", report.Counts)
	}
}

func TestGoalChildRefsFromTypedLinksIgnoresProseAndMalformedMarkers(t *testing.T) {
	refs := goalChildRefsFromTypedLinks(`
Grounding-gap parent: #160
## Child tickets
- #201
<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=202 -->
<!-- gira:goal-child-link/v1 issue=203 repo=StatPan/gira -->
<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=not-a-number -->
<!-- gira:goal-child-link/v1 repo=not-a-repo issue=204 -->`)
	if len(refs) != 1 || refs[0].Repo.FullName() != "StatPan/gira" || refs[0].Number != 202 || refs[0].RelationSource != GoalChildRelationSourceGiraGoalChildLink {
		t.Fatalf("typed refs = %+v, want exactly the valid typed link", refs)
	}
}

func TestDiscoverGoalChildRefsIgnoresLegacyParentAndProseReferences(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalStatusFixtureRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/100/sub_issues -X GET -H Accept: application/vnd.github+json -H X-GitHub-Api-Version: 2026-03-10 -f per_page=100": `[{"number":101,"title":"Native","state":"open"}]`,
		"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[{"body":"Goal planning says #102 is next. Parent: #100. StatPan/gira#103."}]}`,
	}}
	refs, err := discoverGoalChildRefs(repo, devStartIssue{Number: 100, Body: "## Child tickets\n- #102\nGrounding-gap parent: #160\nParent: #100"}, runner)
	if err != nil {
		t.Fatalf("discoverGoalChildRefs error: %v", err)
	}
	if len(refs) != 1 || refs[0].Number != 101 || refs[0].RelationSource != GoalChildRelationSourceGitHubSubIssue {
		t.Fatalf("legacy/prose references must be ignored: %+v", refs)
	}
}
