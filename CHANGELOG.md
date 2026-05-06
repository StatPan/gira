# Changelog

All notable Gira release changes are tracked here.

Gira uses SemVer tags. User-facing features normally increment the minor version and fixes increment the patch version.

## Unreleased

- Added `gira ticket finish` to complete the ticket loop with linked PR validation, draft-ready handling, merge blockers, safe merge, local main sync, and convergence reporting.
- Added shorter `gira ticket` daily commands with repo inference, positional ticket numbers, and branch/PR ticket context.

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
