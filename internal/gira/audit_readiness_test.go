package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildAuditReadinessReportWarnsForMissingLedger(t *testing.T) {
	repo := mustParseRepoRef(t, "StatPan/gira")
	report := BuildAuditReadinessReport(repo, filepath.Join(t.TempDir(), "*.jsonl"), readyDoctorRunner(), fixedAuditReadinessTime())

	if !report.Ready {
		t.Fatalf("expected missing ledger warning to remain ready, got %+v", report)
	}
	if report.Audit.Status != AuditReadinessStatusMissing {
		t.Fatalf("audit status = %q, want missing", report.Audit.Status)
	}
	if report.Audit.Verify.Failure != "no_audit_files_found" {
		t.Fatalf("audit failure = %q, want no_audit_files_found", report.Audit.Verify.Failure)
	}
	if !strings.Contains(report.NextStep, "gira status --repo StatPan/gira") {
		t.Fatalf("next step should prefer Gira status, got %q", report.NextStep)
	}
}

func TestBuildAuditReadinessReportWarnsForEmptyLedger(t *testing.T) {
	dir := t.TempDir()
	repo := mustParseRepoRef(t, "StatPan/gira")
	path := filepath.Join(dir, "StatPan_gira.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write empty ledger: %v", err)
	}

	report := BuildAuditReadinessReport(repo, filepath.Join(dir, "*.jsonl"), readyDoctorRunner(), fixedAuditReadinessTime())

	if !report.Ready {
		t.Fatalf("expected empty ledger warning to remain ready, got %+v", report)
	}
	if report.Audit.Status != AuditReadinessStatusMissing {
		t.Fatalf("audit status = %q, want missing", report.Audit.Status)
	}
	if report.Audit.Verify.Failure != "no_audit_records" {
		t.Fatalf("audit failure = %q, want no_audit_records", report.Audit.Verify.Failure)
	}
	if !strings.Contains(report.Audit.Detail, "has no records yet") {
		t.Fatalf("audit detail = %q, want empty-history detail", report.Audit.Detail)
	}
}

func TestBuildAuditReadinessReportFailsForInvalidGlob(t *testing.T) {
	repo := mustParseRepoRef(t, "StatPan/gira")
	report := BuildAuditReadinessReport(repo, "[", readyDoctorRunner(), fixedAuditReadinessTime())

	if report.Ready {
		t.Fatalf("expected invalid glob to fail readiness")
	}
	if report.Audit.Status != AuditReadinessStatusFailed {
		t.Fatalf("audit status = %q, want failed", report.Audit.Status)
	}
	if report.Audit.Verify.Failure != "invalid_audit_glob" {
		t.Fatalf("audit failure = %q, want invalid_audit_glob", report.Audit.Verify.Failure)
	}
}

func TestBuildAuditReadinessReportFailsForInvalidLedger(t *testing.T) {
	dir := t.TempDir()
	repo := mustParseRepoRef(t, "StatPan/gira")
	path := filepath.Join(dir, "StatPan_gira.jsonl")
	if err := os.WriteFile(path, []byte("{not-json}\n"), 0o644); err != nil {
		t.Fatalf("write invalid ledger: %v", err)
	}

	report := BuildAuditReadinessReport(repo, filepath.Join(dir, "*.jsonl"), readyDoctorRunner(), fixedAuditReadinessTime())

	if report.Ready {
		t.Fatalf("expected invalid ledger to fail readiness")
	}
	if report.Audit.Status != AuditReadinessStatusFailed {
		t.Fatalf("audit status = %q, want failed", report.Audit.Status)
	}
	if report.Audit.Verify.Failure != "malformed_json" {
		t.Fatalf("audit failure = %q, want malformed_json", report.Audit.Verify.Failure)
	}
	if !strings.Contains(report.NextStep, "gira audit verify --repo StatPan/gira") {
		t.Fatalf("next step should point to audit verify, got %q", report.NextStep)
	}
}

func TestBuildAuditReadinessReportPassesForHealthyLedger(t *testing.T) {
	dir := t.TempDir()
	repo := mustParseRepoRef(t, "StatPan/gira")
	path := filepath.Join(dir, "StatPan_gira.jsonl")
	if err := AppendAuditRecords(path, []AuditRecord{
		NewAuditRecord("sync", "sha256:abc", "label:create", "issue#1", "ok", "", "allowed", fixedAuditReadinessTime()),
	}); err != nil {
		t.Fatalf("append audit: %v", err)
	}

	report := BuildAuditReadinessReport(repo, filepath.Join(dir, "*.jsonl"), readyDoctorRunner(), fixedAuditReadinessTime())

	if !report.Ready {
		t.Fatalf("expected healthy ledger readiness, got %+v", report)
	}
	if report.Audit.Status != AuditReadinessStatusOK {
		t.Fatalf("audit status = %q, want ok", report.Audit.Status)
	}
	if report.Audit.Verify.Records != 1 {
		t.Fatalf("records = %d, want 1", report.Audit.Verify.Records)
	}
	if report.NextStep != "gira status --repo StatPan/gira" {
		t.Fatalf("next step = %q, want gira status", report.NextStep)
	}
}

func TestFormatAuditReadinessReportSeparatesDoctorAndAuditSections(t *testing.T) {
	repo := mustParseRepoRef(t, "StatPan/gira")
	report := BuildAuditReadinessReport(repo, filepath.Join(t.TempDir(), "*.jsonl"), readyDoctorRunner(), fixedAuditReadinessTime())
	out := FormatAuditReadinessReport(report)

	for _, want := range []string{"readiness/doctor checks:", "audit ledger health:", "[warn] audit_ledger"} {
		if !strings.Contains(out, want) {
			t.Fatalf("formatted report missing %q:\n%s", want, out)
		}
	}
}

func fixedAuditReadinessTime() time.Time {
	return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
}
