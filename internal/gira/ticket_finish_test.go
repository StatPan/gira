package gira

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type finishRunner struct {
	outputs map[string][][]byte
	errs    map[string]error
	mu      sync.Mutex
	calls   []string
}

func (r *finishRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	r.mu.Lock()
	defer r.mu.Unlock()
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
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
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
		"gh api repos/StatPan/gira/issues/219": {
			[]byte(`{"number":219,"title":"Finish","state":"closed","labels":[]}`),
			[]byte(`{"number":219,"title":"Finish","state":"closed","labels":[]}`),
		},
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

func TestFinishWorkApplyRemovesActiveStatusFromClosedIssue(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"MERGED","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"UNKNOWN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"gh pr merge 220 --repo StatPan/gira --squash --delete-branch": {nil},
		"git remote get-url origin":                                    {[]byte("git@github.com:StatPan/gira.git\n")},
		"git branch --show-current":                                    {[]byte("issue-219-finish\n")},
		"git status --porcelain":                                       {[]byte("")},
		"git checkout main":                                            {nil},
		"git pull --ff-only origin main":                               {nil},
		"gh api repos/StatPan/gira/issues/219": {
			[]byte(`{"number":219,"title":"Finish","state":"closed","labels":[{"name":"status:in-review"},{"name":"type:task"}]}`),
			[]byte(`{"number":219,"title":"Finish","state":"closed","labels":[{"name":"type:task"}]}`),
		},
		"gh label list --repo StatPan/gira --json name --limit 1000":            {[]byte(`[{"name":"status:ready"}]`)},
		"gh issue edit 219 --repo StatPan/gira --remove-label status:in-review": {nil},
	}, errs: map[string]error{}}

	result, err := FinishWork(repo, 219, false, 0, runner)
	if err != nil {
		t.Fatalf("FinishWork error: %v", err)
	}
	if !containsCall(runner.calls, "gh issue edit 219 --repo StatPan/gira --remove-label status:in-review") {
		t.Fatalf("missing status normalization call: %v", runner.calls)
	}
	if !finishActionStatus(result.Actions, "ticket:normalize-status", "applied") {
		t.Fatalf("missing normalize action: %+v", result.Actions)
	}
}

func TestFinishWorkApplyAddsDoneLabelWhenAvailable(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"MERGED","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"UNKNOWN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"gh pr merge 220 --repo StatPan/gira --squash --delete-branch": {nil},
		"git remote get-url origin":                                    {[]byte("git@github.com:StatPan/gira.git\n")},
		"git branch --show-current":                                    {[]byte("issue-219-finish\n")},
		"git status --porcelain":                                       {[]byte("")},
		"git checkout main":                                            {nil},
		"git pull --ff-only origin main":                               {nil},
		"gh api repos/StatPan/gira/issues/219": {
			[]byte(`{"number":219,"title":"Finish","state":"closed","labels":[{"name":"status:in-review"}]}`),
			[]byte(`{"number":219,"title":"Finish","state":"closed","labels":[{"name":"status:done"}]}`),
		},
		"gh label list --repo StatPan/gira --json name --limit 1000":                                    {[]byte(`[{"name":"status:done"}]`)},
		"gh issue edit 219 --repo StatPan/gira --add-label status:done --remove-label status:in-review": {nil},
	}, errs: map[string]error{}}

	result, err := FinishWork(repo, 219, false, 0, runner)
	if err != nil {
		t.Fatalf("FinishWork error: %v", err)
	}
	if !containsCall(runner.calls, "gh issue edit 219 --repo StatPan/gira --add-label status:done --remove-label status:in-review") {
		t.Fatalf("missing done status normalization call: %v", runner.calls)
	}
	if !strings.Contains(result.Actions[len(result.Actions)-2].Detail, "add=status:done") {
		t.Fatalf("normalize action missing done detail: %+v", result.Actions)
	}
}

func TestFinishWorkDryRunPendingChecksReportsBlockerWithoutWaiting(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"","status":"IN_PROGRESS"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"","status":"IN_PROGRESS"}]}]`),
		},
		"gh api repos/StatPan/gira/issues/219": {
			[]byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-review"}]}`),
			[]byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-review"}]}`),
		},
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
	if got := countFinishCall(runner.calls, "gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20"); got != 1 {
		t.Fatalf("dry-run pending checks should reuse PR status, got %d pr list calls: %v", got, runner.calls)
	}
}

