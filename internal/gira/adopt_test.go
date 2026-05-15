package gira

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type adoptRunner struct {
	outputs map[string][]byte
	calls   []string
}

func (r *adoptRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, key)
	if out, ok := r.outputs[key]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected call: %s", key)
}

func TestBuildAdoptIssuesReportListsUnmappedIssues(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &adoptRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=open -f per_page=100": []byte(`[[` +
			`{"number":1,"title":"No mapping","state":"open","labels":[],"milestone":null,"html_url":"u1"},` +
			`{"number":2,"title":"Mapped","state":"open","labels":[{"name":"type:task"},{"name":"status:ready"}],"milestone":{"title":"MVP"},"html_url":"u2"},` +
			`{"number":3,"title":"PR","state":"open","pull_request":{},"labels":[],"milestone":null}` +
			`]]`),
	}}

	report, err := BuildAdoptIssuesReport(AdoptIssueInput{Repo: repo, DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildAdoptIssuesReport error: %v", err)
	}
	if report.Counts.Scanned != 2 || report.Counts.Unmapped != 1 || report.Unmapped[0].Number != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	text := FormatAdoptIssuesReport(report)
	if !strings.Contains(text, "missing_milestone") || !strings.Contains(text, "gira adopt issues --repo StatPan/gira --issues 1-3") {
		t.Fatalf("formatted report missing adoption hints:\n%s", text)
	}
}

func TestBuildAdoptIssuesReportAppliesSelectedMapping(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &adoptRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=open -f per_page=100":           []byte(`[[{"number":1,"title":"No mapping","state":"open","labels":[],"milestone":null,"html_url":"u1"}]]`),
		"gh issue edit 1 --repo StatPan/gira --milestone MVP --add-label status:ready --add-label type:task": nil,
	}}

	report, err := BuildAdoptIssuesReport(AdoptIssueInput{Repo: repo, Issues: []int{1}, Milestone: "MVP", Labels: []string{"type:task,status:ready"}, Apply: true}, runner)
	if err != nil {
		t.Fatalf("BuildAdoptIssuesReport error: %v", err)
	}
	if report.Counts.AppliedUpdate != 1 || report.Actions[0].Status != "applied" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if !containsCall(runner.calls, "gh issue edit 1 --repo StatPan/gira --milestone MVP --add-label status:ready --add-label type:task") {
		t.Fatalf("missing issue edit call: %v", runner.calls)
	}
}

func TestBuildAdoptIssuesReportQuotesDynamicNextStepArgs(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &adoptRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=open -f per_page=100": []byte(`[[{"number":1,"title":"No mapping","state":"open","labels":[],"milestone":null,"html_url":"u1"}]]`),
	}}

	report, err := BuildAdoptIssuesReport(AdoptIssueInput{
		Repo:      repo,
		Issues:    []int{1},
		Milestone: "MVP; touch marker",
		Labels:    []string{"type:task$(touch marker)", "status:ready"},
		DryRun:    true,
	}, runner)
	if err != nil {
		t.Fatalf("BuildAdoptIssuesReport error: %v", err)
	}
	for _, want := range []string{
		"--milestone 'MVP; touch marker'",
		"--label status:ready",
		"--label 'type:task$(touch marker)'",
	} {
		if !strings.Contains(report.NextStep, want) {
			t.Fatalf("next step missing quoted arg %q:\n%s", want, report.NextStep)
		}
	}
}

func TestBuildAdoptIssuesReportNormalizesClosedIssueStatus(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &adoptRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100": []byte(`[[` +
			`{"number":10,"title":"Done but active","state":"closed","labels":[{"name":"status:in-progress"},{"name":"type:task"}],"milestone":{"title":"MVP"},"html_url":"u10"},` +
			`{"number":11,"title":"Open active","state":"open","labels":[{"name":"status:in-progress"}],"milestone":{"title":"MVP"},"html_url":"u11"}` +
			`]]`),
		"gh label list --repo StatPan/gira --json name --limit 1000":                                     []byte(`[{"name":"status:done"},{"name":"status:in-progress"}]`),
		"gh issue edit 10 --repo StatPan/gira --add-label status:done --remove-label status:in-progress": nil,
	}}

	report, err := BuildAdoptIssuesReport(AdoptIssueInput{Repo: repo, State: "all", NormalizeStatus: true, Apply: true}, runner)
	if err != nil {
		t.Fatalf("BuildAdoptIssuesReport error: %v", err)
	}
	if report.Counts.AppliedUpdate != 1 || report.Actions[0].Issue != 10 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if !containsCall(runner.calls, "gh issue edit 10 --repo StatPan/gira --add-label status:done --remove-label status:in-progress") {
		t.Fatalf("missing status normalization call: %v", runner.calls)
	}
}

func TestBuildAdoptIssuesReportNormalizesSelectedClosedIssueWithoutDoneLabel(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &adoptRunner{outputs: map[string][]byte{
		"gh api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100": []byte(`[[{"number":12,"title":"Closed blocked","state":"closed","labels":[{"name":"status:blocked"}],"milestone":{"title":"MVP"},"html_url":"u12"}]]`),
		"gh label list --repo StatPan/gira --json name --limit 1000":                              []byte(`[{"name":"status:blocked"}]`),
		"gh issue edit 12 --repo StatPan/gira --remove-label status:blocked":                      nil,
	}}

	report, err := BuildAdoptIssuesReport(AdoptIssueInput{Repo: repo, Issues: []int{12}, State: "all", NormalizeStatus: true, Apply: true}, runner)
	if err != nil {
		t.Fatalf("BuildAdoptIssuesReport error: %v", err)
	}
	if report.Counts.AppliedUpdate != 1 || len(report.Actions[0].Labels) != 0 || len(report.Actions[0].RemoveLabels) != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if !containsCall(runner.calls, "gh issue edit 12 --repo StatPan/gira --remove-label status:blocked") {
		t.Fatalf("missing status cleanup call: %v", runner.calls)
	}
}

