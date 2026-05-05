package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/StatPan/gira/internal/gira"
)

func TestHelpOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "GitHub-native project OS bootstrapper") {
		t.Fatalf("help output missing product description:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "contract") {
		t.Fatalf("help output missing contract command:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "portfolio") {
		t.Fatalf("help output missing portfolio command:\n%s", stdout.String())
	}
}

func TestPortfolioPlanRequiresDryRun(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"portfolio", "plan"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--dry-run is required for portfolio plan") {
		t.Fatalf("stderr missing dry-run guidance:\n%s", stderr.String())
	}
}

func TestPortfolioCapabilityJSON(t *testing.T) {
	restore := newPortfolioCapabilityReport
	t.Cleanup(func() { newPortfolioCapabilityReport = restore })
	newPortfolioCapabilityReport = func(configPath string) (gira.PortfolioCapabilityReport, error) {
		if configPath != "testdata/portfolio.yaml" {
			t.Fatalf("unexpected config path: %s", configPath)
		}
		return gira.PortfolioCapabilityReport{
			Command:       "portfolio capability",
			PortfolioRepo: "StatPan/portfolio",
			Token:         gira.ProjectCapabilityTokenSummary{Kind: "pat", Identity: "alice"},
			Repos: []gira.PortfolioRepoCapability{{
				Repo: "StatPan/gira",
				Role: "execution",
				Mode: "write",
				Capabilities: map[string]gira.ProjectCapabilityStatus{
					"issues:read":  gira.ProjectCapabilityAllowed,
					"issues:write": gira.ProjectCapabilityAllowed,
				},
			}},
			FetchedAt: "2026-05-05T12:00:00Z",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"portfolio", "capability", "--config", "testdata/portfolio.yaml", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"command": "portfolio capability"`, `"portfolio_repo": "StatPan/portfolio"`, `"issues:write": "allowed"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("portfolio capability JSON missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestPortfolioCapabilityBlockedExitOne(t *testing.T) {
	restore := newPortfolioCapabilityReport
	t.Cleanup(func() { newPortfolioCapabilityReport = restore })
	newPortfolioCapabilityReport = func(configPath string) (gira.PortfolioCapabilityReport, error) {
		return gira.PortfolioCapabilityReport{
			Command:       "portfolio capability",
			PortfolioRepo: "StatPan/portfolio",
			Token:         gira.ProjectCapabilityTokenSummary{Kind: "pat", Identity: "alice"},
			Repos: []gira.PortfolioRepoCapability{{
				Repo: "StatPan/gira",
				Role: "execution",
				Mode: "read-only",
				Capabilities: map[string]gira.ProjectCapabilityStatus{
					"issues:read":  gira.ProjectCapabilityAllowed,
					"issues:write": gira.ProjectCapabilityDeniedScope,
				},
			}},
			BlockedActions: []gira.PortfolioCapabilityBlock{{CheckID: "execution:StatPan/gira:issues:write", Repo: "StatPan/gira", Role: "execution", Required: "issues:write", Reason: "token scope or repository permission is insufficient"}},
			FetchedAt:      "2026-05-05T12:00:00Z",
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"portfolio", "capability"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "blocked actions") {
		t.Fatalf("stdout missing blocked actions:\n%s", stdout.String())
	}
}

func TestPortfolioPlanJSON(t *testing.T) {
	restore := newPortfolioReport
	t.Cleanup(func() { newPortfolioReport = restore })
	newPortfolioReport = func(command string, configPath string, dryRun bool) (gira.PortfolioReport, error) {
		if command != "plan" || configPath != "testdata/portfolio.yaml" || !dryRun {
			t.Fatalf("unexpected portfolio args command=%s config=%s dryRun=%t", command, configPath, dryRun)
		}
		return gira.PortfolioReport{
			Command:       "portfolio plan",
			PortfolioRepo: "StatPan/portfolio",
			Repos:         []string{"StatPan/gira"},
			DryRun:        true,
			Counts:        gira.PortfolioCounts{Tickets: 1, OpenTickets: 1, Actions: 1},
			Actions:       []gira.PortfolioPlanAction{{Ticket: 140, Action: "execution_issue:create", Repo: "StatPan/gira", Reason: "no child issue linked for target repo"}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"portfolio", "plan", "--dry-run", "--config", "testdata/portfolio.yaml", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{`"portfolio_repo": "StatPan/portfolio"`, `"action": "execution_issue:create"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("portfolio JSON missing %q:\n%s", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "next step:") {
		t.Fatalf("portfolio JSON contains human prose:\n%s", stdout.String())
	}
}

func TestPortfolioValidateDiagnosticsExitOne(t *testing.T) {
	restore := newPortfolioReport
	t.Cleanup(func() { newPortfolioReport = restore })
	newPortfolioReport = func(command string, configPath string, dryRun bool) (gira.PortfolioReport, error) {
		if command != "validate" || dryRun {
			t.Fatalf("unexpected portfolio args command=%s dryRun=%t", command, dryRun)
		}
		return gira.PortfolioReport{
			Command:       "portfolio validate",
			PortfolioRepo: "StatPan/portfolio",
			Repos:         []string{"StatPan/gira"},
			Counts:        gira.PortfolioCounts{Tickets: 1, OpenTickets: 1, Diagnostics: 1},
			Diagnostics:   []gira.PortfolioDiagnostic{{Ticket: 140, RuleID: "missing_required_field", Detail: "goal is required"}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"portfolio", "validate"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ticket #140 missing_required_field") {
		t.Fatalf("stdout missing diagnostic:\n%s", stdout.String())
	}
}

func TestPortfolioValidateJSONDiagnosticsExitOne(t *testing.T) {
	restore := newPortfolioReport
	t.Cleanup(func() { newPortfolioReport = restore })
	newPortfolioReport = func(command string, configPath string, dryRun bool) (gira.PortfolioReport, error) {
		if command != "validate" || dryRun {
			t.Fatalf("unexpected portfolio args command=%s dryRun=%t", command, dryRun)
		}
		return gira.PortfolioReport{
			Command:       "portfolio validate",
			PortfolioRepo: "StatPan/portfolio",
			Repos:         []string{"StatPan/gira"},
			Counts:        gira.PortfolioCounts{Tickets: 1, OpenTickets: 1, Diagnostics: 1},
			Diagnostics:   []gira.PortfolioDiagnostic{{Ticket: 140, RuleID: "invalid_child_issue", Detail: "bad must be OWNER/REPO#N"}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"portfolio", "validate", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"rule_id": "invalid_child_issue"`) {
		t.Fatalf("stdout missing JSON diagnostic:\n%s", stdout.String())
	}
}

func TestTriageHelpFlagPrintsHelpAndExitsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"triage", "-h"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Backlog triage normalization helpers") {
		t.Fatalf("stdout missing triage help:\n%s", stdout.String())
	}
}

func TestWorkStartRequiresRepoIssueAndMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"work", "start", "--repo", "StatPan/gira", "--issue", "126"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "exactly one of --dry-run/--apply") {
		t.Fatalf("stderr missing mode guidance:\n%s", stderr.String())
	}
}

func TestWorkStartDryRunJSON(t *testing.T) {
	restore := newWorkStartResult
	t.Cleanup(func() { newWorkStartResult = restore })
	newWorkStartResult = func(repo gira.RepoRef, issue int, dryRun bool) (gira.WorkStartResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 126 || !dryRun {
			t.Fatalf("unexpected args repo=%s issue=%d dryRun=%t", repo.FullName(), issue, dryRun)
		}
		return gira.WorkStartResult{Repo: repo.FullName(), Issue: issue, Branch: "issue-126-work-command", DryRun: true, NextStatus: "In progress"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"work", "start", "--repo", "StatPan/gira", "--issue", "126", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"branch": "issue-126-work-command"`) {
		t.Fatalf("stdout missing branch JSON:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "next step:") {
		t.Fatalf("JSON stdout contains human prose:\n%s", stdout.String())
	}
}

func TestStartAliasWrapsWorkStart(t *testing.T) {
	restore := newWorkStartResult
	t.Cleanup(func() { newWorkStartResult = restore })
	newWorkStartResult = func(repo gira.RepoRef, issue int, dryRun bool) (gira.WorkStartResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 130 || dryRun {
			t.Fatalf("unexpected args repo=%s issue=%d dryRun=%t", repo.FullName(), issue, dryRun)
		}
		return gira.WorkStartResult{Repo: repo.FullName(), Issue: issue, Branch: "issue-130-next-step-aliases", NextStatus: "In progress"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"start", "--repo", "StatPan/gira", "--issue", "130", "--apply"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{
		"work start: issue #130",
		"next step: gira work pr --repo StatPan/gira --issue 130 --dry-run",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("start alias output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestStartAliasHelpMentionsAlias(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"start", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Alias: gira start") {
		t.Fatalf("help output missing start alias:\n%s", stdout.String())
	}
}

func TestSyncNextStepKeepsPolicyAndBootstrapFlags(t *testing.T) {
	repo := gira.RepoRef{Owner: "StatPan", Name: "gira"}
	got := syncNextStep(repo, true, true, gira.SyncPolicyAdopt, false)
	want := "next step: gira sync --repo StatPan/gira --policy-mode adopt --bootstrap-issues"
	if got != want {
		t.Fatalf("syncNextStep = %q, want %q", got, want)
	}
}

func TestWorkPRApplyDraftJSON(t *testing.T) {
	restore := newWorkPRResult
	t.Cleanup(func() { newWorkPRResult = restore })
	newWorkPRResult = func(repo gira.RepoRef, issue int, dryRun bool, draft bool) (gira.WorkPRResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 126 || dryRun || !draft {
			t.Fatalf("unexpected args repo=%s issue=%d dryRun=%t draft=%t", repo.FullName(), issue, dryRun, draft)
		}
		return gira.WorkPRResult{Repo: repo.FullName(), Issue: issue, Draft: true, PRNumber: 204, NextStatus: "In progress"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"work", "pr", "--repo", "StatPan/gira", "--issue", "126", "--apply", "--draft", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"draft": true`) {
		t.Fatalf("stdout missing draft JSON:\n%s", stdout.String())
	}
}

func TestWorkStatusJSON(t *testing.T) {
	restore := newWorkStatusResult
	t.Cleanup(func() { newWorkStatusResult = restore })
	newWorkStatusResult = func(repo gira.RepoRef, issue int) (gira.WorkStatusResult, error) {
		if repo.FullName() != "StatPan/gira" || issue != 126 {
			t.Fatalf("unexpected args repo=%s issue=%d", repo.FullName(), issue)
		}
		return gira.WorkStatusResult{Repo: repo.FullName(), Issue: issue, Status: "In progress", Blockers: []string{"draft"}, NextAction: "mark_pr_ready"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"work", "status", "--repo", "StatPan/gira", "--issue", "126", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"next_action": "mark_pr_ready"`) {
		t.Fatalf("stdout missing next action JSON:\n%s", stdout.String())
	}
}

func TestSprintRolloverMissingRepoReturnsTwo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"sprint", "rollover", "--dry-run"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--repo and exactly one of --dry-run/--apply are required") {
		t.Fatalf("stderr missing required-args message:\n%s", stderr.String())
	}
}

func TestTriageMissingRepoReturnsTwo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"triage", "--dry-run"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--repo and exactly one of --dry-run/--apply are required") {
		t.Fatalf("stderr missing required-args message:\n%s", stderr.String())
	}
}

func TestTriageDryRunApplyExclusivityReturnsTwo(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "neither", args: []string{"triage", "--repo", "StatPan/gira"}},
		{name: "both", args: []string{"triage", "--repo", "StatPan/gira", "--dry-run", "--apply"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tc.args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("exit code = %d, want 2", code)
			}
			if !strings.Contains(stderr.String(), "exactly one of --dry-run/--apply") {
				t.Fatalf("stderr missing exclusivity guidance:\n%s", stderr.String())
			}
		})
	}
}

func TestTriageDryRunValidInvocationReturnsZero(t *testing.T) {
	restore := newTriageReport
	t.Cleanup(func() { newTriageReport = restore })
	newTriageReport = func(repo gira.RepoRef, apply bool) (gira.TriageNormalizeReport, error) {
		if repo.FullName() != "StatPan/gira" {
			t.Fatalf("repo = %s, want StatPan/gira", repo.FullName())
		}
		if apply {
			t.Fatalf("apply = true, want false for dry-run")
		}
		return gira.TriageNormalizeReport{Repo: repo.FullName(), Mode: "dry-run"}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"triage", "--repo", "StatPan/gira", "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"repo\": \"StatPan/gira\"") {
		t.Fatalf("stdout missing triage JSON payload:\n%s", stdout.String())
	}
}

func TestContractCRUDMatrix(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"contract", "crud"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"CRUD capability matrix (MVP contract)",
		"surface               create                                           read                                                                                     update                                                            delete",
		"labels                gira sync --repo OWNER/REPO                      gira sync --repo OWNER/REPO --dry-run                                                    gira sync --repo OWNER/REPO                                       unsupported (intentional in MVP)",
		"milestones            gira sync --repo OWNER/REPO                      gira sync --repo OWNER/REPO --dry-run                                                    gira sync --repo OWNER/REPO                                       unsupported (intentional in MVP)",
		"issues                gira sync --repo OWNER/REPO --bootstrap-issues   gira status --repo OWNER/REPO                                                            gira triage apply --apply / gira worker claim|handoff|release     unsupported direct delete in MVP",
		"pr_loop               gira dev pr open --repo OWNER/REPO --issue N     gira dev pr status --repo OWNER/REPO --issue N / gira review queue                        gira merge queue --apply (opt-in destructive)                     unsupported direct delete; close via GitHub UI/API",
		"project_fields_views  unsupported (MVP non-goal)                       gira project capability / gira project sync --dry-run / gira project transitions --dry-run unsupported in MVP (dry-run inspection only)                      unsupported (MVP non-goal)",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("contract output missing %q:\n%s", want, output)
		}
	}
}

func TestContractUnsupportedOperationGuidance(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"contract", "delete"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "unsupported contract operation") {
		t.Fatalf("stderr missing unsupported guidance:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "supported operation: gira contract crud") {
		t.Fatalf("stderr missing supported operation guidance:\n%s", stderr.String())
	}
}

func TestBootstrapRequiresRepo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"bootstrap", "--dry-run"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "--repo is required") {
		t.Fatalf("stderr missing repo requirement:\n%s", stderr.String())
	}
}

func TestBootstrapNonDryRunRequiresPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"bootstrap", "--repo", "StatPan/example"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "--path is required") {
		t.Fatalf("stderr missing path requirement:\n%s", stderr.String())
	}
}

func TestBootstrapNonDryRunRequiresGitRepo(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"bootstrap",
		"--repo",
		"StatPan/example",
		"--path",
		dir,
		"--created-at",
		"2026-04-26",
	}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "not a git repository") {
		t.Fatalf("stderr missing git repo requirement:\n%s", stderr.String())
	}
}

func TestBootstrapNonDryRunCanInstallWithoutBranch(t *testing.T) {
	repo := t.TempDir()
	runCLIGit(t, repo, "init", "-q", "-b", "main")

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"bootstrap",
		"--repo",
		"StatPan/example",
		"--path",
		repo,
		"--no-branch",
		"--created-at",
		"2026-04-26",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "branch:") {
		t.Fatalf("stdout unexpectedly included branch line:\n%s", stdout.String())
	}
	if _, err := os.Stat(repo + "/AGENTS.md"); err != nil {
		t.Fatalf("AGENTS.md was not created: %v", err)
	}
}

func TestBootstrapDryRunIsDeterministic(t *testing.T) {
	args := []string{
		"bootstrap",
		"--repo",
		"StatPan/example",
		"--template",
		"default",
		"--dry-run",
		"--created-at",
		"2026-04-26",
	}

	var firstOut, firstErr, secondOut, secondErr bytes.Buffer
	firstCode := Run(args, &firstOut, &firstErr)
	secondCode := Run(args, &secondOut, &secondErr)

	if firstCode != 0 || secondCode != 0 {
		t.Fatalf("exit codes = %d/%d, want 0/0; stderr: %s%s", firstCode, secondCode, firstErr.String(), secondErr.String())
	}
	if firstOut.String() != secondOut.String() {
		t.Fatal("dry-run output differed between identical runs")
	}
	output := firstOut.String()
	for _, want := range []string{"--- AGENTS.md", "StatPan/example", "example", "2026-04-26", "--- .github/PULL_REQUEST_TEMPLATE.md"} {
		if !strings.Contains(output, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, output)
		}
	}
}

func TestStatusJSONUsesInjectedClient(t *testing.T) {
	restoreClient, restoreNow := newStatusClient, statusNow
	t.Cleanup(func() {
		newStatusClient = restoreClient
		statusNow = restoreNow
	})
	newStatusClient = func(repo gira.RepoRef) gira.StatusClient {
		return cliFakeStatusClient{
			repo: repo,
			responses: map[string]string{
				"api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100": `[[{"number":1,"title":"MVP","state":"open","description":"","due_on":null,"open_issues":1,"closed_issues":1}]]`,
				"api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100":     `[[{"number":1,"title":"Issue 1","state":"open","labels":[],"milestone":null,"updated_at":"2026-04-25T12:00:00Z","html_url":"https://github.com/StatPan/gira/issues/1"}]]`,
			},
		}
	}
	statusNow = func() time.Time {
		return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"status", "--repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("status JSON did not parse: %v\n%s", err, stdout.String())
	}
	if payload["repo"] != "StatPan/gira" {
		t.Fatalf("repo = %v, want StatPan/gira", payload["repo"])
	}
}

func TestOnboardVerifyRequiresRepoAndStage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"onboard", "verify", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "--stage is required") {
		t.Fatalf("stderr missing stage requirement:\n%s", stderr.String())
	}
}

func TestOnboardVerifyJSONUsesInjectedBuilder(t *testing.T) {
	restore := newOnboardVerifyReport
	t.Cleanup(func() { newOnboardVerifyReport = restore })
	newOnboardVerifyReport = func(repo gira.RepoRef, stage gira.OnboardStage) (gira.OnboardVerifyReport, error) {
		return gira.OnboardVerifyReport{
			Repo:      repo.FullName(),
			Command:   "onboard verify",
			Stage:     string(stage),
			CheckedAt: "2026-04-26T12:00:00Z",
			Ready:     false,
			BlockingChecklist: []gira.OnboardCheck{{
				ID:          "bootstrap_pr_template",
				Description: "bootstrap PR template is committed",
				Status:      gira.OnboardCheckFail,
				Remediation: "run gira bootstrap",
			}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"onboard", "verify", "--repo", "StatPan/gira", "--stage", "bootstrap", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"stage\": \"bootstrap\"") {
		t.Fatalf("onboard JSON missing stage: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "\"ready\": false") {
		t.Fatalf("onboard JSON missing ready=false: %s", stdout.String())
	}
}

func TestDoctorJSONUsesInjectedReportAndExitCode(t *testing.T) {
	restore := newDoctorReport
	t.Cleanup(func() { newDoctorReport = restore })
	newDoctorReport = func(repoValue string) gira.DoctorReport {
		if repoValue != "StatPan/gira" {
			t.Fatalf("repoValue = %q, want StatPan/gira", repoValue)
		}
		return gira.DoctorReport{
			Repo:      "StatPan/gira",
			Command:   "doctor",
			CheckedAt: "2026-05-05T12:00:00Z",
			Ready:     false,
			Checks: []gira.DoctorCheck{{
				ID:          "metadata_drift",
				Status:      gira.DoctorCheckFail,
				Detail:      "labels create=1 update=0; milestones create=0 update=0; bootstrap issues create=0",
				Remediation: "run `gira sync --repo StatPan/gira --bootstrap-issues --dry-run`, then apply with `gira sync --repo StatPan/gira --bootstrap-issues`",
			}},
		}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"doctor", "--repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"id\": \"metadata_drift\"") {
		t.Fatalf("doctor JSON missing check id:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "\"remediation\": \"run `gira sync --repo StatPan/gira --bootstrap-issues --dry-run`") {
		t.Fatalf("doctor JSON missing remediation:\n%s", stdout.String())
	}
}

func TestDoctorHelpFlagPrintsHelpAndExitsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"doctor", "-h"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "gira doctor [--repo OWNER/REPO] [--json]") {
		t.Fatalf("stdout missing doctor help:\n%s", stdout.String())
	}
}

func TestSyncDryRunUsesInjectedClientWithoutApplying(t *testing.T) {
	restoreClient := newSyncClient
	t.Cleanup(func() {
		newSyncClient = restoreClient
	})
	client := &cliFakeSyncClient{
		repo: mustCLIRepo(t, "StatPan/gira"),
		labels: []gira.ExistingLabel{
			{Name: gira.BootstrapLabel, Color: "5319E7", Description: "Created or managed by Gira bootstrap metadata sync."},
		},
	}
	newSyncClient = func(repo gira.RepoRef) gira.SyncClient {
		client.repo = repo
		return client
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"sync", "--repo", "StatPan/gira", "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"sync plan:", "would create", "bootstrap issues: 0 would create", "create label:"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("sync dry-run output missing %q:\n%s", want, stdout.String())
		}
	}
	if len(client.calls) != 0 {
		t.Fatalf("dry-run applied calls: %v", client.calls)
	}
}

func TestSyncDryRunWithBootstrapIssuesFlagIncludesIssueCreates(t *testing.T) {
	restoreClient := newSyncClient
	t.Cleanup(func() {
		newSyncClient = restoreClient
	})
	client := &cliFakeSyncClient{
		repo: mustCLIRepo(t, "StatPan/gira"),
		labels: []gira.ExistingLabel{
			{Name: gira.BootstrapLabel, Color: "5319E7", Description: "Created or managed by Gira bootstrap metadata sync."},
		},
	}
	newSyncClient = func(repo gira.RepoRef) gira.SyncClient {
		client.repo = repo
		return client
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"sync", "--repo", "StatPan/gira", "--dry-run", "--bootstrap-issues"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"sync plan:", "bootstrap issues:", "create issue:"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("sync dry-run output missing %q:\n%s", want, stdout.String())
		}
	}
	if len(client.calls) != 0 {
		t.Fatalf("dry-run applied calls: %v", client.calls)
	}
}

func TestSyncPolicyModeAdoptApplyPerformsNoMutations(t *testing.T) {
	restoreClient := newSyncClient
	t.Cleanup(func() {
		newSyncClient = restoreClient
	})
	client := &cliFakeSyncClient{repo: mustCLIRepo(t, "StatPan/gira")}
	newSyncClient = func(repo gira.RepoRef) gira.SyncClient {
		client.repo = repo
		return client
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"sync", "--repo", "StatPan/gira", "--policy-mode", "adopt"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "policy mode:      adopt") {
		t.Fatalf("sync output missing adopt policy mode:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "sync complete") {
		t.Fatalf("sync apply output missing completion:\n%s", stdout.String())
	}
	if len(client.calls) != 0 {
		t.Fatalf("adopt mode should not mutate, calls=%v", client.calls)
	}
}

func TestSyncDryRunNextStepPinsExplicitMergeWhenEnvEnforce(t *testing.T) {
	t.Setenv("GIRA_SYNC_POLICY_MODE", "enforce")
	restoreClient := newSyncClient
	t.Cleanup(func() {
		newSyncClient = restoreClient
	})
	client := &cliFakeSyncClient{repo: mustCLIRepo(t, "StatPan/gira")}
	newSyncClient = func(repo gira.RepoRef) gira.SyncClient {
		client.repo = repo
		return client
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"sync", "--repo", "StatPan/gira", "--dry-run", "--policy-mode", "merge"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "policy mode:      merge") {
		t.Fatalf("sync output missing reviewed merge policy mode:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "next step: gira sync --repo StatPan/gira --policy-mode merge") {
		t.Fatalf("next step must pin explicit merge policy mode:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "next step: gira sync --repo StatPan/gira --policy-mode enforce") {
		t.Fatalf("next step would apply env enforce policy:\n%s", stdout.String())
	}
	if len(client.calls) != 0 {
		t.Fatalf("dry-run applied calls: %v", client.calls)
	}
}

func TestSyncPolicyModeRejectsInvalidValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"sync", "--repo", "StatPan/gira", "--dry-run", "--policy-mode", "bad"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "invalid sync policy mode") {
		t.Fatalf("stderr missing invalid policy mode error:\n%s", stderr.String())
	}
}

func TestSyncApplyUsesInjectedClientAndPrintsComplete(t *testing.T) {
	restoreClient := newSyncClient
	t.Cleanup(func() {
		newSyncClient = restoreClient
	})
	client := &cliFakeSyncClient{
		repo: mustCLIRepo(t, "StatPan/gira"),
		labels: []gira.ExistingLabel{
			{Name: gira.BootstrapLabel, Color: "000000", Description: "Old description."},
		},
		milestones: []gira.ExistingMilestone{
			{Number: 1, Title: "MVP", Description: "CLI-first Gira bootstrapper with templates and GitHub metadata sync."},
			{Number: 2, Title: "Beta", Description: "Broader validation and hardening after the MVP workflow is usable."},
			{Number: 3, Title: "v1", Description: "Stable first release of the GitHub-native project OS workflow."},
		},
		issues: []gira.ExistingIssue{
			{Number: 1, Title: "[Epic] Gira MVP: GitHub-as-OS bootstrap", Labels: []string{gira.BootstrapLabel}},
			{Number: 2, Title: "[Task] Slice 1: CLI skeleton + template dry-run", Labels: []string{gira.BootstrapLabel}},
			{Number: 3, Title: "[Task] Slice 2: idempotent repo file install", Labels: []string{gira.BootstrapLabel}},
			{Number: 4, Title: "[Task] Slice 3: labels/milestones/bootstrap-issues sync", Labels: []string{gira.BootstrapLabel}},
			{Number: 5, Title: "[Task] Slice 4: gira status (text + --json)", Labels: []string{gira.BootstrapLabel}},
		},
	}
	newSyncClient = func(repo gira.RepoRef) gira.SyncClient {
		client.repo = repo
		return client
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"sync", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "sync complete") {
		t.Fatalf("sync apply output missing completion:\n%s", stdout.String())
	}
	if len(client.calls) == 0 || client.calls[0] != "update label "+gira.BootstrapLabel {
		t.Fatalf("apply calls = %v, want first call to update bootstrap label", client.calls)
	}
}

func TestExportDashboardCommandRequiresRepo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"export", "dashboard", "--dry-run"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "--repo is required") {
		t.Fatalf("stderr missing repo requirement:\n%s", stderr.String())
	}
}

func TestExportDashboardCommandRejectsInvalidRepoFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"export", "dashboard", "--repo", "bad-format", "--dry-run"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "OWNER/REPO") {
		t.Fatalf("stderr missing parse error for invalid repo:\n%s", stderr.String())
	}
}

func TestExportDashboardCommandRejectsPositionalArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"export", "dashboard", "--repo", "StatPan/gira", "--dry-run", "unexpected"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "unexpected argument") {
		t.Fatalf("stderr missing unexpected argument error:\n%s", stderr.String())
	}
}

func TestExportDashboardDryRunRequiresDeterministicOutput(t *testing.T) {
	restoreClient, restoreNow := newDashboardExportClient, dashboardExportNow
	t.Cleanup(func() {
		newDashboardExportClient = restoreClient
		dashboardExportNow = restoreNow
	})
	client := &cliFakeDashboardExportClient{
		repo: mustCLIRepo(t, "StatPan/gira"),
		issues: []gira.DashboardRawIssue{
			{IssueNumber: 2, Title: "Second", State: "open", Labels: []string{"status:ready"}},
			{IssueNumber: 1, Title: "First", State: "open", Labels: []string{"status:blocked"}},
		},
		pulls: []gira.DashboardRawPullRequest{
			{PullRequestNumber: 11, Title: "PR", State: "closed", Labels: []string{"priority:p1"}},
		},
		milestones: []gira.DashboardRawMilestone{
			{MilestoneNumber: 3, Title: "Later", State: "closed"},
			{MilestoneNumber: 1, Title: "First", State: "open"},
		},
		projectSync: gira.ProjectSyncSnapshot{
			RoadmapItems: []gira.ProjectRoadmapItem{
				{IssueNumber: 10, IssueTitle: "Roadmap", TypeLabel: "type:epic", IssueURL: "https://github.com/StatPan/gira/issues/10", Roadmapable: true, StartDate: strPtr("2026-04-10"), TargetDate: strPtr("2026-04-20")},
				{IssueNumber: 9, IssueTitle: "Another", TypeLabel: "type:task", IssueURL: "https://github.com/StatPan/gira/issues/9", Roadmapable: false},
			},
		},
		capabilities: gira.ProjectCapabilityReport{
			Capabilities:   map[string]gira.ProjectCapabilityStatus{},
			BlockedActions: []gira.ProjectCapabilityBlock{},
		},
	}
	newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient {
		client.repo = repo
		return client
	}
	dashboardExportNow = func() time.Time {
		return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	}

	var firstOut, firstErr, secondOut, secondErr bytes.Buffer
	firstCode := Run([]string{"export", "dashboard", "--repo", "StatPan/gira", "--dry-run"}, &firstOut, &firstErr)
	secondCode := Run([]string{"export", "dashboard", "--repo", "StatPan/gira", "--dry-run"}, &secondOut, &secondErr)

	if firstCode != 0 || secondCode != 0 {
		t.Fatalf("exit codes = %d/%d, want 0/0; stderr: %s%s", firstCode, secondCode, firstErr.String(), secondErr.String())
	}
	if firstOut.String() != secondOut.String() {
		t.Fatalf("dry-run output changed between identical runs")
	}
	if !strings.Contains(firstOut.String(), "export dashboard plan:") {
		t.Fatalf("dry-run output missing plan header:\n%s", firstOut.String())
	}
}

func TestExportDashboardJSONOnlyStdout(t *testing.T) {
	restoreClient, restoreNow := newDashboardExportClient, dashboardExportNow
	t.Cleanup(func() {
		newDashboardExportClient = restoreClient
		dashboardExportNow = restoreNow
	})
	client := &cliFakeDashboardExportClient{
		repo: mustCLIRepo(t, "StatPan/gira"),
		capabilities: gira.ProjectCapabilityReport{
			Capabilities:   map[string]gira.ProjectCapabilityStatus{},
			BlockedActions: []gira.ProjectCapabilityBlock{},
		},
	}
	newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient {
		client.repo = repo
		return client
	}
	dashboardExportNow = func() time.Time {
		return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"export",
		"dashboard",
		"--repo",
		"StatPan/gira",
		"--dry-run",
		"--json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr not empty in json mode:\n%s", stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("JSON output parse failure: %v\n%s", err, stdout.String())
	}
	if payload["command"] != "export dashboard" {
		t.Fatalf("payload missing command field: %v", payload["command"])
	}
}

func TestExportDashboardApplyWritesArtifactsAndJsonOnlyStdout(t *testing.T) {
	restoreClient, restoreNow := newDashboardExportClient, dashboardExportNow
	t.Cleanup(func() {
		newDashboardExportClient = restoreClient
		dashboardExportNow = restoreNow
	})
	client := &cliFakeDashboardExportClient{
		repo: mustCLIRepo(t, "StatPan/gira"),
		milestones: []gira.DashboardRawMilestone{
			{MilestoneNumber: 1, Title: "MVP", State: "open"},
		},
		capabilities: gira.ProjectCapabilityReport{
			Capabilities: map[string]gira.ProjectCapabilityStatus{
				"issues:read": "allowed",
			},
		},
	}
	newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient {
		client.repo = repo
		return client
	}
	dashboardExportNow = func() time.Time {
		return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	}

	outputRoot := filepath.Join(t.TempDir(), "dashboard-output")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"export",
		"dashboard",
		"--repo",
		"StatPan/gira",
		"--output",
		outputRoot,
		"--json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr not empty in json mode:\n%s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"command\": \"export dashboard\"") {
		t.Fatalf("stdout missing plan JSON:\n%s", stdout.String())
	}

	expected := []string{
		"manifest.json",
		"raw/github.json",
		"raw/transitions.json",
		"raw/capabilities.json",
		"derived/execution_board.json",
		"derived/roadmap_timeline.json",
		"derived/warnings.json",
		"csv/execution_items.csv",
		"csv/roadmap_items.csv",
	}
	for _, relativePath := range expected {
		if _, err := os.Stat(filepath.Join(outputRoot, relativePath)); err != nil {
			t.Fatalf("expected exported file %q: %v", relativePath, err)
		}
	}
}

func TestExportDashboardApplyRejectsOutputFilePath(t *testing.T) {
	restoreClient := newDashboardExportClient
	t.Cleanup(func() { newDashboardExportClient = restoreClient })
	file, err := os.CreateTemp(t.TempDir(), "dashboard-output")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient {
		return &cliFakeDashboardExportClient{repo: repo}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"export",
		"dashboard",
		"--repo",
		"StatPan/gira",
		"--output",
		file.Name(),
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "output path exists but is not a directory") {
		t.Fatalf("stderr missing output path type error:\n%s", stderr.String())
	}
}

func TestStatusTextUsesInjectedClient(t *testing.T) {
	restoreClient, restoreNow := newStatusClient, statusNow
	t.Cleanup(func() {
		newStatusClient = restoreClient
		statusNow = restoreNow
	})
	newStatusClient = func(repo gira.RepoRef) gira.StatusClient {
		return cliFakeStatusClient{
			repo: repo,
			responses: map[string]string{
				"api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100": `[]`,
				"api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100":     `[[{"number":1,"title":"Issue 1","state":"open","labels":[{"name":"status:blocked"}],"milestone":null,"updated_at":"2026-04-01T12:00:00Z","html_url":"https://github.com/StatPan/gira/issues/1"}]]`,
			},
		}
	}
	statusNow = func() time.Time {
		return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"status", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"status: StatPan/gira", "milestone progress: none", "stale open issues: 1", "blocked issues: 1"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("status output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestStatusInfersRepoFromGitOrigin(t *testing.T) {
	restoreClient, restoreRunner, restoreNow := newStatusClient, repoContextRunner, statusNow
	t.Cleanup(func() {
		newStatusClient = restoreClient
		repoContextRunner = restoreRunner
		statusNow = restoreNow
	})
	repoContextRunner = devCLIRunner{outputs: map[string][]byte{
		"git remote get-url origin": []byte("git@github.com:StatPan/gira.git\n"),
	}}
	newStatusClient = func(repo gira.RepoRef) gira.StatusClient {
		if repo.FullName() != "StatPan/gira" {
			t.Fatalf("repo = %s, want StatPan/gira", repo.FullName())
		}
		return cliFakeStatusClient{
			repo: repo,
			responses: map[string]string{
				"api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100": `[]`,
				"api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100":     `[]`,
			},
		}
	}
	statusNow = func() time.Time { return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC) }

	var stdout, stderr bytes.Buffer
	code := Run([]string{"status", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"repo": "StatPan/gira"`) {
		t.Fatalf("stdout missing inferred repo:\n%s", stdout.String())
	}
}

func TestStatusRepoOverrideWinsOverContext(t *testing.T) {
	restoreClient, restoreRunner, restoreNow := newStatusClient, repoContextRunner, statusNow
	t.Cleanup(func() {
		newStatusClient = restoreClient
		repoContextRunner = restoreRunner
		statusNow = restoreNow
	})
	repoContextRunner = devCLIRunner{errs: map[string]error{
		"git remote get-url origin": fmt.Errorf("repo context runner should not be called"),
	}}
	newStatusClient = func(repo gira.RepoRef) gira.StatusClient {
		if repo.FullName() != "StatPan/override" {
			t.Fatalf("repo = %s, want StatPan/override", repo.FullName())
		}
		return cliFakeStatusClient{
			repo: repo,
			responses: map[string]string{
				"api repos/StatPan/override/milestones --paginate --slurp -X GET -f state=all -f per_page=100": `[]`,
				"api repos/StatPan/override/issues --paginate --slurp -X GET -f state=all -f per_page=100":     `[]`,
			},
		}
	}
	statusNow = func() time.Time { return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC) }

	var stdout, stderr bytes.Buffer
	code := Run([]string{"status", "--repo", "StatPan/override", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"repo": "StatPan/override"`) {
		t.Fatalf("stdout missing override repo:\n%s", stdout.String())
	}
}

func TestStatusMissingRepoContextReturnsRemediation(t *testing.T) {
	restoreRunner := repoContextRunner
	t.Cleanup(func() { repoContextRunner = restoreRunner })
	repoContextRunner = devCLIRunner{errs: map[string]error{
		"git remote get-url origin": fmt.Errorf("exit status 2"),
	}}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"status"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "pass --repo OWNER/REPO") {
		t.Fatalf("stderr missing repo context remediation:\n%s", stderr.String())
	}
}

func TestOnboardVerifyInfersRepoFromConfig(t *testing.T) {
	restoreBuilder, restoreRunner := newOnboardVerifyReport, repoContextRunner
	t.Cleanup(func() {
		newOnboardVerifyReport = restoreBuilder
		repoContextRunner = restoreRunner
	})
	withCLIRepoConfig(t, "StatPan/configured")
	repoContextRunner = devCLIRunner{errs: map[string]error{
		"git remote get-url origin": fmt.Errorf("config should win before origin"),
	}}
	newOnboardVerifyReport = func(repo gira.RepoRef, stage gira.OnboardStage) (gira.OnboardVerifyReport, error) {
		if repo.FullName() != "StatPan/configured" {
			t.Fatalf("repo = %s, want StatPan/configured", repo.FullName())
		}
		return gira.OnboardVerifyReport{Repo: repo.FullName(), Command: "onboard verify", Stage: string(stage), Ready: true}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"onboard", "verify", "--stage", "init", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"repo": "StatPan/configured"`) {
		t.Fatalf("stdout missing configured repo:\n%s", stdout.String())
	}
}

func TestOnboardVerifyRepoOverrideWinsOverConfig(t *testing.T) {
	restoreBuilder, restoreRunner := newOnboardVerifyReport, repoContextRunner
	t.Cleanup(func() {
		newOnboardVerifyReport = restoreBuilder
		repoContextRunner = restoreRunner
	})
	withCLIRepoConfig(t, "StatPan/configured")
	repoContextRunner = devCLIRunner{errs: map[string]error{
		"git remote get-url origin": fmt.Errorf("override should not call origin"),
	}}
	newOnboardVerifyReport = func(repo gira.RepoRef, stage gira.OnboardStage) (gira.OnboardVerifyReport, error) {
		if repo.FullName() != "StatPan/override" {
			t.Fatalf("repo = %s, want StatPan/override", repo.FullName())
		}
		return gira.OnboardVerifyReport{Repo: repo.FullName(), Command: "onboard verify", Stage: string(stage), Ready: true}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"onboard", "verify", "--repo", "StatPan/override", "--stage", "init", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"repo": "StatPan/override"`) {
		t.Fatalf("stdout missing override repo:\n%s", stdout.String())
	}
}

func TestReportWeeklyInfersRepoFromConfig(t *testing.T) {
	restoreDash, restoreReview, restoreRunner, restoreNow := newDashboardExportClient, newReviewGateClient, repoContextRunner, reportNow
	t.Cleanup(func() {
		newDashboardExportClient = restoreDash
		newReviewGateClient = restoreReview
		repoContextRunner = restoreRunner
		reportNow = restoreNow
	})
	withCLIRepoConfig(t, "StatPan/configured")
	repoContextRunner = devCLIRunner{errs: map[string]error{
		"git remote get-url origin": fmt.Errorf("config should win before origin"),
	}}
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	reportNow = func() time.Time { return now }
	newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient {
		if repo.FullName() != "StatPan/configured" {
			t.Fatalf("dashboard repo = %s, want StatPan/configured", repo.FullName())
		}
		return weeklyDashClient{repo: repo}
	}
	newReviewGateClient = func(repo gira.RepoRef) gira.ReviewGateClient {
		if repo.FullName() != "StatPan/configured" {
			t.Fatalf("review repo = %s, want StatPan/configured", repo.FullName())
		}
		return weeklyReviewClient{repo: repo}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"report", "weekly", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"repo": "StatPan/configured"`) {
		t.Fatalf("stdout missing configured repo:\n%s", stdout.String())
	}
}

func TestReportWeeklyRepoOverrideWinsOverConfig(t *testing.T) {
	restoreDash, restoreReview, restoreRunner, restoreNow := newDashboardExportClient, newReviewGateClient, repoContextRunner, reportNow
	t.Cleanup(func() {
		newDashboardExportClient = restoreDash
		newReviewGateClient = restoreReview
		repoContextRunner = restoreRunner
		reportNow = restoreNow
	})
	withCLIRepoConfig(t, "StatPan/configured")
	repoContextRunner = devCLIRunner{errs: map[string]error{
		"git remote get-url origin": fmt.Errorf("override should not call origin"),
	}}
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	reportNow = func() time.Time { return now }
	newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient {
		if repo.FullName() != "StatPan/override" {
			t.Fatalf("dashboard repo = %s, want StatPan/override", repo.FullName())
		}
		return weeklyDashClient{repo: repo}
	}
	newReviewGateClient = func(repo gira.RepoRef) gira.ReviewGateClient {
		if repo.FullName() != "StatPan/override" {
			t.Fatalf("review repo = %s, want StatPan/override", repo.FullName())
		}
		return weeklyReviewClient{repo: repo}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"report", "weekly", "--repo", "StatPan/override", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"repo": "StatPan/override"`) {
		t.Fatalf("stdout missing override repo:\n%s", stdout.String())
	}
}

func withCLIRepoConfig(t *testing.T, repo string) {
	t.Helper()
	dir := t.TempDir()
	giraDir := filepath.Join(dir, ".gira")
	if err := os.MkdirAll(giraDir, 0o755); err != nil {
		t.Fatalf("mkdir .gira: %v", err)
	}
	if err := os.WriteFile(filepath.Join(giraDir, "config.yaml"), []byte("repo: "+repo+"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
}

type cliFakeStatusClient struct {
	repo      gira.RepoRef
	responses map[string]string
}

type cliFakeSyncClient struct {
	repo       gira.RepoRef
	labels     []gira.ExistingLabel
	milestones []gira.ExistingMilestone
	issues     []gira.ExistingIssue
	calls      []string
}

type cliFakeDashboardExportClient struct {
	repo         gira.RepoRef
	issues       []gira.DashboardRawIssue
	pulls        []gira.DashboardRawPullRequest
	milestones   []gira.DashboardRawMilestone
	projectSync  gira.ProjectSyncSnapshot
	transitions  gira.ProjectTransitionSnapshot
	capabilities gira.ProjectCapabilityReport
}

func (c *cliFakeDashboardExportClient) Repo() gira.RepoRef { return c.repo }

func (c *cliFakeDashboardExportClient) FetchIssues() ([]gira.DashboardRawIssue, error) {
	return c.issues, nil
}

func (c *cliFakeDashboardExportClient) FetchPullRequests() ([]gira.DashboardRawPullRequest, error) {
	return c.pulls, nil
}

func (c *cliFakeDashboardExportClient) FetchMilestones() ([]gira.DashboardRawMilestone, error) {
	return c.milestones, nil
}

func (c *cliFakeDashboardExportClient) FetchProjectSnapshot() (gira.ProjectSyncSnapshot, error) {
	return c.projectSync, nil
}

func (c *cliFakeDashboardExportClient) FetchTransitionSnapshot() (gira.ProjectTransitionSnapshot, error) {
	return c.transitions, nil
}

func (c *cliFakeDashboardExportClient) FetchCapabilities() (gira.ProjectCapabilityReport, error) {
	return c.capabilities, nil
}

func strPtr(value string) *string {
	return &value
}

func (c *cliFakeSyncClient) Repo() gira.RepoRef { return c.repo }

func (c *cliFakeSyncClient) ListLabels() ([]gira.ExistingLabel, error) {
	return c.labels, nil
}

func (c *cliFakeSyncClient) CreateLabel(label gira.LabelDef) error {
	c.calls = append(c.calls, "create label "+label.Name)
	return nil
}

func (c *cliFakeSyncClient) UpdateLabel(label gira.LabelDef) error {
	c.calls = append(c.calls, "update label "+label.Name)
	return nil
}

func (c *cliFakeSyncClient) ListMilestones() ([]gira.ExistingMilestone, error) {
	return c.milestones, nil
}

func (c *cliFakeSyncClient) CreateMilestone(milestone gira.MilestoneDef) error {
	c.calls = append(c.calls, "create milestone "+milestone.Title)
	return nil
}

func (c *cliFakeSyncClient) UpdateMilestone(number int, milestone gira.MilestoneDef) error {
	c.calls = append(c.calls, "update milestone "+milestone.Title)
	return nil
}

func (c *cliFakeSyncClient) ListBootstrapIssues() ([]gira.ExistingIssue, error) {
	return c.issues, nil
}

func (c *cliFakeSyncClient) CreateIssue(issue gira.BootstrapIssueDef) error {
	c.calls = append(c.calls, "create issue "+issue.Title)
	return nil
}

func mustCLIRepo(t *testing.T, value string) gira.RepoRef {
	t.Helper()
	repo, err := gira.ParseRepoRef(value)
	if err != nil {
		t.Fatalf("ParseRepoRef returned error: %v", err)
	}
	return repo
}

func runCLIGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	if _, err := (gira.ExecCommandRunner{}).Run("git", append([]string{"-C", repo}, args...)...); err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
}

func (c cliFakeStatusClient) Repo() gira.RepoRef {
	return c.repo
}

func (c cliFakeStatusClient) JSON(args []string, target any) error {
	key := strings.Join(args, " ")
	return json.Unmarshal([]byte(c.responses[key]), target)
}

func TestProjectCapabilityCommandRequiresRepo(t *testing.T) {
	restore := newProjectCapabilityReport
	t.Cleanup(func() { newProjectCapabilityReport = restore })
	newProjectCapabilityReport = func(repo gira.RepoRef) (gira.ProjectCapabilityReport, error) {
		report := gira.ProjectCapabilityReport{
			Repo:    repo.FullName(),
			Command: "project capability",
			Mode:    "write",
			Token:   gira.ProjectCapabilityTokenSummary{Kind: "pat", Identity: "alice"},
			Capabilities: map[string]gira.ProjectCapabilityStatus{
				"issues:read": "allowed",
			},
			BlockedActions: []gira.ProjectCapabilityBlock{},
		}
		return report, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"project", "capability", "--repo", "StatPan/example", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"repo\": \"StatPan/example\"") {
		t.Fatalf("project capability JSON missing repo: %s", stdout.String())
	}
}

func TestProjectCapabilityCommandNeedsSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"project", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "project capability") {
		t.Fatalf("project help output unexpected: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "project sync") {
		t.Fatalf("project help output missing sync command: %s", stdout.String())
	}
}

func TestParityJiraCommandJSONUsesInjectedBuilder(t *testing.T) {
	restore := newJiraParityReport
	t.Cleanup(func() { newJiraParityReport = restore })
	newJiraParityReport = func(repo gira.RepoRef) (gira.JiraParityReport, error) {
		return gira.JiraParityReport{
			Repo:    repo.FullName(),
			Command: "parity jira",
			Scores:  gira.JiraParityScores{Earned: 80, Total: 100, Pct: 80},
			Domains: []gira.JiraParityDomain{{Name: "visibility", Weight: 15, Pass: true, Evidence: []string{"gira status"}}},
			Ready:   false,
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"parity", "jira", "--repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"command\": \"parity jira\"") {
		t.Fatalf("parity jira JSON missing command: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "\"percent\": 80") {
		t.Fatalf("parity jira JSON missing percent score: %s", stdout.String())
	}
}

func TestProjectSyncCommandRequiresDryRunOrApply(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"project", "sync", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "exactly one of --dry-run or --apply is required") {
		t.Fatalf("stderr missing dry-run/apply requirement:\n%s", stderr.String())
	}
}

func TestProjectSyncCommandJSONUsesInjectedBuilder(t *testing.T) {
	restore := newProjectSyncReport
	t.Cleanup(func() { newProjectSyncReport = restore })
	newProjectSyncReport = func(repo gira.RepoRef, dryRun bool) (gira.ProjectSyncReport, error) {
		report := gira.ProjectSyncReport{
			Repo:           repo.FullName(),
			Command:        "project sync",
			Project:        "Product OS",
			DryRun:         dryRun,
			MissingProject: true,
			Counts: gira.ProjectSyncCounts{
				FieldsMissing: 6,
			},
		}
		return report, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"project", "sync", "--repo", "StatPan/gira", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"command\": \"project sync\"") {
		t.Fatalf("project sync JSON missing command: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "\"dry_run\": true") {
		t.Fatalf("project sync JSON missing dry_run true: %s", stdout.String())
	}
}

func TestProjectTransitionsCommandRequiresDryRunOrApply(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"project", "transitions", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "exactly one of --dry-run or --apply is required") {
		t.Fatalf("stderr missing dry-run/apply requirement:\n%s", stderr.String())
	}
}

func TestProjectTransitionsCommandJSONUsesInjectedBuilder(t *testing.T) {
	restore := newProjectTransitionsReport
	t.Cleanup(func() { newProjectTransitionsReport = restore })
	newProjectTransitionsReport = func(repo gira.RepoRef, dryRun bool) (gira.ProjectTransitionsReport, error) {
		return gira.ProjectTransitionsReport{
			Repo:    repo.FullName(),
			Command: "project transitions",
			DryRun:  dryRun,
			Counts: gira.ProjectTransitionCounts{
				Applied: 1,
			},
			Transitions: []gira.ProjectTransitionPlanItem{
				{RuleID: "issue_open_default", TargetType: "issue", TargetID: "10", Decision: "apply", From: "null", To: "Backlog"},
			},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"project", "transitions", "--repo", "StatPan/gira", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"command\": \"project transitions\"") {
		t.Fatalf("project transitions JSON missing command: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "\"dry_run\": true") {
		t.Fatalf("project transitions JSON missing dry_run true: %s", stdout.String())
	}
}

func TestProjectSyncApplyCommandJSONUsesInjectedBuilder(t *testing.T) {
	restore := newProjectSyncApplyReport
	t.Cleanup(func() { newProjectSyncApplyReport = restore })
	newProjectSyncApplyReport = func(repo gira.RepoRef) (gira.ProjectSyncApplyReport, error) {
		return gira.ProjectSyncApplyReport{
			Repo:    repo.FullName(),
			Command: "project sync",
			DryRun:  false,
			Applied: []gira.ProjectSyncApplyAction{{Action: "project_status_field:update", Required: "projectsv2:write", Result: "planned"}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"project", "sync", "--repo", "StatPan/gira", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"dry_run\": false") {
		t.Fatalf("project sync apply JSON missing dry_run false: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "\"action\": \"project_status_field:update\"") {
		t.Fatalf("project sync apply JSON missing action: %s", stdout.String())
	}
}

func TestAuditVerifyScopesToRepoInCLI(t *testing.T) {
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "StatPan_gira.jsonl")
	otherPath := filepath.Join(dir, "OtherOrg_other.jsonl")
	if err := gira.AppendAuditRecords(repoPath, []gira.AuditRecord{gira.NewAuditRecord("sync", "sha256:a", "label:create", "issue#1", "ok", "", "allowed", time.Now())}); err != nil {
		t.Fatalf("append repo audit: %v", err)
	}
	if err := gira.AppendAuditRecords(otherPath, []gira.AuditRecord{gira.NewAuditRecord("sync", "sha256:b", "label:create", "issue#2", "ok", "", "allowed", time.Now())}); err != nil {
		t.Fatalf("append other audit: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"audit", "verify", "--repo", "StatPan/gira", "--path", filepath.Join(dir, "*.jsonl")}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "audit verify: ok (1 records)") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestAuditVerifyFailsForMismatchedRepoPath(t *testing.T) {
	dir := t.TempDir()
	otherPath := filepath.Join(dir, "OtherOrg_other.jsonl")
	if err := gira.AppendAuditRecords(otherPath, []gira.AuditRecord{gira.NewAuditRecord("sync", "sha256:b", "label:create", "issue#2", "ok", "", "allowed", time.Now())}); err != nil {
		t.Fatalf("append other audit: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"audit", "verify", "--repo", "StatPan/gira", "--path", otherPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout: %s stderr: %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "audit verify: failed (no_audit_files_found)") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestApplyPathsFailClosedWhenAuditWriteFails(t *testing.T) {
	restoreSyncClient := newSyncClient
	restoreProjectSync := newProjectSyncApplyReport
	restoreTransitions := newProjectTransitionsApplyReport
	t.Cleanup(func() {
		newSyncClient = restoreSyncClient
		newProjectSyncApplyReport = restoreProjectSync
		newProjectTransitionsApplyReport = restoreTransitions
	})

	blockAuditWrites(t)

	syncClient := &cliFakeSyncClient{repo: mustCLIRepo(t, "StatPan/gira")}
	newSyncClient = func(repo gira.RepoRef) gira.SyncClient {
		syncClient.repo = repo
		return syncClient
	}
	newProjectSyncApplyReport = func(repo gira.RepoRef) (gira.ProjectSyncApplyReport, error) {
		return gira.ProjectSyncApplyReport{Repo: repo.FullName(), Command: "project sync", DryRun: false}, nil
	}
	newProjectTransitionsApplyReport = func(repo gira.RepoRef) (gira.ProjectTransitionsApplyReport, error) {
		return gira.ProjectTransitionsApplyReport{Repo: repo.FullName(), Command: "project transitions", DryRun: false}, nil
	}

	cases := [][]string{
		{"sync", "--repo", "StatPan/gira"},
		{"project", "sync", "--repo", "StatPan/gira", "--apply"},
		{"project", "transitions", "--repo", "StatPan/gira", "--apply"},
	}
	for _, args := range cases {
		var stdout, stderr bytes.Buffer
		code := Run(args, &stdout, &stderr)
		if code == 0 {
			t.Fatalf("expected non-zero for %v", args)
		}
		if !strings.Contains(stderr.String(), "audit write failed") {
			t.Fatalf("expected audit failure for %v; stderr: %s", args, stderr.String())
		}
	}
}

func blockAuditWrites(t *testing.T) {
	t.Helper()
	base := filepath.Join(os.TempDir(), "gira-audit-test")
	_ = os.RemoveAll(base)
	if err := os.WriteFile(base, []byte("block"), 0o644); err != nil {
		t.Fatalf("prepare audit block file: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Remove(base)
	})
}

func TestGuardrailsSyncCommandJSONUsesInjectedBuilder(t *testing.T) {
	restore := newGuardrailsSyncReport
	t.Cleanup(func() { newGuardrailsSyncReport = restore })
	newGuardrailsSyncReport = func(repo gira.RepoRef, policyPath string, apply bool, allowRelaxation bool) (gira.GuardrailsSyncReport, error) {
		return gira.GuardrailsSyncReport{Repo: repo.FullName(), BlockedCount: 1}, nil
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"guardrails", "sync", "--repo", "StatPan/gira", "--policy", ".gira/guardrails.yaml", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"blocked_count\": 1") {
		t.Fatalf("missing blocked_count: %s", stdout.String())
	}
}

func TestGuardrailsSyncRequiresMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"guardrails", "sync", "--repo", "StatPan/gira", "--policy", ".gira/guardrails.yaml"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero")
	}
	if !strings.Contains(stderr.String(), "exactly one of --dry-run or --apply is required") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestGuardrailsSyncApplyDeniedPermissionFailsClosed(t *testing.T) {
	restoreCapability := newProjectCapabilityReport
	t.Cleanup(func() { newProjectCapabilityReport = restoreCapability })
	newProjectCapabilityReport = func(repo gira.RepoRef) (gira.ProjectCapabilityReport, error) {
		return gira.ProjectCapabilityReport{
			Repo: repo.FullName(),
			Capabilities: map[string]gira.ProjectCapabilityStatus{
				"repo:settings:write": gira.ProjectCapabilityDeniedScope,
			},
			BlockedActions: []gira.ProjectCapabilityBlock{{
				Action: "repo:settings:write",
				Reason: "token scope or repository permission is insufficient",
			}},
		}, nil
	}

	dir := t.TempDir()
	policyPath := filepath.Join(dir, "guardrails.yaml")
	if err := os.WriteFile(policyPath, []byte(`branch_protection:
  main:
    required_approving_review_count: 2
    require_code_owner_reviews: true
    required_status_checks_strict: true
    allow_force_pushes: false
    allow_deletions: false
`), 0o644); err != nil {
		t.Fatalf("write policy: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"guardrails", "sync", "--repo", "StatPan/gira", "--policy", policyPath, "--apply", "--json"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit for denied permission")
	}
	if !strings.Contains(stderr.String(), "repo:settings:write denied") {
		t.Fatalf("missing explicit denied reason: %s", stderr.String())
	}
}

type devCLIRunner struct {
	outputs map[string][]byte
	errs    map[string]error
}

func (r devCLIRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	if err, ok := r.errs[key]; ok {
		return nil, err
	}
	if out, ok := r.outputs[key]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected call: %s", key)
}

func TestDevStartJSONDryRun(t *testing.T) {
	original := devCommandRunner
	t.Cleanup(func() { devCommandRunner = original })
	devCommandRunner = devCLIRunner{
		outputs: map[string][]byte{
			"gh api repos/StatPan/gira/issues/59": []byte(`{"number":59,"title":"Start branch","state":"open","labels":[{"name":"status:ready"}]}`),
		},
		errs: map[string]error{
			"git show-ref --verify --quiet refs/heads/issue-59-start-branch": fmt.Errorf("exit status 1"),
			"git ls-remote --exit-code --heads origin issue-59-start-branch": fmt.Errorf("exit status 2"),
		},
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"dev", "start", "--repo", "StatPan/gira", "--issue", "59", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"branch": "issue-59-start-branch"`) {
		t.Fatalf("missing branch in output: %s", stdout.String())
	}
}

func TestDevStartJSONReusesLocalBranch(t *testing.T) {
	original := devCommandRunner
	t.Cleanup(func() { devCommandRunner = original })
	devCommandRunner = devCLIRunner{
		outputs: map[string][]byte{
			"gh api repos/StatPan/gira/issues/59":                            []byte(`{"number":59,"title":"Start branch","state":"open","labels":[{"name":"status:ready"}]}`),
			"git show-ref --verify --quiet refs/heads/issue-59-start-branch": nil,
			"git ls-remote --exit-code --heads origin issue-59-start-branch": []byte("abc\trefs/heads/issue-59-start-branch"),
			"git checkout issue-59-start-branch":                             nil,
		},
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"dev", "start", "--repo", "StatPan/gira", "--issue", "59", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"created": false`) {
		t.Fatalf("expected created=false for local branch reuse: %s", stdout.String())
	}
}

func TestDevPROpenJSON(t *testing.T) {
	original := devCommandRunner
	t.Cleanup(func() { devCommandRunner = original })
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues/60":                                          []byte(`{"number":60,"title":"Add PR loop","state":"open","labels":[{"name":"status:ready"}]}`),
		"gh pr create --repo StatPan/gira --title feat: Add PR loop --body Closes #60": []byte("https://github.com/StatPan/gira/pull/99\n"),
	}}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"dev", "pr", "open", "--repo", "StatPan/gira", "--issue", "60", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"pr_number": 99`) {
		t.Fatalf("missing pr_number: %s", stdout.String())
	}
}

func TestDevPRStatusJSON(t *testing.T) {
	original := devCommandRunner
	t.Cleanup(func() { devCommandRunner = original })
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"gh pr list --repo StatPan/gira --search repo:StatPan/gira is:pr 60 --json number,title,body,state,url,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup --limit 20": []byte(`[{"number":99,"title":"x","body":"Closes #60","state":"OPEN","url":"u","reviewDecision":"APPROVED","isDraft":false,"mergeStateStatus":"CLEAN","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`),
	}}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"dev", "pr", "status", "--repo", "StatPan/gira", "--issue", "60", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"ready": true`) {
		t.Fatalf("expected ready true: %s", stdout.String())
	}
}

func TestInitRequiresRepo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"init", "--json"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero without --repo")
	}
	if !strings.Contains(stderr.String(), "--repo is required") {
		t.Fatalf("missing repo error: %s", stderr.String())
	}
}

func TestInitJSONReady(t *testing.T) {
	original := devCommandRunner
	t.Cleanup(func() { devCommandRunner = original })
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"gh --version":                                 []byte("gh version 2"),
		"git --version":                                []byte("git version 2"),
		"gh auth status":                               []byte("ok"),
		"gh repo view StatPan/gira --json name":        []byte(`{"name":"gira"}`),
		"git -C /repo rev-parse --is-inside-work-tree": []byte("true"),
		"git -C /repo diff --quiet":                    nil,
		"git -C /repo diff --cached --quiet":           nil,
	}}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"init", "--repo", "StatPan/gira", "--path", "/repo", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"ready": true`) {
		t.Fatalf("expected ready true: %s", stdout.String())
	}
}

func TestInitReadsConfig(t *testing.T) {
	original := devCommandRunner
	t.Cleanup(func() { devCommandRunner = original })
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"gh --version":                                 []byte("gh version 2"),
		"git --version":                                []byte("git version 2"),
		"gh auth status":                               []byte("ok"),
		"gh repo view StatPan/gira --json name":        []byte(`{"name":"gira"}`),
		"git -C /repo rev-parse --is-inside-work-tree": []byte("true"),
		"git -C /repo diff --quiet":                    nil,
		"git -C /repo diff --cached --quiet":           nil,
	}}
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`profiles:
  default:
    labels: ["type:task"]
    review_policy:
      required_approvals: 1
`), 0o644)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"init", "--repo", "StatPan/gira", "--path", "/repo", "--config", configPath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"ready": true`) {
		t.Fatalf("expected ready true: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"config_path":`) {
		t.Fatalf("expected config path in output: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"config_profile_count": 1`) {
		t.Fatalf("expected config profile count in output: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `config profile \"default\": labels=1 milestones=0 issue_templates=0 approvals=1 codeowners=false`) {
		t.Fatalf("expected config profile plan details in output: %s", stdout.String())
	}
}

func TestInitUsesDefaultConfigPathWhenPresent(t *testing.T) {
	original := devCommandRunner
	t.Cleanup(func() { devCommandRunner = original })
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"gh --version":                                 []byte("gh version 2"),
		"git --version":                                []byte("git version 2"),
		"gh auth status":                               []byte("ok"),
		"gh repo view StatPan/gira --json name":        []byte(`{"name":"gira"}`),
		"git -C /repo rev-parse --is-inside-work-tree": []byte("true"),
		"git -C /repo diff --quiet":                    nil,
		"git -C /repo diff --cached --quiet":           nil,
	}}

	dir := t.TempDir()
	giraDir := filepath.Join(dir, ".gira")
	if err := os.MkdirAll(giraDir, 0o755); err != nil {
		t.Fatalf("mkdir .gira: %v", err)
	}
	configPath := filepath.Join(giraDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`profiles:
  default:
    labels: ["type:task"]
`), 0o644)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir tempdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	var stdout, stderr bytes.Buffer
	code := Run([]string{"init", "--repo", "StatPan/gira", "--path", "/repo", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"config_path": ".gira/config.yaml"`) {
		t.Fatalf("expected default config path in output: %s", stdout.String())
	}
}

func TestInitUsesWorkspaceDefaultConfigPathWhenPresent(t *testing.T) {
	original := devCommandRunner
	t.Cleanup(func() { devCommandRunner = original })

	workspace := t.TempDir()
	devCommandRunner = devCLIRunner{outputs: map[string][]byte{
		"gh --version":                          []byte("gh version 2"),
		"git --version":                         []byte("git version 2"),
		"gh auth status":                        []byte("ok"),
		"gh repo view StatPan/gira --json name": []byte(`{"name":"gira"}`),
		"git -C " + workspace + " rev-parse --is-inside-work-tree": []byte("true"),
		"git -C " + workspace + " diff --quiet":                    nil,
		"git -C " + workspace + " diff --cached --quiet":           nil,
	}}
	giraDir := filepath.Join(workspace, ".gira")
	if err := os.MkdirAll(giraDir, 0o755); err != nil {
		t.Fatalf("mkdir .gira: %v", err)
	}
	configPath := filepath.Join(giraDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`profiles:
  default:
    labels: ["type:task"]
`), 0o644)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"init", "--repo", "StatPan/gira", "--path", workspace, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"config_path": `) || !strings.Contains(stdout.String(), filepath.ToSlash(configPath)) {
		t.Fatalf("expected workspace config path in output: %s", stdout.String())
	}
}

