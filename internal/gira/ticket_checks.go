package gira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	ticketWaitHeartbeat     = 30 * time.Second
	ticketChecksGlanceLimit = 3
)

type TicketChecksOptions struct {
	Wait         time.Duration
	PollInterval time.Duration
	Detail       bool
	OnProgress   func(TicketChecksReport)
	// Now and Sleep are optional. Nil uses the wall clock. Tests set them so
	// elapsed time and the wait heartbeat do not depend on real delays.
	Now   func() time.Time
	Sleep func(time.Duration)
}

type TicketChecksReport struct {
	Repo     string              `json:"repo"`
	Issue    int                 `json:"issue"`
	PRNumber int                 `json:"pr_number,omitempty"`
	PRURL    string              `json:"pr_url,omitempty"`
	State    string              `json:"state,omitempty"`
	Ready    bool                `json:"ready"`
	Wait     string              `json:"wait,omitempty"`
	Blockers []string            `json:"blockers"`
	Checks   []DevPRCheck        `json:"checks"`
	NextStep string              `json:"next_step"`
	Detail   *TicketChecksDetail `json:"detail,omitempty"`
	HeadSHA  string              `json:"-"`

	observedAt time.Time
}

type TicketChecksDetail struct {
	Checks []TicketCheckDetail `json:"checks"`
}

type TicketCheckDetail struct {
	CheckIndex    int              `json:"check_index"`
	Availability  string           `json:"availability"`
	Reason        string           `json:"reason,omitempty"`
	RunID         int64            `json:"run_id,omitempty"`
	RunNumber     int64            `json:"run_number,omitempty"`
	RunAttempt    int              `json:"run_attempt,omitempty"`
	WorkflowName  string           `json:"workflow_name,omitempty"`
	RunURL        string           `json:"run_url,omitempty"`
	JobID         int64            `json:"job_id,omitempty"`
	JobURL        string           `json:"job_url,omitempty"`
	JobStatus     string           `json:"job_status,omitempty"`
	JobConclusion string           `json:"job_conclusion,omitempty"`
	StartedAt     string           `json:"started_at,omitempty"`
	CompletedAt   string           `json:"completed_at,omitempty"`
	Step          *TicketCheckStep `json:"step,omitempty"`
	StepCount     int              `json:"step_count,omitempty"`
}

