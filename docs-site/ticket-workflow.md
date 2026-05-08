# Ticket Workflow

Use Gira for the issue to branch to PR to merge lifecycle. Raw `gh` remains the backend, not the daily UX.

## New Work

```bash
gira ticket new "TITLE" --goal "GOAL" --acceptance "a;b;c" --apply --start
gira ticket list --state open --label status:ready --limit 20
gira ticket pr --apply --draft
gira ticket checks
gira ticket wait --timeout 5m
gira ticket finish --apply
```

## Existing Issue

```bash
gira ticket start 42 --apply
gira ticket pr --apply --draft
gira ticket finish --apply
```

Use `gira ticket list` to discover repo issue-backed tickets without dropping to raw `gh`. It supports `--state open|closed|all`, repeatable or comma-separated `--label`, `--assignee`, `--milestone`, `--limit`, and `--json`.
Use `gira epic list` for the same discovery pattern scoped to `type:epic` issues.

## Agent Rules

- Start from a GitHub issue.
- Use a feature branch per issue.
- Use Gira ticket lifecycle commands for status, start, PR, checks, wait, and finish work; use raw `gh` only when Gira has no lifecycle command.
- Keep PR bodies linked with `Closes #N`, `Fixes #N`, or `Resolves #N`.
- Treat Project-only items as intake until they are backed by repository issues.
