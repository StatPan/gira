package gira

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	PMObserveSchemaVersion = "pm-observe-report/v1"
	PMReplanSchemaVersion  = "pm-replan-report/v1"
)

const pmReplanReceiptMarker = "<!-- gira:pm-replan-receipt/v1 -->"

type PMObserveInput struct {
	Repo   RepoRef
	Ticket int
}

type PMObserveDiagnosis struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Target   string `json:"target"`
	Reason   string `json:"reason"`
	Evidence string `json:"evidence"`
}

type PMObserveAction struct {
	Kind       string `json:"kind"`
	Target     string `json:"target"`
	Reason     string `json:"reason"`
	Capability string `json:"capability"`
	Residual   bool   `json:"residual"`
	Rank       int    `json:"rank"`
}

type PMObserveSnapshot struct {
	ContextDigest     string `json:"context_digest"`
	GraphPlanID       string `json:"graph_plan_id"`
	Children          int    `json:"children"`
	OpenChildren      int    `json:"open_children"`
	BlockedChildren   int    `json:"blocked_children"`
	CurrentRecords    int    `json:"current_records"`
	OutcomeRecords    int    `json:"outcome_records"`
	MeasurementNodes  int    `json:"measurement_nodes"`
	ValidatedOutcomes int    `json:"validated_outcomes"`
}

type PMObserveChange struct {
	Changed        bool   `json:"changed"`
	PreviousPlanID string `json:"previous_plan_id,omitempty"`
	PreviousDigest string `json:"previous_recommendation_digest,omitempty"`
	CurrentDigest  string `json:"current_recommendation_digest"`
	Reason         string `json:"reason"`
}

type PMObserveReport struct {
	Command       string               `json:"command"`
	SchemaVersion string               `json:"schema_version"`
	ReadOnly      bool                 `json:"read_only"`
	Repo          string               `json:"repo"`
	Ticket        int                  `json:"ticket"`
	Snapshot      PMObserveSnapshot    `json:"snapshot"`
	Diagnoses     []PMObserveDiagnosis `json:"diagnoses"`
	Actions       []PMObserveAction    `json:"actions"`
	Change        PMObserveChange      `json:"change"`
	Diagnostics   []PMLedgerDiagnostic `json:"diagnostics"`
	NextStep      string               `json:"next_step"`
	Context       *PMContextReport     `json:"context,omitempty"`
	Discovery     *PMDiscoveryReport   `json:"discovery,omitempty"`
	Measurement   *PMMeasurementReport `json:"measurement,omitempty"`
	WorkGraph     *PMWorkGraphReport   `json:"work_graph,omitempty"`
	GoalStatus    *GoalStatusReport    `json:"goal_status,omitempty"`
}

type PMObserveState struct {
	Context     PMContextReport
	Discovery   PMDiscoveryReport
	Measurement PMMeasurementReport
	WorkGraph   PMWorkGraphReport
	GoalStatus  GoalStatusReport
	PriorPlanID string
	PriorDigest string
}

type pmObserveCachedResult struct {
	value []byte
	err   error
}

type pmObserveCachedRunner struct {
	base    CommandRunner
	results map[string]pmObserveCachedResult
}

func (r *pmObserveCachedRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + "\x00" + strings.Join(args, "\x00")
	if result, ok := r.results[key]; ok {
		return append([]byte(nil), result.value...), result.err
	}
	value, err := r.base.Run(name, args...)
	r.results[key] = pmObserveCachedResult{value: append([]byte(nil), value...), err: err}
	return value, err
}

