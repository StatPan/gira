package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderTicketReviewHTMLEscapesUnsafeTextAndShowsReviewPacket(t *testing.T) {
	report := AgentPromptReport{
		Command: "ticket review",
		Repo:    "StatPan/gira",
		Ticket:  670,
		Role:    AgentPromptRoleReviewer,
		Profile: AgentPromptProfileDefault,
		Issue: AgentPromptIssue{
			Number:     670,
			Title:      `Review <script>alert("x")</script>`,
			State:      "open",
			Labels:     []string{`area:<ai>`, "status:in-review"},
			Goal:       "Review packet HTML",
			Scope:      `Render <safe> HTML`,
			Acceptance: []string{`Show <diff> summary`},
		},
		PR: &AgentPromptPR{
			Number:         674,
			Title:          `review <packet>`,
			URL:            "javascript:alert(1)",
			State:          "OPEN",
			HeadRefName:    `feature/<x>`,
			BaseRefName:    "main",
			ReviewDecision: "CHANGES_REQUESTED",
			MergeState:     "CLEAN",
			Blockers:       []string{`review <blocked>`},
			Checks:         []DevPRCheck{{Name: `ci <x>`, State: "passing", URL: "javascript:evil()"}},
			ChangedFiles:   []string{`internal/<unsafe>.go`},
			FinishReady:    true,
		},
		PRReady: &PRReadinessReport{
			SchemaVersion: PRReadinessSchemaVersion,
			Repo:          "StatPan/gira",
			Issue:         670,
			PullRequest:   674,
			Readiness:     "needs_revision",
			NextAction:    "revise_pr",
			Findings: []PRReadinessFinding{
				{Severity: "error", Kind: "review_blocked", Message: `Review <blocked>`, RecommendedAction: `Address <review>`},
			},
		},
		Review: &AgentReviewContract{
			DiffReferences: []AgentReviewReference{{Kind: "diff", Command: "gh pr diff 674 --repo StatPan/gira"}},
			DiffSummary: &AgentReviewDiffSummary{
				ChangedFiles:   []string{`internal/<unsafe>.go`},
				Files:          []AgentReviewDiffFile{{Path: `internal/<unsafe>.go`, Additions: 3, Deletions: 1, Hunks: []string{`@@ <hunk> @@`}}},
				TotalAdditions: 3,
				TotalDeletions: 1,
				AcceptanceMapping: []AgentReviewAcceptanceHint{
					{Criterion: `Show <diff> summary`, Files: []string{`internal/<unsafe>.go`}, Reason: `filename <match>`},
				},
				RiskAreas:       []string{`internal/<risk>`},
				FullDiffCommand: "gh pr diff 674 --repo StatPan/gira",
				FullDiff:        "diff --git a/x b/x\n+<unsafe>\n",
			},
			Guidance: []AgentPromptGuidance{{Path: "AGENTS.md", Status: "found"}},
			VerdictSchema: AgentReviewVerdictSchema{
				GoalFulfilled:            []string{"yes", "no"},
				AcceptanceCriteriaStatus: []string{"satisfied", "missing"},
				ChecksStatus:             []string{"passed", "failed"},
				EvidenceStatus:           []string{"sufficient", "missing"},
				ResidualRisk:             []string{"low", "high"},
				RecommendedAction:        []string{"approve", "request_changes"},
			},
		},
		Prompt:   "# Gira reviewer prompt\n\nInspect <diff>\n",
		NextStep: "gira ticket finish <unsafe>",
	}

	html := RenderTicketReviewHTML(report)
	for _, bad := range []string{`<script>alert`, `area:<ai>`, `javascript:alert`, `javascript:evil`, `Inspect <diff>`, `Review <blocked>`} {
		if strings.Contains(html, bad) {
			t.Fatalf("HTML contains unsafe raw value %q:\n%s", bad, html)
		}
	}
	for _, want := range []string{
		"Gira review packet",
		`Review &lt;script&gt;alert`,
		`area:&lt;ai&gt;`,
		`gira ticket finish &lt;unsafe&gt;`,
		"CHANGES_REQUESTED",
		"review_blocked",
		`Show &lt;diff&gt; summary`,
		`internal/&lt;unsafe&gt;.go`,
		`Inspect &lt;diff&gt;`,
		PRReadinessSchemaVersion,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("HTML missing %q:\n%s", want, html)
		}
	}
}

func TestWriteTicketReviewHTMLWritesSafeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reports", "review-674.html")
	report := AgentPromptReport{
		Command:  "ticket review",
		Repo:     "StatPan/gira",
		Ticket:   670,
		Role:     AgentPromptRoleReviewer,
		Profile:  AgentPromptProfileDefault,
		Issue:    AgentPromptIssue{Number: 670, Title: "Review packet", State: "open"},
		PR:       &AgentPromptPR{Number: 674, State: "OPEN", FinishReady: true},
		PRReady:  &PRReadinessReport{SchemaVersion: PRReadinessSchemaVersion, Repo: "StatPan/gira", Issue: 670, PullRequest: 674, Readiness: "ready_for_finish", NextAction: "finish_ticket"},
		Review:   &AgentReviewContract{VerdictSchema: AgentReviewVerdictSchema{RecommendedAction: []string{"approve", "request_changes"}}},
		Prompt:   "# Gira reviewer prompt\n",
		NextStep: "gira ticket finish --apply",
	}
	if err := WriteTicketReviewHTML(path, report); err != nil {
		t.Fatalf("WriteTicketReviewHTML error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ticket review HTML: %v", err)
	}
	for _, want := range []string{"Gira review packet", "Review packet", "ready_for_finish", PRReadinessSchemaVersion} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("written report missing %q:\n%s", want, string(got))
		}
	}
}
