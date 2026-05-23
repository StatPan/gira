package gira

import (
	"encoding/json"
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
	Name       string
	Owner      string
	InboxRepo  RepoRef
	Repos      []RepoRef
	Project    ProjectConfig
	Source     string
	ConfigPath string
}

type WorkspaceClient interface {
	FetchInboxTickets(repo RepoRef) ([]PortfolioRawTicket, error)
	FetchStatus(repo RepoRef, now time.Time, staleDays int) (StatusSummary, error)
	CreateInboxTicket(repo RepoRef, title string, body string) (WorkspaceTicketRef, error)
	CreateExecutionIssue(repo RepoRef, ticket PortfolioTicket, inboxRepo RepoRef) (PortfolioLoweredIssue, error)
	UpdateInboxTicketChildIssue(inboxRepo RepoRef, ticket PortfolioTicket, childIssue string) error
}

type WorkspaceQueueStatusClient interface {
	FetchQueueStatuses(repo RepoRef, summary StatusSummary, now time.Time, staleDays int) (WorkspaceQueueStatusSnapshot, error)
}

type WorkspaceQueueStatusSnapshot struct {
	Statuses []WorkStatusResult `json:"statuses"`
	Warnings []string           `json:"warnings,omitempty"`
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
	Workspace  WorkspaceSummary       `json:"workspace"`
	Source     string                 `json:"source,omitempty"`
	ConfigPath string                 `json:"config_path,omitempty"`
	Inbox      WorkspaceInbox         `json:"inbox"`
	Repos      []WorkspaceRepo        `json:"repos"`
	Counts     WorkspaceCounts        `json:"counts"`
	Queues     WorkspaceQueuesReport  `json:"workspace_queues"`
	Backlog    []WorkspaceBacklogItem `json:"backlog,omitempty"`
	RateLimit  *WorkspaceRateLimit    `json:"rate_limit,omitempty"`
	Cache      WorkspaceStatusCache   `json:"cache,omitempty"`
	Warnings   []string               `json:"warnings,omitempty"`
	NextSteps  []string               `json:"next_steps"`
	FetchedAt  string                 `json:"fetched_at"`
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

type WorkspaceRateLimit struct {
	Limit             int    `json:"limit"`
	Remaining         int    `json:"remaining"`
	ResetAt           string `json:"reset_at,omitempty"`
	EstimatedRequests int    `json:"estimated_requests"`
	BudgetOK          bool   `json:"budget_ok"`
}

type WorkspaceStatusCache struct {
	Enabled    bool   `json:"enabled"`
	Root       string `json:"root,omitempty"`
	TTLSeconds int    `json:"ttl_seconds,omitempty"`
	Hits       int    `json:"hits,omitempty"`
	Misses     int    `json:"misses,omitempty"`
	Writes     int    `json:"writes,omitempty"`
	Stale      int    `json:"stale,omitempty"`
}

type WorkspaceBacklogItem struct {
	Source       string   `json:"source"`
	Repo         string   `json:"repo"`
	Number       int      `json:"number"`
	Title        string   `json:"title"`
	State        string   `json:"state"`
	Status       string   `json:"status,omitempty"`
	Priority     string   `json:"priority,omitempty"`
	Milestone    string   `json:"milestone,omitempty"`
	URL          string   `json:"url,omitempty"`
	NeedsRouting bool     `json:"needs_routing,omitempty"`
	ChildIssues  []string `json:"child_issues,omitempty"`
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
	Command        string                 `json:"command"`
	Workspace      WorkspaceSummary       `json:"workspace"`
	InboxRepo      string                 `json:"inbox_repo"`
	Title          string                 `json:"title"`
	Created        *WorkspaceTicketRef    `json:"created,omitempty"`
	TargetRepo     string                 `json:"target_repo,omitempty"`
	DryRun         bool                   `json:"dry_run,omitempty"`
	Actions        []WorkspaceRouteAction `json:"actions,omitempty"`
	ExecutionIssue *PortfolioLoweredIssue `json:"execution_issue,omitempty"`
	NextSteps      []string               `json:"next_steps,omitempty"`
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
	if strings.TrimSpace(path) != "" {
		return resolveWorkspaceConfigFile(path, "explicit_config")
	}
	if resolved, ok, err := resolveWorkspaceConfigFromGlobalRegistry(""); err != nil {
		return WorkspaceConfigResolved{}, err
	} else if ok {
		return resolved, nil
	}
	return resolveWorkspaceConfigFile(DefaultInitConfigPath("."), "repo_local_contract")
}

func resolveWorkspaceConfigFile(path string, source string) (WorkspaceConfigResolved, error) {
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
		return WorkspaceConfigResolved{}, fmt.Errorf("workspace.inbox_repo is required in %s; repo-local configs from gira adopt repo are not workspace-ready. Run gira workspace init --inbox-repo OWNER/backlog --repo OWNER/repo --path . --dry-run, or pass --config to an existing workspace config", path)
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
	return WorkspaceConfigResolved{Name: name, Owner: owner, InboxRepo: inboxRepo, Repos: repos, Project: workspace.Project, Source: source, ConfigPath: path}, nil
}

func resolveWorkspaceConfigFromGlobalRegistry(configRoot string) (WorkspaceConfigResolved, bool, error) {
	root, err := globalConfigRoot(configRoot)
	if err != nil {
		return WorkspaceConfigResolved{}, false, err
	}
	if cfg, err := LoadGlobalConfig(root); err == nil && strings.TrimSpace(cfg.DefaultWorkspace) != "" {
		resolved, err := resolveGlobalWorkspaceConfig(root, cfg.DefaultWorkspace)
		if err != nil {
			return resolved, false, err
		}
		if err := verifyGlobalWorkspaceMatchesCheckout(resolved, root); err != nil {
			return WorkspaceConfigResolved{}, false, err
		}
		return resolved, true, nil
	}
	if ctx, ok, err := repoContextFromGlobalPath(root, "."); err != nil {
		return WorkspaceConfigResolved{}, false, err
	} else if ok && ctx.GlobalRepo != nil && strings.TrimSpace(ctx.GlobalRepo.Workspace.Name) != "" {
		resolved, err := resolveGlobalWorkspaceConfig(root, ctx.GlobalRepo.Workspace.Name)
		return resolved, err == nil, err
	}
	repo, hasRepo, err := repoContextFromConfig(DefaultInitConfigPath("."))
	if err != nil {
		return WorkspaceConfigResolved{}, false, err
	}
	if !hasRepo {
		if repoFromPath, ok, err := repoContextFromGlobalPath(root, "."); err != nil {
			return WorkspaceConfigResolved{}, false, err
		} else if ok {
			repo = repoFromPath.Repo
			hasRepo = true
		}
	}
	if !hasRepo {
		return WorkspaceConfigResolved{}, false, nil
	}
	return resolveGlobalWorkspaceContainingRepo(root, repo)
}

func verifyGlobalWorkspaceMatchesCheckout(workspace WorkspaceConfigResolved, configRoot string) error {
	repo, ok, err := repoContextFromGitOrigin(".", ExecCommandRunner{})
	if err != nil {
		return err
	}
	if !ok {
		repo, ok, err = repoContextFromConfig(DefaultInitConfigPath("."))
		if err != nil {
			return err
		}
	}
	if !ok {
		if ctx, ctxOK, err := repoContextFromGlobalPath(configRoot, "."); err != nil {
			return err
		} else if ctxOK {
			repo = ctx.Repo
			ok = true
		}
	}
	if !ok {
		return nil
	}
	if sameRepoRef(workspace.InboxRepo, repo) || workspaceContainsRepo(workspace.Repos, repo) {
		return nil
	}
	return fmt.Errorf("workspace config unavailable: global workspace %q does not contain checkout repo %s", workspace.Name, repo.FullName())
}

func resolveGlobalWorkspaceConfig(configRoot string, name string) (WorkspaceConfigResolved, error) {
	entry, err := LoadGlobalWorkspaceRegistryEntry(configRoot, name)
	if err != nil {
		return WorkspaceConfigResolved{}, err
	}
	path, err := GlobalWorkspaceRegistryPath(configRoot, name)
	if err != nil {
		return WorkspaceConfigResolved{}, err
	}
	resolved, err := resolveWorkspaceConfigFile(path, "global_workspace")
	if err != nil {
		return WorkspaceConfigResolved{}, err
	}
	resolved.Project = entry.Workspace.Project
	resolved.Source = "global_workspace"
	resolved.ConfigPath = path
	return resolved, nil
}

func resolveGlobalWorkspaceContainingRepo(configRoot string, repo RepoRef) (WorkspaceConfigResolved, bool, error) {
	root, err := GlobalWorkspacesRoot(configRoot)
	if err != nil {
		return WorkspaceConfigResolved{}, false, err
	}
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return WorkspaceConfigResolved{}, false, nil
		}
		return WorkspaceConfigResolved{}, false, err
	}
	var matches []WorkspaceConfigResolved
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".yaml") {
			return nil
		}
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		resolved, err := resolveGlobalWorkspaceConfig(configRoot, name)
		if err != nil {
			return err
		}
		if sameRepoRef(resolved.InboxRepo, repo) || workspaceContainsRepo(resolved.Repos, repo) {
			matches = append(matches, resolved)
		}
		return nil
	})
	if err != nil {
		return WorkspaceConfigResolved{}, false, err
	}
	if len(matches) > 1 {
		return WorkspaceConfigResolved{}, false, fmt.Errorf("workspace config unavailable: multiple global workspaces contain %s", repo.FullName())
	}
	if len(matches) == 1 {
		return matches[0], true, nil
	}
	return WorkspaceConfigResolved{}, false, nil
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
	tickets, err := NewGHPortfolioClient(repo, c.runner).FetchTickets()
	if err != nil {
		return nil, actionableGitHubStatusError(err)
	}
	return tickets, nil
}

