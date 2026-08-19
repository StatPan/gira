package gira

import (
	"fmt"
	"sort"
	"strings"
)

type CommandSpec struct {
	Path        []string
	Summary     string
	Usage       string
	Flags       []FlagSpec
	Since       string
	Docs        []string
	GuideTopics []string
	GuideOrder  int
	Examples    []CommandExample
	Adapter     AdapterCommandCapability
}

type FlagSpec struct {
	Name    string
	Summary string
}

type CommandExample struct {
	Summary string
	Command string
}

type AdapterCapabilityClass string

const (
	AdapterCapabilityRead           AdapterCapabilityClass = "read"
	AdapterCapabilityDryRunMutation AdapterCapabilityClass = "dry_run_mutation"
	AdapterCapabilityApplyMutation  AdapterCapabilityClass = "apply_mutation"
	AdapterCapabilityUnsupported    AdapterCapabilityClass = "unsupported"

	CommandCapabilitySchemaVersion = "gira-command-capabilities/v1"
	JSONSupportStable              = "stable_json"
	JSONSupportPlanned             = "planned"
	JSONSupportNone                = "none"
)

type AdapterCommandCapability struct {
	Class            AdapterCapabilityClass
	MutationBoundary string
	JSONSupport      string
	Aliases          []string
	Notes            string
}

type CommandCapabilityReport struct {
	SchemaVersion string                   `json:"schema_version"`
	Commands      []CommandCapabilityEntry `json:"commands"`
}

type CommandCapabilityEntry struct {
	Path             []string               `json:"path"`
	Canonical        string                 `json:"canonical"`
	Summary          string                 `json:"summary"`
	Capability       AdapterCapabilityClass `json:"capability"`
	MutationBoundary string                 `json:"mutation_boundary,omitempty"`
	JSONSupport      string                 `json:"json_support"`
	Aliases          []string               `json:"aliases,omitempty"`
	Docs             []string               `json:"docs"`
	Since            string                 `json:"since,omitempty"`
	Notes            string                 `json:"notes,omitempty"`
}

