package gira

import (
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
		return out, nil
	}
	if out, ok := defaultBranchPolicyTestOutput(key); ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected call: %s", key)
}

func defaultBranchPolicyTestOutput(key string) ([]byte, bool) {
	if strings.HasPrefix(key, "gh repo view ") && strings.HasSuffix(key, " --json nameWithOwner,viewerPermission,defaultBranchRef") {
		return []byte(`{"nameWithOwner":"StatPan/gira","viewerPermission":"WRITE","defaultBranchRef":{"name":"main"}}`), true
	}
	if key == "git ls-remote --exit-code --heads origin main" {
		return []byte("abc\trefs/heads/main"), true
	}
	if key == "git status --porcelain" {
		return nil, true
	}
	if strings.HasPrefix(key, "gh api repos/") && strings.Contains(key, " -X PATCH -f body=") {
		return nil, true
	}
	return nil, false
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
	if result.Status != "In progress" || !result.CreatedBranch {
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
	if approval.ApplyCommand != "gira work start --repo StatPan/gira --issue 126 --apply" || approval.DryRunCommand != "gira work start --repo StatPan/gira --issue 126 --dry-run" {
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

	_, err := StartWork(repo, 126, false, runner)
	if err == nil || !strings.Contains(err.Error(), "dirty worktree") {
		t.Fatalf("expected dirty worktree error, got %v", err)
	}
	if containsCall(runner.calls, "git checkout -b issue-126-work-command origin/main") {
		t.Fatalf("dirty worktree should block checkout, calls=%v", runner.calls)
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
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"type:task"}]}`),
	}}

	_, err := StartWork(repo, 126, true, runner)
	if err == nil || !strings.Contains(err.Error(), "missing label status:ready") || !strings.Contains(err.Error(), "gira adopt issues --repo StatPan/gira --issue 126 --label status:ready --apply") {
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
}

func TestStartWorkApplyRerunReusesInProgressBranch(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issueJSON := []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`)
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126":                            issueJSON,
		"git show-ref --verify --quiet refs/heads/issue-126-work-command": nil,
		"git ls-remote --exit-code --heads origin issue-126-work-command": nil,
		"git checkout issue-126-work-command":                             nil,
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
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[]`),
		"git branch --show-current":                            []byte("issue-126-work-command\n"),
		"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte("origin/issue-126-work-command\n"),
		"gh pr create --repo StatPan/gira --title feat: Work command --body Closes #126 --base main --draft": []byte("https://github.com/StatPan/gira/pull/200\n"),
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
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[]`),
		"git branch --show-current":                                                                  []byte("issue-126-work-command\n"),
		"git rev-parse --abbrev-ref --symbolic-full-name @{u}":                                       []byte("origin/issue-126-work-command\n"),
		"gh pr create --repo StatPan/gira --title feat: Work command --body Closes #126 --base main": []byte("https://github.com/StatPan/gira/pull/201\n"),
		"gh api repos/StatPan/gira/issues/126/labels/status:in-progress -X DELETE":                   nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-review":           nil,
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
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[]`),
		"git branch --show-current":                            []byte("issue-126-work-command\n"),
		"git rev-parse --abbrev-ref --symbolic-full-name @{u}": []byte("origin/issue-126-work-command\n"),
		"gh pr create --repo StatPan/gira --title feat: Work command --body Closes #126 --base release/2.0": []byte("https://github.com/StatPan/gira/pull/201\n"),
		"gh api repos/StatPan/gira/issues/126/labels/status:in-progress -X DELETE":                          nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-review":                  nil,
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
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	body := "## Goal\nShip status contract\n\n## Scope\nTicket status JSON\n\n## Acceptance Criteria\n- exposes PR state\n\n## Doctor Impact\nUpdates status JSON only.\n\n## Expected Evidence\n- go test ./internal/gira\n\n" + RenderTicketLifecycleBlock(TicketLifecycleState{BaseBranch: "main", BaseSource: "branch_policy.default", BranchPolicyMode: BranchPolicyModeGitHubFlow, WorkBranch: "issue-126-work-command"})
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","body":` + strconv.Quote(body) + `,"milestone":{"title":"2.0 Alpha"},"labels":[{"name":"type:task"},{"name":"status:in-review"},{"name":"priority:p1"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[
			{"number":201,"title":"feat: work","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/201","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-126-work-command","baseRefName":"main","statusCheckRollup":[{"name":"test","workflowName":"ci","status":"COMPLETED","conclusion":"SUCCESS","detailsUrl":"https://ci.example"}]}
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

func TestGetWorkStatusReportsBranchBaseMismatchWarning(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	body := RenderTicketLifecycleBlock(TicketLifecycleState{BaseBranch: "main", BaseSource: "branch_policy.default", BranchPolicyMode: BranchPolicyModeGitHubFlow})
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","body":` + strconv.Quote(body) + `,"labels":[{"name":"status:in-review"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[{"number":201,"title":"feat: work","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/201","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-126-work-command","baseRefName":"develop","statusCheckRollup":[]}]`),
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
}

