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
	TargetRepo       string                `json:"target_repo"`
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
	Repo     string `json:"repo,omitempty"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Status   string `json:"status"`
	URL      string `json:"url,omitempty"`
}

type GoalPlanAction struct {
	Action     string `json:"action"`
	Title      string `json:"title,omitempty"`
	TargetRepo string `json:"target_repo,omitempty"`
	Status     string `json:"status"`
	Issue      int    `json:"issue,omitempty"`
	URL        string `json:"url,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

func BuildGoalPlanReport(input GoalPlanInput, runner CommandRunner) (GoalPlanReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
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
	goalNumber, _, err := ResolveGoalNumber(input.Repo, input.Goal, runner)
	if err != nil {
		return GoalPlanReport{}, err
	}
	input.Goal = goalNumber
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
		targetRepo, title := goalPlanTargetAndTitle(input.Repo, item)
		if duplicate := goalPlanDuplicateChildNumber(title, targetRepo, status.Children); duplicate > 0 {
			report.SkippedCandidates = append(report.SkippedCandidates, GoalPlanSkip{Title: title, Reason: "duplicate_existing_child", DuplicateOf: duplicate})
			report.Actions = append(report.Actions, GoalPlanAction{Action: "child_ticket:skip", Title: title, TargetRepo: targetRepo.FullName(), Status: "skipped", Issue: duplicate, Reason: "duplicate_existing_child"})
			report.Warnings = appendUniqueStrings(report.Warnings, "duplicate_existing_child")
			continue
		}
		ticket := goalPlanTicket(input.Repo, goal, title, targetRepo, objective, scope)
		report.ProposedTickets = append(report.ProposedTickets, ticket)
		report.Actions = append(report.Actions, GoalPlanAction{Action: "child_ticket:create", Title: ticket.Title, TargetRepo: ticket.TargetRepo, Status: "planned", Reason: "proposed goal child ticket"})
	}
	if len(report.ProposedTickets) > 0 {
		report.Actions = append(report.Actions, GoalPlanAction{Action: "goal:comment", TargetRepo: input.Repo.FullName(), Status: "planned", Reason: "record created child links on parent goal"})
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
	labelsByRepo := map[string][]string{}
	for _, ticket := range report.ProposedTickets {
		labelsByRepo[ticket.TargetRepo] = append(labelsByRepo[ticket.TargetRepo], ticket.Labels...)
	}
	for targetRepoName, labels := range labelsByRepo {
		targetRepo, err := ParseRepoRef(targetRepoName)
		if err != nil {
			return err
		}
		if err := preflightTicketNewLabels(targetRepo, appendUniqueStrings(nil, labels...), runner); err != nil {
			report.StopConditions = appendUniqueStrings(report.StopConditions, "label_preflight_failed")
			report.NextAction = "fix_labels"
			report.NextStep = fmt.Sprintf("gira ops sync --repo %s --dry-run", targetRepo.FullName())
			return err
		}
	}
	for _, ticket := range report.ProposedTickets {
		targetRepo, err := ParseRepoRef(ticket.TargetRepo)
		if err != nil {
			return err
		}
		created, err := createRepoTicket(targetRepo, ticket.Title, ticket.Body, ticket.Labels, ticket.Milestone, runner)
		if err != nil {
			return err
		}
		child := GoalPlanChild{Repo: targetRepo.FullName(), Number: created.Number, Title: ticket.Title, Category: "ready", Status: "Ready", URL: created.URL}
		report.CreatedChildren = append(report.CreatedChildren, child)
		goalPlanMarkActionApplied(report.Actions, ticket.Title, targetRepo.FullName(), created)
	}
	if err := postGoalPlanCreatedChildrenComment(repo, report.Goal.Number, report.CreatedChildren, runner); err != nil {
		return err
	}
	goalPlanMarkActionStatus(report.Actions, "goal:comment", "applied")
	return nil
}

func goalPlanMarkActionApplied(actions []GoalPlanAction, title string, targetRepo string, created TicketCreatedIssue) {
	for i := range actions {
		if actions[i].Action == "child_ticket:create" && actions[i].Title == title && actions[i].TargetRepo == targetRepo {
			actions[i].Status = "applied"
			actions[i].Issue = created.Number
			actions[i].URL = created.URL
			return
		}
	}
}

func goalPlanMarkActionStatus(actions []GoalPlanAction, actionName string, status string) {
	for i := range actions {
		if actions[i].Action == actionName {
			actions[i].Status = status
			return
		}
	}
}

func goalPlanChildren(children []GoalStatusChild) []GoalPlanChild {
	out := make([]GoalPlanChild, 0, len(children))
	for _, child := range children {
		out = append(out, GoalPlanChild{Repo: child.Repo, Number: child.Number, Title: child.Title, Category: child.Category, Status: child.Status, URL: child.URL})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Repo != out[j].Repo {
			return out[i].Repo < out[j].Repo
		}
		return out[i].Number < out[j].Number
	})
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

func goalPlanTargetAndTitle(defaultRepo RepoRef, item string) (RepoRef, string) {
	trimmed := strings.TrimSpace(item)
	for _, prefix := range []string{"target_repo:", "target_repo=", "repo:", "repo="} {
		if strings.HasPrefix(strings.ToLower(trimmed), prefix) {
			rest := strings.TrimSpace(trimmed[len(prefix):])
			repoText, titleText := splitGoalPlanRepoPrefix(rest)
			if repo, err := ParseRepoRef(repoText); err == nil {
				return repo, goalPlanTicketTitle(titleText)
			}
		}
	}
	repoText, titleText := splitGoalPlanRepoPrefix(trimmed)
	if repo, err := ParseRepoRef(repoText); err == nil {
		return repo, goalPlanTicketTitle(titleText)
	}
	return defaultRepo, goalPlanTicketTitle(trimmed)
}

