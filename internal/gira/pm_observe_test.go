package gira

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

type countingWorkGraphRunner struct {
	base  *workGraphRunner
	calls map[string]int
}

func (r *countingWorkGraphRunner) Run(name string, args ...string) ([]byte, error) {
	call := name + " " + strings.Join(args, " ")
	r.calls[call]++
	return r.base.Run(name, args...)
}

func TestPMObserveCachesRepeatedSourceReads(t *testing.T) {
	source := PMWorkGraphSource{SchemaVersion: PMWorkGraphSourceSchemaVersion, Nodes: []PMWorkGraphNode{{ID: "build", Title: "Build", Purpose: "Bounded work", Profile: "delivery", ParentOutcome: "goal:100", Size: "small", Uncertainty: "resolved", Verification: []PMWorkGraphVerification{{Method: "test", Evidence: "pass"}}}}}
	runner := &countingWorkGraphRunner{base: &workGraphRunner{body: workGraphGoalBody(t, source)}, calls: map[string]int{}}
	if _, err := BuildPMObserveReport(PMObserveInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Ticket: 100}, runner); err != nil {
		t.Fatal(err)
	}
	contextCall := "gh issue view 100 --repo OWNER/repo --json number,title,body,url,comments"
	if runner.calls[contextCall] != 1 {
		t.Fatalf("repeated PM source read count=%d want=1", runner.calls[contextCall])
	}
}

func TestPMObserveDeterministicDiagnosisOrderingAndEvidenceChange(t *testing.T) {
	input := PMObserveInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Ticket: 100}
	state := pmObserveFixtureState("invalidated")
	first := BuildPMObserveFromState(input, state)
	second := BuildPMObserveFromState(input, state)
	if !reflect.DeepEqual(first.Diagnoses, second.Diagnoses) || !reflect.DeepEqual(first.Actions, second.Actions) || first.Change.CurrentDigest != second.Change.CurrentDigest {
		t.Fatalf("same evidence is nondeterministic:\n%#v\n%#v", first, second)
	}
	if len(first.Actions) < 2 || !hasPMObserveAction(first.Actions, "replan") || !hasPMObserveDiagnosis(first.Diagnoses, "PMO001_INVALIDATED_ASSUMPTION") || !hasPMObserveDiagnosis(first.Diagnoses, "PMO006_BLOCKED_LEARNING") {
		t.Fatalf("missing active PM diagnoses/actions: %#v", first)
	}
	changed := pmObserveFixtureState("supported")
	changed.PriorPlanID = "pmr-old"
	changed.PriorDigest = first.Change.CurrentDigest
	next := BuildPMObserveFromState(input, changed)
	if !next.Change.Changed || next.Change.CurrentDigest == first.Change.CurrentDigest || hasPMObserveDiagnosis(next.Diagnoses, "PMO001_INVALIDATED_ASSUMPTION") {
		t.Fatalf("new evidence did not revise recommendation without history rewrite: %#v", next)
	}
	if !state.Context.Records[0].Current || state.Context.Records[0].Record.Status != "invalidated" {
		t.Fatal("observe mutated historical evidence")
	}
}

func TestPMObserveCoversExpiredDecisionScopeDriftAndPartialContinuation(t *testing.T) {
	state := pmObserveFixtureState("supported")
	state.Context.Records = append(state.Context.Records, PMContextRecord{Current: true, Record: PMLedgerRecord{ID: "decision.old", Kind: "decision", Status: "review_due"}})
	state.GoalStatus.Children = append(state.GoalStatus.Children, GoalStatusChild{Number: 102, Title: "Manual urgent work", State: "open", Status: "In progress"})
	report := BuildPMObserveFromState(PMObserveInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Ticket: 100}, state)
	if !hasPMObserveDiagnosis(report.Diagnoses, "PMO003_EXPIRED_DECISION") || !hasPMObserveDiagnosis(report.Diagnoses, "PMO009_SCOPE_DRIFT") {
		t.Fatalf("external drift/authority diagnosis missing: %#v", report.Diagnoses)
	}
	residual := 0
	for _, action := range report.Actions {
		if action.Residual {
			residual++
		}
		if action.Kind == "stop" {
			t.Fatal("safe independent work was hidden by a stop action")
		}
	}
	if residual != 1 {
		t.Fatalf("authority work should yield one residual action, got %#v", report.Actions)
	}
}

func TestPMObserveRoutesOversizedAndNoBuildEvidence(t *testing.T) {
	state := pmObserveFixtureState("supported")
	state.Context.Records = append(state.Context.Records, PMContextRecord{Current: true, Record: PMLedgerRecord{ID: "learning.stop", Kind: "learning", Status: "active", Conclusion: "no_build"}})
	state.WorkGraph.Diagnostics = []PMWorkGraphDiagnostic{{Code: PMWorkGraphOversized, NodeID: "build", Reason: "slice exceeds the bounded work contract"}}
	report := BuildPMObserveFromState(PMObserveInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Ticket: 100}, state)
	if !hasPMObserveAction(report.Actions, "split") || !hasPMObserveAction(report.Actions, "stop") {
		t.Fatalf("split/stop taxonomy was not evidence-routed: %#v", report.Actions)
	}
}

