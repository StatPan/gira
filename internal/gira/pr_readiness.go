package gira

import (
	"fmt"
	"strings"
)

const PRReadinessSchemaVersion = "pr-readiness/v1"

type PRReadinessReport struct {
	SchemaVersion string                   `json:"schema_version"`
	Repo          string                   `json:"repo"`
	Issue         int                      `json:"issue"`
	PullRequest   int                      `json:"pull_request,omitempty"`
	Readiness     string                   `json:"readiness"`
	Policy        *ResolvedOperationPolicy `json:"policy,omitempty"`
	Findings      []PRReadinessFinding     `json:"findings"`
	NextAction    string                   `json:"next_action"`
}

type PRReadinessFinding struct {
	Severity          string `json:"severity"`
	Kind              string `json:"kind"`
	FindingClass      string `json:"finding_class"`
	Enforced          bool   `json:"enforced"`
	Message           string `json:"message"`
	RecommendedAction string `json:"recommended_action"`
}

type prReadinessInput struct {
	Repo              string
	Issue             int
	PullRequest       int
	PRAvailable       bool
	ClosingReference  bool
	BaseMismatch      bool
	IsDraft           bool
	ChecksStatus      string
	Checks            []DevPRCheck
	ReviewStatus      string
	ReviewDecision    string
	ReviewPolicy      *FinishReviewPolicy
	ReviewEvidence    *FinishReviewEvidence
	FinishReady       bool
	ChangedFiles      []string
	ChangedFilesKnown bool
	Telemetry         *TicketStatusTelemetry
	Acceptance        *TicketStatusAcceptance
	Policy            ResolvedOperationPolicy
}

func EvaluatePRReadinessFromStatus(status WorkStatusResult) PRReadinessReport {
	input := prReadinessInput{
		Repo:           status.Repo,
		Issue:          status.Issue,
		Policy:         compatibilityManagedRequiredPolicy(),
		ChecksStatus:   status.ChecksStatus,
		Checks:         append([]DevPRCheck(nil), status.Checks...),
		ReviewStatus:   status.ReviewStatus,
		ReviewPolicy:   status.ReviewPolicy,
		ReviewEvidence: status.ReviewEvidence,
		Telemetry:      status.Telemetry,
		Acceptance:     status.Acceptance,
	}
	if status.Policy != nil {
		input.Policy = *status.Policy
	}
	if status.PullRequest != nil {
		input.PRAvailable = status.PullRequest.Available
		input.PullRequest = status.PullRequest.Number
		input.IsDraft = status.PullRequest.IsDraft
		input.ReviewDecision = status.PullRequest.ReviewDecision
	}
	if status.Evidence != nil {
		input.ClosingReference = status.Evidence.ClosingReference
		input.FinishReady = status.Evidence.FinishReady
	}
	if status.BranchPolicy != nil {
		input.BaseMismatch = status.BranchPolicy.BaseMismatch
	}
	report := evaluatePRReadinessWithPolicy(input, input.Policy)
	if status.Policy == nil {
		report.Policy = nil
	}
	return report
}

func EvaluatePRReadinessFromAgentReview(report AgentPromptReport) PRReadinessReport {
	readiness := EvaluatePRReadinessFromAgentReviewWithPolicy(report, compatibilityManagedRequiredPolicy())
	readiness.Policy = nil
	return readiness
}

