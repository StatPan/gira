package gira

import (
	"fmt"
	"strings"
	"testing"
)

type ticketParentRunner struct {
	outputs map[string][]byte
	errs    map[string]error
	calls   []string
}

func (r *ticketParentRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, key)
	if err, ok := r.errs[key]; ok {
		return nil, err
	}
	if out, ok := r.outputs[key]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected call: %s", key)
}

func TestTicketParentSetDryRunPlansNativeLink(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &ticketParentRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/42 -H Accept: application/vnd.github+json":                                            []byte(`{"id":4200,"number":42,"title":"Child","state":"open"}`),
		"gh api repos/StatPan/gira/issues/42/parent -H Accept: application/vnd.github+json -H X-GitHub-Api-Version: 2026-03-10": []byte(`{"id":1000,"number":10,"title":"Old parent","state":"open"}`),
		"gh api repos/StatPan/gira/issues/11 -H Accept: application/vnd.github+json":                                            []byte(`{"id":1100,"number":11,"title":"New parent","state":"open"}`),
	}}

	report, err := BuildTicketParentReport(TicketParentInput{Repo: repo, Ticket: 42, Set: 11, DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildTicketParentReport error: %v", err)
	}
	if report.CurrentParent == nil || report.CurrentParent.Number != 10 || report.TargetParent == nil || report.TargetParent.Number != 11 {
		t.Fatalf("unexpected parent report: %+v", report)
	}
	if report.Approval == nil || !approvalHasAction(report.Approval.PlannedActions, "parent:set") {
		t.Fatalf("approval missing set action: %+v", report.Approval)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "/sub_issues") {
			t.Fatalf("dry-run should not mutate sub-issues, calls=%+v", runner.calls)
		}
	}
}

func TestTicketParentSetApplyReplacesExistingParent(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &ticketParentRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/42 -H Accept: application/vnd.github+json":                                                                                                    []byte(`{"id":4200,"number":42,"title":"Child","state":"open"}`),
		"gh api repos/StatPan/gira/issues/42/parent -H Accept: application/vnd.github+json -H X-GitHub-Api-Version: 2026-03-10":                                                         []byte(`{"id":1000,"number":10,"title":"Old parent","state":"open"}`),
		"gh api repos/StatPan/gira/issues/11 -H Accept: application/vnd.github+json":                                                                                                    []byte(`{"id":1100,"number":11,"title":"New parent","state":"open"}`),
		"gh api repos/StatPan/gira/issues/11/sub_issues -X POST -H Accept: application/vnd.github+json -H X-GitHub-Api-Version: 2026-03-10 -F sub_issue_id=4200 -F replace_parent=true": []byte(`{"number":42}`),
	}}

	report, err := BuildTicketParentReport(TicketParentInput{Repo: repo, Ticket: 42, Set: 11, Apply: true}, runner)
	if err != nil {
		t.Fatalf("BuildTicketParentReport error: %v", err)
	}
	if len(report.Actions) != 1 || report.Actions[0].Status != "applied" {
		t.Fatalf("unexpected actions: %+v", report.Actions)
	}
}

func TestTicketParentClearApplyRemovesNativeLink(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &ticketParentRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/42 -H Accept: application/vnd.github+json":                                                                              []byte(`{"id":4200,"number":42,"title":"Child","state":"open"}`),
		"gh api repos/StatPan/gira/issues/42/parent -H Accept: application/vnd.github+json -H X-GitHub-Api-Version: 2026-03-10":                                   []byte(`{"id":1000,"number":10,"title":"Old parent","state":"open"}`),
		"gh api repos/StatPan/gira/issues/10/sub_issue -X DELETE -H Accept: application/vnd.github+json -H X-GitHub-Api-Version: 2026-03-10 -F sub_issue_id=4200": []byte(`{"number":42}`),
	}}

	report, err := BuildTicketParentReport(TicketParentInput{Repo: repo, Ticket: 42, Clear: true, Apply: true}, runner)
	if err != nil {
		t.Fatalf("BuildTicketParentReport error: %v", err)
	}
	if len(report.Actions) != 1 || report.Actions[0].Action != "parent:clear" || report.Actions[0].Status != "applied" {
		t.Fatalf("unexpected actions: %+v", report.Actions)
	}
}

func TestTicketParentStatusHandlesMissingParent(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &ticketParentRunner{
		outputs: map[string][]byte{
			"gh api repos/StatPan/gira/issues/42 -H Accept: application/vnd.github+json": []byte(`{"id":4200,"number":42,"title":"Child","state":"open"}`),
		},
		errs: map[string]error{
			"gh api repos/StatPan/gira/issues/42/parent -H Accept: application/vnd.github+json -H X-GitHub-Api-Version: 2026-03-10": fmt.Errorf("No parent issue found (HTTP 404)"),
		},
	}

	report, err := BuildTicketParentReport(TicketParentInput{Repo: repo, Ticket: 42}, runner)
	if err != nil {
		t.Fatalf("BuildTicketParentReport error: %v", err)
	}
	if report.CurrentParent != nil || !strings.Contains(report.NextStep, "--set PARENT") {
		t.Fatalf("unexpected parent status: %+v", report)
	}
}
