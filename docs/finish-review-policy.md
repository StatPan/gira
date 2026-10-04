# Finish review policy

`finish_review_policy` is a repository contract in `.gira/config.yaml`.
`gira ticket status` and `gira ticket finish` read it from the exact commit at
the linked PR's current base SHA. A checkout-local or PR-candidate config can
neither authorize finish nor weaken the policy already committed at that base.
If the PR base SHA, committed config, or policy cannot be read, finish remains
blocked. This exact-base requirement applies to the existing `required` and
`none` modes as well as the recorded-review mode.

```yaml
finish_review_policy: required # or: none
```

`required` accepts only a native GitHub approving review on the exact current
PR head. `none` remains a deliberate non-blocking choice, but Gira must first
read that choice from the immutable PR base. A missing or unreadable policy
does not inherit a local value. This replaces the earlier local-config fallback
for repositories whose PR base has no readable `.gira/config.yaml` or
`.gira/config.toml`; add a committed policy before relying on finish status.

## Recorded independent review

A repository may opt into `required_or_recorded_independent` when native
approval is not available for its development PRs. The opt-in must itself
already exist at the PR base. It is limited to the explicitly declared
development branch; every other target, including the production branch,
continues to require a native current-head approval.

```yaml
finish_review_policy: required_or_recorded_independent
branch_policy:
  mode: github-flow
  development_base: dev
  production_base: main
recorded_review:
  allowed_recorders:
    - gira-review-bot
  allowed_base_branches:
    - dev
```

`allowed_recorders` lists GitHub logins whose submitted PR review may carry the
receipt. Gira trusts the authenticated GitHub `user.login` on that review and
requires the receipt's `recorder` field to match it. Configure the recorder
through repository policy; a proposed change to the allowlist is not active
for the PR changing it.

Submit the receipt as the body of a GitHub `COMMENTED` PR review. The body must
begin with the marker, followed by one JSON object with no extra properties:

```text
<!-- gira:independent-review/v1 -->
{
  "schema_version": "gira-independent-review/v1",
  "repository": "OWNER/REPO",
  "pull_request": 123,
  "head_sha": "<exact 40-character PR head SHA>",
  "base_ref": "dev",
  "base_sha": "<exact 40-character PR base SHA>",
  "verdict": "GO",
  "reviewer": {"id": "reviewer-run-2026-10-04", "kind": "ai", "run_ref": "https://example.test/reviewer-run"},
  "implementer": {"id": "implementer-run-2026-10-04", "kind": "ai", "run_ref": "https://example.test/implementer-run"},
  "recorder": "gira-review-bot",
  "independent": true,
  "evidence_refs": ["https://github.com/OWNER/REPO/pull/123"],
  "findings": [],
  "limitations": ["State what the review did not verify."]
}
```

The reviewer and implementer IDs and run references must be distinct. `GO`
cannot contain an open blocking finding. `BLOCKED` must identify at least one
open blocking finding. Finding IDs are scoped to the trusted recorder and
reviewer identity. An open finding remains active across later PR heads until
that same trusted recorder and reviewer explicitly records it as `resolved`
with evidence references. A later empty `GO` does not erase it. A malformed
trusted receipt remains blocking until the same recorder explicitly lists its
GitHub review ID in `supersedes_review_ids` on a valid receipt.

Only a receipt bound to the current PR head, current base SHA, and current base
branch can satisfy finish. A stale `GO` is historical evidence only. Gira
rechecks the PR and receipt immediately before merge and pins the merge to the
reviewed head SHA. On the allowlisted development branch, Gira checks trusted
receipt history for unresolved malformed receipts and blocking findings before
considering native approval. Once that history is clear, either a native
approval on the exact current head or a valid current `GO` can satisfy the
policy. Native approval does not erase an unresolved recorded blocker. On every
other base branch, including production, recorded receipts cannot satisfy
review; a native approval on the exact current head remains required. Active
`CHANGES_REQUESTED` reviews block until the reviewer changes or dismisses
their review.

This receipt is an operator-recorded attestation, not an authenticated AI
identity or proof that a particular model ran. `independent: true`, reviewer
and implementer IDs, run references, evidence links, and limitations make the
claim inspectable; they do not cryptographically prove the claimed separation.
The generic `gira ticket self-review` check note is not a receipt and never
satisfies this policy. Goal-status snapshots do not include review bodies, so
they report recorded-review evidence as unavailable instead of treating it as
passing; use `gira ticket status` or `gira ticket finish --dry-run` for the
full receipt check.
