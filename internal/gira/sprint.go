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
	Repo       string            `json:"repo"`
	Iterations map[string]Sprint `json:"iterations"`
}

type Sprint struct {
	Iteration            string `json:"iteration"`
	CapacityTarget       int    `json:"capacity_target"`
	CommittedItems       []int  `json:"committed_items"`
	CompletedItems       []int  `json:"completed_items"`
	SpilloverItems       []int  `json:"spillover_items"`
	RolloverReason       string `json:"rollover_reason"`
	SpilloverDisposition string `json:"spillover_disposition"`
	CommitmentFrozen     bool   `json:"commitment_frozen"`
	StartedAt            string `json:"started_at,omitempty"`
	ClosedAt             string `json:"closed_at,omitempty"`
}

type SprintPlanReport struct {
	Repo           string `json:"repo"`
	Iteration      string `json:"iteration"`
	Mode           string `json:"mode"`
	Capacity       int    `json:"capacity_target"`
	CommitCount    int    `json:"commit_count"`
	CapacityBreach bool   `json:"capacity_breach"`
	Sprint         Sprint `json:"sprint"`
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

type SprintRolloverReport struct {
	Repo             string                `json:"repo"`
	Mode             string                `json:"mode"`
	TargetMilestone  *SprintRolloverTarget `json:"target_milestone,omitempty"`
	TargetResolution string                `json:"target_resolution"`
	Summary          SprintRolloverSummary `json:"summary"`
	Items            []SprintRolloverItem  `json:"items"`
}

type SprintRolloverTarget struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
}

type SprintRolloverSummary struct {
	Candidates int `json:"candidates"`
	Applied    int `json:"applied"`
	Skipped    int `json:"skipped"`
}

