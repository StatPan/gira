package gira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

type CommandRunner interface {
	Run(name string, args ...string) ([]byte, error)
}

type ExecCommandRunner struct{}

func (ExecCommandRunner) Run(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return nil, fmt.Errorf("%s: %w", detail, err)
		}
		return nil, err
	}
	return output, nil
}

type StatusClient interface {
	Repo() RepoRef
	JSON(args []string, target any) error
}

type GHStatusClient struct {
	repo   RepoRef
	runner CommandRunner
}

func NewGHStatusClient(repo RepoRef, runner CommandRunner) GHStatusClient {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	return GHStatusClient{repo: repo, runner: runner}
}

func (c GHStatusClient) Repo() RepoRef {
	return c.repo
}

func (c GHStatusClient) JSON(args []string, target any) error {
	output, err := c.runner.Run("gh", args...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(output, target); err != nil {
		return fmt.Errorf("parse gh JSON: %w", err)
	}
	return nil
}

type StatusSummary struct {
	Counts     StatusCounts     `json:"counts"`
	FetchedAt  string           `json:"fetched_at"`
	Issues     StatusIssueLists `json:"issues"`
	Milestones []MilestoneStats `json:"milestones"`
	Repo       string           `json:"repo"`
	StaleDays  int              `json:"stale_days"`
}

type StatusCounts struct {
	Issues     IssueCounts     `json:"issues"`
	Milestones MilestoneCounts `json:"milestones"`
}

type IssueCounts struct {
	Total                        int `json:"total"`
	Open                         int `json:"open"`
	Closed                       int `json:"closed"`
	StaleOpen                    int `json:"stale_open"`
	BlockedOpen                  int `json:"blocked_open"`
	ClosureLinkMissingOpenIssues int `json:"closure_link_missing_open_issues"`
	PRsMissingClosureLink        int `json:"prs_missing_closure_link"`
}

type MilestoneCounts struct {
	Total  int `json:"total"`
	Open   int `json:"open"`
	Closed int `json:"closed"`
}

type StatusIssueLists struct {
	Open        []IssueStats `json:"open"`
	StaleOpen   []IssueStats `json:"stale_open"`
	BlockedOpen []IssueStats `json:"blocked_open"`
}

type IssueStats struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	State     string   `json:"state"`
	Labels    []string `json:"labels"`
	Milestone *string  `json:"milestone"`
	UpdatedAt string   `json:"updated_at"`
	URL       string   `json:"url"`
}

type MilestoneStats struct {
	Number          int     `json:"number"`
	Title           string  `json:"title"`
	State           string  `json:"state"`
	OpenIssues      int     `json:"open_issues"`
	ClosedIssues    int     `json:"closed_issues"`
	TotalIssues     int     `json:"total_issues"`
	ProgressPercent int     `json:"progress_percent"`
	DueOn           *string `json:"due_on"`
	Description     string  `json:"description"`
}

type normalizedIssue struct {
	Number    int
	Title     string
	State     string
	Labels    []string
	Milestone *string
	UpdatedAt string
	URL       string
}

type normalizedMilestone struct {
	Number       int
	Title        string
	State        string
	Description  string
	DueOn        *string
	OpenIssues   int
	ClosedIssues int
}

type statusPullRequest struct {
	Body  string
	Draft bool
}

func BuildStatusSummary(client StatusClient, fetchedAt time.Time, staleDays int) (StatusSummary, error) {
	var milestones []normalizedMilestone
	var issues []normalizedIssue
	var prs []statusPullRequest
	var milestonesErr error
	var issuesErr error
	var prsErr error
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		milestones, milestonesErr = FetchMilestones(client)
	}()
	go func() {
		defer wg.Done()
		issues, issuesErr = FetchIssues(client)
	}()
	go func() {
		defer wg.Done()
		prs, prsErr = FetchOpenPullRequests(client)
	}()
	wg.Wait()
	if milestonesErr != nil {
		return StatusSummary{}, milestonesErr
	}
	if issuesErr != nil {
		return StatusSummary{}, issuesErr
	}
	summary, err := SummarizeStatus(client.Repo().FullName(), milestones, issues, fetchedAt, staleDays)
	if err != nil {
		return StatusSummary{}, err
	}
	if prsErr == nil {
		summary.Counts.Issues.PRsMissingClosureLink, summary.Counts.Issues.ClosureLinkMissingOpenIssues = closureLinkGapMetrics(prs)
	}
	return summary, nil
}

