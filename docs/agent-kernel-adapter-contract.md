# Agent-Kernel Adapter Contract

This document defines the Gira-side contract for using Gira as an
`agent-kernel` operating app. It is a contract for adapters and durable run
systems, not a new Gira runtime.

Gira remains a GitHub-native workflow control layer:

- GitHub is the source of truth for issues, PRs, branches, checks, reviews,
  labels, milestones, comments, and merge state.
- Gira reads GitHub evidence and emits workflow contracts, readiness reports,
  next actions, dry-run plans, review packets, and finish receipts.
- `agent-kernel` may provide durable runs, tool calls, approval gates, policy,
  ledgers, retries, and projections.
- Workers still execute code outside Gira.

## Non-Goals

Gira must not become:

- an agent framework.
- a hosted worker runtime.
- a prompt/model router.
- a source code sandbox.
- a background GitHub or Jira sync service.
- a replacement database for GitHub issues and PRs.
- an autonomous merge or finish system by default.

The adapter may call Gira commands, but GitHub-visible evidence remains the
durable contract.

## Capability Classes

The adapter should classify every Gira command it exposes into one of four
classes.

| Class | Meaning | Adapter behavior |
| --- | --- | --- |
| `read` | Reads GitHub, local config, or cached state without mutation. | May run without approval when repo access policy allows it. Store output as evidence. |
| `dry_run_mutation` | Computes a mutation plan without applying it. | May run without apply approval. Store the command, arguments, output, and state snapshot as approval evidence. |
| `apply_mutation` | Mutates GitHub, local config, local git state, cache, or Gira ledger state. | Requires a matching dry-run evidence packet and explicit approval unless the repo policy grants pre-approval for that exact command class. |
| `unsupported` | Too low-level, ambiguous, destructive, interactive, or outside the adapter scope. | Do not expose directly. Route to a human or a narrower Gira command. |

Compatibility aliases such as `work`, `dev`, `docs`, `update`, and `start`
should normalize to their canonical command family before policy evaluation.

## Generated Capability Source

The machine-readable adapter map is generated from Gira's command metadata
registry:

```bash
gira guide capabilities --json
```

The generated docs-site view is
[`docs-site/command-capabilities.md`](../docs-site/command-capabilities.md).
Adapters should treat `schema_version: gira-command-capabilities/v1`,
`canonical`, `capability`, `mutation_boundary`, `json_support`, `aliases`, and
`docs` as the policy input. The table below remains a narrative overview for
the first dogfood flow; do not hand-maintain adapter policy from the prose when
the generated capability map is available.

## Initial Command Capability Map

This map covers the first adapter flow. It is intentionally conservative.

