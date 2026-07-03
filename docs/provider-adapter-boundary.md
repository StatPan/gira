# Provider Adapter Boundary

This document resolves #787 under the broader #779 API-limit and provider
boundary work. It defines the first provider adapter boundary for a
GitHub-first Gira with GitLab as the next target provider.

API/provider cluster role: supporting provider boundary. Start with
[GitHub API Limit Operating Model](github-api-limits.md) for the current
entry point, then use this document when evaluating GitLab/provider portability
without moving workflow truth out of provider ledgers.

Gira is not moving workflow truth into a local control plane. GitHub today, and
GitLab in the next provider slice, remain canonical collaboration ledgers for
issues, pull or merge requests, comments, labels, milestones, checks, reviews,
and merge state. Gira is the supported workflow gateway over those ledgers.

## Decision

Keep GitHub as the default and current implementation provider. Introduce a
provider-neutral boundary at the model and command-planning layer, then keep
provider-specific transport and capability details behind provider adapters.

GitLab is the next provider target. Forgejo is not part of this slice.

The boundary is intentionally small:

- normalize the objects Gira needs to plan and verify lifecycle commands;
- keep provider-specific fields available under provider namespaces;
- preserve GitHub and GitLab as collaboration ledgers;
- require fresh provider verification for mutating `--apply` paths;
- treat GitHub Projects v2 as a GitHub-specific visibility capability, not a
  portable core requirement.

## Provider-Neutral Core

The core model should describe the workflow facts Gira needs without assuming
GitHub naming.

| Core concept | GitHub source | GitLab source | Notes |
| --- | --- | --- | --- |
| Work item | Issue | Issue | The executable ticket or larger goal envelope. |
| Change request | Pull request | Merge request | The reviewable change unit linked to a work item. |
| Check result | Check runs, commit statuses, workflow conclusions | Pipelines, jobs, commit statuses | Normalize status and conclusion; keep raw provider details namespaced. |
| Review signal | PR reviews, review decision, requested reviewers | MR approvals, approval rules, discussions | Normalize blocking or approving state, not the full review product. |
| Label | Issue and PR labels | Issue and MR labels | Keep Gira status/type labels portable where possible. |
| Milestone | Milestone | Milestone | Treat as phase or release grouping, not execution truth by itself. |
| Comment | Issue comments, PR comments, timeline comments | Notes and discussions | Used for receipts, handoffs, progress notes, and reconstructed evidence. |
| Branch reference | Base/head refs and SHAs | Source/target branches and SHAs | Required for trusted branch and merge verification. |
| Closing link | Closing keywords and linked PR evidence | Closing patterns and MR issue links | Provider-specific parsing feeds one normalized link state. |

The normalized model should be enough to compute ticket readiness, PR/MR
readiness, finish readiness, queue status, and goal progress. It should not
attempt to hide every provider difference.

## Provider-Specific Capabilities

Provider adapters should expose capabilities explicitly instead of pretending
all providers support the same product surface.

| Capability | Boundary |
| --- | --- |
| GitHub Projects v2 | GitHub-specific visibility and planning surface. It is not a portable Gira core dependency. |
| GitHub Actions details | Provider-specific check detail. The core consumes normalized check result and links back to raw detail. |
| GitLab pipelines and jobs | Provider-specific check detail. The core consumes normalized check result and links back to raw detail. |
| GitHub branch protection | Provider-specific merge policy evidence. Normalize the blocking facts Gira needs. |
| GitLab approval rules | Provider-specific review policy evidence. Normalize the blocking facts Gira needs. |
| Jira provider and mirrors | Separate provider family and operating mode. It must not be folded into the forge adapter core. |

The first boundary should expose a capability report so commands and agents can
explain what is available for the selected provider. Missing capabilities
should produce explicit blockers or reduced read-only views, not silent fallbacks
to a different provider model.

## Current GitHub Assumptions To Untangle

The current CLI is allowed to stay GitHub-first while implementation slices
move assumptions behind a boundary. The main assumptions are:

- `gh` is the transport and authentication source;
- `gh issue`, `gh pr`, and `gh api` shape command behavior;
- issue and PR URLs use GitHub URL patterns;
- PR links depend on GitHub closing keywords and issue timeline semantics;
- checks combine GitHub check-runs, commit statuses, and Actions conclusions;
- reviews depend on GitHub PR review states and branch protection behavior;
- label and milestone sync assumes GitHub REST resources;
- GitHub Projects v2 is a provider-specific visibility surface;
- finish receipts are issue or PR comments in GitHub terminology.

These assumptions should be isolated at transport, URL parsing, capability, and
provider raw-data layers. They should not leak into the provider-neutral command
contracts when a portable concept exists.

## Apply And Provenance Rules

Local cache and future projection stores can reduce repeated provider reads, but
they do not authorize mutation. Every `--apply` path must verify the provider
facts it depends on at the point of mutation.

Provider adapters must preserve provenance:

- Gira-controlled evidence comes from Gira dry-run/apply output or explicit
  receipts.
- Reconstructed evidence comes from visible provider state without a matching
  Gira receipt.
- External drift is provider state changed outside Gira with incomplete,
  conflicting, or policy-sensitive evidence.
- Unknown evidence fails closed for mutating paths.

See [External Drift And Provenance Policy](external-drift-policy.md) for the
shared policy.

## Follow-Up Implementation Slices

Use small slices so GitLab support does not turn into a rewrite.

1. Define provider-neutral domain types for work items, change requests,
   checks, reviews, labels, milestones, comments, branches, and closing links.
2. Put the current GitHub transport behind a GitHub adapter while preserving the
   existing GitHub CLI behavior.
3. Normalize REST-first check and review summaries before adding richer provider
   detail.
4. Add a GitLab read-only status prototype for issue and merge request
   discovery.
5. Add GitLab dry-run lifecycle mapping for start, link, check, review, and
   finish readiness without mutating provider state.
6. Add provider diagnostics that report auth, reachable resources, supported
   capabilities, rate-limit buckets, and known unsupported features.
7. Keep GitHub Projects visibility behind an explicit GitHub capability instead
   of a core queue dependency.

## Non-Goals

- no GitLab implementation in this slice;
- no Forgejo planning in this slice;
- no hosted control plane;
- no SQLite schema;
- no replacement for GitHub or GitLab as provider ledgers;
- no requirement that GitHub Projects v2 exists for portable Gira operation.
