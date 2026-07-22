# Gira

Gira helps humans and coding agents finish GitHub issues safely.

It is a GitHub-native control plane for AI-assisted software work: repository
evidence stays in GitHub, while Gira makes the next safe workflow command
explicit.

It turns the GitHub issue-to-PR loop into a predictable workflow that humans and
coding agents can both follow:

```text
issue -> branch -> PR -> checks -> evidence -> finish
```

AI coding agents can generate code and open pull requests, but real engineering
work is not finished until the repository state converges: the PR is linked,
checks are green, review blockers are clear, the issue is closed, active status
labels are normalized, and the next command is obvious. Gira makes that finish
step explicit.

The product promise is measurable. Gira keeps the final state GitHub-native,
then reduces the workflow labor needed to reach that same state. Gira does not
publish reduction percentages until an agent replay or instrumented benchmark
has recorded the exact fixture, commands, labels, PR actions, provider calls,
and post-apply verification. Until then, workflow scores are internal product
diagnostics, not public proof. The KPI model is documented in
[docs/workflow-efficiency-kpis.md](docs/workflow-efficiency-kpis.md).

Use Gira when you want GitHub to remain the source of truth, while still having
a Jira-like lifecycle that coding agents can operate safely:

- Issues are executable task packets.
- Branches prove work has started.
- PRs are change units.
- Checks and reviews are validation evidence.
- Milestones group sprint/release boundaries.
- `ticket finish` merges only when policy allows, then converges issue state.
- `doctor` detects repo, workflow, label, and closed-issue status drift.

The 2.x control-plane contract extends that loop with stable surfaces for
long-running human and agent work:

- `goal status`, `goal report`, `goal next`, `goal plan --dry-run|--apply`,
  and `goal finish` model a larger objective as an evidence-backed child-ticket
  graph with explicit human-review stop conditions and a visible JSON or HTML
  report.
- Ticket readiness, PR readiness, review packets, finish readiness, finish
  receipts, and drift audits make completion reviewable without trusting an
  agent transcript.
- `workspace status --json` includes `workspace-queues/v1` so operators can see
  agent-ready, review-needed, finish-ready, blocked, failed-check, and
  human-decision queues across repos.
- `queue list`, `queue next`, `queue handoff`, and `queue take` expose the
  Agent Handoff Queue for LLM task selection, handoff packet assembly, and safe
  ticket start without launching a worker by default.
- `stats pulse` reports recent evidence-backed workflow movement for a repo
  without turning operator activity into a people or agent ranking.
- Adapter-facing command metadata and approval evidence, including
  `gira-approval-plan/v1`, let durable agent runtimes treat Gira dry-runs as
  auditable plans before any matching `--apply`.
- `gira completion bash|zsh|fish` prints shell completion scripts for common
  commands and shared flags, plus cache-first dynamic candidates for repos,
  ticket numbers, labels, and milestones.

The 2.x release line is the CLI-first control-plane contract. It is not a
hosted dashboard, UI/TUI launch, provider expansion, or Gira-native planning
database. The release readiness boundary is documented in
[docs/v2-release-readiness.md](docs/v2-release-readiness.md).

The core safety model is Terraform-like:

- `--dry-run` previews GitHub or repository mutations.
- `--apply` executes only after the plan is understood.

Documentation: <https://gira.statpan.com>

Docs source lives in `docs-site/` and is built with VitePress. The docs toolchain is separate from the product runtime; the shipped product remains the Go-built `gira` binary.

The target branch policy contract is documented in [docs/branch-policy.md](docs/branch-policy.md). It defines the planned direction for resolving a ticket base branch once, recording it as lifecycle state, preserving it through PR creation and review, and avoiding hidden local checkout mutation during finish.

Gira is not a Jira clone or a separate planning database. It is a workflow
control layer over GitHub Issues, branches, PRs, checks, labels, and milestones.
Repo-linked Projects are visibility surfaces; Project-only items stay intake or
planning until routed to a repository issue.

Gira PM mode turns raw intent into durable, task-local PM state for coding
workers. It does not use `needs human` as a terminal state; it decomposes
missing judgment into context retrieval, decision policy derivation, risk
reduction, verification, scope splitting, or follow-up task packets.
The canonical human/AI PM role, authority boundary, and shared operating loop
are defined in
[docs/pm-operating-policy.md](docs/pm-operating-policy.md); MCP and other
adapters expose that protocol but do not make a connected model a PM by
themselves.

```bash
gira pm compile --repo OWNER/REPO --goal 123 --from-file request.md
gira pm context --repo OWNER/REPO --ticket 123
gira pm spec --profile delivery --context-ref issue:OWNER/REPO#100 --repo OWNER/REPO --from-file request.md > pm-task.md
gira ticket new --repo OWNER/REPO --title "TITLE" --body-file pm-task.md --type task --dry-run
gira pm qa --repo OWNER/REPO --ticket 123 --diff-summary
```

Jira-primary provider mode is optional. In that mode Jira owns planning and status while GitHub still owns execution evidence. See [docs/jira-primary-provider.md](docs/jira-primary-provider.md).

The core flow is:

```text
install -> auth -> init/readiness -> ticket -> PR -> checks -> finish
```

## 3-Minute Quick Start

Install Gira:

```bash
curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh | sh
```

Or install through a currently published package manager channel:

```bash
npm install -g @statpan/gira
bun install -g @statpan/gira
uv tool install gira-cli
pipx install gira-cli
python -m pip install --user gira-cli
brew tap StatPan/tap
brew install gira
```

Release binary installs do not require Go. Use Go only when building or testing Gira from source.

