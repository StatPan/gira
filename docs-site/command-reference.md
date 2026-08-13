# Command Reference

This page is generated from Gira's command metadata registry. Update `internal/gira/command_registry.go` first, then refresh this page.

## `completion`

Generate shell completion scripts and cache-first dynamic candidates.

Discovery tier: `supporting`.

Usage:

```bash
gira completion bash|zsh|fish; gira completion candidates repo|ticket|label|milestone
```

Since: `v2.1.0`

Flags:

- `bash`: Print Bash completion script.
- `zsh`: Print Zsh completion script.
- `fish`: Print Fish completion script.
- `candidates`: Print local dynamic candidates from the repo registry and workspace status cache.

Examples:

- Install Bash completion locally

```bash
gira completion bash > ~/.local/share/bash-completion/completions/gira
```

- Preview Fish completion

```bash
gira completion fish
```

- Inspect cached label candidates

```bash
gira completion candidates label --repo OWNER/REPO --prefix status
```

Documented in: `README.md`, `docs-site/command-reference.md`

## `config storage`

Show local storage roots, durability, privacy, and rebuild boundaries.

Discovery tier: `assist`.

Usage:

```bash
gira config storage [--repo OWNER/REPO] [--config-root PATH] [--json]
```

Since: `v2.3.0`

Flags:

- `--repo`: Target repo used to include selected repo registry and repo-local contract paths.
- `--config-root`: Override global config root for diagnostics.
- `--json`: Emit stable config-storage-report/v1 JSON.

Examples:

- Inspect local storage boundaries

```bash
gira config storage --repo OWNER/app --json
```

Documented in: `docs/global-config-registry.md`, `docs/state-model.md`, `docs-site/global-config.md`, `docs-site/command-reference.md`

## `dispatch goal`

Build an official dispatch packet from a goal issue, goal handoff, and next safe child ticket worker handoff.

Discovery tier: `advanced_orchestration`.

Workflow role: `canonical_goal_agent_entry_point`.

Usage:

```bash
gira dispatch goal [GOAL] [--repo OWNER/REPO] [--role implementer] [--profile default] [--json|--compact-json|--prompt]
```

Since: `v2.4.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--goal`: Goal issue number. Can also be numeric positional; inferred when omitted.
- `--role`: Handoff role: planner, implementer, or reviewer. Default: implementer.
- `--profile`: Handoff profile: default or python. Default: default.
- `--json`: Emit stable dispatch-packet/v1 JSON.
- `--compact-json`: Emit compact dispatch-compact/v1 JSON without full issue bodies or role packets.
- `--prompt`: Emit a compact prompt for direct LLM handoff.
- `--context-budget`: Maximum compact context size in characters. Default: 12000.

Examples:

- Build a goal dispatch packet for an implementer

```bash
gira dispatch goal --repo OWNER/backlog --role implementer --json
```

- Build a compact LLM handoff prompt

```bash
gira dispatch goal --repo OWNER/backlog --prompt --context-budget 8000
```

Documented in: `docs/dispatch-operating-model.md`, `docs/dispatch-reflection.md`, `docs/goal-operating-model.md`, `docs-site/command-reference.md`

## `feature check`

Validate optional feature map records and work links without mutating GitHub.

Discovery tier: `managed_delivery`.

Compatibility aliases: `gira feat check`.

Usage:

```bash
gira feature check [--repo OWNER/REPO] [--limit N] [--json]
```

Since: `v1.17.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--limit`: Max issues to inspect. Default: 1000.
- `--json`: Emit stable feature-map-check/v1 JSON.

Examples:

- Check feature map health

```bash
gira feat check --repo OWNER/backlog
```

Documented in: `docs/feature-map.md`, `docs-site/feature-map.md`, `docs-site/command-reference.md`

## `feature for`

Show which feature or capability a work issue is linked to.

Discovery tier: `managed_delivery`.

Compatibility aliases: `gira feat for`.

Usage:

```bash
gira feature for ISSUE [--repo OWNER/REPO] [--limit N] [--json]
```

Since: `v1.17.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--issue`: Work issue number. Can also be numeric positional.
- `--limit`: Max issues to inspect. Default: 1000.
- `--json`: Emit stable feature-map-for/v1 JSON.

Examples:

- Inspect one work issue

```bash
gira feat for 123 --repo OWNER/app
```

Documented in: `docs/feature-map.md`, `docs-site/feature-map.md`, `docs-site/command-reference.md`

## `feature list`

List optional issue-backed feature or capability records.

Discovery tier: `managed_delivery`.

Compatibility aliases: `gira feat list`.

Usage:

```bash
gira feature list [--repo OWNER/REPO] [--limit N] [--json]
```

Since: `v1.17.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--limit`: Max issues to inspect. Default: 1000.
- `--json`: Emit stable feature-map-list/v1 JSON.

Examples:

- List feature records

```bash
gira feat list --repo OWNER/backlog
```

Documented in: `docs/feature-map.md`, `docs-site/feature-map.md`, `docs-site/command-reference.md`

## `goal finish`

Preview goal finish readiness, then post receipts and close ready goals or preserve human-review handoffs.

Discovery tier: `advanced_orchestration`.

Usage:

```bash
gira goal finish [GOAL] --dry-run|--apply [--repo OWNER/REPO] [--terminal done|human_review|blocked|superseded|abandoned] [--json]
```

Since: `v1.17.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--goal`: Goal issue number. Can also be numeric positional; inferred when omitted.
- `--dry-run`: Preview readiness and receipt without mutation.
- `--apply`: Apply an explicit done close or human_review handoff mutation.
- `--terminal`: Explicit terminal recommendation override for apply: done, human_review, blocked, superseded, or abandoned.
- `--json`: Emit stable goal-finish-readiness/v1 JSON.

Examples:

- Preview goal finish evidence

```bash
gira goal finish 521 --repo OWNER/app --dry-run --json
```

Documented in: `docs/goal-operating-model.md`, `docs-site/command-reference.md`

## `goal graph`

Compile PM intent and discovery state into a typed, verifiable Goal work graph.

Discovery tier: `advanced_orchestration`.

Workflow role: `typed_work_graph_planning_engine`.

Usage:

```bash
gira goal graph [GOAL] [--dry-run|--apply --expect-plan ID] [--repo OWNER/REPO] [--json|--compact-json]
```

Since: `v3.0.0`

Flags:

- `--repo`: Target GitHub repo.
- `--goal`: Goal issue number; positional is supported.
- `--dry-run`: Preview fingerprinted lowering without mutation.
- `--apply`: Lower create/supersede actions and post an idempotent receipt.
- `--expect-plan`: Required approved dry-run pm-work-graph fingerprint for apply.
- `--json`: Emit full pm-work-graph-report/v1.
- `--compact-json`: Emit body-free pm-work-graph-compact/v1.

Examples:

- Compile a typed work graph

```bash
gira goal graph 521 --repo OWNER/app --compact-json
```

- Preview lowering

```bash
gira goal graph 521 --repo OWNER/app --dry-run --compact-json
```

- Apply an unchanged plan

```bash
gira goal graph 521 --repo OWNER/app --apply --expect-plan pwg-... --compact-json
```

Documented in: `docs/goal-operating-model.md`, `docs/pm-operating-policy.md`, `docs-site/command-reference.md`

## `goal handoff`

Build a goal-level LLM handoff that includes goal context and the next safe child ticket worker packet.

Discovery tier: `advanced_orchestration`.

Workflow role: `advanced_goal_context_builder`.

Usage:

```bash
gira goal handoff [GOAL] [--repo OWNER/REPO] [--role implementer] [--profile default] [--json]
```

Since: `v2.4.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--goal`: Goal issue number. Can also be numeric positional; inferred when omitted.
- `--role`: Handoff role: planner, implementer, or reviewer. Default: implementer.
- `--profile`: Handoff profile: default or python. Default: default.
- `--json`: Emit stable goal-handoff/v1 JSON with worker-handoff/v1 embedded when a child is selected.

Examples:

- Build an implementer handoff for the next goal child

```bash
gira goal handoff 521 --repo OWNER/app --role implementer --json
```

Documented in: `docs/goal-operating-model.md`, `docs-site/command-reference.md`

## `goal new`

Create a Goal Mode issue with objective, scope, autonomy, quality, stop, and child-ticket planning sections.

Discovery tier: `advanced_orchestration`.

Usage:

