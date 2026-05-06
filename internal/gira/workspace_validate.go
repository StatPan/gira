package gira

import (
	"fmt"
	"strings"
)

type WorkspaceValidateReport struct {
	Command   string                  `json:"command"`
	Workspace WorkspaceSummary        `json:"workspace"`
	InboxRepo string                  `json:"inbox_repo"`
	Items     []WorkspaceValidateItem `json:"items"`
	Counts    WorkspaceValidateCounts `json:"counts"`
	NextSteps []string                `json:"next_steps"`
}

type WorkspaceValidateItem struct {
	Ticket      int      `json:"ticket"`
	Title       string   `json:"title"`
	State       string   `json:"state"`
	Status      string   `json:"status"`
	TargetRepos []string `json:"target_repos,omitempty"`
	ChildIssues []string `json:"child_issues,omitempty"`
	Reason      string   `json:"reason"`
	NextStep    string   `json:"next_step,omitempty"`
}

type WorkspaceValidateCounts struct {
	Total        int `json:"total"`
	NeedsRouting int `json:"needs_routing"`
	Routeable    int `json:"routeable"`
	Routed       int `json:"routed"`
	Blocked      int `json:"blocked"`
	Done         int `json:"done"`
}

func BuildWorkspaceValidateReport(config WorkspaceConfigResolved, client WorkspaceClient) (WorkspaceValidateReport, error) {
	tickets, err := client.FetchInboxTickets(config.InboxRepo)
	if err != nil {
		return WorkspaceValidateReport{}, err
	}
	parsed, diagnostics := ParsePortfolioTickets(tickets, config.Repos)
	invalid := portfolioInvalidTickets(diagnostics)
	report := WorkspaceValidateReport{
		Command:   "workspace validate",
		Workspace: WorkspaceSummary{Name: config.Name, Owner: config.Owner},
		InboxRepo: config.InboxRepo.FullName(),
	}
	for _, ticket := range parsed {
		item := workspaceValidateItem(config, ticket, invalid)
		report.Items = append(report.Items, item)
		report.Counts.Total++
		switch item.Status {
		case "needs-routing":
			report.Counts.NeedsRouting++
		case "routeable":
			report.Counts.Routeable++
			if item.NextStep != "" {
				report.NextSteps = append(report.NextSteps, item.NextStep)
			}
		case "routed":
			report.Counts.Routed++
		case "blocked":
			report.Counts.Blocked++
		case "done":
			report.Counts.Done++
		}
	}
	if len(report.NextSteps) == 0 {
		report.NextSteps = []string{"gira workspace status --config .gira/config.yaml"}
	}
	return report, nil
}

func workspaceValidateItem(config WorkspaceConfigResolved, ticket PortfolioTicket, invalid map[int]struct{}) WorkspaceValidateItem {
	item := WorkspaceValidateItem{Ticket: ticket.Number, Title: ticket.Title, State: ticket.State, TargetRepos: append([]string(nil), ticket.TargetRepos...), ChildIssues: append([]string(nil), ticket.ChildIssues...)}
	if strings.EqualFold(ticket.State, "closed") {
		item.Status = "done"
		item.Reason = "ticket is closed"
		return item
	}
	if _, ok := invalid[ticket.Number]; ok {
		item.Status = "blocked"
		item.Reason = "ticket body does not match workspace routing contract"
		return item
	}
	if len(ticket.ChildIssues) > 0 {
		item.Status = "routed"
		item.Reason = "ticket already links execution issue"
		return item
	}
	if ticket.Routing != "single_repo" && ticket.Routing != "multi_repo" {
		item.Status = "needs-routing"
		item.Reason = "routing is not set to single_repo or multi_repo"
		return item
	}
	for _, repoValue := range ticket.TargetRepos {
		repo, err := ParseRepoRef(repoValue)
		if err != nil || !workspaceContainsRepo(config.Repos, repo) {
			item.Status = "blocked"
			item.Reason = "target repo is not in workspace.repos"
			return item
		}
	}
	item.Status = "routeable"
	item.Reason = "ticket can be routed into execution repo"
	if len(ticket.TargetRepos) > 0 {
		item.NextStep = fmt.Sprintf("gira workspace ticket route --ticket %d --repo %s --dry-run", ticket.Number, ticket.TargetRepos[0])
	}
	return item
}

func FormatWorkspaceValidateReport(report WorkspaceValidateReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "workspace validate: total=%d routeable=%d routed=%d needs_routing=%d blocked=%d done=%d\n", report.Counts.Total, report.Counts.Routeable, report.Counts.Routed, report.Counts.NeedsRouting, report.Counts.Blocked, report.Counts.Done)
	for _, item := range report.Items {
		fmt.Fprintf(&b, "  #%d %s status=%s (%s)\n", item.Ticket, item.Title, item.Status, item.Reason)
	}
	if len(report.NextSteps) > 0 {
		fmt.Fprintf(&b, "next step: %s\n", report.NextSteps[0])
	}
	return b.String()
}
