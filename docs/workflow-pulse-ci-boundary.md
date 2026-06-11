# Workflow Pulse And CI Reliability Boundary

## Status

Research boundary for #724.

## Decision

Gira should expose workflow pulse and CI reliability signals as operational readiness evidence, not as productivity rankings, agent leaderboards, or a broad analytics dashboard.

The useful question is whether work is moving safely toward finish. The wrong question is which person or agent is "more productive" based on derived queue, CI, or review telemetry.

## Accepted Signal Families

Gira can compute and expose signals that help operators and agents decide the next safe action.

| Signal family | Purpose | Source |
| --- | --- | --- |
| Finish readiness | Decide whether a ticket can be merged, closed, or receipted. | `finish-readiness/v1`, PR state, checks, reviews, labels, branch trust. |
| Failed-check attention | Identify work that needs correction before finish. | PR check conclusions and `workspace-queues/v1` failed/check lanes. |
| Review movement | Distinguish waiting-for-review, requested-changes, approved, and stale review states. | GitHub PR reviews and `pr-readiness/v1`. |
| Queue movement | Show whether work is ready, in progress, blocked, in review, finish-ready, failed, or waiting on a human. | `workspace-queues/v1` and queue views. |
| Handoff readiness | Decide whether a worker can continue from a ticket or queue handoff. | `worker-handoff/v1` readiness, blockers, and stop reasons. |
| Receipt completeness | Show whether completion or handoff decisions were recorded durably. | Finish receipts, goal receipts, supersede notes, handoff comments. |
| Drift and missing evidence | Identify inconsistencies between labels, branches, PRs, checks, comments, and local config. | Gira audit/readiness commands. |

These signals should stay tied to concrete next actions such as wait, review, revise, refine ticket, finish, ask human, or inspect drift.

## Rejected Uses

Gira should explicitly reject these uses for the first product surface:

- personal productivity ranking;
- agent leaderboard scoring;
- velocity scoring by individual;
- comparing humans against models;
- hidden performance surveillance;
- broad executive analytics dashboards;
- CI flake blame assignment by person or agent;
- model quality ranking based only on workflow metadata.

The data Gira sees is workflow evidence, not a fair measurement of effort, difficulty, interruption, review quality, or product value.

## CLI, Export, And Projects Boundary

Workflow pulse belongs first in CLI JSON and optional export artifacts.

| Surface | Decision |
| --- | --- |
| CLI JSON | Accepted. Keep signals close to `status`, `queue`, `ticket checks`, `ticket finish --dry-run`, `audit`, and `stats pulse` outputs. |
| Local/export artifacts | Accepted when reproducible from GitHub plus Gira config. Useful for reports, audits, and handoff packets. |
| GitHub Projects fields | Deferred. Only promote a signal when it is stable, low-cardinality, human-readable, and useful directly inside GitHub. |
| Analytics database | Rejected for this boundary. It would widen Gira beyond the CLI-first control-plane scope. |
| Hosted dashboard | Rejected for this boundary. Future UI should render the same CLI JSON contracts, not define new scoring semantics. |

Signals that change often, contain high-cardinality reason codes, or depend on computed readiness should remain local/export-only unless a later issue proves a GitHub field is durable and valuable.

## Relationship To Finish Readiness

Workflow pulse should connect back to finish/readiness instead of becoming a separate dashboard product.

A useful pulse signal answers one of these questions:

- What can start safely?
- What is blocked and why?
- What needs review?
- What failed checks?
- What is finish-ready?
- What needs a human decision?
- What evidence is missing before finish?

If a signal cannot drive one of those actions, it should not be added to the core surface.

## CI Reliability Boundary

CI reliability signals are useful when they help decide whether a ticket can move forward.

Accepted examples:

- pending, passing, failing, or missing required checks;
- failed-check queue membership;
- check names, conclusions, and URLs needed for repair;
- repeated pending/failure status as a blocker reason;
- finish-readiness blockers derived from checks.

Rejected examples:

- ranking agents by pass rate;
- assigning blame for flakes;
- using check duration as individual performance evidence;
- treating a single green run as proof of product correctness;
- hiding required review because CI passed.

CI is readiness evidence. It is not the whole definition of completion.

## Review Boundary

Review signals are operational when they clarify who or what must act next.

Accepted examples:

- no linked PR;
- draft PR;
- review required;
- changes requested;
- approved;
- merge blocked by pending checks;
- finish receipt missing.

Rejected examples:

- reviewer productivity scoring;
- worker quality scoring from review count alone;
- pressure metrics based on time-in-review without context.

Review state should feed `pr-readiness/v1`, queues, and finish readiness.

## Follow-up Decision

No implementation ticket is ready from this boundary alone.

The next implementation issue should be created only when a concrete CLI JSON gap is found, for example a missing `stats pulse --json` field, an unstable failed-check reason code, or a queue signal that cannot be reproduced from GitHub evidence.

Until then, the accepted path is to reuse existing readiness, queue, checks, review, audit, and finish contracts.
