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

const ReleaseNotesReportSchemaVersion = "release-notes-report/v1alpha1"

var releaseNotesCSVHeaders = []string{"kind", "repo", "issue", "pr", "title", "group", "status", "milestone", "evidence_level", "evidence", "warnings", "url"}

type ReleaseNotesClient interface {
	FetchIssues() ([]ReleaseNotesIssue, error)
	FetchMergedPRs() ([]ReleaseNotesPullRequest, error)
	FetchMilestones() ([]DashboardRawMilestone, error)
}

type ReleaseNotesIssue struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	State     string   `json:"state"`
	Labels    []string `json:"labels"`
	Milestone string   `json:"milestone,omitempty"`
	URL       string   `json:"url,omitempty"`
}

type ReleaseNotesPullRequest struct {
	Number   int      `json:"number"`
	Title    string   `json:"title"`
	Body     string   `json:"body,omitempty"`
	URL      string   `json:"url,omitempty"`
	MergedAt string   `json:"merged_at,omitempty"`
	Labels   []string `json:"labels,omitempty"`
}

type ReleaseNotesReport struct {
	Command          string                `json:"command"`
	SchemaVersion    string                `json:"schema_version"`
	Repo             string                `json:"repo"`
	Milestone        string                `json:"milestone"`
	GeneratedAt      string                `json:"generated_at"`
	Confidence       string                `json:"confidence"`
	Counts           ReleaseNotesCounts    `json:"counts"`
	Sections         []ReleaseNotesSection `json:"sections"`
	Items            []ReleaseNotesItem    `json:"items"`
	Warnings         []string              `json:"warnings,omitempty"`
	PublishableDraft string                `json:"publishable_draft"`
	Sources          []ReleaseNotesSource  `json:"sources"`
}

type ReleaseNotesCounts struct {
	Issues       int `json:"issues"`
	PullRequests int `json:"pull_requests"`
	Features     int `json:"features"`
	Fixes        int `json:"fixes"`
	Docs         int `json:"docs"`
	Internal     int `json:"internal"`
	Unknown      int `json:"unknown"`
	Warnings     int `json:"warnings"`
}

type ReleaseNotesSection struct {
	Group string `json:"group"`
	Title string `json:"title"`
	Count int    `json:"count"`
}

type ReleaseNotesItem struct {
	Kind          string   `json:"kind"`
	Repo          string   `json:"repo"`
	Issue         int      `json:"issue"`
	PullRequest   int      `json:"pull_request,omitempty"`
	Title         string   `json:"title"`
	Group         string   `json:"group"`
	Status        string   `json:"status"`
	Milestone     string   `json:"milestone"`
	EvidenceLevel string   `json:"evidence_level"`
	Evidence      []string `json:"evidence"`
	Warnings      []string `json:"warnings,omitempty"`
	URL           string   `json:"url,omitempty"`
	PullURL       string   `json:"pull_url,omitempty"`
}

type ReleaseNotesSource struct {
	Name          string `json:"name"`
	SchemaVersion string `json:"schema_version,omitempty"`
}

type GHReleaseNotesClient struct {
	repo   RepoRef
	runner CommandRunner
}

func NewGHReleaseNotesClient(repo RepoRef, runner CommandRunner) GHReleaseNotesClient {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	return GHReleaseNotesClient{repo: repo, runner: runner}
}

