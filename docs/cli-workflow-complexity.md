# CLI Workflow Complexity

This is the #807 UX/AX diagnostic for measuring whether Gira's workflow CLI
reduces task burden compared with raw `gh`.

The answer is not character count. Character count is a weak proxy because a
short command can still require hidden domain knowledge, unsafe fallback, or a
confusing object order. The useful unit is a workflow node: one thing a human
operator or agent must understand, choose, or execute to finish the task.

## Node Model

Use these node types when reviewing a workflow:

| Node | Meaning | UX/AX impact |
| --- | --- | --- |
| Command node | A visible CLI command the operator or agent must run. | More commands increase sequencing burden, but can be acceptable when each command gives a clear next safe command. |
| Argument node | A required value the user must know or choose, such as repo, ticket, parent, label, title, body, or SHA. | More arguments increase memory and prompt length. Ambiguous argument order is especially costly. |
| Decision node | A branch in the workflow: dry-run/apply, wait/fix, draft/ready, create/reuse, merge/stop. | Decisions are expensive because agents need policy context and humans need confidence. |
| Provider node | A provider object or primitive exposed to the user: issue ID, GraphQL object ID, REST endpoint, PR head SHA, Project field ID. | Gira should hide provider nodes unless exposing them materially improves safety. |
| Fallback node | A point where the user must leave Gira and use raw `gh` or provider APIs. | Fallback is high cost because it breaks the workflow contract and increases uncertainty about source of truth. |
| Cognitive node | A product concept the user must map correctly: ticket, issue, parent, child, sub-issue, epic, goal, feature, PR, status, queue. | Cognitive nodes determine whether the CLI feels natural. Compact commands can still fail here. |

## Complexity Score

For diagnostics, report raw counts and a weighted score. The weights are not
intended to be mathematically perfect; they make review conversations concrete.

```text
workflow_cost =
  command_nodes
  + 0.5 * argument_nodes
  + 2.0 * decision_nodes
  + 2.0 * provider_nodes
  + 3.0 * fallback_nodes
  + 1.5 * cognitive_nodes
```

Interpretation:

| Score | Meaning |
| --- | --- |
| 0-8 | Low burden. Good default UX/AX for repeated work. |
| 9-16 | Manageable, but should provide next safe commands and stable JSON. |
| 17-24 | High. Needs a shortcut, clearer object model, or better report output. |
| 25+ | Too complex for a product workflow unless it is explicitly advanced/ops-only. |

Do not use the score alone. A high score may be acceptable for a rare migration
task. A lower score may still be bad if the object order is surprising.

## Representative Flows

The counts below measure the user-visible happy path. They do not count every
internal GitHub API call unless the user must reason about it.

### Ticket Lifecycle

Task: start from a ready issue, open a linked PR, wait for checks, and finish.

| Flow | Commands | Args | Decisions | Provider | Fallback | Cognitive | Score | Diagnosis |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| Gira canonical | 7 | 3 | 3 | 0 | 0 | 4 | 20.5 | Command count is not tiny, but each step maps to a lifecycle state and emits the next safe command. Good AX, acceptable UX. |
| Raw `gh` | 10 | 10 | 6 | 3 | 0 | 6 | 42.0 | More provider and policy nodes: branch naming, labels, PR closing text, checks, merge safety, issue close evidence. |

Typical Gira sequence:

```bash
gira ticket start 123 --dry-run
gira ticket start 123 --apply
gira ticket pr --dry-run
gira ticket pr --apply
gira ticket wait --timeout 5m
gira ticket finish --dry-run
gira ticket finish --apply
```

The visible command count is still noticeable. The UX/AX value comes from
state binding: ticket, branch, PR, checks, closing reference, and labels are
checked together instead of left as separate `gh` operations.

### Create A Native Sub-Issue

Task: create a new child ticket under an existing parent.

| Flow | Commands | Args | Decisions | Provider | Fallback | Cognitive | Score | Diagnosis |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| `gira ticket new --parent` | 2 | 4 | 1 | 0 | 0 | 3 | 10.5 | Good creation-time UX. Parent is a modifier on new ticket creation. |
| Raw `gh` | 4 | 8 | 2 | 2 | 0 | 4 | 22.0 | Requires creating issue, retrieving child node or ID, and calling the sub-issue API. |

Typical Gira sequence:

```bash
gira ticket new "Title" --parent 123 --body-file task.md --dry-run
gira ticket new "Title" --parent 123 --body-file task.md --apply
```

This is compact and aligned with the user's task: create a ticket under a
parent. It should remain the preferred path.

### Attach An Existing Sub-Issue

Task: attach existing child ticket `#456` under parent `#123`.

