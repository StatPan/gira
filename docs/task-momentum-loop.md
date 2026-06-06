# Task Momentum Loop

This document defines the first Gira-native task momentum design.

Parent issue: #704.
Implementation successor: #706.

## Decision

Gira should not copy token, streak, quota, leaderboard, or activity-score
mechanics. The first task momentum loop should translate that product need into
Gira's existing operating model:

- evidence-backed progress;
- visible continuity across tickets, queues, reviews, finish evidence, and
  report bundles;
- workflow hygiene as real progress;
- no social comparison or worker surveillance;
- GitHub as the canonical execution source.

The first implementation should use `pulse` as the primary user-facing word and
reserve `momentum` for the longer-term design category.

`pulse` means a compact recent-period report of meaningful workflow movement.
It answers:

```text
What moved, what got healthier, and what still needs operator attention?
```

It is not a personal productivity score.

## Vocabulary

| Word | Decision | Meaning |
| --- | --- | --- |
| `pulse` | Use first | Recent meaningful workflow movement for a repo or workspace. |
| `momentum` | Use as category | Rolling signal built from repeated pulse snapshots or event windows. |
| `cadence` | Later | Gentle continuity over days or weeks; it should not punish rest. |
| `credit` | Avoid for now | Too close to a reward economy unless an internal weighted model needs it. |
| `token` | Do not use | Conflicts with LLM spend language and reward-token products. |
| `streak` | Do not use first | Too pressure-oriented for Gira's operator workflow. |
| `score` | Avoid in UI | Invites ranking and gaming. Use counts, evidence, and health signals. |

The CLI wording should stay short. `pulse` is concise enough for commands and
still different from existing `status`, `queue`, and `stats` concepts.

## Source Contracts

The first pulse model must be derived from existing Gira/GitHub evidence:

- GitHub issues, labels, milestones, PRs, reviews, checks, and closing links.
- Gira status labels such as `status:ready`, `status:in-progress`,
  `status:in-review`, `status:done`, and `resolution:superseded`.
- Gira receipts such as `finish-receipt/v1`, supersede comments, self-review
  notes, and handoff notes when present.
- `ticket-status/v1`, `pr-readiness/v1`, and `finish-readiness/v1`.
- `workspace-queues/v1` for current queue pressure and next safe actions.
- `dashboard_export/v1alpha1` and `workspace-dashboard/v1alpha1` as optional
  exported views, not canonical inputs.

Local export artifacts may be read only as derived snapshots with visible
freshness metadata. They must not become the source of truth for momentum.

## Signal Model

Pulse should group evidence into named signals instead of one opaque score.

| Signal | Counts When | Source Evidence |
| --- | --- | --- |
| `finished` | A ticket closes through a linked merged PR or finish receipt. | PR state, closing reference, issue state, `finish-receipt/v1`. |
| `reviewed` | A PR receives review movement or becomes finish-ready with checks passing. | PR review decision, checks, `pr-readiness/v1`. |
| `refined` | An unready issue becomes worker-handoff-ready with goal, scope, and acceptance. | label transition to `status:ready`, `ticket-readiness/v1`. |
| `unblocked` | A blocked or human-decision item leaves that lane with evidence. | label/status transition, blocker list, decision note. |
| `superseded` | Stale or replaced work closes with `resolution:superseded`. | issue state, resolution label, supersede note. |
| `started` | Ready work is started through Gira lifecycle commands. | branch policy evidence, `status:in-progress`, branch name. |
| `checked` | A local report bundle is generated as an operator checkpoint. | dashboard manifest, snapshot freshness, warning list. |

`checked` should be separated from work movement because report generation does
not change canonical execution state. It is still useful as an operator habit
signal.

## Anti-Gaming Rules

The first pulse model should reject or down-rank low-value activity:

- Do not reward empty comments, repeated comments, or generic status chatter.
- Do not count issue creation by itself.
- Do not count label churn unless it moves a work item across a meaningful Gira
  lifecycle boundary.
- Do not count closing an issue as finished unless it has a linked merged PR,
  finish receipt, or an explicit non-done resolution such as superseded.
- Do not count report generation as execution progress.
- Do not rank people, agents, assignees, or organizations.
- Do not infer effort from time online, token spend, commit count, or comment
  volume.
