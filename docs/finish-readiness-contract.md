# Finish Readiness Contract

`finish-readiness/v1` and `finish-receipt/v1` are the public control-plane
contracts behind `gira ticket finish`.

They exist so humans, agents, adapters, and future MCP tools can decide whether
a GitHub issue-backed ticket can safely finish without trusting an agent
transcript.

## Command Surfaces

Preview finish readiness:

```bash
gira ticket finish TICKET --repo OWNER/REPO --dry-run --json
```

Apply an approved finish:

```bash
gira ticket finish TICKET --repo OWNER/REPO --apply --json
```

`ticket finish` always expresses terminal merge intent; it is not a ready-only
command. When the linked PR is still Draft, however, one invocation is bounded
to the safe transition it previewed:

1. `--dry-run` previews `pr:ready` and reports all other observed blockers.
2. The matching `--apply` marks the PR ready and stops without merging.
3. The operator must run a new `--dry-run` against the ready PR before a later
   apply may merge or delete the remote branch.

A merge-capable preview emits an `IRREVERSIBLE` warning in human output and in
the JSON/approval warnings, and its approval action set explicitly includes the
merge and remote-branch deletion detail.

The dry-run output includes `work-finish-result/v1`, the embedded
`finish-readiness/v1` report, and `gira-approval-plan/v1` approval evidence.
The apply output includes `work-finish-result/v1` and an embedded
`finish-receipt/v1` when finish evidence is accepted.

## Source Of Truth

Finish readiness is recomputed from GitHub evidence and Gira configuration.

| Evidence | Source |
| --- | --- |
| Ticket state, title, labels, milestone | GitHub issue |
| Branch binding and branch trust | Gira lifecycle block plus Git/PR evidence |
| Pull request state, draft state, base/head refs, mergeability | GitHub PR |
| Closing reference | Linked PR body or GitHub closing reference evidence |
| Checks | GitHub check runs/status rollups |
| Review state | GitHub review decision and blockers |
| Acceptance criteria and telemetry | Ticket body/status report |
| Finish receipt | GitHub issue comment after accepted apply |

Local cache can accelerate reads, but it is not the completion source of truth.

## `finish-readiness/v1`

`finish-readiness/v1` answers one question:

```text
Can this ticket safely finish now?
```

Required top-level fields:

| Field | Meaning |
| --- | --- |
| `schema_version` | Always `finish-readiness/v1`. |
| `repository` | Target repository in `OWNER/REPO` form. |
| `issue` | Ticket issue summary. |
| `pull_request` | Linked PR summary and merge/review state. |
| `checks` | Aggregate check status and counts. |
| `review` | Review status and decision. |
| `evidence` | Boolean evidence summary and evidence sources. |
| `label_state` | Current status label state. |
| `acceptance_criteria` | Parsed acceptance state when available. |
| `closing_reference` | Whether a PR closing reference is present. |
| `ready` | True only when finish blockers are clear and completion evidence is sufficient. |
| `blockers` | Stable reason codes preventing finish. |
| `next_action` | Machine-readable next action. |
| `next_step` | Human-oriented next command or remediation. |
| `warnings` | Non-blocking concerns. |

Important nested fields:

| Path | Meaning |
| --- | --- |
| `issue.number` | GitHub issue number. |
| `issue.state` | GitHub issue state. |
| `issue.status` | Gira active status label summary. |
| `pull_request.available` | Whether a linked PR was found. |
| `pull_request.number` | Linked PR number. |
| `pull_request.state` | PR state from GitHub. |
| `pull_request.mergeable` | GitHub mergeability value when available. |
| `pull_request.review_decision` | GitHub review decision when available. |
| `pull_request.is_draft` | Whether the PR is draft. |
| `pull_request.head_ref_name` | PR head branch. |
| `pull_request.base_ref_name` | PR base branch. |
| `pull_request.head_sha` | Exact head commit when GitHub exposes it. |
| `pull_request.merge_commit_sha` | Exact merge commit for a verified merged PR. |
| `pull_request.closing_reference` | Whether the selected PR closes the ticket. |
| `checks.status` | `passed`, `pending`, `failed`, `missing`, or equivalent normalized status. |
| `checks.total` | Total check count. |
| `checks.passing` | Passing check count. |
| `checks.pending` | Pending check count. |
| `checks.failing` | Failing check count. |
| `checks.missing` | True when no check evidence is available. |
| `review.status` | Normalized review status. |
| `review.decision` | Raw or normalized review decision. |
| `evidence.closing_reference` | Whether closure evidence exists. |
| `evidence.branch_trusted` | Whether branch binding is trusted. |
| `evidence.finish_ready` | Whether status computation believes finish is ready. |
| `evidence.sources` | Evidence sources used for readiness. |
| `label_state.active_status_labels` | Active `status:*` labels currently present. |

