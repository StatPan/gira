package gira

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type workGraphRunner struct {
	body          string
	child         bool
	receipts      []string
	creates       int
	comments      []string
	createdBodies []string
	parentLinks   int
}

func (r *workGraphRunner) Run(name string, args ...string) ([]byte, error) {
	call := name + " " + strings.Join(args, " ")
	switch {
	case call == "gh api repos/OWNER/repo/issues/100":
		return []byte(fmt.Sprintf(`{"number":100,"title":"Typed goal","state":"open","body":%q,"labels":[{"name":"type:epic"},{"name":"status:ready"}]}`, r.body)), nil
	case call == "gh api repos/OWNER/repo/issues/100/sub_issues -X GET -H Accept: application/vnd.github+json -H X-GitHub-Api-Version: 2026-03-10 -f per_page=100":
		if r.child {
			return []byte(`[{"number":101}]`), nil
		}
		return []byte(`[]`), nil
	case call == "gh issue view 100 --repo OWNER/repo --json comments":
		items := []map[string]string{}
		for _, body := range r.receipts {
			items = append(items, map[string]string{"body": body})
		}
		value, _ := json.Marshal(map[string]any{"comments": items})
		return value, nil
	case call == "gh issue view 100 --repo OWNER/repo --json number,title,body,url,comments":
		return []byte(fmt.Sprintf(`{"number":100,"title":"Typed goal","body":%q,"url":"https://example/100","comments":[]}`, r.body)), nil
	case call == "gh api repos/OWNER/repo/labels --paginate --slurp -X GET -f per_page=100":
		return []byte(`[[{"name":"type:task"},{"name":"status:ready"},{"name":"priority:p2"}]]`), nil
	case call == "gh api repos/OWNER/repo/issues/101":
		return []byte(`{"number":101,"title":"[Task] Existing evidence","state":"open","body":"## Goal\nExisting","labels":[{"name":"type:task"},{"name":"status:ready"}]}`), nil
	case strings.HasPrefix(call, "gh pr list --repo OWNER/repo") && strings.Contains(call, " 101 "):
		return []byte(`[]`), nil
	case strings.HasPrefix(call, "gh issue create --repo OWNER/repo --title "):
		r.creates++
		r.child = true
		for i, arg := range args {
			if arg == "--body" && i+1 < len(args) {
				r.createdBodies = append(r.createdBodies, args[i+1])
			}
		}
		return []byte(fmt.Sprintf("https://github.com/OWNER/repo/issues/%d\n", 200+r.creates)), nil
	case strings.HasPrefix(call, "gh api repos/OWNER/repo/issues/20") && strings.HasSuffix(call, " -H Accept: application/vnd.github+json"):
		return []byte(fmt.Sprintf(`{"id":%d,"number":%d,"title":"created"}`, 9000+r.creates, 200+r.creates)), nil
	case strings.HasPrefix(call, "gh api repos/OWNER/repo/issues/100/sub_issues -X POST "):
		r.parentLinks++
		return []byte(`{"number":201}`), nil
	case strings.HasPrefix(call, "gh search issues "):
		return []byte(`[]`), nil
	case strings.HasPrefix(call, "gh issue comment 100 --repo OWNER/repo --body "):
		body := args[len(args)-1]
		r.comments = append(r.comments, body)
		r.receipts = append(r.receipts, body)
		return []byte(`{"url":"https://example/comment"}`), nil
	case strings.HasPrefix(call, "gh issue comment 101 --repo OWNER/repo --body "):
		return []byte(`{}`), nil
	case call == "gh issue close 101 --repo OWNER/repo --reason not planned":
		return []byte(``), nil
	default:
		return nil, fmt.Errorf("unexpected command: %s", call)
	}
}

