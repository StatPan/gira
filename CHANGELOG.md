# Changelog

All notable Gira release changes are tracked here.

Gira uses SemVer tags. User-facing features normally increment the minor version and fixes increment the patch version.

## Unreleased

## v2.3.0 - 2026-06-11

- Added MCP env-token authentication selection for `gira mcp serve`, preferring
  `GIRA_MCP_GITHUB_TOKEN`, then `GITHUB_TOKEN`, then `GH_TOKEN`, and finally
  local `gh` authentication when no token env var is configured.
- Added `gira mcp doctor --repo OWNER/REPO --json` so operators can inspect the
  active MCP auth mode, token variable presence, GitHub host, and next setup
  action without exposing credential values.
- Documented the MCP authentication interface boundary for local env-token
  usage, `gh` auth fallback, hosted-service OAuth expectations, and
  non-goals such as credential storage or MCP mutation tools.

## v2.2.0 - 2026-06-11

- Added the local read-only MCP server with `gira mcp serve`, exposing stable
  ticket and queue read/handoff tools over the existing Gira CLI JSON
  contracts.
- Defined the CLI/MCP parity boundary so CLI, CLI JSON, and MCP remain access
  paths to one evidence-backed Gira lifecycle rather than separate workflow
  models.
- Documented the mixed CLI/MCP operating flow: use MCP for read/context
  surfaces, CLI `--dry-run` for mutation plans, and CLI `--apply` for approved
  mutations.
- Added hosted MCP service boundary documentation that keeps managed MCP
  read-only by default and preserves GitHub evidence plus Gira receipts as the
  workflow source of truth.
- Added local agent workflow benchmark fixtures for readiness, queue,
  failed-check, review, finish-ready, and human-decision workflow decisions.

## v2.0.0 - 2026-05-29

- Stabilizes Gira as a GitHub-native control plane for AI-assisted software
  work: executable tickets, state-aware PR context, readiness reports, review
  packets, evidence-backed finish, goal mode, workspace queues, and adapter
  approval evidence now form one public product contract.
- Defines the 2.0 stable CLI surface around `ticket`, `goal`, `workspace`,
  `audit`, `stats`, Jira-primary provider diagnostics, command capability
  metadata, and dry-run/apply approval boundaries.
- Keeps hosted dashboards, UI/TUI workflows, GitLab/Forgejo providers, Notion
  integration, broad background sync, full bidirectional Jira sync, and
  Gira-native planning databases outside the 2.0 release boundary.
- Adds release-readiness documentation for the maintainer decision to tag
  `v2.0.0` after final verification and human approval.

## v1.17.0 - 2026-05-24

- Added optional issue-backed feature map commands with the short `gira feat`
  alias so operators and agents can list feature records, check feature map
  health, and inspect whether a work issue is linked to a capability.
- Added the feature map convention docs for GitHub issue-backed capability
  records, keeping GitHub Projects as visibility views and GitHub issues as the
  canonical source of truth.
- Added stable JSON contracts for `feature-map-list/v1`,
  `feature-map-check/v1`, and `feature-map-for/v1`.

## v1.16.0 - 2026-05-19

- Added milestone lifecycle commands for creating, listing, assigning, planning,
  and inspecting repo milestones without dropping to raw GitHub commands.
- Hardened state-aware ticket and PR context resolution so ticket commands can
  infer work from current branches or PRs and return actionable ambiguity
  errors when context is missing or unsafe.
- Added deterministic ticket status JSON fields for labels, milestones, branch
  binding, linked PR state, checks, review state, evidence, acceptance
  criteria, telemetry, warnings, and next actions.
- Added stateless role prompt packets for planner, implementer, and reviewer
  workers, plus reviewer packet contracts with diff references, guidance, and
  verdict schemas.
- Added finish readiness and finish receipt contracts so `ticket finish`
  previews evidence-backed completion, writes concise completion receipts on
  apply, and reports AI Delivery Telemetry warnings for agent-routed work.
- Added `gira audit drift` for workflow convergence audits across issue status,
  linked PRs, checks, evidence, and telemetry while preserving the existing
  `audit workflow` compatibility path.
- Improved action-oriented errors for missing ticket context, ambiguous
  workflow state, unready issues, and missing milestone titles.

## v1.15.0 - 2026-05-18

- Added the reviewer packet workflow with `gira ticket review`, including
  linked PR state, checks, changed files, finish readiness, evidence fields,
  and a rendered reviewer prompt for stateless review workers.
- Hardened reviewer prompts with explicit read-only behavior, actual PR diff
  inspection commands, repository-local agent instruction reminders, Gira
  workflow conventions, tool contract checks, telemetry context, and
  changed-surface test expectations.
- Added workflow convergence auditing with `gira audit workflow`, `status:done`
  normalization, and no-open-work readiness semantics for completed queues.
- Added goal-level operating model and contribution provenance support so
  planning, implementation, and review ownership can be tracked across
  human/AI work.
