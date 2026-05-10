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

The first ten minutes after `gira init` should answer four questions: what exists, what Gira recommends, what is safe to apply, and what can an AI worker consume.

1. Detect existing repository state:

   ```bash
   gira init --repo OWNER/REPO --path . --dry-run
   gira adopt repo --repo OWNER/REPO --path . --dry-run
   ```

2. Apply the minimal adoption contract when the plan looks right:

   ```bash
   gira adopt repo --repo OWNER/REPO --path . --strategy merge --apply
   git status --short
   ```

3. Preview GitHub metadata sync (bootstrap issues are opt-in):

   ```bash
   gira ops sync --repo OWNER/REPO --dry-run
   ```

4. Apply labels and milestones after the plan looks right:

   ```bash
   gira ops sync --repo OWNER/REPO
   ```

   For Gira self-dogfood bootstrap issues, opt in explicitly:

   ```bash
   gira ops sync --repo StatPan/gira --dry-run --bootstrap-issues
   gira ops sync --repo StatPan/gira --bootstrap-issues
   ```

5. Inspect the project state:

   ```bash
   # from the target checkout, or any directory with .gira/config.yaml repo set
   gira status
   gira status --json
   ```

6. Verify go/no-go readiness before first daily use:

   ```bash
   # from the target checkout, or any directory with .gira/config.yaml repo set
   gira ops onboard verify --stage init --json
   gira ops onboard verify --stage steady-state --json
   ```

After repo adoption, the operator should see a short local contract summary and use `gira ops sync --repo OWNER/REPO --dry-run` as the next-step hint. After sync, the operator should use `gira status` from a checkout with a GitHub `origin` remote, or from a directory with `.gira/config.yaml` containing `repo: OWNER/REPO`, to pick the next ready ticket. `gira ops onboard verify` should provide the explicit go/no-go verdict and remediation checklist before the team treats the repo as daily-operable. Pass `--repo OWNER/REPO` when scripting or operating outside the target checkout/config directory.

## Command Taxonomy

`adopt repo` is the default existing-repository gateway. It detects existing local files and GitHub state, then applies only the minimal Gira contract selected by the operator. Existing `AGENTS.md`, PR templates, and issue templates are user-owned; Gira may insert or update only a managed block.

`bootstrap` prepares local project files from the default template for fresh repositories. It owns `.github` templates, project docs, task list seeds, and local worker instructions. It should not create labels, milestones, issues, branches outside the target local repo, or remote PRs.

`sync` reconciles GitHub execution metadata through `gh`. It owns Gira-managed labels, milestones, and (when `--bootstrap-issues` is provided) bootstrap issues. It may create or update known Gira metadata, but it must not delete labels, close issues, delete milestones, or change broad repository settings in the MVP.

`status` reads GitHub state and summarizes it. It owns compact human reporting and stable JSON for automation. It must remain read-only.

`onboard verify` is read-only and composes the other recovery steps into a staged go/no-go verdict. It owns prerequisite checks, committed bootstrap artifact checks, metadata convergence checks, and sample daily-run validation.

## Adding Public Commands

Public commands must be added to the command metadata registry before they are
treated as complete. Update `internal/gira/command_registry.go` with the
command path, summary, usage, flags, examples, release version, docs surfaces,
and guide topics.

The docs-site command reference and registry-backed guide sections are rendered
from that registry. If command metadata changes, refresh
`docs-site/command-reference.md` so `go test ./internal/gira` can verify it is
still in sync.

For high-value user-facing commands, also update the relevant user journey page
instead of relying only on the reference page:

- first-run or global workflow: `README.md`, `docs-site/quickstart.md`,
  `docs-site/global-config.md`, and `docs/workspace.md`;
- ticket lifecycle: `README.md`, `docs-site/ticket-workflow.md`, and
  `docs/dogfood.md`;
- agent/operator behavior: `docs/skills/gira-agent-operator.md` and
  `docs-site/agent-operator-skill.md`.

The registry is not a replacement for narrative docs. It is the drift-prevention
source for command facts and examples. Agent adapter snippets must also be
generated through the shared guidance renderer and written only inside managed
blocks such as `<!-- gira:start -->` and `<!-- gira:end -->`. Canonical skill
documents may use their own managed blocks, such as
`<!-- gira:agent-skill:start -->` and `<!-- gira:agent-skill:end -->`, for
registry-backed command guidance while keeping surrounding policy text
human-owned.

This taxonomy keeps a clean recovery model: rerun `bootstrap` for local files, rerun `sync` for GitHub metadata, rerun `status` to decide what to do next, and rerun `onboard verify` to confirm the repo is truly ready for daily operation.

## CLI Development Path

The Go-built `gira` binary is the sole product implementation. Development should continue in narrow, testable slices against the Go CLI.

Current scope:

```bash
go run ./cmd/gira --help
go run ./cmd/gira ops bootstrap --repo OWNER/REPO --template default --dry-run
go run ./cmd/gira ops bootstrap --repo OWNER/REPO --path /path/to/repo
go run ./cmd/gira ops sync --repo OWNER/REPO --dry-run
go run ./cmd/gira ops sync --repo OWNER/REPO
go run ./cmd/gira status --repo OWNER/REPO
go run ./cmd/gira status --repo OWNER/REPO --json
go run ./cmd/gira ops onboard verify --repo OWNER/REPO --stage init --json
go run ./cmd/gira ops onboard verify --repo OWNER/REPO --stage steady-state --json
```