func CoreCommandSpecs() []CommandSpec {
	specs := []CommandSpec{
		{
			Path:    []string{"setup", "global"},
			Summary: "Create or update the OS-user global config, workspace registry, and repo registry.",
			Usage:   "gira setup global [--repo OWNER/REPO] [--path .] [--workspace NAME] [--inbox-repo OWNER/REPO] [--mode global-only|hybrid] --dry-run|--apply",
			Since:   "v1.7.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Initial execution repo."},
				{Name: "--inbox-repo", Summary: "Backlog/intake repo for unassigned work."},
				{Name: "--mode", Summary: "Use global-only or hybrid repo-local contract mode."},
			},
			Docs:        []string{"README.md", "docs/global-config-registry.md", "docs-site/global-config.md", "docs/workspace.md"},
			GuideTopics: []string{"quickstart"},
			Examples: []CommandExample{
				{Summary: "Preview global-first setup", Command: "gira setup global --repo OWNER/app --path . --workspace personal --inbox-repo OWNER/backlog --mode global-only --dry-run"},
			},
		},
		{
			Path:    []string{"config", "storage"},
			Summary: "Show local storage roots, durability, privacy, and rebuild boundaries.",
			Usage:   "gira config storage [--repo OWNER/REPO] [--config-root PATH] [--json]",
			Since:   "v2.3.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target repo used to include selected repo registry and repo-local contract paths."},
				{Name: "--config-root", Summary: "Override global config root for diagnostics."},
				{Name: "--json", Summary: "Emit stable config-storage-report/v1 JSON."},
			},
			Docs:        []string{"docs/global-config-registry.md", "docs/state-model.md", "docs-site/global-config.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"quickstart", "agent"},
			Examples: []CommandExample{
				{Summary: "Inspect local storage boundaries", Command: "gira config storage --repo OWNER/app --json"},
			},
		},
		{
			Path:    []string{"ops", "limit"},
			Summary: "Show GitHub REST, GraphQL, search, secondary-limit, and workflow budget diagnostics.",
			Usage:   "gira ops limit [--repo OWNER/REPO] [--workflow NAME] [--json]",
			Since:   "v2.6.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--workflow", Summary: "Estimate safe remaining runs for a static workflow cost profile."},
				{Name: "--json", Summary: "Emit stable api-limit-report/v1 JSON."},
			},
			Docs:        []string{"docs/github-api-limits.md", "docs/workflow-cost-profiles.md", "docs/command-surface-boundary.md", "docs-site/api-limits.md", "docs-site/cost-profiles.md", "docs-site/command-surface.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"quickstart", "agent"},
			Examples: []CommandExample{
				{Summary: "Inspect current GitHub API budget", Command: "gira ops limit --repo OWNER/app"},
				{Summary: "Estimate remaining ticket lifecycle runs", Command: "gira ops limit --repo OWNER/app --workflow ticket-lifecycle"},
				{Summary: "Emit machine-readable budget diagnostics", Command: "gira ops limit --repo OWNER/app --json"},
			},
			Adapter: AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Diagnostic only; optional workflow estimates use static cost profiles and do not authorize mutations."},
		},
		{
			Path:    []string{"workspace", "repos", "sync"},
			Summary: "Discover GitHub owner/org repos and update a global workspace execution repo allowlist.",
			Usage:   "gira workspace repos sync [--owner OWNER] [--workspace NAME] --dry-run|--apply [--include-archived]",
			Since:   "v1.8.0",
			Flags: []FlagSpec{
				{Name: "--owner", Summary: "GitHub user or organization. Defaults to workspace.owner."},
				{Name: "--workspace", Summary: "Global workspace name. Defaults to global config default_workspace."},
				{Name: "--include-archived", Summary: "Include archived repositories."},
			},
			Docs:        []string{"docs/global-config-registry.md", "docs-site/global-config.md", "docs/workspace.md"},
			GuideTopics: []string{"quickstart"},
			Examples: []CommandExample{
				{Summary: "Preview owner repo sync", Command: "gira workspace repos sync --owner OWNER --workspace personal --dry-run"},
			},
		},
		{
			Path:    []string{"workspace", "status"},
			Summary: "Show inbox and execution repo state from a workspace config or global workspace registry.",
			Usage:   "gira workspace status [--config .gira/config.yaml] [--repo OWNER/REPO] [--limit N] [--active-only] [--cache-ttl 5m] [--refresh] [--json]",
			Since:   "v1.0.0",
			Flags: []FlagSpec{
				{Name: "--config", Summary: "Explicit workspace config path. Defaults to global registry, then .gira/config.yaml."},
				{Name: "--repo", Summary: "Narrow status to one or more execution repos."},
				{Name: "--limit", Summary: "Inspect only the first N selected execution repos."},
				{Name: "--active-only", Summary: "Show only execution repos with open work or an active milestone."},
				{Name: "--max-concurrency", Summary: "Bound concurrent repo status fetches. Default: 4."},
				{Name: "--cache-ttl", Summary: "Reuse recent per-repo status cache for this duration. Default: 5m."},
				{Name: "--refresh", Summary: "Ignore cached status and fetch fresh data."},
				{Name: "--json", Summary: "Emit stable JSON."},
			},
			Docs:        []string{"README.md", "docs/workspace.md", "docs-site/global-config.md"},
			GuideTopics: []string{"quickstart"},
			Examples: []CommandExample{
				{Summary: "Read the default workspace", Command: "gira workspace status"},
				{Summary: "Inspect a bounded subset", Command: "gira workspace status --limit 10 --active-only"},
			},
		},
		{
			Path:    []string{"queue", "list"},
			Summary: "List workspace queue items derived from workspace-queues/v1.",
			Usage:   "gira queue list [--config .gira/config.yaml] [--repo OWNER/REPO] [--queue ready|review|finish|blocked|failed|human] [--limit N] [--compact] [--json]",
			Since:   "v2.1.0",
			Flags: []FlagSpec{
				{Name: "--config", Summary: "Explicit workspace config path. Defaults to global registry, then .gira/config.yaml."},
				{Name: "--repo", Summary: "Narrow queue items to one or more execution repos."},
				{Name: "--queue", Summary: "Filter by queue alias: ready, review, finish, blocked, failed, or human."},
				{Name: "--limit", Summary: "Maximum queue items to print. Default: all."},
				{Name: "--compact", Summary: "Print compact text output."},
				{Name: "--json", Summary: "Emit stable queue-list/v1 JSON."},
			},
			Docs:        []string{"docs/workspace.md", "docs/agent-handoff-queue.md", "docs-site/agent-handoff-queue.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"agent", "quickstart"},
			Examples: []CommandExample{
				{Summary: "List agent-ready work", Command: "gira queue list --queue ready --json"},
			},
			Adapter: AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Derived from workspace-queues/v1; does not mutate GitHub or local run state."},
		},
		{
			Path:    []string{"queue", "next"},
			Summary: "Select the first agent-ready workspace queue item and print handoff and run-start commands.",
			Usage:   "gira queue next [--config .gira/config.yaml] [--repo OWNER/REPO] [--role implementer] [--profile default] [--compact] [--json]",
			Since:   "v2.1.0",
			Flags: []FlagSpec{
				{Name: "--config", Summary: "Explicit workspace config path. Defaults to global registry, then .gira/config.yaml."},
				{Name: "--repo", Summary: "Narrow selection to one or more execution repos."},
				{Name: "--role", Summary: "Handoff role: planner, implementer, or reviewer. Default: implementer."},
				{Name: "--profile", Summary: "Handoff profile: default or python. Default: default."},
				{Name: "--compact", Summary: "Print compact text output."},
				{Name: "--json", Summary: "Emit stable queue-next/v1 JSON."},
			},
			Docs:        []string{"docs/workspace.md", "docs/agent-handoff-queue.md", "docs-site/agent-handoff-queue.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"agent", "quickstart"},
			Examples: []CommandExample{
				{Summary: "Select the next LLM-ready item", Command: "gira queue next --json"},
			},
			Adapter: AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Read-only selection layer over workspace-queues/v1; reports ticket handoff and run start commands without executing them."},
		},
		{
			Path:    []string{"queue", "handoff"},
			Summary: "Select or inspect an agent-ready workspace queue item and embed the worker-handoff/v1 payload.",
			Usage:   "gira queue handoff [--config .gira/config.yaml] [--repo OWNER/REPO] [--ticket N] [--role implementer] [--profile default] [--compact] [--json]",
			Since:   "v2.1.0",
			Flags: []FlagSpec{
				{Name: "--config", Summary: "Explicit workspace config path. Defaults to global registry, then .gira/config.yaml."},
				{Name: "--repo", Summary: "Narrow selection to one execution repo, or select the explicit ticket repo."},
				{Name: "--ticket", Summary: "Explicit ticket number. Without it, handoff uses queue next selection."},
				{Name: "--role", Summary: "Handoff role: planner, implementer, or reviewer. Default: implementer."},
				{Name: "--profile", Summary: "Handoff profile: default or python. Default: default."},
				{Name: "--compact", Summary: "Print compact text output."},
				{Name: "--json", Summary: "Emit stable queue-handoff/v1 JSON with worker-handoff/v1 embedded."},
			},
			Docs:        []string{"docs/workspace.md", "docs/agent-handoff-queue.md", "docs-site/agent-handoff-queue.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"agent", "quickstart"},
			Examples: []CommandExample{
				{Summary: "Build a handoff packet for the next LLM-ready item", Command: "gira queue handoff --json"},
			},
			Adapter: AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Read-only queue selection and ticket handoff packet assembly; does not start branches, write run state, or mutate GitHub."},
		},
		{
			Path:    []string{"queue", "take"},
			Summary: "Start a handoff-safe queue item through the existing ticket start policy.",
			Usage:   "gira queue take [--config .gira/config.yaml] [--repo OWNER/REPO] [--ticket N] [--role implementer] [--profile default] [--compact] --dry-run|--apply [--json]",
			Since:   "v2.1.0",
			Flags: []FlagSpec{
				{Name: "--config", Summary: "Explicit workspace config path. Defaults to global registry, then .gira/config.yaml."},
				{Name: "--repo", Summary: "Narrow selection to one execution repo, or select the explicit ticket repo."},
				{Name: "--ticket", Summary: "Explicit ticket number. Without it, take uses queue next selection."},
				{Name: "--role", Summary: "Handoff role: planner, implementer, or reviewer. Default: implementer."},
				{Name: "--profile", Summary: "Handoff profile: default or python. Default: default."},
				{Name: "--compact", Summary: "Print compact text output."},
				{Name: "--dry-run", Summary: "Preview selection, worker handoff, and ticket start without mutation."},
				{Name: "--apply", Summary: "Start only a handoff-safe and worker-ready queue item."},
				{Name: "--json", Summary: "Emit stable queue-take/v1 JSON with worker-handoff/v1 and work-start-result/v1 embedded."},
			},
			Docs:        []string{"docs/workspace.md", "docs/agent-handoff-queue.md", "docs-site/agent-handoff-queue.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"agent", "quickstart"},
			Examples: []CommandExample{
				{Summary: "Preview taking the next safe queue item", Command: "gira queue take --dry-run --json"},
			},
			Adapter: AdapterCommandCapability{Class: AdapterCapabilityApplyMutation, MutationBoundary: "delegates to ticket start for a handoff-safe queue item; --dry-run previews selection, handoff readiness, and ticket start", JSONSupport: JSONSupportStable},
		},
		{
			Path:    []string{"dispatch", "goal"},
			Summary: "Build an official dispatch packet from a goal issue, goal handoff, and next safe child ticket worker handoff.",
			Usage:   "gira dispatch goal [GOAL] [--repo OWNER/REPO] [--role implementer] [--profile default] [--json|--compact-json|--prompt]",
			Since:   "v2.4.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--goal", Summary: "Goal issue number. Can also be numeric positional; inferred when omitted."},
				{Name: "--role", Summary: "Handoff role: planner, implementer, or reviewer. Default: implementer."},
				{Name: "--profile", Summary: "Handoff profile: default or python. Default: default."},
				{Name: "--json", Summary: "Emit stable dispatch-packet/v1 JSON."},
				{Name: "--compact-json", Summary: "Emit compact dispatch-compact/v1 JSON without full issue bodies or role packets."},
				{Name: "--prompt", Summary: "Emit a compact prompt for direct LLM handoff."},
				{Name: "--context-budget", Summary: "Maximum compact context size in characters. Default: 12000."},
			},
			Docs:        []string{"docs/dispatch-operating-model.md", "docs/dispatch-reflection.md", "docs/goal-operating-model.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"agent", "quickstart"},
			Examples: []CommandExample{
				{Summary: "Build a goal dispatch packet for an implementer", Command: "gira dispatch goal --repo OWNER/backlog --role implementer --json"},
				{Summary: "Build a compact LLM handoff prompt", Command: "gira dispatch goal --repo OWNER/backlog --prompt --context-budget 8000"},
			},
		},
		{
			Path: []string{"pm", "bootstrap"}, Summary: "Hydrate a bounded, resumable PM protocol session from canonical Goal state.", Usage: "gira pm bootstrap --repo OWNER/REPO --ticket N [--role human|ai] [--authority CAPABILITY] [--context-budget N] [--json]", Since: "v3.0.0",
			Flags: []FlagSpec{{Name: "--repo", Summary: "Target GitHub repo."}, {Name: "--ticket", Summary: "Goal issue holding canonical PM state."}, {Name: "--role", Summary: "Caller role: human or ai. Default: human."}, {Name: "--authority", Summary: "Explicit capability evidence; repeatable."}, {Name: "--context-budget", Summary: "Maximum bootstrap characters. Default: 6000."}, {Name: "--json", Summary: "Emit stable pm-bootstrap/v1 JSON."}},
			Docs:  []string{"docs/pm-operating-policy.md", "docs/v3-pm-harness-release-readiness.md", "docs-site/command-reference.md"}, Examples: []CommandExample{{Summary: "Resume an AI PM session without thread memory", Command: "gira pm bootstrap --repo OWNER/app --ticket 123 --role ai --authority issue:read --json"}}, Adapter: adapterRead(JSONSupportStable),
		},
		{
			Path:    []string{"pm", "compile"},
			Summary: "Compile product intent into deterministic pm-ir/v1 and actionable diagnostics.",
			Usage:   "gira pm compile [--intent TEXT|--from-file PATH|-] [--repo OWNER/REPO] [--goal N] [--json]",
			Since:   "v3.0.0",
			Flags: []FlagSpec{
				{Name: "--intent", Summary: "Raw product/development intent."},
				{Name: "--from-file", Summary: "Read raw intent from file, or '-' for stdin."},
				{Name: "--repo", Summary: "Optional target GitHub repo in OWNER/REPO format; required with --goal."},
				{Name: "--goal", Summary: "Optional GitHub Goal issue supplying explicit PM context."},
				{Name: "--json", Summary: "Emit the full stable pm-compile-report/v1 with pm-ir/v1 embedded."},
			},
			Docs:        []string{"docs/pm-operating-policy.md", "docs/pm-skill.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"quickstart"},
			Examples: []CommandExample{
				{Summary: "Compile local intent into compact diagnostics", Command: "gira pm compile --from-file request.md"},
				{Summary: "Include explicit Goal context and emit full IR", Command: "gira pm compile --repo OWNER/app --goal 123 --from-file request.md --json"},
			},
			Adapter: AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Read-only deterministic compilation; optional Goal context reads GitHub and no path mutates files or GitHub."},
		},
		{
			Path:    []string{"pm", "record"},
			Summary: "Append an idempotent typed record to a GitHub-native PM ledger.",
			Usage:   "gira pm record --repo OWNER/REPO --ticket N --id ID --kind KIND [--text TEXT|--from-file PATH|-] [--source REF] [--link RELATION=ID] --dry-run|--apply [--json]",
			Since:   "v3.0.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--ticket", Summary: "GitHub issue holding the PM ledger."},
				{Name: "--id", Summary: "Stable append-safe record ID."},
				{Name: "--kind", Summary: "Ledger kind, including outcome, opportunity, hypothesis, risk, experiment, and measurement."},
				{Name: "--text", Summary: "Record claim or statement."},
				{Name: "--from-file", Summary: "Read record text from file, or '-' for stdin."},
				{Name: "--source", Summary: "Inspectable source reference; repeatable."},
				{Name: "--actor-kind", Summary: "human, ai, system, or integration. Default: human."},
				{Name: "--status", Summary: "Optional kind-specific lifecycle status."},
				{Name: "--supersedes", Summary: "Prior record ID superseded by this record."},
				{Name: "--link", Summary: "Discovery relation=target record ID; repeatable."},
				{Name: "--goal-ref", Summary: "Linked Goal reference; repeatable."},
				{Name: "--task-profile", Summary: "Linked PM task profile; repeatable."},
				{Name: "--risk-type", Summary: "value, usability, feasibility, or viability."},
				{Name: "--evidence-strength", Summary: "anecdotal, qualitative, quantitative, or replicated."},
				{Name: "--confidence", Summary: "low, medium, or high; kept separate from evidence strength."},
				{Name: "--falsification-test", Summary: "Test that can falsify a hypothesis."},
				{Name: "--test-waiver", Summary: "Why a formal falsification test is disproportionate."},
				{Name: "--experiment-state", Summary: "planned, running, success, failure, inconclusive, or invalid."},
				{Name: "--conclusion", Summary: "validated, invalidated, inconclusive, or no_build learning."},
				{Name: "--outcome-state", Summary: "proposed, observing, achieved, not_achieved, or unknown."},
				{Name: "--signal", Summary: "Measurement signal name."},
				{Name: "--signal-kind", Summary: "leading, lagging, delivery, health, or guardrail."},
				{Name: "--evidence-type", Summary: "quantitative, qualitative, or limitation."},
				{Name: "--baseline", Summary: "Baseline value or observation."},
				{Name: "--baseline-definition", Summary: "Baseline population and calculation definition."},
				{Name: "--target", Summary: "Target value or qualitative condition."},
				{Name: "--target-direction", Summary: "increase, decrease, maintain, threshold, or qualitative."},
				{Name: "--observation-window", Summary: "Bounded observation window."},
				{Name: "--data-source", Summary: "Inspectable measurement source."},
				{Name: "--source-status", Summary: "available or unavailable."},
				{Name: "--owner", Summary: "Measurement decision owner."},
				{Name: "--decision-rule", Summary: "Action rule for observed evidence."},
				{Name: "--evaluation", Summary: "met, not_met, inconclusive, unavailable, stable, or regressed."},
				{Name: "--post-change-definition", Summary: "Post-change population and calculation definition."},
				{Name: "--qualitative-method", Summary: "Qualitative evidence method."},
				{Name: "--qualitative-sample", Summary: "Qualitative sample or context."},
				{Name: "--qualitative-limits", Summary: "Qualitative evidence limitations."},
				{Name: "--evidence-limitation", Summary: "Why outcome evidence is unavailable."},
				{Name: "--follow-up-ref", Summary: "Task resolving an evidence limitation."},
				{Name: "--at", Summary: "RFC3339 record time; defaults to current time."},
				{Name: "--dry-run", Summary: "Preview validation and append action without mutation."},
				{Name: "--apply", Summary: "Append the typed issue comment after validation."},
				{Name: "--json", Summary: "Emit stable pm-record-report/v1 JSON."},
			},
			Docs:        []string{"docs/pm-operating-policy.md", "docs/pm-skill.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"quickstart"},
			Examples: []CommandExample{
				{Summary: "Preview an evidence record", Command: "gira pm record --repo OWNER/app --ticket 123 --id evidence.setup.1 --kind evidence --text 'Five setup failures' --source log:run-5 --dry-run"},
				{Summary: "Append an accepted decision", Command: "gira pm record --repo OWNER/app --ticket 123 --id decision.output.1 --kind decision --status accepted --from-file decision.md --source issue:OWNER/app#123 --apply"},
			},
			Adapter: adapterApply("appends a typed GitHub issue comment; --dry-run validates idempotency, privacy, and history resolution", JSONSupportStable),
		},
		{
			Path:    []string{"pm", "context"},
			Summary: "Hydrate compact current PM state from typed and legacy GitHub issue evidence.",
			Usage:   "gira pm context --repo OWNER/REPO --ticket N [--context-budget N] [--json]",
			Since:   "v3.0.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--ticket", Summary: "GitHub issue holding the PM ledger."},
				{Name: "--context-budget", Summary: "Maximum compact context size in characters. Default: 6000."},
				{Name: "--json", Summary: "Emit full stable pm-context/v1 JSON."},
			},
			Docs:        []string{"docs/pm-operating-policy.md", "docs/pm-skill.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"quickstart"},
			Examples: []CommandExample{
				{Summary: "Hydrate bounded current PM state", Command: "gira pm context --repo OWNER/app --ticket 123"},
				{Summary: "Inspect full typed history", Command: "gira pm context --repo OWNER/app --ticket 123 --json"},
			},
			Adapter: adapterRead(JSONSupportStable),
		},
		{
			Path: []string{"pm", "measure"}, Summary: "Validate outcome measurement plans and evidence without mutation.", Usage: "gira pm measure --repo OWNER/REPO --ticket N [--context-budget N] [--json]", Since: "v3.0.0",
			Flags: []FlagSpec{{Name: "--repo", Summary: "Target GitHub repo."}, {Name: "--ticket", Summary: "Issue holding outcome and measurement records."}, {Name: "--context-budget", Summary: "Maximum compact context size. Default: 6000."}, {Name: "--json", Summary: "Emit full pm-measurement-report/v1 JSON."}},
			Docs:  []string{"docs/pm-operating-policy.md", "docs/pm-skill.md", "docs-site/command-reference.md"}, GuideTopics: []string{"quickstart"}, Examples: []CommandExample{{Summary: "Validate current outcome evidence", Command: "gira pm measure --repo OWNER/app --ticket 123"}, {Summary: "Inspect full measurement provenance", Command: "gira pm measure --repo OWNER/app --ticket 123 --json"}}, Adapter: adapterRead(JSONSupportStable),
		},
		{
			Path: []string{"pm", "observe"}, Summary: "Diagnose product-state changes and order bounded PM actions without mutation.", Usage: "gira pm observe --repo OWNER/REPO --ticket N [--json]", Since: "v3.0.0",
			Flags: []FlagSpec{{Name: "--repo", Summary: "Target GitHub repo."}, {Name: "--ticket", Summary: "Goal issue holding typed PM and work graph state."}, {Name: "--json", Summary: "Emit full pm-observe-report/v1 JSON with source reports."}},
			Docs:  []string{"docs/pm-operating-policy.md", "docs/goal-operating-model.md", "docs-site/command-reference.md"}, GuideTopics: []string{"agent", "quickstart"}, Examples: []CommandExample{{Summary: "Inspect the next bounded PM actions", Command: "gira pm observe --repo OWNER/app --ticket 123"}, {Summary: "Inspect source diagnoses and recommendation change", Command: "gira pm observe --repo OWNER/app --ticket 123 --json"}}, Adapter: adapterRead(JSONSupportStable),
		},
		{
			Path: []string{"pm", "replan"}, Summary: "Preview or apply fingerprinted, capability-aware Goal graph mutations.", Usage: "gira pm replan --repo OWNER/REPO --ticket N --dry-run|--apply [--expect-plan ID] [--override ACTION --rationale TEXT] [--json]", Since: "v3.0.0",
			Flags: []FlagSpec{{Name: "--repo", Summary: "Target GitHub repo."}, {Name: "--ticket", Summary: "Goal issue holding typed PM and work graph state."}, {Name: "--dry-run", Summary: "Preview every graph mutation and residual authority action."}, {Name: "--apply", Summary: "Apply only safe mutations from an unchanged plan."}, {Name: "--expect-plan", Summary: "Approved pmr-* dry-run fingerprint required by apply."}, {Name: "--override", Summary: "Explicit human override, including unblock:#N."}, {Name: "--rationale", Summary: "Durable product rationale required with an override."}, {Name: "--json", Summary: "Emit stable pm-replan-report/v1 JSON."}},
			Docs:  []string{"docs/pm-operating-policy.md", "docs/goal-operating-model.md", "docs-site/command-reference.md"}, GuideTopics: []string{"agent", "quickstart"}, Examples: []CommandExample{{Summary: "Preview evidence-triggered mutations", Command: "gira pm replan --repo OWNER/app --ticket 123 --dry-run --json"}, {Summary: "Apply an unchanged replan", Command: "gira pm replan --repo OWNER/app --ticket 123 --apply --expect-plan pmr-..."}}, Adapter: adapterApply("applies fingerprint-approved safe graph mutations and durable override/replan receipts; irreversible actions remain residual decisions", JSONSupportStable),
		},
		{
			Path: []string{"pm", "accept"}, Summary: "Validate and persist source-linked delivery acceptance and product outcome validation.", Usage: "gira pm accept --repo OWNER/REPO --ticket N --from-file RESULT.json --dry-run|--apply [--json]", Since: "v3.0.0",
			Flags: []FlagSpec{{Name: "--repo", Summary: "Target GitHub repo."}, {Name: "--ticket", Summary: "Issue receiving the acceptance result and learning transition."}, {Name: "--from-file", Summary: "pm-acceptance-result/v1 JSON path, or - for stdin."}, {Name: "--dry-run", Summary: "Validate evidence mappings and transitions without persistence."}, {Name: "--apply", Summary: "Persist the verdict and typed ledger transition idempotently."}, {Name: "--json", Summary: "Emit stable pm-acceptance-report/v1 JSON."}},
			Docs:  []string{"docs/pm-operating-policy.md", "docs/pm-skill.md", "docs-site/command-reference.md"}, GuideTopics: []string{"agent", "quickstart"}, Examples: []CommandExample{{Summary: "Validate a PM verdict", Command: "gira pm accept --repo OWNER/app --ticket 123 --from-file acceptance.json --dry-run --json"}, {Summary: "Persist verdict and learning transition", Command: "gira pm accept --repo OWNER/app --ticket 123 --from-file acceptance.json --apply"}}, Adapter: adapterApply("persists an evidence-mapped PM acceptance result and typed learning transition; dry-run rejects delivery proxies for outcome validation", JSONSupportStable),
		},
		{
			Path:    []string{"pm", "discovery"},
			Summary: "Trace product outcomes through opportunities, hypotheses, experiments, learning, and decisions.",
			Usage:   "gira pm discovery --repo OWNER/REPO --ticket N [--context-budget N] [--json]",
			Since:   "v3.0.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--ticket", Summary: "GitHub issue holding the PM ledger."},
				{Name: "--context-budget", Summary: "Maximum compact context size in characters. Default: 6000."},
				{Name: "--json", Summary: "Emit full stable pm-discovery-graph/v1 JSON."},
			},
			Docs:        []string{"docs/pm-operating-policy.md", "docs/pm-skill.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"quickstart"},
			Examples: []CommandExample{
				{Summary: "Inspect a bounded opportunity-to-outcome graph", Command: "gira pm discovery --repo OWNER/app --ticket 123"},
				{Summary: "Inspect the complete trace and diagnostics", Command: "gira pm discovery --repo OWNER/app --ticket 123 --json"},
			},
			Adapter: adapterRead(JSONSupportStable),
		},
		{
			Path:    []string{"pm", "spec"},
			Summary: "Render a compact profile-aware PM packet.",
			Usage:   "gira pm spec [--profile PROFILE] [INPUT] [--json]",
			Since:   "v2.5.0",
			Flags: []FlagSpec{
				{Name: "--title", Summary: "Task title; defaults to the first non-empty intent line."},
				{Name: "--repo", Summary: "Optional target GitHub repo in OWNER/REPO format."},
				{Name: "--intent", Summary: "Raw product/development intent."},
				{Name: "--from-file", Summary: "Read raw intent from file, or '-' for stdin."},
				{Name: "--profile", Summary: "discovery, decision, experiment, delivery, rollout, measurement, documentation, or legacy. Default: delivery."},
				{Name: "--context-ref", Summary: "Stable parent premise or policy reference; repeatable."},
				{Name: "--worker-mode", Summary: "Suggested worker mode override; defaults by profile."},
				{Name: "--json", Summary: "Emit stable gira-pm-task-packet/v2 JSON; legacy profile emits v1."},
			},
			Docs:        []string{"docs/pm-skill.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"agent", "quickstart"},
			Examples: []CommandExample{
				{Summary: "Render a compact delivery packet", Command: "gira pm spec --profile delivery --context-ref issue:OWNER/app#100 --repo OWNER/app --from-file request.md > pm-task.md"},
				{Summary: "Render the legacy universal packet", Command: "gira pm spec --profile legacy --repo OWNER/app --from-file - > pm-task.md"},
				{Summary: "Create a ticket from a rendered packet", Command: "gira ticket new --repo OWNER/app --title \"TITLE\" --body-file pm-task.md --type task --dry-run"},
			},
			Adapter: AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Local rendering only; does not call GitHub or mutate files."},
		},
		{
			Path:    []string{"pm", "qa"},
			Summary: "Render a PM acceptance QA prompt from task-local PM state and PR evidence.",
			Usage:   "gira pm qa --repo OWNER/REPO --ticket N [--pr N] [--diff-summary] [--include-diff] [--json]",
			Since:   "v2.5.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--ticket", Summary: "Ticket number."},
				{Name: "--issue", Summary: "Compatibility alias for --ticket."},
				{Name: "--pr", Summary: "Explicit PR number."},
				{Name: "--diff-summary", Summary: "Include changed files and diff stat."},
				{Name: "--include-diff", Summary: "Include full diff when used with --diff-summary."},
				{Name: "--json", Summary: "Emit stable gira-pm-qa/v1 JSON with prompt embedded."},
			},
			Docs:        []string{"docs/pm-skill.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"agent", "quickstart"},
			Examples: []CommandExample{
				{Summary: "Render PM acceptance QA for a ticket PR", Command: "gira pm qa --repo OWNER/app --ticket 123 --diff-summary"},
			},
			Adapter: AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Reads GitHub issue and PR context; does not mutate GitHub."},
		},
		{
			Path: []string{"pm", "conformance"}, Summary: "Evaluate PM protocol compliance separately from semantic answer quality.", Usage: "gira pm conformance [--from-file RUN.json|-] [--json]", Since: "v3.0.0",
			Flags: []FlagSpec{{Name: "--from-file", Summary: "One pm-conformance-run/v1 object or array; built-in human and AI fixtures are the default."}, {Name: "--json", Summary: "Emit stable pm-conformance-report/v1 JSON."}},
			Docs:  []string{"docs/v3-pm-harness-release-readiness.md", "docs-site/command-reference.md"}, Examples: []CommandExample{{Summary: "Run built-in human and two-host AI conformance", Command: "gira pm conformance --json"}, {Summary: "Evaluate a recorded host run", Command: "gira pm conformance --from-file host-run.json --json"}}, Adapter: adapterRead(JSONSupportStable),
		},
		{
			Path:    []string{"completion"},
			Summary: "Generate shell completion scripts and cache-first dynamic candidates.",
			Usage:   "gira completion bash|zsh|fish; gira completion candidates repo|ticket|label|milestone",
			Since:   "v2.1.0",
			Flags: []FlagSpec{
				{Name: "bash", Summary: "Print Bash completion script."},
				{Name: "zsh", Summary: "Print Zsh completion script."},
				{Name: "fish", Summary: "Print Fish completion script."},
				{Name: "candidates", Summary: "Print local dynamic candidates from the repo registry and workspace status cache."},
			},
			Docs:        []string{"README.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"quickstart"},
			Examples: []CommandExample{
				{Summary: "Install Bash completion locally", Command: "gira completion bash > ~/.local/share/bash-completion/completions/gira"},
				{Summary: "Preview Fish completion", Command: "gira completion fish"},
				{Summary: "Inspect cached label candidates", Command: "gira completion candidates label --repo OWNER/REPO --prefix status"},
			},
		},
		{
			Path:    []string{"feature", "list"},
			Summary: "List optional issue-backed feature or capability records.",
			Usage:   "gira feature list [--repo OWNER/REPO] [--limit N] [--json]",
			Since:   "v1.17.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--limit", Summary: "Max issues to inspect. Default: 1000."},
				{Name: "--json", Summary: "Emit stable feature-map-list/v1 JSON."},
			},
			Docs:        []string{"docs/feature-map.md", "docs-site/feature-map.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"agent", "ticket"},
			Examples: []CommandExample{
				{Summary: "List feature records", Command: "gira feat list --repo OWNER/backlog"},
			},
		},
		{
			Path:    []string{"feature", "check"},
			Summary: "Validate optional feature map records and work links without mutating GitHub.",
			Usage:   "gira feature check [--repo OWNER/REPO] [--limit N] [--json]",
			Since:   "v1.17.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--limit", Summary: "Max issues to inspect. Default: 1000."},
				{Name: "--json", Summary: "Emit stable feature-map-check/v1 JSON."},
			},
			Docs:        []string{"docs/feature-map.md", "docs-site/feature-map.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"agent", "ticket"},
			Examples: []CommandExample{
				{Summary: "Check feature map health", Command: "gira feat check --repo OWNER/backlog"},
			},
		},
		{
			Path:    []string{"feature", "for"},
			Summary: "Show which feature or capability a work issue is linked to.",
			Usage:   "gira feature for ISSUE [--repo OWNER/REPO] [--limit N] [--json]",
			Since:   "v1.17.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--issue", Summary: "Work issue number. Can also be numeric positional."},
				{Name: "--limit", Summary: "Max issues to inspect. Default: 1000."},
				{Name: "--json", Summary: "Emit stable feature-map-for/v1 JSON."},
			},
			Docs:        []string{"docs/feature-map.md", "docs-site/feature-map.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"agent", "ticket"},
			Examples: []CommandExample{
				{Summary: "Inspect one work issue", Command: "gira feat for 123 --repo OWNER/app"},
			},
		},
		{
			Path:    []string{"goal", "new"},
			Summary: "Create a Goal Mode issue with objective, scope, autonomy, quality, stop, and child-ticket planning sections.",
			Usage:   "gira goal new \"Title\" --dry-run|--apply [--repo OWNER/REPO] [--objective TEXT] [--scope TEXT] [--json]",
			Since:   "v2.4.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--title", Summary: "Goal title. Can also be positional."},
				{Name: "--objective", Summary: "Durable goal outcome. Defaults to the title."},
				{Name: "--direction", Summary: "Strategic guidance, priorities, and tradeoffs."},
				{Name: "--scope", Summary: "Included work, target repos, milestones, and explicit non-goals."},
				{Name: "--autonomy", Summary: "Agent lane and permission policy."},
				{Name: "--decomposition", Summary: "Semicolon-separated child planning notes."},
				{Name: "--quality-bar", Summary: "Semicolon-separated verification, review, docs, or release evidence requirements."},
				{Name: "--stop-condition", Summary: "Semicolon-separated conditions that require human input."},
				{Name: "--type", Summary: "Goal issue type label: epic or goal. Default: epic."},
				{Name: "--priority", Summary: "Priority label: p0, p1, p2, or p3."},
				{Name: "--label", Summary: "Additional existing repo label. Repeatable or comma-separated."},
				{Name: "--body", Summary: "Full goal issue body. Overrides structured fields."},
				{Name: "--body-file", Summary: "Read full goal issue body from a file or - for stdin."},
				{Name: "--milestone", Summary: "Milestone title."},
				{Name: "--dry-run", Summary: "Preview issue payload and labels without mutation."},
				{Name: "--apply", Summary: "Create the goal issue."},
				{Name: "--json", Summary: "Emit stable goal-new-report/v1 JSON."},
			},
			Docs:        []string{"docs/goal-operating-model.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"agent", "ticket"},
			Examples: []CommandExample{
				{Summary: "Preview a new goal", Command: "gira goal new \"Ship Goal Mode\" --repo OWNER/app --objective \"Make goal tracking executable\" --scope \"CLI goal commands\" --decomposition \"Add goal new;Update docs\" --dry-run --json"},
				{Summary: "Create a reviewed goal issue", Command: "gira goal new \"Ship Goal Mode\" --repo OWNER/app --body-file goal.md --apply"},
			},
		},
		{
			Path:    []string{"goal", "status"},
			Summary: "Summarize a goal issue, child ticket graph, blockers, and next safe action.",
			Usage:   "gira goal status [GOAL] [--repo OWNER/REPO] [--json]",
			Since:   "v1.17.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--goal", Summary: "Goal issue number. Can also be numeric positional; inferred when omitted."},
				{Name: "--json", Summary: "Emit stable goal-status/v1 JSON."},
			},
			Docs:        []string{"docs/goal-operating-model.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"agent", "ticket"},
			Examples: []CommandExample{
				{Summary: "Inspect goal graph status", Command: "gira goal status 521 --repo OWNER/app --json"},
			},
		},
		{
			Path:    []string{"goal", "report"},
			Summary: "Build a visible report for one goal from stable Goal Mode state. Alias: gira goal dossier.",
			Usage:   "gira goal report [GOAL] [--repo OWNER/REPO] [--view operator|human|ai|stakeholder|audit] [--json|--html --output PATH]",
			Since:   "v2.1.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--goal", Summary: "Goal issue number. Can also be numeric positional; inferred when omitted."},
				{Name: "--view", Summary: "Derived PM view: operator, human, ai, stakeholder, or audit. Default: operator."},
				{Name: "--json", Summary: "Emit stable goal-dossier/v1 JSON."},
				{Name: "--html", Summary: "Write a static local HTML report."},
				{Name: "--output", Summary: "Output path for --html."},
			},
			Docs:        []string{"docs/goal-operating-model.md", "docs-site/goal-mode.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"agent", "ticket"},
			Examples: []CommandExample{
				{Summary: "Export a bounded AI PM hydration view", Command: "gira goal report 521 --repo OWNER/app --view ai --json"},
				{Summary: "Write a local goal report page", Command: "gira goal report 521 --repo OWNER/app --html --output out/gira/goal-521.html"},
			},
		},
		{
			Path:    []string{"goal", "plan"},
			Summary: "Propose or create same-repo or target-repo child ticket packets from a goal issue.",
			Usage:   "gira goal plan [GOAL] --dry-run|--apply [--repo OWNER/REPO] [--json|--compact-json] [--expect-plan ID]",
			Since:   "v1.17.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--goal", Summary: "Goal issue number. Can also be numeric positional; inferred when omitted."},
				{Name: "--dry-run", Summary: "Preview proposed child tickets, including target_repo, without mutation."},
				{Name: "--apply", Summary: "Create reviewed child tickets in their target repos from the proposed plan."},
				{Name: "--compact-json", Summary: "Emit compact goal-plan-compact/v1 JSON; compact apply requires --expect-plan from dry-run."},
				{Name: "--expect-plan", Summary: "Required dry-run plan ID for --compact-json --apply."},
				{Name: "--json", Summary: "Emit stable goal-plan/v1 JSON."},
			},
			Docs:        []string{"docs/goal-operating-model.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"agent", "ticket"},
			Examples: []CommandExample{
				{Summary: "Preview child ticket plan", Command: "gira goal plan 521 --repo OWNER/app --dry-run --json"},
				{Summary: "Create planned child tickets", Command: "gira goal plan 521 --repo OWNER/app --apply --json"},
			},
		},
		{
			Path: []string{"goal", "graph"}, Summary: "Compile PM intent and discovery state into a typed, verifiable Goal work graph.", Usage: "gira goal graph [GOAL] [--dry-run|--apply --expect-plan ID] [--repo OWNER/REPO] [--json|--compact-json]", Since: "v3.0.0",
			Flags: []FlagSpec{{Name: "--repo", Summary: "Target GitHub repo."}, {Name: "--goal", Summary: "Goal issue number; positional is supported."}, {Name: "--dry-run", Summary: "Preview fingerprinted lowering without mutation."}, {Name: "--apply", Summary: "Lower create/supersede actions and post an idempotent receipt."}, {Name: "--expect-plan", Summary: "Required approved dry-run pm-work-graph fingerprint for apply."}, {Name: "--json", Summary: "Emit full pm-work-graph-report/v1."}, {Name: "--compact-json", Summary: "Emit body-free pm-work-graph-compact/v1."}},
			Docs:  []string{"docs/goal-operating-model.md", "docs/pm-operating-policy.md", "docs-site/command-reference.md"}, GuideTopics: []string{"agent", "quickstart"}, Examples: []CommandExample{{Summary: "Compile a typed work graph", Command: "gira goal graph 521 --repo OWNER/app --compact-json"}, {Summary: "Preview lowering", Command: "gira goal graph 521 --repo OWNER/app --dry-run --compact-json"}, {Summary: "Apply an unchanged plan", Command: "gira goal graph 521 --repo OWNER/app --apply --expect-plan pwg-... --compact-json"}}, Adapter: adapterApply("compiles read-only by default; --apply lowers fingerprint-approved child actions and posts a receipt", JSONSupportStable),
		},
		{
			Path:    []string{"goal", "next"},
			Summary: "Select the next safe child ticket for a goal or explain why work must stop.",
			Usage:   "gira goal next [GOAL] [--repo OWNER/REPO] [--json]",
			Since:   "v1.17.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--goal", Summary: "Goal issue number. Can also be numeric positional; inferred when omitted."},
				{Name: "--json", Summary: "Emit stable goal-next/v1 JSON."},
			},
			Docs:        []string{"docs/goal-operating-model.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"agent", "ticket"},
			Examples: []CommandExample{
				{Summary: "Choose the next goal child", Command: "gira goal next 521 --repo OWNER/app --json"},
			},
		},
		{
			Path:    []string{"goal", "handoff"},
			Summary: "Build a goal-level LLM handoff that includes goal context and the next safe child ticket worker packet.",
			Usage:   "gira goal handoff [GOAL] [--repo OWNER/REPO] [--role implementer] [--profile default] [--json]",
			Since:   "v2.4.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--goal", Summary: "Goal issue number. Can also be numeric positional; inferred when omitted."},
				{Name: "--role", Summary: "Handoff role: planner, implementer, or reviewer. Default: implementer."},
				{Name: "--profile", Summary: "Handoff profile: default or python. Default: default."},
				{Name: "--json", Summary: "Emit stable goal-handoff/v1 JSON with worker-handoff/v1 embedded when a child is selected."},
			},
			Docs:        []string{"docs/goal-operating-model.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"agent", "ticket"},
			Examples: []CommandExample{
				{Summary: "Build an implementer handoff for the next goal child", Command: "gira goal handoff 521 --repo OWNER/app --role implementer --json"},
			},
		},
		{
			Path:    []string{"goal", "finish"},
			Summary: "Preview goal finish readiness, then post receipts and close ready goals or preserve human-review handoffs.",
			Usage:   "gira goal finish [GOAL] --dry-run|--apply [--repo OWNER/REPO] [--terminal done|human_review|blocked|superseded|abandoned] [--json]",
			Since:   "v1.17.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--goal", Summary: "Goal issue number. Can also be numeric positional; inferred when omitted."},
				{Name: "--dry-run", Summary: "Preview readiness and receipt without mutation."},
				{Name: "--apply", Summary: "Apply an explicit done close or human_review handoff mutation."},
				{Name: "--terminal", Summary: "Explicit terminal recommendation override for apply: done, human_review, blocked, superseded, or abandoned."},
				{Name: "--json", Summary: "Emit stable goal-finish-readiness/v1 JSON."},
			},
			Docs:        []string{"docs/goal-operating-model.md", "docs-site/command-reference.md"},
			GuideTopics: []string{"agent", "ticket"},
			Examples: []CommandExample{
				{Summary: "Preview goal finish evidence", Command: "gira goal finish 521 --repo OWNER/app --dry-run --json"},
			},
		},
		{
			Path:    []string{"report", "weekly"},
			Summary: "Build a weekly PM cockpit report with deterministic KPIs and top exceptions.",
			Usage:   "gira report weekly [--repo OWNER/REPO] [--format text|md|json|csv|html|bundle] [--output PATH]",
			Since:   "v2.5.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--format", Summary: "Output format: text, md, json, csv, html, or bundle."},
				{Name: "--output", Summary: "Output path for md/csv/html, or output root for bundle."},
				{Name: "--json", Summary: "Emit stable weekly-report/v1alpha1 JSON."},
				{Name: "--md", Summary: "Emit Markdown report."},
				{Name: "--csv", Summary: "Emit CSV rows."},
				{Name: "--html", Summary: "Emit a static local HTML report."},
			},
			Adapter: AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Read-only GitHub issue, milestone, and PR inspection; optional output writes local report artifacts only."},
			Docs:    []string{"README.md", "docs-site/command-reference.md"},
			Examples: []CommandExample{
				{Summary: "Render weekly PM cockpit markdown", Command: "gira report weekly --repo OWNER/app --format md"},
				{Summary: "Write a weekly report bundle", Command: "gira report weekly --repo OWNER/app --format bundle --output out/weekly"},
			},
		},
		{
			Path:    []string{"report", "portfolio"},
			Summary: "Render a self-contained local HTML overview of milestone progress, dated gates, and blocked or review-waiting queues.",
			Usage:   "gira report portfolio [--repo OWNER/REPO ...] [--milestone TITLE ...] [--since YYYY-MM-DD] [--until YYYY-MM-DD] --output PATH",
			Since:   "v2.6.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Repository filter; repeat to include multiple repositories."},
				{Name: "--milestone", Summary: "Exact milestone-title filter; repeat to include multiple milestones."},
				{Name: "--since", Summary: "Inclusive timeline and queue window start in YYYY-MM-DD."},
				{Name: "--until", Summary: "Inclusive timeline and queue window end in YYYY-MM-DD."},
				{Name: "--output", Summary: "Required local HTML output path; generation never publishes, serves, or opens it."},
			},
			Adapter: AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportNone, Notes: "Reads stable GitHub/Gira contracts and writes only the explicitly selected local HTML path; never publishes or opens the artifact."},
			Docs:    []string{"README.md", "docs/visual-portfolio-report.md", "docs-site/command-reference.md"},
			Examples: []CommandExample{
				{Summary: "Render a bounded local portfolio view", Command: "gira report portfolio --repo OWNER/app --milestone v2.1.0 --since 2026-07-01 --until 2026-09-30 --output out/portfolio.html"},
			},
		},
		{
			Path:    []string{"report", "wbs"},
			Summary: "Build structural or execution-focused WBS reports from GitHub epics, issues, milestones, and roadmap dates.",
			Usage:   "gira report wbs [--repo OWNER/REPO] [--state open|closed|all] [--mode structural|execution] [--scenario current|one-month] [--format text|json|csv|html|bundle] [--output PATH]",
			Since:   "v2.5.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--state", Summary: "Issue state filter: open, closed, or all. Default: open."},
				{Name: "--mode", Summary: "Report model: structural preserves hierarchy-first WBS; execution emits actionable planning rows."},
				{Name: "--scenario", Summary: "Planning scenario for execution mode: current or one-month."},
				{Name: "--format", Summary: "Output format: text, json, csv, html, or bundle."},
				{Name: "--output", Summary: "Output path for csv/html, or output root for bundle."},
				{Name: "--json", Summary: "Emit stable wbs-report/v1alpha1 JSON."},
				{Name: "--csv", Summary: "Emit WBS CSV rows."},
				{Name: "--html", Summary: "Emit a static local HTML report."},
			},
			Adapter: AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Read-only GitHub issue, milestone, and roadmap inspection; optional output writes local report artifacts only."},
			Docs:    []string{"README.md", "docs-site/command-reference.md"},
			Examples: []CommandExample{
				{Summary: "Render a terminal WBS summary", Command: "gira report wbs --repo OWNER/app"},
				{Summary: "Render execution WBS rows for Sheets", Command: "gira report wbs --repo OWNER/app --mode execution --format csv"},
				{Summary: "Write a shareable WBS report bundle", Command: "gira report wbs --repo OWNER/app --format bundle --output out/wbs"},
			},
		},
		{
			Path:    []string{"report", "schedule"},
			Summary: "Build a schedule-oriented execution report sorted by date and week bucket.",
			Usage:   "gira report schedule [--repo OWNER/REPO] [--state open|closed|all] [--by week] [--scenario current|one-month] [--format text|json|csv] [--output PATH]",
			Since:   "v2.5.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--state", Summary: "Issue state filter: open, closed, or all. Default: open."},
				{Name: "--by", Summary: "Schedule grouping. Currently supports week."},
				{Name: "--scenario", Summary: "Planning scenario: current or one-month."},
				{Name: "--format", Summary: "Output format: text, json, or csv."},
				{Name: "--output", Summary: "Output path for csv."},
				{Name: "--json", Summary: "Emit stable execution-report/v1alpha1 JSON."},
				{Name: "--csv", Summary: "Emit execution rows as CSV."},
			},
			Adapter: AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Read-only schedule projection over WBS issue, milestone, and roadmap evidence; optional output writes local report artifacts only."},
			Docs:    []string{"README.md", "docs-site/command-reference.md"},
			Examples: []CommandExample{
				{Summary: "Render weekly schedule rows for Sheets", Command: "gira report schedule --repo OWNER/app --by week --format csv"},
				{Summary: "Compare a compressed one-month planning scenario", Command: "gira report schedule --repo OWNER/app --scenario one-month --format json"},
			},
		},
		{
			Path:    []string{"report", "release-notes"},
			Summary: "Build human-readable release notes from milestone issues and merged PR closing evidence.",
			Usage:   "gira report release-notes --repo OWNER/REPO --milestone TITLE [--format text|md|json|csv|html|bundle] [--output PATH]",
			Since:   "v2.5.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--milestone", Summary: "Release milestone title to include."},
				{Name: "--format", Summary: "Output format: text, md, json, csv, html, or bundle."},
				{Name: "--output", Summary: "Output path for md/csv/html, or output root for bundle."},
				{Name: "--json", Summary: "Emit stable release-notes-report/v1alpha1 JSON."},
				{Name: "--md", Summary: "Emit Markdown release notes."},
				{Name: "--csv", Summary: "Emit release item CSV rows."},
				{Name: "--html", Summary: "Emit a static local HTML report."},
			},
			Adapter: AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Read-only GitHub issue, merged PR, milestone, and closing-reference inspection; optional output writes local report artifacts only."},
			Docs:    []string{"README.md", "docs-site/command-reference.md"},
			Examples: []CommandExample{
				{Summary: "Render release notes markdown", Command: "gira report release-notes --repo OWNER/app --milestone v2.1.0 --format md"},
				{Summary: "Write a release notes bundle", Command: "gira report release-notes --repo OWNER/app --milestone v2.1.0 --format bundle --output out/release-notes"},
			},
		},
		{
			Path:    []string{"report", "changelog"},
			Summary: "Build a changelog document from the same milestone and merged PR evidence as release notes.",
			Usage:   "gira report changelog --repo OWNER/REPO --milestone TITLE [--format text|md|json|csv|html|bundle] [--output PATH]",
			Since:   "v2.5.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--milestone", Summary: "Release milestone title to include."},
				{Name: "--format", Summary: "Output format: text, md, json, csv, html, or bundle."},
				{Name: "--output", Summary: "Output path for md/csv/html, or output root for bundle."},
				{Name: "--json", Summary: "Emit stable release-notes-report/v1alpha1 JSON."},
				{Name: "--md", Summary: "Emit Markdown changelog."},
				{Name: "--csv", Summary: "Emit changelog CSV rows."},
				{Name: "--html", Summary: "Emit a static local HTML report."},
			},
			Adapter: AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Read-only GitHub issue, merged PR, milestone, and closing-reference inspection; optional output writes local report artifacts only."},
			Docs:    []string{"README.md", "docs-site/command-reference.md"},
			Examples: []CommandExample{
				{Summary: "Render changelog markdown", Command: "gira report changelog --repo OWNER/app --milestone v2.1.0 --format md"},
			},
		},
		{
			Path:    []string{"report", "milestone"},
			Summary: "Build a milestone progress report from GitHub milestone and issue evidence.",
			Usage:   "gira report milestone --repo OWNER/REPO --milestone TITLE [--format text|md|json|csv|html|bundle] [--output PATH]",
			Since:   "v2.5.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--milestone", Summary: "Milestone title to inspect."},
				{Name: "--format", Summary: "Output format: text, md, json, csv, html, or bundle."},
				{Name: "--output", Summary: "Output path for md/csv/html, or output root for bundle."},
				{Name: "--json", Summary: "Emit stable project-report/v1alpha1 JSON."},
				{Name: "--md", Summary: "Emit Markdown report."},
				{Name: "--csv", Summary: "Emit CSV rows."},
				{Name: "--html", Summary: "Emit a static local HTML report."},
			},
			Adapter: AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Read-only GitHub issue, milestone, and PR inspection; optional output writes local report artifacts only."},
			Docs:    []string{"README.md", "docs-site/command-reference.md"},
			Examples: []CommandExample{
				{Summary: "Render milestone progress", Command: "gira report milestone --repo OWNER/app --milestone v2.1.0 --format md"},
			},
		},
		{
			Path:    []string{"report", "backlog-health"},
			Summary: "Build a backlog health report from open issue status, age, labels, and planning evidence.",
			Usage:   "gira report backlog-health [--repo OWNER/REPO] [--format text|md|json|csv|html|bundle] [--output PATH]",
			Since:   "v2.5.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--format", Summary: "Output format: text, md, json, csv, html, or bundle."},
				{Name: "--output", Summary: "Output path for md/csv/html, or output root for bundle."},
				{Name: "--json", Summary: "Emit stable project-report/v1alpha1 JSON."},
				{Name: "--md", Summary: "Emit Markdown report."},
				{Name: "--csv", Summary: "Emit CSV rows."},
				{Name: "--html", Summary: "Emit a static local HTML report."},
			},
			Adapter: AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Read-only GitHub issue, milestone, and PR inspection; optional output writes local report artifacts only."},
			Docs:    []string{"README.md", "docs-site/command-reference.md"},
			Examples: []CommandExample{
				{Summary: "Render backlog health", Command: "gira report backlog-health --repo OWNER/app"},
			},
		},
		{
			Path:    []string{"report", "delivery-status"},
			Summary: "Build a delivery status report from milestone progress, blockers, and PR readiness evidence.",
			Usage:   "gira report delivery-status [--repo OWNER/REPO] [--format text|md|json|csv|html|bundle] [--output PATH]",
			Since:   "v2.5.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--format", Summary: "Output format: text, md, json, csv, html, or bundle."},
				{Name: "--output", Summary: "Output path for md/csv/html, or output root for bundle."},
				{Name: "--json", Summary: "Emit stable project-report/v1alpha1 JSON."},
				{Name: "--md", Summary: "Emit Markdown report."},
				{Name: "--csv", Summary: "Emit CSV rows."},
				{Name: "--html", Summary: "Emit a static local HTML report."},
			},
			Adapter: AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Read-only GitHub issue, milestone, and PR inspection; optional output writes local report artifacts only."},
			Docs:    []string{"README.md", "docs-site/command-reference.md"},
			Examples: []CommandExample{
				{Summary: "Render delivery status", Command: "gira report delivery-status --repo OWNER/app --format md"},
			},
		},
		{
			Path:    []string{"report", "qa-checklist"},
			Summary: "Build a QA checklist report from issue labels, open PR checks, review state, and closure-link evidence.",
			Usage:   "gira report qa-checklist [--repo OWNER/REPO] [--milestone TITLE] [--format text|md|json|csv|html|bundle] [--output PATH]",
			Since:   "v2.5.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--milestone", Summary: "Optional milestone title to scope issue checks."},
				{Name: "--format", Summary: "Output format: text, md, json, csv, html, or bundle."},
				{Name: "--output", Summary: "Output path for md/csv/html, or output root for bundle."},
				{Name: "--json", Summary: "Emit stable project-report/v1alpha1 JSON."},
				{Name: "--md", Summary: "Emit Markdown report."},
				{Name: "--csv", Summary: "Emit CSV rows."},
				{Name: "--html", Summary: "Emit a static local HTML report."},
			},
			Adapter: AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Read-only GitHub issue, milestone, and PR inspection; optional output writes local report artifacts only."},
			Docs:    []string{"README.md", "docs-site/command-reference.md"},
			Examples: []CommandExample{
				{Summary: "Render milestone QA checklist", Command: "gira report qa-checklist --repo OWNER/app --milestone v2.1.0 --format md"},
			},
		},
		{
			Path:    []string{"stats", "repo"},
			Summary: "Show a read-only Closure Funnel report for one GitHub repo.",
			Usage:   "gira stats repo [OWNER/REPO] [--repo OWNER/REPO] [--since 90d] [--stale-days 14] [--limit 100] [--json]",
			Since:   "v1.12.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo. May also be positional."},
				{Name: "--since", Summary: "Reporting window such as 90d or YYYY-MM-DD. Default: 90d."},
				{Name: "--stale-days", Summary: "Count open issues and PRs stale after this many days. Default: 14."},
				{Name: "--limit", Summary: "Max GitHub rows per query. Default: 100."},
				{Name: "--json", Summary: "Emit stable JSON for automation."},
			},
			Docs:        []string{"README.md", "docs/closure-funnel-stats.md", "docs-site/closure-funnel-stats.md"},
			GuideTopics: []string{"stats"},
			Examples: []CommandExample{
				{Summary: "Render the default repo report", Command: "gira stats repo --repo OWNER/app --since 90d"},
			},
		},
		{
			Path:    []string{"stats", "pulse"},
			Summary: "Show a read-only recent workflow pulse for one GitHub repo.",
			Usage:   "gira stats pulse [OWNER/REPO] [--repo OWNER/REPO] [--since 7d] [--limit 100] [--json]",
			Since:   "v2.2.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo. May also be positional."},
				{Name: "--since", Summary: "Reporting window such as 7d or YYYY-MM-DD. Default: 7d."},
				{Name: "--limit", Summary: "Max GitHub rows per query. Default: 100."},
				{Name: "--json", Summary: "Emit stable pulse-report/v1alpha1 JSON."},
			},
			Docs:        []string{"docs/task-momentum-loop.md", "docs/closure-funnel-stats.md", "docs-site/task-momentum-loop.md", "docs-site/closure-funnel-stats.md"},
			GuideTopics: []string{"stats"},
			Examples: []CommandExample{
				{Summary: "Render the recent repo pulse", Command: "gira stats pulse --repo OWNER/app --since 7d"},
			},
			Adapter: AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Read-only pulse-report/v1alpha1 over GitHub/Gira issue and PR evidence; does not score people or mutate state."},
		},
		{
			Path:    []string{"stats", "workspace"},
			Summary: "Planned multi-repo Closure Funnel rollup for a configured workspace.",
			Usage:   "gira stats workspace [--since 90d]",
			Since:   "planned",
			Flags: []FlagSpec{
				{Name: "--since", Summary: "Reporting window such as 90d or YYYY-MM-DD."},
			},
			Docs:        []string{"docs/closure-funnel-stats.md", "docs-site/closure-funnel-stats.md"},
			GuideTopics: []string{"stats"},
			Examples: []CommandExample{
				{Summary: "Planned workspace rollup", Command: "gira stats workspace --since 90d"},
			},
		},
		{
			Path:    []string{"milestone", "new"},
			Summary: "Preview and create a GitHub milestone as a first-class Gira work batch.",
			Usage:   "gira milestone new \"TITLE\" [--repo OWNER/REPO] [--description TEXT] [--due-on YYYY-MM-DD] --dry-run|--apply [--json]",
			Since:   "v1.16.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--description", Summary: "Milestone description."},
				{Name: "--due-on", Summary: "Milestone due date or timestamp."},
				{Name: "--dry-run", Summary: "Preview milestone creation."},
				{Name: "--apply", Summary: "Create the milestone."},
				{Name: "--json", Summary: "Emit stable JSON."},
			},
			Docs:        []string{"docs-site/sprint-release.md", "docs-site/ticket-workflow.md"},
			GuideTopics: []string{"quickstart", "ticket"},
			Examples: []CommandExample{
				{Summary: "Preview a milestone", Command: "gira milestone new \"2.0 Alpha - State-Aware Ticket Runtime\" --dry-run"},
			},
		},
		{
			Path:    []string{"milestone", "list"},
			Summary: "List GitHub milestones with Gira progress fields.",
			Usage:   "gira milestone list [--repo OWNER/REPO] [--state open|closed|all] [--json]",
			Since:   "v1.16.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--state", Summary: "Milestone state: open, closed, or all. Default: open."},
				{Name: "--json", Summary: "Emit stable JSON."},
			},
			Docs:        []string{"docs-site/sprint-release.md", "docs-site/ticket-workflow.md"},
			GuideTopics: []string{"quickstart", "ticket"},
			Examples: []CommandExample{
				{Summary: "List open milestones", Command: "gira milestone list --state open"},
			},
		},
		{
			Path:    []string{"milestone", "status"},
			Summary: "Summarize child ticket state for one milestone work batch.",
			Usage:   "gira milestone status MILESTONE [--repo OWNER/REPO] [--json]",
			Since:   "v1.16.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--json", Summary: "Emit stable JSON."},
			},
			Docs:        []string{"docs-site/sprint-release.md", "docs-site/ticket-workflow.md"},
			GuideTopics: []string{"quickstart", "ticket"},
			Examples: []CommandExample{
				{Summary: "Inspect a milestone", Command: "gira milestone status \"2.0 Alpha - State-Aware Ticket Runtime\""},
			},
		},
		{
			Path:    []string{"milestone", "assign"},
			Summary: "Bulk attach selected tickets to a milestone through dry-run/apply.",
			Usage:   "gira milestone assign MILESTONE --tickets 1,2,3 [--repo OWNER/REPO] --dry-run|--apply [--json]",
			Since:   "v1.16.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--tickets", Summary: "Comma-separated ticket numbers."},
				{Name: "--dry-run", Summary: "Preview assignment."},
				{Name: "--apply", Summary: "Assign selected tickets."},
				{Name: "--json", Summary: "Emit stable JSON."},
			},
			Docs:        []string{"docs-site/sprint-release.md", "docs-site/ticket-workflow.md"},
			GuideTopics: []string{"quickstart", "ticket"},
			Examples: []CommandExample{
				{Summary: "Preview bulk assignment", Command: "gira milestone assign \"2.0 Alpha\" --tickets 12,13 --dry-run"},
			},
		},
		{
			Path:    []string{"milestone", "plan"},
			Summary: "Select candidate tickets by labels and assign them to a milestone.",
			Usage:   "gira milestone plan MILESTONE [--repo OWNER/REPO] [--label LABEL] [--state open|closed|all] [--limit N] --dry-run|--apply [--json]",
			Since:   "v1.16.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--label", Summary: "Candidate label filter. Defaults to status:ready."},
				{Name: "--state", Summary: "Ticket state: open, closed, or all. Default: open."},
				{Name: "--limit", Summary: "Maximum candidate tickets. Default: 20."},
				{Name: "--dry-run", Summary: "Preview assignment plan."},
				{Name: "--apply", Summary: "Assign selected tickets."},
				{Name: "--json", Summary: "Emit stable JSON."},
			},
			Docs:        []string{"docs-site/sprint-release.md", "docs-site/ticket-workflow.md"},
			GuideTopics: []string{"quickstart", "ticket"},
			Examples: []CommandExample{
				{Summary: "Plan from ready tickets", Command: "gira milestone plan \"2.0 Alpha\" --label status:ready --dry-run"},
			},
		},
		{
			Path:    []string{"jira", "init"},
			Summary: "Discover a Jira project and write reviewed non-secret provider config.",
			Usage:   "gira jira init --repo OWNER/REPO --api-base URL --project KEY --dry-run|--apply [--config-root PATH] [--overwrite] [--json]",
			Since:   "v1.13.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--api-base", Summary: "Jira site base URL, such as https://example.atlassian.net."},
				{Name: "--project", Summary: "Jira project key to discover."},
				{Name: "--config-root", Summary: "Override the global Gira config root."},
				{Name: "--overwrite", Summary: "Replace an existing providers.jira block after review."},
				{Name: "--dry-run", Summary: "Preview provider discovery and config payload without writing files."},
				{Name: "--apply", Summary: "Write the reviewed non-secret provider config."},
				{Name: "--json", Summary: "Emit stable JSON."},
			},
			Docs:        []string{"README.md", "docs/jira-primary-provider.md", "docs-site/jira-primary-provider.md"},
			GuideTopics: []string{"jira"},
			GuideOrder:  10,
			Examples: []CommandExample{
				{Summary: "Preview Jira provider setup", Command: "gira jira init --repo OWNER/app --api-base https://example.atlassian.net --project ABC --dry-run"},
			},
		},
		{
			Path:    []string{"jira", "doctor"},
			Summary: "Diagnose Jira-primary provider compatibility without mutating Jira or GitHub.",
			Usage:   "gira jira doctor --repo OWNER/REPO [--project KEY] [--api-base URL] [--sample-key JIRA-123] [--config-root PATH] [--json]",
			Since:   "v1.13.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--project", Summary: "Override the configured Jira project key for diagnostics."},
				{Name: "--api-base", Summary: "Override the configured Jira API base URL."},
				{Name: "--sample-key", Summary: "Representative Jira issue key for transition and required-field diagnostics."},
				{Name: "--config-root", Summary: "Override the global Gira config root."},
				{Name: "--json", Summary: "Emit stable JSON."},
			},
			Docs:        []string{"README.md", "docs/jira-primary-provider.md", "docs-site/jira-primary-provider.md"},
			GuideTopics: []string{"jira"},
			GuideOrder:  15,
			Examples: []CommandExample{
				{Summary: "Diagnose a configured Jira-primary repo", Command: "gira jira doctor --repo OWNER/app --sample-key ABC-123"},
			},
		},
		{
			Path:    []string{"jira", "mirror"},
			Summary: "Create or reuse a GitHub mirror issue for one Jira key.",
			Usage:   "gira jira mirror JIRA-123 --repo OWNER/REPO --dry-run|--apply [--api-base URL] [--config-root PATH] [--json]",
			Since:   "v1.13.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--api-base", Summary: "Override the configured Jira API base URL."},
				{Name: "--config-root", Summary: "Override the global Gira config root."},
				{Name: "--dry-run", Summary: "Preview mirror issue creation or reuse."},
				{Name: "--apply", Summary: "Create the GitHub mirror issue when missing."},
				{Name: "--json", Summary: "Emit stable JSON."},
			},
			Docs:        []string{"README.md", "docs/jira-primary-provider.md", "docs-site/jira-primary-provider.md"},
			GuideTopics: []string{"jira"},
			GuideOrder:  21,
			Examples: []CommandExample{
				{Summary: "Preview one Jira mirror", Command: "gira jira mirror ABC-123 --repo OWNER/app --dry-run"},
			},
		},
		{
			Path:    []string{"jira", "transition"},
			Summary: "Plan one Jira status transition without mutation.",
			Usage:   "gira jira transition JIRA-123 --repo OWNER/REPO --to ready|in_progress|review|done --dry-run [--api-base URL] [--config-root PATH] [--json]",
			Since:   "v1.13.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--to", Summary: "Target Gira status mapped through providers.jira.status_map."},
				{Name: "--api-base", Summary: "Override the configured Jira API base URL."},
				{Name: "--config-root", Summary: "Override the global Gira config root."},
				{Name: "--dry-run", Summary: "Required; transition planning is read-only."},
				{Name: "--json", Summary: "Emit stable JSON."},
			},
			Docs:        []string{"README.md", "docs/jira-primary-provider.md", "docs-site/jira-primary-provider.md"},
			GuideTopics: []string{"jira"},
			GuideOrder:  30,
			Examples: []CommandExample{
				{Summary: "Inspect whether Done is reachable", Command: "gira jira transition ABC-123 --repo OWNER/app --to done --dry-run"},
			},
		},
		{
			Path:    []string{"jira", "import"},
			Summary: "Import Jira CSV/JSON or read-only Jira API issues into GitHub issues.",
			Usage:   "gira jira import --repo OWNER/REPO --source PATH --dry-run|--apply [--json]\ngira jira import --repo OWNER/REPO --api-base URL --project KEY --dry-run|--apply [--json]",
			Since:   "v1.13.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--source", Summary: "CSV or JSON import source path."},
				{Name: "--api-base", Summary: "Jira API base URL for read-only API import."},
				{Name: "--project", Summary: "Jira project key for read-only API import."},
				{Name: "--dry-run", Summary: "Preview issue creates without mutation."},
				{Name: "--apply", Summary: "Create GitHub issues for non-duplicate Jira items."},
				{Name: "--json", Summary: "Emit stable JSON."},
			},
			Docs:        []string{"README.md", "docs/jira-primary-provider.md", "docs-site/jira-primary-provider.md"},
			GuideTopics: []string{"jira"},
			GuideOrder:  40,
			Examples: []CommandExample{
				{Summary: "Preview a Jira CSV import", Command: "gira jira import --repo OWNER/app --source jira.csv --dry-run"},
			},
		},
		{
			Path:    []string{"jira", "export"},
			Summary: "Export GitHub issue state into Jira-friendly JSON and CSV artifacts.",
			Usage:   "gira jira export --repo OWNER/REPO --output PATH [--json]",
			Since:   "v1.13.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--output", Summary: "Output directory for export artifacts."},
				{Name: "--json", Summary: "Emit stable JSON."},
			},
			Docs:        []string{"README.md", "docs/jira-primary-provider.md", "docs-site/jira-primary-provider.md"},
			GuideTopics: []string{"jira"},
			GuideOrder:  50,
			Examples: []CommandExample{
				{Summary: "Export GitHub issue state", Command: "gira jira export --repo OWNER/app --output out/jira"},
			},
		},
		{
			Path:    []string{"ticket", "new"},
			Summary: "Create a repo-bound executable GitHub issue with structured or full Markdown body input.",
			Usage:   "gira ticket new \"Title\" --dry-run|--apply [--parent N] [--body TEXT|--body-file PATH|-] [--release-impact MODE] [--start]",
			Since:   "v1.0.0",
			Flags: []FlagSpec{
				{Name: "--goal", Summary: "Structured issue goal."},
				{Name: "--acceptance", Summary: "Semicolon-separated acceptance criteria."},
				{Name: "--type", Summary: "Ticket type: epic, story, task, bug, spike, or chore."},
				{Name: "--priority", Summary: "Priority: p0, p1, p2, or p3."},
				{Name: "--parent", Summary: "Native GitHub parent issue for the created ticket."},
				{Name: "--label", Summary: "Additional repo label that must already exist."},
				{Name: "--body", Summary: "Full issue body."},
				{Name: "--body-file", Summary: "Read full issue body from file or stdin with -."},
				{Name: "--release-impact", Summary: "Release impact: user-facing, internal, or exempt."},
				{Name: "--release-impact-reason", Summary: "Reason required for exempt."},
				{Name: "--start", Summary: "Start the created ticket after apply."},
			},
			Docs:        []string{"README.md", "docs-site/ticket-workflow.md", "docs/dogfood.md"},
			GuideTopics: []string{"quickstart", "ticket", "agent"},
			GuideOrder:  10,
			Examples: []CommandExample{
				{Summary: "Preview structured ticket", Command: "gira ticket new \"TITLE\" --goal \"GOAL\" --acceptance \"a;b;c\" --dry-run"},
				{Summary: "Preview full Markdown packet", Command: "gira ticket new --title \"TITLE\" --body-file issue.md --dry-run"},
			},
		},
		{
			Path:    []string{"ticket", "parent"},
			Summary: "Show, set, or clear a native GitHub sub-issue parent without adding a separate link command family.",
			Usage:   "gira ticket parent TICKET [--set PARENT|--clear] [--dry-run|--apply] [--repo OWNER/REPO] [--json]",
			Since:   "v1.17.0",
			Flags: []FlagSpec{
				{Name: "--set", Summary: "Set the native GitHub parent issue."},
				{Name: "--clear", Summary: "Clear the native GitHub parent issue."},
				{Name: "--dry-run", Summary: "Preview the parent mutation."},
				{Name: "--apply", Summary: "Apply the parent mutation."},
			},
			Docs:        []string{"README.md", "docs/command-surface-boundary.md"},
			GuideTopics: []string{"ticket", "agent"},
			GuideOrder:  11,
			Examples: []CommandExample{
				{Summary: "Preview parent link", Command: "gira ticket parent 42 --set 10 --dry-run"},
				{Summary: "Show current parent", Command: "gira ticket parent 42"},
			},
		},
		{
			Path:        []string{"ticket", "view"},
			Summary:     "Show a Gira operating card for the ticket, linked PR, blockers, and next action. Alias: gira ticket show.",
			Usage:       "gira ticket view|show [TICKET] [--repo OWNER/REPO] [--json]",
			Since:       "v1.12.0",
			Docs:        []string{"README.md", "docs-site/ticket-workflow.md", "docs/dogfood.md"},
			GuideTopics: []string{"ticket", "agent"},
			GuideOrder:  15,
			Examples: []CommandExample{
				{Summary: "Inspect current branch ticket context", Command: "gira ticket view"},
				{Summary: "Inspect an explicit ticket with the show alias", Command: "gira ticket show 42 --repo OWNER/app"},
			},
		},
		{
			Path:    []string{"ticket", "prompt"},
			Summary: "Render a stateless planner, implementer, or reviewer prompt from ticket context.",
			Usage:   "gira ticket prompt [TICKET] [planner|implementer|reviewer] [--role planner|implementer|reviewer] [--profile default|python] [--repo OWNER/REPO] [--pr N] [--json]",
			Since:   "v1.14.0",
			Flags: []FlagSpec{
				{Name: "--role", Summary: "Prompt role: planner, implementer, or reviewer."},
				{Name: "--profile", Summary: "Prompt profile: default or python. Default: default."},
				{Name: "--pr", Summary: "Optional PR number for reviewer prompt context."},
				{Name: "--json", Summary: "Emit stable JSON including the rendered prompt."},
			},
			Docs:        []string{"README.md", "docs-site/ticket-workflow.md", "docs/dogfood.md"},
			GuideTopics: []string{"ticket", "agent"},
			GuideOrder:  18,
			Examples: []CommandExample{
				{Summary: "Render an implementation worker prompt for the current branch ticket", Command: "gira ticket prompt implementer --profile python"},
				{Summary: "Render a reviewer prompt with an explicit PR override", Command: "gira ticket prompt reviewer --pr 77"},
			},
		},
		{
			Path:    []string{"ticket", "handoff"},
			Summary: "Compile a worker-neutral handoff packet from ticket context.",
			Usage:   "gira ticket handoff [TICKET] [planner|implementer|reviewer] [--role planner|implementer|reviewer] [--profile default|python] [--repo OWNER/REPO] [--json]",
			Since:   "v1.17.0",
			Flags: []FlagSpec{
				{Name: "--role", Summary: "Handoff role: planner, implementer, or reviewer. Default: implementer."},
				{Name: "--profile", Summary: "Handoff profile: default or python. Default: default."},
				{Name: "--json", Summary: "Emit stable worker-handoff/v1 JSON."},
			},
			Docs:        []string{"docs-site/ticket-workflow.md", "docs-site/command-reference.md", "docs/dogfood.md"},
			GuideTopics: []string{"ticket", "agent"},
			GuideOrder:  19,
			Examples: []CommandExample{
				{Summary: "Compile an implementer handoff packet for the current branch ticket", Command: "gira ticket handoff --json"},
				{Summary: "Compile a reviewer handoff packet for the current branch ticket", Command: "gira ticket handoff reviewer --json"},
			},
		},
		{
			Path:    []string{"ticket", "review"},
			Summary: "Render a reviewer packet from current ticket and linked PR state.",
			Usage:   "gira ticket review [TICKET] [--repo OWNER/REPO] [--pr N] [--diff-summary] [--include-diff] [--json|--html --output PATH]",
			Since:   "v1.15.0",
			Flags: []FlagSpec{
				{Name: "--pr", Summary: "Optional PR number override for reviewer packet context."},
				{Name: "--diff-summary", Summary: "Include changed files, diff stat, hunk headers, acceptance mapping candidates, and risk hints."},
				{Name: "--include-diff", Summary: "Include the full PR diff. Output can be long and must be requested explicitly."},
				{Name: "--json", Summary: "Emit stable JSON including issue, PR, evidence, repo guidance, verdict schema, and prompt fields."},
				{Name: "--html", Summary: "Write a static local HTML review packet."},
				{Name: "--output", Summary: "Output path for --html."},
			},
			Docs:        []string{"docs-site/ticket-workflow.md", "docs-site/command-reference.md", "docs/dogfood.md"},
			GuideTopics: []string{"ticket", "agent"},
			GuideOrder:  20,
			Examples: []CommandExample{
				{Summary: "Render reviewer packet for current branch ticket", Command: "gira ticket review --diff-summary"},
				{Summary: "Render reviewer packet with an explicit PR override", Command: "gira ticket review --ticket 42 --pr 77 --json"},
				{Summary: "Write a local review packet page", Command: "gira ticket review 42 --repo OWNER/app --diff-summary --html --output out/gira/review-42.html"},
			},
		},
		{
			Path:    []string{"ticket", "self-review"},
			Summary: "Post a self-review check note for the current branch ticket and linked PR.",
			Usage:   "gira ticket self-review [TICKET] [--repo OWNER/REPO] [--pr N] [--diff-summary] --dry-run|--apply [--json]",
			Since:   "v1.18.0",
			Flags: []FlagSpec{
				{Name: "--pr", Summary: "Optional PR number override for self-review context."},
				{Name: "--diff-summary", Summary: "Include compact PR diff summary in the check note. Default: true."},
				{Name: "--dry-run", Summary: "Preview the self-review PR note without posting."},
				{Name: "--apply", Summary: "Post the self-review check note to the linked PR."},
				{Name: "--json", Summary: "Emit stable ticket-self-review-report/v1 JSON."},
			},
			Docs:        []string{"docs-site/ticket-workflow.md", "docs-site/command-reference.md", "docs/dogfood.md"},
			GuideTopics: []string{"ticket", "agent"},
			GuideOrder:  32,
			Examples: []CommandExample{
				{Summary: "Preview current branch self-review note", Command: "gira ticket self-review --diff-summary --dry-run"},
				{Summary: "Post current branch self-review note", Command: "gira ticket self-review --diff-summary --apply"},
			},
		},
		{
			Path:    []string{"ticket", "start"},
			Summary: "Start a ready issue with an explicit branch strategy.",
			Usage:   "gira ticket start [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--base BRANCH] [--create|--current|--adopt BRANCH]",
			Since:   "v1.0.0",
			Flags: []FlagSpec{
				{Name: "--base", Summary: "Explicit lifecycle base branch override recorded on the ticket."},
				{Name: "--create", Summary: "Create the policy-suggested work branch."},
				{Name: "--current", Summary: "Bind the current branch without checkout or push."},
				{Name: "--adopt", Summary: "Bind an existing local or origin branch without checkout or push."},
				{Name: "--json", Summary: "Emit the stable ticket-status/v1 JSON contract with issue, branch, PR, checks, review, evidence, blockers, warnings, and next action."},
			},
			Docs:        []string{"README.md", "docs-site/ticket-workflow.md", "docs/dogfood.md"},
			GuideTopics: []string{"ticket", "agent"},
			GuideOrder:  20,
			Examples: []CommandExample{
				{Summary: "Create the suggested branch for a ready issue", Command: "gira ticket start 42 --create --apply"},
			},
		},
		{
			Path:        []string{"ticket", "pr"},
			Summary:     "Create or validate a linked PR with required issue closing text.",
			Usage:       "gira ticket pr [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--draft]",
			Since:       "v1.0.0",
			Docs:        []string{"README.md", "docs-site/ticket-workflow.md", "docs/dogfood.md"},
			GuideTopics: []string{"ticket", "agent"},
			GuideOrder:  30,
			Examples: []CommandExample{
				{Summary: "Open a draft PR", Command: "gira ticket pr --apply --draft"},
			},
		},
		{
			Path:    []string{"ticket", "note"},
			Summary: "Post a structured context note to the issue, linked PR, or both.",
			Usage:   "gira ticket note [TICKET] \"BODY\" --dry-run|--apply [--repo OWNER/REPO] [--kind progress|blocker|decision|handoff|summary|check] [--target auto|issue|pr|both]",
			Since:   "v1.12.0",
			Flags: []FlagSpec{
				{Name: "--kind", Summary: "Template kind for the note. Default: progress."},
				{Name: "--target", Summary: "Comment target: auto, issue, pr, or both. Default: auto."},
				{Name: "--body", Summary: "Explicit note body."},
				{Name: "--body-file", Summary: "Read note body from file or stdin with -."},
				{Name: "--dry-run", Summary: "Preview target resolution and rendered note without posting."},
				{Name: "--apply", Summary: "Post the rendered note."},
			},
			Docs:        []string{"README.md", "docs-site/ticket-workflow.md", "docs/dogfood.md"},
			GuideTopics: []string{"ticket", "agent"},
			GuideOrder:  35,
			Examples: []CommandExample{
				{Summary: "Preview a progress note", Command: "gira ticket note \"Implemented parser path\" --dry-run"},
				{Summary: "Post a blocker to both issue and PR", Command: "gira ticket note --kind blocker --target both --body-file note.md --apply"},
			},
		},
		{
			Path:    []string{"ticket", "supersede"},
			Summary: "Close a ticket as superseded and create a linked replacement ticket.",
			Usage:   "gira ticket supersede [TICKET] --replacement-title TITLE --body-file PATH|- --dry-run|--apply [--repo OWNER/REPO] [--close-draft-pr]",
			Since:   "v1.12.0",
			Flags: []FlagSpec{
				{Name: "--replacement-title", Summary: "Title for the replacement issue."},
				{Name: "--body", Summary: "Replacement issue body."},
				{Name: "--body-file", Summary: "Read replacement issue body from file or stdin with -."},
				{Name: "--label", Summary: "Additional replacement issue label."},
				{Name: "--milestone", Summary: "Override replacement issue milestone."},
				{Name: "--close-draft-pr", Summary: "Close a linked draft PR when superseding."},
				{Name: "--dry-run", Summary: "Preview all planned mutations."},
				{Name: "--apply", Summary: "Create the replacement, cross-link notes, status update, and close the original."},
			},
			Docs:        []string{"README.md", "docs-site/ticket-workflow.md", "docs/dogfood.md"},
			GuideTopics: []string{"ticket", "agent"},
			GuideOrder:  38,
			Examples: []CommandExample{
				{Summary: "Preview a replacement ticket", Command: "gira ticket supersede 64 --replacement-title \"Define release gate\" --body-file replacement.md --dry-run"},
			},
		},
		{
			Path:        []string{"ticket", "checks"},
			Summary:     "Show linked PR checks, review blockers, and next action.",
			Usage:       "gira ticket checks [TICKET] [--repo OWNER/REPO] [--json]",
			Since:       "v1.0.0",
			Docs:        []string{"README.md", "docs-site/ticket-workflow.md", "docs/dogfood.md"},
			GuideTopics: []string{"ticket", "agent"},
			GuideOrder:  40,
			Examples: []CommandExample{
				{Summary: "Inspect PR readiness", Command: "gira ticket checks"},
			},
		},
		{
			Path:        []string{"ticket", "wait"},
			Summary:     "Wait for pending linked PR checks without merging.",
			Usage:       "gira ticket wait [TICKET] [--repo OWNER/REPO] [--timeout 5m] [--interval 5s]",
			Since:       "v1.0.0",
			Docs:        []string{"README.md", "docs-site/ticket-workflow.md", "docs/dogfood.md"},
			GuideTopics: []string{"ticket", "agent"},
			GuideOrder:  50,
			Examples: []CommandExample{
				{Summary: "Wait for CI", Command: "gira ticket wait --timeout 5m"},
			},
		},
		{
			Path:        []string{"ticket", "finish"},
			Summary:     "Merge the linked PR when policy allows; Draft PRs stop after ready transition and require a new finish preview.",
			Usage:       "gira ticket finish [TICKET] --dry-run|--apply [--repo OWNER/REPO] [--sync-local]",
			Since:       "v1.0.0",
			Docs:        []string{"README.md", "docs-site/ticket-workflow.md", "docs/dogfood.md"},
			GuideTopics: []string{"ticket", "agent"},
			GuideOrder:  60,
			Examples: []CommandExample{
				{Summary: "Preview finish", Command: "gira ticket finish --dry-run"},
			},
		},
		{
			Path:    []string{"ticket", "status"},
			Summary: "Report ticket status, linked PR blockers, and next action.",
			Usage:   "gira ticket status [TICKET] [--repo OWNER/REPO] [--json|--html --output PATH]",
			Since:   "v1.0.0",
			Flags: []FlagSpec{
				{Name: "--repo", Summary: "Target GitHub repo in OWNER/REPO format."},
				{Name: "--ticket", Summary: "Ticket number. Can also be numeric positional."},
				{Name: "--issue", Summary: "Compatibility alias for --ticket."},
				{Name: "--json", Summary: "Emit the stable ticket-status/v1 JSON contract with issue, branch, PR, checks, review, evidence, blockers, warnings, and next action."},
				{Name: "--html", Summary: "Write a static local HTML report from ticket-status/v1."},
				{Name: "--output", Summary: "Output path for --html."},
			},
			Docs:        []string{"README.md", "docs-site/ticket-workflow.md", "docs/dogfood.md"},
			GuideTopics: []string{"ticket", "agent"},
			GuideOrder:  70,
			Examples: []CommandExample{
				{Summary: "Inspect current branch ticket", Command: "gira ticket status"},
				{Summary: "Export a ticket status page", Command: "gira ticket status 42 --repo OWNER/app --html --output out/gira/ticket-42.html"},
			},
		},
	}
	applyAdapterCapabilities(specs)
	return specs
}

