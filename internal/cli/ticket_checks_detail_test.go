package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestTicketChecksDetailJSONAndText(t *testing.T) {
	restoreRunner := devCommandRunner
	restoreNow := ticketChecksNow
	t.Cleanup(func() {
		devCommandRunner = restoreRunner
		ticketChecksNow = restoreNow
	})
	now := time.Date(2026, 9, 28, 0, 1, 0, 0, time.UTC)
	ticketChecksNow = func() time.Time { return now }
	runner := &queueCLIRunner{outputs: map[string][][]byte{
		"gh api repos/StatPan/gira/issues/227/timeline --paginate": {[]byte(cliDetailTimeline)},
		"gh api repos/StatPan/gira/pulls/228":                      {[]byte(cliDetailPull("unstable"))},
		"gh api repos/StatPan/gira/pulls/228/reviews --paginate":   {[]byte(cliDetailReviews)},
		cliDetailChecksKey: {[]byte(`{"check_runs":[` + cliActionsCheck("Build", "in_progress", "", 100, 200) + `]}`)},
		"gh api repos/StatPan/gira/commits/abc123/status": {[]byte(`{"statuses":[]}`)},
		cliDetailRunKey:  {[]byte(cliDetailRunMeta), []byte(cliDetailRunMeta)},
		cliDetailJobsKey: {[]byte(`{"jobs":[` + cliActionsJob(200, 100, 2, "in_progress", "Run tests") + `]}`)},
	}}
	devCommandRunner = runner

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "checks", "227", "--repo", "StatPan/gira", "--detail"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"run #128", "attempt 2", "step 3/8: Run tests", "https://github.com/StatPan/gira/actions/runs/100/job/200"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("checks detail missing %q:\n%s", want, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("checks detail wrote progress to stderr:\n%s", stderr.String())
	}

	stdout.Reset()
	runner.outputs = map[string][][]byte{
		"gh api repos/StatPan/gira/issues/227/timeline --paginate": {[]byte(cliDetailTimeline)},
		"gh api repos/StatPan/gira/pulls/228":                      {[]byte(cliDetailPull("unstable"))},
		"gh api repos/StatPan/gira/pulls/228/reviews --paginate":   {[]byte(cliDetailReviews)},
		cliDetailChecksKey: {[]byte(`{"check_runs":[` + cliActionsCheck("Build", "in_progress", "", 100, 200) + `]}`)},
		"gh api repos/StatPan/gira/commits/abc123/status": {[]byte(`{"statuses":[]}`)},
		cliDetailRunKey:  {[]byte(cliDetailRunMeta), []byte(cliDetailRunMeta)},
		cliDetailJobsKey: {[]byte(`{"jobs":[` + cliActionsJob(200, 100, 2, "in_progress", "Run tests") + `]}`)},
	}
	code = Run([]string{"ticket", "checks", "227", "--repo", "StatPan/gira", "--detail", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("json exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("detail JSON did not decode: %v\n%s", err, stdout.String())
	}
	detail, ok := decoded["detail"].(map[string]any)
	if !ok {
		t.Fatalf("detail JSON missing detail object:\n%s", stdout.String())
	}
	checks, _ := detail["checks"].([]any)
	if len(checks) != 1 {
		t.Fatalf("detail checks = %#v", detail["checks"])
	}
}

func TestTicketWaitDetailProgressSeparatesStreams(t *testing.T) {
	restoreRunner := devCommandRunner
	restoreNow := ticketChecksNow
	restoreSleep := ticketChecksSleep
	t.Cleanup(func() {
		devCommandRunner = restoreRunner
		ticketChecksNow = restoreNow
		ticketChecksSleep = restoreSleep
	})
	now := time.Date(2026, 9, 28, 0, 1, 0, 0, time.UTC)
	ticketChecksNow = func() time.Time { return now }
	ticketChecksSleep = func(d time.Duration) { now = now.Add(d) }
	runner := &queueCLIRunner{outputs: map[string][][]byte{
		"gh api repos/StatPan/gira/issues/227/timeline --paginate": repeatCLI(6, cliDetailTimeline),
		"gh api repos/StatPan/gira/pulls/228":                      append(repeatCLI(5, cliDetailPull("unstable")), []byte(cliDetailPull("clean"))),
		"gh api repos/StatPan/gira/pulls/228/reviews --paginate":   repeatCLI(6, cliDetailReviews),
		"gh api repos/StatPan/gira/commits/abc123/status":          repeatCLI(6, `{"statuses":[]}`),
		cliDetailRunKey: repeatCLI(12, cliDetailRunMeta),
		cliDetailChecksKey: {
			[]byte(`{"check_runs":[` + cliActionsCheck("Build", "in_progress", "", 100, 200) + `]}`),
			[]byte(`{"check_runs":[` + cliActionsCheck("Build", "in_progress", "", 100, 200) + `]}`),
			[]byte(`{"check_runs":[` + cliActionsCheck("Build", "in_progress", "", 100, 200) + `]}`),
			[]byte(`{"check_runs":[` + cliActionsCheck("Build", "in_progress", "", 100, 200) + `]}`),
			[]byte(`{"check_runs":[` + cliActionsCheck("Build", "in_progress", "", 100, 200) + `]}`),
			[]byte(`{"check_runs":[` + cliActionsCheck("Build", "completed", "success", 100, 200) + `]}`),
		},
		cliDetailJobsKey: {
			[]byte(`{"jobs":[` + cliActionsJob(200, 100, 2, "in_progress", "Run tests") + `]}`),
			[]byte(`{"jobs":[` + cliActionsJob(200, 100, 2, "in_progress", "Run tests") + `]}`),
			[]byte(`{"jobs":[` + cliActionsJob(200, 100, 2, "in_progress", "Run tests") + `]}`),
			[]byte(`{"jobs":[` + cliActionsJob(200, 100, 2, "in_progress", "Run tests") + `]}`),
			[]byte(`{"jobs":[` + cliActionsJob(200, 100, 2, "in_progress", "Upload") + `]}`),
			[]byte(`{"jobs":[` + cliActionsJob(200, 100, 2, "completed", "") + `]}`),
		},
	}}
	devCommandRunner = runner

	var stdout, stderr bytes.Buffer
	code := Run([]string{"ticket", "wait", "227", "--repo", "StatPan/gira", "--detail", "--json", "--timeout", "2m", "--interval", "10s"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s\nstdout: %s", code, stderr.String(), stdout.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("wait JSON stdout is not one object: %v\n%s", err, stdout.String())
	}
	if _, ok := decoded["detail"].(map[string]any); !ok {
		t.Fatalf("wait JSON missing detail:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "ticket wait:") {
		t.Fatalf("progress leaked into JSON stdout:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "step 3/8: Run tests") || !strings.Contains(stderr.String(), "still waiting elapsed 30s") || !strings.Contains(stderr.String(), "step 4/8: Upload") {
		t.Fatalf("stderr missing semantic progress:\n%s", stderr.String())
	}
	if strings.Count(stderr.String(), "step 3/8: Run tests") != 1 {
		t.Fatalf("unchanged or elapsed-only snapshot repeated step 3:\n%s", stderr.String())
	}
}

type queueCLIRunner struct {
	outputs map[string][][]byte
	calls   []string
}

func (r *queueCLIRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, key)
	queue := r.outputs[key]
	if len(queue) == 0 {
		return nil, fmt.Errorf("unexpected call: %s", key)
	}
	out := queue[0]
	r.outputs[key] = queue[1:]
	return out, nil
}

const (
	cliDetailTimeline  = `[{"event":"cross-referenced","source":{"issue":{"number":228,"body":"Closes #227","pull_request":{"url":"https://api.github.com/repos/StatPan/gira/pulls/228"}}}}]`
	cliDetailReviews   = `[{"state":"APPROVED","submitted_at":"2026-06-18T09:00:00Z"}]`
	cliDetailRunMeta   = `{"name":"CI","run_number":128,"head_sha":"abc123","html_url":"https://github.com/StatPan/gira/actions/runs/100","workflow_id":42}`
	cliDetailRunKey    = "gh api repos/StatPan/gira/actions/runs/100"
	cliDetailJobsKey   = "gh api repos/StatPan/gira/actions/runs/100/jobs -X GET -f filter=all -f per_page=100 --paginate --slurp"
	cliDetailChecksKey = "gh api repos/StatPan/gira/commits/abc123/check-runs -X GET -f per_page=100 -f filter=all --paginate --slurp"
)

func cliDetailPull(mergeable string) string {
	return `{"number":228,"title":"x","body":"Closes #227","state":"open","html_url":"u","draft":false,"mergeable_state":"` + mergeable + `","head":{"ref":"issue-227-checks","sha":"abc123"},"base":{"ref":"main"}}`
}

func cliActionsCheck(name, status, conclusion string, runID, jobID int) string {
	return fmt.Sprintf(`{"name":%q,"status":%q,"conclusion":%q,"html_url":"https://github.com/StatPan/gira/actions/runs/%d/job/%d","app":{"id":15368,"name":"GitHub Actions","slug":"github-actions"}}`, name, status, conclusion, runID, jobID)
}

func cliActionsJob(jobID, runID, attempt int, status, selected string) string {
	steps := make([]string, 0, 8)
	for number := 1; number <= 8; number++ {
		stepStatus := "completed"
		conclusion := `"success"`
		name := fmt.Sprintf("Step %d", number)
		switch {
		case selected == "Run tests" && number == 3:
			name = "Run tests"
			stepStatus = "in_progress"
			conclusion = "null"
		case selected == "Run tests" && number > 3:
			stepStatus = "queued"
			conclusion = "null"
		case selected == "Upload" && number == 4:
			name = "Upload"
			stepStatus = "in_progress"
			conclusion = "null"
		}
		completed := "null"
		if stepStatus == "completed" {
			completed = `"2026-09-28T00:01:30Z"`
		}
		steps = append(steps, fmt.Sprintf(`{"name":%q,"number":%d,"status":%q,"conclusion":%s,"started_at":"2026-09-28T00:00:10Z","completed_at":%s}`, name, number, stepStatus, conclusion, completed))
	}
	jobConclusion := "null"
	completed := "null"
	if status == "completed" {
		jobConclusion = `"success"`
		completed = `"2026-09-28T00:01:30Z"`
	}
	return fmt.Sprintf(`{"id":%d,"run_id":%d,"run_attempt":%d,"html_url":"https://github.com/StatPan/gira/actions/runs/%d/job/%d","status":%q,"conclusion":%s,"started_at":"2026-09-28T00:00:00Z","completed_at":%s,"steps":[%s]}`, jobID, runID, attempt, runID, jobID, status, jobConclusion, completed, strings.Join(steps, ","))
}

func repeatCLI(n int, raw string) [][]byte {
	out := make([][]byte, n)
	for i := range out {
		out[i] = []byte(raw)
	}
	return out
}
