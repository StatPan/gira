# Gira

Gira is a GitHub-native project OS bootstrapper: it turns a repository into an AI-ready workspace for PRD, issues, milestones, PR workflow, and worker handoff.

Name shorthand: **Gira: Jira-style project flow on GitHub.**

## Quick Start

Install Gira:

```bash
curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh | sh
```

Or install through a package manager:

```bash
pipx install gira-cli
python -m pip install --user gira-cli
brew tap StatPan/tap
brew install gira
```

Prepare a repository without changing it:

```bash
gira doctor --repo OWNER/REPO
gira ops bootstrap --repo OWNER/REPO --template default --dry-run
gira ops sync --repo OWNER/REPO --dry-run
```

Apply the repository setup after reviewing the dry-run output:

```bash
gira ops bootstrap --repo OWNER/REPO --path /path/to/repo
gira ops sync --repo OWNER/REPO
gira ops onboard verify --repo OWNER/REPO --stage steady-state
```

Run the daily Jira-style ticket loop:

```bash
gira status --repo OWNER/REPO
gira ticket start --repo OWNER/REPO --ticket 12 --dry-run
gira ticket start --repo OWNER/REPO --ticket 12 --apply
gira ticket pr --repo OWNER/REPO --ticket 12 --dry-run
gira ticket pr --repo OWNER/REPO --ticket 12 --apply --draft
gira ticket status --repo OWNER/REPO --ticket 12
```

Use `--json` for automation:

```bash
gira status --repo OWNER/REPO --json
gira ticket status --repo OWNER/REPO --ticket 12 --json
```

## Command Model

The Go-built `gira` binary is the sole product implementation. The default user experience is Jira-style ticket work backed by GitHub issues, PRs, labels, and milestones.

- `gira ticket ...` is the daily issue -> branch -> PR workflow.
- `gira sprint ...`, `gira release`, and `gira status` are daily planning and reporting commands.
- `gira ops ...` contains advanced setup, migration, policy, audit, and raw GitHub controls.
- `gira start` and `gira work ...` remain compatibility aliases.

## Jira To GitHub Mapping

Gira keeps the user-facing workflow close to Jira while storing canonical state in GitHub. The v1 model is intentionally small:

| Jira concept | GitHub object | Gira behavior |
| --- | --- | --- |
| Project | Repository | A repo is the default execution space. Multi-repo work starts as a top-level ticket and is lowered into repo issues when ownership is clear. |
| Epic | Parent or top-level issue | A milestone-sized outcome. Cross-repo epics coordinate child repo issues instead of becoming a separate database record. |
| Story / Task / Bug | Issue | The main work packet. Type, priority, blocked, and status are represented with managed labels and issue metadata. |
| Sprint | Milestone | `gira sprint` plans, starts, closes, and rolls over milestone-scoped work. |
| Status | Labels plus PR evidence | Gira reads and updates status labels, then cross-checks branch, PR, review, and check state. |
| Assignee | GitHub assignee | Ownership stays visible in GitHub and can be supplemented with owner or worker labels. |
| Branch | Issue execution context | `gira ticket start` verifies the ticket, creates or reuses a branch, and moves work to in-progress on apply. |
| Pull request | Change unit | `gira ticket pr` creates or validates a linked PR with a closing keyword such as `Closes #12`. |
| Done | Merged PR plus closed issue | Completion is proven by GitHub merge and close evidence, not by hidden local state. |
| Release | GitHub Release plus readiness report | `gira release readiness` checks whether issue, PR, review, and milestone evidence are ready for delivery. |

GitHub Projects v2 boards, Web UI/TUI, chat bots, LLM decomposition, and Jira import/export are not v1 product workflows. They can be future integration layers, but v1 stays CLI-first on Issues, Labels, Milestones, PRs, and Releases.

## Install, Upgrade, and Remove

Gira is implemented as a Go-built CLI. For v1 users, `install.sh` is the official release install and upgrade path. It installs the Go-built binary from GitHub release assets; it does not build from source and does not mutate any repository.

### Install Script

Install the latest tagged release:

```bash
curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh | sh
```

Pin a version:

```bash
curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh | GIRA_VERSION=v1.0.0 sh
```

Install to a custom directory:

```bash
curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh | GIRA_INSTALL_DIR="${HOME}/bin" sh
```

The install script:

- Detects `os` and `arch`, then selects the matching GitHub release archive.
- Installs `latest` by default and accepts an explicit version, for example `GIRA_VERSION=v1.0.0`.
- Downloads from GitHub release assets, not from a source checkout or CI artifact.
- Requires the release `checksums.txt` asset and verifies the selected archive before unpacking it.
- Installs to `${GIRA_INSTALL_DIR}` when set, otherwise to `${HOME}/.local/bin`.
- Prints PATH guidance when the install directory is not already on `PATH`.
- Replaces an existing `gira` binary in the selected install directory atomically when possible.
- Never modifies repository files, GitHub labels, milestones, or issues during binary installation.

Verification:

```bash
command -v gira
gira --help
gira version
gira doctor --repo OWNER/REPO
```

### Developer Go Install

Use `go install` for source and development workflows, not as the preferred v1 user install path:

```bash
go install github.com/StatPan/gira/cmd/gira@latest
```

The module is `github.com/StatPan/gira` and the binary package is under `cmd/gira`, so the install path includes `/cmd/gira`. If the repository is private in your environment, configure Go private module access first, for example with `GOPRIVATE=github.com/StatPan/gira` plus normal GitHub authentication.

From this checkout, build and install the current source version:

```bash
GOBIN="${HOME}/.local/bin" go install ./cmd/gira
```

Use a temporary install directory for smoke tests:

```bash
GOBIN="$(mktemp -d)" go install ./cmd/gira
"${GOBIN}/gira" --help
"${GOBIN}/gira" version
```

### GitHub Release Archives

Manual release-archive installation uses the same release assets as the install script. Release archive names follow:

```text
gira_VERSION_linux_amd64.tar.gz
gira_VERSION_linux_arm64.tar.gz
gira_VERSION_darwin_amd64.tar.gz
gira_VERSION_darwin_arm64.tar.gz
gira_VERSION_windows_amd64.zip
```

Example:

```bash
version=v1.0.0
curl -fLO "https://github.com/StatPan/gira/releases/download/${version}/gira_${version}_linux_amd64.tar.gz"
tar -xzf "gira_${version}_linux_amd64.tar.gz"
install -m 0755 "gira_${version}_linux_amd64/gira" "${HOME}/.local/bin/gira"
gira --help
```

Verify the archive with the release `checksums.txt` before installing the binary. Every v1 release should publish `checksums.txt`; treat a missing checksum asset as a release defect.

### Wrapper Distribution Boundaries

Package-manager wrappers such as Homebrew, npm, bun, pip, pipx, or `uv` are allowed only as distribution channels for the Go-built release binary. They must not reimplement Gira in another runtime, change the command surface, or install unversioned CI artifacts.

The npm/bun wrapper package is maintained under `packages/npm`, and the PyPI wrapper package is maintained under `packages/pypi`. Homebrew publishing targets the external `StatPan/homebrew-tap` repository. These channels install the Go-built release binary and verify release checksums.

Wrapper packages must preserve the same command surface as the native binary:

```bash
gira version
gira ticket start --repo OWNER/REPO --ticket 12 --dry-run
gira ops bootstrap --repo OWNER/REPO --template default --dry-run
gira ops sync --repo OWNER/REPO --dry-run
gira status --repo OWNER/REPO --json
```

Official install channels:

```bash
python -m pip install --user gira-cli
pipx install gira-cli
brew tap StatPan/tap
brew install gira
```

The pip and pipx channel installs the `gira-cli` wrapper from PyPI. The Homebrew channel is published through `StatPan/homebrew-tap`.

Pending wrapper channels:

```bash
npm install -g @statpan/gira
bun install -g @statpan/gira
```

The npm and bun channel will install the same Go-built release binary through the npm registry after npm publishing is enabled for `@statpan/gira`.

apt/deb packaging is a future target, not an initial official channel. It should wait until usage justifies signing keys, repository hosting, architecture matrix maintenance, and upgrade policy support.

Unsupported distribution channels are source snapshots, unversioned binaries copied from CI artifacts, package wrappers that do not execute the official Go-built binary, and alternate product runtimes.

### Upgrade

Use the same channel that installed Gira:

```bash
curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh | sh
curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh | GIRA_VERSION=v1.0.0 sh
go install github.com/StatPan/gira/cmd/gira@latest
GOBIN="${HOME}/.local/bin" go install ./cmd/gira
python -m pip install --user --upgrade gira-cli
pipx upgrade gira-cli
brew update && brew upgrade gira
```

