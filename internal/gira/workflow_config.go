package gira

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type WorkflowStatus struct {
	Name     string         `yaml:"name"`
	Label    string         `yaml:"label"`
	Category statusCategory `yaml:"category"`
}
type workflowSources []string

func (s *workflowSources) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		var value string
		if err := n.Decode(&value); err != nil {
			return err
		}
		*s = []string{value}
		return nil
	}
	var values []string
	if err := n.Decode(&values); err != nil {
		return err
	}
	*s = values
	return nil
}

type WorkflowTransition struct {
	Name string          `yaml:"name"`
	From workflowSources `yaml:"from"`
	To   string          `yaml:"to"`
}
type Workflow struct {
	Statuses     []WorkflowStatus     `yaml:"statuses"`
	Transitions  []WorkflowTransition `yaml:"transitions"`
	FinishTarget string               `yaml:"finish_target"`
}

func defaultWorkflow() Workflow {
	w := Workflow{FinishTarget: "done"}
	for _, s := range defaultStatuses {
		w.Statuses = append(w.Statuses, WorkflowStatus{Name: strings.TrimPrefix(s.label, "status:"), Label: s.label, Category: s.category})
	}
	w.Transitions = []WorkflowTransition{
		{Name: "prepare", From: workflowSources{"backlog"}, To: "ready"},
		{Name: "start", From: workflowSources{"ready"}, To: "in-progress"},
		{Name: "review", From: workflowSources{"in-progress"}, To: "in-review"},
		{Name: "draft", From: workflowSources{"in-review"}, To: "in-progress"},
		{Name: "finish", From: workflowSources{"in-review"}, To: "done"},
		{Name: "block", From: workflowSources{"backlog", "ready", "in-progress", "in-review"}, To: "blocked"},
		{Name: "unblock", From: workflowSources{"blocked"}, To: "backlog"},
		{Name: "unblock-ready", From: workflowSources{"blocked"}, To: "ready"},
		{Name: "unblock-progress", From: workflowSources{"blocked"}, To: "in-progress"},
		{Name: "unblock-review", From: workflowSources{"blocked"}, To: "in-review"},
		{Name: "supersede", From: workflowSources{"any"}, To: "done"},
	}
	return w
}

func resolveWorkflow(config *Workflow) (Workflow, []DoctorCheck) {
	if config == nil {
		return defaultWorkflow(), nil
	}
	w := *config
	// Labels may be omitted, as in the workflow-model example.
	w.Statuses = append([]WorkflowStatus(nil), config.Statuses...)
	var findings []DoctorCheck
	add := func(id, detail string) {
		findings = append(findings, DoctorCheck{ID: id, Status: DoctorCheckFail, Detail: detail, Remediation: "fix workflow in .gira/config.yaml", FindingClass: DoctorFindingEnvironment})
	}
	names, labels := map[string]bool{}, map[string]bool{}
	for i := range w.Statuses {
		s := &w.Statuses[i]
		if s.Label == "" && s.Name != "" {
			s.Label = "status:" + s.Name
		}
		if s.Name == "" {
			add("workflow_status_name", "status name is required")
		} else if names[s.Name] {
			add("workflow_duplicate_status_name", fmt.Sprintf("duplicate status name %q", s.Name))
		}
		names[s.Name] = true
		labelKey := strings.ToLower(s.Label)
		if labels[labelKey] {
			add("workflow_duplicate_status_label", fmt.Sprintf("duplicate status label %q", s.Label))
		}
		labels[labelKey] = true
		if !strings.HasPrefix(s.Label, "status:") || s.Label == "status:" {
			add("workflow_status_label", fmt.Sprintf("status %q has invalid label %q", s.Name, s.Label))
		}
		if s.Category != statusCategoryToDo && s.Category != statusCategoryInProgress && s.Category != statusCategoryDone {
			add("workflow_status_category", fmt.Sprintf("status %q has invalid category %q", s.Name, s.Category))
		}
	}
	for _, t := range w.Transitions {
		if !names[t.To] || t.To == "" {
			add("workflow_transition_to", fmt.Sprintf("transition %q has undeclared destination %q", t.Name, t.To))
		}
		if len(t.From) == 0 {
			add("workflow_transition_from", fmt.Sprintf("transition %q has no source", t.Name))
		}
		for _, from := range t.From {
			if from != "any" && from != "global" && (!names[from] || from == "") {
				add("workflow_transition_from", fmt.Sprintf("transition %q has undeclared source %q", t.Name, from))
			}
		}
	}
	if !names[w.FinishTarget] || w.FinishTarget == "" {
		add("workflow_finish_target", fmt.Sprintf("undeclared finish target %q", w.FinishTarget))
		return w, findings
	}
	reachable := map[string]bool{}
	for _, s := range w.Statuses {
		if s.Category == statusCategoryToDo && s.Name != "" {
			reachable[s.Name] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, t := range w.Transitions {
			if !names[t.To] || reachable[t.To] {
				continue
			}
			for _, from := range t.From {
				if reachable[from] || ((from == "any" || from == "global") && len(reachable) > 0) {
					reachable[t.To] = true
					changed = true
					break
				}
			}
		}
	}
	if !reachable[w.FinishTarget] {
		add("workflow_finish_unreachable", fmt.Sprintf("finish target %q is unreachable from to_do statuses", w.FinishTarget))
	}
	return w, findings
}

// Read only the current checkout's config, without changing other commands' parsing.
func localWorkflowFindings(repo RepoRef) []DoctorCheck {
	dir, err := os.Getwd()
	if err != nil {
		return nil
	}
	for {
		path := filepath.Join(dir, ".gira", "config.yaml")
		data, readErr := os.ReadFile(path)
		if readErr == nil {
			var cfg struct {
				Repo     string    `yaml:"repo"`
				Workflow *Workflow `yaml:"workflow"`
			}
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return []DoctorCheck{{ID: "workflow_config_parse", Status: DoctorCheckFail, Detail: err.Error(), Remediation: "fix workflow in .gira/config.yaml", FindingClass: DoctorFindingEnvironment}}
			}
			if cfg.Repo != "" && repo.FullName() != "/" {
				configured, e := ParseRepoRef(cfg.Repo)
				if e != nil || !sameRepoRef(configured, repo) {
					return nil
				}
			}
			_, findings := resolveWorkflow(cfg.Workflow)
			return findings
		}
		if !os.IsNotExist(readErr) {
			return []DoctorCheck{{ID: "workflow_config_read", Status: DoctorCheckFail, Detail: readErr.Error(), Remediation: "make .gira/config.yaml readable", FindingClass: DoctorFindingEnvironment}}
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil
}
