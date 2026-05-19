package gira

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type WorkStartResult struct {
	Repo          string          `json:"repo"`
	Issue         int             `json:"issue"`
	JiraKey       string          `json:"jira_key,omitempty"`
	MirrorIssue   int             `json:"mirror_issue,omitempty"`
	Title         string          `json:"title"`
	Branch        string          `json:"branch"`
	DryRun        bool            `json:"dry_run"`
	CreatedBranch bool            `json:"created_branch"`
	Status        string          `json:"status"`
	NextStatus    string          `json:"next_status"`
	NextStep      string          `json:"next_step,omitempty"`
	Checks        map[string]bool `json:"checks"`
}

type WorkPRResult struct {
	Repo       string `json:"repo"`
	Issue      int    `json:"issue"`
	DryRun     bool   `json:"dry_run"`
	Draft      bool   `json:"draft"`
	PRNumber   int    `json:"pr_number,omitempty"`
	PRURL      string `json:"pr_url,omitempty"`
	Created    bool   `json:"created"`
	Status     string `json:"status"`
	NextStatus string `json:"next_status"`
	Branch     string `json:"branch,omitempty"`
	BranchPush string `json:"branch_push,omitempty"`
	PushRemote string `json:"push_remote,omitempty"`
	LocalGit   string `json:"local_git,omitempty"`

	Blockers    []string `json:"blockers"`
	ClosingBody string   `json:"closing_body"`
}

type WorkStatusResult struct {
	Command          string                   `json:"command,omitempty"`
	SchemaVersion    string                   `json:"schema_version,omitempty"`
	Repo             string                   `json:"repo"`
	Issue            int                      `json:"issue"`
	Title            string                   `json:"title"`
	State            string                   `json:"state"`
	Status           string                   `json:"status"`
	Labels           []string                 `json:"labels,omitempty"`
	Milestone        string                   `json:"milestone"`
	PRNumber         int                      `json:"pr_number,omitempty"`
	PRURL            string                   `json:"pr_url,omitempty"`
	PRState          string                   `json:"pr_state,omitempty"`
	PRLookupAttempts int                      `json:"pr_lookup_attempts,omitempty"`
	Blockers         []string                 `json:"blockers"`
	NextAction       string                   `json:"next_action"`
	NextStep         string                   `json:"next_step"`
	Branch           *TicketStatusBranch      `json:"branch,omitempty"`
	PullRequest      *TicketStatusPullRequest `json:"pull_request,omitempty"`
	ChecksStatus     string                   `json:"checks_status,omitempty"`
	Checks           []DevPRCheck             `json:"checks,omitempty"`
	ReviewStatus     string                   `json:"review_status,omitempty"`
	Evidence         *TicketStatusEvidence    `json:"evidence,omitempty"`
	Acceptance       *TicketStatusAcceptance  `json:"acceptance_criteria,omitempty"`
	Warnings         []string                 `json:"warnings,omitempty"`
}

type TicketStatusBranch struct {
	Expected string `json:"expected"`
	Current  string `json:"current"`
	Trusted  bool   `json:"trusted"`
	Source   string `json:"source"`
}

type TicketStatusPullRequest struct {
	Available      bool   `json:"available"`
	Number         int    `json:"number"`
	URL            string `json:"url"`
	State          string `json:"state"`
	Mergeable      string `json:"mergeable"`
	HeadRefName    string `json:"head_ref_name"`
	BaseRefName    string `json:"base_ref_name"`
	ReviewDecision string `json:"review_decision"`
	IsDraft        bool   `json:"is_draft"`
}

type TicketStatusEvidence struct {
	ClosingReference bool     `json:"closing_reference"`
	BranchTrusted    bool     `json:"branch_trusted"`
	FinishReady      bool     `json:"finish_ready"`
	Sources          []string `json:"sources"`
}

type TicketStatusAcceptance struct {
	Status     string `json:"status"`
	Total      int    `json:"total"`
	Complete   int    `json:"complete"`
	Incomplete int    `json:"incomplete"`
}

var workStatusMissingPRRetryAttempts = 3
var workStatusMissingPRRetryDelay = time.Second

