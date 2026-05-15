package gira

import (
	"fmt"
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
