package gira

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestBuildDetachReportPlansDeterministicSafeActions(t *testing.T) {
	client := &fakeDetachClient{
		repo: mustRepo(t, "StatPan/gira"),
		labels: []ExistingLabel{
			{Name: BootstrapLabel, Color: "5319E7", Description: "Created or managed by Gira bootstrap metadata sync."},
			{Name: "type:task", Color: "1D76DB", Description: "Concrete implementation task."},
			{Name: "status:ready", Color: "000000", Description: "User changed."},
		},
		labelUseCounts: map[string]int{"type:task": 1},
		milestones: []DetachMilestone{
			{Number: 1, Title: "MVP", Description: "CLI-first Gira bootstrapper with templates and GitHub metadata sync."},
			{Number: 2, Title: "Beta", Description: "User changed."},
			{Number: 3, Title: "v1", Description: "Stable daily-use command surface for real repositories.", OpenIssues: 1},
		},
		issues: []DetachIssue{
			{Number: 10, Title: "[Task] Slice 1: CLI skeleton + template dry-run", State: "open", Labels: []string{BootstrapLabel}},
			{Number: 11, Title: "[Task] Closed", State: "closed", Labels: []string{BootstrapLabel}},
			{Number: 12, Title: "User-authored bootstrap-labeled issue", State: "open", Labels: []string{BootstrapLabel}},
		},
	}

	report, err := BuildDetachReport(client, true)
	if err != nil {
		t.Fatalf("BuildDetachReport returned error: %v", err)
	}
	if !report.DryRun || report.Repo != "StatPan/gira" || report.Command != "detach" {
		t.Fatalf("unexpected report header: %+v", report)
	}
	if report.Counts.CloseBootstrapIssues != 1 || report.Counts.DeleteLabels != 1 || report.Counts.DeleteMilestones != 1 {
		t.Fatalf("counts = %+v, want close=1 labels=1 milestones=1", report.Counts)
	}
	if report.Counts.ManualFiles == 0 {
		t.Fatalf("manual managed files should be reported")
	}
	if !hasDetachAction(report.Actions, "label", "delete", BootstrapLabel, "planned") {
		t.Fatalf("missing planned bootstrap label deletion: %+v", report.Actions)
	}
	if !hasDetachAction(report.Actions, "label", "delete", "type:task", "skipped") {
		t.Fatalf("missing skipped in-use label: %+v", report.Actions)
	}
	if !hasDetachAction(report.Actions, "milestone", "delete", "v1", "skipped") {
		t.Fatalf("missing skipped in-use milestone: %+v", report.Actions)
	}
	if !hasDetachAction(report.Actions, "label", "delete", "status:ready", "skipped") {
		t.Fatalf("missing skipped modified label: %+v", report.Actions)
	}
	if !hasDetachAction(report.Actions, "bootstrap_issue", "close", "[Task] Slice 1: CLI skeleton + template dry-run", "planned") {
		t.Fatalf("missing planned bootstrap issue close: %+v", report.Actions)
	}
	if !hasDetachAction(report.Actions, "bootstrap_issue", "close", "User-authored bootstrap-labeled issue", "skipped") {
		t.Fatalf("missing skipped non-default bootstrap issue: %+v", report.Actions)
	}
}

func TestDetachReportJSONIsStable(t *testing.T) {
	client := &fakeDetachClient{
		repo:   mustRepo(t, "StatPan/gira"),
		labels: []ExistingLabel{{Name: BootstrapLabel, Color: "5319E7", Description: "Created or managed by Gira bootstrap metadata sync."}},
	}
	report, err := BuildDetachReport(client, true)
	if err != nil {
		t.Fatalf("BuildDetachReport returned error: %v", err)
	}
	first, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent returned error: %v", err)
	}
	second, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent returned error: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("JSON changed:\n%s\n---\n%s", first, second)
	}
	if !strings.Contains(string(first), `"command": "detach"`) || !strings.Contains(string(first), `"managed_files"`) {
		t.Fatalf("JSON missing stable fields:\n%s", first)
	}
}

func TestFormatDetachReportIncludesTextSummary(t *testing.T) {
	report := DetachReport{
		Repo:   "StatPan/gira",
		DryRun: true,
		Counts: DetachCounts{CloseBootstrapIssues: 1, DeleteLabels: 2, DeleteMilestones: 1, ManualFiles: 1, Skipped: 3},
		Actions: []DetachAction{
			{Kind: "bootstrap_issue", Action: "close", Target: "[Task] Slice 1", Status: "planned"},
			{Kind: "label", Action: "delete", Target: "status:ready", Status: "skipped", SkipReason: "not_deterministic"},
		},
		ManagedFiles: []DetachManagedFile{{Path: "AGENTS.md", Action: "manual_remove"}},
	}

	text := FormatDetachReport(report)
	for _, want := range []string{
		"detach plan: StatPan/gira",
		"mode: dry-run",
		"close bootstrap issues: 1",
		"delete labels:           2",
		"skip label: status:ready (not_deterministic)",
		"manual file: AGENTS.md",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("detach text missing %q:\n%s", want, text)
		}
	}
}

