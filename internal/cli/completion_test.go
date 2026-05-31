package cli

import (
	"bytes"
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
