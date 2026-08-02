package gira

import (
	"fmt"
	"path/filepath"
	"strconv"
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
	if len(queue) == 0 && key == "gh api repos/StatPan/gira/pulls/220" {
		return []byte(`{"number":220,"body":"Closes #219","state":"closed","merged_at":"2026-07-22T00:00:00Z","merge_commit_sha":"merge220","html_url":"https://github.com/StatPan/gira/pull/220","head":{"ref":"issue-219-finish","sha":"head220"},"base":{"ref":"main"}}`), nil
	}
	if len(queue) == 0 && strings.HasPrefix(key, "gh issue comment ") {
		return nil, nil
	}
	if len(queue) == 0 {
		return nil, fmt.Errorf("unexpected call: %s", key)
	}
	out := queue[0]
	r.outputs[key] = queue[1:]
	return out, nil
}

func TestFinishWorkApplyMarksDraftReadyAndStopsBeforeMerge(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"https://github.com/StatPan/gira/pull/220","reviewDecision":"APPROVED","isDraft":true,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"https://github.com/StatPan/gira/pull/220","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"gh pr ready 220 --repo StatPan/gira": {nil},
		"gh api repos/StatPan/gira/issues/219": {
			[]byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-review"}]}`),
		},
	}, errs: map[string]error{}}

	result, err := FinishWork(repo, 219, false, 0, runner)
	if err != nil {
		t.Fatalf("FinishWork error: %v", err)
	}
	if result.Merged || result.PRNumber != 220 || len(result.Blockers) != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !containsCall(runner.calls, "gh pr ready 220 --repo StatPan/gira") {
		t.Fatalf("missing ready call in %v", runner.calls)
	}
	if containsCall(runner.calls, "gh pr merge 220 --repo StatPan/gira --squash --delete-branch") {
		t.Fatalf("Draft finish apply expanded into merge: %v", runner.calls)
	}
	if result.NextStep != "gira ticket finish --repo StatPan/gira --ticket 219 --dry-run" {
		t.Fatalf("next step = %q, want a new dry-run", result.NextStep)
	}
	if !finishActionStatus(result.Actions, "finish:intent", "observed") || !finishActionStatus(result.Actions, "pr:ready", "applied") {
		t.Fatalf("missing bounded Draft actions: %+v", result.Actions)
	}
	if len(result.Warnings) == 0 || !strings.Contains(result.Warnings[0], "requires a new dry-run") {
		t.Fatalf("missing Draft terminal-intent warning: %+v", result.Warnings)
	}
	for _, unexpected := range []string{
		"git remote get-url origin",
		"git checkout main",
		"git pull --ff-only origin main",
	} {
		if containsCall(runner.calls, unexpected) {
			t.Fatalf("default finish should not sync local checkout; unexpected %q in %v", unexpected, runner.calls)
		}
	}
	if !result.LocalSync.Skipped || result.LocalSync.Reason != "ready_transition_only" {
		t.Fatalf("expected ready-only finish boundary, got %+v", result.LocalSync)
	}
}

