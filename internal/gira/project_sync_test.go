package gira

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var projectSyncNowFixture = time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

func TestBuildProjectSyncReportMissingProject(t *testing.T) {
	report, err := BuildProjectSyncReport(
		"StatPan/gira",
		ProjectSyncSnapshot{},
		projectSyncNowFixture,
	)
	if err != nil {
		t.Fatalf("BuildProjectSyncReport returned error: %v", err)
	}
	if !report.MissingProject {
		t.Fatalf("MissingProject = false, want true")
	}
	if report.Counts.FieldsMissing != len(ProductOSCanonicalFields) {
		t.Fatalf("fields missing = %d, want %d", report.Counts.FieldsMissing, len(ProductOSCanonicalFields))
	}
	text := FormatProjectSyncPlan(report)
	if !strings.Contains(text, "fields:           6 missing, 0 mismatched, 0 present") {
		t.Fatalf("project sync text missing field counts:\n%s", text)
	}
	if !strings.Contains(text, "missing project: Product OS") {
		t.Fatalf("project sync text missing missing-project line:\n%s", text)
	}
}

func TestBuildProjectSyncReportMissingFields(t *testing.T) {
	report, err := BuildProjectSyncReport(
		"StatPan/gira",
		ProjectSyncSnapshot{
			ProjectName: "Product OS",
			FieldTypes: map[string]string{
				"Status":     "SINGLE_SELECT",
				"Start date": "DATE",
			},
		},
		projectSyncNowFixture,
	)
	if err != nil {
		t.Fatalf("BuildProjectSyncReport returned error: %v", err)
	}
	if report.MissingProject {
		t.Fatalf("MissingProject = true, want false")
	}
	if report.Counts.FieldsMissing == 0 {
		t.Fatalf("fields missing = 0, want > 0")
	}
	text := FormatProjectSyncPlan(report)
	if !strings.Contains(text, "fields:           4 missing, 0 mismatched, 2 present") {
		t.Fatalf("project sync text missing field counts:\n%s", text)
	}
	for _, want := range []string{"missing field: Priority", "missing field: Layer / workstream", "missing field: Owner / agent", "missing field: Target date"} {
		if !strings.Contains(text, want) {
			t.Fatalf("project sync text missing %q:\n%s", want, text)
		}
	}
}

func TestBuildProjectSyncReportWrongFieldType(t *testing.T) {
	report, err := BuildProjectSyncReport(
		"StatPan/gira",
		ProjectSyncSnapshot{
			ProjectName: "Product OS",
			FieldTypes: map[string]string{
				"Status":             "TEXT",
				"Priority":           "SINGLE_SELECT",
				"Layer / workstream": "SINGLE_SELECT",
				"Owner / agent":      "SINGLE_SELECT",
				"Start date":         "DATE",
				"Target date":        "DATE",
			},
		},
		projectSyncNowFixture,
	)
	if err != nil {
		t.Fatalf("BuildProjectSyncReport returned error: %v", err)
	}
	if report.Counts.FieldsMismatch != 1 {
		t.Fatalf("fields mismatch = %d, want 1", report.Counts.FieldsMismatch)
	}
	if report.Counts.FieldsPresent != 5 {
		t.Fatalf("fields present = %d, want 5", report.Counts.FieldsPresent)
	}
	text := FormatProjectSyncPlan(report)
	for _, want := range []string{
		"fields:           0 missing, 1 mismatched, 5 present",
		"wrong field type: Status (expected single_select, got text)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("project sync text missing %q:\n%s", want, text)
		}
	}
}

func TestProjectSyncJSONStable(t *testing.T) {
	report, err := BuildProjectSyncReport(
		"StatPan/gira",
		ProjectSyncSnapshot{
			ProjectName: "Product OS",
			FieldTypes: map[string]string{
				"Status":             "SINGLE_SELECT",
				"Priority":           "SINGLE_SELECT",
				"Layer / workstream": "SINGLE_SELECT",
				"Owner / agent":      "SINGLE_SELECT",
				"Start date":         "DATE",
				"Target date":        "DATE",
			},
		},
		projectSyncNowFixture,
	)
	if err != nil {
		t.Fatalf("BuildProjectSyncReport returned error: %v", err)
	}

	first, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent returned error: %v", err)
	}
	second, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent returned error: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("project sync JSON changed across identical marshals:\n%s\n---\n%s", first, second)
	}

	var payload map[string]any
	if err := json.Unmarshal(first, &payload); err != nil {
		t.Fatalf("project sync JSON did not parse: %v\n%s", err, first)
	}
	for _, key := range []string{"repo", "project", "command", "dry_run", "counts", "fields", "date_validation", "fetched_at"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("project sync JSON missing key %q:\n%s", key, first)
		}
	}
}

