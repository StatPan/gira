package gira

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type JiraDoctorInput struct {
	Repo       RepoRef `json:"-"`
	APIBase    string  `json:"api_base,omitempty"`
	Project    string  `json:"project,omitempty"`
	SampleKey  string  `json:"sample_key,omitempty"`
	ConfigRoot string  `json:"config_root,omitempty"`
	Email      string  `json:"-"`
	Token      string  `json:"-"`
}

type JiraDoctorReport struct {
	Command       string                     `json:"command"`
	Repo          string                     `json:"repo"`
	APIBase       string                     `json:"api_base,omitempty"`
	ConfigRoot    string                     `json:"config_root,omitempty"`
	ProjectKey    string                     `json:"project_key,omitempty"`
	Status        string                     `json:"status"`
	ReadOnly      bool                       `json:"read_only"`
	Provider      JiraProviderConfig         `json:"provider,omitempty"`
	Project       JiraProviderProject        `json:"project,omitempty"`
	IssueTypes    []JiraProviderIssueType    `json:"issue_types,omitempty"`
	Statuses      []JiraProviderStatus       `json:"statuses,omitempty"`
	Priorities    []JiraProviderPriority     `json:"priorities,omitempty"`
	Checks        []JiraDoctorCheck          `json:"checks"`
	Mirror        JiraDoctorMirrorDiagnostic `json:"mirror"`
	Transitions   JiraDoctorTransitionCheck  `json:"transitions"`
	Compatibility string                     `json:"compatibility"`
	NextSteps     []string                   `json:"next_steps,omitempty"`
}

type JiraDoctorCheck struct {
	Name        string `json:"id"`
	Status      string `json:"status"`
	Detail      string `json:"detail"`
	Remediation string `json:"remediation,omitempty"`
}

type JiraDoctorMirrorDiagnostic struct {
	Status            string                         `json:"status"`
	Detail            string                         `json:"detail"`
	SampleKey         string                         `json:"sample_key,omitempty"`
	SampleIssue       JiraMirrorIssue                `json:"sample_issue,omitempty"`
	IssueCount        int                            `json:"issue_count"`
	MirrorCount       int                            `json:"mirror_count"`
	DuplicateKeys     []JiraDoctorMirrorDuplicate    `json:"duplicate_keys,omitempty"`
	MissingKeyLabels  []JiraDoctorMirrorIssueProblem `json:"missing_key_labels,omitempty"`
	PermissionProblem string                         `json:"permission_problem,omitempty"`
}

type JiraDoctorMirrorDuplicate struct {
	Key    string            `json:"key"`
	Issues []JiraMirrorIssue `json:"issues"`
}

type JiraDoctorMirrorIssueProblem struct {
	Key   string          `json:"key"`
	Issue JiraMirrorIssue `json:"issue"`
}

type JiraDoctorTransitionCheck struct {
	Status             string                    `json:"status"`
	Detail             string                    `json:"detail"`
	SampleKey          string                    `json:"sample_key,omitempty"`
	CurrentStatus      string                    `json:"current_status,omitempty"`
	TargetStatuses     []string                  `json:"target_statuses,omitempty"`
	Candidate          JiraTransitionCandidate   `json:"candidate,omitempty"`
	AllowedTransitions []JiraTransitionCandidate `json:"allowed_transitions,omitempty"`
	RequiredFields     []string                  `json:"required_fields,omitempty"`
}