```bash
gira goal new "Title" --dry-run|--apply [--repo OWNER/REPO] [--objective TEXT] [--scope TEXT] [--json]
```

Since: `v2.4.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--title`: Goal title. Can also be positional.
- `--objective`: Durable goal outcome. Defaults to the title.
- `--direction`: Strategic guidance, priorities, and tradeoffs.
- `--scope`: Included work, target repos, milestones, and explicit non-goals.
- `--autonomy`: Agent lane and permission policy.
- `--decomposition`: Semicolon-separated child planning notes.
- `--quality-bar`: Semicolon-separated verification, review, docs, or release evidence requirements.
- `--stop-condition`: Semicolon-separated conditions that require human input.
- `--type`: Goal issue type label: epic or goal. Default: epic.
- `--priority`: Priority label: p0, p1, p2, or p3.
- `--label`: Additional existing repo label. Repeatable or comma-separated.
- `--body`: Full goal issue body. Overrides structured fields.
- `--body-file`: Read full goal issue body from a file or - for stdin.
- `--milestone`: Milestone title.
- `--dry-run`: Preview issue payload and labels without mutation.
- `--apply`: Create the goal issue.
- `--json`: Emit stable goal-new-report/v1 JSON.

Examples:

- Preview a new goal

```bash
gira goal new "Ship Goal Mode" --repo OWNER/app --objective "Make goal tracking executable" --scope "CLI goal commands" --decomposition "Add goal new;Update docs" --dry-run --json
```

- Create a reviewed goal issue

```bash
gira goal new "Ship Goal Mode" --repo OWNER/app --body-file goal.md --apply
```

Documented in: `docs/goal-operating-model.md`, `docs-site/command-reference.md`

## `goal next`

Select the next safe child ticket for a goal or explain why work must stop.

Discovery tier: `advanced_orchestration`.

Usage:

```bash
gira goal next [GOAL] [--repo OWNER/REPO] [--json]
```

Since: `v1.17.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--goal`: Goal issue number. Can also be numeric positional; inferred when omitted.
- `--json`: Emit stable goal-next/v1 JSON.

Examples:

- Choose the next goal child

```bash
gira goal next 521 --repo OWNER/app --json
```

Documented in: `docs/goal-operating-model.md`, `docs-site/command-reference.md`

## `goal plan`

Propose or create same-repo or target-repo child ticket packets from a goal issue.

Discovery tier: `advanced_orchestration`.

Workflow role: `legacy_bullet_planning_engine`.

Usage:

```bash
gira goal plan [GOAL] --dry-run|--apply [--repo OWNER/REPO] [--json|--compact-json] [--expect-plan ID]
```

Since: `v1.17.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--goal`: Goal issue number. Can also be numeric positional; inferred when omitted.
- `--dry-run`: Preview proposed child tickets, including target_repo, without mutation.
- `--apply`: Create reviewed child tickets in their target repos from the proposed plan.
- `--compact-json`: Emit compact goal-plan-compact/v1 JSON; compact apply requires --expect-plan from dry-run.
- `--expect-plan`: Required dry-run plan ID for --compact-json --apply.
- `--json`: Emit stable goal-plan/v1 JSON.

Examples:

- Preview child ticket plan

```bash
gira goal plan 521 --repo OWNER/app --dry-run --json
```

- Create planned child tickets

```bash
gira goal plan 521 --repo OWNER/app --apply --json
```

Documented in: `docs/goal-operating-model.md`, `docs-site/command-reference.md`

## `goal report`

Build a visible report for one goal from stable Goal Mode state. Alias: gira goal dossier.

Discovery tier: `advanced_orchestration`.

Compatibility aliases: `gira goal dossier`.

Usage:

```bash
gira goal report [GOAL] [--repo OWNER/REPO] [--view operator|human|ai|stakeholder|audit] [--json|--html --output PATH]
```

Since: `v2.1.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--goal`: Goal issue number. Can also be numeric positional; inferred when omitted.
- `--view`: Derived PM view: operator, human, ai, stakeholder, or audit. Default: operator.
- `--json`: Emit stable goal-dossier/v1 JSON.
- `--html`: Write a static local HTML report.
- `--output`: Output path for --html.

Examples:

- Export a bounded AI PM hydration view

```bash
gira goal report 521 --repo OWNER/app --view ai --json
```

- Write a local goal report page

```bash
gira goal report 521 --repo OWNER/app --html --output out/gira/goal-521.html
```

Documented in: `docs/goal-operating-model.md`, `docs-site/goal-mode.md`, `docs-site/command-reference.md`

## `goal status`

Summarize a goal issue, child ticket graph, blockers, and next safe action.

Discovery tier: `advanced_orchestration`.

Usage:

```bash
gira goal status [GOAL] [--repo OWNER/REPO] [--json]
```

Since: `v1.17.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--goal`: Goal issue number. Can also be numeric positional; inferred when omitted.
- `--json`: Emit stable goal-status/v1 JSON.

Examples:

- Inspect goal graph status

```bash
gira goal status 521 --repo OWNER/app --json
```

Documented in: `docs/goal-operating-model.md`, `docs-site/command-reference.md`

## `jira doctor`

Diagnose Jira-primary provider compatibility without mutating Jira or GitHub.

Discovery tier: `assist`.

Usage:

```bash
gira jira doctor --repo OWNER/REPO [--project KEY] [--api-base URL] [--sample-key JIRA-123] [--config-root PATH] [--json]
```

Since: `v1.13.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--project`: Override the configured Jira project key for diagnostics.
- `--api-base`: Override the configured Jira API base URL.
- `--sample-key`: Representative Jira issue key for transition and required-field diagnostics.
- `--config-root`: Override the global Gira config root.
- `--json`: Emit stable JSON.

Examples:

- Diagnose a configured Jira-primary repo

```bash
gira jira doctor --repo OWNER/app --sample-key ABC-123
```

Documented in: `README.md`, `docs/jira-primary-provider.md`, `docs-site/jira-primary-provider.md`

## `jira export`

Export GitHub issue state into Jira-friendly JSON and CSV artifacts.

Discovery tier: `supporting`.

Usage:

```bash
gira jira export --repo OWNER/REPO --output PATH [--json]
```

Since: `v1.13.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--output`: Output directory for export artifacts.
- `--json`: Emit stable JSON.

Examples:

- Export GitHub issue state

```bash
gira jira export --repo OWNER/app --output out/jira
```

Documented in: `README.md`, `docs/jira-primary-provider.md`, `docs-site/jira-primary-provider.md`

## `jira import`

Import Jira CSV/JSON or read-only Jira API issues into GitHub issues.

Discovery tier: `supporting`.

Usage:

```bash
gira jira import --repo OWNER/REPO --source PATH --dry-run|--apply [--json]
gira jira import --repo OWNER/REPO --api-base URL --project KEY --dry-run|--apply [--json]
```

Since: `v1.13.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--source`: CSV or JSON import source path.
- `--api-base`: Jira API base URL for read-only API import.
- `--project`: Jira project key for read-only API import.
- `--dry-run`: Preview issue creates without mutation.
- `--apply`: Create GitHub issues for non-duplicate Jira items.
- `--json`: Emit stable JSON.

Examples:

- Preview a Jira CSV import

```bash
gira jira import --repo OWNER/app --source jira.csv --dry-run
```

Documented in: `README.md`, `docs/jira-primary-provider.md`, `docs-site/jira-primary-provider.md`

## `jira init`

Discover a Jira project and write reviewed non-secret provider config.

Discovery tier: `supporting`.

Usage:

```bash
gira jira init --repo OWNER/REPO --api-base URL --project KEY --dry-run|--apply [--config-root PATH] [--overwrite] [--json]
```

Since: `v1.13.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--api-base`: Jira site base URL, such as https://example.atlassian.net.
- `--project`: Jira project key to discover.
- `--config-root`: Override the global Gira config root.
- `--overwrite`: Replace an existing providers.jira block after review.
- `--dry-run`: Preview provider discovery and config payload without writing files.
- `--apply`: Write the reviewed non-secret provider config.
- `--json`: Emit stable JSON.

Examples:

- Preview Jira provider setup

```bash
gira jira init --repo OWNER/app --api-base https://example.atlassian.net --project ABC --dry-run
```

Documented in: `README.md`, `docs/jira-primary-provider.md`, `docs-site/jira-primary-provider.md`

## `jira mirror`

Create or reuse a GitHub mirror issue for one Jira key.

Discovery tier: `supporting`.

Usage:

```bash
gira jira mirror JIRA-123 --repo OWNER/REPO --dry-run|--apply [--api-base URL] [--config-root PATH] [--json]
```

Since: `v1.13.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--api-base`: Override the configured Jira API base URL.
- `--config-root`: Override the global Gira config root.
- `--dry-run`: Preview mirror issue creation or reuse.
- `--apply`: Create the GitHub mirror issue when missing.
- `--json`: Emit stable JSON.

Examples:

- Preview one Jira mirror

```bash
gira jira mirror ABC-123 --repo OWNER/app --dry-run
```

Documented in: `README.md`, `docs/jira-primary-provider.md`, `docs-site/jira-primary-provider.md`

## `jira transition`

Plan one Jira status transition without mutation.

Discovery tier: `supporting`.

Usage:

```bash
gira jira transition JIRA-123 --repo OWNER/REPO --to ready|in_progress|review|done --dry-run [--api-base URL] [--config-root PATH] [--json]
```

Since: `v1.13.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--to`: Target Gira status mapped through providers.jira.status_map.
- `--api-base`: Override the configured Jira API base URL.
- `--config-root`: Override the global Gira config root.
- `--dry-run`: Required; transition planning is read-only.
- `--json`: Emit stable JSON.

Examples:

- Inspect whether Done is reachable

```bash
gira jira transition ABC-123 --repo OWNER/app --to done --dry-run
```

Documented in: `README.md`, `docs/jira-primary-provider.md`, `docs-site/jira-primary-provider.md`

## `milestone assign`

Bulk attach selected tickets to a milestone through dry-run/apply.

Discovery tier: `managed_delivery`.

Usage:

```bash
gira milestone assign MILESTONE --tickets 1,2,3 [--repo OWNER/REPO] --dry-run|--apply [--json]
```

Since: `v1.16.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--tickets`: Comma-separated ticket numbers.
- `--dry-run`: Preview assignment.
- `--apply`: Assign selected tickets.
- `--json`: Emit stable JSON.

Examples:

- Preview bulk assignment

```bash
gira milestone assign "2.0 Alpha" --tickets 12,13 --dry-run
```

Documented in: `docs-site/sprint-release.md`, `docs-site/ticket-workflow.md`

## `milestone list`

List GitHub milestones with Gira progress fields.

Discovery tier: `managed_delivery`.

Usage:

```bash
gira milestone list [--repo OWNER/REPO] [--state open|closed|all] [--json]
```

Since: `v1.16.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--state`: Milestone state: open, closed, or all. Default: open.
- `--json`: Emit stable JSON.

Examples:

- List open milestones

```bash
gira milestone list --state open
```

Documented in: `docs-site/sprint-release.md`, `docs-site/ticket-workflow.md`

## `milestone new`

Preview and create a GitHub milestone as a first-class Gira work batch.

Discovery tier: `managed_delivery`.

Usage:

```bash
gira milestone new "TITLE" [--repo OWNER/REPO] [--description TEXT] [--due-on YYYY-MM-DD] --dry-run|--apply [--json]
```

Since: `v1.16.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--description`: Milestone description.
- `--due-on`: Milestone due date or timestamp.
- `--dry-run`: Preview milestone creation.
- `--apply`: Create the milestone.
- `--json`: Emit stable JSON.

Examples:

- Preview a milestone

```bash
gira milestone new "2.0 Alpha - State-Aware Ticket Runtime" --dry-run
```

Documented in: `docs-site/sprint-release.md`, `docs-site/ticket-workflow.md`

## `milestone plan`

Select candidate tickets by labels and assign them to a milestone.

Discovery tier: `managed_delivery`.

Usage:

```bash
gira milestone plan MILESTONE [--repo OWNER/REPO] [--label LABEL] [--state open|closed|all] [--limit N] --dry-run|--apply [--json]
```

Since: `v1.16.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--label`: Candidate label filter. Defaults to status:ready.
- `--state`: Ticket state: open, closed, or all. Default: open.
- `--limit`: Maximum candidate tickets. Default: 20.
- `--dry-run`: Preview assignment plan.
- `--apply`: Assign selected tickets.
- `--json`: Emit stable JSON.

Examples:

- Plan from ready tickets

```bash
gira milestone plan "2.0 Alpha" --label status:ready --dry-run
```

Documented in: `docs-site/sprint-release.md`, `docs-site/ticket-workflow.md`

## `milestone status`

Summarize child ticket state for one milestone work batch.

Discovery tier: `managed_delivery`.

Usage:

```bash
gira milestone status MILESTONE [--repo OWNER/REPO] [--json]
```

Since: `v1.16.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--json`: Emit stable JSON.

Examples:

- Inspect a milestone

```bash
gira milestone status "2.0 Alpha - State-Aware Ticket Runtime"
```

Documented in: `docs-site/sprint-release.md`, `docs-site/ticket-workflow.md`

## `ops limit`

Show GitHub REST, GraphQL, search, secondary-limit, and workflow budget diagnostics.

Discovery tier: `supporting`.

Usage:

```bash
gira ops limit [--repo OWNER/REPO] [--workflow NAME] [--json]
```

Since: `v2.6.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--workflow`: Estimate safe remaining runs for a static workflow cost profile.
- `--json`: Emit stable api-limit-report/v1 JSON.

Examples:

- Inspect current GitHub API budget

```bash
gira ops limit --repo OWNER/app
```

- Estimate remaining ticket lifecycle runs

```bash
gira ops limit --repo OWNER/app --workflow ticket-lifecycle
```

- Emit machine-readable budget diagnostics

```bash
gira ops limit --repo OWNER/app --json
```

Documented in: `docs/github-api-limits.md`, `docs/workflow-cost-profiles.md`, `docs/command-surface-boundary.md`, `docs-site/api-limits.md`, `docs-site/cost-profiles.md`, `docs-site/command-surface.md`, `docs-site/command-reference.md`

## `pm accept`

Validate and persist source-linked delivery acceptance and product outcome validation.

Discovery tier: `advanced_orchestration`.

Usage:

```bash
gira pm accept --repo OWNER/REPO --ticket N --from-file RESULT.json --dry-run|--apply [--json]
```

Since: `v3.0.0`

Flags:

- `--repo`: Target GitHub repo.
- `--ticket`: Issue receiving the acceptance result and learning transition.
- `--from-file`: pm-acceptance-result/v1 JSON path, or - for stdin.
- `--dry-run`: Validate evidence mappings and transitions without persistence.
- `--apply`: Persist the verdict and typed ledger transition idempotently.
- `--json`: Emit stable pm-acceptance-report/v1 JSON.

Examples:

- Validate a PM verdict

```bash
gira pm accept --repo OWNER/app --ticket 123 --from-file acceptance.json --dry-run --json
```

- Persist verdict and learning transition

```bash
gira pm accept --repo OWNER/app --ticket 123 --from-file acceptance.json --apply
```

Documented in: `docs/pm-operating-policy.md`, `docs/pm-skill.md`, `docs-site/command-reference.md`

## `pm bootstrap`

Hydrate a bounded, resumable PM protocol session from canonical Goal state.

Discovery tier: `advanced_orchestration`.

Usage:

```bash
gira pm bootstrap --repo OWNER/REPO --ticket N [--role human|ai] [--authority CAPABILITY] [--context-budget N] [--json]
```

Since: `v3.0.0`

Flags:

- `--repo`: Target GitHub repo.
- `--ticket`: Goal issue holding canonical PM state.
- `--role`: Caller role: human or ai. Default: human.
- `--authority`: Explicit capability evidence; repeatable.
- `--context-budget`: Maximum bootstrap characters. Default: 6000.
- `--json`: Emit stable pm-bootstrap/v1 JSON.

Examples:

- Resume an AI PM session without thread memory

```bash
gira pm bootstrap --repo OWNER/app --ticket 123 --role ai --authority issue:read --json
```

Documented in: `docs/pm-operating-policy.md`, `docs/v3-pm-harness-release-readiness.md`, `docs-site/command-reference.md`

## `pm compile`

Compile product intent into deterministic pm-ir/v1 and actionable diagnostics.

Discovery tier: `advanced_orchestration`.

Usage:

```bash
gira pm compile [--intent TEXT|--from-file PATH|-] [--repo OWNER/REPO] [--goal N] [--json]
```

Since: `v3.0.0`

Flags:

- `--intent`: Raw product/development intent.
- `--from-file`: Read raw intent from file, or '-' for stdin.
- `--repo`: Optional target GitHub repo in OWNER/REPO format; required with --goal.
- `--goal`: Optional GitHub Goal issue supplying explicit PM context.
- `--json`: Emit the full stable pm-compile-report/v1 with pm-ir/v1 embedded.

Examples:

- Compile local intent into compact diagnostics

```bash
gira pm compile --from-file request.md
```

- Include explicit Goal context and emit full IR

```bash
gira pm compile --repo OWNER/app --goal 123 --from-file request.md --json
```

Documented in: `docs/pm-operating-policy.md`, `docs/pm-skill.md`, `docs-site/command-reference.md`

## `pm conformance`

Evaluate PM protocol compliance separately from semantic answer quality.

Discovery tier: `advanced_orchestration`.

Usage:

```bash
gira pm conformance [--from-file RUN.json|-] [--json]
```

Since: `v3.0.0`

Flags:

- `--from-file`: One pm-conformance-run/v1 object or array; built-in human and AI fixtures are the default.
- `--json`: Emit stable pm-conformance-report/v1 JSON.

Examples:

- Run built-in human and two-host AI conformance

```bash
gira pm conformance --json
```

- Evaluate a recorded host run

```bash
gira pm conformance --from-file host-run.json --json
```

Documented in: `docs/v3-pm-harness-release-readiness.md`, `docs-site/command-reference.md`

## `pm context`

Hydrate compact current PM state from typed and legacy GitHub issue evidence.

Discovery tier: `advanced_orchestration`.

Usage:

```bash
gira pm context --repo OWNER/REPO --ticket N [--context-budget N] [--json]
```

Since: `v3.0.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--ticket`: GitHub issue holding the PM ledger.
- `--context-budget`: Maximum compact context size in characters. Default: 6000.
- `--json`: Emit full stable pm-context/v1 JSON.

Examples:

- Hydrate bounded current PM state

```bash
gira pm context --repo OWNER/app --ticket 123
```

- Inspect full typed history

```bash
gira pm context --repo OWNER/app --ticket 123 --json
```

Documented in: `docs/pm-operating-policy.md`, `docs/pm-skill.md`, `docs-site/command-reference.md`

## `pm discovery`

Trace product outcomes through opportunities, hypotheses, experiments, learning, and decisions.

Discovery tier: `advanced_orchestration`.

Usage:

```bash
gira pm discovery --repo OWNER/REPO --ticket N [--context-budget N] [--json]
```

Since: `v3.0.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--ticket`: GitHub issue holding the PM ledger.
- `--context-budget`: Maximum compact context size in characters. Default: 6000.
- `--json`: Emit full stable pm-discovery-graph/v1 JSON.

Examples:

- Inspect a bounded opportunity-to-outcome graph

```bash
gira pm discovery --repo OWNER/app --ticket 123
```

- Inspect the complete trace and diagnostics

```bash
gira pm discovery --repo OWNER/app --ticket 123 --json
```

Documented in: `docs/pm-operating-policy.md`, `docs/pm-skill.md`, `docs-site/command-reference.md`

## `pm measure`

Validate outcome measurement plans and evidence without mutation.

Discovery tier: `advanced_orchestration`.

Usage:

```bash
gira pm measure --repo OWNER/REPO --ticket N [--context-budget N] [--json]
```

Since: `v3.0.0`

Flags:

- `--repo`: Target GitHub repo.
- `--ticket`: Issue holding outcome and measurement records.
- `--context-budget`: Maximum compact context size. Default: 6000.
- `--json`: Emit full pm-measurement-report/v1 JSON.

Examples:

- Validate current outcome evidence

```bash
gira pm measure --repo OWNER/app --ticket 123
```

- Inspect full measurement provenance

```bash
gira pm measure --repo OWNER/app --ticket 123 --json
```

Documented in: `docs/pm-operating-policy.md`, `docs/pm-skill.md`, `docs-site/command-reference.md`

## `pm observe`

Diagnose product-state changes and order bounded PM actions without mutation.

Discovery tier: `advanced_orchestration`.

Usage:

```bash
gira pm observe --repo OWNER/REPO --ticket N [--json]
```

Since: `v3.0.0`

Flags:

- `--repo`: Target GitHub repo.
- `--ticket`: Goal issue holding typed PM and work graph state.
- `--json`: Emit full pm-observe-report/v1 JSON with source reports.

Examples:

- Inspect the next bounded PM actions

```bash
gira pm observe --repo OWNER/app --ticket 123
```

- Inspect source diagnoses and recommendation change

```bash
gira pm observe --repo OWNER/app --ticket 123 --json
```

Documented in: `docs/pm-operating-policy.md`, `docs/goal-operating-model.md`, `docs-site/command-reference.md`

## `pm qa`

Render a PM acceptance QA prompt from task-local PM state and PR evidence.

Discovery tier: `advanced_orchestration`.

Usage:

```bash
gira pm qa --repo OWNER/REPO --ticket N [--pr N] [--diff-summary] [--include-diff] [--json]
```

Since: `v2.5.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--ticket`: Ticket number.
- `--issue`: Compatibility alias for --ticket.
- `--pr`: Explicit PR number.
- `--diff-summary`: Include changed files and diff stat.
- `--include-diff`: Include full diff when used with --diff-summary.
- `--json`: Emit stable gira-pm-qa/v1 JSON with prompt embedded.

Examples:

- Render PM acceptance QA for a ticket PR

```bash
gira pm qa --repo OWNER/app --ticket 123 --diff-summary
```

Documented in: `docs/pm-skill.md`, `docs-site/command-reference.md`

## `pm record`

Append an idempotent typed record to a GitHub-native PM ledger.

Discovery tier: `advanced_orchestration`.

Usage:

```bash
gira pm record --repo OWNER/REPO --ticket N --id ID --kind KIND [--text TEXT|--from-file PATH|-] [--source REF] [--link RELATION=ID] --dry-run|--apply [--json]
```

Since: `v3.0.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--ticket`: GitHub issue holding the PM ledger.
- `--id`: Stable append-safe record ID.
- `--kind`: Ledger kind, including outcome, opportunity, hypothesis, risk, experiment, and measurement.
- `--text`: Record claim or statement.
- `--from-file`: Read record text from file, or '-' for stdin.
- `--source`: Inspectable source reference; repeatable.
- `--actor-kind`: human, ai, system, or integration. Default: human.
- `--status`: Optional kind-specific lifecycle status.
- `--supersedes`: Prior record ID superseded by this record.
- `--link`: Discovery relation=target record ID; repeatable.
- `--goal-ref`: Linked Goal reference; repeatable.
- `--task-profile`: Linked PM task profile; repeatable.
- `--risk-type`: value, usability, feasibility, or viability.
- `--evidence-strength`: anecdotal, qualitative, quantitative, or replicated.
- `--confidence`: low, medium, or high; kept separate from evidence strength.
- `--falsification-test`: Test that can falsify a hypothesis.
- `--test-waiver`: Why a formal falsification test is disproportionate.
- `--experiment-state`: planned, running, success, failure, inconclusive, or invalid.
- `--conclusion`: validated, invalidated, inconclusive, or no_build learning.
- `--outcome-state`: proposed, observing, achieved, not_achieved, or unknown.
- `--signal`: Measurement signal name.
- `--signal-kind`: leading, lagging, delivery, health, or guardrail.
- `--evidence-type`: quantitative, qualitative, or limitation.
- `--baseline`: Baseline value or observation.
- `--baseline-definition`: Baseline population and calculation definition.
- `--target`: Target value or qualitative condition.
- `--target-direction`: increase, decrease, maintain, threshold, or qualitative.
- `--observation-window`: Bounded observation window.
- `--data-source`: Inspectable measurement source.
- `--source-status`: available or unavailable.
- `--owner`: Measurement decision owner.
- `--decision-rule`: Action rule for observed evidence.
- `--evaluation`: met, not_met, inconclusive, unavailable, stable, or regressed.
- `--post-change-definition`: Post-change population and calculation definition.
- `--qualitative-method`: Qualitative evidence method.
- `--qualitative-sample`: Qualitative sample or context.
- `--qualitative-limits`: Qualitative evidence limitations.
- `--evidence-limitation`: Why outcome evidence is unavailable.
- `--follow-up-ref`: Task resolving an evidence limitation.
- `--at`: RFC3339 record time; defaults to current time.
- `--dry-run`: Preview validation and append action without mutation.
- `--apply`: Append the typed issue comment after validation.
- `--json`: Emit stable pm-record-report/v1 JSON.

