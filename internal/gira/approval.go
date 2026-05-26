package gira

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const ApprovalPlanSchemaVersion = "gira-approval-plan/v1"

type ApprovalEvidence struct {
	SchemaVersion         string                  `json:"schema_version"`
	Capability            AdapterCapabilityClass  `json:"capability"`
	CanonicalCommand      string                  `json:"canonical_command"`
	DryRunCommand         string                  `json:"dry_run_command"`
	ApplyCommand          string                  `json:"apply_command"`
	DryRunArgv            []string                `json:"dry_run_argv,omitempty"`
	ApplyArgv             []string                `json:"apply_argv,omitempty"`
	Repo                  string                  `json:"repo,omitempty"`
	Issue                 int                     `json:"issue,omitempty"`
	OutputSchema          string                  `json:"output_schema"`
	PlannedActions        []ApprovalPlannedAction `json:"planned_actions"`
	Blockers              []string                `json:"blockers"`
	Warnings              []string                `json:"warnings"`
	PostApplyVerification string                  `json:"post_apply_verification"`
}

func (e ApprovalEvidence) MarshalJSON() ([]byte, error) {
	type approvalEvidenceJSON ApprovalEvidence
	out := approvalEvidenceJSON(e)
	if len(out.DryRunArgv) == 0 && strings.TrimSpace(out.DryRunCommand) != "" {
		out.DryRunArgv = parseApprovalCommandLine(out.DryRunCommand)
	}
	if len(out.ApplyArgv) == 0 && strings.TrimSpace(out.ApplyCommand) != "" {
		out.ApplyArgv = parseApprovalCommandLine(out.ApplyCommand)
	}
	return json.Marshal(out)
}