func BuildJiraDoctorReport(input JiraDoctorInput, runner CommandRunner) (JiraDoctorReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	root, rootErr := globalConfigRoot(input.ConfigRoot)
	if rootErr != nil {
		root = strings.TrimSpace(input.ConfigRoot)
	}
	report := JiraDoctorReport{
		Command:    "jira doctor",
		Repo:       input.Repo.FullName(),
		ConfigRoot: root,
		Status:     "ready",
		ReadOnly:   true,
	}
	add := func(name string, status string, detail string, remediation string) {
		report.Checks = append(report.Checks, JiraDoctorCheck{Name: name, Status: status, Detail: detail, Remediation: remediation})
	}
	if rootErr != nil {
		add("config_root", "blocked", rootErr.Error(), "Pass --config-root or configure a writable Gira home config directory.")
		report.Status = "blocked"
		report.Compatibility = "blocked"
		report.NextSteps = jiraDoctorNextSteps(report)
		return report, nil
	}

	provider, providerLoaded, configErr := resolveJiraDoctorProvider(input)
	if configErr != nil {
		add("provider_config", "blocked", configErr.Error(), fmt.Sprintf("Run `gira jira init --repo %s --api-base URL --project KEY --dry-run`.", input.Repo.FullName()))
	} else {
		add("provider_config", "ready", "providers.jira config loaded from the global repo registry", "")
	}
	if strings.TrimSpace(input.APIBase) != "" {
		apiBase, err := normalizeJiraAPIBase(input.APIBase)
		if err != nil {
			add("api_base", "blocked", err.Error(), "Pass a credential-free absolute Jira URL such as https://example.atlassian.net.")
			provider.BaseURL = ""
		} else {
			provider.BaseURL = apiBase
		}
	}
	if strings.TrimSpace(input.Project) != "" {
		provider.ProjectKey = strings.ToUpper(strings.TrimSpace(input.Project))
	}
	if providerLoaded || strings.TrimSpace(input.APIBase) != "" || strings.TrimSpace(input.Project) != "" {
		report.Provider = provider
	}
	report.APIBase = strings.TrimSpace(provider.BaseURL)
	report.ProjectKey = strings.ToUpper(strings.TrimSpace(provider.ProjectKey))
	if report.APIBase == "" {
		add("api_base", "blocked", "no Jira API base URL is configured", "Run jira init or pass --api-base.")
	}
	if report.ProjectKey == "" {
		add("project", "blocked", "no Jira project key is configured", "Run jira init or pass --project.")
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
		add("credentials", "blocked", "JIRA_EMAIL and JIRA_API_TOKEN are required for jira doctor", "Set Jira credentials in the environment; Gira will not write them to config.")
		report.Mirror = buildJiraDoctorMirrorDiagnostic(input.Repo, input.SampleKey, runner)
		report.Transitions = jiraDoctorTransitionSkipped(input.SampleKey)
		report.Status = aggregateJiraDoctorStatus(report)
		report.Compatibility = jiraDoctorCompatibility(report.Status)
		report.NextSteps = jiraDoctorNextSteps(report)
		return report, nil
	}
	if providerLoaded && configErr == nil {
		add("source_of_truth", "ready", "Jira owns planning/status while GitHub owns execution evidence", "")
	}

	if report.APIBase != "" && report.ProjectKey != "" {
		project, err := fetchJiraProviderProject(report.APIBase, report.ProjectKey, email, token)
		if err != nil {
			add("project_reachability", jiraDoctorAPIFailureStatus(err), err.Error(), "Confirm Jira URL, project key, credentials, and Browse Projects permission.")
		} else {
			report.Project = project
			add("project_reachability", "ready", fmt.Sprintf("%s %s (%s)", project.Key, project.Name, project.ProjectType), "")
			issueTypes, issueErr := fetchJiraProviderIssueTypes(report.APIBase, project.ID, email, token)
			if issueErr != nil {
				add("issue_types", jiraDoctorAPIFailureStatus(issueErr), issueErr.Error(), "Confirm Jira issue type read permission for the project.")
			} else {
				report.IssueTypes = issueTypes
				add("issue_types", jiraDoctorNonEmptyStatus(issueTypes), fmt.Sprintf("%d issue types discovered", len(issueTypes)), "Add at least one Jira issue type before using provider mode.")
			}
			statuses, statusErr := fetchJiraProviderStatuses(report.APIBase, project.ID, email, token)
			if statusErr != nil {
				add("statuses", jiraDoctorAPIFailureStatus(statusErr), statusErr.Error(), "Confirm Jira status read permission for the project.")
			} else {
				report.Statuses = statuses
				add("statuses", jiraDoctorNonEmptyStatus(statuses), fmt.Sprintf("%d statuses discovered", len(statuses)), "Add or expose Jira workflow statuses before using provider mode.")
				for _, check := range jiraDoctorStatusMapChecks(provider, statuses) {
					report.Checks = append(report.Checks, check)
				}
			}
			priorities, priorityErr := fetchJiraProviderPriorities(report.APIBase, project.ID, email, token)
			if priorityErr != nil {
				add("priorities", "warning", priorityErr.Error(), "Priority discovery is optional; mirror labels will be less complete until it is readable.")
			} else {
				report.Priorities = priorities
				add("priorities", jiraDoctorNonEmptyStatus(priorities), fmt.Sprintf("%d priorities discovered", len(priorities)), "Priority discovery is optional but useful for mirror labels.")
			}
		}
	}
	report.Mirror = buildJiraDoctorMirrorDiagnostic(input.Repo, input.SampleKey, runner)
	report.Checks = append(report.Checks, jiraDoctorMirrorCheck(report.Mirror))
	report.Transitions = buildJiraDoctorTransitionCheck(report.APIBase, provider, input.SampleKey, email, token)
	report.Checks = append(report.Checks, JiraDoctorCheck{Name: "transition_reachability", Status: report.Transitions.Status, Detail: report.Transitions.Detail, Remediation: jiraDoctorTransitionRemediation(report.Transitions)})
	report.Status = aggregateJiraDoctorStatus(report)
	report.Compatibility = jiraDoctorCompatibility(report.Status)
	report.NextSteps = jiraDoctorNextSteps(report)
	return report, nil
}

