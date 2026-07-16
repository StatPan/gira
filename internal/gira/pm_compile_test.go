package gira

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestBuildPMCompileReportGoldenScenarios(t *testing.T) {
	tests := []struct {
		name      string
		wantCodes []string
		forbid    []string
	}{
		{name: "complete", wantCodes: []string{PMDiagnosticAuthorityBound}, forbid: []string{PMDiagnosticMissingActor, PMDiagnosticMissingProblem, PMDiagnosticMissingOutcome, PMDiagnosticLowEvidence, PMDiagnosticMissingSuccess}},
		{name: "incomplete", wantCodes: []string{PMDiagnosticMissingActor, PMDiagnosticMissingProblem, PMDiagnosticMissingOutcome, PMDiagnosticLowEvidence, PMDiagnosticUnstructuredIntent, PMDiagnosticMissingSuccess}},
		{name: "conflicting", wantCodes: []string{PMDiagnosticConflictingState}},
		{name: "authority", wantCodes: []string{PMDiagnosticAuthorityBound}},
		{name: "low-evidence", wantCodes: []string{PMDiagnosticLowEvidence}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := readPMCompileFixture(t, test.name+".md")
			report, err := BuildPMCompileReport(PMCompileInput{RawIntent: raw})
			if err != nil {
				t.Fatalf("BuildPMCompileReport: %v", err)
			}
			if report.SchemaVersion != PMCompileReportSchemaVersion || report.IR.SchemaVersion != PMIRSchemaVersion || !report.ReadOnly {
				t.Fatalf("unexpected contract metadata: %#v", report)
			}
			for _, code := range test.wantCodes {
				if !hasPMDiagnostic(report, code) {
					t.Errorf("missing diagnostic %s in %#v", code, report.Diagnostics)
				}
			}
			for _, code := range test.forbid {
				if hasPMDiagnostic(report, code) {
					t.Errorf("unexpected diagnostic %s in %#v", code, report.Diagnostics)
				}
			}
		})
	}
}

func TestBuildPMCompileReportPreservesMeaningAndSourceSpans(t *testing.T) {
	raw := readPMCompileFixture(t, "complete.md")
	report, err := BuildPMCompileReport(PMCompileInput{RawIntent: raw})
	if err != nil {
		t.Fatalf("BuildPMCompileReport: %v", err)
	}
	if report.IR.Actor.Value != "The product manager handing work to an AI or human worker." || report.IR.Actor.Provenance != PMProvenanceSupplied {
		t.Fatalf("actor was not preserved: %#v", report.IR.Actor)
	}
	if len(report.IR.Actor.Sources) != 1 || report.IR.Actor.Sources[0].StartLine != 5 || report.IR.Actor.Sources[0].EndLine != 5 {
		t.Fatalf("unexpected actor source span: %#v", report.IR.Actor.Sources)
	}
	if len(report.IR.Constraints.Items) != 2 || report.IR.Constraints.Items[0].Value != "Keep GitHub as the execution backend." {
		t.Fatalf("constraints were not preserved: %#v", report.IR.Constraints)
	}
	if report.IR.SourceDigest == "" || !strings.HasPrefix(report.IR.SourceDigest, "sha256:") {
		t.Fatalf("missing stable source digest: %q", report.IR.SourceDigest)
	}
	if len(report.IR.Sources) != 1 || report.IR.Sources[0].Content != strings.TrimSpace(raw) {
		t.Fatalf("full source content was not preserved: %#v", report.IR.Sources)
	}
}

func TestBuildPMCompileReportReportsButPreservesUnknownSections(t *testing.T) {
	raw := "# Product Bet\nAdoption rises when setup is reversible."
	report, err := BuildPMCompileReport(PMCompileInput{RawIntent: raw})
	if err != nil {
		t.Fatal(err)
	}
	if !hasPMDiagnostic(report, PMDiagnosticUnrecognizedSection) {
		t.Fatalf("missing unrecognized-section diagnostic: %#v", report.Diagnostics)
	}
	if !hasPMDiagnostic(report, PMDiagnosticUnstructuredIntent) {
		t.Fatalf("intent with no recognized PM heading was not reported: %#v", report.Diagnostics)
	}
	if report.IR.Sources[0].Content != raw {
		t.Fatalf("unknown section source was discarded: %#v", report.IR.Sources[0])
	}
}