Authenticate GitHub first. Gira uses GitHub as the execution backend and shells through `gh` for GitHub API access:

```bash
gira version
gh auth status
gira guide
```

`gira guide` is built into the installed binary. Use `gira guide quickstart`, `gira guide ticket`, `gira guide agent`, or `gira guide concepts` when you need the workflow without opening this README. `gira docs` is an alias.

Read before writing. For repository setup, run init from a clean checkout so Gira can fail closed before apply steps:

```bash
gira init --repo OWNER/REPO --path . --dry-run
gira adopt repo --repo OWNER/REPO --path . --dry-run
gira status --repo OWNER/REPO
```

Start and finish development from a ticket:

```bash
gira status
gira ticket new "Add login retry" \
  --goal "Retry transient auth failures" \
  --acceptance "retries 3 times;does not retry 401;has tests" \
  --dry-run
gira ticket new "Add login retry" \
  --goal "Retry transient auth failures" \
  --acceptance "retries 3 times;does not retry 401;has tests" \
  --apply --start
gira ticket list --state open --label status:ready --limit 20

# implement the bounded issue scope, then verify locally
go test ./...

gira ticket pr --dry-run
gira ticket pr --apply --draft
gira ticket view
gira ticket prompt --role implementer --profile python
gira ticket note "Parser path is implemented; local tests are next." --dry-run
gira ticket review --diff-summary
gira ticket self-review --diff-summary --dry-run
gira ticket checks
gira ticket wait --timeout 5m
gira ticket finish --dry-run
gira ticket finish --apply
gira ticket status
```

`gira ticket new` creates a repo-bound executable ticket. It writes a structured issue body from `--goal`, `--scope`, `--acceptance`, and `--notes`, or accepts a complete issue packet through `--body`, `--body-file PATH`, or `--body-file -`. It applies `type:*` and `status:ready`, previews the exact payload with `--dry-run`, can link the created issue to a native GitHub parent with `--parent N`, and can immediately start the branch with `--start`. `gira ticket parent TICKET --set PARENT --dry-run|--apply` links existing issues through the same native sub-issue surface; `--clear` removes that parent link, and omitting both flags shows the current parent. Requested labels must already exist in the repo; `ticket new` preflights labels during dry-run/apply and reports missing taxonomy instead of creating labels implicitly. `gira ticket list` lists GitHub issue-backed tickets with compact filters: `--state open|closed|all`, repeatable or comma-separated `--label`, `--assignee`, `--milestone`, `--limit`, and `--json`. `gira ticket view` shows the ticket operating card: issue state, Gira status, linked PR, blockers, next action, and next command. `gira ticket prompt` renders stateless planner, implementer, or reviewer prompts from the ticket, with `default` and `python` profiles. `gira ticket review --diff-summary` renders the current ticket's review packet with changed files, diff stat, hunk headers, acceptance mapping candidates, risk hints, and the full diff command; `--include-diff` prints the full diff only when explicitly requested. `gira ticket self-review --diff-summary --dry-run` previews a `kind=check` PR note for the current branch ticket, and `--apply` posts it to the linked PR. `gira ticket note` renders a structured context note and posts it to the issue, linked PR, or both after `--dry-run` review. `gira ticket supersede` creates a linked replacement issue, writes decision notes on both tickets, removes active status labels from the old ticket, adds `resolution:superseded`, and closes it after dry-run review. `gira ticket` resolves `--repo` from `.gira/config.yaml` or git origin. After `ticket start` checks out an `issue-N-*` branch, `ticket view`, `ticket prompt`, `ticket handoff`, `ticket review`, `ticket self-review`, `ticket note`, `ticket pr`, `ticket checks`, `ticket wait`, `ticket finish`, and `ticket status` can infer the ticket from the current branch or linked PR. Pass `--repo OWNER/REPO` and `--ticket N` when running outside that context.

For an existing repository, run `gira adopt repo --dry-run` before full bootstrap. Gira detects existing issues, labels, milestones, Projects, `AGENTS.md`, and GitHub templates, then recommends an adoption strategy. The default `merge` strategy preserves user-owned files and metadata while adding only the minimal Gira contract, such as `.gira/config.yaml` and an `AGENTS.md` managed block. Bootstrap sample issues are never required for normal adoption.

You can still call `gh` directly for low-level GitHub operations, but lifecycle work should use Gira ticket controls when available: `ticket view`, `ticket status`, `ticket start`, `ticket pr`, `ticket review`, `ticket self-review`, `ticket note`, `ticket supersede`, `ticket checks`, `ticket wait`, and `ticket finish`. Use raw `gh` only when Gira has no lifecycle command or when you intentionally need an unopinionated GitHub operation. The ticket commands keep the lifecycle consistent: they create or reuse the linked PR, require a closing body such as `Closes #TICKET`, compute blockers, render review packets and context notes, supersede stale work with linked replacement evidence, wait for pending checks, merge only when review and checks allow it, clean up the branch when safe, and give the next Gira command to run.

### Agent GitHub Replay Result

Gira includes an executable replay fixture for a GitHub-native ticket lifecycle:
`internal/gira/testdata/agent_github_replay/ticket-lifecycle.yaml`.

Run the comparison with:

```bash
go test ./internal/gira -run AgentGitHubReplay -v
```

Current fixture result:

| Metric | Raw `gh` | Gira | Delta |
| --- | ---: | ---: | ---: |
| Command nodes | 12 | 11 | 1 |
| Argument nodes | 39 | 34 | 5 |
| Decision nodes | 16 | 6 | 10 |
| Cognitive nodes | 7 | 0 | 7 |
| Direct labels touched by the agent | 4 | 0 | 4 |
| Provider calls | 12 | 16 | -4 |
| Discovery reads | 5 | 6 | -1 |
| Fallback escapes | 0 | 0 | 0 |
| Workflow cost x2 | 196 | 144 | 52 |

