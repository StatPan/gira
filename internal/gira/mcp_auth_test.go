package gira

import (
	"errors"
	"strings"
	"testing"
)

type mcpAuthRunner struct {
	name string
	args []string
	err  error
}

func (r *mcpAuthRunner) Run(name string, args ...string) ([]byte, error) {
	r.name = name
	r.args = append([]string(nil), args...)
	if r.err != nil {
		return nil, r.err
	}
	return []byte("ok"), nil
}

func TestResolveMCPAuthConfigPrecedence(t *testing.T) {
	auth := ResolveMCPAuthConfig([]string{
		"GH_TOKEN=gh-token",
		"GITHUB_TOKEN=github-token",
		"GIRA_MCP_GITHUB_TOKEN=gira-token",
		"GITHUB_HOST=github.example.com",
	})
	if auth.Mode != MCPAuthModeEnvToken || auth.TokenVariable != "GIRA_MCP_GITHUB_TOKEN" || !auth.TokenPresent {
		t.Fatalf("unexpected auth config: %+v", auth)
	}
	if auth.GitHubHost != "github.example.com" {
		t.Fatalf("github host = %q", auth.GitHubHost)
	}
}

func TestMCPCommandRunnerInjectsSelectedTokenAsGHToken(t *testing.T) {
	auth := ResolveMCPAuthConfig([]string{"GIRA_MCP_GITHUB_TOKEN=secret-value", "GH_TOKEN=old-value"})
	runner := MCPCommandRunner{Auth: auth}
	env := runner.commandEnv()
	values := envMap(env)
	if values["GH_TOKEN"] != "secret-value" {
		t.Fatalf("GH_TOKEN = %q", values["GH_TOKEN"])
	}
	if values["GIRA_MCP_AUTH_MODE"] != string(MCPAuthModeEnvToken) {
		t.Fatalf("auth mode env = %q", values["GIRA_MCP_AUTH_MODE"])
	}
}

func TestResolveMCPAuthConfigFallsBackToLocalGH(t *testing.T) {
	auth := ResolveMCPAuthConfig([]string{"PATH=/usr/bin"})
	if auth.Mode != MCPAuthModeLocalGH || auth.TokenPresent || auth.TokenVariable != "" {
		t.Fatalf("unexpected auth config: %+v", auth)
	}
}

func TestBuildMCPAuthReportUsesEnvTokenWithoutRunningGH(t *testing.T) {
	runner := &mcpAuthRunner{}
	report := BuildMCPAuthReport("StatPan/gira", []string{"GITHUB_TOKEN=secret"}, runner)
	if report.Mode != MCPAuthModeEnvToken || report.TokenVariable != "GITHUB_TOKEN" || !report.GHAuthOK {
		t.Fatalf("unexpected report: %+v", report)
	}
	if runner.name != "" {
		t.Fatalf("env-token mode should not run gh auth status: %+v", runner)
	}
	text := FormatMCPAuthReport(report)
	if strings.Contains(text, "secret") || !strings.Contains(text, "GITHUB_TOKEN") {
		t.Fatalf("report should redact token but name variable:\n%s", text)
	}
}

func TestBuildMCPAuthReportReportsLocalGHFailureGuidance(t *testing.T) {
	runner := &mcpAuthRunner{err: errors.New("not logged in")}
	report := BuildMCPAuthReport("StatPan/gira", []string{"GITHUB_HOST=github.example.com"}, runner)
	if report.Mode != MCPAuthModeLocalGH || report.GHAuthOK || report.NextAction != "configure_auth" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if runner.name != "gh" || strings.Join(runner.args, " ") != "auth status --hostname github.example.com" {
		t.Fatalf("runner call = %s %#v", runner.name, runner.args)
	}
	if !strings.Contains(report.NextStep, "GIRA_MCP_GITHUB_TOKEN") {
		t.Fatalf("next step should mention env token: %q", report.NextStep)
	}
}

func TestRedactSecrets(t *testing.T) {
	auth := ResolveMCPAuthConfig([]string{"GIRA_MCP_GITHUB_TOKEN=super-secret"})
	redacted := redactSecrets("failed with super-secret", auth)
	if strings.Contains(redacted, "super-secret") || !strings.Contains(redacted, "[redacted]") {
		t.Fatalf("redacted = %q", redacted)
	}
}
