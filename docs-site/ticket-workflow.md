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
gira ticket handoff --json
gira ticket review --diff-summary
gira ticket self-review --diff-summary --dry-run
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
gira ticket review --diff-summary
gira ticket note "Ready for review." --target pr --dry-run
gira ticket finish --apply
```

Use `gira ticket new --body`, `--body-file PATH`, or `--body-file -` when the issue packet is already drafted in Markdown. Dry-run output includes the title, repo, labels, milestone, start intent, body that will be sent to GitHub, and a `ticket-readiness/v1` work-order check. `ticket new` does not create taxonomy labels implicitly; requested labels must already exist and are checked during dry-run/apply. Use `gira ticket list` to discover repo issue-backed tickets without dropping to raw `gh`. It supports `--state open|closed|all`, repeatable or comma-separated `--label`, `--assignee`, `--milestone`, `--limit`, and `--json`.
Use `gira milestone new`, `gira milestone list`, `gira milestone status`, `gira milestone assign`, and `gira milestone plan` when a work batch should be created, inspected, or filled before ticket execution starts. Milestone mutation commands use dry-run/apply, and `milestone plan` defaults to selecting `status:ready` tickets.
Use `gira ticket view` instead of composing `gh issue view` and `gh pr view` when you need the ticket operating card. Use `gira ticket note --dry-run` before `--apply` when you need a progress, blocker, decision, handoff, summary, or check note on the issue or linked PR.
Use `gira ticket prompt TICKET planner|implementer|reviewer --profile default|python` when a stateless agent needs a deterministic handoff prompt from the GitHub issue. `--role planner|implementer|reviewer` is still accepted for scripts. Each prompt includes a structured role packet with readiness, work-order, risk, expected-evidence, and repo-guidance fields relevant to that worker. Use `gira ticket handoff --json` from an `issue-N-*` branch when a worker adapter needs the machine contract without making Gira launch or host the worker; pass `TICKET` only when operating outside that branch context. The handoff packet uses schema `worker-handoff/v1` and includes the ticket readiness result, work order, branch policy/base context, expected evidence, required checks, review expectations, prohibited actions, telemetry/provenance expectations, repo guidance, and the next safe command. The [worker boundary](/worker-boundary) explains why exact tools, models, trace IDs, and attempt IDs belong in telemetry/provenance metadata rather than high-cardinality labels. Use `gira ticket review --diff-summary` when a reviewer needs the current ticket, linked PR, checks, changed files, PR readiness, finish readiness, and evidence fields in one packet. The diff summary includes changed files, diff stat, hunk headers, acceptance mapping candidates, risk hints, and the full diff command; `--include-diff` includes the full diff only when explicitly requested. Use `gira ticket self-review --diff-summary --dry-run` to preview the `kind=check` PR note for the current branch ticket, then `--apply` to post it. Reviewer prompts are read-only briefs: they tell the reviewer to inspect the actual PR diff, check repo-local instructions, and account for AI Delivery Telemetry, Gira label/workflow conventions, tool contracts, and tests required by the changed surface. Reviewer prompts and packets can include explicit PR context with `--pr N`; `--json` includes the rendered prompt, `pr-readiness/v1`, and structured role/review contract for automation.
Use `gira ticket supersede TICKET --replacement-title "TITLE" --body-file replacement.md --dry-run` when an issue should be replaced instead of manually closing the old issue and creating cross-links. Superseded tickets are closed with `resolution:superseded`, not `status:done`, so lifecycle reports can separate replaced work from completed work.
Use `gira epic list` for the same discovery pattern scoped to `type:epic` issues.

The target [branch policy contract](/branch-policy) defines how future lifecycle
commands should resolve a base branch once, record it as ticket state, preserve
it through PR creation and review, and avoid hidden local checkout mutation
during finish.

`gira ticket status --json` is the stable per-ticket state contract for automation. It preserves the compact legacy fields (`repo`, `issue`, `status`, `pr_number`, `blockers`, `next_action`, `next_step`) and adds `schema_version`, `labels`, `milestone`, `branch`, `pull_request`, `checks_status`, `checks`, `review_status`, `evidence`, `ticket_readiness`, `pr_readiness`, and `warnings`. The `ticket_readiness` object uses schema `ticket-readiness/v1` and tells workers whether the issue is ready to start, needs refinement, is blocked, or needs human input before handoff. The `pr_readiness` object uses schema `pr-readiness/v1` and tells reviewers or worker adapters whether the linked PR is ready for review, needs revision, should wait for checks, is ready for finish, or is blocked. Missing linked PR, unknown branch, and missing checks are represented as explicit values so queue, review, finish, and audit commands can consume the same state without guessing from terminal text.

`pr-readiness/v1` is narrower than `finish-readiness/v1`. PR readiness checks whether worker output is reviewable or needs revision: closing link, recorded base, draft state, checks, review blocker state, telemetry/provenance warnings, acceptance evidence, and changed-file context when review packets can fetch it. Finish readiness remains the final merge/closure gate and uses GitHub evidence to decide whether `ticket finish` may merge, normalize labels, post a receipt, and close the loop.

`gira ticket finish --dry-run --json` includes a `readiness` report with schema `finish-readiness/v1`. The report groups issue state, linked PR state, checks, review status, label state, closing-reference evidence, acceptance-checklist counts, blockers, warnings, and the next safe action before any merge or provider mutation is attempted. Human output stays compact and shows `readiness=ready|blocked|unknown` beside the existing blockers and action plan.

Finish also builds a `receipt` with schema `finish-receipt/v1`. Dry-run previews the concise issue comment; apply posts it after successful completion. The receipt records final issue/PR state, check and review summaries, evidence sources, label normalization, warnings, and AI Delivery Telemetry status. Agent-routed work such as `agent:worker`, `agent:codex`, `agent:gira`, `agent:reviewer`, `lane:agent`, or `lane:hybrid` expects an AI Delivery Telemetry or Gira provenance block; missing telemetry is reported as a warning in the receipt rather than expanded into a raw log dump.

Use `gira audit drift --repo OWNER/REPO --json` when you need a repo-local convergence report without mutating anything. It is the drift-focused alias for the workflow audit and detects stale status labels, open `status:done` issues, multiple `status:*` labels, in-review issues without linked PRs, merged PRs whose issues did not converge, failed or pending checks on finished tickets, and missing AI Delivery Telemetry or completion evidence. Each finding includes severity, kind, current state, expected state, evidence, and a recommended manual action.

When [Jira-primary provider mode](/jira-primary-provider) is enabled, `gira ticket finish` also gates Jira Done on GitHub execution evidence. It refuses Done while the mirror issue, linked PR, review, checks, merge, or close evidence is incomplete.

## Agent Rules

- Start from a GitHub issue.
- Use a feature branch per issue.
- Use Gira ticket lifecycle commands for view, status, start, PR, note, supersede, checks, wait, and finish work; use raw `gh` only when Gira has no lifecycle command or you intentionally need a low-level GitHub operation.
- Keep PR bodies linked with `Closes #N`, `Fixes #N`, or `Resolves #N`.
- Treat Project-only items as intake until they are backed by repository issues.
