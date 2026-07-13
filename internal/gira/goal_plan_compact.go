package gira

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const GoalPlanCompactSchemaVersion = "goal-plan-compact/v1"

type GoalPlanCompactReport struct {
	Command        string                  `json:"command"`
	SchemaVersion  string                  `json:"schema_version"`
	Mode           string                  `json:"mode"`
	PlanID         string                  `json:"plan_id"`
	ExpectedPlanID string                  `json:"expected_plan_id,omitempty"`
	Matched        bool                    `json:"matched"`
	Source         GoalPlanCompactSource   `json:"source"`
	Defaults       GoalPlanCompactDefaults `json:"defaults,omitempty"`
	Proposals      []GoalPlanCompactTicket `json:"proposals,omitempty"`
	Receipt        *GoalPlanCompactReceipt `json:"receipt,omitempty"`
	StopConditions []string                `json:"stop_conditions,omitempty"`
	Warnings       []string                `json:"warnings,omitempty"`
	NextAction     string                  `json:"next_action"`
	NextStep       string                  `json:"next_step"`
	DetailCommand  string                  `json:"detail_command"`
}

type GoalPlanCompactSource struct {
	Repo  string `json:"repo"`
	Goal  int    `json:"goal"`
	Title string `json:"title"`
}
type GoalPlanCompactDefaults struct {
	Type             string   `json:"type"`
	Priority         string   `json:"priority,omitempty"`
	Labels           []string `json:"labels,omitempty"`
	Scope            string   `json:"scope,omitempty"`
	ExpectedEvidence []string `json:"expected_evidence,omitempty"`
}
type GoalPlanCompactTicket struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	TargetRepo    string   `json:"target_repo"`
	Goal          string   `json:"goal"`
	Acceptance    []string `json:"acceptance"`
	Dependencies  []int    `json:"dependencies,omitempty"`
	Milestone     string   `json:"milestone,omitempty"`
	PayloadSHA256 string   `json:"payload_sha256"`
}
type GoalPlanCompactReceipt struct {
	Created []GoalPlanChild  `json:"created,omitempty"`
	Skipped []GoalPlanSkip   `json:"skipped,omitempty"`
	Actions []GoalPlanAction `json:"actions,omitempty"`
}

func BuildGoalPlanCompactReport(report GoalPlanReport, mode string, expected string) GoalPlanCompactReport {
	compact := GoalPlanCompactReport{Command: "goal plan", SchemaVersion: GoalPlanCompactSchemaVersion, Mode: mode, ExpectedPlanID: expected, Matched: expected == "" || expected == goalPlanFingerprint(report), Source: GoalPlanCompactSource{Repo: report.Repo, Goal: report.Goal.Number, Title: report.Goal.Title}, StopConditions: append([]string(nil), report.StopConditions...), Warnings: append([]string(nil), report.Warnings...), NextAction: report.NextAction, NextStep: report.NextStep, DetailCommand: fmt.Sprintf("gira goal plan --repo %s --goal %d --dry-run --json", report.Repo, report.Goal.Number)}
	compact.PlanID = goalPlanFingerprint(report)
	if mode == "apply" {
		compact.Proposals = nil
		compact.Receipt = &GoalPlanCompactReceipt{Created: append([]GoalPlanChild(nil), report.CreatedChildren...), Skipped: append([]GoalPlanSkip(nil), report.SkippedCandidates...), Actions: append([]GoalPlanAction(nil), report.Actions...)}
		return compact
	}
	if len(report.ProposedTickets) > 0 {
		first := report.ProposedTickets[0]
		compact.Defaults = GoalPlanCompactDefaults{Type: first.Type, Priority: first.Priority, Labels: append([]string(nil), first.Labels...), Scope: first.Scope, ExpectedEvidence: append([]string(nil), first.ExpectedEvidence...)}
	}
	for i, ticket := range report.ProposedTickets {
		sum := sha256.Sum256([]byte(ticket.Body))
		compact.Proposals = append(compact.Proposals, GoalPlanCompactTicket{ID: fmt.Sprintf("proposal-%d", i+1), Title: ticket.Title, TargetRepo: ticket.TargetRepo, Goal: ticket.Goal, Acceptance: append([]string(nil), ticket.Acceptance...), Dependencies: append([]int(nil), ticket.Dependencies...), Milestone: ticket.Milestone, PayloadSHA256: hex.EncodeToString(sum[:])})
	}
	return compact
}

func goalPlanFingerprint(report GoalPlanReport) string {
	type fingerprint struct {
		Repo     string
		Goal     GoalStatusIssue
		Proposed []GoalPlanTicket
		Existing []GoalPlanChild
		Stops    []string
		Warnings []string
	}
	value := fingerprint{Repo: report.Repo, Goal: report.Goal, Proposed: report.ProposedTickets, Existing: report.ExistingChildren, Stops: report.StopConditions, Warnings: report.Warnings}
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return "gpp-" + hex.EncodeToString(sum[:16])
}
