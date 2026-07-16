package gira

import (
	"strings"
	"testing"
	"time"
)

func TestPMDiscoveryRecordValidationIsTypedAndProportional(t *testing.T) {
	tests := []struct {
		name string
		in   PMRecordInput
		code string
	}{
		{name: "hypothesis needs falsification", in: PMRecordInput{ID: "h1", Kind: "hypothesis", Links: []PMLedgerLink{{Relation: "addresses", TargetID: "p1"}}}, code: PMDiscoveryDiagnosticMissingTest},
		{name: "risk needs one type", in: PMRecordInput{ID: "r1", Kind: "risk", Links: []PMLedgerLink{{Relation: "risks", TargetID: "h1"}}}, code: PMDiscoveryDiagnosticInvalidRisk},
		{name: "experiment needs lifecycle", in: PMRecordInput{ID: "e1", Kind: "experiment", Links: []PMLedgerLink{{Relation: "tests", TargetID: "h1"}}}, code: PMDiscoveryDiagnosticInvalidExperiment},
		{name: "completed experiment needs inspectable evidence", in: PMRecordInput{ID: "e1", Kind: "experiment", ExperimentState: "failure", Links: []PMLedgerLink{{Relation: "tests", TargetID: "h1"}}}, code: PMDiscoveryDiagnosticMissingEvidence},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.in.Text = "Inspectable claim"
			test.in.SourceRefs = []string{"issue:42"}
			test.in.ActorKind = "human"
			_, diagnostics := normalizePMLedgerRecord(test.in)
			if !hasPMLedgerDiagnostic(diagnostics, test.code) {
				t.Fatalf("missing %s: %#v", test.code, diagnostics)
			}
		})
	}

	_, diagnostics := normalizePMLedgerRecord(PMRecordInput{
		ID: "risk.value", Kind: "risk", Text: "Users may not value the workflow", SourceRefs: []string{"interview:5"}, ActorKind: "human",
		RiskType: "value", Links: []PMLedgerLink{{Relation: "risks", TargetID: "hypothesis.1"}},
	})
	if len(diagnostics) != 0 {
		t.Fatalf("one relevant risk type should be sufficient: %#v", diagnostics)
	}
}

func TestBuildPMDiscoveryReportTracesCompetingOpportunitiesAndNoBuild(t *testing.T) {
	records := []PMLedgerRecord{
		discoveryRecord("outcome.activation", "outcome", nil, func(r *PMLedgerRecord) { r.OutcomeState = "observing" }),
		discoveryRecord("opportunity.setup", "opportunity", []PMLedgerLink{{Relation: "supports", TargetID: "outcome.activation"}}, nil),
		discoveryRecord("opportunity.trust", "opportunity", []PMLedgerLink{{Relation: "supports", TargetID: "outcome.activation"}}, nil),
		discoveryRecord("hypothesis.wizard", "hypothesis", []PMLedgerLink{{Relation: "addresses", TargetID: "opportunity.setup"}}, func(r *PMLedgerRecord) { r.FalsificationTest = "Activation does not improve by 5%." }),
		discoveryRecord("hypothesis.badges", "hypothesis", []PMLedgerLink{{Relation: "addresses", TargetID: "opportunity.trust"}}, func(r *PMLedgerRecord) { r.FalsificationTest = "Trust score is unchanged." }),
		discoveryRecord("experiment.wizard", "experiment", []PMLedgerLink{{Relation: "tests", TargetID: "hypothesis.wizard"}}, func(r *PMLedgerRecord) {
			r.ExperimentState, r.EvidenceStrength, r.Confidence = "failure", "quantitative", "high"
		}),
		discoveryRecord("experiment.badges", "experiment", []PMLedgerLink{{Relation: "tests", TargetID: "hypothesis.badges"}}, func(r *PMLedgerRecord) {
			r.ExperimentState, r.EvidenceStrength, r.Confidence = "success", "qualitative", "medium"
		}),
		discoveryRecord("learning.wizard", "learning", []PMLedgerLink{{Relation: "learned_from", TargetID: "experiment.wizard"}}, func(r *PMLedgerRecord) {
			r.Conclusion, r.EvidenceStrength, r.Confidence = "invalidated", "quantitative", "high"
		}),
		discoveryRecord("learning.badges", "learning", []PMLedgerLink{{Relation: "learned_from", TargetID: "experiment.badges"}}, func(r *PMLedgerRecord) {
			r.Conclusion, r.EvidenceStrength, r.Confidence = "no_build", "qualitative", "medium"
			r.GoalRefs, r.TaskProfiles = []string{"issue:OWNER/repo#100"}, []string{"discovery", "decision"}
		}),
		discoveryRecord("decision.stop-wizard", "decision", []PMLedgerLink{{Relation: "based_on", TargetID: "learning.wizard"}}, func(r *PMLedgerRecord) { r.Status = "accepted" }),
		discoveryRecord("decision.no-build", "decision", []PMLedgerLink{{Relation: "based_on", TargetID: "learning.badges"}}, func(r *PMLedgerRecord) { r.Status = "accepted" }),
	}
	runner := &pmLedgerRunner{context: pmLedgerContextJSON(t, "", discoveryComments(records))}
	report, err := BuildPMDiscoveryReport(PMDiscoveryInput{Repo: ParseRepoRefMust("OWNER/repo"), Ticket: 42}, runner)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Diagnostics) != 0 || report.Summary.ByKind["opportunity"] != 2 || report.Summary.LearningConclusions["invalidated"] != 1 || report.Summary.LearningConclusions["no_build"] != 1 {
		t.Fatalf("unexpected discovery summary: %#v diagnostics=%#v", report.Summary, report.Diagnostics)
	}
	if len(report.Traces) != 2 || report.Traces[0].Path[0] != "outcome.activation" {
		t.Fatalf("expected two complete competing traces: %#v", report.Traces)
	}
	linked := false
	for _, node := range report.Nodes {
		if node.ID == "learning.badges" && len(node.GoalRefs) == 1 && len(node.TaskProfiles) == 2 {
			linked = true
		}
	}
	if !linked {
		t.Fatalf("Goal/task profile links were not projected: %#v", report.Nodes)
	}
	encoded, err := FormatPMDiscoveryJSON(report)
	if err != nil || !strings.Contains(string(encoded), "no_build") || !strings.Contains(string(encoded), "invalidated") {
		t.Fatalf("full JSON lost discovery conclusions: err=%v %s", err, encoded)
	}
	compact := FormatPMDiscovery(report, 512)
	if len(compact) > 512 || !strings.Contains(compact, "detail:") {
		t.Fatalf("compact graph exceeded budget: %d %s", len(compact), compact)
	}
}

