package gira

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"time"
)

const GoalDossierSchemaVersion = "goal-dossier/v1"

type GoalDossierInput struct {
	Repo RepoRef `json:"repo"`
	Goal int     `json:"goal"`
	View string  `json:"view,omitempty"`
}

type GoalDossierReport struct {
	Command                 string                     `json:"command"`
	SchemaVersion           string                     `json:"schema_version"`
	Repo                    string                     `json:"repo"`
	GeneratedAt             string                     `json:"generated_at"`
	Goal                    GoalStatusIssue            `json:"goal"`
	ChildGroups             []GoalDossierChildGroup    `json:"child_groups"`
	Counts                  map[string]int             `json:"counts"`
	Blockers                []string                   `json:"blockers,omitempty"`
	StopConditions          []string                   `json:"stop_conditions,omitempty"`
	NextAction              string                     `json:"next_action"`
	NextStep                string                     `json:"next_step"`
	SelectedTicket          *GoalNextCandidate         `json:"selected_ticket,omitempty"`
	RemainingAutonomousWork int                        `json:"remaining_autonomous_work"`
	HandoffReceiptPresent   bool                       `json:"handoff_receipt_present"`
	Evidence                GoalDossierEvidenceSummary `json:"evidence"`
	Sources                 []GoalDossierSource        `json:"sources"`
	Measurement             *PMMeasurementReport       `json:"measurement,omitempty"`
	PMView                  *GoalPMView                `json:"pm_view,omitempty"`
	Diagnostics             []string                   `json:"diagnostics,omitempty"`
}

type GoalDossierChildGroup struct {
	Category string            `json:"category"`
	Count    int               `json:"count"`
	Children []GoalStatusChild `json:"children"`
}

type GoalDossierEvidenceSummary struct {
	Sources                 []string                 `json:"sources"`
	ChildCount              int                      `json:"child_count"`
	RemainingAutonomousWork int                      `json:"remaining_autonomous_work"`
	HandoffReceiptPresent   bool                     `json:"handoff_receipt_present"`
	BlockerCount            int                      `json:"blocker_count"`
	Checks                  GoalDossierChecksSummary `json:"checks"`
	Reviews                 map[string]int           `json:"reviews,omitempty"`
}

type GoalDossierChecksSummary struct {
	Total   int `json:"total"`
	Passing int `json:"passing"`
	Pending int `json:"pending"`
	Failing int `json:"failing"`
	Missing int `json:"missing"`
	Unknown int `json:"unknown"`
}

type GoalDossierSource struct {
	Name          string `json:"name"`
	SchemaVersion string `json:"schema_version"`
}

func BuildGoalDossierReport(input GoalDossierInput, runner CommandRunner) (GoalDossierReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	runner = &pmObserveCachedRunner{base: runner, results: map[string]*pmObserveCachedResult{}}
	status, err := BuildGoalStatusReport(GoalStatusInput{Repo: input.Repo, Goal: input.Goal}, runner)
	if err != nil {
		return GoalDossierReport{}, err
	}
	next := BuildGoalNextReportFromStatus(input.Repo, status)
	report := BuildGoalDossierReportFromStatus(status, next)
	observe, observeErr := BuildPMObserveReport(PMObserveInput{Repo: input.Repo, Ticket: input.Goal}, runner)
	if observeErr != nil {
		report.Diagnostics = append(report.Diagnostics, "pm_state_unavailable: "+observeErr.Error())
		return report, nil
	}
	if observe.Measurement != nil && (observe.Measurement.Summary.Outcomes > 0 || observe.Measurement.Summary.Measurements > 0) {
		report.Measurement = observe.Measurement
	}
	goal, fetchErr := fetchDevIssue(input.Repo, input.Goal, runner)
	if fetchErr != nil {
		report.Diagnostics = append(report.Diagnostics, "pm_ir_source_unavailable: "+fetchErr.Error())
		return report, nil
	}
	compile, compileErr := BuildPMCompileReport(PMCompileInput{RawIntent: goal.Body, Repo: input.Repo.FullName(), Goal: &PMCompileGoal{Number: goal.Number, Title: goal.Title, Body: goal.Body, URL: githubIssueURL(input.Repo, goal.Number)}})
	if compileErr != nil {
		report.Diagnostics = append(report.Diagnostics, "pm_ir_unavailable: "+compileErr.Error())
		return report, nil
	}
	view, viewErr := BuildGoalPMView(input.View, compile, observe)
	if viewErr != nil {
		return report, viewErr
	}
	report.PMView = &view
	report.Sources = append(report.Sources, GoalDossierSource{Name: "goal_pm_view", SchemaVersion: GoalPMViewSchemaVersion})
	if report.Measurement != nil {
		report.Sources = append(report.Sources, GoalDossierSource{Name: "pm_measurement", SchemaVersion: PMMeasurementReportSchemaVersion})
	}
	if view.Kind != "operator" {
		report.ChildGroups = nil
		report.SelectedTicket = nil
	}
	if view.Kind == "stakeholder" {
		report.Blockers = nil
		report.StopConditions = nil
	}
	return report, nil
}

