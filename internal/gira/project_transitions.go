package gira

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const ProductOSTransitionsProjectName = "Product OS"

type ProjectTransitionsClient interface {
	Repo() RepoRef
	Snapshot() (ProjectTransitionSnapshot, error)
}

type GHProjectTransitionsClient struct {
	repo   RepoRef
	runner CommandRunner
}

func NewGHProjectTransitionsClient(repo RepoRef, runner CommandRunner) GHProjectTransitionsClient {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	return GHProjectTransitionsClient{repo: repo, runner: runner}
}

func (c GHProjectTransitionsClient) Repo() RepoRef {
	return c.repo
}

type ProjectTransitionSnapshot struct {
	Issues       []ProjectTransitionIssue       `json:"issues"`
	PullRequests []ProjectTransitionPullRequest `json:"pull_requests"`
	Branches     []string                       `json:"branches"`
	Milestones   []ProjectTransitionMilestone   `json:"milestones"`
}

type ProjectTransitionIssue struct {
	Number int      `json:"number"`
	Title  string   `json:"title"`
	State  string   `json:"state"`
	Labels []string `json:"labels"`
	Body   string   `json:"body"`
	URL    string   `json:"url,omitempty"`
}

type ProjectTransitionPullRequest struct {
	Number   int     `json:"number"`
	State    string  `json:"state"`
	Draft    bool    `json:"draft"`
	MergedAt *string `json:"merged_at,omitempty"`
	Body     string  `json:"body"`
	HeadRef  string  `json:"head_ref"`
	URL      string  `json:"url,omitempty"`
}

type ProjectTransitionMilestone struct {
	Number       int    `json:"number"`
	Title        string `json:"title"`
	State        string `json:"state,omitempty"`
	OpenIssues   int    `json:"open_issues"`
	ClosedIssues int    `json:"closed_issues"`
}

type ProjectTransitionsReport struct {
	Repo        string                      `json:"repo"`
	Project     string                      `json:"project"`
	Command     string                      `json:"command"`
	DryRun      bool                        `json:"dry_run"`
	Counts      ProjectTransitionCounts     `json:"counts"`
	Transitions []ProjectTransitionPlanItem `json:"transitions"`
	FetchedAt   string                      `json:"fetched_at"`
}

type ProjectTransitionCounts struct {
	Evaluated int `json:"evaluated"`
	Applied   int `json:"applied"`
	Skipped   int `json:"skipped"`
	Conflicts int `json:"conflicts"`
}

type ProjectTransitionPlanItem struct {
	RuleID             string `json:"rule_id"`
	TargetType         string `json:"target_type"`
	TargetID           string `json:"target_id"`
	From               string `json:"from"`
	To                 string `json:"to"`
	Reason             string `json:"reason"`
	ConflictResolution string `json:"conflict_resolution,omitempty"`
	Decision           string `json:"decision"`
}

func (c GHProjectTransitionsClient) Snapshot() (ProjectTransitionSnapshot, error) {
	issues, err := c.fetchIssues()
	if err != nil {
		return ProjectTransitionSnapshot{}, err
	}
	pulls, err := c.fetchPullRequests()
	if err != nil {
		return ProjectTransitionSnapshot{}, err
	}
	branches, err := c.fetchBranches()
	if err != nil {
		return ProjectTransitionSnapshot{}, err
	}
	milestones, err := c.fetchMilestones()
	if err != nil {
		return ProjectTransitionSnapshot{}, err
	}
	return ProjectTransitionSnapshot{
		Issues:       issues,
		PullRequests: pulls,
		Branches:     branches,
		Milestones:   milestones,
	}, nil
}

