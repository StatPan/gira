# Portfolio Intake Layer

Gira's portfolio layer adds a project-agnostic backlog above repo execution issues. It keeps the implementation truth on GitHub, but lets operators capture product intent before the target repository or split is final.

## Ownership

- **Portfolio repo issues** own top-level tickets.
- **Execution repo issues** own implementation packets.
- **PRs** own code/doc change units and closing evidence.
- **Gira** owns read-only validation, lowering plans, and future explicit apply behavior.

The first implementation is dry-run only. It must not create, edit, close, or label GitHub issues.

## Config

Portfolio planning is scoped by `.gira/config.yaml`:

```yaml
repo: StatPan/gira
portfolio:
  repo: StatPan/gira-portfolio
  repos:
    - StatPan/gira
    - StatPan/docs
profiles:
  default:
    labels: ["type:task"]
```

- `portfolio.repo` is the GitHub repo whose issues are top-level tickets.
- `portfolio.repos` is the explicit allowlist of execution repos.
- Gira does not scan an entire GitHub org in this phase.

## Top-Level Ticket Contract

Each top-level ticket should use these fields in the issue body:

```md
## Goal
What outcome should exist?

## Scope
What is included and excluded?

## Routing
unrouted | single_repo | multi_repo | deferred

## Target Repos
- OWNER/REPO

## Acceptance Criteria
- Observable completion condition

## Child Issues
- OWNER/REPO#123
```

Required fields:

- `goal`
- `scope`
- `routing`
- `target_repos` for `single_repo` and `multi_repo`
- `acceptance_criteria`

Optional fields:

- `child_issues`
- `priority`
- `target_date`
- `non_goals`

## Commands

```bash
gira portfolio status --config .gira/config.yaml
gira portfolio validate --config .gira/config.yaml
gira portfolio plan --dry-run --config .gira/config.yaml
```

JSON variants are stable automation surfaces:

```bash
gira portfolio plan --dry-run --config .gira/config.yaml --json
```

Plan actions:

- `ticket:needs_routing`: the ticket is not ready to lower.
- `ticket:blocked_invalid_repo`: a target repo is invalid or outside the allowlist.
- `execution_issue:create`: a future apply command would create a repo execution issue.
- `execution_issue:link_existing`: linked child issues already exist and should be reused.

## Future Apply Boundary

A future `gira portfolio lower --apply` may create or link repo execution issues, but it must be:

- dry-run-first
- idempotent
- capability-gated
- explicit about target repos
- non-destructive

Out of scope for this phase:

- GitHub Projects v2 automation
- Web UI
- Jira import/export
- LLM decomposition
- org-wide repo discovery
