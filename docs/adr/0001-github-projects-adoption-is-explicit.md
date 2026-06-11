# ADR 0001: GitHub Projects adoption is explicit

## Status

Accepted

## Date

2026-06-11

## Context

Gira uses GitHub repositories, issues, labels, milestones, branches, pull
requests, and optionally GitHub Projects as its operating surface. GitHub
Projects are visibility and planning surfaces, not the execution source of
truth.

GitHub Project visibility can be broader than repository ownership. A `gh`
token with Projects scope may list every visible user or organization Project
for an owner, including Projects that are unrelated to the repository currently
being adopted.

Before this decision, `gira adopt repo --strategy merge --dry-run` could turn
passively discovered owner-level Projects into a planned `projects:link` action.
That made a repository adoption plan look broader than the operator explicitly
requested.

## Decision

Gira must not treat passive GitHub Projects discovery as operator consent to
adopt, link, or sync a Project.

Repository adoption may report visible Projects as informational context, but
it must not plan Project linkage unless the operator explicitly selects a
Project through a dedicated command or explicit Project-selection inputs.

The supported explicit path today is:

```bash
gira workspace project adopt --dry-run
gira workspace project adopt --apply
```

Future `gira adopt repo` Project flags, if added, must preserve the same
principle: Project adoption is opt-in, narrowly scoped, and dry-run-first.

## Consequences

- `gira adopt repo` remains safe for multi-project organizations where a token
  can see unrelated Projects.
- `github.projects` in adopt reports remains useful discovery context.
- `projects:link` is no longer emitted from passive discovery alone.
- Operators who want a Project visibility surface must choose it explicitly.
- `gira projects sync` remains bounded by the configured workspace Project and
  workspace repo set.

## Related work

- #718: stop implicit Project linkage during repo adoption.
- #716: keep `projects sync` item-level mutations inside `workspace.repos`.
