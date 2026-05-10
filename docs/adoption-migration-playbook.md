# Adoption & Migration Playbook (pre-configured repositories)

This guide defines a safe, deterministic migration path for adopting Gira on repositories that already have labels, milestones, and active workflows.

## Goals

- Discover existing repository metadata before changing anything.
- Produce deterministic dry-run output so operators can review and diff plans.
- Classify conflicts and recommend a policy mode: `adopt`, `merge`, or `enforce`.
- Provide rollback guidance for each mutation class.

## Preconditions

- Install `gira` from the official release path unless you are intentionally testing local source:
  `curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh | sh`
- `gh` authenticated with repository admin/maintainer access.
- Working tree clean, and local `main` up to date.
- Run from the repository root.
- `gira doctor --repo OWNER/REPO` passes or any failures are understood before applying migration changes.

The install script upgrades or replaces only the local Go-built `gira` binary. It does not change repository files, labels, milestones, issues, or GitHub settings. Package-manager wrappers remain distribution channels for the same Go-built release binary and are not alternate Gira runtimes.

## Migration Modes

Gira supports three metadata policy modes on `gira ops sync`:

- `adopt`: observe existing metadata and do not create/update labels, milestones, or bootstrap issues.
- `merge` (default): preserve user-owned metadata while creating/updating only Gira-owned desired state.
- `enforce`: reconcile all desired definitions, including updates where drift exists.

## Deterministic Dry-Run Workflow

Always run these in order:

```bash
gira adopt repo --repo OWNER/REPO --path . --dry-run
gira ops sync --repo OWNER/REPO --dry-run --policy-mode adopt
gira ops sync --repo OWNER/REPO --dry-run --policy-mode merge
gira ops sync --repo OWNER/REPO --dry-run --policy-mode enforce
```

Why this order:
1. `adopt repo` detects existing files, issues, Projects, labels, and milestones before any scaffold is proposed.
2. `adopt` gives a no-mutation metadata baseline inventory.
3. `merge` shows minimal safe convergence.
4. `enforce` shows full convergence cost.

Use repo adoption to choose how much Gira should write locally:

```bash
gira adopt repo --repo OWNER/REPO --path . --strategy observe --dry-run
gira adopt repo --repo OWNER/REPO --path . --strategy merge --dry-run
gira adopt repo --repo OWNER/REPO --path . --strategy merge --apply
```

`merge` preserves existing `AGENTS.md`, PR templates, and issue templates. If `AGENTS.md` exists, Gira inserts or updates only the `<!-- gira:start -->` managed block. Bootstrap sample issues are not part of normal adoption.

The plan output is deterministic for a fixed repository state (same plan counts and sorted item actions).

## Repo-Local Contract to Global Registry

For personal global-first operation, use setup instead of hand-editing global
YAML files:

```bash
gira setup global --repo OWNER/REPO --path . --workspace personal --inbox-repo OWNER/REPO --mode global-only --dry-run
gira setup global --repo OWNER/REPO --path . --workspace personal --inbox-repo OWNER/REPO --mode global-only --apply
```

`global-only` detects an existing `.gira/config.yaml` but does not reference it
from the global repo entry. This is the right mode when the current OS-user's
global registry should be the operating source.

Existing repositories that already have `.gira/config.yaml` and want to keep it
as shared team policy should migrate by adding a global repo registry entry, not
by moving or replacing the repo-local file. The repo-local file remains the
optional shared contract, while the global registry stores personal operator
metadata such as checkout path and workspace association.

Plan the migration first:

```bash
gira repo migrate --path . --dry-run
```

Apply only after the plan is correct:

```bash
gira repo migrate --path . --apply
gira config repo --repo OWNER/REPO
```

The migration writes a global repo entry such as
`~/.config/gira/repos/OWNER/REPO.yaml` and records
`contract: .gira/config.yaml` when the repo-local contract exists. It preserves
`.gira/config.yaml`; deleting or rewriting that file is not part of the normal
migration.

The equivalent setup mode is:

```bash
gira setup global --repo OWNER/REPO --path . --workspace personal --mode hybrid --dry-run
gira setup global --repo OWNER/REPO --path . --workspace personal --mode hybrid --apply
```

If the target repo already has a different global registry entry, use
`--overwrite` only after reviewing the diff. Symlink migration is an advanced
explicit operation and is not the default compatibility path, because symlinks
can hide ownership and portability differences across machines.

Adopt an existing profile or org GitHub Project only after the local workspace config exists:

```bash
gira workspace project adopt --owner OWNER --title "Existing Board" --config .gira/config.yaml --dry-run
gira workspace project adopt --owner OWNER --title "Existing Board" --config .gira/config.yaml --apply
gira projects sync --config .gira/config.yaml --dry-run
```

Use `--number N` instead of `--title TITLE` when multiple Projects share a title. This command registers an existing Project in `workspace.project`; it does not create Projects and does not replace a different existing `workspace.project`. Repository issues remain the execution source of truth, and `projects sync` mirrors them into the selected Project for board and roadmap visibility.

Keep metadata sync separate from existing issue adoption:

```bash
gira adopt issues --repo OWNER/REPO --dry-run
gira adopt issues --repo OWNER/REPO --issue 1 --issue 2 --milestone MVP --label type:task --label status:ready --dry-run
gira adopt issues --repo OWNER/REPO --issue 1 --issue 2 --milestone MVP --label type:task --label status:ready --apply
```

The first command lists unmapped existing issues: missing milestone, missing `type:*`, or missing `status:*`. The apply command only updates explicitly selected issues.

## Conflict Categories and Recommended Mode

Use this matrix for decisioning:

| Conflict category | Example | Recommended mode |
| --- | --- | --- |
| Existing org/repo taxonomy should be preserved | team labels already in active use | `adopt` or `merge` |
| Partial overlap with Gira desired metadata | some labels/milestones match, others missing | `merge` |
| Explicit standardization required | repo must match canonical Gira definitions | `enforce` |
| Historical bootstrap artifacts only | old milestone/issue names differ but no process dependency | `merge` then targeted cleanup |
| Uncertain ownership of metadata | cannot determine whether metadata is policy-owned | start `adopt`, escalate later |

## Safe Cutover Procedure

1. Run all three dry-runs and capture output in CI or artifacts.
2. Choose mode using the conflict matrix.
3. Apply in smallest safe scope:
   - First: `gira ops sync --repo OWNER/REPO --policy-mode <mode>`
   - Optional second pass for bootstrap issue creation:
     `gira ops sync --repo OWNER/REPO --policy-mode <mode> --bootstrap-issues`
4. Re-run dry-run in same mode and verify plan converges to mostly `skip`.

## Rollback & Recovery

### Labels
- Recovery mechanism: `gh label edit`, `gh label delete`, `gh label create`.
- Rollback approach:
  - Restore previous name/color/description from pre-migration inventory.
  - Recreate deleted labels if referenced by automations.

### Milestones
- Recovery mechanism: `gh api repos/OWNER/REPO/milestones/<number> -X PATCH ...`.
- Rollback approach:
  - Restore title/description/due date to pre-migration values.
  - Reopen/close state as needed to match prior workflow.

### Bootstrap Issues
- Recovery mechanism: close/reopen/edit issue metadata.
- Rollback approach:
  - If created unintentionally, close with note and remove workflow labels.
  - If updated unintentionally, restore labels/milestone assignment from inventory.

## Operational Notes

- Prefer `merge` for first-time adoption in active repositories.
- Use `enforce` only after stakeholders agree on canonical metadata ownership.
- Keep dry-run logs for auditability and post-change review.
