package gira

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type DevPROpenResult struct {
	Repo     string `json:"repo"`
	Issue    int    `json:"issue"`
	PRNumber int    `json:"pr_number"`
	PRURL    string `json:"pr_url"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	Draft    bool   `json:"draft"`
	Base     string `json:"base,omitempty"`
}

type DevPRCreateOptions struct {
	Draft bool
	Base  string
}

type DevPRStatusResult struct {
	Repo              string       `json:"repo"`
	Issue             int          `json:"issue"`
	PRNumber          int          `json:"pr_number,omitempty"`
	PRURL             string       `json:"pr_url,omitempty"`
	State             string       `json:"state,omitempty"`
	Mergeable         string       `json:"mergeable,omitempty"`
	ReviewDecision    string       `json:"review_decision,omitempty"`
	IsDraft           bool         `json:"is_draft,omitempty"`
	Binding           DevPRBinding `json:"binding,omitempty"`
	Blockers          []string     `json:"blockers"`
	Checks            []DevPRCheck `json:"checks,omitempty"`
	ChecksUnavailable bool         `json:"checks_unavailable,omitempty"`
	Ready             bool         `json:"ready"`
	LookupAttempts    int          `json:"lookup_attempts,omitempty"`
	HeadSHA           string       `json:"head_sha,omitempty"`
	MergeCommitSHA    string       `json:"merge_commit_sha,omitempty"`
	ClosingReference  bool         `json:"closing_reference"`
}

type DevPRBinding struct {
	Trusted              bool     `json:"trusted"`
	Source               string   `json:"source"`
	HeadRef              string   `json:"head_ref,omitempty"`
	BaseRef              string   `json:"base_ref,omitempty"`
	ExpectedHeadPrefixes []string `json:"expected_head_prefixes,omitempty"`
	Blockers             []string `json:"blockers,omitempty"`
	Warnings             []string `json:"warnings,omitempty"`
	CandidatePRs         []int    `json:"candidate_prs,omitempty"`
}

type devPRBindingPolicy struct {
	RecordedWorkBranch string
	ResolvedWorkBranch string
}

type DevPRCheck struct {
	Name        string `json:"name,omitempty"`
	Workflow    string `json:"workflow,omitempty"`
	AppID       int    `json:"app_id,omitempty"`
	AppSlug     string `json:"app_slug,omitempty"`
	WorkflowID  int    `json:"workflow_id,omitempty"`
	HeadSHA     string `json:"head_sha,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
	Superseded  bool   `json:"superseded,omitempty"`
	Status      string `json:"status,omitempty"`
	Conclusion  string `json:"conclusion,omitempty"`
	URL         string `json:"url,omitempty"`
	State       string `json:"state"`
}

type prSummary struct {
	Number         int    `json:"number"`
	Title          string `json:"title"`
	Body           string `json:"body"`
	State          string `json:"state"`
	URL            string `json:"url"`
	ReviewDecision string `json:"reviewDecision"`
	IsDraft        bool   `json:"isDraft"`
	MergeState     string `json:"mergeStateStatus"`
	HeadRefName    string `json:"headRefName"`
	BaseRefName    string `json:"baseRefName"`
	HeadRefOID     string `json:"headRefOid"`
	MergeCommit    *struct {
		OID string `json:"oid"`
	} `json:"mergeCommit"`
	StatusRollup []struct {
		Name       string `json:"name"`
		Workflow   string `json:"workflowName"`
		Conclusion string `json:"conclusion"`
		Status     string `json:"status"`
		URL        string `json:"detailsUrl"`
	} `json:"statusCheckRollup"`
}

type restTimelineEvent struct {
	Event  string `json:"event"`
	Source struct {
		Issue *struct {
			Number      int    `json:"number"`
			Title       string `json:"title"`
			Body        string `json:"body"`
			State       string `json:"state"`
			HTMLURL     string `json:"html_url"`
			PullRequest *struct {
				URL     string `json:"url"`
				HTMLURL string `json:"html_url"`
			} `json:"pull_request"`
		} `json:"issue"`
	} `json:"source"`
}

type restPull struct {
	Number         int     `json:"number"`
	Title          string  `json:"title"`
	Body           string  `json:"body"`
	State          string  `json:"state"`
	HTMLURL        string  `json:"html_url"`
	Draft          bool    `json:"draft"`
	MergedAt       *string `json:"merged_at"`
	MergeCommitSHA string  `json:"merge_commit_sha"`
	MergeableState string  `json:"mergeable_state"`
	Head           struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
}

type restPullListItem struct {
	Number int `json:"number"`
}

type restReview struct {
	State       string `json:"state"`
	SubmittedAt string `json:"submitted_at"`
}

type restCheckRuns struct {
	CheckRuns []struct {
		Name        string `json:"name"`
		Status      string `json:"status"`
		Conclusion  string `json:"conclusion"`
		HTMLURL     string `json:"html_url"`
		CompletedAt string `json:"completed_at"`
		App         *struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Slug string `json:"slug"`
		} `json:"app"`
	} `json:"check_runs"`
}

