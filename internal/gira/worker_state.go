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

func WorkerStatePath(repo RepoRef, issue int) string {
	if strings.HasSuffix(os.Args[0], ".test") {
		return filepath.Join(os.TempDir(), "gira-worker-test", fmt.Sprintf("%s_%s_issue-%d.json", repo.Owner, repo.Name, issue))
	}
	return filepath.Join(".gira", "workers", fmt.Sprintf("%s_%s", repo.Owner, repo.Name), fmt.Sprintf("issue-%d.json", issue))
}

func ClaimWorkerLease(path string, claim WorkerClaim, now time.Time) error {
	state, _ := readWorkerState(path)
	if state.Claim != nil && state.Claim.Worker != claim.Worker && IsLeaseActive(now, *state.Claim) {
		return fmt.Errorf("claim_conflict")
	}
	state.Claim = &claim
	return writeWorkerState(path, state)
}

func ReleaseWorkerLease(path string, worker string) error {
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
}

func WriteWorkerHandoff(path string, payload WorkerHandoffPayload) error {
	if err := ValidateWorkerHandoffPayload(payload); err != nil {
		return err
	}
	state, _ := readWorkerState(path)
	state.Handoff = &payload
	return writeWorkerState(path, state)
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
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