This is replay evidence, not live API instrumentation. The result shows less
agent workflow burden in the recorded command transcript, while provider calls
and discovery reads still need stateful Gira work before making API-efficiency
claims.

`gira epic list`, `gira epic status`, and `gira epic finish` close the larger planning loop without requiring raw `gh issue list` or `gh issue close`. `epic list` is a `type:epic` view over GitHub issues with the same compact list filters as ticket list. Gira can resolve an epic from the current `issue-N-*` branch, `--title`, `--slug`, `--milestone`, or a sole open `type:epic`; `--ticket N` remains the explicit fallback. `epic status` reads native GitHub sub-issues when available and merges them with older body or milestone child inference. `epic finish --apply` refuses to close while child issues are still open, then normalizes active status labels and closes the epic through GitHub.

`gira stats repo --repo OWNER/REPO --since 90d` renders the first Closure Funnel report. It is GitHub read-only, defaults to human text output, and uses `--json` only for automation. The report counts opened/closed issues, superseded issues, opened/merged PRs, closing-link hygiene, check friction, stale open work, and closure rate. `gira stats workspace --since 90d` is the planned multi-repo rollup; see [docs/closure-funnel-stats.md](docs/closure-funnel-stats.md).

## Jira-Primary Provider Mode

GitHub-native mode remains the default. Jira-primary mode is explicit and keeps a split source of truth: Jira owns planning and status, while GitHub owns branch, PR, review, checks, merge, and close evidence.

```bash
gira jira init --repo OWNER/REPO --api-base https://example.atlassian.net --project ABC --dry-run
gira jira init --repo OWNER/REPO --api-base https://example.atlassian.net --project ABC --apply
gira jira doctor --repo OWNER/REPO --sample-key ABC-123
gira jira mirror ABC-123 --repo OWNER/REPO --dry-run
gira jira mirror ABC-123 --repo OWNER/REPO --apply
gira ticket start ABC-123 --repo OWNER/REPO --apply
gira ticket finish --dry-run
gira ticket finish --apply
```

Provider config is non-secret and lives in the user-global repo registry. Jira credentials come from `JIRA_EMAIL` and `JIRA_API_TOKEN`. `gira jira doctor` is read-only and reports whether a Jira-primary repo is `supported`, `partially_supported`, or `blocked` before heavier operation. `ticket finish` refuses Jira Done while GitHub evidence is incomplete, including missing mirror issue, missing linked PR, draft PR, review blocker, failing or pending checks, or unmerged PR. Jira workflow mutation, background sync, full bidirectional sync, hosted dashboards, and Jira-only completion are outside the OSS CLI slices. See [docs/jira-primary-provider.md](docs/jira-primary-provider.md) and `gira guide jira`.

## Ticket Flow Cases

Most first-time work starts by creating a repo-bound ticket and immediately starting it:

```bash
gira ticket new "Fix install retry" \
  --goal "Retry transient install downloads" \
  --acceptance "retries temporary network failures;does not hide checksum errors;has tests" \
  --apply --start
```

If the GitHub issue already exists, start from that ticket number once. After Gira checks out the `issue-N-*` branch, the rest of the flow infers the ticket from context:

```bash
gira ticket start 42 --apply
gira ticket pr --apply --draft
gira ticket view
gira ticket review --diff-summary
gira ticket self-review --diff-summary --dry-run
gira ticket note "Ready for CI review." --target pr --dry-run
gira ticket checks
gira ticket wait --timeout 5m
gira ticket finish --apply
```

If someone reports an issue in GitHub without enough structure, normalize it before starting work: add the missing goal, scope, acceptance criteria, and `status:ready`/`type:*` labels in GitHub or recreate it through `gira ticket new`. Start only after the ticket is executable.

Use `--json` for automation:

```bash
gira status --repo OWNER/REPO --json
gira ticket status --repo OWNER/REPO --ticket 12 --json
```

## LLM And Agent Runbook

When an LLM or coding agent operates a Gira-managed repository, follow this order exactly. Do not skip dry-runs, do not use raw `gh` for product workflow steps when a `gira` lifecycle command exists, and keep the PR body linked to the source issue with `Closes #TICKET`. A Project item by itself does not authorize branch, PR, checks, merge, or finish work unless it is backed by a repository issue; route or lower repo-agnostic items first.

```bash
# 1. Read current state.
gh auth status
gira init --repo OWNER/REPO --path . --dry-run
gira adopt repo --repo OWNER/REPO --path . --dry-run
gira status --repo OWNER/REPO

# 2. Create and start a repo-bound ticket.
gira ticket new "TITLE" --goal "GOAL" --acceptance "item 1;item 2" --dry-run
gira ticket new "TITLE" --goal "GOAL" --acceptance "item 1;item 2" --apply --start
gira ticket new --title "TITLE" --body-file issue.md --dry-run
gira ticket new --title "TITLE" --body-file issue.md --apply --start

# 3. Implement the bounded issue scope, then verify locally.
go test ./...

# 4. Open or validate the PR through Gira.
gira ticket pr --dry-run
gira ticket pr --apply --draft
gira ticket view
gira ticket review --diff-summary
gira ticket self-review --diff-summary --dry-run
gira ticket note "Ready for review." --target pr --dry-run
gira ticket note "Ready for review." --target pr --apply

# 5. Finish through Gira after review and checks are ready.
gira ticket checks
gira ticket wait --timeout 5m
gira ticket finish --dry-run
gira ticket finish --apply

# 6. Re-check the ticket and Project bridge.
gira ticket status
```

