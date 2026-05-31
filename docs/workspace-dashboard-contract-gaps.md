# Workspace Dashboard Contract Gaps

This document defines the contract gap between the existing workspace status
read model and the Gira 3.0 local report bundle.

Parent roadmap: #525. Research predecessor: #666. Contract slice: #667.
Implementation successor: #668.

## Decision

The first workspace dashboard export should bridge existing contracts instead
of inventing a new dashboard state model.

The source contracts are:

- `gira workspace status --json`
- `workspace-queues/v1` under `workspace_queues`
- `gira export dashboard --repo OWNER/REPO --output out/dashboard`
- `dashboard_export/v1alpha1`

The missing contract is a workspace snapshot layer inside the dashboard bundle.
That layer should copy or derive from workspace status output and then expose a
small index artifact for local HTML.

The dashboard must not recrawl GitHub from the browser, write a local database,
or make HTML the canonical state.

## Current Contracts

`workspace status --json` already answers the operator-level question:
what work exists across my inbox and execution repos?

Important fields:

- `workspace`
- `source`
- `config_path`
- `inbox`
- `repos`
- `counts`
- `workspace_queues`
- `rate_limit`
- `cache`
- `next_steps`
- `fetched_at`

`workspace_queues` already carries schema `workspace-queues/v1` and groups work
into:

- `agent_ready`
- `review_needed`
- `finish_ready`
- `blocked`
- `failed_check`
- `human_decision`

Each queue item already has the UI-critical fields: repo, issue, title, status,
labels, PR summary when known, evidence, reason codes, and
`next_safe_command`.

`dashboard_export/v1alpha1` already writes the repo-scoped machine bundle:

```text
out/dashboard/
  manifest.json
  raw/
    github.json
    transitions.json
    capabilities.json
  derived/
    execution_board.json
    roadmap_timeline.json
    warnings.json
  csv/
    execution_items.csv
    roadmap_items.csv
```

That bundle is deterministic and vendor-neutral, but it does not yet carry the
workspace queue surface needed by a local operator dashboard.

## Gap Summary

The 3.0 workspace report bundle needs these additions:

| Gap | Required artifact or field | Reason |
| --- | --- | --- |
| Workspace snapshot | `raw/workspace_status.json` | Preserve the read model used to build the UI. |
| Queue contract | `derived/workspace_queues.json` | Let HTML render queues without recomputing membership. |
| Dashboard index data | `derived/workspace_dashboard.json` | Give `index.html` one compact data source. |
| Queue CSV | `csv/workspace_queue_items.csv` | Keep spreadsheet and CLI-adjacent workflows cheap. |
| Human entry point | `index.html` | Make the bundle openable without a server. |
| Workspace status schema marker | `schema_version` or index `source.contract` | Identify the raw snapshot contract even if existing CLI output predates a top-level marker. |
| Stable warning codes | `warnings[].code` | Let UI and tests reason about degraded exports. |
| Manifest visibility | artifact entries for all files above | Let consumers discover what exists. |

## Required Bundle Additions

When workspace mode is available, the local report bundle should add:

```text
out/dashboard/
  raw/
    workspace_status.json
  derived/
    workspace_queues.json
    workspace_dashboard.json
  csv/
    workspace_queue_items.csv
  index.html
```

`raw/workspace_status.json` is the full workspace status payload. It may include
large arrays, rate-limit metadata, cache metadata, and backlog items. It is the
debuggable source snapshot.

`derived/workspace_queues.json` is the `workspace-queues/v1` object from
`workspace_status.workspace_queues`. It should not be recalculated by the HTML
renderer.

`derived/workspace_dashboard.json` is the compact presentation index. It should
contain enough information for the top-level HTML page to render summary cards,
queue counts, warnings, and the first safe actions.

`csv/workspace_queue_items.csv` is a flattened queue-item export for
spreadsheets, shell filtering, and quick inspection.

`index.html` is a static view over the JSON artifacts. It is not a source of
truth.

## Command Gap

The existing command is repo-scoped:

```bash
gira export dashboard --repo OWNER/REPO --output out/dashboard --dry-run
```

#668 should keep that path compatible and add a workspace export path:

```bash
gira export dashboard --config .gira/config.yaml --output out/dashboard --dry-run
gira export dashboard --config .gira/config.yaml --output out/dashboard
```

The first #668 implementation uses that `--config` path and supports `--repo`
as an optional workspace repo filter.

If neither `--repo` nor `--config` is provided, the command may resolve the
default global workspace the same way `gira workspace status` does. That is a
convenience path, not the required implementation for #668.

If `--repo` is provided without workspace context, the command should continue
to generate the existing repo bundle and omit workspace artifacts with a stable
warning code. That keeps existing automation compatible.

## `workspace-dashboard/v1alpha1`

`derived/workspace_dashboard.json` should use schema
`workspace-dashboard/v1alpha1`.

Suggested shape:

