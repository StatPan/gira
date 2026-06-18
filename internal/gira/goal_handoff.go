package gira

import (
	"fmt"
	"strings"
)

const GoalHandoffSchemaVersion = "goal-handoff/v1"

type GoalHandoffInput struct {
	Repo    RepoRef `json:"repo"`
	Goal    int     `json:"goal"`
	Role    string  `json:"role"`
	Profile string  `json:"profile"`
}

type GoalHandoffReport struct {
	Command         string               `json:"command"`
	SchemaVersion   string               `json:"schema_version"`
	Repo            string               `json:"repo"`
	Role            string               `json:"role"`
	Profile         string               `json:"profile"`
	Goal            GoalStatusIssue      `json:"goal"`
	GoalContext     GoalHandoffContext   `json:"goal_context"`
	GoalStatus      GoalStatusReport     `json:"goal_status"`
	GoalNext        GoalNextReport       `json:"goal_next"`
	SelectedTicket  *GoalNextCandidate   `json:"selected_ticket,omitempty"`
	WorkerHandoff   *TicketHandoffReport `json:"worker_handoff,omitempty"`
	Blockers        []string             `json:"blockers,omitempty"`
	StopReasons     []string             `json:"stop_reasons,omitempty"`
	NextAction      string               `json:"next_action"`
	NextSafeCommand string               `json:"next_safe_command"`
	PublicSafe      bool                 `json:"public_safe"`
	PrivateStorage  bool                 `json:"private_storage"`
	StorageNotice   string               `json:"storage_notice,omitempty"`
}

type GoalHandoffContext struct {
	Objective      string   `json:"objective,omitempty"`
	Direction      string   `json:"direction,omitempty"`
	Scope          string   `json:"scope,omitempty"`
	Autonomy       string   `json:"autonomy,omitempty"`
	Decomposition  []string `json:"decomposition,omitempty"`
	QualityBar     []string `json:"quality_bar,omitempty"`
	StopConditions []string `json:"stop_conditions,omitempty"`
}

func BuildGoalHandoffReport(input GoalHandoffInput, runner CommandRunner) (GoalHandoffReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	role, err := normalizeAgentPromptRole(input.Role)
	if err != nil {
		return GoalHandoffReport{}, err
	}
	profile, err := normalizeAgentPromptProfile(input.Profile)
	if err != nil {
		return GoalHandoffReport{}, err
	}
	goalNumber, _, err := ResolveGoalNumber(input.Repo, input.Goal, runner)
	if err != nil {
		return GoalHandoffReport{}, err
	}
	input.Goal = goalNumber
	status, err := BuildGoalStatusReport(GoalStatusInput{Repo: input.Repo, Goal: input.Goal}, runner)
	if err != nil {
		return GoalHandoffReport{}, err
	}
	next := BuildGoalNextReportFromStatus(input.Repo, status)
	goal, err := fetchDevIssue(input.Repo, input.Goal, runner)
	if err != nil {
		return GoalHandoffReport{}, err
	}
	report := GoalHandoffReport{
		Command:         "goal handoff",
		SchemaVersion:   GoalHandoffSchemaVersion,
		Repo:            input.Repo.FullName(),
		Role:            role,
		Profile:         profile,
		Goal:            status.Goal,
		GoalContext:     goalHandoffContext(goal.Body),
		GoalStatus:      status,
		GoalNext:        next,
		Blockers:        append([]string(nil), next.Blockers...),
		StopReasons:     append([]string(nil), next.StopReasons...),
		NextAction:      next.NextAction,
		NextSafeCommand: next.NextStep,
		PublicSafe:      false,
		PrivateStorage:  true,
		StorageNotice:   "goal handoff embeds worker context and is not public-safe by default",
	}
	if next.SelectedTicket == nil {
		return report, nil
	}
	selected := *next.SelectedTicket
	report.SelectedTicket = &selected
	childRepo := input.Repo
	if repo, err := ParseRepoRef(selected.Repo); err == nil {
		childRepo = repo
	}
	handoff, err := BuildTicketHandoffReport(TicketHandoffInput{
		Repo:         childRepo,
		Ticket:       selected.Number,
		Role:         role,
		Profile:      profile,
		ContextNotes: goalHandoffContextNotes(status, report.GoalContext),
	}, runner)
	if err != nil {
		report.StopReasons = appendUniqueStrings(report.StopReasons, "worker_handoff_unavailable")
		report.Blockers = appendUniqueStrings(report.Blockers, "worker_handoff_unavailable:"+err.Error())
		report.NextAction = "inspect_child"
		report.NextSafeCommand = fmt.Sprintf("gira ticket handoff %d --repo %s --role %s --profile %s --json", selected.Number, childRepo.FullName(), role, profile)
		return report, err
	}
	report.WorkerHandoff = &handoff
	report.NextAction = "handoff_child"
	report.NextSafeCommand = handoff.NextSafeCommand
	return report, nil
}