| Command family | Capability | Notes |
| --- | --- | --- |
| `gira version`, `gira upgrade`, `gira config global`, `gira config repo`, `gira config doctor` | `read` | Safe environment and configuration inspection. `upgrade` is advisory and must not run package managers. |
| `gira status`, `gira workspace status`, `gira ticket list`, `gira ticket view`, `gira ticket status`, `gira ticket checks`, `gira ticket review`, `gira ticket handoff`, `gira ticket prompt` | `read` | Primary state, work-order, review, and handoff surfaces. Prefer JSON where available. Prompt output is evidence, not an instruction to bypass policy. |
| `gira goal status`, `gira goal next`, `gira goal plan --dry-run`, `gira goal finish --dry-run` | `read` or `dry_run_mutation` | `goal plan --dry-run` and `goal finish --dry-run` prepare child-ticket plans or receipts but do not apply. `goal next` can select work or stop. |
| `gira audit readiness`, `gira audit drift`, `gira audit workflow`, `gira audit verify`, `gira stats repo` | `read` | Use for workflow convergence and integrity evidence. |
| `gira jira doctor`, `gira jira transition --dry-run`, `gira jira export` | `read`, `dry_run_mutation`, or `apply_mutation` | Provider diagnostics and migration export. `jira transition --dry-run` emits `jira-transition-plan/v1` as read-only planning evidence, not approval to mutate Jira. `jira export` writes local export artifacts and therefore needs an approved or sandboxed output boundary. |
| `gira ticket new --dry-run`, `gira ticket start --dry-run`, `gira ticket pr --dry-run`, `gira ticket note --dry-run`, `gira ticket finish --dry-run`, `gira ticket supersede --dry-run` | `dry_run_mutation` | These are approval evidence surfaces for issue, branch, PR, comment, merge, close, and supersede mutations. |
| `gira adopt repo --dry-run`, `gira adopt issues --dry-run`, `gira setup global --dry-run`, `gira workspace repos sync --dry-run`, `gira repo register --dry-run`, `gira repo migrate --dry-run` | `dry_run_mutation` | Local config, repo metadata, or issue adoption plans. |
| `gira milestone new --dry-run`, `gira milestone assign --dry-run`, `gira milestone plan --dry-run`, `gira sprint plan`, `gira sprint rollover --dry-run`, `gira release readiness` | `dry_run_mutation` or `read` | Release readiness is read-only. Sprint and milestone plans need approval before apply. |
| `gira goal plan --apply`, `gira ticket new --apply`, `gira ticket start --apply`, `gira ticket pr --apply`, `gira ticket note --apply`, `gira ticket finish --apply`, `gira ticket supersede --apply` | `apply_mutation` | Requires matching dry-run evidence and approval. `goal plan --apply` creates linked child tickets from a reviewed plan. `ticket finish --apply` may merge PRs, normalize labels, post receipts, and close tickets. |
| `gira goal finish --terminal human_review --apply` | `apply_mutation` | Current apply-safe goal path posts an idempotent human-review handoff receipt. It does not close the goal. |
| `gira setup global --apply`, `gira workspace repos sync --apply`, `gira repo register --apply`, `gira repo migrate --apply`, `gira adopt repo --apply`, `gira adopt issues --apply` | `apply_mutation` | Mutates local registry, repo files, or GitHub issue metadata depending on command. |
| `gira milestone new --apply`, `gira milestone assign --apply`, `gira milestone plan --apply`, `gira sprint start --apply`, `gira cache prune --apply` | `apply_mutation` | Cache pruning mutates local cache only but still needs approval. |
| `gira ops *`, `gira dev *`, raw `gh`, shell commands | `unsupported` by default | Expose only through a narrowly-scoped policy exception with an explicit command allowlist. |

## Required Adapter Evidence

For every Gira command call, `agent-kernel` should store a tool-call evidence
record with:

| Field | Required | Meaning |
| --- | --- | --- |
| `schema_version` | Yes | Adapter evidence schema, for example `gira-tool-call/v1`. |
| `run_id` | Yes | Durable agent-kernel run ID. |
| `tool_call_id` | Yes | Durable tool call ID. |
| `repo` | Yes when repo-scoped | `OWNER/REPO`. |
| `cwd` | Yes when local checkout matters | Local path used for command execution. |
| `command` | Yes | Canonical Gira command path and arguments. |
| `capability` | Yes | One of `read`, `dry_run_mutation`, `apply_mutation`, `unsupported`. |
| `started_at`, `finished_at` | Yes | UTC timestamps. |
| `exit_code` | Yes | Process exit code. |
| `stdout_digest`, `stderr_digest` | Yes | Hashes of captured output for ledger integrity. |
| `stdout_json` | When JSON is available | Parsed JSON object, not a string copy. |
| `blockers` | When present | Gira blockers from JSON or parsed output. |
| `next_action`, `next_step` | When present | Gira next-action contract. |
| `approval_id` | For apply | Approval that authorized the apply. |
| `dry_run_tool_call_id` | For apply | Matching dry-run evidence. |
| `post_state_tool_call_id` | For apply | Follow-up read command that verified the mutation. |

