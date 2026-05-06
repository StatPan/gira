package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/StatPan/gira/internal/gira"
)

const rootHelp = `Gira: Jira-style project flow on GitHub.

Usage:
  gira <command> [flags]

Daily commands:
  guide       Built-in quickstart and workflow guides
  workspace   Personal workspace inbox and backlog overview
  projects    Sync visible GitHub Projects board items
  ticket      Jira-style ticket lifecycle commands
  sprint      Sprint iteration planning/start/close workflow
  release     Release readiness gate report
  status      Show a compact read-only GitHub status summary
  upgrade     Check latest release and print upgrade instructions
  version     Show Gira build version
  start       Shortcut for ticket start

Setup:
  init        One-command onboarding with prerequisite checks and next-step plan

Advanced:
  ops         Advanced setup, migration, policy, audit, and raw GitHub controls
  work        Compatibility alias for ticket lifecycle commands
  dev         Compatibility developer workflow helpers

Flags:
  -h, --help   Show help
  --version    Show Gira build version
`

const guideHelp = `Built-in Gira guides for installed CLI users.

Usage:
  gira guide [quickstart|ticket|agent|concepts]
  gira docs [quickstart|ticket|agent|concepts]

Topics:
  quickstart  First successful flow from auth to merged PR
  ticket      Daily ticket lifecycle commands
  agent       Minimal rules for AI/coding agents
  concepts    Jira terms mapped to Gira and GitHub

Start here:
  gira guide quickstart
`

const guideQuickstart = `Gira quickstart: first ticket to merged PR

1. Authenticate GitHub.
   gh auth status

2. Confirm repo state.
   gira status
   gira projects sync --config .gira/config.yaml --dry-run

3. Create and start a ticket.
   gira ticket new "TITLE" --goal "GOAL" --acceptance "item 1;item 2" --apply --start

4. Implement the bounded scope and verify locally.
   go test ./...

5. Open or reuse the linked PR.
   gira ticket pr --apply --draft

6. Watch readiness through Gira.
   gira ticket checks
   gira ticket wait --timeout 5m

7. Finish the ticket.
   gira ticket finish --apply
   gira ticket status
`

const guideTicket = `Gira ticket guide

Daily loop:
  gira ticket new "TITLE" --goal "GOAL" --acceptance "a;b;c" --apply --start
  gira ticket pr --apply --draft
  gira ticket checks
  gira ticket wait --timeout 5m
  gira ticket finish --apply

Existing GitHub issue:
  gira ticket start 42 --apply
  gira ticket pr --apply --draft
  gira ticket finish --apply

Context rules:
  After ticket start checks out issue-N-*, ticket pr/checks/wait/finish/status infer the ticket.
  Use --repo OWNER/REPO and --ticket N only when outside a repo or branch context.

Safety:
  Use --dry-run before mutating commands when unsure.
  PR bodies must contain Closes #N, Fixes #N, or Resolves #N.
`

const guideAgent = `Gira agent runbook

Rules:
  Start from a GitHub issue.
  Use a feature branch per issue.
  Prefer Gira commands over raw gh when a Gira command exists.
  Keep changes bounded to the ticket.
  Run tests before PR and before finish.

Flow:
  gira status
  gira ticket new "TITLE" --goal "GOAL" --acceptance "a;b;c" --apply --start
  go test ./...
  gira ticket pr --apply --draft
  gira ticket checks
  gira ticket wait --timeout 5m
  gira ticket finish --apply
  gira projects sync --config .gira/config.yaml --dry-run

Do not:
  Do not call raw gh pr checks when gira ticket checks/wait exists.
  Do not merge without Gira finish unless explicitly instructed.
  Do not change unrelated files or revert user changes.
`

const guideConcepts = `Gira concepts: Jira terms on GitHub

Ticket:
  GitHub issue with Gira labels and structured body.

Epic:
  Top-level or parent issue for a milestone-sized outcome.

Story/task/bug/spike:
  GitHub issue type labels such as type:story, type:task, type:bug, type:spike.

Sprint or release phase:
  GitHub milestone.

Workflow status:
  status:* labels and mirrored GitHub Project Status field when projects sync is configured.

Branch:
  Work-start evidence, usually issue-N-title.

Pull request:
  Change unit. Must link back with Closes #N, Fixes #N, or Resolves #N.

Project board:
  Visibility bridge. Issues, labels, milestones, and PRs remain the source of truth.
`

const bootstrapHelp = `Bootstrap a repository into a Gira-managed project workspace.

Usage:
  gira bootstrap --repo OWNER/REPO --template default --dry-run [--created-at YYYY-MM-DD]
  gira bootstrap --repo OWNER/REPO --path PATH [--overwrite] [--branch BRANCH|--no-branch]

Flags:
  --repo string        Target GitHub repo in OWNER/REPO format
  --template string   Template name to render (default "default")
  --dry-run           Render without writing files or calling GitHub
  --path string        Local target git repo path (required for non-dry-run)
  --overwrite          Overwrite existing files that differ
  --branch string      Branch to create/checkout before install (default "chore/gira-bootstrap")
  --no-branch          Skip branch creation/checkout
  --created-at string Override render date for deterministic tests
  -h, --help          Show help
`

const initHelp = `One-command onboarding with prerequisite checks and fail-closed planning.

Usage:
  gira init --repo OWNER/REPO [--path .] [--config PATH] [--dry-run] [--json]

Flags:
  --repo string       Target GitHub repo in OWNER/REPO format
  --path string       Local git workspace path to validate (default ".")
  --config string     Optional init profile schema path (.gira/config.yaml|.gira/config.toml)
  --dry-run           Emit plan only (default true for this planning slice)
  --json              Emit stable JSON report for automation
  -h, --help          Show help
`

const statusHelp = `Show a compact read-only status summary from GitHub issues and milestones.

Usage:
  gira status [--repo OWNER/REPO] [--json] [--stale-days N]

Flags:
  --repo string       Target GitHub repo in OWNER/REPO format (default: infer from .gira/config.yaml or git origin)
  --json              Emit stable JSON for automation
  --stale-days int    Days since update before open issues count as stale (default 14)
  -h, --help          Show help
`

const workspaceHelp = `Personal workspace inbox and backlog commands.

Usage:
  gira workspace status [--config .gira/config.yaml] [--json]
  gira workspace backlog [--config .gira/config.yaml] [--json]
  gira workspace sync --dry-run|--apply [--config .gira/config.yaml] [--bootstrap-issues] [--json]
  gira workspace ticket new --title TEXT [--body TEXT] [--config .gira/config.yaml] [--json]
  gira workspace ticket route --ticket N --repo OWNER/REPO --dry-run|--apply [--config .gira/config.yaml] [--json]
  gira workspace project plan [--config .gira/config.yaml] [--json]

Commands:
  status   Show inbox and repo execution state in one Jira-like overview
  backlog  List inbox tickets and repo issues together
  sync     Sync Gira metadata across inbox and execution repos
  ticket   Create or route repo-agnostic inbox tickets
  project  Read-only GitHub Projects v2 visibility planning

Flags:
  --config string  Workspace config path (default ".gira/config.yaml")
  --json           Emit stable JSON output
  -h, --help       Show help
`

const projectsHelp = `Sync visible GitHub Projects board items.

Usage:
  gira projects sync --dry-run|--apply [--config .gira/config.yaml] [--archive-closed] [--json]

Commands:
  sync  Add missing workspace issues to the configured GitHub Project

Flags:
  --config string    Workspace config path (default ".gira/config.yaml")
  --archive-closed   Archive Project items whose backing issues are closed
  --json             Emit stable JSON output
  -h, --help       Show help
`

const versionHelp = `Show Gira build version.

Usage:
  gira version [--json]
  gira --version

Flags:
  --json      Emit stable JSON version info
  -h, --help  Show help
`

const upgradeHelp = `Check latest release and print safe upgrade instructions.

Usage:
  gira upgrade [--channel auto|install.sh|pipx|pip|homebrew|npm|bun|go|unknown] [--json]
  gira update [--channel auto|install.sh|pipx|pip|homebrew|npm|bun|go|unknown] [--json]

Flags:
  --channel string  Installed channel to use for the next-step command (default "auto")
  --json            Emit stable JSON upgrade info
  -h, --help        Show help
`

const onboardHelp = `Verify onboarding readiness from init to daily operation.

Usage:
  gira onboard verify [--repo OWNER/REPO] --stage init|bootstrap|first-sprint|steady-state [--json]

Commands:
  verify       Run prerequisite, bootstrap, metadata, and daily-run checks

Flags:
  --repo string   Target GitHub repo in OWNER/REPO format (default: infer from .gira/config.yaml or git origin)
  --stage string  Readiness stage to verify
  --json          Emit stable JSON readiness artifact
  -h, --help      Show help
`

const doctorHelp = `Diagnose install, auth, repo, drift, and local git readiness.

Usage:
  gira doctor [--repo OWNER/REPO] [--json]

Flags:
  --repo string   Target GitHub repo in OWNER/REPO format. Inferred from gh when omitted
  --json          Emit stable JSON report for automation
  -h, --help      Show help
`

const syncHelp = `Sync Gira labels, milestones, and optionally bootstrap issues through gh.

Usage:
  gira sync --repo OWNER/REPO [--dry-run] [--bootstrap-issues] [--policy-mode adopt|merge|enforce]

Flags:
  --repo string              Target GitHub repo in OWNER/REPO format
  --dry-run                  Plan sync without creating or updating GitHub metadata
  --bootstrap-issues         Enable creation of default Gira bootstrap issues
  --policy-mode string       Metadata policy mode (adopt|merge|enforce). Default: merge. Can also be set by GIRA_SYNC_POLICY_MODE
  -h, --help                 Show help
`

const detachHelp = `Detach Gira-managed repository artifacts safely.

Usage:
  gira detach --repo OWNER/REPO --dry-run|--apply [--json]

Flags:
  --repo string  Target GitHub repo in OWNER/REPO format
  --dry-run      Plan safe detach actions without mutation
  --apply        Apply only planned safe GitHub detach actions
  --json         Emit stable JSON output
  -h, --help     Show help
`

const projectHelp = `Project OS capability utilities for permission-aware automation.

Usage:
  gira project capability --repo OWNER/REPO [--json]
  gira project sync --repo OWNER/REPO --dry-run [--json]
  gira project sync --repo OWNER/REPO --apply [--json]
  gira project transitions --repo OWNER/REPO --dry-run [--json]
  gira project transitions --repo OWNER/REPO --apply [--json]

Commands:
  capability   Probe current token capabilities without applying any changes
  sync         Read-only dry-run inspection of Product OS project fields and roadmap dates
  transitions  Read-only dry-run lifecycle transition plan from documented rule matrix

Flags:
  --repo string  Target GitHub repo in OWNER/REPO format
  --dry-run      Required for this read-only slice
  --json         Emit stable JSON summary
  -h, --help     Show help
`

const exportHelp = `Export read-only dashboard artifacts from GitHub.

Usage:
  gira export dashboard --repo OWNER/REPO [--output PATH] [--dry-run] [--json]

Flags:
  --repo string       Target GitHub repo in OWNER/REPO format
  --output string     Output root directory for artifacts (default "./out/dashboard")
  --dry-run           Plan export without writing artifacts
  --json              Emit stable JSON summary
  -h, --help          Show help
`

const portfolioHelp = `Plan top-level portfolio intake lowering from a portfolio repo.

Usage:
  gira portfolio capability [--config .gira/config.yaml] [--json]
  gira portfolio status [--config .gira/config.yaml] [--json]
  gira portfolio validate [--config .gira/config.yaml] [--json]
  gira portfolio plan --dry-run [--config .gira/config.yaml] [--json]
  gira portfolio lower (--dry-run | --apply) [--config .gira/config.yaml] [--json]

Commands:
  capability  Probe repo issue permissions required for future lowering
  status    Summarize portfolio tickets and configured execution repos
  validate  Validate top-level ticket schema, routing, and links
  plan      Compute read-only lowering actions for top-level tickets
  lower     Create or link execution issues from portfolio tickets

Flags:
  --config string  Portfolio config path (default ".gira/config.yaml")
  --dry-run        Required for portfolio plan
  --json           Emit stable JSON output
  -h, --help       Show help
`

const auditHelp = `Audit utilities for append-only mutation ledger verification.

Usage:
  gira audit verify --repo OWNER/REPO --path .gira/audit/*.jsonl [--json]

Commands:
  verify       Validate JSONL schema and hash-chain integrity

Flags:
  --repo string  Target GitHub repo in OWNER/REPO format
  --path string  Glob path to audit JSONL files (default ".gira/audit/*.jsonl")
  --json         Emit stable JSON summary
  -h, --help     Show help
`

const parityHelp = `Jira-parity scorecard for objective replacement readiness.

Usage:
  gira parity jira --repo OWNER/REPO [--json]

Commands:
  jira         Compute weighted parity report with domain evidence and blockers

Flags:
  --repo string  Target GitHub repo in OWNER/REPO format
  --json         Emit stable JSON summary
  -h, --help     Show help
`

const jiraHelp = `Jira import/export command family.

Usage:
  gira jira import --repo OWNER/REPO --source PATH --dry-run|--apply [--json]
  gira jira import --repo OWNER/REPO --api-base URL --project KEY --dry-run|--apply [--json]
  gira jira export --repo OWNER/REPO --output PATH [--json]

Commands:
  import      Import Jira CSV/JSON or read-only Jira API issues into GitHub issues
  export      Export GitHub issue state into Jira-friendly JSON and CSV artifacts

Flags:
  --repo string      Target GitHub repo in OWNER/REPO format
  --source string    CSV or JSON import source path
  --api-base string  Jira API base URL, for example https://example.atlassian.net
  --project string   Jira project key for API import
  --output string    Output directory for export artifacts
  --dry-run          Preview import without creating GitHub issues
  --apply            Create GitHub issues for non-duplicate Jira items
  --json             Emit stable JSON output
  -h, --help         Show help
`

