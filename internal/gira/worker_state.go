package gira

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type WorkerState struct {
	Claim   *WorkerClaim          `json:"claim,omitempty"`
	Handoff *WorkerHandoffPayload `json:"handoff,omitempty"`
}

const (
	workerStateLockStaleAfter = time.Minute
	workerStateLockWait       = 5 * time.Second
	workerStateLockRetryDelay = 10 * time.Millisecond
)

func WorkerStatePath(repo RepoRef, issue int) string {
	if strings.HasSuffix(os.Args[0], ".test") {
		return filepath.Join(os.TempDir(), "gira-worker-test", fmt.Sprintf("%s_%s_issue-%d.json", repo.Owner, repo.Name, issue))
	}
	return filepath.Join(".gira", "workers", fmt.Sprintf("%s_%s", repo.Owner, repo.Name), fmt.Sprintf("issue-%d.json", issue))
}

func ClaimWorkerLease(path string, claim WorkerClaim, now time.Time) error {
	return withWorkerStateLock(path, func() error {
		state, err := readWorkerState(path)
		if err != nil {
			return err
		}
		if state.Claim != nil && state.Claim.Worker != claim.Worker && IsLeaseActive(now, *state.Claim) {
			return fmt.Errorf("claim_conflict")
		}
		state.Claim = &claim
		return writeWorkerState(path, state)
	})
}

func ReleaseWorkerLease(path string, worker string) error {
	return withWorkerStateLock(path, func() error {
		state, err := readWorkerState(path)
		if err != nil {
			return err
		}
		if state.Claim == nil {
			return nil
		}
		if strings.TrimSpace(worker) != "" && state.Claim.Worker != worker {
			return fmt.Errorf("release_denied_not_owner")
		}
		state.Claim = nil
		return writeWorkerState(path, state)
	})
}

func WriteWorkerHandoff(path string, payload WorkerHandoffPayload) error {
	if err := ValidateWorkerHandoffPayload(payload); err != nil {
		return err
	}
	return withWorkerStateLock(path, func() error {
		state, err := readWorkerState(path)
		if err != nil {
			return err
		}
		state.Handoff = &payload
		return writeWorkerState(path, state)
	})
}

func withWorkerStateLock(path string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lockPath := path + ".lock"
	deadline := time.Now().Add(workerStateLockWait)
	for {
		err := os.Mkdir(lockPath, 0o700)
		if err == nil {
			defer func() { _ = os.Remove(lockPath) }()
			return fn()
		}
		if !os.IsExist(err) {
			return err
		}

		lockInfo, statErr := os.Lstat(lockPath)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return statErr
		}
		if lockInfo.Mode()&os.ModeSymlink != 0 || !lockInfo.IsDir() {
			return fmt.Errorf("worker state lock path is not a directory: %s", lockPath)
		}
		if time.Since(lockInfo.ModTime()) > workerStateLockStaleAfter {
			if err := os.Remove(lockPath); err == nil || os.IsNotExist(err) {
				continue
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("worker state lock timeout: %s", path)
		}
		time.Sleep(workerStateLockRetryDelay)
	}
}

func readWorkerState(path string) (WorkerState, error) {
	var state WorkerState
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return state, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(b, &state); err != nil {
		return WorkerState{}, err
	}
	return state, nil
}

func writeWorkerState(path string, state WorkerState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".worker-state-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Chmod(0o644); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(append(b, '\n')); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
