package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/StatPan/gira/internal/gira"
)

const rootHelp = `Gira: GitHub-native project OS bootstrapper.

Usage:
  gira <command> [flags]

Commands:
  bootstrap   Bootstrap a repository into a Gira-managed project workspace
  sync        Sync Gira labels, milestones, and bootstrap issues through gh
  status      Show a compact read-only GitHub status summary

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

const syncHelp = `Sync Gira labels, milestones, and bootstrap issues through gh.

Usage:
  gira sync --repo OWNER/REPO [--dry-run]

Flags:
  --repo string  Target GitHub repo in OWNER/REPO format
  --dry-run      Plan sync without creating or updating GitHub metadata
  -h, --help     Show help
`

var newStatusClient = func(repo gira.RepoRef) gira.StatusClient {
	return gira.NewGHStatusClient(repo, gira.ExecCommandRunner{})
}

var newSyncClient = func(repo gira.RepoRef) gira.SyncClient {
	return gira.NewGHSyncClient(repo, gira.ExecCommandRunner{})
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

func runSync(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	dryRun := fs.Bool("dry-run", false, "Plan sync without creating or updating GitHub metadata")
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
	plan, err := gira.BuildSyncPlan(client)
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