func BuildGoalDossierReportFromStatus(status GoalStatusReport, next GoalNextReport) GoalDossierReport {
	report := GoalDossierReport{
		Command:                 "goal dossier",
		SchemaVersion:           GoalDossierSchemaVersion,
		Repo:                    status.Repo,
		GeneratedAt:             time.Now().UTC().Format(time.RFC3339),
		Goal:                    status.Goal,
		ChildGroups:             goalDossierChildGroups(status.Children),
		Counts:                  copyStringIntMap(status.Counts),
		Blockers:                append([]string(nil), status.Blockers...),
		StopConditions:          append([]string(nil), next.StopReasons...),
		NextAction:              next.NextAction,
		NextStep:                next.NextStep,
		RemainingAutonomousWork: status.RemainingAutonomousWork,
		HandoffReceiptPresent:   status.HandoffReceiptPresent,
		Evidence:                goalDossierEvidence(status),
		Sources: []GoalDossierSource{
			{Name: "goal_status", SchemaVersion: GoalStatusSchemaVersion},
			{Name: "goal_next", SchemaVersion: GoalNextSchemaVersion},
		},
	}
	if next.SelectedTicket != nil {
		selected := *next.SelectedTicket
		report.SelectedTicket = &selected
	}
	return report
}

func goalDossierChildGroups(children []GoalStatusChild) []GoalDossierChildGroup {
	byCategory := map[string][]GoalStatusChild{}
	for _, child := range children {
		byCategory[child.Category] = append(byCategory[child.Category], child)
	}
	for category := range byCategory {
		sort.Slice(byCategory[category], func(i, j int) bool {
			return byCategory[category][i].Number < byCategory[category][j].Number
		})
	}
	order := []string{"ready", "in_progress", "in_review", "blocked", "done", "closed_other", "unknown"}
	out := []GoalDossierChildGroup{}
	seen := map[string]struct{}{}
	for _, category := range order {
		children := byCategory[category]
		if len(children) == 0 {
			continue
		}
		out = append(out, GoalDossierChildGroup{Category: category, Count: len(children), Children: append([]GoalStatusChild(nil), children...)})
		seen[category] = struct{}{}
	}
	extra := []string{}
	for category := range byCategory {
		if _, ok := seen[category]; !ok {
			extra = append(extra, category)
		}
	}
	sort.Strings(extra)
	for _, category := range extra {
		children := byCategory[category]
		out = append(out, GoalDossierChildGroup{Category: category, Count: len(children), Children: append([]GoalStatusChild(nil), children...)})
	}
	return out
}

func goalDossierEvidence(status GoalStatusReport) GoalDossierEvidenceSummary {
	return GoalDossierEvidenceSummary{
		Sources:                 []string{"goal_status", "goal_next"},
		ChildCount:              len(status.Children),
		RemainingAutonomousWork: status.RemainingAutonomousWork,
		HandoffReceiptPresent:   status.HandoffReceiptPresent,
		BlockerCount:            len(status.Blockers),
		Checks:                  goalDossierChecks(status.Children),
		Reviews:                 goalDossierReviews(status.Children),
	}
}

func goalDossierChecks(children []GoalStatusChild) GoalDossierChecksSummary {
	var out GoalDossierChecksSummary
	for _, child := range children {
		out.Total++
		switch strings.ToLower(strings.TrimSpace(child.ChecksStatus)) {
		case "":
			out.Missing++
		case "passed", "passing", "success":
			out.Passing++
		case "pending", "queued", "in_progress":
			out.Pending++
		case "failed", "failure", "failing", "error", "cancelled", "timed_out":
			out.Failing++
		case "missing":
			out.Missing++
		default:
			out.Unknown++
		}
	}
	return out
}

