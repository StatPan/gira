package gira

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type TriagePolicy struct {
	RequiredFields struct {
		Assignee bool `yaml:"assignee" json:"assignee"`
		Priority bool `yaml:"priority" json:"priority"`
		DueDate  bool `yaml:"due_date" json:"due_date"`
	} `yaml:"required_fields" json:"required_fields"`
	SLAWindowsByPriority map[string]string   `yaml:"sla_windows_by_priority" json:"sla_windows_by_priority"`
	SLAWindowsByType     map[string]string   `yaml:"sla_windows_by_type" json:"sla_windows_by_type"`
	EscalationLabels     map[string][]string `yaml:"escalation_labels" json:"escalation_labels"`
}

type TriageIssue struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	State     string   `json:"state"`
	Labels    []string `json:"labels"`
	Assignees []string `json:"assignees"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	URL       string   `json:"url"`
}

type TriageQueueReport struct {
	Repo      string                 `json:"repo"`
	Generated string                 `json:"generated_at"`
	Issues    []TriageIssue          `json:"issues"`
	Buckets   map[string][]TriageRow `json:"buckets"`
}

type TriageRow struct {
	Issue      TriageIssue `json:"issue"`
	Violations []string    `json:"violations"`
}

type TriageApplyReport struct {
	Repo      string              `json:"repo"`
	Mode      string              `json:"mode"`
	Generated string              `json:"generated_at"`
	Valid     bool                `json:"valid"`
	Actions   []TriageApplyAction `json:"actions"`
}

type TriageApplyAction struct {
	Issue  int      `json:"issue"`
	Add    []string `json:"add_labels"`
	Reason []string `json:"violations"`
}

type TriageClient interface {
	Repo() RepoRef
	ListOpenIssues() ([]TriageIssue, error)
	AddLabels(issue int, labels []string) error
}

type GHTriageClient struct {
	repo   RepoRef
	runner CommandRunner
}

func NewGHTriageClient(repo RepoRef, runner CommandRunner) GHTriageClient {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	return GHTriageClient{repo: repo, runner: runner}
}

func (c GHTriageClient) Repo() RepoRef { return c.repo }

func (c GHTriageClient) ListOpenIssues() ([]TriageIssue, error) {
	args := []string{"api", "repos/" + c.repo.FullName() + "/issues", "--paginate", "--slurp", "-X", "GET", "-f", "state=open", "-f", "per_page=100"}
	output, err := c.runner.Run("gh", args...)
	if err != nil {
		return nil, err
	}
	var pages json.RawMessage
	if err := json.Unmarshal(output, &pages); err != nil {
		return nil, fmt.Errorf("parse gh JSON: %w", err)
	}
	rows, err := flattenPages(pages)
	if err != nil {
		return nil, err
	}
	issues := make([]TriageIssue, 0, len(rows))
	for _, row := range rows {
		var raw struct {
			Number int    `json:"number"`
			Title  string `json:"title"`
			State  string `json:"state"`
			Labels []struct {
				Name string `json:"name"`
			} `json:"labels"`
			Assignees []struct {
				Login string `json:"login"`
			} `json:"assignees"`
			CreatedAt  string           `json:"created_at"`
			UpdatedAt  string           `json:"updated_at"`
			HTMLURL    string           `json:"html_url"`
			URL        string           `json:"url"`
			PullRequst *json.RawMessage `json:"pull_request"`
		}
		if err := json.Unmarshal(row, &raw); err != nil {
			return nil, fmt.Errorf("parse issue: %w", err)
		}
		if raw.PullRequst != nil {
			continue
		}
		labels := make([]string, 0, len(raw.Labels))
		for _, l := range raw.Labels {
			labels = append(labels, l.Name)
		}
		sort.Strings(labels)
		assignees := make([]string, 0, len(raw.Assignees))
		for _, a := range raw.Assignees {
			assignees = append(assignees, a.Login)
		}
		sort.Strings(assignees)
		url := raw.HTMLURL
		if url == "" {
			url = raw.URL
		}
		issues = append(issues, TriageIssue{Number: raw.Number, Title: raw.Title, State: raw.State, Labels: labels, Assignees: assignees, CreatedAt: raw.CreatedAt, UpdatedAt: raw.UpdatedAt, URL: url})
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].Number < issues[j].Number })
	return issues, nil
}

func (c GHTriageClient) AddLabels(issue int, labels []string) error {
	if len(labels) == 0 {
		return nil
	}
	args := []string{"issue", "edit", fmt.Sprintf("%d", issue), "--repo", c.repo.FullName()}
	for _, l := range labels {
		args = append(args, "--add-label", l)
	}
	_, err := c.runner.Run("gh", args...)
	return err
}

func LoadTriagePolicy(path string) (TriagePolicy, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return TriagePolicy{}, err
	}
	var policy TriagePolicy
	if err := yaml.Unmarshal(content, &policy); err != nil {
		return TriagePolicy{}, fmt.Errorf("invalid policy: %w", err)
	}
	if err := ValidateTriagePolicy(policy); err != nil {
		return TriagePolicy{}, err
	}
	return policy, nil
}

func ValidateTriagePolicy(policy TriagePolicy) error {
	for k, v := range policy.SLAWindowsByPriority {
		if strings.TrimSpace(k) == "" {
			return fmt.Errorf("invalid policy: sla_windows_by_priority key is empty")
		}
		if _, err := time.ParseDuration(v); err != nil {
			return fmt.Errorf("invalid policy: sla_windows_by_priority[%s]: %v", k, err)
		}
	}
	for k, v := range policy.SLAWindowsByType {
		if strings.TrimSpace(k) == "" {
			return fmt.Errorf("invalid policy: sla_windows_by_type key is empty")
		}
		if _, err := time.ParseDuration(v); err != nil {
			return fmt.Errorf("invalid policy: sla_windows_by_type[%s]: %v", k, err)
		}
	}
	return nil
}

func BuildTriageQueue(client TriageClient, now time.Time) (TriageQueueReport, error) {
	issues, err := client.ListOpenIssues()
	if err != nil {
		return TriageQueueReport{}, err
	}
	buckets := map[string][]TriageRow{"unowned": {}, "no-priority": {}, "no-due-date": {}, "overdue": {}, "sla-risk": {}}
	for _, issue := range issues {
		violations := classifyViolations(issue, now)
		row := TriageRow{Issue: issue, Violations: violations}
		if contains(violations, "missing_assignee") {
			buckets["unowned"] = append(buckets["unowned"], row)
		}
		if contains(violations, "missing_priority") {
			buckets["no-priority"] = append(buckets["no-priority"], row)
		}
		if contains(violations, "missing_due_date") {
			buckets["no-due-date"] = append(buckets["no-due-date"], row)
		}
		if contains(violations, "overdue") {
			buckets["overdue"] = append(buckets["overdue"], row)
		}
		if contains(violations, "sla_risk") {
			buckets["sla-risk"] = append(buckets["sla-risk"], row)
		}
	}
	return TriageQueueReport{Repo: client.Repo().FullName(), Generated: now.UTC().Format(time.RFC3339), Issues: issues, Buckets: buckets}, nil
}

func ApplyTriagePolicy(client TriageClient, policy TriagePolicy, apply bool, now time.Time) (TriageApplyReport, error) {
	if err := ValidateTriagePolicy(policy); err != nil {
		return TriageApplyReport{}, err
	}
	queue, err := BuildTriageQueue(client, now)
	if err != nil {
		return TriageApplyReport{}, err
	}
	actions := make([]TriageApplyAction, 0)
	for _, issue := range queue.Issues {
		violations := classifyViolations(issue, now)
		add := make([]string, 0)
		for _, v := range violations {
			add = append(add, policy.EscalationLabels[v]...)
		}
		add = labelsNotPresent(add, issue.Labels)
		if len(add) == 0 {
			continue
		}
		actions = append(actions, TriageApplyAction{Issue: issue.Number, Add: add, Reason: violations})
		if apply {
			if err := client.AddLabels(issue.Number, add); err != nil {
				return TriageApplyReport{}, err
			}
		}
	}
	mode := "dry-run"
	if apply {
		mode = "apply"
	}
	return TriageApplyReport{Repo: client.Repo().FullName(), Mode: mode, Generated: now.UTC().Format(time.RFC3339), Valid: true, Actions: actions}, nil
}

func classifyViolations(issue TriageIssue, now time.Time) []string {
	v := make([]string, 0)
	if len(issue.Assignees) == 0 {
		v = append(v, "missing_assignee")
	}
	priority := findPrefix(issue.Labels, "priority:")
	if priority == "" {
		v = append(v, "missing_priority")
	}
	due := findPrefix(issue.Labels, "due:")
	if due == "" {
		v = append(v, "missing_due_date")
	} else if d, err := time.Parse("2006-01-02", due); err == nil {
		if d.Before(now.UTC().Truncate(24 * time.Hour)) {
			v = append(v, "overdue")
		}
	}
	if created, err := time.Parse(time.RFC3339, issue.CreatedAt); err == nil {
		window := 7 * 24 * time.Hour
		if priority == "p0" || priority == "p1" {
			window = 48 * time.Hour
		}
		if created.Add(window).Before(now.Add(24 * time.Hour)) {
			v = append(v, "sla_risk")
		}
	}
	return v
}

func findPrefix(labels []string, prefix string) string {
	for _, l := range labels {
		if strings.HasPrefix(strings.ToLower(l), prefix) {
			return strings.TrimPrefix(strings.ToLower(l), prefix)
		}
	}
	return ""
}

func labelsNotPresent(add []string, existing []string) []string {
	set := map[string]struct{}{}
	for _, e := range existing {
		set[e] = struct{}{}
	}
	out := make([]string, 0, len(add))
	seen := map[string]struct{}{}
	for _, l := range add {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if _, ok := set[l]; ok {
			continue
		}
		if _, ok := seen[l]; ok {
			continue
		}
		seen[l] = struct{}{}
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
