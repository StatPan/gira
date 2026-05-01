package gira

import "time"

type JiraParityReport struct {
	Repo      string             `json:"repo"`
	Command   string             `json:"command"`
	Generated string             `json:"generated_at"`
	Weights   JiraParityWeights  `json:"weights"`
	Scores    JiraParityScores   `json:"scores"`
	Domains   []JiraParityDomain `json:"domains"`
	Blockers  []JiraParityGap    `json:"blockers"`
	Ready     bool               `json:"ready"`
}

type JiraParityWeights struct {
	Onboarding     int `json:"onboarding"`
	PMLifecycle    int `json:"pm_lifecycle"`
	ExecutionLoop  int `json:"execution_loop"`
	TeamGuardrails int `json:"team_guardrails"`
	Visibility     int `json:"visibility"`
}

type JiraParityScores struct {
	Earned int `json:"earned"`
	Total  int `json:"total"`
	Pct    int `json:"percent"`
}

type JiraParityDomain struct {
	Name     string          `json:"name"`
	Weight   int             `json:"weight"`
	Pass     bool            `json:"pass"`
	Evidence []string        `json:"evidence"`
	Missing  []JiraParityGap `json:"missing"`
}

type JiraParityGap struct {
	Capability string `json:"capability"`
	Command    string `json:"command"`
	Reason     string `json:"reason"`
}

func BuildJiraParityReport(repo RepoRef, capability ProjectCapabilityReport, now time.Time) JiraParityReport {
	weights := JiraParityWeights{Onboarding: 20, PMLifecycle: 25, ExecutionLoop: 20, TeamGuardrails: 20, Visibility: 15}
	domains := []JiraParityDomain{
		buildDomain("onboarding", weights.Onboarding, capability, []domainCheck{{"issues:read", "gira init"}, {"projectsv2:read", "gira project capability"}}, []string{"gira init", "gira project capability"}),
		buildDomain("pm_lifecycle", weights.PMLifecycle, capability, []domainCheck{{"projectsv2:read", "gira project sync --dry-run"}, {"issues:write", "gira project transitions --apply"}}, []string{"gira project sync --dry-run", "gira project transitions --apply"}),
		buildDomain("execution_loop", weights.ExecutionLoop, capability, []domainCheck{{"issues:read", "gira dev start"}, {"pullrequests:write", "gira dev pr open"}}, []string{"gira dev start", "gira dev pr open"}),
		buildDomain("team_guardrails", weights.TeamGuardrails, capability, []domainCheck{{"repo:settings:write", "gira guardrails sync --apply"}, {"issues:write", "gira worker claim"}}, []string{"gira guardrails sync --apply", "gira worker claim"}),
		buildDomain("visibility", weights.Visibility, capability, []domainCheck{{"issues:read", "gira status"}, {"projectsv2:read", "gira export dashboard"}}, []string{"gira status", "gira export dashboard"}),
	}

	total := weights.Onboarding + weights.PMLifecycle + weights.ExecutionLoop + weights.TeamGuardrails + weights.Visibility
	earned := 0
	allBlockers := make([]JiraParityGap, 0)
	for _, domain := range domains {
		if domain.Pass {
			earned += domain.Weight
		} else {
			allBlockers = append(allBlockers, domain.Missing...)
		}
	}
	pct := 0
	if total > 0 {
		pct = (earned * 100) / total
	}

	return JiraParityReport{
		Repo:      repo.FullName(),
		Command:   "parity jira",
		Generated: now.UTC().Format(time.RFC3339),
		Weights:   weights,
		Scores: JiraParityScores{
			Earned: earned,
			Total:  total,
			Pct:    pct,
		},
		Domains:  domains,
		Blockers: allBlockers,
		Ready:    len(allBlockers) == 0,
	}
}

type domainCheck struct {
	capability string
	command    string
}

func buildDomain(name string, weight int, report ProjectCapabilityReport, checks []domainCheck, evidence []string) JiraParityDomain {
	missing := make([]JiraParityGap, 0)
	for _, check := range checks {
		status := report.Capabilities[check.capability]
		if status != ProjectCapabilityAllowed {
			reason := CapabilityDeniedReason(report, check.capability)
			if reason == "" {
				reason = "permission denied"
			}
			missing = append(missing, JiraParityGap{Capability: check.capability, Command: check.command, Reason: reason})
		}
	}
	return JiraParityDomain{Name: name, Weight: weight, Pass: len(missing) == 0, Evidence: evidence, Missing: missing}
}
