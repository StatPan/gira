# Command Reference

This page is generated from Gira's command metadata registry. Update `internal/gira/command_registry.go` first, then refresh this page.

## `jira doctor`

Diagnose Jira-primary provider compatibility without mutating Jira or GitHub.

Usage:

```bash
gira jira doctor --repo OWNER/REPO [--project KEY] [--api-base URL] [--sample-key JIRA-123] [--config-root PATH] [--json]
```

Since: `v1.13.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--project`: Override the configured Jira project key for diagnostics.
- `--api-base`: Override the configured Jira API base URL.
- `--sample-key`: Representative Jira issue key for transition and required-field diagnostics.
- `--config-root`: Override the global Gira config root.
- `--json`: Emit stable JSON.

Examples:

- Diagnose a configured Jira-primary repo

```bash
gira jira doctor --repo OWNER/app --sample-key ABC-123
```

Documented in: `README.md`, `docs/jira-primary-provider.md`, `docs-site/jira-primary-provider.md`

## `jira export`

Export GitHub issue state into Jira-friendly JSON and CSV artifacts.

Usage:

```bash
gira jira export --repo OWNER/REPO --output PATH [--json]
```

Since: `v1.13.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--output`: Output directory for export artifacts.
- `--json`: Emit stable JSON.

Examples:

- Export GitHub issue state

```bash
gira jira export --repo OWNER/app --output out/jira
```

Documented in: `README.md`, `docs/jira-primary-provider.md`, `docs-site/jira-primary-provider.md`

## `jira import`

Import Jira CSV/JSON or read-only Jira API issues into GitHub issues.

Usage:

```bash
gira jira import --repo OWNER/REPO --source PATH --dry-run|--apply [--json]
gira jira import --repo OWNER/REPO --api-base URL --project KEY --dry-run|--apply [--json]
```

Since: `v1.13.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--source`: CSV or JSON import source path.
- `--api-base`: Jira API base URL for read-only API import.
- `--project`: Jira project key for read-only API import.
- `--dry-run`: Preview issue creates without mutation.
- `--apply`: Create GitHub issues for non-duplicate Jira items.
- `--json`: Emit stable JSON.

Examples:

- Preview a Jira CSV import

```bash
gira jira import --repo OWNER/app --source jira.csv --dry-run
```

Documented in: `README.md`, `docs/jira-primary-provider.md`, `docs-site/jira-primary-provider.md`

## `jira init`

Discover a Jira project and write reviewed non-secret provider config.

Usage:

```bash
gira jira init --repo OWNER/REPO --api-base URL --project KEY --dry-run|--apply [--config-root PATH] [--overwrite] [--json]
```

Since: `v1.13.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--api-base`: Jira site base URL, such as https://example.atlassian.net.
- `--project`: Jira project key to discover.
- `--config-root`: Override the global Gira config root.
- `--overwrite`: Replace an existing providers.jira block after review.
- `--dry-run`: Preview provider discovery and config payload without writing files.
- `--apply`: Write the reviewed non-secret provider config.
- `--json`: Emit stable JSON.

Examples:

- Preview Jira provider setup

```bash
gira jira init --repo OWNER/app --api-base https://example.atlassian.net --project ABC --dry-run
```

Documented in: `README.md`, `docs/jira-primary-provider.md`, `docs-site/jira-primary-provider.md`

## `jira mirror`

Create or reuse a GitHub mirror issue for one Jira key.

Usage:

```bash
gira jira mirror JIRA-123 --repo OWNER/REPO --dry-run|--apply [--api-base URL] [--config-root PATH] [--json]
```

Since: `v1.13.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--api-base`: Override the configured Jira API base URL.
- `--config-root`: Override the global Gira config root.
- `--dry-run`: Preview mirror issue creation or reuse.
- `--apply`: Create the GitHub mirror issue when missing.
- `--json`: Emit stable JSON.

Examples:

- Preview one Jira mirror

```bash
gira jira mirror ABC-123 --repo OWNER/app --dry-run
```

Documented in: `README.md`, `docs/jira-primary-provider.md`, `docs-site/jira-primary-provider.md`

## `jira transition`

Plan one Jira status transition without mutation.

Usage:

```bash
gira jira transition JIRA-123 --repo OWNER/REPO --to ready|in_progress|review|done --dry-run [--api-base URL] [--config-root PATH] [--json]
```

Since: `v1.13.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--to`: Target Gira status mapped through providers.jira.status_map.
- `--api-base`: Override the configured Jira API base URL.
- `--config-root`: Override the global Gira config root.
- `--dry-run`: Required; transition planning is read-only.
- `--json`: Emit stable JSON.

Examples:

- Inspect whether Done is reachable

```bash
gira jira transition ABC-123 --repo OWNER/app --to done --dry-run
```

