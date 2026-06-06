# Command Reference

This page is generated from Gira's command metadata registry. Update `internal/gira/command_registry.go` first, then refresh this page.

## `completion`

Generate static shell completion scripts for common commands, subcommands, and flags.

Usage:

```bash
gira completion bash|zsh|fish
```

Since: `v2.1.0`

Flags:

- `bash`: Print Bash completion script.
- `zsh`: Print Zsh completion script.
- `fish`: Print Fish completion script.

Examples:

- Install Bash completion locally

```bash
gira completion bash > ~/.local/share/bash-completion/completions/gira
```

- Preview Fish completion

```bash
gira completion fish
```

Documented in: `README.md`, `docs-site/command-reference.md`

## `feature check`

Validate optional feature map records and work links without mutating GitHub.

Usage:

```bash
gira feature check [--repo OWNER/REPO] [--limit N] [--json]
```

Since: `v1.17.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--limit`: Max issues to inspect. Default: 1000.
- `--json`: Emit stable feature-map-check/v1 JSON.

Examples:

- Check feature map health

```bash
gira feat check --repo OWNER/backlog
```

Documented in: `docs/feature-map.md`, `docs-site/feature-map.md`, `docs-site/command-reference.md`

## `feature for`

Show which feature or capability a work issue is linked to.

Usage:

```bash
gira feature for ISSUE [--repo OWNER/REPO] [--limit N] [--json]
```

Since: `v1.17.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--issue`: Work issue number. Can also be numeric positional.
- `--limit`: Max issues to inspect. Default: 1000.
- `--json`: Emit stable feature-map-for/v1 JSON.

Examples:

- Inspect one work issue

```bash
gira feat for 123 --repo OWNER/app
```

Documented in: `docs/feature-map.md`, `docs-site/feature-map.md`, `docs-site/command-reference.md`

## `feature list`

List optional issue-backed feature or capability records.

Usage:

```bash
gira feature list [--repo OWNER/REPO] [--limit N] [--json]
```

Since: `v1.17.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--limit`: Max issues to inspect. Default: 1000.
- `--json`: Emit stable feature-map-list/v1 JSON.

Examples:

- List feature records

```bash
gira feat list --repo OWNER/backlog
```

Documented in: `docs/feature-map.md`, `docs-site/feature-map.md`, `docs-site/command-reference.md`

## `goal finish`

Preview goal finish readiness, then post receipts and close ready goals or preserve human-review handoffs.

Usage:

```bash
gira goal finish [GOAL] --dry-run|--apply [--repo OWNER/REPO] [--terminal done|human_review|blocked|superseded|abandoned] [--json]
```

Since: `v1.17.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--goal`: Goal issue number. Can also be numeric positional.
- `--dry-run`: Preview readiness and receipt without mutation.
- `--apply`: Apply an explicit done close or human_review handoff mutation.
- `--terminal`: Explicit terminal recommendation override for apply: done, human_review, blocked, superseded, or abandoned.
- `--json`: Emit stable goal-finish-readiness/v1 JSON.

Examples:

- Preview goal finish evidence

```bash
gira goal finish 521 --repo OWNER/app --dry-run --json
```

Documented in: `docs/goal-operating-model.md`, `docs-site/command-reference.md`

## `goal next`

Select the next safe child ticket for a goal or explain why work must stop.

Usage:

```bash
gira goal next [GOAL] [--repo OWNER/REPO] [--json]
```

Since: `v1.17.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--goal`: Goal issue number. Can also be numeric positional.
- `--json`: Emit stable goal-next/v1 JSON.

Examples:

- Choose the next goal child

```bash
gira goal next 521 --repo OWNER/app --json
```

Documented in: `docs/goal-operating-model.md`, `docs-site/command-reference.md`

## `goal plan`

Propose or create same-repo or target-repo child ticket packets from a goal issue.

Usage:

```bash
gira goal plan [GOAL] --dry-run|--apply [--repo OWNER/REPO] [--json]
```

Since: `v1.17.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--goal`: Goal issue number. Can also be numeric positional.
- `--dry-run`: Preview proposed child tickets, including target_repo, without mutation.
- `--apply`: Create reviewed child tickets in their target repos from the proposed plan.
- `--json`: Emit stable goal-plan/v1 JSON.

Examples:

- Preview child ticket plan

```bash
gira goal plan 521 --repo OWNER/app --dry-run --json
```

