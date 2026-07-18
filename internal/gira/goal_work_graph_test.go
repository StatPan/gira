package gira

import (
	"encoding/json"
	"fmt"
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
	case strings.HasPrefix(call, `gh issue list --repo OWNER/repo --state all --search repo:OWNER/repo is:issue "Parent: #100"`):
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
