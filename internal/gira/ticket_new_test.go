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
	if out, ok := defaultBranchPolicyTestOutput(key); ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected call: %s", key)
}

func ticketNewLabelOutputs(labels ...string) map[string][]byte {
	rows := make([]string, 0, len(labels))
	for _, label := range labels {
		rows = append(rows, fmt.Sprintf(`{"name":%q}`, label))
	}
	return map[string][]byte{
		"gh label list --repo StatPan/gira --json name --limit 1000": []byte("[" + strings.Join(rows, ",") + "]"),
	}
}

func ticketNewRESTLabelOutputs(labels ...string) map[string][]byte {
	rows := make([]string, 0, len(labels))
	for _, label := range labels {
		rows = append(rows, fmt.Sprintf(`{"name":%q}`, label))
	}
	return map[string][]byte{
		"gh api repos/StatPan/gira/labels --paginate --slurp -X GET -f per_page=100": []byte("[[" + strings.Join(rows, ",") + "]]"),
	}
}

func TestTicketNewDryRunRendersStructuredBody(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &ticketNewRunner{outputs: ticketNewLabelOutputs("type:bug", "status:ready", "priority:p1", "area:backend")}

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
	}, runner)
	if err != nil {
		t.Fatalf("BuildTicketNewReport error: %v", err)
	}
	for _, want := range []string{"## Goal", "Retry transient auth failures", "- retries 3 times", "## Doctor Impact", "## Notes", "Keep logs terse", ProvenanceBlockStart, "planning: human", ProvenanceBlockEnd} {
		if !strings.Contains(report.Body, want) {
			t.Fatalf("body missing %q:\n%s", want, report.Body)
		}
	}
	for _, want := range []string{"type:bug", "status:ready", "priority:p1", "area:backend"} {
		if !containsString(report.Labels, want) {
			t.Fatalf("labels missing %q: %+v", want, report.Labels)
		}
	}
	if report.TicketReadiness.SchemaVersion != TicketReadinessSchemaVersion || report.TicketReadiness.Readiness != "ready" {
		t.Fatalf("expected ticket readiness report, got %+v", report.TicketReadiness)
	}
	if report.SchemaVersion != TicketNewReportSchemaVersion || report.Type != "bug" || report.Priority != "p1" || report.Approval == nil {
		t.Fatalf("expected ticket new schema/type/approval evidence: %+v", report)
	}
	if report.Approval.SchemaVersion != ApprovalPlanSchemaVersion || report.Approval.CanonicalCommand != "gira ticket new" || report.Approval.OutputSchema != TicketNewReportSchemaVersion {
		t.Fatalf("unexpected ticket new approval evidence: %+v", report.Approval)
	}
	if report.Approval.PostApplyVerification != "gira ticket status <created-ticket> --repo StatPan/gira --json" {
		t.Fatalf("unexpected ticket new approval commands: %+v", report.Approval)
	}
	for _, want := range []string{"gira ticket new 'Add retry' --repo StatPan/gira --body '## Goal\nRetry transient auth failures", "--type bug", "--priority p1", "--label area:backend", "--apply"} {
		if !strings.Contains(report.Approval.ApplyCommand, want) {
			t.Fatalf("ticket new approval command missing %q: %+v", want, report.Approval)
		}
	}
	if report.Approval.Blockers == nil || report.Approval.Warnings == nil || !approvalHasAction(report.Approval.PlannedActions, "issue:create") {
		t.Fatalf("unexpected ticket new approval plan: %+v", report.Approval)
	}
}

func TestTicketNewExplicitStatusLabelOverridesDefaultReady(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &ticketNewRunner{outputs: ticketNewLabelOutputs("type:task", "status:ready", "status:blocked")}

	report, err := BuildTicketNewReport(TicketNewInput{
		Repo:   repo,
		Title:  "Repro explicit status",
		Goal:   "Repro explicit status",
		Type:   "task",
		Labels: []string{"status:blocked"},
		DryRun: true,
	}, runner)
	if err != nil {
		t.Fatalf("BuildTicketNewReport error: %v", err)
	}
	if containsString(report.Labels, "status:ready") {
		t.Fatalf("labels include default status:ready despite explicit status label: %+v", report.Labels)
	}
	if !containsString(report.Labels, "status:blocked") {
		t.Fatalf("labels missing explicit status:blocked: %+v", report.Labels)
	}
	if report.TicketReadiness.Readiness != "blocked" {
		t.Fatalf("unexpected readiness with explicit status override: %+v", report.TicketReadiness)
	}
	for _, finding := range report.TicketReadiness.Findings {
		if finding.Kind == "multiple_status_labels" {
			t.Fatalf("explicit status override still produces multiple status finding: %+v", report.TicketReadiness)
		}
	}
}

