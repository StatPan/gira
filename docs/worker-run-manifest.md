# Worker Run Manifest

`worker-run/v1` is the runtime evidence manifest for worker executions that
produce or review GitHub work under a Gira contract.

It lets Gira export worker evidence and lets Agentree render a worker overlay
without making either product the canonical tracker for work state. GitHub
issues, branches, PRs, checks, reviews, comments, and closing semantics remain
the source of truth for durable execution state.

## Purpose

Current local orchestration uses ad hoc files such as:

```text
/tmp/statpan-orchestration/*-events.jsonl
/tmp/statpan-orchestration/*-stderr.log
/tmp/statpan-orchestration/*-result.md
/tmp/statpan-orchestration/*.prompt
```

Those files are useful runtime evidence, but they are not a stable contract.
`worker-run/v1` records enough typed metadata for handoff, review, recovery,
and dashboard rendering while keeping raw transcripts and local logs optional.

## Ownership Boundary

| Owner | Owns | Does not own |
| --- | --- | --- |
| GitHub | Issues, labels, branches, PRs, commits, checks, reviews, comments, merge and close state. | Local worker prompts, event streams, stderr, token use, or private runtime files. |
| Gira | Normalization, validation, export, and links between GitHub work and worker evidence. | Live worker supervision, retry scheduling, model routing, or process control. |
| Worker runtime | Local execution, prompts, tool events, stderr, result notes, and optional manifest production. | Canonical ticket, PR, review, or finish state. |
| Agentree | Presentation and correlation of Gira/GitHub work nodes with optional runtime evidence. | Source-of-truth status fields or write-back execution state. |

The sync direction is one-way:

```text
GitHub work state + local worker manifest -> Gira export -> Agentree/renderers
```

## Contract Shape

Every manifest uses:

```json
{
  "schema_version": "worker-run/v1"
}
```
Top-level fields:

| Field | Required | Meaning |
| --- | --- | --- |
| `schema_version` | Yes | Must be `worker-run/v1`. |
| `run_id` | Yes | Stable ID for this worker attempt. Prefer a deterministic operator slug or UUID. |
| `attempt` | No | Attempt number when the same issue or PR is retried. |
| `session` | Yes | Worker session/thread/process metadata. |
| `model` | No | Model/provider metadata when known. |
| `scope` | Yes | GitHub work scope: repo, issue, branch, PR, goal, and related work IDs. |
| `status` | Yes | Current or terminal worker-run status. |
| `status_transitions` | Yes | Ordered lifecycle events for this run. |
| `artifacts` | Yes | Prompt, event log, stderr, result, report, and evidence references. |
| `safety_boundary` | Yes | Delegation lane, allowed/prohibited operations, credential and approval limits. |
| `verification` | No | Commands run by the worker and their outcomes. |
| `outputs` | No | Produced commits, PRs, issue comments, PR comments, labels, or exported reports. |
| `human_decision_gates` | No | Decisions a human supplied or still needs to supply. |
| `summary` | No | Short human-readable result summary. |
| `warnings` | No | Degraded or incomplete manifest evidence. |

## Status Values

`status` is the worker-run state, not the GitHub issue or PR state.

Allowed values:

| Status | Meaning |
| --- | --- |
| `queued` | Run is known but has not started. |
| `running` | Worker is active or its latest event is in progress. |
| `completed` | Worker finished its assigned task and produced a result. |
| `failed` | Worker stopped because of an execution failure. |
| `blocked` | Worker cannot safely proceed without an external dependency or decision. |
| `review_needed` | Worker produced reviewable evidence or a PR that needs review. |
| `human_decision_required` | Worker reached an explicit human decision gate. |
| `cancelled` | Operator or runtime cancelled the run. |
| `superseded` | A later run replaces this attempt. |
| `unknown` | Evidence exists, but status cannot be determined. |

Each `status_transitions[]` item should include:

| Field | Meaning |
| --- | --- |
| `status` | One allowed status value. |
| `observed_at` | RFC 3339 timestamp when the status was observed. |
| `source` | Evidence source such as `worker_event_log`, `operator`, `github_pr`, or `gira_export`. |
| `reason` | Short machine-readable reason code. |
| `message` | Optional human-readable note. |

## Session And Model

`session` identifies the runtime attempt:

```json
{
  "session": {
    "worker_id": "codex",
    "thread_id": "019e93a2-6ed5-73d0-84b1-69df542477b5",
    "session_id": "gira-686-worker",
    "host": "statpan-local",
    "pid": null,
    "started_at": "2026-06-05T02:15:00+09:00",
    "completed_at": null
  }
}
```

`model` should be descriptive, not policy-bearing:

```json
{
  "model": {
    "provider": "openai",
    "name": "gpt-5-codex",
    "mode": "implementation_worker"
  }
}
```

Exact model names, prompt IDs, trace IDs, and attempt IDs belong here or in the
provenance envelope. They should not become high-cardinality GitHub labels.

## Scope

`scope` ties runtime evidence to durable work:

| Field | Meaning |
| --- | --- |
| `repo` | `OWNER/REPO` for the execution repo. |
| `issue` | Issue number and URL when known. |
| `branch` | Work branch name when known. |
| `pull_request` | PR number and URL when known. |
| `goal` | Parent goal issue when the run belongs to a goal graph. |
| `workspace` | Optional workspace name or config source. |
| `source_refs` | Stable refs such as `issue:StatPan/gira#686`, `branch:issue-686-worker-run-v1-manifest`, or `worker-run:gira-686-worker`. |

The scope should preserve GitHub URLs where possible so dashboard consumers can
link back to canonical evidence.

## Artifact Visibility

`artifacts[]` separates GitHub-visible evidence from local runtime files.

| Visibility | Meaning | Render behavior |
| --- | --- | --- |
| `github_visible` | Evidence is on a GitHub issue, PR, commit, check, or comment. | Always safe for Gira/Agentree to link if the viewer has repo access. |
| `local_runtime` | Evidence exists only on the worker host filesystem. | Local renderers may link or open it; hosted renderers should show metadata only unless explicitly published. |
| `published_artifact` | Evidence was exported or uploaded to a chosen artifact root. | Render as an artifact link, but do not treat it as canonical work state. |

Artifact fields:

| Field | Required | Meaning |
| --- | --- | --- |
| `kind` | Yes | `prompt`, `event_log`, `stderr`, `result`, `report`, `diff`, `screenshot`, `browser_evidence`, `github_comment`, `commit`, or `other`. |
| `visibility` | Yes | One of the visibility values above. |
| `path` | No | Local or exported path. Use relative paths inside export bundles when possible. |
| `url` | No | GitHub or published URL. |
| `content_type` | No | MIME-like type such as `application/jsonl` or `text/markdown`. |
| `sha256` | No | Optional content hash for immutable artifact checks. |
| `size_bytes` | No | Optional size for rendering and audit. |
| `redaction` | No | `none`, `path_only`, `summary_only`, or `redacted`. |
| `description` | No | Short display text. |

Raw prompt, event, and stderr contents should not be copied into the manifest.
The manifest points to them.

## Safety Boundary

The safety boundary records what the worker was allowed to do. It should be
kept even when the worker completed successfully.

Fields:

| Field | Meaning |
| --- | --- |
| `delegation_lane` | `lane:agent`, `lane:hybrid`, `lane:human`, or `unknown`. |
| `approval_policy` | Human-readable policy such as `no_external_service_writes`. |
| `allowed_operations` | Bounded operations allowed by the assignment. |
| `prohibited_operations` | Explicit non-goals and unsafe operations. |
| `credential_boundary` | Credential and secret handling boundary. |
| `mutation_boundary` | Whether the run may mutate repo files, GitHub objects, external services, or none. |
| `requires_human_approval` | Boolean for final merge/release/decision gates. |
| `stop_conditions` | Conditions where the worker must stop and ask. |

## Verification

`verification.commands[]` records commands the worker ran:

| Field | Meaning |
| --- | --- |
| `command` | Command line. |
| `cwd` | Working directory, usually repo root. |
| `started_at`, `completed_at` | Optional timestamps. |
| `exit_code` | Numeric exit code when known. |
| `status` | `passed`, `failed`, `skipped`, or `unknown`. |
| `summary` | Short result. |

Use this for local commands such as `git diff --check`, `go test ./...`, or
targeted tests. GitHub Actions checks remain GitHub-visible evidence and should
also be linked through `outputs` or `artifacts`.

## Outputs

`outputs` records durable evidence produced by the run:

| Field | Meaning |
| --- | --- |
| `commits[]` | Commit SHA, branch, subject, and URL when pushed. |
| `pull_requests[]` | PR number, URL, state, draft flag, and closing references. |
| `comments[]` | Issue or PR comments written by the worker or Gira lifecycle commands. |
| `labels[]` | Labels added or removed when relevant. |
| `checks[]` | Check names and URLs when available. |
| `reports[]` | Exported reports or receipts. |

This section should not invent completion. A PR is complete only when GitHub
shows merge/close evidence.

## Human Decision Gates

`human_decision_gates[]` records approval points:

| Field | Meaning |
| --- | --- |
| `gate_id` | Stable ID within the run. |
| `kind` | `approval`, `review`, `credential`, `product_decision`, `merge`, `release`, or `other`. |
| `status` | `open`, `approved`, `rejected`, `deferred`, or `not_required`. |
| `required_by` | Source policy or stop condition. |
| `question` | Decision needed. |
| `decision` | Human decision when supplied. |
| `decided_by` | Optional actor class such as `human` or `maintainer`; avoid private identity inference. |
| `decided_at` | Timestamp when supplied. |
| `evidence_ref` | Link to GitHub comment, PR review, or local result. |

Human intervention is a safety signal. It is not a productivity score.

## Export Placement

When Gira exports a dashboard or workspace bundle, worker manifests should be
included as optional runtime overlay artifacts:

```text
out/dashboard/
  raw/
    worker_runs.json
  worker-runs/
    RUN_ID.json
```

