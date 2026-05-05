# Release And Distribution

Gira ships as one Go-built binary. Package-manager channels are wrappers around that binary, not alternate runtimes.

## Version Policy

- Stable releases are created from `v*` tags only.
- Published release assets are immutable. Reusing a tag or replacing assets for the same version is a release defect; publish a new patch version instead.
- Pull requests and `main` pushes run validation builds but do not publish stable package-manager releases.
- During `v0.x`, user-facing feature work increments the minor version and fixes increment the patch version.
- Every release should update `CHANGELOG.md` before the tag is pushed.

## Release Flow

1. Merge feature PRs into `main`.
2. Verify `go test ./...`, release workflow checks, and the release readiness gate.
3. Update `CHANGELOG.md`.
4. Tag `main`, for example `git tag v0.1.0 && git push origin v0.1.0`.
5. The GitHub Actions release workflow builds archives, verifies checksums, publishes the GitHub Release, and then publishes configured package-manager channels.

## Channels

### install.sh

`install.sh` is the direct install path. It downloads the tagged GitHub Release archive for the current platform, verifies `checksums.txt`, and installs the binary.

### npm and bun

`@statpan/gira` is an npm-compatible wrapper. It resolves the package version to the matching GitHub tag, downloads the release archive, verifies checksums, and installs the native binary under the package `vendor/` directory. Bun uses the same npm registry package.

Publishing requires `NPM_TOKEN`. If the secret is missing, the release workflow skips npm publishing without blocking the GitHub Release.

### Homebrew

Homebrew publishing targets `StatPan/homebrew-tap`. The release workflow updates `Formula/gira.rb` with the tagged archive URLs and checksums.

Publishing requires `HOMEBREW_TAP_TOKEN`. If the secret is missing, the release workflow skips the tap update without blocking the GitHub Release.

## Required Release Checks

- `gira version` reports the tagged version.
- `gira version --json` includes `version`, `commit`, and `date`.
- `install.sh` succeeds against the release archive and checksum asset.
- npm wrapper tests pass.
- GitHub Release assets include Linux, macOS, Windows archives and `checksums.txt`.
