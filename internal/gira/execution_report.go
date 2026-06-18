package gira

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const ExecutionReportSchemaVersion = "execution-report/v1alpha1"

var executionReportCSVHeaders = []string{
	"phase",
	"workstream",
	"task",
	"owner",
	"start_date",
	"due_date",
	"status",
	"priority",
	"dependency",
	"milestone",
	"issue_url",
	"issue",
	"item_type",
	"row_type",
	"week",
	"source_due_date",
	"scenario_due_date",
	"delta_days",
	"diagnostics",
}

type ExecutionReportOptions struct {
	Command  string
	Mode     string
	By       string
	Scenario string
}

type ExecutionReport struct {
	Command          string                    `json:"command"`
	SchemaVersion    string                    `json:"schema_version"`
	Repo             string                    `json:"repo"`
	StateFilter      string                    `json:"state_filter"`
	Mode             string                    `json:"mode"`
	By               string                    `json:"by,omitempty"`
	Scenario         string                    `json:"scenario,omitempty"`
	GeneratedAt      string                    `json:"generated_at"`
	Rows             []ExecutionReportRow      `json:"rows"`
	Counts           ExecutionReportCounts     `json:"counts"`
	Diagnostics      []ExecutionDiagnostic     `json:"diagnostics,omitempty"`
	MilestoneCleanup []WBSMilestoneCleanupItem `json:"milestone_cleanup,omitempty"`
	Sources          []WBSReportSource         `json:"sources"`
}

type ExecutionReportRow struct {
	Phase           string   `json:"phase"`
	Workstream      string   `json:"workstream"`
	Task            string   `json:"task"`
	Owner           string   `json:"owner,omitempty"`
	StartDate       string   `json:"start_date,omitempty"`
	DueDate         string   `json:"due_date,omitempty"`
	Status          string   `json:"status,omitempty"`
	Priority        string   `json:"priority,omitempty"`
	Dependency      string   `json:"dependency,omitempty"`
	Milestone       string   `json:"milestone,omitempty"`
	IssueURL        string   `json:"issue_url,omitempty"`
	Issue           int      `json:"issue,omitempty"`
	ItemType        string   `json:"item_type"`
	RowType         string   `json:"row_type"`
	WBSID           string   `json:"wbs_id,omitempty"`
	ParentID        string   `json:"parent_id,omitempty"`
	Week            string   `json:"week,omitempty"`
	SourceDueDate   string   `json:"source_due_date,omitempty"`
	ScenarioDueDate string   `json:"scenario_due_date,omitempty"`
	DeltaDays       int      `json:"delta_days,omitempty"`
	Diagnostics     []string `json:"diagnostics,omitempty"`
}

type ExecutionReportCounts struct {
	Rows              int `json:"rows"`
	ActionableRows    int `json:"actionable_rows"`
	SummaryRows       int `json:"summary_rows"`
	MissingOwner      int `json:"missing_owner"`
	MissingDate       int `json:"missing_date"`
	MissingParent     int `json:"missing_parent"`
	MissingDependency int `json:"missing_dependency"`
	CleanupItems      int `json:"cleanup_items"`
}

type ExecutionDiagnostic struct {
	Code        string `json:"code"`
	Severity    string `json:"severity"`
	Issue       int    `json:"issue,omitempty"`
	Task        string `json:"task,omitempty"`
	Milestone   string `json:"milestone,omitempty"`
	Description string `json:"description"`
}

func BuildExecutionReportFromWBS(wbs WBSReport, options ExecutionReportOptions) (ExecutionReport, error) {
	mode := strings.ToLower(strings.TrimSpace(options.Mode))
	if mode == "" {
		mode = "execution"
	}
	if mode != "execution" && mode != "schedule" {
		return ExecutionReport{}, fmt.Errorf("--mode must be execution or schedule")
	}
	by := strings.ToLower(strings.TrimSpace(options.By))
	if by == "" && mode == "schedule" {
		by = "week"
	}
	if by != "" && by != "week" {
		return ExecutionReport{}, fmt.Errorf("--by must be week")
	}
	scenario, err := normalizeExecutionScenario(options.Scenario)
	if err != nil {
		return ExecutionReport{}, err
	}
	command := strings.TrimSpace(options.Command)
	if command == "" {
		command = "report wbs"
	}
	report := ExecutionReport{
		Command:          command,
		SchemaVersion:    ExecutionReportSchemaVersion,
		Repo:             wbs.Repo,
		StateFilter:      wbs.StateFilter,
		Mode:             mode,
		By:               by,
		Scenario:         scenario,
		GeneratedAt:      wbs.GeneratedAt,
		MilestoneCleanup: wbs.MilestoneCleanup,
		Sources: append([]WBSReportSource{
			{Name: "wbs_report", SchemaVersion: wbs.SchemaVersion},
		}, wbs.Sources...),
	}
	for _, item := range wbs.Items {
		row := executionRowFromWBSItem(item)
		report.Rows = append(report.Rows, row)
		report.Diagnostics = append(report.Diagnostics, executionDiagnosticsForRow(row)...)
	}
	applyExecutionScenario(&report)
	sortExecutionRows(report.Rows, mode)
	report.Counts = countExecutionRows(report.Rows, report.MilestoneCleanup)
	return report, nil
}

