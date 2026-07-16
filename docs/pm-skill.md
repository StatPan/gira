# Gira PM Skill

Gira PM converts intent into durable, task-local state for coding workers.

Canonical role and authority: [Gira PM Operating
Policy](pm-operating-policy.md). This file defines CLI packets and QA.

## Core Principle

Gira does not use `needs human` as a terminal state.

Classify missing context/policy, conflicts, irreversible risk, insufficient
verification, authority, or undefined success. Retrieve evidence, derive policy,
choose a reversible default, split risk, test, reduce scope, or create a packet.

## Task-Local PM State

The task packet, not thread memory, is authoritative.

V2 packets share actor, problem, outcome, goal, parent/source refs, and
non-goals. Add one profile: discovery, decision, experiment,
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
authority, evidence, assumptions, decision debt, success, and work.
Free prose is retained but not guessed into fields. Goal context is
read-only and requires both `--repo` and `--goal`.

Persist after preview; hydrate current state:

```bash
gira pm record --repo OWNER/REPO --ticket 123 --id evidence.1 --kind evidence --text "Observed result" --source log:5 --dry-run
gira pm context --repo OWNER/REPO --ticket 123
gira pm discovery --repo OWNER/REPO --ticket 123
```

Supersede via new IDs; retries do not overwrite. Store safe references.
Discovery traces outcome→opportunity→hypothesis→experiment→learning→decision.

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

PM QA asks:

- Does the PR solve the stated problem?
- Does it satisfy each acceptance criterion?
- Did it violate any non-goal?
- Did it preserve the decision policy?
- Did it leave unresolved risk that should become a follow-up task?

PM QA does not end at `needs human`. It decomposes missing judgment into policy
repair, context retrieval, risk reduction, implementation, or follow-up work.

## Benchmarked PM Practices

Gira combines Product Owner and user-story outcome alignment; atomic acceptance
criteria; Shape Up appetite and boundaries; Goals-Signals-Metrics evidence;
Working Backwards; and reversible delivery slices. These are proportional
checks, not separate ceremonies: they shape the packet and later test the PR.

Render PM acceptance QA for a linked PR:

```bash
gira pm qa --repo OWNER/REPO --ticket 123 --diff-summary
```
