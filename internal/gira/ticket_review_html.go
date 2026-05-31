package gira

import (
	"fmt"
	"html"
	"strings"
)

func WriteTicketReviewHTML(path string, report AgentPromptReport) error {
	return writeSafeLocalFile(path, []byte(RenderTicketReviewHTML(report)), 0o644)
}

func RenderTicketReviewHTML(report AgentPromptReport) string {
	var b strings.Builder
	title := fmt.Sprintf("Review packet #%d", report.Ticket)
	if strings.TrimSpace(report.Issue.Title) != "" {
		title = fmt.Sprintf("%s - %s", title, report.Issue.Title)
	}

	b.WriteString("<!doctype html>\n")
	b.WriteString("<html lang=\"en\">\n")
	b.WriteString("<head>\n")
	b.WriteString("<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	fmt.Fprintf(&b, "<title>%s</title>\n", ticketReviewHTMLText(title))
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
main { max-width: 1120px; margin: 0 auto; padding: 28px 18px 42px; }
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
h3 { font-size: 14px; margin: 12px 0 8px; }
a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }
code, pre {
  max-width: 100%;
  overflow-wrap: anywhere;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: #f2f4f7;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
}
code { display: inline-block; padding: 2px 6px; }
pre { overflow: auto; white-space: pre-wrap; padding: 10px; margin: 8px 0 0; }
table { width: 100%; border-collapse: collapse; }
th, td { border-top: 1px solid var(--line); padding: 7px 6px; text-align: left; vertical-align: top; }
th { color: var(--muted); font-weight: 600; }
ul { margin: 6px 0 0; padding-left: 20px; }
.eyebrow { color: var(--muted); font-size: 12px; text-transform: uppercase; margin-bottom: 4px; }
.summary { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 14px; }
.pill {
  border: 1px solid var(--line);
  border-radius: 999px;
  padding: 3px 9px;
  background: #fbfcfd;
  color: var(--muted);
}
.next { border-color: #b9d8ef; background: var(--accent-soft); }
.warn { border-color: #efd48d; background: var(--warn-soft); }
.danger { border-color: #efb0b0; background: var(--danger-soft); color: var(--danger); }
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
	b.WriteString("<p class=\"eyebrow\">Gira review packet</p>\n")
	fmt.Fprintf(&b, "<h1>%s</h1>\n", ticketReviewHTMLText(ticketStatusIssueLabel(report.Ticket, report.Issue.Title)))
	b.WriteString("<div class=\"summary\">\n")
	ticketReviewHTMLWritePill(&b, "repo", report.Repo, "")
	ticketReviewHTMLWritePill(&b, "role", report.Role, "")
	ticketReviewHTMLWritePill(&b, "profile", report.Profile, "")
	if report.PR != nil {
		ticketReviewHTMLWritePill(&b, "PR", fmt.Sprintf("#%d", report.PR.Number), "")
		ticketReviewHTMLWritePill(&b, "review", report.PR.ReviewDecision, ticketReviewHTMLReviewClass(report.PR.ReviewDecision))
	}
	if report.PRReady != nil {
		ticketReviewHTMLWritePill(&b, "readiness", report.PRReady.Readiness, ticketReviewHTMLReadinessClass(report.PRReady.Readiness))
	}
	b.WriteString("</div>\n")
	if href := ticketReviewHTMLIssueHref(report); href != "" {
		fmt.Fprintf(&b, "<p class=\"meta\"><a href=\"%s\">Open ticket</a></p>\n", href)
	}
	b.WriteString("</header>\n")

	b.WriteString("<section>\n<h2>Next</h2>\n")
	if strings.TrimSpace(report.NextStep) != "" {
		fmt.Fprintf(&b, "<p class=\"meta\">Next step</p><code>%s</code>\n", ticketReviewHTMLText(report.NextStep))
	} else {
		b.WriteString("<p class=\"empty\">No next step recorded.</p>\n")
	}
	b.WriteString("</section>\n")

	ticketReviewHTMLWriteTicket(&b, report)
	ticketReviewHTMLWritePullRequest(&b, report)
	ticketReviewHTMLWriteReadiness(&b, report)
	ticketReviewHTMLWriteDiffSummary(&b, report)
	ticketReviewHTMLWriteReviewContract(&b, report)
	ticketReviewHTMLWritePrompt(&b, report)

	b.WriteString("</main>\n</body>\n</html>\n")
	return b.String()
}

func ticketReviewHTMLWriteTicket(b *strings.Builder, report AgentPromptReport) {
	b.WriteString("<section>\n<h2>Ticket Context</h2>\n<div class=\"grid\">\n")
	ticketReviewHTMLWriteMetric(b, "state", report.Issue.State)
	ticketReviewHTMLWriteMetric(b, "labels", strings.Join(report.Issue.Labels, ", "))
	ticketReviewHTMLWriteMetric(b, "source", "ticket review JSON")
	b.WriteString("</div>\n")
	if strings.TrimSpace(report.Issue.Goal) != "" {
		fmt.Fprintf(b, "<h3>Goal</h3>\n<p>%s</p>\n", ticketReviewHTMLText(report.Issue.Goal))
	}
	if strings.TrimSpace(report.Issue.Scope) != "" {
		fmt.Fprintf(b, "<h3>Scope</h3>\n<p>%s</p>\n", ticketReviewHTMLText(report.Issue.Scope))
	}
	ticketReviewHTMLWriteStringList(b, "Acceptance Criteria", report.Issue.Acceptance, "")
	b.WriteString("</section>\n")
}

func ticketReviewHTMLWritePullRequest(b *strings.Builder, report AgentPromptReport) {
	b.WriteString("<section>\n<h2>Pull Request</h2>\n")
	if report.PR == nil {
		b.WriteString("<p class=\"empty\">No linked pull request.</p>\n")
		if report.Evidence != nil {
			ticketReviewHTMLWriteStringList(b, "Blockers", report.Evidence.Blockers, "danger")
		}
		b.WriteString("</section>\n")
		return
	}
	pr := report.PR
	b.WriteString("<div class=\"grid\">\n")
	ticketReviewHTMLWriteMetric(b, "number", fmt.Sprintf("#%d", pr.Number))
	ticketReviewHTMLWriteMetric(b, "state", pr.State)
	ticketReviewHTMLWriteMetric(b, "draft", ticketReviewHTMLBool(pr.IsDraft))
	ticketReviewHTMLWriteMetric(b, "finish ready", ticketReviewHTMLBool(pr.FinishReady))
	ticketReviewHTMLWriteMetric(b, "review decision", pr.ReviewDecision)
	ticketReviewHTMLWriteMetric(b, "merge state", pr.MergeState)
	ticketReviewHTMLWriteMetric(b, "head", pr.HeadRefName)
	ticketReviewHTMLWriteMetric(b, "base", pr.BaseRefName)
	ticketReviewHTMLWriteMetric(b, "recorded base", pr.RecordedBase)
	ticketReviewHTMLWriteMetric(b, "base mismatch", ticketReviewHTMLBool(pr.BaseMismatch))
	b.WriteString("</div>\n")
	if href := ticketReviewHTMLSafeHref(pr.URL); href != "" {
		fmt.Fprintf(b, "<p class=\"meta\"><a href=\"%s\">Open pull request</a></p>\n", href)
	}
	ticketReviewHTMLWriteStringList(b, "Blockers", pr.Blockers, "danger")
	ticketReviewHTMLWriteChangedFiles(b, pr.ChangedFiles)
	ticketReviewHTMLWriteChecks(b, pr.Checks)
	b.WriteString("</section>\n")
}

func ticketReviewHTMLWriteReadiness(b *strings.Builder, report AgentPromptReport) {
	b.WriteString("<section>\n<h2>PR Readiness</h2>\n")
	if report.PRReady == nil {
		b.WriteString("<p class=\"empty\">No PR readiness report.</p>\n</section>\n")
		return
	}
	b.WriteString("<div class=\"grid\">\n")
	ticketReviewHTMLWriteMetric(b, "schema", report.PRReady.SchemaVersion)
	ticketReviewHTMLWriteMetric(b, "readiness", report.PRReady.Readiness)
	ticketReviewHTMLWriteMetric(b, "next action", report.PRReady.NextAction)
	if report.PRReady.PullRequest > 0 {
		ticketReviewHTMLWriteMetric(b, "pull request", fmt.Sprintf("#%d", report.PRReady.PullRequest))
	}
	b.WriteString("</div>\n")
	if len(report.PRReady.Findings) == 0 {
		b.WriteString("<p class=\"empty\">No readiness findings.</p>\n</section>\n")
		return
	}
	b.WriteString("<div class=\"list\">\n")
	for _, finding := range report.PRReady.Findings {
		b.WriteString("<div class=\"item\">\n")
		fmt.Fprintf(b, "<p class=\"item-title\">%s: %s</p>\n", ticketReviewHTMLText(finding.Severity), ticketReviewHTMLText(finding.Kind))
		fmt.Fprintf(b, "<p>%s</p>\n", ticketReviewHTMLText(finding.Message))
		if strings.TrimSpace(finding.RecommendedAction) != "" {
			fmt.Fprintf(b, "<p class=\"meta\">%s</p>\n", ticketReviewHTMLText(finding.RecommendedAction))
		}
		b.WriteString("</div>\n")
	}
	b.WriteString("</div>\n</section>\n")
}

func ticketReviewHTMLWriteDiffSummary(b *strings.Builder, report AgentPromptReport) {
	b.WriteString("<section>\n<h2>Diff Summary</h2>\n")
	if report.Review == nil || report.Review.DiffSummary == nil {
		b.WriteString("<p class=\"empty\">No diff summary included.</p>\n</section>\n")
		return
	}
	summary := report.Review.DiffSummary
	if strings.TrimSpace(summary.UnsupportedMessage) != "" {
		fmt.Fprintf(b, "<p class=\"empty\">%s</p>\n</section>\n", ticketReviewHTMLText(summary.UnsupportedMessage))
		return
	}
	b.WriteString("<div class=\"grid\">\n")
	ticketReviewHTMLWriteMetric(b, "files changed", fmt.Sprintf("%d", len(summary.ChangedFiles)))
	ticketReviewHTMLWriteMetric(b, "additions", fmt.Sprintf("%d", summary.TotalAdditions))
	ticketReviewHTMLWriteMetric(b, "deletions", fmt.Sprintf("%d", summary.TotalDeletions))
	ticketReviewHTMLWriteMetric(b, "full diff", summary.FullDiffCommand)
	b.WriteString("</div>\n")
	if len(summary.Files) > 0 {
		b.WriteString("<table>\n<thead><tr><th>File</th><th>Additions</th><th>Deletions</th><th>Hunks</th></tr></thead>\n<tbody>\n")
		for _, file := range summary.Files {
			fmt.Fprintf(
				b,
				"<tr><td>%s</td><td>%d</td><td>%d</td><td>%s</td></tr>\n",
				ticketReviewHTMLText(file.Path),
				file.Additions,
				file.Deletions,
				ticketReviewHTMLText(strings.Join(file.Hunks, " | ")),
			)
		}
		b.WriteString("</tbody>\n</table>\n")
	}
	ticketReviewHTMLWriteAcceptanceMapping(b, summary.AcceptanceMapping)
	ticketReviewHTMLWriteStringList(b, "Risk Areas", summary.RiskAreas, "warn")
	if strings.TrimSpace(summary.FullDiff) != "" {
		fmt.Fprintf(b, "<h3>Included Full Diff</h3>\n<pre>%s</pre>\n", ticketReviewHTMLText(summary.FullDiff))
	}
	b.WriteString("</section>\n")
}

func ticketReviewHTMLWriteReviewContract(b *strings.Builder, report AgentPromptReport) {
	if report.Review == nil {
		return
	}
	b.WriteString("<section>\n<h2>Review Contract</h2>\n")
	if len(report.Review.DiffReferences) > 0 {
		b.WriteString("<h3>Diff References</h3>\n<div class=\"list\">\n")
		for _, ref := range report.Review.DiffReferences {
			fmt.Fprintf(b, "<div class=\"item\"><span class=\"item-title\">%s</span><br><code>%s</code></div>\n", ticketReviewHTMLText(ref.Kind), ticketReviewHTMLText(ref.Command))
		}
		b.WriteString("</div>\n")
	}
	if len(report.Review.Guidance) > 0 {
		b.WriteString("<h3>Repo Guidance</h3>\n<div class=\"list\">\n")
		for _, guidance := range report.Review.Guidance {
			fmt.Fprintf(b, "<div class=\"item\"><span class=\"item-title\">%s</span><p class=\"meta\">%s</p></div>\n", ticketReviewHTMLText(guidance.Path), ticketReviewHTMLText(guidance.Status))
		}
		b.WriteString("</div>\n")
	}
	b.WriteString("<h3>Verdict Schema</h3>\n<div class=\"grid\">\n")
	ticketReviewHTMLWriteMetric(b, "goal fulfilled", strings.Join(report.Review.VerdictSchema.GoalFulfilled, " / "))
	ticketReviewHTMLWriteMetric(b, "acceptance", strings.Join(report.Review.VerdictSchema.AcceptanceCriteriaStatus, " / "))
	ticketReviewHTMLWriteMetric(b, "checks", strings.Join(report.Review.VerdictSchema.ChecksStatus, " / "))
	ticketReviewHTMLWriteMetric(b, "evidence", strings.Join(report.Review.VerdictSchema.EvidenceStatus, " / "))
	ticketReviewHTMLWriteMetric(b, "risk", strings.Join(report.Review.VerdictSchema.ResidualRisk, " / "))
	ticketReviewHTMLWriteMetric(b, "action", strings.Join(report.Review.VerdictSchema.RecommendedAction, " / "))
	b.WriteString("</div>\n</section>\n")
}

func ticketReviewHTMLWritePrompt(b *strings.Builder, report AgentPromptReport) {
	if strings.TrimSpace(report.Prompt) == "" {
		return
	}
	b.WriteString("<section>\n<h2>Reviewer Prompt</h2>\n")
	fmt.Fprintf(b, "<pre>%s</pre>\n", ticketReviewHTMLText(report.Prompt))
	b.WriteString("</section>\n")
}

func ticketReviewHTMLWriteChangedFiles(b *strings.Builder, files []string) {
	ticketReviewHTMLWriteStringList(b, "Changed Files", files, "")
}

func ticketReviewHTMLWriteChecks(b *strings.Builder, checks []DevPRCheck) {
	if len(checks) == 0 {
		return
	}
	b.WriteString("<h3>Checks</h3>\n<table>\n<thead><tr><th>Name</th><th>State</th><th>Status</th><th>Conclusion</th></tr></thead>\n<tbody>\n")
	for _, check := range checks {
		name := ticketStatusHTMLFirstNonEmpty(check.Name, check.Workflow, "check")
		if strings.TrimSpace(check.Workflow) != "" && strings.TrimSpace(check.Name) != "" {
			name = check.Workflow + "/" + check.Name
		}
		if href := ticketReviewHTMLSafeHref(check.URL); href != "" {
			name = fmt.Sprintf("<a href=\"%s\">%s</a>", href, ticketReviewHTMLText(name))
		} else {
			name = ticketReviewHTMLText(name)
		}
		fmt.Fprintf(
			b,
			"<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
			name,
			ticketReviewHTMLText(ticketReviewHTMLValue(check.State)),
			ticketReviewHTMLText(ticketReviewHTMLValue(check.Status)),
			ticketReviewHTMLText(ticketReviewHTMLValue(check.Conclusion)),
		)
	}
	b.WriteString("</tbody>\n</table>\n")
}

func ticketReviewHTMLWriteAcceptanceMapping(b *strings.Builder, mappings []AgentReviewAcceptanceHint) {
	if len(mappings) == 0 {
		return
	}
	b.WriteString("<h3>Acceptance Mapping</h3>\n<div class=\"list\">\n")
	for _, mapping := range mappings {
		b.WriteString("<div class=\"item\">\n")
		fmt.Fprintf(b, "<p class=\"item-title\">%s</p>\n", ticketReviewHTMLText(mapping.Criterion))
		fmt.Fprintf(b, "<p class=\"meta\">files=%s reason=%s</p>\n", ticketReviewHTMLText(ticketReviewHTMLValue(strings.Join(mapping.Files, ", "))), ticketReviewHTMLText(mapping.Reason))
		b.WriteString("</div>\n")
	}
	b.WriteString("</div>\n")
}

func ticketReviewHTMLWriteStringList(b *strings.Builder, title string, values []string, className string) {
	if len(values) == 0 {
		return
	}
	fmt.Fprintf(b, "<h3>%s</h3>\n<ul>\n", ticketReviewHTMLText(title))
	for _, value := range values {
		classAttr := ""
		if strings.TrimSpace(className) != "" {
			classAttr = fmt.Sprintf(" class=\"%s\"", ticketReviewHTMLText(className))
		}
		fmt.Fprintf(b, "<li%s>%s</li>\n", classAttr, ticketReviewHTMLText(value))
	}
	b.WriteString("</ul>\n")
}

func ticketReviewHTMLWriteMetric(b *strings.Builder, label string, value string) {
	fmt.Fprintf(b, "<div class=\"metric\"><strong>%s</strong><span>%s</span></div>\n", ticketReviewHTMLText(ticketReviewHTMLValue(value)), ticketReviewHTMLText(label))
}

func ticketReviewHTMLWritePill(b *strings.Builder, label string, value string, className string) {
	classes := "pill"
	if strings.TrimSpace(className) != "" {
		classes += " " + className
	}
	fmt.Fprintf(b, "<span class=\"%s\">%s: %s</span>\n", ticketReviewHTMLText(classes), ticketReviewHTMLText(label), ticketReviewHTMLText(ticketReviewHTMLValue(value)))
}

func ticketReviewHTMLIssueHref(report AgentPromptReport) string {
	if report.Ticket <= 0 || strings.Count(report.Repo, "/") != 1 {
		return ""
	}
	return ticketReviewHTMLSafeHref(fmt.Sprintf("https://github.com/%s/issues/%d", report.Repo, report.Ticket))
}

func ticketReviewHTMLReviewClass(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "CHANGES_REQUESTED":
		return "danger"
	case "", "REVIEW_REQUIRED":
		return "warn"
	default:
		return ""
	}
}

func ticketReviewHTMLReadinessClass(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "needs_revision", "blocked":
		return "danger"
	case "ready_for_review":
		return "warn"
	default:
		return ""
	}
}

func ticketReviewHTMLSafeHref(value string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		return ticketReviewHTMLText(value)
	}
	return ""
}

func ticketReviewHTMLText(value string) string {
	return html.EscapeString(value)
}

func ticketReviewHTMLBool(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func ticketReviewHTMLValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "none"
	}
	return value
}
