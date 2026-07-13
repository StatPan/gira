package gira

import (
	"fmt"
	"sort"
	"strings"
)

const WorkDecompositionSchemaVersion = "work-decomposition/v1"

type WorkDecomposition struct {
	SchemaVersion   string                  `json:"schema_version"`
	Parent          WorkDecompositionParent `json:"parent"`
	Children        []DecomposedWorkUnit    `json:"children"`
	CompletedWork   []string                `json:"completed_work"`
	AvailableInputs []string                `json:"available_inputs"`
}

type WorkDecompositionParent struct {
	Objective string   `json:"objective"`
	NonGoals  []string `json:"non_goals"`
}

type DecomposedWorkUnit struct {
	ID             string                      `json:"id"`
	Title          string                      `json:"title"`
	Goal           string                      `json:"goal"`
	Dependencies   []string                    `json:"dependencies"`
	RequiredInputs []string                    `json:"required_inputs"`
	Outputs        []string                    `json:"outputs"`
	Acceptance     []DecompositionAcceptance   `json:"acceptance"`
	Verification   []DecompositionVerification `json:"verification"`
	DecisionGap    *DecisionGap                `json:"decision_gap,omitempty"`
}

type DecompositionAcceptance struct {
	Criterion  string `json:"criterion"`
	Measurable bool   `json:"measurable"`
}

type DecompositionVerification struct {
	ID               string `json:"id"`
	Method           string `json:"method"`
	ExpectedEvidence string `json:"expected_evidence"`
}

type WorkDecompositionResult struct {
	SchemaVersion string                 `json:"schema_version"`
	Order         []string               `json:"order"`
	Ready         []DecomposedWorkUnit   `json:"ready"`
	Waiting       []DecompositionWaiting `json:"waiting"`
	Gaps          []DecisionGap          `json:"gaps"`
}

type DecompositionWaiting struct {
	WorkID              string   `json:"work_id"`
	MissingDependencies []string `json:"missing_dependencies"`
	MissingInputs       []string `json:"missing_inputs"`
}

func EvaluateWorkDecomposition(decomposition WorkDecomposition) (WorkDecompositionResult, error) {
	units, order, err := validateWorkDecomposition(decomposition)
	if err != nil {
		return WorkDecompositionResult{}, err
	}
	result := WorkDecompositionResult{
		SchemaVersion: WorkDecompositionSchemaVersion,
		Order:         order,
		Ready:         []DecomposedWorkUnit{},
		Waiting:       []DecompositionWaiting{},
		Gaps:          []DecisionGap{},
	}
	completed := decompositionStringSet(decomposition.CompletedWork)
	available := decompositionStringSet(decomposition.AvailableInputs)
	for _, id := range order {
		unit := units[id]
		if completed[id] {
			continue
		}
		if unit.DecisionGap != nil {
			result.Gaps = append(result.Gaps, cloneDecisionGap(*unit.DecisionGap))
			continue
		}
		waiting := DecompositionWaiting{WorkID: id, MissingDependencies: []string{}, MissingInputs: []string{}}
		for _, dependency := range unit.Dependencies {
			if !completed[dependency] {
				waiting.MissingDependencies = append(waiting.MissingDependencies, dependency)
			}
		}
		for _, input := range unit.RequiredInputs {
			if !available[input] {
				waiting.MissingInputs = append(waiting.MissingInputs, input)
			}
		}
		sort.Strings(waiting.MissingDependencies)
		sort.Strings(waiting.MissingInputs)
		if len(waiting.MissingDependencies) > 0 || len(waiting.MissingInputs) > 0 {
			result.Waiting = append(result.Waiting, waiting)
			if len(waiting.MissingInputs) > 0 {
				result.Gaps = append(result.Gaps, missingInputDecisionGap(unit, waiting.MissingInputs))
			}
			continue
		}
		result.Ready = append(result.Ready, cloneDecomposedWorkUnit(unit))
	}
	return result, nil
}