func StartWork(repo RepoRef, issueNumber int, dryRun bool, runner CommandRunner) (WorkStartResult, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	issue, err := fetchDevIssue(repo, issueNumber, runner)
	if err != nil {
		return WorkStartResult{}, err
	}
	status := displayStatus(managedStatusFromLabels(issue.Labels))
	alreadyStarted := status == "In progress"
	if alreadyStarted && !strings.EqualFold(issue.State, "open") {
		return WorkStartResult{}, fmt.Errorf("issue #%d is not open", issue.Number)
	}

	start, err := StartDevBranch(repo, issueNumber, DefaultDevBranchPattern, dryRun, alreadyStarted, runner)
	result := WorkStartResult{
		Repo:          repo.FullName(),
		Issue:         issueNumber,
		Title:         start.Title,
		Branch:        start.Branch,
		DryRun:        dryRun,
		CreatedBranch: start.Created,
		Status:        status,
		NextStatus:    "In progress",
		NextStep:      workStartNextStep(repo.FullName(), issueNumber, issue.State, status, dryRun),
		Checks:        start.Checked,
	}
	if err != nil {
		return result, err
	}
	if dryRun {
		return result, nil
	}
	if err := setIssueStatus(repo, issueNumber, issue.Labels, "status:in-progress", runner); err != nil {
		return result, err
	}
	result.Status = "In progress"
	return result, nil
}

func OpenWorkPR(repo RepoRef, issueNumber int, dryRun bool, draft bool, runner CommandRunner) (WorkPRResult, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	issue, err := fetchDevIssue(repo, issueNumber, runner)
	if err != nil {
		return WorkPRResult{}, err
	}
	status := displayStatus(managedStatusFromLabels(issue.Labels))
	targetStatus := "In review"
	if draft {
		targetStatus = "In progress"
	}
	result := WorkPRResult{
		Repo:        repo.FullName(),
		Issue:       issueNumber,
		DryRun:      dryRun,
		Draft:       draft,
		Status:      status,
		NextStatus:  targetStatus,
		Blockers:    []string{},
		ClosingBody: fmt.Sprintf("Closes #%d", issueNumber),
	}

	prStatus, err := DevPRStatus(repo, issueNumber, runner)
	if err != nil {
		return result, err
	}
	if prStatus.PRNumber != 0 {
		actualDraft := hasWorkBlocker(prStatus.Blockers, "draft")
		targetStatus = "In review"
		if actualDraft {
			targetStatus = "In progress"
		}
		result.Draft = actualDraft
		result.NextStatus = targetStatus
		result.PRNumber = prStatus.PRNumber
		result.PRURL = prStatus.PRURL
		result.Blockers = prStatus.Blockers
		if dryRun {
			return result, nil
		}
		if err := setIssueStatus(repo, issueNumber, issue.Labels, statusLabelForDraft(actualDraft), runner); err != nil {
			return result, err
		}
		result.Status = targetStatus
		return result, nil
	}

	result.Blockers = prStatus.Blockers
	if dryRun {
		push, err := prepareWorkPRBranchPush(issue, issueNumber, dryRun, runner)
		if err != nil {
			return result, err
		}
		result.Branch = push.Branch
		result.BranchPush = push.Status
		result.PushRemote = push.Remote
		result.LocalGit = push.LocalGit
		if push.Status == "planned" {
			result.Blockers = appendMissingWorkBlocker(result.Blockers, "branch_push_required")
		}
		return result, nil
	}

	push, err := prepareWorkPRBranchPush(issue, issueNumber, dryRun, runner)
	if err != nil {
		return result, err
	}
	result.Branch = push.Branch
	result.BranchPush = push.Status
	result.PushRemote = push.Remote
	result.LocalGit = push.LocalGit

	opened, err := OpenDevPRWithOptions(repo, issueNumber, draft, runner)
	if err != nil {
		return result, err
	}
	result.PRNumber = opened.PRNumber
	result.PRURL = opened.PRURL
	result.Created = true
	result.Blockers = []string{}
	if draft {
		result.Blockers = []string{"draft"}
	}
	if err := setIssueStatus(repo, issueNumber, issue.Labels, statusLabelForDraft(draft), runner); err != nil {
		return result, err
	}
	result.Status = targetStatus
	return result, nil
}

func GetWorkStatus(repo RepoRef, issueNumber int, runner CommandRunner) (WorkStatusResult, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	var issue devStartIssue
	var prStatus DevPRStatusResult
	var issueErr error
	var prErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		issue, issueErr = fetchDevIssue(repo, issueNumber, runner)
	}()
	go func() {
		defer wg.Done()
		prStatus, prErr = DevPRStatus(repo, issueNumber, runner)
	}()
	wg.Wait()
	if issueErr != nil {
		return WorkStatusResult{}, issueErr
	}
	if prErr != nil {
		return WorkStatusResult{}, prErr
	}
	if shouldRetryWorkStatusMissingPR(issue, prStatus) {
		prStatus, prErr = retryDevPRStatusAfterMissing(repo, issueNumber, runner, prStatus, workStatusMissingPRRetryAttempts, workStatusMissingPRRetryDelay, nil)
		if prErr != nil {
			return WorkStatusResult{}, prErr
		}
	}
	return workStatusFromIssueAndPR(repo, issueNumber, issue, prStatus), nil
}

