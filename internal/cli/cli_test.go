package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/StatPan/gira/internal/gira"
)

func TestHelpOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "GitHub-native project OS bootstrapper") {
		t.Fatalf("help output missing product description:\n%s", stdout.String())
	}
}

func TestBootstrapRequiresRepo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"bootstrap", "--dry-run"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "--repo is required") {
		t.Fatalf("stderr missing repo requirement:\n%s", stderr.String())
	}
}

func TestBootstrapNonDryRunRequiresPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"bootstrap", "--repo", "StatPan/example"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "--path is required") {
		t.Fatalf("stderr missing path requirement:\n%s", stderr.String())
	}
}

func TestBootstrapNonDryRunRequiresGitRepo(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"bootstrap",
		"--repo",
		"StatPan/example",
		"--path",
		dir,
		"--created-at",
		"2026-04-26",
	}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "not a git repository") {
		t.Fatalf("stderr missing git repo requirement:\n%s", stderr.String())
	}
}

func TestBootstrapNonDryRunCanInstallWithoutBranch(t *testing.T) {
	repo := t.TempDir()
	runCLIGit(t, repo, "init", "-q", "-b", "main")

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"bootstrap",
		"--repo",
		"StatPan/example",
		"--path",
		repo,
		"--no-branch",
		"--created-at",
		"2026-04-26",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "branch:") {
		t.Fatalf("stdout unexpectedly included branch line:\n%s", stdout.String())
	}
	if _, err := os.Stat(repo + "/AGENTS.md"); err != nil {
		t.Fatalf("AGENTS.md was not created: %v", err)
	}
}

func TestBootstrapDryRunIsDeterministic(t *testing.T) {
	args := []string{
		"bootstrap",
		"--repo",
		"StatPan/example",
		"--template",
		"default",
		"--dry-run",
		"--created-at",
		"2026-04-26",
	}

	var firstOut, firstErr, secondOut, secondErr bytes.Buffer
	firstCode := Run(args, &firstOut, &firstErr)
	secondCode := Run(args, &secondOut, &secondErr)

	if firstCode != 0 || secondCode != 0 {
		t.Fatalf("exit codes = %d/%d, want 0/0; stderr: %s%s", firstCode, secondCode, firstErr.String(), secondErr.String())
	}
	if firstOut.String() != secondOut.String() {
		t.Fatal("dry-run output differed between identical runs")
	}
	output := firstOut.String()
	for _, want := range []string{"--- AGENTS.md", "StatPan/example", "example", "2026-04-26", "--- .github/PULL_REQUEST_TEMPLATE.md"} {
		if !strings.Contains(output, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, output)
		}
	}
}

func TestStatusJSONUsesInjectedClient(t *testing.T) {
	restoreClient, restoreNow := newStatusClient, statusNow
	t.Cleanup(func() {
		newStatusClient = restoreClient
		statusNow = restoreNow
	})
	newStatusClient = func(repo gira.RepoRef) gira.StatusClient {
		return cliFakeStatusClient{
			repo: repo,
			responses: map[string]string{
				"api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100": `[[{"number":1,"title":"MVP","state":"open","description":"","due_on":null,"open_issues":1,"closed_issues":1}]]`,
				"api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100":     `[[{"number":1,"title":"Issue 1","state":"open","labels":[],"milestone":null,"updated_at":"2026-04-25T12:00:00Z","html_url":"https://github.com/StatPan/gira/issues/1"}]]`,
			},
		}
	}
	statusNow = func() time.Time {
		return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"status", "--repo", "StatPan/gira", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("status JSON did not parse: %v\n%s", err, stdout.String())
	}
	if payload["repo"] != "StatPan/gira" {
		t.Fatalf("repo = %v, want StatPan/gira", payload["repo"])
	}
}

