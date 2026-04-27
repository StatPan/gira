# Dashboard Export Artifacts

This document defines the first concrete artifact contract for Gira dashboard exports.

It turns the vendor-neutral dashboard boundary from `docs/dashboard-consumer-contract.md` into an implementable file/output shape for downstream consumers.

## Contract Goals

The artifact contract should let Gira produce one stable export bundle that downstream tools can consume without becoming canonical owners.

The bundle must be:

- one-way
- export-first
- vendor-neutral
- deterministic
- safe to regenerate
- readable by both machines and operators

## Canonical Ownership Reminder

- **GitHub** remains the execution canonical source.
- **Google Calendar** remains the time canonical source.
- **Gira** owns normalization, planning, and export.
- **Dashboards** consume exported artifacts only.

## Output Root

A dashboard export writes into one output root chosen by the operator.

Example:

```text
out/dashboard/
```

All first-version artifacts should live under that root so the bundle is easy to archive, diff, publish, or ingest elsewhere.

## Recommended Layout

```text
out/dashboard/
  manifest.json
  raw/
    github.json
    calendar.json
    transitions.json
    capabilities.json
  derived/
    execution_board.json
    roadmap_timeline.json
    owner_workload.json
    upcoming_deadlines.json
    upcoming_meetings.json
    warnings.json
  csv/
    execution_items.csv
    roadmap_items.csv
    owner_workload.csv
    upcoming_events.csv
```

Not every file must exist on day one, but the layout should be stable from the first implementation so later slices can add content without reorganizing the bundle.

## Versioning

Every export bundle must carry a schema version.

Initial rule:

- `schema_version` is required in `manifest.json`
- major shape changes must increment the version
- additive fields are preferred over breaking renames
- canonical artifacts should prefer snapshot-derived metadata over wall-clock-only run metadata when determinism matters

Suggested initial version:

```text
v1alpha1
```

## Manifest

`manifest.json` is the required entry point for the bundle.

Minimum keys:

- `schema_version`
- `snapshot_at`
- `repo`
- `sources`
- `artifacts`
- `generator`

Suggested shape:

```json
{
  "schema_version": "v1alpha1",
  "snapshot_at": "2026-04-27T14:29:50Z",
  "repo": "StatPan/gira",
  "sources": {
    "github": {
      "included": true,
      "snapshot_at": "2026-04-27T14:29:50Z"
    },
    "google_calendar": {
      "included": false,
      "snapshot_at": null,
      "reason": "not_enabled"
    }
  },
  "artifacts": [
    {"path": "raw/github.json", "kind": "raw_json"},
    {"path": "derived/execution_board.json", "kind": "derived_json"},
    {"path": "csv/execution_items.csv", "kind": "csv"}
  ],
  "generator": {
    "name": "gira",
    "mode": "dashboard_export"
  }
}
```

`snapshot_at` should represent the canonical source snapshot time that the export bundle is derived from. Implementations may log a separate wall-clock run time elsewhere, but canonical JSON artifacts should not require volatile per-run timestamps when the source snapshot is unchanged.

## Raw JSON Layer

The raw layer is the canonical machine-readable export surface.

Design rules:

- stay close to canonical source semantics
- normalize timestamps and identifiers
- avoid presentation-specific denormalization here
- preserve enough metadata to rebuild derived views deterministically

### `raw/github.json`

Minimum top-level keys:

- `repo`
- `snapshot_at`
- `issues`
- `pull_requests`
- `milestones`

Optional expansion keys:

- `project_items`

Suggested per-record identifier rules:

- issues: `issue_number` and `issue_id`
- PRs: `pr_number` and `pr_id`
- milestones: `milestone_number` and `milestone_id`
- project items, when exported in a later slice: stable exported `project_item_id`

### `raw/calendar.json`

Calendar support may be absent in the first implementation, but the contract should reserve its place.

Minimum top-level keys:

- `snapshot_at`
- `included`
- `events`
- `timeboxes`
- `warnings`

If calendar export is not enabled yet, the file may be omitted and represented in `manifest.json` as `included: false`.

### `raw/transitions.json`

This file carries lifecycle planning evidence already aligned with the Product OS direction.

Minimum top-level keys:

- `snapshot_at`
- `repo`
- `transitions`
- `conflicts`
- `warnings`

Each transition entry should preserve:

- `rule_id`
- `subject_type`
- `subject_id`
- `from`
- `to`
- `reason`
- `conflict_resolution`

### `raw/capabilities.json`

This file records what the active credential could inspect or mutate at export time.

Minimum top-level keys:

- `snapshot_at`
- `repo`
- `capabilities`
- `blocked_actions`
- `warnings`

## Derived JSON Layer

The derived layer is for dashboard consumers that should not have to rebuild planning logic themselves.

Derived artifacts may denormalize, but they must remain reproducible from canonical/raw inputs.

### `derived/execution_board.json`

Purpose:
- one row-like object per active execution item
- optimized for kanban/table consumers

Minimum top-level keys:

- `snapshot_at`
- `repo`
- `items`
- `warnings`

Per-item suggested keys:

- `id`
- `kind`
- `title`
- `status`
- `priority`
- `owner`
- `milestone`
- `target_date`
- `source_refs`

### `derived/roadmap_timeline.json`

Purpose:
- timeline/roadmap consumers
- milestone and dated work slices

Minimum top-level keys:

- `snapshot_at`
- `repo`
- `items`
- `warnings`

Per-item suggested keys:

- `id`
- `title`
- `start_date`
- `target_date`
- `status`
- `phase`
- `source_refs`

### `derived/owner_workload.json`

Purpose:
- summarize active work by person/agent

Minimum top-level keys:

- `snapshot_at`
- `repo`
- `owners`
- `warnings`

### `derived/upcoming_deadlines.json`

Purpose:
- compact deadline-centric view

Minimum top-level keys:

- `snapshot_at`
- `repo`
- `items`
- `warnings`

### `derived/upcoming_meetings.json`

Purpose:
- compact calendar-centric view

Minimum top-level keys:

- `snapshot_at`
- `items`
- `warnings`

### `derived/warnings.json`

Purpose:
- centralized machine-readable warning surface

Minimum top-level keys:

- `snapshot_at`
- `repo`
- `warnings`

## CSV Layer

CSV is a convenience export, not the canonical contract.

Recommended first files:

- `csv/execution_items.csv`
- `csv/roadmap_items.csv`
- `csv/owner_workload.csv`
- `csv/upcoming_events.csv`

### CSV Rules

- UTF-8
- header row required
- deterministic column order
- flattened scalar fields only
- nested values must be converted explicitly or omitted

Recommended first CSV columns for `execution_items.csv`:

- `id`
- `kind`
- `number`
- `title`
- `status`
- `priority`
- `owner`
- `milestone`
- `start_date`
- `target_date`
- `url`

## Join Strategy

Dashboard consumers need stable joins across execution and time views.

Minimum guidance:

- every derived row should carry a stable `id`
- every derived row should carry `source_refs`
- `source_refs` should contain canonical references such as:
  - `github:issue:42`
  - `github:pr:41`
  - `github:milestone:beta`
  - `calendar:event:abc123`

This avoids forcing every downstream consumer to reverse-engineer joins from prose.

## Determinism Rules

Exports should be diff-friendly.

Required rules:

- stable key casing
- stable field names
- ISO 8601 timestamps
- deterministic item ordering
- explicit nulls where fields are known-but-empty
- no mixed prose on JSON stdout when `--json` is requested

Recommended ordering:

- sort issues and PRs by number ascending
- sort milestones by due date, then title
- sort derived timeline rows by `start_date`, then `target_date`, then `id`
- sort warnings by severity, then code, then subject id

## Dry-Run Expectations

A future `gira export dashboard --dry-run` should preview:

- output root
- artifact paths
- source inclusion/exclusion
- record counts where available
- warnings about missing calendar support or incomplete joins

Dry-run should not write any files.

## First Implementation Boundary

The first implementation may be GitHub-only while reserving calendar fields and paths in the contract.

That means the first shipped export command may produce:

- `manifest.json`
- `raw/github.json`
- `raw/transitions.json`
- `raw/capabilities.json`
- `derived/execution_board.json`
- `derived/roadmap_timeline.json`
- `derived/warnings.json`
- `csv/execution_items.csv`
- `csv/roadmap_items.csv`

and mark calendar support as not included yet.

## Non-Goals

This contract does not define:

- direct Notion writes
- a Gira-owned UI
- bidirectional sync
- background refresh scheduling
- vendor-specific dashboard schema quirks

Those can be layered on later without changing canonical ownership.
