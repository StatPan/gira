package gira

import (
	"fmt"
	"strings"
)

type WorkspaceValidateReport struct {
	Command   string                  `json:"command"`
	Scope     string                  `json:"scope"`
	Workspace WorkspaceSummary        `json:"workspace"`
	InboxRepo string                  `json:"inbox_repo"`
	Items     []WorkspaceValidateItem `json:"items"`
	Counts    WorkspaceValidateCounts `json:"counts"`
	Warnings  []string                `json:"warnings,omitempty"`
	NextSteps []string                `json:"next_steps"`
}

type WorkspaceValidateItem struct {
	Ticket      int      `json:"ticket"`
	Title       string   `json:"title"`
	State       string   `json:"state"`
	Scope       string   `json:"scope"`
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
	OutOfScope   int `json:"out_of_scope"`
	Done         int `json:"done"`
}

func BuildWorkspaceValidateReport(config WorkspaceConfigResolved, client WorkspaceClient) (WorkspaceValidateReport, error) {
	report := WorkspaceValidateReport{
		Command:   "workspace validate",
		Scope:     "inbox-routing",
		Workspace: WorkspaceSummary{Name: config.Name, Owner: config.Owner},
		InboxRepo: config.InboxRepo.FullName(),
	}
	if workspaceContainsRepo(config.Repos, config.InboxRepo) {
		report.Scope = "repo-execution"
		report.Warnings = append(report.Warnings, "workspace validate checks inbox routing contracts; this inbox repo is also an execution repo, so use workspace status for repo execution readiness")
		report.NextSteps = []string{"gira workspace status --config .gira/config.yaml"}
		return report, nil
	}
	tickets, err := client.FetchInboxTickets(config.InboxRepo)
	if err != nil {
		return WorkspaceValidateReport{}, err
	}
	parsed, diagnostics := ParsePortfolioTickets(tickets, config.Repos)
	invalid := portfolioInvalidTickets(diagnostics)
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
		case "out-of-scope":
			report.Counts.OutOfScope++
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
	item := WorkspaceValidateItem{Ticket: ticket.Number, Title: ticket.Title, State: ticket.State, Scope: "inbox-routing", TargetRepos: append([]string(nil), ticket.TargetRepos...), ChildIssues: append([]string(nil), ticket.ChildIssues...)}
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
	fmt.Fprintf(&b, "workspace validate: scope=%s total=%d routeable=%d routed=%d needs_routing=%d blocked=%d out_of_scope=%d done=%d\n", report.Scope, report.Counts.Total, report.Counts.Routeable, report.Counts.Routed, report.Counts.NeedsRouting, report.Counts.Blocked, report.Counts.OutOfScope, report.Counts.Done)
	for _, warning := range report.Warnings {
		fmt.Fprintf(&b, "warning: %s\n", warning)
	}
	for _, item := range report.Items {
		fmt.Fprintf(&b, "  #%d %s scope=%s status=%s (%s)\n", item.Ticket, item.Title, item.Scope, item.Status, item.Reason)
	}
	if len(report.NextSteps) > 0 {
		fmt.Fprintf(&b, "next step: %s\n", report.NextSteps[0])
	}
	return b.String()
}