func BuildPMObserveReport(input PMObserveInput, runner CommandRunner) (PMObserveReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	runner = &pmObserveCachedRunner{base: runner, results: map[string]pmObserveCachedResult{}}
	report := PMObserveReport{Command: "pm observe", SchemaVersion: PMObserveSchemaVersion, ReadOnly: true, Repo: input.Repo.FullName(), Ticket: input.Ticket, Diagnoses: []PMObserveDiagnosis{}, Actions: []PMObserveAction{}, Diagnostics: []PMLedgerDiagnostic{}}
	if input.Ticket <= 0 {
		return report, fmt.Errorf("ticket must be > 0")
	}
	context, err := BuildPMContextReport(PMContextInput{Repo: input.Repo, Ticket: input.Ticket}, runner)
	if err != nil {
		return report, err
	}
	discovery, err := BuildPMDiscoveryReport(PMDiscoveryInput{Repo: input.Repo, Ticket: input.Ticket}, runner)
	if err != nil {
		return report, err
	}
	measurement, err := BuildPMMeasurementReport(PMMeasurementInput{Repo: input.Repo, Ticket: input.Ticket}, runner)
	if err != nil {
		return report, err
	}
	graph, err := BuildPMWorkGraphReport(PMWorkGraphInput{Repo: input.Repo, Goal: input.Ticket}, runner)
	if err != nil {
		return report, err
	}
	status, err := BuildGoalStatusReport(GoalStatusInput{Repo: input.Repo, Goal: input.Ticket}, runner)
	if err != nil {
		return report, err
	}
	priorPlan, priorDigest := latestPMReplanReceipt(input.Repo, input.Ticket, runner)
	state := PMObserveState{Context: context, Discovery: discovery, Measurement: measurement, WorkGraph: graph, GoalStatus: status, PriorPlanID: priorPlan, PriorDigest: priorDigest}
	report = BuildPMObserveFromState(input, state)
	report.Context, report.Discovery, report.Measurement, report.WorkGraph, report.GoalStatus = &context, &discovery, &measurement, &graph, &status
	return report, nil
}

