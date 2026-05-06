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
	if !strings.Contains(text, "missing_milestone") || !strings.Contains(text, "gira adopt issues --repo StatPan/gira --issue N") {
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
