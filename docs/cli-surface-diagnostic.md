# CLI Surface Diagnostic

This is the #803 diagnosis for keeping Gira convenient while the command
surface grows. It uses the current help output and command registry as the
baseline, then turns the findings into decision rules for future work.

## Current Surface

As of this diagnosis:

| Source | Count | Notes |
| --- | ---: | --- |
| Root help entries | 31 | 27 daily entries, 1 setup entry, and 3 advanced entries are visible from `gira --help`. |
| Registry-backed commands | 61 | `gira guide capabilities --json` reports command facts used by adapters and generated docs. |
| Registry roots | 15 | `ticket`, `report`, `goal`, `jira`, `milestone`, `queue`, `feature`, `stats`, `workspace`, `pm`, `completion`, `config`, `dispatch`, `ops`, and `setup`. |
| Ticket subcommands | 15 | The daily lifecycle family is broad but cohesive: create, parent, inspect, start, PR, checks, note, review, supersede, and finish. |
| Capability classes | 38 read, 21 apply mutation, 1 dry-run mutation, 1 unsupported | Most public metadata is read-oriented; mutation surfaces are concentrated in lifecycle, setup, planning, and provider commands. |
| Alias-bearing commands | 6 | Aliases help compatibility but can make the apparent surface look larger than the canonical surface. |

The root help is the part most likely to feel large to a human. The command
registry is larger by design because it also feeds agents, docs, and adapter
capabilities. These two surfaces should not be judged the same way.

## Diagnosis

Gira is more convenient than raw `gh` when it reduces the amount of state a
person or agent must remember. The strong surfaces do this by binding a
workflow object to a next safe action:

- `gira ticket start` turns an issue into a branch and status transition.
- `gira ticket pr` binds branch, PR body, and closing reference.
- `gira ticket finish` checks PR state before completing the loop.
- `gira queue take` selects an agent-ready item and delegates to ticket start.
- `gira ops limit` explains API budget without putting noisy quota detail in
  daily commands.
- `gira ticket new --parent N` links hierarchy at creation time without making
  the user learn raw GitHub sub-issue calls.

Gira becomes less convenient when users must choose between many commands that
appear to answer the same question, or when a provider primitive becomes a new
top-level mental model. The risky areas are:

| Area | Risk | Keep/change recommendation |
| --- | --- | --- |
| Root help | `gira --help` already exposes many daily entries before a user understands the operating loop. | Keep root additions rare. Promote the golden path in docs and move compatibility or low-frequency surfaces out of the teaching path. |
| `ticket` | The family has 15 subcommands, but most map to one lifecycle object. | Keep it as the main lifecycle family. Prefer flags for creation-time modifiers, such as `--parent`, and state-oriented subcommands such as `ticket parent` for existing tickets. |
| `goal`, `epic`, `feature` | All can look like hierarchy or planning commands. | Define the boundary: goals are autonomy envelopes, epics are planning/progress views over grouped issues, and feature maps are optional capability records. Avoid adding duplicate child-link commands under all three. |
| `report`, `stats`, `status` | These are all read-only views and can feel interchangeable. | Keep `status` as immediate operational next action, `stats` as numeric workflow health, and `report` as shareable or exportable narrative output. |
| `work`, `dev`, `start`, and aliases | Aliases reduce migration pain but inflate perceived command count. | Treat aliases as compatibility surfaces. Teach canonical commands first and avoid adding aliases unless they protect an existing workflow. |
| `ops`, `config`, `audit` | Diagnostics can sprawl if each concern receives its own root. | Keep provider/runtime diagnostics under `ops`, source and storage diagnostics under `config`, and evidence/policy inspection under `audit` or existing lifecycle read commands. |
| GitHub Projects and provider commands | Provider features invite one-to-one CLI wrappers. | Wrap only Gira workflows. Leave low-level provider operations to `gh` unless Gira can add policy, idempotency, or next-action value. |

## Sub-Issue Case Study

Native GitHub sub-issues are a good example of a compact addition. The feature
could have become `gira subissue link`, `gira issue link`, or a new hierarchy
root. Instead, the current surface keeps the behavior inside the ticket family:

- `gira ticket new --parent N` handles the creation-time workflow.
- `gira ticket parent TICKET --set PARENT` changes an existing relationship.
- `gira ticket parent TICKET --clear` removes that relationship.
- `gira ticket parent TICKET` reads the current parent.

That shape is better than raw `gh` because the user only supplies ticket
numbers and dry-run/apply intent. It is better than a new command family because
sub-issue state remains part of the ticket lifecycle, not a separate product
concept.

## Compactness Rules

Use these rules before adding or exposing a command:

1. Add a root command only for a durable product noun that operators use often.
2. Prefer an option when the behavior modifies an existing verb at the same
   decision point, such as creation, filtering, output format, or safety mode.
3. Prefer one state-oriented subcommand when an object needs show, set, and
   clear behavior.
4. Put runtime/provider diagnostics under `ops`, storage and source diagnostics
   under `config`, and shareable read-only artifacts under `report`.
5. Keep raw provider primitives out of Gira unless the wrapper adds Gira policy,
   idempotency, workflow state, or a next safe command.
6. Teach canonical commands first. Aliases are compatibility affordances, not
   new product concepts.
7. Keep generated command facts in `internal/gira/command_registry.go`; use
   narrative docs only for decision boundaries, workflows, and tradeoffs.
8. A command is compact only if its output reduces the next prompt. It should
   say what happened, what is blocked, and which safe command to run next.

## Follow-Up Slices

These are separate tasks, not part of this diagnosis:

1. Add command surface metadata to the registry, such as `surface_class` and
   `teaching_priority`, so generated docs can separate daily, advanced,
   diagnostic, and compatibility surfaces.
2. Review root help grouping so compatibility and low-frequency commands do not
   compete with the first-run and ticket lifecycle path.
3. Reconcile the hierarchy vocabulary across `goal`, `epic`, `feature`, and
   native GitHub parent links.
4. Clarify the read-only view taxonomy for `status`, `stats`, and `report` in
   docs and help summaries.
5. Add a lightweight command-surface checklist to the public-command workflow so
   every new command states why it is a root, subcommand, option, diagnostic, or
   generated reference entry.

## Decision

The next implementation work should not add a new CLI command just to diagnose
surface size. The immediate action is to keep this diagnosis visible, then
split any consolidation work into narrow issues. New feature work should use
the compactness rules above before changing the public surface.
