# Visual Portfolio Report

`gira report portfolio` creates a point-in-time, self-contained HTML overview
from existing GitHub and Gira report contracts. It is an explicit local export,
not a hosted dashboard.

```bash
gira report portfolio \
  --repo OWNER/app \
  --repo OWNER/service \
  --milestone "V3 - Active AI PM" \
  --since 2026-07-01 \
  --until 2026-09-30 \
  --output out/portfolio.html
```

`--output` is required. The command writes only that local file. It never
publishes, uploads, serves, refreshes in the background, or opens a browser.
Generation is read-only with respect to GitHub.

## Sections and semantics

| Section | Numerator, denominator, or date | Source contract |
| --- | --- | --- |
| Milestone delivery | GitHub milestone `closed_issues / (open_issues + closed_issues)`; zero total produces 0%, not an inferred forecast | `github-milestones/v1` |
| Milestone schedule | Exact GitHub milestone `due_on`; absent or invalid values render as unknown | `github-milestones/v1` |
| Timeline milestone gate | Exact milestone `due_on` | `github-milestones/v1` |
| Timeline work start | Exact Product OS Project `Start date` | `product-os-project-snapshot/v1` |
| Timeline named gate | Exact Product OS Project `Target date` | `product-os-project-snapshot/v1` |
| Blocked queue | Open GitHub issues with `status:blocked`, `blocked`, or a blocker label | `github-issues/v1` |
| Review-waiting queue | Open PRs without an approved review decision, including Draft and changes-requested PRs | `review-gate/v1` |

The generated page includes the source contract, snapshot time, and trace text
for each progress or timeline surface. Product/outcome confidence is shown as
unsupported because this report does not query a stable outcome-evidence
contract; it is never inferred from issue completion.

## Filters

- Repeat `--repo` and `--milestone` to select an explicit union. Repository
  names are normalized and sorted; milestone titles use exact matching.
- Without `--repo`, Gira resolves the current repository context.
- `--since` and `--until` are inclusive `YYYY-MM-DD` bounds. They filter dated
  timeline entries and queue items with parseable update timestamps.
- Milestone cards remain visible when their due dates fall outside the window;
  the window limits dated events, not the selected milestone inventory.
- Queue items with unavailable timestamps remain visible as unknown. Review PRs
  do not expose a stable milestone relation through `review-gate/v1`, so their
  milestone is displayed as unsupported instead of guessed.

## Partial access and empty states

Issues, milestones, Product OS project dates, and review data are fetched as
separate sources for each repository. If one source is inaccessible, the other
sections still render and the source table records the unavailable contract and
reason. Empty milestone, timeline, blocked, and review sections retain explicit
empty-state text.

The HTML uses semantic headings, native links, a skip link, visible keyboard
focus, labelled progress bars, responsive layouts, and no JavaScript or remote
assets. This keeps the artifact portable and usable offline.
