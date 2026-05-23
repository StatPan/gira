package gira

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const GoalStatusSchemaVersion = "goal-status/v1"

type GoalStatusInput struct {
	Repo RepoRef `json:"repo"`
	Goal int     `json:"goal"`
}

type GoalStatusReport struct {
	Command                 string            `json:"command"`
	SchemaVersion           string            `json:"schema_version"`
	Repo                    string            `json:"repo"`
	Goal                    GoalStatusIssue   `json:"goal"`
	Children                []GoalStatusChild `json:"children"`
	Counts                  map[string]int    `json:"counts"`
	Blockers                []string          `json:"blockers,omitempty"`
	HandoffReceiptPresent   bool              `json:"handoff_receipt_present"`
	NextAction              string            `json:"next_action"`
	NextStep                string            `json:"next_step"`
	RemainingAutonomousWork int               `json:"remaining_autonomous_work"`
}

type GoalStatusIssue struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	State     string   `json:"state"`
	Status    string   `json:"status"`
	Labels    []string `json:"labels,omitempty"`
	Milestone string   `json:"milestone,omitempty"`
	URL       string   `json:"url,omitempty"`
}

type GoalStatusChild struct {
	Number       int      `json:"number"`
	Title        string   `json:"title"`
	State        string   `json:"state"`
	Status       string   `json:"status"`
	Category     string   `json:"category"`
	Labels       []string `json:"labels,omitempty"`
	PRNumber     int      `json:"pr_number,omitempty"`
	PRURL        string   `json:"pr_url,omitempty"`
	PRState      string   `json:"pr_state,omitempty"`
	ChecksStatus string   `json:"checks_status,omitempty"`
	ReviewStatus string   `json:"review_status,omitempty"`
	Blockers     []string `json:"blockers,omitempty"`
	NextAction   string   `json:"next_action"`
	NextStep     string   `json:"next_step"`
	URL          string   `json:"url,omitempty"`
}

func BuildGoalStatusReport(input GoalStatusInput, runner CommandRunner) (GoalStatusReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if input.Goal <= 0 {
		return GoalStatusReport{}, fmt.Errorf("goal must be > 0")
	}
	goal, err := fetchDevIssue(input.Repo, input.Goal, runner)
	if err != nil {
		return GoalStatusReport{}, err
	}
	if goal.IsPR {
		return GoalStatusReport{}, fmt.Errorf("goal #%d resolves to a pull request", input.Goal)
	}
	status := displayStatus(managedStatusFromLabels(goal.Labels))
	report := GoalStatusReport{
		Command:       "goal status",
		SchemaVersion: GoalStatusSchemaVersion,
		Repo:          input.Repo.FullName(),
		Goal: GoalStatusIssue{
			Number:    goal.Number,
			Title:     goal.Title,
			State:     goal.State,
			Status:    status,
			Labels:    append([]string(nil), goal.Labels...),
			Milestone: goal.Milestone,
			URL:       githubIssueURL(input.Repo, goal.Number),
		},
		Counts: map[string]int{},
	}
	report.HandoffReceiptPresent = goalFinishGoalReceiptPresent(input.Repo, goal.Number, runner)
	childNumbers, err := discoverGoalChildNumbers(input.Repo, goal, runner)
	if err != nil {
		return report, err
	}
	for _, childNumber := range childNumbers {
		childStatus, err := GetWorkStatus(input.Repo, childNumber, runner)
		if err != nil {
			report.Blockers = appendUniqueStrings(report.Blockers, fmt.Sprintf("child_%d_status_unavailable", childNumber))
			continue
		}
		child := goalStatusChildFromWorkStatus(input.Repo, childStatus)
		report.Children = append(report.Children, child)
		report.Counts[child.Category]++
		if child.Category == "blocked" && len(child.Blockers) == 0 {
			report.Blockers = appendUniqueStrings(report.Blockers, fmt.Sprintf("child_%d:blocked", child.Number))
		} else if len(child.Blockers) > 0 && child.Category != "done" && child.Category != "closed_other" {
			report.Blockers = appendUniqueStrings(report.Blockers, fmt.Sprintf("child_%d:%s", child.Number, strings.Join(child.Blockers, ",")))
		}
	}
	sort.Slice(report.Children, func(i, j int) bool {
		return report.Children[i].Number < report.Children[j].Number
	})
	report.Counts["total"] = len(report.Children)
	report.RemainingAutonomousWork = goalRemainingAutonomousWork(report.Children)
	report.NextAction, report.NextStep = goalStatusNextAction(input.Repo, report)
	return report, nil
}

