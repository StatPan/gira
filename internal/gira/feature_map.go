package gira

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	FeatureMapListSchemaVersion  = "feature-map-list/v1"
	FeatureMapCheckSchemaVersion = "feature-map-check/v1"
	FeatureMapForSchemaVersion   = "feature-map-for/v1"
)

var (
	featureLinkPattern = regexp.MustCompile(`(?im)^\s*(?:related\s+)?(?:capability|feature)\s*:\s*#?(\d+)\b`)
	featureKeyPattern  = regexp.MustCompile(`(?im)^\s*(?:feature\s+key|key)\s*:\s*([A-Za-z0-9][A-Za-z0-9_-]{0,31})\s*$`)
	featureStatusLine  = regexp.MustCompile(`(?im)^\s*(?:capability\s+status|feature\s+status|maturity|status)\s*:\s*([A-Za-z][A-Za-z_-]{0,31})\s*$`)
)

type FeatureMapOptions struct {
	Repo  RepoRef `json:"repo"`
	Limit int     `json:"limit"`
}

type FeatureForOptions struct {
	Repo  RepoRef `json:"repo"`
	Issue int     `json:"issue"`
	Limit int     `json:"limit"`
}

type FeatureMapListReport struct {
	SchemaVersion string               `json:"schema_version"`
	Command       string               `json:"command"`
	Repo          string               `json:"repo"`
	Source        string               `json:"source"`
	Mode          string               `json:"mode"`
	Limit         int                  `json:"limit"`
	Features      []FeatureMapFeature  `json:"features"`
	Counts        FeatureMapListCounts `json:"counts"`
	NextStep      string               `json:"next_step"`
}

type FeatureMapListCounts struct {
	Features int `json:"features"`
}

type FeatureMapCheckReport struct {
	SchemaVersion string                 `json:"schema_version"`
	Command       string                 `json:"command"`
	Repo          string                 `json:"repo"`
	Source        string                 `json:"source"`
	Mode          string                 `json:"mode"`
	Limit         int                    `json:"limit"`
	Features      []FeatureMapFeature    `json:"features"`
	Diagnostics   []FeatureMapDiagnostic `json:"diagnostics"`
	Counts        FeatureMapCheckCounts  `json:"counts"`
	NextStep      string                 `json:"next_step"`
}

type FeatureMapCheckCounts struct {
	Features        int `json:"features"`
	Errors          int `json:"errors"`
	Warnings        int `json:"warnings"`
	LinkedWork      int `json:"linked_work"`
	MissingLinkWork int `json:"missing_link_work"`
}

type FeatureMapForReport struct {
	SchemaVersion string                 `json:"schema_version"`
	Command       string                 `json:"command"`
	Repo          string                 `json:"repo"`
	Issue         FeatureMapWorkIssue    `json:"issue"`
	Feature       *FeatureMapFeature     `json:"feature,omitempty"`
	Diagnostics   []FeatureMapDiagnostic `json:"diagnostics,omitempty"`
	NextStep      string                 `json:"next_step"`
}

type FeatureMapFeature struct {
	Number   int      `json:"number"`
	Title    string   `json:"title"`
	State    string   `json:"state"`
	Key      string   `json:"key,omitempty"`
	Area     string   `json:"area,omitempty"`
	Maturity string   `json:"maturity,omitempty"`
	Labels   []string `json:"labels,omitempty"`
	URL      string   `json:"url,omitempty"`
}

type FeatureMapWorkIssue struct {
	Number        int      `json:"number"`
	Title         string   `json:"title"`
	State         string   `json:"state"`
	Labels        []string `json:"labels,omitempty"`
	URL           string   `json:"url,omitempty"`
	LinkedFeature int      `json:"linked_feature,omitempty"`
}

