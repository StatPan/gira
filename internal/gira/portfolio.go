package gira

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

const PortfolioCommandName = "portfolio"

type PortfolioClient interface {
	PortfolioRepo() RepoRef
	FetchTickets() ([]PortfolioRawTicket, error)
}

type GHPortfolioClient struct {
	portfolioRepo RepoRef
	runner        CommandRunner
}

func NewGHPortfolioClient(portfolioRepo RepoRef, runner CommandRunner) GHPortfolioClient {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	return GHPortfolioClient{portfolioRepo: portfolioRepo, runner: runner}
}

func (c GHPortfolioClient) PortfolioRepo() RepoRef { return c.portfolioRepo }

func (c GHPortfolioClient) FetchTickets() ([]PortfolioRawTicket, error) {
	output, err := c.runner.Run("gh", "api", "repos/"+c.portfolioRepo.FullName()+"/issues", "--paginate", "--slurp", "-X", "GET", "-f", "state=all", "-f", "per_page=100")
	if err != nil {
		return nil, err
	}
	var pages json.RawMessage
	if err := json.Unmarshal(output, &pages); err != nil {
		return nil, fmt.Errorf("parse portfolio issue pages: %w", err)
	}
	rows, err := flattenPages(pages)
	if err != nil {
		return nil, err
	}
	tickets := make([]PortfolioRawTicket, 0, len(rows))
	for _, row := range rows {
		var raw struct {
			Number      int       `json:"number"`
			Title       string    `json:"title"`
			State       string    `json:"state"`
			Body        string    `json:"body"`
			HTMLURL     string    `json:"html_url"`
			PullRequest *struct{} `json:"pull_request"`
			Labels      []struct {
				Name string `json:"name"`
			} `json:"labels"`
		}
		if err := json.Unmarshal(row, &raw); err != nil {
			return nil, fmt.Errorf("parse portfolio issue row: %w", err)
		}
		if raw.PullRequest != nil {
			continue
		}
		labels := make([]string, 0, len(raw.Labels))
		for _, label := range raw.Labels {
			labels = append(labels, label.Name)
		}
		sort.Strings(labels)
		tickets = append(tickets, PortfolioRawTicket{
			Number: raw.Number,
			Title:  raw.Title,
			State:  raw.State,
			Body:   raw.Body,
			URL:    raw.HTMLURL,
			Labels: labels,
		})
	}
	sort.Slice(tickets, func(i, j int) bool { return tickets[i].Number < tickets[j].Number })
	return tickets, nil
}

type PortfolioRawTicket struct {
	Number int      `json:"number"`
	Title  string   `json:"title"`
	State  string   `json:"state"`
	Body   string   `json:"body"`
	URL    string   `json:"url,omitempty"`
	Labels []string `json:"labels,omitempty"`
}

type PortfolioTicket struct {
	Number             int      `json:"number"`
	Title              string   `json:"title"`
	State              string   `json:"state"`
	URL                string   `json:"url,omitempty"`
	Body               string   `json:"-"`
	Goal               string   `json:"goal,omitempty"`
	Scope              string   `json:"scope,omitempty"`
	Routing            string   `json:"routing,omitempty"`
	TargetRepos        []string `json:"target_repos"`
	AcceptanceCriteria string   `json:"acceptance_criteria,omitempty"`
	ChildIssues        []string `json:"child_issues"`
	Priority           string   `json:"priority,omitempty"`
	TargetDate         string   `json:"target_date,omitempty"`
	NonGoals           string   `json:"non_goals,omitempty"`
}

type PortfolioConfigResolved struct {
	PortfolioRepo RepoRef
	Repos         []RepoRef
}

type PortfolioCounts struct {
	Tickets       int `json:"tickets"`
	OpenTickets   int `json:"open_tickets"`
	LinkedTickets int `json:"linked_tickets"`
	Unlinked      int `json:"unlinked_tickets"`
	Diagnostics   int `json:"diagnostics"`
	Actions       int `json:"actions,omitempty"`
}

type PortfolioDiagnostic struct {
	Ticket int    `json:"ticket"`
	RuleID string `json:"rule_id"`
	Detail string `json:"detail"`
}

