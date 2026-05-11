package gira

import (
	"fmt"
	"strings"
	"testing"
)

type devStartRunner struct {
	outputs map[string][]byte
	errs    map[string]error
	calls   []string
}

func (r *devStartRunner) Run(name string, args ...string) ([]byte, error) {
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

func TestStartDevBranchDryRunReadyIssue(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueJSON := `{"number":59,"title":"Add API: start branch","state":"open","labels":[{"name":"status:ready"}]}`
	runner := &devStartRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/59":                                    []byte(issueJSON),
		"git show-ref --verify --quiet refs/heads/issue-59-add-api-start-branch": nil,
	}, errs: map[string]error{
		"git show-ref --verify --quiet refs/heads/issue-59-add-api-start-branch": fmt.Errorf("exit status 1"),
		"git ls-remote --exit-code --heads origin issue-59-add-api-start-branch": fmt.Errorf("exit status 2"),
	}}

	result, err := StartDevBranch(repo, 59, DefaultDevBranchPattern, true, false, runner)
	if err != nil {
		t.Fatalf("StartDevBranch error: %v", err)
	}
	if result.Branch != "issue-59-add-api-start-branch" {
		t.Fatalf("branch = %q", result.Branch)
	}
}

func TestStartDevBranchFailsWhenNotReady(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueJSON := `{"number":59,"title":"Add API: start branch","state":"open","labels":[{"name":"type:task"}]}`
	runner := &devStartRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/59": []byte(issueJSON),
	}}
	_, err := StartDevBranch(repo, 59, DefaultDevBranchPattern, true, false, runner)
	if err == nil || !strings.Contains(err.Error(), "not ready") {
		t.Fatalf("expected not ready error, got %v", err)
	}
}

func TestStartDevBranchRemoteConflict(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueJSON := `{"number":59,"title":"Add API: start branch","state":"open","labels":[{"name":"status:ready"}]}`
	runner := &devStartRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/59":                                    []byte(issueJSON),
		"git ls-remote --exit-code --heads origin issue-59-add-api-start-branch": []byte("abc\trefs/heads/issue-59-add-api-start-branch"),
	}, errs: map[string]error{
		"git show-ref --verify --quiet refs/heads/issue-59-add-api-start-branch": fmt.Errorf("exit status 1"),
	}}
	_, err := StartDevBranch(repo, 59, DefaultDevBranchPattern, true, false, runner)
	if err == nil || !strings.Contains(err.Error(), "branch conflict") {
		t.Fatalf("expected branch conflict error, got %v", err)
	}
}

func TestStartDevBranchReusesExistingLocalBranch(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueJSON := `{"number":59,"title":"Add API: start branch","state":"open","labels":[{"name":"status:ready"}]}`
	runner := &devStartRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/59":                                    []byte(issueJSON),
		"git show-ref --verify --quiet refs/heads/issue-59-add-api-start-branch": nil,
		"git ls-remote --exit-code --heads origin issue-59-add-api-start-branch": []byte("abc\trefs/heads/issue-59-add-api-start-branch"),
		"git checkout issue-59-add-api-start-branch":                             nil,
	}}

	result, err := StartDevBranch(repo, 59, DefaultDevBranchPattern, false, false, runner)
	if err != nil {
		t.Fatalf("StartDevBranch error: %v", err)
	}
	if result.Created {
		t.Fatalf("expected existing local branch to be reused without creating")
	}
	if !containsCall(runner.calls, "git checkout issue-59-add-api-start-branch") {
		t.Fatalf("expected checkout existing branch, calls=%v", runner.calls)
	}
	if containsCall(runner.calls, "git checkout -b issue-59-add-api-start-branch") {
		t.Fatalf("did not expect branch creation call, calls=%v", runner.calls)
	}
}

func TestStartDevBranchFailsOnIssueNumberMismatch(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &devStartRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/59": []byte(`{"number":60,"title":"Add API: start branch","state":"open","labels":[{"name":"status:ready"}]}`),
	}}

	_, err := StartDevBranch(repo, 59, DefaultDevBranchPattern, true, false, runner)
	if err == nil || !strings.Contains(err.Error(), "expected issue #59, got #60") {
		t.Fatalf("expected issue number mismatch error, got %v", err)
	}
}

func TestFetchDevIssueAcceptsNullBody(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &devStartRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/59": []byte(`{"number":59,"title":"No body","body":null,"state":"open","labels":[{"name":"status:ready"}]}`),
	}}

	issue, err := fetchDevIssue(repo, 59, runner)
	if err != nil {
		t.Fatalf("fetchDevIssue error: %v", err)
	}
	if issue.Body != "" {
		t.Fatalf("body = %q, want empty string", issue.Body)
	}
}

func containsCall(calls []string, target string) bool {
	for _, call := range calls {
		if call == target {
			return true
		}
	}
	return false
}
