package gira

import (
	"strings"
	"testing"
)

func TestFormatConfigReportsExposeReadableStatus(t *testing.T) {
	global := FormatConfigGlobalReport(ConfigGlobalReport{
		ConfigRoot:     "/tmp/gira",
		Config:         ConfigFileStatus{Path: "/tmp/gira/config.yaml", Exists: true, Valid: true},
		ReposRoot:      ConfigPathStatus{Path: "/tmp/gira/repos", Exists: true},
		WorkspacesRoot: ConfigPathStatus{Path: "/tmp/gira/workspaces", Exists: false},
	})
	for _, want := range []string{"config global: /tmp/gira", "config: /tmp/gira/config.yaml exists=true valid=true", "repos: /tmp/gira/repos exists=true", "workspaces: /tmp/gira/workspaces exists=false"} {
		if !strings.Contains(global, want) {
			t.Fatalf("global report missing %q:\n%s", want, global)
		}
	}

	repo := FormatConfigRepoReport(ConfigRepoReport{
		Repo:       "StatPan/gira",
		Source:     "explicit",
		Detail:     "from --repo",
		GlobalRepo: ConfigFileStatus{Path: "/tmp/gira/repos/StatPan/gira.yaml", Exists: true, Valid: false, Error: "bad yaml"},
		RepoContracts: []ConfigFileStatus{
			{Path: ".gira/config.yaml", Exists: false},
		},
	})
	for _, want := range []string{"config repo: StatPan/gira", "source: explicit", "detail: from --repo", "global repo: /tmp/gira/repos/StatPan/gira.yaml exists=true valid=false error=bad yaml", "repo contract: .gira/config.yaml exists=false"} {
		if !strings.Contains(repo, want) {
			t.Fatalf("repo report missing %q:\n%s", want, repo)
		}
	}

	doctor := FormatConfigDoctorReport(ConfigDoctorReport{
		ConfigRoot:   "/tmp/gira",
		Source:       "defaults",
		GlobalConfig: ConfigFileStatus{Path: "/tmp/gira/config.yaml", Exists: true, Valid: true},
		NextSteps:    []string{"register a repo"},
	})
	for _, want := range []string{"config doctor: /tmp/gira", "source: defaults", "global config: /tmp/gira/config.yaml exists=true valid=true", "next step: register a repo"} {
		if !strings.Contains(doctor, want) {
			t.Fatalf("doctor report missing %q:\n%s", want, doctor)
		}
	}
}

func TestFormatDoctorReportShowsFirstRemediationAndReadyNextStep(t *testing.T) {
	failing := FormatDoctorReport(DoctorReport{
		Ready: false,
		Repo:  "StatPan/gira",
		Checks: []DoctorCheck{
			{ID: "gh_available", Status: DoctorCheckPass, Detail: "gh version 2"},
			{ID: "gh_auth", Status: DoctorCheckFail, Detail: "not logged in", Remediation: "run `gh auth login`"},
			{ID: "repo_access", Status: DoctorCheckFail, Detail: "skipped", Remediation: "fix auth first"},
		},
	})
	for _, want := range []string{"doctor: NOT READY", "repo: StatPan/gira", "- [fail] gh_auth: not logged in", "remediation: run `gh auth login`", "next step: fix gh_auth: run `gh auth login`"} {
		if !strings.Contains(failing, want) {
			t.Fatalf("failing doctor report missing %q:\n%s", want, failing)
		}
	}

	ready := FormatDoctorReport(DoctorReport{Ready: true, Repo: "StatPan/gira"})
	if !strings.Contains(ready, "doctor: READY") || !strings.Contains(ready, "next step: gira status --repo StatPan/gira") {
		t.Fatalf("ready doctor report unexpected:\n%s", ready)
	}
}