type restWorkflowRun struct {
	WorkflowID int    `json:"workflow_id"`
	HeadSHA    string `json:"head_sha"`
}

type restCombinedStatus struct {
	Statuses []struct {
		Context     string `json:"context"`
		State       string `json:"state"`
		TargetURL   string `json:"target_url"`
		Description string `json:"description"`
	} `json:"statuses"`
}

func OpenDevPR(repo RepoRef, issueNumber int, runner CommandRunner) (DevPROpenResult, error) {
	return OpenDevPRWithOptions(repo, issueNumber, false, runner)
}

func OpenDevPRWithOptions(repo RepoRef, issueNumber int, draft bool, runner CommandRunner) (DevPROpenResult, error) {
	return OpenDevPRWithCreateOptions(repo, issueNumber, DevPRCreateOptions{Draft: draft}, runner)
}

func OpenDevPRWithCreateOptions(repo RepoRef, issueNumber int, options DevPRCreateOptions, runner CommandRunner) (DevPROpenResult, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	issue, err := fetchDevIssue(repo, issueNumber, runner)
	if err != nil {
		return DevPROpenResult{}, err
	}
	title := fmt.Sprintf("feat: %s", issue.Title)
	body := releaseImpactPRBody(issueNumber, issue.Body)
	args := []string{"pr", "create", "--repo", repo.FullName(), "--title", title, "--body", body}
	base := strings.TrimSpace(options.Base)
	if base != "" {
		args = append(args, "--base", base)
	}
	if options.Draft {
		args = append(args, "--draft")
	}
	out, err := runner.Run("gh", args...)
	if err != nil {
		return DevPROpenResult{}, err
	}
	url := strings.TrimSpace(string(out))
	prNumber := extractPRNumber(url)
	return DevPROpenResult{Repo: repo.FullName(), Issue: issueNumber, PRNumber: prNumber, PRURL: url, Title: title, Body: body, Draft: options.Draft, Base: base}, nil
}

func DevPRStatus(repo RepoRef, issueNumber int, runner CommandRunner) (DevPRStatusResult, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	bindingPolicy := resolveDevPRBindingPolicy(repo, issueNumber, runner)
	if status, ok := devPRStatusRESTFirst(repo, issueNumber, bindingPolicy, runner); ok {
		return status, nil
	}
	if status, ok := devPRStatusRESTSearchFallback(repo, issueNumber, bindingPolicy, runner); ok {
		return status, nil
	}
	if devPRGraphQLFallbackEnabled() {
		return devPRStatusGraphQLFallback(repo, issueNumber, bindingPolicy, runner)
	}
	result := DevPRStatusResult{Repo: repo.FullName(), Issue: issueNumber, Blockers: []string{}}
	result.Blockers = append(result.Blockers, "missing_linked_pr")
	result.Ready = false
	return result, nil
}

func devPRGraphQLFallbackEnabled() bool {
	value := strings.TrimSpace(os.Getenv("GIRA_DEV_PR_GRAPHQL_FALLBACK"))
	return value == "1" || strings.EqualFold(value, "true") || strings.EqualFold(value, "yes")
}

func devPRStatusGraphQLFallback(repo RepoRef, issueNumber int, bindingPolicy devPRBindingPolicy, runner CommandRunner) (DevPRStatusResult, error) {
	search := fmt.Sprintf("repo:%s is:pr %d", repo.FullName(), issueNumber)
	out, err := runner.Run("gh", "pr", "list", "--repo", repo.FullName(), "--state", "all", "--search", search, "--json", "number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid", "--limit", "20")
	if err != nil {
		return DevPRStatusResult{}, err
	}
	var prs []prSummary
	if err := json.Unmarshal(out, &prs); err != nil {
		return DevPRStatusResult{}, fmt.Errorf("parse pr list JSON: %w", err)
	}
	candidates := []prSummary{}
	for _, pr := range prs {
		if hasClosingKeyword(pr.Body, issueNumber) {
			candidates = append(candidates, pr)
		}
	}
	selected, ambiguity := selectClosingPRSummary(issueNumber, candidates, bindingPolicy)
	if selected.Number == 0 {
		if ambiguity != nil {
			return ambiguousDevPRStatus(repo, issueNumber, candidates[0], *ambiguity), nil
		}
		return DevPRStatusResult{Repo: repo.FullName(), Issue: issueNumber, Blockers: []string{"missing_linked_pr"}}, nil
	}
	result := devPRStatusFromSummary(repo, issueNumber, selected, bindingPolicy)
	if ambiguity != nil {
		result.Binding = *ambiguity
		result.Blockers = appendUniqueStrings(result.Blockers, ambiguity.Blockers...)
	}
	result.Ready = len(result.Blockers) == 0
	return result, nil
}

