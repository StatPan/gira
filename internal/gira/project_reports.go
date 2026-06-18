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

const ProjectReportSchemaVersion = "project-report/v1alpha1"

var projectReportCSVHeaders = []string{"kind", "repo", "issue", "pr", "title", "group", "status", "priority", "milestone", "age_days", "evidence", "warnings", "url"}

type ProjectReportOptions struct {
	Kind      string
	Milestone string
}

type ProjectReport struct {
	Command       string                 `json:"command"`
	SchemaVersion string                 `json:"schema_version"`
	Repo          string                 `json:"repo"`
	Kind          string                 `json:"kind"`
	Title         string                 `json:"title"`
	GeneratedAt   string                 `json:"generated_at"`
	Confidence    string                 `json:"confidence"`
	Counts        ProjectReportCounts    `json:"counts"`
	Sections      []ProjectReportSection `json:"sections"`
	Items         []ProjectReportItem    `json:"items"`
	Warnings      []string               `json:"warnings,omitempty"`
	Sources       []ProjectReportSource  `json:"sources"`
}

type ProjectReportCounts struct {
	OpenIssues        int `json:"open_issues"`
	ClosedIssues      int `json:"closed_issues"`
	OpenPRs           int `json:"open_prs"`
	Blocked           int `json:"blocked"`
	Stale             int `json:"stale"`
	Unplanned         int `json:"unplanned"`
	Milestones        int `json:"milestones"`
	Warnings          int `json:"warnings"`
	CompletionPct     int `json:"completion_pct,omitempty"`
	ChecklistItems    int `json:"checklist_items,omitempty"`
	ChecklistComplete int `json:"checklist_complete,omitempty"`
}

type ProjectReportSection struct {
	Group string `json:"group"`
	Title string `json:"title"`
	Count int    `json:"count"`
}

type ProjectReportItem struct {
	Kind      string   `json:"kind"`
	Repo      string   `json:"repo"`
	Issue     int      `json:"issue,omitempty"`
	PR        int      `json:"pr,omitempty"`
	Title     string   `json:"title"`
	Group     string   `json:"group"`
	Status    string   `json:"status"`
	Priority  string   `json:"priority,omitempty"`
	Milestone string   `json:"milestone,omitempty"`
	AgeDays   int      `json:"age_days,omitempty"`
	Evidence  []string `json:"evidence"`
	Warnings  []string `json:"warnings,omitempty"`
	URL       string   `json:"url,omitempty"`
}

type ProjectReportSource struct {
	Name          string `json:"name"`
	SchemaVersion string `json:"schema_version,omitempty"`
}

func BuildProjectReport(repo RepoRef, dashboard DashboardExportClient, review ReviewGateClient, now time.Time, options ProjectReportOptions) (ProjectReport, error) {
	kind := strings.ToLower(strings.TrimSpace(options.Kind))
	if kind == "" {
		return ProjectReport{}, fmt.Errorf("report kind is required")
	}
	if dashboard == nil {
		return ProjectReport{}, fmt.Errorf("dashboard client is required")
	}
	if review == nil {
		return ProjectReport{}, fmt.Errorf("review client is required")
	}
	issues, err := dashboard.FetchIssues()
	if err != nil {
		return ProjectReport{}, err
	}
	milestones, err := dashboard.FetchMilestones()
	if err != nil {
		return ProjectReport{}, err
	}
	openPRs, prWarning, err := projectReportOpenPRs(dashboard, review)
	if err != nil {
		return ProjectReport{}, err
	}
	report := ProjectReport{
		Command:       "report " + kind,
		SchemaVersion: ProjectReportSchemaVersion,
		Repo:          repo.FullName(),
		Kind:          kind,
		Title:         projectReportTitle(kind, options.Milestone),
		GeneratedAt:   now.UTC().Format(time.RFC3339),
		Confidence:    "ready",
		Sources: []ProjectReportSource{
			{Name: "github_issues"},
			{Name: "github_milestones"},
			{Name: "github_pull_requests"},
		},
	}
	if prWarning != "" {
		report.Warnings = appendUniqueStrings(report.Warnings, prWarning)
	}
	switch kind {
	case "milestone":
		if strings.TrimSpace(options.Milestone) == "" {
			return ProjectReport{}, fmt.Errorf("--milestone is required")
		}
		buildMilestoneProjectReport(&report, issues, milestones, now, strings.TrimSpace(options.Milestone))
	case "backlog-health":
		buildBacklogHealthProjectReport(&report, issues, now)
	case "delivery-status":
		buildDeliveryStatusProjectReport(&report, issues, milestones, openPRs, now)
	case "qa-checklist":
		buildQAChecklistProjectReport(&report, issues, openPRs, now, strings.TrimSpace(options.Milestone))
	default:
		return ProjectReport{}, fmt.Errorf("unsupported project report kind %q", kind)
	}
	finalizeProjectReport(&report)
	return report, nil
}

