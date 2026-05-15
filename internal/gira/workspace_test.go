package gira

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runGitCommand(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run()
}

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

func TestResolveWorkspaceConfigGlobalDefaultWorkspace(t *testing.T) {
	chdirTemp(t)
	root := defaultGlobalConfigRootForTest(t)
	writeTestFile(t, filepath.Join(root, "config.yaml"), "default_workspace: personal\n")
	writeTestFile(t, filepath.Join(root, "workspaces", "personal.yaml"), "workspace:\n  name: personal\n  owner: StatPan\n  inbox_repo: StatPan/backlog\n  repos:\n    - StatPan/gira\n")

	resolved, err := ResolveWorkspaceConfig("")
	if err != nil {
		t.Fatalf("ResolveWorkspaceConfig error: %v", err)
	}
	if resolved.Source != "global_workspace" || resolved.ConfigPath != filepath.Join(root, "workspaces", "personal.yaml") || resolved.InboxRepo.FullName() != "StatPan/backlog" {
		t.Fatalf("unexpected global workspace resolution: %+v", resolved)
	}
}

func TestResolveWorkspaceConfigGlobalWinsUnlessConfigExplicit(t *testing.T) {
	dir := chdirTemp(t)
	root := defaultGlobalConfigRootForTest(t)
	writeTestFile(t, filepath.Join(root, "config.yaml"), "default_workspace: personal\n")
	writeTestFile(t, filepath.Join(root, "workspaces", "personal.yaml"), "workspace:\n  name: personal\n  owner: StatPan\n  inbox_repo: StatPan/backlog\n  repos:\n    - StatPan/gira\n")
	localPath := filepath.Join(dir, ".gira", "config.yaml")
	writeTestFile(t, localPath, "workspace:\n  name: local\n  owner: Local\n  inbox_repo: Local/backlog\n  repos:\n    - Local/app\n")

	resolved, err := ResolveWorkspaceConfig("")
	if err != nil {
		t.Fatalf("ResolveWorkspaceConfig default error: %v", err)
	}
	if resolved.Source != "global_workspace" || resolved.InboxRepo.FullName() != "StatPan/backlog" {
		t.Fatalf("default should prefer global workspace, got %+v", resolved)
	}

	explicit, err := ResolveWorkspaceConfig(localPath)
	if err != nil {
		t.Fatalf("ResolveWorkspaceConfig explicit error: %v", err)
	}
	if explicit.Source != "explicit_config" || explicit.InboxRepo.FullName() != "Local/backlog" {
		t.Fatalf("explicit config should preserve repo-local behavior, got %+v", explicit)
	}
}

