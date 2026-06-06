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

type StatsPulseOptions struct {
	Repo  RepoRef   `json:"-"`
	Since string    `json:"since"`
	Now   time.Time `json:"-"`
	Limit int       `json:"limit"`
}

type PulseReport struct {
	SchemaVersion   string               `json:"schema_version"`
	Command         string               `json:"command"`
	Scope           PulseScope           `json:"scope"`
	Window          PulseWindow          `json:"window"`
	Source          StatsSource          `json:"source"`
	Summary         PulseSummary         `json:"summary"`
	Health          PulseHealth          `json:"health"`
	Items           []PulseItem          `json:"items"`
	Warnings        []PulseWarning       `json:"warnings"`
	PrivacyBoundary PulsePrivacyBoundary `json:"privacy_boundary"`
}

type PulseScope struct {
	Kind string `json:"kind"`
	Repo string `json:"repo"`
}

type PulseWindow struct {
	Since   string `json:"since"`
	SinceAt string `json:"since_at"`
	Until   string `json:"until"`
	Label   string `json:"label"`
	Limit   int    `json:"limit"`
}

type PulseSummary struct {
	Finished   int `json:"finished"`
	Reviewed   int `json:"reviewed"`
	Refined    int `json:"refined"`
	Unblocked  int `json:"unblocked"`
	Superseded int `json:"superseded"`
	Started    int `json:"started"`
	Checked    int `json:"checked"`
}

type PulseHealth struct {
	Ready         int `json:"ready"`
	ReviewNeeded  int `json:"review_needed"`
	FinishReady   int `json:"finish_ready"`
	Blocked       int `json:"blocked"`
	FailedCheck   int `json:"failed_check"`
	HumanDecision int `json:"human_decision"`
}

type PulseItem struct {
	Kind       string   `json:"kind"`
	Repo       string   `json:"repo"`
	Issue      int      `json:"issue,omitempty"`
	PR         int      `json:"pr,omitempty"`
	Title      string   `json:"title"`
	Confidence string   `json:"confidence"`
	OccurredAt string   `json:"occurred_at,omitempty"`
	Evidence   []string `json:"evidence"`
	SourceRefs []string `json:"source_refs"`
}

type PulseWarning struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type PulsePrivacyBoundary struct {
	Scope      string   `json:"scope"`
	Prohibited []string `json:"prohibited"`
}

type statsIssueRow struct {
	Number    int             `json:"number"`
	Title     string          `json:"title"`
	Body      string          `json:"body"`
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
	ReviewDecision    string                `json:"reviewDecision"`
	IsDraft           bool                  `json:"isDraft"`
	CreatedAt         string                `json:"createdAt"`
	MergedAt          string                `json:"mergedAt"`
	UpdatedAt         string                `json:"updatedAt"`
	URL               string                `json:"url"`
	Labels            []statsLabelRow       `json:"labels"`
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
	args := []string{"issue", "list", "--repo", repo.FullName(), "--state", state, "--limit", strconv.Itoa(limit)}
	if strings.TrimSpace(search) != "" {
		args = append(args, "--search", search)
	}
	args = append(args, "--json", "number,title,state,createdAt,closedAt,updatedAt,labels,url")
	out, err := runner.Run("gh", args...)
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
	args := []string{"pr", "list", "--repo", repo.FullName(), "--state", state, "--limit", strconv.Itoa(limit)}
	if strings.TrimSpace(search) != "" {
		args = append(args, "--search", search)
	}
	args = append(args, "--json", "number,title,body,state,createdAt,mergedAt,updatedAt,url,statusCheckRollup")
	out, err := runner.Run("gh", args...)
	if err != nil {
		return nil, err
	}
	var rows []statsPRRow
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("parse PR JSON: %w", err)
	}
	return rows, nil
}

