# Closure Funnel Stats

Gira stats are workflow integrity reports. They show whether GitHub work moved
from issue to PR to checks to merge and closed issue.

## Start

```bash
gira stats repo --repo OWNER/REPO --since 90d
gira stats repo OWNER/REPO --since 90d --json
gira stats pulse --repo OWNER/REPO --since 7d
gira stats pulse OWNER/REPO --since 7d --json
```

Text output is the default for humans. Use `--json` for automation and
dashboard exports.

## Pulse

`gira stats pulse` reports recent evidence-backed workflow movement for one
repo. It emits `pulse-report/v1alpha1` and groups movement into named signals:
`finished`, `reviewed`, `refined`, `unblocked`, `superseded`, `started`, and
`checked`.

Pulse also shows current attention counts for ready, review-needed,
finish-ready, blocked, failed-check, and human-decision work. It is not a
ranking surface.

## Metrics

The repo report includes opened issues, closed issues, superseded issues,
opened PRs, merged PRs, PRs with closing links, merged PRs with linked issues,
pending or failing checks, stale open issues, stale open PRs, and closure rate.

Closure rate is:

```text
merged PRs with linked issues / opened issues
```

It is a workflow evidence rate, not a productivity score.

## Confidence

The report is GitHub read-only and works on non-Gira repos. Confidence improves
when a repository uses Gira conventions: `status:*`, `type:*`,
`resolution:*`, required closing links, and lifecycle commands.

Superseded issues are counted separately from completed issues when they carry
`resolution:superseded`.

## Workspace Direction

`gira stats workspace --since 90d` is the planned multi-repo rollup. It should
reuse workspace repo selection, bounded fetches, cache TTLs, refresh behavior,
and API budget reporting from `gira workspace status`.

## Non-goals

- Personal productivity score.
- Individual leaderboard.
- Full DORA suite.
- AI spend or token analytics.
- Dashboard UI in the first slice.
- Precise agent attribution in the first slice.
