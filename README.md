# Gira

Gira is a GitHub-native project OS bootstrapper: it turns a repository into an AI-ready workspace for PRD, issues, milestones, PR workflow, and worker handoff.

Korean shorthand: **기라(Gira): 깃허브로 굴리는 지라.**

## MVP Direction

The Python MVP currently owns the full CLI-first workflow:

- `gira bootstrap --repo OWNER/REPO --template default --dry-run`
- `gira bootstrap --repo OWNER/REPO --path PATH`
- `gira sync --repo OWNER/REPO --dry-run`
- `gira sync --repo OWNER/REPO --dry-run --bootstrap-issues`  # Gira self-bootstrap only
- `gira sync --repo OWNER/REPO`
- `gira sync --repo OWNER/REPO --bootstrap-issues`            # Gira self-bootstrap only
- `gira status --repo OWNER/REPO`

The Go CLI is being introduced in small slices as the long-term product CLI. The current Go path supports bootstrap dry-run/local install, GitHub metadata sync, status, and onboarding verification:

```bash
go run ./cmd/gira bootstrap --repo OWNER/REPO --template default --dry-run
go run ./cmd/gira bootstrap --repo OWNER/REPO --path /path/to/repo
go run ./cmd/gira sync --repo OWNER/REPO --dry-run
go run ./cmd/gira sync --repo OWNER/REPO --dry-run --bootstrap-issues  # Gira self-bootstrap only
go run ./cmd/gira sync --repo OWNER/REPO
go run ./cmd/gira sync --repo OWNER/REPO --bootstrap-issues            # Gira self-bootstrap only
go run ./cmd/gira status --repo OWNER/REPO
go run ./cmd/gira status --repo OWNER/REPO --json
go run ./cmd/gira onboard verify --repo OWNER/REPO --stage init --json
go run ./cmd/gira onboard verify --repo OWNER/REPO --stage steady-state --json
```

Python remains the reference and fallback implementation until final cutover. Do not remove it while Go parity is still being completed.

## Install, Upgrade, and Remove

Gira is implemented as a Go-built CLI, but users should not need Go installed to adopt it. The primary product install path is a versioned install script that downloads the matching GitHub release binary.

### Install Script (Primary)

The official happy path is:

```bash
curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh | sh
```

Pin a version with:

```bash
curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh | GIRA_VERSION=v0.1.0 sh
```

The install script:

- Detect `os` and `arch`, then select the matching GitHub release archive.
- Install `latest` by default and accept an explicit version, for example `GIRA_VERSION=v0.1.0`.
- Download from GitHub release assets, not from a source checkout.
- Verify checksums when the release publishes checksum assets; fail closed if a checksum exists but does not match.
- Install to `${GIRA_INSTALL_DIR}` when set, otherwise to `${HOME}/.local/bin`.
- Print PATH guidance when the install directory is not already on `PATH`.
- Replace an existing `gira` binary in the selected install directory atomically when possible.
- Never modify repository files, GitHub labels, milestones, or issues during binary installation.

Verification:

```bash
gira --help
gira status --repo OWNER/REPO --json
```

### GitHub Release Archives (Manual)

Manual installation uses the same release assets as the install script. Release archive names follow:

```text
gira_VERSION_linux_amd64.tar.gz
gira_VERSION_linux_arm64.tar.gz
gira_VERSION_darwin_amd64.tar.gz
gira_VERSION_darwin_arm64.tar.gz
gira_VERSION_windows_amd64.zip
```

Example:

```bash
version=v0.1.0
curl -fLO "https://github.com/StatPan/gira/releases/download/${version}/gira_${version}_linux_amd64.tar.gz"
tar -xzf "gira_${version}_linux_amd64.tar.gz"
install -m 0755 "gira_${version}_linux_amd64/gira" "${HOME}/.local/bin/gira"
gira --help
```

If checksum assets are published for the release, verify the archive before installing the binary.

### Developer Fallback: Go Install

`go install` remains available for developers and contributors, but it is not the default product onboarding path because it requires Go and module access.

```bash
go install github.com/StatPan/gira/cmd/gira@latest
```

The module is `github.com/StatPan/gira` and the binary package is under `cmd/gira`, so the install path includes `/cmd/gira`. If the repository is private in your environment, configure Go private module access first, for example with `GOPRIVATE=github.com/StatPan/gira` plus normal GitHub authentication.

### Planned Package Channels

Homebrew is the near-term official package-manager target for macOS and Linuxbrew users, but the tap is not yet an available install path.

npm, bun, and `uv` packages are experimental candidate wrapper channels for AI-era developer workflows. These wrappers should install or dispatch the same Go-built release binary rather than reimplementing the CLI. Until those packages exist, use the install script, manual release archive, or developer `go install` path above.

Wrapper packages must preserve the same command surface as the native binary:

```bash
gira --help
gira bootstrap --repo OWNER/REPO --template default --dry-run
gira sync --repo OWNER/REPO --dry-run
gira status --repo OWNER/REPO --json
```

apt/deb packaging is a future target, not an initial official channel. It should wait until usage justifies signing keys, repository hosting, architecture matrix maintenance, and upgrade policy support.

Unsupported distribution channels are source snapshots, unversioned binaries copied from CI artifacts, and package wrappers that do not execute the official Go-built binary.

### Upgrade

Use the same channel that installed Gira:

```bash
curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh | sh
curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh | GIRA_VERSION=v0.1.0 sh
go install github.com/StatPan/gira/cmd/gira@latest
```

