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

## Stop Conditions

An agent must stop and ask a human when any of these conditions appears:

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

Stop comments should be specific. They should name the blocker, the evidence,
the safest next command or decision, and what work remains possible afterward.

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
judgment. If decomposition reveals missing direction, the goal should move to a
human-decision state instead of generating speculative tickets.

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

`gira goal finish --terminal human_review --apply` is the first apply-safe
handoff path. It posts the `goal-finish-receipt/v1` comment when blockers remain
and preserves those blockers in the receipt. It does not close the goal, mark it
done, waive missing child evidence, or invent historical PR/check/receipt
evidence. This path is for completed goal graphs that need a maintainer to
accept or decide how to handle historical evidence gaps.

## CLI Slices

Suggested implementation order:

| Command | First slice behavior |
| --- | --- |
| `gira goal plan` | Read a goal issue and print proposed child ticket packets. Optional `--apply` creates linked child issues. |
| `gira goal next` | Select the next safe child ticket or explain the stop condition. |
| `gira goal status` | Summarize goal, child tickets, PR evidence, blockers, drift, and remaining autonomous work. |
| `gira goal finish` | Verify child ticket convergence, post a human-review receipt when blockers remain, and later close or hand off the goal. |

Each command should support `--dry-run|--apply` for mutations and `--json` for
automation. The commands should reuse `ticket start`, `ticket pr`,
`ticket checks`, `ticket wait`, and `ticket finish` instead of reimplementing
ticket lifecycle logic.

## GitHub State Mapping

Minimum viable GitHub mapping:

- Goal: GitHub issue with `type:epic` or future `type:goal`.
- Child tickets: normal GitHub issues linked from the goal body or comments.
- Parent link: child issue body contains `Parent goal: #N` or an equivalent
  structured link.
- Status: existing `status:*` labels.
- Autonomy: `lane:*`, `requires-human-approval`, and `agent:*` labels.
- Evidence: PR closing references, check runs, reviews, issue comments, and
  final receipt comment.
- Grouping: milestone and optional Projects v2 item are visibility surfaces,
  not the source of truth.

Future slices may add a compact machine-readable block in goal comments, but
the readable GitHub issue should remain sufficient for humans to audit the
goal without a Gira database.