func projectReportOpenPRs(dashboard DashboardExportClient, review ReviewGateClient) ([]ReviewPR, string, error) {
	openPRs, err := review.ListOpenPRs()
	if err == nil {
		return openPRs, "", nil
	}
	rawPRs, fallbackErr := dashboard.FetchPullRequests()
	if fallbackErr != nil {
		return nil, "", err
	}
	fallback := make([]ReviewPR, 0, len(rawPRs))
	for _, pr := range rawPRs {
		if !strings.EqualFold(pr.State, "open") {
			continue
		}
		fallback = append(fallback, ReviewPR{
			Number:      pr.PullRequestNumber,
			Title:       pr.Title,
			URL:         pr.URL,
			IsDraft:     pr.Draft,
			CheckStatus: "unknown",
			Labels:      append([]string(nil), pr.Labels...),
		})
	}
	return fallback, "open_pr_review_evidence_unavailable", nil
}

func buildMilestoneProjectReport(report *ProjectReport, issues []DashboardRawIssue, milestones []DashboardRawMilestone, now time.Time, milestoneTitle string) {
	found := false
	for _, milestone := range milestones {
		if milestone.Title == milestoneTitle {
			found = true
			report.Counts.OpenIssues = milestone.OpenIssues
			report.Counts.ClosedIssues = milestone.ClosedIssues
			total := milestone.OpenIssues + milestone.ClosedIssues
			if total > 0 {
				report.Counts.CompletionPct = (milestone.ClosedIssues * 100) / total
			}
			if milestone.State != "open" && milestone.OpenIssues > 0 {
				report.Warnings = appendUniqueStrings(report.Warnings, "closed_milestone_has_open_issues")
			}
			if milestone.DueOn == nil || strings.TrimSpace(*milestone.DueOn) == "" {
				report.Warnings = appendUniqueStrings(report.Warnings, "milestone_missing_due_date")
			}
			break
		}
	}
	if !found {
		report.Warnings = appendUniqueStrings(report.Warnings, "milestone_not_found")
	}
	for _, issue := range issues {
		if issue.Milestone != milestoneTitle {
			continue
		}
		item := projectIssueItem(report.Repo, issue, now)
		item.Group = "done"
		if strings.EqualFold(issue.State, "open") {
			item.Group = "remaining"
		}
		if hasSubstring(issue.Labels, "blocked") {
			item.Group = "risk"
			item.Warnings = appendUniqueStrings(item.Warnings, "blocked")
		}
		if wbsTypeLabel(issue.Labels) == "" {
			item.Warnings = appendUniqueStrings(item.Warnings, "missing_type_label")
		}
		report.Items = append(report.Items, item)
	}
}

func buildBacklogHealthProjectReport(report *ProjectReport, issues []DashboardRawIssue, now time.Time) {
	for _, issue := range issues {
		if !strings.EqualFold(issue.State, "open") {
			continue
		}
		item := projectIssueItem(report.Repo, issue, now)
		item.Group = "ready"
		if strings.TrimSpace(issue.Milestone) == "" {
			item.Group = "unplanned"
			item.Warnings = appendUniqueStrings(item.Warnings, "missing_milestone")
		}
		if item.AgeDays >= 14 {
			item.Group = "stale"
			item.Warnings = appendUniqueStrings(item.Warnings, "stale_14d")
		}
		if hasSubstring(issue.Labels, "blocked") {
			item.Group = "blocked"
			item.Warnings = appendUniqueStrings(item.Warnings, "blocked")
		}
		if wbsTypeLabel(issue.Labels) == "" {
			item.Warnings = appendUniqueStrings(item.Warnings, "missing_type_label")
		}
		report.Items = append(report.Items, item)
	}
}