func (c GHProjectTransitionsClient) fetchIssues() ([]ProjectTransitionIssue, error) {
	output, err := c.runner.Run(
		"gh", "api", "repos/"+c.repo.FullName()+"/issues", "--paginate", "--slurp", "-X", "GET", "-f", "state=all", "-f", "per_page=100",
	)
	if err != nil {
		return nil, err
	}
	var pages json.RawMessage
	if err := json.Unmarshal(output, &pages); err != nil {
		return nil, fmt.Errorf("parse issue pages: %w", err)
	}
	rows, err := flattenPages(pages)
	if err != nil {
		return nil, err
	}
	issues := make([]ProjectTransitionIssue, 0, len(rows))
	for _, row := range rows {
		var raw struct {
			Number      int       `json:"number"`
			Title       string    `json:"title"`
			State       string    `json:"state"`
			Body        string    `json:"body"`
			HTMLURL     string    `json:"html_url"`
			PullRequest *struct{} `json:"pull_request"`
			Labels      []struct {
				Name string `json:"name"`
			} `json:"labels"`
		}
		if err := json.Unmarshal(row, &raw); err != nil {
			return nil, fmt.Errorf("parse issue row: %w", err)
		}
		if raw.PullRequest != nil {
			continue
		}
		labels := make([]string, 0, len(raw.Labels))
		for _, label := range raw.Labels {
			labels = append(labels, label.Name)
		}
		sort.Strings(labels)
		issues = append(issues, ProjectTransitionIssue{
			Number: raw.Number,
			Title:  raw.Title,
			State:  raw.State,
			Labels: labels,
			Body:   raw.Body,
			URL:    raw.HTMLURL,
		})
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].Number < issues[j].Number })
	return issues, nil
}

func (c GHProjectTransitionsClient) fetchPullRequests() ([]ProjectTransitionPullRequest, error) {
	output, err := c.runner.Run(
		"gh", "api", "repos/"+c.repo.FullName()+"/pulls", "--paginate", "--slurp", "-X", "GET", "-f", "state=all", "-f", "per_page=100",
	)
	if err != nil {
		return nil, err
	}
	var pages json.RawMessage
	if err := json.Unmarshal(output, &pages); err != nil {
		return nil, fmt.Errorf("parse pull pages: %w", err)
	}
	rows, err := flattenPages(pages)
	if err != nil {
		return nil, err
	}
	pulls := make([]ProjectTransitionPullRequest, 0, len(rows))
	for _, row := range rows {
		var raw struct {
			Number   int     `json:"number"`
			State    string  `json:"state"`
			Draft    bool    `json:"draft"`
			MergedAt *string `json:"merged_at"`
			Body     string  `json:"body"`
			HTMLURL  string  `json:"html_url"`
			Head     struct {
				Ref string `json:"ref"`
			} `json:"head"`
		}
		if err := json.Unmarshal(row, &raw); err != nil {
			return nil, fmt.Errorf("parse pull row: %w", err)
		}
		pulls = append(pulls, ProjectTransitionPullRequest{
			Number:   raw.Number,
			State:    raw.State,
			Draft:    raw.Draft,
			MergedAt: raw.MergedAt,
			Body:     raw.Body,
			HeadRef:  raw.Head.Ref,
			URL:      raw.HTMLURL,
		})
	}
	sort.Slice(pulls, func(i, j int) bool { return pulls[i].Number < pulls[j].Number })
	return pulls, nil
}

func (c GHProjectTransitionsClient) fetchBranches() ([]string, error) {
	output, err := c.runner.Run(
		"gh", "api", "repos/"+c.repo.FullName()+"/branches", "--paginate", "--slurp", "-X", "GET", "-f", "per_page=100",
	)
	if err != nil {
		return nil, err
	}
	var pages json.RawMessage
	if err := json.Unmarshal(output, &pages); err != nil {
		return nil, fmt.Errorf("parse branch pages: %w", err)
	}
	rows, err := flattenPages(pages)
	if err != nil {
		return nil, err
	}
	branches := make([]string, 0, len(rows))
	for _, row := range rows {
		var raw struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(row, &raw); err != nil {
			return nil, fmt.Errorf("parse branch row: %w", err)
		}
		if strings.TrimSpace(raw.Name) != "" {
			branches = append(branches, raw.Name)
		}
	}
	sort.Strings(branches)
	return branches, nil
}

