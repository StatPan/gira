# Executable PRD v2: GitHub-Native PM Maximization (MVP/gh-first)

## 0. Purpose

This document defines the operating standard Gira uses when GitHub is the execution backend. It must be readable by humans and directly executable by automation agents.

The core principles are **MVP boundaries**, **gh-first execution**, and **idempotency**. This document is a product and operations contract; it does not itself require code changes or feature implementation.

---

## 1. Problem

GitHub Issues, Pull Requests, and Milestones can support project execution, but team-specific habits often create inconsistent workflows:

- Issues vary in execution quality because goals, scope, verification, or blocking conditions are missing.
- Automation agents cannot reliably determine which files to change or how completion should be judged.
- Repeated execution drifts as labels, milestones, and templates diverge.

The result is lower automation success and less predictable project operation.

---

## 2. Goals

1. Standardize Jira-style execution using GitHub-native objects: Issues, Pull Requests, Labels, and Milestones.
2. Establish an operating contract centered on the `gh` CLI.
3. Shape issues so agents can execute them without additional interpretation.
4. Guarantee idempotent operations when the same command or issue is processed more than once.

---

## 3. Non-goals

The MVP explicitly excludes:

- **GitHub Projects v2 automation**, including field, board, and automation creation or synchronization.
- Jira API integration, import/export, or bidirectional sync.
- New Web UI development.
- Slack or Discord PM bot integration.
- LLM-based PRD-to-issue decomposition.

---

## 4. MVP Scope

- Standardize the issue execution contract and templates.
- Standardize the minimum label taxonomy for priority, type, and status.
- Define milestone cadence, naming, and rollover rules.
- Define minimal PR linkage and merge gate rules, including `Closes #N`.
- Establish operations docs, issue drafts, and verification commands.

---

## 5. Principles

### 5.1 gh-first

- GitHub automation should prefer the `gh` CLI.
- Direct API calls are not the default MVP strategy.

### 5.2 Idempotency-first

- Repeating the same command or issue should not corrupt state.
- Examples:
  - Existing labels should become noops or updates, not hard failures.
  - Existing milestones should not be duplicated.
  - Template or documentation updates should exit with no changes when the diff is empty.

### 5.3 Explicit Blocker Reporting

- When automation needs a decision, permission, or external input, it must report the blocker immediately.
- **Fixed blocker_format:**
  - `BLOCKED: <reason> | needed: <specific decision/input> | owner: <person/role>`

---

## 6. Jira-style Mapping (MVP Canonical)

- Epic -> Parent Issue
- Story / Task / Bug -> Issue
- Sprint -> Milestone
- Development linkage -> PR body containing `Closes #N`, `Fixes #N`, or `Resolves #N`

GitHub Projects v2 status fields and automation mappings are reference-only in the MVP. They must not be automated by default.

---

## 7. Cron-executable Issue Contract

Every executable issue must include these seven fields:

1. `title`
2. `goal`
3. `scope`
4. `files_to_change`
5. `verification_commands`
6. `acceptance_criteria`
7. `blocker_format`

### Field Rules

- Field order is fixed: `title -> goal -> scope -> files_to_change -> verification_commands -> acceptance_criteria -> blocker_format`.
- `goal`: describe one outcome.
- `scope`: clearly define included and excluded work.
- `files_to_change`: use relative paths and avoid broad wildcards.
- `verification_commands`: use commands that can be reproduced locally.
- `acceptance_criteria`: use a checklist for completion conditions.
- `blocker_format`: include the fixed blocker string exactly.

### Executability Checks

- The title must show both the work type and target artifact.
- The scope must say what the issue does and at least one thing it does not do.
- Verification commands must have clear success or failure behavior; do not use prose as a command.
- Acceptance criteria should map directly to verification commands or produced artifacts.

### Good Example

```md
### title
[Task] Document issue execution contract rules

### goal
Define the standard rules that make every work issue use the same seven fields.

### scope
- Define required template fields
- Document field-level writing rules
- Exclude automated GitHub issue publishing

### files_to_change
- docs/prd/github-native-pm-maximization-prd-v2.md

### verification_commands
- git diff --check
- rg "title|goal|scope|files_to_change|verification_commands|acceptance_criteria|blocker_format" docs/prd/github-native-pm-maximization-prd-v2.md

### acceptance_criteria
- [ ] The seven fields are defined in the document.
- [ ] The fixed blocker_format string is included.

### blocker_format
BLOCKED: <reason> | needed: <specific decision/input> | owner: <person/role>
```

### Bad Example

```md
### goal
Clean up docs

### scope
- Change what is needed

### verification_commands
- Check it
```

Problems: missing required fields, no file boundary, unverifiable command, and unclear completion criteria.

This contract should stay aligned with `docs/ops/issue-drafts-github-native-pm-v2.md`.

---

## 8. Operating Policy (MVP)

### 8.1 Labels

- Minimum taxonomy:
  - `priority/P0`, `priority/P1`, `priority/P2`, `priority/P3`
  - `type/feature`, `type/bug`, `type/chore`, `type/docs`
- Label synchronization must be idempotent; existing labels should be preserved or corrected.

### 8.2 Milestones

- Cadence: choose one team-wide cadence, usually weekly or biweekly.
- Naming: use one team-wide pattern such as `YYYY-Www` or `YYYY-MM-SprintN`.
- Rollover: move unfinished issues to the next milestone and record the reason.

### 8.3 PR Merge Gate

- Required: related issue link such as `Closes #N`, plus passing tests or verification.
- Do not auto-merge when a blocking review exists.
- Escalate conflicts, permission issues, and policy issues using `blocker_format`.

---

## 9. MVP Success Metrics

- Issue-to-PR linkage rate is 95% or higher.
- Issue template required-field compliance is 95% or higher.
- Rerunnable operations fail less often over time.
- Average time spent in blocked state decreases.

---

## 10. Rollout Plan

1. Freeze the PRD v2 and three executable issue drafts.
2. Pilot in one repository for one to two weeks.
3. Measure template compliance, linkage rate, and blocker lead time.
4. Expand the same rules to additional repositories.

---

## 11. Definition of Done

- The PRD v2 documents MVP boundaries, gh-first execution, idempotency, and blocker_format.
- Three immediately executable issues satisfy the standard field contract.
- A team can reproduce the same operating model from the documentation alone.