```json
{
  "schema_version": "workspace-dashboard/v1alpha1",
  "snapshot_at": "2026-05-31T09:59:14Z",
  "workspace": {
    "name": "personal",
    "owner": "OWNER"
  },
  "source": {
    "contract": "workspace-status/v1",
    "path": "raw/workspace_status.json"
  },
  "counts": {
    "backlog": 12,
    "repo_open": 34,
    "ready": 5,
    "in_progress": 2,
    "blocked": 1,
    "stale": 4
  },
  "queue_counts": {
    "agent_ready": 5,
    "review_needed": 1,
    "finish_ready": 1,
    "blocked": 1,
    "failed_check": 0,
    "human_decision": 2
  },
  "top_actions": [
    {
      "queue": "agent_ready",
      "repo": "OWNER/app",
      "issue": 12,
      "title": "Implement local workspace report bundle export",
      "reason_codes": ["ticket_ready"],
      "next_safe_command": "gira ticket start --repo OWNER/app --ticket 12 --apply",
      "source_refs": ["workspace_queue:agent_ready:OWNER/app#12", "issue:OWNER/app#12"]
    }
  ],
  "warnings": [
    {
      "code": "workspace_cache_stale",
      "severity": "warning",
      "message": "One or more repo summaries came from stale cache."
    }
  ],
  "artifacts": {
    "workspace_status": "raw/workspace_status.json",
    "workspace_queues": "derived/workspace_queues.json",
    "queue_items_csv": "csv/workspace_queue_items.csv"
  }
}
```

`workspace-status/v1` is the dashboard source contract name for the raw
workspace snapshot. If #668 does not add a top-level `schema_version` to
`workspace status --json`, the dashboard index must still identify the raw
snapshot with `source.contract`.

The top action list should be bounded. A local HTML index does not need every
queue item inline when `derived/workspace_queues.json` already carries the full
queue contract.

## Queue CSV

`csv/workspace_queue_items.csv` should use stable headers:

```text
queue,repo,issue,title,state,status,pr_number,pr_state,reason_codes,next_safe_command,url
```

Rules:

- One row per queue membership, not one row per issue.
- If an issue appears in two queues, write two rows.
- `reason_codes` should be joined with commas.
- Text values should be CSV-escaped by the standard CSV writer.
- URLs should point to GitHub issue or PR evidence when available.

## Warning Codes

The workspace dashboard layer should prefer structured warning objects.

Initial codes:

| Code | Severity | Meaning |
| --- | --- | --- |
| `workspace_context_missing` | `info` | Repo-scoped export ran without workspace context. |
| `workspace_status_unavailable` | `warning` | Workspace status could not be read, but repo export continued. |
| `workspace_rate_budget_low` | `warning` | API budget was present but not enough for an unrestricted refresh. |
| `workspace_cache_stale` | `warning` | Some repo summaries came from stale cache. |
| `workspace_queue_detail_incomplete` | `warning` | Queue detail status could not be fetched for some candidates. |
| `dashboard_html_omitted` | `info` | Machine bundle was generated without HTML. |

Existing prose warnings may remain in raw artifacts, but the dashboard index
should expose codes so HTML, tests, and downstream consumers can make stable
decisions.

## Manifest Rules

`manifest.json` remains the bundle entry point.

For workspace mode, it should list the workspace artifacts:

```json
{
  "path": "derived/workspace_dashboard.json",
  "kind": "derived_json",
  "will_write": true
}
```

`index.html` should use kind `html`.

If an artifact is intentionally omitted, the manifest should either omit it or
include it with `will_write: false` and a matching warning code. The first #668
implementation should choose one behavior and document it in tests.

## Implementation Order For #668

1. Add workspace export inputs to `gira export dashboard` without breaking
   existing `--repo` behavior.
2. Reuse `BuildWorkspaceStatusReportWithOptions` for the workspace snapshot.
3. Write `raw/workspace_status.json`.
4. Copy `workspace_queues` into `derived/workspace_queues.json`.
5. Build `derived/workspace_dashboard.json` from workspace counts, queue counts,
   warnings, next steps, and bounded top actions.
6. Write `csv/workspace_queue_items.csv`.
7. Add `index.html` only after JSON artifacts are deterministic.
8. Add tests for dry-run artifacts, apply writes, stable CSV headers, and
   repo-only compatibility.

The first implemented workspace bundle writes:

- `raw/workspace_status.json`
- `derived/workspace_queues.json`
- `derived/workspace_dashboard.json`
- `csv/workspace_queue_items.csv`
- `index.html`

## Non-Goals

#667 and #668 should not introduce:

- a SQLite state store
- a browser-side GitHub API client
- hosted dashboard infrastructure
- bidirectional dashboard edits
- user or agent productivity scoring
- dashboard-owned status fields
- a second workflow vocabulary for queue membership

## Acceptance Checklist

#668 can start when the implementation follows these decisions:

- `workspace-queues/v1` remains the queue membership source.
- `workspace-dashboard/v1alpha1` is only a compact presentation index.
- Workspace artifacts are listed in `manifest.json`.
- `index.html` renders exported JSON and does not fetch GitHub directly.
- Existing repo-scoped `gira export dashboard --repo` remains compatible.
- JSON remains canonical and HTML remains delete/regenerate presentation.
