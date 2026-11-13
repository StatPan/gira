package gira

import (
	"errors"
	"testing"
)

func TestTransitionEngine(t *testing.T) {
	calls := []string{}
	step := func(name string, fail bool) TransitionStep {
		return func(*TransitionContext) error {
			calls = append(calls, name)
			if fail {
				return errors.New(name)
			}
			return nil
		}
	}
	registry := TransitionRegistry{Conditions: map[string]TransitionStep{"condition": step("condition", false)}, Validators: map[string]TransitionStep{"validator": step("validator", false)}, Post: map[string]TransitionStep{"one": step("one", false), "two": step("two", false)}}
	workflow := Workflow{Transitions: []WorkflowTransition{{Name: "test", From: workflowSources{"ready"}, To: "done", Conditions: []string{"condition"}, Validators: []string{"validator"}, Post: []string{"one", "two"}}}}
	t.Run("missing", func(t *testing.T) {
		r := RunTransition(workflow, "other", "done", nil, registry, true)
		if len(r.Blockers) != 1 || r.Blockers[0] != "missing_transition" {
			t.Fatal(r)
		}
	})
	t.Run("condition", func(t *testing.T) {
		calls = nil
		registry.Conditions["condition"] = step("condition", true)
		r := RunTransition(workflow, "ready", "done", nil, registry, true)
		if r.Available || len(calls) != 1 || len(r.Blockers) != 1 {
			t.Fatal(r, calls)
		}
		registry.Conditions["condition"] = step("condition", false)
	})
	t.Run("validator", func(t *testing.T) {
		calls = nil
		registry.Validators["validator"] = step("validator", true)
		r := RunTransition(workflow, "ready", "done", nil, registry, true)
		if len(calls) != 2 || len(r.Blockers) != 1 || len(r.Applied) != 0 {
			t.Fatal(r, calls)
		}
		registry.Validators["validator"] = step("validator", false)
	})
	t.Run("dry run", func(t *testing.T) {
		calls = nil
		r := RunTransition(workflow, "ready", "done", nil, registry, false)
		if len(calls) != 2 || len(r.Planned) != 2 || len(r.Applied) != 0 {
			t.Fatal(r, calls)
		}
	})
	t.Run("post error", func(t *testing.T) {
		calls = nil
		registry.Post["one"] = step("one", true)
		r := RunTransition(workflow, "ready", "done", nil, registry, true)
		if len(calls) != 3 || len(r.Blockers) != 1 || len(r.Applied) != 0 {
			t.Fatal(r, calls)
		}
	})
	for _, source := range []string{"any", "global"} {
		t.Run(source, func(t *testing.T) {
			w := Workflow{Transitions: []WorkflowTransition{{From: workflowSources{source}, To: "done"}}}
			r := RunTransition(w, "ready", "done", nil, registry, true)
			if !r.Available || len(r.Blockers) != 0 {
				t.Fatal(r)
			}
		})
	}
}

func TestTransitionRegistryValidation(t *testing.T) {
	w := defaultWorkflow()
	_, findings := resolveWorkflow(&w)
	if len(findings) != 0 {
		t.Fatal(findings)
	}
	w.Transitions[0].Conditions = []string{"missing_condition"}
	w.Transitions[0].Validators = []string{"missing_validator"}
	w.Transitions[0].Post = []string{"missing_post"}
	_, findings = resolveWorkflow(&w)
	if len(findings) != 3 {
		t.Fatal(findings)
	}
	for _, finding := range findings {
		if finding.ID != "workflow_unknown_step" {
			t.Fatal(findings)
		}
	}
	_, findings = resolveWorkflow(nil)
	if len(findings) != 0 {
		t.Fatal(findings)
	}
}