type FeatureMapDiagnostic struct {
	Severity string `json:"severity"`
	Issue    int    `json:"issue,omitempty"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

type featureMapRawIssue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Body   string `json:"body"`
	URL    string `json:"url"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

func BuildFeatureMapListReport(options FeatureMapOptions, runner CommandRunner) (FeatureMapListReport, error) {
	issues, limit, err := fetchFeatureMapIssues(options, runner)
	if err != nil {
		return FeatureMapListReport{}, err
	}
	features := featureMapFeatures(issues)
	report := FeatureMapListReport{
		SchemaVersion: FeatureMapListSchemaVersion,
		Command:       "feature list",
		Repo:          options.Repo.FullName(),
		Source:        "github_issues",
		Mode:          featureMapMode(features),
		Limit:         limit,
		Features:      features,
		Counts:        FeatureMapListCounts{Features: len(features)},
		NextStep:      featureMapListNextStep(options.Repo, len(features)),
	}
	return report, nil
}

func BuildFeatureMapCheckReport(options FeatureMapOptions, runner CommandRunner) (FeatureMapCheckReport, error) {
	issues, limit, err := fetchFeatureMapIssues(options, runner)
	if err != nil {
		return FeatureMapCheckReport{}, err
	}
	features := featureMapFeatures(issues)
	diagnostics, linkedWork, missingLinkWork := featureMapDiagnostics(issues, features)
	counts := FeatureMapCheckCounts{Features: len(features), LinkedWork: linkedWork, MissingLinkWork: missingLinkWork}
	for _, diagnostic := range diagnostics {
		switch diagnostic.Severity {
		case "error":
			counts.Errors++
		case "warning":
			counts.Warnings++
		}
	}
	report := FeatureMapCheckReport{
		SchemaVersion: FeatureMapCheckSchemaVersion,
		Command:       "feature check",
		Repo:          options.Repo.FullName(),
		Source:        "github_issues",
		Mode:          featureMapMode(features),
		Limit:         limit,
		Features:      features,
		Diagnostics:   diagnostics,
		Counts:        counts,
		NextStep:      featureMapCheckNextStep(options.Repo, counts),
	}
	return report, nil
}

func BuildFeatureMapForReport(options FeatureForOptions, runner CommandRunner) (FeatureMapForReport, error) {
	if options.Issue <= 0 {
		return FeatureMapForReport{}, fmt.Errorf("issue must be > 0")
	}
	issues, _, err := fetchFeatureMapIssues(FeatureMapOptions{Repo: options.Repo, Limit: options.Limit}, runner)
	if err != nil {
		return FeatureMapForReport{}, err
	}
	features := featureMapFeatures(issues)
	featuresByNumber := map[int]FeatureMapFeature{}
	for _, feature := range features {
		featuresByNumber[feature.Number] = feature
	}
	for _, issue := range issues {
		if issue.Number != options.Issue {
			continue
		}
		work := featureMapWorkIssue(issue)
		report := FeatureMapForReport{
			SchemaVersion: FeatureMapForSchemaVersion,
			Command:       "feature for",
			Repo:          options.Repo.FullName(),
			Issue:         work,
		}
		if isFeatureRecord(issue) {
			feature := featureMapFeature(issue)
			report.Feature = &feature
			report.NextStep = fmt.Sprintf("gira feature check --repo %s", options.Repo.FullName())
			return report, nil
		}
		if work.LinkedFeature == 0 {
			report.Diagnostics = append(report.Diagnostics, FeatureMapDiagnostic{Severity: "warning", Issue: issue.Number, Code: "missing_feature_link", Message: "work issue is not linked to a feature or capability"})
			report.NextStep = "add Related capability: #FEATURE to the issue body"
			return report, nil
		}
		feature, ok := featuresByNumber[work.LinkedFeature]
		if !ok {
			report.Diagnostics = append(report.Diagnostics, FeatureMapDiagnostic{Severity: "error", Issue: issue.Number, Code: "linked_feature_not_found", Message: fmt.Sprintf("linked feature #%d was not found", work.LinkedFeature)})
			report.NextStep = fmt.Sprintf("update issue #%d feature link", issue.Number)
			return report, nil
		}
		report.Feature = &feature
		report.NextStep = fmt.Sprintf("gira ticket status %d --repo %s", issue.Number, options.Repo.FullName())
		return report, nil
	}
	return FeatureMapForReport{}, fmt.Errorf("issue #%d not found in %s", options.Issue, options.Repo.FullName())
}

func fetchFeatureMapIssues(options FeatureMapOptions, runner CommandRunner) ([]featureMapRawIssue, int, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	limit := options.Limit
	if limit == 0 {
		limit = 1000
	}
	if limit < 0 {
		return nil, 0, fmt.Errorf("--limit must be greater than 0")
	}
	output, err := runner.Run("gh", "issue", "list", "--repo", options.Repo.FullName(), "--state", "all", "--limit", fmt.Sprintf("%d", limit), "--json", "number,title,state,labels,body,url")
	if err != nil {
		return nil, 0, err
	}
	var rows []featureMapRawIssue
	if err := json.Unmarshal(output, &rows); err != nil {
		return nil, 0, fmt.Errorf("parse gh issue list JSON: %w", err)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Number < rows[j].Number })
	return rows, limit, nil
}

