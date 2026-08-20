# Readiness And Audit

Gira exposes readiness and audit reports so humans and agents can make decisions
from GitHub evidence instead of terminal guesses. These commands are read-first
unless an explicit lifecycle command says `--apply`.

## Operation Policy

Every readiness report exposes the resolved policy, its source, each finding's
class, and whether the finding is enforced. The same report shape works across
the three supported operating modes:

| Policy | Gira-specific requirements | Provider facts |
| --- | --- | --- |
| `observation` | Advisory only; labels, closing links, and approval conventions do not block. | Reported neutrally. |
| `managed` + `delivery_policy: advisory` | Reported as warnings with provenance. | Still reported and never hidden. |
| `managed` + `delivery_policy: required` | Enforced as readiness blockers where applicable. | Unknown/provider errors fail closed. |

This distinction keeps an unenrolled repository observable without silently
enrolling it in Gira's delivery process. Use `gira status --json` or
`gira doctor --json` to inspect policy provenance before applying a mutation.

## Ticket Readiness

```bash
gira ticket status --json
gira ticket status 42 --repo OWNER/app --html --output out/gira/ticket-42.html
gira ticket view
```

`ticket status --json` emits the stable `ticket-status/v1` contract. The report
includes labels, milestone, branch policy, linked PR, checks, review state,
evidence, warnings, and nested readiness reports.
`ticket status --html --output PATH` writes the same state as a static local
ticket detail page for human review.
Run these from the `issue-N-*` branch for the daily path. Pass an explicit
ticket and repo only when auditing from detached context.

The `ticket_readiness` object uses `ticket-readiness/v1` to say whether an issue
is ready to start, needs refinement, is blocked, or needs human input before
handoff.

## PR And Review Readiness

```bash
gira ticket review --diff-summary --json
gira ticket review 42 --repo OWNER/app --diff-summary --html --output out/gira/review-42.html
gira ticket checks
gira ticket wait --timeout 5m
```

`pr-readiness/v1` checks whether worker output is reviewable or should wait,
revise, request review, finish, or stop. Review packets include the linked PR,
changed-file context, checks, finish readiness, evidence fields, and a verdict
schema for reviewer judgment. `ticket review --html --output PATH` writes that
packet as a static local review page.

## Finish Readiness And Receipts

```bash
gira ticket finish --dry-run --json
```

`finish-readiness/v1` is the final merge and closure gate. It verifies linked PR
state, check status, review state, label normalization, closing-reference
evidence, acceptance checklist counts, warnings, and blockers before merge or
receipt posting. Apply posts a concise `finish-receipt/v1` comment after
successful completion.

## Drift Audit

```bash
gira audit drift --repo OWNER/app --json
```

Use drift audit when repo workflow state does not look converged. It reports
stale status labels, open `status:done` issues, multiple status labels,
in-review issues without linked PRs, merged PRs whose issues did not converge,
failed or pending checks on finished tickets, and missing telemetry or finish
evidence.

## Provider Doctor

```bash
gira jira doctor --repo OWNER/app --sample-key ABC-123 --json
```

In Jira-primary mode, provider doctor validates configuration, credentials,
sample issue reachability, transition readiness, required fields, and GitHub
mirror compatibility without mutating Jira or GitHub.

## Related Pages

- [Ticket Workflow](/ticket-workflow)
- [Jira-Primary Provider](/jira-primary-provider)
- [Troubleshooting](/troubleshooting)
- [Command Reference](/command-reference)