func TestPMWorkGraphCompilesMixedProfilesAndExplicitActions(t *testing.T) {
	source := PMWorkGraphSource{SchemaVersion: PMWorkGraphSourceSchemaVersion, Nodes: []PMWorkGraphNode{
		{ID: "discover", Title: "Existing evidence", Purpose: "Resolve user value", Profile: "discovery", ParentOutcome: "goal:100", Size: "small", Verification: []PMWorkGraphVerification{{Method: "interviews", Evidence: "learning receipt"}}},
		{ID: "decision", Title: "Choose policy", Purpose: "Resolve rollout policy", Profile: "decision", ParentOutcome: "goal:100", Size: "small", Verification: []PMWorkGraphVerification{{Method: "decision review", Evidence: "accepted receipt"}}, Dependencies: []PMWorkGraphDependency{{NodeID: "discover", Kind: "information", Reason: "learning selects viable options"}}},
		{ID: "delivery", Title: "Build selected flow", Purpose: "Deliver chosen solution", Profile: "delivery", ParentOutcome: "goal:100", Size: "small", Uncertainty: "unresolved", DeferUntil: "decision", ResumeCondition: "decision receipt is accepted", Verification: []PMWorkGraphVerification{{Method: "go test ./...", Evidence: "passing checks"}}, Dependencies: []PMWorkGraphDependency{{NodeID: "decision", Kind: "information", Reason: "implementation requires selected policy"}}},
		{ID: "big", Title: "Large migration", Purpose: "Migrate safely", Profile: "rollout", ParentOutcome: "goal:100", Size: "oversized", SplitInto: []string{"part-a", "part-b"}, Verification: []PMWorkGraphVerification{{Method: "migration audit", Evidence: "audit receipt"}}},
		{ID: "part-a", Title: "Migration part A", Purpose: "Migrate first cohort", Profile: "rollout", ParentOutcome: "goal:100", Size: "small", Verification: []PMWorkGraphVerification{{Method: "cohort check", Evidence: "stable guardrails"}}},
		{ID: "part-b", Title: "Migration part B", Purpose: "Migrate second cohort", Profile: "rollout", ParentOutcome: "goal:100", Size: "small", Verification: []PMWorkGraphVerification{{Method: "cohort check", Evidence: "stable guardrails"}}, Dependencies: []PMWorkGraphDependency{{NodeID: "part-a", Kind: "ordering", Reason: "second cohort follows first cohort stability"}}},
		{ID: "replace", Title: "Replace obsolete child", Purpose: "Supersede obsolete work", Profile: "decision", ParentOutcome: "goal:100", Size: "small", SupersedesIssue: 101, Verification: []PMWorkGraphVerification{{Method: "supersession receipt", Evidence: "replacement link"}}},
	}}
	runner := &workGraphRunner{body: workGraphGoalBody(t, source), child: true}
	report, err := BuildPMWorkGraphReport(PMWorkGraphInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Goal: 100, DryRun: true}, runner)
	if err != nil {
		t.Fatal(err)
	}
	if hasPMWorkGraphErrors(report.Diagnostics) {
		t.Fatalf("diagnostics=%#v", report.Diagnostics)
	}
	actions := map[string]string{}
	for _, a := range report.Actions {
		actions[a.NodeID] = a.Action
	}
	for id, want := range map[string]string{"discover": "reuse", "delivery": "defer", "big": "split", "decision": "create", "replace": "supersede"} {
		if actions[id] != want {
			t.Fatalf("action %s=%s want %s", id, actions[id], want)
		}
	}
	if report.PlanID == "" || len(report.Order) != 7 || report.PMIRDigest == "" {
		t.Fatalf("incomplete graph report: %#v", report)
	}
	compact := BuildPMWorkGraphCompact(report)
	if len(compact.Nodes) != 7 || compact.Nodes[0].PayloadSHA256 == "" {
		t.Fatalf("compact lost fingerprints: %#v", compact)
	}
	fullJSON, _ := json.Marshal(report)
	compactJSON, _ := json.Marshal(compact)
	if len(compactJSON) >= len(fullJSON)/2 || strings.Contains(string(compactJSON), "Resolve rollout policy") {
		t.Fatalf("compact repeats graph bodies: full=%d compact=%d", len(fullJSON), len(compactJSON))
	}
}

