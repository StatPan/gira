# Global Config Registry

This document defines the Gira global config terminology and precedence
contract for #339. It is the decision input for the implementation slices that
add global registry support.

## Decision Summary

Gira should use `global` as the user-facing term for the per-OS-user config
registry, similar to `git config --global`. `global` does not mean GitHub-wide,
organization-wide, or machine-wide for every OS account. It means the current
operating-system user's Gira config area.

Gira should keep the product concept of a "Gira home" or "global registry",
but default physical storage should follow OS-standard config, cache, and state
locations instead of putting every file under one `~/.gira` directory.

On Linux/XDG-style systems, the default roots are:

```text
~/.config/gira        durable config and registry
~/.cache/gira         disposable cache
~/.local/state/gira   runtime state, logs, locks, and recent-run state
```

Future implementations may allow an explicit `GIRA_HOME` override for operators
who want a single root, but the default should keep config, cache, and state
separate.

Repo-local `.gira/config.yaml` remains supported as an optional shared repo
contract. Global registry support should be additive first. A later default
flip can prefer the global registry when available, after migration guidance and
diagnostics are in place.

## Terms

| Term | Meaning |
| --- | --- |
| Global config | OS-user global Gira config, analogous to `git config --global`. |
| Global registry | Repo and workspace entries under the global config root. |
| Gira home | Product concept for Gira's user-owned operating area. It may map to several OS-standard roots. |
| Repo registry entry | Per-repo personal metadata such as checkout path, aliases, defaults, and optional contract reference. |
| Workspace registry entry | Personal grouping of inbox and execution repos. |
| Repo-local contract | Optional shared `.gira/config.yaml` committed to a repo to declare shared policy. |
| Local override | Optional `.gira/config.local.yaml`, gitignored and machine-specific. |

## Default Global Layout

Durable config and registry:

```text
~/.config/gira/
  config.yaml
  repos/
    OWNER/
      REPO.yaml
  workspaces/
    NAME.yaml
```

Disposable cache:

```text
~/.cache/gira/
```

Runtime state:

```text
~/.local/state/gira/
```

The global repo registry entry may reference a repo-local contract instead of
copying every shared policy field:

```yaml
repo: StatPan/gira
path: ~/workspace/apps/gira
contract: .gira/config.yaml

defaults:
  agent: codex
  assignee: ilgukim

workspace:
  name: personal
```

For normal global-first setup, prefer the intention-based setup flow. It
creates the global config, workspace registry entry, and repo registry entry in
one reviewed plan:

```bash
gira setup global \
  --repo StatPan/gira \
  --path ~/workspace/apps/gira \
  --workspace personal \
  --inbox-repo StatPan/backlog \
  --mode global-only \
  --dry-run

gira setup global \
  --repo StatPan/gira \
  --path ~/workspace/apps/gira \
  --workspace personal \
  --inbox-repo StatPan/backlog \
  --mode global-only \
  --apply
```

Use `--mode hybrid` when the repo-local `.gira/config.yaml` should remain a
referenced shared contract. Use the lower-level commands below only when you
need to compose the registry primitives manually.

`--inbox-repo` is the backlog/intake repository for work that has not yet been
assigned to an execution repo. It can match `--repo` for a small single-repo
setup, but multi-repo global operation should usually use a dedicated repo such
as `OWNER/backlog`.

After the first setup, populate the workspace execution repo allowlist from a
GitHub user or organization:

```bash
gira workspace repos sync --owner StatPan --workspace personal --dry-run
gira workspace repos sync --owner StatPan --workspace personal --apply
```

This command updates `~/.config/gira/workspaces/NAME.yaml` from reviewed GitHub
repo discovery. It skips the configured `workspace.inbox_repo` because that repo
is backlog/intake rather than an execution target. Add `--include-archived` only
when archived repositories should be listed too.

Create the personal workspace registry without writing repo files:

```bash
gira workspace init --scope global \
  --name personal \
  --inbox-repo StatPan/backlog \
  --repo StatPan/gira \
  --dry-run

gira workspace init --scope global \
  --name personal \
  --inbox-repo StatPan/backlog \
  --repo StatPan/gira \
  --apply
```

Register a checkout in the global repo registry:

```bash
gira repo register StatPan/gira --path ~/workspace/apps/gira --dry-run
gira repo register StatPan/gira --path ~/workspace/apps/gira --apply
```

## Target Selection Order

Target selection decides which GitHub repo or workspace a command operates on.
It should be deterministic and explainable in `gira config doctor`.

Recommended order:

1. Explicit CLI flags:
   - `--repo`
   - `--config`
   - `--workspace`
2. Current git checkout:
   - infer `OWNER/REPO` from the GitHub `origin` remote.
3. Global repo registry:
   - match the inferred repo, registered checkout path, or registered alias.
4. Global workspace registry:
   - use an explicitly selected workspace, configured default workspace, or a
     workspace containing the selected repo.
5. Repo-local contract fallback:
   - read `.gira/config.yaml` or `.gira/config.toml` when no higher-confidence
     target source exists.
6. Built-in defaults:
   - only for optional values that have safe defaults.

If a git origin and repo-local contract both identify a repo and they disagree,
Gira should report a configuration conflict instead of silently choosing one.

## Config Merge Ownership

Target selection and config merging are related but separate. After the target
repo or workspace is known, Gira should merge settings by ownership.

Explicit CLI flags always win for the current invocation.

Global config owns personal operator preferences:

- default owner
- default workspace
- default inbox repo
- registered checkout paths
- repo aliases
- preferred agent labels
- default assignee
- output preferences
- cache and state path preferences

Global workspace entries own personal workspace grouping:

- workspace name
- workspace owner
- inbox repo
- execution repo list
- personal workspace-level defaults

Global repo entries own personal repo metadata:

- local checkout path
- aliases
- default agent or assignee for that operator
- pointer to a repo-local contract path

Repo-local contracts own shared repo policy:

- repo identity, which must match selected target when both are known
- label/status/type policy
- review and check policy
- ticket lifecycle policy
- issue and PR template policy
- canonical agent skill path

Local overrides own machine-specific values only:

- local checkout path adjustments
- local cache or state path overrides
- machine-specific tool paths

Use `.gira/config.local.yaml` for local overrides. It must be gitignored and
should not be used to change shared repo policy. If an override attempts to
change shared policy, Gira should warn or reject it once validation exists.

## Merge Precedence

For fields with the same ownership, later layers override earlier layers:

1. Built-in defaults
2. Global config
3. Global workspace entry
4. Global repo entry
5. Repo-local contract for shared policy fields
6. Local override for machine-specific fields
7. Explicit CLI flags

For fields with different ownership, ownership wins over raw layer order. For
example, a repo-local contract wins for shared label policy, while the global
repo entry wins for a local checkout path.

## Migration Stance

Existing `.gira/config.yaml` users should not be forced into a breaking change.
When a repo-local config exists, Gira should guide migration rather than
silently changing behavior.

Recommended migration:

1. Detect existing `.gira/config.yaml`.
2. Offer a dry-run migration plan with `gira repo migrate --path . --dry-run`.
3. Create or update a global repo registry entry with personal metadata.
4. Preserve `.gira/config.yaml` as the repo-local contract.
5. Store a `contract: .gira/config.yaml` reference in the global repo entry
   when appropriate.

Apply only after the plan is correct:

```bash
gira repo migrate --path . --apply
gira config repo --repo OWNER/REPO
```

Symlink migration should be explicit and advanced, not the default. Import plus
contract reference is the preferred compatibility path.

## Default Flip Strategy

Global registry support ships in stages:

1. Add paths, schemas, loaders, and diagnostics without changing defaults.
2. Add repo registration and workspace global initialization as opt-in flows.
3. Add migration guidance for existing repo-local configs.
4. Document global mode as the recommended personal/operator mode.
5. Flip default workspace config resolution to the global registry when a
   matching global workspace is available.

The default flip is compatibility-sensitive. The rollback and opt-out path is
to pass an explicit config path:

```bash
gira workspace status --config .gira/config.yaml
gira projects sync --config .gira/config.yaml --dry-run
```

Explicit `--config` flags preserve repo-local contract behavior. Without
`--config`, workspace commands prefer a matching global workspace and then fall
back to `.gira/config.yaml`.
