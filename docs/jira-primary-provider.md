# Jira-Primary Provider Mode

Gira defaults to GitHub-native operation. GitHub issues are executable work packets, branches show work start, PRs are change units, checks and review are execution evidence, and merged PR plus closed issue is completion evidence.

Jira-primary mode is optional. It is for teams that already plan in Jira but want Gira to keep GitHub execution safe and observable.

## Source Of Truth

| Domain | Owner | Gira behavior |
| --- | --- | --- |
| Planning | Jira | Jira issue key, summary, priority, assignee, and planning status remain Jira-owned. |
| Status | Jira | GitHub `status:*` labels are mirrors or execution hints, not the authority. |
| Execution | GitHub | Branch, PR, review, checks, merge, and closing-link evidence remain authoritative. |
| Completion | Both | Jira Done is allowed only after GitHub execution evidence is clean. |

This split prevents Jira from marking work Done before the linked GitHub PR is reviewable, passing, merged, and connected to a mirror issue.

## Setup

Discover the Jira project and preview the provider config:

```bash
gira jira init \
  --repo OWNER/REPO \
  --api-base https://example.atlassian.net \
  --project ABC \
  --dry-run
```

Apply only after reviewing the generated non-secret config:

```bash
gira jira init \
  --repo OWNER/REPO \
  --api-base https://example.atlassian.net \
  --project ABC \
  --apply
```

The provider block is stored in the user-global repo registry, such as `~/.config/gira/repos/OWNER/REPO.yaml`. It stores base URL, project key, source-of-truth policy, and status mapping. It must not store Jira tokens.

Credentials are read from environment variables:

```bash
export JIRA_EMAIL=you@example.com
export JIRA_API_TOKEN=...
```

## Mirror And Work

Create or reuse a GitHub mirror issue for one Jira key:

```bash
gira jira mirror ABC-123 --repo OWNER/REPO --dry-run
gira jira mirror ABC-123 --repo OWNER/REPO --apply
```

Then use the normal ticket lifecycle with either the Jira key or GitHub mirror issue number:

```bash
gira ticket view ABC-123 --repo OWNER/REPO
gira ticket start ABC-123 --repo OWNER/REPO --apply
gira ticket pr --repo OWNER/REPO --apply --draft
gira ticket checks --repo OWNER/REPO
gira ticket finish --repo OWNER/REPO --dry-run
gira ticket finish --repo OWNER/REPO --apply
```

`ticket finish` refuses Jira Done when GitHub evidence is incomplete. Blockers include missing mirror issue, missing linked PR, draft PR, review blockers, failing or pending checks, and an unmerged PR. Apply performs the Jira Done transition only after GitHub merge evidence is clean and `--apply` is explicit.

## Transition Planning

Inspect Jira workflow reachability without mutation:

```bash
gira jira transition ABC-123 --repo OWNER/REPO --to done --dry-run
```

The planner reports current Jira status, mapped target statuses, candidate transition, required fields, allowed transition count, and a decision such as `direct_transition`, `already_at_target`, `missing_transition`, `unmapped_status`, or `manual_admin_required`.

## Import And Export

Migration utilities are explicit. Imports are dry-run-first because they can create GitHub issues. Export writes local artifacts for review and does not mutate GitHub or Jira.

```bash
gira jira import --repo OWNER/REPO --source jira.csv --dry-run
gira jira import --repo OWNER/REPO --api-base https://example.atlassian.net --project ABC --dry-run
gira jira export --repo OWNER/REPO --output out/jira
```

These commands help inspect and migrate data. They do not make Gira a background sync engine.

## Boundaries

The OSS CLI supports provider discovery, mirror issue creation, read-only transition planning, finish-time Done gating, and import/export migration helpers.

The first Jira-primary slices do not support:

- Jira workflow status creation or workflow mutation.
- Jira-only completion.
- Background polling or background sync.
- Full bidirectional sync.
- Hosted dashboards or hosted operational services.
- Storing Jira secrets in repo-local or user-global config.

GitHub-native mode remains the default. Enabling Jira-primary requires explicit provider config and does not hide the GitHub execution loop.
