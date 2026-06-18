package gira

import (
	"fmt"
	"strings"
	"testing"
)

type devPRRunner struct {
	outputs map[string][]byte
	errs    map[string]error
	calls   *[]string
}

func (r devPRRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	if r.calls != nil {
		*r.calls = append(*r.calls, key)
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
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 60 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[
			{"number":99,"title":"x","body":"Closes #60","state":"OPEN","url":"u","reviewDecision":"REVIEW_REQUIRED","isDraft":false,"mergeStateStatus":"BLOCKED","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}
		]`),
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