func TestFinishWorkDraftDryRunPreviewsOnlyReadyTransition(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[{"number":220,"body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":true,"mergeStateStatus":"CLEAN","headRefName":"issue-219-finish","baseRefName":"main","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"gh api repos/StatPan/gira/issues/219": {[]byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-review"}]}`)},
	}, errs: map[string]error{}}

	result, err := FinishWork(repo, 219, true, 0, runner)
	if err != nil {
		t.Fatalf("FinishWork error: %v", err)
	}
	if !finishActionStatus(result.Actions, "pr:ready", "planned") || finishActionStatus(result.Actions, "pr:merge", "planned") {
		t.Fatalf("Draft dry-run action set expanded: %+v", result.Actions)
	}
	if result.Approval == nil || !approvalHasAction(result.Approval.PlannedActions, "pr:ready") || approvalHasAction(result.Approval.PlannedActions, "pr:merge") {
		t.Fatalf("Draft approval action set is not bounded: %+v", result.Approval)
	}
	if len(result.Approval.Warnings) == 0 || !strings.Contains(strings.Join(result.Approval.Warnings, " "), "new dry-run") {
		t.Fatalf("Draft approval missing terminal-intent warning: %+v", result.Approval.Warnings)
	}
}

func TestFinishWorkDraftDryRunRetainsOtherBlockers(t *testing.T) {
	tests := []struct {
		name           string
		reviewDecision string
		checkStatus    string
		conclusion     string
		want           string
	}{
		{name: "pending checks", reviewDecision: "APPROVED", checkStatus: "IN_PROGRESS", want: "checks_pending"},
		{name: "failed checks", reviewDecision: "APPROVED", checkStatus: "COMPLETED", conclusion: "FAILURE", want: "checks"},
		{name: "review required", reviewDecision: "REVIEW_REQUIRED", checkStatus: "COMPLETED", conclusion: "SUCCESS", want: "review"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := RepoRef{Owner: "StatPan", Name: "gira"}
			pr := fmt.Sprintf(`[{"number":220,"body":"Closes #219","state":"OPEN","url":"u","reviewDecision":%q,"isDraft":true,"mergeStateStatus":"BLOCKED","headRefName":"issue-219-finish","baseRefName":"main","statusCheckRollup":[{"conclusion":%q,"status":%q}]}]`, tt.reviewDecision, tt.conclusion, tt.checkStatus)
			runner := &finishRunner{outputs: map[string][][]byte{
				"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {[]byte(pr)},
				"gh api repos/StatPan/gira/issues/219": {[]byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-review"}]}`)},
			}, errs: map[string]error{}}

			result, err := FinishWork(repo, 219, true, 0, runner)
			if err != nil {
				t.Fatalf("FinishWork error: %v", err)
			}
			if !containsString(result.Blockers, "draft") || !containsString(result.Blockers, tt.want) {
				t.Fatalf("blockers = %+v, want draft and %s", result.Blockers, tt.want)
			}
			if finishActionStatus(result.Actions, "pr:merge", "planned") {
				t.Fatalf("blocked Draft unexpectedly planned merge: %+v", result.Actions)
			}
		})
	}
}

func TestFinishWorkApplyGraphQLRateLimitFallsBackToRESTMerge(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"https://github.com/StatPan/gira/pull/220","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-219-finish","baseRefName":"main","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"MERGED","url":"https://github.com/StatPan/gira/pull/220","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"UNKNOWN","headRefName":"issue-219-finish","baseRefName":"main","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"gh api rate_limit": {
			[]byte(`{"resources":{"core":{"limit":5000,"remaining":4916,"used":84,"reset":1783069200},"graphql":{"limit":5000,"remaining":4942,"used":58,"reset":1783069200},"search":{"limit":30,"remaining":30,"used":0,"reset":1783069200}}}`),
		},
		"gh api repos/StatPan/gira/pulls/220": {
			[]byte(`{"state":"open","mergeable":true,"mergeable_state":"clean","head":{"sha":"abc123"}}`),
		},
		"gh api -X PUT repos/StatPan/gira/pulls/220/merge -f merge_method=squash -f sha=abc123": {
			[]byte(`{"sha":"merge123","merged":true,"message":"Pull Request successfully merged"}`),
		},
		"gh api repos/StatPan/gira/issues/219": {
			[]byte(`{"number":219,"title":"Finish","state":"closed","labels":[]}`),
			[]byte(`{"number":219,"title":"Finish","state":"closed","labels":[]}`),
		},
	}, errs: map[string]error{
		"gh pr merge 220 --repo StatPan/gira --squash --delete-branch": fmt.Errorf("GraphQL: API rate limit exceeded for user ID 159117309"),
	}}

	result, err := FinishWork(repo, 219, false, 0, runner)
	if err != nil {
		t.Fatalf("FinishWork error: %v", err)
	}
	if !result.Merged || result.PRNumber != 220 || len(result.Blockers) != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	for _, want := range []string{
		"gh pr merge 220 --repo StatPan/gira --squash --delete-branch",
		"gh api rate_limit",
		"gh api repos/StatPan/gira/pulls/220",
		"gh api -X PUT repos/StatPan/gira/pulls/220/merge -f merge_method=squash -f sha=abc123",
	} {
		if !containsCall(runner.calls, want) {
			t.Fatalf("missing call %q in %v", want, runner.calls)
		}
	}
	detail := finishActionDetail(result.Actions, "pr:merge_fallback")
	for _, want := range []string{"expected_head_sha=abc123", "core_remaining=4916/5000", "graphql_remaining=4942/5000"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("fallback detail missing %q: %q", want, detail)
		}
	}
	if !finishActionStatus(result.Actions, "pr:merge_fallback", "applied") {
		t.Fatalf("missing applied fallback action: %+v", result.Actions)
	}
	if result.Receipt.PullRequest.Number != 220 || result.Receipt.PullRequest.HeadSHA != "head220" || result.Receipt.PullRequest.MergeCommitSHA != "merge220" || !result.Receipt.PullRequest.ClosingReference {
		t.Fatalf("finish receipt did not preserve verified PR evidence: %+v", result.Receipt.PullRequest)
	}
}

