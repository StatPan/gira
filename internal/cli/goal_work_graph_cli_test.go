package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/StatPan/gira/internal/gira"
)

func TestRunGoalGraphCompactAndFingerprintApply(t *testing.T) {
	original := newPMWorkGraphReport
	t.Cleanup(func() { newPMWorkGraphReport = original })
	var captured gira.PMWorkGraphInput
	newPMWorkGraphReport = func(input gira.PMWorkGraphInput) (gira.PMWorkGraphReport, error) {
		captured = input
		return gira.PMWorkGraphReport{Command: "goal graph", SchemaVersion: gira.PMWorkGraphReportSchemaVersion, Mode: map[bool]string{true: "apply"}[input.Apply], Repo: input.Repo.FullName(), Goal: gira.GoalStatusIssue{Number: input.Goal}, PlanID: "pwg-123", ExpectedPlanID: input.ExpectedPlanID, Matched: true, Nodes: []gira.PMWorkGraphNode{{ID: "n1", Profile: "delivery", Verification: []gira.PMWorkGraphVerification{{Method: "test", Evidence: "pass"}}}}, Actions: []gira.PMWorkGraphAction{{NodeID: "n1", Action: "create", Status: "planned"}}}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runGoal([]string{"graph", "100", "--repo", "OWNER/repo", "--dry-run", "--compact-json"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), gira.PMWorkGraphCompactSchemaVersion) || !captured.DryRun {
		t.Fatalf("compact failed: code=%d output=%s stderr=%s input=%#v", code, stdout.String(), stderr.String(), captured)
	}
	stdout.Reset()
	stderr.Reset()
	code = runGoal([]string{"graph", "100", "--repo", "OWNER/repo", "--apply"}, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "requires --expect-plan") {
		t.Fatalf("unguarded apply accepted: code=%d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = runGoal([]string{"graph", "100", "--repo", "OWNER/repo", "--apply", "--expect-plan", "pwg-123", "--json"}, &stdout, &stderr)
	if code != 0 || captured.ExpectedPlanID != "pwg-123" || !captured.Apply {
		t.Fatalf("guarded apply failed: code=%d stderr=%s input=%#v", code, stderr.String(), captured)
	}
}
