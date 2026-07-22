package gira

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

type devPRRunner struct {
	outputs map[string][]byte
	queues  map[string][]devPRRunResult
	errs    map[string]error
	calls   *[]string
}

type devPRRunResult struct {
	out []byte
	err error
}

func (r devPRRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	if r.calls != nil {
		*r.calls = append(*r.calls, key)
	}
	if queue := r.queues[key]; len(queue) > 0 {
		next := queue[0]
		r.queues[key] = queue[1:]
		if next.err != nil {
			return nil, next.err
		}
		return next.out, nil
	}
	if err, ok := r.errs[key]; ok {
		return nil, err
	}
	if out, ok := r.outputs[key]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected call: %s", key)
}

func TestOpenDevPR(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := devPRRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/60":                                          []byte(`{"number":60,"title":"Add PR loop","state":"open","labels":[{"name":"status:ready"}]}`),
		"gh pr create --repo StatPan/gira --title feat: Add PR loop --body Closes #60": []byte("https://github.com/StatPan/gira/pull/99\n"),
	}}
	result, err := OpenDevPR(repo, 60, runner)
	if err != nil {
		t.Fatalf("OpenDevPR err: %v", err)
	}
	if result.PRNumber != 99 {
		t.Fatalf("pr number = %d", result.PRNumber)
	}
}