func fetchPulseIssues(repo RepoRef, state string, search string, limit int, runner CommandRunner) ([]statsIssueRow, error) {
	args := []string{"issue", "list", "--repo", repo.FullName(), "--state", state, "--limit", strconv.Itoa(limit)}
	if strings.TrimSpace(search) != "" {
		args = append(args, "--search", search)
	}
	args = append(args, "--json", "number,title,body,state,createdAt,closedAt,updatedAt,labels,url")
	out, err := runner.Run("gh", args...)
	if err != nil {
		return nil, err
	}
	var rows []statsIssueRow
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("parse pulse issues JSON: %w", err)
	}
	return rows, nil
}

func fetchPulsePRs(repo RepoRef, state string, search string, limit int, runner CommandRunner) ([]statsPRRow, error) {
	args := []string{"pr", "list", "--repo", repo.FullName(), "--state", state, "--limit", strconv.Itoa(limit)}
	if strings.TrimSpace(search) != "" {
		args = append(args, "--search", search)
	}
	args = append(args, "--json", "number,title,body,state,reviewDecision,isDraft,createdAt,mergedAt,updatedAt,url,labels,statusCheckRollup")
	out, err := runner.Run("gh", args...)
	if err != nil {
		return nil, err
	}
	var rows []statsPRRow
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("parse pulse PR JSON: %w", err)
	}
	return rows, nil
}

func BuildStatsPulseReport(options StatsPulseOptions, runner CommandRunner) (PulseReport, error) {
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
		return PulseReport{}, fmt.Errorf("--limit must be greater than 0")
	}
	sinceText := strings.TrimSpace(options.Since)
	if sinceText == "" {
		sinceText = "7d"
	}
	sinceAt, err := parseStatsSince(sinceText, now)
	if err != nil {
		return PulseReport{}, err
	}
	sinceDate := sinceAt.Format("2006-01-02")

	updatedIssues, err := fetchPulseIssues(options.Repo, "all", "updated:>="+sinceDate, limit, runner)
	if err != nil {
		return PulseReport{}, fmt.Errorf("fetch updated issues: %w", err)
	}
	closedIssues, err := fetchPulseIssues(options.Repo, "closed", "closed:>="+sinceDate, limit, runner)
	if err != nil {
		return PulseReport{}, fmt.Errorf("fetch closed issues: %w", err)
	}
	openIssues, err := fetchPulseIssues(options.Repo, "open", "", limit, runner)
	if err != nil {
		return PulseReport{}, fmt.Errorf("fetch open issues: %w", err)
	}
	updatedPRs, err := fetchPulsePRs(options.Repo, "all", "updated:>="+sinceDate, limit, runner)
	if err != nil {
		return PulseReport{}, fmt.Errorf("fetch updated PRs: %w", err)
	}
	openPRs, err := fetchPulsePRs(options.Repo, "open", "", limit, runner)
	if err != nil {
		return PulseReport{}, fmt.Errorf("fetch open PRs: %w", err)
	}

	report := PulseReport{
		SchemaVersion: "pulse-report/v1alpha1",
		Command:       "stats pulse",
		Scope:         PulseScope{Kind: "repo", Repo: options.Repo.FullName()},
		Window: PulseWindow{
			Since:   sinceText,
			SinceAt: sinceAt.UTC().Format(time.RFC3339),
			Until:   now.UTC().Format(time.RFC3339),
			Label:   sinceText,
			Limit:   limit,
		},
		Source: StatsSource{
			Backend:  "github",
			ReadOnly: true,
			Notes:    "Uses GitHub issue, PR, check, review, label, and closing-link metadata; no source code or diffs are read.",
		},
		Health:          computePulseHealth(openIssues, openPRs),
		PrivacyBoundary: pulsePrivacyBoundary(),
		Warnings: []PulseWarning{{
			Code:     "transition_history_partial",
			Severity: "info",
			Message:  "Refined, started, and unblocked signals use current labels plus recent updates because GitHub list metadata does not expose full label transition history.",
		}},
	}
	report.Items = buildPulseItems(options.Repo, sinceAt, updatedIssues, closedIssues, updatedPRs)
	report.Summary = summarizePulseItems(report.Items)
	if len(updatedIssues) == limit || len(closedIssues) == limit || len(openIssues) == limit || len(updatedPRs) == limit || len(openPRs) == limit {
		report.Warnings = append(report.Warnings, PulseWarning{Code: "row_limit_reached", Severity: "warning", Message: "One or more GitHub list reads reached --limit; pulse counts may be incomplete."})
	}
	sortPulseItems(report.Items)
	return report, nil
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

