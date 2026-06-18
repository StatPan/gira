package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/StatPan/gira/internal/gira"
)

const completionHelp = `Generate shell completion scripts and local dynamic candidates.

Usage:
  gira completion bash
  gira completion zsh
  gira completion fish
  gira completion candidates repo|ticket|label|milestone [--repo OWNER/REPO] [--prefix TEXT]

Shell scripts complete common Gira commands, lifecycle subcommands, and shared
flags. Dynamic candidates are cache-first and local: repo candidates come from
the global repo registry and current git origin; ticket, label, and milestone
candidates come from the workspace status cache when available.
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
	if args[0] == "candidates" {
		return runCompletionCandidates(args[1:], stdout, stderr)
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

func runCompletionCandidates(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		_, _ = io.WriteString(stdout, completionHelp)
		return 0
	}
	kind := strings.ToLower(strings.TrimSpace(args[0]))
	fs := flag.NewFlagSet("completion candidates", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo")
	prefix := fs.String("prefix", "", "Candidate prefix")
	configRoot := fs.String("config-root", "", "Override global config root")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, completionHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, completionHelp)
		return 0
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected completion candidates argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, completionHelp)
		return 2
	}
	var candidates []string
	var err error
	switch kind {
	case "repo":
		candidates, err = completionRepoCandidates(*configRoot)
	case "ticket":
		candidates, err = completionTicketCandidates(*repoValue, *configRoot)
	case "label":
		candidates, err = completionLabelCandidates(*repoValue, *configRoot)
	case "milestone":
		candidates, err = completionMilestoneCandidates(*repoValue, *configRoot)
	default:
		fmt.Fprintf(stderr, "unsupported completion candidate kind %q; use repo, ticket, label, or milestone\n", args[0])
		return 2
	}
	if err != nil {
		// Shell completion should fail closed and quiet in normal use.
		return 0
	}
	for _, candidate := range filterCompletionCandidates(candidates, *prefix) {
		fmt.Fprintln(stdout, candidate)
	}
	return 0
}

func completionRepoCandidates(configRoot string) ([]string, error) {
	values := []string{}
	if ctx, err := gira.ResolveRepoContextDetails(gira.RepoContextOptions{ConfigRoot: configRoot}); err == nil {
		values = append(values, ctx.Repo.FullName())
	}
	entries, err := gira.ListGlobalRepoRegistryEntries(configRoot)
	if err != nil {
		return uniqueCompletionCandidates(values), nil
	}
	for _, entry := range entries {
		values = append(values, entry.Repo.FullName())
		values = append(values, entry.Entry.Aliases...)
	}
	return uniqueCompletionCandidates(values), nil
}

func completionTicketCandidates(repoValue string, configRoot string) ([]string, error) {
	summary, ok := completionCachedStatus(repoValue, configRoot)
	if !ok {
		return nil, nil
	}
	values := make([]string, 0, len(summary.Issues.Open))
	for _, issue := range summary.Issues.Open {
		if issue.Number > 0 {
			values = append(values, strconv.Itoa(issue.Number))
		}
	}
	return values, nil
}

func completionLabelCandidates(repoValue string, configRoot string) ([]string, error) {
	values := make([]string, 0, len(gira.DesiredLabels))
	for _, label := range gira.DesiredLabels {
		values = append(values, label.Name)
	}
	if summary, ok := completionCachedStatus(repoValue, configRoot); ok {
		for _, issue := range summary.Issues.Open {
			values = append(values, issue.Labels...)
		}
	}
	return uniqueCompletionCandidates(values), nil
}

func completionMilestoneCandidates(repoValue string, configRoot string) ([]string, error) {
	summary, ok := completionCachedStatus(repoValue, configRoot)
	if !ok {
		return nil, nil
	}
	values := make([]string, 0, len(summary.Milestones))
	for _, milestone := range summary.Milestones {
		if strings.TrimSpace(milestone.Title) != "" {
			values = append(values, milestone.Title)
		}
	}
	return uniqueCompletionCandidates(values), nil
}

func completionCachedStatus(repoValue string, configRoot string) (gira.StatusSummary, bool) {
	repo, err := completionResolveRepo(repoValue, configRoot)
	if err != nil {
		return gira.StatusSummary{}, false
	}
	root, err := completionCacheRoot(configRoot)
	if err != nil {
		return gira.StatusSummary{}, false
	}
	path := filepath.Join(root, "workspace-status", repo.Owner, repo.Name+".json")
	content, err := os.ReadFile(path)
	if err != nil {
		return gira.StatusSummary{}, false
	}
	var summary gira.StatusSummary
	if err := json.Unmarshal(content, &summary); err != nil {
		return gira.StatusSummary{}, false
	}
	return summary, true
}

func completionResolveRepo(repoValue string, configRoot string) (gira.RepoRef, error) {
	ctx, err := gira.ResolveRepoContextDetails(gira.RepoContextOptions{RepoValue: repoValue, ConfigRoot: configRoot})
	if err != nil {
		return gira.RepoRef{}, err
	}
	return ctx.Repo, nil
}

func completionCacheRoot(configRoot string) (string, error) {
	if strings.TrimSpace(configRoot) != "" {
		if cfg, err := gira.LoadGlobalConfig(configRoot); err == nil && strings.TrimSpace(cfg.Paths.CacheRoot) != "" {
			return filepath.Abs(completionExpandHome(cfg.Paths.CacheRoot))
		}
	}
	return gira.DefaultGiraCacheRoot()
}

func completionExpandHome(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "~" || strings.HasPrefix(trimmed, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			if trimmed == "~" {
				return home
			}
			return filepath.Join(home, strings.TrimPrefix(trimmed, "~/"))
		}
	}
	return trimmed
}

func filterCompletionCandidates(candidates []string, prefix string) []string {
	prefix = strings.TrimSpace(prefix)
	unique := uniqueCompletionCandidates(candidates)
	if prefix == "" {
		return unique
	}
	out := make([]string, 0, len(unique))
	for _, candidate := range unique {
		if strings.HasPrefix(strings.ToLower(candidate), strings.ToLower(prefix)) {
			out = append(out, candidate)
		}
	}
	return out
}

func uniqueCompletionCandidates(candidates []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" || strings.ContainsRune(trimmed, '\n') || strings.ContainsRune(trimmed, 0) {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out
}

func renderBashCompletion() string {
	var b strings.Builder
	b.WriteString("# bash completion for gira\n")
	b.WriteString("_gira_completion() {\n")
	b.WriteString("  local cur prev cmd sub repo_arg\n")
	b.WriteString("  COMPREPLY=()\n")
	b.WriteString("  cur=\"${COMP_WORDS[COMP_CWORD]}\"\n")
	b.WriteString("  prev=\"${COMP_WORDS[COMP_CWORD-1]}\"\n")
	b.WriteString("  cmd=\"${COMP_WORDS[1]}\"\n\n")
	fmt.Fprintf(&b, "  local root_commands=\"%s\"\n", completionNames(completionRootEntries))
	fmt.Fprintf(&b, "  local common_flags=\"%s\"\n\n", completionNames(completionCommonFlags))
	b.WriteString("  for ((i=1; i<COMP_CWORD; i++)); do\n")
	b.WriteString("    if [[ \"${COMP_WORDS[i]}\" == \"--repo\" && $((i+1)) -lt ${COMP_CWORD} ]]; then\n")
	b.WriteString("      repo_arg=\"${COMP_WORDS[i+1]}\"\n")
	b.WriteString("    fi\n")
	b.WriteString("  done\n\n")
	b.WriteString("  _gira_dynamic() {\n")
	b.WriteString("    local kind=\"$1\"\n")
	b.WriteString("    local args=(completion candidates \"${kind}\" --prefix \"${cur}\")\n")
	b.WriteString("    if [[ -n \"${repo_arg}\" && \"${kind}\" != \"repo\" ]]; then\n")
	b.WriteString("      args+=(--repo \"${repo_arg}\")\n")
	b.WriteString("    fi\n")
	b.WriteString("    mapfile -t COMPREPLY < <(gira \"${args[@]}\" 2>/dev/null)\n")
	b.WriteString("  }\n\n")
	b.WriteString("  if [[ ${COMP_CWORD} -eq 1 ]]; then\n")
	b.WriteString("    COMPREPLY=( $(compgen -W \"${root_commands} ${common_flags}\" -- \"${cur}\") )\n")
	b.WriteString("    return 0\n")
	b.WriteString("  fi\n\n")
	b.WriteString("  case \"${prev}\" in\n")
	b.WriteString("    --repo)\n")
	b.WriteString("      _gira_dynamic repo\n")
	b.WriteString("      return 0\n")
	b.WriteString("      ;;\n")
	b.WriteString("    --label)\n")
	b.WriteString("      _gira_dynamic label\n")
	b.WriteString("      return 0\n")
	b.WriteString("      ;;\n")
	b.WriteString("    --milestone)\n")
	b.WriteString("      _gira_dynamic milestone\n")
	b.WriteString("      return 0\n")
	b.WriteString("      ;;\n")
	b.WriteString("  esac\n\n")
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
	b.WriteString("  sub=\"${COMP_WORDS[2]}\"\n")
	b.WriteString("  if [[ (\"${cmd}\" == \"ticket\" || \"${cmd}\" == \"work\") && ${COMP_CWORD} -eq 3 ]]; then\n")
	b.WriteString("    case \"${sub}\" in\n")
	b.WriteString("      view|show|start|pr|checks|wait|finish|status|prompt|handoff|review|self-review|note|supersede)\n")
	b.WriteString("        _gira_dynamic ticket\n")
	b.WriteString("        return 0\n")
	b.WriteString("        ;;\n")
	b.WriteString("    esac\n")
	b.WriteString("  fi\n\n")
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
	b.WriteString("  local repo_arg\n")
	b.WriteString("  local i\n")
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
	b.WriteString("  for (( i = 1; i < CURRENT; i++ )); do\n")
	b.WriteString("    if [[ \"${words[i]}\" == \"--repo\" && $((i + 1)) -lt ${CURRENT} ]]; then\n")
	b.WriteString("      repo_arg=\"${words[i+1]}\"\n")
	b.WriteString("    fi\n")
	b.WriteString("  done\n\n")
	b.WriteString("  _gira_dynamic() {\n")
	b.WriteString("    local kind=\"$1\"\n")
	b.WriteString("    local -a args candidates\n")
	b.WriteString("    args=(completion candidates \"${kind}\" --prefix \"${PREFIX}\")\n")
	b.WriteString("    if [[ -n \"${repo_arg}\" && \"${kind}\" != \"repo\" ]]; then\n")
	b.WriteString("      args+=(--repo \"${repo_arg}\")\n")
	b.WriteString("    fi\n")
	b.WriteString("    candidates=(${(f)\"$(gira ${args[@]} 2>/dev/null)\"})\n")
	b.WriteString("    compadd -Q -- ${candidates[@]}\n")
	b.WriteString("  }\n\n")
	b.WriteString("  case ${words[CURRENT-1]} in\n")
	b.WriteString("    --repo)\n")
	b.WriteString("      _gira_dynamic repo\n")
	b.WriteString("      return\n")
	b.WriteString("      ;;\n")
	b.WriteString("    --label)\n")
	b.WriteString("      _gira_dynamic label\n")
	b.WriteString("      return\n")
	b.WriteString("      ;;\n")
	b.WriteString("    --milestone)\n")
	b.WriteString("      _gira_dynamic milestone\n")
	b.WriteString("      return\n")
	b.WriteString("      ;;\n")
	b.WriteString("  esac\n\n")
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
	b.WriteString("  if [[ (${words[2]} == ticket || ${words[2]} == work) && ${CURRENT} == 4 ]]; then\n")
	b.WriteString("    case ${words[3]} in\n")
	b.WriteString("      view|show|start|pr|checks|wait|finish|status|prompt|handoff|review|self-review|note|supersede)\n")
	b.WriteString("        _gira_dynamic ticket\n")
	b.WriteString("        return\n")
	b.WriteString("        ;;\n")
	b.WriteString("    esac\n")
	b.WriteString("  fi\n\n")
	b.WriteString("  _arguments ${common_flags[@]}\n")
	b.WriteString("}\n\n")
	b.WriteString("_gira \"$@\"\n")
	return b.String()
}

func renderFishCompletion() string {
	var b strings.Builder
	b.WriteString("# fish completion for gira\n")
	b.WriteString("complete -c gira -f\n")
	b.WriteString("function __gira_repo_arg\n")
	b.WriteString("    set -l tokens (commandline -opc)\n")
	b.WriteString("    for i in (seq 1 (count $tokens))\n")
	b.WriteString("        if test \"$tokens[$i]\" = \"--repo\"; and test (math $i + 1) -le (count $tokens)\n")
	b.WriteString("            echo $tokens[(math $i + 1)]\n")
	b.WriteString("            return 0\n")
	b.WriteString("        end\n")
	b.WriteString("    end\n")
	b.WriteString("end\n")
	b.WriteString("function __gira_candidates\n")
	b.WriteString("    set -l kind $argv[1]\n")
	b.WriteString("    set -l prefix (commandline -ct)\n")
	b.WriteString("    set -l repo (__gira_repo_arg)\n")
	b.WriteString("    if test -n \"$repo\"; and test \"$kind\" != \"repo\"\n")
	b.WriteString("        gira completion candidates $kind --repo \"$repo\" --prefix \"$prefix\" 2>/dev/null\n")
	b.WriteString("    else\n")
	b.WriteString("        gira completion candidates $kind --prefix \"$prefix\" 2>/dev/null\n")
	b.WriteString("    end\n")
	b.WriteString("end\n")
	for _, entry := range completionCommonFlags {
		writeFishFlag(&b, entry)
	}
	b.WriteString("complete -c gira -n '__fish_seen_argument -l repo' -a '(__gira_candidates repo)'\n")
	b.WriteString("complete -c gira -n '__fish_seen_argument -l label' -a '(__gira_candidates label)'\n")
	b.WriteString("complete -c gira -n '__fish_seen_argument -l milestone' -a '(__gira_candidates milestone)'\n")
	b.WriteString("complete -c gira -n '__fish_seen_subcommand_from ticket; and __fish_seen_subcommand_from view show start pr checks wait finish status prompt handoff review self-review note supersede' -a '(__gira_candidates ticket)'\n")
	b.WriteString("complete -c gira -n '__fish_seen_subcommand_from work; and __fish_seen_subcommand_from view show start pr checks wait finish status prompt handoff review self-review note supersede' -a '(__gira_candidates ticket)'\n")
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