func normalizeExecutionScenario(scenario string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(scenario))
	switch trimmed {
	case "", "current", "none":
		return "", nil
	case "one-month":
		return "one-month", nil
	default:
		return "", fmt.Errorf("--scenario must be current, none, or one-month")
	}
}

func executionRowFromWBSItem(item WBSReportItem) ExecutionReportRow {
	rowType := "actionable"
	if !wbsItemIsExecutable(item) {
		rowType = "summary"
	}
	phase := strings.TrimSpace(item.Milestone)
	if phase == "" {
		phase = "Unscheduled"
	}
	workstream := strings.TrimSpace(item.Workstream)
	if workstream == "" {
		workstream = strings.TrimSpace(item.Milestone)
	}
	if workstream == "" && item.ParentID != "" {
		workstream = "linked-work"
	}
	if workstream == "" {
		workstream = strings.TrimSpace(item.Kind)
	}
	sourceDueDate := strings.TrimSpace(item.TargetDate)
	return ExecutionReportRow{
		Phase:         phase,
		Workstream:    workstream,
		Task:          item.Title,
		Owner:         item.Owner,
		StartDate:     item.StartDate,
		DueDate:       sourceDueDate,
		Status:        item.Status,
		Priority:      item.Priority,
		Dependency:    item.Dependency,
		Milestone:     item.Milestone,
		IssueURL:      item.URL,
		Issue:         item.Issue,
		ItemType:      item.Kind,
		RowType:       rowType,
		WBSID:         item.WBSID,
		ParentID:      item.ParentID,
		Week:          executionWeekBucket(sourceDueDate),
		SourceDueDate: sourceDueDate,
		Diagnostics:   executionDiagnosticCodes(item, rowType),
	}
}

func executionDiagnosticCodes(item WBSReportItem, rowType string) []string {
	if rowType != "actionable" {
		return nil
	}
	codes := []string{}
	if strings.TrimSpace(item.Owner) == "" {
		codes = append(codes, "missing_owner")
	}
	if strings.TrimSpace(item.TargetDate) == "" {
		codes = append(codes, "missing_date")
	}
	if strings.TrimSpace(item.ParentID) == "" {
		codes = append(codes, "missing_parent")
	}
	if strings.TrimSpace(item.Dependency) == "" {
		codes = append(codes, "missing_dependency")
	}
	return codes
}

func executionDiagnosticsForRow(row ExecutionReportRow) []ExecutionDiagnostic {
	out := []ExecutionDiagnostic{}
	for _, code := range row.Diagnostics {
		out = append(out, ExecutionDiagnostic{
			Code:        code,
			Severity:    "warning",
			Issue:       row.Issue,
			Task:        row.Task,
			Milestone:   row.Milestone,
			Description: executionDiagnosticDescription(code),
		})
	}
	return out
}

func executionDiagnosticDescription(code string) string {
	switch code {
	case "missing_owner":
		return "Actionable row has no owner."
	case "missing_date":
		return "Actionable row has no due date from Project target date or milestone due date."
	case "missing_parent":
		return "Actionable row is not linked to a parent epic or summary container."
	case "missing_dependency":
		return "Actionable row has no dependency metadata."
	default:
		return code
	}
}

func applyExecutionScenario(report *ExecutionReport) {
	if report.Scenario != "one-month" {
		return
	}
	generatedAt, err := time.Parse(time.RFC3339, report.GeneratedAt)
	if err != nil {
		generatedAt = time.Now().UTC()
	}
	actionable := []*ExecutionReportRow{}
	for i := range report.Rows {
		if report.Rows[i].RowType == "actionable" {
			actionable = append(actionable, &report.Rows[i])
		}
	}
	sort.Slice(actionable, func(i, j int) bool {
		if actionable[i].SourceDueDate != actionable[j].SourceDueDate {
			return actionable[i].SourceDueDate < actionable[j].SourceDueDate
		}
		if actionable[i].Priority != actionable[j].Priority {
			return actionable[i].Priority < actionable[j].Priority
		}
		return actionable[i].Issue < actionable[j].Issue
	})
	step := 1
	if len(actionable) > 1 {
		step = 30 / len(actionable)
		if step < 1 {
			step = 1
		}
	}
	for i, row := range actionable {
		scenarioDate := generatedAt.AddDate(0, 0, i*step).Format("2006-01-02")
		row.ScenarioDueDate = scenarioDate
		row.DueDate = scenarioDate
		row.Week = executionWeekBucket(scenarioDate)
		if source, ok := parseExecutionDate(row.SourceDueDate); ok {
			if scenario, ok := parseExecutionDate(scenarioDate); ok {
				row.DeltaDays = int(scenario.Sub(source).Hours() / 24)
			}
		}
	}
}

