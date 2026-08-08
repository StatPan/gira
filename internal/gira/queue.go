package gira

import (
	"fmt"
	"strings"
)

const (
	QueueListSchemaVersion    = "queue-list/v1"
	QueueNextSchemaVersion    = "queue-next/v1"
	QueueHandoffSchemaVersion = "queue-handoff/v1"
	QueueTakeSchemaVersion    = "queue-take/v1"
)

type QueueFilterSummary struct {
	Queues []string `json:"queues,omitempty"`
	Repos  []string `json:"repos,omitempty"`
	Limit  int      `json:"limit,omitempty"`
}

type QueueListOptions struct {
	QueueNames  []string
	RepoFilters []string
	Limit       int
}

type QueueListReport struct {
	SchemaVersion  string               `json:"schema_version"`
	Command        string               `json:"command"`
	Workspace      WorkspaceSummary     `json:"workspace"`
	SourceContract string               `json:"source_contract"`
	ConfigPath     string               `json:"config_path,omitempty"`
	Filters        QueueFilterSummary   `json:"filters,omitempty"`
	Queues         WorkspaceQueues      `json:"queues"`
	Counts         WorkspaceQueueCounts `json:"counts"`
	Items          []WorkspaceQueueItem `json:"items"`
	Warnings       []string             `json:"warnings,omitempty"`
	FetchedAt      string               `json:"fetched_at,omitempty"`
}

type QueueNextOptions struct {
	RepoFilters []string
	Role        string
	Profile     string
}

type QueueNextReport struct {
	SchemaVersion  string               `json:"schema_version"`
	Command        string               `json:"command"`
	Workspace      WorkspaceSummary     `json:"workspace"`
	SourceContract string               `json:"source_contract"`
	ConfigPath     string               `json:"config_path,omitempty"`
	Filters        QueueFilterSummary   `json:"filters,omitempty"`
	Counts         WorkspaceQueueCounts `json:"counts"`
	Selected       *QueueNextSelection  `json:"selected"`
	StopReasons    []string             `json:"stop_reasons,omitempty"`
	NextAction     string               `json:"next_action"`
	NextStep       string               `json:"next_step"`
	Warnings       []string             `json:"warnings,omitempty"`
	FetchedAt      string               `json:"fetched_at,omitempty"`
}

type QueueNextSelection struct {
	WorkspaceQueueItem
	SelectionReason string `json:"selection_reason"`
	HandoffCommand  string `json:"handoff_command"`
	RunCommand      string `json:"run_command"`
}

type QueueHandoffReport struct {
	SchemaVersion  string               `json:"schema_version"`
	Command        string               `json:"command"`
	Workspace      WorkspaceSummary     `json:"workspace"`
	SourceContract string               `json:"source_contract"`
	ConfigPath     string               `json:"config_path,omitempty"`
	Filters        QueueFilterSummary   `json:"filters,omitempty"`
	Counts         WorkspaceQueueCounts `json:"counts"`
	Selected       *QueueNextSelection  `json:"selected"`
	WorkerHandoff  *TicketHandoffReport `json:"worker_handoff"`
	Role           string               `json:"role"`
	Profile        string               `json:"profile"`
	HandoffCommand string               `json:"handoff_command,omitempty"`
	RunCommand     string               `json:"run_command,omitempty"`
	StopReasons    []string             `json:"stop_reasons,omitempty"`
	NextAction     string               `json:"next_action"`
	NextStep       string               `json:"next_step"`
	Warnings       []string             `json:"warnings,omitempty"`
	FetchedAt      string               `json:"fetched_at,omitempty"`
}

type QueueTakeReport struct {
	SchemaVersion string             `json:"schema_version"`
	Command       string             `json:"command"`
	DryRun        bool               `json:"dry_run"`
	Apply         bool               `json:"apply"`
	Handoff       QueueHandoffReport `json:"handoff"`
	StartResult   *WorkStartResult   `json:"start_result"`
	StopReasons   []string           `json:"stop_reasons,omitempty"`
	NextAction    string             `json:"next_action"`
	NextStep      string             `json:"next_step"`
	Approval      *ApprovalEvidence  `json:"approval,omitempty"`
	Warnings      []string           `json:"warnings,omitempty"`
	FetchedAt     string             `json:"fetched_at,omitempty"`
}

