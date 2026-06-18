package gira

import (
	"fmt"
	"strings"
)

const PMAcceptanceQASchemaVersion = "gira-pm-qa/v1"

type PMAcceptanceQAInput struct {
	Repo               RepoRef `json:"repo"`
	Ticket             int     `json:"ticket"`
	PRNumber           int     `json:"pr_number,omitempty"`
	IncludeDiffSummary bool    `json:"include_diff_summary,omitempty"`
	IncludeDiff        bool    `json:"include_diff,omitempty"`
}

type PMAcceptanceQAReport struct {
	Command        string                  `json:"command"`
	SchemaVersion  string                  `json:"schema_version"`
	Repo           string                  `json:"repo"`
	Ticket         int                     `json:"ticket"`
	Issue          AgentPromptIssue        `json:"issue"`
	PR             *AgentPromptPR          `json:"pr,omitempty"`
	PMStatePresent bool                    `json:"pm_state_present"`
	DiffSummary    *AgentReviewDiffSummary `json:"diff_summary,omitempty"`
	VerdictSchema  PMAcceptanceQAVerdict   `json:"verdict_schema"`
	Prompt         string                  `json:"prompt"`
	NextStep       string                  `json:"next_step"`
}

type PMAcceptanceQAVerdict struct {
	ProblemSolved           []string `json:"problem_solved"`
	GoalSatisfied           []string `json:"goal_satisfied"`
	AcceptanceStatus        []string `json:"acceptance_status"`
	DecisionPolicyPreserved []string `json:"decision_policy_preserved"`
	NonGoalStatus           []string `json:"non_goal_status"`
	ScopeDrift              []string `json:"scope_drift"`
	RiskDisposition         []string `json:"risk_disposition"`
	RecommendedAction       []string `json:"recommended_action"`
}

func BuildPMAcceptanceQAReport(input PMAcceptanceQAInput, runner CommandRunner) (PMAcceptanceQAReport, error) {
	if input.Ticket <= 0 {
		return PMAcceptanceQAReport{}, fmt.Errorf("ticket must be > 0")
	}
	agent, err := BuildAgentPromptReport(AgentPromptInput{
		Repo:               input.Repo,
		Ticket:             input.Ticket,
		Role:               AgentPromptRoleReviewer,
		Profile:            AgentPromptProfileDefault,
		PRNumber:           input.PRNumber,
		IncludeDiffSummary: input.IncludeDiffSummary,
		IncludeDiff:        input.IncludeDiff,
	}, runner)
	if err != nil {
		return PMAcceptanceQAReport{}, err
	}
	report := PMAcceptanceQAReport{
		Command:        "pm qa",
		SchemaVersion:  PMAcceptanceQASchemaVersion,
		Repo:           input.Repo.FullName(),
		Ticket:         input.Ticket,
		Issue:          agent.Issue,
		PR:             agent.PR,
		PMStatePresent: strings.Contains(agent.Issue.Body, PMStateMarker),
		VerdictSchema: PMAcceptanceQAVerdict{
			ProblemSolved:           []string{"yes", "no", "partial", "unknown"},
			GoalSatisfied:           []string{"yes", "no", "partial", "unknown"},
			AcceptanceStatus:        []string{"satisfied", "missing", "partial", "unknown"},
			DecisionPolicyPreserved: []string{"yes", "no", "not_present", "unknown"},
			NonGoalStatus:           []string{"preserved", "violated", "not_present", "unknown"},
			ScopeDrift:              []string{"none", "minor", "major", "unknown"},
			RiskDisposition:         []string{"reduced", "not_reduced", "moved_to_follow_up", "unknown"},
			RecommendedAction:       []string{"pm:accepted", "pm:implementation-mismatch", "pm:spec-repair", "pm:follow-up-task", "pm:risk-reduction-task"},
		},
		NextStep: "Use the PM QA verdict to accept, request implementation fix, repair the spec, or create a follow-up task packet.",
	}
	if agent.Review != nil && agent.Review.DiffSummary != nil {
		report.DiffSummary = agent.Review.DiffSummary
	}
	report.Prompt = RenderPMAcceptanceQAPrompt(report)
	return report, nil
}

