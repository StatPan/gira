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
	ReleaseImpact      *TicketReleaseImpact           `json:"release_impact,omitempty"`
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
	Available        bool   `json:"available"`
	Number           int    `json:"number,omitempty"`
	URL              string `json:"url,omitempty"`
	State            string `json:"state,omitempty"`
	Mergeable        string `json:"mergeable,omitempty"`
	ReviewDecision   string `json:"review_decision,omitempty"`
	IsDraft          bool   `json:"is_draft,omitempty"`
	HeadRefName      string `json:"head_ref_name,omitempty"`
	BaseRefName      string `json:"base_ref_name,omitempty"`
	HeadSHA          string `json:"head_sha,omitempty"`
	BaseSHA          string `json:"base_sha,omitempty"`
	MergeCommitSHA   string `json:"merge_commit_sha,omitempty"`
	ClosingReference bool   `json:"closing_reference"`
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
	Status   string               `json:"status"`
	Decision string               `json:"decision,omitempty"`
	Policy   FinishReviewPolicy   `json:"policy"`
	Evidence FinishReviewEvidence `json:"evidence"`
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
	SchemaVersion      string                      `json:"schema_version"`
	FinishedAt         string                      `json:"finished_at"`
	Repository         string                      `json:"repository"`
	Issue              WorkFinishReadinessIssue    `json:"issue"`
	PullRequest        WorkFinishReceiptPR         `json:"pull_request"`
	HeadConstraint     *WorkFinishHeadConstraint   `json:"head_constraint,omitempty"`
	MergeRequestStatus string                      `json:"merge_request_status,omitempty"`
	ChecksSummary      WorkFinishReadinessChecks   `json:"checks_summary"`
	ReviewSummary      WorkFinishReadinessReview   `json:"review_summary"`
	EvidenceSummary    WorkFinishReadinessEvidence `json:"evidence_summary"`
	TelemetrySummary   *TicketStatusTelemetry      `json:"telemetry_summary,omitempty"`
	LabelChanges       []string                    `json:"label_changes"`
	FinalState         WorkFinishReceiptFinalState `json:"final_state"`
	Warnings           []string                    `json:"warnings,omitempty"`
	Target             string                      `json:"target"`
	RenderedBody       string                      `json:"rendered_body"`
}

// WorkFinishHeadConstraint records the immutable PR-head identity supplied by
// a caller or captured from recorded review evidence, along with every native
// observation and backend pin used during finish.
type WorkFinishHeadConstraint struct {
	ExpectedHeadSHA          string `json:"expected_head_sha,omitempty"`
	ExpectedSource           string `json:"expected_source,omitempty"`
	ObservedInitialHeadSHA   string `json:"observed_initial_head_sha,omitempty"`
	ObservedPreMergeHeadSHA  string `json:"observed_pre_merge_head_sha,omitempty"`
	ObservedAfterReadySHA    string `json:"observed_after_ready_sha,omitempty"`
	ObservedPostMergeHeadSHA string `json:"observed_post_merge_head_sha,omitempty"`
	PinMechanism             string `json:"pin_mechanism,omitempty"`
	State                    string `json:"state,omitempty"`
	MismatchReason           string `json:"mismatch_reason,omitempty"`
}

type WorkFinishReceiptPR struct {
	Number           int    `json:"number,omitempty"`
	URL              string `json:"url,omitempty"`
	State            string `json:"state,omitempty"`
	Merged           bool   `json:"merged"`
	HeadSHA          string `json:"head_sha,omitempty"`
	BaseSHA          string `json:"base_sha,omitempty"`
	MergeCommitSHA   string `json:"merge_commit_sha,omitempty"`
	ClosingReference bool   `json:"closing_reference"`
}

type WorkFinishReceiptFinalState struct {
	IssueState       string `json:"issue_state,omitempty"`
	Status           string `json:"status,omitempty"`
	GitHubIssueState string `json:"github_issue_state,omitempty"`
	GiraStatus       string `json:"gira_status,omitempty"`
	NextAction       string `json:"next_action,omitempty"`
	NextStep         string `json:"next_step,omitempty"`
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
	SchemaVersion      string                    `json:"schema_version,omitempty"`
	Repo               string                    `json:"repo"`
	Issue              int                       `json:"issue"`
	JiraKey            string                    `json:"jira_key,omitempty"`
	DryRun             bool                      `json:"dry_run"`
	Wait               string                    `json:"wait"`
	SyncLocal          bool                      `json:"sync_local,omitempty"`
	PRLookupAttempts   int                       `json:"pr_lookup_attempts,omitempty"`
	PRNumber           int                       `json:"pr_number,omitempty"`
	PRURL              string                    `json:"pr_url,omitempty"`
	PRState            string                    `json:"pr_state,omitempty"`
	Merged             bool                      `json:"merged"`
	MergeRequestStatus string                    `json:"merge_request_status,omitempty"`
	AlreadyDone        bool                      `json:"already_done"`
	JiraTransition     *WorkFinishJiraTransition `json:"jira_transition,omitempty"`
	Actions            []WorkFinishAction        `json:"actions"`
	Blockers           []string                  `json:"blockers"`
	Warnings           []string                  `json:"warnings,omitempty"`
	ReviewPolicy       FinishReviewPolicy        `json:"review_policy,omitempty"`
	ReviewEvidence     FinishReviewEvidence      `json:"review_evidence,omitempty"`
	HeadConstraint     *WorkFinishHeadConstraint `json:"head_constraint,omitempty"`
	LocalSync          WorkFinishLocalSync       `json:"local_sync"`
	FinalStatus        WorkStatusResult          `json:"final_status"`
	Readiness          WorkFinishReadinessReport `json:"readiness"`
	Receipt            WorkFinishReceipt         `json:"receipt"`
	NextStep           string                    `json:"next_step"`
	Approval           *ApprovalEvidence         `json:"approval,omitempty"`
}

type WorkFinishOptions struct {
	SyncLocal       bool   `json:"sync_local"`
	ExpectedHeadSHA string `json:"expected_head_sha,omitempty"`
}

