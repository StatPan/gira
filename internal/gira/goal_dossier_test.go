package gira

import (
	"os"
	"path/filepath"
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
	measurement := completeQuantitativePlan("Goal activation", "met")
	measurementContext := string(pmLedgerContextJSON(t, "", []pmLedgerTestComment{{Body: RenderPMLedgerRecordComment(discoveryRecord("outcome.goal", "outcome", nil, nil))}, {Body: RenderPMLedgerRecordComment(measurementRecord("measurement.goal", "outcome.goal", measurement))}}))
	runner.responses["gh issue view 100 --repo StatPan/gira --json number,title,body,url,comments"] = strings.Replace(measurementContext, `"number":42`, `"number":100`, 1)

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
	if report.Measurement == nil || report.Measurement.Summary.Validated != 1 || report.Sources[len(report.Sources)-1].SchemaVersion != PMMeasurementReportSchemaVersion {
		t.Fatalf("goal dossier lost measurement evidence: %+v", report.Measurement)
	}
	if !strings.Contains(FormatGoalReport(report), "outcomes: validated=1") || !strings.Contains(RenderGoalReportHTML(report), "outcomes validated") {
		t.Fatal("goal report views omitted measurement evidence")
	}
	for _, want := range []string{"goal report: #100 children=2 remaining=1 next=start_child", "children: ready=1 done=1", "selected: #101 next_ready_child"} {
		if !strings.Contains(FormatGoalReport(report), want) {
			t.Fatalf("formatted report missing %q:\n%s", want, FormatGoalReport(report))
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
	if !strings.Contains(FormatGoalReport(report), "stop: no_child_tickets") {
		t.Fatalf("formatted report missing stop condition:\n%s", FormatGoalReport(report))
	}
}

func TestRenderGoalReportHTMLEscapesUnsafeTextAndLinks(t *testing.T) {
	selected := GoalNextCandidate{
		Number:   102,
		Title:    `Pick <b>me</b>`,
		Category: "ready",
		Reason:   `because <ok>`,
		URL:      "javascript:alert(1)",
	}
	report := BuildGoalDossierReportFromStatus(GoalStatusReport{
		Command:       "goal status",
		SchemaVersion: GoalStatusSchemaVersion,
		Repo:          "StatPan/gira",
		Goal: GoalStatusIssue{
			Number: 100,
			Title:  `Goal <script>alert("x")</script>`,
			State:  "open",
			Status: "Ready",
			URL:    "javascript:alert(1)",
		},
		Children: []GoalStatusChild{
			{
				Number:       101,
				Title:        `Child <b>x</b>`,
				State:        "open",
				Status:       "Ready",
				Category:     "ready",
				URL:          "javascript:evil()",
				ChecksStatus: "passed",
				ReviewStatus: "approved",
				NextAction:   "start_child",
				NextStep:     "gira ticket start <unsafe>",
			},
		},
		Counts:                  map[string]int{"total": 1, "ready": 1},
		NextAction:              "start_child",
		NextStep:                "gira goal next <unsafe>",
		RemainingAutonomousWork: 1,
	}, GoalNextReport{
		Command:        "goal next",
		SchemaVersion:  GoalNextSchemaVersion,
		Repo:           "StatPan/gira",
		Goal:           GoalStatusIssue{Number: 100},
		SelectedTicket: &selected,
		Counts:         map[string]int{"total": 1, "ready": 1},
		NextAction:     "start_child",
		NextStep:       "gira ticket start <unsafe>",
	})

	html := RenderGoalReportHTML(report)
	for _, bad := range []string{`<script>alert`, `<b>x</b>`, `Pick <b>me</b>`, `javascript:alert`, `javascript:evil`} {
		if strings.Contains(html, bad) {
			t.Fatalf("HTML contains unsafe raw value %q:\n%s", bad, html)
		}
	}
	for _, want := range []string{`Goal &lt;script&gt;alert`, `Child &lt;b&gt;x&lt;/b&gt;`, `Pick &lt;b&gt;me&lt;/b&gt;`, `gira ticket start &lt;unsafe&gt;`, GoalDossierSchemaVersion} {
		if !strings.Contains(html, want) {
			t.Fatalf("HTML missing escaped value %q:\n%s", want, html)
		}
	}
}

func TestWriteGoalReportHTMLWritesSafeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reports", "goal-100.html")
	report := GoalDossierReport{
		Command:       "goal report",
		SchemaVersion: GoalDossierSchemaVersion,
		Repo:          "StatPan/gira",
		GeneratedAt:   "2026-05-31T00:00:00Z",
		Goal:          GoalStatusIssue{Number: 100, Title: "Goal mode", State: "open", Status: "Ready"},
		Counts:        map[string]int{"total": 0},
		NextAction:    "plan_children",
		NextStep:      "gira goal plan --repo StatPan/gira --goal 100 --dry-run",
		Evidence:      GoalDossierEvidenceSummary{Sources: []string{"goal_status", "goal_next"}},
	}
	if err := WriteGoalReportHTML(path, report); err != nil {
		t.Fatalf("WriteGoalReportHTML error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report HTML: %v", err)
	}
	for _, want := range []string{"Gira goal report", "Goal mode", GoalDossierSchemaVersion} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("written report missing %q:\n%s", want, string(got))
		}
	}
}
