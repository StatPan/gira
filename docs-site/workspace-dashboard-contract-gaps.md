# Workspace Dashboard Contract Gaps

The first Gira 3.0 dashboard implementation should bridge existing workspace
and dashboard export contracts instead of inventing a new state model.

Source document:
[`docs/workspace-dashboard-contract-gaps.md`](https://github.com/StatPan/gira/blob/main/docs/workspace-dashboard-contract-gaps.md).

## Decision

The workspace dashboard bundle should use these source contracts:

- `gira workspace status --json`
- `workspace-queues/v1`
- `gira export dashboard`
- `dashboard_export/v1alpha1`

The missing piece is a workspace snapshot layer in the dashboard bundle. HTML
should render that exported snapshot and should not recrawl GitHub or own
workflow state.

## Required Artifacts

Workspace mode should add:

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

`workspace_queues.json` keeps queue membership from `workspace-queues/v1`.
`workspace_dashboard.json` is the compact index for the local HTML page.

## Command Direction

Keep the existing repo export compatible:

```bash
gira export dashboard --repo OWNER/REPO --output out/dashboard --dry-run
```

Add workspace export:

```bash
gira export dashboard --config .gira/config.yaml --output out/dashboard --dry-run
gira export dashboard --config .gira/config.yaml --output out/dashboard
```

The first implementation uses that `--config` path and supports `--repo` as an
optional workspace repo filter. Repo-only export remains compatible.

## New Index Contract

`derived/workspace_dashboard.json` should use
`workspace-dashboard/v1alpha1` and include:

- workspace identity
- snapshot source path
- workspace counts
- queue counts
- bounded top actions
- structured warning codes
- artifact paths

The full queue items stay in `derived/workspace_queues.json`.

## Warning Codes

Initial warning codes:

| Code | Meaning |
| --- | --- |
| `workspace_context_missing` | Repo export ran without workspace context. |
| `workspace_status_unavailable` | Workspace status could not be read. |
| `workspace_rate_budget_low` | API budget is too low for unrestricted refresh. |
| `workspace_cache_stale` | Some repo summaries came from stale cache. |
| `workspace_queue_detail_incomplete` | Some queue detail reads were unavailable. |
| `dashboard_html_omitted` | Machine bundle was generated without HTML. |

## Implementation Order

1. Add workspace export inputs without breaking `--repo`.
2. Reuse workspace status as the snapshot source.
3. Write workspace raw, derived, and CSV artifacts.
4. Add deterministic `index.html` over exported JSON.
5. Test dry-run artifacts, apply writes, CSV headers, and repo-only
   compatibility.
