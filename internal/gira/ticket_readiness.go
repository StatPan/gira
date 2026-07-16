package gira

import (
	"fmt"
	"strings"
)

const TicketReadinessSchemaVersion = "ticket-readiness/v1"

type TicketReadinessReport struct {
	SchemaVersion string                   `json:"schema_version"`
	Readiness     string                   `json:"readiness"`
	Findings      []TicketReadinessFinding `json:"findings"`
	NextAction    string                   `json:"next_action"`
}

type TicketReadinessFinding struct {
	Severity          string `json:"severity"`
	Kind              string `json:"kind"`
	Message           string `json:"message"`
	RecommendedAction string `json:"recommended_action"`
}

func EvaluateTicketReadiness(body string, labels []string, issueState string) TicketReadinessReport {
	report := TicketReadinessReport{
		SchemaVersion: TicketReadinessSchemaVersion,
		Readiness:     "ready",
		Findings:      []TicketReadinessFinding{},
		NextAction:    "start_ticket",
	}

	if strings.EqualFold(strings.TrimSpace(issueState), "closed") {
		report.Findings = append(report.Findings, ticketReadinessFinding(
			"info",
			"closed_ticket",
			"Ticket is closed.",
			"Inspect closure evidence instead of starting new work.",
		))
		report.Readiness = "unknown"
		report.NextAction = "ask_human"
		return report
	}

	typeLabels := labelsWithPrefix(labels, "type:")
	if len(typeLabels) == 0 {
		report.Findings = append(report.Findings, ticketReadinessFinding(
			"error",
			"missing_type_label",
			"Ticket is missing a type label.",
			"Add a managed type label such as type:task, type:story, type:bug, or type:spike.",
		))
	}
	if len(typeLabels) > 1 {
		report.Findings = append(report.Findings, ticketReadinessFinding(
			"warning",
			"multiple_type_labels",
			fmt.Sprintf("Ticket has multiple type labels: %s.", strings.Join(typeLabels, ",")),
			"Keep one primary type label before worker handoff.",
		))
	}

	statusLabels := managedStatusLabels(labels)
	if len(statusLabels) == 0 {
		report.Findings = append(report.Findings, ticketReadinessFinding(
			"error",
			"missing_status_label",
			"Ticket is missing an active status label.",
			"Add exactly one managed status label before worker handoff.",
		))
	}
	if len(statusLabels) > 1 {
		report.Findings = append(report.Findings, ticketReadinessFinding(
			"error",
			"multiple_status_labels",
			fmt.Sprintf("Ticket has multiple status labels: %s.", strings.Join(statusLabels, ",")),
			"Keep exactly one managed status label before worker handoff.",
		))
	}
	if hasTicketReadinessBlockedLabel(labels) {
		report.Findings = append(report.Findings, ticketReadinessFinding(
			"error",
			"blocked_ticket",
			"Ticket is blocked.",
			"Resolve the blocker or ask a human before starting work.",
		))
		report.Readiness = "blocked"
		report.NextAction = "blocked"
		return report
	}

	if profile := PMTaskProfileFromBody(body); profile != "" {
		profileReadiness := EvaluatePMProfileReadiness(body)
		report.Findings = append(report.Findings, profileReadiness.Findings...)
		if hasTicketReadinessSeverity(report.Findings, "error") {
			report.Readiness = "needs_refinement"
			report.NextAction = profileReadiness.NextAction
		}
		return report
	}

	if emptyReadinessSection(markdownSection(body, "Goal")) {
		report.Findings = append(report.Findings, ticketReadinessFinding(
			"error",
			"missing_goal",
			"Goal is missing or empty.",
			"Add a concrete goal that describes the intended outcome.",
		))
	}

	if emptyReadinessSection(markdownSection(body, "Scope")) && !ticketReadinessAllowsLightweightScope(labels, body) {
		report.Findings = append(report.Findings, ticketReadinessFinding(
			"error",
			"missing_scope",
			"Scope is missing or `_No response_`.",
			"Add the included work, explicit non-goals, or state why this ticket is intentionally lightweight.",
		))
	}

	acceptance := ticketReadinessAcceptanceItems(body)
	if len(acceptance) == 0 {
		report.Findings = append(report.Findings, ticketReadinessFinding(
			"error",
			"missing_acceptance",
			"Acceptance Criteria are missing or empty.",
			"Add measurable acceptance criteria before worker handoff.",
		))
	}

	if ticketReadinessNeedsDoctorImpact(body) && emptyReadinessSection(markdownSection(body, "Doctor Impact")) {
		report.Findings = append(report.Findings, ticketReadinessFinding(
			"error",
			"missing_doctor_impact",
			"Doctor Impact is missing for workflow/status/readiness-sensitive work.",
			"State whether doctor, audit, status, finish, branch policy, provider, or workflow reports need updates, or explicitly say no impact.",
		))
	}

	if ticketReadinessNeedsExpectedDelivery(labels, body) && !ticketReadinessHasExpectedDelivery(body) {
		report.Findings = append(report.Findings, ticketReadinessFinding(
			"warning",
			"missing_expected_delivery",
			"Expected delivery, review, or merge mode is not described.",
			"Add delivery/review/merge expectations for AI-assisted work.",
		))
	}

	if len(acceptance) > 0 && !ticketReadinessHasEvidenceExpectation(body) {
		report.Findings = append(report.Findings, ticketReadinessFinding(
			"warning",
			"weak_evidence",
			"Evidence expectations are missing or unclear.",
			"Add expected tests, docs, review packet, telemetry, finish receipt, or an explicit no-op.",
		))
	}

	if hasTicketReadinessSeverity(report.Findings, "error") {
		report.Readiness = "needs_refinement"
		report.NextAction = "refine_ticket"
	}
	return report
}