func (c GHProjectTransitionsClient) fetchMilestones() ([]ProjectTransitionMilestone, error) {
	output, err := c.runner.Run(
		"gh", "api", "repos/"+c.repo.FullName()+"/milestones", "--paginate", "--slurp", "-X", "GET", "-f", "state=all", "-f", "per_page=100",
	)
	if err != nil {
		return nil, err
	}
	var pages json.RawMessage
	if err := json.Unmarshal(output, &pages); err != nil {
		return nil, fmt.Errorf("parse milestone pages: %w", err)
	}
	rows, err := flattenPages(pages)
	if err != nil {
		return nil, err
	}
	milestones := make([]ProjectTransitionMilestone, 0, len(rows))
	for _, row := range rows {
		var raw struct {
			Number       int    `json:"number"`
			Title        string `json:"title"`
			State        string `json:"state"`
			OpenIssues   int    `json:"open_issues"`
			ClosedIssues int    `json:"closed_issues"`
		}
		if err := json.Unmarshal(row, &raw); err != nil {
			return nil, fmt.Errorf("parse milestone row: %w", err)
		}
		milestones = append(milestones, ProjectTransitionMilestone{
			Number:       raw.Number,
			Title:        raw.Title,
			State:        raw.State,
			OpenIssues:   raw.OpenIssues,
			ClosedIssues: raw.ClosedIssues,
		})
	}
	sort.Slice(milestones, func(i, j int) bool { return milestones[i].Number < milestones[j].Number })
	return milestones, nil
}

func BuildProjectTransitionsReportForClient(client ProjectTransitionsClient, fetchedAt time.Time) (ProjectTransitionsReport, error) {
	snapshot, err := client.Snapshot()
	if err != nil {
		return ProjectTransitionsReport{}, err
	}
	return BuildProjectTransitionsReport(client.Repo().FullName(), snapshot, fetchedAt)
}

