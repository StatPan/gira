package gira

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

type WorkspaceConfigResolved struct {
	Name      string
	Owner     string
	InboxRepo RepoRef
	Repos     []RepoRef
	Project   ProjectConfig
}

type WorkspaceClient interface {
	FetchInboxTickets(repo RepoRef) ([]PortfolioRawTicket, error)
	FetchStatus(repo RepoRef, now time.Time, staleDays int) (StatusSummary, error)
	CreateInboxTicket(repo RepoRef, title string, body string) (WorkspaceTicketRef, error)
	CreateExecutionIssue(repo RepoRef, ticket PortfolioTicket, inboxRepo RepoRef) (PortfolioLoweredIssue, error)
	UpdateInboxTicketChildIssue(inboxRepo RepoRef, ticket PortfolioTicket, childIssue string) error
}

type GHWorkspaceClient struct {
	runner CommandRunner
}

func NewGHWorkspaceClient(runner CommandRunner) GHWorkspaceClient {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	return GHWorkspaceClient{runner: runner}
}

type WorkspaceReport struct {
	Workspace WorkspaceSummary       `json:"workspace"`
	Inbox     WorkspaceInbox         `json:"inbox"`
	Repos     []WorkspaceRepo        `json:"repos"`
	Counts    WorkspaceCounts        `json:"counts"`
	Backlog   []WorkspaceBacklogItem `json:"backlog,omitempty"`
	NextSteps []string               `json:"next_steps"`
	FetchedAt string                 `json:"fetched_at"`
}

type WorkspaceSummary struct {
	Name  string `json:"name"`
	Owner string `json:"owner"`
}

type WorkspaceInbox struct {
	Repo           string `json:"repo"`
	Open           int    `json:"open"`
	NeedsRouting   int    `json:"needs_routing"`
	ExecutionReady int    `json:"execution_ready"`
}

type WorkspaceRepo struct {
	Repo            string `json:"repo"`
	Open            int    `json:"open"`
	Ready           int    `json:"ready"`
	InProgress      int    `json:"in_progress"`
	Blocked         int    `json:"blocked"`
	Stale           int    `json:"stale"`
	ActiveMilestone string `json:"active_milestone,omitempty"`
	ProgressPercent int    `json:"progress_percent,omitempty"`
}

type WorkspaceCounts struct {
	Backlog    int `json:"backlog"`
	InboxOpen  int `json:"inbox_open"`
	RepoOpen   int `json:"repo_open"`
	Ready      int `json:"ready"`
	InProgress int `json:"in_progress"`
	Blocked    int `json:"blocked"`
	Stale      int `json:"stale"`
}

type WorkspaceBacklogItem struct {
	Source       string `json:"source"`
	Repo         string `json:"repo"`
	Number       int    `json:"number"`
	Title        string `json:"title"`
	State        string `json:"state"`
	Status       string `json:"status,omitempty"`
	Priority     string `json:"priority,omitempty"`
	Milestone    string `json:"milestone,omitempty"`
	URL          string `json:"url,omitempty"`
	NeedsRouting bool   `json:"needs_routing,omitempty"`
}

type WorkspaceSyncReport struct {
	Workspace WorkspaceSummary          `json:"workspace"`
	Command   string                    `json:"command"`
	DryRun    bool                      `json:"dry_run"`
	Repos     []WorkspaceSyncRepoReport `json:"repos"`
	Counts    WorkspaceSyncCounts       `json:"counts"`
	NextSteps []string                  `json:"next_steps"`
}

type WorkspaceSyncRepoReport struct {
	Repo                  string `json:"repo"`
	Role                  string `json:"role"`
	LabelsCreate          int    `json:"labels_create"`
	LabelsUpdate          int    `json:"labels_update"`
	MilestonesCreate      int    `json:"milestones_create"`
	MilestonesUpdate      int    `json:"milestones_update"`
	BootstrapIssuesCreate int    `json:"bootstrap_issues_create"`
}