func GetWorkStatusWithPRStatus(repo RepoRef, issueNumber int, prStatus DevPRStatusResult, runner CommandRunner) (WorkStatusResult, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	issue, err := fetchDevIssue(repo, issueNumber, runner)
	if err != nil {
		return WorkStatusResult{}, err
	}
	return workStatusFromIssueAndPR(repo, issueNumber, issue, prStatus), nil
}

func workStatusFromIssueAndPR(repo RepoRef, issueNumber int, issue devStartIssue, prStatus DevPRStatusResult) WorkStatusResult {
	status := displayStatus(managedStatusFromLabels(issue.Labels))
	nextAction := nextWorkAction(issue.State, status, prStatus)
	if nextAction == "done" {
		status = "Done"
	} else if nextAction == "closed" && (status == "" || status == "null") {
		status = "Closed"
		prStatus.Blockers = nil
	}
	result := WorkStatusResult{
		Command:          "ticket status",
		SchemaVersion:    "ticket-status/v1",
		Repo:             repo.FullName(),
		Issue:            issueNumber,
		Title:            issue.Title,
		State:            issue.State,
		Status:           status,
		Labels:           append([]string(nil), issue.Labels...),
		Milestone:        issue.Milestone,
		PRNumber:         prStatus.PRNumber,
		PRURL:            prStatus.PRURL,
		PRState:          prStatus.State,
		PRLookupAttempts: prStatus.LookupAttempts,
		Blockers:         prStatus.Blockers,
		NextAction:       nextAction,
		Branch:           ticketStatusBranch(issue, prStatus),
		PullRequest:      ticketStatusPullRequest(prStatus),
		ChecksStatus:     ticketStatusChecksStatus(prStatus),
		Checks:           append([]DevPRCheck(nil), prStatus.Checks...),
		ReviewStatus:     ticketStatusReviewStatus(prStatus),
		Evidence:         ticketStatusEvidence(prStatus, nextAction),
		Acceptance:       ticketStatusAcceptance(issue.Body),
		Warnings:         ticketStatusWarnings(issue, prStatus),
	}
	result.NextStep = workStatusNextStep(result)
	return result
}

func shouldRetryWorkStatusMissingPR(issue devStartIssue, prStatus DevPRStatusResult) bool {
	return containsString(prStatus.Blockers, "missing_linked_pr") && strings.EqualFold(managedStatusFromLabels(issue.Labels), "In review")
}

func ticketStatusBranch(issue devStartIssue, pr DevPRStatusResult) *TicketStatusBranch {
	expected := formatDevBranch(DefaultDevBranchPattern, issue.Number, issue.Title)
	current := "unknown"
	source := "expected_from_issue_title"
	trusted := false
	if pr.Binding.HeadRef != "" {
		current = pr.Binding.HeadRef
		source = pr.Binding.Source
		trusted = pr.Binding.Trusted
	}
	return &TicketStatusBranch{Expected: expected, Current: current, Trusted: trusted, Source: source}
}

func ticketStatusPullRequest(pr DevPRStatusResult) *TicketStatusPullRequest {
	if pr.PRNumber == 0 {
		return &TicketStatusPullRequest{Available: false, State: "missing", Mergeable: "unknown", ReviewDecision: "unknown", HeadRefName: "unknown", BaseRefName: "unknown"}
	}
	return &TicketStatusPullRequest{
		Available:      true,
		Number:         pr.PRNumber,
		URL:            pr.PRURL,
		State:          valueOrUnknown(pr.State),
		Mergeable:      valueOrUnknown(pr.Mergeable),
		HeadRefName:    valueOrUnknown(pr.Binding.HeadRef),
		BaseRefName:    valueOrUnknown(pr.Binding.BaseRef),
		ReviewDecision: valueOrUnknown(pr.ReviewDecision),
		IsDraft:        pr.IsDraft,
	}
}

func ticketStatusChecksStatus(pr DevPRStatusResult) string {
	if len(pr.Checks) == 0 {
		return "missing"
	}
	if containsString(pr.Blockers, "checks") {
		return "failed"
	}
	if containsString(pr.Blockers, "checks_pending") {
		return "pending"
	}
	return "passed"
}

