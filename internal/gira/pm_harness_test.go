package gira

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPMBootstrapIsBoundedDeterministicAndResumable(t *testing.T) {
	source := PMWorkGraphSource{SchemaVersion: PMWorkGraphSourceSchemaVersion, Nodes: []PMWorkGraphNode{{ID: "build", Title: "Build", Purpose: "Deliver the bounded outcome", Profile: "delivery", ParentOutcome: "goal:100", Size: "small", Uncertainty: "resolved", Verification: []PMWorkGraphVerification{{Method: "go test", Evidence: "passing suite"}}}}}
	body := workGraphGoalBody(t, source)
	input := PMBootstrapInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Ticket: 100, Role: "ai", Authority: []string{"issue:read", "issue:read", "report:read"}, Budget: 6000}
	first, err := BuildPMBootstrapReport(input, &workGraphRunner{body: body})
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildPMBootstrapReport(input, &workGraphRunner{body: body})
	if err != nil {
		t.Fatal(err)
	}
	one, _ := json.Marshal(first)
	two, _ := json.Marshal(second)
	if string(one) != string(two) || first.SessionID == "" || first.Role != "ai" {
		t.Fatalf("bootstrap is not deterministic/resumable: %#v", first)
	}
	if len(one) > 6000 || first.Budget.Characters != len(one) || len(first.Context) != 7 {
		t.Fatalf("bootstrap budget/source contract failed: bytes=%d report=%#v", len(one), first)
	}
	if !first.CurrentPlan.Compiled || first.CurrentPlan.WorkGraphPlanID == "" || len(first.Protocol) < 6 {
		t.Fatalf("bootstrap omitted compile/plan/protocol state: %#v", first)
	}
	for _, transition := range first.Protocol {
		if transition.Mutation && strings.TrimSpace(transition.Gate) == "" {
			t.Fatalf("mutation lacks an explicit gate: %#v", transition)
		}
		if containsPMValue([]string{"plan", "replan"}, transition.Stage) && !strings.Contains(transition.Gate, "expect-plan") {
			t.Fatalf("graph mutation lacks fingerprint gate: %#v", transition)
		}
	}
}

func TestPMConformanceSeparatesProtocolFromSemanticQualityAndContainsWeakHost(t *testing.T) {
	report := BuildPMConformanceReport(nil)
	if !report.ProtocolCompliant || report.SemanticQuality != "reported_separately" || report.Summary.Runs != 3 || report.Summary.HumanRuns != 1 || report.Summary.AIConfigurations != 2 {
		t.Fatalf("unexpected conformance summary: %#v", report)
	}
	if report.Summary.RecordedFailures != 5 || report.Summary.UnsafeMutations != 0 || report.Runs[2].SemanticQuality != "limited" {
		t.Fatalf("weak host failures were not safely recorded: %#v", report)
	}
}

func TestPMConformanceRejectsStaleUnsafeUnsupportedAndPrivateRun(t *testing.T) {
	run := BuiltinPMConformanceRuns()[1]
	run.PolicyVersion = "gira-pm-policy/v0"
	run.Claims = append(run.Claims, PMConformanceClaim{Claim: "unsupported"})
	run.FailureAttempts = []PMConformanceFailureAttempt{{Mode: "authority_overreach", Blocked: false}}
	run.Privacy.TokenProductivityScoring = true
	report := BuildPMConformanceReport([]PMConformanceRun{run})
	if report.ProtocolCompliant || report.Summary.UnsafeMutations != 1 {
		t.Fatalf("unsafe run passed: %#v", report)
	}
	joined := strings.Join(report.Runs[0].Findings, " ")
	for _, want := range []string{"stale_or_unsupported_contract", "unsupported_claim", "unsafe_mutation", "privacy_boundary_violation"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("findings missing %s: %s", want, joined)
		}
	}
}

func TestPMConformanceRejectsExplicitlyMissingEvidence(t *testing.T) {
	report := BuildPMConformanceReport([]PMConformanceRun{})
	if report.ProtocolCompliant || report.Summary.Runs != 0 {
		t.Fatalf("missing conformance evidence passed: %#v", report)
	}
}
