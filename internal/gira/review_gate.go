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

const (
	BlockerMissingApproval   = "missing_approval"
	BlockerFailingChecks     = "failing_checks"
	BlockerUnresolvedBlocker = "unresolved_blockers"
	BlockerPolicyViolation   = "policy_violation"

	ReviewFindingClassProvider = "provider_observation"
	ReviewFindingClassPolicy   = "managed_policy"
)

type ReviewPR struct {
	Number             int      `json:"number"`
	Title              string   `json:"title"`
	Body               string   `json:"body"`
	URL                string   `json:"url"`
	IsDraft            bool     `json:"is_draft"`
	ReviewDecision     string   `json:"review_decision"`
	CheckStatus        string   `json:"check_status"`
	Labels             []string `json:"labels"`
	RequestedReviewers []string `json:"requested_reviewers"`
	Assignees          []string `json:"assignees"`
	UpdatedAt          string   `json:"updated_at"`
}

type ReviewIssue struct {
	Number int      `json:"number"`
	Labels []string `json:"labels"`
}

type ReviewGateClient interface {
	Repo() RepoRef
	ListOpenPRs() ([]ReviewPR, error)
	ListOpenIssues() ([]ReviewIssue, error)
	MergePR(number int) error
}

// ReviewGateFinding separates facts supplied by GitHub from checks that only
// exist because a repository explicitly opted into managed delivery policy.
// The fields are additive so existing blockers consumers remain compatible.
type ReviewGateFinding struct {
	FindingClass string `json:"finding_class"`
	Code         string `json:"code"`
	Enforced     bool   `json:"enforced"`
	Message      string `json:"message"`
	Target       string `json:"target,omitempty"`
}

type GHReviewGateClient struct {
	repo   RepoRef
	runner CommandRunner
}

func (c GHReviewGateClient) ResolveOperationPolicy() (ResolvedOperationPolicy, error) {
	return ResolveRepoOperationPolicy(c.repo, c.runner)
}

func NewGHReviewGateClient(repo RepoRef, runner CommandRunner) GHReviewGateClient {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	return GHReviewGateClient{repo: repo, runner: runner}
}

func (c GHReviewGateClient) Repo() RepoRef { return c.repo }

func (c GHReviewGateClient) ListOpenPRs() ([]ReviewPR, error) {
	out, err := c.runner.Run("gh", "pr", "list", "--repo", c.repo.FullName(), "--state", "open", "--limit", "200", "--json", "number,title,body,url,isDraft,reviewDecision,statusCheckRollup,labels,reviewRequests,assignees,updatedAt")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Number         int    `json:"number"`
		Title          string `json:"title"`
		Body           string `json:"body"`
		URL            string `json:"url"`
		IsDraft        bool   `json:"isDraft"`
		ReviewDecision string `json:"reviewDecision"`
		StatusRollup   []struct {
			State string `json:"state"`
		} `json:"statusCheckRollup"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		ReviewRequests []struct {
			RequestedReviewer struct {
				Login string `json:"login"`
			} `json:"requestedReviewer"`
		} `json:"reviewRequests"`
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
		UpdatedAt string `json:"updatedAt"`
	}
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("parse pr list: %w", err)
	}
	prs := make([]ReviewPR, 0, len(rows))
	for _, row := range rows {
		labels := make([]string, 0, len(row.Labels))
		for _, l := range row.Labels {
			labels = append(labels, l.Name)
		}
		sort.Strings(labels)
		reviewers := make([]string, 0, len(row.ReviewRequests))
		for _, rr := range row.ReviewRequests {
			if rr.RequestedReviewer.Login != "" {
				reviewers = append(reviewers, rr.RequestedReviewer.Login)
			}
		}
		sort.Strings(reviewers)
		assignees := make([]string, 0, len(row.Assignees))
		for _, a := range row.Assignees {
			assignees = append(assignees, a.Login)
		}
		sort.Strings(assignees)
		check := "pending"
		if len(row.StatusRollup) == 0 {
			check = "none"
		} else {
			allPass := true
			anyFail := false
			for _, s := range row.StatusRollup {
				state := strings.ToUpper(strings.TrimSpace(s.State))
				if state == "FAILURE" || state == "ERROR" || state == "TIMED_OUT" || state == "CANCELLED" {
					anyFail = true
				}
				if state != "SUCCESS" && state != "NEUTRAL" && state != "SKIPPED" {
					allPass = false
				}
			}
			if anyFail {
				check = "failing"
			} else if allPass {
				check = "passing"
			}
		}
		prs = append(prs, ReviewPR{Number: row.Number, Title: row.Title, Body: row.Body, URL: row.URL, IsDraft: row.IsDraft, ReviewDecision: strings.ToUpper(row.ReviewDecision), CheckStatus: check, Labels: labels, RequestedReviewers: reviewers, Assignees: assignees, UpdatedAt: row.UpdatedAt})
	}
	sort.Slice(prs, func(i, j int) bool { return prs[i].Number < prs[j].Number })
	return prs, nil
}

func (c GHReviewGateClient) ListOpenIssues() ([]ReviewIssue, error) {
	out, err := c.runner.Run("gh", "issue", "list", "--repo", c.repo.FullName(), "--state", "open", "--limit", "200", "--json", "number,labels")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Number int `json:"number"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("parse issue list: %w", err)
	}
	issues := make([]ReviewIssue, 0, len(rows))
	for _, row := range rows {
		labels := make([]string, 0, len(row.Labels))
		for _, l := range row.Labels {
			labels = append(labels, l.Name)
		}
		issues = append(issues, ReviewIssue{Number: row.Number, Labels: labels})
	}
	return issues, nil
}