Use GitHub assignees for accountable humans. Use `agent:*` labels for the execution actor, for example `agent:human`, `agent:codex`, `agent:reviewer`, or `agent:gira`. Use `lane:*` labels for delegation authority and risk, such as `lane:agent`, `lane:human`, or `lane:hybrid`. `gira projects sync` mirrors the execution actor label into the Project planning field; it does not replace GitHub assignees or make the Project item the source of truth.

Advanced visibility commands such as `gira projects sync --config .gira/config.yaml --dry-run`, `gira epic status`, and `gira epic finish --dry-run` are useful after the first ticket loop is working.

## Command Model

The Go-built `gira` binary is the sole product implementation. The default user experience is Jira-style ticket work backed by GitHub issues, PRs, labels, and milestones.

- `gira ticket ...` is the daily issue -> branch -> PR workflow.
- `gira workspace ...` is the Jira-like personal workspace and inbox layer for repo-agnostic backlog before work is routed to an execution repo.
- `gira projects ...` mirrors canonical repository issue state into visible GitHub Projects board items.
- `gira sprint ...`, `gira release`, and `gira status` are daily planning and reporting commands.
- `gira audit readiness --repo OWNER/REPO` combines doctor checks, audit ledger health, and the next Gira-first action.
- `gira ops ...` contains advanced setup, migration, policy, audit, and raw GitHub controls.
- `gira start` and `gira work ...` remain compatibility aliases.

## Advanced Setup

Most daily work should use `gira ticket`, `gira workspace`, `gira projects`, `gira sprint`, `gira release`, and `gira status`. Use `gira ops` for setup, adoption, policy, audit, and lower-level GitHub controls.

Plan setup without changing GitHub or repository files:

```bash
gira adopt repo --repo OWNER/REPO --path . --dry-run
gira ops bootstrap --repo OWNER/REPO --template default --dry-run
gira ops sync --repo OWNER/REPO --dry-run
```

Apply the reviewed setup:

```bash
gira ops bootstrap --repo OWNER/REPO --path /path/to/repo
gira ops sync --repo OWNER/REPO
gira ops onboard verify --repo OWNER/REPO --stage steady-state
```

## Jira To GitHub Mapping

Gira keeps the user-facing workflow close to Jira while storing canonical state in GitHub. The v1 model is intentionally small:

| Jira concept | GitHub object | Gira behavior |
| --- | --- | --- |
| Workspace | GitHub account plus configured inbox and execution repos | A workspace groups repo-agnostic intake with one or more execution repos so personal backlog is visible before it is assigned to a repo. |
| Project | Repository plus optional repo-linked GitHub Project board | A repo is the default execution space. Repo-linked Projects can show repo issue state even when the Project URL is under `users/OWNER/projects/N` or `orgs/OWNER/projects/N`. Multi-repo work starts as a top-level ticket and is lowered into repo issues when ownership is clear. |
| Backlog | Inbox repo issues plus repo issues | `gira workspace status` and `gira workspace backlog` show unrouted inbox tickets together with routed repo work. |
| Epic | Parent or top-level issue | A milestone-sized outcome. `gira epic status` and `gira epic finish` inspect and close epics without requiring the issue number when branch, title, slug, milestone, or repo context is enough. |
| Story / Task / Bug | Issue | The main work packet. Type, priority, blocked, and status are represented with managed labels and issue metadata. |
| Sprint | Milestone | `gira sprint` plans, starts, closes, and rolls over milestone-scoped work. |
| Status | Labels plus PR evidence | Gira reads and updates status labels, then cross-checks branch, PR, review, and check state. In Jira-primary mode, Jira owns planning/status and GitHub status labels are mirrors or execution hints. |
| Assignee | GitHub assignee | Ownership stays visible in GitHub and can be supplemented with owner or worker labels. |
| Branch | Issue execution context | `gira ticket start` verifies the ticket, creates or reuses a branch, and moves work to in-progress on apply. |
| Pull request | Change unit | `gira ticket pr` creates or validates a linked PR with a closing keyword such as `Closes #12`. |
| Done | Merged PR plus closed issue | Completion is proven by GitHub merge and close evidence, not by hidden local state. Jira Done transitions are gated on this evidence when Jira-primary mode is enabled. |
| Release | GitHub Release plus readiness report | `gira release readiness` checks whether issue, PR, review, and milestone evidence are ready for delivery. |

Full GitHub Projects v2 view automation, Web UI/TUI, chat bots, LLM decomposition, background Jira sync, and full bidirectional Jira sync are not v1 product workflows. Jira import/export commands are explicit migration helpers. Projects sync links and mirrors repository issue state into Projects; it does not make Project items the execution source of truth.

## Workspace Backlog

For a single repo, `gira status --repo OWNER/REPO` is enough. For Jira-style intake that is not yet tied to a repository, configure a personal workspace:

```yaml
workspace:
  name: personal
  owner: OWNER
  inbox_repo: OWNER/backlog
  repos:
    - OWNER/app
    - OWNER/cli
```

The inbox repo is where repo-agnostic tickets live. Execution repos are where branch, PR, milestone, and release work happens.

