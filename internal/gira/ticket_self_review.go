package gira

import (
	"fmt"
	"strings"
)

const TicketSelfReviewReportSchemaVersion = "ticket-self-review-report/v1"

type TicketSelfReviewInput struct {
	Repo        RepoRef `json:"-"`
	Ticket      int     `json:"ticket"`
	PRNumber    int     `json:"pr_number,omitempty"`
	DiffSummary bool    `json:"diff_summary,omitempty"`
	DryRun      bool    `json:"dry_run"`
	Apply       bool    `json:"apply"`
}

type TicketSelfReviewReport struct {
	SchemaVersion string            `json:"schema_version"`
	Command       string            `json:"command"`
	Repo          string            `json:"repo"`
	Ticket        int               `json:"ticket"`
	PullRequest   int               `json:"pull_request,omitempty"`
	DiffSummary   bool              `json:"diff_summary"`
	DryRun        bool              `json:"dry_run"`
	Review        AgentPromptReport `json:"review"`
	Note          *TicketNoteReport `json:"note,omitempty"`
	RenderedBody  string            `json:"rendered_body,omitempty"`
	NextStep      string            `json:"next_step,omitempty"`
	Approval      *ApprovalEvidence `json:"approval,omitempty"`
}

func BuildTicketSelfReviewReport(input TicketSelfReviewInput, runner CommandRunner) (TicketSelfReviewReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if input.Ticket <= 0 {
		return TicketSelfReviewReport{}, fmt.Errorf("ticket must be > 0")
	}
	if input.DryRun == input.Apply {
		return TicketSelfReviewReport{}, fmt.Errorf("exactly one of dry_run/apply is required")
	}
	includeSummary := input.DiffSummary
	if !includeSummary {
		includeSummary = true
	}
	review, err := BuildAgentPromptReport(AgentPromptInput{
		Repo:               input.Repo,
		Ticket:             input.Ticket,
		Role:               AgentPromptRoleReviewer,
		Profile:            AgentPromptProfileDefault,
		PRNumber:           input.PRNumber,
		IncludeDiffSummary: includeSummary,
	}, runner)
	report := TicketSelfReviewReport{
		SchemaVersion: TicketSelfReviewReportSchemaVersion,
		Command:       "ticket self-review",
		Repo:          input.Repo.FullName(),
		Ticket:        input.Ticket,
		DiffSummary:   includeSummary,
		DryRun:        input.DryRun,
		Review:        review,
	}
	if err != nil {
		return report, err
	}
	if review.PR == nil || review.PR.Number <= 0 {
		report.NextStep = "gira ticket pr --dry-run"
		return report, fmt.Errorf("self-review requires a linked PR for ticket #%d; run `gira ticket pr --dry-run` first", input.Ticket)
	}
	report.PullRequest = review.PR.Number
	body := RenderTicketSelfReviewBody(review)
	note, err := BuildTicketNoteReport(TicketNoteInput{
		Repo:   input.Repo,
		Ticket: input.Ticket,
		Body:   body,
		Kind:   "check",
		Target: "pr",
		DryRun: input.DryRun,
		Apply:  input.Apply,
	}, runner)
	report.Note = &note
	report.RenderedBody = note.RenderedBody
	report.NextStep = note.NextStep
	if input.DryRun {
		report.Approval = TicketSelfReviewApprovalEvidence(report)
	}
	if err != nil {
		return report, err
	}
	return report, nil
}

