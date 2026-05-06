package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveWorkspaceConfigValidAndPortfolioAlias(t *testing.T) {
	dir := t.TempDir()
	workspacePath := filepath.Join(dir, "workspace.yaml")
	if err := os.WriteFile(workspacePath, []byte(`workspace:
  name: personal
  owner: StatPan
  inbox_repo: StatPan/gira-inbox
  repos:
    - StatPan/gira
`), 0o644); err != nil {
		t.Fatalf("write workspace config: %v", err)
	}
	resolved, err := ResolveWorkspaceConfig(workspacePath)
	if err != nil {
		t.Fatalf("ResolveWorkspaceConfig error: %v", err)
	}
	if resolved.Name != "personal" || resolved.Owner != "StatPan" || resolved.InboxRepo.FullName() != "StatPan/gira-inbox" || len(resolved.Repos) != 1 {
		t.Fatalf("resolved workspace = %+v", resolved)
	}

	portfolioPath := filepath.Join(dir, "portfolio.yaml")
	if err := os.WriteFile(portfolioPath, []byte(`portfolio:
  repo: StatPan/backlog
  repos:
    - StatPan/gira
`), 0o644); err != nil {
		t.Fatalf("write portfolio config: %v", err)
	}
	resolved, err = ResolveWorkspaceConfig(portfolioPath)
	if err != nil {
		t.Fatalf("ResolveWorkspaceConfig portfolio alias error: %v", err)
	}
	if resolved.InboxRepo.FullName() != "StatPan/backlog" || resolved.Owner != "StatPan" || resolved.Name != "personal" {
		t.Fatalf("portfolio alias resolved = %+v", resolved)
	}
}

func TestBuildWorkspaceStatusReportAggregatesInboxAndRepos(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/backlog"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
	}
	milestone := "v1.1 Dogfood"
	client := fakeWorkspaceClient{
		inbox: []PortfolioRawTicket{
			{Number: 1, Title: "Route docs", State: "open", Body: portfolioBody("unrouted", "", ""), URL: "https://github.com/StatPan/backlog/issues/1"},
			{Number: 2, Title: "Ready route", State: "open", Body: portfolioBody("single_repo", "StatPan/gira", ""), URL: "https://github.com/StatPan/backlog/issues/2"},
		},
		status: map[string]StatusSummary{
			"StatPan/gira": {
				Repo: "StatPan/gira",
				Counts: StatusCounts{
					Issues:     IssueCounts{Open: 2, StaleOpen: 1, BlockedOpen: 1},
					Milestones: MilestoneCounts{Total: 1, Open: 1},
				},
				Issues: StatusIssueLists{Open: []IssueStats{
					{Number: 10, Title: "Ready issue", State: "open", Labels: []string{"status:ready", "priority:p1"}, Milestone: &milestone, URL: "https://github.com/StatPan/gira/issues/10"},
					{Number: 11, Title: "Blocked issue", State: "open", Labels: []string{"status:blocked"}, Milestone: &milestone, URL: "https://github.com/StatPan/gira/issues/11"},
				}},
				Milestones: []MilestoneStats{{Title: milestone, State: "open", OpenIssues: 2, ClosedIssues: 1, TotalIssues: 3, ProgressPercent: 33}},
			},
		},
	}

	report, err := BuildWorkspaceStatusReport(config, client, time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC), 14)
	if err != nil {
		t.Fatalf("BuildWorkspaceStatusReport error: %v", err)
	}
	if report.Inbox.Open != 2 || report.Inbox.NeedsRouting != 1 || report.Inbox.ExecutionReady != 1 {
		t.Fatalf("inbox = %+v, want open=2 needs=1 ready=1", report.Inbox)
	}
	if report.Counts.Backlog != 4 || report.Counts.RepoOpen != 2 || report.Counts.Ready != 1 || report.Counts.Blocked != 1 || report.Counts.Stale != 1 {
		t.Fatalf("counts = %+v", report.Counts)
	}
	if len(report.Repos) != 1 || report.Repos[0].ActiveMilestone != milestone || report.Repos[0].ProgressPercent != 33 {
		t.Fatalf("repos = %+v", report.Repos)
	}
	text := FormatWorkspaceReport(report)
	for _, want := range []string{"workspace: personal", "inbox:", "repos:", "backlog:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("workspace text missing %q:\n%s", want, text)
		}
	}
}

func TestBuildWorkspaceTicketRouteDryRun(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/backlog"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
	}
	client := fakeWorkspaceClient{
		inbox: []PortfolioRawTicket{{Number: 5, Title: "Route me", State: "open", Body: portfolioBody("unrouted", "", "")}},
	}

	report, err := BuildWorkspaceTicketRouteReport(config, client, 5, ParseRepoRefMust("StatPan/gira"), true)
	if err != nil {
		t.Fatalf("BuildWorkspaceTicketRouteReport error: %v", err)
	}
	if !report.DryRun || len(report.Actions) != 1 || report.Actions[0].Action != "execution_issue:create" {
		t.Fatalf("report = %+v", report)
	}
}

func TestBuildWorkspaceStatusReportSkipsInboxParsingWhenInboxIsExecutionRepo(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
	}
	client := fakeWorkspaceClient{
		inbox: []PortfolioRawTicket{{Number: 1, Title: "Normal repo issue", State: "open", Body: "not a portfolio ticket"}},
		status: map[string]StatusSummary{
			"StatPan/gira": {
				Repo:   "StatPan/gira",
				Counts: StatusCounts{Issues: IssueCounts{Open: 1}},
				Issues: StatusIssueLists{Open: []IssueStats{{Number: 1, Title: "Normal repo issue", State: "open", Labels: []string{"status:ready"}}}},
			},
		},
	}

	report, err := BuildWorkspaceStatusReport(config, client, time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC), 14)
	if err != nil {
		t.Fatalf("BuildWorkspaceStatusReport error: %v", err)
	}
	if report.Inbox.Open != 0 || report.Counts.Backlog != 1 || len(report.Backlog) != 1 || report.Backlog[0].Source != "repo" {
		t.Fatalf("report = %+v", report)
	}
}

type fakeWorkspaceClient struct {
	inbox  []PortfolioRawTicket
	status map[string]StatusSummary
}

func (c fakeWorkspaceClient) FetchInboxTickets(repo RepoRef) ([]PortfolioRawTicket, error) {
	return c.inbox, nil
}

func (c fakeWorkspaceClient) FetchStatus(repo RepoRef, now time.Time, staleDays int) (StatusSummary, error) {
	return c.status[repo.FullName()], nil
}

func (c fakeWorkspaceClient) CreateInboxTicket(repo RepoRef, title string, body string) (WorkspaceTicketRef, error) {
	return WorkspaceTicketRef{Repo: repo.FullName(), Number: 9, URL: "https://github.com/" + repo.FullName() + "/issues/9"}, nil
}

func (c fakeWorkspaceClient) CreateExecutionIssue(repo RepoRef, ticket PortfolioTicket, inboxRepo RepoRef) (PortfolioLoweredIssue, error) {
	return PortfolioLoweredIssue{Repo: repo.FullName(), Number: 10, URL: "https://github.com/" + repo.FullName() + "/issues/10"}, nil
}

func (c fakeWorkspaceClient) UpdateInboxTicketChildIssue(inboxRepo RepoRef, ticket PortfolioTicket, childIssue string) error {
	return nil
}
