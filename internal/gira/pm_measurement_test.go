package gira

import (
	"strings"
	"testing"
)

func TestPMMeasurementValidatesQuantitativeOutcomeAndGuardrail(t *testing.T) {
	records := []PMLedgerRecord{
		discoveryRecord("outcome.activation", "outcome", nil, nil),
		measurementRecord("measurement.activation", "outcome.activation", PMMeasurementPlan{Signal: "Activated teams in 7 days", SignalKind: "leading", EvidenceType: "quantitative", Baseline: "31%", BaselineDefinition: "new teams completing setup within 7 days", Target: "40%", TargetDirection: "increase", ObservationWindow: "28 days after rollout", DataSource: "warehouse:activation_v2", SourceStatus: "available", Owner: "growth-pm", DecisionRule: "Keep rollout when target is met and guardrail is stable", Evaluation: "met", PostChangeDefinition: "new teams completing setup within 7 days"}),
		measurementRecord("measurement.errors", "outcome.activation", PMMeasurementPlan{Signal: "setup error rate", SignalKind: "guardrail", EvidenceType: "quantitative", Baseline: "2%", BaselineDefinition: "setup attempts ending in error", Target: "<=2%", TargetDirection: "maintain", ObservationWindow: "28 days after rollout", DataSource: "logs:setup-errors", SourceStatus: "available", Owner: "growth-pm", DecisionRule: "Rollback above 2%", Evaluation: "stable", PostChangeDefinition: "setup attempts ending in error"}),
	}
	report, err := BuildPMMeasurementReport(PMMeasurementInput{Repo: ParseRepoRefMust("OWNER/repo"), Ticket: 42}, &pmLedgerRunner{context: pmLedgerContextJSON(t, "", discoveryComments(records))})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Diagnostics) != 0 || report.Summary.Validated != 1 || report.Outcomes[0].State != "validated" {
		t.Fatalf("unexpected measurable report: %#v diagnostics=%#v", report.Summary, report.Diagnostics)
	}
	encoded, encodeErr := FormatPMMeasurementJSON(report)
	if encodeErr != nil || !strings.Contains(string(encoded), "warehouse:activation_v2") || !strings.Contains(string(encoded), "baseline_definition") {
		t.Fatalf("full measurement provenance missing: err=%v %s", encodeErr, encoded)
	}
}

func TestPMMeasurementGuardrailRegressionBlocksImprovedTarget(t *testing.T) {
	records := []PMLedgerRecord{discoveryRecord("outcome.1", "outcome", nil, nil), measurementRecord("m.primary", "outcome.1", completeQuantitativePlan("Activation", "met")), measurementRecord("m.guardrail", "outcome.1", PMMeasurementPlan{Signal: "Error rate", SignalKind: "guardrail", EvidenceType: "quantitative", Baseline: "1%", BaselineDefinition: "failed attempts / attempts", Target: "<=1%", TargetDirection: "maintain", ObservationWindow: "14 days", DataSource: "logs:error", SourceStatus: "available", Owner: "pm", DecisionRule: "Rollback on regression", Evaluation: "regressed", PostChangeDefinition: "failed attempts / attempts"})}
	report, err := BuildPMMeasurementReport(PMMeasurementInput{Repo: ParseRepoRefMust("OWNER/repo"), Ticket: 42}, &pmLedgerRunner{context: pmLedgerContextJSON(t, "", discoveryComments(records))})
	if err != nil {
		t.Fatal(err)
	}
	if report.Outcomes[0].State != "blocked" || !hasPMLedgerDiagnostic(report.Diagnostics, PMMeasurementGuardrailRegression) {
		t.Fatalf("guardrail did not block validation: %#v", report)
	}
}