func devPRStatusRESTFirst(repo RepoRef, issueNumber int, bindingPolicy devPRBindingPolicy, runner CommandRunner) (DevPRStatusResult, bool) {
	prNumbers, ok := linkedPRNumbersFromIssueTimeline(repo, issueNumber, runner)
	if !ok {
		return DevPRStatusResult{}, false
	}
	result := DevPRStatusResult{Repo: repo.FullName(), Issue: issueNumber, Blockers: []string{}}
	if len(prNumbers) == 0 {
		result.Blockers = append(result.Blockers, "missing_linked_pr")
		result.Ready = false
		return result, true
	}
	candidates := []restPull{}
	for _, prNumber := range prNumbers {
		pr, ok := fetchRESTPull(repo, prNumber, runner)
		if !ok || !hasClosingKeyword(pr.Body, issueNumber) {
			continue
		}
		candidates = append(candidates, pr)
	}
	return devPRStatusFromRESTCandidates(repo, issueNumber, candidates, bindingPolicy, runner), true
}

func devPRStatusRESTSearchFallback(repo RepoRef, issueNumber int, bindingPolicy devPRBindingPolicy, runner CommandRunner) (DevPRStatusResult, bool) {
	out, err := runner.Run("gh", "api", fmt.Sprintf("repos/%s/pulls", repo.FullName()), "-X", "GET", "-f", "state=all", "-f", "sort=updated", "-f", "direction=desc", "-f", "per_page=100")
	if err != nil {
		return DevPRStatusResult{}, false
	}
	var pulls []restPullListItem
	if err := json.Unmarshal(out, &pulls); err != nil {
		return DevPRStatusResult{}, false
	}
	candidates := []restPull{}
	for _, item := range pulls {
		if item.Number <= 0 {
			continue
		}
		pr, ok := fetchRESTPull(repo, item.Number, runner)
		if !ok || !hasClosingKeyword(pr.Body, issueNumber) {
			continue
		}
		candidates = append(candidates, pr)
	}
	if len(candidates) == 0 {
		return DevPRStatusResult{}, false
	}
	return devPRStatusFromRESTCandidates(repo, issueNumber, candidates, bindingPolicy, runner), true
}

func devPRStatusFromRESTCandidates(repo RepoRef, issueNumber int, candidates []restPull, bindingPolicy devPRBindingPolicy, runner CommandRunner) DevPRStatusResult {
	if len(candidates) == 0 {
		return DevPRStatusResult{Repo: repo.FullName(), Issue: issueNumber, Blockers: []string{"missing_linked_pr"}}
	}
	summaries := make([]prSummary, 0, len(candidates))
	byNumber := map[int]restPull{}
	for _, pr := range candidates {
		summary := restPullSummary(pr)
		summaries = append(summaries, summary)
		byNumber[pr.Number] = pr
	}
	selected, ambiguity := selectClosingPRSummary(issueNumber, summaries, bindingPolicy)
	if selected.Number == 0 {
		return ambiguousDevPRStatus(repo, issueNumber, summaries[0], *ambiguity)
	}
	result := devPRStatusFromRESTPull(repo, issueNumber, byNumber[selected.Number], bindingPolicy, runner)
	if ambiguity != nil {
		result.Binding = *ambiguity
		result.Blockers = appendUniqueStrings(result.Blockers, ambiguity.Blockers...)
		result.Ready = false
	}
	return result
}

func devPRStatusFromRESTPull(repo RepoRef, issueNumber int, pr restPull, bindingPolicy devPRBindingPolicy, runner CommandRunner) DevPRStatusResult {
	result := DevPRStatusResult{Repo: repo.FullName(), Issue: issueNumber, Blockers: []string{}}
	result.PRNumber = pr.Number
	result.PRURL = pr.HTMLURL
	result.State = restPRState(pr)
	result.Mergeable = strings.ToUpper(strings.TrimSpace(pr.MergeableState))
	result.ReviewDecision = restPRReviewDecision(repo, pr.Number, pr.Base.Ref, runner)
	result.IsDraft = pr.Draft
	result.HeadSHA = strings.TrimSpace(pr.Head.SHA)
	result.MergeCommitSHA = strings.TrimSpace(pr.MergeCommitSHA)
	result.ClosingReference = hasClosingKeyword(pr.Body, issueNumber)
	summary := restPullSummary(pr)
	summary.ReviewDecision = result.ReviewDecision
	summary.MergeState = result.Mergeable
	result.Binding = validateDevPRBinding(issueNumber, summary, bindingPolicy)
	result.Blockers = append(result.Blockers, result.Binding.Blockers...)
	if pr.Draft {
		result.Blockers = append(result.Blockers, "draft")
	}
	if result.ReviewDecision == "CHANGES_REQUESTED" || result.ReviewDecision == "REVIEW_REQUIRED" {
		result.Blockers = append(result.Blockers, "review")
	}
	result.Checks, result.ChecksUnavailable = restPRChecksWithAvailability(repo, pr.Head.SHA, runner)
	if result.ChecksUnavailable {
		result.Blockers = appendUniqueStrings(result.Blockers, "checks")
	}
	result.Blockers = append(result.Blockers, devPRCheckBlockers(result.Checks)...)
	result.Ready = len(result.Blockers) == 0
	return result
}