func buildPulseItems(repo RepoRef, sinceAt time.Time, updatedIssues []statsIssueRow, closedIssues []statsIssueRow, updatedPRs []statsPRRow) []PulseItem {
	items := []PulseItem{}
	closedByNumber := map[int]statsIssueRow{}
	for _, issue := range closedIssues {
		closedByNumber[issue.Number] = issue
		if statsIssueHasLabel(issue, "resolution:superseded") {
			items = append(items, pulseItemFromIssue(repo, "superseded", issue, "high", []string{"closed_issue", "resolution_superseded"}))
		}
	}
	for _, pr := range updatedPRs {
		if isStatsPRMerged(pr) && statsPRHasClosingLink(pr) {
			issueNumber := firstStatsClosingIssue(pr)
			evidence := []string{"merged_pr", "closing_reference"}
			if closedIssue, ok := closedByNumber[issueNumber]; ok && strings.EqualFold(closedIssue.State, "CLOSED") {
				evidence = append(evidence, "closed_issue")
			}
			items = append(items, pulseItemFromPR(repo, "finished", pr, issueNumber, "high", evidence))
			continue
		}
		if pulsePRReviewed(pr) {
			items = append(items, pulseItemFromPR(repo, "reviewed", pr, 0, "medium", pulsePREvidence(pr)))
		}
	}
	for _, issue := range updatedIssues {
		if strings.EqualFold(issue.State, "CLOSED") {
			continue
		}
		if !statsIssueCreatedBefore(issue, sinceAt) {
			continue
		}
		status := statsIssueStatus(issue)
		if pulseIssueUnblocked(issue) {
			items = append(items, pulseItemFromIssue(repo, "unblocked", issue, "medium", []string{"recent_update", "unblock_evidence"}))
		}
		if status == "status:ready" && pulseIssueLooksRefined(issue) {
			items = append(items, pulseItemFromIssue(repo, "refined", issue, "medium", []string{"status_ready", "structured_body", "recent_update"}))
		}
		if status == "status:in-progress" {
			items = append(items, pulseItemFromIssue(repo, "started", issue, "medium", []string{"status_in_progress", "recent_update"}))
		}
	}
	return items
}

func summarizePulseItems(items []PulseItem) PulseSummary {
	var summary PulseSummary
	for _, item := range items {
		switch item.Kind {
		case "finished":
			summary.Finished++
		case "reviewed":
			summary.Reviewed++
		case "refined":
			summary.Refined++
		case "unblocked":
			summary.Unblocked++
		case "superseded":
			summary.Superseded++
		case "started":
			summary.Started++
		case "checked":
			summary.Checked++
		}
	}
	return summary
}

func computePulseHealth(openIssues []statsIssueRow, openPRs []statsPRRow) PulseHealth {
	var health PulseHealth
	for _, issue := range openIssues {
		status := statsIssueStatus(issue)
		switch status {
		case "status:ready":
			health.Ready++
		case "status:blocked":
			health.Blocked++
		}
		if pulseIssueNeedsHuman(issue) {
			health.HumanDecision++
		}
	}
	for _, pr := range openPRs {
		pending, failing := statsPRCheckState(pr)
		if failing {
			health.FailedCheck++
		}
		if pulsePRFinishReady(pr) {
			health.FinishReady++
			continue
		}
		if !pr.IsDraft && (pulsePRNeedsReview(pr) || pending || failing) {
			health.ReviewNeeded++
		}
	}
	return health
}

