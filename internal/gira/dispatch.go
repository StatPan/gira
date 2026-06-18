package gira

import (
	"fmt"
	"sort"
	"strings"
)

const DispatchPacketSchemaVersion = "dispatch-packet/v1"
const DispatchCompactSchemaVersion = "dispatch-compact/v1"
const DefaultDispatchContextBudget = 12000

type DispatchGoalInput struct {
	Repo    RepoRef `json:"repo"`
	Goal    int     `json:"goal"`
	Role    string  `json:"role"`
	Profile string  `json:"profile"`
}

type DispatchPacket struct {
	Command         string               `json:"command"`
	SchemaVersion   string               `json:"schema_version"`
	Source          DispatchSource       `json:"source"`
	Role            string               `json:"role"`
	Profile         string               `json:"profile"`
	Authority       []DispatchReference  `json:"authority"`
	References      []DispatchReference  `json:"references"`
	Instruction     DispatchInstruction  `json:"instruction"`
	GoalHandoff     *GoalHandoffReport   `json:"goal_handoff,omitempty"`
	WorkerHandoff   *TicketHandoffReport `json:"worker_handoff,omitempty"`
	StopReasons     []string             `json:"stop_reasons,omitempty"`
	NextAction      string               `json:"next_action"`
	NextSafeCommand string               `json:"next_safe_command"`
	PublicSafe      bool                 `json:"public_safe"`
	PrivateStorage  bool                 `json:"private_storage"`
	StorageNotice   string               `json:"storage_notice,omitempty"`
}

type DispatchSource struct {
	Kind   string `json:"kind"`
	Repo   string `json:"repo"`
	Number int    `json:"number,omitempty"`
}

type DispatchReference struct {
	Kind          string `json:"kind"`
	Repo          string `json:"repo,omitempty"`
	Number        int    `json:"number,omitempty"`
	Title         string `json:"title,omitempty"`
	URL           string `json:"url,omitempty"`
	SchemaVersion string `json:"schema_version,omitempty"`
	Role          string `json:"role,omitempty"`
}

type DispatchInstruction struct {
	Objective        string   `json:"objective,omitempty"`
	SelectedWork     string   `json:"selected_work,omitempty"`
	AllowedActions   []string `json:"allowed_actions"`
	StopConditions   []string `json:"stop_conditions,omitempty"`
	EvidenceRequired []string `json:"evidence_required,omitempty"`
}

type DispatchCompactPacket struct {
	Command         string                   `json:"command"`
	SchemaVersion   string                   `json:"schema_version"`
	Source          DispatchSource           `json:"source"`
	Role            string                   `json:"role"`
	Profile         string                   `json:"profile"`
	ContextBudget   int                      `json:"context_budget"`
	EstimatedChars  int                      `json:"estimated_chars"`
	Truncated       bool                     `json:"truncated"`
	Authority       []DispatchReference      `json:"authority,omitempty"`
	References      []DispatchReference      `json:"references,omitempty"`
	Goal            DispatchCompactGoal      `json:"goal"`
	SelectedTicket  *DispatchCompactTicket   `json:"selected_ticket,omitempty"`
	Instruction     DispatchInstruction      `json:"instruction"`
	WorkOrder       DispatchCompactWorkOrder `json:"work_order,omitempty"`
	State           DispatchCompactState     `json:"state"`
	LinkedPR        *DispatchCompactLinkedPR `json:"linked_pr,omitempty"`
	StopReasons     []string                 `json:"stop_reasons,omitempty"`
	NextAction      string                   `json:"next_action"`
	NextSafeCommand string                   `json:"next_safe_command"`
	PublicSafe      bool                     `json:"public_safe"`
	PrivateStorage  bool                     `json:"private_storage"`
	StorageNotice   string                   `json:"storage_notice,omitempty"`
}

type DispatchCompactGoal struct {
	Repo           string   `json:"repo"`
	Number         int      `json:"number"`
	Title          string   `json:"title"`
	State          string   `json:"state,omitempty"`
	Status         string   `json:"status,omitempty"`
	URL            string   `json:"url,omitempty"`
	Objective      string   `json:"objective,omitempty"`
	Direction      string   `json:"direction,omitempty"`
	Scope          string   `json:"scope,omitempty"`
	Autonomy       string   `json:"autonomy,omitempty"`
	StopConditions []string `json:"stop_conditions,omitempty"`
}

