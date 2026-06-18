package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildWBSReportLinksEpicChildrenAndRendersCSV(t *testing.T) {
	client := &fakeWBSReportClient{
		issues: []WBSRawIssue{
			{IssueNumber: 10, Title: "Admin workflows", State: "open", Body: "Tracks #11", Labels: []string{"type:epic", "status:ready", "priority:p1"}, Milestone: "M1", URL: "u10"},
			{IssueNumber: 11, Title: "Create admin list", State: "closed", Labels: []string{"type:task", "status:done"}, Milestone: "M1", URL: "u11"},
			{IssueNumber: 12, Title: "Unlinked docs", State: "open", Labels: []string{"type:task", "status:ready"}, URL: "u12"},
		},
		milestones: []DashboardRawMilestone{
			{MilestoneNumber: 1, Title: "M1", State: "open", DueOn: wbsPtr("2026-07-01T00:00:00Z")},
		},
		projectSnapshot: ProjectSyncSnapshot{
			RoadmapItems: []ProjectRoadmapItem{
				{IssueNumber: 10, IssueTitle: "Admin workflows", TypeLabel: "type:epic", StartDate: wbsPtr("2026-06-20"), TargetDate: wbsPtr("2026-06-30"), Roadmapable: true},
			},
		},
	}
	report, err := BuildWBSReport(ParseRepoRefMust("StatPan/gira"), client, time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildWBSReport returned error: %v", err)
	}
	if report.SchemaVersion != WBSReportSchemaVersion || report.Counts.Epics != 1 || report.Counts.LinkedIssues != 1 || report.Counts.UnlinkedItems != 1 {
		t.Fatalf("unexpected report metadata: %+v", report)
	}
	if len(report.Items) != 3 {
		t.Fatalf("items = %d, want 3: %+v", len(report.Items), report.Items)
	}
	if report.Items[0].WBSID != "1" || report.Items[0].Progress != 100 || report.Items[0].StartDate != "2026-06-20" {
		t.Fatalf("unexpected epic item: %+v", report.Items[0])
	}
	if report.Items[1].WBSID != "1.1" || report.Items[1].ParentID != "1" || report.Items[1].Source != "body,milestone" || report.Items[1].TargetDate != "2026-07-01" {
		t.Fatalf("unexpected child item: %+v", report.Items[1])
	}
	csvBytes, err := RenderWBSReportCSV(report)
	if err != nil {
		t.Fatalf("RenderWBSReportCSV returned error: %v", err)
	}
	csvText := string(csvBytes)
	if !strings.HasPrefix(csvText, "wbs_id,parent_id,level,kind,repo,issue,title,state,status,priority,owner,milestone,start_date,target_date,progress,children,source,url\n") {
		t.Fatalf("unexpected CSV header:\n%s", csvText)
	}
	if !strings.Contains(csvText, "1.1,1,2,task,StatPan/gira,11,Create admin list") {
		t.Fatalf("CSV missing linked child row:\n%s", csvText)
	}
}

func TestWriteWBSReportBundleWritesHumanAndMachineArtifacts(t *testing.T) {
	report := WBSReport{
		Command:       "report wbs",
		SchemaVersion: WBSReportSchemaVersion,
		Repo:          "StatPan/gira",
		GeneratedAt:   "2026-06-18T09:00:00Z",
		Items: []WBSReportItem{
			{WBSID: "1", Level: 1, Kind: "epic", Repo: "StatPan/gira", Issue: 10, Title: "Admin workflows", Status: "Ready", Progress: 50},
		},
	}
	outputRoot := filepath.Join(t.TempDir(), "wbs")
	if err := WriteWBSReportBundle(outputRoot, report); err != nil {
		t.Fatalf("WriteWBSReportBundle returned error: %v", err)
	}
	for _, rel := range []string{"index.html", "csv/wbs_items.csv", "derived/wbs_tree.json"} {
		if _, err := os.Stat(filepath.Join(outputRoot, rel)); err != nil {
			t.Fatalf("expected artifact %s: %v", rel, err)
		}
	}
}

type fakeWBSReportClient struct {
	issues          []WBSRawIssue
	milestones      []DashboardRawMilestone
	projectSnapshot ProjectSyncSnapshot
}

func (c *fakeWBSReportClient) FetchIssues() ([]WBSRawIssue, error) {
	return c.issues, nil
}

func (c *fakeWBSReportClient) FetchMilestones() ([]DashboardRawMilestone, error) {
	return c.milestones, nil
}

func (c *fakeWBSReportClient) FetchProjectSnapshot() (ProjectSyncSnapshot, error) {
	return c.projectSnapshot, nil
}

func wbsPtr(value string) *string {
	return &value
}
