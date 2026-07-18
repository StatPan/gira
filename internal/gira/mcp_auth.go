package gira

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const MCPAuthReportSchemaVersion = "gira-mcp-auth/v1"

type MCPAuthMode string

const (
	MCPAuthModeEnvToken MCPAuthMode = "env-token"
	MCPAuthModeLocalGH  MCPAuthMode = "local-gh"
)

type MCPAuthConfig struct {
	Mode          MCPAuthMode
	TokenVariable string
	TokenPresent  bool
	GitHubHost    string
	env           []string
}

type MCPAuthReport struct {
	SchemaVersion string             `json:"schema_version"`
	Command       string             `json:"command"`
	Repo          string             `json:"repo,omitempty"`
	Mode          MCPAuthMode        `json:"mode"`
	TokenVariable string             `json:"token_variable,omitempty"`
	TokenPresent  bool               `json:"token_present"`
	GitHubHost    string             `json:"github_host,omitempty"`
	GHAuthOK      bool               `json:"gh_auth_ok"`
	PMHarness     MCPPMHarnessParity `json:"pm_harness"`
	Warnings      []string           `json:"warnings,omitempty"`
	NextAction    string             `json:"next_action"`
	NextStep      string             `json:"next_step"`
}

type MCPPMHarnessParity struct {
	Ready            bool     `json:"ready"`
	SchemaCurrent    bool     `json:"schema_current"`
	EvidencePresent  bool     `json:"conformance_evidence_present"`
	PolicyVersion    string   `json:"policy_version"`
	ProtocolVersion  string   `json:"protocol_version"`
	FocusedTools     []string `json:"focused_tools"`
	MissingTools     []string `json:"missing_tools,omitempty"`
	ConformanceRuns  int      `json:"conformance_runs"`
	AIConfigurations int      `json:"ai_configurations"`
	UnsafeMutations  int      `json:"unsafe_mutations"`
}

type MCPCommandRunner struct {
	Auth MCPAuthConfig
}

func NewMCPCommandRunnerFromEnv() MCPCommandRunner {
	return MCPCommandRunner{Auth: ResolveMCPAuthConfig(os.Environ())}
}

func ResolveMCPAuthConfig(env []string) MCPAuthConfig {
	values := envMap(env)
	host := strings.TrimSpace(values["GITHUB_HOST"])
	for _, name := range []string{"GIRA_MCP_GITHUB_TOKEN", "GITHUB_TOKEN", "GH_TOKEN"} {
		if strings.TrimSpace(values[name]) != "" {
			return MCPAuthConfig{Mode: MCPAuthModeEnvToken, TokenVariable: name, TokenPresent: true, GitHubHost: host, env: append([]string(nil), env...)}
		}
	}
	return MCPAuthConfig{Mode: MCPAuthModeLocalGH, GitHubHost: host, env: append([]string(nil), env...)}
}

func (r MCPCommandRunner) Run(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = r.commandEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		detail := redactSecrets(strings.TrimSpace(stderr.String()), r.Auth)
		if detail != "" {
			return nil, fmt.Errorf("%s: %w", detail, err)
		}
		return nil, err
	}
	return output, nil
}

func (r MCPCommandRunner) commandEnv() []string {
	env := append([]string(nil), r.Auth.env...)
	values := envMap(env)
	if r.Auth.Mode == MCPAuthModeEnvToken && r.Auth.TokenVariable != "" {
		token := strings.TrimSpace(values[r.Auth.TokenVariable])
		if token != "" {
			env = setEnvValue(env, "GH_TOKEN", token)
			env = setEnvValue(env, "GIRA_MCP_AUTH_MODE", string(MCPAuthModeEnvToken))
		}
	}
	if r.Auth.Mode == MCPAuthModeLocalGH {
		env = setEnvValue(env, "GIRA_MCP_AUTH_MODE", string(MCPAuthModeLocalGH))
	}
	return env
}