type WorkspaceSyncCounts struct {
	Repos                 int `json:"repos"`
	LabelsCreate          int `json:"labels_create"`
	LabelsUpdate          int `json:"labels_update"`
	MilestonesCreate      int `json:"milestones_create"`
	MilestonesUpdate      int `json:"milestones_update"`
	BootstrapIssuesCreate int `json:"bootstrap_issues_create"`
}

type WorkspaceTicketRef struct {
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	URL    string `json:"url"`
}

type WorkspaceTicketNewReport struct {
	Command   string             `json:"command"`
	Workspace WorkspaceSummary   `json:"workspace"`
	InboxRepo string             `json:"inbox_repo"`
	Title     string             `json:"title"`
	Created   WorkspaceTicketRef `json:"created"`
}

type WorkspaceTicketRouteReport struct {
	Command    string                 `json:"command"`
	Workspace  WorkspaceSummary       `json:"workspace"`
	InboxRepo  string                 `json:"inbox_repo"`
	Ticket     int                    `json:"ticket"`
	TargetRepo string                 `json:"target_repo"`
	DryRun     bool                   `json:"dry_run"`
	Actions    []WorkspaceRouteAction `json:"actions"`
	Created    *PortfolioLoweredIssue `json:"created,omitempty"`
	NextSteps  []string               `json:"next_steps"`
}

type WorkspaceRouteAction struct {
	Action string `json:"action"`
	Repo   string `json:"repo"`
	Reason string `json:"reason"`
	Issue  int    `json:"issue,omitempty"`
}

type WorkspaceProjectPlanReport struct {
	Workspace WorkspaceSummary    `json:"workspace"`
	Command   string              `json:"command"`
	Repos     []ProjectSyncReport `json:"repos"`
	NextSteps []string            `json:"next_steps"`
}

func ResolveWorkspaceConfig(path string) (WorkspaceConfigResolved, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultInitConfigPath(".")
	}
	cfg, err := loadWorkspaceConfig(path)
	if err != nil {
		return WorkspaceConfigResolved{}, err
	}
	workspace := cfg.Workspace
	if strings.TrimSpace(workspace.InboxRepo) == "" && strings.TrimSpace(cfg.Portfolio.Repo) != "" {
		workspace.InboxRepo = cfg.Portfolio.Repo
		workspace.Repos = append([]string{}, cfg.Portfolio.Repos...)
	}
	if strings.TrimSpace(workspace.InboxRepo) == "" {
		return WorkspaceConfigResolved{}, fmt.Errorf("workspace.inbox_repo is required in %s", path)
	}
	if len(workspace.Repos) == 0 {
		return WorkspaceConfigResolved{}, fmt.Errorf("workspace.repos must include at least one execution repo in %s", path)
	}
	inboxRepo, err := ParseRepoRef(workspace.InboxRepo)
	if err != nil {
		return WorkspaceConfigResolved{}, fmt.Errorf("workspace.inbox_repo must be in OWNER/REPO format")
	}
	repos := make([]RepoRef, 0, len(workspace.Repos))
	seen := map[string]struct{}{}
	for i, value := range workspace.Repos {
		repo, err := ParseRepoRef(value)
		if err != nil {
			return WorkspaceConfigResolved{}, fmt.Errorf("workspace.repos[%d] must be in OWNER/REPO format", i)
		}
		key := strings.ToLower(repo.FullName())
		if _, ok := seen[key]; ok {
			return WorkspaceConfigResolved{}, fmt.Errorf("workspace.repos[%d] duplicates %s", i, repo.FullName())
		}
		seen[key] = struct{}{}
		repos = append(repos, repo)
	}
	name := strings.TrimSpace(workspace.Name)
	if name == "" {
		name = "personal"
	}
	owner := strings.TrimSpace(workspace.Owner)
	if owner == "" {
		owner = inboxRepo.Owner
	}
	return WorkspaceConfigResolved{Name: name, Owner: owner, InboxRepo: inboxRepo, Repos: repos, Project: workspace.Project}, nil
}

