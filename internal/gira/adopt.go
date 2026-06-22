package gira

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type AdoptIssueInput struct {
	Repo            RepoRef  `json:"repo"`
	Issues          []int    `json:"issues,omitempty"`
	Milestone       string   `json:"milestone,omitempty"`
	Labels          []string `json:"labels,omitempty"`
	State           string   `json:"state,omitempty"`
	NormalizeStatus bool     `json:"normalize_status,omitempty"`
	DryRun          bool     `json:"dry_run"`
	Apply           bool     `json:"apply"`
}

const AdoptIssuesReportSchemaVersion = "adopt-issues-report/v1"

type AdoptIssuesReport struct {
	SchemaVersion   string              `json:"schema_version,omitempty"`
	Repo            string              `json:"repo"`
	DryRun          bool                `json:"dry_run"`
	Apply           bool                `json:"apply"`
	State           string              `json:"state"`
	Issues          []int               `json:"issues,omitempty"`
	Milestone       string              `json:"milestone,omitempty"`
	Labels          []string            `json:"labels,omitempty"`
	NormalizeStatus bool                `json:"normalize_status,omitempty"`
	Counts          AdoptIssuesCounts   `json:"counts"`
	Unmapped        []AdoptIssueItem    `json:"unmapped,omitempty"`
	BeforeUnmapped  []AdoptIssueItem    `json:"before_unmapped"`
	AfterUnmapped   []AdoptIssueItem    `json:"after_unmapped"`
	Actions         []AdoptIssuesAction `json:"actions,omitempty"`
	NextStep        string              `json:"next_step"`
	Approval        *ApprovalEvidence   `json:"approval,omitempty"`
}

func EnsureAdoptIssuesReportSchema(report *AdoptIssuesReport) {
	if report != nil && strings.TrimSpace(report.SchemaVersion) == "" {
		report.SchemaVersion = AdoptIssuesReportSchemaVersion
	}
}

type AdoptIssuesCounts struct {
	Scanned        int `json:"scanned"`
	Unmapped       int `json:"unmapped"`
	BeforeUnmapped int `json:"before_unmapped"`
	AfterUnmapped  int `json:"after_unmapped"`
	Selected       int `json:"selected"`
	WouldUpdate    int `json:"would_update"`
	AppliedUpdate  int `json:"applied_update"`
}

type AdoptIssueItem struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	State     string   `json:"state"`
	Labels    []string `json:"labels,omitempty"`
	Milestone string   `json:"milestone,omitempty"`
	URL       string   `json:"url,omitempty"`
	Reasons   []string `json:"reasons"`
}

type AdoptIssuesAction struct {
	Issue        int      `json:"issue"`
	Title        string   `json:"title"`
	Action       string   `json:"action"`
	Status       string   `json:"status"`
	Milestone    string   `json:"milestone,omitempty"`
	Labels       []string `json:"labels,omitempty"`
	RemoveLabels []string `json:"remove_labels,omitempty"`
	Reason       string   `json:"reason"`
}

type adoptRawIssue struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	State       string    `json:"state"`
	HTMLURL     string    `json:"html_url"`
	PullRequest *struct{} `json:"pull_request"`
	Labels      []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Milestone *struct {
		Title string `json:"title"`
	} `json:"milestone"`
}

