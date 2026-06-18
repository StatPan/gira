# Gira PM Skill

Gira PM converts raw intent into durable, task-local PM state that coding
workers can execute without relying on hidden thread memory.

## Principle

Gira does not use `needs human` as a terminal state. If a task appears to need
human judgment, Gira decomposes why and converts that reason into executable
work: retrieve context, derive a decision policy, reduce risk, add verification,
split scope, or create a follow-up task packet.

## Render a PM task packet

```bash
gira pm spec --repo OWNER/REPO --from-file request.md > pm-task.md
```

Then create a ticket from the durable packet:

```bash
gira ticket new --repo OWNER/REPO --title "TITLE" --body-file pm-task.md --type task --dry-run
```

The generated packet includes:

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

Thread memory is optional. The task packet is authoritative.

## PM acceptance QA

After implementation and engineering review, rehydrate the same PM state from
the issue and compare it with PR evidence:

```bash
gira pm qa --repo OWNER/REPO --ticket 123 --diff-summary
```

PM QA checks product/task acceptance. Engineering review still owns code
quality, correctness, regression risk, security, and tests.

PM QA expects an implementation claims matrix:

| Acceptance criterion | PR claim | Evidence | PM QA result |
| --- | --- | --- | --- |
| Criterion from PM state | Claimed behavior | Test, diff, screenshot, log, command, or explanation | accepted / mismatch / unknown |
