package gira

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const DashboardExportSchemaVersion = "v1alpha1"
const dashboardExportGeneratorName = "gira"
const dashboardExportGeneratorMode = "dashboard_export"

var dashboardExecutionCSVHeaders = []string{"id", "kind", "title", "status", "priority", "owner", "milestone", "target_date", "source_refs"}
var dashboardRoadmapCSVHeaders = []string{"id", "title", "start_date", "target_date", "status", "phase", "source_refs"}

var dashboardExportSourceGoogleCalendarReason = "not_enabled"

//go:generate none

type DashboardExportSource struct {
	Name       string  `json:"name"`
	Included   bool    `json:"included"`
	SnapshotAt *string `json:"snapshot_at"`
	Reason     string  `json:"reason,omitempty"`
}

type DashboardExportArtifact struct {
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	WillWrite bool   `json:"will_write"`
}

type DashboardExportCounts struct {
	Issues       int `json:"issues"`
	PullRequests int `json:"pull_requests"`
	Milestones   int `json:"milestones"`
	RoadmapItems int `json:"roadmap_items"`
	Transitions  int `json:"transitions"`
	Warnings     int `json:"warnings"`
}

type DashboardExportPlan struct {
	Command       string                    `json:"command"`
	DryRun        bool                      `json:"dry_run"`
	Repo          string                    `json:"repo"`
	OutputRoot    string                    `json:"output_root"`
	SchemaVersion string                    `json:"schema_version"`
	SnapshotAt    string                    `json:"snapshot_at"`
	Sources       []DashboardExportSource   `json:"sources"`
	Artifacts     []DashboardExportArtifact `json:"artifacts"`
	Counts        DashboardExportCounts     `json:"counts"`
	Warnings      []string                  `json:"warnings"`
}

type DashboardExportGenerator struct {
	Name string `json:"name"`
	Mode string `json:"mode"`
}

type DashboardExportManifest struct {
	SchemaVersion string                    `json:"schema_version"`
	SnapshotAt    string                    `json:"snapshot_at"`
	Repo          string                    `json:"repo"`
	Sources       []DashboardExportSource   `json:"sources"`
	Artifacts     []DashboardExportArtifact `json:"artifacts"`
	Generator     DashboardExportGenerator  `json:"generator"`
}

type DashboardRawIssue struct {
	IssueNumber int      `json:"issue_number"`
	IssueID     string   `json:"issue_id,omitempty"`
	Title       string   `json:"title"`
	State       string   `json:"state"`
	Labels      []string `json:"labels"`
	UpdatedAt   string   `json:"updated_at"`
	Milestone   string   `json:"milestone,omitempty"`
	URL         string   `json:"url"`
}

type DashboardRawPullRequest struct {
	PullRequestNumber int      `json:"pr_number"`
	PullRequestID     string   `json:"pr_id,omitempty"`
	Title             string   `json:"title"`
	State             string   `json:"state"`
	Draft             bool     `json:"draft"`
	Labels            []string `json:"labels"`
	URL               string   `json:"url"`
}

type DashboardRawMilestone struct {
	MilestoneNumber int     `json:"milestone_number"`
	MilestoneID     string  `json:"milestone_id,omitempty"`
	Title           string  `json:"title"`
	State           string  `json:"state"`
	Description     string  `json:"description"`
	DueOn           *string `json:"due_on"`
	OpenIssues      int     `json:"open_issues"`
	ClosedIssues    int     `json:"closed_issues"`
}

type DashboardRawProjectItem struct {
	IssueNumber      int     `json:"issue_number"`
	IssueTitle       string  `json:"issue_title"`
	IssueURL         string  `json:"issue_url"`
	TypeLabel        string  `json:"type_label"`
	Roadmapable      bool    `json:"roadmapable"`
	StartDate        *string `json:"start_date,omitempty"`
	TargetDate       *string `json:"target_date,omitempty"`
	MilestoneDueDate *string `json:"milestone_due_date,omitempty"`
}

type DashboardExportRawGitHub struct {
	Repo         string                    `json:"repo"`
	SnapshotAt   string                    `json:"snapshot_at"`
	Issues       []DashboardRawIssue       `json:"issues"`
	PullRequests []DashboardRawPullRequest `json:"pull_requests"`
	Milestones   []DashboardRawMilestone   `json:"milestones"`
	ProjectItems []DashboardRawProjectItem `json:"project_items"`
}

