<!-- gira:pm-policy canonical=v1 -->
# Gira PM Operating Policy

This document is the canonical, model-independent operating policy for product
management in Gira. It defines the PM role shared by human operators and AI
hosts. CLI commands, MCP tools, task-packet templates, Goal Mode, and adapter
instructions may expose or summarize this policy, but must not redefine it.

`docs/pm-skill.md` is the companion task-packet and acceptance-QA guide.
`docs/skills/gira-agent-operator.md` is the companion execution lifecycle.

## Product Boundary

Gira owns a PM protocol, not a claim that any connected model is a product
manager. The protocol makes product work resumable, reviewable, and testable:

1. hydrate durable context;
2. compile intent and expose gaps;
3. distinguish evidence, inference, assumption, and decision;
4. discover opportunities and risks before committing to a solution;
5. shape a bounded, verifiable work graph;
6. act only inside declared authority and reversibility boundaries;
7. observe delivery and outcome evidence;
8. replan or escalate one residual decision;
9. derive reports and handoffs from the same canonical state.

GitHub issues, comments, links, PRs, checks, labels, milestones, and Gira
receipts remain the durable source of truth. A model transcript and local cache
may help reconstruct state, but neither is authoritative.

## Normative Invariants

Gira PM implementations and hosts MUST preserve these rules:

- **Outcome before output.** Work MUST link to a user, product, business, or
  operational outcome. Delivery completion MUST NOT be presented as validated
  product impact without outcome evidence.
- **Evidence before inference.** Supplied evidence, derived inference,
  assumption, decision, and unresolved question MUST remain distinguishable.
- **Decomposition before escalation.** A broad `needs human`, `ask_human`, or
  `human_review` signal MUST first be classified and reduced into safe work.
- **Reversible progress before irreversible commitment.** Independent safe
  work SHOULD continue while irreversible or authority-bound work is isolated.
- **Explicit authority.** Credentials, secrets, permissions, billing,
  production state, legal or policy commitments, and destructive external
  actions MUST NOT be inferred from a general product goal.
- **Discovery and delivery stay connected.** Discovery findings, decisions,
  delivery work, verification, and learning MUST retain links to the same goal.
- **One source, multiple views.** Status, stakeholder reporting, PM handoff,
  AI hydration, and audit SHOULD be derived views rather than duplicate state.
- **Compact by default.** Repeated source text SHOULD be replaced with a bounded
  summary and stable references. Detail remains available explicitly.

Hosts MAY use probabilistic reasoning to interpret product meaning. Gira core
MUST keep policy validation, schema validation, readiness diagnostics,
authority gates, mutation plans, and receipts deterministic.

## Human And AI PM Roles

Human and AI PMs use the same state and leave the same evidence. Their primary
affordances differ, but neither receives a separate lifecycle.

| Actor | Primary affordances | Required receipts |
| --- | --- | --- |
| Human PM | inspect, amend, approve, override with rationale, communicate | changed premise or policy, decision rationale, approval boundary, handoff |
| AI PM host | hydrate, compile, retrieve, propose, monitor, replan, validate, escalate | sources used, assumptions made, plan fingerprint, verification, residual decision |
| Gira core | validate, classify, plan mutations, enforce gates, persist evidence, derive views | schema versions, diagnostics, approval plan, apply result, post-apply verification |

An adapter such as MCP is a transport. It MUST call the same Gira domain
contracts as CLI operation and MUST NOT become a second PM workflow brain.

## Epistemic Contract

PM state SHOULD classify material claims as:

| Kind | Meaning | Minimum metadata |
| --- | --- | --- |
| Evidence | An inspectable observation or source | source reference, freshness, scope |
| Inference | A conclusion derived from evidence | evidence references, reasoning boundary |
| Assumption | A claim accepted provisionally | validation path, risk, expiry or review condition |
| Decision | A selected course or policy | options, rationale, authority, scope, review condition |
| Question | An unresolved fact or judgment | impact, resolution path, resume condition |

Missing metadata is a diagnostic, not permission to invent it. Legacy prose may
be retained as untyped evidence until a bounded migration explicitly classifies
it.

## Causal Resolution Before Human Escalation

When work appears to need a person, the PM MUST classify the cause before
stopping all work:

| Cause | First resolution | Escalate only when |
| --- | --- | --- |
| Missing context | retrieve repository, issue, customer, or operational evidence | required evidence remains unavailable after bounded retrieval |
| Missing decision policy | derive from explicit product principles and prior decisions | no authorized principle distinguishes viable options |
| Conflicting constraints | isolate the conflict and compare scope, authority, and reversibility | equally ranked principles remain incompatible |
| Irreversible risk | split preparation, simulation, rollback, or reversible rollout work | the remaining action itself is irreversible |
| Insufficient verification | create a test, measurement, or evidence task | verification cannot be obtained within authorized scope |
| Authority boundary | continue independent safe work | the residual action requires authority not granted by the goal |
| Undefined success metric | define a signal, baseline, target, or qualitative evidence plan | product direction is required to choose what success means |

A residual decision packet MUST contain one exact question, viable options,
known impacts, evidence already gathered, authority required, recommended safe
default when one exists, work that can continue, and the resume condition.
Legacy public values such as `ask_human` and `human_review` remain compatibility
states until their owning schemas migrate; policy consumers MUST interpret them
as unresolved routing inputs, not proof that decomposition has completed.

## Authority, Reversibility, And Overrides

A PM MAY propose work inside the Goal scope. Apply authority is narrower than
planning authority. Before mutation, the caller MUST have a visible dry-run or
equivalent plan, required capability, and an unchanged target state where the
command contract provides a fingerprint.

A human override is valid only inside that person's authority. It SHOULD record
the overridden policy or diagnostic, rationale, affected work, risk accepted,
and review condition. An override MUST NOT fabricate missing evidence or erase
the previous decision.

## Shared PM Loop And Receipts

| Stage | Required output | Completion evidence |
| --- | --- | --- |
| Hydrate | premise, goal, policy, authority, current graph, source refs | bounded context with freshness |
| Compile | typed state plus errors, warnings, assumptions, decision debt | stable schema and diagnostics |
| Discover | opportunity, risk, hypothesis, or no-build finding | evidence and learning link |
| Decide | selected policy or residual decision packet | rationale, authority, alternatives |
| Plan | typed nodes, real dependencies, verification, non-goals | reviewable plan fingerprint |
| Execute | bounded ticket and PR lifecycle | GitHub branch, PR, checks, review receipts |
| Observe | delivery and outcome evidence | source-linked status or measurement |
| Replan | create, reuse, split, defer, supersede, or stop proposal | dry-run/apply receipt and reason delta |
| Validate | delivery acceptance and separate outcome validation | criterion-to-evidence verdict |
| Report | operator, stakeholder, handoff, or audit view | source schema versions and refs |

## Current Command And Schema Coverage

This map describes the current V3 baseline. `partial` means the surface is
useful but does not yet satisfy the complete PM policy.

