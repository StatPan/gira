# Agent Workflow Completion Benchmark

## Status

Research note for #725. Accepted implementation successor: #734.

## Objective

The Gira agent workflow completion benchmark should evaluate whether Gira and its adapters make correct workflow decisions from GitHub-like evidence.

It should not evaluate whether a model can write a code patch. Existing code-generation benchmarks already cover that surface. Gira's distinctive value is deciding when work is ready, blocked, reviewable, finishable, or unsafe to continue autonomously.

## Recommendation

Accept a small local fixture suite as the next step.

Reject these directions for now:

- hosted benchmark service;
- model leaderboard;
- SWE-bench-style patch generation benchmark;
- broad analytics dashboard;
- GitHub-networked benchmark runner.

The first implementation should be deterministic, local, and contract-focused. It should protect Gira's readiness, handoff, queue, review, and finish decisions as the product evolves.

## What The Benchmark Measures

The benchmark should measure workflow decision quality.

| Capability | Expected decision |
| --- | --- |
| Issue readiness | Is the ticket specific enough to start, or does it need refinement? |
| PR readiness | Is the linked PR reviewable, blocked, draft, approved, or missing evidence? |
| CI blocker detection | Are required checks pending, failing, missing, or passing? |
| Review blocker detection | Does review state require wait, revise, human review, or finish? |
| Next command selection | Does Gira choose the next safe Gira command instead of raw shell or broad GitHub actions? |
| Safe refusal | Does Gira stop when branch policy, evidence, human decision, or queue state is unsafe? |
| Handoff quality | Does `worker-handoff/v1` preserve enough context for another actor? |
| Finish receipt correctness | Does finish planning require the right merge, close, and receipt evidence? |

## Non-Goals

This benchmark is not:

- a coding benchmark;
- a patch correctness benchmark;
- a model leaderboard;
- a productivity ranking;
- a hosted evaluation product;
- a replacement for unit, integration, or end-to-end tests;
- a substitute for human product judgment.

A scenario should be considered successful when the workflow decision matches the expected policy, not when an agent writes more code.

## Smallest Fixture Format

The fixture format should model just enough GitHub-like state to exercise Gira workflow contracts.

A minimal fixture can include:

```yaml
schema_version: agent-workflow-benchmark/v1
name: ready-ticket-start
repo: StatPan/example
issue:
  number: 101
  title: Add finish readiness docs
  state: open
  labels: [type:task, priority:p2, status:ready]
  body_sections:
    goal: present
    scope: present
    acceptance_criteria: present
    non_goals: present
    evidence_plan: present
branch:
  expected: issue-101-add-finish-readiness-docs
  current: ""
  trusted: false
pull_request: null
checks: []
reviews: []
receipts: []
expected:
  readiness: ready
  blockers: []
  next_action: start_work
  next_command: gira ticket start 101 --repo StatPan/example --dry-run
```

The fixture should avoid full GitHub API payloads. It should represent the normalized evidence Gira needs for decisions, then assert the expected readiness, blockers, next action, and next safe command.

## Candidate Scenarios

At least these scenarios should be covered by #734:

| Scenario | Evidence | Expected result |
| --- | --- | --- |
| Ready ticket start | Complete issue body, ready label, no linked PR. | `ready`, no blockers, next command is `ticket start --dry-run`. |
| Needs refinement | Missing acceptance criteria or evidence plan. | Blocked/refine, blocker identifies the missing section. |
| Review required | Linked non-draft PR with checks passing but missing review. | Not finish-ready, next action is review or wait for review. |
| Failed checks | Linked PR has required failing check. | Failed-check blocker, next action is revise/fix. |
| Finish ready | Linked PR is mergeable, checks pass, review policy satisfied, closing reference exists. | Finish dry-run plans merge/receipt. |
| Human decision | Labels or receipts indicate human-review handoff. | Stop autonomous work and preserve handoff context. |
| Branch policy mismatch | PR base or branch binding conflicts with policy. | Safe refusal with branch policy blocker. |
| Queue not handoff-safe | Item is in review, finish, failed, blocked, or human lane. | Queue handoff stops with a stable stop reason. |

The first implementation only needs five scenarios, but the fixture format should support the rest.

## Evaluation Outputs

Each scenario should assert stable fields that Gira already treats as contracts:

- `schema_version`;
- `readiness`;
- `blockers[]`;
- `warnings[]` when policy allows non-blocking concerns;
- `next_action`;
- `next_step` or `next_command`;
- selected queue lane when relevant;
- finish planned actions when relevant;
- handoff stop reasons when relevant.

The benchmark should not assert prose formatting unless the prose is itself a documented receipt contract.

## Relationship To Existing Contracts

The benchmark should exercise existing contracts instead of inventing a parallel scoring model:

- `ticket-readiness/v1`;
- `pr-readiness/v1`;
- `finish-readiness/v1`;
- `finish-receipt/v1`;
- `workspace-queues/v1`;
- `queue-*` contracts;
- `worker-handoff/v1`;
- `gira-approval-plan/v1` for dry-run/apply boundaries.

If a scenario cannot be expressed through those contracts, the first response should be to harden the relevant CLI JSON contract, not to add benchmark-only semantics.

## Implementation Successor

#734 should implement the local fixture suite.

It should not create a public benchmark brand, hosted service, or leaderboard. Its purpose is regression protection for Gira's workflow-control semantics.

## Fixture Suite

The accepted local fixture suite lives under `internal/gira/testdata/agent_workflow_benchmark/` and is exercised by `TestAgentWorkflowCompletionBenchmarkFixtures`.

Each fixture declares normalized GitHub-like evidence plus expected workflow decisions:

- `ticket_readiness`;
- queue classification;
- reason code;
- next action;
- next safe command;
- blockers.

The suite is intentionally local and deterministic. It does not call GitHub, execute model providers, or score individual humans or agents.