type DashboardExportRawTransitions struct {
	Repo        string                      `json:"repo"`
	SnapshotAt  string                      `json:"snapshot_at"`
	Transitions []ProjectTransitionPlanItem `json:"transitions"`
	Conflicts   []ProjectTransitionPlanItem `json:"conflicts"`
	Warnings    []string                    `json:"warnings"`
}

type DashboardExportRawCapabilities struct {
	Repo           string                             `json:"repo"`
	SnapshotAt     string                             `json:"snapshot_at"`
	Capabilities   map[string]ProjectCapabilityStatus `json:"capabilities"`
	BlockedActions []ProjectCapabilityBlock           `json:"blocked_actions"`
	Warnings       []string                           `json:"warnings"`
}

type DashboardExecutionBoardItem struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	Title      string   `json:"title"`
	Status     string   `json:"status"`
	Priority   string   `json:"priority"`
	Owner      string   `json:"owner"`
	Milestone  string   `json:"milestone"`
	TargetDate string   `json:"target_date,omitempty"`
	SourceRefs []string `json:"source_refs"`
}

type DashboardExportExecutionBoard struct {
	Repo       string                        `json:"repo"`
	SnapshotAt string                        `json:"snapshot_at"`
	Items      []DashboardExecutionBoardItem `json:"items"`
	Warnings   []string                      `json:"warnings"`
}

type DashboardRoadmapTimelineItem struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	StartDate  string   `json:"start_date,omitempty"`
	TargetDate string   `json:"target_date,omitempty"`
	Status     string   `json:"status"`
	Phase      string   `json:"phase"`
	SourceRefs []string `json:"source_refs"`
}

type DashboardExportRoadmapTimeline struct {
	Repo       string                         `json:"repo"`
	SnapshotAt string                         `json:"snapshot_at"`
	Items      []DashboardRoadmapTimelineItem `json:"items"`
	Warnings   []string                       `json:"warnings"`
}

type DashboardExportWarnings struct {
	Repo       string   `json:"repo"`
	SnapshotAt string   `json:"snapshot_at"`
	Warnings   []string `json:"warnings"`
}

type DashboardExportBundle struct {
	Manifest        DashboardExportManifest        `json:"manifest"`
	RawGitHub       DashboardExportRawGitHub       `json:"raw_github"`
	RawTransitions  DashboardExportRawTransitions  `json:"raw_transitions"`
	RawCapabilities DashboardExportRawCapabilities `json:"raw_capabilities"`
	ExecutionBoard  DashboardExportExecutionBoard  `json:"execution_board"`
	RoadmapTimeline DashboardExportRoadmapTimeline `json:"roadmap_timeline"`
	Warnings        DashboardExportWarnings        `json:"warnings"`
}

func DashboardExportArtifacts() []DashboardExportArtifact {
	return []DashboardExportArtifact{
		{Path: "manifest.json", Kind: "manifest_json", WillWrite: true},
		{Path: "raw/github.json", Kind: "raw_json", WillWrite: true},
		{Path: "raw/transitions.json", Kind: "raw_json", WillWrite: true},
		{Path: "raw/capabilities.json", Kind: "raw_json", WillWrite: true},
		{Path: "derived/execution_board.json", Kind: "derived_json", WillWrite: true},
		{Path: "derived/roadmap_timeline.json", Kind: "derived_json", WillWrite: true},
		{Path: "derived/warnings.json", Kind: "derived_json", WillWrite: true},
		{Path: "csv/execution_items.csv", Kind: "csv", WillWrite: true},
		{Path: "csv/roadmap_items.csv", Kind: "csv", WillWrite: true},
	}
}