func ticketStatusReviewStatus(pr DevPRStatusResult) string {
	if pr.PRNumber == 0 {
		return "missing"
	}
	if containsString(pr.Blockers, "review") {
		return "blocked"
	}
	if strings.EqualFold(pr.ReviewDecision, "APPROVED") {
		return "approved"
	}
	if strings.TrimSpace(pr.ReviewDecision) == "" {
		return "unknown"
	}
	return strings.ToLower(pr.ReviewDecision)
}

func ticketStatusEvidence(pr DevPRStatusResult, nextAction string) *TicketStatusEvidence {
	sources := []string{}
	if pr.PRNumber > 0 {
		sources = append(sources, "closing_reference")
	}
	if pr.Binding.Trusted {
		sources = append(sources, "branch_binding")
	}
	if len(pr.Checks) > 0 {
		sources = append(sources, "checks")
	}
	if strings.TrimSpace(pr.ReviewDecision) != "" {
		sources = append(sources, "review_state")
	}
	if len(sources) == 0 {
		sources = append(sources, "none")
	}
	return &TicketStatusEvidence{
		ClosingReference: pr.PRNumber > 0,
		BranchTrusted:    pr.Binding.Trusted,
		FinishReady:      nextAction == "merge_when_policy_allows" || nextAction == "done",
		Sources:          sources,
	}
}

func ticketStatusAcceptance(body string) *TicketStatusAcceptance {
	report := &TicketStatusAcceptance{Status: "missing"}
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if !strings.HasPrefix(lower, "- [") && !strings.HasPrefix(lower, "* [") {
			continue
		}
		if len(lower) < 5 || lower[1] != ' ' || lower[2] != '[' || lower[4] != ']' {
			continue
		}
		marker := lower[3]
		switch marker {
		case 'x':
			report.Total++
			report.Complete++
		case ' ':
			report.Total++
			report.Incomplete++
		}
	}
	if report.Total == 0 {
		return report
	}
	if report.Incomplete > 0 {
		report.Status = "incomplete"
	} else {
		report.Status = "complete"
	}
	return report
}

func ticketStatusWarnings(issue devStartIssue, pr DevPRStatusResult) []string {
	warnings := []string{}
	if len(managedStatusLabels(issue.Labels)) > 1 {
		warnings = append(warnings, "multiple_status_labels")
	}
	if pr.PRNumber == 0 {
		warnings = append(warnings, "missing_linked_pr")
	}
	if acceptance := ticketStatusAcceptance(issue.Body); acceptance.Status == "missing" {
		warnings = append(warnings, "missing_acceptance_criteria")
	} else if acceptance.Status == "incomplete" {
		warnings = append(warnings, "incomplete_acceptance_criteria")
	}
	if containsString(pr.Blockers, "pr_binding") {
		warnings = append(warnings, "untrusted_pr_branch_binding")
	}
	if len(warnings) == 0 {
		return []string{}
	}
	return warnings
}

func setIssueStatus(repo RepoRef, issueNumber int, labels []string, targetLabel string, runner CommandRunner) error {
	existing := managedStatusLabels(labels)
	if len(existing) == 1 && strings.EqualFold(existing[0], targetLabel) {
		return nil
	}
	return replaceIssueStatusLabel(repo, issueNumber, existing, targetLabel, runner)
}

func statusLabelForDraft(draft bool) string {
	if draft {
		return "status:in-progress"
	}
	return "status:in-review"
}

func nextWorkAction(issueState string, status string, pr DevPRStatusResult) string {
	if strings.EqualFold(pr.State, "MERGED") {
		return "done"
	}
	if strings.EqualFold(issueState, "closed") {
		return "closed"
	}
	if pr.PRNumber == 0 {
		if status == "Blocked" {
			return "resolve_blockers"
		}
		if status == "Ready" {
			return "start_work"
		}
		if status == "In progress" {
			return "open_pr"
		}
		return "start_work"
	}
	for _, blocker := range pr.Blockers {
		switch blocker {
		case "draft":
			return "mark_pr_ready"
		case "review":
			return "address_review"
		case "checks", "checks_pending":
			return "wait_for_checks"
		}
	}
	return "merge_when_policy_allows"
}

func hasWorkBlocker(blockers []string, target string) bool {
	for _, blocker := range blockers {
		if blocker == target {
			return true
		}
	}
	return false
}