- Prefer evidence confidence over inflated totals.

Every pulse row should carry source refs so an operator can inspect why it was
counted.

## First JSON Shape

The first stable schema should be `pulse-report/v1alpha1`.

Suggested repo-level shape:

```json
{
  "schema_version": "pulse-report/v1alpha1",
  "scope": {
    "kind": "repo",
    "repo": "OWNER/REPO"
  },
  "window": {
    "since": "2026-06-01T00:00:00Z",
    "until": "2026-06-06T10:00:00Z",
    "label": "7d"
  },
  "summary": {
    "finished": 2,
    "reviewed": 1,
    "refined": 3,
    "unblocked": 1,
    "superseded": 1,
    "started": 2,
    "checked": 1
  },
  "health": {
    "ready": 4,
    "review_needed": 1,
    "finish_ready": 0,
    "blocked": 0,
    "failed_check": 0,
    "human_decision": 1
  },
  "items": [
    {
      "kind": "finished",
      "repo": "OWNER/REPO",
      "issue": 12,
      "title": "Harden workspace report bundle",
      "confidence": "high",
      "occurred_at": "2026-06-06T10:27:23Z",
      "evidence": ["merged_pr", "closing_reference", "finish_receipt"],
      "source_refs": ["issue:OWNER/REPO#12", "pr:OWNER/REPO#15"]
    }
  ],
  "warnings": [],
  "privacy_boundary": {
    "scope": "work_item_state_only",
    "prohibited": [
      "personal_productivity_ranking",
      "agent_productivity_ranking",
      "time_online_scoring",
      "token_spend_scoring",
      "leaderboard"
    ]
  }
}
```

Workspace-level pulse can keep the same schema with:

```json
{
  "scope": {
    "kind": "workspace",
    "workspace": "personal",
    "owner": "OWNER",
    "repos": ["OWNER/app", "OWNER/cli"]
  }
}
```

## Command Shape

The first implementation should add a read-only stats command:

```bash
gira stats pulse --repo OWNER/REPO --since 7d --json
gira stats pulse --repo OWNER/REPO --since 7d
```

After the repo contract is stable, add workspace pulse:

```bash
gira workspace pulse --config .gira/config.yaml --since 7d --json
```

The workspace command should reuse workspace repo selection, bounded concurrency,
cache TTL, refresh behavior, and rate-limit diagnostics from `workspace status`.

Do not start with `gira stats momentum`. `momentum` is a useful product category,
but the first operator command should be the simpler current-period report.

## Report Bundle Placement

The local dashboard export should include pulse only after the CLI JSON contract
is stable.

Recommended artifacts:

```text
out/dashboard/
  derived/
    workspace_pulse.json
  csv/
    workspace_pulse_items.csv
```

`derived/workspace_dashboard.json` can then include a compact `pulse` section:

```json
{
  "pulse": {
    "schema_version": "pulse-report/v1alpha1",
    "path": "derived/workspace_pulse.json",
    "summary": {
      "finished": 2,
      "reviewed": 1,
      "refined": 3,
      "unblocked": 1,
      "superseded": 1,
      "started": 2,
      "checked": 1
    }
  }
}
```

The HTML page should present pulse as an operating summary near queue counts and
top actions. It should not render a rank, badge economy, or global score.

## Implementation Slices

1. Add `gira stats pulse --repo OWNER/REPO --since 7d --json`.
2. Add fixtures and tests for high-confidence finish, supersede, refinement,
   unblocking, and anti-gaming exclusions.
3. Add text output that emphasizes evidence and next attention, not score.
4. Add `gira workspace pulse` once repo pulse is stable.
5. Add dashboard export artifacts and HTML summary.

## Acceptance Mapping

- Vocabulary: `pulse` first, `momentum` as category, avoid `token`.
- Signal model: named evidence groups, not a single productivity score.
- Source contracts: GitHub/Gira evidence first; local exports derived only.
- CLI: first command is `gira stats pulse`.
- Privacy: no ranking, surveillance, token spend, or time-online signals.
- Report bundle: add `workspace_pulse.json` only after CLI schema stabilizes.
- Follow-up: create one implementation ticket for the repo-level pulse slice.
