package gira

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type EpicInput struct {
	Repo      RepoRef `json:"repo"`
	Ticket    int     `json:"ticket,omitempty"`
	Title     string  `json:"title,omitempty"`
	Slug      string  `json:"slug,omitempty"`
	Milestone string  `json:"milestone,omitempty"`
	DryRun    bool    `json:"dry_run,omitempty"`
	Apply     bool    `json:"apply,omitempty"`
}

type EpicReport struct {
	Repo       string          `json:"repo"`
	Epic       EpicIssue       `json:"epic"`
	DryRun     bool            `json:"dry_run,omitempty"`
	Apply      bool            `json:"apply,omitempty"`
	ChildCount EpicChildCount  `json:"child_count"`
	Children   []EpicChild     `json:"children,omitempty"`
	Actions    []EpicAction    `json:"actions,omitempty"`
	Blockers   []string        `json:"blockers,omitempty"`
	Candidates []EpicCandidate `json:"candidates,omitempty"`
	NextStep   string          `json:"next_step"`
}

type EpicIssue struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	State     string   `json:"state"`
	URL       string   `json:"url,omitempty"`
	Slug      string   `json:"slug"`
	Milestone string   `json:"milestone,omitempty"`
	Labels    []string `json:"labels,omitempty"`
}

type EpicCandidate struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Slug      string `json:"slug"`
	Milestone string `json:"milestone,omitempty"`
}

type EpicChild struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	State     string   `json:"state"`
	URL       string   `json:"url,omitempty"`
	Milestone string   `json:"milestone,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	Source    string   `json:"source"`
}

type EpicChildCount struct {
	Open   int `json:"open"`
	Closed int `json:"closed"`
	Total  int `json:"total"`
}

type EpicAction struct {
	Action       string   `json:"action"`
	Status       string   `json:"status"`
	Detail       string   `json:"detail,omitempty"`
	AddLabels    []string `json:"add_labels,omitempty"`
	RemoveLabels []string `json:"remove_labels,omitempty"`
}

type epicRawIssue struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	State       string    `json:"state"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PullRequest *struct{} `json:"pull_request"`
	Labels      []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Milestone *struct {
		Title string `json:"title"`
	} `json:"milestone"`
}

func BuildEpicStatusReport(input EpicInput, runner CommandRunner) (EpicReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	epic, candidates, err := resolveEpic(input, runner)
	report := EpicReport{Repo: input.Repo.FullName(), Candidates: candidates, NextStep: fmt.Sprintf("gira epic status --repo %s", input.Repo.FullName())}
	if err != nil {
		return report, err
	}
	report.Epic = epicIssueFromRaw(epic)
	children, err := fetchEpicChildren(input.Repo, epic, runner)
	if err != nil {
		return report, err
	}
	report.Children = children
	report.ChildCount = countEpicChildren(children)
	report.NextStep = epicStatusNextStep(input.Repo, report)
	return report, nil
}

func FinishEpic(input EpicInput, runner CommandRunner) (EpicReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if input.DryRun == input.Apply {
		return EpicReport{}, fmt.Errorf("exactly one of --dry-run or --apply is required")
	}
	report, err := BuildEpicStatusReport(input, runner)
	report.DryRun = input.DryRun
	report.Apply = input.Apply
	if err != nil {
		return report, err
	}
	if strings.EqualFold(report.Epic.State, "closed") {
		report.Actions = append(report.Actions, EpicAction{Action: "epic:close", Status: "skipped", Detail: "epic is already closed"})
		report.NextStep = "epic is closed"
		return report, nil
	}
	for _, child := range report.Children {
		if strings.EqualFold(child.State, "open") {
			report.Blockers = append(report.Blockers, fmt.Sprintf("open_child:#%d", child.Number))
		}
	}
	if len(report.Blockers) > 0 {
		report.Actions = append(report.Actions, EpicAction{Action: "epic:close", Status: "blocked", Detail: strings.Join(report.Blockers, ",")})
		report.NextStep = fmt.Sprintf("close child issues, then gira epic finish --repo %s --ticket %d --apply", input.Repo.FullName(), report.Epic.Number)
		if input.DryRun {
			return report, nil
		}
		return report, fmt.Errorf("epic finish blocked: %s", strings.Join(report.Blockers, ", "))
	}
	statusDoneExists, err := repoHasLabel(input.Repo, "status:done", runner)
	if err != nil {
		return report, err
	}
	removeLabels := activeStatusLabels(report.Epic.Labels)
	addLabels := []string{}
	if statusDoneExists && !hasLabel(report.Epic.Labels, "status:done") {
		addLabels = append(addLabels, "status:done")
	}
	if len(addLabels) > 0 || len(removeLabels) > 0 {
		action := EpicAction{Action: "epic:normalize-status", Status: plannedOrAppliedStatus(input.DryRun), AddLabels: addLabels, RemoveLabels: removeLabels}
		report.Actions = append(report.Actions, action)
		if input.Apply {
			if err := applyEpicLabels(input.Repo, report.Epic.Number, addLabels, removeLabels, runner); err != nil {
				return report, err
			}
		}
	}
	report.Actions = append(report.Actions, EpicAction{Action: "epic:close", Status: plannedOrAppliedStatus(input.DryRun), Detail: fmt.Sprintf("close epic #%d", report.Epic.Number)})
	if input.Apply {
		if _, err := runner.Run("gh", "issue", "close", strconv.Itoa(report.Epic.Number), "--repo", input.Repo.FullName(), "--comment", "Closed by gira epic finish"); err != nil {
			return report, err
		}
	}
	if input.DryRun {
		report.NextStep = fmt.Sprintf("gira epic finish --repo %s --ticket %d --apply", input.Repo.FullName(), report.Epic.Number)
	} else {
		report.NextStep = "epic is closed"
	}
	return report, nil
}