func featureMapFeatures(issues []featureMapRawIssue) []FeatureMapFeature {
	features := make([]FeatureMapFeature, 0)
	for _, issue := range issues {
		if isFeatureRecord(issue) {
			features = append(features, featureMapFeature(issue))
		}
	}
	sort.Slice(features, func(i, j int) bool {
		if features[i].Area != features[j].Area {
			return features[i].Area < features[j].Area
		}
		return features[i].Number < features[j].Number
	})
	return features
}

func featureMapFeature(issue featureMapRawIssue) FeatureMapFeature {
	labels := featureMapLabels(issue)
	return FeatureMapFeature{
		Number:   issue.Number,
		Title:    issue.Title,
		State:    strings.ToLower(issue.State),
		Key:      featureMapKey(issue, labels),
		Area:     featureMapArea(labels),
		Maturity: featureMapMaturity(issue, labels),
		Labels:   featureMapKeyLabels(labels),
		URL:      issue.URL,
	}
}

func featureMapWorkIssue(issue featureMapRawIssue) FeatureMapWorkIssue {
	labels := featureMapLabels(issue)
	return FeatureMapWorkIssue{
		Number:        issue.Number,
		Title:         issue.Title,
		State:         strings.ToLower(issue.State),
		Labels:        featureMapKeyLabels(labels),
		URL:           issue.URL,
		LinkedFeature: extractFeatureLink(issue.Body),
	}
}

func featureMapDiagnostics(issues []featureMapRawIssue, features []FeatureMapFeature) ([]FeatureMapDiagnostic, int, int) {
	featureNumbers := map[int]struct{}{}
	for _, feature := range features {
		featureNumbers[feature.Number] = struct{}{}
	}
	diagnostics := []FeatureMapDiagnostic{}
	linkedWork := 0
	missingLinkWork := 0
	if len(features) == 0 {
		diagnostics = append(diagnostics, FeatureMapDiagnostic{Severity: "info", Code: "feature_map_not_configured", Message: "no issue-backed feature records found"})
		return diagnostics, linkedWork, missingLinkWork
	}
	for _, issue := range issues {
		labels := featureMapLabels(issue)
		if isFeatureRecord(issue) {
			diagnostics = append(diagnostics, featureRecordDiagnostics(issue, labels)...)
			continue
		}
		if strings.EqualFold(issue.State, "closed") {
			continue
		}
		linked := extractFeatureLink(issue.Body)
		if linked == 0 {
			missingLinkWork++
			continue
		}
		linkedWork++
		if _, ok := featureNumbers[linked]; !ok {
			diagnostics = append(diagnostics, FeatureMapDiagnostic{Severity: "error", Issue: issue.Number, Code: "linked_feature_not_found", Message: fmt.Sprintf("linked feature #%d was not found", linked)})
		}
	}
	sort.Slice(diagnostics, func(i, j int) bool {
		if diagnostics[i].Severity != diagnostics[j].Severity {
			return diagnostics[i].Severity < diagnostics[j].Severity
		}
		if diagnostics[i].Issue != diagnostics[j].Issue {
			return diagnostics[i].Issue < diagnostics[j].Issue
		}
		return diagnostics[i].Code < diagnostics[j].Code
	})
	return diagnostics, linkedWork, missingLinkWork
}