func TestFinishWorkAlreadyMergedSelectsReplacementAndVerifiesReceipt(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	body := RenderTicketLifecycleBlock(TicketLifecycleState{WorkBranch: "issue-597-replacement"})
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh issue view 597 --repo StatPan/gira --json number,title,body": {
			[]byte(`{"number":597,"title":"Replacement delivery","body":` + strconv.Quote(body) + `}`),
		},
		"gh api repos/StatPan/gira/issues/597/timeline --paginate": {
			[]byte(`[
				{"source":{"issue":{"number":599,"pull_request":{"url":"https://api.github.com/repos/StatPan/gira/pulls/599"}}}},
				{"source":{"issue":{"number":634,"pull_request":{"url":"https://api.github.com/repos/StatPan/gira/pulls/634"}}}}
			]`),
		},
		"gh api repos/StatPan/gira/pulls/599": {
			[]byte(`{"number":599,"body":"Closes #597","state":"closed","html_url":"old","head":{"ref":"issue-597-old","sha":"head599"},"base":{"ref":"main"}}`),
		},
		"gh api repos/StatPan/gira/pulls/634": {
			[]byte(`{"number":634,"body":"Closes #597","state":"closed","merged_at":"2026-07-18T10:00:00Z","merge_commit_sha":"merge634","html_url":"replacement","head":{"ref":"issue-597-replacement","sha":"head634"},"base":{"ref":"main"}}`),
			[]byte(`{"number":634,"body":"Closes #597","state":"closed","merged_at":"2026-07-18T10:00:00Z","merge_commit_sha":"merge634","html_url":"replacement","head":{"ref":"issue-597-replacement","sha":"head634"},"base":{"ref":"main"}}`),
		},
		"gh api repos/StatPan/gira/pulls/634/reviews --paginate": {
			[]byte(`[{"state":"APPROVED","submitted_at":"2026-07-18T09:00:00Z"}]`),
		},
		"gh api repos/StatPan/gira/commits/head634/check-runs -X GET -f per_page=100 -f filter=all --paginate --slurp": {
			[]byte(`{"check_runs":[{"status":"completed","conclusion":"success"}]}`),
		},
		"gh api repos/StatPan/gira/commits/head634/status": {[]byte(`{"statuses":[]}`)},
		"gh api repos/StatPan/gira/issues/597": {
			[]byte(`{"number":597,"title":"Replacement delivery","state":"closed","labels":[]}`),
			[]byte(`{"number":597,"title":"Replacement delivery","state":"closed","labels":[]}`),
		},
	}, errs: map[string]error{}}

	result, err := FinishWork(repo, 597, false, 0, runner)
	if err != nil {
		t.Fatalf("FinishWork error: %v; calls=%v", err, runner.calls)
	}
	pr := result.Receipt.PullRequest
	if !result.AlreadyDone || !result.Merged || result.PRNumber != 634 || pr.Number != 634 || pr.State != "MERGED" || pr.HeadSHA != "head634" || pr.MergeCommitSHA != "merge634" || !pr.ClosingReference {
		t.Fatalf("finish receipt regressed to superseded PR: result=%+v receipt=%+v", result, pr)
	}
	if strings.Contains(result.Receipt.RenderedBody, "#599 state=") {
		t.Fatalf("receipt represented closed-unmerged PR #599:\n%s", result.Receipt.RenderedBody)
	}
}

