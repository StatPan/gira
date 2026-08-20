package gira

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func autoStartRunner(issue int, title string, current string, apply bool) *workRunner {
	outputs := map[string][]byte{
		fmt.Sprintf("gh api repos/StatPan/gira/issues/%d", issue): []byte(fmt.Sprintf(`{"number":%d,"title":%q,"state":"open","labels":[{"name":"status:ready"}]}`, issue, title)),
		"git branch --show-current":                               []byte(current + "\n"),
	}
	errs := map[string]error{
		"git show-ref --verify --quiet refs/heads/issue-956-automatic-branch": fmt.Errorf("exit status 1"),
		"git ls-remote --exit-code --heads origin issue-956-automatic-branch": fmt.Errorf("exit status 2"),
	}
	if apply {
		outputs[fmt.Sprintf("gh api repos/StatPan/gira/issues/%d/labels/status:ready -X DELETE", issue)] = nil
		outputs[fmt.Sprintf("gh api repos/StatPan/gira/issues/%d/labels -X POST -f labels[]=status:in-progress", issue)] = nil
		outputs[fmt.Sprintf("git checkout -b issue-%d-%s origin/main", issue, strings.ReplaceAll(strings.ToLower(title), " ", "-"))] = nil
	}
	return &workRunner{outputs: outputs, errs: errs}
}

func TestStartWorkAutoSelectsBaseNonBaseAndDetached(t *testing.T) {
	tests := []struct {
		name          string
		current       string
		dryRun        bool
		wantStrategy  string
		wantSource    string
		wantBranch    string
		wantCreated   bool
		wantCheckout  bool
		wantSelection string
	}{
		{name: "resolved base creates suggested branch", current: "main", dryRun: true, wantStrategy: "create", wantSource: "generated", wantBranch: "issue-956-automatic-branch"},
		{name: "non-base binds current", current: "release/fix", dryRun: false, wantStrategy: "current", wantSource: "current", wantBranch: "release/fix"},
		{name: "detached safely creates", current: "", dryRun: true, wantStrategy: "create", wantSource: "generated", wantBranch: "issue-956-automatic-branch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := autoStartRunner(956, "Automatic branch", tt.current, !tt.dryRun)
			result, err := StartWorkWithOptions(RepoRef{Owner: "StatPan", Name: "gira"}, 956, WorkStartOptions{DryRun: tt.dryRun}, runner)
			if err != nil {
				t.Fatalf("StartWorkWithOptions error: %v", err)
			}
			if result.Branch != tt.wantBranch || result.BranchStrategy != tt.wantStrategy || result.BranchSource != tt.wantSource || result.BranchSelection != "auto" {
				t.Fatalf("unexpected selection: %+v", result)
			}
			if tt.wantSource == "current" && containsCall(runner.calls, "git checkout") {
				t.Fatalf("auto non-base selection must not checkout: %v", runner.calls)
			}
			if tt.wantSource == "generated" && tt.dryRun && result.CreatedBranch {
				t.Fatalf("dry-run must not report a created branch: %+v", result)
			}
		})
	}
}

func TestStartWorkUnifiedBranchSelectionsAndSafety(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	for _, tt := range []struct {
		name     string
		branch   string
		current  string
		want     string
		source   string
		setLocal bool
	}{
		{name: "new", branch: "new", current: "main", want: "issue-956-automatic-branch", source: "generated"},
		{name: "current", branch: "current", current: "release/fix", want: "release/fix", source: "current"},
		{name: "named existing", branch: "team/work", current: "main", want: "team/work", source: "adopted", setLocal: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runner := autoStartRunner(956, "Automatic branch", tt.current, false)
			if tt.setLocal {
				runner.outputs["git show-ref --verify --quiet refs/heads/team/work"] = nil
			}
			result, err := StartWorkWithOptions(repo, 956, WorkStartOptions{DryRun: true, Branch: tt.branch}, runner)
			if err != nil {
				t.Fatalf("StartWorkWithOptions error: %v", err)
			}
			if result.Branch != tt.want || result.BranchStrategy == "selection-required" || result.BranchSource != tt.source || result.BranchSelection != tt.branch {
				t.Fatalf("unexpected result: %+v", result)
			}
			if !strings.Contains(result.Approval.ApplyCommand, "--branch "+tt.branch) {
				t.Fatalf("approval did not preserve unified selection: %+v", result.Approval)
			}
		})
	}

	runner := autoStartRunner(956, "Automatic branch", "main", false)
	_, err := StartWorkWithOptions(repo, 956, WorkStartOptions{DryRun: true, Branch: "current"}, runner)
	if err == nil || !strings.Contains(err.Error(), "resolved base branch") {
		t.Fatalf("base branch must never bind as work branch, error=%v", err)
	}

	runner = autoStartRunner(956, "Automatic branch", "main", false)
	_, err = StartWorkWithOptions(repo, 956, WorkStartOptions{DryRun: true, Branch: "auto", Create: true}, runner)
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") || len(runner.calls) != 0 {
		t.Fatalf("unified/legacy conflict must be rejected before fetch/mutation: err=%v calls=%v", err, runner.calls)
	}
}