func (c GHReleaseNotesClient) FetchIssues() ([]ReleaseNotesIssue, error) {
	raw, err := c.runner.Run("gh", "api", "repos/"+c.repo.FullName()+"/issues", "--paginate", "--slurp", "-X", "GET", "-f", "state=all", "-f", "per_page=100")
	if err != nil {
		return nil, err
	}
	var pages json.RawMessage
	if err := json.Unmarshal(raw, &pages); err != nil {
		return nil, fmt.Errorf("parse issue pages: %w", err)
	}
	rows, err := flattenPages(pages)
	if err != nil {
		return nil, err
	}
	issues := make([]ReleaseNotesIssue, 0, len(rows))
	for _, row := range rows {
		var issue struct {
			Number      int    `json:"number"`
			Title       string `json:"title"`
			State       string `json:"state"`
			HTMLURL     string `json:"html_url"`
			URL         string `json:"url"`
			PullRequest *struct {
			} `json:"pull_request"`
			Milestone *struct {
				Title string `json:"title"`
			} `json:"milestone"`
			Labels []struct {
				Name string `json:"name"`
			} `json:"labels"`
		}
		if err := json.Unmarshal(row, &issue); err != nil {
			return nil, fmt.Errorf("parse issue row: %w", err)
		}
		if issue.PullRequest != nil {
			continue
		}
		labels := make([]string, 0, len(issue.Labels))
		for _, label := range issue.Labels {
			if strings.TrimSpace(label.Name) != "" {
				labels = append(labels, label.Name)
			}
		}
		sort.Strings(labels)
		milestone := ""
		if issue.Milestone != nil {
			milestone = issue.Milestone.Title
		}
		url := issue.HTMLURL
		if url == "" {
			url = issue.URL
		}
		issues = append(issues, ReleaseNotesIssue{Number: issue.Number, Title: issue.Title, State: strings.ToLower(issue.State), Labels: labels, Milestone: milestone, URL: url})
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].Number < issues[j].Number })
	return issues, nil
}

func (c GHReleaseNotesClient) FetchMergedPRs() ([]ReleaseNotesPullRequest, error) {
	raw, err := c.runner.Run("gh", "api", "repos/"+c.repo.FullName()+"/pulls", "--paginate", "--slurp", "-X", "GET", "-f", "state=closed", "-f", "per_page=100")
	if err != nil {
		return nil, err
	}
	var pages json.RawMessage
	if err := json.Unmarshal(raw, &pages); err != nil {
		return nil, fmt.Errorf("parse pull pages: %w", err)
	}
	pageRows, err := flattenPages(pages)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Number   int    `json:"number"`
		Title    string `json:"title"`
		Body     string `json:"body"`
		URL      string `json:"url"`
		HTMLURL  string `json:"html_url"`
		MergedAt string `json:"merged_at"`
		Labels   []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	for _, row := range pageRows {
		var parsed struct {
			Number   int    `json:"number"`
			Title    string `json:"title"`
			Body     string `json:"body"`
			URL      string `json:"url"`
			HTMLURL  string `json:"html_url"`
			MergedAt string `json:"merged_at"`
			Labels   []struct {
				Name string `json:"name"`
			} `json:"labels"`
		}
		if err := json.Unmarshal(row, &parsed); err != nil {
			return nil, fmt.Errorf("parse pull row: %w", err)
		}
		rows = append(rows, parsed)
	}
	prs := make([]ReleaseNotesPullRequest, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.MergedAt) == "" {
			continue
		}
		labels := make([]string, 0, len(row.Labels))
		for _, label := range row.Labels {
			if strings.TrimSpace(label.Name) != "" {
				labels = append(labels, label.Name)
			}
		}
		sort.Strings(labels)
		url := row.HTMLURL
		if url == "" {
			url = row.URL
		}
		prs = append(prs, ReleaseNotesPullRequest{Number: row.Number, Title: row.Title, Body: row.Body, URL: url, MergedAt: row.MergedAt, Labels: labels})
	}
	sort.Slice(prs, func(i, j int) bool { return prs[i].Number < prs[j].Number })
	return prs, nil
}

func (c GHReleaseNotesClient) FetchMilestones() ([]DashboardRawMilestone, error) {
	client := NewGHDashboardExportClient(c.repo, c.runner)
	return client.FetchMilestones()
}