Examples:

- Preview an evidence record

```bash
gira pm record --repo OWNER/app --ticket 123 --id evidence.setup.1 --kind evidence --text 'Five setup failures' --source log:run-5 --dry-run
```

- Append an accepted decision

```bash
gira pm record --repo OWNER/app --ticket 123 --id decision.output.1 --kind decision --status accepted --from-file decision.md --source issue:OWNER/app#123 --apply
```

Documented in: `docs/pm-operating-policy.md`, `docs/pm-skill.md`, `docs-site/command-reference.md`

## `pm replan`

Preview or apply fingerprinted, capability-aware Goal graph mutations.

Discovery tier: `advanced_orchestration`.

Usage:

```bash
gira pm replan --repo OWNER/REPO --ticket N --dry-run|--apply [--expect-plan ID] [--override ACTION --rationale TEXT] [--json]
```

Since: `v3.0.0`

Flags:

- `--repo`: Target GitHub repo.
- `--ticket`: Goal issue holding typed PM and work graph state.
- `--dry-run`: Preview every graph mutation and residual authority action.
- `--apply`: Apply only safe mutations from an unchanged plan.
- `--expect-plan`: Approved pmr-* dry-run fingerprint required by apply.
- `--override`: Explicit human override, including unblock:#N.
- `--rationale`: Durable product rationale required with an override.
- `--json`: Emit stable pm-replan-report/v1 JSON.

Examples:

- Preview evidence-triggered mutations

```bash
gira pm replan --repo OWNER/app --ticket 123 --dry-run --json
```

- Apply an unchanged replan

```bash
gira pm replan --repo OWNER/app --ticket 123 --apply --expect-plan pmr-...
```

Documented in: `docs/pm-operating-policy.md`, `docs/goal-operating-model.md`, `docs-site/command-reference.md`

## `pm spec`

Render a compact profile-aware PM packet.

Discovery tier: `advanced_orchestration`.

Usage:

```bash
gira pm spec [--profile PROFILE] [INPUT] [--json]
```

Since: `v2.5.0`

Flags:

- `--title`: Task title; defaults to the first non-empty intent line.
- `--repo`: Optional target GitHub repo in OWNER/REPO format.
- `--intent`: Raw product/development intent.
- `--from-file`: Read raw intent from file, or '-' for stdin.
- `--profile`: discovery, decision, experiment, delivery, rollout, measurement, documentation, or legacy. Default: delivery.
- `--context-ref`: Stable parent premise or policy reference; repeatable.
- `--worker-mode`: Suggested worker mode override; defaults by profile.
- `--json`: Emit stable gira-pm-task-packet/v2 JSON; legacy profile emits v1.

Examples:

- Render a compact delivery packet

```bash
gira pm spec --profile delivery --context-ref issue:OWNER/app#100 --repo OWNER/app --from-file request.md > pm-task.md
```

- Render the legacy universal packet

```bash
gira pm spec --profile legacy --repo OWNER/app --from-file - > pm-task.md
```

- Create a ticket from a rendered packet

```bash
gira ticket new --repo OWNER/app --title "TITLE" --body-file pm-task.md --type task --dry-run
```

Documented in: `docs/pm-skill.md`, `docs-site/command-reference.md`

## `queue handoff`

Select or inspect an agent-ready workspace queue item and embed the worker-handoff/v1 payload.

Discovery tier: `advanced_orchestration`.

Workflow role: `advanced_workspace_selector`.

Usage:

```bash
gira queue handoff [--config .gira/config.yaml] [--repo OWNER/REPO] [--ticket N] [--role implementer] [--profile default] [--compact] [--json]
```

Since: `v2.1.0`

Flags:

- `--config`: Explicit workspace config path. Defaults to global registry, then .gira/config.yaml.
- `--repo`: Narrow selection to one execution repo, or select the explicit ticket repo.
- `--ticket`: Explicit ticket number. Without it, handoff uses queue next selection.
- `--role`: Handoff role: planner, implementer, or reviewer. Default: implementer.
- `--profile`: Handoff profile: default or python. Default: default.
- `--compact`: Print compact text output.
- `--json`: Emit stable queue-handoff/v1 JSON with worker-handoff/v1 embedded.

Examples:

- Build a handoff packet for the next LLM-ready item

```bash
gira queue handoff --json
```

Documented in: `docs/workspace.md`, `docs/agent-handoff-queue.md`, `docs-site/agent-handoff-queue.md`, `docs-site/command-reference.md`

## `queue list`

List workspace queue items derived from workspace-queues/v1.

Discovery tier: `advanced_orchestration`.

Usage:

```bash
gira queue list [--config .gira/config.yaml] [--repo OWNER/REPO] [--queue ready|review|finish|blocked|failed|human] [--limit N] [--compact] [--json]
```

Since: `v2.1.0`

Flags:

- `--config`: Explicit workspace config path. Defaults to global registry, then .gira/config.yaml.
- `--repo`: Narrow queue items to one or more execution repos.
- `--queue`: Filter by queue alias: ready, review, finish, blocked, failed, or human.
- `--limit`: Maximum queue items to print. Default: all.
- `--compact`: Print compact text output.
- `--json`: Emit stable queue-list/v1 JSON.

Examples:

- List agent-ready work

```bash
gira queue list --queue ready --json
```

Documented in: `docs/workspace.md`, `docs/agent-handoff-queue.md`, `docs-site/agent-handoff-queue.md`, `docs-site/command-reference.md`

## `queue next`

Select the first agent-ready workspace queue item and print handoff and run-start commands.

Discovery tier: `advanced_orchestration`.

Usage:

```bash
gira queue next [--config .gira/config.yaml] [--repo OWNER/REPO] [--role implementer] [--profile default] [--compact] [--json]
```

Since: `v2.1.0`

Flags:

- `--config`: Explicit workspace config path. Defaults to global registry, then .gira/config.yaml.
- `--repo`: Narrow selection to one or more execution repos.
- `--role`: Handoff role: planner, implementer, or reviewer. Default: implementer.
- `--profile`: Handoff profile: default or python. Default: default.
- `--compact`: Print compact text output.
- `--json`: Emit stable queue-next/v1 JSON.

Examples:

- Select the next LLM-ready item

```bash
gira queue next --json
```

Documented in: `docs/workspace.md`, `docs/agent-handoff-queue.md`, `docs-site/agent-handoff-queue.md`, `docs-site/command-reference.md`

## `queue take`

Start a handoff-safe queue item through the existing ticket start policy.

Discovery tier: `advanced_orchestration`.

Usage:

```bash
gira queue take [--config .gira/config.yaml] [--repo OWNER/REPO] [--ticket N] [--role implementer] [--profile default] [--compact] --dry-run|--apply [--json]
```

Since: `v2.1.0`

Flags:

- `--config`: Explicit workspace config path. Defaults to global registry, then .gira/config.yaml.
- `--repo`: Narrow selection to one execution repo, or select the explicit ticket repo.
- `--ticket`: Explicit ticket number. Without it, take uses queue next selection.
- `--role`: Handoff role: planner, implementer, or reviewer. Default: implementer.
- `--profile`: Handoff profile: default or python. Default: default.
- `--compact`: Print compact text output.
- `--dry-run`: Preview selection, worker handoff, and ticket start without mutation.
- `--apply`: Start only a handoff-safe and worker-ready queue item.
- `--json`: Emit stable queue-take/v1 JSON with worker-handoff/v1 and work-start-result/v1 embedded.

Examples:

- Preview taking the next safe queue item

```bash
gira queue take --dry-run --json
```

Documented in: `docs/workspace.md`, `docs/agent-handoff-queue.md`, `docs-site/agent-handoff-queue.md`, `docs-site/command-reference.md`

## `report backlog-health`

Build a backlog health report from open issue status, age, labels, and planning evidence.

Discovery tier: `assist`.

Usage:

```bash
gira report backlog-health [--repo OWNER/REPO] [--format text|md|json|csv|html|bundle] [--output PATH]
```

Since: `v2.5.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--format`: Output format: text, md, json, csv, html, or bundle.
- `--output`: Output path for md/csv/html, or output root for bundle.
- `--json`: Emit stable project-report/v1alpha1 JSON.
- `--md`: Emit Markdown report.
- `--csv`: Emit CSV rows.
- `--html`: Emit a static local HTML report.

Examples:

- Render backlog health