func discoverGoalChildNumbers(repo RepoRef, goal devStartIssue, runner CommandRunner) ([]int, error) {
	numbers := map[int]struct{}{}
	search := fmt.Sprintf("repo:%s is:issue \"Parent: #%d\"", repo.FullName(), goal.Number)
	out, err := runner.Run("gh", "issue", "list", "--repo", repo.FullName(), "--state", "all", "--search", search, "--json", "number,title,state,url", "--limit", "100")
	if err != nil {
		return nil, fmt.Errorf("discover goal children: %w", err)
	}
	var rows []struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("parse goal child search JSON: %w", err)
	}
	for _, row := range rows {
		if row.Number > 0 && row.Number != goal.Number {
			numbers[row.Number] = struct{}{}
		}
	}
	for _, number := range goalChildRefsFromGoalText(goal.Body) {
		if number > 0 && number != goal.Number {
			numbers[number] = struct{}{}
		}
	}
	for _, number := range goalChildRefsFromComments(repo, goal.Number, runner) {
		if number > 0 && number != goal.Number {
			numbers[number] = struct{}{}
		}
	}
	outNumbers := make([]int, 0, len(numbers))
	for number := range numbers {
		outNumbers = append(outNumbers, number)
	}
	sort.Ints(outNumbers)
	return outNumbers, nil
}

func goalChildRefsFromComments(repo RepoRef, goal int, runner CommandRunner) []int {
	out, err := runner.Run("gh", "issue", "view", strconv.Itoa(goal), "--repo", repo.FullName(), "--json", "comments")
	if err != nil {
		return nil
	}
	var raw struct {
		Comments []struct {
			Body string `json:"body"`
		} `json:"comments"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil
	}
	numbers := []int{}
	for _, comment := range raw.Comments {
		numbers = append(numbers, goalChildRefsFromGoalText(comment.Body)...)
	}
	return numbers
}

var goalIssueRefPattern = regexp.MustCompile(`#([0-9]+)`)

func goalChildRefsFromGoalText(value string) []int {
	numbers := []int{}
	inChildSection := false
	for _, line := range strings.Split(value, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			inChildSection = goalChildHeading(trimmed)
			continue
		}
		if !inChildSection && !goalChildReferenceLine(trimmed) {
			continue
		}
		numbers = append(numbers, issueRefsFromText(trimmed)...)
	}
	return numbers
}

func goalChildHeading(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "child") && (strings.Contains(lower, "ticket") || strings.Contains(lower, "issue"))
}

func goalChildReferenceLine(line string) bool {
	lower := strings.ToLower(line)
	if strings.Contains(lower, "parent goal") || strings.HasPrefix(lower, "parent:") {
		return false
	}
	if strings.Contains(lower, "goal planning") || strings.Contains(lower, "goal plan") {
		return true
	}
	return strings.Contains(lower, "child") && (strings.Contains(lower, "ticket") || strings.Contains(lower, "issue"))
}

func issueRefsFromText(value string) []int {
	matches := goalIssueRefPattern.FindAllStringSubmatchIndex(value, -1)
	out := []int{}
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		if goalIssueRefHasPRPrefix(value, match[0]) {
			continue
		}
		number, err := strconv.Atoi(value[match[2]:match[3]])
		if err == nil && number > 0 {
			out = append(out, number)
		}
	}
	return out
}

func goalIssueRefHasPRPrefix(value string, hashStart int) bool {
	start := hashStart - 24
	if start < 0 {
		start = 0
	}
	prefix := strings.ToLower(strings.TrimSpace(value[start:hashStart]))
	for _, candidate := range []string{"pr", "pr:", "pull request", "pull-request", "pull_request"} {
		if strings.HasSuffix(prefix, candidate) {
			return true
		}
	}
	return false
}

