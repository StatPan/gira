# Agent Operator Skill

The canonical Gira agent/operator skill lives in
[`docs/skills/gira-agent-operator.md`](https://github.com/StatPan/gira/blob/main/docs/skills/gira-agent-operator.md).

This docs-site page is a thin copy for public navigation. Keep lifecycle,
safety, and evidence policy in the canonical skill, then refresh this page from
the shared docs contract renderer.

Use it as the source of truth for coding agents operating Gira-managed
repositories. PM task-packet rules live in
[`docs/pm-skill.md`](https://github.com/StatPan/gira/blob/main/docs/pm-skill.md).
Existing adapters such as `AGENTS.md`, generated summaries such as
`gira guide agent`, and future optional adapter paths should summarize the
canonical sources instead of redefining them.

## Operating Model

- GitHub Issues are executable work packets.
- Branches are work-start evidence.
- PRs are change units.
- Merged PR plus closed issue is completion evidence.

## Registry-Backed Lifecycle Commands

- `gira ticket new "Title" --dry-run|--apply [--parent N] [--body TEXT|--body-file PATH|-] [--start]`: Create a repo-bound executable GitHub issue with structured or full Markdown body input.
- `gira ticket parent TICKET [--set PARENT|--clear] [--dry-run|--apply] [--repo OWNER/REPO] [--json]`: Show, set, or clear a native GitHub sub-issue parent without adding a separate link command family.
- `gira ticket view|show [TICKET] [--repo OWNER/REPO] [--json]`: Show a Gira operating card for the ticket, linked PR, blockers, and next action. Alias: gira ticket show.
- `gira ticket prompt [TICKET] [planner|implementer|reviewer] [--role planner|implementer|reviewer] [--profile default|python] [--repo OWNER/REPO] [--pr N] [--json]`: Render a stateless planner, implementer, or reviewer prompt from ticket context.
- `gira ticket handoff [TICKET] [planner|implementer|reviewer] [--role planner|implementer|reviewer] [--profile default|python] [--repo OWNER/REPO] [--json]`: Compile a worker-neutral handoff packet from ticket context.
- `gira ticket review [TICKET] [--repo OWNER/REPO] [--pr N] [--diff-summary] [--include-diff] [--json|--html --output PATH]`: Render a reviewer packet from current ticket and linked PR state.
- `gira ticket start [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--base BRANCH]`: Verify a ready issue, create or reuse its branch, and move it to in-progress.
- `gira ticket pr [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--draft]`: Create or validate a linked PR with required issue closing text.
- `gira ticket self-review [TICKET] [--repo OWNER/REPO] [--pr N] [--diff-summary] --dry-run|--apply [--json]`: Post a self-review check note for the current branch ticket and linked PR.
- `gira ticket note [TICKET] "BODY" --dry-run|--apply [--repo OWNER/REPO] [--kind progress|blocker|decision|handoff|summary|check] [--target auto|issue|pr|both]`: Post a structured context note to the issue, linked PR, or both.
- `gira ticket supersede [TICKET] --replacement-title TITLE --body-file PATH|- --dry-run|--apply [--repo OWNER/REPO] [--close-draft-pr]`: Close a ticket as superseded and create a linked replacement ticket.
- `gira ticket checks [TICKET] [--repo OWNER/REPO] [--json]`: Show linked PR checks, review blockers, and next action.
- `gira ticket wait [TICKET] [--repo OWNER/REPO] [--timeout 5m] [--interval 5s]`: Wait for pending linked PR checks without merging.
- `gira ticket finish [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--sync-local]`: Merge the linked PR when policy allows and close the ticket loop without local checkout sync by default.
- `gira ticket status [TICKET] [--repo OWNER/REPO] [--json|--html --output PATH]`: Report ticket status, linked PR blockers, and next action.
- `gira config storage [--repo OWNER/REPO] [--config-root PATH] [--json]`: Show local storage roots, durability, privacy, and rebuild boundaries.
- `gira dispatch goal [GOAL] [--repo OWNER/REPO] [--role implementer] [--profile default] [--json|--compact-json|--prompt]`: Build an official dispatch packet from a goal issue, goal handoff, and next safe child ticket worker handoff.
- `gira feature check [--repo OWNER/REPO] [--limit N] [--json]`: Validate optional feature map records and work links without mutating GitHub.
- `gira feature for ISSUE [--repo OWNER/REPO] [--limit N] [--json]`: Show which feature or capability a work issue is linked to.
- `gira feature list [--repo OWNER/REPO] [--limit N] [--json]`: List optional issue-backed feature or capability records.
- `gira goal finish [GOAL] --dry-run|--apply [--repo OWNER/REPO] [--terminal done|human_review|blocked|superseded|abandoned] [--json]`: Preview goal finish readiness, then post receipts and close ready goals or preserve human-review handoffs.
- `gira goal graph [GOAL] [--dry-run|--apply --expect-plan ID] [--repo OWNER/REPO] [--json|--compact-json]`: Compile PM intent and discovery state into a typed, verifiable Goal work graph.
- `gira goal handoff [GOAL] [--repo OWNER/REPO] [--role implementer] [--profile default] [--json]`: Build a goal-level LLM handoff that includes goal context and the next safe child ticket worker packet.
- `gira goal new "Title" --dry-run|--apply [--repo OWNER/REPO] [--objective TEXT] [--scope TEXT] [--json]`: Create a Goal Mode issue with objective, scope, autonomy, quality, stop, and child-ticket planning sections.
- `gira goal next [GOAL] [--repo OWNER/REPO] [--json]`: Select the next safe child ticket for a goal or explain why work must stop.
- `gira goal plan [GOAL] --dry-run|--apply [--repo OWNER/REPO] [--json|--compact-json] [--expect-plan ID]`: Propose or create same-repo or target-repo child ticket packets from a goal issue.
- `gira goal report [GOAL] [--repo OWNER/REPO] [--view operator|human|ai|stakeholder|audit] [--json|--html --output PATH]`: Build a visible report for one goal from stable Goal Mode state. Alias: gira goal dossier.
- `gira goal status [GOAL] [--repo OWNER/REPO] [--json]`: Summarize a goal issue, child ticket graph, blockers, and next safe action.
- `gira ops limit [--repo OWNER/REPO] [--workflow NAME] [--json]`: Show GitHub REST, GraphQL, search, secondary-limit, and workflow budget diagnostics.
- `gira pm accept --repo OWNER/REPO --ticket N --from-file RESULT.json --dry-run|--apply [--json]`: Validate and persist source-linked delivery acceptance and product outcome validation.
- `gira pm observe --repo OWNER/REPO --ticket N [--json]`: Diagnose product-state changes and order bounded PM actions without mutation.
- `gira pm qa --repo OWNER/REPO --ticket N [--pr N] [--diff-summary] [--include-diff] [--json]`: Render a PM acceptance QA prompt from task-local PM state and PR evidence.
- `gira pm replan --repo OWNER/REPO --ticket N --dry-run|--apply [--expect-plan ID] [--override ACTION --rationale TEXT] [--json]`: Preview or apply fingerprinted, capability-aware Goal graph mutations.
- `gira pm spec [--profile PROFILE] [INPUT] [--json]`: Render a compact profile-aware PM packet.
- `gira queue handoff [--config .gira/config.yaml] [--repo OWNER/REPO] [--ticket N] [--role implementer] [--profile default] [--compact] [--json]`: Select or inspect an agent-ready workspace queue item and embed the worker-handoff/v1 payload.
- `gira queue list [--config .gira/config.yaml] [--repo OWNER/REPO] [--queue ready|review|finish|blocked|failed|human] [--limit N] [--compact] [--json]`: List workspace queue items derived from workspace-queues/v1.
- `gira queue next [--config .gira/config.yaml] [--repo OWNER/REPO] [--role implementer] [--profile default] [--compact] [--json]`: Select the first agent-ready workspace queue item and print handoff and run-start commands.
- `gira queue take [--config .gira/config.yaml] [--repo OWNER/REPO] [--ticket N] [--role implementer] [--profile default] [--compact] --dry-run|--apply [--json]`: Start a handoff-safe queue item through the existing ticket start policy.

## Rules

- Use --dry-run before --apply for mutating Gira operations.
- Prefer Gira commands over raw gh when a Gira command exists.
- PR bodies must contain Closes #N, Fixes #N, or Resolves #N.
- Keep changes bounded to the ticket.
- Route project-only items to repository issues before implementation.
- Do not start work missing status:ready until triaged or adopted.
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
refresh generated managed blocks from the shared renderer, and update
CLI/docs tests whenever lifecycle wording changes.
