package gira

import (
	"strings"
	"testing"
)

func TestWorkStatusNextStepGuidesUnreadyIssueToAdoptReady(t *testing.T) {
	result := WorkStatusResult{
		Repo:       "StatPan/statpan-infra",
		Issue:      33,
		State:      "open",
		Status:     "null",
		NextAction: "start_work",
	}

	got := workStatusNextStep(result)
	want := "gira adopt issues --repo StatPan/statpan-infra --issue 33 --label status:ready --apply"
	if got != want {
		t.Fatalf("next step = %q, want %q", got, want)
	}
}

func TestWorkStatusNextStepStartsReadyIssue(t *testing.T) {
	result := WorkStatusResult{
		Repo:       "StatPan/statpan-infra",
		Issue:      33,
		State:      "open",
		Status:     "Ready",
		NextAction: "start_work",
	}

	got := workStatusNextStep(result)
	want := "gira work start --repo StatPan/statpan-infra --issue 33 --apply"
	if got != want {
		t.Fatalf("next step = %q, want %q", got, want)
	}
}

func TestWorkStatusNextStepDoesNotAdoptExplicitBlockedStatus(t *testing.T) {
	result := WorkStatusResult{
		Repo:       "StatPan/statpan-infra",
		Issue:      33,
		State:      "open",
		Status:     "Blocked",
		NextAction: "resolve_blockers",
	}

	got := workStatusNextStep(result)
	want := "resolve blockers, then set status:ready before starting work"
	if got != want {
		t.Fatalf("next step = %q, want %q", got, want)
	}
}

func TestNextWorkActionResolvesBlockedIssueBeforeStart(t *testing.T) {
	action := nextWorkAction("open", "Blocked", DevPRStatusResult{})
	if action != "resolve_blockers" {
		t.Fatalf("next action = %q, want resolve_blockers", action)
	}
}

func TestWorkStartDryRunGuidesToApplyBeforePR(t *testing.T) {
	got := workStartNextStep("StatPan/statpan-infra", 33, "open", "Ready", true)
	want := "gira work start --repo StatPan/statpan-infra --issue 33 --apply"
	if got != want {
		t.Fatalf("next step = %q, want %q", got, want)
	}
}

func TestStartWorkMissingReadyIncludesActionableNextStep(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "statpan-infra"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/statpan-infra/issues/33": []byte(`{"number":33,"title":"RAG Docling","state":"open","labels":[{"name":"type:task"}]}`),
	}}

	result, err := StartWork(repo, 33, false, runner)
	if err == nil {
		t.Fatal("expected missing ready error")
	}
	want := "gira adopt issues --repo StatPan/statpan-infra --issue 33 --label status:ready --apply"
	if result.NextStep != want {
		t.Fatalf("next step = %q, want %q", result.NextStep, want)
	}
}

func TestFormatWorkStartUsesActionableReadyNextStep(t *testing.T) {
	result := WorkStartResult{
		Repo:       "StatPan/statpan-infra",
		Issue:      33,
		Branch:     "issue-33-rag-docling",
		Status:     "null",
		NextStatus: "In progress",
		NextStep:   "gira adopt issues --repo StatPan/statpan-infra --issue 33 --label status:ready --apply",
	}

	out := FormatWorkStart(result)
	if !strings.Contains(out, result.NextStep) {
		t.Fatalf("output missing actionable next step:\n%s", out)
	}
	if strings.Contains(out, "gira work start --repo StatPan/statpan-infra --issue 33 --apply") {
		t.Fatalf("output should not point back to the same blocked start command:\n%s", out)
	}
}
