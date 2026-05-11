package gira

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type StatsRepoOptions struct {
	Repo      RepoRef   `json:"-"`
	Since     string    `json:"since"`
	Now       time.Time `json:"-"`
	Limit     int       `json:"limit"`
	StaleDays int       `json:"stale_days"`
}

type StatsRepoReport struct {
	Command     string           `json:"command"`
	Repo        string           `json:"repo"`
	GeneratedAt string           `json:"generated_at"`
	Window      StatsWindow      `json:"window"`
	Source      StatsSource      `json:"source"`
	Metrics     StatsRepoMetrics `json:"metrics"`
	Confidence  StatsConfidence  `json:"confidence"`
	NonGoals    []string         `json:"non_goals"`
	NextSteps   []string         `json:"next_steps"`
}

type StatsWindow struct {
	Since     string `json:"since"`
	SinceAt   string `json:"since_at"`
	StaleDays int    `json:"stale_days"`
	Limit     int    `json:"limit"`
}

type StatsSource struct {
	Backend  string `json:"backend"`
	ReadOnly bool   `json:"read_only"`
	Notes    string `json:"notes"`
}

type StatsRepoMetrics struct {
	OpenedIssues              int     `json:"opened_issues"`
	ClosedIssues              int     `json:"closed_issues"`
	SupersededIssues          int     `json:"superseded_issues"`
	CompletedIssues           int     `json:"completed_issues"`
	OpenedPRs                 int     `json:"opened_prs"`
	MergedPRs                 int     `json:"merged_prs"`
	PRsWithClosingLinks       int     `json:"prs_with_closing_links"`
	MergedPRsWithLinkedIssues int     `json:"merged_prs_with_linked_issues"`
	ChecksPendingPRs          int     `json:"checks_pending_prs"`
	ChecksFailingPRs          int     `json:"checks_failing_prs"`
	StaleOpenIssues           int     `json:"stale_open_issues"`
	StaleOpenPRs              int     `json:"stale_open_prs"`
	ClosureRate               float64 `json:"closure_rate"`
}

type StatsConfidence struct {
	Level   string   `json:"level"`
	Signals []string `json:"signals"`
}

type statsIssueRow struct {
	Number    int             `json:"number"`
	Title     string          `json:"title"`
	State     string          `json:"state"`
	CreatedAt string          `json:"createdAt"`
	ClosedAt  string          `json:"closedAt"`
	UpdatedAt string          `json:"updatedAt"`
	Labels    []statsLabelRow `json:"labels"`
	URL       string          `json:"url"`
}

type statsPRRow struct {
	Number            int                   `json:"number"`
	Title             string                `json:"title"`
	Body              string                `json:"body"`
	State             string                `json:"state"`
	CreatedAt         string                `json:"createdAt"`
	MergedAt          string                `json:"mergedAt"`
	UpdatedAt         string                `json:"updatedAt"`
	URL               string                `json:"url"`
	StatusCheckRollup []statsStatusCheckRow `json:"statusCheckRollup"`
}

type statsLabelRow struct {
	Name string `json:"name"`
}

