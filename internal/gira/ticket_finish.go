package gira

import (
	"fmt"
	"strings"
	"time"
)

type WorkFinishAction struct {
	Action string `json:"action"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type WorkFinishLocalSync struct {
	Attempted bool   `json:"attempted"`
	Skipped   bool   `json:"skipped"`
	Reason    string `json:"reason,omitempty"`
	Branch    string `json:"branch,omitempty"`
}

type WorkFinishResult struct {
	Repo        string              `json:"repo"`
	Issue       int                 `json:"issue"`
	DryRun      bool                `json:"dry_run"`
	Wait        string              `json:"wait"`
	PRNumber    int                 `json:"pr_number,omitempty"`
	PRURL       string              `json:"pr_url,omitempty"`
	PRState     string              `json:"pr_state,omitempty"`
	Merged      bool                `json:"merged"`
	AlreadyDone bool                `json:"already_done"`
	Actions     []WorkFinishAction  `json:"actions"`
	Blockers    []string            `json:"blockers"`
	LocalSync   WorkFinishLocalSync `json:"local_sync"`
	FinalStatus WorkStatusResult    `json:"final_status"`
	NextStep    string              `json:"next_step"`
}

func FinishWork(repo RepoRef, issueNumber int, dryRun bool, wait time.Duration, runner CommandRunner) (WorkFinishResult, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if issueNumber <= 0 {
		return WorkFinishResult{}, fmt.Errorf("ticket must be > 0")
	}
	result := WorkFinishResult{
		Repo:     repo.FullName(),
		Issue:    issueNumber,
		DryRun:   dryRun,
		Wait:     wait.String(),
		Actions:  []WorkFinishAction{},
		Blockers: []string{},
		NextStep: fmt.Sprintf("gira ticket status --repo %s --ticket %d", repo.FullName(), issueNumber),
	}

	status, err := DevPRStatus(repo, issueNumber, runner)
	if err != nil {
		return result, err
	}
	result.PRNumber = status.PRNumber
	result.PRURL = status.PRURL
	result.PRState = status.State
	result.Actions = append(result.Actions, WorkFinishAction{Action: "linked_pr:inspect", Status: "done", Detail: linkedPRDetail(status)})
	if status.PRNumber == 0 {
		result.Blockers = append(result.Blockers, "missing_linked_pr")
		result.NextStep = fmt.Sprintf("gira ticket pr --repo %s --ticket %d --apply", repo.FullName(), issueNumber)
		return finishWithStatus(repo, issueNumber, runner, result, nil)
	}
	if strings.EqualFold(status.State, "MERGED") {
		result.AlreadyDone = true
		result.Merged = true
		result.Actions = append(result.Actions, WorkFinishAction{Action: "pr:merge", Status: "skipped", Detail: "PR is already merged"})
		return finishWithLocalSync(repo, issueNumber, runner, result, true)
	}

	if containsString(status.Blockers, "draft") {
		result.Actions = append(result.Actions, plannedOrAppliedAction("pr:ready", dryRun, fmt.Sprintf("mark PR #%d ready for review", status.PRNumber)))
		if dryRun {
			result.Blockers = append(result.Blockers, "draft")
			result.NextStep = fmt.Sprintf("gira ticket finish --repo %s --ticket %d --apply", repo.FullName(), issueNumber)
			return finishWithStatus(repo, issueNumber, runner, result, nil)
		}
		if _, err := runner.Run("gh", "pr", "ready", fmt.Sprintf("%d", status.PRNumber), "--repo", repo.FullName()); err != nil {
			return result, fmt.Errorf("mark PR ready: %w", err)
		}
		status, err = DevPRStatus(repo, issueNumber, runner)
		if err != nil {
			return result, err
		}
		result.PRState = status.State
	}

	if containsString(status.Blockers, "checks_pending") && wait > 0 {
		result.Actions = append(result.Actions, WorkFinishAction{Action: "checks:wait", Status: "applied", Detail: wait.String()})
		deadline := time.Now().Add(wait)
		for containsString(status.Blockers, "checks_pending") && time.Now().Before(deadline) {
			time.Sleep(5 * time.Second)
			status, err = DevPRStatus(repo, issueNumber, runner)
			if err != nil {
				return result, err
			}
		}
	}

	result.Blockers = mergeBlockers(status.Blockers)
	if len(result.Blockers) > 0 {
		result.Actions = append(result.Actions, WorkFinishAction{Action: "pr:merge", Status: "blocked", Detail: strings.Join(result.Blockers, ",")})
		result.NextStep = finishBlockedNextStep(repo, issueNumber, result.Blockers)
		report, reportErr := finishWithStatus(repo, issueNumber, runner, result, nil)
		if dryRun {
			return report, reportErr
		}
		if reportErr != nil {
			return report, reportErr
		}
		return report, fmt.Errorf("ticket finish blocked: %s", strings.Join(result.Blockers, ", "))
	}

	result.Actions = append(result.Actions, plannedOrAppliedAction("pr:merge", dryRun, fmt.Sprintf("squash merge PR #%d and delete remote branch", status.PRNumber)))
	if dryRun {
		return finishWithLocalSync(repo, issueNumber, runner, result, true)
	}
	if _, err := runner.Run("gh", "pr", "merge", fmt.Sprintf("%d", status.PRNumber), "--repo", repo.FullName(), "--squash", "--delete-branch"); err != nil {
		return result, fmt.Errorf("merge PR: %w", err)
	}
	result.Merged = true
	return finishWithLocalSync(repo, issueNumber, runner, result, true)
}

func finishWithLocalSync(repo RepoRef, issueNumber int, runner CommandRunner, result WorkFinishResult, mergePlanned bool) (WorkFinishResult, error) {
	local, actions := planLocalMainSync(repo, runner, result.DryRun)
	result.LocalSync = local
	result.Actions = append(result.Actions, actions...)
	if !result.DryRun && mergePlanned && local.Attempted && !local.Skipped {
		if _, err := runner.Run("git", "checkout", "main"); err != nil {
			return result, fmt.Errorf("checkout main: %w", err)
		}
		if _, err := runner.Run("git", "pull", "--ff-only", "origin", "main"); err != nil {
			return result, fmt.Errorf("pull main: %w", err)
		}
	}
	report, err := finishWithStatus(repo, issueNumber, runner, result, nil)
	if report.DryRun && mergePlanned && !report.AlreadyDone && len(report.Blockers) == 0 {
		report.NextStep = fmt.Sprintf("gira ticket finish --repo %s --ticket %d --apply", repo.FullName(), issueNumber)
	}
	return report, err
}

func finishWithStatus(repo RepoRef, issueNumber int, runner CommandRunner, result WorkFinishResult, err error) (WorkFinishResult, error) {
	status, statusErr := GetWorkStatus(repo, issueNumber, runner)
	if statusErr == nil {
		result.FinalStatus = status
		if len(result.Blockers) == 0 {
			result.NextStep = status.NextStep
		}
	} else if len(result.Blockers) == 0 {
		result.Blockers = append(result.Blockers, "final_status_unavailable")
		result.Actions = append(result.Actions, WorkFinishAction{Action: "ticket:status", Status: "blocked", Detail: statusErr.Error()})
	}
	result.Actions = append(result.Actions, WorkFinishAction{Action: "projects:sync", Status: "planned", Detail: "gira projects sync --dry-run"})
	return result, err
}

func planLocalMainSync(repo RepoRef, runner CommandRunner, dryRun bool) (WorkFinishLocalSync, []WorkFinishAction) {
	local := WorkFinishLocalSync{}
	actions := []WorkFinishAction{}
	remoteOut, err := runner.Run("git", "remote", "get-url", "origin")
	if err != nil {
		local.Skipped = true
		local.Reason = "not_git_checkout"
		actions = append(actions, WorkFinishAction{Action: "local:sync_main", Status: "skipped", Detail: local.Reason})
		return local, actions
	}
	currentRepo, err := ParseGitHubRemoteRepo(strings.TrimSpace(string(remoteOut)))
	if err != nil || !strings.EqualFold(currentRepo.FullName(), repo.FullName()) {
		local.Skipped = true
		local.Reason = "checkout_repo_mismatch"
		actions = append(actions, WorkFinishAction{Action: "local:sync_main", Status: "skipped", Detail: local.Reason})
		return local, actions
	}
	branchOut, err := runner.Run("git", "branch", "--show-current")
	if err != nil {
		local.Skipped = true
		local.Reason = "current_branch_unavailable"
		actions = append(actions, WorkFinishAction{Action: "local:sync_main", Status: "skipped", Detail: local.Reason})
		return local, actions
	}
	local.Branch = strings.TrimSpace(string(branchOut))
	statusOut, err := runner.Run("git", "status", "--porcelain")
	if err != nil {
		local.Skipped = true
		local.Reason = "worktree_status_unavailable"
		actions = append(actions, WorkFinishAction{Action: "local:sync_main", Status: "skipped", Detail: local.Reason})
		return local, actions
	}
	if strings.TrimSpace(string(statusOut)) != "" {
		local.Skipped = true
		local.Reason = "dirty_worktree"
		actions = append(actions, WorkFinishAction{Action: "local:sync_main", Status: "skipped", Detail: local.Reason})
		return local, actions
	}
	local.Attempted = true
	actions = append(actions, plannedOrAppliedAction("local:sync_main", dryRun, "checkout main and pull --ff-only"))
	return local, actions
}

func mergeBlockers(blockers []string) []string {
	result := make([]string, 0)
	for _, blocker := range blockers {
		switch blocker {
		case "missing_linked_pr", "draft", "review", "checks", "checks_pending":
			result = append(result, blocker)
		}
	}
	return result
}

func finishBlockedNextStep(repo RepoRef, issueNumber int, blockers []string) string {
	if containsString(blockers, "checks_pending") {
		return "wait for required checks, then " + fmt.Sprintf("gira ticket finish --repo %s --ticket %d --apply", repo.FullName(), issueNumber)
	}
	if containsString(blockers, "checks") {
		return "fix failing checks, then " + fmt.Sprintf("gira ticket finish --repo %s --ticket %d --apply", repo.FullName(), issueNumber)
	}
	if containsString(blockers, "review") {
		return "resolve review requirements, then " + fmt.Sprintf("gira ticket finish --repo %s --ticket %d --apply", repo.FullName(), issueNumber)
	}
	if containsString(blockers, "draft") {
		return fmt.Sprintf("gira ticket finish --repo %s --ticket %d --apply", repo.FullName(), issueNumber)
	}
	return fmt.Sprintf("gira ticket status --repo %s --ticket %d", repo.FullName(), issueNumber)
}

func linkedPRDetail(status DevPRStatusResult) string {
	if status.PRNumber == 0 {
		return "no PR with closing keyword found"
	}
	return fmt.Sprintf("PR #%d %s", status.PRNumber, status.State)
}

func plannedOrAppliedAction(action string, dryRun bool, detail string) WorkFinishAction {
	status := "applied"
	if dryRun {
		status = "planned"
	}
	return WorkFinishAction{Action: action, Status: status, Detail: detail}
}

func FormatWorkFinish(result WorkFinishResult) string {
	blockers := strings.Join(result.Blockers, ",")
	if blockers == "" {
		blockers = "none"
	}
	actions := make([]string, 0, len(result.Actions))
	for _, action := range result.Actions {
		actions = append(actions, action.Action+":"+action.Status)
	}
	if len(actions) == 0 {
		actions = append(actions, "none")
	}
	return fmt.Sprintf(
		"work finish: issue #%d pr=%d merged=%t blockers=%s actions=%s\nnext step: %s\n",
		result.Issue,
		result.PRNumber,
		result.Merged,
		blockers,
		strings.Join(actions, ","),
		result.NextStep,
	)
}