func BuildQueueListReport(workspaceReport WorkspaceReport, options QueueListOptions) (QueueListReport, error) {
	if options.Limit < 0 {
		return QueueListReport{}, fmt.Errorf("queue list limit must be at least 0")
	}
	queueNames, err := NormalizeWorkspaceQueueNames(options.QueueNames)
	if err != nil {
		return QueueListReport{}, err
	}
	selected := map[string]bool{}
	for _, queue := range queueNames {
		selected[queue] = true
	}
	repoFilters := normalizedQueueRepoFilters(options.RepoFilters)
	out := QueueListReport{
		SchemaVersion:  QueueListSchemaVersion,
		Command:        "queue list",
		Workspace:      workspaceReport.Workspace,
		SourceContract: WorkspaceQueuesSchemaVersion,
		ConfigPath:     workspaceReport.ConfigPath,
		Filters: QueueFilterSummary{
			Queues: queueNames,
			Repos:  append([]string(nil), options.RepoFilters...),
			Limit:  options.Limit,
		},
		Queues:    emptyWorkspaceQueues(),
		Items:     []WorkspaceQueueItem{},
		Warnings:  append([]string(nil), workspaceReport.Warnings...),
		FetchedAt: workspaceReport.FetchedAt,
	}
	for _, queue := range WorkspaceQueueOrder() {
		if !selected[queue] {
			continue
		}
		for _, item := range workspaceQueueItemsByName(workspaceReport.Queues.Queues, queue) {
			if len(repoFilters) > 0 && !repoFilters[strings.ToLower(item.Repo)] {
				continue
			}
			if options.Limit > 0 && len(out.Items) >= options.Limit {
				break
			}
			addWorkspaceQueueItem(&out.Queues, item)
			out.Items = append(out.Items, item)
		}
		if options.Limit > 0 && len(out.Items) >= options.Limit {
			break
		}
	}
	out.Counts = countWorkspaceQueues(out.Queues)
	return out, nil
}

func BuildQueueNextReport(workspaceReport WorkspaceReport, options QueueNextOptions) (QueueNextReport, error) {
	role, err := NormalizeQueueRole(options.Role)
	if err != nil {
		return QueueNextReport{}, err
	}
	profile, err := NormalizeQueueProfile(options.Profile)
	if err != nil {
		return QueueNextReport{}, err
	}
	list, err := BuildQueueListReport(workspaceReport, QueueListOptions{
		QueueNames:  []string{"agent_ready"},
		RepoFilters: options.RepoFilters,
		Limit:       1,
	})
	if err != nil {
		return QueueNextReport{}, err
	}
	report := QueueNextReport{
		SchemaVersion:  QueueNextSchemaVersion,
		Command:        "queue next",
		Workspace:      workspaceReport.Workspace,
		SourceContract: WorkspaceQueuesSchemaVersion,
		ConfigPath:     workspaceReport.ConfigPath,
		Filters: QueueFilterSummary{
			Queues: []string{"agent_ready"},
			Repos:  append([]string(nil), options.RepoFilters...),
			Limit:  1,
		},
		Counts:    workspaceReport.Queues.Counts,
		Warnings:  append([]string(nil), workspaceReport.Warnings...),
		FetchedAt: workspaceReport.FetchedAt,
	}
	if len(list.Items) == 0 {
		report.NextAction = "inspect_queues"
		report.StopReasons = queueNextStopReasons(workspaceReport.Queues.Counts)
		report.NextStep = queueListNextStep(workspaceReport.ConfigPath, options.RepoFilters)
		return report, nil
	}
	item := list.Items[0]
	selection := QueueNextSelection{
		WorkspaceQueueItem: item,
		SelectionReason:    queueSelectionReason(item),
		HandoffCommand:     QueueHandoffCommand(item, role, profile),
		RunCommand:         QueueRunCommand(item, role, profile),
	}
	report.Selected = &selection
	report.NextAction = "handoff_ticket"
	report.NextStep = selection.HandoffCommand
	return report, nil
}