- Improved ticket and workspace operating UX with `ticket show` guidance,
  retry behavior for linked PR lookup after PR creation, workspace init merge
  support, adopt repo/workspace guidance, adopt issues before/after state
  clarity, project sync skip reasons, and closed epic status guidance.
- Hardened security-sensitive boundaries for generated CLI continuation
  commands, Jira API base URLs, release and site publishing workflows, review
  gate policy checks, ticket finish branch binding, workspace guardrail
  resolution, symlink-safe local writes, and branch push execution.
- Expanded test coverage across ticket lifecycle formatting, workspace routing,
  projects sync, Jira import/export and doctor diagnostics, sprint/release
  review paths, report formatters, config and repo registry paths, and
  deterministic output contracts.
- Improved public positioning and documentation for Gira's issue-to-PR
  agent workflow, business-group multi-repo workflows, release distribution,
  and canonical agent operator guidance.

## v1.12.0 - 2026-05-14

- Added Jira-primary provider workflow support across provider discovery,
  config apply, GitHub mirror issue resolution, ticket view/start by Jira key,
  transition dry-run planning, and Done transition gating on GitHub execution
  evidence.
- Added Jira provider documentation, compatibility diagnostics, and hosted
  control-plane roadmap notes for future provider health checks.
- Added `gira ticket view`, `gira ticket note`, `gira ticket supersede`,
  closure funnel stats, and stateless planner/implementer/reviewer prompt
  rendering for ticket workflows.
- Expanded GitHub issue and PR workflow templates plus canonical agent
  delegation guidance for Gira-oriented human/AI handoffs.
- Improved workflow contract test coverage, `ticket new` label preflight and
  unsupported type guidance, `ticket finish` status label normalization, and
  doctor checks for workflow policy labels and closed issue status drift.

## v1.11.0 - 2026-05-10

- Added rate-limit-aware `workspace status` operation with GitHub API budget
  reporting, bounded multi-repo fetch concurrency, per-repo status caching,
  and repo/limit/active filters.
- Added a generated managed block to the canonical agent operator skill so
  lifecycle command guidance stays in sync with command registry metadata.
- Added shared agent guidance renderers so `gira guide agent`, `gira guide
  skill`, docs-site agent guidance, and `AGENTS.md` managed blocks can reuse
  command registry descriptions without overwriting human-owned instructions.

## v1.10.0 - 2026-05-10

- Rendered the built-in ticket guide command section from the command metadata
  registry to reduce help/docs drift.
- Added a command metadata registry with a generated docs-site command
  reference and drift-prevention tests for core public commands.

## v1.9.0 - 2026-05-10

- Added full-body ticket creation support with `gira ticket new --body`,
  `--body-file PATH`, and `--body-file -` while preserving dry-run previews.

## v1.8.0 - 2026-05-10

- Added `gira workspace repos sync` to discover GitHub user/org repositories
  and update a global workspace execution repo list through dry-run/apply.

## v1.7.0 - 2026-05-10

- Added `gira setup global` to configure global-first operation through one
  dry-run/apply flow, including global defaults, workspace registry, repo
  registry, and global-only versus hybrid repo-local contract modes.
- Clarified global setup guidance so `inbox_repo` is presented as a
  backlog/intake repo, with single-repo inbox fallback called out as a limited
  convenience.

## v1.6.0 - 2026-05-10

- Prefer the OS-user global registry for default workspace config resolution
  when a matching global workspace is available, while preserving explicit
  `--config .gira/config.yaml` as the repo-local contract opt-out.
- Added global registry commands and docs for repo registration,
  repo-local contract migration, and `workspace init --scope global`.

## v1.5.1 - 2026-05-09

- Fixed `gira ticket start` guidance for open issues without `status:ready`, including actionable human and JSON next steps.
- Added a dependency security audit gate for Next.js and React Server DOM advisory floors across docs and npm wrapper build paths.

## v1.5.0 - 2026-05-08

- Added `gira audit readiness` to combine doctor checks, audit ledger health, and a Gira-first next action in one self-audit command.
- Added `gira ticket list`, `gira epic list`, and `gira workspace list` so daily issue, epic, and backlog listing no longer requires dropping to raw `gh`.
- Added multi-repository status summaries with `gira status --all` and owner discovery with `gira status --owner OWNER`.
- Clarified binary-first install guidance across direct installer, npm/bun, PyPI, uv/pipx/pip, and Homebrew channels.
- Raised the Go module baseline to Go 1.26.

## v1.4.6 - 2026-05-08

- Improved workspace project adoption help and follow-up guidance.

## v1.4.5 - 2026-05-08

- Added adoption support for existing workspace Project configuration.

## v1.4.4 - 2026-05-08

- Added `gira cache prune` to preview and remove stale wrapper-managed binary caches.

## v1.4.3 - 2026-05-08

- Added default workspace Project board setup.
- Added numberless workspace ticket routing.
- Clarified the repository execution boundary and launch positioning.

## v1.4.2 - 2026-05-07