func TestResolveWorkspaceConfigRejectsGlobalWorkspaceForDifferentCheckout(t *testing.T) {
	dir := chdirTemp(t)
	root := defaultGlobalConfigRootForTest(t)
	writeTestFile(t, filepath.Join(root, "config.yaml"), "default_workspace: personal\n")
	writeTestFile(t, filepath.Join(root, "workspaces", "personal.yaml"), "workspace:\n  name: personal\n  owner: StatPan\n  inbox_repo: StatPan/backlog\n  repos:\n    - StatPan/gira\n")
	if err := runGitCommand(dir, "init"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := runGitCommand(dir, "remote", "add", "origin", "git@github.com:Other/repo.git"); err != nil {
		t.Fatalf("git remote add: %v", err)
	}

	_, err := ResolveWorkspaceConfig("")
	if err == nil || !strings.Contains(err.Error(), "does not contain checkout repo Other/repo") {
		t.Fatalf("error = %v, want checkout mismatch", err)
	}
}

func TestResolveWorkspaceConfigRepoLocalFallback(t *testing.T) {
	dir := chdirTemp(t)
	defaultGlobalConfigRootForTest(t)
	writeTestFile(t, filepath.Join(dir, ".gira", "config.yaml"), "workspace:\n  name: local\n  owner: StatPan\n  inbox_repo: StatPan/backlog\n  repos:\n    - StatPan/gira\n")

	resolved, err := ResolveWorkspaceConfig("")
	if err != nil {
		t.Fatalf("ResolveWorkspaceConfig error: %v", err)
	}
	if resolved.Source != "repo_local_contract" || resolved.InboxRepo.FullName() != "StatPan/backlog" {
		t.Fatalf("unexpected repo-local fallback: %+v", resolved)
	}
}

func TestResolveWorkspaceConfigMissingContext(t *testing.T) {
	chdirTemp(t)
	defaultGlobalConfigRootForTest(t)

	_, err := ResolveWorkspaceConfig("")
	if err == nil || !strings.Contains(err.Error(), ".gira/config.yaml") {
		t.Fatalf("error = %v, want missing repo-local config after global miss", err)
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

func TestBuildWorkspaceTicketNewRouteDryRunDoesNotMutate(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/backlog"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
	}
	client := &recordingWorkspaceClient{}

	report, err := BuildWorkspaceTicketNewRouteReport(config, client, "Route me", "", ParseRepoRefMust("StatPan/gira"), true)
	if err != nil {
		t.Fatalf("BuildWorkspaceTicketNewRouteReport error: %v", err)
	}
	if !report.DryRun || report.Created != nil || report.ExecutionIssue != nil {
		t.Fatalf("dry-run report = %+v", report)
	}
	if client.createdInbox != 0 || client.createdExecution != 0 || client.updatedInbox != 0 {
		t.Fatalf("dry-run mutated client: %+v", client)
	}
	if len(report.Actions) != 2 || report.Actions[0].Action != "inbox_ticket:create" || report.Actions[1].Action != "execution_issue:create" {
		t.Fatalf("actions = %+v", report.Actions)
	}
}

func TestBuildWorkspaceTicketNewRouteApplyCreatesAndLinks(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/backlog"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
	}
	client := &recordingWorkspaceClient{}

	report, err := BuildWorkspaceTicketNewRouteReport(config, client, "Route me", "", ParseRepoRefMust("StatPan/gira"), false)
	if err != nil {
		t.Fatalf("BuildWorkspaceTicketNewRouteReport error: %v", err)
	}
	if report.Created == nil || report.Created.Number != 9 {
		t.Fatalf("created inbox = %+v", report.Created)
	}
	if report.ExecutionIssue == nil || report.ExecutionIssue.Number != 10 {
		t.Fatalf("execution issue = %+v", report.ExecutionIssue)
	}
	if client.createdInbox != 1 || client.createdExecution != 1 || client.updatedInbox != 1 || client.childIssue != "StatPan/gira#10" {
		t.Fatalf("client mutations = %+v", client)
	}
	if report.NextSteps[0] != "gira ticket start --repo StatPan/gira --ticket 10 --apply" {
		t.Fatalf("next steps = %+v", report.NextSteps)
	}
}

func TestBuildWorkspaceTicketNewRouteRejectsRepoOutsideWorkspace(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/backlog"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
	}
	_, err := BuildWorkspaceTicketNewRouteReport(config, &recordingWorkspaceClient{}, "Route me", "", ParseRepoRefMust("Other/repo"), true)
	if err == nil || !strings.Contains(err.Error(), "is not in workspace.repos") {
		t.Fatalf("error = %v, want workspace.repos rejection", err)
	}
}

