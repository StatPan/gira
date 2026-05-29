# Gira 2.0 Release Readiness

Gira 2.0 is the point where the CLI-first product contract is stable enough to
name publicly:

> A GitHub-native control plane that turns human and AI work into reviewable,
> auditable, evidence-backed completion.

This is not a hosted launch, UI launch, provider expansion, or new planning
database. The 2.0 release is a stabilization line around the GitHub execution
loop and the machine-readable contracts that make that loop safe for humans,
agents, and durable adapters.

## Stable 2.0 Contract

The stable 2.0 contract is:

| Area | Stable surface |
| --- | --- |
| Ticket lifecycle | `ticket new`, `ticket start`, `ticket pr`, `ticket status`, `ticket view`, `ticket review`, `ticket checks`, `ticket wait`, `ticket note`, `ticket self-review`, `ticket supersede`, and `ticket finish` over GitHub issue-backed work. |
| Evidence-backed finish | `ticket-readiness/v1`, `pr-readiness/v1`, `finish-readiness/v1`, `finish-receipt/v1`, closing-link validation, checks/review blockers, label normalization, and drift audit evidence. |
| Goal mode | `goal-status/v1`, `goal-next/v1`, `goal-plan/v1`, `goal-finish-readiness/v1`, and idempotent `goal-finish-receipt/v1` human-review handoff over child ticket graphs. |
| Workspace health | `workspace-queues/v1` in `workspace status --json`, including agent-ready, review-needed, finish-ready, blocked, failed-check, and human-decision queues. |
| Agent adapter boundary | Command capability metadata, schema-versioned JSON surfaces, and shared `gira-approval-plan/v1` dry-run approval evidence for matching apply commands. |
| Distribution | One Go-built `gira` binary shipped through GitHub Releases, `install.sh`, npm/bun, PyPI/uv/pipx/pip, and Homebrew wrappers. |

These surfaces are stable enough for daily CLI use, dogfood operation, and
adapter planning. Compatibility still means schema-versioned evolution; it does
not mean every command family has identical JSON coverage.

## Preview Or Hardening After 2.0

These items can continue after the `v2.0.0` tag without changing the 2.0
definition:

- Add JSON contracts or explicit adapter-unsupported markings to remaining
  legacy text-first command families.
- Add `post_apply_verification` fields to apply reports that still require
  command-specific follow-up knowledge.
- Improve human-readable summaries for goal, workspace, and release decisions.
- Continue adoption, docs, package-wrapper, and release smoke-test polish.
- Keep Jira transition planning as `jira-transition-plan/v1` read-only
  evidence unless a future Jira mutation boundary is explicitly designed.

## Out Of Scope For 2.0

These are not blockers for `v2.0.0`:

- Hosted dashboards or server-side background sync.
- Web UI, TUI, chat bots, or hosted agent execution.
- GitLab, Forgejo, Gitea, Notion, Linear, or other provider support.
- Full bidirectional Jira sync or Jira workflow administration.
- LLM PRD-to-issue decomposition.
- SQLite or a Gira-native planning database as the source of truth.
- Backfilling historical finish receipts for completed child tickets that
  predate the receipt contract.

## Current Readiness

As of 2026-05-29, the 2.0 milestone structure in `StatPan/gira` shows the core
work complete:

- `2.0 Alpha - State-Aware Ticket Runtime`: 100% complete.
- `2.0 Beta - Evidence-Based Finish and Audit`: 100% complete.
- `2.0 RC - Goal Mode`: 100% complete.
- `2.0 GA - Workspace Health and Agent Queues`: 100% complete.
- `2.0 Branch Policy Hardening`: 100% complete.

The parent goal #521 has an idempotent `goal-finish-receipt/v1` human-review
handoff. It remains open by design because older child tickets predate finish
receipts and should not receive invented historical evidence.

## Tagging Recommendation

Recommendation: `v2.0.0` is a reasonable next stable tag after this readiness
package is reviewed and merged.

Required before tagging:

1. Maintainer reviews the 2.0 scope and confirms no additional runtime feature
   is required for the public release.
2. Release branch or main has a clean worktree.
3. Verification passes:

   ```bash
   go test ./...
   sh scripts/check-docs-contract.sh .
   git diff --check
   ```

4. `CHANGELOG.md` is finalized by replacing the pending heading date with the
   actual release date.
5. Maintainer explicitly approves the tag/publish operation.

Suggested release command after approval:

```bash
git tag -a v2.0.0 -m "gira v2.0.0"
git push origin v2.0.0
```

Publishing remains owned by the release workflow and package-manager wrappers.
Do not create or push the tag from an agent run without explicit maintainer
approval.
