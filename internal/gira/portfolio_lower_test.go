package gira

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPortfolioLowerPlanCreateLinkAndParentUpdate(t *testing.T) {
	client := &fakePortfolioLowerClient{
		existing: map[string][]PortfolioLoweredIssue{
			"StatPan/gira:32": {{Repo: "StatPan/gira", Number: 10, URL: "https://github.com/StatPan/gira/issues/10"}},
		},
	}
	tickets, diagnostics := ParsePortfolioTickets([]PortfolioRawTicket{
		{Number: 31, Title: "Create", State: "open", Body: portfolioBody("single_repo", "StatPan/gira", "")},
		{Number: 32, Title: "Link", State: "open", Body: portfolioBody("multi_repo", "StatPan/gira\n- StatPan/docs", "StatPan/docs#20")},
	}, []RepoRef{mustRepoRefForPortfolio("StatPan/gira"), mustRepoRefForPortfolio("StatPan/docs")})

	actions, err := PortfolioLowerPlan(tickets, diagnostics, mustRepoRefForPortfolio("StatPan/portfolio"), []RepoRef{mustRepoRefForPortfolio("StatPan/gira"), mustRepoRefForPortfolio("StatPan/docs")}, client, allowAllPortfolioLowerCapability())
	if err != nil {
		t.Fatalf("PortfolioLowerPlan error: %v", err)
	}
	counts := map[string]int{}
	for _, action := range actions {
		counts[action.Action]++
	}
	if counts["execution_issue:create"] != 1 || counts["execution_issue:link_existing"] != 2 || counts["portfolio_ticket:update_child_issues"] != 2 {
		t.Fatalf("actions = %+v, want create, links, and parent updates", actions)
	}
}

func TestPortfolioLowerPlanAmbiguousExisting(t *testing.T) {
	client := &fakePortfolioLowerClient{existing: map[string][]PortfolioLoweredIssue{
		"StatPan/gira:31": {
			{Repo: "StatPan/gira", Number: 10},
			{Repo: "StatPan/gira", Number: 11},
		},
	}}
	tickets, diagnostics := ParsePortfolioTickets([]PortfolioRawTicket{
		{Number: 31, Title: "Ambiguous", State: "open", Body: portfolioBody("single_repo", "StatPan/gira", "")},
	}, []RepoRef{mustRepoRefForPortfolio("StatPan/gira")})

	actions, err := PortfolioLowerPlan(tickets, diagnostics, mustRepoRefForPortfolio("StatPan/portfolio"), []RepoRef{mustRepoRefForPortfolio("StatPan/gira")}, client, allowAllPortfolioLowerCapability())
	if err != nil {
		t.Fatalf("PortfolioLowerPlan error: %v", err)
	}
	if len(actions) != 1 || actions[0].Action != "execution_issue:ambiguous_existing" {
		t.Fatalf("actions = %+v, want ambiguous block", actions)
	}
}

func TestApplyPortfolioLowerActionsCreatesAndUpdatesParent(t *testing.T) {
	client := &fakePortfolioLowerClient{}
	tickets, _ := ParsePortfolioTickets([]PortfolioRawTicket{
		{Number: 31, Title: "Create", State: "open", Body: portfolioBody("single_repo", "StatPan/gira", "")},
	}, []RepoRef{mustRepoRefForPortfolio("StatPan/gira")})
	actions := []PortfolioLowerAction{
		{Ticket: 31, Action: "execution_issue:create", Repo: "StatPan/gira"},
		{Ticket: 31, Action: "portfolio_ticket:update_child_issues", Repo: "StatPan/gira"},
	}

	applied, err := ApplyPortfolioLowerActions(actions, tickets, mustRepoRefForPortfolio("StatPan/portfolio"), client, nil)
	if err != nil {
		t.Fatalf("ApplyPortfolioLowerActions error: %v", err)
	}
	if len(client.created) != 1 || len(client.updated) != 1 {
		t.Fatalf("created=%+v updated=%+v, want one each", client.created, client.updated)
	}
	if !applied[0].Applied || !applied[1].Applied || applied[0].IssueNumber == 0 || applied[1].IssueNumber == 0 {
		t.Fatalf("applied actions = %+v, want created issue numbers and applied flags", applied)
	}
}