func TestFinishWorkApplyConvergesOpenIssueAfterMergedNonDefaultDelivery(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"MERGED","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"UNKNOWN","headRefName":"issue-219-finish","baseRefName":"dev","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"gh api repos/StatPan/gira/issues/219": {
			[]byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-review"}]}`),
			[]byte(`{"number":219,"title":"Finish","state":"closed","labels":[{"name":"status:in-review"}]}`),
			[]byte(`{"number":219,"title":"Finish","state":"closed","labels":[{"name":"status:done"}]}`),
		},
		"gh issue close 219 --repo StatPan/gira --reason completed":                                     {nil},
		"gh label list --repo StatPan/gira --json name --limit 1000":                                    {[]byte(`[{"name":"status:done"}]`)},
		"gh issue edit 219 --repo StatPan/gira --add-label status:done --remove-label status:in-review": {nil},
	}, errs: map[string]error{}}

	result, err := FinishWork(repo, 219, false, 0, runner)
	if err != nil {
		t.Fatalf("FinishWork error: %v; calls=%v", err, runner.calls)
	}
	for _, want := range []string{
		"gh issue close 219 --repo StatPan/gira --reason completed",
		"gh issue edit 219 --repo StatPan/gira --add-label status:done --remove-label status:in-review",
	} {
		if !containsCall(runner.calls, want) {
			t.Fatalf("missing convergence call %q in %v", want, runner.calls)
		}
	}
	if !finishActionStatus(result.Actions, "ticket:close", "applied") || !finishActionStatus(result.Actions, "ticket:normalize-status", "applied") {
		t.Fatalf("missing completion convergence actions: %+v", result.Actions)
	}
	if result.Receipt.FinalState.GitHubIssueState != "closed" || result.Receipt.FinalState.GiraStatus != "Done" {
		t.Fatalf("receipt did not report converged GitHub and Gira state: %+v", result.Receipt.FinalState)
	}
	if !strings.Contains(result.Receipt.RenderedBody, "gira_status=Done github_issue_state=closed") {
		t.Fatalf("receipt did not render explicit completion state:\n%s", result.Receipt.RenderedBody)
	}
}

func TestFinishWorkDryRunOffersCompletionConvergenceForAlreadyMergedOpenIssue(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"MERGED","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"UNKNOWN","headRefName":"issue-219-finish","baseRefName":"dev","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"gh api repos/StatPan/gira/issues/219": {
			[]byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-review"}]}`),
			[]byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-review"}]}`),
		},
	}, errs: map[string]error{}}

	result, err := FinishWork(repo, 219, true, 0, runner)
	if err != nil {
		t.Fatalf("FinishWork dry-run error: %v; calls=%v", err, runner.calls)
	}
	if !result.AlreadyDone || !finishActionStatus(result.Actions, "ticket:close", "planned") {
		t.Fatalf("missing planned closure repair: %+v", result)
	}
	if result.NextStep != "gira ticket finish --repo StatPan/gira --ticket 219 --apply" || result.FinalStatus.NextAction != "converge_completion_state" {
		t.Fatalf("dry-run did not offer completion convergence: %+v", result)
	}
	if containsCall(runner.calls, "gh issue close 219 --repo StatPan/gira --reason completed") {
		t.Fatalf("dry-run must not close the issue: %v", runner.calls)
	}
	if result.Receipt.FinalState.GitHubIssueState != "open" || result.Receipt.FinalState.GiraStatus != "In review" {
		t.Fatalf("receipt must expose the unresolved state mismatch: %+v", result.Receipt.FinalState)
	}
}

