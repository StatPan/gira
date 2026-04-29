package gira

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type GuardrailsSyncReport struct {
	Repo         string               `json:"repo"`
	Desired      GuardrailsPolicy     `json:"desired"`
	Current      GuardrailsState      `json:"current"`
	Diff         []GuardrailsDiffItem `json:"diff"`
	Applied      []string             `json:"applied"`
	Skipped      []string             `json:"skipped"`
	Blocked      []GuardrailsBlocked  `json:"blocked"`
	BlockedCount int                  `json:"blocked_count"`
}

type GuardrailsState struct {
	BranchProtection map[string]GuardrailsBranchProtection `json:"branch_protection"`
	Rulesets         []GuardrailsRulesetPolicy             `json:"rulesets"`
}

type GuardrailsDiffItem struct {
	Kind    string `json:"kind"`
	Target  string `json:"target"`
	Field   string `json:"field"`
	From    any    `json:"from"`
	To      any    `json:"to"`
	Action  string `json:"action"`
	Blocked bool   `json:"blocked"`
	Reason  string `json:"reason,omitempty"`
}

type GuardrailsBlocked struct {
	Target string `json:"target"`
	Reason string `json:"reason"`
}

type GuardrailsClient interface {
	FetchCurrentGuardrails() (GuardrailsState, error)
	ApplyBranchProtection(pattern string, cfg GuardrailsBranchProtection) error
	ApplyRuleset(rs GuardrailsRulesetPolicy) error
}

type GHGuardrailsClient struct {
	repo   RepoRef
	runner CommandRunner
}

func NewGHGuardrailsClient(repo RepoRef, runner CommandRunner) GHGuardrailsClient {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	return GHGuardrailsClient{repo: repo, runner: runner}
}

func (c GHGuardrailsClient) FetchCurrentGuardrails() (GuardrailsState, error) {
	state := GuardrailsState{BranchProtection: map[string]GuardrailsBranchProtection{}}
	branchesRaw, err := c.runner.Run("gh", "api", "repos/"+c.repo.FullName()+"/branches")
	if err != nil {
		return GuardrailsState{}, err
	}
	var branches []struct {
		Name      string `json:"name"`
		Protected bool   `json:"protected"`
	}
	if err := json.Unmarshal(branchesRaw, &branches); err != nil {
		return GuardrailsState{}, fmt.Errorf("parse branches: %w", err)
	}
	for _, b := range branches {
		if !b.Protected {
			continue
		}
		protectionRaw, err := c.runner.Run("gh", "api", "repos/"+c.repo.FullName()+"/branches/"+b.Name+"/protection")
		if err != nil {
			continue
		}
		var p struct {
			RequiredPullRequestReviews struct {
				RequiredApprovingReviewCount int  `json:"required_approving_review_count"`
				RequireCodeOwnerReviews      bool `json:"require_code_owner_reviews"`
			} `json:"required_pull_request_reviews"`
			RequiredStatusChecks struct {
				Strict bool `json:"strict"`
			} `json:"required_status_checks"`
			AllowForcePushes struct {
				Enabled bool `json:"enabled"`
			} `json:"allow_force_pushes"`
			AllowDeletions struct {
				Enabled bool `json:"enabled"`
			} `json:"allow_deletions"`
		}
		if err := json.Unmarshal(protectionRaw, &p); err != nil {
			return GuardrailsState{}, fmt.Errorf("parse protection for %s: %w", b.Name, err)
		}
		state.BranchProtection[b.Name] = GuardrailsBranchProtection{
			RequiredApprovingReviewCount: p.RequiredPullRequestReviews.RequiredApprovingReviewCount,
			RequireCodeOwnerReviews:      p.RequiredPullRequestReviews.RequireCodeOwnerReviews,
			RequiredStatusChecksStrict:   p.RequiredStatusChecks.Strict,
			AllowForcePushes:             p.AllowForcePushes.Enabled,
			AllowDeletions:               p.AllowDeletions.Enabled,
		}
	}
	rulesRaw, err := c.runner.Run("gh", "api", "repos/"+c.repo.FullName()+"/rulesets")
	if err == nil {
		var rules []struct {
			Name        string `json:"name"`
			Target      string `json:"target"`
			Enforcement string `json:"enforcement"`
		}
		if err := json.Unmarshal(rulesRaw, &rules); err != nil {
			return GuardrailsState{}, fmt.Errorf("parse rulesets: %w", err)
		}
		for _, r := range rules {
			state.Rulesets = append(state.Rulesets, GuardrailsRulesetPolicy{Name: r.Name, Target: r.Target, Enforcement: r.Enforcement})
		}
	}
	return state, nil
}