- Create planned child tickets

```bash
gira goal plan 521 --repo OWNER/app --apply --json
```

Documented in: `docs/goal-operating-model.md`, `docs-site/command-reference.md`

## `goal report`

Build a visible report for one goal from stable Goal Mode state. Alias: gira goal dossier.

Usage:

```bash
gira goal report [GOAL] [--repo OWNER/REPO] [--json|--html --output PATH]
```

Since: `v2.1.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--goal`: Goal issue number. Can also be numeric positional.
- `--json`: Emit stable goal-dossier/v1 JSON.
- `--html`: Write a static local HTML report.
- `--output`: Output path for --html.

Examples:

- Export a goal report JSON contract

```bash
gira goal report 521 --repo OWNER/app --json
```

- Write a local goal report page

```bash
gira goal report 521 --repo OWNER/app --html --output out/gira/goal-521.html
```

Documented in: `docs/goal-operating-model.md`, `docs-site/goal-mode.md`, `docs-site/command-reference.md`

## `goal status`

Summarize a goal issue, child ticket graph, blockers, and next safe action.

Usage:

```bash
gira goal status [GOAL] [--repo OWNER/REPO] [--json]
```

Since: `v1.17.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--goal`: Goal issue number. Can also be numeric positional.
- `--json`: Emit stable goal-status/v1 JSON.

Examples:

- Inspect goal graph status

```bash
gira goal status 521 --repo OWNER/app --json
```

Documented in: `docs/goal-operating-model.md`, `docs-site/command-reference.md`

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

## `milestone assign`

Bulk attach selected tickets to a milestone through dry-run/apply.

Usage:

```bash
gira milestone assign MILESTONE --tickets 1,2,3 [--repo OWNER/REPO] --dry-run|--apply [--json]
```

Since: `v1.16.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--tickets`: Comma-separated ticket numbers.
- `--dry-run`: Preview assignment.
- `--apply`: Assign selected tickets.
- `--json`: Emit stable JSON.

Examples:

- Preview bulk assignment

```bash
gira milestone assign "2.0 Alpha" --tickets 12,13 --dry-run
```

Documented in: `docs-site/sprint-release.md`, `docs-site/ticket-workflow.md`

## `milestone list`

List GitHub milestones with Gira progress fields.

Usage:

```bash
gira milestone list [--repo OWNER/REPO] [--state open|closed|all] [--json]
```

Since: `v1.16.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--state`: Milestone state: open, closed, or all. Default: open.
- `--json`: Emit stable JSON.

Examples:

- List open milestones

```bash
gira milestone list --state open
```

Documented in: `docs-site/sprint-release.md`, `docs-site/ticket-workflow.md`

## `milestone new`

Preview and create a GitHub milestone as a first-class Gira work batch.

Usage:

```bash
gira milestone new "TITLE" [--repo OWNER/REPO] [--description TEXT] [--due-on YYYY-MM-DD] --dry-run|--apply [--json]
```

Since: `v1.16.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--description`: Milestone description.
- `--due-on`: Milestone due date or timestamp.
- `--dry-run`: Preview milestone creation.
- `--apply`: Create the milestone.
- `--json`: Emit stable JSON.

Examples:

- Preview a milestone

```bash
gira milestone new "2.0 Alpha - State-Aware Ticket Runtime" --dry-run
```

Documented in: `docs-site/sprint-release.md`, `docs-site/ticket-workflow.md`

## `milestone plan`

Select candidate tickets by labels and assign them to a milestone.

Usage:

```bash
gira milestone plan MILESTONE [--repo OWNER/REPO] [--label LABEL] [--state open|closed|all] [--limit N] --dry-run|--apply [--json]
```

Since: `v1.16.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--label`: Candidate label filter. Defaults to status:ready.
- `--state`: Ticket state: open, closed, or all. Default: open.
- `--limit`: Maximum candidate tickets. Default: 20.
- `--dry-run`: Preview assignment plan.
- `--apply`: Assign selected tickets.
- `--json`: Emit stable JSON.

Examples:

- Plan from ready tickets

```bash
gira milestone plan "2.0 Alpha" --label status:ready --dry-run
```

Documented in: `docs-site/sprint-release.md`, `docs-site/ticket-workflow.md`

## `milestone status`

Summarize child ticket state for one milestone work batch.

Usage:

```bash
gira milestone status MILESTONE [--repo OWNER/REPO] [--json]
```

