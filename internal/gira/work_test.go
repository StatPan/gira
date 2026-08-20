package gira

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type workRunner struct {
	outputs map[string][]byte
	queues  map[string][][]byte
	errs    map[string]error
	delays  map[string]time.Duration
	mu      sync.Mutex
	calls   []string
}

func (r *workRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	r.mu.Lock()
	r.calls = append(r.calls, key)
	delay := r.delays[key]
	err, hasErr := r.errs[key]
	queue := r.queues[key]
	if len(queue) > 0 {
		out := queue[0]
		r.queues[key] = queue[1:]
		r.mu.Unlock()
		if delay > 0 {
			time.Sleep(delay)
		}
		if hasErr {
			return nil, err
		}
		return out, nil
	}
	out, hasOut := r.outputs[key]
	r.mu.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}
	if hasErr {
		return nil, err
	}
	if hasOut {
		if strings.HasPrefix(key, "gh pr list ") {
			out = workRunnerPRListWithHeadSHA(out)
		}
		return out, nil
	}
	if out, ok := defaultBranchPolicyTestOutput(key); ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected call: %s", key)
}

func defaultBranchPolicyTestOutput(key string) ([]byte, bool) {
	if key == "git remote get-url origin" {
		return []byte("https://github.com/StatPan/gira.git"), true
	}
	if strings.HasPrefix(key, "gh api repos/") && strings.HasSuffix(key, "/reviews --paginate --slurp") {
		return []byte(`[[{"state":"APPROVED","commit_id":"head220"}]]`), true
	}
	if strings.HasPrefix(key, "gh repo view ") && strings.HasSuffix(key, " --json nameWithOwner,viewerPermission,defaultBranchRef") {
		return []byte(`{"nameWithOwner":"StatPan/gira","viewerPermission":"WRITE","defaultBranchRef":{"name":"main"}}`), true
	}
	if key == "git ls-remote --exit-code --heads origin main" {
		return []byte("abc\trefs/heads/main"), true
	}
	if key == "git status --porcelain" {
		return nil, true
	}
	if key == "git branch --show-current" {
		return []byte("main\n"), true
	}
	if strings.HasPrefix(key, "gh api repos/") && strings.Contains(key, " -X PATCH -f body=") {
		return nil, true
	}
	return nil, false
}

func workRunnerPRListWithHeadSHA(out []byte) []byte {
	var prs []map[string]any
	if err := json.Unmarshal(out, &prs); err != nil {
		return out
	}
	for _, pr := range prs {
		if _, ok := pr["headRefOid"]; !ok {
			pr["headRefOid"] = "head220"
		}
	}
	normalized, err := json.Marshal(prs)
	if err != nil {
		return out
	}
	return normalized
}

func TestStartWorkApplyReadyIssueCreatesBranchAndMovesInProgress(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueJSON := []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:ready"}]}`)
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126":                                               issueJSON,
		"git checkout -b issue-126-work-command origin/main":                                 nil,
		"gh api repos/StatPan/gira/issues/126/labels/status:ready -X DELETE":                 nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-progress": nil,
	}, errs: map[string]error{
		"git show-ref --verify --quiet refs/heads/issue-126-work-command": fmt.Errorf("exit status 1"),
		"git ls-remote --exit-code --heads origin issue-126-work-command": fmt.Errorf("exit status 2"),
	}}

	result, err := StartWork(repo, 126, false, runner)
	if err != nil {
		t.Fatalf("StartWork error: %v", err)
	}
	if result.Status != "In progress" || !result.CreatedBranch || !result.Started || result.ExecutionState != "applied" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.SchemaVersion != WorkStartResultSchemaVersion {
		t.Fatalf("schema version = %q, want %q", result.SchemaVersion, WorkStartResultSchemaVersion)
	}
	if result.BaseBranch != "main" || result.BaseSource != "branch_policy.default" {
		t.Fatalf("unexpected base resolution: %+v", result)
	}
	if !containsCall(runner.calls, "gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-progress") {
		t.Fatalf("missing status apply call: %v", runner.calls)
	}
	if !containsCallWith(runner.calls, "gh api repos/StatPan/gira/issues/126 -X PATCH -f body=", "base_branch: main") {
		t.Fatalf("missing lifecycle base recording call: %v", runner.calls)
	}
}

func TestStartWorkUsesExplicitRepositoryBranchPattern(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".gira"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := "repo: StatPan/gira\nprofiles:\n  default:\n    labels: []\n    milestones: []\n    issue_templates: []\nbranch_policy:\n  mode: custom\n  feature_branch_pattern: feat/i{number}-{slug}\n"
	if err := os.WriteFile(filepath.Join(root, ".gira", "config.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueJSON := []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:ready"}]}`)
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": issueJSON,
	}, errs: map[string]error{
		"git show-ref --verify --quiet refs/heads/feat/i126-work-command": fmt.Errorf("exit status 1"),
		"git ls-remote --exit-code --heads origin feat/i126-work-command": fmt.Errorf("exit status 2"),
	}}

	result, err := StartWork(repo, 126, true, runner)
	if err != nil {
		t.Fatalf("StartWork error: %v", err)
	}
	if result.Branch != "feat/i126-work-command" || result.PolicyMode != BranchPolicyModeCustom {
		t.Fatalf("repository branch pattern was not resolved: %+v", result)
	}
}

func TestStartWorkExplicitModeRequiresBranchSelectionBeforeApply(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".gira"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := "repo: StatPan/gira\nprofiles:\n  default:\n    labels: []\n    milestones: []\n    issue_templates: []\nbranch_policy:\n  mode: github-flow\n  start_mode: explicit\n"
	if err := os.WriteFile(filepath.Join(root, ".gira", "config.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:ready"}]}`),
	}}
	dry, err := StartWorkWithOptions(repo, 126, WorkStartOptions{DryRun: true}, runner)
	if err != nil || !dry.SelectionRequired || dry.BranchStrategy != "selection-required" || dry.SuggestedBranch != "issue/126-work-command" {
		t.Fatalf("unexpected explicit dry run: result=%+v err=%v", dry, err)
	}
	if containsCall(runner.calls, "git checkout -b issue-126-work-command origin/main") {
		t.Fatalf("explicit dry-run must not plan an implicit branch creation: %v", runner.calls)
	}
	_, err = StartWorkWithOptions(repo, 126, WorkStartOptions{}, runner)
	if err == nil || !strings.Contains(err.Error(), "branch strategy is required") {
		t.Fatalf("apply without strategy error = %v", err)
	}
	if containsCall(runner.calls, "gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-progress") {
		t.Fatalf("explicit selection failure must remain before mutation: %v", runner.calls)
	}
}

func TestStartWorkExplicitModeBindsCurrentBranchWithoutCheckout(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".gira"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := "repo: StatPan/gira\nprofiles:\n  default:\n    labels: []\n    milestones: []\n    issue_templates: []\nbranch_policy:\n  mode: github-flow\n  start_mode: explicit\n"
	if err := os.WriteFile(filepath.Join(root, ".gira", "config.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126":                                               []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:ready"}]}`),
		"git branch --show-current":                                                          []byte("release/fix-queue\n"),
		"gh api repos/StatPan/gira/issues/126/labels/status:ready -X DELETE":                 nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-progress": nil,
	}}
	result, err := StartWorkWithOptions(repo, 126, WorkStartOptions{Current: true}, runner)
	if err != nil || result.Branch != "release/fix-queue" || result.BranchStrategy != "current" || result.CreatedBranch {
		t.Fatalf("unexpected current-branch bind: result=%+v err=%v", result, err)
	}
	if containsCall(runner.calls, "git checkout release/fix-queue") || containsCall(runner.calls, "git push -u origin release/fix-queue") {
		t.Fatalf("current-branch bind must not checkout or push: %v", runner.calls)
	}
	if !containsCallWith(runner.calls, "gh api repos/StatPan/gira/issues/126 -X PATCH -f body=", "work_branch_source: current") {
		t.Fatalf("missing current branch lifecycle source: %v", runner.calls)
	}
}

