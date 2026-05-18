package gira

import (
	"strings"
	"testing"
	"time"
)

func TestBuildWorkflowAuditReportDetectsDrift(t *testing.T) {
	repo := mustRepo(t, "StatPan/gira")
	runner := onboardFakeRunner{
		responses: map[string]string{
			"gh issue list --repo StatPan/gira --state all --limit 1000 --json number,title,state,labels,body": `[
				{"number":1,"title":"Closed active","state":"CLOSED","body":"","labels":[{"name":"status:in-review"},{"name":"agent:worker"}]},
				{"number":2,"title":"Open done","state":"OPEN","body":"<!-- gira:provenance:start -->\nplanning: human\nimplementation:\nreview:\n<!-- gira:provenance:end -->","labels":[{"name":"status:done"}]},
				{"number":3,"title":"Multiple","state":"OPEN","body":"","labels":[{"name":"status:ready"},{"name":"status:blocked"}]},
				{"number":4,"title":"Review no PR","state":"OPEN","body":"","labels":[{"name":"status:in-review"}]},
				{"number":5,"title":"Merged drift","state":"OPEN","body":"","labels":[{"name":"status:in-progress"}]}
			]`,
			"gh pr list --repo StatPan/gira --state all --limit 1000 --json number,title,body,state,mergedAt": `[
				{"number":10,"title":"Merge work","body":"Closes #5","state":"MERGED","mergedAt":"2026-05-18T00:00:00Z"}
			]`,
			"gh label list --repo StatPan/gira --json name --limit 1000": `[{"name":"status:done"}]`,
		},
		errors: map[string]error{},
	}

	report, err := BuildWorkflowAuditReport(repo, runner, time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildWorkflowAuditReport error: %v", err)
	}
	if report.Ready {
		t.Fatal("ready = true, want false")
	}
	for _, id := range []string{
		"closed_issue_active_status",
		"closed_issue_missing_done",
		"open_issue_done_status",
		"multiple_status_labels",
		"in_review_without_linked_pr",
		"missing_provenance",
		"merged_pr_issue_not_converged",
	} {
		if !workflowFindingsContain(report.Findings, id) {
			t.Fatalf("missing finding %s: %+v", id, report.Findings)
		}
	}
	if report.Counts.IssuesScanned != 5 || report.Counts.PRsScanned != 1 || report.Counts.Findings != len(report.Findings) {
		t.Fatalf("unexpected counts: %+v findings=%d", report.Counts, len(report.Findings))
	}
	if !strings.Contains(report.NextStep, "gira adopt issues --repo StatPan/gira --state all --issues 1,2,3,5 --normalize-status --dry-run") {
		t.Fatalf("next step = %q", report.NextStep)
	}
}

func TestFormatWorkflowAuditReportIncludesActionableNextStep(t *testing.T) {
	report := WorkflowAuditReport{
		Repo:    "StatPan/gira",
		Command: "audit workflow",
		Ready:   false,
		Counts:  WorkflowAuditCounts{IssuesScanned: 2, PRsScanned: 1, Findings: 1},
		Findings: []WorkflowAuditFinding{{
			ID:          "open_issue_done_status",
			Severity:    "fail",
			IssueNumber: 2,
			Detail:      "open issue has terminal status:done",
			Remediation: "run normalize",
		}},
		NextStep: "gira adopt issues --repo StatPan/gira --issues 2 --normalize-status --dry-run",
	}

	out := FormatWorkflowAuditReport(report)
	for _, want := range []string{"audit workflow: NOT READY", "scanned: issues=2 prs=1 findings=1", "open_issue_done_status issue=#2", "remediation: run normalize", "next step: gira adopt issues"} {
		if !strings.Contains(out, want) {
			t.Fatalf("workflow audit output missing %q:\n%s", want, out)
		}
	}
}

func workflowFindingsContain(findings []WorkflowAuditFinding, id string) bool {
	for _, finding := range findings {
		if finding.ID == id {
			return true
		}
	}
	return false
}