func TestPMWorkGraphRejectsCyclesFalseDependenciesAndResidualDeliveryJudgment(t *testing.T) {
	source := PMWorkGraphSource{SchemaVersion: PMWorkGraphSourceSchemaVersion, Nodes: []PMWorkGraphNode{{ID: "a", Title: "A", Purpose: "A", Profile: "delivery", ParentOutcome: "goal:100", Uncertainty: "unresolved", Verification: []PMWorkGraphVerification{{Method: "test", Evidence: "result"}}, Dependencies: []PMWorkGraphDependency{{NodeID: "b", Kind: "presentation"}}}, {ID: "b", Title: "B", Purpose: "B", Profile: "documentation", ParentOutcome: "missing", Verification: []PMWorkGraphVerification{{Method: "", Evidence: ""}}, Dependencies: []PMWorkGraphDependency{{NodeID: "a", Kind: "ordering", Reason: "real order"}}, DeferUntil: "later"}}}
	report, err := BuildPMWorkGraphReport(PMWorkGraphInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Goal: 100}, &workGraphRunner{body: workGraphGoalBody(t, source)})
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{PMWorkGraphCycle, PMWorkGraphFalseDependency, PMWorkGraphUnresolvedJudgment, PMWorkGraphUnknownOutcome, PMWorkGraphUnverifiable, PMWorkGraphMissingResume} {
		if !hasWorkGraphDiagnostic(report.Diagnostics, code) {
			t.Fatalf("missing %s: %#v", code, report.Diagnostics)
		}
	}
}

func TestPMWorkGraphMissingSourceGuidesLegacyGoalPlan(t *testing.T) {
	runner := &workGraphRunner{body: "## Goal\nPlain goal.\n\n## Scope\nBounded CLI work.\n\n## Decomposition\n- Add the first child ticket.\n"}
	report, err := BuildPMWorkGraphReport(PMWorkGraphInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Goal: 100, DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildPMWorkGraphReport: %v", err)
	}
	diagnostic := workGraphDiagnosticByCode(report.Diagnostics, PMWorkGraphMissingSource)
	if diagnostic == nil || diagnostic.Severity != "info" || !strings.Contains(diagnostic.Repair, "goal plan") {
		t.Fatalf("missing-source diagnostic did not guide legacy planning: %#v", report.Diagnostics)
	}
	if workGraphDiagnosticByCode(report.Diagnostics, PMWorkGraphInvalidSource) != nil || hasPMWorkGraphErrors(report.Diagnostics) {
		t.Fatalf("absent opt-in graph was treated as invalid: %#v", report.Diagnostics)
	}
	if !strings.Contains(report.NextStep, "gira goal plan") || len(report.Nodes) != 0 {
		t.Fatalf("missing-source next step/nodes = %q/%d", report.NextStep, len(report.Nodes))
	}
}

func TestPMWorkGraphInvalidSourceFailsClosed(t *testing.T) {
	runner := &workGraphRunner{body: "## Goal\nTyped goal.\n\n## Scope\nTyped scope.\n\n## Work Graph\n```json\n{\"schema_version\":\"wrong\",\"nodes\":[]}\n```\n"}
	report, err := BuildPMWorkGraphReport(PMWorkGraphInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Goal: 100, DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildPMWorkGraphReport: %v", err)
	}
	diagnostic := workGraphDiagnosticByCode(report.Diagnostics, PMWorkGraphInvalidSource)
	if diagnostic == nil || diagnostic.Severity != "error" {
		t.Fatalf("invalid source was not an error: %#v", report.Diagnostics)
	}
	if workGraphDiagnosticByCode(report.Diagnostics, PMWorkGraphMissingSource) != nil {
		t.Fatalf("invalid source was mislabeled missing: %#v", report.Diagnostics)
	}
}

