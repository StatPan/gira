# Agent Delegation Lanes

Gira treats agent delegation as a safety and quality problem, not a generic AI productivity problem.

The key question is not "can AI do it?" The operating question is:

> Can an agent safely complete this issue without human approval?

Use lanes to build an AI-workable queue in GitHub, route work by approval risk, and measure whether delegated work reached closure safely.

## Labels

| Label | Meaning |
| --- | --- |
| `lane:agent` | An agent may execute the issue through completion when checks and repository policy pass. |
| `lane:human` | A human must own execution and approval. Agents may assist only under human direction. |
| `lane:hybrid` | An agent may prepare work, but a human decision, review, credential, or acceptance step is required. |
| `requires-human-approval` | Explicit approval gate. This overrides agent completion authority. |
| `agent:auto-merge-allowed` | Narrow opt-in for agent-prepared PRs to auto-merge only when checks, review rules, and lane policy pass. |

Use at most one primary lane label: `lane:agent`, `lane:human`, or `lane:hybrid`. If labels are missing or conflicting, treat delegation safety as unknown.

## Examples

Agent-suitable work:

- documentation-only updates with clear source material
- deterministic CLI fixes with focused tests
- template, label, milestone, or generated-reference updates
- packaging metadata updates with specified verification
- tests for already-defined behavior

Hybrid work:

- user-facing behavior that needs product approval
- bounded refactors that touch shared interfaces
- release preparation where a maintainer approves publication
- security or permission-policy changes that need risk approval
- multi-repo work where sequencing needs an operator

Human-required work:

- product direction, roadmap, pricing, legal, or policy decisions
- credential rotation, secret handling, production access, or admin changes
- incident response or external commitments
- merges that bypass required checks or branch protection
- work with ambiguous acceptance criteria and unstated business context

## Delegation Quality Metrics

These metrics layer on top of the Closure Funnel Report from issue #393. Closure Funnel answers whether work moved from issue to branch, PR, checks, merge, and closed issue. Delegation Quality answers whether agent-delegated work moved through that funnel within the lane's authority and safety rules.

| Metric | Meaning |
| --- | --- |
| Agent-eligible issue ratio | Share of issues labeled `lane:agent` without `requires-human-approval`. |
| Agent-start success rate | Share of agent-eligible issues where an agent successfully started work. |
| Agent PR check pass rate | Share of agent-prepared PRs whose required checks pass. |
| Agent PR merge rate | Share of agent-prepared PRs that merge. |
| Agent rework rate | Share of agent-prepared PRs that needed material follow-up after first review or checks. |
| Human intervention rate | Share of delegated items that required a human to unblock, approve, redirect, or take over. |
| Escalation reasons | Counts by reason such as ambiguous scope, missing acceptance criteria, failed checks, policy block, or required admin access. |
| Auto-merge safe rate | Share of `agent:auto-merge-allowed` PRs that merged with passing checks and no later safety reversal in the configured window. |

The first Closure Funnel implementation does not need exact agent attribution. Delegation metrics can add attribution, lane, approval, and escalation fields after the base workflow report exists.

## Attribution Confidence

| Level | Meaning |
| --- | --- |
| High | Direct machine-readable evidence identifies an agent, such as an `agent:*` label, Gira lifecycle metadata, bot/app author, mapped PR author, or signed automation comment. |
| Medium | Strong circumstantial evidence exists, such as branch naming plus lane label, PR template marker, or explicit handoff comment. |
| Low | Evidence is weak or mixed, such as a human-authored PR with agent-like branch naming or lane labels added after completion. |
| Unknown | Normal GitHub data has no reliable attribution evidence. |

Evidence sources include issue labels, assignees, PR and commit authors, branch names, linked issue references, Gira lifecycle notes, review states, check states, merge state, and escalation comments. When evidence conflicts, prefer lower confidence and explain why.

For the broader product boundary and optional trace-oriented provenance
envelope, see [Worker Boundary](/worker-boundary). Exact implementation tool
names, model names, trace IDs, prompt IDs, and attempt IDs belong in
telemetry/provenance metadata, not high-cardinality labels.

## Safety Boundary

Delegation Quality reports should describe workflow safety by repository, workspace, milestone, lane, or time window. They should not rank individual humans or agents, infer productivity from time online, token spend, or commit counts, or turn human intervention into a failure score.

GitHub remains the auditable source of truth. The accountable human owner remains separate from the execution actor.
