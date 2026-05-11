# Hosted Gira Control-Plane Roadmap

This document records the product and architecture research for a future hosted
Gira service. It does not approve a hosted implementation, credential storage,
background sync, pricing, or broad workflow mutation.

## Verdict

Hosted Gira should not become a Jira clone or a generic engineering analytics
dashboard.

The strongest hosted direction is a GitHub-native workflow closure control
plane:

```text
PM tool ticket
-> GitHub execution issue
-> branch / PR / review / checks
-> evidence-based finish
-> PM tool status convergence
```

The first serviceable wedge is provider health and completion readiness for
teams that already use GitHub as the execution surface and Jira or another PM
tool as the planning surface.

## Product Boundary

| Capability | OSS CLI | Hosted team service | Future enterprise |
| --- | --- | --- | --- |
| GitHub-native ticket lifecycle | Primary implementation | Reads lifecycle evidence and highlights drift | Policy packs and organization-wide controls |
| Jira-primary provider setup | Local `jira init`, `jira mirror`, `jira doctor`, `jira transition --dry-run` | Scheduled provider health and mirror diagnostics | Multi-provider fleet health and compliance reports |
| Dry-run planning | Local text/JSON plans | Web review of dry-run diffs and readiness blockers | Approval workflows, audit retention, export controls |
| Apply operations | Explicit local `--apply` only | No v0 apply by default | Optional gated apply with multi-user approval |
| Dashboard | CLI text/JSON and export artifacts | One-page team health and readiness report | Multi-workspace reporting and SSO/RBAC |
| Agent handoff | GitHub issues, labels, branches, PRs | Queue view over executable issues and blocked agent PRs | Policy-managed queues across workspaces |
| Audit | Local append-only ledgers and GitHub evidence | Server-side event/audit log for plans, checks, and approvals | Retention, export, legal hold, and admin audit integration |
| Background sync | Non-goal until policy is explicit | Scheduled read-only drift detection | Controlled sync jobs with approvals and rollback evidence |

## Minimal Hosted v0

The smallest billable workflow is not "a hosted Jira." It is:

> A private GitHub workspace gets a weekly and on-demand report showing whether
> planned work can safely close.

v0 should include:

- GitHub App installation for selected repositories.
- Optional Jira Cloud connection for Jira-primary workspaces.
- Scheduled read-only `doctor` equivalent for provider health.
- Mirror issue health and duplicate/missing-link diagnostics.
- Completion readiness report for open PRs and linked issues.
- Closure Funnel and stale blocker summary from GitHub metadata.
- Web view for readiness blockers and generated read-only reports.
- Audit log for scans, generated reports, and provider connection changes.

v0 should not include:

- Hosted merge, close, or Jira transition apply by default.
- Approval workflows, warning dismissal workflow, or multi-user apply review.
- Full bidirectional Jira sync.
- Jira workflow mutation or project administration.
- Personal productivity leaderboards.
- Source code, diff, or secret storage by default.
- Dashboard customization beyond the core readiness report.

## Architecture Sketch

```text
GitHub App webhooks + scheduled scans
        |
        v
ingestion workers -> normalized event/store -> derived readiness models
        |                         |                    |
        |                         |                    v
        |                         |             web report / API
        |                         |
        |                         v
        |                  audit log
        |
        v
Jira OAuth/Forge connection -> provider health / transition diagnostics
```

Core server-side state:

- workspace, repo, and provider connection records
- GitHub installation IDs and selected repositories
- encrypted provider credentials or credential references
- sync cursors, webhook delivery IDs, ETags, and freshness timestamps
- normalized issue, PR, check, review, milestone, and Jira key references
- derived readiness reports, drift findings, and dismissal state
- approval records and audit events
- agent queue records derived from GitHub issue labels and PR state

State that should not be stored by default:

- repository source code
- raw PR diffs
- long-lived installation access tokens
- Jira API tokens in plaintext
- individual productivity profiles

## GitHub App Permission Model

