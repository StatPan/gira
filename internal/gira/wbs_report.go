package gira

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"
	"time"
)

const WBSReportSchemaVersion = "wbs-report/v1alpha1"

var wbsReportCSVHeaders = []string{"wbs_id", "parent_id", "level", "kind", "repo", "issue", "title", "state", "status", "priority", "owner", "milestone", "start_date", "target_date", "progress", "children", "source", "url"}

type WBSReportClient interface {
	FetchIssues() ([]WBSRawIssue, error)
	FetchMilestones() ([]DashboardRawMilestone, error)
	FetchProjectSnapshot() (ProjectSyncSnapshot, error)
}

type WBSRawIssue struct {
	IssueNumber int      `json:"issue_number"`
	Title       string   `json:"title"`
	State       string   `json:"state"`
	Body        string   `json:"body,omitempty"`
	Labels      []string `json:"labels"`
	Milestone   string   `json:"milestone,omitempty"`
	URL         string   `json:"url"`
}

type WBSReport struct {
	Command       string            `json:"command"`
	SchemaVersion string            `json:"schema_version"`
	Repo          string            `json:"repo"`
	StateFilter   string            `json:"state_filter"`
	GeneratedAt   string            `json:"generated_at"`
	Items         []WBSReportItem   `json:"items"`
	Counts        WBSReportCounts   `json:"counts"`
	Warnings      []string          `json:"warnings,omitempty"`
	Sources       []WBSReportSource `json:"sources"`
}

