# Gira State Model

Gira should keep durable workflow state on the GitHub object that already owns
the evidence. It should not turn every derived condition into a label, and it
should not create a hidden local planning database.

The practical rule is:

```text
GitHub labels = small public workflow taxonomy
Gira JSON = computed operating state
GitHub comments = durable receipts and handoffs
Local files/cache = configuration, bindings, and acceleration
```

## State Ownership

| State owner | Examples | Purpose | Source of truth |
| --- | --- | --- | --- |
| GitHub labels | `status:ready`, `status:in-progress`, `status:in-review`, `status:blocked`, `status:done`, `type:*`, `priority:*`, `area:*`, `agent:*`, optional `lane:*` | Small public taxonomy that humans can scan in GitHub. | GitHub issue labels. |
| GitHub issue/PR state | Open or closed issue, linked PR, draft PR, merged PR, reviews, checks, milestones, assignees, comments. | Durable execution evidence. | GitHub issue, PR, check, review, milestone, and comment APIs. |
| Gira computed JSON | `ticket_readiness`, `pr_readiness`, `finish-readiness/v1`, `workspace-queues/v1`, `queue-list/v1`, `queue-next/v1`, `queue-handoff/v1`, `queue-take/v1`, `goal-next/v1`, `config-storage-report/v1`, `blockers`, `reason_codes`, `next_action`, `next_step`, `remaining_autonomous_work`. | High-cardinality operating state for agents, adapters, dashboards, and CLI decisions. | Recomputed from GitHub evidence and Gira config. |
| Receipt/comment state | `finish-receipt/v1`, `goal-finish-receipt/v1`, supersede decision notes, worker handoff notes, progress notes. | Durable audit trail that explains why a transition or handoff happened. | GitHub issue or PR comments. |
| Local or global config | `.gira/config.yaml`, global repo registry, workspace registry, branch policy defaults, provider config pointers. | Operator configuration and repo/workspace selection. | Local files unless explicitly committed as repo-local contract. |
| Local cache/state | Workspace status cache, API budget-friendly snapshots, recent command state, checkout path metadata. | Speed and ergonomics only. | Disposable local cache; never the authoritative workflow state. |
| Adapter approval evidence | `gira-approval-plan/v1`, command capability metadata, planned actions, post-apply verification command. | Machine-readable approval boundary before a matching `--apply`. | Dry-run JSON output stored by the adapter or operator. |

## Label-Backed State

Labels should remain few, stable, and useful in the GitHub UI.

The core status labels are:

| Label | Meaning |
| --- | --- |
| `status:ready` | A ticket can be started after normal readiness checks. |
| `status:in-progress` | Work has started, usually with a trusted branch. |
| `status:in-review` | A PR or review surface is active. |
| `status:blocked` | External dependency, decision, or policy blocks progress. |
| `status:done` | Completion was accepted through merge, close, supersede, or explicit receipt evidence. |

Other stable label families:

- `type:*` for issue kind.
- `priority:*` for coarse priority.
- `area:*` or future `layer:*` for product or implementation area.
- `agent:*` for execution actor class, not a replacement for assignees.
- `lane:*` for delegation authority when a repo opts into agent lanes.

Gira should avoid labels such as `status:finish-ready`,
`status:review-needed`, `status:failed-check`, `status:human-decision`, or
`status:missing-receipt`. Those are computed views over richer evidence, and
making them labels would create drift and taxonomy churn.

## Computed State

Computed state is where Gira can be expressive without polluting GitHub labels.

| Computed state | Derived from | Used by |
| --- | --- | --- |
| `ticket-readiness/v1` | Issue body, labels, state, scope, acceptance criteria, policy. | `ticket status`, `workspace_queues`, worker handoff. |
| `pr-readiness/v1` | Linked PR, draft state, checks, reviews, closing reference, branch binding. | `ticket review`, `ticket finish`, review queues. |
| `finish-readiness/v1` | Ticket status, linked PR, checks, review, labels, branch trust, telemetry. | `ticket finish --dry-run`. |
| `workspace-queues/v1` | Workspace issue summaries plus targeted ticket status details. | Multi-repo operator queues and future UI dashboards. |
| `queue-list/v1`, `queue-next/v1`, `queue-handoff/v1`, `queue-take/v1` | `workspace-queues/v1`, worker handoff readiness, and delegated ticket-start dry-runs. | CLI task selection, adapter handoff, and safe work start. |
| `goal-status/v1` and `goal-next/v1` | Goal issue, child issue links, child ticket state, PR evidence, receipts. | Long-running agent delegation and stop decisions. |
| Adapter capability and approval reports | Command registry, dry-run result, planned actions. | Durable agent runtimes and approval gates. |

