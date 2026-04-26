package gira

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallTemplatesCreatesSkipsConflictsAndOverwrites(t *testing.T) {
	repo := newGitRepo(t)
	rendered := []RenderedTemplate{
		{Path: "AGENTS.md", Content: "agents\n"},
		{Path: "docs/plans/README.md", Content: "plans\n"},
	}

	first, err := InstallTemplates(repo, rendered, false, DefaultBranch)
	if err != nil {
		t.Fatalf("InstallTemplates returned error: %v", err)
	}
	if len(first.Created) != 2 || first.Branch != DefaultBranch {
		t.Fatalf("first result = %+v, want 2 created on default branch", first)
	}
	if current := gitOutput(t, repo, "rev-parse", "--abbrev-ref", "HEAD"); current != DefaultBranch {
		t.Fatalf("current branch = %q, want %q", current, DefaultBranch)
	}

	second, err := InstallTemplates(repo, rendered, false, DefaultBranch)
	if err != nil {
		t.Fatalf("second InstallTemplates returned error: %v", err)
	}
	if len(second.Skipped) != 2 || len(second.Created) != 0 {
		t.Fatalf("second result = %+v, want 2 skipped and 0 created", second)
	}

	if err := os.WriteFile(filepath.Join(repo, "AGENTS.md"), []byte("custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	conflict, err := InstallTemplates(repo, rendered, false, DefaultBranch)
	if err != nil {
		t.Fatalf("conflict InstallTemplates returned error: %v", err)
	}
	if len(conflict.Conflicts) != 1 || conflict.Conflicts[0] != "AGENTS.md" {
		t.Fatalf("conflict result = %+v, want AGENTS.md conflict", conflict)
	}
	if got := readFile(t, filepath.Join(repo, "AGENTS.md")); got != "custom\n" {
		t.Fatalf("AGENTS.md = %q, want custom contents preserved", got)
	}

	overwrite, err := InstallTemplates(repo, rendered, true, DefaultBranch)
	if err != nil {
		t.Fatalf("overwrite InstallTemplates returned error: %v", err)
	}
	if len(overwrite.Overwritten) != 1 || overwrite.Overwritten[0] != "AGENTS.md" {
		t.Fatalf("overwrite result = %+v, want AGENTS.md overwritten", overwrite)
	}
	if got := readFile(t, filepath.Join(repo, "AGENTS.md")); got != "agents\n" {
		t.Fatalf("AGENTS.md = %q, want rendered contents", got)
	}
}

func TestInstallTemplatesNoBranchKeepsCurrentBranch(t *testing.T) {
	repo := newGitRepo(t)
	result, err := InstallTemplates(repo, []RenderedTemplate{{Path: "AGENTS.md", Content: "agents\n"}}, false, "")
	if err != nil {
		t.Fatalf("InstallTemplates returned error: %v", err)
	}
	if result.Branch != "" {
		t.Fatalf("result branch = %q, want empty", result.Branch)
	}
	if current := gitOutput(t, repo, "rev-parse", "--abbrev-ref", "HEAD"); current != "main" {
		t.Fatalf("current branch = %q, want main", current)
	}
}

func TestInstallTemplatesRejectsUnsafeTemplatePaths(t *testing.T) {
	repo := newGitRepo(t)
	for _, path := range []string{"../AGENTS.md", "/tmp/AGENTS.md", `docs\..\AGENTS.md`} {
		_, err := InstallTemplates(repo, []RenderedTemplate{{Path: path, Content: "x\n"}}, false, "")
		if err == nil || !strings.Contains(err.Error(), "unsafe template path") {
			t.Fatalf("path %q error = %v, want unsafe template path", path, err)
		}
	}
}

func TestInstallTemplatesRequiresExistingGitRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := InstallTemplates(dir, nil, false, "")
	if err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("error = %v, want not a git repository", err)
	}

	missing := filepath.Join(t.TempDir(), "missing")
	_, err = InstallTemplates(missing, nil, false, "")
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("error = %v, want not a directory", err)
	}
}

func TestFormatInstallSummaryMatchesPythonShape(t *testing.T) {
	got := FormatInstallSummary(InstallResult{
		Created:     []string{"AGENTS.md"},
		Skipped:     []string{"tasks/backlog.md"},
		Overwritten: []string{".github/PULL_REQUEST_TEMPLATE.md"},
		Conflicts:   []string{"docs/plans/README.md"},
		Branch:      DefaultBranch,
	})
	want := "branch: chore/gira-bootstrap\ncreated:     1\nskipped:     1\noverwritten: 1\nconflicts:   1\n  conflict: docs/plans/README.md\n"
	if got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
}

func newGitRepo(t *testing.T) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "target")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "init", "-q", "-b", "main")
	gitRun(t, repo, "config", "user.email", "test@example.com")
	gitRun(t, repo, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", ".")
	gitRun(t, repo, "commit", "-q", "-m", "seed")
	return repo
}

func gitOutput(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
	return strings.TrimSpace(string(output))
}

func gitRun(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
