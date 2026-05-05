package gira

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type PortfolioLowerClient interface {
	SearchLoweredIssues(repo RepoRef, portfolioRepo RepoRef, ticket int) ([]PortfolioLoweredIssue, error)
	CreateExecutionIssue(repo RepoRef, ticket PortfolioTicket, portfolioRepo RepoRef) (PortfolioLoweredIssue, error)
	UpdatePortfolioChildIssues(ticket PortfolioTicket, childIssues []string) error
}

type GHPortfolioLowerClient struct {
	portfolioRepo RepoRef
	runner        CommandRunner
}

func NewGHPortfolioLowerClient(portfolioRepo RepoRef, runner CommandRunner) GHPortfolioLowerClient {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	return GHPortfolioLowerClient{portfolioRepo: portfolioRepo, runner: runner}
}

type PortfolioLoweredIssue struct {
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	URL    string `json:"url"`
	Body   string `json:"-"`
}

type PortfolioLowerAction struct {
	Ticket      int    `json:"ticket"`
	Action      string `json:"action"`
	Repo        string `json:"repo,omitempty"`
	IssueNumber int    `json:"issue_number,omitempty"`
	IssueURL    string `json:"issue_url,omitempty"`
	Applied     bool   `json:"applied"`
	Reason      string `json:"reason"`
}

type PortfolioLowerCounts struct {
	Tickets          int `json:"tickets"`
	OpenTickets      int `json:"open_tickets"`
	ExecutionReady   int `json:"execution_ready"`
	Blocked          int `json:"blocked"`
	Skipped          int `json:"skipped"`
	Linked           int `json:"linked"`
	CreateNeeded     int `json:"create_needed"`
	Actions          int `json:"actions"`
	Applied          int `json:"applied"`
	Diagnostics      int `json:"diagnostics"`
	PermissionBlocks int `json:"permission_blocks"`
}

type PortfolioLowerReport struct {
	Command          string                     `json:"command"`
	PortfolioRepo    string                     `json:"portfolio_repo"`
	Repos            []string                   `json:"repos"`
	DryRun           bool                       `json:"dry_run"`
	Apply            bool                       `json:"apply"`
	Counts           PortfolioLowerCounts       `json:"counts"`
	Tickets          []PortfolioTicket          `json:"tickets,omitempty"`
	Actions          []PortfolioLowerAction     `json:"actions,omitempty"`
	Capability       *PortfolioCapabilityReport `json:"capability,omitempty"`
	PermissionBlocks []PortfolioCapabilityBlock `json:"permission_blocks,omitempty"`
	Diagnostics      []PortfolioDiagnostic      `json:"diagnostics,omitempty"`
	FetchedAt        string                     `json:"fetched_at"`
}

func BuildPortfolioLowerReport(portfolioClient PortfolioClient, lowerClient PortfolioLowerClient, repos []RepoRef, capability PortfolioCapabilityReport, apply bool, now time.Time) (PortfolioLowerReport, error) {
	raw, err := portfolioClient.FetchTickets()
	if err != nil {
		return PortfolioLowerReport{}, err
	}
	tickets, diagnostics := ParsePortfolioTickets(raw, repos)
	actions, err := PortfolioLowerPlan(tickets, diagnostics, portfolioClient.PortfolioRepo(), repos, lowerClient, capability)
	if err != nil {
		return PortfolioLowerReport{}, err
	}
	permissionBlocks := PortfolioLowerCapabilityBlocks(capability, actions)
	if apply {
		actions, err = ApplyPortfolioLowerActions(actions, tickets, portfolioClient.PortfolioRepo(), lowerClient, permissionBlocks)
		if err != nil {
			return PortfolioLowerReport{}, err
		}
	}
	report := PortfolioLowerReport{
		Command:          "portfolio lower",
		PortfolioRepo:    portfolioClient.PortfolioRepo().FullName(),
		Repos:            repoNames(repos),
		DryRun:           !apply,
		Apply:            apply,
		Counts:           portfolioLowerCounts(tickets, actions, diagnostics, permissionBlocks),
		Tickets:          tickets,
		Actions:          actions,
		Capability:       &capability,
		PermissionBlocks: permissionBlocks,
		Diagnostics:      diagnostics,
		FetchedAt:        now.UTC().Format(time.RFC3339),
	}
	return report, nil
}

