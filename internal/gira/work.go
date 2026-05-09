package gira

import (
	"fmt"
	"strings"
	"sync"
)

type WorkStartResult struct {
	Repo          string          `json:"repo"`
	Issue         int             `json:"issue"`
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

	Blockers    []string `json:"blockers"`
	ClosingBody string   `json:"closing_body"`
}

type WorkStatusResult struct {
	Repo       string   `json:"repo"`
	Issue      int      `json:"issue"`
	Title      string   `json:"title"`
	State      string   `json:"state"`
	Status     string   `json:"status"`
	PRNumber   int      `json:"pr_number,omitempty"`
	PRURL      string   `json:"pr_url,omitempty"`
	PRState    string   `json:"pr_state,omitempty"`
	Blockers   []string `json:"blockers"`
	NextAction string   `json:"next_action"`
	NextStep   string   `json:"next_step"`
}

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
		NextStep:      workStartNextStep(repo.FullName(), issueNumber, issue.State, status),
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
		Repo:       repo.FullName(),
		Issue:      issueNumber,
		Title:      issue.Title,
		State:      issue.State,
		Status:     status,
		PRNumber:   prStatus.PRNumber,
		PRURL:      prStatus.PRURL,
		PRState:    prStatus.State,
		Blockers:   prStatus.Blockers,
		NextAction: nextAction,
	}
	result.NextStep = workStatusNextStep(result)
	return result
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
	Branch string
	Status string
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
	if _, err := runner.Run("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"); err == nil {
		return workPRBranchPush{Branch: currentBranch, Status: "skipped"}, nil
	}
	if dryRun {
		return workPRBranchPush{Branch: currentBranch, Status: "planned"}, nil
	}
	if _, err := runner.Run("git", "push", "-u", "origin", currentBranch); err != nil {
		return workPRBranchPush{Branch: currentBranch, Status: "failed"}, fmt.Errorf("push ticket branch before PR create: %w", err)
	}
	return workPRBranchPush{Branch: currentBranch, Status: "applied"}, nil
}

func isTicketBranchForIssue(branch string, issueNumber int, expectedBranch string) bool {
	if branch == expectedBranch {
		return true
	}
	return strings.HasPrefix(branch, fmt.Sprintf("issue-%d-", issueNumber))
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

func workStartNextStep(repo string, issue int, issueState string, status string) string {
	if !strings.EqualFold(issueState, "open") {
		return fmt.Sprintf("gira work status --repo %s --issue %d", repo, issue)
	}
	if status != "Ready" && status != "In progress" {
		return readyStatusNextStep(repo, issue)
	}
	return fmt.Sprintf("gira work pr --repo %s --issue %d --dry-run", repo, issue)
}

func workStatusNextStep(result WorkStatusResult) string {
	switch result.NextAction {
	case "start_work":
		if result.Status != "Ready" {
			return readyStatusNextStep(result.Repo, result.Issue)
		}
		return fmt.Sprintf("gira work start --repo %s --issue %d --apply", result.Repo, result.Issue)
	case "open_pr":
		return fmt.Sprintf("gira work pr --repo %s --issue %d --apply", result.Repo, result.Issue)
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
