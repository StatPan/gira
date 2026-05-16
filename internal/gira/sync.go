package gira

import (
	"encoding/json"
	"fmt"
	"strings"
)

const BootstrapLabel = "gira:bootstrap"

type LabelDef struct {
	Name        string
	Color       string
	Description string
}

type MilestoneDef struct {
	Title       string
	Description string
	DueOn       *string
}

type BootstrapIssueDef struct {
	Title     string
	Body      string
	Labels    []string
	Milestone *string
}

type ExistingLabel struct {
	Name        string
	Color       string
	Description string
}

type ExistingMilestone struct {
	Number      int
	Title       string
	Description string
	DueOn       *string
}

type ExistingIssue struct {
	Number int
	Title  string
	Labels []string
}

type PlanAction string

const (
	PlanCreate PlanAction = "create"
	PlanUpdate PlanAction = "update"
	PlanSkip   PlanAction = "skip"
)

type LabelPlan struct {
	Action   PlanAction
	Desired  LabelDef
	Existing *ExistingLabel
}

type MilestonePlan struct {
	Action   PlanAction
	Desired  MilestoneDef
	Existing *ExistingMilestone
}

type BootstrapIssuePlan struct {
	Action   PlanAction
	Desired  BootstrapIssueDef
	Existing *ExistingIssue
}

type SyncPlan struct {
	PolicyMode      SyncPolicyMode
	Labels          []LabelPlan
	Milestones      []MilestonePlan
	BootstrapIssues []BootstrapIssuePlan
}

type SyncPlanOptions struct {
	EnableBootstrapIssues bool
	PolicyMode            SyncPolicyMode
}

type SyncPolicyMode string

const (
	SyncPolicyMerge   SyncPolicyMode = "merge"
	SyncPolicyAdopt   SyncPolicyMode = "adopt"
	SyncPolicyEnforce SyncPolicyMode = "enforce"
)

func ParseSyncPolicyMode(value string) (SyncPolicyMode, error) {
	mode := SyncPolicyMode(strings.ToLower(strings.TrimSpace(value)))
	switch mode {
	case "", SyncPolicyMerge:
		return SyncPolicyMerge, nil
	case SyncPolicyAdopt, SyncPolicyEnforce:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid sync policy mode %q: must be one of adopt|merge|enforce", value)
	}
}

type SyncClient interface {
	Repo() RepoRef
	ListLabels() ([]ExistingLabel, error)
	CreateLabel(LabelDef) error
	UpdateLabel(LabelDef) error
	ListMilestones() ([]ExistingMilestone, error)
	CreateMilestone(MilestoneDef) error
	UpdateMilestone(int, MilestoneDef) error
	ListBootstrapIssues() ([]ExistingIssue, error)
	CreateIssue(BootstrapIssueDef) error
}

type GHSyncClient struct {
	repo   RepoRef
	runner CommandRunner
}

func NewGHSyncClient(repo RepoRef, runner CommandRunner) GHSyncClient {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	return GHSyncClient{repo: repo, runner: runner}
}

func (c GHSyncClient) Repo() RepoRef {
	return c.repo
}

func (c GHSyncClient) run(args ...string) ([]byte, error) {
	return c.runner.Run("gh", args...)
}

