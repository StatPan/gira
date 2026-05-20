# Branch Policy Contract

This document defines the branch policy contract for Gira ticket lifecycle
commands. Config schema and preset loading are implemented; lifecycle command
behavior is being delivered in the follow-up slices listed below.

Gira should not make users repeat low-level `gh` or `git` branch flags at every
step. The Gira-native value is to resolve branch intent once, record it as
lifecycle state, and then preserve and validate it through the ticket flow.

## Decision

Gira treats branch strategy as repo or workspace policy.

- The default branch policy mode is `github-flow`.
- Gira provides built-in branch policy presets, similar to its default label set.
- `ticket start` resolves the intended base branch once.
- The resolved base branch is recorded as ticket lifecycle state.
- `ticket pr`, `ticket review`, `ticket status`, and `ticket finish` consume and
  validate the recorded base.
- `--base` is an explicit lifecycle override, not a required repeated flag.
- `ticket finish` must not mutate the local checkout by default.

## Built-In Modes

### github-flow

Default mode. The base branch is usually the GitHub default branch, commonly
`main`.

Use this for OSS projects, CLI-first tools, small products, and fast PR-based
workflows.

### trunk

Mainline-oriented. This mode assumes short-lived work branches and strong CI
discipline around fast integration.

Use this when the team wants stricter convergence around a single integration
line.

### git-flow

Supports `develop`, `release/*`, and `hotfix/*` branch families.

Use this for versioned releases, packaged applications, or teams that
intentionally separate integration and production branches.

### release-train

Supports a primary mainline plus `release/*` or environment-specific train
branches.

Use this for staged delivery, beta/stable streams, and release batch workflows.

### custom

Explicit user-defined policy for repositories that do not fit a built-in mode.

## Config Shape

`branch_policy` can be declared in repo-local config, global repo registry
entries, or global workspace registry entries. When it is absent, Gira resolves
the `github-flow` preset against the GitHub default branch.

Supported fields:

| Field | Meaning |
| --- | --- |
| `mode` | One of `github-flow`, `trunk`, `git-flow`, `release-train`, or `custom`. |
| `default_base` | Base branch used by the default target. |
| `development_base` | Integration branch for development work, such as `develop`. |
| `production_base` | Production branch, commonly `main`. |
| `default_target` | Named target used when no explicit lifecycle target is provided. |
| `feature_branch_pattern` | Pattern for feature branch names. |
| `release_branch_pattern` | Pattern for release branch names. |
| `hotfix_branch_pattern` | Pattern for hotfix branch names. |
| `preserve_start_base` | Whether `ticket start` should preserve the resolved base. |
| `forbid_implicit_current_branch_base` | Whether current checkout may be used as an implicit base. |
| `pr_base_source` | Source for PR base selection. Currently `recorded_ticket_base`. |
| `finish_sync_local` | Whether finish may sync local checkout. Default is false. |
| `targets` | Named target to base branch map, for example `dev: develop`. |

Minimal `github-flow` shape:

```yaml
branch_policy:
  mode: github-flow
```

Explicit `github-flow` shape:

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

Explicit `git-flow` shape:

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

Custom release target example:

```yaml
branch_policy:
  mode: custom
  default_base: develop
  production_base: main
  default_target: dev
  targets:
    dev: develop
    release/2.0: release/2.0
    production: main
```

## Preset Defaults

| Mode | Default base | Development base | Production base | Default target | Branch patterns |
| --- | --- | --- | --- | --- | --- |
| `github-flow` | GitHub default branch | GitHub default branch | GitHub default branch | `default` | `issue/{number}-{slug}` |
| `trunk` | GitHub default branch | GitHub default branch | GitHub default branch | `dev` | `issue/{number}-{slug}` |
| `git-flow` | `develop` | `develop` | `main` | `dev` | `feature/{number}-{slug}`, `release/*`, `hotfix/*` |
| `release-train` | GitHub default branch | GitHub default branch | GitHub default branch | `dev` | `issue/{number}-{slug}`, `release/*` |
| `custom` | GitHub default branch | GitHub default branch | GitHub default branch | `default` | `issue/{number}-{slug}` |

## Base Resolution Order

Gira resolves the intended base branch in this order:

1. Explicit lifecycle override, such as `--base release/2.0`.
2. Recorded ticket lifecycle state, when the ticket has already started.
3. Issue metadata, when a base, release target, or train is explicitly recorded.
4. Milestone or release policy, when available.
5. Repo or workspace `branch_policy`.
6. GitHub default branch.

