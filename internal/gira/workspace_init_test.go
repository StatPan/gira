package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildWorkspaceInitReportDryRunPersonalRepoBound(t *testing.T) {
	report, err := BuildWorkspaceInitReport(WorkspaceInitInput{
		InboxRepo: "StatPan/gira",
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("BuildWorkspaceInitReport error: %v", err)
	}
	if report.Workspace.Owner != "StatPan" || report.Workspace.Name != "gira" || report.InboxRepo != "StatPan/gira" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if len(report.Repos) != 1 || report.Repos[0] != "StatPan/gira" {
		t.Fatalf("repos = %+v, want repo-bound default", report.Repos)
	}
	if !strings.Contains(report.Content, "workspace:") || !strings.Contains(report.Content, "inbox_repo: StatPan/gira") {
		t.Fatalf("content missing workspace config:\n%s", report.Content)
	}
}

func TestBuildWorkspaceInitReportApplyWritesConfig(t *testing.T) {
	dir := t.TempDir()
	report, err := BuildWorkspaceInitReport(WorkspaceInitInput{
		Name:      "personal",
		Owner:     "StatPan",
		InboxRepo: "StatPan/backlog",
		Repos:     []string{"StatPan/gira", "StatPan/docs"},
		Path:      dir,
		Apply:     true,
	})
	if err != nil {
		t.Fatalf("BuildWorkspaceInitReport error: %v", err)
	}
	if !report.Created || !report.Applied {
		t.Fatalf("expected created/applied report: %+v", report)
	}
	resolved, err := ResolveWorkspaceConfig(filepath.Join(dir, ".gira", "config.yaml"))
	if err != nil {
		t.Fatalf("ResolveWorkspaceConfig error: %v", err)
	}
	if resolved.InboxRepo.FullName() != "StatPan/backlog" || len(resolved.Repos) != 2 {
		t.Fatalf("resolved = %+v", resolved)
	}
	if _, err := os.Stat(report.ConfigPath); err != nil {
		t.Fatalf("config path not written: %v", err)
	}
}

func TestBuildWorkspaceInitReportValidatesRepoAllowlist(t *testing.T) {
	_, err := BuildWorkspaceInitReport(WorkspaceInitInput{InboxRepo: "StatPan/gira", Repos: []string{"StatPan/gira", "StatPan/gira"}, DryRun: true})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate repo error, got %v", err)
	}
}