func restPullSummary(pr restPull) prSummary {
	summary := prSummary{Number: pr.Number, Title: pr.Title, Body: pr.Body, State: restPRState(pr), URL: pr.HTMLURL, IsDraft: pr.Draft, HeadRefName: pr.Head.Ref, BaseRefName: pr.Base.Ref, HeadRefOID: pr.Head.SHA}
	if strings.TrimSpace(pr.MergeCommitSHA) != "" {
		summary.MergeCommit = &struct {
			OID string `json:"oid"`
		}{OID: strings.TrimSpace(pr.MergeCommitSHA)}
	}
	return summary
}

func devPRStatusFromSummary(repo RepoRef, issueNumber int, pr prSummary, bindingPolicy devPRBindingPolicy) DevPRStatusResult {
	result := DevPRStatusResult{
		Repo:             repo.FullName(),
		Issue:            issueNumber,
		PRNumber:         pr.Number,
		PRURL:            pr.URL,
		State:            pr.State,
		Mergeable:        pr.MergeState,
		ReviewDecision:   pr.ReviewDecision,
		IsDraft:          pr.IsDraft,
		Binding:          validateDevPRBinding(issueNumber, pr, bindingPolicy),
		Blockers:         []string{},
		HeadSHA:          strings.TrimSpace(pr.HeadRefOID),
		ClosingReference: hasClosingKeyword(pr.Body, issueNumber),
	}
	if pr.MergeCommit != nil {
		result.MergeCommitSHA = strings.TrimSpace(pr.MergeCommit.OID)
	}
	result.Blockers = append(result.Blockers, result.Binding.Blockers...)
	if pr.IsDraft {
		result.Blockers = append(result.Blockers, "draft")
	}
	if pr.ReviewDecision == "CHANGES_REQUESTED" || pr.ReviewDecision == "REVIEW_REQUIRED" {
		result.Blockers = append(result.Blockers, "review")
	}
	for _, check := range pr.StatusRollup {
		result.Checks = append(result.Checks, DevPRCheck{Name: check.Name, Workflow: check.Workflow, Status: check.Status, Conclusion: check.Conclusion, URL: check.URL, State: classifyDevPRCheck(check.Status, check.Conclusion)})
	}
	result.Blockers = appendUniqueStrings(result.Blockers, devPRCheckBlockers(result.Checks)...)
	result.Ready = len(result.Blockers) == 0
	return result
}

func selectClosingPRSummary(issueNumber int, candidates []prSummary, bindingPolicy devPRBindingPolicy) (prSummary, *DevPRBinding) {
	if len(candidates) == 0 {
		return prSummary{}, nil
	}
	merged := []prSummary{}
	trustedOpen := []prSummary{}
	for _, candidate := range candidates {
		if strings.EqualFold(candidate.State, "MERGED") {
			merged = append(merged, candidate)
			continue
		}
		binding := validateDevPRBinding(issueNumber, candidate, bindingPolicy)
		if strings.EqualFold(candidate.State, "OPEN") && binding.Trusted {
			trustedOpen = append(trustedOpen, candidate)
		}
	}
	if len(merged) == 1 {
		return merged[0], nil
	}
	if len(merged) > 1 {
		return prSummary{}, ambiguousPRBinding(issueNumber, candidates, bindingPolicy, "ambiguous_multiple_merged_prs")
	}
	if len(trustedOpen) == 1 {
		return trustedOpen[0], nil
	}
	if len(trustedOpen) > 1 {
		return prSummary{}, ambiguousPRBinding(issueNumber, candidates, bindingPolicy, "ambiguous_multiple_trusted_open_prs")
	}
	if len(candidates) == 1 {
		candidate := candidates[0]
		if strings.EqualFold(candidate.State, "CLOSED") {
			return candidate, ambiguousPRBinding(issueNumber, candidates, bindingPolicy, "closed_unmerged_pr")
		}
		return candidate, nil
	}
	return prSummary{}, ambiguousPRBinding(issueNumber, candidates, bindingPolicy, "ambiguous_closing_prs")
}

func ambiguousPRBinding(issueNumber int, candidates []prSummary, bindingPolicy devPRBindingPolicy, source string) *DevPRBinding {
	binding := validateDevPRBinding(issueNumber, candidates[0], bindingPolicy)
	binding.Trusted = false
	binding.Source = source
	binding.Blockers = []string{"pr_binding"}
	binding.CandidatePRs = make([]int, 0, len(candidates))
	for _, candidate := range candidates {
		binding.CandidatePRs = append(binding.CandidatePRs, candidate.Number)
	}
	sort.Ints(binding.CandidatePRs)
	return &binding
}