Hosted Gira should prefer a GitHub App over user personal tokens because an app
can be installed on selected repositories and can create installation access
tokens with bounded repository and permission scope. GitHub documents that
installation tokens can be limited to specific repositories and cannot exceed
the permissions granted to the app.

Recommended v0 permissions:

| Permission | Level | Why |
| --- | --- | --- |
| Metadata | Read | Repository identity and basic listing. |
| Issues | Read | Issues, labels, milestones, comments, events, and mirror issue health. |
| Pull requests | Read | PR state, review state, merge state, and linkage evidence. |
| Checks | Read | Check suite/run status for completion readiness. |
| Commit statuses | Read | Legacy or non-Actions status contexts. |
| Actions | Read, optional | Workflow run details when check diagnostics require it. |
| Contents | Read, optional | Escalated branch/commit diagnostics only; not a default v0 scope. |
| Organization projects | Read, optional | Only if workspace reporting reads organization Projects. |
| Members | Read, optional | Only for team/RBAC mapping in organizations. |

Write permissions should be separate opt-ins:

- Issues write: comments, labels, mirror issue creation, or close actions.
- Pull requests write: PR comments or merge actions.
- Contents write: branch/file changes, not part of hosted v0.
- Checks write: only if Gira later publishes its own check runs.

The hosted service should subscribe to webhooks instead of polling as the
primary freshness mechanism, then use conditional requests, GraphQL batching,
and rate-limit backoff for reconciliation. GitHub documents GraphQL rate limits
for installation tokens and recommends webhooks, conditional requests,
consolidated GraphQL queries, and reset/retry headers to stay under limits.

## Jira Cloud Auth And Permission Model

For Jira Cloud, hosted Gira should prefer a SaaS OAuth 2.0 app for normal hosted
operation. Forge is a separate Atlassian-hosted app model and should be treated
as an alternate deployment path, not as the default Gira Cloud architecture.
Atlassian documents that OAuth and Forge apps should determine scopes from the
exact API operations used, and it recommends classic scopes where available.

Recommended v0 Jira scopes:

| Scope | Need |
| --- | --- |
| `read:jira-work` | Read issues, project data, statuses, priorities, and work metadata for provider health. |
| `read:jira-user` | Resolve users where assignee or reporter context is shown. |
| `manage:jira-webhook` | Only if the hosted service registers Jira webhooks. |
| `write:jira-work` | Later opt-in for approved comments, issue edits, or status convergence. Not v0 default. |

Scopes that should not be required for v0:

- `manage:jira-project`
- `manage:jira-configuration`
- broad workflow administration scopes

For service-account style operation, Atlassian supports OAuth 2.0 credentials
for service accounts and uses the `api.atlassian.com/ex/jira/<cloudId>/...`
gateway form for Jira public API calls. That model is an enterprise candidate,
not the default v0 path, because it requires centralized Atlassian user
management, credential creation, secure client secret storage, rotation, and
revocation procedures.

## Dry-Run UI Boundary

The hosted UI should show the same plan/apply philosophy as the CLI:

- read current GitHub and Jira state
- compute a deterministic plan
- show blockers, skipped actions, and confidence
- require explicit approval before any future hosted apply
- write an audit event for generated plans and approvals

Good UI surfaces:

- provider health: ready, warning, blocked
- mirror issue health: missing, duplicate, stale, label drift
- completion readiness: linked PR, reviews, checks, mergeability, closing link
- drift diff: expected provider/status/mirror policy versus current state
- agent queue: executable issues, blocked agent PRs, escalation reasons

Bad UI surfaces:

- personal productivity ranking
- hidden status override fields
- one-click workflow mutation
- opaque auto-sync that changes Jira or GitHub without a plan

## Audit And RBAC Requirements

Hosted Gira needs server-side audit and RBAC as soon as it stores provider
connections or approval decisions.

Minimum audit events:

- provider connected, disconnected, or reauthorized
- repository added or removed from a workspace
- scheduled scan started and completed
- doctor/report generated with result status
- dry-run plan generated, after Phase 2 adds plan review
- warning dismissed, after Phase 2 adds dismissal workflow
- approval requested, approved, rejected, or expired, after Phase 2 adds approvals
- hosted apply attempted, succeeded, failed, or rolled back, if apply is ever enabled

