package gira

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGoalPMViewsShareCanonicalStateAndSeparateDeliveryOutcome(t *testing.T) {
	compile, observe := goalPMViewFixture()
	digests := map[string]bool{}
	for _, kind := range goalPMViewKinds {
		view, err := BuildGoalPMView(kind, compile, observe)
		if err != nil {
			t.Fatal(err)
		}
		if view.SchemaVersion != GoalPMViewSchemaVersion || len(view.Sources) < 7 || view.Delivery.Done != 1 || view.Outcome.Validated != 0 {
			t.Fatalf("view %s conflated or lost canonical state: %#v", kind, view)
		}
		digests[view.StateDigest] = true
	}
	if len(digests) != 1 {
		t.Fatalf("views derived different state digests: %#v", digests)
	}
	changed := observe
	changedStatus := *observe.GoalStatus
	changedStatus.Children = append([]GoalStatusChild(nil), changedStatus.Children...)
	changedStatus.Children[1].Category = "done"
	changed.GoalStatus = &changedStatus
	changedView, err := BuildGoalPMView("operator", compile, changed)
	if err != nil {
		t.Fatal(err)
	}
	for digest := range digests {
		if changedView.StateDigest == digest {
			t.Fatal("canonical state digest did not change with delivery state")
		}
	}
}

func TestGoalPMHumanAIStakeholderAndAuditContracts(t *testing.T) {
	compile, observe := goalPMViewFixture()
	human, _ := BuildGoalPMView("human", compile, observe)
	text := strings.Join(human.Summaries, "\n")
	for _, want := range []string{"Changed:", "Why:", "Learned:", "Next judgment:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("human handoff missing %s: %#v", want, human)
		}
	}
	ai1, _ := BuildGoalPMView("ai", compile, observe)
	ai2, _ := BuildGoalPMView("ai", compile, observe)
	one, _ := json.Marshal(ai1)
	two, _ := json.Marshal(ai2)
	if string(one) != string(two) || ai1.Budget == nil || ai1.Budget.Characters != len(one) || len(one) > 6000 || ai1.ExpandCommand == "" {
		t.Fatalf("AI hydration is unbounded or nondeterministic: bytes=%d view=%#v", len(one), ai1)
	}
	full, _ := json.Marshal(observe)
	if len(one)*2 >= len(full) {
		t.Fatalf("AI view does not meet 50%% compact baseline: compact=%d full=%d", len(one), len(full))
	}
	stakeholder, _ := BuildGoalPMView("stakeholder", compile, observe)
	stakeholderJSON, _ := json.Marshal(stakeholder)
	if strings.Contains(string(stakeholderJSON), "Build bounded slice") || !strings.Contains(string(stakeholderJSON), "Outcome evidence") {
		t.Fatalf("stakeholder view leaks implementation noise or omits outcome: %s", stakeholderJSON)
	}
	audit, _ := BuildGoalPMView("audit", compile, observe)
	if len(audit.Audit) != 3 || audit.Audit[1].ContentDigest == "" || audit.Audit[2].Supersedes == "" || len(audit.References) != 0 {
		t.Fatalf("audit lost provenance/supersession or repeats refs: %#v", audit)
	}
	if len(human.Residual) == 0 || len(human.Residual[0].Options) != 3 || human.Residual[0].Authority == "" || human.Residual[0].Impact == "" || human.Residual[0].ResumeCondition == "" {
		t.Fatalf("human residual decision is not actionable: %#v", human.Residual)
	}
}

func TestGoalPMHTMLViewEscapesSummariesAndOmitsStakeholderTickets(t *testing.T) {
	view := GoalPMView{SchemaVersion: GoalPMViewSchemaVersion, Kind: "stakeholder", StateDigest: "sha256:x", Summaries: []string{"Outcome <script>alert(1)</script>"}}
	report := BuildGoalDossierReportFromStatus(GoalStatusReport{Repo: "OWNER/repo", Goal: GoalStatusIssue{Number: 100, Title: "Goal"}, Children: []GoalStatusChild{{Number: 101, Title: "Implementation secret", Category: "in_progress"}}, Counts: map[string]int{"total": 1}}, GoalNextReport{})
	report.PMView = &view
	report.ChildGroups = nil
	html := RenderGoalReportHTML(report)
	if strings.Contains(html, "<script>") || strings.Contains(html, "Implementation secret") || !strings.Contains(html, "Outcome &lt;script&gt;") {
		t.Fatalf("unsafe/noisy PM HTML: %s", html)
	}
}

func goalPMViewFixture() (PMCompileReport, PMObserveReport) {
	context := PMContextReport{Records: []PMContextRecord{
		{Current: true, CommentURL: "https://example/outcome", Record: PMLedgerRecord{ID: "outcome.activation", Kind: "outcome", Text: "More users complete setup", Status: "active", OutcomeState: "observing", SourceRefs: []string{"metric:activation"}, RecordedAt: "2026-07-18T00:00:00Z"}},
		{Current: false, CommentURL: "https://example/decision-old", Record: PMLedgerRecord{ID: "decision.old", Kind: "decision", Text: "Use old policy", Status: "superseded", SourceRefs: []string{"issue:1"}, RecordedAt: "2026-07-18T01:00:00Z"}},
		{Current: true, CommentURL: "https://example/decision-new", Record: PMLedgerRecord{ID: "decision.new", Kind: "decision", Text: "Use reversible policy", Status: "review_due", Supersedes: "decision.old", SourceRefs: []string{"issue:2"}, RecordedAt: "2026-07-18T02:00:00Z"}},
	}}
	measurement := PMMeasurementReport{Summary: PMMeasurementSummary{Outcomes: 1, NotValidated: 1}}
	status := GoalStatusReport{Goal: GoalStatusIssue{Number: 100}, Children: []GoalStatusChild{{Number: 101, Title: "Build bounded slice", State: "closed", Category: "done"}, {Number: 102, Title: "Observe rollout", State: "open", Category: "in_progress"}}}
	largeEvidence := strings.Repeat("bounded source evidence ", 400)
	state := PMObserveState{Context: context, Discovery: PMDiscoveryReport{Summary: PMDiscoverySummary{ByKind: map[string]int{"outcome": 1}}, Nodes: []PMDiscoveryNode{{ID: "outcome.activation", Kind: "outcome", Text: largeEvidence, Current: true}}}, Measurement: measurement, WorkGraph: PMWorkGraphReport{PlanID: "pwg-1", Nodes: []PMWorkGraphNode{{ID: "build", Title: "Build bounded slice"}, {ID: "observe", Title: "Observe rollout"}}}, GoalStatus: status, PriorPlanID: "pmr-old", PriorDigest: "sha256:old"}
	observe := BuildPMObserveFromState(PMObserveInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Ticket: 100}, state)
	observe.Context = &context
	observe.Discovery = &state.Discovery
	observe.Measurement = &measurement
	observe.WorkGraph = &state.WorkGraph
	observe.GoalStatus = &status
	compile := PMCompileReport{IR: PMIR{SchemaVersion: PMIRSchemaVersion, SourceDigest: "sha256:intent", Premise: PMIRField{Value: "Users need a reliable setup path"}, DesiredOutcome: PMIRField{Value: "More users complete setup"}}}
	return compile, observe
}
