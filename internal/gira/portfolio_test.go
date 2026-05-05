package gira

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var portfolioNowFixture = time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)

func TestParsePortfolioTicketsValidSingleAndMultiRepo(t *testing.T) {
	repos := []RepoRef{mustRepoRefForPortfolio("StatPan/gira"), mustRepoRefForPortfolio("StatPan/docs")}
	tickets, diagnostics := ParsePortfolioTickets([]PortfolioRawTicket{
		{Number: 10, Title: "Single", State: "open", Body: portfolioBody("single_repo", "StatPan/gira", "")},
		{Number: 11, Title: "Multi", State: "open", Body: portfolioBody("multi_repo", "StatPan/gira\n- StatPan/docs", "StatPan/gira#10")},
	}, repos)

	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v, want none", diagnostics)
	}
	if tickets[0].Routing != "single_repo" || len(tickets[0].TargetRepos) != 1 {
		t.Fatalf("single ticket = %+v", tickets[0])
	}
	if tickets[1].Routing != "multi_repo" || len(tickets[1].TargetRepos) != 2 || len(tickets[1].ChildIssues) != 1 {
		t.Fatalf("multi ticket = %+v", tickets[1])
	}
}

func TestParsePortfolioTicketsDiagnostics(t *testing.T) {
	repos := []RepoRef{mustRepoRefForPortfolio("StatPan/gira")}
	_, diagnostics := ParsePortfolioTickets([]PortfolioRawTicket{
		{Number: 20, Title: "Invalid", State: "open", Body: portfolioBody("single_repo", "StatPan/docs", "")},
		{Number: 21, Title: "Missing", State: "open", Body: "routing: nonsense"},
	}, repos)

	var foundInvalidRepo, foundMissing, foundRouting bool
	for _, diag := range diagnostics {
		if diag.Ticket == 20 && diag.RuleID == "invalid_target_repo" {
			foundInvalidRepo = true
		}
		if diag.Ticket == 21 && diag.RuleID == "missing_required_field" {
			foundMissing = true
		}
		if diag.Ticket == 21 && diag.RuleID == "invalid_routing" {
			foundRouting = true
		}
	}
	if !foundInvalidRepo || !foundMissing || !foundRouting {
		t.Fatalf("diagnostics = %+v, want invalid repo, missing field, invalid routing", diagnostics)
	}
}

func TestPortfolioPlanActions(t *testing.T) {
	repos := []RepoRef{mustRepoRefForPortfolio("StatPan/gira"), mustRepoRefForPortfolio("StatPan/docs")}
	tickets, diagnostics := ParsePortfolioTickets([]PortfolioRawTicket{
		{Number: 30, Title: "Unrouted", State: "open", Body: portfolioBody("unrouted", "", "")},
		{Number: 31, Title: "Create", State: "open", Body: portfolioBody("single_repo", "StatPan/gira", "")},
		{Number: 32, Title: "Link", State: "open", Body: portfolioBody("multi_repo", "StatPan/gira\n- StatPan/docs", "StatPan/gira#10")},
		{Number: 33, Title: "Invalid", State: "open", Body: portfolioBody("single_repo", "StatPan/missing", "")},
	}, repos)

	actions := PortfolioPlan(tickets, diagnostics, repos)
	countByAction := map[string]int{}
	for _, action := range actions {
		countByAction[action.Action]++
	}
	if countByAction["ticket:needs_routing"] != 1 {
		t.Fatalf("actions = %+v, want one needs_routing", actions)
	}
	if countByAction["execution_issue:create"] != 1 {
		t.Fatalf("actions = %+v, want one create", actions)
	}
	if countByAction["execution_issue:link_existing"] != 2 {
		t.Fatalf("actions = %+v, want two link_existing", actions)
	}
	if countByAction["ticket:blocked_invalid_repo"] != 1 {
		t.Fatalf("actions = %+v, want one blocked", actions)
	}
}

func TestBuildPortfolioPlanReportJSONShape(t *testing.T) {
	client := fakePortfolioClient{
		repo: mustRepoRefForPortfolio("StatPan/portfolio"),
		tickets: []PortfolioRawTicket{
			{Number: 40, Title: "Create", State: "open", Body: portfolioBody("single_repo", "StatPan/gira", "")},
		},
	}
	report, err := BuildPortfolioPlanReport(client, []RepoRef{mustRepoRefForPortfolio("StatPan/gira")}, portfolioNowFixture)
	if err != nil {
		t.Fatalf("BuildPortfolioPlanReport error: %v", err)
	}
	if report.Command != "portfolio plan" || !report.DryRun || report.Counts.Actions != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	output, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	for _, key := range []string{"portfolio_repo", "repos", "dry_run", "counts", "tickets", "actions", "diagnostics"} {
		if !strings.Contains(string(output), `"`+key+`"`) {
			t.Fatalf("portfolio JSON missing %q:\n%s", key, output)
		}
	}
}

func TestResolvePortfolioConfigDoesNotRequireInitProfiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `portfolio:
  repo: StatPan/portfolio
  repos:
    - StatPan/gira
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	resolved, err := ResolvePortfolioConfig(path)
	if err != nil {
		t.Fatalf("ResolvePortfolioConfig error: %v", err)
	}
	if resolved.PortfolioRepo.FullName() != "StatPan/portfolio" || len(resolved.Repos) != 1 || resolved.Repos[0].FullName() != "StatPan/gira" {
		t.Fatalf("resolved = %+v", resolved)
	}
}

func TestResolvePortfolioConfigValidatesAllowlist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `portfolio:
  repo: StatPan/portfolio
  repos:
    - bad-format
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, err := ResolvePortfolioConfig(path)
	if err == nil {
		t.Fatal("expected invalid repo error")
	}
	if !strings.Contains(err.Error(), "portfolio.repos[0] must be in OWNER/REPO format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFormatPortfolioReportIncludesNextStep(t *testing.T) {
	report := PortfolioReport{Command: "portfolio status", PortfolioRepo: "StatPan/portfolio", Repos: []string{"StatPan/gira"}, Counts: PortfolioCounts{Tickets: 1, OpenTickets: 1, Unlinked: 1}}
	text := FormatPortfolioReport(report)
	if !strings.Contains(text, "next step: gira portfolio plan --dry-run --config .gira/config.yaml") {
		t.Fatalf("text missing next step:\n%s", text)
	}
}

type fakePortfolioClient struct {
	repo    RepoRef
	tickets []PortfolioRawTicket
}

func (c fakePortfolioClient) PortfolioRepo() RepoRef { return c.repo }
func (c fakePortfolioClient) FetchTickets() ([]PortfolioRawTicket, error) {
	return c.tickets, nil
}

func portfolioBody(routing, targetRepos, childIssues string) string {
	if targetRepos == "" {
		targetRepos = ""
	}
	return "## Goal\nShip portfolio intake\n\n## Scope\nPlan only\n\n## Routing\n" + routing + "\n\n## Target Repos\n" + targetRepos + "\n\n## Acceptance Criteria\n- plan is stable\n\n## Child Issues\n" + childIssues + "\n"
}

func mustRepoRefForPortfolio(value string) RepoRef {
	repo, err := ParseRepoRef(value)
	if err != nil {
		panic(err)
	}
	return repo
}
