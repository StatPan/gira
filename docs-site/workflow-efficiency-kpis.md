# Workflow Efficiency KPIs

Gira is GitHub-native. The final source of truth is still GitHub issues,
branches, PRs, checks, reviews, labels, comments, and milestones.

The measurable product claim is that an agent can reach the same GitHub-backed
outcome with less workflow labor, fewer unsafe decisions, and fewer
raw-provider escapes.

The current scores are modeled baselines from command graphs and workflow-node
counts. They are not live replay benchmarks or API instrumentation results.
Do not publish numeric reduction claims until an agent replay or instrumented
benchmark has recorded the exact fixture, commands, labels, PR actions,
provider calls, and post-apply verification.

Empirical validation should compare three paths:

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
| Workflow burden reduction | `(raw_gh_workflow_cost - gira_workflow_cost) / raw_gh_workflow_cost` | How much less task burden an agent must handle. |
| Agent burden reduction | `(raw_agent_steps - gira_agent_steps) / raw_agent_steps` | How much less an agent must infer, sequence, or repair. |
| Discovery read reduction | `avoided_discovery_reads / raw_gh_discovery_reads` | How much Terraform-like state can reduce repeated provider lookup. |
| Provider API reduction | `(raw_gh_provider_calls - gira_provider_calls) / raw_gh_provider_calls` | How much GitHub API pressure is reduced. |
| Fallback escape rate | `gira_fallback_steps / gira_workflow_steps` | Whether users stay inside the Gira workflow surface. |

Workflow burden uses the node model from
[CLI Workflow Complexity](/cli-workflow-complexity): commands, arguments,
decisions, provider leakage, fallback, and cognitive concepts.

## Agent Replay Basis

Benchmark an agent performing concrete GitHub-native actions:

- ticket discovery and normalization;
- label and status transitions;
- branch binding;
- PR creation or validation with closing text;
- review packet and durable comments;
- check and review inspection;
- finish or merge with issue closure evidence;
- parent/child links and supersede flows;
- raw provider escapes such as `gh api`, REST, or GraphQL.

Modeled scores can guide product work, but public percentages require replay
or instrumentation evidence.

## Stateful Direction

Terraform-like local state should reduce discovery work, not replace GitHub as
truth. Good state candidates include ticket-to-branch binding, ticket-to-PR
binding, parent/child snapshots, dry-run approval plans, finish readiness
snapshots, provider budget observations, and no-op convergence receipts.

Final mergeability, checks, review blockers, issue existence, and permission
decisions must still be refreshed from GitHub before irreversible actions.

Canonical source:
[docs/workflow-efficiency-kpis.md](https://github.com/StatPan/gira/blob/main/docs/workflow-efficiency-kpis.md).