func TestBuildWorkspaceTicketRouteReusesExistingChildIssue(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/backlog"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
	}
	client := fakeWorkspaceClient{
		inbox: []PortfolioRawTicket{{Number: 5, Title: "Route me", State: "open", Body: portfolioBody("single_repo", "StatPan/gira", "StatPan/gira#77")}},
	}

	report, err := BuildWorkspaceTicketRouteReport(config, client, 5, ParseRepoRefMust("StatPan/gira"), false)
	if err != nil {
		t.Fatalf("BuildWorkspaceTicketRouteReport error: %v", err)
	}
	if len(report.Actions) != 1 || report.Actions[0].Action != "execution_issue:reuse" || report.Created == nil || report.Created.Number != 77 {
		t.Fatalf("expected existing child issue reuse, got %+v", report)
	}
	if report.NextSteps[0] != "gira ticket start --repo StatPan/gira --ticket 77 --dry-run" {
		t.Fatalf("next steps = %+v", report.NextSteps)
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

func TestBuildWorkspaceStatusReportNormalizesRoutedAndClosedInboxTickets(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/backlog"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
	}
	client := fakeWorkspaceClient{
		inbox: []PortfolioRawTicket{
			{Number: 1, Title: "Routed", State: "open", Body: portfolioBody("single_repo", "StatPan/gira", "StatPan/gira#10")},
			{Number: 2, Title: "Closed", State: "closed", Body: portfolioBody("unrouted", "", "")},
		},
		status: map[string]StatusSummary{
			"StatPan/gira": {Repo: "StatPan/gira"},
		},
	}

	report, err := BuildWorkspaceStatusReport(config, client, time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC), 14)
	if err != nil {
		t.Fatalf("BuildWorkspaceStatusReport error: %v", err)
	}
	if report.Inbox.Open != 1 || report.Inbox.NeedsRouting != 0 || report.Inbox.ExecutionReady != 0 {
		t.Fatalf("unexpected inbox counts: %+v", report.Inbox)
	}
	byNumber := map[int]WorkspaceBacklogItem{}
	for _, item := range report.Backlog {
		byNumber[item.Number] = item
	}
	if byNumber[1].Status != "routed" || len(byNumber[1].ChildIssues) != 1 || byNumber[1].NeedsRouting {
		t.Fatalf("routed item = %+v", byNumber[1])
	}
	if byNumber[2].Status != "done" || byNumber[2].NeedsRouting {
		t.Fatalf("closed item = %+v", byNumber[2])
	}
}

func TestBuildWorkspaceStatusReportWithOptionsCachesRepoStatus(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/gira"),
		Repos:     []RepoRef{ParseRepoRefMust("StatPan/gira")},
	}
	client := &countingWorkspaceClient{
		status: map[string]StatusSummary{
			"StatPan/gira": {Repo: "StatPan/gira", Counts: StatusCounts{Issues: IssueCounts{Open: 1}}},
		},
	}
	options := WorkspaceStatusOptions{CacheRoot: t.TempDir(), CacheTTL: time.Hour, MaxConcurrency: 1}
	now := time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC)

	first, err := BuildWorkspaceStatusReportWithOptions(config, client, now, 14, options)
	if err != nil {
		t.Fatalf("first BuildWorkspaceStatusReportWithOptions error: %v", err)
	}
	second, err := BuildWorkspaceStatusReportWithOptions(config, client, now.Add(time.Minute), 14, options)
	if err != nil {
		t.Fatalf("second BuildWorkspaceStatusReportWithOptions error: %v", err)
	}
	if client.fetchStatusCalls != 1 {
		t.Fatalf("FetchStatus calls = %d, want 1 cache-backed reuse", client.fetchStatusCalls)
	}
	if first.Cache.Misses != 1 || first.Cache.Writes != 1 || second.Cache.Hits != 1 {
		t.Fatalf("unexpected cache summaries first=%+v second=%+v", first.Cache, second.Cache)
	}
}

func TestBuildWorkspaceStatusReportWithOptionsNarrowsReposAndReportsRateBudget(t *testing.T) {
	config := WorkspaceConfigResolved{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: ParseRepoRefMust("StatPan/inbox"),
		Repos: []RepoRef{
			ParseRepoRefMust("StatPan/app-a"),
			ParseRepoRefMust("StatPan/app-b"),
			ParseRepoRefMust("StatPan/app-c"),
		},
	}
	client := &rateLimitWorkspaceClient{
		fakeWorkspaceClient: fakeWorkspaceClient{
			status: map[string]StatusSummary{
				"StatPan/app-b": {Repo: "StatPan/app-b", Counts: StatusCounts{Issues: IssueCounts{Open: 1}}},
			},
		},
		rateLimit: WorkspaceRateLimit{Limit: 5000, Remaining: 2},
	}
	report, err := BuildWorkspaceStatusReportWithOptions(config, client, time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC), 14, WorkspaceStatusOptions{
		Repos: []RepoRef{ParseRepoRefMust("StatPan/app-b")},
	})
	if err != nil {
		t.Fatalf("BuildWorkspaceStatusReportWithOptions error: %v", err)
	}
	if len(report.Repos) != 1 || report.Repos[0].Repo != "StatPan/app-b" {
		t.Fatalf("repos were not narrowed: %+v", report.Repos)
	}
	if report.RateLimit == nil || report.RateLimit.EstimatedRequests != 3 || report.RateLimit.BudgetOK {
		t.Fatalf("unexpected rate limit report: %+v", report.RateLimit)
	}
	if len(report.Warnings) == 0 || !strings.Contains(report.Warnings[0], "GitHub API budget low") {
		t.Fatalf("missing budget warning: %+v", report.Warnings)
	}
}

