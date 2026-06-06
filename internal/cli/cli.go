package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/StatPan/gira/internal/gira"
)

var jiraKeyPositionalPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d+$`)

const rootHelp = `Gira: Jira-style project flow on GitHub.

Usage:
  gira <command> [flags]

Daily commands:
  guide       Built-in quickstart and workflow guides
  setup       Intention-based first-run and global registry setup
  workspace   Personal workspace inbox and backlog overview
  queue       Agent-ready workspace queue selection commands
  projects    Sync visible GitHub Projects board items
  repo        Manage global registry entries for repositories
  adopt       Plan or apply adoption for existing repositories and issues
  ticket      Jira-style ticket lifecycle commands
  run         Local Codex run manifests and execution state
  feature     Optional issue-backed feature map commands. Alias: feat
  goal        Goal-mode planning, status, and visible report commands
  epic        Numberless epic status and finish commands
  milestone   Milestone lifecycle and bulk ticket assignment
  sprint      Sprint iteration planning/start/close workflow
  release     Release readiness gate report
  status      Show a compact read-only GitHub status summary
  stats       Read-only workflow closure statistics
  config      Inspect global and repo-local Gira config sources
  upgrade     Check latest release and print upgrade instructions
  cache       Manage local Gira caches
  completion  Generate static shell completion scripts
  version     Show Gira build version
  start       Shortcut for ticket start

Setup:
  init        One-command onboarding with prerequisite checks and next-step plan

Advanced:
  audit       Self-audit readiness and append-only ledger verification
  ops         Advanced setup, migration, policy, audit, and raw GitHub controls
  work        Compatibility alias for ticket lifecycle commands
  dev         Compatibility developer workflow helpers

Flags:
  -h, --help   Show help
  --version    Show Gira build version
`

const guideHelp = `Built-in Gira guides for installed CLI users.

Usage:
  gira guide [quickstart|ticket|stats|jira|agent|skill|concepts|capabilities]
  gira docs [quickstart|ticket|stats|jira|agent|skill|concepts|capabilities]

Topics:
  quickstart  First successful flow from auth to merged PR
  ticket      Daily ticket lifecycle commands
  stats       Closure funnel and workflow health metrics
  jira        Jira-primary provider mode and workflow safety
  agent       Minimal rules for AI/coding agents
  skill       Alias for the canonical agent operator summary
  concepts    Jira terms mapped to Gira and GitHub
  capabilities Adapter command capability map

Start here:
  gira guide quickstart
`

const guideQuickstart = `Gira quickstart: first ticket to merged PR

1. Authenticate GitHub.
   gh auth status

2. Confirm repo state.
   gira init --repo OWNER/REPO --path . --dry-run
   gira adopt repo --repo OWNER/REPO --path . --dry-run
   gira status

3. Create and start a ticket.
   gira ticket new "TITLE" --goal "GOAL" --acceptance "item 1;item 2" --apply --start

4. Implement the bounded scope and verify locally.
   go test ./...

5. Open or reuse the linked PR.
   gira ticket pr --apply --draft

6. Watch readiness through Gira.
   gira ticket review --diff-summary
   gira ticket checks
   gira ticket wait --timeout 5m

7. Finish the ticket.
   gira ticket finish --apply
   gira ticket status

8. Optional planning checks after the first loop works.
   gira epic status
   gira epic finish --dry-run
`

const guideTicketIntro = `Gira ticket guide

Daily loop:
  gira ticket new "TITLE" --goal "GOAL" --acceptance "a;b;c" --apply --start
  gira ticket pr --apply --draft
  gira ticket review --diff-summary
  gira ticket self-review --diff-summary --dry-run
  gira ticket checks
  gira ticket wait --timeout 5m
  gira ticket finish --apply

Existing GitHub issue:
  gira ticket start 42 --apply
  gira ticket pr --apply --draft
  gira ticket review --diff-summary
  gira ticket finish --apply

Context rules:
  After ticket start checks out issue-N-*, ticket view/prompt/handoff/review/self-review/pr/checks/wait/finish/status infer the ticket.
  Use --repo OWNER/REPO and --ticket N only when outside a repo or branch context.

Safety:
  Use --dry-run before mutating commands when unsure.
  PR bodies must contain Closes #N, Fixes #N, or Resolves #N.

Registry-backed commands:
`

const guideStats = `Gira stats guide

Purpose:
  Closure Funnel reports answer whether GitHub work actually moves from issue to PR to checks to merge and closed issue.

Start with a single repo:
  gira stats repo --repo OWNER/REPO --since 90d
  gira stats repo --repo OWNER/REPO --since 90d --json

Workspace direction:
  gira stats workspace --since 90d is the planned multi-repo rollup. It should reuse the same metrics, bounded repo selection, cache TTLs, and rate-limit reporting used by workspace status.

Rules:
  Text is the default output for humans.
  --json is for automation and downstream dashboards.
  Reports are GitHub read-only and can inspect non-Gira repos with lower confidence.
  Gira labels, closing links, and lifecycle commands raise confidence.

Non-goals:
  No personal productivity score, full DORA suite, AI spend/token analytics, dashboard UI, or precise agent attribution in the first slice.
`

const guideJira = `Gira Jira-primary provider guide

Default mode:
  GitHub-native remains the default. GitHub issues are work packets, PRs are change units, and merged PR plus closed issue is completion evidence.

Jira-primary mode:
  Jira owns planning and status. GitHub owns execution evidence. Gira mirrors Jira keys into GitHub issues, then refuses Jira Done until the linked PR, review, checks, merge, and mirror evidence are clean.

Setup and mirror:
  gira jira init --repo OWNER/REPO --api-base https://example.atlassian.net --project ABC --dry-run
  gira jira init --repo OWNER/REPO --api-base https://example.atlassian.net --project ABC --apply
  gira jira doctor --repo OWNER/REPO --sample-key ABC-123
  gira jira mirror ABC-123 --repo OWNER/REPO --dry-run
  gira jira mirror ABC-123 --repo OWNER/REPO --apply
  gira ticket view ABC-123 --repo OWNER/REPO
  gira ticket start ABC-123 --repo OWNER/REPO --apply

Transition safety:
  gira jira transition ABC-123 --repo OWNER/REPO --to done --dry-run
  gira ticket finish --dry-run
  gira ticket finish --apply

Migration helpers:
  gira jira import --repo OWNER/REPO --source jira.csv --dry-run
  gira jira import --repo OWNER/REPO --api-base https://example.atlassian.net --project ABC --dry-run
  gira jira export --repo OWNER/REPO --output out/jira

Boundaries:
  Provider config stores non-secret base URL, project key, source-of-truth policy, and status map in the global repo registry.
  Credentials come from JIRA_EMAIL and JIRA_API_TOKEN; Gira does not write Jira tokens to repo or global config.
  Workflow status creation, workflow mutation, background sync, full bidirectional sync, and hosted dashboards are outside the OSS CLI slices.
`

const statsHelp = `Read-only workflow closure statistics.

Usage:
  gira stats repo [OWNER/REPO] [--repo OWNER/REPO] [--since 90d] [--stale-days 14] [--limit 100] [--json]
  gira stats workspace [--since 90d]

Commands:
  repo       Show a Closure Funnel report for one GitHub repo
  workspace  Planned multi-repo Closure Funnel rollup

Flags:
  --repo string       Target GitHub repo in OWNER/REPO format. Defaults to .gira config or git origin
  --since string      Reporting window such as 90d or YYYY-MM-DD. Default: 90d
  --stale-days int    Count open issues/PRs stale after this many days. Default: 14
  --limit int         Max GitHub rows per query. Default: 100
  --json              Emit stable JSON output
  -h, --help          Show help
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
  gira status --all [--config .gira/config.yaml] [--json] [--stale-days N]
  gira status --owner OWNER [--limit N] [--include-archived] [--json] [--stale-days N]

Flags:
  --repo string       Target GitHub repo in OWNER/REPO format (default: infer from .gira/config.yaml or git origin)
  --all               Summarize all execution repos from workspace config
  --owner string      Discover repositories owned by a user or organization with gh repo list
  --config string     Workspace config path for --all (default ".gira/config.yaml")
  --limit int         Maximum repositories to inspect for --owner (default 50)
  --include-archived  Include archived repositories in --owner discovery
  --json              Emit stable JSON for automation
  --stale-days int    Days since update before open issues count as stale (default 14)
  -h, --help          Show help
`

const workspaceHelp = `Personal workspace inbox and backlog commands.

Usage:
  gira workspace init --inbox-repo OWNER/REPO [--repo OWNER/REPO] [--scope repo|global] [--project-owner OWNER] [--project-title TITLE] [--project-number N] [--merge] --dry-run|--apply [--path .] [--config-root PATH]
  gira workspace validate [--config .gira/config.yaml] [--json]
  gira workspace status [--config .gira/config.yaml] [--repo OWNER/REPO] [--limit N] [--active-only] [--cache-ttl 5m] [--refresh] [--json]
  gira workspace backlog [--config .gira/config.yaml] [--repo OWNER/REPO] [--limit N] [--cache-ttl 5m] [--refresh] [--json]
  gira workspace list [--config .gira/config.yaml] [--repo OWNER/REPO] [--limit N] [--cache-ttl 5m] [--refresh] [--json]  (alias: backlog)
  gira workspace sync --dry-run|--apply [--config .gira/config.yaml] [--bootstrap-issues] [--json]
  gira workspace repos sync [--owner OWNER] [--workspace NAME] --dry-run|--apply [--config-root PATH] [--limit N] [--include-archived] [--json]
  gira workspace ticket new "Title" [--body TEXT|--body-file PATH|-] [--repo OWNER/REPO --dry-run|--apply] [--config .gira/config.yaml] [--json]
  gira workspace ticket route --ticket N --repo OWNER/REPO --dry-run|--apply [--config .gira/config.yaml] [--json]
  gira workspace capability [--config .gira/config.yaml] [--json]
  gira workspace project plan [--config .gira/config.yaml] [--json]
  gira workspace project adopt --owner OWNER (--title TITLE | --number N) --dry-run|--apply [--config .gira/config.yaml] [--json]

Commands:
  init     Create a workspace config for personal or repo-bound backlog use
  validate Validate inbox backlog routing readiness without mutation
  status   Show inbox and repo execution state in one Jira-like overview
  backlog  List inbox tickets and repo issues together. Alias: list
  sync     Sync Gira metadata across inbox and execution repos
  repos    Discover and register GitHub owner repos into a global workspace
  ticket   Create or route repo-agnostic inbox tickets
  capability  Check inbox and execution repo read/write permissions
  project  Read-only GitHub Projects v2 visibility planning and existing Project adoption

Flags:
  --config string       Explicit workspace config path; defaults to global registry, then ".gira/config.yaml"
  --repo string         Narrow status/backlog/list to execution repo. Repeatable or comma-separated
  --limit int           Maximum execution repos to inspect for status/backlog/list
  --active-only         Show only repos with open work or active milestone
  --max-concurrency int Maximum concurrent repo status fetches (default 4)
  --cache-ttl duration  Reuse recent per-repo status cache (default 5m)
  --refresh             Ignore cached workspace status and fetch fresh data
  --json                Emit stable JSON output
  -h, --help            Show help
`

const queueHelp = `Agent-ready workspace queue selection commands.

Usage:
  gira queue list [--config .gira/config.yaml] [--repo OWNER/REPO] [--queue ready|review|finish|blocked|failed|human] [--limit N] [--compact] [--cache-ttl 5m] [--refresh] [--json]
  gira queue next [--config .gira/config.yaml] [--repo OWNER/REPO] [--role implementer] [--profile default] [--compact] [--cache-ttl 5m] [--refresh] [--json]
  gira queue handoff [--config .gira/config.yaml] [--repo OWNER/REPO] [--ticket N] [--role implementer] [--profile default] [--compact] [--cache-ttl 5m] [--refresh] [--json]
  gira queue take [--config .gira/config.yaml] [--repo OWNER/REPO] [--ticket N] [--role implementer] [--profile default] [--compact] [--cache-ttl 5m] [--refresh] --dry-run|--apply [--json]

Commands:
  list     Show queue items derived from workspace-queues/v1
  next     Select the first agent-ready item and print handoff/run commands
  handoff  Select or inspect an agent-ready item and embed worker-handoff/v1
  take     Start a handoff-safe queue item through ticket start

Flags:
  --config string       Explicit workspace config path; defaults to global registry, then ".gira/config.yaml"
  --repo string         Narrow queue selection to an execution repo. Repeatable or comma-separated
  --queue string        Queue filter for list: ready, review, finish, blocked, failed, or human
  --limit int           Maximum queue items to print for list. Default: all
  --ticket int          Explicit ticket number for handoff
  --issue int           Compatibility alias for --ticket on handoff
  --role string         Handoff role: planner, implementer, or reviewer. Default: implementer
  --profile string      Handoff profile: default or python. Default: default
  --compact             Print compact text output
  --max-concurrency int Maximum concurrent repo status fetches (default 4)
  --cache-ttl duration  Reuse recent per-repo status cache (default 5m)
  --refresh             Ignore cached workspace status and fetch fresh data
  --json                Emit stable JSON output
  -h, --help            Show help
`

const workspaceProjectHelp = `Manage workspace GitHub Projects v2 visibility.

Usage:
  gira workspace project plan [--config .gira/config.yaml] [--json]
  gira workspace project adopt --owner OWNER (--title TITLE | --number N) --dry-run|--apply [--config .gira/config.yaml] [--json]

Commands:
  plan    Inspect repository Project visibility without mutation
  adopt   Register an existing profile or org Project in workspace.project

Notes:
  Profile and org Projects are the portfolio board surface.
  Repository issues remain the execution source of truth.
  Use gira projects sync after adoption to mirror repo issues into the configured Project.

Flags:
  --config string  Workspace config path (default ".gira/config.yaml")
  --json           Emit stable JSON output
  -h, --help       Show help
`

const workspaceProjectAdoptHelp = `Register an existing GitHub Project for a Gira workspace.

Usage:
  gira workspace project adopt --owner OWNER --title TITLE --dry-run|--apply [--config .gira/config.yaml] [--json]
  gira workspace project adopt --owner OWNER --number N --dry-run|--apply [--config .gira/config.yaml] [--json]

What it does:
  Finds an existing profile or org GitHub Project owned by OWNER.
  Records it in workspace.project in .gira/config.yaml.
  Does not create Projects and does not replace a different configured Project.

After adoption:
  Run gira projects sync --config .gira/config.yaml --dry-run.
  Run gira projects sync --config .gira/config.yaml --apply when the planned item and status changes look right.

Flags:
  --owner string   GitHub Project owner
  --title string   GitHub Project title
  --number int     GitHub Project number
  --config string  Workspace config path (default ".gira/config.yaml")
  --dry-run        Plan project adoption without mutation
  --apply          Apply project adoption
  --json           Emit stable JSON output
  -h, --help       Show help
`

const projectsHelp = `Sync visible GitHub Projects board items.

Usage:
  gira projects sync --dry-run|--apply [--config .gira/config.yaml] [--archive-closed] [--json]

Commands:
  sync  Mirror configured repo issues into the existing workspace Project

Notes:
  The configured Project can be a profile or org Project linked to a repo.
  Repo issues, labels, and milestones remain the execution source of truth.
  Sync adds missing open issues, mirrors status labels, keeps closed issues Done,
  and can archive closed issue items when --archive-closed is set.

Flags:
  --config string    Explicit workspace config path; defaults to global registry, then ".gira/config.yaml"
  --archive-closed   Archive Project items whose backing issues are closed
  --json             Emit stable JSON output
  -h, --help       Show help
`

const adoptHelp = `Adopt existing GitHub repositories and issues into Gira planning.

Usage:
  gira adopt repo --repo OWNER/REPO --path . --dry-run [--strategy observe|merge|normalize] [--json]
  gira adopt repo --repo OWNER/REPO --path . --strategy observe|merge|normalize --apply [--json]
  gira adopt issues --repo OWNER/REPO --dry-run [--state open|all] [--json]
  gira adopt issues --repo OWNER/REPO --issue N [--issue N] --milestone TITLE [--label LABEL] --dry-run|--apply [--json]
  gira adopt issues --repo OWNER/REPO --issues 1,2,3|1-12 --milestone TITLE [--label LABEL] --dry-run|--apply [--json]
  gira adopt issues --repo OWNER/REPO --state all --normalize-status --dry-run|--apply [--json]

Commands:
  repo    Detect existing repo state and apply a minimal Gira contract
  issues  List unmapped issues and apply explicit milestone/label mappings

Flags:
  --repo string       Target GitHub repo in OWNER/REPO format
  --path string       Local git workspace path for repo adoption (default ".")
  --strategy string   Repo adoption strategy: observe, merge, or normalize
  --yes               Apply the recommended repo adoption strategy
  --issue int         Issue number to map; repeatable
  --issues string     Issue numbers or ranges to map, for example 1,2,3 or 1-12; repeatable
  --milestone string  Milestone title to assign to selected issues
  --label string      Label to add to selected issues; repeatable or comma-separated
  --state string      Issues to inspect: open or all (default "open")
  --normalize-status  Remove active status labels from closed selected issues
  --dry-run           Preview without mutation
  --apply             Apply selected explicit mappings
  --json              Emit stable JSON output
  -h, --help          Show help
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
  gira upgrade [--channel auto|install.sh|uv|pipx|pip|homebrew|npm|bun|go|unknown] [--json]
  gira update [--channel auto|install.sh|uv|pipx|pip|homebrew|npm|bun|go|unknown] [--json]

Flags:
  --channel string  Installed channel to use for the next-step command (default "auto")
  --json            Emit stable JSON upgrade info
  -h, --help        Show help
`

const cacheHelp = `Manage local Gira caches.

Usage:
  gira cache prune --dry-run|--apply [--root PATH] [--json]

Commands:
  prune  Remove stale wrapper-managed native binary cache directories

Flags:
  --root string  Cache root (default: GIRA_PYPI_CACHE_DIR or ~/.cache/gira-cli)
  --dry-run      Preview stale version directories without deleting
  --apply        Delete stale version directories
  --json         Emit stable JSON output
  -h, --help     Show help
`

const configHelp = `Inspect Gira config sources without mutation.

Usage:
  gira config global [--config-root PATH] [--json]
  gira config repo [--repo OWNER/REPO] [--config-root PATH] [--json]
  gira config doctor [--repo OWNER/REPO] [--config-root PATH] [--json]

Commands:
  global  Show the resolved global config root and registry paths
  repo    Show repo-specific global registry and repo-local contract paths
  doctor  Explain which config source is selected and why

Flags:
  --repo string         Target GitHub repo in OWNER/REPO format
  --config-root string  Override global config root for diagnostics
  --json                Emit stable JSON output
  -h, --help            Show help
`

const setupHelp = `Intention-based Gira setup flows.

Usage:
  gira setup global [--repo OWNER/REPO] [--path .] [--workspace NAME] [--inbox-repo OWNER/REPO] [--mode global-only|hybrid] --dry-run|--apply [--config-root PATH] [--overwrite] [--json]

Commands:
  global  Configure OS-user global Gira operation in one dry-run/apply flow

Flags:
  --repo string          Target GitHub repo in OWNER/REPO format. Defaults to git origin or registry context
  --path string          Local checkout path to validate and record. Default: .
  --workspace string     Global workspace name. Default: personal
  --owner string         Workspace/default owner. Default: inbox repo owner
  --inbox-repo string    Backlog/intake repo in OWNER/REPO format. Default: target repo with a single-repo warning
  --project-owner string GitHub Projects v2 owner. Default: workspace owner
  --project-title string GitHub Projects v2 title. Default: workspace name
  --project-number int   GitHub Projects v2 number for disambiguation
  --mode string          global-only ignores repo-local contracts; hybrid references .gira/config.yaml when present. Default: global-only
  --agent string         Default coding agent name
  --assignee string      Default assignee login
  --agent-label string   Preferred agent label. Repeatable or comma-separated
  --config-root string   Override global config root
  --overwrite            Replace existing global setup files with different content
  --dry-run              Preview without writing files
  --apply                Write setup files
  --json                 Emit stable JSON output
  -h, --help             Show help
`

const repoHelp = `Manage Gira global registry entries for repositories.

Usage:
  gira repo register OWNER/REPO [--path PATH] --dry-run|--apply [--config-root PATH] [--overwrite] [--json]
  gira repo migrate [--repo OWNER/REPO] [--path PATH] --dry-run|--apply [--config-root PATH] [--overwrite] [--json]

Commands:
  register  Register a GitHub repository and optional checkout path in the global registry
  migrate   Import repo-local contract metadata into the global registry

Flags:
  --path string        Local checkout path to validate and record
  --repo string        Target GitHub repo in OWNER/REPO format
  --config-root string Override global config root
  --overwrite          Replace a conflicting existing registry entry
  --dry-run            Preview without writing files
  --apply              Write the registry entry
  --json               Emit stable JSON output
  -h, --help           Show help
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

const doctorHelp = `Diagnose install, auth, repo, drift, workflow policy, and local git readiness.

Usage:
  gira doctor [--repo OWNER/REPO] [--json]

Flags:
  --repo string   Target GitHub repo in OWNER/REPO format. Inferred from gh when omitted
  --json          Emit stable JSON report for automation
  -h, --help      Show help
`

const syncHelp = `Sync Gira labels, milestones, and optionally bootstrap issues through gh.

Usage:
  gira ops sync --repo OWNER/REPO [--dry-run] [--bootstrap-issues] [--policy-mode adopt|merge|enforce]
  gira sync --repo OWNER/REPO [--dry-run] [--bootstrap-issues] [--policy-mode adopt|merge|enforce]  (alias)

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
  gira export dashboard --config .gira/config.yaml [--repo OWNER/REPO] [--output PATH] [--dry-run] [--json]

Flags:
  --repo string       Target GitHub repo in OWNER/REPO format
  --config string     Workspace config path for workspace dashboard export
  --limit int         Maximum workspace execution repos to inspect
  --active-only       Include only active workspace repos
  --max-concurrency int
                      Maximum concurrent workspace repo status fetches (default 4)
  --cache-ttl duration
                      Reuse recent per-repo status cache for this duration (default 5m)
  --cache-root string Workspace status cache root
  --refresh           Ignore cached workspace status and fetch fresh data
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
  gira audit readiness --repo OWNER/REPO [--path .gira/audit/*.jsonl] [--json]
  gira audit drift --repo OWNER/REPO [--json]
  gira audit workflow --repo OWNER/REPO [--json]
  gira audit verify --repo OWNER/REPO --path .gira/audit/*.jsonl [--json]

Commands:
  readiness    Inspect repo readiness, audit health, and next Gira step
  drift        Inspect issue/PR workflow convergence, checks, evidence, and telemetry drift
  workflow     Inspect issue/PR workflow convergence and provenance drift
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

const jiraHelp = `Jira provider, import, and export command family.

