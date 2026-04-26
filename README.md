# Gira

Gira is a GitHub-native project OS bootstrapper: it turns a repository into an AI-ready workspace for PRD, issues, milestones, PR workflow, and worker handoff.

Korean shorthand: **기라(Gira): 깃허브로 굴리는 지라.**

## MVP Direction

The Python MVP currently owns the full CLI-first workflow:

- `gira bootstrap --repo OWNER/REPO --template default --dry-run`
- `gira sync --repo OWNER/REPO --dry-run`
- `gira sync --repo OWNER/REPO`
- `gira status --repo OWNER/REPO`

The Go CLI is being introduced in small slices as the long-term product CLI. The current Go path supports bootstrap dry-run, GitHub metadata sync, and status:

```bash
go run ./cmd/gira bootstrap --repo OWNER/REPO --template default --dry-run
go run ./cmd/gira sync --repo OWNER/REPO --dry-run
go run ./cmd/gira sync --repo OWNER/REPO
go run ./cmd/gira status --repo OWNER/REPO
go run ./cmd/gira status --repo OWNER/REPO --json
```

To smoke-test the binary from outside the source checkout:

```bash
GOBIN=/tmp/gira-bin go install ./cmd/gira
(cd /tmp && /tmp/gira-bin/gira --help)
(cd /tmp && /tmp/gira-bin/gira bootstrap --repo OWNER/REPO --template default --dry-run)
(cd /tmp && /tmp/gira-bin/gira sync --repo OWNER/REPO --dry-run)
(cd /tmp && /tmp/gira-bin/gira status --repo OWNER/REPO --json)
```

Developer experience conventions for first-run onboarding, dry-run/apply output, JSON, recovery, and the issue-to-PR loop are documented in [docs/dx.md](docs/dx.md).

Explicit non-goals for MVP: GitHub Projects v2 automation, LLM PRD-to-issue decomposition, Web UI, Jira import/export, and chat-bot integration.