func BuildPMObserveFromState(input PMObserveInput, state PMObserveState) PMObserveReport {
	report := PMObserveReport{Command: "pm observe", SchemaVersion: PMObserveSchemaVersion, ReadOnly: true, Repo: input.Repo.FullName(), Ticket: input.Ticket, Diagnoses: []PMObserveDiagnosis{}, Actions: []PMObserveAction{}, Diagnostics: []PMLedgerDiagnostic{}}
	report.Diagnostics = append(report.Diagnostics, state.Context.Diagnostics...)
	report.Diagnostics = append(report.Diagnostics, state.Discovery.Diagnostics...)
	report.Diagnostics = append(report.Diagnostics, state.Measurement.Diagnostics...)
	report.Snapshot.ContextDigest = pmObserveContextDigest(state.Context)
	report.Snapshot.GraphPlanID = state.WorkGraph.PlanID
	report.Snapshot.Children = len(state.GoalStatus.Children)
	report.Snapshot.CurrentRecords = state.Context.Summary.Current
	report.Snapshot.OutcomeRecords = state.Discovery.Summary.ByKind["outcome"]
	report.Snapshot.MeasurementNodes = state.Measurement.Summary.Measurements
	report.Snapshot.ValidatedOutcomes = state.Measurement.Summary.Validated
	for _, child := range state.GoalStatus.Children {
		if strings.EqualFold(child.State, "open") {
			report.Snapshot.OpenChildren++
		}
		if strings.EqualFold(child.Status, "Blocked") {
			report.Snapshot.BlockedChildren++
		}
	}
	for _, item := range state.Context.Records {
		if !item.Current {
			continue
		}
		r := item.Record
		switch {
		case r.Kind == "assumption" && r.Status == "invalidated":
			report.Diagnoses = append(report.Diagnoses, pmObserveDiagnosis("PMO001_INVALIDATED_ASSUMPTION", "error", r.ID, "an active assumption was invalidated", item.CommentURL))
		case r.Kind == "assumption" && r.Status == "expired":
			report.Diagnoses = append(report.Diagnoses, pmObserveDiagnosis("PMO002_STALE_CONTEXT", "warning", r.ID, "an assumption expired", item.CommentURL))
		case r.Kind == "decision" && containsPMValue([]string{"review_due", "revoked"}, r.Status):
			report.Diagnoses = append(report.Diagnoses, pmObserveDiagnosis("PMO003_EXPIRED_DECISION", "error", r.ID, "a decision requires renewed authority or evidence", item.CommentURL))
		case r.Kind == "question" && r.Status == "open":
			report.Diagnoses = append(report.Diagnoses, pmObserveDiagnosis("PMO004_OPEN_QUESTION", "warning", r.ID, "an open product question blocks confident delivery", item.CommentURL))
		case r.Kind == "learning" && r.Conclusion == "invalidated":
			report.Diagnoses = append(report.Diagnoses, pmObserveDiagnosis("PMO005_INVALIDATED_LEARNING", "error", r.ID, "learning invalidates the prior delivery direction", item.CommentURL))
		case r.Kind == "learning" && r.Conclusion == "no_build":
			report.Diagnoses = append(report.Diagnoses, pmObserveDiagnosis("PMO011_NO_BUILD_CONCLUSION", "warning", r.ID, "validated learning recommends stopping this solution direction", item.CommentURL))
		}
	}
	for _, node := range state.Discovery.Nodes {
		if node.Current && node.Kind == "experiment" && containsPMValue([]string{"failure", "inconclusive", "invalid"}, node.ExperimentState) && !pmObserveHasLearning(state.Discovery, node.ID) {
			report.Diagnoses = append(report.Diagnoses, pmObserveDiagnosis("PMO006_BLOCKED_LEARNING", "warning", node.ID, "finished experiment has no current learning conclusion", node.CommentURL))
		}
	}
	if report.Snapshot.OutcomeRecords == 0 {
		report.Diagnoses = append(report.Diagnoses, pmObserveDiagnosis("PMO007_MISSING_OUTCOME", "error", "goal:"+strconv.Itoa(input.Ticket), "Goal has no typed product outcome", state.Context.Issue.URL))
	} else if report.Snapshot.MeasurementNodes == 0 || report.Snapshot.ValidatedOutcomes < report.Snapshot.OutcomeRecords {
		report.Diagnoses = append(report.Diagnoses, pmObserveDiagnosis("PMO008_MISSING_OUTCOME_EVIDENCE", "warning", "goal:"+strconv.Itoa(input.Ticket), "current outcomes lack validated measurement evidence", state.Measurement.DetailCommand))
	}
	if pmObserveScopeDrift(state.WorkGraph.Nodes, state.GoalStatus.Children) {
		report.Diagnoses = append(report.Diagnoses, pmObserveDiagnosis("PMO009_SCOPE_DRIFT", "warning", "goal:"+strconv.Itoa(input.Ticket), "open child work is not represented by the current typed graph", "gira goal status"))
	}
	if hasPMWorkGraphErrors(state.WorkGraph.Diagnostics) {
		report.Diagnoses = append(report.Diagnoses, pmObserveDiagnosis("PMO010_INVALID_WORK_GRAPH", "error", "goal:"+strconv.Itoa(input.Ticket), "typed work graph cannot be safely lowered", "gira goal graph --json"))
	}
	for _, diagnostic := range state.WorkGraph.Diagnostics {
		if diagnostic.Code == PMWorkGraphOversized {
			report.Diagnoses = append(report.Diagnoses, pmObserveDiagnosis("PMO012_OVERSIZED_WORK", "error", diagnostic.NodeID, diagnostic.Reason, "gira goal graph --json"))
		}
	}
	sort.Slice(report.Diagnoses, func(i, j int) bool {
		if report.Diagnoses[i].Code != report.Diagnoses[j].Code {
			return report.Diagnoses[i].Code < report.Diagnoses[j].Code
		}
		return report.Diagnoses[i].Target < report.Diagnoses[j].Target
	})
	report.Actions = pmObserveActions(report.Diagnoses, state)
	digest := pmObserveRecommendationDigest(report.Diagnoses, report.Actions)
	report.Change = PMObserveChange{PreviousPlanID: state.PriorPlanID, PreviousDigest: state.PriorDigest, CurrentDigest: digest}
	switch {
	case state.PriorDigest == "":
		report.Change.Reason = "no prior replan receipt"
	case state.PriorDigest != digest:
		report.Change.Changed = true
		report.Change.Reason = "current evidence changed the deterministic diagnosis or action set"
	default:
		report.Change.Reason = "recommendation is unchanged"
	}
	report.NextStep = fmt.Sprintf("gira pm replan --repo %s --ticket %d --dry-run --json", input.Repo.FullName(), input.Ticket)
	return report
}

