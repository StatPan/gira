package gira

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type WorkflowAuditReport struct {
	Repo      string                 `json:"repo"`
	Command   string                 `json:"command"`
	CheckedAt string                 `json:"checked_at"`
	Ready     bool                   `json:"ready"`
	Counts    WorkflowAuditCounts    `json:"counts"`
	Findings  []WorkflowAuditFinding `json:"findings"`
	NextStep  string                 `json:"next_step"`
}

type WorkflowAuditCounts struct {
	IssuesScanned int `json:"issues_scanned"`
	PRsScanned    int `json:"prs_scanned"`
	Findings      int `json:"findings"`
}

type WorkflowAuditFinding struct {
	ID          string   `json:"id"`
	Severity    string   `json:"severity"`
	IssueNumber int      `json:"issue_number,omitempty"`
	PRNumber    int      `json:"pr_number,omitempty"`
	Title       string   `json:"title,omitempty"`
	State       string   `json:"state,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	Detail      string   `json:"detail"`
	Remediation string   `json:"remediation"`
}

type workflowAuditIssue struct {
	Number int
	Title  string
	State  string
	Labels []string
	Body   string
}

type workflowAuditPR struct {
	Number   int
	Title    string
	Body     string
	State    string
	MergedAt string
}

func BuildWorkflowAuditReport(repo RepoRef, runner CommandRunner, checkedAt time.Time) (WorkflowAuditReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	issues, err := fetchWorkflowAuditIssues(repo, runner)
	if err != nil {
		return WorkflowAuditReport{}, err
	}
	prs, err := fetchWorkflowAuditPRs(repo, runner)
	if err != nil {
		return WorkflowAuditReport{}, err
	}
	statusDoneExists, err := repoHasLabel(repo, "status:done", runner)
	if err != nil {
		return WorkflowAuditReport{}, err
	}
	findings := workflowAuditFindings(repo, issues, prs, statusDoneExists)
	report := WorkflowAuditReport{
		Repo:      repo.FullName(),
		Command:   "audit workflow",
		CheckedAt: checkedAt.UTC().Format(time.RFC3339),
		Ready:     workflowAuditFailureCount(findings) == 0,
		Counts: WorkflowAuditCounts{
			IssuesScanned: len(issues),
			PRsScanned:    len(prs),
			Findings:      len(findings),
		},
		Findings: findings,
		NextStep: workflowAuditNextStep(repo, findings),
	}
	return report, nil
}

func fetchWorkflowAuditIssues(repo RepoRef, runner CommandRunner) ([]workflowAuditIssue, error) {
	output, err := runner.Run("gh", "issue", "list", "--repo", repo.FullName(), "--state", "all", "--limit", "1000", "--json", "number,title,state,labels,body")
	if err != nil {
		return nil, fmt.Errorf("fetch workflow issues: %w", err)
	}
	var rows []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		State  string `json:"state"`
		Body   string `json:"body"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.Unmarshal(output, &rows); err != nil {
		return nil, fmt.Errorf("parse workflow issues: %w", err)
	}
	issues := make([]workflowAuditIssue, 0, len(rows))
	for _, row := range rows {
		labels := make([]string, 0, len(row.Labels))
		for _, label := range row.Labels {
			labels = append(labels, strings.TrimSpace(label.Name))
		}
		sort.Strings(labels)
		issues = append(issues, workflowAuditIssue{Number: row.Number, Title: row.Title, State: strings.ToLower(row.State), Labels: labels, Body: row.Body})
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].Number < issues[j].Number })
	return issues, nil
}

func fetchWorkflowAuditPRs(repo RepoRef, runner CommandRunner) ([]workflowAuditPR, error) {
	output, err := runner.Run("gh", "pr", "list", "--repo", repo.FullName(), "--state", "all", "--limit", "1000", "--json", "number,title,body,state,mergedAt")
	if err != nil {
		return nil, fmt.Errorf("fetch workflow prs: %w", err)
	}
	var rows []struct {
		Number   int    `json:"number"`
		Title    string `json:"title"`
		Body     string `json:"body"`
		State    string `json:"state"`
		MergedAt string `json:"mergedAt"`
	}
	if err := json.Unmarshal(output, &rows); err != nil {
		return nil, fmt.Errorf("parse workflow prs: %w", err)
	}
	prs := make([]workflowAuditPR, 0, len(rows))
	for _, row := range rows {
		prs = append(prs, workflowAuditPR{Number: row.Number, Title: row.Title, Body: row.Body, State: strings.ToLower(row.State), MergedAt: row.MergedAt})
	}
	sort.Slice(prs, func(i, j int) bool { return prs[i].Number < prs[j].Number })
	return prs, nil
}

