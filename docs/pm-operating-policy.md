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

This map describes the baseline at the start of V3. `partial` means the surface
is useful but does not yet satisfy the complete PM policy.

| PM stage | Current surface | Contract | Coverage | V3 gap |
| --- | --- | --- | --- | --- |
| Hydrate | `ticket handoff`, `goal handoff`, `dispatch goal` | `worker-handoff/v1`, `goal-handoff/v1`, dispatch packet | partial | no PM policy/session bootstrap or typed premise ledger |
| Compile | `pm spec` | `gira-pm-task-packet/v1` | partial | template rendering, no PM IR or diagnostic compiler |
| Discover | issue prose and research tickets | no dedicated contract | unsupported | opportunity, hypothesis, experiment, and learning types |
| Decide | decision policy helpers and queue resolution from #839 | `decision-policy/v1`, causal resolution metadata | partial | not integrated across PM state and Goal routing |
| Plan | `goal plan --compact-json` | `goal-plan/v1`, `goal-plan-compact/v1` | partial | manually supplied bullet decomposition and generic task packets |
| Execute | ticket lifecycle and queue take | readiness, approval, start, PR, checks, finish schemas | supported | consume typed PM profiles without weakening lifecycle gates |
| Observe | `goal status`, `goal report`, workspace queues | `goal-status/v1`, `goal-dossier/v1`, `workspace-queues/v1` | partial | delivery health without product learning or outcome confidence |
| Replan | manual issue edits and supersede | ticket supersede receipt | partial | no Goal-level evidence-triggered replan contract |
| Validate | `pm qa` | `gira-pm-qa/v1` prompt and verdict vocabulary | partial | verdict not persisted; delivery and outcome validation not separated |
| Report | Goal dossier, workspace and release reports | versioned report schemas | partial | premise, decisions, assumptions, learning, and plan deltas absent |
| Adapter | generic MCP CLI parity | MCP tool envelopes over CLI | partial | tool access does not activate or prove PM protocol conformance |

Unsupported coverage MUST be reported honestly. Adapters MUST NOT imply that a
prompt, template, or generic command transport enforces a PM capability that the
core does not implement.

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
