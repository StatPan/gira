# External Drift And Provenance Policy

This document resolves #788 under the broader #779 API-limit and provider
boundary work. It defines how Gira treats provider changes made outside Gira
commands.

API/provider cluster role: supporting provenance policy. Start with
[GitHub API Limit Operating Model](github-api-limits.md) for the current
entry point, then use this document when classifying Gira-controlled,
reconstructed, external-drift, or unknown evidence.

Gira remains a supported workflow gateway over provider ledgers. GitHub today,
and GitLab in the future, remain the canonical collaboration ledgers for issues,
pull or merge requests, comments, labels, milestones, checks, reviews, and merge
state. Gira receipts explain Gira-controlled decisions. Provider state that
exists without a Gira receipt can still be imported, but it has lower
provenance.

## Policy

Changes made through raw `gh`, the GitHub web UI, GitLab UI, provider APIs,
future provider CLIs, or automation that bypasses Gira lifecycle commands are
external drift unless Gira can reconstruct enough evidence to classify them.

External drift is not automatically bad. Humans and external systems must remain
able to use the provider. The policy is that Gira must not silently treat those
changes as if Gira itself planned, approved, and applied them.

## Provenance Levels

| Level | Meaning | Examples | Gira behavior |
| --- | --- | --- | --- |
| Gira-controlled | A Gira command produced dry-run/apply evidence or an explicit receipt. | `gira ticket start --apply`, `gira ticket finish --apply`, finish receipt, supersede receipt. | May be used as first-class lifecycle evidence. |
| Reconstructed | Provider state is visible and enough evidence exists to infer the workflow state, but no Gira receipt is present. | A PR with `Closes #N`, passing checks, matching branch policy, and a closed issue. | May be imported for status and audit, but reports should preserve that provenance is reconstructed. |
| External drift | Provider state changed outside Gira and evidence is incomplete, conflicting, or policy-sensitive. | Raw label changes, web UI close without receipt, PR base mismatch, manual merge without closing link, comment-only completion. | Surface as drift, require review or normalization before treating it as Gira-complete. |
| Unknown | Gira cannot read enough provider state to classify the change. | Rate-limit failure, missing permissions, deleted branches, inaccessible PRs. | Fail closed for `--apply`; point to diagnostics or provider inspection. |

## Import Rules

Gira may import externally visible provider state for read-only reports:

- issue open or closed state;
- labels and milestones;
- linked PR or merge request state;
- checks, reviews, and merge state;
- comments that contain recognizable Gira receipts or handoff markers.

Importing state does not grant Gira provenance. A read-only command may say a
ticket appears done from provider evidence, while an audit or finish command may
still warn that the finish receipt is missing or reconstructed.

## Receipt Rules

Gira-controlled workflow transitions should leave visible provider evidence:

- starting work records branch policy state on the issue;
- opening PRs uses closing keywords such as `Closes #N`;
- notes and self-reviews use structured issue or PR comments;
- finish posts a concise finish receipt;
- goal finish and supersede paths post their own receipts.

Receipts should be idempotent where possible. Gira should prefer finding an
existing receipt over posting duplicates.

Provider state without a receipt can still be useful, but it is reconstructed
evidence. Gira must not claim that a raw provider mutation was Gira-applied.

## Apply Freshness

Local cache, exported reports, dashboard artifacts, or future runtime
projection stores may accelerate reads. They must not authorize mutation.

Every `--apply` path still needs fresh provider verification for the state it
depends on. If provider freshness cannot be verified because of permissions,
rate limits, ambiguous links, missing checks, or conflicting state, the apply
path should fail closed or require a human decision.

## Agent Guidance

Agents should use Gira CLI or MCP lifecycle commands as the supported workflow
gateway when Gira provides the operation. Raw provider commands are allowed for
diagnostics or gaps in Gira coverage, but they should not bypass start, PR,
check/wait, finish, supersede, or receipt-producing flows.

When an agent must use raw provider commands, it should:

1. explain why no Gira command covered the operation;
2. prefer read-only provider commands;
3. leave a visible note or receipt if it performed a workflow-significant
   mutation;
4. rerun the relevant Gira status or audit command afterward.

## Audit And Follow-Up

Future enforcement slices should keep this policy as the boundary:

- audit reports should distinguish Gira-controlled, reconstructed, drift, and
  unknown provenance;
- status reports may use reconstructed state for next actions, but should not
  hide missing receipts when completion is policy-sensitive;
- provider adapters should expose the same provenance levels for GitHub and
  GitLab. See [Provider Adapter Boundary](provider-adapter-boundary.md);
- local projection storage should preserve provenance metadata rather than
  flattening everything into one trusted state.

Non-goals for this policy slice:

- no runtime enforcement implementation;
- no provider adapter implementation;
- no local SQLite schema;
- no attempt to block humans from using provider-native UI or CLI tools.
