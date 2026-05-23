# Agent Delegation Lanes

This document defines the Gira policy model for Delegation Safety and Delegation Quality.

The question is not "can AI do it?" Many coding agents can implement a large share of repository work. The useful operating question is:

> Can an agent safely complete this issue without human approval?

Gira should help operators create an AI-workable queue inside GitHub, route issues by approval risk, and measure whether delegated work reached completion safely. This is not a generic AI productivity dashboard and must not become individual productivity surveillance.

## Relationship To Closure Funnel

Issue #393 defines the Closure Funnel Report: whether work flows from issue to branch, PR, checks, merge, and closed issue. That report is the base workflow integrity layer.

Agent delegation metrics layer on top of the closure funnel instead of replacing it:

- Closure Funnel answers whether GitHub work reached closure.
- Delegation Quality answers whether work that was delegated to agents reached closure within the lane's authority and safety rules.
- Closure Funnel can ship first without exact agent attribution.
- Delegation Quality can add lane, approval, escalation, and attribution confidence fields as conventions become available.

This ordering keeps the first stats slice useful on non-Gira repositories while allowing higher-confidence delegation reporting on repositories that use Gira labels and lifecycle commands.

## Delegation Labels

Delegation lanes are GitHub labels. They describe authority and approval policy, not model capability.

| Label | Meaning | Completion authority |
| --- | --- | --- |
| `lane:agent` | Work is suitable for an agent to execute through PR readiness or merge, subject to repository policy. | Agent may complete without additional human approval only when no `requires-human-approval` label is present and branch protection/check policy allows it. |
| `lane:human` | Work requires human ownership because the risk, judgment, access, or accountability cannot be delegated safely. | Human must own execution and approval. Agents may assist only as tools under human direction. |
| `lane:hybrid` | Work can be partially delegated, but at least one decision, review, credential, or acceptance step needs a human. | Agent may prepare changes; human approval is required before final completion. |
| `requires-human-approval` | Explicit approval gate. This overrides `lane:agent` for merge/close authority. | Human approval is required before merge, release, or closure. |
| `agent:auto-merge-allowed` | Narrow opt-in for agent-prepared PRs to merge automatically when checks, review rules, and lane policy pass. | Auto-merge may be enabled only for `lane:agent` work with passing required checks and no human-approval gate. |

Rules:

- At most one primary lane label should be present: `lane:agent`, `lane:human`, or `lane:hybrid`.
- `requires-human-approval` is a modifier and may appear with any lane.
- `agent:auto-merge-allowed` is a modifier, not a lane. It is meaningful only when the work is otherwise agent-completable.
- If lane labels conflict or are missing, reports should treat the issue as unknown or not agent-eligible rather than assuming delegation is safe.

## Lane Examples

Agent-suitable work usually has a bounded change, explicit acceptance criteria, local or CI verification, and low product or operational ambiguity.

Examples:

- Documentation-only updates with clear source material and reviewable diffs.
- Deterministic CLI behavior fixes with focused tests.
- Template, label, milestone, or generated-reference updates where dry-run output can be compared.
- Dependency metadata or packaging adjustments when the version and verification command are specified.
- Test coverage for an already-defined behavior.

Hybrid work is safe to start with an agent but unsafe to finish without human judgment.

Examples:

- User-facing behavior where product copy, compatibility, or UX tradeoffs need approval.
- Refactors that are mechanically bounded but touch shared interfaces.
- Release preparation where an agent can assemble evidence, but a maintainer must approve publication.
- Security or permission-policy changes where the implementation is straightforward but the risk decision belongs to a human.
- Multi-repository coordination where an agent can prepare one repo slice but sequencing needs an operator.

Human-required work needs accountable human ownership.

Examples:

- Product direction, prioritization, roadmap, pricing, legal, or policy decisions.
- Credential rotation, secret handling, production access, or admin permission changes.
- Incident response, customer-specific commitments, or external communications.
- Merges that intentionally bypass required checks or branch protection.
- Work with ambiguous acceptance criteria where the correct outcome depends on unstated business context.

## Delegation Quality Metrics

Metrics should be reported at repository, workspace, milestone, or time-window level. They should not rank individual people.

| Metric | Definition | Suggested numerator | Suggested denominator |
| --- | --- | --- | --- |
| Agent-eligible issue ratio | Share of issues that are safe delegation candidates. | Opened or active issues labeled `lane:agent` and not labeled `requires-human-approval`. | Opened or active issues in scope with known lane data. |
| Agent-start success rate | Share of agent-eligible issues where an agent could start work successfully. | Agent-eligible issues with start evidence such as branch creation, `status:in-progress`, or Gira start metadata. | Agent-eligible issues attempted by an agent. |
| Agent PR check pass rate | Share of agent-prepared PRs whose required checks pass. | Agent-attributed PRs with passing required checks. | Agent-attributed PRs with completed required checks. |
| Agent PR merge rate | Share of agent-prepared PRs that merge. | Agent-attributed PRs merged in scope. | Agent-attributed PRs opened in scope. |
| Agent rework rate | Share of agent-prepared PRs that required material follow-up after first review/check run. | Agent-attributed PRs with requested changes, failed checks followed by new commits, reopen cycles, or explicit rework labels/comments. | Agent-attributed PRs opened in scope. |
| Human intervention rate | Share of delegated issues or PRs that required a human to unblock, approve, redirect, or take over. | Agent-delegated items with intervention evidence. | Agent-delegated items attempted in scope. |
| Escalation reasons | Categorized reasons agent work could not continue safely. | Count by reason code. | Not a ratio by default; report counts and top reasons. |
| Auto-merge safe rate | Share of auto-merge-allowed agent PRs that merged without later safety reversal. | `agent:auto-merge-allowed` PRs that merged with passing required checks and no revert, hotfix, or human override marker within the configured window. | `agent:auto-merge-allowed` PRs merged in scope. |

