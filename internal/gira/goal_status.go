package gira

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const GoalStatusSchemaVersion = "goal-status/v2"

type GoalStatusInput struct {
	Repo RepoRef `json:"repo"`
	Goal int     `json:"goal"`
}

type GoalStatusReport struct {
	Command                      string              `json:"command"`
	SchemaVersion                string              `json:"schema_version"`
	Repo                         string              `json:"repo"`
	Goal                         GoalStatusIssue     `json:"goal"`
	Children                     []GoalStatusChild   `json:"children"`
	DiscoveryComplete            bool                `json:"discovery_complete"`
	StatusComplete               bool                `json:"status_complete"`
	Counts                       map[string]int      `json:"counts"`
	Blockers                     []string            `json:"blockers,omitempty"`
	AcquisitionFailures          []GoalStatusFailure `json:"acquisition_failures,omitempty"`
	HandoffReceiptPresent        bool                `json:"handoff_receipt_present"`
	PlanningEngine               string              `json:"planning_engine"`
	NextAction                   string              `json:"next_action"`
	NextStep                     string              `json:"next_step"`
	KnownRemainingAutonomousWork int                 `json:"known_remaining_autonomous_work"`
	RemainingAutonomousWork      *int                `json:"remaining_autonomous_work"`
}

type GoalStatusIssue struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	State     string   `json:"state"`
	Status    string   `json:"status"`
	Labels    []string `json:"labels,omitempty"`
	Milestone string   `json:"milestone,omitempty"`
	URL       string   `json:"url,omitempty"`
}

type GoalStatusChild struct {
	Repo            string   `json:"repo,omitempty"`
	Number          int      `json:"number"`
	Title           string   `json:"title"`
	State           string   `json:"state"`
	Status          string   `json:"status"`
	Category        string   `json:"category"`
	StatusAvailable bool     `json:"status_available"`
	RelationSource  string   `json:"relation_source"`
	Labels          []string `json:"labels,omitempty"`
	PRNumber        int      `json:"pr_number,omitempty"`
	PRURL           string   `json:"pr_url,omitempty"`
	PRState         string   `json:"pr_state,omitempty"`
	ChecksStatus    string   `json:"checks_status,omitempty"`
	ReviewStatus    string   `json:"review_status,omitempty"`
	Blockers        []string `json:"blockers,omitempty"`
	NextAction      string   `json:"next_action"`
	NextStep        string   `json:"next_step"`
	URL             string   `json:"url,omitempty"`
}

