# Install Gira

Every channel installs the same Go-built `gira` binary. Package managers are wrappers, not alternate runtimes, and normal release installs do not require Go.

## One-line Installer

```bash
curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh | sh
```

## Package Managers

```bash
npm install -g @statpan/gira
bun install -g @statpan/gira
uv tool install gira-cli
pipx install gira-cli
python -m pip install --user gira-cli
brew tap StatPan/tap
brew install gira
```

## Verify

```bash
gira version
gh auth status
```

Gira uses GitHub as the execution backend and shells through `gh` for GitHub API access.
