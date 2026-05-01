package gira

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSprintFlowPlanStartClose(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	statePath := filepath.Join(t.TempDir(), "state.json")

	plan, err := PlanSprint(statePath, repo, "2026-W18", 2, []int{11, 12, 13}, true)
	if err != nil {
		t.Fatalf("PlanSprint error: %v", err)
	}
	if !plan.CapacityBreach {
		t.Fatalf("expected capacity breach true")
	}

	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if _, err := StartSprint(statePath, repo, "2026-W18", true, now); err != nil {
		t.Fatalf("StartSprint error: %v", err)
	}

	closeReport, err := CloseSprint(statePath, repo, "2026-W18", []int{11}, "carry", "dependency blocked", true, now)
	if err != nil {
		t.Fatalf("CloseSprint error: %v", err)
	}
	if len(closeReport.Summary.SpilloverItems) != 2 || closeReport.Summary.SpilloverItems[0] != 12 || closeReport.Summary.SpilloverItems[1] != 13 {
		t.Fatalf("unexpected spillover items: %#v", closeReport.Summary.SpilloverItems)
	}
}

func TestPlanRespectsFreeze(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	statePath := filepath.Join(t.TempDir(), "state.json")
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	_, _ = PlanSprint(statePath, repo, "2026-W18", 3, []int{1, 2}, true)
	_, _ = StartSprint(statePath, repo, "2026-W18", true, now)
	if _, err := PlanSprint(statePath, repo, "2026-W18", 3, []int{1, 2, 3}, true); err == nil {
		t.Fatal("expected freeze error")
	}
}