func TestDevPRStatusBlocked(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := devPRRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/60/timeline --paginate": []byte(`[
			{"event":"cross-referenced","source":{"issue":{"number":99,"body":"Closes #60","pull_request":{"url":"https://api.github.com/repos/StatPan/gira/pulls/99"}}}}
		]`),
		"gh api repos/StatPan/gira/pulls/99": []byte(`{
			"number":99,
			"title":"x",
			"body":"Closes #60",
			"state":"open",
			"html_url":"u",
			"draft":false,
			"mergeable_state":"blocked",
			"head":{"ref":"issue-60-blocked","sha":"abc123"},
			"base":{"ref":"main"}
		}`),
		"gh api repos/StatPan/gira/pulls/99/reviews --paginate":                            []byte(`[]`),
		"gh api repos/StatPan/gira/branches/main/protection/required_pull_request_reviews": []byte(`{"required_approving_review_count":1}`),
		"gh api repos/StatPan/gira/commits/abc123/check-runs -X GET -f per_page=100":       []byte(`{"check_runs":[{"conclusion":"success","status":"completed"}]}`),
		"gh api repos/StatPan/gira/commits/abc123/status":                                  []byte(`{"statuses":[]}`),
	}}
	result, err := DevPRStatus(repo, 60, runner)
	if err != nil {
		t.Fatalf("DevPRStatus err: %v", err)
	}
	if result.Ready {
		t.Fatalf("expected not ready")
	}
	if len(result.Blockers) == 0 || result.Blockers[0] != "review" {
		t.Fatalf("unexpected blockers: %+v", result.Blockers)
	}
}

func TestDevPRStatusUsesRESTFirstLinkedPRSnapshot(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	calls := []string{}
	runner := devPRRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/60/timeline --paginate": []byte(`[
			{"event":"cross-referenced","source":{"issue":{"number":99,"title":"x","body":"Closes #60","state":"open","html_url":"https://github.com/StatPan/gira/pull/99","pull_request":{"url":"https://api.github.com/repos/StatPan/gira/pulls/99"}}}}
		]`),
		"gh api repos/StatPan/gira/pulls/99": []byte(`{
			"number":99,
			"title":"x",
			"body":"Closes #60",
			"state":"open",
			"html_url":"https://github.com/StatPan/gira/pull/99",
			"draft":false,
			"mergeable_state":"clean",
			"head":{"ref":"issue-60-rest-first","sha":"abc123"},
			"base":{"ref":"main"}
		}`),
		"gh api repos/StatPan/gira/pulls/99/reviews --paginate":                      []byte(`[{"state":"APPROVED","submitted_at":"2026-06-18T09:00:00Z"}]`),
		"gh api repos/StatPan/gira/commits/abc123/check-runs -X GET -f per_page=100": []byte(`{"check_runs":[{"name":"test","status":"completed","conclusion":"success","html_url":"https://ci.example","app":{"name":"GitHub Actions"}}]}`),
		"gh api repos/StatPan/gira/commits/abc123/status":                            []byte(`{"statuses":[]}`),
	}, calls: &calls}
	result, err := DevPRStatus(repo, 60, runner)
	if err != nil {
		t.Fatalf("DevPRStatus err: %v", err)
	}
	if !result.Ready || result.PRNumber != 99 || result.ReviewDecision != "APPROVED" || result.Mergeable != "CLEAN" {
		t.Fatalf("unexpected REST-first status: %+v", result)
	}
	if len(result.Checks) != 1 || result.Checks[0].State != "passing" {
		t.Fatalf("unexpected checks: %+v", result.Checks)
	}
	for _, call := range calls {
		if strings.HasPrefix(call, "gh pr list ") {
			t.Fatalf("REST-first path should not call GraphQL-heavy gh pr list: %v", calls)
		}
	}
}

func TestDevPRStatusTrustsExactRecordedWorkBranch(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	body := RenderTicketLifecycleBlock(TicketLifecycleState{WorkBranch: "feat/i60-a2a-unary-adapter"})
	runner := devPRRunner{outputs: map[string][]byte{
		"gh issue view 60 --repo StatPan/gira --json number,title,body":              []byte(`{"number":60,"title":"A2A unary adapter","body":` + strconv.Quote(body) + `}`),
		"gh api repos/StatPan/gira/issues/60/timeline --paginate":                    []byte(`[{"source":{"issue":{"number":99,"pull_request":{"url":"https://api.github.com/repos/StatPan/gira/pulls/99"}}}}]`),
		"gh api repos/StatPan/gira/pulls/99":                                         []byte(`{"number":99,"body":"Closes #60","state":"open","html_url":"u","mergeable_state":"clean","head":{"ref":"feat/i60-a2a-unary-adapter","sha":"abc123"},"base":{"ref":"dev"}}`),
		"gh api repos/StatPan/gira/pulls/99/reviews --paginate":                      []byte(`[]`),
		"gh api repos/StatPan/gira/commits/abc123/check-runs -X GET -f per_page=100": []byte(`{"check_runs":[]}`),
		"gh api repos/StatPan/gira/commits/abc123/status":                            []byte(`{"statuses":[]}`),
	}}

	result, err := DevPRStatus(repo, 60, runner)
	if err != nil {
		t.Fatalf("DevPRStatus error: %v", err)
	}
	if !result.Binding.Trusted || result.Binding.Source != "recorded_work_branch" || containsString(result.Blockers, "pr_binding") {
		t.Fatalf("recorded branch was not trusted: %+v", result)
	}
}

func TestValidateDevPRBindingKeepsStrategiesDistinct(t *testing.T) {
	recorded := validateDevPRBinding(60, prSummary{State: "OPEN", HeadRefName: "feat/i60-adapter"}, devPRBindingPolicy{RecordedWorkBranch: "feat/i60-adapter", ResolvedWorkBranch: "work/60-adapter"})
	resolved := validateDevPRBinding(60, prSummary{State: "OPEN", HeadRefName: "work/60-adapter"}, devPRBindingPolicy{RecordedWorkBranch: "feat/i60-adapter", ResolvedWorkBranch: "work/60-adapter"})
	legacy := validateDevPRBinding(60, prSummary{State: "OPEN", HeadRefName: "issue-60-adapter"}, devPRBindingPolicy{})
	mismatch := validateDevPRBinding(60, prSummary{State: "OPEN", HeadRefName: "feat/i61-unrelated"}, devPRBindingPolicy{RecordedWorkBranch: "feat/i60-adapter"})
	if recorded.Source != "recorded_work_branch" || resolved.Source != "branch_policy.feature_branch_pattern" || legacy.Source != "legacy_issue_branch" {
		t.Fatalf("unexpected strategy sources: recorded=%+v resolved=%+v legacy=%+v", recorded, resolved, legacy)
	}
	if !recorded.Trusted || !resolved.Trusted || !legacy.Trusted || mismatch.Trusted || !containsString(mismatch.Blockers, "pr_binding") {
		t.Fatalf("unexpected strategy trust: recorded=%+v resolved=%+v legacy=%+v mismatch=%+v", recorded, resolved, legacy, mismatch)
	}
}

func TestDevPRStatusRESTFirstMapsPendingChecks(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := devPRRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/60/timeline --paginate": []byte(`[
			{"event":"cross-referenced","source":{"issue":{"number":99,"body":"Closes #60","pull_request":{"url":"https://api.github.com/repos/StatPan/gira/pulls/99"}}}}
		]`),
		"gh api repos/StatPan/gira/pulls/99": []byte(`{
			"number":99,
			"body":"Closes #60",
			"state":"open",
			"html_url":"https://github.com/StatPan/gira/pull/99",
			"mergeable_state":"clean",
			"head":{"ref":"issue-60-rest-first","sha":"abc123"},
			"base":{"ref":"main"}
		}`),
		"gh api repos/StatPan/gira/pulls/99/reviews --paginate":                      []byte(`[]`),
		"gh api repos/StatPan/gira/commits/abc123/check-runs -X GET -f per_page=100": []byte(`{"check_runs":[{"name":"test","status":"queued","conclusion":""}]}`),
		"gh api repos/StatPan/gira/commits/abc123/status":                            []byte(`{"statuses":[]}`),
	}}
	result, err := DevPRStatus(repo, 60, runner)
	if err != nil {
		t.Fatalf("DevPRStatus err: %v", err)
	}
	if result.Ready || !containsString(result.Blockers, "checks_pending") {
		t.Fatalf("expected pending checks blocker: %+v", result)
	}
}

func TestDevPRStatusRESTFirstMissingPRDoesNotFallBackToGraphQL(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	calls := []string{}
	runner := devPRRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/60/timeline --paginate": []byte(`[]`),
	}, calls: &calls}
	result, err := DevPRStatus(repo, 60, runner)
	if err != nil {
		t.Fatalf("DevPRStatus err: %v", err)
	}
	if result.Ready || !containsString(result.Blockers, "missing_linked_pr") {
		t.Fatalf("expected missing linked PR: %+v", result)
	}
	for _, call := range calls {
		if strings.HasPrefix(call, "gh pr list ") {
			t.Fatalf("empty REST timeline should not call GraphQL-heavy gh pr list: %v", calls)
		}
	}
}

func TestDevPRStatusUsesRESTSearchFallbackWhenTimelineUnavailable(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	calls := []string{}
	runner := devPRRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/pulls -X GET -f state=all -f sort=updated -f direction=desc -f per_page=100": []byte(`[{"number":99},{"number":98}]`),
		"gh api repos/StatPan/gira/pulls/99": []byte(`{
			"number":99,
			"title":"x",
			"body":"Closes #60",
			"state":"open",
			"html_url":"https://github.com/StatPan/gira/pull/99",
			"mergeable_state":"clean",
			"head":{"ref":"issue-60-rest-fallback","sha":"abc123"},
			"base":{"ref":"main"}
		}`),
		"gh api repos/StatPan/gira/pulls/99/reviews --paginate":                      []byte(`[{"state":"APPROVED","submitted_at":"2026-06-18T09:00:00Z"}]`),
		"gh api repos/StatPan/gira/commits/abc123/check-runs -X GET -f per_page=100": []byte(`{"check_runs":[{"name":"test","status":"completed","conclusion":"success"}]}`),
		"gh api repos/StatPan/gira/commits/abc123/status":                            []byte(`{"statuses":[]}`),
	}, errs: map[string]error{
		"gh api repos/StatPan/gira/issues/60/timeline --paginate": fmt.Errorf("timeline unavailable"),
	}, calls: &calls}
	result, err := DevPRStatus(repo, 60, runner)
	if err != nil {
		t.Fatalf("DevPRStatus err: %v", err)
	}
	if !result.Ready || result.PRNumber != 99 || result.Binding.HeadRef != "issue-60-rest-fallback" {
		t.Fatalf("unexpected REST search fallback status: %+v", result)
	}
	for _, call := range calls {
		if strings.HasPrefix(call, "gh pr list ") {
			t.Fatalf("REST search fallback should avoid GraphQL-heavy gh pr list: %v", calls)
		}
	}
}

func TestDevPRStatusDoesNotUseGraphQLFallbackWhenRESTUnavailable(t *testing.T) {
	t.Setenv("GIRA_DEV_PR_GRAPHQL_FALLBACK", "")
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	calls := []string{}
	restSearch := "gh api repos/StatPan/gira/pulls -X GET -f state=all -f sort=updated -f direction=desc -f per_page=100"
	graphQLSearch := "gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 60 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20"
	runner := devPRRunner{outputs: map[string][]byte{
		graphQLSearch: []byte(`[{"number":99,"body":"Closes #60"}]`),
	}, errs: map[string]error{
		"gh api repos/StatPan/gira/issues/60/timeline --paginate": fmt.Errorf("timeline unavailable"),
		restSearch: fmt.Errorf("temporary REST list failure"),
	}, calls: &calls}
	result, err := DevPRStatus(repo, 60, runner)
	if err != nil {
		t.Fatalf("DevPRStatus err: %v", err)
	}
	if result.Ready || result.PRNumber != 0 || !containsString(result.Blockers, "missing_linked_pr") {
		t.Fatalf("expected fail-closed missing PR status: %+v", result)
	}
	if countString(calls, restSearch) != 1 {
		t.Fatalf("REST search calls = %v, want one; all calls=%v", countString(calls, restSearch), calls)
	}
	if countString(calls, graphQLSearch) != 0 {
		t.Fatalf("GraphQL search calls = %v, want zero; all calls=%v", countString(calls, graphQLSearch), calls)
	}
}

func countString(values []string, want string) int {
	count := 0
	for _, value := range values {
		if value == want {
			count++
		}
	}
	return count
}