Usage:
  gira jira init --repo OWNER/REPO --api-base URL --project KEY --dry-run|--apply [--config-root PATH] [--overwrite] [--json]
  gira jira doctor --repo OWNER/REPO [--project KEY] [--api-base URL] [--sample-key JIRA-123] [--config-root PATH] [--json]
  gira jira mirror JIRA-123 --repo OWNER/REPO --dry-run|--apply [--api-base URL] [--config-root PATH] [--json]
  gira jira transition JIRA-123 --repo OWNER/REPO --to ready|in_progress|review|done --dry-run [--api-base URL] [--config-root PATH] [--json]
  gira jira import --repo OWNER/REPO --source PATH --dry-run|--apply [--json]
  gira jira import --repo OWNER/REPO --api-base URL --project KEY --dry-run|--apply [--json]
  gira jira export --repo OWNER/REPO --output PATH [--json]

Commands:
  init        Discover a Jira project and write reviewed non-secret provider config
  doctor      Diagnose Jira-primary compatibility without mutating Jira or GitHub
  mirror      Create or reuse a GitHub mirror issue for one Jira key
  transition  Plan one Jira status transition without mutation
  import      Import Jira CSV/JSON or read-only Jira API issues into GitHub issues
  export      Export GitHub issue state into Jira-friendly JSON and CSV artifacts

Flags:
  --repo string      Target GitHub repo in OWNER/REPO format
  --source string    CSV or JSON import source path
  --api-base string  Jira API base URL, for example https://example.atlassian.net
  --project string   Jira project key
  --sample-key string Representative Jira issue key for transition diagnostics
  --to string        Target Gira status for Jira transition planning
  --config-root PATH Override the global Gira config root
  --overwrite        Replace an existing providers.jira block after review
  --output string    Output directory for export artifacts
  --dry-run          Preview without mutation
  --apply            Apply supported Jira command mutations after review
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

const milestoneHelp = `Milestone command family.

Usage:
  gira milestone new "TITLE" [--repo OWNER/REPO] [--description TEXT] [--due-on YYYY-MM-DD] --dry-run|--apply [--json]
  gira milestone list [--repo OWNER/REPO] [--state open|closed|all] [--json]
  gira milestone status MILESTONE [--repo OWNER/REPO] [--json]
  gira milestone assign MILESTONE --tickets 1,2,3 [--repo OWNER/REPO] --dry-run|--apply [--json]
  gira milestone plan MILESTONE [--repo OWNER/REPO] [--label LABEL] [--state open|closed|all] [--limit N] --dry-run|--apply [--json]
`

const ticketHelp = `Jira-style ticket lifecycle commands.

Usage:
  gira ticket new "Title" --dry-run|--apply [--body TEXT|--body-file PATH|-] [--start] [--json]
  gira ticket list [--repo OWNER/REPO] [--state open|closed|all] [--label LABEL] [--assignee LOGIN] [--milestone TITLE] [--limit N] [--json]
  gira ticket view|show [TICKET|JIRA-KEY] [--repo OWNER/REPO] [--json]
  gira ticket prompt [TICKET] [planner|implementer|reviewer] [--role planner|implementer|reviewer] [--profile default|python] [--repo OWNER/REPO] [--pr N] [--json]
  gira ticket handoff [TICKET] [planner|implementer|reviewer] [--role planner|implementer|reviewer] [--profile default|python] [--repo OWNER/REPO] [--json]
  gira ticket review [TICKET] [--repo OWNER/REPO] [--pr N] [--diff-summary] [--include-diff] [--json|--html --output PATH]
  gira ticket self-review [TICKET] [--repo OWNER/REPO] [--pr N] [--diff-summary] --dry-run|--apply [--json]
  gira ticket start [TICKET|JIRA-KEY] --dry-run|--apply [--repo OWNER/REPO] [--base BRANCH] [--json]
  gira ticket pr [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--draft] [--json]
  gira ticket note [TICKET] "BODY" --dry-run|--apply [--repo OWNER/REPO] [--kind progress|blocker|decision|handoff|summary|check] [--target auto|issue|pr|both] [--body TEXT|--body-file PATH|-] [--json]
  gira ticket supersede [TICKET] --replacement-title TITLE --body-file PATH|- --dry-run|--apply [--repo OWNER/REPO] [--close-draft-pr] [--json]
  gira ticket checks [TICKET] [--repo OWNER/REPO] [--json]
  gira ticket wait [TICKET] [--repo OWNER/REPO] [--timeout 5m] [--interval 5s] [--json]
  gira ticket finish [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--wait 0s] [--sync-local] [--json]
  gira ticket status [TICKET] [--repo OWNER/REPO] [--json|--html --output PATH]

Commands:
  new     Create a repo-bound executable ticket with a structured Gira body
  list    List repo tickets with compact GitHub issue-backed filters
  view    Show an operating card for the ticket, linked PR, blockers, and next action. Alias: show
  prompt  Render a stateless planner, implementer, or reviewer prompt from ticket context
  handoff Compile a worker-neutral handoff packet from ticket context
  review  Render a reviewer packet from current ticket and linked PR state
  self-review Render and optionally post a self-review check note to the linked PR
  start   Verify a ready ticket, create/reuse its branch, and move to in-progress on apply. Alias: gira start
  pr      Validate or create a linked PR with Closes #N and update review status on apply
  note    Post a structured context note to the issue, linked PR, or both
  supersede Close a ticket as superseded and create a linked replacement ticket
  checks  Show linked PR checks, review blockers, and next action
  wait    Wait for pending linked PR checks without merging
  finish  Mark the linked PR ready when needed, merge safely, and report convergence
  status  Report ticket status, linked PR blockers, and next action

Flags:
  --repo string    Target GitHub repo in OWNER/REPO format. Defaults to .gira config or git origin
  --ticket int     Ticket number. GitHub issue number in v1. Can also be numeric positional
  --issue int      Compatibility alias for --ticket
  --state string   Ticket list state filter: open, closed, or all. Default: open
  --label string   Ticket list filter, or existing repo label for ticket new. Repeatable or comma-separated
  --body string    Full issue body for ticket new. Overrides structured goal/scope fields
  --body-file string Read full issue body from file, or "-" for stdin
  --kind string    Ticket note kind: progress, blocker, decision, handoff, summary, or check. Default: progress
  --target string  Ticket note target: auto, issue, pr, or both. Default: auto
  --role string    Ticket prompt/handoff role: planner, implementer, or reviewer
  --profile string Ticket prompt/handoff profile: default or python. Default: default
  --pr int         Optional PR number for reviewer prompt context
  --diff-summary  Include a compact PR diff summary in reviewer/self-review output
  --include-diff  Include full PR diff in reviewer output. Use explicitly; output can be long
  --replacement-title string Replacement ticket title for ticket supersede
  --close-draft-pr   Close a linked draft PR when superseding
  --assignee string Ticket list assignee login
  --milestone string Ticket list milestone title
  --limit int      Ticket list item limit. Default: 30
  --dry-run        Preview without mutation
  --apply          Apply branch, PR, and status label changes
  --draft          Create/keep PR as draft for ticket pr
  --wait duration  Optional pending-check wait for ticket finish. Default: 0s
  --timeout duration  Pending-check wait timeout for ticket wait. Default: 5m
  --interval duration  Poll interval for ticket wait. Default: 5s
  --start          Start a newly created ticket after ticket new --apply
  --json           Emit stable JSON output
  --html           Write a static local HTML report
  --output string  Output path for --html
  -h, --help       Show help
`

const runHelp = `Local Codex run manifests and execution state.

Usage:
  gira run start [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--role implementer] [--profile default] [--context TEXT|--context-file PATH|-] [--name NAME] [--state-root PATH] [--workdir PATH] [--exec] [--json]
  gira run status --latest|--id RUN_ID [--ticket N] [--repo OWNER/REPO] [--state-root PATH] [--json]
  gira run collect --latest|--id RUN_ID [--ticket N] [--repo OWNER/REPO] [--state-root PATH] [--json]

Commands:
  start    Store a ticket handoff prompt and prepared Codex command in private local state
  status   Show the selected local run manifest
  collect  Print the selected run result file

Flags:
  --repo string       Target GitHub repo in OWNER/REPO format. start defaults to .gira config or git origin
  --ticket int        Ticket number. GitHub issue number in v1. Can also be numeric positional for start
  --issue int         Compatibility alias for --ticket on start
  --role string       Ticket handoff role: planner, implementer, or reviewer. Default: implementer
  --profile string    Ticket handoff profile: default or python. Default: default
  --context string    Small operator note to include in the private run prompt
  --context-file path Read a small operator note from file, or "-" for stdin
  --name string       Optional human-readable local run name
  --id string         Optional run id for status/collect, or custom run id for start
  --latest           Select the newest matching local run
  --state-root path   Override private local Gira state root
  --workdir path      Codex working directory for start. Default: current directory
  --exec             Start Codex in the background after writing the manifest
  --dry-run          Preview without writing local run files
  --apply            Write private local run files
  --json             Emit stable JSON output
  -h, --help         Show help
`

const goalHelp = `Goal-mode commands for long-running AI-assisted work.

Usage:
  gira goal plan [GOAL] --dry-run|--apply [--repo OWNER/REPO] [--json]
  gira goal report [GOAL] [--repo OWNER/REPO] [--json|--html --output PATH]
  gira goal status [GOAL] [--repo OWNER/REPO] [--json]
  gira goal next [GOAL] [--repo OWNER/REPO] [--json]
  gira goal finish [GOAL] --dry-run|--apply [--repo OWNER/REPO] [--terminal done|human_review|blocked|superseded|abandoned] [--json]

Commands:
  plan    Propose or create child ticket packets from a goal issue
  report  Build a visible goal report. Alias: dossier
  status  Summarize a goal issue, child ticket graph, blockers, and next safe action
  next    Select the next safe child ticket or explain why the goal must stop
  finish  Preview goal finish readiness, then close ready goals or preserve handoffs

Flags:
  --repo string  Target GitHub repo in OWNER/REPO format. Defaults to .gira config or git origin
  --goal int     Goal issue number. Can also be numeric positional
  --dry-run      Preview without mutation
  --apply        Apply goal plan child ticket creation or explicit supported goal finish mutations
  --terminal string Goal terminal recommendation override for finish
  --json         Emit stable goal JSON
  --html         Write a static local HTML report
  --output path  Output path for --html
  -h, --help     Show help
`

const epicHelp = `Jira-style epic lifecycle commands.

Usage:
  gira epic list [--repo OWNER/REPO] [--state open|closed|all] [--label LABEL] [--assignee LOGIN] [--milestone TITLE] [--limit N] [--json]
  gira epic status [--repo OWNER/REPO] [--ticket N] [--title TEXT] [--slug SLUG] [--milestone TITLE] [--json]
  gira epic finish --dry-run|--apply [--repo OWNER/REPO] [--ticket N] [--title TEXT] [--slug SLUG] [--milestone TITLE] [--json]

Commands:
  list    List repo epics backed by type:epic GitHub issues
  status  Resolve an epic without requiring its number and show child readiness
  finish  Close an epic through Gira when all child issues are closed

Selection:
  Gira first uses --ticket when provided, then the current issue-N-* branch or linked PR context.
  Without a ticket context, --title, --slug, and --milestone narrow open type:epic issues.
  If exactly one open epic remains in the repo, Gira selects it automatically.

Flags:
  --repo string       Target GitHub repo in OWNER/REPO format. Defaults to .gira config or git origin
  --ticket int        Epic issue number. Advanced fallback when context is ambiguous
  --issue int         Compatibility alias for --ticket
  --title string      Match an open epic title by substring
  --slug string       Match an open epic title slug
  --milestone string  Match an open epic milestone title
  --state string      Epic list state filter: open, closed, or all. Default: open
  --label string      Additional epic list label filter. Repeatable or comma-separated
  --assignee string   Epic list assignee login
  --limit int         Epic list item limit. Default: 30
  --dry-run           Preview without mutation
  --apply             Apply close and status label changes
  --json              Emit stable JSON output
  -h, --help          Show help
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
  gira review gate [--json] [--local-exec]
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

const featureHelp = `Optional issue-backed feature map commands.

Usage:
  gira feature list [--repo OWNER/REPO] [--limit N] [--json]
  gira feature check [--repo OWNER/REPO] [--limit N] [--json]
  gira feature for ISSUE [--repo OWNER/REPO] [--limit N] [--json]
  gira feat list|check|for ...  (alias)

Commands:
  list   List feature/capability records backed by GitHub issues
  check  Validate optional feature map records and work links
  for    Show which feature/capability a work issue is linked to

Conventions:
  Feature records are GitHub issues labeled type:capability or type:feature,
  or issues whose title starts with Capability: or Feature:.
  Short daily keys can be recorded in the issue body as Key: VALUE.
  Work issues can link back with Related capability: #N or Feature: #N.
  GitHub Projects are visibility views; GitHub issues remain canonical.

Flags:
  --repo string  Target GitHub repo in OWNER/REPO format. Defaults to .gira config or git origin
  --issue int    Work issue number for feature for. Can also be numeric positional
  --limit int    Max issues to inspect. Default: 1000
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

var listStatusReposForOwner = func(owner string, limit int, includeArchived bool) ([]gira.RepoRef, error) {
	return ghStatusReposForOwner(owner, limit, includeArchived, gira.ExecCommandRunner{})
}

var newOnboardVerifyReport = func(repo gira.RepoRef, stage gira.OnboardStage) (gira.OnboardVerifyReport, error) {
	return gira.BuildOnboardVerifyReport(repo, stage, gira.ExecCommandRunner{}, time.Now().UTC()), nil
}

var newDoctorReport = func(repoValue string) gira.DoctorReport {
	return gira.BuildDoctorReport(repoValue, gira.ExecCommandRunner{}, time.Now().UTC())
}

var newAuditReadinessReport = func(repo gira.RepoRef, ledgerPath string) gira.AuditReadinessReport {
	return gira.BuildAuditReadinessReport(repo, ledgerPath, gira.ExecCommandRunner{}, time.Now().UTC())
}

