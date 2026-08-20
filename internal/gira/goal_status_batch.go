package gira

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// goalStatusRepositorySnapshot is intentionally scoped to one goal status
// invocation. It is not cached between commands: a report must always use a
// fresh, internally consistent view of each repository.
type goalStatusRepositorySnapshot struct {
	Issues            map[int]devStartIssue
	PRs               []prSummary
	PRsIncomplete     map[int]bool
	Reviews           map[int][]goalStatusGraphQLReview
	ReviewsIncomplete map[int]bool
}

type goalStatusRepositoryPolicies struct {
	Branch    *ResolvedBranchPolicy
	Review    FinishReviewPolicy
	Operation ResolvedOperationPolicy
}

// A single alias query also requests bounded timeline, review, and check
// connections.
// Keep the repository batch deliberately conservative so its GraphQL cost and
// response size stay bounded; larger goals fail closed rather than silently
// falling back to O(children) reads.
const goalStatusRepositoryChildLimit = 50

// goalStatusRepositorySnapshotFor reads the issue and PR metadata needed by
// all children in one repository. A complete snapshot is required before it
// can produce child statuses; partial API responses are surfaced as
// unavailable for every child; no per-ticket fallback can turn an API error
// into a false-ready result.
func goalStatusRepositorySnapshotFor(repo RepoRef, childNumbers []int, runner CommandRunner) (goalStatusRepositorySnapshot, bool, error) {
	issues, prs, incompletePRs, reviews, reviewsIncomplete, attempted, err := goalStatusIssueSnapshot(repo, childNumbers, runner)
	if err != nil {
		return goalStatusRepositorySnapshot{}, attempted, err
	}
	for _, number := range childNumbers {
		if _, ok := issues[number]; !ok {
			return goalStatusRepositorySnapshot{}, true, fmt.Errorf("issue #%d is missing from repository snapshot", number)
		}
	}
	return goalStatusRepositorySnapshot{Issues: issues, PRs: prs, PRsIncomplete: incompletePRs, Reviews: reviews, ReviewsIncomplete: reviewsIncomplete}, true, nil
}

