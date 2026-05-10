# Changelog

All notable Gira release changes are tracked here.

Gira uses SemVer tags. User-facing features normally increment the minor version and fixes increment the patch version.

## Unreleased

- Added `gira setup global` to configure global-first operation through one
  dry-run/apply flow, including global defaults, workspace registry, repo
  registry, and global-only versus hybrid repo-local contract modes.

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