func BuildAdoptIssuesReport(input AdoptIssueInput, runner CommandRunner) (AdoptIssuesReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if input.DryRun == input.Apply {
		return AdoptIssuesReport{}, fmt.Errorf("exactly one of --dry-run or --apply is required")
	}
	state := strings.ToLower(strings.TrimSpace(input.State))
	if state == "" {
		state = "open"
	}
	if state != "open" && state != "all" {
		return AdoptIssuesReport{}, fmt.Errorf("--state must be one of open, all")
	}
	issues, err := fetchAdoptIssues(input.Repo, state, runner)
	if err != nil {
		return AdoptIssuesReport{}, err
	}
	labels := normalizeAdoptLabels(input.Labels)
	report := AdoptIssuesReport{
		SchemaVersion:   AdoptIssuesReportSchemaVersion,
		Repo:            input.Repo.FullName(),
		DryRun:          input.DryRun,
		Apply:           input.Apply,
		State:           state,
		Issues:          append([]int(nil), input.Issues...),
		Milestone:       strings.TrimSpace(input.Milestone),
		Labels:          labels,
		NormalizeStatus: input.NormalizeStatus,
		BeforeUnmapped:  []AdoptIssueItem{},
		AfterUnmapped:   []AdoptIssueItem{},
	}
	report.Counts.Scanned = len(issues)
	for _, issue := range issues {
		item, unmapped := adoptIssueItem(issue)
		if unmapped {
			report.Unmapped = append(report.Unmapped, item)
		}
	}
	report.Counts.Unmapped = len(report.Unmapped)
	report.Counts.BeforeUnmapped = len(report.Unmapped)
	report.BeforeUnmapped = append([]AdoptIssueItem{}, report.Unmapped...)

	selected := map[int]struct{}{}
	for _, issueNumber := range input.Issues {
		if issueNumber <= 0 {
			return report, fmt.Errorf("--issue values must be > 0")
		}
		selected[issueNumber] = struct{}{}
	}
	if len(selected) == 0 {
		if !input.NormalizeStatus {
			report.NextStep = fmt.Sprintf("gira adopt issues --repo %s --issues 1-3 --milestone TITLE --label type:task --dry-run", QuoteShellArg(input.Repo.FullName()))
			if input.DryRun {
				report.Approval = AdoptIssuesApprovalEvidence(report)
			}
			return report, nil
		}
		for _, issue := range issues {
			selected[issue.Number] = struct{}{}
		}
	}
	if strings.TrimSpace(input.Milestone) == "" && len(normalizeAdoptLabels(input.Labels)) == 0 && !input.NormalizeStatus {
		return report, fmt.Errorf("--milestone, --label, or --normalize-status is required when issues are selected")
	}
	byNumber := map[int]AdoptIssueItem{}
	for _, issue := range issues {
		item, _ := adoptIssueItem(issue)
		byNumber[item.Number] = item
	}
	statusDoneExists := false
	if input.NormalizeStatus {
		statusDoneExists, err = repoHasLabel(input.Repo, "status:done", runner)
		if err != nil {
			return report, err
		}
	}
	for issueNumber := range selected {
		item, ok := byNumber[issueNumber]
		if !ok {
			return report, fmt.Errorf("issue #%d was not found in %s issues", issueNumber, state)
		}
		removeLabels := []string{}
		addLabels := append([]string{}, labels...)
		actionName := "issue:update"
		reason := "explicit issue adoption mapping"
		if input.NormalizeStatus && strings.EqualFold(item.State, "closed") {
			removeLabels = activeStatusLabels(item.Labels)
			if statusDoneExists && !hasLabel(item.Labels, "status:done") {
				addLabels = append(addLabels, "status:done")
			}
			addLabels = normalizeAdoptLabels(addLabels)
			if strings.TrimSpace(input.Milestone) == "" && len(labels) == 0 {
				actionName = "issue:normalize-status"
				reason = "closed issue status normalization"
			}
		}
		if strings.TrimSpace(input.Milestone) == "" && len(addLabels) == 0 && len(removeLabels) == 0 {
			continue
		}
		action := AdoptIssuesAction{Issue: item.Number, Title: item.Title, Action: actionName, Status: "planned", Milestone: strings.TrimSpace(input.Milestone), Labels: addLabels, RemoveLabels: removeLabels, Reason: reason}
		if input.Apply {
			if err := applyAdoptIssue(input.Repo, action, runner); err != nil {
				return report, err
			}
			action.Status = "applied"
			report.Counts.AppliedUpdate++
		} else {
			report.Counts.WouldUpdate++
		}
		report.Actions = append(report.Actions, action)
	}
	sort.Slice(report.Actions, func(i, j int) bool { return report.Actions[i].Issue < report.Actions[j].Issue })
	report.Counts.Selected = len(report.Actions)
	if input.Apply {
		report.AfterUnmapped = estimateAdoptIssuesAfterUnmapped(report.BeforeUnmapped, report.Actions)
		report.Counts.AfterUnmapped = len(report.AfterUnmapped)
	}
	if input.DryRun {
		report.NextStep = fmt.Sprintf("gira adopt issues --repo %s", QuoteShellArg(input.Repo.FullName()))
		if len(input.Issues) > 0 {
			report.NextStep += " --issues " + joinIssueNumbers(input.Issues)
		}
		if input.NormalizeStatus && len(input.Issues) == 0 {
			report.NextStep += " --state " + state
		}
		if strings.TrimSpace(input.Milestone) != "" {
			report.NextStep += " --milestone " + QuoteShellArg(input.Milestone)
		}
		for _, label := range labels {
			report.NextStep += " --label " + QuoteShellArg(label)
		}
		if input.NormalizeStatus {
			report.NextStep += " --normalize-status"
		}
		report.NextStep += " --apply"
		report.Approval = AdoptIssuesApprovalEvidence(report)
	} else {
		report.NextStep = fmt.Sprintf("gira status --repo %s", QuoteShellArg(input.Repo.FullName()))
	}
	return report, nil
}