func loadWorkspaceConfig(path string) (InitConfig, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return InitConfig{}, fmt.Errorf("read workspace config %q: %w", path, err)
	}
	var cfg InitConfig
	switch strings.ToLower(filepath.Ext(path)) {
	case ".toml":
		if err := toml.Unmarshal(content, &cfg); err != nil {
			return InitConfig{}, fmt.Errorf("parse workspace config %q: %w", path, err)
		}
	default:
		if err := yaml.Unmarshal(content, &cfg); err != nil {
			return InitConfig{}, fmt.Errorf("parse workspace config %q: %w", path, err)
		}
	}
	return cfg, nil
}

func (c GHWorkspaceClient) FetchInboxTickets(repo RepoRef) ([]PortfolioRawTicket, error) {
	return NewGHPortfolioClient(repo, c.runner).FetchTickets()
}

func (c GHWorkspaceClient) FetchStatus(repo RepoRef, now time.Time, staleDays int) (StatusSummary, error) {
	return BuildStatusSummary(NewGHStatusClient(repo, c.runner), now, staleDays)
}

func (c GHWorkspaceClient) CreateInboxTicket(repo RepoRef, title string, body string) (WorkspaceTicketRef, error) {
	output, err := c.runner.Run("gh", "issue", "create", "--repo", repo.FullName(), "--title", title, "--body", body, "--label", "type:epic", "--label", "status:ready")
	if err != nil {
		return WorkspaceTicketRef{}, err
	}
	url := strings.TrimSpace(string(output))
	return WorkspaceTicketRef{Repo: repo.FullName(), Number: extractIssueNumber(url), URL: url}, nil
}

func (c GHWorkspaceClient) CreateExecutionIssue(repo RepoRef, ticket PortfolioTicket, inboxRepo RepoRef) (PortfolioLoweredIssue, error) {
	return NewGHPortfolioLowerClient(inboxRepo, c.runner).CreateExecutionIssue(repo, ticket, inboxRepo)
}

func (c GHWorkspaceClient) UpdateInboxTicketChildIssue(inboxRepo RepoRef, ticket PortfolioTicket, childIssue string) error {
	return NewGHPortfolioLowerClient(inboxRepo, c.runner).UpdatePortfolioChildIssues(ticket, []string{childIssue})
}

func BuildWorkspaceStatusReport(config WorkspaceConfigResolved, client WorkspaceClient, now time.Time, staleDays int) (WorkspaceReport, error) {
	report := WorkspaceReport{
		Workspace: WorkspaceSummary{Name: config.Name, Owner: config.Owner},
		Inbox:     WorkspaceInbox{Repo: config.InboxRepo.FullName()},
		FetchedAt: now.UTC().Format(time.RFC3339),
	}
	if !workspaceContainsRepo(config.Repos, config.InboxRepo) {
		tickets, err := client.FetchInboxTickets(config.InboxRepo)
		if err != nil {
			return WorkspaceReport{}, err
		}
		parsed, diagnostics := ParsePortfolioTickets(tickets, config.Repos)
		invalid := portfolioInvalidTickets(diagnostics)
		for _, ticket := range parsed {
			if !strings.EqualFold(ticket.State, "open") {
				continue
			}
			report.Inbox.Open++
			status := workspaceInboxStatus(ticket, invalid)
			item := WorkspaceBacklogItem{Source: "inbox", Repo: config.InboxRepo.FullName(), Number: ticket.Number, Title: ticket.Title, State: ticket.State, Status: status, Priority: ticket.Priority, URL: ticket.URL, NeedsRouting: status == "needs-routing"}
			report.Backlog = append(report.Backlog, item)
			if item.NeedsRouting {
				report.Inbox.NeedsRouting++
			}
			if status == "ready-to-route" {
				report.Inbox.ExecutionReady++
			}
		}
	}
	for _, repo := range config.Repos {
		summary, err := client.FetchStatus(repo, now, staleDays)
		if err != nil {
			return WorkspaceReport{}, err
		}
		repoView := workspaceRepoFromStatus(summary)
		report.Repos = append(report.Repos, repoView)
		report.Counts.RepoOpen += repoView.Open
		report.Counts.Ready += repoView.Ready
		report.Counts.InProgress += repoView.InProgress
		report.Counts.Blocked += repoView.Blocked
		report.Counts.Stale += repoView.Stale
		for _, issue := range summary.Issues.Open {
			report.Backlog = append(report.Backlog, workspaceBacklogFromIssue(repo.FullName(), issue))
		}
	}
	sortWorkspaceBacklog(report.Backlog)
	report.Counts.InboxOpen = report.Inbox.Open
	report.Counts.Backlog = len(report.Backlog)
	report.NextSteps = workspaceNextSteps(report)
	return report, nil
}