| PM stage | Current surface | Contract | Coverage | V3 gap |
| --- | --- | --- | --- | --- |
| Hydrate | `pm bootstrap`, `pm context`, `ticket handoff`, `goal handoff`, `dispatch goal` | `pm-bootstrap/v1`, `pm-context/v1`, `worker-handoff/v1`, `goal-handoff/v1`, dispatch packet | supported | bootstrap binds policy, role, authority, source refs, fingerprints, and next protocol action without hidden thread memory |
| Compile | `pm compile`, `pm spec` | `pm-ir/v1`, `pm-compile-report/v1`, `gira-pm-task-packet/v1`, `gira-pm-task-packet/v2` | partial | deterministic intent diagnostics and profile-aware packets are implemented; automatic IR projection remains follow-up work |
| Discover | `pm record`, `pm discovery` | `pm-ledger-record/v1`, `pm-discovery-graph/v1` | supported | connect the graph to portfolio measurement and automatic replanning in later slices |
| Decide | `pm record`, decision policy helpers and queue resolution from #839 | `pm-ledger-record/v1`, `pm-record-report/v1`, `decision-policy/v1` | partial | append-safe decision state exists; option comparison and Goal routing integration remain follow-up work |
| Plan | `pm spec`, `goal plan --compact-json`, `goal graph` | `pm-task-profile/v1`, `pm-profile-promotion/v1`, `gira-pm-task-packet/v2`, `goal-plan/v1`, `goal-plan-compact/v1`, `pm-work-graph-report/v1`, `pm-work-graph-compact/v1` | supported | keep graph compilation deterministic without embedding an LLM planner |
| Execute | ticket lifecycle and queue take | readiness, approval, start, PR, checks, finish schemas | supported | consume typed PM profiles without weakening lifecycle gates |
| Observe | `pm observe`, `pm measure`, `goal status`, `goal report`, PM QA, workspace queues | `pm-observe-report/v1`, `pm-measurement-report/v1`, `goal-status/v1`, `goal-dossier/v1`, `gira-pm-qa/v1`, `workspace-queues/v1` | supported | portfolio aggregation remains follow-up work |
| Replan | `pm observe`, `pm replan` | `pm-observe-report/v1`, `pm-replan-report/v1` | supported | connect future portfolio-wide triggers without adding a background daemon or implicit mutation |
| Validate | `pm qa`, `pm accept` | `gira-pm-qa/v1`, `pm-acceptance-result/v1`, `pm-acceptance-report/v1` | supported | retain engineering review as a separate branch-protection responsibility |
| Report | `goal report --view operator|human|ai|stakeholder|audit`, workspace and release reports | `goal-dossier/v1`, `goal-pm-view/v1`, source schema refs | supported | portfolio aggregation and hosted presentation remain outside this local derived-view slice |
| Adapter | generic MCP CLI parity plus focused bootstrap, compile, observe, replan-plan, validate, and report reads | MCP tool envelopes over CLI; `pm-conformance-report/v1` | supported | model judgment remains host responsibility and is reported separately from protocol compliance |

### PM harness bootstrap and conformance

`gira pm bootstrap --repo OWNER/REPO --ticket N --role human|ai --json`
creates a bounded read-only session packet. It carries policy and protocol
versions, explicit authority evidence, source schema/digest references, current
compile and plan fingerprints, required receipts, and one next protocol action.
The `pms-*` session identifier is deterministic state evidence, not a credential
and not a substitute for mutation fingerprints.

Human CLI and MCP-assisted AI callers follow the same transition table. Focused
MCP PM tools only invoke the corresponding read or dry-run CLI command. Apply
continues through the normal Gira CLI contract: compile errors block lowering,
`--expect-plan` rejects stale graph/replan state, and approval/capability evidence
defines the mutation boundary.

`gira pm conformance` evaluates protocol versions, stages, receipts, supported
claims, contained weak-model failures, and privacy boundaries. Its protocol
verdict is deliberately separate from `semantic_quality`; transport compliance
does not claim that a model has strong PM judgment.
Importantly, tool access does not activate or prove PM protocol conformance; the bootstrap
and evaluator only make the required evidence and violations explicit.

### Durable PM Acceptance

`gira pm qa` remains a read-only review prompt. Its durable boundary is
`gira pm accept --from-file RESULT.json --dry-run|--apply`. The
`pm-acceptance-result/v1` input maps every criterion and implementation claim to
inspectable evidence, records delivery acceptance separately from product
outcome validation, and identifies the reviewed PR. Merge, check, test, or diff
evidence may support delivery acceptance but MUST NOT by itself set an outcome
to `validated`; outcome support requires an explicit measurement, metric, data,
research, customer, or experiment reference.

Delivery states distinguish `implementation_mismatch` from `spec_repair` so
the former produces delivery work and the latter produces decision work.
`inconclusive` outcome evidence produces measurement/observation work rather
than a success or failure declaration. Apply appends the immutable acceptance
result plus a typed ledger transition. Identical evidence is idempotent; changed
evidence receives a new content-derived ID and supersedes, rather than rewrites,
the prior verdict. `pm observe` consumes the latest valid verdict and routes it
back into the active PM action loop.

Unsupported coverage MUST be reported honestly. Adapters MUST NOT imply that a
prompt, template, or generic command transport enforces a PM capability that the
core does not implement.

### PM Intent Compiler Boundary

`gira pm compile` is the read-only front door for `pm-ir/v1`. It accepts intent
from `--intent` or `--from-file`; `--repo OWNER/REPO --goal N` may add explicit
Goal context. The compiler extracts recognized Markdown sections, preserves
their source spans, and labels values as `supplied`, `inferred`, `assumed`, or
`unresolved`. It does not infer an actor, problem, or outcome from free prose.
The only cross-source inference in v1 is an explicitly headed Goal objective
used as the desired outcome when the local intent leaves that field empty.