func TestPortfolioLowerPlanSkipsSearchWhenReadBlocked(t *testing.T) {
	client := &fakePortfolioLowerClient{failOnSearch: true}
	tickets, diagnostics := ParsePortfolioTickets([]PortfolioRawTicket{
		{Number: 31, Title: "Create", State: "open", Body: portfolioBody("single_repo", "StatPan/gira", "")},
	}, []RepoRef{mustRepoRefForPortfolio("StatPan/gira")})
	capability := PortfolioCapabilityReport{
		PortfolioRepo: "StatPan/portfolio",
		Repos: []PortfolioRepoCapability{{
			Repo: "StatPan/gira",
			Role: "execution",
			Capabilities: map[string]ProjectCapabilityStatus{
				"issues:read": ProjectCapabilityDeniedScope,
			},
		}},
	}

	actions, err := PortfolioLowerPlan(tickets, diagnostics, mustRepoRefForPortfolio("StatPan/portfolio"), []RepoRef{mustRepoRefForPortfolio("StatPan/gira")}, client, capability)
	if err != nil {
		t.Fatalf("PortfolioLowerPlan error: %v", err)
	}
	if len(actions) != 1 || actions[0].Action != "execution_issue:blocked_permission" {
		t.Fatalf("actions = %+v, want blocked permission without search", actions)
	}
	if client.searchCalls != 0 {
		t.Fatalf("search calls = %d, want 0", client.searchCalls)
	}
}

func TestApplyPortfolioLowerActionsSkipsPermissionBlockedCreate(t *testing.T) {
	client := &fakePortfolioLowerClient{}
	tickets, _ := ParsePortfolioTickets([]PortfolioRawTicket{
		{Number: 31, Title: "Create", State: "open", Body: portfolioBody("single_repo", "StatPan/gira", "")},
	}, []RepoRef{mustRepoRefForPortfolio("StatPan/gira")})
	actions := []PortfolioLowerAction{{Ticket: 31, Action: "execution_issue:create", Repo: "StatPan/gira"}}

	applied, err := ApplyPortfolioLowerActions(actions, tickets, mustRepoRefForPortfolio("StatPan/portfolio"), client, []PortfolioCapabilityBlock{{Repo: "StatPan/gira", Required: "issues:write"}})
	if err != nil {
		t.Fatalf("ApplyPortfolioLowerActions error: %v", err)
	}
	if len(client.created) != 0 || applied[0].Applied {
		t.Fatalf("created=%+v applied=%+v, want permission-blocked create skipped", client.created, applied)
	}
}

func TestApplyPortfolioLowerActionsSkipsCreateWhenParentUpdateBlocked(t *testing.T) {
	client := &fakePortfolioLowerClient{}
	tickets, _ := ParsePortfolioTickets([]PortfolioRawTicket{
		{Number: 31, Title: "Create", State: "open", Body: portfolioBody("single_repo", "StatPan/gira", "")},
	}, []RepoRef{mustRepoRefForPortfolio("StatPan/gira")})
	actions := []PortfolioLowerAction{
		{Ticket: 31, Action: "execution_issue:create", Repo: "StatPan/gira"},
		{Ticket: 31, Action: "portfolio_ticket:update_child_issues", Repo: "StatPan/gira"},
	}

	applied, err := ApplyPortfolioLowerActions(actions, tickets, mustRepoRefForPortfolio("StatPan/portfolio"), client, []PortfolioCapabilityBlock{{Repo: "StatPan/portfolio", Required: "issues:write"}})
	if err != nil {
		t.Fatalf("ApplyPortfolioLowerActions error: %v", err)
	}
	if len(client.created) != 0 || applied[0].Applied || applied[1].Applied {
		t.Fatalf("created=%+v applied=%+v, want create skipped when parent update is blocked", client.created, applied)
	}
}

