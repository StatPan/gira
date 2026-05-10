package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildWorkspaceRepoSyncReportDryRun(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "config.yaml"), "default_workspace: personal\n")
	writeTestFile(t, filepath.Join(root, "workspaces", "personal.yaml"), `workspace:
  name: personal
  owner: StatPan
  inbox_repo: StatPan/backlog
  repos:
    - StatPan/gira
`)
	runner := fakeOwnerRepoRunner{responses: map[string]string{
		"gh repo list StatPan --limit 100 --json nameWithOwner,isArchived --no-archived": `[
			{"nameWithOwner":"StatPan/backlog","isArchived":false},
			{"nameWithOwner":"StatPan/gira","isArchived":false},
			{"nameWithOwner":"StatPan/statpan-infra","isArchived":false}
		]`,
	}}

	report, err := BuildWorkspaceRepoSyncReport(WorkspaceRepoSyncInput{ConfigRoot: root, Owner: "StatPan", DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildWorkspaceRepoSyncReport returned error: %v", err)
	}
	if report.Status != "planned" || report.File.Action != "update" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if strings.Join(report.TargetRepos, ",") != "StatPan/gira,StatPan/statpan-infra" {
		t.Fatalf("target repos = %v", report.TargetRepos)
	}
	if strings.Join(report.AddedRepos, ",") != "StatPan/statpan-infra" {
		t.Fatalf("added repos = %v", report.AddedRepos)
	}
	if strings.Join(report.SkippedRepos, ",") != "StatPan/backlog" {
		t.Fatalf("skipped repos = %v", report.SkippedRepos)
	}
	content, err := os.ReadFile(filepath.Join(root, "workspaces", "personal.yaml"))
	if err != nil {
		t.Fatalf("read workspace: %v", err)
	}
	if strings.Contains(string(content), "statpan-infra") {
		t.Fatalf("dry-run mutated workspace:\n%s", string(content))
	}
}

func TestBuildWorkspaceRepoSyncReportApply(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "config.yaml"), "default_workspace: personal\n")
	writeTestFile(t, filepath.Join(root, "workspaces", "personal.yaml"), `workspace:
  name: personal
  owner: StatPan
  inbox_repo: StatPan/backlog
  repos:
    - StatPan/gira
`)
	runner := fakeOwnerRepoRunner{responses: map[string]string{
		"gh repo list StatPan --limit 100 --json nameWithOwner,isArchived --no-archived": `[
			{"nameWithOwner":"StatPan/backlog","isArchived":false},
			{"nameWithOwner":"StatPan/gira","isArchived":false},
			{"nameWithOwner":"StatPan/statpan-infra","isArchived":false}
		]`,
	}}

	report, err := BuildWorkspaceRepoSyncReport(WorkspaceRepoSyncInput{ConfigRoot: root, Owner: "StatPan", Apply: true}, runner)
	if err != nil {
		t.Fatalf("BuildWorkspaceRepoSyncReport apply returned error: %v", err)
	}
	if !report.Applied || report.Status != "applied" {
		t.Fatalf("unexpected apply report: %+v", report)
	}
	entry, err := LoadGlobalWorkspaceRegistryEntry(root, "personal")
	if err != nil {
		t.Fatalf("LoadGlobalWorkspaceRegistryEntry returned error: %v", err)
	}
	if strings.Join(entry.Workspace.Repos, ",") != "StatPan/gira,StatPan/statpan-infra" {
		t.Fatalf("workspace repos = %v", entry.Workspace.Repos)
	}
}

func TestBuildWorkspaceRepoSyncReportIncludeArchived(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "config.yaml"), "default_workspace: personal\n")
	writeTestFile(t, filepath.Join(root, "workspaces", "personal.yaml"), `workspace:
  name: personal
  owner: StatPan
  inbox_repo: StatPan/backlog
  repos:
    - StatPan/gira
`)
	runner := fakeOwnerRepoRunner{responses: map[string]string{
		"gh repo list StatPan --limit 100 --json nameWithOwner,isArchived --no-archived": `[{"nameWithOwner":"StatPan/gira","isArchived":false}]`,
		"gh repo list StatPan --limit 100 --json nameWithOwner,isArchived --archived":    `[{"nameWithOwner":"StatPan/old","isArchived":true}]`,
	}}
	report, err := BuildWorkspaceRepoSyncReport(WorkspaceRepoSyncInput{ConfigRoot: root, Owner: "StatPan", IncludeArchived: true, DryRun: true}, runner)
	if err != nil {
		t.Fatalf("BuildWorkspaceRepoSyncReport returned error: %v", err)
	}
	if strings.Join(report.TargetRepos, ",") != "StatPan/gira,StatPan/old" {
		t.Fatalf("target repos = %v", report.TargetRepos)
	}
}

type fakeOwnerRepoRunner struct {
	responses map[string]string
}

func (f fakeOwnerRepoRunner) Run(name string, args ...string) ([]byte, error) {
	key := name
	if len(args) > 0 {
		key += " " + strings.Join(args, " ")
	}
	if value, ok := f.responses[key]; ok {
		return []byte(value), nil
	}
	return nil, os.ErrNotExist
}