func TestTicketNewLabelPreflightUsesRESTBeforeGraphQLHeavyLabelList(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &ticketNewRunner{
		outputs: ticketNewRESTLabelOutputs("type:task", "status:ready"),
		errs: map[string]error{
			"gh label list --repo StatPan/gira --json name --limit 1000": fmt.Errorf("GraphQL-heavy label list should not be called when REST succeeds"),
		},
	}

	report, err := BuildTicketNewReport(TicketNewInput{
		Repo:       repo,
		Title:      "Add CLI",
		Goal:       "Add CLI",
		Scope:      "Ticket creation label preflight",
		Acceptance: []string{"uses REST labels"},
		Type:       "task",
		DryRun:     true,
	}, runner)
	if err != nil {
		t.Fatalf("BuildTicketNewReport error: %v", err)
	}
	if report.TicketReadiness.Readiness != "ready" {
		t.Fatalf("unexpected readiness: %+v", report.TicketReadiness)
	}
	for _, call := range runner.calls {
		if strings.HasPrefix(call, "gh label list ") {
			t.Fatalf("called GraphQL-heavy label list despite REST success: %v", runner.calls)
		}
	}
}

func TestTicketNewApplyCreatesIssue(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	outputs := ticketNewLabelOutputs("type:task", "status:ready")
	outputs["gh issue create --repo StatPan/gira --title Add retry --body "+defaultTicketNewBody("Add retry")+" --label type:task --label status:ready --milestone v1.2"] = []byte("https://github.com/StatPan/gira/issues/224\n")
	runner := &ticketNewRunner{outputs: outputs}

	report, err := BuildTicketNewReport(TicketNewInput{Repo: repo, Title: "Add retry", Type: "task", Milestone: "v1.2"}, runner)
	if err != nil {
		t.Fatalf("BuildTicketNewReport error: %v", err)
	}
	if report.Created.Number != 224 || report.NextStep != "gira ticket start 224 --apply" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Approval != nil {
		t.Fatalf("apply output should not include dry-run approval evidence: %+v", report.Approval)
	}
}

func TestTicketNewParentDryRunPlansNativeSubIssue(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	outputs := ticketNewLabelOutputs("type:task", "status:ready")
	outputs["gh api repos/StatPan/gira/issues/10 -H Accept: application/vnd.github+json"] = []byte(`{"id":1000,"number":10,"title":"Parent","state":"open"}`)
	runner := &ticketNewRunner{outputs: outputs}

	report, err := BuildTicketNewReport(TicketNewInput{Repo: repo, Title: "Add child", Type: "task", Parent: 10, DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildTicketNewReport error: %v", err)
	}
	if report.Parent != 10 {
		t.Fatalf("parent = %d, want 10", report.Parent)
	}
	if report.Approval == nil || !approvalHasAction(report.Approval.PlannedActions, "parent:set") || !strings.Contains(report.Approval.ApplyCommand, "--parent 10") {
		t.Fatalf("approval missing parent plan: %+v", report.Approval)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "/sub_issues") {
			t.Fatalf("dry-run should not mutate sub-issues, calls=%+v", runner.calls)
		}
	}
}

func TestTicketNewParentApplyLinksNativeSubIssue(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	outputs := ticketNewLabelOutputs("type:task", "status:ready")
	outputs["gh api repos/StatPan/gira/issues/10 -H Accept: application/vnd.github+json"] = []byte(`{"id":1000,"number":10,"title":"Parent","state":"open"}`)
	outputs["gh issue create --repo StatPan/gira --title Add child --body "+defaultTicketNewBody("Add child")+" --label type:task --label status:ready"] = []byte("https://github.com/StatPan/gira/issues/224\n")
	outputs["gh api repos/StatPan/gira/issues/224 -H Accept: application/vnd.github+json"] = []byte(`{"id":2240,"number":224,"title":"Add child","state":"open"}`)
	outputs["gh api repos/StatPan/gira/issues/10/sub_issues -X POST -H Accept: application/vnd.github+json -H X-GitHub-Api-Version: 2026-03-10 -F sub_issue_id=2240"] = []byte(`{"number":224}`)
	runner := &ticketNewRunner{outputs: outputs}

	report, err := BuildTicketNewReport(TicketNewInput{Repo: repo, Title: "Add child", Type: "task", Parent: 10}, runner)
	if err != nil {
		t.Fatalf("BuildTicketNewReport error: %v", err)
	}
	if report.Created.Number != 224 {
		t.Fatalf("created = %+v, want #224", report.Created)
	}
}