```bash
gira report backlog-health --repo OWNER/app
```

Documented in: `README.md`, `docs-site/command-reference.md`

## `report changelog`

Build a changelog document from the same milestone and merged PR evidence as release notes.

Discovery tier: `assist`.

Usage:

```bash
gira report changelog --repo OWNER/REPO --milestone TITLE [--format text|md|json|csv|html|bundle] [--output PATH]
```

Since: `v2.5.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--milestone`: Release milestone title to include.
- `--format`: Output format: text, md, json, csv, html, or bundle.
- `--output`: Output path for md/csv/html, or output root for bundle.
- `--json`: Emit stable release-notes-report/v1alpha1 JSON.
- `--md`: Emit Markdown changelog.
- `--csv`: Emit changelog CSV rows.
- `--html`: Emit a static local HTML report.

Examples:

- Render changelog markdown

```bash
gira report changelog --repo OWNER/app --milestone v2.1.0 --format md
```

Documented in: `README.md`, `docs-site/command-reference.md`

## `report delivery-status`

Build a delivery status report from milestone progress, blockers, and PR readiness evidence.

Discovery tier: `assist`.

Usage:

```bash
gira report delivery-status [--repo OWNER/REPO] [--format text|md|json|csv|html|bundle] [--output PATH]
```

Since: `v2.5.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--format`: Output format: text, md, json, csv, html, or bundle.
- `--output`: Output path for md/csv/html, or output root for bundle.
- `--json`: Emit stable project-report/v1alpha1 JSON.
- `--md`: Emit Markdown report.
- `--csv`: Emit CSV rows.
- `--html`: Emit a static local HTML report.

Examples:

- Render delivery status

```bash
gira report delivery-status --repo OWNER/app --format md
```

Documented in: `README.md`, `docs-site/command-reference.md`

## `report milestone`

Build a milestone progress report from GitHub milestone and issue evidence.

Discovery tier: `assist`.

Usage:

```bash
gira report milestone --repo OWNER/REPO --milestone TITLE [--format text|md|json|csv|html|bundle] [--output PATH]
```

Since: `v2.5.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--milestone`: Milestone title to inspect.
- `--format`: Output format: text, md, json, csv, html, or bundle.
- `--output`: Output path for md/csv/html, or output root for bundle.
- `--json`: Emit stable project-report/v1alpha1 JSON.
- `--md`: Emit Markdown report.
- `--csv`: Emit CSV rows.
- `--html`: Emit a static local HTML report.

Examples:

- Render milestone progress

```bash
gira report milestone --repo OWNER/app --milestone v2.1.0 --format md
```

Documented in: `README.md`, `docs-site/command-reference.md`

## `report portfolio`

Render a self-contained local HTML overview of milestone progress, dated gates, and blocked or review-waiting queues.

Discovery tier: `assist`.

Usage:

```bash
gira report portfolio [--repo OWNER/REPO ...] [--milestone TITLE ...] [--since YYYY-MM-DD] [--until YYYY-MM-DD] --output PATH
```

Since: `v2.6.0`

Flags:

- `--repo`: Repository filter; repeat to include multiple repositories.
- `--milestone`: Exact milestone-title filter; repeat to include multiple milestones.
- `--since`: Inclusive timeline and queue window start in YYYY-MM-DD.
- `--until`: Inclusive timeline and queue window end in YYYY-MM-DD.
- `--output`: Required local HTML output path; generation never publishes, serves, or opens it.

Examples:

- Render a bounded local portfolio view

```bash
gira report portfolio --repo OWNER/app --milestone v2.1.0 --since 2026-07-01 --until 2026-09-30 --output out/portfolio.html
```

Documented in: `README.md`, `docs/visual-portfolio-report.md`, `docs-site/command-reference.md`

## `report qa-checklist`

Build a QA checklist report from issue labels, open PR checks, review state, and closure-link evidence.

Discovery tier: `assist`.

Usage:

```bash
gira report qa-checklist [--repo OWNER/REPO] [--milestone TITLE] [--format text|md|json|csv|html|bundle] [--output PATH]
```

Since: `v2.5.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--milestone`: Optional milestone title to scope issue checks.
- `--format`: Output format: text, md, json, csv, html, or bundle.
- `--output`: Output path for md/csv/html, or output root for bundle.
- `--json`: Emit stable project-report/v1alpha1 JSON.
- `--md`: Emit Markdown report.
- `--csv`: Emit CSV rows.
- `--html`: Emit a static local HTML report.

Examples:

- Render milestone QA checklist

```bash
gira report qa-checklist --repo OWNER/app --milestone v2.1.0 --format md
```

Documented in: `README.md`, `docs-site/command-reference.md`

## `report release-notes`

Build human-readable release notes from milestone issues and merged PR closing evidence.

Discovery tier: `assist`.

Usage:

```bash
gira report release-notes --repo OWNER/REPO --milestone TITLE [--format text|md|json|csv|html|bundle] [--output PATH]
```

Since: `v2.5.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--milestone`: Release milestone title to include.
- `--format`: Output format: text, md, json, csv, html, or bundle.
- `--output`: Output path for md/csv/html, or output root for bundle.
- `--json`: Emit stable release-notes-report/v1alpha1 JSON.
- `--md`: Emit Markdown release notes.
- `--csv`: Emit release item CSV rows.
- `--html`: Emit a static local HTML report.

Examples:

- Render release notes markdown

```bash
gira report release-notes --repo OWNER/app --milestone v2.1.0 --format md
```

- Write a release notes bundle

```bash
gira report release-notes --repo OWNER/app --milestone v2.1.0 --format bundle --output out/release-notes
```

Documented in: `README.md`, `docs-site/command-reference.md`

## `report schedule`

Build a schedule-oriented execution report sorted by date and week bucket.

Discovery tier: `assist`.

Usage:

```bash
gira report schedule [--repo OWNER/REPO] [--state open|closed|all] [--by week] [--scenario current|one-month] [--format text|json|csv] [--output PATH]
```

Since: `v2.5.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--state`: Issue state filter: open, closed, or all. Default: open.
- `--by`: Schedule grouping. Currently supports week.
- `--scenario`: Planning scenario: current or one-month.
- `--format`: Output format: text, json, or csv.
- `--output`: Output path for csv.
- `--json`: Emit stable execution-report/v1alpha1 JSON.
- `--csv`: Emit execution rows as CSV.

Examples:

- Render weekly schedule rows for Sheets

```bash
gira report schedule --repo OWNER/app --by week --format csv
```

- Compare a compressed one-month planning scenario

```bash
gira report schedule --repo OWNER/app --scenario one-month --format json
```

Documented in: `README.md`, `docs-site/command-reference.md`

## `report wbs`

Build structural or execution-focused WBS reports from GitHub epics, issues, milestones, and roadmap dates.

Discovery tier: `assist`.

Usage:

```bash
gira report wbs [--repo OWNER/REPO] [--state open|closed|all] [--mode structural|execution] [--scenario current|one-month] [--format text|json|csv|html|bundle] [--output PATH]
```

Since: `v2.5.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--state`: Issue state filter: open, closed, or all. Default: open.
- `--mode`: Report model: structural preserves hierarchy-first WBS; execution emits actionable planning rows.
- `--scenario`: Planning scenario for execution mode: current or one-month.
- `--format`: Output format: text, json, csv, html, or bundle.
- `--output`: Output path for csv/html, or output root for bundle.
- `--json`: Emit stable wbs-report/v1alpha1 JSON.
- `--csv`: Emit WBS CSV rows.
- `--html`: Emit a static local HTML report.

Examples:

- Render a terminal WBS summary

```bash
gira report wbs --repo OWNER/app
```

- Render execution WBS rows for Sheets

```bash
gira report wbs --repo OWNER/app --mode execution --format csv
```

- Write a shareable WBS report bundle

```bash
gira report wbs --repo OWNER/app --format bundle --output out/wbs
```

Documented in: `README.md`, `docs-site/command-reference.md`

## `report weekly`

Build a weekly PM cockpit report with deterministic KPIs and top exceptions.

Discovery tier: `assist`.

Usage:

```bash
gira report weekly [--repo OWNER/REPO] [--format text|md|json|csv|html|bundle] [--output PATH]
```

Since: `v2.5.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--format`: Output format: text, md, json, csv, html, or bundle.
- `--output`: Output path for md/csv/html, or output root for bundle.
- `--json`: Emit stable weekly-report/v1alpha1 JSON.
- `--md`: Emit Markdown report.
- `--csv`: Emit CSV rows.
- `--html`: Emit a static local HTML report.

