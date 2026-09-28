package gira

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

const (
	detailTimeline  = `[{"event":"cross-referenced","source":{"issue":{"number":228,"body":"Closes #227","pull_request":{"url":"https://api.github.com/repos/StatPan/gira/pulls/228"}}}}]`
	detailReviews   = `[{"state":"APPROVED","submitted_at":"2026-06-18T09:00:00Z"}]`
	detailStatus    = `{"statuses":[]}`
	detailRunMeta   = `{"name":"CI","run_number":128,"head_sha":"abc123","html_url":"https://github.com/StatPan/gira/actions/runs/100","workflow_id":42}`
	detailRunKey    = "gh api repos/StatPan/gira/actions/runs/100"
	detailJobsKey   = "gh api repos/StatPan/gira/actions/runs/100/jobs -X GET -f filter=all -f per_page=100 --paginate --slurp"
	detailChecksKey = "gh api repos/StatPan/gira/commits/abc123/check-runs -X GET -f per_page=100 -f filter=all --paginate --slurp"
)

func TestBuildTicketChecksReportDetailKeepsAttemptPerJob(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	jobs := `{"jobs":[` + actionsJobJSON(200, 100, 1, "in_progress", "", "Run tests", "in_progress", "") + `,` + actionsJobJSON(201, 100, 2, "in_progress", "", "Upload", "in_progress", "") + `,` + actionsJobJSON(202, 100, 3, "queued", "", "", "queued", "") + `]}`
	runner := detailRunner(1, detailPullJSON("unstable"), []string{detailCheckRuns(actionsCheckJSON("test", "in_progress", "", 100, 200), actionsCheckJSON("test", "in_progress", "", 100, 201))}, []string{jobs}, nil)
	now := time.Date(2026, 9, 28, 0, 1, 0, 0, time.UTC)
	report, err := BuildTicketChecksReport(repo, 227, TicketChecksOptions{Detail: true, Now: func() time.Time { return now }}, runner)
	if err != nil {
		t.Fatalf("BuildTicketChecksReport error: %v", err)
	}
	if report.Ready || !containsString(report.Blockers, "checks_pending") {
		t.Fatalf("detail changed readiness: ready=%t blockers=%v", report.Ready, report.Blockers)
	}
	if report.Detail == nil || len(report.Detail.Checks) != 2 {
		t.Fatalf("detail rows = %+v", report.Detail)
	}
	byJob := map[int64]TicketCheckDetail{}
	for _, detail := range report.Detail.Checks {
		byJob[detail.JobID] = detail
	}
	if byJob[200].RunAttempt != 1 || byJob[200].Step == nil || byJob[200].Step.Name != "Run tests" || byJob[200].Step.Number != 3 || byJob[200].RunNumber != 128 || byJob[200].RunID != 100 {
		t.Fatalf("attempt 1 row mixed identities: %+v", byJob[200])
	}
	if byJob[201].RunAttempt != 2 || byJob[201].Step == nil || byJob[201].Step.Name != "Upload" || byJob[201].Step.Number != 4 {
		t.Fatalf("attempt 2 row mixed identities: %+v", byJob[201])
	}
	text := FormatTicketChecks(report)
	for _, want := range []string{"run #128", "attempt 1", "attempt 2", "run 100", "step 3/8: Run tests", "step 4/8: Upload", "https://github.com/StatPan/gira/actions/runs/100/job/200", "https://github.com/StatPan/gira/actions/runs/100/job/201"} {
		if !strings.Contains(text, want) {
			t.Fatalf("detail text missing %q:\n%s", want, text)
		}
	}
	for _, banned := range []string{"attempt 128", "attempt 100", "run #100", "run #2"} {
		if strings.Contains(text, banned) {
			t.Fatalf("detail text confused run number with attempt or id %q:\n%s", banned, text)
		}
	}
	if calls := countCalls(runner.calls, detailJobsKey); calls != 1 {
		t.Fatalf("jobs calls = %d, want 1 for one shared run: %v", calls, runner.calls)
	}
	if calls := countCalls(runner.calls, detailRunKey); calls != 2 {
		t.Fatalf("run metadata calls = %d, want 2 (readiness + one detail fetch): %v", calls, runner.calls)
	}
}

