package gira

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecisionPolicyDerivesStrictPreference(t *testing.T) {
	request := decisionPolicyTestRequest()
	request.Evidence = []DecisionEvidence{
		{OptionID: "safe", PrincipleID: "safety", Effect: "supports", Source: "rollback evidence"},
		{OptionID: "fast", PrincipleID: "safety", Effect: "violates", Source: "irreversible migration"},
		{OptionID: "fast", PrincipleID: "speed", Effect: "supports", Source: "shorter execution"},
	}

	result, err := EvaluateDecisionPolicy(request)
	if err != nil {
		t.Fatalf("EvaluateDecisionPolicy() error = %v", err)
	}
	if result.Outcome != "derived" || result.SelectedOption == nil || result.SelectedOption.ID != "safe" {
		t.Fatalf("result = %+v, want derived safe option", result)
	}
	if len(result.AppliedEvidence) != 1 || result.AppliedEvidence[0].PrincipleID != "safety" {
		t.Fatalf("applied evidence = %+v", result.AppliedEvidence)
	}
	if strings.Join(result.AppliedPrinciples, ",") != "safety,speed" {
		t.Fatalf("applied principles = %+v", result.AppliedPrinciples)
	}
}

func TestDecisionPolicyChoosesSingleVerifiableReversibleDefault(t *testing.T) {
	request := decisionPolicyTestRequest()
	request.Policy.AllowReversibleDefault = true
	request.Options[0].Reversible = true
	request.Options[0].Assumptions = []string{"mock behavior represents the provider contract"}
	request.Options[0].VerificationRequired = []string{"run the provider contract suite"}

	result, err := EvaluateDecisionPolicy(request)
	if err != nil {
		t.Fatalf("EvaluateDecisionPolicy() error = %v", err)
	}
	if result.Outcome != "reversible_default" || result.SelectedOption == nil || result.SelectedOption.ID != "safe" {
		t.Fatalf("result = %+v, want reversible default", result)
	}
	if len(result.Assumptions) != 1 || len(result.VerificationRequired) != 1 {
		t.Fatalf("result does not preserve assumption and verification: %+v", result)
	}
}

func TestDecisionPolicyReturnsConflictingConstraintGap(t *testing.T) {
	request := decisionPolicyTestRequest()
	request.Policy.Constraints = []DecisionConstraint{{ID: "no-network", Statement: "Do not use network access."}}
	request.Options[0].ViolatedConstraints = []string{"no-network"}
	request.Options[1].ViolatedConstraints = []string{"no-network"}
	request.Question = "Which implementation path remains viable?"
	request.AffectedWork = []string{"ticket #840", "provider verification"}

	result, err := EvaluateDecisionPolicy(request)
	if err != nil {
		t.Fatalf("EvaluateDecisionPolicy() error = %v", err)
	}
	assertDecisionGap(t, result, DecisionReasonConflictingConstraints)
	if len(result.DecisionGap.AffectedWork) != 2 || result.DecisionGap.Question != request.Question {
		t.Fatalf("decision gap is not minimal and actionable: %+v", result.DecisionGap)
	}
}

func TestDecisionPolicyReturnsAuthorityBoundaryGap(t *testing.T) {
	request := decisionPolicyTestRequest()
	request.Policy.AuthorityBoundaries = []string{"production-deploy"}
	for index := range request.Options {
		request.Options[index].RequiredAuthorities = []string{"production-deploy"}
	}

	result, err := EvaluateDecisionPolicy(request)
	if err != nil {
		t.Fatalf("EvaluateDecisionPolicy() error = %v", err)
	}
	assertDecisionGap(t, result, DecisionReasonAuthorityBoundary)
	if !strings.Contains(result.DecisionGap.MissingInput, "authority grant") {
		t.Fatalf("missing input = %q", result.DecisionGap.MissingInput)
	}
}

