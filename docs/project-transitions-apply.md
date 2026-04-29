# Project Transitions Apply Mode (Issue #51)

This document defines the implementation contract for adding `--apply` support to `gira project transitions` while keeping the current dry-run behavior deterministic.

## Scope

- Keep `--dry-run` as the default planning mode.
- Add `--apply` for **issue-level status field updates only**.
- Do not close issues, merge PRs, or mutate milestone state in apply mode.

## Apply target

For this slice, issue-level status updates are represented as managed status labels:

- `status:backlog`
- `status:ready`
- `status:blocked`
- `status:in-progress`
- `status:in-review`
- `status:done`

The apply step should:

1. Read transition plan output.
2. Filter to `decision=apply` and `target_type=issue`.
3. For each issue, remove existing `status:*` labels.
4. Add exactly one mapped status label from the transition `to` state.

## State mapping

| Transition `to` | Label to apply |
| --- | --- |
| `Backlog` | `status:backlog` |
| `Ready` | `status:ready` |
| `Blocked` | `status:blocked` |
| `In progress` | `status:in-progress` |
| `In review` | `status:in-review` |
| `Done` | `status:done` |

## Safety and idempotency

- Applying the same plan repeatedly must be idempotent.
- Non-issue targets (for example `milestone_all_closed`) remain report-only in this slice.
- If an issue already has the target status label and no conflicting `status:*` labels, emit `skip` with reason `already_set`.

## Machine-readable apply result shape

```json
{
  "repo": "OWNER/REPO",
  "command": "project transitions",
  "dry_run": false,
  "applied": [
    {
      "issue": 27,
      "rule_id": "pr_opened",
      "from": "Ready",
      "to": "In review",
      "label_applied": "status:in-review"
    }
  ],
  "skipped": [
    {
      "issue": 42,
      "rule_id": "branch_started",
      "reason": "already_set"
    }
  ]
}
```

## Explicit non-goals

- No Projects v2 field writes in this slice.
- No issue close/reopen mutations.
- No milestone close operations.
- No PR state mutations.