Documented in: `README.md`, `docs/jira-primary-provider.md`, `docs-site/jira-primary-provider.md`

## `setup global`

Create or update the OS-user global config, workspace registry, and repo registry.

Usage:

```bash
gira setup global [--repo OWNER/REPO] [--path .] [--workspace NAME] [--inbox-repo OWNER/REPO] [--mode global-only|hybrid] --dry-run|--apply
```

Since: `v1.7.0`

Flags:

- `--repo`: Initial execution repo.
- `--inbox-repo`: Backlog/intake repo for unassigned work.
- `--mode`: Use global-only or hybrid repo-local contract mode.

Examples:

- Preview global-first setup

```bash
gira setup global --repo OWNER/app --path . --workspace personal --inbox-repo OWNER/backlog --mode global-only --dry-run
```

Documented in: `README.md`, `docs/global-config-registry.md`, `docs-site/global-config.md`, `docs/workspace.md`

## `stats repo`

Show a read-only Closure Funnel report for one GitHub repo.

Usage:

```bash
gira stats repo [OWNER/REPO] [--repo OWNER/REPO] [--since 90d] [--stale-days 14] [--limit 100] [--json]
```

Since: `v1.12.0`

Flags:

- `--repo`: Target GitHub repo. May also be positional.
- `--since`: Reporting window such as 90d or YYYY-MM-DD. Default: 90d.
- `--stale-days`: Count open issues and PRs stale after this many days. Default: 14.
- `--limit`: Max GitHub rows per query. Default: 100.
- `--json`: Emit stable JSON for automation.

Examples:

- Render the default repo report

```bash
gira stats repo --repo OWNER/app --since 90d
```

Documented in: `README.md`, `docs/closure-funnel-stats.md`, `docs-site/closure-funnel-stats.md`

## `stats workspace`

Planned multi-repo Closure Funnel rollup for a configured workspace.

Usage:

```bash
gira stats workspace [--since 90d]
```

Since: `planned`

Flags:

- `--since`: Reporting window such as 90d or YYYY-MM-DD.

Examples:

- Planned workspace rollup

```bash
gira stats workspace --since 90d
```

Documented in: `docs/closure-funnel-stats.md`, `docs-site/closure-funnel-stats.md`

## `ticket checks`

Show linked PR checks, review blockers, and next action.

Usage:

```bash
gira ticket checks [TICKET] [--repo OWNER/REPO] [--json]
```

Since: `v1.0.0`

Examples:

- Inspect PR readiness

```bash
gira ticket checks
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `ticket finish`

Merge the linked PR when policy allows, sync main, and close the ticket loop.

Usage:

```bash
gira ticket finish [TICKET] --dry-run|--apply [--repo OWNER/REPO]
```

Since: `v1.0.0`

Examples:

- Preview finish

```bash
gira ticket finish --dry-run
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `ticket new`

Create a repo-bound executable GitHub issue with structured or full Markdown body input.

Usage:

```bash
gira ticket new "Title" --dry-run|--apply [--body TEXT|--body-file PATH|-] [--start]
```

Since: `v1.0.0`

Flags:

- `--goal`: Structured issue goal.
- `--acceptance`: Semicolon-separated acceptance criteria.
- `--body`: Full issue body.
- `--body-file`: Read full issue body from file or stdin with -.
- `--start`: Start the created ticket after apply.

Examples:

- Preview structured ticket

```bash
gira ticket new "TITLE" --goal "GOAL" --acceptance "a;b;c" --dry-run
```

- Preview full Markdown packet

```bash
gira ticket new --title "TITLE" --body-file issue.md --dry-run
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `ticket note`

Post a structured context note to the issue, linked PR, or both.

Usage:

```bash
gira ticket note [TICKET] "BODY" --dry-run|--apply [--repo OWNER/REPO] [--kind progress|blocker|decision|handoff|summary|check] [--target auto|issue|pr|both]
```

Since: `v1.12.0`

Flags:

- `--kind`: Template kind for the note. Default: progress.
- `--target`: Comment target: auto, issue, pr, or both. Default: auto.
- `--body`: Explicit note body.
- `--body-file`: Read note body from file or stdin with -.
- `--dry-run`: Preview target resolution and rendered note without posting.
- `--apply`: Post the rendered note.

Examples:

- Preview a progress note

```bash
gira ticket note "Implemented parser path" --dry-run
```

- Post a blocker to both issue and PR

```bash
gira ticket note --kind blocker --target both --body-file note.md --apply
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `ticket pr`

Create or validate a linked PR with required issue closing text.

Usage:

```bash
gira ticket pr [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--draft]
```

Since: `v1.0.0`

Examples:

- Open a draft PR

