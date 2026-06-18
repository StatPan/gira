package gira

import (
	"strings"
	"testing"
)

func TestGoalNewDryRunRendersStructuredBody(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &ticketNewRunner{outputs: ticketNewLabelOutputs("type:epic", "status:ready", "priority:p1", "lane:agent")}

	report, err := BuildGoalNewReport(GoalNewInput{
		Repo:           repo,
		Title:          "Ship goal mode",
		Objective:      "Make long-running agent work inspectable",
		Direction:      "Prefer CLI-first evidence",
		Scope:          "Goal commands only",
		Autonomy:       "lane:agent until finish",
		Decomposition:  []string{"Add goal new", "Update command docs"},
		QualityBar:     []string{"go test ./..."},
		StopConditions: []string{"missing labels"},
		Type:           "epic",
		Priority:       "p1",
		Labels:         []string{"lane:agent"},
		DryRun:         true,
	}, runner)
	if err != nil {
		t.Fatalf("BuildGoalNewReport error: %v", err)
	}
	for _, want := range []string{"## Goal", "Make long-running agent work inspectable", "## Direction", "Prefer CLI-first evidence", "## Scope", "Goal commands only", "## Autonomy", "lane:agent until finish", "## Decomposition", "- Add goal new", "## Quality Bar", "- go test ./...", "## Stop Conditions", "- missing labels", "## Child Tickets", ProvenanceBlockStart, ProvenanceBlockEnd} {
		if !strings.Contains(report.Body, want) {
			t.Fatalf("body missing %q:\n%s", want, report.Body)
		}
	}
	for _, want := range []string{"type:epic", "status:ready", "priority:p1", "lane:agent"} {
		if !containsString(report.Labels, want) {
			t.Fatalf("labels missing %q: %+v", want, report.Labels)
		}
	}
	if report.SchemaVersion != GoalNewReportSchemaVersion || report.Command != "goal new" || report.Approval == nil {
		t.Fatalf("unexpected goal new report metadata: %+v", report)
	}
	if report.Approval.CanonicalCommand != "gira goal new" || report.Approval.OutputSchema != GoalNewReportSchemaVersion {
		t.Fatalf("unexpected approval evidence: %+v", report.Approval)
	}
	for _, want := range []string{"gira goal new 'Ship goal mode'", "--body '## Goal\nMake long-running agent work inspectable", "--priority p1", "--label lane:agent", "--apply"} {
		if !strings.Contains(report.Approval.ApplyCommand, want) {
			t.Fatalf("goal new approval command missing %q: %+v", want, report.Approval)
		}
	}
}

func TestGoalNewApplyCreatesIssue(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	outputs := ticketNewLabelOutputs("type:epic", "status:ready")
	body := defaultGoalNewBody("Ship goal mode")
	outputs["gh issue create --repo StatPan/gira --title Ship goal mode --body "+body+" --label type:epic --label status:ready"] = []byte("https://github.com/StatPan/gira/issues/521\n")
	runner := &ticketNewRunner{outputs: outputs}

	report, err := BuildGoalNewReport(GoalNewInput{Repo: repo, Title: "Ship goal mode"}, runner)
	if err != nil {
		t.Fatalf("BuildGoalNewReport error: %v", err)
	}
	if report.Created.Number != 521 || report.NextStep != "gira goal status 521 --repo StatPan/gira --json" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Approval != nil {
		t.Fatalf("apply output should not include dry-run approval evidence: %+v", report.Approval)
	}
}

func TestGoalNewUsesFullBody(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	body := "## Goal\nUse exact goal packet\n\n## Scope\nPreserved"
	runner := &ticketNewRunner{outputs: ticketNewLabelOutputs("type:goal", "status:ready")}

	report, err := BuildGoalNewReport(GoalNewInput{Repo: repo, Title: "Exact", Body: body, Type: "goal", DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildGoalNewReport error: %v", err)
	}
	if report.Body != body || report.Type != "goal" || !containsString(report.Labels, "type:goal") {
		t.Fatalf("unexpected full-body goal report: %+v", report)
	}
}

func TestGoalNewRejectsInvalidTypeAndPriority(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	if _, err := BuildGoalNewReport(GoalNewInput{Repo: repo, Title: "x", Type: "task", DryRun: true}, &ticketNewRunner{}); err == nil || !strings.Contains(err.Error(), "--type") {
		t.Fatalf("expected type error, got %v", err)
	}
	if _, err := BuildGoalNewReport(GoalNewInput{Repo: repo, Title: "x", Type: "epic", Priority: "high", DryRun: true}, &ticketNewRunner{}); err == nil || !strings.Contains(err.Error(), "--priority") {
		t.Fatalf("expected priority error, got %v", err)
	}
}

func defaultGoalNewBody(title string) string {
	return "## Goal\n" + title + "\n\n## Direction\n_No response_\n\n## Scope\n_No response_\n\n## Autonomy\n_No response_\n\n## Decomposition\n_No response_\n\n## Quality Bar\n_No response_\n\n## Stop Conditions\n_No response_\n\n## Child Tickets\n_No child tickets yet._\n\n" + DefaultProvenanceBlock() + "\n"
}
