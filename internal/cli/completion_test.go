package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletionBashIncludesCommonCommandsAndFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"completion", "bash"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{
		"complete -F _gira_completion gira",
		"gira \"${args[@]}\"",
		"completion candidates",
		"goal",
		"report",
		"dossier",
		"ticket",
		"start",
		"finish",
		"--repo",
		"--json",
		"--dry-run",
		"--apply",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("bash completion missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestCompletionZshIncludesCommonCommandsAndFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"completion", "zsh"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{
		"#compdef gira",
		"gira ${args[@]}",
		"completion candidates",
		"'goal:Goal-mode planning and reports'",
		"'report:Build a goal report'",
		"'ticket:Ticket lifecycle commands'",
		"'start:Start ticket work'",
		"'finish:Finish ticket work'",
		"'--repo[Target GitHub repo]'",
		"'--json[Emit JSON output]'",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("zsh completion missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestCompletionFishIncludesCommonCommandsAndFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"completion", "fish"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{
		"complete -c gira -f",
		"function __gira_candidates",
		"gira completion candidates $kind",
		"complete -c gira -n '__fish_use_subcommand' -a 'goal'",
		"complete -c gira -n '__fish_seen_subcommand_from goal' -a 'report'",
		"complete -c gira -n '__fish_seen_subcommand_from ticket' -a 'start'",
		"complete -c gira -n '__fish_seen_subcommand_from ticket' -a 'finish'",
		"complete -c gira -l repo",
		"complete -c gira -l json",
		"complete -c gira -l dry-run",
		"complete -c gira -l apply",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("fish completion missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestCompletionCandidatesFromRegistryAndStatusCache(t *testing.T) {
	configHome := t.TempDir()
	cacheHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	writeCompletionTestFile(t, filepath.Join(configHome, "gira", "repos", "StatPan", "gira.yaml"), "repo: StatPan/gira\naliases:\n  - gira\n")
	writeCompletionTestFile(t, filepath.Join(cacheHome, "gira", "workspace-status", "StatPan", "gira.json"), `{
  "repo": "StatPan/gira",
  "issues": {
    "open": [
      {"number": 644, "title": "Dynamic completion", "state": "open", "labels": ["status:ready", "area:cli"], "milestone": "M1"}
    ]
  },
  "milestones": [
    {"number": 1, "title": "M1", "state": "open"},
    {"number": 2, "title": "Later", "state": "closed"}
  ]
}
`)

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{name: "repo", args: []string{"completion", "candidates", "repo", "--prefix", "gi"}, want: []string{"gira"}},
		{name: "ticket", args: []string{"completion", "candidates", "ticket", "--repo", "StatPan/gira"}, want: []string{"644"}},
		{name: "label", args: []string{"completion", "candidates", "label", "--repo", "StatPan/gira", "--prefix", "area"}, want: []string{"area:cli"}},
		{name: "milestone", args: []string{"completion", "candidates", "milestone", "--repo", "StatPan/gira", "--prefix", "M"}, want: []string{"M1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tt.args, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
			}
			for _, want := range tt.want {
				if !strings.Contains(stdout.String(), want+"\n") {
					t.Fatalf("candidates missing %q:\nstdout=%s\nstderr=%s", want, stdout.String(), stderr.String())
				}
			}
		})
	}
}

func writeCompletionTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestCompletionRejectsUnsupportedShell(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"completion", "powershell"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), `unsupported completion shell "powershell"`) {
		t.Fatalf("stderr missing unsupported shell guidance:\n%s", stderr.String())
	}
}

func TestCompletionHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"completion", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"gira completion bash", "gira completion zsh", "gira completion fish"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("completion help missing %q:\n%s", want, stdout.String())
		}
	}
}