func BuildProjectTransitionsReport(repo string, snapshot ProjectTransitionSnapshot, fetchedAt time.Time) (ProjectTransitionsReport, error) {
	report := ProjectTransitionsReport{
		Repo:      repo,
		Project:   ProductOSTransitionsProjectName,
		Command:   "project transitions",
		DryRun:    true,
		FetchedAt: formatGitHubTime(fetchedAt),
	}

	issueCandidates := map[string][]projectTransitionCandidate{}
	skips := make([]ProjectTransitionPlanItem, 0)

	linkedPRs := mapPRsToIssues(snapshot.PullRequests)
	closingLinkedPRs := mapPRsToIssuesByClosingKeywords(snapshot.PullRequests)
	branchIssues := mapBranchesToIssues(snapshot.Branches)

	sort.Slice(snapshot.Issues, func(i, j int) bool { return snapshot.Issues[i].Number < snapshot.Issues[j].Number })
	for _, issue := range snapshot.Issues {
		itemKey := transitionItemKey("issue", issue.Number)
		status := inferIssueStatus(issue)
		blocked := hasBlockedLabel(issue.Labels)
		prs := linkedPRs[issue.Number]
		nonDraftPR, hasNonDraftPR := firstLinkedPR(prs, false)
		draftPR, hasDraftPR := firstLinkedPR(prs, true)

		if blocked && status != "Blocked" {
			issueCandidates[itemKey] = append(issueCandidates[itemKey], makeCandidate(
				"blocked_added", "issue", issue.Number, displayStatus(status), "Blocked", "blocked label present", "blocked_added wins over workflow states", 1,
			))
		}
		if !blocked && status == "Blocked" {
			targetState, reason, resolution := blockedRemovalTarget(issue, prs, branchIssues[issue.Number])
			issueCandidates[itemKey] = append(issueCandidates[itemKey], makeCandidate(
				"blocked_removed", "issue", issue.Number, "Blocked", targetState, reason, resolution, 1,
			))
		}

		if mergedPR, ok := firstMergedLinkedPR(closingLinkedPRs[issue.Number]); ok {
			if strings.EqualFold(issue.State, "closed") {
				if status != "Done" {
					issueCandidates[itemKey] = append(issueCandidates[itemKey], makeCandidate(
						"pr_merged_closes_issue", "issue", issue.Number, displayStatus(status), "Done", fmt.Sprintf("merged PR #%d references closing keyword", mergedPR.Number), "github_closure_observed", 2,
					))
				}
			} else {
				skips = append(skips, ProjectTransitionPlanItem{
					RuleID:             "pr_merged_closes_issue",
					TargetType:         "issue",
					TargetID:           strconv.Itoa(issue.Number),
					From:               displayStatus(status),
					To:                 displayStatus(status),
					Reason:             fmt.Sprintf("merged PR #%d references issue but issue is still open", mergedPR.Number),
					ConflictResolution: "closure_missing",
					Decision:           "skip",
				})
			}
		}

		if hasNonDraftPR {
			if status == "In progress" {
				issueCandidates[itemKey] = append(issueCandidates[itemKey], makeCandidate(
					"pr_ready_for_review", "issue", issue.Number, "In progress", "In review", fmt.Sprintf("linked non-draft PR #%d is reviewable", nonDraftPR.Number), "review_state_projection", 3,
				))
			}
			if status == "Ready" {
				issueCandidates[itemKey] = append(issueCandidates[itemKey], makeCandidate(
					"pr_opened", "issue", issue.Number, "Ready", "In review", fmt.Sprintf("linked non-draft PR #%d references issue", nonDraftPR.Number), "review_state_projection", 3,
				))
			}
			if status == "Blocked" || blocked {
				skips = append(skips, ProjectTransitionPlanItem{
					RuleID:             "pr_ready_for_review",
					TargetType:         "issue",
					TargetID:           strconv.Itoa(issue.Number),
					From:               displayStatus(status),
					To:                 "In review",
					Reason:             fmt.Sprintf("linked non-draft PR #%d is reviewable but issue is blocked", nonDraftPR.Number),
					ConflictResolution: "blocked_overrides_review",
					Decision:           "skip",
				})
			}
		} else if hasDraftPR && (status == "Ready" || status == "In progress") {
			skips = append(skips, ProjectTransitionPlanItem{
				RuleID:             "pr_opened",
				TargetType:         "issue",
				TargetID:           strconv.Itoa(issue.Number),
				From:               displayStatus(status),
				To:                 "In review",
				Reason:             fmt.Sprintf("linked PR #%d is draft", draftPR.Number),
				ConflictResolution: "draft_pr",
				Decision:           "skip",
			})
		}

		if _, linked := branchIssues[issue.Number]; linked && status == "Ready" {
			issueCandidates[itemKey] = append(issueCandidates[itemKey], makeCandidate(
				"branch_started", "issue", issue.Number, "Ready", "In progress", "branch linkage resolves to issue", "branch_started unless review state exists", 4,
			))
		}

		if status == "Backlog" && triageCriteriaMet(issue) {
			issueCandidates[itemKey] = append(issueCandidates[itemKey], makeCandidate(
				"issue_triaged_ready", "issue", issue.Number, "Backlog", "Ready", "required triage labels/criteria satisfied", "triage_ready_projection", 5,
			))
		}

		if strings.EqualFold(issue.State, "open") && status == "" {
			issueCandidates[itemKey] = append(issueCandidates[itemKey], makeCandidate(
				"issue_open_default", "issue", issue.Number, "null", "Backlog", "issue has no managed status", "default_status_projection", 5,
			))
		}

		if isReleaseChecklistIssue(issue) {
			allDone, hasChecklist := checklistCompletion(issue.Body)
			if hasChecklist && allDone && status != "Done" {
				issueCandidates[itemKey] = append(issueCandidates[itemKey], makeCandidate(
					"release_checklist_complete", "issue", issue.Number, displayStatus(status), "Done", "all release checklist items are complete", "release_gate_projection", 6,
				))
			}
			if hasChecklist && !allDone && status == "Done" {
				issueCandidates[itemKey] = append(issueCandidates[itemKey], makeCandidate(
					"release_checklist_complete", "issue", issue.Number, "Done", "In progress", "release checklist contains unchecked required items", "checklist_reopened", 6,
				))
			}
		}
	}

	for _, milestone := range snapshot.Milestones {
		if milestone.ClosedIssues > 0 && milestone.OpenIssues == 0 {
			key := transitionItemKey("milestone", milestone.Number)
			issueCandidates[key] = append(issueCandidates[key], makeCandidate(
				"milestone_all_closed", "milestone", milestone.Number, "Milestone phase", "Complete", "all milestone issues are closed", "computed_aggregate_state", 6,
			))
		}
	}

	applies := make([]ProjectTransitionPlanItem, 0)
	conflicts := 0
	for key := range issueCandidates {
		candidates := issueCandidates[key]
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].Precedence == candidates[j].Precedence {
				return candidates[i].Item.RuleID < candidates[j].Item.RuleID
			}
			return candidates[i].Precedence < candidates[j].Precedence
		})
		winner := candidates[0].Item
		winner.Decision = "apply"
		applies = append(applies, winner)
		for _, loser := range candidates[1:] {
			skips = append(skips, ProjectTransitionPlanItem{
				RuleID:             loser.Item.RuleID,
				TargetType:         loser.Item.TargetType,
				TargetID:           loser.Item.TargetID,
				From:               loser.Item.From,
				To:                 loser.Item.To,
				Reason:             loser.Item.Reason,
				ConflictResolution: "overridden_by:" + winner.RuleID,
				Decision:           "skip",
			})
			conflicts++
		}
	}

	sortTransitions(applies)
	sortTransitions(skips)
	report.Transitions = append(report.Transitions, applies...)
	report.Transitions = append(report.Transitions, skips...)
	report.Counts.Applied = len(applies)
	report.Counts.Skipped = len(skips)
	report.Counts.Conflicts = conflicts
	report.Counts.Evaluated = report.Counts.Applied + report.Counts.Skipped
	return report, nil
}