func TestPMMeasurementConflictingProductEvidenceCannotValidate(t *testing.T) {
	records := []PMLedgerRecord{discoveryRecord("outcome.1", "outcome", nil, nil), measurementRecord("m.met", "outcome.1", completeQuantitativePlan("Activation", "met")), measurementRecord("m.missed", "outcome.1", completeQuantitativePlan("Retention", "not_met"))}
	report, err := BuildPMMeasurementReport(PMMeasurementInput{Repo: ParseRepoRefMust("OWNER/repo"), Ticket: 42}, &pmLedgerRunner{context: pmLedgerContextJSON(t, "", discoveryComments(records))})
	if err != nil {
		t.Fatal(err)
	}
	if report.Outcomes[0].State != "blocked" || !hasPMLedgerDiagnostic(report.Diagnostics, PMMeasurementMissingDecision) {
		t.Fatalf("conflict appeared validated: %#v", report)
	}
}

func TestPMMeasurementSupportsQualitativeAndExplicitLimitation(t *testing.T) {
	qualitative := PMMeasurementPlan{Signal: "Operators can explain the new workflow", SignalKind: "leading", EvidenceType: "qualitative", Baseline: "3 of 8 confused", BaselineDefinition: "moderated task interview with payroll operators", Target: "At least 7 of 8 complete and explain", TargetDirection: "qualitative", ObservationWindow: "two interview rounds", DataSource: "research:round-4", SourceStatus: "available", Owner: "research-pm", DecisionRule: "Proceed when target is met with no severe confusion", Evaluation: "met", PostChangeDefinition: "moderated task interview with payroll operators", QualitativeMethod: "moderated task interview", QualitativeSample: "8 payroll operators across two company sizes", QualitativeLimits: "small purposive sample; not prevalence evidence"}
	limited := PMMeasurementPlan{Signal: "Long-term retention", SignalKind: "lagging", EvidenceType: "limitation", SourceStatus: "unavailable", Owner: "retention-pm", DecisionRule: "Do not claim retention impact until follow-up completes", EvidenceLimitation: "Product has no cohort history yet", FollowUpRef: "issue:OWNER/repo#99"}
	for _, test := range []struct {
		name string
		plan PMMeasurementPlan
		want string
	}{{"qualitative", qualitative, "validated"}, {"limitation", limited, "limited"}} {
		t.Run(test.name, func(t *testing.T) {
			records := []PMLedgerRecord{discoveryRecord("outcome.1", "outcome", nil, nil), measurementRecord("m.1", "outcome.1", test.plan)}
			report, err := BuildPMMeasurementReport(PMMeasurementInput{Repo: ParseRepoRefMust("OWNER/repo"), Ticket: 42}, &pmLedgerRunner{context: pmLedgerContextJSON(t, "", discoveryComments(records))})
			if err != nil {
				t.Fatal(err)
			}
			if report.Outcomes[0].State != test.want {
				t.Fatalf("state=%s diagnostics=%#v", report.Outcomes[0].State, report.Diagnostics)
			}
		})
	}
}

func TestPMMeasurementDiagnosesMisleadingAndIncomparableSignals(t *testing.T) {
	delivery := completeQuantitativePlan("Download count", "met")
	delivery.SignalKind = "delivery"
	delivery.PostChangeDefinition = "unique downloads including retries"
	records := []PMLedgerRecord{discoveryRecord("outcome.1", "outcome", nil, nil), measurementRecord("m.delivery", "outcome.1", delivery)}
	report, err := BuildPMMeasurementReport(PMMeasurementInput{Repo: ParseRepoRefMust("OWNER/repo"), Ticket: 42}, &pmLedgerRunner{context: pmLedgerContextJSON(t, "", discoveryComments(records))})
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{PMMeasurementDeliveryProxy, PMMeasurementVanityMetric, PMMeasurementIncomparable} {
		if !hasPMLedgerDiagnostic(report.Diagnostics, code) {
			t.Fatalf("missing %s: %#v", code, report.Diagnostics)
		}
	}
}