func TestSyncDryRunUsesInjectedClientWithoutApplying(t *testing.T) {
	restoreClient := newSyncClient
	t.Cleanup(func() {
		newSyncClient = restoreClient
	})
	client := &cliFakeSyncClient{
		repo: mustCLIRepo(t, "StatPan/gira"),
		labels: []gira.ExistingLabel{
			{Name: gira.BootstrapLabel, Color: "5319E7", Description: "Created or managed by Gira bootstrap metadata sync."},
		},
	}
	newSyncClient = func(repo gira.RepoRef) gira.SyncClient {
		client.repo = repo
		return client
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"sync", "--repo", "StatPan/gira", "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"sync plan:", "would create", "bootstrap issues: 0 would create", "create label:"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("sync dry-run output missing %q:\n%s", want, stdout.String())
		}
	}
	if len(client.calls) != 0 {
		t.Fatalf("dry-run applied calls: %v", client.calls)
	}
}

func TestSyncDryRunWithBootstrapIssuesFlagIncludesIssueCreates(t *testing.T) {
	restoreClient := newSyncClient
	t.Cleanup(func() {
		newSyncClient = restoreClient
	})
	client := &cliFakeSyncClient{
		repo: mustCLIRepo(t, "StatPan/gira"),
		labels: []gira.ExistingLabel{
			{Name: gira.BootstrapLabel, Color: "5319E7", Description: "Created or managed by Gira bootstrap metadata sync."},
		},
	}
	newSyncClient = func(repo gira.RepoRef) gira.SyncClient {
		client.repo = repo
		return client
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"sync", "--repo", "StatPan/gira", "--dry-run", "--bootstrap-issues"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"sync plan:", "bootstrap issues:", "create issue:"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("sync dry-run output missing %q:\n%s", want, stdout.String())
		}
	}
	if len(client.calls) != 0 {
		t.Fatalf("dry-run applied calls: %v", client.calls)
	}
}

func TestSyncApplyUsesInjectedClientAndPrintsComplete(t *testing.T) {
	restoreClient := newSyncClient
	t.Cleanup(func() {
		newSyncClient = restoreClient
	})
	client := &cliFakeSyncClient{
		repo: mustCLIRepo(t, "StatPan/gira"),
		labels: []gira.ExistingLabel{
			{Name: gira.BootstrapLabel, Color: "000000", Description: "Old description."},
		},
		milestones: []gira.ExistingMilestone{
			{Number: 1, Title: "MVP", Description: "CLI-first Gira bootstrapper with templates and GitHub metadata sync."},
			{Number: 2, Title: "Beta", Description: "Broader validation and hardening after the MVP workflow is usable."},
			{Number: 3, Title: "v1", Description: "Stable first release of the GitHub-native project OS workflow."},
		},
		issues: []gira.ExistingIssue{
			{Number: 1, Title: "[Epic] Gira MVP: GitHub-as-OS bootstrap", Labels: []string{gira.BootstrapLabel}},
			{Number: 2, Title: "[Task] Slice 1: CLI skeleton + template dry-run", Labels: []string{gira.BootstrapLabel}},
			{Number: 3, Title: "[Task] Slice 2: idempotent repo file install", Labels: []string{gira.BootstrapLabel}},
			{Number: 4, Title: "[Task] Slice 3: labels/milestones/bootstrap-issues sync", Labels: []string{gira.BootstrapLabel}},
			{Number: 5, Title: "[Task] Slice 4: gira status (text + --json)", Labels: []string{gira.BootstrapLabel}},
		},
	}
	newSyncClient = func(repo gira.RepoRef) gira.SyncClient {
		client.repo = repo
		return client
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"sync", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "sync complete") {
		t.Fatalf("sync apply output missing completion:\n%s", stdout.String())
	}
	if len(client.calls) == 0 || client.calls[0] != "update label "+gira.BootstrapLabel {
		t.Fatalf("apply calls = %v, want first call to update bootstrap label", client.calls)
	}
}

func TestExportDashboardCommandRequiresRepo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"export", "dashboard", "--dry-run"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "--repo is required") {
		t.Fatalf("stderr missing repo requirement:\n%s", stderr.String())
	}
}

func TestExportDashboardCommandRejectsInvalidRepoFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"export", "dashboard", "--repo", "bad-format", "--dry-run"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "OWNER/REPO") {
		t.Fatalf("stderr missing parse error for invalid repo:\n%s", stderr.String())
	}
}

func TestExportDashboardCommandRejectsPositionalArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"export", "dashboard", "--repo", "StatPan/gira", "--dry-run", "unexpected"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "unexpected argument") {
		t.Fatalf("stderr missing unexpected argument error:\n%s", stderr.String())
	}
}

func TestExportDashboardDryRunRequiresDeterministicOutput(t *testing.T) {
	restoreClient, restoreNow := newDashboardExportClient, dashboardExportNow
	t.Cleanup(func() {
		newDashboardExportClient = restoreClient
		dashboardExportNow = restoreNow
	})
	client := &cliFakeDashboardExportClient{
		repo: mustCLIRepo(t, "StatPan/gira"),
		issues: []gira.DashboardRawIssue{
			{IssueNumber: 2, Title: "Second", State: "open", Labels: []string{"status:ready"}},
			{IssueNumber: 1, Title: "First", State: "open", Labels: []string{"status:blocked"}},
		},
		pulls: []gira.DashboardRawPullRequest{
			{PullRequestNumber: 11, Title: "PR", State: "closed", Labels: []string{"priority:p1"}},
		},
		milestones: []gira.DashboardRawMilestone{
			{MilestoneNumber: 3, Title: "Later", State: "closed"},
			{MilestoneNumber: 1, Title: "First", State: "open"},
		},
		projectSync: gira.ProjectSyncSnapshot{
			RoadmapItems: []gira.ProjectRoadmapItem{
				{IssueNumber: 10, IssueTitle: "Roadmap", TypeLabel: "type:epic", IssueURL: "https://github.com/StatPan/gira/issues/10", Roadmapable: true, StartDate: strPtr("2026-04-10"), TargetDate: strPtr("2026-04-20")},
				{IssueNumber: 9, IssueTitle: "Another", TypeLabel: "type:task", IssueURL: "https://github.com/StatPan/gira/issues/9", Roadmapable: false},
			},
		},
		capabilities: gira.ProjectCapabilityReport{
			Capabilities:   map[string]gira.ProjectCapabilityStatus{},
			BlockedActions: []gira.ProjectCapabilityBlock{},
		},
	}
	newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient {
		client.repo = repo
		return client
	}
	dashboardExportNow = func() time.Time {
		return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	}

	var firstOut, firstErr, secondOut, secondErr bytes.Buffer
	firstCode := Run([]string{"export", "dashboard", "--repo", "StatPan/gira", "--dry-run"}, &firstOut, &firstErr)
	secondCode := Run([]string{"export", "dashboard", "--repo", "StatPan/gira", "--dry-run"}, &secondOut, &secondErr)

	if firstCode != 0 || secondCode != 0 {
		t.Fatalf("exit codes = %d/%d, want 0/0; stderr: %s%s", firstCode, secondCode, firstErr.String(), secondErr.String())
	}
	if firstOut.String() != secondOut.String() {
		t.Fatalf("dry-run output changed between identical runs")
	}
	if !strings.Contains(firstOut.String(), "export dashboard plan:") {
		t.Fatalf("dry-run output missing plan header:\n%s", firstOut.String())
	}
}

func TestExportDashboardJSONOnlyStdout(t *testing.T) {
	restoreClient, restoreNow := newDashboardExportClient, dashboardExportNow
	t.Cleanup(func() {
		newDashboardExportClient = restoreClient
		dashboardExportNow = restoreNow
	})
	client := &cliFakeDashboardExportClient{
		repo: mustCLIRepo(t, "StatPan/gira"),
		capabilities: gira.ProjectCapabilityReport{
			Capabilities:   map[string]gira.ProjectCapabilityStatus{},
			BlockedActions: []gira.ProjectCapabilityBlock{},
		},
	}
	newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient {
		client.repo = repo
		return client
	}
	dashboardExportNow = func() time.Time {
		return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"export",
		"dashboard",
		"--repo",
		"StatPan/gira",
		"--dry-run",
		"--json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr not empty in json mode:\n%s", stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("JSON output parse failure: %v\n%s", err, stdout.String())
	}
	if payload["command"] != "export dashboard" {
		t.Fatalf("payload missing command field: %v", payload["command"])
	}
}

