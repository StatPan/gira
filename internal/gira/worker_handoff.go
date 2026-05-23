package gira

import (
	"fmt"
	"strings"
	"time"
)

const (
	WorkerHandoffSchemaVersion      = "worker-handoff/v1"
	WorkerStateHandoffSchemaVersion = "worker-state-handoff/v1"
)

type TicketHandoffInput struct {
	Repo    RepoRef `json:"repo"`
	Ticket  int     `json:"ticket"`
	Role    string  `json:"role"`
	Profile string  `json:"profile"`
}

type TicketHandoffReport struct {
	Command                string                    `json:"command"`
	SchemaVersion          string                    `json:"schema_version"`
	Role                   string                    `json:"role"`
	Profile                string                    `json:"profile"`
	Repo                   string                    `json:"repo"`
	Issue                  int                       `json:"issue"`
	IssueURL               string                    `json:"issue_url"`
	Title                  string                    `json:"title"`
	State                  string                    `json:"state"`
	Labels                 []string                  `json:"labels,omitempty"`
	Readiness              TicketReadinessReport     `json:"readiness"`
	WorkOrder              TicketHandoffWorkOrder    `json:"work_order"`
	BranchPolicy           TicketHandoffBranchPolicy `json:"branch_policy"`
	RiskHints              []string                  `json:"risk_hints,omitempty"`
	EvidenceExpectations   []string                  `json:"evidence_expectations"`
	RequiredChecks         []string                  `json:"required_checks"`
	ReviewExpectations     []string                  `json:"review_expectations"`
	ProhibitedActions      []string                  `json:"prohibited_actions"`
	TelemetryExpectations  TicketHandoffTelemetry    `json:"telemetry_expectations"`
	ProvenanceExpectations TicketHandoffProvenance   `json:"provenance_expectations"`
	Guidance               []AgentPromptGuidance     `json:"guidance,omitempty"`
	NextAction             string                    `json:"next_action"`
	NextSafeCommand        string                    `json:"next_safe_command"`
}

type TicketHandoffWorkOrder struct {
	Goal       string   `json:"goal,omitempty"`
	Scope      string   `json:"scope,omitempty"`
	Acceptance []string `json:"acceptance,omitempty"`
}

type TicketHandoffBranchPolicy struct {
	Base        string   `json:"base,omitempty"`
	Source      string   `json:"source,omitempty"`
	Mode        string   `json:"mode,omitempty"`
	Target      string   `json:"target,omitempty"`
	WorkBranch  string   `json:"work_branch,omitempty"`
	Diagnostics []string `json:"diagnostics,omitempty"`
}

type TicketHandoffTelemetry struct {
	Required bool     `json:"required"`
	Present  bool     `json:"present"`
	Status   string   `json:"status"`
	Sources  []string `json:"sources,omitempty"`
}

type TicketHandoffProvenance struct {
	Optional    bool     `json:"optional"`
	Recommended bool     `json:"recommended"`
	Fields      []string `json:"fields"`
}

type WorkerClaim struct {
	Repo          string    `json:"repo"`
	IssueNumber   int       `json:"issue_number"`
	Worker        string    `json:"worker"`
	LeaseUntilUTC time.Time `json:"lease_until_utc"`
	Version       string    `json:"version"`
}

type WorkerHandoffPayload struct {
	SchemaVersion        string   `json:"schema_version"`
	Goal                 string   `json:"goal"`
	Context              string   `json:"context"`
	AcceptanceCriteria   []string `json:"acceptance_criteria"`
	VerificationCommands []string `json:"verification_commands"`
	RollbackNotes        string   `json:"rollback_notes"`
}

func ValidateWorkerHandoffPayload(payload WorkerHandoffPayload) error {
	if payload.SchemaVersion != WorkerStateHandoffSchemaVersion {
		return fmt.Errorf("invalid handoff schema version")
	}
	if strings.TrimSpace(payload.Goal) == "" {
		return fmt.Errorf("missing goal")
	}
	if strings.TrimSpace(payload.Context) == "" {
		return fmt.Errorf("missing context")
	}
	if len(payload.AcceptanceCriteria) == 0 {
		return fmt.Errorf("missing acceptance criteria")
	}
	if len(payload.VerificationCommands) == 0 {
		return fmt.Errorf("missing verification commands")
	}
	if strings.TrimSpace(payload.RollbackNotes) == "" {
		return fmt.Errorf("missing rollback notes")
	}
	return nil
}

func IsLeaseActive(now time.Time, claim WorkerClaim) bool {
	return claim.LeaseUntilUTC.After(now.UTC())
}

