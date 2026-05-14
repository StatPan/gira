package gira

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const (
	AgentPromptRolePlanner     = "planner"
	AgentPromptRoleImplementer = "implementer"
	AgentPromptRoleReviewer    = "reviewer"

	AgentPromptProfileDefault = "default"
	AgentPromptProfilePython  = "python"
)

type AgentPromptInput struct {
	Repo     RepoRef `json:"repo"`
	Ticket   int     `json:"ticket"`
	Role     string  `json:"role"`
	Profile  string  `json:"profile"`
	PRNumber int     `json:"pr_number,omitempty"`
}

type AgentPromptReport struct {
	Command  string           `json:"command"`
	Repo     string           `json:"repo"`
	Ticket   int              `json:"ticket"`
	Role     string           `json:"role"`
	Profile  string           `json:"profile"`
	Issue    AgentPromptIssue `json:"issue"`
	PR       *AgentPromptPR   `json:"pr,omitempty"`
	Prompt   string           `json:"prompt"`
	NextStep string           `json:"next_step"`
}

type AgentPromptIssue struct {
	Number int      `json:"number"`
	Title  string   `json:"title"`
	State  string   `json:"state"`
	Body   string   `json:"body"`
	Labels []string `json:"labels"`
}

type AgentPromptPR struct {
	Number         int          `json:"number"`
	Title          string       `json:"title,omitempty"`
	Body           string       `json:"body,omitempty"`
	State          string       `json:"state,omitempty"`
	URL            string       `json:"url,omitempty"`
	ReviewDecision string       `json:"review_decision,omitempty"`
	IsDraft        bool         `json:"is_draft,omitempty"`
	MergeState     string       `json:"merge_state,omitempty"`
	Blockers       []string     `json:"blockers,omitempty"`
	Checks         []DevPRCheck `json:"checks,omitempty"`
}

func BuildAgentPromptReport(input AgentPromptInput, runner CommandRunner) (AgentPromptReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	role, err := normalizeAgentPromptRole(input.Role)
	if err != nil {
		return AgentPromptReport{}, err
	}
	profile, err := normalizeAgentPromptProfile(input.Profile)
	if err != nil {
		return AgentPromptReport{}, err
	}
	if input.Ticket <= 0 {
		return AgentPromptReport{}, fmt.Errorf("ticket must be > 0")
	}
	issue, err := fetchDevIssue(input.Repo, input.Ticket, runner)
	if err != nil {
		return AgentPromptReport{}, err
	}
	if issue.IsPR {
		return AgentPromptReport{}, fmt.Errorf("ticket #%d resolves to a pull request", input.Ticket)
	}
	report := AgentPromptReport{
		Command: "ticket prompt",
		Repo:    input.Repo.FullName(),
		Ticket:  input.Ticket,
		Role:    role,
		Profile: profile,
		Issue: AgentPromptIssue{
			Number: issue.Number,
			Title:  issue.Title,
			State:  issue.State,
			Body:   issue.Body,
			Labels: append([]string(nil), issue.Labels...),
		},
		NextStep: agentPromptNextStep(input.Repo, input.Ticket, role),
	}
	if role == AgentPromptRoleReviewer {
		pr, err := resolveAgentPromptPR(input.Repo, input.Ticket, input.PRNumber, runner)
		if err != nil {
			return report, err
		}
		report.PR = pr
	}
	report.Prompt = RenderAgentPrompt(report)
	return report, nil
}

