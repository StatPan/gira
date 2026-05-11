package gira

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type TicketSupersedeInput struct {
	Repo             RepoRef  `json:"-"`
	Ticket           int      `json:"ticket"`
	ReplacementTitle string   `json:"replacement_title"`
	Body             string   `json:"body,omitempty"`
	Labels           []string `json:"labels,omitempty"`
	Milestone        string   `json:"milestone,omitempty"`
	CloseDraftPR     bool     `json:"close_draft_pr"`
	DryRun           bool     `json:"dry_run"`
	Apply            bool     `json:"apply"`
}

type TicketSupersedeReport struct {
	Command     string                  `json:"command"`
	Repo        string                  `json:"repo"`
	DryRun      bool                    `json:"dry_run"`
	Original    TicketSupersedeIssue    `json:"original"`
	Replacement TicketSupersedeIssue    `json:"replacement"`
	DraftPR     TicketSupersedeDraftPR  `json:"draft_pr,omitempty"`
	Actions     []TicketSupersedeAction `json:"actions"`
	NextStep    string                  `json:"next_step"`
}

type TicketSupersedeIssue struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	State     string   `json:"state,omitempty"`
	URL       string   `json:"url,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	Milestone string   `json:"milestone,omitempty"`
	Body      string   `json:"body,omitempty"`
}

type TicketSupersedeDraftPR struct {
	Number int    `json:"number,omitempty"`
	URL    string `json:"url,omitempty"`
	State  string `json:"state,omitempty"`
	Draft  bool   `json:"draft"`
	Action string `json:"action,omitempty"`
}

type TicketSupersedeAction struct {
	Action string `json:"action"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type ticketSupersedeOriginal struct {
	Number    int
	Title     string
	State     string
	URL       string
	Labels    []string
	Milestone string
}

