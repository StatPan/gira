# Gira PM Skill

Gira PM converts raw intent into durable, task-local PM state for coding
workers.

Canonical PM role and authority: [Gira PM Operating
Policy](pm-operating-policy.md). This file defines task packets and acceptance
QA only.

## Core Principle

Gira does not use `needs human` as a terminal state.

When a task appears to require human judgment, Gira first identifies the reason:

- missing context
- missing decision policy
- conflicting constraints
- irreversible risk
- insufficient verification
- authority boundary
- undefined success metric

Then Gira converts that reason into executable work:

- retrieve context
- derive policy from prior decisions
- choose a reversible default
- split irreversible work out
- create verification criteria
- reduce scope
- produce a follow-up task packet

## Task-Local PM State

Thread memory is optional. The task packet is authoritative.

Every PM-generated issue body should include:

- `<!-- gira:pm-state version=1 -->`
- product context
- customer or user outcome
- product goal alignment
- problem
- goal
- decision policy
- appetite and boundary
- acceptance criteria
- signals, metrics, or evidence
- non-goals
- rabbit holes
- context packet
- risk decomposition
- reversibility or rollout
- verification expectations
- suggested worker mode
- next action

## CLI Seed

Compile intent without mutation; compact diagnostics are the default and JSON
contains source-linked `pm-ir/v1`:

```bash
gira pm compile --from-file request.md
gira pm compile --repo OWNER/REPO --goal 123 --from-file request.md --json
```

Headings map to premise, actor, problem, outcome, constraints, non-goals,
authority, evidence, assumptions, decision debt, success, and candidate work.
Free prose is retained but not guessed into missing fields. Goal context is
read-only and requires both `--repo` and `--goal`.

After repairing material diagnostics, use the compatible `pm spec` renderer:

```bash
gira pm spec --repo OWNER/REPO --from-file request.md > pm-task.md
```

Create a GitHub issue from that packet:

```bash
gira ticket new --repo OWNER/REPO --title "TITLE" --body-file pm-task.md --type task --dry-run
```

Apply only after the packet is bounded enough to execute:

```bash
gira ticket new --repo OWNER/REPO --title "TITLE" --body-file pm-task.md --type task --apply
```

## Review Boundary

Engineering review checks code quality, correctness, regression risk, security,
and tests.

PM acceptance QA checks the implemented PR against the task-local PM state:

- Does the PR solve the stated problem?
- Does it satisfy each acceptance criterion?
- Did it violate any non-goal?
- Did it preserve the decision policy?
- Did it leave unresolved risk that should become a follow-up task?

PM QA does not end at `needs human`. If judgment is missing, it decomposes the
missing judgment into a decision policy repair, context retrieval task, risk
reduction task, implementation fix, or follow-up task packet.

## Benchmarked PM Practices

Gira combines Product Owner and user-story outcome alignment; atomic acceptance
criteria; Shape Up appetite and boundaries; Goals-Signals-Metrics evidence;
Working Backwards; and reversible delivery slices. These are proportional
checks, not separate ceremonies: they shape the packet and later test the PR.

Render PM acceptance QA for a linked PR:

```bash
gira pm qa --repo OWNER/REPO --ticket 123 --diff-summary
```