func goalStatusIssueSnapshot(repo RepoRef, childNumbers []int, runner CommandRunner) (map[int]devStartIssue, []prSummary, map[int]bool, map[int][]goalStatusGraphQLReview, map[int]bool, bool, error) {
	numbers := append([]int(nil), childNumbers...)
	sort.Ints(numbers)
	if len(numbers) > goalStatusRepositoryChildLimit {
		return nil, nil, nil, nil, nil, true, fmt.Errorf("repository has %d goal children; snapshot limit is %d", len(numbers), goalStatusRepositoryChildLimit)
	}
	aliases := make([]string, 0, len(numbers))
	for _, number := range numbers {
		aliases = append(aliases, fmt.Sprintf("issue%d: issue(number: %d) { number title state body labels(first: 100) { nodes { name } pageInfo { hasNextPage } } milestone { title } timelineItems(first: 100, itemTypes: [CROSS_REFERENCED_EVENT]) { nodes { ... on CrossReferencedEvent { source { ... on PullRequest { number title body state url isDraft mergeStateStatus reviewDecision headRefName baseRefName headRefOid mergeCommit { oid } reviews(first: 100) { nodes { state commit { oid } } pageInfo { hasNextPage } } statusCheckRollup { contexts(first: 100) { nodes { ... on CheckRun { name status conclusion detailsUrl completedAt } ... on StatusContext { context state targetUrl description } } pageInfo { hasNextPage } } } } } } } pageInfo { hasNextPage } } }", number, number))
	}
	query := fmt.Sprintf("query($owner: String!, $name: String!) { repository(owner: $owner, name: $name) { %s } }", strings.Join(aliases, " "))
	out, err := runner.Run("gh", "api", "graphql", "-f", "owner="+repo.Owner, "-f", "name="+repo.Name, "-f", "query="+query)
	if err != nil {
		return nil, nil, nil, nil, nil, false, err
	}
	var payload struct {
		Data struct {
			Repository map[string]*goalStatusGraphQLIssue `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, nil, nil, nil, nil, true, fmt.Errorf("parse issue snapshot JSON: %w", err)
	}
	if len(payload.Errors) > 0 {
		return nil, nil, nil, nil, nil, true, fmt.Errorf("issue snapshot GraphQL error: %s", payload.Errors[0].Message)
	}
	issues := map[int]devStartIssue{}
	prs := []prSummary{}
	seenPRs := map[int]bool{}
	incompletePRs := map[int]bool{}
	reviews := map[int][]goalStatusGraphQLReview{}
	reviewsIncomplete := map[int]bool{}
	for _, number := range numbers {
		raw := payload.Data.Repository[fmt.Sprintf("issue%d", number)]
		if raw == nil || raw.Number <= 0 {
			continue
		}
		if raw.Labels.PageInfo.HasNextPage {
			return nil, nil, nil, nil, nil, true, fmt.Errorf("labels for issue #%d exceed the snapshot limit", raw.Number)
		}
		body := ""
		if raw.Body != nil {
			body = *raw.Body
		}
		labels := make([]string, 0, len(raw.Labels.Nodes))
		for _, label := range raw.Labels.Nodes {
			labels = append(labels, label.Name)
		}
		milestone := ""
		if raw.Milestone != nil {
			milestone = raw.Milestone.Title
		}
		issues[raw.Number] = devStartIssue{
			Number: raw.Number, Title: raw.Title, State: strings.ToLower(raw.State),
			Body: body, Labels: labels, Milestone: milestone,
		}
		if raw.TimelineItems.PageInfo.HasNextPage {
			incompletePRs[raw.Number] = true
		}
		for _, timeline := range raw.TimelineItems.Nodes {
			if timeline.Source == nil || timeline.Source.Number <= 0 {
				continue
			}
			prNumber := timeline.Source.Number
			if _, ok := reviews[prNumber]; !ok {
				reviews[prNumber] = append([]goalStatusGraphQLReview(nil), timeline.Source.Reviews.Nodes...)
			}
			if timeline.Source.Reviews.PageInfo.HasNextPage {
				reviewsIncomplete[prNumber] = true
			}
			if seenPRs[timeline.Source.Number] {
				continue
			}
			seenPRs[timeline.Source.Number] = true
			prs = append(prs, goalStatusGraphQLPRSummary(*timeline.Source))
		}
		if raw.TimelineItems.TotalCount >= 100 {
			incompletePRs[raw.Number] = true
		}
	}
	return issues, prs, incompletePRs, reviews, reviewsIncomplete, true, nil
}

type goalStatusGraphQLIssue struct {
	Number int     `json:"number"`
	Title  string  `json:"title"`
	State  string  `json:"state"`
	Body   *string `json:"body"`
	Labels struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
		PageInfo struct {
			HasNextPage bool `json:"hasNextPage"`
		} `json:"pageInfo"`
	} `json:"labels"`
	Milestone *struct {
		Title string `json:"title"`
	} `json:"milestone"`
	TimelineItems struct {
		TotalCount int `json:"totalCount"`
		PageInfo   struct {
			HasNextPage bool `json:"hasNextPage"`
		} `json:"pageInfo"`
		Nodes []struct {
			Source *goalStatusGraphQLPR `json:"source"`
		} `json:"nodes"`
	} `json:"timelineItems"`
}

type goalStatusGraphQLPR struct {
	Number         int    `json:"number"`
	Title          string `json:"title"`
	Body           string `json:"body"`
	State          string `json:"state"`
	URL            string `json:"url"`
	IsDraft        bool   `json:"isDraft"`
	MergeState     string `json:"mergeStateStatus"`
	ReviewDecision string `json:"reviewDecision"`
	HeadRefName    string `json:"headRefName"`
	BaseRefName    string `json:"baseRefName"`
	HeadRefOID     string `json:"headRefOid"`
	MergeCommit    *struct {
		OID string `json:"oid"`
	} `json:"mergeCommit"`
	Reviews struct {
		Nodes    []goalStatusGraphQLReview `json:"nodes"`
		PageInfo struct {
			HasNextPage bool `json:"hasNextPage"`
		} `json:"pageInfo"`
	} `json:"reviews"`
	StatusCheckRollup struct {
		Contexts struct {
			Nodes    []goalStatusGraphQLCheck `json:"nodes"`
			PageInfo struct {
				HasNextPage bool `json:"hasNextPage"`
			} `json:"pageInfo"`
		} `json:"contexts"`
	} `json:"statusCheckRollup"`
}

type goalStatusGraphQLReview struct {
	State  string `json:"state"`
	Commit *struct {
		OID string `json:"oid"`
	} `json:"commit"`
}

type goalStatusGraphQLCheck struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Conclusion  string `json:"conclusion"`
	DetailsURL  string `json:"detailsUrl"`
	CompletedAt string `json:"completedAt"`
	Context     string `json:"context"`
	State       string `json:"state"`
	TargetURL   string `json:"targetUrl"`
	Description string `json:"description"`
}

func goalStatusGraphQLPRSummary(pr goalStatusGraphQLPR) prSummary {
	state := strings.ToUpper(strings.TrimSpace(pr.State))
	if state == "CLOSED" && pr.MergeCommit != nil && strings.TrimSpace(pr.MergeCommit.OID) != "" {
		state = "MERGED"
	}
	summary := prSummary{
		Number: pr.Number, Title: pr.Title, Body: pr.Body, State: state,
		URL: pr.URL, ReviewDecision: strings.ToUpper(strings.TrimSpace(pr.ReviewDecision)),
		IsDraft: pr.IsDraft, MergeState: strings.ToUpper(strings.TrimSpace(pr.MergeState)),
		HeadRefName: pr.HeadRefName, BaseRefName: pr.BaseRefName, HeadRefOID: pr.HeadRefOID,
	}
	if pr.MergeCommit != nil {
		summary.MergeCommit = &struct {
			OID string `json:"oid"`
		}{OID: strings.TrimSpace(pr.MergeCommit.OID)}
	}
	for _, check := range pr.StatusCheckRollup.Contexts.Nodes {
		name, status, conclusion, url := check.Name, check.Status, check.Conclusion, check.DetailsURL
		if strings.TrimSpace(name) == "" {
			name = check.Context
		}
		if strings.TrimSpace(status) == "" && strings.TrimSpace(check.State) != "" {
			status, conclusion = restCommitStatusState(check.State)
		}
		if strings.TrimSpace(url) == "" {
			url = check.TargetURL
		}
		summary.StatusRollup = append(summary.StatusRollup, struct {
			Name       string `json:"name"`
			Workflow   string `json:"workflowName"`
			Conclusion string `json:"conclusion"`
			Status     string `json:"status"`
			URL        string `json:"detailsUrl"`
		}{Name: name, Conclusion: strings.ToUpper(conclusion), Status: strings.ToUpper(status), URL: url})
	}
	if pr.StatusCheckRollup.Contexts.PageInfo.HasNextPage {
		// A truncated rollup cannot establish readiness. Keep the unknown
		// sentinel in the existing check contract so normal fail-closed
		// blocker handling remains authoritative.
		summary.StatusRollup = append(summary.StatusRollup, struct {
			Name       string `json:"name"`
			Workflow   string `json:"workflowName"`
			Conclusion string `json:"conclusion"`
			Status     string `json:"status"`
			URL        string `json:"detailsUrl"`
		}{Name: "status_check_rollup_truncated"})
	}
	return summary
}

func goalStatusPRForIssue(repo RepoRef, issue devStartIssue, prs []prSummary, incomplete bool, repoPolicy *ResolvedBranchPolicy) DevPRStatusResult {
	candidates := make([]prSummary, 0)
	for _, pr := range prs {
		if hasClosingKeyword(pr.Body, issue.Number) {
			candidates = append(candidates, pr)
		}
	}
	policy := devPRBindingPolicyFromIssue(issue, nil, repo)
	if repoPolicy != nil && repoPolicy.Source == "config" && strings.TrimSpace(repoPolicy.FeatureBranchPattern) != "" {
		policy.ResolvedWorkBranch = formatDevBranch(repoPolicy.FeatureBranchPattern, issue.Number, issue.Title)
	}
	if len(candidates) == 0 {
		if incomplete {
			return DevPRStatusResult{Repo: repo.FullName(), Issue: issue.Number, Blockers: []string{"pr_snapshot_incomplete"}}
		}
		return DevPRStatusResult{Repo: repo.FullName(), Issue: issue.Number, Blockers: []string{"missing_linked_pr"}}
	}
	selected, ambiguity := selectClosingPRSummary(issue.Number, candidates, policy)
	if selected.Number == 0 {
		if ambiguity != nil {
			result := ambiguousDevPRStatus(repo, issue.Number, candidates[0], *ambiguity)
			if incomplete {
				result.Blockers = appendUniqueStrings(result.Blockers, "pr_snapshot_incomplete")
				result.Ready = false
			}
			return result
		}
		return DevPRStatusResult{Repo: repo.FullName(), Issue: issue.Number, Blockers: []string{"missing_linked_pr"}}
	}
	result := devPRStatusFromSummary(repo, issue.Number, selected, policy)
	if ambiguity != nil {
		result.Binding = *ambiguity
		result.Blockers = appendUniqueStrings(result.Blockers, ambiguity.Blockers...)
		result.Ready = false
	}
	if incomplete {
		result.Blockers = appendUniqueStrings(result.Blockers, "pr_snapshot_incomplete")
		result.Ready = false
	}
	return result
}

// loadGoalStatusBranchPolicy is best-effort metadata. A policy lookup failure does
// not make a child ready; closing references and the existing binding checks
// still determine the status. Snapshot failures remain unavailable and never
// re-enter a per-child fallback path.
func loadGoalStatusBranchPolicy(repo RepoRef, runner CommandRunner) *ResolvedBranchPolicy {
	policy, err := resolveRepoBranchPolicy(repo, runner)
	if err != nil {
		return nil
	}
	return &policy
}

func goalStatusSnapshotNumbers(refs []goalChildRef) map[string][]int {
	byRepo := map[string][]int{}
	for _, ref := range refs {
		key := ref.Repo.FullName()
		if !containsInt(byRepo[key], ref.Number) {
			byRepo[key] = append(byRepo[key], ref.Number)
		}
	}
	return byRepo
}

func containsInt(values []int, value int) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func goalStatusChildFromSnapshot(ref goalChildRef, snapshot goalStatusRepositorySnapshot, policies goalStatusRepositoryPolicies) (GoalStatusChild, error) {
	issue, ok := snapshot.Issues[ref.Number]
	if !ok {
		return GoalStatusChild{}, fmt.Errorf("issue #%d is missing from repository snapshot", ref.Number)
	}
	prStatus := goalStatusPRForIssue(ref.Repo, issue, snapshot.PRs, snapshot.PRsIncomplete[ref.Number], policies.Branch)
	review := goalStatusReviewEvidenceForPR(issue, prStatus, snapshot, policies.Review)
	workStatus := workStatusFromIssueAndPRWithPreparedReview(ref.Repo, issue.Number, issue, prStatus, policies.Operation, policies.Review, review)
	child := goalStatusChildFromWorkStatus(ref.Repo, ref.RelationSource, workStatus)
	return child, nil
}

func goalStatusReviewEvidenceForPR(issue devStartIssue, status DevPRStatusResult, snapshot goalStatusRepositorySnapshot, policy FinishReviewPolicy) *FinishReviewEvidence {
	if status.PRNumber == 0 {
		return nil
	}
	workStatus := displayStatus(managedStatusFromLabels(issue.Labels))
	nextAction := nextWorkAction(issue.State, workStatus, status)
	if !strings.EqualFold(workStatus, "In review") || (nextAction != "merge_when_policy_allows" && !containsString(status.Blockers, "review")) {
		return nil
	}
	evidence := FinishReviewEvidence{Decision: strings.ToUpper(strings.TrimSpace(status.ReviewDecision)), HeadSHA: strings.TrimSpace(status.HeadSHA)}
	if policy.Value == FinishReviewPolicyNone {
		evidence.Status = "not_required"
		return &evidence
	}
	if policy.Value == FinishReviewPolicyMissing {
		evidence.Status = "blocked"
		evidence.Blocker = "review_policy_not_configured"
		evidence.Remediation = "Set finish_review_policy: required or none in .gira/config.yaml."
		return &evidence
	}
	if evidence.Decision != "APPROVED" {
		evidence.Status = "blocked"
		evidence.Blocker = "review_required_but_absent"
		evidence.Remediation = "Request and record an approving review for the current PR head."
		return &evidence
	}
	if evidence.HeadSHA == "" || snapshot.ReviewsIncomplete[status.PRNumber] {
		evidence.Status = "blocked"
		evidence.Blocker = "review_evidence_unavailable"
		evidence.Remediation = "Restore readable GitHub review evidence and rerun ticket finish."
		return &evidence
	}
	for _, review := range snapshot.Reviews[status.PRNumber] {
		if strings.EqualFold(strings.TrimSpace(review.State), "APPROVED") && review.Commit != nil && strings.EqualFold(strings.TrimSpace(review.Commit.OID), evidence.HeadSHA) {
			evidence.Status = "approved"
			evidence.ApprovalSHA = strings.TrimSpace(review.Commit.OID)
			return &evidence
		}
	}
	evidence.Status = "blocked"
	evidence.Blocker = "review_approval_stale"
	evidence.Remediation = "Request a new approving review after the current PR head change."
	return &evidence
}

func goalStatusSnapshotRepoRef(refs []goalChildRef, fullName string) RepoRef {
	for _, ref := range refs {
		if ref.Repo.FullName() == fullName {
			return ref.Repo
		}
	}
	return RepoRef{}
}
