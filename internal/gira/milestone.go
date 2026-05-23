package gira

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

type MilestoneNewInput struct {
	Repo        RepoRef `json:"repo"`
	Title       string  `json:"title"`
	Description string  `json:"description,omitempty"`
	DueOn       string  `json:"due_on,omitempty"`
	DryRun      bool    `json:"dry_run"`
	Apply       bool    `json:"apply"`
}

type MilestoneListOptions struct {
	Repo  RepoRef `json:"repo"`
	State string  `json:"state"`
}

type MilestoneStatusOptions struct {
	Repo      RepoRef `json:"repo"`
	Milestone string  `json:"milestone"`
}

type MilestoneAssignInput struct {
	Repo      RepoRef `json:"repo"`
	Milestone string  `json:"milestone"`
	Tickets   []int   `json:"tickets"`
	DryRun    bool    `json:"dry_run"`
	Apply     bool    `json:"apply"`
}

type MilestonePlanInput struct {
	Repo      RepoRef  `json:"repo"`
	Milestone string   `json:"milestone"`
	Labels    []string `json:"labels,omitempty"`
	State     string   `json:"state,omitempty"`
	Limit     int      `json:"limit,omitempty"`
	DryRun    bool     `json:"dry_run"`
	Apply     bool     `json:"apply"`
}

const MilestoneReportSchemaVersion = "milestone-report/v1"

type MilestoneReport struct {
	SchemaVersion string              `json:"schema_version,omitempty"`
	Command       string              `json:"command"`
	Repo          string              `json:"repo"`
	DryRun        bool                `json:"dry_run,omitempty"`
	Apply         bool                `json:"apply,omitempty"`
	Milestone     *MilestoneItem      `json:"milestone,omitempty"`
	Milestones    []MilestoneItem     `json:"milestones,omitempty"`
	Filters       MilestoneFilters    `json:"filters,omitempty"`
	Counts        MilestoneWorkCounts `json:"counts,omitempty"`
	Tickets       []MilestoneTicket   `json:"tickets,omitempty"`
	Actions       []MilestoneAction   `json:"actions,omitempty"`
	NextStep      string              `json:"next_step,omitempty"`
	Approval      *ApprovalEvidence   `json:"approval,omitempty"`
}

func EnsureMilestoneReportSchema(report *MilestoneReport) {
	if report != nil && strings.TrimSpace(report.SchemaVersion) == "" {
		report.SchemaVersion = MilestoneReportSchemaVersion
	}
}

func MilestoneReportSupportsApproval(report MilestoneReport) bool {
	switch report.Command {
	case "milestone new", "milestone assign", "milestone plan":
		return true
	default:
		return false
	}
}

type MilestoneFilters struct {
	State     string   `json:"state,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	Milestone string   `json:"milestone,omitempty"`
	Limit     int      `json:"limit,omitempty"`
}

type MilestoneWorkCounts struct {
	Milestones      int `json:"milestones,omitempty"`
	Tickets         int `json:"tickets,omitempty"`
	Open            int `json:"open,omitempty"`
	Closed          int `json:"closed,omitempty"`
	Ready           int `json:"ready,omitempty"`
	InProgress      int `json:"in_progress,omitempty"`
	InReview        int `json:"in_review,omitempty"`
	Blocked         int `json:"blocked,omitempty"`
	Done            int `json:"done,omitempty"`
	FinishReady     int `json:"finish_ready,omitempty"`
	WouldAssign     int `json:"would_assign,omitempty"`
	AppliedAssign   int `json:"applied_assign,omitempty"`
	SkippedAssigned int `json:"skipped_assigned,omitempty"`
}

type MilestoneItem struct {
	Number          int     `json:"number"`
	Title           string  `json:"title"`
	State           string  `json:"state"`
	DueOn           *string `json:"due_on,omitempty"`
	Description     string  `json:"description,omitempty"`
	OpenIssues      int     `json:"open_issues"`
	ClosedIssues    int     `json:"closed_issues"`
	TotalIssues     int     `json:"total_issues"`
	ProgressPercent int     `json:"progress_percent"`
}

type MilestoneTicket struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	State     string   `json:"state"`
	Status    string   `json:"status,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	Milestone string   `json:"milestone,omitempty"`
	URL       string   `json:"url,omitempty"`
}

type MilestoneAction struct {
	Action    string `json:"action"`
	Status    string `json:"status"`
	Issue     int    `json:"issue,omitempty"`
	Title     string `json:"title,omitempty"`
	Milestone string `json:"milestone"`
	Reason    string `json:"reason,omitempty"`
}

