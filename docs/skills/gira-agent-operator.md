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
2. Start ticket.
   - Prefer `gira ticket start TICKET --repo OWNER/REPO --dry-run`.
   - Apply only after the dry-run is understood: `gira ticket start TICKET --repo OWNER/REPO --apply`.
   - Use one feature branch per issue.
3. Implement bounded scope.
   - Keep changes limited to the issue goal and acceptance criteria.
   - Do not revert user changes or unrelated local work.
   - Run the relevant local tests and checks.
4. Open or validate PR.
   - Prefer `gira ticket pr TICKET --repo OWNER/REPO --dry-run`.
   - Apply with `gira ticket pr TICKET --repo OWNER/REPO --apply`.
   - The PR body must contain `Closes #N`, `Fixes #N`, or `Resolves #N`
     unless the issue is intentionally kept open.
5. Check and wait.
   - Prefer `gira ticket checks TICKET --repo OWNER/REPO`.
   - Prefer `gira ticket wait TICKET --repo OWNER/REPO --timeout 5m`.
   - Investigate failed checks before requesting finish.
6. Finish.
   - Prefer `gira ticket finish TICKET --repo OWNER/REPO --dry-run`.
   - Apply only after the dry-run is clean: `gira ticket finish TICKET --repo OWNER/REPO --apply`.
   - Completion requires a merged linked PR and the issue closed by GitHub or
     Gira lifecycle handling.

<!-- gira:agent-skill:start -->
## Registry-Backed Lifecycle Command Guidance

This generated section contains command facts for the agent lifecycle. Update `internal/gira/command_registry.go` first, then refresh this block.

- `gira ticket new "Title" --dry-run|--apply [--body TEXT|--body-file PATH|-] [--start]`: Create a repo-bound executable GitHub issue with structured or full Markdown body input.
- `gira ticket view [TICKET] [--repo OWNER/REPO] [--json]`: Show a Gira operating card for the ticket, linked PR, blockers, and next action.
- `gira ticket start [TICKET] --dry-run|--apply [--repo OWNER/REPO]`: Verify a ready issue, create or reuse its branch, and move it to in-progress.
- `gira ticket pr [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--draft]`: Create or validate a linked PR with required issue closing text.
- `gira ticket note [TICKET] "BODY" --dry-run|--apply [--repo OWNER/REPO] [--kind progress|blocker|decision|handoff|summary|check] [--target auto|issue|pr|both]`: Post a structured context note to the issue, linked PR, or both.
- `gira ticket supersede [TICKET] --replacement-title TITLE --body-file PATH|- --dry-run|--apply [--repo OWNER/REPO] [--close-draft-pr]`: Close a ticket as superseded and create a linked replacement ticket.
- `gira ticket checks [TICKET] [--repo OWNER/REPO] [--json]`: Show linked PR checks, review blockers, and next action.
- `gira ticket wait [TICKET] [--repo OWNER/REPO] [--timeout 5m] [--interval 5s]`: Wait for pending linked PR checks without merging.
- `gira ticket finish [TICKET] --dry-run|--apply [--repo OWNER/REPO]`: Merge the linked PR when policy allows, sync main, and close the ticket loop.
- `gira ticket status [TICKET] [--repo OWNER/REPO] [--json]`: Report ticket status, linked PR blockers, and next action.

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
- Keep the registry-backed section in this skill inside
  `<!-- gira:agent-skill:start -->` and `<!-- gira:agent-skill:end -->`; tests
  enforce that block stays in sync while the surrounding policy text remains
  human-owned.
- Add or update CLI/docs tests when changing required lifecycle wording.
- Prefer managed blocks for generated adapters so Gira can refresh summaries
  without overwriting human-owned content.