```bash
gira ticket pr --apply --draft
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `ticket prompt`

Render a stateless planner, implementer, or reviewer prompt from ticket context.

Usage:

```bash
gira ticket prompt [TICKET] --role planner|implementer|reviewer [--profile default|python] [--repo OWNER/REPO] [--pr N] [--json]
```

Since: `v1.14.0`

Flags:

- `--role`: Prompt role: planner, implementer, or reviewer.
- `--profile`: Prompt profile: default or python. Default: default.
- `--pr`: Optional PR number for reviewer prompt context.
- `--json`: Emit stable JSON including the rendered prompt.

Examples:

- Render an implementation worker prompt

```bash
gira ticket prompt 42 --role implementer --profile python
```

- Render a reviewer prompt with PR context

```bash
gira ticket prompt 42 --role reviewer --pr 77
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `ticket start`

Verify a ready issue, create or reuse its branch, and move it to in-progress.

Usage:

```bash
gira ticket start [TICKET] --dry-run|--apply [--repo OWNER/REPO]
```

Since: `v1.0.0`

Examples:

- Start an existing ready issue

```bash
gira ticket start 42 --apply
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `ticket status`

Report ticket status, linked PR blockers, and next action.

Usage:

```bash
gira ticket status [TICKET] [--repo OWNER/REPO] [--json]
```

Since: `v1.0.0`

Examples:

- Inspect current branch ticket

```bash
gira ticket status
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `ticket supersede`

Close a ticket as superseded and create a linked replacement ticket.

Usage:

```bash
gira ticket supersede [TICKET] --replacement-title TITLE --body-file PATH|- --dry-run|--apply [--repo OWNER/REPO] [--close-draft-pr]
```

Since: `v1.12.0`

Flags:

- `--replacement-title`: Title for the replacement issue.
- `--body`: Replacement issue body.
- `--body-file`: Read replacement issue body from file or stdin with -.
- `--label`: Additional replacement issue label.
- `--milestone`: Override replacement issue milestone.
- `--close-draft-pr`: Close a linked draft PR when superseding.
- `--dry-run`: Preview all planned mutations.
- `--apply`: Create the replacement, cross-link notes, status update, and close the original.

Examples:

- Preview a replacement ticket

```bash
gira ticket supersede 64 --replacement-title "Define release gate" --body-file replacement.md --dry-run
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `ticket view`

Show a Gira operating card for the ticket, linked PR, blockers, and next action.

Usage:

```bash
gira ticket view [TICKET] [--repo OWNER/REPO] [--json]
```

Since: `v1.12.0`

Examples:

- Inspect current branch ticket context

```bash
gira ticket view
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `ticket wait`

Wait for pending linked PR checks without merging.

Usage:

```bash
gira ticket wait [TICKET] [--repo OWNER/REPO] [--timeout 5m] [--interval 5s]
```

Since: `v1.0.0`

Examples:

- Wait for CI

```bash
gira ticket wait --timeout 5m
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `workspace repos sync`

Discover GitHub owner/org repos and update a global workspace execution repo allowlist.

Usage:

```bash
gira workspace repos sync [--owner OWNER] [--workspace NAME] --dry-run|--apply [--include-archived]
```

Since: `v1.8.0`

Flags:

- `--owner`: GitHub user or organization. Defaults to workspace.owner.
- `--workspace`: Global workspace name. Defaults to global config default_workspace.
- `--include-archived`: Include archived repositories.

Examples:

- Preview owner repo sync

```bash
gira workspace repos sync --owner OWNER --workspace personal --dry-run
```

Documented in: `docs/global-config-registry.md`, `docs-site/global-config.md`, `docs/workspace.md`

## `workspace status`

Show inbox and execution repo state from a workspace config or global workspace registry.

Usage:

```bash
gira workspace status [--config .gira/config.yaml] [--repo OWNER/REPO] [--limit N] [--active-only] [--cache-ttl 5m] [--refresh] [--json]
```

Since: `v1.0.0`

Flags:

- `--config`: Explicit workspace config path. Defaults to global registry, then .gira/config.yaml.
- `--repo`: Narrow status to one or more execution repos.
- `--limit`: Inspect only the first N selected execution repos.
- `--active-only`: Show only execution repos with open work or an active milestone.
- `--max-concurrency`: Bound concurrent repo status fetches. Default: 4.
- `--cache-ttl`: Reuse recent per-repo status cache for this duration. Default: 5m.
- `--refresh`: Ignore cached status and fetch fresh data.
- `--json`: Emit stable JSON.

Examples:

- Read the default workspace

```bash
gira workspace status
```

- Inspect a bounded subset

```bash
gira workspace status --limit 10 --active-only
```

Documented in: `README.md`, `docs/workspace.md`, `docs-site/global-config.md`

