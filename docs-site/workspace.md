# Workspace

Workspace commands are for a personal or team operator who needs one bounded
view across several GitHub repositories. They do not replace repo-local issues,
PRs, labels, milestones, or checks as the source of truth.

## Setup

Create the global workspace and repo registry together when starting from a
local checkout:

```bash
gira setup global \
  --repo OWNER/app \
  --path . \
  --workspace personal \
  --inbox-repo OWNER/backlog \
  --mode global-only \
  --dry-run

gira setup global \
  --repo OWNER/app \
  --path . \
  --workspace personal \
  --inbox-repo OWNER/backlog \
  --mode global-only \
  --apply
```

Use a dedicated `--inbox-repo` for backlog intake when the workspace spans more
than one execution repository. A small single-repo setup may reuse the execution
repo as the inbox.

## Repo Allowlist

Sync the workspace repo allowlist explicitly from a GitHub owner:

```bash
gira workspace repos sync --owner OWNER --workspace personal --dry-run
gira workspace repos sync --owner OWNER --workspace personal --apply
```

This is not background discovery. The command updates the global workspace
registry and skips the configured inbox repo because intake is not an execution
repo.

## Status And Queues

Use bounded status reads for daily inspection:

```bash
gira workspace status --limit 10 --active-only
gira workspace status --repo OWNER/app
gira workspace status --json
```

`workspace status` reports repo summaries, GitHub API budget when available,
cache freshness, warnings, and `workspace_queues`. The queue contract groups
work that needs operator attention:

| Queue | Meaning |
| --- | --- |
| `agent_ready` | Executable agent-lane issues that are ready to start. |
| `review` | Reviewable PR or in-review ticket work. |
| `finish` | Work that appears ready for finish or closure convergence. |
| `blocked` | Tickets blocked by labels, missing evidence, or human decisions. |
| `failed_checks` | PR-backed work with failed or pending check evidence. |

Use the printed next steps before mutating anything. Workspace commands are
read-first; repo and ticket lifecycle commands still own branch, PR, note, and
finish mutations.

## Related Pages

- [Global Config](/global-config)
- [Ticket Workflow](/ticket-workflow)
- [Readiness And Audit](/readiness-audit)
- [Command Reference](/command-reference)