func resolveJiraDoctorProvider(input JiraDoctorInput) (JiraProviderConfig, bool, error) {
	entry, err := LoadGlobalRepoRegistryEntry(input.ConfigRoot, input.Repo)
	if err != nil {
		return JiraProviderConfig{}, false, fmt.Errorf("load Jira provider config: %w", err)
	}
	if entry.Providers == nil || entry.Providers.Jira == nil {
		return JiraProviderConfig{}, false, fmt.Errorf("repo registry has no providers.jira config")
	}
	provider := *entry.Providers.Jira
	if err := validateJiraProviderConfig("repo registry", "providers.jira", provider); err != nil {
		return provider, true, err
	}
	return provider, true, nil
}

func jiraDoctorNonEmptyStatus[T any](items []T) string {
	if len(items) == 0 {
		return "warning"
	}
	return "ready"
}

func jiraDoctorStatusMapChecks(provider JiraProviderConfig, statuses []JiraProviderStatus) []JiraDoctorCheck {
	statusNames := map[string]string{}
	for _, status := range statuses {
		statusNames[strings.ToLower(strings.TrimSpace(status.Name))] = strings.TrimSpace(status.Name)
	}
	mappedToGira := map[string][]string{}
	var missingConfigured []string
	for _, mapping := range provider.StatusMap {
		giraStatus := strings.TrimSpace(mapping.GiraStatus)
		for _, jiraStatus := range mapping.JiraStatuses {
			normalized := strings.ToLower(strings.TrimSpace(jiraStatus))
			if normalized == "" {
				continue
			}
			if _, ok := statusNames[normalized]; !ok {
				missingConfigured = append(missingConfigured, jiraStatus)
			}
			mappedToGira[normalized] = append(mappedToGira[normalized], giraStatus)
		}
	}
	var unmapped []string
	for _, status := range statuses {
		name := strings.TrimSpace(status.Name)
		if name == "" {
			continue
		}
		if _, ok := mappedToGira[strings.ToLower(name)]; !ok {
			unmapped = append(unmapped, name)
		}
	}
	var conflicts []string
	for jiraStatus, giraStatuses := range mappedToGira {
		unique := uniqueStrings(giraStatuses)
		if len(unique) > 1 {
			conflicts = append(conflicts, statusNames[jiraStatus]+" -> "+strings.Join(unique, ","))
		}
	}
	sort.Strings(unmapped)
	sort.Strings(missingConfigured)
	sort.Strings(conflicts)
	checks := []JiraDoctorCheck{}
	if len(conflicts) > 0 {
		checks = append(checks, JiraDoctorCheck{Name: "status_map_conflicts", Status: "blocked", Detail: strings.Join(conflicts, "; "), Remediation: "Map each Jira status to only one Gira status."})
	} else {
		checks = append(checks, JiraDoctorCheck{Name: "status_map_conflicts", Status: "ready", Detail: "no Jira status is mapped to multiple Gira statuses"})
	}
	if len(unmapped) > 0 {
		checks = append(checks, JiraDoctorCheck{Name: "unmapped_statuses", Status: "warning", Detail: strings.Join(unmapped, ", "), Remediation: "Add these Jira statuses to providers.jira.status_map or intentionally document them as unsupported."})
	} else {
		checks = append(checks, JiraDoctorCheck{Name: "unmapped_statuses", Status: "ready", Detail: "all discovered Jira statuses are mapped"})
	}
	if len(missingConfigured) > 0 {
		checks = append(checks, JiraDoctorCheck{Name: "configured_statuses_exist", Status: "warning", Detail: strings.Join(uniqueStrings(missingConfigured), ", "), Remediation: "Remove stale status_map entries or update them to current Jira status names."})
	} else {
		checks = append(checks, JiraDoctorCheck{Name: "configured_statuses_exist", Status: "ready", Detail: "configured Jira statuses exist in the project"})
	}
	return checks
}

