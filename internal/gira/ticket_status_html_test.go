package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderTicketStatusHTMLEscapesUnsafeTextAndShowsReviewState(t *testing.T) {
	status := WorkStatusResult{
		Command:       "ticket status",
		SchemaVersion: TicketStatusSchemaVersion,
		Repo:          "StatPan/gira",
		Issue:         669,
		Title:         `Detail <script>alert("x")</script>`,
		State:         "open",
		Status:        "In review",
		Labels:        []string{`type:<task>`, "status:in-review"},
		Milestone:     `3 <next>`,
		Blockers:      []string{`review <blocked>`},
		NextAction:    "address_review",
		NextStep:      "gira ticket review <unsafe>",
		Branch:        &TicketStatusBranch{Expected: `issue-669-<x>`, Current: `feature/<x>`, Trusted: true, Source: "closing_reference"},
		PullRequest: &TicketStatusPullRequest{
			Available:      true,
			Number:         670,
			URL:            "javascript:alert(1)",
			State:          "OPEN",
			Mergeable:      "UNKNOWN",
			HeadRefName:    `feature/<x>`,
			BaseRefName:    "main",
			ReviewDecision: "CHANGES_REQUESTED",
		},
		ChecksStatus: "failed",
		Checks: []DevPRCheck{
			{Name: `ci <x>`, State: "failing", Status: "completed", Conclusion: "failure", URL: "javascript:evil()"},
		},
		ReviewStatus: "blocked",
		Evidence:     &TicketStatusEvidence{ClosingReference: true, BranchTrusted: true, FinishReady: false, Sources: []string{"issue", "pull_request"}},
		Acceptance:   &TicketStatusAcceptance{Status: "incomplete", Total: 2, Complete: 1, Incomplete: 1},
		Telemetry:    &TicketStatusTelemetry{Required: true, Present: false, Status: "missing", Warnings: []string{`telemetry <missing>`}},
		TicketReadiness: &TicketReadinessReport{
			SchemaVersion: TicketReadinessSchemaVersion,
			Readiness:     "ready",
			NextAction:    "start_ticket",
			Findings: []TicketReadinessFinding{
				{Severity: "warning", Kind: "weak_evidence", Message: `Need <evidence>`, RecommendedAction: `Map <tests>`},
			},
		},
		PRReadiness: &PRReadinessReport{
			SchemaVersion: PRReadinessSchemaVersion,
			Repo:          "StatPan/gira",
			Issue:         669,
			PullRequest:   670,
			Readiness:     "needs_revision",
			NextAction:    "revise_pr",
			Findings: []PRReadinessFinding{
				{Severity: "error", Kind: "review_blocked", Message: `Review <blocked>`, RecommendedAction: `Address <review>`},
			},
		},
	}

	html := RenderTicketStatusHTML(status)
	for _, bad := range []string{`<script>alert`, `type:<task>`, `javascript:alert`, `javascript:evil`, `Need <evidence>`, `Review <blocked>`} {
		if strings.Contains(html, bad) {
			t.Fatalf("HTML contains unsafe raw value %q:\n%s", bad, html)
		}
	}
	for _, want := range []string{
		"Gira ticket report",
		`Detail &lt;script&gt;alert`,
		`type:&lt;task&gt;`,
		`gira ticket review &lt;unsafe&gt;`,
		"review: blocked",
		"CHANGES_REQUESTED",
		"review_blocked",
		`Need &lt;evidence&gt;`,
		TicketStatusSchemaVersion,
		PRReadinessSchemaVersion,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("HTML missing %q:\n%s", want, html)
		}
	}
}

func TestWriteTicketStatusHTMLWritesSafeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reports", "ticket-669.html")
	status := WorkStatusResult{
		Command:       "ticket status",
		SchemaVersion: TicketStatusSchemaVersion,
		Repo:          "StatPan/gira",
		Issue:         669,
		Title:         "Ticket detail",
		State:         "open",
		Status:        "Ready",
		NextAction:    "start_work",
		NextStep:      "gira ticket start 669 --apply",
	}
	if err := WriteTicketStatusHTML(path, status); err != nil {
		t.Fatalf("WriteTicketStatusHTML error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ticket status HTML: %v", err)
	}
	for _, want := range []string{"Gira ticket report", "Ticket detail", TicketStatusSchemaVersion} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("written report missing %q:\n%s", want, string(got))
		}
	}
}