type DispatchCompactTicket struct {
	Repo       string   `json:"repo"`
	Number     int      `json:"number"`
	Title      string   `json:"title"`
	State      string   `json:"state,omitempty"`
	Status     string   `json:"status,omitempty"`
	Category   string   `json:"category,omitempty"`
	URL        string   `json:"url,omitempty"`
	Readiness  string   `json:"readiness,omitempty"`
	NextAction string   `json:"next_action,omitempty"`
	NextStep   string   `json:"next_step,omitempty"`
	Blockers   []string `json:"blockers,omitempty"`
}

type DispatchCompactWorkOrder struct {
	Goal             string   `json:"goal,omitempty"`
	Scope            string   `json:"scope,omitempty"`
	Acceptance       []string `json:"acceptance,omitempty"`
	ExpectedDelivery string   `json:"expected_delivery,omitempty"`
	ReviewGuidance   string   `json:"review_guidance,omitempty"`
	RequiredChecks   []string `json:"required_checks,omitempty"`
	ExpectedEvidence []string `json:"expected_evidence,omitempty"`
	BranchBase       string   `json:"branch_base,omitempty"`
	WorkBranch       string   `json:"work_branch,omitempty"`
}

type DispatchCompactState struct {
	Counts                  map[string]int `json:"counts,omitempty"`
	Blockers                []string       `json:"blockers,omitempty"`
	HandoffReceiptPresent   bool           `json:"handoff_receipt_present"`
	RemainingAutonomousWork int            `json:"remaining_autonomous_work"`
}

type DispatchCompactLinkedPR struct {
	Available      bool     `json:"available"`
	Number         int      `json:"number,omitempty"`
	URL            string   `json:"url,omitempty"`
	State          string   `json:"state,omitempty"`
	ReviewDecision string   `json:"review_decision,omitempty"`
	Ready          bool     `json:"ready"`
	Blockers       []string `json:"blockers,omitempty"`
	ChecksStatus   string   `json:"checks_status,omitempty"`
}

func BuildDispatchGoalPacket(input DispatchGoalInput, runner CommandRunner) (DispatchPacket, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	handoff, err := BuildGoalHandoffReport(GoalHandoffInput{
		Repo:    input.Repo,
		Goal:    input.Goal,
		Role:    input.Role,
		Profile: input.Profile,
	}, runner)
	report := dispatchPacketFromGoalHandoff(input.Repo, handoff)
	if err != nil {
		return report, err
	}
	return report, nil
}

func BuildDispatchCompactPacket(packet DispatchPacket, contextBudget int) DispatchCompactPacket {
	budget := normalizeDispatchContextBudget(contextBudget)
	compact := DispatchCompactPacket{
		Command:       "dispatch goal",
		SchemaVersion: DispatchCompactSchemaVersion,
		Source:        packet.Source,
		Role:          packet.Role,
		Profile:       packet.Profile,
		ContextBudget: budget,
		Authority:     append([]DispatchReference(nil), packet.Authority...),
		References:    append([]DispatchReference(nil), packet.References...),
		Instruction: DispatchInstruction{
			Objective:        packet.Instruction.Objective,
			SelectedWork:     packet.Instruction.SelectedWork,
			AllowedActions:   append([]string(nil), packet.Instruction.AllowedActions...),
			StopConditions:   append([]string(nil), packet.Instruction.StopConditions...),
			EvidenceRequired: append([]string(nil), packet.Instruction.EvidenceRequired...),
		},
		StopReasons:     append([]string(nil), packet.StopReasons...),
		NextAction:      packet.NextAction,
		NextSafeCommand: packet.NextSafeCommand,
		PublicSafe:      false,
		PrivateStorage:  true,
		StorageNotice:   "compact dispatch excludes full issue bodies and role packets; fetch referenced issues for audit detail",
	}
	if packet.GoalHandoff != nil {
		compact.Goal = dispatchCompactGoal(*packet.GoalHandoff)
		compact.State = dispatchCompactState(packet.GoalHandoff.GoalStatus)
		if packet.GoalHandoff.SelectedTicket != nil {
			compact.SelectedTicket = dispatchCompactTicket(*packet.GoalHandoff.SelectedTicket, packet.WorkerHandoff)
		}
	}
	if packet.WorkerHandoff != nil {
		compact.WorkOrder = dispatchCompactWorkOrder(*packet.WorkerHandoff)
		compact.LinkedPR = dispatchCompactLinkedPR(packet.WorkerHandoff.LinkedPR)
	}
	compact.EstimatedChars = dispatchCompactEstimatedChars(compact)
	if compact.EstimatedChars > budget {
		compact = trimDispatchCompactPacket(compact, budget)
	}
	compact.EstimatedChars = dispatchCompactEstimatedChars(compact)
	return compact
}

