package gira

import (
	"strings"
	"testing"
)

func TestBuildGoalDossierReportSummarizesGoalState(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := onboardFakeRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/100": `{"number":100,"title":"Goal mode","state":"open","body":"## Goal\nShip goal mode","labels":[{"name":"type:epic"},{"name":"status:ready"}]}`,
		`gh issue list --repo StatPan/gira --state all --search repo:StatPan/gira is:issue "Parent: #100" --json number,title,state,url --limit 100`: `[{"number":101},{"number":102}]`,
		"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[]}`,
		"gh api repos/StatPan/gira/issues/101":                  `{"number":101,"title":"Ready child","state":"open","body":"## Goal\nReady\n\n## Acceptance Criteria\n- done","labels":[{"name":"type:task"},{"name":"status:ready"}]}`,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 101 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": `[]`,
		"gh api repos/StatPan/gira/issues/102": `{"number":102,"title":"Done child","state":"closed","body":"## Goal\nDone\n\n## Acceptance Criteria\n- done","labels":[{"name":"type:task"},{"name":"status:done"}]}`,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 102 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": `[]`,
	}}

	report, err := BuildGoalDossierReport(GoalDossierInput{Repo: repo, Goal: 100}, runner)
	if err != nil {
		t.Fatalf("BuildGoalDossierReport error: %v", err)
	}
	if report.SchemaVersion != GoalDossierSchemaVersion || report.Command != "goal dossier" || report.GeneratedAt == "" {
		t.Fatalf("unexpected dossier metadata: %+v", report)
	}
	if report.Counts["total"] != 2 || len(report.ChildGroups) != 2 || report.ChildGroups[0].Category != "ready" || report.ChildGroups[1].Category != "done" {
		t.Fatalf("unexpected child grouping: %+v", report.ChildGroups)
	}
	if report.SelectedTicket == nil || report.SelectedTicket.Number != 101 || report.NextAction != "start_child" {
		t.Fatalf("unexpected next selection: %+v", report)
	}
	if report.Evidence.ChildCount != 2 || report.Evidence.RemainingAutonomousWork != 1 || report.Evidence.Checks.Total != 2 {
		t.Fatalf("unexpected evidence summary: %+v", report.Evidence)
	}
	for _, want := range []string{"goal dossier: #100 children=2 remaining=1 next=start_child", "children: ready=1 done=1", "selected: #101 next_ready_child"} {
		if !strings.Contains(FormatGoalDossier(report), want) {
			t.Fatalf("formatted dossier missing %q:\n%s", want, FormatGoalDossier(report))
		}
	}
}

func TestBuildGoalDossierReportCarriesStopConditions(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := onboardFakeRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/100": `{"number":100,"title":"Goal mode","state":"open","body":"## Goal\nShip goal mode","labels":[{"name":"type:epic"},{"name":"status:ready"}]}`,
		`gh issue list --repo StatPan/gira --state all --search repo:StatPan/gira is:issue "Parent: #100" --json number,title,state,url --limit 100`: `[]`,
		"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[]}`,
	}}

	report, err := BuildGoalDossierReport(GoalDossierInput{Repo: repo, Goal: 100}, runner)
	if err != nil {
		t.Fatalf("BuildGoalDossierReport error: %v", err)
	}
	if !containsString(report.StopConditions, "no_child_tickets") || report.NextAction != "plan_children" {
		t.Fatalf("unexpected no-child dossier: %+v", report)
	}
	if !strings.Contains(FormatGoalDossier(report), "stop: no_child_tickets") {
		t.Fatalf("formatted dossier missing stop condition:\n%s", FormatGoalDossier(report))
	}
}
