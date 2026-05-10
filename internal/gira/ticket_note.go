package gira

import (
	"fmt"
	"strings"
)

type TicketNoteInput struct {
	Repo   RepoRef `json:"-"`
	Ticket int     `json:"ticket"`
	Body   string  `json:"body"`
	Kind   string  `json:"kind"`
	Target string  `json:"target"`
	DryRun bool    `json:"dry_run"`
	Apply  bool    `json:"apply"`
}

type TicketNoteReport struct {
	Command      string           `json:"command"`
	Repo         string           `json:"repo"`
	Ticket       int              `json:"ticket"`
	Kind         string           `json:"kind"`
	Target       string           `json:"target"`
	Targets      []TicketNoteSink `json:"targets"`
	DryRun       bool             `json:"dry_run"`
	RenderedBody string           `json:"rendered_body"`
	Status       WorkStatusResult `json:"status"`
	NextStep     string           `json:"next_step"`
}

type TicketNoteSink struct {
	Type   string `json:"type"`
	Number int    `json:"number"`
	Status string `json:"status"`
}

func BuildTicketNoteReport(input TicketNoteInput, runner CommandRunner) (TicketNoteReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	kind, err := normalizeTicketNoteKind(input.Kind)
	if err != nil {
		return TicketNoteReport{}, err
	}
	target, err := normalizeTicketNoteTarget(input.Target)
	if err != nil {
		return TicketNoteReport{}, err
	}
	if input.Ticket <= 0 {
		return TicketNoteReport{}, fmt.Errorf("ticket must be > 0")
	}
	body := strings.TrimSpace(input.Body)
	if body == "" {
		return TicketNoteReport{}, fmt.Errorf("ticket note body is required")
	}
	if input.DryRun == input.Apply {
		return TicketNoteReport{}, fmt.Errorf("exactly one of dry_run/apply is required")
	}
	status, err := GetWorkStatus(input.Repo, input.Ticket, runner)
	report := TicketNoteReport{
		Command: "ticket note",
		Repo:    input.Repo.FullName(),
		Ticket:  input.Ticket,
		Kind:    kind,
		Target:  target,
		DryRun:  input.DryRun,
		Status:  status,
	}
	if err != nil {
		return report, err
	}
	status.NextStep = ticketLifecycleNextStep(status)
	report.Status = status
	targets, err := resolveTicketNoteTargets(target, status)
	if err != nil {
		report.NextStep = ticketNoteNextStep(report)
		return report, err
	}
	report.Targets = targets
	report.RenderedBody = RenderTicketNoteBody(kind, body, status)
	report.NextStep = ticketNoteNextStep(report)
	if input.DryRun {
		return report, nil
	}
	for i := range targets {
		sink := &targets[i]
		switch sink.Type {
		case "issue":
			_, err = runner.Run("gh", "issue", "comment", fmt.Sprintf("%d", sink.Number), "--repo", input.Repo.FullName(), "--body", report.RenderedBody)
		case "pr":
			_, err = runner.Run("gh", "pr", "comment", fmt.Sprintf("%d", sink.Number), "--repo", input.Repo.FullName(), "--body", report.RenderedBody)
		default:
			err = fmt.Errorf("unsupported ticket note target: %s", sink.Type)
		}
		if err != nil {
			sink.Status = "failed"
			report.Targets = targets
			return report, err
		}
		sink.Status = "applied"
	}
	report.Targets = targets
	report.NextStep = "gira ticket view"
	return report, nil
}

func RenderTicketNoteBody(kind string, body string, status WorkStatusResult) string {
	heading := ticketNoteHeading(kind)
	pr := "none"
	if status.PRNumber > 0 {
		pr = fmt.Sprintf("#%d", status.PRNumber)
	}
	blockers := strings.Join(status.Blockers, ",")
	if blockers == "" {
		blockers = "none"
	}
	next := ticketLifecycleNextStep(status)
	var b strings.Builder
	fmt.Fprintf(&b, "## %s\n\n", heading)
	b.WriteString(strings.TrimSpace(body))
	b.WriteString("\n\n")
	b.WriteString("Context:\n")
	fmt.Fprintf(&b, "- Ticket: #%d\n", status.Issue)
	fmt.Fprintf(&b, "- Status: %s\n", status.Status)
	fmt.Fprintf(&b, "- Linked PR: %s\n", pr)
	fmt.Fprintf(&b, "- Blockers: %s\n", blockers)
	fmt.Fprintf(&b, "- Next: %s\n", next)
	return b.String()
}

func FormatTicketNote(report TicketNoteReport) string {
	var targets []string
	for _, target := range report.Targets {
		status := strings.TrimSpace(target.Status)
		if status == "" {
			status = "planned"
		}
		targets = append(targets, fmt.Sprintf("%s#%d:%s", target.Type, target.Number, status))
	}
	if len(targets) == 0 {
		targets = append(targets, "none")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "ticket note: ticket #%d kind=%s target=%s dry_run=%t\n", report.Ticket, report.Kind, strings.Join(targets, ","), report.DryRun)
	b.WriteString("body:\n")
	b.WriteString(report.RenderedBody)
	if !strings.HasSuffix(report.RenderedBody, "\n") {
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}

func normalizeTicketNoteKind(kind string) (string, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		kind = "progress"
	}
	switch kind {
	case "progress", "blocker", "decision", "handoff", "summary", "check":
		return kind, nil
	default:
		return "", fmt.Errorf("unsupported ticket note kind %q; use progress, blocker, decision, handoff, summary, or check", kind)
	}
}

func normalizeTicketNoteTarget(target string) (string, error) {
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		target = "auto"
	}
	switch target {
	case "auto", "issue", "pr", "both":
		return target, nil
	default:
		return "", fmt.Errorf("unsupported ticket note target %q; use auto, issue, pr, or both", target)
	}
}

func resolveTicketNoteTargets(target string, status WorkStatusResult) ([]TicketNoteSink, error) {
	switch target {
	case "issue":
		return []TicketNoteSink{{Type: "issue", Number: status.Issue, Status: "planned"}}, nil
	case "pr":
		if status.PRNumber <= 0 {
			return nil, fmt.Errorf("ticket #%d has no linked PR; use --target issue or open a PR first", status.Issue)
		}
		return []TicketNoteSink{{Type: "pr", Number: status.PRNumber, Status: "planned"}}, nil
	case "both":
		if status.PRNumber <= 0 {
			return nil, fmt.Errorf("ticket #%d has no linked PR; use --target issue or open a PR first", status.Issue)
		}
		return []TicketNoteSink{{Type: "issue", Number: status.Issue, Status: "planned"}, {Type: "pr", Number: status.PRNumber, Status: "planned"}}, nil
	default:
		if status.PRNumber > 0 && strings.EqualFold(status.Status, "In review") {
			return []TicketNoteSink{{Type: "pr", Number: status.PRNumber, Status: "planned"}}, nil
		}
		return []TicketNoteSink{{Type: "issue", Number: status.Issue, Status: "planned"}}, nil
	}
}

func ticketNoteHeading(kind string) string {
	switch kind {
	case "blocker":
		return "Blocker"
	case "decision":
		return "Decision"
	case "handoff":
		return "Handoff"
	case "summary":
		return "Summary"
	case "check":
		return "Check"
	default:
		return "Progress Update"
	}
}

func ticketNoteNextStep(report TicketNoteReport) string {
	if report.DryRun {
		return "gira ticket note --apply"
	}
	return "gira ticket view"
}
