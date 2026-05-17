package gira

import (
	"strings"
	"testing"
)

func TestBuildWorkspaceValidateReportClassifiesRoutingStates(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/backlog"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
	}
	client := fakeWorkspaceClient{
		inbox: []PortfolioRawTicket{
			{Number: 1, Title: "Needs target", State: "open", Body: portfolioBody("unrouted", "", "")},
			{Number: 2, Title: "Ready", State: "open", Body: portfolioBody("single_repo", "StatPan/gira", "")},
			{Number: 3, Title: "Routed", State: "open", Body: portfolioBody("single_repo", "StatPan/gira", "StatPan/gira#9")},
			{Number: 4, Title: "Done", State: "closed", Body: portfolioBody("single_repo", "StatPan/gira", "StatPan/gira#10")},
		},
	}

	report, err := BuildWorkspaceValidateReport(config, client)
	if err != nil {
		t.Fatalf("BuildWorkspaceValidateReport error: %v", err)
	}
	if report.Counts.NeedsRouting != 1 || report.Counts.Routeable != 1 || report.Counts.Routed != 1 || report.Counts.Done != 1 {
		t.Fatalf("counts = %+v", report.Counts)
	}
	if report.NextSteps[0] != "gira workspace ticket route --ticket 2 --repo StatPan/gira --dry-run" {
		t.Fatalf("next steps = %+v", report.NextSteps)
	}
	text := FormatWorkspaceValidateReport(report)
	if !strings.Contains(text, "scope=inbox-routing") || !strings.Contains(text, "routeable=1") || !strings.Contains(text, "#2 Ready scope=inbox-routing status=routeable") {
		t.Fatalf("formatted report missing routeable evidence:\n%s", text)
	}
}

func TestBuildWorkspaceValidateReportScopesExecutionRepoInboxToStatus(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
	}
	client := fakeWorkspaceClient{
		inbox: []PortfolioRawTicket{
			{Number: 10, Title: "Repo ready issue", State: "open", Body: "ordinary repo issue body", Labels: []string{"status:ready"}},
		},
	}

	report, err := BuildWorkspaceValidateReport(config, client)
	if err != nil {
		t.Fatalf("BuildWorkspaceValidateReport error: %v", err)
	}
	if report.Scope != "repo-execution" || report.Counts.Blocked != 0 || len(report.Items) != 0 {
		t.Fatalf("execution repo inbox should not be parsed as routing contract: %+v", report)
	}
	text := FormatWorkspaceValidateReport(report)
	for _, want := range []string{"scope=repo-execution", "blocked=0", "use workspace status for repo execution readiness"} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted report missing %q:\n%s", want, text)
		}
	}
}
