# ADR 0003: CLI and MCP parity boundary

## Status

Accepted for #738.

## Context

Gira now has two access paths for agent workflow control:

- the local CLI, including text output, `--json` contracts, `--dry-run`, and `--apply`;
- the local MCP server, which wraps stable read-only Gira CLI JSON commands for coding-agent harnesses.

Those paths must not become two workflow models. If CLI users and MCP agents learn different meanings for readiness, blockers, handoff, finish, or next actions, Gira loses the evidence-backed lifecycle that makes it safe to use as an agent workflow control plane.

## Decision

The CLI command model is the canonical workflow contract.

Gira JSON contracts expose workflow state, readiness, blockers, handoff packets, receipts, and dry-run plans. MCP tools are adapters over those contracts. MCP must not introduce independent lifecycle semantics, hidden state transitions, or MCP-only definitions of readiness.

MCP may expose read, status, checks, queue, handoff, and dry-run planning evidence. Apply/mutation remains bound to the CLI `--apply` boundary unless a future ADR explicitly supersedes this rule with an equal or stricter approval boundary.

## Product rule

CLI and MCP are two access paths to the same Gira lifecycle, not two separate workflows.

The intended mixed-mode flow is:

1. MCP reads state or prepares context from stable Gira JSON contracts.
2. The agent or operator uses CLI `--dry-run` for any mutation plan.
3. A human or supervising runtime approves the mutation boundary.
4. CLI `--apply` performs the mutation.
5. MCP or CLI reads back the resulting GitHub-backed state.

MCP output can guide the agent toward the next safe Gira action. It is not a shell script and must not be executed blindly.

## Parity principles

MCP tools should preserve these CLI concepts:

| Concept | Parity rule |
| --- | --- |
| Schema versions | MCP payloads should preserve underlying Gira schema names such as `ticket-status/v1`, `queue-handoff/v1`, `worker-handoff/v1`, and `finish-readiness/v1`. |
| Readiness | MCP must report CLI/Gira readiness fields, not reinterpret readiness locally. |
| Blockers and stop reasons | MCP should preserve stable blocker and stop reason codes verbatim. |
| Next actions | MCP may expose `next_action`, `next_step`, and `next_safe_command`, but those are recommendations over canonical Gira commands. |
| Command mapping | MCP tool names may be friendlier than CLI commands, but each tool must map back to a documented Gira command family. |
| Mutation boundary | MCP v1 must not expose `--apply` or hidden mutations. Future MCP mutation tools require a later ADR. |
| Evidence | GitHub evidence, Gira receipts, and CLI JSON remain the workflow evidence boundary. |

## Prohibited divergences

MCP must not:

- define MCP-only ticket states, queue states, readiness values, blockers, or finish semantics;
- maintain hidden workflow state that competes with GitHub evidence or Gira receipts;
- execute `next_step` or `next_safe_command` as arbitrary shell strings;
- call raw `gh` or shell as a fallback when Gira exposes the workflow operation;
- expose `--apply`, repository mutation, GitHub mutation, or local checkout mutation in v1;
- let hosted logs or adapter ledgers become the workflow source of truth;
- silently downgrade CLI blockers into best-effort MCP warnings;
- infer readiness from labels alone when CLI JSON has richer evidence.

## Future dry-run MCP tools

Dry-run MCP tools may be introduced later if they preserve the CLI approval model.

A future dry-run MCP tool should:

- call or reproduce the same CLI dry-run JSON contract;
- include the planned command, planned actions, blockers, warnings, and post-apply verification command;
- emit `gira-approval-plan/v1` or a successor approval evidence contract;
- refuse to perform the apply itself unless a later ADR defines a stricter approved mutation boundary.

A dry-run MCP tool does not authorize mutation by itself.

## Hosted MCP implication

Hosted MCP inherits the same parity rule.

A hosted service may become a managed control surface later, but it must not become a second workflow brain. Hosted state, logs, caches, and adapter ledgers may support diagnostics and audit, but GitHub evidence and explicit Gira receipts remain canonical workflow evidence.

Hosted apply/mutation is out of scope until a later ADR defines consent, token ownership, tenant isolation, audit receipts, rollback expectations, and an approval boundary equivalent to or stricter than CLI `--apply`.

## Consequences

Positive consequences:

- Agents can move between CLI, CLI JSON, and MCP without learning a second lifecycle.
- MCP integrations stay small and safer because they wrap stable Gira contracts.
- Human operators can audit MCP decisions using the same CLI outputs.
- Hosted MCP can be evaluated later without weakening local safety properties.

Costs and tradeoffs:

- Some MCP users will still need CLI access for mutations.
- MCP tool names and payloads must stay disciplined around CLI concepts.
- Hosted MCP product work needs more policy design before implementation.

## Follow-ups

- #739 should document the mixed CLI/MCP workflow with side-by-side examples.
- #740 should define the hosted MCP service boundary before any hosted implementation starts.
- Future MCP dry-run or mutation tools require a separate issue and, for mutation, a successor ADR.
