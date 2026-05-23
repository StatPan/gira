# Worker Boundary And Provenance

Gira is a GitHub-native work contract and control-plane CLI. It is not a
coding agent, cloud VM, or long-running worker runtime.

The boundary is deliberate:

- GitHub remains the source of truth for issues, labels, milestones, branches,
  PRs, checks, reviews, comments, and merge state.
- Gira compiles GitHub state into ticket contracts, readiness reports, branch
  policy checks, worker handoff packets, review packets, audit findings, and
  finish receipts.
- External workers such as Jules, Codex, Claude Code, Copilot coding agent,
  OpenHands, or local agents execute code changes and open PRs.
- `agent-kernel` can later sit below Gira as durable execution infrastructure
  for approvals, retries, event ledgers, and trace correlation, but Gira should
  not depend on it for the core GitHub workflow.

## Operating Flow

```text
GitHub issue
  -> Gira ticket readiness and worker-handoff/v1
  -> external worker implements on a bounded branch
  -> PR with closing reference, evidence, and optional provenance
  -> Gira review/pr-readiness/finish-readiness
  -> Gira finish receipt and GitHub closure
```

Gira owns the contract, not the worker process. A worker may receive
`gira ticket handoff --json`, a prompt from `gira ticket prompt`, or a review
packet from `gira ticket review`, but the worker is responsible for execution
inside its own environment.

## Gira Owns

- Work-order shape: goal, scope, acceptance criteria, evidence expectations,
  and next safe command.
- Ticket readiness: whether an issue is ready, blocked, or needs refinement.
- Branch policy: recorded base branch, expected work branch, and PR base
  mismatch detection.
- Review context: linked PR, changed files, checks, review state, repository
  guidance, and verdict schema.
- Finish convergence: checks, closing references, label normalization, merge,
  close, and finish receipt.
- Audit and drift reports that explain workflow state from GitHub evidence.

## Workers Own

- Reading the repository and local instructions.
- Editing files within the ticket scope.
- Running focused and broad verification commands.
- Opening or updating the PR with `Closes #N`, `Fixes #N`, or `Resolves #N`.
- Recording implementation evidence, caveats, and residual risk.
- Supplying optional telemetry or provenance metadata when agent-assisted work
  needs traceability.

## Gira Does Not Own

- Hidden repository state outside GitHub.
- Long-running worker orchestration.
- Cloud development VMs or sandboxes.
- LLM invocation, model routing, token accounting, or prompt hosting.
- Vendor-specific worker labels such as `agent:claude` or `agent:codex` as a
  primary schema. Exact tools and models belong in metadata.

## Provenance Envelope

The provenance envelope is optional metadata that complements the existing AI
Delivery Telemetry block. It is useful when a worker adapter or reviewer needs
machine-readable traceability without adding high-cardinality labels.

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

Use it in issue bodies, PR bodies, or structured comments when the work was
agent-assisted or when a durable execution system needs correlation. Keep it
visible in GitHub so review and audit can inspect the same evidence as humans.

## Relationship To AI Delivery Telemetry

AI Delivery Telemetry answers whether agent-assisted work declared its delivery
context. The provenance envelope adds optional trace fields for tools that need
correlation across attempts, prompts, reviews, or event ledgers.

They should work together:

- Telemetry can stay human-readable and compact.
- Provenance can carry trace IDs, worker IDs, attempt IDs, exact tool names,
  exact model names, prompt source, and human intervention notes.
- Neither block replaces GitHub PR evidence, checks, reviews, or finish
  receipts.
- Human-only work does not need provenance unless the repository policy asks
  for it.

## Labels Versus Metadata

Labels should remain low-cardinality workflow policy. Use labels for lane and
state, such as `lane:agent`, `lane:hybrid`, `requires-human-approval`, and
`status:in-review`.

Do not create labels for exact tools, model versions, prompt IDs, trace IDs, or
attempt IDs. Put those values in AI Delivery Telemetry or the provenance
envelope. This keeps GitHub labels useful for filtering while preserving exact
metadata for audit.

## Agent-Kernel Placement

`agent-kernel` is optional infrastructure below Gira. It can provide durable
execution mechanics such as approval gates, retries, event logs, trace IDs, and
worker leases. Gira should consume its GitHub-visible results or provenance
metadata, but Gira should still be able to operate without it.

The Gira-side adapter contract is defined in
[Agent-Kernel Adapter Contract](agent-kernel-adapter-contract.md). That contract
classifies Gira commands as read, dry-run mutation, apply mutation, or
unsupported, and defines the evidence an adapter should preserve before any
approved apply.

The durable stack should look like this:

```text
Gira contract/readiness layer
  -> optional agent-kernel durable execution layer
  -> worker runtime or coding agent
  -> GitHub PR/check/review evidence
```

This keeps Gira worker-neutral and prevents the roadmap from drifting into a
coding-agent clone.
