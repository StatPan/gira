package gira

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const detachCloseComment = "Detached Gira bootstrap metadata; closing bootstrap issue instead of deleting it."

type DetachClient interface {
	Repo() RepoRef
	ListLabels() ([]ExistingLabel, error)
	LabelUseCount(string) (int, error)
	ListMilestones() ([]DetachMilestone, error)
	ListBootstrapIssues() ([]DetachIssue, error)
	CloseIssue(int) error
	DeleteLabel(string) error
	DeleteMilestone(int) error
}

type DetachMilestone struct {
	Number       int     `json:"number"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	DueOn        *string `json:"due_on,omitempty"`
	OpenIssues   int     `json:"open_issues"`
	ClosedIssues int     `json:"closed_issues"`
}

type DetachIssue struct {
	Number int      `json:"number"`
	Title  string   `json:"title"`
	State  string   `json:"state"`
	Labels []string `json:"labels"`
}

type DetachReport struct {
	Repo         string              `json:"repo"`
	Command      string              `json:"command"`
	DryRun       bool                `json:"dry_run"`
	Counts       DetachCounts        `json:"counts"`
	Actions      []DetachAction      `json:"actions"`
	ManagedFiles []DetachManagedFile `json:"managed_files"`
}

type DetachCounts struct {
	CloseBootstrapIssues int `json:"close_bootstrap_issues"`
	DeleteLabels         int `json:"delete_labels"`
	DeleteMilestones     int `json:"delete_milestones"`
	ManualFiles          int `json:"manual_files"`
	Skipped              int `json:"skipped"`
}

type DetachAction struct {
	Kind       string `json:"kind"`
	Action     string `json:"action"`
	Target     string `json:"target"`
	Number     int    `json:"number,omitempty"`
	Reason     string `json:"reason"`
	Status     string `json:"status"`
	SkipReason string `json:"skip_reason,omitempty"`
}

type DetachManagedFile struct {
	Path   string `json:"path"`
	Action string `json:"action"`
	Reason string `json:"reason"`
}

type GHDetachClient struct {
	repo   RepoRef
	runner CommandRunner
}

func NewGHDetachClient(repo RepoRef, runner CommandRunner) GHDetachClient {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	return GHDetachClient{repo: repo, runner: runner}
}

func (c GHDetachClient) Repo() RepoRef {
	return c.repo
}

func (c GHDetachClient) run(args ...string) ([]byte, error) {
	return c.runner.Run("gh", args...)
}

func (c GHDetachClient) json(args []string, target any) error {
	output, err := c.run(args...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(output, target); err != nil {
		return fmt.Errorf("parse gh JSON: %w", err)
	}
	return nil
}

func (c GHDetachClient) ListLabels() ([]ExistingLabel, error) {
	var rows []struct {
		Name        string `json:"name"`
		Color       string `json:"color"`
		Description string `json:"description"`
	}
	if err := c.json([]string{"label", "list", "--repo", c.repo.FullName(), "--json", "name,color,description", "--limit", "1000"}, &rows); err != nil {
		return nil, err
	}
	labels := make([]ExistingLabel, 0, len(rows))
	for _, row := range rows {
		labels = append(labels, ExistingLabel{Name: row.Name, Color: normalizeColor(row.Color), Description: row.Description})
	}
	return labels, nil
}

func (c GHDetachClient) ListMilestones() ([]DetachMilestone, error) {
	var pages json.RawMessage
	if err := c.json([]string{"api", "repos/" + c.repo.FullName() + "/milestones", "--paginate", "--slurp", "-X", "GET", "-f", "state=all", "-f", "per_page=100"}, &pages); err != nil {
		return nil, err
	}
	rows, err := flattenPages(pages)
	if err != nil {
		return nil, err
	}
	milestones := make([]DetachMilestone, 0, len(rows))
	for _, row := range rows {
		var raw struct {
			Number       int     `json:"number"`
			Title        string  `json:"title"`
			Description  *string `json:"description"`
			DueOn        *string `json:"due_on"`
			OpenIssues   int     `json:"open_issues"`
			ClosedIssues int     `json:"closed_issues"`
		}
		if err := json.Unmarshal(row, &raw); err != nil {
			return nil, fmt.Errorf("parse milestone: %w", err)
		}
		description := ""
		if raw.Description != nil {
			description = *raw.Description
		}
		milestones = append(milestones, DetachMilestone{
			Number:       raw.Number,
			Title:        raw.Title,
			Description:  description,
			DueOn:        raw.DueOn,
			OpenIssues:   raw.OpenIssues,
			ClosedIssues: raw.ClosedIssues,
		})
	}
	return milestones, nil
}

func (c GHDetachClient) LabelUseCount(name string) (int, error) {
	var rows []struct {
		Number int `json:"number"`
	}
	if err := c.json([]string{"issue", "list", "--repo", c.repo.FullName(), "--state", "all", "--label", name, "--json", "number", "--limit", "1"}, &rows); err != nil {
		return 0, err
	}
	return len(rows), nil
}

func (c GHDetachClient) ListBootstrapIssues() ([]DetachIssue, error) {
	var rows []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		State  string `json:"state"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := c.json([]string{"issue", "list", "--repo", c.repo.FullName(), "--state", "all", "--label", BootstrapLabel, "--json", "number,title,state,labels", "--limit", "1000"}, &rows); err != nil {
		return nil, err
	}
	issues := make([]DetachIssue, 0, len(rows))
	for _, row := range rows {
		labels := make([]string, 0, len(row.Labels))
		for _, label := range row.Labels {
			labels = append(labels, label.Name)
		}
		issues = append(issues, DetachIssue{Number: row.Number, Title: row.Title, State: row.State, Labels: labels})
	}
	return issues, nil
}

