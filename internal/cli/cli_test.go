package cli

import (
	"bytes"
	"strings"
	"testing"
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

func TestBootstrapRejectsNonDryRun(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"bootstrap", "--repo", "StatPan/example"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "supports --dry-run only") {
		t.Fatalf("stderr missing dry-run-only message:\n%s", stderr.String())
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