func (c GHReviewGateClient) MergePR(number int) error {
	_, err := c.runner.Run("gh", "pr", "merge", fmt.Sprintf("%d", number), "--repo", c.repo.FullName(), "--squash", "--delete-branch")
	return err
}

type ReviewQueueItem struct {
	PR           ReviewPR            `json:"pr"`
	RouteTo      string              `json:"route_to"`
	StaleReview  bool                `json:"stale_review"`
	Blockers     []string            `json:"blockers"`
	Findings     []ReviewGateFinding `json:"findings,omitempty"`
	QueueRank    int                 `json:"queue_rank"`
	RequeueToken string              `json:"requeue_token"`
	UpdatedTime  time.Time           `json:"-"`
}

type ReviewQueueReport struct {
	Repo      string                  `json:"repo"`
	Generated string                  `json:"generated_at"`
	Policy    ResolvedOperationPolicy `json:"policy"`
	Items     []ReviewQueueItem       `json:"items"`
}

type MergeQueueReport struct {
	Repo       string            `json:"repo"`
	Mode       string            `json:"mode"`
	Generated  string            `json:"generated_at"`
	Candidates []ReviewQueueItem `json:"candidates"`
	Merged     []int             `json:"merged"`
}

type ReleaseReadinessReport struct {
	Repo            string                  `json:"repo"`
	Generated       string                  `json:"generated_at"`
	Policy          ResolvedOperationPolicy `json:"policy"`
	Ready           bool                    `json:"ready"`
	BlockingPRs     []ReviewQueueItem       `json:"blocking_prs"`
	OpenBlockers    []int                   `json:"open_blocker_issues"`
	OpenMustFix     []int                   `json:"open_must_fix_issues"`
	BlockerTaxonomy []string                `json:"blocker_taxonomy"`
	Findings        []ReviewGateFinding     `json:"findings,omitempty"`
}

func BuildReviewQueue(client ReviewGateClient, now time.Time) (ReviewQueueReport, error) {
	policy, err := resolveReviewGatePolicy(client)
	if err != nil {
		return ReviewQueueReport{}, err
	}
	return buildReviewQueueWithPolicy(client, now, policy)
}

