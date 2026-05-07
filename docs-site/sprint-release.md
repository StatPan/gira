# Sprint And Release

Milestones act as sprint or release boundaries. Gira keeps planning explicit and reviewable.

## Sprint

```bash
gira sprint plan --repo OWNER/REPO --iteration "v1.4" --capacity 3 --issues 1,2,3 --dry-run
gira sprint start --repo OWNER/REPO --iteration "v1.4" --apply
gira sprint rollover --repo OWNER/REPO --dry-run
```

## Release

```bash
gira release readiness --repo OWNER/REPO
gira version
gira update
```