func TestPMDiscoveryRejectsUnknownTaskProfile(t *testing.T) {
	_, diagnostics := normalizePMLedgerRecord(PMRecordInput{ID: "outcome.1", Kind: "outcome", Text: "Activation", SourceRefs: []string{"metric:a"}, ActorKind: "human", TaskProfiles: []string{"mystery"}})
	if !hasPMLedgerDiagnostic(diagnostics, PMLedgerDiagnosticInvalidRecord) {
		t.Fatalf("unknown task profile accepted: %#v", diagnostics)
	}
}

func TestPMDiscoveryRecordApprovalPreservesTypedLinks(t *testing.T) {
	record, diagnostics := normalizePMLedgerRecord(PMRecordInput{ID: "learning.1", Kind: "learning", Text: "Do not build", SourceRefs: []string{"experiment:1"}, ActorKind: "ai", Links: []PMLedgerLink{{Relation: "learned_from", TargetID: "experiment.1"}}, GoalRefs: []string{"issue:OWNER/repo#100"}, TaskProfiles: []string{"decision"}, EvidenceStrength: "qualitative", Confidence: "medium", Conclusion: "no_build", RecordedAt: time.Date(2026, 7, 16, 1, 2, 3, 0, time.UTC)})
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected record diagnostics: %#v", diagnostics)
	}
	approval := PMRecordApprovalEvidence(PMRecordReport{Repo: "OWNER/repo", Ticket: 42, Record: record})
	for _, fragment := range []string{"--link learned_from=experiment.1", "--goal-ref 'issue:OWNER/repo#100'", "--task-profile decision", "--evidence-strength qualitative", "--confidence medium", "--conclusion no_build"} {
		if !strings.Contains(approval.ApplyCommand, fragment) {
			t.Fatalf("approval command lost %q: %s", fragment, approval.ApplyCommand)
		}
	}
}

func TestPMDiscoveryMetadataUsesLedgerPrivacyBoundary(t *testing.T) {
	report, err := BuildPMRecordReport(PMRecordInput{
		Repo: ParseRepoRefMust("OWNER/repo"), Ticket: 42, ID: "hypothesis.secret", Kind: "hypothesis", Text: "Test setup", SourceRefs: []string{"issue:42"}, ActorKind: "human",
		Links: []PMLedgerLink{{Relation: "addresses", TargetID: "opportunity.1"}}, FalsificationTest: "api_key=super-secret-value", DryRun: true,
	}, &pmLedgerRunner{})
	if err == nil || report.Record.FalsificationTest != "" || strings.Contains(report.Record.Text, "super-secret-value") {
		t.Fatalf("discovery metadata crossed privacy boundary: report=%#v err=%v", report, err)
	}
}

