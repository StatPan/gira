# Workflow Efficiency KPIs

Gira is GitHub-native. The final source of truth is still GitHub issues,
branches, PRs, checks, reviews, labels, comments, and milestones.

The measurable product claim is that Gira reduces the workflow labor needed to
reach the same GitHub-backed outcome.

Compare three paths:

```text
raw gh baseline
vs
current Gira workflow
vs
stateful Gira target
```

## Main KPIs

| KPI | Formula | What it proves |
| --- | --- | --- |
| Workflow burden reduction | `(raw_gh_workflow_cost - gira_workflow_cost) / raw_gh_workflow_cost` | How much less task burden a human feels. |
| Agent burden reduction | `(raw_agent_steps - gira_agent_steps) / raw_agent_steps` | How much less an agent must infer, sequence, or repair. |
| Discovery read reduction | `avoided_discovery_reads / raw_gh_discovery_reads` | How much Terraform-like state can reduce repeated provider lookup. |
| Provider API reduction | `(raw_gh_provider_calls - gira_provider_calls) / raw_gh_provider_calls` | How much GitHub API pressure is reduced. |
| Fallback escape rate | `gira_fallback_steps / gira_workflow_steps` | Whether users stay inside the Gira workflow surface. |

Workflow burden uses the node model from
[CLI Workflow Complexity](/cli-workflow-complexity): commands, arguments,
decisions, provider leakage, fallback, and cognitive concepts.

## Current Baseline

| Workflow | Raw `gh` cost | Gira cost | Burden reduction |
| --- | ---: | ---: | ---: |
| Ticket lifecycle | 42.0 | 20.5 | 51% |
| Create native sub-issue | 22.0 | 10.5 | 52% |
| Attach existing sub-issue | 20.0 | 12.0 | 40% |
| Supersede ticket | 30.0 | 12.0 | 60% |

Use these numbers as a scorecard. If a Gira workflow does not beat raw `gh`,
it needs a clear product reason: safety, idempotency, audit evidence, or agent
reliability.

## Stateful Direction

Terraform-like local state should reduce discovery work, not replace GitHub as
truth. Good state candidates include ticket-to-branch binding, ticket-to-PR
binding, parent/child snapshots, dry-run approval plans, finish readiness
snapshots, provider budget observations, and no-op convergence receipts.

Final mergeability, checks, review blockers, issue existence, and permission
decisions must still be refreshed from GitHub before irreversible actions.

Canonical source:
[docs/workflow-efficiency-kpis.md](https://github.com/StatPan/gira/blob/main/docs/workflow-efficiency-kpis.md).