func buildJiraDoctorMirrorDiagnostic(repo RepoRef, sampleKey string, runner CommandRunner) JiraDoctorMirrorDiagnostic {
	output, err := runner.Run("gh", "issue", "list", "--repo", repo.FullName(), "--state", "all", "--limit", "1000", "--json", "number,title,body,url,labels")
	if err != nil {
		return JiraDoctorMirrorDiagnostic{Status: "blocked", Detail: "GitHub issue list failed", PermissionProblem: err.Error()}
	}
	var rows []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		URL    string `json:"url"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.Unmarshal(output, &rows); err != nil {
		return JiraDoctorMirrorDiagnostic{Status: "blocked", Detail: "parse GitHub issues: " + err.Error()}
	}
	normalizedSample := strings.TrimSpace(sampleKey)
	if normalizedSample != "" {
		var err error
		normalizedSample, err = normalizeJiraKey(normalizedSample)
		if err != nil {
			return JiraDoctorMirrorDiagnostic{Status: "blocked", Detail: err.Error(), SampleKey: sampleKey, IssueCount: len(rows)}
		}
	}
	byKey := map[string][]JiraMirrorIssue{}
	var missing []JiraDoctorMirrorIssueProblem
	for _, row := range rows {
		key := JiraKeyFromBody(row.Body)
		if key == "" {
			continue
		}
		issue := JiraMirrorIssue{Number: row.Number, Title: row.Title, URL: row.URL}
		byKey[key] = append(byKey[key], issue)
		if !jiraDoctorHasLabel(row.Labels, "jira:"+key) {
			missing = append(missing, JiraDoctorMirrorIssueProblem{Key: key, Issue: issue})
		}
	}
	var duplicates []JiraDoctorMirrorDuplicate
	mirrorCount := 0
	for key, issues := range byKey {
		mirrorCount += len(issues)
		sort.Slice(issues, func(i, j int) bool { return issues[i].Number < issues[j].Number })
		if len(issues) > 1 {
			duplicates = append(duplicates, JiraDoctorMirrorDuplicate{Key: key, Issues: issues})
		}
	}
	sort.Slice(duplicates, func(i, j int) bool { return duplicates[i].Key < duplicates[j].Key })
	sort.Slice(missing, func(i, j int) bool {
		if missing[i].Key == missing[j].Key {
			return missing[i].Issue.Number < missing[j].Issue.Number
		}
		return missing[i].Key < missing[j].Key
	})
	status := "ready"
	detail := fmt.Sprintf("%d Jira mirror issues found", mirrorCount)
	var sampleIssue JiraMirrorIssue
	if len(duplicates) > 0 {
		status = "blocked"
		detail = fmt.Sprintf("%d duplicate Jira mirror keys found", len(duplicates))
	} else if normalizedSample != "" {
		matches := byKey[normalizedSample]
		switch len(matches) {
		case 0:
			status = "blocked"
			detail = "sample Jira key has no GitHub mirror issue"
		case 1:
			sampleIssue = matches[0]
		default:
			status = "blocked"
			detail = "sample Jira key has multiple GitHub mirror issues"
		}
	} else if len(missing) > 0 {
		status = "warning"
		detail = fmt.Sprintf("%d Jira mirror issues are missing jira:KEY labels", len(missing))
	} else if mirrorCount == 0 {
		status = "warning"
		detail = "no Jira mirror issues found yet"
	}
	if status == "ready" && len(missing) > 0 {
		status = "warning"
		detail = fmt.Sprintf("%d Jira mirror issues are missing jira:KEY labels", len(missing))
	}
	return JiraDoctorMirrorDiagnostic{Status: status, Detail: detail, SampleKey: normalizedSample, SampleIssue: sampleIssue, IssueCount: len(rows), MirrorCount: mirrorCount, DuplicateKeys: duplicates, MissingKeyLabels: missing}
}

func jiraDoctorHasLabel(labels []struct {
	Name string `json:"name"`
}, want string) bool {
	for _, label := range labels {
		if strings.EqualFold(strings.TrimSpace(label.Name), want) {
			return true
		}
	}
	return false
}

func jiraDoctorMirrorCheck(mirror JiraDoctorMirrorDiagnostic) JiraDoctorCheck {
	remediation := ""
	switch mirror.Status {
	case "blocked":
		remediation = "Resolve duplicate Jira-Key metadata or fix GitHub issue read permissions."
	case "warning":
		remediation = "Run `gira jira mirror KEY --apply` for missing mirrors, or add missing jira:KEY labels to existing mirrors."
	}
	return JiraDoctorCheck{Name: "mirror_issue_health", Status: mirror.Status, Detail: mirror.Detail, Remediation: remediation}
}

func buildJiraDoctorTransitionCheck(apiBase string, provider JiraProviderConfig, sampleKey string, email string, token string) JiraDoctorTransitionCheck {
	sampleKey = strings.TrimSpace(sampleKey)
	if sampleKey == "" {
		return jiraDoctorTransitionSkipped("")
	}
	key, err := normalizeJiraKey(sampleKey)
	if err != nil {
		return JiraDoctorTransitionCheck{Status: "blocked", SampleKey: sampleKey, Detail: err.Error()}
	}
	if strings.TrimSpace(apiBase) == "" {
		return JiraDoctorTransitionCheck{Status: "blocked", SampleKey: key, Detail: "no Jira API base URL is configured"}
	}
	targetStatuses := jiraTargetStatuses(provider, "done")
	if len(targetStatuses) == 0 {
		return JiraDoctorTransitionCheck{Status: "blocked", SampleKey: key, Detail: "providers.jira.status_map has no target Jira statuses for done"}
	}
	item, err := FetchJiraIssueByKey(apiBase, key, email, token)
	if err != nil {
		return JiraDoctorTransitionCheck{Status: jiraDoctorAPIFailureStatus(err), SampleKey: key, Detail: err.Error(), TargetStatuses: targetStatuses}
	}
	transitions, err := fetchJiraIssueTransitions(apiBase, key, email, token)
	if err != nil {
		return JiraDoctorTransitionCheck{Status: jiraDoctorAPIFailureStatus(err), SampleKey: key, CurrentStatus: item.Status, Detail: err.Error(), TargetStatuses: targetStatuses}
	}
	check := JiraDoctorTransitionCheck{
		Status:             "ready",
		Detail:             "sample issue has a direct Done transition without required fields",
		SampleKey:          key,
		CurrentStatus:      item.Status,
		TargetStatuses:     targetStatuses,
		AllowedTransitions: transitions,
	}
	if containsJiraTargetStatus(targetStatuses, item.Status) {
		check.Status = "warning"
		check.Detail = "sample issue is already in a configured Done status; use a representative non-Done issue to verify inbound Done transition reachability"
		return check
	}
	candidate, ok := findJiraTransitionCandidate(transitions, targetStatuses)
	if !ok {
		check.Status = "warning"
		check.Detail = "sample issue has no allowed transition to a configured Done status"
		return check
	}
	check.Candidate = candidate
	check.RequiredFields = append([]string(nil), candidate.RequiredFields...)
	if len(candidate.RequiredFields) > 0 {
		check.Status = "blocked"
		check.Detail = "sample Done transition requires fields that Gira will not populate automatically"
	}
	return check
}

func jiraDoctorTransitionSkipped(sampleKey string) JiraDoctorTransitionCheck {
	detail := "transition reachability is issue-specific; pass --sample-key JIRA-123 to inspect allowed transitions and required fields"
	if strings.TrimSpace(sampleKey) != "" {
		detail = "transition reachability was not checked"
	}
	return JiraDoctorTransitionCheck{Status: "warning", SampleKey: strings.TrimSpace(sampleKey), Detail: detail}
}

func jiraDoctorTransitionRemediation(check JiraDoctorTransitionCheck) string {
	switch check.Status {
	case "blocked":
		return "Adjust Jira workflow/status_map or handle required transition fields manually before relying on Done automation."
	case "warning":
		if strings.TrimSpace(check.SampleKey) == "" {
			return "Run jira doctor again with --sample-key for a representative issue in this workflow."
		}
		return "Use Jira admin workflow settings or status_map changes to make completion paths explicit."
	default:
		return ""
	}
}

func jiraDoctorAPIFailureStatus(err error) string {
	if isJiraTransitionPermissionDiagnostic(err) {
		return "blocked"
	}
	return "blocked"
}

func aggregateJiraDoctorStatus(report JiraDoctorReport) string {
	status := "ready"
	for _, check := range report.Checks {
		switch check.Status {
		case "blocked":
			return "blocked"
		case "warning":
			status = "warning"
		}
	}
	return status
}

func jiraDoctorCompatibility(status string) string {
	switch status {
	case "ready":
		return "supported"
	case "warning":
		return "partially_supported"
	default:
		return "blocked"
	}
}

func jiraDoctorNextSteps(report JiraDoctorReport) []string {
	var steps []string
	for _, check := range report.Checks {
		if strings.TrimSpace(check.Remediation) != "" && check.Status != "ready" {
			steps = append(steps, check.Remediation)
		}
	}
	if len(steps) == 0 {
		steps = append(steps, "Use `gira jira transition KEY --to done --dry-run` before any Jira Done convergence.")
	}
	return uniqueStrings(steps)
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func FormatJiraDoctorReport(report JiraDoctorReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "jira doctor: %s (%s)\n", report.Status, report.Compatibility)
	fmt.Fprintf(&b, "repo: %s\n", report.Repo)
	if report.ProjectKey != "" {
		fmt.Fprintf(&b, "project: %s\n", report.ProjectKey)
	}
	if report.Project.Name != "" {
		fmt.Fprintf(&b, "project_type: %s simplified=%t\n", report.Project.ProjectType, report.Project.Simplified)
	}
	b.WriteString("checks:\n")
	for _, check := range report.Checks {
		fmt.Fprintf(&b, "  - %s: %s", check.Name, check.Status)
		if strings.TrimSpace(check.Detail) != "" {
			fmt.Fprintf(&b, " - %s", check.Detail)
		}
		b.WriteString("\n")
		if strings.TrimSpace(check.Remediation) != "" && check.Status != "ready" {
			fmt.Fprintf(&b, "    fix: %s\n", check.Remediation)
		}
	}
	fmt.Fprintf(&b, "mirror_issues: %s (%d mirrors / %d issues)\n", report.Mirror.Status, report.Mirror.MirrorCount, report.Mirror.IssueCount)
	for _, duplicate := range report.Mirror.DuplicateKeys {
		fmt.Fprintf(&b, "  duplicate %s:", duplicate.Key)
		for _, issue := range duplicate.Issues {
			fmt.Fprintf(&b, " #%d", issue.Number)
		}
		b.WriteString("\n")
	}
	if report.Transitions.SampleKey != "" || report.Transitions.Status != "" {
		fmt.Fprintf(&b, "transition_sample: %s", report.Transitions.Status)
		if report.Transitions.SampleKey != "" {
			fmt.Fprintf(&b, " %s", report.Transitions.SampleKey)
		}
		if report.Transitions.Detail != "" {
			fmt.Fprintf(&b, " - %s", report.Transitions.Detail)
		}
		b.WriteString("\n")
	}
	if len(report.NextSteps) > 0 {
		b.WriteString("next steps:\n")
		for _, step := range report.NextSteps {
			fmt.Fprintf(&b, "  - %s\n", step)
		}
	}
	return b.String()
}
