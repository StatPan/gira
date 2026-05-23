# State Model

Gira keeps GitHub as the source of truth, but it does not turn every operating
condition into a GitHub label.

The state model is:

```text
GitHub labels = small public workflow taxonomy
Gira JSON = computed operating state
GitHub comments = durable receipts and handoffs
Local files/cache = configuration, bindings, and acceleration
```

## Ownership

| Owner | Examples | Use |
| --- | --- | --- |
| GitHub labels | `status:ready`, `status:in-progress`, `status:in-review`, `status:blocked`, `status:done`, `type:*`, `priority:*`, `area:*`, `agent:*` | Public state humans can scan in GitHub. |
| GitHub evidence | Issues, PRs, checks, reviews, milestones, comments, closing references. | Durable proof of work and completion. |
| Gira JSON | `ticket_readiness`, `pr_readiness`, `finish-readiness/v1`, `workspace-queues/v1`, `goal-next/v1`, blockers, next steps. | Rich computed state for CLI, agents, adapters, and future UI. |
| Receipts | `finish-receipt/v1`, `goal-finish-receipt/v1`, supersede notes, handoff notes. | Durable audit comments explaining a decision. |
| Local config/cache | Workspace registry, repo registry, branch policy records, cache. | Ergonomics and performance, not hidden completion state. |

## Why Not More Labels?

Labels should stay few and stable. Conditions like `finish_ready`,
`review_needed`, `failed_check`, `missing_review`, or
`human_review_handoff_present` are computed from richer evidence. If each became
a label, the label taxonomy would drift and operators would need to clean up
labels that Gira can already calculate.

Use labels for coarse workflow state. Use JSON for precise operating state.

## Goal Vocabulary

| Term | Meaning |
| --- | --- |
| Ticket | One executable work packet, normally one branch and one PR. |
| PR | Reviewable change and implementation evidence. |
| Milestone | Time, phase, sprint, or release grouping. |
| Epic | Large GitHub issue that groups work. |
| Goal | A GitHub issue that Gira interprets as delegated multi-ticket work with child tickets, stop conditions, and finish receipts. |
| Workspace queue | Computed view over many tickets and PRs, such as agent-ready or review-needed. |

`goal next` does not mean milestone next. It means: choose the next safe child
ticket inside a delegated goal issue, or stop with a human-review reason.

## Future UI Boundary

The CLI and JSON contracts are the computation layer. Future UI should render
goal graphs, workspace queues, finish evidence, and blockers from these
contracts instead of creating a second workflow model.

Canonical source: [docs/state-model.md](https://github.com/StatPan/gira/blob/main/docs/state-model.md).