func FetchMilestones(client StatusClient) ([]normalizedMilestone, error) {
	var pages json.RawMessage
	err := client.JSON([]string{
		"api",
		"repos/" + client.Repo().FullName() + "/milestones",
		"--paginate",
		"--slurp",
		"-X",
		"GET",
		"-f",
		"state=all",
		"-f",
		"per_page=100",
	}, &pages)
	if err != nil {
		return nil, err
	}

	rows, err := flattenPages(pages)
	if err != nil {
		return nil, err
	}
	milestones := make([]normalizedMilestone, 0, len(rows))
	for _, row := range rows {
		var raw struct {
			Number       int     `json:"number"`
			Title        string  `json:"title"`
			State        string  `json:"state"`
			Description  *string `json:"description"`
			DueOn        *string `json:"due_on"`
			OpenIssues   int     `json:"open_issues"`
			ClosedIssues int     `json:"closed_issues"`
		}
		if err := json.Unmarshal(row, &raw); err != nil {
			return nil, fmt.Errorf("parse milestone: %w", err)
		}
		description := ""
		if raw.Description != nil {
			description = *raw.Description
		}
		milestones = append(milestones, normalizedMilestone{
			Number:       raw.Number,
			Title:        raw.Title,
			State:        raw.State,
			Description:  description,
			DueOn:        raw.DueOn,
			OpenIssues:   raw.OpenIssues,
			ClosedIssues: raw.ClosedIssues,
		})
	}
	return milestones, nil
}

func FetchIssues(client StatusClient) ([]normalizedIssue, error) {
	var rows []json.RawMessage
	err := client.JSON([]string{
		"issue",
		"list",
		"--repo",
		client.Repo().FullName(),
		"--state",
		"all",
		"--limit",
		"1000",
		"--json",
		"number,title,state,labels,milestone,updatedAt,url",
	}, &rows)
	if err != nil {
		return fetchIssuesREST(client)
	}
	return normalizeIssueRows(rows)
}

func fetchIssuesREST(client StatusClient) ([]normalizedIssue, error) {
	var pages json.RawMessage
	err := client.JSON([]string{
		"api",
		"repos/" + client.Repo().FullName() + "/issues",
		"--paginate",
		"--slurp",
		"-X",
		"GET",
		"-f",
		"state=all",
		"-f",
		"per_page=100",
	}, &pages)
	if err != nil {
		return nil, err
	}
	rows, err := flattenPages(pages)
	if err != nil {
		return nil, err
	}
	return normalizeIssueRows(rows)
}

func normalizeIssueRows(rows []json.RawMessage) ([]normalizedIssue, error) {
	issues := make([]normalizedIssue, 0, len(rows))
	for _, row := range rows {
		var raw struct {
			Number int `json:"number"`
			Title  string
			State  string `json:"state"`
			Labels []struct {
				Name string `json:"name"`
			} `json:"labels"`
			Milestone *struct {
				Title string `json:"title"`
			} `json:"milestone"`
			UpdatedAt       string           `json:"updatedAt"`
			UpdatedAtREST   string           `json:"updated_at"`
			HTMLURL         string           `json:"html_url"`
			URL             string           `json:"url"`
			PullRequestREST *json.RawMessage `json:"pull_request"`
		}
		if err := json.Unmarshal(row, &raw); err != nil {
			return nil, fmt.Errorf("parse issue: %w", err)
		}
		if raw.PullRequestREST != nil {
			continue
		}
		labels := make([]string, 0, len(raw.Labels))
		for _, label := range raw.Labels {
			labels = append(labels, label.Name)
		}
		sort.Strings(labels)
		var milestone *string
		if raw.Milestone != nil {
			title := raw.Milestone.Title
			milestone = &title
		}
		url := raw.HTMLURL
		if url == "" {
			url = raw.URL
		}
		updatedAt := raw.UpdatedAt
		if updatedAt == "" {
			updatedAt = raw.UpdatedAtREST
		}
		issues = append(issues, normalizedIssue{
			Number:    raw.Number,
			Title:     raw.Title,
			State:     strings.ToLower(raw.State),
			Labels:    labels,
			Milestone: milestone,
			UpdatedAt: updatedAt,
			URL:       url,
		})
	}
	return issues, nil
}

func FetchOpenPullRequests(client StatusClient) ([]statusPullRequest, error) {
	var pages json.RawMessage
	err := client.JSON([]string{
		"api",
		"repos/" + client.Repo().FullName() + "/pulls",
		"--paginate",
		"--slurp",
		"-X",
		"GET",
		"-f",
		"state=open",
		"-f",
		"per_page=100",
	}, &pages)
	if err != nil {
		return nil, err
	}
	rows, err := flattenPages(pages)
	if err != nil {
		return nil, err
	}
	prs := make([]statusPullRequest, 0, len(rows))
	for _, row := range rows {
		var raw struct {
			Body  string `json:"body"`
			Draft bool   `json:"draft"`
		}
		if err := json.Unmarshal(row, &raw); err != nil {
			return nil, fmt.Errorf("parse pull request: %w", err)
		}
		prs = append(prs, statusPullRequest{Body: raw.Body, Draft: raw.Draft})
	}
	return prs, nil
}