func TestBuildTicketChecksReportWaitDetailEmitsSemanticProgress(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	step3 := actionsJobJSON(200, 100, 2, "in_progress", "", "Run tests", "in_progress", "")
	step4 := actionsJobJSON(200, 100, 2, "in_progress", "", "Upload", "in_progress", "")
	done := actionsJobJSON(200, 100, 2, "completed", "success", "", "completed", "success")
	checkRuns := []string{
		detailCheckRuns(actionsCheckJSON("Build", "in_progress", "", 100, 200)),
		detailCheckRuns(actionsCheckJSON("Build", "in_progress", "", 100, 200)),
		detailCheckRuns(actionsCheckJSON("Build", "in_progress", "", 100, 200)),
		detailCheckRuns(actionsCheckJSON("Build", "in_progress", "", 100, 200)),
		detailCheckRuns(actionsCheckJSON("Build", "in_progress", "", 100, 200)),
		detailCheckRuns(actionsCheckJSON("Build", "completed", "success", 100, 200)),
	}
	jobs := []string{
		`{"jobs":[` + step3 + `]}`,
		`{"jobs":[` + step3 + `]}`,
		`{"jobs":[` + step3 + `]}`,
		`{"jobs":[` + step3 + `]}`,
		`{"jobs":[` + step4 + `]}`,
		`{"jobs":[` + done + `]}`,
	}
	runner := detailRunner(6, detailPullJSON("unstable"), checkRuns, jobs, nil)
	runner.outputs["gh api repos/StatPan/gira/pulls/228"][5] = []byte(detailPullJSON("clean"))
	now := time.Date(2026, 9, 28, 0, 1, 0, 0, time.UTC)
	var progress *TicketWaitProgress
	var texts []string
	options := TicketChecksOptions{
		Wait:         2 * time.Minute,
		PollInterval: 10 * time.Second,
		Detail:       true,
		Now:          func() time.Time { return now },
		Sleep:        func(d time.Duration) { now = now.Add(d) },
		OnProgress: func(report TicketChecksReport) {
			if progress == nil {
				progress = NewTicketWaitProgress(now, 2*time.Minute)
			}
			if text := progress.Observe(report, now); text != "" {
				texts = append(texts, text)
			}
		},
	}
	report, err := BuildTicketChecksReport(repo, 227, options, runner)
	if err != nil {
		t.Fatalf("BuildTicketChecksReport error: %v", err)
	}
	if !report.Ready || len(report.Blockers) != 0 {
		t.Fatalf("final readiness = ready:%t blockers:%v", report.Ready, report.Blockers)
	}
	if len(texts) != 4 {
		t.Fatalf("progress events = %d, want initial, heartbeat, step 4, completed:\n%s", len(texts), strings.Join(texts, "\n---\n"))
	}
	if !strings.Contains(texts[0], "ticket wait: elapsed 0s remaining 2m0s") || !strings.Contains(texts[0], "step 3/8: Run tests") {
		t.Fatalf("initial progress = %q", texts[0])
	}
	if !strings.Contains(texts[1], "ticket wait: still waiting elapsed 30s remaining 1m30s") || strings.Contains(texts[1], "Run tests") || strings.Contains(texts[1], "step ") {
		t.Fatalf("heartbeat repeated a row or missed the clock: %q", texts[1])
	}
	if !strings.Contains(texts[2], "step 4/8: Upload") || strings.Contains(texts[2], "still waiting") {
		t.Fatalf("step change progress = %q", texts[2])
	}
	if !strings.Contains(texts[3], "success") || strings.Contains(texts[3], "step ") {
		t.Fatalf("completed progress invented a current step: %q", texts[3])
	}
	if calls := countCalls(runner.calls, detailJobsKey); calls != 6 {
		t.Fatalf("jobs calls = %d, want one per snapshot: %v", calls, runner.calls)
	}
}