var finishMissingPRRetryAttempts = 3
var finishMissingPRRetryDelay = time.Second
var finishChecksPollInterval = 5 * time.Second
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
	if options.ExpectedHeadSHA != "" && !IsFullCommitSHA(options.ExpectedHeadSHA) {
		return WorkFinishResult{}, fmt.Errorf("--expect-head must be a full 40-character commit SHA")
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
	if options.ExpectedHeadSHA != "" {
		result.HeadConstraint = &WorkFinishHeadConstraint{
			ExpectedHeadSHA: options.ExpectedHeadSHA,
			ExpectedSource:  "caller",
			PinMechanism:    "planned_gh_match_head_commit",
			State:           "pending",
		}
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
	initialStatus := status
	result.Actions = append(result.Actions, WorkFinishAction{Action: "linked_pr:inspect", Status: "done", Detail: linkedPRDetail(status)})
	if result.HeadConstraint != nil {
		if status.PRNumber == 0 {
			result.HeadConstraint.State = "not_verified"
			result.HeadConstraint.MismatchReason = "no linked pull request was available for the expected-head comparison"
		} else {
			result.HeadConstraint.ObservedInitialHeadSHA = strings.TrimSpace(status.HeadSHA)
			if !strings.EqualFold(status.State, "MERGED") {
				if blocker, reason := validateFinishHeadIdentity(repo, issueNumber, options.ExpectedHeadSHA, initialStatus, status, "initial intake"); reason != "" {
					return blockFinishHeadConstraint(repo, issueNumber, runner, result, status, blocker, reason)
				}
				result.HeadConstraint.State = "matched"
			}
		}
	}
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
		if options.ExpectedHeadSHA != "" {
			result.NextStep += "; then " + workFinishCommand(repo, issueNumber, options.ExpectedHeadSHA, "--dry-run")
		}
		return finishWithStatus(repo, issueNumber, runner, result, &status, nil)
	}
	if strings.EqualFold(status.State, "MERGED") {
		status, err = verifyMergedDevPR(repo, issueNumber, status.PRNumber, status, runner)
		if err != nil {
			result.Blockers = appendUniqueStrings(result.Blockers, "pr_binding")
			if result.HeadConstraint != nil {
				result.HeadConstraint.State = "reconciliation_failed"
				result.HeadConstraint.MismatchReason = err.Error()
			}
			return finishWithStatus(repo, issueNumber, runner, result, &status, err)
		}
		if result.HeadConstraint != nil {
			result.HeadConstraint.ObservedPostMergeHeadSHA = strings.TrimSpace(status.HeadSHA)
			if blocker, reason := validateFinishHeadIdentity(repo, issueNumber, options.ExpectedHeadSHA, initialStatus, status, "already-merged reconciliation"); reason != "" {
				result.Merged = true
				result.MergeRequestStatus = "already_merged_head_mismatch"
				return blockFinishHeadConstraint(repo, issueNumber, runner, result, status, blocker, reason)
			}
			result.HeadConstraint.State = "already_merged_verified"
		}
		result.AlreadyDone = true
		result.Merged = true
		result.MergeRequestStatus = "already_merged_verified"
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
			if result.HeadConstraint != nil {
				result.HeadConstraint.State = "ready_transition_preview"
			}
			result.Blockers = mergeBlockers(status.Blockers)
			result.Blockers = appendUniqueStrings(result.Blockers, jiraDone.Blockers...)
			appendJiraDoneBlockedAction(&result, jiraDone)
			result.NextStep = fmt.Sprintf("%s; then rerun --dry-run before merge", workFinishCommand(repo, issueNumber, options.ExpectedHeadSHA, "--apply"))
			return finishWithStatus(repo, issueNumber, runner, result, &status, nil)
		}
		if _, err := runner.Run("gh", "pr", "ready", fmt.Sprintf("%d", status.PRNumber), "--repo", repo.FullName()); err != nil {
			return result, fmt.Errorf("mark PR ready: %w", err)
		}
		status, err = DevPRStatus(repo, issueNumber, runner)
		if err != nil {
			if result.HeadConstraint != nil {
				result.HeadConstraint.State = "reconciliation_failed"
				result.HeadConstraint.MismatchReason = "PR state could not be reread after the ready transition: " + err.Error()
			}
			return result, err
		}
		result.PRState = status.State
		if result.HeadConstraint != nil {
			result.HeadConstraint.ObservedAfterReadySHA = strings.TrimSpace(status.HeadSHA)
			if blocker, reason := validateFinishHeadIdentity(repo, issueNumber, options.ExpectedHeadSHA, initialStatus, status, "after ready transition"); reason != "" {
				result.LocalSync = WorkFinishLocalSync{Skipped: true, Reason: "ready_transition_only"}
				return blockFinishHeadConstraint(repo, issueNumber, runner, result, status, blocker, reason)
			}
			result.HeadConstraint.State = "ready_only"
		}
		result.Blockers = mergeBlockers(status.Blockers)
		result.LocalSync = WorkFinishLocalSync{Skipped: true, Reason: "ready_transition_only"}
		nextStep := workFinishCommand(repo, issueNumber, options.ExpectedHeadSHA, "--dry-run")
		result.NextStep = nextStep
		report, reportErr := finishWithStatus(repo, issueNumber, runner, result, &status, nil)
		setWorkFinishNextStep(&report, nextStep)
		return report, reportErr
	}

	if containsString(status.Blockers, "checks_pending") && wait > 0 {
		result.Actions = append(result.Actions, WorkFinishAction{Action: "checks:wait", Status: "applied", Detail: wait.String()})
		deadline := time.Now().Add(wait)
		for containsString(status.Blockers, "checks_pending") && time.Now().Before(deadline) {
			time.Sleep(finishChecksPollInterval)
			status, err = DevPRStatus(repo, issueNumber, runner)
			if err != nil {
				return result, err
			}
			if result.HeadConstraint != nil {
				result.HeadConstraint.ObservedPreMergeHeadSHA = strings.TrimSpace(status.HeadSHA)
				if blocker, reason := validateFinishHeadIdentity(repo, issueNumber, options.ExpectedHeadSHA, initialStatus, status, "after checks refresh"); reason != "" {
					return blockFinishHeadConstraint(repo, issueNumber, runner, result, status, blocker, reason)
				}
				result.HeadConstraint.State = "matched"
			}
		}
	}

	policy := resolveFinishReviewPolicy(repo, status, runner)
	review := finishReviewEvidence(repo, status, policy, runner)
	result.ReviewPolicy = policy
	result.ReviewEvidence = review
	statusBlockers := mergeBlockers(status.Blockers)
	if policy.Value == FinishReviewPolicyNone || review.Blocker != "" || review.Status == "approved" || review.Status == "independent_recorded" {
		statusBlockers = removeString(statusBlockers, "review")
	}
	result.Blockers = appendUniqueStrings(result.Blockers, statusBlockers...)
	result.Blockers = appendUniqueStrings(result.Blockers, jiraDone.Blockers...)
	if review.Blocker != "" {
		result.Blockers = appendUniqueStrings(result.Blockers, review.Blocker)
		result.Actions = append(result.Actions, WorkFinishAction{Action: "review:verify", Status: "blocked", Detail: review.Blocker})
		appendJiraDoneBlockedAction(&result, jiraDone)
		result.NextStep = review.Remediation
		report, reportErr := finishWithStatus(repo, issueNumber, runner, result, &status, nil)
		if dryRun {
			return report, reportErr
		}
		if reportErr != nil {
			return report, reportErr
		}
		return report, fmt.Errorf("ticket finish blocked: %s", review.Blocker)
	}
	result.Actions = append(result.Actions, WorkFinishAction{Action: "review:verify", Status: "done", Detail: review.Status})
	if len(result.Blockers) > 0 {
		appendJiraDoneBlockedAction(&result, jiraDone)
		result.Actions = append(result.Actions, WorkFinishAction{Action: "pr:merge", Status: "blocked", Detail: strings.Join(result.Blockers, ",")})
		result.NextStep = finishBlockedNextStep(repo, issueNumber, result.Blockers, options.ExpectedHeadSHA)
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
			result.NextStep = workFinishCommand(repo, issueNumber, options.ExpectedHeadSHA, "--apply")
		}
		return finishWithLocalSync(repo, issueNumber, runner, result, true, &status, &status, options)
	}

	if options.ExpectedHeadSHA != "" && policy.Value != FinishReviewPolicyRecordedIndependent {
		fresh, refreshErr := DevPRStatus(repo, issueNumber, runner)
		if refreshErr != nil {
			result.HeadConstraint.State = "reconciliation_failed"
			result.HeadConstraint.MismatchReason = "PR state could not be reread immediately before merge: " + refreshErr.Error()
			result.Blockers = appendUniqueStrings(result.Blockers, "expected_head_verification_unavailable")
			result.NextStep = workFinishCommand(repo, issueNumber, options.ExpectedHeadSHA, "--dry-run")
			return finishWithStatus(repo, issueNumber, runner, result, &status, fmt.Errorf("ticket finish blocked: expected head could not be revalidated: %w", refreshErr))
		}
		result.HeadConstraint.ObservedPreMergeHeadSHA = strings.TrimSpace(fresh.HeadSHA)
		if blocker, reason := validateFinishHeadIdentity(repo, issueNumber, options.ExpectedHeadSHA, initialStatus, fresh, "immediately before merge"); reason != "" {
			return blockFinishHeadConstraint(repo, issueNumber, runner, result, fresh, blocker, reason)
		}
		freshPolicy := resolveFinishReviewPolicy(repo, fresh, runner)
		freshReview := finishReviewEvidence(repo, fresh, freshPolicy, runner)
		freshBlockers := mergeBlockers(fresh.Blockers)
		if freshPolicy.Value == FinishReviewPolicyNone || freshReview.Blocker != "" || freshReview.Status == "approved" || freshReview.Status == "independent_recorded" {
			freshBlockers = removeString(freshBlockers, "review")
		}
		if freshReview.Blocker != "" {
			freshBlockers = appendUniqueStrings(freshBlockers, freshReview.Blocker)
		}
		if freshPolicy.ValidationError != "" {
			freshBlockers = appendUniqueStrings(freshBlockers, "review_evidence_unavailable")
		}
		if len(freshBlockers) > 0 {
			result.Blockers = appendUniqueStrings(result.Blockers, freshBlockers...)
			result.ReviewPolicy = freshPolicy
			result.ReviewEvidence = freshReview
			result.Actions = append(result.Actions, WorkFinishAction{Action: "pr:merge", Status: "blocked", Detail: strings.Join(result.Blockers, ",")})
			result.NextStep = finishBlockedNextStep(repo, issueNumber, result.Blockers, options.ExpectedHeadSHA)
			return finishWithStatus(repo, issueNumber, runner, result, &fresh, fmt.Errorf("ticket finish blocked: %s", strings.Join(result.Blockers, ", ")))
		}
		status = fresh
		result.PRState = fresh.State
		result.ReviewPolicy = freshPolicy
		result.ReviewEvidence = freshReview
		policy = freshPolicy
		review = freshReview
	}
	if policy.Value == FinishReviewPolicyRecordedIndependent {
		if result.HeadConstraint == nil {
			result.HeadConstraint = &WorkFinishHeadConstraint{
				ExpectedHeadSHA:         strings.TrimSpace(status.HeadSHA),
				ExpectedSource:          "recorded_review",
				ObservedInitialHeadSHA:  strings.TrimSpace(initialStatus.HeadSHA),
				ObservedPreMergeHeadSHA: strings.TrimSpace(status.HeadSHA),
				PinMechanism:            "planned_gh_match_head_commit",
				State:                   "matched",
			}
		}
		if expected := strings.TrimSpace(result.HeadConstraint.ExpectedHeadSHA); expected != "" && !strings.EqualFold(expected, strings.TrimSpace(review.HeadSHA)) {
			return blockFinishHeadConstraint(repo, issueNumber, runner, result, status, "expected_head_mismatch", fmt.Sprintf("recorded review is bound to head %s while caller expected %s", valueOrUnknown(review.HeadSHA), expected))
		}
	}
	nativeMergedVerified := false
	if policy.Value == FinishReviewPolicyRecordedIndependent {
		if err := finishRecordedReviewMerge(repo, issueNumber, status, policy, result.HeadConstraint.ExpectedHeadSHA, runner, &result); err != nil {
			var requestErr *finishMergeRequestError
			if errors.As(err, &requestErr) {
				current, verified, reconcileErr := reconcileFinishMergeRequestFailure(repo, issueNumber, initialStatus, result.HeadConstraint.ExpectedHeadSHA, status, runner, &result, err)
				if reconcileErr != nil {
					return finishWithStatus(repo, issueNumber, runner, result, &current, reconcileErr)
				}
				status = current
				nativeMergedVerified = verified
			} else {
				current, statusErr := DevPRStatus(repo, issueNumber, runner)
				result.MergeRequestStatus = "not_requested"
				result.Blockers = appendUniqueStrings(result.Blockers, "review_evidence_unavailable")
				result.HeadConstraint.State = "review_revalidation_failed"
				result.HeadConstraint.MismatchReason = err.Error()
				result.Actions = append(result.Actions, WorkFinishAction{Action: "pr:merge", Status: "blocked", Detail: "recorded review revalidation failed before merge dispatch"})
				if statusErr != nil || current.PRNumber <= 0 || strings.TrimSpace(current.State) == "" || strings.EqualFold(current.State, "UNKNOWN") {
					current = unknownFinishPRStatus(status)
					result.PRState = current.State
					result.Blockers = appendUniqueStrings(result.Blockers, "expected_head_verification_unavailable")
					result.HeadConstraint.MismatchReason = fmt.Sprintf("recorded-review revalidation failed before merge dispatch (%v) and current native PR identity is unavailable (%v)", err, statusErr)
				} else {
					result.HeadConstraint.ObservedPreMergeHeadSHA = strings.TrimSpace(current.HeadSHA)
					if blocker, reason := validateFinishHeadIdentity(repo, issueNumber, result.HeadConstraint.ExpectedHeadSHA, initialStatus, current, "recorded-review merge revalidation"); reason != "" {
						return blockFinishHeadConstraint(repo, issueNumber, runner, result, current, blocker, reason)
					}
					status = current
					result.PRState = current.State
					if strings.EqualFold(current.State, "MERGED") {
						verifiedStatus, verifyErr := verifyMergedDevPR(repo, issueNumber, current.PRNumber, current, runner)
						result.Merged = true
						if strings.TrimSpace(verifiedStatus.State) != "" && !strings.EqualFold(verifiedStatus.State, "UNKNOWN") {
							result.Merged = strings.EqualFold(verifiedStatus.State, "MERGED")
						}
						result.HeadConstraint.ObservedPostMergeHeadSHA = strings.TrimSpace(verifiedStatus.HeadSHA)
						if verifyErr != nil {
							result.MergeRequestStatus = "not_requested_native_unverified"
							result.HeadConstraint.State = "reconciliation_failed"
							result.HeadConstraint.MismatchReason = verifyErr.Error()
							result.Blockers = appendUniqueStrings(result.Blockers, "pr_binding")
							status = verifiedStatus
						} else {
							result.MergeRequestStatus = "not_requested_native_merged"
							status = verifiedStatus
							if blocker, reason := validateFinishHeadIdentity(repo, issueNumber, result.HeadConstraint.ExpectedHeadSHA, initialStatus, verifiedStatus, "native merged-state readback after pre-dispatch failure"); reason != "" {
								return blockFinishHeadConstraint(repo, issueNumber, runner, result, verifiedStatus, blocker, reason)
							}
						}
					}
				}
				return finishWithStatus(repo, issueNumber, runner, result, &status, fmt.Errorf("ticket finish blocked: %w", err))
			}
		}
	} else if err := finishMergePR(repo, status, runner, &result, options.ExpectedHeadSHA); err != nil {
		if options.ExpectedHeadSHA == "" {
			return result, err
		}
		var requestErr *finishMergeRequestError
		if !errors.As(err, &requestErr) {
			current, readErr := DevPRStatus(repo, issueNumber, runner)
			result.MergeRequestStatus = "not_requested"
			result.HeadConstraint.State = "pre_dispatch_failed"
			result.HeadConstraint.MismatchReason = err.Error()
			if readErr != nil || current.PRNumber <= 0 || strings.TrimSpace(current.State) == "" || strings.EqualFold(current.State, "UNKNOWN") {
				current = unknownFinishPRStatus(status)
				result.PRState = current.State
				result.HeadConstraint.MismatchReason = fmt.Sprintf("merge request was not dispatched (%v) and current native PR identity is unavailable (%v)", err, readErr)
				result.Blockers = appendUniqueStrings(result.Blockers, "expected_head_verification_unavailable")
			} else {
				result.PRState = current.State
				result.HeadConstraint.ObservedPreMergeHeadSHA = strings.TrimSpace(current.HeadSHA)
				if blocker, reason := validateFinishHeadIdentity(repo, issueNumber, options.ExpectedHeadSHA, initialStatus, current, "before merge dispatch"); reason != "" {
					return blockFinishHeadConstraint(repo, issueNumber, runner, result, current, blocker, reason)
				}
				result.Blockers = appendUniqueStrings(result.Blockers, "merge_failed")
			}
			result.Actions = append(result.Actions, WorkFinishAction{Action: "pr:merge", Status: "blocked", Detail: "merge request failed before backend dispatch"})
			return finishWithStatus(repo, issueNumber, runner, result, &current, fmt.Errorf("ticket finish blocked before merge dispatch: %w", err))
		}
		current, verified, reconcileErr := reconcileFinishMergeRequestFailure(repo, issueNumber, initialStatus, options.ExpectedHeadSHA, status, runner, &result, err)
		if reconcileErr != nil {
			return finishWithStatus(repo, issueNumber, runner, result, &current, reconcileErr)
		}
		status = current
		nativeMergedVerified = verified
	}
	if result.MergeRequestStatus == "" {
		result.MergeRequestStatus = "accepted"
	}
	verifiedStatus := status
	if !nativeMergedVerified {
		verifiedStatus, err = verifyMergedDevPR(repo, issueNumber, status.PRNumber, status, runner)
		if err != nil {
			status = verifiedStatus
			result.PRState = status.State
			result.Merged = strings.EqualFold(status.State, "MERGED")
			if result.HeadConstraint != nil {
				result.HeadConstraint.ObservedPostMergeHeadSHA = strings.TrimSpace(status.HeadSHA)
			}
			result.Blockers = appendUniqueStrings(result.Blockers, "pr_binding")
			if result.HeadConstraint != nil {
				result.HeadConstraint.State = "reconciliation_failed"
				result.HeadConstraint.MismatchReason = err.Error()
			}
			if result.MergeRequestStatus == "accepted" {
				result.MergeRequestStatus = "accepted_native_unverified"
			}
			return finishWithStatus(repo, issueNumber, runner, result, &status, err)
		}
	}
	status = verifiedStatus
	result.PRState = status.State
	result.Merged = true
	if result.MergeRequestStatus == "accepted" {
		result.MergeRequestStatus = "accepted_native_verified"
	}
	if result.HeadConstraint != nil {
		result.HeadConstraint.ObservedPostMergeHeadSHA = strings.TrimSpace(status.HeadSHA)
		expectedHeadSHA := strings.TrimSpace(result.HeadConstraint.ExpectedHeadSHA)
		if expectedHeadSHA == "" {
			expectedHeadSHA = strings.TrimSpace(options.ExpectedHeadSHA)
		}
		if expectedHeadSHA != "" {
			if blocker, reason := validateFinishHeadIdentity(repo, issueNumber, expectedHeadSHA, initialStatus, status, "native merged-state readback"); reason != "" {
				if blocker == "expected_head_mismatch" {
					result.MergeRequestStatus = "accepted_native_head_mismatch"
				} else {
					result.MergeRequestStatus = "accepted_native_identity_mismatch"
				}
				return blockFinishHeadConstraint(repo, issueNumber, runner, result, status, blocker, reason)
			}
			result.HeadConstraint.State = "merged_verified"
		}
	}
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
	return finishWithLocalSync(repo, issueNumber, runner, result, true, &status, &status, options)
}