func fetchAdoptIssues(repo RepoRef, state string, runner CommandRunner) ([]adoptRawIssue, error) {
	output, err := runner.Run("gh", "api", "repos/"+repo.FullName()+"/issues", "--paginate", "--slurp", "-X", "GET", "-f", "state="+state, "-f", "per_page=100")
	if err != nil {
		return nil, err
	}
	var pages json.RawMessage
	if err := json.Unmarshal(output, &pages); err != nil {
		return nil, fmt.Errorf("parse issue pages: %w", err)
	}
	rows, err := flattenPages(pages)
	if err != nil {
		return nil, err
	}
	issues := make([]adoptRawIssue, 0, len(rows))
	for _, row := range rows {
		var raw adoptRawIssue
		if err := json.Unmarshal(row, &raw); err != nil {
			return nil, fmt.Errorf("parse issue row: %w", err)
		}
		if raw.PullRequest != nil {
			continue
		}
		issues = append(issues, raw)
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].Number < issues[j].Number })
	return issues, nil
}

func adoptIssueItem(issue adoptRawIssue) (AdoptIssueItem, bool) {
	labels := make([]string, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		labels = append(labels, label.Name)
	}
	sort.Strings(labels)
	milestone := ""
	if issue.Milestone != nil {
		milestone = issue.Milestone.Title
	}
	return classifyAdoptIssueItem(AdoptIssueItem{Number: issue.Number, Title: issue.Title, State: issue.State, Labels: labels, Milestone: milestone, URL: issue.HTMLURL})
}

func classifyAdoptIssueItem(item AdoptIssueItem) (AdoptIssueItem, bool) {
	labels := append([]string{}, item.Labels...)
	sort.Strings(labels)
	reasons := []string{}
	if strings.TrimSpace(item.Milestone) == "" {
		reasons = append(reasons, "missing_milestone")
	}
	if !hasAnyLabelPrefix(labels, "type:") {
		reasons = append(reasons, "missing_type")
	}
	if !hasAnyLabelPrefix(labels, "status:") {
		reasons = append(reasons, "missing_status")
	}
	item.Labels = labels
	item.Reasons = reasons
	return item, len(reasons) > 0
}

func estimateAdoptIssuesAfterUnmapped(before []AdoptIssueItem, actions []AdoptIssuesAction) []AdoptIssueItem {
	byIssue := map[int]AdoptIssuesAction{}
	for _, action := range actions {
		if action.Status == "applied" {
			byIssue[action.Issue] = action
		}
	}
	after := []AdoptIssueItem{}
	for _, item := range before {
		action, ok := byIssue[item.Number]
		if ok {
			item = applyAdoptActionToItem(item, action)
		}
		classified, unmapped := classifyAdoptIssueItem(item)
		if unmapped {
			after = append(after, classified)
		}
	}
	return after
}