Upgrade with the same package manager used for installation.

An upgrade replaces only the local `gira` binary or package wrapper. It must not mutate repository files or GitHub metadata. After upgrading, verify the command still resolves from the expected install location:

```bash
command -v gira
gira --help
gira version
gira doctor --repo OWNER/REPO
```

### Binary Uninstall

Binary uninstall removes the local CLI from the machine. It does not detach a repository from Gira and does not delete GitHub labels, milestones, issues, or files.

For install-script or manual installs:

```bash
gira_path="${GIRA_INSTALL_DIR:+${GIRA_INSTALL_DIR}/gira}"
gira_path="${gira_path:-$(command -v gira)}"
rm "${gira_path}"
```

If you installed to a custom directory, set `GIRA_INSTALL_DIR` to that same directory or remove the path printed by `command -v gira`. For npm, bun, pip, pipx, or Homebrew installs, uninstall with the same package manager used for installation.

Verification:

```bash
command -v gira
```

If `command -v gira` still prints a path, remove the remaining binary or package from that location.

### Repository Detach

Repository detach is separate from binary uninstall. Detach means removing or disabling Gira-managed repository files and GitHub metadata for one repository while leaving the local `gira` binary installed.

Future command contract:

```bash
gira ops detach --repo OWNER/REPO --dry-run
gira ops detach --repo OWNER/REPO --dry-run --json
gira ops detach --repo OWNER/REPO --apply
```

Default behavior must be dry-run-first:

- `gira ops detach --repo OWNER/REPO --dry-run` reports the Gira-managed files, labels, milestones, and bootstrap issues that would be removed, archived, or left in place.
- `gira ops detach --repo OWNER/REPO --dry-run --json` emits the same plan in machine-readable form for review and automation.
- `gira ops detach --repo OWNER/REPO --apply` performs only the actions shown by the dry-run plan and should require an explicit apply flag.
- Destructive deletion is never default behavior. File deletion, GitHub issue closure, label deletion, and milestone deletion must be opt-in and visible in the dry-run plan before apply.
- Detach must not delete user-authored project history by default. Prefer archiving, closing with an explanatory comment, or leaving unmanaged resources in place unless the operator explicitly requests cleanup.

Verification after detach:

```bash
gira status --repo OWNER/REPO --json
gira ops sync --repo OWNER/REPO --dry-run
```

`status` should make clear that the repository is not fully Gira-managed. `sync --dry-run` should show what would be recreated if the repository is adopted again.

## Daily Happy Path

From a fresh shell, make sure the install directory is on `PATH`, then run Gira directly (no source checkout):

```bash
gira --help
gira ops sync --repo OWNER/REPO --dry-run
gira ops onboard verify --repo OWNER/REPO --stage steady-state
gira status --repo OWNER/REPO
gira ticket start --repo OWNER/REPO --ticket 12 --dry-run
gira ticket pr --repo OWNER/REPO --ticket 12 --dry-run
gira ticket status --repo OWNER/REPO --ticket 12
```

This is the canonical operator path for daily use.

Use `--json` for automation only; human output ends with a concise `next step:` line where Gira can suggest a safe continuation.

`gira start` and `gira work start|pr|status` remain compatibility aliases for users and scripts that already adopted the earlier issue-oriented wording. New documentation uses `ticket` because the intended mental model is Jira-style tickets mapped onto GitHub issues.

## Advanced Adoption And Migration

Use the bootstrap and policy-mode commands when introducing Gira to a new or already-configured repository:

```bash
gira ops bootstrap --repo OWNER/REPO --template default --dry-run
gira ops bootstrap --repo OWNER/REPO --path /path/to/repo
gira ops sync --repo OWNER/REPO --dry-run --policy-mode adopt
gira ops sync --repo OWNER/REPO --dry-run --policy-mode merge
gira ops sync --repo OWNER/REPO --dry-run --policy-mode enforce
gira ops onboard verify --repo OWNER/REPO --stage init --json
gira status --repo OWNER/REPO --json
```

Package-manager wrappers such as `uv`, npm, bun, or Homebrew may be added as distribution channels when they install or invoke the Go-built `gira` binary. They are not alternate product implementations.

For local development from this checkout:

```bash
GOBIN="$(mktemp -d)" go install ./cmd/gira
"${GOBIN}/gira" --help
```

