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
