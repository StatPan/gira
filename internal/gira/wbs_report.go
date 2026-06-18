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
	Command          string                    `json:"command"`
	SchemaVersion    string                    `json:"schema_version"`
	Repo             string                    `json:"repo"`
	StateFilter      string                    `json:"state_filter"`
	GeneratedAt      string                    `json:"generated_at"`
	Items            []WBSReportItem           `json:"items"`
	Counts           WBSReportCounts           `json:"counts"`
	Warnings         []string                  `json:"warnings,omitempty"`
	WarningItems     []WBSWarningItem          `json:"warning_items,omitempty"`
	MilestoneCleanup []WBSMilestoneCleanupItem `json:"milestone_cleanup,omitempty"`
	Sources          []WBSReportSource         `json:"sources"`
}

type WBSReportItem struct {
	WBSID                  string               `json:"wbs_id"`
	ParentID               string               `json:"parent_id,omitempty"`
	Level                  int                  `json:"level"`
	Kind                   string               `json:"kind"`
	Repo                   string               `json:"repo"`
	Issue                  int                  `json:"issue,omitempty"`
	Title                  string               `json:"title"`
	State                  string               `json:"state,omitempty"`
	Status                 string               `json:"status,omitempty"`
	Priority               string               `json:"priority,omitempty"`
	Owner                  string               `json:"owner,omitempty"`
	Workstream             string               `json:"workstream,omitempty"`
	Milestone              string               `json:"milestone,omitempty"`
	StartDate              string               `json:"start_date,omitempty"`
	TargetDate             string               `json:"target_date,omitempty"`
	Dependency             string               `json:"dependency,omitempty"`
	Progress               int                  `json:"progress"`
	Children               int                  `json:"children"`
	Source                 string               `json:"source"`
	ParentSource           string               `json:"parent_source,omitempty"`
	ParentCandidates       []WBSParentCandidate `json:"parent_candidates,omitempty"`
	ParentResolutionReason string               `json:"parent_resolution_reason,omitempty"`
	URL                    string               `json:"url,omitempty"`
	SourceRefs             []string             `json:"source_refs,omitempty"`
}

type WBSWarningItem struct {
	Code             string               `json:"code"`
	Warning          string               `json:"warning"`
	Milestone        string               `json:"milestone,omitempty"`
	CandidateParents []WBSParentCandidate `json:"candidate_parents,omitempty"`
	AffectedChildren []WBSAffectedChild   `json:"affected_children,omitempty"`
	EvidenceSources  []string             `json:"evidence_sources,omitempty"`
	Remediation      string               `json:"remediation,omitempty"`
}

type WBSMilestoneCleanupItem struct {
	Milestone       string `json:"milestone"`
	State           string `json:"state,omitempty"`
	DueDate         string `json:"due_date,omitempty"`
	TotalItems      int    `json:"total_items"`
	ExecutableItems int    `json:"executable_items"`
	Reason          string `json:"reason"`
}

type WBSParentCandidate struct {
	Issue    int      `json:"issue"`
	Title    string   `json:"title,omitempty"`
	Evidence []string `json:"evidence,omitempty"`
	Strength string   `json:"strength,omitempty"`
	URL      string   `json:"url,omitempty"`
}

