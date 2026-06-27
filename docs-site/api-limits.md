# API Limits

Gira remains GitHub-native, but GitHub API budget is a product constraint. The
CLI should not assume GitHub can be polled like an unlimited runtime database.

This page summarizes the limit buckets Gira must model before adding workflow
run-count estimates, local projections, or provider-specific runtime behavior.

## Buckets

| Bucket | Current GitHub limit shape | Gira implication |
| --- | --- | --- |
| REST core | Authenticated user requests generally share a 5,000 requests/hour user budget. | Most `gh api`, `gh issue`, and `gh pr` reads and writes consume this budget or a related REST bucket. |
| GraphQL primary | Authenticated user requests generally share a 5,000 points/hour user budget. | Project v2, permission, status, and rich PR queries can exhaust a separate budget from REST core. |
| Search | Search endpoints have a more restrictive bucket than normal REST core. | Search-backed workflows need separate accounting. |
| Content creation | GitHub applies secondary limits to content-generating requests. | Ticket lifecycle commands can be blocked even when primary budgets look healthy. |
| Secondary limits | Concurrency, per-endpoint points/minute, CPU time, content creation, and undisclosed protections are separate from primary hourly budgets. | Gira must detect symptoms and back off or fail closed for risky mutations. |

Sources:

- [GitHub REST API rate limits](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api)
- [GitHub GraphQL rate and query limits](https://docs.github.com/en/graphql/overview/rate-limits-and-query-limits-for-the-graphql-api)

## Authentication

The CLI-first product path currently operates through the authenticated GitHub
identity exposed by `gh`. The cost model must not assume GitHub App
installation budgets until Gira has an explicit GitHub App authentication
design.

| Auth class | Current modeling status |
| --- | --- |
| User token / PAT via `gh` | Current default. Model REST and GraphQL as user-scoped primary budgets. |
| `GITHUB_TOKEN` | CI-specific and lower than normal user budgets for non-Enterprise repositories. |
| GitHub App installation token | Future/blocking decision. Do not assume this budget in current CLI estimates. |

## Observable State

Gira can observe primary limits through response headers, `gh api rate_limit`,
and GraphQL `rateLimit` fields.

Gira cannot directly query remaining secondary limit budget. It can only
observe symptoms such as HTTP `403` or `429`, `retry-after` headers,
`x-ratelimit-remaining: 0`, and GraphQL errors.

## Modeling Rules

1. Count REST core, GraphQL, search, and write/content pressure separately.
2. Treat secondary limits as guarded but not precomputable.
3. Start with static workflow profiles for common Gira flows.
4. Prefer conservative estimates when showing remaining safe workflow runs.
5. Keep daily command output compact. Detailed diagnostics belong under
   `gira ops limit`.
6. Never let a budget estimate authorize a mutation. `--apply` commands still
   need fresh provider verification.
7. Do not assume GitHub App installation budgets until a separate auth design
   resolves permissions, setup, token storage, and operator migration.

## Boundary

```text
GitHub = canonical collaboration ledger
Gira CLI/MCP = supported workflow gateway
Local state/cache = acceleration, receipts, and projections
```

GitHub rate limits are not a reason to move completion truth into local state.
They are a reason to make Gira commands budget-aware, cache-aware, and explicit
about freshness before future local runtime projection work.

See [Command Surface](command-surface.md) for why API budget diagnostics use the
`gira ops limit` surface instead of a new root command.
