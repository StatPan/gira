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

Local overrides must be gitignored and should not be used to change shared
repo policy. If an override attempts to change shared policy, Gira should warn
or reject it once validation exists.

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
2. Offer a dry-run migration plan.
3. Create or update a global repo registry entry with personal metadata.
4. Preserve `.gira/config.yaml` as the repo-local contract.
5. Store a `contract: .gira/config.yaml` reference in the global repo entry
   when appropriate.

Symlink migration should be explicit and advanced, not the default. Import plus
contract reference is the preferred compatibility path.

## Default Flip Strategy

Global registry support should ship in stages:

1. Add paths, schemas, loaders, and diagnostics without changing defaults.
2. Add repo registration and workspace global initialization as opt-in flows.
3. Add migration guidance for existing repo-local configs.
4. Document global mode as the recommended personal/operator mode.
5. Later, flip default config resolution to global registry when available.

The default flip is a separate compatibility-sensitive change. It should happen
only after `gira config doctor` can explain which source won and after docs
provide an opt-out or explicit legacy path such as `--config .gira/config.yaml`.
