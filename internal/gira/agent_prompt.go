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
	Command  string               `json:"command"`
	Repo     string               `json:"repo"`
	Ticket   int                  `json:"ticket"`
	Role     string               `json:"role"`
	Profile  string               `json:"profile"`
	Issue    AgentPromptIssue     `json:"issue"`
	PR       *AgentPromptPR       `json:"pr,omitempty"`
	Evidence *AgentPromptEvidence `json:"evidence,omitempty"`
	Prompt   string               `json:"prompt"`
	NextStep string               `json:"next_step"`
}

type AgentPromptIssue struct {
	Number     int      `json:"number"`
	Title      string   `json:"title"`
	State      string   `json:"state"`
	Body       string   `json:"body"`
	Labels     []string `json:"labels"`
	Goal       string   `json:"goal,omitempty"`
	Scope      string   `json:"scope,omitempty"`
	Acceptance []string `json:"acceptance,omitempty"`
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
	ChangedFiles   []string     `json:"changed_files,omitempty"`
	FinishReady    bool         `json:"finish_ready"`
}

type AgentPromptEvidence struct {
	ClosingIssues []int        `json:"closing_issues,omitempty"`
	Checks        []DevPRCheck `json:"checks,omitempty"`
	Blockers      []string     `json:"blockers,omitempty"`
	ChangedFiles  []string     `json:"changed_files,omitempty"`
	FinishReady   bool         `json:"finish_ready"`
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
	report.Issue.Goal = markdownSection(issue.Body, "Goal")
	report.Issue.Scope = markdownSection(issue.Body, "Scope")
	report.Issue.Acceptance = markdownListSection(issue.Body, "Acceptance Criteria")
	if role == AgentPromptRoleReviewer {
		pr, err := resolveAgentPromptPR(input.Repo, input.Ticket, input.PRNumber, runner)
		if err != nil {
			return report, err
		}
		report.PR = pr
		if pr != nil {
			report.Evidence = agentPromptEvidence(pr)
		}
	}
	report.Prompt = RenderAgentPrompt(report)
	return report, nil
}

func agentPromptEvidence(pr *AgentPromptPR) *AgentPromptEvidence {
	return &AgentPromptEvidence{
		ClosingIssues: ExtractClosureIssueNumbers(pr.Body),
		Checks:        append([]DevPRCheck(nil), pr.Checks...),
		Blockers:      append([]string(nil), pr.Blockers...),
		ChangedFiles:  append([]string(nil), pr.ChangedFiles...),
		FinishReady:   pr.FinishReady,
	}
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
	if strings.TrimSpace(report.Issue.Goal) != "" {
		fmt.Fprintf(&b, "- Goal: %s\n", report.Issue.Goal)
	}
	if strings.TrimSpace(report.Issue.Scope) != "" {
		fmt.Fprintf(&b, "- Scope: %s\n", report.Issue.Scope)
	}
	if len(report.Issue.Acceptance) > 0 {
		b.WriteString("- Acceptance Criteria:\n")
		for _, item := range report.Issue.Acceptance {
			fmt.Fprintf(&b, "  - %s\n", item)
		}
	}
	if strings.TrimSpace(report.Issue.Goal) != "" || strings.TrimSpace(report.Issue.Scope) != "" || len(report.Issue.Acceptance) > 0 {
		b.WriteString("\n")
	}
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
		if strings.TrimSpace(report.PR.ReviewDecision) != "" {
			fmt.Fprintf(&b, "- Review Decision: `%s`\n", report.PR.ReviewDecision)
		}
		if strings.TrimSpace(report.PR.MergeState) != "" {
			fmt.Fprintf(&b, "- Merge State: `%s`\n", report.PR.MergeState)
		}
		fmt.Fprintf(&b, "- Draft: `%t`\n", report.PR.IsDraft)
		fmt.Fprintf(&b, "- Finish Ready: `%t`\n", report.PR.FinishReady)
		if len(report.PR.Blockers) > 0 {
			fmt.Fprintf(&b, "- Blockers: %s\n", strings.Join(report.PR.Blockers, ", "))
		}
		if len(report.PR.Checks) > 0 {
			b.WriteString("- Checks:\n")
			for _, check := range report.PR.Checks {
				name := valueOrUnknown(check.Name)
				if strings.TrimSpace(check.Workflow) != "" {
					name = check.Workflow + "/" + name
				}
				fmt.Fprintf(&b, "  - %s: %s\n", name, valueOrUnknown(check.State))
			}
		}
		if len(report.PR.ChangedFiles) > 0 {
			b.WriteString("- Changed Files:\n")
			for _, file := range report.PR.ChangedFiles {
				fmt.Fprintf(&b, "  - `%s`\n", file)
			}
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
		return nil, fmt.Errorf("reviewer prompt requires a linked PR for ticket #%d; run `gira ticket pr --repo %s --ticket %d --dry-run`", ticket, repo.FullName(), ticket)
	}
	pr, err := fetchAgentPromptPR(repo, status.PRNumber, runner)
	if err != nil {
		return nil, err
	}
	pr.Blockers = append([]string(nil), status.Blockers...)
	pr.Checks = append([]DevPRCheck(nil), status.Checks...)
	pr.FinishReady = status.Ready
	return &pr, nil
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
	changedFiles := fetchAgentPromptChangedFiles(repo, raw.Number, runner)
	blockers := agentPromptPRBlockers(raw, checks)
	return AgentPromptPR{
		Number:         raw.Number,
		Title:          raw.Title,
		Body:           raw.Body,
		State:          raw.State,
		URL:            raw.URL,
		ReviewDecision: raw.ReviewDecision,
		IsDraft:        raw.IsDraft,
		MergeState:     raw.MergeState,
		Blockers:       blockers,
		Checks:         checks,
		ChangedFiles:   changedFiles,
		FinishReady:    len(blockers) == 0,
	}, nil
}

func fetchAgentPromptChangedFiles(repo RepoRef, prNumber int, runner CommandRunner) []string {
	out, err := runner.Run("gh", "pr", "diff", strconv.Itoa(prNumber), "--repo", repo.FullName(), "--name-only")
	if err != nil {
		return nil
	}
	files := []string{}
	for _, line := range strings.Split(string(out), "\n") {
		file := strings.TrimSpace(line)
		if file != "" {
			files = append(files, file)
		}
	}
	return files
}

func agentPromptPRBlockers(raw prSummary, checks []DevPRCheck) []string {
	blockers := []string{}
	if raw.IsDraft {
		blockers = append(blockers, "draft")
	}
	if raw.ReviewDecision == "CHANGES_REQUESTED" || raw.ReviewDecision == "REVIEW_REQUIRED" {
		blockers = append(blockers, "review")
	}
	for _, check := range checks {
		switch check.State {
		case "failing":
			return append(blockers, "checks")
		case "pending":
			return append(blockers, "checks_pending")
		}
	}
	return blockers
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

func markdownSection(body string, heading string) string {
	lines := strings.Split(body, "\n")
	inSection := false
	values := []string{}
	target := strings.ToLower(strings.TrimSpace(heading))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			current := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")))
			if inSection {
				break
			}
			inSection = current == target
			continue
		}
		if inSection {
			values = append(values, line)
		}
	}
	return strings.TrimSpace(strings.Join(values, "\n"))
}

func markdownListSection(body string, heading string) []string {
	section := markdownSection(body, heading)
	items := []string{}
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			item := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			if item != "" {
				items = append(items, item)
			}
		}
	}
	return items
}