func TestDecisionPolicyReturnsMissingContextGap(t *testing.T) {
	request := decisionPolicyTestRequest()
	request.Question = "Which CI strategy satisfies the user intent?"
	request.MissingInput = "provider failure evidence"
	request.ResumeCondition = "attach the provider failure evidence and reevaluate"

	result, err := EvaluateDecisionPolicy(request)
	if err != nil {
		t.Fatalf("EvaluateDecisionPolicy() error = %v", err)
	}
	assertDecisionGap(t, result, DecisionReasonMissingContext)
	if result.DecisionGap.MissingInput != request.MissingInput || result.DecisionGap.ResumeCondition != request.ResumeCondition {
		t.Fatalf("decision gap lost explicit resume inputs: %+v", result.DecisionGap)
	}
}

func TestDecisionPolicyValidation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*DecisionPolicyRequest)
		wantErr string
	}{
		{name: "missing objective", mutate: func(r *DecisionPolicyRequest) { r.Policy.Objective = "" }, wantErr: "objective is required"},
		{name: "duplicate rank", mutate: func(r *DecisionPolicyRequest) { r.Policy.Principles[1].Rank = 1 }, wantErr: "duplicate decision principle rank"},
		{name: "empty option set", mutate: func(r *DecisionPolicyRequest) { r.Options = nil }, wantErr: "at least one option"},
		{name: "missing question", mutate: func(r *DecisionPolicyRequest) { r.Question = "" }, wantErr: "question is required"},
		{name: "missing affected work", mutate: func(r *DecisionPolicyRequest) { r.AffectedWork = nil }, wantErr: "affected_work is required"},
		{name: "unknown reason", mutate: func(r *DecisionPolicyRequest) { r.UnresolvedReason = "surprise" }, wantErr: "unknown decision causal reason"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := decisionPolicyTestRequest()
			tt.mutate(&request)
			_, err := EvaluateDecisionPolicy(request)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestDecisionPolicyJSONRoundTrip(t *testing.T) {
	request := decisionPolicyTestRequest()
	request.UnresolvedReason = DecisionReasonInsufficientVerification
	result, err := EvaluateDecisionPolicy(request)
	if err != nil {
		t.Fatalf("EvaluateDecisionPolicy() error = %v", err)
	}
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var decoded DecisionPolicyResult
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded.SchemaVersion != DecisionPolicySchemaVersion || decoded.DecisionGap == nil || decoded.DecisionGap.Reason != DecisionReasonInsufficientVerification {
		t.Fatalf("decoded result = %+v", decoded)
	}
}

func decisionPolicyTestRequest() DecisionPolicyRequest {
	return DecisionPolicyRequest{
		Policy: DecisionPolicy{
			SchemaVersion: DecisionPolicySchemaVersion,
			Objective:     "Choose a safe implementation path.",
			Principles: []DecisionPrinciple{
				{ID: "safety", Rank: 1, Statement: "Preserve completion safety."},
				{ID: "speed", Rank: 2, Statement: "Reduce delivery time."},
			},
			Constraints:         []DecisionConstraint{},
			AuthorityBoundaries: []string{},
		},
		Question:     "Which option should be selected?",
		AffectedWork: []string{"ticket #840"},
		Options: []DecisionOption{
			{ID: "safe", Summary: "Use a bounded mock-backed implementation."},
			{ID: "fast", Summary: "Apply the provider change directly."},
		},
		Evidence: []DecisionEvidence{},
	}
}

func assertDecisionGap(t *testing.T, result DecisionPolicyResult, reason DecisionCausalReason) {
	t.Helper()
	if result.Outcome != "decision_gap" || result.DecisionGap == nil || result.DecisionGap.Reason != reason {
		t.Fatalf("result = %+v, want decision gap reason %s", result, reason)
	}
	if result.DecisionGap.Question == "" || result.DecisionGap.MissingInput == "" || result.DecisionGap.ResumeCondition == "" {
		t.Fatalf("decision gap is incomplete: %+v", result.DecisionGap)
	}
}