func TestStartWorkExplicitModeAdoptsExistingBranchWithoutCheckout(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".gira"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := "repo: StatPan/gira\nprofiles:\n  default:\n    labels: []\n    milestones: []\n    issue_templates: []\nbranch_policy:\n  mode: github-flow\n  start_mode: explicit\n"
	if err := os.WriteFile(filepath.Join(root, ".gira", "config.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126":                                               []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:ready"}]}`),
		"git show-ref --verify --quiet refs/heads/team/release-fix":                          nil,
		"gh api repos/StatPan/gira/issues/126/labels/status:ready -X DELETE":                 nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-progress": nil,
	}}
	result, err := StartWorkWithOptions(repo, 126, WorkStartOptions{AdoptBranch: "team/release-fix"}, runner)
	if err != nil || result.Branch != "team/release-fix" || result.BranchStrategy != "adopt" || result.CreatedBranch {
		t.Fatalf("unexpected existing-branch adoption: result=%+v err=%v", result, err)
	}
	if containsCall(runner.calls, "git checkout team/release-fix") || containsCall(runner.calls, "git push -u origin team/release-fix") {
		t.Fatalf("adoption must not checkout or push: %v", runner.calls)
	}
	if !containsCallWith(runner.calls, "gh api repos/StatPan/gira/issues/126 -X PATCH -f body=", "work_branch_source: adopted") {
		t.Fatalf("missing adopted branch lifecycle source: %v", runner.calls)
	}
}

func TestStartWorkExplicitModeRejectsMissingBaseBeforeBindingBranch(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".gira"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := "repo: StatPan/gira\nprofiles:\n  default:\n    labels: []\n    milestones: []\n    issue_templates: []\nbranch_policy:\n  mode: github-flow\n  start_mode: explicit\n"
	if err := os.WriteFile(filepath.Join(root, ".gira", "config.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:ready"}]}`),
		"git branch --show-current":            []byte("release/fix-queue\n"),
	}, errs: map[string]error{
		"git ls-remote --exit-code --heads origin main": fmt.Errorf("exit status 2"),
	}}

	_, err := StartWorkWithOptions(repo, 126, WorkStartOptions{Current: true}, runner)
	if err == nil || !strings.Contains(err.Error(), `base branch "main" does not exist`) {
		t.Fatalf("expected missing base error, got %v", err)
	}
	if containsCall(runner.calls, "git branch --show-current") || containsCall(runner.calls, "gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-progress") {
		t.Fatalf("missing base must fail before binding or mutation: %v", runner.calls)
	}
}

func TestPrepareWorkPRBranchPushRejectsResolvedNonMainBase(t *testing.T) {
	runner := &workRunner{outputs: map[string][]byte{
		"git branch --show-current": []byte("develop\n"),
	}}
	_, err := prepareWorkPRBranchPush(devStartIssue{Number: 126, Title: "Work command"}, 126, "develop", true, runner)
	if err == nil || !strings.Contains(err.Error(), "resolved base branch develop") {
		t.Fatalf("expected resolved base branch guard, got %v", err)
	}
}

func TestStartWorkDryRunAcceptsExplicitReleaseAndHotfixBase(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	for _, base := range []string{"release/2.0", "hotfix/urgent-fix"} {
		t.Run(base, func(t *testing.T) {
			runner := &workRunner{outputs: map[string][]byte{
				"gh api repos/StatPan/gira/issues/126":             []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:ready"}]}`),
				"git ls-remote --exit-code --heads origin " + base: []byte("abc\trefs/heads/" + base),
			}, errs: map[string]error{
				"git show-ref --verify --quiet refs/heads/issue-126-work-command": fmt.Errorf("exit status 1"),
				"git ls-remote --exit-code --heads origin issue-126-work-command": fmt.Errorf("exit status 2"),
			}}

			result, err := StartWorkWithOptions(repo, 126, WorkStartOptions{DryRun: true, BaseOverride: base}, runner)
			if err != nil {
				t.Fatalf("StartWorkWithOptions error: %v", err)
			}
			if result.BaseBranch != base || result.BaseSource != "explicit --base" {
				t.Fatalf("unexpected base result: %+v", result)
			}
			if result.SchemaVersion != WorkStartResultSchemaVersion {
				t.Fatalf("schema version = %q, want %q", result.SchemaVersion, WorkStartResultSchemaVersion)
			}
			if !result.Checks["base_branch_exists"] {
				t.Fatalf("expected base branch check: %+v", result.Checks)
			}
		})
	}
}

func TestStartWorkDryRunIncludesApprovalEvidence(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:ready"}]}`),
	}, errs: map[string]error{
		"git show-ref --verify --quiet refs/heads/issue-126-work-command": fmt.Errorf("exit status 1"),
		"git ls-remote --exit-code --heads origin issue-126-work-command": fmt.Errorf("exit status 2"),
	}}

	result, err := StartWork(repo, 126, true, runner)
	if err != nil {
		t.Fatalf("StartWork error: %v", err)
	}
	if result.Started || result.ExecutionState != "planned" || result.NextStatus != "In progress" {
		t.Fatalf("dry run must be explicitly planned, got %+v", result)
	}
	approval := result.Approval
	if approval == nil {
		t.Fatalf("expected approval evidence: %+v", result)
	}
	if approval.SchemaVersion != ApprovalPlanSchemaVersion || approval.CanonicalCommand != "gira work start" || approval.Capability != AdapterCapabilityApplyMutation {
		t.Fatalf("unexpected approval identity: %+v", approval)
	}
	if result.SchemaVersion != WorkStartResultSchemaVersion {
		t.Fatalf("schema version = %q, want %q", result.SchemaVersion, WorkStartResultSchemaVersion)
	}
	if approval.ApplyCommand != "gira work start --repo StatPan/gira --issue 126 --branch new --apply" || approval.DryRunCommand != "gira work start --repo StatPan/gira --issue 126 --branch new --dry-run" {
		t.Fatalf("unexpected approval commands: %+v", approval)
	}
	if approval.OutputSchema != WorkStartResultSchemaVersion || approval.PostApplyVerification != "gira ticket status 126 --repo StatPan/gira --json" {
		t.Fatalf("unexpected approval verification fields: %+v", approval)
	}
	if len(approval.PlannedActions) < 3 || !approvalHasAction(approval.PlannedActions, "branch:create_or_reuse") || !approvalHasAction(approval.PlannedActions, "issue_status:update") {
		t.Fatalf("approval missing planned actions: %+v", approval.PlannedActions)
	}
	if approval.Blockers == nil || approval.Warnings == nil {
		t.Fatalf("approval blockers and warnings must be stable arrays: %+v", approval)
	}
}

func TestStartWorkApplyRejectsDirtyWorktreeBeforeBranchMutation(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:ready"}]}`),
		"git status --porcelain":               []byte(" M internal/gira/work.go\n"),
	}, errs: map[string]error{
		"git show-ref --verify --quiet refs/heads/issue-126-work-command": fmt.Errorf("exit status 1"),
		"git ls-remote --exit-code --heads origin issue-126-work-command": fmt.Errorf("exit status 2"),
	}}

	result, err := StartWork(repo, 126, false, runner)
	if err == nil || !strings.Contains(err.Error(), "dirty worktree") {
		t.Fatalf("expected dirty worktree error, got %v", err)
	}
	if result.Started || result.ExecutionState != "blocked_before_mutation" || result.Status != "Ready" || result.NextStatus != "Ready" {
		t.Fatalf("dirty worktree must not look started: %+v", result)
	}
	if result.Preflight == nil || !result.Preflight.Dirty || result.Preflight.ExpectedBranch != "issue-126-work-command" {
		t.Fatalf("missing dirty preflight diagnostics: %+v", result.Preflight)
	}
	if containsCall(runner.calls, "git checkout -b issue-126-work-command origin/main") {
		t.Fatalf("dirty worktree should block checkout, calls=%v", runner.calls)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "/labels") || strings.Contains(call, " -X PATCH -f body=") {
			t.Fatalf("dirty worktree must not mutate ticket state, calls=%v", runner.calls)
		}
	}
}

