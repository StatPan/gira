package gira

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestDefaultWorkflow(t *testing.T) {
	w, findings := resolveWorkflow(nil)
	if len(findings) != 0 || len(w.Statuses) != len(defaultStatuses) || w.FinishTarget != "done" {
		t.Fatalf("unexpected default: %+v %+v", w, findings)
	}
	for i, s := range defaultStatuses {
		if w.Statuses[i].Label != s.label || w.Statuses[i].Category != s.category {
			t.Fatalf("status %d: %+v", i, w.Statuses[i])
		}
	}
	if _, findings = resolveWorkflow(&w); len(findings) != 0 {
		t.Fatalf("default invalid: %+v", findings)
	}
	for _, edge := range [][2]string{{"backlog", "ready"}, {"ready", "in-progress"}, {"in-progress", "in-review"}, {"in-review", "in-progress"}, {"in-review", "done"}, {"any", "done"}, {"ready", "blocked"}, {"blocked", "ready"}} {
		found := false
		for _, transition := range w.Transitions {
			for _, from := range transition.From {
				if from == edge[0] && transition.To == edge[1] {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("missing edge %v", edge)
		}
	}
}

func TestWorkflowValidation(t *testing.T) {
	tests := []struct{ name, yaml, code string }{
		{"category missing", "statuses: [{name: ready}, {name: done, category: done}]\nfinish_target: done", "workflow_status_category"},
		{"category invalid", "statuses: [{name: ready, category: invalid}]\nfinish_target: ready", "workflow_status_category"},
		{"duplicate name", "statuses: [{name: ready, category: to_do}, {name: ready, label: 'status:other', category: to_do}]\nfinish_target: ready", "workflow_duplicate_status_name"},
		{"duplicate label", "statuses: [{name: ready, category: to_do}, {name: other, label: 'status:ready', category: done}]\nfinish_target: ready", "workflow_duplicate_status_label"},
		{"label prefix", "statuses: [{name: ready, label: ready, category: to_do}]\nfinish_target: ready", "workflow_status_label"},
		{"source", "statuses: [{name: ready, category: to_do}]\ntransitions: [{from: [missing], to: ready}]\nfinish_target: ready", "workflow_transition_from"},
		{"destination", "statuses: [{name: ready, category: to_do}]\ntransitions: [{from: ready, to: missing}]\nfinish_target: ready", "workflow_transition_to"},
		{"finish", "statuses: [{name: ready, category: to_do}]\nfinish_target: missing", "workflow_finish_target"},
		{"unreachable", "statuses: [{name: ready, category: to_do}, {name: done, category: done}]\nfinish_target: done", "workflow_finish_unreachable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w Workflow
			if err := yaml.Unmarshal([]byte(tt.yaml), &w); err != nil {
				t.Fatal(err)
			}
			_, findings := resolveWorkflow(&w)
			for _, f := range findings {
				if f.ID == tt.code {
					return
				}
			}
			t.Fatalf("missing %s: %+v", tt.code, findings)
		})
	}
}

func TestWorkflowDoctorDefaultAndCustom(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.MkdirAll(filepath.Join(dir, ".gira"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".gira", "config.yaml")
	if err := os.WriteFile(path, []byte("repo: owner/repo\nprofiles: {default: {}}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := localWorkflowFindings(RepoRef{}); len(got) != 0 {
		t.Fatalf("default added findings: %+v", got)
	}
	report := BuildDoctorReport("owner/repo", readyDoctorRunner(), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	for _, check := range report.Checks {
		if strings.HasPrefix(check.ID, "workflow_") && check.ID != "workflow_policy_labels" && check.ID != "workflow_nonconformance" {
			t.Fatalf("default doctor added finding: %+v", check)
		}
	}
	data := "workflow:\n  statuses: [{name: ready, category: to_do}]\n  transitions: [{from: [ready], to: missing}]\n  finish_target: ready\n"
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	got := localWorkflowFindings(RepoRef{})
	if len(got) != 1 || !reflect.DeepEqual(got[0].ID, "workflow_transition_to") {
		t.Fatalf("got %+v", got)
	}
}

func TestWorkflowModelDocExampleValidates(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "workflow-model.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(data)
	start := strings.Index(doc, "```yaml\nworkflow:")
	if start < 0 {
		t.Fatal("workflow yaml example not found")
	}
	block := doc[start+len("```yaml\n"):]
	block = block[:strings.Index(block, "```")]
	var cfg struct {
		Workflow *Workflow `yaml:"workflow"`
	}
	if err := yaml.Unmarshal([]byte(block), &cfg); err != nil {
		t.Fatal(err)
	}
	if _, findings := resolveWorkflow(cfg.Workflow); len(findings) != 0 {
		t.Fatalf("doc example findings: %+v", findings)
	}
}
