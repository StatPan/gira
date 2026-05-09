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
