# Gira 3.0 Local Report Bundle UX

This document records the first Gira 3.0 UX decision after the 2.0 CLI
control-plane release.

Gira 3.0 should start with a local report bundle, not a hosted dashboard. The
bundle is a file-based UI surface generated from existing Gira state contracts.
It makes workflow state visible without moving canonical ownership away from
GitHub or replacing the CLI execution loop.

Parent roadmap: #525. First research ticket: #666.

## Decision

The first Gira 3.0 surface should be:

```bash
gira export dashboard --repo OWNER/REPO --output out/dashboard --dry-run
gira export dashboard --repo OWNER/REPO --output out/dashboard
gira export dashboard --config .gira/config.yaml --output out/dashboard --dry-run
gira export dashboard --config .gira/config.yaml --output out/dashboard
gira goal report GOAL --repo OWNER/REPO --html --output out/dashboard/goals/goal-GOAL.html
```

The existing `gira export dashboard` artifact contract is the base layer. The
3.0 product layer should make that bundle feel like an operator-facing report:
easy to regenerate, easy to open locally, and clear about the next safe Gira
command.

This choice keeps the first 3.0 slice:

- CLI-first
- local and inspectable
- deterministic enough to diff
- usable without a server
- compatible with future hosted or TUI surfaces
- grounded in GitHub as the source of truth

## User Jobs

The local report bundle should answer operational questions that are currently
spread across CLI commands:

| User job | Current state contract | Local report surface |
| --- | --- | --- |
| See what is ready for an agent | `workspace-queues/v1`, `ticket status` | Ready queue with start commands |
| See what needs human review | `ticket review`, PR readiness | Review queue with PR links and review packet links |
| See what can finish | `ticket status`, `finish-readiness/v1` | Finish-ready queue with `ticket finish --dry-run` commands |
| See what is blocked or failing | `ticket status`, checks, blockers | Blocked and failed-check sections |
| See goal progress | `goal-status/v1`, `goal-dossier/v1` | Goal cards and goal HTML reports |
| See dashboard export health | `dashboard_export/v1alpha1` manifest and warnings | Manifest summary and warnings page |

## Operator Flow

The first experience should be a short CLI loop:

```bash
gira workspace status --json
gira export dashboard --repo OWNER/REPO --output out/dashboard --dry-run
gira export dashboard --repo OWNER/REPO --output out/dashboard
open out/dashboard/index.html
```

If an HTML index is not implemented yet, the bundle should still be useful via:

```bash
ls out/dashboard
cat out/dashboard/manifest.json
cat out/dashboard/derived/execution_board.json
```

The key 3.0 improvement is not the file write itself. It is that the bundle
becomes a guided operating surface:

- top-level health summary
- queue sections
- evidence links
- warning list
- next safe commands
- clear source and snapshot metadata

## Bundle Information Architecture

The existing dashboard export contract already defines the machine layer:

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

The 3.0 local report layer should add human-facing files without changing the
canonical raw and derived JSON contracts:

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
  goals/
    goal-525.html
  tickets/
    ticket-666.html
  reviews/
    pr-665.html
```

The HTML files are views over exported or freshly computed Gira state. They are
not durable state and must be safe to delete and regenerate.

## UX Rules

1. Every page must show its source contract and snapshot time.
2. Every queue item must link back to the GitHub issue or PR.
3. Every actionable item must show the next safe Gira command.
4. Warnings must be visible without opening raw JSON.
5. The report must be useful offline after generation.
6. The report must not require a local daemon or hosted backend.
7. HTML output must escape GitHub-controlled text.
8. JSON remains canonical; HTML is a presentation layer.

## Command Shape

The first implementation should extend existing command language rather than
introduce UI-specific verbs.

Preferred direction:

```bash
gira export dashboard --repo OWNER/REPO --output out/dashboard --html
```

Compatibility direction:

```bash
gira report bundle --repo OWNER/REPO --output out/gira-report
```

The preferred path is better for the first slice because `export dashboard`
already owns the artifact layout, dry-run plan, schema version, manifest, raw
JSON, derived JSON, and CSV outputs.

## Contract Gaps

The existing export bundle is a good foundation, but a local report UX needs a
few additional contracts:

- Workspace queue snapshot included in the bundle.
- Ticket detail report generated from `ticket-status/v1`.
- Review report generated from `ticket review --json`.
- Goal HTML reports linked from the bundle index.
- Stable warning codes instead of prose-only warning strings.
- A top-level `index.html` artifact listed in `manifest.json`.

These gaps map directly to the next 3.0 child tickets:

- #667 Define workspace dashboard contract gaps for UI-shaped reports.
- #668 Implement local workspace report bundle export.
- #669 Add ticket detail HTML report from ticket status and review state.
- #670 Add review packet HTML report for human review flow.

The #667 contract decisions are documented in
[workspace-dashboard-contract-gaps.md](workspace-dashboard-contract-gaps.md).

## Non-Goals

The local report bundle must not introduce:

- hosted dashboard infrastructure
- browser-side GitHub API reads
- SQLite or another local database as source of truth
- bidirectional dashboard edits
- background sync
- authentication or RBAC model
- vendor-specific Notion, Sheets, or Airtable coupling

Those can be evaluated later after the report contract proves useful.

## Success Criteria

The local report bundle is successful when a maintainer can:

- generate it from one command
- open a local entry page
- see agent-ready, review-needed, finish-ready, blocked, and failed-check work
- jump to GitHub evidence
- copy the next safe Gira command
- regenerate without hidden state
- review the same data as JSON for automation

## First Implementation Recommendation

Start with #667 before #668.

The current export bundle already has `execution_board`, `roadmap_timeline`,
and `warnings`, but the 3.0 dashboard experience needs an explicit workspace
queue contract in the bundle. Defining that gap first keeps the implementation
small and prevents HTML from reimplementing queue logic.

After #667, #668 can add `index.html` and the bundle-level UX. Ticket and review
HTML pages can follow as specialized deep links.
