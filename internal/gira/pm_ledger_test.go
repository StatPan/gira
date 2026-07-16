package gira

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

type pmLedgerRunner struct {
	context []byte
	calls   []string
	posted  []string
}

func (r *pmLedgerRunner) Run(name string, args ...string) ([]byte, error) {
	call := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, call)
	if call == "gh issue view 42 --repo OWNER/repo --json number,title,body,url,comments" {
		return r.context, nil
	}
	if len(args) >= 7 && name == "gh" && args[0] == "issue" && args[1] == "comment" && args[2] == "42" {
		r.posted = append(r.posted, args[6])
		return []byte(`{"url":"https://example/comment"}`), nil
	}
	return nil, fmt.Errorf("unexpected call: %s", call)
}

func TestPMLedgerRecordRoundTripAndLifecycleDefaults(t *testing.T) {
	record, diagnostics := normalizePMLedgerRecord(PMRecordInput{
		ID: "assumption.setup", Kind: "assumption", Text: "Setup friction drives abandonment.",
		SourceRefs: []string{"issue:OWNER/repo#42", "metric:onboarding"}, ActorKind: "ai",
		RecordedAt: time.Date(2026, 7, 16, 1, 2, 3, 0, time.UTC),
	})
	if len(diagnostics) != 0 || record.Status != "proposed" || record.SchemaVersion != PMLedgerRecordSchemaVersion {
		t.Fatalf("unexpected normalized record: %#v diagnostics=%#v", record, diagnostics)
	}
	parsed, found, err := ParsePMLedgerRecordComment(RenderPMLedgerRecordComment(record))
	if err != nil || !found || !samePMLedgerSemantics(record, parsed) || parsed.RecordedAt != record.RecordedAt {
		t.Fatalf("record round trip failed: found=%t err=%v parsed=%#v", found, err, parsed)
	}
}

func TestBuildPMRecordDryRunApplyAndIdempotentRetry(t *testing.T) {
	now := time.Date(2026, 7, 16, 1, 2, 3, 0, time.UTC)
	input := PMRecordInput{Repo: ParseRepoRefMust("OWNER/repo"), Ticket: 42, ID: "evidence.1", Kind: "evidence", Text: "Five setup failures were observed.", SourceRefs: []string{"log:run-5"}, ActorKind: "human", RecordedAt: now}
	empty := &pmLedgerRunner{context: pmLedgerContextJSON(t, "", nil)}
	dry := input
	dry.DryRun = true
	report, err := BuildPMRecordReport(dry, empty)
	if err != nil {
		t.Fatal(err)
	}
	if report.Approval == nil || len(empty.posted) != 0 || report.Actions[0].Status != "planned" {
		t.Fatalf("unexpected dry-run report: %#v posted=%#v", report, empty.posted)
	}
	applyRunner := &pmLedgerRunner{context: pmLedgerContextJSON(t, "", nil)}
	apply := input
	apply.Apply = true
	applied, err := BuildPMRecordReport(apply, applyRunner)
	if err != nil || len(applyRunner.posted) != 1 || applied.Actions[0].Status != "applied" {
		t.Fatalf("unexpected apply: report=%#v posted=%#v err=%v", applied, applyRunner.posted, err)
	}
	existing := []pmLedgerTestComment{{Body: applyRunner.posted[0], URL: "https://example/c1", Author: "pm", CreatedAt: now.Format(time.RFC3339)}}
	retryRunner := &pmLedgerRunner{context: pmLedgerContextJSON(t, "", existing)}
	retry := input
	retry.Apply = true
	retry.RecordedAt = now.Add(time.Hour)
	retried, err := BuildPMRecordReport(retry, retryRunner)
	if err != nil || !retried.Idempotent || len(retryRunner.posted) != 0 || retried.Actions[0].Status != "skipped" {
		t.Fatalf("retry was not idempotent: report=%#v posted=%#v err=%v", retried, retryRunner.posted, err)
	}
}

func TestBuildPMRecordRejectsConflictingIDAndMissingSupersession(t *testing.T) {
	prior := PMLedgerRecord{SchemaVersion: PMLedgerRecordSchemaVersion, ID: "decision.ui", Kind: "decision", Text: "Use text output.", ActorKind: "human", RecordedAt: "2026-07-16T01:00:00Z", Status: "accepted", SourceRefs: []string{"issue:42"}}
	comments := []pmLedgerTestComment{{Body: RenderPMLedgerRecordComment(prior)}}
	runner := &pmLedgerRunner{context: pmLedgerContextJSON(t, "", comments)}
	_, err := BuildPMRecordReport(PMRecordInput{Repo: ParseRepoRefMust("OWNER/repo"), Ticket: 42, ID: "decision.ui", Kind: "decision", Text: "Use HTML output.", SourceRefs: []string{"issue:42"}, ActorKind: "human", Status: "accepted", Apply: true}, runner)
	if err == nil || len(runner.posted) != 0 {
		t.Fatalf("conflicting ID should fail closed: err=%v posted=%#v", err, runner.posted)
	}
	runner = &pmLedgerRunner{context: pmLedgerContextJSON(t, "", comments)}
	report, err := BuildPMRecordReport(PMRecordInput{Repo: ParseRepoRefMust("OWNER/repo"), Ticket: 42, ID: "decision.ui.2", Kind: "decision", Text: "Use HTML output.", SourceRefs: []string{"issue:42"}, ActorKind: "human", Status: "accepted", Supersedes: "missing", DryRun: true}, runner)
	if err == nil || !hasPMLedgerDiagnostic(report.Diagnostics, PMLedgerDiagnosticMissingSupersession) {
		t.Fatalf("missing supersession target was not diagnosed: report=%#v err=%v", report, err)
	}
}