func (c GHWorkspaceClient) FetchStatus(repo RepoRef, now time.Time, staleDays int) (StatusSummary, error) {
	summary, err := BuildStatusSummaryWithOptions(NewGHStatusClient(repo, c.runner), now, staleDays, StatusSummaryOptions{IncludePullRequests: false})
	if err != nil {
		return StatusSummary{}, actionableGitHubStatusError(err)
	}
	return summary, nil
}

func (c GHWorkspaceClient) FetchQueueStatuses(repo RepoRef, summary StatusSummary, _ time.Time, _ int) (WorkspaceQueueStatusSnapshot, error) {
	snapshot := WorkspaceQueueStatusSnapshot{}
	for _, issue := range summary.Issues.Open {
		status := workspaceQueueStatusFromIssue(summary.Repo, issue)
		if workspaceQueueStatusIs(status, "in-review") {
			detailed, err := GetWorkStatus(repo, issue.Number, c.runner)
			if err != nil {
				snapshot.Warnings = append(snapshot.Warnings, fmt.Sprintf("workspace queue detail unavailable for %s#%d: %v", repo.FullName(), issue.Number, err))
			} else {
				status = detailed
			}
		}
		snapshot.Statuses = append(snapshot.Statuses, status)
	}
	return snapshot, nil
}