type WBSAffectedChild struct {
	Issue            int      `json:"issue"`
	Title            string   `json:"title,omitempty"`
	CandidateParents []int    `json:"candidate_parents,omitempty"`
	Evidence         []string `json:"evidence,omitempty"`
	Resolution       string   `json:"resolution,omitempty"`
	ResolutionReason string   `json:"resolution_reason,omitempty"`
	URL              string   `json:"url,omitempty"`
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
	report.Items, report.Warnings, report.WarningItems = buildWBSReportItems(repo, issues, milestones, projectSnapshot, report.Warnings)
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
	report.MilestoneCleanup = buildWBSMilestoneCleanup(report.Items, milestones)
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

func buildWBSReportItems(repo RepoRef, issues []WBSRawIssue, milestones []DashboardRawMilestone, projectSnapshot ProjectSyncSnapshot, warnings []string) ([]WBSReportItem, []string, []WBSWarningItem) {
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

	epicByNumber := map[int]WBSRawIssue{}
	for _, epic := range epics {
		epicByNumber[epic.IssueNumber] = epic
	}
	epicEvidenceByChild := map[int]map[int][]string{}
	for _, epic := range epics {
		for childNumber, evidence := range extractWBSIssueRefEvidence(epic.Body) {
			if _, ok := epicEvidenceByChild[childNumber]; !ok {
				epicEvidenceByChild[childNumber] = map[int][]string{}
			}
			epicEvidenceByChild[childNumber][epic.IssueNumber] = appendUniqueStrings(epicEvidenceByChild[childNumber][epic.IssueNumber], evidence...)
		}
	}
	childrenByEpic := map[int][]WBSRawIssue{}
	linked := map[int]struct{}{}
	epicsByMilestone := map[string][]WBSRawIssue{}
	for _, epic := range epics {
		if strings.TrimSpace(epic.Milestone) != "" {
			epicsByMilestone[epic.Milestone] = append(epicsByMilestone[epic.Milestone], epic)
		}
	}
	candidatesByIssue := map[int][]WBSParentCandidate{}
	resolutionByIssue := map[int]wbsParentResolution{}
	for _, issue := range nonEpics {
		candidates := wbsParentCandidatesForIssue(issue, epicsByMilestone, epicEvidenceByChild, epicByNumber)
		candidatesByIssue[issue.IssueNumber] = candidates
		parent, source, reason, ok := resolveWBSParent(candidates)
		resolutionByIssue[issue.IssueNumber] = wbsParentResolution{Parent: parent, Source: source, Reason: reason}
		if !ok {
			continue
		}
		childrenByEpic[parent] = append(childrenByEpic[parent], issue)
		linked[issue.IssueNumber] = struct{}{}
	}
	warningItems := []WBSWarningItem{}
	for milestone, milestoneEpics := range epicsByMilestone {
		if len(milestoneEpics) > 1 {
			warning := "ambiguous_milestone_parent:" + milestone
			warnings = appendUniqueStrings(warnings, warning)
			warningItems = append(warningItems, buildWBSMilestoneParentWarning(warning, milestone, milestoneEpics, nonEpics, candidatesByIssue, resolutionByIssue))
		}
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
			resolution := resolutionByIssue[child.IssueNumber]
			childItem := wbsItemFromIssue(repo, child, childID, topID, 2, datesByIssue, milestoneDue, resolution.Source)
			childItem.ParentSource = resolution.Source
			childItem.ParentCandidates = candidatesByIssue[child.IssueNumber]
			childItem.ParentResolutionReason = resolution.Reason
			items = append(items, childItem)
		}
		nextTop++
	}
	for _, issue := range nonEpics {
		if _, ok := linked[issue.IssueNumber]; ok {
			continue
		}
		item := wbsItemFromIssue(repo, issue, strconv.Itoa(nextTop), "", 1, datesByIssue, milestoneDue, "unlinked")
		item.ParentCandidates = candidatesByIssue[issue.IssueNumber]
		if resolution := resolutionByIssue[issue.IssueNumber]; resolution.Reason != "" {
			item.ParentResolutionReason = resolution.Reason
		}
		items = append(items, item)
		nextTop++
	}
	return items, warnings, warningItems
}

type wbsParentResolution struct {
	Parent int
	Source string
	Reason string
}

func wbsParentCandidatesForIssue(issue WBSRawIssue, epicsByMilestone map[string][]WBSRawIssue, epicEvidenceByChild map[int]map[int][]string, epicByNumber map[int]WBSRawIssue) []WBSParentCandidate {
	byEpic := map[int]WBSParentCandidate{}
	for parentNumber, evidence := range extractWBSParentRefs(issue.Body) {
		if epic, ok := epicByNumber[parentNumber]; ok {
			addWBSParentCandidate(byEpic, epic, evidence...)
		}
	}
	for parentNumber, evidence := range epicEvidenceByChild[issue.IssueNumber] {
		if epic, ok := epicByNumber[parentNumber]; ok {
			addWBSParentCandidate(byEpic, epic, evidence...)
		}
	}
	if strings.TrimSpace(issue.Milestone) != "" {
		for _, epic := range epicsByMilestone[issue.Milestone] {
			addWBSParentCandidate(byEpic, epic, "milestone")
		}
	}
	candidates := make([]WBSParentCandidate, 0, len(byEpic))
	for _, candidate := range byEpic {
		candidate.Evidence = sortWBSEvidence(candidate.Evidence)
		candidate.Strength = wbsEvidenceStrengthLabel(candidate.Evidence)
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if wbsEvidenceRank(candidates[i].Evidence) != wbsEvidenceRank(candidates[j].Evidence) {
			return wbsEvidenceRank(candidates[i].Evidence) > wbsEvidenceRank(candidates[j].Evidence)
		}
		return candidates[i].Issue < candidates[j].Issue
	})
	return candidates
}

