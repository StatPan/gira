# Gira Agent Instructions

## Purpose

Gira turns a GitHub repository into an AI-ready project operating system. GitHub is the execution backend: Issues are task packets, PRs are change units, milestones are phase/sprint boundaries, and repo templates define the process.

## MVP Boundaries

Keep the product CLI-first and small. The Go-built `gira` binary is the only product implementation.

- Go `gira` CLI for the product path.
- Default template only.
- `gh` CLI first unless a direct Go GitHub API client is explicitly chosen for a slice.
- Idempotent file install, label sync, milestone sync, bootstrap issue creation, and compact status summary.
- Package-manager wrappers such as `uv`, npm, bun, or Homebrew are allowed only as distribution channels for the Go-built binary, not as alternate runtimes.

Do not implement these in MVP unless explicitly requested:

- GitHub Projects v2 automation.
- LLM PRD-to-issue decomposition.
- Web UI.
- Jira import/export.
- Slack/Discord bot integration.

## Worker Rules

- Start implementation from a GitHub Issue.
- Use a feature branch per issue.
- PR body must contain `Closes #N`, `Fixes #N`, or `Resolves #N` unless the issue is intentionally kept open.
- Keep changes bounded to the target issue.
- Avoid destructive operations: no secret rotation, credential edits, repository deletion, or broad GitHub setting changes without explicit user approval.
- Prefer tests for CLI behavior and idempotency.