func pulseItemFromIssue(repo RepoRef, kind string, issue statsIssueRow, confidence string, evidence []string) PulseItem {
	return PulseItem{
		Kind:       kind,
		Repo:       repo.FullName(),
		Issue:      issue.Number,
		Title:      issue.Title,
		Confidence: confidence,
		OccurredAt: pulseIssueOccurredAt(issue),
		Evidence:   append([]string(nil), evidence...),
		SourceRefs: []string{fmt.Sprintf("issue:%s#%d", repo.FullName(), issue.Number)},
	}
}

func pulseItemFromPR(repo RepoRef, kind string, pr statsPRRow, issueNumber int, confidence string, evidence []string) PulseItem {
	item := PulseItem{
		Kind:       kind,
		Repo:       repo.FullName(),
		Issue:      issueNumber,
		PR:         pr.Number,
		Title:      pr.Title,
		Confidence: confidence,
		OccurredAt: pulsePROccurredAt(pr),
		Evidence:   append([]string(nil), evidence...),
		SourceRefs: []string{fmt.Sprintf("pr:%s#%d", repo.FullName(), pr.Number)},
	}
	if issueNumber > 0 {
		item.SourceRefs = append([]string{fmt.Sprintf("issue:%s#%d", repo.FullName(), issueNumber)}, item.SourceRefs...)
	}
	return item
}

func pulseIssueOccurredAt(issue statsIssueRow) string {
	if strings.TrimSpace(issue.ClosedAt) != "" {
		return issue.ClosedAt
	}
	return issue.UpdatedAt
}

func pulsePROccurredAt(pr statsPRRow) string {
	if strings.TrimSpace(pr.MergedAt) != "" {
		return pr.MergedAt
	}
	return pr.UpdatedAt
}

func firstStatsClosingIssue(pr statsPRRow) int {
	issues := ExtractClosureIssueNumbers(pr.Title + "\n" + pr.Body)
	if len(issues) == 0 {
		return 0
	}
	return issues[0]
}

func statsIssueStatus(issue statsIssueRow) string {
	for _, label := range issue.Labels {
		name := strings.ToLower(strings.TrimSpace(label.Name))
		if strings.HasPrefix(name, "status:") {
			return name
		}
	}
	return ""
}

func statsIssueCreatedBefore(issue statsIssueRow, sinceAt time.Time) bool {
	created, err := time.Parse(time.RFC3339, strings.TrimSpace(issue.CreatedAt))
	if err != nil {
		return false
	}
	return created.Before(sinceAt)
}

func pulseIssueLooksRefined(issue statsIssueRow) bool {
	body := strings.ToLower(issue.Body)
	return strings.Contains(body, "## goal") && strings.Contains(body, "## scope") && strings.Contains(body, "## acceptance")
}

func pulseIssueUnblocked(issue statsIssueRow) bool {
	if statsIssueHasLabel(issue, "resolution:unblocked") || statsIssueHasLabel(issue, "status:unblocked") {
		return true
	}
	body := strings.ToLower(issue.Body)
	return strings.Contains(body, "unblocked") || strings.Contains(body, "blocker resolved")
}

func pulseIssueNeedsHuman(issue statsIssueRow) bool {
	for _, label := range issue.Labels {
		name := strings.ToLower(strings.TrimSpace(label.Name))
		switch name {
		case "agent:human", "needs:human", "needs:decision", "type:decision":
			return true
		}
	}
	return false
}

func pulsePRReviewed(pr statsPRRow) bool {
	if pr.IsDraft || isStatsPRMerged(pr) {
		return false
	}
	decision := strings.ToUpper(strings.TrimSpace(pr.ReviewDecision))
	if decision == "APPROVED" {
		return true
	}
	pending, failing := statsPRCheckState(pr)
	return !pending && !failing && len(pr.StatusCheckRollup) > 0
}

func pulsePRNeedsReview(pr statsPRRow) bool {
	decision := strings.ToUpper(strings.TrimSpace(pr.ReviewDecision))
	return decision == "" || decision == "REVIEW_REQUIRED" || decision == "CHANGES_REQUESTED"
}

func pulsePRFinishReady(pr statsPRRow) bool {
	if pr.IsDraft {
		return false
	}
	pending, failing := statsPRCheckState(pr)
	if pending || failing || len(pr.StatusCheckRollup) == 0 {
		return false
	}
	decision := strings.ToUpper(strings.TrimSpace(pr.ReviewDecision))
	return decision == "APPROVED"
}