func RenderAgentPrompt(report AgentPromptReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Gira %s prompt\n\n", report.Role)
	fmt.Fprintf(&b, "Repository: `%s`\n", report.Repo)
	fmt.Fprintf(&b, "Ticket: `#%d` %s\n", report.Ticket, report.Issue.Title)
	fmt.Fprintf(&b, "Role: `%s`\n", report.Role)
	fmt.Fprintf(&b, "Profile: `%s`\n\n", report.Profile)

	b.WriteString("## Operating Rules\n")
	for _, rule := range agentPromptRoleRules(report.Role) {
		fmt.Fprintf(&b, "- %s\n", rule)
	}
	for _, rule := range agentPromptProfileRules(report.Profile) {
		fmt.Fprintf(&b, "- %s\n", rule)
	}
	b.WriteString("\n")

	b.WriteString("## Ticket Context\n")
	fmt.Fprintf(&b, "- State: `%s`\n", valueOrUnknown(report.Issue.State))
	fmt.Fprintf(&b, "- Labels: %s\n\n", valueOrNone(strings.Join(report.Issue.Labels, ", ")))
	fmt.Fprintf(&b, "### Issue Body\n%s\n\n", fencedOrNone(report.Issue.Body))

	if report.PR != nil {
		b.WriteString("## Pull Request Context\n")
		fmt.Fprintf(&b, "- PR: `#%d`\n", report.PR.Number)
		if strings.TrimSpace(report.PR.Title) != "" {
			fmt.Fprintf(&b, "- Title: %s\n", report.PR.Title)
		}
		if strings.TrimSpace(report.PR.State) != "" {
			fmt.Fprintf(&b, "- State: `%s`\n", report.PR.State)
		}
		if strings.TrimSpace(report.PR.URL) != "" {
			fmt.Fprintf(&b, "- URL: %s\n", report.PR.URL)
		}
		if len(report.PR.Blockers) > 0 {
			fmt.Fprintf(&b, "- Blockers: %s\n", strings.Join(report.PR.Blockers, ", "))
		}
		if strings.TrimSpace(report.PR.Body) != "" {
			fmt.Fprintf(&b, "\n### PR Body\n%s\n", fencedOrNone(report.PR.Body))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Expected Output\n")
	for _, item := range agentPromptExpectedOutput(report.Role) {
		fmt.Fprintf(&b, "- %s\n", item)
	}
	fmt.Fprintf(&b, "\nNext Gira step: `%s`\n", report.NextStep)
	return b.String()
}

func FormatAgentPrompt(report AgentPromptReport) string {
	return strings.TrimRight(report.Prompt, "\n") + "\n"
}

func normalizeAgentPromptRole(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case AgentPromptRolePlanner, AgentPromptRoleImplementer, AgentPromptRoleReviewer:
		return value, nil
	default:
		return "", fmt.Errorf("--role must be one of planner, implementer, reviewer")
	}
}

func normalizeAgentPromptProfile(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = AgentPromptProfileDefault
	}
	switch value {
	case AgentPromptProfileDefault, AgentPromptProfilePython:
		return value, nil
	default:
		return "", fmt.Errorf("--profile must be one of default, python")
	}
}

func resolveAgentPromptPR(repo RepoRef, ticket int, prNumber int, runner CommandRunner) (*AgentPromptPR, error) {
	if prNumber > 0 {
		pr, err := fetchAgentPromptPR(repo, prNumber, runner)
		if err != nil {
			return nil, err
		}
		return &pr, nil
	}
	status, err := DevPRStatus(repo, ticket, runner)
	if err != nil {
		return nil, err
	}
	if status.PRNumber == 0 {
		return nil, nil
	}
	return &AgentPromptPR{
		Number:     status.PRNumber,
		State:      status.State,
		URL:        status.PRURL,
		MergeState: status.Mergeable,
		Blockers:   append([]string(nil), status.Blockers...),
		Checks:     append([]DevPRCheck(nil), status.Checks...),
	}, nil
}

