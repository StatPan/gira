package gira

import (
	"errors"
	"fmt"
	"os"
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

type WorkFinishJiraTransition struct {
	Key       string                  `json:"key,omitempty"`
	Decision  string                  `json:"decision,omitempty"`
	Reason    string                  `json:"reason,omitempty"`
	Candidate JiraTransitionCandidate `json:"candidate,omitempty"`
	Applied   bool                    `json:"applied"`
	DryRun    bool                    `json:"dry_run"`
}

type WorkFinishResult struct {
	Repo           string                    `json:"repo"`
	Issue          int                       `json:"issue"`
	JiraKey        string                    `json:"jira_key,omitempty"`
	DryRun         bool                      `json:"dry_run"`
	Wait           string                    `json:"wait"`
	PRNumber       int                       `json:"pr_number,omitempty"`
	PRURL          string                    `json:"pr_url,omitempty"`
	PRState        string                    `json:"pr_state,omitempty"`
	Merged         bool                      `json:"merged"`
	AlreadyDone    bool                      `json:"already_done"`
	JiraTransition *WorkFinishJiraTransition `json:"jira_transition,omitempty"`
	Actions        []WorkFinishAction        `json:"actions"`
	Blockers       []string                  `json:"blockers"`
	LocalSync      WorkFinishLocalSync       `json:"local_sync"`
	FinalStatus    WorkStatusResult          `json:"final_status"`
	NextStep       string                    `json:"next_step"`
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
	jiraDone, err := inspectJiraDoneTransition(repo, issueNumber, dryRun, runner)
	if err != nil {
		return result, err
	}
	if jiraDone.Enabled {
		result.JiraKey = jiraDone.Key
		result.JiraTransition = jiraDone.Transition
	}
	if status.PRNumber == 0 {
		result.Blockers = append(result.Blockers, "missing_linked_pr")
		result.Blockers = appendUniqueStrings(result.Blockers, jiraDone.Blockers...)
		appendJiraDoneBlockedAction(&result, jiraDone)
		result.NextStep = fmt.Sprintf("gira ticket pr --repo %s --ticket %d --apply", repo.FullName(), issueNumber)
		return finishWithStatus(repo, issueNumber, runner, result, &status, nil)
	}
	if strings.EqualFold(status.State, "MERGED") {
		result.AlreadyDone = true
		result.Merged = true
		result.Actions = append(result.Actions, WorkFinishAction{Action: "pr:merge", Status: "skipped", Detail: "PR is already merged"})
		jiraDone, err = planJiraDoneTransition(repo, dryRun, jiraDone)
		if err != nil {
			return result, err
		}
		if jiraDone.Enabled {
			result.JiraTransition = jiraDone.Transition
			result.Blockers = appendUniqueStrings(result.Blockers, jiraDone.Blockers...)
		}
		if len(result.Blockers) > 0 {
			appendJiraDoneBlockedAction(&result, jiraDone)
			return finishWithStatus(repo, issueNumber, runner, result, &status, nil)
		}
		if jiraDone.ReadyToApply() {
			if dryRun {
				result.Actions = append(result.Actions, plannedOrAppliedAction("jira:done", true, jiraDone.ApplyDetail()))
			} else if err := applyJiraDoneTransition(jiraDone); err != nil {
				return result, err
			} else {
				jiraDone.Transition.Applied = true
				result.Actions = append(result.Actions, plannedOrAppliedAction("jira:done", false, jiraDone.ApplyDetail()))
			}
		} else if jiraDone.AlreadyDone() {
			result.Actions = append(result.Actions, WorkFinishAction{Action: "jira:done", Status: "skipped", Detail: jiraDone.ApplyDetail()})
		}
		return finishWithLocalSync(repo, issueNumber, runner, result, true, &status)
	}

	if containsString(status.Blockers, "draft") {
		result.Actions = append(result.Actions, plannedOrAppliedAction("pr:ready", dryRun, fmt.Sprintf("mark PR #%d ready for review", status.PRNumber)))
		if dryRun {
			result.Blockers = append(result.Blockers, "draft")
			result.Blockers = appendUniqueStrings(result.Blockers, jiraDone.Blockers...)
			appendJiraDoneBlockedAction(&result, jiraDone)
			result.NextStep = fmt.Sprintf("gira ticket finish --repo %s --ticket %d --apply", repo.FullName(), issueNumber)
			return finishWithStatus(repo, issueNumber, runner, result, &status, nil)
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
	result.Blockers = appendUniqueStrings(result.Blockers, jiraDone.Blockers...)
	if len(result.Blockers) > 0 {
		appendJiraDoneBlockedAction(&result, jiraDone)
		result.Actions = append(result.Actions, WorkFinishAction{Action: "pr:merge", Status: "blocked", Detail: strings.Join(result.Blockers, ",")})
		result.NextStep = finishBlockedNextStep(repo, issueNumber, result.Blockers)
		report, reportErr := finishWithStatus(repo, issueNumber, runner, result, &status, nil)
		if dryRun {
			return report, reportErr
		}
		if reportErr != nil {
			return report, reportErr
		}
		return report, fmt.Errorf("ticket finish blocked: %s", strings.Join(result.Blockers, ", "))
	}

	if jiraDone.AlreadyDone() {
		result.Actions = append(result.Actions, WorkFinishAction{Action: "jira:done", Status: "skipped", Detail: jiraDone.ApplyDetail()})
	}
	result.Actions = append(result.Actions, plannedOrAppliedAction("pr:merge", dryRun, fmt.Sprintf("squash merge PR #%d and delete remote branch", status.PRNumber)))
	if dryRun {
		if jiraDone.Enabled {
			result.Blockers = appendUniqueStrings(result.Blockers, "unmerged_pr")
			appendJiraDoneBlockedAction(&result, jiraDone)
			result.NextStep = fmt.Sprintf("gira ticket finish --repo %s --ticket %d --apply", repo.FullName(), issueNumber)
		}
		return finishWithLocalSync(repo, issueNumber, runner, result, true, &status)
	}
	if _, err := runner.Run("gh", "pr", "merge", fmt.Sprintf("%d", status.PRNumber), "--repo", repo.FullName(), "--squash", "--delete-branch"); err != nil {
		return result, fmt.Errorf("merge PR: %w", err)
	}
	result.Merged = true
	jiraDone, err = planJiraDoneTransition(repo, dryRun, jiraDone)
	if err != nil {
		return result, err
	}
	if jiraDone.Enabled {
		result.JiraTransition = jiraDone.Transition
	}
	result.Blockers = appendUniqueStrings(result.Blockers, jiraDone.Blockers...)
	if len(result.Blockers) > 0 {
		appendJiraDoneBlockedAction(&result, jiraDone)
		return finishWithStatus(repo, issueNumber, runner, result, nil, fmt.Errorf("ticket finish blocked: %s", strings.Join(result.Blockers, ", ")))
	}
	if jiraDone.ReadyToApply() {
		if err := applyJiraDoneTransition(jiraDone); err != nil {
			return result, err
		}
		jiraDone.Transition.Applied = true
		result.Actions = append(result.Actions, plannedOrAppliedAction("jira:done", false, jiraDone.ApplyDetail()))
	}
	return finishWithLocalSync(repo, issueNumber, runner, result, true, nil)
}

type jiraDoneTransitionGate struct {
	Enabled    bool
	Key        string
	Transition *WorkFinishJiraTransition
	Blockers   []string
	APIBase    string
}

func (gate jiraDoneTransitionGate) ReadyToApply() bool {
	return gate.Enabled && gate.Transition != nil && gate.Transition.Decision == "direct_transition" && gate.Transition.Candidate.ID != ""
}

func (gate jiraDoneTransitionGate) AlreadyDone() bool {
	return gate.Enabled && gate.Transition != nil && gate.Transition.Decision == "already_at_target"
}

func (gate jiraDoneTransitionGate) ApplyDetail() string {
	if strings.TrimSpace(gate.Key) == "" {
		return "Jira mirror issue is missing Jira-Key metadata"
	}
	if gate.AlreadyDone() {
		return fmt.Sprintf("%s is already in a Done-equivalent Jira status", gate.Key)
	}
	if gate.Transition != nil && gate.Transition.Candidate.ID != "" {
		return fmt.Sprintf("transition %s to done via %s after GitHub merge evidence is clean", gate.Key, gate.Transition.Candidate.ID)
	}
	if gate.Transition != nil && strings.TrimSpace(gate.Transition.Reason) != "" {
		return gate.Transition.Reason
	}
	return "Jira Done transition is not available"
}

func inspectJiraDoneTransition(repo RepoRef, issueNumber int, dryRun bool, runner CommandRunner) (jiraDoneTransitionGate, error) {
	provider, enabled, err := loadJiraFinishProvider(repo)
	if err != nil || !enabled {
		return jiraDoneTransitionGate{}, err
	}
	gate := jiraDoneTransitionGate{Enabled: true, APIBase: provider.BaseURL}
	issue, err := fetchDevIssue(repo, issueNumber, runner)
	if err != nil {
		return gate, err
	}
	key := JiraKeyFromBody(issue.Body)
	gate.Key = key
	if key == "" {
		gate.Blockers = append(gate.Blockers, "missing_mirror_issue")
		gate.Transition = &WorkFinishJiraTransition{
			Decision: "blocked",
			Reason:   "GitHub mirror issue is missing Jira-Key metadata",
			DryRun:   dryRun,
		}
		return gate, nil
	}
	return gate, nil
}

func planJiraDoneTransition(repo RepoRef, dryRun bool, gate jiraDoneTransitionGate) (jiraDoneTransitionGate, error) {
	if !gate.Enabled || strings.TrimSpace(gate.Key) == "" || len(gate.Blockers) > 0 || gate.Transition != nil {
		return gate, nil
	}
	plan, err := BuildJiraTransitionPlan(JiraTransitionPlanInput{
		Repo:   repo,
		Key:    gate.Key,
		Target: "done",
		DryRun: true,
	})
	if err != nil {
		gate.Blockers = append(gate.Blockers, "jira_done_transition")
		gate.Transition = &WorkFinishJiraTransition{
			Key:      gate.Key,
			Decision: "blocked",
			Reason:   err.Error(),
			DryRun:   dryRun,
		}
		return gate, nil
	}
	gate.APIBase = plan.APIBase
	gate.Transition = &WorkFinishJiraTransition{
		Key:       gate.Key,
		Decision:  plan.Decision,
		Reason:    plan.Reason,
		Candidate: plan.Candidate,
		DryRun:    dryRun,
	}
	switch plan.Decision {
	case "direct_transition", "already_at_target":
		return gate, nil
	default:
		gate.Blockers = append(gate.Blockers, "jira_done_transition")
		return gate, nil
	}
}

func loadJiraFinishProvider(repo RepoRef) (JiraProviderConfig, bool, error) {
	entry, err := LoadGlobalRepoRegistryEntry("", repo)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return JiraProviderConfig{}, false, nil
		}
		return JiraProviderConfig{}, false, err
	}
	if entry.Providers == nil || entry.Providers.Jira == nil {
		return JiraProviderConfig{}, false, nil
	}
	provider := *entry.Providers.Jira
	if !provider.Enabled || !strings.EqualFold(provider.Mode, "primary") || !strings.EqualFold(provider.SourceOfTruth.Status, "jira") {
		return JiraProviderConfig{}, false, nil
	}
	return provider, true, nil
}

func applyJiraDoneTransition(gate jiraDoneTransitionGate) error {
	if !gate.ReadyToApply() {
		return nil
	}
	email := strings.TrimSpace(os.Getenv("JIRA_EMAIL"))
	token := strings.TrimSpace(os.Getenv("JIRA_API_TOKEN"))
	return ApplyJiraTransition(gate.APIBase, gate.Key, gate.Transition.Candidate.ID, email, token)
}

func appendJiraDoneBlockedAction(result *WorkFinishResult, gate jiraDoneTransitionGate) {
	if !gate.Enabled || len(result.Blockers) == 0 {
		return
	}
	detail := gate.ApplyDetail()
	if len(gate.Blockers) == 0 {
		detail = "GitHub execution evidence is incomplete: " + strings.Join(result.Blockers, ",")
	}
	result.Actions = append(result.Actions, WorkFinishAction{Action: "jira:done", Status: "blocked", Detail: detail})
}

func finishWithLocalSync(repo RepoRef, issueNumber int, runner CommandRunner, result WorkFinishResult, mergePlanned bool, knownPRStatus *DevPRStatusResult) (WorkFinishResult, error) {
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
	if mergePlanned && len(result.Blockers) == 0 {
		action, err := normalizeFinishedIssueStatus(repo, issueNumber, result.DryRun, runner)
		if err != nil {
			return result, err
		}
		if strings.TrimSpace(action.Action) != "" {
			result.Actions = append(result.Actions, action)
		}
	}
	report, err := finishWithStatus(repo, issueNumber, runner, result, knownPRStatus, nil)
	if report.DryRun && mergePlanned && !report.AlreadyDone && len(report.Blockers) == 0 {
		report.NextStep = fmt.Sprintf("gira ticket finish --repo %s --ticket %d --apply", repo.FullName(), issueNumber)
	}
	return report, err
}

func normalizeFinishedIssueStatus(repo RepoRef, issueNumber int, dryRun bool, runner CommandRunner) (WorkFinishAction, error) {
	issue, err := fetchDevIssue(repo, issueNumber, runner)
	if err != nil {
		return WorkFinishAction{}, fmt.Errorf("normalize finished issue status: %w", err)
	}
	if !strings.EqualFold(issue.State, "closed") {
		return WorkFinishAction{}, nil
	}
	removeLabels := activeStatusLabels(issue.Labels)
	if len(removeLabels) == 0 {
		return WorkFinishAction{}, nil
	}
	addLabels := []string{}
	statusDoneExists, err := repoHasLabel(repo, "status:done", runner)
	if err != nil {
		return WorkFinishAction{}, fmt.Errorf("normalize finished issue status: %w", err)
	}
	if statusDoneExists && !hasLabel(issue.Labels, "status:done") {
		addLabels = append(addLabels, "status:done")
	}
	action := WorkFinishAction{
		Action: "ticket:normalize-status",
		Status: plannedOrAppliedStatus(dryRun),
		Detail: finishStatusNormalizeDetail(addLabels, removeLabels),
	}
	if dryRun {
		return action, nil
	}
	if err := applyFinishedIssueStatusLabels(repo, issueNumber, addLabels, removeLabels, runner); err != nil {
		return action, err
	}
	return action, nil
}

func finishStatusNormalizeDetail(addLabels []string, removeLabels []string) string {
	parts := []string{}
	if len(addLabels) > 0 {
		parts = append(parts, "add="+strings.Join(addLabels, ","))
	}
	if len(removeLabels) > 0 {
		parts = append(parts, "remove="+strings.Join(removeLabels, ","))
	}
	return strings.Join(parts, " ")
}

func applyFinishedIssueStatusLabels(repo RepoRef, issueNumber int, addLabels []string, removeLabels []string, runner CommandRunner) error {
	args := []string{"issue", "edit", fmt.Sprintf("%d", issueNumber), "--repo", repo.FullName()}
	for _, label := range addLabels {
		args = append(args, "--add-label", label)
	}
	for _, label := range removeLabels {
		args = append(args, "--remove-label", label)
	}
	_, err := runner.Run("gh", args...)
	return err
}

func finishWithStatus(repo RepoRef, issueNumber int, runner CommandRunner, result WorkFinishResult, knownPRStatus *DevPRStatusResult, err error) (WorkFinishResult, error) {
	var status WorkStatusResult
	var statusErr error
	if knownPRStatus != nil {
		status, statusErr = GetWorkStatusWithPRStatus(repo, issueNumber, *knownPRStatus, runner)
	} else {
		status, statusErr = GetWorkStatus(repo, issueNumber, runner)
	}
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

func appendUniqueStrings(values []string, additions ...string) []string {
	for _, addition := range additions {
		if strings.TrimSpace(addition) == "" || containsString(values, addition) {
			continue
		}
		values = append(values, addition)
	}
	return values
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