func BuildReleaseNotesReport(repo RepoRef, milestone string, client ReleaseNotesClient, generatedAt time.Time) (ReleaseNotesReport, error) {
	milestone = strings.TrimSpace(milestone)
	if milestone == "" {
		return ReleaseNotesReport{}, fmt.Errorf("--milestone is required")
	}
	if client == nil {
		return ReleaseNotesReport{}, fmt.Errorf("release notes client is required")
	}
	issues, err := client.FetchIssues()
	if err != nil {
		return ReleaseNotesReport{}, err
	}
	prs, err := client.FetchMergedPRs()
	if err != nil {
		return ReleaseNotesReport{}, err
	}
	milestones, err := client.FetchMilestones()
	if err != nil {
		return ReleaseNotesReport{}, err
	}
	report := ReleaseNotesReport{
		Command:       "report release-notes",
		SchemaVersion: ReleaseNotesReportSchemaVersion,
		Repo:          repo.FullName(),
		Milestone:     milestone,
		GeneratedAt:   generatedAt.UTC().Format(time.RFC3339),
		Confidence:    "ready",
		Sources: []ReleaseNotesSource{
			{Name: "github_issues"},
			{Name: "github_pull_requests"},
			{Name: "github_milestones"},
		},
	}
	if !releaseNotesMilestoneExists(milestones, milestone) {
		report.Warnings = append(report.Warnings, "milestone_not_found")
	}
	prsByIssue := releaseNotesPRsByIssue(prs)
	for _, issue := range issues {
		if issue.Milestone != milestone {
			continue
		}
		if strings.EqualFold(issue.State, "open") {
			report.Warnings = appendUniqueStrings(report.Warnings, fmt.Sprintf("open_issue_in_milestone:#%d", issue.Number))
			continue
		}
		item := releaseNotesItemFromIssue(repo, issue, prsByIssue[issue.Number])
		report.Items = append(report.Items, item)
		report.Counts.Issues++
		if item.PullRequest > 0 {
			report.Counts.PullRequests++
		}
		for _, warning := range item.Warnings {
			report.Warnings = appendUniqueStrings(report.Warnings, fmt.Sprintf("issue#%d:%s", issue.Number, warning))
		}
	}
	sort.Slice(report.Items, func(i, j int) bool {
		if report.Items[i].Group != report.Items[j].Group {
			return releaseNotesGroupOrder(report.Items[i].Group) < releaseNotesGroupOrder(report.Items[j].Group)
		}
		return report.Items[i].Issue < report.Items[j].Issue
	})
	report.Counts.Features = releaseNotesCountGroup(report.Items, "features")
	report.Counts.Fixes = releaseNotesCountGroup(report.Items, "fixes")
	report.Counts.Docs = releaseNotesCountGroup(report.Items, "docs")
	report.Counts.Internal = releaseNotesCountGroup(report.Items, "internal")
	report.Counts.Unknown = releaseNotesCountGroup(report.Items, "unknown")
	report.Counts.Warnings = len(report.Warnings)
	report.Sections = releaseNotesSections(report.Items)
	report.PublishableDraft = renderReleaseNotesPublishableDraft(report)
	if len(report.Warnings) > 0 || report.Counts.Unknown > 0 {
		report.Confidence = "review_required"
	}
	return report, nil
}

func releaseNotesItemFromIssue(repo RepoRef, issue ReleaseNotesIssue, pr *ReleaseNotesPullRequest) ReleaseNotesItem {
	group, groupEvidence := releaseNotesGroup(issue.Labels, issue.Title)
	evidence := []string{"github_issue", "milestone"}
	if groupEvidence != "" {
		evidence = append(evidence, groupEvidence)
	}
	item := ReleaseNotesItem{
		Kind:          "issue",
		Repo:          repo.FullName(),
		Issue:         issue.Number,
		Title:         issue.Title,
		Group:         group,
		Status:        "included",
		Milestone:     issue.Milestone,
		EvidenceLevel: "derived",
		Evidence:      evidence,
		URL:           issue.URL,
	}
	if pr != nil {
		item.PullRequest = pr.Number
		item.PullURL = pr.URL
		item.Evidence = append(item.Evidence, "merged_pr", "closing_reference")
	} else {
		item.Warnings = append(item.Warnings, "missing_linked_pr")
	}
	if wbsTypeLabel(issue.Labels) == "" {
		item.Warnings = append(item.Warnings, "missing_type_label")
	}
	if releaseNotesTitleNeedsReview(issue.Title) {
		item.Warnings = append(item.Warnings, "needs_editorial_review")
		item.Status = "needs_editorial_review"
	}
	if len(item.Warnings) == 0 && pr != nil {
		item.EvidenceLevel = "guaranteed"
	}
	return item
}

func releaseNotesPRsByIssue(prs []ReleaseNotesPullRequest) map[int]*ReleaseNotesPullRequest {
	out := map[int]*ReleaseNotesPullRequest{}
	for i := range prs {
		pr := &prs[i]
		for _, issue := range parseClosingIssueNumbers(pr.Body) {
			if _, ok := out[issue]; !ok {
				out[issue] = pr
			}
		}
	}
	return out
}

func releaseNotesMilestoneExists(milestones []DashboardRawMilestone, title string) bool {
	for _, milestone := range milestones {
		if milestone.Title == title {
			return true
		}
	}
	return false
}