func addWBSParentCandidate(byEpic map[int]WBSParentCandidate, epic WBSRawIssue, evidence ...string) {
	candidate := byEpic[epic.IssueNumber]
	if candidate.Issue == 0 {
		candidate = WBSParentCandidate{
			Issue: epic.IssueNumber,
			Title: epic.Title,
			URL:   epic.URL,
		}
	}
	candidate.Evidence = appendUniqueStrings(candidate.Evidence, evidence...)
	byEpic[epic.IssueNumber] = candidate
}

func resolveWBSParent(candidates []WBSParentCandidate) (int, string, string, bool) {
	if len(candidates) == 0 {
		return 0, "", "no_parent_candidates", false
	}
	if len(candidates) == 1 {
		source := strings.Join(candidates[0].Evidence, ",")
		return candidates[0].Issue, source, "selected_only_candidate", true
	}
	topRank := wbsEvidenceRank(candidates[0].Evidence)
	top := []WBSParentCandidate{}
	for _, candidate := range candidates {
		if wbsEvidenceRank(candidate.Evidence) == topRank {
			top = append(top, candidate)
		}
	}
	if len(top) == 1 && topRank > wbsEvidenceRank([]string{"milestone"}) {
		source := strings.Join(top[0].Evidence, ",")
		return top[0].Issue, source, "selected_unique_strongest_candidate", true
	}
	return 0, "", "ambiguous_parent_candidates", false
}

func buildWBSMilestoneParentWarning(warning string, milestone string, milestoneEpics []WBSRawIssue, nonEpics []WBSRawIssue, candidatesByIssue map[int][]WBSParentCandidate, resolutionByIssue map[int]wbsParentResolution) WBSWarningItem {
	parentEvidence := map[int][]string{}
	parentByNumber := map[int]WBSRawIssue{}
	for _, epic := range milestoneEpics {
		parentByNumber[epic.IssueNumber] = epic
		parentEvidence[epic.IssueNumber] = appendUniqueStrings(parentEvidence[epic.IssueNumber], "milestone")
	}
	affected := []WBSAffectedChild{}
	evidenceSources := []string{"milestone"}
	for _, child := range nonEpics {
		if child.Milestone != milestone {
			continue
		}
		childCandidates := []int{}
		childEvidence := []string{}
		for _, candidate := range candidatesByIssue[child.IssueNumber] {
			if _, ok := parentByNumber[candidate.Issue]; !ok {
				continue
			}
			childCandidates = append(childCandidates, candidate.Issue)
			childEvidence = appendUniqueStrings(childEvidence, candidate.Evidence...)
			parentEvidence[candidate.Issue] = appendUniqueStrings(parentEvidence[candidate.Issue], candidate.Evidence...)
			evidenceSources = appendUniqueStrings(evidenceSources, candidate.Evidence...)
		}
		sort.Ints(childCandidates)
		resolution := resolutionByIssue[child.IssueNumber]
		affected = append(affected, WBSAffectedChild{
			Issue:            child.IssueNumber,
			Title:            child.Title,
			CandidateParents: childCandidates,
			Evidence:         sortWBSEvidence(childEvidence),
			Resolution:       wbsParentResolutionValue(resolution),
			ResolutionReason: resolution.Reason,
			URL:              child.URL,
		})
	}
	candidates := make([]WBSParentCandidate, 0, len(milestoneEpics))
	for _, epic := range milestoneEpics {
		evidence := sortWBSEvidence(parentEvidence[epic.IssueNumber])
		candidates = append(candidates, WBSParentCandidate{
			Issue:    epic.IssueNumber,
			Title:    epic.Title,
			Evidence: evidence,
			Strength: wbsEvidenceStrengthLabel(evidence),
			URL:      epic.URL,
		})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Issue < candidates[j].Issue })
	sort.Slice(affected, func(i, j int) bool { return affected[i].Issue < affected[j].Issue })
	return WBSWarningItem{
		Code:             "ambiguous_milestone_parent",
		Warning:          warning,
		Milestone:        milestone,
		CandidateParents: candidates,
		AffectedChildren: affected,
		EvidenceSources:  sortWBSEvidence(evidenceSources),
		Remediation:      "Add explicit Parent: #EPIC to affected child issues, convert weak Related links to epic checklist items, or leave multiple root epics in the milestone intentionally.",
	}
}