func BuildTicketSupersedeReport(input TicketSupersedeInput, runner CommandRunner) (TicketSupersedeReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if input.Ticket <= 0 {
		return TicketSupersedeReport{}, fmt.Errorf("ticket must be > 0")
	}
	if input.DryRun == input.Apply {
		return TicketSupersedeReport{}, fmt.Errorf("exactly one of dry_run/apply is required")
	}
	input.ReplacementTitle = strings.TrimSpace(input.ReplacementTitle)
	if input.ReplacementTitle == "" {
		return TicketSupersedeReport{}, fmt.Errorf("--replacement-title is required")
	}
	original, err := fetchTicketSupersedeOriginal(input.Repo, input.Ticket, runner)
	report := TicketSupersedeReport{
		Command: "ticket supersede",
		Repo:    input.Repo.FullName(),
		DryRun:  input.DryRun,
		Original: TicketSupersedeIssue{
			Number:    input.Ticket,
			Title:     original.Title,
			State:     original.State,
			URL:       original.URL,
			Labels:    original.Labels,
			Milestone: original.Milestone,
		},
		NextStep: "gira ticket supersede --apply",
	}
	if err != nil {
		return report, err
	}

	replacementLabels := ticketSupersedeReplacementLabels(original.Labels, input.Labels)
	replacementMilestone := strings.TrimSpace(input.Milestone)
	if replacementMilestone == "" {
		replacementMilestone = original.Milestone
	}
	replacementBody := ticketSupersedeReplacementBody(input.Body, input.Ticket)
	report.Replacement = TicketSupersedeIssue{
		Title:     input.ReplacementTitle,
		Labels:    replacementLabels,
		Milestone: replacementMilestone,
		Body:      replacementBody,
	}
	report.Actions = append(report.Actions,
		plannedSupersedeAction("replacement:create", input.DryRun, "create replacement issue"),
		plannedSupersedeAction("original:comment", input.DryRun, "add superseded note to original issue"),
		plannedSupersedeAction("replacement:comment", input.DryRun, "add origin note to replacement issue"),
		plannedSupersedeAction("original:status", input.DryRun, "move original issue to status:done"),
		plannedSupersedeAction("original:close", input.DryRun, "close original issue"),
	)
	prStatus, prErr := DevPRStatus(input.Repo, input.Ticket, runner)
	if prErr == nil && prStatus.PRNumber > 0 {
		report.DraftPR = TicketSupersedeDraftPR{
			Number: prStatus.PRNumber,
			URL:    prStatus.PRURL,
			State:  prStatus.State,
			Draft:  containsString(prStatus.Blockers, "draft"),
			Action: "report",
		}
		if report.DraftPR.Draft && input.CloseDraftPR {
			report.DraftPR.Action = "close"
			report.Actions = append(report.Actions, plannedSupersedeAction("draft_pr:close", input.DryRun, fmt.Sprintf("close draft PR #%d", prStatus.PRNumber)))
		} else {
			report.Actions = append(report.Actions, TicketSupersedeAction{Action: "draft_pr:inspect", Status: "skipped", Detail: "linked PR reported but not closed"})
		}
	} else {
		report.Actions = append(report.Actions, TicketSupersedeAction{Action: "draft_pr:inspect", Status: "skipped", Detail: "no linked PR found"})
	}

	if input.DryRun {
		return report, nil
	}

	created, err := createRepoTicket(input.Repo, input.ReplacementTitle, replacementBody, replacementLabels, replacementMilestone, runner)
	if err != nil {
		return report, fmt.Errorf("create replacement issue: %w", err)
	}
	report.Replacement.Number = created.Number
	report.Replacement.URL = created.URL
	markSupersedeActionApplied(report.Actions, "replacement:create")

	originalNote := ticketSupersedeOriginalNote(input.Ticket, created.Number, input.ReplacementTitle)
	if _, err := runner.Run("gh", "issue", "comment", strconv.Itoa(input.Ticket), "--repo", input.Repo.FullName(), "--body", originalNote); err != nil {
		return report, fmt.Errorf("comment original issue: %w", err)
	}
	markSupersedeActionApplied(report.Actions, "original:comment")

	replacementNote := ticketSupersedeReplacementNote(input.Ticket, original.Title)
	if _, err := runner.Run("gh", "issue", "comment", strconv.Itoa(created.Number), "--repo", input.Repo.FullName(), "--body", replacementNote); err != nil {
		return report, fmt.Errorf("comment replacement issue: %w", err)
	}
	markSupersedeActionApplied(report.Actions, "replacement:comment")

	if err := setIssueStatus(input.Repo, input.Ticket, original.Labels, "status:done", runner); err != nil {
		return report, fmt.Errorf("set original status: %w", err)
	}
	markSupersedeActionApplied(report.Actions, "original:status")

	if _, err := runner.Run("gh", "issue", "close", strconv.Itoa(input.Ticket), "--repo", input.Repo.FullName()); err != nil {
		return report, fmt.Errorf("close original issue: %w", err)
	}
	report.Original.State = "closed"
	markSupersedeActionApplied(report.Actions, "original:close")

	if report.DraftPR.Number > 0 && report.DraftPR.Draft && input.CloseDraftPR {
		body := fmt.Sprintf("Superseded by #%d.", created.Number)
		if _, err := runner.Run("gh", "pr", "close", strconv.Itoa(report.DraftPR.Number), "--repo", input.Repo.FullName(), "--comment", body); err != nil {
			return report, fmt.Errorf("close draft PR: %w", err)
		}
		markSupersedeActionApplied(report.Actions, "draft_pr:close")
	}
	report.NextStep = fmt.Sprintf("gira ticket start %d --apply", created.Number)
	return report, nil
}