func releaseNotesGroup(labels []string, title string) (string, string) {
	lowerTitle := strings.ToLower(title)
	for _, label := range labels {
		lower := strings.ToLower(strings.TrimSpace(label))
		switch {
		case strings.Contains(lower, "breaking"):
			return "breaking", "label:" + label
		case lower == "type:bug" || strings.Contains(lower, "bug"):
			return "fixes", "label:" + label
		case lower == "type:docs" || lower == "area:docs" || strings.Contains(lower, "docs"):
			return "docs", "label:" + label
		case lower == "type:chore" || lower == "type:maintenance" || strings.Contains(lower, "internal"):
			return "internal", "label:" + label
		case lower == "type:epic" || lower == "type:story" || lower == "type:feature" || strings.Contains(lower, "feature"):
			return "features", "label:" + label
		}
	}
	switch {
	case strings.Contains(lowerTitle, "fix") || strings.Contains(lowerTitle, "bug"):
		return "fixes", "title_keyword"
	case strings.Contains(lowerTitle, "doc") || strings.Contains(lowerTitle, "readme"):
		return "docs", "title_keyword"
	case strings.Contains(lowerTitle, "release") || strings.Contains(lowerTitle, "refactor") || strings.Contains(lowerTitle, "test"):
		return "internal", "title_keyword"
	default:
		return "unknown", ""
	}
}

func releaseNotesTitleNeedsReview(title string) bool {
	trimmed := strings.TrimSpace(title)
	lower := strings.ToLower(trimmed)
	return len(trimmed) < 12 || lower == "fix" || lower == "bug" || strings.HasPrefix(lower, "wip")
}

func releaseNotesCountGroup(items []ReleaseNotesItem, group string) int {
	count := 0
	for _, item := range items {
		if item.Group == group {
			count++
		}
	}
	return count
}

func releaseNotesSections(items []ReleaseNotesItem) []ReleaseNotesSection {
	titles := map[string]string{
		"breaking": "Breaking Changes",
		"features": "Features",
		"fixes":    "Fixes",
		"docs":     "Documentation",
		"internal": "Internal",
		"unknown":  "Needs Classification",
	}
	sections := []ReleaseNotesSection{}
	for _, group := range []string{"breaking", "features", "fixes", "docs", "internal", "unknown"} {
		count := releaseNotesCountGroup(items, group)
		if count == 0 {
			continue
		}
		sections = append(sections, ReleaseNotesSection{Group: group, Title: titles[group], Count: count})
	}
	return sections
}

func releaseNotesGroupOrder(group string) int {
	switch group {
	case "breaking":
		return 0
	case "features":
		return 1
	case "fixes":
		return 2
	case "docs":
		return 3
	case "internal":
		return 4
	default:
		return 5
	}
}

func FormatReleaseNotesReport(report ReleaseNotesReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "release notes: %s milestone=%s items=%d confidence=%s warnings=%d\n", report.Repo, report.Milestone, len(report.Items), report.Confidence, len(report.Warnings))
	for _, section := range report.Sections {
		fmt.Fprintf(&b, "%s: %d\n", section.Title, section.Count)
	}
	if len(report.Warnings) > 0 {
		fmt.Fprintf(&b, "warnings: %s\n", strings.Join(report.Warnings, ","))
	}
	return b.String()
}

func AsChangelogReport(report ReleaseNotesReport) ReleaseNotesReport {
	report.Command = "report changelog"
	return report
}

func FormatChangelogReport(report ReleaseNotesReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "changelog: %s milestone=%s items=%d confidence=%s warnings=%d\n", report.Repo, report.Milestone, len(report.Items), report.Confidence, len(report.Warnings))
	for _, section := range report.Sections {
		fmt.Fprintf(&b, "%s: %d\n", section.Title, section.Count)
	}
	if len(report.Warnings) > 0 {
		fmt.Fprintf(&b, "warnings: %s\n", strings.Join(report.Warnings, ","))
	}
	return b.String()
}