func TestBuildTicketChecksReportDetailStatesStayTruthful(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	now := time.Date(2026, 9, 28, 0, 2, 0, 0, time.UTC)
	options := TicketChecksOptions{Detail: true, Now: func() time.Time { return now }}

	t.Run("queued", func(t *testing.T) {
		runner := detailRunner(1, detailPullJSON("unstable"), []string{detailCheckRuns(actionsCheckJSON("Build", "queued", "", 100, 200))}, []string{`{"jobs":[` + actionsJobJSON(200, 100, 2, "queued", "", "", "", "") + `]}`}, nil)
		report, err := BuildTicketChecksReport(repo, 227, options, runner)
		if err != nil {
			t.Fatal(err)
		}
		text := FormatTicketChecks(report)
		if report.Detail.Checks[0].Step != nil || !strings.Contains(text, "\n  queued https://github.com/StatPan/gira/actions/runs/100/job/200\n") {
			t.Fatalf("queued detail invented a step:\n%s\n%+v", text, report.Detail.Checks[0])
		}
	})

	t.Run("failed step before cleanup", func(t *testing.T) {
		job := `{"id":200,"run_id":100,"run_attempt":2,"html_url":"https://github.com/StatPan/gira/actions/runs/100/job/200","status":"completed","conclusion":"failure","started_at":"2026-09-28T00:00:00Z","completed_at":"2026-09-28T00:01:00Z","steps":[` +
			`{"name":"Checkout","number":1,"status":"completed","conclusion":"success","started_at":"2026-09-28T00:00:00Z","completed_at":"2026-09-28T00:00:05Z"},` +
			`{"name":"Run tests","number":2,"status":"completed","conclusion":"failure","started_at":"2026-09-28T00:00:05Z","completed_at":"2026-09-28T00:00:20Z"},` +
			`{"name":"Cleanup","number":3,"status":"completed","conclusion":"success","started_at":"2026-09-28T00:00:20Z","completed_at":"2026-09-28T00:01:00Z"}]}`
		runner := detailRunner(1, detailPullJSON("unstable"), []string{detailCheckRuns(actionsCheckJSON("Build", "completed", "failure", 100, 200))}, []string{`{"jobs":[` + job + `]}`}, nil)
		report, err := BuildTicketChecksReport(repo, 227, options, runner)
		if err != nil {
			t.Fatal(err)
		}
		if report.Ready || !containsString(report.Blockers, "checks") {
			t.Fatalf("failure readiness changed: %+v", report.Blockers)
		}
		text := FormatTicketChecks(report)
		if !strings.Contains(text, "failure failed step 2/3: Run tests") || strings.Contains(text, "Cleanup") {
			t.Fatalf("failed step selection:\n%s", text)
		}
	})

	t.Run("superseded cancellation", func(t *testing.T) {
		checks := `{"check_runs":[` +
			`{"name":"Build","status":"completed","conclusion":"cancelled","completed_at":"2026-09-28T00:00:00Z","html_url":"https://github.com/StatPan/gira/actions/runs/100/job/200","app":{"id":15368,"name":"GitHub Actions","slug":"github-actions"}},` +
			`{"name":"Build","status":"completed","conclusion":"success","completed_at":"2026-09-28T00:05:00Z","html_url":"https://github.com/StatPan/gira/actions/runs/101/job/201","app":{"id":15368,"name":"GitHub Actions","slug":"github-actions"}}]}`
		runner := detailRunner(1, detailPullJSON("clean"), []string{checks}, []string{`{"jobs":[` + actionsJobJSON(200, 100, 1, "completed", "cancelled", "Run tests", "completed", "failure") + `]}`}, nil)
		runner.outputs["gh api repos/StatPan/gira/actions/runs/101"] = repeatPayload(2, `{"name":"CI","run_number":129,"head_sha":"abc123","html_url":"https://github.com/StatPan/gira/actions/runs/101","workflow_id":42}`)
		runner.outputs["gh api repos/StatPan/gira/actions/runs/101/jobs -X GET -f filter=all -f per_page=100 --paginate --slurp"] = [][]byte{[]byte(`{"jobs":[` + actionsJobJSON(201, 101, 1, "completed", "success", "", "completed", "success") + `]}`)}
		report, err := BuildTicketChecksReport(repo, 227, options, runner)
		if err != nil {
			t.Fatal(err)
		}
		var cancelled DevPRCheck
		for _, check := range report.Checks {
			if check.Superseded {
				cancelled = check
			}
		}
		if !cancelled.Superseded || cancelled.State != "passing" {
			t.Fatalf("superseded classification changed: %+v", report.Checks)
		}
		text := FormatTicketChecks(report)
		if !strings.Contains(text, "cancelled (superseded)") || strings.Contains(text, "failed step") {
			t.Fatalf("superseded detail:\n%s", text)
		}
	})

	t.Run("unsupported external", func(t *testing.T) {
		runner := detailRunner(1, detailPullJSON("unstable"), []string{detailCheckRuns(`{"name":"External","status":"in_progress","conclusion":"","html_url":"https://ci.example/check/9","app":{"name":"Go release"}}`)}, nil, nil)
		report, err := BuildTicketChecksReport(repo, 227, options, runner)
		if err != nil {
			t.Fatal(err)
		}
		text := FormatTicketChecks(report)
		if report.Detail.Checks[0].Availability != "unsupported" || !strings.Contains(text, "step detail unsupported https://ci.example/check/9") {
			t.Fatalf("unsupported detail:\n%s\n%+v", text, report.Detail.Checks[0])
		}
		if countCalls(runner.calls, detailJobsKey) != 0 {
			t.Fatalf("unsupported check fetched jobs: %v", runner.calls)
		}
	})

	t.Run("sha and job mismatch", func(t *testing.T) {
		runner := detailRunner(1, detailPullJSON("unstable"), []string{detailCheckRuns(actionsCheckJSON("Build", "in_progress", "", 100, 200), actionsCheckJSON("Other", "in_progress", "", 100, 999))}, []string{`{"jobs":[` + actionsJobJSON(200, 100, 2, "in_progress", "", "Run tests", "in_progress", "") + `]}`}, nil)
		runner.outputs[detailRunKey] = repeatPayload(2, `{"name":"CI","run_number":128,"head_sha":"different","html_url":"https://github.com/StatPan/gira/actions/runs/100","workflow_id":42}`)
		report, err := BuildTicketChecksReport(repo, 227, options, runner)
		if err != nil {
			t.Fatal(err)
		}
		for _, detail := range report.Detail.Checks {
			if detail.Availability != "unavailable" || detail.Reason != "run head does not match pull request head" || detail.RunAttempt != 0 || detail.Step != nil {
				t.Fatalf("sha mismatch invented job data: %+v", detail)
			}
		}
		if !strings.Contains(FormatTicketChecks(report), "detail unavailable: run head does not match pull request head") {
			t.Fatalf("sha mismatch text:\n%s", FormatTicketChecks(report))
		}
	})

	t.Run("permission malformed empty and missing job", func(t *testing.T) {
		cases := []struct {
			name    string
			jobs    string
			jobsErr error
			reason  string
		}{
			{name: "permission", jobsErr: fmt.Errorf("permission denied"), reason: "jobs unavailable"},
			{name: "malformed", jobs: "not-json", reason: "jobs response malformed"},
			{name: "empty", jobs: `{"jobs":[]}`, reason: "jobs response empty"},
			{name: "missing", jobs: `{"jobs":[{"id":1,"run_id":100,"run_attempt":9}]}`, reason: "job not found"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				var errs map[string]error
				var jobs []string
				if tc.jobsErr != nil {
					errs = map[string]error{detailJobsKey: tc.jobsErr}
				} else {
					jobs = []string{tc.jobs}
				}
				runner := detailRunner(1, detailPullJSON("clean"), []string{detailCheckRuns(actionsCheckJSON("Build", "completed", "success", 100, 200))}, jobs, errs)
				report, err := BuildTicketChecksReport(repo, 227, options, runner)
				if err != nil {
					t.Fatal(err)
				}
				if !report.Ready || report.Detail.Checks[0].Availability != "unavailable" || report.Detail.Checks[0].Reason != tc.reason || report.Detail.Checks[0].Step != nil || report.Detail.Checks[0].RunAttempt != 0 {
					t.Fatalf("unavailable detail = ready:%t %+v", report.Ready, report.Detail.Checks[0])
				}
				if !strings.Contains(FormatTicketChecks(report), "detail unavailable: "+tc.reason) {
					t.Fatalf("text missing reason %s:\n%s", tc.reason, FormatTicketChecks(report))
				}
			})
		}
	})

	t.Run("paginated jobs", func(t *testing.T) {
		page1 := make([]string, 100)
		for i := range page1 {
			page1[i] = fmt.Sprintf(`{"id":%d,"run_id":100,"run_attempt":1,"status":"completed","conclusion":"success"}`, i+1)
		}
		page2 := `{"jobs":[` + actionsJobJSON(200, 100, 2, "in_progress", "", "Run tests", "in_progress", "") + `]}`
		slurp := `[{"jobs":[` + strings.Join(page1, ",") + `]},` + page2 + `]`
		runner := detailRunner(1, detailPullJSON("unstable"), []string{detailCheckRuns(actionsCheckJSON("Build", "in_progress", "", 100, 200))}, []string{slurp}, nil)
		report, err := BuildTicketChecksReport(repo, 227, options, runner)
		if err != nil {
			t.Fatal(err)
		}
		if report.Detail.Checks[0].Availability != "available" || report.Detail.Checks[0].RunAttempt != 2 || report.Detail.Checks[0].Step == nil || report.Detail.Checks[0].Step.Name != "Run tests" {
			t.Fatalf("page 2 job not matched: %+v", report.Detail.Checks[0])
		}
		if countCalls(runner.calls, detailJobsKey) != 1 {
			t.Fatalf("paginated jobs were fetched per page or per check: %v", runner.calls)
		}
	})
}

