# Agent Operator Skill

The canonical Gira agent/operator skill lives in
[`docs/skills/gira-agent-operator.md`](https://github.com/StatPan/gira/blob/main/docs/skills/gira-agent-operator.md).

This page is the short public route for coding agents. The canonical skill
contains the complete safety and lifecycle contract; PM task-packet rules live
in [`docs/pm-skill.md`](https://github.com/StatPan/gira/blob/main/docs/pm-skill.md).
The command entries below are generated from the registry; the exhaustive
[Command Reference](/command-reference) remains the contract index.

## Operating Model

- GitHub Issues are executable work packets.
- Branches are work-start evidence.
- PRs are change units.
- Merged PR plus closed issue is completion evidence.

## Golden Path

1. Start with `gira ticket handoff TICKET --repo OWNER/REPO --json` for a
   single issue.
2. Preview and apply `gira ticket start` with the automatic branch policy.
3. Implement on the issue branch, then preview the PR, checks, and finish
   gates.
4. Use `gira dispatch goal` only when a Goal must coordinate multiple
   tickets. Goal, queue, and PM commands are advanced orchestration.

## Registry-Backed Entry Points

- `gira ticket handoff [TICKET] [planner|implementer|reviewer] [--role planner|implementer|reviewer] [--profile default|python] [--repo OWNER/REPO] [--json]`: Compile a worker-neutral handoff packet from ticket context.
- `gira ticket start [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--base BRANCH] [--branch auto|new|current|NAME] [--create|--current|--adopt BRANCH]`: Start a ready issue with branch selection.
- `gira ticket pr [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--draft]`: Create or validate a linked PR with required issue closing text.
- `gira ticket checks [TICKET] [--repo OWNER/REPO] [--json]`: Show linked PR checks, review blockers, and next action.
- `gira ticket wait [TICKET] [--repo OWNER/REPO] [--timeout 5m] [--interval 5s]`: Wait for pending linked PR checks without merging.
- `gira ticket finish [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--sync-local]`: Merge the linked PR when policy allows; Draft PRs stop after ready transition and require a new finish preview.
- `gira ticket status [TICKET] [--repo OWNER/REPO] [--json|--html --output PATH]`: Report ticket status, linked PR blockers, and next action.
- `gira dispatch goal [GOAL] [--repo OWNER/REPO] [--role implementer] [--profile default] [--json|--compact-json|--prompt]`: Build an official dispatch packet from a goal issue, goal handoff, and next safe child ticket worker handoff.

## Rules

- Use --dry-run before --apply for mutating Gira operations.
- Prefer Gira commands over raw gh when a Gira command exists.
- PR bodies must contain Closes #N, Fixes #N, or Resolves #N.
- Keep changes bounded to the ticket.
- Route project-only items to repository issues before implementation.
- Only managed-required blocks on missing status:ready; otherwise warn and honor provider/base safety.
- Reuse an existing branch or PR only when it clearly belongs to the ticket.
- Do not merge or finish work with failed checks unless explicitly instructed
  and the risk is documented.
- Ask for clarification when acceptance criteria or repo/ticket context is ambiguous.

## Raw `gh`

- gh auth status.
- Extra read-only issue, PR, or workflow diagnostics not exposed by Gira.
- Operations where Gira has no lifecycle command for the needed action.

## Drift Prevention

Keep the canonical skill as the source of truth. Keep adapter files short,
refresh generated pages from the shared renderer, and update CLI/docs tests
whenever lifecycle wording changes.