type workPRBranchPush struct {
	Branch   string
	Remote   string
	Status   string
	LocalGit string
}

func prepareWorkPRBranchPush(issue devStartIssue, issueNumber int, dryRun bool, runner CommandRunner) (workPRBranchPush, error) {
	expectedBranch := formatDevBranch(DefaultDevBranchPattern, issue.Number, issue.Title)
	currentOut, err := runner.Run("git", "branch", "--show-current")
	if err != nil {
		return workPRBranchPush{}, fmt.Errorf("read current branch before PR create: %w", err)
	}
	currentBranch := strings.TrimSpace(string(currentOut))
	if currentBranch == "" {
		return workPRBranchPush{}, fmt.Errorf("cannot create PR from detached HEAD; checkout the ticket branch, then run `gira ticket pr --apply`")
	}
	if currentBranch == "main" || currentBranch == "master" {
		return workPRBranchPush{}, fmt.Errorf("refusing to create ticket PR from %s; run `gira ticket start %d --apply` first", currentBranch, issueNumber)
	}
	if !isTicketBranchForIssue(currentBranch, issueNumber, expectedBranch) {
		return workPRBranchPush{}, fmt.Errorf("current branch %q is not the ticket branch for #%d; run `gira ticket start %d --apply` first", currentBranch, issueNumber, issueNumber)
	}
	const remote = "origin"
	if err := validateGitPushTarget(remote, currentBranch); err != nil {
		return workPRBranchPush{}, err
	}
	localGit := fmt.Sprintf("git push -u %s <validated-ticket-branch>", remote)
	if _, err := runner.Run("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"); err == nil {
		return workPRBranchPush{Branch: currentBranch, Remote: remote, Status: "skipped", LocalGit: localGit}, nil
	}
	if dryRun {
		return workPRBranchPush{Branch: currentBranch, Remote: remote, Status: "planned", LocalGit: localGit}, nil
	}
	if _, err := runner.Run("git", "push", "-u", remote, currentBranch); err != nil {
		return workPRBranchPush{Branch: currentBranch, Remote: remote, Status: "failed", LocalGit: localGit}, fmt.Errorf("push ticket branch before PR create failed; inspect local git output and credentials outside Gira logs")
	}
	return workPRBranchPush{Branch: currentBranch, Remote: remote, Status: "applied", LocalGit: localGit}, nil
}

func isTicketBranchForIssue(branch string, issueNumber int, expectedBranch string) bool {
	if branch == expectedBranch {
		return true
	}
	return strings.HasPrefix(branch, fmt.Sprintf("issue-%d-", issueNumber))
}

func validateGitPushTarget(remote string, branch string) error {
	if err := validateGitRemoteName(remote); err != nil {
		return err
	}
	if err := validateGitBranchPushName(branch); err != nil {
		return err
	}
	return nil
}

func validateGitRemoteName(remote string) error {
	if strings.TrimSpace(remote) != remote || remote == "" {
		return fmt.Errorf("invalid git push remote: expected a configured remote name")
	}
	if strings.HasPrefix(remote, "-") {
		return fmt.Errorf("invalid git push remote: remote names cannot start with '-'")
	}
	for _, r := range remote {
		if r <= 0x20 || r == 0x7f || r == ':' || r == '/' || r == '\\' {
			return fmt.Errorf("invalid git push remote: expected a configured remote name")
		}
	}
	return nil
}

func validateGitBranchPushName(branch string) error {
	if strings.TrimSpace(branch) != branch || branch == "" {
		return fmt.Errorf("invalid git push branch: expected a local ticket branch")
	}
	if strings.HasPrefix(branch, "-") {
		return fmt.Errorf("invalid git push branch: branch names cannot start with '-'")
	}
	if strings.Contains(branch, "..") ||
		strings.Contains(branch, "@{") ||
		strings.Contains(branch, "\\") ||
		strings.Contains(branch, ":") ||
		strings.Contains(branch, "//") ||
		strings.HasPrefix(branch, "/") ||
		strings.HasSuffix(branch, "/") ||
		strings.HasSuffix(branch, ".lock") {
		return fmt.Errorf("invalid git push branch: expected a plain local branch name")
	}
	for _, part := range strings.Split(branch, "/") {
		if part == "" || part == "." || strings.HasSuffix(part, ".") {
			return fmt.Errorf("invalid git push branch: expected a plain local branch name")
		}
	}
	for _, r := range branch {
		if r <= 0x20 || r == 0x7f || r == '~' || r == '^' || r == '?' || r == '*' || r == '[' {
			return fmt.Errorf("invalid git push branch: expected a plain local branch name")
		}
	}
	return nil
}