func featureRecordDiagnostics(issue featureMapRawIssue, labels []string) []FeatureMapDiagnostic {
	diagnostics := []FeatureMapDiagnostic{}
	if featureMapKey(issue, labels) == "" {
		diagnostics = append(diagnostics, FeatureMapDiagnostic{Severity: "warning", Issue: issue.Number, Code: "missing_key", Message: "feature record should define a short key for daily UX"})
	}
	maturities := featureMapMaturityValues(issue, labels)
	if len(maturities) == 0 {
		diagnostics = append(diagnostics, FeatureMapDiagnostic{Severity: "warning", Issue: issue.Number, Code: "missing_maturity", Message: "feature record should define one maturity value"})
	} else if len(maturities) > 1 {
		diagnostics = append(diagnostics, FeatureMapDiagnostic{Severity: "warning", Issue: issue.Number, Code: "multiple_maturity", Message: "feature record should define only one maturity value"})
	}
	for _, maturity := range maturities {
		if !validFeatureMaturity(maturity) {
			diagnostics = append(diagnostics, FeatureMapDiagnostic{Severity: "error", Issue: issue.Number, Code: "invalid_maturity", Message: fmt.Sprintf("unknown maturity %q", maturity)})
		}
	}
	for _, section := range []string{"User Need", "Capability", "Surface"} {
		if strings.TrimSpace(markdownSection(issue.Body, section)) == "" {
			diagnostics = append(diagnostics, FeatureMapDiagnostic{Severity: "warning", Issue: issue.Number, Code: "missing_section", Message: "feature record is missing ## " + section})
		}
	}
	if featureMapMaturity(issue, labels) == "stable" {
		for _, section := range []string{"Docs", "Evidence"} {
			if strings.TrimSpace(markdownSection(issue.Body, section)) == "" {
				diagnostics = append(diagnostics, FeatureMapDiagnostic{Severity: "warning", Issue: issue.Number, Code: "stable_missing_section", Message: "stable feature is missing ## " + section})
			}
		}
	}
	return diagnostics
}

func isFeatureRecord(issue featureMapRawIssue) bool {
	labels := featureMapLabels(issue)
	for _, label := range labels {
		if label == "type:capability" || label == "type:feature" {
			return true
		}
	}
	title := strings.ToLower(strings.TrimSpace(issue.Title))
	return strings.HasPrefix(title, "capability:") || strings.HasPrefix(title, "feature:")
}

func featureMapLabels(issue featureMapRawIssue) []string {
	labels := make([]string, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		name := strings.TrimSpace(label.Name)
		if name != "" {
			labels = append(labels, name)
		}
	}
	sort.Strings(labels)
	return labels
}

func featureMapKey(issue featureMapRawIssue, labels []string) string {
	for _, label := range labels {
		if key := strings.TrimPrefix(label, "feature-key:"); key != label && key != "" {
			return key
		}
	}
	if match := featureKeyPattern.FindStringSubmatch(issue.Body); len(match) == 2 {
		return strings.ToLower(strings.TrimSpace(match[1]))
	}
	return ""
}

func featureMapArea(labels []string) string {
	for _, label := range labels {
		if strings.HasPrefix(label, "area:") {
			return strings.TrimPrefix(label, "area:")
		}
	}
	return ""
}

func featureMapMaturity(issue featureMapRawIssue, labels []string) string {
	values := featureMapMaturityValues(issue, labels)
	if len(values) == 1 && validFeatureMaturity(values[0]) {
		return values[0]
	}
	return ""
}

func featureMapMaturityValues(issue featureMapRawIssue, labels []string) []string {
	values := []string{}
	for _, label := range labels {
		if strings.HasPrefix(label, "feature-key:") {
			continue
		}
		for _, prefix := range []string{"capability:", "feature:"} {
			if strings.HasPrefix(label, prefix) {
				value := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(label, prefix)))
				if value != "" {
					values = append(values, value)
				}
			}
		}
	}
	if match := featureStatusLine.FindStringSubmatch(issue.Body); len(match) == 2 {
		values = append(values, strings.ToLower(strings.TrimSpace(match[1])))
	}
	return uniqueFeatureMapStrings(values)
}

func featureMapKeyLabels(labels []string) []string {
	keyLabels := []string{}
	for _, label := range labels {
		switch {
		case label == "type:capability",
			label == "type:feature",
			strings.HasPrefix(label, "capability:"),
			strings.HasPrefix(label, "feature:"),
			strings.HasPrefix(label, "area:"):
			keyLabels = append(keyLabels, label)
		}
	}
	return keyLabels
}