func goalHandoffContext(body string) GoalHandoffContext {
	return GoalHandoffContext{
		Objective:      strings.TrimSpace(markdownSection(body, "Goal")),
		Direction:      strings.TrimSpace(markdownSection(body, "Direction")),
		Scope:          strings.TrimSpace(markdownSection(body, "Scope")),
		Autonomy:       strings.TrimSpace(markdownSection(body, "Autonomy")),
		Decomposition:  markdownListSection(body, "Decomposition"),
		QualityBar:     markdownListSection(body, "Quality Bar"),
		StopConditions: markdownListSection(body, "Stop Conditions"),
	}
}

func goalHandoffContextNotes(status GoalStatusReport, context GoalHandoffContext) []string {
	notes := []string{
		fmt.Sprintf("Parent goal #%d: %s", status.Goal.Number, strings.TrimSpace(status.Goal.Title)),
		"Operate within the selected child ticket; use the parent goal only for direction and stop conditions.",
	}
	if strings.TrimSpace(context.Objective) != "" {
		notes = append(notes, "Goal objective: "+strings.TrimSpace(context.Objective))
	}
	if strings.TrimSpace(context.Direction) != "" {
		notes = append(notes, "Goal direction: "+strings.TrimSpace(context.Direction))
	}
	if strings.TrimSpace(context.Autonomy) != "" {
		notes = append(notes, "Goal autonomy: "+strings.TrimSpace(context.Autonomy))
	}
	if len(context.StopConditions) > 0 {
		notes = append(notes, "Goal stop conditions: "+strings.Join(context.StopConditions, "; "))
	}
	return notes
}

func FormatGoalHandoff(report GoalHandoffReport) string {
	var b strings.Builder
	if report.SelectedTicket == nil {
		fmt.Fprintf(&b, "goal handoff: #%d stop=%s next=%s\n", report.Goal.Number, strings.Join(report.StopReasons, ","), report.NextAction)
		if len(report.Blockers) > 0 {
			fmt.Fprintf(&b, "blockers: %s\n", strings.Join(report.Blockers, ","))
		}
		fmt.Fprintf(&b, "next safe command: %s\n", report.NextSafeCommand)
		return b.String()
	}
	fmt.Fprintf(&b, "goal handoff: #%d selected=%s#%d role=%s\n", report.Goal.Number, report.SelectedTicket.Repo, report.SelectedTicket.Number, report.Role)
	fmt.Fprintf(&b, "goal next: %s\n", report.GoalNext.NextAction)
	if report.WorkerHandoff != nil {
		fmt.Fprintf(&b, "worker handoff: schema=%s readiness=%s next=%s\n", report.WorkerHandoff.SchemaVersion, report.WorkerHandoff.Readiness.Readiness, report.WorkerHandoff.NextAction)
	}
	fmt.Fprintf(&b, "next safe command: %s\n", report.NextSafeCommand)
	return b.String()
}