func fetchAgentPromptPR(repo RepoRef, prNumber int, runner CommandRunner) (AgentPromptPR, error) {
	out, err := runner.Run("gh", "pr", "view", strconv.Itoa(prNumber), "--repo", repo.FullName(), "--json", "number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup")
	if err != nil {
		return AgentPromptPR{}, fmt.Errorf("fetch pr: %w", err)
	}
	var raw prSummary
	if err := json.Unmarshal(out, &raw); err != nil {
		return AgentPromptPR{}, fmt.Errorf("parse pr JSON: %w", err)
	}
	if raw.Number <= 0 {
		return AgentPromptPR{}, fmt.Errorf("parse pr JSON: missing or invalid PR number")
	}
	checks := make([]DevPRCheck, 0, len(raw.StatusRollup))
	for _, check := range raw.StatusRollup {
		checks = append(checks, DevPRCheck{Name: check.Name, Workflow: check.Workflow, Status: check.Status, Conclusion: check.Conclusion, URL: check.URL, State: classifyDevPRCheck(check.Status, check.Conclusion)})
	}
	return AgentPromptPR{
		Number:         raw.Number,
		Title:          raw.Title,
		Body:           raw.Body,
		State:          raw.State,
		URL:            raw.URL,
		ReviewDecision: raw.ReviewDecision,
		IsDraft:        raw.IsDraft,
		MergeState:     raw.MergeState,
		Checks:         checks,
	}, nil
}

func agentPromptRoleRules(role string) []string {
	switch role {
	case AgentPromptRolePlanner:
		return []string{
			"Treat the GitHub issue as the source of truth.",
			"Plan the issue into queue-ready tasks with explicit goals, scope, acceptance criteria, verification, and dependencies.",
			"Do not implement code or claim completion from this prompt.",
			"Keep each proposed task small enough for a stateless implementation worker.",
		}
	case AgentPromptRoleImplementer:
		return []string{
			"Assume no prior chat state; use only the repository, issue, and prompt context.",
			"Keep changes bounded to the target ticket and avoid unrelated refactors.",
			"Prefer existing repo patterns, helpers, and documented commands.",
			"Report changed files, verification commands, and blockers before handing off.",
			"Any PR body must include a closing reference to the target ticket, such as Closes #N.",
		}
	case AgentPromptRoleReviewer:
		return []string{
			"Review findings first, ordered by severity, with file and line references where available.",
			"Prioritize bugs, regressions, missing tests, security, data loss, and operational risks.",
			"Do not lead with a summary before findings.",
			"Call out residual test gaps even when no blocking findings remain.",
		}
	default:
		return nil
	}
}

func agentPromptProfileRules(profile string) []string {
	switch profile {
	case AgentPromptProfilePython:
		return []string{
			"Python profile: follow the repository's existing package manager, lockfile, and virtualenv conventions.",
			"Python profile: run pytest when tests are configured; run ruff and mypy only when configured by the repo.",
			"Python profile: check pyproject.toml, dependency metadata, and lockfile impact when dependencies or packaging change.",
			"Python profile: prefer targeted tests first, then broaden to the repo's normal test command when risk warrants it.",
		}
	default:
		return []string{
			"Default profile: discover verification commands from repo docs, manifests, scripts, and CI before inventing commands.",
			"Default profile: prefer focused checks for narrow changes and broader checks for shared behavior.",
		}
	}
}

func agentPromptExpectedOutput(role string) []string {
	switch role {
	case AgentPromptRolePlanner:
		return []string{
			"A task breakdown with one clear goal per task.",
			"Acceptance criteria and verification commands for each task.",
			"Dependencies, sequencing, and any human decisions required.",
		}
	case AgentPromptRoleImplementer:
		return []string{
			"Implementation summary focused on behavior changed.",
			"Files changed and verification commands run.",
			"Blockers or follow-up issues, if any.",
		}
	case AgentPromptRoleReviewer:
		return []string{
			"Findings first; say clearly if no findings are found.",
			"Open questions or assumptions.",
			"Test gaps and residual risk.",
		}
	default:
		return nil
	}
}

func agentPromptNextStep(repo RepoRef, ticket int, role string) string {
	switch role {
	case AgentPromptRolePlanner:
		return fmt.Sprintf("gira ticket view --repo %s --ticket %d", repo.FullName(), ticket)
	case AgentPromptRoleReviewer:
		return fmt.Sprintf("gira ticket checks --repo %s --ticket %d", repo.FullName(), ticket)
	default:
		return fmt.Sprintf("gira ticket pr --repo %s --ticket %d --dry-run", repo.FullName(), ticket)
	}
}

func fencedOrNone(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "_No response_"
	}
	return "```markdown\n" + value + "\n```"
}

func valueOrUnknown(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}