func FormatProjectTransitionsPlan(report ProjectTransitionsReport) string {
	var b strings.Builder
	b.WriteString("project transitions plan:\n")
	fmt.Fprintf(&b, "repo:             %s\n", report.Repo)
	fmt.Fprintf(&b, "project:          %s\n", report.Project)
	fmt.Fprintf(&b, "dry_run:          %t\n", report.DryRun)
	fmt.Fprintf(&b, "transitions:      %d apply, %d skip, %d conflict\n", report.Counts.Applied, report.Counts.Skipped, report.Counts.Conflicts)
	for _, transition := range report.Transitions {
		target := transition.TargetType + "#" + transition.TargetID
		if transition.Decision == "apply" {
			fmt.Fprintf(&b, "  apply %s %s: %s -> %s (%s)\n", target, transition.RuleID, transition.From, transition.To, transition.Reason)
			continue
		}
		if transition.ConflictResolution != "" {
			fmt.Fprintf(&b, "  skip %s %s: %s (%s)\n", target, transition.RuleID, transition.ConflictResolution, transition.Reason)
		} else {
			fmt.Fprintf(&b, "  skip %s %s: %s\n", target, transition.RuleID, transition.Reason)
		}
	}
	return b.String()
}

type projectTransitionCandidate struct {
	Item       ProjectTransitionPlanItem
	Precedence int
}

func makeCandidate(ruleID, targetType string, targetID int, from, to, reason, conflictResolution string, precedence int) projectTransitionCandidate {
	return projectTransitionCandidate{
		Item: ProjectTransitionPlanItem{
			RuleID:             ruleID,
			TargetType:         targetType,
			TargetID:           strconv.Itoa(targetID),
			From:               from,
			To:                 to,
			Reason:             reason,
			ConflictResolution: conflictResolution,
			Decision:           "apply",
		},
		Precedence: precedence,
	}
}

