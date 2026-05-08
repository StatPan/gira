package gira

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildCachePruneReportSelectsOnlyOlderStableVersionDirs(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, "v1.0.0"))
	mkdirAll(t, filepath.Join(root, "v1.2.0"))
	mkdirAll(t, filepath.Join(root, "v1.3.0"))
	mkdirAll(t, filepath.Join(root, "dev"))
	mkdirAll(t, filepath.Join(root, "v1.1.0-beta.1"))

	report, err := BuildCachePruneReport(CachePruneOptions{
		Root:          root,
		ActiveVersion: "v1.2.0",
		DryRun:        true,
	})
	if err != nil {
		t.Fatalf("BuildCachePruneReport error = %v", err)
	}
	if report.Counts.Planned != 1 {
		t.Fatalf("planned = %d, want 1; actions=%#v", report.Counts.Planned, report.Actions)
	}
	assertAction(t, report, "v1.0.0", "prune", "planned")
	assertAction(t, report, "v1.2.0", "skip", "skipped")
	assertAction(t, report, "v1.3.0", "skip", "skipped")
	assertAction(t, report, "dev", "skip", "skipped")
	assertAction(t, report, "v1.1.0-beta.1", "skip", "skipped")
}

func TestBuildCachePruneReportDryRunDoesNotDelete(t *testing.T) {
	root := t.TempDir()
	stale := filepath.Join(root, "v1.0.0")
	mkdirAll(t, stale)

	report, err := BuildCachePruneReport(CachePruneOptions{
		Root:          root,
		ActiveVersion: "v1.1.0",
		DryRun:        true,
	})
	if err != nil {
		t.Fatalf("BuildCachePruneReport error = %v", err)
	}
	if report.Counts.Planned != 1 {
		t.Fatalf("planned = %d, want 1", report.Counts.Planned)
	}
	if _, err := os.Stat(stale); err != nil {
		t.Fatalf("dry-run removed stale dir: %v", err)
	}
}

func TestBuildCachePruneReportApplyDeletesOnlyPlannedStaleDirs(t *testing.T) {
	root := t.TempDir()
	stale := filepath.Join(root, "v1.0.0")
	active := filepath.Join(root, "v1.1.0")
	newer := filepath.Join(root, "v1.2.0")
	mkdirAll(t, stale)
	mkdirAll(t, active)
	mkdirAll(t, newer)

	report, err := BuildCachePruneReport(CachePruneOptions{
		Root:          root,
		ActiveVersion: "v1.1.0",
		Apply:         true,
	})
	if err != nil {
		t.Fatalf("BuildCachePruneReport error = %v", err)
	}
	if report.Counts.Applied != 1 {
		t.Fatalf("applied = %d, want 1; actions=%#v", report.Counts.Applied, report.Actions)
	}
	assertMissing(t, stale)
	assertExists(t, active)
	assertExists(t, newer)
}

func TestBuildCachePruneReportSkipsMalformedFilesAndSymlinks(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, "v1.0.0"))
	if err := os.WriteFile(filepath.Join(root, "v0.9.0"), []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.Symlink(filepath.Join(root, "v1.0.0"), filepath.Join(root, "v0.8.0")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	mkdirAll(t, filepath.Join(root, "latest"))

	report, err := BuildCachePruneReport(CachePruneOptions{
		Root:          root,
		ActiveVersion: "v1.1.0",
		Apply:         true,
	})
	if err != nil {
		t.Fatalf("BuildCachePruneReport error = %v", err)
	}
	assertAction(t, report, "v1.0.0", "prune", "applied")
	assertAction(t, report, "v0.9.0", "skip", "skipped")
	assertAction(t, report, "v0.8.0", "skip", "skipped")
	assertAction(t, report, "latest", "skip", "skipped")
	assertExists(t, filepath.Join(root, "v0.9.0"))
	assertExists(t, filepath.Join(root, "v0.8.0"))
	assertExists(t, filepath.Join(root, "latest"))
}

func TestDefaultCachePruneRootUsesEnv(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GIRA_PYPI_CACHE_DIR", root)

	got, err := DefaultCachePruneRoot()
	if err != nil {
		t.Fatalf("DefaultCachePruneRoot error = %v", err)
	}
	if got != root {
		t.Fatalf("root = %q, want %q", got, root)
	}
}

func TestDefaultCachePruneRootMatchesWrapperHomeCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GIRA_PYPI_CACHE_DIR", "")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(t.TempDir(), "xdg-cache"))

	got, err := DefaultCachePruneRoot()
	if err != nil {
		t.Fatalf("DefaultCachePruneRoot error = %v", err)
	}
	want := filepath.Join(home, ".cache", "gira-cli")
	if got != want {
		t.Fatalf("root = %q, want wrapper cache root %q", got, want)
	}
}

