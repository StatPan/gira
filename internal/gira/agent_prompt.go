package gira

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	Command  string                 `json:"command"`
	Repo     string                 `json:"repo"`
	Ticket   int                    `json:"ticket"`
	Role     string                 `json:"role"`
	Profile  string                 `json:"profile"`
	Issue    AgentPromptIssue       `json:"issue"`
	PR       *AgentPromptPR         `json:"pr,omitempty"`
	Evidence *AgentPromptEvidence   `json:"evidence,omitempty"`
	Packet   *AgentPromptRolePacket `json:"packet,omitempty"`
	PRReady  *PRReadinessReport     `json:"pr_readiness,omitempty"`
	Review   *AgentReviewContract   `json:"review,omitempty"`
	Prompt   string                 `json:"prompt"`
	NextStep string                 `json:"next_step"`
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
	Number             int          `json:"number"`
	Title              string       `json:"title,omitempty"`
	Body               string       `json:"body,omitempty"`
	State              string       `json:"state,omitempty"`
	URL                string       `json:"url,omitempty"`
	HeadRefName        string       `json:"head_ref_name,omitempty"`
	BaseRefName        string       `json:"base_ref_name,omitempty"`
	RecordedBase       string       `json:"recorded_base,omitempty"`
	RecordedBaseSource string       `json:"recorded_base_source,omitempty"`
	BaseMismatch       bool         `json:"base_mismatch,omitempty"`
	ReviewDecision     string       `json:"review_decision,omitempty"`
	IsDraft            bool         `json:"is_draft,omitempty"`
	MergeState         string       `json:"merge_state,omitempty"`
	Blockers           []string     `json:"blockers,omitempty"`
	Checks             []DevPRCheck `json:"checks,omitempty"`
	ChangedFiles       []string     `json:"changed_files,omitempty"`
	FinishReady        bool         `json:"finish_ready"`
}

type AgentPromptEvidence struct {
	ClosingIssues []int        `json:"closing_issues,omitempty"`
	Checks        []DevPRCheck `json:"checks,omitempty"`
	Blockers      []string     `json:"blockers,omitempty"`
	ChangedFiles  []string     `json:"changed_files,omitempty"`
	FinishReady   bool         `json:"finish_ready"`
}

type AgentPromptRolePacket struct {
	Role             string                `json:"role"`
	Goal             string                `json:"goal,omitempty"`
	Scope            string                `json:"scope,omitempty"`
	Labels           []string              `json:"labels,omitempty"`
	Readiness        []string              `json:"readiness,omitempty"`
	WorkOrder        []string              `json:"work_order,omitempty"`
	Risk             []string              `json:"risk,omitempty"`
	ExpectedEvidence []string              `json:"expected_evidence,omitempty"`
	Guidance         []AgentPromptGuidance `json:"guidance,omitempty"`
}

type AgentReviewContract struct {
	DiffReferences []AgentReviewReference   `json:"diff_references"`
	Guidance       []AgentPromptGuidance    `json:"guidance"`
	VerdictSchema  AgentReviewVerdictSchema `json:"verdict_schema"`
}

type AgentReviewReference struct {
	Kind    string `json:"kind"`
	Command string `json:"command"`
}

type AgentPromptGuidance struct {
	Path    string `json:"path"`
	Status  string `json:"status"`
	Content string `json:"content,omitempty"`
}

