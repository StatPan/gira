# Release And Distribution

Gira ships as one Go-built binary. Package-manager channels are wrappers around that binary, not alternate runtimes.

## Version Policy

- Stable releases are created from `v*` tags only.
- Publishing workflows accept only `vMAJOR.MINOR.PATCH` release tags. Treat tag
  names, refs, and PR content as untrusted input until the workflow has
  validated the tag shape.
- Published release assets are immutable. Reusing a tag or replacing assets for the same version is a release defect; publish a new patch version instead.
- Pull requests and `main` pushes run validation builds but do not publish stable package-manager releases.
- During `v0.x`, user-facing feature work increments the minor version and fixes increment the patch version.
- Every release should update `CHANGELOG.md` before the tag is pushed.

## Release Flow

1. Merge feature PRs into `main`.
2. Verify `go test ./...`, release workflow checks, and the release readiness gate.
3. Update `CHANGELOG.md`.
4. Tag `main`, for example `git tag -a v1.0.0 -m "gira v1.0.0" && git push origin v1.0.0`.
5. The GitHub Actions release workflow builds archives, verifies checksums, publishes the GitHub Release, and then publishes configured package-manager channels.
6. Smoke-test the tagged installer and any published package-manager channels before advertising the new version.
7. Optionally capture an adoption signal snapshot for the release using
   `docs/adoption-signal-model.md`. The snapshot is directional evidence only;
   do not convert package downloads into active-user claims.

The README on `main` should describe the next public release surface only after that release is tagged. If `main` documents commands that are not in the latest stable tag, cut a new minor or patch release before directing users to the installer.

## Channels

### install.sh

`install.sh` is the direct install path. It downloads the tagged GitHub Release archive for the current platform, verifies `checksums.txt`, and installs the binary.

### npm and bun

`@statpan/gira` is an npm-compatible wrapper. It resolves the package version to the matching GitHub tag, downloads the release archive, verifies checksums, and installs the native binary under the package `vendor/` directory. Bun uses the same npm registry package.

Publishing requires `NPM_TOKEN`. If the secret is missing, the release workflow skips npm publishing without blocking the GitHub Release.

### PyPI

`gira-cli` is a Python packaging wrapper for uv, pip, and pipx installs. It resolves the package version to the matching GitHub tag, downloads the release archive on first command execution, verifies `checksums.txt`, and caches the native binary under `~/.cache/gira-cli` by default.

The native binary cache root is `GIRA_PYPI_CACHE_DIR` when set, otherwise `~/.cache/gira-cli`. Each release is cached under `<root>/<version>`. Stale wrapper-managed release caches can be previewed and removed with:

```bash
gira cache prune --dry-run
gira cache prune --apply
```

The prune command only removes direct child stable semver release directories older than the active Gira version. It skips the active version, newer versions, malformed entries, files, symlinks, and any directory containing the current executable. Use `--root PATH` for a custom cache root and `--json` in automation.

Agents that need release awareness should call `gira update --notify-once --json`.
When a newer stable release exists, the JSON report includes a `notice` object
with `kind: new_version`, the latest version, and a channel-specific `next_step`.
The once marker is local state only; Gira still does not run package managers or
mutate repositories from the upgrade path.

Normal `gira` commands also run a passive, rate-limited release check and emit a
one-time stderr notice for a new latest release. This gives AI agents an update
signal during ordinary CLI usage without corrupting JSON stdout. Set
`GIRA_UPDATE_NOTICE=off` or `GIRA_DISABLE_UPDATE_NOTICE=1` to opt out.

Publishing requires `PYPI_API_TOKEN`. If the secret is missing, the release workflow skips PyPI publishing without blocking the GitHub Release.

### Homebrew

Homebrew publishing targets `StatPan/homebrew-tap`. The release workflow updates `Formula/gira.rb` with the tagged archive URLs and checksums.

Publishing requires `HOMEBREW_TAP_TOKEN`. If the secret is missing, the release workflow skips the tap update without blocking the GitHub Release.

## Workflow Trust Boundary

Pull requests and `main` pushes run build and documentation validation without
package-manager publish authority. GitHub Pages deploy permissions are granted
only to the deploy job on `push`, not to the pull-request docs build. Package
publish tokens are scoped to the token check and publish steps instead of the
entire job.

Before publishing, the release workflow validates the tag name and passes release
metadata to scripts as data through environment variables. Do not interpolate
unchecked refs into generated package metadata or release scripts.

## Required Release Checks

- `gira version` reports the tagged version.
- `gira version --json` includes `version`, `commit`, and `date`.
- `install.sh` succeeds against the release archive and checksum asset.
- npm wrapper tests pass.
- PyPI wrapper tests pass.
- GitHub Release assets include Linux, macOS, Windows archives and `checksums.txt`.

## Adoption Snapshot

After publishing or before a public announcement, maintainers may capture a
small adoption signal snapshot:

```bash
curl -fsSL 'https://api.npmjs.org/downloads/point/last-week/@statpan/gira'
curl -fsSL 'https://pypistats.org/api/packages/gira-cli/recent'
gh api repos/StatPan/gira/releases --paginate
gh api repos/StatPan/gira/traffic/views
gh api repos/StatPan/gira/traffic/clones
gh api repos/StatPan/gira
```

Use the snapshot to compare release channels and docs interest. It is not a
product analytics system and must not include user-identifying telemetry.
