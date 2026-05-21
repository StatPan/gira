package gira

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const DefaultDevBranchPattern = "issue-%d-%s"

type DevStartResult struct {
	Repo     string            `json:"repo"`
	Issue    int               `json:"issue"`
	Title    string            `json:"title"`
	Branch   string            `json:"branch"`
	Base     string            `json:"base,omitempty"`
	DryRun   bool              `json:"dry_run"`
	Created  bool              `json:"created"`
	Checked  map[string]bool   `json:"checks"`
	Failures map[string]string `json:"failures,omitempty"`
}

type DevStartOptions struct {
	Pattern              string
	Base                 string
	DryRun               bool
	Force                bool
	RequireCleanWorktree bool
}

type devStartIssue struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	State     string   `json:"state"`
	Body      string   `json:"body"`
	Labels    []string `json:"labels"`
	Milestone string   `json:"milestone,omitempty"`
	IsPR      bool     `json:"is_pr"`
}

func StartDevBranch(repo RepoRef, issueNumber int, pattern string, dryRun bool, force bool, runner CommandRunner) (DevStartResult, error) {
	return StartDevBranchWithOptions(repo, issueNumber, DevStartOptions{Pattern: pattern, DryRun: dryRun, Force: force}, runner)
}

func StartDevBranchWithOptions(repo RepoRef, issueNumber int, options DevStartOptions, runner CommandRunner) (DevStartResult, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if issueNumber <= 0 {
		return DevStartResult{}, fmt.Errorf("issue must be > 0")
	}
	pattern := options.Pattern
	if strings.TrimSpace(pattern) == "" {
		pattern = DefaultDevBranchPattern
	}
	base := strings.TrimSpace(options.Base)
	issue, err := fetchDevIssue(repo, issueNumber, runner)
	if err != nil {
		return DevStartResult{}, err
	}
	branch := formatDevBranch(pattern, issue.Number, issue.Title)
	result := DevStartResult{Repo: repo.FullName(), Issue: issue.Number, Title: issue.Title, Branch: branch, Base: base, DryRun: options.DryRun, Checked: map[string]bool{}, Failures: map[string]string{}}

	result.Checked["issue_exists"] = true
	if issue.IsPR {
		result.Failures["issue_type"] = "pull_request"
		return result, fmt.Errorf("issue #%d resolves to a pull request", issueNumber)
	}
	result.Checked["issue_open"] = strings.EqualFold(issue.State, "open")
	if !result.Checked["issue_open"] && !options.Force {
		result.Failures["issue_open"] = "not_open"
		return result, fmt.Errorf("issue #%d is not open", issue.Number)
	}
	result.Checked["ready_label"] = hasReadyLabel(issue.Labels)
	if !result.Checked["ready_label"] && !options.Force {
		result.Failures["ready_label"] = "missing_status:ready"
		return result, fmt.Errorf("issue #%d is not ready for start: missing label status:ready; try `gira adopt issues --repo %s --issue %d --label status:ready --apply` after confirming the issue is executable", issue.Number, repo.FullName(), issue.Number)
	}

	if base != "" {
		if err := validateGitBranchPushName(base); err != nil {
			result.Failures["base_branch"] = "invalid"
			return result, fmt.Errorf("invalid base branch: %w", err)
		}
		if err := validateRemoteBranchExists("origin", base, runner); err != nil {
			result.Failures["base_branch"] = "missing"
			return result, err
		}
		result.Checked["base_branch_exists"] = true
	}

	localExists, err := gitLocalBranchExists(branch, runner)
	if err != nil {
		return result, err
	}
	result.Checked["local_branch_absent_or_reusable"] = true

	remoteExists, err := gitRemoteBranchExists(branch, runner)
	if err != nil {
		return result, err
	}
	result.Checked["remote_branch_absent"] = !remoteExists
	if remoteExists && !localExists {
		result.Failures["branch_conflict"] = "remote_exists"
		return result, fmt.Errorf("branch conflict: remote branch %q already exists", branch)
	}

	if options.DryRun {
		return result, nil
	}

	if options.RequireCleanWorktree {
		if err := ensureCleanWorktreeBeforeBranchMutation(runner); err != nil {
			result.Failures["worktree"] = "dirty"
			return result, err
		}
	}
	if localExists {
		if _, err := runner.Run("git", "checkout", branch); err != nil {
			return result, err
		}
		return result, nil
	}

	args := []string{"checkout", "-b", branch}
	if base != "" {
		args = append(args, "origin/"+base)
	}
	if _, err := runner.Run("git", args...); err != nil {
		return result, err
	}
	result.Created = true
	return result, nil
}