type PortfolioPlanAction struct {
	Ticket int    `json:"ticket"`
	Action string `json:"action"`
	Repo   string `json:"repo,omitempty"`
	Reason string `json:"reason"`
}

type PortfolioReport struct {
	Command          string                     `json:"command"`
	PortfolioRepo    string                     `json:"portfolio_repo"`
	Repos            []string                   `json:"repos"`
	DryRun           bool                       `json:"dry_run"`
	Counts           PortfolioCounts            `json:"counts"`
	Tickets          []PortfolioTicket          `json:"tickets,omitempty"`
	Actions          []PortfolioPlanAction      `json:"actions,omitempty"`
	Capability       *PortfolioCapabilityReport `json:"capability,omitempty"`
	PermissionBlocks []PortfolioCapabilityBlock `json:"permission_blocks,omitempty"`
	Diagnostics      []PortfolioDiagnostic      `json:"diagnostics,omitempty"`
	FetchedAt        string                     `json:"fetched_at"`
}

func ResolvePortfolioConfig(path string) (PortfolioConfigResolved, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultInitConfigPath(".")
	}
	cfg, err := loadPortfolioConfig(path)
	if err != nil {
		return PortfolioConfigResolved{}, err
	}
	if strings.TrimSpace(cfg.Repo) == "" {
		return PortfolioConfigResolved{}, fmt.Errorf("portfolio.repo is required in %s", path)
	}
	if len(cfg.Repos) == 0 {
		return PortfolioConfigResolved{}, fmt.Errorf("portfolio.repos must include at least one execution repo in %s", path)
	}
	portfolioRepo, err := ParseRepoRef(cfg.Repo)
	if err != nil {
		return PortfolioConfigResolved{}, fmt.Errorf("portfolio.repo must be in OWNER/REPO format")
	}
	repos := make([]RepoRef, 0, len(cfg.Repos))
	seen := map[string]struct{}{}
	for i, value := range cfg.Repos {
		repo, err := ParseRepoRef(value)
		if err != nil {
			return PortfolioConfigResolved{}, fmt.Errorf("portfolio.repos[%d] must be in OWNER/REPO format", i)
		}
		key := strings.ToLower(repo.FullName())
		if _, ok := seen[key]; ok {
			return PortfolioConfigResolved{}, fmt.Errorf("portfolio.repos[%d] duplicates %s", i, repo.FullName())
		}
		seen[key] = struct{}{}
		repos = append(repos, repo)
	}
	return PortfolioConfigResolved{PortfolioRepo: portfolioRepo, Repos: repos}, nil
}

func loadPortfolioConfig(path string) (PortfolioConfig, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return PortfolioConfig{}, fmt.Errorf("read portfolio config %q: %w", path, err)
	}
	var cfg struct {
		Portfolio PortfolioConfig `yaml:"portfolio" toml:"portfolio"`
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".toml":
		if err := toml.Unmarshal(content, &cfg); err != nil {
			return PortfolioConfig{}, fmt.Errorf("parse portfolio config %q: %w", path, err)
		}
	default:
		if err := yaml.Unmarshal(content, &cfg); err != nil {
			return PortfolioConfig{}, fmt.Errorf("parse portfolio config %q: %w", path, err)
		}
	}
	return cfg.Portfolio, nil
}

func BuildPortfolioStatusReport(client PortfolioClient, repos []RepoRef, now time.Time) (PortfolioReport, error) {
	tickets, err := client.FetchTickets()
	if err != nil {
		return PortfolioReport{}, err
	}
	parsed, diagnostics := ParsePortfolioTickets(tickets, repos)
	counts := portfolioCounts(parsed, diagnostics, nil)
	return PortfolioReport{
		Command:       "portfolio status",
		PortfolioRepo: client.PortfolioRepo().FullName(),
		Repos:         repoNames(repos),
		Counts:        counts,
		FetchedAt:     now.UTC().Format(time.RFC3339),
	}, nil
}

func BuildPortfolioValidateReport(client PortfolioClient, repos []RepoRef, now time.Time) (PortfolioReport, error) {
	tickets, err := client.FetchTickets()
	if err != nil {
		return PortfolioReport{}, err
	}
	parsed, diagnostics := ParsePortfolioTickets(tickets, repos)
	return PortfolioReport{
		Command:       "portfolio validate",
		PortfolioRepo: client.PortfolioRepo().FullName(),
		Repos:         repoNames(repos),
		Counts:        portfolioCounts(parsed, diagnostics, nil),
		Tickets:       parsed,
		Diagnostics:   diagnostics,
		FetchedAt:     now.UTC().Format(time.RFC3339),
	}, nil
}