func TestFinishWorkDryRunReadySuggestsFinishApply(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"git remote get-url origin": {[]byte("git@github.com:StatPan/gira.git\n")},
		"git branch --show-current": {[]byte("issue-219-finish\n")},
		"git status --porcelain":    {[]byte("")},
		"gh api repos/StatPan/gira/issues/219": {
			[]byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-review"}]}`),
			[]byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-review"}]}`),
		},
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
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
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

func TestFinishWorkBlocksUnexpectedPRHeadBinding(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"feature/unrelated","baseRefName":"main","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"feature/unrelated","baseRefName":"main","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"gh api repos/StatPan/gira/issues/219": {[]byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-review"}]}`)},
	}, errs: map[string]error{}}

	result, err := FinishWork(repo, 219, false, 0, runner)
	if err == nil || !strings.Contains(err.Error(), "pr_binding") {
		t.Fatalf("expected PR binding blocker, got result=%+v err=%v", result, err)
	}
	if !containsString(result.Blockers, "pr_binding") {
		t.Fatalf("expected pr_binding blocker: %+v", result.Blockers)
	}
	if result.FinalStatus.PRNumber != 220 || result.FinalStatus.Blockers == nil {
		t.Fatalf("expected final status to retain linked PR context: %+v", result.FinalStatus)
	}
	if containsCall(runner.calls, "gh pr merge 220 --repo StatPan/gira --squash --delete-branch") {
		t.Fatalf("unexpectedly merged untrusted PR binding: %v", runner.calls)
	}
}

func TestFinishWorkMissingLinkedPRSuggestsOpenPR(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
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
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
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
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"MERGED","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"UNKNOWN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"MERGED","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"UNKNOWN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"git remote get-url origin": {[]byte("git@github.com:StatPan/gira.git\n")},
		"git branch --show-current": {[]byte("main\n")},
		"git status --porcelain":    {[]byte("")},
		"gh api repos/StatPan/gira/issues/219": {
			[]byte(`{"number":219,"title":"Finish","state":"closed","labels":[]}`),
			[]byte(`{"number":219,"title":"Finish","state":"closed","labels":[]}`),
		},
	}, errs: map[string]error{}}

	result, err := FinishWork(repo, 219, true, 0, runner)
	if err != nil {
		t.Fatalf("FinishWork error: %v", err)
	}
	if !result.AlreadyDone || !result.Merged || len(result.Blockers) != 0 {
		t.Fatalf("unexpected already merged result: %+v", result)
	}
}

func TestFinishWorkJiraPrimaryApplyTransitionsDoneAfterMerge(t *testing.T) {
	writeJiraFinishConfig(t)
	posts := fakeJiraFinishAPI(t, "ABC-123", "In Progress", `{"transitions":[{"id":"31","name":"Done","to":{"name":"Done"},"fields":{}}]}`)
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"MERGED","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"UNKNOWN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"gh api repos/StatPan/gira/issues/219": {
			[]byte(`{"number":219,"title":"Finish","body":"Jira-Key: ABC-123\n","state":"open","labels":[{"name":"status:in-review"}]}`),
			[]byte(`{"number":219,"title":"Finish","body":"Jira-Key: ABC-123\n","state":"closed","labels":[]}`),
		},
		"gh pr merge 220 --repo StatPan/gira --squash --delete-branch": {nil},
		"git remote get-url origin":                                    {[]byte("git@github.com:StatPan/gira.git\n")},
		"git branch --show-current":                                    {[]byte("issue-219-finish\n")},
		"git status --porcelain":                                       {[]byte("")},
		"git checkout main":                                            {nil},
		"git pull --ff-only origin main":                               {nil},
	}, errs: map[string]error{}}

	result, err := FinishWork(repo, 219, false, 0, runner)
	if err != nil {
		t.Fatalf("FinishWork error: %v", err)
	}
	if !result.Merged || result.JiraKey != "ABC-123" || result.JiraTransition == nil || !result.JiraTransition.Applied {
		t.Fatalf("expected merged Jira Done apply result: %+v", result)
	}
	if len(*posts) != 1 || !strings.Contains((*posts)[0], `"id":"31"`) {
		t.Fatalf("expected one Jira transition POST with id 31, got %v", *posts)
	}
	if !finishActionStatus(result.Actions, "jira:done", "applied") {
		t.Fatalf("missing applied jira:done action: %+v", result.Actions)
	}
}

