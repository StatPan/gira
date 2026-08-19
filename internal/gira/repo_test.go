package gira

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGitHubRemoteRepo(t *testing.T) {
	cases := []struct {
		name   string
		remote string
		want   string
	}{
		{name: "https", remote: "https://github.com/StatPan/gira.git", want: "StatPan/gira"},
		{name: "ssh scp", remote: "git@github.com:StatPan/gira.git", want: "StatPan/gira"},
		{name: "ssh url", remote: "ssh://git@github.com/StatPan/gira.git", want: "StatPan/gira"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, err := ParseGitHubRemoteRepo(tc.remote)
			if err != nil {
				t.Fatalf("ParseGitHubRemoteRepo returned error: %v", err)
			}
			if repo.FullName() != tc.want {
				t.Fatalf("repo = %s, want %s", repo.FullName(), tc.want)
			}
		})
	}
}

func TestParseRepoRefAcceptsValidGitHubIdentifiers(t *testing.T) {
	for _, raw := range []string{
		"StatPan/gira",
		"owner-123/repo_name.v2",
		"github/.github",
		" OWNER/repo ",
	} {
		repo, err := ParseRepoRef(raw)
		if err != nil {
			t.Fatalf("ParseRepoRef(%q) returned error: %v", raw, err)
		}
		if repo.Owner == "" || repo.Name == "" || strings.ContainsAny(repo.Owner+repo.Name, `/\\`) {
			t.Fatalf("ParseRepoRef(%q) returned unsafe repo: %+v", raw, repo)
		}
	}
}

func TestParseRepoRefRejectsHostileOrInvalidIdentifiers(t *testing.T) {
	for _, raw := range []string{
		"../gira",
		"StatPan/../../gira",
		"StatPan\\gira",
		"StatPan/gira\\escape",
		"StatPan/gira\n",
		"StatPan/gira\x00",
		"Stat!Pan/gira",
		"StatPan/repo?",
		"StatPan/.",
		"StatPan/..",
	} {
		if _, err := ParseRepoRef(raw); err == nil {
			t.Fatalf("ParseRepoRef(%q) returned nil error", raw)
		}
	}
}

func TestResolveRepoContextOverrideWins(t *testing.T) {
	isolateDefaultGlobalConfig(t)
	repo, err := ResolveRepoContext("StatPan/override", repoContextTestRunner{
		errs: map[string]error{"git remote get-url origin": fmt.Errorf("should not be called")},
	})
	if err != nil {
		t.Fatalf("ResolveRepoContext returned error: %v", err)
	}
	if repo.FullName() != "StatPan/override" {
		t.Fatalf("repo = %s, want StatPan/override", repo.FullName())
	}
}

func TestResolveRepoContextFromHTTPSOrigin(t *testing.T) {
	isolateDefaultGlobalConfig(t)
	repo, err := ResolveRepoContext("", repoContextTestRunner{
		outputs: map[string][]byte{"git remote get-url origin": []byte("https://github.com/StatPan/gira.git\n")},
	})
	if err != nil {
		t.Fatalf("ResolveRepoContext returned error: %v", err)
	}
	if repo.FullName() != "StatPan/gira" {
		t.Fatalf("repo = %s, want StatPan/gira", repo.FullName())
	}
}

func TestResolveRepoContextOriginWinsOverRepoLocalConfig(t *testing.T) {
	isolateDefaultGlobalConfig(t)
	dir := t.TempDir()
	giraDir := filepath.Join(dir, ".gira")
	if err := os.MkdirAll(giraDir, 0o755); err != nil {
		t.Fatalf("mkdir .gira: %v", err)
	}
	if err := os.WriteFile(filepath.Join(giraDir, "config.yaml"), []byte("repo: StatPan/configured\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	repo, err := ResolveRepoContext("", repoContextTestRunner{
		outputs: map[string][]byte{"git remote get-url origin": []byte("https://github.com/StatPan/origin.git\n")},
	})
	if err != nil {
		t.Fatalf("ResolveRepoContext returned error: %v", err)
	}
	if repo.FullName() != "StatPan/origin" {
		t.Fatalf("repo = %s, want StatPan/origin", repo.FullName())
	}
}

func TestResolveRepoContextFromConfigFallback(t *testing.T) {
	isolateDefaultGlobalConfig(t)
	dir := t.TempDir()
	giraDir := filepath.Join(dir, ".gira")
	if err := os.MkdirAll(giraDir, 0o755); err != nil {
		t.Fatalf("mkdir .gira: %v", err)
	}
	if err := os.WriteFile(filepath.Join(giraDir, "config.yaml"), []byte("repo: StatPan/configured\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	repo, err := ResolveRepoContext("", repoContextTestRunner{
		errs: map[string]error{"git remote get-url origin": fmt.Errorf("exit status 2")},
	})
	if err != nil {
		t.Fatalf("ResolveRepoContext returned error: %v", err)
	}
	if repo.FullName() != "StatPan/configured" {
		t.Fatalf("repo = %s, want StatPan/configured", repo.FullName())
	}
}

