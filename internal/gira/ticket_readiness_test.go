package gira

import "testing"

func TestEvaluateTicketReadinessReportsMissingWorkOrderFields(t *testing.T) {
	body := "## Goal\nShip readiness\n\n## Scope\n_No response_\n\n## Acceptance Criteria\n_No response_\n"
	report := EvaluateTicketReadiness(body, []string{"type:task", "status:ready"}, "open")

	if report.SchemaVersion != TicketReadinessSchemaVersion || report.Readiness != "needs_refinement" || report.NextAction != "refine_ticket" {
		t.Fatalf("unexpected readiness report: %+v", report)
	}
	for _, want := range []string{"missing_scope", "missing_acceptance"} {
		if !ticketReadinessHasFinding(report, want) {
			t.Fatalf("missing finding %q: %+v", want, report.Findings)
		}
	}
}

func TestEvaluateTicketReadinessReportsMissingLabels(t *testing.T) {
	body := "## Goal\nShip readiness\n\n## Scope\nCLI only\n\n## Acceptance Criteria\n- emits JSON\n"
	report := EvaluateTicketReadiness(body, nil, "open")

	for _, want := range []string{"missing_type_label", "missing_status_label"} {
		if !ticketReadinessHasFinding(report, want) {
			t.Fatalf("missing finding %q: %+v", want, report.Findings)
		}
	}
	if report.Readiness != "needs_refinement" {
		t.Fatalf("readiness = %q, want needs_refinement", report.Readiness)
	}
}

func TestEvaluateTicketReadinessRequiresDoctorImpactForWorkflowWork(t *testing.T) {
	body := "## Goal\nExpose ticket status readiness\n\n## Scope\nCLI JSON\n\n## Acceptance Criteria\n- ticket status includes readiness\n\n## Doctor Impact\n_No response_\n"
	report := EvaluateTicketReadiness(body, []string{"type:task", "status:ready"}, "open")

	if !ticketReadinessHasFinding(report, "missing_doctor_impact") {
		t.Fatalf("expected doctor impact finding: %+v", report.Findings)
	}
}

func TestEvaluateTicketReadinessWarnsForAgentDeliveryAndEvidence(t *testing.T) {
	body := "## Goal\nPrepare AI worker packet\n\n## Scope\nPrompt output\n\n## Acceptance Criteria\n- worker gets JSON\n"
	report := EvaluateTicketReadiness(body, []string{"type:task", "status:ready", "area:ai"}, "open")

	if report.Readiness != "ready" {
		t.Fatalf("warnings should not block readiness: %+v", report)
	}
	for _, want := range []string{"missing_expected_delivery", "weak_evidence"} {
		if !ticketReadinessHasFinding(report, want) {
			t.Fatalf("missing warning %q: %+v", want, report.Findings)
		}
	}
}

func TestEvaluateTicketReadinessReadyTicket(t *testing.T) {
	body := "## Goal\nShip ticket readiness\n\n## Scope\nCLI status and ticket new output\n\n## Acceptance Criteria\n- emits ticket-readiness/v1\n- has tests\n\n## Doctor Impact\nUpdates status JSON only.\n\n## Expected Evidence\n- go test ./internal/gira\n\n## Expected Delivery\nOpen a draft PR for review.\n"
	report := EvaluateTicketReadiness(body, []string{"type:task", "status:ready", "area:backend"}, "open")

	if report.Readiness != "ready" || report.NextAction != "start_ticket" || len(report.Findings) != 0 {
		t.Fatalf("unexpected ready report: %+v", report)
	}
}

func TestEvaluateTicketReadinessBlockedTicket(t *testing.T) {
	body := "## Goal\nBlocked\n\n## Scope\nKnown\n\n## Acceptance Criteria\n- clear blocker\n"
	report := EvaluateTicketReadiness(body, []string{"type:task", "status:blocked"}, "open")

	if report.Readiness != "blocked" || report.NextAction != "blocked" || !ticketReadinessHasFinding(report, "blocked_ticket") {
		t.Fatalf("unexpected blocked report: %+v", report)
	}
}

func ticketReadinessHasFinding(report TicketReadinessReport, kind string) bool {
	for _, finding := range report.Findings {
		if finding.Kind == kind {
			return true
		}
	}
	return false
}