Since: `v1.16.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--json`: Emit stable JSON.

Examples:

- Inspect a milestone

```bash
gira milestone status "2.0 Alpha - State-Aware Ticket Runtime"
```

Documented in: `docs-site/sprint-release.md`, `docs-site/ticket-workflow.md`

## `queue handoff`

Select or inspect an agent-ready workspace queue item and embed the worker-handoff/v1 payload.

Usage:

```bash
gira queue handoff [--config .gira/config.yaml] [--repo OWNER/REPO] [--ticket N] [--role implementer] [--profile default] [--compact] [--json]
```

Since: `v2.1.0`

Flags:

- `--config`: Explicit workspace config path. Defaults to global registry, then .gira/config.yaml.
- `--repo`: Narrow selection to one execution repo, or select the explicit ticket repo.
- `--ticket`: Explicit ticket number. Without it, handoff uses queue next selection.
- `--role`: Handoff role: planner, implementer, or reviewer. Default: implementer.
- `--profile`: Handoff profile: default or python. Default: default.
- `--compact`: Print compact text output.
- `--json`: Emit stable queue-handoff/v1 JSON with worker-handoff/v1 embedded.

Examples:

- Build a handoff packet for the next LLM-ready item

```bash
gira queue handoff --json
```

Documented in: `docs/workspace.md`, `docs-site/command-reference.md`

## `queue list`

List workspace queue items derived from workspace-queues/v1.

Usage:

```bash
gira queue list [--config .gira/config.yaml] [--repo OWNER/REPO] [--queue ready|review|finish|blocked|failed|human] [--limit N] [--compact] [--json]
```

Since: `v2.1.0`

Flags:

- `--config`: Explicit workspace config path. Defaults to global registry, then .gira/config.yaml.
- `--repo`: Narrow queue items to one or more execution repos.
- `--queue`: Filter by queue alias: ready, review, finish, blocked, failed, or human.
- `--limit`: Maximum queue items to print. Default: all.
- `--compact`: Print compact text output.
- `--json`: Emit stable queue-list/v1 JSON.

Examples:

- List agent-ready work

```bash
gira queue list --queue ready --json
```

Documented in: `docs/workspace.md`, `docs-site/command-reference.md`

## `queue next`

Select the first agent-ready workspace queue item and print handoff and run-start commands.

Usage:

```bash
gira queue next [--config .gira/config.yaml] [--repo OWNER/REPO] [--role implementer] [--profile default] [--compact] [--json]
```

Since: `v2.1.0`

Flags:

- `--config`: Explicit workspace config path. Defaults to global registry, then .gira/config.yaml.
- `--repo`: Narrow selection to one or more execution repos.
- `--role`: Handoff role: planner, implementer, or reviewer. Default: implementer.
- `--profile`: Handoff profile: default or python. Default: default.
- `--compact`: Print compact text output.
- `--json`: Emit stable queue-next/v1 JSON.

Examples:

- Select the next LLM-ready item

```bash
gira queue next --json
```

Documented in: `docs/workspace.md`, `docs-site/command-reference.md`

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

Merge the linked PR when policy allows and close the ticket loop without local checkout sync by default.

Usage:

```bash
gira ticket finish [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--sync-local]
```

Since: `v1.0.0`

Examples:

- Preview finish

```bash
gira ticket finish --dry-run
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `ticket handoff`

Compile a worker-neutral handoff packet from ticket context.

Usage:

```bash
gira ticket handoff [TICKET] [planner|implementer|reviewer] [--role planner|implementer|reviewer] [--profile default|python] [--repo OWNER/REPO] [--json]
```

Since: `v1.17.0`

Flags:

- `--role`: Handoff role: planner, implementer, or reviewer. Default: implementer.
- `--profile`: Handoff profile: default or python. Default: default.
- `--json`: Emit stable worker-handoff/v1 JSON.

Examples:

- Compile an implementer handoff packet for the current branch ticket

```bash
gira ticket handoff --json
```

- Compile a reviewer handoff packet for the current branch ticket

```bash
gira ticket handoff reviewer --json
```

Documented in: `docs-site/ticket-workflow.md`, `docs-site/command-reference.md`, `docs/dogfood.md`

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
- `--type`: Ticket type: epic, story, task, bug, spike, or chore.
- `--priority`: Priority: p0, p1, p2, or p3.
- `--label`: Additional repo label that must already exist.
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
gira ticket prompt [TICKET] [planner|implementer|reviewer] [--role planner|implementer|reviewer] [--profile default|python] [--repo OWNER/REPO] [--pr N] [--json]
```