func TestBuildCachePruneReportUsesCustomRoot(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, "v1.0.0"))

	report, err := BuildCachePruneReport(CachePruneOptions{
		Root:          root,
		ActiveVersion: "v1.1.0",
		DryRun:        true,
	})
	if err != nil {
		t.Fatalf("BuildCachePruneReport error = %v", err)
	}
	if report.Root != root {
		t.Fatalf("report root = %q, want %q", report.Root, root)
	}
	if report.Counts.Planned != 1 {
		t.Fatalf("planned = %d, want 1", report.Counts.Planned)
	}
}

func TestBuildCachePruneReportProtectsDirectoryContainingExecutable(t *testing.T) {
	root := t.TempDir()
	versionDir := filepath.Join(root, "v1.0.0")
	bin := filepath.Join(versionDir, "linux_amd64", "gira")
	mkdirAll(t, filepath.Dir(bin))
	if err := os.WriteFile(bin, []byte("#!/usr/bin/env sh\n"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}

	report, err := BuildCachePruneReport(CachePruneOptions{
		Root:           root,
		ActiveVersion:  "v1.2.0",
		ExecutablePath: bin,
		Apply:          true,
	})
	if err != nil {
		t.Fatalf("BuildCachePruneReport error = %v", err)
	}
	assertAction(t, report, "v1.0.0", "skip", "skipped")
	assertExists(t, versionDir)
}

func TestBuildCachePruneReportNonComparableActiveDoesNotApply(t *testing.T) {
	root := t.TempDir()
	stale := filepath.Join(root, "v1.0.0")
	mkdirAll(t, stale)

	report, err := BuildCachePruneReport(CachePruneOptions{
		Root:          root,
		ActiveVersion: "dev",
		Apply:         true,
	})
	if err != nil {
		t.Fatalf("BuildCachePruneReport error = %v", err)
	}
	if report.ActiveComparable {
		t.Fatal("ActiveComparable = true, want false")
	}
	assertAction(t, report, "v1.0.0", "skip", "skipped")
	assertExists(t, stale)
}

func TestBuildCachePruneReportRequiresMode(t *testing.T) {
	root := t.TempDir()
	if _, err := BuildCachePruneReport(CachePruneOptions{Root: root}); err == nil {
		t.Fatal("BuildCachePruneReport without mode error = nil, want error")
	}
	if _, err := BuildCachePruneReport(CachePruneOptions{Root: root, DryRun: true, Apply: true}); err == nil {
		t.Fatal("BuildCachePruneReport with both modes error = nil, want error")
	}
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func assertAction(t *testing.T, report CachePruneReport, name string, action string, status string) {
	t.Helper()
	for _, candidate := range report.Actions {
		if candidate.Name == name {
			if candidate.Action != action || candidate.Status != status {
				t.Fatalf("%s action/status = %s/%s, want %s/%s; action=%#v", name, candidate.Action, candidate.Status, action, status, candidate)
			}
			return
		}
	}
	t.Fatalf("missing action for %s; actions=%#v", name, report.Actions)
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("%s should exist: %v", path, err)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("%s should be missing, stat err=%v", path, err)
	}
}