func TestFinishWorkJiraPrimaryDryRunBlocksDoneUntilPRMerged(t *testing.T) {
	writeJiraFinishConfig(t)
	posts := fakeJiraFinishAPI(t, "ABC-123", "In Progress", `{"transitions":[{"id":"31","name":"Done","to":{"name":"Done"},"fields":{}}]}`)
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"gh api repos/StatPan/gira/issues/219": {
			[]byte(`{"number":219,"title":"Finish","body":"Jira-Key: ABC-123\n","state":"open","labels":[{"name":"status:in-review"}]}`),
			[]byte(`{"number":219,"title":"Finish","body":"Jira-Key: ABC-123\n","state":"open","labels":[{"name":"status:in-review"}]}`),
		},
		"git remote get-url origin": {[]byte("git@github.com:StatPan/gira.git\n")},
		"git branch --show-current": {[]byte("issue-219-finish\n")},
		"git status --porcelain":    {[]byte("")},
	}, errs: map[string]error{}}

	result, err := FinishWork(repo, 219, true, 0, runner)
	if err != nil {
		t.Fatalf("FinishWork error: %v", err)
	}
	if !containsString(result.Blockers, "unmerged_pr") || !finishActionStatus(result.Actions, "jira:done", "blocked") {
		t.Fatalf("expected unmerged PR Jira Done blocker: %+v", result)
	}
	if result.JiraTransition != nil && result.JiraTransition.Decision == "direct_transition" {
		t.Fatalf("Jira Done transition should not be planned before merge evidence exists: %+v", result)
	}
	if len(*posts) != 0 {
		t.Fatalf("dry-run should not POST Jira transition, got %v", *posts)
	}
	if !finishActionStatus(result.Actions, "pr:merge", "planned") {
		t.Fatalf("missing planned PR merge action: %+v", result.Actions)
	}
}

func TestFinishWorkJiraPrimaryBlocksDoneWhenGitHubEvidenceIncomplete(t *testing.T) {
	cases := []struct {
		name    string
		prJSON  string
		blocker string
	}{
		{name: "missing PR", prJSON: `[]`, blocker: "missing_linked_pr"},
		{name: "draft PR", prJSON: `[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":true,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`, blocker: "draft"},
		{name: "failing checks", prJSON: `[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"UNSTABLE","statusCheckRollup":[{"conclusion":"FAILURE","status":"COMPLETED"}]}]`, blocker: "checks"},
		{name: "pending checks", prJSON: `[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"","status":"IN_PROGRESS"}]}]`, blocker: "checks_pending"},
		{name: "review blocker", prJSON: `[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"REVIEW_REQUIRED","isDraft":false,"mergeStateStatus":"BLOCKED","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`, blocker: "review"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeJiraFinishConfig(t)
			posts := fakeJiraFinishAPI(t, "ABC-123", "In Progress", `{"transitions":[{"id":"31","name":"Done","to":{"name":"Done"},"fields":{}}]}`)
			repo := RepoRef{Owner: "StatPan", Name: "gira"}
			runner := &finishRunner{outputs: map[string][][]byte{
				"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
					[]byte(tc.prJSON),
				},
				"gh api repos/StatPan/gira/issues/219": {
					[]byte(`{"number":219,"title":"Finish","body":"Jira-Key: ABC-123\n","state":"open","labels":[{"name":"status:in-review"}]}`),
					[]byte(`{"number":219,"title":"Finish","body":"Jira-Key: ABC-123\n","state":"open","labels":[{"name":"status:in-review"}]}`),
				},
			}, errs: map[string]error{}}

			result, err := FinishWork(repo, 219, true, 0, runner)
			if err != nil {
				t.Fatalf("FinishWork dry-run error: %v", err)
			}
			if !containsString(result.Blockers, tc.blocker) || !finishActionStatus(result.Actions, "jira:done", "blocked") {
				t.Fatalf("expected blocker %q and blocked Jira action, got result=%+v", tc.blocker, result)
			}
			if result.JiraTransition != nil && result.JiraTransition.Decision == "direct_transition" {
				t.Fatalf("Jira transition should not be planned while GitHub evidence is incomplete: %+v", result.JiraTransition)
			}
			if len(*posts) != 0 {
				t.Fatalf("blocked finish should not POST Jira transition, got %v", *posts)
			}
		})
	}
}

