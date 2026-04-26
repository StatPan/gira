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

func TestMergedPRWithoutClosingKeywordDoesNotCloseByBranchFallback(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	snapshot := ProjectTransitionSnapshot{
		Issues: []ProjectTransitionIssue{
			{
				Number: 33,
				Title:  "Closed manually",
				State:  "closed",
				Labels: []string{"status:in-review"},
			},
		},
		PullRequests: []ProjectTransitionPullRequest{
			{
				Number:   300,
				State:    "closed",
				Draft:    false,
				MergedAt: strPtr("2026-04-26T12:00:00Z"),
				Body:     "Refactors follow-up work",
				HeadRef:  "feat/issue-33-project-transitions-dry-run",
			},
		},
	}

	report, err := BuildProjectTransitionsReport("StatPan/gira", snapshot, now)
	if err != nil {
		t.Fatalf("BuildProjectTransitionsReport returned error: %v", err)
	}

	apply := findTransition(report.Transitions, "pr_merged_closes_issue", "issue", "33", "apply")
	if apply != nil {
		t.Fatalf("unexpected apply transition: %+v", *apply)
	}
	skip := findTransition(report.Transitions, "pr_merged_closes_issue", "issue", "33", "skip")
	if skip != nil {
		t.Fatalf("unexpected skip transition: %+v", *skip)
	}
}

func TestBlockedRemovedRecomputesToInReviewWhenNonDraftPRExists(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	snapshot := ProjectTransitionSnapshot{
		Issues: []ProjectTransitionIssue{
			{
				Number: 44,
				Title:  "Unblock with PR ready",
				State:  "open",
				Labels: []string{"status:blocked"},
			},
		},
		PullRequests: []ProjectTransitionPullRequest{
			{
				Number:  301,
				State:   "open",
				Draft:   false,
				Body:    "Fixes #44",
				HeadRef: "feat/issue-44-unblock",
			},
		},
	}

	report, err := BuildProjectTransitionsReport("StatPan/gira", snapshot, now)
	if err != nil {
		t.Fatalf("BuildProjectTransitionsReport returned error: %v", err)
	}

	apply := findTransition(report.Transitions, "blocked_removed", "issue", "44", "apply")
	if apply == nil {
		t.Fatalf("missing blocked_removed apply transition in %+v", report.Transitions)
	}
	if apply.To != "In review" {
		t.Fatalf("blocked_removed To = %q, want %q", apply.To, "In review")
	}
	if apply.ConflictResolution != "missing_previous_state_recomputed" {
		t.Fatalf("blocked_removed conflict resolution = %q, want %q", apply.ConflictResolution, "missing_previous_state_recomputed")
	}
}

func TestNonDraftPRBlockedIssueRecordsExplicitBlockedOverrideSkip(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	snapshot := ProjectTransitionSnapshot{
		Issues: []ProjectTransitionIssue{
			{
				Number: 52,
				Title:  "Blocked issue with open PR",
				State:  "open",
				Labels: []string{"status:blocked", "blocked"},
			},
		},
		PullRequests: []ProjectTransitionPullRequest{
			{
				Number:  302,
				State:   "open",
				Draft:   false,
				Body:    "Fixes #52",
				HeadRef: "feat/issue-52",
			},
		},
	}

	report, err := BuildProjectTransitionsReport("StatPan/gira", snapshot, now)
	if err != nil {
		t.Fatalf("BuildProjectTransitionsReport returned error: %v", err)
	}

	skip := findTransition(report.Transitions, "pr_ready_for_review", "issue", "52", "skip")
	if skip == nil {
		skip = findTransition(report.Transitions, "pr_opened", "issue", "52", "skip")
	}
	if skip == nil {
		t.Fatalf("missing blocked override review skip in %+v", report.Transitions)
	}
	if skip.ConflictResolution != "blocked_overrides_review" {
		t.Fatalf("blocked override conflict resolution = %q, want %q", skip.ConflictResolution, "blocked_overrides_review")
	}
}

func TestClosedIssueWithBlockedLabelDoesNotTransitionBackToBlocked(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	snapshot := ProjectTransitionSnapshot{
		Issues: []ProjectTransitionIssue{
			{
				Number: 77,
				Title:  "Closed issue with lingering blocked label",
				State:  "closed",
				Labels: []string{"blocked"},
			},
		},
	}

	report, err := BuildProjectTransitionsReport("StatPan/gira", snapshot, now)
	if err != nil {
		t.Fatalf("BuildProjectTransitionsReport returned error: %v", err)
	}

	apply := findTransition(report.Transitions, "blocked_added", "issue", "77", "apply")
	if apply != nil {
		t.Fatalf("unexpected blocked_added apply transition for closed issue: %+v", *apply)
	}
}

func TestProjectTransitionsReportNotesIssueLabelStatusInferenceOnly(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	snapshot := ProjectTransitionSnapshot{
		Issues: []ProjectTransitionIssue{
			{Number: 61, Title: "No managed status", State: "open"},
		},
	}

	report, err := BuildProjectTransitionsReport("StatPan/gira", snapshot, now)
	if err != nil {
		t.Fatalf("BuildProjectTransitionsReport returned error: %v", err)
	}

	apply := findTransition(report.Transitions, "issue_open_default", "issue", "61", "apply")
	if apply == nil {
		t.Fatalf("missing issue_open_default apply transition in %+v", report.Transitions)
	}
	if !strings.Contains(strings.ToLower(apply.Reason), "managed status") {
		t.Fatalf("unexpected reason: %q", apply.Reason)
	}
}

func findTransition(items []ProjectTransitionPlanItem, ruleID, targetType, targetID, decision string) *ProjectTransitionPlanItem {
	for i := range items {
		item := &items[i]
		if item.RuleID == ruleID && item.TargetType == targetType && item.TargetID == targetID && item.Decision == decision {
			return item
		}
	}
	return nil
}

func strPtr(value string) *string {
	return &value
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