func TestFormatTicketReportsKeepLifecycleOutputStable(t *testing.T) {
	checks := FormatTicketChecks(TicketChecksReport{
		Issue:    42,
		PRNumber: 43,
		Ready:    false,
		Blockers: []string{"checks_pending"},
		Checks:   []DevPRCheck{{Name: "", Workflow: "CI", State: "pending"}},
		NextStep: "gira ticket wait --repo StatPan/gira --ticket 42",
	})
	for _, want := range []string{"ticket checks: ticket #42 pr=43 ready=false blockers=checks_pending", "- pending (unnamed) (CI)", "next step: gira ticket wait"} {
		if !strings.Contains(checks, want) {
			t.Fatalf("ticket checks output missing %q:\n%s", want, checks)
		}
	}

	created := FormatTicketNew(TicketNewReport{Repo: "StatPan/gira", Title: "Add thing", Labels: []string{"type:task", "status:ready"}, Body: "## Goal\nAdd thing", DryRun: true, NextStep: "gira ticket new --apply"})
	for _, want := range []string{"ticket new: dry-run Add thing", "labels: type:task,status:ready", "body:\n## Goal\nAdd thing", "next step: gira ticket new --apply"} {
		if !strings.Contains(created, want) {
			t.Fatalf("ticket new output missing %q:\n%s", want, created)
		}
	}

	note := FormatTicketNote(TicketNoteReport{
		Ticket:       42,
		Kind:         "decision",
		DryRun:       true,
		Targets:      []TicketNoteSink{{Type: "issue", Number: 42}},
		RenderedBody: "## Decision\n\nUse formatter contract tests.\n",
		NextStep:     "gira ticket note --apply",
	})
	for _, want := range []string{"ticket note: ticket #42 kind=decision target=issue#42:planned dry_run=true", "## Decision", "next step: gira ticket note --apply"} {
		if !strings.Contains(note, want) {
			t.Fatalf("ticket note output missing %q:\n%s", want, note)
		}
	}

	finish := FormatWorkFinish(WorkFinishResult{Issue: 42, PRNumber: 43, Readiness: WorkFinishReadinessReport{SchemaVersion: "finish-readiness/v1", Ready: false}, Blockers: []string{"review"}, Actions: []WorkFinishAction{{Action: "pr:merge", Status: "blocked"}}, NextStep: "resolve review"})
	for _, want := range []string{"work finish: issue #42 pr=43 merged=false readiness=blocked blockers=review actions=pr:merge:blocked", "next step: resolve review"} {
		if !strings.Contains(finish, want) {
			t.Fatalf("finish output missing %q:\n%s", want, finish)
		}
	}
}

func TestFormatCachePruneReportIncludesModeCountsAndReasons(t *testing.T) {
	out := FormatCachePruneReport(CachePruneReport{
		Root:          "/tmp/gira-cache",
		ActiveVersion: "v1.2.0",
		DryRun:        true,
		Counts:        CachePruneCounts{Planned: 1, Skipped: 1, Errors: 1},
		Actions: []CachePruneAction{
			{Action: "prune", Status: "planned", Name: "v1.0.0", Reason: "older than active version"},
			{Action: "skip", Status: "skipped", Name: "dev", Reason: "entry name is not a stable semver release"},
			{Action: "skip", Status: "error", Name: "broken", Reason: "inspect cache entry failed", Error: "permission denied"},
		},
	})
	for _, want := range []string{"cache prune: gira", "root: /tmp/gira-cache", "mode: dry-run", "counts: planned=1 applied=0 skipped=1 errors=1", "- planned prune: v1.0.0 (older than active version)", "- error skip: broken (inspect cache entry failed): permission denied"} {
		if !strings.Contains(out, want) {
			t.Fatalf("cache prune output missing %q:\n%s", want, out)
		}
	}
}

func TestFormatLowRiskReportsKeepOutputContracts(t *testing.T) {
	prompt := FormatAgentPrompt(AgentPromptReport{Prompt: "Plan this\n\n"})
	if prompt != "Plan this\n" {
		t.Fatalf("agent prompt = %q, want single trailing newline", prompt)
	}

	version := FormatVersionInfo(VersionInfo{Version: "v1.2.3", Commit: "abc123", Date: "2026-05-17"})
	if version != "gira v1.2.3 (abc123, 2026-05-17)\n" {
		t.Fatalf("version output = %q", version)
	}

	upgrade := FormatUpgradeReport(UpgradeReport{
		Current:  "v1.0.0",
		Latest:   "v1.1.0",
		Status:   "update_available",
		Channel:  "pipx",
		NextStep: "pipx upgrade gira-cli",
	})
	for _, want := range []string{"upgrade: gira", "current: v1.0.0", "latest:  v1.1.0", "status:  update_available", "channel: pipx", "next step:\n  pipx upgrade gira-cli"} {
		if !strings.Contains(upgrade, want) {
			t.Fatalf("upgrade report missing %q:\n%s", want, upgrade)
		}
	}

	none := FormatWorkStatus(WorkStatusResult{Repo: "StatPan/gira", Issue: 42, Status: "Ready", NextAction: "start_work"})
	if !strings.Contains(none, "blockers=none") || !strings.Contains(none, "next step: gira work start --repo StatPan/gira --issue 42 --dry-run") {
		t.Fatalf("work status no-blocker output unexpected:\n%s", none)
	}
	blocked := FormatWorkStatus(WorkStatusResult{Repo: "StatPan/gira", Issue: 43, Status: "In review", PRNumber: 44, Blockers: []string{"draft", "checks"}, NextAction: "mark_pr_ready"})
	if !strings.Contains(blocked, "work status: issue #43 status=In review pr=44 blockers=draft,checks next=mark_pr_ready") || !strings.Contains(blocked, "next step: mark the PR ready for review") {
		t.Fatalf("work status blocker output unexpected:\n%s", blocked)
	}
}