func BuildQueueHandoffReportFromNext(next QueueNextReport, handoff *TicketHandoffReport, role string, profile string) QueueHandoffReport {
	role, _ = NormalizeQueueRole(role)
	profile, _ = NormalizeQueueProfile(profile)
	report := QueueHandoffReport{
		SchemaVersion:  QueueHandoffSchemaVersion,
		Command:        "queue handoff",
		Workspace:      next.Workspace,
		SourceContract: next.SourceContract,
		ConfigPath:     next.ConfigPath,
		Filters:        next.Filters,
		Counts:         next.Counts,
		Selected:       next.Selected,
		WorkerHandoff:  handoff,
		Role:           role,
		Profile:        profile,
		Warnings:       append([]string(nil), next.Warnings...),
		FetchedAt:      next.FetchedAt,
	}
	if next.Selected == nil {
		report.StopReasons = append([]string(nil), next.StopReasons...)
		report.NextAction = "inspect_queues"
		report.NextStep = next.NextStep
		return report
	}
	report.HandoffCommand = next.Selected.HandoffCommand
	report.RunCommand = next.Selected.RunCommand
	if handoff == nil {
		report.StopReasons = []string{"missing_worker_handoff"}
		report.NextAction = "inspect_ticket"
		report.NextStep = next.Selected.HandoffCommand
		return report
	}
	if !QueueWorkerHandoffReady(*handoff) {
		report.StopReasons = QueueHandoffStopReasonsForWorkerHandoff(*handoff)
		report.NextAction = handoff.NextAction
		if strings.TrimSpace(report.NextAction) == "" {
			report.NextAction = "inspect_ticket"
		}
		report.NextStep = handoff.NextSafeCommand
		if strings.TrimSpace(report.NextStep) == "" {
			report.NextStep = next.Selected.HandoffCommand
		}
		return report
	}
	report.NextAction = "start_run"
	report.NextStep = next.Selected.RunCommand
	return report
}

func BuildQueueHandoffStopReport(workspaceReport WorkspaceReport, filters QueueFilterSummary, role string, profile string, stopReasons []string, nextStep string) QueueHandoffReport {
	role, _ = NormalizeQueueRole(role)
	profile, _ = NormalizeQueueProfile(profile)
	return QueueHandoffReport{
		SchemaVersion:  QueueHandoffSchemaVersion,
		Command:        "queue handoff",
		Workspace:      workspaceReport.Workspace,
		SourceContract: WorkspaceQueuesSchemaVersion,
		ConfigPath:     workspaceReport.ConfigPath,
		Filters:        filters,
		Counts:         workspaceReport.Queues.Counts,
		Selected:       nil,
		WorkerHandoff:  nil,
		Role:           role,
		Profile:        profile,
		StopReasons:    append([]string(nil), stopReasons...),
		NextAction:     "inspect_queues",
		NextStep:       nextStep,
		Warnings:       append([]string(nil), workspaceReport.Warnings...),
		FetchedAt:      workspaceReport.FetchedAt,
	}
}