func resolveEpic(input EpicInput, runner CommandRunner) (epicRawIssue, []EpicCandidate, error) {
	if input.Ticket > 0 {
		issue, err := fetchEpicIssue(input.Repo, input.Ticket, runner)
		if err != nil {
			return epicRawIssue{}, nil, err
		}
		if !hasLabel(rawLabels(issue), "type:epic") {
			return epicRawIssue{}, nil, fmt.Errorf("issue #%d is not an epic; expected label type:epic", input.Ticket)
		}
		return issue, nil, nil
	}
	if branchIssue := inferIssueNumberFromLocalContext(input.Repo, runner); branchIssue > 0 {
		issue, err := fetchEpicIssue(input.Repo, branchIssue, runner)
		if err == nil && hasLabel(rawLabels(issue), "type:epic") {
			return issue, nil, nil
		}
	}
	issues, err := fetchOpenEpics(input.Repo, runner)
	if err != nil {
		return epicRawIssue{}, nil, err
	}
	filtered := filterEpicCandidates(issues, input)
	candidates := epicCandidatesFromRaw(filtered)
	if len(filtered) == 1 {
		return filtered[0], candidates, nil
	}
	if len(filtered) == 0 {
		return epicRawIssue{}, candidates, fmt.Errorf("epic context unavailable: pass --ticket N, --title, --slug, --milestone, run from an epic branch, or keep exactly one open epic")
	}
	return epicRawIssue{}, candidates, fmt.Errorf("epic context ambiguous: pass --ticket N, --title, --slug, or --milestone")
}

func fetchEpicIssue(repo RepoRef, issueNumber int, runner CommandRunner) (epicRawIssue, error) {
	if issueNumber <= 0 {
		return epicRawIssue{}, fmt.Errorf("ticket must be > 0")
	}
	out, err := runner.Run("gh", "api", "repos/"+repo.FullName()+"/issues/"+strconv.Itoa(issueNumber))
	if err != nil {
		return epicRawIssue{}, err
	}
	var issue epicRawIssue
	if err := json.Unmarshal(out, &issue); err != nil {
		return epicRawIssue{}, fmt.Errorf("parse issue JSON: %w", err)
	}
	if issue.PullRequest != nil {
		return epicRawIssue{}, fmt.Errorf("#%d is a pull request, not an epic issue", issueNumber)
	}
	return issue, nil
}

func fetchOpenEpics(repo RepoRef, runner CommandRunner) ([]epicRawIssue, error) {
	issues, err := fetchEpicIssues(repo, "open", runner)
	if err != nil {
		return nil, err
	}
	out := []epicRawIssue{}
	for _, issue := range issues {
		if hasLabel(rawLabels(issue), "type:epic") {
			out = append(out, issue)
		}
	}
	return out, nil
}