Examples:

- Render weekly PM cockpit markdown

```bash
gira report weekly --repo OWNER/app --format md
```

- Write a weekly report bundle

```bash
gira report weekly --repo OWNER/app --format bundle --output out/weekly
```

Documented in: `README.md`, `docs-site/command-reference.md`

## `setup global`

Create or update the OS-user global config, workspace registry, and repo registry.

Discovery tier: `supporting`.

Usage:

```bash
gira setup global [--repo OWNER/REPO] [--path .] [--workspace NAME] [--inbox-repo OWNER/REPO] [--mode global-only|hybrid] --dry-run|--apply
```

Since: `v1.7.0`

Flags:

- `--repo`: Initial execution repo.
- `--inbox-repo`: Backlog/intake repo for unassigned work.
- `--mode`: Use global-only or hybrid repo-local contract mode.

Examples:

- Preview global-first setup

```bash
gira setup global --repo OWNER/app --path . --workspace personal --inbox-repo OWNER/backlog --mode global-only --dry-run
```

Documented in: `README.md`, `docs/global-config-registry.md`, `docs-site/global-config.md`, `docs/workspace.md`

## `stats pulse`

Show a read-only recent workflow pulse for one GitHub repo.

Discovery tier: `assist`.

Usage:

```bash
gira stats pulse [OWNER/REPO] [--repo OWNER/REPO] [--since 7d] [--limit 100] [--json]
```

Since: `v2.2.0`

Flags:

- `--repo`: Target GitHub repo. May also be positional.
- `--since`: Reporting window such as 7d or YYYY-MM-DD. Default: 7d.
- `--limit`: Max GitHub rows per query. Default: 100.
- `--json`: Emit stable pulse-report/v1alpha1 JSON.

Examples:

- Render the recent repo pulse

```bash
gira stats pulse --repo OWNER/app --since 7d
```

Documented in: `docs/task-momentum-loop.md`, `docs/closure-funnel-stats.md`, `docs-site/task-momentum-loop.md`, `docs-site/closure-funnel-stats.md`

## `stats repo`

Show a read-only Closure Funnel report for one GitHub repo.

Discovery tier: `assist`.

Usage:

```bash
gira stats repo [OWNER/REPO] [--repo OWNER/REPO] [--since 90d] [--stale-days 14] [--limit 100] [--json]
```

Since: `v1.12.0`

Flags:

- `--repo`: Target GitHub repo. May also be positional.
- `--since`: Reporting window such as 90d or YYYY-MM-DD. Default: 90d.
- `--stale-days`: Count open issues and PRs stale after this many days. Default: 14.
- `--limit`: Max GitHub rows per query. Default: 100.
- `--json`: Emit stable JSON for automation.

Examples:

- Render the default repo report

```bash
gira stats repo --repo OWNER/app --since 90d
```

Documented in: `README.md`, `docs/closure-funnel-stats.md`, `docs-site/closure-funnel-stats.md`

## `stats workspace`

Planned multi-repo Closure Funnel rollup for a configured workspace.

Discovery tier: `assist`.

Usage:

```bash
gira stats workspace [--since 90d]
```

Since: `planned`

Flags:

- `--since`: Reporting window such as 90d or YYYY-MM-DD.

Examples:

- Planned workspace rollup

```bash
gira stats workspace --since 90d
```

Documented in: `docs/closure-funnel-stats.md`, `docs-site/closure-funnel-stats.md`

## `ticket checks`

Show linked PR checks, review blockers, and next action.

Discovery tier: `managed_delivery`.

Usage:

```bash
gira ticket checks [TICKET] [--repo OWNER/REPO] [--json]
```

Since: `v1.0.0`

Examples:

- Inspect PR readiness

```bash
gira ticket checks
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `ticket finish`

Merge the linked PR when policy allows; Draft PRs stop after ready transition and require a new finish preview.

Discovery tier: `managed_delivery`.

Usage:

```bash
gira ticket finish [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--sync-local]
```

Since: `v1.0.0`

Examples:

- Preview finish

```bash
gira ticket finish --dry-run
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `ticket handoff`

Compile a worker-neutral handoff packet from ticket context.

Discovery tier: `managed_delivery`.

Workflow role: `canonical_single_issue_agent_entry_point`.

Usage:

```bash
gira ticket handoff [TICKET] [planner|implementer|reviewer] [--role planner|implementer|reviewer] [--profile default|python] [--repo OWNER/REPO] [--json]
```

Since: `v1.17.0`

Flags:

- `--role`: Handoff role: planner, implementer, or reviewer. Default: implementer.
- `--profile`: Handoff profile: default or python. Default: default.
- `--json`: Emit stable worker-handoff/v1 JSON.

Examples:

- Compile an implementer handoff packet for the current branch ticket

```bash
gira ticket handoff --json
```

- Compile a reviewer handoff packet for the current branch ticket

```bash
gira ticket handoff reviewer --json
```

Documented in: `docs-site/ticket-workflow.md`, `docs-site/command-reference.md`, `docs/dogfood.md`

## `ticket new`

Create a repo-bound executable GitHub issue with structured or full Markdown body input.

Discovery tier: `managed_delivery`.

Usage:

```bash
gira ticket new "Title" --dry-run|--apply [--parent N] [--body TEXT|--body-file PATH|-] [--release-impact MODE] [--start]
```

Since: `v1.0.0`

Flags:

- `--goal`: Structured issue goal.
- `--acceptance`: Semicolon-separated acceptance criteria.
- `--type`: Ticket type: epic, story, task, bug, spike, or chore.
- `--priority`: Priority: p0, p1, p2, or p3.
- `--parent`: Native GitHub parent issue for the created ticket.
- `--label`: Additional repo label that must already exist.
- `--body`: Full issue body.
- `--body-file`: Read full issue body from file or stdin with -.
- `--release-impact`: Release impact: user-facing, internal, or exempt.
- `--release-impact-reason`: Reason required for exempt.
- `--start`: Start the created ticket after apply.

Examples:

- Preview structured ticket

```bash
gira ticket new "TITLE" --goal "GOAL" --acceptance "a;b;c" --dry-run
```

- Preview full Markdown packet

```bash
gira ticket new --title "TITLE" --body-file issue.md --dry-run
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `ticket note`

Post a structured context note to the issue, linked PR, or both.

Discovery tier: `managed_delivery`.

Usage:

```bash
gira ticket note [TICKET] "BODY" --dry-run|--apply [--repo OWNER/REPO] [--kind progress|blocker|decision|handoff|summary|check] [--target auto|issue|pr|both]
```

Since: `v1.12.0`

Flags:

- `--kind`: Template kind for the note. Default: progress.
- `--target`: Comment target: auto, issue, pr, or both. Default: auto.
- `--body`: Explicit note body.
- `--body-file`: Read note body from file or stdin with -.
- `--dry-run`: Preview target resolution and rendered note without posting.
- `--apply`: Post the rendered note.

Examples:

- Preview a progress note

```bash
gira ticket note "Implemented parser path" --dry-run
```

- Post a blocker to both issue and PR

```bash
gira ticket note --kind blocker --target both --body-file note.md --apply
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `ticket parent`

Show, set, or clear a native GitHub sub-issue parent without adding a separate link command family.

Discovery tier: `managed_delivery`.

Usage:

```bash
gira ticket parent TICKET [--set PARENT|--clear] [--dry-run|--apply] [--repo OWNER/REPO] [--json]
```

Since: `v1.17.0`

Flags:

- `--set`: Set the native GitHub parent issue.
- `--clear`: Clear the native GitHub parent issue.
- `--dry-run`: Preview the parent mutation.
- `--apply`: Apply the parent mutation.

Examples:

- Preview parent link

```bash
gira ticket parent 42 --set 10 --dry-run
```

- Show current parent

```bash
gira ticket parent 42
```

Documented in: `README.md`, `docs/command-surface-boundary.md`

## `ticket pr`

Create or validate a linked PR with required issue closing text.

Discovery tier: `managed_delivery`.

Usage:

```bash
gira ticket pr [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--draft]
```

Since: `v1.0.0`

Examples:

- Open a draft PR

