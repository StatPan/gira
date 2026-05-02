package gira

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type WeeklyReport struct {
	Repo       string                  `json:"repo"`
	Generated  string                  `json:"generated_at"`
	KPIs       WeeklyReportKPIs        `json:"kpis"`
	Exceptions []WeeklyReportException `json:"exceptions"`
}

type WeeklyReportKPIs struct {
	BacklogHealth                string  `json:"backlog_health"`
	OpenIssues                   int     `json:"open_issues"`
	StaleIssues                  int     `json:"stale_issues"`
	SLABreaches                  int     `json:"sla_breaches"`
	SprintCommitment             int     `json:"sprint_commitment"`
	SprintCompleted              int     `json:"sprint_completed"`
	BlockedIssues                int     `json:"blocked_issues"`
	BlockedAgingDaysP95          int     `json:"blocked_aging_days_p95"`
	ReviewPendingPRs             int     `json:"review_pending_prs"`
	ReviewLatencyHoursAverage    float64 `json:"review_latency_hours_avg"`
	ReleaseBlockers              int     `json:"release_blockers"`
	ClosureLinkMissingOpenIssues int     `json:"closure_link_missing_open_issues"`
	PRsMissingClosureLink        int     `json:"prs_missing_closure_link"`
}

type WeeklyReportException struct {
	Kind  string `json:"kind"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Owner string `json:"owner"`
	Age   int    `json:"age_days"`
}

func BuildWeeklyReport(repo RepoRef, now time.Time, dashboard DashboardExportClient, review ReviewGateClient) (WeeklyReport, error) {
	issues, err := dashboard.FetchIssues()
	if err != nil {
		return WeeklyReport{}, err
	}
	prs, err := review.ListOpenPRs()
	if err != nil {
		return WeeklyReport{}, err
	}
	release, err := BuildReleaseReadiness(review, now)
	if err != nil {
		return WeeklyReport{}, err
	}

	openIssues := make([]DashboardRawIssue, 0)
	staleIssues := 0
	blockedIssues := make([]WeeklyReportException, 0)
	for _, issue := range issues {
		if strings.ToLower(issue.State) != "open" {
			continue
		}
		openIssues = append(openIssues, issue)
		updated := parseTime(issue.UpdatedAt)
		ageDays := ageInDays(now, updated)
		if ageDays >= 14 {
			staleIssues++
		}
		if hasSubstring(issue.Labels, "blocked") {
			blockedIssues = append(blockedIssues, WeeklyReportException{Kind: "issue", Title: fmt.Sprintf("#%d %s", issue.IssueNumber, issue.Title), URL: issue.URL, Owner: "unassigned", Age: ageDays})
		}
	}

	reviewPending := make([]WeeklyReportException, 0)
	var reviewAgeHoursTotal float64
	for _, pr := range prs {
		if pr.IsDraft || pr.ReviewDecision == "APPROVED" {
			continue
		}
		updated := parseTime(pr.UpdatedAt)
		ageDays := ageInDays(now, updated)
		reviewAgeHoursTotal += now.Sub(updated).Hours()
		owner := "unassigned"
		if len(pr.RequestedReviewers) > 0 {
			owner = pr.RequestedReviewers[0]
		} else if len(pr.Assignees) > 0 {
			owner = pr.Assignees[0]
		}
		reviewPending = append(reviewPending, WeeklyReportException{Kind: "pr", Title: fmt.Sprintf("#%d %s", pr.Number, pr.Title), URL: pr.URL, Owner: owner, Age: ageDays})
	}

	blockedAges := make([]int, 0, len(blockedIssues))
	for _, ex := range blockedIssues {
		blockedAges = append(blockedAges, ex.Age)
	}
	sort.Ints(blockedAges)
	blockedP95 := 0
	if len(blockedAges) > 0 {
		idx := int(float64(len(blockedAges)-1) * 0.95)
		blockedP95 = blockedAges[idx]
	}

	avgReview := 0.0
	if len(reviewPending) > 0 {
		avgReview = reviewAgeHoursTotal / float64(len(reviewPending))
	}

	prsMissingClosure, closureLinkMissingIssues := closureLinkGapMetricsFromReviewPRs(prs)

	backlogHealth := "green"
	if staleIssues > 0 || len(blockedIssues) > 0 {
		backlogHealth = "amber"
	}
	if staleIssues > 5 || len(blockedIssues) > 3 {
		backlogHealth = "red"
	}

	kpis := WeeklyReportKPIs{
		BacklogHealth:                backlogHealth,
		OpenIssues:                   len(openIssues),
		StaleIssues:                  staleIssues,
		SLABreaches:                  staleIssues,
		SprintCommitment:             len(openIssues),
		SprintCompleted:              0,
		BlockedIssues:                len(blockedIssues),
		BlockedAgingDaysP95:          blockedP95,
		ReviewPendingPRs:             len(reviewPending),
		ReviewLatencyHoursAverage:    avgReview,
		ReleaseBlockers:              len(release.BlockingPRs) + len(release.OpenBlockers) + len(release.OpenMustFix),
		ClosureLinkMissingOpenIssues: closureLinkMissingIssues,
		PRsMissingClosureLink:        prsMissingClosure,
	}

	exceptions := append([]WeeklyReportException{}, blockedIssues...)
	exceptions = append(exceptions, reviewPending...)
	sort.Slice(exceptions, func(i, j int) bool {
		if exceptions[i].Age == exceptions[j].Age {
			return exceptions[i].Title < exceptions[j].Title
		}
		return exceptions[i].Age > exceptions[j].Age
	})
	if len(exceptions) > 10 {
		exceptions = exceptions[:10]
	}

	return WeeklyReport{Repo: repo.FullName(), Generated: now.UTC().Format(time.RFC3339), KPIs: kpis, Exceptions: exceptions}, nil
}

func FormatWeeklyReportMarkdown(report WeeklyReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Weekly PM Cockpit (%s)\n\n", report.Repo)
	fmt.Fprintf(&b, "- Generated: %s\n", report.Generated)
	fmt.Fprintf(&b, "- Backlog health: **%s**\n", report.KPIs.BacklogHealth)
	fmt.Fprintf(&b, "- SLA breaches: **%d**\n", report.KPIs.SLABreaches)
	fmt.Fprintf(&b, "- Sprint commitment vs completion: **%d / %d**\n", report.KPIs.SprintCommitment, report.KPIs.SprintCompleted)
	fmt.Fprintf(&b, "- Blocked aging p95 (days): **%d**\n", report.KPIs.BlockedAgingDaysP95)
	fmt.Fprintf(&b, "- Review latency avg (hours): **%.1f**\n", report.KPIs.ReviewLatencyHoursAverage)
	fmt.Fprintf(&b, "- Release blockers: **%d**\n\n", report.KPIs.ReleaseBlockers)
	fmt.Fprintf(&b, "## Top exceptions\n")
	if len(report.Exceptions) == 0 {
		b.WriteString("- none\n")
		return b.String()
	}
	for _, ex := range report.Exceptions {
		fmt.Fprintf(&b, "- [%s] [%s](%s) owner:%s age:%dd\n", ex.Kind, ex.Title, ex.URL, ex.Owner, ex.Age)
	}
	return b.String()
}

func parseTime(v string) time.Time {
	t, _ := time.Parse(time.RFC3339, v)
	return t
}

func ageInDays(now, t time.Time) int {
	if t.IsZero() {
		return 0
	}
	return int(now.Sub(t).Hours() / 24)
}

func hasSubstring(values []string, needle string) bool {
	needle = strings.ToLower(strings.TrimSpace(needle))
	for _, v := range values {
		if strings.Contains(strings.ToLower(v), needle) {
			return true
		}
	}
	return false
}

func closureLinkGapMetricsFromReviewPRs(prs []ReviewPR) (int, int) {
	missingPRs := 0
	for _, pr := range prs {
		if pr.IsDraft {
			continue
		}
		if len(ExtractClosureIssueNumbers(pr.Body)) == 0 {
			missingPRs++
		}
	}
	return missingPRs, missingPRs
}

func (r WeeklyReport) MarshalJSON() ([]byte, error) {
	type alias WeeklyReport
	copy := alias(r)
	copy.KPIs.ReviewLatencyHoursAverage = float64(int(copy.KPIs.ReviewLatencyHoursAverage*10+0.5)) / 10
	return json.Marshal(copy)
}