func goalStatusChildFromWorkStatus(repo RepoRef, status WorkStatusResult) GoalStatusChild {
	category := goalChildCategory(status)
	blockers := goalRelevantChildBlockers(category, status)
	return GoalStatusChild{
		Number:       status.Issue,
		Title:        status.Title,
		State:        status.State,
		Status:       status.Status,
		Category:     category,
		Labels:       append([]string(nil), status.Labels...),
		PRNumber:     status.PRNumber,
		PRURL:        status.PRURL,
		PRState:      status.PRState,
		ChecksStatus: status.ChecksStatus,
		ReviewStatus: status.ReviewStatus,
		Blockers:     blockers,
		NextAction:   status.NextAction,
		NextStep:     status.NextStep,
		URL:          githubIssueURL(repo, status.Issue),
	}
}

func goalRelevantChildBlockers(category string, status WorkStatusResult) []string {
	if category == "done" || category == "closed_other" {
		return []string{}
	}
	blockers := []string{}
	for _, blocker := range status.Blockers {
		if blocker == "missing_linked_pr" {
			if category == "in_review" {
				blockers = append(blockers, blocker)
			}
			continue
		}
		blockers = appendUniqueStrings(blockers, blocker)
	}
	return blockers
}

func goalChildCategory(status WorkStatusResult) string {
	display := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(status.Status), " ", "_"))
	if strings.EqualFold(status.State, "closed") {
		if display == "done" {
			return "done"
		}
		return "closed_other"
	}
	if containsString(status.Blockers, "blocked") || display == "blocked" || status.NextAction == "blocked" {
		return "blocked"
	}
	switch display {
	case "ready":
		return "ready"
	case "in_progress":
		return "in_progress"
	case "in_review":
		return "in_review"
	case "done":
		return "done"
	default:
		return "unknown"
	}
}

func goalRemainingAutonomousWork(children []GoalStatusChild) int {
	remaining := 0
	for _, child := range children {
		switch child.Category {
		case "done", "closed_other":
			continue
		default:
			remaining++
		}
	}
	return remaining
}

func goalStatusNextAction(repo RepoRef, report GoalStatusReport) (string, string) {
	if len(report.Children) == 0 {
		return "plan_children", fmt.Sprintf("gira goal plan --repo %s --goal %d --dry-run", repo.FullName(), report.Goal.Number)
	}
	if len(report.Blockers) > 0 || report.Counts["blocked"] > 0 {
		return "resolve_blockers", fmt.Sprintf("gira goal status --repo %s --goal %d --json", repo.FullName(), report.Goal.Number)
	}
	if report.Counts["in_review"] > 0 {
		return "review_child", fmt.Sprintf("gira goal next --repo %s --goal %d --json", repo.FullName(), report.Goal.Number)
	}
	if report.Counts["in_progress"] > 0 {
		return "continue_child", fmt.Sprintf("gira goal next --repo %s --goal %d --json", repo.FullName(), report.Goal.Number)
	}
	if report.Counts["ready"] > 0 {
		return "start_next_child", fmt.Sprintf("gira goal next --repo %s --goal %d --json", repo.FullName(), report.Goal.Number)
	}
	if report.RemainingAutonomousWork == 0 {
		if report.HandoffReceiptPresent {
			return "human_review", fmt.Sprintf("review goal-finish-receipt/v1 handoff on #%d", report.Goal.Number)
		}
		return "finish_goal", fmt.Sprintf("gira goal finish --repo %s --goal %d --dry-run", repo.FullName(), report.Goal.Number)
	}
	return "inspect_goal", fmt.Sprintf("gira goal status --repo %s --goal %d --json", repo.FullName(), report.Goal.Number)
}

func githubIssueURL(repo RepoRef, issue int) string {
	if issue <= 0 {
		return ""
	}
	return fmt.Sprintf("https://github.com/%s/issues/%d", repo.FullName(), issue)
}

func FormatGoalStatus(report GoalStatusReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "goal status: #%d %s children=%d remaining=%d next=%s\n", report.Goal.Number, report.Goal.Status, len(report.Children), report.RemainingAutonomousWork, report.NextAction)
	if len(report.Children) > 0 {
		keys := []string{"ready", "in_progress", "in_review", "blocked", "done", "closed_other", "unknown"}
		parts := []string{}
		for _, key := range keys {
			if count := report.Counts[key]; count > 0 {
				parts = append(parts, fmt.Sprintf("%s=%d", key, count))
			}
		}
		if len(parts) > 0 {
			fmt.Fprintf(&b, "children: %s\n", strings.Join(parts, " "))
		}
	}
	if len(report.Blockers) > 0 {
		fmt.Fprintf(&b, "blockers: %s\n", strings.Join(report.Blockers, ","))
	}
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}