func FormatDispatchPrompt(packet DispatchPacket, contextBudget int) string {
	compact := BuildDispatchCompactPacket(packet, contextBudget)
	var b strings.Builder
	fmt.Fprintf(&b, "# Gira Dispatch\n\n")
	fmt.Fprintf(&b, "Schema: %s\n", compact.SchemaVersion)
	fmt.Fprintf(&b, "Role: %s\n", compact.Role)
	fmt.Fprintf(&b, "Source: %s#%d\n", compact.Source.Repo, compact.Source.Number)
	if compact.Goal.Title != "" {
		fmt.Fprintf(&b, "Goal: #%d %s\n", compact.Goal.Number, compact.Goal.Title)
	}
	fmt.Fprintf(&b, "\n## Next Safe Command\n%s\n", compact.NextSafeCommand)
	if compact.Instruction.Objective != "" {
		fmt.Fprintf(&b, "\n## Objective\n%s\n", compact.Instruction.Objective)
	}
	if compact.SelectedTicket != nil {
		fmt.Fprintf(&b, "\n## Selected Work\n%s#%d %s\n", compact.SelectedTicket.Repo, compact.SelectedTicket.Number, compact.SelectedTicket.Title)
		if compact.SelectedTicket.Readiness != "" {
			fmt.Fprintf(&b, "Readiness: %s\n", compact.SelectedTicket.Readiness)
		}
	}
	dispatchPromptList(&b, "Acceptance", compact.WorkOrder.Acceptance)
	dispatchPromptList(&b, "Required Evidence", compact.WorkOrder.ExpectedEvidence)
	dispatchPromptList(&b, "Required Checks", compact.WorkOrder.RequiredChecks)
	dispatchPromptList(&b, "Stop Conditions", compact.Instruction.StopConditions)
	dispatchPromptList(&b, "Allowed Actions", compact.Instruction.AllowedActions)
	dispatchPromptSection(&b, "Scope", compact.WorkOrder.Scope)
	if len(compact.StopReasons) > 0 {
		dispatchPromptList(&b, "Stop Reasons", compact.StopReasons)
	}
	if compact.LinkedPR != nil && compact.LinkedPR.Available {
		fmt.Fprintf(&b, "\n## Linked PR\n#%d %s ready=%t review=%s checks=%s\n", compact.LinkedPR.Number, compact.LinkedPR.URL, compact.LinkedPR.Ready, compact.LinkedPR.ReviewDecision, compact.LinkedPR.ChecksStatus)
	}
	fmt.Fprintf(&b, "\nContext budget: %d chars; estimated: %d chars; truncated: %t\n", compact.ContextBudget, compact.EstimatedChars, compact.Truncated)
	out := b.String()
	if len(out) > compact.ContextBudget {
		suffix := fmt.Sprintf("\n...[truncated]\nContext budget: %d chars; estimated: %d chars; truncated: true\n", compact.ContextBudget, compact.EstimatedChars)
		if len(suffix) >= compact.ContextBudget {
			return dispatchTruncateString(out, compact.ContextBudget)
		}
		return strings.TrimSpace(dispatchTruncateString(out, compact.ContextBudget-len(suffix))) + suffix
	}
	return out
}

func dispatchPacketFromGoalHandoff(repo RepoRef, handoff GoalHandoffReport) DispatchPacket {
	report := DispatchPacket{
		Command:       "dispatch goal",
		SchemaVersion: DispatchPacketSchemaVersion,
		Source: DispatchSource{
			Kind:   "goal",
			Repo:   repo.FullName(),
			Number: handoff.Goal.Number,
		},
		Role:            handoff.Role,
		Profile:         handoff.Profile,
		GoalHandoff:     &handoff,
		WorkerHandoff:   handoff.WorkerHandoff,
		StopReasons:     append([]string(nil), handoff.StopReasons...),
		NextAction:      handoff.NextAction,
		NextSafeCommand: handoff.NextSafeCommand,
		PublicSafe:      false,
		PrivateStorage:  true,
		StorageNotice:   "dispatch packets embed worker context and are not public-safe by default",
	}
	report.Authority = dispatchAuthorityFromGoalHandoff(handoff)
	report.References = dispatchReferencesFromGoalHandoff(handoff)
	report.Instruction = dispatchInstructionFromGoalHandoff(handoff)
	return report
}

