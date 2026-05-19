# Troubleshooting

## GitHub auth fails

```bash
gh auth status
gh auth login
```

## Repo context is missing

Run from a GitHub checkout, pass `--repo OWNER/REPO`, or set `repo` in `.gira/config.yaml`.

## Ticket context is missing

Run from an `issue-N-*` branch or pass `--ticket N`. After `gira ticket start`, most ticket commands infer the ticket.

## Workflow context errors

Gira errors should name the missing or ambiguous context and print the next safe command instead of guessing.

- Missing ticket context: pass `--ticket N`, run from an `issue-N-*` branch, or open a PR with `Closes #N`.
- Ambiguous ticket context: inspect the printed candidates and re-run with `--ticket N`.
- Missing PR context: run `gira ticket pr --dry-run` to preview the linked PR step before applying it.
- Missing milestone title: run `gira milestone list --repo OWNER/REPO`, then re-run the target command with the exact milestone title.
- Missing `status:ready`: confirm the issue is executable, then use the printed `gira adopt issues ... --label status:ready --apply` command.

## Custom domain

The Pages artifact includes `CNAME` for `gira.statpan.com`. DNS should point that host to GitHub Pages according to GitHub's Pages domain setup guide.
