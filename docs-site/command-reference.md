# Command Reference

This page is generated from Gira's command metadata registry. Update `internal/gira/command_registry.go` first, then refresh this page.

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
gira workspace status [--config .gira/config.yaml] [--json]
```

Since: `v1.0.0`

Flags:

- `--config`: Explicit workspace config path. Defaults to global registry, then .gira/config.yaml.
- `--json`: Emit stable JSON.

Examples:

- Read the default workspace

```bash
gira workspace status
```

Documented in: `README.md`, `docs/workspace.md`, `docs-site/global-config.md`

