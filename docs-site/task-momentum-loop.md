# Task Momentum Loop

Gira's task momentum loop should not copy token, streak, quota, leaderboard, or
activity-score mechanics. The first design translates that need into Gira's
existing operating model: evidence-backed progress, workflow hygiene, visible
continuity, and operator control.

Source document:
[`docs/task-momentum-loop.md`](https://github.com/StatPan/gira/blob/main/docs/task-momentum-loop.md).

Design issue: #704. Implementation successor: #706.

## Decision

Use `pulse` as the first user-facing word. `Pulse` means a compact recent-period
report of meaningful workflow movement:

```text
What moved, what got healthier, and what still needs operator attention?
```

Keep `momentum` as the broader product category. Avoid `token`, `streak`,
`score`, and leaderboard language in the first surface.

## Counted Signals

Pulse should group evidence into named signals instead of one opaque score:

| Signal | Meaning |
| --- | --- |
| `finished` | A ticket closes through a linked merged PR or finish receipt. |
| `reviewed` | A PR moves toward finish with review or passing-check evidence. |
| `refined` | An unready issue becomes worker-handoff-ready. |
| `unblocked` | A blocked or human-decision item leaves that lane with evidence. |
| `superseded` | Stale or replaced work closes with `resolution:superseded`. |
| `started` | Ready work is started through Gira lifecycle commands. |
| `checked` | A local report bundle is generated as an operator checkpoint. |

`checked` is not execution progress. It is a useful operating habit signal.

## First Command

Start with a read-only repo command:

```bash
gira stats pulse --repo OWNER/REPO --since 7d --json
gira stats pulse --repo OWNER/REPO --since 7d
```

After the repo contract is stable, add workspace pulse:

```bash
gira workspace pulse --config .gira/config.yaml --since 7d --json
```

The first schema should be `pulse-report/v1alpha1`.

## Dashboard Direction

The local dashboard bundle should consume pulse only after the CLI JSON contract
is stable.

Recommended artifacts:

```text
out/dashboard/
  derived/
    workspace_pulse.json
  csv/
    workspace_pulse_items.csv
```

`workspace_dashboard.json` can then include a compact pointer and summary. The
HTML page should show pulse near queue counts and top actions, without a badge
economy or global score.

## Boundaries

- Do not reward empty comments, repeated comments, or label churn.
- Do not count issue creation by itself.
- Do not count report generation as execution progress.
- Do not rank people, agents, assignees, or organizations.
- Do not infer effort from time online, token spend, commit count, or comment
  volume.
- Keep GitHub/Gira evidence canonical; local exports are derived snapshots.
