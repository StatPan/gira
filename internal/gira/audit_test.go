package gira

import (
	"os"
	"path/filepath"
	"strings"
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

func TestAppendAuditRecordsRejectsSymlinkLedger(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := os.WriteFile(outside, []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("write outside ledger: %v", err)
	}
	path := filepath.Join(dir, "audit.jsonl")
	if err := os.Symlink(outside, path); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	err := AppendAuditRecords(path, []AuditRecord{
		NewAuditRecord("sync", "sha256:abc", "label:create", "issue#1", "ok", "", "allowed", time.Now()),
	})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
	if got := readText(t, outside); got != "seed\n" {
		t.Fatalf("outside ledger changed through symlink: %q", got)
	}
}

func TestAuditVerifyForRepoScopesToRequestedRepo(t *testing.T) {
	dir := t.TempDir()
	repoA := mustParseRepoRef(t, "StatPan/gira")
	pathA := filepath.Join(dir, "StatPan_gira.jsonl")
	pathB := filepath.Join(dir, "OtherOrg_other.jsonl")

	if err := AppendAuditRecords(pathA, []AuditRecord{
		NewAuditRecord("sync", "sha256:abc", "label:create", "issue#1", "ok", "", "allowed", time.Now()),
	}); err != nil {
		t.Fatalf("append A: %v", err)
	}
	if err := AppendAuditRecords(pathB, []AuditRecord{
		NewAuditRecord("sync", "sha256:def", "label:create", "issue#2", "ok", "", "allowed", time.Now()),
	}); err != nil {
		t.Fatalf("append B: %v", err)
	}

	report := VerifyAuditLedgerForRepo(filepath.Join(dir, "*.jsonl"), repoA)
	if !report.Valid {
		t.Fatalf("expected valid scoped report, got %+v", report)
	}
	if len(report.Files) != 1 || filepath.Base(report.Files[0]) != "StatPan_gira.jsonl" {
		t.Fatalf("unexpected scoped files: %+v", report.Files)
	}
}

func TestAuditVerifyForRepoFailsWhenRepoFileMissing(t *testing.T) {
	dir := t.TempDir()
	repo := mustParseRepoRef(t, "StatPan/gira")
	otherPath := filepath.Join(dir, "OtherOrg_other.jsonl")
	if err := AppendAuditRecords(otherPath, []AuditRecord{
		NewAuditRecord("sync", "sha256:def", "label:create", "issue#2", "ok", "", "allowed", time.Now()),
	}); err != nil {
		t.Fatalf("append other: %v", err)
	}

	report := VerifyAuditLedgerForRepo(filepath.Join(dir, "*.jsonl"), repo)
	if report.Valid {
		t.Fatalf("expected invalid report when repo file missing")
	}
	if report.Failure != "no_audit_files_found" {
		t.Fatalf("failure = %q, want no_audit_files_found", report.Failure)
	}
}

func TestAuditRejectsMissingRequiredField(t *testing.T) {
	rec := NewAuditRecord("sync", "sha256:abc", "label:create", "issue#1", "ok", "", "allowed", time.Now())
	rec.Actor = ""
	if err := validateAuditRecord(rec, true); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestVerifyAuditLedgerResetsChainPerFile(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.jsonl")
	pathB := filepath.Join(dir, "b.jsonl")

	if err := AppendAuditRecords(pathA, []AuditRecord{
		NewAuditRecord("sync", "sha256:abc", "label:create", "issue#1", "ok", "", "allowed", time.Now()),
	}); err != nil {
		t.Fatalf("append A: %v", err)
	}
	if err := AppendAuditRecords(pathB, []AuditRecord{
		NewAuditRecord("sync", "sha256:abc", "label:create", "issue#2", "ok", "", "allowed", time.Now().Add(time.Second)),
	}); err != nil {
		t.Fatalf("append B: %v", err)
	}

	report := VerifyAuditLedger(filepath.Join(dir, "*.jsonl"))
	if !report.Valid {
		t.Fatalf("expected valid report for multi-file ledgers, got %+v", report)
	}
}

func TestComputeAuditHashNoDelimiterCollision(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	recA := AuditRecord{SchemaVersion: AuditSchemaVersion, TS: now, Actor: "a|b", Command: "c", PolicyHash: "sha256:abc", Action: "d", Target: "t", Result: "ok"}
	recB := AuditRecord{SchemaVersion: AuditSchemaVersion, TS: now, Actor: "a", Command: "b|c", PolicyHash: "sha256:abc", Action: "d", Target: "t", Result: "ok"}
	hA := computeAuditHash(recA)
	hB := computeAuditHash(recB)
	if hA == hB {
		t.Fatalf("unexpected hash collision for delimiter-shaped inputs: %s", hA)
	}
	if !strings.HasPrefix(hA, "sha256:") || !strings.HasPrefix(hB, "sha256:") {
		t.Fatalf("unexpected hash prefix: A=%s B=%s", hA, hB)
	}
}

func mustParseRepoRef(t *testing.T, raw string) RepoRef {
	t.Helper()
	repo, err := ParseRepoRef(raw)
	if err != nil {
		t.Fatalf("parse repo %q: %v", raw, err)
	}
	return repo
}