func transitionItemKey(targetType string, id int) string {
	return targetType + "#" + strconv.Itoa(id)
}

func sortTransitions(items []ProjectTransitionPlanItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].TargetType != items[j].TargetType {
			return items[i].TargetType < items[j].TargetType
		}
		leftID, _ := strconv.Atoi(items[i].TargetID)
		rightID, _ := strconv.Atoi(items[j].TargetID)
		if leftID != rightID {
			return leftID < rightID
		}
		if items[i].Decision != items[j].Decision {
			return items[i].Decision < items[j].Decision
		}
		return items[i].RuleID < items[j].RuleID
	})
}

func mapPRsToIssues(pulls []ProjectTransitionPullRequest) map[int][]ProjectTransitionPullRequest {
	mapped := map[int][]ProjectTransitionPullRequest{}
	for _, pr := range pulls {
		ids := parseClosingIssueNumbers(pr.Body)
		if len(ids) == 0 {
			ids = parseIssueNumbersFromRef(pr.HeadRef)
		}
		for _, id := range ids {
			mapped[id] = append(mapped[id], pr)
		}
	}
	for id := range mapped {
		sort.SliceStable(mapped[id], func(i, j int) bool {
			return mapped[id][i].Number < mapped[id][j].Number
		})
	}
	return mapped
}

func mapPRsToIssuesByClosingKeywords(pulls []ProjectTransitionPullRequest) map[int][]ProjectTransitionPullRequest {
	mapped := map[int][]ProjectTransitionPullRequest{}
	for _, pr := range pulls {
		for _, id := range parseClosingIssueNumbers(pr.Body) {
			mapped[id] = append(mapped[id], pr)
		}
	}
	for id := range mapped {
		sort.SliceStable(mapped[id], func(i, j int) bool {
			return mapped[id][i].Number < mapped[id][j].Number
		})
	}
	return mapped
}

func mapBranchesToIssues(branches []string) map[int]bool {
	mapped := map[int]bool{}
	for _, branch := range branches {
		for _, id := range parseIssueNumbersFromRef(branch) {
			mapped[id] = true
		}
	}
	return mapped
}

func firstLinkedPR(prs []ProjectTransitionPullRequest, draft bool) (ProjectTransitionPullRequest, bool) {
	for _, pr := range prs {
		if pr.Draft == draft {
			return pr, true
		}
	}
	return ProjectTransitionPullRequest{}, false
}

func firstMergedLinkedPR(prs []ProjectTransitionPullRequest) (ProjectTransitionPullRequest, bool) {
	for _, pr := range prs {
		if pr.MergedAt != nil {
			return pr, true
		}
	}
	return ProjectTransitionPullRequest{}, false
}

func inferIssueStatus(issue ProjectTransitionIssue) string {
	status := managedStatusFromLabels(issue.Labels)
	if status != "" {
		return status
	}
	if strings.EqualFold(issue.State, "closed") {
		return "Done"
	}
	return ""
}

func blockedRemovalTarget(issue ProjectTransitionIssue, prs []ProjectTransitionPullRequest, branchLinked bool) (string, string, string) {
	if previous, ok := previousNonBlockedStatus(issue.Labels); ok {
		return previous, fmt.Sprintf("blocked label removed; restored previous status %s", previous), "restored_previous_state"
	}
	if nonDraftPR, ok := firstLinkedPR(prs, false); ok {
		return "In review", fmt.Sprintf("blocked label removed; previous state unavailable, recomputed from linked non-draft PR #%d", nonDraftPR.Number), "missing_previous_state_recomputed"
	}
	if branchLinked {
		return "In progress", "blocked label removed; previous state unavailable, recomputed from linked branch", "missing_previous_state_recomputed"
	}
	return "Ready", "blocked label removed; previous state unavailable", "missing_previous_state"
}

