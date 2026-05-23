package gira

import (
	"fmt"
	"strings"
)

const ApprovalPlanSchemaVersion = "gira-approval-plan/v1"

type ApprovalEvidence struct {
	SchemaVersion         string                  `json:"schema_version"`
	Capability            AdapterCapabilityClass  `json:"capability"`
	CanonicalCommand      string                  `json:"canonical_command"`
	DryRunCommand         string                  `json:"dry_run_command"`
	ApplyCommand          string                  `json:"apply_command"`
	Repo                  string                  `json:"repo,omitempty"`
	Issue                 int                     `json:"issue,omitempty"`
	OutputSchema          string                  `json:"output_schema"`
	PlannedActions        []ApprovalPlannedAction `json:"planned_actions"`
	Blockers              []string                `json:"blockers"`
	Warnings              []string                `json:"warnings"`
	PostApplyVerification string                  `json:"post_apply_verification"`
}

type ApprovalPlannedAction struct {
	Action string `json:"action"`
	Target string `json:"target,omitempty"`
	Detail string `json:"detail,omitempty"`
}

func WorkStartApprovalEvidence(result WorkStartResult, canonicalCommand string) *ApprovalEvidence {
	canonicalCommand = strings.TrimSpace(canonicalCommand)
	if canonicalCommand == "" {
		canonicalCommand = "gira work start"
	}
	applyCommand := workStartApprovalCommand(result, canonicalCommand, "--apply")
	dryRunCommand := strings.Replace(applyCommand, " --apply", " --dry-run", 1)
	return &ApprovalEvidence{
		SchemaVersion:         ApprovalPlanSchemaVersion,
		Capability:            AdapterCapabilityApplyMutation,
		CanonicalCommand:      canonicalCommand,
		DryRunCommand:         dryRunCommand,
		ApplyCommand:          applyCommand,
		Repo:                  result.Repo,
		Issue:                 result.Issue,
		OutputSchema:          WorkStartResultSchemaVersion,
		PlannedActions:        workStartApprovalActions(result),
		Blockers:              []string{},
		Warnings:              []string{},
		PostApplyVerification: fmt.Sprintf("gira ticket status %d --repo %s --json", result.Issue, result.Repo),
	}
}

func workStartApprovalCommand(result WorkStartResult, canonicalCommand string, mode string) string {
	args := []string{canonicalCommand}
	if strings.Contains(canonicalCommand, "ticket") {
		args = append(args, fmt.Sprintf("%d", result.Issue), "--repo", result.Repo)
	} else {
		args = append(args, "--repo", result.Repo, "--issue", fmt.Sprintf("%d", result.Issue))
	}
	if result.BaseSource == "explicit --base" && result.BaseBranch != "" {
		args = append(args, "--base", result.BaseBranch)
	}
	args = append(args, mode)
	return strings.Join(args, " ")
}

func workStartApprovalActions(result WorkStartResult) []ApprovalPlannedAction {
	actions := []ApprovalPlannedAction{
		{Action: "branch:create_or_reuse", Target: result.Branch, Detail: "create or reuse ticket work branch"},
	}
	if result.BaseBranch != "" {
		actions = append(actions, ApprovalPlannedAction{Action: "branch_policy:record", Target: result.BaseBranch, Detail: "record lifecycle base branch evidence"})
	}
	if !strings.EqualFold(result.Status, "In progress") {
		actions = append(actions, ApprovalPlannedAction{Action: "issue_status:update", Target: fmt.Sprintf("#%d", result.Issue), Detail: "move ticket to status:in-progress"})
	}
	return actions
}

func WorkPRApprovalEvidence(result WorkPRResult, canonicalCommand string) *ApprovalEvidence {
	canonicalCommand = strings.TrimSpace(canonicalCommand)
	if canonicalCommand == "" {
		canonicalCommand = "gira work pr"
	}
	applyCommand := workPRApprovalCommand(result, canonicalCommand, "--apply")
	dryRunCommand := strings.Replace(applyCommand, " --apply", " --dry-run", 1)
	return &ApprovalEvidence{
		SchemaVersion:         ApprovalPlanSchemaVersion,
		Capability:            AdapterCapabilityApplyMutation,
		CanonicalCommand:      canonicalCommand,
		DryRunCommand:         dryRunCommand,
		ApplyCommand:          applyCommand,
		Repo:                  result.Repo,
		Issue:                 result.Issue,
		OutputSchema:          WorkPRResultSchemaVersion,
		PlannedActions:        workPRApprovalActions(result),
		Blockers:              stableStringSlice(result.Blockers),
		Warnings:              []string{},
		PostApplyVerification: fmt.Sprintf("gira ticket status %d --repo %s --json", result.Issue, result.Repo),
	}
}