```bash
gira ticket pr --apply --draft
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `ticket prompt`

Render a stateless planner, implementer, or reviewer prompt from ticket context.

Discovery tier: `managed_delivery`.

Usage:

```bash
gira ticket prompt [TICKET] [planner|implementer|reviewer] [--role planner|implementer|reviewer] [--profile default|python] [--repo OWNER/REPO] [--pr N] [--json]
```

Since: `v1.14.0`

Flags:

- `--role`: Prompt role: planner, implementer, or reviewer.
- `--profile`: Prompt profile: default or python. Default: default.
- `--pr`: Optional PR number for reviewer prompt context.
- `--json`: Emit stable JSON including the rendered prompt.

Examples:

- Render an implementation worker prompt for the current branch ticket

```bash
gira ticket prompt implementer --profile python
```

- Render a reviewer prompt with an explicit PR override

```bash
gira ticket prompt reviewer --pr 77
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `ticket review`

Render a reviewer packet from current ticket and linked PR state.

Discovery tier: `managed_delivery`.

Usage:

```bash
gira ticket review [TICKET] [--repo OWNER/REPO] [--pr N] [--diff-summary] [--include-diff] [--json|--html --output PATH]
```

Since: `v1.15.0`

Flags:

- `--pr`: Optional PR number override for reviewer packet context.
- `--diff-summary`: Include changed files, diff stat, hunk headers, acceptance mapping candidates, and risk hints.
- `--include-diff`: Include the full PR diff. Output can be long and must be requested explicitly.
- `--json`: Emit stable JSON including issue, PR, evidence, repo guidance, verdict schema, and prompt fields.
- `--html`: Write a static local HTML review packet.
- `--output`: Output path for --html.

Examples:

- Render reviewer packet for current branch ticket

```bash
gira ticket review --diff-summary
```

- Render reviewer packet with an explicit PR override

```bash
gira ticket review --ticket 42 --pr 77 --json
```

- Write a local review packet page

```bash
gira ticket review 42 --repo OWNER/app --diff-summary --html --output out/gira/review-42.html
```

Documented in: `docs-site/ticket-workflow.md`, `docs-site/command-reference.md`, `docs/dogfood.md`

## `ticket self-review`

Post a self-review check note for the current branch ticket and linked PR.

Discovery tier: `managed_delivery`.

Usage:

```bash
gira ticket self-review [TICKET] [--repo OWNER/REPO] [--pr N] [--diff-summary] --dry-run|--apply [--json]
```

Since: `v1.18.0`

Flags:

- `--pr`: Optional PR number override for self-review context.
- `--diff-summary`: Include compact PR diff summary in the check note. Default: true.
- `--dry-run`: Preview the self-review PR note without posting.
- `--apply`: Post the self-review check note to the linked PR.
- `--json`: Emit stable ticket-self-review-report/v1 JSON.

Examples:

- Preview current branch self-review note

```bash
gira ticket self-review --diff-summary --dry-run
```

- Post current branch self-review note

```bash
gira ticket self-review --diff-summary --apply
```

Documented in: `docs-site/ticket-workflow.md`, `docs-site/command-reference.md`, `docs/dogfood.md`

## `ticket start`

Start a ready issue with an explicit branch strategy.

Discovery tier: `managed_delivery`.

Compatibility aliases: `gira start`.

Usage:

```bash
gira ticket start [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--base BRANCH] [--create|--current|--adopt BRANCH]
```

Since: `v1.0.0`

Flags:

- `--base`: Explicit lifecycle base branch override recorded on the ticket.
- `--create`: Create the policy-suggested work branch.
- `--current`: Bind the current branch without checkout or push.
- `--adopt`: Bind an existing local or origin branch without checkout or push.
- `--json`: Emit the stable ticket-status/v1 JSON contract with issue, branch, PR, checks, review, evidence, blockers, warnings, and next action.

Examples:

- Create the suggested branch for a ready issue

```bash
gira ticket start 42 --create --apply
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `ticket status`

Report ticket status, linked PR blockers, and next action.

Discovery tier: `managed_delivery`.

Usage:

```bash
gira ticket status [TICKET] [--repo OWNER/REPO] [--json|--html --output PATH]
```

Since: `v1.0.0`

Flags:

- `--repo`: Target GitHub repo in OWNER/REPO format.
- `--ticket`: Ticket number. Can also be numeric positional.
- `--issue`: Compatibility alias for --ticket.
- `--json`: Emit the stable ticket-status/v1 JSON contract with issue, branch, PR, checks, review, evidence, blockers, warnings, and next action.
- `--html`: Write a static local HTML report from ticket-status/v1.
- `--output`: Output path for --html.

Examples:

- Inspect current branch ticket

```bash
gira ticket status
```

- Export a ticket status page

```bash
gira ticket status 42 --repo OWNER/app --html --output out/gira/ticket-42.html
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `ticket supersede`

Close a ticket as superseded and create a linked replacement ticket.

Discovery tier: `managed_delivery`.

Usage:

```bash
gira ticket supersede [TICKET] --replacement-title TITLE --body-file PATH|- --dry-run|--apply [--repo OWNER/REPO] [--close-draft-pr]
```

Since: `v1.12.0`

Flags:

- `--replacement-title`: Title for the replacement issue.
- `--body`: Replacement issue body.
- `--body-file`: Read replacement issue body from file or stdin with -.
- `--label`: Additional replacement issue label.
- `--milestone`: Override replacement issue milestone.
- `--close-draft-pr`: Close a linked draft PR when superseding.
- `--dry-run`: Preview all planned mutations.
- `--apply`: Create the replacement, cross-link notes, status update, and close the original.

Examples:

- Preview a replacement ticket

```bash
gira ticket supersede 64 --replacement-title "Define release gate" --body-file replacement.md --dry-run
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `ticket view`

Show a Gira operating card for the ticket, linked PR, blockers, and next action. Alias: gira ticket show.

Discovery tier: `managed_delivery`.

Compatibility aliases: `gira ticket show`.

Usage:

```bash
gira ticket view|show [TICKET] [--repo OWNER/REPO] [--json]
```

Since: `v1.12.0`

Examples:

- Inspect current branch ticket context

```bash
gira ticket view
```

- Inspect an explicit ticket with the show alias

```bash
gira ticket show 42 --repo OWNER/app
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `ticket wait`

Wait for pending linked PR checks without merging.

Discovery tier: `managed_delivery`.

Usage:

```bash
gira ticket wait [TICKET] [--repo OWNER/REPO] [--timeout 5m] [--interval 5s]
```

Since: `v1.0.0`

Examples:

- Wait for CI

```bash
gira ticket wait --timeout 5m
```

Documented in: `README.md`, `docs-site/ticket-workflow.md`, `docs/dogfood.md`

## `workspace repos sync`

Discover GitHub owner/org repos and update a global workspace execution repo allowlist.

Discovery tier: `managed_delivery`.

Usage:

```bash
gira workspace repos sync [--owner OWNER] [--workspace NAME] --dry-run|--apply [--include-archived]
```

Since: `v1.8.0`

Flags:

- `--owner`: GitHub user or organization. Defaults to workspace.owner.
- `--workspace`: Global workspace name. Defaults to global config default_workspace.
- `--include-archived`: Include archived repositories.

Examples:

- Preview owner repo sync

```bash
gira workspace repos sync --owner OWNER --workspace personal --dry-run
```

Documented in: `docs/global-config-registry.md`, `docs-site/global-config.md`, `docs/workspace.md`

## `workspace status`

Show inbox and execution repo state from a workspace config or global workspace registry.

Discovery tier: `managed_delivery`.

Usage:

```bash
gira workspace status [--config .gira/config.yaml] [--repo OWNER/REPO] [--limit N] [--active-only] [--cache-ttl 5m] [--refresh] [--json]
```

Since: `v1.0.0`

Flags:

- `--config`: Explicit workspace config path. Defaults to global registry, then .gira/config.yaml.
- `--repo`: Narrow status to one or more execution repos.
- `--limit`: Inspect only the first N selected execution repos.
- `--active-only`: Show only execution repos with open work or an active milestone.
- `--max-concurrency`: Bound concurrent repo status fetches. Default: 4.
- `--cache-ttl`: Reuse recent per-repo status cache for this duration. Default: 5m.
- `--refresh`: Ignore cached status and fetch fresh data.
- `--json`: Emit stable JSON.

Examples:

- Read the default workspace

```bash
gira workspace status
```

- Inspect a bounded subset

```bash
gira workspace status --limit 10 --active-only
```

Documented in: `README.md`, `docs/workspace.md`, `docs-site/global-config.md`

