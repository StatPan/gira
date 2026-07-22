package gira

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const VisualPortfolioReportSchemaVersion = "visual-portfolio-report/v1alpha1"

type VisualPortfolioReportOptions struct {
	Repos      []RepoRef
	Milestones []string
	Since      string
	Until      string
	Now        time.Time
}

type VisualPortfolioReport struct {
	Command       string                      `json:"command"`
	SchemaVersion string                      `json:"schema_version"`
	GeneratedAt   string                      `json:"generated_at"`
	Filters       VisualPortfolioFilters      `json:"filters"`
	Summary       VisualPortfolioSummary      `json:"summary"`
	Repositories  []VisualPortfolioRepository `json:"repositories"`
	Milestones    []VisualPortfolioMilestone  `json:"milestones"`
	Timeline      []VisualPortfolioTimeline   `json:"timeline"`
	Queues        []VisualPortfolioQueueItem  `json:"queues"`
	Sources       []VisualPortfolioSource     `json:"sources"`
	Unsupported   []string                    `json:"unsupported"`
	Warnings      []string                    `json:"warnings,omitempty"`
}

type VisualPortfolioFilters struct {
	Repositories []string `json:"repositories"`
	Milestones   []string `json:"milestones,omitempty"`
	Since        string   `json:"since,omitempty"`
	Until        string   `json:"until,omitempty"`
}

type VisualPortfolioSummary struct {
	Repositories       int `json:"repositories"`
	AvailableRepos     int `json:"available_repositories"`
	Milestones         int `json:"milestones"`
	OpenIssues         int `json:"open_issues"`
	ClosedIssues       int `json:"closed_issues"`
	BlockedItems       int `json:"blocked_items"`
	ReviewWaitingItems int `json:"review_waiting_items"`
}

type VisualPortfolioRepository struct {
	Repo       string   `json:"repo"`
	Status     string   `json:"status"`
	OpenIssues int      `json:"open_issues"`
	Milestones int      `json:"milestones"`
	Warnings   []string `json:"warnings,omitempty"`
}

type VisualPortfolioMilestone struct {
	Repo              string `json:"repo"`
	Number            int    `json:"number"`
	Title             string `json:"title"`
	State             string `json:"state"`
	DueOn             string `json:"due_on,omitempty"`
	ScheduleState     string `json:"schedule_state"`
	OpenIssues        int    `json:"open_issues"`
	ClosedIssues      int    `json:"closed_issues"`
	TotalIssues       int    `json:"total_issues"`
	CompletionPercent int    `json:"completion_percent"`
	URL               string `json:"url"`
	SourceContract    string `json:"source_contract"`
	Trace             string `json:"trace"`
}

type VisualPortfolioTimeline struct {
	Repo           string `json:"repo"`
	Title          string `json:"title"`
	Kind           string `json:"kind"`
	Date           string `json:"date"`
	URL            string `json:"url,omitempty"`
	SourceContract string `json:"source_contract"`
	Trace          string `json:"trace"`
}