func BuildQueueTakeReport(handoff QueueHandoffReport, start *WorkStartResult, dryRun bool, apply bool) QueueTakeReport {
	report := QueueTakeReport{
		SchemaVersion: QueueTakeSchemaVersion,
		Command:       "queue take",
		DryRun:        dryRun,
		Apply:         apply,
		Handoff:       handoff,
		Warnings:      append([]string(nil), handoff.Warnings...),
		FetchedAt:     handoff.FetchedAt,
	}
	if handoff.Selected == nil || len(handoff.StopReasons) > 0 || handoff.WorkerHandoff == nil {
		report.StopReasons = append([]string(nil), handoff.StopReasons...)
		if handoff.Selected == nil && len(report.StopReasons) == 0 {
			report.StopReasons = append(report.StopReasons, "missing_queue_selection")
		}
		if handoff.Selected != nil && handoff.WorkerHandoff == nil && len(report.StopReasons) == 0 {
			report.StopReasons = append(report.StopReasons, "missing_worker_handoff")
		}
		report.NextAction = strings.TrimSpace(handoff.NextAction)
		if report.NextAction == "" {
			report.NextAction = "inspect_queues"
		}
		report.NextStep = handoff.NextStep
		return report
	}
	if start == nil {
		report.StopReasons = []string{"missing_start_result"}
		report.NextAction = "start_ticket"
		report.NextStep = QueueTakeCommand(handoff.Selected.WorkspaceQueueItem, handoff.Role, handoff.Profile, "--apply")
		return report
	}
	EnsureWorkStartResultSchema(start)
	if dryRun && start.Approval == nil {
		start.Approval = WorkStartApprovalEvidence(*start, "gira ticket start")
	}
	report.StartResult = start
	if start.SelectionRequired {
		report.StopReasons = []string{"branch_strategy_required"}
		report.NextAction = "choose_branch_strategy"
		report.NextStep = start.NextStep
		return report
	}
	if dryRun {
		report.NextAction = "apply_ticket_start"
		report.NextStep = queueTakeCommandForStart(handoff.Selected.WorkspaceQueueItem, handoff.Role, handoff.Profile, "--apply", start)
		report.Approval = QueueTakeApprovalEvidence(report)
		return report
	}
	report.NextAction = "handoff_ticket"
	report.NextStep = handoff.HandoffCommand
	if strings.TrimSpace(report.NextStep) == "" {
		report.NextStep = handoff.Selected.HandoffCommand
	}
	return report
}

func QueueTakeApprovalEvidence(report QueueTakeReport) *ApprovalEvidence {
	if report.Handoff.Selected == nil {
		return nil
	}
	item := *report.Handoff.Selected
	queueItem := item.WorkspaceQueueItem
	actions := []ApprovalPlannedAction{
		{Action: "queue:select", Target: fmt.Sprintf("%s#%d", item.Repo, item.Issue), Detail: item.SelectionReason},
		{Action: "ticket_start:delegate", Target: fmt.Sprintf("%s#%d", item.Repo, item.Issue), Detail: "use ticket start branch and status policy"},
	}
	if report.StartResult != nil && strings.TrimSpace(report.StartResult.Branch) != "" {
		actions = append(actions, ApprovalPlannedAction{Action: "branch:create_or_reuse", Target: report.StartResult.Branch, Detail: "planned by ticket start"})
	}
	return &ApprovalEvidence{
		SchemaVersion:         ApprovalPlanSchemaVersion,
		Capability:            AdapterCapabilityApplyMutation,
		CanonicalCommand:      "gira queue take",
		DryRunCommand:         queueTakeCommandForStart(queueItem, report.Handoff.Role, report.Handoff.Profile, "--dry-run", report.StartResult),
		ApplyCommand:          queueTakeCommandForStart(queueItem, report.Handoff.Role, report.Handoff.Profile, "--apply", report.StartResult),
		Repo:                  item.Repo,
		Issue:                 item.Issue,
		OutputSchema:          QueueTakeSchemaVersion,
		PlannedActions:        actions,
		Blockers:              []string{},
		Warnings:              append([]string(nil), report.Warnings...),
		PostApplyVerification: fmt.Sprintf("gira ticket status %d --repo %s --json", item.Issue, item.Repo),
	}
}

func queueTakeCommandForStart(item WorkspaceQueueItem, role string, profile string, mode string, start *WorkStartResult) string {
	command := QueueTakeCommand(item, role, profile, "")
	if start != nil && start.BranchStrategy == "create" {
		command += " --create"
	}
	if strings.TrimSpace(mode) != "" {
		command += " " + strings.TrimSpace(mode)
	}
	return command
}

func QueueNextSelectionFromItem(item WorkspaceQueueItem, role string, profile string) QueueNextSelection {
	return QueueNextSelection{
		WorkspaceQueueItem: item,
		SelectionReason:    queueSelectionReason(item),
		HandoffCommand:     QueueHandoffCommand(item, role, profile),
		RunCommand:         QueueRunCommand(item, role, profile),
	}
}

