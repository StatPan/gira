# gira-cli

PyPI distribution wrapper for the official Go-built `gira` binary.

This package does not reimplement Gira in Python. The `gira` console command downloads the matching GitHub Release archive on first run, verifies `checksums.txt`, caches the native binary, and then executes it.

```bash
pip install gira-cli
gira version
```

The PyPI package name is `gira-cli` because the `gira` package name is already occupied on PyPI. The installed command is still `gira`.