func (c GHWorkspaceClient) FetchRateLimit() (WorkspaceRateLimit, error) {
	var raw struct {
		Resources struct {
			Core struct {
				Limit     int   `json:"limit"`
				Remaining int   `json:"remaining"`
				Reset     int64 `json:"reset"`
			} `json:"core"`
		} `json:"resources"`
		Rate struct {
			Limit     int   `json:"limit"`
			Remaining int   `json:"remaining"`
			Reset     int64 `json:"reset"`
		} `json:"rate"`
	}
	output, err := c.runner.Run("gh", "api", "rate_limit")
	if err != nil {
		return WorkspaceRateLimit{}, actionableGitHubStatusError(err)
	}
	if err := json.Unmarshal(output, &raw); err != nil {
		return WorkspaceRateLimit{}, fmt.Errorf("parse gh rate limit JSON: %w", err)
	}
	limit := raw.Resources.Core.Limit
	remaining := raw.Resources.Core.Remaining
	reset := raw.Resources.Core.Reset
	if limit == 0 && raw.Rate.Limit > 0 {
		limit = raw.Rate.Limit
		remaining = raw.Rate.Remaining
		reset = raw.Rate.Reset
	}
	resetAt := ""
	if reset > 0 {
		resetAt = time.Unix(reset, 0).UTC().Format(time.RFC3339)
	}
	return WorkspaceRateLimit{Limit: limit, Remaining: remaining, ResetAt: resetAt}, nil
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
	return BuildWorkspaceStatusReportWithOptions(config, client, now, staleDays, WorkspaceStatusOptions{})
}

