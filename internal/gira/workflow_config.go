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
	Name       string          `yaml:"name"`
	From       workflowSources `yaml:"from"`
	To         string          `yaml:"to"`
	Conditions []string        `yaml:"conditions"`
	Validators []string        `yaml:"validators"`
	Post       []string        `yaml:"post"`
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
		{Name: "start", From: workflowSources{"ready"}, To: "in-progress", Conditions: []string{"open_issue", "ready_status", "clean_worktree"}, Validators: []string{"ticket_readiness", "branch_reuse_safety", "remote_branch_conflict"}, Post: []string{"create_or_reuse_branch", "set_status_label"}},
		{Name: "review", From: workflowSources{"in-progress"}, To: "in-review", Conditions: []string{"branch_exists"}, Validators: []string{"closing_reference", "pr_base"}, Post: []string{"create_or_update_pr", "set_status_label"}},
		{Name: "draft", From: workflowSources{"in-review"}, To: "in-progress", Post: []string{"set_status_label"}},
		{Name: "finish", From: workflowSources{"in-review"}, To: "done", Conditions: []string{"linked_pr_selected"}, Validators: []string{"finish_readiness"}, Post: []string{"merge", "close_issue", "set_status_label", "finish_receipt", "local_sync", "jira_done"}},
		{Name: "block", From: workflowSources{"backlog", "ready", "in-progress", "in-review"}, To: "blocked"},
		{Name: "unblock", From: workflowSources{"blocked"}, To: "backlog"},
		{Name: "unblock-ready", From: workflowSources{"blocked"}, To: "ready"},
		{Name: "unblock-progress", From: workflowSources{"blocked"}, To: "in-progress"},
		{Name: "unblock-review", From: workflowSources{"blocked"}, To: "in-review"},
		{Name: "supersede", From: workflowSources{"any"}, To: "done", Validators: []string{"replacement_ticket_created"}, Post: []string{"close_issue", "decision_note"}},
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
	registry := defaultTransitionRegistry()
	for _, t := range w.Transitions {
		for _, group := range []struct {
			kind  string
			names []string
			steps map[string]TransitionStep
		}{
			{"condition", t.Conditions, registry.Conditions}, {"validator", t.Validators, registry.Validators}, {"post", t.Post, registry.Post},
		} {
			for _, name := range group.names {
				if _, ok := group.steps[name]; !ok {
					add("workflow_unknown_step", fmt.Sprintf("transition %q has unknown %s %q", t.Name, group.kind, name))
				}
			}
		}
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
	var toDo []string
	for _, s := range w.Statuses {
		if s.Category == statusCategoryToDo && s.Name != "" {
			toDo = append(toDo, s.Name)
		}
	}
	if !workflowReachable(w, toDo, names)[w.FinishTarget] {
		add("workflow_finish_unreachable", fmt.Sprintf("finish target %q is unreachable from to_do statuses", w.FinishTarget))
	}
	for _, t := range w.Transitions {
		if t.Name == "review" && names[t.To] && !workflowReachable(w, []string{t.To}, names)[w.FinishTarget] {
			add("workflow_finish_unreachable_from_review", fmt.Sprintf("finish target %q is unreachable from review status %q", w.FinishTarget, t.To))
		}
	}
	return w, findings
}

// loadDeclaredWorkflow reads only the current checkout's matching config.
// Findings must be checked before using the returned workflow.
func loadDeclaredWorkflow(repo RepoRef) (Workflow, bool, []DoctorCheck) {
	dir, err := os.Getwd()
	if err != nil {
		return Workflow{}, false, nil
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
				return Workflow{}, false, []DoctorCheck{{ID: "workflow_config_parse", Status: DoctorCheckFail, Detail: err.Error(), Remediation: "fix workflow in .gira/config.yaml", FindingClass: DoctorFindingEnvironment}}
			}
			if cfg.Repo != "" && repo.FullName() != "/" {
				configured, e := ParseRepoRef(cfg.Repo)
				if e != nil || !sameRepoRef(configured, repo) {
					return Workflow{}, false, nil
				}
			}
			workflow, findings := resolveWorkflow(cfg.Workflow)
			return workflow, cfg.Workflow != nil, findings
		}
		if !os.IsNotExist(readErr) {
			return Workflow{}, false, []DoctorCheck{{ID: "workflow_config_read", Status: DoctorCheckFail, Detail: readErr.Error(), Remediation: "make .gira/config.yaml readable", FindingClass: DoctorFindingEnvironment}}
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
	return defaultWorkflow(), false, nil
}

func localWorkflowFindings(repo RepoRef) []DoctorCheck {
	_, _, findings := loadDeclaredWorkflow(repo)
	return findings
}

// declaredStatusMove describes a lifecycle status change without running steps.
// Callers must check findings even when declared is false, since malformed or
// unreadable configuration may prevent determining whether a workflow exists.
type declaredStatusMove struct {
	from, to, label string
	category        statusCategory
	noOp            bool
	blocker, detail string
}

