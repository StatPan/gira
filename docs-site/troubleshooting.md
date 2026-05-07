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

## Custom domain

The Pages artifact includes `CNAME` for `gira.statpan.com`. DNS should point that host to GitHub Pages according to GitHub's Pages domain setup guide.
