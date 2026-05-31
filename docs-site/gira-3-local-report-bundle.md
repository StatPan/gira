# Gira 3.0 Local Report Bundle

Gira 3.0 should start with a local report bundle, not a hosted dashboard.

The bundle is a file-based UI surface generated from existing Gira state
contracts. It makes workflow state visible without moving canonical ownership
away from GitHub or replacing the CLI execution loop.

Source document:
[`docs/gira-3-local-report-bundle-ux.md`](https://github.com/StatPan/gira/blob/main/docs/gira-3-local-report-bundle-ux.md).

## Decision

The first 3.0 surface should build on:

```bash
gira export dashboard --repo OWNER/REPO --output out/dashboard --dry-run
gira export dashboard --repo OWNER/REPO --output out/dashboard
gira goal report GOAL --repo OWNER/REPO --html --output out/dashboard/goals/goal-GOAL.html
```

`gira export dashboard` remains the machine contract. The 3.0 layer should add
operator-facing HTML pages and next-command guidance over the exported JSON and
CSV artifacts.

## UX Rules

- Every page shows its source contract and snapshot time.
- Every queue item links back to the GitHub issue or PR.
- Every actionable item shows the next safe Gira command.
- Warnings are visible without opening raw JSON.
- The report is useful offline after generation.
- The report does not require a daemon or hosted backend.
- HTML output escapes GitHub-controlled text.
- JSON remains canonical; HTML is presentation only.

## Bundle Shape

Existing machine artifacts:

```text
out/dashboard/
  manifest.json
  raw/
    github.json
    transitions.json
    capabilities.json
  derived/
    execution_board.json
    roadmap_timeline.json
    warnings.json
  csv/
    execution_items.csv
    roadmap_items.csv
```

3.0 local report additions:

```text
out/dashboard/
  index.html
  goals/
    goal-525.html
  tickets/
    ticket-666.html
  reviews/
    pr-665.html
```

The HTML files are views over Gira-computed state. They are safe to delete and
regenerate.

## Next Slices

- #667 Define [workspace dashboard contract gaps](/workspace-dashboard-contract-gaps) for UI-shaped reports.
- #668 Implement local workspace report bundle export.
- #669 Add ticket detail HTML report from ticket status and review state.
- #670 Add review packet HTML report for human review flow.

Start with #667 so the bundle gets a clear queue contract before HTML starts
rebuilding workflow logic.
