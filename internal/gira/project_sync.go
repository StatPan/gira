package gira

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const ProductOSProjectName = "Product OS"

type CanonicalProjectField struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Type string `json:"type"`
}

var ProductOSCanonicalFields = []CanonicalProjectField{
	{Key: "status", Name: "Status", Type: "SINGLE_SELECT"},
	{Key: "priority", Name: "Priority", Type: "SINGLE_SELECT"},
	{Key: "layer", Name: "Layer / workstream", Type: "SINGLE_SELECT"},
	{Key: "owner_agent", Name: "Owner / agent", Type: "SINGLE_SELECT"},
	{Key: "start_date", Name: "Start date", Type: "DATE"},
	{Key: "target_date", Name: "Target date", Type: "DATE"},
}

type ProjectSyncClient interface {
	Repo() RepoRef
	Snapshot(projectName string) (ProjectSyncSnapshot, error)
}

type GHProjectSyncClient struct {
	repo   RepoRef
	runner CommandRunner
}

func NewGHProjectSyncClient(repo RepoRef, runner CommandRunner) GHProjectSyncClient {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	return GHProjectSyncClient{repo: repo, runner: runner}
}

func (c GHProjectSyncClient) Repo() RepoRef {
	return c.repo
}

type ProjectSyncSnapshot struct {
	ProjectName  string
	FieldTypes   map[string]string
	RoadmapItems []ProjectRoadmapItem
}

type ProjectRoadmapItem struct {
	IssueNumber      int
	IssueTitle       string
	IssueURL         string
	TypeLabel        string
	Roadmapable      bool
	StartDate        *string
	TargetDate       *string
	MilestoneDueDate *string
}

type ProjectSyncReport struct {
	Repo           string                  `json:"repo"`
	Project        string                  `json:"project"`
	Command        string                  `json:"command"`
	DryRun         bool                    `json:"dry_run"`
	MissingProject bool                    `json:"missing_project"`
	Counts         ProjectSyncCounts       `json:"counts"`
	Fields         []ProjectFieldDiff      `json:"fields"`
	DateValidation []ProjectDateValidation `json:"date_validation"`
	FetchedAt      string                  `json:"fetched_at"`
}

type ProjectSyncCounts struct {
	FieldsPresent  int `json:"fields_present"`
	FieldsMissing  int `json:"fields_missing"`
	FieldsMismatch int `json:"fields_mismatch"`
	DateWarnings   int `json:"date_warnings"`
	DateBlocks     int `json:"date_blocks"`
}

type ProjectFieldDiff struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Present      bool   `json:"present"`
	ActualType   string `json:"actual_type,omitempty"`
	TypeMismatch bool   `json:"type_mismatch,omitempty"`
}

type ProjectDateFallback struct {
	Source string `json:"source"`
	Value  string `json:"value"`
}

type ProjectDateValidation struct {
	Issue      int                  `json:"issue"`
	Title      string               `json:"title"`
	Status     string               `json:"status"`
	Severity   string               `json:"severity"`
	StartDate  *string              `json:"start_date,omitempty"`
	TargetDate *string              `json:"target_date,omitempty"`
	Fallback   *ProjectDateFallback `json:"fallback,omitempty"`
	Reason     string               `json:"reason,omitempty"`
}

type ProjectSyncApplyReport struct {
	Repo         string                             `json:"repo"`
	Command      string                             `json:"command"`
	DryRun       bool                               `json:"dry_run"`
	Capabilities map[string]ProjectCapabilityStatus `json:"capabilities"`
	Applied      []ProjectSyncApplyAction           `json:"applied"`
	Skipped      []ProjectSyncSkippedAction         `json:"skipped"`
	BlockedCount int                                `json:"blocked_count"`
}

type ProjectSyncApplyAction struct {
	Action   string `json:"action"`
	Required string `json:"required"`
	Result   string `json:"result"`
}

type ProjectSyncSkippedAction struct {
	Action   string `json:"action"`
	Required string `json:"required"`
	Reason   string `json:"reason"`
}