When Homebrew, npm, bun, or `uv` wrappers become available, upgrade with the same package manager used for installation.

An upgrade replaces only the local `gira` binary or package wrapper. It must not mutate repository files or GitHub metadata. After upgrading, verify the command still resolves from the expected install location:

```bash
command -v gira
gira --help
gira status --repo OWNER/REPO --json
```

### Binary Uninstall

Binary uninstall removes the local CLI from the machine. It does not detach a repository from Gira and does not delete GitHub labels, milestones, issues, or files.

For install-script or manual installs:

```bash
gira_path="${GIRA_INSTALL_DIR:+${GIRA_INSTALL_DIR}/gira}"
gira_path="${gira_path:-$(command -v gira)}"
rm "${gira_path}"
```

If you installed to a custom directory, set `GIRA_INSTALL_DIR` to that same directory or remove the path printed by `command -v gira`. When Homebrew, npm, bun, or `uv` wrappers become available, uninstall with the same package manager used for installation.

Verification:

```bash
command -v gira
```

If `command -v gira` still prints a path, remove the remaining binary or package from that location.

### Repository Detach

Repository detach is separate from binary uninstall. Detach means removing or disabling Gira-managed repository files and GitHub metadata for one repository while leaving the local `gira` binary installed.

Future command contract:

```bash
gira detach --repo OWNER/REPO --dry-run
gira detach --repo OWNER/REPO --dry-run --json
gira detach --repo OWNER/REPO --apply
```

Default behavior must be dry-run-first:

- `gira detach --repo OWNER/REPO --dry-run` reports the Gira-managed files, labels, milestones, and bootstrap issues that would be removed, archived, or left in place.
- `gira detach --repo OWNER/REPO --dry-run --json` emits the same plan in machine-readable form for review and automation.
- `gira detach --repo OWNER/REPO --apply` performs only the actions shown by the dry-run plan and should require an explicit apply flag.
- Destructive deletion is never default behavior. File deletion, GitHub issue closure, label deletion, and milestone deletion must be opt-in and visible in the dry-run plan before apply.
- Detach must not delete user-authored project history by default. Prefer archiving, closing with an explanatory comment, or leaving unmanaged resources in place unless the operator explicitly requests cleanup.

Verification after detach:

```bash
gira status --repo OWNER/REPO --json
gira sync --repo OWNER/REPO --dry-run
```

`status` should make clear that the repository is not fully Gira-managed. `sync --dry-run` should show what would be recreated if the repository is adopted again.

## Use it today (daily CLI path)

From a fresh shell, make sure the install directory is on `PATH`, then run Gira directly (no source checkout):

```bash
gira --help
gira bootstrap --repo OWNER/REPO --template default --dry-run
gira sync --repo OWNER/REPO --dry-run
gira onboard verify --repo OWNER/REPO --stage init --json
gira onboard verify --repo OWNER/REPO --stage steady-state --json
gira status --repo OWNER/REPO --json
```

This is the canonical operator path for daily use.

For local development from this checkout:

```bash
GOBIN="$(mktemp -d)" go install ./cmd/gira
"${GOBIN}/gira" --help
```

To smoke-test the binary from outside the source checkout:

```bash
GOBIN="$(mktemp -d)" go install ./cmd/gira
(cd /tmp && "${GOBIN}/gira" --help)
(cd /tmp && "${GOBIN}/gira" bootstrap --repo OWNER/REPO --template default --dry-run)
(cd /tmp && "${GOBIN}/gira" bootstrap --repo OWNER/REPO --path /path/to/repo --no-branch)
(cd /tmp && "${GOBIN}/gira" sync --repo OWNER/REPO --dry-run)
(cd /tmp && "${GOBIN}/gira" sync --repo OWNER/REPO --dry-run --bootstrap-issues)
(cd /tmp && "${GOBIN}/gira" status --repo OWNER/REPO --json)
```

## Release Flow

Tagged Go releases are built by `.github/workflows/release.yml`. The workflow runs `go test ./...`, builds Linux, macOS, and Windows CLI archives, and publishes those archives to the GitHub release for tags that start with `v`.

Maintainer flow:

```bash
git checkout main
git pull --ff-only origin main
git tag v0.1.0
git push origin v0.1.0
```

Developer experience conventions for first-run onboarding, dry-run/apply output, JSON, recovery, and the issue-to-PR loop are documented in [docs/dx.md](docs/dx.md).

The GitHub-native Product OS schema for future Projects v2 planning, roadmap date semantics, permission/secret model, and dry-run-first automation is documented in [docs/product-os-schema.md](docs/product-os-schema.md). The execution roadmap for that phase is tracked in [docs/product-os-roadmap.md](docs/product-os-roadmap.md). The Jira-vs-Gira operating boundary, work decomposition contract, and assistant/dev-agent split are documented in [docs/jira-gira-operating-boundary.md](docs/jira-gira-operating-boundary.md). The vendor-neutral dashboard/export boundary for Notion and other consumers is documented in [docs/dashboard-consumer-contract.md](docs/dashboard-consumer-contract.md), and the first concrete export bundle layout is documented in [docs/dashboard-export-artifacts.md](docs/dashboard-export-artifacts.md). The MVP CRUD support contract is documented in [docs/crud-capability-matrix.md](docs/crud-capability-matrix.md). Adoption on pre-configured repositories is documented in [docs/adoption-migration-playbook.md](docs/adoption-migration-playbook.md).

Explicit non-goals for MVP: GitHub Projects v2 automation, LLM PRD-to-issue decomposition, Web UI, Jira import/export, and chat-bot integration.
