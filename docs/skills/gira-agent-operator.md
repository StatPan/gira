# Gira Agent Operator Skill

Canonical operator skill; adapters point here.

## Purpose

Gira uses GitHub: issues are work packets, branches start evidence, PRs are
change units, and milestones phase boundaries.

## Canonical Surfaces

- PM: `docs/pm-operating-policy.md`, `docs/pm-skill.md`.
- CLI: `gira guide agent`; Codex adapter: `AGENTS.md`.

## Operating Model

- A branch starts one issue; its PR is the review and delivery unit; merged PR plus closed issue is completion evidence.
- Milestones group phases; project-only items become repository issues.

## Agent Entry Points

- One issue: `ticket handoff`; Goal: `dispatch goal`; Goal/queue/PM: advanced.

## Standard Agent Flow

1. Inspect state.
   - Run `gh auth status`.
   - Run `gira status --repo OWNER/REPO` or inspect the specific issue.
   - Confirm the issue exists, is open, and is ready for implementation.
   - Treat `ticket-readiness/v1` findings from `gira ticket status --json`
     or `gira ticket new --dry-run` as the work-order gate before worker
     handoff. Refine tickets with missing goal, scope, acceptance criteria,
     required labels, doctor impact, or evidence expectations before starting
     implementation.
2. Start ticket.
   - Choose `--create`, `--current`, or `--adopt BRANCH`; dry-run first.
   - Apply only the previewed strategy.
   - Use one feature branch per issue.
   - After checkout, prefer current-branch commands without ticket or PR
     numbers. Use explicit `--ticket`, `--pr`, and `--repo` only for detached
     context, ambiguous branch state, other-person PR review, or historical
     audit.
3. Implement bounded scope.
   - Keep changes limited to the issue goal and acceptance criteria.
   - For feature or workflow changes, record the intended doctor impact in the
     issue or PR: new check, existing check update, or explicit no-op.
   - Do not revert user changes or unrelated local work.
   - Run the relevant local tests and checks.
4. Open or validate PR.
   - Prefer `gira ticket pr --dry-run` from the ticket branch.
   - Apply with `gira ticket pr --apply`.
   - The PR body must contain `Closes #N`, `Fixes #N`, or `Resolves #N`
     unless the issue is intentionally kept open.
   - On `pr_base_mismatch`, use status guidance to deliberately retarget the PR.
5. Check and wait.
   - Prefer `gira ticket review --diff-summary`.
   - Prefer `gira ticket checks`.
   - Prefer `gira ticket wait --timeout 5m`.
   - Investigate failed checks before requesting finish.
   - Treat `pr-readiness/v1` from `gira ticket status --json` or
     `gira ticket review --diff-summary --json` as the PR handoff gate. Revise PRs with
     missing closing links, base mismatches, draft state, failed checks,
     review blockers, or required telemetry gaps before finish.
   - For review handoff, use `gira ticket review` or
     `gira ticket prompt --role reviewer`; the reviewer prompt is read-only,
     points to the actual PR diff, and reminds reviewers to check repo-local
     agent instructions, Gira workflow conventions, tool contracts, telemetry,
     and changed-surface tests.
   - For author or agent self-check evidence, use
     `gira ticket self-review --diff-summary --dry-run`, then `--apply` after
     the rendered PR check note is reviewed. This does not replace required
     human or branch-protection review.
6. Finish.
   - Prefer `gira ticket finish --dry-run`.
   - Apply only after the dry-run is clean: `gira ticket finish --apply`.
   - Completion requires a merged linked PR and the issue closed by GitHub or
     Gira lifecycle handling.

<!-- gira:agent-skill:start -->
## Registry-Backed Lifecycle Command Guidance

This generated section contains command facts for the agent lifecycle. Update `internal/gira/command_registry.go` first, then refresh this block.