const guardrailsHelp = `Audit and apply repository guardrails policy.

Usage:
  gira guardrails sync --repo OWNER/REPO --policy .gira/guardrails.yaml --dry-run [--json] [--allow-relaxation]
  gira guardrails sync --repo OWNER/REPO --policy .gira/guardrails.yaml --apply [--json] [--allow-relaxation]

Flags:
  --repo string          Target GitHub repo in OWNER/REPO format
  --policy string        Guardrails policy file path
  --dry-run              Compute deterministic full diff only
  --apply                Apply policy-owned settings only
  --json                 Emit stable JSON summary
  --allow-relaxation     Allow relaxation changes
  -h, --help             Show help
`

const triageHelp = `Backlog triage normalization helpers.

Usage:
  gira triage --repo OWNER/REPO --dry-run|--apply [--json]

Flags:
  --repo string  Target GitHub repo in OWNER/REPO format
  --dry-run      Preview planned label additions only
  --apply        Apply missing axis labels to open issues
  --json         Emit stable JSON output
  -h, --help     Show help
`

const sprintHelp = `Sprint/iteration command family.

Usage:
  gira sprint plan --repo OWNER/REPO --iteration ID --capacity N --issues 1,2,3 --dry-run|--apply [--json]
  gira sprint start --repo OWNER/REPO --iteration ID --dry-run|--apply [--json]
  gira sprint close --repo OWNER/REPO --iteration ID --completed 1,2 --spillover-disposition carry|drop --rollover-reason TEXT --dry-run|--apply [--json]
  gira sprint rollover --repo OWNER/REPO [--to MILESTONE] --dry-run|--apply [--json]
`

const ticketHelp = `Jira-style ticket lifecycle commands.

Usage:
  gira ticket new "Title" --dry-run|--apply [--start] [--json]
  gira ticket start [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--json]
  gira ticket pr [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--draft] [--json]
  gira ticket checks [TICKET] [--repo OWNER/REPO] [--json]
  gira ticket wait [TICKET] [--repo OWNER/REPO] [--timeout 5m] [--interval 5s] [--json]
  gira ticket finish [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--wait 0s] [--json]
  gira ticket status [TICKET] [--repo OWNER/REPO] [--json]

Commands:
  new     Create a repo-bound executable ticket with a structured Gira body
  start   Verify a ready ticket, create/reuse its branch, and move to in-progress on apply. Alias: gira start
  pr      Validate or create a linked PR with Closes #N and update review status on apply
  checks  Show linked PR checks, review blockers, and next action
  wait    Wait for pending linked PR checks without merging
  finish  Mark the linked PR ready when needed, merge safely, and report convergence
  status  Report ticket status, linked PR blockers, and next action

Flags:
  --repo string    Target GitHub repo in OWNER/REPO format. Defaults to .gira config or git origin
  --ticket int     Ticket number. GitHub issue number in v1. Can also be positional
  --issue int      Compatibility alias for --ticket
  --dry-run        Preview without mutation
  --apply          Apply branch, PR, and status label changes
  --draft          Create/keep PR as draft for ticket pr
  --wait duration  Optional pending-check wait for ticket finish. Default: 0s
  --timeout duration  Pending-check wait timeout for ticket wait. Default: 5m
  --interval duration  Poll interval for ticket wait. Default: 5s
  --start          Start a newly created ticket after ticket new --apply
  --json           Emit stable JSON output
  -h, --help       Show help
`

const workerHelp = `Worker coordination commands for issue ownership and handoff payloads.

Usage:
  gira worker claim --repo OWNER/REPO --issue N --worker NAME [--lease-minutes 30]
  gira worker handoff --repo OWNER/REPO --issue N --goal TEXT --context TEXT --acceptance "a;b" --verify "cmd1;cmd2" --rollback TEXT
  gira worker release --repo OWNER/REPO --issue N --worker NAME
`

const graphHelp = `Work graph validation (parent/depends_on/blocks).

Usage:
  gira graph validate --repo OWNER/REPO [--json]
`

const reviewHelp = `Review/approval routing queue.

Usage:
  gira review queue [--repo OWNER/REPO] [--json]
  gira review gate [--json]
`

const mergeHelp = `Merge queue with policy checks.

Usage:
  gira merge queue --repo OWNER/REPO --dry-run|--apply [--json]
`

const releaseHelp = `Release readiness gate across approvals/checks/blockers/must-fix labels.

Usage:
  gira release readiness --repo OWNER/REPO [--json]
`

const reportHelp = `Weekly PM cockpit report with deterministic KPIs and top exceptions.

Usage:
  gira report weekly [--repo OWNER/REPO] [--json|--md]
`

const contractHelp = `CRUD capability matrix and command contract.

Usage:
  gira contract crud

Commands:
  crud         Show per-surface create/read/update/delete support contract

Notes:
  MVP-safe surfaces: labels, milestones, issues, PR loop, and project inspection.
  Destructive operations are opt-in: no broad delete/apply without explicit apply flags.
`

const devHelp = `Developer workflow helpers for issue-to-branch execution.

Usage:
  gira dev start --repo OWNER/REPO --issue N [--dry-run] [--json] [--force] [--branch-pattern "issue-%d-%s"]
  gira dev pr open --repo OWNER/REPO --issue N [--json]
  gira dev pr status --repo OWNER/REPO --issue N [--json]
`

const workHelp = `Daily issue lifecycle command.

Usage:
  gira start --repo OWNER/REPO --issue N --dry-run|--apply [--json]
  gira work start --repo OWNER/REPO --issue N --dry-run|--apply [--json]
  gira work pr --repo OWNER/REPO --issue N --dry-run|--apply [--draft] [--json]
  gira work status --repo OWNER/REPO --issue N [--json]

Commands:
  start   Verify a ready issue, create/reuse its branch, and move to in-progress on apply. Alias: gira start
  pr      Validate or create a linked PR with Closes #N and update review status on apply
  status  Report issue status, linked PR blockers, and next action

Flags:
  --repo string  Target GitHub repo in OWNER/REPO format
  --issue int    Issue number
  --dry-run      Preview without mutation
  --apply        Apply branch, PR, and status label changes
  --draft        Create/keep PR as draft for work pr
  --json         Emit stable JSON output
  -h, --help     Show help
`

const opsHelp = `Advanced Gira controls.

Usage:
  gira ops <command> [flags]

Commands:
  sync        Sync labels, milestones, and bootstrap issues
  detach      Plan/apply safe removal of Gira-managed repository artifacts
  doctor      Diagnose install, auth, repo, drift, and local git readiness
  onboard     Verify onboarding readiness from init to steady-state
  bootstrap   Bootstrap a repository into a Gira-managed project workspace
  project     Inspect permission capability for Project OS lifecycle actions
  guardrails  Audit and apply branch protection/ruleset policy
  audit       Verify audit ledgers for mutation integrity
  export      Export dashboard artifacts from read-only GitHub data
  portfolio   Plan top-level portfolio intake lowering from a portfolio repo
  contract    CRUD capability matrix and command contract
  parity      Compute deterministic Jira-replacement parity scorecard
  jira        Import/export Jira migration artifacts
  triage      Backlog triage queue and policy apply helpers
  graph       Work graph validation
  review      Review routing queue
  merge       Policy-checked merge queue
  report      Weekly PM cockpit report
  worker      Manage worker claim/handoff/release state for tickets
  dev         Lower-level issue-to-branch execution helpers

Flags:
  -h, --help  Show help
`

var newStatusClient = func(repo gira.RepoRef) gira.StatusClient {
	return gira.NewGHStatusClient(repo, gira.ExecCommandRunner{})
}

var newOnboardVerifyReport = func(repo gira.RepoRef, stage gira.OnboardStage) (gira.OnboardVerifyReport, error) {
	return gira.BuildOnboardVerifyReport(repo, stage, gira.ExecCommandRunner{}, time.Now().UTC()), nil
}

var newDoctorReport = func(repoValue string) gira.DoctorReport {
	return gira.BuildDoctorReport(repoValue, gira.ExecCommandRunner{}, time.Now().UTC())
}

var newSyncClient = func(repo gira.RepoRef) gira.SyncClient {
	return gira.NewGHSyncClient(repo, gira.ExecCommandRunner{})
}

var newDetachReport = func(repo gira.RepoRef, dryRun bool, apply bool) (gira.DetachReport, error) {
	client := gira.NewGHDetachClient(repo, gira.ExecCommandRunner{})
	report, err := gira.BuildDetachReport(client, dryRun)
	if err != nil {
		return gira.DetachReport{}, err
	}
	if apply {
		if err := gira.ApplyDetachReport(client, &report); err != nil {
			return gira.DetachReport{}, err
		}
	}
	return report, nil
}

var newProjectCapabilityReport = func(repo gira.RepoRef) (gira.ProjectCapabilityReport, error) {
	return gira.BuildProjectCapabilityReport(repo, gira.ExecCommandRunner{})
}

var newProjectSyncReport = func(repo gira.RepoRef, dryRun bool) (gira.ProjectSyncReport, error) {
	if !dryRun {
		return gira.ProjectSyncReport{}, fmt.Errorf("--dry-run is required for project sync unless --apply is provided")
	}
	return gira.BuildProjectSyncReportForClient(gira.NewGHProjectSyncClient(repo, gira.ExecCommandRunner{}), time.Now())
}

var newProjectSyncApplyReport = func(repo gira.RepoRef) (gira.ProjectSyncApplyReport, error) {
	capability, err := gira.BuildProjectCapabilityReport(repo, gira.ExecCommandRunner{})
	if err != nil {
		return gira.ProjectSyncApplyReport{}, err
	}
	return gira.BuildProjectSyncApplyReport(capability), nil
}

var newProjectTransitionsReport = func(repo gira.RepoRef, dryRun bool) (gira.ProjectTransitionsReport, error) {
	if !dryRun {
		return gira.ProjectTransitionsReport{}, fmt.Errorf("--dry-run is required for project transitions unless --apply is provided")
	}
	return gira.BuildProjectTransitionsReportForClient(gira.NewGHProjectTransitionsClient(repo, gira.ExecCommandRunner{}), time.Now())
}

var newProjectTransitionsApplyReport = func(repo gira.RepoRef) (gira.ProjectTransitionsApplyReport, error) {
	return gira.ApplyProjectTransitionsForClient(gira.NewGHProjectTransitionsClient(repo, gira.ExecCommandRunner{}), gira.ExecCommandRunner{}, time.Now())
}

var newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient {
	return gira.NewGHDashboardExportClient(repo, gira.ExecCommandRunner{})
}

var newPortfolioReport = func(command string, configPath string, dryRun bool) (gira.PortfolioReport, error) {
	resolved, err := gira.ResolvePortfolioConfig(configPath)
	if err != nil {
		return gira.PortfolioReport{}, err
	}
	client := gira.NewGHPortfolioClient(resolved.PortfolioRepo, gira.ExecCommandRunner{})
	switch command {
	case "status":
		return gira.BuildPortfolioStatusReport(client, resolved.Repos, time.Now())
	case "validate":
		return gira.BuildPortfolioValidateReport(client, resolved.Repos, time.Now())
	case "plan":
		if !dryRun {
			return gira.PortfolioReport{}, fmt.Errorf("--dry-run is required for portfolio plan")
		}
		report, err := gira.BuildPortfolioPlanReport(client, resolved.Repos, time.Now())
		if err != nil {
			return gira.PortfolioReport{}, err
		}
		capability, err := gira.BuildPortfolioCapabilityReport(resolved.PortfolioRepo, resolved.Repos, gira.ExecCommandRunner{}, time.Now())
		if err != nil {
			return gira.PortfolioReport{}, err
		}
		report.Capability = &capability
		report.PermissionBlocks = gira.PortfolioCapabilityBlocksForActions(capability, report.Actions)
		return report, nil
	default:
		return gira.PortfolioReport{}, fmt.Errorf("unknown portfolio command: %s", command)
	}
}

var newPortfolioCapabilityReport = func(configPath string) (gira.PortfolioCapabilityReport, error) {
	resolved, err := gira.ResolvePortfolioConfig(configPath)
	if err != nil {
		return gira.PortfolioCapabilityReport{}, err
	}
	return gira.BuildPortfolioCapabilityReport(resolved.PortfolioRepo, resolved.Repos, gira.ExecCommandRunner{}, time.Now())
}

var newPortfolioLowerReport = func(configPath string, apply bool) (gira.PortfolioLowerReport, error) {
	resolved, err := gira.ResolvePortfolioConfig(configPath)
	if err != nil {
		return gira.PortfolioLowerReport{}, err
	}
	runner := gira.ExecCommandRunner{}
	capability, err := gira.BuildPortfolioCapabilityReport(resolved.PortfolioRepo, resolved.Repos, runner, time.Now())
	if err != nil {
		return gira.PortfolioLowerReport{}, err
	}
	return gira.BuildPortfolioLowerReport(
		gira.NewGHPortfolioClient(resolved.PortfolioRepo, runner),
		gira.NewGHPortfolioLowerClient(resolved.PortfolioRepo, runner),
		resolved.Repos,
		capability,
		apply,
		time.Now(),
	)
}

var newWorkspaceStatusReport = func(configPath string) (gira.WorkspaceReport, error) {
	resolved, err := gira.ResolveWorkspaceConfig(configPath)
	if err != nil {
		return gira.WorkspaceReport{}, err
	}
	return gira.BuildWorkspaceStatusReport(resolved, gira.NewGHWorkspaceClient(gira.ExecCommandRunner{}), time.Now(), 14)
}

