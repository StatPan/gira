---
layout: home

hero:
  name: Gira
  text: GitHub-native work control plane
  tagline: Turn human and AI work into reviewable, auditable, evidence-backed completion.
  actions:
    - theme: brand
      text: Quick Start
      link: /quickstart
    - theme: alt
      text: Install
      link: /install

features:
  - title: Ticket lifecycle
    details: Create, start, review, check, wait, and finish issue-backed work with dry-run/apply safety.
  - title: Automatic branch selection
    details: Start from the base branch, an existing work branch, or a detached checkout without repeating low-level branch flags.
  - title: Agent handoff
    details: Give one issue to an external coding agent through a bounded worker handoff, then keep PR and check evidence in GitHub.
  - title: Readiness and audit
    details: Inspect policy, evidence, checks, review state, and the next safe action before mutation.
  - title: Advanced coordination
    details: Use workspace, queue, Goal, milestone, PM, and provider workflows only when the work needs them.
---

## Find The Smallest Path

| Need | Start here | Main commands |
| --- | --- | --- |
| Install and verify | [Install](/install), [Quick Start](/quickstart) | `gira version`, `gh auth status` |
| Create or adopt one ticket | [Ticket Workflow](/ticket-workflow) | `gira new`, `gira t n`, `gira ticket start` |
| Choose a branch safely | [Branch Behavior](/branch-policy) | `--branch auto|new|current|NAME` |
| Hand work to an agent | [Agent Operator](/agent-operator-skill) | `gira ticket handoff`, `gira dispatch goal` |
| Inspect before changing state | [Readiness And Audit](/readiness-audit), [Troubleshooting](/troubleshooting) | `gira ticket status`, `gira ticket review`, `gira doctor` |
| Coordinate larger work | [Workspace](/workspace), [Goal Mode](/goal-mode) | `gira queue`, `gira goal`, `gira milestone` |

## Command Families

| Family | Use it for | Docs |
| --- | --- | --- |
| `guide` | Built-in quickstart, ticket, stats, Jira, agent, skill, and concepts guides in the installed CLI. | [Quick Start](/quickstart), [Command Reference](/command-reference) |
| `setup`, `init`, `repo`, `adopt`, `config` | First-run setup, global registry entries, repo adoption, issue adoption, and config source diagnosis. | [Global Config](/global-config), [Troubleshooting](/troubleshooting) |
| `workspace`, `queue`, `projects`, `status` | Multi-repo workspace status, agent handoff queue selection, repo allowlist sync, existing Project mirroring, and compact read-only repo summaries. | [Workspace](/workspace), [Agent Handoff Queue](/agent-handoff-queue) |
| `feature`, `feat` | Optional issue-backed feature map listing, validation, and work issue linkage checks. | [Feature Map](/feature-map) |
| `ticket`, `start`, `work`, `dev` | Daily issue to branch to PR to finish lifecycle, plus compatibility aliases and lower-level helpers. | [Ticket Workflow](/ticket-workflow) |
| `goal`, `epic`, `milestone`, `sprint`, `release` | Larger objective tracking, child ticket selection, milestone batches, sprint planning, and release readiness. | [Goal Mode](/goal-mode), [Sprint And Release](/sprint-release) |
| `audit`, `jira` | Drift, readiness, provider compatibility, Jira mirror, transition planning, import, and export diagnostics. | [Readiness And Audit](/readiness-audit), [Jira Provider](/jira-primary-provider) |
| `stats`, `upgrade`, `cache`, `version` | Closure funnel metrics, release upgrade guidance, wrapper cache cleanup, installed binary inspection, and manual adoption signal snapshots. | [Closure Funnel Stats](/closure-funnel-stats), [Distribution](/distribution), [Adoption Signals](/adoption-signals) |
| `ops` | Advanced setup, migration, guardrails, portfolio, graph, review, merge, report, worker, and raw control surfaces. | [Command Reference](/command-reference) |

## Daily Loop

```bash
gh auth status
gira init --repo OWNER/REPO --path . --dry-run
gira adopt repo --repo OWNER/REPO --path . --strategy merge --dry-run
gira adopt repo --repo OWNER/REPO --path . --strategy merge --apply
gira new "TITLE" --goal "GOAL" --acceptance "done criteria" --start --dry-run
gira new "TITLE" --goal "GOAL" --acceptance "done criteria" --start --apply
gira ticket pr --dry-run
gira ticket pr --apply
gira ticket checks
gira ticket wait --timeout 5m
gira ticket finish --dry-run
gira ticket finish --apply
```

Gira is a Go-built CLI. Package managers are distribution channels for the same official binary, not alternate product runtimes.

`gira init --dry-run` is the read-only onboarding plan. Apply the repository
adoption separately with `gira adopt repo --dry-run` followed by
`gira adopt repo --strategy merge --apply` (or use the exact strategy emitted
by the preview).

`gira new` and `gira t n` are the short ticket aliases. They retain the
canonical command's dry-run/apply behavior. The default branch mode is `auto`:
Gira creates a suggested branch from the resolved base, reuses an existing
non-base checkout without renaming or pushing it, and starts safely from a
detached checkout. Use `--branch new|current|NAME` when the choice must be
explicit. See [Branch Behavior](/branch-policy) for the full contract.

The shortest loop uses a non-draft PR. Add `--draft` when review must happen
before a PR is ready; a finish preview for a draft PR only marks it ready, so
run `gira ticket finish --dry-run` again after that transition before applying
the final preview.

Operation policy changes the enforcement level, not the evidence shape:
`observation` reports neutral provider facts, managed `advisory` reports Gira
conventions as warnings, and managed `required` enforces those conventions.
See [Readiness And Audit](/readiness-audit) for the policy fields and finding
classes.

Use [workspace status](/workspace) for a bounded multi-repo view, and [goal
mode](/goal-mode) when a larger objective needs child-ticket convergence.

Goal, PM, provider, version, diagnostics, and architecture material stays
available under [Advanced Orchestration](/goal-mode) and [Reference](/command-reference);
it is intentionally secondary to the daily ticket path.

The [Worker Boundary](/worker-boundary) keeps Gira focused on contracts,
readiness, review packets, provenance, and finish convergence while external
coding agents execute the work.
