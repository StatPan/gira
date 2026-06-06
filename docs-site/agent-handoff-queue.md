# Agent Handoff Queue

The Agent Handoff Queue is the 3.1 operating layer for choosing LLM-ready work
from a workspace. It is not a new database, worker runtime, or dashboard source.
It is a CLI-first layer over `workspace-queues/v1`, `ticket handoff`, and the
normal ticket lifecycle.

## Source Contract

`workspace-queues/v1` remains the source contract. It is computed from GitHub
issues, labels, PRs, checks, reviews, readiness reports, and workspace config.

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

## Stop Reasons

Stop reasons are policy gates, not noise.

| Stop reason family | Meaning | Usual next step |
| --- | --- | --- |
| `no_agent_ready_item` | No item is eligible for automatic handoff. | Inspect `queue list`. |
| `queue_not_handoff_safe` | The ticket is in review, finish, blocked, failed, or human-decision state. | Run the item's `next_safe_command`. |
| `worker_handoff_not_ready` | `worker-handoff/v1` found weak or missing ticket context. | Refine or view the ticket. |
| `readiness_needs_refinement`, `finding_*`, `next_refine_ticket` | Ticket readiness named concrete missing evidence. | Add scope, acceptance, doctor impact, or evidence expectations. |

Adapters should refuse `queue take --apply` when any stop reason is present.

## Worker Boundary

The queue layer stops at selection, handoff, and branch start. External workers
still own implementation, verification, PR updates, and provenance.

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

## Related Planning

The 3.1 queue model resolves the first planning layer for older query, export,
and backlog candidates:

| Candidate | Decision |
| --- | --- |
| #643 Query UX | Queue filters and `workspace-queues/v1` are the first operational query surface. |
| #641 Export reports | Export/report work should consume `workspace-queues/v1` and `queue-*` JSON. |
| #645 Backlog locality | Executable work state should converge through GitHub issues, queue contracts, ticket handoff, and queue take. |

See also [Workspace](/workspace), [Worker Boundary](/worker-boundary), and
[Command Reference](/command-reference).