func applyAdapterCapabilities(specs []CommandSpec) {
	for i := range specs {
		switch commandSpecKey(specs[i].Path) {
		case "setup global":
			specs[i].Adapter = adapterApply("writes global config and repo registry files; --dry-run previews file changes", JSONSupportStable)
		case "workspace repos sync":
			specs[i].Adapter = adapterApply("updates workspace repo allowlist; --dry-run previews selected repositories", JSONSupportStable)
		case "config storage":
			specs[i].Adapter = AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Read-only config-storage-report/v1 over local paths; does not read private run artifact contents or mutate files."}
		case "ops limit":
			specs[i].Adapter = AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Diagnostic only; optional workflow estimates use static cost profiles and do not authorize mutations."}
		case "workspace status":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "queue list":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "queue next":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "queue handoff":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "queue take":
			specs[i].Adapter = adapterApply("delegates to ticket start for a handoff-safe queue item; --dry-run previews selection, handoff readiness, and ticket start", JSONSupportStable)
		case "dispatch goal":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "pm bootstrap":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "pm spec":
			specs[i].Adapter = AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Local rendering only; does not call GitHub or mutate files."}
		case "pm compile":
			specs[i].Adapter = AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Read-only deterministic compilation; optional Goal context reads GitHub and no path mutates files or GitHub."}
		case "pm record":
			specs[i].Adapter = adapterApply("appends a typed GitHub issue comment; --dry-run validates idempotency, privacy, and history resolution", JSONSupportStable)
		case "pm context":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "pm discovery":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "pm measure":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "pm observe":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "pm replan":
			specs[i].Adapter = adapterApply("applies fingerprint-approved safe graph mutations and durable override/replan receipts; irreversible actions remain residual decisions", JSONSupportStable)
		case "pm accept":
			specs[i].Adapter = adapterApply("persists an evidence-mapped PM acceptance result and typed learning transition; dry-run rejects delivery proxies for outcome validation", JSONSupportStable)
		case "pm qa":
			specs[i].Adapter = AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Reads GitHub issue and PR context; does not mutate GitHub."}
		case "pm conformance":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "completion":
			specs[i].Adapter = adapterRead(JSONSupportNone)
		case "feature list":
			specs[i].Adapter = adapterRead(JSONSupportStable, "gira feat list")
		case "feature check":
			specs[i].Adapter = adapterRead(JSONSupportStable, "gira feat check")
		case "feature for":
			specs[i].Adapter = adapterRead(JSONSupportStable, "gira feat for")
		case "goal new":
			specs[i].Adapter = adapterApply("creates a GitHub issue with Goal Mode operating sections; --dry-run previews payload, labels, and approval evidence", JSONSupportStable)
		case "goal status":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "goal report":
			specs[i].Adapter = adapterRead(JSONSupportStable, "gira goal dossier")
		case "goal plan":
			specs[i].Adapter = adapterApply("creates linked child tickets from reviewed goal-plan proposals when run with --apply; --dry-run previews the same plan", JSONSupportStable)
		case "goal graph":
			specs[i].Adapter = adapterApply("compiles read-only by default; --apply lowers fingerprint-approved child actions and posts a receipt", JSONSupportStable)
		case "goal next":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "goal handoff":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "goal finish":
			specs[i].Adapter = adapterApply("posts an idempotent goal finish receipt; explicit --terminal done may normalize labels and close the goal, while explicit --terminal human_review preserves blocker handoff", JSONSupportStable)
		case "report weekly":
			specs[i].Adapter = AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Read-only GitHub issue, milestone, and PR inspection; optional output writes local report artifacts only."}
		case "report portfolio":
			specs[i].Adapter = AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportNone, Notes: "Reads stable GitHub/Gira contracts and writes only the explicitly selected local HTML path; never publishes or opens the artifact."}
		case "report wbs":
			specs[i].Adapter = AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Read-only GitHub issue, milestone, and roadmap inspection; optional output writes local report artifacts only."}
		case "report schedule":
			specs[i].Adapter = AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Read-only schedule projection over WBS issue, milestone, and roadmap evidence; optional output writes local report artifacts only."}
		case "report release-notes":
			specs[i].Adapter = AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Read-only GitHub issue, merged PR, milestone, and closing-reference inspection; optional output writes local report artifacts only."}
		case "report changelog":
			specs[i].Adapter = AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Read-only GitHub issue, merged PR, milestone, and closing-reference inspection; optional output writes local report artifacts only."}
		case "report milestone", "report backlog-health", "report delivery-status", "report qa-checklist":
			specs[i].Adapter = AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Read-only GitHub issue, milestone, and PR inspection; optional output writes local report artifacts only."}
		case "stats repo":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "stats pulse":
			specs[i].Adapter = AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: JSONSupportStable, Notes: "Read-only pulse-report/v1alpha1 over GitHub/Gira issue and PR evidence; does not score people or mutate state."}
		case "stats workspace":
			specs[i].Adapter = adapterUnsupported("planned command; adapters should not expose it until implemented", JSONSupportPlanned)
		case "milestone new":
			specs[i].Adapter = adapterApply("creates a GitHub milestone; --dry-run previews payload and target repo", JSONSupportStable)
		case "milestone list":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "milestone status":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "milestone assign":
			specs[i].Adapter = adapterApply("assigns selected issues to a milestone; --dry-run previews issue updates", JSONSupportStable)
		case "milestone plan":
			specs[i].Adapter = adapterApply("selects and assigns candidate tickets; --dry-run previews candidate set and mutations", JSONSupportStable)
		case "jira init":
			specs[i].Adapter = adapterApply("writes reviewed non-secret Jira provider config; --dry-run previews discovered config", JSONSupportStable)
		case "jira doctor":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "jira mirror":
			specs[i].Adapter = adapterApply("creates or reuses a GitHub mirror issue; --dry-run previews mirror resolution", JSONSupportStable)
		case "jira transition":
			specs[i].Adapter = adapterDryRun("plans Jira transition reachability only; adapters must not treat it as Jira mutation approval", JSONSupportStable)
		case "jira import":
			specs[i].Adapter = adapterApply("creates GitHub issues from Jira import sources; --dry-run previews created and skipped issues", JSONSupportStable)
		case "jira export":
			specs[i].Adapter = adapterApply("writes Jira-friendly export artifacts to the requested output path", JSONSupportStable)
		case "ticket new":
			specs[i].Adapter = adapterApply("creates a GitHub issue, may set a native parent, and may optionally start it; --dry-run previews issue body, labels, and parent plan", JSONSupportStable, "gira new", "gira t new", "gira t n")
		case "ticket parent":
			specs[i].Adapter = adapterApply("sets or clears a native GitHub sub-issue parent; read mode shows the current parent and mutation modes require --dry-run or --apply", JSONSupportStable)
		case "ticket view":
			specs[i].Adapter = adapterRead(JSONSupportStable, "gira ticket show")
		case "ticket prompt":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "ticket handoff":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "ticket review":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "ticket self-review":
			specs[i].Adapter = adapterApply("posts a self-review check note to the linked PR; --dry-run previews the rendered note and approval evidence", JSONSupportStable)
		case "ticket start":
			specs[i].Adapter = adapterApply("applies a branch strategy, records lifecycle state, and moves the issue to in-progress; --dry-run previews readiness", JSONSupportStable, "gira start")
		case "ticket pr":
			specs[i].Adapter = adapterApply("creates or validates a linked PR; --dry-run previews PR body and branch binding", JSONSupportStable)
		case "ticket note":
			specs[i].Adapter = adapterApply("posts issue or PR comments; --dry-run previews resolved targets and rendered note", JSONSupportStable)
		case "ticket supersede":
			specs[i].Adapter = adapterApply("creates a replacement ticket, posts cross-links, and closes the original; --dry-run previews all planned mutations", JSONSupportStable)
		case "ticket checks":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "ticket wait":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		case "ticket finish":
			specs[i].Adapter = adapterApply("may merge the linked PR, post receipts, normalize labels, and close the issue; Draft PR apply stops after ready transition, and --dry-run warns before merge or remote branch deletion", JSONSupportStable)
		case "ticket status":
			specs[i].Adapter = adapterRead(JSONSupportStable)
		default:
			specs[i].Adapter = adapterUnsupported("missing adapter capability metadata", "")
		}
	}
}

