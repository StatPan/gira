package gira

import (
	"fmt"
	"strings"
	"time"
)

type JiraParityReport struct {
	Repo      string             `json:"repo"`
	Command   string             `json:"command"`
	Generated string             `json:"generated_at"`
	Weights   JiraParityWeights  `json:"weights"`
	Scores    JiraParityScores   `json:"scores"`
	Domains   []JiraParityDomain `json:"domains"`
	Blockers  []JiraParityGap    `json:"blockers"`
	NextSteps []string           `json:"next_steps"`
	Missing   []JiraParityGap    `json:"missing_surfaces"`
	Ready     bool               `json:"ready"`
}

type JiraParityWeights struct {
	InstallDetach       int `json:"install_detach"`
	JiraMigration       int `json:"jira_migration"`
	DailyWorkLoop       int `json:"daily_work_loop"`
	SprintRelease       int `json:"sprint_release_report"`
	GovernanceReadiness int `json:"governance_readiness"`
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
	Missing  []JiraParityGap `json:"missing"`
	Evidence []string        `json:"evidence"`
}

type JiraParityGap struct {
	Capability string `json:"capability"`
	Command    string `json:"command"`
	Reason     string `json:"reason"`
}

type JiraParityEvidence struct {
	Commands map[string]bool
}

func BuildJiraParityReport(repo RepoRef, capability ProjectCapabilityReport, now time.Time) JiraParityReport {
	return BuildJiraParityReportWithEvidence(repo, capability, DefaultJiraParityEvidence(), now)
}