var newAuditWorkflowReport = func(repo gira.RepoRef) (gira.WorkflowAuditReport, error) {
	return gira.BuildWorkflowAuditReport(repo, gira.ExecCommandRunner{}, time.Now().UTC())
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

var newWorkspaceDashboardExportBundle = func(configPath string, outputRoot string, snapshotAt time.Time, dryRun bool, options gira.WorkspaceStatusOptions) (gira.DashboardExportPlan, gira.DashboardExportBundle, error) {
	resolved, err := gira.ResolveWorkspaceConfig(configPath)
	if err != nil {
		return gira.DashboardExportPlan{}, gira.DashboardExportBundle{}, err
	}
	return gira.BuildWorkspaceDashboardExportPlan(resolved, outputRoot, snapshotAt, dryRun, gira.NewGHWorkspaceClient(gira.ExecCommandRunner{}), 14, options)
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
	return newWorkspaceStatusReportWithOptions(configPath, gira.WorkspaceStatusOptions{})
}

var newWorkspaceStatusReportWithOptions = func(configPath string, options gira.WorkspaceStatusOptions) (gira.WorkspaceReport, error) {
	resolved, err := gira.ResolveWorkspaceConfig(configPath)
	if err != nil {
		return gira.WorkspaceReport{}, err
	}
	return gira.BuildWorkspaceStatusReportWithOptions(resolved, gira.NewGHWorkspaceClient(gira.ExecCommandRunner{}), time.Now(), 14, options)
}

var newWorkspaceInitReport = func(input gira.WorkspaceInitInput) (gira.WorkspaceInitReport, error) {
	return gira.BuildWorkspaceInitReport(input)
}

var newWorkspaceCapabilityReport = func(configPath string) (gira.WorkspaceCapabilityReport, error) {
	resolved, err := gira.ResolveWorkspaceConfig(configPath)
	if err != nil {
		return gira.WorkspaceCapabilityReport{}, err
	}
	return gira.BuildWorkspaceCapabilityReport(resolved, gira.ExecCommandRunner{}, time.Now())
}

var newWorkspaceValidateReport = func(configPath string) (gira.WorkspaceValidateReport, error) {
	resolved, err := gira.ResolveWorkspaceConfig(configPath)
	if err != nil {
		return gira.WorkspaceValidateReport{}, err
	}
	return gira.BuildWorkspaceValidateReport(resolved, gira.NewGHWorkspaceClient(gira.ExecCommandRunner{}))
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

var newWorkspaceRepoSyncReport = func(input gira.WorkspaceRepoSyncInput) (gira.WorkspaceRepoSyncReport, error) {
	return gira.BuildWorkspaceRepoSyncReport(input, gira.ExecCommandRunner{})
}

var newWorkspaceTicketNewReport = func(configPath string, title string, body string, targetRepo gira.RepoRef, route bool, dryRun bool) (gira.WorkspaceTicketNewReport, error) {
	resolved, err := gira.ResolveWorkspaceConfig(configPath)
	if err != nil {
		return gira.WorkspaceTicketNewReport{}, err
	}
	if route {
		return gira.BuildWorkspaceTicketNewRouteReport(resolved, gira.NewGHWorkspaceClient(gira.ExecCommandRunner{}), title, body, targetRepo, dryRun)
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

var newWorkspaceProjectAdoptReport = func(input gira.WorkspaceProjectAdoptInput) (gira.WorkspaceProjectAdoptReport, error) {
	return gira.BuildWorkspaceProjectAdoptReport(input, gira.NewGHProjectsSyncClient(gira.ExecCommandRunner{}))
}

var newProjectsSyncReport = func(configPath string, dryRun bool, archiveClosed bool) (gira.ProjectsSyncReport, error) {
	resolved, err := gira.ResolveProjectsSyncWorkspaceConfig(configPath)
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

var newSprintRolloverReport = func(repo gira.RepoRef, toMilestone string, apply bool) (gira.SprintRolloverReport, error) {
	return gira.SprintRollover(repo, toMilestone, apply, time.Now(), gira.ExecCommandRunner{})
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

var newJiraProviderInitReport = func(input gira.JiraProviderInitInput) (gira.JiraProviderInitReport, error) {
	return gira.BuildJiraProviderInitReport(input)
}

var newJiraMirrorReport = func(input gira.JiraMirrorInput) (gira.JiraMirrorReport, error) {
	return gira.MirrorJiraIssue(input, jiraCommandRunner)
}

var newJiraDoctorReport = func(input gira.JiraDoctorInput) (gira.JiraDoctorReport, error) {
	return gira.BuildJiraDoctorReport(input, jiraCommandRunner)
}

var newJiraTransitionPlanReport = func(input gira.JiraTransitionPlanInput) (gira.JiraTransitionPlanReport, error) {
	return gira.BuildJiraTransitionPlan(input)
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

var newWorkStartResultWithOptions = func(repo gira.RepoRef, issue int, options gira.WorkStartOptions) (gira.WorkStartResult, error) {
	if strings.TrimSpace(options.BaseOverride) == "" {
		return newWorkStartResult(repo, issue, options.DryRun)
	}
	return gira.StartWorkWithOptions(repo, issue, options, devCommandRunner)
}

var newWorkPRResult = func(repo gira.RepoRef, issue int, dryRun bool, draft bool) (gira.WorkPRResult, error) {
	return gira.OpenWorkPR(repo, issue, dryRun, draft, devCommandRunner)
}

var newWorkStatusResult = func(repo gira.RepoRef, issue int) (gira.WorkStatusResult, error) {
	return gira.GetWorkStatus(repo, issue, devCommandRunner)
}

var newWorkFinishResult = func(repo gira.RepoRef, issue int, dryRun bool, wait time.Duration, options gira.WorkFinishOptions) (gira.WorkFinishResult, error) {
	return gira.FinishWorkWithOptions(repo, issue, dryRun, wait, options, devCommandRunner)
}

var newTicketNewReport = func(input gira.TicketNewInput) (gira.TicketNewReport, error) {
	return gira.BuildTicketNewReport(input, devCommandRunner)
}

var newTicketListReport = func(options gira.TicketListOptions) (gira.TicketListReport, error) {
	return gira.BuildTicketListReport(options, devCommandRunner)
}

var newFeatureMapListReport = func(options gira.FeatureMapOptions) (gira.FeatureMapListReport, error) {
	return gira.BuildFeatureMapListReport(options, devCommandRunner)
}

var newFeatureMapCheckReport = func(options gira.FeatureMapOptions) (gira.FeatureMapCheckReport, error) {
	return gira.BuildFeatureMapCheckReport(options, devCommandRunner)
}

var newFeatureMapForReport = func(options gira.FeatureForOptions) (gira.FeatureMapForReport, error) {
	return gira.BuildFeatureMapForReport(options, devCommandRunner)
}

var newMilestoneNewReport = func(input gira.MilestoneNewInput) (gira.MilestoneReport, error) {
	return gira.BuildMilestoneNewReport(input, devCommandRunner)
}

var newMilestoneListReport = func(options gira.MilestoneListOptions) (gira.MilestoneReport, error) {
	return gira.BuildMilestoneListReport(options, devCommandRunner)
}

var newMilestoneStatusReport = func(options gira.MilestoneStatusOptions) (gira.MilestoneReport, error) {
	return gira.BuildMilestoneStatusReport(options, devCommandRunner)
}

var newMilestoneAssignReport = func(input gira.MilestoneAssignInput) (gira.MilestoneReport, error) {
	return gira.BuildMilestoneAssignReport(input, devCommandRunner)
}

var newMilestonePlanReport = func(input gira.MilestonePlanInput) (gira.MilestoneReport, error) {
	return gira.BuildMilestonePlanReport(input, devCommandRunner)
}

var newTicketChecksReport = func(repo gira.RepoRef, issue int, wait time.Duration, pollInterval time.Duration) (gira.TicketChecksReport, error) {
	return gira.BuildTicketChecksReport(repo, issue, wait, pollInterval, devCommandRunner)
}

var newTicketViewReport = func(repo gira.RepoRef, issue int) (gira.TicketViewReport, error) {
	return gira.BuildTicketViewReport(repo, issue, devCommandRunner)
}

var newTicketPromptReport = func(input gira.AgentPromptInput) (gira.AgentPromptReport, error) {
	return gira.BuildAgentPromptReport(input, devCommandRunner)
}

var newTicketHandoffReport = func(input gira.TicketHandoffInput) (gira.TicketHandoffReport, error) {
	return gira.BuildTicketHandoffReport(input, devCommandRunner)
}

var newGoalStatusReport = func(input gira.GoalStatusInput) (gira.GoalStatusReport, error) {
	return gira.BuildGoalStatusReport(input, devCommandRunner)
}

var newGoalDossierReport = func(input gira.GoalDossierInput) (gira.GoalDossierReport, error) {
	return gira.BuildGoalDossierReport(input, devCommandRunner)
}

var newGoalNextReport = func(input gira.GoalNextInput) (gira.GoalNextReport, error) {
	return gira.BuildGoalNextReport(input, devCommandRunner)
}

var newGoalPlanReport = func(input gira.GoalPlanInput) (gira.GoalPlanReport, error) {
	return gira.BuildGoalPlanReport(input, devCommandRunner)
}

var newGoalFinishReport = func(input gira.GoalFinishInput) (gira.GoalFinishReport, error) {
	return gira.BuildGoalFinishReport(input, devCommandRunner)
}

var newJiraMirrorIssueResolver = func(repo gira.RepoRef, key string) (gira.JiraMirrorIssue, error) {
	return gira.ResolveJiraMirrorIssue(repo, key, devCommandRunner)
}

var newTicketNoteReport = func(input gira.TicketNoteInput) (gira.TicketNoteReport, error) {
	return gira.BuildTicketNoteReport(input, devCommandRunner)
}

var newTicketSelfReviewReport = func(input gira.TicketSelfReviewInput) (gira.TicketSelfReviewReport, error) {
	return gira.BuildTicketSelfReviewReport(input, devCommandRunner)
}

var newTicketSupersedeReport = func(input gira.TicketSupersedeInput) (gira.TicketSupersedeReport, error) {
	return gira.BuildTicketSupersedeReport(input, devCommandRunner)
}

var newEpicStatusReport = func(input gira.EpicInput) (gira.EpicReport, error) {
	return gira.BuildEpicStatusReport(input, devCommandRunner)
}

var newEpicFinishReport = func(input gira.EpicInput) (gira.EpicReport, error) {
	return gira.FinishEpic(input, devCommandRunner)
}

var newUpgradeReport = func(channel string) (gira.UpgradeReport, error) {
	executable, _ := os.Executable()
	return gira.BuildUpgradeReport(channel, executable, nil)
}

var newCachePruneReport = func(options gira.CachePruneOptions) (gira.CachePruneReport, error) {
	executable, _ := os.Executable()
	options.ExecutablePath = executable
	options.ActiveVersion = gira.BuildVersionInfo().Version
	return gira.BuildCachePruneReport(options)
}

var newConfigGlobalReport = func(configRoot string) (gira.ConfigGlobalReport, error) {
	return gira.BuildConfigGlobalReport(configRoot)
}

var newConfigRepoReport = func(repoValue string, configRoot string) (gira.ConfigRepoReport, error) {
	return gira.BuildConfigRepoReport(repoValue, configRoot, repoContextRunner)
}

var newConfigDoctorReport = func(repoValue string, configRoot string) (gira.ConfigDoctorReport, error) {
	return gira.BuildConfigDoctorReport(repoValue, configRoot, repoContextRunner)
}

var newSetupGlobalReport = func(input gira.SetupGlobalInput) (gira.SetupGlobalReport, error) {
	return gira.BuildSetupGlobalReport(input, devCommandRunner)
}

var newRepoRegisterReport = func(input gira.RepoRegisterInput) (gira.RepoRegisterReport, error) {
	return gira.BuildRepoRegisterReport(input, devCommandRunner)
}

var newRepoMigrateReport = func(input gira.RepoMigrateInput) (gira.RepoMigrateReport, error) {
	return gira.BuildRepoMigrateReport(input, devCommandRunner)
}

var newAdoptIssuesReport = func(input gira.AdoptIssueInput) (gira.AdoptIssuesReport, error) {
	return gira.BuildAdoptIssuesReport(input, gira.ExecCommandRunner{})
}

var newAdoptRepoReport = func(input gira.AdoptRepoInput) (gira.AdoptRepoReport, error) {
	return gira.BuildAdoptRepoReport(input, gira.ExecCommandRunner{})
}

var newStatsRepoReport = func(input gira.StatsRepoOptions) (gira.StatsRepoReport, error) {
	input.Now = reportNow()
	return gira.BuildStatsRepoReport(input, gira.ExecCommandRunner{})
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
	case "setup":
		return runSetup(args[1:], stdout, stderr)
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "workspace":
		return runWorkspace(args[1:], stdout, stderr)
	case "queue":
		return runQueue(args[1:], stdout, stderr)
	case "projects":
		return runProjects(args[1:], stdout, stderr)
	case "repo":
		return runRepo(args[1:], stdout, stderr)
	case "adopt":
		return runAdopt(args[1:], stdout, stderr)
	case "start":
		return runTicketStart(args[1:], stdout, stderr)
	case "ticket":
		return runTicket(args[1:], stdout, stderr)
	case "run":
		return runRun(args[1:], stdout, stderr)
	case "feature", "feat":
		return runFeature(args[1:], stdout, stderr)
	case "goal":
		return runGoal(args[1:], stdout, stderr)
	case "epic":
		return runEpic(args[1:], stdout, stderr)
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
	case "stats":
		return runStats(args[1:], stdout, stderr)
	case "config":
		return runConfig(args[1:], stdout, stderr)
	case "upgrade", "update":
		return runUpgrade(args[1:], stdout, stderr)
	case "cache":
		return runCache(args[1:], stdout, stderr)
	case "completion":
		return runCompletion(args[1:], stdout, stderr)
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
	case "milestone":
		return runMilestone(args[1:], stdout, stderr)
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
	if args[0] == "capabilities" {
		return runGuideCapabilities(args[1:], stdout, stderr)
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
		fmt.Fprint(stdout, renderTicketGuide())
	case "stats":
		fmt.Fprint(stdout, guideStats)
	case "jira":
		fmt.Fprint(stdout, guideJira)
	case "agent", "skill":
		fmt.Fprint(stdout, renderAgentGuide())
	case "concepts":
		fmt.Fprint(stdout, guideConcepts)
	default:
		fmt.Fprintf(stderr, "unknown guide topic: %s\n\n", args[0])
		fmt.Fprint(stderr, guideHelp)
		return 2
	}
	return 0
}

func runGuideCapabilities(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("guide capabilities", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOutput := fs.Bool("json", false, "Emit stable JSON capability map")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "guide capabilities: %v\n\n", err)
		fmt.Fprint(stderr, guideHelp)
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, guideHelp)
		return 2
	}
	if *jsonOutput {
		if err := json.NewEncoder(stdout).Encode(gira.BuildCommandCapabilityReport(gira.CoreCommandSpecs())); err != nil {
			fmt.Fprintf(stderr, "encode command capabilities JSON: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprint(stdout, gira.RenderCommandCapabilitiesMarkdown(gira.CoreCommandSpecs()))
	return 0
}

func renderTicketGuide() string {
	return guideTicketIntro + gira.RenderGuideCommandSection("ticket", gira.CoreCommandSpecs()) + "\n"
}

func renderAgentGuide() string {
	return gira.RenderAgentGuide(gira.CoreAgentGuidanceSpec(), gira.CoreCommandSpecs())
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

func runCache(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprint(stdout, cacheHelp)
		return 0
	}
	switch args[0] {
	case "prune":
		return runCachePrune(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown cache command: %s\n\n", args[0])
		fmt.Fprint(stderr, cacheHelp)
		return 2
	}
}

func runConfig(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, configHelp)
		return 0
	}
	switch args[0] {
	case "--help", "-h":
		fmt.Fprint(stdout, configHelp)
		return 0
	case "global":
		return runConfigGlobal(args[1:], stdout, stderr)
	case "repo":
		return runConfigRepo(args[1:], stdout, stderr)
	case "doctor":
		return runConfigDoctor(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown config command: %s\n\n", args[0])
		fmt.Fprint(stderr, configHelp)
		return 2
	}
}

func runConfigGlobal(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("config global", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configRoot := fs.String("config-root", "", "Override global config root")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, configHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, configHelp)
		return 0
	}
	report, err := newConfigGlobalReport(*configRoot)
	if err != nil {
		fmt.Fprintf(stderr, "config global failed: %v\n", err)
		return 1
	}
	if *jsonOutput {
		out, _ := json.MarshalIndent(report, "", "  ")
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatConfigGlobalReport(report))
	return 0
}

func runConfigRepo(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("config repo", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	configRoot := fs.String("config-root", "", "Override global config root")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, configHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, configHelp)
		return 0
	}
	report, err := newConfigRepoReport(*repoValue, *configRoot)
	if err != nil {
		fmt.Fprintf(stderr, "config repo failed: %v\n", err)
		return 1
	}
	if *jsonOutput {
		out, _ := json.MarshalIndent(report, "", "  ")
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatConfigRepoReport(report))
	return 0
}

func runConfigDoctor(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("config doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	configRoot := fs.String("config-root", "", "Override global config root")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, configHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, configHelp)
		return 0
	}
	report, err := newConfigDoctorReport(*repoValue, *configRoot)
	if err != nil {
		fmt.Fprintf(stderr, "config doctor failed: %v\n", err)
		return 1
	}
	if *jsonOutput {
		out, _ := json.MarshalIndent(report, "", "  ")
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatConfigDoctorReport(report))
	return 0
}

func runSetup(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, setupHelp)
		return 0
	}
	switch args[0] {
	case "--help", "-h":
		fmt.Fprint(stdout, setupHelp)
		return 0
	case "global":
		return runSetupGlobal(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown setup command: %s\n\n", args[0])
		fmt.Fprint(stderr, setupHelp)
		return 2
	}
}

func runSetupGlobal(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("setup global", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	pathValue := fs.String("path", ".", "Local checkout path to validate and record")
	configRoot := fs.String("config-root", "", "Override global config root")
	workspaceName := fs.String("workspace", "personal", "Global workspace name")
	owner := fs.String("owner", "", "Workspace/default owner")
	inboxRepo := fs.String("inbox-repo", "", "Inbox repo in OWNER/REPO format")
	projectOwner := fs.String("project-owner", "", "GitHub Projects v2 owner; defaults to workspace owner")
	projectTitle := fs.String("project-title", "", "GitHub Projects v2 title; defaults to workspace name")
	projectNumber := fs.Int("project-number", 0, "GitHub Projects v2 number for disambiguation")
	mode := fs.String("mode", "global-only", "Setup mode: global-only or hybrid")
	agent := fs.String("agent", "", "Default coding agent name")
	assignee := fs.String("assignee", "", "Default assignee login")
	overwrite := fs.Bool("overwrite", false, "Replace conflicting existing setup files")
	dryRun := fs.Bool("dry-run", false, "Preview without writing files")
	apply := fs.Bool("apply", false, "Write setup files")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	var agentLabels repeatedStringFlag
	fs.Var(&agentLabels, "agent-label", "Preferred agent label. Repeatable or comma-separated")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, setupHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, setupHelp)
		return 0
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, setupHelp)
		return 2
	}
	if *dryRun == *apply {
		fmt.Fprint(stderr, "exactly one of --dry-run/--apply is required\n\n")
		fmt.Fprint(stderr, setupHelp)
		return 2
	}
	var repo gira.RepoRef
	if strings.TrimSpace(*repoValue) != "" {
		parsed, err := gira.ParseRepoRef(*repoValue)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n\n", err)
			fmt.Fprint(stderr, setupHelp)
			return 2
		}
		repo = parsed
	}
	report, err := newSetupGlobalReport(gira.SetupGlobalInput{
		Repo:          repo,
		Path:          *pathValue,
		ConfigRoot:    *configRoot,
		WorkspaceName: *workspaceName,
		Owner:         *owner,
		InboxRepo:     *inboxRepo,
		ProjectOwner:  *projectOwner,
		ProjectTitle:  *projectTitle,
		ProjectNumber: *projectNumber,
		Mode:          *mode,
		Agent:         *agent,
		Assignee:      *assignee,
		AgentLabels:   agentLabels,
		Overwrite:     *overwrite,
		DryRun:        *dryRun,
		Apply:         *apply,
	})
	if err != nil {
		if *jsonOutput {
			gira.EnsureSetupGlobalReportSchema(&report)
			if *dryRun && strings.TrimSpace(report.Command) != "" {
				report.Approval = gira.SetupGlobalApprovalEvidence(report)
			}
			out, _ := json.MarshalIndent(report, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
		}
		fmt.Fprintf(stderr, "setup global failed: %v\n", err)
		return 1
	}
	if *jsonOutput {
		gira.EnsureSetupGlobalReportSchema(&report)
		if *dryRun {
			report.Approval = gira.SetupGlobalApprovalEvidence(report)
		}
		out, _ := json.MarshalIndent(report, "", "  ")
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatSetupGlobalReport(report))
	return 0
}

func runRepo(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, repoHelp)
		return 0
	}
	switch args[0] {
	case "--help", "-h":
		fmt.Fprint(stdout, repoHelp)
		return 0
	case "register":
		return runRepoRegister(args[1:], stdout, stderr)
	case "migrate":
		return runRepoMigrate(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown repo command: %s\n\n", args[0])
		fmt.Fprint(stderr, repoHelp)
		return 2
	}
}

func runRepoRegister(args []string, stdout io.Writer, stderr io.Writer) int {
	args, repoArg := extractRepoRegisterPositional(args)
	fs := flag.NewFlagSet("repo register", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	pathValue := fs.String("path", "", "Local checkout path to validate and record")
	configRoot := fs.String("config-root", "", "Override global config root")
	overwrite := fs.Bool("overwrite", false, "Replace conflicting existing registry entry")
	dryRun := fs.Bool("dry-run", false, "Preview without writing files")
	apply := fs.Bool("apply", false, "Write the registry entry")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, repoHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, repoHelp)
		return 0
	}
	if repoArg == "" || fs.NArg() != 0 {
		fmt.Fprint(stderr, "repo register requires OWNER/REPO\n\n")
		fmt.Fprint(stderr, repoHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(repoArg)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, repoHelp)
		return 2
	}
	report, err := newRepoRegisterReport(gira.RepoRegisterInput{Repo: repo, Path: *pathValue, ConfigRoot: *configRoot, Overwrite: *overwrite, DryRun: *dryRun, Apply: *apply})
	if err != nil {
		if *jsonOutput {
			gira.EnsureRepoRegisterReportSchema(&report)
			if *dryRun && strings.TrimSpace(report.Repo) != "" {
				report.Approval = gira.RepoRegisterApprovalEvidence(report)
			}
			out, _ := json.MarshalIndent(report, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
		}
		fmt.Fprintf(stderr, "repo register failed: %v\n", err)
		return 1
	}
	if *jsonOutput {
		gira.EnsureRepoRegisterReportSchema(&report)
		if *dryRun {
			report.Approval = gira.RepoRegisterApprovalEvidence(report)
		}
		out, _ := json.MarshalIndent(report, "", "  ")
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatRepoRegisterReport(report))
	return 0
}

func extractRepoRegisterPositional(args []string) ([]string, string) {
	var repo string
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--path" || arg == "--config-root":
			out = append(out, arg)
			if i+1 < len(args) {
				i++
				out = append(out, args[i])
			}
		case strings.HasPrefix(arg, "--path=") || strings.HasPrefix(arg, "--config-root="):
			out = append(out, arg)
		case strings.HasPrefix(arg, "-"):
			out = append(out, arg)
		case repo == "":
			repo = arg
		default:
			out = append(out, arg)
		}
	}
	return out, repo
}

func runRepoMigrate(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("repo migrate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	pathValue := fs.String("path", ".", "Local checkout path containing .gira/config.yaml")
	configRoot := fs.String("config-root", "", "Override global config root")
	overwrite := fs.Bool("overwrite", false, "Replace conflicting existing registry entry")
	dryRun := fs.Bool("dry-run", false, "Preview without writing files")
	apply := fs.Bool("apply", false, "Write the registry entry")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, repoHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, repoHelp)
		return 0
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, repoHelp)
		return 2
	}
	var repo gira.RepoRef
	if strings.TrimSpace(*repoValue) != "" {
		parsed, err := gira.ParseRepoRef(*repoValue)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n\n", err)
			fmt.Fprint(stderr, repoHelp)
			return 2
		}
		repo = parsed
	}
	report, err := newRepoMigrateReport(gira.RepoMigrateInput{Repo: repo, Path: *pathValue, ConfigRoot: *configRoot, Overwrite: *overwrite, DryRun: *dryRun, Apply: *apply})
	if err != nil {
		if *jsonOutput {
			gira.EnsureRepoMigrateReportSchema(&report)
			if *dryRun && strings.TrimSpace(report.Command) != "" {
				report.Approval = gira.RepoMigrateApprovalEvidence(report)
			}
			out, _ := json.MarshalIndent(report, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
		}
		fmt.Fprintf(stderr, "repo migrate failed: %v\n", err)
		return 1
	}
	if *jsonOutput {
		gira.EnsureRepoMigrateReportSchema(&report)
		if *dryRun {
			report.Approval = gira.RepoMigrateApprovalEvidence(report)
		}
		out, _ := json.MarshalIndent(report, "", "  ")
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatRepoMigrateReport(report))
	return 0
}

func runCachePrune(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("cache prune", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", "", "Cache root")
	dryRun := fs.Bool("dry-run", false, "Preview stale version directories")
	applyMode := fs.Bool("apply", false, "Delete stale version directories")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, cacheHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, cacheHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, cacheHelp)
		return 2
	}
	if *dryRun == *applyMode {
		fmt.Fprint(stderr, "exactly one of --dry-run or --apply is required\n\n")
		fmt.Fprint(stderr, cacheHelp)
		return 2
	}

	report, err := newCachePruneReport(gira.CachePruneOptions{Root: *root, DryRun: *dryRun, Apply: *applyMode})
	if err != nil {
		fmt.Fprintf(stderr, "cache prune failed: %v\n", err)
		return 1
	}
	if *jsonOutput {
		gira.EnsureCachePruneReportSchema(&report)
		if report.DryRun {
			report.Approval = gira.CachePruneApprovalEvidence(report)
		}
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode cache prune JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatCachePruneReport(report))
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
	case "init":
		return runJiraInit(args[1:], stdout, stderr)
	case "doctor":
		return runJiraDoctor(args[1:], stdout, stderr)
	case "mirror":
		return runJiraMirror(args[1:], stdout, stderr)
	case "transition":
		return runJiraTransition(args[1:], stdout, stderr)
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

func runJiraInit(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("jira init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	apiBase := fs.String("api-base", "", "Jira API base URL")
	project := fs.String("project", "", "Jira project key")
	configRoot := fs.String("config-root", "", "Override Gira global config root")
	overwrite := fs.Bool("overwrite", false, "Replace existing providers.jira config")
	dryRun := fs.Bool("dry-run", false, "Preview without mutation")
	apply := fs.Bool("apply", false, "Apply reviewed provider config")
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
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newJiraProviderInitReport(gira.JiraProviderInitInput{
		Repo:       repo,
		APIBase:    *apiBase,
		Project:    *project,
		ConfigRoot: *configRoot,
		Overwrite:  *overwrite,
		DryRun:     *dryRun,
		Apply:      *apply,
	})
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode jira init JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatJiraProviderInitReport(report))
	return 0
}

func runJiraDoctor(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("jira doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	apiBase := fs.String("api-base", "", "Jira API base URL")
	project := fs.String("project", "", "Jira project key")
	sampleKey := fs.String("sample-key", "", "Representative Jira issue key")
	configRoot := fs.String("config-root", "", "Override Gira global config root")
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
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newJiraDoctorReport(gira.JiraDoctorInput{
		Repo:       repo,
		APIBase:    *apiBase,
		Project:    *project,
		SampleKey:  *sampleKey,
		ConfigRoot: *configRoot,
	})
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode jira doctor JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatJiraDoctorReport(report))
	return 0
}

