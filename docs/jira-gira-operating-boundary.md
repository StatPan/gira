# Jira vs Gira Operating Boundary

Gira is GitHub-native project operating software, not a full Jira clone. Jira owns a broad, product-agnostic planning database. Gira deliberately keeps GitHub as the execution backend: issues are work packets, PRs are change units, milestones are sprint or phase boundaries, and CLI commands make those objects safer to operate.

The current product is execution-strong and portfolio/backlog-weak. That is an intentional MVP tradeoff: Gira should make repository work reliable before adding a separate intake or portfolio layer.

## Canonical Concept Mapping

Gira uses Jira-like language at the CLI boundary, then maps that language onto durable GitHub objects. The mapping below is the v1 contract.

| Jira concept | GitHub source of truth | Gira command surface | Notes |
| --- | --- | --- | --- |
| Project | Repository | `gira ops bootstrap`, `gira ops sync`, `gira status` | A repository is the normal execution space. Portfolio-level intake is only for work that has not yet been routed to a repo. |
| Epic | Parent issue or top-level ticket | `gira portfolio ...`, issue templates | A large outcome stays as an issue. Cross-repo epics coordinate child repo issues instead of creating a separate planning database. |
| Story / Task / Bug | Issue | `gira ticket ...`, templates, `gira ops sync` | The issue is the executable work packet for humans and agents. Labels carry type, status, priority, and blocked state. |
| Sprint | Milestone | `gira sprint plan|start|close|rollover` | Milestones are the canonical sprint or phase boundary. |
| Workflow status | Status labels plus PR evidence | `gira ticket start|pr|status`, `gira status` | Status labels are visible in GitHub; branch, PR, review, and check state provide execution evidence. |
| Assignee / owner | GitHub assignee plus optional labels | `gira status`, worker commands | Ownership remains GitHub-visible and does not require a hidden Gira user table. |
| Priority | Managed label | `gira ops sync`, triage/status commands | Priority is intentionally lightweight in v1. |
| Branch | Git branch linked to an issue | `gira ticket start` | Branches prove work has started, but they are not the source of truth without an issue or PR link. |
| Pull request | Pull request with closing keyword | `gira ticket pr` | PRs are change units. PR bodies must include `Closes #N`, `Fixes #N`, or `Resolves #N` unless the issue intentionally stays open. |
| Done | Merged PR and closed issue | `gira ticket status`, release/readiness commands | Completion is derived from GitHub evidence, not a local-only status file. |
| Release | GitHub Release plus readiness evidence | `gira release readiness` | Releases are delivery checkpoints backed by issue, PR, milestone, review, and check state. |
| Board / roadmap | Future or external view over GitHub state | Dashboard export, Project inspection | Projects v2 automation and UI surfaces are not v1 sources of truth. |

## Capability Matrix

| Capability | Jira native behavior | Current Gira state | Target direction |
| --- | --- | --- | --- |
| Intake / backlog layer | Project-agnostic tickets can exist before repository routing. | Partially supported. Portfolio repo issues can now act as top-level tickets, `gira portfolio plan --dry-run` computes lowering plans, and `gira portfolio lower --apply` can create/link execution issues. | Harden the lowering UX with templates and post-lowering validation. |
| Repo execution issue layer | Work items can represent tasks, stories, bugs, and epics. | Supported. GitHub issues plus Gira templates, labels, milestones, status, triage, and worker handoff form the execution packet. | Keep GitHub issues as the canonical execution unit. |
| Workflow state / transitions | Boards and workflow schemes own state transitions. | Partially supported. Gira uses status labels and transition commands; Project v2 field mutation remains bounded and capability-gated. | Improve everyday UX so ticket -> branch -> PR updates status without operators composing several commands. |
| Assignee / priority / SLA semantics | First-class assignees, priorities, queues, and SLA policies. | Partially supported. Priority and ownership are labels or GitHub metadata; stale/blocked/review signals are reported. SLA policy is not first-class. | Add clearer policy docs and reports before adding heavier automation. |
| Sprint / milestone planning | Sprints and releases are native planning objects. | Supported for MVP. Milestones represent sprint or phase boundaries; sprint plan/start/close/rollover commands exist. | Keep milestones as the canonical sprint boundary and avoid separate state unless needed. |
| Roadmap / project views | Roadmaps, boards, filters, and dashboards are native UI surfaces. | Partially supported. Gira can inspect Product OS state and export dashboard artifacts; Projects v2 automation is not MVP scope. | Keep export and inspection vendor-neutral; defer full Project v2 automation. |
| Automation / integrations | Rich automation rules and app ecosystem. | Partially supported. Gira is `gh`-first, dry-run-first, and command-driven; Jira import/export, chat bots, and UI integrations are not v1 product workflows. | Prefer explicit CLI workflows over hidden automation until behavior is stable. |
| Auditability / GitHub constraints | Jira audit trails live in Jira. | Supported through GitHub-native evidence: issues, PRs, labels, milestones, review state, and append-only Gira audit ledgers for mutations. | Preserve traceability without creating a shadow source of truth. |

