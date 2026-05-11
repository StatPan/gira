package gira

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
)

type JiraTransitionPlanInput struct {
	Repo       RepoRef `json:"-"`
	Key        string  `json:"key"`
	Target     string  `json:"target"`
	APIBase    string  `json:"api_base,omitempty"`
	ConfigRoot string  `json:"config_root,omitempty"`
	Email      string  `json:"-"`
	Token      string  `json:"-"`
	DryRun     bool    `json:"dry_run"`
}

type JiraTransitionPlanReport struct {
	Command            string                    `json:"command"`
	Repo               string                    `json:"repo"`
	Key                string                    `json:"key"`
	APIBase            string                    `json:"api_base"`
	CurrentStatus      string                    `json:"current_status"`
	Target             string                    `json:"target"`
	TargetStatuses     []string                  `json:"target_statuses"`
	Candidate          JiraTransitionCandidate   `json:"candidate,omitempty"`
	AllowedTransitions []JiraTransitionCandidate `json:"allowed_transitions"`
	Decision           string                    `json:"decision"`
	Reason             string                    `json:"reason,omitempty"`
	DryRun             bool                      `json:"dry_run"`
	ReadOnly           bool                      `json:"read_only"`
}

type JiraTransitionCandidate struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	ToStatus       string   `json:"to_status"`
	RequiredFields []string `json:"required_fields,omitempty"`
}

func BuildJiraTransitionPlan(input JiraTransitionPlanInput) (JiraTransitionPlanReport, error) {
	if !input.DryRun {
		return JiraTransitionPlanReport{}, fmt.Errorf("jira transition only supports --dry-run in this slice")
	}
	key, err := normalizeJiraKey(input.Key)
	if err != nil {
		return JiraTransitionPlanReport{}, err
	}
	target := strings.ToLower(strings.TrimSpace(input.Target))
	if target == "" {
		return JiraTransitionPlanReport{}, fmt.Errorf("--to is required for jira transition")
	}
	provider, err := loadJiraTransitionProviderConfig(input)
	if err != nil {
		return JiraTransitionPlanReport{}, err
	}
	apiBase := strings.TrimSpace(input.APIBase)
	if apiBase == "" {
		apiBase = provider.BaseURL
	}
	apiBase, err = normalizeJiraAPIBase(apiBase)
	if err != nil {
		return JiraTransitionPlanReport{}, err
	}
	email := strings.TrimSpace(input.Email)
	if email == "" {
		email = strings.TrimSpace(os.Getenv("JIRA_EMAIL"))
	}
	token := strings.TrimSpace(input.Token)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("JIRA_API_TOKEN"))
	}
	if email == "" || token == "" {
		return JiraTransitionPlanReport{}, fmt.Errorf("JIRA_EMAIL and JIRA_API_TOKEN are required for jira transition")
	}
	targetStatuses := jiraTargetStatuses(provider, target)
	item, err := FetchJiraIssueByKey(apiBase, key, email, token)
	if err != nil {
		return JiraTransitionPlanReport{}, err
	}
	report := JiraTransitionPlanReport{
		Command:        "jira transition",
		Repo:           input.Repo.FullName(),
		Key:            key,
		APIBase:        apiBase,
		CurrentStatus:  item.Status,
		Target:         target,
		TargetStatuses: targetStatuses,
		DryRun:         true,
		ReadOnly:       true,
	}
	if len(targetStatuses) == 0 {
		report.Decision = "unmapped_status"
		report.Reason = "providers.jira.status_map has no target Jira statuses for " + target
		return report, nil
	}
	if containsJiraTargetStatus(targetStatuses, item.Status) {
		report.Decision = "already_at_target"
		report.Reason = "Jira issue is already in a configured target status"
		return report, nil
	}
	transitions, err := fetchJiraIssueTransitions(apiBase, key, email, token)
	if err != nil {
		if isJiraTransitionPermissionDiagnostic(err) {
			report.Decision = "manual_admin_required"
			report.Reason = err.Error()
			return report, nil
		}
		return JiraTransitionPlanReport{}, err
	}
	report.AllowedTransitions = transitions
	if len(transitions) == 0 {
		report.Decision = "manual_admin_required"
		report.Reason = "Jira returned no allowed transitions for this issue"
		return report, nil
	}
	candidate, ok := findJiraTransitionCandidate(transitions, targetStatuses)
	if !ok {
		report.Decision = "missing_transition"
		report.Reason = "no allowed transition reaches a configured target status"
		return report, nil
	}
	report.Candidate = candidate
	if len(candidate.RequiredFields) > 0 {
		report.Decision = "manual_admin_required"
		report.Reason = "candidate transition requires fields that Gira will not populate in dry-run planner"
		return report, nil
	}
	report.Decision = "direct_transition"
	return report, nil
}

