package gira

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const WorkFinishResultSchemaVersion = "work-finish-result/v1"

type WorkFinishAction struct {
	Action string `json:"action"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type WorkFinishLocalSync struct {
	Attempted    bool   `json:"attempted"`
	Skipped      bool   `json:"skipped"`
	Reason       string `json:"reason,omitempty"`
	Branch       string `json:"branch,omitempty"`
	TargetBranch string `json:"target_branch,omitempty"`
}

type WorkFinishReadinessReport struct {
	SchemaVersion      string                         `json:"schema_version"`
	Repository         string                         `json:"repository"`
	Issue              WorkFinishReadinessIssue       `json:"issue"`
	PullRequest        WorkFinishReadinessPullRequest `json:"pull_request"`
	Checks             WorkFinishReadinessChecks      `json:"checks"`
	Review             WorkFinishReadinessReview      `json:"review"`
	Evidence           WorkFinishReadinessEvidence    `json:"evidence"`
	LabelState         WorkFinishReadinessLabelState  `json:"label_state"`
	AcceptanceCriteria *TicketStatusAcceptance        `json:"acceptance_criteria,omitempty"`
	ClosingReference   WorkFinishClosingReference     `json:"closing_reference"`
	Ready              bool                           `json:"ready"`
	Blockers           []string                       `json:"blockers"`
	NextAction         string                         `json:"next_action"`
	NextStep           string                         `json:"next_step"`
	Warnings           []string                       `json:"warnings,omitempty"`
}

type WorkFinishReadinessIssue struct {
	Number    int    `json:"number"`
	Title     string `json:"title,omitempty"`
	State     string `json:"state,omitempty"`
	Status    string `json:"status,omitempty"`
	Milestone string `json:"milestone,omitempty"`
}

type WorkFinishReadinessPullRequest struct {
	Available      bool   `json:"available"`
	Number         int    `json:"number,omitempty"`
	URL            string `json:"url,omitempty"`
	State          string `json:"state,omitempty"`
	Mergeable      string `json:"mergeable,omitempty"`
	ReviewDecision string `json:"review_decision,omitempty"`
	IsDraft        bool   `json:"is_draft,omitempty"`
	HeadRefName    string `json:"head_ref_name,omitempty"`
	BaseRefName    string `json:"base_ref_name,omitempty"`
}

type WorkFinishReadinessChecks struct {
	Status  string `json:"status"`
	Total   int    `json:"total"`
	Passing int    `json:"passing"`
	Pending int    `json:"pending"`
	Failing int    `json:"failing"`
	Missing bool   `json:"missing"`
}

type WorkFinishReadinessReview struct {
	Status   string `json:"status"`
	Decision string `json:"decision,omitempty"`
}

type WorkFinishReadinessEvidence struct {
	ClosingReference bool     `json:"closing_reference"`
	BranchTrusted    bool     `json:"branch_trusted"`
	FinishReady      bool     `json:"finish_ready"`
	Sources          []string `json:"sources"`
}

type WorkFinishReadinessLabelState struct {
	Status             string   `json:"status,omitempty"`
	Labels             []string `json:"labels,omitempty"`
	ActiveStatusLabels []string `json:"active_status_labels,omitempty"`
}

type WorkFinishClosingReference struct {
	Present bool   `json:"present"`
	Source  string `json:"source"`
}

type WorkFinishReceipt struct {
	SchemaVersion    string                      `json:"schema_version"`
	FinishedAt       string                      `json:"finished_at"`
	Repository       string                      `json:"repository"`
	Issue            WorkFinishReadinessIssue    `json:"issue"`
	PullRequest      WorkFinishReceiptPR         `json:"pull_request"`
	ChecksSummary    WorkFinishReadinessChecks   `json:"checks_summary"`
	ReviewSummary    WorkFinishReadinessReview   `json:"review_summary"`
	EvidenceSummary  WorkFinishReadinessEvidence `json:"evidence_summary"`
	TelemetrySummary *TicketStatusTelemetry      `json:"telemetry_summary,omitempty"`
	LabelChanges     []string                    `json:"label_changes"`
	FinalState       WorkFinishReceiptFinalState `json:"final_state"`
	Warnings         []string                    `json:"warnings,omitempty"`
	Target           string                      `json:"target"`
	RenderedBody     string                      `json:"rendered_body"`
}

type WorkFinishReceiptPR struct {
	Number int    `json:"number,omitempty"`
	URL    string `json:"url,omitempty"`
	State  string `json:"state,omitempty"`
	Merged bool   `json:"merged"`
}

type WorkFinishReceiptFinalState struct {
	IssueState string `json:"issue_state,omitempty"`
	Status     string `json:"status,omitempty"`
	NextAction string `json:"next_action,omitempty"`
	NextStep   string `json:"next_step,omitempty"`
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
	SchemaVersion    string                    `json:"schema_version,omitempty"`
	Repo             string                    `json:"repo"`
	Issue            int                       `json:"issue"`
	JiraKey          string                    `json:"jira_key,omitempty"`
	DryRun           bool                      `json:"dry_run"`
	Wait             string                    `json:"wait"`
	SyncLocal        bool                      `json:"sync_local,omitempty"`
	PRLookupAttempts int                       `json:"pr_lookup_attempts,omitempty"`
	PRNumber         int                       `json:"pr_number,omitempty"`
	PRURL            string                    `json:"pr_url,omitempty"`
	PRState          string                    `json:"pr_state,omitempty"`
	Merged           bool                      `json:"merged"`
	AlreadyDone      bool                      `json:"already_done"`
	JiraTransition   *WorkFinishJiraTransition `json:"jira_transition,omitempty"`
	Actions          []WorkFinishAction        `json:"actions"`
	Blockers         []string                  `json:"blockers"`
	Warnings         []string                  `json:"warnings,omitempty"`
	LocalSync        WorkFinishLocalSync       `json:"local_sync"`
	FinalStatus      WorkStatusResult          `json:"final_status"`
	Readiness        WorkFinishReadinessReport `json:"readiness"`
	Receipt          WorkFinishReceipt         `json:"receipt"`
	NextStep         string                    `json:"next_step"`
	Approval         *ApprovalEvidence         `json:"approval,omitempty"`
}

type WorkFinishOptions struct {
	SyncLocal bool `json:"sync_local"`
}

var finishMissingPRRetryAttempts = 3
var finishMissingPRRetryDelay = time.Second
var finishReceiptNow = func() time.Time { return time.Now().UTC() }

func EnsureWorkFinishResultSchema(result *WorkFinishResult) {
	if result != nil && strings.TrimSpace(result.SchemaVersion) == "" {
		result.SchemaVersion = WorkFinishResultSchemaVersion
	}
}

func FinishWork(repo RepoRef, issueNumber int, dryRun bool, wait time.Duration, runner CommandRunner) (WorkFinishResult, error) {
	return FinishWorkWithOptions(repo, issueNumber, dryRun, wait, WorkFinishOptions{}, runner)
}

func FinishWorkWithOptions(repo RepoRef, issueNumber int, dryRun bool, wait time.Duration, options WorkFinishOptions, runner CommandRunner) (WorkFinishResult, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if issueNumber <= 0 {
		return WorkFinishResult{}, fmt.Errorf("ticket must be > 0")
	}
	result := WorkFinishResult{
		SchemaVersion: WorkFinishResultSchemaVersion,
		Repo:          repo.FullName(),
		Issue:         issueNumber,
		DryRun:        dryRun,
		Wait:          wait.String(),
		SyncLocal:     options.SyncLocal,
		Actions:       []WorkFinishAction{},
		Blockers:      []string{},
		Warnings:      []string{},
		NextStep:      fmt.Sprintf("gira ticket status --repo %s --ticket %d", repo.FullName(), issueNumber),
	}

	var status DevPRStatusResult
	var err error
	if dryRun {
		status, err = DevPRStatus(repo, issueNumber, runner)
	} else {
		status, err = DevPRStatusWithMissingPRRetry(repo, issueNumber, runner, finishMissingPRRetryAttempts, finishMissingPRRetryDelay)
	}
	if err != nil {
		return result, err
	}
	result.PRLookupAttempts = status.LookupAttempts
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
		return finishWithLocalSync(repo, issueNumber, runner, result, true, &status, &status, options)
	}

	if containsString(status.Blockers, "draft") {
		result.Actions = append(result.Actions, WorkFinishAction{Action: "finish:intent", Status: "observed", Detail: "terminal finish requested while the current lifecycle recommendation is mark_pr_ready"})
		result.Actions = append(result.Actions, plannedOrAppliedAction("pr:ready", dryRun, fmt.Sprintf("mark PR #%d ready for review", status.PRNumber)))
		result.Warnings = append(result.Warnings, "terminal finish requested for a Draft PR; this invocation stops after marking it ready and requires a new dry-run before merge")
		if dryRun {
			result.Blockers = mergeBlockers(status.Blockers)
			result.Blockers = appendUniqueStrings(result.Blockers, jiraDone.Blockers...)
			appendJiraDoneBlockedAction(&result, jiraDone)
			result.NextStep = fmt.Sprintf("gira ticket finish --repo %s --ticket %d --apply; then rerun --dry-run before merge", repo.FullName(), issueNumber)
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
		result.Blockers = mergeBlockers(status.Blockers)
		result.LocalSync = WorkFinishLocalSync{Skipped: true, Reason: "ready_transition_only"}
		nextStep := fmt.Sprintf("gira ticket finish --repo %s --ticket %d --dry-run", repo.FullName(), issueNumber)
		result.NextStep = nextStep
		report, reportErr := finishWithStatus(repo, issueNumber, runner, result, &status, nil)
		report.NextStep = nextStep
		report.Readiness.NextStep = nextStep
		report.Receipt.FinalState.NextStep = nextStep
		return report, reportErr
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
	result.Warnings = append(result.Warnings, fmt.Sprintf("IRREVERSIBLE: ticket finish --apply will squash merge PR #%d and delete its remote branch", status.PRNumber))
	if dryRun {
		if jiraDone.Enabled {
			result.Blockers = appendUniqueStrings(result.Blockers, "unmerged_pr")
			appendJiraDoneBlockedAction(&result, jiraDone)
			result.NextStep = fmt.Sprintf("gira ticket finish --repo %s --ticket %d --apply", repo.FullName(), issueNumber)
		}
		return finishWithLocalSync(repo, issueNumber, runner, result, true, &status, &status, options)
	}
	if err := finishMergePR(repo, status, runner, &result); err != nil {
		return result, err
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
	return finishWithLocalSync(repo, issueNumber, runner, result, true, nil, &status, options)
}

func finishMergePR(repo RepoRef, status DevPRStatusResult, runner CommandRunner, result *WorkFinishResult) error {
	if _, err := runner.Run("gh", "pr", "merge", fmt.Sprintf("%d", status.PRNumber), "--repo", repo.FullName(), "--squash", "--delete-branch"); err != nil {
		if !finishGraphQLRateLimitError(err) {
			return fmt.Errorf("merge PR: %w", err)
		}
		diagnostic := finishMergeRateLimitDiagnostic(repo, runner)
		fallbackDetail, fallbackErr := finishMergePRViaREST(repo, status.PRNumber, runner)
		if fallbackErr != nil {
			if result != nil {
				result.Actions = append(result.Actions, WorkFinishAction{Action: "pr:merge_fallback", Status: "blocked", Detail: strings.TrimSpace("GraphQL merge rate limit; " + diagnostic + "; " + fallbackErr.Error())})
			}
			return fmt.Errorf("merge PR: GraphQL rate limit; %s; REST fallback failed: %w", diagnostic, fallbackErr)
		}
		if result != nil {
			result.Actions = append(result.Actions, WorkFinishAction{Action: "pr:merge_fallback", Status: "applied", Detail: strings.TrimSpace(fallbackDetail + "; " + diagnostic)})
		}
	}
	return nil
}

func finishGraphQLRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "graphql") && (strings.Contains(message, "rate limit") || strings.Contains(message, "rate_limit") || strings.Contains(message, "api rate"))
}

func finishMergeRateLimitDiagnostic(repo RepoRef, runner CommandRunner) string {
	report, err := BuildAPILimitReport(repo, runner, time.Now().UTC())
	if err != nil {
		return "visible API budget unavailable: " + err.Error()
	}
	return fmt.Sprintf(
		"visible API budget core_remaining=%d/%d graphql_remaining=%d/%d",
		report.Core.Remaining,
		report.Core.Limit,
		report.GraphQL.Remaining,
		report.GraphQL.Limit,
	)
}

func finishMergePRViaREST(repo RepoRef, prNumber int, runner CommandRunner) (string, error) {
	output, err := runner.Run("gh", "api", fmt.Sprintf("repos/%s/pulls/%d", repo.FullName(), prNumber))
	if err != nil {
		return "", fmt.Errorf("inspect PR via REST: %w", err)
	}
	var pr struct {
		State          string `json:"state"`
		Mergeable      *bool  `json:"mergeable"`
		MergeableState string `json:"mergeable_state"`
		Head           struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := json.Unmarshal(output, &pr); err != nil {
		return "", fmt.Errorf("parse REST PR JSON: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(pr.State), "open") {
		return "", fmt.Errorf("PR #%d state is %q, want open", prNumber, pr.State)
	}
	if pr.Mergeable == nil {
		return "", fmt.Errorf("PR #%d REST mergeable value is unavailable", prNumber)
	}
	if !*pr.Mergeable {
		return "", fmt.Errorf("PR #%d is not REST mergeable", prNumber)
	}
	if !strings.EqualFold(strings.TrimSpace(pr.MergeableState), "clean") {
		return "", fmt.Errorf("PR #%d mergeable_state is %q, want clean", prNumber, pr.MergeableState)
	}
	headSHA := strings.TrimSpace(pr.Head.SHA)
	if headSHA == "" {
		return "", fmt.Errorf("PR #%d REST head SHA is empty", prNumber)
	}
	if _, err := runner.Run("gh", "api", "-X", "PUT", fmt.Sprintf("repos/%s/pulls/%d/merge", repo.FullName(), prNumber), "-f", "merge_method=squash", "-f", "sha="+headSHA); err != nil {
		return "", fmt.Errorf("REST squash merge PR #%d with expected_head_sha=%s: %w", prNumber, headSHA, err)
	}
	return fmt.Sprintf("REST squash merge PR #%d with expected_head_sha=%s after clean mergeable REST check", prNumber, headSHA), nil
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

func finishWithLocalSync(repo RepoRef, issueNumber int, runner CommandRunner, result WorkFinishResult, mergePlanned bool, knownPRStatus *DevPRStatusResult, localSyncPRStatus *DevPRStatusResult, options WorkFinishOptions) (WorkFinishResult, error) {
	local, actions := planLocalBaseSync(repo, runner, result.DryRun, localSyncPRStatus, options.SyncLocal)
	result.LocalSync = local
	result.Actions = append(result.Actions, actions...)
	if !result.DryRun && mergePlanned && local.Attempted && !local.Skipped {
		if _, err := runner.Run("git", "checkout", local.TargetBranch); err != nil {
			return result, fmt.Errorf("checkout %s: %w", local.TargetBranch, err)
		}
		if _, err := runner.Run("git", "pull", "--ff-only", "origin", local.TargetBranch); err != nil {
			return result, fmt.Errorf("pull %s: %w", local.TargetBranch, err)
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
	result.Readiness = buildWorkFinishReadiness(result)
	result.Receipt = buildWorkFinishReceipt(result)
	if receiptAction, receiptErr := maybeApplyWorkFinishReceipt(repo, runner, &result, err); receiptAction.Action != "" {
		result.Actions = append(result.Actions, receiptAction)
		if receiptErr != nil {
			return result, receiptErr
		}
	}
	result.Actions = append(result.Actions, WorkFinishAction{Action: "projects:sync", Status: "planned", Detail: "gira projects sync --dry-run"})
	EnsureWorkFinishResultSchema(&result)
	if result.DryRun {
		result.Approval = WorkFinishApprovalEvidence(result)
	}
	return result, err
}

func buildWorkFinishReadiness(result WorkFinishResult) WorkFinishReadinessReport {
	status := result.FinalStatus
	report := WorkFinishReadinessReport{
		SchemaVersion: "finish-readiness/v1",
		Repository:    firstNonEmpty(status.Repo, result.Repo),
		Issue: WorkFinishReadinessIssue{
			Number:    firstPositive(status.Issue, result.Issue),
			Title:     status.Title,
			State:     status.State,
			Status:    status.Status,
			Milestone: status.Milestone,
		},
		Checks: WorkFinishReadinessChecks{
			Status:  firstNonEmpty(status.ChecksStatus, "missing"),
			Total:   len(status.Checks),
			Missing: len(status.Checks) == 0,
		},
		Review: WorkFinishReadinessReview{
			Status: firstNonEmpty(status.ReviewStatus, "missing"),
		},
		LabelState: WorkFinishReadinessLabelState{
			Status:             status.Status,
			Labels:             append([]string(nil), status.Labels...),
			ActiveStatusLabels: activeStatusLabels(status.Labels),
		},
		AcceptanceCriteria: status.Acceptance,
		NextAction:         status.NextAction,
		NextStep:           firstNonEmpty(result.NextStep, status.NextStep),
		Warnings:           append([]string(nil), status.Warnings...),
	}
	if status.PullRequest != nil {
		report.PullRequest = WorkFinishReadinessPullRequest{
			Available:      status.PullRequest.Available,
			Number:         status.PullRequest.Number,
			URL:            status.PullRequest.URL,
			State:          status.PullRequest.State,
			Mergeable:      status.PullRequest.Mergeable,
			ReviewDecision: status.PullRequest.ReviewDecision,
			IsDraft:        status.PullRequest.IsDraft,
			HeadRefName:    status.PullRequest.HeadRefName,
			BaseRefName:    status.PullRequest.BaseRefName,
		}
	} else if result.PRNumber > 0 {
		report.PullRequest = WorkFinishReadinessPullRequest{
			Available: true,
			Number:    result.PRNumber,
			URL:       result.PRURL,
			State:     result.PRState,
		}
	}
	if status.Evidence != nil {
		report.Evidence = WorkFinishReadinessEvidence{
			ClosingReference: status.Evidence.ClosingReference,
			BranchTrusted:    status.Evidence.BranchTrusted,
			FinishReady:      status.Evidence.FinishReady,
			Sources:          append([]string(nil), status.Evidence.Sources...),
		}
	}
	report.ClosingReference = WorkFinishClosingReference{
		Present: report.Evidence.ClosingReference,
		Source:  closingReferenceSource(report.Evidence.ClosingReference),
	}
	report.Checks.Passing, report.Checks.Pending, report.Checks.Failing = finishCheckCounts(status.Checks)
	report.Review.Decision = report.PullRequest.ReviewDecision
	report.Blockers = finishReadinessBlockers(result, report)
	report.Ready = len(report.Blockers) == 0 && finishEvidenceReady(report, status.NextAction, result)
	if !report.Ready && report.NextAction == "" {
		report.NextAction = "resolve_finish_blockers"
	}
	return report
}

func finishCheckCounts(checks []DevPRCheck) (passing int, pending int, failing int) {
	for _, check := range checks {
		switch check.State {
		case "passing":
			passing++
		case "pending":
			pending++
		case "failing":
			failing++
		}
	}
	return passing, pending, failing
}

func finishReadinessBlockers(result WorkFinishResult, report WorkFinishReadinessReport) []string {
	blockers := make([]string, 0, len(result.Blockers))
	blockers = append(blockers, result.Blockers...)
	if !report.PullRequest.Available {
		blockers = appendUniqueStrings(blockers, "missing_linked_pr")
	}
	if report.Checks.Status == "failed" {
		blockers = appendUniqueStrings(blockers, "checks")
	}
	if report.Checks.Status == "pending" {
		blockers = appendUniqueStrings(blockers, "checks_pending")
	}
	if report.PullRequest.IsDraft {
		blockers = appendUniqueStrings(blockers, "draft")
	}
	if report.Review.Status == "blocked" {
		blockers = appendUniqueStrings(blockers, "review")
	}
	return blockers
}

func finishEvidenceReady(report WorkFinishReadinessReport, nextAction string, result WorkFinishResult) bool {
	if result.AlreadyDone {
		return true
	}
	if nextAction == "done" || nextAction == "merge_when_policy_allows" {
		return report.ClosingReference.Present && report.PullRequest.Available
	}
	return false
}

func closingReferenceSource(present bool) string {
	if present {
		return "linked_pull_request_body"
	}
	return "missing"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func buildWorkFinishReceipt(result WorkFinishResult) WorkFinishReceipt {
	readiness := result.Readiness
	receipt := WorkFinishReceipt{
		SchemaVersion:    "finish-receipt/v1",
		FinishedAt:       finishReceiptNow().Format(time.RFC3339),
		Repository:       readiness.Repository,
		Issue:            readiness.Issue,
		PullRequest:      WorkFinishReceiptPR{Number: readiness.PullRequest.Number, URL: readiness.PullRequest.URL, State: readiness.PullRequest.State, Merged: result.Merged || result.AlreadyDone || strings.EqualFold(readiness.PullRequest.State, "MERGED")},
		ChecksSummary:    readiness.Checks,
		ReviewSummary:    readiness.Review,
		EvidenceSummary:  readiness.Evidence,
		TelemetrySummary: result.FinalStatus.Telemetry,
		LabelChanges:     finishReceiptLabelChanges(result.Actions),
		FinalState: WorkFinishReceiptFinalState{
			IssueState: readiness.Issue.State,
			Status:     readiness.Issue.Status,
			NextAction: readiness.NextAction,
			NextStep:   readiness.NextStep,
		},
		Warnings: append([]string(nil), readiness.Warnings...),
		Target:   fmt.Sprintf("issue#%d", result.Issue),
	}
	if receipt.TelemetrySummary != nil {
		receipt.Warnings = appendUniqueStrings(receipt.Warnings, receipt.TelemetrySummary.Warnings...)
	}
	receipt.RenderedBody = renderWorkFinishReceipt(receipt)
	return receipt
}

func finishReceiptLabelChanges(actions []WorkFinishAction) []string {
	changes := []string{}
	for _, action := range actions {
		if action.Action != "ticket:normalize-status" || strings.TrimSpace(action.Detail) == "" {
			continue
		}
		changes = append(changes, action.Detail)
	}
	if len(changes) == 0 {
		return []string{}
	}
	return changes
}

func renderWorkFinishReceipt(receipt WorkFinishReceipt) string {
	pr := "none"
	if receipt.PullRequest.Number > 0 {
		pr = fmt.Sprintf("#%d", receipt.PullRequest.Number)
	}
	evidence := strings.Join(receipt.EvidenceSummary.Sources, ",")
	if evidence == "" {
		evidence = "none"
	}
	warnings := strings.Join(receipt.Warnings, ",")
	if warnings == "" {
		warnings = "none"
	}
	labels := strings.Join(receipt.LabelChanges, "; ")
	if labels == "" {
		labels = "none"
	}
	telemetry := "unknown"
	if receipt.TelemetrySummary != nil {
		telemetry = receipt.TelemetrySummary.Status
	}
	var b strings.Builder
	b.WriteString("## Finish Receipt\n\n")
	fmt.Fprintf(&b, "- Finished at: %s\n", receipt.FinishedAt)
	fmt.Fprintf(&b, "- Ticket: #%d status=%s state=%s\n", receipt.Issue.Number, valueOrUnknown(receipt.Issue.Status), valueOrUnknown(receipt.Issue.State))
	fmt.Fprintf(&b, "- Linked PR: %s state=%s merged=%t\n", pr, valueOrUnknown(receipt.PullRequest.State), receipt.PullRequest.Merged)
	fmt.Fprintf(&b, "- Checks: %s total=%d passing=%d pending=%d failing=%d\n", valueOrUnknown(receipt.ChecksSummary.Status), receipt.ChecksSummary.Total, receipt.ChecksSummary.Passing, receipt.ChecksSummary.Pending, receipt.ChecksSummary.Failing)
	fmt.Fprintf(&b, "- Review: %s\n", valueOrUnknown(receipt.ReviewSummary.Status))
	fmt.Fprintf(&b, "- Evidence: %s\n", evidence)
	fmt.Fprintf(&b, "- AI Delivery Telemetry: %s\n", telemetry)
	fmt.Fprintf(&b, "- Label changes: %s\n", labels)
	fmt.Fprintf(&b, "- Warnings: %s\n", warnings)
	fmt.Fprintf(&b, "- Next: %s\n", valueOrUnknown(receipt.FinalState.NextStep))
	return b.String()
}

func maybeApplyWorkFinishReceipt(repo RepoRef, runner CommandRunner, result *WorkFinishResult, finishErr error) (WorkFinishAction, error) {
	if result.Receipt.SchemaVersion == "" {
		return WorkFinishAction{}, nil
	}
	if len(result.Blockers) > 0 || finishErr != nil {
		return WorkFinishAction{Action: "finish:receipt", Status: "blocked", Detail: "finish evidence is incomplete"}, nil
	}
	if result.DryRun {
		return WorkFinishAction{Action: "finish:receipt", Status: "planned", Detail: "post concise finish receipt to issue"}, nil
	}
	if _, err := runner.Run("gh", "issue", "comment", fmt.Sprintf("%d", result.Issue), "--repo", repo.FullName(), "--body", result.Receipt.RenderedBody); err != nil {
		return WorkFinishAction{Action: "finish:receipt", Status: "failed", Detail: "post concise finish receipt to issue"}, fmt.Errorf("post finish receipt: %w", err)
	}
	return WorkFinishAction{Action: "finish:receipt", Status: "applied", Detail: "posted concise finish receipt to issue"}, nil
}

func planLocalBaseSync(repo RepoRef, runner CommandRunner, dryRun bool, knownPRStatus *DevPRStatusResult, syncLocal bool) (WorkFinishLocalSync, []WorkFinishAction) {
	local := WorkFinishLocalSync{}
	actions := []WorkFinishAction{}
	if !syncLocal {
		local.Skipped = true
		local.Reason = "local_sync_disabled"
		actions = append(actions, WorkFinishAction{Action: "local:sync_base", Status: "skipped", Detail: local.Reason})
		return local, actions
	}
	targetBranch := ""
	if knownPRStatus != nil {
		targetBranch = strings.TrimSpace(knownPRStatus.Binding.BaseRef)
	}
	if targetBranch == "" {
		local.Skipped = true
		local.Reason = "missing_pr_base"
		actions = append(actions, WorkFinishAction{Action: "local:sync_base", Status: "skipped", Detail: local.Reason})
		return local, actions
	}
	if err := validateGitBranchPushName(targetBranch); err != nil {
		local.Skipped = true
		local.Reason = "invalid_pr_base"
		local.TargetBranch = targetBranch
		actions = append(actions, WorkFinishAction{Action: "local:sync_base", Status: "skipped", Detail: local.Reason})
		return local, actions
	}
	local.TargetBranch = targetBranch
	remoteOut, err := runner.Run("git", "remote", "get-url", "origin")
	if err != nil {
		local.Skipped = true
		local.Reason = "not_git_checkout"
		actions = append(actions, WorkFinishAction{Action: "local:sync_base", Status: "skipped", Detail: local.Reason})
		return local, actions
	}
	currentRepo, err := ParseGitHubRemoteRepo(strings.TrimSpace(string(remoteOut)))
	if err != nil || !strings.EqualFold(currentRepo.FullName(), repo.FullName()) {
		local.Skipped = true
		local.Reason = "checkout_repo_mismatch"
		actions = append(actions, WorkFinishAction{Action: "local:sync_base", Status: "skipped", Detail: local.Reason})
		return local, actions
	}
	branchOut, err := runner.Run("git", "branch", "--show-current")
	if err != nil {
		local.Skipped = true
		local.Reason = "current_branch_unavailable"
		actions = append(actions, WorkFinishAction{Action: "local:sync_base", Status: "skipped", Detail: local.Reason})
		return local, actions
	}
	local.Branch = strings.TrimSpace(string(branchOut))
	statusOut, err := runner.Run("git", "status", "--porcelain")
	if err != nil {
		local.Skipped = true
		local.Reason = "worktree_status_unavailable"
		actions = append(actions, WorkFinishAction{Action: "local:sync_base", Status: "skipped", Detail: local.Reason})
		return local, actions
	}
	if strings.TrimSpace(string(statusOut)) != "" {
		local.Skipped = true
		local.Reason = "dirty_worktree"
		actions = append(actions, WorkFinishAction{Action: "local:sync_base", Status: "skipped", Detail: local.Reason})
		return local, actions
	}
	local.Attempted = true
	actions = append(actions, plannedOrAppliedAction("local:sync_base", dryRun, "checkout "+targetBranch+" and pull --ff-only"))
	return local, actions
}

func mergeBlockers(blockers []string) []string {
	result := make([]string, 0)
	for _, blocker := range blockers {
		switch blocker {
		case "missing_linked_pr", "draft", "review", "checks", "checks_pending", "pr_binding":
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
	readiness := "unknown"
	if result.Readiness.SchemaVersion != "" {
		readiness = "blocked"
		if result.Readiness.Ready {
			readiness = "ready"
		}
	}
	actions := make([]string, 0, len(result.Actions))
	for _, action := range result.Actions {
		actions = append(actions, action.Action+":"+action.Status)
	}
	if len(actions) == 0 {
		actions = append(actions, "none")
	}
	output := fmt.Sprintf(
		"work finish: issue #%d pr=%d merged=%t readiness=%s blockers=%s actions=%s\nnext step: %s\n",
		result.Issue,
		result.PRNumber,
		result.Merged,
		readiness,
		blockers,
		strings.Join(actions, ","),
		result.NextStep,
	)
	for _, warning := range result.Warnings {
		output = "WARNING: " + warning + "\n" + output
	}
	return output
}
