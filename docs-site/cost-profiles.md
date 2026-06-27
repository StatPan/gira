# Cost Profiles

Gira uses fixed workflow cost profiles to reason about expected GitHub API
budget usage. These profiles are conservative planning data, not dynamic
measurements.

`gira ops limit --workflow NAME` uses the conservative profile by default and
computes safe remaining runs as `floor((remaining primary bucket * 80%) /
conservative profile cost)`. The lowest measurable bucket becomes the limiting
bucket. Write/content cost is reported, but secondary budget is not directly
measurable.

## Buckets

| Bucket | Meaning |
| --- | --- |
| REST core | Expected GitHub REST core requests. |
| GraphQL | Expected GitHub GraphQL primary points. |
| Search | Expected GitHub search bucket requests. |
| Write/content | Expected issue, PR, label, comment, merge, close, or similar secondary-limit pressure. |

## Modes

`conservative` is the default. It includes normal fallback, linked PR/check
inspection, and bounded retry or polling pressure. `optimistic` models a clean
path with no repeated polling or fallback reads.

GitHub App authentication remains a future/blocking decision. Current profiles
assume the `gh` user-token operating path.

## Static Profiles

| Profile | Conservative REST | Conservative GraphQL | Conservative Search | Conservative write/content | Optimistic REST | Optimistic GraphQL | Optimistic Search | Optimistic write/content |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `status` | 6 | 2 | 0 | 0 | 3 | 1 | 0 | 0 |
| `workspace-status` | 40 | 8 | 0 | 0 | 15 | 3 | 0 | 0 |
| `queue-next` | 24 | 4 | 1 | 0 | 10 | 2 | 1 | 0 |
| `queue-take` | 35 | 6 | 1 | 3 | 18 | 3 | 1 | 2 |
| `ticket-status-view` | 18 | 4 | 0 | 0 | 8 | 2 | 0 | 0 |
| `ticket-lifecycle` | 110 | 24 | 1 | 12 | 60 | 12 | 0 | 7 |
| `goal-status-next` | 45 | 8 | 1 | 0 | 18 | 4 | 0 | 0 |

## Boundaries

The model does not use dynamic measurement, rolling averages, SQLite, local
projection storage, provider adapters, or GitHub App budgets.

Mutating commands still require fresh provider verification at `--apply`.