func EvaluatePRReadinessFromAgentReviewWithPolicy(report AgentPromptReport, policy ResolvedOperationPolicy) PRReadinessReport {
	input := prReadinessInput{
		Repo:       report.Repo,
		Issue:      report.Ticket,
		Policy:     policy,
		Telemetry:  ticketStatusTelemetry(report.Issue.Body, report.Issue.Labels),
		Acceptance: ticketStatusAcceptance(report.Issue.Body),
	}
	if report.PR != nil {
		input.PRAvailable = true
		input.PullRequest = report.PR.Number
		input.ClosingReference = hasClosingKeyword(report.PR.Body, report.Ticket)
		input.BaseMismatch = report.PR.BaseMismatch
		input.IsDraft = report.PR.IsDraft
		input.Checks = append([]DevPRCheck(nil), report.PR.Checks...)
		input.ChecksStatus = prReadinessChecksStatus(report.PR.Checks)
		input.ReviewStatus = prReadinessReviewStatus(report.PR.ReviewDecision, report.PR.Blockers, report.PR.Number)
		input.ReviewDecision = report.PR.ReviewDecision
		input.FinishReady = report.PR.FinishReady
		input.ChangedFiles = append([]string(nil), report.PR.ChangedFiles...)
		input.ChangedFilesKnown = true
	}
	return evaluatePRReadinessWithPolicy(input, policy)
}

func evaluatePRReadiness(input prReadinessInput) PRReadinessReport {
	if input.Policy.Source == "" && input.Policy.OperationMode == "" && input.Policy.DeliveryPolicy == "" {
		input.Policy = compatibilityManagedRequiredPolicy()
	}
	readiness := evaluatePRReadinessWithPolicy(input, input.Policy)
	readiness.Policy = nil
	return readiness
}

