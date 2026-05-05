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
gira portfolio capability --config .gira/config.yaml
gira portfolio status --config .gira/config.yaml
gira portfolio validate --config .gira/config.yaml
gira portfolio plan --dry-run --config .gira/config.yaml
gira portfolio lower --dry-run --config .gira/config.yaml
gira portfolio lower --apply --config .gira/config.yaml
```

JSON variants are stable automation surfaces:

```bash
gira portfolio capability --config .gira/config.yaml --json
gira portfolio plan --dry-run --config .gira/config.yaml --json
gira portfolio lower --dry-run --config .gira/config.yaml --json
```

Plan actions:

- `ticket:needs_routing`: the ticket is not ready to lower.
- `ticket:deferred`: the ticket is intentionally delayed and should not lower yet.
- `ticket:blocked_invalid_repo`: a target repo is invalid or outside the allowlist.
- `execution_issue:create`: a future apply command would create a repo execution issue.
- `execution_issue:link_existing`: linked child issues already exist and should be reused.
- `execution_issue:ambiguous_existing`: multiple matching execution issues exist for the same parent/target pair.
- `portfolio_ticket:update_child_issues`: a future apply command would append missing child issue links to the parent portfolio ticket.

`gira portfolio plan --dry-run --json` includes the same capability summary used by `gira portfolio capability` plus `permission_blocks` for planned actions that cannot safely apply with the active credential.

## Lowering Contract

`gira portfolio lower` is the only planned mutation path from portfolio tickets to repo execution issues.

The command is dry-run-first:

- `gira portfolio lower --dry-run` computes the exact create/link/skip/block actions and performs no GitHub mutation.
- `gira portfolio lower --apply` may perform only the actions shown by the same dry-run under the same config and credential capability.
- `--apply` and `--dry-run` are mutually exclusive. One of them is required.
- JSON output is data-only. Human output ends with one next-step line.
- Dry-run and apply must both perform the same discovery steps, including target repo searches for existing `## Gira Lowering` evidence. Apply must not discover a matching execution issue that dry-run would have missed.

Supported ticket states:

| Ticket condition | Lowering behavior |
| --- | --- |
| open + `single_repo` + one valid target repo + no child issue | create one execution issue |
| open + `single_repo` + one valid child issue for the target repo | link/reuse the existing child issue |
| open + `multi_repo` + valid target repos + no child issues | create one execution issue per target repo |
| open + `multi_repo` + some valid child issues | link/reuse existing child issues and create only missing target repo issues |
| open + all target repos already linked | no execution issue creation; optionally update missing parent child-link evidence |
| open + `unrouted` | skip with `ticket:needs_routing` |
| open + `deferred` | skip with `ticket:deferred` |
| invalid schema, invalid target repo, or invalid child issue | block with diagnostics; no mutation for that ticket |
| closed portfolio ticket | skip; no lowering action |

Execution issue body:

```md
## Goal
Copied from the portfolio ticket.

## Scope
Copied from the portfolio ticket, narrowed to this repo when possible.

## Acceptance Criteria
Copied from the portfolio ticket.

## Files To Change
Unknown until refined.

## Verification Commands
Unknown until refined.

## Blocker Format
Comment on this issue with the blocker, attempted command, and required decision.

## Parent Ticket
OWNER/PORTFOLIO_REPO#123

## Gira Lowering
portfolio_repo: OWNER/PORTFOLIO_REPO
portfolio_ticket: 123
target_repo: OWNER/REPO
```

Lowered issues are execution shells, not always worker-ready implementation packets. If `files_to_change` or `verification_commands` are unknown, the issue should remain `status:ready` only for triage/refinement, not direct worker handoff. A later UX slice may add `status:needs-design` or `status:needs-refinement` if the label taxonomy supports it.

Required labels for created execution issues:

- `type:task`
- `status:ready` when enough implementation detail exists
- `status:needs-design` when the repo has that label and files/verification are unknown
- `agent:worker` only when the issue is ready for direct implementation

If a required label is missing, apply must either create it through an existing label-sync path or block with a capability/remediation diagnostic. It must not silently create ad hoc labels outside the configured Gira taxonomy.

## Idempotency

Lowering must be rerunnable without duplicate execution issues.

Idempotency keys:

- parent portfolio ticket: `OWNER/PORTFOLIO_REPO#N`
- target repo: `OWNER/REPO`
- execution issue evidence: a `## Gira Lowering` block containing `portfolio_repo`, `portfolio_ticket`, and `target_repo`

Before creating an execution issue, dry-run and apply must search the target repo for an open or closed issue containing matching lowering evidence. If one exists, the action becomes `execution_issue:link_existing`. If multiple matches exist, the ticket is blocked with `execution_issue:ambiguous_existing` and no mutation is performed for that target repo.

The portfolio ticket's `## Child Issues` list is useful evidence but not the only idempotency source. Gira must not rely solely on parent body links because a user may edit the portfolio ticket manually.

## Parent Updates

The first apply implementation may update the portfolio ticket only to append missing child issue links under `## Child Issues`.

Dry-run must show this as an explicit `portfolio_ticket:update_child_issues` action. Apply must not update the parent unless that action is present in dry-run output.

Parent updates must be:

- dry-run-first
- idempotent
- capability-gated
- explicit about target repos
- non-destructive

Parent updates must not rewrite goal, scope, routing, target repos, acceptance criteria, priority, target date, or non-goals. If the `## Child Issues` section is missing, apply may either append a new section at the end or block with a remediation diagnostic. The chosen behavior must be covered by tests before apply ships.

## Capability Gates

`portfolio lower --apply` requires:

- read access to `portfolio.repo`
- issue read access to target execution repos for the tickets being lowered
- issue write access for target repos that need creation
- issue write access to the portfolio repo only if parent child-link updates are enabled

Denied or unknown capability must produce stable diagnostics and skip mutation for the affected repo. A denied configured repo does not block unrelated tickets that do not target that repo. For a multi-repo ticket, denied capability blocks only the affected target repo action; independent target repo actions may still apply when dry-run marks the denied repo as blocked and the remaining actions are idempotent.

## Future Apply Boundary

Out of scope for this phase:

- GitHub Projects v2 automation
- Web UI
- Jira import/export
- LLM decomposition
- org-wide repo discovery
