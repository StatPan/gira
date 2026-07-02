package gira

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const TicketParentReportSchemaVersion = "ticket-parent-report/v1"

type TicketParentInput struct {
	Repo   RepoRef `json:"repo"`
	Ticket int     `json:"ticket"`
	Set    int     `json:"set,omitempty"`
	Clear  bool    `json:"clear,omitempty"`
	DryRun bool    `json:"dry_run,omitempty"`
	Apply  bool    `json:"apply,omitempty"`
}

type TicketParentReport struct {
	SchemaVersion string               `json:"schema_version"`
	Repo          string               `json:"repo"`
	Ticket        int                  `json:"ticket"`
	CurrentParent *TicketParentIssue   `json:"current_parent,omitempty"`
	TargetParent  *TicketParentIssue   `json:"target_parent,omitempty"`
	DryRun        bool                 `json:"dry_run,omitempty"`
	Apply         bool                 `json:"apply,omitempty"`
	Actions       []TicketParentAction `json:"actions,omitempty"`
	NextStep      string               `json:"next_step,omitempty"`
	Approval      *ApprovalEvidence    `json:"approval,omitempty"`
}

type TicketParentIssue struct {
	Number int    `json:"number"`
	Title  string `json:"title,omitempty"`
	State  string `json:"state,omitempty"`
	URL    string `json:"url,omitempty"`
}

