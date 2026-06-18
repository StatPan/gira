package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

const completionHelp = `Generate static shell completion scripts.

Usage:
  gira completion bash
  gira completion zsh
  gira completion fish

The first completion slice is static and local. It completes common Gira
commands, lifecycle subcommands, and shared flags without querying GitHub.
`

type completionEntry struct {
	Name        string
	Description string
}

var completionRootEntries = []completionEntry{
	{Name: "adopt", Description: "Plan or apply adoption for existing repositories and issues"},
	{Name: "audit", Description: "Audit readiness, drift, workflow, and ledgers"},
	{Name: "cache", Description: "Manage local Gira caches"},
	{Name: "completion", Description: "Generate static shell completion scripts"},
	{Name: "config", Description: "Inspect Gira config sources"},
	{Name: "contract", Description: "Inspect contract surfaces"},
	{Name: "dispatch", Description: "Build AI work-order packets"},
	{Name: "docs", Description: "Alias for guide"},
	{Name: "doctor", Description: "Run repo and environment diagnostics"},
	{Name: "epic", Description: "Manage numberless epic status and finish"},
	{Name: "export", Description: "Export local artifacts"},
	{Name: "feature", Description: "Manage optional issue-backed feature records"},
	{Name: "feat", Description: "Alias for feature"},
	{Name: "goal", Description: "Goal-mode planning and reports"},
	{Name: "guide", Description: "Show built-in workflow guides"},
	{Name: "init", Description: "Run first setup checks"},
	{Name: "jira", Description: "Jira provider commands"},
	{Name: "milestone", Description: "Milestone lifecycle commands"},
	{Name: "pm", Description: "PM skill commands for task packets"},
	{Name: "release", Description: "Release readiness gate report"},
	{Name: "repo", Description: "Manage repo registry entries"},
	{Name: "report", Description: "Generate reports"},
	{Name: "setup", Description: "Run setup commands"},
	{Name: "sprint", Description: "Sprint iteration commands"},
	{Name: "start", Description: "Shortcut for ticket start"},
	{Name: "stats", Description: "Read-only workflow statistics"},
	{Name: "status", Description: "Show compact repo status"},
	{Name: "ticket", Description: "Ticket lifecycle commands"},
	{Name: "upgrade", Description: "Check upgrade instructions"},
	{Name: "version", Description: "Show Gira build version"},
	{Name: "workspace", Description: "Workspace inbox and backlog overview"},
	{Name: "work", Description: "Compatibility alias for ticket"},
}

