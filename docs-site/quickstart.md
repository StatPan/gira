# Quick Start

The first successful Gira flow is install, authenticate, inspect, create a ticket, open a PR, wait for checks, and finish.

## 1. Authenticate and Inspect

```bash
gh auth status
gira version
gira init --repo OWNER/REPO --path . --dry-run
gira adopt repo --repo OWNER/REPO --path . --dry-run
gira status --repo OWNER/REPO
```

## 2. Create and Start a Ticket

```bash
gira ticket new "Add login retry" \
  --goal "Retry transient auth failures" \
  --acceptance "retries 3 times;does not retry 401;has tests" \
  --apply --start
```

## 3. Open and Check the PR

```bash
gira ticket pr --apply --draft
gira ticket checks
gira ticket wait --timeout 5m
```

## 4. Finish

```bash
gira ticket finish --dry-run
gira ticket finish --apply
gira ticket status
```
