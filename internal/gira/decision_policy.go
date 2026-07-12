package gira

import (
	"fmt"
	"sort"
	"strings"
)

const DecisionPolicySchemaVersion = "decision-policy/v1"

type DecisionCausalReason string

const (
	DecisionReasonMissingContext           DecisionCausalReason = "missing_context"
	DecisionReasonMissingPolicy            DecisionCausalReason = "missing_policy"
	DecisionReasonConflictingConstraints   DecisionCausalReason = "conflicting_constraints"
	DecisionReasonIrreversibleRisk         DecisionCausalReason = "irreversible_risk"
	DecisionReasonInsufficientVerification DecisionCausalReason = "insufficient_verification"
	DecisionReasonAuthorityBoundary        DecisionCausalReason = "authority_boundary"
	DecisionReasonUndefinedSuccessMetric   DecisionCausalReason = "undefined_success_metric"
)

type DecisionPolicy struct {
	SchemaVersion          string               `json:"schema_version"`
	Objective              string               `json:"objective"`
	Principles             []DecisionPrinciple  `json:"principles"`
	Constraints            []DecisionConstraint `json:"constraints"`
	AuthorityBoundaries    []string             `json:"authority_boundaries"`
	AllowReversibleDefault bool                 `json:"allow_reversible_default"`
}

type DecisionPrinciple struct {
	ID        string `json:"id"`
	Rank      int    `json:"rank"`
	Statement string `json:"statement"`
}

type DecisionConstraint struct {
	ID        string `json:"id"`
	Statement string `json:"statement"`
}

type DecisionOption struct {
	ID                   string   `json:"id"`
	Summary              string   `json:"summary"`
	Reversible           bool     `json:"reversible"`
	ViolatedConstraints  []string `json:"violated_constraints,omitempty"`
	RequiredAuthorities  []string `json:"required_authorities,omitempty"`
	Assumptions          []string `json:"assumptions,omitempty"`
	VerificationRequired []string `json:"verification_required,omitempty"`
}

type DecisionEvidence struct {
	OptionID    string `json:"option_id"`
	PrincipleID string `json:"principle_id"`
	Effect      string `json:"effect"`
	Source      string `json:"source"`
}

type DecisionPolicyRequest struct {
	Policy             DecisionPolicy       `json:"policy"`
	Question           string               `json:"question"`
	AffectedWork       []string             `json:"affected_work"`
	Options            []DecisionOption     `json:"options"`
	Evidence           []DecisionEvidence   `json:"evidence"`
	GrantedAuthorities []string             `json:"granted_authorities,omitempty"`
	UnresolvedReason   DecisionCausalReason `json:"unresolved_reason,omitempty"`
	MissingInput       string               `json:"missing_input,omitempty"`
	ResumeCondition    string               `json:"resume_condition,omitempty"`
}

type DecisionPolicyResult struct {
	SchemaVersion        string             `json:"schema_version"`
	Outcome              string             `json:"outcome"`
	SelectedOption       *DecisionOption    `json:"selected_option,omitempty"`
	AppliedPrinciples    []string           `json:"applied_principles"`
	AppliedEvidence      []DecisionEvidence `json:"applied_evidence"`
	Assumptions          []string           `json:"assumptions"`
	VerificationRequired []string           `json:"verification_required"`
	DecisionGap          *DecisionGap       `json:"decision_gap,omitempty"`
}

type DecisionGap struct {
	Reason          DecisionCausalReason `json:"reason"`
	Question        string               `json:"question"`
	AffectedWork    []string             `json:"affected_work"`
	ViableOptions   []DecisionOption     `json:"viable_options"`
	MissingInput    string               `json:"missing_input"`
	ResumeCondition string               `json:"resume_condition"`
}

func EvaluateDecisionPolicy(request DecisionPolicyRequest) (DecisionPolicyResult, error) {
	if err := validateDecisionPolicyRequest(request); err != nil {
		return DecisionPolicyResult{}, err
	}
	principles := append([]DecisionPrinciple(nil), request.Policy.Principles...)
	sort.Slice(principles, func(i, j int) bool { return principles[i].Rank < principles[j].Rank })
	viable, authorityRejected, constraintRejected := viableDecisionOptions(request)
	result := DecisionPolicyResult{
		SchemaVersion:        DecisionPolicySchemaVersion,
		AppliedPrinciples:    decisionPrincipleIDs(principles),
		AppliedEvidence:      []DecisionEvidence{},
		Assumptions:          []string{},
		VerificationRequired: []string{},
	}

	if selected, evidence, ok := strictlyPreferredDecisionOption(viable, principles, request.Evidence); ok {
		result.Outcome = "derived"
		result.SelectedOption = &selected
		result.AppliedEvidence = evidence
		result.VerificationRequired = append([]string(nil), selected.VerificationRequired...)
		return result, nil
	}
	if selected, ok := reversibleDefaultDecisionOption(viable, request.Policy.AllowReversibleDefault); ok {
		result.Outcome = "reversible_default"
		result.SelectedOption = &selected
		result.Assumptions = append([]string(nil), selected.Assumptions...)
		result.VerificationRequired = append([]string(nil), selected.VerificationRequired...)
		return result, nil
	}

	reason := request.UnresolvedReason
	if reason == "" {
		reason = inferDecisionGapReason(request, viable, authorityRejected, constraintRejected)
	}
	result.Outcome = "decision_gap"
	result.DecisionGap = &DecisionGap{
		Reason:          reason,
		Question:        strings.TrimSpace(request.Question),
		AffectedWork:    append([]string(nil), request.AffectedWork...),
		ViableOptions:   append([]DecisionOption(nil), viable...),
		MissingInput:    firstNonEmpty(strings.TrimSpace(request.MissingInput), defaultDecisionMissingInput(reason)),
		ResumeCondition: firstNonEmpty(strings.TrimSpace(request.ResumeCondition), defaultDecisionResumeCondition(reason)),
	}
	return result, nil
}