func validFeatureMaturity(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "optional", "planned", "preview", "stable", "legacy", "deprecated":
		return true
	default:
		return false
	}
}

func extractFeatureLink(body string) int {
	match := featureLinkPattern.FindStringSubmatch(body)
	if len(match) != 2 {
		return 0
	}
	n, err := strconv.Atoi(match[1])
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func uniqueFeatureMapStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func featureMapMode(features []FeatureMapFeature) string {
	if len(features) == 0 {
		return "none"
	}
	return "optional"
}

func featureMapListNextStep(repo RepoRef, count int) string {
	if count == 0 {
		return "create an issue-backed feature record when this repo needs a feature map"
	}
	return fmt.Sprintf("gira feature check --repo %s", repo.FullName())
}

func featureMapCheckNextStep(repo RepoRef, counts FeatureMapCheckCounts) string {
	if counts.Features == 0 {
		return "feature map is optional; no action required"
	}
	if counts.Errors > 0 || counts.Warnings > 0 {
		return "resolve feature map diagnostics or keep the map optional"
	}
	return fmt.Sprintf("gira feature list --repo %s", repo.FullName())
}

func FormatFeatureMapList(report FeatureMapListReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "feature map: %s source=%s mode=%s count=%d\n", report.Repo, report.Source, report.Mode, report.Counts.Features)
	if len(report.Features) == 0 {
		b.WriteString("features: none\n")
	} else {
		for _, feature := range report.Features {
			meta := []string{}
			if feature.Key != "" {
				meta = append(meta, "key="+feature.Key)
			}
			if feature.Area != "" {
				meta = append(meta, "area="+feature.Area)
			}
			if feature.Maturity != "" {
				meta = append(meta, "maturity="+feature.Maturity)
			}
			suffix := ""
			if len(meta) > 0 {
				suffix = " [" + strings.Join(meta, " ") + "]"
			}
			fmt.Fprintf(&b, "#%d %s%s\n", feature.Number, feature.Title, suffix)
		}
	}
	if report.NextStep != "" {
		fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	}
	return b.String()
}

func FormatFeatureMapCheck(report FeatureMapCheckReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "feature map check: %s source=%s mode=%s features=%d warnings=%d errors=%d\n", report.Repo, report.Source, report.Mode, report.Counts.Features, report.Counts.Warnings, report.Counts.Errors)
	if report.Counts.LinkedWork > 0 || report.Counts.MissingLinkWork > 0 {
		fmt.Fprintf(&b, "work links: linked=%d missing=%d\n", report.Counts.LinkedWork, report.Counts.MissingLinkWork)
	}
	if len(report.Diagnostics) == 0 {
		b.WriteString("diagnostics: none\n")
	} else {
		b.WriteString("diagnostics:\n")
		for _, diagnostic := range report.Diagnostics {
			target := ""
			if diagnostic.Issue > 0 {
				target = fmt.Sprintf(" #%d", diagnostic.Issue)
			}
			fmt.Fprintf(&b, "  %s%s %s: %s\n", diagnostic.Severity, target, diagnostic.Code, diagnostic.Message)
		}
	}
	if report.NextStep != "" {
		fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	}
	return b.String()
}

func FormatFeatureMapFor(report FeatureMapForReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "feature for: %s issue=#%d %s\n", report.Repo, report.Issue.Number, report.Issue.Title)
	if report.Feature == nil {
		b.WriteString("feature: missing\n")
	} else {
		feature := report.Feature
		meta := []string{}
		if feature.Key != "" {
			meta = append(meta, "key="+feature.Key)
		}
		if feature.Maturity != "" {
			meta = append(meta, "maturity="+feature.Maturity)
		}
		if feature.Area != "" {
			meta = append(meta, "area="+feature.Area)
		}
		fmt.Fprintf(&b, "feature: #%d %s", feature.Number, feature.Title)
		if len(meta) > 0 {
			fmt.Fprintf(&b, " [%s]", strings.Join(meta, " "))
		}
		b.WriteString("\n")
	}
	for _, diagnostic := range report.Diagnostics {
		fmt.Fprintf(&b, "%s: %s\n", diagnostic.Severity, diagnostic.Message)
	}
	if report.NextStep != "" {
		fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	}
	return b.String()
}