func TestPMWorkGraphMissingSourceApplyDoesNotFabricateGraph(t *testing.T) {
	runner := &workGraphRunner{body: "## Goal\nPlain goal.\n\n## Scope\nBounded CLI work.\n\n## Decomposition\n- Add the first child ticket.\n"}
	preview, err := BuildPMWorkGraphReport(PMWorkGraphInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Goal: 100, DryRun: true}, runner)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	_, err = BuildPMWorkGraphReport(PMWorkGraphInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Goal: 100, Apply: true, ExpectedPlanID: preview.PlanID}, runner)
	if err == nil || !strings.Contains(err.Error(), "goal plan") || len(runner.comments) != 0 || runner.creates != 0 {
		t.Fatalf("missing graph apply fabricated work: err=%v comments=%d creates=%d", err, len(runner.comments), runner.creates)
	}
}

func TestPMWorkGraphFingerprintGuardApplyAndIdempotentRetry(t *testing.T) {
	source := PMWorkGraphSource{SchemaVersion: PMWorkGraphSourceSchemaVersion, Nodes: []PMWorkGraphNode{{
		ID: "build", Title: "Build bounded slice", Purpose: "Deliver verified behavior", Profile: "delivery", ParentOutcome: "goal:100", Size: "small", Uncertainty: "resolved",
		Verification: []PMWorkGraphVerification{{Method: "go test ./...", Evidence: "passing tests"}},
	}}}
	runner := &workGraphRunner{body: workGraphGoalBody(t, source)}
	preview, err := BuildPMWorkGraphReport(PMWorkGraphInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Goal: 100, DryRun: true}, runner)
	if err != nil {
		t.Fatal(err)
	}
	changed := source
	changed.Nodes[0].Purpose = "Changed purpose"
	changedRunner := &workGraphRunner{body: workGraphGoalBody(t, changed)}
	mismatch, err := BuildPMWorkGraphReport(PMWorkGraphInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Goal: 100, Apply: true, ExpectedPlanID: preview.PlanID}, changedRunner)
	if err == nil || mismatch.Matched || changedRunner.creates != 0 {
		t.Fatalf("changed plan applied: %#v err=%v", mismatch, err)
	}
	applied, err := BuildPMWorkGraphReport(PMWorkGraphInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Goal: 100, Apply: true, ExpectedPlanID: preview.PlanID}, runner)
	if err != nil || runner.creates != 1 || runner.parentLinks != 1 || len(applied.Created) != 1 {
		t.Fatalf("apply failed: report=%#v err=%v", applied, err)
	}
	if len(runner.createdBodies) != 1 || EvaluatePMProfileReadiness(runner.createdBodies[0]).Readiness != "ready" {
		t.Fatalf("lowered child is not profile-ready: %#v", runner.createdBodies)
	}
	retry, err := BuildPMWorkGraphReport(PMWorkGraphInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Goal: 100, Apply: true, ExpectedPlanID: preview.PlanID}, runner)
	if err != nil || !retry.Idempotent || runner.creates != 1 {
		t.Fatalf("retry not idempotent: %#v err=%v", retry, err)
	}
}

func workGraphGoalBody(t *testing.T, source PMWorkGraphSource) string {
	t.Helper()
	encoded, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	return "## Premise\nTyped planning.\n\n## Actor\nPM.\n\n## Problem\nGeneric tickets lose semantics.\n\n## Desired Outcome\nVerifiable graph.\n\n## Goal\nCompile graph.\n\n## Scope\nTyped nodes.\n\n## Success Conditions\nEvery node verified.\n\n## Candidate Work\n- Compile nodes\n\n## Work Graph\n```json\n" + string(encoded) + "\n```\n"
}
func hasWorkGraphDiagnostic(values []PMWorkGraphDiagnostic, code string) bool {
	for _, v := range values {
		if v.Code == code {
			return true
		}
	}
	return false
}

func workGraphDiagnosticByCode(values []PMWorkGraphDiagnostic, code string) *PMWorkGraphDiagnostic {
	for i := range values {
		if values[i].Code == code {
			return &values[i]
		}
	}
	return nil
}

type resumableWorkGraphIssue struct {
	ID     int64
	Number int
	Title  string
	Body   string
}

