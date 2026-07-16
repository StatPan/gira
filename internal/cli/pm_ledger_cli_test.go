package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/StatPan/gira/internal/gira"
)

func TestRunPMRecordDryRunJSON(t *testing.T) {
	original := newPMRecordReport
	t.Cleanup(func() { newPMRecordReport = original })
	var captured gira.PMRecordInput
	newPMRecordReport = func(input gira.PMRecordInput) (gira.PMRecordReport, error) {
		captured = input
		return gira.PMRecordReport{
			Command: "pm record", SchemaVersion: gira.PMRecordReportSchemaVersion, Repo: input.Repo.FullName(), Ticket: input.Ticket, DryRun: input.DryRun,
			Record:  gira.PMLedgerRecord{SchemaVersion: gira.PMLedgerRecordSchemaVersion, ID: input.ID, Kind: input.Kind, Text: input.Text, SourceRefs: input.SourceRefs, ActorKind: input.ActorKind, Status: "active", RecordedAt: input.RecordedAt.Format(time.RFC3339)},
			Actions: []gira.PMRecordAction{{Action: "record:append", Status: "planned", Target: input.ID}}, NextStep: "apply",
		}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runPM([]string{"record", "--repo", "OWNER/repo", "--ticket", "42", "--id", "evidence.1", "--kind", "evidence", "--text", "Five failures", "--source", "log:5", "--at", "2026-07-16T01:02:03Z", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !captured.DryRun || captured.Apply || captured.Ticket != 42 || captured.ID != "evidence.1" || len(captured.SourceRefs) != 1 || captured.RecordedAt.Format(time.RFC3339) != "2026-07-16T01:02:03Z" {
		t.Fatalf("unexpected input: %#v", captured)
	}
	var report gira.PMRecordReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil || report.SchemaVersion != gira.PMRecordReportSchemaVersion {
		t.Fatalf("invalid JSON report: err=%v output=%s", err, stdout.String())
	}
}

func TestRunPMRecordPassesDiscoveryMetadata(t *testing.T) {
	original := newPMRecordReport
	t.Cleanup(func() { newPMRecordReport = original })
	var captured gira.PMRecordInput
	newPMRecordReport = func(input gira.PMRecordInput) (gira.PMRecordReport, error) {
		captured = input
		return gira.PMRecordReport{Command: "pm record", SchemaVersion: gira.PMRecordReportSchemaVersion, Repo: input.Repo.FullName(), Ticket: input.Ticket, DryRun: true, Record: gira.PMLedgerRecord{ID: input.ID, Kind: input.Kind}}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runPM([]string{"record", "--repo", "OWNER/repo", "--ticket", "42", "--id", "learning.1", "--kind", "learning", "--text", "No build", "--source", "experiment:1", "--link", "learned_from=experiment.1", "--goal-ref", "issue:OWNER/repo#100", "--task-profile", "delivery", "--evidence-strength", "qualitative", "--confidence", "medium", "--conclusion", "no_build", "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if len(captured.Links) != 1 || captured.Links[0].Relation != "learned_from" || captured.Links[0].TargetID != "experiment.1" || len(captured.GoalRefs) != 1 || len(captured.TaskProfiles) != 1 || captured.TaskProfiles[0] != "delivery" || captured.EvidenceStrength != "qualitative" || captured.Confidence != "medium" || captured.Conclusion != "no_build" {
		t.Fatalf("discovery fields were not passed: %#v", captured)
	}
	stdout.Reset()
	stderr.Reset()
	code = runPM([]string{"record", "--repo", "OWNER/repo", "--ticket", "42", "--id", "x", "--kind", "learning", "--text", "x", "--source", "x", "--link", "broken", "--dry-run"}, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "relation=target") {
		t.Fatalf("malformed link accepted: code=%d stderr=%s", code, stderr.String())
	}
}

func TestRunPMContextCompactAndBudgetValidation(t *testing.T) {
	original := newPMContextReport
	t.Cleanup(func() { newPMContextReport = original })
	newPMContextReport = func(input gira.PMContextInput) (gira.PMContextReport, error) {
		return gira.PMContextReport{
			Command: "pm context", SchemaVersion: gira.PMContextReportSchemaVersion, ReadOnly: true, Repo: input.Repo.FullName(),
			Issue:   gira.PMContextIssue{Number: input.Ticket, Title: "Ledger"},
			Records: []gira.PMContextRecord{{Current: true, Record: gira.PMLedgerRecord{ID: "decision.1", Kind: "decision", Status: "accepted", Text: "Keep GitHub canonical."}}},
			Summary: gira.PMContextSummary{Records: 1, Current: 1, ByKind: map[string]int{"decision": 1}}, DetailCommand: "gira pm context --json",
		}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runPM([]string{"context", "--repo", "OWNER/repo", "--ticket", "42", "--context-budget", "512"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "decision.1") || !strings.Contains(stdout.String(), "detail:") {
		t.Fatalf("code=%d stderr=%s output=%s", code, stderr.String(), stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = runPM([]string{"context", "--repo", "OWNER/repo", "--ticket", "42", "--context-budget", "100"}, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "between 512 and 20000") {
		t.Fatalf("invalid budget accepted: code=%d stderr=%s", code, stderr.String())
	}
}

func TestRunPMDiscoveryCompactAndJSON(t *testing.T) {
	original := newPMDiscoveryReport
	t.Cleanup(func() { newPMDiscoveryReport = original })
	newPMDiscoveryReport = func(input gira.PMDiscoveryInput) (gira.PMDiscoveryReport, error) {
		return gira.PMDiscoveryReport{
			Command: "pm discovery", SchemaVersion: gira.PMDiscoveryReportSchemaVersion, ReadOnly: true, Repo: input.Repo.FullName(),
			Issue:         gira.PMContextIssue{Number: input.Ticket, Title: "Discovery"},
			Nodes:         []gira.PMDiscoveryNode{{ID: "outcome.1", Kind: "outcome", Current: true, OutcomeState: "observing"}},
			Traces:        []gira.PMDiscoveryTrace{{OutcomeID: "outcome.1", Path: []string{"outcome.1"}}},
			Summary:       gira.PMDiscoverySummary{Nodes: 1, CurrentNodes: 1, Traces: 1, ByKind: map[string]int{"outcome": 1}},
			DetailCommand: "gira pm discovery --json",
		}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runPM([]string{"discovery", "--repo", "OWNER/repo", "--ticket", "42", "--context-budget", "512"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "trace outcome.1") || !strings.Contains(stdout.String(), "detail:") {
		t.Fatalf("compact discovery failed: code=%d stderr=%s output=%s", code, stderr.String(), stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = runPM([]string{"discovery", "--repo", "OWNER/repo", "--ticket", "42", "--json"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), gira.PMDiscoveryReportSchemaVersion) {
		t.Fatalf("JSON discovery failed: code=%d stderr=%s output=%s", code, stderr.String(), stdout.String())
	}
}
