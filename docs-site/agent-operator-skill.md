# Agent Operator Skill

The canonical Gira agent/operator skill lives in
[`docs/skills/gira-agent-operator.md`](https://github.com/StatPan/gira/blob/main/docs/skills/gira-agent-operator.md).

Use it as the source of truth for coding agents operating Gira-managed
repositories. Adapter files such as `AGENTS.md`, `CLAUDE.md`,
`.github/copilot-instructions.md`, Cursor rules, and `gira guide agent` should
summarize that skill instead of redefining it.

## Operating Model

- GitHub Issues are executable work packets.
- Branches are work-start evidence.
- PRs are change units.
- Merged PR plus closed issue is completion evidence.

## Registry-Backed Lifecycle Commands

- `gira ticket new "Title" --dry-run|--apply [--body TEXT|--body-file PATH|-] [--start]`: Create a repo-bound executable GitHub issue with structured or full Markdown body input.
- `gira ticket view|show [TICKET] [--repo OWNER/REPO] [--json]`: Show a Gira operating card for the ticket, linked PR, blockers, and next action. Alias: gira ticket show.
- `gira ticket prompt [TICKET] [planner|implementer|reviewer] [--role planner|implementer|reviewer] [--profile default|python] [--repo OWNER/REPO] [--pr N] [--json]`: Render a stateless planner, implementer, or reviewer prompt from ticket context.
- `gira ticket handoff [TICKET] [planner|implementer|reviewer] [--role planner|implementer|reviewer] [--profile default|python] [--repo OWNER/REPO] [--json]`: Compile a worker-neutral handoff packet from ticket context.
- `gira ticket review [TICKET] [--repo OWNER/REPO] [--pr N] [--json]`: Render a reviewer packet from current ticket and linked PR state.
- `gira ticket start [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--base BRANCH]`: Verify a ready issue, create or reuse its branch, and move it to in-progress.
- `gira ticket pr [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--draft]`: Create or validate a linked PR with required issue closing text.
- `gira ticket note [TICKET] "BODY" --dry-run|--apply [--repo OWNER/REPO] [--kind progress|blocker|decision|handoff|summary|check] [--target auto|issue|pr|both]`: Post a structured context note to the issue, linked PR, or both.
- `gira ticket supersede [TICKET] --replacement-title TITLE --body-file PATH|- --dry-run|--apply [--repo OWNER/REPO] [--close-draft-pr]`: Close a ticket as superseded and create a linked replacement ticket.
- `gira ticket checks [TICKET] [--repo OWNER/REPO] [--json]`: Show linked PR checks, review blockers, and next action.
- `gira ticket wait [TICKET] [--repo OWNER/REPO] [--timeout 5m] [--interval 5s]`: Wait for pending linked PR checks without merging.
- `gira ticket finish [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--sync-local]`: Merge the linked PR when policy allows and close the ticket loop without local checkout sync by default.
- `gira ticket status [TICKET] [--repo OWNER/REPO] [--json]`: Report ticket status, linked PR blockers, and next action.
- `gira goal next [GOAL] [--repo OWNER/REPO] [--json]`: Select the next safe child ticket for a goal or explain why work must stop.
- `gira goal status [GOAL] [--repo OWNER/REPO] [--json]`: Summarize a goal issue, child ticket graph, blockers, and next safe action.

## Rules

- Use --dry-run before --apply for mutating Gira operations.
- Prefer Gira commands over raw gh when a Gira command exists.
- PR bodies must contain Closes #N, Fixes #N, or Resolves #N.
- Keep changes bounded to the ticket.
- Route project-only items to repository issues before implementation.
- Do not start work missing status:ready until triaged or adopted.
- Reuse an existing branch or PR only when it clearly belongs to the ticket.
- Fix failed checks before finish unless explicitly instructed.
- Ask for clarification when acceptance criteria or repo/ticket context is ambiguous.

## Raw `gh`

- gh auth status.
- Extra read-only issue, PR, or workflow diagnostics not exposed by Gira.
- Operations where Gira has no lifecycle command for the needed action.

## Drift Prevention

Keep the canonical skill as the source of truth. Keep adapter files short,
refresh generated managed blocks from the shared renderer, and update
CLI/docs tests whenever lifecycle wording changes.