func adapterRead(jsonSupport string, aliases ...string) AdapterCommandCapability {
	return AdapterCommandCapability{Class: AdapterCapabilityRead, JSONSupport: jsonSupport, Aliases: aliases}
}

func adapterDryRun(boundary string, jsonSupport string, aliases ...string) AdapterCommandCapability {
	return AdapterCommandCapability{Class: AdapterCapabilityDryRunMutation, MutationBoundary: boundary, JSONSupport: jsonSupport, Aliases: aliases}
}

func adapterApply(boundary string, jsonSupport string, aliases ...string) AdapterCommandCapability {
	return AdapterCommandCapability{Class: AdapterCapabilityApplyMutation, MutationBoundary: boundary, JSONSupport: jsonSupport, Aliases: aliases}
}

func adapterUnsupported(reason string, jsonSupport string, aliases ...string) AdapterCommandCapability {
	return AdapterCommandCapability{Class: AdapterCapabilityUnsupported, JSONSupport: jsonSupport, Aliases: aliases, Notes: reason}
}

func FindCommandSpec(path ...string) (CommandSpec, bool) {
	key := commandSpecKey(path)
	for _, spec := range CoreCommandSpecs() {
		if commandSpecKey(spec.Path) == key {
			return spec, true
		}
	}
	return CommandSpec{}, false
}