func sortExecutionRows(rows []ExecutionReportRow, mode string) {
	sort.SliceStable(rows, func(i, j int) bool {
		if mode == "schedule" {
			if rows[i].DueDate != rows[j].DueDate {
				if rows[i].DueDate == "" {
					return false
				}
				if rows[j].DueDate == "" {
					return true
				}
				return rows[i].DueDate < rows[j].DueDate
			}
			if rows[i].Week != rows[j].Week {
				return rows[i].Week < rows[j].Week
			}
		}
		if rows[i].RowType != rows[j].RowType {
			return rows[i].RowType == "actionable"
		}
		if rows[i].WBSID != rows[j].WBSID {
			return rows[i].WBSID < rows[j].WBSID
		}
		return rows[i].Issue < rows[j].Issue
	})
}

func countExecutionRows(rows []ExecutionReportRow, cleanup []WBSMilestoneCleanupItem) ExecutionReportCounts {
	counts := ExecutionReportCounts{Rows: len(rows), CleanupItems: len(cleanup)}
	for _, row := range rows {
		if row.RowType == "summary" {
			counts.SummaryRows++
		} else {
			counts.ActionableRows++
		}
		for _, code := range row.Diagnostics {
			switch code {
			case "missing_owner":
				counts.MissingOwner++
			case "missing_date":
				counts.MissingDate++
			case "missing_parent":
				counts.MissingParent++
			case "missing_dependency":
				counts.MissingDependency++
			}
		}
	}
	return counts
}

func RenderExecutionReportCSV(report ExecutionReport) ([]byte, error) {
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	if err := writer.Write(executionReportCSVHeaders); err != nil {
		return nil, err
	}
	for _, row := range report.Rows {
		record := []string{
			row.Phase,
			row.Workstream,
			row.Task,
			row.Owner,
			row.StartDate,
			row.DueDate,
			row.Status,
			row.Priority,
			row.Dependency,
			row.Milestone,
			row.IssueURL,
			wbsIssueNumber(row.Issue),
			row.ItemType,
			row.RowType,
			row.Week,
			row.SourceDueDate,
			row.ScenarioDueDate,
			executionDeltaDays(row.DeltaDays),
			strings.Join(row.Diagnostics, ";"),
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return buffer.Bytes(), writer.Error()
}

func RenderExecutionReportJSON(report ExecutionReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

func FormatExecutionReport(report ExecutionReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s rows=%d actionable=%d summary=%d cleanup=%d\n", report.Command, report.Repo, report.Counts.Rows, report.Counts.ActionableRows, report.Counts.SummaryRows, report.Counts.CleanupItems)
	if report.Scenario != "" {
		fmt.Fprintf(&b, "scenario: %s\n", report.Scenario)
	}
	for _, row := range report.Rows {
		due := ""
		if row.DueDate != "" {
			due = " due=" + row.DueDate
		}
		fmt.Fprintf(&b, "%s #%d %s [%s]%s\n", row.RowType, row.Issue, row.Task, row.Status, due)
	}
	if len(report.MilestoneCleanup) > 0 {
		b.WriteString("milestone cleanup:\n")
		for _, item := range report.MilestoneCleanup {
			fmt.Fprintf(&b, "- %s: %s total=%d executable=%d\n", item.Milestone, item.Reason, item.TotalItems, item.ExecutableItems)
		}
	}
	return b.String()
}

func WriteExecutionReportCSV(path string, report ExecutionReport) error {
	csvBytes, err := RenderExecutionReportCSV(report)
	if err != nil {
		return err
	}
	return writeSafeLocalFile(path, csvBytes, 0o644)
}

func executionWeekBucket(dateValue string) string {
	date, ok := parseExecutionDate(dateValue)
	if !ok {
		return ""
	}
	weekday := int(date.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	monday := date.AddDate(0, 0, -(weekday - 1))
	return monday.Format("2006-01-02")
}

func parseExecutionDate(dateValue string) (time.Time, bool) {
	trimmed := strings.TrimSpace(dateValue)
	if trimmed == "" {
		return time.Time{}, false
	}
	if normalized, ok := normalizeDate(trimmed); ok {
		trimmed = normalized
	}
	date, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return time.Time{}, false
	}
	return date, true
}

func executionDeltaDays(delta int) string {
	if delta == 0 {
		return ""
	}
	return strconv.Itoa(delta)
}
