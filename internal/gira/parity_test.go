package gira

import (
	"testing"
	"time"
)

func TestBuildJiraParityReportIncludesDomainBlockersAndScore(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	report := ProjectCapabilityReport{
		Capabilities: map[string]ProjectCapabilityStatus{
			"issues:read":         ProjectCapabilityAllowed,
			"issues:write":        ProjectCapabilityDeniedScope,
			"pullrequests:write":  ProjectCapabilityDeniedScope,
			"projectsv2:read":     ProjectCapabilityAllowed,
			"repo:settings:write": ProjectCapabilityDeniedScope,
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
}
