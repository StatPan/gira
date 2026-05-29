# Gira 2.0 Control Plane

Gira 2.0 names the stable CLI-first contract for AI-assisted software work:

> A GitHub-native control plane that turns human and AI work into reviewable,
> auditable, evidence-backed completion.

It is not a hosted service, UI launch, or provider expansion. GitHub remains
the execution ledger for issues, branches, PRs, checks, reviews, labels,
milestones, and completion evidence.

## Stable Surface

| Area | Contract |
| --- | --- |
| Ticket lifecycle | State-aware issue to branch to PR to finish commands with dry-run/apply safety. |
| Readiness and audit | Ticket, PR, finish, receipt, telemetry, and drift evidence. |
| Goal mode | Child-ticket graph status, next safe work selection, dry-run planning, and human-review handoff receipts. |
| Workspace health | Agent-ready, review-needed, finish-ready, blocked, failed-check, and human-decision queues. |
| Adapter boundary | Command capability metadata and `gira-approval-plan/v1` dry-run evidence for apply approval. |

## Daily Shape

```text
issue -> branch -> PR -> checks -> review -> evidence -> finish
```

Gira makes each step inspectable. A worker can start from a ready ticket, open a
linked PR, wait for checks, and finish only when policy allows. A reviewer can
read a packet built from the actual PR state. A maintainer can inspect receipts
without trusting an agent transcript.

## What Remains Outside 2.0

- Hosted dashboards and background sync.
- Web UI/TUI, chat bots, or hosted agent execution.
- GitLab, Forgejo, Gitea, Notion, or Linear providers.
- Full bidirectional Jira sync or Jira workflow administration.
- LLM PRD-to-ticket decomposition.
- A Gira-native planning database as the source of truth.

Those directions can build on the same contracts later. They are not required
for the 2.0 CLI release.

## Release Readiness

The release-readiness decision is tracked in the source docs:

- [Gira 2.0 Release Readiness](https://github.com/StatPan/gira/blob/main/docs/v2-release-readiness.md)
- [Product OS Roadmap](https://github.com/StatPan/gira/blob/main/docs/product-os-roadmap.md)
- [State Model](/state-model)
- [Worker Boundary](/worker-boundary)