func (c GHProjectSyncClient) Snapshot(projectName string) (ProjectSyncSnapshot, error) {
	query := `query($o: String!, $n: String!){ repository(owner: $o, name: $n){ projectsV2(first: 20){ nodes{ title fields(first: 50){ nodes{ ... on ProjectV2Field{ name dataType } ... on ProjectV2SingleSelectField{ name dataType } ... on ProjectV2IterationField{ name dataType } } } items(first: 100){ nodes{ content{ ... on Issue{ number title url labels(first: 50){ nodes{ name } } milestone{ dueOn } } } fieldValues(first: 50){ nodes{ ... on ProjectV2ItemFieldDateValue{ date field{ ... on ProjectV2FieldCommon{ name } } } } } } } } } } }`
	output, err := c.runner.Run("gh", "api", "graphql", "-f", "query="+query, "-f", "o="+c.repo.Owner, "-f", "n="+c.repo.Name)
	if err != nil {
		return ProjectSyncSnapshot{}, err
	}

	var payload struct {
		Data struct {
			Repository struct {
				ProjectsV2 struct {
					Nodes []struct {
						Title  string `json:"title"`
						Fields struct {
							Nodes []struct {
								Name     string `json:"name"`
								DataType string `json:"dataType"`
							} `json:"nodes"`
						} `json:"fields"`
						Items struct {
							Nodes []struct {
								Content *struct {
									Number int    `json:"number"`
									Title  string `json:"title"`
									URL    string `json:"url"`
									Labels struct {
										Nodes []struct {
											Name string `json:"name"`
										} `json:"nodes"`
									} `json:"labels"`
									Milestone *struct {
										DueOn *string `json:"dueOn"`
									} `json:"milestone"`
								} `json:"content"`
								FieldValues struct {
									Nodes []struct {
										Date  *string `json:"date"`
										Field *struct {
											Name string `json:"name"`
										} `json:"field"`
									} `json:"nodes"`
								} `json:"fieldValues"`
							} `json:"nodes"`
						} `json:"items"`
					} `json:"nodes"`
				} `json:"projectsV2"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return ProjectSyncSnapshot{}, fmt.Errorf("parse project sync graphql: %w", err)
	}

	snapshot := ProjectSyncSnapshot{
		FieldTypes: make(map[string]string),
	}
	for _, project := range payload.Data.Repository.ProjectsV2.Nodes {
		if project.Title != projectName {
			continue
		}
		snapshot.ProjectName = project.Title
		for _, field := range project.Fields.Nodes {
			if field.Name == "" {
				continue
			}
			snapshot.FieldTypes[field.Name] = strings.ToUpper(field.DataType)
		}
		for _, item := range project.Items.Nodes {
			if item.Content == nil {
				continue
			}
			labels := make([]string, 0, len(item.Content.Labels.Nodes))
			for _, label := range item.Content.Labels.Nodes {
				labels = append(labels, label.Name)
			}
			typeLabel := roadmapTypeLabel(labels)
			roadmap := ProjectRoadmapItem{
				IssueNumber: item.Content.Number,
				IssueTitle:  item.Content.Title,
				IssueURL:    item.Content.URL,
				TypeLabel:   typeLabel,
			}
			if item.Content.Milestone != nil && item.Content.Milestone.DueOn != nil {
				if due, ok := normalizeDate(*item.Content.Milestone.DueOn); ok {
					roadmap.MilestoneDueDate = &due
				}
			}
			for _, value := range item.FieldValues.Nodes {
				if value.Field == nil || value.Date == nil {
					continue
				}
				normalized, ok := normalizeDate(*value.Date)
				if !ok {
					continue
				}
				switch value.Field.Name {
				case "Start date":
					roadmap.StartDate = &normalized
				case "Target date":
					roadmap.TargetDate = &normalized
				}
			}
			roadmap.Roadmapable = isRoadmapable(roadmap)
			snapshot.RoadmapItems = append(snapshot.RoadmapItems, roadmap)
		}
		break
	}
	return snapshot, nil
}

func BuildProjectSyncReportForClient(client ProjectSyncClient, fetchedAt time.Time) (ProjectSyncReport, error) {
	snapshot, err := client.Snapshot(ProductOSProjectName)
	if err != nil {
		return ProjectSyncReport{}, err
	}
	return BuildProjectSyncReport(client.Repo().FullName(), snapshot, fetchedAt)
}

func BuildProjectSyncReport(repo string, snapshot ProjectSyncSnapshot, fetchedAt time.Time) (ProjectSyncReport, error) {
	report := ProjectSyncReport{
		Repo:      repo,
		Project:   ProductOSProjectName,
		Command:   "project sync",
		DryRun:    true,
		Fields:    make([]ProjectFieldDiff, 0, len(ProductOSCanonicalFields)),
		FetchedAt: formatGitHubTime(fetchedAt),
	}
	if snapshot.ProjectName == "" {
		report.MissingProject = true
	}

	for _, canonical := range ProductOSCanonicalFields {
		actualType, present := snapshot.FieldTypes[canonical.Name]
		field := ProjectFieldDiff{
			Key:     canonical.Key,
			Name:    canonical.Name,
			Type:    canonical.Type,
			Present: present,
		}
		if present {
			field.ActualType = strings.ToUpper(actualType)
			if field.ActualType == canonical.Type {
				report.Counts.FieldsPresent++
			} else {
				field.TypeMismatch = true
				report.Counts.FieldsMismatch++
			}
		} else {
			report.Counts.FieldsMissing++
		}
		report.Fields = append(report.Fields, field)
	}

	validations, err := buildProjectDateValidation(snapshot.RoadmapItems)
	if err != nil {
		return ProjectSyncReport{}, err
	}
	report.DateValidation = validations
	for _, validation := range validations {
		switch validation.Severity {
		case "warn":
			report.Counts.DateWarnings++
		case "block":
			report.Counts.DateBlocks++
		}
	}

	return report, nil
}

func BuildProjectSyncApplyReport(capability ProjectCapabilityReport) ProjectSyncApplyReport {
	report := ProjectSyncApplyReport{
		Repo:         capability.Repo,
		Command:      "project sync",
		DryRun:       false,
		Capabilities: capability.Capabilities,
		Applied:      make([]ProjectSyncApplyAction, 0),
		Skipped:      make([]ProjectSyncSkippedAction, 0),
	}

	appendAction := func(action, required, result string) {
		report.Applied = append(report.Applied, ProjectSyncApplyAction{Action: action, Required: required, Result: result})
	}
	appendDenied := func(action, required string) {
		report.Skipped = append(report.Skipped, ProjectSyncSkippedAction{
			Action:   action,
			Required: required,
			Reason:   "permission denied: " + action + " requires " + required,
		})
		report.BlockedCount++
	}

	if capability.Capabilities["projectsv2:read"] == ProjectCapabilityAllowed {
		appendAction("date_validation_report", "projectsv2:read", "ok")
	} else {
		appendDenied("date_validation_report", "projectsv2:read")
	}

	if capability.Capabilities["projectsv2:write"] == ProjectCapabilityAllowed {
		appendAction("project_status_field:update", "projectsv2:write", "ok")
	} else {
		appendDenied("project_status_field:update", "projectsv2:write")
	}

	if capability.Capabilities["issues:write"] == ProjectCapabilityAllowed {
		appendAction("milestone_complete_annotation:create", "issues:write", "ok")
	} else {
		appendDenied("milestone_complete_annotation:create", "issues:write")
	}

	return report
}

func buildProjectDateValidation(items []ProjectRoadmapItem) ([]ProjectDateValidation, error) {
	sorted := append([]ProjectRoadmapItem(nil), items...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].IssueNumber == sorted[j].IssueNumber {
			return sorted[i].IssueTitle < sorted[j].IssueTitle
		}
		return sorted[i].IssueNumber < sorted[j].IssueNumber
	})

	validations := make([]ProjectDateValidation, 0)
	for _, item := range sorted {
		start, err := normalizeDatePtr(item.StartDate)
		if err != nil {
			return nil, err
		}
		target, err := normalizeDatePtr(item.TargetDate)
		if err != nil {
			return nil, err
		}
		due, err := normalizeDatePtr(item.MilestoneDueDate)
		if err != nil {
			return nil, err
		}
		if !item.Roadmapable {
			reason := "missing roadmapable type label"
			if item.TypeLabel != "" {
				reason = item.TypeLabel + " requires a meaningful delivery checkpoint or release marker"
			}
			validations = append(validations, ProjectDateValidation{
				Issue:    item.IssueNumber,
				Title:    item.IssueTitle,
				Status:   "not_roadmapable",
				Severity: "info",
				Reason:   reason,
			})
			continue
		}

		switch {
		case start == nil && target == nil:
			validation := ProjectDateValidation{
				Issue:    item.IssueNumber,
				Title:    item.IssueTitle,
				Status:   "missing_dates",
				Severity: "warn",
				Reason:   "missing_required_field:start_date,target_date",
			}
			if due != nil {
				validation.Status = "phase_due_date_fallback"
				validation.Fallback = &ProjectDateFallback{Source: "milestone_due_date", Value: *due}
				validation.Reason = "missing_required_field:start_date,target_date;fallback:milestone_due_date"
			}
			validations = append(validations, validation)
		case start == nil && target != nil:
			validations = append(validations, ProjectDateValidation{
				Issue:      item.IssueNumber,
				Title:      item.IssueTitle,
				Status:     "missing_start_date",
				Severity:   "warn",
				TargetDate: target,
				Reason:     "missing_required_field:start_date",
			})
		case start != nil && target == nil:
			validation := ProjectDateValidation{
				Issue:     item.IssueNumber,
				Title:     item.IssueTitle,
				Status:    "missing_target_date",
				Severity:  "warn",
				StartDate: start,
				Reason:    "missing_required_field:target_date",
			}
			if due != nil {
				validation.Fallback = &ProjectDateFallback{Source: "milestone_due_date", Value: *due}
				validation.Reason = "missing_required_field:target_date;fallback:milestone_due_date"
			}
			validations = append(validations, validation)
		default:
			startDate, _ := time.Parse(time.DateOnly, *start)
			targetDate, _ := time.Parse(time.DateOnly, *target)
			if !targetDate.Before(startDate) {
				validations = append(validations, ProjectDateValidation{
					Issue:      item.IssueNumber,
					Title:      item.IssueTitle,
					Status:     "ok",
					Severity:   "info",
					StartDate:  start,
					TargetDate: target,
					Reason:     "date_validation_passed",
				})
				continue
			}
			if targetDate.Before(startDate) {
				validations = append(validations, ProjectDateValidation{
					Issue:      item.IssueNumber,
					Title:      item.IssueTitle,
					Status:     "invalid_date_range",
					Severity:   "block",
					StartDate:  start,
					TargetDate: target,
					Reason:     "target_date is before start_date",
				})
			}
		}
	}
	return validations, nil
}

func FormatProjectSyncPlan(report ProjectSyncReport) string {
	var b strings.Builder
	b.WriteString("project sync plan:\n")
	fmt.Fprintf(&b, "repo:             %s\n", report.Repo)
	fmt.Fprintf(&b, "project:          %s\n", report.Project)
	fmt.Fprintf(&b, "fields:           %d missing, %d mismatched, %d present\n", report.Counts.FieldsMissing, report.Counts.FieldsMismatch, report.Counts.FieldsPresent)
	fmt.Fprintf(&b, "dates:            %d warning, %d block\n", report.Counts.DateWarnings, report.Counts.DateBlocks)

	if report.MissingProject {
		fmt.Fprintf(&b, "  missing project: %s\n", report.Project)
	}
	for _, field := range report.Fields {
		if !field.Present {
			fmt.Fprintf(&b, "  missing field: %s (%s)\n", field.Name, strings.ToLower(field.Type))
			continue
		}
		if field.TypeMismatch {
			fmt.Fprintf(&b, "  wrong field type: %s (expected %s, got %s)\n", field.Name, strings.ToLower(field.Type), strings.ToLower(field.ActualType))
		}
	}
	for _, validation := range report.DateValidation {
		switch validation.Status {
		case "not_roadmapable":
			fmt.Fprintf(&b, "  skip issue #%d: not_roadmapable (%s)\n", validation.Issue, validation.Reason)
		case "missing_target_date":
			line := fmt.Sprintf("  warn issue #%d: missing_target_date", validation.Issue)
			if validation.Fallback != nil {
				line += fmt.Sprintf("; milestone due date %s available as reporting fallback", validation.Fallback.Value)
			}
			b.WriteString(line + "\n")
		case "invalid_date_range":
			if validation.TargetDate != nil && validation.StartDate != nil {
				fmt.Fprintf(&b, "  block issue #%d: target_date %s is before start_date %s\n", validation.Issue, *validation.TargetDate, *validation.StartDate)
			}
		case "missing_start_date":
			fmt.Fprintf(&b, "  warn issue #%d: missing_start_date\n", validation.Issue)
		case "missing_dates":
			if validation.Fallback != nil {
				fmt.Fprintf(&b, "  warn issue #%d: missing_dates; milestone due date %s available as reporting fallback\n", validation.Issue, validation.Fallback.Value)
			} else {
				fmt.Fprintf(&b, "  warn issue #%d: missing_dates\n", validation.Issue)
			}
		case "phase_due_date_fallback":
			if validation.Fallback != nil {
				fmt.Fprintf(&b, "  warn issue #%d: phase_due_date_fallback; milestone due date %s available as reporting fallback\n", validation.Issue, validation.Fallback.Value)
			}
		}
	}
	return b.String()
}

func normalizeDatePtr(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	normalized, ok := normalizeDate(*value)
	if !ok {
		return nil, fmt.Errorf("parse date %q: unsupported format", *value)
	}
	return &normalized, nil
}

func normalizeDate(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}
	if parsed, err := time.Parse(time.DateOnly, trimmed); err == nil {
		return parsed.Format(time.DateOnly), true
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed.UTC().Format(time.DateOnly), true
	}
	return "", false
}

func roadmapTypeLabel(labels []string) string {
	for _, label := range labels {
		switch label {
		case "type:epic", "type:story", "type:task", "type:spike", "type:bug":
			return label
		}
	}
	return ""
}

func isRoadmapable(item ProjectRoadmapItem) bool {
	switch item.TypeLabel {
	case "type:epic", "type:story":
		return true
	case "type:task", "type:spike", "type:bug":
		return item.StartDate != nil || item.TargetDate != nil || item.MilestoneDueDate != nil
	default:
		return false
	}
}
