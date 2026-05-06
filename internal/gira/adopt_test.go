package gira

import (
	"fmt"
	"strings"
	"testing"
)

type adoptRunner struct {
	outputs map[string][]byte
	calls   []string
}

func (r *adoptRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, key)
	if out, ok := r.outputs[key]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected call: %s", key)
}

func TestBuildAdoptIssuesReportListsUnmappedIssues(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &adoptRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=open -f per_page=100": []byte(`[[` +
			`{"number":1,"title":"No mapping","state":"open","labels":[],"milestone":null,"html_url":"u1"},` +
			`{"number":2,"title":"Mapped","state":"open","labels":[{"name":"type:task"},{"name":"status:ready"}],"milestone":{"title":"MVP"},"html_url":"u2"},` +
			`{"number":3,"title":"PR","state":"open","pull_request":{},"labels":[],"milestone":null}` +
			`]]`),
	}}

	report, err := BuildAdoptIssuesReport(AdoptIssueInput{Repo: repo, DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildAdoptIssuesReport error: %v", err)
	}
	if report.Counts.Scanned != 2 || report.Counts.Unmapped != 1 || report.Unmapped[0].Number != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	text := FormatAdoptIssuesReport(report)
	if !strings.Contains(text, "missing_milestone") || !strings.Contains(text, "gira adopt issues --repo StatPan/gira --issues 1-3") {
		t.Fatalf("formatted report missing adoption hints:\n%s", text)
	}
}

func TestBuildAdoptIssuesReportAppliesSelectedMapping(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &adoptRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=open -f per_page=100":           []byte(`[[{"number":1,"title":"No mapping","state":"open","labels":[],"milestone":null,"html_url":"u1"}]]`),
		"gh issue edit 1 --repo StatPan/gira --milestone MVP --add-label status:ready --add-label type:task": nil,
	}}

	report, err := BuildAdoptIssuesReport(AdoptIssueInput{Repo: repo, Issues: []int{1}, Milestone: "MVP", Labels: []string{"type:task,status:ready"}, Apply: true}, runner)
	if err != nil {
		t.Fatalf("BuildAdoptIssuesReport error: %v", err)
	}
	if report.Counts.AppliedUpdate != 1 || report.Actions[0].Status != "applied" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if !containsCall(runner.calls, "gh issue edit 1 --repo StatPan/gira --milestone MVP --add-label status:ready --add-label type:task") {
		t.Fatalf("missing issue edit call: %v", runner.calls)
	}
}

func TestBuildAdoptIssuesReportNormalizesClosedIssueStatus(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &adoptRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100": []byte(`[[` +
			`{"number":10,"title":"Done but active","state":"closed","labels":[{"name":"status:in-progress"},{"name":"type:task"}],"milestone":{"title":"MVP"},"html_url":"u10"},` +
			`{"number":11,"title":"Open active","state":"open","labels":[{"name":"status:in-progress"}],"milestone":{"title":"MVP"},"html_url":"u11"}` +
			`]]`),
		"gh label list --repo StatPan/gira --json name --limit 1000":                                     []byte(`[{"name":"status:done"},{"name":"status:in-progress"}]`),
		"gh issue edit 10 --repo StatPan/gira --add-label status:done --remove-label status:in-progress": nil,
	}}

	report, err := BuildAdoptIssuesReport(AdoptIssueInput{Repo: repo, State: "all", NormalizeStatus: true, Apply: true}, runner)
	if err != nil {
		t.Fatalf("BuildAdoptIssuesReport error: %v", err)
	}
	if report.Counts.AppliedUpdate != 1 || report.Actions[0].Issue != 10 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if !containsCall(runner.calls, "gh issue edit 10 --repo StatPan/gira --add-label status:done --remove-label status:in-progress") {
		t.Fatalf("missing status normalization call: %v", runner.calls)
	}
}

func TestBuildAdoptIssuesReportNormalizesSelectedClosedIssueWithoutDoneLabel(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &adoptRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100": []byte(`[[{"number":12,"title":"Closed blocked","state":"closed","labels":[{"name":"status:blocked"}],"milestone":{"title":"MVP"},"html_url":"u12"}]]`),
		"gh label list --repo StatPan/gira --json name --limit 1000":                              []byte(`[{"name":"status:blocked"}]`),
		"gh issue edit 12 --repo StatPan/gira --remove-label status:blocked":                      nil,
	}}

	report, err := BuildAdoptIssuesReport(AdoptIssueInput{Repo: repo, Issues: []int{12}, State: "all", NormalizeStatus: true, Apply: true}, runner)
	if err != nil {
		t.Fatalf("BuildAdoptIssuesReport error: %v", err)
	}
	if report.Counts.AppliedUpdate != 1 || len(report.Actions[0].Labels) != 0 || len(report.Actions[0].RemoveLabels) != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if !containsCall(runner.calls, "gh issue edit 12 --repo StatPan/gira --remove-label status:blocked") {
		t.Fatalf("missing status cleanup call: %v", runner.calls)
	}
}