func TestStartWorkDirtyCurrentWorktreeSuggestsCleanLinkedTarget(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126":            []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:ready"}]}`),
		"git rev-parse --show-toplevel":                   []byte("/workspace/current\n"),
		"git status --porcelain":                          []byte(" M internal/gira/work.go\n"),
		"git worktree list --porcelain":                   []byte("worktree /workspace/current\nHEAD abc\nbranch refs/heads/main\n\nworktree /workspace/ticket-126\nHEAD def\nbranch refs/heads/issue-126-work-command\n\n"),
		"git -C /workspace/ticket-126 status --porcelain": nil,
	}, errs: map[string]error{
		"git show-ref --verify --quiet refs/heads/issue-126-work-command": fmt.Errorf("exit status 1"),
		"git ls-remote --exit-code --heads origin issue-126-work-command": fmt.Errorf("exit status 2"),
	}}

	result, err := StartWork(repo, 126, false, runner)
	if err == nil {
		t.Fatal("expected dirty worktree error")
	}
	if result.Preflight == nil || result.Preflight.CurrentWorktree != "/workspace/current" || result.Preflight.SuggestedWorktree != "/workspace/ticket-126" {
		t.Fatalf("missing linked worktree guidance: %+v", result.Preflight)
	}
	if !strings.Contains(result.NextStep, "cd /workspace/ticket-126") || !strings.Contains(result.NextStep, "gira work start") {
		t.Fatalf("next step must direct the operator to the clean linked worktree: %q", result.NextStep)
	}
}

func TestStartWorkLifecycleFailureIsExplicitlyPartial(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126":               []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:ready"}]}`),
		"git checkout -b issue-126-work-command origin/main": nil,
	}, errs: map[string]error{
		"git show-ref --verify --quiet refs/heads/issue-126-work-command": fmt.Errorf("exit status 1"),
		"git ls-remote --exit-code --heads origin issue-126-work-command": fmt.Errorf("exit status 2"),
		"gh api repos/StatPan/gira/issues/126 -X PATCH -f body=<!-- gira:lifecycle:start -->\nbase_branch: main\nbase_source: branch_policy.default\nbranch_policy_mode: github_flow\ntarget: default\nwork_branch: issue-126-work-command\n<!-- gira:lifecycle:end -->\n": fmt.Errorf("write failed"),
	}}

	result, err := StartWork(repo, 126, false, runner)
	if err == nil {
		t.Fatalf("expected lifecycle failure, got result=%+v err=%v", result, err)
	}
	if result.Started || result.ExecutionState != "partially_applied" || !result.CreatedBranch || result.NextStatus != "Ready" || result.NextStep != "gira work status --repo StatPan/gira --issue 126 --json" {
		t.Fatalf("lifecycle failure must preserve explicit partial state: %+v", result)
	}
}

func approvalHasAction(actions []ApprovalPlannedAction, action string) bool {
	for _, item := range actions {
		if item.Action == action {
			return true
		}
	}
	return false
}

func TestStartWorkFailsMissingReady(t *testing.T) {
	configureManagedRequiredPolicyTest(t)
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"type:task"}]}`),
	}}

	_, err := StartWork(repo, 126, true, runner)
	if err == nil || !strings.Contains(err.Error(), "missing label status:ready") || !strings.Contains(err.Error(), "gira adopt issues --repo StatPan/gira --issue 126 --label status:ready --dry-run") {
		t.Fatalf("expected missing ready error, got %v", err)
	}
}

func TestStartWorkApplyReusesExistingBranch(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueJSON := []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:ready"}]}`)
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126":                                               issueJSON,
		"git show-ref --verify --quiet refs/heads/issue-126-work-command":                    nil,
		"git ls-remote --exit-code --heads origin issue-126-work-command":                    nil,
		"git merge-base issue-126-work-command origin/main":                                  []byte("base126\n"),
		"git rev-list --left-right --count issue-126-work-command...origin/main":             []byte("1\t0\n"),
		"git cherry -v origin/main issue-126-work-command":                                   []byte("+ local126 retained change\n"),
		"git checkout issue-126-work-command":                                                nil,
		"gh api repos/StatPan/gira/issues/126/labels/status:ready -X DELETE":                 nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-progress": nil,
	}}

	result, err := StartWork(repo, 126, false, runner)
	if err != nil {
		t.Fatalf("StartWork error: %v", err)
	}
	if result.CreatedBranch {
		t.Fatalf("expected branch reuse, got created")
	}
	if !containsCall(runner.calls, "git checkout issue-126-work-command") {
		t.Fatalf("expected checkout existing branch, calls=%v", runner.calls)
	}
	if result.BranchReuse == nil || !result.BranchReuse.Safe || result.BranchReuse.Ahead != 1 || result.BranchReuse.Behind != 0 {
		t.Fatalf("expected clean reuse diagnostics, got %+v", result.BranchReuse)
	}
}

func TestStartWorkBlocksStaleDuplicateBranchBeforeStatusMutation(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueJSON := []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:ready"}]}`)
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126":                                   issueJSON,
		"git show-ref --verify --quiet refs/heads/issue-126-work-command":        nil,
		"git ls-remote --exit-code --heads origin issue-126-work-command":        nil,
		"git merge-base issue-126-work-command origin/main":                      []byte("base126\n"),
		"git rev-list --left-right --count issue-126-work-command...origin/main": []byte("1\t1\n"),
		"git cherry -v origin/main issue-126-work-command":                       []byte("- local126 already delivered by squash merge\n"),
	}}

	result, err := StartWork(repo, 126, false, runner)
	if err == nil || !strings.Contains(err.Error(), "behind_base=1") || !strings.Contains(err.Error(), "duplicate_patch_candidates=1") {
		t.Fatalf("expected stale duplicate branch rejection, got result=%+v err=%v", result, err)
	}
	if result.BranchReuse == nil || result.BranchReuse.Safe || result.BranchReuse.Ahead != 1 || result.BranchReuse.Behind != 1 || len(result.BranchReuse.DuplicatePatches) != 1 {
		t.Fatalf("missing deterministic reuse diagnostics: %+v", result.BranchReuse)
	}
	for _, call := range runner.calls {
		if strings.HasPrefix(call, "git checkout") || strings.Contains(call, "/labels") || strings.Contains(call, " -X PATCH -f body=") {
			t.Fatalf("unsafe branch reuse must not checkout or mutate ticket state, calls=%v", runner.calls)
		}
	}
}

