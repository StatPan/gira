package gira

import (
	"fmt"
	"os"
	"strings"
)

type TicketNewInput struct {
	Repo       RepoRef  `json:"repo"`
	Title      string   `json:"title"`
	Goal       string   `json:"goal,omitempty"`
	Scope      string   `json:"scope,omitempty"`
	Acceptance []string `json:"acceptance,omitempty"`
	Notes      string   `json:"notes,omitempty"`
	Body       string   `json:"body,omitempty"`
	Type       string   `json:"type"`
	Priority   string   `json:"priority,omitempty"`
	Milestone  string   `json:"milestone,omitempty"`
	Labels     []string `json:"labels,omitempty"`
	BodyFile   string   `json:"body_file,omitempty"`
	Start      bool     `json:"start"`
	DryRun     bool     `json:"dry_run"`
}

type TicketCreatedIssue struct {
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	URL    string `json:"url"`
}

type TicketNewReport struct {
	Repo        string             `json:"repo"`
	Title       string             `json:"title"`
	DryRun      bool               `json:"dry_run"`
	Start       bool               `json:"start"`
	Labels      []string           `json:"labels"`
	Milestone   string             `json:"milestone,omitempty"`
	Body        string             `json:"body"`
	Created     TicketCreatedIssue `json:"created,omitempty"`
	StartResult WorkStartResult    `json:"start_result,omitempty"`
	NextStep    string             `json:"next_step"`
}

func BuildTicketNewReport(input TicketNewInput, runner CommandRunner) (TicketNewReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return TicketNewReport{}, fmt.Errorf("ticket title is required")
	}
	ticketType := strings.TrimSpace(input.Type)
	if ticketType == "" {
		ticketType = "task"
	}
	if !validTicketType(ticketType) {
		return TicketNewReport{}, fmt.Errorf("--type must be one of epic, story, task, bug, spike, chore")
	}
	priority := strings.TrimSpace(input.Priority)
	if priority != "" && !validTicketPriority(priority) {
		return TicketNewReport{}, fmt.Errorf("--priority must be one of p0, p1, p2, p3")
	}
	body, err := ticketNewBody(input)
	if err != nil {
		return TicketNewReport{}, err
	}
	labels := ticketNewLabels(ticketType, priority, input.Labels)
	report := TicketNewReport{
		Repo:      input.Repo.FullName(),
		Title:     input.Title,
		DryRun:    input.DryRun,
		Start:     input.Start,
		Labels:    labels,
		Milestone: strings.TrimSpace(input.Milestone),
		Body:      body,
		NextStep:  "gira ticket new --apply",
	}
	if input.DryRun {
		return report, nil
	}
	created, err := createRepoTicket(input.Repo, input.Title, body, labels, report.Milestone, runner)
	if err != nil {
		return report, err
	}
	report.Created = created
	report.NextStep = fmt.Sprintf("gira ticket start %d --apply", created.Number)
	if input.Start {
		start, err := StartWork(input.Repo, created.Number, false, runner)
		report.StartResult = start
		if err != nil {
			return report, err
		}
		report.NextStep = "gira ticket pr --dry-run"
	}
	return report, nil
}

func ticketNewBody(input TicketNewInput) (string, error) {
	if strings.TrimSpace(input.Body) != "" && strings.TrimSpace(input.BodyFile) != "" {
		return "", fmt.Errorf("use either --body or --body-file, not both")
	}
	if strings.TrimSpace(input.Body) != "" {
		return strings.TrimSpace(input.Body), nil
	}
	if strings.TrimSpace(input.BodyFile) != "" {
		content, err := os.ReadFile(input.BodyFile)
		if err != nil {
			return "", fmt.Errorf("read --body-file: %w", err)
		}
		body := strings.TrimSpace(string(content))
		if body == "" {
			return "", fmt.Errorf("--body-file is empty")
		}
		return body, nil
	}
	goal := strings.TrimSpace(input.Goal)
	if goal == "" {
		goal = input.Title
	}
	scope := noResponse(input.Scope)
	notes := noResponse(input.Notes)
	acceptance := input.Acceptance
	var b strings.Builder
	fmt.Fprintf(&b, "## Goal\n%s\n\n", goal)
	fmt.Fprintf(&b, "## Scope\n%s\n\n", scope)
	b.WriteString("## Acceptance Criteria\n")
	if len(acceptance) == 0 {
		b.WriteString("_No response_\n\n")
	} else {
		for _, item := range acceptance {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				fmt.Fprintf(&b, "- %s\n", trimmed)
			}
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "## Notes\n%s\n", notes)
	return b.String(), nil
}

func createRepoTicket(repo RepoRef, title string, body string, labels []string, milestone string, runner CommandRunner) (TicketCreatedIssue, error) {
	args := []string{"issue", "create", "--repo", repo.FullName(), "--title", title, "--body", body}
	for _, label := range labels {
		args = append(args, "--label", label)
	}
	if strings.TrimSpace(milestone) != "" {
		args = append(args, "--milestone", milestone)
	}
	out, err := runner.Run("gh", args...)
	if err != nil {
		return TicketCreatedIssue{}, err
	}
	url := strings.TrimSpace(string(out))
	number := extractPRNumber(url)
	return TicketCreatedIssue{Repo: repo.FullName(), Number: number, URL: url}, nil
}

func ticketNewLabels(ticketType string, priority string, extra []string) []string {
	labels := []string{"type:" + ticketType, "status:ready"}
	if priority != "" {
		labels = append(labels, "priority:"+priority)
	}
	seen := map[string]struct{}{}
	deduped := make([]string, 0, len(labels)+len(extra))
	for _, label := range append(labels, extra...) {
		trimmed := strings.TrimSpace(label)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		deduped = append(deduped, trimmed)
	}
	return deduped
}

func validTicketType(value string) bool {
	switch value {
	case "epic", "story", "task", "bug", "spike", "chore":
		return true
	default:
		return false
	}
}

func validTicketPriority(value string) bool {
	switch value {
	case "p0", "p1", "p2", "p3":
		return true
	default:
		return false
	}
}

func noResponse(value string) string {
	if strings.TrimSpace(value) == "" {
		return "_No response_"
	}
	return strings.TrimSpace(value)
}

func FormatTicketNew(report TicketNewReport) string {
	if report.DryRun {
		var b strings.Builder
		fmt.Fprintf(&b, "ticket new: dry-run %s\n", report.Title)
		fmt.Fprintf(&b, "repo: %s\n", report.Repo)
		fmt.Fprintf(&b, "labels: %s\n", strings.Join(report.Labels, ","))
		if strings.TrimSpace(report.Milestone) != "" {
			fmt.Fprintf(&b, "milestone: %s\n", report.Milestone)
		}
		if report.Start {
			b.WriteString("after create: start ticket\n")
		}
		fmt.Fprintf(&b, "body:\n%s\nnext step: %s\n", strings.TrimSpace(report.Body), report.NextStep)
		return b.String()
	}
	return fmt.Sprintf("ticket new: ticket #%d %s\nnext step: %s\n", report.Created.Number, report.Title, report.NextStep)
}