func buildDeliveryStatusProjectReport(report *ProjectReport, issues []DashboardRawIssue, milestones []DashboardRawMilestone, openPRs []ReviewPR, now time.Time) {
	report.Counts.Milestones = len(milestones)
	for _, milestone := range milestones {
		if milestone.State != "open" {
			continue
		}
		total := milestone.OpenIssues + milestone.ClosedIssues
		progress := 0
		if total > 0 {
			progress = (milestone.ClosedIssues * 100) / total
		}
		item := ProjectReportItem{
			Kind:      "milestone",
			Repo:      report.Repo,
			Title:     milestone.Title,
			Group:     "milestone",
			Status:    fmt.Sprintf("%d%% complete", progress),
			Milestone: milestone.Title,
			Evidence:  []string{"github_milestone"},
		}
		if milestone.OpenIssues > 0 {
			item.Warnings = appendUniqueStrings(item.Warnings, fmt.Sprintf("%d_open_issues", milestone.OpenIssues))
		}
		if milestone.DueOn == nil || strings.TrimSpace(*milestone.DueOn) == "" {
			item.Warnings = appendUniqueStrings(item.Warnings, "missing_due_date")
		}
		report.Items = append(report.Items, item)
	}
	for _, issue := range issues {
		if !strings.EqualFold(issue.State, "open") || !hasSubstring(issue.Labels, "blocked") {
			continue
		}
		item := projectIssueItem(report.Repo, issue, now)
		item.Group = "risk"
		item.Warnings = appendUniqueStrings(item.Warnings, "blocked")
		report.Items = append(report.Items, item)
	}
	for _, pr := range openPRs {
		item := projectPRItem(report.Repo, pr, now)
		item.Group = "review"
		if pr.IsDraft {
			item.Group = "draft"
			item.Warnings = appendUniqueStrings(item.Warnings, "draft_pr")
		}
		if pr.CheckStatus == "failing" {
			item.Group = "risk"
			item.Warnings = appendUniqueStrings(item.Warnings, "failing_checks")
		}
		if pr.ReviewDecision != "APPROVED" {
			item.Warnings = appendUniqueStrings(item.Warnings, "review_not_approved")
		}
		report.Items = append(report.Items, item)
	}
}

func buildQAChecklistProjectReport(report *ProjectReport, issues []DashboardRawIssue, openPRs []ReviewPR, now time.Time, milestoneTitle string) {
	for _, issue := range issues {
		if milestoneTitle != "" && issue.Milestone != milestoneTitle {
			continue
		}
		item := projectIssueItem(report.Repo, issue, now)
		item.Kind = "check"
		item.Group = "issue_readiness"
		item.Status = "pass"
		if strings.EqualFold(issue.State, "open") {
			item.Status = "review"
			item.Warnings = appendUniqueStrings(item.Warnings, "issue_open")
		}
		if wbsTypeLabel(issue.Labels) == "" {
			item.Status = "review"
			item.Warnings = appendUniqueStrings(item.Warnings, "missing_type_label")
		}
		if !hasSubstring(issue.Labels, "qa") && !hasSubstring(issue.Labels, "test") && !hasSubstring(issue.Labels, "docs") {
			item.Status = "review"
			item.Warnings = appendUniqueStrings(item.Warnings, "missing_qa_evidence_label")
		}
		report.Items = append(report.Items, item)
	}
	for _, pr := range openPRs {
		item := projectPRItem(report.Repo, pr, now)
		item.Kind = "check"
		item.Group = "pr_readiness"
		item.Status = "pass"
		if pr.IsDraft {
			item.Status = "review"
			item.Warnings = appendUniqueStrings(item.Warnings, "draft_pr")
		}
		if pr.CheckStatus == "failing" || pr.CheckStatus == "pending" {
			item.Status = "review"
			item.Warnings = appendUniqueStrings(item.Warnings, "checks_not_passing")
		}
		if pr.ReviewDecision != "APPROVED" {
			item.Status = "review"
			item.Warnings = appendUniqueStrings(item.Warnings, "review_not_approved")
		}
		if len(ExtractClosureIssueNumbers(pr.Body)) == 0 {
			item.Status = "review"
			item.Warnings = appendUniqueStrings(item.Warnings, "missing_closure_link")
		}
		report.Items = append(report.Items, item)
	}
}

