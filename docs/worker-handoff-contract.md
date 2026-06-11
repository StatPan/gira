# Worker Handoff Contract

## Status

Public contract for `worker-handoff/v1`.

## Purpose

`worker-handoff/v1` is the handoff packet Gira emits when a ticket or queue item is safe enough for a coding agent, orchestration layer, or human operator to continue without rereading the whole repository.

It is harness-independent. Codex, Claude Code, opencode, ADK, custom kernels, and local scripts should be able to consume the same packet without provider-specific fields or prompt assumptions.

The contract is not an agent runner. It does not execute work, start a model, open a PR, or approve a mutation.

## Producers

The same contract should appear wherever Gira hands executable work to a worker:

| Producer | Embedding | Notes |
| --- | --- | --- |
| `gira ticket handoff <ticket> --json` | Top-level handoff payload. | Use when the caller already selected a ticket. |
| `gira queue handoff --json` | Embedded as `worker_handoff`. | Use when Gira selected or evaluated a queue item. |
| `gira queue take --dry-run --json` | Embedded as approval evidence before branch start. | The apply path still delegates to `ticket start`; it does not run a worker. |
| Goal handoff or human-review receipts | Embedded or linked from receipt evidence. | Use to preserve why delegated work stopped or moved to a human. |

## Required Fields

The first stable shape should include these fields when relevant:

| Field | Meaning |
| --- | --- |
| `schema_version` | Must be `worker-handoff/v1`. |
| `repo` | GitHub repository in `OWNER/REPO` form. |
| `issue` or `ticket` | Source GitHub issue number. |
| `title` | Human-readable work title. |
| `state` or `status` | Current lifecycle state from Gira/GitHub. |
| `labels` | Public workflow labels used for policy and lane decisions. |
| `readiness.ready` | Boolean handoff gate. |
| `readiness.blockers[]` | Stable blocker reason codes. Empty means no handoff blocker was found. |
| `readiness.warnings[]` | Non-blocking concerns the worker should preserve. |
| `readiness.findings[]` | Structured readiness findings, when available. |
| `acceptance_criteria` | Parsed or summarized completion expectations. |
| `allowed_next_commands[]` | Commands the receiving worker may run next without inventing workflow state. |
| `stop_reasons[]` | Reasons the handoff stopped instead of selecting executable work. |
| `next_action` | Gira's next-action code. |
| `next_step` | Human-readable next safe command or instruction. |
| `branch.expected` | Expected branch name for the ticket. |
| `branch.current` | Current branch when local checkout context is known. |
| `branch.trusted` | Whether Gira trusts the current branch binding. |
| `branch_policy` | Recorded base, actual PR base, mismatch, and policy diagnostics when known. |
| `pull_request` | Linked PR summary when one exists. |
| `checks_status` or `checks[]` | Check state when review or finish decisions depend on it. |
| `review_status` or `review` | Review decision when known. |
| `evidence` | Closing references, branch trust, receipt presence, or source evidence. |

Implementations may include additional fields, but consumers should not require provider-specific runtime fields.

## Readiness Semantics

`readiness.ready = true` means the packet is sufficiently specified for the next worker to continue under Gira policy. It does not mean the worker may merge, close, publish, or run mutating commands without the usual dry-run/apply approval boundary.

`readiness.ready = false` means the packet is still useful, but the receiver should stop and handle the blockers. Typical blockers include missing scope, missing acceptance criteria, missing evidence plan, ambiguous branch policy, or a queue item that belongs to review, finish, failed, blocked, or human-decision lanes.

Consumers should preserve blocker codes verbatim. Do not collapse them into a generic "not ready" state.

## Stop Reasons

Stop reasons explain why Gira did not hand off executable work. They are policy outputs, not parser failures.

Common stop reason families:

| Stop reason | Meaning | Safe response |
| --- | --- | --- |
| `no_agent_ready_item` | No eligible queue item exists. | Inspect queue inventory or ask a human. |
| `queue_not_handoff_safe` | The item is in a lane that should not start worker implementation. | Run the item's next safe command. |
| `worker_handoff_not_ready` | The selected item failed handoff readiness. | Refine the ticket or inspect findings. |
| `readiness_needs_refinement` | Ticket context is incomplete. | Add scope, acceptance, doctor impact, or evidence expectations. |
| `branch_policy_*` | Branch binding or base policy is ambiguous. | Resolve branch policy before starting or finishing. |
| `checks_*` | PR checks are pending or failing. | Wait, fix, or route to review depending on the check state. |
| `human_review` | Gira requires a human decision. | Stop autonomous execution and preserve context. |

Adapters should stop on unknown stop reasons unless the operator explicitly whitelists them.

## Allowed Next Commands

`allowed_next_commands[]` should contain exact Gira command families that are safe for the current handoff state.

Examples:

```json
[
  "gira ticket view 723 --repo StatPan/gira",
  "gira ticket start 723 --repo StatPan/gira --dry-run",
  "gira ticket status 723 --repo StatPan/gira --json"
]
```

Rules:

- Prefer Gira commands over raw `gh` or shell commands.
- Include `--dry-run` for any command that would prepare a mutation.
- Do not include `--apply` unless the producer is an explicit approval plan, and never treat worker handoff alone as apply approval.
- Do not include provider-specific model prompts as workflow commands.

If `allowed_next_commands[]` is absent, consumers should fall back to `next_step` only as human-readable guidance, not an executable shell string.

## Evidence Expectations

A worker handoff should carry enough evidence for the next actor to understand:

- what issue is being worked;
- why it is ready or why it stopped;
- what branch and PR are expected;
- what acceptance criteria define completion;
- what checks, reviews, or receipts are relevant;
- what the next safe Gira command is.

Workers should append new durable evidence through normal Gira ticket commands, PRs, checks, reviews, or receipts. They should not create hidden local workflow state that contradicts GitHub.

## Runtime Independence

The contract deliberately avoids:

- model names;
- provider-specific prompt roles;
- terminal session IDs;
- editor state;
- opaque local worker memory;
- hosted run URLs as required fields;
- shell command strings as the primary execution contract.

Runtime-specific systems may wrap a handoff packet in their own ledger, but the Gira packet should remain reconstructable from GitHub evidence plus Gira config.

## Consumer Rules

Consumers should:

- parse by `schema_version`;
- reject missing `repo` or ticket identifiers;
- stop on unknown blocker or stop reason codes;
- prefer JSON fields over prose;
- execute allowed Gira commands through direct argv construction;
- preserve Gira outputs as evidence;
- require a separate dry-run approval plan before any apply mutation.

Consumers should not:

- infer readiness by label alone;
- treat `next_step` as an arbitrary shell command;
- bypass Gira with raw `gh` when Gira exposes the operation;
- start a worker from review, finish, failed, blocked, or human-decision lanes;
- convert a handoff packet into merge, close, or publish authorization.

## Known Gaps

No separate implementation gap is required for this document. If future command output lacks one of the fields above, create a bounded CLI JSON hardening issue for that producer before adding MCP, adapter, or runtime-specific workarounds.