func (c GHDetachClient) CloseIssue(number int) error {
	_, err := c.run("issue", "close", fmt.Sprintf("%d", number), "--repo", c.repo.FullName(), "--comment", detachCloseComment)
	return err
}

func (c GHDetachClient) DeleteLabel(name string) error {
	_, err := c.run("label", "delete", name, "--repo", c.repo.FullName(), "--yes")
	return err
}

func (c GHDetachClient) DeleteMilestone(number int) error {
	_, err := c.run("api", fmt.Sprintf("repos/%s/milestones/%d", c.repo.FullName(), number), "-X", "DELETE")
	return err
}

func BuildDetachReport(client DetachClient, dryRun bool) (DetachReport, error) {
	labels, err := client.ListLabels()
	if err != nil {
		return DetachReport{}, err
	}
	milestones, err := client.ListMilestones()
	if err != nil {
		return DetachReport{}, err
	}
	issues, err := client.ListBootstrapIssues()
	if err != nil {
		return DetachReport{}, err
	}

	labelUseCounts := map[string]int{}
	for _, desired := range DesiredLabels {
		if !hasExistingLabel(labels, desired.Name) {
			continue
		}
		count, err := client.LabelUseCount(desired.Name)
		if err != nil {
			return DetachReport{}, err
		}
		labelUseCounts[desired.Name] = count
	}

	report := DetachReport{
		Repo:         client.Repo().FullName(),
		Command:      "detach",
		DryRun:       dryRun,
		Actions:      buildDetachActions(labels, labelUseCounts, milestones, issues, dryRun),
		ManagedFiles: buildDetachManagedFiles(client.Repo()),
	}
	report.Counts = countDetachReport(report)
	return report, nil
}

func ApplyDetachReport(client DetachClient, report *DetachReport) error {
	for i := range report.Actions {
		action := &report.Actions[i]
		if action.Status == "skipped" {
			continue
		}
		if action.Status != "planned" && action.Status != "pending" {
			continue
		}
		switch action.Kind + ":" + action.Action {
		case "bootstrap_issue:close":
			if err := client.CloseIssue(action.Number); err != nil {
				return err
			}
		case "label:delete":
			if err := client.DeleteLabel(action.Target); err != nil {
				return err
			}
		case "milestone:delete":
			if err := client.DeleteMilestone(action.Number); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported detach action: %s %s", action.Kind, action.Action)
		}
		action.Status = "applied"
	}
	report.DryRun = false
	report.Counts = countDetachReport(*report)
	return nil
}

func FormatDetachReport(report DetachReport) string {
	var b strings.Builder
	mode := "apply"
	if report.DryRun {
		mode = "dry-run"
	}
	fmt.Fprintf(&b, "detach plan: %s\n", report.Repo)
	fmt.Fprintf(&b, "mode: %s\n", mode)
	fmt.Fprintf(&b, "close bootstrap issues: %d\n", report.Counts.CloseBootstrapIssues)
	fmt.Fprintf(&b, "delete labels:           %d\n", report.Counts.DeleteLabels)
	fmt.Fprintf(&b, "delete milestones:       %d\n", report.Counts.DeleteMilestones)
	fmt.Fprintf(&b, "manual files:            %d\n", report.Counts.ManualFiles)
	fmt.Fprintf(&b, "skipped:                 %d\n", report.Counts.Skipped)
	for _, action := range report.Actions {
		if action.Status == "skipped" {
			fmt.Fprintf(&b, "  skip %s: %s (%s)\n", action.Kind, action.Target, action.SkipReason)
			continue
		}
		fmt.Fprintf(&b, "  %s %s: %s\n", action.Action, action.Kind, action.Target)
	}
	for _, file := range report.ManagedFiles {
		fmt.Fprintf(&b, "  manual file: %s\n", file.Path)
	}
	return b.String()
}

