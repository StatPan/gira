package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/StatPan/gira/internal/gira"
)

const rootHelp = `Gira: GitHub-native project OS bootstrapper.

Usage:
  gira <command> [flags]

Commands:
  bootstrap   Bootstrap a repository into a Gira-managed project workspace
  sync        Sync Gira labels, milestones, and optionally bootstrap issues through gh
  status      Show a compact read-only GitHub status summary
  export      Export dashboard artifacts from read-only GitHub data
  project     Inspect permission capability for Project OS lifecycle actions

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

const statusHelp = `Show a compact read-only status summary from GitHub issues and milestones.

Usage:
  gira status --repo OWNER/REPO [--json] [--stale-days N]

Flags:
  --repo string       Target GitHub repo in OWNER/REPO format
  --json              Emit stable JSON for automation
  --stale-days int    Days since update before open issues count as stale (default 14)
  -h, --help          Show help
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

var newStatusClient = func(repo gira.RepoRef) gira.StatusClient {
	return gira.NewGHStatusClient(repo, gira.ExecCommandRunner{})
}

var newSyncClient = func(repo gira.RepoRef) gira.SyncClient {
	return gira.NewGHSyncClient(repo, gira.ExecCommandRunner{})
}

var newProjectCapabilityReport = func(repo gira.RepoRef) (gira.ProjectCapabilityReport, error) {
	return gira.BuildProjectCapabilityReport(repo, gira.ExecCommandRunner{})
}

var newProjectSyncReport = func(repo gira.RepoRef, dryRun bool) (gira.ProjectSyncReport, error) {
	if !dryRun {
		return gira.ProjectSyncReport{}, fmt.Errorf("--dry-run is required for project sync in this slice")
	}
	return gira.BuildProjectSyncReportForClient(gira.NewGHProjectSyncClient(repo, gira.ExecCommandRunner{}), time.Now())
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

var dashboardExportNow = func() time.Time {
	return time.Now()
}

var statusNow = func() time.Time {
	return time.Now()
}

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, rootHelp)
		return 0
	}

	switch args[0] {
	case "bootstrap":
		return runBootstrap(args[1:], stdout, stderr)
	case "sync":
		return runSync(args[1:], stdout, stderr)
	case "status":
		return runStatus(args[1:], stdout, stderr)
	case "export":
		return runExport(args[1:], stdout, stderr)
	case "project":
		return runProject(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		fmt.Fprint(stderr, rootHelp)
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
		fmt.Fprintln(stdout, "sync complete")
	}
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
	dryRun := fs.Bool("dry-run", false, "Required for this read-only slice")
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
	if !*dryRun {
		fmt.Fprint(stderr, "--dry-run is required\n\n")
		fmt.Fprint(stderr, projectHelp)
		return 2
	}

	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
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
