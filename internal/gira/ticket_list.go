package gira

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type TicketListOptions struct {
	Repo      RepoRef  `json:"repo"`
	State     string   `json:"state"`
	Labels    []string `json:"labels,omitempty"`
	Assignee  string   `json:"assignee,omitempty"`
	Milestone string   `json:"milestone,omitempty"`
	Limit     int      `json:"limit"`
}

type TicketListReport struct {
	Command string            `json:"command"`
	Repo    string            `json:"repo"`
	Filters TicketListFilters `json:"filters"`
	Tickets []TicketListItem  `json:"tickets"`
	Counts  TicketListCounts  `json:"counts"`
}

type TicketListFilters struct {
	State     string   `json:"state"`
	Labels    []string `json:"labels,omitempty"`
	Assignee  string   `json:"assignee,omitempty"`
	Milestone string   `json:"milestone,omitempty"`
	Limit     int      `json:"limit"`
}

type TicketListCounts struct {
	Tickets int `json:"tickets"`
}

type TicketListItem struct {
	Number    int      `json:"number"`
	State     string   `json:"state"`
	Title     string   `json:"title"`
	Status    string   `json:"status,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	Assignees []string `json:"assignees,omitempty"`
	Milestone string   `json:"milestone,omitempty"`
	URL       string   `json:"url,omitempty"`
}

func BuildTicketListReport(options TicketListOptions, runner CommandRunner) (TicketListReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	state := strings.ToLower(strings.TrimSpace(options.State))
	if state == "" {
		state = "open"
	}
	if state != "open" && state != "closed" && state != "all" {
		return TicketListReport{}, fmt.Errorf("--state must be one of open, closed, all")
	}
	limit := options.Limit
	if limit == 0 {
		limit = 30
	}
	if limit < 0 {
		return TicketListReport{}, fmt.Errorf("--limit must be greater than 0")
	}
	labels := normalizeTicketListLabels(options.Labels)
	args := []string{
		"issue",
		"list",
		"--repo",
		options.Repo.FullName(),
		"--state",
		state,
		"--limit",
		fmt.Sprintf("%d", limit),
		"--json",
		"number,title,state,labels,assignees,milestone,url",
	}
	for _, label := range labels {
		args = append(args, "--label", label)
	}
	assignee := strings.TrimSpace(options.Assignee)
	if assignee != "" {
		args = append(args, "--assignee", assignee)
	}
	milestone := strings.TrimSpace(options.Milestone)
	if milestone != "" {
		args = append(args, "--milestone", milestone)
	}
	output, err := runner.Run("gh", args...)
	if err != nil {
		return TicketListReport{}, err
	}
	tickets, err := normalizeTicketListRows(output)
	if err != nil {
		return TicketListReport{}, err
	}
	report := TicketListReport{
		Command: "ticket list",
		Repo:    options.Repo.FullName(),
		Filters: TicketListFilters{
			State:     state,
			Labels:    labels,
			Assignee:  assignee,
			Milestone: milestone,
			Limit:     limit,
		},
		Tickets: tickets,
		Counts:  TicketListCounts{Tickets: len(tickets)},
	}
	return report, nil
}

func normalizeTicketListLabels(values []string) []string {
	seen := map[string]struct{}{}
	labels := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			label := strings.TrimSpace(part)
			if label == "" {
				continue
			}
			if _, ok := seen[label]; ok {
				continue
			}
			seen[label] = struct{}{}
			labels = append(labels, label)
		}
	}
	return labels
}

func normalizeTicketListRows(output []byte) ([]TicketListItem, error) {
	var rows []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		State  string `json:"state"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
		Milestone *struct {
			Title string `json:"title"`
		} `json:"milestone"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(output, &rows); err != nil {
		return nil, fmt.Errorf("parse gh issue list JSON: %w", err)
	}
	tickets := make([]TicketListItem, 0, len(rows))
	for _, row := range rows {
		labels := make([]string, 0, len(row.Labels))
		for _, label := range row.Labels {
			if strings.TrimSpace(label.Name) != "" {
				labels = append(labels, label.Name)
			}
		}
		sort.Strings(labels)
		assignees := make([]string, 0, len(row.Assignees))
		for _, assignee := range row.Assignees {
			if strings.TrimSpace(assignee.Login) != "" {
				assignees = append(assignees, assignee.Login)
			}
		}
		sort.Strings(assignees)
		milestone := ""
		if row.Milestone != nil {
			milestone = row.Milestone.Title
		}
		tickets = append(tickets, TicketListItem{
			Number:    row.Number,
			State:     strings.ToLower(row.State),
			Title:     row.Title,
			Status:    statusFromLabels(labels),
			Labels:    ticketListKeyLabels(labels),
			Assignees: assignees,
			Milestone: milestone,
			URL:       row.URL,
		})
	}
	return tickets, nil
}

func ticketListKeyLabels(labels []string) []string {
	keyLabels := make([]string, 0, len(labels))
	for _, label := range labels {
		switch {
		case strings.HasPrefix(label, "status:"),
			strings.HasPrefix(label, "type:"),
			strings.HasPrefix(label, "priority:"),
			strings.HasPrefix(label, "area:"),
			strings.HasPrefix(label, "agent:"),
			label == "blocked":
			keyLabels = append(keyLabels, label)
		}
	}
	return keyLabels
}

func FormatTicketList(report TicketListReport) string {
	var b strings.Builder
	command := report.Command
	if command == "" {
		command = "ticket list"
	}
	fmt.Fprintf(&b, "%s: %s state=%s count=%d\n", command, report.Repo, report.Filters.State, report.Counts.Tickets)
	if len(report.Tickets) == 0 {
		if command == "epic list" {
			b.WriteString("epics: none\n")
			return b.String()
		}
		b.WriteString("tickets: none\n")
		return b.String()
	}
	for _, ticket := range report.Tickets {
		meta := make([]string, 0, 4)
		if len(ticket.Labels) > 0 {
			meta = append(meta, "labels="+strings.Join(ticket.Labels, ","))
		}
		if len(ticket.Assignees) > 0 {
			meta = append(meta, "assignees="+strings.Join(ticket.Assignees, ","))
		}
		if ticket.Milestone != "" {
			meta = append(meta, "milestone="+ticket.Milestone)
		}
		suffix := ""
		if len(meta) > 0 {
			suffix = " " + strings.Join(meta, " ")
		}
		fmt.Fprintf(&b, "  #%d %-6s %s%s\n", ticket.Number, ticket.State, ticket.Title, suffix)
	}
	return b.String()
}
