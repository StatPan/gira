package gira

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestPlanLabelsCreatesUpdatesAndSkipsManagedLabels(t *testing.T) {
	plan := PlanLabels(
		[]LabelDef{
			{Name: "agent:worker", Color: "BFDADC", Description: "Ready for a worker."},
			{Name: "agent:human", Color: "FBCA04", Description: "Human owned."},
			{Name: "status:ready", Color: "C2E0C6", Description: "Ready to start."},
		},
		[]ExistingLabel{
			{Name: "agent:worker", Color: "#bfdadc", Description: "Ready for a worker."},
			{Name: "agent:human", Color: "000000", Description: "Old description."},
			{Name: "custom", Color: "111111", Description: "Preserve me."},
		},
	)

	got := []PlanAction{plan[0].Action, plan[1].Action, plan[2].Action}
	want := []PlanAction{PlanSkip, PlanUpdate, PlanCreate}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("actions = %v, want %v", got, want)
		}
	}
}

func TestPlanMilestonesCreatesUpdatesAndSkipsByTitle(t *testing.T) {
	plan := PlanMilestones(
		[]MilestoneDef{
			{Title: "MVP", Description: "CLI-first scope."},
			{Title: "Beta", Description: "Hardening."},
		},
		[]ExistingMilestone{
			{Number: 1, Title: "MVP", Description: "Old scope."},
			{Number: 99, Title: "User milestone", Description: "Preserve me."},
		},
	)

	if plan[0].Action != PlanUpdate || plan[0].Existing == nil || plan[0].Existing.Number != 1 {
		t.Fatalf("first milestone plan = %+v, want update of #1", plan[0])
	}
	if plan[1].Action != PlanCreate {
		t.Fatalf("second milestone action = %s, want create", plan[1].Action)
	}
}

func TestPlanBootstrapIssuesDeduplicatesOnlyBootstrapLabeledMatches(t *testing.T) {
	plan := PlanBootstrapIssues(
		[]BootstrapIssueDef{
			{Title: "[Task] Slice 3", Body: "body", Labels: []string{BootstrapLabel}},
			{Title: "[Task] Slice 4", Body: "body", Labels: []string{BootstrapLabel}},
		},
		[]ExistingIssue{
			{Number: 3, Title: "[Task] Slice 3", Labels: []string{BootstrapLabel, "agent:worker"}},
			{Number: 4, Title: "[Task] Slice 4", Labels: []string{"agent:worker"}},
		},
	)

	if plan[0].Action != PlanSkip || plan[0].Existing == nil || plan[0].Existing.Number != 3 {
		t.Fatalf("first issue plan = %+v, want skip of #3", plan[0])
	}
	if plan[1].Action != PlanCreate {
		t.Fatalf("second issue action = %s, want create", plan[1].Action)
	}
}

func TestFormatSyncPlanMatchesDryRunLanguage(t *testing.T) {
	plan := SyncPlan{
		Labels:          []LabelPlan{{Action: PlanCreate, Desired: LabelDef{Name: "type:bug"}}},
		Milestones:      []MilestonePlan{{Action: PlanSkip, Desired: MilestoneDef{Title: "MVP"}}},
		BootstrapIssues: []BootstrapIssuePlan{{Action: PlanCreate, Desired: BootstrapIssueDef{Title: "[Task] Slice 3"}}},
	}

	text := FormatSyncPlan(plan, true)
	for _, want := range []string{
		"sync plan:",
		"labels:           1 would create, 0 would update, 0 skip",
		"milestones:       0 would create, 0 would update, 1 skip",
		"bootstrap issues: 1 would create, 0 skip",
		"  create label: type:bug",
		"  create issue: [Task] Slice 3",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("sync plan missing %q:\n%s", want, text)
		}
	}
}