var completionSubcommandEntries = map[string][]completionEntry{
	"audit": {
		{Name: "readiness", Description: "Check audit readiness"},
		{Name: "drift", Description: "Inspect workflow drift"},
		{Name: "workflow", Description: "Audit workflow labels"},
		{Name: "verify", Description: "Verify append-only ledger entries"},
	},
	"cache": {
		{Name: "status", Description: "Show cache status"},
		{Name: "prune", Description: "Prune local cache entries"},
	},
	"config": {
		{Name: "global", Description: "Show global config"},
		{Name: "repo", Description: "Show repo config"},
		{Name: "doctor", Description: "Check config sources"},
	},
	"docs": guideCompletionEntries(),
	"dispatch": {
		{Name: "goal", Description: "Dispatch a goal work order"},
	},
	"feature": {
		{Name: "list", Description: "List feature records"},
		{Name: "check", Description: "Check feature records"},
		{Name: "for", Description: "Show feature for a ticket"},
	},
	"feat": {
		{Name: "list", Description: "List feature records"},
		{Name: "check", Description: "Check feature records"},
		{Name: "for", Description: "Show feature for a ticket"},
	},
	"goal": {
		{Name: "new", Description: "Create a goal issue"},
		{Name: "plan", Description: "Plan child tickets"},
		{Name: "report", Description: "Build a goal report"},
		{Name: "dossier", Description: "Compatibility alias for report"},
		{Name: "status", Description: "Show goal status"},
		{Name: "next", Description: "Choose the next child ticket"},
		{Name: "handoff", Description: "Build a goal handoff"},
		{Name: "finish", Description: "Finish or hand off a goal"},
	},
	"guide": guideCompletionEntries(),
	"jira": {
		{Name: "init", Description: "Initialize Jira provider config"},
		{Name: "doctor", Description: "Check Jira provider config"},
		{Name: "mirror", Description: "Mirror a Jira issue"},
		{Name: "transition", Description: "Plan a Jira transition"},
		{Name: "import", Description: "Import Jira items"},
		{Name: "export", Description: "Export Jira-friendly artifacts"},
	},
	"milestone": {
		{Name: "new", Description: "Create a milestone"},
		{Name: "list", Description: "List milestones"},
		{Name: "status", Description: "Show milestone status"},
		{Name: "assign", Description: "Assign tickets to a milestone"},
		{Name: "plan", Description: "Plan milestone ticket assignment"},
	},
	"pm": {
		{Name: "spec", Description: "Render a PM state task packet"},
		{Name: "qa", Description: "Render a PM acceptance QA prompt"},
	},
	"report": {
		{Name: "weekly", Description: "Render a weekly PM report"},
		{Name: "release-notes", Description: "Render release notes"},
		{Name: "changelog", Description: "Render a changelog document"},
		{Name: "milestone", Description: "Render a milestone progress report"},
		{Name: "backlog-health", Description: "Render a backlog health report"},
		{Name: "delivery-status", Description: "Render a delivery status report"},
		{Name: "qa-checklist", Description: "Render a QA checklist report"},
		{Name: "wbs", Description: "Render a work breakdown report"},
	},
	"sprint": {
		{Name: "plan", Description: "Plan sprint scope"},
		{Name: "start", Description: "Start sprint work"},
		{Name: "close", Description: "Close sprint work"},
		{Name: "status", Description: "Show sprint status"},
	},
	"stats": {
		{Name: "repo", Description: "Show repo stats"},
		{Name: "workspace", Description: "Show workspace stats"},
	},
	"ticket": ticketCompletionEntries(),
	"work":   ticketCompletionEntries(),
	"workspace": {
		{Name: "status", Description: "Show workspace status"},
		{Name: "repos", Description: "Manage workspace repos"},
	},
}

var completionCommonFlags = []completionEntry{
	{Name: "--repo", Description: "Target GitHub repo"},
	{Name: "--ticket", Description: "Ticket number"},
	{Name: "--goal", Description: "Goal issue number; inferred when omitted"},
	{Name: "--json", Description: "Emit JSON output"},
	{Name: "--compact-json", Description: "Emit compact JSON output"},
	{Name: "--prompt", Description: "Emit prompt output"},
	{Name: "--context-budget", Description: "Maximum compact context size"},
	{Name: "--dry-run", Description: "Preview without mutation"},
	{Name: "--apply", Description: "Apply the planned mutation"},
	{Name: "--output", Description: "Output path"},
	{Name: "--html", Description: "Write HTML output"},
	{Name: "--help", Description: "Show help"},
	{Name: "-h", Description: "Show help"},
	{Name: "--version", Description: "Show version"},
}

func ticketCompletionEntries() []completionEntry {
	return []completionEntry{
		{Name: "new", Description: "Create a ticket"},
		{Name: "list", Description: "List tickets"},
		{Name: "view", Description: "Show a ticket card"},
		{Name: "show", Description: "Alias for view"},
		{Name: "prompt", Description: "Render a role prompt"},
		{Name: "handoff", Description: "Render a worker handoff"},
		{Name: "review", Description: "Render a review packet"},
		{Name: "self-review", Description: "Post a self-review note"},
		{Name: "start", Description: "Start ticket work"},
		{Name: "pr", Description: "Create or validate a PR"},
		{Name: "note", Description: "Post a structured note"},
		{Name: "supersede", Description: "Supersede a ticket"},
		{Name: "checks", Description: "Show PR checks"},
		{Name: "wait", Description: "Wait for PR checks"},
		{Name: "finish", Description: "Finish ticket work"},
		{Name: "status", Description: "Show ticket status"},
	}
}