```bash
gira workspace status --config .gira/config.yaml
gira workspace backlog --config .gira/config.yaml
gira workspace list --config .gira/config.yaml
gira workspace sync --dry-run --config .gira/config.yaml
gira workspace ticket new "Define billing model" --body-file issue.md --config .gira/config.yaml
gira workspace ticket new "Define billing model" --repo OWNER/app --dry-run --config .gira/config.yaml
gira workspace ticket new "Define billing model" --repo OWNER/app --apply --config .gira/config.yaml
gira workspace ticket route --ticket 12 --repo OWNER/app --dry-run --config .gira/config.yaml
gira workspace ticket route --ticket 12 --repo OWNER/app --apply --config .gira/config.yaml
gira workspace project adopt --owner OWNER --title "Gira" --dry-run --config .gira/config.yaml
gira workspace project adopt --owner OWNER --title "Gira" --apply --config .gira/config.yaml
gira projects sync --dry-run --config .gira/config.yaml
gira projects sync --apply --config .gira/config.yaml
```

`workspace status` is safe for larger global workspaces: it reports GitHub API
budget when available, bounds concurrent repo reads, caches per-repo status for
five minutes by default, and supports `--repo`, `--limit`, `--active-only`, and
`--refresh` for practical CLI and future GUI refresh loops.

Use `gira config storage --repo OWNER/REPO` when you need to see what Gira
stores locally. It is read-only and classifies config, private runtime state,
disposable cache, export bundles, audit ledgers, and wrapper binary cache by
durability, privacy, rebuild source, and source-of-truth boundary.

`workspace ticket new --repo OWNER/REPO --apply` creates an inbox ticket, routes it to a repo execution issue, and links the child issue back to the inbox ticket without requiring the user to copy the inbox issue number. Use `--body`, `--body-file PATH`, or `--body-file -` when the inbox packet is already drafted in Markdown. `workspace ticket route --ticket N` remains available for older or externally-created inbox tickets. After routing, the normal loop continues with `gira ticket start`, `gira ticket pr`, and `gira ticket status` on the target repo.

`workspace project adopt` registers an existing profile or org GitHub Project in `workspace.project`; it never creates Projects and fails instead of replacing a different configured Project. `projects sync` then keeps that existing GitHub Projects v2 board visible by linking configured repos, adding missing open issues as project items, mirroring Gira status labels to the board's standard Status field, keeping closed issues as `Done`, creating supported planning fields, mirroring `priority:*`, `area:*`, and `agent:*` labels into Project planning fields, and copying milestone due dates into `Target date`. A Project may live under `users/OWNER/projects/N` or `orgs/OWNER/projects/N`; when it is linked to the repo and its items are repo issues, it is still a normal repo board surface. Repo issues remain the execution source of truth. Add `--archive-closed` only when closed issue items should leave the active Project item set. GitHub does not expose supported Project view creation APIs, so Gira reports the manual Board/Schedule view setup step instead of hiding it behind raw Project numbers.

## GitHub Workflow Templates

The default bootstrap template includes GitHub issue forms and a pull request template under `.github/`. These templates are part of the Gira repository contract: they help downstream repos collect the context needed before an issue becomes `status:ready`, then keep PRs tied to the issue -> branch -> checks -> finish loop.

```bash
gira ops bootstrap --repo OWNER/REPO --template default --dry-run
gira ops bootstrap --repo OWNER/REPO --path /path/to/repo
```

Issue forms collect goal, scope, acceptance criteria, priority, optional source ticket, rollout or migration notes, observability/security impact, and expected verification. They do not ask for a Gira ticket ID before issue creation because the GitHub issue number becomes the ticket ID after submission. The PR template requires a closing issue reference, records the expected `gira ticket start` and `gira ticket finish` lifecycle commands, and includes production-readiness checks for tests, migrations, rollout, observability, security, and docs.

## Install, Upgrade, and Remove

Gira is implemented as a Go-built CLI. Normal users should install the release binary through `install.sh`, npm/bun, PyPI via uv/pipx/pip, or Homebrew. These channels do not require Go, do not build from source, and do not mutate any repository.

### Release Binary Channels

Use one of the release channels:

```bash
curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh | sh
npm install -g @statpan/gira
bun install -g @statpan/gira
uv tool install gira-cli
python -m pip install --user gira-cli
pipx install gira-cli
brew tap StatPan/tap
brew install gira
```

All channels install the same Go-built `gira` binary. Package managers are wrappers around the release binary, not alternate runtimes.

### Install Script

Install the latest tagged release:

```bash
curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh | sh
```

Pin a version:

```bash
curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh | GIRA_VERSION=v1.0.0 sh
```

Install to a custom directory:

```bash
curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh | GIRA_INSTALL_DIR="${HOME}/bin" sh
```

The install script:

- Detects `os` and `arch`, then selects the matching GitHub release archive.
- Installs `latest` by default and accepts an explicit version, for example `GIRA_VERSION=v1.0.0`.
- Downloads from GitHub release assets, not from a source checkout or CI artifact.
- Requires the release `checksums.txt` asset and verifies the selected archive before unpacking it.
- Installs to `${GIRA_INSTALL_DIR}` when set, otherwise to `${HOME}/.local/bin`.
- Prints PATH guidance when the install directory is not already on `PATH`.
- Replaces an existing `gira` binary in the selected install directory atomically when possible.
- Never modifies repository files, GitHub labels, milestones, or issues during binary installation.

Verification:

```bash
command -v gira
gira --help
gira version
gira upgrade
gira doctor --repo OWNER/REPO
gira audit readiness --repo OWNER/REPO
```

### Developer Source Build

Use `go install` only for source and development workflows, not as a normal user install path:

```bash
go install github.com/StatPan/gira/cmd/gira@latest
```

The module is `github.com/StatPan/gira` and the binary package is under `cmd/gira`, so the install path includes `/cmd/gira`. If the repository is private in your environment, configure Go private module access first, for example with `GOPRIVATE=github.com/StatPan/gira` plus normal GitHub authentication.

