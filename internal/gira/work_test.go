package gira

import (
	"fmt"
	"strings"
	"testing"
)

type workRunner struct {
	outputs map[string][]byte
	errs    map[string]error
	calls   []string
}

func (r *workRunner) Run(name string, args ...string) ([]byte, error) {
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
		"gh pr list --repo StatPan/gira --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": []byte(`[]`),
		"gh pr create --repo StatPan/gira --title feat: Work command --body Closes #126 --draft":                                                                                      []byte("https://github.com/StatPan/gira/pull/200\n"),
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
		"gh pr list --repo StatPan/gira --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": []byte(`[]`),
		"gh pr create --repo StatPan/gira --title feat: Work command --body Closes #126":                                                                                              []byte("https://github.com/StatPan/gira/pull/201\n"),
		"gh api repos/StatPan/gira/issues/126/labels/status:in-progress -X DELETE":                                                                                                    nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-review":                                                                                            nil,
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
		"gh pr list --repo StatPan/gira --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": []byte(`[]`),
	}}

	result, err := OpenWorkPR(repo, 126, true, false, runner)
	if err != nil {
		t.Fatalf("OpenWorkPR error: %v", err)
	}
	if !containsCall(result.Blockers, "missing_linked_pr") {
		t.Fatalf("expected missing linked PR blocker, got %+v", result.Blockers)
	}
	if containsCall(runner.calls, "gh pr create --repo StatPan/gira --title feat: Work command --body Closes #126") {
		t.Fatalf("dry-run should not create PR, calls=%v", runner.calls)
	}
}

func TestOpenWorkPRApplyReusesExistingLinkedPR(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": []byte(`[{"number":202,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/202","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`),
		"gh api repos/StatPan/gira/issues/126/labels/status:in-progress -X DELETE":                                                                                                    nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-review":                                                                                            nil,
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
		"gh pr list --repo StatPan/gira --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": []byte(`[{"number":202,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/202","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`),
		"gh api repos/StatPan/gira/issues/126/labels/status:in-progress -X DELETE":                                                                                                    nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-review":                                                                                            nil,
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
		"gh pr list --repo StatPan/gira --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": []byte(`[{"number":202,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/202","reviewDecision":"","isDraft":true,"mergeStateStatus":"UNKNOWN","statusCheckRollup":[]}]`),
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
		"gh pr list --repo StatPan/gira --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": []byte(`[{"number":203,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/203","reviewDecision":"","isDraft":true,"mergeStateStatus":"UNKNOWN","statusCheckRollup":[]}]`),
	}}

	result, err := GetWorkStatus(repo, 126, runner)
	if err != nil {
		t.Fatalf("GetWorkStatus error: %v", err)
	}
	if result.NextAction != "mark_pr_ready" {
		t.Fatalf("next action = %q", result.NextAction)
	}
}

func TestGetWorkStatusReadyWithoutPRSuggestsStartWork(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:ready"}]}`),
		"gh pr list --repo StatPan/gira --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": []byte(`[]`),
	}}

	result, err := GetWorkStatus(repo, 126, runner)
	if err != nil {
		t.Fatalf("GetWorkStatus error: %v", err)
	}
	if result.NextAction != "start_work" {
		t.Fatalf("next action = %q", result.NextAction)
	}
}