func appendMissingWorkBlocker(blockers []string, blocker string) []string {
	if hasWorkBlocker(blockers, blocker) {
		return blockers
	}
	return append(blockers, blocker)
}

func FormatWorkStart(result WorkStartResult) string {
	next := strings.TrimSpace(result.NextStep)
	if next == "" {
		next = fmt.Sprintf("gira work pr --repo %s --issue %d --dry-run", result.Repo, result.Issue)
	}
	return fmt.Sprintf(
		"work start: issue #%d branch=%s status=%s\nnext step: %s\n",
		result.Issue,
		result.Branch,
		result.NextStatus,
		next,
	)
}

func FormatWorkPR(result WorkPRResult) string {
	created := "reused"
	if result.Created {
		created = "created"
	}
	url := strings.TrimSpace(result.PRURL)
	if url == "" {
		url = "(planned)"
		created = "planned"
	}
	next := fmt.Sprintf("gira work status --repo %s --issue %d", result.Repo, result.Issue)
	if result.DryRun {
		next = fmt.Sprintf("gira work pr --repo %s --issue %d --apply", result.Repo, result.Issue)
		if result.Draft {
			next += " --draft"
		}
	}
	if result.Draft && !result.DryRun {
		next = "mark the PR ready, then " + next
	}
	branchPush := ""
	if result.BranchPush != "" && result.BranchPush != "skipped" {
		branchPush = fmt.Sprintf(" branch_push=%s", result.BranchPush)
		if result.LocalGit != "" {
			branchPush += fmt.Sprintf(" local_git=%q", result.LocalGit)
		}
	}
	return fmt.Sprintf("work pr: issue #%d pr=%s status=%s %s%s\nnext step: %s\n", result.Issue, url, result.NextStatus, created, branchPush, next)
}

func FormatWorkStatus(result WorkStatusResult) string {
	blockers := strings.Join(result.Blockers, ",")
	if blockers == "" {
		blockers = "none"
	}
	return fmt.Sprintf(
		"work status: issue #%d status=%s pr=%d blockers=%s next=%s\nnext step: %s\n",
		result.Issue,
		result.Status,
		result.PRNumber,
		blockers,
		result.NextAction,
		workStatusNextStep(result),
	)
}

func workStartNextStep(repo string, issue int, issueState string, status string, dryRun bool) string {
	if !strings.EqualFold(issueState, "open") {
		return fmt.Sprintf("gira work status --repo %s --issue %d", repo, issue)
	}
	if isMissingWorkStatus(status) {
		return readyStatusNextStep(repo, issue)
	}
	if dryRun && (status == "Ready" || status == "In progress") {
		return fmt.Sprintf("gira work start --repo %s --issue %d --apply", repo, issue)
	}
	if status == "Ready" || status == "In progress" {
		return fmt.Sprintf("gira work pr --repo %s --issue %d --dry-run", repo, issue)
	}
	return fmt.Sprintf("gira work status --repo %s --issue %d", repo, issue)
}

func workStatusNextStep(result WorkStatusResult) string {
	switch result.NextAction {
	case "start_work":
		if isMissingWorkStatus(result.Status) {
			return readyStatusNextStep(result.Repo, result.Issue)
		}
		return fmt.Sprintf("gira work start --repo %s --issue %d --apply", result.Repo, result.Issue)
	case "open_pr":
		return fmt.Sprintf("gira work pr --repo %s --issue %d --apply", result.Repo, result.Issue)
	case "resolve_blockers":
		return "resolve blockers, then set status:ready before starting work"
	case "mark_pr_ready":
		return "mark the PR ready for review"
	case "address_review":
		return "address review blockers"
	case "wait_for_checks":
		return "wait for required checks to finish or fix failing checks"
	case "merge_when_policy_allows":
		return "merge when policy checks pass"
	case "done":
		return "ticket is done"
	case "closed":
		return "ticket is closed; inspect GitHub history if more evidence is needed"
	default:
		return fmt.Sprintf("gira status --repo %s", result.Repo)
	}
}

func readyStatusNextStep(repo string, issue int) string {
	return fmt.Sprintf("gira adopt issues --repo %s --issue %d --label status:ready --apply", repo, issue)
}

func isMissingWorkStatus(status string) bool {
	trimmed := strings.TrimSpace(status)
	return trimmed == "" || strings.EqualFold(trimmed, "null")
}