func BuildDashboardExportPlan(repo RepoRef, outputRoot string, snapshotAt time.Time, dryRun bool, client DashboardExportClient) (DashboardExportPlan, DashboardExportBundle, error) {
	if client == nil {
		return DashboardExportPlan{}, DashboardExportBundle{}, fmt.Errorf("dashboard export client is required")
	}

	issues, err := client.FetchIssues()
	if err != nil {
		return DashboardExportPlan{}, DashboardExportBundle{}, err
	}
	pulls, err := client.FetchPullRequests()
	if err != nil {
		return DashboardExportPlan{}, DashboardExportBundle{}, err
	}
	milestones, err := client.FetchMilestones()
	if err != nil {
		return DashboardExportPlan{}, DashboardExportBundle{}, err
	}
	projectSnapshot, err := client.FetchProjectSnapshot()
	if err != nil {
		return DashboardExportPlan{}, DashboardExportBundle{}, err
	}
	transitionSnapshot, err := client.FetchTransitionSnapshot()
	if err != nil {
		return DashboardExportPlan{}, DashboardExportBundle{}, err
	}
	capabilityReport, err := client.FetchCapabilities()
	if err != nil {
		return DashboardExportPlan{}, DashboardExportBundle{}, err
	}

	transitionReport, err := BuildProjectTransitionsReport(repo.FullName(), transitionSnapshot, snapshotAt)
	if err != nil {
		return DashboardExportPlan{}, DashboardExportBundle{}, err
	}

	snapshotText := formatGitHubTime(snapshotAt)
	artifacts := DashboardExportArtifacts()
	sources := dashboardExportSources(snapshotText)
	projectItems := buildDashboardRawProjectItems(projectSnapshot.RoadmapItems)
	applyTransitions, conflicts := splitTransitionPlans(transitionReport.Transitions)

	warnings := make([]string, 0)
	if len(conflicts) > 0 {
		warnings = append(warnings, "transition conflicts detected")
	}
	for _, blockedAction := range capabilityReport.BlockedActions {
		if strings.TrimSpace(blockedAction.Action) != "" {
			warnings = append(warnings, blockedAction.Action+" is blocked")
		}
	}
	executionBoardItems := buildDashboardExecutionItems(issues, pulls)
	roadmapItems := buildDashboardRoadmapItems(milestones, projectItems)
	roadmapWarnings := normalizeWarnings(capabilityReport.BlockedActions)
	if transitionReport.Counts.Conflicts > 0 {
		roadmapWarnings = append(roadmapWarnings, "transition conflicts detected")
	}
	plan := DashboardExportPlan{
		Command:       "export dashboard",
		DryRun:        dryRun,
		Repo:          repo.FullName(),
		OutputRoot:    outputRoot,
		SchemaVersion: DashboardExportSchemaVersion,
		SnapshotAt:    snapshotText,
		Sources:       sources,
		Artifacts:     artifacts,
		Counts: DashboardExportCounts{
			Issues:       len(issues),
			PullRequests: len(pulls),
			Milestones:   len(milestones),
			RoadmapItems: len(projectItems),
			Transitions:  len(transitionReport.Transitions),
			Warnings:     len(warnings),
		},
		Warnings: warnings,
	}

	bundle := DashboardExportBundle{
		Manifest: DashboardExportManifest{
			SchemaVersion: DashboardExportSchemaVersion,
			SnapshotAt:    snapshotText,
			Repo:          repo.FullName(),
			Sources:       sources,
			Artifacts:     artifacts,
			Generator: DashboardExportGenerator{
				Name: dashboardExportGeneratorName,
				Mode: dashboardExportGeneratorMode,
			},
		},
		RawGitHub: DashboardExportRawGitHub{
			Repo:         repo.FullName(),
			SnapshotAt:   snapshotText,
			Issues:       issues,
			PullRequests: pulls,
			Milestones:   milestones,
			ProjectItems: projectItems,
		},
		RawTransitions: DashboardExportRawTransitions{
			Repo:        repo.FullName(),
			SnapshotAt:  snapshotText,
			Transitions: applyTransitions,
			Conflicts:   conflicts,
			Warnings:    transitionReportWarnings(transitionReport),
		},
		RawCapabilities: DashboardExportRawCapabilities{
			Repo:           repo.FullName(),
			SnapshotAt:     snapshotText,
			Capabilities:   capabilityReport.Capabilities,
			BlockedActions: capabilityReport.BlockedActions,
			Warnings:       roadmapWarnings,
		},
		ExecutionBoard: DashboardExportExecutionBoard{
			Repo:       repo.FullName(),
			SnapshotAt: snapshotText,
			Items:      executionBoardItems,
			Warnings:   warnings,
		},
		RoadmapTimeline: DashboardExportRoadmapTimeline{
			Repo:       repo.FullName(),
			SnapshotAt: snapshotText,
			Items:      roadmapItems,
			Warnings:   roadmapWarnings,
		},
		Warnings: DashboardExportWarnings{
			Repo:       repo.FullName(),
			SnapshotAt: snapshotText,
			Warnings:   warnings,
		},
	}

	return plan, bundle, nil
}