func TestStartWorkApplyRerunReusesInProgressBranch(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueJSON := []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`)
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126":                                   issueJSON,
		"git show-ref --verify --quiet refs/heads/issue-126-work-command":        nil,
		"git ls-remote --exit-code --heads origin issue-126-work-command":        nil,
		"git merge-base issue-126-work-command origin/main":                      []byte("base126\n"),
		"git rev-list --left-right --count issue-126-work-command...origin/main": []byte("0\t0\n"),
		"git cherry -v origin/main issue-126-work-command":                       []byte(""),
		"git checkout issue-126-work-command":                                    nil,
	}}

	result, err := StartWork(repo, 126, false, runner)
	if err != nil {
		t.Fatalf("StartWork error: %v", err)
	}
	if result.CreatedBranch || result.Status != "In progress" || result.NextStatus != "In progress" {
		t.Fatalf("expected idempotent in-progress reuse, got %+v", result)
	}
	if !containsCall(runner.calls, "git checkout issue-126-work-command") {
		t.Fatalf("expected checkout existing branch, calls=%v", runner.calls)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "/labels") {
			t.Fatalf("in-progress rerun should not mutate labels, calls=%v", runner.calls)
		}
	}
}

func TestOpenWorkPRApplyDraftKeepsInProgress(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueJSON := []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`)
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": issueJSON,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[]`),
		"git branch --show-current":                            []byte("issue-126-work-command\n"),
		"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte("origin/issue-126-work-command\n"),
		"gh pr create --repo StatPan/gira --title feat: Work command --body Closes #126 --base main --draft": []byte("https://github.com/StatPan/gira/pull/200\n"),
	}, queues: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": {
			[]byte(`[]`),
			[]byte(`[{"number":200,"title":"feat: Work command","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/200","reviewDecision":"","isDraft":true,"mergeStateStatus":"CLEAN","headRefName":"issue-126-work-command","baseRefName":"main","headRefOid":"head220","statusCheckRollup":[]}]`),
		},
	}}

	result, err := OpenWorkPR(repo, 126, false, true, runner)
	if err != nil {
		t.Fatalf("OpenWorkPR error: %v", err)
	}
	if result.NextStatus != "In progress" || result.PRNumber != 200 || !containsCall(result.Blockers, "draft") {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestOpenWorkPRApplyNonDraftMovesInReview(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueJSON := []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`)
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": issueJSON,
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[]`),
		"git branch --show-current":                                                                  []byte("issue-126-work-command\n"),
		"git rev-parse --abbrev-ref --symbolic-full-name @{u}":                                       []byte("origin/issue-126-work-command\n"),
		"gh pr create --repo StatPan/gira --title feat: Work command --body Closes #126 --base main": []byte("https://github.com/StatPan/gira/pull/201\n"),
		"gh api repos/StatPan/gira/issues/126/labels/status:in-progress -X DELETE":                   nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-review":           nil,
	}, queues: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": {
			[]byte(`[]`),
			[]byte(`[{"number":201,"title":"feat: Work command","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/201","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-126-work-command","baseRefName":"main","headRefOid":"head220","statusCheckRollup":[]}]`),
		},
	}}

	result, err := OpenWorkPR(repo, 126, false, false, runner)
	if err != nil {
		t.Fatalf("OpenWorkPR error: %v", err)
	}
	if result.NextStatus != "In review" || result.PRNumber != 201 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestOpenWorkPRApplyUsesRecordedLifecycleBase(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	body := RenderTicketLifecycleBlock(TicketLifecycleState{BaseBranch: "release/2.0", BaseSource: "branch_policy.release", BranchPolicyMode: BranchPolicyModeReleaseTrain, Target: "release"})
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","body":` + strconv.Quote(body) + `,"labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[]`),
		"git ls-remote --exit-code --heads origin release/2.0":                                              []byte("abc\trefs/heads/release/2.0"),
		"git branch --show-current":                                                                         []byte("issue-126-work-command\n"),
		"git rev-parse --abbrev-ref --symbolic-full-name @{u}":                                              []byte("origin/issue-126-work-command\n"),
		"gh pr create --repo StatPan/gira --title feat: Work command --body Closes #126 --base release/2.0": []byte("https://github.com/StatPan/gira/pull/201\n"),
		"gh api repos/StatPan/gira/issues/126/labels/status:in-progress -X DELETE":                          nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-review":                  nil,
	}, queues: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": {
			[]byte(`[]`),
			[]byte(`[{"number":201,"title":"feat: Work command","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/201","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-126-work-command","baseRefName":"release/2.0","headRefOid":"head220","statusCheckRollup":[]}]`),
		},
	}}

	result, err := OpenWorkPR(repo, 126, false, false, runner)
	if err != nil {
		t.Fatalf("OpenWorkPR error: %v", err)
	}
	if result.RecordedBase != "release/2.0" || result.ActualBase != "release/2.0" || result.BaseMismatch {
		t.Fatalf("unexpected base result: %+v", result)
	}
}

func TestGetWorkStatusIncludesDeterministicJSONContractFields(t *testing.T) {
	useFinishReviewPolicy(t, FinishReviewPolicyRequired)
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	body := "## Goal\nShip status contract\n\n## Scope\nTicket status JSON\n\n## Acceptance Criteria\n- exposes PR state\n\n## Doctor Impact\nUpdates status JSON only.\n\n## Expected Evidence\n- go test ./internal/gira\n\n" + RenderTicketLifecycleBlock(TicketLifecycleState{BaseBranch: "main", BaseSource: "branch_policy.default", BranchPolicyMode: BranchPolicyModeGitHubFlow, WorkBranch: "issue-126-work-command"})
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","body":` + strconv.Quote(body) + `,"milestone":{"title":"2.0 Alpha"},"labels":[{"name":"type:task"},{"name":"status:in-review"},{"name":"priority:p1"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[
			{"number":201,"title":"feat: work","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/201","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-126-work-command","baseRefName":"main","headRefOid":"head220","statusCheckRollup":[{"name":"test","workflowName":"ci","status":"COMPLETED","conclusion":"SUCCESS","detailsUrl":"https://ci.example"}]}
		]`),
	}}

	result, err := GetWorkStatus(repo, 126, runner)
	if err != nil {
		t.Fatalf("GetWorkStatus error: %v", err)
	}
	if result.Command != "ticket status" || result.SchemaVersion != "ticket-status/v1" || result.Milestone != "2.0 Alpha" {
		t.Fatalf("missing status contract metadata: %+v", result)
	}
	if !containsString(result.Labels, "priority:p1") || result.Branch == nil || result.Branch.Expected != "issue-126-work-command" || !result.Branch.Trusted {
		t.Fatalf("missing label/branch contract: %+v", result)
	}
	if result.PullRequest == nil || !result.PullRequest.Available || result.PullRequest.HeadRefName != "issue-126-work-command" || result.PullRequest.BaseRefName != "main" {
		t.Fatalf("missing PR contract: %+v", result.PullRequest)
	}
	if result.BranchPolicy == nil || result.BranchPolicy.RecordedBase != "main" || result.BranchPolicy.ActualPRBase != "main" || result.BranchPolicy.BaseMismatch {
		t.Fatalf("missing branch policy contract: %+v", result.BranchPolicy)
	}
	if result.ChecksStatus != "passed" || len(result.Checks) != 1 || result.ReviewStatus != "approved" {
		t.Fatalf("missing check/review contract: %+v", result)
	}
	if result.Evidence == nil || !result.Evidence.ClosingReference || !result.Evidence.BranchTrusted || !containsString(result.Evidence.Sources, "checks") {
		t.Fatalf("missing evidence contract: %+v", result.Evidence)
	}
	if result.TicketReadiness == nil || result.TicketReadiness.SchemaVersion != TicketReadinessSchemaVersion || result.TicketReadiness.Readiness != "ready" {
		t.Fatalf("missing ticket readiness contract: %+v", result.TicketReadiness)
	}
	if result.PRReadiness == nil || result.PRReadiness.SchemaVersion != PRReadinessSchemaVersion || result.PRReadiness.PullRequest != 201 {
		t.Fatalf("missing PR readiness contract: %+v", result.PRReadiness)
	}
}

func TestGetWorkStatusPropagatesStaleApprovalEvidenceAcrossReadinessAndQueues(t *testing.T) {
	useFinishReviewPolicy(t, FinishReviewPolicyRequired)
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Stale review","state":"open","labels":[{"name":"status:in-review"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[{"number":201,"title":"x","body":"Closes #126","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-126-stale-review","baseRefName":"main","headRefOid":"current-head","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
		"gh api repos/StatPan/gira/pulls/201/reviews --paginate --slurp": []byte(`[[{"state":"APPROVED","commit_id":"old-head"}]]`),
	}}

	status, err := GetWorkStatus(repo, 126, runner)
	if err != nil {
		t.Fatalf("GetWorkStatus error: %v", err)
	}
	if !containsString(status.Blockers, "review_approval_stale") || status.ReviewStatus != "blocked" || status.NextAction != "address_review" {
		t.Fatalf("stale review must block ticket status: %+v", status)
	}
	if status.ReviewEvidence == nil || status.ReviewEvidence.Blocker != "review_approval_stale" || status.PRReadiness == nil || !hasPRReadinessFinding(status.PRReadiness.Findings, "review_approval_stale") {
		t.Fatalf("stale review evidence was not propagated to PR readiness: %+v", status)
	}
	queues := BuildWorkspaceQueues(WorkspaceSummary{}, []WorkStatusResult{status})
	if queues.Counts.FinishReady != 0 || queues.Counts.Blocked != 1 || queues.Queues.Blocked[0].Evidence.ReviewEvidence == nil || queues.Queues.Blocked[0].Evidence.ReviewEvidence.Blocker != "review_approval_stale" {
		t.Fatalf("stale review must not enter finish-ready queue: %+v", queues)
	}
}