| Flow | Commands | Args | Decisions | Provider | Fallback | Cognitive | Score | Diagnosis |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| Current Gira canonical | 2 | 4 | 1 | 0 | 0 | 4 | 12.0 | Mechanically compact, but object order is not natural: `ticket parent CHILD --set PARENT` describes implementation state more than user intent. |
| Proposed ergonomic alias | 2 | 4 | 1 | 0 | 0 | 3 | 10.5 | `ticket child add PARENT CHILD` matches the user phrase "add child under parent". |
| Raw `gh` | 3 | 6 | 2 | 2 | 0 | 4 | 20.0 | Requires issue lookup and native sub-issue API details. |

Current command:

```bash
gira ticket parent 456 --set 123 --dry-run
gira ticket parent 456 --set 123 --apply
```

This should remain as the canonical state command because it is precise:
ticket `#456` has parent `#123`. But it is not the best teaching surface. For
UX/AX, the preferred user-facing shape should be evaluated as a follow-up:

```bash
gira ticket child add 123 456 --dry-run
gira ticket child add 123 456 --apply
gira ticket child list 123
```

The important issue is not only count. The current form forces the user to
think from the child's metadata field outward. The ergonomic form follows the
parent-to-child action that people naturally describe.

### Supersede A Ticket

Task: replace a wrongly scoped ticket with a new one and close the old one.

| Flow | Commands | Args | Decisions | Provider | Fallback | Cognitive | Score | Diagnosis |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| Gira `ticket supersede` | 2 | 4 | 1 | 0 | 0 | 4 | 12.0 | Good when bounded. The #805 fix keeps normal dry-run from waiting on linked PR discovery unless `--close-draft-pr` is requested. |
| Raw `gh` workaround | 5 | 10 | 4 | 0 | 1 | 6 | 30.0 | Create replacement, cross-comment, relabel, close original, and preserve audit semantics manually. |

Typical Gira sequence:

```bash
gira ticket supersede 323 --replacement-title "New scope" --body-file - --dry-run --json
gira ticket supersede 323 --replacement-title "New scope" --body-file - --apply --json
```

This flow shows why AX depends on bounded behavior. Even if the command count
is low, a silent wait creates an infinite decision node for agents. A command
that does not return a report, error, or next step has effectively unbounded
complexity.

## Findings

1. Gira is not currently checking task-level workflow nodes. It checks command
   registry metadata and generated docs, but not UX/AX burden per task.
2. Character count should not be the main metric. Object order, decision
   count, fallback risk, and provider leakage explain the user's discomfort
   better than command length.
3. `ticket new --parent` is a good surface because it keeps parent selection at
   creation time.
4. `ticket parent CHILD --set PARENT` is compact but not fully intuitive. It is
   a good canonical state command and a weaker ergonomic command.
5. Gira's strongest UX/AX value is next-safe-command sequencing. When commands
   return structured blockers and next steps, multiple command nodes are
   acceptable. When a command hangs or sends users to raw `gh`, complexity
   spikes.
6. Hidden provider calls should be treated as product risk when they can block,
   exhaust quotas, or require raw fallback. They do not need to be visible in
   daily UX, but they should be represented in diagnostics.

## UX/AX Review Gate

Before adding or teaching a workflow command, answer these questions in the
issue or PR:

1. What task does the command complete?
2. What is the Gira command graph for the happy path?
3. What is the raw `gh` graph for the same task?
4. Which provider nodes does Gira hide?
5. Which decisions remain visible to the user or agent?
6. Does each step emit a next safe command or structured blocker?
7. Does the object order match the user's sentence?
8. What fallback path remains, and is it rare enough to accept?

If Gira adds a command but the task score is not lower than raw `gh`, the
command needs a stronger justification: safety, idempotency, audit evidence,
or agent reliability.

## Follow-Up Slices

1. Add command-registry metadata for `surface_class`, `workflow_role`, and
   `teaching_priority`.
2. Add a generated workflow-complexity report for selected golden paths rather
   than every command.
3. Evaluate an ergonomic native sub-issue alias such as
   `gira ticket child add PARENT CHILD` while keeping `ticket parent` as the
   canonical state command.
4. Add an AX rule that every agent-facing mutating dry-run must terminate with
   JSON, a structured error, or a bounded timeout.
5. Teach golden paths separately from reference docs so humans see task flows
   before full command families.

## Decision

Task-level workflow complexity should become the next command-surface quality
metric. The current command count is useful but insufficient. Gira should
measure whether it reduces command, argument, decision, provider, fallback, and
cognitive nodes for representative tasks, then use that evidence before adding
more public command surface.

See [Workflow Efficiency KPIs](workflow-efficiency-kpis.md) for the product
metrics that turn these workflow-cost scores into raw `gh` reduction
percentages.