var newWorkspaceSyncReport = func(configPath string, dryRun bool, bootstrapIssues bool) (gira.WorkspaceSyncReport, error) {
	resolved, err := gira.ResolveWorkspaceConfig(configPath)
	if err != nil {
		return gira.WorkspaceSyncReport{}, err
	}
	syncer := func(repo gira.RepoRef, dryRun bool, bootstrapIssues bool) (gira.SyncPlan, error) {
		client := gira.NewGHSyncClient(repo, gira.ExecCommandRunner{})
		plan, err := gira.BuildSyncPlan(client, gira.SyncPlanOptions{EnableBootstrapIssues: bootstrapIssues, PolicyMode: gira.SyncPolicyMerge})
		if err != nil {
			return gira.SyncPlan{}, err
		}
		if !dryRun {
			if err := gira.ApplySyncPlan(client, plan); err != nil {
				return gira.SyncPlan{}, err
			}
		}
		return plan, nil
	}
	return gira.BuildWorkspaceSyncReport(resolved, syncer, dryRun, bootstrapIssues)
}

var newWorkspaceTicketNewReport = func(configPath string, title string, body string) (gira.WorkspaceTicketNewReport, error) {
	resolved, err := gira.ResolveWorkspaceConfig(configPath)
	if err != nil {
		return gira.WorkspaceTicketNewReport{}, err
	}
	return gira.BuildWorkspaceTicketNewReport(resolved, gira.NewGHWorkspaceClient(gira.ExecCommandRunner{}), title, body)
}

var newWorkspaceTicketRouteReport = func(configPath string, ticketValue string, repo gira.RepoRef, dryRun bool) (gira.WorkspaceTicketRouteReport, error) {
	resolved, err := gira.ResolveWorkspaceConfig(configPath)
	if err != nil {
		return gira.WorkspaceTicketRouteReport{}, err
	}
	ticket, err := gira.WorkspaceTicketNumber(ticketValue)
	if err != nil {
		return gira.WorkspaceTicketRouteReport{}, err
	}
	return gira.BuildWorkspaceTicketRouteReport(resolved, gira.NewGHWorkspaceClient(gira.ExecCommandRunner{}), ticket, repo, dryRun)
}

var newWorkspaceProjectPlanReport = func(configPath string) (gira.WorkspaceProjectPlanReport, error) {
	resolved, err := gira.ResolveWorkspaceConfig(configPath)
	if err != nil {
		return gira.WorkspaceProjectPlanReport{}, err
	}
	builder := func(repo gira.RepoRef) (gira.ProjectSyncReport, error) {
		return gira.BuildProjectSyncReportForClient(gira.NewGHProjectSyncClient(repo, gira.ExecCommandRunner{}), time.Now())
	}
	return gira.BuildWorkspaceProjectPlanReport(resolved, builder)
}

var newProjectsSyncReport = func(configPath string, dryRun bool, archiveClosed bool) (gira.ProjectsSyncReport, error) {
	resolved, err := gira.ResolveWorkspaceConfig(configPath)
	if err != nil {
		return gira.ProjectsSyncReport{}, err
	}
	return gira.BuildProjectsSyncReportWithOptions(resolved, gira.NewGHProjectsSyncClient(gira.ExecCommandRunner{}), gira.ProjectsSyncOptions{DryRun: dryRun, ArchiveClosed: archiveClosed, FetchedAt: time.Now()})
}

var newGraphClient = func(repo gira.RepoRef) gira.GraphClient {
	return gira.NewGHGraphClient(repo, gira.ExecCommandRunner{})
}

var newReviewGateClient = func(repo gira.RepoRef) gira.ReviewGateClient {
	client := gira.NewGHReviewGateClient(repo, gira.ExecCommandRunner{})
	return client
}

var reviewGateRunner gira.CommandRunner = gira.ExecCommandRunner{}

var newGuardrailsSyncReport = func(repo gira.RepoRef, policyPath string, apply bool, allowRelaxation bool) (gira.GuardrailsSyncReport, error) {
	policy, err := gira.LoadGuardrailsPolicy(policyPath)
	if err != nil {
		return gira.GuardrailsSyncReport{}, err
	}
	if apply {
		capability, err := newProjectCapabilityReport(repo)
		if err != nil {
			return gira.GuardrailsSyncReport{}, err
		}
		if !gira.HasCapability(capability, "repo:settings:write") {
			return gira.GuardrailsSyncReport{}, fmt.Errorf("repo:settings:write denied: %s", gira.CapabilityDeniedReason(capability, "repo:settings:write"))
		}
	}
	return gira.SyncGuardrailsForClient(repo, policy, gira.NewGHGuardrailsClient(repo, gira.ExecCommandRunner{}), apply, allowRelaxation)
}

var newJiraParityReport = func(repo gira.RepoRef) (gira.JiraParityReport, error) {
	capability, err := newProjectCapabilityReport(repo)
	if err != nil {
		return gira.JiraParityReport{}, err
	}
	return gira.BuildJiraParityReport(repo, capability, time.Now()), nil
}

var jiraCommandRunner gira.CommandRunner = gira.ExecCommandRunner{}

var newJiraImportReport = func(repo gira.RepoRef, source string, apiBase string, project string, dryRun bool, apply bool) (gira.JiraImportReport, error) {
	if strings.TrimSpace(source) != "" {
		items, err := gira.LoadJiraImportFile(source)
		if err != nil {
			return gira.JiraImportReport{}, err
		}
		return gira.ImportJiraItems(repo, source, items, dryRun, apply, jiraCommandRunner)
	}
	return gira.ImportJiraFromAPI(repo, apiBase, project, dryRun, apply, jiraCommandRunner)
}

var newJiraExportReport = func(repo gira.RepoRef, outputRoot string) (gira.JiraExportReport, error) {
	return gira.ExportJiraIssues(repo, outputRoot, jiraCommandRunner)
}

var dashboardExportNow = func() time.Time {
	return time.Now()
}

var statusNow = func() time.Time {
	return time.Now()
}

var reportNow = func() time.Time {
	return time.Now()
}

var newTriageReport = func(repo gira.RepoRef, apply bool) (gira.TriageNormalizeReport, error) {
	return gira.NormalizeOpenIssueTriage(gira.NewGHTriageClient(repo, gira.ExecCommandRunner{}), apply, time.Now())
}

var devCommandRunner gira.CommandRunner = gira.ExecCommandRunner{}
var repoContextRunner gira.CommandRunner = gira.ExecCommandRunner{}

var newWorkStartResult = func(repo gira.RepoRef, issue int, dryRun bool) (gira.WorkStartResult, error) {
	return gira.StartWork(repo, issue, dryRun, devCommandRunner)
}

var newWorkPRResult = func(repo gira.RepoRef, issue int, dryRun bool, draft bool) (gira.WorkPRResult, error) {
	return gira.OpenWorkPR(repo, issue, dryRun, draft, devCommandRunner)
}

var newWorkStatusResult = func(repo gira.RepoRef, issue int) (gira.WorkStatusResult, error) {
	return gira.GetWorkStatus(repo, issue, devCommandRunner)
}

var newWorkFinishResult = func(repo gira.RepoRef, issue int, dryRun bool, wait time.Duration) (gira.WorkFinishResult, error) {
	return gira.FinishWork(repo, issue, dryRun, wait, devCommandRunner)
}

var newTicketNewReport = func(input gira.TicketNewInput) (gira.TicketNewReport, error) {
	return gira.BuildTicketNewReport(input, devCommandRunner)
}

var newTicketChecksReport = func(repo gira.RepoRef, issue int, wait time.Duration, pollInterval time.Duration) (gira.TicketChecksReport, error) {
	return gira.BuildTicketChecksReport(repo, issue, wait, pollInterval, devCommandRunner)
}

var newUpgradeReport = func(channel string) (gira.UpgradeReport, error) {
	executable, _ := os.Executable()
	return gira.BuildUpgradeReport(channel, executable, nil)
}

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, rootHelp)
		return 0
	}
	if args[0] == "--version" {
		return runVersion(nil, stdout, stderr)
	}

	switch args[0] {
	case "guide", "docs":
		return runGuide(args[1:], stdout, stderr)
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "workspace":
		return runWorkspace(args[1:], stdout, stderr)
	case "projects":
		return runProjects(args[1:], stdout, stderr)
	case "start":
		return runTicketStart(args[1:], stdout, stderr)
	case "ticket":
		return runTicket(args[1:], stdout, stderr)
	case "ops":
		return runOps(args[1:], stdout, stderr)
	case "bootstrap":
		return runBootstrap(args[1:], stdout, stderr)
	case "onboard":
		return runOnboard(args[1:], stdout, stderr)
	case "doctor":
		return runDoctor(args[1:], stdout, stderr)
	case "work":
		return runWork(args[1:], stdout, stderr)
	case "dev":
		return runDev(args[1:], stdout, stderr)
	case "sync":
		return runSync(args[1:], stdout, stderr)
	case "detach":
		return runDetach(args[1:], stdout, stderr)
	case "status":
		return runStatus(args[1:], stdout, stderr)
	case "upgrade", "update":
		return runUpgrade(args[1:], stdout, stderr)
	case "version":
		return runVersion(args[1:], stdout, stderr)
	case "jira":
		return runJira(args[1:], stdout, stderr)
	case "export":
		return runExport(args[1:], stdout, stderr)
	case "portfolio":
		return runPortfolio(args[1:], stdout, stderr)
	case "parity":
		return runParity(args[1:], stdout, stderr)
	case "project":
		return runProject(args[1:], stdout, stderr)
	case "audit":
		return runAudit(args[1:], stdout, stderr)
	case "worker":
		return runWorker(args[1:], stdout, stderr)
	case "guardrails":
		return runGuardrails(args[1:], stdout, stderr)
	case "triage":
		return runTriage(args[1:], stdout, stderr)
	case "sprint":
		return runSprint(args[1:], stdout, stderr)
	case "graph":
		return runGraph(args[1:], stdout, stderr)
	case "review":
		return runReview(args[1:], stdout, stderr)
	case "merge":
		return runMerge(args[1:], stdout, stderr)
	case "release":
		return runRelease(args[1:], stdout, stderr)
	case "report":
		return runReport(args[1:], stdout, stderr)
	case "contract":
		return runContract(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		fmt.Fprint(stderr, rootHelp)
		return 2
	}
}

func runGuide(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, guideQuickstart)
		return 0
	}
	if len(args) > 1 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", args[1])
		fmt.Fprint(stderr, guideHelp)
		return 2
	}
	switch args[0] {
	case "--help", "-h", "help":
		fmt.Fprint(stdout, guideHelp)
	case "quickstart":
		fmt.Fprint(stdout, guideQuickstart)
	case "ticket":
		fmt.Fprint(stdout, guideTicket)
	case "agent":
		fmt.Fprint(stdout, guideAgent)
	case "concepts":
		fmt.Fprint(stdout, guideConcepts)
	default:
		fmt.Fprintf(stderr, "unknown guide topic: %s\n\n", args[0])
		fmt.Fprint(stderr, guideHelp)
		return 2
	}
	return 0
}

func runVersion(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOutput := fs.Bool("json", false, "Emit stable JSON version info")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, versionHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, versionHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, versionHelp)
		return 2
	}

	info := gira.BuildVersionInfo()
	if *jsonOutput {
		out, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode version JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatVersionInfo(info))
	return 0
}

func runUpgrade(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	channel := fs.String("channel", "auto", "Installed channel to use for the next-step command")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON upgrade info")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, upgradeHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, upgradeHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, upgradeHelp)
		return 2
	}

	report, err := newUpgradeReport(*channel)
	if err != nil {
		fmt.Fprintf(stderr, "upgrade check failed: %v\n", err)
		return 1
	}
	if *jsonOutput {
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode upgrade JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatUpgradeReport(report))
	return 0
}

func runOps(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, opsHelp)
		return 0
	}
	switch args[0] {
	case "bootstrap":
		return runBootstrap(args[1:], stdout, stderr)
	case "onboard":
		return runOnboard(args[1:], stdout, stderr)
	case "doctor":
		return runDoctor(args[1:], stdout, stderr)
	case "sync":
		return runSyncWithCommand(args[1:], stdout, stderr, "gira ops sync")
	case "detach":
		return runDetach(args[1:], stdout, stderr)
	case "jira":
		return runJira(args[1:], stdout, stderr)
	case "export":
		return runExport(args[1:], stdout, stderr)
	case "portfolio":
		return runPortfolio(args[1:], stdout, stderr)
	case "parity":
		return runParity(args[1:], stdout, stderr)
	case "project":
		return runProject(args[1:], stdout, stderr)
	case "audit":
		return runAudit(args[1:], stdout, stderr)
	case "guardrails":
		return runGuardrails(args[1:], stdout, stderr)
	case "triage":
		return runTriage(args[1:], stdout, stderr)
	case "graph":
		return runGraph(args[1:], stdout, stderr)
	case "review":
		return runReview(args[1:], stdout, stderr)
	case "merge":
		return runMerge(args[1:], stdout, stderr)
	case "report":
		return runReport(args[1:], stdout, stderr)
	case "worker":
		return runWorker(args[1:], stdout, stderr)
	case "dev":
		return runDev(args[1:], stdout, stderr)
	case "contract":
		return runContract(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown ops command: %s\n\n", args[0])
		fmt.Fprint(stderr, opsHelp)
		return 2
	}
}

func resolveRepoContext(repoValue string, stderr io.Writer, help string) (gira.RepoRef, bool) {
	repo, err := gira.ResolveRepoContext(repoValue, repoContextRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, help)
		return gira.RepoRef{}, false
	}
	return repo, true
}