func TestFormatSetupGlobalReportIncludesDryRunContentAndGuidance(t *testing.T) {
	out := FormatSetupGlobalReport(SetupGlobalReport{
		Mode:       SetupGlobalModeHybrid,
		ConfigRoot: "/tmp/gira",
		Repo:       "StatPan/gira",
		Workspace:  WorkspaceSummary{Name: "personal", Owner: "StatPan"},
		InboxRepo:  "StatPan/backlog",
		RepoContract: ConfigFileStatus{
			Path:   "/repo/.gira/config.yaml",
			Exists: true,
			Valid:  true,
		},
		DryRun: true,
		Status: "planned",
		Files: []SetupGlobalFilePlan{{
			Path:    "/tmp/gira/config.yaml",
			Action:  "create",
			Content: "default_workspace: personal\n",
		}},
		Notes:    []string{"global registry remains personal metadata"},
		NextStep: "gira setup global --apply",
	})

	for _, want := range []string{
		"setup global: planned hybrid",
		"config root: /tmp/gira",
		"repo: StatPan/gira",
		"workspace: personal (StatPan)",
		"inbox: StatPan/backlog",
		"repo-local contract: /repo/.gira/config.yaml valid=true",
		"file: /tmp/gira/config.yaml action=create",
		"default_workspace: personal",
		"note: global registry remains personal metadata",
		"next step: gira setup global --apply",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("setup global report missing %q:\n%s", want, out)
		}
	}
}

func TestFormatJiraReportsKeepOutputContracts(t *testing.T) {
	importReport := FormatJiraImportReport(JiraImportReport{
		Repo:   "StatPan/gira",
		Counts: JiraImportCounts{SourceItems: 3, Create: 2, Duplicate: 1, Applied: 2},
	})
	for _, want := range []string{"jira import:", "repo: StatPan/gira", "source_items: 3", "create: 2", "duplicate: 1", "applied: 2"} {
		if !strings.Contains(importReport, want) {
			t.Fatalf("jira import report missing %q:\n%s", want, importReport)
		}
	}

	mirror := FormatJiraMirrorReport(JiraMirrorReport{
		Repo:   "StatPan/gira",
		Key:    "ABC-123",
		Status: "blocked",
		Action: "manual_resolve",
		Issue:  JiraMirrorIssue{Number: 12, Title: "Primary mirror"},
		Duplicates: []JiraMirrorIssue{
			{Number: 13, Title: "Duplicate mirror"},
		},
		Labels:   []string{"jira:ABC", "status:ready"},
		NextStep: "resolve duplicate mirrors",
	})
	for _, want := range []string{"jira mirror: blocked ABC-123", "repo: StatPan/gira", "action: manual_resolve", "issue: #12 Primary mirror", "duplicates:", "- #13 Duplicate mirror", "labels: jira:ABC,status:ready", "next step: resolve duplicate mirrors"} {
		if !strings.Contains(mirror, want) {
			t.Fatalf("jira mirror report missing %q:\n%s", want, mirror)
		}
	}

	exportReport := JiraExportReport{OutputRoot: "/tmp/gira-jira-export"}
	exportReport.Counts.Issues = 4
	exported := FormatJiraExportReport(exportReport)
	for _, want := range []string{"jira export artifacts written to /tmp/gira-jira-export", "issues: 4"} {
		if !strings.Contains(exported, want) {
			t.Fatalf("jira export report missing %q:\n%s", want, exported)
		}
	}
}