func TestPortfolioLowerCapabilityBlocks(t *testing.T) {
	report := PortfolioCapabilityReport{
		PortfolioRepo: "StatPan/portfolio",
		Repos: []PortfolioRepoCapability{{
			Repo: "StatPan/portfolio",
			Role: "portfolio",
			Capabilities: map[string]ProjectCapabilityStatus{
				"issues:write": ProjectCapabilityDeniedScope,
			},
		}},
		BlockedActions: []PortfolioCapabilityBlock{
			{Repo: "StatPan/gira", Required: "issues:write"},
		},
	}
	blocks := PortfolioLowerCapabilityBlocks(report, []PortfolioLowerAction{
		{Action: "execution_issue:create", Repo: "StatPan/gira"},
		{Action: "portfolio_ticket:update_child_issues", Repo: "StatPan/gira"},
	})
	if len(blocks) != 2 {
		t.Fatalf("blocks = %+v, want execution write and parent write blocks", blocks)
	}
}

func TestPortfolioLowerCapabilityBlocksIncludesBlockedRead(t *testing.T) {
	report := PortfolioCapabilityReport{
		PortfolioRepo: "StatPan/portfolio",
		Repos: []PortfolioRepoCapability{{
			Repo: "StatPan/gira",
			Role: "execution",
			Capabilities: map[string]ProjectCapabilityStatus{
				"issues:read": ProjectCapabilityDeniedScope,
			},
		}},
	}

	blocks := PortfolioLowerCapabilityBlocks(report, []PortfolioLowerAction{
		{Action: "execution_issue:blocked_permission", Repo: "StatPan/gira"},
	})

	if len(blocks) != 1 {
		t.Fatalf("blocks = %+v, want one blocked read", blocks)
	}
	if blocks[0].Repo != "StatPan/gira" || blocks[0].Required != "issues:read" {
		t.Fatalf("block = %+v, want execution issues:read block", blocks[0])
	}
}

func TestBuildPortfolioLowerReportCountsBlockedReadPermission(t *testing.T) {
	portfolioClient := fakePortfolioClient{
		repo: mustRepoRefForPortfolio("StatPan/portfolio"),
		tickets: []PortfolioRawTicket{
			{Number: 31, Title: "Create", State: "open", Body: portfolioBody("single_repo", "StatPan/gira", "")},
		},
	}
	capability := PortfolioCapabilityReport{
		PortfolioRepo: "StatPan/portfolio",
		Repos: []PortfolioRepoCapability{{
			Repo: "StatPan/gira",
			Role: "execution",
			Capabilities: map[string]ProjectCapabilityStatus{
				"issues:read": ProjectCapabilityDeniedScope,
			},
		}},
	}

	report, err := BuildPortfolioLowerReport(portfolioClient, &fakePortfolioLowerClient{failOnSearch: true}, []RepoRef{mustRepoRefForPortfolio("StatPan/gira")}, capability, false, portfolioNowFixture)
	if err != nil {
		t.Fatalf("BuildPortfolioLowerReport error: %v", err)
	}
	if report.Counts.PermissionBlocks != 1 || len(report.PermissionBlocks) != 1 {
		t.Fatalf("permission blocks = count %d values %+v, want one blocked read", report.Counts.PermissionBlocks, report.PermissionBlocks)
	}
	if report.Actions[0].Action != "execution_issue:blocked_permission" {
		t.Fatalf("actions = %+v, want blocked permission action", report.Actions)
	}
}

