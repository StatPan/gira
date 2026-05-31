# Workspace Backlog

Gira's workspace layer gives a single personal view over repo-agnostic intake and repo execution work. It is the v1 answer for users who want Jira-like backlog capture without starting from a GitHub repository project board.

The operating model is intentionally close to Terraform: read current GitHub state, show a dry-run plan, then apply the reviewed change. Gira does not keep a separate planning database. It converges GitHub issues, labels, milestones, Projects, branches, and PRs into the workflow state described by `.gira/config.yaml` and Gira-managed conventions.

The source of truth is still GitHub:

| Gira term | GitHub source of truth | Purpose |
| --- | --- | --- |
| Workspace | A local `.gira/config.yaml` grouping | Names the personal operating space and lists the inbox plus execution repos. |
| Inbox | A GitHub repository used for backlog/intake issues | Holds tickets that are not ready to be assigned to an execution repo. |
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

For multi-repo global operation, prefer a dedicated backlog repo such as
`OWNER/backlog`. Using the same repo for `inbox_repo` and `repos` is fine for a
small single-repo setup, but it mixes untriaged intake with product execution
issues.

For personal multi-repo operation, prefer the OS-user global registry:

```bash
gira setup global --repo OWNER/app --path . --workspace personal --inbox-repo OWNER/backlog --dry-run
gira setup global --repo OWNER/app --path . --workspace personal --inbox-repo OWNER/backlog --apply
gira workspace status
gira workspace status --limit 10 --active-only
gira workspace status --repo OWNER/app --refresh
```

`gira setup global` creates the global default config, workspace entry, and repo
registry entry together. Use `--mode global-only` when personal global config is
the operating source. Use `--mode hybrid` when an existing repo-local
`.gira/config.yaml` should remain referenced as a shared contract.

After the first global setup, populate the workspace execution repo allowlist
from a GitHub user or organization:

```bash
gira workspace repos sync --owner OWNER --workspace personal --dry-run
gira workspace repos sync --owner OWNER --workspace personal --apply
```

Repository discovery is opt-in. Gira does not automatically scan every GitHub
repo during normal workspace commands. The workspace inbox repo is treated as
backlog/intake and is skipped from `workspace.repos`; pass `--include-archived`
only when archived repositories should remain visible in the workspace.

For large global workspaces, `workspace status` is rate-limit aware. It checks
the GitHub API budget when available, bounds concurrent repo fetches with
`--max-concurrency` (default `4`), and caches per-repo status under the Gira
cache directory for `--cache-ttl` (default `5m`). Use `--repo`, `--limit`, and
`--active-only` for frequent CLI or future GUI refreshes. Use `--refresh` only
when a fresh full read is more important than preserving API budget.

For a shared repository contract, keep using repo scope:

```bash
gira workspace init --scope repo --inbox-repo OWNER/backlog --repo OWNER/app --path . --dry-run
gira workspace init --scope repo --inbox-repo OWNER/backlog --repo OWNER/app --path . --apply
```

Repo scope writes `.gira/config.yaml` in the checkout. Global scope writes
`~/.config/gira/workspaces/NAME.yaml` and leaves the repository untouched.
If `.gira/config.yaml` already exists as a repo-local contract without a
`workspace:` block, use explicit merge mode to add only workspace fields while
preserving the existing repo contract:

```bash
gira workspace init --scope repo --inbox-repo OWNER/backlog --repo OWNER/app --path . --merge --dry-run
gira workspace init --scope repo --inbox-repo OWNER/backlog --repo OWNER/app --path . --merge --apply
```

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
gira ticket start 34 --repo OWNER/app --apply
gira ticket pr --apply --draft
gira ticket review --diff-summary
gira ticket status
```

## Workspace Queues

`workspace-queues/v1` is the read-only contract for turning ticket and PR
evidence into operator queues. `gira workspace status --json` includes it under
`workspace_queues`. The contract maps existing `ticket status` evidence,
including `ticket_readiness`, `pr_readiness`, checks, review state, labels,
blockers, and next actions, into stable queue items.

For compact workspace reads, Gira builds ready, blocked, and human-decision
queues from the normal workspace issue summary. It reads detailed ticket status
for `status:in-review` candidates so review, finish, and failed-check queues can
use PR, checks, and review evidence without scanning every issue deeply.

The JSON report shape is:

```json
{
  "schema_version": "workspace-queues/v1",
  "workspace": {"name": "personal", "owner": "OWNER"},
  "queues": {
    "agent_ready": [],
    "review_needed": [],
    "finish_ready": [],
    "blocked": [],
    "failed_check": [],
    "human_decision": []
  },
  "counts": {
    "agent_ready": 0,
    "review_needed": 0,
    "finish_ready": 0,
    "blocked": 0,
    "failed_check": 0,
    "human_decision": 0
  },
  "privacy_boundary": {
    "scope": "work_item_state_only",
    "prohibited": [
      "personal_productivity_ranking",
      "agent_productivity_ranking",
      "time_online_scoring",
      "token_spend_scoring"
    ]
  }
}
```

Each queued item has:

| Field | Meaning |
| --- | --- |
| `queue` | Queue name that emitted the item. |
| `repo`, `issue`, `title`, `state`, `status`, `labels`, `milestone` | GitHub issue identity and workflow labels. |
| `pull_request` | PR number, URL, state, draft flag, and review decision when known. |
| `evidence` | Normalized evidence names: `ticket_readiness`, `pr_readiness`, `checks_status`, `review_status`, `next_action`, and `blockers`. |
| `reason_codes` | Stable machine-readable reasons for membership. |
| `next_safe_command` | The next Gira command an operator can run without mutating hidden state unexpectedly. |

Queue membership rules:

| Queue | Membership evidence | Next safe command |
| --- | --- | --- |
| `agent_ready` | Open issue with `status:ready`, no linked PR, no blockers, no human-decision label, and missing or `ready` `ticket_readiness`. | `gira ticket start --repo OWNER/REPO --ticket N --apply` |
| `review_needed` | Open non-draft PR with `status:in-review`, missing or required review state, or `pr_readiness.next_action=request_review`. | `gira ticket review --repo OWNER/REPO --ticket N --diff-summary --json` |
| `finish_ready` | Open non-draft PR with `pr_readiness=ready_for_finish`, `next_action=merge_when_policy_allows` or `finish_ticket`, or finish-ready evidence, with passed checks, approved review, and no blockers. | `gira ticket finish --repo OWNER/REPO --ticket N --dry-run` |
| `blocked` | `status:blocked`, explicit blockers, or error findings from ticket or PR readiness. | `gira ticket status --repo OWNER/REPO --ticket N --json` |
| `failed_check` | Failed or failing checks, a checks blocker, or PR readiness check-failure findings. | `gira ticket status --repo OWNER/REPO --ticket N --json` |
| `human_decision` | Labels such as `agent:human`, `needs:human`, `needs:decision`, `type:decision`, or readiness `next_action=ask_human`. | `gira ticket handoff --repo OWNER/REPO --ticket N planner --json` |

Queues are not mutually exclusive. For example, a PR with failed checks can
appear in both `blocked` and `failed_check` so a dashboard can show both the
general blocker lane and the specialized CI lane without losing evidence.

The privacy boundary is part of the contract: workspace queues describe
work-item state and safe next commands only. They must not rank people or
agents, score productivity, infer availability from time online, or turn token
spend into an execution metric.

This is the CLI/read-only shape for the hosted control-plane direction: a future
hosted view can render the same queues across workspaces, but GitHub issues and
PRs remain the source of truth and mutation still flows through explicit Gira
commands.

For the Gira 3.0 local dashboard bundle, the contract gap between
`workspace status --json`, `workspace-queues/v1`, and `gira export dashboard`
is documented in
[Workspace Dashboard Contract Gaps](workspace-dashboard-contract-gaps.md).

For the broader ownership boundary between labels, computed JSON state,
receipts, and local cache, see [Gira State Model](state-model.md).

## Boundary

This is not a separate Jira import/export database. Workspace commands operate on issues, labels, milestones, and links that remain visible in GitHub. `gira projects sync` links and mirrors repository issue state into an existing GitHub Project; repo issues, labels, and milestones remain the source of truth. It mirrors `priority:*` labels to `Priority`, `area:*` labels to `Layer / workstream`, `agent:*` labels to `Owner / agent`, status labels to `Status`, and milestone due dates to `Target date`.

Project items that are not linked to repository issues are intake, portfolio, or visibility context only. Route or lower them to a repo issue before using ticket lifecycle commands.

GitHub assignees remain the accountable human owners. The `agent:*` label is only the execution actor or workflow hint, for example `agent:human`, `agent:codex`, `agent:reviewer`, or `agent:gira`. Do not use `Owner / agent` as a replacement for assignee or reporter metadata.

Closed issues stay in the Project as `Done` by default; use `--archive-closed` only when the active Project item set should drop completed work. GitHub Project views still need to be created in the GitHub UI because supported CLI/GraphQL APIs do not currently create or edit views.

Use `workspace` for personal or cross-repo intake. Use `ticket`, `sprint`, `release`, and `status` once the work belongs to one execution repo.
