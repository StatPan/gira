package gira

import (
	"fmt"
	"strings"
)

const GoalNewReportSchemaVersion = "goal-new-report/v1"

type GoalNewInput struct {
	Repo           RepoRef  `json:"repo"`
	Title          string   `json:"title"`
	Objective      string   `json:"objective,omitempty"`
	Direction      string   `json:"direction,omitempty"`
	Scope          string   `json:"scope,omitempty"`
	Autonomy       string   `json:"autonomy,omitempty"`
	Decomposition  []string `json:"decomposition,omitempty"`
	QualityBar     []string `json:"quality_bar,omitempty"`
	StopConditions []string `json:"stop_conditions,omitempty"`
	Body           string   `json:"body,omitempty"`
	Type           string   `json:"type"`
	Priority       string   `json:"priority,omitempty"`
	Milestone      string   `json:"milestone,omitempty"`
	Labels         []string `json:"labels,omitempty"`
	DryRun         bool     `json:"dry_run"`
}

type GoalNewReport struct {
	Command       string             `json:"command"`
	SchemaVersion string             `json:"schema_version"`
	Repo          string             `json:"repo"`
	Title         string             `json:"title"`
	DryRun        bool               `json:"dry_run"`
	Type          string             `json:"type"`
	Priority      string             `json:"priority,omitempty"`
	Labels        []string           `json:"labels"`
	Milestone     string             `json:"milestone,omitempty"`
	Body          string             `json:"body"`
	Created       TicketCreatedIssue `json:"created,omitempty"`
	NextStep      string             `json:"next_step"`
	Approval      *ApprovalEvidence  `json:"approval,omitempty"`
	Warnings      []string           `json:"warnings,omitempty"`
}

func BuildGoalNewReport(input GoalNewInput, runner CommandRunner) (GoalNewReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return GoalNewReport{}, fmt.Errorf("goal title is required")
	}
	goalType := strings.ToLower(strings.TrimSpace(input.Type))
	if goalType == "" {
		goalType = "epic"
	}
	if !validGoalType(goalType) {
		return GoalNewReport{}, fmt.Errorf("--type must be one of epic, goal")
	}
	priority := strings.TrimSpace(input.Priority)
	if priority != "" && !validTicketPriority(priority) {
		return GoalNewReport{}, fmt.Errorf("--priority must be one of p0, p1, p2, p3")
	}
	body := strings.TrimSpace(input.Body)
	if body == "" {
		body = renderGoalNewBody(input, title)
	}
	labels := ticketNewLabels(goalType, priority, input.Labels)
	report := GoalNewReport{
		Command:       "goal new",
		SchemaVersion: GoalNewReportSchemaVersion,
		Repo:          input.Repo.FullName(),
		Title:         title,
		DryRun:        input.DryRun,
		Type:          goalType,
		Priority:      priority,
		Labels:        labels,
		Milestone:     strings.TrimSpace(input.Milestone),
		Body:          body,
		NextStep:      "gira goal new --apply",
	}
	if err := preflightTicketNewLabels(input.Repo, labels, runner); err != nil {
		return report, err
	}
	if input.DryRun {
		report.Approval = GoalNewApprovalEvidence(report)
		return report, nil
	}
	created, err := createRepoTicket(input.Repo, title, body, labels, report.Milestone, runner)
	if err != nil {
		return report, err
	}
	report.Created = created
	report.NextStep = fmt.Sprintf("gira goal status %d --repo %s --json", created.Number, input.Repo.FullName())
	return report, nil
}

func validGoalType(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "epic", "goal":
		return true
	default:
		return false
	}
}

func renderGoalNewBody(input GoalNewInput, title string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Goal\n%s\n\n", goalNewSection(input.Objective, title))
	fmt.Fprintf(&b, "## Direction\n%s\n\n", noResponse(input.Direction))
	fmt.Fprintf(&b, "## Scope\n%s\n\n", goalNewSection(input.Scope, "Bound the first child ticket to the objective above and its independently verifiable outcome."))
	fmt.Fprintf(&b, "## Autonomy\n%s\n\n", noResponse(input.Autonomy))
	b.WriteString("## Decomposition\n")
	if len(nonEmptyGoalNewValues(input.Decomposition)) == 0 {
		b.WriteString("- Define the first independently verifiable child ticket.\n")
	} else {
		writeGoalNewList(&b, input.Decomposition)
	}
	b.WriteString("\n## Quality Bar\n")
	writeGoalNewList(&b, input.QualityBar)
	b.WriteString("\n## Stop Conditions\n")
	writeGoalNewList(&b, input.StopConditions)
	b.WriteString("\n## Child Tickets\n_No child tickets yet._\n\n")
	b.WriteString(DefaultProvenanceBlock())
	b.WriteString("\n")
	return b.String()
}

func nonEmptyGoalNewValues(values []string) []string {
	result := []string{}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, value)
		}
	}
	return result
}

func goalNewSection(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return strings.TrimSpace(fallback)
	}
	return strings.TrimSpace(value)
}

func writeGoalNewList(b *strings.Builder, values []string) {
	wrote := false
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		fmt.Fprintf(b, "- %s\n", trimmed)
		wrote = true
	}
	if !wrote {
		b.WriteString("_No response_\n")
	}
}

func FormatGoalNew(report GoalNewReport) string {
	if report.DryRun {
		var b strings.Builder
		fmt.Fprintf(&b, "goal new: dry-run %s\n", report.Title)
		fmt.Fprintf(&b, "repo: %s\n", report.Repo)
		fmt.Fprintf(&b, "labels: %s\n", strings.Join(report.Labels, ","))
		if strings.TrimSpace(report.Milestone) != "" {
			fmt.Fprintf(&b, "milestone: %s\n", report.Milestone)
		}
		fmt.Fprintf(&b, "body:\n%s\nnext step: %s\n", strings.TrimSpace(report.Body), report.NextStep)
		return b.String()
	}
	return fmt.Sprintf("goal new: goal #%d %s\nnext step: %s\n", report.Created.Number, report.Title, report.NextStep)
}
