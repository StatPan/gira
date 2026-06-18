# Dispatch Reflection

This note captures the handoff-surface reflection before adding larger Dispatch
features. The goal is to keep Gira from becoming another system operators must
manage. Gira should issue a trustworthy work order from existing GitHub state,
then let agents execute through the normal ticket loop.

## Current Handoff Surfaces

| Surface | Current role | Risk |
| --- | --- | --- |
| `gira ticket handoff` | Builds `worker-handoff/v1` for one executable issue. | Too low-level when the operator thinks in larger goals. |
| `gira goal handoff` | Adds parent goal context and selects the next safe child ticket. | Can become another top-level concept if not unified. |
| `gira queue handoff` | Selects an agent-ready workspace queue item and embeds `worker-handoff/v1`. | Queue selection and work-order issuance can feel separate. |

These surfaces are useful, but they should converge into a single operator
mental model: ask Gira for the next official work order, give that packet to an
agent, then close the loop with evidence.

## Reflection Findings

- The durable source of truth remains GitHub issues, PRs, labels, comments,
  checks, reviews, and referenced docs.
- The agent-facing output should be a formal dispatch packet, not an informal
  prompt transcript.
- Goal-level context should guide priorities and stop conditions, but the
  executable mutation boundary should remain the selected child ticket.
- Existing handoff commands should remain as expert/read-only surfaces while
  Dispatch becomes the preferred operator entrypoint.
- Gira should avoid asking humans to maintain a parallel Gira state. It should
  compute, connect, and issue from current evidence.

## Consolidation Direction

The next control-plane layer is `dispatch-packet/v1`.

```text
goal / ticket / queue / workspace
        -> dispatch packet
        -> agent or subagent
        -> PR/check/review evidence
        -> receipt
        -> next dispatch
```

The first implemented slice is `gira dispatch goal`, which wraps Goal Mode
state and the embedded `worker-handoff/v1` for the next safe child ticket. Later
slices can add `dispatch ticket` and `dispatch queue` without changing the
operator's top-level entrypoint.

## Design Constraints

- Dispatch commands are read-only until an explicit lifecycle command such as
  `ticket start --apply` or `queue take --apply` is invoked.
- Dispatch packets must name authority, references, selected work, allowed
  actions, stop conditions, evidence expectations, and next safe command.
- Dispatch packets are not public-safe by default because they may embed worker
  context and operator notes.
- Every selected child ticket still follows normal Gira issue -> branch -> PR
  -> checks -> finish flow.