func BuildCommandCapabilityReport(specs []CommandSpec) CommandCapabilityReport {
	specs = append([]CommandSpec(nil), specs...)
	sort.Slice(specs, func(i, j int) bool {
		return commandSpecKey(specs[i].Path) < commandSpecKey(specs[j].Path)
	})
	report := CommandCapabilityReport{SchemaVersion: CommandCapabilitySchemaVersion}
	for _, spec := range specs {
		adapter := spec.Adapter
		if adapter.Class == "" {
			adapter = adapterUnsupported("missing adapter capability metadata", "")
		}
		report.Commands = append(report.Commands, CommandCapabilityEntry{
			Path:             append([]string(nil), spec.Path...),
			Canonical:        "gira " + strings.Join(spec.Path, " "),
			Summary:          spec.Summary,
			Capability:       adapter.Class,
			MutationBoundary: adapter.MutationBoundary,
			JSONSupport:      adapter.JSONSupport,
			Aliases:          append([]string(nil), adapter.Aliases...),
			Docs:             append([]string(nil), spec.Docs...),
			Since:            spec.Since,
			Notes:            adapter.Notes,
		})
	}
	return report
}

func RenderCommandCapabilitiesMarkdown(specs []CommandSpec) string {
	report := BuildCommandCapabilityReport(specs)
	var b strings.Builder
	b.WriteString("# Command Capabilities\n\n")
	b.WriteString("This page is generated from Gira's command metadata registry. Update `internal/gira/command_registry.go` first, then refresh this page.\n\n")
	fmt.Fprintf(&b, "Schema version: `%s`\n\n", report.SchemaVersion)
	b.WriteString("| Command | Aliases | Capability | JSON support | Mutation boundary | Docs |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- |\n")
	for _, command := range report.Commands {
		boundary := command.MutationBoundary
		if boundary == "" {
			boundary = "none"
		}
		aliases := strings.Join(command.Aliases, ", ")
		if aliases == "" {
			aliases = "none"
		}
		docs := strings.Join(command.Docs, ", ")
		if docs == "" {
			docs = "none"
		}
		fmt.Fprintf(&b, "| `%s` | %s | `%s` | `%s` | %s | %s |\n", command.Canonical, aliases, command.Capability, command.JSONSupport, boundary, docs)
	}
	return b.String()
}