type resumableWorkGraphRunner struct {
	body         string
	issues       map[int]resumableWorkGraphIssue
	linked       map[int]bool
	comments     []string
	creates      int
	parentLinks  int
	failCreateOn int
	failLinkOn   int
	failSearch   bool
	failComments bool
}

func newResumableWorkGraphRunner(body string) *resumableWorkGraphRunner {
	return &resumableWorkGraphRunner{body: body, issues: map[int]resumableWorkGraphIssue{}, linked: map[int]bool{}}
}

func (r *resumableWorkGraphRunner) Run(name string, args ...string) ([]byte, error) {
	call := name + " " + strings.Join(args, " ")
	switch {
	case call == "gh api repos/OWNER/repo/issues/100":
		return []byte(fmt.Sprintf(`{"number":100,"title":"Typed goal","state":"open","body":%q,"labels":[{"name":"type:epic"},{"name":"status:ready"}]}`, r.body)), nil
	case call == "gh api repos/OWNER/repo/issues/100/sub_issues -X GET -H Accept: application/vnd.github+json -H X-GitHub-Api-Version: 2026-03-10 -f per_page=100":
		numbers := make([]int, 0)
		for number, linked := range r.linked {
			if linked {
				numbers = append(numbers, number)
			}
		}
		sort.Ints(numbers)
		rows := make([]map[string]any, 0, len(numbers))
		for _, number := range numbers {
			issue := r.issues[number]
			rows = append(rows, map[string]any{"id": issue.ID, "number": issue.Number})
		}
		return json.Marshal(rows)
	case call == "gh issue view 100 --repo OWNER/repo --json comments":
		if r.failComments {
			return nil, fmt.Errorf("injected comments read failure")
		}
		items := make([]map[string]string, 0, len(r.comments))
		for _, body := range r.comments {
			items = append(items, map[string]string{"body": body})
		}
		return json.Marshal(map[string]any{"comments": items})
	case call == "gh issue view 100 --repo OWNER/repo --json number,title,body,url,comments":
		return []byte(fmt.Sprintf(`{"number":100,"title":"Typed goal","body":%q,"url":"https://example/100","comments":[]}`, r.body)), nil
	case call == "gh api repos/OWNER/repo/labels --paginate --slurp -X GET -f per_page=100":
		return []byte(`[[{"name":"type:task"},{"name":"status:ready"},{"name":"priority:p2"}]]`), nil
	case strings.HasPrefix(call, "gh api repos/OWNER/repo/issues/") && strings.HasSuffix(call, " -H Accept: application/vnd.github+json"):
		fields := strings.Fields(call)
		if len(fields) < 5 {
			return nil, fmt.Errorf("malformed issue fetch: %s", call)
		}
		number, err := strconv.Atoi(strings.TrimPrefix(fields[2], "repos/OWNER/repo/issues/"))
		if err != nil {
			return nil, err
		}
		issue, ok := r.issues[number]
		if !ok {
			return nil, fmt.Errorf("unknown issue %d", number)
		}
		return []byte(fmt.Sprintf(`{"id":%d,"number":%d,"title":%q,"state":"open","body":%q,"labels":[{"name":"type:task"},{"name":"status:ready"}]}`, issue.ID, issue.Number, issue.Title, issue.Body)), nil
	case strings.HasPrefix(call, "gh pr list --repo OWNER/repo"):
		return []byte(`[]`), nil
	case strings.HasPrefix(call, "gh search issues "):
		if r.failSearch {
			return nil, fmt.Errorf("injected search failure")
		}
		rows := make([]map[string]any, 0, len(r.issues))
		numbers := make([]int, 0, len(r.issues))
		for number := range r.issues {
			numbers = append(numbers, number)
		}
		sort.Ints(numbers)
		for _, number := range numbers {
			issue := r.issues[number]
			rows = append(rows, map[string]any{"number": issue.Number, "title": issue.Title, "body": issue.Body, "url": fmt.Sprintf("https://example/%d", issue.Number), "state": "open"})
		}
		return json.Marshal(rows)
	case strings.HasPrefix(call, "gh issue create --repo OWNER/repo --title "):
		r.creates++
		title := argumentValue(args, "--title")
		body := argumentValue(args, "--body")
		number := 200 + r.creates
		r.issues[number] = resumableWorkGraphIssue{ID: int64(9000 + number), Number: number, Title: title, Body: body}
		if r.failCreateOn == r.creates {
			return nil, fmt.Errorf("injected create failure after issue #%d persisted", number)
		}
		return []byte(fmt.Sprintf("https://github.com/OWNER/repo/issues/%d\n", number)), nil
	case strings.HasPrefix(call, "gh api repos/OWNER/repo/issues/100/sub_issues -X POST "):
		r.parentLinks++
		if r.failLinkOn == r.parentLinks {
			return nil, fmt.Errorf("injected parent link failure")
		}
		for _, arg := range args {
			if strings.HasPrefix(arg, "sub_issue_id=") {
				id, _ := strconv.ParseInt(strings.TrimPrefix(arg, "sub_issue_id="), 10, 64)
				for number, issue := range r.issues {
					if issue.ID == id {
						r.linked[number] = true
					}
				}
			}
		}
		return []byte(`{"number":201}`), nil
	case strings.HasPrefix(call, "gh issue comment 100 --repo OWNER/repo --body "):
		r.comments = append(r.comments, args[len(args)-1])
		return []byte(`{"url":"https://example/comment"}`), nil
	default:
		return nil, fmt.Errorf("unexpected command: %s", call)
	}
}

