package gira

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

type transitionFixture struct {
	Name     string                       `json:"name"`
	Snapshot ProjectTransitionSnapshot    `json:"snapshot"`
	Expect   transitionFixtureExpectation `json:"expect"`
}

type transitionFixtureExpectation struct {
	AppliedRuleIDs []string `json:"applied_rule_ids"`
	AppliedTargets []string `json:"applied_targets"`
	SkippedRuleIDs []string `json:"skipped_rule_ids"`
}

func TestBuildProjectTransitionsReportFixtures(t *testing.T) {
	fixtures := loadTransitionFixtures(t)
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			report, err := BuildProjectTransitionsReport("StatPan/gira", fixture.Snapshot, now)
			if err != nil {
				t.Fatalf("BuildProjectTransitionsReport returned error: %v", err)
			}

			appliedRules := make([]string, 0)
			appliedTargets := make([]string, 0)
			skippedRules := make([]string, 0)
			for _, transition := range report.Transitions {
				target := transition.TargetType + "#" + transition.TargetID
				switch transition.Decision {
				case "apply":
					appliedRules = append(appliedRules, transition.RuleID)
					appliedTargets = append(appliedTargets, target)
				case "skip":
					skippedRules = append(skippedRules, transition.RuleID)
				}
			}
			sort.Strings(appliedRules)
			sort.Strings(appliedTargets)
			sort.Strings(skippedRules)

			wantAppliedRules := append([]string(nil), fixture.Expect.AppliedRuleIDs...)
			wantAppliedTargets := append([]string(nil), fixture.Expect.AppliedTargets...)
			wantSkippedRules := append([]string(nil), fixture.Expect.SkippedRuleIDs...)
			sort.Strings(wantAppliedRules)
			sort.Strings(wantAppliedTargets)
			sort.Strings(wantSkippedRules)

			if !reflect.DeepEqual(appliedRules, wantAppliedRules) {
				t.Fatalf("applied rules = %v, want %v", appliedRules, wantAppliedRules)
			}
			if !reflect.DeepEqual(appliedTargets, wantAppliedTargets) {
				t.Fatalf("applied targets = %v, want %v", appliedTargets, wantAppliedTargets)
			}
			if !reflect.DeepEqual(normalizeStringSlice(skippedRules), normalizeStringSlice(wantSkippedRules)) {
				t.Fatalf("skipped rules = %v, want %v", skippedRules, wantSkippedRules)
			}
		})
	}
}

func TestProjectTransitionsDeterministicTextAndJSON(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	snapshot := ProjectTransitionSnapshot{
		Issues: []ProjectTransitionIssue{
			{Number: 44, Title: "Deterministic", State: "open", Labels: []string{"status:ready", "blocked"}},
		},
	}

	reportA, err := BuildProjectTransitionsReport("StatPan/gira", snapshot, now)
	if err != nil {
		t.Fatalf("BuildProjectTransitionsReport returned error: %v", err)
	}
	reportB, err := BuildProjectTransitionsReport("StatPan/gira", snapshot, now)
	if err != nil {
		t.Fatalf("BuildProjectTransitionsReport returned error: %v", err)
	}

	textA := FormatProjectTransitionsPlan(reportA)
	textB := FormatProjectTransitionsPlan(reportB)
	if textA != textB {
		t.Fatalf("text output changed across identical runs:\n%s\n---\n%s", textA, textB)
	}
	if !strings.Contains(textA, "project transitions plan:") {
		t.Fatalf("missing transitions header:\n%s", textA)
	}
	if !strings.Contains(textA, "apply issue#44 blocked_added: Ready -> Blocked") {
		t.Fatalf("missing expected transition line:\n%s", textA)
	}

	jsonA, err := json.MarshalIndent(reportA, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent returned error: %v", err)
	}
	jsonB, err := json.MarshalIndent(reportB, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent returned error: %v", err)
	}
	if string(jsonA) != string(jsonB) {
		t.Fatalf("json output changed across identical runs:\n%s\n---\n%s", jsonA, jsonB)
	}
}

func loadTransitionFixtures(t *testing.T) []transitionFixture {
	t.Helper()
	path := filepath.Join("testdata", "project_transitions", "cases.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture file: %v", err)
	}
	var fixtures []transitionFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatalf("parse fixture file: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("fixture file returned no cases")
	}
	return fixtures
}

func normalizeStringSlice(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}
