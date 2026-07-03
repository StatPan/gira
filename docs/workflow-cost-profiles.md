# Workflow Cost Profiles

This document defines the first static cost model for #779 and #783. The
numbers are conservative planning estimates, not measured rolling averages.

API/provider cluster role: supporting cost table. Start with
[GitHub API Limit Operating Model](github-api-limits.md) for the current
entry point, then use this document when calculating safe run-count estimates.

The model exists so Gira can later answer "how many more times can I run this
flow?" without pretending that GitHub is an unlimited database. The estimates
are intentionally fixed for the first implementation slice.

`gira ops limit --workflow NAME` uses the conservative profile by default. It
computes safe remaining runs as:

```text
floor((remaining primary bucket * 80%) / conservative profile cost)
```

The lowest measurable bucket result becomes `safe_runs` and
`limiting_bucket`. Write/content cost remains visible, but secondary
write/content budget is not directly measurable by GitHub.

## Buckets

| Bucket | Meaning |
| --- | --- |
| REST core | Expected GitHub REST core requests. |
| GraphQL | Expected GitHub GraphQL primary points. |
| Search | Expected GitHub search bucket requests. |
| Write/content | Expected issue, PR, label, comment, merge, close, or similar content-generation pressure. This is not directly observable through `gh api rate_limit`; it represents secondary-limit risk. |

## Modes

Each profile has two estimates:

- `optimistic`: a clean path with no repeated polling, fallback reads, or extra
  reconciliation.
- `conservative`: the default model used by operators and agents. It includes
  normal fallback, linked PR/check inspection, and bounded retry or polling
  pressure.

GitHub App authentication is blocked/future for this model. These profiles
assume the current `gh` user-token operating path and must not assume higher
installation-token budgets.

## Static Profiles

| Profile | Commands | Conservative REST | Conservative GraphQL | Conservative Search | Conservative write/content | Optimistic REST | Optimistic GraphQL | Optimistic Search | Optimistic write/content |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `status` | `gira status`, `gira stats repo` | 6 | 2 | 0 | 0 | 3 | 1 | 0 | 0 |
| `workspace-status` | `gira workspace status` | 40 | 8 | 0 | 0 | 15 | 3 | 0 | 0 |
| `queue-next` | `gira queue next`, `gira queue handoff` | 24 | 4 | 1 | 0 | 10 | 2 | 1 | 0 |
| `queue-take` | `gira queue take` | 35 | 6 | 1 | 3 | 18 | 3 | 1 | 2 |
| `ticket-status-view` | `gira ticket status`, `gira ticket view` | 18 | 4 | 0 | 0 | 8 | 2 | 0 | 0 |
| `ticket-lifecycle` | `gira ticket start`, `gira ticket pr`, `gira ticket self-review`, `gira ticket checks`, `gira ticket wait`, `gira ticket finish` | 110 | 8 | 1 | 12 | 60 | 4 | 0 | 7 |

The `ticket-lifecycle` GraphQL estimate excludes the legacy rich PR readiness
fallback. By default, linked PR readiness uses REST timeline/list, review,
check-run, and commit-status endpoints and fails closed when REST lookup cannot
resolve the PR. The hidden `GIRA_DEV_PR_GRAPHQL_FALLBACK=1` compatibility switch
is not part of the normal cost model.
| `goal-status-next` | `gira goal status`, `gira goal next` | 45 | 8 | 1 | 0 | 18 | 4 | 0 | 0 |

## Boundaries

The model is deliberately static:

- no dynamic measurement;
- no rolling average;
- no SQLite or local projection dependency;
- no provider adapter abstraction;
- no assumption that an estimate authorizes a mutation.

Mutating commands still require fresh provider verification at the point of
`--apply`. A budget estimate is a planning signal, not approval evidence.
