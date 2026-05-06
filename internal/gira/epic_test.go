package gira

import (
	"fmt"
	"strings"
	"testing"
)

type epicRunner struct {
	outputs map[string][]byte
	calls   []string
}

func (r *epicRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, key)
	if out, ok := r.outputs[key]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected call: %s", key)
}

func TestBuildEpicStatusSelectsSoleOpenEpic(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &epicRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=open -f per_page=100": []byte(`[[` +
			`{"number":10,"title":"[Epic] Public docs","state":"open","body":"Tracks #11","labels":[{"name":"type:epic"},{"name":"status:ready"}],"milestone":{"title":"v1.4"},"html_url":"u10"},` +
			`{"number":11,"title":"Write guide","state":"open","labels":[{"name":"type:task"}],"milestone":{"title":"v1.4"},"html_url":"u11"}` +
			`]]`),
		"gh api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100": []byte(`[[` +
			`{"number":10,"title":"[Epic] Public docs","state":"open","body":"Tracks #11","labels":[{"name":"type:epic"},{"name":"status:ready"}],"milestone":{"title":"v1.4"},"html_url":"u10"},` +
			`{"number":11,"title":"Write guide","state":"open","labels":[{"name":"type:task"}],"milestone":{"title":"v1.4"},"html_url":"u11"},` +
			`{"number":12,"title":"Closed task","state":"closed","labels":[{"name":"type:task"}],"milestone":{"title":"v1.4"},"html_url":"u12"}` +
			`]]`),
	}}

	report, err := BuildEpicStatusReport(EpicInput{Repo: repo}, runner)
	if err != nil {
		t.Fatalf("BuildEpicStatusReport error: %v", err)
	}
	if report.Epic.Number != 10 || report.ChildCount.Open != 1 || report.ChildCount.Closed != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if !strings.Contains(report.NextStep, "gira epic finish --repo StatPan/gira --ticket 10 --dry-run") {
		t.Fatalf("unexpected next step: %s", report.NextStep)
	}
}

func TestBuildEpicStatusSelectsEpicFromBranch(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &epicRunner{outputs: map[string][]byte{
		"git branch --show-current":           []byte("issue-70-epic-roadmap\n"),
		"gh api repos/StatPan/gira/issues/70": []byte(`{"number":70,"title":"Roadmap Epic","state":"open","body":"","labels":[{"name":"type:epic"}],"milestone":null,"html_url":"u70"}`),
		"gh api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100": []byte(`[[{"number":70,"title":"Roadmap Epic","state":"open","labels":[{"name":"type:epic"}],"milestone":null,"html_url":"u70"}]]`),
	}}

	report, err := BuildEpicStatusReport(EpicInput{Repo: repo}, runner)
	if err != nil {
		t.Fatalf("BuildEpicStatusReport error: %v", err)
	}
	if report.Epic.Number != 70 {
		t.Fatalf("expected branch epic #70, got %+v", report.Epic)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "state=open") {
			t.Fatalf("branch epic should not require open epic list, calls=%v", runner.calls)
		}
	}
}

func TestBuildEpicStatusAmbiguousEpicsReturnsCandidates(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &epicRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=open -f per_page=100": []byte(`[[` +
			`{"number":1,"title":"Alpha Epic","state":"open","labels":[{"name":"type:epic"}],"milestone":{"title":"M1"}},` +
			`{"number":2,"title":"Beta Epic","state":"open","labels":[{"name":"type:epic"}],"milestone":{"title":"M2"}}` +
			`]]`),
	}}

	report, err := BuildEpicStatusReport(EpicInput{Repo: repo}, runner)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous error, got report=%+v err=%v", report, err)
	}
	if len(report.Candidates) != 2 || report.Candidates[0].Slug != "alpha-epic" {
		t.Fatalf("expected candidates with slugs, got %+v", report.Candidates)
	}
}

func TestFinishEpicBlocksOpenChildren(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &epicRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/10": []byte(`{"number":10,"title":"Epic","state":"open","body":"Tracks #11","labels":[{"name":"type:epic"},{"name":"status:ready"}],"milestone":{"title":"MVP"},"html_url":"u10"}`),
		"gh api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100": []byte(`[[` +
			`{"number":10,"title":"Epic","state":"open","labels":[{"name":"type:epic"}],"milestone":{"title":"MVP"},"html_url":"u10"},` +
			`{"number":11,"title":"Child","state":"open","labels":[{"name":"type:task"}],"milestone":{"title":"MVP"},"html_url":"u11"}` +
			`]]`),
	}}

	report, err := FinishEpic(EpicInput{Repo: repo, Ticket: 10, DryRun: true}, runner)
	if err != nil {
		t.Fatalf("FinishEpic dry-run should report blockers without error: %v", err)
	}
	if len(report.Blockers) != 1 || report.Actions[0].Status != "blocked" {
		t.Fatalf("expected blocked report, got %+v", report)
	}
}

func TestFinishEpicApplyClosesAndNormalizesStatus(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &epicRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/10": []byte(`{"number":10,"title":"Epic","state":"open","body":"Tracks #11","labels":[{"name":"type:epic"},{"name":"status:in-progress"}],"milestone":{"title":"MVP"},"html_url":"u10"}`),
		"gh api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100": []byte(`[[` +
			`{"number":10,"title":"Epic","state":"open","labels":[{"name":"type:epic"},{"name":"status:in-progress"}],"milestone":{"title":"MVP"},"html_url":"u10"},` +
			`{"number":11,"title":"Child","state":"closed","labels":[{"name":"type:task"}],"milestone":{"title":"MVP"},"html_url":"u11"}` +
			`]]`),
		"gh label list --repo StatPan/gira --json name --limit 1000":                                     []byte(`[{"name":"status:done"},{"name":"status:in-progress"}]`),
		"gh issue edit 10 --repo StatPan/gira --add-label status:done --remove-label status:in-progress": nil,
		"gh issue close 10 --repo StatPan/gira --comment Closed by gira epic finish":                     []byte("closed\n"),
	}}

	report, err := FinishEpic(EpicInput{Repo: repo, Ticket: 10, Apply: true}, runner)
	if err != nil {
		t.Fatalf("FinishEpic error: %v", err)
	}
	if len(report.Actions) != 2 || report.Actions[0].Action != "epic:normalize-status" || report.Actions[1].Action != "epic:close" {
		t.Fatalf("unexpected actions: %+v", report.Actions)
	}
	if !containsCall(runner.calls, "gh issue close 10 --repo StatPan/gira --comment Closed by gira epic finish") {
		t.Fatalf("missing close call: %v", runner.calls)
	}
}