func PortfolioLowerPlan(tickets []PortfolioTicket, diagnostics []PortfolioDiagnostic, portfolioRepo RepoRef, repos []RepoRef, client PortfolioLowerClient, capability PortfolioCapabilityReport) ([]PortfolioLowerAction, error) {
	invalid := portfolioInvalidTickets(diagnostics)
	actions := make([]PortfolioLowerAction, 0)
	for _, ticket := range tickets {
		if !strings.EqualFold(ticket.State, "open") {
			continue
		}
		if _, blocked := invalid[ticket.Number]; blocked {
			actions = append(actions, PortfolioLowerAction{Ticket: ticket.Number, Action: "ticket:blocked_invalid_repo", Reason: "ticket has validation errors"})
			continue
		}
		if ticket.Routing == "deferred" {
			actions = append(actions, PortfolioLowerAction{Ticket: ticket.Number, Action: "ticket:deferred", Reason: "ticket is deferred"})
			continue
		}
		if ticket.Routing != "single_repo" && ticket.Routing != "multi_repo" {
			actions = append(actions, PortfolioLowerAction{Ticket: ticket.Number, Action: "ticket:needs_routing", Reason: "routing is not execution-ready"})
			continue
		}
		childByRepo := childIssueRefs(ticket.ChildIssues)
		for _, repoName := range ticket.TargetRepos {
			repo, err := ParseRepoRef(repoName)
			if err != nil {
				actions = append(actions, PortfolioLowerAction{Ticket: ticket.Number, Action: "ticket:blocked_invalid_repo", Repo: repoName, Reason: "target repo is invalid"})
				continue
			}
			if block, ok := portfolioCapabilityBlockFor(capability, repo.FullName(), "execution", "issues:read"); ok {
				actions = append(actions, PortfolioLowerAction{Ticket: ticket.Number, Action: "execution_issue:blocked_permission", Repo: repo.FullName(), Reason: block.Reason})
				continue
			}
			existing, err := client.SearchLoweredIssues(repo, portfolioRepo, ticket.Number)
			if err != nil {
				return nil, err
			}
			if len(existing) > 1 {
				actions = append(actions, PortfolioLowerAction{Ticket: ticket.Number, Action: "execution_issue:ambiguous_existing", Repo: repo.FullName(), Reason: "multiple matching lowered issues exist"})
				continue
			}
			if len(existing) == 1 {
				actions = append(actions, PortfolioLowerAction{Ticket: ticket.Number, Action: "execution_issue:link_existing", Repo: repo.FullName(), IssueNumber: existing[0].Number, IssueURL: existing[0].URL, Reason: "lowering evidence already exists"})
				if _, ok := childByRepo[repo.FullName()]; !ok {
					actions = append(actions, PortfolioLowerAction{Ticket: ticket.Number, Action: "portfolio_ticket:update_child_issues", Repo: repo.FullName(), IssueNumber: existing[0].Number, IssueURL: existing[0].URL, Reason: "parent child issue link is missing"})
				}
				continue
			}
			if child, ok := childByRepo[repo.FullName()]; ok {
				actions = append(actions, PortfolioLowerAction{Ticket: ticket.Number, Action: "execution_issue:link_existing", Repo: repo.FullName(), IssueNumber: child.Number, Reason: "child_issues already references execution work"})
				continue
			}
			actions = append(actions, PortfolioLowerAction{Ticket: ticket.Number, Action: "execution_issue:create", Repo: repo.FullName(), Reason: "no lowered issue found for target repo"})
			actions = append(actions, PortfolioLowerAction{Ticket: ticket.Number, Action: "portfolio_ticket:update_child_issues", Repo: repo.FullName(), Reason: "parent child issue link will be appended after creation"})
		}
	}
	return actions, nil
}

