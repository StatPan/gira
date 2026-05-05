# Gira Developer Experience

Gira should make a GitHub repository feel like a small, explicit project operating system. The CLI owns setup and inspection. GitHub owns execution state. Humans and AI workers should see the same next step, with enough structure to continue safely after an interruption.

## Principles

- Keep the happy path copy-paste obvious.
- Prefer dry-run first, then apply with a clear diff-like summary.
- Make every mutating command idempotent so reruns are normal recovery, not a special mode.
- Preserve user-owned repository content and metadata unless the operator explicitly chooses overwrite or a future destructive mode.
- Keep human text concise, but expose stable JSON for Hermes and other workers where automation needs it.
- Treat GitHub Issues as task packets and PRs as change units.

## First 10 Minutes

The first ten minutes after `gira bootstrap` should answer four questions: what changed, what is safe to rerun, what should I do next, and what can an AI worker consume.

1. Preview the repo template:

   ```bash
   gira bootstrap --repo OWNER/REPO --template default --dry-run
   ```

2. Install project files into a local git repo on a bootstrap branch:

   ```bash
   gira bootstrap --repo OWNER/REPO --template default --path .
   git status --short
   ```

3. Preview GitHub metadata sync (bootstrap issues are opt-in):

   ```bash
   gira sync --repo OWNER/REPO --dry-run
   ```

4. Apply labels and milestones after the plan looks right:

   ```bash
   gira sync --repo OWNER/REPO
   ```

   For Gira self-dogfood bootstrap issues, opt in explicitly:

   ```bash
   gira sync --repo StatPan/gira --dry-run --bootstrap-issues
   gira sync --repo StatPan/gira --bootstrap-issues
   ```

5. Inspect the project state:

   ```bash
   gira status --repo OWNER/REPO
   gira status --repo OWNER/REPO --json
   ```

6. Verify go/no-go readiness before first daily use:

   ```bash
   gira onboard verify --repo OWNER/REPO --stage init --json
   gira onboard verify --repo OWNER/REPO --stage steady-state --json
   ```

After bootstrap, the operator should see a short install summary and use `gira sync --dry-run` as the next-step hint. After sync, the operator should use `gira status` to pick the next ready issue. `gira onboard verify` should provide the explicit go/no-go verdict and remediation checklist before the team treats the repo as daily-operable.

## Command Taxonomy

`bootstrap` prepares local project files from the default template. It owns `.github` templates, project docs, task list seeds, and local worker instructions. It should not create labels, milestones, issues, branches outside the target local repo, or remote PRs.

`sync` reconciles GitHub execution metadata through `gh`. It owns Gira-managed labels, milestones, and (when `--bootstrap-issues` is provided) bootstrap issues. It may create or update known Gira metadata, but it must not delete labels, close issues, delete milestones, or change broad repository settings in the MVP.

`status` reads GitHub state and summarizes it. It owns compact human reporting and stable JSON for automation. It must remain read-only.

`onboard verify` is read-only and composes the other recovery steps into a staged go/no-go verdict. It owns prerequisite checks, committed bootstrap artifact checks, metadata convergence checks, and sample daily-run validation.

This taxonomy keeps a clean recovery model: rerun `bootstrap` for local files, rerun `sync` for GitHub metadata, rerun `status` to decide what to do next, and rerun `onboard verify` to confirm the repo is truly ready for daily operation.

## CLI Development Path

The Go-built `gira` binary is the sole product implementation. Development should continue in narrow, testable slices against the Go CLI.

Current scope:

```bash
go run ./cmd/gira --help
go run ./cmd/gira bootstrap --repo OWNER/REPO --template default --dry-run
go run ./cmd/gira bootstrap --repo OWNER/REPO --path /path/to/repo
go run ./cmd/gira sync --repo OWNER/REPO --dry-run
go run ./cmd/gira sync --repo OWNER/REPO
go run ./cmd/gira status --repo OWNER/REPO
go run ./cmd/gira status --repo OWNER/REPO --json
go run ./cmd/gira onboard verify --repo OWNER/REPO --stage init --json
go run ./cmd/gira onboard verify --repo OWNER/REPO --stage steady-state --json
```

The CLI can be installed for daily use from source:

```bash
go install github.com/StatPan/gira/cmd/gira@latest
```

