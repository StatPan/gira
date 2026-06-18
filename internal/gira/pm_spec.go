package gira

import (
	"encoding/json"
	"fmt"
	"strings"
)

const PMTaskPacketSchemaVersion = "gira-pm-task-packet/v1"
const PMStateMarker = "<!-- gira:pm-state version=1 -->"

type PMTaskSpecInput struct {
	Title               string `json:"title,omitempty"`
	Repo                string `json:"repo,omitempty"`
	RawIntent           string `json:"raw_intent"`
	SuggestedWorkerMode string `json:"suggested_worker_mode,omitempty"`
}

type PMTaskSpecReport struct {
	Command             string   `json:"command"`
	SchemaVersion       string   `json:"schema_version"`
	Title               string   `json:"title,omitempty"`
	Repo                string   `json:"repo,omitempty"`
	RawIntent           string   `json:"raw_intent"`
	SuggestedWorkerMode string   `json:"suggested_worker_mode"`
	RequiredSections    []string `json:"required_sections"`
	Markdown            string   `json:"markdown"`
	NextStep            string   `json:"next_step"`
}

func BuildPMTaskSpecReport(input PMTaskSpecInput) (PMTaskSpecReport, error) {
	rawIntent := strings.TrimSpace(input.RawIntent)
	if rawIntent == "" {
		return PMTaskSpecReport{}, fmt.Errorf("raw intent is required")
	}
	mode := normalizePMWorkerMode(input.SuggestedWorkerMode)
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = firstNonEmptyLine(rawIntent)
	}
	repo := strings.TrimSpace(input.Repo)
	report := PMTaskSpecReport{
		Command:             "pm spec",
		SchemaVersion:       PMTaskPacketSchemaVersion,
		Title:               title,
		Repo:                repo,
		RawIntent:           rawIntent,
		SuggestedWorkerMode: mode,
		RequiredSections: []string{
			"Product Context",
			"Customer / User Outcome",
			"Product Goal Alignment",
			"Problem",
			"Goal",
			"Decision Policy",
			"Appetite / Boundary",
			"Acceptance Criteria",
			"Signals / Metrics / Evidence",
			"Non-goals",
			"Rabbit Holes",
			"Context Packet",
			"Risk Decomposition",
			"Reversibility / Rollout",
			"Verification Expectations",
			"Suggested Worker Mode",
			"Next Action",
		},
	}
	report.Markdown = RenderPMTaskSpecMarkdown(report)
	report.NextStep = pmTaskSpecNextStep(report)
	return report, nil
}

