# CLI And MCP Workflow Parity

## Purpose

Gira can be used through the local CLI, CLI JSON automation, and MCP tools. Those access paths should feel like the same workflow.

The rule is simple:

> MCP may guide the agent toward the next safe Gira action, but MCP output is not a shell script and should not be executed blindly.

CLI and MCP are two access paths to the same evidence-backed Gira lifecycle. The CLI command model remains canonical, as defined by [ADR 0003](adr/0003-cli-and-mcp-parity-boundary.md).

## Recommended Mixed-Mode Flow

Use MCP for read/context steps and CLI for mutation steps.

```text
1. MCP reads state: queue_next / ticket_view / ticket_status.
2. MCP prepares context: queue_handoff or finish_plan.
3. CLI previews mutation: gira ... --dry-run --json.
4. Human or supervising runtime approves.
5. CLI applies mutation: gira ... --apply.
6. MCP or CLI reads back final ticket/PR status.
```

This keeps agent ergonomics high without creating an MCP-only workflow model.

## Surface Boundary

| Need | Preferred surface | Reason |
| --- | --- | --- |
| Read ticket context | MCP or CLI JSON | Both expose the same Gira state contract. |
| Read queue selection | MCP or CLI JSON | MCP gives agents structured tool access; CLI remains canonical. |
| Prepare handoff | MCP or CLI JSON | `worker-handoff/v1` should be identical in meaning. |
| Inspect checks or finish blockers | MCP or CLI JSON | `finish-readiness/v1` and check blockers are evidence, not mutation. |
| Preview a mutation | CLI `--dry-run --json` | Dry-run is approval evidence for a later apply. |
| Apply a mutation | CLI `--apply` | MCP v1 intentionally does not expose apply. |
| Unsupported MCP action | CLI fallback | Use the equivalent Gira command, not raw `gh` or ad hoc shell. |

## PM harness parity

Start both human and AI PM operation with the same canonical bootstrap:

| PM step | CLI | Focused MCP adapter |
| --- | --- | --- |
| Bootstrap | `gira pm bootstrap --repo OWNER/REPO --ticket N --role human|ai --json` | `gira_pm_bootstrap` (AI role) |
| Compile Goal | `gira pm compile --repo OWNER/REPO --goal N --json` | `gira_pm_compile` |
| Observe | `gira pm observe --repo OWNER/REPO --ticket N --json` | `gira_pm_observe` |
| Preview replan | `gira pm replan --repo OWNER/REPO --ticket N --dry-run --json` | `gira_pm_replan_plan` |
| Prepare validation | `gira pm qa --repo OWNER/REPO --ticket N --json` | `gira_pm_validate` |
| Report | `gira goal report N --repo OWNER/REPO --view ai --json` | `gira_pm_report` |

The adapters contain no PM state machine. Their argv points back to these CLI
contracts and they never add `--apply`. Mutations use `gira_cli` or direct CLI
only after a visible dry-run, matching fingerprint, and authority evidence.
`gira mcp doctor --json` reports the focused-tool parity set, policy/protocol
versions, built-in conformance evidence, and unsafe-mutation count.

## Normal Ticket Lifecycle

A human or agent can use either CLI or MCP for read steps, then use CLI for mutation.

| Step | CLI | MCP |
| --- | --- | --- |
| Inspect ticket | `gira ticket view 42 --repo OWNER/REPO --json` | `gira_ticket_view` with `repo=OWNER/REPO`, `ticket=42` |
| Inspect status | `gira ticket status 42 --repo OWNER/REPO --json` | `gira_ticket_status` with `repo=OWNER/REPO`, `ticket=42` |
| Inspect checks | `gira ticket checks 42 --repo OWNER/REPO --json` | `gira_ticket_checks` with `repo=OWNER/REPO`, `ticket=42` |
| Plan finish | `gira ticket finish 42 --repo OWNER/REPO --dry-run --json` | `gira_ticket_finish_plan` with `repo=OWNER/REPO`, `ticket=42` |
| Apply finish | `gira ticket finish 42 --repo OWNER/REPO --apply` | Not available in MCP v1. Use CLI. |
| Read back | `gira ticket status 42 --repo OWNER/REPO --json` | `gira_ticket_status` with `repo=OWNER/REPO`, `ticket=42` |

The agent should preserve blockers, warnings, `next_action`, `next_step`, and receipt evidence from the read surfaces. It should not execute `next_step` as an arbitrary shell string.

## Queue-Driven Agent Handoff

Queue flows follow the same rule: MCP may select and hand off; CLI applies lifecycle mutations.

| Step | CLI | MCP |
| --- | --- | --- |
| Select next work | `gira queue next --repo OWNER/REPO --json` | `gira_queue_next` with `repo=OWNER/REPO` |
| Build handoff | `gira queue handoff --repo OWNER/REPO --json` | `gira_queue_handoff` with `repo=OWNER/REPO` |
| Preview branch start | `gira queue take --repo OWNER/REPO --dry-run --json` or `gira ticket start 42 --repo OWNER/REPO --dry-run --json` | Not available in MCP v1. Use CLI. |
| Apply branch start | `gira queue take --repo OWNER/REPO --apply` or `gira ticket start 42 --repo OWNER/REPO --apply` | Not available in MCP v1. Use CLI. |
| Read back | `gira ticket status 42 --repo OWNER/REPO --json` | `gira_ticket_status` with `repo=OWNER/REPO`, `ticket=42` |

`gira_queue_next` and `gira_queue_handoff` do not claim work, start a branch, launch a worker, or mutate GitHub. They provide selection and handoff evidence.

## Fallback Rules

Use these rules when an MCP tool is missing or a workflow step is ambiguous:

- If an MCP tool is not available, fall back to the equivalent `gira ... --json` CLI command.
- If the action mutates GitHub, local git state, local Gira state, labels, milestones, PRs, comments, or receipts, use CLI `--dry-run` first.
- If approval is required, store or show the dry-run output before running CLI `--apply`.
- If MCP and CLI appear to disagree, trust the CLI JSON contract backed by GitHub evidence and investigate the MCP wrapper.
- If the agent cannot identify a safe next command, stop and ask for human review instead of inventing a workflow step.
- Do not use raw `gh` or shell as a shortcut when Gira exposes the workflow operation.

## Unsafe Patterns

Avoid these patterns:

- Treating MCP output as a shell script.
- Executing `next_step` without checking that it is an allowed Gira command.
- Adding MCP-only readiness states or queue states.
- Using MCP to bypass CLI dry-run/apply approval.
- Letting hosted, adapter, or local logs become the workflow source of truth.

## References

- [ADR 0003: CLI and MCP parity boundary](adr/0003-cli-and-mcp-parity-boundary.md)
- [Gira read-only MCP surface](mcp-read-only-surface.md)
- [Agent-Kernel Adapter Contract](agent-kernel-adapter-contract.md)
- [Worker Handoff Contract](worker-handoff-contract.md)