func wbsParentResolutionValue(resolution wbsParentResolution) string {
	if resolution.Parent <= 0 {
		return ""
	}
	return fmt.Sprintf("#%d", resolution.Parent)
}

func extractWBSIssueRefEvidence(body string) map[int][]string {
	out := map[int][]string{}
	for _, line := range strings.Split(body, "\n") {
		evidence := wbsLineEvidence(line)
		for issueNumber := range extractIssueRefs(line) {
			out[issueNumber] = appendUniqueStrings(out[issueNumber], evidence)
		}
	}
	return out
}

func extractWBSParentRefs(body string) map[int][]string {
	out := map[int][]string{}
	for _, line := range strings.Split(body, "\n") {
		if !strings.Contains(strings.ToLower(line), "parent:") {
			continue
		}
		for issueNumber := range extractIssueRefs(line) {
			out[issueNumber] = appendUniqueStrings(out[issueNumber], "parent")
		}
	}
	return out
}

func wbsLineEvidence(line string) string {
	trimmed := strings.TrimSpace(strings.ToLower(line))
	if strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "- [x]") || strings.HasPrefix(trimmed, "* [ ]") || strings.HasPrefix(trimmed, "* [x]") {
		return "checklist"
	}
	if strings.Contains(trimmed, "parent:") {
		return "parent"
	}
	if strings.Contains(trimmed, "related:") || strings.Contains(trimmed, "relates:") || strings.Contains(trimmed, "relates to") {
		return "related"
	}
	return "body"
}

func sortWBSEvidence(evidence []string) []string {
	order := map[string]int{"parent": 0, "checklist": 1, "body": 2, "related": 3, "milestone": 4}
	out := append([]string(nil), evidence...)
	sort.Slice(out, func(i, j int) bool {
		if order[out[i]] != order[out[j]] {
			return order[out[i]] < order[out[j]]
		}
		return out[i] < out[j]
	})
	return out
}

func wbsEvidenceRank(evidence []string) int {
	rank := 0
	for _, value := range evidence {
		switch value {
		case "parent", "checklist":
			if rank < 3 {
				rank = 3
			}
		case "body":
			if rank < 2 {
				rank = 2
			}
		case "related", "milestone":
			if rank < 1 {
				rank = 1
			}
		}
	}
	return rank
}

func wbsEvidenceStrengthLabel(evidence []string) string {
	switch wbsEvidenceRank(evidence) {
	case 3:
		return "strong"
	case 2:
		return "inferred"
	case 1:
		return "weak"
	default:
		return ""
	}
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
		Owner:      wbsOwnerLabel(issue.Labels),
		Workstream: wbsAreaLabel(issue.Labels),
		Milestone:  issue.Milestone,
		StartDate:  datesForIssue.StartDate,
		TargetDate: targetDate,
		Dependency: wbsDependencyRefs(issue.Body),
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

func wbsAreaLabel(labels []string) string {
	for _, label := range labels {
		trimmed := strings.ToLower(strings.TrimSpace(label))
		if strings.HasPrefix(trimmed, "area:") {
			return strings.TrimPrefix(trimmed, "area:")
		}
	}
	return ""
}

func wbsOwnerLabel(labels []string) string {
	for _, label := range labels {
		trimmed := strings.TrimSpace(label)
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "owner:") {
			return strings.TrimPrefix(trimmed, "owner:")
		}
	}
	return dashboardExportOwnerFromLabels(labels)
}

