# Workflow Efficiency KPIs

Gira is GitHub-native, so the canonical state change is intentionally the same
as a correct raw `gh` workflow: GitHub issues, labels, branches, PRs, checks,
reviews, comments, and milestones remain the source of truth.

The product claim is not that Gira creates a different backend. The claim is
that Gira reduces the amount of workflow labor needed to reach the same
GitHub-backed outcome.

The current numbers in this document are modeled baselines from the #807
workflow-node diagnostic. They are not live replay benchmarks, wall-clock
measurements, or GitHub API instrumentation results yet.

To validate the claim empirically, each golden workflow should compare:

```text
raw gh baseline
vs
current Gira workflow
vs
stateful Gira target
```

The `stateful Gira target` is the future Terraform-like direction: local
projection, cache, ledger, or recent command state can avoid repeated discovery
reads and repeated operator decisions, while GitHub remains authoritative for
workflow truth.

## Primary KPIs

### Workflow Burden Reduction

This is the main product KPI for humans. It measures how much task-level
workflow burden Gira removes compared with raw `gh`.

```text
workflow_burden_reduction =
  (raw_gh_workflow_cost - gira_workflow_cost)
  / raw_gh_workflow_cost
```

Use the workflow cost model from [CLI Workflow Complexity](cli-workflow-complexity.md):

```text
workflow_cost =
  command_nodes
  + 0.5 * argument_nodes
  + 2.0 * decision_nodes
  + 2.0 * provider_nodes
  + 3.0 * fallback_nodes
  + 1.5 * cognitive_nodes
```

This can be used in product copy because it describes what users feel:
commands, required arguments, decisions, provider leakage, fallback, and
concept mapping.

### Agent Burden Reduction

This is the main AX KPI. It measures how much less an agent has to infer,
sequence, or repair.

```text
agent_burden_reduction =
  (raw_agent_steps - gira_agent_steps)
  / raw_agent_steps
```

Count `raw_agent_steps` as:

- visible commands the agent must run;
- required IDs or values it must discover;
- decisions it must make without a structured blocker;
- waits that do not have a bounded command contract;
- non-JSON outputs it must parse for workflow state;
- raw `gh` fallback commands.

Gira improves this KPI when commands emit stable JSON, `schema_version`,
`next_step`, blockers, dry-run approval plans, and finish receipts.

### Discovery Read Reduction

This is the main Terraform-like state KPI. It measures how much repeated
provider discovery can be avoided by local projection or recent command state.

```text
discovery_read_reduction =
  avoided_discovery_reads
  / raw_gh_discovery_reads
```

Discovery reads include:

- resolving issue state, labels, milestone, assignee, and parent/child links;
- finding the work branch for a ticket;
- finding or validating the linked PR;
- checking whether the PR body closes the ticket;
- looking up checks, review decision, mergeability, and status labels;
- retrieving provider IDs that users should not need to handle.

Do not count required final verification as avoidable. Before merge or finish,
Gira must refresh the minimum GitHub evidence needed to avoid acting on stale
state.

### Provider API Efficiency

This measures GitHub API pressure, separate from human workflow efficiency.

```text
provider_api_reduction =
  (raw_gh_provider_calls - gira_provider_calls)
  / raw_gh_provider_calls
```

This number may be lower than workflow burden reduction because Gira may use
the same GitHub backend calls while hiding them behind safer commands. That is
acceptable. API efficiency matters most when GraphQL cost, REST rate limits, or
secondary limits become a product constraint.

### Fallback Escape Rate

This measures whether Gira keeps the operator inside the product workflow.
Unlike the other KPIs, this is not a raw `gh` reduction formula because raw
`gh` is already the provider surface. The target is zero escape from Gira for
workflow steps that Gira claims to own.

```text
fallback_escape_rate =
  gira_fallback_steps
  / gira_workflow_steps
```

Count fallback when the Gira workflow still forces the user or agent to leave
Gira for a lifecycle step that Gira claims to own.

### No-Op Convergence

This measures whether a Terraform-like plan/apply flow actually converges.

```text
no_op_convergence =
  apply_success
  and next_dry_run_planned_actions == 0
```

A command that supports `--dry-run|--apply` should be able to prove that the
same operation becomes a no-op after a successful apply, unless the underlying
provider state changed externally.

## Example KPI Table

The current #807 diagnostic gives a first modeled burden baseline:

| Workflow | Raw `gh` cost | Gira cost | Burden reduction | Interpretation |
| --- | ---: | ---: | ---: | --- |
| Ticket lifecycle | 42.0 | 20.5 | 51% | Gira halves the task burden by binding issue, branch, PR, checks, labels, and finish evidence. |
| Create native sub-issue | 22.0 | 10.5 | 52% | `ticket new --parent` hides native sub-issue provider details. |
| Attach existing sub-issue | 20.0 | 12.0 | 40% | Good reduction, but the command shape is less intuitive than it should be. |
| Supersede ticket | 30.0 | 12.0 | 60% | Gira replaces a manual audit trail with one bounded dry-run/apply command pair. |

The table should be treated as a living product scorecard. A Gira workflow that
does not beat the raw `gh` baseline needs a reason: stronger safety,
idempotency, auditability, or agent reliability. If it has none, improve or
remove that surface.

## Evidence Levels

Use explicit evidence labels when publishing KPI numbers:

| Evidence level | Meaning | Allowed claim |
| --- | --- | --- |
| Modeled | Calculated from documented command graphs and workflow-node counts. | "Model estimates 51% lower burden." |
| Replayed | Raw `gh` and Gira happy paths were executed in a disposable repo and steps were recorded. | "Replay reduced operator steps by X%." |
| Instrumented | Provider calls, GraphQL cost, REST requests, wall time, and no-op convergence were captured during replay. | "Instrumentation reduced provider reads by X%." |

Do not present modeled percentages as empirical benchmarks. Before using a KPI
as a benchmark claim, record the exact repo fixture, commands, provider call
counts, API quota deltas, timestamps, and post-apply no-op check.

## Measurement Procedure

For each golden workflow:

1. Write the desired GitHub-native outcome.
2. Write the raw `gh` happy path needed to reach that outcome.
3. Count command, argument, decision, provider, fallback, and cognitive nodes.
4. Write the current Gira happy path.
5. Count the same nodes.
6. List provider reads that are repeated discovery rather than required
   final verification.
7. Estimate the stateful Gira target by removing only avoidable discovery
   reads and avoidable repeated decisions.
8. Report percent reduction for workflow burden, agent burden, discovery
   reads, and provider calls. Report fallback escape rate separately.

Use one table per workflow:

```text
workflow: ticket lifecycle
outcome: issue started, linked PR opened, checks observed, PR merged, issue done

raw_gh:
  workflow_cost:
  agent_steps:
  provider_calls:
  discovery_reads:
  fallback_steps:

current_gira:
  workflow_cost:
  agent_steps:
  provider_calls:
  discovery_reads:
  fallback_steps:

stateful_gira_target:
  workflow_cost:
  agent_steps:
  provider_calls:
  discovery_reads:
  fallback_steps:

reported:
  workflow_burden_reduction:
  agent_burden_reduction:
  discovery_read_reduction:
  provider_api_reduction:
  fallback_escape_rate:
```

## How KPIs Drive Product Work

Use the KPIs as a quality gate:

- If Gira reduces burden by 50% or more, teach that workflow prominently.
- If Gira reduces burden by 20-49%, inspect object order, defaults, next steps,
  and fallback.
- If Gira reduces burden by less than 20%, the workflow needs a product reason
  beyond convenience.
- If provider API reduction is weak but workflow burden reduction is strong,
  keep the workflow but consider stateful projection only when API budget hurts.
- If discovery read reduction is high, local state or ledger is probably worth
  implementing.

Use the same numbers for product proof:

```text
Gira reduced ticket lifecycle burden by 51% against raw gh.
Gira reduced native sub-issue creation burden by 52%.
Gira reduced supersede workflow burden by 60%.
```

These statements are useful only when the raw `gh` baseline, Gira workflow, and
counting method are visible. The score should make Gira easier to trust, not
turn into hidden marketing math.

## Stateful Direction

Terraform-like state should be justified by these KPIs, not by architecture
preference.

Good candidates for local projection or ledger:

- ticket to branch binding;
- ticket to PR binding;
- last observed parent/child issue graph;
- last dry-run approval plan;
- last finish readiness snapshot;
- provider budget and request cost observations;
- no-op convergence receipts.

Bad candidates for local authority:

- final mergeability;
- final check state;
- final review blockers;
- whether the issue or PR still exists;
- security-sensitive permission decisions.

The rule is simple: local state may reduce discovery and improve ergonomics,
but GitHub evidence must still approve irreversible transitions.
