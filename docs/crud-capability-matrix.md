# Gira CRUD Capability Matrix (MVP)

This document defines the command contract for CRUD behavior by surface in the Go CLI MVP.

## Support Levels

- **Supported**: command exists and is intended for daily use in MVP.
- **Supported (opt-in destructive)**: operation exists but requires explicit apply semantics.
- **Unsupported (intentional)**: operation is intentionally out of MVP scope; CLI should return explicit guidance.

## Matrix

| Surface | Create | Read | Update | Delete |
|---|---|---|---|---|
| Labels | `gira sync --repo OWNER/REPO` | `gira sync --repo OWNER/REPO --dry-run` | `gira sync --repo OWNER/REPO` | Unsupported (intentional in MVP) |
| Milestones | `gira sync --repo OWNER/REPO` | `gira sync --repo OWNER/REPO --dry-run` | `gira sync --repo OWNER/REPO` | Unsupported (intentional in MVP) |
| Issues (bootstrap/ops) | `gira sync --repo OWNER/REPO --bootstrap-issues` | `gira status --repo OWNER/REPO` | `gira triage apply --apply`, `gira worker claim|handoff|release` | Unsupported direct delete in MVP |
| PR loop | `gira dev pr open --repo OWNER/REPO --issue N` | `gira dev pr status --repo OWNER/REPO --issue N`, `gira review queue` | `gira merge queue --apply` (opt-in destructive) | Unsupported direct delete; close via GitHub UI/API |
| Project fields/views | Unsupported (MVP non-goal) | `gira project capability`, `gira project sync --dry-run`, `gira project transitions --dry-run` | Unsupported in MVP (dry-run inspection only) | Unsupported (MVP non-goal) |

## Unsupported Operation Guidance

For the contract command family, unsupported operations return explicit guidance:

- `gira contract <op>` where `<op>` is not `crud` returns:
  - `unsupported contract operation: <op>`
  - `supported operation: gira contract crud`

This explicit response is intentional so automation can fail-closed with a clear remediation path.