From this checkout, build and install the current source version:

```bash
GOBIN="${HOME}/.local/bin" go install ./cmd/gira
```

Use a temporary install directory for smoke tests:

```bash
GOBIN="$(mktemp -d)" go install ./cmd/gira
"${GOBIN}/gira" --help
"${GOBIN}/gira" version
```

### GitHub Release Archives

Manual release-archive installation uses the same release assets as the install script. Release archive names follow:

```text
gira_VERSION_linux_amd64.tar.gz
gira_VERSION_linux_arm64.tar.gz
gira_VERSION_darwin_amd64.tar.gz
gira_VERSION_darwin_arm64.tar.gz
gira_VERSION_windows_amd64.zip
```

Example:

```bash
version=v1.0.0
curl -fLO "https://github.com/StatPan/gira/releases/download/${version}/gira_${version}_linux_amd64.tar.gz"
tar -xzf "gira_${version}_linux_amd64.tar.gz"
install -m 0755 "gira_${version}_linux_amd64/gira" "${HOME}/.local/bin/gira"
gira --help
```

Verify the archive with the release `checksums.txt` before installing the binary. Every v1 release should publish `checksums.txt`; treat a missing checksum asset as a release defect.

### Wrapper Distribution Boundaries

Package-manager wrappers such as Homebrew, npm, bun, pip, pipx, or `uv` are allowed only as distribution channels for the Go-built release binary. They must not reimplement Gira in another runtime, change the command surface, or install unversioned CI artifacts.

The npm/bun wrapper package is maintained under `packages/npm`, and the PyPI wrapper package is maintained under `packages/pypi`. Homebrew publishing targets the external `StatPan/homebrew-tap` repository. These channels install the Go-built release binary and verify release checksums.

Wrapper packages must preserve the same command surface as the native binary:

```bash
gira version
gira ticket start 12 --repo OWNER/REPO --dry-run
gira ops bootstrap --repo OWNER/REPO --template default --dry-run
gira ops sync --repo OWNER/REPO --dry-run
gira status --repo OWNER/REPO --json
```

Official release install channels:

```bash
npm install -g @statpan/gira
bun install -g @statpan/gira
uv tool install gira-cli
python -m pip install --user gira-cli
pipx install gira-cli
brew tap StatPan/tap
brew install gira
```

The install script downloads GitHub Release assets directly. The npm and bun channels install the `@statpan/gira` wrapper from the npm registry. The uv, pip, and pipx channels install the `gira-cli` wrapper from PyPI. The Homebrew channel is published through `StatPan/homebrew-tap`. All channels install the same Go-built release binary and keep the `gira` command surface unchanged. Normal release-binary users do not need Go installed.

apt/deb packaging is a future target, not an initial official channel. It should wait until usage justifies signing keys, repository hosting, architecture matrix maintenance, and upgrade policy support.

Unsupported distribution channels are source snapshots, unversioned binaries copied from CI artifacts, package wrappers that do not execute the official Go-built binary, and alternate product runtimes.

### Upgrade

Check the latest release and print the right next step for your install channel:

```bash
gira upgrade
gira update
gira upgrade --channel uv
gira upgrade --channel pipx
gira upgrade --json
```

`gira upgrade` and `gira update` are aliases. They are advisory by default: the installed Go-built binary checks the latest GitHub release and prints the channel-specific command, but it does not run package managers or mutate repositories. If Gira cannot confidently infer how it was installed, pass `--channel install.sh|uv|pipx|pip|homebrew|npm|bun`. Use `--channel go` only for developer source builds.

Use the same channel that installed Gira:

```bash
curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh | sh
curl -fsSL https://raw.githubusercontent.com/StatPan/gira/main/install.sh | GIRA_VERSION=v1.0.0 sh
uv tool upgrade gira-cli
python -m pip install --user --upgrade gira-cli
pipx upgrade gira-cli
brew update && brew upgrade gira
npm update -g @statpan/gira
bun update -g @statpan/gira
```

Upgrade with the same release channel used for installation. Developer source builds can be refreshed with `go install github.com/StatPan/gira/cmd/gira@latest` or `GOBIN="${HOME}/.local/bin" go install ./cmd/gira`, but those are not normal release-binary upgrade paths.

For uv, pip, and pipx installs, the `gira-cli` PyPI wrapper caches downloaded native binaries under `GIRA_PYPI_CACHE_DIR` when set, otherwise under `~/.cache/gira-cli/<version>`. After upgrading, preview stale cache cleanup before deleting anything:

```bash
gira cache prune --dry-run
gira cache prune --apply
```

`gira cache prune` only removes direct child directories with stable semver release names older than the active `gira` version. It skips the active version, newer versions, malformed entries, files, symlinks, and any directory containing the current executable. Use `--root PATH` for a custom wrapper cache and `--json` for automation.

An upgrade replaces only the local `gira` binary or package wrapper. It must not mutate repository files or GitHub metadata. After upgrading, verify the command still resolves from the expected install location:

```bash
command -v gira
gira --help
gira version
gira upgrade
gira doctor --repo OWNER/REPO
gira audit readiness --repo OWNER/REPO
```

### Binary Uninstall

Binary uninstall removes the local CLI from the machine. It does not detach a repository from Gira and does not delete GitHub labels, milestones, issues, or files.

For install-script or manual installs:

```bash
gira_path="${GIRA_INSTALL_DIR:+${GIRA_INSTALL_DIR}/gira}"
gira_path="${gira_path:-$(command -v gira)}"
rm "${gira_path}"
```

