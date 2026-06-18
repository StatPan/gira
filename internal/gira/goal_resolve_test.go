package gira

import (
	"strings"
	"testing"
)

func TestResolveGoalNumberSelectsCurrentGoalBranch(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &epicRunner{outputs: map[string][]byte{
		"git branch --show-current":           []byte("issue-70-dispatch-goal\n"),
		"gh api repos/StatPan/gira/issues/70": []byte(`{"number":70,"title":"Dispatch Goal","state":"open","body":"## Goal\nDispatch","labels":[{"name":"type:epic"},{"name":"status:ready"}]}`),
	}}

	goal, candidates, err := ResolveGoalNumber(repo, 0, runner)
	if err != nil {
		t.Fatalf("ResolveGoalNumber error: %v", err)
	}
	if goal != 70 || len(candidates) != 1 || candidates[0].Source != "current_issue" {
		t.Fatalf("unexpected goal resolution: goal=%d candidates=%+v", goal, candidates)
	}
}

func TestResolveGoalNumberSelectsParentGoalFromChildBranch(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &epicRunner{outputs: map[string][]byte{
		"git branch --show-current":            []byte("issue-201-child-work\n"),
		"gh api repos/StatPan/gira/issues/201": []byte(`{"number":201,"title":"Child","state":"open","body":"## Goal\nChild\n\nParent: #70","labels":[{"name":"type:task"},{"name":"status:ready"}]}`),
		"gh api repos/StatPan/gira/issues/70":  []byte(`{"number":70,"title":"Dispatch Goal","state":"open","body":"## Goal\nDispatch","labels":[{"name":"type:epic"},{"name":"status:ready"}]}`),
	}}

	goal, candidates, err := ResolveGoalNumber(repo, 0, runner)
	if err != nil {
		t.Fatalf("ResolveGoalNumber error: %v", err)
	}
	if goal != 70 || len(candidates) != 1 || candidates[0].Source != "parent_goal" {
		t.Fatalf("unexpected parent goal resolution: goal=%d candidates=%+v", goal, candidates)
	}
}

func TestResolveGoalNumberSelectsSoleOpenGoal(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &epicRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=open -f per_page=100": []byte(`[[` +
			`{"number":70,"title":"Dispatch Goal","state":"open","body":"## Goal\nDispatch","labels":[{"name":"type:epic"},{"name":"status:ready"}],"html_url":"u70"},` +
			`{"number":201,"title":"Child","state":"open","labels":[{"name":"type:task"}]}` +
			`]]`),
	}}

	goal, candidates, err := ResolveGoalNumber(repo, 0, runner)
	if err != nil {
		t.Fatalf("ResolveGoalNumber error: %v", err)
	}
	if goal != 70 || len(candidates) != 1 || candidates[0].Source != "open_goal" {
		t.Fatalf("unexpected sole goal resolution: goal=%d candidates=%+v", goal, candidates)
	}
}

func TestResolveGoalNumberAmbiguousGoals(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &epicRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=open -f per_page=100": []byte(`[[` +
			`{"number":70,"title":"Dispatch Goal","state":"open","labels":[{"name":"type:epic"}]},` +
			`{"number":71,"title":"Second Goal","state":"open","labels":[{"name":"type:goal"}]}` +
			`]]`),
	}}

	goal, candidates, err := ResolveGoalNumber(repo, 0, runner)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous error, got goal=%d candidates=%+v err=%v", goal, candidates, err)
	}
	if len(candidates) != 2 || !strings.Contains(FormatGoalCandidates(candidates), "#70 Dispatch Goal") {
		t.Fatalf("unexpected candidates: %+v", candidates)
	}
}
