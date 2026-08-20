# Command Surface Boundary

This document is the #779 and #781 baseline for keeping Gira's command surface
small while still exposing the diagnostics operators and agents need. The #803
inventory and risk diagnosis lives in
[docs/cli-surface-diagnostic.md](cli-surface-diagnostic.md). The UX/AX
task-level workflow complexity diagnosis lives in
[docs/cli-workflow-complexity.md](cli-workflow-complexity.md).

CLI surface cluster entry point: this is the current command-surface policy.
Read it first when deciding whether behavior belongs in a root command,
subcommand, option, diagnostic surface, generated reference entry, or no public
command. Use [CLI Surface Diagnostic](cli-surface-diagnostic.md) for the
current inventory and [CLI Workflow Complexity](cli-workflow-complexity.md) for
task-level UX/AX node counts.

Gira should stay CLI-first, but CLI-first does not mean every internal concern
gets a top-level command. Daily commands should guide work. Ops commands should
explain the machine.

## Discovery Tiers

Discovery tiers organize the taught path; they do not remove compatible commands
or change authorization.

| Tier | Start here when | Canonical agent entry point | Keep out of the default path |
| --- | --- | --- | --- |
| Assist | Reading GitHub state, diagnosing readiness, or choosing a next action. | Read the relevant `status`, `report`, or `review` output. | Mutating lifecycle commands and planning engines. |
| Managed Delivery | One GitHub issue is the bounded unit of work. | `gira ticket handoff TICKET --repo OWNER/REPO --json` | Goal, queue, and PM coordination unless the work is genuinely multi-ticket. |
| Advanced Orchestration | A Goal, a workspace-wide queue, or a durable PM protocol needs explicit coordination. | `gira dispatch goal GOAL --repo OWNER/REPO --compact-json` for Goal-level work. | Treating Goal, queue, or PM commands as prerequisites for a normal ticket. |

Compatibility paths remain supported but are not taught as the default. Examples
include `gira start` for `gira ticket start`, `gira work` for the ticket family,
`gira docs` for `gira guide`, and `gira goal dossier` for `gira goal report`.
The command registry records aliases and generated references label them so an
agent can normalize to the canonical command before policy evaluation.

## Surface Classes

| Surface | Purpose | Examples | Rule |
| --- | --- | --- | --- |
| Daily workflow | Move issue-backed work through the normal operating loop. | `gira status`, `gira ticket ...`, `gira queue ...`, `gira goal ...`, `gira workspace ...` | Keep output action-oriented and compact. |
| Ops diagnostics | Explain provider, runtime, setup, and operating constraints. | `gira ops ...` | Use for deep diagnostics that would make daily output noisy. |
| Audit/readiness | Inspect drift, evidence, and policy state. | `gira audit ...`, `gira ticket review`, `gira ticket status` | Prefer evidence and next action over broad raw dumps. |
| Config/storage | Explain local and global configuration sources. | `gira config storage`, `gira config doctor` | Keep source-of-truth and rebuild boundaries explicit. |
| Reference | Teach or enumerate command contracts. | `gira guide ...`, `gira completion`, generated command reference | Generated surfaces should come from `internal/gira/command_registry.go` when command facts are involved. |

## One-Word Ops Name For API Limits

API budget and rate-limit diagnostics belong under `gira ops`, not the root
surface. The selected short command name is:

```bash
gira ops limit
```

`limit` is short enough for agents and operators, while still pointing at the
user-visible problem: GitHub API limits. Follow-up implementation work may add
flags such as `--workflow ticket-lifecycle` or `--json`, but the command family
should remain under `ops`.

Do not add separate root commands such as `gira budget`, `gira api`, or
`gira rate-limit` for the #779 work. Those names make diagnostics look like a
new daily workflow surface.

## Daily Output Rule

Daily commands may surface API budget information only when it changes the next
safe action.

Good daily output:

```text
warning: GitHub API budget is low; inspect with gira ops limit
```

Avoid daily output that prints full REST, GraphQL, search, reset, and workflow
cost details during healthy operation. Detailed budgets, workflow estimates,
and secondary-limit explanations belong in `gira ops limit`.

## Adding A Public Command

Before adding a public command, answer these questions in the issue or PR:

1. Is this a daily workflow step, an ops diagnostic, a config/storage
   diagnostic, an audit/readiness report, or generated reference material?
2. Can an existing command family hold this behavior without ambiguity?
3. Will the command be used frequently enough to deserve a public surface?
4. Can the command output remain stable enough for agents or scripts?
5. Does the command need command-registry metadata and docs-site regeneration?
6. If the command mutates state, does it have a dry-run/apply boundary and
   approval evidence?

Prefer extending an existing family over adding a new root family. Prefer a
diagnostic subcommand under `ops` when the feature explains runtime conditions
rather than moving work forward.

Native GitHub sub-issue support follows this rule. Creation uses
`gira ticket new --parent N`, and existing-ticket changes use the single
`gira ticket parent` surface instead of separate root-level or duplicated
`link` and `unlink` command families.

Use [docs/cli-surface-diagnostic.md](cli-surface-diagnostic.md) when deciding
whether a new behavior should be a root command, subcommand, option, diagnostic
surface, generated reference entry, or no public command.

Use [docs/cli-workflow-complexity.md](cli-workflow-complexity.md) when deciding
whether a workflow surface actually reduces command, argument, decision,
provider, fallback, and cognitive nodes compared with raw `gh`.

## Follow-Up Constraints

For the #779 API limit work:

- #782 should add `gira ops limit` as the diagnostic entrypoint.
- #783 should keep workflow cost profiles static at first.
- #784 should put run-count estimates behind `gira ops limit --workflow ...`.
- #785 should add only concise low-budget warnings to daily commands.
- GitHub App authentication remains a blocked/future decision until a separate
  auth design resolves permissions, setup, token storage, and migration.