To smoke-test the binary from outside the source checkout:

```bash
GOBIN="$(mktemp -d)" go install ./cmd/gira
(cd /tmp && "${GOBIN}/gira" --help)
(cd /tmp && "${GOBIN}/gira" version)
(cd /tmp && "${GOBIN}/gira" ops bootstrap --repo OWNER/REPO --template default --dry-run)
(cd /tmp && "${GOBIN}/gira" ops bootstrap --repo OWNER/REPO --path /path/to/repo --no-branch)
(cd /tmp && "${GOBIN}/gira" ops sync --repo OWNER/REPO --dry-run)
(cd /tmp && "${GOBIN}/gira" ops sync --repo OWNER/REPO --dry-run --bootstrap-issues)
(cd /tmp && "${GOBIN}/gira" status --repo OWNER/REPO --json)
```

## Release Flow

Tagged Go releases are built by `.github/workflows/release.yml`. Pull requests and `main` pushes run validation builds only. Tags that start with `v` publish stable GitHub Release assets, then publish configured package-manager channels.

The workflow checks `install.sh` syntax, runs `go test ./...`, runs npm and PyPI wrapper tests, builds Linux, macOS, and Windows CLI archives with version metadata, generates `checksums.txt`, verifies the checksum manifest, and publishes those assets to the GitHub release. Published release assets are treated as immutable; rerun with a new patch tag instead of replacing an existing release. If `NPM_TOKEN` is configured, it publishes `@statpan/gira`. If `PYPI_API_TOKEN` is configured, it publishes `gira-cli`. If `HOMEBREW_TAP_TOKEN` is configured, it updates `StatPan/homebrew-tap`.

Maintainer flow:

```bash
git checkout main
git pull --ff-only origin main
$EDITOR CHANGELOG.md
git tag -a v1.0.0 -m "gira v1.0.0"
git push origin v1.0.0
```

Expected release assets:

```text
checksums.txt
gira_VERSION_linux_amd64.tar.gz
gira_VERSION_linux_arm64.tar.gz
gira_VERSION_darwin_amd64.tar.gz
gira_VERSION_darwin_arm64.tar.gz
gira_VERSION_windows_amd64.zip
```

After publishing, smoke-test the official installer against the tag:

```bash
tmpdir="$(mktemp -d)"
GIRA_INSTALL_DIR="${tmpdir}" GIRA_VERSION=v1.0.0 sh install.sh
"${tmpdir}/gira" --help
"${tmpdir}/gira" version
```

The release policy and package-manager channel details are documented in [docs/release-distribution.md](docs/release-distribution.md). Developer experience conventions for first-run onboarding, dry-run/apply output, JSON, recovery, and the issue-to-PR loop are documented in [docs/dx.md](docs/dx.md).

The GitHub-native Product OS schema for future Projects v2 planning, roadmap date semantics, permission/secret model, and dry-run-first automation is documented in [docs/product-os-schema.md](docs/product-os-schema.md). The execution roadmap for that phase is tracked in [docs/product-os-roadmap.md](docs/product-os-roadmap.md). The Jira-vs-Gira operating boundary, work decomposition contract, and assistant/dev-agent split are documented in [docs/jira-gira-operating-boundary.md](docs/jira-gira-operating-boundary.md). The portfolio intake layer for top-level tickets and multi-repo lowering plans is documented in [docs/portfolio-intake.md](docs/portfolio-intake.md). The vendor-neutral dashboard/export boundary for Notion and other consumers is documented in [docs/dashboard-consumer-contract.md](docs/dashboard-consumer-contract.md), and the first concrete export bundle layout is documented in [docs/dashboard-export-artifacts.md](docs/dashboard-export-artifacts.md). The MVP CRUD support contract is documented in [docs/crud-capability-matrix.md](docs/crud-capability-matrix.md). Adoption on pre-configured repositories is documented in [docs/adoption-migration-playbook.md](docs/adoption-migration-playbook.md).

This repository dogfoods Gira for its own work. The active operating loop, sprint commands, and maintainer handoff are documented in [docs/dogfood.md](docs/dogfood.md).

Explicit non-goals for v1: GitHub Projects v2 automation, LLM PRD-to-issue decomposition, Web UI/TUI, chat-bot integration, and Jira import/export. Gira may keep compatibility or migration inspection commands behind `gira ops`, but the v1 source of truth is the GitHub execution loop.
