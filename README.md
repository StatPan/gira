# Gira

Gira is a GitHub-native project OS bootstrapper: it turns a repository into an AI-ready workspace for PRD, issues, milestones, PR workflow, and worker handoff.

Korean shorthand: **기라(Gira): 깃허브로 굴리는 지라.**

## MVP Direction

The Python MVP currently owns the full CLI-first workflow:

- `gira bootstrap --repo OWNER/REPO --template default --dry-run`
- `gira sync --repo OWNER/REPO --dry-run`
- `gira sync --repo OWNER/REPO`
- `gira status --repo OWNER/REPO`

The Go CLI is being introduced in small slices as the long-term product CLI. The first Go path supports a bootstrap dry-run preview without touching GitHub or local files:

```bash
go run ./cmd/gira bootstrap --repo OWNER/REPO --template default --dry-run
```

Developer experience conventions for first-run onboarding, dry-run/apply output, JSON, recovery, and the issue-to-PR loop are documented in [docs/dx.md](docs/dx.md).

Explicit non-goals for MVP: GitHub Projects v2 automation, LLM PRD-to-issue decomposition, Web UI, Jira import/export, and chat-bot integration.