func ApplyPortfolioLowerActions(actions []PortfolioLowerAction, tickets []PortfolioTicket, portfolioRepo RepoRef, client PortfolioLowerClient, permissionBlocks []PortfolioCapabilityBlock) ([]PortfolioLowerAction, error) {
	blocked := map[string]struct{}{}
	for _, block := range permissionBlocks {
		blocked[block.Repo+":"+block.Required] = struct{}{}
	}
	ticketsByNumber := map[int]PortfolioTicket{}
	for _, ticket := range tickets {
		ticketsByNumber[ticket.Number] = ticket
	}
	created := map[string]PortfolioLoweredIssue{}
	out := append([]PortfolioLowerAction(nil), actions...)
	parentWriteBlocked := false
	if _, ok := blocked[portfolioRepo.FullName()+":issues:write"]; ok {
		parentWriteBlocked = true
	}
	createNeedsParentUpdate := map[string]struct{}{}
	for _, action := range out {
		if action.Action == "portfolio_ticket:update_child_issues" {
			createNeedsParentUpdate[actionKey(action)] = struct{}{}
		}
	}
	for i, action := range out {
		if action.Action != "execution_issue:create" {
			continue
		}
		if _, ok := blocked[action.Repo+":issues:write"]; ok {
			continue
		}
		if parentWriteBlocked {
			if _, ok := createNeedsParentUpdate[actionKey(action)]; ok {
				continue
			}
		}
		repo, err := ParseRepoRef(action.Repo)
		if err != nil {
			continue
		}
		issue, err := client.CreateExecutionIssue(repo, ticketsByNumber[action.Ticket], portfolioRepo)
		if err != nil {
			return nil, err
		}
		out[i].Applied = true
		out[i].IssueNumber = issue.Number
		out[i].IssueURL = issue.URL
		created[actionKey(action)] = issue
	}
	parentUpdates := map[int][]string{}
	parentUpdateIndexes := map[int][]int{}
	for i, action := range out {
		if action.Action != "portfolio_ticket:update_child_issues" {
			continue
		}
		if _, ok := blocked[portfolioRepo.FullName()+":issues:write"]; ok {
			continue
		}
		issueNumber := action.IssueNumber
		issueURL := action.IssueURL
		if issueNumber == 0 {
			if createdIssue, ok := created[actionKey(action)]; ok {
				issueNumber = createdIssue.Number
				issueURL = createdIssue.URL
				out[i].IssueNumber = issueNumber
				out[i].IssueURL = issueURL
			}
		}
		if issueNumber == 0 {
			continue
		}
		parentUpdates[action.Ticket] = append(parentUpdates[action.Ticket], fmt.Sprintf("%s#%d", action.Repo, issueNumber))
		parentUpdateIndexes[action.Ticket] = append(parentUpdateIndexes[action.Ticket], i)
	}
	for ticketNumber, childIssues := range parentUpdates {
		ticket := ticketsByNumber[ticketNumber]
		if err := client.UpdatePortfolioChildIssues(ticket, childIssues); err != nil {
			return nil, err
		}
		for _, index := range parentUpdateIndexes[ticketNumber] {
			out[index].Applied = true
		}
	}
	return out, nil
}

func PortfolioLowerCapabilityBlocks(report PortfolioCapabilityReport, actions []PortfolioLowerAction) []PortfolioCapabilityBlock {
	planActions := make([]PortfolioPlanAction, 0, len(actions))
	for _, action := range actions {
		if action.Action == "execution_issue:create" || action.Action == "execution_issue:link_existing" {
			planActions = append(planActions, PortfolioPlanAction{Action: action.Action, Repo: action.Repo})
		}
	}
	blocks := PortfolioCapabilityBlocksForActions(report, planActions)
	seen := map[string]struct{}{}
	for _, block := range blocks {
		seen[block.CheckID] = struct{}{}
	}
	for _, action := range actions {
		if action.Action != "execution_issue:blocked_permission" {
			continue
		}
		block, _ := portfolioCapabilityBlockFor(report, action.Repo, "execution", "issues:read")
		if _, ok := seen[block.CheckID]; ok {
			continue
		}
		blocks = append(blocks, block)
		seen[block.CheckID] = struct{}{}
	}
	for _, action := range actions {
		if action.Action == "portfolio_ticket:update_child_issues" {
			if block, ok := portfolioCapabilityBlockFor(report, report.PortfolioRepo, "portfolio", "issues:write"); ok {
				if _, seenBlock := seen[block.CheckID]; !seenBlock {
					blocks = append(blocks, block)
					seen[block.CheckID] = struct{}{}
				}
			}
			break
		}
	}
	return blocks
}

