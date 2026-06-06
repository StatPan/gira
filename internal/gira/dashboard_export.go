package gira

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strconv"
	"strings"
	"time"
)

const DashboardExportSchemaVersion = "v1alpha1"
const WorkspaceDashboardSchemaVersion = "workspace-dashboard/v1alpha1"
const WorkspaceStatusSourceContract = "workspace-status/v1"
const dashboardExportGeneratorName = "gira"
const dashboardExportGeneratorMode = "dashboard_export"
const workspaceDashboardTopActionLimit = 10
const workspaceDashboardManifestPath = "manifest.json"
const workspaceDashboardRawStatusPath = "raw/workspace_status.json"
const workspaceDashboardQueuesPath = "derived/workspace_queues.json"
const workspaceDashboardIndexPath = "derived/workspace_dashboard.json"
const workspaceDashboardQueueCSVPath = "csv/workspace_queue_items.csv"
const workspaceDashboardHTMLPath = "index.html"

var dashboardExecutionCSVHeaders = []string{"id", "kind", "title", "status", "priority", "owner", "milestone", "target_date", "source_refs"}
var dashboardRoadmapCSVHeaders = []string{"id", "title", "start_date", "target_date", "status", "phase", "source_refs"}
var dashboardWorkspaceQueueCSVHeaders = []string{"queue", "repo", "issue", "title", "state", "status", "pr_number", "pr_state", "reason_codes", "next_safe_command", "url"}

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
	Issues              int `json:"issues"`
	PullRequests        int `json:"pull_requests"`
	Milestones          int `json:"milestones"`
	RoadmapItems        int `json:"roadmap_items"`
	Transitions         int `json:"transitions"`
	WorkspaceRepos      int `json:"workspace_repos,omitempty"`
	WorkspaceQueueItems int `json:"workspace_queue_items,omitempty"`
	Warnings            int `json:"warnings"`
}

type DashboardExportPlan struct {
	Command       string                    `json:"command"`
	DryRun        bool                      `json:"dry_run"`
	Repo          string                    `json:"repo,omitempty"`
	Workspace     *WorkspaceSummary         `json:"workspace,omitempty"`
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
	Repo          string                    `json:"repo,omitempty"`
	Workspace     *WorkspaceSummary         `json:"workspace,omitempty"`
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
	Repo       string            `json:"repo"`
	Workspace  *WorkspaceSummary `json:"workspace,omitempty"`
	SnapshotAt string            `json:"snapshot_at"`
	Warnings   []string          `json:"warnings"`
}

type DashboardWorkspaceSource struct {
	Contract string `json:"contract"`
	Path     string `json:"path"`
}

type DashboardWorkspaceTopAction struct {
	Queue           string   `json:"queue"`
	Repo            string   `json:"repo"`
	Issue           int      `json:"issue"`
	Title           string   `json:"title"`
	URL             string   `json:"url"`
	LocalTicketHTML string   `json:"local_ticket_html,omitempty"`
	LocalReviewHTML string   `json:"local_review_html,omitempty"`
	ReasonCodes     []string `json:"reason_codes"`
	NextSafeCommand string   `json:"next_safe_command"`
	SourceRefs      []string `json:"source_refs"`
}

type DashboardWorkspaceWarning struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type DashboardWorkspaceArtifacts struct {
	Manifest           string   `json:"manifest"`
	WorkspaceStatus    string   `json:"workspace_status"`
	WorkspaceQueues    string   `json:"workspace_queues"`
	WorkspaceDashboard string   `json:"workspace_dashboard"`
	QueueItemsCSV      string   `json:"queue_items_csv"`
	IndexHTML          string   `json:"index_html"`
	TicketReports      []string `json:"ticket_reports,omitempty"`
	ReviewReports      []string `json:"review_reports,omitempty"`
}

type DashboardWorkspaceDashboard struct {
	SchemaVersion string                        `json:"schema_version"`
	SnapshotAt    string                        `json:"snapshot_at"`
	Workspace     WorkspaceSummary              `json:"workspace"`
	Source        DashboardWorkspaceSource      `json:"source"`
	Counts        WorkspaceCounts               `json:"counts"`
	QueueCounts   WorkspaceQueueCounts          `json:"queue_counts"`
	TopActions    []DashboardWorkspaceTopAction `json:"top_actions"`
	Warnings      []DashboardWorkspaceWarning   `json:"warnings"`
	Artifacts     DashboardWorkspaceArtifacts   `json:"artifacts"`
}

