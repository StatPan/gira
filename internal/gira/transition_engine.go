package gira

import (
	"fmt"
	"strings"
)

// TransitionContext contains the inputs needed by registered transition steps.
// Callers must supply the evidence collected by the corresponding lifecycle command.
type TransitionContext struct {
	Repo                  RepoRef
	Issue                 int
	IssueBody, IssueState string
	Labels                []string
	Policy                *ResolvedOperationPolicy
	Branch, Base          string
	PRBody                string
	TargetLabel           string
	Runner                CommandRunner
	Finish                *WorkFinishResult
}

type TransitionStep func(*TransitionContext) error

type TransitionRegistry struct {
	Conditions, Validators, Post map[string]TransitionStep
}

type TransitionStepOutcome struct {
	Kind, Name, Status, Detail string
}

type TransitionResult struct {
	Name, From, To   string
	Available        bool
	Steps            []TransitionStepOutcome
	Blockers         []string
	Planned, Applied []string
}

// RunTransition selects the first matching declaration and runs its rules in order.
// Apply is explicit; a dry run never invokes post functions.
func RunTransition(w Workflow, from, to string, ctx *TransitionContext, registry TransitionRegistry, apply bool) TransitionResult {
	result := TransitionResult{From: from, To: to}
	var selected *WorkflowTransition
	for i := range w.Transitions {
		t := &w.Transitions[i]
		if t.To != to {
			continue
		}
		for _, source := range t.From {
			if source == from || source == "any" || source == "global" {
				selected = t
				break
			}
		}
		if selected != nil {
			break
		}
	}
	if selected == nil {
		result.Blockers = append(result.Blockers, "missing_transition")
		return result
	}
	result.Name, result.Available = selected.Name, true
	run := func(kind string, names []string, steps map[string]TransitionStep) bool {
		success := true
		for _, name := range names {
			outcome := TransitionStepOutcome{Kind: kind, Name: name}
			step, ok := steps[name]
			if !ok || step == nil {
				outcome.Status, outcome.Detail = "failed", "unknown step"
			} else if kind == "post" && !apply {
				outcome.Status = "planned"
				result.Planned = append(result.Planned, name)
			} else if err := step(ctx); err != nil {
				outcome.Status, outcome.Detail = "failed", err.Error()
			} else {
				outcome.Status = "passed"
				if kind == "post" {
					outcome.Status = "applied"
					result.Applied = append(result.Applied, name)
				}
			}
			result.Steps = append(result.Steps, outcome)
			if outcome.Status == "failed" {
				result.Blockers = append(result.Blockers, kind+":"+name)
				success = false
				if kind == "post" {
					break
				}
			}
		}
		return success
	}
	if !run("condition", selected.Conditions, registry.Conditions) {
		result.Available = false
		return result
	}
	if !run("validator", selected.Validators, registry.Validators) {
		return result
	}
	run("post", selected.Post, registry.Post)
	return result
}

func defaultTransitionRegistry() TransitionRegistry {
	unavailable := func(*TransitionContext) error { return fmt.Errorf("unavailable in phase 2") }
	return TransitionRegistry{
		Conditions: map[string]TransitionStep{
			"open_issue": unavailable, "ready_status": unavailable, "clean_worktree": unavailable,
			"branch_exists": unavailable, "linked_pr_selected": unavailable,
		},
		Validators: map[string]TransitionStep{
			"remote_branch_conflict": unavailable, "pr_base": unavailable, "replacement_ticket_created": unavailable,
			"ticket_readiness": func(c *TransitionContext) error {
				if c == nil || c.Policy == nil {
					return fmt.Errorf("ticket readiness input unavailable")
				}
				report := EvaluateTicketReadinessWithPolicy(c.IssueBody, c.Labels, c.IssueState, *c.Policy)
				if report.Readiness != "ready" {
					return fmt.Errorf("ticket readiness: %s", report.Readiness)
				}
				return nil
			},
			"branch_reuse_safety": func(c *TransitionContext) error {
				if c == nil || c.Runner == nil || c.Branch == "" || c.Base == "" {
					return fmt.Errorf("branch reuse input unavailable")
				}
				report, err := inspectExistingBranchReuse(c.Branch, c.Base, c.Runner)
				if err != nil {
					return err
				}
				if !report.Safe {
					return fmt.Errorf("unsafe branch reuse: %s", strings.Join(report.Diagnostics, "; "))
				}
				return nil
			},
			"closing_reference": func(c *TransitionContext) error {
				if c == nil || c.Issue <= 0 {
					return fmt.Errorf("closing reference input unavailable")
				}
				if !hasClosingKeyword(c.PRBody, c.Issue) {
					return fmt.Errorf("missing closing reference")
				}
				return nil
			},
			"finish_readiness": func(c *TransitionContext) error {
				if c == nil || c.Finish == nil {
					return fmt.Errorf("finish readiness input unavailable")
				}
				report := buildWorkFinishReadiness(*c.Finish)
				if !report.Ready {
					return fmt.Errorf("finish not ready: %s", strings.Join(report.Blockers, ", "))
				}
				return nil
			},
		},
		Post: map[string]TransitionStep{
			"create_or_reuse_branch": unavailable, "create_or_update_pr": unavailable,
			"merge": unavailable, "local_sync": unavailable, "jira_done": unavailable, "decision_note": unavailable,
			"set_status_label": func(c *TransitionContext) error {
				if c == nil || c.Runner == nil || c.Issue <= 0 || c.TargetLabel == "" {
					return fmt.Errorf("status label input unavailable")
				}
				return setIssueStatus(c.Repo, c.Issue, c.Labels, c.TargetLabel, c.Runner)
			},
			// Closing is embedded in the finish command's larger mutation sequence.
			"close_issue": unavailable,
			"finish_receipt": func(c *TransitionContext) error {
				if c == nil || c.Runner == nil || c.Finish == nil || c.Finish.DryRun || c.Finish.Receipt.SchemaVersion == "" {
					return fmt.Errorf("finish receipt input unavailable")
				}
				action, err := maybeApplyWorkFinishReceipt(c.Repo, c.Runner, c.Finish, nil)
				if err != nil {
					return err
				}
				if action.Status != "applied" {
					return fmt.Errorf("finish receipt %s: %s", action.Status, action.Detail)
				}
				return nil
			},
		},
	}
}