func TestApplyDetachReportUsesOnlyPlannedActions(t *testing.T) {
	client := &fakeDetachClient{repo: mustRepo(t, "StatPan/gira")}
	report := DetachReport{
		Repo:   "StatPan/gira",
		DryRun: true,
		Actions: []DetachAction{
			{Kind: "bootstrap_issue", Action: "close", Target: "Slice", Number: 10, Status: "planned"},
			{Kind: "label", Action: "delete", Target: BootstrapLabel, Status: "planned"},
			{Kind: "milestone", Action: "delete", Target: "MVP", Number: 1, Status: "planned"},
			{Kind: "label", Action: "delete", Target: "type:bug", Status: "applied"},
			{Kind: "label", Action: "delete", Target: "type:story", Status: ""},
			{Kind: "label", Action: "delete", Target: "status:ready", Status: "skipped", SkipReason: "not_deterministic"},
		},
	}

	if err := ApplyDetachReport(client, &report); err != nil {
		t.Fatalf("ApplyDetachReport returned error: %v", err)
	}
	want := []string{"close issue 10", "delete label gira:bootstrap", "delete milestone 1"}
	if !reflect.DeepEqual(client.calls, want) {
		t.Fatalf("calls = %#v, want %#v", client.calls, want)
	}
	if report.DryRun {
		t.Fatalf("apply should mark report as non-dry-run")
	}
	if report.Actions[0].Status != "applied" || report.Actions[3].Status != "applied" || report.Actions[4].Status != "" || report.Actions[5].Status != "skipped" {
		t.Fatalf("unexpected apply statuses: %+v", report.Actions)
	}
}

func TestGHDetachClientUsesGhCommandShapes(t *testing.T) {
	runner := &recordingRunner{}
	client := NewGHDetachClient(mustRepo(t, "StatPan/gira"), runner)
	report := DetachReport{
		Repo: "StatPan/gira",
		Actions: []DetachAction{
			{Kind: "bootstrap_issue", Action: "close", Target: "Slice", Number: 10, Status: "planned"},
			{Kind: "label", Action: "delete", Target: BootstrapLabel, Status: "planned"},
			{Kind: "milestone", Action: "delete", Target: "MVP", Number: 1, Status: "planned"},
		},
	}

	if err := ApplyDetachReport(client, &report); err != nil {
		t.Fatalf("ApplyDetachReport returned error: %v", err)
	}
	want := []string{
		"gh issue close 10 --repo StatPan/gira --comment " + detachCloseComment,
		"gh label delete gira:bootstrap --repo StatPan/gira --yes",
		"gh api repos/StatPan/gira/milestones/1 -X DELETE",
	}
	if !reflect.DeepEqual(runner.commands, want) {
		t.Fatalf("commands = %#v, want %#v", runner.commands, want)
	}
}

type fakeDetachClient struct {
	repo           RepoRef
	labels         []ExistingLabel
	labelUseCounts map[string]int
	milestones     []DetachMilestone
	issues         []DetachIssue
	calls          []string
}

func (c *fakeDetachClient) Repo() RepoRef { return c.repo }

func (c *fakeDetachClient) ListLabels() ([]ExistingLabel, error) {
	return c.labels, nil
}

func (c *fakeDetachClient) LabelUseCount(name string) (int, error) {
	if c.labelUseCounts == nil {
		return 0, nil
	}
	return c.labelUseCounts[name], nil
}

func (c *fakeDetachClient) ListMilestones() ([]DetachMilestone, error) {
	return c.milestones, nil
}

func (c *fakeDetachClient) ListBootstrapIssues() ([]DetachIssue, error) {
	return c.issues, nil
}

func (c *fakeDetachClient) CloseIssue(number int) error {
	c.calls = append(c.calls, "close issue "+strconv.Itoa(number))
	return nil
}

func (c *fakeDetachClient) DeleteLabel(name string) error {
	c.calls = append(c.calls, "delete label "+name)
	return nil
}

func (c *fakeDetachClient) DeleteMilestone(number int) error {
	c.calls = append(c.calls, "delete milestone "+strconv.Itoa(number))
	return nil
}

func hasDetachAction(actions []DetachAction, kind string, action string, target string, status string) bool {
	for _, item := range actions {
		if item.Kind == kind && item.Action == action && item.Target == target && item.Status == status {
			return true
		}
	}
	return false
}