func TestGetWorkStatusKeepsNoneReviewPolicyNonBlockingAcrossQueues(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".gira", "config.yaml"), "repo: StatPan/gira\nfinish_review_policy: none\nprofiles:\n  default:\n    labels: []\n")
	t.Chdir(root)

	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"No review needed","state":"open","labels":[{"name":"status:in-review"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[{"number":201,"title":"x","body":"Closes #126","state":"OPEN","url":"u","reviewDecision":"CHANGES_REQUESTED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-126-none-review","baseRefName":"main","headRefOid":"current-head","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
	}}

	status, err := GetWorkStatus(repo, 126, runner)
	if err != nil {
		t.Fatalf("GetWorkStatus error: %v", err)
	}
	if containsString(status.Blockers, "review") || status.ReviewStatus != "not_required" || status.NextAction != "merge_when_policy_allows" || status.PRReadiness == nil || status.PRReadiness.Readiness != "ready_for_finish" {
		t.Fatalf("none policy must keep review evidence nonblocking: %+v", status)
	}
	queues := BuildWorkspaceQueues(WorkspaceSummary{}, []WorkStatusResult{status})
	if queues.Counts.ReviewNeeded != 0 || queues.Counts.FinishReady != 1 {
		t.Fatalf("none policy must remain finish-ready outside ticket finish: %+v", queues)
	}
}

func TestGetWorkStatusReportsBranchBaseMismatchWarning(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	body := RenderTicketLifecycleBlock(TicketLifecycleState{BaseBranch: "main", BaseSource: "branch_policy.default", BranchPolicyMode: BranchPolicyModeGitHubFlow})
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","body":` + strconv.Quote(body) + `,"labels":[{"name":"status:in-review"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[{"number":201,"title":"feat: work","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/201","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-126-work-command","baseRefName":"develop","headRefOid":"head220","statusCheckRollup":[]}]`),
	}}

	result, err := GetWorkStatus(repo, 126, runner)
	if err != nil {
		t.Fatalf("GetWorkStatus error: %v", err)
	}
	if result.BranchPolicy == nil || !result.BranchPolicy.BaseMismatch || !containsString(result.BranchPolicy.Diagnostics, "recorded_base_actual_pr_base_mismatch") {
		t.Fatalf("expected branch policy mismatch: %+v", result.BranchPolicy)
	}
	if !containsString(result.Warnings, "recorded_base_actual_pr_base_mismatch") {
		t.Fatalf("expected mismatch warning: %+v", result.Warnings)
	}
	if !containsString(result.Blockers, "pr_base_mismatch") || result.NextAction != "correct_pr_base" || result.NextStep != "gh pr edit 201 --repo StatPan/gira --base main" {
		t.Fatalf("expected safe base correction guidance: %+v", result)
	}
}

func TestOpenWorkPRDryRunReportsMissingLinkedPR(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[]`),
		"git branch --show-current": []byte("issue-126-work-command\n"),
	}, errs: map[string]error{
		"git rev-parse --abbrev-ref --symbolic-full-name @{u}": fmt.Errorf("fatal: no upstream configured: exit status 128"),
	}}

	result, err := OpenWorkPR(repo, 126, true, false, runner)
	if err != nil {
		t.Fatalf("OpenWorkPR error: %v", err)
	}
	if !containsCall(result.Blockers, "missing_linked_pr") {
		t.Fatalf("expected missing linked PR blocker, got %+v", result.Blockers)
	}
	if !containsCall(result.Blockers, "branch_push_required") || result.BranchPush != "planned" {
		t.Fatalf("expected planned branch push blocker, got %+v", result)
	}
	if result.Approval == nil {
		t.Fatalf("expected PR approval evidence: %+v", result)
	}
	if result.Approval.SchemaVersion != ApprovalPlanSchemaVersion || result.Approval.CanonicalCommand != "gira work pr" || result.Approval.OutputSchema != WorkPRResultSchemaVersion {
		t.Fatalf("unexpected PR approval evidence: %+v", result.Approval)
	}
	if result.Approval.ApplyCommand != "gira work pr --repo StatPan/gira --issue 126 --apply" || result.Approval.DryRunCommand != "gira work pr --repo StatPan/gira --issue 126 --dry-run" {
		t.Fatalf("unexpected PR approval commands: %+v", result.Approval)
	}
	if !approvalHasAction(result.Approval.PlannedActions, "branch:push") || !approvalHasAction(result.Approval.PlannedActions, "pr:create") {
		t.Fatalf("approval missing PR planned actions: %+v", result.Approval.PlannedActions)
	}
	if !containsCall(result.Approval.Blockers, "branch_push_required") || result.Approval.Warnings == nil || result.Approval.PostApplyVerification != "gira ticket status 126 --repo StatPan/gira --json" {
		t.Fatalf("unexpected PR approval verification fields: %+v", result.Approval)
	}
	if result.PushRemote != "origin" || result.LocalGit != "git push -u origin <validated-ticket-branch>" {
		t.Fatalf("expected explicit local git boundary, got %+v", result)
	}
	formatted := FormatWorkPR(result)
	if !strings.Contains(formatted, `local_git="git push -u origin <validated-ticket-branch>"`) {
		t.Fatalf("formatted output missing local git boundary:\n%s", formatted)
	}
	if containsCall(runner.calls, "gh pr create --repo StatPan/gira --title feat: Work command --body Closes #126 --base main") {
		t.Fatalf("dry-run should not create PR, calls=%v", runner.calls)
	}
	if containsCall(runner.calls, "git push -u origin issue-126-work-command") {
		t.Fatalf("dry-run should not push branch, calls=%v", runner.calls)
	}
}

func TestOpenWorkPRApplyPushesUnpushedTicketBranch(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[]`),
		"git branch --show-current":                 []byte("issue-126-work-command\n"),
		"git push -u origin issue-126-work-command": nil,
		"gh pr create --repo StatPan/gira --title feat: Work command --body Closes #126 --base main --draft": []byte("https://github.com/StatPan/gira/pull/204\n"),
	}, queues: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": {
			[]byte(`[]`),
			[]byte(`[{"number":204,"title":"feat: Work command","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/204","reviewDecision":"","isDraft":true,"mergeStateStatus":"CLEAN","headRefName":"issue-126-work-command","baseRefName":"main","headRefOid":"head220","statusCheckRollup":[]}]`),
		},
	}, errs: map[string]error{
		"git rev-parse --abbrev-ref --symbolic-full-name @{u}": fmt.Errorf("fatal: no upstream configured: exit status 128"),
	}}

	result, err := OpenWorkPR(repo, 126, false, true, runner)
	if err != nil {
		t.Fatalf("OpenWorkPR error: %v", err)
	}
	if result.Branch != "issue-126-work-command" || result.BranchPush != "applied" || result.PRNumber != 204 {
		t.Fatalf("expected pushed branch and created PR, got %+v", result)
	}
	pushIndex := callIndex(runner.calls, "git push -u origin issue-126-work-command")
	createIndex := callIndex(runner.calls, "gh pr create --repo StatPan/gira --title feat: Work command --body Closes #126 --base main --draft")
	if pushIndex < 0 || createIndex < 0 || pushIndex > createIndex {
		t.Fatalf("expected push before PR create, calls=%v", runner.calls)
	}
}