func splitGoalPlanRepoPrefix(value string) (string, string) {
	trimmed := strings.TrimSpace(value)
	for _, sep := range []string{" - ", " — ", ": "} {
		if idx := strings.Index(trimmed, sep); idx > 0 {
			return strings.TrimSpace(trimmed[:idx]), strings.TrimSpace(trimmed[idx+len(sep):])
		}
	}
	fields := strings.Fields(trimmed)
	if len(fields) >= 2 {
		return strings.TrimSpace(fields[0]), strings.TrimSpace(strings.TrimPrefix(trimmed, fields[0]))
	}
	return trimmed, ""
}

func goalPlanTicket(parentRepo RepoRef, goal devStartIssue, title string, targetRepo RepoRef, objective string, scope string) GoalPlanTicket {
	labels := goalPlanLabels(goal.Labels)
	milestone := goal.Milestone
	if targetRepo.FullName() != parentRepo.FullName() {
		milestone = ""
	}
	acceptance := []string{
		fmt.Sprintf("%s is defined with stable behavior", strings.TrimPrefix(title, "[Task] ")),
		"JSON and compact human output are covered by tests or documented examples",
		fmt.Sprintf("Parent goal #%d remains the source of truth", goal.Number),
	}
	evidence := []string{"go test ./internal/gira", "go test ./internal/cli", "go test ./..."}
	body := renderGoalPlanTicketBody(parentRepo, targetRepo, goal.Number, objective, title, scope, acceptance, evidence)
	return GoalPlanTicket{
		Title:            title,
		TargetRepo:       targetRepo.FullName(),
		Type:             "task",
		Priority:         goalPlanPriority(goal.Labels),
		Labels:           labels,
		Milestone:        milestone,
		ParentGoal:       goal.Number,
		Goal:             fmt.Sprintf("%s for goal #%d.", strings.TrimPrefix(title, "[Task] "), goal.Number),
		Scope:            scope,
		Acceptance:       acceptance,
		ExpectedEvidence: evidence,
		Body:             body,
		TicketReadiness:  EvaluateTicketReadiness(body, labels, "open"),
	}
}

func renderGoalPlanTicketBody(parentRepo RepoRef, targetRepo RepoRef, parent int, objective string, title string, scope string, acceptance []string, evidence []string) string {
	var b strings.Builder
	parentRef := fmt.Sprintf("#%d", parent)
	if parentRepo.FullName() != targetRepo.FullName() {
		parentRef = fmt.Sprintf("%s#%d", parentRepo.FullName(), parent)
	}
	fmt.Fprintf(&b, "## Goal\n%s\n\nParent: %s\n\n", strings.TrimPrefix(title, "[Task] "), parentRef)
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

func goalPlanDuplicateChildNumber(title string, targetRepo RepoRef, children []GoalStatusChild) int {
	key := goalPlanTitleKey(title)
	if key == "" {
		return 0
	}
	existing := goalPlanExistingTitleIndexForRepo(children, targetRepo.FullName())
	if duplicate := existing[key]; duplicate > 0 {
		return duplicate
	}
	candidateTokens := goalPlanSignificantTokens(key)
	for _, child := range children {
		if child.Repo != "" && child.Repo != targetRepo.FullName() {
			continue
		}
		if goalPlanFuzzyTitleMatch(candidateTokens, goalPlanSignificantTokens(goalPlanTitleKey(child.Title))) {
			return child.Number
		}
	}
	return 0
}

func goalPlanExistingTitleIndexForRepo(children []GoalStatusChild, targetRepo string) map[string]int {
	index := map[string]int{}
	for _, child := range children {
		if child.Repo != "" && child.Repo != targetRepo {
			continue
		}
		key := goalPlanTitleKey(child.Title)
		if key != "" {
			index[key] = child.Number
		}
	}
	return index
}

func postGoalPlanCreatedChildrenComment(repo RepoRef, goal int, children []GoalPlanChild, runner CommandRunner) error {
	if len(children) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("## Goal Plan Children\n\n")
	b.WriteString("Created child tickets:\n")
	for _, child := range children {
		ref := fmt.Sprintf("#%d", child.Number)
		if child.Repo != "" && child.Repo != repo.FullName() {
			ref = fmt.Sprintf("%s#%d", child.Repo, child.Number)
		}
		fmt.Fprintf(&b, "- %s %s\n", ref, child.Title)
	}
	_, err := runner.Run("gh", "issue", "comment", fmt.Sprintf("%d", goal), "--repo", repo.FullName(), "--body", strings.TrimSpace(b.String()))
	return err
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
		ref := fmt.Sprintf("#%d", child.Number)
		if child.Repo != "" && child.Repo != report.Repo {
			ref = fmt.Sprintf("%s#%d", child.Repo, child.Number)
		}
		fmt.Fprintf(&b, "- created %s %s\n", ref, child.Title)
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
			if ticket.TargetRepo != "" && ticket.TargetRepo != report.Repo {
				fmt.Fprintf(&b, "- [%s] %s\n", ticket.TargetRepo, ticket.Title)
				continue
			}
			fmt.Fprintf(&b, "- %s\n", ticket.Title)
		}
	}
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}
