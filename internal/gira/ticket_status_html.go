package gira

import (
	"fmt"
	"html"
	"strings"
)

const TicketStatusSchemaVersion = "ticket-status/v1"

func WriteTicketStatusHTML(path string, status WorkStatusResult) error {
	return writeSafeLocalFile(path, []byte(RenderTicketStatusHTML(status)), 0o644)
}

func RenderTicketStatusHTML(status WorkStatusResult) string {
	var b strings.Builder
	title := fmt.Sprintf("Ticket report #%d", status.Issue)
	if strings.TrimSpace(status.Title) != "" {
		title = fmt.Sprintf("%s - %s", title, status.Title)
	}

	b.WriteString("<!doctype html>\n")
	b.WriteString("<html lang=\"en\">\n")
	b.WriteString("<head>\n")
	b.WriteString("<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	fmt.Fprintf(&b, "<title>%s</title>\n", ticketStatusHTMLText(title))
	b.WriteString(`<style>
:root {
  color-scheme: light;
  --bg: #f6f7f9;
  --panel: #ffffff;
  --text: #20242a;
  --muted: #606b78;
  --line: #d8dde4;
  --accent: #1769aa;
  --accent-soft: #e7f2fb;
  --warn: #935f00;
  --warn-soft: #fff4d9;
  --danger: #9f2f2f;
  --danger-soft: #fdeaea;
}
* { box-sizing: border-box; }
body {
  margin: 0;
  background: var(--bg);
  color: var(--text);
  font: 14px/1.5 -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}
main { max-width: 1080px; margin: 0 auto; padding: 28px 18px 42px; }
header, section {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 8px;
  margin: 0 0 14px;
  padding: 18px;
}
h1, h2, h3, p { margin: 0; }
h1 { font-size: 24px; line-height: 1.2; }
h2 { font-size: 16px; margin-bottom: 10px; }
h3 { font-size: 14px; margin-bottom: 8px; }
a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }
code {
  display: inline-block;
  max-width: 100%;
  overflow-wrap: anywhere;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: #f2f4f7;
  padding: 2px 6px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
}
table { width: 100%; border-collapse: collapse; }
th, td { border-top: 1px solid var(--line); padding: 7px 6px; text-align: left; vertical-align: top; }
th { color: var(--muted); font-weight: 600; }
.eyebrow { color: var(--muted); font-size: 12px; text-transform: uppercase; margin-bottom: 4px; }
.summary { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 14px; }
.pill {
  border: 1px solid var(--line);
  border-radius: 999px;
  padding: 3px 9px;
  background: #fbfcfd;
  color: var(--muted);
}
.next {
  border-color: #b9d8ef;
  background: var(--accent-soft);
}
.warn {
  border-color: #efd48d;
  background: var(--warn-soft);
}
.danger {
  border-color: #efb0b0;
  background: var(--danger-soft);
  color: var(--danger);
}
.grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(210px, 1fr)); gap: 10px; }
.metric { border: 1px solid var(--line); border-radius: 8px; padding: 10px; background: #fbfcfd; }
.metric strong { display: block; font-size: 18px; line-height: 1.2; overflow-wrap: anywhere; }
.metric span { color: var(--muted); }
.list { display: grid; gap: 8px; }
.item { border-top: 1px solid var(--line); padding-top: 8px; }
.item:first-child { border-top: 0; padding-top: 0; }
.item-title { font-weight: 600; }
.meta { color: var(--muted); font-size: 12px; margin-top: 2px; }
.empty { color: var(--muted); }
</style>
`)
	b.WriteString("</head>\n<body>\n<main>\n")
	b.WriteString("<header>\n")
	b.WriteString("<p class=\"eyebrow\">Gira ticket report</p>\n")
	fmt.Fprintf(&b, "<h1>%s</h1>\n", ticketStatusHTMLText(ticketStatusIssueLabel(status.Issue, status.Title)))
	b.WriteString("<div class=\"summary\">\n")
	ticketStatusHTMLWritePill(&b, "repo", status.Repo, "")
	ticketStatusHTMLWritePill(&b, "status", status.Status, "")
	ticketStatusHTMLWritePill(&b, "state", status.State, "")
	ticketStatusHTMLWritePill(&b, "next", status.NextAction, "next")
	ticketStatusHTMLWritePill(&b, "review", status.ReviewStatus, ticketStatusHTMLReviewClass(status.ReviewStatus))
	ticketStatusHTMLWritePill(&b, "checks", status.ChecksStatus, ticketStatusHTMLChecksClass(status.ChecksStatus))
	b.WriteString("</div>\n")
	if href := ticketStatusHTMLIssueHref(status); href != "" {
		fmt.Fprintf(&b, "<p class=\"meta\"><a href=\"%s\">Open ticket</a></p>\n", href)
	}
	b.WriteString("</header>\n")

	b.WriteString("<section>\n<h2>Next</h2>\n")
	if strings.TrimSpace(status.NextStep) != "" {
		fmt.Fprintf(&b, "<p class=\"meta\">Next step</p><code>%s</code>\n", ticketStatusHTMLText(status.NextStep))
	} else {
		b.WriteString("<p class=\"empty\">No next step recorded.</p>\n")
	}
	ticketStatusHTMLWriteInlineList(&b, "Blockers", status.Blockers, "danger")
	ticketStatusHTMLWriteInlineList(&b, "Warnings", status.Warnings, "warn")
	b.WriteString("</section>\n")

	b.WriteString("<section>\n<h2>Ticket</h2>\n<div class=\"grid\">\n")
	ticketStatusHTMLWriteMetric(&b, "status", status.Status)
	ticketStatusHTMLWriteMetric(&b, "state", status.State)
	ticketStatusHTMLWriteMetric(&b, "milestone", status.Milestone)
	ticketStatusHTMLWriteMetric(&b, "labels", strings.Join(status.Labels, ", "))
	b.WriteString("</div>\n")
	ticketStatusHTMLWriteBranch(&b, status)
	b.WriteString("</section>\n")

	ticketStatusHTMLWritePullRequest(&b, status)
	ticketStatusHTMLWriteChecks(&b, status.Checks)
	ticketStatusHTMLWriteReadiness(&b, status)
	ticketStatusHTMLWriteEvidence(&b, status)

	b.WriteString("</main>\n</body>\n</html>\n")
	return b.String()
}

func ticketStatusHTMLWriteBranch(b *strings.Builder, status WorkStatusResult) {
	if status.Branch == nil && status.BranchPolicy == nil {
		return
	}
	b.WriteString("<h3>Branch</h3>\n<div class=\"grid\">\n")
	if status.Branch != nil {
		ticketStatusHTMLWriteMetric(b, "expected", status.Branch.Expected)
		ticketStatusHTMLWriteMetric(b, "current", status.Branch.Current)
		ticketStatusHTMLWriteMetric(b, "trusted", ticketStatusHTMLBool(status.Branch.Trusted))
		ticketStatusHTMLWriteMetric(b, "source", status.Branch.Source)
	}
	if status.BranchPolicy != nil {
		ticketStatusHTMLWriteMetric(b, "recorded base", status.BranchPolicy.RecordedBase)
		ticketStatusHTMLWriteMetric(b, "actual PR base", status.BranchPolicy.ActualPRBase)
		ticketStatusHTMLWriteMetric(b, "base mismatch", ticketStatusHTMLBool(status.BranchPolicy.BaseMismatch))
		ticketStatusHTMLWriteMetric(b, "policy mode", status.BranchPolicy.PolicyMode)
	}
	b.WriteString("</div>\n")
	if status.BranchPolicy != nil {
		ticketStatusHTMLWriteInlineList(b, "Branch policy diagnostics", status.BranchPolicy.Diagnostics, "warn")
	}
}

func ticketStatusHTMLWritePullRequest(b *strings.Builder, status WorkStatusResult) {
	b.WriteString("<section>\n<h2>Pull Request</h2>\n")
	pr := status.PullRequest
	if pr == nil || !pr.Available {
		b.WriteString("<p class=\"empty\">No linked pull request.</p>\n</section>\n")
		return
	}
	b.WriteString("<div class=\"grid\">\n")
	ticketStatusHTMLWriteMetric(b, "number", fmt.Sprintf("#%d", pr.Number))
	ticketStatusHTMLWriteMetric(b, "state", pr.State)
	ticketStatusHTMLWriteMetric(b, "draft", ticketStatusHTMLBool(pr.IsDraft))
	ticketStatusHTMLWriteMetric(b, "mergeable", pr.Mergeable)
	ticketStatusHTMLWriteMetric(b, "review decision", pr.ReviewDecision)
	ticketStatusHTMLWriteMetric(b, "head", pr.HeadRefName)
	ticketStatusHTMLWriteMetric(b, "base", pr.BaseRefName)
	b.WriteString("</div>\n")
	if href := ticketStatusHTMLSafeHref(pr.URL); href != "" {
		fmt.Fprintf(b, "<p class=\"meta\"><a href=\"%s\">Open pull request</a></p>\n", href)
	}
	b.WriteString("</section>\n")
}

func ticketStatusHTMLWriteChecks(b *strings.Builder, checks []DevPRCheck) {
	b.WriteString("<section>\n<h2>Checks</h2>\n")
	if len(checks) == 0 {
		b.WriteString("<p class=\"empty\">No checks found.</p>\n</section>\n")
		return
	}
	b.WriteString("<table>\n<thead><tr><th>Name</th><th>State</th><th>Status</th><th>Conclusion</th></tr></thead>\n<tbody>\n")
	for _, check := range checks {
		name := ticketStatusHTMLFirstNonEmpty(check.Name, check.Workflow, "check")
		if href := ticketStatusHTMLSafeHref(check.URL); href != "" {
			name = fmt.Sprintf("<a href=\"%s\">%s</a>", href, ticketStatusHTMLText(name))
		} else {
			name = ticketStatusHTMLText(name)
		}
		fmt.Fprintf(
			b,
			"<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
			name,
			ticketStatusHTMLText(ticketStatusHTMLValue(check.State)),
			ticketStatusHTMLText(ticketStatusHTMLValue(check.Status)),
			ticketStatusHTMLText(ticketStatusHTMLValue(check.Conclusion)),
		)
	}
	b.WriteString("</tbody>\n</table>\n</section>\n")
}

func ticketStatusHTMLWriteReadiness(b *strings.Builder, status WorkStatusResult) {
	b.WriteString("<section>\n<h2>Readiness</h2>\n<div class=\"grid\">\n")
	if status.TicketReadiness != nil {
		ticketStatusHTMLWriteMetric(b, "ticket readiness", status.TicketReadiness.Readiness)
		ticketStatusHTMLWriteMetric(b, "ticket next", status.TicketReadiness.NextAction)
		ticketStatusHTMLWriteMetric(b, "ticket schema", status.TicketReadiness.SchemaVersion)
	} else {
		ticketStatusHTMLWriteMetric(b, "ticket readiness", "unknown")
	}
	if status.PRReadiness != nil {
		ticketStatusHTMLWriteMetric(b, "PR readiness", status.PRReadiness.Readiness)
		ticketStatusHTMLWriteMetric(b, "PR next", status.PRReadiness.NextAction)
		ticketStatusHTMLWriteMetric(b, "PR schema", status.PRReadiness.SchemaVersion)
	} else {
		ticketStatusHTMLWriteMetric(b, "PR readiness", "unknown")
	}
	b.WriteString("</div>\n")
	if status.TicketReadiness != nil {
		ticketStatusHTMLWriteTicketReadinessFindings(b, "Ticket findings", status.TicketReadiness.Findings)
	}
	if status.PRReadiness != nil {
		ticketStatusHTMLWritePRReadinessFindings(b, "PR findings", status.PRReadiness.Findings)
	}
	b.WriteString("</section>\n")
}

func ticketStatusHTMLWriteTicketReadinessFindings(b *strings.Builder, title string, findings []TicketReadinessFinding) {
	fmt.Fprintf(b, "<h3>%s</h3>\n", ticketStatusHTMLText(title))
	if len(findings) == 0 {
		b.WriteString("<p class=\"empty\">None.</p>\n")
		return
	}
	b.WriteString("<div class=\"list\">\n")
	for _, finding := range findings {
		b.WriteString("<div class=\"item\">\n")
		fmt.Fprintf(b, "<p class=\"item-title\">%s: %s</p>\n", ticketStatusHTMLText(finding.Severity), ticketStatusHTMLText(finding.Kind))
		fmt.Fprintf(b, "<p>%s</p>\n", ticketStatusHTMLText(finding.Message))
		if strings.TrimSpace(finding.RecommendedAction) != "" {
			fmt.Fprintf(b, "<p class=\"meta\">%s</p>\n", ticketStatusHTMLText(finding.RecommendedAction))
		}
		b.WriteString("</div>\n")
	}
	b.WriteString("</div>\n")
}

func ticketStatusHTMLWritePRReadinessFindings(b *strings.Builder, title string, findings []PRReadinessFinding) {
	fmt.Fprintf(b, "<h3>%s</h3>\n", ticketStatusHTMLText(title))
	if len(findings) == 0 {
		b.WriteString("<p class=\"empty\">None.</p>\n")
		return
	}
	b.WriteString("<div class=\"list\">\n")
	for _, finding := range findings {
		b.WriteString("<div class=\"item\">\n")
		fmt.Fprintf(b, "<p class=\"item-title\">%s: %s</p>\n", ticketStatusHTMLText(finding.Severity), ticketStatusHTMLText(finding.Kind))
		fmt.Fprintf(b, "<p>%s</p>\n", ticketStatusHTMLText(finding.Message))
		if strings.TrimSpace(finding.RecommendedAction) != "" {
			fmt.Fprintf(b, "<p class=\"meta\">%s</p>\n", ticketStatusHTMLText(finding.RecommendedAction))
		}
		b.WriteString("</div>\n")
	}
	b.WriteString("</div>\n")
}

func ticketStatusHTMLWriteEvidence(b *strings.Builder, status WorkStatusResult) {
	b.WriteString("<section>\n<h2>Evidence</h2>\n<div class=\"grid\">\n")
	if status.Evidence != nil {
		ticketStatusHTMLWriteMetric(b, "closing reference", ticketStatusHTMLBool(status.Evidence.ClosingReference))
		ticketStatusHTMLWriteMetric(b, "branch trusted", ticketStatusHTMLBool(status.Evidence.BranchTrusted))
		ticketStatusHTMLWriteMetric(b, "finish ready", ticketStatusHTMLBool(status.Evidence.FinishReady))
		ticketStatusHTMLWriteMetric(b, "sources", strings.Join(status.Evidence.Sources, ", "))
	} else {
		ticketStatusHTMLWriteMetric(b, "evidence", "unknown")
	}
	if status.Acceptance != nil {
		ticketStatusHTMLWriteMetric(b, "acceptance", fmt.Sprintf("%s %d/%d", status.Acceptance.Status, status.Acceptance.Complete, status.Acceptance.Total))
	}
	if status.Telemetry != nil {
		ticketStatusHTMLWriteMetric(b, "telemetry", status.Telemetry.Status)
		ticketStatusHTMLWriteMetric(b, "telemetry present", ticketStatusHTMLBool(status.Telemetry.Present))
	}
	ticketStatusHTMLWriteMetric(b, "source contract", ticketStatusHTMLSchemaVersion(status))
	b.WriteString("</div>\n")
	if status.Telemetry != nil {
		ticketStatusHTMLWriteInlineList(b, "Telemetry warnings", status.Telemetry.Warnings, "warn")
	}
	b.WriteString("</section>\n")
}

func ticketStatusHTMLWriteInlineList(b *strings.Builder, title string, values []string, className string) {
	if len(values) == 0 {
		return
	}
	fmt.Fprintf(b, "<h3>%s</h3>\n<div class=\"list\">\n", ticketStatusHTMLText(title))
	for _, value := range values {
		classes := "item"
		if strings.TrimSpace(className) != "" {
			classes += " " + className
		}
		fmt.Fprintf(b, "<div class=\"%s\">%s</div>\n", ticketStatusHTMLText(classes), ticketStatusHTMLText(value))
	}
	b.WriteString("</div>\n")
}

func ticketStatusHTMLWriteMetric(b *strings.Builder, label string, value string) {
	fmt.Fprintf(b, "<div class=\"metric\"><strong>%s</strong><span>%s</span></div>\n", ticketStatusHTMLText(ticketStatusHTMLValue(value)), ticketStatusHTMLText(label))
}

func ticketStatusHTMLWritePill(b *strings.Builder, label string, value string, className string) {
	classes := "pill"
	if strings.TrimSpace(className) != "" {
		classes += " " + className
	}
	fmt.Fprintf(b, "<span class=\"%s\">%s: %s</span>\n", ticketStatusHTMLText(classes), ticketStatusHTMLText(label), ticketStatusHTMLText(ticketStatusHTMLValue(value)))
}

func ticketStatusHTMLReviewClass(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "blocked", "changes_requested":
		return "danger"
	case "missing", "unknown":
		return "warn"
	default:
		return ""
	}
}

func ticketStatusHTMLChecksClass(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "failed", "failing", "error":
		return "danger"
	case "pending", "missing", "unknown", "":
		return "warn"
	default:
		return ""
	}
}

func ticketStatusIssueLabel(number int, title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Sprintf("#%d", number)
	}
	return fmt.Sprintf("#%d %s", number, title)
}

func ticketStatusHTMLIssueHref(status WorkStatusResult) string {
	if status.Issue <= 0 || strings.Count(status.Repo, "/") != 1 {
		return ""
	}
	return ticketStatusHTMLSafeHref(fmt.Sprintf("https://github.com/%s/issues/%d", status.Repo, status.Issue))
}

func ticketStatusHTMLSchemaVersion(status WorkStatusResult) string {
	if strings.TrimSpace(status.SchemaVersion) != "" {
		return status.SchemaVersion
	}
	return TicketStatusSchemaVersion
}

func ticketStatusHTMLSafeHref(value string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		return ticketStatusHTMLText(value)
	}
	return ""
}

func ticketStatusHTMLText(value string) string {
	return html.EscapeString(value)
}

func ticketStatusHTMLBool(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func ticketStatusHTMLValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "none"
	}
	return value
}

func ticketStatusHTMLFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