func BuildGoalStatusReport(input GoalStatusInput, runner CommandRunner) (GoalStatusReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	goalNumber, _, err := ResolveGoalNumber(input.Repo, input.Goal, runner)
	if err != nil {
		return GoalStatusReport{}, err
	}
	goal, err := fetchDevIssue(input.Repo, goalNumber, runner)
	if err != nil {
		return GoalStatusReport{}, err
	}
	if goal.IsPR {
		return GoalStatusReport{}, fmt.Errorf("goal #%d resolves to a pull request", goalNumber)
	}
	status := displayStatus(managedStatusFromLabels(goal.Labels))
	report := GoalStatusReport{
		Command:       "goal status",
		SchemaVersion: GoalStatusSchemaVersion,
		Repo:          input.Repo.FullName(),
		Goal: GoalStatusIssue{
			Number:    goal.Number,
			Title:     goal.Title,
			State:     goal.State,
			Status:    status,
			Labels:    append([]string(nil), goal.Labels...),
			Milestone: goal.Milestone,
			URL:       githubIssueURL(input.Repo, goal.Number),
		},
		Counts: map[string]int{},
	}
	report.HandoffReceiptPresent = goalFinishGoalReceiptPresent(input.Repo, goal.Number, runner)
	report.PlanningEngine = goalPlanningEngine(goal.Body)
	discovery := discoverGoalChildRefs(input.Repo, goal, runner)
	childRefs := discovery.Refs
	report.DiscoveryComplete = discovery.Complete
	report.AcquisitionFailures = append(report.AcquisitionFailures, discovery.Failures...)
	if !discovery.Complete {
		report.Blockers = appendUniqueStrings(report.Blockers, "child_discovery_incomplete")
	}
	// Child status is read from a repository-scoped snapshot when possible.
	// The snapshot is deliberately local to this invocation. Incomplete or
	// unavailable snapshots remain visible as child status blockers.
	snapshots := map[string]goalStatusRepositorySnapshot{}
	policies := map[string]goalStatusRepositoryPolicies{}
	reviewPolicies := goalStatusReviewPolicyCache{}
	operationPolicyUnavailable := map[string]bool{}
	childNumbersByRepo := goalStatusSnapshotNumbers(childRefs)
	type groupedChildFailure struct {
		repository string
		source     string
		stage      string
		code       string
		refs       []goalChildRef
	}
	groupedChildFailures := map[string]*groupedChildFailure{}
	addChildFailure := func(repository, source, stage, code string, ref goalChildRef) {
		key := strings.Join([]string{repository, source, stage, code}, "\x00")
		group := groupedChildFailures[key]
		if group == nil {
			group = &groupedChildFailure{repository: repository, source: source, stage: stage, code: code}
			groupedChildFailures[key] = group
		}
		group.refs = append(group.refs, ref)
	}
	repoNames := make([]string, 0, len(childNumbersByRepo))
	for repoName := range childNumbersByRepo {
		repoNames = append(repoNames, repoName)
	}
	sort.Strings(repoNames)
	for _, repoName := range repoNames {
		childNumbers := childNumbersByRepo[repoName]
		childRepo := goalStatusSnapshotRepoRef(childRefs, repoName)
		snapshot, _, snapshotErr := goalStatusRepositorySnapshotFor(childRepo, childNumbers, runner)
		if snapshotErr != nil {
			stage, code := goalStatusFailureClass(snapshotErr)
			report.AcquisitionFailures = append(report.AcquisitionFailures, goalStatusFailure(repoName, "repository_snapshot", stage, code, goalStatusRefsForRepo(childRefs, repoName), true))
			continue
		}
		snapshots[repoName] = snapshot
		operationPolicy, operationErr := ResolveRepoOperationPolicy(childRepo, runner)
		if operationErr != nil {
			operationPolicyUnavailable[repoName] = true
			report.AcquisitionFailures = append(report.AcquisitionFailures, goalStatusFailure(repoName, "operation_policy", goalStatusFailureStageOperationPolicy, "policy_unavailable", goalStatusRefsForRepo(childRefs, repoName), true))
			continue
		}
		policies[repoName] = goalStatusRepositoryPolicies{
			Branch:    loadGoalStatusBranchPolicy(childRepo, runner),
			Operation: operationPolicy,
		}
	}
	for _, childRef := range childRefs {
		var child GoalStatusChild
		repoName := childRef.Repo.FullName()
		snapshot, snapshotAvailable := snapshots[repoName]
		var descriptor *devStartIssue
		if snapshotAvailable {
			if issue, ok := snapshot.Issues[childRef.Number]; ok {
				descriptor = &issue
			}
		}
		statusAvailable := snapshotAvailable && !operationPolicyUnavailable[repoName]
		var childErr error
		if statusAvailable {
			child, childErr = goalStatusChildFromSnapshot(childRef, snapshot, policies[repoName], reviewPolicies, runner)
		}
		childBlockerKey := goalStatusChildBlockerKey(input.Repo, childRef.Repo, childRef.Number)
		if !statusAvailable || childErr != nil {
			child = goalStatusUnavailableChild(childRef, descriptor)
			report.Blockers = appendUniqueStrings(report.Blockers, childBlockerKey+"_status_unavailable")
			if childErr != nil {
				stage, code := goalStatusFailureClass(childErr)
				addChildFailure(repoName, "child_status", stage, code, childRef)
			}
		}
		report.Children = append(report.Children, child)
		if child.StatusAvailable && goalStatusKnownCategory(child.Category) {
			report.Counts["known"]++
			report.Counts[child.Category]++
		} else {
			report.Counts["unknown"]++
			if child.StatusAvailable {
				report.Blockers = appendUniqueStrings(report.Blockers, childBlockerKey+":unknown_status")
				addChildFailure(repoName, "child_status", goalStatusFailureStageChildStatus, "unknown_category", childRef)
			}
		}
		if child.Category == "blocked" && len(child.Blockers) == 0 {
			report.Blockers = appendUniqueStrings(report.Blockers, childBlockerKey+":blocked")
		} else if len(child.Blockers) > 0 && child.Category != "done" && child.Category != "closed_other" {
			report.Blockers = appendUniqueStrings(report.Blockers, fmt.Sprintf("%s:%s", childBlockerKey, strings.Join(child.Blockers, ",")))
		}
	}
	for _, group := range groupedChildFailures {
		report.AcquisitionFailures = append(report.AcquisitionFailures, goalStatusFailure(group.repository, group.source, group.stage, group.code, group.refs, true))
	}
	sort.Slice(report.Children, func(i, j int) bool {
		if report.Children[i].Repo != report.Children[j].Repo {
			return report.Children[i].Repo < report.Children[j].Repo
		}
		return report.Children[i].Number < report.Children[j].Number
	})
	report.Counts["total"] = len(childRefs)
	report.StatusComplete = report.Counts["unknown"] == 0
	report.KnownRemainingAutonomousWork = goalKnownRemainingAutonomousWork(report.Children)
	if report.DiscoveryComplete && report.StatusComplete {
		remaining := report.KnownRemainingAutonomousWork
		report.RemainingAutonomousWork = &remaining
	}
	goalStatusSortFailures(report.AcquisitionFailures)
	report.NextAction, report.NextStep = goalStatusNextAction(input.Repo, report)
	return report, nil
}

