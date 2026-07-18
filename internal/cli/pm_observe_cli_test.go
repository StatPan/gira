package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/StatPan/gira/internal/gira"
)

type pmObserveCLIRunner struct{}

func (pmObserveCLIRunner) Run(name string, args ...string) ([]byte, error) {
	return []byte("StatPan/gira\n"), nil
}

func TestPMObserveCLIAndGuardedReplan(t *testing.T) {
	originalObserve, originalReplan, originalRepo := newPMObserveReport, newPMReplanReport, repoContextRunner
	t.Cleanup(func() {
		newPMObserveReport, newPMReplanReport, repoContextRunner = originalObserve, originalReplan, originalRepo
	})
	repoContextRunner = pmObserveCLIRunner{}
	newPMObserveReport = func(input gira.PMObserveInput) (gira.PMObserveReport, error) {
		return gira.PMObserveReport{Command: "pm observe", SchemaVersion: gira.PMObserveSchemaVersion, Repo: input.Repo.FullName(), Ticket: input.Ticket, Actions: []gira.PMObserveAction{{Kind: "continue", Target: "goal:865"}}}, nil
	}
	newPMReplanReport = func(input gira.PMReplanInput) (gira.PMReplanReport, error) {
		return gira.PMReplanReport{Command: "pm replan", SchemaVersion: gira.PMReplanSchemaVersion, Repo: input.Repo.FullName(), Ticket: input.Ticket, Mode: map[bool]string{true: "apply", false: "dry_run"}[input.Apply], PlanID: "pmr-stable", ExpectedPlanID: input.ExpectedPlanID, Matched: true}, nil
	}
	var stdout, stderr bytes.Buffer
	if code := runPM([]string{"observe", "--repo", "StatPan/gira", "--ticket", "865", "--json"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), `"schema_version": "pm-observe-report/v1"`) {
		t.Fatalf("observe code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runPM([]string{"replan", "--repo", "StatPan/gira", "--ticket", "865", "--apply", "--json"}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "--expect-plan") {
		t.Fatalf("unguarded apply code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runPM([]string{"replan", "--repo", "StatPan/gira", "--ticket", "865", "--apply", "--expect-plan", "pmr-stable", "--json"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), `"mode": "apply"`) {
		t.Fatalf("guarded apply code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}
