package gira

import (
	"fmt"
	"strings"
	"time"
)

type TicketChecksReport struct {
	Repo     string       `json:"repo"`
	Issue    int          `json:"issue"`
	PRNumber int          `json:"pr_number,omitempty"`
	PRURL    string       `json:"pr_url,omitempty"`
	State    string       `json:"state,omitempty"`
	Ready    bool         `json:"ready"`
	Wait     string       `json:"wait,omitempty"`
	Blockers []string     `json:"blockers"`
	Checks   []DevPRCheck `json:"checks"`
	NextStep string       `json:"next_step"`
}

func BuildTicketChecksReport(repo RepoRef, issueNumber int, wait time.Duration, pollInterval time.Duration, runner CommandRunner) (TicketChecksReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if issueNumber <= 0 {
		return TicketChecksReport{}, fmt.Errorf("ticket must be > 0")
	}
	status, err := DevPRStatus(repo, issueNumber, runner)
	if err != nil {
		return TicketChecksReport{}, err
	}
	if wait > 0 {
		deadline := time.Now().Add(wait)
		for containsString(status.Blockers, "checks_pending") && time.Now().Before(deadline) {
			if pollInterval > 0 {
				time.Sleep(pollInterval)
			}
			status, err = DevPRStatus(repo, issueNumber, runner)
			if err != nil {
				return TicketChecksReport{}, err
			}
			if pollInterval <= 0 {
				break
			}
		}
	}
	return ticketChecksReportFromStatus(repo, issueNumber, status, wait), nil
}

func ticketChecksReportFromStatus(repo RepoRef, issueNumber int, status DevPRStatusResult, wait time.Duration) TicketChecksReport {
	blockers := status.Blockers
	if blockers == nil {
		blockers = []string{}
	}
	checks := status.Checks
	if checks == nil {
		checks = []DevPRCheck{}
	}
	report := TicketChecksReport{
		Repo:     repo.FullName(),
		Issue:    issueNumber,
		PRNumber: status.PRNumber,
		PRURL:    status.PRURL,
		State:    status.State,
		Ready:    status.Ready,
		Wait:     wait.String(),
		Blockers: blockers,
		Checks:   checks,
		NextStep: ticketChecksNextStep(repo, issueNumber, status),
	}
	if wait == 0 {
		report.Wait = ""
	}
	return report
}

func ticketChecksNextStep(repo RepoRef, issueNumber int, status DevPRStatusResult) string {
	if status.PRNumber == 0 {
		return fmt.Sprintf("gira ticket pr --repo %s --ticket %d --apply", repo.FullName(), issueNumber)
	}
	if containsString(status.Blockers, "draft") {
		return fmt.Sprintf("gira ticket finish --repo %s --ticket %d --apply", repo.FullName(), issueNumber)
	}
	if containsString(status.Blockers, "checks_pending") {
		return fmt.Sprintf("gira ticket wait --repo %s --ticket %d", repo.FullName(), issueNumber)
	}
	if containsString(status.Blockers, "checks") {
		return "fix failing checks, then " + fmt.Sprintf("gira ticket checks --repo %s --ticket %d", repo.FullName(), issueNumber)
	}
	if containsString(status.Blockers, "review") {
		return "resolve review requirements, then " + fmt.Sprintf("gira ticket checks --repo %s --ticket %d", repo.FullName(), issueNumber)
	}
	return fmt.Sprintf("gira ticket finish --repo %s --ticket %d --apply", repo.FullName(), issueNumber)
}

func FormatTicketChecks(report TicketChecksReport) string {
	blockers := strings.Join(report.Blockers, ",")
	if blockers == "" {
		blockers = "none"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "ticket checks: ticket #%d pr=%d ready=%t blockers=%s\n", report.Issue, report.PRNumber, report.Ready, blockers)
	if len(report.Checks) == 0 {
		b.WriteString("checks: none\n")
	} else {
		for _, check := range report.Checks {
			name := strings.TrimSpace(check.Name)
			if name == "" {
				name = "(unnamed)"
			}
			fmt.Fprintf(&b, "- %s %s", check.State, name)
			if strings.TrimSpace(check.Workflow) != "" {
				fmt.Fprintf(&b, " (%s)", check.Workflow)
			}
			b.WriteString("\n")
		}
	}
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}