func (c GHSyncClient) json(args []string, target any) error {
	output, err := c.run(args...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(output, target); err != nil {
		return fmt.Errorf("parse gh JSON: %w", err)
	}
	return nil
}

func (c GHSyncClient) ListLabels() ([]ExistingLabel, error) {
	var rows []struct {
		Name        string `json:"name"`
		Color       string `json:"color"`
		Description string `json:"description"`
	}
	if err := c.json([]string{
		"label",
		"list",
		"--repo",
		c.repo.FullName(),
		"--json",
		"name,color,description",
		"--limit",
		"1000",
	}, &rows); err != nil {
		return nil, err
	}
	labels := make([]ExistingLabel, 0, len(rows))
	for _, row := range rows {
		labels = append(labels, ExistingLabel{
			Name:        row.Name,
			Color:       normalizeColor(row.Color),
			Description: row.Description,
		})
	}
	return labels, nil
}

func (c GHSyncClient) CreateLabel(label LabelDef) error {
	_, err := c.run(
		"label",
		"create",
		label.Name,
		"--repo",
		c.repo.FullName(),
		"--color",
		label.Color,
		"--description",
		label.Description,
	)
	return err
}

func (c GHSyncClient) UpdateLabel(label LabelDef) error {
	_, err := c.run(
		"label",
		"edit",
		label.Name,
		"--repo",
		c.repo.FullName(),
		"--color",
		label.Color,
		"--description",
		label.Description,
	)
	return err
}

func (c GHSyncClient) ListMilestones() ([]ExistingMilestone, error) {
	var pages json.RawMessage
	if err := c.json([]string{
		"api",
		"repos/" + c.repo.FullName() + "/milestones",
		"--paginate",
		"--slurp",
		"-X",
		"GET",
		"-f",
		"state=all",
		"-f",
		"per_page=100",
	}, &pages); err != nil {
		return nil, err
	}
	rows, err := flattenPages(pages)
	if err != nil {
		return nil, err
	}
	milestones := make([]ExistingMilestone, 0, len(rows))
	for _, row := range rows {
		var raw struct {
			Number      int     `json:"number"`
			Title       string  `json:"title"`
			Description *string `json:"description"`
			DueOn       *string `json:"due_on"`
		}
		if err := json.Unmarshal(row, &raw); err != nil {
			return nil, fmt.Errorf("parse milestone: %w", err)
		}
		description := ""
		if raw.Description != nil {
			description = *raw.Description
		}
		milestones = append(milestones, ExistingMilestone{
			Number:      raw.Number,
			Title:       raw.Title,
			Description: description,
			DueOn:       raw.DueOn,
		})
	}
	return milestones, nil
}

func (c GHSyncClient) CreateMilestone(milestone MilestoneDef) error {
	args := []string{
		"api",
		"repos/" + c.repo.FullName() + "/milestones",
		"-X",
		"POST",
		"-f",
		"title=" + milestone.Title,
		"-f",
		"description=" + milestone.Description,
	}
	if milestone.DueOn != nil {
		args = append(args, "-f", "due_on="+*milestone.DueOn)
	}
	_, err := c.run(args...)
	return err
}

func (c GHSyncClient) UpdateMilestone(number int, milestone MilestoneDef) error {
	args := []string{
		"api",
		fmt.Sprintf("repos/%s/milestones/%d", c.repo.FullName(), number),
		"-X",
		"PATCH",
		"-f",
		"title=" + milestone.Title,
		"-f",
		"description=" + milestone.Description,
	}
	if milestone.DueOn != nil {
		args = append(args, "-f", "due_on="+*milestone.DueOn)
	} else {
		args = append(args, "-F", "due_on=null")
	}
	_, err := c.run(args...)
	return err
}

func (c GHSyncClient) ListBootstrapIssues() ([]ExistingIssue, error) {
	var rows []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := c.json([]string{
		"issue",
		"list",
		"--repo",
		c.repo.FullName(),
		"--state",
		"all",
		"--label",
		BootstrapLabel,
		"--json",
		"number,title,labels",
		"--limit",
		"1000",
	}, &rows); err != nil {
		return nil, err
	}
	issues := make([]ExistingIssue, 0, len(rows))
	for _, row := range rows {
		labels := make([]string, 0, len(row.Labels))
		for _, label := range row.Labels {
			labels = append(labels, label.Name)
		}
		issues = append(issues, ExistingIssue{
			Number: row.Number,
			Title:  row.Title,
			Labels: labels,
		})
	}
	return issues, nil
}

func (c GHSyncClient) CreateIssue(issue BootstrapIssueDef) error {
	args := []string{
		"issue",
		"create",
		"--repo",
		c.repo.FullName(),
		"--title",
		issue.Title,
		"--body",
		issue.Body,
	}
	for _, label := range issue.Labels {
		args = append(args, "--label", label)
	}
	if issue.Milestone != nil {
		args = append(args, "--milestone", *issue.Milestone)
	}
	_, err := c.run(args...)
	return err
}

var DesiredLabels = []LabelDef{
	{Name: "gira:bootstrap", Color: "5319E7", Description: "Created or managed by Gira bootstrap metadata sync."},
	{Name: "type:epic", Color: "0E8A16", Description: "Large outcome that groups related implementation tasks."},
	{Name: "type:task", Color: "1D76DB", Description: "Concrete implementation task."},
	{Name: "type:chore", Color: "D4C5F9", Description: "Maintenance or process work."},
	{Name: "type:bug", Color: "D73A4A", Description: "Defect report that should be reproduced and fixed."},
	{Name: "type:story", Color: "A2EEEF", Description: "User-facing capability with acceptance criteria."},
	{Name: "type:spike", Color: "FBCA04", Description: "Research or feasibility investigation with a bounded output."},
	{Name: "agent:human", Color: "FBCA04", Description: "Owned by a human project lead."},
	{Name: "agent:worker", Color: "BFDADC", Description: "Ready for an implementation worker."},
	{Name: "status:ready", Color: "C2E0C6", Description: "Ready to start."},
	{Name: "status:in-progress", Color: "1D76DB", Description: "Work has started on a branch or active implementation."},
	{Name: "status:in-review", Color: "7057FF", Description: "A linked PR or review surface is active."},
	{Name: "status:blocked", Color: "E99695", Description: "Blocked by an external dependency or decision."},
	{Name: "priority:p1", Color: "D93F0B", Description: "High priority work."},
	{Name: "priority:p2", Color: "FBCA04", Description: "Medium priority work."},
	{Name: "area:backend", Color: "0052CC", Description: "Backend, CLI, and core workflow implementation."},
	{Name: "area:docs", Color: "0075CA", Description: "Documentation and process guidance."},
	{Name: "area:ai", Color: "7057FF", Description: "AI and agent workflow behavior."},
}

var DesiredMilestones = []MilestoneDef{
	{Title: "MVP", Description: "CLI-first Gira bootstrapper with templates and GitHub metadata sync."},
	{Title: "Beta", Description: "Broader validation and hardening after the MVP workflow is usable."},
	{Title: "v1", Description: "Stable first release of the GitHub-native project OS workflow."},
}

var DesiredBootstrapIssues = []BootstrapIssueDef{
	{
		Title: "[Epic] Gira MVP: GitHub-as-OS bootstrap",
		Body: "## Goal\n" +
			"Ship the CLI-first Gira MVP.\n\n" +
			"## Scope\n" +
			"- local template bootstrap\n" +
			"- GitHub label, milestone, and bootstrap issue sync\n" +
			"- compact status summary\n",
		Labels:    []string{"gira:bootstrap", "type:epic", "agent:human", "status:ready"},
		Milestone: stringPtr("MVP"),
	},
	{
		Title:     "[Task] Slice 1: CLI skeleton + template dry-run",
		Body:      "## Goal\nCreate the Go CLI entrypoint and default project template dry-run.",
		Labels:    []string{"gira:bootstrap", "type:task", "agent:worker", "status:ready"},
		Milestone: stringPtr("MVP"),
	},
	{
		Title:     "[Task] Slice 2: idempotent repo file install",
		Body:      "## Goal\nInstall rendered template files into a local git repository idempotently.",
		Labels:    []string{"gira:bootstrap", "type:task", "agent:worker", "status:ready"},
		Milestone: stringPtr("MVP"),
	},
	{
		Title:     "[Task] Slice 3: labels/milestones/bootstrap-issues sync",
		Body:      "## Goal\nSync GitHub labels, milestones, and bootstrap issues through the gh CLI.",
		Labels:    []string{"gira:bootstrap", "type:task", "agent:worker", "status:ready"},
		Milestone: stringPtr("MVP"),
	},
	{
		Title:     "[Task] Slice 4: gira status (text + --json)",
		Body:      "## Goal\nShow a compact text and JSON status summary for a Gira-managed GitHub repository.",
		Labels:    []string{"gira:bootstrap", "type:task", "agent:worker", "status:ready"},
		Milestone: stringPtr("MVP"),
	},
}

func BuildSyncPlan(client SyncClient, opts SyncPlanOptions) (SyncPlan, error) {
	mode, err := ParseSyncPolicyMode(string(opts.PolicyMode))
	if err != nil {
		return SyncPlan{}, err
	}

	labels, err := client.ListLabels()
	if err != nil {
		return SyncPlan{}, err
	}
	milestones, err := client.ListMilestones()
	if err != nil {
		return SyncPlan{}, err
	}

	var bootstrapPlan []BootstrapIssuePlan
	if opts.EnableBootstrapIssues {
		issues, err := client.ListBootstrapIssues()
		if err != nil {
			return SyncPlan{}, err
		}
		bootstrapPlan = PlanBootstrapIssues(DesiredBootstrapIssues, issues)
	} else {
		bootstrapPlan = make([]BootstrapIssuePlan, 0, len(DesiredBootstrapIssues))
		for _, issue := range DesiredBootstrapIssues {
			bootstrapPlan = append(bootstrapPlan, BootstrapIssuePlan{Action: PlanSkip, Desired: issue})
		}
	}

	plan := SyncPlan{
		PolicyMode:      mode,
		Labels:          PlanLabels(DesiredLabels, labels),
		Milestones:      PlanMilestones(DesiredMilestones, milestones),
		BootstrapIssues: bootstrapPlan,
	}
	if mode == SyncPolicyAdopt {
		for i := range plan.Labels {
			plan.Labels[i].Action = PlanSkip
		}
		for i := range plan.Milestones {
			plan.Milestones[i].Action = PlanSkip
		}
		for i := range plan.BootstrapIssues {
			plan.BootstrapIssues[i].Action = PlanSkip
		}
	}
	return plan, nil
}

func ApplySyncPlan(client SyncClient, plan SyncPlan) error {
	for _, item := range plan.Labels {
		switch item.Action {
		case PlanCreate:
			if err := client.CreateLabel(item.Desired); err != nil {
				return err
			}
		case PlanUpdate:
			if err := client.UpdateLabel(item.Desired); err != nil {
				return err
			}
		}
	}
	for _, item := range plan.Milestones {
		switch item.Action {
		case PlanCreate:
			if err := client.CreateMilestone(item.Desired); err != nil {
				return err
			}
		case PlanUpdate:
			if item.Existing == nil {
				return fmt.Errorf("missing existing milestone for update: %s", item.Desired.Title)
			}
			if err := client.UpdateMilestone(item.Existing.Number, item.Desired); err != nil {
				return err
			}
		}
	}
	for _, item := range plan.BootstrapIssues {
		if item.Action == PlanCreate {
			if err := client.CreateIssue(item.Desired); err != nil {
				return err
			}
		}
	}
	return nil
}

func PlanLabels(desired []LabelDef, existing []ExistingLabel) []LabelPlan {
	byName := make(map[string]ExistingLabel, len(existing))
	for _, item := range existing {
		byName[item.Name] = item
	}
	plan := make([]LabelPlan, 0, len(desired))
	for _, label := range desired {
		current, ok := byName[label.Name]
		if !ok {
			plan = append(plan, LabelPlan{Action: PlanCreate, Desired: label})
			continue
		}
		if normalizeColor(current.Color) != normalizeColor(label.Color) || current.Description != label.Description {
			existingCopy := current
			plan = append(plan, LabelPlan{Action: PlanUpdate, Desired: label, Existing: &existingCopy})
			continue
		}
		existingCopy := current
		plan = append(plan, LabelPlan{Action: PlanSkip, Desired: label, Existing: &existingCopy})
	}
	return plan
}

func PlanMilestones(desired []MilestoneDef, existing []ExistingMilestone) []MilestonePlan {
	byTitle := make(map[string]ExistingMilestone, len(existing))
	for _, item := range existing {
		byTitle[item.Title] = item
	}
	plan := make([]MilestonePlan, 0, len(desired))
	for _, milestone := range desired {
		current, ok := byTitle[milestone.Title]
		if !ok {
			plan = append(plan, MilestonePlan{Action: PlanCreate, Desired: milestone})
			continue
		}
		if current.Description != milestone.Description || !stringPtrEqual(current.DueOn, milestone.DueOn) {
			existingCopy := current
			plan = append(plan, MilestonePlan{Action: PlanUpdate, Desired: milestone, Existing: &existingCopy})
			continue
		}
		existingCopy := current
		plan = append(plan, MilestonePlan{Action: PlanSkip, Desired: milestone, Existing: &existingCopy})
	}
	return plan
}

func PlanBootstrapIssues(desired []BootstrapIssueDef, existing []ExistingIssue) []BootstrapIssuePlan {
	byTitle := make(map[string]ExistingIssue, len(existing))
	for _, issue := range existing {
		if hasLabel(issue.Labels, BootstrapLabel) {
			byTitle[issue.Title] = issue
		}
	}
	plan := make([]BootstrapIssuePlan, 0, len(desired))
	for _, issue := range desired {
		current, ok := byTitle[issue.Title]
		if !ok {
			plan = append(plan, BootstrapIssuePlan{Action: PlanCreate, Desired: issue})
			continue
		}
		existingCopy := current
		plan = append(plan, BootstrapIssuePlan{Action: PlanSkip, Desired: issue, Existing: &existingCopy})
	}
	return plan
}

func FormatSyncPlan(plan SyncPlan, dryRun bool) string {
	prefix := ""
	if dryRun {
		prefix = "would "
	}
	var b strings.Builder
	b.WriteString("sync plan:\n")
	mode := plan.PolicyMode
	if mode == "" {
		mode = SyncPolicyMerge
	}
	fmt.Fprintf(&b, "policy mode:      %s\n", mode)
	fmt.Fprintf(&b, "labels:           %d %screate, %d %supdate, %d skip\n", countLabelActions(plan.Labels, PlanCreate), prefix, countLabelActions(plan.Labels, PlanUpdate), prefix, countLabelActions(plan.Labels, PlanSkip))
	fmt.Fprintf(&b, "milestones:       %d %screate, %d %supdate, %d skip\n", countMilestoneActions(plan.Milestones, PlanCreate), prefix, countMilestoneActions(plan.Milestones, PlanUpdate), prefix, countMilestoneActions(plan.Milestones, PlanSkip))
	fmt.Fprintf(&b, "bootstrap issues: %d %screate, %d skip\n", countBootstrapIssueActions(plan.BootstrapIssues, PlanCreate), prefix, countBootstrapIssueActions(plan.BootstrapIssues, PlanSkip))
	for _, item := range plan.Labels {
		if item.Action != PlanSkip {
			fmt.Fprintf(&b, "  %s label: %s\n", item.Action, item.Desired.Name)
		}
	}
	for _, item := range plan.Milestones {
		if item.Action != PlanSkip {
			fmt.Fprintf(&b, "  %s milestone: %s\n", item.Action, item.Desired.Title)
		}
	}
	for _, item := range plan.BootstrapIssues {
		if item.Action != PlanSkip {
			fmt.Fprintf(&b, "  %s issue: %s\n", item.Action, item.Desired.Title)
		}
	}
	return b.String()
}

func normalizeColor(value string) string {
	return strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(value), "#"))
}

func stringPtr(value string) *string {
	return &value
}

func stringPtrEqual(left *string, right *string) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func countLabelActions(items []LabelPlan, action PlanAction) int {
	count := 0
	for _, item := range items {
		if item.Action == action {
			count++
		}
	}
	return count
}

func countMilestoneActions(items []MilestonePlan, action PlanAction) int {
	count := 0
	for _, item := range items {
		if item.Action == action {
			count++
		}
	}
	return count
}

func countBootstrapIssueActions(items []BootstrapIssuePlan, action PlanAction) int {
	count := 0
	for _, item := range items {
		if item.Action == action {
			count++
		}
	}
	return count
}