func RenderTicketSelfReviewBody(review AgentPromptReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Self-review packet for ticket #%d", review.Ticket)
	if review.PR != nil && review.PR.Number > 0 {
		fmt.Fprintf(&b, " and PR #%d", review.PR.Number)
	}
	b.WriteString(".\n\n")
	if review.PRReady != nil {
		fmt.Fprintf(&b, "- PR readiness: %s\n", review.PRReady.Readiness)
	}
	if review.Evidence != nil {
		fmt.Fprintf(&b, "- Finish ready: %t\n", review.Evidence.FinishReady)
		if len(review.Evidence.Blockers) > 0 {
			fmt.Fprintf(&b, "- Blockers: %s\n", strings.Join(review.Evidence.Blockers, ", "))
		}
	}
	if review.Review != nil && review.Review.DiffSummary != nil {
		summary := review.Review.DiffSummary
		if strings.TrimSpace(summary.UnsupportedMessage) != "" {
			fmt.Fprintf(&b, "- Diff summary: unavailable: %s\n", summary.UnsupportedMessage)
		} else {
			fmt.Fprintf(&b, "- Diff summary: %d files changed, +%d/-%d\n", len(summary.ChangedFiles), summary.TotalAdditions, summary.TotalDeletions)
			for _, file := range summary.Files {
				fmt.Fprintf(&b, "  - `%s`: +%d/-%d\n", file.Path, file.Additions, file.Deletions)
			}
			if len(summary.AcceptanceMapping) > 0 {
				b.WriteString("- Acceptance mapping candidates:\n")
				for _, mapping := range summary.AcceptanceMapping {
					files := strings.Join(mapping.Files, ", ")
					if files == "" {
						files = "manual verification required"
					}
					fmt.Fprintf(&b, "  - %s -> %s\n", mapping.Criterion, files)
				}
			}
			if len(summary.RiskAreas) > 0 {
				fmt.Fprintf(&b, "- Risk areas: %s\n", strings.Join(summary.RiskAreas, ", "))
			}
		}
	}
	if review.PR != nil && review.PR.Number > 0 {
		fmt.Fprintf(&b, "- Full diff: `gh pr diff %d --repo %s`\n", review.PR.Number, review.Repo)
	}
	b.WriteString("\nReviewer notes:\n- Findings: none recorded by this self-review command.\n- Test gaps: verify local and CI evidence before finish.\n- Recommended action: continue normal review/check/finish flow.\n")
	return strings.TrimSpace(b.String())
}

func TicketSelfReviewApprovalEvidence(report TicketSelfReviewReport) *ApprovalEvidence {
	applyCommand := fmt.Sprintf("gira ticket self-review %d --repo %s --apply", report.Ticket, report.Repo)
	dryRunCommand := strings.Replace(applyCommand, " --apply", " --dry-run", 1)
	return &ApprovalEvidence{
		SchemaVersion:         ApprovalPlanSchemaVersion,
		Capability:            AdapterCapabilityApplyMutation,
		CanonicalCommand:      "gira ticket self-review",
		DryRunCommand:         dryRunCommand,
		ApplyCommand:          applyCommand,
		Repo:                  report.Repo,
		Issue:                 report.Ticket,
		OutputSchema:          TicketSelfReviewReportSchemaVersion,
		PlannedActions:        []ApprovalPlannedAction{{Action: "pr:comment", Target: fmt.Sprintf("#%d", report.PullRequest), Detail: "post self-review check note"}},
		Blockers:              []string{},
		Warnings:              []string{},
		PostApplyVerification: fmt.Sprintf("gira ticket review %d --repo %s --diff-summary --json", report.Ticket, report.Repo),
	}
}

func FormatTicketSelfReview(report TicketSelfReviewReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ticket self-review: ticket #%d", report.Ticket)
	if report.PullRequest > 0 {
		fmt.Fprintf(&b, " pr #%d", report.PullRequest)
	}
	fmt.Fprintf(&b, " dry_run=%t\n", report.DryRun)
	if strings.TrimSpace(report.RenderedBody) != "" {
		b.WriteString("body:\n")
		b.WriteString(report.RenderedBody)
		if !strings.HasSuffix(report.RenderedBody, "\n") {
			b.WriteString("\n")
		}
	}
	if strings.TrimSpace(report.NextStep) != "" {
		fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	}
	return b.String()
}
