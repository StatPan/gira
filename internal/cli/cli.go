package cli

import (
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

Flags:
  -h, --help  Show help
`

const bootstrapHelp = `Bootstrap a repository into a Gira-managed project workspace.

Usage:
  gira bootstrap --repo OWNER/REPO --template default --dry-run [--created-at YYYY-MM-DD]

Flags:
  --repo string        Target GitHub repo in OWNER/REPO format
  --template string   Template name to render (default "default")
  --dry-run           Render without writing files or calling GitHub
  --created-at string Override render date for deterministic tests
  -h, --help          Show help
`

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, rootHelp)
		return 0
	}

	switch args[0] {
	case "bootstrap":
		return runBootstrap(args[1:], stdout, stderr)
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
	if !*dryRun {
		fmt.Fprint(stderr, "Go bootstrap currently supports --dry-run only\n")
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
	fmt.Fprint(stdout, gira.FormatDryRun(rendered))
	return 0
}