func TestPMRecordFailsClosedOnBrokenDiscoveryGraph(t *testing.T) {
	runner := &pmLedgerRunner{context: pmLedgerContextJSON(t, "", nil)}
	report, err := BuildPMRecordReport(PMRecordInput{
		Repo: ParseRepoRefMust("OWNER/repo"), Ticket: 42, ID: "opportunity.1", Kind: "opportunity", Text: "Setup friction", SourceRefs: []string{"interview:1"}, ActorKind: "human",
		Links: []PMLedgerLink{{Relation: "supports", TargetID: "missing.outcome"}}, DryRun: true,
	}, runner)
	if err == nil || !hasPMLedgerDiagnostic(report.Diagnostics, PMDiscoveryDiagnosticUnknownTarget) || len(runner.posted) != 0 {
		t.Fatalf("broken graph did not fail closed: report=%#v err=%v", report, err)
	}
}

func TestPMDiscoveryRejectsFalseValidationAndInvalidLinks(t *testing.T) {
	records := []PMLedgerRecord{
		discoveryRecord("outcome.1", "outcome", nil, nil),
		discoveryRecord("hypothesis.1", "hypothesis", []PMLedgerLink{{Relation: "addresses", TargetID: "outcome.1"}}, func(r *PMLedgerRecord) { r.TestWaiver = "Cheap reversible probe." }),
		discoveryRecord("experiment.1", "experiment", []PMLedgerLink{{Relation: "tests", TargetID: "hypothesis.1"}}, func(r *PMLedgerRecord) {
			r.ExperimentState, r.EvidenceStrength, r.Confidence = "inconclusive", "anecdotal", "low"
		}),
		discoveryRecord("learning.1", "learning", []PMLedgerLink{{Relation: "learned_from", TargetID: "experiment.1"}}, func(r *PMLedgerRecord) {
			r.Conclusion, r.EvidenceStrength, r.Confidence = "validated", "anecdotal", "low"
		}),
		discoveryRecord("decision.1", "decision", []PMLedgerLink{{Relation: "based_on", TargetID: "missing.learning"}}, nil),
	}
	runner := &pmLedgerRunner{context: pmLedgerContextJSON(t, "", discoveryComments(records))}
	report, err := BuildPMDiscoveryReport(PMDiscoveryInput{Repo: ParseRepoRefMust("OWNER/repo"), Ticket: 42}, runner)
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{PMDiscoveryDiagnosticInvalidRelation, PMDiscoveryDiagnosticFalseValidation, PMDiscoveryDiagnosticUnknownTarget} {
		if !hasPMLedgerDiagnostic(report.Diagnostics, code) {
			t.Fatalf("missing %s: %#v", code, report.Diagnostics)
		}
	}
}

func TestPMDiscoveryTraceUsesCurrentSupersedingRecord(t *testing.T) {
	old := discoveryRecord("opportunity.old", "opportunity", []PMLedgerLink{{Relation: "supports", TargetID: "outcome.1"}}, nil)
	current := discoveryRecord("opportunity.new", "opportunity", []PMLedgerLink{{Relation: "supports", TargetID: "outcome.1"}}, func(r *PMLedgerRecord) { r.Supersedes = "opportunity.old" })
	records := []PMLedgerRecord{discoveryRecord("outcome.1", "outcome", nil, nil), old, current}
	report, err := BuildPMDiscoveryReport(PMDiscoveryInput{Repo: ParseRepoRefMust("OWNER/repo"), Ticket: 42}, &pmLedgerRunner{context: pmLedgerContextJSON(t, "", discoveryComments(records))})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Traces) != 1 || strings.Contains(strings.Join(report.Traces[0].Path, " "), "opportunity.old") || !strings.Contains(strings.Join(report.Traces[0].Path, " "), "opportunity.new") {
		t.Fatalf("trace did not resolve supersession: %#v", report.Traces)
	}
}

func discoveryRecord(id, kind string, links []PMLedgerLink, alter func(*PMLedgerRecord)) PMLedgerRecord {
	record := PMLedgerRecord{SchemaVersion: PMLedgerRecordSchemaVersion, ID: id, Kind: kind, Text: "Claim for " + id, SourceRefs: []string{"issue:42"}, ActorKind: "human", RecordedAt: "2026-07-16T01:00:00Z", Status: defaultPMLedgerStatus(kind), Links: links}
	if kind == "outcome" {
		record.OutcomeState = "proposed"
	}
	if alter != nil {
		alter(&record)
	}
	return record
}

func discoveryComments(records []PMLedgerRecord) []pmLedgerTestComment {
	comments := make([]pmLedgerTestComment, 0, len(records))
	for _, record := range records {
		comments = append(comments, pmLedgerTestComment{Body: RenderPMLedgerRecordComment(record), URL: "https://example/comments/" + record.ID, CreatedAt: record.RecordedAt, Author: "pm"})
	}
	return comments
}
