package gira

import (
	"fmt"
	"sort"
	"strings"
)

type CommandSpec struct {
	Path        []string
	Summary     string
	Usage       string
	Flags       []FlagSpec
	Since       string
	Docs        []string
	GuideTopics []string
	GuideOrder  int
	Examples    []CommandExample
}

type FlagSpec struct {
	Name    string
	Summary string
}

type CommandExample struct {
	Summary string
	Command string
}

func CoreCommandSpecs() []CommandSpec {
	return []CommandSpec{
		{
			Path:    []string{"setup", "global"},
			Summary: "Create or update the OS-user global config, workspace registry, and repo registry.",
			Usage:   "gira setup global [--repo OWNER/REPO] [--path .] [--workspace NAME] [--inbox-repo OWNER/REPO] [--mode global-only|hybrid] --dry-run|--apply",
			Since:   "v1.7.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Initial execution repo."},
				{Name: "--inbox-repo", Summary: "Backlog/intake repo for unassigned work."},
				{Name: "--mode", Summary: "Use global-only or hybrid repo-local contract mode."},
			},
			Docs:        []string{"README.md", "docs/global-config-registry.md", "docs-site/global-config.md", "docs/workspace.md"},
			GuideTopics: []string{"quickstart"},
			Examples: []CommandExample{
				{Summary: "Preview global-first setup", Command: "gira setup global --repo OWNER/app --path . --workspace personal --inbox-repo OWNER/backlog --mode global-only --dry-run"},
			},
		},
		{
			Path:    []string{"workspace", "repos", "sync"},
			Summary: "Discover GitHub owner/org repos and update a global workspace execution repo allowlist.",
			Usage:   "gira workspace repos sync [--owner OWNER] [--workspace NAME] --dry-run|--apply [--include-archived]",
			Since:   "v1.8.0",
			Flags: []FlagSpec{
				{Name: "--owner", Summary: "GitHub user or organization. Defaults to workspace.owner."},
				{Name: "--workspace", Summary: "Global workspace name. Defaults to global config default_workspace."},
				{Name: "--include-archived", Summary: "Include archived repositories."},
			},
			Docs:        []string{"docs/global-config-registry.md", "docs-site/global-config.md", "docs/workspace.md"},
			GuideTopics: []string{"quickstart"},
			Examples: []CommandExample{
				{Summary: "Preview owner repo sync", Command: "gira workspace repos sync --owner OWNER --workspace personal --dry-run"},
			},
		},
		{
			Path:    []string{"workspace", "status"},
			Summary: "Show inbox and execution repo state from a workspace config or global workspace registry.",
			Usage:   "gira workspace status [--config .gira/config.yaml] [--repo OWNER/REPO] [--limit N] [--active-only] [--cache-ttl 5m] [--refresh] [--json]",
			Since:   "v1.0.0",
			Flags: []FlagSpec{
				{Name: "--config", Summary: "Explicit workspace config path. Defaults to global registry, then .gira/config.yaml."},
				{Name: "--repo", Summary: "Narrow status to one or more execution repos."},
				{Name: "--limit", Summary: "Inspect only the first N selected execution repos."},
				{Name: "--active-only", Summary: "Show only execution repos with open work or an active milestone."},
				{Name: "--max-concurrency", Summary: "Bound concurrent repo status fetches. Default: 4."},
				{Name: "--cache-ttl", Summary: "Reuse recent per-repo status cache for this duration. Default: 5m."},
				{Name: "--refresh", Summary: "Ignore cached status and fetch fresh data."},
				{Name: "--json", Summary: "Emit stable JSON."},
			},
			Docs:        []string{"README.md", "docs/workspace.md", "docs-site/global-config.md"},
			GuideTopics: []string{"quickstart"},
			Examples: []CommandExample{
				{Summary: "Read the default workspace", Command: "gira workspace status"},
				{Summary: "Inspect a bounded subset", Command: "gira workspace status --limit 10 --active-only"},
			},
		},
		{
			Path:    []string{"ticket", "new"},
			Summary: "Create a repo-bound executable GitHub issue with structured or full Markdown body input.",
			Usage:   "gira ticket new \"Title\" --dry-run|--apply [--body TEXT|--body-file PATH|-] [--start]",
			Since:   "v1.0.0",
			Flags: []FlagSpec{
				{Name: "--goal", Summary: "Structured issue goal."},
				{Name: "--acceptance", Summary: "Semicolon-separated acceptance criteria."},
				{Name: "--body", Summary: "Full issue body."},
				{Name: "--body-file", Summary: "Read full issue body from file or stdin with -."},
				{Name: "--start", Summary: "Start the created ticket after apply."},
			},
			Docs:        []string{"README.md", "docs-site/ticket-workflow.md", "docs/dogfood.md"},
			GuideTopics: []string{"quickstart", "ticket", "agent"},
			GuideOrder:  10,
			Examples: []CommandExample{
				{Summary: "Preview structured ticket", Command: "gira ticket new \"TITLE\" --goal \"GOAL\" --acceptance \"a;b;c\" --dry-run"},
				{Summary: "Preview full Markdown packet", Command: "gira ticket new --title \"TITLE\" --body-file issue.md --dry-run"},
			},
		},
		{
			Path:        []string{"ticket", "view"},
			Summary:     "Show a Gira operating card for the ticket, linked PR, blockers, and next action.",
			Usage:       "gira ticket view [TICKET] [--repo OWNER/REPO] [--json]",
			Since:       "v1.12.0",
			Docs:        []string{"README.md", "docs-site/ticket-workflow.md", "docs/dogfood.md"},
			GuideTopics: []string{"ticket", "agent"},
			GuideOrder:  15,
			Examples: []CommandExample{
				{Summary: "Inspect current branch ticket context", Command: "gira ticket view"},
			},
		},
		{
			Path:        []string{"ticket", "start"},
			Summary:     "Verify a ready issue, create or reuse its branch, and move it to in-progress.",
			Usage:       "gira ticket start [TICKET] --dry-run|--apply [--repo OWNER/REPO]",
			Since:       "v1.0.0",
			Docs:        []string{"README.md", "docs-site/ticket-workflow.md", "docs/dogfood.md"},
			GuideTopics: []string{"ticket", "agent"},
			GuideOrder:  20,
			Examples: []CommandExample{
				{Summary: "Start an existing ready issue", Command: "gira ticket start 42 --apply"},
			},
		},
		{
			Path:        []string{"ticket", "pr"},
			Summary:     "Create or validate a linked PR with required issue closing text.",
			Usage:       "gira ticket pr [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--draft]",
			Since:       "v1.0.0",
			Docs:        []string{"README.md", "docs-site/ticket-workflow.md", "docs/dogfood.md"},
			GuideTopics: []string{"ticket", "agent"},
			GuideOrder:  30,
			Examples: []CommandExample{
				{Summary: "Open a draft PR", Command: "gira ticket pr --apply --draft"},
			},
		},
		{
			Path:    []string{"ticket", "note"},
			Summary: "Post a structured context note to the issue, linked PR, or both.",
			Usage:   "gira ticket note [TICKET] \"BODY\" --dry-run|--apply [--repo OWNER/REPO] [--kind progress|blocker|decision|handoff|summary|check] [--target auto|issue|pr|both]",
			Since:   "v1.12.0",
			Flags: []FlagSpec{
				{Name: "--kind", Summary: "Template kind for the note. Default: progress."},
				{Name: "--target", Summary: "Comment target: auto, issue, pr, or both. Default: auto."},
				{Name: "--body", Summary: "Explicit note body."},
				{Name: "--body-file", Summary: "Read note body from file or stdin with -."},
				{Name: "--dry-run", Summary: "Preview target resolution and rendered note without posting."},
				{Name: "--apply", Summary: "Post the rendered note."},
			},
			Docs:        []string{"README.md", "docs-site/ticket-workflow.md", "docs/dogfood.md"},
			GuideTopics: []string{"ticket", "agent"},
			GuideOrder:  35,
			Examples: []CommandExample{
				{Summary: "Preview a progress note", Command: "gira ticket note \"Implemented parser path\" --dry-run"},
				{Summary: "Post a blocker to both issue and PR", Command: "gira ticket note --kind blocker --target both --body-file note.md --apply"},
			},
		},
		{
			Path:        []string{"ticket", "checks"},
			Summary:     "Show linked PR checks, review blockers, and next action.",
			Usage:       "gira ticket checks [TICKET] [--repo OWNER/REPO] [--json]",
			Since:       "v1.0.0",
			Docs:        []string{"README.md", "docs-site/ticket-workflow.md", "docs/dogfood.md"},
			GuideTopics: []string{"ticket", "agent"},
			GuideOrder:  40,
			Examples: []CommandExample{
				{Summary: "Inspect PR readiness", Command: "gira ticket checks"},
			},
		},
		{
			Path:        []string{"ticket", "wait"},
			Summary:     "Wait for pending linked PR checks without merging.",
			Usage:       "gira ticket wait [TICKET] [--repo OWNER/REPO] [--timeout 5m] [--interval 5s]",
			Since:       "v1.0.0",
			Docs:        []string{"README.md", "docs-site/ticket-workflow.md", "docs/dogfood.md"},
			GuideTopics: []string{"ticket", "agent"},
			GuideOrder:  50,
			Examples: []CommandExample{
				{Summary: "Wait for CI", Command: "gira ticket wait --timeout 5m"},
			},
		},
		{
			Path:        []string{"ticket", "finish"},
			Summary:     "Merge the linked PR when policy allows, sync main, and close the ticket loop.",
			Usage:       "gira ticket finish [TICKET] --dry-run|--apply [--repo OWNER/REPO]",
			Since:       "v1.0.0",
			Docs:        []string{"README.md", "docs-site/ticket-workflow.md", "docs/dogfood.md"},
			GuideTopics: []string{"ticket", "agent"},
			GuideOrder:  60,
			Examples: []CommandExample{
				{Summary: "Preview finish", Command: "gira ticket finish --dry-run"},
			},
		},
		{
			Path:        []string{"ticket", "status"},
			Summary:     "Report ticket status, linked PR blockers, and next action.",
			Usage:       "gira ticket status [TICKET] [--repo OWNER/REPO] [--json]",
			Since:       "v1.0.0",
			Docs:        []string{"README.md", "docs-site/ticket-workflow.md", "docs/dogfood.md"},
			GuideTopics: []string{"ticket", "agent"},
			GuideOrder:  70,
			Examples: []CommandExample{
				{Summary: "Inspect current branch ticket", Command: "gira ticket status"},
			},
		},
	}
}

