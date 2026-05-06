# Workspace Backlog

Gira's workspace layer gives a single personal view over repo-agnostic intake and repo execution work. It is the v1 answer for users who want Jira-like backlog capture without starting from a GitHub repository project board.

The source of truth is still GitHub:

| Gira term | GitHub source of truth | Purpose |
| --- | --- | --- |
| Workspace | A local `.gira/config.yaml` grouping | Names the personal operating space and lists the inbox plus execution repos. |
| Inbox | A GitHub repository used for intake issues | Holds tickets that are not ready to be assigned to a repo. |
| Execution repo | Normal GitHub repository | Holds issues that can become branches, PRs, milestones, releases, and done evidence. |
| Route | Issue creation plus parent link | Turns an inbox ticket into a repo execution issue when ownership is clear. |

## Configuration

```yaml
workspace:
  name: personal
  owner: OWNER
  inbox_repo: OWNER/backlog
  repos:
    - OWNER/app
    - OWNER/cli
```

`workspace.inbox_repo` is required. It may be a private personal repo if the backlog should not appear inside any execution repo. `workspace.repos` is the explicit allowlist of repositories that can receive routed execution issues.

The older `portfolio` config remains a compatibility alias, but new docs should use `workspace` because it matches the intended Jira-like user model.

## Daily Flow

Read the whole workspace:

```bash
gira workspace status --config .gira/config.yaml
gira workspace backlog --config .gira/config.yaml
```

Normalize labels and milestones across the inbox and execution repos:

```bash
gira workspace sync --dry-run --config .gira/config.yaml
gira workspace sync --apply --config .gira/config.yaml
```

Capture a repo-agnostic ticket:

```bash
gira workspace ticket new --title "Define billing model" --config .gira/config.yaml
```

Route the ticket once the execution repo is known:

```bash
gira workspace ticket route --ticket 12 --repo OWNER/app --dry-run --config .gira/config.yaml
gira workspace ticket route --ticket 12 --repo OWNER/app --apply --config .gira/config.yaml
```

After routing, continue in the target repo:

```bash
gira ticket start --repo OWNER/app --ticket 34 --apply
gira ticket pr --repo OWNER/app --ticket 34 --apply --draft
gira ticket status --repo OWNER/app --ticket 34
```

## Boundary

This is not GitHub Projects v2 automation and not a separate Jira import/export database. Workspace commands only operate on issues, labels, milestones, and links that remain visible in GitHub.

Use `workspace` for personal or cross-repo intake. Use `ticket`, `sprint`, `release`, and `status` once the work belongs to one execution repo.
