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

const rootHelp = `Gira: GitHub-native project OS bootstrapper.

Usage:
  gira <command> [flags]

Commands:
  init        One-command onboarding with prerequisite checks and next-step plan
  bootstrap   Bootstrap a repository into a Gira-managed project workspace
  onboard     Verify onboarding readiness from init to steady-state
  dev         Issue to branch execution helpers
  sync        Sync Gira labels, milestones, and optionally bootstrap issues through gh
  status      Show a compact read-only GitHub status summary
  export      Export dashboard artifacts from read-only GitHub data
  parity      Compute deterministic Jira-replacement parity scorecard
  project     Inspect permission capability for Project OS lifecycle actions
  audit       Verify audit ledgers for mutation integrity
  worker      Manage worker claim/handoff/release state for issues
  guardrails  Audit and apply branch protection/ruleset policy
  triage      Backlog triage queue and policy apply helpers
  sprint      Sprint iteration planning/start/close workflow
  graph       Work graph validation (parent/depends_on/blocks)
  review      Review routing queue with stale-review detection
  merge       Policy-checked merge queue (dry-run/apply)
  release     Release readiness gate report
  report      Weekly PM cockpit report

Flags:
  -h, --help  Show help
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
  --config string     Optional init profile schema path (.gira/config.yaml)
  --dry-run           Emit plan only (default true for this planning slice)
  --json              Emit stable JSON report for automation
  -h, --help          Show help
`

const statusHelp = `Show a compact read-only status summary from GitHub issues and milestones.

Usage:
  gira status --repo OWNER/REPO [--json] [--stale-days N]

Flags:
  --repo string       Target GitHub repo in OWNER/REPO format
  --json              Emit stable JSON for automation
  --stale-days int    Days since update before open issues count as stale (default 14)
  -h, --help          Show help
`

const onboardHelp = `Verify onboarding readiness from init to daily operation.

Usage:
  gira onboard verify --repo OWNER/REPO --stage init|bootstrap|first-sprint|steady-state [--json]

Commands:
  verify       Run prerequisite, bootstrap, metadata, and daily-run checks

Flags:
  --repo string   Target GitHub repo in OWNER/REPO format
  --stage string  Readiness stage to verify
  --json          Emit stable JSON readiness artifact
  -h, --help      Show help
`

const syncHelp = `Sync Gira labels, milestones, and optionally bootstrap issues through gh.

Usage:
  gira sync --repo OWNER/REPO [--dry-run] [--bootstrap-issues]

Flags:
  --repo string              Target GitHub repo in OWNER/REPO format
  --dry-run                  Plan sync without creating or updating GitHub metadata
  --bootstrap-issues         Enable creation of default Gira bootstrap issues
  -h, --help                 Show help
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

const triageHelp = `Backlog triage queue and policy apply helpers.

Usage:
  gira triage queue --repo OWNER/REPO [--json]
  gira triage apply --repo OWNER/REPO --policy FILE --dry-run|--apply [--json]
`

const sprintHelp = `Sprint/iteration command family.

Usage:
  gira sprint plan --repo OWNER/REPO --iteration ID --capacity N --issues 1,2,3 --dry-run|--apply [--json]
  gira sprint start --repo OWNER/REPO --iteration ID --dry-run|--apply [--json]
  gira sprint close --repo OWNER/REPO --iteration ID --completed 1,2 --spillover-disposition carry|drop --rollover-reason TEXT --dry-run|--apply [--json]
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
  gira review queue --repo OWNER/REPO [--json]
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
  gira report weekly --repo OWNER/REPO [--json|--md]
`

const devHelp = `Developer workflow helpers for issue-to-branch execution.

Usage:
  gira dev start --repo OWNER/REPO --issue N [--dry-run] [--json] [--force] [--branch-pattern "issue-%d-%s"]
  gira dev pr open --repo OWNER/REPO --issue N [--json]
  gira dev pr status --repo OWNER/REPO --issue N [--json]