func argumentValue(args []string, key string) string {
	for i, arg := range args {
		if arg == key && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func TestPMWorkGraphApplyResumesAfterChildCreateFailureWithoutDuplicates(t *testing.T) {
	source := PMWorkGraphSource{SchemaVersion: PMWorkGraphSourceSchemaVersion, Nodes: []PMWorkGraphNode{
		{ID: "first", Title: "First bounded slice", Purpose: "Deliver first slice", Profile: "delivery", ParentOutcome: "goal:100", Size: "small", Uncertainty: "resolved", Verification: []PMWorkGraphVerification{{Method: "go test", Evidence: "passing tests"}}},
		{ID: "second", Title: "Second bounded slice", Purpose: "Deliver second slice", Profile: "delivery", ParentOutcome: "goal:100", Size: "small", Uncertainty: "resolved", Verification: []PMWorkGraphVerification{{Method: "go test", Evidence: "passing tests"}}},
	}}
	runner := newResumableWorkGraphRunner(workGraphGoalBody(t, source))
	preview, err := BuildPMWorkGraphReport(PMWorkGraphInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Goal: 100, DryRun: true}, runner)
	if err != nil {
		t.Fatal(err)
	}
	runner.failCreateOn = 2
	if _, err := BuildPMWorkGraphReport(PMWorkGraphInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Goal: 100, Apply: true, ExpectedPlanID: preview.PlanID}, runner); err == nil {
		t.Fatal("expected injected child create failure")
	}
	if runner.creates != 2 || len(runner.issues) != 2 {
		t.Fatalf("partial create state = creates:%d issues:%d", runner.creates, len(runner.issues))
	}
	runner.failCreateOn = 0
	retry, err := BuildPMWorkGraphReport(PMWorkGraphInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Goal: 100, Apply: true, ExpectedPlanID: preview.PlanID}, runner)
	if err != nil {
		t.Fatalf("resume after child create failure: %v", err)
	}
	if runner.creates != 2 || len(runner.issues) != 2 || len(runner.linked) != 2 {
		t.Fatalf("resume duplicated or failed to link: creates:%d issues:%d links:%d report=%#v", runner.creates, len(runner.issues), len(runner.linked), retry)
	}
	if len(retry.Created) != 0 || len(runner.comments) == 0 {
		t.Fatalf("retry did not reconcile existing children: created=%#v comments=%d", retry.Created, len(runner.comments))
	}
}