The current checkout branch is not an implicit base by default. A policy may
explicitly allow it, but the default contract forbids relying on accidental
local checkout state for branch base selection.

## Lifecycle State

The resolved base branch must be recorded in a place that ticket lifecycle
commands can read without depending on the current shell or local checkout.

The first implementation slice should decide the exact storage surface, but the
recorded state must support these properties:

- It is tied to the ticket, not only the local branch.
- It survives command reruns and new shells.
- It can be compared against the linked PR `baseRefName`.
- It is visible in `ticket status --json`.
- It can be surfaced in reviewer packets and doctor diagnostics.

Candidate storage surfaces include a structured marker in the issue body, a
managed issue comment, or a Gira-specific lifecycle metadata block. Local
`.git/config` may be useful as a cache, but it cannot be the sole source of
truth because Gira operates across machines and agents.

## Command Contract

### ticket start

`ticket start` resolves the base branch, validates it, creates or reuses the
feature branch from that base, records the resolved base, and moves the ticket
to `status:in-progress`.

Required behavior:

- Resolve base from the source-of-truth order.
- Validate that the base branch exists or return a clear diagnostic.
- Refuse accidental current-branch base selection when policy forbids it.
- Treat `--base` as an explicit lifecycle override.
- Surface dirty worktree risk before branch mutation.
- Record the resolved base as ticket lifecycle state.

### ticket pr

`ticket pr` uses the recorded ticket base and passes the PR base explicitly to
GitHub.

Required behavior:

- Do not let `gh` or GitHub defaults silently choose the base.
- Refuse or warn when an existing PR targets a different base.
- Keep draft and non-draft behavior unchanged except for explicit base
  preservation.
- Include the recorded base and actual PR base in JSON output when available.

### ticket review

`ticket review` includes branch policy context in the reviewer packet.

Required behavior:

- Include recorded base branch.
- Include actual PR base branch.
- Surface recorded-base vs actual-base mismatch as review context.
- Help reviewers verify that the implementation targets the intended release or
  mainline branch.

### ticket finish

`ticket finish` validates finish readiness without silently changing local
checkout state.

Required behavior:

- Validate that PR base matches the recorded lifecycle base.
- Keep existing checks, review, merge, close, provider, and receipt gates.
- Do not run `git checkout`, `git switch`, `git pull`, or local branch cleanup
  by default.
- Local sync or cleanup requires explicit opt-in through a flag or config value.
- Any opt-in local sync must target the recorded or actual PR base, not a
  hard-coded branch name.

### ticket status and doctor

`ticket status` and doctor-style diagnostics surface branch policy drift.

Required diagnostics:

- Missing branch policy.
- Unknown policy mode.
- Missing or inaccessible base branch.
- Dirty worktree before branch mutation.
- Recorded base vs actual PR base mismatch.
- Unsafe local checkout mutation configuration.
- Current branch being used as an implicit base when policy forbids it.

## Backward Compatibility

Repos without `branch_policy` config use `github-flow`.

For existing tickets that have no recorded lifecycle base:

- `ticket status` should report the base as unknown rather than inventing one.
- If a linked PR exists, Gira may infer a candidate from PR `baseRefName`, but it
  should mark that source as inferred.
- `ticket finish` should avoid local checkout mutation unless an explicit
  compatibility config enables it.
- `ticket pr` for already-started tickets without recorded base should fall back
  to the repo policy or GitHub default branch and report the source.

## Follow-Up Implementation Slices

The design should be implemented in narrow tickets:

- Branch policy config schema and preset loading.
- `ticket start` base resolution and lifecycle recording.
- `ticket pr` recorded base preservation.
- `ticket review` base context and mismatch reporting.
- `ticket finish` local mutation policy.
- `ticket status` and doctor branch policy diagnostics.

## Acceptance Questions

This contract is complete when it answers:

- What are the built-in branch policy presets?
- What is the default policy?
- What is the source-of-truth order for base selection?
- Where is the resolved base recorded?
- How do `ticket start`, `ticket pr`, `ticket review`, and `ticket finish`
  consume that recorded base?
- What local checkout mutations are allowed by default?
- What mismatch or error cases must be surfaced by status or doctor?
- What migration behavior applies to repos without branch policy config?
