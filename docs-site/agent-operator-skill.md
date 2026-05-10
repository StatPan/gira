# Agent Operator Skill

The canonical Gira agent/operator skill lives in
[`docs/skills/gira-agent-operator.md`](https://github.com/StatPan/gira/blob/main/docs/skills/gira-agent-operator.md).

Use it as the source of truth for coding agents operating Gira-managed
repositories. Adapter files such as `AGENTS.md`, `CLAUDE.md`,
`.github/copilot-instructions.md`, Cursor rules, and `gira guide agent` should
summarize that skill instead of redefining it.

## Operating Model

- GitHub Issues are executable work packets.
- Branches are work-start evidence.
- Pull requests are change units.
- A merged PR plus a closed linked issue is completion evidence.
- GitHub Projects are visibility surfaces; project-only items must be routed to
  repository issues before implementation.

## Standard Flow

```bash
gh auth status
gira status --repo OWNER/REPO
gira ticket start TICKET --repo OWNER/REPO --dry-run
gira ticket start TICKET --repo OWNER/REPO --apply
go test ./...
gira ticket pr TICKET --repo OWNER/REPO --dry-run
gira ticket pr TICKET --repo OWNER/REPO --apply
gira ticket checks TICKET --repo OWNER/REPO
gira ticket wait TICKET --repo OWNER/REPO --timeout 5m
gira ticket finish TICKET --repo OWNER/REPO --dry-run
gira ticket finish TICKET --repo OWNER/REPO --apply
```

## Raw `gh`

Prefer Gira lifecycle commands for start, PR, checks, wait, and finish. Raw
`gh` is appropriate for authentication checks, extra read-only issue or PR
context, workflow diagnostics, or operations that Gira does not provide yet.

## Drift Prevention

Keep the canonical skill as the source of truth. Keep adapter files short,
refresh generated managed blocks from the canonical text when available, and
update CLI/docs tests whenever lifecycle wording changes.
