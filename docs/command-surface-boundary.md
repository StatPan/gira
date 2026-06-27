# Command Surface Boundary

This document is the #779 and #781 baseline for keeping Gira's command surface
small while still exposing the diagnostics operators and agents need.

Gira should stay CLI-first, but CLI-first does not mean every internal concern
gets a top-level command. Daily commands should guide work. Ops commands should
explain the machine.

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

## Follow-Up Constraints

For the #779 API limit work:

- #782 should add `gira ops limit` as the diagnostic entrypoint.
- #783 should keep workflow cost profiles static at first.
- #784 should put run-count estimates behind `gira ops limit --workflow ...`.
- #785 should add only concise low-budget warnings to daily commands.
- GitHub App authentication remains a blocked/future decision until a separate
  auth design resolves permissions, setup, token storage, and migration.

