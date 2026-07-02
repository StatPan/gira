package gira

import (
	"fmt"
	"strings"
	"testing"
)

type ticketSupersedeRunner struct {
	calls []string
}

func (r *ticketSupersedeRunner) Run(name string, args ...string) ([]byte, error) {
	call := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, call)
	switch {
	case call == "gh api repos/StatPan/gira/issues/64":
		return []byte(`{"number":64,"title":"Old gate","state":"open","html_url":"https://github.com/StatPan/gira/issues/64","labels":[{"name":"type:task"},{"name":"status:ready"},{"name":"priority:p1"},{"name":"area:docs"},{"name":"resolution:duplicate"}],"milestone":{"title":"MVP"}}`), nil
	case strings.HasPrefix(call, "gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 64 "):
		return []byte(`[{"number":65,"title":"draft","body":"Closes #64","state":"OPEN","url":"https://github.com/StatPan/gira/pull/65","reviewDecision":"","isDraft":true,"mergeStateStatus":"UNKNOWN","statusCheckRollup":[]}]`), nil
	case strings.HasPrefix(call, "gh issue create --repo StatPan/gira --title New gate --body "):
		if !strings.Contains(call, "Supersedes #64") || !strings.Contains(call, "--label status:ready") || strings.Contains(call, "--label status:done") || strings.Contains(call, "--label resolution:") {
			return nil, fmt.Errorf("unexpected create call: %s", call)
		}
		return []byte("https://github.com/StatPan/gira/issues/94\n"), nil
	case strings.HasPrefix(call, "gh issue comment 64 --repo StatPan/gira --body ## Superseded"):
		return nil, nil
	case strings.HasPrefix(call, "gh issue comment 94 --repo StatPan/gira --body ## Replacement"):
		return nil, nil
	case call == "gh api repos/StatPan/gira/issues/64/labels/status:ready -X DELETE":
		return nil, nil
	case call == "gh api repos/StatPan/gira/issues/64/labels -X POST -f labels[]=resolution:superseded":
		return nil, nil
	case call == "gh issue close 64 --repo StatPan/gira":
		return nil, nil
	case call == "gh pr close 65 --repo StatPan/gira --comment Superseded by #94.":
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected call: %s", call)
	}
}

func TestBuildTicketSupersedeReportDryRunPlansReplacement(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &ticketSupersedeRunner{}

	report, err := BuildTicketSupersedeReport(TicketSupersedeInput{
		Repo:             repo,
		Ticket:           64,
		ReplacementTitle: "New gate",
		Body:             "## Goal\nDefine release gate.",
		DryRun:           true,
	}, runner)
	if err != nil {
		t.Fatalf("BuildTicketSupersedeReport error: %v", err)
	}
	if report.Replacement.Number != 0 || !report.DryRun || report.Replacement.Milestone != "MVP" {
		t.Fatalf("unexpected dry-run report: %+v", report)
	}
	if !containsString(report.Replacement.Labels, "status:ready") || containsString(report.Replacement.Labels, "status:done") {
		t.Fatalf("replacement labels not normalized: %+v", report.Replacement.Labels)
	}
	if containsString(report.Replacement.Labels, "resolution:duplicate") || containsString(report.Replacement.Labels, "resolution:superseded") {
		t.Fatalf("replacement carried resolution labels: %+v", report.Replacement.Labels)
	}
	if !strings.Contains(report.Replacement.Body, "Supersedes #64") {
		t.Fatalf("replacement body missing original link:\n%s", report.Replacement.Body)
	}
	if report.SchemaVersion != TicketSupersedeReportSchemaVersion || report.Approval == nil {
		t.Fatalf("expected supersede schema and approval evidence: %+v", report)
	}
	if report.Approval.SchemaVersion != ApprovalPlanSchemaVersion || report.Approval.CanonicalCommand != "gira ticket supersede" || report.Approval.OutputSchema != TicketSupersedeReportSchemaVersion {
		t.Fatalf("unexpected supersede approval evidence: %+v", report.Approval)
	}
	if report.Approval.ApplyCommand != "gira ticket supersede 64 --repo StatPan/gira --replacement-title 'New gate' --body '## Goal\nDefine release gate.' --apply" || report.Approval.PostApplyVerification != "gira ticket status 64 --repo StatPan/gira --json" {
		t.Fatalf("unexpected supersede approval commands: %+v", report.Approval)
	}
	if report.Approval.Blockers == nil || report.Approval.Warnings == nil || !approvalHasAction(report.Approval.PlannedActions, "replacement:create") || !approvalHasAction(report.Approval.PlannedActions, "original:close") {
		t.Fatalf("unexpected supersede approval plan: %+v", report.Approval)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "gh pr list") || strings.Contains(call, "/pulls") || strings.Contains(call, "/timeline") {
			t.Fatalf("dry-run without --close-draft-pr should not inspect linked PRs: %v", runner.calls)
		}
		if strings.Contains(call, "issue create") || strings.Contains(call, "issue close") || strings.Contains(call, "issue comment") {
			t.Fatalf("dry-run mutated GitHub: %v", runner.calls)
		}
	}
}

