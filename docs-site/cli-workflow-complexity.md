# CLI Workflow Complexity

Gira should reduce task burden compared with raw `gh`. The right measurement
is not character count. The useful unit is a workflow node: one thing a human
operator or agent must understand, choose, or execute to finish a task.

## Node Types

| Node | Meaning |
| --- | --- |
| Command | A visible CLI command the operator or agent must run. |
| Argument | A required value such as repo, ticket, parent, label, title, body, or SHA. |
| Decision | A workflow branch such as dry-run/apply, wait/fix, draft/ready, or merge/stop. |
| Provider | A provider primitive exposed to the user, such as issue IDs, REST endpoints, PR head SHAs, or Project field IDs. |
| Fallback | A point where the user must leave Gira and use raw `gh`. |
| Cognitive | A product concept the user must map correctly: ticket, issue, parent, child, epic, goal, PR, status, or queue. |

Character count is a weak proxy. Object order, decision count, provider
leakage, and fallback risk usually explain UX/AX discomfort better.

## Representative Findings

| Flow | Gira score | Raw `gh` score | Diagnosis |
| --- | ---: | ---: | --- |
| Ticket lifecycle | 20.5 | 42.0 | Gira has several commands, but each binds lifecycle state and emits next safe actions. |
| Create a native sub-issue | 10.5 | 22.0 | `ticket new --parent` is a good creation-time surface. |
| Attach an existing sub-issue | 12.0 | 20.0 | `ticket parent CHILD --set PARENT` is compact but not the most intuitive object order. |
| Supersede a ticket | 12.0 | 30.0 | Gira is much simpler when dry-run is bounded and observable. |

The current attach-existing-sub-issue command should remain canonical:

```bash
gira ticket parent 456 --set 123 --dry-run
```

But a future ergonomic alias may be clearer:

```bash
gira ticket child add 123 456 --dry-run
```

The second form follows the user's sentence: add child `#456` under parent
`#123`.

## Review Gate

Before adding or teaching a workflow command, answer:

1. What task does the command complete?
2. What is the Gira command graph for the happy path?
3. What is the raw `gh` graph for the same task?
4. Which provider nodes does Gira hide?
5. Which decisions remain visible?
6. Does each step emit a next safe command or structured blocker?
7. Does the object order match the user's sentence?
8. What fallback path remains?

Canonical source:
[docs/cli-workflow-complexity.md](https://github.com/StatPan/gira/blob/main/docs/cli-workflow-complexity.md).

Related:
[Workflow Efficiency KPIs](/workflow-efficiency-kpis).