- `gira ticket new "Title" --dry-run|--apply [--parent N] [--body TEXT|--body-file PATH|-] [--release-impact MODE] [--start]`: Create a repo-bound executable GitHub issue with structured or full Markdown body input.
- `gira ticket parent TICKET [--set PARENT|--clear] [--dry-run|--apply] [--repo OWNER/REPO] [--json]`: Show, set, or clear a native GitHub sub-issue parent without adding a separate link command family.
- `gira ticket view|show [TICKET] [--repo OWNER/REPO] [--json]`: Show a Gira operating card for the ticket, linked PR, blockers, and next action. Alias: gira ticket show.
- `gira ticket prompt [TICKET] [planner|implementer|reviewer] [--role planner|implementer|reviewer] [--profile default|python] [--repo OWNER/REPO] [--pr N] [--json]`: Render a stateless planner, implementer, or reviewer prompt from ticket context.
- `gira ticket handoff [TICKET] [planner|implementer|reviewer] [--role planner|implementer|reviewer] [--profile default|python] [--repo OWNER/REPO] [--json]`: Compile a worker-neutral handoff packet from ticket context.
- `gira ticket review [TICKET] [--repo OWNER/REPO] [--pr N] [--diff-summary] [--include-diff] [--json|--html --output PATH]`: Render a reviewer packet from current ticket and linked PR state.
- `gira ticket start [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--base BRANCH] [--create|--current|--adopt BRANCH]`: Start a ready issue with an explicit branch strategy.
- `gira ticket pr [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--draft]`: Create or validate a linked PR with required issue closing text.
- `gira ticket self-review [TICKET] [--repo OWNER/REPO] [--pr N] [--diff-summary] --dry-run|--apply [--json]`: Post a self-review check note for the current branch ticket and linked PR.
- `gira ticket note [TICKET] "BODY" --dry-run|--apply [--repo OWNER/REPO] [--kind progress|blocker|decision|handoff|summary|check] [--target auto|issue|pr|both]`: Post a structured context note to the issue, linked PR, or both.
- `gira ticket supersede [TICKET] --replacement-title TITLE --body-file PATH|- --dry-run|--apply [--repo OWNER/REPO] [--close-draft-pr]`: Close a ticket as superseded and create a linked replacement ticket.
- `gira ticket checks [TICKET] [--repo OWNER/REPO] [--json]`: Show linked PR checks, review blockers, and next action.
- `gira ticket wait [TICKET] [--repo OWNER/REPO] [--timeout 5m] [--interval 5s]`: Wait for pending linked PR checks without merging.
- `gira ticket finish [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--sync-local]`: Merge the linked PR when policy allows; Draft PRs stop after ready transition and require a new finish preview.
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

<!-- gira:agent-skill:end -->


## Safety Rules

- Use `--dry-run` before `--apply` for mutating Gira operations.
- Prefer Gira lifecycle commands over raw `gh` whenever Gira has a command for
  the operation.
- Keep implementation scope bounded to the target issue.
- Preserve user changes and unrelated work in the checkout.
- Do not rotate secrets, edit credentials, delete repositories, or make broad
  GitHub settings changes without explicit human approval.
- Do not merge or finish work with failed checks unless explicitly instructed
  and the risk is documented.

## When Raw `gh` Is Allowed

Raw `gh` is allowed when Gira does not yet provide the needed lifecycle command,
or when reading GitHub state that Gira does not expose. Common allowed uses:

- `gh auth status` for authentication checks.
- `gh issue view` or `gh issue list` for extra issue context.
- `gh pr view` for PR details not shown by Gira.
- `gh run view` or `gh run watch` for workflow diagnostics after
  `gira ticket checks` identifies a failing or missing check.
- Explicit repo adoption or metadata work when Gira prints a `gh` remediation
  and no safer Gira command exists.

Raw `gh` should not bypass available Gira lifecycle commands for start, PR
creation, check/wait, or finish.

Provider changes made through raw `gh`, web UI, provider APIs, future provider
CLIs, or external automation are treated as external drift unless Gira can
reconstruct enough evidence to classify them. See
[External Drift And Provenance Policy](../external-drift-policy.md) for the
provenance levels and receipt expectations.

## Edge Cases

- Project-only item: do not implement directly. Route it to a repository issue
  first, then start the issue.
- Missing `status:ready`: do not start. Use Gira adoption or triage flow to add
  the correct status after confirming the issue is executable.
- Existing branch: inspect whether the branch belongs to the same issue. Reuse
  it only when it is clearly the issue branch and local changes are safe.
- Existing PR: validate linkage and checks instead of opening a duplicate PR.
- Failed checks: diagnose and fix within the issue scope, then rerun or wait.
- Incomplete acceptance criteria: follow the PM policy; classify and split safe
  work, then ask only for the residual decision.
- Ambiguous repo or ticket context: pass `--repo OWNER/REPO` and the ticket
  number explicitly.

## Drift Prevention

- Keep this file as the source of truth for lifecycle and safety rules.
- Keep adapter files short and refer back to this file.
- Keep command summaries, usage, examples, and guide ordering in
  `internal/gira/command_registry.go`; `gira guide agent`, `gira guide skill`,
  `gira guide ticket`, and docs-site command surfaces render from that
  registry.
- Keep exactly one registry-backed section in this skill between
  `<!-- gira:agent-skill:start -->` and `<!-- gira:agent-skill:end -->`.