func validateFinishHeadIdentity(repo RepoRef, issueNumber int, expectedHeadSHA string, initial, observed DevPRStatusResult, phase string) (string, string) {
	if observed.PRNumber <= 0 || initial.PRNumber <= 0 || observed.PRNumber != initial.PRNumber {
		return "pr_binding", fmt.Sprintf("%s: linked PR identity changed from #%d to #%d", phase, initial.PRNumber, observed.PRNumber)
	}
	if observed.Repo != "" && !strings.EqualFold(observed.Repo, repo.FullName()) {
		return "pr_binding", fmt.Sprintf("%s: PR repository changed from %s to %s", phase, repo.FullName(), observed.Repo)
	}
	if observed.Issue > 0 && observed.Issue != issueNumber {
		return "pr_binding", fmt.Sprintf("%s: PR is bound to ticket #%d, not #%d", phase, observed.Issue, issueNumber)
	}
	if expectedHeadSHA != "" && !strings.EqualFold(strings.TrimSpace(observed.HeadSHA), expectedHeadSHA) {
		return "expected_head_mismatch", fmt.Sprintf("%s: expected PR head %s but observed %s", phase, expectedHeadSHA, valueOrUnknown(observed.HeadSHA))
	}
	if initial.Binding.BaseRef == "" || observed.Binding.BaseRef == "" {
		return "pr_base_mismatch", fmt.Sprintf("%s: target base branch identity is unavailable", phase)
	}
	if strings.TrimSpace(initial.Binding.BaseRef) != strings.TrimSpace(observed.Binding.BaseRef) {
		return "pr_base_mismatch", fmt.Sprintf("%s: target base branch changed from %s to %s", phase, initial.Binding.BaseRef, observed.Binding.BaseRef)
	}
	if !observed.ClosingReference || !observed.Binding.Trusted {
		return "pr_binding", fmt.Sprintf("%s: PR closing reference or trusted branch binding is unavailable", phase)
	}
	return "", ""
}