type fakeWorkspaceClient struct {
	inbox  []PortfolioRawTicket
	status map[string]StatusSummary
}

type countingWorkspaceClient struct {
	fakeWorkspaceClient
	status           map[string]StatusSummary
	fetchStatusCalls int
}

type rateLimitWorkspaceClient struct {
	fakeWorkspaceClient
	rateLimit WorkspaceRateLimit
}

type recordingWorkspaceClient struct {
	createdInbox     int
	createdExecution int
	updatedInbox     int
	childIssue       string
}

func (c *recordingWorkspaceClient) FetchInboxTickets(repo RepoRef) ([]PortfolioRawTicket, error) {
	return nil, nil
}

func (c *recordingWorkspaceClient) FetchStatus(repo RepoRef, now time.Time, staleDays int) (StatusSummary, error) {
	return StatusSummary{}, nil
}

func (c *recordingWorkspaceClient) CreateInboxTicket(repo RepoRef, title string, body string) (WorkspaceTicketRef, error) {
	c.createdInbox++
	return WorkspaceTicketRef{Repo: repo.FullName(), Number: 9, URL: "https://github.com/" + repo.FullName() + "/issues/9"}, nil
}

func (c *recordingWorkspaceClient) CreateExecutionIssue(repo RepoRef, ticket PortfolioTicket, inboxRepo RepoRef) (PortfolioLoweredIssue, error) {
	c.createdExecution++
	return PortfolioLoweredIssue{Repo: repo.FullName(), Number: 10, URL: "https://github.com/" + repo.FullName() + "/issues/10"}, nil
}

func (c *recordingWorkspaceClient) UpdateInboxTicketChildIssue(inboxRepo RepoRef, ticket PortfolioTicket, childIssue string) error {
	c.updatedInbox++
	c.childIssue = childIssue
	return nil
}

func (c fakeWorkspaceClient) FetchInboxTickets(repo RepoRef) ([]PortfolioRawTicket, error) {
	return c.inbox, nil
}

func (c fakeWorkspaceClient) FetchStatus(repo RepoRef, now time.Time, staleDays int) (StatusSummary, error) {
	return c.status[repo.FullName()], nil
}

func (c *countingWorkspaceClient) FetchInboxTickets(repo RepoRef) ([]PortfolioRawTicket, error) {
	return nil, nil
}

func (c *countingWorkspaceClient) FetchStatus(repo RepoRef, now time.Time, staleDays int) (StatusSummary, error) {
	c.fetchStatusCalls++
	return c.status[repo.FullName()], nil
}

func (c *countingWorkspaceClient) CreateInboxTicket(repo RepoRef, title string, body string) (WorkspaceTicketRef, error) {
	return WorkspaceTicketRef{}, nil
}

func (c *countingWorkspaceClient) CreateExecutionIssue(repo RepoRef, ticket PortfolioTicket, inboxRepo RepoRef) (PortfolioLoweredIssue, error) {
	return PortfolioLoweredIssue{}, nil
}

func (c *countingWorkspaceClient) UpdateInboxTicketChildIssue(inboxRepo RepoRef, ticket PortfolioTicket, childIssue string) error {
	return nil
}

func (c *rateLimitWorkspaceClient) FetchRateLimit() (WorkspaceRateLimit, error) {
	return c.rateLimit, nil
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

func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	return dir
}

func defaultGlobalConfigRootForTest(t *testing.T) string {
	t.Helper()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	return filepath.Join(xdg, "gira")
}