func BuildWorkspaceSyncReport(config WorkspaceConfigResolved, syncer func(RepoRef, bool, bool) (SyncPlan, error), dryRun bool, bootstrapIssues bool) (WorkspaceSyncReport, error) {
	report := WorkspaceSyncReport{Workspace: WorkspaceSummary{Name: config.Name, Owner: config.Owner}, Command: "workspace sync", DryRun: dryRun}
	repos := append([]RepoRef{config.InboxRepo}, config.Repos...)
	for i, repo := range repos {
		plan, err := syncer(repo, dryRun, bootstrapIssues)
		if err != nil {
			return WorkspaceSyncReport{}, err
		}
		role := "execution"
		if i == 0 {
			role = "inbox"
		}
		row := WorkspaceSyncRepoReport{
			Repo:                  repo.FullName(),
			Role:                  role,
			LabelsCreate:          countLabelActions(plan.Labels, PlanCreate),
			LabelsUpdate:          countLabelActions(plan.Labels, PlanUpdate),
			MilestonesCreate:      countMilestoneActions(plan.Milestones, PlanCreate),
			MilestonesUpdate:      countMilestoneActions(plan.Milestones, PlanUpdate),
			BootstrapIssuesCreate: countBootstrapIssueActions(plan.BootstrapIssues, PlanCreate),
		}
		report.Repos = append(report.Repos, row)
		report.Counts.Repos++
		report.Counts.LabelsCreate += row.LabelsCreate
		report.Counts.LabelsUpdate += row.LabelsUpdate
		report.Counts.MilestonesCreate += row.MilestonesCreate
		report.Counts.MilestonesUpdate += row.MilestonesUpdate
		report.Counts.BootstrapIssuesCreate += row.BootstrapIssuesCreate
	}
	if dryRun {
		report.NextSteps = []string{"review workspace sync plan", "gira workspace sync --apply --config .gira/config.yaml"}
	} else {
		report.NextSteps = []string{"gira workspace status --config .gira/config.yaml"}
	}
	return report, nil
}

func BuildWorkspaceTicketNewReport(config WorkspaceConfigResolved, client WorkspaceClient, title string, body string) (WorkspaceTicketNewReport, error) {
	if strings.TrimSpace(title) == "" {
		return WorkspaceTicketNewReport{}, fmt.Errorf("--title is required")
	}
	if strings.TrimSpace(body) == "" {
		body = renderWorkspaceInboxBody(title)
	}
	created, err := client.CreateInboxTicket(config.InboxRepo, title, body)
	if err != nil {
		return WorkspaceTicketNewReport{}, err
	}
	return WorkspaceTicketNewReport{Command: "workspace ticket new", Workspace: WorkspaceSummary{Name: config.Name, Owner: config.Owner}, InboxRepo: config.InboxRepo.FullName(), Title: title, Created: created}, nil
}

