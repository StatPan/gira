# Project Sync Capability Probe and Permission-Gated Apply (Issue #52)

This document specifies capability-aware apply behavior for `gira project sync`.

## Goals

- Emit explicit capability summary before apply.
- Execute only operations allowed by current token/repo permissions.
- Report skipped operations with stable reasons.

## Capability keys

- `issues:read`
- `issues:write`
- `pullrequests:read`
- `pullrequests:write`
- `projectsv2:read`
- `projectsv2:write`
- `repo:settings:write`

## Execution model

1. Build dry-run plan regardless of permissions.
2. Build capability report.
3. In apply mode, gate each plan step by required capability.
4. Execute allowed actions in deterministic order.
5. Emit `applied`, `skipped`, and `blocked_count` summary.

## Required error line

When an operation is denied, emit the stable message:

`permission denied: <action> requires <capability>`

## JSON output shape

```json
{
  "repo": "OWNER/REPO",
  "command": "project sync",
  "dry_run": false,
  "capabilities": {
    "issues:read": "allowed",
    "issues:write": "allowed",
    "pullrequests:read": "allowed",
    "pullrequests:write": "denied:token_scope",
    "projectsv2:read": "allowed",
    "projectsv2:write": "denied:token_scope",
    "repo:settings:write": "denied:token_scope"
  },
  "applied": [
    {
      "action": "date_validation_report",
      "required": "projectsv2:read",
      "result": "ok"
    }
  ],
  "skipped": [
    {
      "action": "project_status_field:update",
      "required": "projectsv2:write",
      "reason": "permission denied: project_status_field:update requires projectsv2:write"
    }
  ],
  "blocked_count": 1
}
```

## Exit code contract

- Exit `0` when all mandatory operations for the selected command path succeed.
- Exit non-zero when mandatory operations are blocked (`blocked_count > 0`).
- Continue executing non-blocked operations even when some actions are blocked.

## Non-goals for this slice

- No destructive repository settings changes.
- No automatic privilege escalation or token mutation.