func TestInitInvalidConfigFails(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`profiles:
  default:
    review_policy:
      required_approvals: -1
`), 0o644)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"init", "--repo", "StatPan/gira", "--config", configPath}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero for invalid config")
	}
	if !strings.Contains(stderr.String(), "required_approvals") {
		t.Fatalf("expected actionable config error: %s", stderr.String())
	}
}

func TestReportWeeklyJSON(t *testing.T) {
	restoreDash := newDashboardExportClient
	restoreReview := newReviewGateClient
	restoreNow := reportNow
	t.Cleanup(func() {
		newDashboardExportClient = restoreDash
		newReviewGateClient = restoreReview
		reportNow = restoreNow
	})
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	reportNow = func() time.Time { return now }
	newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient {
		return weeklyDashClient{repo: repo, issues: []gira.DashboardRawIssue{{IssueNumber: 70, Title: "Blocked", State: "open", Labels: []string{"blocked"}, UpdatedAt: now.Add(-15 * 24 * time.Hour).Format(time.RFC3339), URL: "https://example/issues/70"}}}
	}
	newReviewGateClient = func(repo gira.RepoRef) gira.ReviewGateClient {
		return weeklyReviewClient{repo: repo, prs: []gira.ReviewPR{{Number: 77, Title: "Wait", URL: "https://example/pr/77", UpdatedAt: now.Add(-72 * time.Hour).Format(time.RFC3339), RequestedReviewers: []string{"alice"}}}, issues: []gira.ReviewIssue{{Number: 70, Labels: []string{"blocker"}}}}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"report", "weekly", "--repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"backlog_health": "amber"`) {
		t.Fatalf("missing backlog health in output: %s", stdout.String())
	}
}

