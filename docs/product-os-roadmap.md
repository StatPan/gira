# Product OS Roadmap

Gira's Product OS phase extends the current Go CLI from repository bootstrap/status tooling into a GitHub-native planning and lifecycle layer.

This roadmap keeps three constraints fixed:

1. GitHub remains the canonical execution backend.
2. All new Product OS automation is dry-run-first.
3. Capability/permission checks gate apply behavior before any mutation.

## Current Baseline

Shipped before this roadmap:

- Go CLI parity for `bootstrap`, `sync`, and `status`
- Product OS schema and roadmap date semantics in `docs/product-os-schema.md`
- Lifecycle transition matrix design for issue/PR/milestone/project state changes
- Permission model documented for future `project` commands

## Phase 1 — Capability-first foundation

### 1.1 Capability probe
Status: in progress on `feat/issue-35-project-capability`

Goal:
- add `gira project capability --repo OWNER/REPO [--json]`
- report effective repo/projects capability before any Product OS apply behavior exists

Exit criteria:
- deterministic text + JSON output
- denied capability cases covered in tests
- zero GitHub mutation

### 1.2 Transition planner dry-run
Status: next

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

## Suggested execution order

1. #35 capability probe
2. transition planner dry-run
3. project sync dry-run
4. #25 target-repo-safe sync
5. partial apply gates
6. milestone/project completion automation
7. #12 daily CLI polish as ongoing hardening

## Non-goals during this roadmap

- no Jira-style separate database
- no web UI
- no destructive repo-wide cleanup automation
- no LLM PRD-to-issue decomposition in this phase