`raw/worker_runs.json` may be an index with compact rows. Each
`worker-runs/RUN_ID.json` is the full `worker-run/v1` manifest.

Manifest entries should be listed in `manifest.json`:

```json
{
  "path": "worker-runs/gira-686-worker.json",
  "kind": "worker_run",
  "schema_version": "worker-run/v1"
}
```

Gira may export these manifests from local runtime files or from
GitHub-visible provenance comments. Export is read-only with respect to worker
execution: it must not start, stop, retry, or supervise workers.

## Agentree Consumption

Agentree can render `worker-run/v1` as a runtime overlay on a work node keyed by:

```text
repo + issue + branch + pull_request + run_id
```

Agentree should:

- Link to GitHub issue and PR evidence as canonical state.
- Show local runtime artifacts only when the viewer is on the same host or the
  artifacts were explicitly published.
- Render worker status as runtime status, visually separate from GitHub issue
  and PR status.
- Surface open human decision gates without treating Agentree fields as
  canonical approvals.
- Avoid ranking agents, people, token spend, or time online.

## Example

This example is based on the current overnight worker files for
`StatPan/gira#686`.

```json
{
  "schema_version": "worker-run/v1",
  "run_id": "gira-686-worker",
  "attempt": 1,
  "session": {
    "worker_id": "codex",
    "thread_id": "019e93a2-6ed5-73d0-84b1-69df542477b5",
    "session_id": "gira-686-worker",
    "host": "statpan-local",
    "pid": null,
    "started_at": "2026-06-05T02:15:00+09:00",
    "completed_at": null
  },
  "model": {
    "provider": "openai",
    "name": "gpt-5-codex",
    "mode": "implementation_worker"
  },
  "scope": {
    "repo": "StatPan/gira",
    "issue": {
      "number": 686,
      "url": "https://github.com/StatPan/gira/issues/686"
    },
    "branch": "issue-686-worker-run-v1-manifest",
    "pull_request": null,
    "goal": null,
    "source_refs": [
      "issue:StatPan/gira#686",
      "branch:issue-686-worker-run-v1-manifest",
      "worker-run:gira-686-worker"
    ]
  },
  "status": "running",
  "status_transitions": [
    {
      "status": "running",
      "observed_at": "2026-06-05T02:15:00+09:00",
      "source": "worker_event_log",
      "reason": "thread_started",
      "message": "Worker thread and first command events were recorded."
    }
  ],
  "artifacts": [
    {
      "kind": "prompt",
      "visibility": "local_runtime",
      "path": "/tmp/statpan-orchestration/gira-686-worker.prompt",
      "content_type": "text/plain",
      "redaction": "path_only",
      "description": "Worker assignment prompt."
    },
    {
      "kind": "event_log",
      "visibility": "local_runtime",
      "path": "/tmp/statpan-orchestration/gira-686-worker-events.jsonl",
      "content_type": "application/jsonl",
      "redaction": "path_only",
      "description": "Codex runtime events for the worker attempt."
    },
    {
      "kind": "stderr",
      "visibility": "local_runtime",
      "path": "/tmp/statpan-orchestration/gira-686-worker-stderr.log",
      "content_type": "text/plain",
      "redaction": "path_only",
      "description": "Worker stderr stream."
    },
    {
      "kind": "github_comment",
      "visibility": "github_visible",
      "url": "https://github.com/StatPan/gira/issues/686",
      "content_type": "text/markdown",
      "redaction": "none",
      "description": "Issue body defines the requested worker-run/v1 contract."
    }
  ],
  "safety_boundary": {
    "delegation_lane": "unknown",
    "approval_policy": "docs_and_fixture_only_no_runtime_supervision",
    "allowed_operations": [
      "read repo docs and GitHub issue state",
      "edit documentation and sample fixture files",
      "run local verification commands",
      "commit and open a linked PR"
    ],
    "prohibited_operations": [
      "implement hosted dashboard",
      "implement long-running worker runtime",
      "write external service state or credentials",
      "make Gira a live process supervisor"
    ],
    "credential_boundary": "no provider credentials or external service writes",
    "mutation_boundary": "repo_files_and_github_pr_only",
    "requires_human_approval": true,
    "stop_conditions": [
      "scope expands beyond contract artifact",
      "external service write is required",
      "merge or release approval is needed"
    ]
  },
  "verification": {
    "commands": []
  },
  "outputs": {
    "commits": [],
    "pull_requests": [],
    "comments": [],
    "checks": [],
    "reports": []
  },
  "human_decision_gates": [
    {
      "gate_id": "pr-review",
      "kind": "review",
      "status": "open",
      "required_by": "normal_pr_review",
      "question": "Should the worker-run/v1 contract be accepted?",
      "evidence_ref": "issue:StatPan/gira#686"
    }
  ],
  "summary": "Runtime evidence manifest definition worker for StatPan/gira#686.",
  "warnings": [
    {
      "code": "run_in_progress",
      "severity": "info",
      "message": "This example was captured while the worker run was still active."
    }
  ]
}
```