func BuildTicketHandoffReport(input TicketHandoffInput, runner CommandRunner) (TicketHandoffReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	role, err := normalizeAgentPromptRole(input.Role)
	if err != nil {
		return TicketHandoffReport{}, err
	}
	profile, err := normalizeAgentPromptProfile(input.Profile)
	if err != nil {
		return TicketHandoffReport{}, err
	}
	if input.Ticket <= 0 {
		return TicketHandoffReport{}, fmt.Errorf("ticket must be > 0")
	}
	issue, err := fetchDevIssue(input.Repo, input.Ticket, runner)
	if err != nil {
		return TicketHandoffReport{}, err
	}
	if issue.IsPR {
		return TicketHandoffReport{}, fmt.Errorf("ticket #%d resolves to a pull request", input.Ticket)
	}

	readiness := EvaluateTicketReadiness(issue.Body, issue.Labels, issue.State)
	telemetry := ticketStatusTelemetry(issue.Body, issue.Labels)
	report := TicketHandoffReport{
		Command:       "ticket handoff",
		SchemaVersion: WorkerHandoffSchemaVersion,
		Role:          role,
		Profile:       profile,
		Repo:          input.Repo.FullName(),
		Issue:         issue.Number,
		IssueURL:      fmt.Sprintf("https://github.com/%s/issues/%d", input.Repo.FullName(), issue.Number),
		Title:         issue.Title,
		State:         issue.State,
		Labels:        append([]string(nil), issue.Labels...),
		Readiness:     readiness,
		WorkOrder: TicketHandoffWorkOrder{
			Goal:       markdownSection(issue.Body, "Goal"),
			Scope:      markdownSection(issue.Body, "Scope"),
			Acceptance: ticketReadinessAcceptanceItems(issue.Body),
		},
		BranchPolicy:           buildTicketHandoffBranchPolicy(input.Repo, issue, runner),
		RiskHints:              agentPromptRiskSignals(issue.Labels),
		EvidenceExpectations:   ticketHandoffEvidenceExpectations(issue.Body),
		RequiredChecks:         ticketHandoffRequiredChecks(issue.Body),
		ReviewExpectations:     ticketHandoffReviewExpectations(issue.Number),
		ProhibitedActions:      ticketHandoffProhibitedActions(),
		TelemetryExpectations:  TicketHandoffTelemetry{Required: telemetry.Required, Present: telemetry.Present, Status: telemetry.Status, Sources: append([]string(nil), telemetry.Sources...)},
		ProvenanceExpectations: ticketHandoffProvenanceExpectations(),
		Guidance:               loadAgentPromptGuidance(),
	}
	report.NextAction, report.NextSafeCommand = ticketHandoffNextStep(input.Repo, issue, role, readiness)
	return report, nil
}

func buildTicketHandoffBranchPolicy(repo RepoRef, issue devStartIssue, runner CommandRunner) TicketHandoffBranchPolicy {
	state := ParseTicketLifecycleState(issue.Body)
	policy := TicketHandoffBranchPolicy{
		Base:       strings.TrimSpace(state.BaseBranch),
		Source:     strings.TrimSpace(state.BaseSource),
		Mode:       strings.TrimSpace(state.BranchPolicyMode),
		Target:     strings.TrimSpace(state.Target),
		WorkBranch: strings.TrimSpace(state.WorkBranch),
	}
	if policy.WorkBranch == "" {
		policy.WorkBranch = formatDevBranch(DefaultDevBranchPattern, issue.Number, issue.Title)
	}
	if policy.Base != "" {
		return policy
	}
	resolved, err := resolveTicketStartBase(repo, issue, "", runner)
	if err != nil {
		policy.Diagnostics = append(policy.Diagnostics, "base_resolution_failed: "+err.Error())
		return policy
	}
	policy.Base = resolved.BaseBranch
	policy.Source = resolved.BaseSource
	policy.Mode = resolved.PolicyMode
	policy.Target = resolved.Target
	return policy
}

func ticketHandoffEvidenceExpectations(body string) []string {
	items := markdownListSection(body, "Expected Evidence")
	if len(items) > 0 {
		return items
	}
	items = markdownListSection(body, "Verification")
	if len(items) > 0 {
		return items
	}
	return []string{
		"implementation summary in the PR body",
		"verification commands and results",
		"changed-file scope matches the ticket work order",
		"AI Delivery Telemetry or provenance metadata when agent-assisted work is used",
	}
}