func TestTicketNewApplyCreatesIssueWithFullBody(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	body := "## Goal\nUse exact packet\n\n## Acceptance Criteria\n- preserved"
	outputs := ticketNewLabelOutputs("type:task", "status:ready")
	outputs["gh issue create --repo StatPan/gira --title Add packet --body "+body+" --label type:task --label status:ready"] = []byte("https://github.com/StatPan/gira/issues/225\n")
	runner := &ticketNewRunner{outputs: outputs}

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
	runner := &ticketNewRunner{outputs: ticketNewLabelOutputs("type:task", "status:ready")}

	report, err := BuildTicketNewReport(TicketNewInput{Repo: repo, Title: "Add packet", Body: body, Type: "task", DryRun: true}, runner)
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
	runner := &ticketNewRunner{outputs: ticketNewLabelOutputs("type:epic", "status:ready")}

	report, err := BuildTicketNewReport(TicketNewInput{Repo: repo, Title: "Native adoption flow", Type: "epic", DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildTicketNewReport error: %v", err)
	}
	if !containsString(report.Labels, "type:epic") {
		t.Fatalf("labels missing type:epic: %+v", report.Labels)
	}
}

func TestTicketNewApplyStartRunsStartWork(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	outputs := ticketNewLabelOutputs("type:task", "status:ready")
	outputs["gh issue create --repo StatPan/gira --title Add retry --body "+defaultTicketNewBody("Add retry")+" --label type:task --label status:ready"] = []byte("https://github.com/StatPan/gira/issues/224\n")
	outputs["gh api repos/StatPan/gira/issues/224"] = []byte(`{"number":224,"title":"Add retry","state":"open","labels":[{"name":"status:ready"}]}`)
	outputs["git checkout -b issue-224-add-retry origin/main"] = nil
	outputs["gh api repos/StatPan/gira/issues/224/labels/status:ready -X DELETE"] = nil
	outputs["gh api repos/StatPan/gira/issues/224/labels -X POST -f labels[]=status:in-progress"] = nil
	runner := &ticketNewRunner{outputs: outputs, errs: map[string]error{
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
	if !containsCall(runner.calls, "git checkout -b issue-224-add-retry origin/main") {
		t.Fatalf("missing branch start call: %v", runner.calls)
	}
}

func defaultTicketNewBody(title string) string {
	return "## Goal\n" + title + "\n\n## Scope\n_No response_\n\n## Acceptance Criteria\n_No response_\n\n## Doctor Impact\n_No response_\n\n## Notes\n_No response_\n\n" + DefaultProvenanceBlock() + "\n"
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

func TestTicketNewRejectsFeatureTypeWithGuidance(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}

	_, err := BuildTicketNewReport(TicketNewInput{Repo: repo, Title: "x", Type: "feature", DryRun: true}, &ticketNewRunner{})
	if err == nil {
		t.Fatal("expected feature type error")
	}
	for _, want := range []string{"unsupported --type feature", "--type story --label enhancement"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q: %v", want, err)
		}
	}
}

func TestTicketNewDryRunPreflightsMissingLabels(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &ticketNewRunner{outputs: ticketNewLabelOutputs("type:task", "status:ready")}

	report, err := BuildTicketNewReport(TicketNewInput{Repo: repo, Title: "Add CLI", Type: "task", Labels: []string{"area:cli"}, DryRun: true}, runner)
	if err == nil {
		t.Fatal("expected missing label error")
	}
	if report.Title != "Add CLI" || !containsString(report.Labels, "area:cli") {
		t.Fatalf("expected partial report with labels, got %+v", report)
	}
	if !strings.Contains(err.Error(), "missing repo labels: area:cli") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTicketNewMissingTypeLabelSuggestsManagedCandidates(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &ticketNewRunner{outputs: ticketNewLabelOutputs("type:task", "type:story", "type:bug", "status:ready")}

	_, err := BuildTicketNewReport(TicketNewInput{Repo: repo, Title: "Add feature", Type: "task", Labels: []string{"type:feature"}, DryRun: true}, runner)
	if err == nil {
		t.Fatal("expected missing feature label error")
	}
	for _, want := range []string{"missing repo labels: type:feature", "candidates:", "type:task", "type:story", "type:bug"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q: %v", want, err)
		}
	}
}

func TestTicketNewMissingAreaLabelSuggestsManagedCandidates(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &ticketNewRunner{outputs: ticketNewLabelOutputs("type:task", "status:ready", "area:backend", "area:docs")}

	_, err := BuildTicketNewReport(TicketNewInput{Repo: repo, Title: "Add CLI", Type: "task", Labels: []string{"area:cli"}, DryRun: true}, runner)
	if err == nil {
		t.Fatal("expected missing area label error")
	}
	for _, want := range []string{"missing repo labels: area:cli", "candidates:", "area:backend", "area:docs"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q: %v", want, err)
		}
	}
}

func TestTicketNewApplyPreflightStopsBeforeIssueCreate(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &ticketNewRunner{outputs: ticketNewLabelOutputs("type:task")}

	_, err := BuildTicketNewReport(TicketNewInput{Repo: repo, Title: "Add CLI", Type: "task"}, runner)
	if err == nil {
		t.Fatal("expected missing status label error")
	}
	if containsCall(runner.calls, "gh issue create") {
		t.Fatalf("issue create should not run on missing labels: %v", runner.calls)
	}
}
