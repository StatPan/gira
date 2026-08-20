# Goal Operating Model

This document defines a Gira goal as the operating contract for long-running
AI-assisted work. A goal is not a larger issue, a milestone replacement, or a
hidden planning database. It is an autonomy envelope that tells an agent what it
may do, when it must stop, how it should lower work into tickets, and what
receipt proves that the work converged.

GitHub remains the source of truth. Goals are represented by GitHub issues,
labels, links, comments, milestones, and PR evidence. The first implementation
slice should add goal commands around those artifacts instead of introducing a
separate Gira planning store.

## Object Model

A goal contains these fields:

| Field | Meaning | GitHub mapping |
| --- | --- | --- |
| `objective` | The durable outcome the operator wants. | Goal issue title and `## Goal` body section. |
| `direction` | Strategic guidance, priorities, and tradeoffs that help agents choose between valid paths. | `## Direction` body section or pinned issue comment. |
| `scope` | Included repos, milestones, feature areas, and explicit non-goals. | Body sections plus labels such as `area:*`, `milestone`, and linked repo issues. |
| `autonomy` | What agents may do without asking. | Labels such as `lane:agent`, `lane:hybrid`, `requires-human-approval`, and body policy. |
| `decomposition_rules` | Rules for splitting the goal into executable tickets. | Body checklist and linked child issues. |
| `quality_bar` | Required verification, review, docs, doctor, release, and compatibility evidence. | Body acceptance criteria, PR checks, reviews, and final comments. |
| `stop_conditions` | Conditions that require human input before continuing. | Body policy plus `status:blocked` or `requires-human-approval`. |
| `receipt` | Final evidence that all work converged. | Closing comment linking tickets, PRs, checks, remaining risks, and state changes. |

The goal issue should be labeled with a normal type label, usually `type:epic`
or a future `type:goal`, plus `status:*`, `priority:*`, and area labels. Child
work remains normal repo-local tickets. PRs should close child tickets, not the
goal issue, unless the PR is explicitly the final goal receipt.

## Backlog Goal Handles

In a multi-repo workspace, the configured inbox repo can also hold cross-repo
goal handles. For example, `OWNER/backlog#12` can remain the durable issue that
states the objective, open decisions, and child ticket links while execution
happens in `OWNER/app`, `OWNER/api`, and `OWNER/infra`.

Use a backlog goal handle when the work is broad, repo ownership is mixed, or a
human still needs one place to inspect convergence. Do not create placeholder
execution tickets in the backlog repo just to keep the plan visible. Once a
slice is executable in a specific codebase, lower it into a repo-local child
ticket and let that child own its branch, PR, checks, review, and finish
receipt.

This boundary is the same for broad workspaces and narrowed daily control
workspaces. A broad workspace may scan many execution repos and keep the
backlog issue open as the coordination hub. A daily control workspace may
filter to one repo or a small active subset, but backlog issues still coordinate
and repo-local child issues still execute.

## Relationship To Existing Units

| Unit | Role |
| --- | --- |
| Ticket | Executable work packet for one bounded change. A ticket has one branch and one primary PR. |
| PR | Reviewable change unit. It supplies implementation, checks, review, and merge evidence. |
| Epic | Large GitHub issue that groups related work. It may be human-managed and does not imply autonomous execution. |
| Milestone | Time, phase, or release boundary. It groups issues but does not define agent authority. |
| Goal | Autonomy envelope for sustained agent work. It defines direction, decomposition, safe progress, stop rules, and final receipt. |

A milestone can contain many goals. A goal can create or reference many tickets.
An epic can be promoted into a goal when it has explicit autonomy and quality
rules. A ticket should not become a goal just because it is large; it should be
split until each child ticket is executable by the normal Gira lifecycle.

## Autonomous Actions

When a goal is in an agent lane and no stop condition is active, an agent may:

- Inspect GitHub issues, labels, milestones, PRs, checks, and repository files.
- Propose or create child tickets after a dry-run-style plan is visible.
- Start one ready child ticket at a time with `gira ticket start`.
- Implement bounded changes for that ticket.
- Add focused tests, docs, and doctor/readiness updates required by the ticket.
- Open a PR with a closing reference to the child ticket.
- Wait for checks and gather review evidence.
- Finish the child ticket through `gira ticket finish` when policy allows.
- Post progress, blocker, handoff, and summary comments that link evidence.

