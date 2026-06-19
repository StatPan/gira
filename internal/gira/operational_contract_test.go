package gira

import "testing"

func TestOperationalContractAcceptsValidTicketStatus(t *testing.T) {
	status := WorkStatusResult{
		SchemaVersion: TicketStatusSchemaVersion,
		Repo:          "StatPan/gira",
		Issue:         767,
		State:         "open",
		Status:        "In review",
		PRNumber:      768,
		PullRequest:   &TicketStatusPullRequest{Available: true, Number: 768, State: "open"},
		ChecksStatus:  "passed",
		Checks:        []DevPRCheck{{Name: "ci", State: "passing"}},
		ReviewStatus:  "approved",
		NextAction:    "finish_ticket",
		Evidence:      &TicketStatusEvidence{ClosingReference: true, FinishReady: true},
		PRReadiness:   &PRReadinessReport{SchemaVersion: PRReadinessSchemaVersion, PullRequest: 768, Readiness: "ready_for_finish", NextAction: "finish_ticket"},
	}
	if findings := ValidateWorkStatusContract(status); len(findings) > 0 {
		t.Fatalf("unexpected findings: %+v", findings)
	}
}

func TestOperationalContractRejectsImpossibleTicketStatus(t *testing.T) {
	tests := []struct {
		name string
		in   WorkStatusResult
		code string
	}{
		{
			name: "passed checks without check evidence",
			in: WorkStatusResult{
				SchemaVersion: TicketStatusSchemaVersion,
				Repo:          "StatPan/gira",
				Issue:         1,
				State:         "open",
				PRNumber:      2,
				PullRequest:   &TicketStatusPullRequest{Available: true, Number: 2},
				ChecksStatus:  "passed",
				NextAction:    "request_review",
			},
			code: "passed_checks_require_check_evidence",
		},
		{
			name: "done on open issue",
			in: WorkStatusResult{
				SchemaVersion: TicketStatusSchemaVersion,
				Repo:          "StatPan/gira",
				Issue:         1,
				State:         "open",
				NextAction:    "done",
			},
			code: "done_requires_closed_issue",
		},
		{
			name: "finish action without closing reference",
			in: WorkStatusResult{
				SchemaVersion: TicketStatusSchemaVersion,
				Repo:          "StatPan/gira",
				Issue:         1,
				State:         "open",
				PRNumber:      2,
				PullRequest:   &TicketStatusPullRequest{Available: true, Number: 2},
				ChecksStatus:  "passed",
				Checks:        []DevPRCheck{{Name: "ci", State: "passing"}},
				ReviewStatus:  "approved",
				NextAction:    "merge_when_policy_allows",
			},
			code: "finish_action_requires_closing_reference",
		},
		{
			name: "failed summary without failure evidence",
			in: WorkStatusResult{
				SchemaVersion: TicketStatusSchemaVersion,
				Repo:          "StatPan/gira",
				Issue:         1,
				State:         "open",
				PRNumber:      2,
				PullRequest:   &TicketStatusPullRequest{Available: true, Number: 2},
				ChecksStatus:  "failed",
				Checks:        []DevPRCheck{{Name: "ci", State: "passing"}},
				NextAction:    "revise_pr",
			},
			code: "failed_checks_require_failure_evidence",
		},
		{
			name: "pr number marked unavailable",
			in: WorkStatusResult{
				SchemaVersion: TicketStatusSchemaVersion,
				Repo:          "StatPan/gira",
				Issue:         1,
				State:         "open",
				PRNumber:      2,
				PullRequest:   &TicketStatusPullRequest{Available: false, Number: 2},
				NextAction:    "request_review",
			},
			code: "pr_number_without_available_pull_request",
		},
		{
			name: "unknown lifecycle action",
			in: WorkStatusResult{
				SchemaVersion: TicketStatusSchemaVersion,
				Repo:          "StatPan/gira",
				Issue:         1,
				State:         "open",
				NextAction:    "ship_magic",
			},
			code: "unsupported_next_action",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := ValidateWorkStatusContract(tt.in)
			if !hasOperationalContractFinding(findings, tt.code) {
				t.Fatalf("findings = %+v, want code %q", findings, tt.code)
			}
		})
	}
}

func hasOperationalContractFinding(findings []OperationalContractFinding, code string) bool {
	for _, finding := range findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}