func TestBuildPMContextResolvesSupersessionAndLegacyEvidence(t *testing.T) {
	first := PMLedgerRecord{SchemaVersion: PMLedgerRecordSchemaVersion, ID: "assumption.1", Kind: "assumption", Text: "Setup is hard.", ActorKind: "ai", RecordedAt: "2026-07-15T01:00:00Z", Status: "testing", SourceRefs: []string{"research:1"}}
	second := PMLedgerRecord{SchemaVersion: PMLedgerRecordSchemaVersion, ID: "assumption.2", Kind: "assumption", Text: "Setup is hard for first-time users.", ActorKind: "human", RecordedAt: "2026-07-16T01:00:00Z", Status: "supported", Supersedes: first.ID, SourceRefs: []string{"research:2"}}
	comments := []pmLedgerTestComment{
		{Body: RenderPMLedgerRecordComment(first)},
		{Body: "## Decision\nKeep legacy notes readable.", URL: "https://example/legacy"},
		{Body: RenderPMLedgerRecordComment(second)},
	}
	runner := &pmLedgerRunner{context: pmLedgerContextJSON(t, "", comments)}
	report, err := BuildPMContextReport(PMContextInput{Repo: ParseRepoRefMust("OWNER/repo"), Ticket: 42}, runner)
	if err != nil {
		t.Fatal(err)
	}
	if !report.ReadOnly || report.Summary.Current != 1 || report.Summary.Superseded != 1 || report.Summary.LegacyEvidence != 1 || !report.Records[1].Current {
		t.Fatalf("unexpected resolved context: %#v", report)
	}
	compact := FormatPMContext(report, 700)
	if len(compact) > 700 || strings.Contains(compact, first.Text) || !strings.Contains(compact, "assumption.2") || !strings.Contains(compact, "detail:") {
		t.Fatalf("compact context violated budget or current-state rules: bytes=%d\n%s", len(compact), compact)
	}
	report.DetailCommand = "gira pm context --repo " + strings.Repeat("very-long/", 200) + " --json"
	if compact = FormatPMContext(report, 512); len(compact) > 512 || !strings.Contains(compact, "detail:") {
		t.Fatalf("fallback context exceeded hard budget: bytes=%d\n%s", len(compact), compact)
	}
}

func TestPMRecordRejectsSecretsAndPrivateTranscripts(t *testing.T) {
	for _, text := range []string{"api_key=super-secret-value", "Private transcript: customer said hidden things"} {
		record, diagnostics := normalizePMLedgerRecord(PMRecordInput{ID: "evidence.secret", Kind: "evidence", Text: text, ActorKind: "human"})
		if record.Text == "" || !hasPMLedgerDiagnostic(diagnostics, PMLedgerDiagnosticSensitiveContent) {
			t.Fatalf("sensitive content was not rejected: %#v", diagnostics)
		}
	}
	report, err := BuildPMRecordReport(PMRecordInput{Repo: ParseRepoRefMust("OWNER/repo"), Ticket: 42, ID: "evidence.secret", Kind: "evidence", Text: "api_key=super-secret-value", SourceRefs: []string{"issue:42"}, ActorKind: "human", DryRun: true}, &pmLedgerRunner{})
	if err == nil || strings.Contains(report.Record.Text, "super-secret-value") || report.Record.Text != "[redacted: sensitive PM ledger content]" {
		t.Fatalf("validation report leaked sensitive input: record=%#v err=%v", report.Record, err)
	}
}

func TestPMContextRedactsTypedAndLegacySensitiveContent(t *testing.T) {
	record := PMLedgerRecord{SchemaVersion: PMLedgerRecordSchemaVersion, ID: "evidence.secret", Kind: "evidence", Text: "api_key=super-secret-value", SourceRefs: []string{"issue:42"}, ActorKind: "human", RecordedAt: "2026-07-16T01:00:00Z", Status: "active"}
	comments := []pmLedgerTestComment{
		{Body: RenderPMLedgerRecordComment(record)},
		{Body: "## Decision\npassword=also-secret"},
	}
	runner := &pmLedgerRunner{context: pmLedgerContextJSON(t, "", comments)}
	report, err := BuildPMContextReport(PMContextInput{Repo: ParseRepoRefMust("OWNER/repo"), Ticket: 42}, runner)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(report)
	if strings.Contains(string(encoded), "super-secret-value") || strings.Contains(string(encoded), "also-secret") || len(report.LegacyEvidence) != 0 || report.Records[0].Record.Text != "[redacted: sensitive PM ledger content]" {
		t.Fatalf("context leaked restricted content: %s", encoded)
	}
	if !hasPMLedgerDiagnostic(report.Diagnostics, PMLedgerDiagnosticSensitiveContent) {
		t.Fatalf("missing sensitive-content diagnostic: %#v", report.Diagnostics)
	}
}

type pmLedgerTestComment struct {
	Body      string
	URL       string
	Author    string
	CreatedAt string
}

func pmLedgerContextJSON(t *testing.T, body string, comments []pmLedgerTestComment) []byte {
	t.Helper()
	items := make([]map[string]any, 0, len(comments))
	for _, comment := range comments {
		items = append(items, map[string]any{"body": comment.Body, "url": comment.URL, "createdAt": comment.CreatedAt, "author": map[string]string{"login": comment.Author}})
	}
	value := map[string]any{"number": 42, "title": "PM ledger", "body": body, "url": "https://example/issues/42", "comments": items}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func hasPMLedgerDiagnostic(diagnostics []PMLedgerDiagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
