package gira

import (
	"fmt"
	"strings"
	"testing"
)

type finishRunner struct {
	outputs map[string][][]byte
	errs    map[string]error
	calls   []string
}

func (r *finishRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, key)
	if err, ok := r.errs[key]; ok {
		return nil, err
	}
	queue := r.outputs[key]
	if len(queue) == 0 {
		return nil, fmt.Errorf("unexpected call: %s", key)
	}
	out := queue[0]
	r.outputs[key] = queue[1:]
	return out, nil
}

func TestFinishWorkApplyMarksDraftReadyThenMerges(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"https://github.com/StatPan/gira/pull/220","reviewDecision":"APPROVED","isDraft":true,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"https://github.com/StatPan/gira/pull/220","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"MERGED","url":"https://github.com/StatPan/gira/pull/220","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"UNKNOWN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"gh pr ready 220 --repo StatPan/gira":                          {nil},
		"gh pr merge 220 --repo StatPan/gira --squash --delete-branch": {nil},
		"git remote get-url origin":                                    {[]byte("git@github.com:StatPan/gira.git\n")},
		"git branch --show-current":                                    {[]byte("issue-219-finish\n")},
		"git status --porcelain":                                       {[]byte("")},
		"git checkout main":                                            {nil},
		"git pull --ff-only origin main":                               {nil},
		"gh api repos/StatPan/gira/issues/219":                         {[]byte(`{"number":219,"title":"Finish","state":"closed","labels":[]}`)},
	}, errs: map[string]error{}}

	result, err := FinishWork(repo, 219, false, 0, runner)
	if err != nil {
		t.Fatalf("FinishWork error: %v", err)
	}
	if !result.Merged || result.PRNumber != 220 || len(result.Blockers) != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	for _, want := range []string{
		"gh pr ready 220 --repo StatPan/gira",
		"gh pr merge 220 --repo StatPan/gira --squash --delete-branch",
		"git checkout main",
		"git pull --ff-only origin main",
	} {
		if !containsCall(runner.calls, want) {
			t.Fatalf("missing call %q in %v", want, runner.calls)
		}
	}
}

func TestFinishWorkDryRunPendingChecksReportsBlockerWithoutWaiting(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"","status":"IN_PROGRESS"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"","status":"IN_PROGRESS"}]}]`),
		},
		"gh api repos/StatPan/gira/issues/219": {[]byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-review"}]}`)},
	}, errs: map[string]error{}}

	result, err := FinishWork(repo, 219, true, 0, runner)
	if err != nil {
		t.Fatalf("FinishWork dry-run error: %v", err)
	}
	if !containsString(result.Blockers, "checks_pending") {
		t.Fatalf("expected checks_pending blocker, got %+v", result.Blockers)
	}
	if containsCall(runner.calls, "gh pr merge 220 --repo StatPan/gira --squash --delete-branch") {
		t.Fatalf("dry-run pending checks should not merge: %v", runner.calls)
	}
}

func TestFinishWorkDryRunReadySuggestsFinishApply(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"git remote get-url origin":            {[]byte("git@github.com:StatPan/gira.git\n")},
		"git branch --show-current":            {[]byte("issue-219-finish\n")},
		"git status --porcelain":               {[]byte("")},
		"gh api repos/StatPan/gira/issues/219": {[]byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-review"}]}`)},
	}, errs: map[string]error{}}

	result, err := FinishWork(repo, 219, true, 0, runner)
	if err != nil {
		t.Fatalf("FinishWork error: %v", err)
	}
	if result.NextStep != "gira ticket finish --repo StatPan/gira --ticket 219 --apply" {
		t.Fatalf("unexpected next step: %q", result.NextStep)
	}
}

func TestFinishWorkApplyReviewRequiredBlocksMerge(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"REVIEW_REQUIRED","isDraft":false,"mergeStateStatus":"BLOCKED","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"REVIEW_REQUIRED","isDraft":false,"mergeStateStatus":"BLOCKED","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"gh api repos/StatPan/gira/issues/219": {[]byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-review"}]}`)},
	}, errs: map[string]error{}}

	result, err := FinishWork(repo, 219, false, 0, runner)
	if err == nil || !strings.Contains(err.Error(), "review") {
		t.Fatalf("expected review blocker error, got result=%+v err=%v", result, err)
	}
	if containsCall(runner.calls, "gh pr merge 220 --repo StatPan/gira --squash --delete-branch") {
		t.Fatalf("blocked PR should not merge: %v", runner.calls)
	}
}

func TestFinishWorkMissingLinkedPRSuggestsOpenPR(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": {
			[]byte(`[]`),
			[]byte(`[]`),
		},
		"gh api repos/StatPan/gira/issues/219": {[]byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-progress"}]}`)},
	}, errs: map[string]error{}}

	result, err := FinishWork(repo, 219, true, 0, runner)
	if err != nil {
		t.Fatalf("FinishWork error: %v", err)
	}
	if !containsString(result.Blockers, "missing_linked_pr") || !strings.Contains(result.NextStep, "ticket pr") {
		t.Fatalf("unexpected missing PR result: %+v", result)
	}
}

func TestFinishWorkDryRunSkipsLocalSyncOnDirtyWorktree(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"git remote get-url origin":            {[]byte("https://github.com/StatPan/gira.git\n")},
		"git branch --show-current":            {[]byte("issue-219-finish\n")},
		"git status --porcelain":               {[]byte(" M README.md\n")},
		"gh api repos/StatPan/gira/issues/219": {[]byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-review"}]}`)},
	}, errs: map[string]error{}}

	result, err := FinishWork(repo, 219, true, 0, runner)
	if err != nil {
		t.Fatalf("FinishWork error: %v", err)
	}
	if !result.LocalSync.Skipped || result.LocalSync.Reason != "dirty_worktree" {
		t.Fatalf("expected dirty local sync skip, got %+v", result.LocalSync)
	}
}

func TestFinishWorkAlreadyMergedIsIdempotent(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"MERGED","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"UNKNOWN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"MERGED","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"UNKNOWN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"git remote get-url origin":            {[]byte("git@github.com:StatPan/gira.git\n")},
		"git branch --show-current":            {[]byte("main\n")},
		"git status --porcelain":               {[]byte("")},
		"gh api repos/StatPan/gira/issues/219": {[]byte(`{"number":219,"title":"Finish","state":"closed","labels":[]}`)},
	}, errs: map[string]error{}}

	result, err := FinishWork(repo, 219, true, 0, runner)
	if err != nil {
		t.Fatalf("FinishWork error: %v", err)
	}
	if !result.AlreadyDone || !result.Merged || len(result.Blockers) != 0 {
		t.Fatalf("unexpected already merged result: %+v", result)
	}
}