func ticketHandoffRequiredChecks(body string) []string {
	items := append([]string{}, markdownListSection(body, "Expected Evidence")...)
	items = append(items, markdownListSection(body, "Verification")...)
	checks := []string{}
	for _, item := range items {
		lower := strings.ToLower(item)
		for _, needle := range []string{"go test", "pytest", "npm test", "pnpm test", "bun test", "make ", "sh ", "scripts/"} {
			if strings.Contains(lower, needle) {
				checks = append(checks, item)
				break
			}
		}
	}
	if len(checks) > 0 {
		return checks
	}
	return []string{"discover and run the repo's relevant focused checks before PR handoff"}
}

func ticketHandoffReviewExpectations(issue int) []string {
	return []string{
		fmt.Sprintf("PR body contains `Closes #%d`, `Fixes #%d`, or `Resolves #%d`", issue, issue, issue),
		"PR summary maps behavior changes to the ticket goal and acceptance criteria",
		"PR evidence lists verification commands, caveats, and any residual risk",
		"reviewers can run `gira ticket review --json` and `gira ticket checks` against the linked PR",
	}
}

func ticketHandoffProhibitedActions() []string {
	return []string{
		"do not broaden scope beyond the ticket work order without a human decision",
		"do not run destructive git or filesystem operations without explicit approval",
		"do not bypass Gira lifecycle commands for start, PR, checks, and finish when a Gira command exists",
		"do not treat Gira as the worker runtime; external tools execute the work and Gira compiles/contracts it",
	}
}

func ticketHandoffProvenanceExpectations() TicketHandoffProvenance {
	return TicketHandoffProvenance{
		Optional:    true,
		Recommended: true,
		Fields: []string{
			"trace_id",
			"span_id",
			"worker_id",
			"attempt_id",
			"implementation_tool",
			"implementation_model",
			"review_tool",
			"review_model",
			"prompt_source",
			"human_interventions",
			"completed_at",
		},
	}
}

func ticketHandoffNextStep(repo RepoRef, issue devStartIssue, role string, readiness TicketReadinessReport) (string, string) {
	if readiness.NextAction == "refine_ticket" || readiness.NextAction == "blocked" || readiness.NextAction == "ask_human" {
		return readiness.NextAction, fmt.Sprintf("gira ticket view --repo %s --ticket %d", repo.FullName(), issue.Number)
	}
	switch role {
	case AgentPromptRolePlanner:
		return "plan", fmt.Sprintf("gira ticket prompt --repo %s --ticket %d --role planner --json", repo.FullName(), issue.Number)
	case AgentPromptRoleReviewer:
		return "request_review", fmt.Sprintf("gira ticket review --repo %s --ticket %d --json", repo.FullName(), issue.Number)
	default:
		status := displayStatus(managedStatusFromLabels(issue.Labels))
		if strings.EqualFold(status, "In progress") || strings.EqualFold(status, "In review") {
			return "implement", fmt.Sprintf("gira ticket pr --repo %s --ticket %d --dry-run", repo.FullName(), issue.Number)
		}
		return "start_ticket", fmt.Sprintf("gira ticket start --repo %s --ticket %d --dry-run", repo.FullName(), issue.Number)
	}
}

func FormatTicketHandoff(report TicketHandoffReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ticket handoff: #%d role=%s readiness=%s next=%s\n", report.Issue, report.Role, report.Readiness.Readiness, report.NextAction)
	fmt.Fprintf(&b, "repo: %s\n", report.Repo)
	fmt.Fprintf(&b, "issue: %s\n", report.IssueURL)
	fmt.Fprintf(&b, "branch: base=%s source=%s work=%s\n", valueOrUnknown(report.BranchPolicy.Base), valueOrUnknown(report.BranchPolicy.Source), valueOrUnknown(report.BranchPolicy.WorkBranch))
	if strings.TrimSpace(report.WorkOrder.Goal) != "" {
		fmt.Fprintf(&b, "goal: %s\n", strings.TrimSpace(report.WorkOrder.Goal))
	}
	if len(report.WorkOrder.Acceptance) > 0 {
		fmt.Fprintf(&b, "acceptance: %d item(s)\n", len(report.WorkOrder.Acceptance))
	}
	if len(report.Readiness.Findings) > 0 {
		b.WriteString("readiness findings:\n")
		for _, finding := range report.Readiness.Findings {
			fmt.Fprintf(&b, "- %s:%s %s\n", finding.Severity, finding.Kind, finding.Message)
		}
	}
	fmt.Fprintf(&b, "next safe command: %s\n", report.NextSafeCommand)
	return b.String()
}
