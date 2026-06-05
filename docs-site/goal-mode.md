# Goal Mode

Goal mode helps an operator move from a broad GitHub issue to child tickets,
safe next actions, and finish evidence. GitHub issues remain the durable graph:
the goal issue names or links child tickets, child tickets close through PRs,
and finish handoff is written back as a GitHub comment.

## Backlog Goals

In a multi-repo workspace, the inbox repo can hold cross-repo goal handles. A
goal such as `OWNER/backlog#12` can stay open as the coordination issue while
repo-local child tickets execute in `OWNER/app`, `OWNER/api`, or `OWNER/infra`.

Use this when the objective spans repos, ownership is mixed, or a human still
needs one stable issue URL for decisions and convergence. Do not put every child
task in the backlog repo. Once a slice is executable in a codebase, route it to
that target repo and let the child ticket own its branch, PR, checks, review,
and finish evidence.

Broad workspace views and narrowed daily control views use the same rule:
backlog issues coordinate; repo-local issues execute.

## Plan

Preview child ticket packets without creating anything:

```bash
gira goal plan 521 --repo OWNER/app --dry-run --json
```

Use the plan when a goal issue has enough scope and acceptance detail to split
into bounded tickets. Same-repo children remain the default. To route a child
into another execution repo, prefix a plan item with `OWNER/REPO:` or
`target_repo: OWNER/REPO -`. The `goal-plan/v1` output includes `target_repo`
for every proposed child. If the goal requires a human decision or the target
repo is ambiguous, the report stops with an explicit reason instead of
inventing work.

After reviewing the plan, create the linked child tickets:

```bash
gira goal plan 521 --repo OWNER/app --apply --json
```

The apply path creates normal GitHub issues in each child target repo. Same-repo
children keep a readable `Parent: #521` reference in their body; cross-repo
children use `Parent: OWNER/app#521`. Gira also comments on the parent goal with
the created child links so later `goal status` and `goal plan` runs can discover
cross-repo children without a separate planning database. Each child carries the
proposed labels; same-repo children also inherit the parent milestone. A child
is skipped on later runs when an existing linked child in the same target repo
already matches the proposed title.

## Status

Inspect the current goal graph:

```bash
gira goal status 521 --repo OWNER/app --json
```

The `goal-status/v1` report includes the goal issue, child ticket counts,
blockers, remaining autonomous work, handoff receipt presence, and the next safe
action. It is the read-only summary to use before starting a new child ticket.

## Report

Build the first visible v3 operating artifact for a goal:

```bash
gira goal report 521 --repo OWNER/app --json
gira goal report 521 --repo OWNER/app --html --output out/gira/goal-521.html
```

The `goal-dossier/v1` report packages the goal summary, grouped child tickets,
blockers, stop conditions, selected next child, evidence summary, and next safe
step into one JSON contract or local HTML page. It is generated from existing
Goal Mode state and does not become a source of truth. `gira goal dossier`
remains as a compatibility alias.

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

Close a ready goal only with an explicit done terminal:

```bash
gira goal finish 521 --repo OWNER/app --terminal done --apply --json
```

This posts a `goal-finish-receipt/v1` done receipt, normalizes active status
labels to `status:done`, and closes the goal when readiness is clean.

Use explicit human-review when blockers or historical evidence gaps need a
maintainer handoff:

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
