package gira

import (
	"fmt"
	"strings"
	"testing"
)

type devPRRunner struct {
	outputs map[string][]byte
	errs    map[string]error
}

func (r devPRRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
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