Compact text is the default and contains bounded diagnostics plus a command for
full detail. `--json` emits the complete `pm-compile-report/v1` with embedded
`pm-ir/v1`, source digests, provenance, all preserved statements, and stable
diagnostic codes. Compilation never creates files, issues, comments, or other
GitHub state. Reading optional Goal context does not broaden authority.

| Code | Meaning |
| --- | --- |
| `PM001_MISSING_ACTOR` | affected actor is unresolved |
| `PM002_MISSING_PROBLEM` | product or operational problem is unresolved |
| `PM003_MISSING_OUTCOME` | desired outcome is unresolved |
| `PM004_AMBIGUOUS_FIELD` | scalar sections disagree |
| `PM005_LOW_EVIDENCE` | no inspectable evidence reference is supplied |
| `PM006_CONFLICTING_STATE` | the same state is required and prohibited |
| `PM007_AUTHORITY_BOUND` | intent declares an apply-authority boundary |
| `PM008_UNSTRUCTURED_INTENT` | no PM headings are recognized |
| `PM009_MISSING_SUCCESS_CONDITION` | success cannot be evaluated |
| `PM010_UNRECOGNIZED_SECTION` | source is preserved but not semantically mapped |

Errors block reliable product evaluation, warnings expose material risk, and
info diagnostics describe safe compiler limitations. Codes and severities are
stable within `pm-compile-report/v1`.

`gira pm spec` remains compatible and continues rendering
`gira-pm-task-packet/v2` (or explicit `legacy` v1); compilation does not silently
rewrite either packet. Later projection work must consume `pm-ir/v1` explicitly and keep
mixed-version behavior visible.

### Typed PM Ledger Boundary

`gira pm record` appends one `pm-ledger-record/v1` as a typed GitHub issue
comment after `--dry-run`; `--apply` is the only mutation path. Records require
a stable ID, kind, statement, actor kind, timestamp, lifecycle status, and at
least one inspectable source reference. Kinds are context source,
evidence, inference, assumption, decision, question, learning, outcome,
opportunity, hypothesis, risk, and experiment.

Assumptions progress through `proposed`, `testing`, `supported`, `invalidated`,
or `expired`. Decisions progress through `proposed`, `accepted`, `superseded`,
`revoked`, or `review_due`. A new ID plus `supersedes` preserves the predecessor;
records are never silently overwritten. Identical retries are idempotent.
Conflicting IDs, missing predecessors, divergent successors, cycles, secrets,
and private transcript content fail closed with stable `PML001`-`PML006`
diagnostics.

`gira pm context` reads that append-only history into `pm-context/v1`, resolves
the current records deterministically, and emits a bounded reference-oriented
summary by default. `--json` expands the full typed history. Existing untyped
decision/evidence prose remains visible only as `legacy_evidence`; Gira does not
fabricate typed fields during migration. GitHub stays canonical and no private
transcript or secret belongs in the ledger.

### Discovery And Learning Graph

Discovery records form an inspectable, append-only trace: outcome ← opportunity
← hypothesis ← experiment ← learning ← decision. Opportunities `support`
outcomes, hypotheses `address` opportunities, experiments `test` hypotheses,
learning is `learned_from` experiments, and decisions are `based_on` learning.
Risks attach to hypotheses and use one proportionate type: value, usability,
feasibility, or viability. A task need not manufacture all four risk types.

Every hypothesis has a falsification test or an explicit proportionality waiver.
Completed experiments distinguish success, failure, inconclusive, and invalid.
Evidence strength (`anecdotal`, `qualitative`, `quantitative`, `replicated`) and
confidence (`low`, `medium`, `high`) remain separate fields; Gira does not hide
them in an aggregate score. Learning concludes validated, invalidated,
inconclusive, or no-build. An inconclusive experiment cannot validate a claim.
Outcome state is recorded separately from delivery completion.

