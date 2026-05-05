package gira

import (
	"fmt"
	"strings"
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
	Checks        map[string]bool `json:"checks"`
}

type WorkPRResult struct {
	Repo        string   `json:"repo"`
	Issue       int      `json:"issue"`
	DryRun      bool     `json:"dry_run"`
	Draft       bool     `json:"draft"`
	PRNumber    int      `json:"pr_number,omitempty"`
	PRURL       string   `json:"pr_url,omitempty"`
	Created     bool     `json:"created"`
	Status      string   `json:"status"`
	NextStatus  string   `json:"next_status"`
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
		return result, nil
	}

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
	issue, err := fetchDevIssue(repo, issueNumber, runner)
	if err != nil {
		return WorkStatusResult{}, err
	}
	prStatus, err := DevPRStatus(repo, issueNumber, runner)
	if err != nil {
		return WorkStatusResult{}, err
	}
	status := displayStatus(managedStatusFromLabels(issue.Labels))
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
		NextAction: nextWorkAction(status, prStatus),
	}
	return result, nil
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

func nextWorkAction(status string, pr DevPRStatusResult) string {
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

func FormatWorkStart(result WorkStartResult) string {
	return fmt.Sprintf(
		"work start: issue #%d branch=%s status=%s\nnext step: gira work pr --repo %s --issue %d --dry-run\n",
		result.Issue,
		result.Branch,
		result.NextStatus,
		result.Repo,
		result.Issue,
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
	}
	next := fmt.Sprintf("gira work status --repo %s --issue %d", result.Repo, result.Issue)
	if result.Draft {
		next = "mark the PR ready, then " + next
	}
	return fmt.Sprintf("work pr: issue #%d pr=%s status=%s %s\nnext step: %s\n", result.Issue, url, result.NextStatus, created, next)
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

func workStatusNextStep(result WorkStatusResult) string {
	switch result.NextAction {
	case "start_work":
		return fmt.Sprintf("gira work start --repo %s --issue %d --apply", result.Repo, result.Issue)
	case "open_pr":
		return fmt.Sprintf("gira work pr --repo %s --issue %d --apply", result.Repo, result.Issue)
	case "mark_pr_ready":
		return "mark the PR ready for review"
	case "address_review":
		return "address review blockers"
	case "merge_when_policy_allows":
		return "merge when policy checks pass"
	default:
		return fmt.Sprintf("gira status --repo %s", result.Repo)
	}
}