func TestOpenWorkPRDryRunReportsMissingLinkedPR(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[]`),
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
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[]`),
		"git branch --show-current":                 []byte("issue-126-work-command\n"),
		"git push -u origin issue-126-work-command": nil,
		"gh pr create --repo StatPan/gira --title feat: Work command --body Closes #126 --base main --draft": []byte("https://github.com/StatPan/gira/pull/204\n"),
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
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[]`),
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
	pr := DevPRStatusResult{PRNumber: 201, State: "OPEN", Binding: DevPRBinding{HeadRef: "feat/i126-other", BaseRef: "main"}, Blockers: []string{"pr_binding"}}

	result := workStatusFromIssueAndPR(repo, 126, issue, pr)
	if result.Branch == nil || result.Branch.Expected != "feat/i126-work-command" || result.Branch.Trusted {
		t.Fatalf("unexpected status branch: %+v", result.Branch)
	}
	if result.BranchPolicy == nil || !containsString(result.BranchPolicy.Diagnostics, "recorded_work_branch_actual_pr_head_mismatch") {
		t.Fatalf("missing recorded branch mismatch diagnostic: %+v", result.BranchPolicy)
	}
}

func TestOpenWorkPRApplyPushesWhenUpstreamIsBaseBranch(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[]`),
		"git branch --show-current":                                                                  []byte("issue-126-work-command\n"),
		"git rev-parse --abbrev-ref --symbolic-full-name @{u}":                                       []byte("origin/main\n"),
		"git push -u origin issue-126-work-command":                                                  nil,
		"gh pr create --repo StatPan/gira --title feat: Work command --body Closes #126 --base main": []byte("https://github.com/StatPan/gira/pull/204\n"),
		"gh api repos/StatPan/gira/issues/126/labels/status:in-progress -X DELETE":                   nil,
		"gh api repos/StatPan/gira/issues/126/labels -X POST -f labels[]=status:in-review":           nil,
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
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[]`),
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
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[]`),
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

func TestOpenWorkPRApplyRejectsWrongBranch(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[]`),
		"git branch --show-current": []byte("feature/other\n"),
	}}

	_, err := OpenWorkPR(repo, 126, false, false, runner)
	if err == nil || !strings.Contains(err.Error(), "not the ticket branch") {
		t.Fatalf("expected wrong branch error, got %v", err)
	}
	if containsCall(runner.calls, "git push -u origin feature/other") {
		t.Fatalf("wrong branch should not be pushed, calls=%v", runner.calls)
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
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[{"number":202,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/202","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`),
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
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[{"number":202,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/202","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","headRefName":"issue-126-work-command","baseRefName":"develop","statusCheckRollup":[]}]`),
	}}

	result, err := OpenWorkPR(repo, 126, false, false, runner)
	if err == nil || !strings.Contains(err.Error(), "does not match recorded ticket base") {
		t.Fatalf("expected base mismatch error, got result=%+v err=%v", result, err)
	}
	if !result.BaseMismatch || result.RecordedBase != "main" || result.ActualBase != "develop" || !containsString(result.Blockers, "pr_base_mismatch") {
		t.Fatalf("missing mismatch result details: %+v", result)
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
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[{"number":202,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/202","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`),
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
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[{"number":202,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/202","reviewDecision":"","isDraft":true,"mergeStateStatus":"UNKNOWN","statusCheckRollup":[]}]`),
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

func TestGetWorkStatusReportsNextAction(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[{"number":203,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/203","reviewDecision":"","isDraft":true,"mergeStateStatus":"UNKNOWN","statusCheckRollup":[]}]`),
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
	prCall := "gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20"
	runner := &workRunner{
		outputs: map[string][]byte{
			issueCall: []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-progress"}]}`),
			prCall:    []byte(`[{"number":203,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/203","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`),
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
	restoreDelay := workStatusMissingPRRetryDelay
	workStatusMissingPRRetryDelay = 0
	t.Cleanup(func() {
		workStatusMissingPRRetryDelay = restoreDelay
	})
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	prCall := "gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20"
	runner := &workRunner{
		outputs: map[string][]byte{
			"gh api repos/StatPan/gira/issues/126": []byte(`{"number":126,"title":"Work command","state":"open","labels":[{"name":"status:in-review"}]}`),
		},
		queues: map[string][][]byte{
			prCall: {
				[]byte(`[]`),
				[]byte(`[{"number":203,"title":"x","body":"Closes #126","state":"OPEN","url":"https://github.com/StatPan/gira/pull/203","reviewDecision":"","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`),
			},
		},
	}

	result, err := GetWorkStatus(repo, 126, runner)
	if err != nil {
		t.Fatalf("GetWorkStatus error: %v", err)
	}
	if result.PRNumber != 203 || result.PRLookupAttempts != 2 || result.NextAction != "merge_when_policy_allows" {
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
	prCall := "gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20"
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
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 126 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[]`),
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
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 184 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[{"number":185,"title":"x","body":"Closes #184","state":"MERGED","url":"https://github.com/StatPan/gira/pull/185","reviewDecision":"","isDraft":false,"mergeStateStatus":"UNKNOWN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
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

func TestGetWorkStatusClosedIssueWithoutPRDoesNotSuggestStart(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &workRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/188": []byte(`{"number":188,"title":"Closed manually","state":"closed","labels":[]}`),
		"gh pr list --repo StatPan/gira --state all --search repo:StatPan/gira is:pr 188 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup,headRefName,baseRefName --limit 20": []byte(`[]`),
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