func ambiguousDevPRStatus(repo RepoRef, issueNumber int, representative prSummary, binding DevPRBinding) DevPRStatusResult {
	result := devPRStatusFromSummary(repo, issueNumber, representative, devPRBindingPolicy{})
	result.Binding = binding
	result.Blockers = appendUniqueStrings(removeString(result.Blockers, "pr_binding"), "pr_binding")
	result.Ready = false
	return result
}

func linkedPRNumbersFromIssueTimeline(repo RepoRef, issueNumber int, runner CommandRunner) ([]int, bool) {
	out, err := runner.Run("gh", "api", fmt.Sprintf("repos/%s/issues/%d/timeline", repo.FullName(), issueNumber), "--paginate")
	if err != nil {
		return nil, false
	}
	var events []restTimelineEvent
	if err := json.Unmarshal(out, &events); err != nil {
		return nil, false
	}
	numbers := []int{}
	for _, event := range events {
		source := event.Source.Issue
		if source == nil || source.PullRequest == nil || source.Number <= 0 {
			continue
		}
		numbers = appendUniqueInts(numbers, source.Number)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(numbers)))
	return numbers, true
}

func fetchRESTPull(repo RepoRef, prNumber int, runner CommandRunner) (restPull, bool) {
	out, err := runner.Run("gh", "api", fmt.Sprintf("repos/%s/pulls/%d", repo.FullName(), prNumber))
	if err != nil {
		return restPull{}, false
	}
	var pr restPull
	if err := json.Unmarshal(out, &pr); err != nil {
		return restPull{}, false
	}
	if pr.Number == 0 {
		pr.Number = prNumber
	}
	return pr, true
}

func verifyMergedDevPR(repo RepoRef, issueNumber int, prNumber int, previous DevPRStatusResult, runner CommandRunner) (DevPRStatusResult, error) {
	pr, ok := fetchRESTPull(repo, prNumber, runner)
	if !ok {
		return previous, fmt.Errorf("verify merged PR #%d: current GitHub PR state is unavailable", prNumber)
	}
	if pr.Number != prNumber || !hasClosingKeyword(pr.Body, issueNumber) {
		return previous, fmt.Errorf("verify merged PR #%d: PR number or closing relationship does not match ticket #%d", prNumber, issueNumber)
	}
	if !strings.EqualFold(restPRState(pr), "MERGED") || strings.TrimSpace(pr.MergeCommitSHA) == "" || strings.TrimSpace(pr.Head.SHA) == "" {
		return previous, fmt.Errorf("verify merged PR #%d: merged state, merge commit SHA, and head SHA are required", prNumber)
	}
	verified := previous
	verified.PRNumber = pr.Number
	verified.PRURL = pr.HTMLURL
	verified.State = "MERGED"
	verified.HeadSHA = strings.TrimSpace(pr.Head.SHA)
	verified.MergeCommitSHA = strings.TrimSpace(pr.MergeCommitSHA)
	verified.ClosingReference = true
	verified.Binding = validateDevPRBinding(issueNumber, restPullSummary(pr), devPRBindingPolicy{})
	verified.Blockers = nil
	verified.Ready = true
	return verified, nil
}

func restPRState(pr restPull) string {
	if pr.MergedAt != nil && strings.TrimSpace(*pr.MergedAt) != "" {
		return "MERGED"
	}
	return strings.ToUpper(strings.TrimSpace(pr.State))
}

func restPRReviewDecision(repo RepoRef, prNumber int, baseRef string, runner CommandRunner) string {
	decision := restPRReviewsDecision(repo, prNumber, runner)
	if decision == "APPROVED" || decision == "CHANGES_REQUESTED" {
		return decision
	}
	if restBranchRequiresReviews(repo, baseRef, runner) {
		return "REVIEW_REQUIRED"
	}
	return decision
}

func restPRReviewsDecision(repo RepoRef, prNumber int, runner CommandRunner) string {
	out, err := runner.Run("gh", "api", fmt.Sprintf("repos/%s/pulls/%d/reviews", repo.FullName(), prNumber), "--paginate")
	if err != nil {
		return ""
	}
	var reviews []restReview
	if err := json.Unmarshal(out, &reviews); err != nil {
		return ""
	}
	latestState := ""
	latestAt := ""
	for _, review := range reviews {
		state := strings.ToUpper(strings.TrimSpace(review.State))
		if state != "APPROVED" && state != "CHANGES_REQUESTED" {
			continue
		}
		if strings.TrimSpace(review.SubmittedAt) >= latestAt {
			latestAt = strings.TrimSpace(review.SubmittedAt)
			latestState = state
		}
	}
	return latestState
}

func restBranchRequiresReviews(repo RepoRef, baseRef string, runner CommandRunner) bool {
	baseRef = strings.TrimSpace(baseRef)
	if baseRef == "" {
		return false
	}
	_, err := runner.Run("gh", "api", fmt.Sprintf("repos/%s/branches/%s/protection/required_pull_request_reviews", repo.FullName(), baseRef))
	return err == nil
}

