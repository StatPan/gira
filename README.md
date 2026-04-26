# Gira

Gira is a GitHub-native project OS bootstrapper: it turns a repository into an AI-ready workspace for PRD, issues, milestones, PR workflow, and worker handoff.

Korean shorthand: **기라(Gira): 깃허브로 굴리는 지라.**

## MVP Direction

The MVP focuses on a CLI-first workflow:

- `gira bootstrap --repo OWNER/REPO --template default --dry-run`
- `gira sync --repo OWNER/REPO --dry-run`
- `gira sync --repo OWNER/REPO`
- `gira status --repo OWNER/REPO`

Explicit non-goals for MVP: GitHub Projects v2 automation, LLM PRD-to-issue decomposition, Web UI, Jira import/export, and chat-bot integration.
