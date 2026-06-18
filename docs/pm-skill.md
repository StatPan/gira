# Gira PM Skill

Gira PM converts raw intent into durable, task-local PM state that coding
workers can execute without relying on hidden thread memory.

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

This lets implementation workers, engineering reviewers, and PM acceptance QA
rehydrate the same task context from GitHub instead of depending on a private
chat transcript.

## CLI Seed

Render a worker-ready PM task packet:

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

Gira PM templates combine several proven product-development practices:

- Scrum/Product Owner discipline: connect work to product goals and make backlog items transparent enough to execute.
- User-story discipline: name the affected user or job, the desired outcome, and the reason the work matters.
- Acceptance criteria discipline: define pass/fail, outcome-focused criteria that tell developers when to stop, QA how to test, and PM what to expect.
- Shape Up discipline: set appetite, boundaries, rabbit holes, and no-gos before implementation.
- Goals-Signals-Metrics discipline: map intended outcomes to observable signals or evidence.
- Working Backwards discipline: reason from the desired customer experience back to the work.
- Acceptance-testing discipline: decompose requirements into atomic, testable criteria.
- Reversibility discipline: reduce risky work through small slices, rollout plans, feature flags, compatibility paths, or branch-by-abstraction.

These practices are not separate ceremony. They are the checklist Gira uses to
turn raw intent into a worker-ready task packet and later verify whether a PR
actually satisfies that packet.

Render PM acceptance QA for a linked PR:

```bash
gira pm qa --repo OWNER/REPO --ticket 123 --diff-summary
```