func TestPMReplanDryApplyFingerprintOverrideAndIdempotency(t *testing.T) {
	source := PMWorkGraphSource{SchemaVersion: PMWorkGraphSourceSchemaVersion, Nodes: []PMWorkGraphNode{{ID: "build", Title: "Build bounded slice", Purpose: "Deliver verified behavior", Profile: "delivery", ParentOutcome: "goal:100", Size: "small", Uncertainty: "resolved", Verification: []PMWorkGraphVerification{{Method: "go test ./...", Evidence: "passing checks"}}}}}
	runner := &workGraphRunner{body: workGraphGoalBody(t, source)}
	input := PMReplanInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Ticket: 100, DryRun: true, Override: "continue", OverrideRationale: "Continue safe delivery while the residual outcome decision is reviewed."}
	preview, err := BuildPMReplanReport(input, runner)
	if err != nil {
		t.Fatal(err)
	}
	if preview.PlanID == "" || preview.Override == nil || len(preview.Mutations) != 1 || preview.Mutations[0].Action != "create" {
		t.Fatalf("incomplete replan preview: %#v", preview)
	}
	if _, err := BuildPMReplanReport(PMReplanInput{Repo: input.Repo, Ticket: 100, DryRun: true, Override: "unblock:#999", OverrideRationale: "Explicitly resume a verified blocked child after evidence review."}, runner); err == nil {
		t.Fatal("unblock override accepted a non-child target")
	}
	bad, err := BuildPMReplanReport(PMReplanInput{Repo: input.Repo, Ticket: 100, Apply: true, ExpectedPlanID: "pmr-stale", Override: input.Override, OverrideRationale: input.OverrideRationale}, runner)
	if err == nil || bad.Matched || runner.creates != 0 {
		t.Fatalf("stale replan mutated state: %#v err=%v", bad, err)
	}
	applied, err := BuildPMReplanReport(PMReplanInput{Repo: input.Repo, Ticket: 100, Apply: true, ExpectedPlanID: preview.PlanID, Override: input.Override, OverrideRationale: input.OverrideRationale}, runner)
	if err != nil {
		t.Fatal(err)
	}
	if runner.creates != 2 || runner.parentLinks != 2 || applied.Mutations[0].Status != "applied" {
		t.Fatalf("safe graph and residual packet were not applied: %#v runner=%#v", applied, runner)
	}
	if !hasLedgerRecordComment(runner.comments, applied.Override.DurableID) {
		t.Fatalf("human override was not durable decision evidence: %#v", runner.comments)
	}
	retry, err := BuildPMReplanReport(PMReplanInput{Repo: input.Repo, Ticket: 100, Apply: true, ExpectedPlanID: preview.PlanID, Override: input.Override, OverrideRationale: input.OverrideRationale}, runner)
	if err != nil || !retry.Idempotent || runner.creates != 2 {
		t.Fatalf("replan retry duplicated mutations: %#v err=%v", retry, err)
	}
}

func pmObserveFixtureState(assumptionStatus string) PMObserveState {
	context := PMContextReport{Issue: PMContextIssue{Number: 100, URL: "https://example/100"}, Summary: PMContextSummary{Current: 2, ByKind: map[string]int{"assumption": 1, "outcome": 1}}, Records: []PMContextRecord{
		{Current: true, CommentURL: "https://example/a", Record: PMLedgerRecord{ID: "assumption.value", Kind: "assumption", Status: assumptionStatus}},
		{Current: true, CommentURL: "https://example/o", Record: PMLedgerRecord{ID: "outcome.activation", Kind: "outcome", Status: "active", OutcomeState: "observing"}},
	}}
	discovery := PMDiscoveryReport{Summary: PMDiscoverySummary{ByKind: map[string]int{"outcome": 1}}, Nodes: []PMDiscoveryNode{{ID: "outcome.activation", Kind: "outcome", Current: true, OutcomeState: "observing"}, {ID: "experiment.value", Kind: "experiment", Current: true, ExperimentState: "inconclusive", CommentURL: "https://example/e"}}}
	measurement := PMMeasurementReport{Summary: PMMeasurementSummary{Outcomes: 1, Measurements: 1, Validated: 1}}
	node := PMWorkGraphNode{ID: "build", Title: "Build bounded slice"}
	graph := PMWorkGraphReport{PlanID: "pwg-stable", Nodes: []PMWorkGraphNode{node}, Actions: []PMWorkGraphAction{{NodeID: "build", Action: "reuse", ExistingIssue: 101}}}
	status := GoalStatusReport{Goal: GoalStatusIssue{Number: 100}, Children: []GoalStatusChild{{Number: 101, Title: goalPlanTicketTitle(node.Title), State: "open", Status: "In progress"}}}
	return PMObserveState{Context: context, Discovery: discovery, Measurement: measurement, WorkGraph: graph, GoalStatus: status}
}

func hasPMObserveDiagnosis(values []PMObserveDiagnosis, code string) bool {
	for _, value := range values {
		if value.Code == code {
			return true
		}
	}
	return false
}
func hasPMObserveAction(values []PMObserveAction, kind string) bool {
	for _, value := range values {
		if value.Kind == kind {
			return true
		}
	}
	return false
}
func hasLedgerRecordComment(values []string, id string) bool {
	for _, value := range values {
		if !strings.Contains(value, pmLedgerRecordMarker) {
			continue
		}
		var record PMLedgerRecord
		start, end := strings.Index(value, "```json\n"), strings.LastIndex(value, "\n```")
		if start >= 0 && end > start && json.Unmarshal([]byte(value[start+8:end]), &record) == nil && record.ID == id {
			return true
		}
	}
	return false
}