type finishMergeRequestError struct {
	err error
}

func (err *finishMergeRequestError) Error() string {
	if err == nil || err.err == nil {
		return "merge request failed"
	}
	return err.err.Error()
}

func (err *finishMergeRequestError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.err
}

func unknownFinishPRStatus(previous DevPRStatusResult) DevPRStatusResult {
	previous.State = "UNKNOWN"
	previous.Mergeable = ""
	previous.ReviewDecision = ""
	previous.IsDraft = false
	previous.Binding = DevPRBinding{Source: "unavailable", Blockers: []string{"pr_binding"}}
	previous.Blockers = []string{"merge_state_unavailable"}
	previous.Checks = nil
	previous.ChecksUnavailable = true
	previous.Ready = false
	previous.HeadSHA = ""
	previous.BaseSHA = ""
	previous.MergeCommitSHA = ""
	previous.ClosingReference = false
	return previous
}

func reconcileFinishMergeRequestFailure(repo RepoRef, issueNumber int, initial DevPRStatusResult, expectedHeadSHA string, previous DevPRStatusResult, runner CommandRunner, result *WorkFinishResult, requestErr error) (DevPRStatusResult, bool, error) {
	current, readErr := DevPRStatus(repo, issueNumber, runner)
	if readErr != nil || current.PRNumber <= 0 || strings.TrimSpace(current.State) == "" || strings.EqualFold(current.State, "UNKNOWN") {
		current = unknownFinishPRStatus(previous)
		result.PRState = current.State
		result.MergeRequestStatus = "ambiguous_native_unverified"
		result.HeadConstraint.State = "reconciliation_failed"
		result.HeadConstraint.MismatchReason = fmt.Sprintf("merge request was dispatched (%v) but current native PR identity is unavailable (%v)", requestErr, readErr)
		result.Blockers = appendUniqueStrings(result.Blockers, "merge_state_unavailable", "expected_head_verification_unavailable")
		result.Actions = append(result.Actions, WorkFinishAction{Action: "pr:merge", Status: "blocked", Detail: "merge request was dispatched; native merged state could not be verified"})
		return current, false, fmt.Errorf("ticket finish blocked: %s", result.HeadConstraint.MismatchReason)
	}
	result.PRState = current.State
	result.HeadConstraint.ObservedPostMergeHeadSHA = strings.TrimSpace(current.HeadSHA)
	if blocker, reason := validateFinishHeadIdentity(repo, issueNumber, expectedHeadSHA, initial, current, "native readback after dispatched merge request"); reason != "" {
		result.MergeRequestStatus = "ambiguous_native_identity_mismatch"
		result.Merged = strings.EqualFold(current.State, "MERGED")
		result.HeadConstraint.MismatchReason = reason
		if blocker == "expected_head_mismatch" {
			result.HeadConstraint.State = "mismatch"
		} else {
			result.HeadConstraint.State = "identity_mismatch"
		}
		result.Blockers = appendUniqueStrings(result.Blockers, blocker)
		result.Actions = append(result.Actions, WorkFinishAction{Action: "head:verify", Status: "blocked", Detail: reason})
		result.NextStep = finishHeadMismatchNextStep(repo, issueNumber, result.HeadConstraint, blocker)
		return current, false, fmt.Errorf("ticket finish blocked: %s: %s", blocker, reason)
	}
	if !strings.EqualFold(current.State, "MERGED") {
		result.MergeRequestStatus = "ambiguous_native_not_merged"
		result.HeadConstraint.State = "merge_failed"
		result.HeadConstraint.MismatchReason = fmt.Sprintf("merge request was dispatched but native readback is %s: %v", valueOrUnknown(current.State), requestErr)
		result.Blockers = appendUniqueStrings(result.Blockers, "merge_failed")
		result.Actions = append(result.Actions, WorkFinishAction{Action: "pr:merge", Status: "failed", Detail: result.HeadConstraint.MismatchReason})
		return current, false, fmt.Errorf("ticket finish blocked: %s", result.HeadConstraint.MismatchReason)
	}
	result.MergeRequestStatus = "ambiguous_native_merged"
	result.Merged = true
	verified, verifyErr := verifyMergedDevPR(repo, issueNumber, current.PRNumber, current, runner)
	result.PRState = verified.State
	result.Merged = true
	if strings.TrimSpace(verified.State) != "" && !strings.EqualFold(verified.State, "UNKNOWN") {
		result.Merged = strings.EqualFold(verified.State, "MERGED")
	}
	result.HeadConstraint.ObservedPostMergeHeadSHA = strings.TrimSpace(verified.HeadSHA)
	if verifyErr != nil {
		result.MergeRequestStatus = "ambiguous_native_unverified"
		result.HeadConstraint.State = "reconciliation_failed"
		result.HeadConstraint.MismatchReason = fmt.Sprintf("native PR reported MERGED after dispatched request but immutable merge identity could not be verified: %v", verifyErr)
		result.Blockers = appendUniqueStrings(result.Blockers, "pr_binding")
		result.Actions = append(result.Actions, WorkFinishAction{Action: "pr:merge", Status: "blocked", Detail: result.HeadConstraint.MismatchReason})
		return verified, false, fmt.Errorf("ticket finish blocked: %s", result.HeadConstraint.MismatchReason)
	}
	if blocker, reason := validateFinishHeadIdentity(repo, issueNumber, expectedHeadSHA, initial, verified, "native merged-state readback after ambiguous request"); reason != "" {
		result.MergeRequestStatus = "ambiguous_native_head_mismatch"
		result.Merged = true
		result.HeadConstraint.MismatchReason = reason
		if blocker == "expected_head_mismatch" {
			result.HeadConstraint.State = "mismatch"
		} else {
			result.HeadConstraint.State = "identity_mismatch"
		}
		result.Blockers = appendUniqueStrings(result.Blockers, blocker)
		result.Actions = append(result.Actions, WorkFinishAction{Action: "head:verify", Status: "blocked", Detail: reason})
		result.NextStep = finishHeadMismatchNextStep(repo, issueNumber, result.HeadConstraint, blocker)
		return verified, false, fmt.Errorf("ticket finish blocked: %s: %s", blocker, reason)
	}
	result.HeadConstraint.State = "merged_verified"
	result.MergeRequestStatus = "ambiguous_native_verified"
	return verified, true, nil
}

