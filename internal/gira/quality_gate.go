package gira

import (
	"fmt"
	"strings"
)

type QualityCheckResult struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Status  string `json:"status"`
	Hint    string `json:"hint,omitempty"`
}

type QualityGateReport struct {
	Ready          bool                 `json:"ready"`
	EvidenceSource string               `json:"evidence_source"`
	ExecutionMode  string               `json:"execution_mode"`
	Checks         []QualityCheckResult `json:"checks"`
	Blockers       []string             `json:"blockers"`
}

func RunStaticQualityGate() QualityGateReport {
	return QualityGateReport{
		Ready:          false,
		EvidenceSource: "static_policy",
		ExecutionMode:  "no_local_execution",
		Checks: []QualityCheckResult{
			{Name: "local-exec", Command: "gira review gate --local-exec", Status: "blocked", Hint: "Default review gate does not execute repository code. Use --local-exec only in a trusted checkout, or rely on ticket checks for CI evidence."},
		},
		Blockers: []string{"local execution not enabled"},
	}
}

func RunQualityGate(runner CommandRunner) QualityGateReport {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	checks := []QualityCheckResult{
		runGofmtCheck(runner),
		runQualityCheck(runner, "govet", "go vet ./...", "Run go vet ./... and fix reported diagnostics."),
		runQualityCheck(runner, "gotest", "go test ./...", "Run go test ./... and address failing tests."),
		runQualityCheck(runner, "gotest-race", "go test -race ./...", "Run go test -race ./... and address race/test failures."),
	}
	blockers := make([]string, 0)
	ready := true
	for _, c := range checks {
		if c.Status == "failed" {
			ready = false
			blockers = append(blockers, fmt.Sprintf("%s failed", c.Name))
		}
	}
	return QualityGateReport{Ready: ready, EvidenceSource: "local_execution", ExecutionMode: "local_exec", Checks: checks, Blockers: blockers}
}

func runGofmtCheck(runner CommandRunner) QualityCheckResult {
	const command = "gofmt -l ."
	out, err := runner.Run("gofmt", "-l", ".")
	if err != nil {
		return QualityCheckResult{Name: "gofmt", Command: command, Status: "failed", Hint: "Run gofmt -w . to apply formatting."}
	}
	if strings.TrimSpace(string(out)) != "" {
		return QualityCheckResult{Name: "gofmt", Command: command, Status: "failed", Hint: "Run gofmt -w . to apply formatting."}
	}
	return QualityCheckResult{Name: "gofmt", Command: command, Status: "passed"}
}

func runQualityCheck(runner CommandRunner, name string, command string, hint string) QualityCheckResult {
	parts := strings.Split(command, " ")
	_, err := runner.Run(parts[0], parts[1:]...)
	if err == nil {
		return QualityCheckResult{Name: name, Command: command, Status: "passed"}
	}
	if name == "gotest-race" && strings.Contains(strings.ToLower(err.Error()), "not supported") {
		return QualityCheckResult{Name: name, Command: command, Status: "skipped", Hint: "Race detector unsupported on this platform; continue with non-race checks."}
	}
	return QualityCheckResult{Name: name, Command: command, Status: "failed", Hint: hint}
}