func BuildMilestoneNewReport(input MilestoneNewInput, runner CommandRunner) (MilestoneReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if input.DryRun == input.Apply {
		return MilestoneReport{}, fmt.Errorf("exactly one of --dry-run or --apply is required")
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return MilestoneReport{}, fmt.Errorf("milestone title is required")
	}
	dueOn := normalizeMilestoneDueOn(input.DueOn)
	report := MilestoneReport{SchemaVersion: MilestoneReportSchemaVersion, Command: "milestone new", Repo: input.Repo.FullName(), DryRun: input.DryRun, Apply: input.Apply}
	existing, err := fetchMilestonesForRepo(input.Repo, runner)
	if err != nil {
		return report, err
	}
	if match, ok := findMilestoneByTitle(existing, title); ok {
		item := milestoneItem(match)
		report.Milestone = &item
		report.Actions = []MilestoneAction{{Action: "milestone:create", Status: "skipped", Milestone: title, Reason: "milestone already exists"}}
		report.NextStep = fmt.Sprintf("gira milestone status %s --repo %s", QuoteShellArg(title), QuoteShellArg(input.Repo.FullName()))
		if input.DryRun {
			report.Approval = MilestoneApprovalEvidence(report)
		}
		return report, nil
	}
	item := MilestoneItem{Title: title, State: "open", Description: strings.TrimSpace(input.Description)}
	if dueOn != "" {
		item.DueOn = &dueOn
	}
	action := MilestoneAction{Action: "milestone:create", Status: "planned", Milestone: title, Reason: "new milestone"}
	if input.Apply {
		created, err := createMilestone(input.Repo, title, strings.TrimSpace(input.Description), dueOn, runner)
		if err != nil {
			return report, err
		}
		item = milestoneItem(created)
		action.Status = "applied"
	}
	report.Milestone = &item
	report.Actions = []MilestoneAction{action}
	if input.DryRun {
		report.NextStep = fmt.Sprintf("gira milestone new %s --repo %s --apply", QuoteShellArg(title), QuoteShellArg(input.Repo.FullName()))
		report.Approval = MilestoneApprovalEvidence(report)
	} else {
		report.NextStep = fmt.Sprintf("gira milestone status %s --repo %s", QuoteShellArg(title), QuoteShellArg(input.Repo.FullName()))
	}
	return report, nil
}

func BuildMilestoneListReport(options MilestoneListOptions, runner CommandRunner) (MilestoneReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	state := strings.ToLower(strings.TrimSpace(options.State))
	if state == "" {
		state = "open"
	}
	if state != "open" && state != "closed" && state != "all" {
		return MilestoneReport{}, fmt.Errorf("--state must be one of open, closed, all")
	}
	milestones, err := fetchMilestonesForRepo(options.Repo, runner)
	if err != nil {
		return MilestoneReport{}, err
	}
	items := []MilestoneItem{}
	for _, milestone := range milestones {
		if state != "all" && !strings.EqualFold(milestone.State, state) {
			continue
		}
		items = append(items, milestoneItem(milestone))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Number < items[j].Number })
	return MilestoneReport{
		SchemaVersion: MilestoneReportSchemaVersion,
		Command:       "milestone list",
		Repo:          options.Repo.FullName(),
		Milestones:    items,
		Filters:       MilestoneFilters{State: state},
		Counts:        MilestoneWorkCounts{Milestones: len(items)},
		NextStep:      "gira milestone status MILESTONE",
	}, nil
}

func BuildMilestoneStatusReport(options MilestoneStatusOptions, runner CommandRunner) (MilestoneReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	title := strings.TrimSpace(options.Milestone)
	if title == "" {
		return MilestoneReport{}, fmt.Errorf("milestone title is required")
	}
	client := NewGHStatusClient(options.Repo, runner)
	milestones, err := FetchMilestones(client)
	if err != nil {
		return MilestoneReport{}, err
	}
	milestone, err := resolveMilestone(milestones, title)
	if err != nil {
		return MilestoneReport{}, err
	}
	issues, err := FetchIssues(client)
	if err != nil {
		return MilestoneReport{}, err
	}
	item := milestoneItem(milestone)
	report := MilestoneReport{SchemaVersion: MilestoneReportSchemaVersion, Command: "milestone status", Repo: options.Repo.FullName(), Milestone: &item, Filters: MilestoneFilters{Milestone: milestone.Title}}
	for _, issue := range issues {
		if issue.Milestone == nil || *issue.Milestone != milestone.Title {
			continue
		}
		ticket := milestoneTicket(issue)
		report.Tickets = append(report.Tickets, ticket)
		countMilestoneTicket(&report.Counts, ticket)
	}
	sort.Slice(report.Tickets, func(i, j int) bool { return report.Tickets[i].Number < report.Tickets[j].Number })
	report.Counts.Tickets = len(report.Tickets)
	report.NextStep = milestoneStatusNextStep(report)
	return report, nil
}