func blockFinishHeadConstraint(repo RepoRef, issueNumber int, runner CommandRunner, result WorkFinishResult, status DevPRStatusResult, blocker string, reason string) (WorkFinishResult, error) {
	if result.HeadConstraint == nil {
		result.HeadConstraint = &WorkFinishHeadConstraint{ExpectedSource: "caller"}
	}
	result.PRState = status.State
	result.HeadConstraint.MismatchReason = reason
	if blocker == "expected_head_mismatch" {
		result.HeadConstraint.State = "mismatch"
	} else {
		result.HeadConstraint.State = "identity_mismatch"
	}
	result.Blockers = appendUniqueStrings(result.Blockers, blocker)
	result.Actions = append(result.Actions, WorkFinishAction{Action: "head:verify", Status: "blocked", Detail: reason})
	result.NextStep = finishHeadMismatchNextStep(repo, issueNumber, result.HeadConstraint, blocker)
	var finishErr error
	if !result.DryRun {
		finishErr = fmt.Errorf("ticket finish blocked: %s: %s", blocker, reason)
	}
	return finishWithStatus(repo, issueNumber, runner, result, &status, finishErr)
}

func finishHeadMismatchNextStep(repo RepoRef, issueNumber int, evidence *WorkFinishHeadConstraint, blocker string) string {
	if blocker == "expected_head_mismatch" {
		expected := ""
		if evidence != nil {
			expected = evidence.ExpectedHeadSHA
		}
		return fmt.Sprintf("review the PR head drift and restore the caller-reviewed commit, then run %s", workFinishCommand(repo, issueNumber, expected, "--dry-run"))
	}
	expected := ""
	if evidence != nil {
		expected = evidence.ExpectedHeadSHA
	}
	return fmt.Sprintf("resolve the PR identity or target-base blocker, then run %s", workFinishCommand(repo, issueNumber, expected, "--dry-run"))
}

func workFinishCommand(repo RepoRef, issueNumber int, expectedHeadSHA string, mode string) string {
	command := fmt.Sprintf("gira ticket finish --repo %s --ticket %d", repo.FullName(), issueNumber)
	if expectedHeadSHA != "" {
		command += " --expect-head " + expectedHeadSHA
	}
	if strings.TrimSpace(mode) != "" {
		command += " " + strings.TrimSpace(mode)
	}
	return command
}

