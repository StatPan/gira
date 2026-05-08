# Distribution

Gira ships tagged Go release archives. Package managers download or wrap that official binary. Normal release-binary users do not need Go installed.

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
gira upgrade --channel uv
gira upgrade --channel npm
```

Then upgrade with the same channel that installed Gira, for example `uv tool upgrade gira-cli`, `npm update -g @statpan/gira`, or `brew update && brew upgrade gira`.

## Wrapper Cache Cleanup

uv, pipx, and pip installs use the `gira-cli` PyPI wrapper. The wrapper downloads the matching Go-built release binary on first run and caches it under `GIRA_PYPI_CACHE_DIR` when set, otherwise under `~/.cache/gira-cli/<version>`.

After upgrading, preview and apply stale native binary cache cleanup with:

```bash
gira cache prune --dry-run
gira cache prune --apply
```

`gira cache prune` skips the active version, newer versions, malformed entries, files, symlinks, and any directory containing the current executable. Use `--root PATH` for a custom wrapper cache root or `--json` for automation.
