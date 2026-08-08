# Distribution

Gira ships tagged Go release archives. Package managers download or wrap that official binary. Normal release-binary users do not need Go installed.

## Release Impact

New `story` tickets default to `user-facing` release impact. Gira copies that
decision to the linked PR, and CI requires an `Unreleased` entry in
`CHANGELOG.md` in the same PR. Internal work can declare
`--release-impact internal`; an exemption needs `--release-impact-reason`.

| Channel | Command |
| --- | --- |
| install.sh | `curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh \| sh` |
| npm | `npm install -g @statpan/gira` |
| bun | `bun install -g @statpan/gira` |
| uv | `uv tool install gira-cli` |
| pipx | `pipx install gira-cli` |
| pip | `python -m pip install --user gira-cli` |
| Homebrew | `brew tap StatPan/tap && brew install gira` |

## Upgrade

`gira update` and `gira upgrade` are advisory. They check the latest GitHub release and print the channel-specific command; they do not run package managers or mutate repositories.

```bash
gira update
gira update --notify-once --json
gira upgrade --channel uv
gira upgrade --channel npm
```

Then upgrade with the same channel that installed Gira, for example `uv tool upgrade gira-cli`, `npm update -g @statpan/gira`, or `brew update && brew upgrade gira`.

Use `--notify-once` when an agent or shell startup wants one local notice per
new latest release. The command writes only a small marker under the Gira global
config root and keeps printing the same `next_step` command; it never runs the
package manager for you.

Normal `gira` commands also perform a passive, rate-limited release check and
print the same once-per-version notice to stderr when a newer release exists.
This keeps JSON stdout parseable while giving AI agents a signal they can act
on during ordinary CLI usage. Set `GIRA_UPDATE_NOTICE=off` or
`GIRA_DISABLE_UPDATE_NOTICE=1` to disable passive notices.

## Wrapper Cache Cleanup

uv, pipx, and pip installs use the `gira-cli` PyPI wrapper. The wrapper downloads the matching Go-built release binary on first run and caches it under `GIRA_PYPI_CACHE_DIR` when set, otherwise under `~/.cache/gira-cli/<version>`.

After upgrading, preview and apply stale native binary cache cleanup with:

```bash
gira cache prune --dry-run
gira cache prune --apply
```

`gira cache prune` skips the active version, newer versions, malformed entries, files, symlinks, and any directory containing the current executable. Use `--root PATH` for a custom wrapper cache root or `--json` for automation.

## Adoption Signals

Distribution metrics are directional. npm, PyPI, and GitHub Release downloads
show channel reach and binary pulls, not active users.

Use the [Adoption Signals](/adoption-signals) model when a release needs a
manual snapshot across npm `@statpan/gira`, PyPI `gira-cli`, GitHub Release
assets, repository traffic, and privacy-safe docs measurement.
