package gira

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const GoalDossierSchemaVersion = "goal-dossier/v1"

type GoalDossierInput struct {
	Repo RepoRef `json:"repo"`
	Goal int     `json:"goal"`
}

type GoalDossierReport struct {
	Command                 string                     `json:"command"`
	SchemaVersion           string                     `json:"schema_version"`
	Repo                    string                     `json:"repo"`
	GeneratedAt             string                     `json:"generated_at"`
	Goal                    GoalStatusIssue            `json:"goal"`
	ChildGroups             []GoalDossierChildGroup    `json:"child_groups"`
	Counts                  map[string]int             `json:"counts"`
	Blockers                []string                   `json:"blockers,omitempty"`
	StopConditions          []string                   `json:"stop_conditions,omitempty"`
	NextAction              string                     `json:"next_action"`
	NextStep                string                     `json:"next_step"`
	SelectedTicket          *GoalNextCandidate         `json:"selected_ticket,omitempty"`
	RemainingAutonomousWork int                        `json:"remaining_autonomous_work"`
	HandoffReceiptPresent   bool                       `json:"handoff_receipt_present"`
	Evidence                GoalDossierEvidenceSummary `json:"evidence"`
	Sources                 []GoalDossierSource        `json:"sources"`
}

type GoalDossierChildGroup struct {
	Category string            `json:"category"`
	Count    int               `json:"count"`
	Children []GoalStatusChild `json:"children"`
}

type GoalDossierEvidenceSummary struct {
	Sources                 []string                 `json:"sources"`
	ChildCount              int                      `json:"child_count"`
	RemainingAutonomousWork int                      `json:"remaining_autonomous_work"`
	HandoffReceiptPresent   bool                     `json:"handoff_receipt_present"`
	BlockerCount            int                      `json:"blocker_count"`
	Checks                  GoalDossierChecksSummary `json:"checks"`
	Reviews                 map[string]int           `json:"reviews,omitempty"`
}

type GoalDossierChecksSummary struct {
	Total   int `json:"total"`
	Passing int `json:"passing"`
	Pending int `json:"pending"`
	Failing int `json:"failing"`
	Missing int `json:"missing"`
	Unknown int `json:"unknown"`
}

type GoalDossierSource struct {
	Name          string `json:"name"`
	SchemaVersion string `json:"schema_version"`
}

func BuildGoalDossierReport(input GoalDossierInput, runner CommandRunner) (GoalDossierReport, error) {
	status, err := BuildGoalStatusReport(GoalStatusInput{Repo: input.Repo, Goal: input.Goal}, runner)
	if err != nil {
		return GoalDossierReport{}, err
	}
	next := BuildGoalNextReportFromStatus(input.Repo, status)
	return BuildGoalDossierReportFromStatus(status, next), nil
}

func BuildGoalDossierReportFromStatus(status GoalStatusReport, next GoalNextReport) GoalDossierReport {
	report := GoalDossierReport{
		Command:                 "goal dossier",
		SchemaVersion:           GoalDossierSchemaVersion,
		Repo:                    status.Repo,
		GeneratedAt:             time.Now().UTC().Format(time.RFC3339),
		Goal:                    status.Goal,
		ChildGroups:             goalDossierChildGroups(status.Children),
		Counts:                  copyStringIntMap(status.Counts),
		Blockers:                append([]string(nil), status.Blockers...),
		StopConditions:          append([]string(nil), next.StopReasons...),
		NextAction:              next.NextAction,
		NextStep:                next.NextStep,
		RemainingAutonomousWork: status.RemainingAutonomousWork,
		HandoffReceiptPresent:   status.HandoffReceiptPresent,
		Evidence:                goalDossierEvidence(status),
		Sources: []GoalDossierSource{
			{Name: "goal_status", SchemaVersion: GoalStatusSchemaVersion},
			{Name: "goal_next", SchemaVersion: GoalNextSchemaVersion},
		},
	}
	if next.SelectedTicket != nil {
		selected := *next.SelectedTicket
		report.SelectedTicket = &selected
	}
	return report
}

