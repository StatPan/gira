package gira

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func chdirDeclaredWorkflow(t *testing.T, workflow string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".gira"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := "repo: StatPan/gira\nprofiles:\n  default:\n    labels: []\n    milestones: []\n    issue_templates: []\nworkflow:\n" + workflow
	if err := os.WriteFile(filepath.Join(root, ".gira", "config.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
}

const customLabelWorkflow = `  statuses:
    - { name: ready, category: to_do }
    - { name: doing, label: "status:doing", category: in_progress }
    - { name: done, category: done }
  transitions:
    - { name: start, from: ready, to: doing }
    - { name: finish, from: doing, to: done }
    - { name: supersede, from: any, to: done }
  finish_target: done
`

func declaredStartRunner(extra map[string][]byte) *workRunner {
	outputs := map[string][]byte{
		"gh api repos/StatPan/gira/issues/126":               []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:ready"}]}`),
		"git checkout -b issue-126-work-command origin/main": nil,
	}
	for key, value := range extra {
		outputs[key] = value
	}
	return &workRunner{outputs: outputs, errs: map[string]error{
		"git show-ref --verify --quiet refs/heads/issue-126-work-command": fmt.Errorf("exit status 1"),
		"git ls-remote --exit-code --heads origin issue-126-work-command": fmt.Errorf("exit status 2"),
	}}
}

func assertNoStatusMutation(t *testing.T, calls []string) {
	t.Helper()
	for _, call := range calls {
		if strings.Contains(call, "-X POST") || strings.Contains(call, "-X DELETE") || strings.Contains(call, "-X PATCH") || strings.Contains(call, "checkout -b") || strings.Contains(call, "issue close") || strings.Contains(call, "issue create") {
			t.Fatalf("blocked command mutated state: %v", calls)
		}
	}
}

func TestStartWorkDeclaredWorkflowWritesCustomLabel(t *testing.T) {
	chdirDeclaredWorkflow(t, customLabelWorkflow)
	runner := declaredStartRunner(map[string][]byte{
		"gh api repos/StatPan/gira/issues/126/labels/status:ready -X DELETE":           nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:doing": nil,
	})
	if _, err := StartWork(RepoRef{Owner: "StatPan", Name: "gira"}, 126, false, runner); err != nil {
		t.Fatalf("StartWork error: %v", err)
	}
	if !containsCall(runner.calls, "gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:doing") {
		t.Fatalf("declared label not written: %v", runner.calls)
	}
	if containsCall(runner.calls, "gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-progress") {
		t.Fatalf("default label written under declared workflow: %v", runner.calls)
	}
}

func TestStartWorkDeclaredWorkflowMissingTransitionBlocksBeforeMutation(t *testing.T) {
	chdirDeclaredWorkflow(t, `  statuses:
    - { name: ready, category: to_do }
    - { name: in-progress, category: in_progress }
    - { name: done, category: done }
  transitions:
    - { name: finish, from: ready, to: done }
  finish_target: done
`)
	for _, dryRun := range []bool{true, false} {
		runner := declaredStartRunner(nil)
		_, err := StartWork(RepoRef{Owner: "StatPan", Name: "gira"}, 126, dryRun, runner)
		if err == nil || !strings.Contains(err.Error(), "missing_transition") || !strings.Contains(err.Error(), "from ready to in-progress") {
			t.Fatalf("dryRun=%v: expected missing_transition, got %v", dryRun, err)
		}
		assertNoStatusMutation(t, runner.calls)
	}
}

func TestStartWorkDeclaredWorkflowInvalidBlocks(t *testing.T) {
	chdirDeclaredWorkflow(t, "  statuses: [{name: ready}]\n  finish_target: ready\n")
	runner := declaredStartRunner(nil)
	_, err := StartWork(RepoRef{Owner: "StatPan", Name: "gira"}, 126, false, runner)
	if err == nil || !strings.HasPrefix(err.Error(), "invalid_workflow") {
		t.Fatalf("expected invalid_workflow, got %v", err)
	}
	assertNoStatusMutation(t, runner.calls)
}

func TestStartWorkDeclaredWorkflowSameStatusIsNoOp(t *testing.T) {
	m := resolveDeclaredStatusMove(mustDeclaredWorkflow(t, customLabelWorkflow), "start", []string{"status:doing"})
	if !m.noOp || m.blocker != "" {
		t.Fatalf("expected no-op move: %+v", m)
	}
}

func mustDeclaredWorkflow(t *testing.T, workflow string) Workflow {
	t.Helper()
	chdirDeclaredWorkflow(t, workflow)
	gate, err := loadWorkflowGate(RepoRef{Owner: "StatPan", Name: "gira"})
	if err != nil || !gate.declared {
		t.Fatalf("load gate: declared=%v err=%v", gate.declared, err)
	}
	return gate.workflow
}

func TestOpenWorkPRDeclaredWorkflowMissingReviewBlocksBeforePush(t *testing.T) {
	chdirDeclaredWorkflow(t, customLabelWorkflow)
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126":                                                                  []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:doing"}]}`),
		"gh api repos/StatPan/gira/issues/126/timeline --paginate":                                              []byte(`[]`),
		"gh api repos/StatPan/gira/pulls -X GET -f state=all -f sort=updated -f direction=desc -f per_page=100": []byte(`[]`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid,baseRefOid --limit 20": []byte(`[]`),
	}}
	_, err := OpenWorkPR(RepoRef{Owner: "StatPan", Name: "gira"}, 126, true, false, runner)
	if err == nil || !strings.Contains(err.Error(), "missing_transition") {
		t.Fatalf("expected missing_transition, got %v (calls %v)", err, runner.calls)
	}
	for _, call := range runner.calls {
		if strings.HasPrefix(call, "git push") || strings.Contains(call, "pr create") {
			t.Fatalf("blocked PR pushed or created: %v", runner.calls)
		}
	}
	assertNoStatusMutation(t, runner.calls)
}

func TestFinishWorkDeclaredWorkflowMissingTransitionBlocksBeforePRLookup(t *testing.T) {
	chdirDeclaredWorkflow(t, `  statuses:
    - { name: ready, category: to_do }
    - { name: in-review, category: in_progress }
    - { name: done, category: done }
  transitions:
    - { name: review, from: ready, to: in-review }
    - { name: reopen, from: done, to: ready }
  finish_target: in-review