func TestBuildPMCompileReportDoesNotInferSemanticsFromUnstructuredIntent(t *testing.T) {
	report, err := BuildPMCompileReport(PMCompileInput{RawIntent: "Build a dashboard for operators so incidents fall."})
	if err != nil {
		t.Fatalf("BuildPMCompileReport: %v", err)
	}
	if report.IR.Premise.Value == "" || report.IR.Premise.Provenance != PMProvenanceSupplied {
		t.Fatalf("raw intent should be preserved as premise: %#v", report.IR.Premise)
	}
	for name, field := range map[string]PMIRField{"actor": report.IR.Actor, "problem": report.IR.Problem, "outcome": report.IR.DesiredOutcome} {
		if field.Provenance != PMProvenanceUnresolved || field.Value != "" {
			t.Errorf("%s was guessed: %#v", name, field)
		}
	}
}

func TestBuildPMCompileReportUsesOnlyExplicitGoalObjectiveAsInference(t *testing.T) {
	report, err := BuildPMCompileReport(PMCompileInput{
		RawIntent: "# Actor\nMaintainer\n# Problem\nThe outcome is not written locally.\n# Evidence\n- issue #857\n# Success Conditions\n- Goal outcome is retained.",
		Repo:      "OWNER/repo",
		Goal: &PMCompileGoal{
			Number: 857,
			Title:  "V3 active PM",
			Body:   "# Objective\nGira proactively compiles product intent.\n# Direction\nPreserve user authority.",
			URL:    "https://github.com/OWNER/repo/issues/857",
		},
	})
	if err != nil {
		t.Fatalf("BuildPMCompileReport: %v", err)
	}
	if report.IR.DesiredOutcome.Value != "Gira proactively compiles product intent." || report.IR.DesiredOutcome.Provenance != PMProvenanceInferred {
		t.Fatalf("explicit goal objective was not retained as inference: %#v", report.IR.DesiredOutcome)
	}
	if len(report.IR.DesiredOutcome.Sources) != 1 || report.IR.DesiredOutcome.Sources[0].SourceID != "goal:857" {
		t.Fatalf("goal provenance was lost: %#v", report.IR.DesiredOutcome.Sources)
	}
	if report.IR.Repository == nil || report.IR.Repository.FullName.Value != "OWNER/repo" {
		t.Fatalf("repository context was not preserved: %#v", report.IR.Repository)
	}
	if len(report.IR.Sources) != 3 || report.IR.Sources[2].Content != "# Objective\nGira proactively compiles product intent.\n# Direction\nPreserve user authority." {
		t.Fatalf("goal source content and line basis were not preserved: %#v", report.IR.Sources)
	}
}

func TestPMCompileJSONRoundTripAndCompactBudget(t *testing.T) {
	report, err := BuildPMCompileReport(PMCompileInput{RawIntent: readPMCompileFixture(t, "complete.md")})
	if err != nil {
		t.Fatalf("BuildPMCompileReport: %v", err)
	}
	encoded, err := FormatPMCompileJSON(report)
	if err != nil {
		t.Fatalf("FormatPMCompileJSON: %v", err)
	}
	var decoded PMCompileReport
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if decoded.IR.SourceDigest != report.IR.SourceDigest || len(decoded.IR.CandidateWork.Items) != 2 {
		t.Fatalf("JSON round trip changed the IR: %#v", decoded.IR)
	}
	compact := FormatPMCompile(report)
	if len(compact) > 4096 {
		t.Fatalf("compact output exceeded 4096-byte budget: %d", len(compact))
	}
	if strings.Contains(compact, report.IR.Premise.Value) || !strings.Contains(compact, "detail: gira pm compile") {
		t.Fatalf("compact output should omit source prose and point to full JSON:\n%s", compact)
	}
}

