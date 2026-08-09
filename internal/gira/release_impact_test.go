package gira

import (
	"strings"
	"testing"
)

func TestTicketNewStoryDeclaresUserFacingReleaseImpact(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &ticketNewRunner{outputs: ticketNewLabelOutputs("type:story", "status:ready")}
	report, err := BuildTicketNewReport(TicketNewInput{Repo: repo, Title: "Export notes", Type: "story", DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildTicketNewReport error: %v", err)
	}
	if !report.ReleaseImpact.Declared || report.ReleaseImpact.Impact != ReleaseImpactUserFacing || !report.ReleaseImpact.ChangelogRequired {
		t.Fatalf("unexpected release impact: %+v", report.ReleaseImpact)
	}
	for _, want := range []string{ReleaseImpactBlockStart, "impact: user-facing", ReleaseImpactBlockEnd} {
		if !strings.Contains(report.Body, want) {
			t.Fatalf("ticket body missing %q:\n%s", want, report.Body)
		}
	}
}

func TestTicketNewExemptReleaseImpactRequiresReason(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &ticketNewRunner{outputs: ticketNewLabelOutputs("type:task", "status:ready")}
	_, err := BuildTicketNewReport(TicketNewInput{Repo: repo, Title: "Internal cleanup", Type: "task", ReleaseImpact: ReleaseImpactExempt, DryRun: true}, runner)
	if err == nil || !strings.Contains(err.Error(), "release-impact-reason") {
		t.Fatalf("expected exempt reason error, got %v", err)
	}
}

func TestReleaseImpactPRBodyCopiesTicketDeclaration(t *testing.T) {
	body, err := RenderTicketReleaseImpact(ReleaseImpactUserFacing, "")
	if err != nil {
		t.Fatal(err)
	}
	prBody := releaseImpactPRBody(82, "Ticket\n\n"+body)
	for _, want := range []string{"Closes #82", ReleaseImpactBlockStart, "impact: user-facing", ReleaseImpactBlockEnd} {
		if !strings.Contains(prBody, want) {
			t.Fatalf("PR body missing %q:\n%s", want, prBody)
		}
	}
}

func TestTicketReleaseImpactWarningsPreservesLegacyStoriesAsDiagnostic(t *testing.T) {
	if warnings := ticketReleaseImpactWarnings("legacy body", []string{"type:story"}); !containsString(warnings, "missing_release_impact") {
		t.Fatalf("expected legacy story diagnostic, got %v", warnings)
	}
	if warnings := ticketReleaseImpactWarnings("legacy body", []string{"type:task"}); len(warnings) != 0 {
		t.Fatalf("task should not require a release-impact declaration: %v", warnings)
	}
}
