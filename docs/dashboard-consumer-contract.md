# Dashboard Consumer Contract

Gira should expose a stable, vendor-neutral planning/export contract for downstream dashboards. A Notion dashboard is one intended consumer, but it must not be the only one. The same contract should support CSV-driven workflows, custom visualizations, BI tools, spreadsheets, and future thin UIs without moving canonical ownership away from GitHub and Google Calendar.

## Canonical Ownership

Keep one canonical owner per durable fact:

- **GitHub** owns execution state.
  - Issues, PRs, branches, milestones, labels, and closing semantics.
  - Product OS lifecycle evidence and transition planning inputs.
- **Google Calendar** owns time state.
  - Meetings, timeboxes, approved deadlines, and operator-facing calendar records.
- **Gira** owns normalization, validation, dry-run planning, and export.
  - It computes stable read models from GitHub and Calendar.
  - It must stay read-first and dry-run-first for Product OS slices.
- **Dashboards** own presentation only.
  - Notion, spreadsheets, CSV consumers, BI tools, and future thin UIs should not become the source of truth.

This split keeps the system auditable: execution remains GitHub-native, time remains Calendar-native, and the human-facing overview layer can change without rewriting the operating model.

## Sync Direction

Dashboard integration is **one-way only**.

```text
GitHub + Google Calendar -> Gira normalize/plan/export -> dashboard consumer
```

Requirements:

- No round-trip sync from Notion or any dashboard back into canonical execution state.
- No dashboard-owned status field that competes with GitHub/Calendar truth.
- Human edits in a dashboard are annotations or derived organization unless a future explicit write-back slice is approved.

## Desired Operator Experience

The intended operator feel is closer to Terraform than to a bespoke app backend:

- declare the desired Product OS shape
- inspect current state
- compute a dry-run diff/plan
- export a stable read model
- rerun safely

That means the export layer should be:

- deterministic
- idempotent to regenerate
- diffable in git or on disk
- safe to cache and republish

## Why The Contract Must Be Vendor-Neutral

Notion is a strong candidate for a polished human dashboard, but direct dashboard-specific integration creates unnecessary coupling:

- API limits and auth quirks vary by vendor.
- Rebuilding views in a different tool should not require rewriting Gira core.
- The same project data may need multiple consumers at once: Notion, Sheets, CSV pipelines, DuckDB, or custom visualization code.

Therefore the contract priority is:

1. stable JSON for automation and exported consumers
2. optional flat CSV for generic tools
3. stable human CLI text for operators

## API Budget Rule

The dashboard layer must not trigger a full GitHub/Calendar crawl on every page refresh.

Current Product OS reads already require multiple snapshot surfaces. For example, the transitions slice reads:

- issues
- pull requests
- branches
- milestones

and paginates those surfaces as repositories grow. The project sync slice also reads Projects v2 fields/items through GraphQL.

So the contract must assume:

- **full snapshot on every dashboard load is not acceptable**
- **export/cache first is the default path**
- on-demand live reads are acceptable only for small repos or explicit operator checks

## Recommended Export Architecture

```text
GitHub + Calendar
  -> Gira raw normalized snapshot
  -> Gira derived dashboard views
  -> JSON / CSV export artifacts
  -> Notion or other consumer reads exported artifacts
```

The export layer should separate two concerns.

### 1. Raw normalized export

This is the canonical machine-facing read model after normalization.

Suggested JSON groups:

- `issues`
- `pull_requests`
- `milestones`
- `project_items`
- `calendar_events`
- `calendar_timeboxes`
- `transitions`
- `capabilities`

Properties should stay close to canonical sources while normalizing names and timestamps.

### 2. Derived dashboard export

This is a convenience surface for dashboards so every consumer does not have to re-implement planning logic.

Suggested JSON groups:

- `roadmap_timeline`
- `execution_board`
- `owner_workload`
- `upcoming_deadlines`
- `upcoming_meetings`
- `transition_conflicts`
- `warnings`

This layer may denormalize data for convenience, but it must remain reproducible from canonical sources.

## CSV Is Explicitly Supported

Flat CSV export is a valid and recommended convenience surface.

Good CSV candidates:

- `roadmap_items.csv`
- `execution_items.csv`
- `owner_workload.csv`
- `calendar_windows.csv`
- `upcoming_events.csv`

Why CSV is useful:

- works with spreadsheets and BI tools immediately
- easy to load into pandas, DuckDB, SQLite, or notebooks
- reduces coupling to one dashboard vendor
- simplifies bulk inspection and ad-hoc filtering

CSV is not enough by itself because it is weak at representing:

- nested structures
- multi-rule conflict reasoning
- many-to-many relationships
- rich transition diagnostics

So the contract should be:

- **JSON is canonical**
- **CSV is convenience**

## Join Strategy Across GitHub And Calendar

A dashboard needs a stable way to show execution and time together.

The contract should support at least one durable join strategy:

- issue or PR number referenced in the calendar event metadata or description
- milestone or phase title mapping
- explicit exported `source_refs` arrays on derived dashboard items

A future implementation may support richer join metadata, but the first contract must at least define how a dashboard can relate a GitHub work item to a calendar block or meeting without scraping human prose heuristically on every render.

## Minimum Consumer Guarantees

Any dashboard consumer should be able to rely on these guarantees:

1. Stable top-level object names and key casing.
2. Deterministic timestamps and ordering.
3. No extra prose on JSON stdout for CLI `--json` modes.
4. One-way export semantics.
5. Counts and warning surfaces for incomplete/ambiguous planning data.
6. Stable identifiers for issues, PRs, milestones, and derived dashboard rows.

## Initial Command Shape

The first contract does not require a direct Notion sync command.

Preferred evolution:

- existing read/plan commands continue to mature
- a future export command writes vendor-neutral artifacts
- dashboards consume exported files or cached JSON

Representative future surfaces:

```text
gira project transitions --repo OWNER/REPO --dry-run --json
gira project sync --repo OWNER/REPO --dry-run --json
gira export dashboard --repo OWNER/REPO --format json --output ./out/
gira export dashboard --repo OWNER/REPO --format csv --output ./out/
```

The exact command names may change, but the contract should remain export-first rather than dashboard-vendor-first. The first concrete bundle layout and artifact boundaries are specified in [dashboard-export-artifacts.md](dashboard-export-artifacts.md).

## Non-Goals Of This Contract

- direct Notion page/database mutation as part of Gira MVP core
- a web UI owned by Gira core
- bidirectional status sync
- dashboard-side canonical state
- mandatory live API reads on every dashboard refresh

## Practical Recommendation

Use Notion as a polished first presentation layer if it proves useful, but build the Gira contract so the same data can be consumed by:

- Notion
- Google Sheets
- Airtable-like tools
- DuckDB/pandas pipelines
- custom plots or dashboards
- future lightweight apps

That keeps Gira focused on the durable layer: setup, validation, planning, and export.