Computed state can have many values because it is not a GitHub taxonomy. It can
name precise blockers such as `missing_linked_pr`, `checks_failed`,
`missing_review`, `child_524_missing_finish_receipt`, or
`human_review_handoff_present` without asking users to maintain those as labels.

See [Finish Readiness Contract](finish-readiness-contract.md) for the public
`finish-readiness/v1` and `finish-receipt/v1` field contract.

## Receipt And Comment State

Some state is too important to be only computed, but too contextual to be a
label. That belongs in a durable GitHub comment.

Use comments for:

- Finish receipts that explain why ticket completion was accepted.
- Goal finish receipts and human-review handoffs.
- Supersede decisions and replacement links.
- Worker handoff notes that preserve context between agents or humans.
- Progress, blocker, and decision notes that are useful audit evidence.

Receipts should be idempotent where possible. Gira should not create duplicate
goal handoff receipts when an existing `goal-finish-receipt/v1` comment already
records the same terminal state.

## Local State

Local state is allowed, but it must not become the hidden source of truth for
workflow completion.

Appropriate local state:

- Repo and workspace registry entries.
- Checkout path and default workspace selection.
- Branch policy defaults and recorded branch binding evidence.
- Cache entries for bounded workspace reads.
- Provider configuration pointers that do not contain secrets.
- Private `gira run` prompts, event logs, stderr, results, and manifests when
  the operator explicitly uses local run evidence.

Inappropriate local state:

- "This issue is done" without GitHub close/merge/receipt evidence.
- Hidden ticket queues that disagree with labels and PR state.
- Agent-only completion decisions that are not written back to GitHub.

If local state disappears, Gira should be able to reconstruct the workflow from
GitHub plus repo/global configuration, possibly with less performance or less
ergonomic command resolution.

Use `gira config storage --repo OWNER/REPO` to inspect the local storage map.
It reports config, state, cache, run evidence, audit, export, and distribution
cache surfaces with durability, privacy, rebuild, and source-of-truth
classifications. It is a diagnostic report, not a local database.

## Vocabulary

| Term | User-facing explanation | Not the same as |
| --- | --- | --- |
| Ticket | One executable work packet. It has one primary branch and usually one primary PR. | A goal or a milestone. |
| PR | The reviewable change unit and main implementation evidence for a ticket. | A ticket by itself. |
| Milestone | A time, phase, sprint, or release boundary. It groups work. | A delegation envelope. |
| Epic | A large GitHub issue that groups related work. It can remain human-managed. | Automatically an autonomous goal. |
| Goal | A large objective that Gira treats as an autonomy envelope over child tickets, stop conditions, and finish receipts. | A GitHub-native object or milestone. |
| Workspace queue | A computed operating view over many tickets and PRs. | A label taxonomy or hidden backlog database. |
| Agent handoff queue | The CLI selection and handoff layer over `workspace-queues/v1`. | A worker runtime, model router, or execution database. |
| Receipt | A durable comment explaining a transition, completion, supersede, or handoff decision. | A status label. |

The shortest explanation for goal mode is:

> A goal is a GitHub issue that Gira interprets as delegated multi-ticket work.
> `goal next` chooses the next safe child ticket, and `goal finish` records
> convergence or human-review handoff evidence.

## UI And Adapter Boundary

The CLI and JSON contracts should remain the computation layer. Future 3.0 UI
or hosted surfaces should render the same state instead of reimplementing it.

Good UI candidates:

- Goal graph summary and why `goal next` stopped.
- Workspace queues with blockers and next safe commands.
- Agent handoff queue lanes using `workspace-queues/v1` and `queue-*` contracts.
- Ticket detail evidence cards.
- Finish readiness and receipt review.
- Drift and missing evidence dashboards.

Adapter runtimes should use command capability metadata and
`gira-approval-plan/v1` instead of guessing whether a command is safe. A dry-run
approval for one command must not authorize a different command, repo, issue,
branch, or flag set.

## Follow-Up Planning Candidates

This state model suggests these follow-up issues:

- Improve human-readable goal summaries, possibly `gira goal brief`.
- Add a compact state-model page to `gira guide` if users keep confusing labels,
  computed state, receipts, and cache.
- Audit legacy docs for wording that implies local cache or Projects are the
  source of truth.
- Decide whether future `lane:*` labels should be part of the default taxonomy
  or opt-in delegation policy only.
