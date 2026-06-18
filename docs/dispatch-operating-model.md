# Dispatch Operating Model

Dispatch is Gira's official AI work-order layer. It turns current GitHub and
Gira evidence into a packet an agent can trust without requiring the operator
to manage a separate planning system.

Dispatch is not a worker runtime, a dashboard, or a new planning database. It is
a read-only issuance layer over existing Gira state.

## Purpose

A dispatch packet answers:

- What authorizes this work?
- Which goal or ticket provides the objective?
- Which exact child ticket is executable now?
- Which docs, issues, PRs, and schemas are referenced?
- What may the agent do without asking?
- When must the agent stop?
- What evidence is required before the loop can advance?
- Which command is safe to run next?

The packet should feel like an official work order. It should be durable enough
for agents, subagents, and humans to share a common basis for trust.

## Packet Contract

The first stable schema is `dispatch-packet/v1`.

Core fields:

| Field | Meaning |
| --- | --- |
| `source` | The entrypoint that produced the packet, such as `goal`. |
| `authority` | The goal, ticket, queue item, and schema facts that authorize the packet. |
| `references` | Issues, PRs, docs, and handoff schemas the agent should treat as context. |
| `instruction` | Objective, selected work, allowed actions, stop conditions, and evidence required. |
| `goal_handoff` | Goal Mode context when the dispatch source is a goal. |
| `worker_handoff` | Embedded `worker-handoff/v1` for the selected child ticket when one is available. |
| `stop_reasons` | Why no agent execution should start. |
| `next_safe_command` | The next Gira command to run. |

Dispatch packets are private by default. They may include issue bodies, operator
notes, branch policy, PR context, and review guidance.

## Compact Context

Full `dispatch-packet/v1` output is the audit/debug form. Agent handoff should
prefer one of the budgeted forms:

```bash
gira dispatch goal --compact-json --context-budget 8000
gira dispatch goal --prompt --context-budget 8000
```

Compact output uses `dispatch-compact/v1`. It keeps authority, selected work,
objective, acceptance, required evidence, stop conditions, linked PR summary,
and the next safe command. It deliberately omits full issue bodies, role
packets, complete child graphs, and verbose check details. Those remain
available through references when audit detail is needed.

## Goal Dispatch

`gira dispatch goal [GOAL] --json` is the first implementation slice. The goal
number is inferred from the current branch, its parent goal, or the only open
goal/epic when that context is unambiguous.

It performs this read-only chain:

```text
goal status
  -> goal next
  -> goal handoff
  -> dispatch-packet/v1
```

If a child ticket is selected, the packet embeds `worker-handoff/v1`. If no
child is safe, the packet reports stop reasons and a next safe command such as
`goal plan`, `goal status`, or `goal finish --dry-run`.

Goal dispatch deliberately does not start branches or mutate GitHub. Execution
still happens through the selected child ticket's lifecycle commands.

## Operator Loop

The preferred larger-task loop becomes:

```bash
gira dispatch goal --repo OWNER/backlog --role implementer --json
# give dispatch-packet/v1 to an agent
# agent works only inside the selected child ticket
gira ticket checks
gira ticket finish --dry-run
gira ticket finish --apply
gira dispatch goal --repo OWNER/backlog --role implementer --json
```

This keeps the user-facing loop small while allowing goals to coordinate large
multi-ticket work.

## Relationship To Existing Handoff Commands

| Existing command | Dispatch relationship |
| --- | --- |
| `ticket handoff` | Low-level worker packet for one issue. Dispatch may embed it. |
| `goal handoff` | Goal-aware selector and context builder. `dispatch goal` wraps it. |
| `queue handoff` | Workspace queue selector. Future `dispatch queue` can wrap it. |

Existing commands remain useful for diagnostics and expert flows. Dispatch is
the intended default entrypoint for agent work orders.

## Future Slices

- `gira dispatch ticket TICKET --json`: wrap `ticket handoff` directly.
- `gira dispatch queue --json`: wrap `queue handoff` selection.
- Dispatch receipt summaries that compare required evidence with PR/check/review
  state before advancing to the next packet.
- Adapter guidance that treats Dispatch as the preferred agent input surface.
