package gira

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestClaimWorkerLeaseAllowsRenewalAndExpiredReplacement(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()

	renewPath := filepath.Join(dir, "renew.json")
	first := WorkerClaim{Repo: "StatPan/gira", IssueNumber: 72, Worker: "alice", LeaseUntilUTC: now.Add(time.Minute), Version: "v1"}
	if err := ClaimWorkerLease(renewPath, first, now); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	renewed := first
	renewed.LeaseUntilUTC = now.Add(2 * time.Minute)
	if err := ClaimWorkerLease(renewPath, renewed, now.Add(10*time.Second)); err != nil {
		t.Fatalf("renewal claim: %v", err)
	}
	state, err := readWorkerState(renewPath)
	if err != nil {
		t.Fatalf("read renewed state: %v", err)
	}
	if state.Claim == nil || !state.Claim.LeaseUntilUTC.Equal(renewed.LeaseUntilUTC) {
		t.Fatalf("renewed state = %+v, want lease until %v", state.Claim, renewed.LeaseUntilUTC)
	}

	expiredPath := filepath.Join(dir, "expired.json")
	expired := WorkerClaim{Repo: "StatPan/gira", IssueNumber: 72, Worker: "alice", LeaseUntilUTC: now.Add(-time.Minute), Version: "v1"}
	if err := ClaimWorkerLease(expiredPath, expired, now); err != nil {
		t.Fatalf("expired claim: %v", err)
	}
	replacement := expired
	replacement.Worker = "bob"
	replacement.LeaseUntilUTC = now.Add(time.Minute)
	if err := ClaimWorkerLease(expiredPath, replacement, now); err != nil {
		t.Fatalf("replace expired claim: %v", err)
	}
}

func TestClaimWorkerLeaseAllowsOnlyOneConcurrentProcess(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	startGate := filepath.Join(dir, "start")
	commands := make([]*exec.Cmd, 0, 2)
	stderrs := make([]*bytes.Buffer, 0, 2)
	for _, worker := range []string{"alice", "bob"} {
		cmd := exec.Command(os.Args[0], "-test.run", "^TestWorkerLeaseClaimHelper$", "--")
		cmd.Env = append(os.Environ(),
			"GIRA_WORKER_CLAIM_HELPER=1",
			"GIRA_WORKER_CLAIM_PATH="+statePath,
			"GIRA_WORKER_CLAIM_START="+startGate,
			"GIRA_WORKER_CLAIM_WORKER="+worker,
		)
		stderr := &bytes.Buffer{}
		cmd.Stderr = stderr
		commands = append(commands, cmd)
		stderrs = append(stderrs, stderr)
		if err := cmd.Start(); err != nil {
			t.Fatalf("start %s claimant: %v", worker, err)
		}
	}
	if err := os.WriteFile(startGate, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	successes := 0
	conflicts := 0
	for i, cmd := range commands {
		err := cmd.Wait()
		if err == nil {
			successes++
			continue
		}
		if strings.Contains(stderrs[i].String(), "claim_conflict") {
			conflicts++
			continue
		}
		t.Fatalf("claimant %d failed: %v (%s)", i, err, stderrs[i].String())
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent claim results = successes %d, conflicts %d; want exactly one of each", successes, conflicts)
	}
}

func TestWorkerLeaseClaimHelper(t *testing.T) {
	if os.Getenv("GIRA_WORKER_CLAIM_HELPER") != "1" {
		return
	}
	path := os.Getenv("GIRA_WORKER_CLAIM_PATH")
	startGate := os.Getenv("GIRA_WORKER_CLAIM_START")
	worker := os.Getenv("GIRA_WORKER_CLAIM_WORKER")
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(startGate); err == nil {
			break
		} else if !os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(3)
		}
		if time.Now().After(deadline) {
			fmt.Fprintln(os.Stderr, "claim helper start gate timeout")
			os.Exit(3)
		}
		time.Sleep(time.Millisecond)
	}
	claim := WorkerClaim{
		Repo:          "StatPan/gira",
		IssueNumber:   72,
		Worker:        worker,
		LeaseUntilUTC: time.Now().UTC().Add(time.Minute),
		Version:       "v1",
	}
	if err := ClaimWorkerLease(path, claim, time.Now().UTC()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(0)
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

func TestWorkerStateLockCleansUpAfterError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	claim := WorkerClaim{Repo: "StatPan/gira", IssueNumber: 72, Worker: "alice", LeaseUntilUTC: time.Now().UTC().Add(time.Minute), Version: "v1"}
	if err := ClaimWorkerLease(path, claim, time.Now().UTC()); err == nil {
		t.Fatal("expected read error for state directory")
	}
	if _, err := os.Lstat(path + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("lock path error = %v, want lock cleanup", err)
	}
}

func TestWorkerStateLockCleansUpStaleLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	lockPath := path + ".lock"
	if err := os.Mkdir(lockPath, 0o700); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * workerStateLockStaleAfter)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatal(err)
	}
	claim := WorkerClaim{Repo: "StatPan/gira", IssueNumber: 72, Worker: "alice", LeaseUntilUTC: time.Now().UTC().Add(time.Minute), Version: "v1"}
	if err := ClaimWorkerLease(path, claim, time.Now().UTC()); err != nil {
		t.Fatalf("claim after stale lock: %v", err)
	}
}