func parseApprovalCommandLine(command string) []string {
	args := []string{}
	var current strings.Builder
	inToken := false
	inSingle := false
	inDouble := false
	escaped := false

	flush := func() {
		if !inToken {
			return
		}
		args = append(args, current.String())
		current.Reset()
		inToken = false
	}

	for _, r := range command {
		switch {
		case escaped:
			current.WriteRune(r)
			inToken = true
			escaped = false
		case inSingle:
			if r == '\'' {
				inSingle = false
			} else {
				current.WriteRune(r)
				inToken = true
			}
		case inDouble:
			switch r {
			case '"':
				inDouble = false
			case '\\':
				escaped = true
				inToken = true
			default:
				current.WriteRune(r)
				inToken = true
			}
		case r == '\'':
			inSingle = true
			inToken = true
		case r == '"':
			inDouble = true
			inToken = true
		case r == '\\':
			escaped = true
			inToken = true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			flush()
		default:
			current.WriteRune(r)
			inToken = true
		}
	}
	if escaped {
		current.WriteRune('\\')
		inToken = true
	}
	flush()
	return args
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
	dryRunCommand := workStartApprovalCommand(result, canonicalCommand, "--dry-run")
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
		args = append(args, "--base", QuoteShellArg(result.BaseBranch))
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
	dryRunCommand := workPRApprovalCommand(result, canonicalCommand, "--dry-run")
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
	dryRunCommand := ticketNoteApprovalCommand(report, "--dry-run")
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
		args = append(args, "--body", QuoteShellArg(report.Body))
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

func WorkFinishApprovalEvidence(result WorkFinishResult) *ApprovalEvidence {
	applyCommand := workFinishApprovalCommand(result, "--apply")
	dryRunCommand := workFinishApprovalCommand(result, "--dry-run")
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
	dryRunCommand := ticketSupersedeApprovalCommand(report, "--dry-run")
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
		"--replacement-title", QuoteShellArg(report.Replacement.Title),
	}
	if strings.TrimSpace(report.Body) != "" {
		args = append(args, "--body", QuoteShellArg(report.Body))
	}
	for _, label := range report.Labels {
		args = append(args, "--label", QuoteShellArg(label))
	}
	if strings.TrimSpace(report.Milestone) != "" {
		args = append(args, "--milestone", QuoteShellArg(report.Milestone))
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
	dryRunCommand := ticketNewApprovalCommand(report, "--dry-run")
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
		QuoteShellArg(report.Title),
		"--repo", report.Repo,
		"--body", QuoteShellArg(report.Body),
	}
	if strings.TrimSpace(report.Type) != "" && report.Type != "task" {
		args = append(args, "--type", report.Type)
	}
	if strings.TrimSpace(report.Priority) != "" {
		args = append(args, "--priority", report.Priority)
	}
	for _, label := range ticketNewApprovalExtraLabels(report) {
		args = append(args, "--label", QuoteShellArg(label))
	}
	if strings.TrimSpace(report.Milestone) != "" {
		args = append(args, "--milestone", QuoteShellArg(report.Milestone))
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

func RepoRegisterApprovalEvidence(report RepoRegisterReport) *ApprovalEvidence {
	applyCommand := repoRegisterApprovalCommand(report, "--apply")
	dryRunCommand := repoRegisterApprovalCommand(report, "--dry-run")
	return &ApprovalEvidence{
		SchemaVersion:         ApprovalPlanSchemaVersion,
		Capability:            AdapterCapabilityApplyMutation,
		CanonicalCommand:      "gira repo register",
		DryRunCommand:         dryRunCommand,
		ApplyCommand:          applyCommand,
		Repo:                  report.Repo,
		OutputSchema:          RepoRegisterReportSchemaVersion,
		PlannedActions:        repoRegisterApprovalActions(report),
		Blockers:              repoRegisterApprovalBlockers(report),
		Warnings:              []string{},
		PostApplyVerification: fmt.Sprintf("gira config repo --repo %s --config-root %s --json", QuoteShellArg(report.Repo), QuoteShellArg(report.ConfigRoot)),
	}
}

func repoRegisterApprovalCommand(report RepoRegisterReport, mode string) string {
	args := []string{
		"gira repo register",
		QuoteShellArg(report.Repo),
	}
	if strings.TrimSpace(report.Path) != "" {
		args = append(args, "--path", QuoteShellArg(report.Path))
	}
	if strings.TrimSpace(report.ConfigRoot) != "" {
		args = append(args, "--config-root", QuoteShellArg(report.ConfigRoot))
	}
	if report.Action == "overwrite" {
		args = append(args, "--overwrite")
	}
	args = append(args, mode)
	return strings.Join(args, " ")
}

func repoRegisterApprovalActions(report RepoRegisterReport) []ApprovalPlannedAction {
	action := "registry:" + strings.TrimSpace(report.Action)
	if action == "registry:" {
		action = "registry:update"
	}
	detail := "register " + report.Repo
	if strings.TrimSpace(report.Path) != "" {
		detail += " at " + report.Path
	}
	return []ApprovalPlannedAction{{
		Action: action,
		Target: report.File,
		Detail: detail,
	}}
}

func repoRegisterApprovalBlockers(report RepoRegisterReport) []string {
	if strings.EqualFold(report.Status, "blocked") || report.Action == "conflict" {
		return []string{"repo_registry_conflict"}
	}
	return []string{}
}

func RepoMigrateApprovalEvidence(report RepoMigrateReport) *ApprovalEvidence {
	applyCommand := repoMigrateApprovalCommand(report, "--apply")
	dryRunCommand := repoMigrateApprovalCommand(report, "--dry-run")
	return &ApprovalEvidence{
		SchemaVersion:         ApprovalPlanSchemaVersion,
		Capability:            AdapterCapabilityApplyMutation,
		CanonicalCommand:      "gira repo migrate",
		DryRunCommand:         dryRunCommand,
		ApplyCommand:          applyCommand,
		Repo:                  report.Repo,
		OutputSchema:          RepoMigrateReportSchemaVersion,
		PlannedActions:        repoMigrateApprovalActions(report),
		Blockers:              repoMigrateApprovalBlockers(report),
		Warnings:              stableStringSlice(report.Notes),
		PostApplyVerification: repoMigratePostApplyVerification(report),
	}
}

func repoMigrateApprovalCommand(report RepoMigrateReport, mode string) string {
	args := []string{"gira repo migrate"}
	if strings.TrimSpace(report.Repo) != "" {
		args = append(args, "--repo", QuoteShellArg(report.Repo))
	}
	if strings.TrimSpace(report.Path) != "" {
		args = append(args, "--path", QuoteShellArg(report.Path))
	}
	if strings.TrimSpace(report.ConfigRoot) != "" {
		args = append(args, "--config-root", QuoteShellArg(report.ConfigRoot))
	}
	if report.Action == "overwrite" {
		args = append(args, "--overwrite")
	}
	args = append(args, mode)
	return strings.Join(args, " ")
}

func repoMigrateApprovalActions(report RepoMigrateReport) []ApprovalPlannedAction {
	actions := []ApprovalPlannedAction{}
	if strings.TrimSpace(report.ContractFile) != "" {
		actions = append(actions, ApprovalPlannedAction{
			Action: "contract:preserve",
			Target: report.ContractFile,
			Detail: "preserve repo-local .gira/config.yaml as shared contract",
		})
	}
	action := "registry:" + strings.TrimSpace(report.Action)
	if action == "registry:" || action == "registry:plan" {
		action = "registry:update"
	}
	detail := "import repo metadata into global registry"
	if strings.TrimSpace(report.Repo) != "" {
		detail = "import " + report.Repo + " metadata into global registry"
	}
	actions = append(actions, ApprovalPlannedAction{
		Action: action,
		Target: report.File,
		Detail: detail,
	})
	return actions
}

func repoMigrateApprovalBlockers(report RepoMigrateReport) []string {
	blockers := []string{}
	if report.Register != nil && report.Register.Approval != nil {
		blockers = appendUniqueStrings(blockers, report.Register.Approval.Blockers...)
	}
	if strings.EqualFold(report.Status, "blocked") {
		blockers = appendUniqueStrings(blockers, "repo_migrate_blocked")
	}
	return stableStringSlice(blockers)
}

func repoMigratePostApplyVerification(report RepoMigrateReport) string {
	if strings.TrimSpace(report.Repo) != "" {
		return fmt.Sprintf("gira config repo --repo %s --config-root %s --json", QuoteShellArg(report.Repo), QuoteShellArg(report.ConfigRoot))
	}
	return "gira config doctor --config-root " + QuoteShellArg(report.ConfigRoot) + " --json"
}

func SetupGlobalApprovalEvidence(report SetupGlobalReport) *ApprovalEvidence {
	applyCommand := setupGlobalApprovalCommand(report, "--apply")
	dryRunCommand := setupGlobalApprovalCommand(report, "--dry-run")
	return &ApprovalEvidence{
		SchemaVersion:         ApprovalPlanSchemaVersion,
		Capability:            AdapterCapabilityApplyMutation,
		CanonicalCommand:      "gira setup global",
		DryRunCommand:         dryRunCommand,
		ApplyCommand:          applyCommand,
		Repo:                  report.Repo,
		OutputSchema:          SetupGlobalReportSchemaVersion,
		PlannedActions:        setupGlobalApprovalActions(report),
		Blockers:              setupGlobalApprovalBlockers(report),
		Warnings:              stableStringSlice(report.Notes),
		PostApplyVerification: setupGlobalPostApplyVerification(report),
	}
}

func setupGlobalApprovalCommand(report SetupGlobalReport, mode string) string {
	args := []string{"gira setup global"}
	if strings.TrimSpace(report.Repo) != "" {
		args = append(args, "--repo", QuoteShellArg(report.Repo))
	}
	if strings.TrimSpace(report.Path) != "" {
		args = append(args, "--path", QuoteShellArg(report.Path))
	}
	if strings.TrimSpace(report.ConfigRoot) != "" {
		args = append(args, "--config-root", QuoteShellArg(report.ConfigRoot))
	}
	if strings.TrimSpace(report.Workspace.Name) != "" {
		args = append(args, "--workspace", QuoteShellArg(report.Workspace.Name))
	}
	if strings.TrimSpace(report.Workspace.Owner) != "" {
		args = append(args, "--owner", QuoteShellArg(report.Workspace.Owner))
	}
	if strings.TrimSpace(report.InboxRepo) != "" {
		args = append(args, "--inbox-repo", QuoteShellArg(report.InboxRepo))
	}
	if strings.TrimSpace(report.Mode) != "" {
		args = append(args, "--mode", QuoteShellArg(report.Mode))
	}
	project := report.GlobalWorkspace.Workspace.Project
	if strings.TrimSpace(project.Owner) != "" {
		args = append(args, "--project-owner", QuoteShellArg(project.Owner))
	}
	if strings.TrimSpace(project.Title) != "" {
		args = append(args, "--project-title", QuoteShellArg(project.Title))
	}
	if project.Number > 0 {
		args = append(args, "--project-number", fmt.Sprintf("%d", project.Number))
	}
	if strings.TrimSpace(report.Defaults.Agent) != "" {
		args = append(args, "--agent", QuoteShellArg(report.Defaults.Agent))
	}
	if strings.TrimSpace(report.Defaults.Assignee) != "" {
		args = append(args, "--assignee", QuoteShellArg(report.Defaults.Assignee))
	}
	for _, label := range report.Defaults.AgentLabels {
		args = append(args, "--agent-label", QuoteShellArg(label))
	}
	if setupGlobalHasOverwrite(report) {
		args = append(args, "--overwrite")
	}
	args = append(args, mode)
	return strings.Join(args, " ")
}

func setupGlobalHasOverwrite(report SetupGlobalReport) bool {
	for _, plan := range report.Files {
		if plan.Action == "overwrite" {
			return true
		}
	}
	return false
}

func setupGlobalApprovalActions(report SetupGlobalReport) []ApprovalPlannedAction {
	actions := make([]ApprovalPlannedAction, 0, len(report.Files))
	for _, plan := range report.Files {
		action := "file:" + strings.TrimSpace(plan.Action)
		if action == "file:" {
			action = "file:update"
		}
		detail := "setup global " + report.Mode
		if plan.Exists {
			detail += "; file exists"
		}
		actions = append(actions, ApprovalPlannedAction{
			Action: action,
			Target: plan.Path,
			Detail: detail,
		})
	}
	return actions
}

func setupGlobalApprovalBlockers(report SetupGlobalReport) []string {
	blockers := []string{}
	for _, plan := range report.Files {
		if plan.Action == "conflict" {
			blockers = appendUniqueStrings(blockers, "setup_global_file_conflict")
		}
	}
	if strings.EqualFold(report.Status, "blocked") {
		blockers = appendUniqueStrings(blockers, "setup_global_blocked")
	}
	return stableStringSlice(blockers)
}

func setupGlobalPostApplyVerification(report SetupGlobalReport) string {
	if strings.TrimSpace(report.Repo) != "" {
		return fmt.Sprintf("gira config doctor --repo %s --config-root %s --json", QuoteShellArg(report.Repo), QuoteShellArg(report.ConfigRoot))
	}
	return "gira config doctor --config-root " + QuoteShellArg(report.ConfigRoot) + " --json"
}

func WorkspaceReposSyncApprovalEvidence(report WorkspaceRepoSyncReport) *ApprovalEvidence {
	applyCommand := workspaceReposSyncApprovalCommand(report, "--apply")
	dryRunCommand := workspaceReposSyncApprovalCommand(report, "--dry-run")
	return &ApprovalEvidence{
		SchemaVersion:         ApprovalPlanSchemaVersion,
		Capability:            AdapterCapabilityApplyMutation,
		CanonicalCommand:      "gira workspace repos sync",
		DryRunCommand:         dryRunCommand,
		ApplyCommand:          applyCommand,
		OutputSchema:          WorkspaceReposSyncReportSchemaVersion,
		PlannedActions:        workspaceReposSyncApprovalActions(report),
		Blockers:              workspaceReposSyncApprovalBlockers(report),
		Warnings:              stableStringSlice(report.Notes),
		PostApplyVerification: "gira workspace status --config " + QuoteShellArg(report.ConfigPath) + " --json",
	}
}

func workspaceReposSyncApprovalCommand(report WorkspaceRepoSyncReport, mode string) string {
	args := []string{"gira workspace repos sync"}
	if strings.TrimSpace(report.Owner) != "" {
		args = append(args, "--owner", QuoteShellArg(report.Owner))
	}
	if strings.TrimSpace(report.Workspace.Name) != "" {
		args = append(args, "--workspace", QuoteShellArg(report.Workspace.Name))
	}
	if strings.TrimSpace(report.ConfigRoot) != "" {
		args = append(args, "--config-root", QuoteShellArg(report.ConfigRoot))
	}
	if report.Limit > 0 {
		args = append(args, "--limit", fmt.Sprintf("%d", report.Limit))
	}
	if report.IncludeArchived {
		args = append(args, "--include-archived")
	}
	args = append(args, mode)
	return strings.Join(args, " ")
}

func workspaceReposSyncApprovalActions(report WorkspaceRepoSyncReport) []ApprovalPlannedAction {
	actions := []ApprovalPlannedAction{{
		Action: "file:" + strings.TrimSpace(report.File.Action),
		Target: report.File.Path,
		Detail: "update workspace repo registry",
	}}
	if actions[0].Action == "file:" {
		actions[0].Action = "file:update"
	}
	for _, repo := range report.AddedRepos {
		actions = append(actions, ApprovalPlannedAction{Action: "workspace_repo:add", Target: repo, Detail: "add discovered execution repo"})
	}
	for _, repo := range report.RemovedRepos {
		actions = append(actions, ApprovalPlannedAction{Action: "workspace_repo:remove", Target: repo, Detail: "remove repo missing from discovery target set"})
	}
	for _, repo := range report.SkippedRepos {
		actions = append(actions, ApprovalPlannedAction{Action: "workspace_repo:skip", Target: repo, Detail: "skip workspace inbox repo"})
	}
	return actions
}

func workspaceReposSyncApprovalBlockers(report WorkspaceRepoSyncReport) []string {
	blockers := []string{}
	if report.File.Action == "conflict" {
		blockers = appendUniqueStrings(blockers, "workspace_repos_sync_file_conflict")
	}
	if strings.EqualFold(report.Status, "blocked") {
		blockers = appendUniqueStrings(blockers, "workspace_repos_sync_blocked")
	}
	return stableStringSlice(blockers)
}

func AdoptRepoApprovalEvidence(report AdoptRepoReport) *ApprovalEvidence {
	applyCommand := adoptRepoApprovalCommand(report, "--apply")
	dryRunCommand := adoptRepoApprovalCommand(report, "--dry-run")
	return &ApprovalEvidence{
		SchemaVersion:         ApprovalPlanSchemaVersion,
		Capability:            AdapterCapabilityApplyMutation,
		CanonicalCommand:      "gira adopt repo",
		DryRunCommand:         dryRunCommand,
		ApplyCommand:          applyCommand,
		Repo:                  report.Repo,
		OutputSchema:          AdoptRepoReportSchemaVersion,
		PlannedActions:        adoptRepoApprovalActions(report),
		Blockers:              adoptRepoApprovalBlockers(report),
		Warnings:              stableStringSlice(report.Warnings),
		PostApplyVerification: fmt.Sprintf("gira config repo --repo %s --json", QuoteShellArg(report.Repo)),
	}
}

func adoptRepoApprovalCommand(report AdoptRepoReport, mode string) string {
	args := []string{
		"gira adopt repo",
		"--repo", QuoteShellArg(report.Repo),
		"--path", QuoteShellArg(report.Path),
	}
	if strings.TrimSpace(report.Strategy) != "" {
		args = append(args, "--strategy", QuoteShellArg(report.Strategy))
	}
	args = append(args, mode)
	return strings.Join(args, " ")
}

func adoptRepoApprovalActions(report AdoptRepoReport) []ApprovalPlannedAction {
	actions := make([]ApprovalPlannedAction, 0, len(report.Actions))
	for _, action := range report.Actions {
		if strings.TrimSpace(action.Action) == "" {
			continue
		}
		detail := strings.TrimSpace(action.Reason)
		if strings.TrimSpace(action.Status) != "" {
			if detail == "" {
				detail = action.Status
			} else {
				detail = action.Status + ": " + detail
			}
		}
		actions = append(actions, ApprovalPlannedAction{
			Action: action.Action,
			Target: action.Target,
			Detail: detail,
		})
	}
	return actions
}

func adoptRepoApprovalBlockers(report AdoptRepoReport) []string {
	blockers := []string{}
	for _, action := range report.Actions {
		if action.Status == "conflict" {
			blockers = appendUniqueStrings(blockers, "adopt_repo_conflict_action")
		}
	}
	return stableStringSlice(blockers)
}

func AdoptIssuesApprovalEvidence(report AdoptIssuesReport) *ApprovalEvidence {
	applyCommand := adoptIssuesApprovalCommand(report, "--apply")
	dryRunCommand := adoptIssuesApprovalCommand(report, "--dry-run")
	return &ApprovalEvidence{
		SchemaVersion:         ApprovalPlanSchemaVersion,
		Capability:            AdapterCapabilityApplyMutation,
		CanonicalCommand:      "gira adopt issues",
		DryRunCommand:         dryRunCommand,
		ApplyCommand:          applyCommand,
		Repo:                  report.Repo,
		OutputSchema:          AdoptIssuesReportSchemaVersion,
		PlannedActions:        adoptIssuesApprovalActions(report),
		Blockers:              adoptIssuesApprovalBlockers(report),
		Warnings:              []string{},
		PostApplyVerification: fmt.Sprintf("gira status --repo %s --json", QuoteShellArg(report.Repo)),
	}
}

func adoptIssuesApprovalCommand(report AdoptIssuesReport, mode string) string {
	args := []string{
		"gira adopt issues",
		"--repo", QuoteShellArg(report.Repo),
	}
	if strings.TrimSpace(report.State) != "" {
		args = append(args, "--state", QuoteShellArg(report.State))
	}
	if len(report.Issues) > 0 {
		args = append(args, "--issues", joinIssueNumbers(report.Issues))
	}
	if strings.TrimSpace(report.Milestone) != "" {
		args = append(args, "--milestone", QuoteShellArg(report.Milestone))
	}
	for _, label := range report.Labels {
		args = append(args, "--label", QuoteShellArg(label))
	}
	if report.NormalizeStatus {
		args = append(args, "--normalize-status")
	}
	args = append(args, mode)
	return strings.Join(args, " ")
}

func adoptIssuesApprovalActions(report AdoptIssuesReport) []ApprovalPlannedAction {
	actions := make([]ApprovalPlannedAction, 0, len(report.Actions))
	for _, action := range report.Actions {
		if strings.TrimSpace(action.Action) == "" {
			continue
		}
		detail := strings.TrimSpace(action.Reason)
		if strings.TrimSpace(action.Status) != "" {
			if detail == "" {
				detail = action.Status
			} else {
				detail = action.Status + ": " + detail
			}
		}
		if strings.TrimSpace(action.Milestone) != "" {
			detail = appendApprovalDetail(detail, "milestone="+action.Milestone)
		}
		if len(action.Labels) > 0 {
			detail = appendApprovalDetail(detail, "labels="+strings.Join(action.Labels, ","))
		}
		if len(action.RemoveLabels) > 0 {
			detail = appendApprovalDetail(detail, "remove_labels="+strings.Join(action.RemoveLabels, ","))
		}
		actions = append(actions, ApprovalPlannedAction{
			Action: action.Action,
			Target: fmt.Sprintf("#%d", action.Issue),
			Detail: detail,
		})
	}
	return actions
}

func appendApprovalDetail(detail string, suffix string) string {
	suffix = strings.TrimSpace(suffix)
	if suffix == "" {
		return detail
	}
	if strings.TrimSpace(detail) == "" {
		return suffix
	}
	return detail + "; " + suffix
}

func adoptIssuesApprovalBlockers(report AdoptIssuesReport) []string {
	blockers := []string{}
	if report.DryRun && len(report.Actions) == 0 {
		blockers = appendUniqueStrings(blockers, "adopt_issues_no_planned_actions")
	}
	for _, action := range report.Actions {
		if action.Status == "blocked" || action.Status == "conflict" {
			blockers = appendUniqueStrings(blockers, "adopt_issues_blocked_action")
		}
	}
	return stableStringSlice(blockers)
}

func MilestoneApprovalEvidence(report MilestoneReport) *ApprovalEvidence {
	applyCommand := milestoneApprovalCommand(report, "--apply")
	dryRunCommand := milestoneApprovalCommand(report, "--dry-run")
	return &ApprovalEvidence{
		SchemaVersion:         ApprovalPlanSchemaVersion,
		Capability:            AdapterCapabilityApplyMutation,
		CanonicalCommand:      "gira " + report.Command,
		DryRunCommand:         dryRunCommand,
		ApplyCommand:          applyCommand,
		Repo:                  report.Repo,
		OutputSchema:          MilestoneReportSchemaVersion,
		PlannedActions:        milestoneApprovalActions(report),
		Blockers:              milestoneApprovalBlockers(report),
		Warnings:              []string{},
		PostApplyVerification: fmt.Sprintf("gira milestone status %s --repo %s --json", QuoteShellArg(milestoneApprovalTitle(report)), QuoteShellArg(report.Repo)),
	}
}

func milestoneApprovalCommand(report MilestoneReport, mode string) string {
	title := milestoneApprovalTitle(report)
	args := []string{"gira " + report.Command}
	if title != "" {
		args = append(args, QuoteShellArg(title))
	}
	args = append(args, "--repo", QuoteShellArg(report.Repo))
	switch report.Command {
	case "milestone new":
		if report.Milestone != nil {
			if strings.TrimSpace(report.Milestone.Description) != "" {
				args = append(args, "--description", QuoteShellArg(report.Milestone.Description))
			}
			if report.Milestone.DueOn != nil && strings.TrimSpace(*report.Milestone.DueOn) != "" {
				args = append(args, "--due-on", QuoteShellArg(*report.Milestone.DueOn))
			}
		}
	case "milestone assign":
		if tickets := milestoneApprovalTickets(report); len(tickets) > 0 {
			args = append(args, "--tickets", joinIssueNumbers(tickets))
		}
	case "milestone plan":
		if strings.TrimSpace(report.Filters.State) != "" {
			args = append(args, "--state", QuoteShellArg(report.Filters.State))
		}
		for _, label := range report.Filters.Labels {
			args = append(args, "--label", QuoteShellArg(label))
		}
		if report.Filters.Limit > 0 {
			args = append(args, "--limit", fmt.Sprintf("%d", report.Filters.Limit))
		}
	}
	args = append(args, mode)
	return strings.Join(args, " ")
}

func milestoneApprovalTitle(report MilestoneReport) string {
	if report.Milestone != nil && strings.TrimSpace(report.Milestone.Title) != "" {
		return strings.TrimSpace(report.Milestone.Title)
	}
	if strings.TrimSpace(report.Filters.Milestone) != "" {
		return strings.TrimSpace(report.Filters.Milestone)
	}
	for _, action := range report.Actions {
		if strings.TrimSpace(action.Milestone) != "" {
			return strings.TrimSpace(action.Milestone)
		}
	}
	return ""
}

func milestoneApprovalTickets(report MilestoneReport) []int {
	seen := map[int]struct{}{}
	tickets := []int{}
	for _, action := range report.Actions {
		if action.Issue <= 0 {
			continue
		}
		if _, ok := seen[action.Issue]; ok {
			continue
		}
		seen[action.Issue] = struct{}{}
		tickets = append(tickets, action.Issue)
	}
	sort.Ints(tickets)
	return tickets
}

func milestoneApprovalActions(report MilestoneReport) []ApprovalPlannedAction {
	actions := make([]ApprovalPlannedAction, 0, len(report.Actions))
	for _, action := range report.Actions {
		if strings.TrimSpace(action.Action) == "" {
			continue
		}
		target := action.Milestone
		if action.Issue > 0 {
			target = fmt.Sprintf("#%d", action.Issue)
		}
		detail := strings.TrimSpace(action.Reason)
		if strings.TrimSpace(action.Status) != "" {
			if detail == "" {
				detail = action.Status
			} else {
				detail = action.Status + ": " + detail
			}
		}
		if action.Issue > 0 && strings.TrimSpace(action.Milestone) != "" {
			detail = appendApprovalDetail(detail, "milestone="+action.Milestone)
		}
		actions = append(actions, ApprovalPlannedAction{
			Action: action.Action,
			Target: target,
			Detail: detail,
		})
	}
	return actions
}

func milestoneApprovalBlockers(report MilestoneReport) []string {
	blockers := []string{}
	hasPlanned := false
	for _, action := range report.Actions {
		if action.Status == "planned" {
			hasPlanned = true
		}
		if action.Status == "blocked" || action.Status == "conflict" {
			blockers = appendUniqueStrings(blockers, "milestone_blocked_action")
		}
	}
	if report.DryRun && !hasPlanned {
		blockers = appendUniqueStrings(blockers, "milestone_no_planned_actions")
	}
	return stableStringSlice(blockers)
}

func CachePruneApprovalEvidence(report CachePruneReport) *ApprovalEvidence {
	applyCommand := cachePruneApprovalCommand(report, "--apply")
	dryRunCommand := cachePruneApprovalCommand(report, "--dry-run")
	return &ApprovalEvidence{
		SchemaVersion:         ApprovalPlanSchemaVersion,
		Capability:            AdapterCapabilityApplyMutation,
		CanonicalCommand:      "gira cache prune",
		DryRunCommand:         dryRunCommand,
		ApplyCommand:          applyCommand,
		OutputSchema:          CachePruneReportSchemaVersion,
		PlannedActions:        cachePruneApprovalActions(report),
		Blockers:              cachePruneApprovalBlockers(report),
		Warnings:              []string{},
		PostApplyVerification: dryRunCommand + " --json",
	}
}

func cachePruneApprovalCommand(report CachePruneReport, mode string) string {
	args := []string{"gira cache prune"}
	if strings.TrimSpace(report.Root) != "" {
		args = append(args, "--root", QuoteShellArg(report.Root))
	}
	args = append(args, mode)
	return strings.Join(args, " ")
}

func cachePruneApprovalActions(report CachePruneReport) []ApprovalPlannedAction {
	actions := []ApprovalPlannedAction{}
	for _, action := range report.Actions {
		if action.Status != "planned" || action.Action != "prune" {
			continue
		}
		detail := strings.TrimSpace(action.Reason)
		if strings.TrimSpace(action.Version) != "" {
			detail = appendApprovalDetail(detail, "version="+action.Version)
		}
		actions = append(actions, ApprovalPlannedAction{
			Action: "cache:prune",
			Target: action.Path,
			Detail: detail,
		})
	}
	return actions
}

func cachePruneApprovalBlockers(report CachePruneReport) []string {
	blockers := []string{}
	if report.DryRun && report.Counts.Planned == 0 {
		blockers = appendUniqueStrings(blockers, "cache_prune_no_planned_actions")
	}
	if report.Counts.Errors > 0 {
		blockers = appendUniqueStrings(blockers, "cache_prune_action_error")
	}
	return stableStringSlice(blockers)
}

func SprintPlanApprovalEvidence(report SprintPlanReport) *ApprovalEvidence {
	applyCommand := sprintPlanApprovalCommand(report, "--apply")
	dryRunCommand := sprintPlanApprovalCommand(report, "--dry-run")
	return &ApprovalEvidence{
		SchemaVersion:         ApprovalPlanSchemaVersion,
		Capability:            AdapterCapabilityApplyMutation,
		CanonicalCommand:      "gira sprint plan",
		DryRunCommand:         dryRunCommand,
		ApplyCommand:          applyCommand,
		Repo:                  report.Repo,
		OutputSchema:          SprintPlanReportSchemaVersion,
		PlannedActions:        sprintPlanApprovalActions(report),
		Blockers:              []string{},
		Warnings:              sprintPlanApprovalWarnings(report),
		PostApplyVerification: sprintStartApprovalCommand(SprintStartReport{Repo: report.Repo, Iteration: report.Iteration}, "--dry-run") + " --json",
	}
}

func sprintPlanApprovalCommand(report SprintPlanReport, mode string) string {
	args := []string{
		"gira sprint plan",
		"--repo", QuoteShellArg(report.Repo),
		"--iteration", QuoteShellArg(report.Iteration),
		"--capacity", fmt.Sprintf("%d", report.Capacity),
	}
	if len(report.Sprint.CommittedItems) > 0 {
		args = append(args, "--issues", joinIssueNumbers(report.Sprint.CommittedItems))
	}
	args = append(args, mode)
	return strings.Join(args, " ")
}

func sprintPlanApprovalActions(report SprintPlanReport) []ApprovalPlannedAction {
	return []ApprovalPlannedAction{{
		Action: "sprint:plan",
		Target: report.Iteration,
		Detail: fmt.Sprintf("persist sprint plan capacity=%d committed=%s", report.Capacity, joinIssueNumbers(report.Sprint.CommittedItems)),
	}}
}

func sprintPlanApprovalWarnings(report SprintPlanReport) []string {
	if report.CapacityBreach {
		return []string{"sprint_capacity_breach"}
	}
	return []string{}
}

func SprintStartApprovalEvidence(report SprintStartReport) *ApprovalEvidence {
	applyCommand := sprintStartApprovalCommand(report, "--apply")
	dryRunCommand := sprintStartApprovalCommand(report, "--dry-run")
	return &ApprovalEvidence{
		SchemaVersion:         ApprovalPlanSchemaVersion,
		Capability:            AdapterCapabilityApplyMutation,
		CanonicalCommand:      "gira sprint start",
		DryRunCommand:         dryRunCommand,
		ApplyCommand:          applyCommand,
		Repo:                  report.Repo,
		OutputSchema:          SprintStartReportSchemaVersion,
		PlannedActions:        sprintStartApprovalActions(report),
		Blockers:              []string{},
		Warnings:              []string{},
		PostApplyVerification: dryRunCommand + " --json",
	}
}

func sprintStartApprovalCommand(report SprintStartReport, mode string) string {
	args := []string{
		"gira sprint start",
		"--repo", QuoteShellArg(report.Repo),
		"--iteration", QuoteShellArg(report.Iteration),
		mode,
	}
	return strings.Join(args, " ")
}

func sprintStartApprovalActions(report SprintStartReport) []ApprovalPlannedAction {
	return []ApprovalPlannedAction{{
		Action: "sprint:start",
		Target: report.Iteration,
		Detail: "freeze sprint commitment and record started_at",
	}}
}

func SprintCloseApprovalEvidence(report SprintCloseReport) *ApprovalEvidence {
	applyCommand := sprintCloseApprovalCommand(report, "--apply")
	dryRunCommand := sprintCloseApprovalCommand(report, "--dry-run")
	return &ApprovalEvidence{
		SchemaVersion:         ApprovalPlanSchemaVersion,
		Capability:            AdapterCapabilityApplyMutation,
		CanonicalCommand:      "gira sprint close",
		DryRunCommand:         dryRunCommand,
		ApplyCommand:          applyCommand,
		Repo:                  report.Repo,
		OutputSchema:          SprintCloseReportSchemaVersion,
		PlannedActions:        sprintCloseApprovalActions(report),
		Blockers:              sprintCloseApprovalBlockers(report),
		Warnings:              []string{},
		PostApplyVerification: dryRunCommand + " --json",
	}
}

func sprintCloseApprovalCommand(report SprintCloseReport, mode string) string {
	args := []string{
		"gira sprint close",
		"--repo", QuoteShellArg(report.Repo),
		"--iteration", QuoteShellArg(report.Iteration),
	}
	if len(report.Summary.CompletedItems) > 0 {
		args = append(args, "--completed", joinIssueNumbers(report.Summary.CompletedItems))
	}
	if strings.TrimSpace(report.Summary.SpilloverDisposition) != "" {
		args = append(args, "--spillover-disposition", QuoteShellArg(report.Summary.SpilloverDisposition))
	}
	if strings.TrimSpace(report.Summary.RolloverReason) != "" {
		args = append(args, "--rollover-reason", QuoteShellArg(report.Summary.RolloverReason))
	}
	args = append(args, mode)
	return strings.Join(args, " ")
}

func sprintCloseApprovalActions(report SprintCloseReport) []ApprovalPlannedAction {
	detail := fmt.Sprintf("completed=%s spillover=%s disposition=%s", joinIssueNumbers(report.Summary.CompletedItems), joinIssueNumbers(report.Summary.SpilloverItems), report.Summary.SpilloverDisposition)
	if strings.TrimSpace(report.Summary.RolloverReason) != "" {
		detail = appendApprovalDetail(detail, "reason="+report.Summary.RolloverReason)
	}
	return []ApprovalPlannedAction{{
		Action: "sprint:close",
		Target: report.Iteration,
		Detail: detail,
	}}
}

func sprintCloseApprovalBlockers(report SprintCloseReport) []string {
	blockers := []string{}
	if strings.TrimSpace(report.Summary.SpilloverDisposition) == "" {
		blockers = appendUniqueStrings(blockers, "sprint_close_missing_spillover_disposition")
	}
	if strings.TrimSpace(report.Summary.RolloverReason) == "" {
		blockers = appendUniqueStrings(blockers, "sprint_close_missing_rollover_reason")
	}
	return stableStringSlice(blockers)
}

func SprintRolloverApprovalEvidence(report SprintRolloverReport) *ApprovalEvidence {
	applyCommand := sprintRolloverApprovalCommand(report, "--apply")
	dryRunCommand := sprintRolloverApprovalCommand(report, "--dry-run")
	return &ApprovalEvidence{
		SchemaVersion:         ApprovalPlanSchemaVersion,
		Capability:            AdapterCapabilityApplyMutation,
		CanonicalCommand:      "gira sprint rollover",
		DryRunCommand:         dryRunCommand,
		ApplyCommand:          applyCommand,
		Repo:                  report.Repo,
		OutputSchema:          SprintRolloverReportSchemaVersion,
		PlannedActions:        sprintRolloverApprovalActions(report),
		Blockers:              sprintRolloverApprovalBlockers(report),
		Warnings:              []string{},
		PostApplyVerification: dryRunCommand + " --json",
	}
}

func sprintRolloverApprovalCommand(report SprintRolloverReport, mode string) string {
	args := []string{
		"gira sprint rollover",
		"--repo", QuoteShellArg(report.Repo),
	}
	if report.TargetMilestone != nil && report.TargetResolution == "explicit --to" {
		args = append(args, "--to", QuoteShellArg(report.TargetMilestone.Title))
	}
	args = append(args, mode)
	return strings.Join(args, " ")
}

func sprintRolloverApprovalActions(report SprintRolloverReport) []ApprovalPlannedAction {
	actions := []ApprovalPlannedAction{}
	for _, item := range report.Items {
		if item.Action != "would-apply" {
			continue
		}
		detail := item.FromMilestone + " -> " + item.TargetMilestone
		if strings.TrimSpace(item.CandidateReason) != "" {
			detail = appendApprovalDetail(detail, "reason="+item.CandidateReason)
		}
		actions = append(actions, ApprovalPlannedAction{
			Action: "issue:rollover-milestone",
			Target: fmt.Sprintf("#%d", item.IssueNumber),
			Detail: detail,
		})
	}
	return actions
}

func sprintRolloverApprovalBlockers(report SprintRolloverReport) []string {
	blockers := []string{}
	if report.Mode == "dry-run" && len(sprintRolloverApprovalActions(report)) == 0 {
		blockers = appendUniqueStrings(blockers, "sprint_rollover_no_planned_actions")
	}
	for _, item := range report.Items {
		if item.Action == "skipped" && item.SkipReason == "no target open milestone" {
			blockers = appendUniqueStrings(blockers, "sprint_rollover_no_target_milestone")
		}
		if item.Action == "skipped" && strings.HasPrefix(item.SkipReason, "apply failed:") {
			blockers = appendUniqueStrings(blockers, "sprint_rollover_apply_failed")
		}
	}
	return stableStringSlice(blockers)
}