Canonical daily operator path (fresh shell, outside source checkout):

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
gira --help
gira bootstrap --repo OWNER/REPO --template default --dry-run
gira sync --repo OWNER/REPO --dry-run
gira status --repo OWNER/REPO --json
```

The module is `github.com/StatPan/gira` and the binary package is under `cmd/gira`, so the install path includes `/cmd/gira`. Private repository installs need Go private module access, such as `GOPRIVATE=github.com/StatPan/gira` plus normal GitHub authentication. The bootstrap path embeds the default template so output and local installs are independent of the caller's working directory. `sync` shells out through `gh` to create or update only Gira-managed labels, milestones, and bootstrap issues. `status` is read-only and shells out through `gh api` with stable JSON for worker automation.

Package-manager wrappers such as `uv`, npm, bun, or Homebrew may be used as distribution channels when they install or invoke the Go-built `gira` binary. They should not introduce a second product runtime.

Tagged Go releases are built by `.github/workflows/release.yml`. Maintainers publish one by tagging `main` with a `v*` tag and pushing the tag; the workflow runs Go tests, builds Linux/macOS/Windows archives, and attaches them to the GitHub release.

## Output Conventions

Human output should be short, sectioned, and deterministic enough to compare between runs. Counts should come first; details should only list changed or attention-worthy items.

Dry-run output uses future-tense language:

```text
sync plan:
labels:           3 would create, 0 would update, 8 skip
milestones:       0 would create, 0 would update, 3 skip
bootstrap issues: 0 would create, 5 skip
  create label: type:bug
```

Apply output uses direct action language and ends with completion only after all requested operations succeed:

```text
sync plan:
labels:           3 create, 0 update, 8 skip
milestones:       0 create, 0 update, 3 skip
bootstrap issues: 0 create, 5 skip
  create label: type:bug
sync complete
```

Commands should exit non-zero when conflicts or failed external calls leave work incomplete. Partial success should be visible in the preceding summary or in `gh` error context, and recovery should be a rerun after fixing the cause.

## JSON For Workers

Use `--json` where a worker needs stable machine-readable state. In MVP, `status --json` is the primary automation contract. It should remain valid JSON on stdout, with no extra prose, and include:

- `repo`
- `fetched_at`
- `stale_days`
- `counts`
- `milestones`
- `issues.open`
- `issues.stale_open`
- `issues.blocked_open`

Future JSON modes for mutating commands should follow the same rule: data only on stdout, diagnostics on stderr, and stable keys tested in the suite. Worker consumers should rely on labels, issue numbers, milestone names, URLs, and updated timestamps rather than formatted human lines.

## Failure And Recovery

Bootstrap recovery:

- If a target path is not a git repo, fail before writing files.
- If a file already exists with different content, preserve it by default, report a conflict, and exit non-zero.
- If `--overwrite` is used, replace only conflicting template-owned files.
- If branch creation is unwanted, `--no-branch` keeps the current branch.
- Rerunning after a successful install should report zero created files and leave content unchanged.

Sync recovery:

- Build the full plan before applying changes.
- Create or update known labels and milestones by name/title.
- Deduplicate bootstrap issues by title plus `gira:bootstrap`.
- Never delete extra labels, milestones, or issues in MVP.
- If `gh` fails midway, fix authentication, permissions, or the remote conflict, then rerun `gira sync --repo OWNER/REPO --dry-run` before applying again.

Status recovery:

- `status` is read-only, so failure usually means `gh` auth, repository access, or API availability.
- Workers should treat invalid or absent JSON as a command failure, not as an empty project.

## Issue To PR Loop

1. Start from a ready GitHub issue with `gira work start --repo OWNER/REPO --issue N --apply`.
2. Create or reuse the issue branch; `work start --apply` moves the issue to `status:in-progress`.
3. Keep the change bounded to the issue body and acceptance criteria.
4. Run the relevant local verification.
5. Open or validate the linked PR with `gira work pr --repo OWNER/REPO --issue N --apply [--draft]`.
6. Include `Closes #N`, `Fixes #N`, or `Resolves #N`; `work pr` creates PRs with `Closes #N`.
7. Add a verification comment or test plan note when a reviewer or worker needs exact reproduction commands.
8. Merge only after review and passing checks.

For Hermes and AI workers, the issue body should be self-contained enough to execute: goal, context, acceptance criteria, out-of-scope boundaries, verification commands, and any safety constraints. PRs should make the result auditable: summary, test plan, linked issue, and caveats.

## Template Expectations

Issue templates should serve both humans and workers:

- Epic: goal and scope for a milestone-sized outcome.
- Story: user story and acceptance criteria.
- Task: goal and checklist-style acceptance criteria.
- Spike: question and expected output.
- Bug: actual behavior and expected behavior.

The default issue templates use labels that `gira sync` should manage: `type:epic`, `type:story`, `type:task`, `type:spike`, and `type:bug`.

The PR template should stay short and enforce the essentials:

- Summary of the change.
- Test plan with commands or manual checks.
- Closing keyword for the source issue.

## Dogfood Notes 2026-04-26

- Local-only bootstrap is safe to dogfood in a temporary git repo because non-dry-run requires `--path` and fails before writing outside a git repo.
- The default bootstrap branch name `chore/gira-bootstrap` makes first-run changes easy to inspect before opening a PR.
- `sync --dry-run` is readable, but it currently lacks `--json`; automation should use `status --json` until a sync JSON contract is added.
- The default issue forms previously referenced `type:bug`, `type:story`, and `type:spike` labels that were not in the sync label set; those labels are now part of desired metadata.
- A real issue-to-branch-to-PR loop was exercised by implementing this spike from Issue #7 on `docs/issue-7-dx-workflow` and opening the resulting PR.
