# Sprint And Release

Milestones act as sprint or release boundaries. Gira keeps planning explicit and reviewable.

## Sprint

```bash
gira milestone new "v1.4" --dry-run
gira milestone list --state open
gira milestone status "v1.4"
gira milestone assign "v1.4" --tickets 1,2,3 --dry-run
gira milestone plan "v1.4" --label status:ready --dry-run
gira sprint plan --repo OWNER/REPO --iteration "v1.4" --capacity 3 --issues 1,2,3 --dry-run
gira sprint start --repo OWNER/REPO --iteration "v1.4" --apply
gira sprint rollover --repo OWNER/REPO --dry-run
```

Use `gira milestone new` when a work batch does not exist yet. Use `gira milestone status` to inspect open, closed, ready, in-progress, in-review, blocked, done, and finish-ready ticket counts for a milestone. Use `gira milestone assign` for explicit ticket selection and `gira milestone plan` to select candidates by labels such as `status:ready` before assigning them with dry-run/apply.

## Release

```bash
gira release readiness --repo OWNER/REPO
gira version
gira update
```