func BuildJiraParityReportWithEvidence(repo RepoRef, capability ProjectCapabilityReport, evidence JiraParityEvidence, now time.Time) JiraParityReport {
	weights := JiraParityWeights{InstallDetach: 20, JiraMigration: 20, DailyWorkLoop: 25, SprintRelease: 25, GovernanceReadiness: 10}
	domains := []JiraParityDomain{
		buildDomain("install_detach", weights.InstallDetach, capability, evidence, []domainCheck{
			{capability: "issues:read", command: "gira init"},
			{capability: "issues:write", command: "gira bootstrap"},
			{command: "gira doctor"},
			{command: "gira detach"},
		}),
		buildDomain("jira_migration", weights.JiraMigration, capability, evidence, []domainCheck{
			{capability: "issues:read", command: "gira sync --dry-run"},
			{capability: "issues:write", command: "gira sync --policy-mode adopt|merge|enforce"},
			{capability: "issues:write", command: "gira jira import"},
			{capability: "issues:read", command: "gira jira export"},
			{capability: "issues:write", command: "gira triage --apply"},
			{command: "gira contract crud"},
		}),
		buildDomain("daily_work_loop", weights.DailyWorkLoop, capability, evidence, []domainCheck{
			{capability: "issues:read", command: "gira status"},
			{capability: "issues:write", command: "gira work start"},
			{capability: "pullrequests:write", command: "gira work pr"},
			{capability: "issues:read", command: "gira work status"},
			{capability: "issues:write", command: "gira worker claim|handoff|release"},
		}),
		buildDomain("sprint_release_report", weights.SprintRelease, capability, evidence, []domainCheck{
			{capability: "issues:write", command: "gira sprint plan|start|close"},
			{capability: "repo:milestone:close", command: "gira sprint rollover"},
			{capability: "pullrequests:read", command: "gira release readiness"},
			{capability: "issues:read", command: "gira report weekly"},
			{capability: "issues:read", command: "gira export dashboard"},
		}),
		buildDomain("governance_readiness", weights.GovernanceReadiness, capability, evidence, []domainCheck{
			{capability: "repo:settings:write", command: "gira guardrails sync --apply"},
			{capability: "issues:read", command: "gira graph validate"},
			{capability: "pullrequests:read", command: "gira review queue|gate"},
			{capability: "pullrequests:write", command: "gira merge queue --apply"},
			{command: "gira audit verify"},
		}),
	}

	total := weights.InstallDetach + weights.JiraMigration + weights.DailyWorkLoop + weights.SprintRelease + weights.GovernanceReadiness
	earned := 0
	allBlockers := make([]JiraParityGap, 0)
	missingSurfaces := make([]JiraParityGap, 0)
	for _, domain := range domains {
		if domain.Pass {
			earned += domain.Weight
		} else {
			allBlockers = append(allBlockers, domain.Missing...)
			for _, gap := range domain.Missing {
				if gap.Capability == "" {
					missingSurfaces = append(missingSurfaces, gap)
				}
			}
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
		Domains:   domains,
		Blockers:  allBlockers,
		NextSteps: jiraParityNextSteps(allBlockers),
		Missing:   missingSurfaces,
		Ready:     len(allBlockers) == 0,
	}
}

type domainCheck struct {
	capability string
	command    string
}

func buildDomain(name string, weight int, report ProjectCapabilityReport, evidence JiraParityEvidence, checks []domainCheck) JiraParityDomain {
	missing := make([]JiraParityGap, 0)
	present := make([]string, 0, len(checks))
	for _, check := range checks {
		if !evidence.Commands[check.command] {
			missing = append(missing, JiraParityGap{Command: check.command, Reason: "v1 command surface is not available"})
			continue
		}
		present = append(present, check.command)
		if check.capability == "" {
			continue
		}
		if report.Capabilities[check.capability] != ProjectCapabilityAllowed {
			reason := CapabilityDeniedReason(report, check.capability)
			if reason == "" {
				reason = "permission denied"
			}
			missing = append(missing, JiraParityGap{Capability: check.capability, Command: check.command, Reason: reason})
		}
	}
	return JiraParityDomain{Name: name, Weight: weight, Pass: len(missing) == 0, Missing: missing, Evidence: present}
}

func DefaultJiraParityEvidence() JiraParityEvidence {
	return JiraParityEvidence{Commands: map[string]bool{
		"gira init":           true,
		"gira bootstrap":      true,
		"gira doctor":         true,
		"gira detach":         true,
		"gira sync --dry-run": true,
		"gira sync --policy-mode adopt|merge|enforce": true,
		"gira jira import":                            true,
		"gira jira export":                            true,
		"gira triage --apply":                         true,
		"gira contract crud":                          true,
		"gira status":                                 true,
		"gira work start":                             true,
		"gira work pr":                                true,
		"gira work status":                            true,
		"gira worker claim|handoff|release":           true,
		"gira sprint plan|start|close":                true,
		"gira sprint rollover":                        true,
		"gira release readiness":                      true,
		"gira report weekly":                          true,
		"gira export dashboard":                       true,
		"gira guardrails sync --apply":                true,
		"gira graph validate":                         true,
		"gira review queue|gate":                      true,
		"gira merge queue --apply":                    true,
		"gira audit verify":                           true,
	}}
}

func jiraParityNextSteps(blockers []JiraParityGap) []string {
	steps := make([]string, 0)
	seen := map[string]bool{}
	for _, blocker := range blockers {
		step := strings.TrimSpace(blocker.Command)
		if step == "" {
			step = "restore missing capability: " + blocker.Capability
		}
		if blocker.Capability != "" {
			step = fmt.Sprintf("grant %s for %s", blocker.Capability, blocker.Command)
		}
		if !seen[step] {
			seen[step] = true
			steps = append(steps, step)
		}
	}
	return steps
}

func FormatJiraParityReport(report JiraParityReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "jira parity: %d/%d (%d%%) ready=%t\n", report.Scores.Earned, report.Scores.Total, report.Scores.Pct, report.Ready)
	fmt.Fprintf(&b, "repo: %s\n", report.Repo)
	if len(report.Blockers) == 0 {
		b.WriteString("blockers: none\n")
	} else {
		b.WriteString("blockers:\n")
		for _, blocker := range report.Blockers {
			fmt.Fprintf(&b, "  - %s: %s\n", blocker.Command, blocker.Reason)
		}
	}
	if len(report.NextSteps) > 0 {
		b.WriteString("next steps:\n")
		for _, step := range report.NextSteps {
			fmt.Fprintf(&b, "  - %s\n", step)
		}
	}
	if len(report.Missing) > 0 {
		b.WriteString("missing surfaces:\n")
		for _, missing := range report.Missing {
			fmt.Fprintf(&b, "  - %s\n", missing.Command)
		}
	}
	b.WriteString("domains:\n")
	for _, domain := range report.Domains {
		fmt.Fprintf(&b, "  - %s: %t (%d)\n", domain.Name, domain.Pass, domain.Weight)
	}
	return b.String()
}