type AgentReviewVerdictSchema struct {
	GoalFulfilled            []string `json:"goal_fulfilled"`
	AcceptanceCriteriaStatus []string `json:"acceptance_criteria_status"`
	ChecksStatus             []string `json:"checks_status"`
	EvidenceStatus           []string `json:"evidence_status"`
	ResidualRisk             []string `json:"residual_risk"`
	RecommendedAction        []string `json:"recommended_action"`
	ReviewerNotes            string   `json:"reviewer_notes"`
	TestGaps                 string   `json:"test_gaps"`
	FollowUps                string   `json:"follow_ups"`
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
	report.Packet = buildAgentPromptRolePacket(report, issue)
	if role == AgentPromptRoleReviewer {
		pr, err := resolveAgentPromptPR(input.Repo, input.Ticket, input.PRNumber, runner)
		if err != nil {
			if !isMissingLinkedPRPromptError(err) {
				return report, err
			}
			report.Evidence = &AgentPromptEvidence{Blockers: []string{"missing_linked_pr"}}
		} else if pr != nil {
			annotateAgentPromptPRBranchContext(pr, issue)
			report.PR = pr
			report.Evidence = agentPromptEvidence(pr)
		}
		prReady := EvaluatePRReadinessFromAgentReview(report)
		report.PRReady = &prReady
		report.Review = buildAgentReviewContract(report)
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

func annotateAgentPromptPRBranchContext(pr *AgentPromptPR, issue devStartIssue) {
	state := ParseTicketLifecycleState(issue.Body)
	recorded := strings.TrimSpace(state.BaseBranch)
	if recorded == "" {
		return
	}
	pr.RecordedBase = recorded
	pr.RecordedBaseSource = strings.TrimSpace(state.BaseSource)
	actual := strings.TrimSpace(pr.BaseRefName)
	if actual == "" || actual == recorded {
		return
	}
	pr.BaseMismatch = true
	pr.Blockers = appendMissingWorkBlocker(pr.Blockers, "pr_base_mismatch")
	pr.FinishReady = false
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
		if strings.TrimSpace(report.PR.HeadRefName) != "" {
			fmt.Fprintf(&b, "- Head: `%s`\n", report.PR.HeadRefName)
		}
		if strings.TrimSpace(report.PR.BaseRefName) != "" {
			fmt.Fprintf(&b, "- Base: `%s`\n", report.PR.BaseRefName)
		}
		if strings.TrimSpace(report.PR.RecordedBase) != "" {
			fmt.Fprintf(&b, "- Recorded Base: `%s`", report.PR.RecordedBase)
			if strings.TrimSpace(report.PR.RecordedBaseSource) != "" {
				fmt.Fprintf(&b, " (%s)", report.PR.RecordedBaseSource)
			}
			b.WriteString("\n")
			fmt.Fprintf(&b, "- Base Matches Recorded: `%t`\n", !report.PR.BaseMismatch)
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
		if report.Role == AgentPromptRoleReviewer {
			b.WriteString("\n## Review Evidence Commands\n")
			fmt.Fprintf(&b, "- Inspect the actual diff: `gh pr diff %d --repo %s`\n", report.PR.Number, report.Repo)
			fmt.Fprintf(&b, "- Inspect the changed file list: `gh pr diff %d --repo %s --name-only`\n", report.PR.Number, report.Repo)
			if strings.TrimSpace(report.PR.RecordedBase) != "" {
				fmt.Fprintf(&b, "- Verify the PR targets the recorded base `%s`, not only the current checkout or GitHub default branch.\n", report.PR.RecordedBase)
			}
		}
		if strings.TrimSpace(report.PR.Body) != "" {
			fmt.Fprintf(&b, "\n### PR Body\n%s\n", fencedOrNone(report.PR.Body))
		}
		b.WriteString("\n")
	}

	if report.PRReady != nil {
		b.WriteString("## PR Readiness\n")
		fmt.Fprintf(&b, "- Schema: `%s`\n", report.PRReady.SchemaVersion)
		fmt.Fprintf(&b, "- Readiness: `%s`\n", report.PRReady.Readiness)
		fmt.Fprintf(&b, "- Next Action: `%s`\n", report.PRReady.NextAction)
		if len(report.PRReady.Findings) > 0 {
			b.WriteString("- Findings:\n")
			for _, finding := range report.PRReady.Findings {
				fmt.Fprintf(&b, "  - %s/%s: %s\n", finding.Severity, finding.Kind, finding.Message)
			}
		}
		b.WriteString("\n")
	}

	if report.Packet != nil {
		b.WriteString("## Role Packet\n")
		if len(report.Packet.Readiness) > 0 {
			b.WriteString("- Readiness:\n")
			for _, item := range report.Packet.Readiness {
				fmt.Fprintf(&b, "  - %s\n", item)
			}
		}
		if len(report.Packet.WorkOrder) > 0 {
			b.WriteString("- Work Order:\n")
			for _, item := range report.Packet.WorkOrder {
				fmt.Fprintf(&b, "  - %s\n", item)
			}
		}
		if len(report.Packet.ExpectedEvidence) > 0 {
			b.WriteString("- Expected Evidence:\n")
			for _, item := range report.Packet.ExpectedEvidence {
				fmt.Fprintf(&b, "  - %s\n", item)
			}
		}
		if len(report.Packet.Risk) > 0 {
			b.WriteString("- Risk Signals:\n")
			for _, item := range report.Packet.Risk {
				fmt.Fprintf(&b, "  - %s\n", item)
			}
		}
		if len(report.Packet.Guidance) > 0 {
			b.WriteString("- Repo-local Guidance:\n")
			for _, guidance := range report.Packet.Guidance {
				fmt.Fprintf(&b, "  - `%s`: %s\n", guidance.Path, guidance.Status)
			}
		}
		b.WriteString("\n")
	}

	if report.Review != nil {
		b.WriteString("## Review Packet Contract\n")
		if len(report.Review.DiffReferences) > 0 {
			b.WriteString("- Diff References:\n")
			for _, ref := range report.Review.DiffReferences {
				fmt.Fprintf(&b, "  - %s: `%s`\n", ref.Kind, ref.Command)
			}
		}
		if len(report.Review.Guidance) > 0 {
			b.WriteString("- Repo-local Guidance:\n")
			for _, guidance := range report.Review.Guidance {
				fmt.Fprintf(&b, "  - `%s`: %s\n", guidance.Path, guidance.Status)
			}
		}
		b.WriteString("- Verdict Schema:\n")
		fmt.Fprintf(&b, "  - goal_fulfilled: %s\n", strings.Join(report.Review.VerdictSchema.GoalFulfilled, " / "))
		fmt.Fprintf(&b, "  - acceptance_criteria_status: %s\n", strings.Join(report.Review.VerdictSchema.AcceptanceCriteriaStatus, " / "))
		fmt.Fprintf(&b, "  - checks_status: %s\n", strings.Join(report.Review.VerdictSchema.ChecksStatus, " / "))
		fmt.Fprintf(&b, "  - evidence_status: %s\n", strings.Join(report.Review.VerdictSchema.EvidenceStatus, " / "))
		fmt.Fprintf(&b, "  - residual_risk: %s\n", strings.Join(report.Review.VerdictSchema.ResidualRisk, " / "))
		fmt.Fprintf(&b, "  - recommended_action: %s\n\n", strings.Join(report.Review.VerdictSchema.RecommendedAction, " / "))
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
	out, err := runner.Run("gh", "pr", "view", strconv.Itoa(prNumber), "--repo", repo.FullName(), "--json", "number,title,body,state,url,headRefName,baseRefName,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup")
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
		HeadRefName:    raw.HeadRefName,
		BaseRefName:    raw.BaseRefName,
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
			"Treat this as a read-only review brief; do not modify files, commit, push, or resolve comments.",
			"Inspect the actual diff when a PR is known; do not review only the issue body, PR body, or changed-file list.",
			"Check AGENTS.md and repository-local agent instructions before applying generic review assumptions.",
			"Consider AI Delivery Telemetry, Gira label/workflow conventions, CLI/tool contract conventions, and tests required by the changed surface.",
			"Review findings first, ordered by severity, with file and line references where available.",
			"Prioritize bugs, regressions, missing tests, security, data loss, and operational risks.",
			"Do not lead with a summary before findings.",
			"Call out residual test gaps even when no blocking findings remain.",
		}
	default:
		return nil
	}
}

func buildAgentReviewContract(report AgentPromptReport) *AgentReviewContract {
	refs := []AgentReviewReference{}
	if report.PR != nil && report.PR.Number > 0 {
		refs = append(refs,
			AgentReviewReference{Kind: "diff", Command: fmt.Sprintf("gh pr diff %d --repo %s", report.PR.Number, report.Repo)},
			AgentReviewReference{Kind: "changed_files", Command: fmt.Sprintf("gh pr diff %d --repo %s --name-only", report.PR.Number, report.Repo)},
			AgentReviewReference{Kind: "metadata", Command: fmt.Sprintf("gh pr view %d --repo %s --json state,reviewDecision,mergeStateStatus,statusCheckRollup", report.PR.Number, report.Repo)},
		)
	}
	return &AgentReviewContract{
		DiffReferences: refs,
		Guidance:       loadAgentPromptGuidance(),
		VerdictSchema: AgentReviewVerdictSchema{
			GoalFulfilled:            []string{"yes", "no", "partial", "unknown"},
			AcceptanceCriteriaStatus: []string{"satisfied", "missing", "partial", "unknown"},
			ChecksStatus:             []string{"passed", "failed", "pending", "missing", "unknown"},
			EvidenceStatus:           []string{"sufficient", "missing", "partial", "unknown"},
			ResidualRisk:             []string{"low", "medium", "high", "unknown"},
			RecommendedAction:        []string{"approve", "request_changes", "ask_human", "wait_for_checks", "gather_evidence"},
			ReviewerNotes:            "Short review notes with findings first and file/line references where applicable.",
			TestGaps:                 "Explicit remaining verification gaps, or none.",
			FollowUps:                "Follow-up issues or actions that should not block this review, or none.",
		},
	}
}

func loadAgentPromptGuidance() []AgentPromptGuidance {
	path, ok := findFileUpward("AGENTS.md")
	if !ok {
		return []AgentPromptGuidance{{Path: "AGENTS.md", Status: "missing"}}
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return []AgentPromptGuidance{{Path: "AGENTS.md", Status: "unreadable"}}
	}
	return []AgentPromptGuidance{{Path: "AGENTS.md", Status: "found", Content: strings.TrimSpace(string(content))}}
}

func findFileUpward(name string) (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func isMissingLinkedPRPromptError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "requires a linked PR")
}