func guideCompletionEntries() []completionEntry {
	return []completionEntry{
		{Name: "quickstart", Description: "Show quickstart guide"},
		{Name: "ticket", Description: "Show ticket guide"},
		{Name: "stats", Description: "Show stats guide"},
		{Name: "jira", Description: "Show Jira guide"},
		{Name: "agent", Description: "Show agent guide"},
		{Name: "skill", Description: "Show agent skill summary"},
		{Name: "concepts", Description: "Show Gira concepts"},
		{Name: "capabilities", Description: "Show adapter capabilities"},
	}
}

func runCompletion(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		_, _ = io.WriteString(stdout, completionHelp)
		return 0
	}
	if len(args) > 1 {
		fmt.Fprintf(stderr, "unexpected completion argument: %s\n\n", args[1])
		_, _ = io.WriteString(stderr, completionHelp)
		return 2
	}
	shell := strings.ToLower(strings.TrimSpace(args[0]))
	var out string
	switch shell {
	case "bash":
		out = renderBashCompletion()
	case "zsh":
		out = renderZshCompletion()
	case "fish":
		out = renderFishCompletion()
	default:
		fmt.Fprintf(stderr, "unsupported completion shell %q; use bash, zsh, or fish\n", args[0])
		return 2
	}
	_, _ = io.WriteString(stdout, out)
	return 0
}

func renderBashCompletion() string {
	var b strings.Builder
	b.WriteString("# bash completion for gira\n")
	b.WriteString("_gira_completion() {\n")
	b.WriteString("  local cur cmd\n")
	b.WriteString("  COMPREPLY=()\n")
	b.WriteString("  cur=\"${COMP_WORDS[COMP_CWORD]}\"\n")
	b.WriteString("  cmd=\"${COMP_WORDS[1]}\"\n\n")
	fmt.Fprintf(&b, "  local root_commands=\"%s\"\n", completionNames(completionRootEntries))
	fmt.Fprintf(&b, "  local common_flags=\"%s\"\n\n", completionNames(completionCommonFlags))
	b.WriteString("  if [[ ${COMP_CWORD} -eq 1 ]]; then\n")
	b.WriteString("    COMPREPLY=( $(compgen -W \"${root_commands} ${common_flags}\" -- \"${cur}\") )\n")
	b.WriteString("    return 0\n")
	b.WriteString("  fi\n\n")
	b.WriteString("  case \"${cmd}\" in\n")
	for _, root := range completionSortedSubcommandRoots() {
		fmt.Fprintf(&b, "    %s)\n", root)
		b.WriteString("      if [[ ${COMP_CWORD} -eq 2 ]]; then\n")
		fmt.Fprintf(&b, "        COMPREPLY=( $(compgen -W \"%s\" -- \"${cur}\") )\n", completionNames(completionSubcommandEntries[root]))
		b.WriteString("        return 0\n")
		b.WriteString("      fi\n")
		b.WriteString("      ;;\n")
	}
	b.WriteString("  esac\n\n")
	b.WriteString("  COMPREPLY=( $(compgen -W \"${common_flags}\" -- \"${cur}\") )\n")
	b.WriteString("  return 0\n")
	b.WriteString("}\n")
	b.WriteString("complete -F _gira_completion gira\n")
	return b.String()
}

