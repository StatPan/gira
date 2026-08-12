package gira

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestResolveRepoBranchPolicyUsesGlobalRepoRegistryOutsideCheckout(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Chdir(t.TempDir())
	writeTestFile(t, filepath.Join(configHome, "gira", "repos", "StatPan", "gira.yaml"), `repo: StatPan/gira
branch_policy:
  mode: git-flow
  default_target: dev
  targets:
    dev: dev
    production: main
`)

	policy, err := resolveRepoBranchPolicy(ParseRepoRefMust("StatPan/gira"), &workRunner{})
	if err != nil {
		t.Fatalf("resolveRepoBranchPolicy returned error: %v", err)
	}
	if policy.ConfigSource != "global_repo_registry" || policy.DefaultTarget != "dev" || policy.Targets["dev"] != "dev" {
		t.Fatalf("global repo policy was not selected: %+v", policy)
	}
	base, err := resolveTicketPRBase(ParseRepoRefMust("StatPan/gira"), devStartIssue{}, &workRunner{})
	if err != nil {
		t.Fatalf("resolveTicketPRBase returned error: %v", err)
	}
	if base.BaseBranch != "dev" || base.BaseSource != "branch_policy.global_repo_registry.dev" {
		t.Fatalf("unexpected resolved base: %+v", base)
	}
	start, err := resolveTicketStartBase(ParseRepoRefMust("StatPan/gira"), devStartIssue{}, "", &workRunner{})
	if err != nil {
		t.Fatalf("resolveTicketStartBase returned error: %v", err)
	}
	if start.BaseBranch != "dev" || start.BaseSource != "branch_policy.global_repo_registry.dev" {
		t.Fatalf("unexpected ticket start base: %+v", start)
	}
}

func TestOpenWorkPRUsesRegisteredDevBase(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Chdir(t.TempDir())
	writeTestFile(t, filepath.Join(configHome, "gira", "repos", "StatPan", "gira.yaml"), `repo: StatPan/gira
branch_policy:
  mode: git-flow
  default_target: dev
  targets:
    dev: dev
    production: main
`)
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[]`),
		"git ls-remote --exit-code --heads origin dev":                                              []byte("abc\trefs/heads/dev"),
		"git branch --show-current":                                                                 []byte("issue-126-work-command\n"),
		"git rev-parse --abbrev-ref --symbolic-full-name @{u}":                                      []byte("origin/issue-126-work-command\n"),
		"gh pr create --repo StatPan/gira --title feat: Work command --body Closes #126 --base dev": []byte("https://github.com/StatPan/gira/pull/201\n"),
		"gh api repos/StatPan/gira/issues/126/labels/status:in-progress -X DELETE":                  nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-review":          nil,
	}}

	result, err := OpenWorkPR(ParseRepoRefMust("StatPan/gira"), 126, false, false, runner)
	if err != nil {
		t.Fatalf("OpenWorkPR returned error: %v", err)
	}
	if result.RecordedBase != "dev" || result.RecordedBaseSource != "branch_policy.global_repo_registry.dev" || result.ActualBase != "dev" {
		t.Fatalf("registered dev base was not preserved through PR create: %+v", result)
	}
}

func TestResolveRepoBranchPolicyUsesWorkspaceThenLocalContractPrecedence(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	checkout := t.TempDir()
	t.Chdir(checkout)
	writeTestFile(t, filepath.Join(configHome, "gira", "config.yaml"), "default_workspace: team\n")
	writeTestFile(t, filepath.Join(configHome, "gira", "workspaces", "team.yaml"), `workspace:
  name: team
  owner: StatPan
  inbox_repo: StatPan/backlog
  repos:
    - StatPan/gira
branch_policy:
  mode: git-flow
  default_target: dev
  targets:
    dev: dev
    production: main
`)

	policy, err := resolveRepoBranchPolicy(ParseRepoRefMust("StatPan/gira"), &workRunner{})
	if err != nil {
		t.Fatalf("resolve workspace policy: %v", err)
	}
	if policy.ConfigSource != "global_workspace_registry" || policy.Targets[policy.DefaultTarget] != "dev" {
		t.Fatalf("workspace policy was not selected: %+v", policy)
	}

	writeTestFile(t, filepath.Join(checkout, ".gira", "config.yaml"), `repo: StatPan/gira
branch_policy:
  mode: github-flow
  default_base: main
profiles:
  default:
    labels: []
`)
	policy, err = resolveRepoBranchPolicy(ParseRepoRefMust("StatPan/gira"), &workRunner{})
	if err != nil {
		t.Fatalf("resolve local policy: %v", err)
	}
	if policy.ConfigSource != "repo_local_contract" || policy.DefaultBase != "main" {
		t.Fatalf("repo-local policy must override registry policy: %+v", policy)
	}
}

func TestOpenWorkPRFailsBeforeMutationWhenRecordedBaseIsMissing(t *testing.T) {
	repo := ParseRepoRefMust("StatPan/gira")
	body := RenderTicketLifecycleBlock(TicketLifecycleState{BaseBranch: "dev", BaseSource: "branch_policy.global_repo_registry.dev"})
	runner := &workRunner{
		outputs: map[string][]byte{
			"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","body":` + strconv.Quote(body) + `,"labels":[{"name":"status:in-progress"}]}`),
			"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[]`),
		},
		errs: map[string]error{
			"git ls-remote --exit-code --heads origin dev": errors.New("exit status 2"),
		},
	}

	_, err := OpenWorkPR(repo, 126, false, false, runner)
	if err == nil || !strings.Contains(err.Error(), `base branch "dev" does not exist on origin`) {
		t.Fatalf("expected missing base error, got %v", err)
	}
	for _, call := range runner.calls {
		if strings.HasPrefix(call, "git push ") || strings.HasPrefix(call, "gh pr create ") || strings.Contains(call, "/labels") {
			t.Fatalf("missing base must fail before mutation, calls=%v", runner.calls)
		}
	}
}

func TestGetWorkStatusReportsRecordedDevActualMainWithoutRetargeting(t *testing.T) {
	repo := ParseRepoRefMust("StatPan/gira")
	body := RenderTicketLifecycleBlock(TicketLifecycleState{BaseBranch: "dev", BaseSource: "branch_policy.global_repo_registry.dev"})
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","body":` + strconv.Quote(body) + `,"labels":[{"name":"status:in-review"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[{"number":201,"title":"feat: work","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/201","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-126-work-command","baseRefName":"main","headRefOid":"head220","statusCheckRollup":[]}]`),
	}}

	result, err := GetWorkStatus(repo, 126, runner)
	if err != nil {
		t.Fatalf("GetWorkStatus returned error: %v", err)
	}
	if result.BranchPolicy == nil || !result.BranchPolicy.BaseMismatch || result.BranchPolicy.RecordedBase != "dev" || result.BranchPolicy.ActualPRBase != "main" {
		t.Fatalf("expected recorded dev versus actual main mismatch: %+v", result.BranchPolicy)
	}
	if result.NextAction != "correct_pr_base" || result.NextStep != "gh pr edit 201 --repo StatPan/gira --base dev" {
		t.Fatalf("expected explicit, non-automatic correction path: %+v", result)
	}
	for _, call := range runner.calls {
		if strings.HasPrefix(call, "gh pr edit ") {
			t.Fatalf("status must not retarget the PR, calls=%v", runner.calls)
		}
	}
}