func dashboardExportSources(snapshotAt string) []DashboardExportSource {
	github := snapshotAt
	return []DashboardExportSource{
		{Name: "github", Included: true, SnapshotAt: &github},
		{Name: "google_calendar", Included: false, SnapshotAt: nil, Reason: dashboardExportSourceGoogleCalendarReason},
	}
}

func buildDashboardExecutionItems(issues []DashboardRawIssue, pulls []DashboardRawPullRequest) []DashboardExecutionBoardItem {
	items := make([]DashboardExecutionBoardItem, 0, len(issues)+len(pulls))
	for _, issue := range issues {
		items = append(items, DashboardExecutionBoardItem{
			ID:         "issue:" + strconv.Itoa(issue.IssueNumber),
			Kind:       "issue",
			Title:      issue.Title,
			Status:     dashboardExportStatusFromLabels(issue.Labels),
			Priority:   dashboardExportPriorityFromLabels(issue.Labels),
			Owner:      dashboardExportOwnerFromLabels(issue.Labels),
			Milestone:  issue.Milestone,
			SourceRefs: []string{"issue:" + strconv.Itoa(issue.IssueNumber)},
		})
	}
	for _, pull := range pulls {
		items = append(items, DashboardExecutionBoardItem{
			ID:         "pr:" + strconv.Itoa(pull.PullRequestNumber),
			Kind:       "pull_request",
			Title:      pull.Title,
			Status:     dashboardExportPullStatusFromState(pull.State),
			Priority:   dashboardExportPriorityFromLabels(pull.Labels),
			Owner:      dashboardExportOwnerFromLabels(pull.Labels),
			SourceRefs: []string{"pr:" + strconv.Itoa(pull.PullRequestNumber)},
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind == items[j].Kind {
			if items[i].Kind == "issue" {
				left := dashboardExportItemNumber(items[i].ID)
				right := dashboardExportItemNumber(items[j].ID)
				if left != right {
					return left < right
				}
			} else if items[i].Kind == "pull_request" {
				left := dashboardExportItemNumber(items[i].ID)
				right := dashboardExportItemNumber(items[j].ID)
				if left != right {
					return left < right
				}
			}
			return items[i].Title < items[j].Title
		}
		return items[i].Kind < items[j].Kind
	})
	return items
}

func buildDashboardRoadmapItems(milestones []DashboardRawMilestone, projectItems []DashboardRawProjectItem) []DashboardRoadmapTimelineItem {
	items := make([]DashboardRoadmapTimelineItem, 0, len(milestones)+len(projectItems))
	for _, milestone := range milestones {
		item := DashboardRoadmapTimelineItem{
			ID:         "milestone:" + strconv.Itoa(milestone.MilestoneNumber),
			Title:      milestone.Title,
			Status:     milestone.State,
			Phase:      "milestone",
			SourceRefs: []string{"milestone:" + strconv.Itoa(milestone.MilestoneNumber)},
		}
		if milestone.DueOn != nil {
			if normalized, ok := normalizeDate(*milestone.DueOn); ok {
				item.TargetDate = normalized
			}
		}
		items = append(items, item)
	}
	for _, projectItem := range projectItems {
		status := "roadmapable"
		if !projectItem.Roadmapable {
			status = "not_roadmapable"
		}
		item := DashboardRoadmapTimelineItem{
			ID:         "issue:" + strconv.Itoa(projectItem.IssueNumber),
			Title:      projectItem.IssueTitle,
			Status:     status,
			Phase:      projectItem.TypeLabel,
			SourceRefs: []string{"issue:" + strconv.Itoa(projectItem.IssueNumber)},
		}
		if projectItem.StartDate != nil {
			item.StartDate = *projectItem.StartDate
		}
		if projectItem.TargetDate != nil {
			item.TargetDate = *projectItem.TargetDate
		}
		if item.TargetDate == "" && projectItem.MilestoneDueDate != nil {
			item.TargetDate = *projectItem.MilestoneDueDate
		}
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		leftDate := dashboardExportSortDate(items[i].TargetDate)
		rightDate := dashboardExportSortDate(items[j].TargetDate)
		if leftDate != rightDate {
			return leftDate < rightDate
		}
		if items[i].ID != items[j].ID {
			return items[i].ID < items[j].ID
		}
		return items[i].Title < items[j].Title
	})
	return items
}