type SprintRolloverItem struct {
	IssueNumber     int    `json:"issue_number"`
	IssueTitle      string `json:"issue_title"`
	FromMilestone   string `json:"from_milestone"`
	LifecycleStatus string `json:"lifecycle_status,omitempty"`
	CandidateReason string `json:"candidate_reason"`
	Action          string `json:"action"`
	SkipReason      string `json:"skip_reason,omitempty"`
	TargetMilestone string `json:"target_milestone,omitempty"`
	NextStep        string `json:"next_step,omitempty"`
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

func SprintRollover(repo RepoRef, toMilestone string, apply bool, now time.Time, runner CommandRunner) (SprintRolloverReport, error) {
	client := NewGHStatusClient(repo, runner)
	return SprintRolloverForClient(client, runner, toMilestone, apply, now)
}

func SprintRolloverForClient(client StatusClient, runner CommandRunner, toMilestone string, apply bool, now time.Time) (SprintRolloverReport, error) {
	milestones, err := FetchMilestones(client)
	if err != nil {
		return SprintRolloverReport{}, err
	}
	issues, err := FetchIssues(client)
	if err != nil {
		return SprintRolloverReport{}, err
	}

	milestoneByTitle := map[string]normalizedMilestone{}
	for _, m := range milestones {
		milestoneByTitle[m.Title] = m
	}

	mode := "dry-run"
	if apply {
		mode = "apply"
	}
	report := SprintRolloverReport{Repo: client.Repo().FullName(), Mode: mode, Items: make([]SprintRolloverItem, 0)}
	target, resolution := resolveRolloverTarget(milestones, strings.TrimSpace(toMilestone), now)
	report.TargetResolution = resolution
	if target != nil {
		report.TargetMilestone = &SprintRolloverTarget{Number: target.Number, Title: target.Title}
	}

	for _, issue := range issues {
		if issue.State != "open" || issue.Milestone == nil {
			continue
		}
		source, ok := milestoneByTitle[*issue.Milestone]
		if !ok {
			continue
		}
		reason := rolloverCandidateReason(source, now)
		if reason == "" {
			continue
		}
		lifecycleStatus := statusFromLabels(issue.Labels)
		item := SprintRolloverItem{IssueNumber: issue.Number, IssueTitle: issue.Title, FromMilestone: source.Title, LifecycleStatus: lifecycleStatus, CandidateReason: reason, NextStep: fmt.Sprintf("gira ticket status --repo %s --ticket %d", client.Repo().FullName(), issue.Number)}
		report.Summary.Candidates++
		if lifecycleStatus == "done" {
			item.Action = "skipped"
			item.SkipReason = "ticket already has status:done"
			report.Summary.Skipped++
			report.Items = append(report.Items, item)
			continue
		}
		if target == nil {
			item.Action = "skipped"
			item.SkipReason = "no target open milestone"
			report.Summary.Skipped++
			report.Items = append(report.Items, item)
			continue
		}
		if source.Number == target.Number {
			item.Action = "skipped"
			item.SkipReason = "already in target milestone"
			item.TargetMilestone = target.Title
			report.Summary.Skipped++
			report.Items = append(report.Items, item)
			continue
		}
		item.TargetMilestone = target.Title
		if apply {
			if err := setIssueMilestone(runner, client.Repo(), issue.Number, target.Number); err != nil {
				item.Action = "skipped"
				item.SkipReason = "apply failed: " + err.Error()
				report.Summary.Skipped++
			} else {
				item.Action = "applied"
				report.Summary.Applied++
			}
		} else {
			item.Action = "would-apply"
			report.Summary.Applied++
		}
		report.Items = append(report.Items, item)
	}

	sort.Slice(report.Items, func(i, j int) bool { return report.Items[i].IssueNumber < report.Items[j].IssueNumber })
	return report, nil
}

func resolveRolloverTarget(milestones []normalizedMilestone, explicit string, now time.Time) (*normalizedMilestone, string) {
	if explicit != "" {
		for _, m := range milestones {
			if m.Title == explicit {
				if strings.EqualFold(m.State, "open") {
					copy := m
					return &copy, "explicit --to"
				}
				return nil, "explicit --to is not open"
			}
		}
		return nil, "explicit --to not found"
	}
	open := make([]normalizedMilestone, 0)
	for _, m := range milestones {
		if strings.EqualFold(m.State, "open") {
			open = append(open, m)
		}
	}
	if len(open) == 0 {
		return nil, "no open milestones"
	}
	sort.Slice(open, func(i, j int) bool {
		di := open[i].DueOn != nil
		dj := open[j].DueOn != nil
		if di != dj {
			return di
		}
		if !di {
			return open[i].Number < open[j].Number
		}
		left, lerr := parseGitHubTime(*open[i].DueOn)
		right, rerr := parseGitHubTime(*open[j].DueOn)
		if lerr != nil || rerr != nil || left.Equal(right) {
			return open[i].Number < open[j].Number
		}
		return left.Before(right)
	})
	for _, m := range open {
		if m.DueOn == nil {
			continue
		}
		due, err := parseGitHubTime(*m.DueOn)
		if err == nil && !due.Before(now.UTC()) {
			copy := m
			return &copy, "cadence next open milestone"
		}
	}
	copy := open[0]
	return &copy, "cadence next open milestone"
}

func rolloverCandidateReason(m normalizedMilestone, now time.Time) string {
	if strings.EqualFold(m.State, "closed") {
		return "source milestone is closed"
	}
	if m.DueOn == nil {
		return ""
	}
	due, err := parseGitHubTime(*m.DueOn)
	if err != nil {
		return ""
	}
	if due.Before(now.UTC()) {
		return "source milestone due date passed"
	}
	return ""
}

func setIssueMilestone(runner CommandRunner, repo RepoRef, issueNumber int, milestoneNumber int) error {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	_, err := runner.Run(
		"gh",
		"api",
		"repos/"+repo.FullName()+"/issues/"+fmt.Sprintf("%d", issueNumber),
		"-X",
		"PATCH",
		"-f",
		"milestone="+fmt.Sprintf("%d", milestoneNumber),
	)
	return err
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