func BuildMilestoneAssignReport(input MilestoneAssignInput, runner CommandRunner) (MilestoneReport, error) {
	return buildMilestoneAssignment(input.Repo, input.Milestone, input.Tickets, input.DryRun, input.Apply, runner, "milestone assign")
}

func BuildMilestonePlanReport(input MilestonePlanInput, runner CommandRunner) (MilestoneReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if input.DryRun == input.Apply {
		return MilestoneReport{}, fmt.Errorf("exactly one of --dry-run or --apply is required")
	}
	labels := normalizeTicketListLabels(input.Labels)
	if len(labels) == 0 {
		labels = []string{"status:ready"}
	}
	state := strings.ToLower(strings.TrimSpace(input.State))
	if state == "" {
		state = "open"
	}
	limit := input.Limit
	if limit == 0 {
		limit = 20
	}
	list, err := BuildTicketListReport(TicketListOptions{Repo: input.Repo, State: state, Labels: labels, Limit: limit}, runner)
	if err != nil {
		return MilestoneReport{}, err
	}
	tickets := make([]int, 0, len(list.Tickets))
	for _, ticket := range list.Tickets {
		tickets = append(tickets, ticket.Number)
	}
	if len(tickets) == 0 {
		milestones, err := fetchMilestonesForRepo(input.Repo, runner)
		if err != nil {
			return MilestoneReport{}, err
		}
		milestone, err := resolveMilestone(milestones, input.Milestone)
		if err != nil {
			return MilestoneReport{}, err
		}
		item := milestoneItem(milestone)
		report := MilestoneReport{
			SchemaVersion: MilestoneReportSchemaVersion,
			Command:       "milestone plan",
			Repo:          input.Repo.FullName(),
			DryRun:        input.DryRun,
			Apply:         input.Apply,
			Milestone:     &item,
			Filters:       MilestoneFilters{Milestone: milestone.Title, State: state, Labels: labels, Limit: limit},
			NextStep:      "no matching tickets found",
		}
		if input.DryRun {
			report.Approval = MilestoneApprovalEvidence(report)
		}
		return report, nil
	}
	report, err := buildMilestoneAssignment(input.Repo, input.Milestone, tickets, input.DryRun, input.Apply, runner, "milestone plan")
	if err != nil {
		return report, err
	}
	report.Filters.State = state
	report.Filters.Labels = labels
	report.Filters.Limit = limit
	if input.DryRun {
		next := fmt.Sprintf("gira milestone plan %s --repo %s", QuoteShellArg(report.Milestone.Title), QuoteShellArg(input.Repo.FullName()))
		for _, label := range labels {
			next += " --label " + QuoteShellArg(label)
		}
		next += fmt.Sprintf(" --limit %d --apply", limit)
		report.NextStep = next
		report.Approval = MilestoneApprovalEvidence(report)
	}
	return report, nil
}

func buildMilestoneAssignment(repo RepoRef, title string, tickets []int, dryRun bool, apply bool, runner CommandRunner, command string) (MilestoneReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if dryRun == apply {
		return MilestoneReport{}, fmt.Errorf("exactly one of --dry-run or --apply is required")
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return MilestoneReport{}, fmt.Errorf("milestone title is required")
	}
	if len(tickets) == 0 {
		return MilestoneReport{}, fmt.Errorf("--tickets or a matching plan selection is required")
	}
	milestones, err := fetchMilestonesForRepo(repo, runner)
	if err != nil {
		return MilestoneReport{}, err
	}
	milestone, err := resolveMilestone(milestones, title)
	if err != nil {
		return MilestoneReport{}, err
	}
	item := milestoneItem(milestone)
	report := MilestoneReport{SchemaVersion: MilestoneReportSchemaVersion, Command: command, Repo: repo.FullName(), DryRun: dryRun, Apply: apply, Milestone: &item, Filters: MilestoneFilters{Milestone: milestone.Title}}
	seen := map[int]struct{}{}
	for _, ticket := range tickets {
		if ticket <= 0 {
			return report, fmt.Errorf("ticket numbers must be > 0")
		}
		if _, ok := seen[ticket]; ok {
			continue
		}
		seen[ticket] = struct{}{}
		action := MilestoneAction{Action: "issue:assign-milestone", Issue: ticket, Milestone: milestone.Title, Status: "planned", Reason: "selected ticket"}
		if apply {
			if err := assignIssueMilestone(repo, ticket, milestone.Title, runner); err != nil {
				return report, err
			}
			action.Status = "applied"
			report.Counts.AppliedAssign++
		} else {
			report.Counts.WouldAssign++
		}
		report.Actions = append(report.Actions, action)
	}
	report.Counts.Tickets = len(report.Actions)
	if dryRun {
		report.NextStep = fmt.Sprintf("gira %s %s --repo %s --tickets %s --apply", command, QuoteShellArg(milestone.Title), QuoteShellArg(repo.FullName()), joinIssueNumbers(tickets))
		report.Approval = MilestoneApprovalEvidence(report)
	} else {
		report.NextStep = fmt.Sprintf("gira milestone status %s --repo %s", QuoteShellArg(milestone.Title), QuoteShellArg(repo.FullName()))
	}
	return report, nil
}