func TestOpenWorkPRAcceptsRecordedCustomWorkBranch(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	body := RenderTicketLifecycleBlock(TicketLifecycleState{BaseBranch: "main", WorkBranch: "feat/i126-work-command"})
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","body":` + strconv.Quote(body) + `,"labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[]`),
		"git branch --show-current":                            []byte("feat/i126-work-command\n"),
		"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte("origin/feat/i126-work-command\n"),
	}}

	result, err := OpenWorkPR(repo, 126, true, false, runner)
	if err != nil {
		t.Fatalf("OpenWorkPR error: %v", err)
	}
	if result.Branch != "feat/i126-work-command" || result.BranchPush != "skipped" {
		t.Fatalf("custom recorded branch was not accepted: %+v", result)
	}
}

func TestWorkStatusUsesRecordedWorkBranchAndReportsMismatch(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	body := RenderTicketLifecycleBlock(TicketLifecycleState{WorkBranch: "feat/i126-work-command"})
	issue := devStartIssue{Number: 126, Title: "Work command", State: "open", Body: body, Labels: []string{"type:bug", "status:in-review"}}
	pr := DevPRStatusResult{PRNumber: 201, State: "OPEN", Binding: DevPRBinding{Trusted: true, Source: "closing_reference", HeadRef: "feat/i126-other", BaseRef: "main", Warnings: []string{"branch_name_differs_from_suggestion"}}}

	result := workStatusFromIssueAndPR(repo, 126, issue, pr)
	if result.Branch == nil || result.Branch.Expected != "feat/i126-work-command" || !result.Branch.Trusted || result.Branch.Source != "closing_reference" {
		t.Fatalf("unexpected status branch: %+v", result.Branch)
	}
	if result.BranchPolicy == nil || !containsString(result.BranchPolicy.Diagnostics, "branch_name_differs_from_suggestion") || !containsString(result.Warnings, "branch_name_differs_from_suggestion") {
		t.Fatalf("missing naming advisory: branch_policy=%+v warnings=%+v", result.BranchPolicy, result.Warnings)
	}
}

func TestOpenWorkPRApplyPushesWhenUpstreamIsBaseBranch(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[]`),
		"git branch --show-current":                                                                  []byte("issue-126-work-command\n"),
		"git rev-parse --abbrev-ref --symbolic-full-name @{u}":                                       []byte("origin/main\n"),
		"git push -u origin issue-126-work-command":                                                  nil,
		"gh pr create --repo StatPan/gira --title feat: Work command --body Closes #126 --base main": []byte("https://github.com/StatPan/gira/pull/204\n"),
		"gh api repos/StatPan/gira/issues/126/labels/status:in-progress -X DELETE":                   nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-review":           nil,
	}, queues: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": {
			[]byte(`[]`),
			[]byte(`[{"number":204,"title":"feat: Work command","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/204","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-126-work-command","baseRefName":"main","headRefOid":"head220","statusCheckRollup":[]}]`),
		},
	}}

	result, err := OpenWorkPR(repo, 126, false, false, runner)
	if err != nil {
		t.Fatalf("OpenWorkPR error: %v", err)
	}
	if result.BranchPush != "applied" || result.PRNumber != 204 {
		t.Fatalf("expected push despite base upstream, got %+v", result)
	}
}

func TestOpenWorkPRApplyRejectsUnsafeTicketBranchBeforePush(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[]`),
		"git branch --show-current": []byte("issue-126-work-command:refs/heads/main\n"),
	}}

	_, err := OpenWorkPR(repo, 126, false, false, runner)
	if err == nil || !strings.Contains(err.Error(), "invalid git push branch") {
		t.Fatalf("expected unsafe branch error, got %v", err)
	}
	if containsCall(runner.calls, "git push -u origin issue-126-work-command:refs/heads/main") {
		t.Fatalf("unsafe branch should not be pushed, calls=%v", runner.calls)
	}
}

func TestOpenWorkPRApplySanitizesPushFailure(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[]`),
		"git branch --show-current": []byte("issue-126-work-command\n"),
	}, errs: map[string]error{
		"git rev-parse --abbrev-ref --symbolic-full-name @{u}": fmt.Errorf("fatal: no upstream configured: exit status 128"),
		"git push -u origin issue-126-work-command":            fmt.Errorf("remote https://token@example.invalid/repo denied"),
	}}

	_, err := OpenWorkPR(repo, 126, false, false, runner)
	if err == nil {
		t.Fatalf("expected push failure")
	}
	if strings.Contains(err.Error(), "token@example") || strings.Contains(err.Error(), "https://") {
		t.Fatalf("push error leaked credential material: %v", err)
	}
	if !strings.Contains(err.Error(), "inspect local git output") {
		t.Fatalf("push error missing operator guidance: %v", err)
	}
}

func TestValidateGitPushTargetRejectsRemoteURLWithoutEchoingCredentials(t *testing.T) {
	err := validateGitPushTarget("https://token@example.invalid/repo", "issue-126-work-command")
	if err == nil {
		t.Fatalf("expected invalid remote")
	}
	if strings.Contains(err.Error(), "token") || strings.Contains(err.Error(), "https://") {
		t.Fatalf("remote validation leaked credential material: %v", err)
	}
}

func TestOpenWorkPRApplyAcceptsValidCustomBranchWithAdvisory(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[]`),
		"git branch --show-current":            []byte("team/work-command\n"),
		"git push -u origin team/work-command": nil,
		"gh pr create --repo StatPan/gira --title feat: Work command --body Closes #126 --base main": []byte("https://github.com/StatPan/gira/pull/204\n"),
		"gh api repos/StatPan/gira/issues/126/labels/status:in-progress -X DELETE":                   nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-review":           nil,
	}, queues: map[string][][]byte{
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": {
			[]byte(`[]`),
			[]byte(`[{"number":204,"title":"feat: Work command","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/204","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"team/work-command","baseRefName":"main","headRefOid":"head220","statusCheckRollup":[]}]`),
		},
	}, errs: map[string]error{
		"git rev-parse --abbrev-ref --symbolic-full-name @{u}": fmt.Errorf("fatal: no upstream configured: exit status 128"),
	}}

	result, err := OpenWorkPR(repo, 126, false, false, runner)
	if err != nil {
		t.Fatalf("OpenWorkPR error: %v", err)
	}
	if result.Branch != "team/work-command" || result.BranchPush != "applied" || !result.Created || !containsString(result.Warnings, "branch_name_differs_from_suggestion") {
		t.Fatalf("expected custom branch to be created with advisory, got %+v", result)
	}
	if !containsCall(runner.calls, "git push -u origin team/work-command") {
		t.Fatalf("custom branch should be pushed, calls=%v", runner.calls)
	}
}

func callIndex(calls []string, target string) int {
	for i, call := range calls {
		if call == target {
			return i
		}
	}
	return -1
}

func containsCallWith(calls []string, prefix string, contains string) bool {
	for _, call := range calls {
		if strings.HasPrefix(call, prefix) && strings.Contains(call, contains) {
			return true
		}
	}
	return false
}