func TestFinishWorkApplySyncLocalOptInTargetsPRBase(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-219-finish","baseRefName":"release/2.0","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"MERGED","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"UNKNOWN","headRefName":"issue-219-finish","baseRefName":"release/2.0","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"gh pr merge 220 --repo StatPan/gira --squash --delete-branch": {nil},
		"gh api repos/StatPan/gira/pulls/220":                          {[]byte(`{"number":220,"body":"Closes #219","state":"closed","merged_at":"2026-07-22T00:00:00Z","merge_commit_sha":"merge220","html_url":"u","head":{"ref":"issue-219-finish","sha":"head220"},"base":{"ref":"release/2.0"}}`)},
		"git remote get-url origin":                                    {[]byte("git@github.com:StatPan/gira.git\n")},
		"git branch --show-current":                                    {[]byte("issue-219-finish\n")},
		"git status --porcelain":                                       {[]byte("")},
		"git worktree list --porcelain":                                {[]byte("worktree /workspace/gira\nHEAD abc\nbranch refs/heads/issue-219-finish\n\n")},
		"git checkout release/2.0":                                     {nil},
		"git pull --ff-only origin release/2.0":                        {nil},
		"gh api repos/StatPan/gira/issues/219": {
			[]byte(`{"number":219,"title":"Finish","state":"closed","labels":[]}`),
			[]byte(`{"number":219,"title":"Finish","state":"closed","labels":[]}`),
		},
	}, errs: map[string]error{}}

	result, err := FinishWorkWithOptions(repo, 219, false, 0, WorkFinishOptions{SyncLocal: true}, runner)
	if err != nil {
		t.Fatalf("FinishWorkWithOptions error: %v", err)
	}
	for _, want := range []string{
		"git checkout release/2.0",
		"git pull --ff-only origin release/2.0",
	} {
		if !containsCall(runner.calls, want) {
			t.Fatalf("missing opt-in local sync call %q in %v", want, runner.calls)
		}
	}
	for _, unexpected := range []string{
		"git checkout main",
		"git pull --ff-only origin main",
	} {
		if containsCall(runner.calls, unexpected) {
			t.Fatalf("local sync should target PR base, not hard-coded main; calls=%v", runner.calls)
		}
	}
	if !result.LocalSync.Attempted || result.LocalSync.Skipped || result.LocalSync.TargetBranch != "release/2.0" {
		t.Fatalf("expected applied local sync against PR base, got %+v", result.LocalSync)
	}
	if !finishActionStatus(result.Actions, "local:sync_base", "applied") {
		t.Fatalf("missing applied local sync action: %+v", result.Actions)
	}
}

func TestFinishWorkApplySyncLocalSkipsBranchCheckedOutInAnotherWorktree(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-219-finish","baseRefName":"release/2.0","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"MERGED","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"UNKNOWN","headRefName":"issue-219-finish","baseRefName":"release/2.0","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"gh pr merge 220 --repo StatPan/gira --squash --delete-branch": {nil},
		"gh api repos/StatPan/gira/pulls/220":                          {[]byte(`{"number":220,"body":"Closes #219","state":"closed","merged_at":"2026-07-22T00:00:00Z","merge_commit_sha":"merge220","html_url":"u","head":{"ref":"issue-219-finish","sha":"head220"},"base":{"ref":"release/2.0"}}`)},
		"git remote get-url origin":                                    {[]byte("git@github.com:StatPan/gira.git\n")},
		"git branch --show-current":                                    {[]byte("issue-219-finish\n")},
		"git status --porcelain":                                       {[]byte("")},
		"git worktree list --porcelain":                                {[]byte("worktree /workspace/gira\nHEAD abc\nbranch refs/heads/issue-219-finish\n\nworktree /workspace/gira-dev\nHEAD def\nbranch refs/heads/release/2.0\n\n")},
		"gh api repos/StatPan/gira/issues/219": {
			[]byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-review"}]}`),
			[]byte(`{"number":219,"title":"Finish","state":"closed","labels":[{"name":"status:in-review"}]}`),
			[]byte(`{"number":219,"title":"Finish","state":"closed","labels":[{"name":"status:done"}]}`),
		},
		"gh issue close 219 --repo StatPan/gira --reason completed":                                     {nil},
		"gh label list --repo StatPan/gira --json name --limit 1000":                                    {[]byte(`[{"name":"status:done"}]`)},
		"gh issue edit 219 --repo StatPan/gira --add-label status:done --remove-label status:in-review": {nil},
	}, errs: map[string]error{}}

	result, err := FinishWorkWithOptions(repo, 219, false, 0, WorkFinishOptions{SyncLocal: true}, runner)
	if err != nil {
		t.Fatalf("FinishWorkWithOptions error: %v", err)
	}
	if !result.Merged || !result.LocalSync.Skipped || result.LocalSync.Reason != "branch_checked_out_elsewhere" {
		t.Fatalf("expected merge with a safe local-sync skip, got %+v", result)
	}
	if containsCall(runner.calls, "git checkout release/2.0") || containsCall(runner.calls, "git pull --ff-only origin release/2.0") {
		t.Fatalf("must not touch a branch owned by another worktree: %v", runner.calls)
	}
	if !finishActionStatus(result.Actions, "local:sync_base", "skipped") {
		t.Fatalf("expected skipped local sync action: %+v", result.Actions)
	}
	if !finishActionStatus(result.Actions, "ticket:close", "applied") || !finishActionStatus(result.Actions, "ticket:normalize-status", "applied") || result.Receipt.FinalState.GitHubIssueState != "closed" || result.Receipt.FinalState.GiraStatus != "Done" {
		t.Fatalf("merge, completion convergence, and safe local skip must be independently visible: %+v", result)
	}
}