The adapter should prefer `--json` outputs when Gira supports them. If a command
does not support JSON yet, the adapter may store text output but must mark
`stdout_json` as absent and lower automation confidence.

## Shared Approval Plan

Mutating dry-run JSON reports should include a common `approval` object.
Implemented producers include `gira ticket new --dry-run --json` and
`gira ticket start --dry-run --json` and
`gira ticket pr --dry-run --json` and
`gira ticket note --dry-run --json` and
`gira ticket finish --dry-run --json` and
`gira ticket supersede --dry-run --json` and
`gira repo register --dry-run --json` and
`gira repo migrate --dry-run --json` and
`gira setup global --dry-run --json` and
`gira workspace repos sync --dry-run --json` and
`gira adopt repo --dry-run --json`.
The surrounding report emits a versioned `schema_version`, and
`approval.output_schema` references that same version.

`approval.schema_version` is `gira-approval-plan/v1`.

| Field | Meaning |
| --- | --- |
| `schema_version` | Approval evidence schema version. |
| `capability` | Capability class for the eventual apply command, usually `apply_mutation`. |
| `canonical_command` | Canonical Gira command family, for example `gira ticket start`. |
| `dry_run_argv` | Machine-readable dry-run argv whose output produced this evidence. Execute directly without a shell. |
| `apply_argv` | Machine-readable apply argv this evidence can approve. Execute directly without a shell. |
| `dry_run_command` | Display/backward-compatibility dry-run command string. Do not execute through a shell. |
| `apply_command` | Display/backward-compatibility apply command string. Do not execute through a shell. |
| `repo` | Target `OWNER/REPO` when repo-scoped. |
| `issue` | Target issue or ticket number when present. |
| `output_schema` | Versioned shape of the surrounding dry-run report. |
| `planned_actions` | Stable action list the apply command is expected to perform. |
| `blockers` | Stable blockers array. Empty means the dry-run did not block approval. |
| `warnings` | Stable warnings array. Empty means no warnings were emitted. |
| `post_apply_verification` | Read command the adapter should run after apply. |

Adapters must compare the approved `apply_argv`, `repo`, `issue`, and planned
action evidence before executing apply. Execute the approved argv directly with
no shell interpolation. The command string fields exist for human display and
older consumers only; they must not be passed to `sh -c`, `bash -c`, CI shell
steps, or equivalent command-string execution. A matching approval does not
authorize a different command, repo, issue, or additional flags.

## Stable JSON Fields Needed By The First Flow

The first safe flow can run with existing Gira commands if these fields are
present when relevant:

| Field | Use |
| --- | --- |
| `schema_version` | Route parser and preserve compatibility. |
| `repo` | Bind output to a repository. |
| `issue` or `ticket` | Bind output to the source GitHub issue. |
| `status` | Human workflow state. |
| `labels` | Lane, type, priority, status, and approval policy. |
| `branch.expected`, `branch.current`, `branch.trusted` | Validate issue-to-branch binding. |
| `branch_policy.recorded_base`, `branch_policy.actual_pr_base`, `branch_policy.base_mismatch` | Enforce branch policy. |
| `pull_request.number`, `pull_request.state`, `pull_request.is_draft` | Decide PR review and finish readiness. |
| `checks_status`, `checks[]` | Wait, revise, or finish decision. |
| `review_status` or `review.decision` | Approval policy evidence. |
| `ticket_readiness.readiness`, `ticket_readiness.findings[]` | Start/handoff gate. |
| `pr_readiness.readiness`, `pr_readiness.findings[]` | Review/finish gate. |
| `readiness.ready`, `readiness.blockers[]`, `readiness.warnings[]` | Finish readiness gate. |
| `actions[]` | Dry-run/apply plan and actual mutation evidence. |
| `blockers[]`, `warnings[]` | Stop conditions. |
| `next_action`, `next_step` | Next safe command. |
| `receipt.schema_version`, `receipt.rendered_body` | Finish or handoff receipt preview/evidence. |
| `handoff_receipt_present` | Goal-level human-review convergence. |