func TestBuildTicketChecksReportOrdinaryOmitsDetail(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := detailRunner(1, detailPullJSON("unstable"), []string{detailCheckRuns(`{"name":"Build linux","status":"in_progress","conclusion":"","html_url":"https://example.test/check","app":{"name":"Go release"}}`)}, nil, nil)
	report, err := BuildTicketChecksReport(repo, 227, TicketChecksOptions{}, runner)
	if err != nil {
		t.Fatal(err)
	}
	if report.Detail != nil {
		t.Fatalf("ordinary report included detail: %+v", report.Detail)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"detail"`) {
		t.Fatalf("ordinary JSON included detail: %s", encoded)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "/jobs") || strings.Contains(call, "/actions/runs/") {
			t.Fatalf("ordinary checks fetched detail API %s", call)
		}
	}
	if strings.Contains(FormatTicketChecks(report), "detail unavailable") || strings.Contains(FormatTicketChecks(report), "run #") {
		t.Fatalf("ordinary text changed:\n%s", FormatTicketChecks(report))
	}
}

func detailRunner(snapshots int, pull string, checkRuns []string, jobs []string, errs map[string]error) *finishRunner {
	if errs == nil {
		errs = map[string]error{}
	}
	outputs := map[string][][]byte{
		"gh api repos/StatPan/gira/issues/227/timeline --paginate": repeatPayload(snapshots, detailTimeline),
		"gh api repos/StatPan/gira/pulls/228":                      repeatPayload(snapshots, pull),
		"gh api repos/StatPan/gira/pulls/228/reviews --paginate":   repeatPayload(snapshots, detailReviews),
		detailChecksKey: repeatPayload(snapshots, checkRuns[0]),
		"gh api repos/StatPan/gira/commits/abc123/status": repeatPayload(snapshots, detailStatus),
		detailRunKey: repeatPayload(snapshots*2, detailRunMeta),
	}
	if len(checkRuns) > 0 {
		outputs[detailChecksKey] = payloadList(checkRuns)
	}
	if len(jobs) > 0 {
		outputs[detailJobsKey] = payloadList(jobs)
	}
	return &finishRunner{outputs: outputs, errs: errs}
}

func payloadList(items []string) [][]byte {
	out := make([][]byte, len(items))
	for i, item := range items {
		out[i] = []byte(item)
	}
	return out
}

func repeatPayload(n int, raw string) [][]byte {
	out := make([][]byte, n)
	for i := range out {
		out[i] = []byte(raw)
	}
	return out
}

func detailPullJSON(mergeable string) string {
	return `{"number":228,"title":"x","body":"Closes #227","state":"open","html_url":"u","draft":false,"mergeable_state":"` + mergeable + `","head":{"ref":"issue-227-checks","sha":"abc123"},"base":{"ref":"main"}}`
}

func detailCheckRuns(checks ...string) string {
	return `{"check_runs":[` + strings.Join(checks, ",") + `]}`
}

func actionsCheckJSON(name, status, conclusion string, runID, jobID int) string {
	return fmt.Sprintf(`{"name":%q,"status":%q,"conclusion":%q,"html_url":"https://github.com/StatPan/gira/actions/runs/%d/job/%d","app":{"id":15368,"name":"GitHub Actions","slug":"github-actions"}}`, name, status, conclusion, runID, jobID)
}

func actionsJobJSON(jobID, runID, attempt int, status, conclusion, selectedName, selectedStatus, selectedConclusion string) string {
	steps := make([]string, 0, 8)
	for number := 1; number <= 8; number++ {
		stepStatus := "completed"
		stepConclusion := "success"
		name := fmt.Sprintf("Step %d", number)
		switch {
		case selectedName == "Run tests" && number == 3:
			name = "Run tests"
			stepStatus = selectedStatus
			stepConclusion = selectedConclusion
		case selectedName == "Run tests" && number > 3:
			stepStatus = "queued"
			stepConclusion = ""
		case selectedName == "Upload" && number == 4:
			name = "Upload"
			stepStatus = "in_progress"
			stepConclusion = ""
		}
		steps = append(steps, fmt.Sprintf(`{"name":%q,"number":%d,"status":%q,"conclusion":%s,"started_at":"2026-09-28T00:00:10Z","completed_at":%s}`, name, number, stepStatus, jsonStringOrNull(stepConclusion), completedOrNull(stepStatus)))
	}
	if selectedName == "" {
		steps = []string{}
	}
	stepJSON := "[]"
	if len(steps) > 0 {
		stepJSON = "[" + strings.Join(steps, ",") + "]"
	}
	return fmt.Sprintf(`{"id":%d,"run_id":%d,"run_attempt":%d,"html_url":"https://github.com/StatPan/gira/actions/runs/%d/job/%d","status":%q,"conclusion":%s,"started_at":"2026-09-28T00:00:00Z","completed_at":%s,"steps":%s}`, jobID, runID, attempt, runID, jobID, status, jsonStringOrNull(conclusion), completedOrNull(status), stepJSON)
}

func jsonStringOrNull(value string) string {
	if value == "" {
		return "null"
	}
	return strconvQuote(value)
}

func completedOrNull(status string) string {
	if status == "completed" {
		return `"2026-09-28T00:01:30Z"`
	}
	return "null"
}

func strconvQuote(value string) string {
	return fmt.Sprintf("%q", value)
}

func countCalls(calls []string, want string) int {
	count := 0
	for _, call := range calls {
		if call == want {
			count++
		}
	}
	return count
}
