# Gira Workflow Model

Gira makes a forge behave like Jira for human and AI work. Its workflow grammar
therefore follows **Jira's workflow model**: status categories, statuses,
workflows, transitions, transition rules, and resolution.

This is a model choice, not a Jira clone. Gira does not copy Jira's UI or
planning database. Durable state stays on forge objects as described in the
[State Model](state-model.md); this document defines how those objects express
a Jira-style workflow.

Status: phase 1 specification for the epic
[#986](https://github.com/StatPan/gira/issues/986). Nothing in this document
changes current behavior. Later phases implement it.

## Reference Standard

The reference is Atlassian's own documentation, read on 2026-11-13. Only the
statements below are claimed. Where Atlassian is silent, this document says so
and states Gira's own decision instead.

| Source | Used for |
| --- | --- |
| [Defining status field values](https://confluence.atlassian.com/adminjiraserver/defining-status-field-values-938847108.html) (Data Center) | Status categories |
| [Manage how work flows in your team-managed space](https://support.atlassian.com/jira-software-cloud/docs/manage-how-work-flows-in-your-team-managed-project/) | Statuses, transitions, categories in team-managed spaces |
| [Configure advanced work item workflows](https://support.atlassian.com/jira-cloud-administration/docs/configure-advanced-issue-workflows/) | Conditions, validators, post functions, triggers, global transitions, resolution post function |
| [Edit a workflow's rules](https://support.atlassian.com/jira-cloud-administration/docs/edit-an-issue-workflow/) | Rule categories in the current editor |
| [Manage work item transitions in team-managed spaces](https://support.atlassian.com/jira-software-cloud/docs/create-edit-and-delete-transitions-in-team-managed-projects/) | "Any status" transitions |
| [What is a resolution in Jira?](https://support.atlassian.com/jira-cloud-administration/docs/what-is-a-resolution-in-jira/) | Resolution semantics |
| [Understand workflow triggers](https://support.atlassian.com/jira-cloud-administration/docs/understand-workflow-triggers/) | Development trigger events |
| [KB: PR not able to transition a Jira issue](https://support.atlassian.com/jira/kb/configuring-jira-notification-when-a-pull-request-from-github-is-not-able-to-transition-a-jira-issue/) (Data Center) | Missing transition path behavior |

Scrum, Kanban, DORA, and system-of-record integration practices are useful
background but are not standards Gira claims to implement. In particular,
"Definition of Ready" is not part of the Scrum Guide.

## The Jira Model

### Status categories

- Jira groups statuses into three categories: "To Do" (grey), "In Progress"
  (blue), and "Done" (green). Categories "help you identify where issues are in
  their lifecycle" and are used to map statuses to board columns.
- Team-managed spaces start with three statuses, "To do, In progress, Done", and
  let teams "collect many statuses under a to do, in progress, or done category".
- Atlassian does not state in these pages that the category set is closed or
  that a status must belong to exactly one category; the status form only
  offers these three choices.

**Gira decision:** the three categories are fixed, and every Gira status
belongs to exactly one. Reports, queues, and readiness decisions that only need
lifecycle position use the category, not the status name.

### Statuses and workflows

- "Two concepts define a workflow in team-managed spaces: Statuses ...
  Transitions – how a piece of work can move between statuses."
- "Transitions define which statuses a work item can move to from its current
  status."
- "Global transitions allow any status in a workflow to transition to a
  particular status." Team-managed spaces treat "Any status" transitions as
  normal and warn that removing them may cause errors when moving cards.
- Statuses are team-defined: "You can add your own statuses and customize the
  workflow."
- Atlassian does not state that a work item can only move by a transition;
  admin operations such as deleting a status move items without running rules.

**Gira decision:** Gira lifecycle commands move tickets only through declared
transitions. Manual label edits on GitHub are possible and are treated as
[external drift](external-drift-policy.md), not as transitions.

### Transition rules

Classic model (company-managed):

| Rule | Atlassian definition | Failure behavior |
| --- | --- | --- |
| Condition | "check that a transition should be performed by the user" | "the user will not see the transition button" |
| Validator | "check that any input to the transition ... is valid, before the transition is performed" | "the work item does not progress to the destination status of the transition, and the transition's post functions are not executed" |
| Post function | "carry out additional processing, after a transition is performed" | Essential post functions (set status, comment, history, re-index, fire event) "cannot be deleted from a transition or reordered" |
| Trigger | "transition Jira work items when certain events occur in a connected development tool" | See below |
| Property | "key-value pairs that can be used to further customize transitions" | — |

The current editor groups rules as **Restrict transitions**, **Request input**,
**Validate details**, and **Perform action**, with the same validator failure
semantics.

### Triggers

- Trigger events: create commit, create branch, create/reopen/decline/merge
  pull request, start/reject/abandon/close review. Work items are referenced by
  putting the key in the branch name, commit message, or review title. Build
  and deployment events are not trigger events on that page.
- "When a transition is triggered automatically, it ignores any conditions,
  validators or permissions configured on the transition. However, post
  functions are still executed."
- If the current status has no outgoing transition to the target, "the issue
  will not be transitioned" and "no email will be sent out" (Data Center KB).
- Atlassian warns that triggers on global transitions can cause "a work item
  unexpectedly transitioning to the target status".

**Gira decision:** this is the main gap Gira exists to close. Forge evidence
(linked PR, checks, review, exact head, merge) is enforced as **validators**,
even for transitions caused by forge events. A missing transition path is a
reported blocker, never a silent no-op.

### Resolution

- "a work item's Resolution field describes the reason it was moved to Done",
  for example Fixed, Won't Fix, Can't Reproduce.
- Open versus closed follows resolution, not status: "the Open work items filter
  shows work items that don't have a resolution."
- Atlassian "strongly recommend[s] setting up your workflows so that
  resolutions are automatically set when work moves to a Done (green) status",
  and clearing it when reopening.

**Gira decision:** the forge's issue close state and close reason are Gira's
resolution. Entering a Done-category status closes the issue with a reason;
leaving Done reopens it.

## Mapping To Gira

| Jira | Gira on GitHub | Exists today |
| --- | --- | --- |
| Status category | Category attribute of each status | No |
| Status | `status:*` label | Yes, fixed set |
| Workflow | Declared statuses + transitions per repo | No; implicit in command code |
| Transition | A lifecycle command step (`ticket start`, `ticket pr`, `ticket finish`, supersede) | Yes, hardcoded |
| Condition | Command preconditions: issue open, `status:ready`, policy mode, lane authority | Yes |
| Validator | Readiness and evidence gates: ticket readiness, linked PR, closing reference, draft, checks, review policy, exact head, merged verification | Yes |
| Post function | Label change, issue close, receipt comment, local sync, Jira transition | Yes |
| Trigger | Forge events Gira observes: branch created, PR opened, PR ready, PR merged | Partly, via command runs |
| Resolution | Issue state + close reason (`completed`, `not planned`) | Partly |
| Global transition | Transition from any status, for example to `blocked` | Implicit |

### Default statuses

The default workflow is today's label set. Every label gets a category.

| Status label | Category | Notes |
| --- | --- | --- |
| `status:backlog` | To Do | Recognized by project transitions; not in default `ops sync` labels |
| `status:ready` | To Do | Start condition for `ticket start` |
| `status:in-progress` | In Progress | Set by `ticket start` and by `ticket pr --draft` |
| `status:in-review` | In Progress | Set by `ticket pr` for ready PRs |
| `status:blocked` | In Progress | See decision below |
| `status:done` | Done | Set with issue close by `ticket finish`, goal finish |

`needs-*` labels such as `status:needs-review`, `status:needs-design`, and
`status:needs-decision` appear in templates and triage text but are not managed
statuses. They are classified as flags, not statuses.

**`blocked` decision:** keep `status:blocked` as a status in the In Progress
category, reachable by a global transition from any non-Done status, with a
transition back to the previous status. Jira teams commonly model blocked as a
flag; Gira keeps the status because existing queues, triage, and reports depend
on it. A Jira-primary repo may map it to a Jira flag in phase 4.

### Default transitions

```text
backlog ─▶ ready ─▶ in-progress ─▶ in-review ─▶ done
             ▲           │  ▲           │
             │           ▼  │           ▼
             │       (draft PR)    (PR back to draft)
any non-done ─▶ blocked ─▶ previous status
any ─▶ done (supersede, resolution: not planned)
```

| Transition | Command | Conditions | Validators | Post functions |
| --- | --- | --- | --- | --- |
| ready → in-progress | `ticket start` | open issue, `status:ready`, clean worktree on apply | ticket readiness, branch reuse safety, remote branch conflict | create/reuse branch, set label |
| in-progress → in-review | `ticket pr` | branch exists | closing keyword, PR base | create/update PR, set label by draft state |
| in-review → in-progress | `ticket pr` on a draft PR | — | — | set label |
| in-review → done | `ticket finish` | linked PR selected | finish readiness: checks, draft, review policy, exact head, merged verification, Jira done gate | merge, close issue (`completed`), set `status:done`, receipt, optional local sync, Jira Done |
| any → done | `ticket supersede` | — | replacement ticket created | close (`not planned`), decision note |

## Example: Ready For Release

A team that deploys after merge declares one more status.

```yaml
workflow:
  statuses:
    - { name: ready, category: to_do }
    - { name: in-progress, category: in_progress }
    - { name: in-review, category: in_progress }
    - { name: ready-for-release, category: in_progress }   # 배포대기
    - { name: done, category: done }
  transitions:
    - { name: start, from: ready, to: in-progress }
    - { name: review, from: in-progress, to: in-review }
    - { name: finish, from: in-review, to: ready-for-release, command: ticket finish,
        validators: [finish_readiness], post: [merge, set_status_label, finish_receipt] }
    - { name: release, from: ready-for-release, to: done, command: external,
        post: [close_issue, set_status_label] }
  finish_target: ready-for-release
```

Status labels default to `status:` followed by the name; an explicit `label` may
be supplied instead. Transition `from` accepts a status name, a list of names,
or `any` (also spelled `global`). Phase 2 validates transition declarations and
registered names in `conditions`, `validators`, and `post`; doctor reports an
unknown name as `workflow_unknown_step`. `command` remains illustrative and
existing commands do not yet run transition rules (phase 3). A release-evidence
validator for the external `release` transition is not registered yet.

- `ticket finish` merges and moves the ticket to `ready-for-release`; the issue
  stays open because it is not in the Done category.
- `done` requires release evidence that Gira does not produce; it is a declared
  external transition, recorded through a receipt or a later phase command.
- Merge alone never counts as Done for this workflow.

## Source Of Truth Modes

The same declared workflow runs in two modes. Only the status store changes.

| Field | GitHub-native mode | Jira-primary mode |
| --- | --- | --- |
| Status | `status:*` labels | Jira status; labels are a read-only mirror |
| Planning: summary, priority, assignee, sprint | GitHub issue, labels, milestone | Jira |
| Execution: branch, PR, checks, review, merge | GitHub | GitHub |
| Resolution | Issue close reason | Jira resolution, gated on GitHub evidence |

Rules:

- One status writer per transition. In Jira-primary mode either Gira performs
  the Jira transition, or Jira Automation does it from forge triggers; config
  declares which, so a transition is never written twice.
- Because Jira triggers skip validators, Gira's forge-evidence validators run
  in Gira before Gira writes, and are re-checked as drift when Jira Automation
  writes.
- Disagreement between the status store and the mirror is reported as drift;
  the store wins. Last-write-wins is never used.

## Gap Analysis

Current code, as of `origin/main` 56c70fc.

| Gap | Location | Effect |
| --- | --- | --- |
| No category concept | all status users | Reports and queues reason about label names, so a custom status breaks them |
| Status labels hardcoded in transitions | `setIssueStatus` and `statusLabelForDraft` in `internal/gira/work.go` (start, pr, finish paths); `applyFinishedIssueStatusLabels` in `ticket_finish.go` and `goal_finish.go`; `replaceIssueStatusLabel` in `project_transitions.go` | Transitions cannot be declared; no `ready-for-release` stage |
| Two status-name tables | `statusToLabel` / `mapStatusLabel` / `statusPriority` in `project_transitions.go`; default label definitions in `sync.go` | Same statuses defined twice |
| Inconsistent status derivation | `statusFromLabels` in `workspace.go` takes the first `status:` label; `managedStatusFromLabels` in `project_transitions.go` picks by priority | Multi-label issues get different statuses in different reports |
| Finish target fixed to Done | `ticket finish` close + `status:done` | Merge is treated as completion even when the team deploys later |
| Source of truth decided in one command only | `loadJiraFinishProvider` in `ticket_finish.go` | `ticket start/pr`, `status`, `sprint`, `triage` always treat labels as authority |
| Source-of-truth config is a fixed combo | `validateJiraProviderSourceOfTruth` in `jira_provider.go` allows only `jira/jira/github` | Per-field ownership cannot be declared |
| Jira status map inferred by name substring | `inferJiraProviderStatusMap` in `jira_provider.go` (`"qa"` → review, `"resolved"` → done) while the fetched Jira `statusCategory` is ignored | Custom statuses such as Ready for Release are unmapped or mis-mapped |
| Jira map keyed by a separate stage list | `status_map` uses `backlog/ready/in_progress/review/done` | Does not line up with declared statuses or categories |

## Phase Plan

These become child issues of #986.

**Phase 2 — Core**

1. Status category type and a single status table (name, label, category)
   replacing the duplicated tables; one derivation function used by every
   report. Regression tests prove default output is unchanged.
2. Declared workflow config (statuses, transitions, `finish_target`) with
   validation in `doctor`; absent config means the default workflow above.
3. Transition engine: conditions, validators, post functions as named steps,
   with existing gates registered as validators. No behavior change.

**Phase 3 — Commands**

4. `ticket start/pr/finish` and `supersede` executed through declared
   transitions; missing transition path is a blocker.
5. `finish_target` and non-Done targets that keep the issue open; external
   transitions recorded by receipt.
6. `status`, `sprint`, `triage`, workspace queues aggregate by category.

**Phase 4 — Jira provider**

7. Single source-of-truth resolver used by all lifecycle commands; per-field
   ownership config replacing the fixed combo.
8. Jira status map keyed by declared statuses and checked against Jira
   `statusCategory`; name inference only as a dry-run suggestion.
9. Status writer setting (`gira` or `jira_automation`) and drift reporting.
10. `jira doctor` checks: category agreement and transition path to
    `finish_target` from each mapped status.

Out of scope in every phase: background or bidirectional sync, creating Jira
workflows, Web UI.