func TestBuildSyncPlanUsesGhListShapes(t *testing.T) {
	client := &fakeSyncClient{
		repo: mustRepo(t, "StatPan/gira"),
		labels: []ExistingLabel{
			{Name: BootstrapLabel, Color: "5319E7", Description: "Created or managed by Gira bootstrap metadata sync."},
		},
		milestones: []ExistingMilestone{
			{Number: 1, Title: "MVP", Description: "CLI-first Gira bootstrapper with templates and GitHub metadata sync."},
		},
		issues: []ExistingIssue{
			{Number: 1, Title: "[Epic] Gira MVP: GitHub-as-OS bootstrap", Labels: []string{BootstrapLabel}},
		},
	}

	plan, err := BuildSyncPlan(client, SyncPlanOptions{EnableBootstrapIssues: true})
	if err != nil {
		t.Fatalf("BuildSyncPlan returned error: %v", err)
	}
	if countLabelActions(plan.Labels, PlanCreate) != len(DesiredLabels)-1 {
		t.Fatalf("label create count = %d, want %d", countLabelActions(plan.Labels, PlanCreate), len(DesiredLabels)-1)
	}
	if countMilestoneActions(plan.Milestones, PlanCreate) != len(DesiredMilestones)-1 {
		t.Fatalf("milestone create count = %d, want %d", countMilestoneActions(plan.Milestones, PlanCreate), len(DesiredMilestones)-1)
	}
	if countBootstrapIssueActions(plan.BootstrapIssues, PlanCreate) != len(DesiredBootstrapIssues)-1 {
		t.Fatalf("issue create count = %d, want %d", countBootstrapIssueActions(plan.BootstrapIssues, PlanCreate), len(DesiredBootstrapIssues)-1)
	}
}

func TestBuildSyncPlanSkipsBootstrapIssuesWhenDisabled(t *testing.T) {
	client := &fakeSyncClient{
		repo:       mustRepo(t, "StatPan/gira"),
		labels:     []ExistingLabel{{Name: BootstrapLabel, Color: "5319E7", Description: "Created or managed by Gira bootstrap metadata sync."}},
		milestones: []ExistingMilestone{{Number: 1, Title: "MVP", Description: "CLI-first Gira bootstrapper with templates and GitHub metadata sync."}},
		issues:     []ExistingIssue{{Number: 1, Title: "[Epic] Gira MVP: GitHub-as-OS bootstrap", Labels: []string{BootstrapLabel}}},
	}

	plan, err := BuildSyncPlan(client, SyncPlanOptions{EnableBootstrapIssues: false})
	if err != nil {
		t.Fatalf("BuildSyncPlan returned error: %v", err)
	}
	if len(plan.BootstrapIssues) != len(DesiredBootstrapIssues) {
		t.Fatalf("bootstrap issue plan length = %d, want %d", len(plan.BootstrapIssues), len(DesiredBootstrapIssues))
	}
	if countBootstrapIssueActions(plan.BootstrapIssues, PlanCreate) != 0 {
		t.Fatalf("bootstrap issue create count = %d, want 0", countBootstrapIssueActions(plan.BootstrapIssues, PlanCreate))
	}
	if countBootstrapIssueActions(plan.BootstrapIssues, PlanSkip) != len(DesiredBootstrapIssues) {
		t.Fatalf("bootstrap issue skip count = %d, want %d", countBootstrapIssueActions(plan.BootstrapIssues, PlanSkip), len(DesiredBootstrapIssues))
	}
}

func TestApplySyncPlanCreatesAndUpdatesOnlyChangedItems(t *testing.T) {
	client := &fakeSyncClient{repo: mustRepo(t, "StatPan/gira")}
	plan := SyncPlan{
		Labels: []LabelPlan{
			{Action: PlanCreate, Desired: LabelDef{Name: "type:bug"}},
			{Action: PlanUpdate, Desired: LabelDef{Name: "agent:worker"}},
			{Action: PlanSkip, Desired: LabelDef{Name: "custom"}},
		},
		Milestones: []MilestonePlan{
			{Action: PlanCreate, Desired: MilestoneDef{Title: "Beta"}},
			{Action: PlanUpdate, Desired: MilestoneDef{Title: "MVP"}, Existing: &ExistingMilestone{Number: 1}},
		},
		BootstrapIssues: []BootstrapIssuePlan{
			{Action: PlanCreate, Desired: BootstrapIssueDef{Title: "[Task] Slice 3"}},
			{Action: PlanSkip, Desired: BootstrapIssueDef{Title: "[Task] Slice 4"}},
		},
	}

	if err := ApplySyncPlan(client, plan); err != nil {
		t.Fatalf("ApplySyncPlan returned error: %v", err)
	}
	got := strings.Join(client.calls, "|")
	want := "create label type:bug|update label agent:worker|create milestone Beta|update milestone 1 MVP|create issue [Task] Slice 3"
	if got != want {
		t.Fatalf("calls = %q, want %q", got, want)
	}
}

