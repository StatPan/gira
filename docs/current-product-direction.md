# Current Product Direction

Gira 3.x is a GitHub-native, Go-built CLI for running issue, branch, pull
request, review, and release work with explicit evidence.

## Now

- GitHub Issues and pull requests are the execution source of truth.
- Lifecycle commands are dry-run-first and produce inspectable readiness and
  completion receipts.
- Distribution remains one Go binary through GitHub Releases and thin package
  wrappers.

## Next

- Harden generated configuration contracts, release smoke tests, docs, and
  compatibility of machine-readable CLI reports.

## Not now

- Hosted dashboards, a web UI or TUI, hosted agent execution, Jira automation,
  and LLM PRD decomposition are outside the current product boundary.

The [Product OS Roadmap](product-os-roadmap.md) and
[v2 release-readiness package](v2-release-readiness.md) are historical records
of the 2.0 stabilization line, not the current delivery roadmap.
