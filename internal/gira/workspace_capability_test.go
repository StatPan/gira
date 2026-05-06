package gira

import (
	"strings"
	"testing"
	"time"
)

func TestBuildWorkspaceCapabilityReportAllowed(t *testing.T) {
	runner := &fakePortfolioCapabilityRunner{
		authStatus: `{"hosts":{"github.com":[{"state":"success","active":true,"host":"github.com","login":"alice","tokenSource":"/home/user/.config/gh/hosts.yml","scopes":"repo"}]}}`,
		repos: map[string]string{
			"repos/StatPan/backlog":        `{"permissions":{"admin":false,"maintain":false,"pull":true,"push":false,"triage":true}}`,
			"repos/StatPan/backlog/issues": `[]`,
			"repos/StatPan/gira":           `{"permissions":{"admin":false,"maintain":true,"pull":true,"push":true,"triage":true}}`,
			"repos/StatPan/gira/issues":    `[]`,
		},
	}
	config := WorkspaceConfigResolved{Name: "personal", Owner: "StatPan", InboxRepo: ParseRepoRefMust("StatPan/backlog"), Repos: []RepoRef{ParseRepoRefMust("StatPan/gira")}}

	report, err := BuildWorkspaceCapabilityReport(config, runner, time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildWorkspaceCapabilityReport error: %v", err)
	}
	if report.Command != "workspace capability" || len(report.BlockedActions) != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if len(report.Repos) != 2 || report.Repos[0].Role != "inbox" || report.Repos[1].Role != "execution" {
		t.Fatalf("repos = %+v, want inbox plus execution", report.Repos)
	}
}

func TestBuildWorkspaceCapabilityReportDenied(t *testing.T) {
	runner := &fakePortfolioCapabilityRunner{
		authStatus: `{"hosts":{"github.com":[{"state":"success","active":true,"host":"github.com","login":"alice","tokenSource":"env://GITHUB_TOKEN","scopes":""}]}}`,
		repos: map[string]string{
			"repos/StatPan/backlog":        `{"permissions":{"admin":false,"maintain":false,"pull":true,"push":false,"triage":true}}`,
			"repos/StatPan/backlog/issues": `[]`,
			"repos/StatPan/gira":           `{"permissions":{"admin":false,"maintain":false,"pull":false,"push":false,"triage":false}}`,
		},
	}
	config := WorkspaceConfigResolved{Name: "personal", Owner: "StatPan", InboxRepo: ParseRepoRefMust("StatPan/backlog"), Repos: []RepoRef{ParseRepoRefMust("StatPan/gira")}}

	report, err := BuildWorkspaceCapabilityReport(config, runner, time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildWorkspaceCapabilityReport error: %v", err)
	}
	if len(report.BlockedActions) == 0 {
		t.Fatalf("expected blocked actions: %+v", report)
	}
	text := FormatWorkspaceCapabilityReport(report)
	if !strings.Contains(text, "fix blocked repo permissions before workspace sync or route --apply") {
		t.Fatalf("text missing workspace remediation:\n%s", text)
	}
}