type statsStatusCheckRow struct {
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

var closingLinkPattern = regexp.MustCompile(`(?i)\b(close[sd]?|fix(e[sd])?|resolve[sd]?)\s+#[0-9]+`)

func BuildStatsRepoReport(options StatsRepoOptions, runner CommandRunner) (StatsRepoReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	now := options.Now
	if now.IsZero() {
		now = time.Now()
	}
	limit := options.Limit
	if limit == 0 {
		limit = 100
	}
	if limit < 0 {
		return StatsRepoReport{}, fmt.Errorf("--limit must be greater than 0")
	}
	staleDays := options.StaleDays
	if staleDays == 0 {
		staleDays = 14
	}
	if staleDays < 0 {
		return StatsRepoReport{}, fmt.Errorf("--stale-days must be greater than 0")
	}
	sinceText := strings.TrimSpace(options.Since)
	if sinceText == "" {
		sinceText = "90d"
	}
	sinceAt, err := parseStatsSince(sinceText, now)
	if err != nil {
		return StatsRepoReport{}, err
	}
	sinceDate := sinceAt.Format("2006-01-02")
	staleDate := now.AddDate(0, 0, -staleDays).Format("2006-01-02")

	openedIssues, err := fetchStatsIssues(options.Repo, "all", "created:>="+sinceDate, limit, runner)
	if err != nil {
		return StatsRepoReport{}, fmt.Errorf("fetch opened issues: %w", err)
	}
	closedIssues, err := fetchStatsIssues(options.Repo, "closed", "closed:>="+sinceDate, limit, runner)
	if err != nil {
		return StatsRepoReport{}, fmt.Errorf("fetch closed issues: %w", err)
	}
	staleIssues, err := fetchStatsIssues(options.Repo, "open", "updated:<"+staleDate, limit, runner)
	if err != nil {
		return StatsRepoReport{}, fmt.Errorf("fetch stale issues: %w", err)
	}
	openedPRs, err := fetchStatsPRs(options.Repo, "all", "created:>="+sinceDate, limit, runner)
	if err != nil {
		return StatsRepoReport{}, fmt.Errorf("fetch opened PRs: %w", err)
	}
	stalePRs, err := fetchStatsPRs(options.Repo, "open", "updated:<"+staleDate, limit, runner)
	if err != nil {
		return StatsRepoReport{}, fmt.Errorf("fetch stale PRs: %w", err)
	}

	metrics := computeStatsRepoMetrics(openedIssues, closedIssues, staleIssues, openedPRs, stalePRs)
	report := StatsRepoReport{
		Command:     "stats repo",
		Repo:        options.Repo.FullName(),
		GeneratedAt: now.UTC().Format(time.RFC3339),
		Window: StatsWindow{
			Since:     sinceText,
			SinceAt:   sinceAt.UTC().Format(time.RFC3339),
			StaleDays: staleDays,
			Limit:     limit,
		},
		Source: StatsSource{
			Backend:  "github",
			ReadOnly: true,
			Notes:    "Uses GitHub issue, PR, and check metadata; no source code or diffs are read.",
		},
		Metrics:    metrics,
		Confidence: computeStatsConfidence(openedIssues, closedIssues),
		NonGoals: []string{
			"personal productivity score",
			"full DORA suite",
			"AI spend or token analytics",
			"dashboard UI",
			"precise agent attribution in the first slice",
		},
		NextSteps: []string{
			"Use missing closing-link counts to improve PR templates.",
			"Use stale open issue and PR counts to find workflow breaks.",
			"Use Gira labels and lifecycle commands for higher-confidence closure reports.",
		},
	}
	return report, nil
}

func fetchStatsIssues(repo RepoRef, state string, search string, limit int, runner CommandRunner) ([]statsIssueRow, error) {
	out, err := runner.Run("gh", "issue", "list", "--repo", repo.FullName(), "--state", state, "--limit", strconv.Itoa(limit), "--search", search, "--json", "number,title,state,createdAt,closedAt,updatedAt,labels,url")
	if err != nil {
		return nil, err
	}
	var rows []statsIssueRow
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("parse issues JSON: %w", err)
	}
	return rows, nil
}

func fetchStatsPRs(repo RepoRef, state string, search string, limit int, runner CommandRunner) ([]statsPRRow, error) {
	out, err := runner.Run("gh", "pr", "list", "--repo", repo.FullName(), "--state", state, "--limit", strconv.Itoa(limit), "--search", search, "--json", "number,title,body,state,createdAt,mergedAt,updatedAt,url,statusCheckRollup")
	if err != nil {
		return nil, err
	}
	var rows []statsPRRow
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("parse PR JSON: %w", err)
	}
	return rows, nil
}

func computeStatsRepoMetrics(openedIssues []statsIssueRow, closedIssues []statsIssueRow, staleIssues []statsIssueRow, openedPRs []statsPRRow, stalePRs []statsPRRow) StatsRepoMetrics {
	metrics := StatsRepoMetrics{
		OpenedIssues:    len(openedIssues),
		ClosedIssues:    len(closedIssues),
		OpenedPRs:       len(openedPRs),
		StaleOpenIssues: len(staleIssues),
		StaleOpenPRs:    len(stalePRs),
	}
	for _, issue := range closedIssues {
		if statsIssueHasLabel(issue, "resolution:superseded") {
			metrics.SupersededIssues++
		}
	}
	metrics.CompletedIssues = metrics.ClosedIssues - metrics.SupersededIssues
	for _, pr := range openedPRs {
		merged := isStatsPRMerged(pr)
		hasClosingLink := statsPRHasClosingLink(pr)
		if merged {
			metrics.MergedPRs++
		}
		if hasClosingLink {
			metrics.PRsWithClosingLinks++
		}
		if merged && hasClosingLink {
			metrics.MergedPRsWithLinkedIssues++
		}
		pending, failing := statsPRCheckState(pr)
		if pending {
			metrics.ChecksPendingPRs++
		}
		if failing {
			metrics.ChecksFailingPRs++
		}
	}
	if metrics.OpenedIssues > 0 {
		metrics.ClosureRate = math.Round((float64(metrics.MergedPRsWithLinkedIssues)/float64(metrics.OpenedIssues))*1000) / 1000
	}
	return metrics
}