`)
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-review"}]}`),
	}}
	result, err := FinishWork(RepoRef{Owner: "StatPan", Name: "gira"}, 126, false, 0, runner)
	if err == nil || !strings.Contains(err.Error(), "missing_transition") || !containsString(result.Blockers, "missing_transition") {
		t.Fatalf("expected missing_transition blocker, got %v %+v", err, result.Blockers)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "pr list") || strings.Contains(call, "pr merge") {
			t.Fatalf("blocked finish inspected or merged PR: %v", runner.calls)
		}
	}
}

type declaredSupersedeRunner struct{ ticketSupersedeRunner }

func (r *declaredSupersedeRunner) Run(name string, args ...string) ([]byte, error) {
	call := name + " " + strings.Join(args, " ")
	if call == "gh issue close 64 --repo StatPan/gira --reason not planned" {
		r.calls = append(r.calls, call)
		return nil, nil
	}
	return r.ticketSupersedeRunner.Run(name, args...)
}

func TestBuildTicketSupersedeDeclaredWorkflowClosesNotPlanned(t *testing.T) {
	chdirDeclaredWorkflow(t, customLabelWorkflow)
	runner := &declaredSupersedeRunner{}
	_, err := BuildTicketSupersedeReport(TicketSupersedeInput{
		Repo:             RepoRef{Owner: "StatPan", Name: "gira"},
		Ticket:           64,
		ReplacementTitle: "New gate",
		Body:             "## Goal\nDefine release gate.",
		Apply:            true,
	}, runner)
	if err != nil {
		t.Fatalf("supersede error: %v", err)
	}
	if !containsCall(runner.calls, "gh issue close 64 --repo StatPan/gira --reason not planned") {
		t.Fatalf("expected not planned close: %v", runner.calls)
	}
}

func TestBuildTicketSupersedeDeclaredWorkflowMissingTransitionBlocks(t *testing.T) {
	chdirDeclaredWorkflow(t, `  statuses:
    - { name: ready, category: to_do }
    - { name: doing, category: in_progress }
    - { name: done, category: done }
  transitions:
    - { name: start, from: ready, to: doing }
    - { name: finish, from: doing, to: done }
  finish_target: done
`)
	runner := &declaredSupersedeRunner{}
	_, err := BuildTicketSupersedeReport(TicketSupersedeInput{
		Repo:             RepoRef{Owner: "StatPan", Name: "gira"},
		Ticket:           64,
		ReplacementTitle: "New gate",
		Body:             "## Goal\nDefine release gate.",
		Apply:            true,
	}, runner)
	if err == nil || !strings.Contains(err.Error(), "missing_transition") {
		t.Fatalf("expected missing_transition, got %v", err)
	}
	assertNoStatusMutation(t, runner.calls)
}
