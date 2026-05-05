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

The install script upgrades or replaces only the local Go-built `gira` binary. It does not change repository files, labels, milestones, issues, or GitHub settings. Package-manager wrappers, when added, must remain distribution channels for the same Go-built release binary and are not alternate Gira runtimes.

## Migration Modes

Gira supports three metadata policy modes on `sync`:

- `adopt`: observe existing metadata and do not create/update labels, milestones, or bootstrap issues.
- `merge` (default): preserve user-owned metadata while creating/updating only Gira-owned desired state.
- `enforce`: reconcile all desired definitions, including updates where drift exists.

## Deterministic Dry-Run Workflow

Always run these in order:

```bash
gira sync --repo OWNER/REPO --dry-run --policy-mode adopt
gira sync --repo OWNER/REPO --dry-run --policy-mode merge
gira sync --repo OWNER/REPO --dry-run --policy-mode enforce
```

Why this order:
1. `adopt` gives a no-mutation baseline inventory.
2. `merge` shows minimal safe convergence.
3. `enforce` shows full convergence cost.

The plan output is deterministic for a fixed repository state (same plan counts and sorted item actions).

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
   - First: `gira sync --repo OWNER/REPO --policy-mode <mode>`
   - Optional second pass for bootstrap issue creation:
     `gira sync --repo OWNER/REPO --policy-mode <mode> --bootstrap-issues`
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
