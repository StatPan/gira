# Global Config Mode

Gira's global config is per operating-system user, similar to
`git config --global`. It is the default place for personal multi-repo
operation because GitHub issues, PRs, labels, milestones, and Projects remain
the execution source of truth.

Default locations:

```text
~/.config/gira        durable config and registry
~/.cache/gira         disposable cache
~/.local/state/gira   runtime state and logs
```

The durable registry separates personal operator state from shared repository
contracts:

```text
~/.config/gira/
  config.yaml
  repos/OWNER/REPO.yaml
  workspaces/NAME.yaml
```

## Personal Mode

Use global workspace and repo registry entries when you want one operator view
across many repositories without committing `.gira/config.yaml` everywhere.

Start with the setup flow when you want Gira to own personal config from the
global registry:

```bash
gira setup global \
  --repo OWNER/app \
  --path ~/workspace/app \
  --workspace personal \
  --inbox-repo OWNER/backlog \
  --mode global-only \
  --dry-run

gira setup global \
  --repo OWNER/app \
  --path ~/workspace/app \
  --workspace personal \
  --inbox-repo OWNER/backlog \
  --mode global-only \
  --apply
```

`setup global` writes the global config, workspace entry, and repo entry
together. It detects an existing repo-local `.gira/config.yaml`; `global-only`
does not reference it, while `--mode hybrid` keeps a `contract:
.gira/config.yaml` pointer for shared repo policy.

`--inbox-repo` is the backlog/intake repo for tickets that are not yet assigned
to an execution repo. Use a dedicated repo such as `OWNER/backlog` for
multi-repo workspaces. Reusing the execution repo as the inbox is acceptable
for a small single-repo setup, but it mixes untriaged intake with product
execution issues.

After the first repo is registered, sync the global workspace execution repo
list from a GitHub user or organization:

```bash
gira workspace repos sync --owner OWNER --workspace personal --dry-run
gira workspace repos sync --owner OWNER --workspace personal --apply
```

This is an explicit registry update, not background discovery. The inbox repo is
skipped because it is backlog/intake, not an execution repo. Use
`--include-archived` only when archived repos should stay in the global view.

For large global workspaces, use bounded and cached status reads:

```bash
gira workspace status --limit 10 --active-only
gira workspace status --repo OWNER/app
```

`workspace status` reports the GitHub API budget when available, bounds
concurrent repo fetches, and reuses recent per-repo status cache for five
minutes by default. Future GUI/background surfaces should refresh on a
multi-minute interval and reserve `--refresh` for explicit operator reads.

Use the lower-level primitives only when you need to compose the pieces
manually:

```bash
gira workspace init --scope global --name personal --inbox-repo OWNER/backlog --repo OWNER/app --dry-run
gira workspace init --scope global --name personal --inbox-repo OWNER/backlog --repo OWNER/app --apply
gira repo register OWNER/app --path ~/workspace/app --dry-run
gira repo register OWNER/app --path ~/workspace/app --apply
```

Inspect what Gira sees:

```bash
gira config global
gira config repo --repo OWNER/app
gira config doctor --repo OWNER/app
```

## Repo Contract Mode

Keep `.gira/config.yaml` in a repository when the repo itself should declare
shared Gira policy, such as labels, status conventions, review policy, ticket
lifecycle policy, or agent skill references.

Repo contract mode is optional. It is useful for teams and agent-ready repos,
but it should not be required for personal multi-repo operation.

Machine-specific overrides belong in `.gira/config.local.yaml`. That file must
be gitignored and should only hold local paths, cache/state overrides, or
machine-specific tool settings.

## Migration

Existing `.gira/config.yaml` users should migrate by adding a global repo
registry entry that references the repo-local contract. Do not move or delete
the repo-local file by default.

```bash
gira repo migrate --path . --dry-run
gira repo migrate --path . --apply
```

The migration stores `contract: .gira/config.yaml` in the global repo entry and
preserves `.gira/config.yaml` as the shared repo contract. Symlink migration is
an advanced explicit operation, not the default compatibility path.
