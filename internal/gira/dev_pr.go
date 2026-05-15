package gira

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type DevPROpenResult struct {
	Repo     string `json:"repo"`
	Issue    int    `json:"issue"`
	PRNumber int    `json:"pr_number"`
	PRURL    string `json:"pr_url"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	Draft    bool   `json:"draft"`
}

type DevPRStatusResult struct {
	Repo      string       `json:"repo"`
	Issue     int          `json:"issue"`
	PRNumber  int          `json:"pr_number,omitempty"`
	PRURL     string       `json:"pr_url,omitempty"`
	State     string       `json:"state,omitempty"`
	Mergeable string       `json:"mergeable,omitempty"`
	Binding   DevPRBinding `json:"binding,omitempty"`
	Blockers  []string     `json:"blockers"`
	Checks    []DevPRCheck `json:"checks,omitempty"`
	Ready     bool         `json:"ready"`
}

type DevPRBinding struct {
	Trusted              bool     `json:"trusted"`
	Source               string   `json:"source"`
	HeadRef              string   `json:"head_ref,omitempty"`
	BaseRef              string   `json:"base_ref,omitempty"`
	ExpectedHeadPrefixes []string `json:"expected_head_prefixes,omitempty"`
	Blockers             []string `json:"blockers,omitempty"`
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

func OpenDevPR(repo RepoRef, issueNumber int, runner CommandRunner) (DevPROpenResult, error) {
	return OpenDevPRWithOptions(repo, issueNumber, false, runner)
}

func OpenDevPRWithOptions(repo RepoRef, issueNumber int, draft bool, runner CommandRunner) (DevPROpenResult, error) {
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
	if draft {
		args = append(args, "--draft")
	}
	out, err := runner.Run("gh", args...)
	if err != nil {
		return DevPROpenResult{}, err
	}
	url := strings.TrimSpace(string(out))
	prNumber := extractPRNumber(url)
	return DevPROpenResult{Repo: repo.FullName(), Issue: issueNumber, PRNumber: prNumber, PRURL: url, Title: title, Body: body, Draft: draft}, nil
}

func DevPRStatus(repo RepoRef, issueNumber int, runner CommandRunner) (DevPRStatusResult, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
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
			result.Binding = validateDevPRBinding(issueNumber, pr)
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

func validateDevPRBinding(issueNumber int, pr prSummary) DevPRBinding {
	expected := []string{fmt.Sprintf("issue-%d-", issueNumber), fmt.Sprintf("issue-%d", issueNumber)}
	binding := DevPRBinding{
		Source:               "closing_keyword_and_branch",
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
	for _, prefix := range expected {
		if binding.HeadRef == prefix || strings.HasPrefix(binding.HeadRef, prefix) {
			binding.Trusted = true
			return binding
		}
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