func TestPMCompileDiagnosticsContractAndCompactCap(t *testing.T) {
	var input strings.Builder
	for i := 0; i < 20; i++ {
		input.WriteString("# Unknown Section ")
		input.WriteString(string(rune('A' + i)))
		input.WriteString("\nPreserved source statement.\n")
	}
	report, err := BuildPMCompileReport(PMCompileInput{RawIntent: input.String()})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Diagnostics) <= 8 {
		t.Fatalf("fixture must exercise compact diagnostic cap: %d", len(report.Diagnostics))
	}
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Severity == "" || diagnostic.Code == "" || diagnostic.Field == "" || diagnostic.Location.SourceID == "" || diagnostic.Location.Section == "" || diagnostic.Reason == "" || diagnostic.Impact == "" || diagnostic.Repair == "" {
			t.Fatalf("incomplete diagnostic contract: %#v", diagnostic)
		}
	}
	compact := FormatPMCompile(report)
	if len(compact) > 4096 || !strings.Contains(compact, "additional diagnostics omitted") {
		t.Fatalf("compact diagnostic output is not bounded: bytes=%d\n%s", len(compact), compact)
	}
}

func TestPMCompileDeterministic(t *testing.T) {
	input := PMCompileInput{RawIntent: readPMCompileFixture(t, "conflicting.md")}
	first, err := BuildPMCompileReport(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildPMCompileReport(input)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := FormatPMCompileJSON(first)
	b, _ := FormatPMCompileJSON(second)
	if string(a) != string(b) {
		t.Fatalf("compile is not deterministic:\n%s\n---\n%s", a, b)
	}
}

func TestBuildPMCompileReportFromRequestHydratesGoalReadOnly(t *testing.T) {
	runner := &devStartRunner{outputs: map[string][]byte{
		"gh api repos/OWNER/repo/issues/857": []byte(`{"number":857,"title":"V3 active PM","body":"# Objective\nCompile explicit product intent.","state":"open","labels":[{"name":"type:goal"}]}`),
	}}
	report, err := BuildPMCompileReportFromRequest(PMCompileRequest{
		RawIntent: "# Actor\nPM\n# Problem\nIntent is implicit.\n# Evidence\n- issue #857\n# Success Conditions\n- Outcome is explicit.",
		Repo:      "OWNER/repo",
		Goal:      857,
	}, runner)
	if err != nil {
		t.Fatalf("BuildPMCompileReportFromRequest: %v", err)
	}
	if len(runner.calls) != 1 || runner.calls[0] != "gh api repos/OWNER/repo/issues/857" {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}
	if report.IR.GoalContext == nil || report.IR.DesiredOutcome.Provenance != PMProvenanceInferred {
		t.Fatalf("goal context was not compiled: %#v", report.IR)
	}
}

func TestBuildPMCompileReportFromRequestRequiresRepoForGoal(t *testing.T) {
	_, err := BuildPMCompileReportFromRequest(PMCompileRequest{RawIntent: "intent", Goal: 857}, nil)
	if err == nil || !strings.Contains(err.Error(), "--goal requires --repo") {
		t.Fatalf("expected repo requirement, got %v", err)
	}
}

func TestBuildPMCompileReportFromRequestRejectsNonGoalIssue(t *testing.T) {
	runner := &devStartRunner{outputs: map[string][]byte{
		"gh api repos/OWNER/repo/issues/10": []byte(`{"number":10,"title":"Task","body":"# Objective\nDo work.","state":"open","labels":[{"name":"type:task"}]}`),
	}}
	_, err := BuildPMCompileReportFromRequest(PMCompileRequest{RawIntent: "intent", Repo: "OWNER/repo", Goal: 10}, runner)
	if err == nil || !strings.Contains(err.Error(), "is not a Goal issue") {
		t.Fatalf("expected Goal type rejection, got %v", err)
	}
}

func readPMCompileFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/pm_compile/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

func hasPMDiagnostic(report PMCompileReport, code string) bool {
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