func buildDashboardRawProjectItems(roadmapItems []ProjectRoadmapItem) []DashboardRawProjectItem {
	raw := make([]DashboardRawProjectItem, 0, len(roadmapItems))
	for _, item := range roadmapItems {
		raw = append(raw, DashboardRawProjectItem{
			IssueNumber:      item.IssueNumber,
			IssueTitle:       item.IssueTitle,
			IssueURL:         item.IssueURL,
			TypeLabel:        item.TypeLabel,
			Roadmapable:      item.Roadmapable,
			StartDate:        item.StartDate,
			TargetDate:       item.TargetDate,
			MilestoneDueDate: item.MilestoneDueDate,
		})
	}
	sort.Slice(raw, func(i, j int) bool {
		if raw[i].IssueNumber == raw[j].IssueNumber {
			return raw[i].IssueTitle < raw[j].IssueTitle
		}
		return raw[i].IssueNumber < raw[j].IssueNumber
	})
	return raw
}

func splitTransitionPlans(transitions []ProjectTransitionPlanItem) ([]ProjectTransitionPlanItem, []ProjectTransitionPlanItem) {
	applies := make([]ProjectTransitionPlanItem, 0)
	conflicts := make([]ProjectTransitionPlanItem, 0)
	for _, transition := range transitions {
		if transition.Decision == "apply" {
			applies = append(applies, transition)
			continue
		}
		if transition.ConflictResolution != "" {
			conflicts = append(conflicts, transition)
		}
	}
	return applies, conflicts
}

func transitionReportWarnings(report ProjectTransitionsReport) []string {
	warnings := make([]string, 0)
	if report.Counts.Conflicts > 0 {
		warnings = append(warnings, "transition conflicts detected")
	}
	return warnings
}

func normalizeWarnings(blockedActions []ProjectCapabilityBlock) []string {
	warnings := make([]string, 0, len(blockedActions))
	for _, block := range blockedActions {
		warning := block.Action
		if warning == "" {
			continue
		}
		if block.Reason != "" {
			warning += ": " + block.Reason
		}
		warnings = append(warnings, warning)
	}
	sort.Strings(warnings)
	return warnings
}

func dashboardExportItemNumber(id string) int {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) != 2 {
		return 0
	}
	value, _ := strconv.Atoi(parts[1])
	return value
}

func dashboardExportSortDate(value string) string {
	if strings.TrimSpace(value) == "" {
		return "9999-12-31"
	}
	if parsed, err := time.Parse(time.DateOnly, value); err == nil {
		return parsed.Format(time.DateOnly)
	}
	return value
}

func dashboardExportStatusFromLabels(labels []string) string {
	for _, label := range labels {
		if strings.HasPrefix(label, "status:") {
			return strings.TrimPrefix(label, "status:")
		}
	}
	return "unknown"
}

func dashboardExportPriorityFromLabels(labels []string) string {
	for _, label := range labels {
		if strings.HasPrefix(label, "priority:") {
			return strings.TrimPrefix(label, "priority:")
		}
	}
	return ""
}

func dashboardExportOwnerFromLabels(labels []string) string {
	for _, label := range labels {
		if strings.HasPrefix(label, "agent:") {
			return strings.TrimPrefix(label, "agent:")
		}
	}
	return ""
}

func dashboardExportPullStatusFromState(state string) string {
	switch strings.ToLower(state) {
	case "open":
		return "open"
	case "closed":
		return "closed"
	default:
		return strings.ToLower(strings.TrimSpace(state))
	}
}

