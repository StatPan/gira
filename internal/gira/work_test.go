package gira

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type workRunner struct {
	outputs map[string][]byte
	errs    map[string]error
	delays  map[string]time.Duration
	mu      sync.Mutex
	calls   []string
}

func (r *workRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	r.mu.Lock()
	r.calls = append(r.calls, key)
	delay := r.delays[key]
	err, hasErr := r.errs[key]
	out, hasOut := r.outputs[key]
	r.mu.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}
	if hasErr {
		return nil, err
	}
	if hasOut {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected call: %s", key)
}

func TestStartWorkApplyReadyIssueCreatesBranchAndMovesInProgress(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueJSON := []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:ready"}]}`)
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126":                                               issueJSON,
		"git checkout -b issue-126-work-command":                                             nil,
		"gh api repos/StatPan/gira/issues/126/labels/status:ready -X DELETE":                 nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-progress": nil,
	}, errs: map[string]error{
		"git show-ref --verify --quiet refs/heads/issue-126-work-command": fmt.Errorf("exit status 1"),
		"git ls-remote --exit-code --heads origin issue-126-work-command": fmt.Errorf("exit status 2"),
	}}

	result, err := StartWork(repo, 126, false, runner)
	if err != nil {
		t.Fatalf("StartWork error: %v", err)
	}
	if result.Status != "In progress" || !result.CreatedBranch {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !containsCall(runner.calls, "gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-progress") {
		t.Fatalf("missing status apply call: %v", runner.calls)
	}
}

func TestStartWorkFailsMissingReady(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"type:task"}]}`),
	}}

	_, err := StartWork(repo, 126, true, runner)
	if err == nil || !strings.Contains(err.Error(), "missing status:ready") {
		t.Fatalf("expected missing ready error, got %v", err)
	}
}

func TestStartWorkApplyReusesExistingBranch(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueJSON := []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:ready"}]}`)
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126":                                               issueJSON,
		"git show-ref --verify --quiet refs/heads/issue-126-work-command":                    nil,
		"git ls-remote --exit-code --heads origin issue-126-work-command":                    nil,
		"git checkout issue-126-work-command":                                                nil,
		"gh api repos/StatPan/gira/issues/126/labels/status:ready -X DELETE":                 nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-progress": nil,
	}}

	result, err := StartWork(repo, 126, false, runner)
	if err != nil {
		t.Fatalf("StartWork error: %v", err)
	}
	if result.CreatedBranch {
		t.Fatalf("expected branch reuse, got created")
	}
	if !containsCall(runner.calls, "git checkout issue-126-work-command") {
		t.Fatalf("expected checkout existing branch, calls=%v", runner.calls)
	}
}