func FindWorkspaceQueueItem(queues WorkspaceQueuesReport, repo string, issue int) (WorkspaceQueueItem, bool) {
	for _, queue := range WorkspaceQueueOrder() {
		for _, item := range workspaceQueueItemsByName(queues.Queues, queue) {
			if strings.EqualFold(item.Repo, repo) && item.Issue == issue {
				return item, true
			}
		}
	}
	return WorkspaceQueueItem{}, false
}

func WorkspaceQueueItemHandoffSafe(item WorkspaceQueueItem) bool {
	return item.Queue == "agent_ready"
}

func QueueHandoffStopReasonsForItem(item WorkspaceQueueItem) []string {
	reasons := []string{"queue_not_handoff_safe"}
	if item.Queue != "" {
		reasons = append(reasons, "queue_"+workspaceQueueReasonToken(item.Queue))
	}
	for _, reason := range item.ReasonCodes {
		if reason != "" {
			reasons = append(reasons, "reason_"+workspaceQueueReasonToken(reason))
		}
	}
	return reasons
}

func QueueWorkerHandoffReady(handoff TicketHandoffReport) bool {
	return handoff.Readiness.Readiness == "ready"
}

func QueueHandoffStopReasonsForWorkerHandoff(handoff TicketHandoffReport) []string {
	reasons := []string{"worker_handoff_not_ready"}
	if strings.TrimSpace(handoff.Readiness.Readiness) != "" {
		reasons = append(reasons, "readiness_"+workspaceQueueReasonToken(handoff.Readiness.Readiness))
	}
	if strings.TrimSpace(handoff.NextAction) != "" {
		reasons = append(reasons, "next_"+workspaceQueueReasonToken(handoff.NextAction))
	}
	for _, finding := range handoff.Readiness.Findings {
		if finding.Kind != "" && finding.Severity != "info" {
			reasons = append(reasons, "finding_"+workspaceQueueReasonToken(finding.Kind))
		}
	}
	return uniqueWorkspaceQueueReasons(reasons)
}

func WorkspaceQueueOrder() []string {
	return []string{"agent_ready", "review_needed", "finish_ready", "blocked", "failed_check", "human_decision"}
}

func NormalizeWorkspaceQueueNames(values []string) ([]string, error) {
	if len(values) == 0 {
		return WorkspaceQueueOrder(), nil
	}
	seen := map[string]bool{}
	out := []string{}
	for _, raw := range values {
		for _, value := range strings.Split(raw, ",") {
			queue, err := NormalizeWorkspaceQueueName(value)
			if err != nil {
				return nil, err
			}
			if seen[queue] {
				continue
			}
			seen[queue] = true
			out = append(out, queue)
		}
	}
	if len(out) == 0 {
		return WorkspaceQueueOrder(), nil
	}
	return out, nil
}

func NormalizeWorkspaceQueueName(value string) (string, error) {
	token := strings.ToLower(strings.TrimSpace(value))
	token = strings.NewReplacer("-", "_", " ", "_").Replace(token)
	switch token {
	case "ready", "agent", "agent_ready":
		return "agent_ready", nil
	case "review", "review_needed":
		return "review_needed", nil
	case "finish", "finish_ready":
		return "finish_ready", nil
	case "blocked":
		return "blocked", nil
	case "failed", "failed_check", "check_failed", "checks_failed":
		return "failed_check", nil
	case "human", "decision", "human_decision":
		return "human_decision", nil
	default:
		return "", fmt.Errorf("unknown queue %q; use ready, review, finish, blocked, failed, or human", value)
	}
}

func ShortWorkspaceQueueName(queue string) string {
	switch queue {
	case "agent_ready":
		return "ready"
	case "review_needed":
		return "review"
	case "finish_ready":
		return "finish"
	case "failed_check":
		return "failed"
	case "human_decision":
		return "human"
	default:
		return queue
	}
}

func QueueHandoffCommand(item WorkspaceQueueItem, role string, profile string) string {
	role, _ = NormalizeQueueRole(role)
	profile, _ = NormalizeQueueProfile(profile)
	command := fmt.Sprintf("gira ticket handoff --repo %s --ticket %d %s", QuoteShellArg(item.Repo), item.Issue, QuoteShellArg(role))
	if profile != AgentPromptProfileDefault {
		command += " --profile " + QuoteShellArg(profile)
	}
	return command + " --json"
}