func closureLinkGapMetrics(prs []statusPullRequest) (int, int) {
	missingPRs := 0
	for _, pr := range prs {
		if pr.Draft {
			continue
		}
		if len(ExtractClosureIssueNumbers(pr.Body)) == 0 {
			missingPRs++
		}
	}
	return missingPRs, missingPRs
}

func SummarizeStatus(repo string, milestones []normalizedMilestone, issues []normalizedIssue, fetchedAt time.Time, staleDays int) (StatusSummary, error) {
	now := fetchedAt.UTC().Truncate(time.Second)
	staleCutoff := now.AddDate(0, 0, -staleDays)

	openIssues := make([]normalizedIssue, 0)
	closedIssues := make([]normalizedIssue, 0)
	staleOpen := make([]normalizedIssue, 0)
	blockedOpen := make([]normalizedIssue, 0)
	for _, issue := range issues {
		switch issue.State {
		case "open":
			openIssues = append(openIssues, issue)
			updatedAt, err := parseGitHubTime(issue.UpdatedAt)
			if err != nil {
				return StatusSummary{}, err
			}
			if !updatedAt.After(staleCutoff) {
				staleOpen = append(staleOpen, issue)
			}
			if hasLabel(issue.Labels, "status:blocked") {
				blockedOpen = append(blockedOpen, issue)
			}
		case "closed":
			closedIssues = append(closedIssues, issue)
		}
	}

	sort.Slice(milestones, func(i, j int) bool { return milestones[i].Number < milestones[j].Number })
	sort.Slice(openIssues, func(i, j int) bool { return openIssues[i].Number < openIssues[j].Number })
	sort.Slice(blockedOpen, func(i, j int) bool { return blockedOpen[i].Number < blockedOpen[j].Number })
	sort.Slice(staleOpen, func(i, j int) bool {
		left, _ := parseGitHubTime(staleOpen[i].UpdatedAt)
		right, _ := parseGitHubTime(staleOpen[j].UpdatedAt)
		if left.Equal(right) {
			return staleOpen[i].Number < staleOpen[j].Number
		}
		return left.Before(right)
	})

	milestoneRows := make([]MilestoneStats, 0, len(milestones))
	for _, milestone := range milestones {
		total := milestone.OpenIssues + milestone.ClosedIssues
		progress := 0
		if total > 0 {
			progress = int(math.RoundToEven(float64(milestone.ClosedIssues) / float64(total) * 100))
		}
		milestoneRows = append(milestoneRows, MilestoneStats{
			Number:          milestone.Number,
			Title:           milestone.Title,
			State:           milestone.State,
			OpenIssues:      milestone.OpenIssues,
			ClosedIssues:    milestone.ClosedIssues,
			TotalIssues:     total,
			ProgressPercent: progress,
			DueOn:           milestone.DueOn,
			Description:     milestone.Description,
		})
	}

	openRows := issueSummaries(openIssues)
	staleRows := issueSummaries(staleOpen)
	blockedRows := issueSummaries(blockedOpen)
	return StatusSummary{
		Counts: StatusCounts{
			Issues: IssueCounts{
				Total:       len(issues),
				Open:        len(openIssues),
				Closed:      len(closedIssues),
				StaleOpen:   len(staleRows),
				BlockedOpen: len(blockedRows),
			},
			Milestones: MilestoneCounts{
				Total:  len(milestoneRows),
				Open:   countMilestones(milestoneRows, "open"),
				Closed: countMilestones(milestoneRows, "closed"),
			},
		},
		FetchedAt: formatGitHubTime(now),
		Issues: StatusIssueLists{
			Open:        openRows,
			StaleOpen:   staleRows,
			BlockedOpen: blockedRows,
		},
		Milestones: milestoneRows,
		Repo:       repo,
		StaleDays:  staleDays,
	}, nil
}