func BuildWorkspaceTicketRouteReport(config WorkspaceConfigResolved, client WorkspaceClient, ticketNumber int, targetRepo RepoRef, dryRun bool) (WorkspaceTicketRouteReport, error) {
	if ticketNumber <= 0 {
		return WorkspaceTicketRouteReport{}, fmt.Errorf("--ticket is required")
	}
	if !workspaceContainsRepo(config.Repos, targetRepo) {
		return WorkspaceTicketRouteReport{}, fmt.Errorf("%s is not in workspace.repos", targetRepo.FullName())
	}
	tickets, err := client.FetchInboxTickets(config.InboxRepo)
	if err != nil {
		return WorkspaceTicketRouteReport{}, err
	}
	parsed, _ := ParsePortfolioTickets(tickets, append(config.Repos, targetRepo))
	var ticket PortfolioTicket
	found := false
	for _, candidate := range parsed {
		if candidate.Number == ticketNumber {
			ticket = normalizeWorkspaceRouteTicket(candidate, targetRepo)
			found = true
			break
		}
	}
	if !found {
		return WorkspaceTicketRouteReport{}, fmt.Errorf("workspace inbox ticket #%d not found", ticketNumber)
	}
	report := WorkspaceTicketRouteReport{
		Command:    "workspace ticket route",
		Workspace:  WorkspaceSummary{Name: config.Name, Owner: config.Owner},
		InboxRepo:  config.InboxRepo.FullName(),
		Ticket:     ticketNumber,
		TargetRepo: targetRepo.FullName(),
		DryRun:     dryRun,
		Actions:    []WorkspaceRouteAction{{Action: "execution_issue:create", Repo: targetRepo.FullName(), Reason: "route inbox ticket to execution repo"}},
	}
	if existing, ok := childIssueRefs(ticket.ChildIssues)[targetRepo.FullName()]; ok {
		report.Actions = []WorkspaceRouteAction{{Action: "execution_issue:reuse", Repo: targetRepo.FullName(), Issue: existing.Number, Reason: "inbox ticket already links this execution repo"}}
		report.Created = &PortfolioLoweredIssue{Repo: targetRepo.FullName(), Number: existing.Number, URL: fmt.Sprintf("https://github.com/%s/issues/%d", targetRepo.FullName(), existing.Number)}
		report.NextSteps = []string{fmt.Sprintf("gira ticket start --repo %s --ticket %d --dry-run", targetRepo.FullName(), existing.Number)}
		return report, nil
	}
	if dryRun {
		report.NextSteps = []string{fmt.Sprintf("gira workspace ticket route --ticket %d --repo %s --apply", ticketNumber, targetRepo.FullName())}
		return report, nil
	}
	created, err := client.CreateExecutionIssue(targetRepo, ticket, config.InboxRepo)
	if err != nil {
		return WorkspaceTicketRouteReport{}, err
	}
	report.Created = &created
	childIssue := fmt.Sprintf("%s#%d", targetRepo.FullName(), created.Number)
	if err := client.UpdateInboxTicketChildIssue(config.InboxRepo, ticket, childIssue); err != nil {
		return WorkspaceTicketRouteReport{}, err
	}
	report.Actions = append(report.Actions, WorkspaceRouteAction{Action: "inbox_ticket:update_child_issues", Repo: config.InboxRepo.FullName(), Reason: "link created execution issue"})
	report.NextSteps = []string{fmt.Sprintf("gira ticket start --repo %s --ticket %d --dry-run", targetRepo.FullName(), created.Number)}
	return report, nil
}

func BuildWorkspaceProjectPlanReport(config WorkspaceConfigResolved, builder func(RepoRef) (ProjectSyncReport, error)) (WorkspaceProjectPlanReport, error) {
	report := WorkspaceProjectPlanReport{Workspace: WorkspaceSummary{Name: config.Name, Owner: config.Owner}, Command: "workspace project plan", NextSteps: []string{"GitHub Projects v2 mutation is not part of this slice"}}
	for _, repo := range config.Repos {
		item, err := builder(repo)
		if err != nil {
			return WorkspaceProjectPlanReport{}, err
		}
		report.Repos = append(report.Repos, item)
	}
	return report, nil
}

