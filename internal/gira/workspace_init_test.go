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
	if report.Project.Owner != "StatPan" || report.Project.Title != "gira" || report.Project.Number != 0 {
		t.Fatalf("project = %+v, want workspace owner/name defaults", report.Project)
	}
	for _, want := range []string{"workspace:", "name: \"gira\"", "inbox_repo: StatPan/gira", "project:", "owner: StatPan", "title: \"gira\""} {
		if !strings.Contains(report.Content, want) {
			t.Fatalf("content missing %q:\n%s", want, report.Content)
		}
	}
	if strings.Contains(report.Content, "number:") {
		t.Fatalf("content should omit project number by default:\n%s", report.Content)
	}
}

func TestBuildWorkspaceInitReportProjectOverrides(t *testing.T) {
	report, err := BuildWorkspaceInitReport(WorkspaceInitInput{
		Name:          "personal",
		Owner:         "StatPan",
		InboxRepo:     "StatPan/backlog",
		ProjectOwner:  "GiraOrg",
		ProjectTitle:  "Roadmap",
		ProjectNumber: 12,
		DryRun:        true,
	})
	if err != nil {
		t.Fatalf("BuildWorkspaceInitReport error: %v", err)
	}
	if report.Project.Owner != "GiraOrg" || report.Project.Title != "Roadmap" || report.Project.Number != 12 {
		t.Fatalf("project = %+v, want overrides", report.Project)
	}
	for _, want := range []string{"owner: GiraOrg", "title: \"Roadmap\"", "number: 12"} {
		if !strings.Contains(report.Content, want) {
			t.Fatalf("content missing %q:\n%s", want, report.Content)
		}
	}
}

func TestBuildWorkspaceInitReportQuotesFreeFormYAMLScalars(t *testing.T) {
	dir := t.TempDir()
	report, err := BuildWorkspaceInitReport(WorkspaceInitInput{
		Name:         `Personal: Q2 "Focus"`,
		Owner:        "StatPan",
		InboxRepo:    "StatPan/backlog",
		ProjectTitle: `Roadmap: Q2 "Focus"`,
		Path:         dir,
		Apply:        true,
	})
	if err != nil {
		t.Fatalf("BuildWorkspaceInitReport error: %v", err)
	}
	resolved, err := ResolveWorkspaceConfig(report.ConfigPath)
	if err != nil {
		t.Fatalf("ResolveWorkspaceConfig error: %v", err)
	}
	if resolved.Name != `Personal: Q2 "Focus"` || resolved.Project.Title != `Roadmap: Q2 "Focus"` {
		t.Fatalf("resolved = %+v, want quoted free-form scalar values", resolved)
	}
}

func TestBuildWorkspaceInitReportRejectsNegativeProjectNumber(t *testing.T) {
	_, err := BuildWorkspaceInitReport(WorkspaceInitInput{
		InboxRepo:     "StatPan/gira",
		ProjectNumber: -1,
		DryRun:        true,
	})
	if err == nil || !strings.Contains(err.Error(), "--project-number must be >= 0") {
		t.Fatalf("expected project number error, got %v", err)
	}
}

func TestFormatWorkspaceInitReportIncludesProject(t *testing.T) {
	report, err := BuildWorkspaceInitReport(WorkspaceInitInput{
		InboxRepo:     "StatPan/gira",
		ProjectNumber: 9,
		DryRun:        true,
	})
	if err != nil {
		t.Fatalf("BuildWorkspaceInitReport error: %v", err)
	}
	output := FormatWorkspaceInitReport(report)
	if !strings.Contains(output, "project: StatPan/gira #9") {
		t.Fatalf("formatted output missing project:\n%s", output)
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
	if resolved.Project.Owner != "StatPan" || resolved.Project.Title != "personal" {
		t.Fatalf("resolved project = %+v, want workspace project defaults", resolved.Project)
	}
	if report.Project != resolved.Project {
		t.Fatalf("report project = %+v, resolved project = %+v", report.Project, resolved.Project)
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