func previousNonBlockedStatus(labels []string) (string, bool) {
	best := ""
	bestScore := -1
	for _, label := range labels {
		status, ok := mapStatusLabel(label)
		if !ok || status == "Blocked" {
			continue
		}
		score := statusPriority(status)
		if score > bestScore {
			best = status
			bestScore = score
		}
	}
	if best == "" {
		return "", false
	}
	return best, true
}

func managedStatusFromLabels(labels []string) string {
	best := ""
	bestScore := -1
	for _, label := range labels {
		status, ok := mapStatusLabel(label)
		if !ok {
			continue
		}
		score := statusPriority(status)
		if score > bestScore {
			bestScore = score
			best = status
		}
	}
	return best
}

func mapStatusLabel(label string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "status:backlog":
		return "Backlog", true
	case "status:ready":
		return "Ready", true
	case "status:blocked":
		return "Blocked", true
	case "status:in-progress", "status:in_progress":
		return "In progress", true
	case "status:in-review", "status:in_review":
		return "In review", true
	case "status:done":
		return "Done", true
	default:
		return "", false
	}
}

func statusPriority(status string) int {
	switch status {
	case "Done":
		return 6
	case "In review":
		return 5
	case "In progress":
		return 4
	case "Blocked":
		return 3
	case "Ready":
		return 2
	case "Backlog":
		return 1
	default:
		return 0
	}
}

func hasBlockedLabel(labels []string) bool {
	for _, label := range labels {
		normalized := strings.ToLower(strings.TrimSpace(label))
		if normalized == "blocked" {
			return true
		}
	}
	return false
}

func triageCriteriaMet(issue ProjectTransitionIssue) bool {
	hasType := false
	hasPriority := false
	hasAgent := false
	for _, label := range issue.Labels {
		normalized := strings.ToLower(strings.TrimSpace(label))
		if strings.HasPrefix(normalized, "type:") {
			hasType = true
		}
		if strings.HasPrefix(normalized, "priority:") {
			hasPriority = true
		}
		if strings.HasPrefix(normalized, "agent:") {
			hasAgent = true
		}
	}
	return hasType && hasPriority && hasAgent && strings.TrimSpace(issue.Body) != ""
}

func displayStatus(status string) string {
	if strings.TrimSpace(status) == "" {
		return "null"
	}
	return status
}

func isReleaseChecklistIssue(issue ProjectTransitionIssue) bool {
	title := strings.ToLower(issue.Title)
	if strings.Contains(title, "release checklist") {
		return true
	}
	for _, label := range issue.Labels {
		if strings.EqualFold(label, "type:release") || strings.EqualFold(label, "release") {
			return true
		}
	}
	return false
}

var checklistItemPattern = regexp.MustCompile(`(?m)^\s*-\s*\[( |x|X)\]\s+.+$`)
var checklistCheckedPattern = regexp.MustCompile(`(?m)^\s*-\s*\[(x|X)\]\s+.+$`)

func checklistCompletion(body string) (allDone bool, hasChecklist bool) {
	items := checklistItemPattern.FindAllString(body, -1)
	if len(items) == 0 {
		return false, false
	}
	checked := checklistCheckedPattern.FindAllString(body, -1)
	return len(items) == len(checked), true
}

var issueRefPattern = regexp.MustCompile(`(?i)(?:issue[-_/]|#)(\d+)`)
var closingRefPattern = regexp.MustCompile(`(?i)\b(?:close|closes|closed|fix|fixes|fixed|resolve|resolves|resolved)\s+#(\d+)`)

func parseIssueNumbersFromRef(value string) []int {
	matches := issueRefPattern.FindAllStringSubmatch(value, -1)
	if len(matches) == 0 {
		return nil
	}
	ids := make([]int, 0, len(matches))
	seen := map[int]bool{}
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		id, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	sort.Ints(ids)
	return ids
}

func parseClosingIssueNumbers(body string) []int {
	matches := closingRefPattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}
	ids := make([]int, 0, len(matches))
	seen := map[int]bool{}
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		id, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	sort.Ints(ids)
	return ids
}