func TestLoweringEvidenceMatchesExactFields(t *testing.T) {
	body := "## Gira Lowering\nportfolio_repo: StatPan/portfolio\nportfolio_ticket: 144\ntarget_repo: StatPan/gira\n"

	if !loweringEvidenceMatches(body, mustRepoRefForPortfolio("StatPan/portfolio"), 144, mustRepoRefForPortfolio("StatPan/gira")) {
		t.Fatalf("lowering evidence should match exact fields")
	}
	if loweringEvidenceMatches(body, mustRepoRefForPortfolio("StatPan/portfolio"), 14, mustRepoRefForPortfolio("StatPan/gira")) {
		t.Fatalf("lowering evidence matched ticket prefix")
	}
	if loweringEvidenceMatches(body, mustRepoRefForPortfolio("StatPan/portfolio"), 144, mustRepoRefForPortfolio("StatPan/gira-docs")) {
		t.Fatalf("lowering evidence matched repo prefix")
	}
}

func TestLoweringEvidenceRequiresGiraLoweringSection(t *testing.T) {
	body := "portfolio_repo: StatPan/portfolio\nportfolio_ticket: 144\ntarget_repo: StatPan/gira\n"

	if loweringEvidenceMatches(body, mustRepoRefForPortfolio("StatPan/portfolio"), 144, mustRepoRefForPortfolio("StatPan/gira")) {
		t.Fatalf("lowering evidence matched without Gira Lowering section")
	}
}

func TestBuildPortfolioLowerReportJSONShape(t *testing.T) {
	portfolioClient := fakePortfolioClient{
		repo: mustRepoRefForPortfolio("StatPan/portfolio"),
		tickets: []PortfolioRawTicket{
			{Number: 31, Title: "Create", State: "open", Body: portfolioBody("single_repo", "StatPan/gira", "")},
		},
	}
	capability := allowAllPortfolioLowerCapability()
	capability.FetchedAt = portfolioNowFixture.Format(time.RFC3339)
	report, err := BuildPortfolioLowerReport(portfolioClient, &fakePortfolioLowerClient{}, []RepoRef{mustRepoRefForPortfolio("StatPan/gira")}, capability, false, portfolioNowFixture)
	if err != nil {
		t.Fatalf("BuildPortfolioLowerReport error: %v", err)
	}
	if report.Command != "portfolio lower" || !report.DryRun || report.Apply {
		t.Fatalf("report = %+v, want lower dry-run report", report)
	}
	output := mustMarshalPortfolioLowerReport(t, report)
	for _, key := range []string{"execution_ready", "blocked", "skipped", "linked", "create_needed"} {
		if !strings.Contains(output, `"`+key+`"`) {
			t.Fatalf("portfolio lower JSON missing %q:\n%s", key, output)
		}
	}
	text := FormatPortfolioLowerReport(report)
	if !strings.Contains(text, "gira portfolio lower --apply") {
		t.Fatalf("text missing apply next step:\n%s", text)
	}
}

func TestPortfolioLowerCountsClassifyOperatorFlow(t *testing.T) {
	client := &fakePortfolioLowerClient{
		existing: map[string][]PortfolioLoweredIssue{
			"StatPan/gira:32": {{Repo: "StatPan/gira", Number: 10, URL: "https://github.com/StatPan/gira/issues/10"}},
		},
	}
	portfolioClient := fakePortfolioClient{
		repo: mustRepoRefForPortfolio("StatPan/portfolio"),
		tickets: []PortfolioRawTicket{
			{Number: 30, Title: "Unrouted", State: "open", Body: portfolioBody("unrouted", "", "")},
			{Number: 31, Title: "Create", State: "open", Body: portfolioBody("single_repo", "StatPan/docs", "")},
			{Number: 32, Title: "Link", State: "open", Body: portfolioBody("single_repo", "StatPan/gira", "")},
			{Number: 33, Title: "Invalid", State: "open", Body: portfolioBody("single_repo", "StatPan/missing", "")},
		},
	}

	report, err := BuildPortfolioLowerReport(portfolioClient, client, []RepoRef{mustRepoRefForPortfolio("StatPan/gira"), mustRepoRefForPortfolio("StatPan/docs")}, allowAllPortfolioLowerCapability(), false, portfolioNowFixture)
	if err != nil {
		t.Fatalf("BuildPortfolioLowerReport error: %v", err)
	}

	if report.Counts.ExecutionReady != 2 || report.Counts.Blocked != 1 || report.Counts.Skipped != 1 || report.Counts.Linked != 1 || report.Counts.CreateNeeded != 1 {
		t.Fatalf("counts = %+v, want ready=2 blocked=1 skipped=1 linked=1 create_needed=1", report.Counts)
	}
}

