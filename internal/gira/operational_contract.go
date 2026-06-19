package gira

import "strings"

const OperationalContractFindingsSchemaVersion = "operational-contract-findings/v1"

type OperationalContractFinding struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Contract string `json:"contract"`
	Message  string `json:"message"`
}

func ValidateWorkStatusContract(status WorkStatusResult) []OperationalContractFinding {
	findings := []OperationalContractFinding{}
	add := func(code, message string) {
		findings = append(findings, OperationalContractFinding{
			Severity: "error",
			Code:     code,
			Contract: TicketStatusSchemaVersion,
			Message:  message,
		})
	}

	if value := strings.TrimSpace(status.SchemaVersion); value != "" && value != TicketStatusSchemaVersion {
		add("unsupported_schema_version", "ticket status schema_version must be ticket-status/v1 when present")
	}
	if strings.TrimSpace(status.Repo) == "" {
		add("missing_repo", "ticket status must include repo")
	}
	if status.Issue <= 0 {
		add("missing_issue", "ticket status must include a positive issue number")
	}
	if state := normalizeContractValue(status.State); state != "" && !allowedContractValue(state, "open", "closed") {
		add("unsupported_issue_state", "ticket status state must be open or closed when present")
	}
	if nextAction := normalizeContractValue(status.NextAction); nextAction != "" && !knownWorkStatusNextAction(nextAction) {
		add("unsupported_next_action", "ticket status next_action is not a known lifecycle action")
	}
	if checksStatus := normalizeContractValue(status.ChecksStatus); checksStatus != "" && !allowedContractValue(checksStatus, "passed", "failed", "failing", "pending", "missing", "unknown") {
		add("unsupported_checks_status", "ticket status checks_status is not a known check summary")
	}
	if reviewStatus := normalizeContractValue(status.ReviewStatus); reviewStatus != "" && !allowedContractValue(reviewStatus, "approved", "blocked", "missing", "unknown", "review_required", "changes_requested", "commented") {
		add("unsupported_review_status", "ticket status review_status is not a known review summary")
	}

	for _, check := range status.Checks {
		if state := normalizeContractValue(check.State); state != "" && !allowedContractValue(state, "passing", "pending", "failing", "unknown") {
			add("unsupported_check_state", "ticket status checks contain an unknown check state")
		}
	}

	prAvailable := status.PullRequest != nil && status.PullRequest.Available
	if status.PRNumber > 0 && status.PullRequest != nil && !status.PullRequest.Available {
		add("pr_number_without_available_pull_request", "ticket status has pr_number but pull_request is marked unavailable")
	}
	if prAvailable && status.PullRequest.Number <= 0 {
		add("available_pull_request_without_number", "available pull_request must include a positive number")
	}
	if prAvailable && status.PRNumber > 0 && status.PullRequest.Number > 0 && status.PRNumber != status.PullRequest.Number {
		add("pull_request_number_mismatch", "pr_number and pull_request.number must match")
	}
	if prAvailable && status.PRReadiness != nil && status.PRReadiness.PullRequest > 0 && status.PullRequest.Number > 0 && status.PRReadiness.PullRequest != status.PullRequest.Number {
		add("pr_readiness_number_mismatch", "pull_request.number and pr_readiness.pull_request must match")
	}

	nextAction := normalizeContractValue(status.NextAction)
	if nextAction == "done" && !strings.EqualFold(strings.TrimSpace(status.State), "closed") {
		add("done_requires_closed_issue", "next_action=done requires a closed issue")
	}
	if nextAction == "merge_when_policy_allows" || nextAction == "finish_ticket" {
		if !hasWorkStatusPR(status) {
			add("finish_action_requires_pull_request", "finish next_action requires a linked pull request")
		}
		if status.Evidence == nil || !status.Evidence.ClosingReference {
			add("finish_action_requires_closing_reference", "finish next_action requires closing-reference evidence")
		}
	}

	checksStatus := normalizeContractValue(status.ChecksStatus)
	switch checksStatus {
	case "passed":
		if prAvailable && len(status.Checks) == 0 {
			add("passed_checks_require_check_evidence", "checks_status=passed requires check evidence for an available pull request")
		}
		if hasWorkStatusCheckState(status.Checks, "failing") || hasWorkStatusCheckState(status.Checks, "pending") {
			add("passed_checks_conflict_with_check_states", "checks_status=passed conflicts with failing or pending checks")
		}
	case "failed", "failing":
		if len(status.Checks) > 0 && !hasWorkStatusCheckState(status.Checks, "failing") && !hasWorkStatusCheckBlocker(status.Blockers) {
			add("failed_checks_require_failure_evidence", "checks_status=failed requires failing check evidence or a check blocker")
		}
	case "pending":
		if len(status.Checks) > 0 && !hasWorkStatusCheckState(status.Checks, "pending") && !hasWorkStatusBlocker(status.Blockers, "checks_pending") {
			add("pending_checks_require_pending_evidence", "checks_status=pending requires pending check evidence or a pending-check blocker")
		}
	}

	return findings
}

func knownWorkStatusNextAction(action string) bool {
	return allowedContractValue(action,
		"address_review",
		"ask_human",
		"blocked",
		"closed",
		"done",
		"finish_ticket",
		"fix_checks",
		"mark_pr_ready",
		"merge_when_policy_allows",
		"refine_ticket",
		"request_review",
		"resolve_blockers",
		"resolve_finish_blockers",
		"revise_pr",
		"start_ticket",
		"start_work",
		"wait_checks",
		"wait_for_checks",
	)
}

func hasWorkStatusPR(status WorkStatusResult) bool {
	return status.PRNumber > 0 || (status.PullRequest != nil && status.PullRequest.Number > 0)
}

func hasWorkStatusCheckState(checks []DevPRCheck, want string) bool {
	for _, check := range checks {
		if normalizeContractValue(check.State) == want {
			return true
		}
	}
	return false
}

func hasWorkStatusCheckBlocker(blockers []string) bool {
	for _, blocker := range blockers {
		value := normalizeContractValue(blocker)
		if strings.Contains(value, "check") {
			return true
		}
	}
	return false
}

func hasWorkStatusBlocker(blockers []string, want string) bool {
	for _, blocker := range blockers {
		if normalizeContractValue(blocker) == want {
			return true
		}
	}
	return false
}

func allowedContractValue(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func normalizeContractValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
