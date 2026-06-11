# ADR 0002: Finish readiness is Gira's core product surface

## Status

Accepted

## Date

2026-06-11

## Context

Gira operates GitHub-native software work. Issues define executable scope,
branches show work start, pull requests carry change evidence, reviews and
checks gate acceptance, and merged PRs plus closed issues prove completion.

AI coding agents increasingly help create branches, commits, and pull requests.
That does not make the work complete. Completion still depends on repository
evidence converging:

- the ticket is the intended work packet;
- the branch belongs to the ticket and follows policy;
- the pull request is linked, reviewable, and mergeable;
- checks and review blockers are clear;
- the PR contains a closing reference;
- merge and issue closure can be audited after the fact.

Gira already has several surfaces around this loop: ticket readiness, PR
readiness, checks, review packets, worker handoff, queues, finish readiness,
and finish receipts. Without a product boundary, those surfaces can look like a
broad project-management platform, hosted dashboard, GitHub Projects
replacement, or coding-agent runtime.

## Decision

Gira's core product surface is finish readiness for GitHub issue and pull
request work.

The core product moment is:

```bash
gira ticket finish --dry-run
```

That dry-run must answer whether the ticket can safely finish from GitHub
evidence. It should make blockers explicit and produce a matching apply command
only when finishing is safe:

```bash
gira ticket finish --apply
```

The core contracts are:

- `finish-readiness/v1`: computed readiness from ticket, branch, PR, review,
  checks, closing-reference, mergeability, labels, and policy evidence.
- `finish-receipt/v1`: durable completion evidence written after an accepted
  finish.
- `gira-approval-plan/v1`: the dry-run/apply approval boundary for finish and
  other mutations.

Other Gira surfaces are supporting surfaces. They are justified when they make
finish readiness safer, clearer, or easier to operate:

- ticket and PR status explain current blockers;
- checks and wait commands converge CI/review evidence;
- review and self-review packets make PRs easier to evaluate;
- worker handoff and queues select safe next work;
- pulse and dashboard exports show workflow movement;
- MCP and adapter contracts expose existing JSON surfaces to agents;
- GitHub Projects remain visibility surfaces over issue evidence.

## Boundaries

Gira must not become:

- a coding agent;
- a generic Jira clone;
- a hidden planning database;
- a hosted dashboard-first product;
- a GitHub Projects replacement;
- a model router, sandbox, or agent runtime;
- an autonomous merge system without explicit dry-run/apply policy.

New features should pass this test:

```text
Does this improve selection, handoff, review, evidence, or safe finish of
GitHub issue/PR work?
```

If the answer is no, the feature should be deferred, kept as research, or
implemented outside the core product path.

## Consequences

- README, docs, and command contracts should emphasize safe finish and
  repository evidence over broad project-management language.
- `finish-readiness/v1` and `finish-receipt/v1` should be treated as public
  control-plane contracts rather than incidental command output.
- MCP work should start as read-only exposure of existing CLI JSON surfaces,
  especially ticket status, checks, finish plans, queues, and handoff.
- Worker handoff and queue work should remain harness-independent and serve the
  issue-to-finish loop.
- Workflow pulse and CI reliability signals should stay operational and
  evidence-focused, not become productivity rankings or agent leaderboards.
- GitHub Projects work should preserve the boundary that Projects are
  visibility surfaces, not execution sources of truth.

## Related work

- #720: add this ADR.
- #721: update README positioning around safe finish and repository evidence.
- #722: harden `finish-readiness/v1` as a public control-plane contract.
- #723: stabilize `worker-handoff/v1` for harness-independent agents.
- #724: define workflow pulse and CI reliability boundaries.
- #725: research an agent workflow completion benchmark.
- #640: define a read-only MCP surface over Gira workflow-control JSON.
- #715: decide GitHub Projects native view adoption boundaries.