func wbsDependencyRefs(body string) string {
	refs := []int{}
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.ToLower(strings.TrimSpace(line))
		if !strings.Contains(trimmed, "depend") && !strings.Contains(trimmed, "blocked by") && !strings.Contains(trimmed, "blocks:") {
			continue
		}
		for issueNumber := range extractIssueRefs(line) {
			refs = append(refs, issueNumber)
		}
	}
	sort.Ints(refs)
	return wbsJoinInts(refs)
}

func buildWBSMilestoneCleanup(items []WBSReportItem, milestones []DashboardRawMilestone) []WBSMilestoneCleanupItem {
	if len(milestones) == 0 {
		return nil
	}
	totalByMilestone := map[string]int{}
	executableByMilestone := map[string]int{}
	for _, item := range items {
		if strings.TrimSpace(item.Milestone) == "" {
			continue
		}
		totalByMilestone[item.Milestone]++
		if wbsItemIsExecutable(item) {
			executableByMilestone[item.Milestone]++
		}
	}
	cleanup := []WBSMilestoneCleanupItem{}
	for _, milestone := range milestones {
		total := totalByMilestone[milestone.Title]
		executable := executableByMilestone[milestone.Title]
		reason := ""
		switch {
		case total == 0:
			reason = "empty_milestone"
		case executable == 0:
			reason = "no_executable_items"
		}
		if reason == "" {
			continue
		}
		dueDate := ""
		if milestone.DueOn != nil {
			if normalized, ok := normalizeDate(*milestone.DueOn); ok {
				dueDate = normalized
			}
		}
		cleanup = append(cleanup, WBSMilestoneCleanupItem{
			Milestone:       milestone.Title,
			State:           strings.ToLower(milestone.State),
			DueDate:         dueDate,
			TotalItems:      total,
			ExecutableItems: executable,
			Reason:          reason,
		})
	}
	sort.Slice(cleanup, func(i, j int) bool {
		if cleanup[i].DueDate != cleanup[j].DueDate {
			return cleanup[i].DueDate < cleanup[j].DueDate
		}
		return cleanup[i].Milestone < cleanup[j].Milestone
	})
	return cleanup
}

func wbsItemIsExecutable(item WBSReportItem) bool {
	switch strings.ToLower(strings.TrimSpace(item.Kind)) {
	case "epic":
		return false
	case "task", "story", "bug", "spike", "chore", "issue":
		return true
	default:
		return item.Issue > 0 && item.Children == 0
	}
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
	if len(report.WarningItems) > 0 {
		b.WriteString("<section>\n<h2>Warning Details</h2>\n")
		for _, warning := range report.WarningItems {
			fmt.Fprintf(&b, "<h3>%s</h3>\n", html.EscapeString(warning.Warning))
			if warning.Remediation != "" {
				fmt.Fprintf(&b, "<p class=\"muted\">%s</p>\n", html.EscapeString(warning.Remediation))
			}
			if len(warning.CandidateParents) > 0 {
				b.WriteString("<p><strong>Candidate parents:</strong> ")
				for i, candidate := range warning.CandidateParents {
					if i > 0 {
						b.WriteString(", ")
					}
					fmt.Fprintf(&b, "#%d %s (%s)", candidate.Issue, html.EscapeString(candidate.Title), html.EscapeString(strings.Join(candidate.Evidence, "+")))
				}
				b.WriteString("</p>\n")
			}
			if len(warning.AffectedChildren) > 0 {
				b.WriteString("<ul>\n")
				for _, child := range warning.AffectedChildren {
					resolution := "unresolved"
					if child.Resolution != "" {
						resolution = child.Resolution
					}
					fmt.Fprintf(&b, "<li>#%d %s · candidates %s · evidence %s · %s</li>\n",
						child.Issue,
						html.EscapeString(child.Title),
						html.EscapeString(wbsJoinInts(child.CandidateParents)),
						html.EscapeString(strings.Join(child.Evidence, "+")),
						html.EscapeString(resolution),
					)
				}
				b.WriteString("</ul>\n")
			}
		}
		b.WriteString("</section>\n")
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

func wbsJoinInts(values []int) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, fmt.Sprintf("#%d", value))
	}
	return strings.Join(parts, ",")
}

func wbsMaxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