func goalPlanningEngine(body string) string {
	hasBulletPlan := goalPlanningHasBulletPlan(body)
	typedGraphState := goalPlanningTypedWorkGraphState(body)
	switch {
	case hasBulletPlan && typedGraphState == "valid":
		return "mixed"
	case hasBulletPlan && typedGraphState == "invalid":
		return "mixed_invalid_typed_work_graph"
	case typedGraphState == "valid":
		return "typed_work_graph"
	case typedGraphState == "invalid":
		return "invalid_typed_work_graph"
	case hasBulletPlan:
		return "bullet_goal_plan"
	default:
		return "unconfigured"
	}
}

func goalPlanningHasBulletPlan(body string) bool {
	for _, heading := range []string{"Goal Plan", "Decomposition", "Child Ticket Plan", "Suggested Child Tickets", "Initial Follow-Up Issues To Create", "Follow-Up Issues"} {
		for _, item := range markdownListSection(body, heading) {
			if !emptyReadinessSection(cleanGoalPlanItem(item)) {
				return true
			}
		}
	}
	return false
}

func goalPlanningTypedWorkGraphState(body string) string {
	section, present := goalWorkGraphSection(body)
	if !present {
		return "absent"
	}
	if strings.TrimSpace(section) == "" {
		return "invalid"
	}
	if emptyReadinessSection(section) {
		return "absent"
	}
	source, err := parsePMWorkGraphSource(body)
	if err != nil {
		return "invalid"
	}
	for _, node := range source.Nodes {
		if emptyReadinessSection(node.ID) || emptyReadinessSection(node.Title) || emptyReadinessSection(node.Purpose) || emptyReadinessSection(node.ParentOutcome) {
			return "invalid"
		}
		if _, ok := FindPMTaskProfile(node.Profile); !ok || len(node.Verification) == 0 {
			return "invalid"
		}
		for _, verification := range node.Verification {
			if emptyReadinessSection(verification.Method) || emptyReadinessSection(verification.Evidence) {
				return "invalid"
			}
		}
	}
	return "valid"
}

type goalChildRef struct {
	Repo           RepoRef
	Number         int
	RelationSource string
}

type goalChildDiscovery struct {
	Refs     []goalChildRef
	Complete bool
	Failures []GoalStatusFailure
}