func TestBuildAdoptRepoReportPlansMergeWithoutOverwritingExistingAgents(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Custom\n\nKeep this.\n"), 0o644); err != nil {
		t.Fatalf("write AGENTS: %v", err)
	}
	runner := adoptRepoRunner()

	report, err := BuildAdoptRepoReport(AdoptRepoInput{Repo: repo, Path: dir, DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildAdoptRepoReport error: %v", err)
	}
	if report.Strategy != "merge" || report.Recommendation != "merge" {
		t.Fatalf("unexpected strategy: %+v", report)
	}
	if !report.Local.AgentsExists || report.Local.AgentsManagedBlock != "missing" {
		t.Fatalf("unexpected local agents state: %+v", report.Local)
	}
	if !adoptRepoHasAction(report.Actions, "agents:managed-block:insert", "planned") {
		t.Fatalf("missing managed block action: %+v", report.Actions)
	}
	if got := readText(t, filepath.Join(dir, "AGENTS.md")); got != "# Custom\n\nKeep this.\n" {
		t.Fatalf("dry-run changed AGENTS.md: %q", got)
	}
}

func TestBuildAdoptRepoReportQuotesDynamicNextStepPath(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	root := t.TempDir()
	dir := filepath.Join(root, "repo; touch marker")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir unsafe path fixture: %v", err)
	}

	report, err := BuildAdoptRepoReport(AdoptRepoInput{Repo: repo, Path: dir, Strategy: "merge", DryRun: true}, adoptRepoRunner())
	if err != nil {
		t.Fatalf("BuildAdoptRepoReport error: %v", err)
	}
	if !strings.Contains(report.NextStep, "--path '"+dir+"'") {
		t.Fatalf("next step did not quote path:\n%s", report.NextStep)
	}
}

func TestBuildAdoptRepoReportApplyMergeWritesConfigAndManagedBlock(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Custom\n"), 0o644); err != nil {
		t.Fatalf("write AGENTS: %v", err)
	}
	runner := adoptRepoRunner()

	report, err := BuildAdoptRepoReport(AdoptRepoInput{Repo: repo, Path: dir, Strategy: "merge", Apply: true}, runner)
	if err != nil {
		t.Fatalf("BuildAdoptRepoReport error: %v", err)
	}
	if report.Counts.AppliedActions != 2 {
		t.Fatalf("applied actions = %d, want 2: %+v", report.Counts.AppliedActions, report.Actions)
	}
	config := readText(t, filepath.Join(dir, ".gira", "config.yaml"))
	if !strings.Contains(config, "repo: StatPan/gira") || !strings.Contains(config, "profiles:") {
		t.Fatalf("config missing contract fields:\n%s", config)
	}
	agents := readText(t, filepath.Join(dir, "AGENTS.md"))
	if !strings.Contains(agents, "# Custom") || !strings.Contains(agents, "<!-- gira:start -->") || !strings.Contains(agents, "gira ticket finish") {
		t.Fatalf("AGENTS managed block not inserted safely:\n%s", agents)
	}
}

func TestBuildAdoptRepoReportRejectsSymlinkedAgentsWrite(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "AGENTS.md")
	if err := os.WriteFile(outside, []byte("# Outside\n"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	_, err := BuildAdoptRepoReport(AdoptRepoInput{Repo: repo, Path: dir, Strategy: "merge", Apply: true}, adoptRepoRunner())
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
	if got := readText(t, outside); got != "# Outside\n" {
		t.Fatalf("outside file changed through symlink: %q", got)
	}
}

func TestBuildAdoptRepoReportApplyRequiresStrategyOrYes(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	_, err := BuildAdoptRepoReport(AdoptRepoInput{Repo: repo, Path: t.TempDir(), Apply: true}, adoptRepoRunner())
	if err == nil || !strings.Contains(err.Error(), "--apply requires --strategy") {
		t.Fatalf("expected apply strategy error, got %v", err)
	}
}

func adoptRepoRunner() *adoptRunner {
	return &adoptRunner{outputs: map[string][]byte{
		"gh label list --repo StatPan/gira --json name,color,description --limit 1000":                []byte(`[{"name":"bug","color":"d73a4a","description":"Bug"}]`),
		"gh api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100": []byte(`[[{"number":1,"title":"Roadmap","description":"Existing","due_on":null}]]`),
		"gh api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=open -f per_page=100": []byte(`[[` +
			`{"number":1,"title":"Legacy issue","state":"open","labels":[{"name":"bug"}],"milestone":null,"html_url":"u1"},` +
			`{"number":2,"title":"Mapped issue","state":"open","labels":[{"name":"type:task"},{"name":"status:ready"}],"milestone":{"title":"Roadmap"},"html_url":"u2"}` +
			`]]`),
		"gh project list --owner StatPan --format json --limit 100": []byte(`{"projects":[{"title":"Gira Backlog","number":1}]}`),
	}}
}

func adoptRepoHasAction(actions []AdoptRepoAction, action string, status string) bool {
	for _, item := range actions {
		if item.Action == action && item.Status == status {
			return true
		}
	}
	return false
}

func readText(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