GitHub-native does not mean full Jira parity. It means the canonical state stays on GitHub objects that developers already use. Gira fills the operational gaps around setup, state normalization, handoff, review, reporting, and safe command execution.

## GitHub Execution Matrix

Jira parity is only half of the decision. Gira can execute reliably only where GitHub exposes durable objects, APIs, permissions, and reviewable history. This matrix shows which GitHub surfaces can carry Jira-like behavior today.

| GitHub surface | Native capability | Gira use today | Execution strength | Main gap |
| --- | --- | --- | --- | --- |
| Issues | Work packets, comments, labels, assignees, task lists, closing references. | Primary execution issue layer, templates, status labels, triage normalization, worker handoff, stale/blocked reporting. | Strong. Issues are durable, scriptable, reviewable, and easy for humans and agents to share. | Repo-scoped by default; not a complete project-agnostic intake layer. |
| Labels | Lightweight taxonomy and status flags. | Type, status, priority, agent/owner, blocked, bootstrap metadata. | Strong for simple deterministic state. | Labels are flat strings, so complex workflow policy needs Gira conventions and validation. |
| Milestones | Sprint, phase, or release boundary with due dates and issue counts. | Sprint planning, sprint close, rollover, milestone progress, phase completion reporting. | Strong for repo-local sprint cadence. | Cross-repo portfolio planning needs an external parent layer or dashboard export. |
| Pull requests | Change unit, review, checks, merge state, closing keywords. | PR creation/status, closure-link gates, review queue, merge queue, release readiness. | Strong. PRs are the best source of execution evidence. | Draft/review/check semantics need a friendlier issue-lifecycle wrapper for daily UX. |
| Branches | Work-start signal and local execution context. | `ticket start` creates issue branches and transition planning can infer in-progress work. | Medium. Useful as evidence, but less durable than issues/PRs. | Branch-only work is ambiguous until linked back to an issue or PR. |
| GitHub Projects v2 | Boards, fields, roadmap views, project items. | Capability probing and dry-run/project inspection; mutation is deliberately limited. | Medium for visibility, weak for MVP mutation. | Projects v2 automation is not MVP scope and has separate permission complexity. |
| GitHub Actions / checks | CI status, scheduled jobs, policy checks. | Review gate, release readiness, quality gate, scheduled reporting concepts. | Medium. Good for verification and monitoring. | Poor default for primary development execution; failures often need interactive repair. |
| Rulesets / branch protection | Merge safety, required checks, review policy. | Guardrails audit/apply and merge readiness inputs. | Strong for enforcement. | Requires admin capability and careful non-destructive policy ownership. |
| Releases | Delivery checkpoint and published artifact boundary. | Release readiness reports. | Medium. Good as an output checkpoint. | Not a planning object by itself; needs issues, milestones, and PR evidence. |
| Repository settings / permissions | Auth, scopes, admin controls. | Capability reports gate apply behavior. | Strong as a safety boundary. | Permission failures must stay explicit and non-magical. |

The practical execution baseline is therefore: Issues + Labels + Milestones + PRs are strong enough for a Jira-like repository operating loop. Projects v2, Actions, and dashboards improve visibility, but they should not become the hidden source of truth in the MVP.

## Work Decomposition Contract

Gira has three work layers:

| Layer | Purpose | Entry criteria | Exit criteria |
| --- | --- | --- | --- |
| Top-level ticket | Product or operator intent before repository ownership is known. | Goal, scope, user impact, routing uncertainty, and parent links are clear. | It is either routed to one repo issue, split into child repo issues, or kept as non-executable planning context. |
| Repo execution issue | Bounded work packet for a human or Codex-style dev agent. | Target repo is known; goal, scope, files to change, verification commands, acceptance criteria, and blocker format are present. | A linked PR closes or resolves the issue, or the issue is explicitly blocked/deferred. |
| PR / change unit | Reviewable implementation delta. | Branch exists, code/docs change is bounded to one issue, and PR body links the issue with `Closes #N`, `Fixes #N`, or `Resolves #N`. | Review and checks pass, PR merges, and GitHub closes the linked issue. |