func pulsePREvidence(pr statsPRRow) []string {
	evidence := []string{}
	if strings.EqualFold(strings.TrimSpace(pr.ReviewDecision), "APPROVED") {
		evidence = append(evidence, "review_approved")
	}
	pending, failing := statsPRCheckState(pr)
	if !pending && !failing && len(pr.StatusCheckRollup) > 0 {
		evidence = append(evidence, "checks_passed")
	}
	if len(evidence) == 0 {
		evidence = append(evidence, "recent_pr_movement")
	}
	return evidence
}

func pulsePrivacyBoundary() PulsePrivacyBoundary {
	return PulsePrivacyBoundary{
		Scope: "work_item_state_only",
		Prohibited: []string{
			"personal_productivity_ranking",
			"agent_productivity_ranking",
			"time_online_scoring",
			"token_spend_scoring",
			"leaderboard",
		},
	}
}

func sortPulseItems(items []PulseItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].OccurredAt != items[j].OccurredAt {
			return items[i].OccurredAt > items[j].OccurredAt
		}
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		if items[i].Issue != items[j].Issue {
			return items[i].Issue < items[j].Issue
		}
		return items[i].PR < items[j].PR
	})
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

func FormatPulseReport(report PulseReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Gira Pulse - %s\n", report.Scope.Repo)
	fmt.Fprintf(&b, "window: since=%s until=%s source=github-readonly\n\n", report.Window.Since, report.Window.Until)
	b.WriteString("Movement\n")
	fmt.Fprintf(&b, "- finished: %d\n", report.Summary.Finished)
	fmt.Fprintf(&b, "- reviewed: %d\n", report.Summary.Reviewed)
	fmt.Fprintf(&b, "- refined: %d\n", report.Summary.Refined)
	fmt.Fprintf(&b, "- unblocked: %d\n", report.Summary.Unblocked)
	fmt.Fprintf(&b, "- superseded: %d\n", report.Summary.Superseded)
	fmt.Fprintf(&b, "- started: %d\n", report.Summary.Started)
	fmt.Fprintf(&b, "- checked: %d\n\n", report.Summary.Checked)
	b.WriteString("Attention\n")
	fmt.Fprintf(&b, "- ready: %d\n", report.Health.Ready)
	fmt.Fprintf(&b, "- review needed: %d\n", report.Health.ReviewNeeded)
	fmt.Fprintf(&b, "- finish ready: %d\n", report.Health.FinishReady)
	fmt.Fprintf(&b, "- blocked: %d\n", report.Health.Blocked)
	fmt.Fprintf(&b, "- failed checks: %d\n", report.Health.FailedCheck)
	fmt.Fprintf(&b, "- human decision: %d\n\n", report.Health.HumanDecision)
	if len(report.Items) > 0 {
		b.WriteString("Recent evidence\n")
		for _, item := range report.Items {
			ref := ""
			switch {
			case item.Issue > 0 && item.PR > 0:
				ref = fmt.Sprintf("#%d PR #%d", item.Issue, item.PR)
			case item.Issue > 0:
				ref = fmt.Sprintf("#%d", item.Issue)
			case item.PR > 0:
				ref = fmt.Sprintf("PR #%d", item.PR)
			}
			if ref != "" {
				ref += " "
			}
			fmt.Fprintf(&b, "- %s: %s%s (%s)\n", item.Kind, ref, item.Title, strings.Join(item.Evidence, ","))
		}
		b.WriteString("\n")
	}
	if len(report.Warnings) > 0 {
		b.WriteString("Warnings\n")
		for _, warning := range report.Warnings {
			fmt.Fprintf(&b, "- %s: %s\n", warning.Code, warning.Message)
		}
		b.WriteString("\n")
	}
	b.WriteString("Boundary\n")
	b.WriteString("- Work-item evidence only; no people or agent ranking.\n")
	return b.String()
}
