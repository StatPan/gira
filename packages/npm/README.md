# @statpan/gira

npm and bun distribution wrapper for the official Go-built `gira` binary.

This package does not reimplement Gira in JavaScript. During installation it downloads the matching GitHub Release archive, verifies `checksums.txt`, and installs the native binary under the package `vendor/` directory.

```bash
npm install -g @statpan/gira
gira version
```

`bun install -g @statpan/gira` uses the same npm-compatible package.