func TestFinishWorkApplySyncLocalCheckoutFailureDoesNotUndoConvergence(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"MERGED","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"UNKNOWN","headRefName":"issue-219-finish","baseRefName":"release/2.0","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"git remote get-url origin":     {[]byte("git@github.com:StatPan/gira.git\n")},
		"git branch --show-current":     {[]byte("issue-219-finish\n")},
		"git status --porcelain":        {[]byte("")},
		"git worktree list --porcelain": {[]byte("worktree /workspace/gira\nHEAD abc\nbranch refs/heads/issue-219-finish\n\n")},
		"gh api repos/StatPan/gira/issues/219": {
			[]byte(`{"number":219,"title":"Finish","state":"open","labels":[]}`),
			[]byte(`{"number":219,"title":"Finish","state":"closed","labels":[]}`),
			[]byte(`{"number":219,"title":"Finish","state":"closed","labels":[]}`),
		},
		"gh issue close 219 --repo StatPan/gira --reason completed": {nil},
	}, errs: map[string]error{
		"git checkout release/2.0": fmt.Errorf("release/2.0 is already checked out at /workspace/gira-dev"),
	}}

	result, err := FinishWorkWithOptions(repo, 219, false, 0, WorkFinishOptions{SyncLocal: true}, runner)
	if err != nil {
		t.Fatalf("local checkout failure must not discard a completed delivery: %v; calls=%v", err, runner.calls)
	}
	if !result.AlreadyDone || !finishActionStatus(result.Actions, "ticket:close", "applied") || !finishActionStatus(result.Actions, "local:sync_base", "failed") || result.LocalSync.Reason != "checkout_failed" {
		t.Fatalf("expected independent convergence and local failure results, got %+v", result)
	}
	if result.Receipt.FinalState.GitHubIssueState != "closed" || !containsFinishCallPrefix(runner.calls, "gh issue comment 219 --repo StatPan/gira --body ## Finish Receipt") {
		t.Fatalf("expected a converged receipt despite local sync failure: %+v calls=%v", result.Receipt, runner.calls)
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

func TestFinishWorkApplyRetriesTransientMissingLinkedPR(t *testing.T) {
	restoreDelay := finishMissingPRRetryDelay
	finishMissingPRRetryDelay = 0
	t.Cleanup(func() {
		finishMissingPRRetryDelay = restoreDelay
	})
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"https://github.com/StatPan/gira/pull/220","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-219-finish","baseRefName":"main","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"MERGED","url":"https://github.com/StatPan/gira/pull/220","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"UNKNOWN","headRefName":"issue-219-finish","baseRefName":"main","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
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
	if !result.Merged || result.PRNumber != 220 || result.PRLookupAttempts != 2 || len(result.Blockers) != 0 {
		t.Fatalf("unexpected retry result: %+v", result)
	}
	if !containsFinishCallPrefix(runner.calls, "gh issue comment 219 --repo StatPan/gira --body ## Finish Receipt") || !finishActionStatus(result.Actions, "finish:receipt", "applied") {
		t.Fatalf("expected applied finish receipt comment, calls=%v actions=%+v", runner.calls, result.Actions)
	}
	if got := countFinishCall(runner.calls, "gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20"); got != 2 {
		t.Fatalf("expected transient retry followed by exact merged-PR verification, got %d discovery calls: %v", got, runner.calls)
	}
}

func TestFinishWorkReceiptWarnsWhenAgentTelemetryMissing(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-219-finish","baseRefName":"main","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-219-finish","baseRefName":"main","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"git remote get-url origin": {[]byte("git@github.com:StatPan/gira.git\n")},
		"git branch --show-current": {[]byte("issue-219-finish\n")},
		"git status --porcelain":    {[]byte("")},
		"gh api repos/StatPan/gira/issues/219": {
			[]byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-review"},{"name":"agent:worker"}]}`),
			[]byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-review"},{"name":"agent:worker"}]}`),
		},
	}, errs: map[string]error{}}

	result, err := FinishWork(repo, 219, true, 0, runner)
	if err != nil {
		t.Fatalf("FinishWork error: %v", err)
	}
	if result.Receipt.TelemetrySummary == nil || result.Receipt.TelemetrySummary.Status != "missing" {
		t.Fatalf("expected missing telemetry summary, got %+v", result.Receipt.TelemetrySummary)
	}
	for _, want := range []string{"AI Delivery Telemetry: missing", "missing_ai_delivery_telemetry"} {
		if !strings.Contains(result.Receipt.RenderedBody, want) {
			t.Fatalf("receipt missing %q:\n%s", want, result.Receipt.RenderedBody)
		}
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
	if !strings.Contains(finishActionDetail(result.Actions, "ticket:normalize-status"), "add=status:done") {
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
	if result.Readiness.Ready || !containsString(result.Readiness.Blockers, "checks_pending") || result.Readiness.Checks.Status != "pending" {
		t.Fatalf("expected blocked pending-check readiness report, got %+v", result.Readiness)
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
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-219-finish","baseRefName":"release/2.0","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-219-finish","baseRefName":"release/2.0","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
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
	if !result.Readiness.Ready || len(result.Readiness.Blockers) != 0 {
		t.Fatalf("expected ready finish readiness report, got %+v", result.Readiness)
	}
	if result.Readiness.SchemaVersion != "finish-readiness/v1" || !result.Readiness.ClosingReference.Present || result.Readiness.Checks.Status != "passed" || result.Readiness.Review.Status != "approved" {
		t.Fatalf("readiness evidence missing expected contract fields: %+v", result.Readiness)
	}
	if !result.LocalSync.Skipped || result.LocalSync.Reason != "local_sync_disabled" {
		t.Fatalf("expected dry-run local sync to be disabled by default, got %+v", result.LocalSync)
	}
	if finishActionStatus(result.Actions, "local:sync_base", "planned") || finishActionStatus(result.Actions, "local:sync_main", "planned") {
		t.Fatalf("dry-run should not plan local sync by default: %+v", result.Actions)
	}
	if containsCall(runner.calls, "git remote get-url origin") {
		t.Fatalf("default finish should not inspect local checkout for sync: %v", runner.calls)
	}
	if result.Receipt.SchemaVersion != "finish-receipt/v1" || !strings.Contains(result.Receipt.RenderedBody, "## Finish Receipt") || !finishActionStatus(result.Actions, "finish:receipt", "planned") {
		t.Fatalf("expected dry-run finish receipt preview and planned action: receipt=%+v actions=%+v", result.Receipt, result.Actions)
	}
	if result.SchemaVersion != WorkFinishResultSchemaVersion || result.Approval == nil {
		t.Fatalf("expected finish schema and approval evidence: %+v", result)
	}
	if result.Approval.SchemaVersion != ApprovalPlanSchemaVersion || result.Approval.CanonicalCommand != "gira ticket finish" || result.Approval.OutputSchema != WorkFinishResultSchemaVersion {
		t.Fatalf("unexpected finish approval evidence: %+v", result.Approval)
	}
	if result.Approval.ApplyCommand != "gira ticket finish 219 --repo StatPan/gira --apply" || result.Approval.PostApplyVerification != "gira ticket status 219 --repo StatPan/gira --json" {
		t.Fatalf("unexpected finish approval commands: %+v", result.Approval)
	}
	if !approvalHasAction(result.Approval.PlannedActions, "pr:merge") || !approvalHasAction(result.Approval.PlannedActions, "finish:receipt") {
		t.Fatalf("finish approval missing planned actions: %+v", result.Approval.PlannedActions)
	}
	if result.Approval.Blockers == nil || result.Approval.Warnings == nil {
		t.Fatalf("finish approval blockers and warnings must be stable arrays: %+v", result.Approval)
	}
	if len(result.Warnings) == 0 || !strings.Contains(result.Warnings[0], "IRREVERSIBLE") || !containsString(result.Approval.Warnings, result.Warnings[0]) {
		t.Fatalf("missing irreversible merge warning: result=%+v approval=%+v", result.Warnings, result.Approval.Warnings)
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

func TestFinishWorkAllowsCustomPRHeadBindingWithAdvisory(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueJSON := []byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-review"}]}`)
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"feature/unrelated","baseRefName":"main","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"feature/unrelated","baseRefName":"main","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"gh api repos/StatPan/gira/issues/219": {issueJSON, issueJSON},
	}, errs: map[string]error{}}

	result, err := FinishWork(repo, 219, true, 0, runner)
	if err != nil {
		t.Fatalf("expected custom branch to remain finish-ready, got result=%+v err=%v", result, err)
	}
	if containsString(result.Blockers, "pr_binding") || !result.Readiness.Ready {
		t.Fatalf("unexpected binding blocker: %+v", result)
	}
	if result.FinalStatus.PRNumber != 220 || !containsString(result.FinalStatus.Warnings, "branch_name_differs_from_suggestion") {
		t.Fatalf("expected final status to retain linked PR context: %+v", result.FinalStatus)
	}
	if containsCall(runner.calls, "gh pr merge 220 --repo StatPan/gira --squash --delete-branch") {
		t.Fatalf("dry-run should not merge: %v", runner.calls)
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
	if result.Readiness.Ready || !containsString(result.Readiness.Blockers, "missing_linked_pr") || result.Readiness.PullRequest.Available {
		t.Fatalf("expected missing PR readiness blocker, got %+v", result.Readiness)
	}
	if result.Readiness.ClosingReference.Present || result.Readiness.Evidence.FinishReady {
		t.Fatalf("missing PR should not report completion evidence: %+v", result.Readiness)
	}
}

func TestFinishWorkApplyStopsRetryingMissingLinkedPR(t *testing.T) {
	restoreDelay := finishMissingPRRetryDelay
	finishMissingPRRetryDelay = 0
	t.Cleanup(func() {
		finishMissingPRRetryDelay = restoreDelay
	})
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[]`),
			[]byte(`[]`),
			[]byte(`[]`),
		},
		"gh api repos/StatPan/gira/issues/219": {[]byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-progress"}]}`)},
	}, errs: map[string]error{}}

	result, err := FinishWork(repo, 219, false, 0, runner)
	if err != nil {
		t.Fatalf("FinishWork error: %v", err)
	}
	if result.PRLookupAttempts != 3 || !containsString(result.Blockers, "missing_linked_pr") || !strings.Contains(result.NextStep, "ticket pr") {
		t.Fatalf("expected bounded missing PR retry result: %+v", result)
	}
	if got := countFinishCall(runner.calls, "gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20"); got != 3 {
		t.Fatalf("expected bounded retry count, got %d calls: %v", got, runner.calls)
	}
}