func validateDecisionPolicyRequest(request DecisionPolicyRequest) error {
	policy := request.Policy
	if policy.SchemaVersion != DecisionPolicySchemaVersion {
		return fmt.Errorf("decision policy schema_version must be %q", DecisionPolicySchemaVersion)
	}
	if strings.TrimSpace(policy.Objective) == "" {
		return fmt.Errorf("decision policy objective is required")
	}
	if len(request.Options) == 0 {
		return fmt.Errorf("decision policy requires at least one option")
	}
	if strings.TrimSpace(request.Question) == "" {
		return fmt.Errorf("decision policy question is required")
	}
	if len(request.AffectedWork) == 0 {
		return fmt.Errorf("decision policy affected_work is required")
	}
	principleIDs := map[string]bool{}
	ranks := map[int]bool{}
	for _, principle := range policy.Principles {
		id := strings.TrimSpace(principle.ID)
		if id == "" || strings.TrimSpace(principle.Statement) == "" || principle.Rank <= 0 {
			return fmt.Errorf("decision principle id, positive rank, and statement are required")
		}
		if principleIDs[id] {
			return fmt.Errorf("duplicate decision principle id %q", id)
		}
		if ranks[principle.Rank] {
			return fmt.Errorf("duplicate decision principle rank %d", principle.Rank)
		}
		principleIDs[id] = true
		ranks[principle.Rank] = true
	}
	constraintIDs := map[string]bool{}
	for _, constraint := range policy.Constraints {
		id := strings.TrimSpace(constraint.ID)
		if id == "" || strings.TrimSpace(constraint.Statement) == "" {
			return fmt.Errorf("decision constraint id and statement are required")
		}
		if constraintIDs[id] {
			return fmt.Errorf("duplicate decision constraint id %q", id)
		}
		constraintIDs[id] = true
	}
	optionIDs := map[string]bool{}
	for _, option := range request.Options {
		id := strings.TrimSpace(option.ID)
		if id == "" || strings.TrimSpace(option.Summary) == "" {
			return fmt.Errorf("decision option id and summary are required")
		}
		if optionIDs[id] {
			return fmt.Errorf("duplicate decision option id %q", id)
		}
		optionIDs[id] = true
		for _, constraint := range option.ViolatedConstraints {
			if !constraintIDs[strings.TrimSpace(constraint)] {
				return fmt.Errorf("decision option %q references unknown constraint %q", id, constraint)
			}
		}
	}
	for _, evidence := range request.Evidence {
		if !optionIDs[strings.TrimSpace(evidence.OptionID)] {
			return fmt.Errorf("decision evidence references unknown option %q", evidence.OptionID)
		}
		if !principleIDs[strings.TrimSpace(evidence.PrincipleID)] {
			return fmt.Errorf("decision evidence references unknown principle %q", evidence.PrincipleID)
		}
		if decisionEvidenceScore(evidence.Effect) == 2 {
			return fmt.Errorf("unknown decision evidence effect %q", evidence.Effect)
		}
		if strings.TrimSpace(evidence.Source) == "" {
			return fmt.Errorf("decision evidence source is required")
		}
	}
	if request.UnresolvedReason != "" && !validDecisionCausalReason(request.UnresolvedReason) {
		return fmt.Errorf("unknown decision causal reason %q", request.UnresolvedReason)
	}
	return nil
}

func viableDecisionOptions(request DecisionPolicyRequest) ([]DecisionOption, int, int) {
	boundaries := stringSet(request.Policy.AuthorityBoundaries)
	granted := stringSet(request.GrantedAuthorities)
	viable := []DecisionOption{}
	authorityRejected := 0
	constraintRejected := 0
	for _, option := range request.Options {
		if len(option.ViolatedConstraints) > 0 {
			constraintRejected++
			continue
		}
		blocked := false
		for _, authority := range option.RequiredAuthorities {
			key := strings.TrimSpace(authority)
			if boundaries[key] && !granted[key] {
				blocked = true
				break
			}
		}
		if blocked {
			authorityRejected++
			continue
		}
		viable = append(viable, option)
	}
	return viable, authorityRejected, constraintRejected
}