These actions are still bounded by repository policy. Branch protection, review
requirements, failed checks, missing labels, missing issue structure, and
permissions override the goal's autonomy text.

## Stop And Resolution Conditions

An agent must stop the affected mutation when any condition below appears. It
must then follow the decomposition-first causal resolution policy in
[Gira PM Operating Policy](pm-operating-policy.md): retrieve missing context,
derive an available policy, isolate conflicts, create verification work, or
split reversible preparation before asking a person. Independent safe work may
continue when the Goal and checkout remain unambiguous.

- The objective, acceptance criteria, or target repo is ambiguous.
- The next step would change credentials, secrets, permissions, billing,
  production settings, legal/policy text, or external commitments.
- A requested change would exceed the goal scope or create a broad refactor.
- Child tickets cannot be made executable without product or priority judgment.
- Required checks fail for reasons that are not safely fixable in the current
  ticket scope.
- Review requests changes that require a direction decision.
- The agent needs to bypass branch protection, merge with failed checks, or
  close work without evidence.
- GitHub state conflicts: duplicate tickets, stale branches, mismatched PR
  links, project state drift, or unresolved blockers.
- The goal is in `lane:hybrid`, `lane:human`, or has `requires-human-approval`
  and the next action is merge, release, closure, or irreversible cleanup.

Only the residual authority or product-direction decision is handed to a
person. The handoff must name one exact question, viable options and impacts,
evidence already gathered, authority required, the safest default when one
exists, the next command, work that remains possible, and the resume condition.
Existing `ask_human` and `human_review` values remain compatibility routing
states; they do not prove causal decomposition has already happened.

## Decomposition

Goal decomposition lowers direction into normal executable tickets:

1. Read the goal issue and current linked tickets.
2. Identify the smallest repo-local changes that can be verified independently.
3. For each child ticket, define goal, scope, acceptance criteria, files or
   modules likely to change, verification commands, dependencies, and stop
   conditions.
4. Preserve ordering only where it is real: migrations before callers, API
   contracts before consumers, docs after behavior, release after checks.
5. Create child tickets with normal Gira labels and link them from the goal.
6. Keep child tickets independent enough for stateless implementer and reviewer
   prompts.

The planner should avoid creating a queue that only mirrors a brainstorm. A
child ticket is ready only when a worker can start without additional product
judgment. If decomposition reveals missing direction, it should create bounded
context, policy, verification, or risk-reduction work where possible. The goal
moves to a residual decision state only when those steps cannot resolve the
remaining product or authority judgment; it must not generate speculative
delivery tickets.

### Typed Work Graph

`gira goal graph` is the deterministic path from PM intent and discovery state
to independently verifiable child work. The bullet-based `goal plan` path
remains available for Goals that have not opted into a typed Work Graph. A Goal
opts in with a `## Work Graph` fenced JSON
block using `pm-work-graph-source/v1`:

```json
{
  "schema_version": "pm-work-graph-source/v1",
  "nodes": [{
    "id": "delivery.cli",
    "title": "Add the bounded CLI slice",
    "purpose": "Deliver the selected workflow",
    "profile": "delivery",
    "parent_outcome": "outcome.activation",
    "uncertainty": "resolved",
    "size": "small",
    "verification": [{"method": "go test ./...", "evidence": "passing checks"}],
    "dependencies": [{"node_id": "decision.policy", "kind": "information", "reason": "implementation requires the selected policy"}]
  }]
}
```

Every node requires purpose, PM task profile, parent outcome, and inspectable
verification. Dependencies require `ordering` or `information` semantics plus a
reason; array order is never treated as dependency evidence. Cycles, missing
resume conditions, oversized unsplit slices, and unresolved delivery judgment
fail closed. Equivalent existing children become `reuse`; explicit graph state
produces `create`, `supersede`, `split`, or `defer` actions.

Compile is read-only by default. `--dry-run --compact-json` emits body-free node
facts and a `pwg-*` fingerprint. `--apply --expect-plan ID` lowers only the
unchanged graph and posts a receipt. Repeating the approved apply returns the
existing receipt without duplicating children. Full JSON retains PM IR digest,
discovery outcome refs, graph semantics, diagnostics, and rendered action data.

### Planning Engine Visibility