func TestResolveRepoContextOverrideWinsOverConfig(t *testing.T) {
	isolateDefaultGlobalConfig(t)
	dir := t.TempDir()
	giraDir := filepath.Join(dir, ".gira")
	if err := os.MkdirAll(giraDir, 0o755); err != nil {
		t.Fatalf("mkdir .gira: %v", err)
	}
	if err := os.WriteFile(filepath.Join(giraDir, "config.yaml"), []byte("repo: StatPan/configured\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	repo, err := ResolveRepoContext("StatPan/override", repoContextTestRunner{
		errs: map[string]error{"git remote get-url origin": fmt.Errorf("should not be called")},
	})
	if err != nil {
		t.Fatalf("ResolveRepoContext returned error: %v", err)
	}
	if repo.FullName() != "StatPan/override" {
		t.Fatalf("repo = %s, want StatPan/override", repo.FullName())
	}
}

func TestResolveRepoContextLoadsGlobalRegistryFromOrigin(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "repos", "StatPan", "gira.yaml"), "repo: StatPan/gira\npath: /workspace/gira\naliases:\n  - gira\n")

	ctx, err := ResolveRepoContextDetails(RepoContextOptions{
		ConfigRoot: root,
		Runner: repoContextTestRunner{
			outputs: map[string][]byte{"git remote get-url origin": []byte("https://github.com/StatPan/gira.git\n")},
		},
	})
	if err != nil {
		t.Fatalf("ResolveRepoContextDetails returned error: %v", err)
	}
	if ctx.Repo.FullName() != "StatPan/gira" || ctx.Source != "git_origin" || ctx.GlobalRepo == nil || ctx.GlobalRepo.Path != "/workspace/gira" {
		t.Fatalf("unexpected repo context: %+v", ctx)
	}
}

func TestResolveRepoContextFromGlobalAlias(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "repos", "StatPan", "gira.yaml"), "repo: StatPan/gira\naliases:\n  - gira\n")

	ctx, err := ResolveRepoContextDetails(RepoContextOptions{
		RepoValue:  "gira",
		ConfigRoot: root,
		Runner:     repoContextTestRunner{errs: map[string]error{"git remote get-url origin": fmt.Errorf("alias should not call origin")}},
	})
	if err != nil {
		t.Fatalf("ResolveRepoContextDetails returned error: %v", err)
	}
	if ctx.Repo.FullName() != "StatPan/gira" || ctx.Source != "global_alias" {
		t.Fatalf("unexpected repo context: %+v", ctx)
	}
}

func TestResolveRepoContextFromGlobalPath(t *testing.T) {
	root := t.TempDir()
	checkout := filepath.Join(t.TempDir(), "gira")
	subdir := filepath.Join(checkout, "docs")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir checkout: %v", err)
	}
	writeTestFile(t, filepath.Join(root, "repos", "StatPan", "gira.yaml"), fmt.Sprintf("repo: StatPan/gira\npath: %s\n", filepath.ToSlash(checkout)))

	ctx, err := ResolveRepoContextDetails(RepoContextOptions{
		ConfigRoot: root,
		WorkDir:    subdir,
		Runner:     repoContextTestRunner{errs: map[string]error{"git -C " + subdir + " remote get-url origin": fmt.Errorf("exit status 2")}},
	})
	if err != nil {
		t.Fatalf("ResolveRepoContextDetails returned error: %v", err)
	}
	if ctx.Repo.FullName() != "StatPan/gira" || ctx.Source != "global_path" {
		t.Fatalf("unexpected repo context: %+v", ctx)
	}
}

func TestResolveRepoContextMissingOriginReturnsRemediation(t *testing.T) {
	isolateDefaultGlobalConfig(t)
	_, err := ResolveRepoContext("", repoContextTestRunner{
		errs: map[string]error{"git remote get-url origin": fmt.Errorf("exit status 2")},
	})
	if err == nil {
		t.Fatal("ResolveRepoContext returned nil error, want remediation")
	}
	if !strings.Contains(err.Error(), "pass --repo OWNER/REPO") {
		t.Fatalf("error missing remediation: %v", err)
	}
}

func TestResolveRepoContextAmbiguousOriginReturnsRemediation(t *testing.T) {
	isolateDefaultGlobalConfig(t)
	_, err := ResolveRepoContext("", repoContextTestRunner{
		outputs: map[string][]byte{"git remote get-url origin": []byte("https://example.com/StatPan/gira.git\n")},
	})
	if err == nil {
		t.Fatal("ResolveRepoContext returned nil error, want remediation")
	}
	if !strings.Contains(err.Error(), "origin remote is not a GitHub OWNER/REPO URL") {
		t.Fatalf("error missing ambiguous origin guidance: %v", err)
	}
}

type repoContextTestRunner struct {
	outputs map[string][]byte
	errs    map[string]error
}

func isolateDefaultGlobalConfig(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

func (r repoContextTestRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	if err, ok := r.errs[key]; ok {
		return nil, err
	}
	if out, ok := r.outputs[key]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected call: %s", key)
}
