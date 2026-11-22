package gira

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

const separateRunsChecksKey = "gh api repos/StatPan/gira/commits/head123/check-runs -X GET -f per_page=100 -f filter=all --paginate --slurp"

type separateRun struct {
	runID, runNumber, jobID, workflowID int
	event, head, name, status, result   string
}

func (r separateRun) withDefaults() separateRun {
	if r.workflowID == 0 {
		r.workflowID = 42
	}
	if r.event == "" {
		r.event = "pull_request"
	}
	if r.head == "" {
		r.head = "head123"
	}
	if r.name == "" {
		r.name = "Ops"
	}
	if r.status == "" {
		r.status = "completed"
	}
	return r
}

// separateRunOutputs renders one check run per entry, each in its own workflow
// run on the same head unless the entry overrides it.
func separateRunOutputs(runs []separateRun) map[string][]byte {
	outputs := map[string][]byte{}
	rows := make([]string, 0, len(runs))
	for _, run := range runs {
		run = run.withDefaults()
		rows = append(rows, fmt.Sprintf(`{"name":%q,"status":%q,"conclusion":%q,"html_url":"https://github.com/StatPan/gira/actions/runs/%d/job/%d","app":{"id":15368,"slug":"github-actions"}}`,
			run.name, run.status, run.result, run.runID, run.jobID))
		meta := fmt.Sprintf(`{"workflow_id":%d,"head_sha":%q`, run.workflowID, run.head)
		if run.runNumber > 0 {
			meta += fmt.Sprintf(`,"run_number":%d`, run.runNumber)
		}
		if run.event != "-" {
			meta += fmt.Sprintf(`,"event":%q`, run.event)
		}
		outputs[fmt.Sprintf("gh api repos/StatPan/gira/actions/runs/%d", run.runID)] = []byte(meta + "}")
	}
	outputs[separateRunsChecksKey] = []byte(`{"check_runs":[` + strings.Join(rows, ",") + `]}`)
	return outputs
}