func renderZshCompletion() string {
	var b strings.Builder
	b.WriteString("#compdef gira\n\n")
	b.WriteString("_gira() {\n")
	b.WriteString("  local -a root_commands common_flags\n")
	b.WriteString("  root_commands=(\n")
	for _, entry := range completionRootEntries {
		fmt.Fprintf(&b, "    '%s:%s'\n", zshQuote(entry.Name), zshQuote(entry.Description))
	}
	b.WriteString("  )\n")
	b.WriteString("  common_flags=(\n")
	for _, entry := range completionCommonFlags {
		fmt.Fprintf(&b, "    '%s[%s]'\n", zshQuote(entry.Name), zshQuote(entry.Description))
	}
	b.WriteString("  )\n\n")
	b.WriteString("  if (( CURRENT == 2 )); then\n")
	b.WriteString("    _describe 'gira command' root_commands\n")
	b.WriteString("    return\n")
	b.WriteString("  fi\n\n")
	b.WriteString("  case ${words[2]} in\n")
	for _, root := range completionSortedSubcommandRoots() {
		fmt.Fprintf(&b, "    %s)\n", root)
		fmt.Fprintf(&b, "      local -a %s_commands\n", completionZshVar(root))
		fmt.Fprintf(&b, "      %s_commands=(\n", completionZshVar(root))
		for _, entry := range completionSubcommandEntries[root] {
			fmt.Fprintf(&b, "        '%s:%s'\n", zshQuote(entry.Name), zshQuote(entry.Description))
		}
		b.WriteString("      )\n")
		b.WriteString("      if (( CURRENT == 3 )); then\n")
		fmt.Fprintf(&b, "        _describe '%s command' %s_commands\n", root, completionZshVar(root))
		b.WriteString("        return\n")
		b.WriteString("      fi\n")
		b.WriteString("      ;;\n")
	}
	b.WriteString("  esac\n\n")
	b.WriteString("  _arguments ${common_flags[@]}\n")
	b.WriteString("}\n\n")
	b.WriteString("_gira \"$@\"\n")
	return b.String()
}

func renderFishCompletion() string {
	var b strings.Builder
	b.WriteString("# fish completion for gira\n")
	b.WriteString("complete -c gira -f\n")
	for _, entry := range completionCommonFlags {
		writeFishFlag(&b, entry)
	}
	for _, entry := range completionRootEntries {
		fmt.Fprintf(&b, "complete -c gira -n '__fish_use_subcommand' -a '%s' -d '%s'\n", fishQuote(entry.Name), fishQuote(entry.Description))
	}
	for _, root := range completionSortedSubcommandRoots() {
		for _, entry := range completionSubcommandEntries[root] {
			fmt.Fprintf(&b, "complete -c gira -n '__fish_seen_subcommand_from %s' -a '%s' -d '%s'\n", fishQuote(root), fishQuote(entry.Name), fishQuote(entry.Description))
		}
	}
	return b.String()
}

func writeFishFlag(b *strings.Builder, entry completionEntry) {
	switch {
	case strings.HasPrefix(entry.Name, "--"):
		fmt.Fprintf(b, "complete -c gira -l %s -d '%s'\n", fishQuote(strings.TrimPrefix(entry.Name, "--")), fishQuote(entry.Description))
	case strings.HasPrefix(entry.Name, "-") && len(entry.Name) == 2:
		fmt.Fprintf(b, "complete -c gira -s %s -d '%s'\n", fishQuote(strings.TrimPrefix(entry.Name, "-")), fishQuote(entry.Description))
	}
}

func completionNames(entries []completionEntry) string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	return strings.Join(names, " ")
}

func completionSortedSubcommandRoots() []string {
	roots := make([]string, 0, len(completionSubcommandEntries))
	for root := range completionSubcommandEntries {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	return roots
}

func completionZshVar(root string) string {
	return strings.NewReplacer("-", "_").Replace(root)
}

func zshQuote(value string) string {
	return strings.ReplaceAll(value, "'", "'\\''")
}

func fishQuote(value string) string {
	return strings.ReplaceAll(value, "'", "\\'")
}