func ticketReadinessFinding(severity string, kind string, message string, action string) TicketReadinessFinding {
	return TicketReadinessFinding{
		Severity:          severity,
		Kind:              kind,
		Message:           message,
		RecommendedAction: action,
	}
}

func emptyReadinessSection(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return true
	}
	return strings.EqualFold(trimmed, "_No response_")
}

func ticketReadinessAcceptanceItems(body string) []string {
	section := markdownSection(body, "Acceptance Criteria")
	items := []string{}
	for _, line := range strings.Split(section, "\n") {
		item := ticketReadinessListItem(line)
		if item == "" || strings.EqualFold(item, "_No response_") {
			continue
		}
		items = append(items, item)
	}
	return items
}

func ticketReadinessListItem(line string) string {
	trimmed := strings.TrimSpace(line)
	for _, prefix := range []string{"- [ ] ", "- [x] ", "- [X] ", "* [ ] ", "* [x] ", "* [X] ", "- ", "* "} {
		if strings.HasPrefix(trimmed, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		}
	}
	return ""
}

func ticketReadinessNeedsDoctorImpact(body string) bool {
	lower := strings.ToLower(markdownBodyWithoutSection(body, "Doctor Impact"))
	for _, needle := range []string{
		"gira status",
		"ticket status",
		"doctor",
		"audit",
		"finish",
		"branch policy",
		"provider",
		"workflow report",
		"workflow reports",
		"readiness",
	} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func markdownBodyWithoutSection(body string, heading string) string {
	lines := strings.Split(body, "\n")
	target := strings.ToLower(strings.TrimSpace(heading))
	inSection := false
	out := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			current := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")))
			if current == target {
				inSection = true
				continue
			}
			inSection = false
		}
		if !inSection {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func ticketReadinessNeedsExpectedDelivery(labels []string, body string) bool {
	if aiDeliveryTelemetryRequired(labels) {
		return true
	}
	for _, label := range labels {
		if strings.EqualFold(strings.TrimSpace(label), "area:ai") {
			return true
		}
	}
	lower := strings.ToLower(body)
	return strings.Contains(lower, "llm") || strings.Contains(lower, "worker") || strings.Contains(lower, "agent")
}

func ticketReadinessHasExpectedDelivery(body string) bool {
	lower := strings.ToLower(body)
	for _, needle := range []string{
		"expected delivery",
		"delivery mode",
		"review mode",
		"merge mode",
		"review expectation",
		"review expectations",
		"worker handoff",
	} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func ticketReadinessHasEvidenceExpectation(body string) bool {
	for _, heading := range []string{"Verification Commands", "Expected Evidence", "Evidence Expectations", "Test Plan"} {
		if !emptyReadinessSection(markdownSection(body, heading)) {
			return true
		}
	}
	if !emptyReadinessSection(markdownSection(body, "Doctor Impact")) {
		return true
	}
	lower := strings.ToLower(body)
	for _, needle := range []string{"go test", "pytest", "verification", "evidence", "finish receipt", "review packet", "telemetry"} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func ticketReadinessAllowsLightweightScope(labels []string, body string) bool {
	for _, label := range labels {
		lower := strings.ToLower(strings.TrimSpace(label))
		if lower == "type:chore" {
			return true
		}
	}
	lowerBody := strings.ToLower(body)
	return strings.Contains(lowerBody, "intentionally lightweight") || strings.Contains(lowerBody, "lightweight ticket")
}

func hasTicketReadinessBlockedLabel(labels []string) bool {
	for _, label := range labels {
		lower := strings.ToLower(strings.TrimSpace(label))
		if lower == "blocked" || lower == "status:blocked" {
			return true
		}
	}
	return false
}

func labelsWithPrefix(labels []string, prefix string) []string {
	out := []string{}
	for _, label := range labels {
		trimmed := strings.TrimSpace(label)
		if strings.HasPrefix(strings.ToLower(trimmed), prefix) {
			out = append(out, trimmed)
		}
	}
	return out
}

func hasTicketReadinessSeverity(findings []TicketReadinessFinding, severity string) bool {
	for _, finding := range findings {
		if strings.EqualFold(finding.Severity, severity) {
			return true
		}
	}
	return false
}

func formatTicketReadinessHuman(report TicketReadinessReport) string {
	if report.SchemaVersion == "" {
		return ""
	}
	missing := []string{}
	warnings := []string{}
	for _, finding := range report.Findings {
		switch finding.Severity {
		case "error":
			missing = append(missing, finding.Message)
		case "warning":
			warnings = append(warnings, finding.Message)
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Readiness: %s\n", strings.ReplaceAll(report.Readiness, "_", "-"))
	if len(missing) > 0 {
		b.WriteString("Missing:\n")
		for _, item := range missing {
			fmt.Fprintf(&b, "- %s\n", item)
		}
	}
	if len(warnings) > 0 {
		b.WriteString("Warnings:\n")
		for _, item := range warnings {
			fmt.Fprintf(&b, "- %s\n", item)
		}
	}
	if strings.TrimSpace(report.NextAction) != "" {
		fmt.Fprintf(&b, "Suggested next action: %s\n", report.NextAction)
	}
	return b.String()
}
