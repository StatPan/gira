package gira

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAuditVerifyPassesAndTamperFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	records := []AuditRecord{
		NewAuditRecord("sync", "sha256:abc", "label:create", "issue#1", "ok", "", "allowed", time.Now()),
		NewAuditRecord("sync", "sha256:abc", "label:update", "issue#2", "ok", "", "allowed", time.Now().Add(time.Second)),
	}
	if err := AppendAuditRecords(path, records); err != nil {
		t.Fatalf("append: %v", err)
	}
	report := VerifyAuditLedger(path)
	if !report.Valid {
		t.Fatalf("expected valid ledger, got %+v", report)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	b[10] = 'X'
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	report = VerifyAuditLedger(path)
	if report.Valid {
		t.Fatalf("expected invalid ledger after tamper")
	}
}

func TestAuditRejectsMissingRequiredField(t *testing.T) {
	rec := NewAuditRecord("sync", "sha256:abc", "label:create", "issue#1", "ok", "", "allowed", time.Now())
	rec.Actor = ""
	if err := validateAuditRecord(rec, true); err == nil {
		t.Fatalf("expected validation error")
	}
}