func restPRChecks(repo RepoRef, headSHA string, runner CommandRunner) []DevPRCheck {
	checks, _ := restPRChecksWithAvailability(repo, headSHA, runner)
	return checks
}

func restPRChecksWithAvailability(repo RepoRef, headSHA string, runner CommandRunner) ([]DevPRCheck, bool) {
	headSHA = strings.TrimSpace(headSHA)
	if headSHA == "" {
		return nil, true
	}
	checkRuns, checkRunsAvailable := restCheckRunsForCommitWithAvailability(repo, headSHA, runner)
	statuses, statusesAvailable := restStatusesForCommitWithAvailability(repo, headSHA, runner)
	checks := append(checkRuns, statuses...)
	sort.Slice(checks, func(i, j int) bool {
		if checks[i].Workflow != checks[j].Workflow {
			return checks[i].Workflow < checks[j].Workflow
		}
		return checks[i].Name < checks[j].Name
	})
	return checks, !checkRunsAvailable || !statusesAvailable
}

func restCheckRunsForCommit(repo RepoRef, headSHA string, runner CommandRunner) []DevPRCheck {
	checks, _ := restCheckRunsForCommitWithAvailability(repo, headSHA, runner)
	return checks
}

func restCheckRunsForCommitWithAvailability(repo RepoRef, headSHA string, runner CommandRunner) ([]DevPRCheck, bool) {
	out, err := runner.Run("gh", "api", fmt.Sprintf("repos/%s/commits/%s/check-runs", repo.FullName(), headSHA), "-X", "GET", "-f", "per_page=100", "-f", "filter=all", "--paginate", "--slurp")
	if err != nil {
		return nil, false
	}
	payloads := []restCheckRuns{}
	if err := json.Unmarshal(out, &payloads); err != nil {
		var payload restCheckRuns
		if err := json.Unmarshal(out, &payload); err != nil {
			return nil, false
		}
		payloads = append(payloads, payload)
	}
	checks := []DevPRCheck{}
	workflowRuns := map[int]restWorkflowRun{}
	for _, payload := range payloads {
		for _, check := range payload.CheckRuns {
			workflow := ""
			if check.App != nil {
				workflow = check.App.Name
			}
			result := DevPRCheck{
				Name:        check.Name,
				Workflow:    workflow,
				Status:      strings.ToUpper(check.Status),
				Conclusion:  strings.ToUpper(check.Conclusion),
				URL:         check.HTMLURL,
				CompletedAt: strings.TrimSpace(check.CompletedAt),
				State:       classifyDevPRCheck(check.Status, check.Conclusion),
			}
			if check.App != nil {
				result.AppID = check.App.ID
				result.AppSlug = strings.TrimSpace(check.App.Slug)
			}
			if runID := githubActionsRunID(check.HTMLURL); isGitHubActionsCheck(result) && runID > 0 {
				run, ok := workflowRuns[runID]
				if !ok {
					run, ok = fetchWorkflowRun(repo, runID, runner)
					if ok {
						workflowRuns[runID] = run
					}
				}
				if ok {
					result.WorkflowID = run.WorkflowID
					result.HeadSHA = strings.TrimSpace(run.HeadSHA)
				}
			}
			checks = append(checks, result)
		}
	}
	return markSupersededCancelledChecks(checks, headSHA), true
}

func isGitHubActionsCheck(check DevPRCheck) bool {
	return check.AppID > 0 && strings.EqualFold(strings.TrimSpace(check.AppSlug), "github-actions")
}

func githubActionsRunID(rawURL string) int {
	parts := strings.Split(strings.TrimSpace(rawURL), "/")
	for index := 0; index+2 < len(parts); index++ {
		if parts[index] != "actions" || parts[index+1] != "runs" {
			continue
		}
		runID, err := strconv.Atoi(parts[index+2])
		if err == nil && runID > 0 {
			return runID
		}
	}
	return 0
}

func fetchWorkflowRun(repo RepoRef, runID int, runner CommandRunner) (restWorkflowRun, bool) {
	out, err := runner.Run("gh", "api", fmt.Sprintf("repos/%s/actions/runs/%d", repo.FullName(), runID))
	if err != nil {
		return restWorkflowRun{}, false
	}
	var run restWorkflowRun
	if err := json.Unmarshal(out, &run); err != nil || run.WorkflowID <= 0 || strings.TrimSpace(run.HeadSHA) == "" {
		return restWorkflowRun{}, false
	}
	return run, true
}

