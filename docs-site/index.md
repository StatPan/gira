---
layout: home

hero:
  name: Gira
  text: Jira-style project flow on GitHub
  tagline: Use GitHub issues, branches, PRs, and milestones as a safer project operating system.
  actions:
    - theme: brand
      text: Quick Start
      link: /quickstart
    - theme: alt
      text: Install
      link: /install

features:
  - title: Plan/apply safety
    details: Preview repository and GitHub changes before applying them, then keep ticket progress visible through GitHub evidence.
  - title: Agent-ready workflow
    details: Use Gira commands instead of raw gh when a Gira lifecycle command exists, and keep each PR linked to a source ticket.
  - title: GitHub remains canonical
    details: Map Jira concepts onto GitHub without creating a separate planning database or forcing Projects v2 automation into v1.
---

## Daily Loop

```bash
gh auth status
gira init --repo OWNER/REPO --path . --dry-run
gira adopt repo --repo OWNER/REPO --path . --dry-run
gira ticket new "TITLE" --goal "GOAL" --acceptance "done criteria" --apply --start
gira ticket pr --apply --draft
gira ticket checks
gira ticket wait --timeout 5m
gira ticket finish --apply
```

Gira is a Go-built CLI. Package managers are distribution channels for the same official binary, not alternate product runtimes.