func strictlyPreferredDecisionOption(options []DecisionOption, principles []DecisionPrinciple, evidence []DecisionEvidence) (DecisionOption, []DecisionEvidence, bool) {
	if len(options) == 0 {
		return DecisionOption{}, nil, false
	}
	if len(options) == 1 {
		return options[0], evidenceForOption(options[0].ID, evidence), true
	}
	scores := map[string][]int{}
	for _, option := range options {
		scores[option.ID] = make([]int, len(principles))
	}
	for _, item := range evidence {
		for index, principle := range principles {
			if item.PrincipleID == principle.ID {
				scores[item.OptionID][index] += decisionEvidenceScore(item.Effect)
			}
		}
	}
	best := options[0]
	tied := false
	for _, option := range options[1:] {
		comparison := compareDecisionScores(scores[option.ID], scores[best.ID])
		if comparison > 0 {
			best = option
			tied = false
		} else if comparison == 0 {
			tied = true
		}
	}
	if tied || allDecisionScoresZero(scores[best.ID]) {
		return DecisionOption{}, nil, false
	}
	return best, evidenceForOption(best.ID, evidence), true
}

func reversibleDefaultDecisionOption(options []DecisionOption, allowed bool) (DecisionOption, bool) {
	if !allowed {
		return DecisionOption{}, false
	}
	candidates := []DecisionOption{}
	for _, option := range options {
		if option.Reversible && len(option.Assumptions) > 0 && len(option.VerificationRequired) > 0 {
			candidates = append(candidates, option)
		}
	}
	if len(candidates) != 1 {
		return DecisionOption{}, false
	}
	return candidates[0], true
}

func inferDecisionGapReason(request DecisionPolicyRequest, viable []DecisionOption, authorityRejected int, constraintRejected int) DecisionCausalReason {
	if authorityRejected > 0 && len(viable) == 0 {
		return DecisionReasonAuthorityBoundary
	}
	if constraintRejected > 0 && len(viable) == 0 {
		return DecisionReasonConflictingConstraints
	}
	if len(request.Evidence) == 0 {
		return DecisionReasonMissingContext
	}
	if len(request.Policy.Principles) == 0 {
		return DecisionReasonMissingPolicy
	}
	return DecisionReasonMissingPolicy
}

func validDecisionCausalReason(reason DecisionCausalReason) bool {
	switch reason {
	case DecisionReasonMissingContext, DecisionReasonMissingPolicy, DecisionReasonConflictingConstraints,
		DecisionReasonIrreversibleRisk, DecisionReasonInsufficientVerification, DecisionReasonAuthorityBoundary,
		DecisionReasonUndefinedSuccessMetric:
		return true
	default:
		return false
	}
}

func decisionEvidenceScore(effect string) int {
	switch strings.ToLower(strings.TrimSpace(effect)) {
	case "supports":
		return 1
	case "neutral":
		return 0
	case "violates":
		return -1
	default:
		return 2
	}
}

func compareDecisionScores(left []int, right []int) int {
	for index := range left {
		if left[index] > right[index] {
			return 1
		}
		if left[index] < right[index] {
			return -1
		}
	}
	return 0
}

func allDecisionScoresZero(scores []int) bool {
	for _, score := range scores {
		if score != 0 {
			return false
		}
	}
	return true
}

func evidenceForOption(optionID string, evidence []DecisionEvidence) []DecisionEvidence {
	result := []DecisionEvidence{}
	for _, item := range evidence {
		if item.OptionID == optionID {
			result = append(result, item)
		}
	}
	return result
}

func decisionPrincipleIDs(principles []DecisionPrinciple) []string {
	result := make([]string, 0, len(principles))
	for _, principle := range principles {
		result = append(result, principle.ID)
	}
	return result
}

func stringSet(values []string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result[trimmed] = true
		}
	}
	return result
}

func defaultDecisionMissingInput(reason DecisionCausalReason) string {
	switch reason {
	case DecisionReasonAuthorityBoundary:
		return "authority grant or an option that stays within the declared authority boundary"
	case DecisionReasonConflictingConstraints:
		return "constraint precedence or a viable option that satisfies the declared constraints"
	case DecisionReasonMissingPolicy:
		return "a principle or precedence rule that distinguishes the viable options"
	case DecisionReasonIrreversibleRisk:
		return "explicit risk acceptance or a reversible alternative"
	case DecisionReasonInsufficientVerification:
		return "a verification method that can establish the required evidence"
	case DecisionReasonUndefinedSuccessMetric:
		return "a measurable success condition"
	default:
		return "evidence needed to distinguish the viable options"
	}
}

func defaultDecisionResumeCondition(reason DecisionCausalReason) string {
	return "record the missing input for " + string(reason) + " and reevaluate the decision policy"
}