func parseStatsSince(value string, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if strings.HasSuffix(value, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err != nil || days <= 0 {
			return time.Time{}, fmt.Errorf("--since must be a positive day window like 90d or YYYY-MM-DD")
		}
		return now.AddDate(0, 0, -days), nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("--since must be a positive day window like 90d or YYYY-MM-DD")
	}
	return parsed, nil
}

func statsPRHasClosingLink(pr statsPRRow) bool {
	return closingLinkPattern.MatchString(pr.Title) || closingLinkPattern.MatchString(pr.Body)
}

func isStatsPRMerged(pr statsPRRow) bool {
	return strings.TrimSpace(pr.MergedAt) != "" || strings.EqualFold(pr.State, "MERGED")
}

func statsPRCheckState(pr statsPRRow) (bool, bool) {
	pending := false
	failing := false
	for _, check := range pr.StatusCheckRollup {
		status := strings.ToUpper(strings.TrimSpace(check.Status))
		conclusion := strings.ToUpper(strings.TrimSpace(check.Conclusion))
		if status != "" && status != "COMPLETED" {
			pending = true
		}
		switch conclusion {
		case "FAILURE", "FAILED", "TIMED_OUT", "ACTION_REQUIRED", "CANCELLED":
			failing = true
		}
	}
	return pending, failing
}

func statsIssueHasLabel(issue statsIssueRow, want string) bool {
	for _, label := range issue.Labels {
		if strings.EqualFold(strings.TrimSpace(label.Name), want) {
			return true
		}
	}
	return false
}

func computeStatsConfidence(openedIssues []statsIssueRow, closedIssues []statsIssueRow) StatsConfidence {
	signals := map[string]struct{}{}
	for _, issue := range append(append([]statsIssueRow{}, openedIssues...), closedIssues...) {
		for _, label := range issue.Labels {
			name := strings.ToLower(strings.TrimSpace(label.Name))
			if strings.HasPrefix(name, "status:") {
				signals["status labels"] = struct{}{}
			}
			if strings.HasPrefix(name, "type:") {
				signals["type labels"] = struct{}{}
			}
			if strings.HasPrefix(name, "resolution:") {
				signals["resolution labels"] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(signals))
	for signal := range signals {
		out = append(out, signal)
	}
	sort.Strings(out)
	level := "low"
	if len(out) >= 2 {
		level = "medium"
	}
	if len(out) >= 3 {
		level = "high"
	}
	if len(out) == 0 {
		out = append(out, "GitHub metadata only")
	}
	return StatsConfidence{Level: level, Signals: out}
}

func FormatStatsRepoReport(report StatsRepoReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Gira Closure Funnel - %s\n", report.Repo)
	fmt.Fprintf(&b, "window: since=%s stale>%dd confidence=%s source=github-readonly\n\n", report.Window.Since, report.Window.StaleDays, report.Confidence.Level)
	b.WriteString("Workflow closure\n")
	fmt.Fprintf(&b, "- opened issues: %d\n", report.Metrics.OpenedIssues)
	fmt.Fprintf(&b, "- completed issues: %d\n", report.Metrics.CompletedIssues)
	fmt.Fprintf(&b, "- superseded issues: %d\n", report.Metrics.SupersededIssues)
	fmt.Fprintf(&b, "- opened PRs: %d\n", report.Metrics.OpenedPRs)
	fmt.Fprintf(&b, "- merged PRs: %d\n", report.Metrics.MergedPRs)
	fmt.Fprintf(&b, "- PRs with closing links: %d\n", report.Metrics.PRsWithClosingLinks)
	fmt.Fprintf(&b, "- merged PRs with linked issues: %d\n", report.Metrics.MergedPRsWithLinkedIssues)
	fmt.Fprintf(&b, "closure rate: %.1f%%\n\n", report.Metrics.ClosureRate*100)
	b.WriteString("Friction signals\n")
	fmt.Fprintf(&b, "- checks pending PRs: %d\n", report.Metrics.ChecksPendingPRs)
	fmt.Fprintf(&b, "- checks failing PRs: %d\n", report.Metrics.ChecksFailingPRs)
	fmt.Fprintf(&b, "- stale open issues: %d\n", report.Metrics.StaleOpenIssues)
	fmt.Fprintf(&b, "- stale open PRs: %d\n\n", report.Metrics.StaleOpenPRs)
	b.WriteString("Non-goals\n")
	for _, nonGoal := range report.NonGoals {
		fmt.Fprintf(&b, "- %s\n", nonGoal)
	}
	b.WriteString("\nNext steps\n")
	for i, step := range report.NextSteps {
		fmt.Fprintf(&b, "%d. %s\n", i+1, step)
	}
	return b.String()
}
