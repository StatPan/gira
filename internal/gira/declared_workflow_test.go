package gira

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDeclaredStatusMove(t *testing.T) {
	w := Workflow{
		Statuses: []WorkflowStatus{
			{Name: "ready", Label: "status:queued", Category: statusCategoryToDo},
			{Name: "doing", Label: "status:working", Category: statusCategoryInProgress},
			{Name: "done", Label: "status:finished", Category: statusCategoryDone},
		},
		Transitions: []WorkflowTransition{
			{Name: "start", From: workflowSources{"ready"}, To: "doing"},
			{Name: "complete", From: workflowSources{"global"}, To: "done"},
		},
	}
	for _, tc := range []struct {
		name, command, label, from, to, target, blocker string
		noOp                                            bool
	}{
		{"custom target", "start", "status:queued", "ready", "doing", "status:working", "", false},
		{"same status", "start", "status:working", "doing", "doing", "status:working", "", true},
		{"global fallback", "finish", "status:working", "doing", "done", "status:finished", "", false},
		{"missing", "review", "status:queued", "ready", "in-review", "", "missing_transition", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := resolveDeclaredStatusMove(w, tc.command, []string{tc.label})
			if m.from != tc.from || m.to != tc.to || m.label != tc.target || m.blocker != tc.blocker || m.noOp != tc.noOp {
				t.Fatalf("unexpected move: %+v", m)
			}
		})
	}
}

func TestLoadDeclaredWorkflow(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Mkdir(filepath.Join(dir, ".gira"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".gira", "config.yaml")
	for _, tc := range []struct {
		name, config      string
		declared, invalid bool
	}{
		{"absent", "repo: owner/repo\n", false, false},
		{"valid", "repo: owner/repo\nworkflow:\n  statuses: [{name: ready, label: 'status:custom', category: to_do}]\n  finish_target: ready\n", true, false},
		{"invalid", "repo: owner/repo\nworkflow:\n  statuses: [{name: ready}]\n  finish_target: ready\n", true, true},
		{"parse error", "workflow: [\n", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tc.config), 0644); err != nil {
				t.Fatal(err)
			}
			w, declared, findings := loadDeclaredWorkflow(RepoRef{})
			if declared != tc.declared || (len(findings) > 0) != tc.invalid {
				t.Fatalf("declared=%v findings=%+v", declared, findings)
			}
			if tc.name == "valid" && w.Statuses[0].Label != "status:custom" {
				t.Fatalf("custom label lost: %+v", w.Statuses)
			}
		})
	}
}
