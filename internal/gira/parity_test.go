package gira

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBuildJiraParityReportIncludesDomainBlockersAndScore(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	report := ProjectCapabilityReport{
		Capabilities: map[string]ProjectCapabilityStatus{
			"issues:read":          ProjectCapabilityAllowed,
			"issues:write":         ProjectCapabilityDeniedScope,
			"pullrequests:read":    ProjectCapabilityAllowed,
			"pullrequests:write":   ProjectCapabilityDeniedScope,
			"repo:settings:write":  ProjectCapabilityDeniedScope,
			"repo:milestone:close": ProjectCapabilityDeniedScope,
		},
		BlockedActions: []ProjectCapabilityBlock{{Action: "issues:write", Reason: "token scope or repository permission is insufficient"}},
	}
	out := BuildJiraParityReport(repo, report, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))

	if out.Command != "parity jira" {
		t.Fatalf("Command = %q, want parity jira", out.Command)
	}
	if out.Scores.Total != 100 {
		t.Fatalf("total score = %d, want 100", out.Scores.Total)
	}
	if out.Scores.Earned >= out.Scores.Total {
		t.Fatalf("earned score = %d, want partial score", out.Scores.Earned)
	}
	if len(out.Blockers) == 0 {
		t.Fatal("expected blockers, got none")
	}
	for _, blocker := range out.Blockers {
		if strings.Contains(blocker.Capability, "projectsv2") {
			t.Fatalf("v1 parity must not require Projects v2, got blocker %+v", blocker)
		}
	}
	if len(out.NextSteps) == 0 {
		t.Fatal("expected next steps for blockers")
	}
}

func TestBuildJiraParityReportRequiresJiraMigrationSurfaces(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	report := v1AllowedCapabilityReport()
	evidence := allV1JiraParityEvidence()
	evidence.Commands["gira ops jira import"] = false
	evidence.Commands["gira ops jira export"] = false

	out := BuildJiraParityReportWithEvidence(repo, report, evidence, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	if out.Ready {
		t.Fatal("expected missing Jira migration surfaces to block readiness")
	}
	for _, want := range []string{"gira ops jira import", "gira ops jira export"} {
		if !hasJiraParityGap(out.Missing, want) {
			t.Fatalf("missing surfaces should include %q: %+v", want, out.Missing)
		}
	}
}

func TestBuildJiraParityReportOrdersBlockersBeforeMissingSurfaces(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	report := v1AllowedCapabilityReport()
	evidence := allV1JiraParityEvidence()
	evidence.Commands["gira ops detach"] = false

	out := BuildJiraParityReportWithEvidence(repo, report, evidence, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	if out.Ready {
		t.Fatal("expected missing detach surface to block readiness")
	}
	if len(out.Blockers) == 0 || len(out.Missing) == 0 {
		t.Fatalf("expected blockers and missing surfaces, got blockers=%v missing=%v", out.Blockers, out.Missing)
	}
	if out.Blockers[0].Command != "gira ops detach" || out.Missing[0].Command != "gira ops detach" {
		t.Fatalf("detach blocker not surfaced first: blockers=%v missing=%v", out.Blockers, out.Missing)
	}

	payload, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	jsonText := string(payload)
	if strings.Index(jsonText, `"blockers"`) > strings.Index(jsonText, `"missing_surfaces"`) {
		t.Fatalf("JSON should list blockers before missing surfaces: %s", jsonText)
	}
	text := FormatJiraParityReport(out)
	if strings.Index(text, "blockers:") > strings.Index(text, "missing surfaces:") {
		t.Fatalf("text should list blockers before missing surfaces:\n%s", text)
	}
}

func TestBuildJiraParityReportScores100WithV1Evidence(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	out := BuildJiraParityReportWithEvidence(repo, v1AllowedCapabilityReport(), allV1JiraParityEvidence(), time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))

	if !out.Ready {
		t.Fatalf("expected ready report, got blockers=%v", out.Blockers)
	}
	if out.Scores.Earned != 100 || out.Scores.Total != 100 || out.Scores.Pct != 100 {
		t.Fatalf("score = %+v, want 100/100", out.Scores)
	}
	if len(out.Blockers) != 0 || len(out.Missing) != 0 || len(out.NextSteps) != 0 {
		t.Fatalf("ready report should not have blockers, missing surfaces, or next steps: %+v", out)
	}
}

func v1AllowedCapabilityReport() ProjectCapabilityReport {
	return ProjectCapabilityReport{Capabilities: map[string]ProjectCapabilityStatus{
		"issues:read":          ProjectCapabilityAllowed,
		"issues:write":         ProjectCapabilityAllowed,
		"pullrequests:read":    ProjectCapabilityAllowed,
		"pullrequests:write":   ProjectCapabilityAllowed,
		"repo:settings:write":  ProjectCapabilityAllowed,
		"repo:milestone:close": ProjectCapabilityAllowed,
	}}
}

func allV1JiraParityEvidence() JiraParityEvidence {
	evidence := DefaultJiraParityEvidence()
	evidence.Commands["gira ops detach"] = true
	return evidence
}

func hasJiraParityGap(gaps []JiraParityGap, command string) bool {
	for _, gap := range gaps {
		if gap.Command == command {
			return true
		}
	}
	return false
}