func TestExportDashboardApplyWritesArtifactsAndJsonOnlyStdout(t *testing.T) {
	restoreClient, restoreNow := newDashboardExportClient, dashboardExportNow
	t.Cleanup(func() {
		newDashboardExportClient = restoreClient
		dashboardExportNow = restoreNow
	})
	client := &cliFakeDashboardExportClient{
		repo: mustCLIRepo(t, "StatPan/gira"),
		milestones: []gira.DashboardRawMilestone{
			{MilestoneNumber: 1, Title: "MVP", State: "open"},
		},
		capabilities: gira.ProjectCapabilityReport{
			Capabilities: map[string]gira.ProjectCapabilityStatus{
				"issues:read": "allowed",
			},
		},
	}
	newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient {
		client.repo = repo
		return client
	}
	dashboardExportNow = func() time.Time {
		return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	}

	outputRoot := filepath.Join(t.TempDir(), "dashboard-output")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"export",
		"dashboard",
		"--repo",
		"StatPan/gira",
		"--output",
		outputRoot,
		"--json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr not empty in json mode:\n%s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"command\": \"export dashboard\"") {
		t.Fatalf("stdout missing plan JSON:\n%s", stdout.String())
	}

	expected := []string{
		"manifest.json",
		"raw/github.json",
		"raw/transitions.json",
		"raw/capabilities.json",
		"derived/execution_board.json",
		"derived/roadmap_timeline.json",
		"derived/warnings.json",
		"csv/execution_items.csv",
		"csv/roadmap_items.csv",
	}
	for _, relativePath := range expected {
		if _, err := os.Stat(filepath.Join(outputRoot, relativePath)); err != nil {
			t.Fatalf("expected exported file %q: %v", relativePath, err)
		}
	}
}

func TestExportDashboardApplyRejectsOutputFilePath(t *testing.T) {
	restoreClient := newDashboardExportClient
	t.Cleanup(func() { newDashboardExportClient = restoreClient })
	file, err := os.CreateTemp(t.TempDir(), "dashboard-output")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	newDashboardExportClient = func(repo gira.RepoRef) gira.DashboardExportClient {
		return &cliFakeDashboardExportClient{repo: repo}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"export",
		"dashboard",
		"--repo",
		"StatPan/gira",
		"--output",
		file.Name(),
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "output path exists but is not a directory") {
		t.Fatalf("stderr missing output path type error:\n%s", stderr.String())
	}
}

