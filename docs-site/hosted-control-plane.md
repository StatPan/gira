# Hosted Control Plane

Hosted Gira is a future product direction, not part of the current OSS CLI.

The recommended wedge is a GitHub-native workflow closure control plane:

```text
PM tool ticket
-> GitHub execution issue
-> branch / PR / review / checks
-> evidence-based finish
-> PM tool status convergence
```

## Boundary

The OSS CLI should keep local dry-run/apply commands, GitHub-native ticket
lifecycle, Jira provider diagnostics, import/export helpers, and
single-operator workflows.

A hosted team service can add scheduled read-only checks, provider health,
mirror issue health, completion readiness, drift reports, dry-run diff review,
audit logs, RBAC, and agent queue visibility.

Enterprise can later add SSO, retention controls, policy packs, approval
workflows, and carefully gated hosted apply operations.

## Minimal v0

v0 should be read-only by default:

- GitHub App installation for selected repositories.
- Optional Jira Cloud connection.
- Scheduled provider and mirror diagnostics.
- Completion readiness and Closure Funnel reports.
- One-page team health dashboard.
- Audit log for scans, generated reports, and provider connection changes.

v0 should not include hosted merge/close/apply, full bidirectional Jira sync,
approval workflow, warning dismissal workflow, Jira workflow mutation, source
code storage, personal productivity leaderboards, or dashboard customization.

## Auth Direction

GitHub should use a GitHub App with least-privilege installation tokens and no
default source-content scope. Jira Cloud should use a SaaS OAuth 2.0 app for the
normal hosted path; Forge is a separate Atlassian-hosted deployment option. The
detailed roadmap is in
[docs/hosted-control-plane-roadmap.md](https://github.com/StatPan/gira/blob/main/docs/hosted-control-plane-roadmap.md).
