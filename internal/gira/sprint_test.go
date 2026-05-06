package gira

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
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

func TestSprintRolloverDryRunDetectsCandidatesAndTarget(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	client := sprintFakeStatusClient{repo: repo, pages: map[string]string{
		"api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100": `[[{"number":1,"title":"W17","state":"closed","due_on":"2026-04-25T00:00:00Z","open_issues":2,"closed_issues":10},{"number":2,"title":"W18","state":"open","due_on":"2026-05-10T00:00:00Z","open_issues":5,"closed_issues":1}]]`,
		"api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100":     `[[{"number":10,"title":"carry me","state":"open","labels":[],"milestone":{"title":"W17"},"updated_at":"2026-05-01T00:00:00Z","html_url":"u"},{"number":11,"title":"done","state":"closed","labels":[],"milestone":{"title":"W17"},"updated_at":"2026-05-01T00:00:00Z","html_url":"u"}]]`,
	}}
	report, err := SprintRolloverForClient(client, &sprintFakeRunner{}, "", false, now)
	if err != nil {
		t.Fatalf("SprintRolloverForClient error: %v", err)
	}
	if report.Summary.Candidates != 1 || report.Summary.Applied != 1 || report.Summary.Skipped != 0 {
		t.Fatalf("unexpected summary: %#v", report.Summary)
	}
	if report.TargetMilestone == nil || report.TargetMilestone.Title != "W18" {
		t.Fatalf("unexpected target: %#v", report.TargetMilestone)
	}
	if len(report.Items) != 1 || report.Items[0].Action != "would-apply" {
		t.Fatalf("unexpected items: %#v", report.Items)
	}
	if report.Items[0].LifecycleStatus != "open" || report.Items[0].NextStep != "gira ticket status --repo StatPan/gira --ticket 10" {
		t.Fatalf("missing lifecycle evidence: %#v", report.Items[0])
	}
}

func TestSprintRolloverSkipsDoneLifecycleTickets(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	client := sprintFakeStatusClient{repo: repo, pages: map[string]string{
		"api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100": `[[{"number":1,"title":"W17","state":"closed","due_on":"2026-04-25T00:00:00Z","open_issues":1,"closed_issues":1},{"number":2,"title":"W18","state":"open","due_on":"2026-05-10T00:00:00Z","open_issues":2,"closed_issues":0}]]`,
		"api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100":     `[[{"number":10,"title":"already done","state":"open","labels":[{"name":"status:done"}],"milestone":{"title":"W17"},"updated_at":"2026-05-01T00:00:00Z","html_url":"u"}]]`,
	}}

	report, err := SprintRolloverForClient(client, &sprintFakeRunner{}, "", false, now)
	if err != nil {
		t.Fatalf("SprintRolloverForClient error: %v", err)
	}
	if report.Summary.Candidates != 1 || report.Summary.Applied != 0 || report.Summary.Skipped != 1 {
		t.Fatalf("unexpected summary: %#v", report.Summary)
	}
	if report.Items[0].Action != "skipped" || report.Items[0].SkipReason != "ticket already has status:done" {
		t.Fatalf("unexpected item: %#v", report.Items[0])
	}
}

func TestSprintRolloverApplyCallsMilestonePatch(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	runner := &sprintFakeRunner{}
	client := sprintFakeStatusClient{repo: repo, pages: map[string]string{
		"api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100": `[[{"number":1,"title":"W17","state":"closed","due_on":"2026-04-25T00:00:00Z","open_issues":1,"closed_issues":1},{"number":2,"title":"W18","state":"open","due_on":"2026-05-10T00:00:00Z","open_issues":2,"closed_issues":0}]]`,
		"api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100":     `[[{"number":10,"title":"carry me","state":"open","labels":[],"milestone":{"title":"W17"},"updated_at":"2026-05-01T00:00:00Z","html_url":"u"}]]`,
	}}
	report, err := SprintRolloverForClient(client, runner, "W18", true, now)
	if err != nil {
		t.Fatalf("SprintRolloverForClient error: %v", err)
	}
	if report.Summary.Applied != 1 {
		t.Fatalf("applied=%d, want 1", report.Summary.Applied)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("runner calls=%v, want 1", runner.calls)
	}
	if !strings.Contains(runner.calls[0], "repos/StatPan/gira/issues/10") || !strings.Contains(runner.calls[0], "milestone=2") {
		t.Fatalf("unexpected patch call: %s", runner.calls[0])
	}
}

type sprintFakeStatusClient struct {
	repo  RepoRef
	pages map[string]string
}

func (f sprintFakeStatusClient) Repo() RepoRef { return f.repo }

func (f sprintFakeStatusClient) JSON(args []string, target any) error {
	key := strings.Join(args, " ")
	payload, ok := f.pages[key]
	if !ok {
		return fmt.Errorf("missing response for %q", key)
	}
	return json.Unmarshal([]byte(payload), target)
}

type sprintFakeRunner struct{ calls []string }

func (f *sprintFakeRunner) Run(name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	if name == "gh" && len(args) >= 2 && args[0] == "api" && strings.Contains(args[1], "/issues/") && hasString(args, "PATCH") {
		return []byte(`{}`), nil
	}
	return nil, fmt.Errorf("unexpected call: %s %s", name, strings.Join(args, " "))
}

func hasString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
