# Ticket Workflow

Use Gira for the issue to branch to PR to merge lifecycle. Raw `gh` remains the backend, not the daily UX.

## New Work

```bash
gira ticket new "TITLE" --goal "GOAL" --acceptance "a;b;c" --apply --start
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

## Agent Rules

- Start from a GitHub issue.
- Use a feature branch per issue.
- Prefer Gira commands over raw `gh` when a Gira command exists.
- Keep PR bodies linked with `Closes #N`, `Fixes #N`, or `Resolves #N`.