func dispatchAuthorityFromGoalHandoff(handoff GoalHandoffReport) []DispatchReference {
	authority := []DispatchReference{{
		Kind:          "goal",
		Repo:          handoff.Repo,
		Number:        handoff.Goal.Number,
		Title:         handoff.Goal.Title,
		URL:           handoff.Goal.URL,
		SchemaVersion: handoff.SchemaVersion,
	}}
	if handoff.SelectedTicket != nil {
		authority = append(authority, DispatchReference{
			Kind:          "selected_ticket",
			Repo:          dispatchSelectedRepo(handoff),
			Number:        handoff.SelectedTicket.Number,
			Title:         handoff.SelectedTicket.Title,
			URL:           handoff.SelectedTicket.URL,
			SchemaVersion: WorkerHandoffSchemaVersion,
			Role:          handoff.Role,
		})
	}
	return authority
}

func dispatchReferencesFromGoalHandoff(handoff GoalHandoffReport) []DispatchReference {
	refs := []DispatchReference{{
		Kind:   "goal_issue",
		Repo:   handoff.Repo,
		Number: handoff.Goal.Number,
		Title:  handoff.Goal.Title,
		URL:    handoff.Goal.URL,
	}}
	if handoff.SelectedTicket != nil {
		refs = append(refs, DispatchReference{
			Kind:   "selected_ticket_issue",
			Repo:   dispatchSelectedRepo(handoff),
			Number: handoff.SelectedTicket.Number,
			Title:  handoff.SelectedTicket.Title,
			URL:    handoff.SelectedTicket.URL,
		})
	}
	if handoff.WorkerHandoff != nil && handoff.WorkerHandoff.LinkedPR != nil && handoff.WorkerHandoff.LinkedPR.Available {
		refs = append(refs, DispatchReference{
			Kind:   "linked_pr",
			Repo:   handoff.WorkerHandoff.Repo,
			Number: handoff.WorkerHandoff.LinkedPR.Number,
			URL:    handoff.WorkerHandoff.LinkedPR.URL,
		})
	}
	return refs
}

func dispatchInstructionFromGoalHandoff(handoff GoalHandoffReport) DispatchInstruction {
	instruction := DispatchInstruction{
		Objective: strings.TrimSpace(handoff.GoalContext.Objective),
		AllowedActions: []string{
			"Use the parent goal for direction, priority, and stop conditions.",
			"Execute only the selected child ticket unless Gira selects another ticket in a later dispatch.",
			"Use normal Gira ticket lifecycle commands for branch, PR, checks, review, and finish.",
		},
		StopConditions: append([]string(nil), handoff.GoalContext.StopConditions...),
	}
	if handoff.SelectedTicket != nil {
		instruction.SelectedWork = fmt.Sprintf("%s#%d %s", dispatchSelectedRepo(handoff), handoff.SelectedTicket.Number, strings.TrimSpace(handoff.SelectedTicket.Title))
	}
	if handoff.WorkerHandoff != nil {
		instruction.EvidenceRequired = append([]string(nil), handoff.WorkerHandoff.RequiredChecks...)
		instruction.EvidenceRequired = append(instruction.EvidenceRequired, handoff.WorkerHandoff.EvidenceExpectations...)
	}
	return instruction
}

func dispatchSelectedRepo(handoff GoalHandoffReport) string {
	if handoff.SelectedTicket != nil && strings.TrimSpace(handoff.SelectedTicket.Repo) != "" {
		return strings.TrimSpace(handoff.SelectedTicket.Repo)
	}
	return handoff.Repo
}

func FormatDispatchPacket(report DispatchPacket) string {
	var b strings.Builder
	fmt.Fprintf(&b, "dispatch: %s schema=%s source=%s#%d next=%s\n", report.Command, report.SchemaVersion, report.Source.Repo, report.Source.Number, report.NextAction)
	if report.Instruction.SelectedWork != "" {
		fmt.Fprintf(&b, "selected work: %s\n", report.Instruction.SelectedWork)
	}
	if len(report.StopReasons) > 0 {
		fmt.Fprintf(&b, "stop: %s\n", strings.Join(report.StopReasons, ","))
	}
	fmt.Fprintf(&b, "next safe command: %s\n", report.NextSafeCommand)
	return b.String()
}

