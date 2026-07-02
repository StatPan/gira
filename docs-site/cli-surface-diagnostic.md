# CLI Surface Diagnostic

Gira's user-facing command surface is large enough that new features need a
clear placement rule before they become commands.

Current baseline:

| Source | Count |
| --- | ---: |
| Root help entries | 31 |
| Registry-backed commands | 61 |
| Registry roots | 15 |
| Ticket subcommands | 15 |
| Read commands | 38 |
| Apply-mutation commands | 21 |

The root help is what humans feel first. The registry is larger because it also
feeds agents, generated docs, and adapter capabilities. These two surfaces
should be separated in teaching and review.

This page measures command surface size. For task-level UX/AX burden, use
[CLI Workflow Complexity](/cli-workflow-complexity).

## Diagnosis

Gira is better than raw `gh` when it reduces workflow state and required
natural-language prompting. `ticket start`, `ticket pr`, `ticket finish`,
`queue take`, `ops limit`, and `ticket new --parent` all bind GitHub state to a
next safe command.

Gira gets worse when provider primitives become new top-level concepts or when
several commands appear to answer the same question. The risky areas are root
help size, hierarchy vocabulary across `goal`/`epic`/`feature`, read-only view
overlap across `status`/`stats`/`report`, and compatibility aliases that make
the apparent command count larger.

## Rules

1. Add a root command only for a durable product noun that operators use often.
2. Prefer an option when behavior modifies an existing verb at the same
   decision point.
3. Prefer one state-oriented subcommand when an object needs show, set, and
   clear behavior.
4. Put runtime/provider diagnostics under `ops`, source and storage diagnostics
   under `config`, and shareable read-only artifacts under `report`.
5. Keep raw provider primitives out of Gira unless the wrapper adds policy,
   idempotency, workflow state, or a next safe command.
6. Teach canonical commands first. Aliases are compatibility affordances.

Native GitHub sub-issues follow this model: `ticket new --parent` and
`ticket parent` keep hierarchy inside the ticket lifecycle instead of adding a
new root command family.

Canonical source:
[docs/cli-surface-diagnostic.md](https://github.com/StatPan/gira/blob/main/docs/cli-surface-diagnostic.md).