func RenderReleaseNotesMarkdown(report ReleaseNotesReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Release Notes: %s\n\n", report.Milestone)
	fmt.Fprintf(&b, "- Repo: `%s`\n", report.Repo)
	fmt.Fprintf(&b, "- Generated: `%s`\n", report.GeneratedAt)
	fmt.Fprintf(&b, "- Confidence: `%s`\n\n", report.Confidence)
	b.WriteString("## Summary\n\n")
	fmt.Fprintf(&b, "This release note draft includes %d closed issue(s) and %d linked merged PR(s) for milestone `%s`.\n\n", report.Counts.Issues, report.Counts.PullRequests, report.Milestone)
	if len(report.Warnings) > 0 {
		b.WriteString("## Risks And Gaps\n\n")
		for _, warning := range report.Warnings {
			fmt.Fprintf(&b, "- `%s`\n", warning)
		}
		b.WriteString("\n")
	}
	b.WriteString("## Changes\n\n")
	for _, section := range report.Sections {
		fmt.Fprintf(&b, "### %s\n\n", section.Title)
		for _, item := range report.Items {
			if item.Group != section.Group {
				continue
			}
			pr := ""
			if item.PullRequest > 0 {
				pr = fmt.Sprintf(" via PR #%d", item.PullRequest)
			}
			fmt.Fprintf(&b, "- %s (#%d%s)\n", item.Title, item.Issue, pr)
		}
		b.WriteString("\n")
	}
	b.WriteString("## Evidence\n\n")
	b.WriteString("| Issue | PR | Group | Evidence | Warnings |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, item := range report.Items {
		issue := fmt.Sprintf("#%d", item.Issue)
		if item.URL != "" {
			issue = fmt.Sprintf("[#%d](%s)", item.Issue, item.URL)
		}
		pr := ""
		if item.PullRequest > 0 {
			pr = fmt.Sprintf("#%d", item.PullRequest)
			if item.PullURL != "" {
				pr = fmt.Sprintf("[#%d](%s)", item.PullRequest, item.PullURL)
			}
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", issue, pr, item.Group, strings.Join(item.Evidence, ", "), strings.Join(item.Warnings, ", "))
	}
	b.WriteString("\n## Publishable Draft\n\n")
	b.WriteString(report.PublishableDraft)
	if !strings.HasSuffix(report.PublishableDraft, "\n") {
		b.WriteString("\n")
	}
	return b.String()
}

func RenderChangelogMarkdown(report ReleaseNotesReport) string {
	text := RenderReleaseNotesMarkdown(AsChangelogReport(report))
	text = strings.Replace(text, "# Release Notes:", "# Changelog:", 1)
	text = strings.Replace(text, "This release note draft includes", "This changelog draft includes", 1)
	return text
}

func renderReleaseNotesPublishableDraft(report ReleaseNotesReport) string {
	var b strings.Builder
	for _, section := range report.Sections {
		if section.Group == "unknown" {
			continue
		}
		fmt.Fprintf(&b, "### %s\n\n", section.Title)
		for _, item := range report.Items {
			if item.Group != section.Group {
				continue
			}
			fmt.Fprintf(&b, "- %s\n", releaseNotesAdvisoryTitle(item.Title))
		}
		b.WriteString("\n")
	}
	if b.Len() == 0 {
		b.WriteString("_No publishable draft items were generated from safe evidence._\n")
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func releaseNotesAdvisoryTitle(title string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(title, "[Task]"), "[Feature]"))
}

func RenderReleaseNotesCSV(report ReleaseNotesReport) ([]byte, error) {
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	if err := writer.Write(releaseNotesCSVHeaders); err != nil {
		return nil, err
	}
	for _, item := range report.Items {
		row := []string{
			item.Kind,
			item.Repo,
			strconv.Itoa(item.Issue),
			releaseNotesPRNumber(item.PullRequest),
			item.Title,
			item.Group,
			item.Status,
			item.Milestone,
			item.EvidenceLevel,
			strings.Join(item.Evidence, ","),
			strings.Join(item.Warnings, ","),
			item.URL,
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return buffer.Bytes(), writer.Error()
}

func RenderReleaseNotesJSON(report ReleaseNotesReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

func RenderReleaseNotesHTML(report ReleaseNotesReport) string {
	var b strings.Builder
	title := "Release Notes: " + report.Milestone
	b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n")
	b.WriteString("<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	fmt.Fprintf(&b, "<title>%s</title>\n", html.EscapeString(title))
	b.WriteString(`<style>
:root { color-scheme: light; --bg:#f6f7f9; --panel:#fff; --text:#20242a; --muted:#606b78; --line:#d8dde4; --accent:#1769aa; --warn:#935f00; --soft:#f2f4f7; }
* { box-sizing: border-box; }
body { margin:0; background:var(--bg); color:var(--text); font:14px/1.5 -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif; }
main { max-width:1080px; margin:0 auto; padding:28px 18px 42px; }
header, section { background:var(--panel); border:1px solid var(--line); border-radius:8px; margin:0 0 14px; padding:18px; }
h1,h2,h3,p { margin:0; }
h1 { font-size:24px; line-height:1.2; }
h2 { font-size:16px; margin-bottom:10px; }
h3 { font-size:14px; margin:16px 0 6px; }
a { color:var(--accent); text-decoration:none; }
a:hover { text-decoration:underline; }
.eyebrow,.muted { color:var(--muted); }
.eyebrow { font-size:12px; text-transform:uppercase; margin-bottom:4px; }
.summary { display:grid; grid-template-columns:repeat(auto-fit,minmax(150px,1fr)); gap:10px; margin-top:14px; }
.metric { border:1px solid var(--line); border-radius:8px; padding:10px; background:#fbfcfd; }
.metric strong { display:block; font-size:20px; line-height:1.2; overflow-wrap:anywhere; }
.metric span { color:var(--muted); }
table { width:100%; border-collapse:collapse; }
th,td { border-top:1px solid var(--line); padding:7px 6px; text-align:left; vertical-align:top; }
th { color:var(--muted); font-weight:600; }
.warn { color:var(--warn); }
@media print { body { background:#fff; } main { max-width:none; padding:0; } header,section { break-inside:avoid; border-color:#ccc; } }
</style>
`)
	b.WriteString("</head>\n<body>\n<main>\n<header>\n")
	b.WriteString("<p class=\"eyebrow\">Gira release notes</p>\n")
	fmt.Fprintf(&b, "<h1>%s</h1>\n", html.EscapeString(report.Milestone))
	b.WriteString("<div class=\"summary\">\n")
	releaseNotesHTMLMetric(&b, "items", strconv.Itoa(len(report.Items)))
	releaseNotesHTMLMetric(&b, "linked PRs", strconv.Itoa(report.Counts.PullRequests))
	releaseNotesHTMLMetric(&b, "confidence", report.Confidence)
	releaseNotesHTMLMetric(&b, "warnings", strconv.Itoa(len(report.Warnings)))
	b.WriteString("</div>\n")
	fmt.Fprintf(&b, "<p class=\"muted\">%s · generated %s</p>\n", html.EscapeString(report.Repo), html.EscapeString(report.GeneratedAt))
	b.WriteString("</header>\n")
	if len(report.Warnings) > 0 {
		b.WriteString("<section>\n<h2>Risks And Gaps</h2>\n<ul>\n")
		for _, warning := range report.Warnings {
			fmt.Fprintf(&b, "<li class=\"warn\">%s</li>\n", html.EscapeString(warning))
		}
		b.WriteString("</ul>\n</section>\n")
	}
	b.WriteString("<section>\n<h2>Changes</h2>\n")
	for _, section := range report.Sections {
		fmt.Fprintf(&b, "<h3>%s</h3>\n<ul>\n", html.EscapeString(section.Title))
		for _, item := range report.Items {
			if item.Group != section.Group {
				continue
			}
			titleHTML := html.EscapeString(item.Title)
			if item.URL != "" {
				titleHTML = fmt.Sprintf("<a href=\"%s\">%s</a>", html.EscapeString(item.URL), titleHTML)
			}
			pr := ""
			if item.PullRequest > 0 {
				pr = fmt.Sprintf(" via PR #%d", item.PullRequest)
			}
			fmt.Fprintf(&b, "<li>%s <span class=\"muted\">(#%d%s)</span></li>\n", titleHTML, item.Issue, html.EscapeString(pr))
		}
		b.WriteString("</ul>\n")
	}
	b.WriteString("</section>\n<section>\n<h2>Evidence</h2>\n<table>\n<thead><tr><th>Issue</th><th>PR</th><th>Group</th><th>Evidence</th><th>Warnings</th></tr></thead>\n<tbody>\n")
	for _, item := range report.Items {
		issue := fmt.Sprintf("#%d", item.Issue)
		if item.URL != "" {
			issue = fmt.Sprintf("<a href=\"%s\">#%d</a>", html.EscapeString(item.URL), item.Issue)
		}
		pr := ""
		if item.PullRequest > 0 {
			pr = fmt.Sprintf("#%d", item.PullRequest)
			if item.PullURL != "" {
				pr = fmt.Sprintf("<a href=\"%s\">#%d</a>", html.EscapeString(item.PullURL), item.PullRequest)
			}
		}
		fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n", issue, pr, html.EscapeString(item.Group), html.EscapeString(strings.Join(item.Evidence, ", ")), html.EscapeString(strings.Join(item.Warnings, ", ")))
	}
	b.WriteString("</tbody>\n</table>\n</section>\n</main>\n</body>\n</html>\n")
	return b.String()
}

func RenderChangelogHTML(report ReleaseNotesReport) string {
	text := RenderReleaseNotesHTML(AsChangelogReport(report))
	text = strings.ReplaceAll(text, "Release Notes:", "Changelog:")
	text = strings.ReplaceAll(text, "Gira release notes", "Gira changelog")
	return text
}

func WriteReleaseNotesBundle(outputRoot string, report ReleaseNotesReport) error {
	scope, err := newLocalWriteScope(outputRoot)
	if err != nil {
		return err
	}
	csvBytes, err := RenderReleaseNotesCSV(report)
	if err != nil {
		return err
	}
	jsonBytes, err := RenderReleaseNotesJSON(report)
	if err != nil {
		return err
	}
	if err := scope.WriteFile("index.html", []byte(RenderReleaseNotesHTML(report)), 0o644); err != nil {
		return err
	}
	if err := scope.WriteFile("release-notes.md", []byte(RenderReleaseNotesMarkdown(report)), 0o644); err != nil {
		return err
	}
	if err := scope.WriteFile("derived/release_notes.json", append(jsonBytes, '\n'), 0o644); err != nil {
		return err
	}
	if err := scope.WriteFile("csv/release_items.csv", csvBytes, 0o644); err != nil {
		return err
	}
	return nil
}

func WriteChangelogBundle(outputRoot string, report ReleaseNotesReport) error {
	report = AsChangelogReport(report)
	scope, err := newLocalWriteScope(outputRoot)
	if err != nil {
		return err
	}
	csvBytes, err := RenderReleaseNotesCSV(report)
	if err != nil {
		return err
	}
	jsonBytes, err := RenderReleaseNotesJSON(report)
	if err != nil {
		return err
	}
	if err := scope.WriteFile("index.html", []byte(RenderChangelogHTML(report)), 0o644); err != nil {
		return err
	}
	if err := scope.WriteFile("changelog.md", []byte(RenderChangelogMarkdown(report)), 0o644); err != nil {
		return err
	}
	if err := scope.WriteFile("derived/changelog.json", append(jsonBytes, '\n'), 0o644); err != nil {
		return err
	}
	return scope.WriteFile("csv/changelog_items.csv", csvBytes, 0o644)
}

func WriteReleaseNotesHTML(path string, report ReleaseNotesReport) error {
	return writeSafeLocalFile(path, []byte(RenderReleaseNotesHTML(report)), 0o644)
}

func WriteChangelogHTML(path string, report ReleaseNotesReport) error {
	return writeSafeLocalFile(path, []byte(RenderChangelogHTML(report)), 0o644)
}

func WriteReleaseNotesMarkdown(path string, report ReleaseNotesReport) error {
	return writeSafeLocalFile(path, []byte(RenderReleaseNotesMarkdown(report)), 0o644)
}

func WriteChangelogMarkdown(path string, report ReleaseNotesReport) error {
	return writeSafeLocalFile(path, []byte(RenderChangelogMarkdown(report)), 0o644)
}

func WriteReleaseNotesCSV(path string, report ReleaseNotesReport) error {
	csvBytes, err := RenderReleaseNotesCSV(report)
	if err != nil {
		return err
	}
	return writeSafeLocalFile(path, csvBytes, 0o644)
}

func releaseNotesPRNumber(number int) string {
	if number <= 0 {
		return ""
	}
	return strconv.Itoa(number)
}

func releaseNotesHTMLMetric(b *strings.Builder, label string, value string) {
	fmt.Fprintf(b, "<div class=\"metric\"><strong>%s</strong><span>%s</span></div>\n", html.EscapeString(value), html.EscapeString(label))
}
