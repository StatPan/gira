# GitHub Projects Native View Boundary

This document decides which Gira workflow signals belong in GitHub Projects
native fields and which must stay in CLI JSON, dashboard export artifacts, or
local/operator-only evidence.

Planning ticket: #715.

## Decision

Keep GitHub Projects as a visibility surface over GitHub issues. Do not turn
Projects into a second workflow database, dashboard store, or agent evidence
ledger.

For the 3.4 native Projects slice, keep the existing `projects sync` projection:

- issue membership;
- Status from canonical `status:*` labels;
- Priority from `priority:*` labels;
- Area from `area:*` labels;
- Agent from `agent:*` labels;
- Target date from milestone due dates.

Do not add native Projects fields for pulse, storage diagnostics, dashboard
warnings, run evidence, queue state, finish readiness, or finish receipts.

## Signal Boundary

| Signal | Current source | GitHub Projects native field? | Decision |
| --- | --- | --- | --- |
| Issue membership | GitHub issues plus `workspace.repos` | Yes | Existing `projects sync` should add in-scope repo issues. |
| Workflow status | `status:*` labels | Yes | Mirror to Project Status for visibility. Labels remain canonical. |
| Priority | `priority:*` labels | Yes | Mirror to native single-select field. |
| Area | `area:*` labels | Yes | Mirror to native single-select/text field used by Gira. |
| Agent/owner class | `agent:*` labels | Yes | Mirror as visibility metadata, not runtime assignment. |
| Target date | Milestone due date | Yes | Mirror due-date planning metadata. |
| Queue state | `workspace-queues/v1` | No | Computed operating view; keep in CLI/export. |
| Finish readiness | `finish-readiness/v1` | No | Computed gate over PR/check/review evidence; keep in CLI JSON. |
| Finish receipt | `finish-receipt/v1` | No | Durable audit comment on the issue, not a Project field. |
| Pulse | `pulse-report/v1alpha1` | No | Movement summary; keep in dashboard/export artifacts. |
| Storage diagnostics | `config-storage-report/v1` | No | Local/operator state; never mirror to Projects. |
| Dashboard warnings | Dashboard export | No | Computed attention signals; keep in export/report output. |
| Run evidence | Private local `gira run` state | No | Private/operator evidence; do not publish into Projects. |
| Local cache freshness | Local cache/state | No | Disposable performance detail; not team execution state. |

## Manual View Guidance

Gira should continue to guide operators toward manual Project views instead of
mutating unsupported Project view APIs.

Supported guidance:

- Board view grouped by Status.
- Table view with Status, Priority, Area, Agent, Milestone, and Target date.
- Schedule or roadmap-style view using Target date where available.
- Filters over existing repo issue membership and labels.

Unsupported for this slice:

- Creating or mutating Project views through undocumented GraphQL behavior.
- Adding queue, pulse, storage, warning, or finish-readiness fields.
- Bidirectional editing where Project fields become the source of truth.
- Cross-repo item mutation outside configured `workspace.repos`.
- Implicit Project adoption from passive discovery.

## Safety Rules

Recent Projects fixes establish the boundary:

- Project adoption is explicit. Passive owner-level Project discovery is not
  consent to link or sync a Project.
- `projects sync` item-level mutations are bounded to `workspace.repos`.
- Project field creation/linking may affect the selected Project, but issue
  item status/archive/planning mutations must stay within the configured
  workspace repo set.

## Implementation Decision

No new Projects-native implementation ticket is ready from this research.

Rejected for 3.4:

- native queue-state fields;
- native pulse or movement-score fields;
- native finish-readiness fields;
- native storage/cache/run-evidence fields;
- Project view mutation automation.

Deferred:

- A future narrow Project view UX helper may be considered only if GitHub
  exposes supported view APIs or if Gira can provide manual setup output without
  mutation.
- A future Project field may be considered only when the source is already a
  durable GitHub issue label, milestone, or issue field and does not duplicate
  computed Gira JSON.

## Consequences

- Gira keeps CLI JSON and receipts as the agent workflow-control contract.
- Dashboard/export artifacts remain the home for computed operator signals.
- GitHub Projects stay useful for shared visibility without becoming the hidden
  execution source of truth.
- The next safe work should focus on README/contract/MCP/handoff surfaces rather
  than broader Projects-native automation.
