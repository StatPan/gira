package gira

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type qualityRunner struct {
	errors  map[string]error
	outputs map[string][]byte
}

func (q qualityRunner) Run(name string, args ...string) ([]byte, error) {
	key := strings.Join(append([]string{name}, args...), " ")
	if err, ok := q.errors[key]; ok {
		return nil, err
	}
	if out, ok := q.outputs[key]; ok {
		return out, nil
	}
	return []byte(""), nil
}

func TestRunQualityGateReady(t *testing.T) {
	report := RunQualityGate(qualityRunner{errors: map[string]error{}})
	if !report.Ready {
		t.Fatalf("expected ready report: %+v", report)
	}
	if report.EvidenceSource != "local_execution" || report.ExecutionMode != "local_exec" {
		t.Fatalf("expected local execution evidence metadata: %+v", report)
	}
	if len(report.Blockers) != 0 {
		t.Fatalf("expected no blockers: %+v", report.Blockers)
	}
}

func TestRunStaticQualityGateBlocksWithoutLocalExecution(t *testing.T) {
	report := RunStaticQualityGate()
	if report.Ready {
		t.Fatalf("static quality gate should block without execution evidence: %+v", report)
	}
	if report.EvidenceSource != "static_policy" || report.ExecutionMode != "no_local_execution" {
		t.Fatalf("unexpected static gate metadata: %+v", report)
	}
	if len(report.Blockers) == 0 || !strings.Contains(report.Blockers[0], "local execution") {
		t.Fatalf("expected local execution blocker: %+v", report.Blockers)
	}
}

func TestRunQualityGateFailedAndHints(t *testing.T) {
	report := RunQualityGate(qualityRunner{errors: map[string]error{
		"go vet ./...": fmt.Errorf("vet failed"),
	}})
	if report.Ready {
		t.Fatalf("expected not ready")
	}
	if len(report.Blockers) == 0 || !strings.Contains(report.Blockers[0], "govet") {
		t.Fatalf("expected govet blocker: %+v", report.Blockers)
	}
	foundHint := false
	for _, check := range report.Checks {
		if check.Name == "govet" {
			foundHint = strings.Contains(check.Hint, "go vet")
		}
	}
	if !foundHint {
		t.Fatalf("expected actionable hint for govet: %+v", report.Checks)
	}
}

func TestRunQualityGateFailsOnFormattingDrift(t *testing.T) {
	report := RunQualityGate(qualityRunner{outputs: map[string][]byte{
		"gofmt -l .": []byte("internal/gira/quality_gate.go\n"),
	}})
	if report.Ready {
		t.Fatalf("expected not ready when formatting drifts")
	}
	for _, check := range report.Checks {
		if check.Name == "gofmt" {
			if check.Status != "failed" {
				t.Fatalf("expected gofmt check to fail, got %q", check.Status)
			}
			if !strings.Contains(check.Hint, "gofmt -w") {
				t.Fatalf("expected actionable gofmt hint, got %q", check.Hint)
			}
			return
		}
	}
	t.Fatalf("missing gofmt check in report: %+v", report.Checks)
}

func TestRunQualityGatePassesWhenFormattingClean(t *testing.T) {
	report := RunQualityGate(qualityRunner{outputs: map[string][]byte{
		"gofmt -l .": []byte("\n"),
	}})
	for _, check := range report.Checks {
		if check.Name == "gofmt" && check.Status != "passed" {
			t.Fatalf("expected gofmt check to pass, got %q", check.Status)
		}
	}
	if !report.Ready {
		t.Fatalf("expected ready report: %+v", report)
	}
}

func TestRunQualityGateSelectsGoProfileFromMetadata(t *testing.T) {
	root := t.TempDir()
	writeQualityFile(t, filepath.Join(root, "go.mod"), "module example.com/checks\n\ngo 1.24\n")
	report := RunQualityGateAt(root, qualityRunner{errors: map[string]error{}})
	if !report.Ready || report.Profile != "go" || report.ProfileSource != "metadata:go.mod" {
		t.Fatalf("expected Go metadata profile, got %+v", report)
	}
	if !hasQualityCheck(report, "gofmt", "gofmt -l .") || !hasQualityCheck(report, "gotest", "go test ./...") {
		t.Fatalf("expected Go checks, got %+v", report.Checks)
	}
}

func TestRunQualityGateSelectsPythonProfileFromMetadata(t *testing.T) {
	root := t.TempDir()
	writeQualityFile(t, filepath.Join(root, "pyproject.toml"), "[project]\nname = \"example\"\nversion = \"0.1.0\"\n\n[tool.ruff]\n\n[tool.mypy]\n\n[tool.pytest.ini_options]\n")
	report := RunQualityGateAt(root, qualityRunner{errors: map[string]error{}})
	if !report.Ready || report.Profile != "python" || report.ProfileSource != "metadata:pyproject.toml" {
		t.Fatalf("expected Python metadata profile, got %+v", report)
	}
	for _, want := range []struct{ name, command string }{{"ruff", "ruff check ."}, {"mypy", "mypy src"}, {"pytest", "pytest"}} {
		if !hasQualityCheck(report, want.name, want.command) {
			t.Fatalf("missing Python check %s: %+v", want.name, report.Checks)
		}
	}
}

func TestRunQualityGateNeedsConfigurationWithoutSafeProfile(t *testing.T) {
	report := RunQualityGateAt(t.TempDir(), qualityRunner{})
	if report.Ready || report.Profile != "configuration_needed" || report.ProfileSource != "none" {
		t.Fatalf("expected configuration-needed report, got %+v", report)
	}
	if len(report.Checks) != 1 || report.Checks[0].Status != "configuration_needed" || len(report.Blockers) != 1 || report.Blockers[0] != "local_review_profile_required" {
		t.Fatalf("expected explicit configuration gap, got %+v", report)
	}
}

func TestRunQualityGateUsesExplicitProfileForMixedRepository(t *testing.T) {
	root := t.TempDir()
	writeQualityFile(t, filepath.Join(root, "go.mod"), "module example.com/checks\n\ngo 1.24\n")
	writeQualityFile(t, filepath.Join(root, "pyproject.toml"), "[tool.ruff]\n")
	writeQualityFile(t, filepath.Join(root, ".gira", "config.yaml"), "profiles:\n  default:\n    labels: []\nreview:\n  local_checks:\n    - name: custom-python\n      command: [\"python\", \"-m\", \"pytest\"]\n")
	report := RunQualityGateAt(root, qualityRunner{errors: map[string]error{}})
	if !report.Ready || report.Profile != "configured" || report.ProfileSource != "config:.gira/config.yaml" {
		t.Fatalf("expected explicit configured profile, got %+v", report)
	}
	if len(report.Checks) != 1 || report.Checks[0].Name != "custom-python" || report.Checks[0].Command != "python -m pytest" {
		t.Fatalf("expected only explicit override check, got %+v", report.Checks)
	}
}

func writeQualityFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func hasQualityCheck(report QualityGateReport, name string, command string) bool {
	for _, check := range report.Checks {
		if check.Name == name && check.Command == command {
			return true
		}
	}
	return false
}