const (
	GoalChildRelationSourceGitHubSubIssue    = "github_sub_issue"
	GoalChildRelationSourceGiraGoalChildLink = "gira_goal_child_link"
)

func discoverGoalChildRefs(repo RepoRef, goal devStartIssue, runner CommandRunner) goalChildDiscovery {
	refs := map[string]goalChildRef{}
	type discoveryFailure struct {
		source string
		code   string
	}
	failures := []discoveryFailure{}
	if nativeChildren, err := listGitHubSubIssues(repo, goal.Number, runner); err == nil {
		if len(nativeChildren) >= 100 {
			failures = append(failures, discoveryFailure{source: "github_sub_issues", code: "source_limit"})
		}
		for _, child := range nativeChildren {
			if child.Number <= 0 || child.Number == goal.Number || child.PullRequest != nil {
				continue
			}
			goalChildRefAdd(refs, goalChildRef{Repo: repo, Number: child.Number, RelationSource: GoalChildRelationSourceGitHubSubIssue})
		}
	} else {
		code := "transport_unknown"
		if strings.HasPrefix(err.Error(), "parse sub-issues JSON:") {
			code = "invalid_json"
		}
		failures = append(failures, discoveryFailure{source: "github_sub_issues", code: code})
	}
	for _, ref := range goalChildRefsFromTypedLinks(goal.Body) {
		if ref.Repo.FullName() != repo.FullName() || ref.Number != goal.Number {
			goalChildRefAdd(refs, ref)
		}
	}
	commentRefs, commentErr := goalChildRefsFromComments(repo, goal.Number, runner)
	if commentErr != nil {
		_, code := goalStatusDiscoveryFailureCode(commentErr)
		failures = append(failures, discoveryFailure{source: "goal_comments", code: code})
	}
	for _, ref := range commentRefs {
		if ref.Repo.FullName() != repo.FullName() || ref.Number != goal.Number {
			goalChildRefAdd(refs, ref)
		}
	}
	outRefs := make([]goalChildRef, 0, len(refs))
	for _, ref := range refs {
		outRefs = append(outRefs, ref)
	}
	sort.Slice(outRefs, func(i, j int) bool {
		if outRefs[i].Repo.FullName() != outRefs[j].Repo.FullName() {
			return outRefs[i].Repo.FullName() < outRefs[j].Repo.FullName()
		}
		return outRefs[i].Number < outRefs[j].Number
	})
	discovery := goalChildDiscovery{Refs: outRefs, Complete: len(failures) == 0}
	for _, failure := range failures {
		stage, code := goalStatusFailureStageChildDiscovery, failure.code
		if code == "" {
			code = "source_unavailable"
		}
		affected := goalStatusRefsForRepo(outRefs, repo.FullName())
		discovery.Failures = append(discovery.Failures, goalStatusFailure(repo.FullName(), failure.source, stage, code, affected, false))
	}
	goalStatusSortFailures(discovery.Failures)
	return discovery
}

func goalChildRefAdd(refs map[string]goalChildRef, ref goalChildRef) {
	if ref.Number <= 0 || strings.TrimSpace(ref.RelationSource) == "" {
		return
	}
	key := goalChildRefKey(ref)
	existing, found := refs[key]
	if !found || (ref.RelationSource == GoalChildRelationSourceGitHubSubIssue && existing.RelationSource != GoalChildRelationSourceGitHubSubIssue) {
		refs[key] = ref
	}
}

func goalChildRefKey(ref goalChildRef) string {
	return ref.Repo.FullName() + "#" + strconv.Itoa(ref.Number)
}

func goalStatusChildBlockerKey(parentRepo RepoRef, childRepo RepoRef, number int) string {
	if childRepo.FullName() == parentRepo.FullName() {
		return fmt.Sprintf("child_%d", number)
	}
	return fmt.Sprintf("child_%s_%d", goalChildRefSlug(childRepo.FullName()), number)
}

