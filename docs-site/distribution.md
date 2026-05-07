# Distribution

Gira ships tagged Go release archives. Package managers download or wrap that official binary.

| Channel | Command |
| --- | --- |
| install.sh | `curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh \| sh` |
| npm | `npm install -g @statpan/gira` |
| bun | `bun install -g @statpan/gira` |
| pipx | `pipx install gira-cli` |
| pip | `python -m pip install --user gira-cli` |
| Homebrew | `brew tap StatPan/tap && brew install gira` |

## Upgrade

```bash
gira update
gira upgrade --channel npm
```