func runDoctor(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON report")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, doctorHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, doctorHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, doctorHelp)
		return 2
	}

	report := newDoctorReport(*repoValue)
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode doctor JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
	} else {
		fmt.Fprint(stdout, gira.FormatDoctorReport(report))
	}
	if !report.Ready {
		return 1
	}
	return 0
}

func runInit(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	pathValue := fs.String("path", ".", "Local git workspace path to validate")
	configPath := fs.String("config", "", "Optional init profile schema path")
	dryRun := fs.Bool("dry-run", true, "Emit plan only")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, initHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, initHelp)
		return 0
	}
	if *repoValue == "" {
		fmt.Fprint(stderr, "--repo is required\n\n")
		_, _ = io.WriteString(stderr, initHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	loadedConfigPath := strings.TrimSpace(*configPath)
	var loadedConfig gira.InitConfig
	if loadedConfigPath == "" {
		defaultConfigPath := gira.DefaultInitConfigPath(".")
		if stat, err := os.Stat(defaultConfigPath); err == nil && !stat.IsDir() {
			loadedConfigPath = defaultConfigPath
		} else {
			workspaceConfigPath := gira.DefaultInitConfigPath(*pathValue)
			if stat, err := os.Stat(workspaceConfigPath); err == nil && !stat.IsDir() {
				loadedConfigPath = workspaceConfigPath
			}
		}
	}
	if loadedConfigPath != "" {
		cfg, err := gira.LoadInitConfig(loadedConfigPath)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
		loadedConfig = cfg
	}

	var configPtr *gira.InitConfig
	if loadedConfigPath != "" {
		configPtr = &loadedConfig
	}
	report, err := gira.BuildInitReportWithConfig(repo, *pathValue, *dryRun, loadedConfigPath, configPtr, devCommandRunner)
	if err != nil {
		if *jsonOutput {
			out, _ := json.MarshalIndent(report, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
		}
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if *jsonOutput {
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode init JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatInitReport(report))
	return 0
}

func runParity(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		_, _ = io.WriteString(stdout, parityHelp)
		return 0
	}
	if args[0] != "jira" {
		fmt.Fprintf(stderr, "unknown parity command: %s\n\n", args[0])
		_, _ = io.WriteString(stderr, parityHelp)
		return 2
	}

	fs := flag.NewFlagSet("parity jira", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, parityHelp)
		return 2
	}
	if *repoValue == "" {
		fmt.Fprint(stderr, "--repo is required\n\n")
		_, _ = io.WriteString(stderr, parityHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newJiraParityReport(repo)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "encode parity JSON: %v\n", err)
		return 2
	}
	if *jsonOutput {
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatJiraParityReport(report))
	return 0
}

func runJira(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		_, _ = io.WriteString(stdout, jiraHelp)
		return 0
	}
	switch args[0] {
	case "import":
		return runJiraImport(args[1:], stdout, stderr)
	case "export":
		return runJiraExport(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown jira command: %s\n\n", args[0])
		_, _ = io.WriteString(stderr, jiraHelp)
		return 2
	}
}

func runJiraImport(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("jira import", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	source := fs.String("source", "", "CSV or JSON import source path")
	apiBase := fs.String("api-base", "", "Jira API base URL")
	project := fs.String("project", "", "Jira project key")
	dryRun := fs.Bool("dry-run", false, "Preview without mutation")
	apply := fs.Bool("apply", false, "Apply GitHub issue creates")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, jiraHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, jiraHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		_, _ = io.WriteString(stderr, jiraHelp)
		return 2
	}
	if *repoValue == "" {
		fmt.Fprint(stderr, "--repo is required\n\n")
		_, _ = io.WriteString(stderr, jiraHelp)
		return 2
	}
	if *dryRun == *apply {
		fmt.Fprint(stderr, "exactly one of --dry-run or --apply is required\n\n")
		_, _ = io.WriteString(stderr, jiraHelp)
		return 2
	}
	fileMode := strings.TrimSpace(*source) != ""
	apiMode := strings.TrimSpace(*apiBase) != "" || strings.TrimSpace(*project) != ""
	if fileMode == apiMode {
		fmt.Fprint(stderr, "exactly one of --source or --api-base/--project is required\n\n")
		_, _ = io.WriteString(stderr, jiraHelp)
		return 2
	}
	if apiMode && (strings.TrimSpace(*apiBase) == "" || strings.TrimSpace(*project) == "") {
		fmt.Fprint(stderr, "--api-base and --project are required for Jira API import\n\n")
		_, _ = io.WriteString(stderr, jiraHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newJiraImportReport(repo, *source, *apiBase, *project, *dryRun, *apply)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode jira import JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatJiraImportReport(report))
	return 0
}

func runJiraExport(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("jira export", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	outputRoot := fs.String("output", "", "Output directory for artifacts")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, jiraHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, jiraHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		_, _ = io.WriteString(stderr, jiraHelp)
		return 2
	}
	if *repoValue == "" || *outputRoot == "" {
		fmt.Fprint(stderr, "--repo and --output are required\n\n")
		_, _ = io.WriteString(stderr, jiraHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newJiraExportReport(repo, *outputRoot)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode jira export JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatJiraExportReport(report))
	return 0
}

func runDev(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		_, _ = io.WriteString(stdout, devHelp)
		return 0
	}
	if args[0] == "pr" {
		return runDevPR(args[1:], stdout, stderr)
	}
	if args[0] != "start" {
		fmt.Fprintf(stderr, "unknown dev command: %s\n\n", args[0])
		_, _ = io.WriteString(stderr, devHelp)
		return 2
	}
	fs := flag.NewFlagSet("dev start", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	issue := fs.Int("issue", 0, "Issue number")
	dryRun := fs.Bool("dry-run", false, "Preview only")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	force := fs.Bool("force", false, "Override readiness checks")
	pattern := fs.String("branch-pattern", gira.DefaultDevBranchPattern, "fmt pattern for branch naming")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, devHelp)
		return 2
	}
	if *repoValue == "" || *issue <= 0 {
		fmt.Fprint(stderr, "--repo and --issue are required\n\n")
		_, _ = io.WriteString(stderr, devHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	result, err := gira.StartDevBranch(repo, *issue, *pattern, *dryRun, *force, devCommandRunner)
	if err != nil {
		if *jsonOutput {
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
		}
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if *jsonOutput {
		out, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode dev start JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprintf(stdout, "dev start: %s\n", result.Branch)
	return 0
}

func runDevPR(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stderr, devHelp)
		return 2
	}
	cmd := args[0]
	fs := flag.NewFlagSet("dev pr "+cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	issue := fs.Int("issue", 0, "Issue number")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, devHelp)
		return 2
	}
	if *repoValue == "" || *issue <= 0 {
		fmt.Fprint(stderr, "--repo and --issue are required\n\n")
		_, _ = io.WriteString(stderr, devHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	switch cmd {
	case "open":
		result, err := gira.OpenDevPR(repo, *issue, devCommandRunner)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		if *jsonOutput {
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
			return 0
		}
		fmt.Fprintf(stdout, "dev pr open: %s\n", result.PRURL)
		return 0
	case "status":
		result, err := gira.DevPRStatus(repo, *issue, devCommandRunner)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		if *jsonOutput {
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
			return 0
		}
		fmt.Fprintf(stdout, "dev pr status: ready=%t blockers=%s\n", result.Ready, strings.Join(result.Blockers, ","))
		return 0
	default:
		fmt.Fprintf(stderr, "unknown dev pr command: %s\n\n", cmd)
		_, _ = io.WriteString(stderr, devHelp)
		return 2
	}
}

func runWork(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		_, _ = io.WriteString(stdout, workHelp)
		return 0
	}
	switch args[0] {
	case "start":
		return runWorkStart(args[1:], stdout, stderr)
	case "pr":
		return runWorkPR(args[1:], stdout, stderr)
	case "status":
		return runWorkStatus(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown work command: %s\n\n", args[0])
		_, _ = io.WriteString(stderr, workHelp)
		return 2
	}
}

func runTicket(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		_, _ = io.WriteString(stdout, ticketHelp)
		return 0
	}
	switch args[0] {
	case "new":
		return runTicketNew(args[1:], stdout, stderr)
	case "start":
		return runTicketStart(args[1:], stdout, stderr)
	case "pr":
		return runTicketPR(args[1:], stdout, stderr)
	case "checks":
		return runTicketChecks(args[1:], stdout, stderr)
	case "wait":
		return runTicketWait(args[1:], stdout, stderr)
	case "finish":
		return runTicketFinish(args[1:], stdout, stderr)
	case "status":
		return runTicketStatus(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown ticket command: %s\n\n", args[0])
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
}

func runTicketNew(args []string, stdout io.Writer, stderr io.Writer) int {
	args, positionalTitle, titleOK := extractTitlePositional(args, stderr)
	if !titleOK {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	fs := flag.NewFlagSet("ticket new", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	title := fs.String("title", "", "Ticket title")
	goal := fs.String("goal", "", "Ticket goal")
	scope := fs.String("scope", "", "Ticket scope")
	acceptance := fs.String("acceptance", "", "Semicolon-separated acceptance criteria")
	notes := fs.String("notes", "", "Ticket notes")
	ticketType := fs.String("type", "task", "Ticket type: task|bug|story|chore")
	priority := fs.String("priority", "", "Priority: p0|p1|p2|p3")
	milestone := fs.String("milestone", "", "Milestone title")
	bodyFile := fs.String("body-file", "", "Read issue body from file")
	start := fs.Bool("start", false, "Start the created ticket")
	dryRun := fs.Bool("dry-run", false, "Preview without mutation")
	apply := fs.Bool("apply", false, "Apply changes")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	var labels repeatedStringFlag
	fs.Var(&labels, "label", "Additional GitHub label")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, ticketHelp)
		return 0
	}
	resolvedTitle, ok := resolveTicketNewTitle(positionalTitle, *title, stderr)
	if !ok {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	if *dryRun == *apply {
		fmt.Fprint(stderr, "exactly one of --dry-run/--apply is required\n\n")
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	repo, err := gira.ResolveRepoContext(*repoValue, repoContextRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newTicketNewReport(gira.TicketNewInput{
		Repo:       repo,
		Title:      resolvedTitle,
		Goal:       *goal,
		Scope:      *scope,
		Acceptance: splitList(*acceptance),
		Notes:      *notes,
		Type:       *ticketType,
		Priority:   *priority,
		Milestone:  *milestone,
		Labels:     labels,
		BodyFile:   *bodyFile,
		Start:      *start,
		DryRun:     *dryRun,
	})
	if err != nil {
		if *jsonOutput {
			out, _ := json.MarshalIndent(report, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
		}
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if *jsonOutput {
		out, _ := json.MarshalIndent(report, "", "  ")
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatTicketNew(report))
	return 0
}

func runTicketStart(args []string, stdout io.Writer, stderr io.Writer) int {
	args, positionalTicket, positionalOK := extractTicketPositional(args, stderr)
	if !positionalOK {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	fs := flag.NewFlagSet("ticket start", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	ticket := fs.Int("ticket", 0, "Ticket number")
	issue := fs.Int("issue", 0, "Compatibility alias for --ticket")
	dryRun := fs.Bool("dry-run", false, "Preview without mutation")
	apply := fs.Bool("apply", false, "Apply changes")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, ticketHelp)
		return 0
	}
	ticketNumber, ok := resolveExplicitTicket(*ticket, *issue, positionalTicket, stderr)
	if !ok {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	repo, ok := parseTicketRequiredFlags(*repoValue, ticketNumber, *dryRun, *apply, false, stderr)
	if !ok {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	result, err := newWorkStartResult(repo, ticketNumber, *dryRun)
	if err != nil {
		if *jsonOutput {
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
		}
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if *jsonOutput {
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, formatTicketStart(result))
	return 0
}

func runTicketPR(args []string, stdout io.Writer, stderr io.Writer) int {
	args, positionalTicket, positionalOK := extractTicketPositional(args, stderr)
	if !positionalOK {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	fs := flag.NewFlagSet("ticket pr", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	ticket := fs.Int("ticket", 0, "Ticket number")
	issue := fs.Int("issue", 0, "Compatibility alias for --ticket")
	dryRun := fs.Bool("dry-run", false, "Preview without mutation")
	apply := fs.Bool("apply", false, "Apply changes")
	draft := fs.Bool("draft", false, "Create/keep PR as draft")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, ticketHelp)
		return 0
	}
	repo, ok := parseTicketRequiredFlags(*repoValue, 1, *dryRun, *apply, true, stderr)
	if !ok {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	ticketNumber, ok := resolveTicketContext(repo, *ticket, *issue, positionalTicket, true, stderr)
	if !ok {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	result, err := newWorkPRResult(repo, ticketNumber, *dryRun, *draft)
	if err != nil {
		if *jsonOutput {
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
		}
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if *jsonOutput {
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, formatTicketPR(result))
	return 0
}

func runTicketChecks(args []string, stdout io.Writer, stderr io.Writer) int {
	return runTicketChecksLike(args, stdout, stderr, false)
}

func runTicketWait(args []string, stdout io.Writer, stderr io.Writer) int {
	return runTicketChecksLike(args, stdout, stderr, true)
}

func runTicketChecksLike(args []string, stdout io.Writer, stderr io.Writer, waitMode bool) int {
	args, positionalTicket, positionalOK := extractTicketPositional(args, stderr)
	if !positionalOK {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	name := "ticket checks"
	if waitMode {
		name = "ticket wait"
	}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	ticket := fs.Int("ticket", 0, "Ticket number")
	issue := fs.Int("issue", 0, "Compatibility alias for --ticket")
	timeout := fs.Duration("timeout", 5*time.Minute, "Pending-check wait timeout")
	interval := fs.Duration("interval", 5*time.Second, "Pending-check poll interval")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, ticketHelp)
		return 0
	}
	repo, err := gira.ResolveRepoContext(*repoValue, repoContextRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	ticketNumber, ok := resolveTicketContext(repo, *ticket, *issue, positionalTicket, true, stderr)
	if !ok {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	wait := time.Duration(0)
	poll := time.Duration(0)
	if waitMode {
		wait = *timeout
		poll = *interval
	}
	result, err := newTicketChecksReport(repo, ticketNumber, wait, poll)
	result.NextStep = shortenTicketNextStep(result.NextStep, result.Repo, result.Issue)
	if err != nil {
		if *jsonOutput {
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
		}
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if *jsonOutput {
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatTicketChecks(result))
	return 0
}

func runTicketFinish(args []string, stdout io.Writer, stderr io.Writer) int {
	args, positionalTicket, positionalOK := extractTicketPositional(args, stderr)
	if !positionalOK {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	fs := flag.NewFlagSet("ticket finish", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	ticket := fs.Int("ticket", 0, "Ticket number")
	issue := fs.Int("issue", 0, "Compatibility alias for --ticket")
	dryRun := fs.Bool("dry-run", false, "Preview without mutation")
	apply := fs.Bool("apply", false, "Apply changes")
	wait := fs.Duration("wait", 0, "Optional pending-check wait")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, ticketHelp)
		return 0
	}
	repo, ok := parseTicketRequiredFlags(*repoValue, 1, *dryRun, *apply, true, stderr)
	if !ok {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	ticketNumber, ok := resolveTicketContext(repo, *ticket, *issue, positionalTicket, true, stderr)
	if !ok {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	result, err := newWorkFinishResult(repo, ticketNumber, *dryRun, *wait)
	result = normalizeTicketFinishResult(result)
	if err != nil {
		if *jsonOutput {
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
		}
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if *jsonOutput {
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, formatTicketFinish(result))
	return 0
}

func runTicketStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	args, positionalTicket, positionalOK := extractTicketPositional(args, stderr)
	if !positionalOK {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	fs := flag.NewFlagSet("ticket status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	ticket := fs.Int("ticket", 0, "Ticket number")
	issue := fs.Int("issue", 0, "Compatibility alias for --ticket")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, ticketHelp)
		return 0
	}
	repo, err := gira.ResolveRepoContext(*repoValue, repoContextRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	ticketNumber, ok := resolveTicketContext(repo, *ticket, *issue, positionalTicket, true, stderr)
	if !ok {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	result, err := newWorkStatusResult(repo, ticketNumber)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	result.NextStep = ticketStatusNextStep(result)
	if *jsonOutput {
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, formatTicketStatus(result))
	return 0
}

func runWorkStart(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("work start", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	issue := fs.Int("issue", 0, "Issue number")
	dryRun := fs.Bool("dry-run", false, "Preview without mutation")
	apply := fs.Bool("apply", false, "Apply changes")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, workHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, workHelp)
		return 0
	}
	repo, ok := parseWorkRequiredFlags(*repoValue, *issue, *dryRun, *apply, stderr)
	if !ok {
		_, _ = io.WriteString(stderr, workHelp)
		return 2
	}
	result, err := newWorkStartResult(repo, *issue, *dryRun)
	if err != nil {
		if *jsonOutput {
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
		}
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if *jsonOutput {
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatWorkStart(result))
	return 0
}

func runWorkPR(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("work pr", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	issue := fs.Int("issue", 0, "Issue number")
	dryRun := fs.Bool("dry-run", false, "Preview without mutation")
	apply := fs.Bool("apply", false, "Apply changes")
	draft := fs.Bool("draft", false, "Create/keep PR as draft")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, workHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, workHelp)
		return 0
	}
	repo, ok := parseWorkRequiredFlags(*repoValue, *issue, *dryRun, *apply, stderr)
	if !ok {
		_, _ = io.WriteString(stderr, workHelp)
		return 2
	}
	result, err := newWorkPRResult(repo, *issue, *dryRun, *draft)
	if err != nil {
		if *jsonOutput {
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
		}
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if *jsonOutput {
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatWorkPR(result))
	return 0
}

func runWorkStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("work status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	issue := fs.Int("issue", 0, "Issue number")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, workHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, workHelp)
		return 0
	}
	if *repoValue == "" || *issue <= 0 {
		fmt.Fprint(stderr, "--repo and --issue are required\n\n")
		_, _ = io.WriteString(stderr, workHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	result, err := newWorkStatusResult(repo, *issue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if *jsonOutput {
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatWorkStatus(result))
	return 0
}

func parseWorkRequiredFlags(repoValue string, issue int, dryRun bool, apply bool, stderr io.Writer) (gira.RepoRef, bool) {
	if repoValue == "" || issue <= 0 || dryRun == apply {
		fmt.Fprint(stderr, "--repo, --issue, and exactly one of --dry-run/--apply are required\n\n")
		return gira.RepoRef{}, false
	}
	repo, err := gira.ParseRepoRef(repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return gira.RepoRef{}, false
	}
	return repo, true
}

func extractTicketPositional(args []string, stderr io.Writer) ([]string, int, bool) {
	cleaned := make([]string, 0, len(args))
	positional := 0
	valueFlags := map[string]struct{}{"--repo": {}, "--ticket": {}, "--issue": {}, "--wait": {}, "--timeout": {}, "--interval": {}}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		cleaned = append(cleaned, arg)
		if _, ok := valueFlags[arg]; ok {
			if i+1 < len(args) {
				i++
				cleaned = append(cleaned, args[i])
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		n, err := strconv.Atoi(arg)
		if err != nil || n <= 0 {
			fmt.Fprintf(stderr, "unexpected positional argument %q; use a numeric ticket or --ticket N\n\n", arg)
			return nil, 0, false
		}
		if positional > 0 && positional != n {
			fmt.Fprint(stderr, "only one positional ticket can be provided\n\n")
			return nil, 0, false
		}
		positional = n
		cleaned = cleaned[:len(cleaned)-1]
	}
	return cleaned, positional, true
}

func extractTitlePositional(args []string, stderr io.Writer) ([]string, string, bool) {
	cleaned := make([]string, 0, len(args))
	title := ""
	valueFlags := map[string]struct{}{"--repo": {}, "--title": {}, "--goal": {}, "--scope": {}, "--acceptance": {}, "--notes": {}, "--type": {}, "--priority": {}, "--milestone": {}, "--label": {}, "--body-file": {}}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		cleaned = append(cleaned, arg)
		if _, ok := valueFlags[arg]; ok {
			if i+1 < len(args) {
				i++
				cleaned = append(cleaned, args[i])
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if title != "" {
			fmt.Fprint(stderr, "only one positional title can be provided; use --title for explicit title\n\n")
			return nil, "", false
		}
		title = arg
		cleaned = cleaned[:len(cleaned)-1]
	}
	return cleaned, title, true
}

func resolveTicketNewTitle(positional string, title string, stderr io.Writer) (string, bool) {
	positional = strings.TrimSpace(positional)
	title = strings.TrimSpace(title)
	if positional != "" && title != "" && positional != title {
		fmt.Fprint(stderr, "positional title and --title must match when both are provided\n\n")
		return "", false
	}
	if title != "" {
		return title, true
	}
	if positional != "" {
		return positional, true
	}
	fmt.Fprint(stderr, "ticket title is required\n\n")
	return "", false
}

func resolveExplicitTicket(ticket int, issue int, positional int, stderr io.Writer) (int, bool) {
	candidates := make([]int, 0, 3)
	for _, n := range []int{ticket, issue, positional} {
		if n > 0 {
			candidates = append(candidates, n)
		}
	}
	for _, n := range candidates {
		if n != candidates[0] {
			fmt.Fprint(stderr, "--ticket, --issue, and positional ticket must refer to the same number when more than one is provided\n\n")
			return 0, false
		}
	}
	if len(candidates) == 0 {
		return 0, true
	}
	return candidates[0], true
}

func resolveTicketContext(repo gira.RepoRef, ticket int, issue int, positional int, allowInference bool, stderr io.Writer) (int, bool) {
	explicit, ok := resolveExplicitTicket(ticket, issue, positional, stderr)
	if !ok {
		return 0, false
	}
	if explicit > 0 {
		return explicit, true
	}
	if !allowInference {
		fmt.Fprint(stderr, "--ticket or positional ticket is required for ticket start\n\n")
		return 0, false
	}
	inferred, err := inferTicketFromCurrentContext(repo, devCommandRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		return 0, false
	}
	return inferred, true
}

func parseTicketRequiredFlags(repoValue string, ticket int, dryRun bool, apply bool, allowTicketInference bool, stderr io.Writer) (gira.RepoRef, bool) {
	if dryRun == apply {
		fmt.Fprint(stderr, "exactly one of --dry-run/--apply is required\n\n")
		return gira.RepoRef{}, false
	}
	if ticket <= 0 && !allowTicketInference {
		fmt.Fprint(stderr, "--ticket or positional ticket is required\n\n")
		return gira.RepoRef{}, false
	}
	repo, err := gira.ResolveRepoContext(repoValue, repoContextRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return gira.RepoRef{}, false
	}
	return repo, true
}

func inferTicketFromCurrentContext(repo gira.RepoRef, runner gira.CommandRunner) (int, error) {
	if runner == nil {
		runner = gira.ExecCommandRunner{}
	}
	if out, err := runner.Run("git", "branch", "--show-current"); err == nil {
		if n := issueNumberFromRef(strings.TrimSpace(string(out))); n > 0 {
			return n, nil
		}
	}
	out, err := runner.Run("gh", "pr", "view", "--repo", repo.FullName(), "--json", "body,headRefName,title")
	if err != nil {
		return 0, fmt.Errorf("ticket context unavailable: pass --ticket N or run from an issue branch")
	}
	var raw struct {
		Body        string `json:"body"`
		HeadRefName string `json:"headRefName"`
		Title       string `json:"title"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return 0, fmt.Errorf("parse PR context JSON: %w", err)
	}
	issues := gira.ExtractClosureIssueNumbers(raw.Body)
	if len(issues) == 1 {
		return issues[0], nil
	}
	if len(issues) > 1 {
		return 0, fmt.Errorf("ticket context ambiguous: PR body references multiple closing issues; pass --ticket N")
	}
	for _, ref := range []string{raw.HeadRefName, raw.Title} {
		if n := issueNumberFromRef(ref); n > 0 {
			return n, nil
		}
	}
	return 0, fmt.Errorf("ticket context unavailable: pass --ticket N or run from an issue branch")
}

func issueNumberFromRef(ref string) int {
	for _, segment := range strings.Split(ref, "/") {
		if !strings.HasPrefix(segment, "issue-") {
			continue
		}
		rest := strings.TrimPrefix(segment, "issue-")
		digits := strings.Builder{}
		for _, r := range rest {
			if r < '0' || r > '9' {
				break
			}
			digits.WriteRune(r)
		}
		if digits.Len() == 0 {
			continue
		}
		n, _ := strconv.Atoi(digits.String())
		if n > 0 {
			return n
		}
	}
	return 0
}

func formatTicketStart(result gira.WorkStartResult) string {
	next := fmt.Sprintf("gira ticket pr --repo %s --ticket %d --dry-run", result.Repo, result.Issue)
	if result.DryRun {
		next = fmt.Sprintf("gira ticket start %d --apply", result.Issue)
	} else {
		next = "gira ticket pr --dry-run"
	}
	return fmt.Sprintf(
		"ticket start: ticket #%d branch=%s status=%s\nnext step: %s\n",
		result.Issue,
		result.Branch,
		result.NextStatus,
		next,
	)
}

func formatTicketPR(result gira.WorkPRResult) string {
	created := "reused"
	if result.Created {
		created = "created"
	}
	url := strings.TrimSpace(result.PRURL)
	if url == "" {
		url = "(planned)"
	}
	next := "gira ticket status"
	if result.Draft {
		next = "mark the PR ready, then " + next
	}
	return fmt.Sprintf("ticket pr: ticket #%d pr=%s status=%s %s\nnext step: %s\n", result.Issue, url, result.NextStatus, created, next)
}

func formatTicketStatus(result gira.WorkStatusResult) string {
	blockers := strings.Join(result.Blockers, ",")
	if blockers == "" {
		blockers = "none"
	}
	return fmt.Sprintf(
		"ticket status: ticket #%d status=%s pr=%d blockers=%s next=%s\nnext step: %s\n",
		result.Issue,
		result.Status,
		result.PRNumber,
		blockers,
		result.NextAction,
		ticketStatusNextStep(result),
	)
}

func formatTicketFinish(result gira.WorkFinishResult) string {
	blockers := strings.Join(result.Blockers, ",")
	if blockers == "" {
		blockers = "none"
	}
	actions := make([]string, 0, len(result.Actions))
	for _, action := range result.Actions {
		actions = append(actions, action.Action+":"+action.Status)
	}
	if len(actions) == 0 {
		actions = append(actions, "none")
	}
	return fmt.Sprintf(
		"ticket finish: ticket #%d pr=%d merged=%t blockers=%s actions=%s\nnext step: %s\n",
		result.Issue,
		result.PRNumber,
		result.Merged,
		blockers,
		strings.Join(actions, ","),
		ticketFinishNextStep(result),
	)
}

func ticketStatusNextStep(result gira.WorkStatusResult) string {
	switch result.NextAction {
	case "start_work":
		return fmt.Sprintf("gira ticket start %d --apply", result.Issue)
	case "open_pr":
		return "gira ticket pr --apply"
	case "mark_pr_ready":
		return "mark the PR ready for review"
	case "address_review":
		return "address review blockers"
	case "wait_for_checks":
		return "wait for required checks to finish or fix failing checks"
	case "merge_when_policy_allows":
		return "gira ticket finish --dry-run"
	case "done":
		return "ticket is done"
	case "closed":
		return "ticket is closed; inspect GitHub history if more evidence is needed"
	default:
		return fmt.Sprintf("gira status --repo %s", result.Repo)
	}
}

func ticketFinishNextStep(result gira.WorkFinishResult) string {
	next := strings.TrimSpace(result.NextStep)
	if next == "" {
		return "gira ticket status"
	}
	next = strings.ReplaceAll(next, "gira work status", "gira ticket status")
	next = strings.ReplaceAll(next, "gira work pr", "gira ticket pr")
	next = strings.ReplaceAll(next, "gira work start", "gira ticket start")
	next = strings.ReplaceAll(next, "--issue", "--ticket")
	next = shortenTicketNextStep(next, result.Repo, result.Issue)
	return next
}

func shortenTicketNextStep(next string, repo string, issue int) string {
	if repo != "" {
		next = strings.ReplaceAll(next, " --repo "+repo, "")
	}
	if issue > 0 {
		next = strings.ReplaceAll(next, fmt.Sprintf(" --ticket %d", issue), "")
		next = strings.ReplaceAll(next, fmt.Sprintf("gira ticket start --ticket %d", issue), fmt.Sprintf("gira ticket start %d", issue))
	}
	return strings.Join(strings.Fields(next), " ")
}

func normalizeTicketFinishResult(result gira.WorkFinishResult) gira.WorkFinishResult {
	result.NextStep = ticketFinishNextStep(result)
	if result.FinalStatus.Repo != "" && result.FinalStatus.Issue > 0 {
		result.FinalStatus.NextStep = ticketStatusNextStep(result.FinalStatus)
	}
	return result
}

func runBootstrap(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	template := fs.String("template", "default", "Template name to render")
	dryRun := fs.Bool("dry-run", false, "Render without writing files or calling GitHub")
	path := fs.String("path", "", "Local target git repo path")
	overwrite := fs.Bool("overwrite", false, "Overwrite existing files that differ")
	branch := fs.String("branch", gira.DefaultBranch, "Branch to create/checkout before install")
	noBranch := fs.Bool("no-branch", false, "Skip branch creation/checkout")
	createdAt := fs.String("created-at", "", "Override render date for deterministic tests")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, bootstrapHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, bootstrapHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, bootstrapHelp)
		return 2
	}
	if *repoValue == "" {
		fmt.Fprint(stderr, "--repo is required\n\n")
		fmt.Fprint(stderr, bootstrapHelp)
		return 2
	}

	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}

	renderDate := *createdAt
	if renderDate == "" {
		renderDate = time.Now().Format(time.DateOnly)
	}

	rendered, err := gira.RenderTemplateTree(*template, repo, renderDate)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *dryRun {
		fmt.Fprint(stdout, gira.FormatDryRun(rendered))
		return 0
	}

	if *path == "" {
		fmt.Fprint(stderr, "--path is required when not running --dry-run\n")
		return 2
	}

	installBranch := *branch
	if *noBranch {
		installBranch = ""
	}
	result, err := gira.InstallTemplates(*path, rendered, *overwrite, installBranch)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	fmt.Fprint(stdout, gira.FormatInstallSummary(result))
	if len(result.Conflicts) > 0 {
		return 1
	}
	return 0
}

func runOnboard(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, onboardHelp)
		return 0
	}
	if args[0] != "verify" {
		fmt.Fprintf(stderr, "unknown onboard command: %s\n\n", args[0])
		fmt.Fprint(stderr, onboardHelp)
		return 2
	}

	fs := flag.NewFlagSet("onboard verify", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	stageValue := fs.String("stage", "", "Readiness stage to verify")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON readiness artifact")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, onboardHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, onboardHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, onboardHelp)
		return 2
	}
	if *stageValue == "" {
		fmt.Fprint(stderr, "--stage is required\n\n")
		fmt.Fprint(stderr, onboardHelp)
		return 2
	}
	repo, ok := resolveRepoContext(*repoValue, stderr, onboardHelp)
	if !ok {
		return 2
	}
	stage, err := gira.ParseOnboardStage(*stageValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, onboardHelp)
		return 2
	}
	report, err := newOnboardVerifyReport(repo, stage)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode onboard verify JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
	} else {
		fmt.Fprint(stdout, gira.FormatOnboardVerifyReport(report))
	}
	if report.Ready {
		return 0
	}
	return 1
}

func runExport(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, exportHelp)
		return 0
	}
	if args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, exportHelp)
		return 0
	}
	if args[0] != "dashboard" {
		fmt.Fprintf(stderr, "unknown export command: %s\n\n", args[0])
		fmt.Fprint(stderr, exportHelp)
		return 2
	}

	return runExportDashboard(args[1:], stdout, stderr)
}

func runPortfolio(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, portfolioHelp)
		return 0
	}
	switch args[0] {
	case "capability":
		return runPortfolioCapability(args[1:], stdout, stderr)
	case "lower":
		return runPortfolioLower(args[1:], stdout, stderr)
	case "status", "validate", "plan":
		return runPortfolioCommand(args[0], args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown portfolio command: %s\n\n", args[0])
		fmt.Fprint(stderr, portfolioHelp)
		return 2
	}
}

func runWorkspace(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, workspaceHelp)
		return 0
	}
	switch args[0] {
	case "status", "backlog":
		return runWorkspaceStatus(args[0], args[1:], stdout, stderr)
	case "sync":
		return runWorkspaceSync(args[1:], stdout, stderr)
	case "ticket":
		return runWorkspaceTicket(args[1:], stdout, stderr)
	case "project":
		return runWorkspaceProject(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown workspace command: %s\n\n", args[0])
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
}

func runWorkspaceStatus(command string, args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("workspace "+command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", gira.DefaultInitConfigPath("."), "Workspace config path")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, workspaceHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
	report, err := newWorkspaceStatusReport(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if command == "backlog" {
		report.NextSteps = []string{"gira workspace status --config .gira/config.yaml"}
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode workspace JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatWorkspaceReport(report))
	return 0
}

func runWorkspaceSync(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("workspace sync", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", gira.DefaultInitConfigPath("."), "Workspace config path")
	dryRun := fs.Bool("dry-run", false, "Plan sync without mutation")
	apply := fs.Bool("apply", false, "Apply workspace sync")
	bootstrapIssues := fs.Bool("bootstrap-issues", false, "Enable bootstrap issue sync")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, workspaceHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
	if *dryRun == *apply {
		fmt.Fprintf(stderr, "exactly one of --dry-run or --apply is required for workspace sync\n\n")
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
	report, err := newWorkspaceSyncReport(*configPath, *dryRun, *bootstrapIssues)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode workspace sync JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatWorkspaceSyncReport(report))
	return 0
}

func runWorkspaceTicket(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, workspaceHelp)
		return 0
	}
	switch args[0] {
	case "new":
		return runWorkspaceTicketNew(args[1:], stdout, stderr)
	case "route":
		return runWorkspaceTicketRoute(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown workspace ticket command: %s\n\n", args[0])
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
}

func runWorkspaceTicketNew(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("workspace ticket new", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", gira.DefaultInitConfigPath("."), "Workspace config path")
	title := fs.String("title", "", "Ticket title")
	body := fs.String("body", "", "Ticket body")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, workspaceHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
	report, err := newWorkspaceTicketNewReport(*configPath, *title, *body)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode workspace ticket JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatWorkspaceTicketNewReport(report))
	return 0
}

func runWorkspaceTicketRoute(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("workspace ticket route", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", gira.DefaultInitConfigPath("."), "Workspace config path")
	ticket := fs.String("ticket", "", "Inbox ticket number")
	repoValue := fs.String("repo", "", "Target execution repo")
	dryRun := fs.Bool("dry-run", false, "Plan route without mutation")
	apply := fs.Bool("apply", false, "Apply route")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, workspaceHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
	if strings.TrimSpace(*ticket) == "" || strings.TrimSpace(*repoValue) == "" || *dryRun == *apply {
		fmt.Fprintf(stderr, "--ticket, --repo, and exactly one of --dry-run or --apply are required for workspace ticket route\n\n")
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newWorkspaceTicketRouteReport(*configPath, *ticket, repo, *dryRun)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode workspace route JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatWorkspaceTicketRouteReport(report))
	return 0
}

func runWorkspaceProject(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, workspaceHelp)
		return 0
	}
	if args[0] != "plan" {
		fmt.Fprintf(stderr, "unknown workspace project command: %s\n\n", args[0])
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
	fs := flag.NewFlagSet("workspace project plan", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", gira.DefaultInitConfigPath("."), "Workspace config path")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, workspaceHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
	report, err := newWorkspaceProjectPlanReport(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode workspace project JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	fmt.Fprintf(stdout, "workspace project plan: %d repos inspected\nnext step: keep Projects v2 read-only until workspace overview is stable\n", len(report.Repos))
	return 0
}

func runProjects(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, projectsHelp)
		return 0
	}
	if args[0] != "sync" {
		fmt.Fprintf(stderr, "unknown projects command: %s\n\n", args[0])
		fmt.Fprint(stderr, projectsHelp)
		return 2
	}
	fs := flag.NewFlagSet("projects sync", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", gira.DefaultInitConfigPath("."), "Workspace config path")
	dryRun := fs.Bool("dry-run", false, "Plan sync without mutation")
	apply := fs.Bool("apply", false, "Apply projects sync")
	archiveClosed := fs.Bool("archive-closed", false, "Archive Project items whose backing issues are closed")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, projectsHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, projectsHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, projectsHelp)
		return 2
	}
	if *dryRun == *apply {
		fmt.Fprintf(stderr, "exactly one of --dry-run or --apply is required for projects sync\n\n")
		fmt.Fprint(stderr, projectsHelp)
		return 2
	}
	report, err := newProjectsSyncReport(*configPath, *dryRun, *archiveClosed)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode projects sync JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatProjectsSyncReport(report))
	return 0
}

func runPortfolioCommand(command string, args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("portfolio "+command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", gira.DefaultInitConfigPath("."), "Portfolio config path")
	dryRun := fs.Bool("dry-run", false, "Compute plan without mutation")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, portfolioHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, portfolioHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, portfolioHelp)
		return 2
	}
	if command != "plan" && *dryRun {
		fmt.Fprintf(stderr, "--dry-run is only supported for portfolio plan\n\n")
		fmt.Fprint(stderr, portfolioHelp)
		return 2
	}
	if command == "plan" && !*dryRun {
		fmt.Fprintf(stderr, "--dry-run is required for portfolio plan\n\n")
		fmt.Fprint(stderr, portfolioHelp)
		return 2
	}
	report, err := newPortfolioReport(command, *configPath, *dryRun)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode portfolio JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		if len(report.Diagnostics) > 0 {
			return 1
		}
		if len(report.PermissionBlocks) > 0 {
			return 1
		}
		return 0
	}
	fmt.Fprint(stdout, gira.FormatPortfolioReport(report))
	if len(report.Diagnostics) > 0 {
		return 1
	}
	if len(report.PermissionBlocks) > 0 {
		return 1
	}
	return 0
}

func runPortfolioLower(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("portfolio lower", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", gira.DefaultInitConfigPath("."), "Portfolio config path")
	dryRun := fs.Bool("dry-run", false, "Plan lowering without mutation")
	apply := fs.Bool("apply", false, "Apply portfolio lowering mutations")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, portfolioHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, portfolioHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, portfolioHelp)
		return 2
	}
	if *dryRun == *apply {
		fmt.Fprintf(stderr, "exactly one of --dry-run or --apply is required for portfolio lower\n\n")
		fmt.Fprint(stderr, portfolioHelp)
		return 2
	}
	report, err := newPortfolioLowerReport(*configPath, *apply)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode portfolio lower JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		if len(report.Diagnostics) > 0 || len(report.PermissionBlocks) > 0 {
			return 1
		}
		return 0
	}
	fmt.Fprint(stdout, gira.FormatPortfolioLowerReport(report))
	if len(report.Diagnostics) > 0 || len(report.PermissionBlocks) > 0 {
		return 1
	}
	return 0
}

func runPortfolioCapability(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("portfolio capability", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", gira.DefaultInitConfigPath("."), "Portfolio config path")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, portfolioHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, portfolioHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, portfolioHelp)
		return 2
	}
	report, err := newPortfolioCapabilityReport(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode portfolio capability JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		if len(report.BlockedActions) > 0 {
			return 1
		}
		return 0
	}
	fmt.Fprint(stdout, gira.FormatPortfolioCapabilityReport(report))
	if len(report.BlockedActions) > 0 {
		return 1
	}
	return 0
}

func runExportDashboard(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("export dashboard", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	outputRoot := fs.String("output", "./out/dashboard", "Output root directory for artifacts")
	dryRun := fs.Bool("dry-run", false, "Plan export without writing artifacts")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON summary")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, exportHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, exportHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, exportHelp)
		return 2
	}
	if *repoValue == "" {
		fmt.Fprint(stderr, "--repo is required\n\n")
		fmt.Fprint(stderr, exportHelp)
		return 2
	}

	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}

	if !*dryRun {
		if outputInfo, err := os.Stat(*outputRoot); err == nil && !outputInfo.IsDir() {
			fmt.Fprintf(stderr, "output path exists but is not a directory: %s\n", *outputRoot)
			return 2
		}
	}

	client := newDashboardExportClient(repo)
	plan, bundle, err := gira.BuildDashboardExportPlan(repo, *outputRoot, dashboardExportNow(), *dryRun, client)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}

	if !*dryRun {
		if err := gira.WriteDashboardExportBundle(*outputRoot, bundle); err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
	}

	if *jsonOutput {
		output, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode export dashboard JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}

	if *dryRun {
		fmt.Fprint(stdout, gira.FormatDashboardExportPlan(plan))
		return 0
	}
	fmt.Fprintf(stdout, "export dashboard artifacts written to %s\n", *outputRoot)
	return 0
}

func runSync(args []string, stdout io.Writer, stderr io.Writer) int {
	return runSyncWithCommand(args, stdout, stderr, "gira sync")
}

func runSyncWithCommand(args []string, stdout io.Writer, stderr io.Writer, commandName string) int {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	dryRun := fs.Bool("dry-run", false, "Plan sync without creating or updating GitHub metadata")
	bootstrapIssues := fs.Bool("bootstrap-issues", false, "Enable creation of default Gira bootstrap issues")
	envPolicyMode := strings.TrimSpace(os.Getenv("GIRA_SYNC_POLICY_MODE"))
	policyModeValue := fs.String("policy-mode", envPolicyMode, "Metadata policy mode (adopt|merge|enforce)")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, syncHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, syncHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, syncHelp)
		return 2
	}
	pinPolicyMode := envPolicyMode != ""
	fs.Visit(func(flag *flag.Flag) {
		if flag.Name == "policy-mode" {
			pinPolicyMode = true
		}
	})
	if *repoValue == "" {
		fmt.Fprint(stderr, "--repo is required\n\n")
		fmt.Fprint(stderr, syncHelp)
		return 2
	}

	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}

	client := newSyncClient(repo)
	mode, err := gira.ParseSyncPolicyMode(*policyModeValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	plan, err := gira.BuildSyncPlan(client, gira.SyncPlanOptions{EnableBootstrapIssues: *bootstrapIssues, PolicyMode: mode})
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	fmt.Fprint(stdout, gira.FormatSyncPlan(plan, *dryRun))
	if !*dryRun {
		if err := gira.ApplySyncPlan(client, plan); err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
		record := gira.NewAuditRecord("sync", "sha256:sync-plan", "metadata_sync:apply", repo.FullName(), "ok", "", "allowed", time.Now())
		if err := appendAuditRecord(repo, record); err != nil {
			fmt.Fprintf(stderr, "audit write failed: %v\n", err)
			return 2
		}
		fmt.Fprintln(stdout, "sync complete")
	}
	fmt.Fprintln(stdout, syncNextStep(commandName, repo, *dryRun, *bootstrapIssues, mode, pinPolicyMode))
	return 0
}

func syncNextStep(commandName string, repo gira.RepoRef, dryRun bool, bootstrapIssues bool, mode gira.SyncPolicyMode, pinPolicyMode bool) string {
	command := commandName + " --repo " + repo.FullName()
	if mode != "" && (pinPolicyMode || mode != gira.SyncPolicyMerge) {
		command += " --policy-mode " + string(mode)
	}
	if bootstrapIssues {
		command += " --bootstrap-issues"
	}
	if dryRun {
		return "next step: " + command
	}
	return "next step: gira status --repo " + repo.FullName()
}

func runDetach(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("detach", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	dryRun := fs.Bool("dry-run", false, "Plan safe detach actions without mutation")
	applyMode := fs.Bool("apply", false, "Apply only planned safe GitHub detach actions")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, detachHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, detachHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, detachHelp)
		return 2
	}
	if *repoValue == "" {
		fmt.Fprint(stderr, "--repo is required\n\n")
		fmt.Fprint(stderr, detachHelp)
		return 2
	}
	if (*dryRun && *applyMode) || (!*dryRun && !*applyMode) {
		fmt.Fprint(stderr, "exactly one of --dry-run or --apply is required\n\n")
		fmt.Fprint(stderr, detachHelp)
		return 2
	}

	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}

	report, err := newDetachReport(repo, *dryRun, *applyMode)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode detach JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatDetachReport(report))
	return 0
}

func runGuardrails(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, guardrailsHelp)
		return 0
	}
	if args[0] != "sync" {
		fmt.Fprintf(stderr, "unknown guardrails command: %s\n\n", args[0])
		fmt.Fprint(stderr, guardrailsHelp)
		return 2
	}
	fs := flag.NewFlagSet("guardrails sync", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	policyPath := fs.String("policy", "", "Guardrails policy file path")
	dryRun := fs.Bool("dry-run", false, "Compute deterministic full diff only")
	applyMode := fs.Bool("apply", false, "Apply policy-owned settings only")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON summary")
	allowRelaxation := fs.Bool("allow-relaxation", false, "Allow relaxation changes")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, guardrailsHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, guardrailsHelp)
		return 0
	}
	if *repoValue == "" || *policyPath == "" {
		fmt.Fprint(stderr, "--repo and --policy are required\n\n")
		fmt.Fprint(stderr, guardrailsHelp)
		return 2
	}
	if (*dryRun && *applyMode) || (!*dryRun && !*applyMode) {
		fmt.Fprint(stderr, "exactly one of --dry-run or --apply is required\n\n")
		fmt.Fprint(stderr, guardrailsHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newGuardrailsSyncReport(repo, *policyPath, *applyMode, *allowRelaxation)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode guardrails JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	output, _ := json.MarshalIndent(report, "", "  ")
	fmt.Fprintf(stdout, "%s\n", output)
	return 0
}

func runProject(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprint(stdout, projectHelp)
		return 0
	}
	if len(args) == 0 {
		fmt.Fprint(stderr, projectHelp)
		return 2
	}

	switch args[0] {
	case "capability":
		return runProjectCapability(args[1:], stdout, stderr)
	case "sync":
		return runProjectSync(args[1:], stdout, stderr)
	case "transitions":
		return runProjectTransitions(args[1:], stdout, stderr)
	case "-h", "--help":
		fmt.Fprint(stdout, projectHelp)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown project command: %s\n\n", args[0])
		fmt.Fprint(stderr, projectHelp)
		return 2
	}
}

func runProjectSync(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("project sync", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	dryRun := fs.Bool("dry-run", false, "Read-only report mode")
	applyMode := fs.Bool("apply", false, "Apply capability-gated status update workflow")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON summary")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, projectHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, projectHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, projectHelp)
		return 2
	}
	if *repoValue == "" {
		fmt.Fprint(stderr, "--repo is required\n\n")
		fmt.Fprint(stderr, projectHelp)
		return 2
	}
	if (*dryRun && *applyMode) || (!*dryRun && !*applyMode) {
		fmt.Fprint(stderr, "exactly one of --dry-run or --apply is required\n\n")
		fmt.Fprint(stderr, projectHelp)
		return 2
	}

	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}

	if *applyMode {
		applyReport, err := newProjectSyncApplyReport(repo)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
		record := gira.NewAuditRecord("project sync", "sha256:project-sync-policy", "project_status_field:update", repo.FullName(), "ok", "", "capability_gated", time.Now())
		if err := appendAuditRecord(repo, record); err != nil {
			fmt.Fprintf(stderr, "audit write failed: %v\n", err)
			return 2
		}
		if *jsonOutput {
			output, err := json.MarshalIndent(applyReport, "", "  ")
			if err != nil {
				fmt.Fprintf(stderr, "encode project sync apply JSON: %v\n", err)
				return 2
			}
			fmt.Fprintf(stdout, "%s\n", output)
			return 0
		}
		fmt.Fprintln(stdout, "project sync apply completed")
		return 0
	}

	report, err := newProjectSyncReport(repo, *dryRun)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report.Command = "project sync"
	report.DryRun = *dryRun

	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode project sync JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}

	fmt.Fprint(stdout, gira.FormatProjectSyncPlan(report))
	return 0
}

func runProjectTransitions(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("project transitions", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	dryRun := fs.Bool("dry-run", false, "Read-only plan mode")
	applyMode := fs.Bool("apply", false, "Apply issue-level status label updates")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON summary")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, projectHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, projectHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, projectHelp)
		return 2
	}
	if *repoValue == "" {
		fmt.Fprint(stderr, "--repo is required\n\n")
		fmt.Fprint(stderr, projectHelp)
		return 2
	}
	if (*dryRun && *applyMode) || (!*dryRun && !*applyMode) {
		fmt.Fprint(stderr, "exactly one of --dry-run or --apply is required\n\n")
		fmt.Fprint(stderr, projectHelp)
		return 2
	}

	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}

	if *applyMode {
		applyReport, err := newProjectTransitionsApplyReport(repo)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
		record := gira.NewAuditRecord("project transitions", "sha256:project-transitions-policy", "issue_status_transition:apply", repo.FullName(), "ok", "", "rule_matrix", time.Now())
		if err := appendAuditRecord(repo, record); err != nil {
			fmt.Fprintf(stderr, "audit write failed: %v\n", err)
			return 2
		}
		if *jsonOutput {
			output, err := json.MarshalIndent(applyReport, "", "  ")
			if err != nil {
				fmt.Fprintf(stderr, "encode project transitions apply JSON: %v\n", err)
				return 2
			}
			fmt.Fprintf(stdout, "%s\n", output)
			return 0
		}
		text, err := json.MarshalIndent(applyReport, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode project transitions apply JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", text)
		return 0
	}

	report, err := newProjectTransitionsReport(repo, *dryRun)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report.Command = "project transitions"
	report.DryRun = *dryRun

	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode project transitions JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}

	fmt.Fprint(stdout, gira.FormatProjectTransitionsPlan(report))
	return 0
}

func runProjectCapability(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("capability", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON summary")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, projectHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, projectHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, projectHelp)
		return 2
	}
	if *repoValue == "" {
		fmt.Fprint(stderr, "--repo is required\n\n")
		fmt.Fprint(stderr, projectHelp)
		return 2
	}

	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}

	report, err := newProjectCapabilityReport(repo)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report.Command = "project capability"
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode project capability JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}

	fmt.Fprint(stdout, gira.FormatProjectCapabilitySummary(report))
	return 0
}

func runStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON for automation")
	staleDays := fs.Int("stale-days", 14, "Days since update before open issues count as stale")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, statusHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, statusHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, statusHelp)
		return 2
	}
	if *staleDays < 1 {
		fmt.Fprint(stderr, "--stale-days must be at least 1\n\n")
		fmt.Fprint(stderr, statusHelp)
		return 2
	}

	repo, ok := resolveRepoContext(*repoValue, stderr, statusHelp)
	if !ok {
		return 2
	}

	summary, err := gira.BuildStatusSummary(newStatusClient(repo), statusNow(), *staleDays)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode status JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatStatusText(summary))
	return 0
}

func runTriage(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("triage", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	dryRun := fs.Bool("dry-run", false, "Preview only")
	apply := fs.Bool("apply", false, "Apply labels")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, triageHelp)
			return 0
		}
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, triageHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, triageHelp)
		return 0
	}
	if *repoValue == "" || (*dryRun == *apply) {
		fmt.Fprint(stderr, "--repo and exactly one of --dry-run/--apply are required\n\n")
		fmt.Fprint(stderr, triageHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newTriageReport(repo, *apply)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	output, _ := json.MarshalIndent(report, "", "  ")
	if *jsonOutput {
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	fmt.Fprintf(stdout, "%s\n", output)
	return 0
}

func runSprint(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, sprintHelp)
		return 0
	}
	switch args[0] {
	case "plan":
		fs := flag.NewFlagSet("sprint plan", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
		iteration := fs.String("iteration", "", "Iteration identifier")
		capacity := fs.Int("capacity", 0, "Capacity target")
		issues := fs.String("issues", "", "Comma-separated committed issue numbers")
		dryRun := fs.Bool("dry-run", false, "Preview only")
		apply := fs.Bool("apply", false, "Persist plan")
		_ = fs.Bool("json", false, "Emit stable JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			fmt.Fprintf(stderr, "%v\n\n", err)
			fmt.Fprint(stderr, sprintHelp)
			return 2
		}
		if *repoValue == "" || *iteration == "" || *capacity <= 0 || (*dryRun == *apply) {
			fmt.Fprint(stderr, "--repo, --iteration, --capacity and exactly one of --dry-run/--apply are required\n\n")
			fmt.Fprint(stderr, sprintHelp)
			return 2
		}
		repo, err := gira.ParseRepoRef(*repoValue)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
		committed, err := parseCSVInts(*issues)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
		report, err := gira.PlanSprint(gira.SprintStatePath(repo), repo, *iteration, *capacity, committed, *apply)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		output, _ := json.MarshalIndent(report, "", "  ")
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	case "start":
		fs := flag.NewFlagSet("sprint start", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
		iteration := fs.String("iteration", "", "Iteration identifier")
		dryRun := fs.Bool("dry-run", false, "Preview only")
		apply := fs.Bool("apply", false, "Start sprint")
		jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			fmt.Fprintf(stderr, "%v\n\n", err)
			fmt.Fprint(stderr, sprintHelp)
			return 2
		}
		if *repoValue == "" || *iteration == "" || (*dryRun == *apply) {
			fmt.Fprint(stderr, "--repo, --iteration and exactly one of --dry-run/--apply are required\n\n")
			fmt.Fprint(stderr, sprintHelp)
			return 2
		}
		repo, err := gira.ParseRepoRef(*repoValue)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
		report, err := gira.StartSprint(gira.SprintStatePath(repo), repo, *iteration, *apply, time.Now())
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		output, _ := json.MarshalIndent(report, "", "  ")
		if *jsonOutput {
			fmt.Fprintf(stdout, "%s\n", output)
			return 0
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	case "close":
		fs := flag.NewFlagSet("sprint close", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
		iteration := fs.String("iteration", "", "Iteration identifier")
		completed := fs.String("completed", "", "Comma-separated completed issue numbers")
		disposition := fs.String("spillover-disposition", "", "carry or drop")
		reason := fs.String("rollover-reason", "", "Why spillover occurred")
		dryRun := fs.Bool("dry-run", false, "Preview only")
		apply := fs.Bool("apply", false, "Close sprint")
		jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			fmt.Fprintf(stderr, "%v\n\n", err)
			fmt.Fprint(stderr, sprintHelp)
			return 2
		}
		if *repoValue == "" || *iteration == "" || *disposition == "" || *reason == "" || (*dryRun == *apply) {
			fmt.Fprint(stderr, "--repo, --iteration, --spillover-disposition, --rollover-reason and exactly one of --dry-run/--apply are required\n\n")
			fmt.Fprint(stderr, sprintHelp)
			return 2
		}
		repo, err := gira.ParseRepoRef(*repoValue)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
		completedItems, err := parseCSVInts(*completed)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
		report, err := gira.CloseSprint(gira.SprintStatePath(repo), repo, *iteration, completedItems, *disposition, *reason, *apply, time.Now())
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		output, _ := json.MarshalIndent(report, "", "  ")
		if *jsonOutput {
			fmt.Fprintf(stdout, "%s\n", output)
			return 0
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	case "rollover":
		fs := flag.NewFlagSet("sprint rollover", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
		toMilestone := fs.String("to", "", "Destination milestone title")
		dryRun := fs.Bool("dry-run", false, "Preview only")
		apply := fs.Bool("apply", false, "Apply rollover")
		jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			fmt.Fprintf(stderr, "%v\n\n", err)
			fmt.Fprint(stderr, sprintHelp)
			return 2
		}
		if *repoValue == "" || (*dryRun == *apply) {
			fmt.Fprint(stderr, "--repo and exactly one of --dry-run/--apply are required\n\n")
			fmt.Fprint(stderr, sprintHelp)
			return 2
		}
		repo, err := gira.ParseRepoRef(*repoValue)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
		report, err := gira.SprintRollover(repo, *toMilestone, *apply, time.Now(), gira.ExecCommandRunner{})
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		output, _ := json.MarshalIndent(report, "", "  ")
		if *jsonOutput {
			fmt.Fprintf(stdout, "%s\n", output)
			return 0
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown sprint command: %s\n\n", args[0])
		fmt.Fprint(stderr, sprintHelp)
		return 2
	}
}

func runWorker(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, workerHelp)
		return 0
	}
	switch args[0] {
	case "claim":
		return runWorkerClaim(args[1:], stdout, stderr)
	case "handoff":
		return runWorkerHandoff(args[1:], stdout, stderr)
	case "release":
		return runWorkerRelease(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown worker command: %s\n\n", args[0])
		fmt.Fprint(stderr, workerHelp)
		return 2
	}
}

func runWorkerClaim(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("worker claim", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo")
	issue := fs.Int("issue", 0, "Issue number")
	worker := fs.String("worker", "", "Worker name")
	leaseMinutes := fs.Int("lease-minutes", 30, "Lease TTL in minutes")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, workerHelp)
		return 2
	}
	if *repoValue == "" || *issue <= 0 || strings.TrimSpace(*worker) == "" {
		fmt.Fprint(stderr, "--repo, --issue, --worker are required\n\n")
		fmt.Fprint(stderr, workerHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	claim := gira.WorkerClaim{Repo: repo.FullName(), IssueNumber: *issue, Worker: *worker, LeaseUntilUTC: time.Now().UTC().Add(time.Duration(*leaseMinutes) * time.Minute), Version: gira.WorkerHandoffSchemaVersion}
	path := gira.WorkerStatePath(repo, *issue)
	if err := gira.ClaimWorkerLease(path, claim, time.Now().UTC()); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	record := gira.NewAuditRecord("worker claim", "sha256:worker-state", "worker_claim", fmt.Sprintf("%s#%d", repo.FullName(), *issue), "ok", "", "allowed", time.Now())
	if err := appendAuditRecord(repo, record); err != nil {
		fmt.Fprintf(stderr, "audit write failed: %v\n", err)
		return 2
	}
	fmt.Fprintln(stdout, "worker claim: ok")
	return 0
}

func runWorkerHandoff(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("worker handoff", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo")
	issue := fs.Int("issue", 0, "Issue number")
	goal := fs.String("goal", "", "Goal")
	context := fs.String("context", "", "Context")
	acceptance := fs.String("acceptance", "", "Semicolon-separated acceptance criteria")
	verify := fs.String("verify", "", "Semicolon-separated verification commands")
	rollback := fs.String("rollback", "", "Rollback notes")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, workerHelp)
		return 2
	}
	if *repoValue == "" || *issue <= 0 {
		fmt.Fprint(stderr, "--repo and --issue are required\n\n")
		fmt.Fprint(stderr, workerHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	payload := gira.WorkerHandoffPayload{SchemaVersion: gira.WorkerHandoffSchemaVersion, Goal: *goal, Context: *context, AcceptanceCriteria: splitList(*acceptance), VerificationCommands: splitList(*verify), RollbackNotes: *rollback}
	path := gira.WorkerStatePath(repo, *issue)
	if err := gira.WriteWorkerHandoff(path, payload); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "worker handoff: ok")
	return 0
}

func runWorkerRelease(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("worker release", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo")
	issue := fs.Int("issue", 0, "Issue number")
	worker := fs.String("worker", "", "Worker name")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, workerHelp)
		return 2
	}
	if *repoValue == "" || *issue <= 0 || strings.TrimSpace(*worker) == "" {
		fmt.Fprint(stderr, "--repo, --issue, --worker are required\n\n")
		fmt.Fprint(stderr, workerHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	path := gira.WorkerStatePath(repo, *issue)
	if err := gira.ReleaseWorkerLease(path, *worker); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	record := gira.NewAuditRecord("worker release", "sha256:worker-state", "worker_release", fmt.Sprintf("%s#%d", repo.FullName(), *issue), "ok", "", "allowed", time.Now())
	if err := appendAuditRecord(repo, record); err != nil {
		fmt.Fprintf(stderr, "audit write failed: %v\n", err)
		return 2
	}
	fmt.Fprintln(stdout, "worker release: ok")
	return 0
}

func parseCSVInts(value string) ([]int, error) {
	if strings.TrimSpace(value) == "" {
		return []int{}, nil
	}
	parts := strings.Split(value, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid integer %q", p)
		}
		out = append(out, n)
	}
	return out, nil
}

func splitList(value string) []string {
	parts := strings.Split(value, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

type repeatedStringFlag []string

func (f *repeatedStringFlag) String() string {
	if f == nil {
		return ""
	}
	return strings.Join(*f, ",")
}

func (f *repeatedStringFlag) Set(value string) error {
	*f = append(*f, strings.TrimSpace(value))
	return nil
}

func appendAuditRecord(repo gira.RepoRef, record gira.AuditRecord) error {
	path := fmt.Sprintf(".gira/audit/%s_%s.jsonl", repo.Owner, repo.Name)
	if strings.HasSuffix(os.Args[0], ".test") {
		path = filepath.Join(os.TempDir(), "gira-audit-test", fmt.Sprintf("%s_%s.jsonl", repo.Owner, repo.Name))
	}
	return gira.AppendAuditRecords(path, []gira.AuditRecord{record})
}

func runAudit(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, auditHelp)
		return 0
	}
	if args[0] != "verify" {
		fmt.Fprintf(stderr, "unknown audit command: %s\n\n", args[0])
		fmt.Fprint(stderr, auditHelp)
		return 2
	}
	return runAuditVerify(args[1:], stdout, stderr)
}

func runAuditVerify(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("audit verify", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	ledgerPath := fs.String("path", ".gira/audit/*.jsonl", "Glob path to audit JSONL files")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON summary")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, auditHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, auditHelp)
		return 0
	}
	if *repoValue == "" {
		fmt.Fprint(stderr, "--repo is required\n\n")
		fmt.Fprint(stderr, auditHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report := gira.VerifyAuditLedgerForRepo(*ledgerPath, repo)
	if *jsonOutput {
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode audit verify JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
	} else if report.Valid {
		fmt.Fprintf(stdout, "audit verify: ok (%d records)\n", report.Records)
	} else {
		fmt.Fprintf(stdout, "audit verify: failed (%s)\n", report.Failure)
	}
	if !report.Valid {
		return 1
	}
	return 0
}

func runGraph(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, graphHelp)
		return 0
	}
	if args[0] != "validate" {
		fmt.Fprintf(stderr, "unknown graph command: %s\n\n", args[0])
		fmt.Fprint(stderr, graphHelp)
		return 2
	}
	fs := flag.NewFlagSet("graph validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, graphHelp)
		return 2
	}
	if *repoValue == "" {
		fmt.Fprint(stderr, "--repo is required\n\n")
		fmt.Fprint(stderr, graphHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := gira.BuildGraphValidationReportForClient(newGraphClient(repo))
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "encode graph JSON: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "%s\n", out)
	_ = jsonOutput
	if report.Counts.Diagnostics > 0 {
		return 1
	}
	return 0
}

func runReview(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, reviewHelp)
		return 0
	}
	if args[0] == "gate" {
		fs := flag.NewFlagSet("review gate", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			fmt.Fprintf(stderr, "%v\n\n", err)
			fmt.Fprint(stderr, reviewHelp)
			return 2
		}
		report := gira.RunQualityGate(reviewGateRunner)
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode review gate JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		if !*jsonOutput {
			// default output is JSON to remain machine-readable
		}
		if !report.Ready {
			return 1
		}
		return 0
	}
	if args[0] != "queue" {
		fmt.Fprintf(stderr, "unknown review command: %s\n\n", args[0])
		fmt.Fprint(stderr, reviewHelp)
		return 2
	}
	fs := flag.NewFlagSet("review queue", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, reviewHelp)
		return 2
	}
	repo, ok := resolveRepoContext(*repoValue, stderr, reviewHelp)
	if !ok {
		return 2
	}
	report, err := gira.BuildReviewQueue(newReviewGateClient(repo), time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "encode review queue JSON: %v\n", err)
		return 2
	}
	if *jsonOutput {
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatReviewQueueText(report))
	return 0
}

func runMerge(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, mergeHelp)
		return 0
	}
	if args[0] != "queue" {
		fmt.Fprintf(stderr, "unknown merge command: %s\n\n", args[0])
		fmt.Fprint(stderr, mergeHelp)
		return 2
	}
	fs := flag.NewFlagSet("merge queue", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	dryRun := fs.Bool("dry-run", false, "Preview only")
	apply := fs.Bool("apply", false, "Apply merges")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, mergeHelp)
		return 2
	}
	if *repoValue == "" || (*dryRun == *apply) {
		fmt.Fprint(stderr, "--repo and exactly one of --dry-run/--apply are required\n\n")
		fmt.Fprint(stderr, mergeHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := gira.BuildMergeQueue(newReviewGateClient(repo), time.Now(), *apply)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	out, _ := json.MarshalIndent(report, "", "  ")
	if *jsonOutput {
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprintf(stdout, "%s\n", out)
	return 0
}

func runRelease(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, releaseHelp)
		return 0
	}
	if args[0] != "readiness" {
		fmt.Fprintf(stderr, "unknown release command: %s\n\n", args[0])
		fmt.Fprint(stderr, releaseHelp)
		return 2
	}
	fs := flag.NewFlagSet("release readiness", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, releaseHelp)
		return 2
	}
	if *repoValue == "" {
		fmt.Fprint(stderr, "--repo is required\n\n")
		fmt.Fprint(stderr, releaseHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := gira.BuildReleaseReadiness(newReviewGateClient(repo), time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	out, _ := json.MarshalIndent(report, "", "  ")
	if *jsonOutput {
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprintf(stdout, "%s\n", out)
	if !report.Ready {
		return 1
	}
	return 0
}

func runContract(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, contractHelp)
		return 0
	}
	if args[0] != "crud" {
		fmt.Fprintf(stderr, "unsupported contract operation: %s\n", args[0])
		fmt.Fprint(stderr, "supported operation: gira contract crud\n\n")
		fmt.Fprint(stderr, contractHelp)
		return 2
	}

	fmt.Fprint(stdout, `CRUD capability matrix (MVP contract)

surface               create                                           read                                                                                     update                                                            delete
labels                gira sync --repo OWNER/REPO                      gira sync --repo OWNER/REPO --dry-run                                                    gira sync --repo OWNER/REPO                                       unsupported (intentional in MVP)
milestones            gira sync --repo OWNER/REPO                      gira sync --repo OWNER/REPO --dry-run                                                    gira sync --repo OWNER/REPO                                       unsupported (intentional in MVP)
issues                gira sync --repo OWNER/REPO --bootstrap-issues   gira status --repo OWNER/REPO                                                            gira triage apply --apply / gira worker claim|handoff|release     unsupported direct delete in MVP
pr_loop               gira dev pr open --repo OWNER/REPO --issue N     gira dev pr status --repo OWNER/REPO --issue N / gira review queue                        gira merge queue --apply (opt-in destructive)                     unsupported direct delete; close via GitHub UI/API
project_fields_views  unsupported (MVP non-goal)                       gira project capability / gira project sync --dry-run / gira project transitions --dry-run unsupported in MVP (dry-run inspection only)                      unsupported (MVP non-goal)
`)
	return 0
}

func runReport(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, reportHelp)
		return 0
	}
	if args[0] != "weekly" {
		fmt.Fprintf(stderr, "unknown report command: %s\n\n", args[0])
		fmt.Fprint(stderr, reportHelp)
		return 2
	}
	fs := flag.NewFlagSet("report weekly", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON summary")
	mdOutput := fs.Bool("md", false, "Emit markdown report")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, reportHelp)
		return 2
	}
	repo, ok := resolveRepoContext(*repoValue, stderr, reportHelp)
	if !ok {
		return 2
	}
	report, err := gira.BuildWeeklyReport(repo, reportNow(), newDashboardExportClient(repo), newReviewGateClient(repo))
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode report JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	if *mdOutput || !*jsonOutput {
		fmt.Fprint(stdout, gira.FormatWeeklyReportMarkdown(report))
		return 0
	}
	return 0
}