func TestOpenWorkPRApplyReusesExistingLinkedPR(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[{"number":202,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/202","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`),
		"gh api repos/StatPan/gira/issues/126/labels/status:in-progress -X DELETE":         nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-review": nil,
	}}

	result, err := OpenWorkPR(repo, 126, false, false, runner)
	if err != nil {
		t.Fatalf("OpenWorkPR error: %v", err)
	}
	if result.Created || result.PRNumber != 202 {
		t.Fatalf("expected existing PR reuse, got %+v", result)
	}
	for _, call := range runner.calls {
		if strings.HasPrefix(call, "gh pr create ") {
			t.Fatalf("unexpected PR create call: %v", runner.calls)
		}
	}
}

func TestOpenWorkPRRejectsExistingPRBaseMismatch(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	body := RenderTicketLifecycleBlock(TicketLifecycleState{BaseBranch: "main", BaseSource: "branch_policy.default"})
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","body":` + strconv.Quote(body) + `,"labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[{"number":202,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/202","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-126-work-command","baseRefName":"develop","headRefOid":"head220","statusCheckRollup":[]}]`),
	}}

	result, err := OpenWorkPR(repo, 126, false, false, runner)
	if err == nil || !strings.Contains(err.Error(), "does not match recorded ticket base") {
		t.Fatalf("expected base mismatch error, got result=%+v err=%v", result, err)
	}
	if !result.BaseMismatch || result.RecordedBase != "main" || result.ActualBase != "develop" || !containsString(result.Blockers, "pr_base_mismatch") {
		t.Fatalf("missing mismatch result details: %+v", result)
	}
	if result.NextStep != "gh pr edit 202 --repo StatPan/gira --base main" {
		t.Fatalf("missing safe base correction next step: %+v", result)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "/labels") {
			t.Fatalf("base mismatch should not mutate labels, calls=%v", runner.calls)
		}
	}
}

func TestOpenWorkPRApplyDraftFlagReusesExistingNonDraftPR(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[{"number":202,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/202","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`),
		"gh api repos/StatPan/gira/issues/126/labels/status:in-progress -X DELETE":         nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-review": nil,
	}}

	result, err := OpenWorkPR(repo, 126, false, true, runner)
	if err != nil {
		t.Fatalf("OpenWorkPR error: %v", err)
	}
	if result.Created || result.Draft || result.NextStatus != "In review" || result.PRNumber != 202 {
		t.Fatalf("expected existing non-draft PR to stay non-draft, got %+v", result)
	}
	for _, call := range runner.calls {
		if strings.HasPrefix(call, "gh pr create ") {
			t.Fatalf("unexpected PR create call: %v", runner.calls)
		}
	}
}

func TestOpenWorkPRExistingDraftStaysInProgress(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:ready"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[{"number":202,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/202","reviewDecision":"","isDraft":true,"mergeStateStatus":"UNKNOWN","statusCheckRollup":[]}]`),
		"gh api repos/StatPan/gira/issues/126/labels/status:ready -X DELETE":                 nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-progress": nil,
	}}

	result, err := OpenWorkPR(repo, 126, false, false, runner)
	if err != nil {
		t.Fatalf("OpenWorkPR error: %v", err)
	}
	if result.NextStatus != "In progress" || !result.Draft {
		t.Fatalf("expected existing draft to stay in progress, got %+v", result)
	}
	if containsCall(runner.calls, "gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-review") {
		t.Fatalf("draft PR should not move to in-review, calls=%v", runner.calls)
	}
}

func TestOpenWorkPRApplyRevalidatesTerminalPairAfterStaleDryRun(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueCall := "gh api repos/StatPan/gira/issues/126"
	prCall := "gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20"
	openIssue := []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`)
	closedDoneIssue := []byte(`{"number":126,"title":"Work command","state":"closed","labels":[{"name":"status:done"}]}`)
	openPR := []byte(`[{"number":202,"title":"feat: Work command","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/202","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-126-work-command","baseRefName":"main","headRefOid":"head220","statusCheckRollup":[]}]`)
	mergedPR := []byte(`[{"number":202,"title":"feat: Work command","body":"Closes #126","state":"MERGED","url":"https://github.com/StatPan/gira/pull/202","reviewDecision":"","isDraft":false,"mergeStateStatus":"UNKNOWN","headRefName":"issue-126-work-command","baseRefName":"main","headRefOid":"head220","statusCheckRollup":[]}]`)
	runner := &workRunner{
		outputs: map[string][]byte{issueCall: closedDoneIssue, prCall: mergedPR},
		queues: map[string][][]byte{
			issueCall: {openIssue, openIssue, closedDoneIssue},
			prCall:    {openPR, openPR, mergedPR},
		},
	}

	dryRun, err := OpenWorkPR(repo, 126, true, false, runner)
	if err != nil {
		t.Fatalf("stale dry-run error: %v", err)
	}
	if dryRun.NextStatus != "In review" {
		t.Fatalf("dry-run should reflect the then-open pair: %+v", dryRun)
	}

	result, err := OpenWorkPR(repo, 126, false, false, runner)
	if err != nil {
		t.Fatalf("terminal apply error: %v", err)
	}
	if result.Status != "Done" || result.NextStatus != "Done" || result.PRNumber != 202 || len(result.Blockers) != 0 {
		t.Fatalf("terminal apply did not converge to done: %+v", result)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "/labels/status:in-review") || strings.Contains(call, "labels[]=status:in-review") {
			t.Fatalf("terminal apply regressed in-review: %v", runner.calls)
		}
	}

	result, err = OpenWorkPR(repo, 126, false, false, runner)
	if err != nil || result.Status != "Done" || result.NextStatus != "Done" {
		t.Fatalf("repeated terminal apply was not idempotent: result=%+v err=%v", result, err)
	}
}

func TestOpenWorkPRApplyFailsClosedIssueOpenPRWithoutLabelMutation(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueCall := "gh api repos/StatPan/gira/issues/126"
	prCall := "gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20"
	issue := []byte(`{"number":126,"title":"Work command","state":"closed","labels":[{"name":"status:done"}]}`)
	pr := []byte(`[{"number":202,"title":"feat: Work command","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/202","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-126-work-command","baseRefName":"main","headRefOid":"head220","statusCheckRollup":[]}]`)
	runner := &workRunner{outputs: map[string][]byte{issueCall: issue, prCall: pr}}

	result, err := OpenWorkPR(repo, 126, false, false, runner)
	if err == nil || !strings.Contains(err.Error(), "terminal_state_mismatch") {
		t.Fatalf("expected closed/open terminal mismatch, result=%+v err=%v", result, err)
	}
	if !containsString(result.Blockers, "terminal_state_mismatch") {
		t.Fatalf("missing stable terminal mismatch blocker: %+v", result)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "/labels") || strings.HasPrefix(call, "gh pr create ") || strings.HasPrefix(call, "git push ") {
			t.Fatalf("closed/open mismatch must not mutate provider state: %v", runner.calls)
		}
	}
}

func TestOpenWorkPRApplyFailsClosedAmbiguousMergedPairWithoutLabelMutation(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueCall := "gh api repos/StatPan/gira/issues/126"
	prCall := "gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20"
	issue := []byte(`{"number":126,"title":"Work command","state":"closed","labels":[{"name":"status:done"}]}`)
	prs := []byte(`[{"number":202,"title":"first","body":"Closes #126","state":"MERGED","url":"https://github.com/StatPan/gira/pull/202","reviewDecision":"","isDraft":false,"mergeStateStatus":"UNKNOWN","headRefName":"issue-126-work-command","baseRefName":"main","headRefOid":"head220","statusCheckRollup":[]},{"number":203,"title":"second","body":"Closes #126","state":"MERGED","url":"https://github.com/StatPan/gira/pull/203","reviewDecision":"","isDraft":false,"mergeStateStatus":"UNKNOWN","headRefName":"issue-126-work-command","baseRefName":"main","headRefOid":"head221","statusCheckRollup":[]}]`)
	runner := &workRunner{outputs: map[string][]byte{issueCall: issue, prCall: prs}}

	result, err := OpenWorkPR(repo, 126, false, false, runner)
	if err == nil || !strings.Contains(err.Error(), "terminal_state_ambiguous") {
		t.Fatalf("expected ambiguous terminal pairing, result=%+v err=%v", result, err)
	}
	if !containsString(result.Blockers, "terminal_state_ambiguous") {
		t.Fatalf("missing stable ambiguity blocker: %+v", result)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "/labels") || strings.Contains(call, "labels[]=status:in-review") {
			t.Fatalf("ambiguous terminal pairing must not mutate labels: %v", runner.calls)
		}
	}
}

func TestOpenWorkPRApplyClosedIssueWithoutPRStopsBeforeCreate(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueCall := "gh api repos/StatPan/gira/issues/126"
	prCall := "gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20"
	runner := &workRunner{outputs: map[string][]byte{
		issueCall: []byte(`{"number":126,"title":"Work command","state":"closed","labels":[{"name":"status:done"}]}`),
		prCall:    []byte(`[]`),
	}}

	result, err := OpenWorkPR(repo, 126, false, false, runner)
	if err == nil || !strings.Contains(err.Error(), "terminal_state_mismatch") {
		t.Fatalf("expected closed/no-pr mismatch, result=%+v err=%v", result, err)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "/labels") || strings.HasPrefix(call, "gh pr create ") || strings.HasPrefix(call, "git push ") {
			t.Fatalf("closed issue without PR must stop before mutation: %v", runner.calls)
		}
	}
}

func TestGetWorkStatusReportsNextAction(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[{"number":203,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/203","reviewDecision":"","isDraft":true,"mergeStateStatus":"UNKNOWN","statusCheckRollup":[]}]`),
	}}

	result, err := GetWorkStatus(repo, 126, runner)
	if err != nil {
		t.Fatalf("GetWorkStatus error: %v", err)
	}
	if result.NextAction != "mark_pr_ready" {
		t.Fatalf("next action = %q", result.NextAction)
	}
}

