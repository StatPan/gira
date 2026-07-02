# Workflow Efficiency KPIs

Gira is GitHub-native, so the canonical state change is intentionally the same
as a correct raw `gh` workflow: GitHub issues, labels, branches, PRs, checks,
reviews, comments, and milestones remain the source of truth.

The product claim is not that Gira creates a different backend. The claim to
prove is that an agent can do the same GitHub-native work with less workflow
labor, fewer unsafe decisions, and fewer raw-provider escapes.

The current numbers in this document are modeled baselines from the #807
workflow-node diagnostic. They are not live replay benchmarks, wall-clock
measurements, or GitHub API instrumentation results yet.

Do not publish numeric reduction claims from modeled baselines. Public numbers
require a replay or instrumented benchmark with a fixed fixture and retained
evidence.

To validate the claim empirically, each golden workflow must compare:

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

## Benchmark Subject

The primary benchmark subject is an agent, not a generic human operator.

An agent benchmark must record every explicit action the agent had to perform
or reason about. The comparison should include both visible commands and the
GitHub-native work those commands complete.

Required action inventory:

| Action area | Raw `gh` action examples | Gira action examples | Count when |
| --- | --- | --- | --- |
| Ticket discovery | `gh issue view`, `gh issue list`, inspect labels/body/milestone | `gira ticket view`, `gira ticket status`, `gira queue next` | Agent must find or validate the work item. |
| Ticket normalization | `gh issue edit --add-label`, edit issue body, add status/type labels | `gira ticket new`, future normalization command, `gira ticket note` | Agent must make an issue executable or record context. |
| Branch binding | create branch name, checkout base, remember issue number | `gira ticket start --dry-run|--apply` | Agent starts issue-backed work. |
| Label/status transition | add/remove `status:*`, `type:*`, `priority:*`, `agent:*`, `lane:*` labels | lifecycle commands that normalize status labels | Agent changes workflow state. |
| PR creation/validation | `gh pr create`, set title/body/base/head, ensure `Closes #N` | `gira ticket pr --dry-run|--apply` | Agent opens or verifies a linked PR. |
| Review packet | `gh pr diff`, inspect changed files, summarize risk | `gira ticket review`, `gira ticket self-review` | Agent prepares review evidence. |
| Comments/receipts | `gh issue comment`, `gh pr comment`, hand-written finish note | `gira ticket note`, finish receipt, supersede notes | Agent writes durable context. |
| Checks and review state | `gh pr checks`, `gh pr view`, status rollup polling | `gira ticket checks`, `gira ticket wait` | Agent waits, branches, or repairs based on CI/review. |
| Finish/merge | `gh pr merge`, delete branch, close issue, relabel Done | `gira ticket finish --dry-run|--apply` | Agent completes the work unit. |
| Parent/child links | raw sub-issue API calls or provider IDs | `gira ticket new --parent`, `gira ticket parent` | Agent creates or repairs hierarchy. |
| Supersede/replace | create replacement issue, cross-comment, relabel, close old issue | `gira ticket supersede --dry-run|--apply` | Agent replaces stale or wrong work. |
| Provider escape | direct REST/GraphQL/`gh api` calls | fallback from Gira to provider command | Agent leaves the product workflow surface. |

The replay log must include the exact command sequence, the required arguments
the agent had to discover, the decision points it had to make, and the durable
GitHub evidence produced.

## Primary KPIs

### Workflow Burden Reduction

This is the main product KPI for agent-operated workflow. It measures how much
task-level workflow burden Gira removes compared with raw `gh`.

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

## Modeled Baselines

The #807 diagnostic contains modeled workflow-node baselines. Treat those
tables as internal product diagnosis only. Do not copy their percentages into
README, release notes, landing pages, or benchmark claims.

Use modeled baselines to choose what to validate next. A Gira workflow that
does not beat the raw `gh` baseline needs a reason: stronger safety,
idempotency, auditability, or agent reliability. If it has none, improve or
remove that surface before publishing the workflow as a product advantage.

## Evidence Levels

Use explicit evidence labels when publishing KPI numbers:

| Evidence level | Meaning | Allowed claim |
| --- | --- | --- |
| Modeled | Calculated from documented command graphs and workflow-node counts. | Internal diagnosis only; do not publish reduction percentages. |
| Replayed | Raw `gh` and Gira agent happy paths were executed in a disposable repo and steps were recorded. | "Agent replay reduced workflow steps by X%." |
| Instrumented | Provider calls, GraphQL cost, REST requests, wall time, and no-op convergence were captured during replay. | "Instrumentation reduced provider reads by X%." |

Do not present modeled percentages as empirical benchmarks. Before using any
number as a benchmark claim, record the exact repo fixture, command transcript,
labels touched, PR actions, comments, provider call counts, API quota deltas,
timestamps, and post-apply no-op check.

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

Use public numbers only after replay or instrumentation evidence exists. The
score should make Gira easier to trust, not turn into hidden marketing math.

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
