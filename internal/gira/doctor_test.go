package gira

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestBuildDoctorReportReady(t *testing.T) {
	report := BuildDoctorReport("StatPan/gira", readyDoctorRunner(), time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))

	if !report.Ready {
		t.Fatalf("ready = false, want true: %+v", report.Checks)
	}
	for _, id := range []string{"gira_cli_visible", "gh_available", "repo_context", "gh_auth", "repo_access", "metadata_drift", "onboard_readiness", "local_git_state"} {
		check := doctorCheckByID(report, id)
		if check == nil {
			t.Fatalf("missing check %s: %+v", id, report.Checks)
		}
		if check.Status != DoctorCheckPass {
			t.Fatalf("check %s status = %s, want pass: %+v", id, check.Status, *check)
		}
	}
	cliCheck := doctorCheckByID(report, "gira_cli_visible")
	if !strings.Contains(cliCheck.Detail, "executable=") {
		t.Fatalf("gira_cli_visible detail = %q, want executable detail", cliCheck.Detail)
	}
}

func TestBuildDoctorReportMissingGhFailsAndSkipsGhDependentChecks(t *testing.T) {
	runner := onboardFakeRunner{
		errors: map[string]error{
			"gh --version": fmt.Errorf("executable file not found"),
		},
		responses: map[string]string{
			"git rev-parse --is-inside-work-tree": "true",
			"git branch --show-current":           "main",
			"git status --porcelain":              "",
		},
	}
	report := BuildDoctorReport("StatPan/gira", runner, time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))

	if report.Ready {
		t.Fatal("ready = true, want false")
	}
	if got := doctorCheckByID(report, "gh_available"); got == nil || got.Status != DoctorCheckFail {
		t.Fatalf("gh_available = %+v, want fail", got)
	}
	if got := doctorCheckByID(report, "metadata_drift"); got == nil || got.Status != DoctorCheckSkip {
		t.Fatalf("metadata_drift = %+v, want skip", got)
	}
}

func TestBuildDoctorReportAuthAndRepoFailure(t *testing.T) {
	runner := readyDoctorRunner()
	runner.errors["gh auth status"] = fmt.Errorf("not logged in")
	runner.errors["gh repo view StatPan/gira --json nameWithOwner,viewerPermission,defaultBranchRef"] = fmt.Errorf("HTTP 404")
	report := BuildDoctorReport("StatPan/gira", runner, time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))

	if report.Ready {
		t.Fatal("ready = true, want false")
	}
	if got := doctorCheckByID(report, "gh_auth"); got == nil || got.Status != DoctorCheckFail {
		t.Fatalf("gh_auth = %+v, want fail", got)
	}
	if got := doctorCheckByID(report, "repo_access"); got == nil || got.Status != DoctorCheckFail {
		t.Fatalf("repo_access = %+v, want fail", got)
	}
}

func TestBuildDoctorReportDriftFailureHasSyncRemediation(t *testing.T) {
	runner := readyDoctorRunner()
	runner.responses["gh label list --repo StatPan/gira --json name,color,description --limit 1000"] = `[]`
	report := BuildDoctorReport("StatPan/gira", runner, time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))

	if report.Ready {
		t.Fatal("ready = true, want false")
	}
	check := doctorCheckByID(report, "metadata_drift")
	if check == nil {
		t.Fatal("metadata_drift check missing")
	}
	if check.Status != DoctorCheckFail {
		t.Fatalf("metadata_drift status = %s, want fail", check.Status)
	}
	for _, want := range []string{
		"`gira ops sync --repo StatPan/gira --dry-run`",
		"`gira ops sync --repo StatPan/gira`",
		"labels create=",
	} {
		if !strings.Contains(check.Remediation+check.Detail, want) {
			t.Fatalf("metadata_drift missing %q: %+v", want, *check)
		}
	}
}

