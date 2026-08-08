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
  - title: Setup and adoption
    details: Initialize global config, register repos, adopt existing GitHub issues, and keep shared repo contracts separate from personal operator state.
  - title: Workspace visibility
    details: Inspect multi-repo workspaces, sync repo allowlists, see bounded status reads, and surface agent, review, finish, blocked, and failed-check queues.
  - title: Ticket lifecycle
    details: Create, start, review, check, wait, note, supersede, and finish GitHub issue-backed work with dry-run/apply safety.
  - title: Goals, epics, and milestones
    details: Plan child tickets, select the next safe goal item, inspect epic progress, and use milestones as sprint or release boundaries.
  - title: Readiness and audit
    details: Use ticket, PR, finish, provider, and drift reports to make missing evidence, blockers, checks, and review state explicit.
  - title: 2.0 control-plane contract
    details: Stabilize ticket lifecycle, goal mode, workspace queues, readiness reports, and adapter approval evidence without introducing a hidden planning database.
  - title: Distribution and providers
    details: Install the same Go-built binary through release archives, npm, PyPI, Homebrew, or install.sh, with optional Jira-primary provider mode.
---

## Current Feature Map

| Need | Start here | Main commands |
| --- | --- | --- |
| Install, upgrade, or inspect adoption signals | [Install](/install), [Distribution](/distribution), [Adoption Signals](/adoption-signals) | `gira version`, `gira upgrade`, `gira cache prune` |
| Set up a repo or personal workspace | [Quick Start](/quickstart), [Global Config](/global-config) | `gira init`, `gira setup global`, `gira repo register`, `gira adopt repo` |
| See work across repos | [Workspace](/workspace) | `gira workspace status`, `gira workspace repos sync` |
| Select LLM-ready work | [Agent Handoff Queue](/agent-handoff-queue) | `gira queue list`, `queue next`, `queue handoff`, `queue take --dry-run` |
| Run issue to PR work | [Ticket Workflow](/ticket-workflow) | `gira ticket new`, `start`, `pr`, `review`, `checks`, `wait`, `finish` |
| Maintain an optional feature map | [Feature Map](/feature-map) | `gira feature list`, `feature check`, `feature for` |
| Manage larger work packets | [Goal Mode](/goal-mode), [Sprint And Release](/sprint-release) | `gira goal status`, `goal next`, `goal finish`, `epic list`, `milestone plan` |
| Understand the 2.0 contract | [Gira 2.0 Control Plane](/v2-control-plane), [State Model](/state-model) | `gira ticket status`, `gira goal status`, `gira workspace status --json` |
| Plan the first 3.0 surface | [Gira 3.0 Local Report Bundle](/gira-3-local-report-bundle) | `gira export dashboard`, `gira goal report --html` |
| Diagnose readiness and drift | [Readiness And Audit](/readiness-audit), [Troubleshooting](/troubleshooting) | `gira ticket status`, `ticket review`, `audit drift`, `jira doctor` |
| Map Jira concepts to GitHub | [Jira Mapping](/jira-mapping), [Jira Provider](/jira-primary-provider) | `gira jira init`, `jira mirror`, `jira transition`, `jira import`, `jira export` |

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
gira adopt repo --repo OWNER/REPO --path . --dry-run
gira ticket new "TITLE" --goal "GOAL" --acceptance "done criteria" --apply
gira ticket start TICKET --create --apply
gira ticket pr --apply --draft
gira ticket checks
gira ticket wait --timeout 5m
gira ticket finish --apply
```

Gira is a Go-built CLI. Package managers are distribution channels for the same official binary, not alternate product runtimes.

Use [Jira-primary provider mode](/jira-primary-provider) only when Jira already owns planning and status for a repo. GitHub-native mode remains the default.

Use [workspace status](/workspace) when a personal operator needs a bounded multi-repo view, and [goal mode](/goal-mode) when a larger objective needs child-ticket convergence before handoff.

The [Gira 2.0 control-plane contract](/v2-control-plane) is CLI-first. The
first 3.0 surface is a [local report bundle](/gira-3-local-report-bundle) over
stable state contracts. The future hosted direction is documented as a bounded
[control-plane roadmap](/hosted-control-plane), not as a replacement for the
CLI.

Gira's [worker boundary](/worker-boundary) keeps the product focused on
contracts, readiness, review packets, provenance, and finish convergence while
external coding agents execute the work.