func fetchEpicIssues(repo RepoRef, state string, runner CommandRunner) ([]epicRawIssue, error) {
	out, err := runner.Run("gh", "api", "repos/"+repo.FullName()+"/issues", "--paginate", "--slurp", "-X", "GET", "-f", "state="+state, "-f", "per_page=100")
	if err != nil {
		return nil, err
	}
	var pages json.RawMessage
	if err := json.Unmarshal(out, &pages); err != nil {
		return nil, fmt.Errorf("parse issue pages: %w", err)
	}
	rows, err := flattenPages(pages)
	if err != nil {
		return nil, err
	}
	issues := []epicRawIssue{}
	for _, row := range rows {
		var issue epicRawIssue
		if err := json.Unmarshal(row, &issue); err != nil {
			return nil, fmt.Errorf("parse issue row: %w", err)
		}
		if issue.PullRequest != nil {
			continue
		}
		issues = append(issues, issue)
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].Number < issues[j].Number })
	return issues, nil
}

func fetchEpicChildren(repo RepoRef, epic epicRawIssue, runner CommandRunner) ([]EpicChild, error) {
	issues, err := fetchEpicIssues(repo, "all", runner)
	if err != nil {
		return nil, err
	}
	referenced := extractIssueRefs(epic.Body)
	childrenByNumber := map[int]EpicChild{}
	for _, issue := range issues {
		if issue.Number == epic.Number || hasLabel(rawLabels(issue), "type:epic") {
			continue
		}
		_, bodyRef := referenced[issue.Number]
		milestoneRef := rawMilestone(issue) != "" && rawMilestone(issue) == rawMilestone(epic)
		if !bodyRef && !milestoneRef {
			continue
		}
		source := "milestone"
		if bodyRef && milestoneRef {
			source = "body,milestone"
		} else if bodyRef {
			source = "body"
		}
		childrenByNumber[issue.Number] = EpicChild{
			Number:    issue.Number,
			Title:     issue.Title,
			State:     issue.State,
			URL:       issue.HTMLURL,
			Milestone: rawMilestone(issue),
			Labels:    rawLabels(issue),
			Source:    source,
		}
	}
	children := make([]EpicChild, 0, len(childrenByNumber))
	for _, child := range childrenByNumber {
		children = append(children, child)
	}
	sort.Slice(children, func(i, j int) bool { return children[i].Number < children[j].Number })
	return children, nil
}

func filterEpicCandidates(issues []epicRawIssue, input EpicInput) []epicRawIssue {
	title := strings.ToLower(strings.TrimSpace(input.Title))
	slug := strings.ToLower(strings.TrimSpace(input.Slug))
	milestone := strings.ToLower(strings.TrimSpace(input.Milestone))
	out := []epicRawIssue{}
	for _, issue := range issues {
		if title != "" && !strings.Contains(strings.ToLower(issue.Title), title) {
			continue
		}
		if slug != "" && slugifyEpic(issue.Title) != slug {
			continue
		}
		if milestone != "" && strings.ToLower(rawMilestone(issue)) != milestone {
			continue
		}
		out = append(out, issue)
	}
	return out
}

func epicIssueFromRaw(issue epicRawIssue) EpicIssue {
	return EpicIssue{Number: issue.Number, Title: issue.Title, State: issue.State, URL: issue.HTMLURL, Slug: slugifyEpic(issue.Title), Milestone: rawMilestone(issue), Labels: rawLabels(issue)}
}

func epicCandidatesFromRaw(issues []epicRawIssue) []EpicCandidate {
	out := make([]EpicCandidate, 0, len(issues))
	for _, issue := range issues {
		out = append(out, EpicCandidate{Number: issue.Number, Title: issue.Title, Slug: slugifyEpic(issue.Title), Milestone: rawMilestone(issue)})
	}
	return out
}

func countEpicChildren(children []EpicChild) EpicChildCount {
	count := EpicChildCount{Total: len(children)}
	for _, child := range children {
		if strings.EqualFold(child.State, "closed") {
			count.Closed++
		} else {
			count.Open++
		}
	}
	return count
}

func epicStatusNextStep(repo RepoRef, report EpicReport) string {
	if report.Epic.Number == 0 {
		return fmt.Sprintf("gira epic status --repo %s", repo.FullName())
	}
	if report.ChildCount.Open > 0 {
		return fmt.Sprintf("close child issues, then gira epic finish --repo %s --ticket %d --dry-run", repo.FullName(), report.Epic.Number)
	}
	if strings.EqualFold(report.Epic.State, "closed") {
		return "epic is closed"
	}
	return fmt.Sprintf("gira epic finish --repo %s --ticket %d --dry-run", repo.FullName(), report.Epic.Number)
}

func applyEpicLabels(repo RepoRef, issueNumber int, addLabels []string, removeLabels []string, runner CommandRunner) error {
	args := []string{"issue", "edit", strconv.Itoa(issueNumber), "--repo", repo.FullName()}
	for _, label := range addLabels {
		args = append(args, "--add-label", label)
	}
	for _, label := range removeLabels {
		args = append(args, "--remove-label", label)
	}
	_, err := runner.Run("gh", args...)
	return err
}