func workPRApprovalCommand(result WorkPRResult, canonicalCommand string, mode string) string {
	args := []string{canonicalCommand}
	if strings.Contains(canonicalCommand, "ticket") {
		args = append(args, fmt.Sprintf("%d", result.Issue), "--repo", result.Repo)
	} else {
		args = append(args, "--repo", result.Repo, "--issue", fmt.Sprintf("%d", result.Issue))
	}
	if result.Draft {
		args = append(args, "--draft")
	}
	args = append(args, mode)
	return strings.Join(args, " ")
}

func workPRApprovalActions(result WorkPRResult) []ApprovalPlannedAction {
	actions := []ApprovalPlannedAction{}
	if result.BranchPush == "planned" || result.BranchPush == "applied" {
		actions = append(actions, ApprovalPlannedAction{Action: "branch:push", Target: result.Branch, Detail: result.LocalGit})
	}
	if result.PRNumber > 0 {
		actions = append(actions, ApprovalPlannedAction{Action: "pr:reuse", Target: fmt.Sprintf("#%d", result.PRNumber), Detail: "validate existing linked PR"})
	} else {
		actions = append(actions, ApprovalPlannedAction{Action: "pr:create", Target: result.Branch, Detail: result.ClosingBody})
	}
	if !strings.EqualFold(result.Status, result.NextStatus) && strings.TrimSpace(result.NextStatus) != "" {
		actions = append(actions, ApprovalPlannedAction{Action: "issue_status:update", Target: fmt.Sprintf("#%d", result.Issue), Detail: "move ticket to " + result.NextStatus})
	}
	if result.RecordedBase != "" {
		actions = append(actions, ApprovalPlannedAction{Action: "branch_policy:verify", Target: result.RecordedBase, Detail: "validate PR base against recorded lifecycle base"})
	}
	return actions
}

func TicketNoteApprovalEvidence(report TicketNoteReport) *ApprovalEvidence {
	applyCommand := ticketNoteApprovalCommand(report, "--apply")
	dryRunCommand := strings.Replace(applyCommand, " --apply", " --dry-run", 1)
	return &ApprovalEvidence{
		SchemaVersion:         ApprovalPlanSchemaVersion,
		Capability:            AdapterCapabilityApplyMutation,
		CanonicalCommand:      "gira ticket note",
		DryRunCommand:         dryRunCommand,
		ApplyCommand:          applyCommand,
		Repo:                  report.Repo,
		Issue:                 report.Ticket,
		OutputSchema:          TicketNoteReportSchemaVersion,
		PlannedActions:        ticketNoteApprovalActions(report),
		Blockers:              []string{},
		Warnings:              []string{},
		PostApplyVerification: fmt.Sprintf("gira ticket view %d --repo %s --json", report.Ticket, report.Repo),
	}
}

func ticketNoteApprovalCommand(report TicketNoteReport, mode string) string {
	args := []string{
		"gira ticket note",
		fmt.Sprintf("%d", report.Ticket),
		"--repo", report.Repo,
		"--kind", report.Kind,
		"--target", report.Target,
	}
	if strings.TrimSpace(report.Body) != "" {
		args = append(args, "--body", shellQuoteArg(report.Body))
	}
	args = append(args, mode)
	return strings.Join(args, " ")
}

func ticketNoteApprovalActions(report TicketNoteReport) []ApprovalPlannedAction {
	actions := make([]ApprovalPlannedAction, 0, len(report.Targets))
	for _, target := range report.Targets {
		action := target.Type + ":comment"
		actions = append(actions, ApprovalPlannedAction{
			Action: action,
			Target: fmt.Sprintf("#%d", target.Number),
			Detail: "post " + report.Kind + " note",
		})
	}
	return actions
}