func BuildWorkspaceStatusReportWithOptions(config WorkspaceConfigResolved, client WorkspaceClient, now time.Time, staleDays int, options WorkspaceStatusOptions) (WorkspaceReport, error) {
	repos, err := selectWorkspaceStatusRepos(config.Repos, options)
	if err != nil {
		return WorkspaceReport{}, err
	}
	report := WorkspaceReport{
		Workspace:  WorkspaceSummary{Name: config.Name, Owner: config.Owner},
		Source:     config.Source,
		ConfigPath: config.ConfigPath,
		Inbox:      WorkspaceInbox{Repo: config.InboxRepo.FullName()},
		FetchedAt:  now.UTC().Format(time.RFC3339),
	}
	cache, err := newWorkspaceStatusCache(options, now)
	if err != nil {
		return WorkspaceReport{}, err
	}
	report.Cache = cache.summary()
	if provider, ok := client.(WorkspaceRateLimitClient); ok {
		rateLimit, err := provider.FetchRateLimit()
		if err != nil {
			report.Warnings = append(report.Warnings, err.Error())
		} else {
			rateLimit.EstimatedRequests = estimateWorkspaceStatusRequests(config, repos)
			rateLimit.BudgetOK = rateLimit.Limit == 0 || rateLimit.Remaining >= rateLimit.EstimatedRequests
			report.RateLimit = &rateLimit
			if !rateLimit.BudgetOK {
				report.Warnings = append(report.Warnings, fmt.Sprintf("GitHub API budget low: remaining=%d estimated=%d reset=%s", rateLimit.Remaining, rateLimit.EstimatedRequests, rateLimit.ResetAt))
			}
		}
	}
	if !workspaceContainsRepo(config.Repos, config.InboxRepo) {
		tickets, err := client.FetchInboxTickets(config.InboxRepo)
		if err != nil {
			return WorkspaceReport{}, err
		}
		parsed, diagnostics := ParsePortfolioTickets(tickets, config.Repos)
		invalid := portfolioInvalidTickets(diagnostics)
		for _, ticket := range parsed {
			status := workspaceInboxStatus(ticket, invalid)
			item := WorkspaceBacklogItem{Source: "inbox", Repo: config.InboxRepo.FullName(), Number: ticket.Number, Title: ticket.Title, State: ticket.State, Status: status, Priority: ticket.Priority, URL: ticket.URL, NeedsRouting: status == "needs-routing", ChildIssues: append([]string(nil), ticket.ChildIssues...)}
			report.Backlog = append(report.Backlog, item)
			if !strings.EqualFold(ticket.State, "open") {
				continue
			}
			report.Inbox.Open++
			if item.NeedsRouting {
				report.Inbox.NeedsRouting++
			}
			if status == "ready-to-route" {
				report.Inbox.ExecutionReady++
			}
		}
	}
	summaries, err := fetchWorkspaceStatusSummaries(repos, client, now, staleDays, options, cache)
	if err != nil {
		return WorkspaceReport{}, err
	}
	report.Cache = cache.summary()
	queueStatuses := []WorkStatusResult{}
	queueDetailRequests := 0
	for _, summary := range summaries {
		repoView := workspaceRepoFromStatus(summary)
		if options.ActiveOnly && !workspaceRepoIsActive(repoView) {
			continue
		}
		addWorkspaceStatusSummary(&report, summary, repoView)
		if _, ok := client.(WorkspaceQueueStatusClient); ok {
			queueDetailRequests += estimateWorkspaceQueueDetailRequests(summary)
		}
		statuses, warnings := workspaceQueueStatuses(client, summary, now, staleDays)
		queueStatuses = append(queueStatuses, statuses...)
		report.Warnings = append(report.Warnings, warnings...)
	}
	report.Queues = BuildWorkspaceQueues(report.Workspace, queueStatuses)
	if report.RateLimit != nil && queueDetailRequests > 0 {
		report.RateLimit.EstimatedRequests += queueDetailRequests
		report.RateLimit.BudgetOK = report.RateLimit.Limit == 0 || report.RateLimit.Remaining >= report.RateLimit.EstimatedRequests
	}
	sortWorkspaceBacklog(report.Backlog)
	report.Counts.InboxOpen = report.Inbox.Open
	report.Counts.Backlog = len(report.Backlog)
	report.NextSteps = workspaceNextSteps(report, config.ConfigPath)
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
		report.NextSteps = []string{"review workspace sync plan", "gira workspace sync --apply --config " + workspaceNextStepConfigPath(config.ConfigPath)}
	} else {
		report.NextSteps = []string{"gira workspace status --config " + workspaceNextStepConfigPath(config.ConfigPath)}
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
	return WorkspaceTicketNewReport{Command: "workspace ticket new", Workspace: WorkspaceSummary{Name: config.Name, Owner: config.Owner}, InboxRepo: config.InboxRepo.FullName(), Title: title, Created: &created}, nil
}