func TestFinishWorkJiraPrimaryBlocksMissingMirrorIssue(t *testing.T) {
	writeJiraFinishConfig(t)
	posts := fakeJiraFinishAPI(t, "ABC-123", "In Progress", `{"transitions":[{"id":"31","name":"Done","to":{"name":"Done"},"fields":{}}]}`)
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"gh api repos/StatPan/gira/issues/219": {
			[]byte(`{"number":219,"title":"Finish","body":"No Jira key here\n","state":"open","labels":[{"name":"status:in-review"}]}`),
			[]byte(`{"number":219,"title":"Finish","body":"No Jira key here\n","state":"open","labels":[{"name":"status:in-review"}]}`),
		},
	}, errs: map[string]error{}}

	result, err := FinishWork(repo, 219, true, 0, runner)
	if err != nil {
		t.Fatalf("FinishWork dry-run error: %v", err)
	}
	if !containsString(result.Blockers, "missing_mirror_issue") || !finishActionStatus(result.Actions, "jira:done", "blocked") {
		t.Fatalf("expected missing mirror blocker, got %+v", result)
	}
	if len(*posts) != 0 {
		t.Fatalf("missing mirror issue should not POST Jira transition, got %v", *posts)
	}
}

func writeJiraFinishConfig(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	writeTestFile(t, filepath.Join(root, "gira", "repos", "StatPan", "gira.yaml"), `repo: StatPan/gira
providers:
  jira:
    enabled: true
    mode: primary
    base_url: https://jira.example
    project_key: ABC
    source_of_truth:
      planning: jira
      status: jira
      execution: github
    status_map:
      - gira_status: in_progress
        jira_statuses:
          - In Progress
      - gira_status: done
        jira_statuses:
          - Done
`)
}

func fakeJiraFinishAPI(t *testing.T, key string, status string, transitions string) *[]string {
	t.Helper()
	t.Setenv("JIRA_EMAIL", "alice@example.com")
	t.Setenv("JIRA_API_TOKEN", "secret-token")
	restoreGet := jiraAPIGet
	restorePost := jiraAPIPost
	t.Cleanup(func() {
		jiraAPIGet = restoreGet
		jiraAPIPost = restorePost
	})
	posts := []string{}
	jiraAPIGet = func(apiBase string, path string, query map[string]string, email string, token string) ([]byte, error) {
		if apiBase != "https://jira.example" || email != "alice@example.com" || token != "secret-token" {
			t.Fatalf("unexpected Jira GET apiBase=%s path=%s query=%v email=%s token=%s", apiBase, path, query, email, token)
		}
		switch path {
		case "/rest/api/3/issue/" + key:
			return []byte(`{"key":"` + key + `","fields":{"summary":"Finish","status":{"name":"` + status + `"}}}`), nil
		case "/rest/api/3/issue/" + key + "/transitions":
			return []byte(transitions), nil
		default:
			t.Fatalf("unexpected Jira GET path: %s", path)
			return nil, nil
		}
	}
	jiraAPIPost = func(apiBase string, path string, body []byte, email string, token string) ([]byte, error) {
		if apiBase != "https://jira.example" || path != "/rest/api/3/issue/"+key+"/transitions" || email != "alice@example.com" || token != "secret-token" {
			t.Fatalf("unexpected Jira POST apiBase=%s path=%s body=%s email=%s token=%s", apiBase, path, string(body), email, token)
		}
		posts = append(posts, string(body))
		return []byte(`{}`), nil
	}
	return &posts
}

func finishActionStatus(actions []WorkFinishAction, action string, status string) bool {
	for _, item := range actions {
		if item.Action == action && item.Status == status {
			return true
		}
	}
	return false
}

func countFinishCall(calls []string, target string) int {
	count := 0
	for _, call := range calls {
		if call == target {
			count++
		}
	}
	return count
}