func BuildPortfolioPlanReport(client PortfolioClient, repos []RepoRef, now time.Time) (PortfolioReport, error) {
	tickets, err := client.FetchTickets()
	if err != nil {
		return PortfolioReport{}, err
	}
	parsed, diagnostics := ParsePortfolioTickets(tickets, repos)
	actions := PortfolioPlan(parsed, diagnostics, repos)
	return PortfolioReport{
		Command:       "portfolio plan",
		PortfolioRepo: client.PortfolioRepo().FullName(),
		Repos:         repoNames(repos),
		DryRun:        true,
		Counts:        portfolioCounts(parsed, diagnostics, actions),
		Tickets:       parsed,
		Actions:       actions,
		Diagnostics:   diagnostics,
		FetchedAt:     now.UTC().Format(time.RFC3339),
	}, nil
}

func ParsePortfolioTickets(raw []PortfolioRawTicket, repos []RepoRef) ([]PortfolioTicket, []PortfolioDiagnostic) {
	allowed := map[string]struct{}{}
	for _, repo := range repos {
		allowed[repo.FullName()] = struct{}{}
	}
	tickets := make([]PortfolioTicket, 0, len(raw))
	diagnostics := make([]PortfolioDiagnostic, 0)
	for _, item := range raw {
		fields := parsePortfolioFields(item.Body)
		ticket := PortfolioTicket{
			Number:             item.Number,
			Title:              item.Title,
			State:              item.State,
			URL:                item.URL,
			Body:               item.Body,
			Goal:               fields["goal"],
			Scope:              fields["scope"],
			Routing:            normalizePortfolioRouting(fields["routing"]),
			TargetRepos:        parsePortfolioList(fields["target_repos"]),
			AcceptanceCriteria: fields["acceptance_criteria"],
			ChildIssues:        parsePortfolioList(fields["child_issues"]),
			Priority:           fields["priority"],
			TargetDate:         fields["target_date"],
			NonGoals:           fields["non_goals"],
		}
		tickets = append(tickets, ticket)
		diagnostics = append(diagnostics, validatePortfolioTicket(ticket, allowed)...)
	}
	sort.Slice(diagnostics, func(i, j int) bool {
		if diagnostics[i].Ticket == diagnostics[j].Ticket {
			return diagnostics[i].RuleID < diagnostics[j].RuleID
		}
		return diagnostics[i].Ticket < diagnostics[j].Ticket
	})
	return tickets, diagnostics
}

func PortfolioPlan(tickets []PortfolioTicket, diagnostics []PortfolioDiagnostic, repos []RepoRef) []PortfolioPlanAction {
	invalid := map[int]struct{}{}
	for _, diag := range diagnostics {
		if diag.RuleID == "invalid_target_repo" || diag.RuleID == "invalid_child_issue" || diag.RuleID == "missing_required_field" || diag.RuleID == "invalid_routing" {
			invalid[diag.Ticket] = struct{}{}
		}
	}
	actions := make([]PortfolioPlanAction, 0)
	for _, ticket := range tickets {
		if !strings.EqualFold(ticket.State, "open") {
			continue
		}
		if _, blocked := invalid[ticket.Number]; blocked {
			actions = append(actions, PortfolioPlanAction{Ticket: ticket.Number, Action: "ticket:blocked_invalid_repo", Reason: "ticket has validation errors"})
			continue
		}
		if ticket.Routing != "single_repo" && ticket.Routing != "multi_repo" {
			actions = append(actions, PortfolioPlanAction{Ticket: ticket.Number, Action: "ticket:needs_routing", Reason: "routing is not execution-ready"})
			continue
		}
		if len(ticket.TargetRepos) == 0 {
			actions = append(actions, PortfolioPlanAction{Ticket: ticket.Number, Action: "ticket:needs_routing", Reason: "routing or target_repos is not execution-ready"})
			continue
		}
		childRepos := childIssueRepos(ticket.ChildIssues)
		for _, repo := range ticket.TargetRepos {
			if _, ok := childRepos[repo]; ok {
				actions = append(actions, PortfolioPlanAction{Ticket: ticket.Number, Action: "execution_issue:link_existing", Repo: repo, Reason: "child_issues already references execution work"})
			} else {
				actions = append(actions, PortfolioPlanAction{Ticket: ticket.Number, Action: "execution_issue:create", Repo: repo, Reason: "no child issue linked for target repo"})
			}
		}
	}
	return actions
}