- Fixed PyPI wrapper executions so uv and pipx installs propagate their install channel to the native `gira` binary before running `gira upgrade`.

## v1.4.1 - 2026-05-07

- Fixed `gira upgrade` channel auto-detection for uv tool installs whose `gira` executable is exposed through a symlink in the user's bin directory.

## v1.4.0 - 2026-05-07

- Added the public documentation site source and VitePress docs build for the GitHub Pages site.
- Added existing repository adoption with `gira adopt repo`, including observe, merge, and normalize strategies that preserve existing user files.
- Added numberless epic lifecycle commands so epics can be created, started, and closed without manually tracking issue numbers.
- Documented the npm install channel and added uv global tool install guidance with `uv tool install gira-cli`.
- Added `gira upgrade --channel uv` so uv installs receive the correct `uv tool upgrade gira-cli` next step.
- Kept Pages deployment non-blocking while the custom domain and Pages environment are being finalized.

## v1.3.1 - 2026-05-06

- Made `gira doctor` adoption-aware so optional Gira sample bootstrap issues no longer make existing repositories look unhealthy.
- Clarified `gira ops sync` as the canonical metadata sync command while keeping `gira sync` as an alias.
- Extended `gira adopt issues` with `--issues` list/range selection for bulk milestone and label adoption.
- Added `gira adopt issues --normalize-status` to remove active status labels from closed issues during existing-repo cleanup.
- Changed doctor local git readiness so Gira-owned audit ledger changes do not fail readiness when no user worktree changes are present.

## v1.3.0 - 2026-05-06

- Added bootstrap/workspace adoption flow polish: default `.gira/config.yaml`, conflict continuation guidance, and first branch push handling for `gira ticket pr`.
- Added `gira workspace init`, `workspace capability`, and `workspace validate` for personal or repo-bound backlog setup, permission checks, and routing readiness.
- Hardened workspace ticket routing with positional inbox ticket creation, child issue reuse, routed/done inbox status, and child issue evidence in workspace backlog JSON.
- Added `gira adopt issues` to inspect existing repository issues and explicitly apply milestone/label mappings during Gira adoption.
- Added epic support to `gira ticket new --type epic`.
- Added a Jira backend parity acceptance matrix covering backlog, epic, story, task, sprint, release, workflow, priority, owner, and blocker concepts.
- Connected sprint rollover output to ticket lifecycle evidence and prevented `status:done` tickets from being moved during rollover.

## v1.2.0 - 2026-05-06

- Added built-in `gira guide` and `gira docs` help topics for quickstart, ticket flow, agent usage, and Jira concept mapping.
- Added advisory `gira upgrade` and `gira update` aliases to check the latest release and print channel-specific upgrade instructions.
- Improved `gira projects sync` performance by reducing duplicate Project field reads and parallelizing independent Project/repo reads.
- Improved daily CLI performance by parallelizing independent GitHub reads and using the faster issue-list path for status summaries.
- Added `gira ticket finish` to complete the ticket loop with linked PR validation, draft-ready handling, merge blockers, safe merge, local main sync, and convergence reporting.
- Added `gira ticket checks` and `gira ticket wait` so linked PR checks can be inspected and waited on without dropping to raw `gh`.
- Added shorter `gira ticket` daily commands with repo inference, positional ticket numbers, and branch/PR ticket context.
- Added `gira ticket new` for repo-bound structured ticket creation with optional immediate start.

## v1.1.1 - 2026-05-06

- Fixed the npm/bun wrapper so the `gira` command can lazily download the native release binary when a package manager skips the install lifecycle script.

## v1.1.0 - 2026-05-06

- Added `gira workspace ...` for Jira-like personal backlog, inbox tickets, and repo routing before work belongs to one execution repository.
- Added `gira projects sync` as a GitHub Projects v2 visibility bridge for configured workspace issues.
- Added Project field mirroring from Gira labels: `status:*` to `Status`, `priority:*` to `Priority`, `area:*` to `Layer / workstream`, `agent:*` to `Owner / agent`, and milestone due dates to `Target date`.
- Changed closed Project item handling so closed issues stay visible as `Done` by default, with `--archive-closed` as an opt-in cleanup mode.
- Added public-release documentation and an LLM/agent runbook with the safe dry-run -> apply -> PR workflow order.
- Clarified install channels: `install.sh`, PyPI `gira-cli`, and Homebrew are published channels; npm/bun use the `@statpan/gira` wrapper when that channel is published.

## v1.0.0 - 2026-05-05

- Added Jira-style `gira ticket start|pr|status` as the default daily work facade.
- Added `gira ops` as the advanced command namespace for setup, migration, policy, audit, and raw controls.
- Added the release/versioning foundation for `gira version`, tagged release metadata, npm/bun wrapper distribution, and Homebrew tap updates.
- Added the `gira-cli` PyPI wrapper distribution for pip and pipx installs of the Go-built release binary.

## v0.1.0

- Initial preview release target for the CLI-first Gira workflow.