func validateWorkDecomposition(decomposition WorkDecomposition) (map[string]DecomposedWorkUnit, []string, error) {
	if decomposition.SchemaVersion != WorkDecompositionSchemaVersion {
		return nil, nil, fmt.Errorf("work decomposition schema_version must be %q", WorkDecompositionSchemaVersion)
	}
	if strings.TrimSpace(decomposition.Parent.Objective) == "" {
		return nil, nil, fmt.Errorf("work decomposition parent objective is required")
	}
	if len(decomposition.Children) == 0 {
		return nil, nil, fmt.Errorf("work decomposition requires at least one child")
	}
	units := map[string]DecomposedWorkUnit{}
	for _, unit := range decomposition.Children {
		id := strings.TrimSpace(unit.ID)
		if id == "" || strings.TrimSpace(unit.Title) == "" || strings.TrimSpace(unit.Goal) == "" {
			return nil, nil, fmt.Errorf("decomposed work id, title, and goal are required")
		}
		if _, exists := units[id]; exists {
			return nil, nil, fmt.Errorf("duplicate decomposed work id %q", id)
		}
		if !decompositionHasText(unit.Outputs) {
			return nil, nil, fmt.Errorf("decomposed work %q requires at least one output", id)
		}
		if len(unit.Acceptance) == 0 {
			return nil, nil, fmt.Errorf("decomposed work %q requires acceptance criteria", id)
		}
		for _, acceptance := range unit.Acceptance {
			if strings.TrimSpace(acceptance.Criterion) == "" || !acceptance.Measurable {
				return nil, nil, fmt.Errorf("decomposed work %q has unmeasurable acceptance", id)
			}
		}
		if len(unit.Verification) == 0 {
			return nil, nil, fmt.Errorf("decomposed work %q requires verification", id)
		}
		verificationIDs := map[string]bool{}
		for _, verification := range unit.Verification {
			verificationID := strings.TrimSpace(verification.ID)
			if verificationID == "" || strings.TrimSpace(verification.Method) == "" || strings.TrimSpace(verification.ExpectedEvidence) == "" {
				return nil, nil, fmt.Errorf("decomposed work %q has incomplete verification", id)
			}
			if verificationIDs[verificationID] {
				return nil, nil, fmt.Errorf("decomposed work %q has duplicate verification id %q", id, verificationID)
			}
			verificationIDs[verificationID] = true
		}
		if unit.DecisionGap != nil {
			if err := validateDecompositionDecisionGap(id, *unit.DecisionGap); err != nil {
				return nil, nil, err
			}
		}
		unit.ID = id
		unit.Dependencies = decompositionUniqueSorted(unit.Dependencies)
		unit.RequiredInputs = decompositionUniqueSorted(unit.RequiredInputs)
		unit.Outputs = decompositionUniqueSorted(unit.Outputs)
		units[id] = unit
	}
	for id, unit := range units {
		for _, dependency := range unit.Dependencies {
			if dependency == id {
				return nil, nil, fmt.Errorf("decomposed work %q cannot depend on itself", id)
			}
			if _, exists := units[dependency]; !exists {
				return nil, nil, fmt.Errorf("decomposed work %q references missing dependency %q", id, dependency)
			}
		}
	}
	for _, completed := range decomposition.CompletedWork {
		completed = strings.TrimSpace(completed)
		if completed == "" {
			continue
		}
		if _, exists := units[completed]; !exists {
			return nil, nil, fmt.Errorf("completion evidence references unknown work %q", completed)
		}
	}
	order, err := decompositionTopologicalOrder(units)
	if err != nil {
		return nil, nil, err
	}
	return units, order, nil
}

func decompositionTopologicalOrder(units map[string]DecomposedWorkUnit) ([]string, error) {
	indegree := map[string]int{}
	dependents := map[string][]string{}
	for id, unit := range units {
		indegree[id] = len(unit.Dependencies)
		for _, dependency := range unit.Dependencies {
			dependents[dependency] = append(dependents[dependency], id)
		}
	}
	ready := []string{}
	for id, degree := range indegree {
		if degree == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)
	order := []string{}
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		order = append(order, id)
		children := append([]string(nil), dependents[id]...)
		sort.Strings(children)
		for _, child := range children {
			indegree[child]--
			if indegree[child] == 0 {
				ready = append(ready, child)
				sort.Strings(ready)
			}
		}
	}
	if len(order) != len(units) {
		return nil, fmt.Errorf("work decomposition dependency graph contains a cycle")
	}
	return order, nil
}

func validateDecompositionDecisionGap(workID string, gap DecisionGap) error {
	if !validDecisionCausalReason(gap.Reason) {
		return fmt.Errorf("decomposed work %q has unknown decision gap reason %q", workID, gap.Reason)
	}
	if strings.TrimSpace(gap.Question) == "" || strings.TrimSpace(gap.MissingInput) == "" || strings.TrimSpace(gap.ResumeCondition) == "" {
		return fmt.Errorf("decomposed work %q has incomplete decision gap", workID)
	}
	if len(gap.AffectedWork) == 0 {
		return fmt.Errorf("decomposed work %q decision gap requires affected_work", workID)
	}
	return nil
}

func missingInputDecisionGap(unit DecomposedWorkUnit, missing []string) DecisionGap {
	return DecisionGap{
		Reason:          DecisionReasonMissingContext,
		Question:        fmt.Sprintf("Which inputs are required before %s can start?", unit.ID),
		AffectedWork:    []string{unit.ID},
		ViableOptions:   []DecisionOption{},
		MissingInput:    strings.Join(missing, ","),
		ResumeCondition: fmt.Sprintf("provide inputs %s and reevaluate the decomposition", strings.Join(missing, ",")),
	}
}

func cloneDecisionGap(gap DecisionGap) DecisionGap {
	gap.AffectedWork = append([]string(nil), gap.AffectedWork...)
	gap.ViableOptions = append([]DecisionOption(nil), gap.ViableOptions...)
	return gap
}

func cloneDecomposedWorkUnit(unit DecomposedWorkUnit) DecomposedWorkUnit {
	unit.Dependencies = append([]string(nil), unit.Dependencies...)
	unit.RequiredInputs = append([]string(nil), unit.RequiredInputs...)
	unit.Outputs = append([]string(nil), unit.Outputs...)
	unit.Acceptance = append([]DecompositionAcceptance(nil), unit.Acceptance...)
	unit.Verification = append([]DecompositionVerification(nil), unit.Verification...)
	if unit.DecisionGap != nil {
		gap := cloneDecisionGap(*unit.DecisionGap)
		unit.DecisionGap = &gap
	}
	return unit
}

func decompositionHasText(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func decompositionStringSet(values []string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result[value] = true
		}
	}
	return result
}

func decompositionUniqueSorted(values []string) []string {
	set := decompositionStringSet(values)
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