func buildReviewQueueWithPolicy(client ReviewGateClient, now time.Time, policy ResolvedOperationPolicy) (ReviewQueueReport, error) {
	prs, err := client.ListOpenPRs()
	if err != nil {
		return ReviewQueueReport{}, err
	}
	items := make([]ReviewQueueItem, 0, len(prs))
	for _, pr := range prs {
		updated, _ := time.Parse(time.RFC3339, pr.UpdatedAt)
		blockers, findings := classifyPRBlockersWithPolicy(pr, policy)
		route := "unassigned"
		if len(pr.RequestedReviewers) > 0 {
			route = pr.RequestedReviewers[0]
		} else if len(pr.Assignees) > 0 {
			route = pr.Assignees[0]
		}
		stale := false
		if !updated.IsZero() && updated.Before(now.Add(-72*time.Hour)) && contains(blockers, BlockerMissingApproval) {
			stale = true
		}
		items = append(items, ReviewQueueItem{PR: pr, RouteTo: route, StaleReview: stale, Blockers: blockers, Findings: findings, UpdatedTime: updated, RequeueToken: fmt.Sprintf("pr-%d:%s", pr.Number, updated.UTC().Format(time.RFC3339))})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].StaleReview != items[j].StaleReview {
			return items[i].StaleReview
		}
		if !items[i].UpdatedTime.Equal(items[j].UpdatedTime) {
			if items[i].UpdatedTime.IsZero() {
				return false
			}
			if items[j].UpdatedTime.IsZero() {
				return true
			}
			return items[i].UpdatedTime.Before(items[j].UpdatedTime)
		}
		return items[i].PR.Number < items[j].PR.Number
	})
	for i := range items {
		items[i].QueueRank = i + 1
	}
	return ReviewQueueReport{Repo: client.Repo().FullName(), Generated: now.UTC().Format(time.RFC3339), Policy: policy, Items: items}, nil
}

func BuildMergeQueue(client ReviewGateClient, now time.Time, apply bool) (MergeQueueReport, error) {
	queue, err := BuildReviewQueue(client, now)
	if err != nil {
		return MergeQueueReport{}, err
	}
	candidates := make([]ReviewQueueItem, 0)
	merged := make([]int, 0)
	for _, item := range queue.Items {
		if len(item.Blockers) > 0 {
			continue
		}
		candidates = append(candidates, item)
		if apply {
			if err := client.MergePR(item.PR.Number); err != nil {
				return MergeQueueReport{}, err
			}
			merged = append(merged, item.PR.Number)
		}
	}
	mode := "dry-run"
	if apply {
		mode = "apply"
	}
	return MergeQueueReport{Repo: client.Repo().FullName(), Mode: mode, Generated: now.UTC().Format(time.RFC3339), Candidates: candidates, Merged: merged}, nil
}

func BuildReleaseReadiness(client ReviewGateClient, now time.Time) (ReleaseReadinessReport, error) {
	policy, err := resolveReviewGatePolicy(client)
	if err != nil {
		return ReleaseReadinessReport{}, err
	}
	queue, err := buildReviewQueueWithPolicy(client, now, policy)
	if err != nil {
		return ReleaseReadinessReport{}, err
	}
	issues, err := client.ListOpenIssues()
	if err != nil {
		return ReleaseReadinessReport{}, err
	}
	blockers := make([]int, 0)
	mustFix := make([]int, 0)
	for _, issue := range issues {
		for _, label := range issue.Labels {
			lower := strings.ToLower(label)
			if strings.Contains(lower, "blocker") {
				blockers = append(blockers, issue.Number)
				break
			}
		}
		for _, label := range issue.Labels {
			lower := strings.ToLower(label)
			if strings.Contains(lower, "must-fix") || strings.Contains(lower, "must_fix") {
				mustFix = append(mustFix, issue.Number)
				break
			}
		}
	}
	sort.Ints(blockers)
	sort.Ints(mustFix)
	blockingPRs := make([]ReviewQueueItem, 0)
	findings := make([]ReviewGateFinding, 0)
	for _, item := range queue.Items {
		findings = append(findings, item.Findings...)
		if len(item.Blockers) > 0 {
			blockingPRs = append(blockingPRs, item)
		}
	}
	for _, issue := range blockers {
		findings = append(findings, managedPolicyFindingForTarget(policy, BlockerUnresolvedBlocker, "open issue has a blocker label", fmt.Sprintf("issue#%d", issue)))
	}
	for _, issue := range mustFix {
		findings = append(findings, managedPolicyFindingForTarget(policy, "must_fix_issue", "open issue has a must-fix label", fmt.Sprintf("issue#%d", issue)))
	}
	ready := len(blockingPRs) == 0 && (!policy.RequiresManagedDelivery() || (len(blockers) == 0 && len(mustFix) == 0))
	return ReleaseReadinessReport{Repo: client.Repo().FullName(), Generated: now.UTC().Format(time.RFC3339), Policy: policy, Ready: ready, BlockingPRs: blockingPRs, OpenBlockers: blockers, OpenMustFix: mustFix, BlockerTaxonomy: []string{BlockerMissingApproval, BlockerFailingChecks, BlockerUnresolvedBlocker, BlockerPolicyViolation}, Findings: findings}, nil
}

