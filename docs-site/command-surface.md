# Command Surface

Gira should stay CLI-first, but CLI-first does not mean every internal concern
gets a top-level command. Daily commands should guide work. Ops commands should
explain the machine.

## Surface Classes

| Surface | Purpose | Examples |
| --- | --- | --- |
| Daily workflow | Move issue-backed work through the normal operating loop. | `gira status`, `gira ticket ...`, `gira queue ...`, `gira goal ...`, `gira workspace ...` |
| Ops diagnostics | Explain provider, runtime, setup, and operating constraints. | `gira ops ...` |
| Audit/readiness | Inspect drift, evidence, and policy state. | `gira audit ...`, `gira ticket review`, `gira ticket status` |
| Config/storage | Explain local and global configuration sources. | `gira config storage`, `gira config doctor` |
| Reference | Teach or enumerate command contracts. | `gira guide ...`, `gira completion`, generated command reference |

## API Limit Diagnostics

API budget and rate-limit diagnostics belong under `gira ops`, not the root
surface. The selected short command name is:

```bash
gira ops limit
```

Do not add separate root commands such as `gira budget`, `gira api`, or
`gira rate-limit` for the API limit work. Those names make diagnostics look
like a new daily workflow surface.

## Daily Output

Daily commands may surface API budget information only when it changes the next
safe action.

```text
warning: GitHub API budget is low; inspect with gira ops limit
```

Detailed budgets, workflow estimates, and secondary-limit explanations belong
in `gira ops limit`.

## Adding A Public Command

Before adding a public command, decide whether it is daily workflow, ops
diagnostic, config/storage diagnostic, audit/readiness, or reference material.
Prefer extending an existing family over adding a new root family.

Commands that mutate state still need dry-run/apply boundaries and approval
evidence. Command facts should be added to `internal/gira/command_registry.go`
when the public surface needs generated docs or adapter metadata.

Canonical source: [docs/command-surface-boundary.md](https://github.com/StatPan/gira/blob/main/docs/command-surface-boundary.md).

For the current inventory, risks, and follow-up slices, see
[CLI Surface Diagnostic](/cli-surface-diagnostic).

For task-level UX/AX node counts, see
[CLI Workflow Complexity](/cli-workflow-complexity).
