# Gira PM Skill

The canonical Gira PM skill lives in
[`docs/pm-skill.md`](https://github.com/StatPan/gira/blob/main/docs/pm-skill.md).
The shared human/AI PM role and authority rules live in the canonical
[`docs/pm-operating-policy.md`](https://github.com/StatPan/gira/blob/main/docs/pm-operating-policy.md).

This docs-site page is a thin copy for public navigation. Keep PM policy,
state fields, review boundaries, and benchmarked practices in the canonical
document, then update this page only as a concise route into that source.

## Purpose

Gira PM converts raw intent into durable, task-local PM state that coding
workers can execute without relying on hidden thread memory. Thread memory is
optional; the task packet in GitHub is authoritative.

## Create a PM Packet

Compile intent into bounded diagnostics without mutation, then use `--json`
when the complete source-linked `pm-ir/v1` is needed:

```bash
gira pm compile --from-file request.md
gira pm compile --repo OWNER/REPO --goal 123 --from-file request.md --json
```

The compiler preserves supplied meaning and marks unresolved fields rather than
guessing product semantics. Persist typed state only after preview:

```bash
gira pm record --repo OWNER/REPO --ticket 123 --id evidence.1 --kind evidence --text "Observed result" --source log:5 --dry-run
gira pm context --repo OWNER/REPO --ticket 123
```

Typed records are append-only GitHub comments. Identical retries are
idempotent; supersession preserves history; compact context expands with
`--json`. Secrets and private transcripts are rejected.

After repairing diagnostics, choose the smallest task profile. Delivery is the
default; use discovery, decision, experiment, rollout, measurement, or
documentation as appropriate. `legacy` retains v1:

```bash
gira pm spec --profile delivery --context-ref issue:OWNER/REPO#100 --repo OWNER/REPO --from-file request.md > pm-task.md
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
