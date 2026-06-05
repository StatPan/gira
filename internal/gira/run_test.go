package gira

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildRunStartReportDryRunAutoID(t *testing.T) {
	stateRoot := t.TempDir()
	report, err := BuildRunStartReport(RunStartInput{
		Repo:      RepoRef{Owner: "StatPan", Name: "gira"},
		Ticket:    688,
		StateRoot: stateRoot,
		WorkDir:   "/tmp/work",
		DryRun:    true,
		Now:       time.Date(2026, 6, 5, 1, 2, 3, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildRunStartReport returned error: %v", err)
	}
	if !strings.HasPrefix(report.Manifest.RunID, "20260605-010203-statpan-gira-688-") {
		t.Fatalf("run id = %q", report.Manifest.RunID)
	}
	if report.Manifest.Status != "dry-run" {
		t.Fatalf("status = %q, want dry-run", report.Manifest.Status)
	}
	if report.Manifest.PublicSafe || !report.Manifest.PrivateStorage {
		t.Fatalf("public/private flags not conservative: %+v", report.Manifest)
	}
	if strings.Contains(strings.Join(report.Manifest.Command, " "), "dangerously-bypass") {
		t.Fatalf("default command should not bypass approvals or sandbox: %+v", report.Manifest.Command)
	}
	if _, err := os.Stat(filepath.Join(stateRoot, "runs")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not create run directory, stat err=%v", err)
	}
}

func TestBuildRunStartReportApplyWritesManifestAndPrompt(t *testing.T) {
	stateRoot := t.TempDir()
	report, err := BuildRunStartReport(RunStartInput{
		Repo:      RepoRef{Owner: "StatPan", Name: "gira"},
		Ticket:    688,
		RunID:     "manual-run",
		StateRoot: stateRoot,
		WorkDir:   "/tmp/work",
		Prompt:    []byte(`{"schema_version":"worker-handoff/v1"}` + "\n"),
		Apply:     true,
		Now:       time.Date(2026, 6, 5, 1, 2, 3, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildRunStartReport returned error: %v", err)
	}
	if report.Manifest.Status != "prepared" {
		t.Fatalf("status = %q, want prepared", report.Manifest.Status)
	}
	data, err := os.ReadFile(report.Manifest.ManifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest RunManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if manifest.RunID != "manual-run" || manifest.Repo != "StatPan/gira" || manifest.Ticket != 688 {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	prompt, err := os.ReadFile(report.Manifest.PromptPath)
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	if !strings.Contains(string(prompt), "worker-handoff/v1") {
		t.Fatalf("prompt missing handoff JSON: %s", string(prompt))
	}
}

func TestBuildRunStartReportRejectsDuplicateCustomRunID(t *testing.T) {
	stateRoot := t.TempDir()
	input := RunStartInput{
		Repo:      RepoRef{Owner: "StatPan", Name: "gira"},
		Ticket:    688,
		RunID:     "duplicate-run",
		StateRoot: stateRoot,
		WorkDir:   "/tmp/work",
		Apply:     true,
		Now:       time.Date(2026, 6, 5, 1, 2, 3, 0, time.UTC),
	}
	if _, err := BuildRunStartReport(input); err != nil {
		t.Fatalf("first BuildRunStartReport returned error: %v", err)
	}
	_, err := BuildRunStartReport(input)
	if err == nil {
		t.Fatal("second BuildRunStartReport returned nil error, want duplicate run id error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("duplicate error = %q", err.Error())
	}
}

func TestBuildRunStatusReportSelectsLatest(t *testing.T) {
	stateRoot := t.TempDir()
	oldRun, err := BuildRunStartReport(RunStartInput{
		Repo:      RepoRef{Owner: "StatPan", Name: "gira"},
		Ticket:    688,
		RunID:     "old-run",
		StateRoot: stateRoot,
		WorkDir:   "/tmp/work",
		Apply:     true,
		Now:       time.Date(2026, 6, 5, 1, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create old run: %v", err)
	}
	newRun, err := BuildRunStartReport(RunStartInput{
		Repo:      RepoRef{Owner: "StatPan", Name: "gira"},
		Ticket:    688,
		RunID:     "new-run",
		StateRoot: stateRoot,
		WorkDir:   "/tmp/work",
		Apply:     true,
		Now:       time.Date(2026, 6, 5, 2, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create new run: %v", err)
	}
	report, err := BuildRunStatusReport(RunSelectInput{
		Repo:      RepoRef{Owner: "StatPan", Name: "gira"},
		Ticket:    688,
		Latest:    true,
		StateRoot: stateRoot,
	})
	if err != nil {
		t.Fatalf("BuildRunStatusReport returned error: %v", err)
	}
	if report.Manifest == nil || report.Manifest.RunID != newRun.Manifest.RunID {
		t.Fatalf("latest manifest = %+v, want %s", report.Manifest, newRun.Manifest.RunID)
	}
	if len(report.Matches) != 2 || report.Matches[1].RunID != oldRun.Manifest.RunID {
		t.Fatalf("matches not sorted newest first: %+v", report.Matches)
	}
}

func TestBuildRunStatusReportSelectsLatestWithinSameSecond(t *testing.T) {
	stateRoot := t.TempDir()
	first, err := BuildRunStartReport(RunStartInput{
		Repo:      RepoRef{Owner: "StatPan", Name: "gira"},
		Ticket:    688,
		RunID:     "first-run",
		StateRoot: stateRoot,
		WorkDir:   "/tmp/work",
		Apply:     true,
		Now:       time.Date(2026, 6, 5, 1, 0, 0, 100, time.UTC),
	})
	if err != nil {
		t.Fatalf("create first run: %v", err)
	}
	second, err := BuildRunStartReport(RunStartInput{
		Repo:      RepoRef{Owner: "StatPan", Name: "gira"},
		Ticket:    688,
		RunID:     "second-run",
		StateRoot: stateRoot,
		WorkDir:   "/tmp/work",
		Apply:     true,
		Now:       time.Date(2026, 6, 5, 1, 0, 0, 200, time.UTC),
	})
	if err != nil {
		t.Fatalf("create second run: %v", err)
	}
	if first.Manifest.StartedAt == second.Manifest.StartedAt {
		t.Fatalf("started_at should preserve sub-second precision: first=%q second=%q", first.Manifest.StartedAt, second.Manifest.StartedAt)
	}
	report, err := BuildRunStatusReport(RunSelectInput{Ticket: 688, Latest: true, StateRoot: stateRoot})
	if err != nil {
		t.Fatalf("BuildRunStatusReport returned error: %v", err)
	}
	if report.Manifest == nil || report.Manifest.RunID != second.Manifest.RunID {
		t.Fatalf("latest manifest = %+v, want %s", report.Manifest, second.Manifest.RunID)
	}
}

func TestBuildRunStatusReportRefreshesCompletedRun(t *testing.T) {
	stateRoot := t.TempDir()
	start, err := BuildRunStartReport(RunStartInput{
		Repo:      RepoRef{Owner: "StatPan", Name: "gira"},
		Ticket:    688,
		RunID:     "completed-run",
		StateRoot: stateRoot,
		WorkDir:   "/tmp/work",
		Apply:     true,
		Exec:      true,
		Now:       time.Date(2026, 6, 5, 1, 2, 3, 0, time.UTC),
		PID:       999999,
	})
	if err != nil {
		t.Fatalf("BuildRunStartReport returned error: %v", err)
	}
	if err := os.WriteFile(start.Manifest.ResultPath, []byte("done\n"), 0o600); err != nil {
		t.Fatalf("write result: %v", err)
	}
	report, err := BuildRunStatusReport(RunSelectInput{RunID: start.Manifest.RunID, StateRoot: stateRoot})
	if err != nil {
		t.Fatalf("BuildRunStatusReport returned error: %v", err)
	}
	if report.Manifest == nil || report.Manifest.Status != "completed" || report.Manifest.FinishedAt == "" {
		t.Fatalf("manifest not refreshed to completed: %+v", report.Manifest)
	}
	data, err := os.ReadFile(start.Manifest.ManifestPath)
	if err != nil {
		t.Fatalf("read refreshed manifest: %v", err)
	}
	var manifest RunManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode refreshed manifest: %v", err)
	}
	if manifest.Status != "completed" {
		t.Fatalf("persisted status = %q, want completed", manifest.Status)
	}
}

func TestBuildRunStatusReportRefreshesExitedRun(t *testing.T) {
	stateRoot := t.TempDir()
	start, err := BuildRunStartReport(RunStartInput{
		Repo:      RepoRef{Owner: "StatPan", Name: "gira"},
		Ticket:    688,
		RunID:     "exited-run",
		StateRoot: stateRoot,
		WorkDir:   "/tmp/work",
		Apply:     true,
		Exec:      true,
		Now:       time.Date(2026, 6, 5, 1, 2, 3, 0, time.UTC),
		PID:       999999,
	})
	if err != nil {
		t.Fatalf("BuildRunStartReport returned error: %v", err)
	}
	report, err := BuildRunStatusReport(RunSelectInput{RunID: start.Manifest.RunID, StateRoot: stateRoot})
	if err != nil {
		t.Fatalf("BuildRunStatusReport returned error: %v", err)
	}
	if report.Manifest == nil || report.Manifest.Status != "exited" || report.Manifest.FinishedAt == "" {
		t.Fatalf("manifest not refreshed to exited: %+v", report.Manifest)
	}
}

func TestReadRunResult(t *testing.T) {
	stateRoot := t.TempDir()
	report, err := BuildRunStartReport(RunStartInput{
		Repo:      RepoRef{Owner: "StatPan", Name: "gira"},
		Ticket:    688,
		RunID:     "result-run",
		StateRoot: stateRoot,
		WorkDir:   "/tmp/work",
		Apply:     true,
		Now:       time.Date(2026, 6, 5, 1, 2, 3, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildRunStartReport returned error: %v", err)
	}
	if err := os.WriteFile(report.Manifest.ResultPath, []byte("done\n"), 0o600); err != nil {
		t.Fatalf("write result: %v", err)
	}
	result, err := ReadRunResult(report.Manifest)
	if err != nil {
		t.Fatalf("ReadRunResult returned error: %v", err)
	}
	if result != "done\n" {
		t.Fatalf("result = %q", result)
	}
}
