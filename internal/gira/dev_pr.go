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
	Repo           string       `json:"repo"`
	Issue          int          `json:"issue"`
	PRNumber       int          `json:"pr_number,omitempty"`
	PRURL          string       `json:"pr_url,omitempty"`
	State          string       `json:"state,omitempty"`
	Mergeable      string       `json:"mergeable,omitempty"`
	ReviewDecision string       `json:"review_decision,omitempty"`
	IsDraft        bool         `json:"is_draft,omitempty"`
	Binding        DevPRBinding `json:"binding,omitempty"`
	Blockers       []string     `json:"blockers"`
	Checks         []DevPRCheck `json:"checks,omitempty"`
	Ready          bool         `json:"ready"`
	LookupAttempts int          `json:"lookup_attempts,omitempty"`
}

type DevPRBinding struct {
	Trusted              bool     `json:"trusted"`
	Source               string   `json:"source"`
	HeadRef              string   `json:"head_ref,omitempty"`
	BaseRef              string   `json:"base_ref,omitempty"`
	ExpectedHeadPrefixes []string `json:"expected_head_prefixes,omitempty"`
	Blockers             []string `json:"blockers,omitempty"`
}

type devPRBindingPolicy struct {
	RecordedWorkBranch string
	ResolvedWorkBranch string
}

