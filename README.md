# Gira

Gira is a GitHub-native project OS bootstrapper: it turns a repository into an AI-ready workspace for PRD, issues, milestones, PR workflow, and worker handoff.

Korean shorthand: **기라(Gira): 깃허브로 굴리는 지라.**

## MVP Direction

The Python MVP currently owns the full CLI-first workflow:

- `gira bootstrap --repo OWNER/REPO --template default --dry-run`
- `gira bootstrap --repo OWNER/REPO --path PATH`
- `gira sync --repo OWNER/REPO --dry-run`
- `gira sync --repo OWNER/REPO`
- `gira status --repo OWNER/REPO`

The Go CLI is being introduced in small slices as the long-term product CLI. The current Go path supports bootstrap dry-run/local install, GitHub metadata sync, and status:

```bash
go run ./cmd/gira bootstrap --repo OWNER/REPO --template default --dry-run
go run ./cmd/gira bootstrap --repo OWNER/REPO --path /path/to/repo
go run ./cmd/gira sync --repo OWNER/REPO --dry-run
go run ./cmd/gira sync --repo OWNER/REPO
go run ./cmd/gira status --repo OWNER/REPO
go run ./cmd/gira status --repo OWNER/REPO --json
```

Python remains the reference and fallback implementation until final cutover. Do not remove it while Go parity is still being completed.

## Install

Install the daily Go CLI from the module source:

```bash
go install github.com/StatPan/gira/cmd/gira@latest
```

The module is `github.com/StatPan/gira` and the binary package is under `cmd/gira`, so the install path includes `/cmd/gira`. If the repository is private in your environment, configure Go private module access first, for example with `GOPRIVATE=github.com/StatPan/gira` plus normal GitHub authentication. After install, make sure your Go binary directory is on `PATH`:

```bash
gira --help
```

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

The GitHub-native Product OS schema for future Projects v2 planning, roadmap date semantics, permission/secret model, and dry-run-first automation is documented in [docs/product-os-schema.md](docs/product-os-schema.md). The execution roadmap for that phase is tracked in [docs/product-os-roadmap.md](docs/product-os-roadmap.md). The vendor-neutral dashboard/export boundary for Notion and other consumers is documented in [docs/dashboard-consumer-contract.md](docs/dashboard-consumer-contract.md).

Explicit non-goals for MVP: GitHub Projects v2 automation, LLM PRD-to-issue decomposition, Web UI, Jira import/export, and chat-bot integration.