func fetchTicketSupersedeOriginal(repo RepoRef, issueNumber int, runner CommandRunner) (ticketSupersedeOriginal, error) {
	out, err := runner.Run("gh", "api", "repos/"+repo.FullName()+"/issues/"+strconv.Itoa(issueNumber))
	if err != nil {
		return ticketSupersedeOriginal{}, fmt.Errorf("fetch issue: %w", err)
	}
	var raw struct {
		Number    int    `json:"number"`
		Title     string `json:"title"`
		State     string `json:"state"`
		URL       string `json:"html_url"`
		Milestone *struct {
			Title string `json:"title"`
		} `json:"milestone"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		PullRequest *any `json:"pull_request"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return ticketSupersedeOriginal{}, fmt.Errorf("parse issue JSON: %w", err)
	}
	if raw.Number != issueNumber {
		return ticketSupersedeOriginal{}, fmt.Errorf("parse issue JSON: expected issue #%d, got #%d", issueNumber, raw.Number)
	}
	if raw.PullRequest != nil {
		return ticketSupersedeOriginal{}, fmt.Errorf("ticket #%d resolves to a pull request", issueNumber)
	}
	labels := make([]string, 0, len(raw.Labels))
	for _, label := range raw.Labels {
		if strings.TrimSpace(label.Name) != "" {
			labels = append(labels, label.Name)
		}
	}
	milestone := ""
	if raw.Milestone != nil {
		milestone = raw.Milestone.Title
	}
	return ticketSupersedeOriginal{Number: raw.Number, Title: raw.Title, State: raw.State, URL: raw.URL, Labels: labels, Milestone: milestone}, nil
}

func ticketSupersedeReplacementLabels(original []string, extra []string) []string {
	labels := make([]string, 0, len(original)+len(extra)+1)
	for _, label := range original {
		trimmed := strings.TrimSpace(label)
		if trimmed == "" || strings.HasPrefix(strings.ToLower(trimmed), "status:") {
			continue
		}
		labels = append(labels, trimmed)
	}
	labels = append(labels, "status:ready")
	labels = append(labels, extra...)
	return dedupeSupersedeLabels(labels)
}

func dedupeSupersedeLabels(labels []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(labels))
	for _, label := range labels {
		trimmed := strings.TrimSpace(label)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func ticketSupersedeReplacementBody(body string, originalIssue int) string {
	body = strings.TrimSpace(body)
	if body == "" {
		body = "_No replacement body provided._"
	}
	return fmt.Sprintf("%s\n\n## Supersedes\n- Supersedes #%d\n", body, originalIssue)
}

func ticketSupersedeOriginalNote(original int, replacement int, title string) string {
	return fmt.Sprintf("## Superseded\n\nThis issue is superseded by #%d: %s.\n", replacement, strings.TrimSpace(title))
}

func ticketSupersedeReplacementNote(original int, title string) string {
	return fmt.Sprintf("## Replacement\n\nThis issue replaces #%d: %s.\n", original, strings.TrimSpace(title))
}

func plannedSupersedeAction(action string, dryRun bool, detail string) TicketSupersedeAction {
	return TicketSupersedeAction{Action: action, Status: "planned", Detail: detail}
}

func markSupersedeActionApplied(actions []TicketSupersedeAction, action string) {
	for i := range actions {
		if actions[i].Action == action {
			actions[i].Status = "applied"
			return
		}
	}
}

func FormatTicketSupersede(report TicketSupersedeReport) string {
	actions := make([]string, 0, len(report.Actions))
	for _, action := range report.Actions {
		actions = append(actions, action.Action+":"+action.Status)
	}
	if len(actions) == 0 {
		actions = append(actions, "none")
	}
	replacement := "(planned)"
	if report.Replacement.Number > 0 {
		replacement = fmt.Sprintf("#%d", report.Replacement.Number)
	}
	return fmt.Sprintf(
		"ticket supersede: original=#%d replacement=%s dry_run=%t actions=%s\nnext step: %s\n",
		report.Original.Number,
		replacement,
		report.DryRun,
		strings.Join(actions, ","),
		report.NextStep,
	)
}
