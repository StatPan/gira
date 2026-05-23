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
