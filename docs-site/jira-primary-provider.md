# Jira-Primary Provider Mode

Gira is GitHub-native by default. Jira-primary mode is optional for teams that plan in Jira while executing code through GitHub PRs.

## Ownership Split

| Domain | Owner | Rule |
| --- | --- | --- |
| Planning | Jira | Jira issue key, summary, priority, assignee, and planning status remain Jira-owned. |
| Status | Jira | GitHub `status:*` labels mirror or summarize Jira status when Jira is primary. |
| Execution | GitHub | Branch, PR, review, checks, merge, and close evidence remain authoritative. |
| Done | Both | Jira Done is allowed only after GitHub execution evidence is clean. |

## Setup

```bash
gira jira init --repo OWNER/REPO --api-base https://example.atlassian.net --project ABC --dry-run
gira jira init --repo OWNER/REPO --api-base https://example.atlassian.net --project ABC --apply
```

Provider config is written to the user-global repo registry, for example `~/.config/gira/repos/OWNER/REPO.yaml`. It stores non-secret configuration only. Use `JIRA_EMAIL` and `JIRA_API_TOKEN` for credentials.

## Mirror And Execute

```bash
gira jira mirror ABC-123 --repo OWNER/REPO --dry-run
gira jira mirror ABC-123 --repo OWNER/REPO --apply
gira ticket view ABC-123 --repo OWNER/REPO
gira ticket start ABC-123 --repo OWNER/REPO --apply
gira ticket pr --repo OWNER/REPO --apply --draft
gira ticket checks --repo OWNER/REPO
gira ticket finish --repo OWNER/REPO --dry-run
gira ticket finish --repo OWNER/REPO --apply
```

`ticket finish` blocks Jira Done while any GitHub execution evidence is incomplete: missing mirror issue, missing linked PR, draft PR, review blocker, failing or pending checks, or unmerged PR.

## Transition Planning

```bash
gira jira transition ABC-123 --repo OWNER/REPO --to done --dry-run
```

The planner is read-only. It reports current Jira status, configured target statuses, candidate transition, required fields, and whether manual workflow administration is required.

## Migration Helpers

```bash
gira jira import --repo OWNER/REPO --source jira.csv --dry-run
gira jira import --repo OWNER/REPO --api-base https://example.atlassian.net --project ABC --dry-run
gira jira export --repo OWNER/REPO --output out/jira
```

Imports are dry-run-first because they can create GitHub issues. Export writes local artifacts for review and does not mutate GitHub or Jira. Import/export commands are migration helpers, not a background sync system.

## Boundaries

The OSS CLI does not create or mutate Jira workflows, run background bidirectional sync, perform Jira-only completion, store Jira secrets in config, or provide hosted dashboards. Those are future operational capabilities, not hidden defaults.
