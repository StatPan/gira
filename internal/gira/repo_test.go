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

func TestResolveRepoContextOverrideWins(t *testing.T) {
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

func TestResolveRepoContextFromConfig(t *testing.T) {
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
		errs: map[string]error{"git remote get-url origin": fmt.Errorf("config should win before origin")},
	})
	if err != nil {
		t.Fatalf("ResolveRepoContext returned error: %v", err)
	}
	if repo.FullName() != "StatPan/configured" {
		t.Fatalf("repo = %s, want StatPan/configured", repo.FullName())
	}
}

func TestResolveRepoContextOverrideWinsOverConfig(t *testing.T) {
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

func TestResolveRepoContextMissingOriginReturnsRemediation(t *testing.T) {
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