func TestGHSyncClientApplyUsesGhCommandShapes(t *testing.T) {
	runner := &recordingRunner{}
	client := NewGHSyncClient(mustRepo(t, "StatPan/gira"), runner)
	plan := SyncPlan{
		Labels: []LabelPlan{
			{Action: PlanCreate, Desired: LabelDef{Name: "type:bug", Color: "D73A4A", Description: "Defect report."}},
			{Action: PlanUpdate, Desired: LabelDef{Name: "agent:worker", Color: "BFDADC", Description: "Ready."}},
		},
		Milestones: []MilestonePlan{
			{Action: PlanCreate, Desired: MilestoneDef{Title: "Beta", Description: "Hardening."}},
			{Action: PlanUpdate, Desired: MilestoneDef{Title: "MVP", Description: "Scope."}, Existing: &ExistingMilestone{Number: 1}},
		},
		BootstrapIssues: []BootstrapIssuePlan{
			{Action: PlanCreate, Desired: BootstrapIssueDef{Title: "[Task] Slice 3", Body: "body", Labels: []string{BootstrapLabel, "type:task"}, Milestone: stringPtr("MVP")}},
		},
	}

	if err := ApplySyncPlan(client, plan); err != nil {
		t.Fatalf("ApplySyncPlan returned error: %v", err)
	}
	want := []string{
		"gh label create type:bug --repo StatPan/gira --color D73A4A --description Defect report.",
		"gh label edit agent:worker --repo StatPan/gira --color BFDADC --description Ready.",
		"gh api repos/StatPan/gira/milestones -X POST -f title=Beta -f description=Hardening.",
		"gh api repos/StatPan/gira/milestones/1 -X PATCH -f title=MVP -f description=Scope. -F due_on=null",
		"gh issue create --repo StatPan/gira --title [Task] Slice 3 --body body --label gira:bootstrap --label type:task --milestone MVP",
	}
	if !reflect.DeepEqual(runner.commands, want) {
		t.Fatalf("commands = %#v, want %#v", runner.commands, want)
	}
}

type fakeSyncClient struct {
	repo       RepoRef
	labels     []ExistingLabel
	milestones []ExistingMilestone
	issues     []ExistingIssue
	calls      []string
}

type recordingRunner struct {
	commands []string
}

func (r *recordingRunner) Run(name string, args ...string) ([]byte, error) {
	r.commands = append(r.commands, strings.Join(append([]string{name}, args...), " "))
	return []byte("{}"), nil
}

func (c *fakeSyncClient) Repo() RepoRef { return c.repo }

func (c *fakeSyncClient) ListLabels() ([]ExistingLabel, error) {
	return c.labels, nil
}

func (c *fakeSyncClient) CreateLabel(label LabelDef) error {
	c.calls = append(c.calls, "create label "+label.Name)
	return nil
}

func (c *fakeSyncClient) UpdateLabel(label LabelDef) error {
	c.calls = append(c.calls, "update label "+label.Name)
	return nil
}

func (c *fakeSyncClient) ListMilestones() ([]ExistingMilestone, error) {
	return c.milestones, nil
}

func (c *fakeSyncClient) CreateMilestone(milestone MilestoneDef) error {
	c.calls = append(c.calls, "create milestone "+milestone.Title)
	return nil
}

func (c *fakeSyncClient) UpdateMilestone(number int, milestone MilestoneDef) error {
	c.calls = append(c.calls, "update milestone "+strconv.Itoa(number)+" "+milestone.Title)
	return nil
}

func (c *fakeSyncClient) ListBootstrapIssues() ([]ExistingIssue, error) {
	return c.issues, nil
}

func (c *fakeSyncClient) CreateIssue(issue BootstrapIssueDef) error {
	c.calls = append(c.calls, "create issue "+issue.Title)
	return nil
}