func workflowAuditFindings(repo RepoRef, issues []workflowAuditIssue, prs []workflowAuditPR, statusDoneExists bool) []WorkflowAuditFinding {
	findings := []WorkflowAuditFinding{}
	byIssue := make(map[int]workflowAuditIssue, len(issues))
	linkedPRs := map[int][]int{}
	for _, issue := range issues {
		byIssue[issue.Number] = issue
	}
	for _, pr := range prs {
		for _, issueNumber := range ExtractClosureIssueNumbers(pr.Body) {
			linkedPRs[issueNumber] = append(linkedPRs[issueNumber], pr.Number)
		}
	}
	for _, issue := range issues {
		active := activeStatusLabels(issue.Labels)
		managed := managedStatusLabels(issue.Labels)
		if strings.EqualFold(issue.State, "closed") && len(active) > 0 {
			findings = append(findings, workflowIssueFinding(repo, "closed_issue_active_status", issue, active, "closed issue retains active workflow status labels"))
		}
		if strings.EqualFold(issue.State, "closed") && statusDoneExists && len(managed) > 0 && !hasLabel(issue.Labels, "status:done") {
			findings = append(findings, workflowIssueFinding(repo, "closed_issue_missing_done", issue, issue.Labels, "closed issue is missing status:done"))
		}
		if strings.EqualFold(issue.State, "open") && hasLabel(issue.Labels, "status:done") {
			findings = append(findings, workflowIssueFinding(repo, "open_issue_done_status", issue, issue.Labels, "open issue has terminal status:done"))
		}
		if len(managed) > 1 {
			findings = append(findings, workflowIssueFinding(repo, "multiple_status_labels", issue, managed, "issue has multiple managed status labels"))
		}
		if strings.EqualFold(issue.State, "open") && hasLabel(issue.Labels, "status:in-review") && len(linkedPRs[issue.Number]) == 0 {
			findings = append(findings, workflowIssueFinding(repo, "in_review_without_linked_pr", issue, issue.Labels, "issue is in review but no closing PR reference was found"))
		}
		if hasLabel(issue.Labels, "agent:worker") && extractProvenanceBlock(issue.Body) == "" {
			findings = append(findings, WorkflowAuditFinding{
				ID:          "missing_provenance",
				Severity:    "warn",
				IssueNumber: issue.Number,
				Title:       issue.Title,
				State:       issue.State,
				Labels:      append([]string(nil), issue.Labels...),
				Detail:      "agent-routed issue is missing a Gira provenance block",
				Remediation: "add a Gira provenance block to the issue body when historical execution attribution matters",
			})
		}
	}
	for _, pr := range prs {
		if !workflowPRMerged(pr) {
			continue
		}
		for _, issueNumber := range ExtractClosureIssueNumbers(pr.Body) {
			issue, ok := byIssue[issueNumber]
			if !ok {
				continue
			}
			active := activeStatusLabels(issue.Labels)
			managed := managedStatusLabels(issue.Labels)
			if !strings.EqualFold(issue.State, "closed") || len(active) > 0 || (statusDoneExists && len(managed) > 0 && !hasLabel(issue.Labels, "status:done")) {
				findings = append(findings, WorkflowAuditFinding{
					ID:          "merged_pr_issue_not_converged",
					Severity:    "fail",
					IssueNumber: issue.Number,
					PRNumber:    pr.Number,
					Title:       issue.Title,
					State:       issue.State,
					Labels:      append([]string(nil), issue.Labels...),
					Detail:      fmt.Sprintf("merged PR #%d closes issue #%d, but issue state/status has not converged", pr.Number, issue.Number),
					Remediation: fmt.Sprintf("run `gira ticket finish %d --repo %s --dry-run` or normalize status with `gira adopt issues --repo %s --issues %d --normalize-status --dry-run`", issue.Number, repo.FullName(), repo.FullName(), issue.Number),
				})
			}
		}
	}
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].IssueNumber != findings[j].IssueNumber {
			return findings[i].IssueNumber < findings[j].IssueNumber
		}
		if findings[i].PRNumber != findings[j].PRNumber {
			return findings[i].PRNumber < findings[j].PRNumber
		}
		return findings[i].ID < findings[j].ID
	})
	return findings
}