func FormatPortfolioReport(report PortfolioReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s:\n", report.Command)
	fmt.Fprintf(&b, "portfolio repo: %s\n", report.PortfolioRepo)
	fmt.Fprintf(&b, "repos:          %s\n", strings.Join(report.Repos, ", "))
	fmt.Fprintf(&b, "tickets:        %d open=%d linked=%d unlinked=%d\n", report.Counts.Tickets, report.Counts.OpenTickets, report.Counts.LinkedTickets, report.Counts.Unlinked)
	if len(report.Actions) > 0 {
		fmt.Fprintf(&b, "actions:        %d\n", len(report.Actions))
		for _, action := range report.Actions {
			if action.Repo != "" {
				fmt.Fprintf(&b, "  %s ticket #%d -> %s (%s)\n", action.Action, action.Ticket, action.Repo, action.Reason)
			} else {
				fmt.Fprintf(&b, "  %s ticket #%d (%s)\n", action.Action, action.Ticket, action.Reason)
			}
		}
	}
	if len(report.Diagnostics) > 0 {
		fmt.Fprintf(&b, "diagnostics:    %d\n", len(report.Diagnostics))
		for _, diag := range report.Diagnostics {
			fmt.Fprintf(&b, "  ticket #%d %s: %s\n", diag.Ticket, diag.RuleID, diag.Detail)
		}
	}
	if len(report.PermissionBlocks) > 0 {
		fmt.Fprintf(&b, "permissions:    %d blocked\n", len(report.PermissionBlocks))
		for _, block := range report.PermissionBlocks {
			fmt.Fprintf(&b, "  %s requires %s (%s)\n", block.CheckID, block.Required, block.Reason)
		}
	}
	switch report.Command {
	case "portfolio plan":
		b.WriteString("next step: review the dry-run actions; apply/lower behavior is not implemented yet\n")
	case "portfolio validate":
		b.WriteString("next step: fix diagnostics before planning portfolio lowering\n")
	default:
		b.WriteString("next step: gira portfolio plan --dry-run --config .gira/config.yaml\n")
	}
	return b.String()
}