func evaluatePRReadinessWithPolicy(input prReadinessInput, policy ResolvedOperationPolicy) (report PRReadinessReport) {
	input.Policy = policy
	report = PRReadinessReport{
		SchemaVersion: PRReadinessSchemaVersion,
		Repo:          input.Repo,
		Issue:         input.Issue,
		PullRequest:   input.PullRequest,
		Readiness:     "ready_for_review",
		Policy:        &policy,
		Findings:      []PRReadinessFinding{},
		NextAction:    "request_review",
	}
	defer func() { finalizePRReadiness(&report, input) }()

	if !input.PRAvailable {
		report.Findings = append(report.Findings, prReadinessFinding(
			"error",
			"missing_linked_pr",
			"No linked pull request was found for this ticket.",
			"Create or link a PR with a closing reference before review.",
		))
		report.Readiness = "blocked"
		report.NextAction = "blocked"
		return report
	}

	if !input.ClosingReference {
		report.Findings = append(report.Findings, prReadinessFinding(
			"error",
			"missing_closing_link",
			fmt.Sprintf("PR body does not close ticket #%d.", input.Issue),
			fmt.Sprintf("Add a closing reference such as `Closes #%d` to the PR body.", input.Issue),
		))
	}
	if input.BaseMismatch {
		report.Findings = append(report.Findings, prReadinessFinding(
			"error",
			"base_mismatch",
			"PR base does not match the recorded ticket base.",
			"Retarget the PR or update the recorded base only after reviewing branch policy.",
		))
	}
	if input.IsDraft {
		report.Findings = append(report.Findings, prReadinessFinding(
			"error",
			"draft_pr",
			"PR is still a draft.",
			"Mark the PR ready for review after the implementation is reviewable.",
		))
	}

	switch input.ChecksStatus {
	case "failed":
		report.Findings = append(report.Findings, prReadinessFinding(
			"error",
			"checks_failing",
			"One or more checks are failing.",
			"Fix failing checks before requesting review or finish.",
		))
	case "pending":
		report.Findings = append(report.Findings, prReadinessFinding(
			"warning",
			"checks_pending",
			"One or more checks are still pending.",
			"Wait for checks to finish before deciding whether the PR can finish.",
		))
	case "unknown":
		report.Findings = append(report.Findings, prReadinessFinding(
			"error",
			"checks_unavailable",
			"Required PR check state is unavailable or unknown.",
			"Retry the PR check query and confirm every required check has an explicit result before finish.",
		))
	case "missing", "":
		report.Findings = append(report.Findings, prReadinessFinding(
			"warning",
			"checks_missing",
			"No PR checks are available.",
			"Confirm whether this repository expects CI checks for this change.",
		))
	}

	if input.ReviewPolicy != nil && input.ReviewPolicy.Value == FinishReviewPolicyNone {
		// The repository explicitly chose a non-blocking review policy.
	} else if input.ReviewEvidence != nil && input.ReviewEvidence.Blocker != "" {
		report.Findings = append(report.Findings, prReadinessFinding(
			"error",
			input.ReviewEvidence.Blocker,
			"Review evidence does not satisfy the configured finish policy.",
			input.ReviewEvidence.Remediation,
		))
	} else if input.ReviewStatus == "blocked" {
		report.Findings = append(report.Findings, prReadinessFinding(
			"error",
			"review_blocked",
			"Review state is blocking the PR.",
			"Address requested changes or review blockers before finish.",
		))
	} else if input.ReviewStatus == "missing" || input.ReviewStatus == "unknown" || strings.TrimSpace(input.ReviewDecision) == "" || strings.EqualFold(input.ReviewDecision, "REVIEW_REQUIRED") {
		severity, kind, action := "info", "missing_review", "Request review or capture reviewer judgment before finish if policy requires it."
		if input.ReviewPolicy != nil && input.ReviewPolicy.Value == FinishReviewPolicyMissing {
			severity, kind, action = "error", "review_policy_not_configured", "Set finish_review_policy: required or none in .gira/config.yaml."
		} else if input.ReviewPolicy != nil && input.ReviewPolicy.Value == FinishReviewPolicyRequired {
			severity, kind, action = "error", "review_required_but_absent", "Request an approving review for the current PR head."
		}
		report.Findings = append(report.Findings, prReadinessFinding(severity, kind, "PR has no approving review decision yet.", action))
	}

	if input.Telemetry != nil && input.Telemetry.Required && !input.Telemetry.Present {
		report.Findings = append(report.Findings, prReadinessFinding(
			"warning",
			"missing_telemetry",
			"AI Delivery Telemetry or Gira provenance is expected but missing.",
			"Add telemetry or provenance metadata before final handoff when required by the lane.",
		))
	}
	if input.Acceptance == nil || input.Acceptance.Status == "missing" || input.Acceptance.Status == "incomplete" {
		report.Findings = append(report.Findings, prReadinessFinding(
			"warning",
			"acceptance_unknown",
			"Acceptance criteria are missing, unchecked, or not mapped to evidence.",
			"Map acceptance criteria to evidence in the PR, review packet, or finish receipt.",
		))
	}
	if input.ChangedFilesKnown && len(input.ChangedFiles) == 0 {
		report.Findings = append(report.Findings, prReadinessFinding(
			"warning",
			"changed_files_missing",
			"Changed file list is unavailable in the review packet.",
			"Provide changed files or diff commands so reviewers can inspect the PR.",
		))
	}

	switch {
	case hasPRReadinessSeverity(report.Findings, "error"):
		report.Readiness = "needs_revision"
		report.NextAction = "revise_pr"
	case hasPRReadinessFinding(report.Findings, "checks_pending"):
		report.Readiness = "blocked"
		report.NextAction = "wait_checks"
	case input.FinishReady && input.ChecksStatus == "passed" && !hasPRReadinessFinding(report.Findings, "missing_telemetry"):
		report.Readiness = "ready_for_finish"
		report.NextAction = "finish_ticket"
	default:
		report.Readiness = "ready_for_review"
		report.NextAction = "request_review"
	}
	return report
}