func fetchMilestonesForRepo(repo RepoRef, runner CommandRunner) ([]normalizedMilestone, error) {
	return FetchMilestones(NewGHStatusClient(repo, runner))
}

func createMilestone(repo RepoRef, title string, description string, dueOn string, runner CommandRunner) (normalizedMilestone, error) {
	args := []string{"api", "repos/" + repo.FullName() + "/milestones", "-X", "POST", "-f", "title=" + title}
	if description != "" {
		args = append(args, "-f", "description="+description)
	}
	if dueOn != "" {
		args = append(args, "-f", "due_on="+dueOn)
	}
	out, err := runner.Run("gh", args...)
	if err != nil {
		return normalizedMilestone{}, err
	}
	var raw struct {
		Number       int     `json:"number"`
		Title        string  `json:"title"`
		State        string  `json:"state"`
		Description  *string `json:"description"`
		DueOn        *string `json:"due_on"`
		OpenIssues   int     `json:"open_issues"`
		ClosedIssues int     `json:"closed_issues"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return normalizedMilestone{}, fmt.Errorf("parse milestone create JSON: %w", err)
	}
	descriptionValue := ""
	if raw.Description != nil {
		descriptionValue = *raw.Description
	}
	return normalizedMilestone{Number: raw.Number, Title: raw.Title, State: raw.State, Description: descriptionValue, DueOn: raw.DueOn, OpenIssues: raw.OpenIssues, ClosedIssues: raw.ClosedIssues}, nil
}

func assignIssueMilestone(repo RepoRef, issue int, milestone string, runner CommandRunner) error {
	_, err := runner.Run("gh", "issue", "edit", strconv.Itoa(issue), "--repo", repo.FullName(), "--milestone", milestone)
	return err
}

func resolveMilestone(milestones []normalizedMilestone, title string) (normalizedMilestone, error) {
	matches := []normalizedMilestone{}
	for _, milestone := range milestones {
		if strings.EqualFold(strings.TrimSpace(milestone.Title), strings.TrimSpace(title)) {
			matches = append(matches, milestone)
		}
	}
	if len(matches) == 0 {
		return normalizedMilestone{}, fmt.Errorf("milestone %q was not found", title)
	}
	if len(matches) > 1 {
		numbers := make([]string, 0, len(matches))
		for _, match := range matches {
			numbers = append(numbers, fmt.Sprintf("#%d", match.Number))
		}
		return normalizedMilestone{}, fmt.Errorf("milestone %q is ambiguous: %s", title, strings.Join(numbers, ", "))
	}
	return matches[0], nil
}

func findMilestoneByTitle(milestones []normalizedMilestone, title string) (normalizedMilestone, bool) {
	for _, milestone := range milestones {
		if strings.EqualFold(strings.TrimSpace(milestone.Title), strings.TrimSpace(title)) {
			return milestone, true
		}
	}
	return normalizedMilestone{}, false
}

func milestoneItem(milestone normalizedMilestone) MilestoneItem {
	total := milestone.OpenIssues + milestone.ClosedIssues
	progress := 0
	if total > 0 {
		progress = int(math.RoundToEven(float64(milestone.ClosedIssues) / float64(total) * 100))
	}
	return MilestoneItem{Number: milestone.Number, Title: milestone.Title, State: milestone.State, DueOn: milestone.DueOn, Description: milestone.Description, OpenIssues: milestone.OpenIssues, ClosedIssues: milestone.ClosedIssues, TotalIssues: total, ProgressPercent: progress}
}

func milestoneTicket(issue normalizedIssue) MilestoneTicket {
	milestone := ""
	if issue.Milestone != nil {
		milestone = *issue.Milestone
	}
	status := statusFromLabels(issue.Labels)
	if strings.EqualFold(issue.State, "closed") && status == "open" {
		status = "done"
	}
	return MilestoneTicket{Number: issue.Number, Title: issue.Title, State: issue.State, Status: status, Labels: ticketListKeyLabels(issue.Labels), Milestone: milestone, URL: issue.URL}
}

func countMilestoneTicket(counts *MilestoneWorkCounts, ticket MilestoneTicket) {
	switch strings.ToLower(ticket.State) {
	case "open":
		counts.Open++
	case "closed":
		counts.Closed++
	}
	switch ticket.Status {
	case "ready":
		counts.Ready++
	case "in-progress":
		counts.InProgress++
	case "in-review", "needs-review":
		counts.InReview++
	case "blocked":
		counts.Blocked++
	case "done":
		counts.Done++
	}
	if strings.EqualFold(ticket.State, "closed") || ticket.Status == "done" {
		counts.FinishReady++
	}
}

func milestoneStatusNextStep(report MilestoneReport) string {
	if report.Counts.Tickets == 0 {
		return "gira milestone plan " + QuoteShellArg(report.Milestone.Title) + " --repo " + QuoteShellArg(report.Repo) + " --label status:ready --dry-run"
	}
	if report.Counts.Blocked > 0 {
		return "resolve blocked tickets before closing this milestone"
	}
	if report.Counts.InReview > 0 {
		return "review in-review tickets before closing this milestone"
	}
	if report.Counts.Open == 0 {
		return "milestone has no open tickets"
	}
	return "continue ticket work or run gira milestone assign/plan to adjust the batch"
}

func normalizeMilestoneDueOn(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) == len("2006-01-02") {
		return value + "T23:59:59Z"
	}
	return value
}

func FormatMilestoneReport(report MilestoneReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s", report.Command, report.Repo)
	if report.DryRun {
		b.WriteString(" dry-run")
	}
	if report.Apply {
		b.WriteString(" applied")
	}
	b.WriteString("\n")
	if report.Milestone != nil {
		writeMilestoneItem(&b, *report.Milestone)
	}
	if len(report.Milestones) > 0 {
		b.WriteString("milestones:\n")
		for _, milestone := range report.Milestones {
			b.WriteString("  ")
			writeMilestoneItem(&b, milestone)
		}
	}
	if report.Counts.Tickets > 0 || len(report.Tickets) > 0 {
		fmt.Fprintf(&b, "tickets: total=%d open=%d closed=%d ready=%d in_progress=%d in_review=%d blocked=%d done=%d finish_ready=%d\n", report.Counts.Tickets, report.Counts.Open, report.Counts.Closed, report.Counts.Ready, report.Counts.InProgress, report.Counts.InReview, report.Counts.Blocked, report.Counts.Done, report.Counts.FinishReady)
	}
	for _, ticket := range report.Tickets {
		fmt.Fprintf(&b, "  #%d %-6s %s", ticket.Number, ticket.State, ticket.Title)
		if ticket.Status != "" {
			fmt.Fprintf(&b, " status=%s", ticket.Status)
		}
		b.WriteString("\n")
	}
	if len(report.Actions) > 0 {
		b.WriteString("actions:\n")
		for _, action := range report.Actions {
			if action.Issue > 0 {
				fmt.Fprintf(&b, "  %s %s issue #%d milestone=%s\n", action.Status, action.Action, action.Issue, action.Milestone)
			} else {
				fmt.Fprintf(&b, "  %s %s milestone=%s", action.Status, action.Action, action.Milestone)
				if action.Reason != "" {
					fmt.Fprintf(&b, " reason=%s", action.Reason)
				}
				b.WriteString("\n")
			}
		}
	}
	if report.NextStep != "" {
		fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	}
	return b.String()
}

func writeMilestoneItem(b *strings.Builder, milestone MilestoneItem) {
	fmt.Fprintf(b, "#%d [%s] %s progress=%d%% open=%d closed=%d", milestone.Number, milestone.State, milestone.Title, milestone.ProgressPercent, milestone.OpenIssues, milestone.ClosedIssues)
	if milestone.DueOn != nil {
		fmt.Fprintf(b, " due_on=%s", *milestone.DueOn)
	}
	b.WriteString("\n")
}