func (c GHGuardrailsClient) ApplyBranchProtection(pattern string, cfg GuardrailsBranchProtection) error {
	payload := fmt.Sprintf("required_pull_request_reviews[required_approving_review_count]=%d", cfg.RequiredApprovingReviewCount)
	args := []string{"api", "repos/" + c.repo.FullName() + "/branches/" + pattern + "/protection", "-X", "PUT", "-H", "Accept: application/vnd.github+json", "-f", payload,
		"-f", fmt.Sprintf("required_pull_request_reviews[require_code_owner_reviews]=%t", cfg.RequireCodeOwnerReviews),
		"-f", fmt.Sprintf("required_status_checks[strict]=%t", cfg.RequiredStatusChecksStrict),
		"-f", fmt.Sprintf("allow_force_pushes=%t", cfg.AllowForcePushes),
		"-f", fmt.Sprintf("allow_deletions=%t", cfg.AllowDeletions)}
	_, err := c.runner.Run("gh", args...)
	return err
}

func (c GHGuardrailsClient) ApplyRuleset(rs GuardrailsRulesetPolicy) error {
	_, err := c.runner.Run("gh", "api", "repos/"+c.repo.FullName()+"/rulesets", "-X", "POST", "-f", "name="+rs.Name, "-f", "target="+rs.Target, "-f", "enforcement="+rs.Enforcement)
	if err != nil && strings.Contains(err.Error(), "already_exists") {
		return nil
	}
	return err
}

func BuildGuardrailsReport(policy GuardrailsPolicy, current GuardrailsState, apply bool, allowRelaxation bool) GuardrailsSyncReport {
	report := GuardrailsSyncReport{
		Desired: policy,
		Current: current,
	}

	for _, pattern := range policyBranchPatterns(policy) {
		desired := policy.BranchProtection[pattern]
		cur, ok := current.BranchProtection[pattern]
		if !ok {
			report.Diff = append(report.Diff, GuardrailsDiffItem{Kind: "branch_protection", Target: pattern, Field: "create", From: nil, To: desired, Action: "update"})
			continue
		}
		appendBranchDiff(&report, pattern, "required_approving_review_count", cur.RequiredApprovingReviewCount, desired.RequiredApprovingReviewCount, allowRelaxation)
		appendBranchDiff(&report, pattern, "require_code_owner_reviews", cur.RequireCodeOwnerReviews, desired.RequireCodeOwnerReviews, allowRelaxation)
		appendBranchDiff(&report, pattern, "required_status_checks_strict", cur.RequiredStatusChecksStrict, desired.RequiredStatusChecksStrict, allowRelaxation)
		appendBranchDiff(&report, pattern, "allow_force_pushes", cur.AllowForcePushes, desired.AllowForcePushes, allowRelaxation)
		appendBranchDiff(&report, pattern, "allow_deletions", cur.AllowDeletions, desired.AllowDeletions, allowRelaxation)
	}

	policyRules := map[string]GuardrailsRulesetPolicy{}
	for _, rs := range policy.Rulesets {
		policyRules[rs.Name] = rs
	}
	for _, rs := range policy.Rulesets {
		cur, found := findRuleset(current.Rulesets, rs.Name)
		if !found {
			report.Diff = append(report.Diff, GuardrailsDiffItem{Kind: "ruleset", Target: rs.Name, Field: "create", From: nil, To: rs, Action: "update"})
			continue
		}
		if cur.Target != rs.Target {
			report.Diff = append(report.Diff, GuardrailsDiffItem{Kind: "ruleset", Target: rs.Name, Field: "target", From: cur.Target, To: rs.Target, Action: "update"})
		}
		if cur.Enforcement != rs.Enforcement {
			report.Diff = append(report.Diff, GuardrailsDiffItem{Kind: "ruleset", Target: rs.Name, Field: "enforcement", From: cur.Enforcement, To: rs.Enforcement, Action: "update"})
		}
	}

	for pattern := range current.BranchProtection {
		if _, ok := policy.BranchProtection[pattern]; !ok {
			report.Blocked = append(report.Blocked, GuardrailsBlocked{Target: pattern, Reason: "unknown branch pattern blocked"})
			report.Skipped = append(report.Skipped, "branch_protection:"+pattern)
		}
	}
	for _, rs := range current.Rulesets {
		if _, ok := policyRules[rs.Name]; !ok {
			report.Skipped = append(report.Skipped, "ruleset:"+rs.Name+":unmanaged")
		}
	}

	sort.SliceStable(report.Diff, func(i, j int) bool {
		if report.Diff[i].Kind == report.Diff[j].Kind {
			if report.Diff[i].Target == report.Diff[j].Target {
				return report.Diff[i].Field < report.Diff[j].Field
			}
			return report.Diff[i].Target < report.Diff[j].Target
		}
		return report.Diff[i].Kind < report.Diff[j].Kind
	})
	report.BlockedCount = len(report.Blocked)
	_ = apply
	return report
}

