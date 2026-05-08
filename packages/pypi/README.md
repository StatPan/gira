# gira-cli

PyPI distribution wrapper for the official Go-built `gira` binary.

This package does not reimplement Gira in Python. The `gira` console command downloads the matching GitHub Release archive on first run, verifies `checksums.txt`, caches the native binary, and then executes it.

```bash
uv tool install gira-cli
gira version
```

or:

```bash
pip install gira-cli
gira version
```

The PyPI package name is `gira-cli` because the `gira` package name is already occupied on PyPI. The installed command is still `gira`.

## Native Binary Cache

The wrapper stores downloaded native binaries under `GIRA_PYPI_CACHE_DIR` when set, otherwise under `~/.cache/gira-cli/<version>`.

Preview and remove stale cached versions after an upgrade:

```bash
gira cache prune --dry-run
gira cache prune --apply
```

The prune command removes only older stable semver release directories. It skips the active version, newer versions, malformed entries, files, symlinks, and any directory containing the current executable. Use `--root PATH` for a custom cache root and `--json` for machine-readable output.

- Repository: https://github.com/StatPan/gira
- Issues: https://github.com/StatPan/gira/issues
- Contact: statpan@outlook.com
