package gira

import (
	"fmt"
	"os"
	"strings"
)

const TicketNewReportSchemaVersion = "ticket-new-report/v1"

type TicketNewInput struct {
	Repo                RepoRef  `json:"repo"`
	Title               string   `json:"title"`
	Parent              int      `json:"parent,omitempty"`
	Goal                string   `json:"goal,omitempty"`
	Scope               string   `json:"scope,omitempty"`
	Acceptance          []string `json:"acceptance,omitempty"`
	Notes               string   `json:"notes,omitempty"`
	Body                string   `json:"body,omitempty"`
	Type                string   `json:"type"`
	Priority            string   `json:"priority,omitempty"`
	Milestone           string   `json:"milestone,omitempty"`
	Labels              []string `json:"labels,omitempty"`
	ReleaseImpact       string   `json:"release_impact,omitempty"`
	ReleaseImpactReason string   `json:"release_impact_reason,omitempty"`
	BodyFile            string   `json:"body_file,omitempty"`
	Start               bool     `json:"start"`
	DryRun              bool     `json:"dry_run"`
}

type TicketCreatedIssue struct {
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	URL    string `json:"url"`
}

type TicketLabelOutcome struct {
	Status             string   `json:"status"`
	RequestedLabels    []string `json:"requested_labels"`
	AppliedLabels      []string `json:"applied_labels"`
	MissingLabels      []string `json:"missing_labels,omitempty"`
	RequiredCapability string   `json:"required_capability,omitempty"`
	Remediation        string   `json:"remediation,omitempty"`
}

type TicketNewReport struct {
	SchemaVersion   string                `json:"schema_version,omitempty"`
	Repo            string                `json:"repo"`
	Title           string                `json:"title"`
	DryRun          bool                  `json:"dry_run"`
	Start           bool                  `json:"start"`
	Type            string                `json:"type,omitempty"`
	Priority        string                `json:"priority,omitempty"`
	Parent          int                   `json:"parent,omitempty"`
	Labels          []string              `json:"labels"`
	RequestedLabels []string              `json:"requested_labels"`
	AppliedLabels   []string              `json:"applied_labels,omitempty"`
	LabelOutcome    TicketLabelOutcome    `json:"label_outcome"`
	Milestone       string                `json:"milestone,omitempty"`
	ReleaseImpact   TicketReleaseImpact   `json:"release_impact"`
	Body            string                `json:"body"`
	TicketReadiness TicketReadinessReport `json:"ticket_readiness"`
	Created         TicketCreatedIssue    `json:"created,omitempty"`
	StartResult     WorkStartResult       `json:"start_result,omitempty"`
	NextStep        string                `json:"next_step"`
	Approval        *ApprovalEvidence     `json:"approval,omitempty"`
}

func EnsureTicketNewReportSchema(report *TicketNewReport) {
	if report != nil && strings.TrimSpace(report.SchemaVersion) == "" {
		report.SchemaVersion = TicketNewReportSchemaVersion
	}
}

