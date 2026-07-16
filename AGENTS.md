# Gira Agent Instructions

## Purpose

Gira turns a GitHub repository into an AI-ready project operating system. GitHub is the execution backend: Issues are task packets, PRs are change units, milestones are phase/sprint boundaries, and repo templates define the process.

Canonical lifecycle: [agent operator](docs/skills/gira-agent-operator.md). Canonical PM policy: [PM operating policy](docs/pm-operating-policy.md); task packets: [PM skill](docs/pm-skill.md). This is the short Codex/OpenAI adapter.

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

- Follow the canonical Gira agent operator skill.
- Start implementation from a GitHub Issue and use one feature branch per issue.
- Prefer Gira lifecycle commands over raw `gh` when Gira provides the operation.
- Use `--dry-run` before `--apply` for mutating Gira commands.
- PR body must contain `Closes #N`, `Fixes #N`, or `Resolves #N` unless the issue is intentionally kept open.
- Keep changes bounded to the target issue and prefer tests for CLI behavior and idempotency.
- Avoid destructive operations without explicit user approval.