func markSupersededCancelledChecks(checks []DevPRCheck, headSHA string) []DevPRCheck {
	headSHA = strings.TrimSpace(headSHA)
	for index := range checks {
		cancelled := &checks[index]
		if !strings.EqualFold(cancelled.Conclusion, "CANCELLED") || cancelled.WorkflowID <= 0 || cancelled.Name == "" || !strings.EqualFold(strings.TrimSpace(cancelled.HeadSHA), headSHA) {
			continue
		}
		cancelledAt, err := time.Parse(time.RFC3339, strings.TrimSpace(cancelled.CompletedAt))
		if err != nil {
			continue
		}
		for candidateIndex := range checks {
			candidate := checks[candidateIndex]
			if !isGitHubActionsCheck(*cancelled) || !isGitHubActionsCheck(candidate) || candidate.AppID != cancelled.AppID || candidate.WorkflowID != cancelled.WorkflowID || candidate.Name != cancelled.Name || !strings.EqualFold(strings.TrimSpace(candidate.HeadSHA), headSHA) || !strings.EqualFold(candidate.Conclusion, "SUCCESS") {
				continue
			}
			candidateAt, err := time.Parse(time.RFC3339, strings.TrimSpace(candidate.CompletedAt))
			if err != nil || !candidateAt.After(cancelledAt) {
				continue
			}
			cancelled.Superseded = true
			cancelled.State = "passing"
			break
		}
	}
	return checks
}

func restStatusesForCommit(repo RepoRef, headSHA string, runner CommandRunner) []DevPRCheck {
	checks, _ := restStatusesForCommitWithAvailability(repo, headSHA, runner)
	return checks
}

func restStatusesForCommitWithAvailability(repo RepoRef, headSHA string, runner CommandRunner) ([]DevPRCheck, bool) {
	out, err := runner.Run("gh", "api", fmt.Sprintf("repos/%s/commits/%s/status", repo.FullName(), headSHA))
	if err != nil {
		return nil, false
	}
	var payload restCombinedStatus
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, false
	}
	checks := []DevPRCheck{}
	for _, status := range payload.Statuses {
		checkStatus, conclusion := restCommitStatusState(status.State)
		checks = append(checks, DevPRCheck{
			Name:       status.Context,
			Status:     checkStatus,
			Conclusion: conclusion,
			URL:        status.TargetURL,
			State:      classifyDevPRCheck(checkStatus, conclusion),
		})
	}
	return checks, true
}

func restCommitStatusState(state string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "success":
		return "COMPLETED", "SUCCESS"
	case "failure", "error":
		return "COMPLETED", "FAILURE"
	case "pending":
		return "PENDING", ""
	default:
		return "", ""
	}
}

func devPRCheckBlockers(checks []DevPRCheck) []string {
	if len(checks) == 0 {
		return []string{"checks"}
	}
	for _, check := range checks {
		if strings.EqualFold(strings.TrimSpace(check.Conclusion), "skipped") {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(check.State), "failing") {
			return []string{"checks"}
		}
	}
	for _, check := range checks {
		if strings.EqualFold(strings.TrimSpace(check.Conclusion), "skipped") {
			continue
		}
		if strings.TrimSpace(check.State) == "" || strings.EqualFold(strings.TrimSpace(check.State), "unknown") {
			return []string{"checks"}
		}
	}
	for _, check := range checks {
		if strings.EqualFold(strings.TrimSpace(check.Conclusion), "skipped") {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(check.State), "pending") {
			return []string{"checks_pending"}
		}
	}
	return nil
}

func appendUniqueInts(values []int, next int) []int {
	for _, value := range values {
		if value == next {
			return values
		}
	}
	return append(values, next)
}

func DevPRStatusWithMissingPRRetry(repo RepoRef, issueNumber int, runner CommandRunner, attempts int, delay time.Duration) (DevPRStatusResult, error) {
	if attempts < 1 {
		attempts = 1
	}
	status, err := DevPRStatus(repo, issueNumber, runner)
	return retryDevPRStatusAfterMissing(repo, issueNumber, runner, status, attempts, delay, err)
}

func retryDevPRStatusAfterMissing(repo RepoRef, issueNumber int, runner CommandRunner, status DevPRStatusResult, attempts int, delay time.Duration, err error) (DevPRStatusResult, error) {
	if attempts < 1 {
		attempts = 1
	}
	if err != nil || !containsString(status.Blockers, "missing_linked_pr") || attempts == 1 {
		return status, err
	}
	status.LookupAttempts = 1
	for attempt := 2; attempt <= attempts; attempt++ {
		if delay > 0 {
			time.Sleep(delay)
		}
		next, err := DevPRStatus(repo, issueNumber, runner)
		if err != nil {
			return next, err
		}
		next.LookupAttempts = attempt
		if !containsString(next.Blockers, "missing_linked_pr") {
			return next, nil
		}
		status = next
	}
	return status, nil
}

func resolveDevPRBindingPolicy(repo RepoRef, issueNumber int, runner CommandRunner) devPRBindingPolicy {
	policy := devPRBindingPolicy{}
	out, err := runner.Run("gh", "issue", "view", strconv.Itoa(issueNumber), "--repo", repo.FullName(), "--json", "number,title,body")
	if err != nil {
		return policy
	}
	var raw struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
	}
	if json.Unmarshal(out, &raw) != nil || raw.Number != issueNumber {
		return policy
	}
	issue := devStartIssue{Number: raw.Number, Title: raw.Title, Body: raw.Body}
	return devPRBindingPolicyFromIssue(issue, runner, repo)
}