func fetchDevIssue(repo RepoRef, issueNumber int, runner CommandRunner) (devStartIssue, error) {
	output, err := runner.Run("gh", "api", "repos/"+repo.FullName()+"/issues/"+strconv.Itoa(issueNumber))
	if err != nil {
		return devStartIssue{}, fmt.Errorf("fetch issue: %w", err)
	}
	var raw struct {
		Number      int     `json:"number"`
		Title       string  `json:"title"`
		State       string  `json:"state"`
		Body        *string `json:"body"`
		PullRequest *any    `json:"pull_request"`
		Labels      []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Milestone *struct {
			Title string `json:"title"`
		} `json:"milestone"`
	}
	if err := json.Unmarshal(output, &raw); err != nil {
		return devStartIssue{}, fmt.Errorf("parse issue JSON: %w", err)
	}
	if raw.Number <= 0 {
		return devStartIssue{}, fmt.Errorf("parse issue JSON: missing or invalid issue number")
	}
	if raw.Number != issueNumber {
		return devStartIssue{}, fmt.Errorf("parse issue JSON: expected issue #%d, got #%d", issueNumber, raw.Number)
	}
	labels := make([]string, 0, len(raw.Labels))
	for _, label := range raw.Labels {
		labels = append(labels, label.Name)
	}
	body := ""
	if raw.Body != nil {
		body = *raw.Body
	}
	milestone := ""
	if raw.Milestone != nil {
		milestone = raw.Milestone.Title
	}
	return devStartIssue{Number: raw.Number, Title: raw.Title, State: raw.State, Body: body, Labels: labels, Milestone: milestone, IsPR: raw.PullRequest != nil}, nil
}

func hasReadyLabel(labels []string) bool {
	for _, label := range labels {
		if strings.EqualFold(strings.TrimSpace(label), "status:ready") {
			return true
		}
	}
	return false
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func formatDevBranch(pattern string, issue int, title string) string {
	title = strings.ToLower(strings.TrimSpace(title))
	title = nonAlnum.ReplaceAllString(title, "-")
	title = strings.Trim(title, "-")
	if title == "" {
		title = "issue"
	}
	return fmt.Sprintf(pattern, issue, title)
}

func gitLocalBranchExists(branch string, runner CommandRunner) (bool, error) {
	if _, err := runner.Run("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err != nil {
		if strings.Contains(err.Error(), "exit status 1") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func gitRemoteBranchExists(branch string, runner CommandRunner) (bool, error) {
	if _, err := runner.Run("git", "ls-remote", "--exit-code", "--heads", "origin", branch); err != nil {
		if strings.Contains(err.Error(), "exit status 2") {
			return false, nil
		}
		if strings.Contains(err.Error(), "exit status 1") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func validateRemoteBranchExists(remote string, branch string, runner CommandRunner) error {
	if err := validateGitRemoteName(remote); err != nil {
		return err
	}
	if _, err := runner.Run("git", "ls-remote", "--exit-code", "--heads", remote, branch); err != nil {
		if strings.Contains(err.Error(), "exit status 2") || strings.Contains(err.Error(), "exit status 1") {
			return fmt.Errorf("base branch %q does not exist on %s", branch, remote)
		}
		return fmt.Errorf("validate base branch %q on %s: %w", branch, remote, err)
	}
	return nil
}

func ensureCleanWorktreeBeforeBranchMutation(runner CommandRunner) error {
	out, err := runner.Run("git", "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("check worktree before branch mutation: %w", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("dirty worktree before branch mutation; commit, stash, or intentionally clear local changes before running ticket start")
	}
	return nil
}
