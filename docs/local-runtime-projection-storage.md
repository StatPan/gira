# Local Runtime Projection Storage

This document resolves #786 under the broader #779 API-limit and provider
boundary work. It defines where future local runtime projections, cache
snapshots, and possible SQLite files may live without turning local state into
workflow truth.

API/provider cluster role: supporting storage boundary. Start with
[GitHub API Limit Operating Model](github-api-limits.md) for the current
entry point, then use this document when deciding where cache, projections,
runtime evidence, or future SQLite files may live.

Gira should use existing storage roots. It should not create a new product home,
hidden planning database, or local source of truth for issue completion.

## Decision

Use the existing storage map:

| Root | Default | Purpose | May contain |
| --- | --- | --- | --- |
| Durable config root | `~/.config/gira` | User-owned config and registries. | Global config, repo registry, workspace registry. |
| Runtime state root | `~/.config/gira/state` unless `paths.state_root` overrides it | Private runtime evidence that may not be reconstructible. | Run manifests, prompts, event logs, optional private audit indexes. |
| Gira cache root | `~/.cache/gira` | Disposable acceleration and projections. | Workspace status cache, provider snapshots, rebuildable projection indexes. |
| Repo-local contract | `.gira/config.yaml` | Optional shared repo policy. | Branch policy, labels, template/config contract. |

The shortest rule is:

```text
provider ledger = collaboration truth
runtime_state_root = private evidence that may not be rebuildable
gira_cache_root = rebuildable acceleration
```

## SQLite Placement

SQLite should not be introduced as a default dependency for this slice. If a
future accepted requirement justifies SQLite, place it according to rebuild
semantics:

| SQLite use | Location | Rebuild rule |
| --- | --- | --- |
| Rebuildable provider projection or query index | `gira_cache_root/projections/` | Safe to delete; rebuilt from provider ledgers, repo config, and durable receipts. |
| Rebuildable workspace status index | `gira_cache_root/workspace-status/` or `gira_cache_root/projections/workspaces/` | Safe to delete; rebuilt by fresh workspace reads. |
| Private runtime evidence index | `runtime_state_root/` under a documented subdirectory such as `runs/` or a future `audit/` | Not trusted for provider completion; deletion may lose private prompts, logs, or operator-only evidence. |
| Export-consumer convenience database | Operator-selected export path | Regenerable from export inputs; not read by lifecycle `--apply` commands. |

A SQLite file under `gira_cache_root` is a projection. A SQLite file under
`runtime_state_root` is private runtime evidence or an index over that evidence.
Neither location may become the source of truth for issue status, queue
membership, PR/MR readiness, completion, or merge state.

## Snapshot, Receipt, Projection, Cache

These terms must stay separate because they carry different trust levels.

| Term | Meaning | Trust boundary |
| --- | --- | --- |
| Snapshot | A provider read captured at a known time with source metadata. | Useful for read-only reports until stale; does not authorize mutation. |
| Receipt | Durable explanation of an accepted transition, handoff, supersede, or finish. | Provider-visible Gira receipts are lifecycle evidence. Local private receipts are operator evidence only. |
| Projection | Derived state or index optimized for query speed. | Rebuildable from provider ledgers, config, receipts, and selected private runtime evidence. |
| Cache | Disposable command acceleration. | Safe to delete; never authoritative. |
| Durable runtime state | Private local evidence produced by an explicit runtime. | May be non-rebuildable, but still cannot override provider truth. |

Provider comments remain the preferred location for workflow-significant
receipts because they travel with the provider ledger. Local private runtime
evidence can explain what an agent saw or did, but it cannot silently mark a
ticket done.

## Fresh, Stale, Offline

Local projection consumers should classify reads explicitly.

| State | Meaning | Allowed behavior |
| --- | --- | --- |
| Fresh | Provider data was verified within the command's freshness window and matches required policy inputs. | May support planning and, after dry-run approval, a mutating `--apply` path. |
| Stale | Projection or snapshot exists, but freshness has expired or provider state may have changed. | Read-only guidance, warnings, and next-step suggestions. |
| Offline | Provider cannot be contacted or permissions/rate limits prevent verification. | Read-only preview only. Mutating `--apply` paths fail closed or require human inspection after provider access returns. |
| Unknown | Gira cannot classify freshness or provenance. | Treat as unsafe for mutation. |

Every `--apply` path must verify the provider facts it depends on at the point
of mutation. A local projection may reduce the number of candidate provider
objects to inspect, but it cannot replace the final provider read for labels,
issue state, PR/MR state, checks, reviews, branch trust, or closing links.

## Provider Ledger Preservation

GitHub today, and GitLab in the next provider slice, remain the collaboration
ledgers. Local storage should preserve provider identifiers and provenance
metadata rather than flattening everything into one trusted state.

Projection records should carry at least:

- provider kind, such as `github` or `gitlab`;
- repository or project identifier;
- provider object type and provider object ID;
- fetched or derived timestamp;
- source command or contract version;
- provenance level when known: Gira-controlled, reconstructed, external drift,
  or unknown.

See [External Drift And Provenance Policy](external-drift-policy.md) and
[Provider Adapter Boundary](provider-adapter-boundary.md).

## Command Behavior

Use `gira config storage --repo OWNER/REPO` to inspect the resolved roots. That
diagnostic is the user-facing place to explain where Gira stores config, state,
cache, exports, and audit surfaces.

Daily commands may use cache or projections for compact read-only output when
freshness is clear. Deep freshness, rate-limit, and projection diagnostics
belong under ops or config/storage surfaces, not as new daily root commands.

Mutating commands must keep the existing dry-run/apply boundary:

1. `--dry-run` reports what provider facts will be verified and what mutations
   are planned.
2. `--apply` re-verifies the provider facts required by the dry-run plan.
3. If provider freshness fails, `--apply` fails closed instead of trusting a
   local projection.

## Non-Goals

- no SQLite schema in this slice;
- no migration from current cache files;
- no provider adapter implementation;
- no hosted control plane;
- no local database as source of truth;
- no offline mutation mode.
