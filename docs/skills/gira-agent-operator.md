# Gira Agent Operator Skill

This is the canonical agent/operator skill for Gira-managed repositories.
Adapters such as `AGENTS.md`, `CLAUDE.md`, `.github/copilot-instructions.md`,
Cursor rules, `gira guide agent`, and docs-site pages should summarize or point
to this file instead of redefining the operating rules.

## Purpose

Gira turns GitHub into the execution backend for an AI-ready project operating
system. Issues are executable work packets, branches are work-start evidence,
pull requests are change units, milestones are phase boundaries, and merged PRs
plus closed issues are completion evidence.

## Canonical And Adapted Surfaces

- Canonical source: `docs/skills/gira-agent-operator.md`.
- CLI summary: `gira guide agent` and `gira guide skill`.
- Codex/OpenAI adapter: `AGENTS.md`.
- Optional adapters: `CLAUDE.md`, `.github/copilot-instructions.md`, and
  `.cursor/rules/gira.mdc`.
- Human documentation: docs-site agent operator skill page.

Adapters may add surface-specific wording, but they should not change the
lifecycle, safety, or evidence rules defined here.

## Operating Model

- GitHub Issues are executable work packets.
- A branch is evidence that work has started for one issue.
- A pull request is the unit of code review and change delivery.
- A merged PR plus a closed linked issue is completion evidence.
- Milestones group bounded phases such as sprints, releases, or MVP slices.
- GitHub Projects are visibility surfaces. Project-only items must be routed to
  repository issues before implementation starts.
- GitHub labels, milestones, issues, and PRs remain the source of truth.

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
   - Prefer `gira ticket start TICKET --repo OWNER/REPO --dry-run`.
   - Apply only after the dry-run is understood: `gira ticket start TICKET --repo OWNER/REPO --apply`.
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

- `gira ticket new "Title" --dry-run|--apply [--body TEXT|--body-file PATH|-] [--start]`: Create a repo-bound executable GitHub issue with structured or full Markdown body input.
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
- `gira goal handoff [GOAL] [--repo OWNER/REPO] [--role implementer] [--profile default] [--json]`: Build a goal-level LLM handoff that includes goal context and the next safe child ticket worker packet.
- `gira goal new "Title" --dry-run|--apply [--repo OWNER/REPO] [--objective TEXT] [--scope TEXT] [--json]`: Create a Goal Mode issue with objective, scope, autonomy, quality, stop, and child-ticket planning sections.
- `gira goal next [GOAL] [--repo OWNER/REPO] [--json]`: Select the next safe child ticket for a goal or explain why work must stop.
- `gira goal plan [GOAL] --dry-run|--apply [--repo OWNER/REPO] [--json]`: Propose or create same-repo or target-repo child ticket packets from a goal issue.
- `gira goal report [GOAL] [--repo OWNER/REPO] [--json|--html --output PATH]`: Build a visible report for one goal from stable Goal Mode state. Alias: gira goal dossier.
- `gira goal status [GOAL] [--repo OWNER/REPO] [--json]`: Summarize a goal issue, child ticket graph, blockers, and next safe action.
- `gira ops limit [--repo OWNER/REPO] [--workflow NAME] [--json]`: Show GitHub REST, GraphQL, search, secondary-limit, and workflow budget diagnostics.
- `gira pm qa --repo OWNER/REPO --ticket N [--pr N] [--diff-summary] [--include-diff] [--json]`: Render a PM acceptance QA prompt from task-local PM state and PR evidence.
- `gira pm spec [--title TITLE] [--repo OWNER/REPO] [--intent TEXT|--from-file PATH|-] [--worker-mode plan] [--json]`: Render a durable PM state and worker-ready task packet from raw intent.
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
- Incomplete acceptance criteria: ask for clarification or update the issue
  before implementing broad inferred behavior.
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
