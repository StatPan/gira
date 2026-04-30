package gira

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestBuildOnboardVerifyReportSteadyStateReady(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := onboardFakeRunner{responses: map[string]string{
		"gh --version": `gh version 2.0.0`,
		"gh repo view StatPan/gira --json nameWithOwner,viewerPermission,defaultBranchRef": `{"nameWithOwner":"StatPan/gira","viewerPermission":"ADMIN","defaultBranchRef":{"name":"main"}}`,
		"gh label list --repo StatPan/gira --json name,color,description --limit 1000":                  desiredLabelsJSON(),
		"gh api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100":  `[[{"number":1,"title":"MVP","description":"CLI-first Gira bootstrapper with templates and GitHub metadata sync.","due_on":null,"state":"open","open_issues":1,"closed_issues":0},{"number":2,"title":"Beta","description":"Broader validation and hardening after the MVP workflow is usable.","due_on":null,"state":"open","open_issues":0,"closed_issues":0},{"number":3,"title":"v1","description":"Stable first release of the GitHub-native project OS workflow.","due_on":null,"state":"open","open_issues":0,"closed_issues":0}]]`,
		"gh issue list --repo StatPan/gira --state all --label gira:bootstrap --json number,title,labels --limit 1000": desiredBootstrapIssuesJSON(),
		"gh api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100":      `[[{"number":71,"title":"Onboarding","state":"open","labels":[{"name":"status:ready"}],"milestone":{"title":"MVP"},"updated_at":"2026-04-26T12:00:00Z","html_url":"https://github.com/StatPan/gira/issues/71"}]]`,
	}}

	report := BuildOnboardVerifyReport(repo, OnboardStageSteadyState, runner, time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC))
	if !report.Ready {
		t.Fatalf("report ready = false, want true: %+v", report.BlockingChecklist)
	}
	if len(report.Stages) != 4 {
		t.Fatalf("stage count = %d, want 4", len(report.Stages))
	}
	if len(report.BlockingChecklist) != 0 {
		t.Fatalf("blocking checklist = %v, want empty", report.BlockingChecklist)
	}
	text := FormatOnboardVerifyReport(report)
	for _, want := range []string{"onboard verify: READY", "stage init: ready", "stage steady-state: ready"} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted report missing %q:\n%s", want, text)
		}
	}
}

func TestBuildOnboardVerifyReportFailsClosedWithRemediation(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := onboardFakeRunner{responses: map[string]string{
		"gh --version": `gh version 2.0.0`,
		"gh repo view StatPan/gira --json nameWithOwner,viewerPermission,defaultBranchRef": `{"nameWithOwner":"StatPan/gira","viewerPermission":"WRITE","defaultBranchRef":{"name":"main"}}`,
		"gh label list --repo StatPan/gira --json name,color,description --limit 1000": desiredLabelsJSON(),
		"gh api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100": `[[{"number":1,"title":"MVP","description":"CLI-first Gira bootstrapper with templates and GitHub metadata sync.","due_on":null,"state":"open","open_issues":1,"closed_issues":0},{"number":2,"title":"Beta","description":"Broader validation and hardening after the MVP workflow is usable.","due_on":null,"state":"open","open_issues":0,"closed_issues":0},{"number":3,"title":"v1","description":"Stable first release of the GitHub-native project OS workflow.","due_on":null,"state":"open","open_issues":0,"closed_issues":0}]]`,
		"gh issue list --repo StatPan/gira --state all --label gira:bootstrap --json number,title,labels --limit 1000": `[]`,
	}}

	report := BuildOnboardVerifyReport(repo, OnboardStageBootstrap, runner, time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC))
	if report.Ready {
		t.Fatal("report ready = true, want false")
	}
	if len(report.BlockingChecklist) == 0 {
		t.Fatal("blocking checklist empty, want failed checks")
	}
	failed := report.BlockingChecklist[0]
	if failed.ID != "bootstrap_operating_objects_ready" {
		t.Fatalf("failed check id = %s, want bootstrap_operating_objects_ready", failed.ID)
	}
	if !strings.Contains(failed.Remediation, "--bootstrap-issues") {
		t.Fatalf("missing remediation: %+v", failed)
	}
}

func desiredBootstrapIssuesJSON() string {
	parts := make([]string, 0, len(DesiredBootstrapIssues))
	for idx, issue := range DesiredBootstrapIssues {
		parts = append(parts, fmt.Sprintf(`{"number":%d,"title":%q,"labels":[{"name":"gira:bootstrap"}]}`, idx+1, issue.Title))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func desiredLabelsJSON() string {
	parts := make([]string, 0, len(DesiredLabels))
	for _, label := range DesiredLabels {
		parts = append(parts, fmt.Sprintf(`{"name":%q,"color":%q,"description":%q}`, label.Name, label.Color, label.Description))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

type onboardFakeRunner struct {
	responses map[string]string
	errors    map[string]error
}

func (r onboardFakeRunner) Run(name string, args ...string) ([]byte, error) {
	key := strings.TrimSpace(name + " " + strings.Join(args, " "))
	if err, ok := r.errors[key]; ok {
		return nil, err
	}
	response, ok := r.responses[key]
	if !ok {
		return nil, fmt.Errorf("unexpected command: %s", key)
	}
	return []byte(response), nil
}