func runJiraMirror(args []string, stdout io.Writer, stderr io.Writer) int {
	args, jiraKey, keyOK := extractJiraMirrorKeyPositional(args, stderr)
	if !keyOK {
		_, _ = io.WriteString(stderr, jiraHelp)
		return 2
	}
	fs := flag.NewFlagSet("jira mirror", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	apiBase := fs.String("api-base", "", "Jira API base URL")
	configRoot := fs.String("config-root", "", "Override Gira global config root")
	dryRun := fs.Bool("dry-run", false, "Preview without mutation")
	apply := fs.Bool("apply", false, "Apply GitHub mirror issue create")
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
	if jiraKey == "" {
		fmt.Fprint(stderr, "jira mirror requires exactly one Jira key\n\n")
		_, _ = io.WriteString(stderr, jiraHelp)
		return 2
	}
	if *repoValue == "" {
		fmt.Fprint(stderr, "--repo is required\n\n")
		_, _ = io.WriteString(stderr, jiraHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newJiraMirrorReport(gira.JiraMirrorInput{
		Repo:       repo,
		Key:        jiraKey,
		APIBase:    *apiBase,
		ConfigRoot: *configRoot,
		DryRun:     *dryRun,
		Apply:      *apply,
	})
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode jira mirror JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatJiraMirrorReport(report))
	return 0
}

func extractJiraMirrorKeyPositional(args []string, stderr io.Writer) ([]string, string, bool) {
	cleaned := make([]string, 0, len(args))
	key := ""
	seen := false
	valueFlags := map[string]struct{}{"--repo": {}, "--api-base": {}, "--config-root": {}, "--to": {}}
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
		if seen {
			fmt.Fprint(stderr, "only one Jira key can be provided\n\n")
			return nil, "", false
		}
		key = arg
		seen = true
		cleaned = cleaned[:len(cleaned)-1]
	}
	return cleaned, key, true
}

func runJiraTransition(args []string, stdout io.Writer, stderr io.Writer) int {
	args, jiraKey, keyOK := extractJiraMirrorKeyPositional(args, stderr)
	if !keyOK {
		_, _ = io.WriteString(stderr, jiraHelp)
		return 2
	}
	fs := flag.NewFlagSet("jira transition", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	target := fs.String("to", "", "Target Gira status")
	apiBase := fs.String("api-base", "", "Jira API base URL")
	configRoot := fs.String("config-root", "", "Override Gira global config root")
	dryRun := fs.Bool("dry-run", false, "Preview transition plan")
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
	if jiraKey == "" {
		fmt.Fprint(stderr, "jira transition requires exactly one Jira key\n\n")
		_, _ = io.WriteString(stderr, jiraHelp)
		return 2
	}
	if *repoValue == "" {
		fmt.Fprint(stderr, "--repo is required\n\n")
		_, _ = io.WriteString(stderr, jiraHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newJiraTransitionPlanReport(gira.JiraTransitionPlanInput{
		Repo:       repo,
		Key:        jiraKey,
		Target:     *target,
		APIBase:    *apiBase,
		ConfigRoot: *configRoot,
		DryRun:     *dryRun,
	})
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	gira.EnsureJiraTransitionPlanReportSchema(&report)
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode jira transition JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatJiraTransitionPlan(report))
	return 0
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
	case "list":
		return runTicketList(args[1:], stdout, stderr)
	case "view", "show":
		return runTicketView(args[1:], stdout, stderr)
	case "prompt":
		return runTicketPrompt(args[1:], stdout, stderr)
	case "handoff":
		return runTicketHandoff(args[1:], stdout, stderr)
	case "review":
		return runTicketReview(args[1:], stdout, stderr)
	case "self-review":
		return runTicketSelfReview(args[1:], stdout, stderr)
	case "start":
		return runTicketStart(args[1:], stdout, stderr)
	case "pr":
		return runTicketPR(args[1:], stdout, stderr)
	case "note":
		return runTicketNote(args[1:], stdout, stderr)
	case "supersede":
		return runTicketSupersede(args[1:], stdout, stderr)
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

func runRun(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		_, _ = io.WriteString(stdout, runHelp)
		return 0
	}
	switch args[0] {
	case "start":
		return runRunStart(args[1:], stdout, stderr)
	case "status":
		return runRunStatus(args[1:], stdout, stderr)
	case "collect":
		return runRunCollect(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown run command: %s\n\n", args[0])
		_, _ = io.WriteString(stderr, runHelp)
		return 2
	}
}

func runRunStart(args []string, stdout io.Writer, stderr io.Writer) int {
	args, positionalTicket, positionalOK := extractRunStartTicketPositional(args, stderr)
	if !positionalOK {
		_, _ = io.WriteString(stderr, runHelp)
		return 2
	}
	fs := flag.NewFlagSet("run start", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	ticket := fs.Int("ticket", 0, "Ticket number")
	issue := fs.Int("issue", 0, "Compatibility alias for --ticket")
	role := fs.String("role", gira.AgentPromptRoleImplementer, "Handoff role: planner|implementer|reviewer")
	profile := fs.String("profile", "default", "Handoff profile: default|python")
	contextNote := fs.String("context", "", "Small operator note to include in the private run prompt")
	contextFile := fs.String("context-file", "", "Read a small operator note from file, or '-' for stdin")
	name := fs.String("name", "", "Optional human-readable local run name")
	runID := fs.String("id", "", "Optional custom local run id")
	stateRoot := fs.String("state-root", "", "Override private local Gira state root")
	workDir := fs.String("workdir", "", "Codex working directory")
	execRun := fs.Bool("exec", false, "Start Codex in the background after writing the manifest")
	dryRun := fs.Bool("dry-run", false, "Preview without writing local run files")
	apply := fs.Bool("apply", false, "Write private local run files")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, runHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, runHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		_, _ = io.WriteString(stderr, runHelp)
		return 2
	}
	if positionalTicket > 0 {
		if *ticket > 0 && *ticket != positionalTicket {
			fmt.Fprint(stderr, "--ticket and positional ticket must refer to the same number\n\n")
			_, _ = io.WriteString(stderr, runHelp)
			return 2
		}
		*ticket = positionalTicket
	}
	if *issue > 0 {
		if *ticket > 0 && *ticket != *issue {
			fmt.Fprint(stderr, "--ticket and --issue must refer to the same number\n\n")
			_, _ = io.WriteString(stderr, runHelp)
			return 2
		}
		*ticket = *issue
	}
	if *ticket <= 0 {
		fmt.Fprint(stderr, "--ticket or positional ticket is required\n\n")
		_, _ = io.WriteString(stderr, runHelp)
		return 2
	}
	if *dryRun == *apply {
		fmt.Fprint(stderr, "exactly one of --dry-run or --apply is required\n\n")
		_, _ = io.WriteString(stderr, runHelp)
		return 2
	}
	if *execRun && !*apply {
		fmt.Fprint(stderr, "--exec requires --apply\n\n")
		_, _ = io.WriteString(stderr, runHelp)
		return 2
	}
	if strings.TrimSpace(*workDir) == "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "resolve working directory: %v\n", err)
			return 1
		}
		*workDir = wd
	}
	contextNotes, err := readRunStartContext(*contextNote, *contextFile, os.Stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	repo, err := gira.ResolveRepoContext(*repoValue, repoContextRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	handoff, err := newTicketHandoffReport(gira.TicketHandoffInput{
		Repo:         repo,
		Ticket:       *ticket,
		Role:         *role,
		Profile:      *profile,
		ContextNotes: contextNotes,
	})
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	prompt, err := json.MarshalIndent(handoff, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "encode run prompt JSON: %v\n", err)
		return 2
	}
	prompt = append(prompt, '\n')
	promptSummary := gira.SummarizeTicketHandoffPrompt(handoff)
	report, err := gira.BuildRunStartReport(gira.RunStartInput{
		Repo:          repo,
		Ticket:        *ticket,
		Role:          *role,
		Profile:       *profile,
		Name:          *name,
		RunID:         *runID,
		StateRoot:     *stateRoot,
		WorkDir:       *workDir,
		Prompt:        prompt,
		DryRun:        *dryRun,
		Apply:         *apply,
		SafeSummary:   fmt.Sprintf("local Codex run for %s#%d role=%s", repo.FullName(), *ticket, strings.TrimSpace(*role)),
		PromptSummary: &promptSummary,
	})
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if *execRun {
		pid, err := startRunProcess(report.Manifest)
		if err != nil {
			fmt.Fprintf(stderr, "start run process: %v\n", err)
			return 1
		}
		report.Exec = true
		report.Manifest.Status = "running"
		report.Manifest.PID = pid
		if err := gira.WriteRunManifest(report.Manifest); err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
	}
	if *jsonOutput {
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode run start JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatRunStart(report))
	return 0
}

func runRunStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	report, code := selectRunFromFlags("run status", args, stdout, stderr)
	if code != 0 {
		return code
	}
	if report.handled {
		return 0
	}
	if report.jsonOutput {
		out, err := json.MarshalIndent(report.status, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode run status JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatRunStatus(report.status))
	return 0
}

func runRunCollect(args []string, stdout io.Writer, stderr io.Writer) int {
	report, code := selectRunFromFlags("run collect", args, stdout, stderr)
	if code != 0 {
		return code
	}
	if report.handled {
		return 0
	}
	if report.status.Manifest == nil {
		fmt.Fprint(stderr, "no matching local run result\n")
		return 1
	}
	result, err := gira.ReadRunResult(*report.status.Manifest)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if report.jsonOutput {
		out, err := json.MarshalIndent(struct {
			SchemaVersion string           `json:"schema_version"`
			Manifest      gira.RunManifest `json:"manifest"`
			Result        string           `json:"result"`
		}{
			SchemaVersion: "gira-run-collect-report/v1",
			Manifest:      *report.status.Manifest,
			Result:        result,
		}, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode run collect JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, result)
	if !strings.HasSuffix(result, "\n") {
		fmt.Fprintln(stdout)
	}
	return 0
}

type runStatusCLIReport struct {
	status     gira.RunStatusReport
	jsonOutput bool
	handled    bool
}

func selectRunFromFlags(commandName string, args []string, stdout io.Writer, stderr io.Writer) (runStatusCLIReport, int) {
	fs := flag.NewFlagSet(commandName, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	ticket := fs.Int("ticket", 0, "Ticket number")
	runID := fs.String("id", "", "Local run id")
	latest := fs.Bool("latest", false, "Select the newest matching local run")
	stateRoot := fs.String("state-root", "", "Override private local Gira state root")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, runHelp)
		return runStatusCLIReport{}, 2
	}
	if *help {
		_, _ = io.WriteString(stdout, runHelp)
		return runStatusCLIReport{handled: true}, 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		_, _ = io.WriteString(stderr, runHelp)
		return runStatusCLIReport{}, 2
	}
	var repo gira.RepoRef
	if strings.TrimSpace(*repoValue) != "" {
		resolved, err := gira.ResolveRepoContext(*repoValue, repoContextRunner)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return runStatusCLIReport{}, 2
		}
		repo = resolved
	}
	status, err := gira.BuildRunStatusReport(gira.RunSelectInput{
		Repo:      repo,
		Ticket:    *ticket,
		RunID:     *runID,
		Latest:    *latest,
		StateRoot: *stateRoot,
	})
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return runStatusCLIReport{}, 1
	}
	return runStatusCLIReport{status: status, jsonOutput: *jsonOutput}, 0
}

func startRunProcess(manifest gira.RunManifest) (int, error) {
	if len(manifest.Command) == 0 {
		return 0, fmt.Errorf("run command is missing")
	}
	prompt, err := os.Open(manifest.PromptPath)
	if err != nil {
		return 0, fmt.Errorf("open run prompt: %w", err)
	}
	defer prompt.Close()
	events, err := os.OpenFile(manifest.EventLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, fmt.Errorf("open run event log: %w", err)
	}
	defer events.Close()
	stderrLog, err := os.OpenFile(manifest.StderrLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, fmt.Errorf("open run stderr log: %w", err)
	}
	defer stderrLog.Close()
	cmd := osexec.Command(manifest.Command[0], manifest.Command[1:]...)
	cmd.Stdin = prompt
	cmd.Stdout = events
	cmd.Stderr = stderrLog
	if strings.TrimSpace(manifest.WorkDir) != "" {
		cmd.Dir = filepath.Clean(manifest.WorkDir)
	}
	configureRunProcess(cmd)
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	if err := cmd.Process.Release(); err != nil {
		return 0, fmt.Errorf("release run process: %w", err)
	}
	return pid, nil
}

func runFeature(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		_, _ = io.WriteString(stdout, featureHelp)
		return 0
	}
	switch args[0] {
	case "list":
		return runFeatureList(args[1:], stdout, stderr)
	case "check":
		return runFeatureCheck(args[1:], stdout, stderr)
	case "for":
		return runFeatureFor(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown feature command: %s\n\n", args[0])
		_, _ = io.WriteString(stderr, featureHelp)
		return 2
	}
}

func runFeatureList(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("feature list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	limit := fs.Int("limit", 1000, "Max issues to inspect")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, featureHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, featureHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		_, _ = io.WriteString(stderr, featureHelp)
		return 2
	}
	repo, err := gira.ResolveRepoContext(*repoValue, repoContextRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newFeatureMapListReport(gira.FeatureMapOptions{Repo: repo, Limit: *limit})
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
			fmt.Fprintf(stderr, "encode feature list JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatFeatureMapList(report))
	return 0
}

func runFeatureCheck(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("feature check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	limit := fs.Int("limit", 1000, "Max issues to inspect")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, featureHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, featureHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		_, _ = io.WriteString(stderr, featureHelp)
		return 2
	}
	repo, err := gira.ResolveRepoContext(*repoValue, repoContextRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newFeatureMapCheckReport(gira.FeatureMapOptions{Repo: repo, Limit: *limit})
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
			fmt.Fprintf(stderr, "encode feature check JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatFeatureMapCheck(report))
	return 0
}

func runFeatureFor(args []string, stdout io.Writer, stderr io.Writer) int {
	args, positionalIssue, ok := extractFeatureForPositional(args, stderr)
	if !ok {
		_, _ = io.WriteString(stderr, featureHelp)
		return 2
	}
	fs := flag.NewFlagSet("feature for", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	issue := fs.Int("issue", 0, "Work issue number")
	limit := fs.Int("limit", 1000, "Max issues to inspect")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, featureHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, featureHelp)
		return 0
	}
	if positionalIssue > 0 {
		if *issue > 0 && *issue != positionalIssue {
			fmt.Fprint(stderr, "--issue and positional issue must refer to the same number\n\n")
			_, _ = io.WriteString(stderr, featureHelp)
			return 2
		}
		*issue = positionalIssue
	}
	if *issue <= 0 {
		fmt.Fprint(stderr, "--issue or positional issue is required\n\n")
		_, _ = io.WriteString(stderr, featureHelp)
		return 2
	}
	repo, err := gira.ResolveRepoContext(*repoValue, repoContextRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newFeatureMapForReport(gira.FeatureForOptions{Repo: repo, Issue: *issue, Limit: *limit})
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
			fmt.Fprintf(stderr, "encode feature for JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatFeatureMapFor(report))
	return 0
}

func runGoal(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		_, _ = io.WriteString(stdout, goalHelp)
		return 0
	}
	switch args[0] {
	case "plan":
		return runGoalPlan(args[1:], stdout, stderr)
	case "report":
		return runGoalReport(args[1:], "report", stdout, stderr)
	case "dossier":
		return runGoalReport(args[1:], "dossier", stdout, stderr)
	case "status":
		return runGoalStatus(args[1:], stdout, stderr)
	case "next":
		return runGoalNext(args[1:], stdout, stderr)
	case "finish":
		return runGoalFinish(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown goal command: %s\n\n", args[0])
		_, _ = io.WriteString(stderr, goalHelp)
		return 2
	}
}

func runGoalPlan(args []string, stdout io.Writer, stderr io.Writer) int {
	args, positionalGoal, positionalOK := extractNumericPositional(args, "goal", stderr)
	if !positionalOK {
		_, _ = io.WriteString(stderr, goalHelp)
		return 2
	}
	fs := flag.NewFlagSet("goal plan", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	goal := fs.Int("goal", 0, "Goal issue number")
	dryRun := fs.Bool("dry-run", false, "Preview without mutation")
	apply := fs.Bool("apply", false, "Create proposed child tickets")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, goalHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, goalHelp)
		return 0
	}
	if positionalGoal > 0 {
		if *goal > 0 && *goal != positionalGoal {
			fmt.Fprint(stderr, "--goal and positional goal must refer to the same number\n\n")
			_, _ = io.WriteString(stderr, goalHelp)
			return 2
		}
		*goal = positionalGoal
	}
	if *goal <= 0 {
		fmt.Fprint(stderr, "--goal or positional goal is required\n\n")
		_, _ = io.WriteString(stderr, goalHelp)
		return 2
	}
	if *dryRun == *apply {
		fmt.Fprint(stderr, "exactly one of --dry-run or --apply is required for goal plan\n\n")
		_, _ = io.WriteString(stderr, goalHelp)
		return 2
	}
	repo, err := gira.ResolveRepoContext(*repoValue, repoContextRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newGoalPlanReport(gira.GoalPlanInput{Repo: repo, Goal: *goal, DryRun: *dryRun, Apply: *apply})
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
			fmt.Fprintf(stderr, "encode goal plan JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatGoalPlan(report))
	return 0
}

func runGoalFinish(args []string, stdout io.Writer, stderr io.Writer) int {
	args, positionalGoal, positionalOK := extractNumericPositional(args, "goal", stderr)
	if !positionalOK {
		_, _ = io.WriteString(stderr, goalHelp)
		return 2
	}
	fs := flag.NewFlagSet("goal finish", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	goal := fs.Int("goal", 0, "Goal issue number")
	dryRun := fs.Bool("dry-run", false, "Preview without mutation")
	apply := fs.Bool("apply", false, "Apply supported human-review handoff")
	terminal := fs.String("terminal", "", "Terminal recommendation: done|human_review|blocked|superseded|abandoned")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, goalHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, goalHelp)
		return 0
	}
	if positionalGoal > 0 {
		if *goal > 0 && *goal != positionalGoal {
			fmt.Fprint(stderr, "--goal and positional goal must refer to the same number\n\n")
			_, _ = io.WriteString(stderr, goalHelp)
			return 2
		}
		*goal = positionalGoal
	}
	if *goal <= 0 {
		fmt.Fprint(stderr, "--goal or positional goal is required\n\n")
		_, _ = io.WriteString(stderr, goalHelp)
		return 2
	}
	if *dryRun == *apply {
		fmt.Fprint(stderr, "exactly one of --dry-run or --apply is required\n\n")
		_, _ = io.WriteString(stderr, goalHelp)
		return 2
	}
	repo, err := gira.ResolveRepoContext(*repoValue, repoContextRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newGoalFinishReport(gira.GoalFinishInput{Repo: repo, Goal: *goal, DryRun: *dryRun, Apply: *apply, Terminal: *terminal})
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
			fmt.Fprintf(stderr, "encode goal finish JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatGoalFinish(report))
	return 0
}

func runGoalStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	args, positionalGoal, positionalOK := extractNumericPositional(args, "goal", stderr)
	if !positionalOK {
		_, _ = io.WriteString(stderr, goalHelp)
		return 2
	}
	fs := flag.NewFlagSet("goal status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	goal := fs.Int("goal", 0, "Goal issue number")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, goalHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, goalHelp)
		return 0
	}
	if positionalGoal > 0 {
		if *goal > 0 && *goal != positionalGoal {
			fmt.Fprint(stderr, "--goal and positional goal must refer to the same number\n\n")
			_, _ = io.WriteString(stderr, goalHelp)
			return 2
		}
		*goal = positionalGoal
	}
	if *goal <= 0 {
		fmt.Fprint(stderr, "--goal or positional goal is required\n\n")
		_, _ = io.WriteString(stderr, goalHelp)
		return 2
	}
	repo, err := gira.ResolveRepoContext(*repoValue, repoContextRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newGoalStatusReport(gira.GoalStatusInput{Repo: repo, Goal: *goal})
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
			fmt.Fprintf(stderr, "encode goal status JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatGoalStatus(report))
	return 0
}

func runGoalReport(args []string, commandName string, stdout io.Writer, stderr io.Writer) int {
	args, positionalGoal, positionalOK := extractNumericPositional(args, "goal", stderr)
	if !positionalOK {
		_, _ = io.WriteString(stderr, goalHelp)
		return 2
	}
	fs := flag.NewFlagSet("goal "+commandName, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	goal := fs.Int("goal", 0, "Goal issue number")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	htmlOutput := fs.Bool("html", false, "Write a static local HTML report")
	outputPath := fs.String("output", "", "Output path for --html")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, goalHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, goalHelp)
		return 0
	}
	if positionalGoal > 0 {
		if *goal > 0 && *goal != positionalGoal {
			fmt.Fprint(stderr, "--goal and positional goal must refer to the same number\n\n")
			_, _ = io.WriteString(stderr, goalHelp)
			return 2
		}
		*goal = positionalGoal
	}
	if *goal <= 0 {
		fmt.Fprint(stderr, "--goal or positional goal is required\n\n")
		_, _ = io.WriteString(stderr, goalHelp)
		return 2
	}
	if *jsonOutput && *htmlOutput {
		fmt.Fprint(stderr, "choose exactly one output format: --json or --html\n\n")
		_, _ = io.WriteString(stderr, goalHelp)
		return 2
	}
	if *htmlOutput && strings.TrimSpace(*outputPath) == "" {
		fmt.Fprint(stderr, "--output is required when using --html\n\n")
		_, _ = io.WriteString(stderr, goalHelp)
		return 2
	}
	if !*htmlOutput && strings.TrimSpace(*outputPath) != "" {
		fmt.Fprint(stderr, "--output requires --html\n\n")
		_, _ = io.WriteString(stderr, goalHelp)
		return 2
	}
	repo, err := gira.ResolveRepoContext(*repoValue, repoContextRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newGoalDossierReport(gira.GoalDossierInput{Repo: repo, Goal: *goal})
	if err != nil {
		if *jsonOutput {
			out, _ := json.MarshalIndent(report, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
		}
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if commandName == "report" {
		report.Command = "goal report"
	}
	if *jsonOutput {
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode goal report JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	if *htmlOutput {
		if err := gira.WriteGoalReportHTML(*outputPath, report); err != nil {
			fmt.Fprintf(stderr, "write goal report HTML: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "goal report html: %s\n", filepath.Clean(*outputPath))
		fmt.Fprintf(stdout, "next step: open %s\n", filepath.Clean(*outputPath))
		return 0
	}
	fmt.Fprint(stdout, gira.FormatGoalReport(report))
	return 0
}

func runGoalNext(args []string, stdout io.Writer, stderr io.Writer) int {
	args, positionalGoal, positionalOK := extractNumericPositional(args, "goal", stderr)
	if !positionalOK {
		_, _ = io.WriteString(stderr, goalHelp)
		return 2
	}
	fs := flag.NewFlagSet("goal next", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	goal := fs.Int("goal", 0, "Goal issue number")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, goalHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, goalHelp)
		return 0
	}
	if positionalGoal > 0 {
		if *goal > 0 && *goal != positionalGoal {
			fmt.Fprint(stderr, "--goal and positional goal must refer to the same number\n\n")
			_, _ = io.WriteString(stderr, goalHelp)
			return 2
		}
		*goal = positionalGoal
	}
	if *goal <= 0 {
		fmt.Fprint(stderr, "--goal or positional goal is required\n\n")
		_, _ = io.WriteString(stderr, goalHelp)
		return 2
	}
	repo, err := gira.ResolveRepoContext(*repoValue, repoContextRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newGoalNextReport(gira.GoalNextInput{Repo: repo, Goal: *goal})
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
			fmt.Fprintf(stderr, "encode goal next JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatGoalNext(report))
	return 0
}

func runTicketList(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("ticket list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	state := fs.String("state", "open", "Ticket state: open|closed|all")
	assignee := fs.String("assignee", "", "Assignee login")
	milestone := fs.String("milestone", "", "Milestone title")
	limit := fs.Int("limit", 30, "Maximum tickets to list")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	var labels repeatedStringFlag
	fs.Var(&labels, "label", "Label filter. Repeatable or comma-separated")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, ticketHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	repo, err := gira.ResolveRepoContext(*repoValue, repoContextRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newTicketListReport(gira.TicketListOptions{
		Repo:      repo,
		State:     *state,
		Labels:    labels,
		Assignee:  *assignee,
		Milestone: *milestone,
		Limit:     *limit,
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
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode ticket list JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatTicketList(report))
	return 0
}

func runTicketView(args []string, stdout io.Writer, stderr io.Writer) int {
	args, positionalIdentifier, positionalOK := extractTicketIdentifierPositional(args, stderr)
	if !positionalOK {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	fs := flag.NewFlagSet("ticket view", flag.ContinueOnError)
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
	ticketNumber, jiraKey, resolved, resolveErr := resolveTicketIdentifierContext(repo, *ticket, *issue, positionalIdentifier, true, stderr)
	if resolveErr != nil {
		fmt.Fprintf(stderr, "%v\n", resolveErr)
		return 1
	}
	if !resolved {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	result, err := newTicketViewReport(repo, ticketNumber)
	if jiraKey != "" {
		result.JiraKey = jiraKey
		result.MirrorIssue = ticketNumber
	}
	result.Status.NextStep = shortenTicketNextStep(result.Status.NextStep, result.Repo, result.Ticket)
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
	fmt.Fprint(stdout, gira.FormatTicketView(result))
	return 0
}

func runTicketPrompt(args []string, stdout io.Writer, stderr io.Writer) int {
	args, positionalRole, roleOK := extractTicketPromptRolePositional(args, stderr)
	if !roleOK {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	args, positionalIdentifier, positionalOK := extractTicketIdentifierPositional(args, stderr)
	if !positionalOK {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	fs := flag.NewFlagSet("ticket prompt", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	ticket := fs.Int("ticket", 0, "Ticket number")
	issue := fs.Int("issue", 0, "Compatibility alias for --ticket")
	role := fs.String("role", "", "Prompt role: planner|implementer|reviewer")
	profile := fs.String("profile", "default", "Prompt profile: default|python")
	prNumber := fs.Int("pr", 0, "Optional PR number for reviewer prompt context")
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
	if positionalRole != "" {
		if strings.TrimSpace(*role) != "" && !strings.EqualFold(*role, positionalRole) {
			fmt.Fprint(stderr, "positional role and --role must match when both are provided\n\n")
			_, _ = io.WriteString(stderr, ticketHelp)
			return 2
		}
		*role = positionalRole
	}
	repo, err := gira.ResolveRepoContext(*repoValue, repoContextRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	ticketNumber, _, resolved, resolveErr := resolveTicketIdentifierContext(repo, *ticket, *issue, positionalIdentifier, true, stderr)
	if resolveErr != nil {
		fmt.Fprintf(stderr, "%v\n", resolveErr)
		return 1
	}
	if !resolved {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	result, err := newTicketPromptReport(gira.AgentPromptInput{
		Repo:     repo,
		Ticket:   ticketNumber,
		Role:     *role,
		Profile:  *profile,
		PRNumber: *prNumber,
	})
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
			fmt.Fprintf(stderr, "encode ticket prompt JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatAgentPrompt(result))
	return 0
}

func runTicketHandoff(args []string, stdout io.Writer, stderr io.Writer) int {
	args, positionalRole, roleOK := extractTicketPromptRolePositional(args, stderr)
	if !roleOK {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	args, positionalIdentifier, positionalOK := extractTicketIdentifierPositional(args, stderr)
	if !positionalOK {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	fs := flag.NewFlagSet("ticket handoff", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	ticket := fs.Int("ticket", 0, "Ticket number")
	issue := fs.Int("issue", 0, "Compatibility alias for --ticket")
	role := fs.String("role", "", "Handoff role: planner|implementer|reviewer. Default: implementer")
	profile := fs.String("profile", "default", "Handoff profile: default|python")
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
	if positionalRole != "" {
		if strings.TrimSpace(*role) != "" && !strings.EqualFold(*role, positionalRole) {
			fmt.Fprint(stderr, "positional role and --role must match when both are provided\n\n")
			_, _ = io.WriteString(stderr, ticketHelp)
			return 2
		}
		*role = positionalRole
	}
	if strings.TrimSpace(*role) == "" {
		*role = gira.AgentPromptRoleImplementer
	}
	repo, err := gira.ResolveRepoContext(*repoValue, repoContextRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	ticketNumber, _, resolved, resolveErr := resolveTicketIdentifierContext(repo, *ticket, *issue, positionalIdentifier, true, stderr)
	if resolveErr != nil {
		fmt.Fprintf(stderr, "%v\n", resolveErr)
		return 1
	}
	if !resolved {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	result, err := newTicketHandoffReport(gira.TicketHandoffInput{
		Repo:    repo,
		Ticket:  ticketNumber,
		Role:    *role,
		Profile: *profile,
	})
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
			fmt.Fprintf(stderr, "encode ticket handoff JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatTicketHandoff(result))
	return 0
}

func runTicketReview(args []string, stdout io.Writer, stderr io.Writer) int {
	args, positionalIdentifier, positionalOK := extractTicketIdentifierPositional(args, stderr)
	if !positionalOK {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	fs := flag.NewFlagSet("ticket review", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	ticket := fs.Int("ticket", 0, "Ticket number")
	issue := fs.Int("issue", 0, "Compatibility alias for --ticket")
	profile := fs.String("profile", "default", "Prompt profile: default|python")
	prNumber := fs.Int("pr", 0, "Optional PR number for reviewer packet context")
	diffSummary := fs.Bool("diff-summary", false, "Include compact PR diff summary")
	includeDiff := fs.Bool("include-diff", false, "Include full PR diff")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	htmlOutput := fs.Bool("html", false, "Write a static local HTML report")
	outputPath := fs.String("output", "", "Output path for --html")
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
	if *jsonOutput && *htmlOutput {
		fmt.Fprint(stderr, "choose exactly one output format: --json or --html\n\n")
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	if *htmlOutput && strings.TrimSpace(*outputPath) == "" {
		fmt.Fprint(stderr, "--output is required when using --html\n\n")
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	if !*htmlOutput && strings.TrimSpace(*outputPath) != "" {
		fmt.Fprint(stderr, "--output requires --html\n\n")
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	repo, err := gira.ResolveRepoContext(*repoValue, repoContextRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	ticketNumber, _, resolved, resolveErr := resolveTicketIdentifierContext(repo, *ticket, *issue, positionalIdentifier, true, stderr)
	if resolveErr != nil {
		fmt.Fprintf(stderr, "%v\n", resolveErr)
		return 1
	}
	if !resolved {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	result, err := newTicketPromptReport(gira.AgentPromptInput{
		Repo:               repo,
		Ticket:             ticketNumber,
		Role:               gira.AgentPromptRoleReviewer,
		Profile:            *profile,
		PRNumber:           *prNumber,
		IncludeDiffSummary: *diffSummary || *includeDiff,
		IncludeDiff:        *includeDiff,
	})
	result.Command = "ticket review"
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
			fmt.Fprintf(stderr, "encode ticket review JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	if *htmlOutput {
		if err := gira.WriteTicketReviewHTML(*outputPath, result); err != nil {
			fmt.Fprintf(stderr, "write ticket review HTML: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "ticket review html: %s\n", filepath.Clean(*outputPath))
		fmt.Fprintf(stdout, "next step: open %s\n", filepath.Clean(*outputPath))
		return 0
	}
	fmt.Fprint(stdout, gira.FormatAgentPrompt(result))
	return 0
}

func runTicketSelfReview(args []string, stdout io.Writer, stderr io.Writer) int {
	args, positionalIdentifier, positionalOK := extractTicketIdentifierPositional(args, stderr)
	if !positionalOK {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	fs := flag.NewFlagSet("ticket self-review", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	ticket := fs.Int("ticket", 0, "Ticket number")
	issue := fs.Int("issue", 0, "Compatibility alias for --ticket")
	prNumber := fs.Int("pr", 0, "Optional PR number for reviewer packet context")
	diffSummary := fs.Bool("diff-summary", true, "Include compact PR diff summary")
	dryRun := fs.Bool("dry-run", false, "Preview self-review PR note")
	apply := fs.Bool("apply", false, "Post self-review PR note")
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
	ticketNumber, _, resolved, resolveErr := resolveTicketIdentifierContext(repo, *ticket, *issue, positionalIdentifier, true, stderr)
	if resolveErr != nil {
		fmt.Fprintf(stderr, "%v\n", resolveErr)
		return 1
	}
	if !resolved {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	result, err := newTicketSelfReviewReport(gira.TicketSelfReviewInput{
		Repo:        repo,
		Ticket:      ticketNumber,
		PRNumber:    *prNumber,
		DiffSummary: *diffSummary,
		DryRun:      *dryRun,
		Apply:       *apply,
	})
	if result.Note != nil {
		result.Note.Status.NextStep = shortenTicketNextStep(result.Note.Status.NextStep, result.Repo, result.Ticket)
		result.Note.NextStep = shortenTicketNextStep(result.Note.NextStep, result.Repo, result.Ticket)
	}
	result.NextStep = shortenTicketNextStep(result.NextStep, result.Repo, result.Ticket)
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
			fmt.Fprintf(stderr, "encode ticket self-review JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatTicketSelfReview(result))
	return 0
}

func runEpic(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		_, _ = io.WriteString(stdout, epicHelp)
		return 0
	}
	switch args[0] {
	case "list":
		return runEpicList(args[1:], stdout, stderr)
	case "status":
		return runEpicStatus(args[1:], stdout, stderr)
	case "finish", "close":
		return runEpicFinish(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown epic command: %s\n\n", args[0])
		_, _ = io.WriteString(stderr, epicHelp)
		return 2
	}
}

func runEpicList(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("epic list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	state := fs.String("state", "open", "Epic state: open|closed|all")
	assignee := fs.String("assignee", "", "Assignee login")
	milestone := fs.String("milestone", "", "Milestone title")
	limit := fs.Int("limit", 30, "Maximum epics to list")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	var labels repeatedStringFlag
	fs.Var(&labels, "label", "Additional label filter. Repeatable or comma-separated")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, epicHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, epicHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		_, _ = io.WriteString(stderr, epicHelp)
		return 2
	}
	repo, err := gira.ResolveRepoContext(*repoValue, repoContextRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	labels = append(repeatedStringFlag{"type:epic"}, labels...)
	report, err := newTicketListReport(gira.TicketListOptions{
		Repo:      repo,
		State:     *state,
		Labels:    labels,
		Assignee:  *assignee,
		Milestone: *milestone,
		Limit:     *limit,
	})
	report.Command = "epic list"
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
			fmt.Fprintf(stderr, "encode epic list JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatTicketList(report))
	return 0
}

func runEpicStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("epic status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	ticket := fs.Int("ticket", 0, "Epic issue number")
	issue := fs.Int("issue", 0, "Compatibility alias for --ticket")
	title := fs.String("title", "", "Epic title substring")
	slug := fs.String("slug", "", "Epic title slug")
	milestone := fs.String("milestone", "", "Epic milestone title")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, epicHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, epicHelp)
		return 0
	}
	positionalTicket, positionalOK := parseOptionalNumericPositional(fs.Args(), "epic", stderr)
	if !positionalOK {
		_, _ = io.WriteString(stderr, epicHelp)
		return 2
	}
	epicNumber, ok := resolveExplicitTicket(*ticket, *issue, positionalTicket, stderr)
	if !ok {
		_, _ = io.WriteString(stderr, epicHelp)
		return 2
	}
	repo, err := gira.ResolveRepoContext(*repoValue, repoContextRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	result, err := newEpicStatusReport(gira.EpicInput{Repo: repo, Ticket: epicNumber, Title: *title, Slug: *slug, Milestone: *milestone})
	result.NextStep = shortenEpicNextStep(result.NextStep, result.Repo, result.Epic.Number)
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
	fmt.Fprint(stdout, gira.FormatEpicReport(result))
	return 0
}

func runEpicFinish(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("epic finish", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	ticket := fs.Int("ticket", 0, "Epic issue number")
	issue := fs.Int("issue", 0, "Compatibility alias for --ticket")
	title := fs.String("title", "", "Epic title substring")
	slug := fs.String("slug", "", "Epic title slug")
	milestone := fs.String("milestone", "", "Epic milestone title")
	dryRun := fs.Bool("dry-run", false, "Preview without mutation")
	apply := fs.Bool("apply", false, "Apply changes")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, epicHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, epicHelp)
		return 0
	}
	if *dryRun == *apply {
		fmt.Fprint(stderr, "exactly one of --dry-run/--apply is required\n\n")
		_, _ = io.WriteString(stderr, epicHelp)
		return 2
	}
	positionalTicket, positionalOK := parseOptionalNumericPositional(fs.Args(), "epic", stderr)
	if !positionalOK {
		_, _ = io.WriteString(stderr, epicHelp)
		return 2
	}
	epicNumber, ok := resolveExplicitTicket(*ticket, *issue, positionalTicket, stderr)
	if !ok {
		_, _ = io.WriteString(stderr, epicHelp)
		return 2
	}
	repo, err := gira.ResolveRepoContext(*repoValue, repoContextRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	result, err := newEpicFinishReport(gira.EpicInput{Repo: repo, Ticket: epicNumber, Title: *title, Slug: *slug, Milestone: *milestone, DryRun: *dryRun, Apply: *apply})
	result.NextStep = shortenEpicNextStep(result.NextStep, result.Repo, result.Epic.Number)
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
	fmt.Fprint(stdout, gira.FormatEpicReport(result))
	return 0
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
	body := fs.String("body", "", "Full issue body")
	ticketType := fs.String("type", "task", "Ticket type: epic|story|task|bug|spike|chore")
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
	resolvedBody, err := readTicketNewBody(*body, *bodyFile, os.Stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
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
		Body:       resolvedBody,
		Type:       *ticketType,
		Priority:   *priority,
		Milestone:  *milestone,
		Labels:     labels,
		Start:      *start,
		DryRun:     *dryRun,
	})
	if err != nil {
		if *jsonOutput {
			gira.EnsureTicketNewReportSchema(&report)
			out, _ := json.MarshalIndent(report, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
		}
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if *jsonOutput {
		gira.EnsureTicketNewReportSchema(&report)
		if *dryRun {
			report.Approval = gira.TicketNewApprovalEvidence(report)
		}
		out, _ := json.MarshalIndent(report, "", "  ")
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatTicketNew(report))
	return 0
}

func runTicketStart(args []string, stdout io.Writer, stderr io.Writer) int {
	args, positionalIdentifier, positionalOK := extractTicketIdentifierPositional(args, stderr)
	if !positionalOK {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	fs := flag.NewFlagSet("ticket start", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	ticket := fs.Int("ticket", 0, "Ticket number")
	issue := fs.Int("issue", 0, "Compatibility alias for --ticket")
	base := fs.String("base", "", "Explicit base branch override for ticket start")
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
	explicitNumber, ok := resolveExplicitTicket(*ticket, *issue, positionalIdentifier.Number, stderr)
	if !ok {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	if explicitNumber > 0 && positionalIdentifier.JiraKey != "" {
		fmt.Fprint(stderr, "--ticket/--issue and Jira key positional cannot be combined\n\n")
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	requiredTicket := explicitNumber
	if positionalIdentifier.JiraKey != "" {
		requiredTicket = 1
	}
	repo, ok := parseTicketRequiredFlags(*repoValue, requiredTicket, *dryRun, *apply, false, stderr)
	if !ok {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	ticketNumber := explicitNumber
	jiraKey := ""
	if positionalIdentifier.JiraKey != "" {
		mirror, err := newJiraMirrorIssueResolver(repo, positionalIdentifier.JiraKey)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		ticketNumber = mirror.Number
		jiraKey = strings.ToUpper(strings.TrimSpace(positionalIdentifier.JiraKey))
	}
	result, err := newWorkStartResultWithOptions(repo, ticketNumber, gira.WorkStartOptions{DryRun: *dryRun, BaseOverride: *base})
	if jiraKey != "" {
		result.JiraKey = jiraKey
		result.MirrorIssue = ticketNumber
	}
	if err != nil {
		if *jsonOutput {
			gira.EnsureWorkStartResultSchema(&result)
			result.NextStep = ticketWorkStartNextStep(result)
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
		}
		fmt.Fprintf(stderr, "%v\n", err)
		if next := ticketWorkStartNextStep(result); next != "" {
			fmt.Fprintf(stderr, "next step: %s\n", next)
		}
		return 1
	}
	if *jsonOutput {
		gira.EnsureWorkStartResultSchema(&result)
		result.NextStep = ticketWorkStartNextStep(result)
		if *dryRun {
			result.Approval = gira.WorkStartApprovalEvidence(result, "gira ticket start")
		}
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
		if *dryRun {
			result.Approval = gira.WorkPRApprovalEvidence(result, "gira ticket pr")
		}
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, formatTicketPR(result))
	return 0
}

func runTicketNote(args []string, stdout io.Writer, stderr io.Writer) int {
	args, positionalTicket, positionalBody, positionalOK := extractTicketNotePositionals(args, stderr)
	if !positionalOK {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	fs := flag.NewFlagSet("ticket note", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	ticket := fs.Int("ticket", 0, "Ticket number")
	issue := fs.Int("issue", 0, "Compatibility alias for --ticket")
	body := fs.String("body", "", "Ticket note body")
	bodyFile := fs.String("body-file", "", "Read ticket note body from file")
	kind := fs.String("kind", "progress", "Ticket note kind")
	target := fs.String("target", "auto", "Ticket note target")
	dryRun := fs.Bool("dry-run", false, "Preview without mutation")
	apply := fs.Bool("apply", false, "Apply comment")
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
	if *dryRun == *apply {
		fmt.Fprint(stderr, "exactly one of --dry-run/--apply is required\n\n")
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	resolvedBody, err := readTicketNoteBody(positionalBody, *body, *bodyFile, os.Stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
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
	result, err := newTicketNoteReport(gira.TicketNoteInput{
		Repo:   repo,
		Ticket: ticketNumber,
		Body:   resolvedBody,
		Kind:   *kind,
		Target: *target,
		DryRun: *dryRun,
		Apply:  *apply,
	})
	result.Status.NextStep = shortenTicketNextStep(result.Status.NextStep, result.Repo, result.Ticket)
	result.NextStep = shortenTicketNextStep(result.NextStep, result.Repo, result.Ticket)
	if err != nil {
		if *jsonOutput {
			gira.EnsureTicketNoteReportSchema(&result)
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
		}
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if *jsonOutput {
		gira.EnsureTicketNoteReportSchema(&result)
		if *dryRun {
			result.Approval = gira.TicketNoteApprovalEvidence(result)
		}
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatTicketNote(result))
	return 0
}

func runTicketSupersede(args []string, stdout io.Writer, stderr io.Writer) int {
	args, positionalTicket, positionalOK := extractTicketPositional(args, stderr)
	if !positionalOK {
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	fs := flag.NewFlagSet("ticket supersede", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	ticket := fs.Int("ticket", 0, "Ticket number")
	issue := fs.Int("issue", 0, "Compatibility alias for --ticket")
	replacementTitle := fs.String("replacement-title", "", "Replacement ticket title")
	body := fs.String("body", "", "Replacement issue body")
	bodyFile := fs.String("body-file", "", "Read replacement issue body from file")
	milestone := fs.String("milestone", "", "Override replacement milestone")
	closeDraftPR := fs.Bool("close-draft-pr", false, "Close linked draft PR")
	dryRun := fs.Bool("dry-run", false, "Preview without mutation")
	apply := fs.Bool("apply", false, "Apply changes")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	var labels repeatedStringFlag
	fs.Var(&labels, "label", "Additional replacement GitHub label")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	if *help {
		_, _ = io.WriteString(stdout, ticketHelp)
		return 0
	}
	if *dryRun == *apply {
		fmt.Fprint(stderr, "exactly one of --dry-run/--apply is required\n\n")
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	resolvedBody, err := readTicketNewBody(*body, *bodyFile, os.Stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
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
	result, err := newTicketSupersedeReport(gira.TicketSupersedeInput{
		Repo:             repo,
		Ticket:           ticketNumber,
		ReplacementTitle: *replacementTitle,
		Body:             resolvedBody,
		Labels:           labels,
		Milestone:        *milestone,
		CloseDraftPR:     *closeDraftPR,
		DryRun:           *dryRun,
		Apply:            *apply,
	})
	result.NextStep = shortenTicketNextStep(result.NextStep, result.Repo, result.Replacement.Number)
	if err != nil {
		if *jsonOutput {
			gira.EnsureTicketSupersedeReportSchema(&result)
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
		}
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if *jsonOutput {
		gira.EnsureTicketSupersedeReportSchema(&result)
		if *dryRun {
			result.Approval = gira.TicketSupersedeApprovalEvidence(result)
		}
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatTicketSupersede(result))
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
	syncLocal := fs.Bool("sync-local", false, "Opt in to syncing the local PR base branch after finish")
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
	result, err := newWorkFinishResult(repo, ticketNumber, *dryRun, *wait, gira.WorkFinishOptions{SyncLocal: *syncLocal})
	if result.Wait == "" {
		result.Wait = wait.String()
	}
	if *syncLocal {
		result.SyncLocal = true
	}
	result = normalizeTicketFinishResult(result)
	if err != nil {
		if *jsonOutput {
			gira.EnsureWorkFinishResultSchema(&result)
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
		}
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if *jsonOutput {
		gira.EnsureWorkFinishResultSchema(&result)
		if *dryRun {
			result.Approval = gira.WorkFinishApprovalEvidence(result)
		}
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
	htmlOutput := fs.Bool("html", false, "Write a static local HTML report")
	outputPath := fs.String("output", "", "Output path for --html")
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
	if *jsonOutput && *htmlOutput {
		fmt.Fprint(stderr, "choose exactly one output format: --json or --html\n\n")
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	if *htmlOutput && strings.TrimSpace(*outputPath) == "" {
		fmt.Fprint(stderr, "--output is required when using --html\n\n")
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
	}
	if !*htmlOutput && strings.TrimSpace(*outputPath) != "" {
		fmt.Fprint(stderr, "--output requires --html\n\n")
		_, _ = io.WriteString(stderr, ticketHelp)
		return 2
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
	if *htmlOutput {
		if err := gira.WriteTicketStatusHTML(*outputPath, result); err != nil {
			fmt.Fprintf(stderr, "write ticket status HTML: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "ticket status html: %s\n", filepath.Clean(*outputPath))
		fmt.Fprintf(stdout, "next step: open %s\n", filepath.Clean(*outputPath))
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
		if next := strings.TrimSpace(result.NextStep); next != "" {
			fmt.Fprintf(stderr, "next step: %s\n", next)
		}
		return 1
	}
	if *jsonOutput {
		gira.EnsureWorkStartResultSchema(&result)
		if *dryRun {
			result.Approval = gira.WorkStartApprovalEvidence(result, "gira work start")
		}
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
		if *dryRun {
			result.Approval = gira.WorkPRApprovalEvidence(result, "gira work pr")
		}
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
	valueFlags := map[string]struct{}{"--repo": {}, "--ticket": {}, "--issue": {}, "--wait": {}, "--timeout": {}, "--interval": {}, "--replacement-title": {}, "--body": {}, "--body-file": {}, "--milestone": {}, "--label": {}, "--output": {}}
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

func extractNumericPositional(args []string, noun string, stderr io.Writer) ([]string, int, bool) {
	cleaned := make([]string, 0, len(args))
	positional := 0
	valueFlags := map[string]struct{}{"--repo": {}, "--goal": {}, "--terminal": {}, "--output": {}}
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
			fmt.Fprintf(stderr, "unexpected positional argument %q; use a numeric %s or --%s N\n\n", arg, noun, noun)
			return nil, 0, false
		}
		if positional > 0 && positional != n {
			fmt.Fprintf(stderr, "only one positional %s can be provided\n\n", noun)
			return nil, 0, false
		}
		positional = n
		cleaned = cleaned[:len(cleaned)-1]
	}
	return cleaned, positional, true
}

func extractRunStartTicketPositional(args []string, stderr io.Writer) ([]string, int, bool) {
	cleaned := make([]string, 0, len(args))
	positional := 0
	valueFlags := map[string]struct{}{
		"--repo":         {},
		"--ticket":       {},
		"--issue":        {},
		"--role":         {},
		"--profile":      {},
		"--context":      {},
		"--context-file": {},
		"--name":         {},
		"--id":           {},
		"--state-root":   {},
		"--workdir":      {},
	}
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

func extractFeatureForPositional(args []string, stderr io.Writer) ([]string, int, bool) {
	cleaned := make([]string, 0, len(args))
	positional := 0
	valueFlags := map[string]struct{}{"--repo": {}, "--issue": {}, "--limit": {}}
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
			fmt.Fprintf(stderr, "unexpected positional argument %q; use a numeric issue or --issue N\n\n", arg)
			return nil, 0, false
		}
		if positional > 0 && positional != n {
			fmt.Fprint(stderr, "only one positional issue can be provided\n\n")
			return nil, 0, false
		}
		positional = n
		cleaned = cleaned[:len(cleaned)-1]
	}
	return cleaned, positional, true
}

func extractTicketPromptRolePositional(args []string, stderr io.Writer) ([]string, string, bool) {
	cleaned := make([]string, 0, len(args))
	role := ""
	valueFlags := map[string]struct{}{"--repo": {}, "--ticket": {}, "--issue": {}, "--role": {}, "--profile": {}, "--pr": {}, "--output": {}}
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
		lower := strings.ToLower(strings.TrimSpace(arg))
		if lower != gira.AgentPromptRolePlanner && lower != gira.AgentPromptRoleImplementer && lower != gira.AgentPromptRoleReviewer {
			continue
		}
		if role != "" && role != lower {
			fmt.Fprint(stderr, "only one positional prompt role can be provided\n\n")
			return nil, "", false
		}
		role = lower
		cleaned = cleaned[:len(cleaned)-1]
	}
	return cleaned, role, true
}

type ticketIdentifier struct {
	Number  int
	JiraKey string
	Seen    bool
}

func extractTicketIdentifierPositional(args []string, stderr io.Writer) ([]string, ticketIdentifier, bool) {
	cleaned := make([]string, 0, len(args))
	var identifier ticketIdentifier
	seen := false
	valueFlags := map[string]struct{}{"--repo": {}, "--ticket": {}, "--issue": {}, "--role": {}, "--profile": {}, "--pr": {}, "--output": {}}
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
		if n, err := strconv.Atoi(arg); err == nil && n > 0 {
			if seen && identifier.Number != n {
				fmt.Fprint(stderr, "only one positional ticket or Jira key can be provided\n\n")
				return nil, ticketIdentifier{}, false
			}
			if seen && identifier.JiraKey != "" {
				fmt.Fprint(stderr, "only one positional ticket or Jira key can be provided\n\n")
				return nil, ticketIdentifier{}, false
			}
			identifier.Number = n
		} else if jiraKeyPositionalPattern.MatchString(strings.ToUpper(strings.TrimSpace(arg))) {
			key := strings.ToUpper(strings.TrimSpace(arg))
			if seen {
				fmt.Fprint(stderr, "only one positional ticket or Jira key can be provided\n\n")
				return nil, ticketIdentifier{}, false
			}
			identifier.JiraKey = key
		} else {
			fmt.Fprintf(stderr, "unexpected positional argument %q; use a numeric ticket, Jira key, or --ticket N\n\n", arg)
			return nil, ticketIdentifier{}, false
		}
		seen = true
		identifier.Seen = true
		cleaned = cleaned[:len(cleaned)-1]
	}
	return cleaned, identifier, true
}

func extractTicketNotePositionals(args []string, stderr io.Writer) ([]string, int, string, bool) {
	cleaned := make([]string, 0, len(args))
	positionalTicket := 0
	positionalBody := ""
	valueFlags := map[string]struct{}{"--repo": {}, "--ticket": {}, "--issue": {}, "--kind": {}, "--target": {}, "--body": {}, "--body-file": {}}
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
		if n, err := strconv.Atoi(arg); err == nil && n > 0 && positionalTicket == 0 {
			positionalTicket = n
			cleaned = cleaned[:len(cleaned)-1]
			continue
		}
		if positionalBody != "" {
			fmt.Fprint(stderr, "only one positional note body can be provided; use --body for explicit body\n\n")
			return nil, 0, "", false
		}
		positionalBody = arg
		cleaned = cleaned[:len(cleaned)-1]
	}
	return cleaned, positionalTicket, positionalBody, true
}

func extractTitlePositional(args []string, stderr io.Writer) ([]string, string, bool) {
	cleaned := make([]string, 0, len(args))
	title := ""
	valueFlags := map[string]struct{}{"--repo": {}, "--title": {}, "--goal": {}, "--scope": {}, "--acceptance": {}, "--notes": {}, "--body": {}, "--type": {}, "--priority": {}, "--milestone": {}, "--label": {}, "--body-file": {}}
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

func readTicketNewBody(body string, bodyFile string, stdin io.Reader) (string, error) {
	body = strings.TrimSpace(body)
	bodyFile = strings.TrimSpace(bodyFile)
	if body != "" && bodyFile != "" {
		return "", fmt.Errorf("use either --body or --body-file, not both")
	}
	if body != "" {
		return body, nil
	}
	if bodyFile == "" {
		return "", nil
	}
	var content []byte
	var err error
	if bodyFile == "-" {
		content, err = io.ReadAll(stdin)
	} else {
		content, err = os.ReadFile(bodyFile)
	}
	if err != nil {
		return "", fmt.Errorf("read --body-file: %w", err)
	}
	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" {
		return "", fmt.Errorf("--body-file is empty")
	}
	return trimmed, nil
}

func readTicketNoteBody(positional string, body string, bodyFile string, stdin io.Reader) (string, error) {
	body = strings.TrimSpace(body)
	positional = strings.TrimSpace(positional)
	bodyFile = strings.TrimSpace(bodyFile)
	set := 0
	if positional != "" {
		set++
	}
	if body != "" {
		set++
	}
	if bodyFile != "" {
		set++
	}
	if set > 1 {
		return "", fmt.Errorf("use only one of positional body, --body, or --body-file")
	}
	if positional != "" {
		return positional, nil
	}
	if body != "" {
		return body, nil
	}
	if bodyFile == "" {
		return "", fmt.Errorf("ticket note body is required")
	}
	return readTicketNewBody("", bodyFile, stdin)
}

func readRunStartContext(contextNote string, contextFile string, stdin io.Reader) ([]string, error) {
	contextNote = strings.TrimSpace(contextNote)
	contextFile = strings.TrimSpace(contextFile)
	notes := []string{}
	if contextNote != "" {
		notes = append(notes, contextNote)
	}
	if contextFile == "" {
		return notes, nil
	}
	var content []byte
	var err error
	if contextFile == "-" {
		content, err = io.ReadAll(stdin)
	} else {
		content, err = os.ReadFile(contextFile)
	}
	if err != nil {
		return nil, fmt.Errorf("read --context-file: %w", err)
	}
	if len(content) > 16*1024 {
		return nil, fmt.Errorf("--context-file must be 16 KiB or smaller")
	}
	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" {
		return nil, fmt.Errorf("--context-file is empty")
	}
	notes = append(notes, trimmed)
	return notes, nil
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

func parseOptionalNumericPositional(args []string, noun string, stderr io.Writer) (int, bool) {
	if len(args) == 0 {
		return 0, true
	}
	if len(args) > 1 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", args[1])
		return 0, false
	}
	n, err := strconv.Atoi(args[0])
	if err != nil || n <= 0 {
		fmt.Fprintf(stderr, "unexpected positional argument %q; use a numeric %s number or --ticket N\n\n", args[0], noun)
		return 0, false
	}
	return n, true
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

func resolveTicketIdentifierContext(repo gira.RepoRef, ticket int, issue int, positional ticketIdentifier, allowInference bool, stderr io.Writer) (int, string, bool, error) {
	explicit, ok := resolveExplicitTicket(ticket, issue, positional.Number, stderr)
	if !ok {
		return 0, "", false, nil
	}
	if explicit > 0 && positional.JiraKey != "" {
		fmt.Fprint(stderr, "--ticket/--issue and Jira key positional cannot be combined\n\n")
		return 0, "", false, nil
	}
	if explicit > 0 {
		return explicit, "", true, nil
	}
	if positional.JiraKey != "" {
		mirror, err := newJiraMirrorIssueResolver(repo, positional.JiraKey)
		if err != nil {
			return 0, "", false, err
		}
		return mirror.Number, strings.ToUpper(strings.TrimSpace(positional.JiraKey)), true, nil
	}
	if !allowInference {
		fmt.Fprint(stderr, "--ticket or positional ticket is required for ticket start\n\n")
		return 0, "", false, nil
	}
	inferred, err := inferTicketFromCurrentContext(repo, devCommandRunner)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		return 0, "", false, nil
	}
	return inferred, "", true, nil
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
	branch := ""
	if out, err := runner.Run("git", "branch", "--show-current"); err == nil {
		branch = strings.TrimSpace(string(out))
		if n := issueNumberFromRef(branch); n > 0 {
			return n, nil
		}
	}
	if branch != "" && branch != "main" && branch != "master" {
		if n, err := inferTicketFromBranchPRs(repo, branch, runner); err == nil && n > 0 {
			return n, nil
		} else if err != nil {
			return 0, err
		}
	}
	out, err := runner.Run("gh", "pr", "view", "--repo", repo.FullName(), "--json", "body,headRefName,title")
	if err != nil {
		return 0, missingTicketContextError(repo, branch)
	}
	var raw struct {
		Body        string `json:"body"`
		HeadRefName string `json:"headRefName"`
		Title       string `json:"title"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return 0, fmt.Errorf("parse PR context JSON: %w", err)
	}
	candidates := ticketContextCandidatesFromPRs([]ticketContextPRCandidate{{
		Body:        raw.Body,
		HeadRefName: raw.HeadRefName,
		Title:       raw.Title,
	}})
	if n, ok := singleTicketContextCandidate(candidates); ok {
		return n, nil
	}
	if len(candidates) > 1 {
		return 0, ambiguousTicketContextError(candidates)
	}
	return 0, missingTicketContextError(repo, branch)
}

type ticketContextPRCandidate struct {
	Number      int    `json:"number"`
	Body        string `json:"body"`
	HeadRefName string `json:"headRefName"`
	Title       string `json:"title"`
	URL         string `json:"url"`
}

type ticketContextCandidate struct {
	Ticket int
	PR     int
	Source string
}

func inferTicketFromBranchPRs(repo gira.RepoRef, branch string, runner gira.CommandRunner) (int, error) {
	out, err := runner.Run("gh", "pr", "list", "--repo", repo.FullName(), "--head", branch, "--state", "all", "--json", "number,body,headRefName,title,url", "--limit", "20")
	if err != nil {
		return 0, nil
	}
	var prs []ticketContextPRCandidate
	if err := json.Unmarshal(out, &prs); err != nil {
		return 0, fmt.Errorf("parse branch PR context JSON: %w", err)
	}
	candidates := ticketContextCandidatesFromPRs(prs)
	if n, ok := singleTicketContextCandidate(candidates); ok {
		return n, nil
	}
	if len(candidates) > 1 {
		return 0, ambiguousTicketContextError(candidates)
	}
	return 0, nil
}

func ticketContextCandidatesFromPRs(prs []ticketContextPRCandidate) []ticketContextCandidate {
	seen := map[string]struct{}{}
	candidates := []ticketContextCandidate{}
	add := func(ticket int, pr int, source string) {
		if ticket <= 0 {
			return
		}
		key := fmt.Sprintf("%d/%d/%s", ticket, pr, source)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		candidates = append(candidates, ticketContextCandidate{Ticket: ticket, PR: pr, Source: source})
	}
	for _, pr := range prs {
		for _, n := range gira.ExtractClosureIssueNumbers(pr.Body) {
			add(n, pr.Number, "closing_reference")
		}
		for _, ref := range []string{pr.HeadRefName, pr.Title} {
			if n := issueNumberFromRef(ref); n > 0 {
				add(n, pr.Number, "ref_name")
			}
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Ticket != candidates[j].Ticket {
			return candidates[i].Ticket < candidates[j].Ticket
		}
		if candidates[i].PR != candidates[j].PR {
			return candidates[i].PR < candidates[j].PR
		}
		return candidates[i].Source < candidates[j].Source
	})
	return candidates
}

func singleTicketContextCandidate(candidates []ticketContextCandidate) (int, bool) {
	if len(candidates) == 0 {
		return 0, false
	}
	ticket := candidates[0].Ticket
	for _, candidate := range candidates[1:] {
		if candidate.Ticket != ticket {
			return 0, false
		}
	}
	return ticket, true
}

func ambiguousTicketContextError(candidates []ticketContextCandidate) error {
	parts := make([]string, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		part := fmt.Sprintf("#%d", candidate.Ticket)
		if candidate.PR > 0 {
			part = fmt.Sprintf("#%d via PR #%d", candidate.Ticket, candidate.PR)
		}
		if candidate.Source != "" {
			part += " " + candidate.Source
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		parts = append(parts, part)
	}
	return fmt.Errorf("ticket context ambiguous: multiple candidates match the current branch or PR context\nCandidates: %s\nRe-run with: --ticket N", strings.Join(parts, ", "))
}

func missingTicketContextError(repo gira.RepoRef, branch string) error {
	if strings.TrimSpace(branch) != "" {
		return fmt.Errorf("cannot determine current ticket for branch %q\nMissing: explicit --ticket, issue-N-* branch name, or PR closing reference\nTry: gira ticket status --repo %s --ticket N\nOr: open a PR with Closes #N in %s", branch, repo.FullName(), repo.FullName())
	}
	return fmt.Errorf("cannot determine current ticket\nMissing: explicit --ticket, issue-N-* branch name, or PR closing reference\nTry: gira ticket status --repo %s --ticket N\nOr: run from an issue-N-* branch or PR checkout", repo.FullName())
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
	next := ticketWorkStartNextStep(result)
	if next == "" {
		next = fmt.Sprintf("gira ticket pr --repo %s --ticket %d --dry-run", result.Repo, result.Issue)
		if result.DryRun {
			next = fmt.Sprintf("gira ticket start %d --apply", result.Issue)
		} else {
			next = "gira ticket pr --dry-run"
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "ticket start: ticket #%d branch=%s status=%s\n", result.Issue, result.Branch, result.NextStatus)
	if strings.TrimSpace(result.BaseBranch) != "" {
		fmt.Fprintf(&b, "base: %s", result.BaseBranch)
		if strings.TrimSpace(result.BaseSource) != "" {
			fmt.Fprintf(&b, " (%s)", result.BaseSource)
		}
		b.WriteString("\n")
	}
	if strings.TrimSpace(result.JiraKey) != "" {
		fmt.Fprintf(&b, "jira key: %s\n", result.JiraKey)
		fmt.Fprintf(&b, "mirror issue: #%d\n", result.MirrorIssue)
	}
	fmt.Fprintf(&b, "next step: %s\n", next)
	return b.String()
}

func formatTicketPR(result gira.WorkPRResult) string {
	created := "reused"
	if result.Created {
		created = "created"
	}
	url := strings.TrimSpace(result.PRURL)
	if url == "" {
		url = "(planned)"
		created = "planned"
	}
	next := "gira ticket status"
	if result.DryRun {
		next = "gira ticket pr --apply"
		if result.Draft {
			next += " --draft"
		}
	}
	if result.Draft && !result.DryRun {
		next = "mark the PR ready, then " + next
	}
	branchPush := ""
	if result.BranchPush != "" && result.BranchPush != "skipped" {
		branchPush = fmt.Sprintf(" branch_push=%s", result.BranchPush)
	}
	base := ""
	if strings.TrimSpace(result.RecordedBase) != "" {
		base = fmt.Sprintf(" base=%s", result.RecordedBase)
		if strings.TrimSpace(result.ActualBase) != "" && result.ActualBase != result.RecordedBase {
			base += fmt.Sprintf(" actual_base=%s", result.ActualBase)
		}
	}
	return fmt.Sprintf("ticket pr: ticket #%d pr=%s status=%s %s%s%s\nnext step: %s\n", result.Issue, url, result.NextStatus, created, branchPush, base, next)
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
	readiness := "unknown"
	if result.Readiness.SchemaVersion != "" {
		readiness = "blocked"
		if result.Readiness.Ready {
			readiness = "ready"
		}
	}
	actions := make([]string, 0, len(result.Actions))
	for _, action := range result.Actions {
		actions = append(actions, action.Action+":"+action.Status)
	}
	if len(actions) == 0 {
		actions = append(actions, "none")
	}
	return fmt.Sprintf(
		"ticket finish: ticket #%d pr=%d merged=%t readiness=%s blockers=%s actions=%s\nnext step: %s\n",
		result.Issue,
		result.PRNumber,
		result.Merged,
		readiness,
		blockers,
		strings.Join(actions, ","),
		ticketFinishNextStep(result),
	)
}

func ticketStatusNextStep(result gira.WorkStatusResult) string {
	switch result.NextAction {
	case "start_work":
		if isMissingTicketStatus(result.Status) {
			return readyTicketStatusNextStep(result.Repo, result.Issue)
		}
		return fmt.Sprintf("gira ticket start %d --apply", result.Issue)
	case "open_pr":
		return "gira ticket pr --apply"
	case "resolve_blockers":
		return "resolve blockers, then set status:ready before starting work"
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

func ticketWorkStartNextStep(result gira.WorkStartResult) string {
	next := strings.TrimSpace(result.NextStep)
	if next == "" {
		return ""
	}
	if strings.HasPrefix(next, "gira adopt issues ") {
		return next
	}
	next = strings.ReplaceAll(next, "gira work status", "gira ticket status")
	next = strings.ReplaceAll(next, "gira work pr", "gira ticket pr")
	next = strings.ReplaceAll(next, "gira work start", "gira ticket start")
	next = strings.ReplaceAll(next, "--issue", "--ticket")
	next = shortenTicketNextStep(next, result.Repo, result.Issue)
	if strings.HasPrefix(next, "gira ticket start ") && result.Issue > 0 && !strings.Contains(next, strconv.Itoa(result.Issue)) {
		next = strings.Replace(next, "gira ticket start ", fmt.Sprintf("gira ticket start %d ", result.Issue), 1)
	}
	return next
}

func readyTicketStatusNextStep(repo string, issue int) string {
	return fmt.Sprintf("gira adopt issues --repo %s --issue %d --label status:ready --apply", repo, issue)
}

func isMissingTicketStatus(status string) bool {
	trimmed := strings.TrimSpace(status)
	return trimmed == "" || strings.EqualFold(trimmed, "null")
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

func shortenEpicNextStep(next string, repo string, epic int) string {
	if repo != "" {
		next = strings.ReplaceAll(next, " --repo "+repo, "")
	}
	if epic > 0 {
		next = strings.ReplaceAll(next, fmt.Sprintf(" --ticket %d", epic), "")
	}
	return strings.Join(strings.Fields(next), " ")
}

func parseRepeatedIssueNumbers(values []string) ([]int, error) {
	seen := map[int]struct{}{}
	var numbers []int
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			if strings.Contains(trimmed, "-") {
				bounds := strings.Split(trimmed, "-")
				if len(bounds) != 2 {
					return nil, fmt.Errorf("--issue/--issues values must be positive integers or ranges")
				}
				start, startErr := strconv.Atoi(strings.TrimSpace(bounds[0]))
				end, endErr := strconv.Atoi(strings.TrimSpace(bounds[1]))
				if startErr != nil || endErr != nil || start <= 0 || end <= 0 || start > end {
					return nil, fmt.Errorf("--issue/--issues ranges must be positive ascending integers")
				}
				for n := start; n <= end; n++ {
					if _, ok := seen[n]; ok {
						continue
					}
					seen[n] = struct{}{}
					numbers = append(numbers, n)
				}
				continue
			}
			n, err := strconv.Atoi(trimmed)
			if err != nil || n <= 0 {
				return nil, fmt.Errorf("--issue/--issues values must be positive integers")
			}
			if _, ok := seen[n]; ok {
				continue
			}
			seen[n] = struct{}{}
			numbers = append(numbers, n)
		}
	}
	return numbers, nil
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
	fmt.Fprint(stdout, gira.FormatBootstrapInstallSummary(result, repo))
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

func runQueue(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, queueHelp)
		return 0
	}
	switch args[0] {
	case "list":
		return runQueueList(args[1:], stdout, stderr)
	case "next":
		return runQueueNext(args[1:], stdout, stderr)
	case "handoff":
		return runQueueHandoff(args[1:], stdout, stderr)
	case "take":
		return runQueueTake(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown queue command: %s\n\n", args[0])
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
}

func runQueueList(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("queue list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", "", "Workspace config path")
	var repos repeatedStringFlag
	var queues repeatedStringFlag
	fs.Var(&repos, "repo", "Execution repo to include. Repeatable or comma-separated")
	fs.Var(&queues, "queue", "Queue filter. Repeatable or comma-separated")
	limit := fs.Int("limit", 0, "Maximum queue items to print")
	compact := fs.Bool("compact", false, "Print compact text output")
	maxConcurrency := fs.Int("max-concurrency", 4, "Maximum concurrent repo status fetches")
	cacheTTL := fs.Duration("cache-ttl", 5*time.Minute, "Reuse recent per-repo status cache for this duration. Use 0 to disable")
	refresh := fs.Bool("refresh", false, "Ignore cached workspace status and fetch fresh data")
	cacheRoot := fs.String("cache-root", "", "Workspace status cache root")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, queueHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	if *limit < 0 {
		fmt.Fprint(stderr, "--limit must be at least 0\n\n")
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	if *maxConcurrency < 1 {
		fmt.Fprint(stderr, "--max-concurrency must be at least 1\n\n")
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	if *cacheTTL < 0 {
		fmt.Fprint(stderr, "--cache-ttl must be non-negative\n\n")
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	selectedRepos, repoFilters, err := parseQueueRepoFilters(repos)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	workspaceReport, err := newWorkspaceStatusReportWithOptions(*configPath, gira.WorkspaceStatusOptions{
		Repos:          selectedRepos,
		MaxConcurrency: *maxConcurrency,
		CacheTTL:       *cacheTTL,
		Refresh:        *refresh,
		CacheRoot:      *cacheRoot,
	})
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := gira.BuildQueueListReport(workspaceReport, gira.QueueListOptions{
		QueueNames:  append([]string(nil), queues...),
		RepoFilters: repoFilters,
		Limit:       *limit,
	})
	if err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	if *jsonOutput {
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode queue list JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatQueueList(report, *compact))
	return 0
}

func runQueueNext(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("queue next", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", "", "Workspace config path")
	var repos repeatedStringFlag
	fs.Var(&repos, "repo", "Execution repo to include. Repeatable or comma-separated")
	role := fs.String("role", gira.AgentPromptRoleImplementer, "Handoff role: planner|implementer|reviewer")
	profile := fs.String("profile", gira.AgentPromptProfileDefault, "Handoff profile: default|python")
	compact := fs.Bool("compact", false, "Print compact text output")
	maxConcurrency := fs.Int("max-concurrency", 4, "Maximum concurrent repo status fetches")
	cacheTTL := fs.Duration("cache-ttl", 5*time.Minute, "Reuse recent per-repo status cache for this duration. Use 0 to disable")
	refresh := fs.Bool("refresh", false, "Ignore cached workspace status and fetch fresh data")
	cacheRoot := fs.String("cache-root", "", "Workspace status cache root")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, queueHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	if *maxConcurrency < 1 {
		fmt.Fprint(stderr, "--max-concurrency must be at least 1\n\n")
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	if *cacheTTL < 0 {
		fmt.Fprint(stderr, "--cache-ttl must be non-negative\n\n")
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	selectedRepos, repoFilters, err := parseQueueRepoFilters(repos)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	workspaceReport, err := newWorkspaceStatusReportWithOptions(*configPath, gira.WorkspaceStatusOptions{
		Repos:          selectedRepos,
		MaxConcurrency: *maxConcurrency,
		CacheTTL:       *cacheTTL,
		Refresh:        *refresh,
		CacheRoot:      *cacheRoot,
	})
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := gira.BuildQueueNextReport(workspaceReport, gira.QueueNextOptions{
		RepoFilters: repoFilters,
		Role:        *role,
		Profile:     *profile,
	})
	if err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	if *jsonOutput {
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode queue next JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatQueueNext(report, *compact))
	return 0
}

func runQueueHandoff(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("queue handoff", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", "", "Workspace config path")
	var repos repeatedStringFlag
	fs.Var(&repos, "repo", "Execution repo to include. Repeatable or comma-separated")
	ticket := fs.Int("ticket", 0, "Ticket number")
	issue := fs.Int("issue", 0, "Compatibility alias for --ticket")
	roleValue := fs.String("role", gira.AgentPromptRoleImplementer, "Handoff role: planner|implementer|reviewer")
	profileValue := fs.String("profile", gira.AgentPromptProfileDefault, "Handoff profile: default|python")
	compact := fs.Bool("compact", false, "Print compact text output")
	maxConcurrency := fs.Int("max-concurrency", 4, "Maximum concurrent repo status fetches")
	cacheTTL := fs.Duration("cache-ttl", 5*time.Minute, "Reuse recent per-repo status cache for this duration. Use 0 to disable")
	refresh := fs.Bool("refresh", false, "Ignore cached workspace status and fetch fresh data")
	cacheRoot := fs.String("cache-root", "", "Workspace status cache root")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, queueHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	if *issue > 0 {
		if *ticket > 0 && *ticket != *issue {
			fmt.Fprint(stderr, "--ticket and --issue must refer to the same number\n\n")
			fmt.Fprint(stderr, queueHelp)
			return 2
		}
		*ticket = *issue
	}
	if *maxConcurrency < 1 {
		fmt.Fprint(stderr, "--max-concurrency must be at least 1\n\n")
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	if *cacheTTL < 0 {
		fmt.Fprint(stderr, "--cache-ttl must be non-negative\n\n")
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	role, err := gira.NormalizeQueueRole(*roleValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	profile, err := gira.NormalizeQueueProfile(*profileValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	selectedRepos, repoFilters, err := parseQueueRepoFilters(repos)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	if *ticket > 0 && len(repoFilters) == 0 {
		repo, err := gira.ResolveRepoContext("", repoContextRunner)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
		selectedRepos = []gira.RepoRef{repo}
		repoFilters = []string{repo.FullName()}
	}
	if *ticket > 0 && len(repoFilters) != 1 {
		fmt.Fprint(stderr, "--ticket requires exactly one --repo or an inferable current repo\n\n")
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	workspaceReport, err := newWorkspaceStatusReportWithOptions(*configPath, gira.WorkspaceStatusOptions{
		Repos:          selectedRepos,
		MaxConcurrency: *maxConcurrency,
		CacheTTL:       *cacheTTL,
		Refresh:        *refresh,
		CacheRoot:      *cacheRoot,
	})
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := queueHandoffReport(workspaceReport, repoFilters, *ticket, role, profile)
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
			fmt.Fprintf(stderr, "encode queue handoff JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatQueueHandoff(report, *compact))
	return 0
}

func runQueueTake(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("queue take", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", "", "Workspace config path")
	var repos repeatedStringFlag
	fs.Var(&repos, "repo", "Execution repo to include. Repeatable or comma-separated")
	ticket := fs.Int("ticket", 0, "Ticket number")
	issue := fs.Int("issue", 0, "Compatibility alias for --ticket")
	roleValue := fs.String("role", gira.AgentPromptRoleImplementer, "Handoff role: planner|implementer|reviewer")
	profileValue := fs.String("profile", gira.AgentPromptProfileDefault, "Handoff profile: default|python")
	dryRun := fs.Bool("dry-run", false, "Preview ticket start without mutation")
	apply := fs.Bool("apply", false, "Apply ticket start for a handoff-safe queue item")
	compact := fs.Bool("compact", false, "Print compact text output")
	maxConcurrency := fs.Int("max-concurrency", 4, "Maximum concurrent repo status fetches")
	cacheTTL := fs.Duration("cache-ttl", 5*time.Minute, "Reuse recent per-repo status cache for this duration. Use 0 to disable")
	refresh := fs.Bool("refresh", false, "Ignore cached workspace status and fetch fresh data")
	cacheRoot := fs.String("cache-root", "", "Workspace status cache root")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, queueHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	if *dryRun == *apply {
		fmt.Fprint(stderr, "choose exactly one of --dry-run or --apply\n\n")
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	if *issue > 0 {
		if *ticket > 0 && *ticket != *issue {
			fmt.Fprint(stderr, "--ticket and --issue must refer to the same number\n\n")
			fmt.Fprint(stderr, queueHelp)
			return 2
		}
		*ticket = *issue
	}
	if *maxConcurrency < 1 {
		fmt.Fprint(stderr, "--max-concurrency must be at least 1\n\n")
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	if *cacheTTL < 0 {
		fmt.Fprint(stderr, "--cache-ttl must be non-negative\n\n")
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	role, err := gira.NormalizeQueueRole(*roleValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	profile, err := gira.NormalizeQueueProfile(*profileValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	selectedRepos, repoFilters, err := parseQueueRepoFilters(repos)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	if *ticket > 0 && len(repoFilters) == 0 {
		repo, err := gira.ResolveRepoContext("", repoContextRunner)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
		selectedRepos = []gira.RepoRef{repo}
		repoFilters = []string{repo.FullName()}
	}
	if *ticket > 0 && len(repoFilters) != 1 {
		fmt.Fprint(stderr, "--ticket requires exactly one --repo or an inferable current repo\n\n")
		fmt.Fprint(stderr, queueHelp)
		return 2
	}
	workspaceReport, err := newWorkspaceStatusReportWithOptions(*configPath, gira.WorkspaceStatusOptions{
		Repos:          selectedRepos,
		MaxConcurrency: *maxConcurrency,
		CacheTTL:       *cacheTTL,
		Refresh:        *refresh,
		CacheRoot:      *cacheRoot,
	})
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := queueTakeReport(workspaceReport, repoFilters, *ticket, role, profile, *dryRun, *apply)
	if *jsonOutput {
		out, encodeErr := json.MarshalIndent(report, "", "  ")
		if encodeErr != nil {
			fmt.Fprintf(stderr, "encode queue take JSON: %v\n", encodeErr)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
	} else {
		fmt.Fprint(stdout, gira.FormatQueueTake(report, *compact))
	}
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if *apply && len(report.StopReasons) > 0 {
		return 1
	}
	return 0
}

func runWorkspace(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, workspaceHelp)
		return 0
	}
	switch args[0] {
	case "init":
		return runWorkspaceInit(args[1:], stdout, stderr)
	case "validate":
		return runWorkspaceValidate(args[1:], stdout, stderr)
	case "status", "backlog", "list":
		return runWorkspaceStatus(args[0], args[1:], stdout, stderr)
	case "sync":
		return runWorkspaceSync(args[1:], stdout, stderr)
	case "repos":
		return runWorkspaceRepos(args[1:], stdout, stderr)
	case "ticket":
		return runWorkspaceTicket(args[1:], stdout, stderr)
	case "capability":
		return runWorkspaceCapability(args[1:], stdout, stderr)
	case "project":
		return runWorkspaceProject(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown workspace command: %s\n\n", args[0])
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
}

func runWorkspaceRepos(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, workspaceHelp)
		return 0
	}
	switch args[0] {
	case "sync":
		return runWorkspaceReposSync(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown workspace repos command: %s\n\n", args[0])
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
}

func runWorkspaceReposSync(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("workspace repos sync", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	owner := fs.String("owner", "", "GitHub owner or organization to discover")
	workspaceName := fs.String("workspace", "", "Global workspace name; defaults to global config default_workspace")
	configRoot := fs.String("config-root", "", "Override global config root")
	limit := fs.Int("limit", 100, "Maximum repositories to discover")
	includeArchived := fs.Bool("include-archived", false, "Include archived repositories")
	dryRun := fs.Bool("dry-run", false, "Preview without writing files")
	apply := fs.Bool("apply", false, "Write workspace registry")
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
		fmt.Fprint(stderr, "exactly one of --dry-run or --apply is required for workspace repos sync\n\n")
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
	report, err := newWorkspaceRepoSyncReport(gira.WorkspaceRepoSyncInput{WorkspaceName: *workspaceName, Owner: *owner, ConfigRoot: *configRoot, Limit: *limit, IncludeArchived: *includeArchived, DryRun: *dryRun, Apply: *apply})
	if err != nil {
		if *jsonOutput {
			gira.EnsureWorkspaceReposSyncReportSchema(&report)
			if *dryRun && strings.TrimSpace(report.Command) != "" {
				report.Approval = gira.WorkspaceReposSyncApprovalEvidence(report)
			}
			out, _ := json.MarshalIndent(report, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
		}
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		gira.EnsureWorkspaceReposSyncReportSchema(&report)
		if *dryRun {
			report.Approval = gira.WorkspaceReposSyncApprovalEvidence(report)
		}
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode workspace repos sync JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatWorkspaceRepoSyncReport(report))
	return 0
}

func runWorkspaceInit(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("workspace init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	name := fs.String("name", "", "Workspace name")
	owner := fs.String("owner", "", "Workspace owner")
	inboxRepo := fs.String("inbox-repo", "", "Inbox repo in OWNER/REPO format")
	projectOwner := fs.String("project-owner", "", "GitHub Projects v2 owner; defaults to workspace owner")
	projectTitle := fs.String("project-title", "", "GitHub Projects v2 title; defaults to workspace name")
	projectNumber := fs.Int("project-number", 0, "GitHub Projects v2 number for disambiguation")
	scope := fs.String("scope", "repo", "Workspace config scope: repo or global")
	configRoot := fs.String("config-root", "", "Override global config root for --scope global")
	pathValue := fs.String("path", ".", "Directory where .gira/config.yaml is written")
	merge := fs.Bool("merge", false, "Merge workspace fields into an existing repo-local config")
	overwrite := fs.Bool("overwrite", false, "Overwrite existing config")
	dryRun := fs.Bool("dry-run", false, "Preview without writing config")
	apply := fs.Bool("apply", false, "Write config")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	var repos repeatedStringFlag
	fs.Var(&repos, "repo", "Execution repo in OWNER/REPO format")
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
	if strings.TrimSpace(*inboxRepo) == "" {
		fmt.Fprint(stderr, "--inbox-repo is required\n\n")
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
	report, err := newWorkspaceInitReport(gira.WorkspaceInitInput{Name: *name, Owner: *owner, InboxRepo: *inboxRepo, Repos: repos, ProjectOwner: *projectOwner, ProjectTitle: *projectTitle, ProjectNumber: *projectNumber, Scope: *scope, ConfigRoot: *configRoot, Path: *pathValue, Merge: *merge, Overwrite: *overwrite, DryRun: *dryRun, Apply: *apply})
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
			fmt.Fprintf(stderr, "encode workspace init JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatWorkspaceInitReport(report))
	return 0
}

func runWorkspaceCapability(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("workspace capability", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", "", "Workspace config path")
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
	report, err := newWorkspaceCapabilityReport(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode workspace capability JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatWorkspaceCapabilityReport(report))
	return 0
}

func runWorkspaceValidate(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("workspace validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", "", "Workspace config path")
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
	report, err := newWorkspaceValidateReport(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode workspace validate JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatWorkspaceValidateReport(report))
	return 0
}

func runWorkspaceStatus(command string, args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("workspace "+command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", "", "Workspace config path")
	var repos repeatedStringFlag
	fs.Var(&repos, "repo", "Execution repo to include. Repeatable or comma-separated")
	limit := fs.Int("limit", 0, "Maximum execution repos to inspect")
	activeOnly := fs.Bool("active-only", false, "Show only execution repos with open work or active milestone")
	maxConcurrency := fs.Int("max-concurrency", 4, "Maximum concurrent repo status fetches")
	cacheTTL := fs.Duration("cache-ttl", 5*time.Minute, "Reuse recent per-repo status cache for this duration. Use 0 to disable")
	refresh := fs.Bool("refresh", false, "Ignore cached workspace status and fetch fresh data")
	cacheRoot := fs.String("cache-root", "", "Workspace status cache root")
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
	if *limit < 0 {
		fmt.Fprint(stderr, "--limit must be at least 0\n\n")
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
	if *maxConcurrency < 1 {
		fmt.Fprint(stderr, "--max-concurrency must be at least 1\n\n")
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
	if *cacheTTL < 0 {
		fmt.Fprint(stderr, "--cache-ttl must be non-negative\n\n")
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
	var selectedRepos []gira.RepoRef
	for _, raw := range repos {
		for _, value := range strings.Split(raw, ",") {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			repo, err := gira.ParseRepoRef(value)
			if err != nil {
				fmt.Fprintf(stderr, "--repo must be in OWNER/REPO format: %v\n\n", err)
				fmt.Fprint(stderr, workspaceHelp)
				return 2
			}
			selectedRepos = append(selectedRepos, repo)
		}
	}
	report, err := newWorkspaceStatusReportWithOptions(*configPath, gira.WorkspaceStatusOptions{
		Repos:          selectedRepos,
		Limit:          *limit,
		ActiveOnly:     *activeOnly,
		MaxConcurrency: *maxConcurrency,
		CacheTTL:       *cacheTTL,
		Refresh:        *refresh,
		CacheRoot:      *cacheRoot,
	})
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if command == "backlog" || command == "list" {
		nextConfig := report.ConfigPath
		if strings.TrimSpace(nextConfig) == "" {
			nextConfig = gira.DefaultInitConfigPath(".")
		}
		report.NextSteps = []string{"gira workspace status --config " + nextConfig}
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
	configPath := fs.String("config", "", "Workspace config path")
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
	configPath := fs.String("config", "", "Workspace config path")
	title := fs.String("title", "", "Ticket title")
	body := fs.String("body", "", "Ticket body")
	bodyFile := fs.String("body-file", "", "Read ticket body from file")
	repoValue := fs.String("repo", "", "Target execution repo")
	dryRun := fs.Bool("dry-run", false, "Create and route without mutation")
	apply := fs.Bool("apply", false, "Create and route")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	flagArgs, positionalArgs := splitWorkspaceTicketNewArgs(args)
	if err := fs.Parse(flagArgs); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, workspaceHelp)
		return 0
	}
	resolvedTitle := strings.TrimSpace(*title)
	if len(positionalArgs) > 0 {
		if resolvedTitle != "" {
			fmt.Fprintf(stderr, "use either positional title or --title, not both\n\n")
			fmt.Fprint(stderr, workspaceHelp)
			return 2
		}
		resolvedTitle = strings.TrimSpace(strings.Join(positionalArgs, " "))
	}
	route := strings.TrimSpace(*repoValue) != ""
	if !route && (*dryRun || *apply) {
		fmt.Fprintf(stderr, "--repo is required when using --dry-run or --apply for workspace ticket new\n\n")
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
	if route && *dryRun == *apply {
		fmt.Fprintf(stderr, "--repo requires exactly one of --dry-run or --apply for workspace ticket new\n\n")
		fmt.Fprint(stderr, workspaceHelp)
		return 2
	}
	var repo gira.RepoRef
	var err error
	if route {
		repo, err = gira.ParseRepoRef(*repoValue)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
	}
	resolvedBody, err := readTicketNewBody(*body, *bodyFile, os.Stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newWorkspaceTicketNewReport(*configPath, resolvedTitle, resolvedBody, repo, route, *dryRun)
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

func splitWorkspaceTicketNewArgs(args []string) ([]string, []string) {
	flagArgs := []string{}
	positionalArgs := []string{}
	valueFlags := map[string]struct{}{"--config": {}, "--title": {}, "--body": {}, "--body-file": {}, "--repo": {}}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if _, ok := valueFlags[arg]; ok {
			flagArgs = append(flagArgs, arg)
			if i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
			continue
		}
		if strings.HasPrefix(arg, "--config=") || strings.HasPrefix(arg, "--title=") || strings.HasPrefix(arg, "--body=") || strings.HasPrefix(arg, "--body-file=") || strings.HasPrefix(arg, "--repo=") {
			flagArgs = append(flagArgs, arg)
			continue
		}
		switch arg {
		case "--json", "--dry-run", "--apply", "--help", "-h":
			flagArgs = append(flagArgs, arg)
		default:
			if strings.HasPrefix(arg, "-") {
				flagArgs = append(flagArgs, arg)
			} else {
				positionalArgs = append(positionalArgs, arg)
			}
		}
	}
	return flagArgs, positionalArgs
}

func runWorkspaceTicketRoute(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("workspace ticket route", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", "", "Workspace config path")
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
		fmt.Fprint(stdout, workspaceProjectHelp)
		return 0
	}
	if args[0] == "adopt" {
		return runWorkspaceProjectAdopt(args[1:], stdout, stderr)
	}
	if args[0] != "plan" {
		fmt.Fprintf(stderr, "unknown workspace project command: %s\n\n", args[0])
		fmt.Fprint(stderr, workspaceProjectHelp)
		return 2
	}
	fs := flag.NewFlagSet("workspace project plan", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", "", "Workspace config path")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, workspaceProjectHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, workspaceProjectHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, workspaceProjectHelp)
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

func runWorkspaceProjectAdopt(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("workspace project adopt", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", gira.DefaultInitConfigPath("."), "Workspace config path")
	owner := fs.String("owner", "", "GitHub Project owner")
	title := fs.String("title", "", "GitHub Project title")
	number := fs.Int("number", 0, "GitHub Project number")
	dryRun := fs.Bool("dry-run", false, "Plan project adoption without mutation")
	apply := fs.Bool("apply", false, "Apply project adoption")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, workspaceProjectAdoptHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, workspaceProjectAdoptHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, workspaceProjectAdoptHelp)
		return 2
	}
	if strings.TrimSpace(*owner) == "" || (strings.TrimSpace(*title) == "") == (*number == 0) || *dryRun == *apply {
		fmt.Fprintf(stderr, "--owner, exactly one of --title or --number, and exactly one of --dry-run or --apply are required for workspace project adopt\n\n")
		fmt.Fprint(stderr, workspaceProjectAdoptHelp)
		return 2
	}
	report, err := newWorkspaceProjectAdoptReport(gira.WorkspaceProjectAdoptInput{
		ConfigPath: *configPath,
		Owner:      *owner,
		Title:      *title,
		Number:     *number,
		DryRun:     *dryRun,
		Apply:      *apply,
	})
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode workspace project adopt JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatWorkspaceProjectAdoptReport(report))
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
	configPath := fs.String("config", "", "Workspace config path")
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

func runAdopt(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, adoptHelp)
		return 0
	}
	if args[0] == "repo" {
		return runAdoptRepo(args[1:], stdout, stderr)
	}
	if args[0] != "issues" {
		fmt.Fprintf(stderr, "unknown adopt command: %s\n\n", args[0])
		fmt.Fprint(stderr, adoptHelp)
		return 2
	}
	fs := flag.NewFlagSet("adopt issues", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	milestone := fs.String("milestone", "", "Milestone title")
	state := fs.String("state", "open", "Issues to inspect: open or all")
	normalizeStatus := fs.Bool("normalize-status", false, "Remove active status labels from closed selected issues")
	dryRun := fs.Bool("dry-run", false, "Preview without mutation")
	apply := fs.Bool("apply", false, "Apply selected mappings")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	var issues repeatedStringFlag
	var issueSpecs repeatedStringFlag
	var labels repeatedStringFlag
	fs.Var(&issues, "issue", "Issue number to map")
	fs.Var(&issueSpecs, "issues", "Issue numbers or ranges to map")
	fs.Var(&labels, "label", "Label to add")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, adoptHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, adoptHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, adoptHelp)
		return 2
	}
	if *repoValue == "" {
		fmt.Fprint(stderr, "--repo is required\n\n")
		fmt.Fprint(stderr, adoptHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	allIssueSpecs := append([]string{}, issues...)
	allIssueSpecs = append(allIssueSpecs, issueSpecs...)
	issueNumbers, err := parseRepeatedIssueNumbers(allIssueSpecs)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newAdoptIssuesReport(gira.AdoptIssueInput{Repo: repo, Issues: issueNumbers, Milestone: *milestone, Labels: labels, State: *state, NormalizeStatus: *normalizeStatus, DryRun: *dryRun, Apply: *apply})
	if err != nil {
		if *jsonOutput {
			gira.EnsureAdoptIssuesReportSchema(&report)
			if *dryRun && strings.TrimSpace(report.Repo) != "" {
				report.Approval = gira.AdoptIssuesApprovalEvidence(report)
			}
			out, _ := json.MarshalIndent(report, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
		}
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if *jsonOutput {
		gira.EnsureAdoptIssuesReportSchema(&report)
		if *dryRun {
			report.Approval = gira.AdoptIssuesApprovalEvidence(report)
		}
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode adopt JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatAdoptIssuesReport(report))
	return 0
}

func runAdoptRepo(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("adopt repo", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	pathValue := fs.String("path", ".", "Local git workspace path")
	strategy := fs.String("strategy", "", "Adoption strategy: observe, merge, or normalize")
	yes := fs.Bool("yes", false, "Apply the recommended strategy")
	dryRun := fs.Bool("dry-run", false, "Preview without mutation")
	apply := fs.Bool("apply", false, "Apply selected repo adoption actions")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, adoptHelp)
		return 2
	}
	if *help {
		fmt.Fprint(stdout, adoptHelp)
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, adoptHelp)
		return 2
	}
	if *repoValue == "" {
		fmt.Fprint(stderr, "--repo is required\n\n")
		fmt.Fprint(stderr, adoptHelp)
		return 2
	}
	repo, err := gira.ParseRepoRef(*repoValue)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	report, err := newAdoptRepoReport(gira.AdoptRepoInput{Repo: repo, Path: *pathValue, Strategy: *strategy, Yes: *yes, DryRun: *dryRun, Apply: *apply})
	if err != nil {
		if *jsonOutput {
			gira.EnsureAdoptRepoReportSchema(&report)
			if *dryRun && strings.TrimSpace(report.Repo) != "" {
				report.Approval = gira.AdoptRepoApprovalEvidence(report)
			}
			out, _ := json.MarshalIndent(report, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
		}
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if *jsonOutput {
		gira.EnsureAdoptRepoReportSchema(&report)
		if *dryRun {
			report.Approval = gira.AdoptRepoApprovalEvidence(report)
		}
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode adopt repo JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatAdoptRepoReport(report))
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
	configPath := fs.String("config", "", "Workspace config path for workspace dashboard export")
	outputRoot := fs.String("output", "./out/dashboard", "Output root directory for artifacts")
	limit := fs.Int("limit", 0, "Maximum workspace execution repos to inspect")
	activeOnly := fs.Bool("active-only", false, "Include only active workspace repos")
	maxConcurrency := fs.Int("max-concurrency", 4, "Maximum concurrent workspace repo status fetches")
	cacheTTL := fs.Duration("cache-ttl", 5*time.Minute, "Reuse recent per-repo status cache for this duration. Use 0 to disable")
	refresh := fs.Bool("refresh", false, "Ignore cached workspace status and fetch fresh data")
	cacheRoot := fs.String("cache-root", "", "Workspace status cache root")
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
	if *repoValue == "" && *configPath == "" {
		fmt.Fprint(stderr, "--repo or --config is required\n\n")
		fmt.Fprint(stderr, exportHelp)
		return 2
	}
	if *limit < 0 {
		fmt.Fprint(stderr, "--limit must be at least 0\n\n")
		fmt.Fprint(stderr, exportHelp)
		return 2
	}
	if *maxConcurrency < 1 {
		fmt.Fprint(stderr, "--max-concurrency must be at least 1\n\n")
		fmt.Fprint(stderr, exportHelp)
		return 2
	}
	if *cacheTTL < 0 {
		fmt.Fprint(stderr, "--cache-ttl must be non-negative\n\n")
		fmt.Fprint(stderr, exportHelp)
		return 2
	}

	if !*dryRun {
		if outputInfo, err := os.Stat(*outputRoot); err == nil && !outputInfo.IsDir() {
			fmt.Fprintf(stderr, "output path exists but is not a directory: %s\n", *outputRoot)
			return 2
		}
	}

	var plan gira.DashboardExportPlan
	var bundle gira.DashboardExportBundle
	var err error
	if *configPath != "" {
		var selectedRepos []gira.RepoRef
		if strings.TrimSpace(*repoValue) != "" {
			repo, parseErr := gira.ParseRepoRef(*repoValue)
			if parseErr != nil {
				fmt.Fprintf(stderr, "--repo must be in OWNER/REPO format: %v\n", parseErr)
				return 2
			}
			selectedRepos = []gira.RepoRef{repo}
		}
		plan, bundle, err = newWorkspaceDashboardExportBundle(*configPath, *outputRoot, dashboardExportNow(), *dryRun, gira.WorkspaceStatusOptions{
			Repos:          selectedRepos,
			Limit:          *limit,
			ActiveOnly:     *activeOnly,
			MaxConcurrency: *maxConcurrency,
			CacheTTL:       *cacheTTL,
			Refresh:        *refresh,
			CacheRoot:      *cacheRoot,
		})
	} else {
		repo, parseErr := gira.ParseRepoRef(*repoValue)
		if parseErr != nil {
			fmt.Fprintf(stderr, "%v\n", parseErr)
			return 2
		}
		client := newDashboardExportClient(repo)
		plan, bundle, err = gira.BuildDashboardExportPlan(repo, *outputRoot, dashboardExportNow(), *dryRun, client)
	}
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
	return runSyncWithCommand(args, stdout, stderr, "gira ops sync")
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
	allRepos := fs.Bool("all", false, "Summarize all execution repos from workspace config")
	owner := fs.String("owner", "", "Discover repositories owned by a user or organization")
	configPath := fs.String("config", ".gira/config.yaml", "Workspace config path for --all")
	limit := fs.Int("limit", 50, "Maximum repositories to inspect for --owner")
	includeArchived := fs.Bool("include-archived", false, "Include archived repositories in --owner discovery")
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
	if *limit < 1 {
		fmt.Fprint(stderr, "--limit must be at least 1\n\n")
		fmt.Fprint(stderr, statusHelp)
		return 2
	}
	multiModes := 0
	if *allRepos {
		multiModes++
	}
	if strings.TrimSpace(*owner) != "" {
		multiModes++
	}
	if strings.TrimSpace(*repoValue) != "" {
		multiModes++
	}
	if multiModes > 1 {
		fmt.Fprint(stderr, "choose only one of --repo, --all, or --owner\n\n")
		fmt.Fprint(stderr, statusHelp)
		return 2
	}
	if *allRepos {
		config, err := gira.ResolveWorkspaceConfig(*configPath)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
		return runGlobalStatus("workspace "+config.Name, config.Repos, *jsonOutput, *staleDays, stdout, stderr)
	}
	if strings.TrimSpace(*owner) != "" {
		repos, err := listStatusReposForOwner(*owner, *limit, *includeArchived)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
		return runGlobalStatus("owner "+strings.TrimSpace(*owner), repos, *jsonOutput, *staleDays, stdout, stderr)
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

func runGlobalStatus(scope string, repos []gira.RepoRef, jsonOutput bool, staleDays int, stdout io.Writer, stderr io.Writer) int {
	if len(repos) == 0 {
		fmt.Fprint(stderr, "no repositories found for status summary\n")
		return 2
	}
	report := gira.BuildGlobalStatusReport(scope, repos, newStatusClient, statusNow(), staleDays)
	if jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode status JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatGlobalStatusText(report))
	return 0
}

func ghStatusReposForOwner(owner string, limit int, includeArchived bool, runner gira.CommandRunner) ([]gira.RepoRef, error) {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return nil, fmt.Errorf("--owner is required")
	}
	if runner == nil {
		runner = gira.ExecCommandRunner{}
	}
	repos, err := ghStatusRepoList(owner, limit, false, runner)
	if err != nil {
		return nil, err
	}
	if includeArchived {
		archived, err := ghStatusRepoList(owner, limit, true, runner)
		if err != nil {
			return nil, err
		}
		repos = append(repos, archived...)
	}
	return uniqueSortedRepos(repos, limit), nil
}

func ghStatusRepoList(owner string, limit int, archived bool, runner gira.CommandRunner) ([]gira.RepoRef, error) {
	args := []string{"repo", "list", owner, "--limit", strconv.Itoa(limit), "--json", "nameWithOwner,isArchived"}
	if archived {
		args = append(args, "--archived")
	} else {
		args = append(args, "--no-archived")
	}
	output, err := runner.Run("gh", args...)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		NameWithOwner string `json:"nameWithOwner"`
		IsArchived    bool   `json:"isArchived"`
	}
	if err := json.Unmarshal(output, &rows); err != nil {
		return nil, fmt.Errorf("parse gh repo list JSON: %w", err)
	}
	repos := make([]gira.RepoRef, 0, len(rows))
	for _, row := range rows {
		if archived && !row.IsArchived {
			continue
		}
		if !archived && row.IsArchived {
			continue
		}
		repo, err := gira.ParseRepoRef(row.NameWithOwner)
		if err != nil {
			return nil, fmt.Errorf("parse repo %q: %w", row.NameWithOwner, err)
		}
		repos = append(repos, repo)
	}
	return repos, nil
}

func uniqueSortedRepos(repos []gira.RepoRef, limit int) []gira.RepoRef {
	seen := map[string]gira.RepoRef{}
	for _, repo := range repos {
		seen[strings.ToLower(repo.FullName())] = repo
	}
	out := make([]gira.RepoRef, 0, len(seen))
	for _, repo := range seen {
		out = append(out, repo)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].FullName()) < strings.ToLower(out[j].FullName())
	})
	if limit > 0 && len(out) > limit {
		return out[:limit]
	}
	return out
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
		gira.EnsureSprintPlanReportSchema(&report)
		if report.Mode == "dry-run" {
			report.Approval = gira.SprintPlanApprovalEvidence(report)
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
		gira.EnsureSprintStartReportSchema(&report)
		if report.Mode == "dry-run" {
			report.Approval = gira.SprintStartApprovalEvidence(report)
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
		gira.EnsureSprintCloseReportSchema(&report)
		if report.Mode == "dry-run" {
			report.Approval = gira.SprintCloseApprovalEvidence(report)
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
		report, err := newSprintRolloverReport(repo, *toMilestone, *apply)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		gira.EnsureSprintRolloverReportSchema(&report)
		if report.Mode == "dry-run" {
			report.Approval = gira.SprintRolloverApprovalEvidence(report)
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

func runMilestone(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, milestoneHelp)
		return 0
	}
	switch args[0] {
	case "new":
		positional, flagArgs := splitLeadingMilestoneTitle(args[1:])
		fs := flag.NewFlagSet("milestone new", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
		titleFlag := fs.String("title", "", "Milestone title")
		description := fs.String("description", "", "Milestone description")
		dueOn := fs.String("due-on", "", "Milestone due date or timestamp")
		dryRun := fs.Bool("dry-run", false, "Preview milestone create")
		apply := fs.Bool("apply", false, "Create milestone")
		jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
		if err := fs.Parse(flagArgs); err != nil {
			fmt.Fprintf(stderr, "%v\n\n", err)
			fmt.Fprint(stderr, milestoneHelp)
			return 2
		}
		title := strings.TrimSpace(*titleFlag)
		if title == "" && positional != "" {
			title = positional
		}
		if title == "" && fs.NArg() == 1 {
			title = fs.Arg(0)
		}
		if fs.NArg() > 1 || (fs.NArg() == 1 && title != fs.Arg(0) && positional != "") {
			fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
			fmt.Fprint(stderr, milestoneHelp)
			return 2
		}
		if title == "" || (*dryRun == *apply) {
			fmt.Fprint(stderr, "title and exactly one of --dry-run/--apply are required\n\n")
			fmt.Fprint(stderr, milestoneHelp)
			return 2
		}
		repo, ok := resolveRepoContext(*repoValue, stderr, milestoneHelp)
		if !ok {
			return 2
		}
		report, err := newMilestoneNewReport(gira.MilestoneNewInput{Repo: repo, Title: title, Description: *description, DueOn: *dueOn, DryRun: *dryRun, Apply: *apply})
		return writeMilestoneReport(report, err, *jsonOutput, stdout, stderr)
	case "list":
		fs := flag.NewFlagSet("milestone list", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
		state := fs.String("state", "open", "Milestone state: open, closed, or all")
		jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			fmt.Fprintf(stderr, "%v\n\n", err)
			fmt.Fprint(stderr, milestoneHelp)
			return 2
		}
		if fs.NArg() > 0 {
			fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
			fmt.Fprint(stderr, milestoneHelp)
			return 2
		}
		repo, ok := resolveRepoContext(*repoValue, stderr, milestoneHelp)
		if !ok {
			return 2
		}
		report, err := newMilestoneListReport(gira.MilestoneListOptions{Repo: repo, State: *state})
		return writeMilestoneReport(report, err, *jsonOutput, stdout, stderr)
	case "status":
		positional, flagArgs := splitLeadingMilestoneTitle(args[1:])
		fs := flag.NewFlagSet("milestone status", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
		jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
		if err := fs.Parse(flagArgs); err != nil {
			fmt.Fprintf(stderr, "%v\n\n", err)
			fmt.Fprint(stderr, milestoneHelp)
			return 2
		}
		if positional == "" && fs.NArg() == 1 {
			positional = fs.Arg(0)
		}
		if positional == "" || fs.NArg() > 1 {
			fmt.Fprint(stderr, "cannot determine milestone title\nMissing: positional milestone title\nTry: gira milestone list --repo OWNER/REPO\nThen: gira milestone status \"MILESTONE\" --repo OWNER/REPO\n\n")
			fmt.Fprint(stderr, milestoneHelp)
			return 2
		}
		repo, ok := resolveRepoContext(*repoValue, stderr, milestoneHelp)
		if !ok {
			return 2
		}
		report, err := newMilestoneStatusReport(gira.MilestoneStatusOptions{Repo: repo, Milestone: positional})
		return writeMilestoneReport(report, err, *jsonOutput, stdout, stderr)
	case "assign":
		positional, flagArgs := splitLeadingMilestoneTitle(args[1:])
		fs := flag.NewFlagSet("milestone assign", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
		ticketsValue := fs.String("tickets", "", "Comma-separated ticket numbers")
		dryRun := fs.Bool("dry-run", false, "Preview assignment")
		apply := fs.Bool("apply", false, "Assign selected tickets")
		jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
		if err := fs.Parse(flagArgs); err != nil {
			fmt.Fprintf(stderr, "%v\n\n", err)
			fmt.Fprint(stderr, milestoneHelp)
			return 2
		}
		if positional == "" && fs.NArg() == 1 {
			positional = fs.Arg(0)
		}
		if positional == "" || fs.NArg() > 1 || *ticketsValue == "" || (*dryRun == *apply) {
			fmt.Fprint(stderr, "milestone, --tickets, and exactly one of --dry-run/--apply are required\n\n")
			fmt.Fprint(stderr, milestoneHelp)
			return 2
		}
		repo, ok := resolveRepoContext(*repoValue, stderr, milestoneHelp)
		if !ok {
			return 2
		}
		tickets, err := parseCSVInts(*ticketsValue)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
		report, err := newMilestoneAssignReport(gira.MilestoneAssignInput{Repo: repo, Milestone: positional, Tickets: tickets, DryRun: *dryRun, Apply: *apply})
		return writeMilestoneReport(report, err, *jsonOutput, stdout, stderr)
	case "plan":
		positional, flagArgs := splitLeadingMilestoneTitle(args[1:])
		fs := flag.NewFlagSet("milestone plan", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
		state := fs.String("state", "open", "Ticket state: open, closed, or all")
		limit := fs.Int("limit", 20, "Maximum candidate tickets")
		dryRun := fs.Bool("dry-run", false, "Preview assignment plan")
		apply := fs.Bool("apply", false, "Assign selected tickets")
		jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
		var labels repeatedStringFlag
		fs.Var(&labels, "label", "Candidate label filter")
		if err := fs.Parse(flagArgs); err != nil {
			fmt.Fprintf(stderr, "%v\n\n", err)
			fmt.Fprint(stderr, milestoneHelp)
			return 2
		}
		if positional == "" && fs.NArg() == 1 {
			positional = fs.Arg(0)
		}
		if positional == "" || fs.NArg() > 1 || (*dryRun == *apply) {
			fmt.Fprint(stderr, "milestone and exactly one of --dry-run/--apply are required\n\n")
			fmt.Fprint(stderr, milestoneHelp)
			return 2
		}
		repo, ok := resolveRepoContext(*repoValue, stderr, milestoneHelp)
		if !ok {
			return 2
		}
		report, err := newMilestonePlanReport(gira.MilestonePlanInput{Repo: repo, Milestone: positional, Labels: labels, State: *state, Limit: *limit, DryRun: *dryRun, Apply: *apply})
		return writeMilestoneReport(report, err, *jsonOutput, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown milestone command: %s\n\n", args[0])
		fmt.Fprint(stderr, milestoneHelp)
		return 2
	}
}

func splitLeadingMilestoneTitle(args []string) (string, []string) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", args
	}
	return args[0], args[1:]
}

func writeMilestoneReport(report gira.MilestoneReport, err error, jsonOutput bool, stdout io.Writer, stderr io.Writer) int {
	if err != nil {
		if jsonOutput {
			gira.EnsureMilestoneReportSchema(&report)
			if report.DryRun && strings.TrimSpace(report.Repo) != "" && gira.MilestoneReportSupportsApproval(report) {
				report.Approval = gira.MilestoneApprovalEvidence(report)
			}
			out, _ := json.MarshalIndent(report, "", "  ")
			fmt.Fprintf(stdout, "%s\n", out)
		}
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if jsonOutput {
		gira.EnsureMilestoneReportSchema(&report)
		if report.DryRun && gira.MilestoneReportSupportsApproval(report) {
			report.Approval = gira.MilestoneApprovalEvidence(report)
		}
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode milestone JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatMilestoneReport(report))
	return 0
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
	claim := gira.WorkerClaim{Repo: repo.FullName(), IssueNumber: *issue, Worker: *worker, LeaseUntilUTC: time.Now().UTC().Add(time.Duration(*leaseMinutes) * time.Minute), Version: gira.WorkerStateHandoffSchemaVersion}
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
	payload := gira.WorkerHandoffPayload{SchemaVersion: gira.WorkerStateHandoffSchemaVersion, Goal: *goal, Context: *context, AcceptanceCriteria: splitList(*acceptance), VerificationCommands: splitList(*verify), RollbackNotes: *rollback}
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

func parseQueueRepoFilters(values repeatedStringFlag) ([]gira.RepoRef, []string, error) {
	selected := []gira.RepoRef{}
	filters := []string{}
	for _, raw := range values {
		for _, value := range strings.Split(raw, ",") {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			repo, err := gira.ParseRepoRef(value)
			if err != nil {
				return nil, nil, fmt.Errorf("--repo must be in OWNER/REPO format: %w", err)
			}
			selected = append(selected, repo)
			filters = append(filters, repo.FullName())
		}
	}
	return selected, filters, nil
}

func queueHandoffReport(workspaceReport gira.WorkspaceReport, repoFilters []string, ticket int, role string, profile string) (gira.QueueHandoffReport, error) {
	if ticket > 0 {
		return queueExplicitHandoffReport(workspaceReport, repoFilters, ticket, role, profile)
	}
	next, err := gira.BuildQueueNextReport(workspaceReport, gira.QueueNextOptions{RepoFilters: repoFilters, Role: role, Profile: profile})
	if err != nil {
		return gira.QueueHandoffReport{}, err
	}
	if next.Selected == nil {
		return gira.BuildQueueHandoffReportFromNext(next, nil, role, profile), nil
	}
	repo, err := gira.ParseRepoRef(next.Selected.Repo)
	if err != nil {
		return gira.BuildQueueHandoffReportFromNext(next, nil, role, profile), err
	}
	handoff, err := newTicketHandoffReport(gira.TicketHandoffInput{
		Repo:    repo,
		Ticket:  next.Selected.Issue,
		Role:    role,
		Profile: profile,
	})
	report := gira.BuildQueueHandoffReportFromNext(next, &handoff, role, profile)
	return report, err
}

func queueExplicitHandoffReport(workspaceReport gira.WorkspaceReport, repoFilters []string, ticket int, role string, profile string) (gira.QueueHandoffReport, error) {
	filters := gira.QueueFilterSummary{Queues: gira.WorkspaceQueueOrder(), Repos: append([]string(nil), repoFilters...)}
	nextStep := "gira queue list"
	if strings.TrimSpace(workspaceReport.ConfigPath) != "" {
		nextStep += " --config " + gira.QuoteShellArg(workspaceReport.ConfigPath)
	}
	for _, repo := range repoFilters {
		nextStep += " --repo " + gira.QuoteShellArg(repo)
	}
	item, ok := gira.FindWorkspaceQueueItem(workspaceReport.Queues, repoFilters[0], ticket)
	if !ok {
		return gira.BuildQueueHandoffStopReport(workspaceReport, filters, role, profile, []string{"ticket_not_in_workspace_queue"}, nextStep), nil
	}
	if !gira.WorkspaceQueueItemHandoffSafe(item) {
		step := item.NextSafeCommand
		if strings.TrimSpace(step) == "" {
			step = nextStep
		}
		return gira.BuildQueueHandoffStopReport(workspaceReport, filters, role, profile, gira.QueueHandoffStopReasonsForItem(item), step), nil
	}
	selection := gira.QueueNextSelectionFromItem(item, role, profile)
	next := gira.QueueNextReport{
		SchemaVersion:  gira.QueueNextSchemaVersion,
		Command:        "queue next",
		Workspace:      workspaceReport.Workspace,
		SourceContract: gira.WorkspaceQueuesSchemaVersion,
		ConfigPath:     workspaceReport.ConfigPath,
		Filters:        filters,
		Counts:         workspaceReport.Queues.Counts,
		Selected:       &selection,
		NextAction:     "handoff_ticket",
		NextStep:       selection.HandoffCommand,
		Warnings:       append([]string(nil), workspaceReport.Warnings...),
		FetchedAt:      workspaceReport.FetchedAt,
	}
	repo, err := gira.ParseRepoRef(item.Repo)
	if err != nil {
		return gira.BuildQueueHandoffReportFromNext(next, nil, role, profile), err
	}
	handoff, err := newTicketHandoffReport(gira.TicketHandoffInput{
		Repo:    repo,
		Ticket:  item.Issue,
		Role:    role,
		Profile: profile,
	})
	report := gira.BuildQueueHandoffReportFromNext(next, &handoff, role, profile)
	return report, err
}

func queueTakeReport(workspaceReport gira.WorkspaceReport, repoFilters []string, ticket int, role string, profile string, dryRun bool, apply bool) (gira.QueueTakeReport, error) {
	handoff, err := queueHandoffReport(workspaceReport, repoFilters, ticket, role, profile)
	if err != nil {
		return gira.BuildQueueTakeReport(handoff, nil, dryRun, apply), err
	}
	if handoff.Selected == nil || len(handoff.StopReasons) > 0 || handoff.WorkerHandoff == nil {
		return gira.BuildQueueTakeReport(handoff, nil, dryRun, apply), nil
	}
	repo, err := gira.ParseRepoRef(handoff.Selected.Repo)
	if err != nil {
		return gira.BuildQueueTakeReport(handoff, nil, dryRun, apply), err
	}
	start, err := newWorkStartResultWithOptions(repo, handoff.Selected.Issue, gira.WorkStartOptions{DryRun: dryRun})
	report := gira.BuildQueueTakeReport(handoff, &start, dryRun, apply)
	return report, err
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
	switch args[0] {
	case "readiness":
		return runAuditReadiness(args[1:], stdout, stderr)
	case "drift":
		return runAuditWorkflow(args[1:], stdout, stderr)
	case "workflow":
		return runAuditWorkflow(args[1:], stdout, stderr)
	case "verify":
		return runAuditVerify(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown audit command: %s\n\n", args[0])
		fmt.Fprint(stderr, auditHelp)
		return 2
	}
}

func runAuditReadiness(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("audit readiness", flag.ContinueOnError)
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
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, auditHelp)
		return 2
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
	report := newAuditReadinessReport(repo, *ledgerPath)
	if *jsonOutput {
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode audit readiness JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
	} else {
		fmt.Fprint(stdout, gira.FormatAuditReadinessReport(report))
	}
	if !report.Ready {
		return 1
	}
	return 0
}

func runAuditWorkflow(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("audit workflow", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
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
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", fs.Arg(0))
		fmt.Fprint(stderr, auditHelp)
		return 2
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
	report, err := newAuditWorkflowReport(repo)
	if err != nil {
		fmt.Fprintf(stderr, "audit workflow failed: %v\n", err)
		return 1
	}
	if *jsonOutput {
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode audit workflow JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
	} else {
		fmt.Fprint(stdout, gira.FormatWorkflowAuditReport(report))
	}
	if !report.Ready {
		return 1
	}
	return 0
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
		localExec := fs.Bool("local-exec", false, "Run local repository checks in the current trusted checkout")
		if err := fs.Parse(args[1:]); err != nil {
			fmt.Fprintf(stderr, "%v\n\n", err)
			fmt.Fprint(stderr, reviewHelp)
			return 2
		}
		report := gira.RunStaticQualityGate()
		if *localExec {
			report = gira.RunQualityGate(reviewGateRunner)
		}
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
labels                gira ops sync --repo OWNER/REPO                  gira ops sync --repo OWNER/REPO --dry-run                                                gira ops sync --repo OWNER/REPO                                   unsupported (intentional in MVP)
milestones            gira ops sync --repo OWNER/REPO                  gira ops sync --repo OWNER/REPO --dry-run                                                gira ops sync --repo OWNER/REPO                                   unsupported (intentional in MVP)
issues                gira ops sync --repo OWNER/REPO --bootstrap-issues gira status --repo OWNER/REPO                                                          gira triage apply --apply / gira worker claim|handoff|release     unsupported direct delete in MVP
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

func runStats(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, statsHelp)
		return 0
	}
	switch args[0] {
	case "repo":
		return runStatsRepo(args[1:], stdout, stderr)
	case "workspace":
		fmt.Fprint(stdout, "gira stats workspace is planned; use gira stats repo --repo OWNER/REPO --since 90d for the first Closure Funnel report.\n")
		return 0
	default:
		fmt.Fprintf(stderr, "unknown stats command: %s\n\n", args[0])
		fmt.Fprint(stderr, statsHelp)
		return 2
	}
}

func runStatsRepo(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("stats repo", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoValue := fs.String("repo", "", "Target GitHub repo in OWNER/REPO format")
	since := fs.String("since", "90d", "Reporting window such as 90d or YYYY-MM-DD")
	staleDays := fs.Int("stale-days", 14, "Count open issues/PRs stale after this many days")
	limit := fs.Int("limit", 100, "Max GitHub rows per query")
	jsonOutput := fs.Bool("json", false, "Emit stable JSON output")
	positionalRepo := ""
	parseArgs := args
	if len(parseArgs) > 0 && !strings.HasPrefix(parseArgs[0], "-") {
		positionalRepo = parseArgs[0]
		parseArgs = parseArgs[1:]
	}
	if err := fs.Parse(parseArgs); err != nil {
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, statsHelp)
		return 2
	}
	remaining := fs.Args()
	if len(remaining) > 1 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n\n", remaining[1])
		fmt.Fprint(stderr, statsHelp)
		return 2
	}
	if len(remaining) == 1 {
		if positionalRepo != "" {
			fmt.Fprintf(stderr, "repo specified more than once\n\n")
			fmt.Fprint(stderr, statsHelp)
			return 2
		}
		positionalRepo = remaining[0]
	}
	if positionalRepo != "" {
		if strings.TrimSpace(*repoValue) != "" {
			fmt.Fprintf(stderr, "repo specified both positionally and with --repo\n\n")
			fmt.Fprint(stderr, statsHelp)
			return 2
		}
		*repoValue = positionalRepo
	}
	repo, ok := resolveRepoContext(*repoValue, stderr, statsHelp)
	if !ok {
		return 2
	}
	report, err := newStatsRepoReport(gira.StatsRepoOptions{
		Repo:      repo,
		Since:     *since,
		StaleDays: *staleDays,
		Limit:     *limit,
	})
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	if *jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode stats JSON: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", output)
		return 0
	}
	fmt.Fprint(stdout, gira.FormatStatsRepoReport(report))
	return 0
}
