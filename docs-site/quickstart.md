# Quick Start

The shortest successful Gira flow is install, authenticate, create one ticket,
open a PR, wait for checks, and finish. Mutating commands are always previewed
with `--dry-run` before `--apply`.

## 1. Authenticate and Inspect

```bash
gh auth status
gira version
gira init --repo OWNER/REPO --path . --dry-run
gira adopt repo --repo OWNER/REPO --path . --strategy merge --dry-run
gira adopt repo --repo OWNER/REPO --path . --strategy merge --apply
gira status --repo OWNER/REPO
```

`gira init --dry-run` only plans onboarding; `gira adopt repo --strategy merge
--apply` is the explicit mutation that installs the reviewed repository
contract. Use the strategy emitted by the preview when it differs.

## 2. Create and Start a Ticket

```bash
gira new "Add login retry" \
  --goal "Retry transient auth failures" \
  --acceptance "retries 3 times;does not retry 401;has tests" \
  --start --dry-run

gira new "Add login retry" \
  --goal "Retry transient auth failures" \
  --acceptance "retries 3 times;does not retry 401;has tests" \
  --start --apply
```

`gira new` and `gira t n` are short aliases for `gira ticket new`. With the
default `--branch auto`, Gira creates a suggested issue branch from the
resolved base, binds an existing non-base checkout without renaming or
pushing it, and handles a detached checkout safely. Use
`--branch new|current|NAME` when the branch choice must be explicit. See
[Branch Behavior](/branch-policy) for `--branch auto` and explicit-policy
repositories.

## 3. Open and Check the PR

```bash
gira ticket pr --dry-run
gira ticket pr --apply
gira ticket checks
gira ticket wait --timeout 5m
```

## 4. Finish

```bash
gira ticket finish --dry-run
gira ticket finish --apply
gira ticket status
```

## Existing Issue Or Agent Handoff

For an existing ready issue, preview and apply the same automatic start:

```bash
gira ticket start 42 --dry-run
gira ticket start 42 --apply
```

For an external coding agent, use the bounded single-ticket handoff. Goal,
queue, and PM commands are advanced paths, not prerequisites for this flow:

```bash
gira ticket handoff 42 --repo OWNER/REPO --json
```

See [Ticket Workflow](/ticket-workflow) for the complete lifecycle and
[Readiness And Audit](/readiness-audit) for observation, managed-advisory, and
managed-required operation modes.
