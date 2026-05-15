# Business-Group Multi-Repo Workflows

This note defines the intended Gira model for a business work stream that spans more than one repository. It is a design contract, not a new implementation surface yet.

## Model

A business group is a named workspace lane over an explicit set of repositories. It does not replace repo-local execution. It gives operators one place to see intake, routed work, cross-repo parent tickets, and adoption state while preserving GitHub issues and PRs as the source of truth.

```yaml
workspace:
  name: platform
  owner: OWNER
  inbox_repo: OWNER/backlog
  repos:
    - OWNER/api
    - OWNER/web
    - OWNER/infra
  groups:
    - name: billing
      title: Billing Platform
      repos:
        - OWNER/api
        - OWNER/web
      default_area: area:billing
      project:
        owner: OWNER
        title: Billing Roadmap
```

The first implementation should treat `workspace.groups` as optional. Existing workspace commands continue to work when it is absent. A group may only reference repositories already listed in `workspace.repos`; Gira should not use a group as an implicit org scan.

## Ownership

| Artifact | Owner | Rule |
| --- | --- | --- |
| Workspace config | Operator or repo contract | Defines inbox, execution repo allowlist, and optional business groups. |
| Inbox issue | Business intake | Holds product intent until routing is clear. |
| Execution issue | Target repo | Owns implementation scope, branch, PR, checks, and finish evidence. |
| Parent ticket link | Gira convention | Connects cross-repo work without moving source of truth out of GitHub. |
| GitHub Project | Visibility surface | Mirrors issue state; it does not own execution lifecycle. |

Adoption and bootstrap must not rewrite user-owned repo metadata. For an existing repository, group adoption should record only the minimal Gira contract needed for deterministic routing and status, then leave labels, templates, branch protection, and project views to explicit sync commands.

## Target Selection

Every mutating ticket command in a multi-repo workspace must resolve an execution repo before it mutates GitHub or local Git state.

Resolution order:

1. Explicit `--repo OWNER/REPO`.
2. Current checkout GitHub `origin`, if it matches the workspace allowlist.
3. Registered repo alias from the global repo registry.
4. A routed execution issue linked from the inbox or parent ticket.

If more than one repo is possible, Gira should block and ask for `--repo`. It should not infer from group name alone. A group narrows the visible set of repos; it does not decide the execution target for a ticket lifecycle command.

Dry-run output should print the resolved repo, the evidence source used for resolution, and the mutation boundary, for example:

```text
target: OWNER/api source=--repo
local_git: no mutation
github: issue create planned in OWNER/api
```

## Dirty-Worktree Safety

Repo-local commands that create branches, push branches, or open PRs must operate from the checkout that matches the resolved repo. If the current checkout is dirty, the command may still report status but must not switch branches or create a PR unless the operation is explicitly designed to work with dirty state.

Rules:

- `workspace status`, `workspace backlog`, and read-only planning may run from any checkout.
- `ticket start` and `ticket pr` must verify that the checkout origin matches the target repo.
- `ticket start` must not switch branches when unrelated worktree changes are present unless a future explicit override exists.
- `ticket pr` must validate that the current branch is a ticket branch for the target issue and target repo.
- Errors should report the repo mismatch or dirty state without dumping remote URLs that may contain credentials.

## Raw GitHub PR Attach And Backfill

Humans and external tools may create PRs without Gira. A business-group workflow needs an attach/backfill path instead of forcing all work through `gira ticket pr`.

Planned attach behavior:

```bash
gira ticket attach-pr --repo OWNER/api --ticket 123 --pr 456 --dry-run
gira ticket attach-pr --repo OWNER/api --ticket 123 --pr 456 --apply
```

Dry-run should verify:

- The issue and PR are in the same target repo.
- The PR base branch is the configured default branch or an allowed release branch.
- The PR head branch is either a valid ticket branch or is explicitly accepted as an external branch.
- The PR body, issue comments, or closing references can be updated without overwriting user text.
- Required labels and status transitions are known.

Apply should only add missing linkage evidence and status metadata. It must not force-push, rewrite PR bodies wholesale, retarget branches, close issues, or change review state. If the PR already merged, apply may backfill completion evidence and normalize issue status only when linked PR checks and merge state are visible.

## Adoption Flow

Business-group adoption should be dry-run-first:

```bash
gira workspace group adopt --workspace platform --group billing --dry-run
gira workspace group adopt --workspace platform --group billing --apply
```

The command should:

- Validate that group repos are listed in `workspace.repos`.
- Report which repos already have a repo-local Gira contract.
- Recommend `gira adopt repo --repo OWNER/REPO --path PATH --strategy merge --dry-run` for each checkout that needs local metadata.
- Preserve existing labels, milestones, issue templates, PR templates, and `AGENTS.md` content unless a later explicit sync command is run.
- Never adopt an org-wide repo set implicitly.

## Follow-Up Slices

1. Add parser and validation support for optional `workspace.groups`.
2. Add `workspace group status --group NAME` as a read-only rollup.
3. Add explicit repo-resolution diagnostics to mutating ticket commands.
4. Add dirty-worktree guardrails for `ticket start` and `ticket pr`.
5. Add `ticket attach-pr` for raw GitHub PR backfill.
6. Add `workspace group adopt --dry-run` recommendations for existing repos.

The implementation order should stay read-only first, then deterministic attach/backfill, then adoption helpers. Gira should not introduce background automation or org-wide discovery for this workflow.
