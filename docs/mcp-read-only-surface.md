# Gira read-only MCP surface

## Status

Research design for #640. Implementation successor: #730.

## Decision

Gira's first MCP surface should be a read-only adapter over stable `gira ... --json` CLI contracts.

The MCP server must not become a second product implementation. It should translate MCP tool calls into direct `gira` argv executions, return the CLI JSON payload with small tool metadata, and refuse any request that could mutate GitHub or the local repository.

This keeps the product contract aligned with the CLI while giving coding-agent harnesses a narrower and safer workflow-control interface.

## Product boundary

The v1 MCP surface is about finishing work safely, not broad project management.

It prioritizes:

- ticket context needed before acting;
- finish readiness and blocker evidence;
- handoff state for harness-independent agent transitions;
- queue visibility for choosing the next bounded work packet.

It does not expose generic GitHub, shell, project-management, or hosted-worker capabilities.

## Tool inventory

| MCP tool | CLI contract | Behavior |
| --- | --- | --- |
| `gira_ticket_view` | `gira ticket view <ticket> --repo <owner/repo> --json` | Return issue packet context, labels, readiness, body, and current workflow state. |
| `gira_ticket_status` | `gira ticket status <ticket> --repo <owner/repo> --json` | Return ticket lifecycle state, branch/PR linkage, and recommended next action. |
| `gira_ticket_checks` | `gira ticket checks <ticket> --repo <owner/repo> --json` | Return PR/check status and finish blockers without changing state. |
| `gira_ticket_finish_plan` | `gira ticket finish <ticket> --repo <owner/repo> --dry-run --json` | Return the finish plan and blockers; `--apply` is never available through MCP. |
| `gira_ticket_handoff` | `gira ticket handoff <ticket> --repo <owner/repo> --json` | Return handoff context for another agent or human to continue safely. |
| `gira_queue_list` | `gira queue list --repo <owner/repo> --json` | Return ready/in-progress queue candidates and their readiness signals. |
| `gira_queue_next` | `gira queue next --repo <owner/repo> --json` | Return the next recommended work packet without claiming or starting it. |
| `gira_queue_handoff` | `gira queue handoff --repo <owner/repo> --json` | Return queue-level handoff context for agent transition. |

A tool may be omitted from the first implementation if the corresponding CLI command does not yet provide stable JSON. The omission should be explicit in docs and tests.

## Input contract

All tools should validate inputs before invoking `gira`.

Common inputs:

| Field | Required | Notes |
| --- | --- | --- |
| `repo` | Yes | GitHub repository in `owner/name` form. MCP v1 should not guess across a workspace. |
| `ticket` | Ticket tools only | Positive GitHub issue number. |
| `limit` | Queue tools only, optional | Positive bounded integer when supported by the CLI. |

Input validation failures should return MCP tool errors without running the CLI.

## Output contract

Successful responses should preserve the CLI JSON as the authoritative payload.

The MCP wrapper may add a small envelope:

```json
{
  "tool": "gira_ticket_status",
  "command": ["gira", "ticket", "status", "640", "--repo", "StatPan/gira", "--json"],
  "read_only": true,
  "payload": {}
}
```

The wrapper should not reinterpret lifecycle semantics unless the CLI contract already exposes that field. For example, `next_action`, `blockers`, `checks`, `branch`, and `pull_request` should come from CLI JSON, not MCP-local inference.

## Error behavior

The MCP server should return deterministic errors for:

- invalid `repo`, `ticket`, or `limit` input;
- unsupported tool names;
- unavailable `gira` binary;
- CLI command timeout;
- non-zero CLI exit;
- stdout that is not valid JSON when JSON was required;
- stderr diagnostics from the CLI.

For CLI failures, include:

```json
{
  "tool": "gira_ticket_checks",
  "command": ["gira", "ticket", "checks", "640", "--repo", "StatPan/gira", "--json"],
  "exit_code": 1,
  "stderr": "...",
  "stdout": "..."
}
```

The MCP wrapper must not retry by switching to mutating commands or raw `gh` commands. Recovery guidance should be returned as data for the caller to decide.

## Explicit exclusions for v1

The MCP surface must refuse or not define tools for:

- any command with `--apply`;
- `gira ticket start`;
- `gira ticket pr`;
- `gira ticket finish --apply`;
- queue claim/take/start operations;
- label, milestone, template, project, or repository mutation;
- raw `gh` execution;
- arbitrary shell execution;
- direct writes through internal Go packages;
- hosted service behavior.

`gira_ticket_finish_plan` is the only finish-related tool in v1, and it is dry-run-only.

## CLI versus internal package boundary

MCP v1 should call the installed `gira` binary instead of importing internal Go packages.

Rationale:

- The CLI JSON contract is already the user-facing workflow-control contract.
- A CLI wrapper keeps MCP behavior aligned with local agent usage.
- Internal packages can continue to change without becoming an external integration surface.
- The wrapper can prove read-only behavior by command allow-listing and argument construction.

Direct internal package use can be reconsidered only after the CLI JSON contracts are stable and a concrete performance or packaging problem exists.

## Security and execution constraints

The implementation should:

- build argv arrays directly and avoid shell interpolation;
- use an allow-list mapping from MCP tool name to exact CLI command template;
- append `--json` itself rather than accepting arbitrary caller flags;
- reject user-provided flags or command fragments;
- set a conservative command timeout;
- document the required `gira` binary discovery behavior;
- rely on the caller's existing GitHub authentication, matching CLI behavior.

## Follow-up

#730 should implement the first local read-only MCP server against this design. If implementation reveals a missing CLI JSON contract, the correct follow-up is to harden that CLI command first, not to bypass the CLI through MCP-specific logic.

## Local server command

The first implementation exposes the surface with:

```bash
gira mcp serve
```

The server uses stdio JSON-RPC and only publishes the read-only tools listed in this document. Harnesses should treat the MCP response payload as an envelope around the underlying Gira CLI JSON response and should fall back to the CLI when they need a command outside the read-only MCP allow-list.

## Mixed CLI/MCP workflow

See [CLI And MCP Workflow Parity](cli-mcp-workflow-parity.md) for the recommended agent flow that uses MCP for read/context steps and CLI `--dry-run`/`--apply` for mutations.
