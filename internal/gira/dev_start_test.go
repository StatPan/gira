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
		"gh api repos/StatPan/gira/issues/59":                     []byte(issueJSON),
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
		"gh api repos/StatPan/gira/issues/59":                       []byte(issueJSON),
		"git ls-remote --exit-code --heads origin issue-59-add-api-start-branch": []byte("abc\trefs/heads/issue-59-add-api-start-branch"),
	}, errs: map[string]error{
		"git show-ref --verify --quiet refs/heads/issue-59-add-api-start-branch": fmt.Errorf("exit status 1"),
	}}
	_, err := StartDevBranch(repo, 59, DefaultDevBranchPattern, true, false, runner)
	if err == nil || !strings.Contains(err.Error(), "branch conflict") {
		t.Fatalf("expected branch conflict error, got %v", err)
	}
}