func finalizePRReadiness(report *PRReadinessReport, input prReadinessInput) {
	if report == nil {
		return
	}
	enforceManaged := input.Policy.RequiresManagedDelivery()
	providerBlocker := false
	enforcedManagedBlocker := false
	for i := range report.Findings {
		finding := &report.Findings[i]
		if isPRManagedPolicyFinding(finding.Kind) {
			finding.FindingClass = ReviewFindingClassPolicy
			finding.Enforced = enforceManaged
			// An explicitly configured finish-review policy remains authoritative
			// even when operation delivery is advisory.
			if (finding.Kind == "review_required_but_absent" || finding.Kind == "review_policy_not_configured") && input.ReviewPolicy != nil && input.ReviewPolicy.Value == FinishReviewPolicyRequired {
				finding.Enforced = true
			}
			if finding.Severity == "error" {
				if finding.Enforced {
					enforcedManagedBlocker = true
				} else {
					finding.Severity = "warning"
				}
			}
			continue
		}
		finding.FindingClass = ReviewFindingClassProvider
		finding.Enforced = true
		if finding.Severity == "error" {
			providerBlocker = true
		}
	}
	if !providerBlocker && !enforcedManagedBlocker && !hasPRReadinessFinding(report.Findings, "checks_pending") {
		if report.Readiness == "needs_revision" {
			report.Readiness = "ready_for_review"
			report.NextAction = "request_review"
		}
	}
}

func isPRManagedPolicyFinding(kind string) bool {
	switch kind {
	case "missing_closing_link", "missing_review", "review_policy_not_configured", "review_required_but_absent", "missing_telemetry", "acceptance_unknown", "changed_files_missing":
		return true
	default:
		return false
	}
}

func prReadinessFinding(severity string, kind string, message string, action string) PRReadinessFinding {
	return PRReadinessFinding{
		Severity:          severity,
		Kind:              kind,
		Message:           message,
		RecommendedAction: action,
	}
}

func prReadinessChecksStatus(checks []DevPRCheck) string {
	if len(checks) == 0 {
		return "unknown"
	}
	for _, check := range checks {
		if strings.EqualFold(strings.TrimSpace(check.Conclusion), "skipped") {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(check.State), "failing") {
			return "failed"
		}
		if strings.EqualFold(strings.TrimSpace(check.State), "pending") {
			return "pending"
		}
		if strings.TrimSpace(check.State) == "" || strings.EqualFold(strings.TrimSpace(check.State), "unknown") {
			return "unknown"
		}
	}
	return "passed"
}

func prReadinessReviewStatus(decision string, blockers []string, prNumber int) string {
	if prNumber == 0 {
		return "missing"
	}
	if containsString(blockers, "review") {
		return "blocked"
	}
	if strings.EqualFold(decision, "APPROVED") {
		return "approved"
	}
	if strings.TrimSpace(decision) == "" || strings.EqualFold(decision, "REVIEW_REQUIRED") {
		return "unknown"
	}
	return strings.ToLower(decision)
}

func hasPRReadinessSeverity(findings []PRReadinessFinding, severity string) bool {
	for _, finding := range findings {
		if strings.EqualFold(finding.Severity, severity) {
			return true
		}
	}
	return false
}

func hasPRReadinessFinding(findings []PRReadinessFinding, kind string) bool {
	for _, finding := range findings {
		if finding.Kind == kind {
			return true
		}
	}
	return false
}

func formatPRReadinessHuman(report PRReadinessReport) string {
	if report.SchemaVersion == "" {
		return ""
	}
	errors := []string{}
	warnings := []string{}
	for _, finding := range report.Findings {
		switch finding.Severity {
		case "error":
			errors = append(errors, finding.Message)
		case "warning":
			warnings = append(warnings, finding.Message)
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "PR Readiness: %s\n", strings.ReplaceAll(report.Readiness, "_", "-"))
	if len(errors) > 0 {
		b.WriteString("PR Blockers:\n")
		for _, item := range errors {
			fmt.Fprintf(&b, "- %s\n", item)
		}
	}
	if len(warnings) > 0 {
		b.WriteString("PR Warnings:\n")
		for _, item := range warnings {
			fmt.Fprintf(&b, "- %s\n", item)
		}
	}
	if strings.TrimSpace(report.NextAction) != "" {
		fmt.Fprintf(&b, "PR Suggested next action: %s\n", report.NextAction)
	}
	return b.String()
}
