# Issue-Backed Feature Map

Gira's feature map is an optional capability map over GitHub issues. It is for
operators who want to keep a durable view of product capabilities while still
using milestones and tickets as the delivery queue.

The feature map is not required for normal ticket lifecycle work. If a repo has
no feature records, `gira feature check` reports the map as not configured and
does not block work.

## Ownership

| Surface | Role |
| --- | --- |
| GitHub issue | Canonical feature or capability record. |
| GitHub Project | Visibility view for PM-style map, roadmap, and todo views. |
| Milestone | Delivery batch for executable work issues. |
| PR | Implementation evidence for a linked work issue. |
| Docs | Optional snapshot or public contract. |
| Gira | Read-only checker/compiler for links, sections, and maturity state. |

This keeps the Project screen useful without making Project-only items the
source of truth.

## Feature Records

A feature record is a GitHub issue that either:

- has `type:capability` or `type:feature`; or
- has a title beginning with `Capability:` or `Feature:`.

Recommended body shape:

```markdown
# Capability: Ticket lifecycle

Key: tl
Status: stable

## User Need

## Capability

## Surface

## Docs

## Evidence
```

Allowed maturity values are:

- `optional`
- `planned`
- `preview`
- `stable`
- `legacy`
- `deprecated`

Maturity can be recorded as a body line such as `Status: stable`, or as a label
such as `capability:stable` or `feature:stable` when the repo chooses to create
those labels. `Key:` is a short daily identifier so operators can avoid typing
long slugs.

## Work Links

Executable work remains normal Gira tickets. A work issue can link to a feature
record with a readable body line:

```markdown
Related capability: #31
```

or:

```markdown
Feature: #31
```

In the first slice, Gira only reads these links. Mutation helpers such as
`gira feat link` are planned separately.

## Commands

```bash
gira feature list --repo OWNER/REPO
gira feature check --repo OWNER/REPO
gira feature for 123 --repo OWNER/REPO
```

`gira feat` is the short alias for daily use:

```bash
gira feat check
gira feat for 123
```

`check` validates feature records and work links. In optional mode, missing
feature links are diagnostics, not ticket readiness blockers.