func goalDossierReviews(children []GoalStatusChild) map[string]int {
	out := map[string]int{}
	for _, child := range children {
		key := strings.ToLower(strings.TrimSpace(child.ReviewStatus))
		if key == "" {
			key = "unknown"
		}
		out[key]++
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func FormatGoalDossier(report GoalDossierReport) string {
	return FormatGoalReport(report)
}

func FormatGoalReport(report GoalDossierReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "goal report: #%d children=%d remaining=%d next=%s\n", report.Goal.Number, report.Counts["total"], report.RemainingAutonomousWork, report.NextAction)
	if len(report.ChildGroups) > 0 {
		parts := []string{}
		for _, group := range report.ChildGroups {
			parts = append(parts, fmt.Sprintf("%s=%d", group.Category, group.Count))
		}
		fmt.Fprintf(&b, "children: %s\n", strings.Join(parts, " "))
	}
	if report.SelectedTicket != nil {
		fmt.Fprintf(&b, "selected: #%d %s\n", report.SelectedTicket.Number, report.SelectedTicket.Reason)
	}
	if len(report.Blockers) > 0 {
		fmt.Fprintf(&b, "blockers: %s\n", strings.Join(report.Blockers, ","))
	}
	if len(report.StopConditions) > 0 {
		fmt.Fprintf(&b, "stop: %s\n", strings.Join(report.StopConditions, ","))
	}
	if report.Measurement != nil {
		fmt.Fprintf(&b, "outcomes: validated=%d not_validated=%d limited=%d blocked=%d diagnostics=%d\n", report.Measurement.Summary.Validated, report.Measurement.Summary.NotValidated, report.Measurement.Summary.Limited, report.Measurement.Summary.Blocked, len(report.Measurement.Diagnostics))
	}
	if report.PMView != nil {
		b.WriteString(FormatGoalPMView(*report.PMView))
	}
	for _, diagnostic := range report.Diagnostics {
		fmt.Fprintf(&b, "diagnostic: %s\n", diagnostic)
	}
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}

func WriteGoalReportHTML(path string, report GoalDossierReport) error {
	return writeSafeLocalFile(path, []byte(RenderGoalReportHTML(report)), 0o644)
}

func RenderGoalReportHTML(report GoalDossierReport) string {
	var b strings.Builder
	title := fmt.Sprintf("Goal report #%d", report.Goal.Number)
	if strings.TrimSpace(report.Goal.Title) != "" {
		title = fmt.Sprintf("%s - %s", title, report.Goal.Title)
	}

	b.WriteString("<!doctype html>\n")
	b.WriteString("<html lang=\"en\">\n")
	b.WriteString("<head>\n")
	b.WriteString("<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	fmt.Fprintf(&b, "<title>%s</title>\n", goalReportHTMLText(title))
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
.grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(210px, 1fr)); gap: 10px; }
.metric { border: 1px solid var(--line); border-radius: 8px; padding: 10px; background: #fbfcfd; }
.metric strong { display: block; font-size: 20px; line-height: 1.2; }
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
	b.WriteString("<p class=\"eyebrow\">Gira goal report</p>\n")
	fmt.Fprintf(&b, "<h1>%s</h1>\n", goalReportHTMLText(goalReportIssueLabel(report.Goal.Number, report.Goal.Title)))
	b.WriteString("<div class=\"summary\">\n")
	goalReportWritePill(&b, "repo", report.Repo, "")
	goalReportWritePill(&b, "status", report.Goal.Status, "")
	goalReportWritePill(&b, "state", report.Goal.State, "")
	goalReportWritePill(&b, "next", report.NextAction, "next")
	goalReportWritePill(&b, "generated", report.GeneratedAt, "")
	b.WriteString("</div>\n")
	if href := goalReportSafeHref(report.Goal.URL); href != "" {
		fmt.Fprintf(&b, "<p class=\"meta\"><a href=\"%s\">Open goal</a></p>\n", href)
	}
	b.WriteString("</header>\n")

	b.WriteString("<section>\n<h2>Next</h2>\n")
	if report.SelectedTicket != nil {
		selected := report.SelectedTicket
		fmt.Fprintf(&b, "<p class=\"item-title\">Selected ticket: %s</p>\n", goalReportHTMLText(goalReportIssueLabel(selected.Number, selected.Title)))
		fmt.Fprintf(&b, "<p class=\"meta\">Reason: %s</p>\n", goalReportHTMLText(selected.Reason))
		if href := goalReportSafeHref(selected.URL); href != "" {
			fmt.Fprintf(&b, "<p class=\"meta\"><a href=\"%s\">Open selected ticket</a></p>\n", href)
		}
	} else {
		b.WriteString("<p class=\"empty\">No selected ticket.</p>\n")
	}
	if strings.TrimSpace(report.NextStep) != "" {
		fmt.Fprintf(&b, "<p class=\"meta\">Next step</p><code>%s</code>\n", goalReportHTMLText(report.NextStep))
	}
	b.WriteString("</section>\n")

	b.WriteString("<section>\n<h2>Counts</h2>\n<div class=\"grid\">\n")
	for _, pair := range goalReportCountPairs(report.Counts) {
		fmt.Fprintf(&b, "<div class=\"metric\"><strong>%d</strong><span>%s</span></div>\n", pair.count, goalReportHTMLText(pair.name))
	}
	b.WriteString("</div>\n</section>\n")
	if report.PMView != nil {
		view := report.PMView
		b.WriteString("<section>\n<h2>Product and PM state</h2>\n<div class=\"grid\">\n")
		fmt.Fprintf(&b, "<div class=\"metric\"><strong>%d/%d</strong><span>delivery done</span></div>\n", view.Delivery.Done, view.Delivery.Total)
		fmt.Fprintf(&b, "<div class=\"metric\"><strong>%d/%d</strong><span>outcomes validated</span></div>\n", view.Outcome.Validated, view.Outcome.Outcomes)
		fmt.Fprintf(&b, "<div class=\"metric\"><strong>%d</strong><span>residual decisions</span></div>\n", len(view.Residual))
		b.WriteString("</div>\n<div class=\"list\">\n")
		for _, summary := range view.Summaries {
			fmt.Fprintf(&b, "<p class=\"item\">%s</p>\n", goalReportHTMLText(summary))
		}
		b.WriteString("</div>\n")
		fmt.Fprintf(&b, "<p class=\"meta\">PM view=%s schema=%s digest=%s</p>\n", goalReportHTMLText(view.Kind), goalReportHTMLText(view.SchemaVersion), goalReportHTMLText(view.StateDigest))
		b.WriteString("</section>\n")
	}

	b.WriteString("<section>\n<h2>Tickets</h2>\n")
	if len(report.ChildGroups) == 0 {
		b.WriteString("<p class=\"empty\">No child tickets.</p>\n")
	} else {
		for _, group := range report.ChildGroups {
			fmt.Fprintf(&b, "<h3>%s (%d)</h3>\n<div class=\"list\">\n", goalReportHTMLText(group.Category), group.Count)
			for _, child := range group.Children {
				b.WriteString("<div class=\"item\">\n")
				goalReportWriteChildTitle(&b, child)
				fmt.Fprintf(&b, "<p class=\"meta\">status=%s checks=%s review=%s next=%s</p>\n", goalReportHTMLText(child.Status), goalReportHTMLText(child.ChecksStatus), goalReportHTMLText(child.ReviewStatus), goalReportHTMLText(child.NextAction))
				if child.PRNumber > 0 {
					fmt.Fprintf(&b, "<p class=\"meta\">PR #%d %s</p>\n", child.PRNumber, goalReportHTMLText(child.PRState))
				}
				if strings.TrimSpace(child.NextStep) != "" {
					fmt.Fprintf(&b, "<code>%s</code>\n", goalReportHTMLText(child.NextStep))
				}
				b.WriteString("</div>\n")
			}
			b.WriteString("</div>\n")
		}
	}
	b.WriteString("</section>\n")

	goalReportWriteStringListSection(&b, "Blockers", report.Blockers, "warn")
	goalReportWriteStringListSection(&b, "Stop", report.StopConditions, "warn")

	b.WriteString("<section>\n<h2>Evidence</h2>\n<div class=\"grid\">\n")
	fmt.Fprintf(&b, "<div class=\"metric\"><strong>%d</strong><span>children</span></div>\n", report.Evidence.ChildCount)
	fmt.Fprintf(&b, "<div class=\"metric\"><strong>%d</strong><span>remaining</span></div>\n", report.Evidence.RemainingAutonomousWork)
	fmt.Fprintf(&b, "<div class=\"metric\"><strong>%d</strong><span>blockers</span></div>\n", report.Evidence.BlockerCount)
	fmt.Fprintf(&b, "<div class=\"metric\"><strong>%d</strong><span>checks failing</span></div>\n", report.Evidence.Checks.Failing)
	if report.Measurement != nil {
		fmt.Fprintf(&b, "<div class=\"metric\"><strong>%d</strong><span>outcomes validated</span></div>\n", report.Measurement.Summary.Validated)
		fmt.Fprintf(&b, "<div class=\"metric\"><strong>%d</strong><span>guardrail blocked</span></div>\n", report.Measurement.Summary.Blocked)
	}
	b.WriteString("</div>\n")
	if len(report.Evidence.Sources) > 0 {
		fmt.Fprintf(&b, "<p class=\"meta\">Sources: %s</p>\n", goalReportHTMLText(strings.Join(report.Evidence.Sources, ", ")))
	}
	fmt.Fprintf(&b, "<p class=\"meta\">Schema: %s</p>\n", goalReportHTMLText(report.SchemaVersion))
	b.WriteString("</section>\n")

	b.WriteString("</main>\n</body>\n</html>\n")
	return b.String()
}

type goalReportCountPair struct {
	name  string
	count int
}

func goalReportCountPairs(counts map[string]int) []goalReportCountPair {
	order := []string{"total", "ready", "in_progress", "in_review", "blocked", "done", "closed_other", "unknown"}
	out := []goalReportCountPair{}
	seen := map[string]struct{}{}
	for _, name := range order {
		if count, ok := counts[name]; ok {
			out = append(out, goalReportCountPair{name: name, count: count})
			seen[name] = struct{}{}
		}
	}
	extra := []string{}
	for name := range counts {
		if _, ok := seen[name]; !ok {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	for _, name := range extra {
		out = append(out, goalReportCountPair{name: name, count: counts[name]})
	}
	if len(out) == 0 {
		out = append(out, goalReportCountPair{name: "total", count: 0})
	}
	return out
}

func goalReportWriteChildTitle(b *strings.Builder, child GoalStatusChild) {
	label := goalReportHTMLText(goalReportIssueLabel(child.Number, child.Title))
	if href := goalReportSafeHref(child.URL); href != "" {
		fmt.Fprintf(b, "<p class=\"item-title\"><a href=\"%s\">%s</a></p>\n", href, label)
		return
	}
	fmt.Fprintf(b, "<p class=\"item-title\">%s</p>\n", label)
}

func goalReportWriteStringListSection(b *strings.Builder, title string, values []string, className string) {
	fmt.Fprintf(b, "<section class=\"%s\">\n<h2>%s</h2>\n", goalReportHTMLText(className), goalReportHTMLText(title))
	if len(values) == 0 {
		b.WriteString("<p class=\"empty\">None.</p>\n</section>\n")
		return
	}
	b.WriteString("<div class=\"list\">\n")
	for _, value := range values {
		fmt.Fprintf(b, "<div class=\"item\">%s</div>\n", goalReportHTMLText(value))
	}
	b.WriteString("</div>\n</section>\n")
}

func goalReportWritePill(b *strings.Builder, label string, value string, className string) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "none"
	}
	classes := "pill"
	if strings.TrimSpace(className) != "" {
		classes += " " + className
	}
	fmt.Fprintf(b, "<span class=\"%s\">%s: %s</span>\n", goalReportHTMLText(classes), goalReportHTMLText(label), goalReportHTMLText(value))
}

func goalReportIssueLabel(number int, title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Sprintf("#%d", number)
	}
	return fmt.Sprintf("#%d %s", number, title)
}

func goalReportSafeHref(value string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		return goalReportHTMLText(value)
	}
	return ""
}

func goalReportHTMLText(value string) string {
	return html.EscapeString(value)
}
