# Product OS Project Schema

Gira's Product OS layer is a GitHub-native, AI-native operating model for product work. GitHub remains the canonical execution backend: issues are task packets, pull requests are change units, milestones are phase boundaries, releases are delivery checkpoints, and Projects v2 can provide planning views over those same objects.

This document defines the canonical Project OS field model before any GitHub Projects automation is implemented. Future automation must be dry-run-first, idempotent, and non-destructive.

## Scope

The schema covers the default Product OS project shape that Gira should be able to validate and eventually sync. It is not a Jira UI clone, not a separate planning database, and not a replacement for GitHub issue, PR, milestone, or release state.

The current MVP CLI must not mutate GitHub Projects v2 data. This schema is a design contract for future commands such as `gira project sync --dry-run`.

## Canonical Fields

| Field | Stable key | Owner | Required | Purpose |
| --- | --- | --- | --- | --- |
| Status | `status` | Projects v2 | Yes | Planning workflow state across issues, PRs, and roadmap items. |
| Priority | `priority` | Label plus optional Projects v2 mirror | Yes | Triage and sorting. Labels remain the execution-readable source. |
| Layer / workstream | `layer` | Label plus optional Projects v2 mirror | Yes | Product area or workstream grouping. |
| Owner / agent | `owner_agent` | Label plus optional Projects v2 mirror | Yes | Human or worker ownership signal. |
| Milestone / phase | `milestone_phase` | GitHub milestone | Yes for phase-bound work | Phase, sprint, or release boundary. |
| Start date | `start_date` | Projects v2 date field | Yes for roadmap-able items | First planned active date for timeline rendering. |
| Target date | `target_date` | Projects v2 date field | Yes for roadmap-able items | Planned completion date for timeline rendering. |
| Item type | `item_type` | Label | Yes | Taxonomy for issues and roadmap rollups. |
| Blocked reason | `blocked_reason` | Issue body/comment plus optional Projects v2 text | Conditional | Explains blocked status without inventing hidden state. |

Field names should stay human-readable in GitHub and stable in CLI JSON. Use Title Case for GitHub field names (`Start date`) and snake case for machine output (`start_date`).

## GitHub-Native Ownership

Gira should keep durable execution state on the GitHub object that already owns it.

Labels own:

- Type: `type:epic`, `type:story`, `type:task`, `type:spike`, `type:bug`.
- Priority: `priority:p0`, `priority:p1`, `priority:p2`, or the current repo's managed priority set.
- Layer or area: `area:docs`, `area:backend`, `area:ai`, `area:infra`, or future `layer:*` labels when the product schema needs a cleaner split from implementation area.
- Owner or agent class: `agent:human`, `agent:worker`, or similar managed labels.
- Cross-cutting state that must be visible without opening a Project view, such as `blocked`, if Gira later manages it.

Milestones own:

- Phase, sprint, or release boundary.
- Due date for the phase as a whole.
- Completion semantics based on the issues assigned to the milestone.

Projects v2 fields own:

- Planning workflow status when a board/table/roadmap view needs a single field.
- Start date and target date for roadmap views.
- Optional mirrors of priority, layer, owner, or milestone when a Project view needs grouping or sorting without parsing labels.
- View-specific planning metadata that should not become repository labels.

Issues and PRs own:

- The concrete work packet and implementation conversation.
- Linked closing semantics through `Closes #N`, `Fixes #N`, or `Resolves #N`.
- Evidence, acceptance criteria, and worker handoff context.

## Status Model

The canonical status options are:

- `Backlog`: known work that is not ready for execution.
- `Ready`: clear enough for a human or AI worker to start.
- `Blocked`: cannot progress without an external decision, dependency, or credential.
- `In progress`: active implementation or investigation.
- `In review`: a PR or equivalent review unit is open.
- `Done`: accepted, merged, or intentionally closed.

Future automation may infer transitions, but it must preview them before applying:

- Issue opened with enough acceptance criteria: `Backlog` or `Ready`.
- Branch or active worker handoff begins: `In progress`.
- Linked PR opens: `In review`.
- Linked PR merges and closes the issue: `Done`.
- All milestone issues are done: milestone or phase can be reported complete.