func applyAdoptActionToItem(item AdoptIssueItem, action AdoptIssuesAction) AdoptIssueItem {
	if strings.TrimSpace(action.Milestone) != "" {
		item.Milestone = strings.TrimSpace(action.Milestone)
	}
	remove := map[string]struct{}{}
	for _, label := range action.RemoveLabels {
		remove[strings.ToLower(strings.TrimSpace(label))] = struct{}{}
	}
	seen := map[string]struct{}{}
	labels := []string{}
	for _, label := range item.Labels {
		normalized := strings.ToLower(strings.TrimSpace(label))
		if normalized == "" {
			continue
		}
		if _, ok := remove[normalized]; ok {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		labels = append(labels, label)
	}
	for _, label := range action.Labels {
		trimmed := strings.TrimSpace(label)
		normalized := strings.ToLower(trimmed)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		labels = append(labels, trimmed)
	}
	sort.Strings(labels)
	item.Labels = labels
	return item
}

func applyAdoptIssue(repo RepoRef, action AdoptIssuesAction, runner CommandRunner) error {
	args := []string{"issue", "edit", strconv.Itoa(action.Issue), "--repo", repo.FullName()}
	if strings.TrimSpace(action.Milestone) != "" {
		args = append(args, "--milestone", action.Milestone)
	}
	for _, label := range action.Labels {
		args = append(args, "--add-label", label)
	}
	for _, label := range action.RemoveLabels {
		args = append(args, "--remove-label", label)
	}
	_, err := runner.Run("gh", args...)
	return err
}

func repoHasLabel(repo RepoRef, label string, runner CommandRunner) (bool, error) {
	labels, err := fetchRepoLabelNames(repo, runner)
	if err != nil {
		return false, err
	}
	for _, row := range labels {
		if strings.EqualFold(strings.TrimSpace(row), label) {
			return true, nil
		}
	}
	return false, nil
}

func normalizeAdoptLabels(values []string) []string {
	seen := map[string]struct{}{}
	labels := []string{}
	for _, value := range values {
		for _, label := range strings.Split(value, ",") {
			trimmed := strings.TrimSpace(label)
			if trimmed == "" {
				continue
			}
			if _, ok := seen[trimmed]; ok {
				continue
			}
			seen[trimmed] = struct{}{}
			labels = append(labels, trimmed)
		}
	}
	sort.Strings(labels)
	return labels
}

func hasAnyLabelPrefix(labels []string, prefix string) bool {
	for _, label := range labels {
		if strings.HasPrefix(strings.ToLower(label), prefix) {
			return true
		}
	}
	return false
}

func activeStatusLabels(labels []string) []string {
	active := map[string]struct{}{
		"status:ready":          {},
		"status:in-progress":    {},
		"status:in-review":      {},
		"status:blocked":        {},
		"status:needs-review":   {},
		"status:needs-design":   {},
		"status:needs-decision": {},
	}
	out := []string{}
	for _, label := range labels {
		normalized := strings.ToLower(strings.TrimSpace(label))
		if _, ok := active[normalized]; ok {
			out = append(out, label)
		}
	}
	sort.Strings(out)
	return out
}

func joinIssueNumbers(values []int) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ",")
}

func FormatAdoptIssuesReport(report AdoptIssuesReport) string {
	var b strings.Builder
	mode := "dry-run"
	if report.Apply {
		mode = "applied"
	}
	if report.Apply {
		fmt.Fprintf(&b, "adopt issues: %s scanned=%d before_unmapped=%d after_unmapped=%d selected=%d applied_update=%d\n", mode, report.Counts.Scanned, report.Counts.BeforeUnmapped, report.Counts.AfterUnmapped, report.Counts.Selected, report.Counts.AppliedUpdate)
	} else {
		fmt.Fprintf(&b, "adopt issues: %s scanned=%d unmapped=%d selected=%d\n", mode, report.Counts.Scanned, report.Counts.Unmapped, report.Counts.Selected)
	}
	if len(report.Unmapped) > 0 {
		if report.Apply {
			b.WriteString("before unmapped:\n")
		} else {
			b.WriteString("unmapped:\n")
		}
		for _, item := range report.Unmapped {
			fmt.Fprintf(&b, "  #%d %s reasons=%s", item.Number, item.Title, strings.Join(item.Reasons, ","))
			if item.Milestone != "" {
				fmt.Fprintf(&b, " milestone=%s", item.Milestone)
			}
			b.WriteString("\n")
		}
	}
	if report.Apply && len(report.AfterUnmapped) > 0 {
		b.WriteString("after unmapped:\n")
		for _, item := range report.AfterUnmapped {
			fmt.Fprintf(&b, "  #%d %s reasons=%s", item.Number, item.Title, strings.Join(item.Reasons, ","))
			if item.Milestone != "" {
				fmt.Fprintf(&b, " milestone=%s", item.Milestone)
			}
			b.WriteString("\n")
		}
	}
	if len(report.Actions) > 0 {
		b.WriteString("actions:\n")
		for _, action := range report.Actions {
			fmt.Fprintf(&b, "  %s issue #%d", action.Status, action.Issue)
			if action.Milestone != "" {
				fmt.Fprintf(&b, " milestone=%s", action.Milestone)
			}
			if len(action.Labels) > 0 {
				fmt.Fprintf(&b, " labels=%s", strings.Join(action.Labels, ","))
			}
			if len(action.RemoveLabels) > 0 {
				fmt.Fprintf(&b, " remove_labels=%s", strings.Join(action.RemoveLabels, ","))
			}
			b.WriteString("\n")
		}
	}
	if report.NextStep != "" {
		fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	}
	return b.String()
}