func TestPMWorkGraphApplyResumesAfterParentLinkFailureWithoutDuplicates(t *testing.T) {
	source := PMWorkGraphSource{SchemaVersion: PMWorkGraphSourceSchemaVersion, Nodes: []PMWorkGraphNode{{
		ID: "build", Title: "Build bounded slice", Purpose: "Deliver verified behavior", Profile: "delivery", ParentOutcome: "goal:100", Size: "small", Uncertainty: "resolved", Verification: []PMWorkGraphVerification{{Method: "go test", Evidence: "passing tests"}},
	}}}
	runner := newResumableWorkGraphRunner(workGraphGoalBody(t, source))
	preview, err := BuildPMWorkGraphReport(PMWorkGraphInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Goal: 100, DryRun: true}, runner)
	if err != nil {
		t.Fatal(err)
	}
	runner.failLinkOn = 1
	if _, err := BuildPMWorkGraphReport(PMWorkGraphInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Goal: 100, Apply: true, ExpectedPlanID: preview.PlanID}, runner); err == nil {
		t.Fatal("expected injected parent link failure")
	}
	if runner.creates != 1 || len(runner.issues) != 1 {
		t.Fatalf("unexpected partial link state: creates=%d issues=%d", runner.creates, len(runner.issues))
	}
	runner.failLinkOn = 0
	if _, err := BuildPMWorkGraphReport(PMWorkGraphInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Goal: 100, Apply: true, ExpectedPlanID: preview.PlanID}, runner); err != nil {
		t.Fatalf("resume after parent link failure: %v", err)
	}
	if runner.creates != 1 || len(runner.issues) != 1 || len(runner.linked) != 1 || runner.parentLinks != 2 {
		t.Fatalf("resume duplicated or failed to link: creates:%d issues:%d links:%d attempts:%d", runner.creates, len(runner.issues), len(runner.linked), runner.parentLinks)
	}
}

func TestPMWorkGraphApplyFailsClosedWhenReconciliationSearchFails(t *testing.T) {
	source := PMWorkGraphSource{SchemaVersion: PMWorkGraphSourceSchemaVersion, Nodes: []PMWorkGraphNode{{
		ID: "build", Title: "Build bounded slice", Purpose: "Deliver verified behavior", Profile: "delivery", ParentOutcome: "goal:100", Size: "small", Uncertainty: "resolved", Verification: []PMWorkGraphVerification{{Method: "go test", Evidence: "passing tests"}},
	}}}
	runner := newResumableWorkGraphRunner(workGraphGoalBody(t, source))
	preview, err := BuildPMWorkGraphReport(PMWorkGraphInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Goal: 100, DryRun: true}, runner)
	if err != nil {
		t.Fatal(err)
	}
	runner.failSearch = true
	if _, err := BuildPMWorkGraphReport(PMWorkGraphInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Goal: 100, Apply: true, ExpectedPlanID: preview.PlanID}, runner); err == nil || runner.creates != 0 {
		t.Fatalf("reconciliation search failure was not fail-closed: creates=%d err=%v", runner.creates, err)
	}
}

func TestPMWorkGraphApplyFailsClosedWhenProgressReadFails(t *testing.T) {
	source := PMWorkGraphSource{SchemaVersion: PMWorkGraphSourceSchemaVersion, Nodes: []PMWorkGraphNode{{
		ID: "build", Title: "Build bounded slice", Purpose: "Deliver verified behavior", Profile: "delivery", ParentOutcome: "goal:100", Size: "small", Uncertainty: "resolved", Verification: []PMWorkGraphVerification{{Method: "go test", Evidence: "passing tests"}},
	}}}
	runner := newResumableWorkGraphRunner(workGraphGoalBody(t, source))
	preview, err := BuildPMWorkGraphReport(PMWorkGraphInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Goal: 100, DryRun: true}, runner)
	if err != nil {
		t.Fatal(err)
	}
	runner.failComments = true
	if _, err := BuildPMWorkGraphReport(PMWorkGraphInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Goal: 100, Apply: true, ExpectedPlanID: preview.PlanID}, runner); err == nil || runner.creates != 0 {
		t.Fatalf("progress read failure was not fail-closed: creates=%d err=%v", runner.creates, err)
	}
}