Status automation should never close issues, merge PRs, delete fields, or rewrite broad repository settings without explicit apply behavior beyond the dry-run plan.

## Roadmap View Requirements

GitHub roadmap and timeline views require date fields that can render an item across time. For Gira, every roadmap-able item must have:

- `Start date`: when the item is expected to begin active planning or execution.
- `Target date`: when the item is expected to complete, ship, or be ready for review.

Both dates are mandatory for reliable roadmap behavior. A target date alone can show a deadline but not duration. A start date alone can show a beginning but not completion. Missing either field makes timeline ordering ambiguous for humans and unreliable for AI workers that need to compare phases, dependencies, or active windows.

Date semantics:

- `start_date` is inclusive.
- `target_date` is inclusive.
- `target_date` must be on or after `start_date`.
- Milestone due dates may be used as a phase-level fallback for reporting, but they are not a substitute for roadmap item dates.
- PR dates are derived execution signals, not roadmap planning fields.

Recommended roadmap views:

- Roadmap by milestone or phase, filtered to roadmap-able item types.
- Roadmap by layer or workstream for parallel product tracks.
- Roadmap by owner or agent for workload inspection.
- Table view with missing-date validation fields surfaced near status.

## Roadmap-Able Item Taxonomy

Roadmap views should include items that represent planned product movement over time:

- Epics: milestone-sized outcomes spanning multiple issues.
- Stories: user-visible behavior or product workflow slices.
- Tasks: roadmap-able only when they represent a meaningful delivery checkpoint.
- Spikes: roadmap-able when they timebox a decision needed by a later phase.
- Bugs: roadmap-able only when tied to a release, launch, or committed quality bar.

Roadmap views should normally exclude:

- Tiny implementation chores.
- Duplicate or superseded issues.
- PRs that only mirror an issue already on the roadmap.
- Bootstrap-only tasks after they are complete.

When an issue is too small for roadmap dates, keep it in the execution issue list and connect it to a dated epic, story, or milestone.

## Missing Date Validation

Future Gira validation should classify roadmap items before applying changes:

- `ok`: `start_date` and `target_date` exist and `target_date >= start_date`.
- `missing_start_date`: target date exists, but start date is absent.
- `missing_target_date`: start date exists, but target date is absent.
- `missing_dates`: neither date exists.
- `invalid_date_range`: target date is earlier than start date.
- `phase_due_date_fallback`: item dates are missing, but the assigned milestone has a due date that can be reported as a fallback.
- `not_roadmapable`: item type is intentionally excluded from roadmap validation.

Fallback behavior must be conservative:

- Dry-run may report a suggested fallback from milestone due date.
- Apply must not invent item dates unless the operator explicitly opts into date population.
- Missing roadmap dates should produce warnings in dry-run output.
- Invalid date ranges should block apply for the affected item until corrected.
- Status and non-roadmap sync may continue when date warnings do not affect the requested operation.


## Permission model (GitHub auth, secret, token)

Product OS automation should be designed for two explicit modes:

- `dry-run`: read-only planning only; should work with minimal credentials and never require write-capable scope.
- `apply`: mutation mode; should only run actions that the current credential actually allows.

### Execution principle

1. Probe capabilities first, then emit a **capability summary** JSON.
2. Compute the full plan from computed status transitions/field operations regardless of missing permissions.
3. In dry-run mode: present full candidate plan + blocked actions per permission boundary.
4. In apply mode:
   - execute only allowed mutations,
   - skip disallowed mutations with explicit `SKIPPED: <reason>` entries,
   - return non-zero only when blocked items are mandatory for the selected command goal.

This allows secretless users to get planning value, while secreted users can apply safely when authorized.

### Capability taxonomy

`gira project sync` should support at least the following capabilities internally:

| Capability key | Meaning | Minimum permission source |
|---|---|---|
| `issues:read` | Read issues, labels, milestones, and linked PR fields | Repository read |
| `issues:write` | Create/update issue labels/comments and add issue labels | Repo write |
| `pullrequests:read` | Read PR state/links for transition inference | Repo read |
| `pullrequests:write` | Merge/close PR operations (future extension only) | Repo write / maintain |
| `projectsv2:read` | Read Projects v2 metadata/fields/items | Projects write/read |
| `projectsv2:write` | Mutate Projects v2 fields, views, and items | Projects write |
| `repo:settings:write` | Change repo-level defaults/settings if implemented later | Repo admin |