func devPRBindingPolicyFromIssue(issue devStartIssue, runner CommandRunner, repo RepoRef) devPRBindingPolicy {
	state := ParseTicketLifecycleState(issue.Body)
	policy := devPRBindingPolicy{RecordedWorkBranch: strings.TrimSpace(state.WorkBranch)}
	if runner == nil {
		return policy
	}
	resolved, err := resolveRepoBranchPolicy(repo, runner)
	if err == nil && resolved.Source == "config" && strings.TrimSpace(resolved.FeatureBranchPattern) != "" {
		policy.ResolvedWorkBranch = formatDevBranch(resolved.FeatureBranchPattern, issue.Number, issue.Title)
	}
	return policy
}

func applyDevPRBindingPolicy(status *DevPRStatusResult, issueNumber int, policy devPRBindingPolicy) {
	if status == nil || status.PRNumber == 0 {
		return
	}
	if strings.HasPrefix(status.Binding.Source, "ambiguous_") || status.Binding.Source == "closed_unmerged_pr" {
		status.Blockers = appendUniqueStrings(status.Blockers, "pr_binding")
		status.Ready = false
		return
	}
	status.Blockers = removeString(status.Blockers, "pr_binding")
	status.Binding = validateDevPRBinding(issueNumber, prSummary{
		State:       status.State,
		HeadRefName: status.Binding.HeadRef,
		BaseRefName: status.Binding.BaseRef,
	}, policy)
	status.Blockers = appendUniqueStrings(status.Blockers, status.Binding.Blockers...)
	status.Ready = len(status.Blockers) == 0
}

func removeString(values []string, target string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}

func validateDevPRBinding(issueNumber int, pr prSummary, policy devPRBindingPolicy) DevPRBinding {
	legacyExact := fmt.Sprintf("issue-%d", issueNumber)
	legacyPrefix := legacyExact + "-"
	expected := []string{}
	for _, branch := range []string{strings.TrimSpace(policy.RecordedWorkBranch), strings.TrimSpace(policy.ResolvedWorkBranch), legacyPrefix, legacyExact} {
		if branch != "" && !containsString(expected, branch) {
			expected = append(expected, branch)
		}
	}
	binding := DevPRBinding{
		Source:               "untrusted_branch",
		HeadRef:              strings.TrimSpace(pr.HeadRefName),
		BaseRef:              strings.TrimSpace(pr.BaseRefName),
		ExpectedHeadPrefixes: expected,
	}
	if strings.EqualFold(pr.State, "MERGED") {
		binding.Trusted = true
		binding.Source = "merged_delivery"
		return binding
	}
	if binding.HeadRef == "" {
		return binding
	}
	if policy.RecordedWorkBranch != "" && binding.HeadRef == strings.TrimSpace(policy.RecordedWorkBranch) {
		binding.Trusted = true
		binding.Source = "recorded_work_branch"
		return binding
	}
	if policy.ResolvedWorkBranch != "" && binding.HeadRef == strings.TrimSpace(policy.ResolvedWorkBranch) {
		binding.Trusted = true
		binding.Source = "branch_policy.feature_branch_pattern"
		return binding
	}
	if binding.HeadRef == legacyExact || strings.HasPrefix(binding.HeadRef, legacyPrefix) {
		binding.Trusted = true
		binding.Source = "legacy_issue_branch"
		return binding
	}
	// A branch name is useful context, not ticket ownership proof. A single PR
	// with a closing reference is already linked by GitHub; keep the naming
	// difference visible without blocking repositories that use another naming
	// convention.
	binding.Trusted = true
	binding.Source = "closing_reference"
	binding.Warnings = append(binding.Warnings, "branch_name_differs_from_suggestion")
	return binding
}

func classifyDevPRCheck(status string, conclusion string) string {
	if strings.EqualFold(conclusion, "failure") || strings.EqualFold(conclusion, "cancelled") || strings.EqualFold(conclusion, "timed_out") {
		return "failing"
	}
	if strings.EqualFold(status, "in_progress") || strings.EqualFold(status, "queued") || strings.EqualFold(status, "pending") {
		return "pending"
	}
	if strings.EqualFold(conclusion, "success") || strings.EqualFold(conclusion, "neutral") || strings.EqualFold(conclusion, "skipped") {
		return "passing"
	}
	return "unknown"
}

func hasClosingKeyword(body string, issueNumber int) bool {
	needle := "#" + strconv.Itoa(issueNumber)
	lower := strings.ToLower(body)
	for _, keyword := range []string{"close", "closes", "closed", "fix", "fixes", "fixed", "resolve", "resolves", "resolved"} {
		if strings.Contains(lower, keyword+" "+needle) {
			return true
		}
	}
	return false
}

func extractPRNumber(url string) int {
	parts := strings.Split(strings.TrimSpace(url), "/")
	if len(parts) == 0 {
		return 0
	}
	n, _ := strconv.Atoi(parts[len(parts)-1])
	return n
}