func dispatchCompactGoal(handoff GoalHandoffReport) DispatchCompactGoal {
	return DispatchCompactGoal{
		Repo:           handoff.Repo,
		Number:         handoff.Goal.Number,
		Title:          handoff.Goal.Title,
		State:          handoff.Goal.State,
		Status:         handoff.Goal.Status,
		URL:            handoff.Goal.URL,
		Objective:      handoff.GoalContext.Objective,
		Direction:      handoff.GoalContext.Direction,
		Scope:          handoff.GoalContext.Scope,
		Autonomy:       handoff.GoalContext.Autonomy,
		StopConditions: append([]string(nil), handoff.GoalContext.StopConditions...),
	}
}

func dispatchCompactState(status GoalStatusReport) DispatchCompactState {
	return DispatchCompactState{
		Counts:                  copyStringIntMap(status.Counts),
		Blockers:                append([]string(nil), status.Blockers...),
		HandoffReceiptPresent:   status.HandoffReceiptPresent,
		RemainingAutonomousWork: status.RemainingAutonomousWork,
	}
}

func dispatchCompactTicket(candidate GoalNextCandidate, worker *TicketHandoffReport) *DispatchCompactTicket {
	ticket := &DispatchCompactTicket{
		Repo:       candidate.Repo,
		Number:     candidate.Number,
		Title:      candidate.Title,
		State:      candidate.State,
		Status:     candidate.Status,
		Category:   candidate.Category,
		URL:        candidate.URL,
		NextAction: candidate.NextAction,
		NextStep:   candidate.NextStep,
		Blockers:   append([]string(nil), candidate.Blockers...),
	}
	if ticket.Repo == "" && worker != nil {
		ticket.Repo = worker.Repo
	}
	if worker != nil {
		ticket.Readiness = worker.Readiness.Readiness
	}
	return ticket
}

func dispatchCompactWorkOrder(worker TicketHandoffReport) DispatchCompactWorkOrder {
	return DispatchCompactWorkOrder{
		Goal:             strings.TrimSpace(worker.WorkOrder.Goal),
		Scope:            strings.TrimSpace(worker.WorkOrder.Scope),
		Acceptance:       append([]string(nil), worker.WorkOrder.Acceptance...),
		ExpectedDelivery: strings.TrimSpace(worker.WorkOrder.ExpectedDelivery),
		ReviewGuidance:   strings.TrimSpace(worker.WorkOrder.ReviewGuidance),
		RequiredChecks:   append([]string(nil), worker.RequiredChecks...),
		ExpectedEvidence: append([]string(nil), worker.EvidenceExpectations...),
		BranchBase:       worker.BranchPolicy.Base,
		WorkBranch:       worker.BranchPolicy.WorkBranch,
	}
}

func dispatchCompactLinkedPR(linked *TicketHandoffLinkedPR) *DispatchCompactLinkedPR {
	if linked == nil {
		return nil
	}
	return &DispatchCompactLinkedPR{
		Available:      linked.Available,
		Number:         linked.Number,
		URL:            linked.URL,
		State:          linked.State,
		ReviewDecision: linked.ReviewDecision,
		Ready:          linked.Ready,
		Blockers:       append([]string(nil), linked.Blockers...),
		ChecksStatus:   dispatchChecksStatus(linked.Checks),
	}
}

func dispatchChecksStatus(checks []DevPRCheck) string {
	if len(checks) == 0 {
		return ""
	}
	counts := map[string]int{}
	for _, check := range checks {
		status := strings.TrimSpace(check.Conclusion)
		if status == "" {
			status = strings.TrimSpace(check.Status)
		}
		if status == "" {
			status = "unknown"
		}
		counts[status]++
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, counts[key]))
	}
	return strings.Join(parts, ",")
}

func normalizeDispatchContextBudget(value int) int {
	if value <= 0 {
		return DefaultDispatchContextBudget
	}
	if value < 1200 {
		return 1200
	}
	return value
}