func finalizeProjectReport(report *ProjectReport) {
	groups := map[string]int{}
	for _, item := range report.Items {
		groups[item.Group]++
		switch {
		case item.Kind == "issue" && strings.EqualFold(item.Status, "open"):
			report.Counts.OpenIssues++
		case item.Kind == "issue" && strings.EqualFold(item.Status, "closed"):
			report.Counts.ClosedIssues++
		case item.Kind == "pr":
			report.Counts.OpenPRs++
		}
		if item.Group == "blocked" || projectHasString(item.Warnings, "blocked") {
			report.Counts.Blocked++
		}
		if item.Group == "stale" || projectHasString(item.Warnings, "stale_14d") {
			report.Counts.Stale++
		}
		if item.Group == "unplanned" || projectHasString(item.Warnings, "missing_milestone") {
			report.Counts.Unplanned++
		}
		if item.Kind == "check" {
			report.Counts.ChecklistItems++
			if strings.EqualFold(item.Status, "pass") {
				report.Counts.ChecklistComplete++
			}
		}
		for _, warning := range item.Warnings {
			report.Warnings = appendUniqueStrings(report.Warnings, warning)
		}
	}
	groupNames := make([]string, 0, len(groups))
	for group := range groups {
		groupNames = append(groupNames, group)
	}
	sort.Strings(groupNames)
	for _, group := range groupNames {
		report.Sections = append(report.Sections, ProjectReportSection{Group: group, Title: projectReportGroupTitle(group), Count: groups[group]})
	}
	sort.Slice(report.Items, func(i, j int) bool {
		if report.Items[i].Group != report.Items[j].Group {
			return report.Items[i].Group < report.Items[j].Group
		}
		if report.Items[i].Issue != report.Items[j].Issue {
			return report.Items[i].Issue < report.Items[j].Issue
		}
		return report.Items[i].Title < report.Items[j].Title
	})
	report.Counts.Warnings = len(report.Warnings)
	if len(report.Warnings) > 0 {
		report.Confidence = "review_required"
	}
}

func projectIssueItem(repo string, issue DashboardRawIssue, now time.Time) ProjectReportItem {
	return ProjectReportItem{
		Kind:      "issue",
		Repo:      repo,
		Issue:     issue.IssueNumber,
		Title:     issue.Title,
		Group:     "issue",
		Status:    strings.ToLower(issue.State),
		Priority:  projectPriority(issue.Labels),
		Milestone: issue.Milestone,
		AgeDays:   ageInDays(now, parseTime(issue.UpdatedAt)),
		Evidence:  []string{"github_issue"},
		URL:       issue.URL,
	}
}

func projectPRItem(repo string, pr ReviewPR, now time.Time) ProjectReportItem {
	status := "open"
	if pr.IsDraft {
		status = "draft"
	}
	return ProjectReportItem{
		Kind:     "pr",
		Repo:     repo,
		PR:       pr.Number,
		Title:    pr.Title,
		Group:    "pr",
		Status:   status,
		Priority: projectPriority(pr.Labels),
		AgeDays:  ageInDays(now, parseTime(pr.UpdatedAt)),
		Evidence: []string{"github_pull_request"},
		URL:      pr.URL,
	}
}

func FormatProjectReport(report ProjectReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s items=%d confidence=%s warnings=%d\n", report.Kind, report.Repo, len(report.Items), report.Confidence, len(report.Warnings))
	for _, section := range report.Sections {
		fmt.Fprintf(&b, "%s: %d\n", section.Title, section.Count)
	}
	return b.String()
}