func TestRestCheckRunsUseNewestSeparateRunPerCheckIdentity(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	tests := []struct {
		name        string
		runs        []separateRun
		wantChecks  int
		wantState   string
		wantBlocker string
	}{
		{
			name: "failure, cancel, then successes on the same head",
			runs: []separateRun{
				{runID: 100, runNumber: 5, jobID: 10, result: "failure"},
				{runID: 101, runNumber: 6, jobID: 11, result: "cancelled"},
				{runID: 102, runNumber: 7, jobID: 12, result: "success"},
				{runID: 103, runNumber: 8, jobID: 13, result: "success"},
			},
			wantChecks: 1, wantState: "passing",
		},
		{
			name: "newest failure is not replaced by an older success",
			runs: []separateRun{
				{runID: 100, runNumber: 5, jobID: 10, result: "success"},
				{runID: 101, runNumber: 6, jobID: 11, result: "failure"},
			},
			wantChecks: 1, wantState: "failing", wantBlocker: "checks",
		},
		{
			name: "newest pending keeps waiting",
			runs: []separateRun{
				{runID: 100, runNumber: 5, jobID: 10, result: "success"},
				{runID: 101, runNumber: 6, jobID: 11, status: "in_progress"},
			},
			wantChecks: 1, wantState: "pending", wantBlocker: "checks_pending",
		},
		{
			name: "newest cancelled is not replaced by an older success",
			runs: []separateRun{
				{runID: 100, runNumber: 5, jobID: 10, result: "success"},
				{runID: 101, runNumber: 6, jobID: 11, result: "cancelled"},
			},
			wantChecks: 1, wantState: "failing", wantBlocker: "checks",
		},
		{
			name: "different workflows are not merged",
			runs: []separateRun{
				{runID: 100, runNumber: 5, jobID: 10, result: "failure"},
				{runID: 101, runNumber: 6, jobID: 11, result: "success", workflowID: 43},
			},
			wantChecks: 2, wantBlocker: "checks",
		},
		{
			name: "different triggering events are not merged",
			runs: []separateRun{
				{runID: 100, runNumber: 5, jobID: 10, result: "failure", event: "push"},
				{runID: 101, runNumber: 6, jobID: 11, result: "success"},
			},
			wantChecks: 2, wantBlocker: "checks",
		},
		{
			name: "different matrix jobs are not merged",
			runs: []separateRun{
				{runID: 100, runNumber: 5, jobID: 10, result: "failure", name: "test (1.22)"},
				{runID: 101, runNumber: 6, jobID: 11, result: "success", name: "test (1.23)"},
			},
			wantChecks: 2, wantBlocker: "checks",
		},
		{
			name: "missing run number keeps the failure",
			runs: []separateRun{
				{runID: 100, jobID: 10, result: "failure"},
				{runID: 101, runNumber: 6, jobID: 11, result: "success"},
			},
			wantChecks: 2, wantBlocker: "checks",
		},
		{
			name: "missing event keeps the failure",
			runs: []separateRun{
				{runID: 100, runNumber: 5, jobID: 10, result: "failure", event: "-"},
				{runID: 101, runNumber: 6, jobID: 11, result: "success", event: "-"},
			},
			wantChecks: 2, wantBlocker: "checks",
		},
		{
			name: "run number and run id disagreeing keeps the failure",
			runs: []separateRun{
				{runID: 101, runNumber: 5, jobID: 10, result: "failure"},
				{runID: 100, runNumber: 6, jobID: 11, result: "success"},
			},
			wantChecks: 2, wantBlocker: "checks",
		},
		{
			name: "success on another head is not a replacement",
			runs: []separateRun{
				{runID: 100, runNumber: 5, jobID: 10, result: "failure"},
				{runID: 101, runNumber: 6, jobID: 11, result: "success", head: "other-head"},
			},
			wantChecks: 2, wantBlocker: "checks",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			checks := restCheckRunsForCommit(repo, "head123", devPRRunner{outputs: separateRunOutputs(tt.runs), calls: &calls})
			if len(checks) != tt.wantChecks {
				t.Fatalf("effective checks = %+v, want %d", checks, tt.wantChecks)
			}
			if tt.wantState != "" && checks[0].State != tt.wantState {
				t.Fatalf("effective state = %q, want %q: %+v", checks[0].State, tt.wantState, checks)
			}
			blockers := devPRCheckBlockers(checks)
			if (tt.wantBlocker == "" && len(blockers) > 0) || (tt.wantBlocker != "" && !containsString(blockers, tt.wantBlocker)) {
				t.Fatalf("check blockers = %v, want %q", blockers, tt.wantBlocker)
			}
			assertReadOnlyGHCalls(t, calls)
		})
	}
}

func TestLatestWorkflowRunChecksKeepsAmbiguousSameRunDuplicates(t *testing.T) {
	check := func(runID, runNumber int, conclusion, state string) DevPRCheck {
		return DevPRCheck{Name: "test", AppID: 15368, AppSlug: "github-actions", WorkflowID: 42, HeadSHA: "head123",
			Conclusion: conclusion, State: state, runID: runID, runNumber: runNumber, runEvent: "pull_request"}
	}
	// Two same-name checks in the newest run (an unnamed matrix, or attempts
	// whose provenance could not be resolved) make the identity ambiguous.
	checks := []DevPRCheck{
		check(100, 5, "FAILURE", "failing"),
		check(101, 6, "SUCCESS", "passing"),
		check(101, 6, "SUCCESS", "passing"),
	}
	effective := latestWorkflowRunChecks(checks, "head123")
	if len(effective) != 3 || !containsString(devPRCheckBlockers(effective), "checks") {
		t.Fatalf("ambiguous identity must keep every check: %+v", effective)
	}
}