type TicketCheckStep struct {
	Number      int    `json:"number"`
	Name        string `json:"name,omitempty"`
	Status      string `json:"status,omitempty"`
	Conclusion  string `json:"conclusion,omitempty"`
	StartedAt   string `json:"started_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
}

type TicketWaitProgress struct {
	Timeout    time.Duration
	started    time.Time
	previous   *TicketChecksReport
	lastNotice time.Time
}

func NewTicketWaitProgress(started time.Time, timeout time.Duration) *TicketWaitProgress {
	return &TicketWaitProgress{Timeout: timeout, started: started}
}

func (p *TicketWaitProgress) Observe(report TicketChecksReport, now time.Time) string {
	if p == nil {
		return ""
	}
	changed := ticketWaitChangedIndexes(p.previous, report)
	semantic := p.previous == nil || len(changed) > 0 || ticketWaitReportChanged(p.previous, report)
	if semantic {
		text := formatTicketWaitChange(report, changed, now, p.started, p.Timeout)
		p.store(report)
		p.lastNotice = now
		return text
	}
	if !p.lastNotice.IsZero() && now.Sub(p.lastNotice) >= ticketWaitHeartbeat {
		text := formatTicketWaitHeartbeat(now, p.started, p.Timeout)
		p.store(report)
		p.lastNotice = now
		return text
	}
	p.store(report)
	return ""
}

func (p *TicketWaitProgress) store(report TicketChecksReport) {
	cloned := report
	p.previous = &cloned
}

func BuildTicketChecksReport(repo RepoRef, issueNumber int, options TicketChecksOptions, runner CommandRunner) (TicketChecksReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if issueNumber <= 0 {
		return TicketChecksReport{}, fmt.Errorf("ticket must be > 0")
	}
	nowFn, sleepFn := ticketChecksClock(options)
	status, err := DevPRStatus(repo, issueNumber, runner)
	if err != nil {
		return TicketChecksReport{}, err
	}
	report := ticketChecksSnapshot(repo, issueNumber, status, options, runner, nowFn())
	if options.OnProgress != nil {
		options.OnProgress(report)
	}
	if options.Wait > 0 {
		deadline := nowFn().Add(options.Wait)
		for containsString(status.Blockers, "checks_pending") && nowFn().Before(deadline) {
			if options.PollInterval > 0 {
				sleepFn(options.PollInterval)
			}
			status, err = DevPRStatus(repo, issueNumber, runner)
			if err != nil {
				return TicketChecksReport{}, err
			}
			report = ticketChecksSnapshot(repo, issueNumber, status, options, runner, nowFn())
			if options.OnProgress != nil {
				options.OnProgress(report)
			}
			if options.PollInterval <= 0 {
				break
			}
		}
	}
	return report, nil
}

func ticketChecksClock(options TicketChecksOptions) (func() time.Time, func(time.Duration)) {
	nowFn := options.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	sleepFn := options.Sleep
	if sleepFn == nil {
		sleepFn = time.Sleep
	}
	return nowFn, sleepFn
}

func ticketChecksSnapshot(repo RepoRef, issueNumber int, status DevPRStatusResult, options TicketChecksOptions, runner CommandRunner, now time.Time) TicketChecksReport {
	report := ticketChecksReportFromStatus(repo, issueNumber, status, options.Wait)
	report.HeadSHA = strings.TrimSpace(status.HeadSHA)
	report.observedAt = now
	if options.Detail {
		report.Detail = &TicketChecksDetail{Checks: attachTicketChecksDetail(repo, status, runner)}
	}
	return report
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
		HeadSHA:  strings.TrimSpace(status.HeadSHA),
	}
	if wait == 0 {
		report.Wait = ""
	}
	return report
}

func ticketChecksNextStep(repo RepoRef, issueNumber int, status DevPRStatusResult) string {
	if status.PRNumber == 0 {
		return fmt.Sprintf("gira ticket pr --repo %s --ticket %d --dry-run", repo.FullName(), issueNumber)
	}
	if containsString(status.Blockers, "draft") {
		return fmt.Sprintf("gira ticket finish --repo %s --ticket %d --dry-run", repo.FullName(), issueNumber)
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
	return fmt.Sprintf("gira ticket finish --repo %s --ticket %d --dry-run", repo.FullName(), issueNumber)
}

func FormatTicketChecks(report TicketChecksReport) string {
	return formatTicketChecks(report, ticketChecksObservedAt(report))
}

func formatTicketChecks(report TicketChecksReport, now time.Time) string {
	blockers := strings.Join(report.Blockers, ",")
	if blockers == "" {
		blockers = "none"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "ticket checks: ticket #%d pr=%d ready=%t blockers=%s\n", report.Issue, report.PRNumber, report.Ready, blockers)
	if len(report.Checks) == 0 {
		b.WriteString("checks: none\n")
	} else if report.Detail == nil {
		for _, check := range report.Checks {
			fmt.Fprintf(&b, "- %s %s", check.State, ticketCheckDisplayName(check))
			if strings.TrimSpace(check.Workflow) != "" {
				fmt.Fprintf(&b, " (%s)", strings.TrimSpace(check.Workflow))
			}
			b.WriteString("\n")
		}
	} else {
		shown := ticketCheckGlanceIndexes(report)
		shownSet := map[int]struct{}{}
		for _, index := range shown {
			shownSet[index] = struct{}{}
			writeTicketCheckDetail(&b, report, index, now)
		}
		if summary := ticketCheckGlanceRemainder(report, shownSet); summary != "" {
			b.WriteString(summary)
			b.WriteString("\n")
		}
	}
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}

func ticketChecksObservedAt(report TicketChecksReport) time.Time {
	if !report.observedAt.IsZero() {
		return report.observedAt
	}
	return time.Now()
}

func writeTicketCheckDetail(b *strings.Builder, report TicketChecksReport, index int, now time.Time) {
	check := report.Checks[index]
	detail := ticketCheckDetailAt(report, index)
	fmt.Fprintf(b, "- %s %s", check.State, ticketCheckDisplayName(check))
	workflow := strings.TrimSpace(detail.WorkflowName)
	if workflow == "" && detail.Availability != "available" {
		workflow = strings.TrimSpace(check.Workflow)
	}
	if workflow != "" {
		fmt.Fprintf(b, " (%s)", workflow)
	}
	if detail.RunNumber > 0 {
		fmt.Fprintf(b, " run #%d", detail.RunNumber)
	}
	if detail.RunAttempt > 0 {
		fmt.Fprintf(b, " attempt %d", detail.RunAttempt)
	}
	fmt.Fprintf(b, " %s", ticketCheckProgressPhrase(check, detail, now))
	if ticketCheckGlanceShowsLink(check, detail) {
		if link := ticketCheckDetailLink(check, detail); link != "" {
			b.WriteByte(' ')
			b.WriteString(link)
		}
	}
	b.WriteString("\n")
}

func ticketCheckGlanceShowsLink(check DevPRCheck, detail TicketCheckDetail) bool {
	return check.State != "passing" || check.Superseded || detail.Availability != "available"
}

func ticketCheckGlanceIndexes(report TicketChecksReport) []int {
	indexes := make([]int, len(report.Checks))
	for index := range report.Checks {
		indexes[index] = index
	}
	sort.SliceStable(indexes, func(i, j int) bool {
		left, right := indexes[i], indexes[j]
		leftRank, leftAt := ticketCheckGlanceRank(report, left)
		rightRank, rightAt := ticketCheckGlanceRank(report, right)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if !leftAt.Equal(rightAt) {
			return leftAt.After(rightAt)
		}
		return left < right
	})
	if len(indexes) > ticketChecksGlanceLimit {
		return indexes[:ticketChecksGlanceLimit]
	}
	return indexes
}

func ticketCheckGlanceRank(report TicketChecksReport, index int) (int, time.Time) {
	check := report.Checks[index]
	detail := ticketCheckDetailAt(report, index)
	rank := 1
	if check.State != "passing" || detail.Availability != "available" {
		rank = 0
	}
	return rank, ticketCheckActivityTime(check, detail)
}

func ticketCheckActivityTime(check DevPRCheck, detail TicketCheckDetail) time.Time {
	candidates := []string{detail.CompletedAt, detail.StartedAt, check.CompletedAt}
	if detail.Step != nil {
		candidates = append([]string{detail.Step.CompletedAt, detail.Step.StartedAt}, candidates...)
	}
	for _, raw := range candidates {
		if parsed, ok := parseTicketCheckTime(raw); ok {
			return parsed
		}
	}
	return time.Time{}
}

func ticketCheckGlanceRemainder(report TicketChecksReport, shown map[int]struct{}) string {
	counts := map[string]int{}
	order := []string{}
	for index, check := range report.Checks {
		if _, ok := shown[index]; ok {
			continue
		}
		state := check.State
		if state == "" {
			state = "unknown"
		}
		if counts[state] == 0 {
			order = append(order, state)
		}
		counts[state]++
	}
	if len(order) == 0 {
		return ""
	}
	parts := make([]string, len(order))
	for index, state := range order {
		parts[index] = fmt.Sprintf("%d %s", counts[state], state)
	}
	return "other checks: " + strings.Join(parts, ", ")
}

func ticketCheckDisplayName(check DevPRCheck) string {
	name := strings.TrimSpace(check.Name)
	if name == "" {
		return "(unnamed)"
	}
	return name
}

func ticketCheckDetailAt(report TicketChecksReport, index int) TicketCheckDetail {
	if report.Detail == nil || index < 0 || index >= len(report.Detail.Checks) {
		return TicketCheckDetail{CheckIndex: index, Availability: "unavailable", Reason: "detail missing"}
	}
	return report.Detail.Checks[index]
}

func ticketCheckProgressPhrase(check DevPRCheck, detail TicketCheckDetail, now time.Time) string {
	switch detail.Availability {
	case "unsupported":
		return "step detail unsupported"
	case "available":
		return ticketCheckAvailablePhrase(check, detail, now)
	default:
		reason := strings.TrimSpace(detail.Reason)
		if reason == "" {
			reason = "unknown"
		}
		return "detail unavailable: " + reason
	}
}

func ticketCheckAvailablePhrase(check DevPRCheck, detail TicketCheckDetail, now time.Time) string {
	var parts []string
	if check.Superseded {
		parts = append(parts, "cancelled (superseded)")
	} else {
		parts = append(parts, ticketCheckStatusPhrase(detail))
	}
	if !check.Superseded && detail.Step != nil {
		label := "step"
		if isFailedStepConclusion(detail.Step.Conclusion) && !isRunningStatus(detail.Step.Status) {
			label = "failed step"
		}
		step := fmt.Sprintf("%s %d", label, detail.Step.Number)
		if detail.StepCount > 0 {
			step = fmt.Sprintf("%s %d/%d", label, detail.Step.Number, detail.StepCount)
		}
		if name := strings.TrimSpace(detail.Step.Name); name != "" {
			step += ": " + name
		}
		parts = append(parts, step)
	}
	if elapsed, ok := ticketCheckElapsed(detail, now); ok {
		parts = append(parts, elapsed.String())
	}
	return strings.Join(parts, " ")
}

func ticketCheckStatusPhrase(detail TicketCheckDetail) string {
	status := strings.ToLower(strings.TrimSpace(detail.JobStatus))
	conclusion := strings.ToLower(strings.TrimSpace(detail.JobConclusion))
	if isRunningStatus(status) || status == "queued" || status == "pending" || status == "waiting" || status == "requested" {
		if status == "" {
			return "unknown"
		}
		return status
	}
	if conclusion != "" {
		return conclusion
	}
	if status == "" {
		return "unknown"
	}
	return status
}

func ticketCheckDetailLink(check DevPRCheck, detail TicketCheckDetail) string {
	if link := strings.TrimSpace(detail.JobURL); link != "" {
		return link
	}
	if link := strings.TrimSpace(check.URL); link != "" {
		return link
	}
	return strings.TrimSpace(detail.RunURL)
}

func ticketCheckElapsed(detail TicketCheckDetail, now time.Time) (time.Duration, bool) {
	startRaw := detail.StartedAt
	endRaw := detail.CompletedAt
	running := isRunningStatus(detail.JobStatus) || detail.JobStatus == "" && detail.Step != nil && isRunningStatus(detail.Step.Status)
	if detail.Step != nil {
		startRaw = detail.Step.StartedAt
		endRaw = detail.Step.CompletedAt
		if isRunningStatus(detail.Step.Status) {
			running = true
		} else if strings.TrimSpace(detail.Step.CompletedAt) != "" {
			running = false
		}
	}
	start, ok := parseTicketCheckTime(startRaw)
	if !ok {
		return 0, false
	}
	if running {
		if now.Before(start) {
			return 0, false
		}
		return now.Sub(start).Truncate(time.Second), true
	}
	end, ok := parseTicketCheckTime(endRaw)
	if !ok || end.Before(start) {
		return 0, false
	}
	return end.Sub(start).Truncate(time.Second), true
}

func parseTicketCheckTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil || parsed.IsZero() || parsed.Year() < 2 {
		return time.Time{}, false
	}
	return parsed, true
}

func isRunningStatus(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "in_progress")
}

func isFailedStepConclusion(conclusion string) bool {
	switch strings.ToLower(strings.TrimSpace(conclusion)) {
	case "failure", "timed_out", "action_required":
		return true
	default:
		return false
	}
}

func formatTicketWaitChange(report TicketChecksReport, changed []int, now, started time.Time, timeout time.Duration) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ticket wait: elapsed %s remaining %s\n", ticketWaitElapsed(now, started), ticketWaitRemaining(now, started, timeout))
	if report.Detail != nil {
		for _, index := range changed {
			if index < 0 || index >= len(report.Checks) {
				continue
			}
			writeTicketCheckDetail(&b, report, index, now)
		}
	}
	return b.String()
}

func formatTicketWaitHeartbeat(now, started time.Time, timeout time.Duration) string {
	return fmt.Sprintf("ticket wait: still waiting elapsed %s remaining %s\n", ticketWaitElapsed(now, started), ticketWaitRemaining(now, started, timeout))
}

func ticketWaitElapsed(now, started time.Time) string {
	if now.Before(started) {
		return "0s"
	}
	return now.Sub(started).Truncate(time.Second).String()
}

func ticketWaitRemaining(now, started time.Time, timeout time.Duration) string {
	if timeout <= 0 {
		return "0s"
	}
	remaining := timeout - now.Sub(started)
	if remaining < 0 {
		remaining = 0
	}
	return remaining.Truncate(time.Second).String()
}

func ticketWaitChangedIndexes(previous *TicketChecksReport, current TicketChecksReport) []int {
	if previous == nil {
		indexes := make([]int, len(current.Checks))
		for index := range current.Checks {
			indexes[index] = index
		}
		return indexes
	}
	changed := []int{}
	limit := len(current.Checks)
	if len(previous.Checks) < limit {
		limit = len(previous.Checks)
	}
	for index := range limit {
		if ticketCheckSemanticKey(*previous, index) != ticketCheckSemanticKey(current, index) {
			changed = append(changed, index)
		}
	}
	for index := len(previous.Checks); index < len(current.Checks); index++ {
		changed = append(changed, index)
	}
	return changed
}

func ticketWaitReportChanged(previous *TicketChecksReport, current TicketChecksReport) bool {
	if previous == nil {
		return true
	}
	if previous.PRNumber != current.PRNumber || previous.HeadSHA != current.HeadSHA || len(previous.Checks) != len(current.Checks) {
		return true
	}
	return strings.Join(previous.Blockers, "\x00") != strings.Join(current.Blockers, "\x00")
}

func ticketCheckSemanticKey(report TicketChecksReport, index int) string {
	check := report.Checks[index]
	detail := ticketCheckDetailAt(report, index)
	stepNumber := 0
	stepName := ""
	stepStatus := ""
	stepConclusion := ""
	if detail.Step != nil {
		stepNumber = detail.Step.Number
		stepName = detail.Step.Name
		stepStatus = detail.Step.Status
		stepConclusion = detail.Step.Conclusion
	}
	return strings.Join([]string{
		strconv.Itoa(index),
		check.Name,
		check.State,
		check.Conclusion,
		check.URL,
		strconv.FormatBool(check.Superseded),
		detail.Availability,
		detail.Reason,
		strconv.FormatInt(detail.RunID, 10),
		strconv.FormatInt(detail.RunNumber, 10),
		strconv.Itoa(detail.RunAttempt),
		strconv.FormatInt(detail.JobID, 10),
		detail.JobStatus,
		detail.JobConclusion,
		detail.WorkflowName,
		strconv.Itoa(stepNumber),
		stepName,
		stepStatus,
		stepConclusion,
	}, "\x00")
}

type actionsRunMeta struct {
	Name      string `json:"name"`
	RunNumber int64  `json:"run_number"`
	HeadSHA   string `json:"head_sha"`
	HTMLURL   string `json:"html_url"`
}

type actionsJobsPage struct {
	Jobs []actionsJob `json:"jobs"`
}

type actionsJob struct {
	ID          int64          `json:"id"`
	RunID       int64          `json:"run_id"`
	RunAttempt  int            `json:"run_attempt"`
	HTMLURL     string         `json:"html_url"`
	Status      string         `json:"status"`
	Conclusion  nullableString `json:"conclusion"`
	StartedAt   nullableString `json:"started_at"`
	CompletedAt nullableString `json:"completed_at"`
	Steps       []actionsStep  `json:"steps"`
}

type actionsStep struct {
	Name        string         `json:"name"`
	Status      string         `json:"status"`
	Conclusion  nullableString `json:"conclusion"`
	Number      int            `json:"number"`
	StartedAt   nullableString `json:"started_at"`
	CompletedAt nullableString `json:"completed_at"`
}

type nullableString string

func (n *nullableString) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*n = ""
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*n = nullableString(value)
	return nil
}

type actionsRunEvidence struct {
	meta    actionsRunMeta
	metaOK  bool
	jobs    []actionsJob
	jobsErr string
}

func attachTicketChecksDetail(repo RepoRef, status DevPRStatusResult, runner CommandRunner) []TicketCheckDetail {
	details := make([]TicketCheckDetail, len(status.Checks))
	cache := map[int64]*actionsRunEvidence{}
	for index, check := range status.Checks {
		details[index] = TicketCheckDetail{CheckIndex: index}
		if !isGitHubActionsCheck(check) {
			details[index].Availability = "unsupported"
			continue
		}
		runID, jobID, ok, reason := parseActionsJobURL(check.URL, repo)
		if !ok {
			details[index].Availability = "unavailable"
			details[index].Reason = reason
			continue
		}
		details[index].RunID = runID
		details[index].JobID = jobID
		evidence := cachedActionsRunEvidence(cache, repo, runID, runner)
		if !evidence.metaOK {
			details[index].Availability = "unavailable"
			details[index].Reason = "run metadata unavailable"
			continue
		}
		details[index].RunNumber = evidence.meta.RunNumber
		details[index].WorkflowName = strings.TrimSpace(evidence.meta.Name)
		details[index].RunURL = strings.TrimSpace(evidence.meta.HTMLURL)
		if !actionsRunHeadMatches(evidence.meta.HeadSHA, status.HeadSHA) {
			details[index].Availability = "unavailable"
			details[index].Reason = actionsRunHeadMismatchReason(status.HeadSHA)
			continue
		}
		if evidence.jobsErr != "" {
			details[index].Availability = "unavailable"
			details[index].Reason = evidence.jobsErr
			continue
		}
		job, found := findActionsJob(evidence.jobs, runID, jobID)
		if !found {
			details[index].Availability = "unavailable"
			if len(evidence.jobs) == 0 {
				details[index].Reason = "jobs response empty"
			} else {
				details[index].Reason = "job not found"
			}
			continue
		}
		fillAvailableJobDetail(&details[index], job)
	}
	return details
}

func actionsRunHeadMatches(runHead, prHead string) bool {
	runHead = strings.TrimSpace(runHead)
	prHead = strings.TrimSpace(prHead)
	return runHead != "" && prHead != "" && strings.EqualFold(runHead, prHead)
}

func actionsRunHeadMismatchReason(prHead string) string {
	if strings.TrimSpace(prHead) == "" {
		return "pull request head unavailable"
	}
	return "run head does not match pull request head"
}

func cachedActionsRunEvidence(cache map[int64]*actionsRunEvidence, repo RepoRef, runID int64, runner CommandRunner) *actionsRunEvidence {
	if evidence, ok := cache[runID]; ok {
		return evidence
	}
	evidence := &actionsRunEvidence{}
	cache[runID] = evidence
	out, err := runner.Run("gh", "api", fmt.Sprintf("repos/%s/actions/runs/%d", repo.FullName(), runID))
	if err != nil {
		return evidence
	}
	if err := json.Unmarshal(out, &evidence.meta); err != nil || strings.TrimSpace(evidence.meta.HeadSHA) == "" {
		return evidence
	}
	evidence.metaOK = true
	jobsOut, err := runner.Run("gh", "api", fmt.Sprintf("repos/%s/actions/runs/%d/jobs", repo.FullName(), runID), "-X", "GET", "-f", "filter=all", "-f", "per_page=100", "--paginate", "--slurp")
	if err != nil {
		evidence.jobsErr = "jobs unavailable"
		return evidence
	}
	jobs, err := decodeActionsJobs(jobsOut)
	if err != nil {
		evidence.jobsErr = "jobs response malformed"
		return evidence
	}
	evidence.jobs = jobs
	return evidence
}

func decodeActionsJobs(out []byte) ([]actionsJob, error) {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, fmt.Errorf("jobs response malformed")
	}
	if trimmed[0] == '[' {
		if bytes.Contains(trimmed, []byte(`"jobs"`)) {
			var pages []actionsJobsPage
			if err := json.Unmarshal(trimmed, &pages); err != nil {
				return nil, fmt.Errorf("jobs response malformed")
			}
			jobs := make([]actionsJob, 0)
			for _, page := range pages {
				jobs = append(jobs, page.Jobs...)
			}
			return jobs, nil
		}
		var bare []actionsJob
		if err := json.Unmarshal(trimmed, &bare); err != nil {
			return nil, fmt.Errorf("jobs response malformed")
		}
		return bare, nil
	}
	if !bytes.Contains(trimmed, []byte(`"jobs"`)) {
		return nil, fmt.Errorf("jobs response malformed")
	}
	var page actionsJobsPage
	if err := json.Unmarshal(trimmed, &page); err != nil {
		return nil, fmt.Errorf("jobs response malformed")
	}
	return page.Jobs, nil
}

func findActionsJob(jobs []actionsJob, runID, jobID int64) (actionsJob, bool) {
	for _, job := range jobs {
		if job.ID == jobID && job.RunID == runID {
			return job, true
		}
	}
	return actionsJob{}, false
}

func fillAvailableJobDetail(detail *TicketCheckDetail, job actionsJob) {
	detail.Availability = "available"
	detail.RunAttempt = job.RunAttempt
	if job.RunID > 0 {
		detail.RunID = job.RunID
	}
	if job.ID > 0 {
		detail.JobID = job.ID
	}
	detail.JobURL = strings.TrimSpace(job.HTMLURL)
	detail.JobStatus = strings.TrimSpace(job.Status)
	detail.JobConclusion = strings.TrimSpace(string(job.Conclusion))
	detail.StartedAt = strings.TrimSpace(string(job.StartedAt))
	detail.CompletedAt = strings.TrimSpace(string(job.CompletedAt))
	detail.StepCount = len(job.Steps)
	detail.Step = selectTicketCheckStep(job.Steps)
}

func selectTicketCheckStep(steps []actionsStep) *TicketCheckStep {
	var inProgress *actionsStep
	var failed *actionsStep
	for index := range steps {
		step := &steps[index]
		if isRunningStatus(step.Status) && (inProgress == nil || step.Number < inProgress.Number) {
			inProgress = step
		}
		if isFailedStepConclusion(string(step.Conclusion)) && (failed == nil || step.Number < failed.Number) {
			failed = step
		}
	}
	chosen := inProgress
	if chosen == nil {
		chosen = failed
	}
	if chosen == nil {
		return nil
	}
	return &TicketCheckStep{
		Number:      chosen.Number,
		Name:        strings.TrimSpace(chosen.Name),
		Status:      strings.TrimSpace(chosen.Status),
		Conclusion:  strings.TrimSpace(string(chosen.Conclusion)),
		StartedAt:   strings.TrimSpace(string(chosen.StartedAt)),
		CompletedAt: strings.TrimSpace(string(chosen.CompletedAt)),
	}
}

func parseActionsJobURL(raw string, repo RepoRef) (int64, int64, bool, string) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.Path == "" {
		return 0, 0, false, "job url missing"
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "github.com" && host != "www.github.com" {
		return 0, 0, false, "job url does not match repository"
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 7 || !strings.EqualFold(parts[0], repo.Owner) || !strings.EqualFold(parts[1], repo.Name) || parts[2] != "actions" || parts[3] != "runs" || parts[5] != "job" {
		return 0, 0, false, "job url does not match repository"
	}
	runID, runErr := strconv.ParseInt(parts[4], 10, 64)
	jobID, jobErr := strconv.ParseInt(parts[6], 10, 64)
	if runErr != nil || jobErr != nil || runID <= 0 || jobID <= 0 {
		return 0, 0, false, "job url missing"
	}
	return runID, jobID, true, ""
}