`gira goal status` exposes `planning_engine` as `legacy_goal_plan`,
`typed_work_graph`, `mixed`, or `unconfigured`. `mixed` means the Goal body has
both legacy bullet-plan and typed work-graph sources. Gira intentionally does
not choose or merge those sources automatically: run the intended engine
explicitly, preserve the other source as migration evidence, then remove it in
a reviewed follow-up. This prevents two planning engines from silently creating
overlapping child tickets.

### Active Observe and Replan Loop

`gira pm observe` reads the current Goal, typed ledger/discovery graph, work
graph, delivery children, and measurement evidence. It deterministically
diagnoses invalidated assumptions, stale context, review-due decisions, open
questions, blocked learning, missing outcome evidence, scope drift, and invalid
work graphs. Recommendations use the bounded vocabulary `continue`, `retrieve`,
`discover`, `decide`, `split`, `replan`, `validate`, `report`, and `stop`, with a
named capability and residual-authority flag for each action.

`gira pm replan --dry-run` projects those diagnoses onto the typed graph and
names every `create`, `reuse`, `supersede`, `split`, `defer`, or explicit
`unblock` mutation. Apply requires the unchanged `pmr-*` fingerprint. Safe
independent creates and reuses continue; closing or otherwise irreversible work
is compiled into one decision packet instead of being silently applied. A
human override requires `--override` and `--rationale`; apply stores the
rationale as durable typed decision evidence before posting the idempotent
replan receipt. Receipts retain the prior recommendation digest, so later
evidence can explain why the proposed plan changed without rewriting history.

## Safe Progress

`goal next` should choose work by evidence, not by optimism:

- Prefer unblocked ready child tickets with clear verification.
- Prefer smaller tickets before broad shared-surface changes.
- Prefer failing-check or review-fix tickets before new feature work.
- Skip tickets that require human approval, secrets, broad access, or unclear
  acceptance criteria.
- Do not start a new child ticket when the current branch or PR needs cleanup.

`goal status` should summarize the workflow graph:

- Goal issue state, labels, lane, and stop conditions.
- Child tickets by state: ready, in progress, in review, blocked, done.
- PR evidence: open, draft, checks pending, checks failed, review blocked,
  mergeable, merged.
- Drift: closed child without merged PR, merged PR without closed issue, stale
  labels, missing close references, or missing final comments.
- Remaining autonomous work and human decisions required.
- Existing `goal-finish-receipt/v1` handoff receipt state, so completed goals
  that already moved to human review do not keep recommending another finish
  command.

## Finish Receipt

`goal finish` should not mean "the agent thinks it is done." It should mean the
goal graph has converged or has reached a declared handoff state.

A finish receipt should include:

- Goal issue number and objective.
- Child tickets created, superseded, closed, or intentionally left open.
- PRs opened and merged, with check and review outcomes.
- Acceptance criteria satisfied and evidence links.
- Labels, milestone, project, and status changes written by Gira.
- Tests, docs, doctor/readiness checks, release notes, or deployment evidence.
- Known residual risk and follow-up tickets.
- Whether completion is `done`, `human-review`, `ready-to-release`, `blocked`,
  `superseded`, or `abandoned`.

The receipt should be posted as a GitHub comment before closing the goal. If
the safe terminal state is not `done`, the goal should remain open or close
with an explicit non-done resolution label.

`gira goal finish --terminal done --apply` is the normal completion path for a
ready goal graph. It requires clean readiness with no blockers, posts an
idempotent `goal-finish-receipt/v1` done receipt, normalizes active status
labels to `status:done`, and closes the goal issue only after receipt posting
succeeds. The command refuses done apply when children remain open, evidence is
missing, checks are not clean, or the terminal is not explicit.

`gira goal finish --terminal human_review --apply` remains the supported
handoff path. It posts the `goal-finish-receipt/v1` comment when blockers remain
and preserves those blockers in the receipt. It does not close the goal, mark it
done, waive missing child evidence, or invent historical PR/check/receipt
evidence. The handoff path is idempotent: if the goal issue already has a
`goal-finish-receipt/v1` handoff comment, dry-run and apply report a skipped
comment action instead of posting a duplicate. This path is for completed goal
graphs that need a maintainer to accept or decide how to handle historical
evidence gaps.

## CLI Slices

Suggested implementation order:

| Command | First slice behavior |
| --- | --- |
| `gira goal plan` | Read a goal issue and print proposed child ticket packets. Optional `--apply` creates linked child issues. |
| `gira goal next` | Select the next safe child ticket or explain the stop condition. |
| `gira goal status` | Summarize goal, child tickets, PR evidence, blockers, drift, and remaining autonomous work. |
| `gira goal report` | Derive an operator, human-PM, AI-hydration, stakeholder, or audit view from the same canonical PM state and package it as text, JSON, or local HTML. Alias: `gira goal dossier`. |
| `gira goal finish` | Verify child ticket convergence, post a human-review receipt when blockers remain, and later close or hand off the goal. |

Each command should support `--dry-run|--apply` for mutations and `--json` for
automation. The commands should reuse `ticket start`, `ticket pr`,
`ticket checks`, `ticket wait`, and `ticket finish` instead of reimplementing
ticket lifecycle logic.

### Canonical PM report views

`gira goal report GOAL --view VIEW` accepts `operator` (the default), `human`,
`ai`, `stakeholder`, or `audit`. These are projections, not additional sources
of truth. Every view carries `goal-pm-view/v1`, its input schema versions, and a
common `state_digest` so consumers can prove that different presentations came
from the same PM state.

- `operator` keeps delivery queues, diagnoses, and the next safe judgment.
- `human` explains what changed, why, what was learned, and what judgment is
  still required. Residual decisions include viable options, authority, impact,
  and a resume condition.
- `ai` is deterministic and bounded. It carries compact references and an
  explicit audit expansion command instead of embedding source documents.
- `stakeholder` separates delivery from validated outcomes and removes ticket
  implementation detail while retaining product risks and decisions.
- `audit` preserves source references, content digests, provenance, and
  supersession metadata without treating the exported report as canonical.

HTML is a local, regenerable artifact. It escapes source-derived text and obeys
the same view filtering as JSON. Missing PM sources are reported as diagnostics;
the existing delivery dossier remains available for backward compatibility.

### Compact goal-plan exchange

`gira goal plan --compact-json` is the bounded machine exchange for agents that
need a plan without the full rendered child-ticket bodies. Run a dry-run first,
record its `plan_id`, then require that exact value for the mutation:

```bash
gira goal plan 123 --repo OWNER/REPO --dry-run --compact-json
gira goal plan 123 --repo OWNER/REPO --apply --compact-json --expect-plan gpp-...
```

The compact dry-run contains proposal summaries and payload hashes. A matching
compact apply emits only a mutation receipt; it does not repeat those proposals.
If GitHub state changes between the two commands, apply stops before mutation
with `plan_changed` and instructs the caller to run dry-run again. `--json`
remains the complete `goal-plan/v1` automation format for callers that need the
full ticket packets.

## GitHub State Mapping

Minimum viable GitHub mapping:

- Goal: GitHub issue with `type:epic` or future `type:goal`. In a workspace,
  this may be an inbox/backlog issue that coordinates children in other repos.
- Child tickets: normal GitHub issues linked through GitHub native sub-issues or
  Gira's explicit typed child-link marker.
- Target repo: same-repo children are the default; a goal plan item can route
  work with `OWNER/REPO: title` or `target_repo: OWNER/REPO - title`.
- Parent link: same-repo children should use `gira ticket parent CHILD --set
  GOAL --apply` (GitHub native sub-issues). For cross-repo children, add the
  exact marker to the parent Goal body or comment:
  `<!-- gira:goal-child-link/v1 repo=OWNER/REPO issue=N -->`.
  `gira goal plan --apply` records this marker for every created child in its
  parent-goal comment. `goal status` and `goal next` ignore free-form issue
  references, headings, comments, and historical `Parent: #N` text; migrate
  those links to a native relationship or typed marker before relying on them.
- Status: existing `status:*` labels.
- Autonomy: `lane:*`, `requires-human-approval`, and `agent:*` labels.
- Evidence: PR closing references, check runs, reviews, issue comments, and
  final receipt comment.
- Grouping: milestone and optional Projects v2 item are visibility surfaces,
  not the source of truth.

Future slices may add a compact machine-readable block in goal comments, but
the readable GitHub issue should remain sufficient for humans to audit the
goal without a Gira database.

For the broader ownership boundary between GitHub labels, computed JSON state,
durable receipts, and local cache, see [Gira State Model](state-model.md).
