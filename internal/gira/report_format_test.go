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

	finish := FormatWorkFinish(WorkFinishResult{Issue: 42, PRNumber: 43, Blockers: []string{"review"}, Actions: []WorkFinishAction{{Action: "pr:merge", Status: "blocked"}}, NextStep: "resolve review"})
	for _, want := range []string{"work finish: issue #42 pr=43 merged=false blockers=review actions=pr:merge:blocked", "next step: resolve review"} {
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