type DevPRCheck struct {
	Name       string `json:"name,omitempty"`
	Workflow   string `json:"workflow,omitempty"`
	Status     string `json:"status,omitempty"`
	Conclusion string `json:"conclusion,omitempty"`
	URL        string `json:"url,omitempty"`
	State      string `json:"state"`
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
	StatusRollup   []struct {
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
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		HTMLURL    string `json:"html_url"`
		App        *struct {
			Name string `json:"name"`
		} `json:"app"`
	} `json:"check_runs"`
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
	body := fmt.Sprintf("Closes #%d", issueNumber)
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
	out, err := runner.Run("gh", "pr", "list", "--repo", repo.FullName(), "--state", "all", "--search", search, "--json", "number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName", "--limit", "20")
	if err != nil {
		return DevPRStatusResult{}, err
	}
	var prs []prSummary
	if err := json.Unmarshal(out, &prs); err != nil {
		return DevPRStatusResult{}, fmt.Errorf("parse pr list JSON: %w", err)
	}
	result := DevPRStatusResult{Repo: repo.FullName(), Issue: issueNumber, Blockers: []string{}}
	for _, pr := range prs {
		if hasClosingKeyword(pr.Body, issueNumber) {
			result.PRNumber = pr.Number
			result.PRURL = pr.URL
			result.State = pr.State
			result.Mergeable = pr.MergeState
			result.ReviewDecision = pr.ReviewDecision
			result.IsDraft = pr.IsDraft
			result.Binding = validateDevPRBinding(issueNumber, pr, bindingPolicy)
			result.Blockers = append(result.Blockers, result.Binding.Blockers...)
			if pr.IsDraft {
				result.Blockers = append(result.Blockers, "draft")
			}
			if pr.ReviewDecision == "CHANGES_REQUESTED" || pr.ReviewDecision == "REVIEW_REQUIRED" {
				result.Blockers = append(result.Blockers, "review")
			}
			for _, check := range pr.StatusRollup {
				result.Checks = append(result.Checks, DevPRCheck{Name: check.Name, Workflow: check.Workflow, Status: check.Status, Conclusion: check.Conclusion, URL: check.URL, State: classifyDevPRCheck(check.Status, check.Conclusion)})
				if strings.EqualFold(check.Conclusion, "failure") || strings.EqualFold(check.Conclusion, "cancelled") || strings.EqualFold(check.Conclusion, "timed_out") {
					result.Blockers = append(result.Blockers, "checks")
					break
				}
				if strings.EqualFold(check.Status, "in_progress") || strings.EqualFold(check.Status, "queued") || strings.EqualFold(check.Status, "pending") {
					result.Blockers = append(result.Blockers, "checks_pending")
					break
				}
			}
			break
		}
	}
	if result.PRNumber == 0 {
		result.Blockers = append(result.Blockers, "missing_linked_pr")
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
	for _, prNumber := range prNumbers {
		pr, ok := fetchRESTPull(repo, prNumber, runner)
		if !ok || !hasClosingKeyword(pr.Body, issueNumber) {
			continue
		}
		return devPRStatusFromRESTPull(repo, issueNumber, pr, bindingPolicy, runner), true
	}
	return DevPRStatusResult{}, false
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
	for _, item := range pulls {
		if item.Number <= 0 {
			continue
		}
		pr, ok := fetchRESTPull(repo, item.Number, runner)
		if !ok || !hasClosingKeyword(pr.Body, issueNumber) {
			continue
		}
		return devPRStatusFromRESTPull(repo, issueNumber, pr, bindingPolicy, runner), true
	}
	return DevPRStatusResult{}, false
}

func devPRStatusFromRESTPull(repo RepoRef, issueNumber int, pr restPull, bindingPolicy devPRBindingPolicy, runner CommandRunner) DevPRStatusResult {
	result := DevPRStatusResult{Repo: repo.FullName(), Issue: issueNumber, Blockers: []string{}}
	result.PRNumber = pr.Number
	result.PRURL = pr.HTMLURL
	result.State = restPRState(pr)
	result.Mergeable = strings.ToUpper(strings.TrimSpace(pr.MergeableState))
	result.ReviewDecision = restPRReviewDecision(repo, pr.Number, pr.Base.Ref, runner)
	result.IsDraft = pr.Draft
	summary := prSummary{
		Number:         pr.Number,
		Title:          pr.Title,
		Body:           pr.Body,
		State:          result.State,
		URL:            pr.HTMLURL,
		ReviewDecision: result.ReviewDecision,
		IsDraft:        pr.Draft,
		MergeState:     result.Mergeable,
		HeadRefName:    pr.Head.Ref,
		BaseRefName:    pr.Base.Ref,
	}
	result.Binding = validateDevPRBinding(issueNumber, summary, bindingPolicy)
	result.Blockers = append(result.Blockers, result.Binding.Blockers...)
	if pr.Draft {
		result.Blockers = append(result.Blockers, "draft")
	}
	if result.ReviewDecision == "CHANGES_REQUESTED" || result.ReviewDecision == "REVIEW_REQUIRED" {
		result.Blockers = append(result.Blockers, "review")
	}
	result.Checks = restPRChecks(repo, pr.Head.SHA, runner)
	result.Blockers = append(result.Blockers, devPRCheckBlockers(result.Checks)...)
	result.Ready = len(result.Blockers) == 0
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
	headSHA = strings.TrimSpace(headSHA)
	if headSHA == "" {
		return nil
	}
	checks := restCheckRunsForCommit(repo, headSHA, runner)
	checks = append(checks, restStatusesForCommit(repo, headSHA, runner)...)
	sort.Slice(checks, func(i, j int) bool {
		if checks[i].Workflow != checks[j].Workflow {
			return checks[i].Workflow < checks[j].Workflow
		}
		return checks[i].Name < checks[j].Name
	})
	return checks
}

func restCheckRunsForCommit(repo RepoRef, headSHA string, runner CommandRunner) []DevPRCheck {
	out, err := runner.Run("gh", "api", fmt.Sprintf("repos/%s/commits/%s/check-runs", repo.FullName(), headSHA), "-X", "GET", "-f", "per_page=100")
	if err != nil {
		return nil
	}
	var payload restCheckRuns
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil
	}
	checks := []DevPRCheck{}
	for _, check := range payload.CheckRuns {
		workflow := ""
		if check.App != nil {
			workflow = check.App.Name
		}
		checks = append(checks, DevPRCheck{
			Name:       check.Name,
			Workflow:   workflow,
			Status:     strings.ToUpper(check.Status),
			Conclusion: strings.ToUpper(check.Conclusion),
			URL:        check.HTMLURL,
			State:      classifyDevPRCheck(check.Status, check.Conclusion),
		})
	}
	return checks
}

func restStatusesForCommit(repo RepoRef, headSHA string, runner CommandRunner) []DevPRCheck {
	out, err := runner.Run("gh", "api", fmt.Sprintf("repos/%s/commits/%s/status", repo.FullName(), headSHA))
	if err != nil {
		return nil
	}
	var payload restCombinedStatus
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil
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
	return checks
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
	for _, check := range checks {
		if check.State == "failing" {
			return []string{"checks"}
		}
		if check.State == "pending" {
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
	binding.Blockers = append(binding.Blockers, "pr_binding")
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
	if strings.EqualFold(status, "completed") && strings.TrimSpace(conclusion) == "" {
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