func TestStartWorkBindingRejectsMismatchedLocalOriginBeforeMutation(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	for _, tt := range []struct {
		name    string
		branch  string
		current string
	}{
		{name: "auto base", branch: "auto", current: "main"},
		{name: "new", branch: "new", current: "main"},
		{name: "auto current", branch: "auto", current: "release/fix"},
		{name: "current", branch: "current", current: "release/fix"},
		{name: "named", branch: "team/work", current: "main"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runner := autoStartRunner(956, "Automatic branch", tt.current, false)
			runner.outputs["git remote get-url origin"] = []byte("git@github.com:Other/repo.git")
			if tt.branch == "team/work" {
				runner.outputs["git show-ref --verify --quiet refs/heads/team/work"] = nil
			}
			_, err := StartWorkWithOptions(repo, 956, WorkStartOptions{DryRun: true, Branch: tt.branch}, runner)
			if err == nil || !strings.Contains(err.Error(), "cannot bind work branch") {
				t.Fatalf("mismatched local origin must reject binding: err=%v", err)
			}
			if containsCallWith(runner.calls, "-X PATCH -f body=", "work_branch:") || containsCall(runner.calls, "gh api repos/StatPan/gira/issues/956/labels -X POST -f labels[]=status:in-progress") {
				t.Fatalf("origin mismatch must fail before lifecycle/status mutation: %v", runner.calls)
			}
		})
	}
}

func TestStartWorkAutoApprovalPinsEffectiveAction(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	for _, tt := range []struct {
		current string
		want    string
	}{
		{current: "main", want: "--branch new"},
		{current: "team/existing", want: "--branch current"},
		{current: "", want: "--branch new"},
	} {
		t.Run(tt.current, func(t *testing.T) {
			runner := autoStartRunner(956, "Automatic branch", tt.current, false)
			result, err := StartWorkWithOptions(repo, 956, WorkStartOptions{DryRun: true}, runner)
			if err != nil {
				t.Fatalf("dry-run error: %v", err)
			}
			if !strings.Contains(result.Approval.ApplyCommand, tt.want) {
				t.Fatalf("approval must pin effective auto action %q: %s", tt.want, result.Approval.ApplyCommand)
			}
		})
	}
}

func TestStartWorkExplicitPolicyStillRequiresSelectionWithBaseOverride(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".gira"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := "repo: StatPan/gira\nprofiles:\n  default:\n    labels: []\n    milestones: []\n    issue_templates: []\nbranch_policy:\n  mode: github-flow\n  start_mode: explicit\n"
	if err := os.WriteFile(filepath.Join(root, ".gira", "config.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	runner := autoStartRunner(956, "Automatic branch", "main", false)
	result, err := StartWorkWithOptions(RepoRef{Owner: "StatPan", Name: "gira"}, 956, WorkStartOptions{DryRun: true, BaseOverride: "release/2.0"}, runner)
	if err != nil || !result.SelectionRequired || result.BranchStrategy != "selection-required" {
		t.Fatalf("base override must not bypass explicit mode: result=%+v err=%v", result, err)
	}
}
