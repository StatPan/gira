# Ticket Workflow

The daily Gira path is one GitHub issue, one work branch, one linked PR, and
one evidence-backed finish. Gira is the workflow UX; `gh` remains the backend.

## Golden Path

Preview every mutation first. The short aliases `gira new` and `gira t n` keep
the same behavior as `gira ticket new`.

```bash
gira new "TITLE" --goal "GOAL" --acceptance "a;b;c" --start --dry-run
gira new "TITLE" --goal "GOAL" --acceptance "a;b;c" --start --apply

gira ticket pr --dry-run
gira ticket pr --apply
gira ticket checks
gira ticket wait --timeout 5m
gira ticket finish --dry-run
gira ticket finish --apply
```

Add `--draft` when review must happen before a PR is ready. A finish preview
for a draft PR only marks it ready; run finish dry-run/apply again after that
transition.

When an issue already exists, start it explicitly:

```bash
gira ticket start 42 --dry-run
gira ticket start 42 --apply
```

## Branch Behavior

The default branch selection is `auto`:

| Checkout | Automatic result |
| --- | --- |
| Resolved base branch | Create the suggested issue branch from that base. |
| Existing non-base branch | Bind the current branch without checkout, rename, or push. |
| Detached checkout | Create the suggested issue branch from the recorded/resolved base. |

Use `--branch auto|new|current|NAME` for a deterministic choice. `NAME`
adopts an existing local or origin branch. The compatibility spellings
`--create`, `--current`, and `--adopt BRANCH` remain supported, but cannot be
combined with `--branch`. A repository with `start_mode: explicit` requires a
branch choice and stops before mutation when none is supplied.

See [Branch Behavior](/branch-policy) for base resolution, lifecycle markers,
and PR-base validation.

## Existing Issue Packets

Use full Markdown when the issue packet is already drafted:

```bash
gira ticket new --title "TITLE" --body-file issue.md --dry-run
gira ticket new --title "TITLE" --body-file issue.md --apply
gira ticket list --state open --label status:ready --limit 20
```

Use `gira ticket view` for the operating card. Use `gira ticket handoff --json`
for a worker-neutral single-ticket packet. `gira dispatch goal`, queue, and PM
commands are advanced orchestration; they are not required for ordinary issue
work. The [Agent Operator](/agent-operator-skill) page describes the minimal
handoff path.

## Review And Finish Evidence

- Keep the PR body linked with `Closes #N`, `Fixes #N`, or `Resolves #N`.
- Run `gira ticket review --diff-summary` before requesting review.
- Run `gira ticket self-review --diff-summary --dry-run` before posting a self-review note.
- Use `gira ticket checks` and `gira ticket wait` to distinguish pending from failed checks.
- Run `gira ticket finish --dry-run` before `--apply`; finish validates the linked PR, checks, review, base, labels, closing reference, and acceptance evidence.

The detailed readiness schemas and reports are in [Readiness And Audit](/readiness-audit).
The generated [Command Reference](/command-reference) contains every flag,
alias, example, and JSON contract. It is exhaustive reference, not the daily
entry point.

## Operation Policy

Operation policy changes enforcement without changing the evidence shape:

| Mode | Meaning |
| --- | --- |
| `observation` | Report neutral provider facts; Gira labels, closing links, and approval conventions are advisory and do not block. |
| `managed` + `delivery_policy: advisory` | Report managed-delivery gaps as warnings with policy provenance. |
| `managed` + `delivery_policy: required` | Enforce managed-delivery requirements and fail closed on unknown policy/provider state. |

See [Readiness And Audit](/readiness-audit) for finding classes and
[Troubleshooting](/troubleshooting) for context failures.

## Agent Rules

- Start from a GitHub issue and keep one feature branch per issue.
- Prefer Gira lifecycle commands over raw `gh` when Gira provides the operation.
- Keep changes bounded to the issue and preserve unrelated work.
- Treat project-only items as intake until a repository issue exists.