Minimum RBAC roles:

| Role | Allowed actions |
| --- | --- |
| Viewer | Read reports and audit events. |
| Operator | Run scans, generate plans, and manage non-secret workspace policy. |
| Approver | Approve future hosted apply operations. |
| Admin | Manage provider connections, repositories, roles, retention, and billing. |

## Agent Handoff Queue

The hosted queue should not execute agents itself in v0. It should expose an
operating queue over GitHub evidence:

- issues labeled `lane:agent` and not `requires-human-approval`
- executable issues with clear acceptance criteria
- open agent-attributed PRs with failed or pending checks
- issues blocked by missing mirror, missing PR, stale review, or failed check
- escalation reasons from `docs/agent-delegation-lanes.md`

The service can later hand off work to Codex, Claude Code, OpenHands, or other
workers through adapters, but the durable contract remains GitHub issue -> PR ->
checks -> merge/close evidence.

## Roadmap

### Phase 0: OSS Readiness

- Keep improving `gira jira doctor`, Closure Funnel stats, and dashboard export
  artifacts.
- Define stable JSON contracts for provider health and completion readiness.
- Add cache freshness and rate-limit reporting to workspace reads.

### Phase 1: Hosted Read-Only Team Health

- GitHub App installation.
- Read-only workspace scan and webhook ingestion.
- Provider health, mirror health, completion readiness, Closure Funnel.
- Weekly report and one-page dashboard.
- Audit log for scans and generated reports.

### Phase 2: Provider Drift And Approval

- Jira OAuth/Forge connection.
- Drift detection for provider config, mirror issues, and status maps.
- Web dry-run diff review.
- Approval records, dismissal workflow, and RBAC.

### Phase 3: Controlled Apply

- Optional hosted apply for narrow, low-risk actions.
- Multi-user approval and policy gates.
- Full audit trail and rollback/remediation guidance.

### Phase 4: Enterprise Controls

- SSO/SAML, SCIM or directory sync, retention controls.
- Workspace templates and policy packs.
- Export APIs and self-hosted or private deployment option.

## Proposed Follow-Up Epics

1. Define hosted provider-health JSON contract.
   - Done when provider health, mirror health, completion readiness, freshness,
     source refs, and redacted error fields have stable JSON names and fixture
     tests.
2. Add workspace scan cache and rate-limit budget reporting.
   - Done when workspace reads expose cache age, rate-limit remaining/reset
     values, partial-scan warnings, and deterministic stale-data behavior.
3. Implement GitHub App read-only ingestion spike.
   - Done when a test app can ingest issues, PRs, checks, statuses, and webhook
     events from selected repositories without `contents:read` by default.
4. Define Jira SaaS OAuth provider connection design.
   - Done when OAuth scopes, cloudId routing, token storage, refresh/rotation,
     revocation, and redacted diagnostics are specified.
5. Design hosted audit log and RBAC model.
   - Done when report generation, provider connection changes, plan creation,
     approvals, and future apply attempts have immutable audit event schemas and
     role checks.

## Sources

- GitHub Apps installation access tokens and permission scoping:
  https://docs.github.com/en/enterprise-cloud@latest/rest/apps/apps
- GitHub endpoint permission reference for GitHub Apps:
  https://docs.github.com/en/rest/authentication/permissions-required-for-github-apps
- GitHub App best practices for webhooks, rate limits, and credential storage:
  https://docs.github.com/en/apps/creating-github-apps/about-creating-github-apps/best-practices-for-creating-a-github-app
- GitHub GraphQL rate limits:
  https://docs.github.com/en/graphql/overview/rate-limits-and-query-limits-for-the-graphql-api
- Atlassian Jira OAuth 2.0 and Forge scopes:
  https://developer.atlassian.com/cloud/jira/platform/scopes-for-oauth-2-3LO-and-forge-apps/
- Atlassian OAuth 2.0 service account credentials:
  https://support.atlassian.com/user-management/docs/create-oauth-2-0-credential-for-service-accounts/
