# Command Capabilities

This page is generated from Gira's command metadata registry. Update `internal/gira/command_registry.go` first, then refresh this page.

Schema version: `gira-command-capabilities/v1`

| Command | Tier | Workflow role | Aliases | Capability | JSON support | Mutation boundary | Docs |
| --- | --- | --- | --- | --- | --- | --- |
| `gira completion` | `supporting` | `none` | none | `read` | `none` | none | README.md, docs-site/command-reference.md |
| `gira config storage` | `assist` | `none` | none | `read` | `stable_json` | none | docs/global-config-registry.md, docs/state-model.md, docs-site/global-config.md, docs-site/command-reference.md |
| `gira dispatch goal` | `advanced_orchestration` | `canonical_goal_agent_entry_point` | none | `read` | `stable_json` | none | docs/dispatch-operating-model.md, docs/dispatch-reflection.md, docs/goal-operating-model.md, docs-site/command-reference.md |
| `gira feature check` | `managed_delivery` | `none` | gira feat check | `read` | `stable_json` | none | docs/feature-map.md, docs-site/feature-map.md, docs-site/command-reference.md |
| `gira feature for` | `managed_delivery` | `none` | gira feat for | `read` | `stable_json` | none | docs/feature-map.md, docs-site/feature-map.md, docs-site/command-reference.md |
| `gira feature list` | `managed_delivery` | `none` | gira feat list | `read` | `stable_json` | none | docs/feature-map.md, docs-site/feature-map.md, docs-site/command-reference.md |
| `gira goal finish` | `advanced_orchestration` | `none` | none | `apply_mutation` | `stable_json` | posts an idempotent goal finish receipt; explicit --terminal done may normalize labels and close the goal, while explicit --terminal human_review preserves blocker handoff | docs/goal-operating-model.md, docs-site/command-reference.md |
| `gira goal graph` | `advanced_orchestration` | `typed_work_graph_planning_engine` | none | `apply_mutation` | `stable_json` | compiles read-only by default; --apply lowers fingerprint-approved child actions and posts a receipt | docs/goal-operating-model.md, docs/pm-operating-policy.md, docs-site/command-reference.md |
| `gira goal handoff` | `advanced_orchestration` | `advanced_goal_context_builder` | none | `read` | `stable_json` | none | docs/goal-operating-model.md, docs-site/command-reference.md |
| `gira goal new` | `advanced_orchestration` | `none` | none | `apply_mutation` | `stable_json` | creates a GitHub issue with Goal Mode operating sections; --dry-run previews payload, labels, and approval evidence | docs/goal-operating-model.md, docs-site/command-reference.md |
| `gira goal next` | `advanced_orchestration` | `none` | none | `read` | `stable_json` | none | docs/goal-operating-model.md, docs-site/command-reference.md |
| `gira goal plan` | `advanced_orchestration` | `goal_plan_bullet_planning_engine` | none | `apply_mutation` | `stable_json` | creates linked child tickets from reviewed goal-plan proposals when run with --apply; --dry-run previews the same plan | docs/goal-operating-model.md, docs-site/command-reference.md |
| `gira goal report` | `advanced_orchestration` | `none` | gira goal dossier | `read` | `stable_json` | none | docs/goal-operating-model.md, docs-site/goal-mode.md, docs-site/command-reference.md |
| `gira goal status` | `advanced_orchestration` | `none` | none | `read` | `stable_json` | none | docs/goal-operating-model.md, docs-site/command-reference.md |
| `gira jira doctor` | `assist` | `none` | none | `read` | `stable_json` | none | README.md, docs/jira-primary-provider.md, docs-site/jira-primary-provider.md |
| `gira jira export` | `supporting` | `none` | none | `apply_mutation` | `stable_json` | writes Jira-friendly export artifacts to the requested output path | README.md, docs/jira-primary-provider.md, docs-site/jira-primary-provider.md |
| `gira jira import` | `supporting` | `none` | none | `apply_mutation` | `stable_json` | creates GitHub issues from Jira import sources; --dry-run previews created and skipped issues | README.md, docs/jira-primary-provider.md, docs-site/jira-primary-provider.md |
| `gira jira init` | `supporting` | `none` | none | `apply_mutation` | `stable_json` | writes reviewed non-secret Jira provider config; --dry-run previews discovered config | README.md, docs/jira-primary-provider.md, docs-site/jira-primary-provider.md |
| `gira jira mirror` | `supporting` | `none` | none | `apply_mutation` | `stable_json` | creates or reuses a GitHub mirror issue; --dry-run previews mirror resolution | README.md, docs/jira-primary-provider.md, docs-site/jira-primary-provider.md |
| `gira jira transition` | `supporting` | `none` | none | `dry_run_mutation` | `stable_json` | plans Jira transition reachability only; adapters must not treat it as Jira mutation approval | README.md, docs/jira-primary-provider.md, docs-site/jira-primary-provider.md |
| `gira milestone assign` | `managed_delivery` | `none` | none | `apply_mutation` | `stable_json` | assigns selected issues to a milestone; --dry-run previews issue updates | docs-site/sprint-release.md, docs-site/ticket-workflow.md |
| `gira milestone list` | `managed_delivery` | `none` | none | `read` | `stable_json` | none | docs-site/sprint-release.md, docs-site/ticket-workflow.md |
| `gira milestone new` | `managed_delivery` | `none` | none | `apply_mutation` | `stable_json` | creates a GitHub milestone; --dry-run previews payload and target repo | docs-site/sprint-release.md, docs-site/ticket-workflow.md |
| `gira milestone plan` | `managed_delivery` | `none` | none | `apply_mutation` | `stable_json` | selects and assigns candidate tickets; --dry-run previews candidate set and mutations | docs-site/sprint-release.md, docs-site/ticket-workflow.md |
| `gira milestone status` | `managed_delivery` | `none` | none | `read` | `stable_json` | none | docs-site/sprint-release.md, docs-site/ticket-workflow.md |
| `gira ops limit` | `supporting` | `none` | none | `read` | `stable_json` | none | docs/github-api-limits.md, docs/workflow-cost-profiles.md, docs/command-surface-boundary.md, docs-site/api-limits.md, docs-site/cost-profiles.md, docs-site/command-surface.md, docs-site/command-reference.md |
| `gira pm accept` | `advanced_orchestration` | `none` | none | `apply_mutation` | `stable_json` | persists an evidence-mapped PM acceptance result and typed learning transition; dry-run rejects delivery proxies for outcome validation | docs/pm-operating-policy.md, docs/pm-skill.md, docs-site/command-reference.md |
| `gira pm bootstrap` | `advanced_orchestration` | `none` | none | `read` | `stable_json` | none | docs/pm-operating-policy.md, docs/v3-pm-harness-release-readiness.md, docs-site/command-reference.md |
| `gira pm compile` | `advanced_orchestration` | `none` | none | `read` | `stable_json` | none | docs/pm-operating-policy.md, docs/pm-skill.md, docs-site/command-reference.md |
| `gira pm conformance` | `advanced_orchestration` | `none` | none | `read` | `stable_json` | none | docs/v3-pm-harness-release-readiness.md, docs-site/command-reference.md |
| `gira pm context` | `advanced_orchestration` | `none` | none | `read` | `stable_json` | none | docs/pm-operating-policy.md, docs/pm-skill.md, docs-site/command-reference.md |
| `gira pm discovery` | `advanced_orchestration` | `none` | none | `read` | `stable_json` | none | docs/pm-operating-policy.md, docs/pm-skill.md, docs-site/command-reference.md |
| `gira pm measure` | `advanced_orchestration` | `none` | none | `read` | `stable_json` | none | docs/pm-operating-policy.md, docs/pm-skill.md, docs-site/command-reference.md |
| `gira pm observe` | `advanced_orchestration` | `none` | none | `read` | `stable_json` | none | docs/pm-operating-policy.md, docs/goal-operating-model.md, docs-site/command-reference.md |
| `gira pm qa` | `advanced_orchestration` | `none` | none | `read` | `stable_json` | none | docs/pm-skill.md, docs-site/command-reference.md |
| `gira pm record` | `advanced_orchestration` | `none` | none | `apply_mutation` | `stable_json` | appends a typed GitHub issue comment; --dry-run validates idempotency, privacy, and history resolution | docs/pm-operating-policy.md, docs/pm-skill.md, docs-site/command-reference.md |
| `gira pm replan` | `advanced_orchestration` | `none` | none | `apply_mutation` | `stable_json` | applies fingerprint-approved safe graph mutations and durable override/replan receipts; irreversible actions remain residual decisions | docs/pm-operating-policy.md, docs/goal-operating-model.md, docs-site/command-reference.md |
| `gira pm spec` | `advanced_orchestration` | `none` | none | `read` | `stable_json` | none | docs/pm-skill.md, docs-site/command-reference.md |
| `gira queue handoff` | `advanced_orchestration` | `advanced_workspace_selector` | none | `read` | `stable_json` | none | docs/workspace.md, docs/agent-handoff-queue.md, docs-site/agent-handoff-queue.md, docs-site/command-reference.md |
| `gira queue list` | `advanced_orchestration` | `none` | none | `read` | `stable_json` | none | docs/workspace.md, docs/agent-handoff-queue.md, docs-site/agent-handoff-queue.md, docs-site/command-reference.md |
| `gira queue next` | `advanced_orchestration` | `none` | none | `read` | `stable_json` | none | docs/workspace.md, docs/agent-handoff-queue.md, docs-site/agent-handoff-queue.md, docs-site/command-reference.md |
| `gira queue take` | `advanced_orchestration` | `none` | none | `apply_mutation` | `stable_json` | delegates to ticket start for a handoff-safe queue item; --dry-run previews selection, handoff readiness, and ticket start | docs/workspace.md, docs/agent-handoff-queue.md, docs-site/agent-handoff-queue.md, docs-site/command-reference.md |
| `gira report backlog-health` | `assist` | `none` | none | `read` | `stable_json` | none | README.md, docs-site/command-reference.md |
| `gira report changelog` | `assist` | `none` | none | `read` | `stable_json` | none | README.md, docs-site/command-reference.md |
| `gira report delivery-status` | `assist` | `none` | none | `read` | `stable_json` | none | README.md, docs-site/command-reference.md |
| `gira report milestone` | `assist` | `none` | none | `read` | `stable_json` | none | README.md, docs-site/command-reference.md |
| `gira report portfolio` | `assist` | `none` | none | `read` | `none` | none | README.md, docs/visual-portfolio-report.md, docs-site/command-reference.md |
| `gira report qa-checklist` | `assist` | `none` | none | `read` | `stable_json` | none | README.md, docs-site/command-reference.md |
| `gira report release-notes` | `assist` | `none` | none | `read` | `stable_json` | none | README.md, docs-site/command-reference.md |
| `gira report schedule` | `assist` | `none` | none | `read` | `stable_json` | none | README.md, docs-site/command-reference.md |
| `gira report wbs` | `assist` | `none` | none | `read` | `stable_json` | none | README.md, docs-site/command-reference.md |
| `gira report weekly` | `assist` | `none` | none | `read` | `stable_json` | none | README.md, docs-site/command-reference.md |
| `gira setup global` | `supporting` | `none` | none | `apply_mutation` | `stable_json` | writes global config and repo registry files; --dry-run previews file changes | README.md, docs/global-config-registry.md, docs-site/global-config.md, docs/workspace.md |
| `gira stats pulse` | `assist` | `none` | none | `read` | `stable_json` | none | docs/task-momentum-loop.md, docs/closure-funnel-stats.md, docs-site/task-momentum-loop.md, docs-site/closure-funnel-stats.md |
| `gira stats repo` | `assist` | `none` | none | `read` | `stable_json` | none | README.md, docs/closure-funnel-stats.md, docs-site/closure-funnel-stats.md |
| `gira stats workspace` | `assist` | `none` | none | `unsupported` | `planned` | none | docs/closure-funnel-stats.md, docs-site/closure-funnel-stats.md |
| `gira ticket checks` | `managed_delivery` | `none` | none | `read` | `stable_json` | none | README.md, docs-site/ticket-workflow.md, docs/dogfood.md |
| `gira ticket finish` | `managed_delivery` | `none` | none | `apply_mutation` | `stable_json` | may merge the linked PR, post receipts, normalize labels, and close the issue; Draft PR apply stops after ready transition, and --dry-run warns before merge or remote branch deletion | README.md, docs-site/ticket-workflow.md, docs/dogfood.md |
| `gira ticket handoff` | `managed_delivery` | `canonical_single_issue_agent_entry_point` | none | `read` | `stable_json` | none | docs-site/ticket-workflow.md, docs-site/command-reference.md, docs/dogfood.md |
| `gira ticket new` | `managed_delivery` | `none` | gira new, gira t new, gira t n | `apply_mutation` | `stable_json` | creates a GitHub issue, may set a native parent, and may optionally start it; --dry-run previews issue body, labels, and parent plan | README.md, docs-site/ticket-workflow.md, docs/dogfood.md |
| `gira ticket note` | `managed_delivery` | `none` | none | `apply_mutation` | `stable_json` | posts issue or PR comments; --dry-run previews resolved targets and rendered note | README.md, docs-site/ticket-workflow.md, docs/dogfood.md |
| `gira ticket parent` | `managed_delivery` | `none` | none | `apply_mutation` | `stable_json` | sets or clears a native GitHub sub-issue parent; read mode shows the current parent and mutation modes require --dry-run or --apply | README.md, docs/command-surface-boundary.md |
| `gira ticket pr` | `managed_delivery` | `none` | none | `apply_mutation` | `stable_json` | creates or validates a linked PR; --dry-run previews PR body and branch binding | README.md, docs-site/ticket-workflow.md, docs/dogfood.md |
| `gira ticket prompt` | `managed_delivery` | `none` | none | `read` | `stable_json` | none | README.md, docs-site/ticket-workflow.md, docs/dogfood.md |
| `gira ticket review` | `managed_delivery` | `none` | none | `read` | `stable_json` | none | docs-site/ticket-workflow.md, docs-site/command-reference.md, docs/dogfood.md |
| `gira ticket self-review` | `managed_delivery` | `none` | none | `apply_mutation` | `stable_json` | posts a self-review check note to the linked PR; --dry-run previews the rendered note and approval evidence | docs-site/ticket-workflow.md, docs-site/command-reference.md, docs/dogfood.md |
| `gira ticket start` | `managed_delivery` | `none` | gira start | `apply_mutation` | `stable_json` | applies a branch strategy, records lifecycle state, and moves the issue to in-progress; --dry-run previews readiness | README.md, docs-site/ticket-workflow.md, docs/dogfood.md |
| `gira ticket status` | `managed_delivery` | `none` | none | `read` | `stable_json` | none | README.md, docs-site/ticket-workflow.md, docs/dogfood.md |
| `gira ticket supersede` | `managed_delivery` | `none` | none | `apply_mutation` | `stable_json` | creates a replacement ticket, posts cross-links, and closes the original; --dry-run previews all planned mutations | README.md, docs-site/ticket-workflow.md, docs/dogfood.md |
| `gira ticket view` | `managed_delivery` | `none` | gira ticket show | `read` | `stable_json` | none | README.md, docs-site/ticket-workflow.md, docs/dogfood.md |
| `gira ticket wait` | `managed_delivery` | `none` | none | `read` | `stable_json` | none | README.md, docs-site/ticket-workflow.md, docs/dogfood.md |
| `gira workspace repos sync` | `managed_delivery` | `none` | none | `apply_mutation` | `stable_json` | updates workspace repo allowlist; --dry-run previews selected repositories | docs/global-config-registry.md, docs-site/global-config.md, docs/workspace.md |
| `gira workspace status` | `managed_delivery` | `none` | none | `read` | `stable_json` | none | README.md, docs/workspace.md, docs-site/global-config.md |