## First Safe Dogfood Flow

The smallest adapter dogfood flow should use existing Gira commands:

1. Start an `agent-kernel` run with a repo and objective.
2. Run `gira config doctor --repo OWNER/REPO --json`.
3. Run `gira status --repo OWNER/REPO --json` or
   `gira workspace status --repo OWNER/REPO --json`.
4. If the objective is goal-scoped, run
   `gira goal status GOAL --repo OWNER/REPO --json` and
   `gira goal next GOAL --repo OWNER/REPO --json`.
5. If a selected issue is returned, run
   `gira ticket status TICKET --repo OWNER/REPO --json`.
6. If readiness says `ready`, run
   `gira ticket handoff TICKET implementer --repo OWNER/REPO --json`.
7. Stop and request approval before any apply mutation.
8. For an approved start, run
   `gira ticket start TICKET --repo OWNER/REPO --dry-run --json`.
9. Store the dry-run output as approval evidence.
10. After approval, run the matching `approval.apply_argv` plus `--json` using
    direct argv execution.
11. Verify with `gira ticket status TICKET --repo OWNER/REPO --json`.
12. Stop at handoff. The first adapter flow should not implement code, open
    PRs, merge, close, or publish without a later approved flow.

Stop immediately when:

- Gira emits blockers.
- `next_action` is `ask_human`, `blocked`, `human_review`, `revise_pr`, or
  unknown to the adapter.
- required JSON fields are missing.
- dry-run output does not match the intended apply command.
- approval is missing, expired, or scoped to different arguments.
- local checkout, branch, or repo binding is ambiguous.

## Approval Boundaries

The adapter must treat every `--apply` as a separate approval boundary unless a
repository policy grants a narrower pre-approval.

The approval packet should include:

- objective.
- repo and ticket.
- canonical apply command.
- matching dry-run command.
- dry-run output digest.
- Gira actions planned by the dry-run.
- blockers and warnings.
- expected post-state verification command.
- expiration time.
- human or policy approver.

An approval for one command must not authorize a different command, a different
repo, a different issue, a different branch, or additional flags.

## Remaining Hardening

The 2.0 control-plane path can run with stable command capability metadata,
schema-versioned goal/workspace/readiness surfaces, and shared approval evidence
for the main Gira apply boundaries. These gaps remain before broad adapter use:

| Gap | Impact | Follow-up |
| --- | --- | --- |
| Not every mutating dry-run emits the shared approval evidence envelope yet. | `agent-kernel` can use `gira-approval-plan/v1` for ticket lifecycle, core config/registry, workspace repo-sync, repo/issue adoption, milestone, cache prune, and sprint dry-runs. Jira transition plans are schema-versioned read-only evidence and intentionally do not emit approval evidence because they do not authorize a matching Gira apply boundary. | Extend the shared `approval` object only where the dry-run authorizes a matching Gira apply boundary. |
| Some command families remain text-first or partially JSON-covered. | Automation confidence drops and adapters need fragile parsing. | Add JSON contracts or mark those commands unsupported for adapters. |
| No explicit post-apply verification link in every apply report. | Adapters need command-specific knowledge to know which read command proves completion. | Add `post_apply_verification` fields to apply reports. |

## Follow-Up Issue Candidates

A later issue may add schema coverage to remaining read-only/reporting JSON
surfaces and post-apply verification links where apply reports still lack them.

Do not create issues for hosted UI, autonomous code execution, model routing, or
background sync as part of this contract.

## Relationship To Existing Contracts

- [Worker Boundary And Provenance](worker-boundary-provenance.md) defines what
  Gira owns versus external workers.
- [Goal Operating Model](goal-operating-model.md) defines goal-level safe next
  work and finish handoff.
- [Agent Delegation Lanes](agent-delegation-lanes.md) defines lane and approval
  policy.
- [Closure Funnel Stats](closure-funnel-stats.md) defines workflow convergence
  metrics.
