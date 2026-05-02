package gira

import (
	"fmt"
	"strings"
	"testing"
)

type qualityRunner struct {
	errors map[string]error
}

func (q qualityRunner) Run(name string, args ...string) ([]byte, error) {
	key := strings.Join(append([]string{name}, args...), " ")
	if err, ok := q.errors[key]; ok {
		return nil, err
	}
	return []byte("ok"), nil
}

func TestRunQualityGateReady(t *testing.T) {
	report := RunQualityGate(qualityRunner{errors: map[string]error{}})
	if !report.Ready {
		t.Fatalf("expected ready report: %+v", report)
	}
	if len(report.Blockers) != 0 {
		t.Fatalf("expected no blockers: %+v", report.Blockers)
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
