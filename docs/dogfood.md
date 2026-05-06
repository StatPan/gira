# Dogfood Operating Loop

Gira operates this repository through the same GitHub-native workflow it gives users. GitHub remains the source of truth: issues are tickets, milestones are sprint or phase boundaries, branches are work-start evidence, and pull requests are change units.

## Current Milestone

The active dogfood milestone is `v1.1 Dogfood`.

Current dogfood tickets should be read from GitHub before choosing work:

```bash
gira status --repo StatPan/gira
gira workspace status --config .gira/config.yaml
gira projects sync --config .gira/config.yaml --dry-run
```

Treat `status:ready` issues in the active milestone as candidates. Treat `type:epic` issues as planning parents unless the issue body explicitly asks for implementation. The local sprint state is recorded in `.gira/sprints/statpan-gira/state.json` so maintainers and agents can reproduce sprint commands from the repository checkout.

## Daily Commands

Start by reading the repo status:

```bash
gira status --repo StatPan/gira
```

If a workspace inbox is configured, read repo-agnostic backlog before choosing work:

```bash
gira workspace status --config .gira/config.yaml
gira workspace backlog --config .gira/config.yaml
gira projects sync --config .gira/config.yaml --dry-run
```

Create a repo-bound ticket, then start work:

```bash
gira ticket new "TITLE" --goal "GOAL" --acceptance "item 1;item 2" --dry-run
gira ticket new "TITLE" --goal "GOAL" --acceptance "item 1;item 2" --apply --start
```

For an existing issue, use `gira ticket start TICKET --apply`.

Open or validate the linked pull request:

```bash
gira ticket pr --dry-run
gira ticket pr --apply --draft
```

Finish the ticket through Gira after review and checks are ready:

```bash
gira ticket finish --dry-run
gira ticket finish --apply
```

Check the ticket at any point:

```bash
gira ticket status
```

## Sprint Commands

Plan the active sprint before starting it:

```bash
gira sprint plan --repo StatPan/gira --iteration "v1.1 Dogfood" --capacity 3 --issues 180,181,189 --dry-run
gira sprint plan --repo StatPan/gira --iteration "v1.1 Dogfood" --capacity 3 --issues 180,181,189 --apply
```

Start the sprint after the commitment is reviewed:

```bash
gira sprint start --repo StatPan/gira --iteration "v1.1 Dogfood" --dry-run
gira sprint start --repo StatPan/gira --iteration "v1.1 Dogfood" --apply
```

Close or roll over work at the end of the sprint:

```bash
gira sprint close --repo StatPan/gira --iteration "v1.1 Dogfood" --completed 180,181 --spillover-disposition carry --rollover-reason "continue docs polish" --dry-run
gira sprint rollover --repo StatPan/gira --dry-run
```

## Release Check

Before cutting a release, run:

```bash
gira release readiness --repo StatPan/gira
```

Use `--json` for automation and saved evidence.

## Command Boundary

- Use `gira ticket`, `gira sprint`, `gira release`, and `gira status` for daily work.
- Use `gira ops` for setup, sync, onboarding, guardrails, audit, export, and lower-level GitHub controls.
- Keep PR bodies linked with `Closes #N`, `Fixes #N`, or `Resolves #N` unless the source issue intentionally stays open.
- Keep changes bounded to the ticket and run the verification commands listed in the issue body.