func TestGetWorkStatusFetchesIssueAndPRConcurrently(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueCall := "gh api repos/StatPan/gira/issues/126"
	prCall := "gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20"
	runner := &workRunner{
		outputs: map[string][]byte{
			issueCall: []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
			prCall:    []byte(`[{"number":203,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/203","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS"}]}]`),
		},
		delays: map[string]time.Duration{
			issueCall: 80 * time.Millisecond,
			prCall:    80 * time.Millisecond,
		},
	}

	start := time.Now()
	result, err := GetWorkStatus(repo, 126, runner)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("GetWorkStatus error: %v", err)
	}
	if result.PRNumber != 203 || result.NextAction != "merge_when_policy_allows" {
		t.Fatalf("unexpected status: %+v", result)
	}
	if elapsed > 140*time.Millisecond {
		t.Fatalf("GetWorkStatus took %s, want concurrent fetch under 140ms", elapsed)
	}
}

func TestGetWorkStatusRetriesTransientMissingLinkedPRForReviewStatus(t *testing.T) {
	useFinishReviewPolicy(t, FinishReviewPolicyRequired)
	restoreDelay := workStatusMissingPRRetryDelay
	workStatusMissingPRRetryDelay = 0
	t.Cleanup(func() {
		workStatusMissingPRRetryDelay = restoreDelay
	})
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	prCall := "gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20"
	runner := &workRunner{
		outputs: map[string][]byte{
			"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-review"}]}`),
		},
		queues: map[string][][]byte{
			prCall: {
				[]byte(`[]`),
				[]byte(`[{"number":203,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/203","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS"}]}]`),
			},
		},
	}

	result, err := GetWorkStatus(repo, 126, runner)
	if err != nil {
		t.Fatalf("GetWorkStatus error: %v", err)
	}
	if result.PRNumber != 203 || result.PRLookupAttempts != 2 || result.NextAction != "address_review" || !containsString(result.Blockers, "review_required_but_absent") {
		t.Fatalf("unexpected retry status: %+v", result)
	}
	if got := countWorkCall(runner.calls, prCall); got != 2 {
		t.Fatalf("expected one retry, got %d calls: %v", got, runner.calls)
	}
}

func TestGetWorkStatusStopsRetryingMissingLinkedPR(t *testing.T) {
	restoreDelay := workStatusMissingPRRetryDelay
	workStatusMissingPRRetryDelay = 0
	t.Cleanup(func() {
		workStatusMissingPRRetryDelay = restoreDelay
	})
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	prCall := "gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20"
	runner := &workRunner{
		outputs: map[string][]byte{
			"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-review"}]}`),
		},
		queues: map[string][][]byte{
			prCall: {
				[]byte(`[]`),
				[]byte(`[]`),
				[]byte(`[]`),
			},
		},
	}

	result, err := GetWorkStatus(repo, 126, runner)
	if err != nil {
		t.Fatalf("GetWorkStatus error: %v", err)
	}
	if result.PRLookupAttempts != 3 || !containsString(result.Blockers, "missing_linked_pr") {
		t.Fatalf("expected bounded missing PR status: %+v", result)
	}
	if got := countWorkCall(runner.calls, prCall); got != 3 {
		t.Fatalf("expected bounded retry count, got %d calls: %v", got, runner.calls)
	}
}

func TestGetWorkStatusReadyWithoutPRSuggestsStartWork(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:ready"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[]`),
	}}

	result, err := GetWorkStatus(repo, 126, runner)
	if err != nil {
		t.Fatalf("GetWorkStatus error: %v", err)
	}
	if result.NextAction != "start_work" {
		t.Fatalf("next action = %q", result.NextAction)
	}
}

func countWorkCall(calls []string, target string) int {
	count := 0
	for _, call := range calls {
		if call == target {
			count++
		}
	}
	return count
}

func TestGetWorkStatusClosedIssueWithMergedPRReportsDone(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/184": []byte(`{"number":184,"title":"Workspace backlog","state":"closed","labels":[{"name":"type:story"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 184 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[{"number":185,"title":"x","body":"Closes #184","state":"MERGED","url":"https://github.com/StatPan/gira/pull/185","reviewDecision":"","isDraft":false,"mergeStateStatus":"UNKNOWN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
	}}

	result, err := GetWorkStatus(repo, 184, runner)
	if err != nil {
		t.Fatalf("GetWorkStatus error: %v", err)
	}
	if result.NextAction != "done" || result.Status != "Done" || result.PRNumber != 185 {
		t.Fatalf("expected done merged PR status, got %+v", result)
	}
	if got := workStatusNextStep(result); got != "ticket is done" {
		t.Fatalf("next step = %q", got)
	}
}

func TestGetWorkStatusOpenIssueWithMergedPRRequiresCompletionConvergence(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/184": []byte(`{"number":184,"title":"Workspace backlog","state":"open","labels":[{"name":"status:in-review"},{"name":"type:story"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 184 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[{"number":185,"title":"x","body":"Closes #184","state":"MERGED","url":"https://github.com/StatPan/gira/pull/185","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"UNKNOWN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
	}}

	result, err := GetWorkStatus(repo, 184, runner)
	if err != nil {
		t.Fatalf("GetWorkStatus error: %v", err)
	}
	if result.NextAction != "converge_completion_state" || result.Status == "Done" {
		t.Fatalf("open GitHub issue must not be reported as done: %+v", result)
	}
	if got := workStatusNextStep(result); got != "gira ticket finish --repo StatPan/gira --ticket 184 --dry-run" {
		t.Fatalf("next step = %q", got)
	}
}

func TestGetWorkStatusClosedIssueWithoutPRDoesNotSuggestStart(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/188": []byte(`{"number":188,"title":"Closed manually","state":"closed","labels":[]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 188 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName,headRefOid --limit 20": []byte(`[]`),
	}}

	result, err := GetWorkStatus(repo, 188, runner)
	if err != nil {
		t.Fatalf("GetWorkStatus error: %v", err)
	}
	if result.NextAction != "closed" || result.Status != "Closed" {
		t.Fatalf("expected closed next action, got %+v", result)
	}
	if strings.Contains(workStatusNextStep(result), "work start") {
		t.Fatalf("closed ticket should not suggest work start: %+v", result)
	}
}

func TestWorkStatusNextStepExplainsCheckBlockers(t *testing.T) {
	result := WorkStatusResult{Repo: "StatPan/gira", Issue: 126, NextAction: "wait_for_checks"}

	if got := workStatusNextStep(result); got != "wait for required checks to finish or fix failing checks" {
		t.Fatalf("next step = %q", got)
	}
}