func FormatReviewQueueText(report ReviewQueueReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "review queue: %s\n", report.Repo)
	fmt.Fprintf(&b, "policy: mode=%s delivery=%s source=%s\n", report.Policy.OperationMode, report.Policy.DeliveryPolicy, report.Policy.Source)
	fmt.Fprintf(&b, "items: %d\n", len(report.Items))
	for _, item := range report.Items {
		blockers := strings.Join(item.Blockers, ",")
		if blockers == "" {
			blockers = "none"
		}
		fmt.Fprintf(&b, "- #%d %s route=%s blockers=%s\n", item.PR.Number, item.PR.Title, item.RouteTo, blockers)
		for _, finding := range item.Findings {
			fmt.Fprintf(&b, "  finding: %s finding_class=%s enforced=%t\n", finding.Code, finding.FindingClass, finding.Enforced)
		}
	}
	fmt.Fprintf(&b, "next step: %s\n", reviewQueueNextStep(report))
	return b.String()
}

func FormatReleaseReadinessText(report ReleaseReadinessReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "release readiness: %s ready=%t\n", report.Repo, report.Ready)
	fmt.Fprintf(&b, "policy: mode=%s delivery=%s source=%s\n", report.Policy.OperationMode, report.Policy.DeliveryPolicy, report.Policy.Source)
	fmt.Fprintf(&b, "blocking PRs: %d open blockers: %d open must-fix: %d\n", len(report.BlockingPRs), len(report.OpenBlockers), len(report.OpenMustFix))
	for _, finding := range report.Findings {
		fmt.Fprintf(&b, "- finding: %s finding_class=%s enforced=%t target=%s\n", finding.Code, finding.FindingClass, finding.Enforced, finding.Target)
	}
	if report.Ready {
		b.WriteString("next step: proceed with release review\n")
	} else {
		b.WriteString("next step: resolve blocking readiness findings\n")
	}
	return b.String()
}

func reviewQueueNextStep(report ReviewQueueReport) string {
	if len(report.Items) == 0 {
		return "gira status --repo " + report.Repo
	}
	return fmt.Sprintf("review PR #%d", report.Items[0].PR.Number)
}

func classifyPRBlockers(pr ReviewPR) []string {
	blockers, _ := classifyPRBlockersWithPolicy(pr, ResolvedOperationPolicy{OperationMode: OperationModeManaged, DeliveryPolicy: DeliveryPolicyRequired, Source: "compatibility"})
	return blockers
}