func appendBranchDiff(report *GuardrailsSyncReport, pattern, field string, from, to any, allowRelaxation bool) {
	if from == to {
		return
	}
	item := GuardrailsDiffItem{Kind: "branch_protection", Target: pattern, Field: field, From: from, To: to, Action: "update"}
	if isRelaxation(field, from, to) && !allowRelaxation {
		item.Blocked = true
		item.Reason = "relaxation denied: pass --allow-relaxation"
		report.Skipped = append(report.Skipped, "branch_protection:"+pattern+":"+field+":relaxation_blocked")
	}
	report.Diff = append(report.Diff, item)
}

func isRelaxation(field string, from, to any) bool {
	switch field {
	case "required_approving_review_count":
		fi := from.(int)
		ti := to.(int)
		return ti < fi
	case "require_code_owner_reviews", "required_status_checks_strict":
		return from.(bool) && !to.(bool)
	}
	return false
}

func findRuleset(items []GuardrailsRulesetPolicy, name string) (GuardrailsRulesetPolicy, bool) {
	for _, i := range items {
		if i.Name == name {
			return i, true
		}
	}
	return GuardrailsRulesetPolicy{}, false
}

func SyncGuardrailsForClient(repo RepoRef, policy GuardrailsPolicy, client GuardrailsClient, apply bool, allowRelaxation bool) (GuardrailsSyncReport, error) {
	current, err := client.FetchCurrentGuardrails()
	if err != nil {
		return GuardrailsSyncReport{}, err
	}
	report := BuildGuardrailsReport(policy, current, apply, allowRelaxation)
	report.Repo = repo.FullName()
	if !apply {
		return report, nil
	}
	appliedBranches := map[string]bool{}
	appliedRulesets := map[string]bool{}
	for _, diff := range report.Diff {
		if diff.Blocked {
			continue
		}
		switch diff.Kind {
		case "branch_protection":
			if appliedBranches[diff.Target] {
				continue
			}
			if err := client.ApplyBranchProtection(diff.Target, policy.BranchProtection[diff.Target]); err != nil {
				return GuardrailsSyncReport{}, fmt.Errorf("apply branch protection %s: %w", diff.Target, err)
			}
			appliedBranches[diff.Target] = true
			report.Applied = append(report.Applied, "branch_protection:"+diff.Target)
		case "ruleset":
			if appliedRulesets[diff.Target] {
				continue
			}
			rs, _ := findRuleset(policy.Rulesets, diff.Target)
			if err := client.ApplyRuleset(rs); err != nil {
				return GuardrailsSyncReport{}, fmt.Errorf("apply ruleset %s: %w", rs.Name, err)
			}
			appliedRulesets[diff.Target] = true
			report.Applied = append(report.Applied, "ruleset:"+rs.Name)
		}
	}
	sort.Strings(report.Applied)
	return report, nil
}
