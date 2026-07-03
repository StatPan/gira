# Gira PM Skill

The canonical Gira PM skill lives in
[`docs/pm-skill.md`](https://github.com/StatPan/gira/blob/main/docs/pm-skill.md).

This docs-site page is a thin copy for public navigation. Keep PM policy,
state fields, review boundaries, and benchmarked practices in the canonical
document, then update this page only as a concise route into that source.

## Purpose

Gira PM converts raw intent into durable, task-local PM state that coding
workers can execute without relying on hidden thread memory. Thread memory is
optional; the task packet in GitHub is authoritative.

## Create a PM Packet

```bash
gira pm spec --repo OWNER/REPO --from-file request.md > pm-task.md
gira ticket new --repo OWNER/REPO --title "TITLE" --body-file pm-task.md --type task --dry-run
```

Apply only after the packet is bounded enough to execute:

```bash
gira ticket new --repo OWNER/REPO --title "TITLE" --body-file pm-task.md --type task --apply
```

## PM Acceptance QA

After implementation and engineering review, compare PR evidence with the
task-local PM state:

```bash
gira pm qa --repo OWNER/REPO --ticket 123 --diff-summary
```

PM QA checks whether the PR satisfies the stated product problem, acceptance
criteria, decision policy, non-goals, and unresolved risk. Engineering review
still owns code quality, correctness, regression risk, security, and tests.