func ApplyJiraTransition(apiBase string, key string, transitionID string, email string, token string) error {
	key, err := normalizeJiraKey(key)
	if err != nil {
		return err
	}
	apiBase, err = normalizeJiraAPIBase(apiBase)
	if err != nil {
		return err
	}
	if strings.TrimSpace(transitionID) == "" {
		return fmt.Errorf("Jira transition id is required")
	}
	if strings.TrimSpace(email) == "" || strings.TrimSpace(token) == "" {
		return fmt.Errorf("JIRA_EMAIL and JIRA_API_TOKEN are required for jira transition apply")
	}
	payload, err := json.Marshal(map[string]any{
		"transition": map[string]string{"id": strings.TrimSpace(transitionID)},
	})
	if err != nil {
		return err
	}
	if _, err := jiraAPIPost(apiBase, "/rest/api/3/issue/"+url.PathEscape(key)+"/transitions", payload, email, token); err != nil {
		return fmt.Errorf("apply Jira transition for %s: %w", key, err)
	}
	return nil
}

func FormatJiraTransitionPlan(report JiraTransitionPlanReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "jira transition: %s %s -> %s\n", report.Decision, report.Key, report.Target)
	fmt.Fprintf(&b, "repo: %s\n", report.Repo)
	fmt.Fprintf(&b, "current_status: %s\n", report.CurrentStatus)
	fmt.Fprintf(&b, "target_statuses: %s\n", strings.Join(report.TargetStatuses, ","))
	if report.Candidate.ID != "" {
		fmt.Fprintf(&b, "candidate: %s %s -> %s\n", report.Candidate.ID, report.Candidate.Name, report.Candidate.ToStatus)
		if len(report.Candidate.RequiredFields) > 0 {
			fmt.Fprintf(&b, "required_fields: %s\n", strings.Join(report.Candidate.RequiredFields, ","))
		}
	}
	if strings.TrimSpace(report.Reason) != "" {
		fmt.Fprintf(&b, "reason: %s\n", report.Reason)
	}
	fmt.Fprintf(&b, "allowed_transitions: %d\n", len(report.AllowedTransitions))
	return b.String()
}

func loadJiraTransitionProviderConfig(input JiraTransitionPlanInput) (JiraProviderConfig, error) {
	entry, err := LoadGlobalRepoRegistryEntry(input.ConfigRoot, input.Repo)
	if err != nil {
		return JiraProviderConfig{}, fmt.Errorf("load Jira provider config: %w", err)
	}
	if entry.Providers == nil || entry.Providers.Jira == nil {
		return JiraProviderConfig{}, fmt.Errorf("repo registry has no providers.jira config")
	}
	return *entry.Providers.Jira, nil
}

func jiraTargetStatuses(provider JiraProviderConfig, target string) []string {
	for _, mapping := range provider.StatusMap {
		if strings.EqualFold(strings.TrimSpace(mapping.GiraStatus), target) {
			out := append([]string(nil), mapping.JiraStatuses...)
			sort.Strings(out)
			return out
		}
	}
	return nil
}

func fetchJiraIssueTransitions(apiBase string, key string, email string, token string) ([]JiraTransitionCandidate, error) {
	content, err := jiraAPIGet(apiBase, "/rest/api/3/issue/"+url.PathEscape(key)+"/transitions", map[string]string{"expand": "transitions.fields"}, email, token)
	if err != nil {
		return nil, fmt.Errorf("fetch Jira transitions for %s: %w", key, err)
	}
	var payload struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			To   struct {
				Name string `json:"name"`
			} `json:"to"`
			Fields map[string]struct {
				Required bool `json:"required"`
			} `json:"fields"`
		} `json:"transitions"`
	}
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, fmt.Errorf("parse Jira transitions JSON: %w", err)
	}
	out := make([]JiraTransitionCandidate, 0, len(payload.Transitions))
	for _, transition := range payload.Transitions {
		var required []string
		for field, meta := range transition.Fields {
			if meta.Required {
				required = append(required, field)
			}
		}
		sort.Strings(required)
		out = append(out, JiraTransitionCandidate{ID: transition.ID, Name: transition.Name, ToStatus: strings.TrimSpace(transition.To.Name), RequiredFields: required})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func findJiraTransitionCandidate(transitions []JiraTransitionCandidate, targets []string) (JiraTransitionCandidate, bool) {
	targetSet := map[string]struct{}{}
	for _, target := range targets {
		targetSet[strings.ToLower(strings.TrimSpace(target))] = struct{}{}
	}
	var fallback JiraTransitionCandidate
	found := false
	for _, transition := range transitions {
		if _, ok := targetSet[strings.ToLower(strings.TrimSpace(transition.ToStatus))]; ok {
			if len(transition.RequiredFields) == 0 {
				return transition, true
			}
			if !found {
				fallback = transition
				found = true
			}
		}
	}
	return fallback, found
}

func containsJiraTargetStatus(targets []string, status string) bool {
	normalized := strings.ToLower(strings.TrimSpace(status))
	for _, target := range targets {
		if strings.ToLower(strings.TrimSpace(target)) == normalized {
			return true
		}
	}
	return false
}

func isJiraTransitionPermissionDiagnostic(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{"401", "403", "forbidden", "unauthorized", "permission"} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}