func shellQuoteArg(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, " \t\n'\"`$\\") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func WorkFinishApprovalEvidence(result WorkFinishResult) *ApprovalEvidence {
	applyCommand := workFinishApprovalCommand(result, "--apply")
	dryRunCommand := strings.Replace(applyCommand, " --apply", " --dry-run", 1)
	return &ApprovalEvidence{
		SchemaVersion:         ApprovalPlanSchemaVersion,
		Capability:            AdapterCapabilityApplyMutation,
		CanonicalCommand:      "gira ticket finish",
		DryRunCommand:         dryRunCommand,
		ApplyCommand:          applyCommand,
		Repo:                  result.Repo,
		Issue:                 result.Issue,
		OutputSchema:          WorkFinishResultSchemaVersion,
		PlannedActions:        workFinishApprovalActions(result),
		Blockers:              stableStringSlice(result.Blockers),
		Warnings:              workFinishApprovalWarnings(result),
		PostApplyVerification: fmt.Sprintf("gira ticket status %d --repo %s --json", result.Issue, result.Repo),
	}
}

func workFinishApprovalCommand(result WorkFinishResult, mode string) string {
	args := []string{
		"gira ticket finish",
		fmt.Sprintf("%d", result.Issue),
		"--repo", result.Repo,
	}
	if strings.TrimSpace(result.Wait) != "" && result.Wait != "0s" {
		args = append(args, "--wait", result.Wait)
	}
	if result.SyncLocal {
		args = append(args, "--sync-local")
	}
	args = append(args, mode)
	return strings.Join(args, " ")
}

func workFinishApprovalActions(result WorkFinishResult) []ApprovalPlannedAction {
	actions := make([]ApprovalPlannedAction, 0, len(result.Actions))
	for _, action := range result.Actions {
		if strings.TrimSpace(action.Action) == "" {
			continue
		}
		target := ""
		switch {
		case strings.HasPrefix(action.Action, "pr:") && result.PRNumber > 0:
			target = fmt.Sprintf("#%d", result.PRNumber)
		case strings.HasPrefix(action.Action, "ticket:") || strings.HasPrefix(action.Action, "finish:"):
			target = fmt.Sprintf("#%d", result.Issue)
		case strings.HasPrefix(action.Action, "jira:") && result.JiraKey != "":
			target = result.JiraKey
		case strings.HasPrefix(action.Action, "local:") && result.LocalSync.TargetBranch != "":
			target = result.LocalSync.TargetBranch
		}
		detail := strings.TrimSpace(action.Detail)
		if action.Status != "" {
			if detail == "" {
				detail = action.Status
			} else {
				detail = action.Status + ": " + detail
			}
		}
		actions = append(actions, ApprovalPlannedAction{Action: action.Action, Target: target, Detail: detail})
	}
	return actions
}

func workFinishApprovalWarnings(result WorkFinishResult) []string {
	warnings := []string{}
	warnings = appendUniqueStrings(warnings, result.Readiness.Warnings...)
	warnings = appendUniqueStrings(warnings, result.Receipt.Warnings...)
	return warnings
}

func stableStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values...)
}

func TicketSupersedeApprovalEvidence(report TicketSupersedeReport) *ApprovalEvidence {
	applyCommand := ticketSupersedeApprovalCommand(report, "--apply")
	dryRunCommand := strings.Replace(applyCommand, " --apply", " --dry-run", 1)
	return &ApprovalEvidence{
		SchemaVersion:         ApprovalPlanSchemaVersion,
		Capability:            AdapterCapabilityApplyMutation,
		CanonicalCommand:      "gira ticket supersede",
		DryRunCommand:         dryRunCommand,
		ApplyCommand:          applyCommand,
		Repo:                  report.Repo,
		Issue:                 report.Original.Number,
		OutputSchema:          TicketSupersedeReportSchemaVersion,
		PlannedActions:        ticketSupersedeApprovalActions(report),
		Blockers:              []string{},
		Warnings:              []string{},
		PostApplyVerification: fmt.Sprintf("gira ticket status %d --repo %s --json", report.Original.Number, report.Repo),
	}
}

func ticketSupersedeApprovalCommand(report TicketSupersedeReport, mode string) string {
	args := []string{
		"gira ticket supersede",
		fmt.Sprintf("%d", report.Original.Number),
		"--repo", report.Repo,
		"--replacement-title", shellQuoteArg(report.Replacement.Title),
	}
	if strings.TrimSpace(report.Body) != "" {
		args = append(args, "--body", shellQuoteArg(report.Body))
	}
	for _, label := range report.Labels {
		args = append(args, "--label", shellQuoteArg(label))
	}
	if strings.TrimSpace(report.Milestone) != "" {
		args = append(args, "--milestone", shellQuoteArg(report.Milestone))
	}
	if report.DraftPR.Action == "close" {
		args = append(args, "--close-draft-pr")
	}
	args = append(args, mode)
	return strings.Join(args, " ")
}

