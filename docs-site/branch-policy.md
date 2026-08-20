# Branch Policy

This page defines the branch policy contract for Gira ticket lifecycle
commands, including automatic branch selection and the explicit opt-in mode.

Gira should not make users repeat low-level `gh` or `git` branch flags at every
step. The Gira-native value is to resolve branch intent once, record it as
lifecycle state, and preserve and validate it through the full ticket flow.

## Decision

- Default branch policy mode: `github-flow`.
- Branch strategy is repo or workspace policy.
- `ticket start` resolves the intended base branch once.
- The resolved base branch is recorded as ticket lifecycle state.
- `ticket pr`, `ticket review`, `ticket status`, and `ticket finish` consume and
  validate the recorded base.
- `--base` is an explicit lifecycle override, not a required repeated flag.
- `ticket finish` must not mutate the local checkout by default.

The default `start_mode: auto` creates the suggested branch from the resolved
base (or from a detached checkout), and binds an existing non-base checkout
without checkout, rename, or push. The resolved base is never bound as a work
branch. Use `--branch auto|new|current|NAME` for deterministic selection;
`--create`, `--current`, and `--adopt BRANCH` remain compatibility spellings.
`--branch` and a compatibility spelling cannot be combined. Set
`start_mode: explicit` to require a selection; strategy-less apply then stops
before mutation while an explicit `--branch` remains valid.

## Built-In Modes

`github-flow` is the default mode. The base branch is usually the GitHub default
branch, commonly `main`.

`trunk` assumes short-lived work branches and strong CI discipline around fast
mainline integration.

`git-flow` supports `develop`, `release/*`, and `hotfix/*` branch families for
teams that separate integration and production branches.

`release-train` supports a primary mainline plus `release/*` or
environment-specific train branches for staged delivery.

`custom` lets a repository define policy when the built-in modes do not fit.

## Config Shape

`branch_policy` can be declared in repo-local config, global repo registry
entries, or global workspace registry entries. When it is absent, Gira resolves
the `github-flow` preset against the GitHub default branch.

Supported fields: `mode`, `default_base`, `development_base`,
`production_base`, `default_target`, `feature_branch_pattern`,
`start_mode`,
`release_branch_pattern`, `hotfix_branch_pattern`, `preserve_start_base`,
`forbid_implicit_current_branch_base`, `pr_base_source`, `finish_sync_local`,
and `targets`.

Minimal default:

```yaml
branch_policy:
  mode: github-flow
  start_mode: auto
```

Explicit GitHub-flow example:

```yaml
branch_policy:
  mode: github-flow
  default_base: main
  default_target: default
  feature_branch_pattern: issue/{number}-{slug}
  preserve_start_base: true
  forbid_implicit_current_branch_base: true
  pr_base_source: recorded_ticket_base
  finish_sync_local: false
  targets:
    default: main
    dev: main
```

Git-flow example:

```yaml
branch_policy:
  mode: git-flow
  default_base: develop
  development_base: develop
  production_base: main
  default_target: dev
  release_branch_pattern: release/*
  hotfix_branch_pattern: hotfix/*
  feature_branch_pattern: feature/{number}-{slug}
  preserve_start_base: true
  forbid_implicit_current_branch_base: true
  pr_base_source: recorded_ticket_base
  finish_sync_local: false
  targets:
    default: develop
    dev: develop
    production: main
```

Preset defaults:

| Mode | Default base | Development base | Production base | Default target |
| --- | --- | --- | --- | --- |
| `github-flow` | GitHub default branch | GitHub default branch | GitHub default branch | `default` |
| `trunk` | GitHub default branch | GitHub default branch | GitHub default branch | `dev` |
| `git-flow` | `develop` | `develop` | `main` | `dev` |
| `release-train` | GitHub default branch | GitHub default branch | GitHub default branch | `dev` |
| `custom` | GitHub default branch | GitHub default branch | GitHub default branch | `default` |

## Base Resolution Order

1. Explicit lifecycle override, such as `--base release/2.0`.
2. Recorded ticket lifecycle state, when the ticket has already started.
3. Issue metadata, when a base, release target, or train is explicitly recorded.
4. Milestone or release policy, when available.
5. Repo or workspace `branch_policy`.
6. GitHub default branch.

The current checkout branch is not an implicit base by default. A policy may
explicitly allow it, but the default contract forbids relying on accidental
local checkout state for branch base selection.

## Command Contract

`ticket start` resolves the base branch, validates it, creates or reuses the
feature branch from that base, records the resolved base, and moves the ticket
to `status:in-progress`.

`ticket pr` uses the recorded ticket base and passes the PR base explicitly to
GitHub. It should not let `gh` or GitHub defaults silently choose the base.

`ticket review` includes recorded base, actual PR base, and mismatch context in
the reviewer packet.

`ticket finish` validates that PR base matches recorded lifecycle base and must
not run local checkout, pull, sync, or cleanup by default. Local mutation
requires explicit opt-in and must target the recorded or actual PR base.

`ticket status` and doctor diagnostics should surface missing policy, unknown
policy mode, missing base branch, dirty worktree before mutation, recorded-base
vs actual-base mismatch, unsafe local sync configuration, and forbidden implicit
current-branch base selection.

## Lifecycle State

The resolved base branch must be recorded in ticket lifecycle state, not only in
local `.git/config`. It must survive reruns, new shells, and agent handoff. It
must also be visible in `ticket status --json`, reviewer packets, and doctor
diagnostics.

Gira stores the resolved base, start mode, selected work branch, and branch
source in the managed issue-body lifecycle block. Local git config can be a
cache, but not the source of truth.

## Backward Compatibility

Repos without `branch_policy` use `github-flow`.

Existing tickets without recorded lifecycle base should report the base as
unknown. If a linked PR exists, Gira may infer a candidate from PR `baseRefName`,
but it should mark that source as inferred. `ticket finish` should avoid local
checkout mutation unless explicit compatibility config enables it.
