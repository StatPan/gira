package gira

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const GoalPlanSchemaVersion = "goal-plan/v1"

type GoalPlanInput struct {
	Repo   RepoRef `json:"repo"`
	Goal   int     `json:"goal"`
	DryRun bool    `json:"dry_run"`
	Apply  bool    `json:"apply"`
}

type GoalPlanReport struct {
	Command           string           `json:"command"`
	SchemaVersion     string           `json:"schema_version"`
	Repo              string           `json:"repo"`
	DryRun            bool             `json:"dry_run"`
	Apply             bool             `json:"apply,omitempty"`
	Goal              GoalStatusIssue  `json:"goal"`
	ProposedTickets   []GoalPlanTicket `json:"proposed_tickets"`
	SkippedCandidates []GoalPlanSkip   `json:"skipped_candidates,omitempty"`
	ExistingChildren  []GoalPlanChild  `json:"existing_children,omitempty"`
	CreatedChildren   []GoalPlanChild  `json:"created_children,omitempty"`
	Actions           []GoalPlanAction `json:"actions,omitempty"`
	StopConditions    []string         `json:"stop_conditions,omitempty"`
	Warnings          []string         `json:"warnings,omitempty"`
	NextAction        string           `json:"next_action"`
	NextStep          string           `json:"next_step"`
}

type GoalPlanTicket struct {
	Title            string                `json:"title"`
	Type             string                `json:"type"`
	Priority         string                `json:"priority,omitempty"`
	Labels           []string              `json:"labels,omitempty"`
	Milestone        string                `json:"milestone,omitempty"`
	ParentGoal       int                   `json:"parent_goal"`
	Dependencies     []int                 `json:"dependencies,omitempty"`
	Goal             string                `json:"goal"`
	Scope            string                `json:"scope"`
	Acceptance       []string              `json:"acceptance"`
	ExpectedEvidence []string              `json:"expected_evidence"`
	Body             string                `json:"body"`
	TicketReadiness  TicketReadinessReport `json:"ticket_readiness"`
}

type GoalPlanSkip struct {
	Title       string `json:"title"`
	Reason      string `json:"reason"`
	DuplicateOf int    `json:"duplicate_of,omitempty"`
}