func TestPortfolioLowerCountsUseTicketLevelFlowCounts(t *testing.T) {
	portfolioClient := fakePortfolioClient{
		repo: mustRepoRefForPortfolio("StatPan/portfolio"),
		tickets: []PortfolioRawTicket{
			{Number: 31, Title: "Multi create", State: "open", Body: portfolioBody("multi_repo", "StatPan/gira\n- StatPan/docs", "")},
		},
	}

	report, err := BuildPortfolioLowerReport(portfolioClient, &fakePortfolioLowerClient{}, []RepoRef{mustRepoRefForPortfolio("StatPan/gira"), mustRepoRefForPortfolio("StatPan/docs")}, allowAllPortfolioLowerCapability(), false, portfolioNowFixture)
	if err != nil {
		t.Fatalf("BuildPortfolioLowerReport error: %v", err)
	}

	if report.Counts.ExecutionReady != 1 || report.Counts.CreateNeeded != 1 {
		t.Fatalf("counts = %+v, want one ready ticket and one create-needed ticket", report.Counts)
	}
	if len(report.Actions) != 4 {
		t.Fatalf("actions = %+v, want create and parent-update actions per target repo", report.Actions)
	}
}

func TestFormatPortfolioLowerReportBlockedNextStep(t *testing.T) {
	report := PortfolioLowerReport{
		Command:       "portfolio lower",
		PortfolioRepo: "StatPan/portfolio",
		Repos:         []string{"StatPan/gira"},
		DryRun:        true,
		Counts:        PortfolioLowerCounts{Blocked: 1},
		Actions:       []PortfolioLowerAction{{Ticket: 31, Action: "execution_issue:ambiguous_existing", Repo: "StatPan/gira", Reason: "multiple matching lowered issues exist"}},
	}

	text := FormatPortfolioLowerReport(report)
	if strings.Contains(text, "gira portfolio lower --apply") {
		t.Fatalf("blocked lower report suggested apply:\n%s", text)
	}
	if !strings.Contains(text, "next step: fix blocked portfolio lower actions") {
		t.Fatalf("blocked lower report missing remediation next step:\n%s", text)
	}
}

func TestFormatPortfolioLowerReportSkippedOnlyNextStep(t *testing.T) {
	report := PortfolioLowerReport{
		Command:       "portfolio lower",
		PortfolioRepo: "StatPan/portfolio",
		Repos:         []string{"StatPan/gira"},
		DryRun:        true,
		Counts:        PortfolioLowerCounts{Skipped: 1},
		Actions:       []PortfolioLowerAction{{Ticket: 31, Action: "ticket:needs_routing", Reason: "routing is not execution-ready"}},
	}

	text := FormatPortfolioLowerReport(report)
	if strings.Contains(text, "gira portfolio lower --apply") {
		t.Fatalf("skipped-only lower report suggested apply:\n%s", text)
	}
	if !strings.Contains(text, "next step: route or refine skipped portfolio tickets") {
		t.Fatalf("skipped-only lower report missing refinement next step:\n%s", text)
	}
}

func mustMarshalPortfolioLowerReport(t *testing.T, report PortfolioLowerReport) string {
	t.Helper()
	output, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("marshal lower report: %v", err)
	}
	return string(output)
}

func TestAppendPortfolioChildIssuesPreservesFollowingSections(t *testing.T) {
	body := "## Goal\nG\n\n## Child Issues\nStatPan/gira#1\n\n## Non Goals\nN\n"
	updated := appendPortfolioChildIssues(body, []string{"StatPan/docs#2"})
	if !strings.Contains(updated, "StatPan/gira#1") || !strings.Contains(updated, "StatPan/docs#2") {
		t.Fatalf("updated body missing child issues:\n%s", updated)
	}
	if !strings.Contains(updated, "## Non Goals\nN") {
		t.Fatalf("updated body did not preserve following section:\n%s", updated)
	}
}

