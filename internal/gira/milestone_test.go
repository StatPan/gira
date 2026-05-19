package gira

import (
	"strings"
	"testing"
)

func TestBuildMilestoneNewReportDryRunAndApply(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	listKey := "gh api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100"
	runner := onboardFakeRunner{responses: map[string]string{
		listKey: `[]`,
		"gh api repos/StatPan/gira/milestones -X POST -f title=2.0 Alpha -f description=State runtime -f due_on=2026-06-01T23:59:59Z": `{"number":12,"title":"2.0 Alpha","state":"open","description":"State runtime","due_on":"2026-06-01T23:59:59Z","open_issues":0,"closed_issues":0}`,
	}, errors: map[string]error{}}

	dry, err := BuildMilestoneNewReport(MilestoneNewInput{Repo: repo, Title: "2.0 Alpha", Description: "State runtime", DueOn: "2026-06-01", DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildMilestoneNewReport dry-run error: %v", err)
	}
	if dry.Milestone == nil || dry.Milestone.Title != "2.0 Alpha" || dry.Actions[0].Status != "planned" {
		t.Fatalf("unexpected dry-run report: %+v", dry)
	}
	if !strings.Contains(dry.NextStep, "gira milestone new") || !strings.Contains(dry.NextStep, "--apply") {
		t.Fatalf("unexpected dry-run next step: %s", dry.NextStep)
	}

	applied, err := BuildMilestoneNewReport(MilestoneNewInput{Repo: repo, Title: "2.0 Alpha", Description: "State runtime", DueOn: "2026-06-01", Apply: true}, runner)
	if err != nil {
		t.Fatalf("BuildMilestoneNewReport apply error: %v", err)
	}
	if applied.Milestone == nil || applied.Milestone.Number != 12 || applied.Actions[0].Status != "applied" {
		t.Fatalf("unexpected apply report: %+v", applied)
	}
}

func TestBuildMilestoneListReportFiltersState(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := onboardFakeRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100": `[[{"number":1,"title":"Open","state":"open","description":"","due_on":null,"open_issues":1,"closed_issues":1},{"number":2,"title":"Closed","state":"closed","description":"","due_on":null,"open_issues":0,"closed_issues":2}]]`,
	}, errors: map[string]error{}}

	report, err := BuildMilestoneListReport(MilestoneListOptions{Repo: repo, State: "open"}, runner)
	if err != nil {
		t.Fatalf("BuildMilestoneListReport error: %v", err)
	}
	if report.Counts.Milestones != 1 || report.Milestones[0].Title != "Open" || report.Milestones[0].ProgressPercent != 50 {
		t.Fatalf("unexpected list report: %+v", report)
	}
}

func TestBuildMilestoneStatusReportCountsChildTickets(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := onboardFakeRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100":                              `[[{"number":7,"title":"2.0 Alpha","state":"open","description":"","due_on":null,"open_issues":2,"closed_issues":1}]]`,
		"gh issue list --repo StatPan/gira --state all --limit 1000 --json number,title,state,labels,milestone,updatedAt,url,body": `[{"number":10,"title":"Ready","state":"OPEN","labels":[{"name":"status:ready"},{"name":"type:task"}],"milestone":{"title":"2.0 Alpha"},"updatedAt":"2026-05-19T00:00:00Z","url":"u10"},{"number":11,"title":"Review","state":"OPEN","labels":[{"name":"status:in-review"},{"name":"type:task"}],"milestone":{"title":"2.0 Alpha"},"updatedAt":"2026-05-19T00:00:00Z","url":"u11"},{"number":12,"title":"Done","state":"CLOSED","labels":[{"name":"status:done"},{"name":"type:task"}],"milestone":{"title":"2.0 Alpha"},"updatedAt":"2026-05-19T00:00:00Z","url":"u12"}]`,
	}, errors: map[string]error{}}

	report, err := BuildMilestoneStatusReport(MilestoneStatusOptions{Repo: repo, Milestone: "2.0 Alpha"}, runner)
	if err != nil {
		t.Fatalf("BuildMilestoneStatusReport error: %v", err)
	}
	if report.Counts.Tickets != 3 || report.Counts.Ready != 1 || report.Counts.InReview != 1 || report.Counts.Done != 1 || report.Counts.FinishReady != 1 {
		t.Fatalf("unexpected counts: %+v", report.Counts)
	}
	if !strings.Contains(report.NextStep, "review") {
		t.Fatalf("unexpected next step: %s", report.NextStep)
	}
}

func TestBuildMilestoneAssignReportDryRunAndApply(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := onboardFakeRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100": `[[{"number":7,"title":"2.0 Alpha","state":"open","description":"","due_on":null,"open_issues":0,"closed_issues":0}]]`,
		"gh issue edit 10 --repo StatPan/gira --milestone 2.0 Alpha":                                  `{}`,
	}, errors: map[string]error{}}

	dry, err := BuildMilestoneAssignReport(MilestoneAssignInput{Repo: repo, Milestone: "2.0 Alpha", Tickets: []int{10}, DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildMilestoneAssignReport dry-run error: %v", err)
	}
	if dry.Counts.WouldAssign != 1 || dry.Actions[0].Status != "planned" {
		t.Fatalf("unexpected dry-run report: %+v", dry)
	}

	applied, err := BuildMilestoneAssignReport(MilestoneAssignInput{Repo: repo, Milestone: "2.0 Alpha", Tickets: []int{10}, Apply: true}, runner)
	if err != nil {
		t.Fatalf("BuildMilestoneAssignReport apply error: %v", err)
	}
	if applied.Counts.AppliedAssign != 1 || applied.Actions[0].Status != "applied" {
		t.Fatalf("unexpected apply report: %+v", applied)
	}
}

func TestBuildMilestonePlanReportSelectsReadyTickets(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := onboardFakeRunner{responses: map[string]string{
		"gh issue list --repo StatPan/gira --state open --limit 20 --json number,title,state,labels,assignees,milestone,url --label status:ready": `[{"number":10,"title":"Ready","state":"OPEN","labels":[{"name":"status:ready"},{"name":"type:task"}],"assignees":[],"milestone":null,"url":"u10"}]`,
		"gh api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100":                                             `[[{"number":7,"title":"2.0 Alpha","state":"open","description":"","due_on":null,"open_issues":0,"closed_issues":0}]]`,
	}, errors: map[string]error{}}

	report, err := BuildMilestonePlanReport(MilestonePlanInput{Repo: repo, Milestone: "2.0 Alpha", DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildMilestonePlanReport error: %v", err)
	}
	if report.Filters.Labels[0] != "status:ready" || report.Counts.WouldAssign != 1 || report.Actions[0].Issue != 10 {
		t.Fatalf("unexpected plan report: %+v", report)
	}
}

func TestResolveMilestoneAmbiguous(t *testing.T) {
	_, err := resolveMilestone([]normalizedMilestone{{Number: 1, Title: "Roadmap"}, {Number: 2, Title: "roadmap"}}, "roadmap")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") || !strings.Contains(err.Error(), "#1") || !strings.Contains(err.Error(), "#2") {
		t.Fatalf("unexpected ambiguous error: %v", err)
	}
}
