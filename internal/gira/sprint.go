package gira

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type SprintState struct {
	Repo       string             `json:"repo"`
	Iterations map[string]Sprint  `json:"iterations"`
}

type Sprint struct {
	Iteration            string   `json:"iteration"`
	CapacityTarget       int      `json:"capacity_target"`
	CommittedItems       []int    `json:"committed_items"`
	CompletedItems       []int    `json:"completed_items"`
	SpilloverItems       []int    `json:"spillover_items"`
	RolloverReason       string   `json:"rollover_reason"`
	SpilloverDisposition string   `json:"spillover_disposition"`
	CommitmentFrozen     bool     `json:"commitment_frozen"`
	StartedAt            string   `json:"started_at,omitempty"`
	ClosedAt             string   `json:"closed_at,omitempty"`
}

type SprintPlanReport struct {
	Repo         string `json:"repo"`
	Iteration    string `json:"iteration"`
	Mode         string `json:"mode"`
	Capacity     int    `json:"capacity_target"`
	CommitCount  int    `json:"commit_count"`
	CapacityBreach bool `json:"capacity_breach"`
	Sprint       Sprint `json:"sprint"`
}

type SprintStartReport struct {
	Repo      string `json:"repo"`
	Iteration string `json:"iteration"`
	Mode      string `json:"mode"`
	Frozen    bool   `json:"commitment_frozen"`
	StartedAt string `json:"started_at"`
}

type SprintCloseReport struct {
	Repo      string `json:"repo"`
	Iteration string `json:"iteration"`
	Mode      string `json:"mode"`
	Summary   Sprint `json:"summary"`
}

func SprintStatePath(repo RepoRef) string {
	return filepath.Join(".gira", "sprints", strings.ReplaceAll(strings.ToLower(repo.FullName()), "/", "-"), "state.json")
}

func PlanSprint(path string, repo RepoRef, iteration string, capacity int, committed []int, apply bool) (SprintPlanReport, error) {
	if strings.TrimSpace(iteration) == "" {
		return SprintPlanReport{}, fmt.Errorf("iteration is required")
	}
	if capacity <= 0 {
		return SprintPlanReport{}, fmt.Errorf("capacity must be > 0")
	}
	state, err := loadSprintState(path, repo)
	if err != nil {
		return SprintPlanReport{}, err
	}
	sprint := state.Iterations[iteration]
	if sprint.CommitmentFrozen {
		return SprintPlanReport{}, fmt.Errorf("commitment freeze is active for iteration %s", iteration)
	}
	committed = normalizeInts(committed)
	sprint = Sprint{Iteration: iteration, CapacityTarget: capacity, CommittedItems: committed, CompletedItems: sprint.CompletedItems, SpilloverItems: sprint.SpilloverItems, RolloverReason: sprint.RolloverReason, SpilloverDisposition: sprint.SpilloverDisposition, CommitmentFrozen: false, StartedAt: sprint.StartedAt, ClosedAt: sprint.ClosedAt}
	breach := len(committed) > capacity
	mode := "dry-run"
	if apply {
		mode = "apply"
		state.Iterations[iteration] = sprint
		if err := saveSprintState(path, state); err != nil {
			return SprintPlanReport{}, err
		}
	}
	return SprintPlanReport{Repo: repo.FullName(), Iteration: iteration, Mode: mode, Capacity: capacity, CommitCount: len(committed), CapacityBreach: breach, Sprint: sprint}, nil
}

func StartSprint(path string, repo RepoRef, iteration string, apply bool, now time.Time) (SprintStartReport, error) {
	state, err := loadSprintState(path, repo)
	if err != nil {
		return SprintStartReport{}, err
	}
	sprint, ok := state.Iterations[iteration]
	if !ok {
		return SprintStartReport{}, fmt.Errorf("iteration %s has no plan", iteration)
	}
	if sprint.ClosedAt != "" {
		return SprintStartReport{}, fmt.Errorf("iteration %s already closed", iteration)
	}
	started := now.UTC().Format(time.RFC3339)
	mode := "dry-run"
	if apply {
		mode = "apply"
		sprint.CommitmentFrozen = true
		sprint.StartedAt = started
		state.Iterations[iteration] = sprint
		if err := saveSprintState(path, state); err != nil {
			return SprintStartReport{}, err
		}
	}
	return SprintStartReport{Repo: repo.FullName(), Iteration: iteration, Mode: mode, Frozen: true, StartedAt: started}, nil
}

func CloseSprint(path string, repo RepoRef, iteration string, completed []int, disposition string, reason string, apply bool, now time.Time) (SprintCloseReport, error) {
	state, err := loadSprintState(path, repo)
	if err != nil {
		return SprintCloseReport{}, err
	}
	sprint, ok := state.Iterations[iteration]
	if !ok {
		return SprintCloseReport{}, fmt.Errorf("iteration %s has no plan", iteration)
	}
	if !sprint.CommitmentFrozen {
		return SprintCloseReport{}, fmt.Errorf("iteration %s has not started; commitment freeze required", iteration)
	}
	disposition = strings.TrimSpace(strings.ToLower(disposition))
	if disposition != "carry" && disposition != "drop" {
		return SprintCloseReport{}, fmt.Errorf("spillover disposition must be carry or drop")
	}
	if strings.TrimSpace(reason) == "" {
		return SprintCloseReport{}, fmt.Errorf("rollover reason is required")
	}
	completed = normalizeInts(completed)
	spill := diffInts(sprint.CommittedItems, completed)
	summary := sprint
	summary.CompletedItems = completed
	summary.SpilloverItems = spill
	summary.RolloverReason = strings.TrimSpace(reason)
	summary.SpilloverDisposition = disposition
	summary.ClosedAt = now.UTC().Format(time.RFC3339)
	mode := "dry-run"
	if apply {
		mode = "apply"
		state.Iterations[iteration] = summary
		if err := saveSprintState(path, state); err != nil {
			return SprintCloseReport{}, err
		}
	}
	return SprintCloseReport{Repo: repo.FullName(), Iteration: iteration, Mode: mode, Summary: summary}, nil
}

func loadSprintState(path string, repo RepoRef) (SprintState, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SprintState{Repo: repo.FullName(), Iterations: map[string]Sprint{}}, nil
		}
		return SprintState{}, err
	}
	var state SprintState
	if err := json.Unmarshal(content, &state); err != nil {
		return SprintState{}, fmt.Errorf("parse sprint state: %w", err)
	}
	if state.Iterations == nil {
		state.Iterations = map[string]Sprint{}
	}
	if state.Repo == "" {
		state.Repo = repo.FullName()
	}
	return state, nil
}

func saveSprintState(path string, state SprintState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o644)
}

func normalizeInts(values []int) []int {
	seen := map[int]struct{}{}
	out := make([]int, 0, len(values))
	for _, v := range values {
		if v <= 0 {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}

func diffInts(all []int, completed []int) []int {
	completedSet := map[int]struct{}{}
	for _, n := range completed {
		completedSet[n] = struct{}{}
	}
out := make([]int, 0)
	for _, n := range normalizeInts(all) {
		if _, ok := completedSet[n]; !ok {
			out = append(out, n)
		}
	}
	return out
}
