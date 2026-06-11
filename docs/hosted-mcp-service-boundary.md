# Hosted MCP Service Boundary

## Status

Research boundary for #740. Predecessor: [ADR 0003](adr/0003-cli-and-mcp-parity-boundary.md).

## Decision

Hosted MCP is deferred as an implementation.

A future hosted or managed Gira MCP service may become a product direction, but v1 must preserve the local CLI/MCP safety model:

```text
Hosted MCP v1 is read-only by default.
No hosted apply/mutation tools.
No broad GitHub credential storage without a later explicit policy.
No hosted hidden workflow state that competes with GitHub evidence.
```

Hosted MCP must remain an access path to the same Gira lifecycle, not a second workflow brain.

## Modes

| Mode | Runs where | Allowed behavior | Source of truth |
| --- | --- | --- | --- |
| Local CLI | User machine or repo checkout | Read, dry-run, apply | GitHub, local repo, Gira receipts, local config. |
| Local MCP | User machine, wrapping CLI JSON | Read, plan, handoff | GitHub evidence through CLI JSON contracts. |
| Hosted MCP | Managed service | Read-only by default; plan evidence only after later policy | GitHub evidence and explicit Gira receipts. Hosted logs are not workflow truth. |

The local CLI remains the broadest capability surface because it runs inside the user's own checkout and approval context.

## Hosted v1 Default Posture

Hosted MCP v1 should expose only read-oriented workflow-control operations unless a later ADR supersedes this boundary.

Allowed by default:

- read ticket state;
- read queue state;
- read checks/review blockers;
- read handoff packets;
- read finish plans if backed by the same dry-run contract and no mutation occurs;
- return structured recommendations, blockers, and schema names.

Rejected by default:

- `--apply` operations;
- branch creation or checkout mutation;
- issue, PR, label, milestone, Project, or receipt mutation;
- raw `gh` execution;
- arbitrary shell execution;
- background sync that mutates customer repositories;
- hosted workflow state that overrides GitHub or Gira receipts.

## Auth And Token Ownership

Hosted MCP should avoid storing broad GitHub credentials.

Preferred options, from safest to riskiest:

| Option | Boundary |
| --- | --- |
| Customer-controlled local MCP | No hosted token storage. The customer runs `gira mcp serve` locally. |
| GitHub App with least-privilege installation | Repo-scoped permissions, revocable by installation, auditable through GitHub. |
| Short-lived delegated token | Narrow duration and scope; avoid long-term storage. |
| Customer-managed secret pointer | Hosted service never sees the raw token unless explicitly invoked by customer infrastructure. |
| Broad stored PAT | Rejected by default; requires separate security policy and explicit consent. |

Hosted MCP should document exactly what it reads: repo metadata, issue bodies, PR bodies, checks, reviews, comments, labels, milestones, and optional file diffs when a future tool needs them.

## Tenant Isolation

Hosted MCP must never mix tenant state.

Minimum requirements before implementation:

- tenant-scoped GitHub installation or credential boundary;
- tenant-scoped logs and metrics;
- no shared queue cache without tenant keying;
- no cross-tenant prompt, handoff, issue body, PR body, or check data reuse;
- explicit deletion and retention policy;
- safe failure mode when tenant identity is ambiguous.

If tenant identity cannot be proven for a request, hosted MCP should refuse the request.

## State Ownership

Hosted service state may support diagnostics. It must not become workflow truth.

| State | Allowed hosted use | Canonical owner |
| --- | --- | --- |
| GitHub issue/PR/check/review state | Read and summarize. | GitHub. |
| Gira receipts and handoff comments | Read and render. | GitHub comments. |
| CLI JSON contracts | Preserve as payload semantics. | Gira CLI contract. |
| Hosted logs | Diagnostics, audit, abuse investigation. | Hosted service only; not workflow truth. |
| Hosted cache | Performance only, bounded TTL. | Rebuildable from GitHub and Gira config. |
| Hosted queue projection | Read-only projection. | Derived from GitHub evidence. |

Hosted logs may prove what the service did. They must not prove that work is done unless GitHub evidence and Gira receipts support that conclusion.

## Mutation Boundary

Hosted mutation is rejected for v1.

If hosted mutation is ever considered, a later ADR must define:

- explicit user consent;
- exact command allow-list;
- dry-run evidence and approval equivalence to CLI `--apply`;
- audit receipt format;
- GitHub-visible mutation receipt where appropriate;
- rollback or remediation expectation;
- token scope and revocation model;
- tenant isolation evidence;
- post-apply readback verification;
- failure handling when hosted and GitHub states disagree.

Without that ADR, hosted MCP must refuse mutation.

## Audit And Retention

Hosted MCP should be attributable enough for enterprise review without retaining unnecessary customer content.

A hosted read/plan audit record should capture:

- tenant or installation identifier;
- repo identifier;
- tool name;
- Gira schema version returned;
- start and finish timestamps;
- success or failure;
- request correlation ID;
- output digest or compact metadata when possible.

Avoid retaining full issue bodies, PR bodies, diffs, or comments unless the customer explicitly enables that retention.

## Consent

Users should understand what hosted MCP can read before connecting it.

Consent copy should distinguish:

- repository metadata;
- issue titles and bodies;
- issue comments and Gira receipts;
- PR titles, bodies, reviews, and checks;
- labels, milestones, and Projects visibility;
- file diffs, only if a future tool requires them.

Consent should also state that hosted MCP v1 does not mutate GitHub.

## Outside OSS/MVP

These remain outside the current OSS CLI/MVP boundary:

- hosted billing;
- hosted tenancy implementation;
- hosted mutation/apply;
- broad background sync;
- hosted dashboard state as source of truth;
- hosted model routing or worker execution;
- cross-repository analytics unrelated to finish/readiness.

## Implementation Prerequisites

Before any hosted MCP implementation ticket can start, Gira needs:

- accepted hosted MCP boundary document;
- accepted auth/token policy;
- tenant isolation design;
- logging and retention policy;
- explicit list of read-only hosted tools;
- confirmation that hosted tool output preserves CLI/MCP parity from ADR 0003;
- decision on whether file diffs are in or out for hosted v1.

## Conclusion

Hosted MCP is a possible managed control surface, but implementation is deferred.

The product should first keep the local CLI/MCP model coherent and safe: one Gira lifecycle, GitHub evidence as the workflow boundary, read-only MCP by default, and mutations bound to explicit CLI approval.
