# Agent Handoff Queue

The Agent Handoff Queue is the 3.1 operating layer for choosing LLM-ready work
from a workspace. It is not a new database, worker runtime, or dashboard source.
It is a CLI-first layer over `workspace-queues/v1`, `ticket handoff`, and the
normal ticket lifecycle.

## Scope

Use the queue commands when an operator or adapter needs to answer:

- What work is ready, blocked, waiting for review, finish-ready, failed, or
  waiting for a human decision?
- Which one item should an LLM worker receive next?
- Is the ticket body strong enough to hand off safely?
- Can Gira start the ticket branch without launching a worker process?

Do not use this layer to store private queue state, rank people, supervise
workers, or replace GitHub issues and PRs.

## Source Contract

`workspace-queues/v1` remains the source contract. It is computed from GitHub
issues, labels, PRs, checks, reviews, readiness reports, and workspace config.
The top-level queue commands expose smaller contracts for specific decisions:

| Command | Schema | Mutates | Purpose |
| --- | --- | --- | --- |
| `gira queue list` | `queue-list/v1` | No | Inventory queue items by canonical queue name. |
| `gira queue next` | `queue-next/v1` | No | Select the first `agent_ready` item and print handoff/run commands. |
| `gira queue handoff` | `queue-handoff/v1` | No | Embed `worker-handoff/v1` and stop if the selected ticket is not worker-ready. |
| `gira queue take --dry-run` | `queue-take/v1` | No | Preview selection, handoff readiness, and delegated `ticket start`. |
| `gira queue take --apply` | `queue-take/v1` | Yes | Delegate to `ticket start` for a handoff-safe and worker-ready item. |

`queue take --apply` is the only mutating queue command. It preserves the
existing `ticket start` branch policy and status behavior. It does not run
Codex, call an LLM, or start a long-running worker.

## Command Roles

`queue list` is for inventory and dashboards. It can show all queues or a
filtered lane such as `ready`, `review`, `finish`, `blocked`, `failed`, or
`human`.

`queue next` is for deterministic selection. It chooses the first
`agent_ready` item after workspace ordering and repo filters. Its output carries
the original `next_safe_command`, a `handoff_command`, and a `run_command`.

`queue handoff` is for worker packets. It calls the same ticket handoff builder
as `gira ticket handoff`, embeds `worker-handoff/v1`, and stops when ticket
readiness says the work needs refinement. See
[Worker Handoff Contract](worker-handoff-contract.md) for the shared ticket and
queue handoff schema.

`queue take` is for safe work start. Dry-run previews the selected repo/ticket,
selection reason, worker handoff, planned `ticket start`, approval evidence, and
next command. Apply starts only when both the queue item and worker handoff are
safe.

## Stop Reasons

Stop reasons are deliberate output, not errors to ignore.

| Stop reason family | Meaning | Usual next step |
| --- | --- | --- |
| `no_agent_ready_item` | No item is eligible for automatic handoff. | Inspect `queue list`. |
| `queue_not_handoff_safe` | The explicit ticket is in review, finish, blocked, failed, or human-decision state. | Run the item's `next_safe_command`. |
| `queue_blocked`, `queue_failed_check`, `queue_review_needed`, `queue_finish_ready`, `queue_human_decision` | The item belongs to a non-start queue. | Review, fix checks, finish, or ask a human instead of starting work. |
| `worker_handoff_not_ready` | The queue item looks ready, but `worker-handoff/v1` readiness found weak or missing ticket context. | Refine or view the ticket. |
| `readiness_needs_refinement`, `finding_*`, `next_refine_ticket` | The ticket readiness report named concrete missing evidence. | Add scope, acceptance, doctor impact, or evidence expectations. |

Adapters should treat stop reasons as policy gates. `queue take --apply` refuses
non-handoff-safe queues and worker-not-ready handoffs before calling
`ticket start`.

## Worker Boundary

The queue layer stops at selection, handoff, and branch start. External workers
still own implementation, verification, PR updates, and provenance. The intended
flow is:

```text
workspace-queues/v1
  -> queue next or queue handoff
  -> worker-handoff/v1
  -> queue take or ticket start
  -> external worker
  -> PR/check/review evidence
  -> ticket finish
```

`gira run start` may be printed as a next command so a local runtime can create
a `worker-run/v1` manifest, but queue commands do not invoke that runtime by
default.

## Relationship To Goal Mode

`goal next` and `queue next` solve different selection problems.

- `goal next` selects the next child ticket inside one delegated objective.
- `queue next` selects the next workspace item across execution repos.
- `workspace-queues/v1` can include goal children, normal tickets, PR review
  work, and finish-ready work.

If a goal child is not ready, goal mode should preserve its stop reason. If a
workspace item is not handoff-safe, queue mode should preserve its stop reason.
Neither mode should hide the other behind a generic query result.

## Candidate Decisions

The 3.1 queue model resolves the first planning layer for older query, export,
and backlog candidates:

| Candidate | Decision |
| --- | --- |
| #643 Query UX for tickets, queues, and workspace state | Queue filters and `workspace-queues/v1` are the first operational query surface. Keep #643 only for remaining query UX gaps; do not introduce a generic query database by default. |
| #641 Export reports and workspace artifacts | Export/report work should consume `workspace-queues/v1` and the `queue-*` JSON contracts instead of creating a parallel queue source. |
| #645 Backlog locality | Backlog locality remains a planning question. Executable work state should still converge through GitHub issues, `workspace-queues/v1`, ticket handoff, and queue take. |

Decision comments were posted on those issues from #698 so later work can treat
this page as the predecessor decision instead of reopening the same source of
truth question.

## Maintenance Rules

- Keep command facts in `internal/gira/command_registry.go`; generated docs must
  be refreshed from that registry.
- Add new queue states to `workspace-queues/v1` before exposing a new queue CLI
  view.
- Prefer stop reasons over silent fallback.
- Keep queue output public-safe: work-item state and next safe commands only.
- Keep local cache disposable; queue membership must be reconstructable from
  GitHub plus Gira config.
- Keep `queue handoff` and `ticket handoff` aligned on
  `worker-handoff/v1`; queue-specific fields should wrap the worker packet
  rather than fork the handoff schema.
