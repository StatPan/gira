package gira

import (
	"strings"
	"testing"
	"time"
)

func TestSelectQueueHeadPrefersActiveIssueForIdempotentResume(t *testing.T) {
	state := ExecutionLoopState{ActiveIssue: 8}
	got := SelectQueueHead([]int{11, 8, 14}, state)
	if got != 8 {
		t.Fatalf("SelectQueueHead()=%d want 8", got)
	}
}

func TestSelectQueueHeadFallsBackToLowestOpenIssue(t *testing.T) {
	state := ExecutionLoopState{ActiveIssue: 8}
	got := SelectQueueHead([]int{11, 14, 9}, state)
	if got != 9 {
		t.Fatalf("SelectQueueHead()=%d want 9", got)
	}
}

func TestRecordBlockerSuppressesRepeatedSpamWithinCooldown(t *testing.T) {
	now := time.Date(2026, 5, 2, 5, 0, 0, 0, time.UTC)
	state := ExecutionLoopState{}
	if !RecordBlocker(&state, 97, "stale branch", now, time.Hour) {
		t.Fatalf("first blocker should report")
	}
	if RecordBlocker(&state, 97, "stale branch", now.Add(15*time.Minute), time.Hour) {
		t.Fatalf("repeated blocker within cooldown should be suppressed")
	}
	note := state.LastBlockerByIssue[97]
	if note.Suppressed != 1 {
		t.Fatalf("suppressed=%d want 1", note.Suppressed)
	}
}

func TestRecordBlockerReportsAfterCooldownOrReasonChange(t *testing.T) {
	now := time.Date(2026, 5, 2, 5, 0, 0, 0, time.UTC)
	state := ExecutionLoopState{}
	_ = RecordBlocker(&state, 97, "stale branch", now, time.Hour)
	if !RecordBlocker(&state, 97, "format drift", now.Add(20*time.Minute), time.Hour) {
		t.Fatalf("new reason should report")
	}
	if !RecordBlocker(&state, 97, "format drift", now.Add(2*time.Hour), time.Hour) {
		t.Fatalf("same reason after cooldown should report")
	}
}

func TestVerifyPostMergeClosureAndNext(t *testing.T) {
	state := ExecutionLoopState{ActiveIssue: 97}
	result := VerifyPostMergeClosureAndNext(97, false, []int{99, 98}, &state)
	if !result.VerifiedClosed {
		t.Fatalf("expected closed verification true")
	}
	if state.LastClosedIssue != 97 {
		t.Fatalf("last closed issue=%d want 97", state.LastClosedIssue)
	}
	if result.NextIssue != 98 {
		t.Fatalf("next issue=%d want 98", result.NextIssue)
	}
}

func TestFormatOperationalReportIsStableThreeLines(t *testing.T) {
	report := FormatOperationalReport(LoopRunSummary{Issue: 97, Action: "verify", Result: "ok", NextIssue: 98, RetryCount: 1})
	parts := strings.Split(report, "\n")
	if len(parts) != 3 {
		t.Fatalf("lines=%d want 3\n%s", len(parts), report)
	}
	if !strings.Contains(parts[2], "blocker=none") {
		t.Fatalf("missing blocker default: %q", parts[2])
	}
}