func BuildWorkspaceTicketNewRouteReport(config WorkspaceConfigResolved, client WorkspaceClient, title string, body string, targetRepo RepoRef, dryRun bool) (WorkspaceTicketNewReport, error) {
	if strings.TrimSpace(title) == "" {
		return WorkspaceTicketNewReport{}, fmt.Errorf("--title is required")
	}
	if !workspaceContainsRepo(config.Repos, targetRepo) {
		return WorkspaceTicketNewReport{}, fmt.Errorf("%s is not in workspace.repos", targetRepo.FullName())
	}
	if strings.TrimSpace(body) == "" {
		body = renderWorkspaceInboxBody(title)
	}
	report := WorkspaceTicketNewReport{
		Command:    "workspace ticket new",
		Workspace:  WorkspaceSummary{Name: config.Name, Owner: config.Owner},
		InboxRepo:  config.InboxRepo.FullName(),
		Title:      title,
		TargetRepo: targetRepo.FullName(),
		DryRun:     dryRun,
		Actions: []WorkspaceRouteAction{
			{Action: "inbox_ticket:create", Repo: config.InboxRepo.FullName(), Reason: "capture workspace ticket before routing"},
			{Action: "execution_issue:create", Repo: targetRepo.FullName(), Reason: "route inbox ticket to execution repo"},
		},
	}
	if dryRun {
		report.NextSteps = []string{fmt.Sprintf("gira workspace ticket new %s --repo %s --apply", QuoteShellArg(title), QuoteShellArg(targetRepo.FullName()))}
		return report, nil
	}
	created, err := client.CreateInboxTicket(config.InboxRepo, title, body)
	if err != nil {
		return WorkspaceTicketNewReport{}, err
	}
	report.Created = &created
	ticket := normalizeWorkspaceRouteTicket(PortfolioTicket{
		Number: created.Number,
		Title:  title,
		State:  "open",
		URL:    created.URL,
		Body:   body,
		Goal:   title,
	}, targetRepo)
	executionIssue, err := client.CreateExecutionIssue(targetRepo, ticket, config.InboxRepo)
	if err != nil {
		return WorkspaceTicketNewReport{}, err
	}
	report.ExecutionIssue = &executionIssue
	childIssue := fmt.Sprintf("%s#%d", targetRepo.FullName(), executionIssue.Number)
	if err := client.UpdateInboxTicketChildIssue(config.InboxRepo, ticket, childIssue); err != nil {
		return WorkspaceTicketNewReport{}, err
	}
	report.Actions = append(report.Actions, WorkspaceRouteAction{Action: "inbox_ticket:update_child_issues", Repo: config.InboxRepo.FullName(), Reason: "link created execution issue"})
	report.NextSteps = []string{fmt.Sprintf("gira ticket start --repo %s --ticket %d --apply", QuoteShellArg(targetRepo.FullName()), executionIssue.Number)}
	return report, nil
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
		report.NextSteps = []string{fmt.Sprintf("gira ticket start --repo %s --ticket %d --dry-run", QuoteShellArg(targetRepo.FullName()), existing.Number)}
		return report, nil
	}
	if dryRun {
		report.NextSteps = []string{fmt.Sprintf("gira workspace ticket route --ticket %d --repo %s --apply", ticketNumber, QuoteShellArg(targetRepo.FullName()))}
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
	report.NextSteps = []string{fmt.Sprintf("gira ticket start --repo %s --ticket %d --dry-run", QuoteShellArg(targetRepo.FullName()), created.Number)}
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
	if strings.TrimSpace(report.Source) != "" {
		fmt.Fprintf(&b, "source: %s %s\n", report.Source, report.ConfigPath)
	}
	if report.RateLimit != nil {
		fmt.Fprintf(&b, "github budget: remaining=%d/%d estimated=%d reset=%s\n", report.RateLimit.Remaining, report.RateLimit.Limit, report.RateLimit.EstimatedRequests, report.RateLimit.ResetAt)
	}
	if report.Cache.Enabled {
		fmt.Fprintf(&b, "cache: ttl=%ds hits=%d misses=%d writes=%d stale=%d root=%s\n", report.Cache.TTLSeconds, report.Cache.Hits, report.Cache.Misses, report.Cache.Writes, report.Cache.Stale, report.Cache.Root)
	}
	for _, warning := range report.Warnings {
		fmt.Fprintf(&b, "warning: %s\n", warning)
	}
	fmt.Fprintf(&b, "inbox:     %s open=%d needs-routing=%d ready-to-route=%d\n", report.Inbox.Repo, report.Inbox.Open, report.Inbox.NeedsRouting, report.Inbox.ExecutionReady)
	b.WriteString("repos:\n")
	for _, repo := range report.Repos {
		fmt.Fprintf(&b, "  %s open=%d ready=%d in-progress=%d blocked=%d stale=%d", repo.Repo, repo.Open, repo.Ready, repo.InProgress, repo.Blocked, repo.Stale)
		if repo.ActiveMilestone != "" {
			fmt.Fprintf(&b, " milestone=%s %d%%", repo.ActiveMilestone, repo.ProgressPercent)
		}
		b.WriteString("\n")
	}
	if report.Queues.SchemaVersion != "" {
		fmt.Fprintf(&b, "queues:   agent-ready=%d review-needed=%d finish-ready=%d blocked=%d failed-check=%d human-decision=%d\n",
			report.Queues.Counts.AgentReady,
			report.Queues.Counts.ReviewNeeded,
			report.Queues.Counts.FinishReady,
			report.Queues.Counts.Blocked,
			report.Queues.Counts.FailedCheck,
			report.Queues.Counts.HumanDecision,
		)
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
	if report.TargetRepo == "" {
		if report.Created == nil {
			return fmt.Sprintf("workspace ticket new: %s\n", report.Title)
		}
		return fmt.Sprintf("workspace ticket new: %s#%d %s\nnext step: gira workspace ticket route --ticket %d --repo OWNER/REPO --dry-run\n", report.Created.Repo, report.Created.Number, report.Title, report.Created.Number)
	}
	var b strings.Builder
	mode := "apply"
	if report.DryRun {
		mode = "dry-run"
	}
	fmt.Fprintf(&b, "workspace ticket new: %s %s -> %s\n", mode, report.Title, report.TargetRepo)
	if report.Created != nil {
		fmt.Fprintf(&b, "inbox: %s#%d %s\n", report.Created.Repo, report.Created.Number, report.Created.URL)
	}
	if report.ExecutionIssue != nil {
		fmt.Fprintf(&b, "execution: %s#%d %s\n", report.ExecutionIssue.Repo, report.ExecutionIssue.Number, report.ExecutionIssue.URL)
	}
	for _, action := range report.Actions {
		fmt.Fprintf(&b, "  %s %s (%s)\n", action.Action, action.Repo, action.Reason)
	}
	if len(report.NextSteps) > 0 {
		fmt.Fprintf(&b, "next step: %s\n", report.NextSteps[0])
	}
	return b.String()
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
	if strings.EqualFold(ticket.State, "closed") {
		return "done"
	}
	if _, ok := invalid[ticket.Number]; ok {
		return "blocked"
	}
	if len(ticket.ChildIssues) > 0 {
		return "routed"
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

func workspaceQueueStatuses(client WorkspaceClient, summary StatusSummary, now time.Time, staleDays int) ([]WorkStatusResult, []string) {
	if provider, ok := client.(WorkspaceQueueStatusClient); ok {
		repo, err := ParseRepoRef(summary.Repo)
		if err != nil {
			return workspaceQueueStatusesFromSummary(summary), []string{fmt.Sprintf("workspace queue repo parse failed for %q: %v", summary.Repo, err)}
		}
		snapshot, err := provider.FetchQueueStatuses(repo, summary, now, staleDays)
		if err != nil {
			return workspaceQueueStatusesFromSummary(summary), []string{fmt.Sprintf("workspace queue detail unavailable for %s: %v", summary.Repo, err)}
		}
		return snapshot.Statuses, snapshot.Warnings
	}
	return workspaceQueueStatusesFromSummary(summary), nil
}

func workspaceQueueStatusesFromSummary(summary StatusSummary) []WorkStatusResult {
	statuses := make([]WorkStatusResult, 0, len(summary.Issues.Open))
	for _, issue := range summary.Issues.Open {
		statuses = append(statuses, workspaceQueueStatusFromIssue(summary.Repo, issue))
	}
	return statuses
}

func estimateWorkspaceQueueDetailRequests(summary StatusSummary) int {
	count := 0
	for _, issue := range summary.Issues.Open {
		if workspaceQueueStatusIs(workspaceQueueStatusFromIssue(summary.Repo, issue), "in-review") {
			count += 3
		}
	}
	return count
}

func workspaceQueueStatusFromIssue(repo string, issue IssueStats) WorkStatusResult {
	milestone := ""
	if issue.Milestone != nil {
		milestone = *issue.Milestone
	}
	status := statusFromLabels(issue.Labels)
	result := WorkStatusResult{
		Command:    "workspace status",
		Repo:       repo,
		Issue:      issue.Number,
		Title:      issue.Title,
		State:      issue.State,
		Status:     status,
		Labels:     append([]string(nil), issue.Labels...),
		Milestone:  milestone,
		NextAction: workspaceQueueSummaryNextAction(status),
	}
	if strings.EqualFold(status, "blocked") {
		result.Blockers = []string{"status_blocked"}
	}
	return result
}

func workspaceQueueSummaryNextAction(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready":
		return "start_work"
	case "blocked":
		return "inspect_blocker"
	case "in-review":
		return "inspect_review"
	default:
		return "inspect_status"
	}
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

func workspaceNextSteps(report WorkspaceReport, configPath string) []string {
	configArg := workspaceNextStepConfigPath(configPath)
	if report.Inbox.NeedsRouting > 0 {
		return []string{"gira workspace backlog --config " + configArg}
	}
	for _, item := range report.Backlog {
		if item.Source == "repo" && item.Status == "ready" {
			return []string{fmt.Sprintf("gira ticket start --repo %s --ticket %d --apply", QuoteShellArg(item.Repo), item.Number)}
		}
	}
	return []string{"gira workspace status --config " + configArg}
}

func workspaceNextStepConfigPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return QuoteShellArg(DefaultInitConfigPath("."))
	}
	return QuoteShellArg(path)
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