func RenderPMAcceptanceQAPrompt(report PMAcceptanceQAReport) string {
	var b strings.Builder
	b.WriteString("# Gira PM Acceptance QA\n\n")
	fmt.Fprintf(&b, "Repository: `%s`\n", report.Repo)
	fmt.Fprintf(&b, "Ticket: `#%d` %s\n", report.Ticket, report.Issue.Title)
	if report.PR != nil {
		fmt.Fprintf(&b, "Pull Request: `#%d` %s\n", report.PR.Number, report.PR.Title)
	}
	fmt.Fprintf(&b, "PM State Present: `%t`\n\n", report.PMStatePresent)
	b.WriteString("## Role Boundary\n\n")
	b.WriteString("- You are performing PM acceptance QA, not engineering code review.\n")
	b.WriteString("- Engineering review owns code quality, correctness, regression risk, security, and tests.\n")
	b.WriteString("- PM QA owns product/task acceptance: problem, goal, decision policy, acceptance criteria, non-goals, scope drift, and risk disposition.\n")
	b.WriteString("- Use the implementation claims matrix when present. If it is absent, reconstruct claims from PR body, diff summary, tests, and comments.\n")
	b.WriteString("- Do not use `needs human` as a terminal state. If judgment is missing, decompose it into spec repair, context retrieval, risk reduction, implementation fix, or follow-up task packet.\n\n")
	b.WriteString("## Authoritative PM State\n\n")
	b.WriteString(report.Issue.Body)
	b.WriteString("\n\n")
	if report.PR != nil {
		b.WriteString("## PR Evidence\n\n")
		fmt.Fprintf(&b, "- Title: %s\n", report.PR.Title)
		fmt.Fprintf(&b, "- State: %s\n", report.PR.State)
		fmt.Fprintf(&b, "- Draft: %t\n", report.PR.IsDraft)
		if len(report.PR.ChangedFiles) > 0 {
			b.WriteString("- Changed files:\n")
			for _, file := range report.PR.ChangedFiles {
				fmt.Fprintf(&b, "  - %s\n", file)
			}
		}
		if strings.TrimSpace(report.PR.Body) != "" {
			b.WriteString("\n### PR Body\n\n")
			b.WriteString(report.PR.Body)
			b.WriteString("\n\n")
		}
	}
	if report.DiffSummary != nil {
		b.WriteString("## Diff Summary\n\n")
		fmt.Fprintf(&b, "- Additions: %d\n", report.DiffSummary.TotalAdditions)
		fmt.Fprintf(&b, "- Deletions: %d\n", report.DiffSummary.TotalDeletions)
		for _, file := range report.DiffSummary.Files {
			fmt.Fprintf(&b, "- `%s`: +%d -%d\n", file.Path, file.Additions, file.Deletions)
		}
		b.WriteString("\n")
	}
	b.WriteString("## Required PM QA Checks\n\n")
	b.WriteString("1. Does the PR solve the stated problem?\n")
	b.WriteString("2. Does the user/customer outcome actually change in the intended way?\n")
	b.WriteString("3. Does the PR align with the product goal or priority in the PM state?\n")
	b.WriteString("4. Does it satisfy each acceptance criterion as a pass/fail outcome?\n")
	b.WriteString("5. Does each PR implementation claim have enough evidence?\n")
	b.WriteString("6. Did it preserve the decision policy?\n")
	b.WriteString("7. Did it avoid violating non-goals/no-gos?\n")
	b.WriteString("8. Did it avoid rabbit holes and unrelated scope drift?\n")
	b.WriteString("9. Did it reduce, isolate, or explicitly follow up risks in the PM state?\n")
	b.WriteString("10. Is the change reversible or safely rolled out when risk requires it?\n\n")
	b.WriteString("## Implementation Claims Matrix\n\n")
	b.WriteString("| Acceptance criterion | PR claim | Evidence | PM QA result |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	b.WriteString("| Criterion from PM state | Claimed implementation behavior | Test, diff, screenshot, log, command, or explanation | accepted / mismatch / unknown |\n\n")
	b.WriteString("## Verdict Schema\n\n")
	b.WriteString("- recommended_action: pm:accepted | pm:implementation-mismatch | pm:spec-repair | pm:follow-up-task | pm:risk-reduction-task\n")
	b.WriteString("- problem_solved: yes | no | partial | unknown\n")
	b.WriteString("- goal_satisfied: yes | no | partial | unknown\n")
	b.WriteString("- acceptance_status: satisfied | missing | partial | unknown\n")
	b.WriteString("- decision_policy_preserved: yes | no | not_present | unknown\n")
	b.WriteString("- non_goal_status: preserved | violated | not_present | unknown\n")
	b.WriteString("- scope_drift: none | minor | major | unknown\n")
	b.WriteString("- risk_disposition: reduced | not_reduced | moved_to_follow_up | unknown\n")
	return strings.TrimSpace(b.String()) + "\n"
}