func BuildMCPAuthReport(repo string, env []string, runner CommandRunner) MCPAuthReport {
	auth := ResolveMCPAuthConfig(env)
	report := MCPAuthReport{
		SchemaVersion: MCPAuthReportSchemaVersion,
		Command:       "mcp doctor",
		Repo:          strings.TrimSpace(repo),
		Mode:          auth.Mode,
		TokenVariable: auth.TokenVariable,
		TokenPresent:  auth.TokenPresent,
		GitHubHost:    auth.GitHubHost,
		NextAction:    "ready",
		NextStep:      "gira mcp serve",
		PMHarness:     buildMCPPMHarnessParity(),
	}
	if auth.Mode == MCPAuthModeEnvToken {
		report.GHAuthOK = true
		report.Warnings = append(report.Warnings, "token value is intentionally not displayed or persisted")
		return report
	}
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	args := []string{"auth", "status"}
	if auth.GitHubHost != "" {
		args = append(args, "--hostname", auth.GitHubHost)
	}
	if _, err := runner.Run("gh", args...); err != nil {
		report.GHAuthOK = false
		report.NextAction = "configure_auth"
		report.NextStep = "run gh auth login or set GIRA_MCP_GITHUB_TOKEN in the MCP client environment"
		report.Warnings = append(report.Warnings, "gh authentication is unavailable or insufficient")
		return report
	}
	report.GHAuthOK = true
	return report
}

func FormatMCPAuthReport(report MCPAuthReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "mcp doctor: mode=%s auth_ok=%t\n", report.Mode, report.GHAuthOK)
	if report.Repo != "" {
		fmt.Fprintf(&b, "repo: %s\n", report.Repo)
	}
	if report.GitHubHost != "" {
		fmt.Fprintf(&b, "github host: %s\n", report.GitHubHost)
	}
	if report.TokenVariable != "" {
		fmt.Fprintf(&b, "token variable: %s present=%t\n", report.TokenVariable, report.TokenPresent)
	}
	fmt.Fprintf(&b, "pm harness: ready=%t policy=%s protocol=%s ai_configs=%d unsafe_mutations=%d\n", report.PMHarness.Ready, report.PMHarness.PolicyVersion, report.PMHarness.ProtocolVersion, report.PMHarness.AIConfigurations, report.PMHarness.UnsafeMutations)
	for _, warning := range report.Warnings {
		fmt.Fprintf(&b, "warning: %s\n", warning)
	}
	fmt.Fprintf(&b, "next action: %s\n", report.NextAction)
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}

func buildMCPPMHarnessParity() MCPPMHarnessParity {
	required := []string{"gira_pm_bootstrap", "gira_pm_compile", "gira_pm_observe", "gira_pm_replan_plan", "gira_pm_validate", "gira_pm_report"}
	available := map[string]bool{}
	for _, tool := range MCPToolSpecs() {
		available[tool.Name] = true
	}
	parity := MCPPMHarnessParity{PolicyVersion: PMHarnessPolicyVersion, ProtocolVersion: PMHarnessProtocolVersion, FocusedTools: required}
	for _, name := range required {
		if !available[name] {
			parity.MissingTools = append(parity.MissingTools, name)
		}
	}
	conformance := BuildPMConformanceReport(nil)
	parity.ConformanceRuns = conformance.Summary.Runs
	parity.AIConfigurations = conformance.Summary.AIConfigurations
	parity.UnsafeMutations = conformance.Summary.UnsafeMutations
	parity.SchemaCurrent = conformance.PolicyVersion == PMHarnessPolicyVersion && conformance.ProtocolVersion == PMHarnessProtocolVersion
	parity.EvidencePresent = parity.ConformanceRuns >= 3 && parity.AIConfigurations >= 2
	parity.Ready = len(parity.MissingTools) == 0 && parity.SchemaCurrent && parity.EvidencePresent && conformance.ProtocolCompliant && parity.UnsafeMutations == 0
	return parity
}

func envMap(env []string) map[string]string {
	out := map[string]string{}
	for _, item := range env {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			out[key] = value
		}
	}
	return out
}

func setEnvValue(env []string, key string, value string) []string {
	prefix := key + "="
	for i, item := range env {
		if strings.HasPrefix(item, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func redactSecrets(message string, auth MCPAuthConfig) string {
	if message == "" || auth.TokenVariable == "" {
		return message
	}
	values := envMap(auth.env)
	token := strings.TrimSpace(values[auth.TokenVariable])
	if token == "" {
		return message
	}
	return strings.ReplaceAll(message, token, "[redacted]")
}