func QueueRunCommand(item WorkspaceQueueItem, role string, profile string) string {
	role, _ = NormalizeQueueRole(role)
	profile, _ = NormalizeQueueProfile(profile)
	command := fmt.Sprintf("gira run start %d --repo %s --role %s", item.Issue, QuoteShellArg(item.Repo), QuoteShellArg(role))
	if profile != AgentPromptProfileDefault {
		command += " --profile " + QuoteShellArg(profile)
	}
	return command + " --dry-run"
}

func QueueTakeCommand(item WorkspaceQueueItem, role string, profile string, mode string) string {
	role, _ = NormalizeQueueRole(role)
	profile, _ = NormalizeQueueProfile(profile)
	command := fmt.Sprintf("gira queue take --repo %s --ticket %d", QuoteShellArg(item.Repo), item.Issue)
	if role != AgentPromptRoleImplementer {
		command += " --role " + QuoteShellArg(role)
	}
	if profile != AgentPromptProfileDefault {
		command += " --profile " + QuoteShellArg(profile)
	}
	if strings.TrimSpace(mode) != "" {
		command += " " + strings.TrimSpace(mode)
	}
	return command
}

func FormatQueueList(report QueueListReport, compact bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "queue list: %s items=%d source=%s\n", workspaceQueueLabel(report.Workspace), len(report.Items), report.SourceContract)
	for _, warning := range report.Warnings {
		fmt.Fprintf(&b, "warning: %s\n", warning)
	}
	if len(report.Items) == 0 {
		fmt.Fprintf(&b, "next step: %s\n", queueListNextStep(report.ConfigPath, report.Filters.Repos))
		return b.String()
	}
	for _, item := range report.Items {
		queue := ShortWorkspaceQueueName(item.Queue)
		if compact {
			fmt.Fprintf(&b, "%-7s %s#%d %s\n", queue, item.Repo, item.Issue, item.Title)
			continue
		}
		status := strings.TrimSpace(item.Status)
		if status == "" {
			status = "unknown"
		}
		fmt.Fprintf(&b, "  %-7s %s#%d %-12s %s\n", queue, item.Repo, item.Issue, status, item.Title)
		if len(item.ReasonCodes) > 0 {
			fmt.Fprintf(&b, "          reasons: %s\n", strings.Join(item.ReasonCodes, ","))
		}
		if strings.TrimSpace(item.NextSafeCommand) != "" {
			fmt.Fprintf(&b, "          next: %s\n", item.NextSafeCommand)
		}
	}
	return b.String()
}

func FormatQueueNext(report QueueNextReport, compact bool) string {
	var b strings.Builder
	if report.Selected == nil {
		fmt.Fprintf(&b, "queue next: stop=%s next=%s\n", strings.Join(report.StopReasons, ","), report.NextAction)
		fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
		return b.String()
	}
	item := report.Selected
	if compact {
		fmt.Fprintf(&b, "queue next: %s#%d %s\n", item.Repo, item.Issue, item.Title)
		fmt.Fprintf(&b, "handoff: %s\n", item.HandoffCommand)
		return b.String()
	}
	fmt.Fprintf(&b, "queue next: selected %s#%d reason=%s\n", item.Repo, item.Issue, item.SelectionReason)
	fmt.Fprintf(&b, "queue: %s\n", ShortWorkspaceQueueName(item.Queue))
	fmt.Fprintf(&b, "title: %s\n", item.Title)
	fmt.Fprintf(&b, "next safe command: %s\n", item.NextSafeCommand)
	fmt.Fprintf(&b, "handoff command: %s\n", item.HandoffCommand)
	fmt.Fprintf(&b, "run command: %s\n", item.RunCommand)
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}

