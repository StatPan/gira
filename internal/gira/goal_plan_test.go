package gira

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func TestBuildGoalPlanReportValidPlan(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalPlanRunner(goalPlanGoalJSON(100, goalPlanBody("## Goal\nShip goal mode\n\n## Scope\nCLI goal planning\n\n## Goal Plan\n- Add goal plan JSON\n- Add goal plan text\n", ""), []string{"type:epic", "priority:p1", "area:backend", "status:ready"}), `[]`, `{"comments":[]}`, nil)

	report, err := BuildGoalPlanReport(GoalPlanInput{Repo: repo, Goal: 100, DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildGoalPlanReport error: %v", err)
	}
	if report.SchemaVersion != GoalPlanSchemaVersion || len(report.ProposedTickets) != 2 || len(report.StopConditions) != 0 {
		t.Fatalf("unexpected plan report: %+v", report)
	}
	first := report.ProposedTickets[0]
	if first.ParentGoal != 100 || !strings.Contains(first.Body, "Parent: #100") || len(first.Acceptance) == 0 || len(first.ExpectedEvidence) == 0 {
		t.Fatalf("proposed ticket is incomplete: %+v", first)
	}
	if first.TicketReadiness.Readiness != "ready" {
		t.Fatalf("ticket readiness = %+v", first.TicketReadiness)
	}
	if !strings.Contains(FormatGoalPlan(report), "proposed=2") {
		t.Fatalf("formatted report missing proposal count:\n%s", FormatGoalPlan(report))
	}
}

func TestBuildGoalPlanReportMissingDirectionStops(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalPlanRunner(goalPlanGoalJSON(100, "## Goal\nShip goal mode\n", []string{"type:epic", "status:ready"}), `[]`, `{"comments":[]}`, nil)

	report, err := BuildGoalPlanReport(GoalPlanInput{Repo: repo, Goal: 100, DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildGoalPlanReport error: %v", err)
	}
	for _, want := range []string{"missing_scope", "missing_decomposition_notes"} {
		if !containsString(report.StopConditions, want) {
			t.Fatalf("stop conditions missing %s: %+v", want, report.StopConditions)
		}
	}
	if report.NextAction != "ask_human" || len(report.ProposedTickets) != 0 {
		t.Fatalf("unexpected missing-direction report: %+v", report)
	}
}

func TestBuildGoalPlanReportDedupesExistingChildren(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalPlanRunner(
		goalPlanGoalJSON(100, goalPlanBody("## Goal\nShip goal mode\n\n## Scope\nCLI goal planning\n\n## Goal Plan\n- Add API\n- Add CLI\n- State-aware ticket resolver hardening\n", ""), []string{"type:epic", "status:ready"}),
		`[{"number":101},{"number":102}]`,
		`{"comments":[]}`,
		map[string]string{
			"gh api repos/StatPan/gira/issues/101": `{"number":101,"title":"[Task] Add API","state":"open","body":"## Goal\nAdd API\n\n## Scope\nAPI\n\n## Acceptance Criteria\n- done","labels":[{"name":"type:task"},{"name":"status:ready"}]}`,
			"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 101 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": `[]`,
			"gh api repos/StatPan/gira/issues/102": `{"number":102,"title":"[Task] Harden state-aware ticket and PR resolver","state":"closed","body":"## Goal\nResolver\n\n## Scope\nState aware ticket resolver\n\n## Acceptance Criteria\n- done","labels":[{"name":"type:task"},{"name":"status:done"}]}`,
			"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 102 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": `[]`,
		},
	)

	report, err := BuildGoalPlanReport(GoalPlanInput{Repo: repo, Goal: 100, DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildGoalPlanReport error: %v", err)
	}
	if len(report.ProposedTickets) != 1 || report.ProposedTickets[0].Title != "[Task] Add CLI" {
		t.Fatalf("unexpected proposals: %+v", report.ProposedTickets)
	}
	if len(report.SkippedCandidates) != 2 || report.SkippedCandidates[0].DuplicateOf != 101 || report.SkippedCandidates[1].DuplicateOf != 102 {
		t.Fatalf("unexpected skipped candidates: %+v", report.SkippedCandidates)
	}
}

func TestBuildGoalPlanReportBlockedHumanDecision(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	body := goalPlanBody("## Goal\nShip goal mode\n\n## Scope\nCLI goal planning\n\n## Goal Plan\n- Add API\n\n## Human Decision\nChoose provider policy first\n", "")
	runner := goalPlanRunner(goalPlanGoalJSON(100, body, []string{"type:epic", "status:ready"}), `[]`, `{"comments":[]}`, nil)

	report, err := BuildGoalPlanReport(GoalPlanInput{Repo: repo, Goal: 100, DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildGoalPlanReport error: %v", err)
	}
	if !containsString(report.StopConditions, "human_decision_required") || report.NextAction != "ask_human" {
		t.Fatalf("unexpected human decision report: %+v", report)
	}
}

func TestBuildGoalPlanReportAmbiguousTargetRepo(t *testing.T) {
	report, err := BuildGoalPlanReport(GoalPlanInput{Goal: 100, DryRun: true}, onboardFakeRunner{})
	if err != nil {
		t.Fatalf("BuildGoalPlanReport error: %v", err)
	}
	if !containsString(report.StopConditions, "ambiguous_target_repo") || report.NextStep != "rerun with --repo OWNER/REPO" {
		t.Fatalf("unexpected ambiguous repo report: %+v", report)
	}
}

func TestBuildGoalPlanReportApplyCreatesChildren(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &goalPlanApplyRunner{
		responses: map[string]string{
			"gh api repos/StatPan/gira/issues/100": goalPlanGoalJSON(100, goalPlanBody("## Goal\nShip goal mode\n\n## Scope\nCLI goal planning\n\n## Goal Plan\n- Add goal plan JSON\n- Add goal plan text\n", ""), []string{"type:epic", "priority:p1", "area:backend", "status:ready"}),
			`gh issue list --repo StatPan/gira --state all --search repo:StatPan/gira is:issue "Parent: #100" --json number,title,state,url --limit 100`: `[]`,
			"gh issue view 100 --repo StatPan/gira --json comments":      `{"comments":[]}`,
			"gh label list --repo StatPan/gira --json name --limit 1000": goalPlanLabelListJSON("type:task", "status:ready", "priority:p1", "area:backend"),
		},
	}

	report, err := BuildGoalPlanReport(GoalPlanInput{Repo: repo, Goal: 100, Apply: true}, runner)
	if err != nil {
		t.Fatalf("BuildGoalPlanReport apply error: %v", err)
	}
	if !report.Apply || report.DryRun || len(report.CreatedChildren) != 2 || len(runner.creates) != 2 {
		t.Fatalf("unexpected apply report=%+v creates=%d", report, len(runner.creates))
	}
	if report.CreatedChildren[0].Number != 700 || report.Actions[0].Status != "applied" || report.NextAction != "inspect_goal" {
		t.Fatalf("unexpected created/action state: report=%+v", report)
	}
	firstCreate := strings.Join(runner.creates[0], "\x00")
	for _, want := range []string{"--title\x00[Task] Add goal plan JSON", "--body\x00## Goal\nAdd goal plan JSON\n\nParent: #100", "--label\x00type:task", "--label\x00status:ready", "--label\x00priority:p1", "--label\x00area:backend", "--milestone\x002.0 RC - Goal Mode"} {
		if !strings.Contains(firstCreate, want) {
			t.Fatalf("created issue args missing %q:\n%q", want, firstCreate)
		}
	}
	text := FormatGoalPlan(report)
	for _, want := range []string{"created=2", "- created #700 [Task] Add goal plan JSON", "next step: gira goal status --repo StatPan/gira --goal 100 --json"} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted apply report missing %q:\n%s", want, text)
		}
	}
}

func TestBuildGoalPlanReportApplyStopsWithoutMutation(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &goalPlanApplyRunner{
		responses: map[string]string{
			"gh api repos/StatPan/gira/issues/100": goalPlanGoalJSON(100, "## Goal\nShip goal mode\n", []string{"type:epic", "status:ready"}),
			`gh issue list --repo StatPan/gira --state all --search repo:StatPan/gira is:issue "Parent: #100" --json number,title,state,url --limit 100`: `[]`,
			"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[]}`,
		},
	}

	report, err := BuildGoalPlanReport(GoalPlanInput{Repo: repo, Goal: 100, Apply: true}, runner)
	if err != nil {
		t.Fatalf("BuildGoalPlanReport apply stop error: %v", err)
	}
	if len(runner.creates) != 0 || !containsString(report.StopConditions, "missing_scope") || !containsString(report.StopConditions, "missing_decomposition_notes") {
		t.Fatalf("apply should stop without mutation: report=%+v creates=%d", report, len(runner.creates))
	}
}

func TestBuildGoalPlanReportApplySkipsDuplicateChildren(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &goalPlanApplyRunner{
		responses: map[string]string{
			"gh api repos/StatPan/gira/issues/100": goalPlanGoalJSON(100, goalPlanBody("## Goal\nShip goal mode\n\n## Scope\nCLI goal planning\n\n## Goal Plan\n- Add API\n- Add CLI\n", ""), []string{"type:epic", "priority:p1", "area:backend", "status:ready"}),
			`gh issue list --repo StatPan/gira --state all --search repo:StatPan/gira is:issue "Parent: #100" --json number,title,state,url --limit 100`: `[{"number":101}]`,
			"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[]}`,
			"gh api repos/StatPan/gira/issues/101":                  `{"number":101,"title":"[Task] Add API","state":"open","body":"## Goal\nAdd API\n\n## Scope\nAPI\n\n## Acceptance Criteria\n- done","labels":[{"name":"type:task"},{"name":"status:ready"}]}`,
			"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 101 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": `[]`,
			"gh label list --repo StatPan/gira --json name --limit 1000": goalPlanLabelListJSON("type:task", "status:ready", "priority:p1", "area:backend"),
		},
	}

	report, err := BuildGoalPlanReport(GoalPlanInput{Repo: repo, Goal: 100, Apply: true}, runner)
	if err != nil {
		t.Fatalf("BuildGoalPlanReport apply error: %v", err)
	}
	if len(runner.creates) != 1 || len(report.CreatedChildren) != 1 || report.CreatedChildren[0].Title != "[Task] Add CLI" {
		t.Fatalf("apply should create only non-duplicates: report=%+v creates=%d", report, len(runner.creates))
	}
	if len(report.SkippedCandidates) != 1 || report.SkippedCandidates[0].DuplicateOf != 101 {
		t.Fatalf("duplicate child was not reported: %+v", report.SkippedCandidates)
	}
	if report.Actions[0].Status != "skipped" || report.Actions[1].Status != "applied" {
		t.Fatalf("unexpected duplicate/apply actions: %+v", report.Actions)
	}
}

func goalPlanRunner(goalJSON string, childrenJSON string, commentsJSON string, extra map[string]string) onboardFakeRunner {
	responses := map[string]string{
		"gh api repos/StatPan/gira/issues/100": goalJSON,
		`gh issue list --repo StatPan/gira --state all --search repo:StatPan/gira is:issue "Parent: #100" --json number,title,state,url --limit 100`: childrenJSON,
		"gh issue view 100 --repo StatPan/gira --json comments": commentsJSON,
	}
	for key, value := range extra {
		responses[key] = value
	}
	return onboardFakeRunner{responses: responses}
}

type goalPlanApplyRunner struct {
	responses map[string]string
	creates   [][]string
}

func (r *goalPlanApplyRunner) Run(name string, args ...string) ([]byte, error) {
	if name == "gh" && len(args) >= 2 && args[0] == "issue" && args[1] == "create" {
		r.creates = append(r.creates, append([]string{}, args...))
		return []byte(fmt.Sprintf("https://github.com/StatPan/gira/issues/%d\n", 699+len(r.creates))), nil
	}
	key := strings.TrimSpace(name + " " + strings.Join(args, " "))
	response, ok := r.responses[key]
	if !ok {
		return nil, fmt.Errorf("unexpected command: %s", key)
	}
	return []byte(response), nil
}

func goalPlanGoalJSON(number int, body string, labels []string) string {
	labelJSON := []string{}
	for _, label := range labels {
		labelJSON = append(labelJSON, `{"name":"`+label+`"}`)
	}
	bodyJSON, _ := json.Marshal(body)
	return `{"number":` + strconv.Itoa(number) + `,"title":"Goal mode","state":"open","body":` + string(bodyJSON) + `,"labels":[` + strings.Join(labelJSON, ",") + `],"milestone":{"title":"2.0 RC - Goal Mode"}}`
}

func goalPlanBody(body string, suffix string) string {
	return body + suffix
}

func goalPlanLabelListJSON(labels ...string) string {
	parts := make([]string, 0, len(labels))
	for _, label := range labels {
		parts = append(parts, `{"name":"`+label+`"}`)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
