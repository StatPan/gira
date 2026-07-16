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