func FormatStatusText(summary StatusSummary) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "status: %s\n", summary.Repo)
	fmt.Fprintf(
		&builder,
		"issues: %d open, %d closed, %d total; %d stale open (%dd); %d blocked\n",
		summary.Counts.Issues.Open,
		summary.Counts.Issues.Closed,
		summary.Counts.Issues.Total,
		summary.Counts.Issues.StaleOpen,
		summary.StaleDays,
		summary.Counts.Issues.BlockedOpen,
	)
	fmt.Fprintf(
		&builder,
		"milestones: %d open, %d closed, %d total\n",
		summary.Counts.Milestones.Open,
		summary.Counts.Milestones.Closed,
		summary.Counts.Milestones.Total,
	)

	if len(summary.Milestones) == 0 {
		builder.WriteString("milestone progress: none\n")
	} else {
		builder.WriteString("milestone progress:\n")
		for _, milestone := range summary.Milestones {
			fmt.Fprintf(
				&builder,
				"  %s: %d open / %d closed (%d%%)\n",
				milestone.Title,
				milestone.OpenIssues,
				milestone.ClosedIssues,
				milestone.ProgressPercent,
			)
		}
	}

	writeIssueSection(&builder, "stale open issues", summary.Issues.StaleOpen, 0)
	writeIssueSection(&builder, "blocked issues", summary.Issues.BlockedOpen, 0)
	writeIssueSection(&builder, "open issues", summary.Issues.Open, 8)
	fmt.Fprintf(&builder, "next step: %s\n", statusNextStep(summary))
	return builder.String()
}

func statusNextStep(summary StatusSummary) string {
	if len(summary.Issues.BlockedOpen) > 0 {
		return fmt.Sprintf("gira work status --repo %s --issue %d", summary.Repo, summary.Issues.BlockedOpen[0].Number)
	}
	if len(summary.Issues.StaleOpen) > 0 {
		return fmt.Sprintf("gira work status --repo %s --issue %d", summary.Repo, summary.Issues.StaleOpen[0].Number)
	}
	if len(summary.Issues.Open) > 0 {
		return fmt.Sprintf("gira work start --repo %s --issue %d --dry-run", summary.Repo, summary.Issues.Open[0].Number)
	}
	return "gira ops sync --repo " + summary.Repo + " --dry-run"
}

func flattenPages(value json.RawMessage) ([]json.RawMessage, error) {
	var top []json.RawMessage
	if err := json.Unmarshal(value, &top); err != nil {
		return nil, fmt.Errorf("parse gh page list: %w", err)
	}
	if len(top) == 0 {
		return nil, nil
	}

	var maybePage []json.RawMessage
	if err := json.Unmarshal(top[0], &maybePage); err == nil {
		rows := make([]json.RawMessage, 0)
		for _, page := range top {
			var pageRows []json.RawMessage
			if err := json.Unmarshal(page, &pageRows); err != nil {
				return nil, fmt.Errorf("parse gh page: %w", err)
			}
			rows = append(rows, pageRows...)
		}
		return rows, nil
	}
	return top, nil
}

func issueSummaries(issues []normalizedIssue) []IssueStats {
	rows := make([]IssueStats, 0, len(issues))
	for _, issue := range issues {
		rows = append(rows, IssueStats{
			Number:    issue.Number,
			Title:     issue.Title,
			State:     issue.State,
			Labels:    append([]string(nil), issue.Labels...),
			Milestone: issue.Milestone,
			UpdatedAt: issue.UpdatedAt,
			URL:       issue.URL,
		})
	}
	return rows
}

func countMilestones(milestones []MilestoneStats, state string) int {
	count := 0
	for _, milestone := range milestones {
		if milestone.State == state {
			count++
		}
	}
	return count
}

func writeIssueSection(builder *strings.Builder, title string, issues []IssueStats, limit int) {
	if len(issues) == 0 {
		fmt.Fprintf(builder, "%s: none\n", title)
		return
	}
	fmt.Fprintf(builder, "%s: %d\n", title, len(issues))
	visible := issues
	if limit > 0 && len(issues) > limit {
		visible = issues[:limit]
	}
	for _, issue := range visible {
		milestone := ""
		if issue.Milestone != nil {
			milestone = " [" + *issue.Milestone + "]"
		}
		fmt.Fprintf(builder, "  #%d %s%s updated %s\n", issue.Number, issue.Title, milestone, issue.UpdatedAt)
	}
	if limit > 0 && len(issues) > limit {
		fmt.Fprintf(builder, "  ... %d more\n", len(issues)-limit)
	}
}

func hasLabel(labels []string, want string) bool {
	for _, label := range labels {
		if label == want {
			return true
		}
	}
	return false
}

func parseGitHubTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse GitHub timestamp %q: %w", value, err)
	}
	return parsed.UTC(), nil
}

func formatGitHubTime(value time.Time) string {
	return value.UTC().Truncate(time.Second).Format(time.RFC3339)
}