func finishRecordedReviewMerge(repo RepoRef, issueNumber int, status DevPRStatusResult, policy FinishReviewPolicy, expectedHeadSHA string, runner CommandRunner, result *WorkFinishResult) error {
	if err := revalidateRecordedReviewBeforeMerge(repo, issueNumber, status, policy, runner); err != nil {
		return fmt.Errorf("recorded review revalidation failed: %w", err)
	}
	if result != nil && result.HeadConstraint != nil {
		result.HeadConstraint.ObservedPreMergeHeadSHA = strings.TrimSpace(status.HeadSHA)
	}
	return finishMergePR(repo, status, runner, result, expectedHeadSHA)
}

func revalidateRecordedReviewBeforeMerge(repo RepoRef, issueNumber int, reviewed DevPRStatusResult, reviewedPolicy FinishReviewPolicy, runner CommandRunner) error {
	fresh, err := DevPRStatus(repo, issueNumber, runner)
	if err != nil {
		return fmt.Errorf("refresh PR metadata: %w", err)
	}
	if fresh.PRNumber != reviewed.PRNumber || !strings.EqualFold(strings.TrimSpace(fresh.State), "OPEN") || fresh.IsDraft || !fresh.ClosingReference || !fresh.Binding.Trusted ||
		!strings.EqualFold(strings.TrimSpace(fresh.HeadSHA), strings.TrimSpace(reviewed.HeadSHA)) ||
		!strings.EqualFold(strings.TrimSpace(fresh.BaseSHA), strings.TrimSpace(reviewed.BaseSHA)) ||
		strings.TrimSpace(fresh.Binding.BaseRef) != strings.TrimSpace(reviewed.Binding.BaseRef) {
		return fmt.Errorf("PR identity, head, base, draft, state, closing reference, or branch binding changed since review")
	}
	for _, blocker := range fresh.Blockers {
		if blocker != "review" {
			return fmt.Errorf("PR has a current non-review blocker: %s", blocker)
		}
	}
	freshPolicy := resolveFinishReviewPolicy(repo, fresh, runner)
	if freshPolicy.ValidationError != "" || freshPolicy.Value != reviewedPolicy.Value || freshPolicy.Source != reviewedPolicy.Source {
		return fmt.Errorf("committed review policy changed or could not be revalidated")
	}
	evidence := finishReviewEvidence(repo, fresh, freshPolicy, runner)
	if evidence.Blocker != "" || (evidence.Status != "independent_recorded" && evidence.Status != "approved") {
		return fmt.Errorf("current recorded review is not satisfied: %s", evidence.Blocker)
	}
	return nil
}