func buildDetachActions(labels []ExistingLabel, labelUseCounts map[string]int, milestones []DetachMilestone, issues []DetachIssue, dryRun bool) []DetachAction {
	status := "planned"
	if !dryRun {
		status = "pending"
	}
	var actions []DetachAction
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].Number < issues[j].Number
	})
	for _, issue := range issues {
		if !hasLabel(issue.Labels, BootstrapLabel) {
			continue
		}
		if !isDesiredBootstrapIssueTitle(issue.Title) {
			actions = append(actions, DetachAction{Kind: "bootstrap_issue", Action: "close", Target: issue.Title, Number: issue.Number, Reason: "bootstrap-labeled issue does not match a default Gira bootstrap issue", Status: "skipped", SkipReason: "not_deterministic"})
			continue
		}
		if strings.EqualFold(issue.State, "closed") {
			actions = append(actions, DetachAction{Kind: "bootstrap_issue", Action: "close", Target: issue.Title, Number: issue.Number, Reason: "bootstrap issue is already closed", Status: "skipped", SkipReason: "already_closed"})
			continue
		}
		actions = append(actions, DetachAction{Kind: "bootstrap_issue", Action: "close", Target: issue.Title, Number: issue.Number, Reason: "bootstrap issues are closed instead of deleted", Status: status})
	}

	labelByName := make(map[string]ExistingLabel, len(labels))
	for _, label := range labels {
		labelByName[label.Name] = label
	}
	for _, desired := range DesiredLabels {
		existing, ok := labelByName[desired.Name]
		if !ok {
			actions = append(actions, DetachAction{Kind: "label", Action: "delete", Target: desired.Name, Reason: "managed label is absent", Status: "skipped", SkipReason: "not_found"})
			continue
		}
		if normalizeColor(existing.Color) != normalizeColor(desired.Color) || existing.Description != desired.Description {
			actions = append(actions, DetachAction{Kind: "label", Action: "delete", Target: desired.Name, Reason: "label no longer exactly matches Gira default metadata", Status: "skipped", SkipReason: "not_deterministic"})
			continue
		}
		if labelUseCounts[desired.Name] > 0 {
			actions = append(actions, DetachAction{Kind: "label", Action: "delete", Target: desired.Name, Reason: "label is still attached to issues", Status: "skipped", SkipReason: "in_use"})
			continue
		}
		actions = append(actions, DetachAction{Kind: "label", Action: "delete", Target: desired.Name, Reason: "label exactly matches Gira default metadata", Status: status})
	}

	milestoneByTitle := make(map[string]DetachMilestone, len(milestones))
	for _, milestone := range milestones {
		milestoneByTitle[milestone.Title] = milestone
	}
	for _, desired := range DesiredMilestones {
		existing, ok := milestoneByTitle[desired.Title]
		if !ok {
			actions = append(actions, DetachAction{Kind: "milestone", Action: "delete", Target: desired.Title, Reason: "managed milestone is absent", Status: "skipped", SkipReason: "not_found"})
			continue
		}
		if existing.Description != desired.Description || !stringPtrEqual(existing.DueOn, desired.DueOn) {
			actions = append(actions, DetachAction{Kind: "milestone", Action: "delete", Target: desired.Title, Number: existing.Number, Reason: "milestone no longer exactly matches Gira default metadata", Status: "skipped", SkipReason: "not_deterministic"})
			continue
		}
		if existing.OpenIssues+existing.ClosedIssues > 0 {
			actions = append(actions, DetachAction{Kind: "milestone", Action: "delete", Target: desired.Title, Number: existing.Number, Reason: "milestone is still assigned to issues", Status: "skipped", SkipReason: "in_use"})
			continue
		}
		actions = append(actions, DetachAction{Kind: "milestone", Action: "delete", Target: desired.Title, Number: existing.Number, Reason: "milestone exactly matches Gira default metadata", Status: status})
	}
	return actions
}

func hasExistingLabel(labels []ExistingLabel, name string) bool {
	for _, label := range labels {
		if label.Name == name {
			return true
		}
	}
	return false
}

func isDesiredBootstrapIssueTitle(title string) bool {
	for _, desired := range DesiredBootstrapIssues {
		if title == desired.Title {
			return true
		}
	}
	return false
}

func buildDetachManagedFiles(repo RepoRef) []DetachManagedFile {
	rendered, err := RenderTemplateTree("default", repo, "")
	if err != nil {
		return nil
	}
	files := make([]DetachManagedFile, 0, len(rendered))
	for _, item := range rendered {
		files = append(files, DetachManagedFile{Path: item.Path, Action: "manual_remove", Reason: "detach has no local path flag in this slice"})
	}
	return files
}

func countDetachReport(report DetachReport) DetachCounts {
	counts := DetachCounts{ManualFiles: len(report.ManagedFiles)}
	for _, action := range report.Actions {
		if action.Status == "skipped" {
			counts.Skipped++
			continue
		}
		switch action.Kind + ":" + action.Action {
		case "bootstrap_issue:close":
			counts.CloseBootstrapIssues++
		case "label:delete":
			counts.DeleteLabels++
		case "milestone:delete":
			counts.DeleteMilestones++
		}
	}
	return counts
}