func TestStartWorkApplyRerunReusesInProgressBranch(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueJSON := []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`)
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126":                            issueJSON,
		"git show-ref --verify --quiet refs/heads/issue-126-work-command": nil,
		"git ls-remote --exit-code --heads origin issue-126-work-command": nil,
		"git checkout issue-126-work-command":                             nil,
	}}

	result, err := StartWork(repo, 126, false, runner)
	if err != nil {
		t.Fatalf("StartWork error: %v", err)
	}
	if result.CreatedBranch || result.Status != "In progress" || result.NextStatus != "In progress" {
		t.Fatalf("expected idempotent in-progress reuse, got %+v", result)
	}
	if !containsCall(runner.calls, "git checkout issue-126-work-command") {
		t.Fatalf("expected checkout existing branch, calls=%v", runner.calls)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "/labels") {
			t.Fatalf("in-progress rerun should not mutate labels, calls=%v", runner.calls)
		}
	}
}

func TestOpenWorkPRApplyDraftKeepsInProgress(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueJSON := []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`)
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": issueJSON,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": []byte(`[]`),
		"git branch --show-current":                                                              []byte("issue-126-work-command\n"),
		"git rev-parse --abbrev-ref --symbolic-full-name @{u}":                                   []byte("origin/issue-126-work-command\n"),
		"gh pr create --repo StatPan/gira --title feat: Work command --body Closes #126 --draft": []byte("https://github.com/StatPan/gira/pull/200\n"),
	}}

	result, err := OpenWorkPR(repo, 126, false, true, runner)
	if err != nil {
		t.Fatalf("OpenWorkPR error: %v", err)
	}
	if result.NextStatus != "In progress" || result.PRNumber != 200 || !containsCall(result.Blockers, "draft") {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestOpenWorkPRApplyNonDraftMovesInReview(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueJSON := []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`)
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": issueJSON,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": []byte(`[]`),
		"git branch --show-current":                                                        []byte("issue-126-work-command\n"),
		"git rev-parse --abbrev-ref --symbolic-full-name @{u}":                             []byte("origin/issue-126-work-command\n"),
		"gh pr create --repo StatPan/gira --title feat: Work command --body Closes #126":   []byte("https://github.com/StatPan/gira/pull/201\n"),
		"gh api repos/StatPan/gira/issues/126/labels/status:in-progress -X DELETE":         nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-review": nil,
	}}

	result, err := OpenWorkPR(repo, 126, false, false, runner)
	if err != nil {
		t.Fatalf("OpenWorkPR error: %v", err)
	}
	if result.NextStatus != "In review" || result.PRNumber != 201 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestOpenWorkPRDryRunReportsMissingLinkedPR(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": []byte(`[]`),
		"git branch --show-current": []byte("issue-126-work-command\n"),
	}, errs: map[string]error{
		"git rev-parse --abbrev-ref --symbolic-full-name @{u}": fmt.Errorf("fatal: no upstream configured: exit status 128"),
	}}

	result, err := OpenWorkPR(repo, 126, true, false, runner)
	if err != nil {
		t.Fatalf("OpenWorkPR error: %v", err)
	}
	if !containsCall(result.Blockers, "missing_linked_pr") {
		t.Fatalf("expected missing linked PR blocker, got %+v", result.Blockers)
	}
	if !containsCall(result.Blockers, "branch_push_required") || result.BranchPush != "planned" {
		t.Fatalf("expected planned branch push blocker, got %+v", result)
	}
	if containsCall(runner.calls, "gh pr create --repo StatPan/gira --title feat: Work command --body Closes #126") {
		t.Fatalf("dry-run should not create PR, calls=%v", runner.calls)
	}
	if containsCall(runner.calls, "git push -u origin issue-126-work-command") {
		t.Fatalf("dry-run should not push branch, calls=%v", runner.calls)
	}
}

func TestOpenWorkPRApplyPushesUnpushedTicketBranch(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": []byte(`[]`),
		"git branch --show-current":                 []byte("issue-126-work-command\n"),
		"git push -u origin issue-126-work-command": nil,
		"gh pr create --repo StatPan/gira --title feat: Work command --body Closes #126 --draft": []byte("https://github.com/StatPan/gira/pull/204\n"),
	}, errs: map[string]error{
		"git rev-parse --abbrev-ref --symbolic-full-name @{u}": fmt.Errorf("fatal: no upstream configured: exit status 128"),
	}}

	result, err := OpenWorkPR(repo, 126, false, true, runner)
	if err != nil {
		t.Fatalf("OpenWorkPR error: %v", err)
	}
	if result.Branch != "issue-126-work-command" || result.BranchPush != "applied" || result.PRNumber != 204 {
		t.Fatalf("expected pushed branch and created PR, got %+v", result)
	}
	pushIndex := callIndex(runner.calls, "git push -u origin issue-126-work-command")
	createIndex := callIndex(runner.calls, "gh pr create --repo StatPan/gira --title feat: Work command --body Closes #126 --draft")
	if pushIndex < 0 || createIndex < 0 || pushIndex > createIndex {
		t.Fatalf("expected push before PR create, calls=%v", runner.calls)
	}
}

func TestOpenWorkPRApplyRejectsWrongBranch(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": []byte(`[]`),
		"git branch --show-current": []byte("feature/other\n"),
	}}

	_, err := OpenWorkPR(repo, 126, false, false, runner)
	if err == nil || !strings.Contains(err.Error(), "not the ticket branch") {
		t.Fatalf("expected wrong branch error, got %v", err)
	}
	if containsCall(runner.calls, "git push -u origin feature/other") {
		t.Fatalf("wrong branch should not be pushed, calls=%v", runner.calls)
	}
}

func callIndex(calls []string, target string) int {
	for i, call := range calls {
		if call == target {
			return i
		}
	}
	return -1
}

func TestOpenWorkPRApplyReusesExistingLinkedPR(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": []byte(`[{"number":202,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/202","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`),
		"gh api repos/StatPan/gira/issues/126/labels/status:in-progress -X DELETE":                                                                                                                nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-review":                                                                                                        nil,
	}}

	result, err := OpenWorkPR(repo, 126, false, false, runner)
	if err != nil {
		t.Fatalf("OpenWorkPR error: %v", err)
	}
	if result.Created || result.PRNumber != 202 {
		t.Fatalf("expected existing PR reuse, got %+v", result)
	}
	for _, call := range runner.calls {
		if strings.HasPrefix(call, "gh pr create ") {
			t.Fatalf("unexpected PR create call: %v", runner.calls)
		}
	}
}

func TestOpenWorkPRApplyDraftFlagReusesExistingNonDraftPR(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": []byte(`[{"number":202,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/202","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`),
		"gh api repos/StatPan/gira/issues/126/labels/status:in-progress -X DELETE":                                                                                                                nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-review":                                                                                                        nil,
	}}

	result, err := OpenWorkPR(repo, 126, false, true, runner)
	if err != nil {
		t.Fatalf("OpenWorkPR error: %v", err)
	}
	if result.Created || result.Draft || result.NextStatus != "In review" || result.PRNumber != 202 {
		t.Fatalf("expected existing non-draft PR to stay non-draft, got %+v", result)
	}
	for _, call := range runner.calls {
		if strings.HasPrefix(call, "gh pr create ") {
			t.Fatalf("unexpected PR create call: %v", runner.calls)
		}
	}
}

func TestOpenWorkPRExistingDraftStaysInProgress(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:ready"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": []byte(`[{"number":202,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/202","reviewDecision":"","isDraft":true,"mergeStateStatus":"UNKNOWN","statusCheckRollup":[]}]`),
		"gh api repos/StatPan/gira/issues/126/labels/status:ready -X DELETE":                 nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-progress": nil,
	}}

	result, err := OpenWorkPR(repo, 126, false, false, runner)
	if err != nil {
		t.Fatalf("OpenWorkPR error: %v", err)
	}
	if result.NextStatus != "In progress" || !result.Draft {
		t.Fatalf("expected existing draft to stay in progress, got %+v", result)
	}
	if containsCall(runner.calls, "gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-review") {
		t.Fatalf("draft PR should not move to in-review, calls=%v", runner.calls)
	}
}

func TestGetWorkStatusReportsNextAction(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": []byte(`[{"number":203,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/203","reviewDecision":"","isDraft":true,"mergeStateStatus":"UNKNOWN","statusCheckRollup":[]}]`),
	}}

	result, err := GetWorkStatus(repo, 126, runner)
	if err != nil {
		t.Fatalf("GetWorkStatus error: %v", err)
	}
	if result.NextAction != "mark_pr_ready" {
		t.Fatalf("next action = %q", result.NextAction)
	}
}

func TestGetWorkStatusFetchesIssueAndPRConcurrently(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueCall := "gh api repos/StatPan/gira/issues/126"
	prCall := "gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20"
	runner := &workRunner{
		outputs: map[string][]byte{
			issueCall: []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
			prCall:    []byte(`[{"number":203,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/203","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`),
		},
		delays: map[string]time.Duration{
			issueCall: 80 * time.Millisecond,
			prCall:    80 * time.Millisecond,
		},
	}

	start := time.Now()
	result, err := GetWorkStatus(repo, 126, runner)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("GetWorkStatus error: %v", err)
	}
	if result.PRNumber != 203 || result.NextAction != "merge_when_policy_allows" {
		t.Fatalf("unexpected status: %+v", result)
	}
	if elapsed > 140*time.Millisecond {
		t.Fatalf("GetWorkStatus took %s, want concurrent fetch under 140ms", elapsed)
	}
}

func TestGetWorkStatusReadyWithoutPRSuggestsStartWork(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:ready"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": []byte(`[]`),
	}}

	result, err := GetWorkStatus(repo, 126, runner)
	if err != nil {
		t.Fatalf("GetWorkStatus error: %v", err)
	}
	if result.NextAction != "start_work" {
		t.Fatalf("next action = %q", result.NextAction)
	}
}

func TestGetWorkStatusClosedIssueWithMergedPRReportsDone(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/184": []byte(`{"number":184,"title":"Workspace backlog","state":"closed","labels":[{"name":"type:story"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 184 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": []byte(`[{"number":185,"title":"x","body":"Closes #184","state":"MERGED","url":"https://github.com/StatPan/gira/pull/185","reviewDecision":"","isDraft":false,"mergeStateStatus":"UNKNOWN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
	}}

	result, err := GetWorkStatus(repo, 184, runner)
	if err != nil {
		t.Fatalf("GetWorkStatus error: %v", err)
	}
	if result.NextAction != "done" || result.Status != "Done" || result.PRNumber != 185 {
		t.Fatalf("expected done merged PR status, got %+v", result)
	}
	if got := workStatusNextStep(result); got != "ticket is done" {
		t.Fatalf("next step = %q", got)
	}
}

func TestGetWorkStatusClosedIssueWithoutPRDoesNotSuggestStart(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/188": []byte(`{"number":188,"title":"Closed manually","state":"closed","labels":[]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 188 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": []byte(`[]`),
	}}

	result, err := GetWorkStatus(repo, 188, runner)
	if err != nil {
		t.Fatalf("GetWorkStatus error: %v", err)
	}
	if result.NextAction != "closed" || result.Status != "Closed" {
		t.Fatalf("expected closed next action, got %+v", result)
	}
	if strings.Contains(workStatusNextStep(result), "work start") {
		t.Fatalf("closed ticket should not suggest work start: %+v", result)
	}
}

func TestWorkStatusNextStepExplainsCheckBlockers(t *testing.T) {
	result := WorkStatusResult{Repo: "StatPan/gira", Issue: 126, NextAction: "wait_for_checks"}

	if got := workStatusNextStep(result); got != "wait for required checks to finish or fix failing checks" {
		t.Fatalf("next step = %q", got)
	}
}
