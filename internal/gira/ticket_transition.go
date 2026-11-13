package gira

import (
	"fmt"
	"strings"
)

const TicketTransitionReportSchemaVersion = "ticket-transition/v1"

type TicketTransitionInput struct {
	Repo   RepoRef
	Ticket int
	To     string
	DryRun bool
}

type TicketTransitionAction struct {
	Action string `json:"action"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// TicketTransitionReport is the result of moving a ticket along one declared
// workflow transition, such as ready-for-release -> done after a deploy.
type TicketTransitionReport struct {
	SchemaVersion string                   `json:"schema_version"`
	Repo          string                   `json:"repo"`
	Ticket        int                      `json:"ticket"`
	DryRun        bool                     `json:"dry_run"`
	Declared      bool                     `json:"declared_workflow"`
	Transition    string                   `json:"transition,omitempty"`
	From          string                   `json:"from"`
	To            string                   `json:"to"`
	Label         string                   `json:"label,omitempty"`
	Category      statusCategory           `json:"category,omitempty"`
	Resolution    string                   `json:"resolution,omitempty"`
	Actions       []TicketTransitionAction `json:"actions"`
	Blockers      []string                 `json:"blockers"`
	NextStep      string                   `json:"next_step,omitempty"`
}

// TransitionTicket moves a ticket to a status through a declared transition.
// Entering a Done-category status closes the issue as completed; leaving the
// Done category reopens it. Without a declared workflow the default applies.
func TransitionTicket(input TicketTransitionInput, runner CommandRunner) (TicketTransitionReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	report := TicketTransitionReport{
		SchemaVersion: TicketTransitionReportSchemaVersion,
		Repo:          input.Repo.FullName(),
		Ticket:        input.Ticket,
		DryRun:        input.DryRun,
		To:            strings.TrimSpace(input.To),
		Actions:       []TicketTransitionAction{},
		Blockers:      []string{},
	}
	if input.Ticket <= 0 {
		return report, fmt.Errorf("ticket must be > 0")
	}
	if report.To == "" {
		return report, fmt.Errorf("--to is required")
	}
	gate, err := loadWorkflowGate(input.Repo)
	if err != nil {
		report.Blockers = append(report.Blockers, workflowBlockerCode(err))
		return report, err
	}
	if !gate.declared {
		gate.workflow = defaultWorkflow()
	}
	report.Declared = gate.declared
	issue, err := fetchDevIssue(input.Repo, input.Ticket, runner)
	if err != nil {
		return report, err
	}
	closed := strings.EqualFold(issue.State, "closed")
	report.From = currentWorkflowStatus(gate.workflow, issue.Labels)
	if report.From == "" && closed {
		report.From = firstStatusInCategory(gate.workflow, statusCategoryDone)
	}
	move := completeStatusMove(gate.workflow, declaredStatusMove{from: report.From, to: report.To})
	report.Label, report.Category = move.label, move.category
	if move.label == "" {
		report.Blockers = append(report.Blockers, "unknown_status")
		return report, fmt.Errorf("unknown_status: %q is not a declared status", report.To)
	}
	if move.blocker != "" {
		report.Blockers = append(report.Blockers, move.blocker)
		return report, fmt.Errorf("%s: %s", move.blocker, move.detail)
	}
	if !move.noOp {
		report.Transition = findWorkflowTransition(gate.workflow, move.from, move.to).Name
	}
	status := plannedOrAppliedStatus(input.DryRun)
	if !gate.hasOnlyStatusLabel(issue.Labels, move.label) {
		report.Actions = append(report.Actions, TicketTransitionAction{Action: "status:set", Status: status, Detail: move.label})
		if !input.DryRun {
			if err := gate.setStatus(input.Repo, input.Ticket, issue.Labels, move, runner); err != nil {
				return report, fmt.Errorf("set status: %w", err)
			}
		}
	}
	done := move.category == statusCategoryDone
	switch {
	case done && !closed:
		report.Resolution = "completed"
		report.Actions = append(report.Actions, TicketTransitionAction{Action: "issue:close", Status: status, Detail: "reason completed"})
		if !input.DryRun {
			if _, err := runner.Run("gh", "issue", "close", fmt.Sprintf("%d", input.Ticket), "--repo", input.Repo.FullName(), "--reason", "completed"); err != nil {
				return report, fmt.Errorf("close issue: %w", err)
			}
		}
	case !done && closed:
		report.Actions = append(report.Actions, TicketTransitionAction{Action: "issue:reopen", Status: status})
		if !input.DryRun {
			if _, err := runner.Run("gh", "issue", "reopen", fmt.Sprintf("%d", input.Ticket), "--repo", input.Repo.FullName()); err != nil {
				return report, fmt.Errorf("reopen issue: %w", err)
			}
		}
	}
	if input.DryRun && len(report.Actions) > 0 {
		report.NextStep = fmt.Sprintf("gira ticket transition %d --repo %s --to %s --apply", input.Ticket, input.Repo.FullName(), report.To)
	}
	return report, nil
}

func firstStatusInCategory(w Workflow, category statusCategory) string {
	for _, status := range w.Statuses {
		if status.Category == category {
			return status.Name
		}
	}
	return ""
}

func FormatTicketTransition(report TicketTransitionReport) string {
	var b strings.Builder
	from := report.From
	if from == "" {
		from = "-"
	}
	fmt.Fprintf(&b, "ticket transition: ticket #%d %s -> %s", report.Ticket, from, report.To)
	if report.Transition != "" {
		fmt.Fprintf(&b, " via %s", report.Transition)
	}
	if len(report.Blockers) > 0 {
		fmt.Fprintf(&b, " blockers=%s", strings.Join(report.Blockers, ","))
	}
	b.WriteString("\n")
	if len(report.Actions) == 0 && len(report.Blockers) == 0 {
		b.WriteString("- already at target\n")
	}
	for _, action := range report.Actions {
		fmt.Fprintf(&b, "- %s %s", action.Status, action.Action)
		if action.Detail != "" {
			fmt.Fprintf(&b, " (%s)", action.Detail)
		}
		b.WriteString("\n")
	}
	if report.NextStep != "" {
		fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	}
	return b.String()
}