Since: `v1.14.0`

Flags:

- `--role`: Prompt role: planner, implementer, or reviewer.
- `--profile`: Prompt profile: default or python. Default: default.
- `--pr`: Optional PR number for reviewer prompt context.
- `--json`: Emit stable JSON including the rendered prompt.

Examples:

- Render an implementation worker prompt for the current branch ticket

```bash
gira ticket prompt implementer --profile python
```

- Render a reviewer prompt with an explicit PR override

```bash
gira ticket prompt reviewer --pr 77
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `ticket review`

Render a reviewer packet from current ticket and linked PR state.

Usage:

```bash
gira ticket review [TICKET] [--repo OWNER/REPO] [--pr N] [--diff-summary] [--include-diff] [--json|--html --output PATH]
```

Since: `v1.15.0`

Flags:

- `--pr`: Optional PR number override for reviewer packet context.
- `--diff-summary`: Include changed files, diff stat, hunk headers, acceptance mapping candidates, and risk hints.
- `--include-diff`: Include the full PR diff. Output can be long and must be requested explicitly.
- `--json`: Emit stable JSON including issue, PR, evidence, repo guidance, verdict schema, and prompt fields.
- `--html`: Write a static local HTML review packet.
- `--output`: Output path for --html.

Examples:

- Render reviewer packet for current branch ticket

```bash
gira ticket review --diff-summary
```

- Render reviewer packet with an explicit PR override

```bash
gira ticket review --ticket 42 --pr 77 --json
```

- Write a local review packet page

```bash
gira ticket review 42 --repo OWNER/app --diff-summary --html --output out/gira/review-42.html
```

Documented in: `docs-site/ticket-workflow.md`, `docs-site/command-reference.md`, `docs/dogfood.md`

## `ticket self-review`

Post a self-review check note for the current branch ticket and linked PR.

Usage:

```bash
gira ticket self-review [TICKET] [--repo OWNER/REPO] [--pr N] [--diff-summary] --dry-run|--apply [--json]
```

Since: `v1.18.0`

Flags:

- `--pr`: Optional PR number override for self-review context.
- `--diff-summary`: Include compact PR diff summary in the check note. Default: true.
- `--dry-run`: Preview the self-review PR note without posting.
- `--apply`: Post the self-review check note to the linked PR.
- `--json`: Emit stable ticket-self-review-report/v1 JSON.

Examples:

- Preview current branch self-review note

```bash
gira ticket self-review --diff-summary --dry-run
```

- Post current branch self-review note

```bash
gira ticket self-review --diff-summary --apply
```

Documented in: `docs-site/ticket-workflow.md`, `docs-site/command-reference.md`, `docs/dogfood.md`

## `ticket start`

Verify a ready issue, create or reuse its branch, and move it to in-progress.

Usage:

```bash
gira ticket start [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--base BRANCH]
```

Since: `v1.0.0`

Flags:

- `--base`: Explicit lifecycle base branch override recorded on the ticket.
- `--json`: Emit the stable ticket-status/v1 JSON contract with issue, branch, PR, checks, review, evidence, blockers, warnings, and next action.

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
gira ticket status [TICKET] [--repo OWNER/REPO] [--json|--html --output PATH]
```

Since: `v1.0.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--ticket`: Ticket number. Can also be numeric positional.
- `--issue`: Compatibility alias for --ticket.
- `--json`: Emit the stable ticket-status/v1 JSON contract with issue, branch, PR, checks, review, evidence, blockers, warnings, and next action.
- `--html`: Write a static local HTML report from ticket-status/v1.
- `--output`: Output path for --html.

Examples:

- Inspect current branch ticket

```bash
gira ticket status
```

- Export a ticket status page

```bash
gira ticket status 42 --repo OWNER/app --html --output out/gira/ticket-42.html
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

Show a Gira operating card for the ticket, linked PR, blockers, and next action. Alias: gira ticket show.

Usage:

```bash
gira ticket view|show [TICKET] [--repo OWNER/REPO] [--json]
```

Since: `v1.12.0`

Examples:

- Inspect current branch ticket context

```bash
gira ticket view
```

- Inspect an explicit ticket with the show alias

```bash
gira ticket show 42 --repo OWNER/app
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