type GoalPlanChild struct {
	Number   int    `json:"number"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Status   string `json:"status"`
	URL      string `json:"url,omitempty"`
}

type GoalPlanAction struct {
	Action string `json:"action"`
	Title  string `json:"title,omitempty"`
	Status string `json:"status"`
	Issue  int    `json:"issue,omitempty"`
	URL    string `json:"url,omitempty"`
	Reason string `json:"reason,omitempty"`
}

func BuildGoalPlanReport(input GoalPlanInput, runner CommandRunner) (GoalPlanReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if input.Goal <= 0 {
		return GoalPlanReport{}, fmt.Errorf("goal must be > 0")
	}
	if strings.TrimSpace(input.Repo.Owner) == "" || strings.TrimSpace(input.Repo.Name) == "" {
		return GoalPlanReport{
			Command:       "goal plan",
			SchemaVersion: GoalPlanSchemaVersion,
			DryRun:        input.DryRun,
			Apply:         input.Apply,
			StopConditions: []string{
				"ambiguous_target_repo",
			},
			NextAction: "ask_human",
			NextStep:   "rerun with --repo OWNER/REPO",
		}, nil
	}
	status, err := BuildGoalStatusReport(GoalStatusInput{Repo: input.Repo, Goal: input.Goal}, runner)
	if err != nil {
		return GoalPlanReport{}, err
	}
	goal, err := fetchDevIssue(input.Repo, input.Goal, runner)
	if err != nil {
		return GoalPlanReport{}, err
	}
	report := GoalPlanReport{
		Command:       "goal plan",
		SchemaVersion: GoalPlanSchemaVersion,
		Repo:          input.Repo.FullName(),
		DryRun:        input.DryRun,
		Apply:         input.Apply,
		Goal:          status.Goal,
		NextAction:    "create_child_tickets",
		NextStep:      fmt.Sprintf("gira goal plan --repo %s --goal %d --apply", input.Repo.FullName(), input.Goal),
	}
	report.ExistingChildren = goalPlanChildren(status.Children)
	objective := goalPlanObjective(goal.Body)
	scope := goalPlanScope(goal.Body)
	if emptyReadinessSection(objective) {
		report.StopConditions = append(report.StopConditions, "missing_objective")
	}
	if emptyReadinessSection(scope) {
		report.StopConditions = append(report.StopConditions, "missing_scope")
	}
	if goalPlanNeedsHumanDecision(goal.Body, goal.Labels) {
		report.StopConditions = append(report.StopConditions, "human_decision_required")
	}
	items := goalPlanItems(goal.Body)
	if len(items) == 0 {
		report.StopConditions = append(report.StopConditions, "missing_decomposition_notes")
	}
	if len(report.StopConditions) > 0 {
		report.NextAction = "ask_human"
		report.NextStep = fmt.Sprintf("add objective, scope, and child planning notes to #%d before running goal plan again", input.Goal)
		return report, nil
	}
	for _, item := range items {
		title := goalPlanTicketTitle(item)
		if duplicate := goalPlanDuplicateChildNumber(title, status.Children); duplicate > 0 {
			report.SkippedCandidates = append(report.SkippedCandidates, GoalPlanSkip{Title: title, Reason: "duplicate_existing_child", DuplicateOf: duplicate})
			report.Actions = append(report.Actions, GoalPlanAction{Action: "child_ticket:skip", Title: title, Status: "skipped", Issue: duplicate, Reason: "duplicate_existing_child"})
			report.Warnings = appendUniqueStrings(report.Warnings, "duplicate_existing_child")
			continue
		}
		ticket := goalPlanTicket(goal, title, objective, scope)
		report.ProposedTickets = append(report.ProposedTickets, ticket)
		report.Actions = append(report.Actions, GoalPlanAction{Action: "child_ticket:create", Title: ticket.Title, Status: "planned", Reason: "proposed goal child ticket"})
	}
	if len(report.ProposedTickets) == 0 {
		report.StopConditions = append(report.StopConditions, "no_new_child_tickets")
		report.NextAction = "inspect_goal"
		report.NextStep = fmt.Sprintf("gira goal status --repo %s --goal %d --json", input.Repo.FullName(), input.Goal)
	}
	if input.Apply {
		if len(report.StopConditions) > 0 {
			return report, nil
		}
		if err := applyGoalPlan(&report, input.Repo, runner); err != nil {
			return report, err
		}
		report.NextAction = "inspect_goal"
		report.NextStep = fmt.Sprintf("gira goal status --repo %s --goal %d --json", input.Repo.FullName(), input.Goal)
	}
	return report, nil
}

func applyGoalPlan(report *GoalPlanReport, repo RepoRef, runner CommandRunner) error {
	labels := []string{}
	for _, ticket := range report.ProposedTickets {
		labels = append(labels, ticket.Labels...)
	}
	if err := preflightTicketNewLabels(repo, appendUniqueStrings(nil, labels...), runner); err != nil {
		report.StopConditions = appendUniqueStrings(report.StopConditions, "label_preflight_failed")
		report.NextAction = "fix_labels"
		report.NextStep = fmt.Sprintf("gira ops sync --repo %s --dry-run", repo.FullName())
		return err
	}
	for _, ticket := range report.ProposedTickets {
		created, err := createRepoTicket(repo, ticket.Title, ticket.Body, ticket.Labels, ticket.Milestone, runner)
		if err != nil {
			return err
		}
		child := GoalPlanChild{Number: created.Number, Title: ticket.Title, Category: "ready", Status: "Ready", URL: created.URL}
		report.CreatedChildren = append(report.CreatedChildren, child)
		goalPlanMarkActionApplied(report.Actions, ticket.Title, created)
	}
	return nil
}

func goalPlanMarkActionApplied(actions []GoalPlanAction, title string, created TicketCreatedIssue) {
	for i := range actions {
		if actions[i].Action == "child_ticket:create" && actions[i].Title == title {
			actions[i].Status = "applied"
			actions[i].Issue = created.Number
			actions[i].URL = created.URL
			return
		}
	}
}

func goalPlanChildren(children []GoalStatusChild) []GoalPlanChild {
	out := make([]GoalPlanChild, 0, len(children))
	for _, child := range children {
		out = append(out, GoalPlanChild{Number: child.Number, Title: child.Title, Category: child.Category, Status: child.Status})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out
}

func goalPlanObjective(body string) string {
	return strings.TrimSpace(markdownSection(body, "Goal"))
}

func goalPlanScope(body string) string {
	for _, heading := range []string{"Scope", "Product Thesis", "Proposed 2.0 Milestones"} {
		if value := strings.TrimSpace(markdownSection(body, heading)); !emptyReadinessSection(value) {
			return value
		}
	}
	return ""
}

func goalPlanItems(body string) []string {
	headings := []string{
		"Goal Plan",
		"Decomposition",
		"Child Ticket Plan",
		"Suggested Child Tickets",
		"Initial Follow-Up Issues To Create",
		"Follow-Up Issues",
	}
	items := []string{}
	for _, heading := range headings {
		for _, item := range markdownListSection(body, heading) {
			cleaned := cleanGoalPlanItem(item)
			if cleaned != "" {
				items = appendUniqueStrings(items, cleaned)
			}
		}
	}
	return items
}

func cleanGoalPlanItem(value string) string {
	trimmed := strings.TrimSpace(value)
	for _, prefix := range []string{"[ ] ", "[x] ", "[X] "} {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
	}
	return strings.Trim(trimmed, ".")
}

func goalPlanTicketTitle(item string) string {
	title := strings.TrimSpace(item)
	if strings.HasPrefix(title, "[") {
		return title
	}
	return "[Task] " + title
}

func goalPlanTicket(goal devStartIssue, title string, objective string, scope string) GoalPlanTicket {
	labels := goalPlanLabels(goal.Labels)
	acceptance := []string{
		fmt.Sprintf("%s is defined with stable behavior", strings.TrimPrefix(title, "[Task] ")),
		"JSON and compact human output are covered by tests or documented examples",
		fmt.Sprintf("Parent goal #%d remains the source of truth", goal.Number),
	}
	evidence := []string{"go test ./internal/gira", "go test ./internal/cli", "go test ./..."}
	body := renderGoalPlanTicketBody(goal.Number, objective, title, scope, acceptance, evidence)
	return GoalPlanTicket{
		Title:            title,
		Type:             "task",
		Priority:         goalPlanPriority(goal.Labels),
		Labels:           labels,
		Milestone:        goal.Milestone,
		ParentGoal:       goal.Number,
		Goal:             fmt.Sprintf("%s for goal #%d.", strings.TrimPrefix(title, "[Task] "), goal.Number),
		Scope:            scope,
		Acceptance:       acceptance,
		ExpectedEvidence: evidence,
		Body:             body,
		TicketReadiness:  EvaluateTicketReadiness(body, labels, "open"),
	}
}

func renderGoalPlanTicketBody(parent int, objective string, title string, scope string, acceptance []string, evidence []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Goal\n%s\n\nParent: #%d\n\n", strings.TrimPrefix(title, "[Task] "), parent)
	fmt.Fprintf(&b, "## Scope\nDerived from parent goal #%d.\n\nParent objective:\n%s\n\nPlanning scope:\n%s\n\n", parent, strings.TrimSpace(objective), strings.TrimSpace(scope))
	b.WriteString("## Acceptance Criteria\n")
	for _, item := range acceptance {
		fmt.Fprintf(&b, "- %s\n", item)
	}
	b.WriteString("\n## Doctor Impact\nNo doctor behavior expected unless the implementation changes status, readiness, audit, or workflow reports.\n\n")
	b.WriteString("## Expected Evidence\n")
	for _, item := range evidence {
		fmt.Fprintf(&b, "- %s\n", item)
	}
	b.WriteString("\n## Expected Delivery\nDraft PR first, then normal Gira review/finish lifecycle.\n")
	return b.String()
}

func goalPlanLabels(goalLabels []string) []string {
	labels := []string{"type:task", "status:ready"}
	if priority := goalPlanPriority(goalLabels); priority != "" {
		labels = append(labels, "priority:"+priority)
	}
	for _, label := range goalLabels {
		if strings.HasPrefix(label, "area:") {
			labels = appendUniqueStrings(labels, label)
		}
	}
	return labels
}

func goalPlanPriority(labels []string) string {
	for _, label := range labels {
		if strings.HasPrefix(label, "priority:") {
			return strings.TrimPrefix(label, "priority:")
		}
	}
	return "p2"
}

func goalPlanNeedsHumanDecision(body string, labels []string) bool {
	for _, label := range labels {
		lower := strings.ToLower(strings.TrimSpace(label))
		if lower == "status:blocked" || lower == "needs:human" {
			return true
		}
	}
	for _, heading := range []string{"Human Decision", "Decision Required", "Open Questions"} {
		if !emptyReadinessSection(markdownSection(body, heading)) {
			return true
		}
	}
	return false
}

func goalPlanExistingTitleIndex(children []GoalStatusChild) map[string]int {
	index := map[string]int{}
	for _, child := range children {
		key := goalPlanTitleKey(child.Title)
		if key != "" {
			index[key] = child.Number
		}
	}
	return index
}

func goalPlanDuplicateChildNumber(title string, children []GoalStatusChild) int {
	key := goalPlanTitleKey(title)
	if key == "" {
		return 0
	}
	existing := goalPlanExistingTitleIndex(children)
	if duplicate := existing[key]; duplicate > 0 {
		return duplicate
	}
	candidateTokens := goalPlanSignificantTokens(key)
	for _, child := range children {
		if goalPlanFuzzyTitleMatch(candidateTokens, goalPlanSignificantTokens(goalPlanTitleKey(child.Title))) {
			return child.Number
		}
	}
	return 0
}

var goalPlanTitleNoise = regexp.MustCompile(`[^a-z0-9]+`)

func goalPlanTitleKey(value string) string {
	lower := strings.ToLower(strings.TrimSpace(value))
	lower = strings.TrimPrefix(lower, "[task]")
	lower = strings.TrimSpace(strings.TrimPrefix(lower, "task"))
	return strings.Trim(goalPlanTitleNoise.ReplaceAllString(lower, " "), " ")
}

func goalPlanSignificantTokens(value string) []string {
	out := []string{}
	for _, token := range strings.Fields(value) {
		if len(token) < 3 || goalPlanStopword(token) {
			continue
		}
		out = appendUniqueStrings(out, token)
	}
	return out
}

func goalPlanStopword(value string) bool {
	switch value {
	case "add", "define", "first", "slice", "format", "contract", "and", "the", "for", "with":
		return true
	default:
		return false
	}
}

func goalPlanFuzzyTitleMatch(candidate []string, existing []string) bool {
	if len(candidate) == 0 || len(existing) == 0 {
		return false
	}
	common := 0
	for _, token := range candidate {
		if containsString(existing, token) {
			common++
		}
	}
	if common >= 2 && common*2 >= len(candidate) {
		return true
	}
	if containsString(candidate, "goal") && containsString(candidate, "mode") && containsString(existing, "goal") {
		return true
	}
	return false
}

func FormatGoalPlan(report GoalPlanReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "goal plan: #%d proposed=%d created=%d skipped=%d stops=%d\n", report.Goal.Number, len(report.ProposedTickets), len(report.CreatedChildren), len(report.SkippedCandidates), len(report.StopConditions))
	if len(report.StopConditions) > 0 {
		fmt.Fprintf(&b, "stop: %s\n", strings.Join(report.StopConditions, ","))
	}
	if len(report.Warnings) > 0 {
		fmt.Fprintf(&b, "warnings: %s\n", strings.Join(report.Warnings, ","))
	}
	for _, child := range report.CreatedChildren {
		fmt.Fprintf(&b, "- created #%d %s\n", child.Number, child.Title)
	}
	for _, skip := range report.SkippedCandidates {
		if skip.DuplicateOf > 0 {
			fmt.Fprintf(&b, "- skipped %s duplicate_existing_child=#%d\n", skip.Title, skip.DuplicateOf)
			continue
		}
		fmt.Fprintf(&b, "- skipped %s %s\n", skip.Title, skip.Reason)
	}
	for _, ticket := range report.ProposedTickets {
		if len(report.CreatedChildren) == 0 {
			fmt.Fprintf(&b, "- %s\n", ticket.Title)
		}
	}
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}