func RenderCommandReferenceMarkdown(specs []CommandSpec) string {
	specs = append([]CommandSpec(nil), specs...)
	sort.Slice(specs, func(i, j int) bool {
		return commandSpecKey(specs[i].Path) < commandSpecKey(specs[j].Path)
	})
	var b strings.Builder
	b.WriteString("# Command Reference\n\n")
	b.WriteString("This page is generated from Gira's command metadata registry. Update `internal/gira/command_registry.go` first, then refresh this page.\n\n")
	for _, spec := range specs {
		fmt.Fprintf(&b, "## `%s`\n\n", strings.Join(spec.Path, " "))
		fmt.Fprintf(&b, "%s\n\n", spec.Summary)
		fmt.Fprintf(&b, "Usage:\n\n```bash\n%s\n```\n\n", spec.Usage)
		if spec.Since != "" {
			fmt.Fprintf(&b, "Since: `%s`\n\n", spec.Since)
		}
		if len(spec.Flags) > 0 {
			b.WriteString("Flags:\n\n")
			for _, flag := range spec.Flags {
				fmt.Fprintf(&b, "- `%s`: %s\n", flag.Name, flag.Summary)
			}
			b.WriteString("\n")
		}
		if len(spec.Examples) > 0 {
			b.WriteString("Examples:\n\n")
			for _, example := range spec.Examples {
				fmt.Fprintf(&b, "- %s\n\n```bash\n%s\n```\n\n", example.Summary, example.Command)
			}
		}
		if len(spec.Docs) > 0 {
			b.WriteString("Documented in: ")
			for i, doc := range spec.Docs {
				if i > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(&b, "`%s`", doc)
			}
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

func RenderGuideCommandSection(topic string, specs []CommandSpec) string {
	specs = filterCommandSpecsForGuide(topic, specs)
	sortGuideSpecs(specs)
	var b strings.Builder
	for _, spec := range specs {
		fmt.Fprintf(&b, "  %s\n", spec.Usage)
		fmt.Fprintf(&b, "    %s\n", spec.Summary)
		for _, example := range spec.Examples {
			fmt.Fprintf(&b, "    Example: %s\n", example.Command)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func filterCommandSpecsForGuide(topic string, specs []CommandSpec) []CommandSpec {
	var filtered []CommandSpec
	for _, spec := range specs {
		for _, guideTopic := range spec.GuideTopics {
			if guideTopic == topic {
				filtered = append(filtered, spec)
				break
			}
		}
	}
	return filtered
}

func sortGuideSpecs(specs []CommandSpec) {
	sort.Slice(specs, func(i, j int) bool {
		if specs[i].GuideOrder != specs[j].GuideOrder {
			if specs[i].GuideOrder == 0 {
				return false
			}
			if specs[j].GuideOrder == 0 {
				return true
			}
			return specs[i].GuideOrder < specs[j].GuideOrder
		}
		return commandSpecKey(specs[i].Path) < commandSpecKey(specs[j].Path)
	})
}

func commandSpecKey(path []string) string {
	return strings.Join(path, " ")
}