func FormatDashboardExportPlan(plan DashboardExportPlan) string {
	var lines []string
	lines = append(lines, "export dashboard plan:")
	lines = append(lines, "repo: "+plan.Repo)
	lines = append(lines, "output_root: "+plan.OutputRoot)
	lines = append(lines, "schema_version: "+plan.SchemaVersion)
	lines = append(lines, "snapshot_at: "+plan.SnapshotAt)
	lines = append(lines, "sources:")
	for _, source := range plan.Sources {
		snapshot := "null"
		if source.SnapshotAt != nil {
			snapshot = *source.SnapshotAt
		}
		lines = append(lines, "  name: "+source.Name)
		lines = append(lines, "    included: "+strconv.FormatBool(source.Included))
		lines = append(lines, "    snapshot_at: "+snapshot)
		if source.Reason != "" {
			lines = append(lines, "    reason: "+source.Reason)
		}
	}
	lines = append(lines, "artifacts:")
	for _, artifact := range plan.Artifacts {
		lines = append(lines, fmt.Sprintf("  - %s (%s) will_write=%t", artifact.Path, artifact.Kind, artifact.WillWrite))
	}
	lines = append(lines, fmt.Sprintf("counts:"))
	lines = append(lines, fmt.Sprintf("  issues: %d", plan.Counts.Issues))
	lines = append(lines, fmt.Sprintf("  pull_requests: %d", plan.Counts.PullRequests))
	lines = append(lines, fmt.Sprintf("  milestones: %d", plan.Counts.Milestones))
	lines = append(lines, fmt.Sprintf("  roadmap_items: %d", plan.Counts.RoadmapItems))
	lines = append(lines, fmt.Sprintf("  transitions: %d", plan.Counts.Transitions))
	lines = append(lines, fmt.Sprintf("  warnings: %d", plan.Counts.Warnings))
	lines = append(lines, "warnings:")
	if len(plan.Warnings) == 0 {
		lines = append(lines, "  none")
	} else {
		for _, warning := range plan.Warnings {
			lines = append(lines, "  - "+warning)
		}
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func WriteDashboardExportBundle(outputRoot string, bundle DashboardExportBundle) error {
	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(outputRoot, "raw"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(outputRoot, "derived"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(outputRoot, "csv"), 0o755); err != nil {
		return err
	}

	executionCSV, err := renderDashboardExecutionCSV(bundle.ExecutionBoard.Items)
	if err != nil {
		return err
	}
	roadmapCSV, err := renderDashboardRoadmapCSV(bundle.RoadmapTimeline.Items)
	if err != nil {
		return err
	}

	if err := writeDashboardExportJSON(filepath.Join(outputRoot, "manifest.json"), bundle.Manifest); err != nil {
		return err
	}
	if err := writeDashboardExportJSON(filepath.Join(outputRoot, "raw", "github.json"), bundle.RawGitHub); err != nil {
		return err
	}
	if err := writeDashboardExportJSON(filepath.Join(outputRoot, "raw", "transitions.json"), bundle.RawTransitions); err != nil {
		return err
	}
	if err := writeDashboardExportJSON(filepath.Join(outputRoot, "raw", "capabilities.json"), bundle.RawCapabilities); err != nil {
		return err
	}
	if err := writeDashboardExportJSON(filepath.Join(outputRoot, "derived", "execution_board.json"), bundle.ExecutionBoard); err != nil {
		return err
	}
	if err := writeDashboardExportJSON(filepath.Join(outputRoot, "derived", "roadmap_timeline.json"), bundle.RoadmapTimeline); err != nil {
		return err
	}
	if err := writeDashboardExportJSON(filepath.Join(outputRoot, "derived", "warnings.json"), bundle.Warnings); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outputRoot, "csv", "execution_items.csv"), executionCSV, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outputRoot, "csv", "roadmap_items.csv"), roadmapCSV, 0o644); err != nil {
		return err
	}
	return nil
}

func writeDashboardExportJSON(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return os.WriteFile(path, encoded, 0o644)
}

func renderDashboardExecutionCSV(items []DashboardExecutionBoardItem) ([]byte, error) {
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	if err := writer.Write(dashboardExecutionCSVHeaders); err != nil {
		return nil, err
	}
	for _, item := range items {
		row := []string{
			item.ID,
			item.Kind,
			item.Title,
			item.Status,
			item.Priority,
			item.Owner,
			item.Milestone,
			item.TargetDate,
			strings.Join(item.SourceRefs, ","),
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return buffer.Bytes(), writer.Error()
}

func renderDashboardRoadmapCSV(items []DashboardRoadmapTimelineItem) ([]byte, error) {
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	if err := writer.Write(dashboardRoadmapCSVHeaders); err != nil {
		return nil, err
	}
	for _, item := range items {
		row := []string{
			item.ID,
			item.Title,
			item.StartDate,
			item.TargetDate,
			item.Status,
			item.Phase,
			strings.Join(item.SourceRefs, ","),
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return buffer.Bytes(), writer.Error()
}