func FormatQueueHandoff(report QueueHandoffReport, compact bool) string {
	var b strings.Builder
	if report.Selected == nil || len(report.StopReasons) > 0 {
		fmt.Fprintf(&b, "queue handoff: stop=%s next=%s\n", strings.Join(report.StopReasons, ","), report.NextAction)
		if report.Selected != nil {
			fmt.Fprintf(&b, "ticket: %s#%d %s\n", report.Selected.Repo, report.Selected.Issue, report.Selected.Title)
		}
		if report.WorkerHandoff != nil && report.WorkerHandoff.SchemaVersion != "" {
			fmt.Fprintf(&b, "worker handoff: schema=%s readiness=%s next=%s\n", report.WorkerHandoff.SchemaVersion, report.WorkerHandoff.Readiness.Readiness, report.WorkerHandoff.NextAction)
		}
		fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
		return b.String()
	}
	item := report.Selected
	if compact {
		fmt.Fprintf(&b, "queue handoff: %s#%d role=%s\n", item.Repo, item.Issue, report.Role)
		fmt.Fprintf(&b, "run: %s\n", item.RunCommand)
		return b.String()
	}
	fmt.Fprintf(&b, "queue handoff: selected %s#%d role=%s profile=%s\n", item.Repo, item.Issue, report.Role, report.Profile)
	fmt.Fprintf(&b, "queue: %s\n", ShortWorkspaceQueueName(item.Queue))
	fmt.Fprintf(&b, "title: %s\n", item.Title)
	fmt.Fprintf(&b, "worker handoff: schema=%s next=%s\n", report.WorkerHandoff.SchemaVersion, report.WorkerHandoff.NextAction)
	fmt.Fprintf(&b, "handoff command: %s\n", item.HandoffCommand)
	fmt.Fprintf(&b, "run command: %s\n", item.RunCommand)
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}

func FormatQueueTake(report QueueTakeReport, compact bool) string {
	var b strings.Builder
	if report.Handoff.Selected == nil || len(report.StopReasons) > 0 {
		fmt.Fprintf(&b, "queue take: stop=%s next=%s\n", strings.Join(report.StopReasons, ","), report.NextAction)
		if report.Handoff.Selected != nil {
			fmt.Fprintf(&b, "ticket: %s#%d %s\n", report.Handoff.Selected.Repo, report.Handoff.Selected.Issue, report.Handoff.Selected.Title)
		}
		if report.Handoff.WorkerHandoff != nil && report.Handoff.WorkerHandoff.SchemaVersion != "" {
			fmt.Fprintf(&b, "worker handoff: schema=%s readiness=%s next=%s\n", report.Handoff.WorkerHandoff.SchemaVersion, report.Handoff.WorkerHandoff.Readiness.Readiness, report.Handoff.WorkerHandoff.NextAction)
		}
		fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
		return b.String()
	}
	item := report.Handoff.Selected
	mode := "apply"
	if report.DryRun {
		mode = "dry-run"
	}
	branch := ""
	if report.StartResult != nil {
		branch = report.StartResult.Branch
	}
	if compact {
		fmt.Fprintf(&b, "queue take: %s %s#%d branch=%s\n", mode, item.Repo, item.Issue, branch)
		fmt.Fprintf(&b, "next: %s\n", report.NextStep)
		return b.String()
	}
	fmt.Fprintf(&b, "queue take: %s %s#%d role=%s profile=%s\n", mode, item.Repo, item.Issue, report.Handoff.Role, report.Handoff.Profile)
	fmt.Fprintf(&b, "queue: %s\n", ShortWorkspaceQueueName(item.Queue))
	fmt.Fprintf(&b, "title: %s\n", item.Title)
	fmt.Fprintf(&b, "handoff command: %s\n", report.Handoff.HandoffCommand)
	if report.StartResult != nil {
		fmt.Fprintf(&b, "ticket start: branch=%s status=%s next=%s\n", report.StartResult.Branch, report.StartResult.NextStatus, report.StartResult.NextStep)
	}
	fmt.Fprintf(&b, "run command: %s\n", report.Handoff.RunCommand)
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}

func emptyWorkspaceQueues() WorkspaceQueues {
	return WorkspaceQueues{
		AgentReady:    []WorkspaceQueueItem{},
		ReviewNeeded:  []WorkspaceQueueItem{},
		FinishReady:   []WorkspaceQueueItem{},
		Blocked:       []WorkspaceQueueItem{},
		FailedCheck:   []WorkspaceQueueItem{},
		HumanDecision: []WorkspaceQueueItem{},
	}
}