func TestStatusTextUsesInjectedClient(t *testing.T) {
	restoreClient, restoreNow := newStatusClient, statusNow
	t.Cleanup(func() {
		newStatusClient = restoreClient
		statusNow = restoreNow
	})
	newStatusClient = func(repo gira.RepoRef) gira.StatusClient {
		return cliFakeStatusClient{
			repo: repo,
			responses: map[string]string{
				"api repos/StatPan/gira/milestones --paginate --slurp -X GET -f state=all -f per_page=100": `[]`,
				"api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100":     `[[{"number":1,"title":"Issue 1","state":"open","labels":[{"name":"status:blocked"}],"milestone":null,"updated_at":"2026-04-01T12:00:00Z","html_url":"https://github.com/StatPan/gira/issues/1"}]]`,
			},
		}
	}
	statusNow = func() time.Time {
		return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"status", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"status: StatPan/gira", "milestone progress: none", "stale open issues: 1", "blocked issues: 1"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("status output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestStatusRequiresRepo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"status"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "--repo is required") {
		t.Fatalf("stderr missing repo requirement:\n%s", stderr.String())
	}
}

type cliFakeStatusClient struct {
	repo      gira.RepoRef
	responses map[string]string
}

type cliFakeSyncClient struct {
	repo       gira.RepoRef
	labels     []gira.ExistingLabel
	milestones []gira.ExistingMilestone
	issues     []gira.ExistingIssue
	calls      []string
}

type cliFakeDashboardExportClient struct {
	repo         gira.RepoRef
	issues       []gira.DashboardRawIssue
	pulls        []gira.DashboardRawPullRequest
	milestones   []gira.DashboardRawMilestone
	projectSync  gira.ProjectSyncSnapshot
	transitions  gira.ProjectTransitionSnapshot
	capabilities gira.ProjectCapabilityReport
}

func (c *cliFakeDashboardExportClient) Repo() gira.RepoRef { return c.repo }

func (c *cliFakeDashboardExportClient) FetchIssues() ([]gira.DashboardRawIssue, error) {
	return c.issues, nil
}

func (c *cliFakeDashboardExportClient) FetchPullRequests() ([]gira.DashboardRawPullRequest, error) {
	return c.pulls, nil
}

func (c *cliFakeDashboardExportClient) FetchMilestones() ([]gira.DashboardRawMilestone, error) {
	return c.milestones, nil
}

func (c *cliFakeDashboardExportClient) FetchProjectSnapshot() (gira.ProjectSyncSnapshot, error) {
	return c.projectSync, nil
}

func (c *cliFakeDashboardExportClient) FetchTransitionSnapshot() (gira.ProjectTransitionSnapshot, error) {
	return c.transitions, nil
}

func (c *cliFakeDashboardExportClient) FetchCapabilities() (gira.ProjectCapabilityReport, error) {
	return c.capabilities, nil
}

func strPtr(value string) *string {
	return &value
}

func (c *cliFakeSyncClient) Repo() gira.RepoRef { return c.repo }

func (c *cliFakeSyncClient) ListLabels() ([]gira.ExistingLabel, error) {
	return c.labels, nil
}

func (c *cliFakeSyncClient) CreateLabel(label gira.LabelDef) error {
	c.calls = append(c.calls, "create label "+label.Name)
	return nil
}

func (c *cliFakeSyncClient) UpdateLabel(label gira.LabelDef) error {
	c.calls = append(c.calls, "update label "+label.Name)
	return nil
}

func (c *cliFakeSyncClient) ListMilestones() ([]gira.ExistingMilestone, error) {
	return c.milestones, nil
}

func (c *cliFakeSyncClient) CreateMilestone(milestone gira.MilestoneDef) error {
	c.calls = append(c.calls, "create milestone "+milestone.Title)
	return nil
}

func (c *cliFakeSyncClient) UpdateMilestone(number int, milestone gira.MilestoneDef) error {
	c.calls = append(c.calls, "update milestone "+milestone.Title)
	return nil
}

func (c *cliFakeSyncClient) ListBootstrapIssues() ([]gira.ExistingIssue, error) {
	return c.issues, nil
}

func (c *cliFakeSyncClient) CreateIssue(issue gira.BootstrapIssueDef) error {
	c.calls = append(c.calls, "create issue "+issue.Title)
	return nil
}

func mustCLIRepo(t *testing.T, value string) gira.RepoRef {
	t.Helper()
	repo, err := gira.ParseRepoRef(value)
	if err != nil {
		t.Fatalf("ParseRepoRef returned error: %v", err)
	}
	return repo
}

func runCLIGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	if _, err := (gira.ExecCommandRunner{}).Run("git", append([]string{"-C", repo}, args...)...); err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
}

func (c cliFakeStatusClient) Repo() gira.RepoRef {
	return c.repo
}

func (c cliFakeStatusClient) JSON(args []string, target any) error {
	key := strings.Join(args, " ")
	return json.Unmarshal([]byte(c.responses[key]), target)
}

func TestProjectCapabilityCommandRequiresRepo(t *testing.T) {
	restore := newProjectCapabilityReport
	t.Cleanup(func() { newProjectCapabilityReport = restore })
	newProjectCapabilityReport = func(repo gira.RepoRef) (gira.ProjectCapabilityReport, error) {
		report := gira.ProjectCapabilityReport{
			Repo:    repo.FullName(),
			Command: "project capability",
			Mode:    "write",
			Token:   gira.ProjectCapabilityTokenSummary{Kind: "pat", Identity: "alice"},
			Capabilities: map[string]gira.ProjectCapabilityStatus{
				"issues:read": "allowed",
			},
			BlockedActions: []gira.ProjectCapabilityBlock{},
		}
		return report, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"project", "capability", "--repo", "StatPan/example", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"repo\": \"StatPan/example\"") {
		t.Fatalf("project capability JSON missing repo: %s", stdout.String())
	}
}

func TestProjectCapabilityCommandNeedsSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"project", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "project capability") {
		t.Fatalf("project help output unexpected: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "project sync") {
		t.Fatalf("project help output missing sync command: %s", stdout.String())
	}
}

func TestProjectSyncCommandRequiresDryRun(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"project", "sync", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "--dry-run is required") {
		t.Fatalf("stderr missing dry-run requirement:\n%s", stderr.String())
	}
}

func TestProjectSyncCommandJSONUsesInjectedBuilder(t *testing.T) {
	restore := newProjectSyncReport
	t.Cleanup(func() { newProjectSyncReport = restore })
	newProjectSyncReport = func(repo gira.RepoRef, dryRun bool) (gira.ProjectSyncReport, error) {
		report := gira.ProjectSyncReport{
			Repo:           repo.FullName(),
			Command:        "project sync",
			Project:        "Product OS",
			DryRun:         dryRun,
			MissingProject: true,
			Counts: gira.ProjectSyncCounts{
				FieldsMissing: 6,
			},
		}
		return report, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"project", "sync", "--repo", "StatPan/gira", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"command\": \"project sync\"") {
		t.Fatalf("project sync JSON missing command: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "\"dry_run\": true") {
		t.Fatalf("project sync JSON missing dry_run true: %s", stdout.String())
	}
}

func TestProjectTransitionsCommandRequiresDryRunOrApply(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"project", "transitions", "--repo", "StatPan/gira"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "exactly one of --dry-run or --apply is required") {
		t.Fatalf("stderr missing dry-run/apply requirement:\n%s", stderr.String())
	}
}

func TestProjectTransitionsCommandJSONUsesInjectedBuilder(t *testing.T) {
	restore := newProjectTransitionsReport
	t.Cleanup(func() { newProjectTransitionsReport = restore })
	newProjectTransitionsReport = func(repo gira.RepoRef, dryRun bool) (gira.ProjectTransitionsReport, error) {
		return gira.ProjectTransitionsReport{
			Repo:    repo.FullName(),
			Command: "project transitions",
			DryRun:  dryRun,
			Counts: gira.ProjectTransitionCounts{
				Applied: 1,
			},
			Transitions: []gira.ProjectTransitionPlanItem{
				{RuleID: "issue_open_default", TargetType: "issue", TargetID: "10", Decision: "apply", From: "null", To: "Backlog"},
			},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"project", "transitions", "--repo", "StatPan/gira", "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"command\": \"project transitions\"") {
		t.Fatalf("project transitions JSON missing command: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "\"dry_run\": true") {
		t.Fatalf("project transitions JSON missing dry_run true: %s", stdout.String())
	}
}

func TestProjectTransitionsApplyCommandJSONUsesInjectedBuilder(t *testing.T) {
	restore := newProjectTransitionsApplyReport
	t.Cleanup(func() { newProjectTransitionsApplyReport = restore })
	newProjectTransitionsApplyReport = func(repo gira.RepoRef) (gira.ProjectTransitionsApplyReport, error) {
		return gira.ProjectTransitionsApplyReport{
			Repo:    repo.FullName(),
			Command: "project transitions",
			DryRun:  false,
			Applied: []gira.ProjectTransitionApplyResultItem{{Issue: 10, RuleID: "issue_open_default", LabelApplied: "status:backlog"}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"project", "transitions", "--repo", "StatPan/gira", "--apply", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"dry_run\": false") {
		t.Fatalf("project transitions apply JSON missing dry_run false: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "\"label_applied\": \"status:backlog\"") {
		t.Fatalf("project transitions apply JSON missing label_applied: %s", stdout.String())
	}
}
