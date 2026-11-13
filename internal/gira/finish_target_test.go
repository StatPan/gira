package gira

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const readyForReleaseWorkflow = `  statuses:
    - { name: ready, category: to_do }
    - { name: in-progress, category: in_progress }
    - { name: in-review, category: in_progress }
    - { name: ready-for-release, category: in_progress }
    - { name: done, category: done }
  transitions:
    - { name: start, from: ready, to: in-progress }
    - { name: review, from: in-progress, to: in-review }
    - { name: finish, from: in-review, to: ready-for-release }
    - { name: release, from: ready-for-release, to: done }
    - { name: supersede, from: any, to: done }
  finish_target: ready-for-release
`

func TestConvergeOpenFinishTargetReopensAndSetsTargetLabel(t *testing.T) {
	chdirDeclaredWorkflow(t, readyForReleaseWorkflow)
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126":                                                     []byte(`{"number":126,"title":"Work","state":"closed","labels":[{"name":"status:in-review"}]}`),
		"gh issue reopen 126 --repo StatPan/gira":                                                  nil,
		"gh api repos/StatPan/gira/issues/126/labels/status:in-review -X DELETE":                   nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:ready-for-release": nil,
	}}
	actions, err := convergeFinishedIssue(RepoRef{Owner: "StatPan", Name: "gira"}, 126, true, false, runner)
	if err != nil {
		t.Fatalf("converge error: %v (calls %v)", err, runner.calls)
	}
	if !hasWorkFinishAction(actions, "ticket:reopen", "applied") || !hasWorkFinishAction(actions, "ticket:finish-target", "applied") {
		t.Fatalf("unexpected actions: %+v", actions)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "issue close") || strings.Contains(call, "status:done") {
			t.Fatalf("non-Done finish target closed or marked done: %v", runner.calls)
		}
	}
	if !containsCall(runner.calls, "gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:ready-for-release") {
		t.Fatalf("target label not written: %v", runner.calls)
	}
}

func TestConvergeOpenFinishTargetIsIdempotent(t *testing.T) {
	chdirDeclaredWorkflow(t, readyForReleaseWorkflow)
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work","state":"open","labels":[{"name":"status:ready-for-release"}]}`),
	}}
	actions, err := convergeFinishedIssue(RepoRef{Owner: "StatPan", Name: "gira"}, 126, true, false, runner)
	if err != nil || len(actions) != 0 {
		t.Fatalf("expected no actions, got %+v err=%v", actions, err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected only the issue read: %v", runner.calls)
	}
}

func TestFinishResolvesToDeclaredFinishTarget(t *testing.T) {
	w := mustDeclaredWorkflow(t, readyForReleaseWorkflow)
	move := resolveDeclaredStatusMove(w, "finish", []string{"status:in-review"})
	if move.to != "ready-for-release" || move.label != "status:ready-for-release" || move.blocker != "" {
		t.Fatalf("unexpected finish move: %+v", move)
	}
}

func TestTransitionTicketReleaseClosesCompleted(t *testing.T) {
	chdirDeclaredWorkflow(t, readyForReleaseWorkflow)
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126":                                           []byte(`{"number":126,"title":"Work","state":"open","labels":[{"name":"status:ready-for-release"}]}`),
		"gh api repos/StatPan/gira/issues/126/labels/status:ready-for-release -X DELETE": nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:done":    nil,
		"gh issue close 126 --repo StatPan/gira --reason completed":                      nil,
	}}
	report, err := TransitionTicket(TicketTransitionInput{Repo: RepoRef{Owner: "StatPan", Name: "gira"}, Ticket: 126, To: "done"}, runner)
	if err != nil {
		t.Fatalf("transition error: %v", err)
	}
	if report.Transition != "release" || report.Resolution != "completed" || report.From != "ready-for-release" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if !containsCall(runner.calls, "gh issue close 126 --repo StatPan/gira --reason completed") {
		t.Fatalf("expected completed close: %v", runner.calls)
	}
}

func TestTransitionTicketDryRunPlansWithoutMutation(t *testing.T) {
	chdirDeclaredWorkflow(t, readyForReleaseWorkflow)
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work","state":"open","labels":[{"name":"status:ready-for-release"}]}`),
	}}
	report, err := TransitionTicket(TicketTransitionInput{Repo: RepoRef{Owner: "StatPan", Name: "gira"}, Ticket: 126, To: "done", DryRun: true}, runner)
	if err != nil || len(report.Actions) != 2 || report.Actions[0].Status != "planned" || report.NextStep == "" {
		t.Fatalf("unexpected dry-run: %+v err=%v", report, err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("dry-run mutated: %v", runner.calls)
	}
}

func TestTransitionTicketMissingPathAndUnknownStatusBlock(t *testing.T) {
	chdirDeclaredWorkflow(t, readyForReleaseWorkflow)
	issue := []byte(`{"number":126,"title":"Work","state":"open","labels":[{"name":"status:ready"}]}`)
	for to, blocker := range map[string]string{"ready-for-release": "missing_transition", "shipped": "unknown_status"} {
		runner := &workRunner{outputs: map[string][]byte{"gh api repos/StatPan/gira/issues/126": issue}}
		report, err := TransitionTicket(TicketTransitionInput{Repo: RepoRef{Owner: "StatPan", Name: "gira"}, Ticket: 126, To: to}, runner)
		if err == nil || !containsString(report.Blockers, blocker) {
			t.Fatalf("to=%s: expected %s, got %+v err=%v", to, blocker, report.Blockers, err)
		}
		assertNoStatusMutation(t, runner.calls)
	}
}

func TestTransitionTicketLeavingDoneReopens(t *testing.T) {
	chdirDeclaredWorkflow(t, strings.Replace(readyForReleaseWorkflow, "  finish_target:", "    - { name: reopen, from: done, to: ready }\n  finish_target:", 1))
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126":                                         []byte(`{"number":126,"title":"Work","state":"closed","labels":[{"name":"status:done"}]}`),
		"gh api repos/StatPan/gira/issues/126/labels/status:done -X DELETE":            nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:ready": nil,
		"gh issue reopen 126 --repo StatPan/gira":                                      nil,
	}}
	report, err := TransitionTicket(TicketTransitionInput{Repo: RepoRef{Owner: "StatPan", Name: "gira"}, Ticket: 126, To: "ready"}, runner)
	if err != nil || report.Transition != "reopen" {
		t.Fatalf("unexpected reopen: %+v err=%v", report, err)
	}
	if !containsCall(runner.calls, "gh issue reopen 126 --repo StatPan/gira") {
		t.Fatalf("expected reopen: %v", runner.calls)
	}
}

func mustParseWorkflow(t *testing.T, text string) Workflow {
	t.Helper()
	var w Workflow
	if err := yaml.Unmarshal([]byte(text), &w); err != nil {
		t.Fatal(err)
	}
	return w
}

func TestWorkflowFinishUnreachableFromReview(t *testing.T) {
	cfg := mustParseWorkflow(t, `statuses:
  - { name: ready, category: to_do }
  - { name: in-review, category: in_progress }
  - { name: done, category: done }
transitions:
  - { name: review, from: ready, to: in-review }
  - { name: close, from: ready, to: done }
finish_target: done
`)
	_, findings := resolveWorkflow(&cfg)
	found := false
	for _, f := range findings {
		found = found || f.ID == "workflow_finish_unreachable_from_review"
	}
	if !found {
		t.Fatalf("expected review reachability finding: %+v", findings)
	}
}