`gira pm discovery` is read-only. Its compact default exposes current traces and
stable `PMD001`-`PMD008` diagnostics within a context budget; `--json` emits the
full graph, source refs, superseded history, and provenance. Missing targets,
invalid relation types, missing evidence, and false validation remain visible;
the command never fabricates a link or mutates GitHub.
Optional typed Goal references and task-profile links preserve where learning
belongs and which work contract it may promote into.

### Outcome Measurement And Validation

Measurement records attach to outcome records with `measures`. Each plan names
a leading, lagging, delivery, health, or guardrail signal and records evidence
type, baseline and definition, target/direction, bounded observation window,
source availability, owner, and decision rule. Quantitative and proportionate
qualitative evidence are both valid; qualitative plans additionally preserve
method, sample/context, and limits. When data is unavailable, an explicit
limitation and follow-up task replace a fabricated result.

`gira pm measure` deterministically reports validated, not-validated,
inconclusive, limited, or blocked outcome state. Delivery-only evidence cannot
validate a product outcome. Baseline/post-change definition drift produces a
comparability warning, and a regressed guardrail blocks validation even when a
primary target is met. Stable `PMM001`-`PMM011` diagnostics also expose missing
baselines, unbounded windows, unavailable sources, vanity metrics, and incomplete
qualitative evidence. Compact output is default; `--json` retains the full plan,
source refs, and diagnostics. PM QA and Goal dossiers include the report when
typed measurement state exists; rollout/measurement task-profile links keep
follow-up work connected without requiring an analytics warehouse.

### Typed Task Profiles And Promotion

`gira pm spec` defaults to the compact `delivery` profile in
`gira-pm-task-packet/v2`; `--profile legacy` retains the readable v1 universal
packet. All v2 profiles share actor, problem, desired outcome, goal alignment,
parent context, source references, and non-goals. Profile-specific contracts are:

| Profile | Required focus | Verification |
| --- | --- | --- |
| discovery | opportunity, evidence gap, research question | learning evidence |
| decision | one question, options, policy, authority | decision receipt |
| experiment | hypothesis, assumption, method, success/stop | experiment evidence |
| delivery | resolved product uncertainty, acceptance, boundary, dependencies | engineering verification |
| rollout | exposure plan, reversibility, guardrails, rollback | rollout evidence |
| measurement | signal, baseline, target, window, data source | measurement evidence |
| documentation | audience, knowledge gap, boundary, source of truth | documentation acceptance |

`pm-task-profile/v1` readiness replaces universal section checks only when its
marker is present. Discovery and experiment are not rejected for missing
implementation details. Delivery fails closed unless Product Uncertainty is
exactly `resolved`. `pm-profile-promotion/v1` additionally requires a ready
discovery, decision, or experiment source and its stable reference in both the
delivery Parent Context and Source References. Mixed-profile Goals evaluate each
child against its own marker; unmarked v1 packets continue through legacy
readiness. Ticket readiness reports invalid profiles and missing profile fields
without rewriting historical packets. Global Doctor remains a no-op because
profile validity is issue-local and already evaluated at ticket handoff.

## Compatibility And Migration

- Existing PM packets, Goal issues, queue names, and finish terminals remain
  readable until their owning implementation ticket provides migration.
- New policy language does not retroactively rewrite historical evidence.
- Compatibility states MAY be mapped to richer causal metadata, but the mapping
  MUST expose when the original source was broad or ambiguous.
- Each V3 schema or mutation change MUST document rollback, mixed-version
  behavior, and Doctor impact or an explicit Doctor no-op.

## External Practice Map

Gira translates selected practices rather than copying whole frameworks:

- SVPG product discovery: outcome focus and value, usability, feasibility, and
  viability risk;
- Continuous Discovery Habits: outcome, opportunity, solution hypothesis, and
  assumption tests;
- Shape Up: appetite, boundaries, rabbit holes, and no-gos;
- Working Backwards: customer experience and difficult questions before build;
- User Story Mapping: preserve user flow while slicing learnable releases;
- Goals-Signals-Metrics: connect intended outcomes to inspectable evidence;
- DACI: explicit decision authority, contributors, options, and outcome;
- GOV.UK Service Manual: evidenced user needs, baselines, measurement, and
  reporting as part of delivery.

These references inform policy rules and schema design. They MUST NOT expand
every ticket into a universal ceremony; task profiles should require only the
smallest contract relevant to the work.