func BuildTicketNewReport(input TicketNewInput, runner CommandRunner) (TicketNewReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return TicketNewReport{}, fmt.Errorf("ticket title is required")
	}
	ticketType := strings.TrimSpace(input.Type)
	if ticketType == "" {
		ticketType = "task"
	}
	if !validTicketType(ticketType) {
		return TicketNewReport{}, ticketNewTypeError(ticketType)
	}
	priority := strings.TrimSpace(input.Priority)
	if priority != "" && !validTicketPriority(priority) {
		return TicketNewReport{}, fmt.Errorf("--priority must be one of p0, p1, p2, p3")
	}
	if input.Parent < 0 {
		return TicketNewReport{}, fmt.Errorf("--parent must be > 0")
	}
	body, releaseImpact, err := ticketNewBody(input, ticketType)
	if err != nil {
		return TicketNewReport{}, err
	}
	labels := ticketNewLabels(ticketType, priority, input.Labels)
	report := TicketNewReport{
		SchemaVersion:   TicketNewReportSchemaVersion,
		Repo:            input.Repo.FullName(),
		Title:           input.Title,
		DryRun:          input.DryRun,
		Start:           input.Start,
		Type:            ticketType,
		Priority:        priority,
		Parent:          input.Parent,
		Labels:          labels,
		RequestedLabels: append([]string(nil), labels...),
		LabelOutcome: TicketLabelOutcome{
			Status:          "planned",
			RequestedLabels: append([]string(nil), labels...),
			AppliedLabels:   []string{},
		},
		Milestone:       strings.TrimSpace(input.Milestone),
		ReleaseImpact:   releaseImpact,
		Body:            body,
		TicketReadiness: EvaluateTicketReadiness(body, labels, "open"),
		NextStep:        "gira ticket new --apply",
	}
	if err := preflightTicketNewLabels(input.Repo, labels, runner); err != nil {
		return report, err
	}
	if input.Parent > 0 {
		if _, err := fetchGitHubIssue(input.Repo, input.Parent, runner); err != nil {
			return report, fmt.Errorf("preflight parent issue: %w", err)
		}
	}
	policy := ResolvedBranchPolicy{StartMode: BranchStartModeLegacyCreate}
	if input.Start || !input.DryRun {
		resolved, err := resolveRepoBranchPolicy(input.Repo, runner)
		if err != nil {
			return report, err
		}
		policy = resolved
	}
	if input.Start {
		if policy.StartMode == BranchStartModeExplicit {
			report.NextStep = "create the ticket, then choose `gira ticket start <ticket> --create|--current|--adopt BRANCH --apply`"
			if !input.DryRun {
				return report, fmt.Errorf("--start is not available when branch_policy.start_mode is explicit; create the ticket first, then choose a ticket start branch strategy")
			}
			return report, nil
		}
	}
	if input.DryRun {
		report.Approval = TicketNewApprovalEvidence(report)
		return report, nil
	}
	created, err := createRepoTicket(input.Repo, input.Title, body, labels, report.Milestone, runner)
	if err != nil {
		return report, err
	}
	report.Created = created
	report.NextStep = ticketNewStartNextStep(created.Number, policy.StartMode)
	createdIssue, err := fetchDevIssue(input.Repo, created.Number, runner)
	if err != nil {
		return report, fmt.Errorf("verify created issue labels: %w", err)
	}
	report.AppliedLabels = append([]string(nil), createdIssue.Labels...)
	// The legacy labels field is the effective label state after an apply. Keep
	// RequestedLabels as the immutable request so JSON consumers cannot mistake
	// an unverified request for labels that GitHub actually applied.
	report.Labels = append([]string(nil), report.AppliedLabels...)
	report.LabelOutcome = ticketNewLabelOutcome(input.Repo, created.Number, report.RequestedLabels, report.AppliedLabels)
	report.TicketReadiness = EvaluateTicketReadiness(body, report.AppliedLabels, createdIssue.State)
	if report.LabelOutcome.Status == "warning" {
		report.NextStep = report.LabelOutcome.Remediation
	}
	if input.Parent > 0 {
		child, err := fetchGitHubIssue(input.Repo, created.Number, runner)
		if err != nil {
			return report, fmt.Errorf("fetch created issue for parent link: %w", err)
		}
		if err := addGitHubSubIssue(input.Repo, input.Parent, child.ID, false, runner); err != nil {
			return report, fmt.Errorf("link parent issue: %w", err)
		}
	}
	if input.Start && report.LabelOutcome.Status != "warning" {
		start, err := StartWork(input.Repo, created.Number, false, runner)
		report.StartResult = start
		if err != nil {
			return report, err
		}
		report.NextStep = "gira ticket pr --dry-run"
	}
	return report, nil
}

func ticketNewStartNextStep(issue int, startMode string) string {
	if startMode == BranchStartModeExplicit {
		return fmt.Sprintf("gira ticket start %d --create --apply", issue)
	}
	return fmt.Sprintf("gira ticket start %d --apply", issue)
}

func ticketNewLabelOutcome(repo RepoRef, issueNumber int, requested []string, applied []string) TicketLabelOutcome {
	outcome := TicketLabelOutcome{
		Status:          "applied",
		RequestedLabels: append([]string(nil), requested...),
		AppliedLabels:   append([]string(nil), applied...),
	}
	missing := []string{}
	for _, requestedLabel := range requested {
		found := false
		for _, appliedLabel := range applied {
			if strings.EqualFold(strings.TrimSpace(requestedLabel), strings.TrimSpace(appliedLabel)) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, requestedLabel)
		}
	}
	if len(missing) == 0 {
		return outcome
	}
	outcome.Status = "warning"
	outcome.MissingLabels = missing
	outcome.RequiredCapability = "repository issues:write or triage permission"
	outcome.Remediation = fmt.Sprintf("Grant %s, then run `gira adopt issues --repo %s --issues %d --label %s --dry-run`.", outcome.RequiredCapability, repo.FullName(), issueNumber, strings.Join(missing, " --label "))
	return outcome
}