func TestFinishWorkDryRunSkipsLocalSyncOnDirtyWorktree(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &finishRunner{outputs: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 219 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": {
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-219-finish","baseRefName":"release/2.0","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
			[]byte(`[{"number":220,"title":"x","body":"Closes #219","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-219-finish","baseRefName":"release/2.0","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		},
		"git remote get-url origin":            {[]byte("https://github.com/StatPan/gira.git\n")},
		"git branch --show-current":            {[]byte("issue-219-finish\n")},
		"git status --porcelain":               {[]byte(" M README.md\n")},
		"gh api repos/StatPan/gira/issues/219": {[]byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-review"}]}`)},
	}, errs: map[string]error{}}

	result, err := FinishWorkWithOptions(repo, 219, true, 0, WorkFinishOptions{SyncLocal: true}, runner)
	if err != nil {
		t.Fatalf("FinishWork error: %v", err)
	}
	if !result.LocalSync.Skipped || result.LocalSync.Reason != "dirty_worktree" {
		t.Fatalf("expected dirty local sync skip, got %+v", result.LocalSync)
	}
	if result.LocalSync.TargetBranch != "release/2.0" {
		t.Fatalf("expected local sync target from PR base, got %+v", result.LocalSync)
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

func finishActionDetail(actions []WorkFinishAction, action string) string {
	for _, item := range actions {
		if item.Action == action {
			return item.Detail
		}
	}
	return ""
}

func containsFinishCallPrefix(calls []string, prefix string) bool {
	for _, call := range calls {
		if strings.HasPrefix(call, prefix) {
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