type VisualPortfolioQueueItem struct {
	Queue          string `json:"queue"`
	Repo           string `json:"repo"`
	Number         int    `json:"number"`
	Title          string `json:"title"`
	Status         string `json:"status"`
	Milestone      string `json:"milestone,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
	URL            string `json:"url"`
	SourceContract string `json:"source_contract"`
}

type VisualPortfolioSource struct {
	Repo       string `json:"repo"`
	Contract   string `json:"contract"`
	Status     string `json:"status"`
	SnapshotAt string `json:"snapshot_at"`
	Reason     string `json:"reason,omitempty"`
}

func BuildVisualPortfolioReport(options VisualPortfolioReportOptions, dashboardFactory func(RepoRef) DashboardExportClient, reviewFactory func(RepoRef) ReviewGateClient) (VisualPortfolioReport, error) {
	if len(options.Repos) == 0 {
		return VisualPortfolioReport{}, fmt.Errorf("at least one repository is required")
	}
	if dashboardFactory == nil || reviewFactory == nil {
		return VisualPortfolioReport{}, fmt.Errorf("portfolio report clients are required")
	}
	now := options.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	since, until, err := parseVisualPortfolioWindow(options.Since, options.Until)
	if err != nil {
		return VisualPortfolioReport{}, err
	}

	report := VisualPortfolioReport{
		Command:       "report portfolio",
		SchemaVersion: VisualPortfolioReportSchemaVersion,
		GeneratedAt:   now.Format(time.RFC3339),
		Filters: VisualPortfolioFilters{
			Repositories: []string{},
			Milestones:   normalizedVisualPortfolioFilters(options.Milestones),
			Since:        strings.TrimSpace(options.Since),
			Until:        strings.TrimSpace(options.Until),
		},
		Unsupported: []string{"product_outcome_confidence: no stable outcome-evidence contract is part of this report; shown as unsupported rather than inferred"},
	}

	repos := append([]RepoRef(nil), options.Repos...)
	sort.Slice(repos, func(i, j int) bool { return repos[i].FullName() < repos[j].FullName() })
	for _, repo := range repos {
		report.Filters.Repositories = append(report.Filters.Repositories, repo.FullName())
		buildVisualPortfolioRepo(&report, repo, now, since, until, dashboardFactory(repo), reviewFactory(repo))
	}
	finalizeVisualPortfolioReport(&report)
	return report, nil
}

func buildVisualPortfolioRepo(report *VisualPortfolioReport, repo RepoRef, now time.Time, since, until *time.Time, dashboard DashboardExportClient, review ReviewGateClient) {
	repoName := repo.FullName()
	repoSummary := VisualPortfolioRepository{Repo: repoName, Status: "available"}
	snapshotAt := report.GeneratedAt

	issues, issuesErr := dashboard.FetchIssues()
	report.Sources = append(report.Sources, visualPortfolioSource(repoName, "github-issues/v1", snapshotAt, issuesErr))
	if issuesErr != nil {
		repoSummary.Status = "partial"
		repoSummary.Warnings = append(repoSummary.Warnings, "issues_unavailable")
	} else {
		for _, issue := range issues {
			if strings.EqualFold(issue.State, "open") {
				repoSummary.OpenIssues++
			}
			if !strings.EqualFold(issue.State, "open") || !visualPortfolioMilestoneMatches(issue.Milestone, report.Filters.Milestones) || !visualPortfolioDateInWindow(issue.UpdatedAt, since, until, true) || !visualPortfolioBlocked(issue.Labels) {
				continue
			}
			report.Queues = append(report.Queues, VisualPortfolioQueueItem{Queue: "blocked", Repo: repoName, Number: issue.IssueNumber, Title: issue.Title, Status: visualPortfolioStatus(issue.Labels, "blocked"), Milestone: valueOrUnknown(issue.Milestone), UpdatedAt: issue.UpdatedAt, URL: issue.URL, SourceContract: "github-issues/v1"})
		}
	}

	milestones, milestonesErr := dashboard.FetchMilestones()
	report.Sources = append(report.Sources, visualPortfolioSource(repoName, "github-milestones/v1", snapshotAt, milestonesErr))
	if milestonesErr != nil {
		repoSummary.Status = "partial"
		repoSummary.Warnings = append(repoSummary.Warnings, "milestones_unavailable")
	} else {
		matched := map[string]bool{}
		for _, milestone := range milestones {
			if !visualPortfolioMilestoneMatches(milestone.Title, report.Filters.Milestones) {
				continue
			}
			matched[milestone.Title] = true
			total := milestone.OpenIssues + milestone.ClosedIssues
			percent := 0
			if total > 0 {
				percent = (milestone.ClosedIssues * 100) / total
			}
			due := ""
			if milestone.DueOn != nil {
				due = strings.TrimSpace(*milestone.DueOn)
			}
			item := VisualPortfolioMilestone{Repo: repoName, Number: milestone.MilestoneNumber, Title: milestone.Title, State: milestone.State, DueOn: due, ScheduleState: visualPortfolioScheduleState(due, milestone.State, now), OpenIssues: milestone.OpenIssues, ClosedIssues: milestone.ClosedIssues, TotalIssues: total, CompletionPercent: percent, URL: fmt.Sprintf("https://github.com/%s/milestone/%d", repoName, milestone.MilestoneNumber), SourceContract: "github-milestones/v1", Trace: fmt.Sprintf("closed_issues=%d / total_issues=%d from GitHub milestone counters", milestone.ClosedIssues, total)}
			report.Milestones = append(report.Milestones, item)
			report.Summary.OpenIssues += milestone.OpenIssues
			report.Summary.ClosedIssues += milestone.ClosedIssues
			if due != "" && visualPortfolioDateInWindow(due, since, until, false) {
				report.Timeline = append(report.Timeline, VisualPortfolioTimeline{Repo: repoName, Title: milestone.Title, Kind: "milestone_due", Date: due[:10], URL: item.URL, SourceContract: "github-milestones/v1", Trace: "milestone.due_on"})
			}
		}
		repoSummary.Milestones = len(matched)
		for _, filter := range report.Filters.Milestones {
			if !matched[filter] {
				repoSummary.Warnings = append(repoSummary.Warnings, "milestone_not_found:"+filter)
			}
		}
	}

	project, projectErr := dashboard.FetchProjectSnapshot()
	report.Sources = append(report.Sources, visualPortfolioSource(repoName, "product-os-project-snapshot/v1", snapshotAt, projectErr))
	if projectErr != nil {
		repoSummary.Status = "partial"
		repoSummary.Warnings = append(repoSummary.Warnings, "roadmap_dates_unavailable")
	} else {
		for _, item := range project.RoadmapItems {
			if item.StartDate != nil && visualPortfolioDateInWindow(*item.StartDate, since, until, false) {
				report.Timeline = append(report.Timeline, VisualPortfolioTimeline{Repo: repoName, Title: item.IssueTitle, Kind: "work_start", Date: (*item.StartDate)[:10], URL: item.IssueURL, SourceContract: "product-os-project-snapshot/v1", Trace: "ProjectV2 Start date"})
			}
			if item.TargetDate != nil && visualPortfolioDateInWindow(*item.TargetDate, since, until, false) {
				report.Timeline = append(report.Timeline, VisualPortfolioTimeline{Repo: repoName, Title: item.IssueTitle, Kind: "named_gate", Date: (*item.TargetDate)[:10], URL: item.IssueURL, SourceContract: "product-os-project-snapshot/v1", Trace: "ProjectV2 Target date"})
			}
		}
	}

	prs, prsErr := review.ListOpenPRs()
	report.Sources = append(report.Sources, visualPortfolioSource(repoName, "review-gate/v1", snapshotAt, prsErr))
	if prsErr != nil {
		repoSummary.Status = "partial"
		repoSummary.Warnings = append(repoSummary.Warnings, "review_queue_unavailable")
	} else {
		for _, pr := range prs {
			if strings.EqualFold(pr.ReviewDecision, "APPROVED") || !visualPortfolioDateInWindow(pr.UpdatedAt, since, until, true) {
				continue
			}
			status := "review_required"
			if pr.IsDraft {
				status = "draft"
			} else if strings.EqualFold(pr.ReviewDecision, "CHANGES_REQUESTED") {
				status = "changes_requested"
			}
			report.Queues = append(report.Queues, VisualPortfolioQueueItem{Queue: "review_waiting", Repo: repoName, Number: pr.Number, Title: pr.Title, Status: status, Milestone: "unsupported", UpdatedAt: pr.UpdatedAt, URL: pr.URL, SourceContract: "review-gate/v1"})
		}
	}

	if len(repoSummary.Warnings) >= 4 {
		repoSummary.Status = "unavailable"
	}
	report.Repositories = append(report.Repositories, repoSummary)
}

func finalizeVisualPortfolioReport(report *VisualPortfolioReport) {
	sort.Slice(report.Milestones, func(i, j int) bool {
		if report.Milestones[i].Repo == report.Milestones[j].Repo {
			return report.Milestones[i].Number < report.Milestones[j].Number
		}
		return report.Milestones[i].Repo < report.Milestones[j].Repo
	})
	sort.Slice(report.Timeline, func(i, j int) bool {
		if report.Timeline[i].Date == report.Timeline[j].Date {
			if report.Timeline[i].Repo == report.Timeline[j].Repo {
				return report.Timeline[i].Title < report.Timeline[j].Title
			}
			return report.Timeline[i].Repo < report.Timeline[j].Repo
		}
		return report.Timeline[i].Date < report.Timeline[j].Date
	})
	sort.Slice(report.Queues, func(i, j int) bool {
		if report.Queues[i].Queue == report.Queues[j].Queue {
			if report.Queues[i].Repo == report.Queues[j].Repo {
				return report.Queues[i].Number < report.Queues[j].Number
			}
			return report.Queues[i].Repo < report.Queues[j].Repo
		}
		return report.Queues[i].Queue < report.Queues[j].Queue
	})
	report.Summary.Repositories = len(report.Repositories)
	report.Summary.Milestones = len(report.Milestones)
	for _, repo := range report.Repositories {
		if repo.Status != "unavailable" {
			report.Summary.AvailableRepos++
		}
	}
	for _, item := range report.Queues {
		if item.Queue == "blocked" {
			report.Summary.BlockedItems++
		} else if item.Queue == "review_waiting" {
			report.Summary.ReviewWaitingItems++
		}
	}
}

func parseVisualPortfolioWindow(sinceRaw, untilRaw string) (*time.Time, *time.Time, error) {
	parse := func(name, value string) (*time.Time, error) {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, nil
		}
		parsed, err := time.Parse("2006-01-02", value)
		if err != nil {
			return nil, fmt.Errorf("--%s must use YYYY-MM-DD", name)
		}
		return &parsed, nil
	}
	since, err := parse("since", sinceRaw)
	if err != nil {
		return nil, nil, err
	}
	until, err := parse("until", untilRaw)
	if err != nil {
		return nil, nil, err
	}
	if since != nil && until != nil && since.After(*until) {
		return nil, nil, fmt.Errorf("--since must be on or before --until")
	}
	return since, until, nil
}

func normalizedVisualPortfolioFilters(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func visualPortfolioMilestoneMatches(value string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	for _, filter := range filters {
		if value == filter {
			return true
		}
	}
	return false
}

func visualPortfolioDateInWindow(raw string, since, until *time.Time, includeUnknown bool) bool {
	value := strings.TrimSpace(raw)
	if value == "" {
		return includeUnknown
	}
	if len(value) >= 10 {
		value = value[:10]
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return includeUnknown
	}
	return (since == nil || !parsed.Before(*since)) && (until == nil || !parsed.After(*until))
}

func visualPortfolioScheduleState(due, state string, now time.Time) string {
	if strings.TrimSpace(due) == "" {
		return "unknown_no_date"
	}
	if len(due) < 10 {
		return "unknown_invalid_date"
	}
	parsed, err := time.Parse("2006-01-02", due[:10])
	if err != nil {
		return "unknown_invalid_date"
	}
	if strings.EqualFold(state, "closed") {
		return "closed"
	}
	if parsed.Before(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)) {
		return "overdue"
	}
	return "scheduled"
}

func visualPortfolioBlocked(labels []string) bool {
	for _, label := range labels {
		value := strings.ToLower(strings.TrimSpace(label))
		if value == "status:blocked" || value == "blocked" || strings.Contains(value, "blocker") {
			return true
		}
	}
	return false
}

func visualPortfolioStatus(labels []string, fallback string) string {
	for _, label := range labels {
		if strings.HasPrefix(strings.ToLower(label), "status:") {
			return strings.TrimPrefix(strings.ToLower(label), "status:")
		}
	}
	return fallback
}

func visualPortfolioSource(repo, contract, snapshotAt string, err error) VisualPortfolioSource {
	source := VisualPortfolioSource{Repo: repo, Contract: contract, Status: "available", SnapshotAt: snapshotAt}
	if err != nil {
		source.Status = "unavailable"
		source.Reason = err.Error()
	}
	return source
}

var visualPortfolioHTMLTemplate = template.Must(template.New("visual-portfolio").Funcs(template.FuncMap{
	"join": strings.Join,
	"queueItems": func(items []VisualPortfolioQueueItem, queue string) []VisualPortfolioQueueItem {
		filtered := []VisualPortfolioQueueItem{}
		for _, item := range items {
			if item.Queue == queue {
				filtered = append(filtered, item)
			}
		}
		return filtered
	},
}).Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Gira portfolio overview</title>
  <style>
    :root { color-scheme: light; --ink:#172033; --muted:#61708a; --line:#dce3ed; --paper:#f5f7fb; --card:#fff; --navy:#173b6c; --cyan:#1d8aa5; --amber:#b65f00; --red:#b42318; --green:#18794e; }
    * { box-sizing:border-box; }
    body { margin:0; color:var(--ink); background:var(--paper); font:15px/1.5 ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif; }
    a { color:var(--navy); text-underline-offset:3px; }
    a:focus-visible { outline:3px solid #f4b400; outline-offset:3px; border-radius:3px; }
    .skip { position:absolute; left:-999px; top:0; padding:.7rem 1rem; background:#fff; z-index:10; }
    .skip:focus { left:1rem; top:1rem; }
    header { color:#fff; background:linear-gradient(120deg,#112d50,#174d6a); }
    .wrap { width:min(1180px,calc(100% - 2rem)); margin:auto; }
    header .wrap { padding:3rem 0 2.25rem; }
    .eyebrow { margin:0 0 .5rem; color:#9fdbe7; font-size:.78rem; font-weight:750; letter-spacing:.12em; text-transform:uppercase; }
    h1 { max-width:760px; margin:0; font-size:clamp(2rem,5vw,3.8rem); line-height:1.03; letter-spacing:-.035em; }
    .lede { max-width:780px; margin:1rem 0 0; color:#d6e7ef; }
    .meta { display:flex; flex-wrap:wrap; gap:.55rem 1.1rem; margin-top:1.3rem; color:#bdd3df; font-size:.86rem; }
    main { padding:1.6rem 0 4rem; }
    section { margin-top:1.2rem; padding:1.25rem; border:1px solid var(--line); border-radius:14px; background:var(--card); box-shadow:0 8px 24px rgba(25,44,75,.05); }
    h2 { margin:0; font-size:1.25rem; letter-spacing:-.01em; }
    .section-head { display:flex; align-items:baseline; justify-content:space-between; gap:1rem; margin-bottom:1rem; }
    .hint { margin:0; color:var(--muted); font-size:.86rem; }
    .stats { display:grid; grid-template-columns:repeat(6,minmax(0,1fr)); gap:.75rem; }
    .stat { padding:.85rem; border-left:3px solid var(--cyan); background:#f6fafc; }
    .stat span { display:block; color:var(--muted); font-size:.74rem; text-transform:uppercase; letter-spacing:.06em; }
    .stat strong { display:block; margin-top:.25rem; font-size:1.65rem; line-height:1; }
    .milestones { display:grid; grid-template-columns:repeat(auto-fit,minmax(250px,1fr)); gap:.85rem; }
    .milestone { padding:1rem; border:1px solid var(--line); border-radius:10px; }
    .kicker { color:var(--muted); font-size:.78rem; }
    .milestone h3 { margin:.3rem 0 .9rem; font-size:1rem; }
    .progress-track { height:10px; overflow:hidden; border-radius:999px; background:#e8edf4; }
    .progress-bar { height:100%; background:linear-gradient(90deg,var(--navy),var(--cyan)); }
    .progress-copy { display:flex; justify-content:space-between; gap:.5rem; margin-top:.5rem; font-size:.8rem; color:var(--muted); }
    .badge { display:inline-flex; margin-top:.75rem; padding:.2rem .48rem; border-radius:999px; background:#edf3f7; color:#31465f; font-size:.72rem; font-weight:700; }
    .badge.overdue { background:#fff0ed; color:var(--red); }
    .timeline { position:relative; margin:0; padding:0 0 0 1.25rem; list-style:none; border-left:2px solid #cbd8e5; }
    .timeline li { position:relative; padding:0 0 1rem 1rem; }
    .timeline li::before { content:""; position:absolute; left:-1.58rem; top:.35rem; width:.65rem; height:.65rem; border-radius:50%; background:var(--cyan); border:2px solid #fff; box-shadow:0 0 0 1px var(--cyan); }
    .timeline time { color:var(--muted); font-variant-numeric:tabular-nums; font-size:.78rem; }
    .timeline strong { display:block; }
    .queues { display:grid; grid-template-columns:1fr 1fr; gap:1rem; }
    .queue h3 { margin:0 0 .7rem; font-size:1rem; }
    .queue ul { margin:0; padding:0; list-style:none; }
    .queue li { padding:.75rem 0; border-top:1px solid var(--line); }
    .queue li:first-child { border-top:0; }
    .queue small { display:block; color:var(--muted); }
    table { width:100%; border-collapse:collapse; }
    th,td { padding:.65rem; border-top:1px solid var(--line); text-align:left; vertical-align:top; }
    th { color:var(--muted); font-size:.75rem; text-transform:uppercase; letter-spacing:.05em; }
    .unknown { padding:.9rem; border:1px dashed #a8b5c6; border-radius:8px; color:var(--muted); background:#fafbfd; }
    footer { margin-top:1.2rem; color:var(--muted); font-size:.8rem; }
    @media (max-width:820px) { .stats { grid-template-columns:repeat(3,1fr); } .queues { grid-template-columns:1fr; } }
    @media (max-width:520px) { .wrap { width:min(100% - 1.15rem,1180px); } header .wrap { padding:2.2rem 0 1.7rem; } section { padding:1rem; border-radius:10px; } .stats { grid-template-columns:repeat(2,1fr); } .section-head { display:block; } .hint { margin-top:.35rem; } table { table-layout:fixed; } th,td { overflow-wrap:anywhere; padding:.5rem .35rem; } th:nth-child(3),td:nth-child(3) { display:none; } }
    @media (prefers-reduced-motion:reduce) { *,*::before,*::after { scroll-behavior:auto!important; } }
  </style>
</head>
<body>
<a class="skip" href="#content">Skip to report content</a>
<header><div class="wrap">
  <p class="eyebrow">Gira / Read-only portfolio</p>
  <h1>Delivery, dates, and queues in one local view.</h1>
  <p class="lede">A point-in-time rendering of canonical GitHub and Gira contracts. Unknown values stay unknown; this file is not published, served, refreshed, or opened automatically.</p>
  <div class="meta"><span>Snapshot {{.GeneratedAt}}</span><span>Repositories: {{join .Filters.Repositories ", "}}</span><span>Milestones: {{if .Filters.Milestones}}{{join .Filters.Milestones ", "}}{{else}}all{{end}}</span><span>Window: {{if .Filters.Since}}{{.Filters.Since}}{{else}}unbounded{{end}} → {{if .Filters.Until}}{{.Filters.Until}}{{else}}unbounded{{end}}</span></div>
</div></header>
<main id="content" class="wrap">
  <section aria-labelledby="summary-title"><div class="section-head"><h2 id="summary-title">Portfolio pulse</h2><p class="hint">Counts are snapshot totals, not forecasts.</p></div>
    <div class="stats">
      <div class="stat"><span>Repositories</span><strong>{{.Summary.AvailableRepos}}/{{.Summary.Repositories}}</strong></div>
      <div class="stat"><span>Milestones</span><strong>{{.Summary.Milestones}}</strong></div>
      <div class="stat"><span>Milestone open</span><strong>{{.Summary.OpenIssues}}</strong></div>
      <div class="stat"><span>Milestone closed</span><strong>{{.Summary.ClosedIssues}}</strong></div>
      <div class="stat"><span>Blocked</span><strong>{{.Summary.BlockedItems}}</strong></div>
      <div class="stat"><span>Review wait</span><strong>{{.Summary.ReviewWaitingItems}}</strong></div>
    </div>
  </section>
  <section aria-labelledby="milestones-title"><div class="section-head"><h2 id="milestones-title">Milestone delivery</h2><p class="hint">closed issues ÷ total milestone issues</p></div>
    <div class="milestones">{{range .Milestones}}<article class="milestone"><div class="kicker">{{.Repo}} · #{{.Number}}</div><h3><a href="{{.URL}}">{{.Title}}</a></h3><div class="progress-track" role="progressbar" aria-label="{{.Title}} completion" aria-valuemin="0" aria-valuemax="100" aria-valuenow="{{.CompletionPercent}}"><div class="progress-bar" style="width:{{.CompletionPercent}}%"></div></div><div class="progress-copy"><span>{{.ClosedIssues}} closed / {{.TotalIssues}} total</span><strong>{{.CompletionPercent}}%</strong></div><span class="badge {{if eq .ScheduleState "overdue"}}overdue{{end}}">{{.ScheduleState}}{{if .DueOn}} · {{.DueOn}}{{end}}</span><div class="kicker">Source: {{.SourceContract}} · {{.Trace}}</div></article>{{else}}<p class="unknown">No milestones matched the selected repository and milestone filters.</p>{{end}}</div>
  </section>
  <section aria-labelledby="timeline-title"><div class="section-head"><h2 id="timeline-title">Timeline and named gates</h2><p class="hint">Only source-provided milestone, Start date, and Target date values are shown.</p></div>
    {{if .Timeline}}<ol class="timeline">{{range .Timeline}}<li><time datetime="{{.Date}}">{{.Date}}</time><strong>{{if .URL}}<a href="{{.URL}}">{{.Title}}</a>{{else}}{{.Title}}{{end}}</strong><span class="kicker">{{.Repo}} · {{.Kind}} · {{.Trace}}</span></li>{{end}}</ol>{{else}}<p class="unknown">No dated milestones or project gates exist inside this time window. Dates were not inferred.</p>{{end}}
  </section>
  <section aria-labelledby="queues-title"><div class="section-head"><h2 id="queues-title">Work queues</h2><p class="hint">Canonical links return to GitHub.</p></div><div class="queues">
    <div class="queue"><h3>Blocked</h3><ul>{{range queueItems .Queues "blocked"}}<li><a href="{{.URL}}">{{.Repo}}#{{.Number}} · {{.Title}}</a><small>{{.Status}} · milestone {{.Milestone}} · updated {{if .UpdatedAt}}{{.UpdatedAt}}{{else}}unknown{{end}}</small></li>{{else}}<li class="unknown">No blocked items in scope.</li>{{end}}</ul></div>
    <div class="queue"><h3>Review waiting</h3><ul>{{range queueItems .Queues "review_waiting"}}<li><a href="{{.URL}}">{{.Repo}}#{{.Number}} · {{.Title}}</a><small>{{.Status}} · milestone unsupported · updated {{if .UpdatedAt}}{{.UpdatedAt}}{{else}}unknown{{end}}</small></li>{{else}}<li class="unknown">No review-waiting items in scope.</li>{{end}}</ul></div>
  </div></section>
  <section aria-labelledby="sources-title"><div class="section-head"><h2 id="sources-title">Sources and support boundaries</h2><p class="hint">Partial access stays visible.</p></div>
    <table><thead><tr><th>Repository</th><th>Contract</th><th>Status</th><th>Snapshot / reason</th></tr></thead><tbody>{{range .Sources}}<tr><td>{{.Repo}}</td><td><code>{{.Contract}}</code></td><td>{{.Status}}</td><td>{{.SnapshotAt}}{{if .Reason}} · {{.Reason}}{{end}}</td></tr>{{end}}</tbody></table>
    {{range .Unsupported}}<p class="unknown">{{.}}</p>{{end}}
  </section>
  <footer>Generated by <code>gira report portfolio</code>. Static local artifact; no background refresh or publication.</footer>
</main>
</body>
</html>`))

func RenderVisualPortfolioHTML(report VisualPortfolioReport) (string, error) {
	var output strings.Builder
	if err := visualPortfolioHTMLTemplate.Execute(&output, report); err != nil {
		return "", fmt.Errorf("render portfolio HTML: %w", err)
	}
	output.WriteByte('\n')
	return output.String(), nil
}

func WriteVisualPortfolioHTML(path string, report VisualPortfolioReport) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("--output is required")
	}
	html, err := RenderVisualPortfolioHTML(report)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create portfolio output directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(html), 0o644); err != nil {
		return fmt.Errorf("write portfolio HTML: %w", err)
	}
	return nil
}