func TestAppendPortfolioChildIssuesUpdatesNormalizedHeadingAndPreservesOrder(t *testing.T) {
	body := "## Goal\nG\n\n### Child-Issues\nStatPan/zeta#9\nStatPan/alpha#1\n\n## Non Goals\nN\n"
	updated := appendPortfolioChildIssues(body, []string{"StatPan/docs#2"})
	if strings.Count(updated, "Child") != 1 {
		t.Fatalf("updated body duplicated child heading:\n%s", updated)
	}
	if !strings.Contains(updated, "StatPan/zeta#9\nStatPan/alpha#1\nStatPan/docs#2") {
		t.Fatalf("updated body did not preserve append order:\n%s", updated)
	}
}

func TestAppendPortfolioChildIssuesUpdatesEmptyHeading(t *testing.T) {
	body := "## Goal\nG\n\n### Child Issues\n\n## Non Goals\nN\n"
	updated := appendPortfolioChildIssues(body, []string{"StatPan/docs#2"})
	if strings.Count(updated, "Child Issues") != 1 {
		t.Fatalf("updated body duplicated child heading:\n%s", updated)
	}
	if !strings.Contains(updated, "### Child Issues\n\nStatPan/docs#2\n## Non Goals") {
		t.Fatalf("updated body did not fill empty child section:\n%s", updated)
	}
}

type fakePortfolioLowerClient struct {
	existing     map[string][]PortfolioLoweredIssue
	created      []PortfolioLoweredIssue
	updated      map[int][]string
	failOnSearch bool
	searchCalls  int
}

func (c *fakePortfolioLowerClient) SearchLoweredIssues(repo RepoRef, portfolioRepo RepoRef, ticket int) ([]PortfolioLoweredIssue, error) {
	c.searchCalls++
	if c.failOnSearch {
		return nil, errors.New("unexpected search")
	}
	if c.existing == nil {
		return nil, nil
	}
	return c.existing[repo.FullName()+":"+itoaForPortfolioLower(ticket)], nil
}

func (c *fakePortfolioLowerClient) CreateExecutionIssue(repo RepoRef, ticket PortfolioTicket, portfolioRepo RepoRef) (PortfolioLoweredIssue, error) {
	issue := PortfolioLoweredIssue{Repo: repo.FullName(), Number: 100 + len(c.created), URL: "https://github.com/" + repo.FullName() + "/issues/" + itoaForPortfolioLower(100+len(c.created))}
	c.created = append(c.created, issue)
	return issue, nil
}

func (c *fakePortfolioLowerClient) UpdatePortfolioChildIssues(ticket PortfolioTicket, childIssues []string) error {
	if c.updated == nil {
		c.updated = map[int][]string{}
	}
	c.updated[ticket.Number] = append(c.updated[ticket.Number], childIssues...)
	return nil
}

func itoaForPortfolioLower(value int) string {
	return strconv.Itoa(value)
}

func allowAllPortfolioLowerCapability() PortfolioCapabilityReport {
	return PortfolioCapabilityReport{
		PortfolioRepo: "StatPan/portfolio",
		Repos: []PortfolioRepoCapability{
			{Repo: "StatPan/portfolio", Role: "portfolio", Capabilities: map[string]ProjectCapabilityStatus{"issues:write": ProjectCapabilityAllowed}},
			{Repo: "StatPan/gira", Role: "execution", Capabilities: map[string]ProjectCapabilityStatus{"issues:read": ProjectCapabilityAllowed, "issues:write": ProjectCapabilityAllowed}},
			{Repo: "StatPan/docs", Role: "execution", Capabilities: map[string]ProjectCapabilityStatus{"issues:read": ProjectCapabilityAllowed, "issues:write": ProjectCapabilityAllowed}},
		},
	}
}