If you installed to a custom directory, set `GIRA_INSTALL_DIR` to that same directory or remove the path printed by `command -v gira`. For npm, bun, uv, pip, pipx, or Homebrew installs, uninstall with the same package manager used for installation.

Verification:

```bash
command -v gira
```

If `command -v gira` still prints a path, remove the remaining binary or package from that location.

### Repository Detach

Repository detach is separate from binary uninstall. Detach means removing or disabling Gira-managed repository files and GitHub metadata for one repository while leaving the local `gira` binary installed.

Future command contract:

```bash
gira ops detach --repo OWNER/REPO --dry-run
gira ops detach --repo OWNER/REPO --dry-run --json
gira ops detach --repo OWNER/REPO --apply
```

Default behavior must be dry-run-first:

- `gira ops detach --repo OWNER/REPO --dry-run` reports the Gira-managed files, labels, milestones, and bootstrap issues that would be removed, archived, or left in place.
- `gira ops detach --repo OWNER/REPO --dry-run --json` emits the same plan in machine-readable form for review and automation.
- `gira ops detach --repo OWNER/REPO --apply` performs only the actions shown by the dry-run plan and should require an explicit apply flag.
- Destructive deletion is never default behavior. File deletion, GitHub issue closure, label deletion, and milestone deletion must be opt-in and visible in the dry-run plan before apply.
- Detach must not delete user-authored project history by default. Prefer archiving, closing with an explanatory comment, or leaving unmanaged resources in place unless the operator explicitly requests cleanup.

Verification after detach:

```bash
gira status --repo OWNER/REPO --json
gira ops sync --repo OWNER/REPO --dry-run
```

`status` should make clear that the repository is not fully Gira-managed. `sync --dry-run` should show what would be recreated if the repository is adopted again.

## Daily Happy Path

From a fresh shell, make sure the install directory is on `PATH`, then run Gira directly (no source checkout):

```bash
gira --help
gira ops sync --repo OWNER/REPO --dry-run
gira ops onboard verify --repo OWNER/REPO --stage steady-state
gira status --repo OWNER/REPO
gira ticket start 12 --repo OWNER/REPO --dry-run
gira ticket start 12 --repo OWNER/REPO --apply
gira ticket pr --dry-run
gira ticket status
```

This is the canonical operator path for daily use.

Use `--json` for automation only; human output ends with a concise `next step:` line where Gira can suggest a safe continuation.

`gira start` and `gira work start|pr|status` remain compatibility aliases for users and scripts that already adopted the earlier issue-oriented wording. New documentation uses `ticket` because the intended mental model is Jira-style tickets mapped onto GitHub issues.

## Advanced Adoption And Migration

Use the bootstrap and policy-mode commands when introducing Gira to a new or already-configured repository:

```bash
gira ops bootstrap --repo OWNER/REPO --template default --dry-run
gira ops bootstrap --repo OWNER/REPO --path /path/to/repo
gira ops sync --repo OWNER/REPO --dry-run --policy-mode adopt
gira ops sync --repo OWNER/REPO --dry-run --policy-mode merge
gira ops sync --repo OWNER/REPO --dry-run --policy-mode enforce
gira ops onboard verify --repo OWNER/REPO --stage init --json
gira status --repo OWNER/REPO --json
```

Package-manager wrappers such as npm, bun, uv, pip, pipx, or Homebrew may be used as distribution channels when they install or invoke the Go-built `gira` binary. They are not alternate product implementations.

For local development from this checkout:

```bash
GOBIN="$(mktemp -d)" go install ./cmd/gira
"${GOBIN}/gira" --help
```

To smoke-test the binary from outside the source checkout:

```bash
GOBIN="$(mktemp -d)" go install ./cmd/gira
(cd /tmp && "${GOBIN}/gira" --help)
(cd /tmp && "${GOBIN}/gira" version)
(cd /tmp && "${GOBIN}/gira" ops bootstrap --repo OWNER/REPO --template default --dry-run)
(cd /tmp && "${GOBIN}/gira" ops bootstrap --repo OWNER/REPO --path /path/to/repo --no-branch)
(cd /tmp && "${GOBIN}/gira" ops sync --repo OWNER/REPO --dry-run)
(cd /tmp && "${GOBIN}/gira" ops sync --repo OWNER/REPO --dry-run --bootstrap-issues)
(cd /tmp && "${GOBIN}/gira" status --repo OWNER/REPO --json)
```

## Release Flow

Tagged Go releases are built by `.github/workflows/release.yml`. Pull requests and `main` pushes run validation builds only. Tags that start with `v` publish stable GitHub Release assets, then publish configured package-manager channels.

The workflow checks `install.sh` syntax, runs `go test ./...`, runs npm and PyPI wrapper tests, builds Linux, macOS, and Windows CLI archives with version metadata, generates `checksums.txt`, verifies the checksum manifest, and publishes those assets to the GitHub release. Published release assets are treated as immutable; rerun with a new patch tag instead of replacing an existing release. If `NPM_TOKEN` is configured, it publishes `@statpan/gira`. If `PYPI_API_TOKEN` is configured, it publishes `gira-cli`. If `HOMEBREW_TAP_TOKEN` is configured, it updates `StatPan/homebrew-tap`.

Maintainer flow:

```bash
git checkout main
git pull --ff-only origin main
$EDITOR CHANGELOG.md
git tag -a v1.0.0 -m "gira v1.0.0"
git push origin v1.0.0
```

Expected release assets:

```text
checksums.txt
gira_VERSION_linux_amd64.tar.gz
gira_VERSION_linux_arm64.tar.gz
gira_VERSION_darwin_amd64.tar.gz
gira_VERSION_darwin_arm64.tar.gz
gira_VERSION_windows_amd64.zip
```

