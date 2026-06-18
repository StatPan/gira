package gira

import (
	"strings"
	"testing"
)

func TestBuildPMTaskSpecReportRendersDurablePMState(t *testing.T) {
	report, err := BuildPMTaskSpecReport(PMTaskSpecInput{
		Title:               "Add PM packet",
		Repo:                "StatPan/gira",
		RawIntent:           "Turn rough product requests into worker-ready issue packets.",
		SuggestedWorkerMode: "research",
	})
	if err != nil {
		t.Fatalf("BuildPMTaskSpecReport error: %v", err)
	}
	if report.SchemaVersion != PMTaskPacketSchemaVersion || report.Command != "pm spec" || report.SuggestedWorkerMode != "research" {
		t.Fatalf("unexpected report: %+v", report)
	}
	for _, want := range []string{
		PMStateMarker,
		"## PM Operating Contract",
		"Do not use `needs human` as a terminal state.",
		"## Raw Intent",
		"Turn rough product requests into worker-ready issue packets.",
		"## Decision Policy",
		"## Risk Decomposition",
		"## Verification Expectations",
		"PM acceptance QA checks this PR against Problem",
	} {
		if !strings.Contains(report.Markdown, want) {
			t.Fatalf("markdown missing %q:\n%s", want, report.Markdown)
		}
	}
}

func TestBuildPMTaskSpecReportRejectsEmptyIntent(t *testing.T) {
	_, err := BuildPMTaskSpecReport(PMTaskSpecInput{})
	if err == nil || !strings.Contains(err.Error(), "raw intent is required") {
		t.Fatalf("expected raw intent error, got %v", err)
	}
}
