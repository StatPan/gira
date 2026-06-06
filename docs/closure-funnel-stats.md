# Closure Funnel Stats

Gira stats reports are workflow integrity reports, not generic productivity
analytics. The first report answers one question:

Did GitHub work actually move from issue to PR to checks to merge and closed
issue?

## Commands

```bash
gira stats repo --repo OWNER/REPO --since 90d
gira stats repo OWNER/REPO --since 90d --json
gira stats pulse --repo OWNER/REPO --since 7d
gira stats pulse OWNER/REPO --since 7d --json

# planned multi-repo rollup
gira stats workspace --since 90d
```

The default output is rendered text for humans. `--json` is for automation,
exports, and future dashboard consumers.

## Report Scope

`gira stats repo` is read-only. It uses GitHub issue, PR, and check metadata
through `gh`; it does not read source code, diffs, or local files. The report
works on non-Gira repositories, but confidence is lower because GitHub metadata
does not always prove work start, blocked duration, replacement intent, or agent
attribution.

Gira-managed repositories raise confidence through:

- `status:*`, `type:*`, and `resolution:*` labels.
- PR bodies with `Closes #N`, `Fixes #N`, or `Resolves #N`.
- Gira lifecycle commands that keep issue, branch, PR, checks, and finish
  evidence linked.
- Superseded tickets closed with `resolution:superseded` instead of
  `status:done`.

## Pulse

`gira stats pulse` is the first Gira-native recent movement report. It emits
`pulse-report/v1alpha1` and groups evidence into named signals instead of one
opaque score:

- `finished`: merged PRs with closing references.
- `reviewed`: open PRs that moved toward finish through review or passed
  checks.
- `refined`: older issues that became structured `status:ready` work.
- `unblocked`: older issues with explicit unblock evidence.
- `superseded`: closed issues carrying `resolution:superseded`.
- `started`: older issues now in `status:in-progress`.
- `checked`: report/checkpoint evidence when a later export slice supplies it.

The report also includes current attention counts for ready, review-needed,
finish-ready, blocked, failed-check, and human-decision work. Transition-history
weaknesses are surfaced as warnings rather than hidden confidence.

## Metrics

The first Closure Funnel report includes:

- opened issues in the reporting window.
- closed issues in the reporting window.
- superseded issues, separated from completed issues.
- opened PRs in the reporting window.
- merged PRs in the reporting window.
- PRs with closing links.
- merged PRs with linked issues.
- PRs with pending checks.
- PRs with failing checks.
- stale open issues.
- stale open PRs.
- closure rate.

Closure rate is intentionally transparent:

```text
merged PRs with linked issues / opened issues
```

This is a workflow evidence rate, not a person or team productivity score.

## Workspace Direction

`gira stats workspace --since 90d` should roll up the same report across a
configured workspace. It should reuse global workspace repo selection, bounded
repo limits, cache TTLs, refresh behavior, and GitHub API budget reporting from
`gira workspace status`.

The workspace report should show:

- one aggregate Closure Funnel.
- per-repo rows for bottleneck discovery.
- freshness and rate-limit diagnostics.
- confidence level per repo.

## Caching And Rate Limits

The repo report starts without a persistent cache so the first slice stays
simple and auditable. The workspace report must add caching before broad
multi-repo sync becomes default.

Recommended strategy:

- Use GitHub App or authenticated `gh` calls; unauthenticated calls are too
  limited for workspace analytics.
- Batch by repo and time window.
- Cache normalized issue/PR/check metadata under the Gira cache root.
- Expose cache freshness in text and JSON.
- Support `--cache-ttl` and `--refresh` on workspace stats before large
  workspaces are encouraged.
- Stop early with a clear diagnostic when the GitHub API budget is too low.

## Non-goals

- Personal productivity score.
- Individual leaderboard.
- Full DORA suite.
- AI spend or token analytics.
- Dashboard UI in the first slice.
- Precise agent attribution in the first slice.

Agent-related metrics should come after the base Closure Funnel. The next layer
is Delegation Quality: whether work marked as agent-eligible or agent-delegated
passed through the same funnel within its approval lane.
