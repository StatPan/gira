package gira

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
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
	Profile        string               `json:"profile,omitempty"`
	ProfileSource  string               `json:"profile_source,omitempty"`
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
	return RunQualityGateAt(findLocalQualityRoot("."), runner)
}

// RunQualityGateAt resolves a safe, repository-specific local profile before
// executing checks. It deliberately does not combine detected languages: a
// mixed repository must select its own review.local_checks configuration.
func RunQualityGateAt(root string, runner CommandRunner) QualityGateReport {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	profile := resolveLocalQualityProfile(root)
	if profile.configurationNeeded {
		return QualityGateReport{
			Ready:          false,
			EvidenceSource: "local_execution",
			ExecutionMode:  "local_exec",
			Profile:        "configuration_needed",
			ProfileSource:  profile.source,
			Checks: []QualityCheckResult{{
				Name:   "local-review-profile",
				Status: "configuration_needed",
				Hint:   profile.hint,
			}},
			Blockers: []string{"local_review_profile_required"},
		}
	}
	checks := make([]QualityCheckResult, 0, len(profile.checks))
	for _, check := range profile.checks {
		if check.name == "gofmt" && strings.Join(check.command, " ") == "gofmt -l ." {
			checks = append(checks, runGofmtCheck(runner))
			continue
		}
		checks = append(checks, runLocalQualityCheck(runner, check))
	}
	blockers := make([]string, 0)
	ready := true
	for _, c := range checks {
		if c.Status == "failed" {
			ready = false
			blockers = append(blockers, fmt.Sprintf("%s failed", c.Name))
		}
	}
	return QualityGateReport{Ready: ready, EvidenceSource: "local_execution", ExecutionMode: "local_exec", Profile: profile.name, ProfileSource: profile.source, Checks: checks, Blockers: blockers}
}

type localQualityCheck struct {
	name    string
	command []string
	hint    string
}

type localQualityProfile struct {
	name                string
	source              string
	checks              []localQualityCheck
	configurationNeeded bool
	hint                string
}

func resolveLocalQualityProfile(root string) localQualityProfile {
	if configured, ok := configuredLocalQualityProfile(root); ok {
		return configured
	}
	hasGo := qualityPathExists(filepath.Join(root, "go.mod"))
	pythonChecks := detectedPythonChecks(root)
	hasPython := len(pythonChecks) > 0
	if hasGo && hasPython {
		return localQualityProfile{configurationNeeded: true, source: "metadata:go.mod,pyproject.toml", hint: "Multiple local review profiles were detected. Configure review.local_checks in .gira/config.yaml to select the intended checks."}
	}
	if hasGo {
		return localQualityProfile{name: "go", source: "metadata:go.mod", checks: []localQualityCheck{
			{name: "gofmt", command: []string{"gofmt", "-l", "."}, hint: "Run gofmt -w . to apply formatting."},
			{name: "govet", command: []string{"go", "vet", "./..."}, hint: "Run go vet ./... and fix reported diagnostics."},
			{name: "gotest", command: []string{"go", "test", "./..."}, hint: "Run go test ./... and address failing tests."},
			{name: "gotest-race", command: []string{"go", "test", "-race", "./..."}, hint: "Run go test -race ./... and address race/test failures."},
		}}
	}
	if hasPython {
		return localQualityProfile{name: "python", source: "metadata:pyproject.toml", checks: pythonChecks}
	}
	return localQualityProfile{configurationNeeded: true, source: "none", hint: "No safe local review profile was detected. Add review.local_checks to .gira/config.yaml, or add supported Go/Python project metadata."}
}

func configuredLocalQualityProfile(root string) (localQualityProfile, bool) {
	for _, name := range []string{"config.yaml", "config.toml"} {
		path := filepath.Join(root, ".gira", name)
		if !qualityPathExists(path) {
			continue
		}
		config, err := LoadInitConfig(path)
		if err != nil || len(config.Review.LocalChecks) == 0 {
			continue
		}
		checks := make([]localQualityCheck, 0, len(config.Review.LocalChecks))
		for _, check := range config.Review.LocalChecks {
			checks = append(checks, localQualityCheck{name: check.Name, command: append([]string(nil), check.Command...), hint: fmt.Sprintf("Fix %s or update review.local_checks in %s.", check.Name, filepath.ToSlash(filepath.Join(".gira", name)))})
		}
		return localQualityProfile{name: "configured", source: "config:" + filepath.ToSlash(filepath.Join(".gira", name)), checks: checks}, true
	}
	return localQualityProfile{}, false
}

func detectedPythonChecks(root string) []localQualityCheck {
	content, err := os.ReadFile(filepath.Join(root, "pyproject.toml"))
	if err != nil {
		return nil
	}
	var project map[string]any
	if toml.Unmarshal(content, &project) != nil {
		return nil
	}
	tool, _ := project["tool"].(map[string]any)
	checks := []localQualityCheck{}
	if _, ok := tool["ruff"]; ok {
		checks = append(checks, localQualityCheck{name: "ruff", command: []string{"ruff", "check", "."}, hint: "Run ruff check . and fix reported diagnostics."})
	}
	if _, ok := tool["mypy"]; ok {
		checks = append(checks, localQualityCheck{name: "mypy", command: []string{"mypy", "src"}, hint: "Run mypy src and fix reported diagnostics."})
	}
	if _, ok := tool["pytest"]; ok {
		checks = append(checks, localQualityCheck{name: "pytest", command: []string{"pytest"}, hint: "Run pytest and address failing tests."})
	}
	sort.SliceStable(checks, func(i, j int) bool { return checks[i].name < checks[j].name })
	return checks
}

func qualityPathExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func findLocalQualityRoot(path string) string {
	root, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	for {
		if qualityPathExists(filepath.Join(root, "go.mod")) || qualityPathExists(filepath.Join(root, "pyproject.toml")) || qualityPathExists(filepath.Join(root, ".gira", "config.yaml")) || qualityPathExists(filepath.Join(root, ".gira", "config.toml")) {
			return root
		}
		parent := filepath.Dir(root)
		if parent == root {
			return path
		}
		root = parent
	}
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
	return runLocalQualityCheck(runner, localQualityCheck{name: name, command: strings.Split(command, " "), hint: hint})
}

func runLocalQualityCheck(runner CommandRunner, check localQualityCheck) QualityCheckResult {
	command := strings.Join(check.command, " ")
	_, err := runner.Run(check.command[0], check.command[1:]...)
	if err == nil {
		return QualityCheckResult{Name: check.name, Command: command, Status: "passed"}
	}
	if check.name == "gotest-race" && strings.Contains(strings.ToLower(err.Error()), "not supported") {
		return QualityCheckResult{Name: check.name, Command: command, Status: "skipped", Hint: "Race detector unsupported on this platform; continue with non-race checks."}
	}
	return QualityCheckResult{Name: check.name, Command: command, Status: "failed", Hint: check.hint}
}
