package gira

import (
	"fmt"
	"strings"
	"testing"
)

type ticketNewRunner struct {
	outputs map[string][]byte
	errs    map[string]error
	calls   []string
}

func (r *ticketNewRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, key)
	if err, ok := r.errs[key]; ok {
		return nil, err
	}
	if out, ok := r.outputs[key]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected call: %s", key)
}

func TestTicketNewDryRunRendersStructuredBody(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}

	report, err := BuildTicketNewReport(TicketNewInput{
		Repo:       repo,
		Title:      "Add retry",
		Goal:       "Retry transient auth failures",
		Scope:      "CLI auth only",
		Acceptance: []string{"retries 3 times", "does not retry 401"},
		Notes:      "Keep logs terse",
		Type:       "bug",
		Priority:   "p1",
		Labels:     []string{"area:backend"},
		DryRun:     true,
	}, &ticketNewRunner{})
	if err != nil {
		t.Fatalf("BuildTicketNewReport error: %v", err)
	}
	for _, want := range []string{"## Goal", "Retry transient auth failures", "- retries 3 times", "## Notes", "Keep logs terse"} {
		if !strings.Contains(report.Body, want) {
			t.Fatalf("body missing %q:\n%s", want, report.Body)
		}
	}
	for _, want := range []string{"type:bug", "status:ready", "priority:p1", "area:backend"} {
		if !containsString(report.Labels, want) {
			t.Fatalf("labels missing %q: %+v", want, report.Labels)
		}
	}
}

func TestTicketNewApplyCreatesIssue(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &ticketNewRunner{outputs: map[string][]byte{
		"gh issue create --repo StatPan/gira --title Add retry --body ## Goal\nAdd retry\n\n## Scope\n_No response_\n\n## Acceptance Criteria\n_No response_\n\n## Notes\n_No response_\n --label type:task --label status:ready --milestone v1.2": []byte("https://github.com/StatPan/gira/issues/224\n"),
	}}

	report, err := BuildTicketNewReport(TicketNewInput{Repo: repo, Title: "Add retry", Type: "task", Milestone: "v1.2"}, runner)
	if err != nil {
		t.Fatalf("BuildTicketNewReport error: %v", err)
	}
	if report.Created.Number != 224 || report.NextStep != "gira ticket start 224 --apply" {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestTicketNewApplyCreatesIssueWithFullBody(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	body := "## Goal\nUse exact packet\n\n## Acceptance Criteria\n- preserved"
	runner := &ticketNewRunner{outputs: map[string][]byte{
		"gh issue create --repo StatPan/gira --title Add packet --body " + body + " --label type:task --label status:ready": []byte("https://github.com/StatPan/gira/issues/225\n"),
	}}

	report, err := BuildTicketNewReport(TicketNewInput{Repo: repo, Title: "Add packet", Body: body, Type: "task"}, runner)
	if err != nil {
		t.Fatalf("BuildTicketNewReport error: %v", err)
	}
	if report.Created.Number != 225 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestTicketNewUsesFullBody(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	body := "## Goal\nUse exact packet\n\n## Acceptance Criteria\n- preserved"

	report, err := BuildTicketNewReport(TicketNewInput{Repo: repo, Title: "Add packet", Body: body, Type: "task", DryRun: true}, &ticketNewRunner{})
	if err != nil {
		t.Fatalf("BuildTicketNewReport error: %v", err)
	}
	if report.Body != body {
		t.Fatalf("body = %q, want exact body %q", report.Body, body)
	}
}

func TestTicketNewRejectsBodyAndBodyFile(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	_, err := BuildTicketNewReport(TicketNewInput{Repo: repo, Title: "Add packet", Body: "body", BodyFile: "issue.md", Type: "task", DryRun: true}, &ticketNewRunner{})
	if err == nil || !strings.Contains(err.Error(), "either --body or --body-file") {
		t.Fatalf("error = %v, want body conflict", err)
	}
}

func TestTicketNewAllowsEpicType(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}

	report, err := BuildTicketNewReport(TicketNewInput{Repo: repo, Title: "Native adoption flow", Type: "epic", DryRun: true}, &ticketNewRunner{})
	if err != nil {
		t.Fatalf("BuildTicketNewReport error: %v", err)
	}
	if !containsString(report.Labels, "type:epic") {
		t.Fatalf("labels missing type:epic: %+v", report.Labels)
	}
}

func TestTicketNewApplyStartRunsStartWork(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &ticketNewRunner{outputs: map[string][]byte{
		"gh issue create --repo StatPan/gira --title Add retry --body ## Goal\nAdd retry\n\n## Scope\n_No response_\n\n## Acceptance Criteria\n_No response_\n\n## Notes\n_No response_\n --label type:task --label status:ready": []byte("https://github.com/StatPan/gira/issues/224\n"),
		"gh api repos/StatPan/gira/issues/224":                                               []byte(`{"number":224,"title":"Add retry","state":"open","labels":[{"name":"status:ready"}]}`),
		"git checkout -b issue-224-add-retry":                                                nil,
		"gh api repos/StatPan/gira/issues/224/labels/status:ready -X DELETE":                 nil,
		"gh api repos/StatPan/gira/issues/224/labels -X POST -f labels[]=status:in-progress": nil,
	}, errs: map[string]error{
		"git show-ref --verify --quiet refs/heads/issue-224-add-retry": fmt.Errorf("exit status 1"),
		"git ls-remote --exit-code --heads origin issue-224-add-retry": fmt.Errorf("exit status 2"),
	}}

	report, err := BuildTicketNewReport(TicketNewInput{Repo: repo, Title: "Add retry", Type: "task", Start: true}, runner)
	if err != nil {
		t.Fatalf("BuildTicketNewReport error: %v", err)
	}
	if report.StartResult.Issue != 224 || report.NextStep != "gira ticket pr --dry-run" {
		t.Fatalf("unexpected start report: %+v", report)
	}
	if !containsCall(runner.calls, "git checkout -b issue-224-add-retry") {
		t.Fatalf("missing branch start call: %v", runner.calls)
	}
}

func TestTicketNewRejectsInvalidTypeAndPriority(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	if _, err := BuildTicketNewReport(TicketNewInput{Repo: repo, Title: "x", Type: "initiative", DryRun: true}, &ticketNewRunner{}); err == nil || !strings.Contains(err.Error(), "--type") {
		t.Fatalf("expected type error, got %v", err)
	}
	if _, err := BuildTicketNewReport(TicketNewInput{Repo: repo, Title: "x", Type: "task", Priority: "high", DryRun: true}, &ticketNewRunner{}); err == nil || !strings.Contains(err.Error(), "--priority") {
		t.Fatalf("expected priority error, got %v", err)
	}
}