func validatePortfolioTicket(ticket PortfolioTicket, allowed map[string]struct{}) []PortfolioDiagnostic {
	diagnostics := make([]PortfolioDiagnostic, 0)
	required := map[string]string{
		"goal":                ticket.Goal,
		"scope":               ticket.Scope,
		"routing":             ticket.Routing,
		"acceptance_criteria": ticket.AcceptanceCriteria,
	}
	for field, value := range required {
		if strings.TrimSpace(value) == "" {
			diagnostics = append(diagnostics, PortfolioDiagnostic{Ticket: ticket.Number, RuleID: "missing_required_field", Detail: field + " is required"})
		}
	}
	if ticket.Routing != "unrouted" && ticket.Routing != "single_repo" && ticket.Routing != "multi_repo" && ticket.Routing != "deferred" {
		diagnostics = append(diagnostics, PortfolioDiagnostic{Ticket: ticket.Number, RuleID: "invalid_routing", Detail: "routing must be one of unrouted, single_repo, multi_repo, deferred"})
	}
	if (ticket.Routing == "single_repo" || ticket.Routing == "multi_repo") && len(ticket.TargetRepos) == 0 {
		diagnostics = append(diagnostics, PortfolioDiagnostic{Ticket: ticket.Number, RuleID: "missing_required_field", Detail: "target_repos is required for execution routing"})
	}
	if ticket.Routing == "single_repo" && len(ticket.TargetRepos) > 1 {
		diagnostics = append(diagnostics, PortfolioDiagnostic{Ticket: ticket.Number, RuleID: "invalid_routing", Detail: "single_repo routing must target exactly one repo"})
	}
	for _, repo := range ticket.TargetRepos {
		if _, ok := allowed[repo]; !ok {
			diagnostics = append(diagnostics, PortfolioDiagnostic{Ticket: ticket.Number, RuleID: "invalid_target_repo", Detail: fmt.Sprintf("%s is not in portfolio.repos", repo)})
		}
	}
	for _, issue := range ticket.ChildIssues {
		repo, ok := childIssueRepo(issue)
		if !ok {
			diagnostics = append(diagnostics, PortfolioDiagnostic{Ticket: ticket.Number, RuleID: "invalid_child_issue", Detail: fmt.Sprintf("%s must be OWNER/REPO#N", issue)})
			continue
		}
		if _, ok := allowed[repo]; !ok {
			diagnostics = append(diagnostics, PortfolioDiagnostic{Ticket: ticket.Number, RuleID: "invalid_child_issue", Detail: fmt.Sprintf("%s points outside portfolio.repos", issue)})
			continue
		}
		if len(ticket.TargetRepos) > 0 && !containsString(ticket.TargetRepos, repo) {
			diagnostics = append(diagnostics, PortfolioDiagnostic{Ticket: ticket.Number, RuleID: "invalid_child_issue", Detail: fmt.Sprintf("%s does not match target_repos", issue)})
		}
	}
	return diagnostics
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func childIssueRepos(issues []string) map[string]struct{} {
	repos := map[string]struct{}{}
	for _, issue := range issues {
		repo, ok := childIssueRepo(issue)
		if ok {
			repos[repo] = struct{}{}
		}
	}
	return repos
}

var portfolioChildIssueRe = regexp.MustCompile(`^([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)#[1-9][0-9]*$`)

func childIssueRepo(issue string) (string, bool) {
	match := portfolioChildIssueRe.FindStringSubmatch(strings.TrimSpace(issue))
	if match == nil {
		return "", false
	}
	return match[1], true
}

var portfolioHeadingRe = regexp.MustCompile(`(?m)^#{2,4}\s+([A-Za-z0-9 _-]+)\s*$`)

func parsePortfolioFields(body string) map[string]string {
	fields := map[string]string{}
	matches := portfolioHeadingRe.FindAllStringSubmatchIndex(body, -1)
	for i, match := range matches {
		name := normalizePortfolioFieldName(body[match[2]:match[3]])
		start := match[1]
		end := len(body)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		fields[name] = cleanPortfolioFieldValue(body[start:end])
	}
	for _, line := range strings.Split(body, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		name := normalizePortfolioFieldName(key)
		switch name {
		case "goal", "scope", "routing", "target_repos", "acceptance_criteria", "child_issues", "priority", "target_date", "non_goals":
			if strings.TrimSpace(fields[name]) == "" {
				fields[name] = cleanPortfolioFieldValue(value)
			}
		}
	}
	return fields
}

func cleanPortfolioFieldValue(value string) string {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "_No response_") {
		return ""
	}
	return value
}

func normalizePortfolioFieldName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	return value
}

func normalizePortfolioRouting(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func parsePortfolioList(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, ",", "\n")
	seen := map[string]struct{}{}
	out := []string{}
	for _, line := range strings.Split(value, "\n") {
		item := strings.TrimSpace(strings.TrimPrefix(line, "-"))
		item = strings.TrimSpace(strings.TrimPrefix(item, "*"))
		if item == "" || strings.EqualFold(item, "_No response_") {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func portfolioCounts(tickets []PortfolioTicket, diagnostics []PortfolioDiagnostic, actions []PortfolioPlanAction) PortfolioCounts {
	counts := PortfolioCounts{Tickets: len(tickets), Diagnostics: len(diagnostics), Actions: len(actions)}
	for _, ticket := range tickets {
		if strings.EqualFold(ticket.State, "open") {
			counts.OpenTickets++
		}
		if len(ticket.ChildIssues) > 0 {
			counts.LinkedTickets++
		} else {
			counts.Unlinked++
		}
	}
	return counts
}

func repoNames(repos []RepoRef) []string {
	names := make([]string, 0, len(repos))
	for _, repo := range repos {
		names = append(names, repo.FullName())
	}
	sort.Strings(names)
	return names
}