func TestBuildTicketSupersedeReportDryRunCloseDraftPRInspectsLinkedPR(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &ticketSupersedeRunner{}

	report, err := BuildTicketSupersedeReport(TicketSupersedeInput{
		Repo:             repo,
		Ticket:           64,
		ReplacementTitle: "New gate",
		Body:             "## Goal\nDefine release gate.",
		CloseDraftPR:     true,
		DryRun:           true,
	}, runner)
	if err != nil {
		t.Fatalf("BuildTicketSupersedeReport error: %v", err)
	}
	if report.DraftPR.Number != 65 || report.DraftPR.Action != "close" {
		t.Fatalf("expected draft PR close plan: %+v", report.DraftPR)
	}
	if !containsSupersedeCallPrefix(runner.calls, "gh pr list --repo StatPan/gira") {
		t.Fatalf("expected linked PR inspection when --close-draft-pr is set: %v", runner.calls)
	}
	if !supersedeHasAction(report.Actions, "draft_pr:close") {
		t.Fatalf("missing draft PR close action: %+v", report.Actions)
	}
}

func TestBuildTicketSupersedeReportApplyCreatesLinksAndCloses(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &ticketSupersedeRunner{}

	report, err := BuildTicketSupersedeReport(TicketSupersedeInput{
		Repo:             repo,
		Ticket:           64,
		ReplacementTitle: "New gate",
		Body:             "## Goal\nDefine release gate.",
		CloseDraftPR:     true,
		Apply:            true,
	}, runner)
	if err != nil {
		t.Fatalf("BuildTicketSupersedeReport error: %v", err)
	}
	if report.Replacement.Number != 94 || report.Original.State != "closed" || report.DraftPR.Action != "close" {
		t.Fatalf("unexpected apply report: %+v", report)
	}
	if report.Approval != nil {
		t.Fatalf("apply output should not include dry-run approval evidence: %+v", report.Approval)
	}
	for _, want := range []string{
		"gh api repos/StatPan/gira/issues/64/labels/status:ready -X DELETE",
		"gh api repos/StatPan/gira/issues/64/labels -X POST -f labels[]=resolution:superseded",
		"gh issue close 64 --repo StatPan/gira",
		"gh pr close 65 --repo StatPan/gira --comment Superseded by #94.",
	} {
		if !containsCall(runner.calls, want) {
			t.Fatalf("missing call %q in %v", want, runner.calls)
		}
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "labels[]=status:done") {
			t.Fatalf("supersede should not mark original done: %v", runner.calls)
		}
	}
	if !strings.Contains(FormatTicketSupersede(report), "replacement=#94") {
		t.Fatalf("format missing replacement:\n%s", FormatTicketSupersede(report))
	}
}

func containsSupersedeCallPrefix(calls []string, prefix string) bool {
	for _, call := range calls {
		if strings.HasPrefix(call, prefix) {
			return true
		}
	}
	return false
}

func supersedeHasAction(actions []TicketSupersedeAction, action string) bool {
	for _, item := range actions {
		if item.Action == action {
			return true
		}
	}
	return false
}