func FormatWorkspaceReport(report WorkspaceReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "workspace: %s (%s)\n", report.Workspace.Name, report.Workspace.Owner)
	fmt.Fprintf(&b, "inbox:     %s open=%d needs-routing=%d ready-to-route=%d\n", report.Inbox.Repo, report.Inbox.Open, report.Inbox.NeedsRouting, report.Inbox.ExecutionReady)
	b.WriteString("repos:\n")
	for _, repo := range report.Repos {
		fmt.Fprintf(&b, "  %s open=%d ready=%d in-progress=%d blocked=%d stale=%d", repo.Repo, repo.Open, repo.Ready, repo.InProgress, repo.Blocked, repo.Stale)
		if repo.ActiveMilestone != "" {
			fmt.Fprintf(&b, " milestone=%s %d%%", repo.ActiveMilestone, repo.ProgressPercent)
		}
		b.WriteString("\n")
	}
	if len(report.Backlog) > 0 {
		b.WriteString("backlog:\n")
		for _, item := range report.Backlog {
			fmt.Fprintf(&b, "  %s#%d %-15s %s\n", item.Repo, item.Number, item.Status, item.Title)
		}
	}
	if len(report.NextSteps) > 0 {
		fmt.Fprintf(&b, "next step: %s\n", report.NextSteps[0])
	}
	return b.String()
}

func FormatWorkspaceSyncReport(report WorkspaceSyncReport) string {
	var b strings.Builder
	mode := "apply"
	if report.DryRun {
		mode = "dry-run"
	}
	fmt.Fprintf(&b, "workspace sync: %s\n", mode)
	fmt.Fprintf(&b, "workspace: %s (%s)\n", report.Workspace.Name, report.Workspace.Owner)
	for _, repo := range report.Repos {
		fmt.Fprintf(&b, "  %s %s labels create=%d update=%d milestones create=%d update=%d bootstrap create=%d\n", repo.Role, repo.Repo, repo.LabelsCreate, repo.LabelsUpdate, repo.MilestonesCreate, repo.MilestonesUpdate, repo.BootstrapIssuesCreate)
	}
	if len(report.NextSteps) > 0 {
		fmt.Fprintf(&b, "next step: %s\n", report.NextSteps[0])
	}
	return b.String()
}

func FormatWorkspaceTicketNewReport(report WorkspaceTicketNewReport) string {
	return fmt.Sprintf("workspace ticket new: %s#%d %s\nnext step: gira workspace ticket route --ticket %d --repo OWNER/REPO --dry-run\n", report.Created.Repo, report.Created.Number, report.Title, report.Created.Number)
}

func FormatWorkspaceTicketRouteReport(report WorkspaceTicketRouteReport) string {
	var b strings.Builder
	mode := "apply"
	if report.DryRun {
		mode = "dry-run"
	}
	fmt.Fprintf(&b, "workspace ticket route: %s inbox#%d -> %s\n", mode, report.Ticket, report.TargetRepo)
	for _, action := range report.Actions {
		issue := ""
		if action.Issue > 0 {
			issue = fmt.Sprintf("#%d ", action.Issue)
		}
		fmt.Fprintf(&b, "  %s %s %s(%s)\n", action.Action, action.Repo, issue, action.Reason)
	}
	if report.Created != nil {
		fmt.Fprintf(&b, "created: %s#%d %s\n", report.TargetRepo, report.Created.Number, report.Created.URL)
	}
	if len(report.NextSteps) > 0 {
		fmt.Fprintf(&b, "next step: %s\n", report.NextSteps[0])
	}
	return b.String()
}

func workspaceInboxStatus(ticket PortfolioTicket, invalid map[int]struct{}) string {
	if _, ok := invalid[ticket.Number]; ok {
		return "blocked"
	}
	if ticket.Routing == "single_repo" || ticket.Routing == "multi_repo" {
		return "ready-to-route"
	}
	return "needs-routing"
}

func workspaceRepoFromStatus(summary StatusSummary) WorkspaceRepo {
	view := WorkspaceRepo{Repo: summary.Repo, Open: summary.Counts.Issues.Open, Blocked: summary.Counts.Issues.BlockedOpen, Stale: summary.Counts.Issues.StaleOpen}
	for _, issue := range summary.Issues.Open {
		status := statusFromLabels(issue.Labels)
		switch status {
		case "ready":
			view.Ready++
		case "in-progress":
			view.InProgress++
		}
	}
	for _, milestone := range summary.Milestones {
		if milestone.State == "open" && milestone.TotalIssues > 0 {
			view.ActiveMilestone = milestone.Title
			view.ProgressPercent = milestone.ProgressPercent
			break
		}
	}
	return view
}