func classifyPRBlockersWithPolicy(pr ReviewPR, policy ResolvedOperationPolicy) ([]string, []ReviewGateFinding) {
	blockers := make([]string, 0)
	findings := make([]ReviewGateFinding, 0)
	if pr.IsDraft {
		blockers = append(blockers, BlockerPolicyViolation)
		findings = append(findings, ReviewGateFinding{FindingClass: ReviewFindingClassProvider, Code: "draft_pr", Enforced: false, Message: "pull request is still a draft", Target: fmt.Sprintf("pr#%d", pr.Number)})
	}
	if pr.ReviewDecision != "APPROVED" {
		finding := managedPolicyFinding(policy, BlockerMissingApproval, "pull request does not have an approving review", pr.Number)
		findings = append(findings, finding)
		if finding.Enforced {
			blockers = append(blockers, BlockerMissingApproval)
		}
	}
	if pr.CheckStatus == "failing" {
		blockers = append(blockers, BlockerFailingChecks)
		findings = append(findings, ReviewGateFinding{FindingClass: ReviewFindingClassProvider, Code: BlockerFailingChecks, Enforced: false, Message: "pull request checks are failing", Target: fmt.Sprintf("pr#%d", pr.Number)})
	}
	if len(ExtractClosureIssueNumbers(pr.Body)) == 0 && !hasSubstring(pr.Labels, "docs") && !hasSubstring(pr.Labels, "chore") {
		finding := managedPolicyFinding(policy, BlockerPolicyViolation, "pull request has no Gira closure link", pr.Number)
		findings = append(findings, finding)
		if finding.Enforced {
			blockers = append(blockers, BlockerPolicyViolation)
		}
	}
	for _, label := range pr.Labels {
		if strings.Contains(strings.ToLower(label), "blocker") {
			finding := managedPolicyFindingForTarget(policy, BlockerUnresolvedBlocker, "pull request has a blocker label", fmt.Sprintf("pr#%d", pr.Number))
			findings = append(findings, finding)
			if finding.Enforced {
				blockers = append(blockers, BlockerUnresolvedBlocker)
			}
			break
		}
	}
	sort.Strings(blockers)
	return blockers, findings
}

func managedPolicyFinding(policy ResolvedOperationPolicy, code, message string, pr int) ReviewGateFinding {
	return managedPolicyFindingForTarget(policy, code, message, fmt.Sprintf("pr#%d", pr))
}

func managedPolicyFindingForTarget(policy ResolvedOperationPolicy, code, message, target string) ReviewGateFinding {
	return ReviewGateFinding{FindingClass: ReviewFindingClassPolicy, Code: code, Enforced: policy.RequiresManagedDelivery(), Message: message, Target: target}
}

type reviewGatePolicyProvider interface {
	ResolveOperationPolicy() (ResolvedOperationPolicy, error)
}

func resolveReviewGatePolicy(client ReviewGateClient) (ResolvedOperationPolicy, error) {
	if provider, ok := client.(reviewGatePolicyProvider); ok {
		policy, err := provider.ResolveOperationPolicy()
		if err != nil {
			return ResolvedOperationPolicy{}, fmt.Errorf("resolve review gate operation policy: %w", err)
		}
		if strings.TrimSpace(policy.OperationMode) == "" || strings.TrimSpace(policy.DeliveryPolicy) == "" || strings.TrimSpace(policy.Source) == "" {
			return ResolvedOperationPolicy{}, fmt.Errorf("resolve review gate operation policy: incomplete resolved policy")
		}
		return policy, nil
	}
	// Preserve the compatibility behavior for downstream ReviewGateClient
	// implementations that predate the operation-policy contract. The
	// GitHub-backed CLI client resolves the real repository policy above.
	return ResolveOperationPolicy(OperationPolicyConfig{}, "legacy_review_gate_client", true)
}

var closureKeywordPattern = regexp.MustCompile(`(?i)\b(?:closes|fixes|resolves)\s+#([0-9]+)\b`)

func ExtractClosureIssueNumbers(body string) []int {
	matches := closureKeywordPattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := map[int]struct{}{}
	issues := make([]int, 0, len(matches))
	for _, m := range matches {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		issues = append(issues, n)
	}
	sort.Ints(issues)
	return issues
}