func workflowIssueFinding(repo RepoRef, id string, issue workflowAuditIssue, labels []string, detail string) WorkflowAuditFinding {
	return WorkflowAuditFinding{
		ID:          id,
		Severity:    "fail",
		IssueNumber: issue.Number,
		Title:       issue.Title,
		State:       issue.State,
		Labels:      append([]string(nil), labels...),
		Detail:      detail,
		Remediation: fmt.Sprintf("run `gira adopt issues --repo %s --issues %d --normalize-status --dry-run` when this is status drift, or update the issue body/labels manually", repo.FullName(), issue.Number),
	}
}

func workflowPRMerged(pr workflowAuditPR) bool {
	return strings.EqualFold(pr.State, "merged") || strings.TrimSpace(pr.MergedAt) != ""
}

func workflowAuditNextStep(repo RepoRef, findings []WorkflowAuditFinding) string {
	if len(findings) == 0 {
		return "workflow contract is converged"
	}
	issueNumbers := make([]int, 0)
	seen := map[int]struct{}{}
	for _, finding := range findings {
		if !workflowFindingStatusNormalizable(finding.ID) {
			continue
		}
		if finding.IssueNumber <= 0 {
			continue
		}
		if _, ok := seen[finding.IssueNumber]; ok {
			continue
		}
		seen[finding.IssueNumber] = struct{}{}
		issueNumbers = append(issueNumbers, finding.IssueNumber)
	}
	sort.Ints(issueNumbers)
	if len(issueNumbers) == 0 {
		return fmt.Sprintf("inspect workflow findings, update provenance/evidence manually where needed, then rerun `gira audit workflow --repo %s`", repo.FullName())
	}
	return fmt.Sprintf("gira adopt issues --repo %s --state all --issues %s --normalize-status --dry-run", repo.FullName(), joinIssueNumbers(issueNumbers))
}

func workflowFindingStatusNormalizable(id string) bool {
	switch id {
	case "closed_issue_active_status", "open_issue_done_status", "multiple_status_labels", "merged_pr_issue_not_converged":
		return true
	default:
		return false
	}
}

func workflowAuditFailureCount(findings []WorkflowAuditFinding) int {
	count := 0
	for _, finding := range findings {
		if strings.EqualFold(finding.Severity, "fail") {
			count++
		}
	}
	return count
}

func formatWorkflowFindingSummary(findings []WorkflowAuditFinding) string {
	ordered := append([]WorkflowAuditFinding(nil), findings...)
	sort.SliceStable(ordered, func(i, j int) bool {
		leftFail := strings.EqualFold(ordered[i].Severity, "fail")
		rightFail := strings.EqualFold(ordered[j].Severity, "fail")
		if leftFail != rightFail {
			return leftFail
		}
		if ordered[i].IssueNumber != ordered[j].IssueNumber {
			return ordered[i].IssueNumber < ordered[j].IssueNumber
		}
		if ordered[i].PRNumber != ordered[j].PRNumber {
			return ordered[i].PRNumber < ordered[j].PRNumber
		}
		return ordered[i].ID < ordered[j].ID
	})
	parts := []string{}
	limit := minInt(len(ordered), 5)
	for i := 0; i < limit; i++ {
		finding := ordered[i]
		target := ""
		if finding.IssueNumber > 0 {
			target = "#" + strconv.Itoa(finding.IssueNumber)
		}
		if finding.PRNumber > 0 {
			target += "/PR#" + strconv.Itoa(finding.PRNumber)
		}
		parts = append(parts, strings.TrimSpace(target+" "+finding.ID))
	}
	if len(ordered) > limit {
		parts = append(parts, fmt.Sprintf("... +%d more", len(ordered)-limit))
	}
	return strings.Join(parts, "; ")
}

func FormatWorkflowAuditReport(report WorkflowAuditReport) string {
	verdict := "READY"
	if !report.Ready {
		verdict = "NOT READY"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "audit workflow: %s\n", verdict)
	fmt.Fprintf(&b, "repo: %s\n", report.Repo)
	fmt.Fprintf(&b, "scanned: issues=%d prs=%d findings=%d\n", report.Counts.IssuesScanned, report.Counts.PRsScanned, report.Counts.Findings)
	for _, finding := range report.Findings {
		target := ""
		if finding.IssueNumber > 0 {
			target = fmt.Sprintf(" issue=#%d", finding.IssueNumber)
		}
		if finding.PRNumber > 0 {
			target += fmt.Sprintf(" pr=#%d", finding.PRNumber)
		}
		fmt.Fprintf(&b, "- [%s] %s%s: %s\n", finding.Severity, finding.ID, target, finding.Detail)
		if strings.TrimSpace(finding.Remediation) != "" {
			fmt.Fprintf(&b, "  remediation: %s\n", finding.Remediation)
		}
	}
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}