func FindCommandSpec(path ...string) (CommandSpec, bool) {
	key := commandSpecKey(path)
	for _, spec := range CoreCommandSpecs() {
		if commandSpecKey(spec.Path) == key {
			return spec, true
		}
	}
	return CommandSpec{}, false
}

func RenderCommandReferenceMarkdown(specs []CommandSpec) string {
	specs = append([]CommandSpec(nil), specs...)
	sort.Slice(specs, func(i, j int) bool {
		return commandSpecKey(specs[i].Path) < commandSpecKey(specs[j].Path)
	})
	var b strings.Builder
	b.WriteString("# Command Reference\n\n")
	b.WriteString("This page is generated from Gira's command metadata registry. Update `internal/gira/command_registry.go` first, then refresh this page.\n\n")
	for _, spec := range specs {
		fmt.Fprintf(&b, "## `%s`\n\n", strings.Join(spec.Path, " "))
		fmt.Fprintf(&b, "%s\n\n", spec.Summary)
		fmt.Fprintf(&b, "Usage:\n\n```bash\n%s\n```\n\n", spec.Usage)
		if spec.Since != "" {
			fmt.Fprintf(&b, "Since: `%s`\n\n", spec.Since)
		}
		if len(spec.Flags) > 0 {
			b.WriteString("Flags:\n\n")
			for _, flag := range spec.Flags {
				fmt.Fprintf(&b, "- `%s`: %s\n", flag.Name, flag.Summary)
			}
			b.WriteString("\n")
		}
		if len(spec.Examples) > 0 {
			b.WriteString("Examples:\n\n")
			for _, example := range spec.Examples {
				fmt.Fprintf(&b, "- %s\n\n```bash\n%s\n```\n\n", example.Summary, example.Command)
			}
		}
		if len(spec.Docs) > 0 {
			b.WriteString("Documented in: ")
			for i, doc := range spec.Docs {
				if i > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(&b, "`%s`", doc)
			}
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

func RenderGuideCommandSection(topic string, specs []CommandSpec) string {
	specs = filterCommandSpecsForGuide(topic, specs)
	sortGuideSpecs(specs)
	var b strings.Builder
	for _, spec := range specs {
		fmt.Fprintf(&b, "  %s\n", spec.Usage)
		fmt.Fprintf(&b, "    %s\n", spec.Summary)
		for _, example := range spec.Examples {
			fmt.Fprintf(&b, "    Example: %s\n", example.Command)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func filterCommandSpecsForGuide(topic string, specs []CommandSpec) []CommandSpec {
	var filtered []CommandSpec
	for _, spec := range specs {
		for _, guideTopic := range spec.GuideTopics {
			if guideTopic == topic {
				filtered = append(filtered, spec)
				break
			}
		}
	}
	return filtered
}

func sortGuideSpecs(specs []CommandSpec) {
	sort.Slice(specs, func(i, j int) bool {
		if specs[i].GuideOrder != specs[j].GuideOrder {
			if specs[i].GuideOrder == 0 {
				return false
			}
			if specs[j].GuideOrder == 0 {
				return true
			}
			return specs[i].GuideOrder < specs[j].GuideOrder
		}
		return commandSpecKey(specs[i].Path) < commandSpecKey(specs[j].Path)
	})
}

func commandSpecKey(path []string) string {
	return strings.Join(path, " ")
}