func TestBuildDoctorReportBootstrapIssueDriftWarnsOnly(t *testing.T) {
	runner := readyDoctorRunner()
	runner.responses["gh issue list --repo StatPan/gira --state all --label gira:bootstrap --json number,title,labels --limit 1000"] = `[]`
	report := BuildDoctorReport("StatPan/gira", runner, time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))

	if !report.Ready {
		t.Fatalf("ready = false, want true for optional bootstrap issue drift: %+v", report.Checks)
	}
	check := doctorCheckByID(report, "metadata_drift")
	if check == nil {
		t.Fatal("metadata_drift check missing")
	}
	if check.Status != DoctorCheckWarn {
		t.Fatalf("metadata_drift status = %s, want warn: %+v", check.Status, *check)
	}
	if !strings.Contains(check.Detail, "optional bootstrap issues create=5") {
		t.Fatalf("metadata_drift detail = %q, want optional bootstrap count", check.Detail)
	}
	if !strings.Contains(check.Remediation, "only when you want Gira sample bootstrap issues") {
		t.Fatalf("metadata_drift remediation = %q, want opt-in guidance", check.Remediation)
	}
}

func TestBuildDoctorReportDetachedHeadFailsReadiness(t *testing.T) {
	runner := readyDoctorRunner()
	runner.responses["git branch --show-current"] = ""
	report := BuildDoctorReport("StatPan/gira", runner, time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))

	if report.Ready {
		t.Fatal("ready = true, want false")
	}
	check := doctorCheckByID(report, "local_git_state")
	if check == nil {
		t.Fatal("local_git_state check missing")
	}
	if check.Status != DoctorCheckFail {
		t.Fatalf("local_git_state status = %s, want fail: %+v", check.Status, *check)
	}
	if !strings.Contains(check.Detail, "detached HEAD") {
		t.Fatalf("local_git_state detail = %q, want detached HEAD", check.Detail)
	}
}

func TestBuildDoctorReportDirtyWorktreeFailsReadiness(t *testing.T) {
	runner := readyDoctorRunner()
	runner.responses["git status --porcelain"] = " M internal/gira/doctor.go\n?? scratch.txt\n"
	report := BuildDoctorReport("StatPan/gira", runner, time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC))

	if report.Ready {
		t.Fatal("ready = true, want false")
	}
	check := doctorCheckByID(report, "local_git_state")
	if check == nil {
		t.Fatal("local_git_state check missing")
	}
	if check.Status != DoctorCheckFail {
		t.Fatalf("local_git_state status = %s, want fail: %+v", check.Status, *check)
	}
	if !strings.Contains(check.Detail, "uncommitted changes=2") {
		t.Fatalf("local_git_state detail = %q, want dirty count", check.Detail)
	}
}

func readyDoctorRunner() onboardFakeRunner {
	return onboardFakeRunner{
		responses: map[string]string{
			"gh --version":   "gh version 2.0.0",
			"gh auth status": "Logged in to github.com",
			"gh repo view StatPan/gira --json nameWithOwner,viewerPermission,defaultBranchRef":                             `{"nameWithOwner":"StatPan/gira","viewerPermission":"WRITE","defaultBranchRef":{"name":"main"}}`,
			"gh label list --repo StatPan/gira --json name,color,description --limit 1000":                                 desiredLabelsJSON(),
			"gh api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100":                  doctorDesiredMilestonesJSON(),
			"gh api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100":                      `[[{"number":129,"title":"Doctor","state":"open","labels":[{"name":"type:task"}],"milestone":{"title":"MVP"},"updated_at":"2026-05-05T12:00:00Z","html_url":"https://github.com/StatPan/gira/issues/129"}]]`,
			"gh issue list --repo StatPan/gira --state all --label gira:bootstrap --json number,title,labels --limit 1000": desiredBootstrapIssuesJSON(),
			"git rev-parse --is-inside-work-tree":                                                                          "true",
			"git branch --show-current":                                                                                    "main",
			"git status --porcelain":                                                                                       "",
		},
		errors: map[string]error{},
	}
}

func doctorDesiredMilestonesJSON() string {
	parts := make([]string, 0, len(DesiredMilestones))
	for idx, milestone := range DesiredMilestones {
		dueOn := "null"
		if milestone.DueOn != nil {
			dueOn = fmt.Sprintf("%q", *milestone.DueOn)
		}
		parts = append(parts, fmt.Sprintf(`{"number":%d,"title":%q,"description":%q,"due_on":%s}`, idx+1, milestone.Title, milestone.Description, dueOn))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func doctorCheckByID(report DoctorReport, id string) *DoctorCheck {
	for i := range report.Checks {
		if report.Checks[i].ID == id {
			return &report.Checks[i]
		}
	}
	return nil
}