func ticketSupersedeApprovalActions(report TicketSupersedeReport) []ApprovalPlannedAction {
	actions := make([]ApprovalPlannedAction, 0, len(report.Actions))
	for _, action := range report.Actions {
		if strings.TrimSpace(action.Action) == "" {
			continue
		}
		target := ""
		switch {
		case strings.HasPrefix(action.Action, "original:"):
			target = fmt.Sprintf("#%d", report.Original.Number)
		case strings.HasPrefix(action.Action, "replacement:"):
			if report.Replacement.Number > 0 {
				target = fmt.Sprintf("#%d", report.Replacement.Number)
			} else {
				target = report.Replacement.Title
			}
		case strings.HasPrefix(action.Action, "draft_pr:") && report.DraftPR.Number > 0:
			target = fmt.Sprintf("#%d", report.DraftPR.Number)
		}
		detail := strings.TrimSpace(action.Detail)
		if action.Status != "" {
			if detail == "" {
				detail = action.Status
			} else {
				detail = action.Status + ": " + detail
			}
		}
		actions = append(actions, ApprovalPlannedAction{Action: action.Action, Target: target, Detail: detail})
	}
	return actions
}

func TicketNewApprovalEvidence(report TicketNewReport) *ApprovalEvidence {
	applyCommand := ticketNewApprovalCommand(report, "--apply")
	dryRunCommand := strings.Replace(applyCommand, " --apply", " --dry-run", 1)
	return &ApprovalEvidence{
		SchemaVersion:         ApprovalPlanSchemaVersion,
		Capability:            AdapterCapabilityApplyMutation,
		CanonicalCommand:      "gira ticket new",
		DryRunCommand:         dryRunCommand,
		ApplyCommand:          applyCommand,
		Repo:                  report.Repo,
		OutputSchema:          TicketNewReportSchemaVersion,
		PlannedActions:        ticketNewApprovalActions(report),
		Blockers:              []string{},
		Warnings:              ticketNewApprovalWarnings(report),
		PostApplyVerification: "gira ticket status <created-ticket> --repo " + report.Repo + " --json",
	}
}

func ticketNewApprovalCommand(report TicketNewReport, mode string) string {
	args := []string{
		"gira ticket new",
		shellQuoteArg(report.Title),
		"--repo", report.Repo,
		"--body", shellQuoteArg(report.Body),
	}
	if strings.TrimSpace(report.Type) != "" && report.Type != "task" {
		args = append(args, "--type", report.Type)
	}
	if strings.TrimSpace(report.Priority) != "" {
		args = append(args, "--priority", report.Priority)
	}
	for _, label := range ticketNewApprovalExtraLabels(report) {
		args = append(args, "--label", shellQuoteArg(label))
	}
	if strings.TrimSpace(report.Milestone) != "" {
		args = append(args, "--milestone", shellQuoteArg(report.Milestone))
	}
	if report.Start {
		args = append(args, "--start")
	}
	args = append(args, mode)
	return strings.Join(args, " ")
}

func ticketNewApprovalActions(report TicketNewReport) []ApprovalPlannedAction {
	actions := []ApprovalPlannedAction{
		{Action: "issue:create", Target: report.Repo, Detail: report.Title},
	}
	if report.Start {
		actions = append(actions, ApprovalPlannedAction{Action: "ticket:start", Target: "<created-ticket>", Detail: "start created ticket after issue creation"})
	}
	return actions
}

func ticketNewApprovalWarnings(report TicketNewReport) []string {
	if report.TicketReadiness.Readiness == "ready" {
		return []string{}
	}
	warnings := []string{}
	for _, finding := range report.TicketReadiness.Findings {
		if strings.TrimSpace(finding.Kind) != "" {
			warnings = appendUniqueStrings(warnings, finding.Kind)
		}
	}
	return stableStringSlice(warnings)
}

func ticketNewApprovalExtraLabels(report TicketNewReport) []string {
	labels := []string{}
	for _, label := range report.Labels {
		trimmed := strings.TrimSpace(label)
		lower := strings.ToLower(trimmed)
		if trimmed == "" || lower == "status:ready" || strings.HasPrefix(lower, "type:") || strings.HasPrefix(lower, "priority:") {
			continue
		}
		labels = append(labels, trimmed)
	}
	return labels
}