type TicketParentAction struct {
	Action string `json:"action"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type githubIssueRaw struct {
	ID      int64  `json:"id"`
	Number  int    `json:"number"`
	Title   string `json:"title"`
	State   string `json:"state"`
	HTMLURL string `json:"html_url"`
	Labels  []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Milestone *struct {
		Title string `json:"title"`
	} `json:"milestone"`
	PullRequest *struct {
		URL string `json:"url"`
	} `json:"pull_request"`
}

func BuildTicketParentReport(input TicketParentInput, runner CommandRunner) (TicketParentReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if input.Ticket <= 0 {
		return TicketParentReport{}, fmt.Errorf("ticket must be > 0")
	}
	if input.Set > 0 && input.Clear {
		return TicketParentReport{}, fmt.Errorf("use either --set or --clear, not both")
	}
	if (input.Set > 0 || input.Clear) && input.DryRun == input.Apply {
		return TicketParentReport{}, fmt.Errorf("exactly one of --dry-run or --apply is required")
	}
	if input.Set == input.Ticket {
		return TicketParentReport{}, fmt.Errorf("ticket cannot be its own parent")
	}

	report := TicketParentReport{
		SchemaVersion: TicketParentReportSchemaVersion,
		Repo:          input.Repo.FullName(),
		Ticket:        input.Ticket,
		DryRun:        input.DryRun,
		Apply:         input.Apply,
		NextStep:      fmt.Sprintf("gira ticket parent %d --repo %s", input.Ticket, input.Repo.FullName()),
	}
	child, err := fetchGitHubIssue(input.Repo, input.Ticket, runner)
	if err != nil {
		return report, fmt.Errorf("fetch ticket: %w", err)
	}
	if child.PullRequest != nil {
		return report, fmt.Errorf("#%d is a pull request, not an issue", input.Ticket)
	}
	current, currentErr := getGitHubParentIssue(input.Repo, input.Ticket, runner)
	if currentErr != nil && !isGitHubParentMissing(currentErr) {
		return report, fmt.Errorf("fetch parent issue: %w", currentErr)
	}
	if current != nil {
		report.CurrentParent = ticketParentIssueFromRaw(*current)
	}

	switch {
	case input.Set > 0:
		parent, err := fetchGitHubIssue(input.Repo, input.Set, runner)
		if err != nil {
			return report, fmt.Errorf("fetch target parent: %w", err)
		}
		if parent.PullRequest != nil {
			return report, fmt.Errorf("#%d is a pull request, not an issue", input.Set)
		}
		report.TargetParent = ticketParentIssueFromRaw(parent)
		status := plannedOrAppliedStatus(input.DryRun)
		report.Actions = append(report.Actions, TicketParentAction{Action: "parent:set", Status: status, Detail: fmt.Sprintf("link #%d under #%d", input.Ticket, input.Set)})
		report.NextStep = fmt.Sprintf("gira ticket parent %d --repo %s", input.Ticket, input.Repo.FullName())
		if input.DryRun {
			report.Approval = TicketParentApprovalEvidence(report, "set")
			return report, nil
		}
		replaceParent := report.CurrentParent != nil && report.CurrentParent.Number != input.Set
		if err := addGitHubSubIssue(input.Repo, input.Set, child.ID, replaceParent, runner); err != nil {
			return report, fmt.Errorf("set parent issue: %w", err)
		}
	case input.Clear:
		status := plannedOrAppliedStatus(input.DryRun)
		if report.CurrentParent == nil {
			report.Actions = append(report.Actions, TicketParentAction{Action: "parent:clear", Status: "skipped", Detail: "ticket has no native parent"})
			report.NextStep = "no parent link present"
			return report, nil
		}
		report.Actions = append(report.Actions, TicketParentAction{Action: "parent:clear", Status: status, Detail: fmt.Sprintf("remove #%d from #%d", input.Ticket, report.CurrentParent.Number)})
		report.NextStep = fmt.Sprintf("gira ticket parent %d --repo %s", input.Ticket, input.Repo.FullName())
		if input.DryRun {
			report.Approval = TicketParentApprovalEvidence(report, "clear")
			return report, nil
		}
		if err := removeGitHubSubIssue(input.Repo, report.CurrentParent.Number, child.ID, runner); err != nil {
			return report, fmt.Errorf("clear parent issue: %w", err)
		}
	default:
		if report.CurrentParent == nil {
			report.NextStep = fmt.Sprintf("gira ticket parent %d --repo %s --set PARENT --dry-run", input.Ticket, input.Repo.FullName())
		} else {
			report.NextStep = fmt.Sprintf("gira epic status --repo %s --ticket %d", input.Repo.FullName(), report.CurrentParent.Number)
		}
	}
	return report, nil
}

func TicketParentApprovalEvidence(report TicketParentReport, mode string) *ApprovalEvidence {
	dryRunCommand := ticketParentApprovalCommand(report, mode, "--dry-run")
	applyCommand := ticketParentApprovalCommand(report, mode, "--apply")
	return &ApprovalEvidence{
		SchemaVersion:         ApprovalPlanSchemaVersion,
		Capability:            AdapterCapabilityApplyMutation,
		CanonicalCommand:      "gira ticket parent",
		DryRunCommand:         dryRunCommand,
		ApplyCommand:          applyCommand,
		Repo:                  report.Repo,
		Issue:                 report.Ticket,
		OutputSchema:          TicketParentReportSchemaVersion,
		PlannedActions:        ticketParentApprovalActions(report),
		Blockers:              []string{},
		Warnings:              []string{},
		PostApplyVerification: fmt.Sprintf("gira ticket parent %d --repo %s --json", report.Ticket, report.Repo),
	}
}

func ticketParentApprovalCommand(report TicketParentReport, mode string, applyMode string) string {
	args := []string{
		"gira ticket parent",
		strconv.Itoa(report.Ticket),
		"--repo", report.Repo,
	}
	switch mode {
	case "set":
		if report.TargetParent != nil {
			args = append(args, "--set", strconv.Itoa(report.TargetParent.Number))
		}
	case "clear":
		args = append(args, "--clear")
	}
	args = append(args, applyMode)
	return strings.Join(args, " ")
}

func ticketParentApprovalActions(report TicketParentReport) []ApprovalPlannedAction {
	actions := make([]ApprovalPlannedAction, 0, len(report.Actions))
	for _, action := range report.Actions {
		actions = append(actions, ApprovalPlannedAction{Action: action.Action, Target: fmt.Sprintf("#%d", report.Ticket), Detail: action.Detail})
	}
	return actions
}

func fetchGitHubIssue(repo RepoRef, issueNumber int, runner CommandRunner) (githubIssueRaw, error) {
	if issueNumber <= 0 {
		return githubIssueRaw{}, fmt.Errorf("issue number must be > 0")
	}
	out, err := runner.Run("gh", "api", "repos/"+repo.FullName()+"/issues/"+strconv.Itoa(issueNumber), "-H", "Accept: application/vnd.github+json")
	if err != nil {
		return githubIssueRaw{}, err
	}
	var issue githubIssueRaw
	if err := json.Unmarshal(out, &issue); err != nil {
		return githubIssueRaw{}, fmt.Errorf("parse issue JSON: %w", err)
	}
	return issue, nil
}

func getGitHubParentIssue(repo RepoRef, issueNumber int, runner CommandRunner) (*githubIssueRaw, error) {
	out, err := runner.Run("gh", "api", "repos/"+repo.FullName()+"/issues/"+strconv.Itoa(issueNumber)+"/parent", "-H", "Accept: application/vnd.github+json", "-H", "X-GitHub-Api-Version: 2026-03-10")
	if err != nil {
		return nil, err
	}
	var issue githubIssueRaw
	if err := json.Unmarshal(out, &issue); err != nil {
		return nil, fmt.Errorf("parse parent issue JSON: %w", err)
	}
	return &issue, nil
}

func listGitHubSubIssues(repo RepoRef, issueNumber int, runner CommandRunner) ([]githubIssueRaw, error) {
	out, err := runner.Run("gh", "api", "repos/"+repo.FullName()+"/issues/"+strconv.Itoa(issueNumber)+"/sub_issues", "-X", "GET", "-H", "Accept: application/vnd.github+json", "-H", "X-GitHub-Api-Version: 2026-03-10", "-f", "per_page=100")
	if err != nil {
		return nil, err
	}
	var issues []githubIssueRaw
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parse sub-issues JSON: %w", err)
	}
	return issues, nil
}

func addGitHubSubIssue(repo RepoRef, parentNumber int, childID int64, replaceParent bool, runner CommandRunner) error {
	args := []string{
		"api", "repos/" + repo.FullName() + "/issues/" + strconv.Itoa(parentNumber) + "/sub_issues",
		"-X", "POST",
		"-H", "Accept: application/vnd.github+json",
		"-H", "X-GitHub-Api-Version: 2026-03-10",
		"-F", fmt.Sprintf("sub_issue_id=%d", childID),
	}
	if replaceParent {
		args = append(args, "-F", "replace_parent=true")
	}
	_, err := runner.Run("gh", args...)
	return err
}

func removeGitHubSubIssue(repo RepoRef, parentNumber int, childID int64, runner CommandRunner) error {
	_, err := runner.Run(
		"gh", "api", "repos/"+repo.FullName()+"/issues/"+strconv.Itoa(parentNumber)+"/sub_issue",
		"-X", "DELETE",
		"-H", "Accept: application/vnd.github+json",
		"-H", "X-GitHub-Api-Version: 2026-03-10",
		"-F", fmt.Sprintf("sub_issue_id=%d", childID),
	)
	return err
}

func ticketParentIssueFromRaw(issue githubIssueRaw) *TicketParentIssue {
	return &TicketParentIssue{Number: issue.Number, Title: issue.Title, State: issue.State, URL: issue.HTMLURL}
}

func isGitHubParentMissing(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "no parent issue found") || strings.Contains(text, "http 404")
}

func FormatTicketParent(report TicketParentReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ticket parent: #%d", report.Ticket)
	if report.CurrentParent != nil {
		fmt.Fprintf(&b, " parent=#%d", report.CurrentParent.Number)
	} else {
		b.WriteString(" parent=none")
	}
	b.WriteString("\n")
	if report.TargetParent != nil {
		fmt.Fprintf(&b, "target parent: #%d %s\n", report.TargetParent.Number, report.TargetParent.Title)
	}
	if len(report.Actions) > 0 {
		b.WriteString("actions:\n")
		for _, action := range report.Actions {
			fmt.Fprintf(&b, "  %s:%s", action.Action, action.Status)
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
