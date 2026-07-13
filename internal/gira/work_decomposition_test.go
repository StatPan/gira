package gira

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestWorkDecompositionEvaluatesParallelReadyAndWaitingWork(t *testing.T) {
	decomposition := validWorkDecomposition()
	result, err := EvaluateWorkDecomposition(decomposition)
	if err != nil {
		t.Fatalf("EvaluateWorkDecomposition() error = %v", err)
	}
	if !reflect.DeepEqual(result.Order, []string{"contract", "docs", "integrate"}) {
		t.Fatalf("order = %+v", result.Order)
	}
	if got := workUnitIDs(result.Ready); !reflect.DeepEqual(got, []string{"contract", "docs"}) {
		t.Fatalf("ready = %+v", got)
	}
	if len(result.Waiting) != 1 || result.Waiting[0].WorkID != "integrate" || !reflect.DeepEqual(result.Waiting[0].MissingDependencies, []string{"contract", "docs"}) {
		t.Fatalf("waiting = %+v", result.Waiting)
	}
	if len(result.Gaps) != 0 {
		t.Fatalf("gaps = %+v", result.Gaps)
	}
}

func TestWorkDecompositionCompletedDependenciesUnlockWork(t *testing.T) {
	decomposition := validWorkDecomposition()
	decomposition.CompletedWork = []string{"docs", "contract"}
	result, err := EvaluateWorkDecomposition(decomposition)
	if err != nil {
		t.Fatalf("EvaluateWorkDecomposition() error = %v", err)
	}
	if got := workUnitIDs(result.Ready); !reflect.DeepEqual(got, []string{"integrate"}) {
		t.Fatalf("ready = %+v", got)
	}
	if len(result.Waiting) != 0 {
		t.Fatalf("waiting = %+v", result.Waiting)
	}
}

func TestWorkDecompositionMissingInputProducesCausalGap(t *testing.T) {
	decomposition := validWorkDecomposition()
	decomposition.Children[0].RequiredInputs = []string{"user-principles"}
	result, err := EvaluateWorkDecomposition(decomposition)
	if err != nil {
		t.Fatalf("EvaluateWorkDecomposition() error = %v", err)
	}
	if len(result.Waiting) != 2 || result.Waiting[0].WorkID != "contract" || !reflect.DeepEqual(result.Waiting[0].MissingInputs, []string{"user-principles"}) {
		t.Fatalf("waiting = %+v", result.Waiting)
	}
	if len(result.Gaps) != 1 || result.Gaps[0].Reason != DecisionReasonMissingContext || result.Gaps[0].AffectedWork[0] != "contract" || !strings.Contains(result.Gaps[0].ResumeCondition, "user-principles") {
		t.Fatalf("gaps = %+v", result.Gaps)
	}
}

func TestWorkDecompositionPreservesExplicitPolicyGap(t *testing.T) {
	decomposition := validWorkDecomposition()
	decomposition.Children[0].DecisionGap = &DecisionGap{
		Reason: DecisionReasonMissingPolicy, Question: "Which safety principle wins?", AffectedWork: []string{"contract"},
		ViableOptions: []DecisionOption{}, MissingInput: "principle precedence", ResumeCondition: "record precedence and reevaluate",
	}
	result, err := EvaluateWorkDecomposition(decomposition)
	if err != nil {
		t.Fatalf("EvaluateWorkDecomposition() error = %v", err)
	}
	if len(result.Gaps) != 1 || result.Gaps[0].Reason != DecisionReasonMissingPolicy || result.Gaps[0].ResumeCondition == "" {
		t.Fatalf("gaps = %+v", result.Gaps)
	}
	if got := workUnitIDs(result.Ready); !reflect.DeepEqual(got, []string{"docs"}) {
		t.Fatalf("ready = %+v", got)
	}
}

func TestWorkDecompositionRejectsInvalidGraphs(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*WorkDecomposition)
		wantErr string
	}{
		{name: "duplicate id", mutate: func(d *WorkDecomposition) { d.Children[1].ID = "contract" }, wantErr: "duplicate decomposed work id"},
		{name: "missing dependency", mutate: func(d *WorkDecomposition) { d.Children[2].Dependencies = []string{"missing"} }, wantErr: "missing dependency"},
		{name: "cycle", mutate: func(d *WorkDecomposition) { d.Children[0].Dependencies = []string{"integrate"} }, wantErr: "contains a cycle"},
		{name: "empty output", mutate: func(d *WorkDecomposition) { d.Children[0].Outputs = nil }, wantErr: "at least one output"},
		{name: "unmeasurable acceptance", mutate: func(d *WorkDecomposition) { d.Children[0].Acceptance[0].Measurable = false }, wantErr: "unmeasurable acceptance"},
		{name: "missing verification", mutate: func(d *WorkDecomposition) { d.Children[0].Verification = nil }, wantErr: "requires verification"},
		{name: "unknown completion evidence", mutate: func(d *WorkDecomposition) { d.CompletedWork = []string{"missing"} }, wantErr: "completion evidence references unknown work"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decomposition := validWorkDecomposition()
			tt.mutate(&decomposition)
			_, err := EvaluateWorkDecomposition(decomposition)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestWorkDecompositionOrderingIgnoresInputSliceOrder(t *testing.T) {
	first := validWorkDecomposition()
	second := validWorkDecomposition()
	second.Children[0], second.Children[2] = second.Children[2], second.Children[0]
	firstResult, err := EvaluateWorkDecomposition(first)
	if err != nil {
		t.Fatal(err)
	}
	secondResult, err := EvaluateWorkDecomposition(second)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(firstResult.Order, secondResult.Order) || !reflect.DeepEqual(workUnitIDs(firstResult.Ready), workUnitIDs(secondResult.Ready)) {
		t.Fatalf("results differ: first=%+v second=%+v", firstResult, secondResult)
	}
}

func TestWorkDecompositionJSONRoundTrip(t *testing.T) {
	result, err := EvaluateWorkDecomposition(validWorkDecomposition())
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded WorkDecompositionResult
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != WorkDecompositionSchemaVersion || !reflect.DeepEqual(decoded.Order, result.Order) || decoded.Ready == nil || decoded.Waiting == nil || decoded.Gaps == nil {
		t.Fatalf("decoded = %+v", decoded)
	}
}

func validWorkDecomposition() WorkDecomposition {
	unit := func(id string, dependencies ...string) DecomposedWorkUnit {
		return DecomposedWorkUnit{
			ID: id, Title: id + " title", Goal: id + " goal", Dependencies: dependencies,
			RequiredInputs: []string{}, Outputs: []string{id + " output"},
			Acceptance:   []DecompositionAcceptance{{Criterion: id + " output exists", Measurable: true}},
			Verification: []DecompositionVerification{{ID: id + "-test", Method: "go test ./...", ExpectedEvidence: "passing test"}},
		}
	}
	return WorkDecomposition{
		SchemaVersion: WorkDecompositionSchemaVersion,
		Parent:        WorkDecompositionParent{Objective: "Ship intent-driven decomposition.", NonGoals: []string{"LLM planning"}},
		Children: []DecomposedWorkUnit{
			unit("contract"),
			unit("docs"),
			unit("integrate", "docs", "contract"),
		},
		CompletedWork: []string{}, AvailableInputs: []string{},
	}
}

func workUnitIDs(units []DecomposedWorkUnit) []string {
	result := make([]string, 0, len(units))
	for _, unit := range units {
		result = append(result, unit.ID)
	}
	return result
}