Lowering rules:

- If the target repo is known, create or attach a repo execution issue.
- If the target repo is unknown, keep the item top-level and do not hand it to a dev agent.
- If the work spans multiple repos, split it into child repo issues and keep the top-level ticket as the parent coordination object.
- If the work is only reporting, policy, or architecture context, it may remain top-level until an executable change is identified.

The default Codex handoff packet for a repo execution issue is:

- `repo`
- `issue_number`
- `goal`
- `scope`
- `files_to_change`
- `verification_commands`
- `acceptance_criteria`
- `blocker_format`
- `parent_ticket` when applicable
- `non_goals` or explicit safety boundaries

Single-repo example: "Add closure-link detection to review gates" starts as a repo issue in `StatPan/gira`, gets branch `issue-120-add-closure-link-detection`, opens a PR with `Closes #120`, then transitions from ready to in progress to in review to done through branch/PR evidence.

Multi-repo example: "Standardize onboarding across CLI and docs site" remains top-level until split into `StatPan/gira` CLI issue and `StatPan/docs` site issue. Each child issue gets its own branch, PR, verification commands, and closing link. The parent closes only when the child issues are done or explicitly deferred.

## Assistant vs Dev-Agent Boundary

| Responsibility | Assistant / orchestrator | Dev agent |
| --- | --- | --- |
| Intake | Clarifies intent, identifies repo ownership, creates or updates top-level tickets. | Does not start until a repo execution issue is clear. |
| Triage | Labels, prioritizes, summarizes blockers, and routes work. | Consumes the routed issue packet. |
| Execution | Tracks progress and handles human interrupts. | Creates branch, edits code/docs, runs tests, opens PR. |
| State updates | May run safe status, report, onboard, triage, and work orchestration commands. | Updates implementation evidence through commits, PR body, test output, and issue comments. |
| Review loop | Summarizes review queue and policy failures. | Fixes requested changes and re-runs verification. |

The default operating model is issue/PR-based execution. Cron is appropriate for review, monitoring, collection, and scheduled reports, but cron-first development is a poor default because implementation often needs interactive judgment, local tests, branch context, and rapid human interruption handling.

Live session orchestration is justified only for narrow exceptions:

- urgent production or release-blocking work,
- multi-repo coordination where sequencing matters,
- credential or permission failures that need human intervention,
- ambiguous product decisions that block execution.

## Everyday UX Target

The current CLI exposes the Jira-style daily loop through `gira ticket`. Advanced setup and policy controls stay under `gira ops` so new users do not need to learn GitHub-native internals before starting work.

```bash
gira ticket new "Title" --goal "Goal" --acceptance "item 1;item 2" --dry-run
gira ticket new "Title" --goal "Goal" --acceptance "item 1;item 2" --apply --start
gira ticket start N --dry-run
gira ticket start N --apply
gira ticket pr --dry-run
gira ticket pr --apply --draft
gira ticket checks
gira ticket wait --timeout 5m
gira ticket finish --dry-run
gira ticket finish --apply
gira ticket status --json
```

Expected behavior:

- `ticket new --apply` creates a repo-bound executable ticket with a structured body, `type:*`, and `status:ready`; `--start` immediately continues into branch start.
- `ticket start --apply` verifies the ticket is ready, creates or checks out the branch, and applies `status:in-progress`.
- `ticket pr --apply` opens a linked PR with `Closes #N`; draft PRs keep `status:in-progress`, non-draft PRs move to `status:in-review`.
- `ticket checks` shows linked PR checks, review blockers, and the next Gira command without requiring raw `gh`.
- `ticket wait` waits for pending linked PR checks and reports the remaining blockers or finish command.
- `ticket finish --apply` marks a linked draft PR ready when needed, refuses failing checks or missing review, merges when safe, and reports final convergence.
- `ticket status` shows linked PR state, checks, review blockers, current issue status, and the next suggested command.

This keeps advanced adoption commands such as `gira ops sync --policy-mode adopt|merge|enforce` available for migrations while making daily Jira-style work feel like one coherent ticket lifecycle.