func goalDossierChildGroups(children []GoalStatusChild) []GoalDossierChildGroup {
	byCategory := map[string][]GoalStatusChild{}
	for _, child := range children {
		byCategory[child.Category] = append(byCategory[child.Category], child)
	}
	for category := range byCategory {
		sort.Slice(byCategory[category], func(i, j int) bool {
			return byCategory[category][i].Number < byCategory[category][j].Number
		})
	}
	order := []string{"ready", "in_progress", "in_review", "blocked", "done", "closed_other", "unknown"}
	out := []GoalDossierChildGroup{}
	seen := map[string]struct{}{}
	for _, category := range order {
		children := byCategory[category]
		if len(children) == 0 {
			continue
		}
		out = append(out, GoalDossierChildGroup{Category: category, Count: len(children), Children: append([]GoalStatusChild(nil), children...)})
		seen[category] = struct{}{}
	}
	extra := []string{}
	for category := range byCategory {
		if _, ok := seen[category]; !ok {
			extra = append(extra, category)
		}
	}
	sort.Strings(extra)
	for _, category := range extra {
		children := byCategory[category]
		out = append(out, GoalDossierChildGroup{Category: category, Count: len(children), Children: append([]GoalStatusChild(nil), children...)})
	}
	return out
}

func goalDossierEvidence(status GoalStatusReport) GoalDossierEvidenceSummary {
	return GoalDossierEvidenceSummary{
		Sources:                 []string{"goal_status", "goal_next"},
		ChildCount:              len(status.Children),
		RemainingAutonomousWork: status.RemainingAutonomousWork,
		HandoffReceiptPresent:   status.HandoffReceiptPresent,
		BlockerCount:            len(status.Blockers),
		Checks:                  goalDossierChecks(status.Children),
		Reviews:                 goalDossierReviews(status.Children),
	}
}

func goalDossierChecks(children []GoalStatusChild) GoalDossierChecksSummary {
	var out GoalDossierChecksSummary
	for _, child := range children {
		out.Total++
		switch strings.ToLower(strings.TrimSpace(child.ChecksStatus)) {
		case "":
			out.Missing++
		case "passed", "passing", "success":
			out.Passing++
		case "pending", "queued", "in_progress":
			out.Pending++
		case "failed", "failure", "failing", "error", "cancelled", "timed_out":
			out.Failing++
		case "missing":
			out.Missing++
		default:
			out.Unknown++
		}
	}
	return out
}

func goalDossierReviews(children []GoalStatusChild) map[string]int {
	out := map[string]int{}
	for _, child := range children {
		key := strings.ToLower(strings.TrimSpace(child.ReviewStatus))
		if key == "" {
			key = "unknown"
		}
		out[key]++
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func FormatGoalDossier(report GoalDossierReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "goal dossier: #%d children=%d remaining=%d next=%s\n", report.Goal.Number, report.Counts["total"], report.RemainingAutonomousWork, report.NextAction)
	if len(report.ChildGroups) > 0 {
		parts := []string{}
		for _, group := range report.ChildGroups {
			parts = append(parts, fmt.Sprintf("%s=%d", group.Category, group.Count))
		}
		fmt.Fprintf(&b, "children: %s\n", strings.Join(parts, " "))
	}
	if report.SelectedTicket != nil {
		fmt.Fprintf(&b, "selected: #%d %s\n", report.SelectedTicket.Number, report.SelectedTicket.Reason)
	}
	if len(report.Blockers) > 0 {
		fmt.Fprintf(&b, "blockers: %s\n", strings.Join(report.Blockers, ","))
	}
	if len(report.StopConditions) > 0 {
		fmt.Fprintf(&b, "stop: %s\n", strings.Join(report.StopConditions, ","))
	}
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}
