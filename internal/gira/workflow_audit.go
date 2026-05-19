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
	ID                string   `json:"id"`
	Severity          string   `json:"severity"`
	Kind              string   `json:"kind,omitempty"`
	IssueNumber       int      `json:"issue_number,omitempty"`
	PRNumber          int      `json:"pr_number,omitempty"`
	Title             string   `json:"title,omitempty"`
	State             string   `json:"state,omitempty"`
	Labels            []string `json:"labels,omitempty"`
	CurrentState      string   `json:"current_state,omitempty"`
	ExpectedState     string   `json:"expected_state,omitempty"`
	Evidence          []string `json:"evidence,omitempty"`
	Detail            string   `json:"detail"`
	Remediation       string   `json:"remediation"`
	RecommendedAction string   `json:"recommended_action,omitempty"`
}

type workflowAuditIssue struct {
	Number int
	Title  string
	State  string
	Labels []string
	Body   string
}

type workflowAuditPR struct {
	Number         int
	Title          string
	Body           string
	State          string
	MergedAt       string
	ReviewDecision string
	Checks         []DevPRCheck
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
		Command:   "audit drift",
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
	output, err := runner.Run("gh", "pr", "list", "--repo", repo.FullName(), "--state", "all", "--limit", "1000", "--json", "number,title,body,state,mergedAt,reviewDecision,statusCheckRollup")
	if err != nil {
		output, err = runner.Run("gh", "pr", "list", "--repo", repo.FullName(), "--state", "all", "--limit", "1000", "--json", "number,title,body,state,mergedAt")
		if err != nil {
			return nil, fmt.Errorf("fetch workflow prs: %w", err)
		}
	}
	var rows []struct {
		Number         int    `json:"number"`
		Title          string `json:"title"`
		Body           string `json:"body"`
		State          string `json:"state"`
		MergedAt       string `json:"mergedAt"`
		ReviewDecision string `json:"reviewDecision"`
		StatusRollup   []struct {
			Name       string `json:"name"`
			Workflow   string `json:"workflowName"`
			Conclusion string `json:"conclusion"`
			Status     string `json:"status"`
			URL        string `json:"detailsUrl"`
		} `json:"statusCheckRollup"`
	}
	if err := json.Unmarshal(output, &rows); err != nil {
		return nil, fmt.Errorf("parse workflow prs: %w", err)
	}
	prs := make([]workflowAuditPR, 0, len(rows))
	for _, row := range rows {
		checks := make([]DevPRCheck, 0, len(row.StatusRollup))
		for _, check := range row.StatusRollup {
			checks = append(checks, DevPRCheck{Name: check.Name, Workflow: check.Workflow, Status: check.Status, Conclusion: check.Conclusion, URL: check.URL, State: classifyDevPRCheck(check.Status, check.Conclusion)})
		}
		prs = append(prs, workflowAuditPR{Number: row.Number, Title: row.Title, Body: row.Body, State: strings.ToLower(row.State), MergedAt: row.MergedAt, ReviewDecision: row.ReviewDecision, Checks: checks})
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
		telemetry := ticketStatusTelemetry(issue.Body, issue.Labels)
		if telemetry.Required && !telemetry.Present {
			findings = append(findings, WorkflowAuditFinding{
				ID:                "missing_ai_delivery_telemetry",
				Severity:          "warn",
				Kind:              "telemetry",
				IssueNumber:       issue.Number,
				Title:             issue.Title,
				State:             issue.State,
				Labels:            append([]string(nil), issue.Labels...),
				CurrentState:      "telemetry=missing",
				ExpectedState:     "telemetry=present",
				Evidence:          telemetry.Sources,
				Detail:            "agent-routed issue is missing AI Delivery Telemetry or Gira provenance evidence",
				Remediation:       "add an AI Delivery Telemetry or Gira provenance block when historical execution attribution matters",
				RecommendedAction: "add telemetry/provenance evidence",
			})
		}
		if workflowIssueFinished(issue) && len(linkedPRs[issue.Number]) == 0 && !telemetry.Present {
			findings = append(findings, WorkflowAuditFinding{
				ID:                "finished_issue_missing_evidence",
				Severity:          "warn",
				Kind:              "evidence",
				IssueNumber:       issue.Number,
				Title:             issue.Title,
				State:             issue.State,
				Labels:            append([]string(nil), issue.Labels...),
				CurrentState:      "finished_without_closing_pr_or_telemetry",
				ExpectedState:     "closing_pr_or_finish_receipt_or_provenance",
				Evidence:          []string{"missing_closing_pr", "missing_telemetry"},
				Detail:            "finished issue has no linked PR or telemetry evidence marker",
				Remediation:       "add a finish receipt/provenance note or link the completing PR",
				RecommendedAction: "attach completion evidence",
			})
		}
	}
	for _, pr := range prs {
		for _, issueNumber := range ExtractClosureIssueNumbers(pr.Body) {
			issue, ok := byIssue[issueNumber]
			if !ok || !workflowIssueFinished(issue) {
				continue
			}
			if workflowPRChecksStatus(pr.Checks) == "failed" {
				findings = append(findings, workflowPRCheckFinding(pr, issue, "finished_issue_failed_checks", "failed checks on a ticket marked done"))
			}
			if workflowPRChecksStatus(pr.Checks) == "pending" {
				findings = append(findings, workflowPRCheckFinding(pr, issue, "finished_issue_pending_checks", "pending checks on a ticket marked done"))
			}
		}
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
					ID:                "merged_pr_issue_not_converged",
					Severity:          "fail",
					Kind:              "convergence",
					IssueNumber:       issue.Number,
					PRNumber:          pr.Number,
					Title:             issue.Title,
					State:             issue.State,
					Labels:            append([]string(nil), issue.Labels...),
					CurrentState:      fmt.Sprintf("issue=%s labels=%s", issue.State, strings.Join(issue.Labels, ",")),
					ExpectedState:     "issue=closed status:done/no-active-status",
					Evidence:          []string{fmt.Sprintf("merged_pr#%d", pr.Number), "closing_reference"},
					Detail:            fmt.Sprintf("merged PR #%d closes issue #%d, but issue state/status has not converged", pr.Number, issue.Number),
					Remediation:       fmt.Sprintf("run `gira ticket finish %d --repo %s --dry-run` or normalize status with `gira adopt issues --repo %s --issues %d --normalize-status --dry-run`", issue.Number, repo.FullName(), repo.FullName(), issue.Number),
					RecommendedAction: "normalize issue status",
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
	remediation := fmt.Sprintf("run `gira adopt issues --repo %s --issues %d --normalize-status --dry-run` when this is status drift, or update the issue body/labels manually", repo.FullName(), issue.Number)
	return WorkflowAuditFinding{
		ID:                id,
		Severity:          "fail",
		Kind:              "issue_status",
		IssueNumber:       issue.Number,
		Title:             issue.Title,
		State:             issue.State,
		Labels:            append([]string(nil), labels...),
		CurrentState:      fmt.Sprintf("issue=%s labels=%s", issue.State, strings.Join(labels, ",")),
		ExpectedState:     "workflow labels converge with issue/PR state",
		Evidence:          []string{"issue_state", "labels"},
		Detail:            detail,
		Remediation:       remediation,
		RecommendedAction: remediation,
	}
}

func workflowIssueFinished(issue workflowAuditIssue) bool {
	return strings.EqualFold(issue.State, "closed") || hasLabel(issue.Labels, "status:done")
}

func workflowPRChecksStatus(checks []DevPRCheck) string {
	for _, check := range checks {
		if check.State == "failing" {
			return "failed"
		}
	}
	for _, check := range checks {
		if check.State == "pending" {
			return "pending"
		}
	}
	if len(checks) == 0 {
		return "missing"
	}
	return "passed"
}

func workflowPRCheckFinding(pr workflowAuditPR, issue workflowAuditIssue, id string, detail string) WorkflowAuditFinding {
	status := workflowPRChecksStatus(pr.Checks)
	return WorkflowAuditFinding{
		ID:                id,
		Severity:          "fail",
		Kind:              "checks",
		IssueNumber:       issue.Number,
		PRNumber:          pr.Number,
		Title:             issue.Title,
		State:             issue.State,
		Labels:            append([]string(nil), issue.Labels...),
		CurrentState:      "checks=" + status,
		ExpectedState:     "checks=passed",
		Evidence:          []string{fmt.Sprintf("pr#%d", pr.Number), "statusCheckRollup"},
		Detail:            detail,
		Remediation:       fmt.Sprintf("inspect PR #%d checks before treating issue #%d as complete", pr.Number, issue.Number),
		RecommendedAction: "repair checks or reopen the ticket",
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
	command := strings.TrimSpace(report.Command)
	if command == "" {
		command = "audit drift"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s\n", command, verdict)
	fmt.Fprintf(&b, "repo: %s\n", report.Repo)
	fmt.Fprintf(&b, "scanned: issues=%d prs=%d findings=%d\n", report.Counts.IssuesScanned, report.Counts.PRsScanned, report.Counts.Findings)
	lastSeverity := ""
	for _, finding := range report.Findings {
		if finding.Severity != lastSeverity {
			lastSeverity = finding.Severity
			fmt.Fprintf(&b, "%s findings:\n", lastSeverity)
		}
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