func workspaceQueueItemsByName(queues WorkspaceQueues, queue string) []WorkspaceQueueItem {
	switch queue {
	case "agent_ready":
		return queues.AgentReady
	case "review_needed":
		return queues.ReviewNeeded
	case "finish_ready":
		return queues.FinishReady
	case "blocked":
		return queues.Blocked
	case "failed_check":
		return queues.FailedCheck
	case "human_decision":
		return queues.HumanDecision
	default:
		return nil
	}
}

func addWorkspaceQueueItem(queues *WorkspaceQueues, item WorkspaceQueueItem) {
	switch item.Queue {
	case "agent_ready":
		queues.AgentReady = append(queues.AgentReady, item)
	case "review_needed":
		queues.ReviewNeeded = append(queues.ReviewNeeded, item)
	case "finish_ready":
		queues.FinishReady = append(queues.FinishReady, item)
	case "blocked":
		queues.Blocked = append(queues.Blocked, item)
	case "failed_check":
		queues.FailedCheck = append(queues.FailedCheck, item)
	case "human_decision":
		queues.HumanDecision = append(queues.HumanDecision, item)
	}
}

func countWorkspaceQueues(queues WorkspaceQueues) WorkspaceQueueCounts {
	return WorkspaceQueueCounts{
		AgentReady:    len(queues.AgentReady),
		ReviewNeeded:  len(queues.ReviewNeeded),
		FinishReady:   len(queues.FinishReady),
		Blocked:       len(queues.Blocked),
		FailedCheck:   len(queues.FailedCheck),
		HumanDecision: len(queues.HumanDecision),
	}
}

func NormalizeQueueRole(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = AgentPromptRoleImplementer
	}
	switch value {
	case AgentPromptRolePlanner, AgentPromptRoleImplementer, AgentPromptRoleReviewer:
		return value, nil
	default:
		return "", fmt.Errorf("--role must be one of planner, implementer, reviewer")
	}
}

func NormalizeQueueProfile(value string) (string, error) {
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

func queueSelectionReason(item WorkspaceQueueItem) string {
	if len(item.ReasonCodes) == 0 {
		return "first_agent_ready"
	}
	return "first_agent_ready:" + strings.Join(item.ReasonCodes, ",")
}

func queueNextStopReasons(counts WorkspaceQueueCounts) []string {
	reasons := []string{"no_agent_ready_item"}
	if counts.ReviewNeeded+counts.FinishReady+counts.Blocked+counts.FailedCheck+counts.HumanDecision == 0 {
		return append(reasons, "no_queue_items")
	}
	if counts.ReviewNeeded > 0 {
		reasons = append(reasons, "review_needed_present")
	}
	if counts.FinishReady > 0 {
		reasons = append(reasons, "finish_ready_present")
	}
	if counts.Blocked > 0 {
		reasons = append(reasons, "blocked_present")
	}
	if counts.FailedCheck > 0 {
		reasons = append(reasons, "failed_check_present")
	}
	if counts.HumanDecision > 0 {
		reasons = append(reasons, "human_decision_present")
	}
	return reasons
}

func queueListNextStep(configPath string, repos []string) string {
	command := "gira queue list"
	if strings.TrimSpace(configPath) != "" {
		command += " --config " + QuoteShellArg(configPath)
	}
	for _, repo := range repos {
		if strings.TrimSpace(repo) != "" {
			command += " --repo " + QuoteShellArg(repo)
		}
	}
	return command
}

func normalizedQueueRepoFilters(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[strings.ToLower(value)] = true
		}
	}
	return out
}

func workspaceQueueLabel(workspace WorkspaceSummary) string {
	if strings.TrimSpace(workspace.Name) == "" && strings.TrimSpace(workspace.Owner) == "" {
		return "workspace"
	}
	if strings.TrimSpace(workspace.Owner) == "" {
		return workspace.Name
	}
	if strings.TrimSpace(workspace.Name) == "" {
		return workspace.Owner
	}
	return fmt.Sprintf("%s (%s)", workspace.Name, workspace.Owner)
}
