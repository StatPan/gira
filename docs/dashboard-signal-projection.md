# Dashboard Signal Projection Strategy

This document decides how Gira should project local dashboard signals before it
adds any local database, hosted dashboard, or broader GitHub Projects view
automation.

Milestone: 3.3 Dashboard Signal Integration. Planning ticket: #711.
Implementation successor: #712.

## Decision

Gira should extend the local dashboard export bundle before adding GitHub
Projects view automation or SQLite.

The next slice is:

```bash
gira export dashboard --config .gira/config.yaml --output out/dashboard --dry-run
gira export dashboard --config .gira/config.yaml --output out/dashboard
```

The bundle should add `pulse` and `storage` artifacts as derived, read-only
operator signals. GitHub remains the execution source of truth. Local files
remain cache, config, private run evidence, or regenerable export artifacts.

## Projection Order

| Order | Surface | Role | Reason |
| --- | --- | --- | --- |
| 1 | CLI JSON and text | Source contract for computed Gira state. | Commands are testable, scriptable, and already dry-run/apply aware. |
| 2 | Local dashboard export | Regenerable operator snapshot. | Gives a dashboard-like view without a daemon, hosted service, or local DB. |
| 3 | GitHub Projects native fields | Shared visibility over GitHub issues. | Useful after the export contract proves which fields are worth projecting. |
| 4 | SQLite or local index | Query acceleration and history only. | Justified only when repeated cross-snapshot queries cannot be answered cheaply from GitHub plus exported artifacts. |

## Signal Map

| Signal | Current source | Dashboard artifact | GitHub Projects projection | SQLite threshold |
| --- | --- | --- | --- | --- |
| Workspace status | `workspace status --json` | `raw/workspace_status.json` and `derived/workspace_dashboard.json` | None beyond existing issue membership and labels. | Not needed unless operators need fast historical workspace comparisons across many snapshots. |
| Queue state | `workspace-queues/v1` | `derived/workspace_queues.json`, `csv/workspace_queue_items.csv`, HTML queue sections. | Mirror only canonical issue status and planning labels through existing `projects sync`. | Not needed unless queue history must answer "how long did this sit in queue?" without GitHub event scans. |
| Pulse | `pulse-report/v1alpha1` from `gira stats pulse` | `derived/workspace_pulse.json`, optional `csv/workspace_pulse_items.csv`, compact HTML summary. | Do not create score or rank fields. A future native view may filter recent issue movement with existing dates/labels only. | Needed only for rolling trend windows that compare many pulse snapshots without rereading GitHub events. |
| Storage diagnostics | `config-storage-report/v1` from `gira config storage` | `derived/storage_diagnostics.json` plus a compact HTML boundary summary. | None. Storage layout is local/operator state, not team execution state. | Not needed; the report is current-state diagnostics, not query history. |
| Run evidence | `gira run` manifests, prompts, events, stderr, and results under private local state. | Do not copy private run contents. Export may include counts, paths, privacy class, and rebuild/source-of-truth classification from storage diagnostics. | None. Private local evidence must not be mirrored into Projects. | Needed only if an operator explicitly asks for local run audit search across many runs, and privacy policy is defined first. |
| GitHub Projects | `workspace.project` plus `projects sync` reports. | Optional `raw/projects_sync.json` or warning summary after Projects sync is run. | Existing native projection: Status, Priority, Area, Agent, Target date, and issue membership. | Not needed for native Projects visibility. |

## Dashboard Bundle Additions

The next no-DB dashboard slice should add these artifacts:

```text
out/dashboard/
  derived/
    workspace_pulse.json
    storage_diagnostics.json
  csv/
    workspace_pulse_items.csv
```

`derived/workspace_dashboard.json` may include compact references:

```json
{
  "pulse": {
    "schema_version": "pulse-report/v1alpha1",
    "path": "derived/workspace_pulse.json",
    "summary": {
      "finished": 2,
      "reviewed": 1,
      "started": 2,
      "blocked": 0
    }
  },
  "storage": {
    "schema_version": "config-storage-report/v1",
    "path": "derived/storage_diagnostics.json",
    "summary": {
      "source_of_truth": "github_plus_config",
      "local_database": false,
      "private_run_evidence_included": false
    }
  }
}
```

The HTML index should show only concise summaries and links to exported JSON.
It must not render private run prompts, stderr, model outputs, or detailed local
run event logs.

## GitHub Projects Boundary

GitHub Projects should stay a visibility surface over GitHub issues, not a
second dashboard database.

The 3.4 native view adoption decision is captured in
[GitHub Projects Native View Boundary](github-projects-native-view-boundary.md).

The minimal native projection is the current `projects sync` shape:

- issue membership
- Status from Gira status labels
- Priority from `priority:*`
- Area from `area:*`
- Agent from `agent:*`
- Target date from milestone due dates

For 3.3, do not add Projects fields for pulse, storage, dashboard warnings, run
evidence, or local cache freshness. Those are computed operator diagnostics.
They belong in CLI/export artifacts until a concrete team view proves otherwise.

GitHub also does not expose supported Project view creation APIs for the full
manual view setup Gira wants. Gira should continue reporting manual Board or
Schedule view setup steps instead of hiding view automation behind unsupported
raw API behavior.

## SQLite Threshold

SQLite should not be introduced as a default dependency for 3.3.

It becomes justified only when at least one of these requirements is accepted:

- Query queue residence time across many snapshots without rereading GitHub
  events every time.
- Compare pulse trends across many days, repos, or workspaces with bounded
  latency.
- Search private local run audit records across many runs after a privacy model
  and retention policy are documented.
- Support an offline dashboard that must answer historical questions after the
  original GitHub evidence is unavailable.
- Maintain a local index that can be deleted and fully rebuilt from GitHub,
  config, receipts, and explicitly retained private run evidence.

Even then, SQLite should be an index or audit store, not the source of truth for
issue status, completion, priority, queue membership, or Projects fields.

## Follow-Up Work

#712 is the first implementation ticket. It should add pulse and storage
artifacts to `gira export dashboard` workspace mode, update the manifest and
HTML summary, and preserve the no-DB boundary.

Future planning can evaluate a Projects-native view adoption slice after #712
ships and the local export proves which dashboard fields are useful in daily
operation.