func goalChildRefSlug(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func goalChildRefsFromComments(repo RepoRef, goal int, runner CommandRunner) ([]goalChildRef, error) {
	out, err := runner.Run("gh", "issue", "view", strconv.Itoa(goal), "--repo", repo.FullName(), "--json", "comments")
	if err != nil {
		return nil, newGoalStatusAcquisitionError(goalStatusFailureStageChildDiscovery, "transport_unknown", err)
	}
	var raw struct {
		Comments []struct {
			Body string `json:"body"`
		} `json:"comments"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, newGoalStatusAcquisitionError(goalStatusFailureStageChildDiscovery, "invalid_json", err)
	}
	refs := []goalChildRef{}
	for _, comment := range raw.Comments {
		refs = append(refs, goalChildRefsFromTypedLinks(comment.Body)...)
	}
	if len(raw.Comments) >= 100 {
		return refs, newGoalStatusAcquisitionError(goalStatusFailureStageChildDiscovery, "source_limit", nil)
	}
	return refs, nil
}

func goalChildRefsFromTypedLinks(value string) []goalChildRef {
	refs := []goalChildRef{}
	for _, match := range strings.Split(value, "<!--") {
		marker, _, found := strings.Cut(match, "-->")
		if !found {
			continue
		}
		fields := strings.Fields(marker)
		if len(fields) != 3 || fields[0] != "gira:goal-child-link/v1" {
			continue
		}
		repoText, repoFound := strings.CutPrefix(fields[1], "repo=")
		issueText, issueFound := strings.CutPrefix(fields[2], "issue=")
		if !repoFound || !issueFound {
			continue
		}
		repo, err := ParseRepoRef(repoText)
		if err != nil {
			continue
		}
		number, err := strconv.Atoi(issueText)
		if err != nil || number <= 0 {
			continue
		}
		refs = append(refs, goalChildRef{Repo: repo, Number: number, RelationSource: GoalChildRelationSourceGiraGoalChildLink})
	}
	return refs
}

func goalStatusChildFromWorkStatus(repo RepoRef, relationSource string, status WorkStatusResult) GoalStatusChild {
	category := goalChildCategory(status)
	blockers := goalRelevantChildBlockers(category, status)
	return GoalStatusChild{
		Repo:            repo.FullName(),
		Number:          status.Issue,
		Title:           status.Title,
		State:           status.State,
		Status:          status.Status,
		Category:        category,
		StatusAvailable: true,
		RelationSource:  relationSource,
		Labels:          append([]string(nil), status.Labels...),
		PRNumber:        status.PRNumber,
		PRURL:           status.PRURL,
		PRState:         status.PRState,
		ChecksStatus:    status.ChecksStatus,
		ReviewStatus:    status.ReviewStatus,
		Blockers:        blockers,
		NextAction:      status.NextAction,
		NextStep:        status.NextStep,
		URL:             githubIssueURL(repo, status.Issue),
	}
}

func goalRelevantChildBlockers(category string, status WorkStatusResult) []string {
	if category == "done" || category == "closed_other" {
		return []string{}
	}
	blockers := []string{}
	for _, blocker := range status.Blockers {
		if blocker == "missing_linked_pr" {
			if category == "in_review" {
				blockers = append(blockers, blocker)
			}
			continue
		}
		blockers = appendUniqueStrings(blockers, blocker)
	}
	return blockers
}

func goalChildCategory(status WorkStatusResult) string {
	display := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(status.Status), " ", "_"))
	if strings.EqualFold(status.State, "closed") {
		if display == "done" {
			return "done"
		}
		return "closed_other"
	}
	if containsString(status.Blockers, "blocked") || display == "blocked" || status.NextAction == "blocked" {
		return "blocked"
	}
	switch display {
	case "ready":
		return "ready"
	case "in_progress":
		return "in_progress"
	case "in_review":
		return "in_review"
	case "done":
		return "done"
	default:
		return "unknown"
	}
}

func goalStatusKnownCategory(category string) bool {
	switch category {
	case "ready", "in_progress", "in_review", "blocked", "done", "closed_other":
		return true
	default:
		return false
	}
}

func goalKnownRemainingAutonomousWork(children []GoalStatusChild) int {
	remaining := 0
	for _, child := range children {
		switch child.Category {
		case "ready", "in_progress", "in_review", "blocked":
			remaining++
		}
	}
	return remaining
}

func goalStatusNextAction(repo RepoRef, report GoalStatusReport) (string, string) {
	if !report.DiscoveryComplete || !report.StatusComplete {
		return "resolve_blockers", fmt.Sprintf("gira goal status --repo %s --goal %d --json", repo.FullName(), report.Goal.Number)
	}
	if goalStatusIssueDone(report.Goal) {
		return "done", "goal is done"
	}
	if report.Counts["total"] == 0 {
		return "plan_children", fmt.Sprintf("gira goal plan --repo %s --goal %d --dry-run", repo.FullName(), report.Goal.Number)
	}
	if len(report.Blockers) > 0 || report.Counts["blocked"] > 0 {
		return "resolve_blockers", fmt.Sprintf("gira goal status --repo %s --goal %d --json", repo.FullName(), report.Goal.Number)
	}
	if report.Counts["in_review"] > 0 {
		return "review_child", fmt.Sprintf("gira goal next --repo %s --goal %d --json", repo.FullName(), report.Goal.Number)
	}
	if report.Counts["in_progress"] > 0 {
		return "continue_child", fmt.Sprintf("gira goal next --repo %s --goal %d --json", repo.FullName(), report.Goal.Number)
	}
	if report.Counts["ready"] > 0 {
		return "start_next_child", fmt.Sprintf("gira goal next --repo %s --goal %d --json", repo.FullName(), report.Goal.Number)
	}
	if report.RemainingAutonomousWork != nil && *report.RemainingAutonomousWork == 0 {
		if report.HandoffReceiptPresent {
			return "human_review", fmt.Sprintf("review goal-finish-receipt/v1 handoff on #%d", report.Goal.Number)
		}
		return "finish_goal", fmt.Sprintf("gira goal finish --repo %s --goal %d --dry-run", repo.FullName(), report.Goal.Number)
	}
	return "inspect_goal", fmt.Sprintf("gira goal status --repo %s --goal %d --json", repo.FullName(), report.Goal.Number)
}

func goalStatusIssueDone(goal GoalStatusIssue) bool {
	return strings.EqualFold(goal.State, "closed") && strings.EqualFold(goal.Status, "Done")
}

func githubIssueURL(repo RepoRef, issue int) string {
	if issue <= 0 {
		return ""
	}
	return fmt.Sprintf("https://github.com/%s/issues/%d", repo.FullName(), issue)
}

func FormatGoalStatus(report GoalStatusReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "goal status: #%d %s children=%d known_remaining=%d remaining=%s discovery_complete=%t status_complete=%t planning_engine=%s next=%s\n", report.Goal.Number, report.Goal.Status, report.Counts["total"], report.KnownRemainingAutonomousWork, goalStatusRemainingText(report.RemainingAutonomousWork), report.DiscoveryComplete, report.StatusComplete, report.PlanningEngine, report.NextAction)
	if len(report.Children) > 0 {
		keys := []string{"known", "unknown", "ready", "in_progress", "in_review", "blocked", "done", "closed_other"}
		parts := []string{}
		for _, key := range keys {
			if count := report.Counts[key]; count > 0 {
				parts = append(parts, fmt.Sprintf("%s=%d", key, count))
			}
		}
		if len(parts) > 0 {
			fmt.Fprintf(&b, "children: %s\n", strings.Join(parts, " "))
		}
	}
	if len(report.Blockers) > 0 {
		fmt.Fprintf(&b, "blockers: %s\n", strings.Join(report.Blockers, ","))
	}
	for _, failure := range report.AcquisitionFailures {
		fmt.Fprintf(&b, "acquisition failure: %s\n", goalStatusFailureSummary(failure))
	}
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}

func goalStatusRemainingText(remaining *int) string {
	if remaining == nil {
		return "unknown"
	}
	return strconv.Itoa(*remaining)
}
