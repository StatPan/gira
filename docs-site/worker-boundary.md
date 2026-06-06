# Worker Boundary

Gira is a GitHub-native work contract and control-plane CLI. It is not a coding
agent, cloud VM, or long-running worker runtime.

## Boundary

- GitHub remains the source of truth for issues, labels, milestones, branches,
  PRs, checks, reviews, comments, and merge state.
- Gira compiles GitHub state into ticket contracts, readiness reports, branch
  policy checks, worker handoff packets, review packets, audit findings, and
  finish receipts.
- External workers such as Jules, Codex, Claude Code, Copilot coding agent,
  OpenHands, or local agents execute code changes and open PRs.
- `agent-kernel` can later sit below Gira as durable execution infrastructure
  for approvals, retries, event ledgers, and trace correlation.

## Flow

```text
Gira contract -> worker -> PR -> Gira readiness -> finish
```

Expanded:

```text
GitHub issue
  -> Gira ticket readiness and worker-handoff/v1
  -> external worker implements on a bounded branch
  -> PR with closing reference, evidence, and optional provenance
  -> Gira review/pr-readiness/finish-readiness
  -> Gira finish receipt and GitHub closure
```

`gira ticket handoff --json` is the worker-neutral contract surface. It gives
adapters the goal, scope, acceptance criteria, branch policy/base context,
readiness, evidence expectations, required checks, review expectations,
prohibited actions, telemetry/provenance expectations, and next safe command.
Gira does not launch the worker.

`worker-run/v1` is the optional runtime evidence manifest for worker attempts.
It links run/session/model metadata, local prompt/event/stderr/result files,
GitHub-visible evidence, verification commands, commits, comments, status
transitions, and human decision gates back to the GitHub issue or PR. Gira can
export that manifest for dashboards, but it still does not start, stop, retry,
or supervise the worker process.

## Queue Placement

The Agent Handoff Queue sits before worker execution. `gira queue list` and
`gira queue next` select from `workspace-queues/v1`; `gira queue handoff`
embeds `worker-handoff/v1`; and `gira queue take` delegates only to
`ticket start`.

Queue commands can choose work, produce a worker packet, or start a trusted
branch, but they do not invoke Codex, route a model, supervise retries, or mark
work complete. See [Agent Handoff Queue](/agent-handoff-queue) for command
roles and stop reasons.

## Ownership

Gira owns:

- work-order shape and next safe command
- ticket readiness and PR readiness
- branch policy and PR base validation
- review packets and finish readiness
- audit/drift reports and finish receipts

Workers own:

- reading the repository and local instructions
- editing files within ticket scope
- running verification commands
- opening or updating the PR with `Closes #N`, `Fixes #N`, or `Resolves #N`
- recording evidence, caveats, residual risk, and optional provenance

Gira does not own:

- hidden repository state outside GitHub
- long-running worker orchestration
- cloud development VMs or sandboxes
- LLM invocation, model routing, token accounting, or prompt hosting
- high-cardinality labels for exact worker tools or model versions

## Provenance Envelope

The provenance envelope is optional metadata that complements AI Delivery
Telemetry. Use it when a worker adapter or reviewer needs traceability without
adding labels for exact tools, models, prompts, or attempts.

```yaml
provenance:
  trace_id:
  span_id:
  worker_id:
  attempt_id:
  implementation_tool:
  implementation_model:
  review_tool:
  review_model:
  prompt_source:
  human_interventions:
  completed_at:
```

Keep provenance visible in GitHub issue bodies, PR bodies, or structured
comments so humans, review packets, finish receipts, and audit reports inspect
the same evidence.

## Labels Versus Metadata

Use labels for low-cardinality workflow policy:

- `lane:agent`
- `lane:hybrid`
- `lane:human`
- `requires-human-approval`
- `status:in-review`

Put exact implementation tool names, model names, trace IDs, prompt IDs,
attempt IDs, and review tool names in AI Delivery Telemetry or the provenance
envelope, not labels.

## Agent-Kernel Placement

`agent-kernel` is optional infrastructure below Gira. It can provide durable
execution mechanics such as approval gates, retries, event logs, trace IDs, and
worker leases. Gira should consume its GitHub-visible results or provenance
metadata, but Gira should still operate without it.

```text
Gira contract/readiness layer
  -> optional agent-kernel durable execution layer
  -> worker runtime or coding agent
  -> GitHub PR/check/review evidence
```