func resolveDeclaredStatusMove(w Workflow, command string, labels []string) declaredStatusMove {
	fallback := map[string]string{
		"start": "in-progress", "review": "in-review", "draft": "in-progress",
		"finish": "done", "supersede": "done",
	}[command]
	move := declaredStatusMove{to: fallback}
	for _, transition := range w.Transitions {
		if transition.Name == command {
			move.to = transition.To
			break
		}
	}
	if command == "finish" && w.FinishTarget != "" {
		move.to = w.FinishTarget
	}
	move.from = currentWorkflowStatus(w, labels)
	return completeStatusMove(w, move)
}

// currentWorkflowStatus maps status labels onto declared statuses; several
// labels resolve by the default priority, otherwise the first declared match.
func currentWorkflowStatus(w Workflow, labels []string) string {
	from := ""
	var mapped []string
	for _, label := range labels {
		for _, status := range w.Statuses {
			if strings.EqualFold(strings.TrimSpace(label), status.Label) {
				if from == "" {
					from = status.Name
				}
				mapped = append(mapped, "status:"+status.Name)
				break
			}
		}
	}
	derived := deriveStatus(mapped)
	if derived.managed != "" {
		for _, status := range w.Statuses {
			if def, ok := statusByLabel("status:" + status.Name); ok && def.name == derived.managed {
				from = status.Name
				break
			}
		}
	}
	return from
}

// completeStatusMove fills the target label and checks the declared path.
func completeStatusMove(w Workflow, move declaredStatusMove) declaredStatusMove {
	for _, status := range w.Statuses {
		if status.Name == move.to {
			move.label, move.category = status.Label, status.Category
			break
		}
	}
	if move.label == "" {
		move.blocker = "missing_transition"
		move.detail = fmt.Sprintf("from %s to %s", move.from, move.to)
		return move
	}
	if move.from == move.to {
		move.noOp = true
	} else if findWorkflowTransition(w, move.from, move.to) == nil {
		move.blocker = "missing_transition"
		move.detail = fmt.Sprintf("from %s to %s", move.from, move.to)
	}
	return move
}

// workflowGate is the declared workflow a lifecycle command must follow.
// A zero gate (declared=false) means callers keep their default behavior.
type workflowGate struct {
	workflow Workflow
	declared bool
}

// loadWorkflowGate returns an invalid_workflow error when a declared workflow
// has validation findings, so commands never guess at a broken declaration.
func loadWorkflowGate(repo RepoRef) (workflowGate, error) {
	w, declared, findings := loadDeclaredWorkflow(repo)
	if !declared {
		return workflowGate{}, nil
	}
	if len(findings) > 0 {
		details := make([]string, 0, len(findings))
		for _, finding := range findings {
			details = append(details, finding.ID+": "+finding.Detail)
		}
		return workflowGate{}, fmt.Errorf("invalid_workflow: %s", strings.Join(details, "; "))
	}
	return workflowGate{workflow: w, declared: true}, nil
}

// move resolves a command's status move; a missing path is a named error.
func (g workflowGate) move(command string, labels []string) (declaredStatusMove, error) {
	move := resolveDeclaredStatusMove(g.workflow, command, labels)
	if move.blocker != "" {
		return move, fmt.Errorf("%s: %s %s", move.blocker, command, move.detail)
	}
	return move, nil
}

// setStatus writes the declared label for a resolved move, removing every
// other declared or default status label.
func (g workflowGate) setStatus(repo RepoRef, issueNumber int, labels []string, move declaredStatusMove, runner CommandRunner) error {
	if g.hasOnlyStatusLabel(labels, move.label) {
		return nil
	}
	return replaceIssueStatusLabel(repo, issueNumber, g.statusLabels(labels), move.label, runner)
}

// statusLabels returns every default or declared status label on an issue.
func (g workflowGate) statusLabels(labels []string) []string {
	existing := managedStatusLabels(labels)
	for _, label := range labels {
		for _, status := range g.workflow.Statuses {
			if strings.EqualFold(strings.TrimSpace(label), status.Label) && !containsFold(existing, label) {
				existing = append(existing, label)
			}
		}
	}
	return existing
}

func (g workflowGate) hasOnlyStatusLabel(labels []string, label string) bool {
	existing := g.statusLabels(labels)
	return len(existing) == 1 && strings.EqualFold(existing[0], label)
}

// openFinishTarget returns the declared finish target when it is not in the
// Done category, so finish must leave the issue open there.
func (g workflowGate) openFinishTarget() (WorkflowStatus, bool) {
	if !g.declared {
		return WorkflowStatus{}, false
	}
	for _, status := range g.workflow.Statuses {
		if status.Name == g.workflow.FinishTarget && status.Category != statusCategoryDone {
			return status, true
		}
	}
	return WorkflowStatus{}, false
}

// workflowReachable returns the statuses reachable from starts, inclusive.
func workflowReachable(w Workflow, starts []string, names map[string]bool) map[string]bool {
	reachable := map[string]bool{}
	for _, name := range starts {
		reachable[name] = true
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
	return reachable
}
