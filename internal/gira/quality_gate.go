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
	Ready    bool                 `json:"ready"`
	Checks   []QualityCheckResult `json:"checks"`
	Blockers []string             `json:"blockers"`
}

func RunQualityGate(runner CommandRunner) QualityGateReport {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	checks := []QualityCheckResult{
		runQualityCheck(runner, "gofmt", "gofmt -w .", "Run gofmt -w . to apply formatting."),
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
	return QualityGateReport{Ready: ready, Checks: checks, Blockers: blockers}
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
