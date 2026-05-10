package gira

import (
	"fmt"
	"strings"
)

type TicketViewReport struct {
	Command string           `json:"command"`
	Repo    string           `json:"repo"`
	Ticket  int              `json:"ticket"`
	Status  WorkStatusResult `json:"status"`
	Summary []TicketViewRow  `json:"summary"`
}

type TicketViewRow struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func BuildTicketViewReport(repo RepoRef, ticket int, runner CommandRunner) (TicketViewReport, error) {
	if ticket <= 0 {
		return TicketViewReport{}, fmt.Errorf("ticket must be > 0")
	}
	status, err := GetWorkStatus(repo, ticket, runner)
	report := TicketViewReport{
		Command: "ticket view",
		Repo:    repo.FullName(),
		Ticket:  ticket,
		Status:  status,
	}
	if err != nil {
		return report, err
	}
	status.NextStep = ticketLifecycleNextStep(status)
	report.Status = status
	report.Summary = ticketViewRows(status)
	return report, nil
}

func ticketViewRows(status WorkStatusResult) []TicketViewRow {
	pr := "none"
	if status.PRNumber > 0 {
		pr = fmt.Sprintf("#%d", status.PRNumber)
		if strings.TrimSpace(status.PRState) != "" {
			pr += " " + status.PRState
		}
	}
	blockers := strings.Join(status.Blockers, ",")
	if blockers == "" {
		blockers = "none"
	}
	next := ticketLifecycleNextStep(status)
	return []TicketViewRow{
		{Name: "title", Value: status.Title},
		{Name: "state", Value: status.State},
		{Name: "status", Value: status.Status},
		{Name: "linked_pr", Value: pr},
		{Name: "blockers", Value: blockers},
		{Name: "next_action", Value: status.NextAction},
		{Name: "next_step", Value: next},
	}
}

func FormatTicketView(report TicketViewReport) string {
	status := report.Status
	blockers := strings.Join(status.Blockers, ",")
	if blockers == "" {
		blockers = "none"
	}
	pr := "none"
	if status.PRNumber > 0 {
		pr = fmt.Sprintf("#%d", status.PRNumber)
		if strings.TrimSpace(status.PRState) != "" {
			pr += " " + status.PRState
		}
	}
	next := ticketLifecycleNextStep(status)
	var b strings.Builder
	fmt.Fprintf(&b, "ticket view: #%d %s\n", report.Ticket, status.Title)
	fmt.Fprintf(&b, "repo: %s\n", report.Repo)
	fmt.Fprintf(&b, "issue: state=%s status=%s\n", status.State, status.Status)
	fmt.Fprintf(&b, "linked pr: %s\n", pr)
	fmt.Fprintf(&b, "blockers: %s\n", blockers)
	fmt.Fprintf(&b, "next action: %s\n", status.NextAction)
	fmt.Fprintf(&b, "next step: %s\n", next)
	return b.String()
}

func ticketLifecycleNextStep(status WorkStatusResult) string {
	next := strings.TrimSpace(status.NextStep)
	if next == "" {
		next = workStatusNextStep(status)
	}
	next = strings.ReplaceAll(next, "gira work status", "gira ticket status")
	next = strings.ReplaceAll(next, "gira work pr", "gira ticket pr")
	next = strings.ReplaceAll(next, "gira work start", "gira ticket start")
	next = strings.ReplaceAll(next, "--issue", "--ticket")
	return strings.Join(strings.Fields(next), " ")
}