After publishing, smoke-test the official installer against the tag:

```bash
tmpdir="$(mktemp -d)"
GIRA_INSTALL_DIR="${tmpdir}" GIRA_VERSION=v1.0.0 sh install.sh
"${tmpdir}/gira" --help
"${tmpdir}/gira" version
```

The release policy and package-manager channel details are documented in [docs/release-distribution.md](docs/release-distribution.md). Developer experience conventions for first-run onboarding, dry-run/apply output, JSON, recovery, and the issue-to-PR loop are documented in [docs/dx.md](docs/dx.md).

The Gira 2.0 release-readiness boundary is documented in [docs/v2-release-readiness.md](docs/v2-release-readiness.md). The GitHub-native Product OS schema for future Projects v2 planning, roadmap date semantics, permission/secret model, and dry-run-first automation is documented in [docs/product-os-schema.md](docs/product-os-schema.md). The execution roadmap for that phase is tracked in [docs/product-os-roadmap.md](docs/product-os-roadmap.md). The Gira state ownership model for labels, computed JSON state, receipts, local cache, and storage diagnostics is documented in [docs/state-model.md](docs/state-model.md). The Jira-vs-Gira operating boundary, work decomposition contract, and assistant/dev-agent split are documented in [docs/jira-gira-operating-boundary.md](docs/jira-gira-operating-boundary.md). Goal-level autonomy for long-running agent work is documented in [docs/goal-operating-model.md](docs/goal-operating-model.md). Jira-primary provider mode and workflow safety are documented in [docs/jira-primary-provider.md](docs/jira-primary-provider.md). Closure Funnel stats are documented in [docs/closure-funnel-stats.md](docs/closure-funnel-stats.md), and the Gira-native task momentum design is documented in [docs/task-momentum-loop.md](docs/task-momentum-loop.md). Agent delegation lanes, approval policy, and Delegation Quality metrics are documented in [docs/agent-delegation-lanes.md](docs/agent-delegation-lanes.md). The worker boundary and runtime evidence manifest are documented in [docs/worker-boundary-provenance.md](docs/worker-boundary-provenance.md) and [docs/worker-run-manifest.md](docs/worker-run-manifest.md). The workspace backlog layer is documented in [docs/workspace.md](docs/workspace.md), and the Agent Handoff Queue operating model is documented in [docs/agent-handoff-queue.md](docs/agent-handoff-queue.md). Business-group multi-repo workflow design is documented in [docs/business-group-workflows.md](docs/business-group-workflows.md), and the older portfolio intake compatibility layer is documented in [docs/portfolio-intake.md](docs/portfolio-intake.md). The current dashboard export bundle layout is documented in [docs/dashboard-export-artifacts.md](docs/dashboard-export-artifacts.md), with workspace dashboard additions in [docs/workspace-dashboard-contract-gaps.md](docs/workspace-dashboard-contract-gaps.md) and signal projection in [docs/dashboard-signal-projection.md](docs/dashboard-signal-projection.md). Hosted control-plane direction is summarized in [docs-site/hosted-control-plane.md](docs-site/hosted-control-plane.md); the older research note is archived in [docs/hosted-control-plane-roadmap.md](docs/hosted-control-plane-roadmap.md). The MVP CRUD support contract is documented in [docs/crud-capability-matrix.md](docs/crud-capability-matrix.md). Adoption on pre-configured repositories is documented in [docs/adoption-migration-playbook.md](docs/adoption-migration-playbook.md).

The first Gira 3.0 UX decision is documented in [docs/gira-3-local-report-bundle-ux.md](docs/gira-3-local-report-bundle-ux.md): start with a local report bundle over existing state contracts before adding hosted or TUI surfaces. The workspace dashboard contract gaps for that bundle are documented in [docs/workspace-dashboard-contract-gaps.md](docs/workspace-dashboard-contract-gaps.md).

The dashboard signal projection strategy is documented in [docs/dashboard-signal-projection.md](docs/dashboard-signal-projection.md): add `pulse` and `storage` to the local export bundle before GitHub Projects view expansion or any SQLite/local index.

The first local workspace report bundle can be generated with `gira export dashboard --config .gira/config.yaml --output out/dashboard --dry-run`, then applied by dropping `--dry-run`.

For an opt-in visual overview across selected repositories and milestones, run
`gira report portfolio --repo OWNER/app --milestone TITLE --since 2026-07-01 --until 2026-09-30 --output out/portfolio.html`.
The self-contained HTML shows milestone progress, dated gates, and
blocked/review-waiting queues. It writes only the explicit local output path and
never publishes, serves, or opens the artifact. See
[Visual Portfolio Report](docs/visual-portfolio-report.md).

WBS parent inference diagnostics and the structural-vs-execution report split for `gira report wbs` and `gira report schedule` are documented in [docs/wbs-report-diagnostics.md](docs/wbs-report-diagnostics.md).

This repository dogfoods Gira for its own work. The active operating loop, sprint commands, and maintainer handoff are documented in [docs/dogfood.md](docs/dogfood.md).

Explicit non-goals for 2.0: full GitHub Projects v2 view automation, LLM PRD-to-issue decomposition, Web UI/TUI, chat-bot integration, hosted dashboards, GitLab/Forgejo/Gitea providers, Notion integration, SQLite or another Gira-native planning database as the source of truth, background Jira sync, full bidirectional Jira sync, and Jira workflow mutation. Gira may keep explicit migration helpers, but the 2.0 source of truth is the GitHub execution loop unless a repo explicitly enables Jira-primary planning/status.