func portfolioCapabilityBlockFor(report PortfolioCapabilityReport, repo string, role string, required string) (PortfolioCapabilityBlock, bool) {
	for _, block := range report.BlockedActions {
		if block.Repo == repo && block.Required == required {
			return block, true
		}
	}
	for _, repoCapability := range report.Repos {
		if repoCapability.Repo != repo || repoCapability.Role != role {
			continue
		}
		if repoCapability.Capabilities[required] == ProjectCapabilityAllowed {
			return PortfolioCapabilityBlock{}, false
		}
		reason := "permission denied"
		if repoCapability.Capabilities[required] == ProjectCapabilityDeniedScope {
			reason = "token scope or repository permission is insufficient"
		} else if repoCapability.Capabilities[required] == ProjectCapabilityUnsupported {
			reason = "issue write capability cannot be proven non-destructively with this token"
		}
		return PortfolioCapabilityBlock{CheckID: role + ":" + repo + ":" + required, Repo: repo, Role: role, Required: required, Reason: reason}, true
	}
	return PortfolioCapabilityBlock{CheckID: role + ":" + repo + ":" + required, Repo: repo, Role: role, Required: required, Reason: "permission denied"}, true
}

func (c GHPortfolioLowerClient) SearchLoweredIssues(repo RepoRef, portfolioRepo RepoRef, ticket int) ([]PortfolioLoweredIssue, error) {
	query := fmt.Sprintf("\"portfolio_ticket: %d\" \"target_repo: %s\"", ticket, repo.FullName())
	output, err := c.runner.Run("gh", "search", "issues", query, "--repo", repo.FullName(), "--match", "body", "--state", "all", "--json", "number,title,body,url,state", "--limit", "20")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Number int    `json:"number"`
		Body   string `json:"body"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(output, &rows); err != nil {
		return nil, fmt.Errorf("parse lowered issue search: %w", err)
	}
	out := []PortfolioLoweredIssue{}
	for _, row := range rows {
		if loweringEvidenceMatches(row.Body, portfolioRepo, ticket, repo) {
			out = append(out, PortfolioLoweredIssue{Repo: repo.FullName(), Number: row.Number, URL: row.URL, Body: row.Body})
		}
	}
	return out, nil
}

func (c GHPortfolioLowerClient) CreateExecutionIssue(repo RepoRef, ticket PortfolioTicket, portfolioRepo RepoRef) (PortfolioLoweredIssue, error) {
	body := renderLoweredIssueBody(ticket, portfolioRepo, repo)
	output, err := c.runner.Run("gh", "issue", "create", "--repo", repo.FullName(), "--title", ticket.Title, "--body", body, "--label", "type:task", "--label", "status:ready")
	if err != nil {
		return PortfolioLoweredIssue{}, err
	}
	url := strings.TrimSpace(string(output))
	return PortfolioLoweredIssue{Repo: repo.FullName(), Number: extractIssueNumber(url), URL: url, Body: body}, nil
}

func (c GHPortfolioLowerClient) UpdatePortfolioChildIssues(ticket PortfolioTicket, childIssues []string) error {
	body := appendPortfolioChildIssues(ticket.BodyForUpdate(), childIssues)
	_, err := c.runner.Run("gh", "api", "repos/"+c.portfolioRepo.FullName()+"/issues/"+strconv.Itoa(ticket.Number), "-X", "PATCH", "-f", "body="+body)
	return err
}

func (t PortfolioTicket) BodyForUpdate() string {
	if strings.TrimSpace(t.Body) != "" {
		return t.Body
	}
	return renderPortfolioTicketBody(t)
}

func renderPortfolioTicketBody(ticket PortfolioTicket) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Goal\n%s\n\n", ticket.Goal)
	fmt.Fprintf(&b, "## Scope\n%s\n\n", ticket.Scope)
	fmt.Fprintf(&b, "## Routing\n%s\n\n", ticket.Routing)
	fmt.Fprintf(&b, "## Target Repos\n%s\n\n", strings.Join(ticket.TargetRepos, "\n"))
	fmt.Fprintf(&b, "## Acceptance Criteria\n%s\n\n", ticket.AcceptanceCriteria)
	if len(ticket.ChildIssues) > 0 {
		fmt.Fprintf(&b, "## Child Issues\n%s\n", strings.Join(ticket.ChildIssues, "\n"))
	}
	return b.String()
}

func renderLoweredIssueBody(ticket PortfolioTicket, portfolioRepo RepoRef, targetRepo RepoRef) string {
	return fmt.Sprintf("## Goal\n%s\n\n## Scope\n%s\n\n## Acceptance Criteria\n%s\n\n## Files To Change\nUnknown until refined.\n\n## Verification Commands\nUnknown until refined.\n\n## Blocker Format\nComment on this issue with the blocker, attempted command, and required decision.\n\n## Parent Ticket\n%s#%d\n\n## Gira Lowering\nportfolio_repo: %s\nportfolio_ticket: %d\ntarget_repo: %s\n", ticket.Goal, ticket.Scope, ticket.AcceptanceCriteria, portfolioRepo.FullName(), ticket.Number, portfolioRepo.FullName(), ticket.Number, targetRepo.FullName())
}

func appendPortfolioChildIssues(body string, childIssues []string) string {
	existing := parsePortfolioFields(body)["child_issues"]
	lines := parsePortfolioListPreserveOrder(existing)
	seen := map[string]struct{}{}
	for _, line := range lines {
		seen[line] = struct{}{}
	}
	for _, child := range childIssues {
		if _, ok := seen[child]; !ok {
			lines = append(lines, child)
			seen[child] = struct{}{}
		}
	}
	base := body
	ranges := portfolioHeadingRe.FindAllStringSubmatchIndex(body, -1)
	for i, match := range ranges {
		name := normalizePortfolioFieldName(body[match[2]:match[3]])
		if name != "child_issues" {
			continue
		}
		end := len(body)
		if i+1 < len(ranges) {
			end = ranges[i+1][0]
		}
		return body[:match[1]] + "\n" + strings.Join(lines, "\n") + "\n" + body[end:]
	}
	return strings.TrimRight(base, "\n") + "\n\n## Child Issues\n" + strings.Join(lines, "\n") + "\n"
}

func parsePortfolioListPreserveOrder(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, ",", "\n")
	seen := map[string]struct{}{}
	out := []string{}
	for _, line := range strings.Split(value, "\n") {
		item := strings.TrimSpace(strings.TrimPrefix(line, "-"))
		item = strings.TrimSpace(strings.TrimPrefix(item, "*"))
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func loweringEvidenceMatches(body string, portfolioRepo RepoRef, ticket int, targetRepo RepoRef) bool {
	evidence := parseLoweringEvidence(body)
	return evidence["portfolio_repo"] == portfolioRepo.FullName() &&
		evidence["portfolio_ticket"] == strconv.Itoa(ticket) &&
		evidence["target_repo"] == targetRepo.FullName()
}

func parseLoweringEvidence(body string) map[string]string {
	section := parsePortfolioFields(body)["gira_lowering"]
	out := map[string]string{}
	for _, line := range strings.Split(section, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = normalizePortfolioFieldName(key)
		switch key {
		case "portfolio_repo", "portfolio_ticket", "target_repo":
			out[key] = strings.TrimSpace(value)
		}
	}
	return out
}

type childIssueRef struct {
	Repo   string
	Number int
}

func childIssueRefs(issues []string) map[string]childIssueRef {
	out := map[string]childIssueRef{}
	for _, issue := range issues {
		value := strings.TrimSpace(issue)
		parts := portfolioChildIssueRe.FindStringSubmatch(value)
		if parts == nil {
			continue
		}
		_, numberRaw, _ := strings.Cut(value, "#")
		number, _ := strconv.Atoi(numberRaw)
		out[parts[1]] = childIssueRef{Repo: parts[1], Number: number}
	}
	return out
}

func extractIssueNumber(url string) int {
	parts := strings.Split(strings.TrimSpace(url), "/")
	if len(parts) == 0 {
		return 0
	}
	n, _ := strconv.Atoi(parts[len(parts)-1])
	return n
}

func actionKey(action PortfolioLowerAction) string {
	return strconv.Itoa(action.Ticket) + ":" + action.Repo
}

func portfolioInvalidTickets(diagnostics []PortfolioDiagnostic) map[int]struct{} {
	invalid := map[int]struct{}{}
	for _, diag := range diagnostics {
		if diag.RuleID == "invalid_target_repo" || diag.RuleID == "invalid_child_issue" || diag.RuleID == "missing_required_field" || diag.RuleID == "invalid_routing" {
			invalid[diag.Ticket] = struct{}{}
		}
	}
	return invalid
}

func portfolioLowerCounts(tickets []PortfolioTicket, actions []PortfolioLowerAction, diagnostics []PortfolioDiagnostic, permissionBlocks []PortfolioCapabilityBlock) PortfolioLowerCounts {
	counts := PortfolioLowerCounts{Tickets: len(tickets), Actions: len(actions), Diagnostics: len(diagnostics), PermissionBlocks: len(permissionBlocks)}
	invalid := portfolioInvalidTickets(diagnostics)
	blockedTickets := map[int]struct{}{}
	skippedTickets := map[int]struct{}{}
	linkedTickets := map[int]struct{}{}
	createNeededTickets := map[int]struct{}{}
	for _, ticket := range tickets {
		if strings.EqualFold(ticket.State, "open") {
			counts.OpenTickets++
			if _, blocked := invalid[ticket.Number]; !blocked && (ticket.Routing == "single_repo" || ticket.Routing == "multi_repo") {
				counts.ExecutionReady++
			}
		}
	}
	for _, action := range actions {
		if action.Applied {
			counts.Applied++
		}
		switch action.Action {
		case "execution_issue:create":
			createNeededTickets[action.Ticket] = struct{}{}
		case "execution_issue:link_existing":
			linkedTickets[action.Ticket] = struct{}{}
		case "ticket:blocked_invalid_repo", "execution_issue:blocked_permission", "execution_issue:ambiguous_existing":
			blockedTickets[action.Ticket] = struct{}{}
		case "ticket:needs_routing", "ticket:deferred":
			skippedTickets[action.Ticket] = struct{}{}
		}
	}
	counts.Blocked = len(blockedTickets)
	counts.Skipped = len(skippedTickets)
	counts.Linked = len(linkedTickets)
	counts.CreateNeeded = len(createNeededTickets)
	return counts
}

func FormatPortfolioLowerReport(report PortfolioLowerReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s:\n", report.Command)
	fmt.Fprintf(&b, "portfolio repo: %s\n", report.PortfolioRepo)
	fmt.Fprintf(&b, "repos:          %s\n", strings.Join(report.Repos, ", "))
	fmt.Fprintf(&b, "flow:           ready=%d blocked=%d skipped=%d linked=%d create_needed=%d\n", report.Counts.ExecutionReady, report.Counts.Blocked, report.Counts.Skipped, report.Counts.Linked, report.Counts.CreateNeeded)
	fmt.Fprintf(&b, "actions:        %d applied=%d\n", report.Counts.Actions, report.Counts.Applied)
	for _, action := range report.Actions {
		if action.IssueNumber > 0 {
			fmt.Fprintf(&b, "  %s ticket #%d -> %s#%d (%s)\n", action.Action, action.Ticket, action.Repo, action.IssueNumber, action.Reason)
		} else if action.Repo != "" {
			fmt.Fprintf(&b, "  %s ticket #%d -> %s (%s)\n", action.Action, action.Ticket, action.Repo, action.Reason)
		} else {
			fmt.Fprintf(&b, "  %s ticket #%d (%s)\n", action.Action, action.Ticket, action.Reason)
		}
	}
	if len(report.PermissionBlocks) > 0 {
		fmt.Fprintf(&b, "permissions:    %d blocked\n", len(report.PermissionBlocks))
	}
	if len(report.Diagnostics) > 0 {
		fmt.Fprintf(&b, "diagnostics:    %d\n", len(report.Diagnostics))
	}
	if report.Apply {
		b.WriteString("next step: review created child issues and rerun gira portfolio lower --dry-run --config .gira/config.yaml\n")
	} else if len(report.Diagnostics) > 0 || len(report.PermissionBlocks) > 0 || report.Counts.Blocked > 0 {
		b.WriteString("next step: fix blocked portfolio lower actions, then rerun gira portfolio lower --dry-run --config .gira/config.yaml\n")
	} else if !portfolioLowerHasApplyableActions(report.Actions) {
		b.WriteString("next step: route or refine skipped portfolio tickets, then rerun gira portfolio lower --dry-run --config .gira/config.yaml\n")
	} else {
		b.WriteString("next step: gira portfolio lower --apply --config .gira/config.yaml\n")
	}
	return b.String()
}

func portfolioLowerHasApplyableActions(actions []PortfolioLowerAction) bool {
	for _, action := range actions {
		if action.Action == "execution_issue:create" || action.Action == "portfolio_ticket:update_child_issues" {
			return true
		}
	}
	return false
}
