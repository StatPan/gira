# GitHub API Limit Operating Model

This document is the #779 and #780 baseline for rate-limit-aware Gira work. It
records the GitHub API limits Gira must model before adding workflow run-count
estimates, local projections, or provider-specific runtime behavior.

Gira should treat GitHub as the canonical collaboration ledger, but it should
not assume GitHub can be polled like an unlimited runtime database. API budget
is an operating constraint that daily commands, agent queues, and future local
projection work must respect.

## Buckets Gira Must Model

| Bucket | Current GitHub limit shape | Gira implication |
| --- | --- | --- |
| REST core | Authenticated user requests generally share a 5,000 requests/hour user budget. | Most `gh api`, `gh issue`, and `gh pr` reads and writes consume this budget or a related REST bucket. |
| GraphQL primary | Authenticated user requests generally share a 5,000 points/hour user budget. Installation tokens can scale differently. | Project v2, permission, status, and rich PR queries can exhaust a separate budget from REST core. |
| Search | Search endpoints have a more restrictive bucket than normal REST core. | Any workflow that uses GitHub search must be counted separately from normal issue/PR reads. |
| Content creation | GitHub applies secondary limits to content-generating requests, including issue comments, PR comments, labels, closes, and similar mutations. | Ticket lifecycle commands can be blocked even when primary REST and GraphQL budgets still look healthy. |
| Secondary limits | Concurrency, per-endpoint points/minute, CPU time, content creation, and undisclosed abuse protections are enforced separately from primary hourly budgets. | Gira cannot fully precompute this budget. It must detect symptoms and back off or fail closed for risky mutations. |

Sources:

- GitHub REST API rate limits:
  <https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api>
- GitHub GraphQL rate and query limits:
  <https://docs.github.com/en/graphql/overview/rate-limits-and-query-limits-for-the-graphql-api>

## Authentication Classes

Gira's CLI-first product path currently operates through the authenticated
GitHub identity exposed by `gh`. The cost model must not assume GitHub App
installation budgets until Gira has an explicit GitHub App authentication
design.

| Auth class | Current modeling status | Notes |
| --- | --- | --- |
| Unauthenticated | Out of scope for normal Gira operations. | Too small for agent or workspace workflows. |
| User token / PAT via `gh` | Current default. | Model REST and GraphQL as user-scoped primary budgets. |
| OAuth app user token | Same user-budget class for Gira's current purposes. | Higher Enterprise Cloud cases exist but should not be assumed. |
| `GITHUB_TOKEN` | CI-specific and lower than normal user budgets for non-Enterprise repositories. | Relevant for CI jobs that run Gira, not for local operator defaults. |
| GitHub App installation token | Future/blocking decision. | Can scale by installation, repo count, user count, and Enterprise context, but adopting it changes auth, permission, and distribution assumptions. |

## Observable And Unobservable State

Gira can observe primary limits with:

- response headers such as `x-ratelimit-limit`, `x-ratelimit-remaining`,
  `x-ratelimit-used`, `x-ratelimit-reset`, and `x-ratelimit-resource`;
- `GET /rate_limit`, usually through `gh api rate_limit`;
- GraphQL `rateLimit` fields for GraphQL-specific budget and query cost.

Gira cannot directly query the remaining secondary limit budget. It can only
observe symptoms:

- HTTP `403` or `429`;
- GraphQL errors that report rate-limit exhaustion;
- `retry-after` headers;
- `x-ratelimit-remaining: 0`;
- repeated failures from endpoint or content-generation pressure.

When secondary-limit symptoms occur, Gira should surface a specific diagnostic
and avoid blind retry loops. For mutating commands, continuing after a
secondary-limit response risks duplicate comments, partial lifecycle state, or
provider-side integration penalties.

Default PR readiness inspection must avoid GraphQL-heavy fallback queries.
`DevPRStatus` uses REST issue timelines, REST PR lookup/listing, REST reviews,
REST check-runs, and REST commit statuses by default. If those REST paths cannot
resolve a linked PR, Gira fails closed with a missing/unknown linked PR state
instead of spending GraphQL budget on `statusCheckRollup`-style `gh pr list`
queries. The hidden `GIRA_DEV_PR_GRAPHQL_FALLBACK=1` compatibility switch is
outside the default operating model and should not be used for normal agent
polling.

`gira ops limit --workflow NAME` estimates remaining safe workflow runs by
dividing current primary bucket budget by the static conservative workflow
profile, then applying an 80% safety factor. The estimate identifies the
limiting measurable bucket among REST core, GraphQL, and search. Write/content
pressure is shown in the selected profile, but it is not directly measurable
and therefore cannot be used as a numeric remaining-run limit.

## Modeling Rules

Use these rules for follow-up implementation slices:

1. Count REST core, GraphQL, search, and write/content pressure separately.
2. Treat secondary limits as guarded but not precomputable.
3. Start with static workflow profiles for common Gira flows. Do not rely on
   dynamic averages for the first cost model. See
   [Workflow Cost Profiles](workflow-cost-profiles.md).
4. Warn when a visible primary bucket is exhausted or at/below 10% remaining.
5. Prefer conservative estimates when showing remaining safe workflow runs.
6. Keep daily command output compact. Detailed diagnostics belong under
   `gira ops limit`.
7. Never let a budget estimate authorize a mutation. `--apply` commands still
   need fresh provider verification at the point of mutation.
8. Do not assume GitHub App installation budgets until a separate auth design
   resolves permissions, setup, token storage, and operator migration.

## Product Boundary

This model preserves the existing Gira state boundary:

```text
GitHub = canonical collaboration ledger
Gira CLI/MCP = supported workflow gateway
Local state/cache = acceleration, receipts, and projections
```

GitHub rate limits are not a reason to move completion truth into local state.
They are a reason to make Gira commands budget-aware, cache-aware, and explicit
about freshness before future local runtime projection work.

See [Command Surface Boundary](command-surface-boundary.md) for why API budget
diagnostics use the `gira ops limit` surface instead of a new root command.
See [Workflow Cost Profiles](workflow-cost-profiles.md) for the fixed profile
data used by follow-up run-count estimates.