func ticketNewBody(input TicketNewInput, ticketType string) (string, TicketReleaseImpact, error) {
	if strings.TrimSpace(input.Body) != "" && strings.TrimSpace(input.BodyFile) != "" {
		return "", TicketReleaseImpact{}, fmt.Errorf("use either --body or --body-file, not both")
	}
	impact, block, err := ticketReleaseImpactForNewTicket(ticketType, input.ReleaseImpact, input.ReleaseImpactReason)
	if err != nil {
		return "", TicketReleaseImpact{}, err
	}
	if strings.TrimSpace(input.Body) != "" {
		return appendTicketReleaseImpact(input.Body, block), impact, nil
	}
	if strings.TrimSpace(input.BodyFile) != "" {
		content, err := os.ReadFile(input.BodyFile)
		if err != nil {
			return "", TicketReleaseImpact{}, fmt.Errorf("read --body-file: %w", err)
		}
		body := strings.TrimSpace(string(content))
		if body == "" {
			return "", TicketReleaseImpact{}, fmt.Errorf("--body-file is empty")
		}
		return appendTicketReleaseImpact(body, block), impact, nil
	}
	goal := strings.TrimSpace(input.Goal)
	if goal == "" {
		goal = input.Title
	}
	scope := noResponse(input.Scope)
	notes := noResponse(input.Notes)
	acceptance := input.Acceptance
	var b strings.Builder
	fmt.Fprintf(&b, "## Goal\n%s\n\n", goal)
	fmt.Fprintf(&b, "## Scope\n%s\n\n", scope)
	b.WriteString("## Acceptance Criteria\n")
	if len(acceptance) == 0 {
		b.WriteString("_No response_\n\n")
	} else {
		for _, item := range acceptance {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				fmt.Fprintf(&b, "- %s\n", trimmed)
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("## Doctor Impact\n_No response_\n\n")
	fmt.Fprintf(&b, "## Notes\n%s\n\n", notes)
	b.WriteString(DefaultProvenanceBlock())
	b.WriteString("\n")
	return appendTicketReleaseImpact(b.String(), block), impact, nil
}

func createRepoTicket(repo RepoRef, title string, body string, labels []string, milestone string, runner CommandRunner) (TicketCreatedIssue, error) {
	args := []string{"issue", "create", "--repo", repo.FullName(), "--title", title, "--body", body}
	for _, label := range labels {
		args = append(args, "--label", label)
	}
	if strings.TrimSpace(milestone) != "" {
		args = append(args, "--milestone", milestone)
	}
	out, err := runner.Run("gh", args...)
	if err != nil {
		return TicketCreatedIssue{}, err
	}
	url := strings.TrimSpace(string(out))
	number := extractPRNumber(url)
	return TicketCreatedIssue{Repo: repo.FullName(), Number: number, URL: url}, nil
}

func preflightTicketNewLabels(repo RepoRef, labels []string, runner CommandRunner) error {
	if len(labels) == 0 {
		return nil
	}
	rows, err := fetchRepoLabelNames(repo, runner)
	if err != nil {
		return fmt.Errorf("preflight labels: %w", err)
	}
	existing := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		name := strings.ToLower(strings.TrimSpace(row))
		if name != "" {
			existing[name] = struct{}{}
		}
	}
	missing := make([]string, 0)
	for _, label := range labels {
		trimmed := strings.TrimSpace(label)
		if trimmed == "" {
			continue
		}
		if _, ok := existing[strings.ToLower(trimmed)]; ok {
			continue
		}
		missing = append(missing, trimmed)
	}
	if len(missing) > 0 {
		suggestions := missingLabelCandidateText(missing, existing)
		if suggestions != "" {
			suggestions = "; candidates: " + suggestions
		}
		return fmt.Errorf("missing repo labels: %s%s; run `gira ops sync --repo %s --dry-run` for managed labels or create reviewed repo labels before ticket new", strings.Join(missing, ","), suggestions, repo.FullName())
	}
	return nil
}

func missingLabelCandidateText(missing []string, existing map[string]struct{}) string {
	candidates := []string{}
	seen := map[string]struct{}{}
	for _, missingLabel := range missing {
		axis, ok := labelAxis(missingLabel)
		if !ok {
			continue
		}
		for _, desired := range DesiredLabels {
			name := strings.TrimSpace(desired.Name)
			if name == "" || strings.EqualFold(name, missingLabel) {
				continue
			}
			candidateAxis, ok := labelAxis(name)
			if !ok || candidateAxis != axis {
				continue
			}
			if _, exists := existing[strings.ToLower(name)]; !exists {
				continue
			}
			if _, duplicate := seen[strings.ToLower(name)]; duplicate {
				continue
			}
			seen[strings.ToLower(name)] = struct{}{}
			candidates = append(candidates, name)
			if len(candidates) >= 8 {
				break
			}
		}
	}
	return strings.Join(candidates, ",")
}

func labelAxis(label string) (string, bool) {
	parts := strings.SplitN(strings.TrimSpace(label), ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", false
	}
	return strings.ToLower(strings.TrimSpace(parts[0])), true
}

func ticketNewLabels(ticketType string, priority string, extra []string) []string {
	labels := []string{"type:" + ticketType}
	if !hasLabelAxis(extra, "status") {
		labels = append(labels, "status:ready")
	}
	if priority != "" {
		labels = append(labels, "priority:"+priority)
	}
	seen := map[string]struct{}{}
	deduped := make([]string, 0, len(labels)+len(extra))
	for _, label := range append(labels, extra...) {
		trimmed := strings.TrimSpace(label)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		deduped = append(deduped, trimmed)
	}
	return deduped
}

func hasLabelAxis(labels []string, axis string) bool {
	axis = strings.ToLower(strings.TrimSpace(axis))
	for _, label := range labels {
		currentAxis, ok := labelAxis(label)
		if ok && currentAxis == axis {
			return true
		}
	}
	return false
}

func validTicketType(value string) bool {
	switch value {
	case "epic", "story", "task", "bug", "spike", "chore":
		return true
	default:
		return false
	}
}

func ticketNewTypeError(value string) error {
	base := "--type must be one of epic, story, task, bug, spike, chore"
	if strings.EqualFold(strings.TrimSpace(value), "feature") {
		return fmt.Errorf("unsupported --type feature; %s; for feature requests, use --type story --label enhancement when that label exists in the repo taxonomy", base)
	}
	return fmt.Errorf("%s", base)
}

func validTicketPriority(value string) bool {
	switch value {
	case "p0", "p1", "p2", "p3":
		return true
	default:
		return false
	}
}

func noResponse(value string) string {
	if strings.TrimSpace(value) == "" {
		return "_No response_"
	}
	return strings.TrimSpace(value)
}

func FormatTicketNew(report TicketNewReport) string {
	if report.DryRun {
		var b strings.Builder
		fmt.Fprintf(&b, "ticket new: dry-run %s\n", report.Title)
		fmt.Fprintf(&b, "repo: %s\n", report.Repo)
		fmt.Fprintf(&b, "labels: %s\n", strings.Join(report.Labels, ","))
		if strings.TrimSpace(report.Milestone) != "" {
			fmt.Fprintf(&b, "milestone: %s\n", report.Milestone)
		}
		if report.Start {
			b.WriteString("after create: start ticket\n")
		}
		b.WriteString(formatTicketReadinessHuman(report.TicketReadiness))
		fmt.Fprintf(&b, "body:\n%s\nnext step: %s\n", strings.TrimSpace(report.Body), report.NextStep)
		return b.String()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "ticket new: ticket #%d %s\n", report.Created.Number, report.Title)
	fmt.Fprintf(&b, "requested labels: %s\n", strings.Join(report.RequestedLabels, ","))
	fmt.Fprintf(&b, "applied labels: %s\n", strings.Join(report.AppliedLabels, ","))
	if report.LabelOutcome.Status == "warning" {
		fmt.Fprintf(&b, "label warning: missing=%s capability=%s\n", strings.Join(report.LabelOutcome.MissingLabels, ","), report.LabelOutcome.RequiredCapability)
	}
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}
