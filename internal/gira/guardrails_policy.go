package gira

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type GuardrailsPolicy struct {
	BranchProtection map[string]GuardrailsBranchProtection `yaml:"branch_protection" json:"branch_protection"`
	Rulesets         []GuardrailsRulesetPolicy             `yaml:"rulesets" json:"rulesets"`
}

type GuardrailsBranchProtection struct {
	RequiredApprovingReviewCount int  `yaml:"required_approving_review_count" json:"required_approving_review_count"`
	RequireCodeOwnerReviews      bool `yaml:"require_code_owner_reviews" json:"require_code_owner_reviews"`
	RequiredStatusChecksStrict   bool `yaml:"required_status_checks_strict" json:"required_status_checks_strict"`
	AllowForcePushes             bool `yaml:"allow_force_pushes" json:"allow_force_pushes"`
	AllowDeletions               bool `yaml:"allow_deletions" json:"allow_deletions"`
}

type GuardrailsRulesetPolicy struct {
	Name        string `yaml:"name" json:"name"`
	Target      string `yaml:"target" json:"target"`
	Enforcement string `yaml:"enforcement" json:"enforcement"`
}

func LoadGuardrailsPolicy(path string) (GuardrailsPolicy, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return GuardrailsPolicy{}, err
	}
	var policy GuardrailsPolicy
	if err := yaml.Unmarshal(content, &policy); err != nil {
		return GuardrailsPolicy{}, fmt.Errorf("invalid policy: %w", err)
	}
	if err := ValidateGuardrailsPolicy(policy); err != nil {
		return GuardrailsPolicy{}, err
	}
	return policy, nil
}

func ValidateGuardrailsPolicy(policy GuardrailsPolicy) error {
	if len(policy.BranchProtection) == 0 {
		return fmt.Errorf("invalid policy: branch_protection must not be empty")
	}
	for pattern, cfg := range policy.BranchProtection {
		if !isAllowedBranchPattern(pattern) {
			return fmt.Errorf("invalid policy: unknown branch pattern %q", pattern)
		}
		if cfg.RequiredApprovingReviewCount < 0 {
			return fmt.Errorf("invalid policy: required_approving_review_count must be >= 0 for %q", pattern)
		}
	}
	for i, rs := range policy.Rulesets {
		if strings.TrimSpace(rs.Name) == "" {
			return fmt.Errorf("invalid policy: rulesets[%d].name is required", i)
		}
		if strings.TrimSpace(rs.Target) == "" {
			return fmt.Errorf("invalid policy: rulesets[%d].target is required", i)
		}
		if strings.TrimSpace(rs.Enforcement) == "" {
			return fmt.Errorf("invalid policy: rulesets[%d].enforcement is required", i)
		}
	}
	return nil
}

func isAllowedBranchPattern(pattern string) bool {
	if strings.TrimSpace(pattern) == "" {
		return false
	}
	if strings.Contains(pattern, " ") || strings.Contains(pattern, "**") {
		return false
	}
	return true
}

func policyBranchPatterns(policy GuardrailsPolicy) []string {
	keys := make([]string, 0, len(policy.BranchProtection))
	for k := range policy.BranchProtection {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
