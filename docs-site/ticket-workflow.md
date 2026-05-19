# Ticket Workflow

Use Gira for the issue to branch to PR to merge lifecycle. Raw `gh` remains the backend, not the daily UX.

## New Work

```bash
gira ticket new "TITLE" --goal "GOAL" --acceptance "a;b;c" --apply --start
gira ticket new --title "TITLE" --body-file issue.md --dry-run
gira ticket new --title "TITLE" --body-file - --apply --start < issue.md
gira ticket list --state open --label status:ready --limit 20
gira milestone new "MILESTONE" --dry-run
gira milestone plan "MILESTONE" --label status:ready --dry-run
gira ticket pr --apply --draft
gira ticket view
gira ticket prompt --role implementer --profile python
gira ticket review
gira ticket note "Implementation is ready for CI." --dry-run
gira ticket checks
gira ticket wait --timeout 5m
gira ticket finish --apply
```

## Existing Issue

```bash
gira ticket start 42 --apply
gira ticket pr --apply --draft
gira ticket view
gira ticket note "Ready for review." --target pr --dry-run
gira ticket finish --apply
```

Use `gira ticket new --body`, `--body-file PATH`, or `--body-file -` when the issue packet is already drafted in Markdown. Dry-run output includes the title, repo, labels, milestone, start intent, and body that will be sent to GitHub. `ticket new` does not create taxonomy labels implicitly; requested labels must already exist and are checked during dry-run/apply. Use `gira ticket list` to discover repo issue-backed tickets without dropping to raw `gh`. It supports `--state open|closed|all`, repeatable or comma-separated `--label`, `--assignee`, `--milestone`, `--limit`, and `--json`.
Use `gira milestone new`, `gira milestone list`, `gira milestone status`, `gira milestone assign`, and `gira milestone plan` when a work batch should be created, inspected, or filled before ticket execution starts. Milestone mutation commands use dry-run/apply, and `milestone plan` defaults to selecting `status:ready` tickets.
Use `gira ticket view` instead of composing `gh issue view` and `gh pr view` when you need the ticket operating card. Use `gira ticket note --dry-run` before `--apply` when you need a progress, blocker, decision, handoff, summary, or check note on the issue or linked PR.
Use `gira ticket prompt TICKET planner|implementer|reviewer --profile default|python` when a stateless agent needs a deterministic handoff prompt from the GitHub issue. `--role planner|implementer|reviewer` is still accepted for scripts. Each prompt includes a structured role packet with readiness, work-order, risk, expected-evidence, and repo-guidance fields relevant to that worker. Use `gira ticket review` when a reviewer needs the current ticket, linked PR, checks, changed files, finish readiness, and evidence fields in one packet. Reviewer packets include diff commands instead of duplicating the full diff, repo-local guidance such as `AGENTS.md` when available, and a verdict schema for goal fulfillment, acceptance status, checks, evidence, residual risk, notes, test gaps, follow-ups, and the recommended action. Reviewer prompts are read-only briefs: they tell the reviewer to inspect the actual PR diff, check repo-local instructions, and account for AI Delivery Telemetry, Gira label/workflow conventions, tool contracts, and tests required by the changed surface. Reviewer prompts and packets can include explicit PR context with `--pr N`; `--json` includes the rendered prompt and structured role/review contract for automation.
Use `gira ticket supersede TICKET --replacement-title "TITLE" --body-file replacement.md --dry-run` when an issue should be replaced instead of manually closing the old issue and creating cross-links. Superseded tickets are closed with `resolution:superseded`, not `status:done`, so lifecycle reports can separate replaced work from completed work.
Use `gira epic list` for the same discovery pattern scoped to `type:epic` issues.

`gira ticket status --json` is the stable per-ticket state contract for automation. It preserves the compact legacy fields (`repo`, `issue`, `status`, `pr_number`, `blockers`, `next_action`, `next_step`) and adds `schema_version`, `labels`, `milestone`, `branch`, `pull_request`, `checks_status`, `checks`, `review_status`, `evidence`, and `warnings`. Missing linked PR, unknown branch, and missing checks are represented as explicit values so queue, review, finish, and audit commands can consume the same state without guessing from terminal text.

`gira ticket finish --dry-run --json` includes a `readiness` report with schema `finish-readiness/v1`. The report groups issue state, linked PR state, checks, review status, label state, closing-reference evidence, acceptance-checklist counts, blockers, warnings, and the next safe action before any merge or provider mutation is attempted. Human output stays compact and shows `readiness=ready|blocked|unknown` beside the existing blockers and action plan.

When [Jira-primary provider mode](/jira-primary-provider) is enabled, `gira ticket finish` also gates Jira Done on GitHub execution evidence. It refuses Done while the mirror issue, linked PR, review, checks, merge, or close evidence is incomplete.

## Agent Rules

- Start from a GitHub issue.
- Use a feature branch per issue.
- Use Gira ticket lifecycle commands for view, status, start, PR, note, supersede, checks, wait, and finish work; use raw `gh` only when Gira has no lifecycle command or you intentionally need a low-level GitHub operation.
- Keep PR bodies linked with `Closes #N`, `Fixes #N`, or `Resolves #N`.
- Treat Project-only items as intake until they are backed by repository issues.
