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
		Blockers:              append([]string(nil), result.Blockers...),
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