func plannedOrAppliedStatus(dryRun bool) string {
	if dryRun {
		return "planned"
	}
	return "applied"
}

func rawLabels(issue epicRawIssue) []string {
	labels := make([]string, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		labels = append(labels, label.Name)
	}
	sort.Strings(labels)
	return labels
}

func rawMilestone(issue epicRawIssue) string {
	if issue.Milestone == nil {
		return ""
	}
	return issue.Milestone.Title
}

func extractIssueRefs(body string) map[int]struct{} {
	refs := map[int]struct{}{}
	re := regexp.MustCompile(`#([1-9][0-9]*)`)
	for _, match := range re.FindAllStringSubmatch(body, -1) {
		n, _ := strconv.Atoi(match[1])
		if n > 0 {
			refs[n] = struct{}{}
		}
	}
	return refs
}

func inferIssueNumberFromLocalContext(repo RepoRef, runner CommandRunner) int {
	if out, err := runner.Run("git", "branch", "--show-current"); err == nil {
		if n := issueNumberFromBranchRef(strings.TrimSpace(string(out))); n > 0 {
			return n
		}
	}
	out, err := runner.Run("gh", "pr", "view", "--repo", repo.FullName(), "--json", "body,headRefName,title")
	if err != nil {
		return 0
	}
	var raw struct {
		Body        string `json:"body"`
		HeadRefName string `json:"headRefName"`
		Title       string `json:"title"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return 0
	}
	issues := ExtractClosureIssueNumbers(raw.Body)
	if len(issues) == 1 {
		return issues[0]
	}
	for _, ref := range []string{raw.HeadRefName, raw.Title} {
		if n := issueNumberFromBranchRef(ref); n > 0 {
			return n
		}
	}
	return 0
}

func issueNumberFromBranchRef(ref string) int {
	for _, segment := range strings.Split(ref, "/") {
		if !strings.HasPrefix(segment, "issue-") {
			continue
		}
		rest := strings.TrimPrefix(segment, "issue-")
		digits := strings.Builder{}
		for _, r := range rest {
			if r < '0' || r > '9' {
				break
			}
			digits.WriteRune(r)
		}
		if digits.Len() == 0 {
			continue
		}
		n, _ := strconv.Atoi(digits.String())
		if n > 0 {
			return n
		}
	}
	return 0
}

func slugifyEpic(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func FormatEpicReport(report EpicReport) string {
	var b strings.Builder
	if report.Epic.Number > 0 {
		fmt.Fprintf(&b, "epic status: epic #%d %s state=%s children=%d open=%d closed=%d\n", report.Epic.Number, report.Epic.Title, report.Epic.State, report.ChildCount.Total, report.ChildCount.Open, report.ChildCount.Closed)
	} else {
		b.WriteString("epic status: unresolved\n")
	}
	if len(report.Candidates) > 1 {
		b.WriteString("candidates:\n")
		for _, candidate := range report.Candidates {
			fmt.Fprintf(&b, "  #%d %s slug=%s", candidate.Number, candidate.Title, candidate.Slug)
			if candidate.Milestone != "" {
				fmt.Fprintf(&b, " milestone=%s", candidate.Milestone)
			}
			b.WriteString("\n")
		}
	}
	if len(report.Children) > 0 {
		b.WriteString("children:\n")
		for _, child := range report.Children {
			fmt.Fprintf(&b, "  #%d %s state=%s source=%s\n", child.Number, child.Title, child.State, child.Source)
		}
	}
	if len(report.Blockers) > 0 {
		fmt.Fprintf(&b, "blockers: %s\n", strings.Join(report.Blockers, ","))
	}
	if len(report.Actions) > 0 {
		b.WriteString("actions:\n")
		for _, action := range report.Actions {
			fmt.Fprintf(&b, "  %s:%s", action.Action, action.Status)
			if len(action.AddLabels) > 0 {
				fmt.Fprintf(&b, " add_labels=%s", strings.Join(action.AddLabels, ","))
			}
			if len(action.RemoveLabels) > 0 {
				fmt.Fprintf(&b, " remove_labels=%s", strings.Join(action.RemoveLabels, ","))
			}
			if action.Detail != "" {
				fmt.Fprintf(&b, " detail=%s", action.Detail)
			}
			b.WriteString("\n")
		}
	}
	if report.NextStep != "" {
		fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	}
	return b.String()
}