func buildAgentPromptRolePacket(report AgentPromptReport, issue devStartIssue) *AgentPromptRolePacket {
	packet := &AgentPromptRolePacket{
		Role:     report.Role,
		Goal:     report.Issue.Goal,
		Scope:    report.Issue.Scope,
		Labels:   append([]string(nil), report.Issue.Labels...),
		Risk:     agentPromptRiskSignals(report.Issue.Labels),
		Guidance: loadAgentPromptGuidance(),
	}
	switch report.Role {
	case AgentPromptRolePlanner:
		packet.Readiness = plannerReadinessSignals(issue, report.Issue.Acceptance)
		packet.ExpectedEvidence = []string{
			"queue-ready issue has a clear goal, bounded scope, acceptance criteria, and verification commands",
			"implementation tasks should be small enough for stateless workers",
			"human decisions should be called out before implementation starts",
		}
	case AgentPromptRoleImplementer:
		branch := formatDevBranch(DefaultDevBranchPattern, report.Ticket, report.Issue.Title)
		packet.WorkOrder = []string{
			fmt.Sprintf("start or reuse branch `%s` with `gira ticket start --repo %s --ticket %d --apply`", branch, report.Repo, report.Ticket),
			"keep changes bounded to the ticket goal, scope, and acceptance criteria",
			fmt.Sprintf("open a PR with a closing reference such as `Closes #%d`", report.Ticket),
		}
		packet.ExpectedEvidence = []string{
			"changed files are limited to the issue scope",
			"local tests or documented verification commands were run",
			"PR body links the ticket and summarizes behavior, tests, and caveats",
			"checks, review, and finish status can be inspected with Gira lifecycle commands",
		}
	case AgentPromptRoleReviewer:
		packet.WorkOrder = []string{
			"inspect the actual PR diff before forming a verdict",
			"compare changed files, checks, and evidence against issue goal and acceptance criteria",
			"use the review verdict schema and put findings first",
		}
		packet.ExpectedEvidence = []string{
			"linked PR has a closing reference to the ticket",
			"checks and review state are represented in the packet when available",
			"residual risk, test gaps, and follow-ups are explicit",
		}
	}
	return packet
}

func plannerReadinessSignals(issue devStartIssue, acceptance []string) []string {
	signals := []string{}
	if strings.EqualFold(issue.State, "open") {
		signals = append(signals, "issue_open")
	} else {
		signals = append(signals, "issue_not_open")
	}
	status := managedStatusFromLabels(issue.Labels)
	if status == "" {
		signals = append(signals, "missing_status_label")
	} else {
		signals = append(signals, "status:"+strings.ToLower(strings.ReplaceAll(displayStatus(status), " ", "-")))
	}
	if len(acceptance) > 0 {
		signals = append(signals, "acceptance_criteria_present")
	} else {
		signals = append(signals, "acceptance_criteria_missing")
	}
	return signals
}

func agentPromptRiskSignals(labels []string) []string {
	risks := []string{}
	for _, label := range labels {
		lower := strings.ToLower(strings.TrimSpace(label))
		if strings.HasPrefix(lower, "priority:p0") || strings.HasPrefix(lower, "priority:p1") || strings.Contains(lower, "security") || strings.Contains(lower, "area:ai") {
			risks = append(risks, label)
		}
	}
	if len(risks) == 0 {
		return []string{"none_detected_from_labels"}
	}
	return risks
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