Release users should install the Go-built binary through a release channel. These installs do not require Go:

```bash
curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh | sh
npm install -g @statpan/gira
bun install -g @statpan/gira
uv tool install gira-cli
pipx install gira-cli
python -m pip install --user gira-cli
brew tap StatPan/tap
brew install gira
```

The CLI can also be built from source for development:

```bash
go install github.com/StatPan/gira/cmd/gira@latest
```

Canonical daily operator path (fresh shell, outside source checkout):

```bash
export PATH="${HOME}/.local/bin:$PATH"
gira --help
gira version
gira ops doctor --repo OWNER/REPO
gira ops bootstrap --repo OWNER/REPO --template default --dry-run
gira ops sync --repo OWNER/REPO --dry-run
gira status --repo OWNER/REPO --json
```

For source builds, the module is `github.com/StatPan/gira` and the binary package is under `cmd/gira`, so the install path includes `/cmd/gira`. Private repository source builds need Go private module access, such as `GOPRIVATE=github.com/StatPan/gira` plus normal GitHub authentication. The bootstrap path embeds the default template so output and local installs are independent of the caller's working directory. `sync` shells out through `gh` to create or update only Gira-managed labels, milestones, and bootstrap issues. `status` is read-only and shells out through `gh api` with stable JSON for worker automation.

Package-manager wrappers such as npm, bun, uv, pip, pipx, or Homebrew are distribution channels for the Go-built `gira` release binary. They should not introduce a second product runtime. apt/deb packaging remains a future channel.

Tagged Go releases are built by `.github/workflows/release.yml`. Maintainers publish one by tagging `main` with a `v*` tag and pushing the tag; the workflow checks the installer syntax, runs Go tests plus npm and PyPI wrapper tests, builds Linux/macOS/Windows archives with version metadata, generates `checksums.txt`, verifies it, attaches the assets to the GitHub release, and publishes configured npm/PyPI/Homebrew channels.

## Output Conventions

Human output should be short, sectioned, and deterministic enough to compare between runs. Counts should come first; details should only list changed or attention-worthy items.
When a safe continuation is known, human output should end with one concise `next step:` line. JSON output must not include prose-only hints.

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
- Conflict output should keep generated non-conflicting files in the worktree and print the continuation flow: create a ticket with `gira ticket new --apply --start`, resolve the listed conflicts on that ticket branch, then open the PR with `gira ticket pr --apply --draft`.
- If `--overwrite` is used, replace only conflicting template-owned files.
- If branch creation is unwanted, `--no-branch` keeps the current branch.
- Rerunning after a successful install should report zero created files and leave content unchanged.

Sync recovery:

- Build the full plan before applying changes.
- Create or update known labels and milestones by name/title.
- Deduplicate bootstrap issues by title plus `gira:bootstrap`.
- Never delete extra labels, milestones, or issues in MVP.
- If `gh` fails midway, fix authentication, permissions, or the remote conflict, then rerun `gira ops sync --repo OWNER/REPO --dry-run` before applying again.

Status recovery:

- `status` is read-only, so failure usually means `gh` auth, repository access, or API availability.
- Workers should treat invalid or absent JSON as a command failure, not as an empty project.

## Issue To PR Loop

1. Start from a ready ticket with `gira ticket start --repo OWNER/REPO --ticket N --apply`.
2. Create or reuse the issue branch; `ticket start --apply` moves the issue to `status:in-progress`.
3. Keep the change bounded to the issue body and acceptance criteria.
4. Run the relevant local verification.
5. Open or validate the linked PR with `gira ticket pr --repo OWNER/REPO --ticket N --apply [--draft]`.
6. Include `Closes #N`, `Fixes #N`, or `Resolves #N`; `ticket pr` creates PRs with `Closes #N`.
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
- Portfolio Ticket: goal, scope, routing, target repos, acceptance criteria, and child issue links for a top-level ticket before execution lowering.

The default issue templates use labels that `gira ops sync` should manage: `type:epic`, `type:story`, `type:task`, `type:spike`, and `type:bug`. The portfolio template intentionally uses `type:epic` so it can work with the existing default label taxonomy while carrying the portfolio-specific routing fields in the issue body.

The PR template should stay short and enforce the essentials:

- Summary of the change.
- Test plan with commands or manual checks.
- Closing keyword for the source issue.

## Dogfood Notes 2026-04-26

- Local-only bootstrap is safe to dogfood in a temporary git repo because non-dry-run requires `--path` and fails before writing outside a git repo.
- The default bootstrap branch name `chore/gira-bootstrap` makes first-run changes easy to inspect before opening a PR.
- `gira ops sync --dry-run` is readable, but it currently lacks `--json`; automation should use `status --json` until a sync JSON contract is added.
- The default issue forms previously referenced `type:bug`, `type:story`, and `type:spike` labels that were not in the sync label set; those labels are now part of desired metadata.
- A real issue-to-branch-to-PR loop was exercised by implementing this spike from Issue #7 on `docs/issue-7-dx-workflow` and opening the resulting PR.