func TestTicketChecksWaitAndFinishAgreeOnNewestSeparateRun(t *testing.T) {
	useFinishReviewPolicy(t, FinishReviewPolicyRequired)
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	const baseSHA = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	outputs := separateRunOutputs([]separateRun{
		{runID: 100, runNumber: 5, jobID: 10, result: "failure"},
		{runID: 101, runNumber: 6, jobID: 11, result: "cancelled"},
		{runID: 102, runNumber: 7, jobID: 12, result: "success"},
		{runID: 103, runNumber: 8, jobID: 13, result: "success"},
	})
	for key, value := range map[string][]byte{
		"gh api repos/StatPan/gira/issues/219":                                                []byte(`{"number":219,"title":"Finish","state":"open","labels":[{"name":"status:in-review"}]}`),
		"gh api repos/StatPan/gira/issues/219/timeline --paginate":                            []byte(`[{"source":{"issue":{"number":220,"pull_request":{"url":"https://api.github.com/repos/StatPan/gira/pulls/220"}}}}]`),
		"gh api repos/StatPan/gira/pulls/220":                                                 []byte(`{"number":220,"body":"Closes #219","state":"open","html_url":"https://github.com/StatPan/gira/pull/220","mergeable_state":"clean","head":{"ref":"issue-219-finish","sha":"head123"},"base":{"ref":"main","sha":"` + baseSHA + `"}}`),
		"gh api repos/StatPan/gira/contents/.gira/config.yaml --method GET -f ref=" + baseSHA: githubContentsFile(".gira/config.yaml", "repo: StatPan/gira\nfinish_review_policy: required\nprofiles:\n  default:\n    labels: []\n"),
		"gh api repos/StatPan/gira/pulls/220/reviews --paginate":                              []byte(`[{"state":"APPROVED","submitted_at":"2026-09-26T09:00:00Z"}]`),
		"gh api repos/StatPan/gira/pulls/220/reviews --paginate --slurp":                      []byte(`[[{"state":"APPROVED","commit_id":"head123"}]]`),
		"gh api repos/StatPan/gira/commits/head123/status":                                    []byte(`{"statuses":[]}`),
	} {
		outputs[key] = value
	}
	var calls []string
	runner := devPRRunner{outputs: outputs, calls: &calls}

	checks, err := BuildTicketChecksReport(repo, 219, TicketChecksOptions{}, runner)
	if err != nil {
		t.Fatal(err)
	}
	if !checks.Ready || containsString(checks.Blockers, "checks") || len(checks.Checks) != 1 {
		t.Fatalf("checks must use the newest run only: ready=%t blockers=%v checks=%+v", checks.Ready, checks.Blockers, checks.Checks)
	}
	waited, err := BuildTicketChecksReport(repo, 219, TicketChecksOptions{Wait: 1, PollInterval: 1, Sleep: func(time.Duration) {}}, runner)
	if err != nil {
		t.Fatal(err)
	}
	if waited.Ready != checks.Ready || strings.Join(waited.Blockers, ",") != strings.Join(checks.Blockers, ",") || len(waited.Checks) != len(checks.Checks) {
		t.Fatalf("wait disagrees with checks: wait=%+v checks=%+v", waited, checks)
	}
	finish, err := FinishWork(repo, 219, true, 0, runner)
	if err != nil {
		t.Fatal(err)
	}
	if !finish.Readiness.Ready || containsString(finish.Blockers, "checks") || finish.Readiness.Checks.Total != 1 ||
		finish.Readiness.Checks.Failing != 0 {
		t.Fatalf("finish --dry-run must agree with checks: %+v", finish.Readiness)
	}
	assertReadOnlyGHCalls(t, calls)
}

func assertReadOnlyGHCalls(t *testing.T, calls []string) {
	t.Helper()
	for _, call := range calls {
		mutating := false
		for _, verb := range []string{"POST", "PATCH", "PUT", "DELETE"} {
			if strings.Contains(call, " -X "+verb) || strings.Contains(call, " --method "+verb) {
				mutating = true
			}
		}
		for _, prefix := range []string{"gh pr merge", "gh pr edit", "gh pr close", "gh pr review", "gh issue edit", "gh issue close", "gh issue comment", "gh run rerun", "gh run cancel", "git push"} {
			if strings.HasPrefix(call, prefix) {
				mutating = true
			}
		}
		if mutating {
			t.Fatalf("readiness must stay read-only, got call %q in %v", call, calls)
		}
	}
}
