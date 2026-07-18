# MCP Agent Workflow Best Practices

Gira MCP should make the agent feel conversational, but it must not make the
lifecycle implicit. The agent may continuously read and summarize project state
through MCP while every mutation remains an explicit Gira CLI `--dry-run` and
`--apply` transition with GitHub evidence.

## Product boundary

MCP is the agent transport for the same Gira CLI lifecycle. Direct CLI usage and
MCP-assisted usage should have the same semantics. GitHub remains the execution
record.

This keeps local CLI work, MCP-assisted agent work, and future hosted MCP work
inside one product model instead of creating separate workflows.

## Local operating loop

1. Select or adopt work.
   Use MCP queue and ticket reads when available. Use `gira_cli` to run
   `gira adopt`, `gira queue`, or `gira ticket new` with `--dry-run` before
   creating or changing workflow state.

2. Inspect context.
   Use MCP ticket view, status, checks, finish-plan, handoff, and queue tools to
   keep the conversation current. The agent should summarize blockers and stale
   assumptions before proposing a mutation.

   For Goal-level PM work, call `gira_pm_bootstrap` first. Expand only the
   referenced compile, context, observation, or report contract needed for the
   next action; do not rely on hidden conversation memory.

3. Plan the mutation.
   Run the matching Gira CLI command with `--dry-run`, either directly in a
   terminal or through `gira_cli` over MCP. The agent should show the dry-run
   result in the conversation and identify the issue, branch, PR, check, or
   release artifact that will change.

4. Apply explicitly.
   After user agreement or an established operating policy, run the approved CLI
   command with `--apply`, either directly in a terminal or through `gira_cli`.
   MCP must not create a second mutation model; it should execute the same Gira
   CLI lifecycle.

5. Record evidence.
   Report GitHub issue, PR, check, workflow, release, or comment links. MCP
   context is advisory unless backed by a Gira command result or GitHub state.

6. Review and finish.
   Use MCP reads to discuss readiness. Use `gira ticket finish --dry-run` before
   `gira ticket finish --apply`. Do not finish work with failing checks, missing
   PR evidence, or unresolved human decisions.

## Built-in guide tool

Agents should call `gira_workflow_guide` when they need the current operating
rules for mixing MCP and CLI. The tool returns `gira-mcp-workflow-guide/v1` JSON
with the local flow, authentication expectations, evidence rules, safety rules,
and hosted-service boundary notes.

The guide tool is intentionally read-only. Execution happens through `gira_cli`,
which invokes the installed `gira` binary with explicit argv and returns exit
code, stdout, stderr, and dry-run/apply metadata.

## Authentication

Use `gira mcp doctor --repo OWNER/REPO --json` before relying on MCP reads in a
new local environment.

Credential selection order for local MCP is:

1. `GIRA_MCP_GITHUB_TOKEN`
2. `GITHUB_TOKEN`
3. `GH_TOKEN`
4. local `gh` authentication

Do not persist GitHub tokens in Gira repo config, workflow documents, or issue
comments.

## Evidence model

GitHub issues are task packets. Pull requests are change units. Checks, reviews,
merge state, and issue comments are completion evidence.

The agent can speak conversationally, but state changes must point to concrete
GitHub or Gira evidence. A good response after mutation includes what changed,
where it changed, and which evidence link proves it.

## Anti-patterns

- Exposing raw shell, raw `gh`, or a mutation path that does not go through the
  Gira CLI lifecycle.
- Treating MCP context as a second source of truth.
- Creating MCP-only lifecycle terms that do not map to Gira CLI states.
- Storing tokens in repo files, Gira config, or generated workflow docs.
- Letting hosted-service assumptions leak into local CLI/MCP behavior.
- Capturing private transcripts, secrets, worker rankings, or token-spend
  productivity scores as PM conformance evidence.

## Hosted MCP boundary

A future hosted MCP service should preserve the same conversational flow and
evidence model. The difference is authentication and tenancy, not lifecycle
semantics.

Hosted authentication should use per-user or per-installation authorization
rather than shared environment tokens. Hosted mutation, if introduced, must have
an explicit dry-run/apply equivalent and must write the same GitHub evidence as
the CLI path.
