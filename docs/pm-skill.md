# Gira PM Skill

Gira PM converts raw intent into durable, task-local PM state for coding
workers.

Canonical PM role and authority: [Gira PM Operating
Policy](pm-operating-policy.md). This file defines PM CLI packets and QA.

## Core Principle

Gira does not use `needs human` as a terminal state.

First classify missing context/policy, conflicting constraints, irreversible
risk, insufficient verification, authority, or undefined success. Then retrieve
evidence, derive policy, choose a reversible default, split risk, add a test,
reduce scope, or create a follow-up packet.

## Task-Local PM State

Thread memory is optional. The task packet is authoritative.

V2 packets share actor, problem, outcome, goal alignment, parent/source refs, and
non-goals. Add only the chosen profile: discovery, decision, experiment,
delivery, rollout, measurement, or documentation. `legacy` retains the v1
universal packet. Delivery additionally requires `Product Uncertainty` to be
`resolved`; promote discovery/decision/experiment work by retaining its stable
reference in both Parent Context and Source References.

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

Persist after preview; hydrate current state:

```bash
gira pm record --repo OWNER/REPO --ticket 123 --id evidence.1 --kind evidence --text "Observed result" --source log:5 --dry-run
gira pm context --repo OWNER/REPO --ticket 123
```

Supersede via new IDs; retries do not overwrite. Store safe references.

After repairing diagnostics, render the smallest sufficient packet:

```bash
gira pm spec --profile delivery --context-ref issue:OWNER/REPO#100 --repo OWNER/REPO --from-file request.md > pm-task.md
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
