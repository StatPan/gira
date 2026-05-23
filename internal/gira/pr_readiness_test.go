package gira

import "testing"

func TestEvaluatePRReadinessReadyForFinish(t *testing.T) {
	report := evaluatePRReadiness(prReadinessInput{
		Repo:             "StatPan/gira",
		Issue:            42,
		PullRequest:      43,
		PRAvailable:      true,
		ClosingReference: true,
		ChecksStatus:     "passed",
		Checks:           []DevPRCheck{{Name: "test", State: "passing"}},
		ReviewStatus:     "approved",
		ReviewDecision:   "APPROVED",
		FinishReady:      true,
		Acceptance:       &TicketStatusAcceptance{Status: "complete", Total: 1, Complete: 1},
	})

	if report.SchemaVersion != PRReadinessSchemaVersion || report.Readiness != "ready_for_finish" || report.NextAction != "finish_ticket" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("ready report should not have findings: %+v", report.Findings)
	}
}

func TestEvaluatePRReadinessDetectsBaseMismatch(t *testing.T) {
	report := evaluatePRReadiness(prReadinessInput{
		Repo:             "StatPan/gira",
		Issue:            42,
		PullRequest:      43,
		PRAvailable:      true,
		ClosingReference: true,
		BaseMismatch:     true,
		ChecksStatus:     "passed",
		ReviewStatus:     "approved",
		ReviewDecision:   "APPROVED",
		Acceptance:       &TicketStatusAcceptance{Status: "complete", Total: 1, Complete: 1},
	})

	if report.Readiness != "needs_revision" || report.NextAction != "revise_pr" || !prReadinessHasFinding(report, "base_mismatch") {
		t.Fatalf("expected base mismatch revision report: %+v", report)
	}
}

func TestEvaluatePRReadinessDetectsMissingClosingLink(t *testing.T) {
	report := evaluatePRReadiness(prReadinessInput{
		Repo:         "StatPan/gira",
		Issue:        42,
		PullRequest:  43,
		PRAvailable:  true,
		ChecksStatus: "passed",
		Acceptance:   &TicketStatusAcceptance{Status: "complete", Total: 1, Complete: 1},
	})

	if report.Readiness != "needs_revision" || !prReadinessHasFinding(report, "missing_closing_link") {
		t.Fatalf("expected missing closing link: %+v", report)
	}
}

func TestEvaluatePRReadinessDetectsFailingChecks(t *testing.T) {
	report := evaluatePRReadiness(prReadinessInput{
		Repo:             "StatPan/gira",
		Issue:            42,
		PullRequest:      43,
		PRAvailable:      true,
		ClosingReference: true,
		ChecksStatus:     "failed",
		Checks:           []DevPRCheck{{Name: "test", State: "failing"}},
		Acceptance:       &TicketStatusAcceptance{Status: "complete", Total: 1, Complete: 1},
	})

	if report.Readiness != "needs_revision" || !prReadinessHasFinding(report, "checks_failing") {
		t.Fatalf("expected failing checks: %+v", report)
	}
}

func TestEvaluatePRReadinessDetectsDraftPR(t *testing.T) {
	report := evaluatePRReadiness(prReadinessInput{
		Repo:             "StatPan/gira",
		Issue:            42,
		PullRequest:      43,
		PRAvailable:      true,
		ClosingReference: true,
		IsDraft:          true,
		ChecksStatus:     "passed",
		Acceptance:       &TicketStatusAcceptance{Status: "complete", Total: 1, Complete: 1},
	})

	if report.Readiness != "needs_revision" || !prReadinessHasFinding(report, "draft_pr") {
		t.Fatalf("expected draft PR finding: %+v", report)
	}
}

func TestEvaluatePRReadinessDetectsMissingTelemetryWarning(t *testing.T) {
	report := evaluatePRReadiness(prReadinessInput{
		Repo:             "StatPan/gira",
		Issue:            42,
		PullRequest:      43,
		PRAvailable:      true,
		ClosingReference: true,
		ChecksStatus:     "passed",
		ReviewStatus:     "approved",
		ReviewDecision:   "APPROVED",
		FinishReady:      true,
		Telemetry:        &TicketStatusTelemetry{Required: true, Present: false, Status: "missing"},
		Acceptance:       &TicketStatusAcceptance{Status: "complete", Total: 1, Complete: 1},
	})

	if report.Readiness != "ready_for_review" || report.NextAction != "request_review" || !prReadinessHasFinding(report, "missing_telemetry") {
		t.Fatalf("expected telemetry warning without finish readiness: %+v", report)
	}
}

func prReadinessHasFinding(report PRReadinessReport, kind string) bool {
	for _, finding := range report.Findings {
		if finding.Kind == kind {
			return true
		}
	}
	return false
}
