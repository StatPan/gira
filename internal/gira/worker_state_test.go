package gira

import (
	"path/filepath"
	"testing"
	"time"
)

func TestClaimWorkerLeaseConflictsWhenOtherActive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	now := time.Now().UTC()

	first := WorkerClaim{Repo: "StatPan/gira", IssueNumber: 72, Worker: "alice", LeaseUntilUTC: now.Add(10 * time.Minute), Version: "v1"}
	if err := ClaimWorkerLease(path, first, now); err != nil {
		t.Fatalf("first claim: %v", err)
	}

	second := WorkerClaim{Repo: "StatPan/gira", IssueNumber: 72, Worker: "bob", LeaseUntilUTC: now.Add(10 * time.Minute), Version: "v1"}
	if err := ClaimWorkerLease(path, second, now); err == nil {
		t.Fatalf("expected claim conflict")
	}
}

func TestReleaseWorkerLeaseOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	now := time.Now().UTC()
	claim := WorkerClaim{Repo: "StatPan/gira", IssueNumber: 72, Worker: "alice", LeaseUntilUTC: now.Add(10 * time.Minute), Version: "v1"}
	if err := ClaimWorkerLease(path, claim, now); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if err := ReleaseWorkerLease(path, "bob"); err == nil {
		t.Fatalf("expected release denied")
	}
	if err := ReleaseWorkerLease(path, "alice"); err != nil {
		t.Fatalf("release owner: %v", err)
	}
}