func finishMergePR(repo RepoRef, status DevPRStatusResult, runner CommandRunner, result *WorkFinishResult, expectedHeadSHA string) error {
	args := []string{"pr", "merge", fmt.Sprintf("%d", status.PRNumber), "--repo", repo.FullName(), "--squash", "--delete-branch"}
	expectedHeadSHA = strings.TrimSpace(expectedHeadSHA)
	if expectedHeadSHA != "" {
		if !strings.EqualFold(expectedHeadSHA, strings.TrimSpace(status.HeadSHA)) {
			return fmt.Errorf("refuse merge: expected head %s does not match selected PR head %s", expectedHeadSHA, valueOrUnknown(status.HeadSHA))
		}
		args = append(args, "--match-head-commit", expectedHeadSHA)
		if result != nil && result.HeadConstraint != nil {
			result.HeadConstraint.PinMechanism = "gh_match_head_commit"
		}
	}
	if _, err := runner.Run("gh", args...); err != nil {
		if !finishGraphQLRateLimitError(err) {
			return &finishMergeRequestError{err: fmt.Errorf("merge PR: %w", err)}
		}
		diagnostic := finishMergeRateLimitDiagnostic(repo, runner)
		var fallbackDetail string
		var fallbackErr error
		if expectedHeadSHA != "" {
			if result != nil && result.HeadConstraint != nil {
				result.HeadConstraint.PinMechanism = "gh_match_head_commit+rest_sha_expected_head"
			}
			expectedStatus := status
			expectedStatus.HeadSHA = expectedHeadSHA
			fallbackDetail, fallbackErr = finishMergePRViaRESTForStatus(repo, expectedStatus, runner)
		} else {
			fallbackDetail, fallbackErr = finishMergePRViaREST(repo, status.PRNumber, runner)
		}
		if fallbackErr != nil {
			if result != nil {
				result.Actions = append(result.Actions, WorkFinishAction{Action: "pr:merge_fallback", Status: "blocked", Detail: strings.TrimSpace("GraphQL merge rate limit; " + diagnostic + "; " + fallbackErr.Error())})
			}
			return &finishMergeRequestError{err: fmt.Errorf("merge PR: GraphQL rate limit; %s; REST fallback failed: %w", diagnostic, fallbackErr)}
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
	return finishMergePRViaRESTExpected(repo, prNumber, nil, runner)
}

func finishMergePRViaRESTForStatus(repo RepoRef, status DevPRStatusResult, runner CommandRunner) (string, error) {
	return finishMergePRViaRESTExpected(repo, status.PRNumber, &status, runner)
}

func finishMergePRViaRESTExpected(repo RepoRef, prNumber int, expected *DevPRStatusResult, runner CommandRunner) (string, error) {
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
		Base struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"base"`
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
	if expected != nil {
		if !strings.EqualFold(headSHA, strings.TrimSpace(expected.HeadSHA)) ||
			!strings.EqualFold(strings.TrimSpace(pr.Base.SHA), strings.TrimSpace(expected.BaseSHA)) ||
			strings.TrimSpace(pr.Base.Ref) != strings.TrimSpace(expected.Binding.BaseRef) {
			return "", fmt.Errorf("PR #%d changed its reviewed head or base before REST merge", prNumber)
		}
		headSHA = strings.TrimSpace(expected.HeadSHA)
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
	if mergePlanned && len(result.Blockers) == 0 {
		actions, err := convergeFinishedIssue(repo, issueNumber, result.Merged || result.AlreadyDone, result.DryRun, runner)
		if err != nil {
			result.Blockers = appendUniqueStrings(result.Blockers, "issue_closure_failed")
			result.Actions = append(result.Actions, WorkFinishAction{Action: "ticket:converge", Status: "failed", Detail: err.Error()})
			return finishWithStatus(repo, issueNumber, runner, result, knownPRStatus, nil)
		}
		result.Actions = append(result.Actions, actions...)
	}
	if !result.DryRun && mergePlanned && local.Attempted && !local.Skipped {
		if _, err := runner.Run("git", "checkout", local.TargetBranch); err != nil {
			result.LocalSync.Reason = "checkout_failed"
			result.Warnings = append(result.Warnings, fmt.Sprintf("local sync skipped after merge: checkout %s: %v", local.TargetBranch, err))
			setWorkFinishAction(result.Actions, "local:sync_base", "failed", result.LocalSync.Reason)
		} else if _, err := runner.Run("git", "pull", "--ff-only", "origin", local.TargetBranch); err != nil {
			result.LocalSync.Reason = "pull_failed"
			result.Warnings = append(result.Warnings, fmt.Sprintf("local sync skipped after merge: pull %s: %v", local.TargetBranch, err))
			setWorkFinishAction(result.Actions, "local:sync_base", "failed", result.LocalSync.Reason)
		} else {
			setWorkFinishAction(result.Actions, "local:sync_base", "applied", "checkout "+local.TargetBranch+" and pull --ff-only")
		}
	}
	report, err := finishWithStatus(repo, issueNumber, runner, result, knownPRStatus, nil)
	if report.DryRun && mergePlanned && !report.AlreadyDone && len(report.Blockers) == 0 {
		setWorkFinishNextStep(&report, workFinishCommand(repo, issueNumber, options.ExpectedHeadSHA, "--apply"))
	}
	return report, err
}

func setWorkFinishNextStep(result *WorkFinishResult, nextStep string) {
	if result == nil {
		return
	}
	result.NextStep = nextStep
	result.Readiness.NextStep = nextStep
	result.Receipt.FinalState.NextStep = nextStep
	if result.Receipt.SchemaVersion != "" {
		result.Receipt.RenderedBody = renderWorkFinishReceipt(result.Receipt)
	}
	if result.DryRun {
		result.Approval = WorkFinishApprovalEvidence(*result)
	}
}

func setWorkFinishAction(actions []WorkFinishAction, action string, status string, detail string) {
	for index := len(actions) - 1; index >= 0; index-- {
		if actions[index].Action == action {
			actions[index].Status = status
			actions[index].Detail = detail
			return
		}
	}
}

// convergeFinishedIssue makes the GitHub issue state match verified delivery
// evidence. GitHub only auto-closes closing references for default-branch
// merges, so finish owns the equivalent convergence for eligible deliveries to
// any base branch. It intentionally runs only after the PR is known merged.
func convergeFinishedIssue(repo RepoRef, issueNumber int, merged bool, dryRun bool, runner CommandRunner) ([]WorkFinishAction, error) {
	if !merged {
		return nil, nil
	}
	issue, err := fetchDevIssue(repo, issueNumber, runner)
	if err != nil {
		return nil, fmt.Errorf("converge finished issue: %w", err)
	}
	actions := []WorkFinishAction{}
	if !strings.EqualFold(issue.State, "closed") {
		action := WorkFinishAction{
			Action: "ticket:close",
			Status: plannedOrAppliedStatus(dryRun),
			Detail: "close open GitHub issue after verified merged linked PR",
		}
		actions = append(actions, action)
		if dryRun {
			return actions, nil
		}
		if _, err := runner.Run("gh", "issue", "close", fmt.Sprintf("%d", issueNumber), "--repo", repo.FullName(), "--reason", "completed"); err != nil {
			return actions, fmt.Errorf("close finished issue: %w", err)
		}
		issue, err = fetchDevIssue(repo, issueNumber, runner)
		if err != nil {
			return actions, fmt.Errorf("verify finished issue closure: %w", err)
		}
		if !strings.EqualFold(issue.State, "closed") {
			return actions, fmt.Errorf("verify finished issue closure: issue #%d is still %s", issueNumber, issue.State)
		}
	}
	normalize, err := normalizeFinishedIssueStatusForIssue(repo, issueNumber, issue, dryRun, runner)
	if err != nil {
		return actions, err
	}
	if strings.TrimSpace(normalize.Action) != "" {
		actions = append(actions, normalize)
	}
	return actions, nil
}

func normalizeFinishedIssueStatus(repo RepoRef, issueNumber int, dryRun bool, runner CommandRunner) (WorkFinishAction, error) {
	issue, err := fetchDevIssue(repo, issueNumber, runner)
	if err != nil {
		return WorkFinishAction{}, fmt.Errorf("normalize finished issue status: %w", err)
	}
	return normalizeFinishedIssueStatusForIssue(repo, issueNumber, issue, dryRun, runner)
}

func normalizeFinishedIssueStatusForIssue(repo RepoRef, issueNumber int, issue devStartIssue, dryRun bool, runner CommandRunner) (WorkFinishAction, error) {
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
			if hasWorkFinishAction(result.Actions, "ticket:close", "planned") {
				result.FinalStatus.NextAction = "converge_completion_state"
				expectedHeadSHA := ""
				if result.HeadConstraint != nil && result.HeadConstraint.ExpectedSource == "caller" {
					expectedHeadSHA = result.HeadConstraint.ExpectedHeadSHA
				}
				result.NextStep = workFinishCommand(repo, issueNumber, expectedHeadSHA, "--apply")
				result.FinalStatus.NextStep = result.NextStep
			} else {
				result.NextStep = status.NextStep
			}
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

func hasWorkFinishAction(actions []WorkFinishAction, action string, status string) bool {
	for _, candidate := range actions {
		if candidate.Action == action && candidate.Status == status {
			return true
		}
	}
	return false
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
			Status:   firstNonEmpty(status.ReviewStatus, "missing"),
			Policy:   finishReviewPolicyFromStatus(status.ReviewPolicy, result.ReviewPolicy),
			Evidence: result.ReviewEvidence,
		},
		LabelState: WorkFinishReadinessLabelState{
			Status:             status.Status,
			Labels:             append([]string(nil), status.Labels...),
			ActiveStatusLabels: activeStatusLabels(status.Labels),
		},
		AcceptanceCriteria: status.Acceptance,
		ReleaseImpact:      status.ReleaseImpact,
		NextAction:         status.NextAction,
		NextStep:           firstNonEmpty(result.NextStep, status.NextStep),
		Warnings:           append([]string(nil), status.Warnings...),
	}
	if status.PullRequest != nil {
		report.PullRequest = WorkFinishReadinessPullRequest{
			Available:        status.PullRequest.Available,
			Number:           status.PullRequest.Number,
			URL:              status.PullRequest.URL,
			State:            status.PullRequest.State,
			Mergeable:        status.PullRequest.Mergeable,
			ReviewDecision:   status.PullRequest.ReviewDecision,
			IsDraft:          status.PullRequest.IsDraft,
			HeadRefName:      status.PullRequest.HeadRefName,
			BaseRefName:      status.PullRequest.BaseRefName,
			HeadSHA:          status.PullRequest.HeadSHA,
			BaseSHA:          status.PullRequest.BaseSHA,
			MergeCommitSHA:   status.PullRequest.MergeCommitSHA,
			ClosingReference: status.PullRequest.ClosingReference,
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
	if report.Review.Evidence.Status == "" {
		report.Review.Evidence.Status = report.Review.Status
		report.Review.Evidence.Decision = report.Review.Decision
	}
	report.Blockers = finishReadinessBlockers(result, report)
	report.Ready = len(report.Blockers) == 0 && finishEvidenceReady(report, status.NextAction, result)
	if !report.Ready && report.NextAction == "" {
		report.NextAction = "resolve_finish_blockers"
	}
	return report
}

func firstFinishReviewPolicy(values ...FinishReviewPolicy) FinishReviewPolicy {
	for _, value := range values {
		if strings.TrimSpace(value.Value) != "" {
			return value
		}
	}
	return FinishReviewPolicy{Value: FinishReviewPolicyMissing, Source: "unavailable"}
}

func finishReviewPolicyFromStatus(status *FinishReviewPolicy, fallback FinishReviewPolicy) FinishReviewPolicy {
	if status != nil {
		return firstFinishReviewPolicy(*status, fallback)
	}
	return firstFinishReviewPolicy(fallback)
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
	if !result.AlreadyDone && report.PullRequest.Available && (report.Checks.Status == "unknown" || report.Checks.Status == "missing" || report.Checks.Status == "") {
		blockers = appendUniqueStrings(blockers, "checks")
	}
	if report.PullRequest.IsDraft {
		blockers = appendUniqueStrings(blockers, "draft")
	}
	if report.Review.Status == "blocked" && report.Review.Policy.Value != FinishReviewPolicyNone && report.Review.Evidence.Blocker == "" {
		blockers = appendUniqueStrings(blockers, "review")
	}
	return blockers
}

func finishEvidenceReady(report WorkFinishReadinessReport, nextAction string, result WorkFinishResult) bool {
	if result.AlreadyDone {
		return strings.EqualFold(report.Issue.State, "closed")
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
	var headConstraint *WorkFinishHeadConstraint
	if result.HeadConstraint != nil {
		copy := *result.HeadConstraint
		headConstraint = &copy
	}
	receipt := WorkFinishReceipt{
		SchemaVersion:      "finish-receipt/v1",
		FinishedAt:         finishReceiptNow().Format(time.RFC3339),
		Repository:         readiness.Repository,
		Issue:              readiness.Issue,
		PullRequest:        WorkFinishReceiptPR{Number: readiness.PullRequest.Number, URL: readiness.PullRequest.URL, State: readiness.PullRequest.State, Merged: result.Merged || result.AlreadyDone || strings.EqualFold(readiness.PullRequest.State, "MERGED"), HeadSHA: readiness.PullRequest.HeadSHA, BaseSHA: readiness.PullRequest.BaseSHA, MergeCommitSHA: readiness.PullRequest.MergeCommitSHA, ClosingReference: readiness.PullRequest.ClosingReference},
		HeadConstraint:     headConstraint,
		MergeRequestStatus: result.MergeRequestStatus,
		ChecksSummary:      readiness.Checks,
		ReviewSummary:      readiness.Review,
		EvidenceSummary:    readiness.Evidence,
		TelemetrySummary:   result.FinalStatus.Telemetry,
		LabelChanges:       finishReceiptLabelChanges(result.Actions),
		FinalState: WorkFinishReceiptFinalState{
			IssueState:       readiness.Issue.State,
			Status:           readiness.Issue.Status,
			GitHubIssueState: readiness.Issue.State,
			GiraStatus:       readiness.Issue.Status,
			NextAction:       readiness.NextAction,
			NextStep:         readiness.NextStep,
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
	fmt.Fprintf(&b, "- Ticket: #%d gira_status=%s github_issue_state=%s\n", receipt.Issue.Number, valueOrUnknown(receipt.FinalState.GiraStatus), valueOrUnknown(receipt.FinalState.GitHubIssueState))
	fmt.Fprintf(&b, "- Linked PR: %s state=%s merged=%t head=%s merge_commit=%s closing_reference=%t\n", pr, valueOrUnknown(receipt.PullRequest.State), receipt.PullRequest.Merged, valueOrUnknown(receipt.PullRequest.HeadSHA), valueOrUnknown(receipt.PullRequest.MergeCommitSHA), receipt.PullRequest.ClosingReference)
	if receipt.MergeRequestStatus != "" {
		fmt.Fprintf(&b, "- Merge request: %s\n", receipt.MergeRequestStatus)
	}
	if receipt.HeadConstraint != nil {
		fmt.Fprintf(&b, "- Head constraint: state=%s source=%s expected=%s observed_initial=%s observed_pre_merge=%s observed_after_ready=%s observed_post_merge=%s pin=%s mismatch=%s\n",
			valueOrUnknown(receipt.HeadConstraint.State),
			valueOrUnknown(receipt.HeadConstraint.ExpectedSource),
			valueOrUnknown(receipt.HeadConstraint.ExpectedHeadSHA),
			valueOrUnknown(receipt.HeadConstraint.ObservedInitialHeadSHA),
			valueOrUnknown(receipt.HeadConstraint.ObservedPreMergeHeadSHA),
			valueOrUnknown(receipt.HeadConstraint.ObservedAfterReadySHA),
			valueOrUnknown(receipt.HeadConstraint.ObservedPostMergeHeadSHA),
			valueOrUnknown(receipt.HeadConstraint.PinMechanism),
			valueOrUnknown(receipt.HeadConstraint.MismatchReason),
		)
	}
	fmt.Fprintf(&b, "- Checks: %s total=%d passing=%d pending=%d failing=%d\n", valueOrUnknown(receipt.ChecksSummary.Status), receipt.ChecksSummary.Total, receipt.ChecksSummary.Passing, receipt.ChecksSummary.Pending, receipt.ChecksSummary.Failing)
	fmt.Fprintf(&b, "- Review: %s policy=%s source=%s evidence=%s\n", valueOrUnknown(receipt.ReviewSummary.Status), valueOrUnknown(receipt.ReviewSummary.Policy.Value), valueOrUnknown(receipt.ReviewSummary.Policy.Source), valueOrUnknown(receipt.ReviewSummary.Evidence.Status))
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
	if local.Branch != targetBranch {
		worktreesOut, err := runner.Run("git", "worktree", "list", "--porcelain")
		if err != nil {
			local.Skipped = true
			local.Reason = "worktree_list_unavailable"
			actions = append(actions, WorkFinishAction{Action: "local:sync_base", Status: "skipped", Detail: local.Reason})
			return local, actions
		}
		if branchCheckedOutInWorktree(string(worktreesOut), targetBranch) {
			local.Skipped = true
			local.Reason = "branch_checked_out_elsewhere"
			actions = append(actions, WorkFinishAction{Action: "local:sync_base", Status: "skipped", Detail: local.Reason})
			return local, actions
		}
	}
	local.Attempted = true
	actions = append(actions, WorkFinishAction{Action: "local:sync_base", Status: "planned", Detail: "checkout " + targetBranch + " and pull --ff-only"})
	return local, actions
}

func branchCheckedOutInWorktree(output string, branch string) bool {
	want := "branch refs/heads/" + strings.TrimSpace(branch)
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
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

func finishBlockedNextStep(repo RepoRef, issueNumber int, blockers []string, expectedHeadSHA ...string) string {
	expected := ""
	if len(expectedHeadSHA) > 0 {
		expected = expectedHeadSHA[0]
	}
	if containsString(blockers, "checks_pending") {
		return "wait for required checks, then " + workFinishCommand(repo, issueNumber, expected, "--dry-run")
	}
	if containsString(blockers, "checks") {
		return "fix failing checks, then " + workFinishCommand(repo, issueNumber, expected, "--dry-run")
	}
	if containsString(blockers, "review") {
		return "resolve review requirements, then " + workFinishCommand(repo, issueNumber, expected, "--dry-run")
	}
	if containsString(blockers, "draft") {
		return workFinishCommand(repo, issueNumber, expected, "--dry-run")
	}
	if expected != "" {
		return workFinishCommand(repo, issueNumber, expected, "--dry-run")
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
	if result.HeadConstraint != nil {
		output += fmt.Sprintf("head constraint: state=%s expected=%s observed_pre_merge=%s observed_post_merge=%s pin=%s mismatch=%s\n",
			valueOrUnknown(result.HeadConstraint.State),
			valueOrUnknown(result.HeadConstraint.ExpectedHeadSHA),
			valueOrUnknown(result.HeadConstraint.ObservedPreMergeHeadSHA),
			valueOrUnknown(result.HeadConstraint.ObservedPostMergeHeadSHA),
			valueOrUnknown(result.HeadConstraint.PinMechanism),
			valueOrUnknown(result.HeadConstraint.MismatchReason),
		)
	}
	if result.MergeRequestStatus != "" {
		output += fmt.Sprintf("merge request: %s\n", result.MergeRequestStatus)
	}
	for _, warning := range result.Warnings {
		output = "WARNING: " + warning + "\n" + output
	}
	return output
}