func trimDispatchCompactPacket(packet DispatchCompactPacket, budget int) DispatchCompactPacket {
	packet.Truncated = true
	packet.Goal.Objective = dispatchTruncateString(packet.Goal.Objective, 900)
	packet.Goal.Direction = dispatchTruncateString(packet.Goal.Direction, 800)
	packet.Goal.Scope = dispatchTruncateString(packet.Goal.Scope, 800)
	packet.Goal.Autonomy = dispatchTruncateString(packet.Goal.Autonomy, 500)
	packet.Instruction.Objective = dispatchTruncateString(packet.Instruction.Objective, 900)
	packet.Instruction.SelectedWork = dispatchTruncateString(packet.Instruction.SelectedWork, 300)
	packet.WorkOrder.Scope = dispatchTruncateString(packet.WorkOrder.Scope, 1000)
	packet.WorkOrder.ExpectedDelivery = dispatchTruncateString(packet.WorkOrder.ExpectedDelivery, 500)
	packet.WorkOrder.ReviewGuidance = dispatchTruncateString(packet.WorkOrder.ReviewGuidance, 500)
	packet.Instruction.AllowedActions = dispatchLimitList(packet.Instruction.AllowedActions, 3, 260)
	packet.Instruction.StopConditions = dispatchLimitList(packet.Instruction.StopConditions, 6, 260)
	packet.Instruction.EvidenceRequired = dispatchLimitList(packet.Instruction.EvidenceRequired, 8, 260)
	packet.WorkOrder.Acceptance = dispatchLimitList(packet.WorkOrder.Acceptance, 8, 260)
	packet.WorkOrder.RequiredChecks = dispatchLimitList(packet.WorkOrder.RequiredChecks, 6, 260)
	packet.WorkOrder.ExpectedEvidence = dispatchLimitList(packet.WorkOrder.ExpectedEvidence, 6, 260)
	packet.State.Blockers = dispatchLimitList(packet.State.Blockers, 8, 220)
	return packet
}

func dispatchCompactEstimatedChars(packet DispatchCompactPacket) int {
	count := len(packet.Command) + len(packet.SchemaVersion) + len(packet.Source.Repo) + len(packet.Role) + len(packet.Profile)
	count += dispatchReferencesChars(packet.Authority) + dispatchReferencesChars(packet.References)
	count += len(packet.Goal.Title) + len(packet.Goal.Objective) + len(packet.Goal.Direction) + len(packet.Goal.Scope) + len(packet.Goal.Autonomy)
	count += dispatchListChars(packet.Goal.StopConditions)
	if packet.SelectedTicket != nil {
		count += len(packet.SelectedTicket.Repo) + len(packet.SelectedTicket.Title) + len(packet.SelectedTicket.State) + len(packet.SelectedTicket.Status) + len(packet.SelectedTicket.Category)
		count += dispatchListChars(packet.SelectedTicket.Blockers)
	}
	count += len(packet.Instruction.Objective) + len(packet.Instruction.SelectedWork)
	count += dispatchListChars(packet.Instruction.AllowedActions) + dispatchListChars(packet.Instruction.StopConditions) + dispatchListChars(packet.Instruction.EvidenceRequired)
	count += len(packet.WorkOrder.Goal) + len(packet.WorkOrder.Scope) + len(packet.WorkOrder.ExpectedDelivery) + len(packet.WorkOrder.ReviewGuidance)
	count += dispatchListChars(packet.WorkOrder.Acceptance) + dispatchListChars(packet.WorkOrder.RequiredChecks) + dispatchListChars(packet.WorkOrder.ExpectedEvidence)
	count += dispatchListChars(packet.State.Blockers) + dispatchListChars(packet.StopReasons) + len(packet.NextAction) + len(packet.NextSafeCommand)
	if packet.LinkedPR != nil {
		count += len(packet.LinkedPR.URL) + len(packet.LinkedPR.State) + len(packet.LinkedPR.ReviewDecision) + len(packet.LinkedPR.ChecksStatus) + dispatchListChars(packet.LinkedPR.Blockers)
	}
	return count
}

func dispatchReferencesChars(refs []DispatchReference) int {
	count := 0
	for _, ref := range refs {
		count += len(ref.Kind) + len(ref.Repo) + len(ref.Title) + len(ref.URL) + len(ref.SchemaVersion) + len(ref.Role)
	}
	return count
}

func dispatchListChars(values []string) int {
	count := 0
	for _, value := range values {
		count += len(value)
	}
	return count
}

func dispatchLimitList(values []string, limit int, itemLimit int) []string {
	out := []string{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if len(out) >= limit {
			break
		}
		out = append(out, dispatchTruncateString(trimmed, itemLimit))
	}
	return out
}

func dispatchTruncateString(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit <= 20 {
		return value[:limit]
	}
	return strings.TrimSpace(value[:limit-15]) + " ...[truncated]"
}

func dispatchPromptSection(b *strings.Builder, title string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	fmt.Fprintf(b, "\n## %s\n%s\n", title, value)
}

func dispatchPromptList(b *strings.Builder, title string, values []string) {
	values = dispatchLimitList(values, len(values), 500)
	if len(values) == 0 {
		return
	}
	fmt.Fprintf(b, "\n## %s\n", title)
	for _, value := range values {
		fmt.Fprintf(b, "- %s\n", value)
	}
}