type WBSReportItem struct {
	WBSID      string   `json:"wbs_id"`
	ParentID   string   `json:"parent_id,omitempty"`
	Level      int      `json:"level"`
	Kind       string   `json:"kind"`
	Repo       string   `json:"repo"`
	Issue      int      `json:"issue,omitempty"`
	Title      string   `json:"title"`
	State      string   `json:"state,omitempty"`
	Status     string   `json:"status,omitempty"`
	Priority   string   `json:"priority,omitempty"`
	Owner      string   `json:"owner,omitempty"`
	Milestone  string   `json:"milestone,omitempty"`
	StartDate  string   `json:"start_date,omitempty"`
	TargetDate string   `json:"target_date,omitempty"`
	Progress   int      `json:"progress"`
	Children   int      `json:"children"`
	Source     string   `json:"source"`
	URL        string   `json:"url,omitempty"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type WBSReportCounts struct {
	Epics         int `json:"epics"`
	Issues        int `json:"issues"`
	LinkedIssues  int `json:"linked_issues"`
	UnlinkedItems int `json:"unlinked_items"`
	Milestones    int `json:"milestones"`
}

type WBSReportSource struct {
	Name          string `json:"name"`
	SchemaVersion string `json:"schema_version,omitempty"`
}

type WBSReportOptions struct {
	State string `json:"state,omitempty"`
}

type GHWBSReportClient struct {
	repo   RepoRef
	runner CommandRunner
}

func NewGHWBSReportClient(repo RepoRef, runner CommandRunner) GHWBSReportClient {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	return GHWBSReportClient{repo: repo, runner: runner}
}

func (c GHWBSReportClient) FetchIssues() ([]WBSRawIssue, error) {
	issues, err := fetchEpicIssues(c.repo, "all", c.runner)
	if err != nil {
		return nil, err
	}
	out := make([]WBSRawIssue, 0, len(issues))
	for _, issue := range issues {
		labels := rawLabels(issue)
		sort.Strings(labels)
		out = append(out, WBSRawIssue{
			IssueNumber: issue.Number,
			Title:       issue.Title,
			State:       issue.State,
			Body:        issue.Body,
			Labels:      labels,
			Milestone:   rawMilestone(issue),
			URL:         issue.HTMLURL,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IssueNumber == out[j].IssueNumber {
			return out[i].Title < out[j].Title
		}
		return out[i].IssueNumber < out[j].IssueNumber
	})
	return out, nil
}

func (c GHWBSReportClient) FetchMilestones() ([]DashboardRawMilestone, error) {
	client := NewGHDashboardExportClient(c.repo, c.runner)
	return client.FetchMilestones()
}

func (c GHWBSReportClient) FetchProjectSnapshot() (ProjectSyncSnapshot, error) {
	client := NewGHProjectSyncClient(c.repo, c.runner)
	return client.Snapshot(ProductOSProjectName)
}

func BuildWBSReport(repo RepoRef, client WBSReportClient, generatedAt time.Time) (WBSReport, error) {
	return BuildWBSReportWithOptions(repo, client, generatedAt, WBSReportOptions{State: "all"})
}

func BuildWBSReportWithOptions(repo RepoRef, client WBSReportClient, generatedAt time.Time, options WBSReportOptions) (WBSReport, error) {
	if client == nil {
		return WBSReport{}, fmt.Errorf("wbs report client is required")
	}
	state, err := normalizeWBSState(options.State)
	if err != nil {
		return WBSReport{}, err
	}
	issues, err := client.FetchIssues()
	if err != nil {
		return WBSReport{}, err
	}
	issues = filterWBSIssuesByState(issues, state)
	milestones, err := client.FetchMilestones()
	if err != nil {
		return WBSReport{}, err
	}
	projectSnapshot, err := client.FetchProjectSnapshot()
	warnings := []string{}
	if err != nil {
		warnings = append(warnings, "project_snapshot_unavailable: "+err.Error())
	}
	report := WBSReport{
		Command:       "report wbs",
		SchemaVersion: WBSReportSchemaVersion,
		Repo:          repo.FullName(),
		StateFilter:   state,
		GeneratedAt:   generatedAt.UTC().Format(time.RFC3339),
		Counts:        WBSReportCounts{Issues: len(issues), Milestones: len(milestones)},
		Warnings:      warnings,
		Sources: []WBSReportSource{
			{Name: "github_issues"},
			{Name: "github_milestones"},
			{Name: "project_snapshot"},
		},
	}
	report.Items, report.Warnings = buildWBSReportItems(repo, issues, milestones, projectSnapshot, report.Warnings)
	for _, item := range report.Items {
		if item.Kind == "epic" {
			report.Counts.Epics++
		}
		if item.ParentID != "" {
			report.Counts.LinkedIssues++
		} else if item.Kind != "epic" {
			report.Counts.UnlinkedItems++
		}
	}
	return report, nil
}

func normalizeWBSState(state string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "", "open":
		return "open", nil
	case "closed":
		return "closed", nil
	case "all":
		return "all", nil
	default:
		return "", fmt.Errorf("--state must be open, closed, or all")
	}
}

func filterWBSIssuesByState(issues []WBSRawIssue, state string) []WBSRawIssue {
	if state == "all" {
		return issues
	}
	out := make([]WBSRawIssue, 0, len(issues))
	for _, issue := range issues {
		if strings.EqualFold(issue.State, state) {
			out = append(out, issue)
		}
	}
	return out
}

func buildWBSReportItems(repo RepoRef, issues []WBSRawIssue, milestones []DashboardRawMilestone, projectSnapshot ProjectSyncSnapshot, warnings []string) ([]WBSReportItem, []string) {
	datesByIssue := wbsProjectDates(projectSnapshot.RoadmapItems)
	milestoneDue := wbsMilestoneDueDates(milestones)
	epics := []WBSRawIssue{}
	nonEpics := []WBSRawIssue{}
	for _, issue := range issues {
		if wbsTypeLabel(issue.Labels) == "type:epic" {
			epics = append(epics, issue)
			continue
		}
		nonEpics = append(nonEpics, issue)
	}
	sort.Slice(epics, func(i, j int) bool { return epics[i].IssueNumber < epics[j].IssueNumber })
	sort.Slice(nonEpics, func(i, j int) bool { return nonEpics[i].IssueNumber < nonEpics[j].IssueNumber })

	childrenByEpic := map[int][]WBSRawIssue{}
	linked := map[int]struct{}{}
	for _, epic := range epics {
		refs := extractIssueRefs(epic.Body)
		for _, issue := range nonEpics {
			if _, ok := refs[issue.IssueNumber]; ok {
				childrenByEpic[epic.IssueNumber] = append(childrenByEpic[epic.IssueNumber], issue)
				linked[issue.IssueNumber] = struct{}{}
			}
		}
	}

	epicsByMilestone := map[string][]WBSRawIssue{}
	for _, epic := range epics {
		if strings.TrimSpace(epic.Milestone) != "" {
			epicsByMilestone[epic.Milestone] = append(epicsByMilestone[epic.Milestone], epic)
		}
	}
	for milestone, milestoneEpics := range epicsByMilestone {
		if len(milestoneEpics) > 1 {
			warnings = appendUniqueStrings(warnings, "ambiguous_milestone_parent:"+milestone)
		}
	}
	for _, issue := range nonEpics {
		if _, ok := linked[issue.IssueNumber]; ok {
			continue
		}
		if strings.TrimSpace(issue.Milestone) == "" {
			continue
		}
		milestoneEpics := epicsByMilestone[issue.Milestone]
		if len(milestoneEpics) != 1 {
			continue
		}
		epic := milestoneEpics[0]
		childrenByEpic[epic.IssueNumber] = append(childrenByEpic[epic.IssueNumber], issue)
		linked[issue.IssueNumber] = struct{}{}
	}
	for epicNumber := range childrenByEpic {
		sort.Slice(childrenByEpic[epicNumber], func(i, j int) bool {
			return childrenByEpic[epicNumber][i].IssueNumber < childrenByEpic[epicNumber][j].IssueNumber
		})
	}

	items := []WBSReportItem{}
	nextTop := 1
	for _, epic := range epics {
		topID := strconv.Itoa(nextTop)
		children := childrenByEpic[epic.IssueNumber]
		item := wbsItemFromIssue(repo, epic, topID, "", 1, datesByIssue, milestoneDue, "epic")
		item.Children = len(children)
		item.Progress = wbsProgress(children, epic.State)
		items = append(items, item)
		for i, child := range children {
			childID := fmt.Sprintf("%s.%d", topID, i+1)
			source := "milestone"
			if refs := extractIssueRefs(epic.Body); len(refs) > 0 {
				if _, ok := refs[child.IssueNumber]; ok {
					source = "body"
					if strings.TrimSpace(child.Milestone) != "" && child.Milestone == epic.Milestone {
						source = "body,milestone"
					}
				}
			}
			items = append(items, wbsItemFromIssue(repo, child, childID, topID, 2, datesByIssue, milestoneDue, source))
		}
		nextTop++
	}
	for _, issue := range nonEpics {
		if _, ok := linked[issue.IssueNumber]; ok {
			continue
		}
		items = append(items, wbsItemFromIssue(repo, issue, strconv.Itoa(nextTop), "", 1, datesByIssue, milestoneDue, "unlinked"))
		nextTop++
	}
	return items, warnings
}

func wbsItemFromIssue(repo RepoRef, issue WBSRawIssue, id string, parentID string, level int, dates map[int]wbsDates, milestoneDue map[string]string, source string) WBSReportItem {
	datesForIssue := dates[issue.IssueNumber]
	targetDate := datesForIssue.TargetDate
	if targetDate == "" && strings.TrimSpace(issue.Milestone) != "" {
		targetDate = milestoneDue[issue.Milestone]
	}
	kind := strings.TrimPrefix(wbsTypeLabel(issue.Labels), "type:")
	if kind == "" {
		kind = "issue"
	}
	progress := 0
	if strings.EqualFold(issue.State, "closed") {
		progress = 100
	}
	return WBSReportItem{
		WBSID:      id,
		ParentID:   parentID,
		Level:      level,
		Kind:       kind,
		Repo:       repo.FullName(),
		Issue:      issue.IssueNumber,
		Title:      issue.Title,
		State:      strings.ToLower(issue.State),
		Status:     dashboardExportStatusFromLabels(issue.Labels),
		Priority:   dashboardExportPriorityFromLabels(issue.Labels),
		Owner:      dashboardExportOwnerFromLabels(issue.Labels),
		Milestone:  issue.Milestone,
		StartDate:  datesForIssue.StartDate,
		TargetDate: targetDate,
		Progress:   progress,
		Source:     source,
		URL:        issue.URL,
		SourceRefs: []string{fmt.Sprintf("issue:%d", issue.IssueNumber)},
	}
}

type wbsDates struct {
	StartDate  string
	TargetDate string
}

func wbsProjectDates(items []ProjectRoadmapItem) map[int]wbsDates {
	out := map[int]wbsDates{}
	for _, item := range items {
		dates := wbsDates{}
		if item.StartDate != nil {
			dates.StartDate = strings.TrimSpace(*item.StartDate)
		}
		if item.TargetDate != nil {
			dates.TargetDate = strings.TrimSpace(*item.TargetDate)
		}
		if dates.StartDate == "" && dates.TargetDate == "" {
			continue
		}
		out[item.IssueNumber] = dates
	}
	return out
}

func wbsMilestoneDueDates(milestones []DashboardRawMilestone) map[string]string {
	out := map[string]string{}
	for _, milestone := range milestones {
		if milestone.DueOn == nil {
			continue
		}
		if normalized, ok := normalizeDate(*milestone.DueOn); ok {
			out[milestone.Title] = normalized
		}
	}
	return out
}

func wbsProgress(children []WBSRawIssue, epicState string) int {
	if len(children) == 0 {
		if strings.EqualFold(epicState, "closed") {
			return 100
		}
		return 0
	}
	closed := 0
	for _, child := range children {
		if strings.EqualFold(child.State, "closed") {
			closed++
		}
	}
	return int(float64(closed)/float64(len(children))*100 + 0.5)
}

func wbsTypeLabel(labels []string) string {
	for _, label := range labels {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(label)), "type:") {
			return strings.ToLower(strings.TrimSpace(label))
		}
	}
	return ""
}

func RenderWBSReportCSV(report WBSReport) ([]byte, error) {
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	if err := writer.Write(wbsReportCSVHeaders); err != nil {
		return nil, err
	}
	for _, item := range report.Items {
		row := []string{
			item.WBSID,
			item.ParentID,
			strconv.Itoa(item.Level),
			item.Kind,
			item.Repo,
			wbsIssueNumber(item.Issue),
			item.Title,
			item.State,
			item.Status,
			item.Priority,
			item.Owner,
			item.Milestone,
			item.StartDate,
			item.TargetDate,
			strconv.Itoa(item.Progress),
			strconv.Itoa(item.Children),
			item.Source,
			item.URL,
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return buffer.Bytes(), writer.Error()
}

func RenderWBSReportJSON(report WBSReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

func FormatWBSReport(report WBSReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "wbs report: %s items=%d epics=%d linked=%d unlinked=%d\n", report.Repo, len(report.Items), report.Counts.Epics, report.Counts.LinkedIssues, report.Counts.UnlinkedItems)
	if len(report.Warnings) > 0 {
		fmt.Fprintf(&b, "warnings: %s\n", strings.Join(report.Warnings, ","))
	}
	for _, item := range report.Items {
		indent := strings.Repeat("  ", wbsMaxInt(0, item.Level-1))
		target := ""
		if item.TargetDate != "" {
			target = " target=" + item.TargetDate
		}
		fmt.Fprintf(&b, "%s%s #%d %s [%s]%s\n", indent, item.WBSID, item.Issue, item.Title, item.Status, target)
	}
	return b.String()
}

func RenderWBSReportHTML(report WBSReport) string {
	var b strings.Builder
	title := "Gira WBS Report"
	if report.Repo != "" {
		title += " - " + report.Repo
	}
	b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n")
	b.WriteString("<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	fmt.Fprintf(&b, "<title>%s</title>\n", html.EscapeString(title))
	b.WriteString(`<style>
:root {
  color-scheme: light;
  --bg: #f6f7f9;
  --panel: #ffffff;
  --text: #20242a;
  --muted: #606b78;
  --line: #d8dde4;
  --accent: #1769aa;
  --good: #1f7a4d;
  --warn: #935f00;
  --soft: #f2f4f7;
}
* { box-sizing: border-box; }
body { margin: 0; background: var(--bg); color: var(--text); font: 14px/1.5 -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
main { max-width: 1180px; margin: 0 auto; padding: 28px 18px 42px; }
header, section { background: var(--panel); border: 1px solid var(--line); border-radius: 8px; margin: 0 0 14px; padding: 18px; }
h1, h2, p { margin: 0; }
h1 { font-size: 24px; line-height: 1.2; }
h2 { font-size: 16px; margin-bottom: 10px; }
a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }
.eyebrow { color: var(--muted); font-size: 12px; text-transform: uppercase; margin-bottom: 4px; }
.summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 10px; margin-top: 14px; }
.metric { border: 1px solid var(--line); border-radius: 8px; padding: 10px; background: #fbfcfd; }
.metric strong { display: block; font-size: 20px; line-height: 1.2; overflow-wrap: anywhere; }
.metric span { color: var(--muted); }
table { width: 100%; border-collapse: collapse; }
th, td { border-top: 1px solid var(--line); padding: 7px 6px; text-align: left; vertical-align: top; }
th { color: var(--muted); font-weight: 600; }
.muted { color: var(--muted); }
.tree-title { font-weight: 600; }
.level-2 .tree-title { padding-left: 18px; }
.bar { height: 8px; min-width: 80px; border-radius: 999px; background: var(--soft); overflow: hidden; }
.bar span { display: block; height: 100%; background: var(--good); }
.warn { color: var(--warn); }
@media print {
  body { background: #fff; }
  main { max-width: none; padding: 0; }
  header, section { break-inside: avoid; border-color: #ccc; }
}
</style>
`)
	b.WriteString("</head>\n<body>\n<main>\n<header>\n")
	b.WriteString("<p class=\"eyebrow\">Gira WBS report</p>\n")
	fmt.Fprintf(&b, "<h1>%s</h1>\n", html.EscapeString(report.Repo))
	b.WriteString("<div class=\"summary\">\n")
	wbsHTMLMetric(&b, "items", strconv.Itoa(len(report.Items)))
	wbsHTMLMetric(&b, "epics", strconv.Itoa(report.Counts.Epics))
	wbsHTMLMetric(&b, "linked issues", strconv.Itoa(report.Counts.LinkedIssues))
	wbsHTMLMetric(&b, "unlinked", strconv.Itoa(report.Counts.UnlinkedItems))
	b.WriteString("</div>\n")
	fmt.Fprintf(&b, "<p class=\"muted\">Generated at %s</p>\n", html.EscapeString(report.GeneratedAt))
	b.WriteString("</header>\n")
	if len(report.Warnings) > 0 {
		b.WriteString("<section>\n<h2>Warnings</h2>\n<ul>\n")
		for _, warning := range report.Warnings {
			fmt.Fprintf(&b, "<li class=\"warn\">%s</li>\n", html.EscapeString(warning))
		}
		b.WriteString("</ul>\n</section>\n")
	}
	b.WriteString("<section>\n<h2>Work Breakdown</h2>\n<table>\n")
	b.WriteString("<thead><tr><th>WBS</th><th>Work item</th><th>Status</th><th>Milestone</th><th>Target</th><th>Progress</th></tr></thead>\n<tbody>\n")
	for _, item := range report.Items {
		titleHTML := html.EscapeString(item.Title)
		if item.URL != "" {
			titleHTML = fmt.Sprintf("<a href=\"%s\">%s</a>", html.EscapeString(item.URL), titleHTML)
		}
		fmt.Fprintf(&b, "<tr class=\"level-%d\"><td>%s</td><td><span class=\"tree-title\">%s</span><br><span class=\"muted\">%s #%s</span></td><td>%s</td><td>%s</td><td>%s</td><td><div class=\"bar\"><span style=\"width:%d%%\"></span></div><span class=\"muted\">%d%%</span></td></tr>\n",
			item.Level,
			html.EscapeString(item.WBSID),
			titleHTML,
			html.EscapeString(item.Kind),
			html.EscapeString(wbsIssueNumber(item.Issue)),
			html.EscapeString(item.Status),
			html.EscapeString(item.Milestone),
			html.EscapeString(item.TargetDate),
			item.Progress,
			item.Progress,
		)
	}
	b.WriteString("</tbody>\n</table>\n</section>\n")
	b.WriteString("</main>\n</body>\n</html>\n")
	return b.String()
}

func WriteWBSReportBundle(outputRoot string, report WBSReport) error {
	scope, err := newLocalWriteScope(outputRoot)
	if err != nil {
		return err
	}
	csvBytes, err := RenderWBSReportCSV(report)
	if err != nil {
		return err
	}
	jsonBytes, err := RenderWBSReportJSON(report)
	if err != nil {
		return err
	}
	if err := scope.WriteFile("index.html", []byte(RenderWBSReportHTML(report)), 0o644); err != nil {
		return err
	}
	if err := scope.WriteFile("csv/wbs_items.csv", csvBytes, 0o644); err != nil {
		return err
	}
	if err := scope.WriteFile("derived/wbs_tree.json", append(jsonBytes, '\n'), 0o644); err != nil {
		return err
	}
	return nil
}

func WriteWBSReportHTML(path string, report WBSReport) error {
	return writeSafeLocalFile(path, []byte(RenderWBSReportHTML(report)), 0o644)
}

func WriteWBSReportCSV(path string, report WBSReport) error {
	csvBytes, err := RenderWBSReportCSV(report)
	if err != nil {
		return err
	}
	return writeSafeLocalFile(path, csvBytes, 0o644)
}

func wbsHTMLMetric(b *strings.Builder, label string, value string) {
	fmt.Fprintf(b, "<div class=\"metric\"><strong>%s</strong><span>%s</span></div>\n", html.EscapeString(value), html.EscapeString(label))
}

func wbsIssueNumber(number int) string {
	if number <= 0 {
		return ""
	}
	return strconv.Itoa(number)
}

func wbsMaxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