Suggested escalation reason codes:

- `missing_acceptance_criteria`
- `ambiguous_scope`
- `requires_product_decision`
- `requires_secret_or_admin_access`
- `requires_human_review`
- `checks_failed`
- `merge_conflict`
- `external_dependency`
- `policy_blocked`
- `unsafe_auto_merge`
- `unknown`

Reports should preserve the raw evidence behind each metric where possible: issue numbers, PR numbers, labels, check conclusions, review states, comments, branch names, and timestamps.

## Attribution Confidence

Exact agent attribution is not required for the first Closure Funnel slice. Repositories that use Gira conventions can improve attribution confidence over time.

| Level | Meaning | Evidence examples |
| --- | --- | --- |
| High | The item has direct, machine-readable evidence that an agent executed or prepared it. | `agent:*` label, Gira lifecycle metadata, bot/app author, branch created by an agent credential, PR author mapped to a known agent identity, signed automation comment. |
| Medium | The item has strong circumstantial evidence but no single authoritative signal. | Branch naming convention plus issue lane, PR template marker, consistent commit author, or issue comment indicating agent handoff. |
| Low | The item might be agent-related but evidence is weak or mixed. | Human-authored PR with agent-like branch name, copied agent output in comments, or lane labels added after completion. |
| Unknown | No reliable attribution evidence exists. | Normal GitHub issue/PR data without Gira or agent conventions. |

Evidence sources should be additive and explainable:

- GitHub issue labels, including lane and `agent:*` labels.
- GitHub assignees for accountable humans.
- PR author, commit author, branch naming, and linked issue references.
- Gira lifecycle commands and future machine-readable notes.
- Review states, check states, merge method, and auto-merge state.
- Explicit comments or labels that record escalation and human takeover.

When evidence conflicts, reports should prefer lower confidence and show why. Unknown attribution must not be treated as human work or agent work by default.

## Provenance Notes

`agent:*` labels are execution actor evidence. They are not final ownership,
review, or accountability evidence. `lane:*` labels are delegation policy. They
say what kind of work may be delegated, not who actually planned, implemented,
or reviewed a specific result.

When labels are too coarse, record a GitHub-visible provenance block in an
issue or PR body/comment:

```markdown
<!-- gira:provenance:start -->
planning: human
implementation: ai
review: human
<!-- gira:provenance:end -->
```

Allowed actor classes are `human` and `ai`. Tool-specific names such as
`codex`, `agent`, `bot`, or `llm` normalize to `ai`; names such as
`maintainer`, `operator`, or `person` normalize to `human`.

Use the phases this way:

| Phase | Meaning |
| --- | --- |
| `planning` | Who shaped the issue, task breakdown, acceptance criteria, or direction. |
| `implementation` | Who produced the material change or execution artifact. |
| `review` | Who reviewed, accepted, requested changes, or supplied final judgment. |

Mixed values are allowed:

```markdown
implementation: human, ai
review: human, ai
```

Status and future stats reports can summarize these blocks without private
identity inference. If a provenance block is absent, Gira may use `agent:*`
labels as a low-friction implementation actor hint, but it must not treat
`lane:*` labels as proof of execution.

For the broader product boundary and the optional trace-oriented provenance
envelope, see [Worker Boundary And Provenance](worker-boundary-provenance.md).
Exact implementation tool names, model names, trace IDs, prompt IDs, and
attempt IDs belong in telemetry/provenance metadata, not high-cardinality
labels.

## Safety And Privacy Boundary

Delegation Quality is an operating safety report. It is not a scorecard for individual productivity.

Required language for future reporting surfaces:

- Report workflow and delegation outcomes by repository, workspace, milestone, lane, or time window.
- Avoid ranking individual humans or agents by output volume.
- Do not infer performance from private effort, time online, token spend, or commit counts.
- Keep GitHub evidence visible and auditable instead of creating hidden behavioral profiles.
- Treat human intervention as a safety signal, not a failure by default.
- Preserve the accountable human owner separately from the execution actor.

The intended business wedge is controlled delegation: make it clear which issues are safe for agents, where agents need approval, where humans must own the work, and whether delegated work safely reached GitHub closure.

## Implementation Notes

Future implementation can expose these fields in a `gira stats` report after the Closure Funnel Report exists.

Minimum useful additions to closure funnel rows:

- lane label
- approval modifier labels
- agent attribution confidence
- attribution evidence summary
- escalation reason when present
- auto-merge eligibility and outcome

Default output should remain text for operators, with `--json` reserved for automation. The report should remain read-only against GitHub and should work on non-Gira repositories with lower confidence.