`

var newStatusClient = func(repo gira.RepoRef) gira.StatusClient {
	return gira.NewGHStatusClient(repo, gira.ExecCommandRunner{})
}

var newOnboardVerifyReport = func(repo gira.RepoRef, stage gira.OnboardStage) (gira.OnboardVerifyReport, error) {
	return gira.BuildOnboardVerifyReport(repo, stage, gira.ExecCommandRunner{}, time.Now().UTC()), nil
}

var newSyncClient = func(repo gira.RepoRef) gira.SyncClient {
	return gira.NewGHSyncClient(repo, gira.ExecCommandRunner{})
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

var newGraphClient = func(repo gira.RepoRef) gira.GraphClient {
	return gira.NewGHGraphClient(repo, gira.ExecCommandRunner{})
}

var newReviewGateClient = func(repo gira.RepoRef) gira.ReviewGateClient {
	client := gira.NewGHReviewGateClient(repo, gira.ExecCommandRunner{})
	return client
}

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

var dashboardExportNow = func() time.Time {
	return time.Now()
}

var statusNow = func() time.Time {
	return time.Now()
}

var reportNow = func() time.Time {
	return time.Now()
}

var devCommandRunner gira.CommandRunner = gira.ExecCommandRunner{}

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, rootHelp)
		return 0
	}

	switch args[0] {
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "bootstrap":
		return runBootstrap(args[1:], stdout, stderr)
	case "onboard":
		return runOnboard(args[1:], stdout, stderr)
	case "dev":
		return runDev(args[1:], stdout, stderr)
	case "sync":
		return runSync(args[1:], stdout, stderr)
	case "status":
		return runStatus(args[1:], stdout, stderr)
	case "export":
		return runExport(args[1:], stdout, stderr)
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
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		fmt.Fprint(stderr, rootHelp)
		return 2
	}
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

	report, err := gira.BuildInitReport(repo, *pathValue, *dryRun, devCommandRunner)
	if loadedConfigPath != "" {
		report.ConfigPath = loadedConfigPath
		report.ConfigProfileCount = len(loadedConfig.Profiles)
	}
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
	fmt.Fprintf(stdout, "%s\n", out)
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
	if *repoValue == "" || *stageValue == "" {
		fmt.Fprint(stderr, "--repo and --stage are required\n\n")
		fmt.Fprint(stderr, onboardHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
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
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	dryRun := fs.Bool("dry-run", false, "Plan sync without creating or updating GitHub metadata")
	bootstrapIssues := fs.Bool("bootstrap-issues", false, "Enable creation of default Gira bootstrap issues")
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
	plan, err := gira.BuildSyncPlan(client, gira.SyncPlanOptions{EnableBootstrapIssues: *bootstrapIssues})
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
	if *repoValue == "" {
		fmt.Fprint(stderr, "--repo is required\n\n")
		fmt.Fprint(stderr, statusHelp)
		return 2
	}
	if *staleDays < 1 {
		fmt.Fprint(stderr, "--stale-days must be at least 1\n\n")
		fmt.Fprint(stderr, statusHelp)
		return 2
	}

	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
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
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, triageHelp)
		return 0
	}
	switch args[0] {
	case "queue":
		fs := flag.NewFlagSet("triage queue", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
		jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			fmt.Fprintf(stderr, "%v\n\n", err)
			fmt.Fprint(stderr, triageHelp)
			return 2
		}
		if *repoValue == "" {
			fmt.Fprint(stderr, "--repo is required\n\n")
			fmt.Fprint(stderr, triageHelp)
			return 2
		}
		repo, err := gira.ParseRepoRef(*repoValue)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
		report, err := gira.BuildTriageQueue(gira.NewGHTriageClient(repo, gira.ExecCommandRunner{}), time.Now())
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
		output, _ := json.MarshalIndent(report, "", "  ")
		if *jsonOutput {
			fmt.Fprintf(stdout, "%s\n", output)
			return 0
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	case "apply":
		fs := flag.NewFlagSet("triage apply", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
		policyPath := fs.String("policy", "", "Policy file path")
		dryRun := fs.Bool("dry-run", false, "Preview only")
		apply := fs.Bool("apply", false, "Apply labels")
		jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			fmt.Fprintf(stderr, "%v\n\n", err)
			fmt.Fprint(stderr, triageHelp)
			return 2
		}
		if *repoValue == "" || *policyPath == "" || (*dryRun == *apply) {
			fmt.Fprint(stderr, "--repo, --policy and exactly one of --dry-run/--apply are required\n\n")
			fmt.Fprint(stderr, triageHelp)
			return 2
		}
		repo, err := gira.ParseRepoRef(*repoValue)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
		policy, err := gira.LoadTriagePolicy(*policyPath)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		report, err := gira.ApplyTriagePolicy(gira.NewGHTriageClient(repo, gira.ExecCommandRunner{}), policy, *apply, time.Now())
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
		fmt.Fprintf(stderr, "unknown triage command: %s\n\n", args[0])
		fmt.Fprint(stderr, triageHelp)
		return 2
	}
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
	if *repoValue == "" {
		fmt.Fprint(stderr, "--repo is required\n\n")
		fmt.Fprint(stderr, reviewHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := gira.BuildReviewQueue(newReviewGateClient(repo), time.Now())
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
	if *repoValue == "" {
		fmt.Fprint(stderr, "--repo is required\n\n")
		fmt.Fprint(stderr, reportHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
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