type DashboardExportBundle struct {
	Manifest           DashboardExportManifest        `json:"manifest"`
	RawGitHub          DashboardExportRawGitHub       `json:"raw_github"`
	RawTransitions     DashboardExportRawTransitions  `json:"raw_transitions"`
	RawCapabilities    DashboardExportRawCapabilities `json:"raw_capabilities"`
	ExecutionBoard     DashboardExportExecutionBoard  `json:"execution_board"`
	RoadmapTimeline    DashboardExportRoadmapTimeline `json:"roadmap_timeline"`
	Warnings           DashboardExportWarnings        `json:"warnings"`
	WorkspaceStatus    *WorkspaceReport               `json:"workspace_status,omitempty"`
	WorkspaceQueues    *WorkspaceQueuesReport         `json:"workspace_queues,omitempty"`
	WorkspaceDashboard *DashboardWorkspaceDashboard   `json:"workspace_dashboard,omitempty"`
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

func DashboardExportWorkspaceArtifacts() []DashboardExportArtifact {
	return []DashboardExportArtifact{
		{Path: workspaceDashboardManifestPath, Kind: "manifest_json", WillWrite: true},
		{Path: workspaceDashboardRawStatusPath, Kind: "raw_json", WillWrite: true},
		{Path: workspaceDashboardQueuesPath, Kind: "derived_json", WillWrite: true},
		{Path: workspaceDashboardIndexPath, Kind: "derived_json", WillWrite: true},
		{Path: workspaceDashboardQueueCSVPath, Kind: "csv", WillWrite: true},
		{Path: workspaceDashboardHTMLPath, Kind: "html", WillWrite: true},
	}
}

type dashboardWorkspaceDeepLink struct {
	TicketPath string
	ReviewPath string
	Item       WorkspaceQueueItem
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

func BuildWorkspaceDashboardExportPlan(config WorkspaceConfigResolved, outputRoot string, snapshotAt time.Time, dryRun bool, client WorkspaceClient, staleDays int, options WorkspaceStatusOptions) (DashboardExportPlan, DashboardExportBundle, error) {
	if client == nil {
		return DashboardExportPlan{}, DashboardExportBundle{}, fmt.Errorf("workspace dashboard export client is required")
	}

	report, err := BuildWorkspaceStatusReportWithOptions(config, client, snapshotAt, staleDays, options)
	if err != nil {
		return DashboardExportPlan{}, DashboardExportBundle{}, err
	}
	snapshotText := report.FetchedAt
	if strings.TrimSpace(snapshotText) == "" {
		snapshotText = formatGitHubTime(snapshotAt)
	}
	workspace := report.Workspace
	sources := dashboardWorkspaceExportSources(snapshotText)
	deepLinks := workspaceDashboardDeepLinks(report.Queues)
	artifacts := append(DashboardExportWorkspaceArtifacts(), workspaceDashboardDeepLinkArtifacts(deepLinks)...)
	dashboard := buildWorkspaceDashboard(report, snapshotText, deepLinks)
	warnings := workspaceDashboardWarningMessages(dashboard.Warnings)

	plan := DashboardExportPlan{
		Command:       "export dashboard",
		DryRun:        dryRun,
		Workspace:     &workspace,
		OutputRoot:    outputRoot,
		SchemaVersion: DashboardExportSchemaVersion,
		SnapshotAt:    snapshotText,
		Sources:       sources,
		Artifacts:     artifacts,
		Counts: DashboardExportCounts{
			WorkspaceRepos:      len(report.Repos),
			WorkspaceQueueItems: countWorkspaceQueueItems(report.Queues),
			Warnings:            len(dashboard.Warnings),
		},
		Warnings: warnings,
	}

	queues := report.Queues
	bundle := DashboardExportBundle{
		Manifest: DashboardExportManifest{
			SchemaVersion: DashboardExportSchemaVersion,
			SnapshotAt:    snapshotText,
			Workspace:     &workspace,
			Sources:       sources,
			Artifacts:     artifacts,
			Generator: DashboardExportGenerator{
				Name: dashboardExportGeneratorName,
				Mode: dashboardExportGeneratorMode,
			},
		},
		Warnings: DashboardExportWarnings{
			Workspace:  &workspace,
			SnapshotAt: snapshotText,
			Warnings:   warnings,
		},
		WorkspaceStatus:    &report,
		WorkspaceQueues:    &queues,
		WorkspaceDashboard: &dashboard,
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

func dashboardWorkspaceExportSources(snapshotAt string) []DashboardExportSource {
	workspace := snapshotAt
	return []DashboardExportSource{
		{Name: "workspace_status", Included: true, SnapshotAt: &workspace},
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

func buildWorkspaceDashboard(report WorkspaceReport, snapshotAt string, deepLinks []dashboardWorkspaceDeepLink) DashboardWorkspaceDashboard {
	warnings := buildWorkspaceDashboardWarnings(report)
	ticketReports, reviewReports := workspaceDashboardDeepLinkPaths(deepLinks)
	return DashboardWorkspaceDashboard{
		SchemaVersion: WorkspaceDashboardSchemaVersion,
		SnapshotAt:    snapshotAt,
		Workspace:     report.Workspace,
		Source: DashboardWorkspaceSource{
			Contract: WorkspaceStatusSourceContract,
			Path:     "raw/workspace_status.json",
		},
		Counts:      report.Counts,
		QueueCounts: report.Queues.Counts,
		TopActions:  buildWorkspaceDashboardTopActions(report.Queues, deepLinks),
		Warnings:    warnings,
		Artifacts: DashboardWorkspaceArtifacts{
			Manifest:           workspaceDashboardManifestPath,
			WorkspaceStatus:    workspaceDashboardRawStatusPath,
			WorkspaceQueues:    workspaceDashboardQueuesPath,
			WorkspaceDashboard: workspaceDashboardIndexPath,
			QueueItemsCSV:      workspaceDashboardQueueCSVPath,
			IndexHTML:          workspaceDashboardHTMLPath,
			TicketReports:      ticketReports,
			ReviewReports:      reviewReports,
		},
	}
}

func buildWorkspaceDashboardTopActions(report WorkspaceQueuesReport, deepLinks []dashboardWorkspaceDeepLink) []DashboardWorkspaceTopAction {
	items := workspaceDashboardAllQueueItems(report)
	actions := make([]DashboardWorkspaceTopAction, 0, minInt(len(items), workspaceDashboardTopActionLimit))
	for _, item := range items {
		if len(actions) >= workspaceDashboardTopActionLimit {
			break
		}
		ticketPath, reviewPath := workspaceDashboardDeepLinkPathsForItem(deepLinks, item)
		actions = append(actions, DashboardWorkspaceTopAction{
			Queue:           item.Queue,
			Repo:            item.Repo,
			Issue:           item.Issue,
			Title:           item.Title,
			URL:             workspaceQueueIssueURL(item),
			LocalTicketHTML: ticketPath,
			LocalReviewHTML: reviewPath,
			ReasonCodes:     append([]string(nil), item.ReasonCodes...),
			NextSafeCommand: item.NextSafeCommand,
			SourceRefs: []string{
				fmt.Sprintf("workspace_queue:%s:%s#%d", item.Queue, item.Repo, item.Issue),
				fmt.Sprintf("issue:%s#%d", item.Repo, item.Issue),
			},
		})
	}
	return actions
}

func workspaceDashboardDeepLinks(report WorkspaceQueuesReport) []dashboardWorkspaceDeepLink {
	items := workspaceDashboardAllQueueItems(report)
	links := make([]dashboardWorkspaceDeepLink, 0, len(items))
	seenTickets := map[string]struct{}{}
	seenReviews := map[string]struct{}{}
	for _, item := range items {
		key := workspaceDashboardItemKey(item)
		link := dashboardWorkspaceDeepLink{Item: item}
		if item.Issue > 0 && strings.TrimSpace(item.Repo) != "" {
			if _, ok := seenTickets[key]; !ok {
				link.TicketPath = workspaceDashboardTicketReportPath(item)
				seenTickets[key] = struct{}{}
			}
		}
		if item.PullRequest != nil && item.PullRequest.Number > 0 {
			reviewKey := workspaceDashboardReviewKey(item)
			if _, ok := seenReviews[reviewKey]; !ok {
				link.ReviewPath = workspaceDashboardReviewReportPath(item)
				seenReviews[reviewKey] = struct{}{}
			}
		}
		if link.TicketPath != "" || link.ReviewPath != "" {
			links = append(links, link)
		}
	}
	return links
}

func workspaceDashboardDeepLinkArtifacts(links []dashboardWorkspaceDeepLink) []DashboardExportArtifact {
	artifacts := make([]DashboardExportArtifact, 0, len(links)*2)
	for _, link := range links {
		if strings.TrimSpace(link.TicketPath) != "" {
			artifacts = append(artifacts, DashboardExportArtifact{Path: link.TicketPath, Kind: "html", WillWrite: true})
		}
		if strings.TrimSpace(link.ReviewPath) != "" {
			artifacts = append(artifacts, DashboardExportArtifact{Path: link.ReviewPath, Kind: "html", WillWrite: true})
		}
	}
	return artifacts
}

func workspaceDashboardDeepLinkPaths(links []dashboardWorkspaceDeepLink) ([]string, []string) {
	tickets := []string{}
	reviews := []string{}
	for _, link := range links {
		if strings.TrimSpace(link.TicketPath) != "" {
			tickets = append(tickets, link.TicketPath)
		}
		if strings.TrimSpace(link.ReviewPath) != "" {
			reviews = append(reviews, link.ReviewPath)
		}
	}
	return tickets, reviews
}

func workspaceDashboardDeepLinkPathsForItem(links []dashboardWorkspaceDeepLink, item WorkspaceQueueItem) (string, string) {
	itemKey := workspaceDashboardItemKey(item)
	reviewKey := workspaceDashboardReviewKey(item)
	ticketPath := ""
	reviewPath := ""
	for _, link := range links {
		if workspaceDashboardItemKey(link.Item) == itemKey {
			if ticketPath == "" && link.TicketPath != "" {
				ticketPath = link.TicketPath
			}
			if reviewPath == "" && reviewKey != "" && workspaceDashboardReviewKey(link.Item) == reviewKey && link.ReviewPath != "" {
				reviewPath = link.ReviewPath
			}
		}
	}
	return ticketPath, reviewPath
}

func workspaceDashboardItemKey(item WorkspaceQueueItem) string {
	return strings.ToLower(strings.TrimSpace(item.Repo)) + "#" + strconv.Itoa(item.Issue)
}

func workspaceDashboardReviewKey(item WorkspaceQueueItem) string {
	if item.PullRequest == nil || item.PullRequest.Number <= 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(item.Repo)) + "#pr-" + strconv.Itoa(item.PullRequest.Number)
}

func workspaceDashboardTicketReportPath(item WorkspaceQueueItem) string {
	return fmt.Sprintf("tickets/%s-ticket-%d.html", workspaceDashboardRepoSlug(item.Repo), item.Issue)
}

func workspaceDashboardReviewReportPath(item WorkspaceQueueItem) string {
	if item.PullRequest == nil || item.PullRequest.Number <= 0 {
		return ""
	}
	return fmt.Sprintf("reviews/%s-pr-%d.html", workspaceDashboardRepoSlug(item.Repo), item.PullRequest.Number)
}

func workspaceDashboardRepoSlug(repo string) string {
	repo = strings.ToLower(strings.TrimSpace(repo))
	var b strings.Builder
	lastDash := false
	for _, r := range repo {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "repo"
	}
	return slug
}

func workspaceDashboardStatusFromQueueItem(item WorkspaceQueueItem) WorkStatusResult {
	status := WorkStatusResult{
		Command:       "ticket status",
		SchemaVersion: TicketStatusSchemaVersion,
		Repo:          item.Repo,
		Issue:         item.Issue,
		Title:         item.Title,
		State:         item.State,
		Status:        item.Status,
		Labels:        append([]string(nil), item.Labels...),
		Milestone:     item.Milestone,
		Blockers:      append([]string(nil), item.Evidence.Blockers...),
		NextAction:    item.Evidence.NextAction,
		NextStep:      item.NextSafeCommand,
		ChecksStatus:  item.Evidence.ChecksStatus,
		ReviewStatus:  item.Evidence.ReviewStatus,
	}
	if item.PullRequest != nil {
		status.PRNumber = item.PullRequest.Number
		status.PRURL = item.PullRequest.URL
		status.PRState = item.PullRequest.State
		status.PullRequest = &TicketStatusPullRequest{
			Available:      item.PullRequest.Number > 0,
			Number:         item.PullRequest.Number,
			URL:            item.PullRequest.URL,
			State:          item.PullRequest.State,
			ReviewDecision: item.PullRequest.ReviewDecision,
			IsDraft:        item.PullRequest.Draft,
		}
	}
	if strings.TrimSpace(item.Evidence.TicketReadiness) != "" {
		status.TicketReadiness = &TicketReadinessReport{
			SchemaVersion: TicketReadinessSchemaVersion,
			Readiness:     item.Evidence.TicketReadiness,
		}
	}
	if strings.TrimSpace(item.Evidence.PRReadiness) != "" || item.PullRequest != nil {
		readiness := strings.TrimSpace(item.Evidence.PRReadiness)
		if readiness == "" {
			readiness = "unknown"
		}
		status.PRReadiness = &PRReadinessReport{
			SchemaVersion: PRReadinessSchemaVersion,
			Repo:          item.Repo,
			Issue:         item.Issue,
			Readiness:     readiness,
			NextAction:    item.Evidence.NextAction,
		}
		if item.PullRequest != nil {
			status.PRReadiness.PullRequest = item.PullRequest.Number
		}
	}
	return status
}

func workspaceDashboardReviewFromQueueItem(item WorkspaceQueueItem) AgentPromptReport {
	report := AgentPromptReport{
		Command: "ticket review",
		Repo:    item.Repo,
		Ticket:  item.Issue,
		Role:    AgentPromptRoleReviewer,
		Profile: AgentPromptProfileDefault,
		Issue: AgentPromptIssue{
			Number: item.Issue,
			Title:  item.Title,
			State:  item.State,
			Labels: append([]string(nil), item.Labels...),
		},
		Evidence: &AgentPromptEvidence{
			Blockers:    append([]string(nil), item.Evidence.Blockers...),
			FinishReady: item.Evidence.PRReadiness == "ready_for_finish",
		},
		NextStep: item.NextSafeCommand,
	}
	if item.PullRequest != nil {
		report.PR = &AgentPromptPR{
			Number:         item.PullRequest.Number,
			URL:            item.PullRequest.URL,
			State:          item.PullRequest.State,
			ReviewDecision: item.PullRequest.ReviewDecision,
			IsDraft:        item.PullRequest.Draft,
			FinishReady:    item.Evidence.PRReadiness == "ready_for_finish",
		}
	}
	readiness := item.Evidence.PRReadiness
	if strings.TrimSpace(readiness) == "" {
		readiness = "unknown"
	}
	report.PRReady = &PRReadinessReport{
		SchemaVersion: PRReadinessSchemaVersion,
		Repo:          item.Repo,
		Issue:         item.Issue,
		Readiness:     readiness,
		NextAction:    item.Evidence.NextAction,
	}
	if item.PullRequest != nil {
		report.PRReady.PullRequest = item.PullRequest.Number
	}
	report.Review = buildAgentReviewContract(report)
	report.Prompt = RenderAgentPrompt(report)
	return report
}

func buildWorkspaceDashboardWarnings(report WorkspaceReport) []DashboardWorkspaceWarning {
	warnings := make([]DashboardWorkspaceWarning, 0)
	if report.RateLimit != nil && !report.RateLimit.BudgetOK {
		warnings = append(warnings, DashboardWorkspaceWarning{
			Code:     "workspace_rate_budget_low",
			Severity: "warning",
			Message:  fmt.Sprintf("GitHub API budget low: remaining=%d estimated=%d reset=%s", report.RateLimit.Remaining, report.RateLimit.EstimatedRequests, report.RateLimit.ResetAt),
		})
	}
	if report.Cache.Enabled && report.Cache.Stale > 0 {
		warnings = append(warnings, DashboardWorkspaceWarning{
			Code:     "workspace_cache_stale",
			Severity: "warning",
			Message:  fmt.Sprintf("%d workspace status cache entries were stale.", report.Cache.Stale),
		})
	}
	for _, warning := range report.Warnings {
		code := workspaceDashboardWarningCode(warning)
		structured := DashboardWorkspaceWarning{Code: code, Severity: "warning", Message: warning}
		if code == "workspace_rate_budget_low" && hasWorkspaceDashboardWarning(warnings, code) {
			continue
		}
		warnings = append(warnings, structured)
	}
	return warnings
}

func workspaceDashboardWarningCode(warning string) string {
	lower := strings.ToLower(warning)
	switch {
	case strings.Contains(lower, "budget low") || strings.Contains(lower, "rate limit"):
		return "workspace_rate_budget_low"
	case strings.Contains(lower, "queue") && (strings.Contains(lower, "detail") || strings.Contains(lower, "status")):
		return "workspace_queue_detail_incomplete"
	case strings.Contains(lower, "cache") && strings.Contains(lower, "stale"):
		return "workspace_cache_stale"
	default:
		return "workspace_status_unavailable"
	}
}

func hasWorkspaceDashboardWarning(warnings []DashboardWorkspaceWarning, code string) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}

func workspaceDashboardWarningMessages(warnings []DashboardWorkspaceWarning) []string {
	values := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		values = append(values, warning.Code+": "+warning.Message)
	}
	return values
}

func workspaceDashboardAllQueueItems(report WorkspaceQueuesReport) []WorkspaceQueueItem {
	items := make([]WorkspaceQueueItem, 0, countWorkspaceQueueItems(report))
	items = append(items, report.Queues.AgentReady...)
	items = append(items, report.Queues.ReviewNeeded...)
	items = append(items, report.Queues.FinishReady...)
	items = append(items, report.Queues.Blocked...)
	items = append(items, report.Queues.FailedCheck...)
	items = append(items, report.Queues.HumanDecision...)
	return items
}

func countWorkspaceQueueItems(report WorkspaceQueuesReport) int {
	return len(report.Queues.AgentReady) +
		len(report.Queues.ReviewNeeded) +
		len(report.Queues.FinishReady) +
		len(report.Queues.Blocked) +
		len(report.Queues.FailedCheck) +
		len(report.Queues.HumanDecision)
}

func workspaceQueueIssueURL(item WorkspaceQueueItem) string {
	if strings.TrimSpace(item.Repo) == "" || item.Issue <= 0 {
		return ""
	}
	return fmt.Sprintf("https://github.com/%s/issues/%d", item.Repo, item.Issue)
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
	if plan.Repo != "" {
		lines = append(lines, "repo: "+plan.Repo)
	}
	if plan.Workspace != nil {
		lines = append(lines, "workspace: "+plan.Workspace.Name+" ("+plan.Workspace.Owner+")")
	}
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
	lines = append(lines, "counts:")
	lines = append(lines, fmt.Sprintf("  issues: %d", plan.Counts.Issues))
	lines = append(lines, fmt.Sprintf("  pull_requests: %d", plan.Counts.PullRequests))
	lines = append(lines, fmt.Sprintf("  milestones: %d", plan.Counts.Milestones))
	lines = append(lines, fmt.Sprintf("  roadmap_items: %d", plan.Counts.RoadmapItems))
	lines = append(lines, fmt.Sprintf("  transitions: %d", plan.Counts.Transitions))
	if plan.Counts.WorkspaceRepos > 0 {
		lines = append(lines, fmt.Sprintf("  workspace_repos: %d", plan.Counts.WorkspaceRepos))
	}
	if plan.Counts.WorkspaceQueueItems > 0 {
		lines = append(lines, fmt.Sprintf("  workspace_queue_items: %d", plan.Counts.WorkspaceQueueItems))
	}
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
	scope, err := newLocalWriteScope(outputRoot)
	if err != nil {
		return err
	}

	if err := writeDashboardExportJSON(scope, "manifest.json", bundle.Manifest); err != nil {
		return err
	}
	if strings.TrimSpace(bundle.Manifest.Repo) != "" {
		executionCSV, err := renderDashboardExecutionCSV(bundle.ExecutionBoard.Items)
		if err != nil {
			return err
		}
		roadmapCSV, err := renderDashboardRoadmapCSV(bundle.RoadmapTimeline.Items)
		if err != nil {
			return err
		}

		if err := writeDashboardExportJSON(scope, "raw/github.json", bundle.RawGitHub); err != nil {
			return err
		}
		if err := writeDashboardExportJSON(scope, "raw/transitions.json", bundle.RawTransitions); err != nil {
			return err
		}
		if err := writeDashboardExportJSON(scope, "raw/capabilities.json", bundle.RawCapabilities); err != nil {
			return err
		}
		if err := writeDashboardExportJSON(scope, "derived/execution_board.json", bundle.ExecutionBoard); err != nil {
			return err
		}
		if err := writeDashboardExportJSON(scope, "derived/roadmap_timeline.json", bundle.RoadmapTimeline); err != nil {
			return err
		}
		if err := writeDashboardExportJSON(scope, "derived/warnings.json", bundle.Warnings); err != nil {
			return err
		}
		if err := scope.WriteFile("csv/execution_items.csv", executionCSV, 0o644); err != nil {
			return err
		}
		if err := scope.WriteFile("csv/roadmap_items.csv", roadmapCSV, 0o644); err != nil {
			return err
		}
	}
	if bundle.WorkspaceStatus != nil {
		if err := writeDashboardExportJSON(scope, workspaceDashboardRawStatusPath, bundle.WorkspaceStatus); err != nil {
			return err
		}
	}
	if bundle.WorkspaceQueues != nil {
		if err := writeDashboardExportJSON(scope, workspaceDashboardQueuesPath, bundle.WorkspaceQueues); err != nil {
			return err
		}
		queueCSV, err := renderDashboardWorkspaceQueueCSV(*bundle.WorkspaceQueues)
		if err != nil {
			return err
		}
		if err := scope.WriteFile(workspaceDashboardQueueCSVPath, queueCSV, 0o644); err != nil {
			return err
		}
	}
	if bundle.WorkspaceDashboard != nil {
		if err := writeDashboardExportJSON(scope, workspaceDashboardIndexPath, bundle.WorkspaceDashboard); err != nil {
			return err
		}
		html, err := renderWorkspaceDashboardHTML(*bundle.WorkspaceDashboard)
		if err != nil {
			return err
		}
		if err := scope.WriteFile(workspaceDashboardHTMLPath, html, 0o644); err != nil {
			return err
		}
	}
	if bundle.WorkspaceQueues != nil {
		for _, link := range workspaceDashboardDeepLinks(*bundle.WorkspaceQueues) {
			if strings.TrimSpace(link.TicketPath) != "" {
				status := workspaceDashboardStatusFromQueueItem(link.Item)
				if err := scope.WriteFile(link.TicketPath, []byte(RenderTicketStatusHTML(status)), 0o644); err != nil {
					return err
				}
			}
			if strings.TrimSpace(link.ReviewPath) != "" {
				review := workspaceDashboardReviewFromQueueItem(link.Item)
				if err := scope.WriteFile(link.ReviewPath, []byte(RenderTicketReviewHTML(review)), 0o644); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func writeDashboardExportJSON(scope localWriteScope, relPath string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return scope.WriteFile(relPath, encoded, 0o644)
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

func renderDashboardWorkspaceQueueCSV(report WorkspaceQueuesReport) ([]byte, error) {
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	if err := writer.Write(dashboardWorkspaceQueueCSVHeaders); err != nil {
		return nil, err
	}
	for _, item := range workspaceDashboardAllQueueItems(report) {
		prNumber := ""
		prState := ""
		if item.PullRequest != nil {
			prNumber = strconv.Itoa(item.PullRequest.Number)
			prState = item.PullRequest.State
		}
		row := []string{
			item.Queue,
			item.Repo,
			strconv.Itoa(item.Issue),
			item.Title,
			item.State,
			item.Status,
			prNumber,
			prState,
			strings.Join(item.ReasonCodes, ","),
			item.NextSafeCommand,
			workspaceQueueIssueURL(item),
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return buffer.Bytes(), writer.Error()
}

var workspaceDashboardHTMLTemplate = template.Must(template.New("workspace-dashboard").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Gira Workspace Dashboard</title>
  <style>
    body { font-family: system-ui, sans-serif; margin: 0; color: #202124; background: #fff; }
    main { max-width: 1120px; margin: 0 auto; padding: 2rem; }
    header, section { border-bottom: 1px solid #dfe3e8; padding: 1.25rem 0; }
    h1, h2 { margin: 0 0 .75rem; }
    dl, table { width: 100%; }
    dl { display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: .75rem; }
    dt { font-size: .8rem; color: #5f6368; }
    dd { margin: .2rem 0 0; font-size: 1.25rem; font-weight: 650; }
    table { border-collapse: collapse; background: #fff; }
    th, td { padding: .6rem; border-top: 1px solid #eceff3; text-align: left; vertical-align: top; }
    th { font-size: .8rem; color: #5f6368; }
    code { white-space: pre-wrap; word-break: break-word; }
    a { color: #0b57d0; }
  </style>
</head>
<body>
<main>
  <header>
    <h1>{{.Workspace.Name}} Workspace Dashboard</h1>
    <p>Owner: {{.Workspace.Owner}} | Snapshot: {{.SnapshotAt}} | Source: {{.Source.Contract}} at {{.Source.Path}}</p>
  </header>
  <section>
    <h2>Counts</h2>
    <dl>
      <div><dt>Backlog</dt><dd>{{.Counts.Backlog}}</dd></div>
      <div><dt>Repo Open</dt><dd>{{.Counts.RepoOpen}}</dd></div>
      <div><dt>Ready</dt><dd>{{.Counts.Ready}}</dd></div>
      <div><dt>In Progress</dt><dd>{{.Counts.InProgress}}</dd></div>
      <div><dt>Blocked</dt><dd>{{.Counts.Blocked}}</dd></div>
      <div><dt>Stale</dt><dd>{{.Counts.Stale}}</dd></div>
    </dl>
  </section>
  <section>
    <h2>Queue Counts</h2>
    <dl>
      <div><dt>Agent Ready</dt><dd>{{.QueueCounts.AgentReady}}</dd></div>
      <div><dt>Review Needed</dt><dd>{{.QueueCounts.ReviewNeeded}}</dd></div>
      <div><dt>Finish Ready</dt><dd>{{.QueueCounts.FinishReady}}</dd></div>
      <div><dt>Blocked</dt><dd>{{.QueueCounts.Blocked}}</dd></div>
      <div><dt>Failed Check</dt><dd>{{.QueueCounts.FailedCheck}}</dd></div>
      <div><dt>Human Decision</dt><dd>{{.QueueCounts.HumanDecision}}</dd></div>
    </dl>
  </section>
  <section>
    <h2>Top Actions</h2>
    <table>
      <thead><tr><th>Queue</th><th>Item</th><th>Reason</th><th>Next Command</th></tr></thead>
      <tbody>
      {{range .TopActions}}
        <tr>
          <td>{{.Queue}}</td>
          <td>
            <a href="{{.URL}}">{{.Repo}}#{{.Issue}}</a><br>{{.Title}}
            {{if .LocalTicketHTML}}<br><a href="{{.LocalTicketHTML}}">Ticket report</a>{{end}}
            {{if .LocalReviewHTML}}<br><a href="{{.LocalReviewHTML}}">Review packet</a>{{end}}
          </td>
          <td>{{range .ReasonCodes}}<code>{{.}}</code> {{end}}</td>
          <td><code>{{.NextSafeCommand}}</code></td>
        </tr>
      {{else}}
        <tr><td colspan="4">No queue actions in this snapshot.</td></tr>
      {{end}}
      </tbody>
    </table>
  </section>
  <section>
    <h2>Warnings</h2>
    <table>
      <thead><tr><th>Code</th><th>Severity</th><th>Message</th></tr></thead>
      <tbody>
      {{range .Warnings}}
        <tr><td><code>{{.Code}}</code></td><td>{{.Severity}}</td><td>{{.Message}}</td></tr>
      {{else}}
        <tr><td colspan="3">No warnings.</td></tr>
      {{end}}
      </tbody>
    </table>
  </section>
</main>
</body>
</html>
`))

func renderWorkspaceDashboardHTML(dashboard DashboardWorkspaceDashboard) ([]byte, error) {
	var buffer bytes.Buffer
	if err := workspaceDashboardHTMLTemplate.Execute(&buffer, dashboard); err != nil {
		return nil, err
	}
	buffer.WriteByte('\n')
	return buffer.Bytes(), nil
}