func pmObserveActions(diagnoses []PMObserveDiagnosis, state PMObserveState) []PMObserveAction {
	byKey := map[string]PMObserveAction{}
	add := func(kind, target, reason, capability string, residual bool, rank int) {
		key := kind + "\x00" + target
		if _, ok := byKey[key]; !ok {
			byKey[key] = PMObserveAction{Kind: kind, Target: target, Reason: reason, Capability: capability, Residual: residual, Rank: rank}
		}
	}
	for _, d := range diagnoses {
		switch d.Code {
		case "PMO001_INVALIDATED_ASSUMPTION", "PMO005_INVALIDATED_LEARNING", "PMO009_SCOPE_DRIFT", "PMO010_INVALID_WORK_GRAPH":
			add("replan", d.Target, d.Reason, "plan:write", false, 20)
		case "PMO002_STALE_CONTEXT":
			add("retrieve", d.Target, d.Reason, "context:read", false, 10)
		case "PMO003_EXPIRED_DECISION":
			add("decide", d.Target, d.Reason, "decision:authority", true, 30)
		case "PMO004_OPEN_QUESTION", "PMO006_BLOCKED_LEARNING":
			add("discover", d.Target, d.Reason, "evidence:write", false, 15)
		case "PMO007_MISSING_OUTCOME":
			add("decide", d.Target, d.Reason, "decision:authority", true, 30)
		case "PMO008_MISSING_OUTCOME_EVIDENCE":
			add("validate", d.Target, d.Reason, "evidence:write", false, 40)
		case "PMO011_NO_BUILD_CONCLUSION":
			add("stop", d.Target, d.Reason, "plan:write", false, 70)
		case "PMO012_OVERSIZED_WORK":
			add("split", d.Target, d.Reason, "plan:write", false, 18)
		}
	}
	if len(byKey) == 0 {
		open := false
		for _, child := range state.GoalStatus.Children {
			if strings.EqualFold(child.State, "open") {
				open = true
				break
			}
		}
		if open {
			add("continue", "goal:"+strconv.Itoa(state.GoalStatus.Goal.Number), "current work remains valid", "issue:read", false, 50)
		} else {
			add("report", "goal:"+strconv.Itoa(state.GoalStatus.Goal.Number), "no open child work remains", "report:read", false, 60)
		}
	}
	out := make([]PMObserveAction, 0, len(byKey))
	for _, a := range byKey {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Rank != out[j].Rank {
			return out[i].Rank < out[j].Rank
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Target < out[j].Target
	})
	return out
}

func pmObserveDiagnosis(code, severity, target, reason, evidence string) PMObserveDiagnosis {
	return PMObserveDiagnosis{Code: code, Severity: severity, Target: target, Reason: reason, Evidence: evidence}
}
func pmObserveHasLearning(discovery PMDiscoveryReport, experiment string) bool {
	for _, edge := range discovery.Edges {
		if edge.Current && edge.Relation == "learned_from" && edge.To == experiment {
			return true
		}
	}
	return false
}
func pmObserveScopeDrift(nodes []PMWorkGraphNode, children []GoalStatusChild) bool {
	if len(nodes) == 0 {
		return len(children) > 0
	}
	for _, child := range children {
		if !strings.EqualFold(child.State, "open") {
			continue
		}
		matched := false
		for _, node := range nodes {
			if goalPlanTicketTitle(node.Title) == child.Title || strings.EqualFold(node.Title, child.Title) {
				matched = true
				break
			}
		}
		if !matched {
			return true
		}
	}
	return false
}
func pmObserveContextDigest(context PMContextReport) string {
	value := []PMLedgerRecord{}
	for _, item := range context.Records {
		if item.Current {
			value = append(value, item.Record)
		}
	}
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func pmObserveRecommendationDigest(d []PMObserveDiagnosis, a []PMObserveAction) string {
	encoded, _ := json.Marshal(struct {
		D []PMObserveDiagnosis
		A []PMObserveAction
	}{d, a})
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func latestPMReplanReceipt(repo RepoRef, ticket int, runner CommandRunner) (string, string) {
	raw, err := runner.Run("gh", "issue", "view", strconv.Itoa(ticket), "--repo", repo.FullName(), "--json", "comments")
	if err != nil {
		return "", ""
	}
	var payload struct {
		Comments []struct {
			Body string `json:"body"`
		} `json:"comments"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return "", ""
	}
	for i := len(payload.Comments) - 1; i >= 0; i-- {
		body := payload.Comments[i].Body
		if !strings.Contains(body, pmReplanReceiptMarker) {
			continue
		}
		var receipt struct {
			PlanID               string `json:"plan_id"`
			RecommendationDigest string `json:"recommendation_digest"`
		}
		start, end := strings.Index(body, "```json\n"), strings.LastIndex(body, "\n```")
		if start >= 0 && end > start && json.Unmarshal([]byte(body[start+8:end]), &receipt) == nil {
			return receipt.PlanID, receipt.RecommendationDigest
		}
	}
	return "", ""
}

func FormatPMObserve(report PMObserveReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "pm observe: #%d diagnoses=%d actions=%d changed=%t\n", report.Ticket, len(report.Diagnoses), len(report.Actions), report.Change.Changed)
	for _, d := range report.Diagnoses {
		fmt.Fprintf(&b, "- %s %s: %s\n", d.Code, d.Target, d.Reason)
	}
	for _, a := range report.Actions {
		fmt.Fprintf(&b, "- action=%s target=%s capability=%s residual=%t\n", a.Kind, a.Target, a.Capability, a.Residual)
	}
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}