func TestBuildProjectSyncReportDateValidationWarningsAndBlocks(t *testing.T) {
	report, err := BuildProjectSyncReport(
		"StatPan/gira",
		ProjectSyncSnapshot{
			ProjectName: "Product OS",
			FieldTypes: map[string]string{
				"Status":             "SINGLE_SELECT",
				"Priority":           "SINGLE_SELECT",
				"Layer / workstream": "SINGLE_SELECT",
				"Owner / agent":      "SINGLE_SELECT",
				"Start date":         "DATE",
				"Target date":        "DATE",
			},
			RoadmapItems: []ProjectRoadmapItem{
				{IssueNumber: 42, IssueTitle: "Story A", TypeLabel: "type:story", Roadmapable: true, StartDate: stringPtr("2026-05-01"), TargetDate: nil, MilestoneDueDate: stringPtr("2026-06-30")},
				{IssueNumber: 51, IssueTitle: "Story B", TypeLabel: "type:story", Roadmapable: true, StartDate: stringPtr("2026-05-08"), TargetDate: stringPtr("2026-05-01")},
			},
		},
		projectSyncNowFixture,
	)
	if err != nil {
		t.Fatalf("BuildProjectSyncReport returned error: %v", err)
	}
	if report.Counts.DateWarnings != 1 {
		t.Fatalf("date warnings = %d, want 1", report.Counts.DateWarnings)
	}
	if report.Counts.DateBlocks != 1 {
		t.Fatalf("date blocks = %d, want 1", report.Counts.DateBlocks)
	}
	text := FormatProjectSyncPlan(report)
	for _, want := range []string{
		"warn issue #42: missing_target_date; milestone due date 2026-06-30 available as reporting fallback",
		"block issue #51: target_date 2026-05-01 is before start_date 2026-05-08",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("project sync text missing %q:\n%s", want, text)
		}
	}
}

func TestBuildProjectSyncReportSkipsNonRoadmapableTasks(t *testing.T) {
	report, err := BuildProjectSyncReport(
		"StatPan/gira",
		ProjectSyncSnapshot{
			ProjectName: "Product OS",
			FieldTypes: map[string]string{
				"Status":             "SINGLE_SELECT",
				"Priority":           "SINGLE_SELECT",
				"Layer / workstream": "SINGLE_SELECT",
				"Owner / agent":      "SINGLE_SELECT",
				"Start date":         "DATE",
				"Target date":        "DATE",
			},
			RoadmapItems: []ProjectRoadmapItem{
				{IssueNumber: 60, IssueTitle: "Tiny chore", TypeLabel: "type:task", Roadmapable: false},
			},
		},
		projectSyncNowFixture,
	)
	if err != nil {
		t.Fatalf("BuildProjectSyncReport returned error: %v", err)
	}
	if report.Counts.DateWarnings != 0 {
		t.Fatalf("date warnings = %d, want 0", report.Counts.DateWarnings)
	}
	if report.Counts.DateBlocks != 0 {
		t.Fatalf("date blocks = %d, want 0", report.Counts.DateBlocks)
	}
	if len(report.DateValidation) != 1 || report.DateValidation[0].Status != "not_roadmapable" {
		t.Fatalf("date validation = %#v, want single not_roadmapable entry", report.DateValidation)
	}
	text := FormatProjectSyncPlan(report)
	if !strings.Contains(text, "skip issue #60: not_roadmapable (type:task requires a meaningful delivery checkpoint or release marker)") {
		t.Fatalf("project sync text missing not_roadmapable skip:\n%s", text)
	}
}

func TestRoadmapabilityPolicy(t *testing.T) {
	cases := []struct {
		name string
		item ProjectRoadmapItem
		want bool
	}{
		{name: "epic always roadmapable", item: ProjectRoadmapItem{TypeLabel: "type:epic"}, want: true},
		{name: "story always roadmapable", item: ProjectRoadmapItem{TypeLabel: "type:story"}, want: true},
		{name: "task without checkpoint excluded", item: ProjectRoadmapItem{TypeLabel: "type:task"}, want: false},
		{name: "task with start date included", item: ProjectRoadmapItem{TypeLabel: "type:task", StartDate: stringPtr("2026-05-01")}, want: true},
		{name: "bug without release marker excluded", item: ProjectRoadmapItem{TypeLabel: "type:bug"}, want: false},
		{name: "bug with milestone due date included", item: ProjectRoadmapItem{TypeLabel: "type:bug", MilestoneDueDate: stringPtr("2026-06-30")}, want: true},
		{name: "spike without date excluded", item: ProjectRoadmapItem{TypeLabel: "type:spike"}, want: false},
		{name: "spike with target date included", item: ProjectRoadmapItem{TypeLabel: "type:spike", TargetDate: stringPtr("2026-05-07")}, want: true},
		{name: "untyped issue excluded", item: ProjectRoadmapItem{}, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRoadmapable(tc.item); got != tc.want {
				t.Fatalf("isRoadmapable(%+v) = %v, want %v", tc.item, got, tc.want)
			}
		})
	}
}