func workspaceBacklogFromIssue(repo string, issue IssueStats) WorkspaceBacklogItem {
	status := statusFromLabels(issue.Labels)
	priority := ""
	for _, label := range issue.Labels {
		if strings.HasPrefix(label, "priority:") {
			priority = label
		}
	}
	milestone := ""
	if issue.Milestone != nil {
		milestone = *issue.Milestone
	}
	return WorkspaceBacklogItem{Source: "repo", Repo: repo, Number: issue.Number, Title: issue.Title, State: issue.State, Status: status, Priority: priority, Milestone: milestone, URL: issue.URL}
}

func statusFromLabels(labels []string) string {
	for _, label := range labels {
		if strings.HasPrefix(label, "status:") {
			return strings.TrimPrefix(label, "status:")
		}
	}
	return "open"
}

func sortWorkspaceBacklog(items []WorkspaceBacklogItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Source != items[j].Source {
			return items[i].Source < items[j].Source
		}
		if items[i].Repo != items[j].Repo {
			return items[i].Repo < items[j].Repo
		}
		return items[i].Number < items[j].Number
	})
}

func workspaceNextSteps(report WorkspaceReport) []string {
	if report.Inbox.NeedsRouting > 0 {
		return []string{"gira workspace backlog --config .gira/config.yaml"}
	}
	for _, item := range report.Backlog {
		if item.Source == "repo" && item.Status == "ready" {
			return []string{fmt.Sprintf("gira ticket start --repo %s --ticket %d --apply", item.Repo, item.Number)}
		}
	}
	return []string{"gira workspace status --config .gira/config.yaml"}
}

func renderWorkspaceInboxBody(title string) string {
	return fmt.Sprintf("## Goal\n%s\n\n## Scope\nTriage this workspace inbox ticket and route it to an execution repo when ready.\n\n## Routing\nunrouted\n\n## Target Repos\n_No response_\n\n## Acceptance Criteria\n- ticket is routed or explicitly deferred\n\n## Child Issues\n_No response_\n", title)
}

func workspaceContainsRepo(repos []RepoRef, want RepoRef) bool {
	for _, repo := range repos {
		if strings.EqualFold(repo.FullName(), want.FullName()) {
			return true
		}
	}
	return false
}

func normalizeWorkspaceRouteTicket(ticket PortfolioTicket, targetRepo RepoRef) PortfolioTicket {
	if strings.TrimSpace(ticket.Goal) == "" {
		ticket.Goal = ticket.Title
	}
	if strings.TrimSpace(ticket.Scope) == "" {
		ticket.Scope = "Route workspace inbox ticket into " + targetRepo.FullName()
	}
	if strings.TrimSpace(ticket.AcceptanceCriteria) == "" {
		ticket.AcceptanceCriteria = "- execution issue is created\n- parent inbox ticket links the child issue"
	}
	ticket.Routing = "single_repo"
	ticket.TargetRepos = []string{targetRepo.FullName()}
	return ticket
}

func appendChildIssueLink(body string, repo string, issue int) string {
	link := fmt.Sprintf("%s#%d", repo, issue)
	if strings.Contains(body, link) {
		return body
	}
	if strings.TrimSpace(body) == "" {
		return "## Child Issues\n" + link + "\n"
	}
	if strings.Contains(strings.ToLower(body), "## child issues") || strings.Contains(strings.ToLower(body), "### child issues") {
		return strings.TrimRight(body, "\n") + "\n- " + link + "\n"
	}
	return strings.TrimRight(body, "\n") + "\n\n## Child Issues\n- " + link + "\n"
}

func WorkspaceTicketNumber(value string) (int, error) {
	value = strings.TrimSpace(strings.TrimPrefix(value, "inbox#"))
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("--ticket must be a positive number or inbox#N")
	}
	return n, nil
}
