package gira

import "testing"

func TestBuildGraphValidationReportDetectsCoreRules(t *testing.T) {
	report := BuildGraphValidationReport("StatPan/gira", []GraphIssue{
		{Number: 1, State: "open", Labels: []string{"type:task"}, Body: "depends_on: #2\nblocks: #9"},
		{Number: 2, State: "open", Labels: []string{"status:done"}, Body: "depends_on: #1"},
	})
	if report.Counts.Diagnostics == 0 {
		t.Fatalf("expected diagnostics")
	}
	want := map[string]bool{
		"missing_parent":            false,
		"unresolved_blocker":        false,
		"broken_blocks":             false,
		"dependency_cycle":          false,
		"done_with_open_dependency": false,
	}
	for _, d := range report.Diagnostics {
		if _, ok := want[d.RuleID]; ok {
			want[d.RuleID] = true
		}
	}
	for rule, seen := range want {
		if !seen {
			t.Fatalf("missing rule %s in diagnostics: %+v", rule, report.Diagnostics)
		}
	}
}

func TestParseIssueLinks(t *testing.T) {
	links := parseIssueLinks("Parent: #7\nDepends on: #3, #4\nBlocks: #10")
	if links.Parent != 7 || len(links.DependsOn) != 2 || len(links.Blocks) != 1 {
		t.Fatalf("unexpected links: %+v", links)
	}
}
