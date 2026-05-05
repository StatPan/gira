# Product OS Roadmap

Gira's Product OS phase extends the current Go CLI from repository bootstrap/sync/status tooling into a GitHub-native planning and lifecycle layer.

This roadmap keeps three constraints fixed:

1. GitHub remains the canonical execution backend.
2. All new Product OS automation is dry-run-first.
3. Capability/permission checks gate apply behavior before any mutation.

## Current Baseline

Shipped before this roadmap:

- Go CLI slices for `bootstrap`, `sync`, and `status`; the Go-built `gira` binary is the sole product implementation
- Product OS schema and roadmap date semantics in `docs/product-os-schema.md`
- Lifecycle transition matrix design for issue/PR/milestone/project state changes
- Permission model documented for future `project` commands

## Phase 1 — Capability-first foundation

### 1.1 Capability probe

Goal:
- add `gira project capability --repo OWNER/REPO [--json]`
- report effective repo/projects capability before any Product OS apply behavior exists

Exit criteria:
- deterministic text + JSON output
- denied capability cases covered in tests
- zero GitHub mutation

### 1.2 Transition planner dry-run

Goal:
- add `gira project transitions --repo OWNER/REPO --dry-run [--json]`
- compute lifecycle transitions from issue/PR/milestone/project evidence only

Exit criteria:
- stable plan entries with `rule_id`, `from`, `to`, `reason`, `conflict_resolution`
- no write behavior
- deterministic output for automation

## Phase 2 — Project OS dry-run orchestration

### 2.1 Project sync dry-run
Goal:
- add `gira project sync --repo OWNER/REPO --dry-run [--json]`
- combine capability report, field/view diff, roadmap date validation, and transition plan

Exit criteria:
- stable counts for create/update/skip/block
- roadmap date warnings and invalid-range blocks
- blocked actions tied to required capabilities

### 2.2 Conflict-aware ownership rules
Goal:
- preserve human/manual ownership and avoid overwriting explicit status decisions

Exit criteria:
- conflict reporting for manual overrides
- clear skip reasons in text/JSON output

## Phase 3 — Safe apply path

### 3.1 Partial apply with capability gates
Goal:
- allow `project sync` apply only for mutations that the active credential can perform

Exit criteria:
- allowed actions apply
- denied actions are skipped with stable diagnostics
- reruns remain idempotent

### 3.2 Milestone/project completion automation
Goal:
- safely annotate completion when all milestone work is done

Exit criteria:
- no destructive close/merge side effects beyond explicit supported actions
- completion semantics remain GitHub-native

## Parallel hardening track

### A. Target-repo-safe sync bootstrap issues (#25)
Goal:
- stop proposing Gira-self-specific bootstrap issues for arbitrary repos by default

Recommended shape:
- make bootstrap issue creation opt-in or template-specific

### B. Daily CLI usability (#12)
Goal:
- make installed `gira` the normal operator path outside the source checkout

Recommended shape:
- preserve `go install .../cmd/gira@latest` as the canonical path
- keep smoke-test commands in README/DX

## Epic #91 breakdown and execution order

Epic: #91 ([Full Jiraization of Gira](https://github.com/StatPan/gira/issues/91))

Child issues (implementation queue):

1. #92 Config schema for optional init profiles (YAML/TOML)
2. #93 Merge policy modes: adopt/merge/enforce for existing GitHub metadata
3. #94 Full CRUD capability matrix and command contract
4. #95 Jira-style workflow state machine mapped to GitHub-native objects
5. #96 Review/quality gate as command-first policy (no workflow dependency)
6. #97 End-to-end automation loop: issue queue -> implement -> verify -> PR -> merge
7. #98 Adoption/migration playbook for pre-configured GitHub repositories

Execution order rationale:

- Start with config and merge policy (#92, #93) so later commands have deterministic inputs.
- Define command contract before behavior expansion (#94).
- Implement state machine and quality gate policy next (#95, #96).
- Add the orchestration loop after primitives stabilize (#97).
- Finish with migration/adoption guidance once behavior is concrete (#98).

## Safety boundaries (explicit)

- No forced overwrite of pre-existing labels/milestones/issues/PR metadata without explicit opt-in.
- No hidden destructive delete behavior; destructive operations must be explicit and reviewable.
- Apply behavior remains capability-gated and idempotent where supported.

## Jira-vs-Gira operating boundary

The explicit capability matrix, top-level ticket -> repo issue -> PR contract, assistant/dev-agent ownership split, and proposed `gira work` UX layer are documented in [jira-gira-operating-boundary.md](jira-gira-operating-boundary.md).

## Portfolio intake phase

The next Jira gap is the project-agnostic backlog layer. Gira's first portfolio slice is read-only and dry-run-first: portfolio repo issues become top-level tickets, configured execution repos define the lowering allowlist, and `gira portfolio status|validate|plan --dry-run` reports how work would be routed without mutating GitHub. The contract is documented in [portfolio-intake.md](portfolio-intake.md).

The next implementation slice is portfolio lowering. It should land in this order:

1. `gira portfolio capability` checks for portfolio and execution repo issue access
2. `gira portfolio lower --dry-run` using the same action model as `portfolio plan`
3. `gira portfolio lower --apply` for idempotent execution issue creation and parent child-link updates
4. portfolio ticket templates and post-lowering validation UX

The lowering command is the first apply slice for the portfolio layer. The remaining work is to make valid ticket creation easier and make post-lowering summaries more operator-friendly.

Apply must remain bounded to GitHub issues and labels. Projects v2, Web UI/TUI, chat integrations, and LLM decomposition remain out of scope for v1; Jira import/export is handled by the `gira jira` migration command family.

## Non-goals during this roadmap

- no Jira-style separate database
- no web UI
- no destructive repo-wide cleanup automation
- no LLM PRD-to-issue decomposition in this phase
