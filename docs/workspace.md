# Workspace Backlog

Gira's workspace layer gives a single personal view over repo-agnostic intake and repo execution work. It is the v1 answer for users who want Jira-like backlog capture without starting from a GitHub repository project board.

The operating model is intentionally close to Terraform: read current GitHub state, show a dry-run plan, then apply the reviewed change. Gira does not keep a separate planning database. It converges GitHub issues, labels, milestones, Projects, branches, and PRs into the workflow state described by `.gira/config.yaml` and Gira-managed conventions.

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
  project:
    owner: OWNER
    title: Gira
```

`workspace.inbox_repo` is required. It may be a private personal repo if the backlog should not appear inside any execution repo. `workspace.repos` is the explicit allowlist of repositories that can receive routed execution issues.

For personal multi-repo operation, prefer the OS-user global registry:

```bash
gira workspace init --scope global --name personal --inbox-repo OWNER/backlog --repo OWNER/app --dry-run
gira workspace init --scope global --name personal --inbox-repo OWNER/backlog --repo OWNER/app --apply
gira workspace status --config ~/.config/gira/workspaces/personal.yaml
```

For a shared repository contract, keep using repo scope:

```bash
gira workspace init --scope repo --inbox-repo OWNER/backlog --repo OWNER/app --path . --dry-run
gira workspace init --scope repo --inbox-repo OWNER/backlog --repo OWNER/app --path . --apply
```

Repo scope writes `.gira/config.yaml` in the checkout. Global scope writes
`~/.config/gira/workspaces/NAME.yaml` and leaves the repository untouched.

`workspace.project` points to an existing GitHub Projects v2 board by user-facing title or Project number. The Project may be owned by a user profile or org and still act as a repo board when it is linked to configured repos and populated with repo issues. Gira does not create the Project in this slice; it syncs configured repo issues, supported planning fields, status, and milestone target dates into that Project so the GitHub Projects tab stays visible without manual `gh project` commands. `number` is supported as the fallback when titles are ambiguous.

Adopt an existing profile or org Project into config before running sync:

```bash
gira workspace project adopt --owner OWNER --title "Gira" --config .gira/config.yaml --dry-run
gira workspace project adopt --owner OWNER --title "Gira" --config .gira/config.yaml --apply
gira workspace project adopt --owner OWNER --number 7 --config .gira/config.yaml --dry-run
```

Adoption only records `workspace.project`; it does not create or replace GitHub Projects. If `workspace.project` already points at the selected Project, the command skips. If it points somewhere else, adoption fails because replace support is intentionally out of scope. The next step after apply is always:

```bash
gira projects sync --config .gira/config.yaml --dry-run
```

Repository issues, labels, and milestones remain the execution source of truth. The GitHub Project is the visibility and planning surface that `projects sync` mirrors into.

The older `portfolio` config remains a compatibility alias, but new docs should use `workspace` because it matches the intended Jira-like user model.

## Daily Flow

Read the whole workspace:

```bash
gira workspace status --config .gira/config.yaml
gira workspace backlog --config .gira/config.yaml
gira workspace list --config .gira/config.yaml
```

Normalize labels and milestones across the inbox and execution repos:

```bash
gira workspace sync --dry-run --config .gira/config.yaml
gira workspace sync --apply --config .gira/config.yaml
```

Sync visible GitHub Projects board items:

```bash
gira projects sync --dry-run --config .gira/config.yaml
gira projects sync --apply --config .gira/config.yaml
```

Capture a repo-agnostic ticket:

```bash
gira workspace ticket new --title "Define billing model" --config .gira/config.yaml
```

Create and route in one command once the execution repo is known:

```bash
gira workspace ticket new "Define billing model" --repo OWNER/app --dry-run --config .gira/config.yaml
gira workspace ticket new "Define billing model" --repo OWNER/app --apply --config .gira/config.yaml
```

Route an older or externally-created inbox ticket by number:

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

This is not a separate Jira import/export database. Workspace commands operate on issues, labels, milestones, and links that remain visible in GitHub. `gira projects sync` links and mirrors repository issue state into an existing GitHub Project; repo issues, labels, and milestones remain the source of truth. It mirrors `priority:*` labels to `Priority`, `area:*` labels to `Layer / workstream`, `agent:*` labels to `Owner / agent`, status labels to `Status`, and milestone due dates to `Target date`.

Project items that are not linked to repository issues are intake, portfolio, or visibility context only. Route or lower them to a repo issue before using ticket lifecycle commands.

GitHub assignees remain the accountable human owners. The `agent:*` label is only the execution actor or workflow hint, for example `agent:human`, `agent:codex`, `agent:reviewer`, or `agent:gira`. Do not use `Owner / agent` as a replacement for assignee or reporter metadata.

Closed issues stay in the Project as `Done` by default; use `--archive-closed` only when the active Project item set should drop completed work. GitHub Project views still need to be created in the GitHub UI because supported CLI/GraphQL APIs do not currently create or edit views.

Use `workspace` for personal or cross-repo intake. Use `ticket`, `sprint`, `release`, and `status` once the work belongs to one execution repo.