func RenderProjectReportMarkdown(report ProjectReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", report.Title)
	fmt.Fprintf(&b, "- Repo: `%s`\n", report.Repo)
	fmt.Fprintf(&b, "- Generated: `%s`\n", report.GeneratedAt)
	fmt.Fprintf(&b, "- Confidence: `%s`\n", report.Confidence)
	if report.Counts.CompletionPct > 0 {
		fmt.Fprintf(&b, "- Completion: **%d%%**\n", report.Counts.CompletionPct)
	}
	fmt.Fprintf(&b, "- Warnings: **%d**\n\n", len(report.Warnings))
	for _, section := range report.Sections {
		fmt.Fprintf(&b, "## %s\n\n", section.Title)
		for _, item := range report.Items {
			if item.Group != section.Group {
				continue
			}
			ref := projectItemRef(item)
			warnings := ""
			if len(item.Warnings) > 0 {
				warnings = " warnings:" + strings.Join(item.Warnings, ",")
			}
			if item.URL != "" {
				fmt.Fprintf(&b, "- [%s %s](%s) `%s`%s\n", item.Kind, ref, item.URL, item.Status, warnings)
			} else {
				fmt.Fprintf(&b, "- %s %s `%s`%s\n", item.Kind, ref, item.Status, warnings)
			}
		}
		b.WriteString("\n")
	}
	if len(report.Warnings) > 0 {
		b.WriteString("## Risks and Gaps\n\n")
		for _, warning := range report.Warnings {
			fmt.Fprintf(&b, "- `%s`\n", warning)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func RenderProjectReportCSV(report ProjectReport) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write(projectReportCSVHeaders); err != nil {
		return nil, err
	}
	for _, item := range report.Items {
		if err := writer.Write([]string{
			item.Kind,
			item.Repo,
			projectOptionalInt(item.Issue),
			projectOptionalInt(item.PR),
			item.Title,
			item.Group,
			item.Status,
			item.Priority,
			item.Milestone,
			strconv.Itoa(item.AgeDays),
			strings.Join(item.Evidence, ";"),
			strings.Join(item.Warnings, ";"),
			item.URL,
		}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return buf.Bytes(), writer.Error()
}

func RenderProjectReportJSON(report ProjectReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

func RenderProjectReportHTML(report ProjectReport) string {
	var b strings.Builder
	b.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><title>")
	b.WriteString(html.EscapeString(report.Title))
	b.WriteString("</title><style>body{font-family:Inter,Arial,sans-serif;margin:32px;color:#17202a;background:#f7f8fa}main{max-width:1180px;margin:0 auto}h1{font-size:28px;margin:0 0 8px}.meta{color:#5f6b7a}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr));gap:12px;margin:24px 0}.metric{background:#fff;border:1px solid #dde3ea;border-radius:8px;padding:14px}.metric strong{display:block;font-size:24px}table{width:100%;border-collapse:collapse;background:#fff;border:1px solid #dde3ea}th,td{text-align:left;border-bottom:1px solid #edf0f3;padding:9px 10px;font-size:13px}th{background:#eef2f6}.warn{color:#9a3412}.ok{color:#166534}</style></head><body><main>")
	fmt.Fprintf(&b, "<h1>%s</h1><p class=\"meta\">%s · generated %s · confidence %s</p>", html.EscapeString(report.Title), html.EscapeString(report.Repo), html.EscapeString(report.GeneratedAt), html.EscapeString(report.Confidence))
	b.WriteString("<section class=\"grid\">")
	projectHTMLMetric(&b, "items", strconv.Itoa(len(report.Items)))
	projectHTMLMetric(&b, "open issues", strconv.Itoa(report.Counts.OpenIssues))
	projectHTMLMetric(&b, "blocked", strconv.Itoa(report.Counts.Blocked))
	projectHTMLMetric(&b, "warnings", strconv.Itoa(len(report.Warnings)))
	b.WriteString("</section><table><thead><tr><th>kind</th><th>ref</th><th>title</th><th>group</th><th>status</th><th>warnings</th></tr></thead><tbody>")
	for _, item := range report.Items {
		fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td>", html.EscapeString(item.Kind), html.EscapeString(projectItemRef(item)))
		if item.URL != "" {
			fmt.Fprintf(&b, "<a href=\"%s\">%s</a>", html.EscapeString(item.URL), html.EscapeString(item.Title))
		} else {
			b.WriteString(html.EscapeString(item.Title))
		}
		fmt.Fprintf(&b, "</td><td>%s</td><td>%s</td><td class=\"warn\">%s</td></tr>", html.EscapeString(item.Group), html.EscapeString(item.Status), html.EscapeString(strings.Join(item.Warnings, ", ")))
	}
	b.WriteString("</tbody></table></main></body></html>")
	return b.String()
}

func WriteProjectReportBundle(outputRoot string, report ProjectReport) error {
	scope, err := newLocalWriteScope(outputRoot)
	if err != nil {
		return err
	}
	csvBytes, err := RenderProjectReportCSV(report)
	if err != nil {
		return err
	}
	jsonBytes, err := RenderProjectReportJSON(report)
	if err != nil {
		return err
	}
	if err := scope.WriteFile("index.html", []byte(RenderProjectReportHTML(report)), 0o644); err != nil {
		return err
	}
	if err := scope.WriteFile("report.md", []byte(RenderProjectReportMarkdown(report)), 0o644); err != nil {
		return err
	}
	if err := scope.WriteFile("derived/report.json", append(jsonBytes, '\n'), 0o644); err != nil {
		return err
	}
	return scope.WriteFile("csv/report_items.csv", csvBytes, 0o644)
}

func WriteProjectReportHTML(path string, report ProjectReport) error {
	return writeSafeLocalFile(path, []byte(RenderProjectReportHTML(report)), 0o644)
}

func WriteProjectReportMarkdown(path string, report ProjectReport) error {
	return writeSafeLocalFile(path, []byte(RenderProjectReportMarkdown(report)), 0o644)
}

func WriteProjectReportCSV(path string, report ProjectReport) error {
	csvBytes, err := RenderProjectReportCSV(report)
	if err != nil {
		return err
	}
	return writeSafeLocalFile(path, csvBytes, 0o644)
}

func projectReportTitle(kind string, milestone string) string {
	switch kind {
	case "milestone":
		return "Milestone Progress Report: " + milestone
	case "backlog-health":
		return "Backlog Health Report"
	case "delivery-status":
		return "Delivery Status Report"
	case "qa-checklist":
		if strings.TrimSpace(milestone) != "" {
			return "QA Checklist Report: " + milestone
		}
		return "QA Checklist Report"
	default:
		return "Project Report"
	}
}

func projectReportGroupTitle(group string) string {
	words := strings.Split(strings.ReplaceAll(group, "_", " "), " ")
	for i := range words {
		if words[i] == "" {
			continue
		}
		words[i] = strings.ToUpper(words[i][:1]) + words[i][1:]
	}
	return strings.Join(words, " ")
}

func projectPriority(labels []string) string {
	for _, label := range labels {
		lower := strings.ToLower(label)
		if strings.Contains(lower, "p0") || strings.Contains(lower, "critical") {
			return "critical"
		}
		if strings.Contains(lower, "p1") || strings.Contains(lower, "high") {
			return "high"
		}
		if strings.Contains(lower, "p2") || strings.Contains(lower, "medium") {
			return "medium"
		}
	}
	return ""
}

func projectItemRef(item ProjectReportItem) string {
	if item.Issue > 0 {
		return "#" + strconv.Itoa(item.Issue)
	}
	if item.PR > 0 {
		return "#" + strconv.Itoa(item.PR)
	}
	return item.Title
}

func projectOptionalInt(value int) string {
	if value == 0 {
		return ""
	}
	return strconv.Itoa(value)
}

func projectHTMLMetric(b *strings.Builder, label string, value string) {
	fmt.Fprintf(b, "<div class=\"metric\"><span>%s</span><strong>%s</strong></div>", html.EscapeString(label), html.EscapeString(value))
}

func projectHasString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