func RenderPMTaskSpecMarkdown(report PMTaskSpecReport) string {
	var b strings.Builder
	b.WriteString(PMStateMarker)
	b.WriteString("\n")
	b.WriteString("<!-- gira:task-packet schema=gira-pm-task-packet/v1 -->\n\n")
	b.WriteString("# Gira PM Task Packet\n\n")
	b.WriteString("This issue body is the durable PM state for the task. Future planning, implementation, engineering review, and PM acceptance QA must use this packet as the source of truth instead of hidden thread memory.\n\n")
	b.WriteString("## PM Operating Contract\n\n")
	b.WriteString("- Do not use `needs human` as a terminal state.\n")
	b.WriteString("- If judgment appears to require a human, decompose why: missing context, missing decision policy, conflicting constraints, irreversible risk, insufficient verification, authority boundary, or undefined success metric.\n")
	b.WriteString("- Convert the decomposed reason into executable work: retrieve context, derive policy from prior decisions, choose a reversible default, split irreversible work out, create verification criteria, reduce scope, or produce a follow-up task packet.\n")
	b.WriteString("- Prefer durable evidence in this issue, linked issues, linked PRs, repository files, and explicitly attached context over transcript memory.\n")
	b.WriteString("- Keep the first implementation slice small, reversible, and directly tied to acceptance criteria.\n\n")
	b.WriteString("## PM Template Guidance\n\n")
	b.WriteString("- Product ownership: connect backlog work to a product goal and make the work transparent enough for agents and reviewers to execute.\n")
	b.WriteString("- User story discipline: describe the affected user or job, the desired outcome, and why it matters.\n")
	b.WriteString("- Acceptance criteria: write pass/fail, outcome-focused criteria that tell the developer when to stop, QA how to test, and PM what to expect.\n")
	b.WriteString("- Shape Up discipline: set appetite and boundaries, call out rabbit holes, and name no-gos before implementation starts.\n")
	b.WriteString("- Measurement discipline: map goals to signals, metrics, or concrete evidence so PM QA can verify the functional outcome.\n")
	b.WriteString("- Risk discipline: keep risky work incremental, reversible, or isolated through rollout strategy, feature flags, branch-by-abstraction, or follow-up packets.\n\n")
	if strings.TrimSpace(report.Repo) != "" {
		fmt.Fprintf(&b, "## Target Repository\n\n%s\n\n", report.Repo)
	}
	b.WriteString("## Raw Intent\n\n")
	b.WriteString(report.RawIntent)
	b.WriteString("\n\n")
	b.WriteString("## Product Context\n\n")
	b.WriteString("Who is affected, what user/customer/job context matters, and why this matters now. If the user is unknown, state the likely actor and the evidence needed to confirm it.\n\n")
	b.WriteString("## Problem\n\n")
	b.WriteString("Describe what is actually wrong, missing, slow, risky, or unclear. Do not restate the raw intent unless it already names the user-visible problem.\n\n")
	b.WriteString("## Customer / User Outcome\n\n")
	b.WriteString("Describe what should be different from the user's perspective after the change. Prefer real workflow language over implementation language.\n\n")
	b.WriteString("## Product Goal Alignment\n\n")
	b.WriteString("Name the product goal, strategy, repo objective, or operational priority this task supports. If none is known, derive the smallest plausible goal from available context.\n\n")
	b.WriteString("## Goal\n\n")
	b.WriteString("Describe what should be true after this task is complete.\n\n")
	b.WriteString("## Decision Policy\n\n")
	b.WriteString("State the judgment rule used for this task and why it is acceptable. If the policy is not obvious, derive it from available context and choose the safest reversible default.\n\n")
	b.WriteString("## Appetite / Boundary\n\n")
	b.WriteString("State how much this task is worth and what scope must fit inside that appetite. Use this to decide what is core, peripheral, or deferred.\n\n")
	b.WriteString("## Acceptance Criteria\n\n")
	b.WriteString("- [ ] Pass/fail outcome criterion 1, stated without implementation details\n")
	b.WriteString("- [ ] Pass/fail outcome criterion 2, including an edge case or error path when relevant\n")
	b.WriteString("- [ ] Verification evidence is recorded in the PR implementation claims\n\n")
	b.WriteString("## Signals / Metrics / Evidence\n\n")
	b.WriteString("| Goal | Signal | Metric or evidence |\n")
	b.WriteString("| --- | --- | --- |\n")
	b.WriteString("| User/product outcome | Observable behavior, event, support signal, test, screenshot, or log | Concrete evidence PM QA can inspect |\n\n")
	b.WriteString("## Non-goals / No-gos\n\n")
	b.WriteString("- Work intentionally excluded from this task\n")
	b.WriteString("- Irreversible or broad changes that should be split into a later packet\n\n")
	b.WriteString("## Rabbit Holes\n\n")
	b.WriteString("| Rabbit hole | Why it is risky | Boundary or escape hatch |\n")
	b.WriteString("| --- | --- | --- |\n")
	b.WriteString("| Over-broad redesign or refactor | Consumes appetite without proving the product outcome | Keep the slice tied to acceptance criteria |\n\n")
	b.WriteString("## Context Packet\n\n")
	b.WriteString("- Source context: this issue body and linked artifacts\n")
	b.WriteString("- Relevant files or commands: to be identified during planning\n")
	b.WriteString("- Prior decisions: to be retrieved or derived before implementation if needed\n\n")
	b.WriteString("## Risk Decomposition\n\n")
	b.WriteString("| Risk or uncertainty | Why it matters | Resolution path |\n")
	b.WriteString("| --- | --- | --- |\n")
	b.WriteString("| Missing context | The agent may optimize for the wrong target | Retrieve repository/docs/issue evidence and update this packet |\n")
	b.WriteString("| Irreversible scope | Broad changes can create hidden product risk | Split into reversible preparation work first |\n")
	b.WriteString("| Insufficient verification | Completion cannot be judged reliably | Add tests, checks, or observable acceptance evidence |\n\n")
	b.WriteString("## Reversibility / Rollout\n\n")
	b.WriteString("- Preferred rollout: smallest reversible slice first\n")
	b.WriteString("- If behavior is risky, use a feature flag, compatibility path, branch-by-abstraction, migration dry run, or follow-up rollout task\n")
	b.WriteString("- Do not bundle irreversible data, permission, billing, or public API changes with unrelated implementation work\n\n")
	b.WriteString("## Verification Expectations\n\n")
	b.WriteString("- Engineering review checks code quality, correctness, regression risk, security, and tests.\n")
	b.WriteString("- PM acceptance QA checks this PR against Problem, Goal, Decision Policy, Acceptance Criteria, Non-goals, and Risk Decomposition.\n")
	b.WriteString("- The PR should include an implementation claims matrix mapping each acceptance criterion to evidence.\n\n")
	b.WriteString("## Implementation Claims Required In PR\n\n")
	b.WriteString("| Acceptance criterion | Implementation claim | Evidence |\n")
	b.WriteString("| --- | --- | --- |\n")
	b.WriteString("| AC1 | What changed to satisfy it | Test, diff, screenshot, log, command, or explanation |\n\n")
	b.WriteString("## Suggested Worker Mode\n\n")
	fmt.Fprintf(&b, "%s\n\n", report.SuggestedWorkerMode)
	b.WriteString("## Next Action\n\n")
	b.WriteString("Run PM decomposition if any required section remains generic. Otherwise run worker planning against this packet.\n")
	return strings.TrimSpace(b.String()) + "\n"
}

func FormatPMTaskSpec(report PMTaskSpecReport) string {
	return report.Markdown
}

func FormatPMTaskSpecJSON(report PMTaskSpecReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

func normalizePMWorkerMode(value string) string {
	mode := strings.TrimSpace(strings.ToLower(value))
	switch mode {
	case "research", "plan", "implement", "review", "fix_review", "pm_qa":
		return mode
	default:
		return "plan"
	}
}

func firstNonEmptyLine(value string) string {
	for _, line := range strings.Split(value, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			if len(trimmed) > 100 {
				return strings.TrimSpace(trimmed[:100])
			}
			return trimmed
		}
	}
	return "Gira PM task"
}

func pmTaskSpecNextStep(report PMTaskSpecReport) string {
	if strings.TrimSpace(report.Repo) != "" {
		return fmt.Sprintf("gira ticket new --repo %s --title %s --body-file pm-task.md --type task --dry-run", report.Repo, QuoteShellArg(report.Title))
	}
	return fmt.Sprintf("gira ticket new --title %s --body-file pm-task.md --type task --dry-run", QuoteShellArg(report.Title))
}