func TestPMMeasurementRejectsMissingBaselineAndWindow(t *testing.T) {
	_, diagnostics := normalizePMLedgerRecord(PMRecordInput{ID: "m.1", Kind: "measurement", Text: "Activation", SourceRefs: []string{"issue:42"}, ActorKind: "human", Links: []PMLedgerLink{{Relation: "measures", TargetID: "outcome.1"}}, Measurement: &PMMeasurementPlan{Signal: "Activation", SignalKind: "leading", EvidenceType: "quantitative", Target: "40%", TargetDirection: "increase", DataSource: "warehouse:a", Owner: "pm", DecisionRule: "Proceed at 40%"}})
	for _, code := range []string{PMMeasurementMissingBaseline, PMMeasurementUnboundedWindow} {
		if !hasPMLedgerDiagnostic(diagnostics, code) {
			t.Fatalf("missing %s: %#v", code, diagnostics)
		}
	}
}

func TestPMMeasurementRequiresTypedOutcomeForPhaseValidation(t *testing.T) {
	report, err := BuildPMMeasurementReport(PMMeasurementInput{Repo: ParseRepoRefMust("OWNER/repo"), Ticket: 42}, &pmLedgerRunner{context: pmLedgerContextJSON(t, "", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if !hasPMLedgerDiagnostic(report.Diagnostics, PMMeasurementMissingPlan) {
		t.Fatalf("empty ledger appeared measurable: %#v", report)
	}
}

func TestPMMeasurementApprovalReplaysDecisionContract(t *testing.T) {
	plan := completeQuantitativePlan("Activation", "met")
	record, diagnostics := normalizePMLedgerRecord(PMRecordInput{ID: "m.1", Kind: "measurement", Text: "Activation", SourceRefs: []string{"metric:a"}, ActorKind: "human", Links: []PMLedgerLink{{Relation: "measures", TargetID: "outcome.1"}}, Measurement: &plan})
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%#v", diagnostics)
	}
	approval := PMRecordApprovalEvidence(PMRecordReport{Repo: "OWNER/repo", Ticket: 42, Record: record})
	for _, fragment := range []string{"--signal Activation", "--baseline-definition 'eligible users completing outcome'", "--observation-window '14 days'", "--decision-rule 'Proceed when target met'", "--evaluation met"} {
		if !strings.Contains(approval.ApplyCommand, fragment) {
			t.Fatalf("approval lost %q: %s", fragment, approval.ApplyCommand)
		}
	}
}

func TestPMAcceptanceQAPromptIncludesOutcomeValidation(t *testing.T) {
	prompt := RenderPMAcceptanceQAPrompt(PMAcceptanceQAReport{Repo: "OWNER/repo", Ticket: 42, Issue: AgentPromptIssue{Title: "Measured change"}, Measurement: &PMMeasurementReport{Summary: PMMeasurementSummary{Validated: 1, Blocked: 1}, Diagnostics: []PMLedgerDiagnostic{{Code: PMMeasurementGuardrailRegression}}}})
	if !strings.Contains(prompt, "Outcome Measurement: validated=1") || !strings.Contains(prompt, "regressed guardrails") {
		t.Fatalf("PM QA omitted measurement contract:\n%s", prompt)
	}
}

func measurementRecord(id, outcome string, plan PMMeasurementPlan) PMLedgerRecord {
	record := discoveryRecord(id, "measurement", []PMLedgerLink{{Relation: "measures", TargetID: outcome}}, nil)
	record.Measurement = &plan
	return record
}
func completeQuantitativePlan(signal, evaluation string) PMMeasurementPlan {
	return PMMeasurementPlan{Signal: signal, SignalKind: "leading", EvidenceType: "quantitative", Baseline: "10", BaselineDefinition: "eligible users completing outcome", Target: "20", TargetDirection: "increase", ObservationWindow: "14 days", DataSource: "warehouse:outcome", SourceStatus: "available", Owner: "pm", DecisionRule: "Proceed when target met", Evaluation: evaluation, PostChangeDefinition: "eligible users completing outcome"}
}
