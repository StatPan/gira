# Command Capabilities

This page is generated from Gira's command metadata registry. Update `internal/gira/command_registry.go` first, then refresh this page.

Schema version: `gira-command-capabilities/v1`

| Command | Aliases | Capability | JSON support | Mutation boundary | Docs |
| --- | --- | --- | --- | --- | --- |
| `gira completion` | none | `read` | `none` | none | README.md, docs-site/command-reference.md |
| `gira feature check` | gira feat check | `read` | `stable_json` | none | docs/feature-map.md, docs-site/feature-map.md, docs-site/command-reference.md |
| `gira feature for` | gira feat for | `read` | `stable_json` | none | docs/feature-map.md, docs-site/feature-map.md, docs-site/command-reference.md |
| `gira feature list` | gira feat list | `read` | `stable_json` | none | docs/feature-map.md, docs-site/feature-map.md, docs-site/command-reference.md |
| `gira goal finish` | none | `apply_mutation` | `stable_json` | posts an idempotent goal finish handoff receipt when run with --apply; --dry-run previews readiness and receipt | docs/goal-operating-model.md, docs-site/command-reference.md |
| `gira goal next` | none | `read` | `stable_json` | none | docs/goal-operating-model.md, docs-site/command-reference.md |
| `gira goal plan` | none | `apply_mutation` | `stable_json` | creates linked child tickets from reviewed goal-plan proposals when run with --apply; --dry-run previews the same plan | docs/goal-operating-model.md, docs-site/command-reference.md |
| `gira goal report` | gira goal dossier | `read` | `stable_json` | none | docs/goal-operating-model.md, docs-site/goal-mode.md, docs-site/command-reference.md |
| `gira goal status` | none | `read` | `stable_json` | none | docs/goal-operating-model.md, docs-site/command-reference.md |
| `gira jira doctor` | none | `read` | `stable_json` | none | README.md, docs/jira-primary-provider.md, docs-site/jira-primary-provider.md |
| `gira jira export` | none | `apply_mutation` | `stable_json` | writes Jira-friendly export artifacts to the requested output path | README.md, docs/jira-primary-provider.md, docs-site/jira-primary-provider.md |
| `gira jira import` | none | `apply_mutation` | `stable_json` | creates GitHub issues from Jira import sources; --dry-run previews created and skipped issues | README.md, docs/jira-primary-provider.md, docs-site/jira-primary-provider.md |
| `gira jira init` | none | `apply_mutation` | `stable_json` | writes reviewed non-secret Jira provider config; --dry-run previews discovered config | README.md, docs/jira-primary-provider.md, docs-site/jira-primary-provider.md |
| `gira jira mirror` | none | `apply_mutation` | `stable_json` | creates or reuses a GitHub mirror issue; --dry-run previews mirror resolution | README.md, docs/jira-primary-provider.md, docs-site/jira-primary-provider.md |
| `gira jira transition` | none | `dry_run_mutation` | `stable_json` | plans Jira transition reachability only; adapters must not treat it as Jira mutation approval | README.md, docs/jira-primary-provider.md, docs-site/jira-primary-provider.md |
| `gira milestone assign` | none | `apply_mutation` | `stable_json` | assigns selected issues to a milestone; --dry-run previews issue updates | docs-site/sprint-release.md, docs-site/ticket-workflow.md |
| `gira milestone list` | none | `read` | `stable_json` | none | docs-site/sprint-release.md, docs-site/ticket-workflow.md |
| `gira milestone new` | none | `apply_mutation` | `stable_json` | creates a GitHub milestone; --dry-run previews payload and target repo | docs-site/sprint-release.md, docs-site/ticket-workflow.md |
| `gira milestone plan` | none | `apply_mutation` | `stable_json` | selects and assigns candidate tickets; --dry-run previews candidate set and mutations | docs-site/sprint-release.md, docs-site/ticket-workflow.md |
| `gira milestone status` | none | `read` | `stable_json` | none | docs-site/sprint-release.md, docs-site/ticket-workflow.md |
| `gira setup global` | none | `apply_mutation` | `stable_json` | writes global config and repo registry files; --dry-run previews file changes | README.md, docs/global-config-registry.md, docs-site/global-config.md, docs/workspace.md |
| `gira stats repo` | none | `read` | `stable_json` | none | README.md, docs/closure-funnel-stats.md, docs-site/closure-funnel-stats.md |
| `gira stats workspace` | none | `unsupported` | `planned` | none | docs/closure-funnel-stats.md, docs-site/closure-funnel-stats.md |
| `gira ticket checks` | none | `read` | `stable_json` | none | README.md, docs-site/ticket-workflow.md, docs/dogfood.md |
| `gira ticket finish` | none | `apply_mutation` | `stable_json` | may merge the linked PR, post receipts, normalize labels, and close the issue; --dry-run previews readiness and actions | README.md, docs-site/ticket-workflow.md, docs/dogfood.md |
| `gira ticket handoff` | none | `read` | `stable_json` | none | docs-site/ticket-workflow.md, docs-site/command-reference.md, docs/dogfood.md |
| `gira ticket new` | none | `apply_mutation` | `stable_json` | creates a GitHub issue and may optionally start it; --dry-run previews issue body and labels | README.md, docs-site/ticket-workflow.md, docs/dogfood.md |
| `gira ticket note` | none | `apply_mutation` | `stable_json` | posts issue or PR comments; --dry-run previews resolved targets and rendered note | README.md, docs-site/ticket-workflow.md, docs/dogfood.md |
| `gira ticket pr` | none | `apply_mutation` | `stable_json` | creates or validates a linked PR; --dry-run previews PR body and branch binding | README.md, docs-site/ticket-workflow.md, docs/dogfood.md |
| `gira ticket prompt` | none | `read` | `stable_json` | none | README.md, docs-site/ticket-workflow.md, docs/dogfood.md |
| `gira ticket review` | none | `read` | `stable_json` | none | docs-site/ticket-workflow.md, docs-site/command-reference.md, docs/dogfood.md |
| `gira ticket self-review` | none | `apply_mutation` | `stable_json` | posts a self-review check note to the linked PR; --dry-run previews the rendered note and approval evidence | docs-site/ticket-workflow.md, docs-site/command-reference.md, docs/dogfood.md |
| `gira ticket start` | gira start | `apply_mutation` | `stable_json` | creates or reuses a branch, records lifecycle state, and moves the issue to in-progress; --dry-run previews readiness and branch plan | README.md, docs-site/ticket-workflow.md, docs/dogfood.md |
| `gira ticket status` | none | `read` | `stable_json` | none | README.md, docs-site/ticket-workflow.md, docs/dogfood.md |
| `gira ticket supersede` | none | `apply_mutation` | `stable_json` | creates a replacement ticket, posts cross-links, and closes the original; --dry-run previews all planned mutations | README.md, docs-site/ticket-workflow.md, docs/dogfood.md |
| `gira ticket view` | gira ticket show | `read` | `stable_json` | none | README.md, docs-site/ticket-workflow.md, docs/dogfood.md |
| `gira ticket wait` | none | `read` | `stable_json` | none | README.md, docs-site/ticket-workflow.md, docs/dogfood.md |
| `gira workspace repos sync` | none | `apply_mutation` | `stable_json` | updates workspace repo allowlist; --dry-run previews selected repositories | docs/global-config-registry.md, docs-site/global-config.md, docs/workspace.md |
| `gira workspace status` | none | `read` | `stable_json` | none | README.md, docs/workspace.md, docs-site/global-config.md |