## Blocker Codes

The blocker list is intentionally compact and stable.

| Blocker | Meaning |
| --- | --- |
| `missing_linked_pr` | No linked PR was found. |
| `checks` | One or more checks failed. |
| `checks_pending` | One or more checks are still running or queued. |
| `draft` | The linked PR is still a draft. |
| `review` | Review state blocks finish. |
| `pr_binding` | Multiple plausible closing PRs are ambiguous, or the only candidate is closed without merge evidence. |
| `final_status_unavailable` | Gira could not compute the final ticket status. |

Additional blockers may be added when they are evidence-backed and actionable.
Do not add labels for these blockers; they are computed state, not GitHub
taxonomy.

## Readiness Rule

`ready=true` requires:

- a linked PR is available;
- closing-reference evidence is present;
- checks are not failing or pending;
- draft status does not block finish;
- review status does not block finish;
- branch and PR evidence satisfy policy;
- no explicit finish blockers remain.

Already-finished tickets may be reported ready when GitHub evidence already
proves convergence.

When several PRs contain a closing reference, Gira resolves them conservatively:

1. one merged PR wins over closed-unmerged or open candidates;
2. otherwise, one open PR with a trusted branch binding wins;
3. multiple merged or multiple trusted-open candidates fail closed with
   `pr_binding`;
4. a closed-unmerged PR never counts as merged delivery evidence.

Finish preserves the selected PR number across merge and final status
resolution. Before writing a successful receipt, Gira fetches that exact PR and
requires its merged state, closing relationship, head SHA, and merge commit SHA.

## `finish-receipt/v1`

`finish-receipt/v1` is durable audit evidence written after an accepted finish.
It is rendered as a GitHub issue comment and embedded in the apply JSON result.

Required fields:

| Field | Meaning |
| --- | --- |
| `schema_version` | Always `finish-receipt/v1`. |
| `finished_at` | UTC completion timestamp. |
| `repository` | Target repository. |
| `issue` | Ticket issue summary at finish time. |
| `pull_request` | Linked PR summary and merged state. |
| `pull_request.number` | Exact PR selected and verified by finish. |
| `pull_request.head_sha` | Verified delivered head commit. |
| `pull_request.merge_commit_sha` | Verified GitHub merge commit. |
| `pull_request.closing_reference` | Verified closing relationship to the ticket. |
| `checks_summary` | Check aggregate at finish time. |
| `review_summary` | Review aggregate at finish time. |
| `evidence_summary` | Evidence used to accept finish. |
| `telemetry_summary` | AI delivery telemetry summary when available. |
| `label_changes` | Status normalization changes applied by finish. |
| `final_state` | Final issue status and next action. |
| `warnings` | Non-blocking warnings carried into the receipt. |
| `target` | Receipt target, currently the issue. |
| `rendered_body` | Markdown body posted to GitHub. |

Receipt comments should stay concise. They explain why finish was accepted;
they are not a full agent transcript.

## Approval Boundary

`ticket finish --dry-run --json` emits `gira-approval-plan/v1`. An adapter or
operator may approve only the matching apply command described by that dry-run
evidence.

The approval does not authorize:

- a different repo;
- a different ticket;
- additional flags not present in the approved argv;
- raw `gh` merge or issue-close commands;
- bypassing failed checks, pending checks, draft state, or review blockers.
- actions that appear only after an intermediate mutation; Draft-to-ready and
  ready-to-merge require separate preview/apply cycles.

## Current Gap Assessment

No immediate schema-breaking gap is required for the next implementation slice.

Candidate future improvements should be filed as separate tickets if accepted:

- expose unresolved review-thread counts when GitHub evidence is available;
- split `review` blockers into more precise reason codes;
- include branch-policy mismatch details directly in `finish-readiness/v1`;
- make receipt idempotency explicit for repeated finish attempts.

These are follow-ups, not prerequisites for treating the current contracts as
public workflow-control surfaces.

## Related Fixture Suite

The accepted local fixture suite lives under
`internal/gira/testdata/agent_workflow_benchmark/` and protects finish,
readiness, review, queue, and handoff workflow decisions without becoming a
model leaderboard or code-generation benchmark.
