# Goal Mode

Goal mode helps an operator move from a broad GitHub issue to child tickets,
safe next actions, and finish evidence. GitHub issues remain the durable graph:
the goal issue names or links child tickets, child tickets close through PRs,
and finish handoff is written back as a GitHub comment.

## Plan

Preview child ticket packets without creating anything:

```bash
gira goal plan 521 --repo OWNER/app --dry-run --json
```

Use the plan when a goal issue has enough scope and acceptance detail to split
into bounded tickets. If the goal requires a human decision or the target repo is
ambiguous, the report stops with an explicit reason instead of inventing work.

## Status

Inspect the current goal graph:

```bash
gira goal status 521 --repo OWNER/app --json
```

The `goal-status/v1` report includes the goal issue, child ticket counts,
blockers, remaining autonomous work, handoff receipt presence, and the next safe
action. It is the read-only summary to use before starting a new child ticket.

## Next

Select the next safe child ticket or stop:

```bash
gira goal next 521 --repo OWNER/app --json
```

`goal-next/v1` prefers actionable child tickets and returns stop reasons when
work should not continue. If every child is done and a
`goal-finish-receipt/v1` human-review handoff already exists, it stops at
`human_review` instead of recommending another finish command.

## Finish

Preview convergence before posting any receipt:

```bash
gira goal finish 521 --repo OWNER/app --dry-run --json
```

The current apply-safe path supports explicit human-review handoff receipts:

```bash
gira goal finish 521 --repo OWNER/app --terminal human_review --apply --json
```

This posts a `goal-finish-receipt/v1` comment when the graph has reached a
declared handoff state. It does not close the goal, mark it done, waive missing
historical evidence, or duplicate an existing handoff receipt.

## Related Work

Use `gira epic list` for numberless discovery of `type:epic` issues. Use
[Sprint And Release](/sprint-release) when the boundary is a GitHub milestone
rather than a goal issue. Use [State Model](/state-model) for the distinction
between labels, computed goal state, receipts, and local cache.