### Credential source rules

Token source changes behavior:

- **GitHub App installation token (recommended):** best for scoped Project API access. Permission probes should prefer this path for service-style automation.
- **PAT (fine-grained):** allowed if granted exact scopes; probes should validate effective scope by operation attempt or metadata endpoint.
- **Actions secret/JWT-fed token:** usually only useful in GitHub Actions context; local CLI can only use it if injected into runtime env. Never assume secret value; treat unavailable tokens as read-only.
- **No token / unauthenticated:** CLI should fail fast in apply mode with clear diagnostic, but still allow dry-run for non-authenticated introspection where possible.

### Capability summary JSON (planned output)

`gira project sync --dry-run --json` should emit machine-readable permissions and action gating:

```json
{
  "repo": "OWNER/REPO",
  "command": "project sync",
  "token": {
    "kind": "github-app|pat|actions-secret|none",
    "identity": "masked-or-unknown",
    "mode": "readwrite|readonly"
  },
  "dry_run": true,
  "capabilities": {
    "issues:read": "allowed",
    "issues:write": "allowed",
    "pullrequests:read": "allowed",
    "pullrequests:write": "denied:token_scope",
    "projectsv2:read": "allowed",
    "projectsv2:write": "denied:token_scope",
    "repo:settings:write": "unknown:unsupported"
  },
  "blocked_actions": [
    {
      "action": "project_status_field:update",
      "reason": "denied:token_scope",
      "required": "projectsv2:write"
    },
    {
      "action": "milestone_complete_annotation:create",
      "reason": "denied:token_scope",
      "required": "issues:write"
    }
  ]
}
```

### Error/UX requirements for capability mismatch

When apply encounters a denied permission:

- print one stable message format: `permission denied: <action> requires <capability>`
- list the denied capability with a short `blocked` reason in dry-run plan output
- keep non-blocking items runnable when they are in-scope and have all dependencies met
- if user asks for full apply and all remaining operations are blocked, return non-zero with summary `blocked_count > 0`

### `project` command planning precedence

For `status` and `sync` integration:

1. `sync` remains focused on bootstrap label/milestone/issue sync.
2. `project sync` owns only Project v2 lifecycle/status automation and roadmap maintenance.
3. Transition rules should run in read-compute order:
   - detect current state,
   - choose target state,
   - verify capability matrix,
   - apply if allowed.
4. Manual ownership override is required: if a non-Gira owner (human/worker) sets an explicit status label/comment, automation should skip conflicting updates and report conflict.


## Future Dry-Run Shape

A future `gira project sync --dry-run` should show a deterministic plan before any Projects v2 mutation:

```text
project sync plan:
project:          OWNER/REPO Product OS
fields:           2 would create, 1 would update, 5 skip
views:            1 would create, 2 skip
items:            4 would add, 12 skip
dates:            8 ok, 2 missing target date, 1 invalid range
status rules:     3 would transition, 9 skip

  create field: Start date (date)
  create field: Target date (date)
  create view: Roadmap by phase
  warn issue #42: missing target_date; milestone due date 2026-06-30 available as reporting fallback
  block issue #51: target_date 2026-05-01 is before start_date 2026-05-08
  transition issue #27: Ready -> In review because linked PR is open
```

JSON output for automation should use stable keys and no prose on stdout:

```json
{
  "repo": "OWNER/REPO",
  "project": "Product OS",
  "dry_run": true,
  "counts": {
    "fields_create": 2,
    "fields_update": 1,
    "views_create": 1,
    "items_add": 4,
    "date_warnings": 2,
    "date_blocks": 1,
    "status_transitions": 3
  },
  "date_validation": [
    {
      "issue": 42,
      "status": "missing_target_date",
      "fallback": {
        "source": "milestone_due_date",
        "value": "2026-06-30"
      }
    }
  ],
  "blocked": [
    {
      "issue": 51,
      "reason": "invalid_date_range"
    }
  ]
}
```

The apply form should follow the existing Gira sync convention: build the full plan first, mutate only Gira-managed fields/views/items, report partial failures clearly, and make reruns safe.