func TestReviewGateFailsWhenChecksFail(t *testing.T) {
	original := reviewGateRunner
	t.Cleanup(func() { reviewGateRunner = original })
	reviewGateRunner = devCLIRunner{outputs: map[string][]byte{
		"gofmt -l .":          []byte(""),
		"go test ./...":       []byte(""),
		"go test -race ./...": []byte(""),
	}, errs: map[string]error{"go vet ./...": fmt.Errorf("vet failed")}}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"review", "gate", "--json"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero when checks fail")
	}
	if !strings.Contains(stdout.String(), `"ready": false`) {
		t.Fatalf("expected readiness false output: %s", stdout.String())
	}
}

type weeklyDashClient struct {
	repo   gira.RepoRef
	issues []gira.DashboardRawIssue
}

func (c weeklyDashClient) Repo() gira.RepoRef                             { return c.repo }
func (c weeklyDashClient) FetchIssues() ([]gira.DashboardRawIssue, error) { return c.issues, nil }
func (c weeklyDashClient) FetchPullRequests() ([]gira.DashboardRawPullRequest, error) {
	return nil, nil
}
func (c weeklyDashClient) FetchMilestones() ([]gira.DashboardRawMilestone, error) { return nil, nil }
func (c weeklyDashClient) FetchProjectSnapshot() (gira.ProjectSyncSnapshot, error) {
	return gira.ProjectSyncSnapshot{}, nil
}
func (c weeklyDashClient) FetchTransitionSnapshot() (gira.ProjectTransitionSnapshot, error) {
	return gira.ProjectTransitionSnapshot{}, nil
}
func (c weeklyDashClient) FetchCapabilities() (gira.ProjectCapabilityReport, error) {
	return gira.ProjectCapabilityReport{}, nil
}

type weeklyReviewClient struct {
	repo   gira.RepoRef
	prs    []gira.ReviewPR
	issues []gira.ReviewIssue
}

func (c weeklyReviewClient) Repo() gira.RepoRef                          { return c.repo }
func (c weeklyReviewClient) ListOpenPRs() ([]gira.ReviewPR, error)       { return c.prs, nil }
func (c weeklyReviewClient) ListOpenIssues() ([]gira.ReviewIssue, error) { return c.issues, nil }
func (c weeklyReviewClient) MergePR(number int) error                    { return nil }
