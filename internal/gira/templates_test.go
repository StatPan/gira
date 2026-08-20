package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type githubIssueForm struct {
	Name        string             `yaml:"name"`
	Description string             `yaml:"description"`
	Title       string             `yaml:"title"`
	Labels      []string           `yaml:"labels"`
	Body        []githubIssueField `yaml:"body"`
}

type githubIssueField struct {
	Type        string                    `yaml:"type"`
	ID          string                    `yaml:"id"`
	Attributes  githubIssueFieldAttribute `yaml:"attributes"`
	Validations githubIssueValidations    `yaml:"validations"`
}

type githubIssueFieldAttribute struct {
	Label   string   `yaml:"label"`
	Options []string `yaml:"options"`
}

type githubIssueValidations struct {
	Required bool `yaml:"required"`
}

func TestRenderTemplateTreeIncludesPortfolioIssueTemplate(t *testing.T) {
	rendered, err := RenderTemplateTree("default", mustRepoRefForPortfolio("StatPan/example"), "2026-04-26")
	if err != nil {
		t.Fatalf("RenderTemplateTree error: %v", err)
	}

	var content string
	for _, item := range rendered {
		if item.Path == ".github/ISSUE_TEMPLATE/portfolio.yml" {
			content = item.Content
			break
		}
	}
	if content == "" {
		t.Fatalf("portfolio issue template was not rendered")
	}
	for _, want := range []string{
		"name: Portfolio Ticket",
		`labels: ["type:epic"]`,
		"Routing",
		"Target Repos",
		"Acceptance Criteria",
		"Child Issues",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("portfolio template missing %q:\n%s", want, content)
		}
	}
}

func TestRenderTemplateTreeIncludesGiraWorkflowIssueTemplates(t *testing.T) {
	rendered, err := RenderTemplateTree("default", mustRepoRefForPortfolio("StatPan/example"), "2026-05-13")
	if err != nil {
		t.Fatalf("RenderTemplateTree error: %v", err)
	}

	byPath := map[string]string{}
	for _, item := range rendered {
		byPath[item.Path] = item.Content
	}

	task := byPath[".github/ISSUE_TEMPLATE/task.yml"]
	if task == "" {
		t.Fatalf("task issue template was not rendered")
	}
	for _, want := range []string{
		"Goal",
		"Scope",
		"Acceptance Criteria",
		"Source Ticket",
		"Rollout / Migration Notes",
		"Observability / Security",
		"Verification",
	} {
		if !strings.Contains(task, want) {
			t.Fatalf("task template missing %q:\n%s", want, task)
		}
	}
	if strings.Contains(task, "Gira ticket ID") {
		t.Fatalf("task template should not ask for a pre-created Gira ticket ID:\n%s", task)
	}

	config := byPath[".github/ISSUE_TEMPLATE/config.yml"]
	if config == "" || !strings.Contains(config, "blank_issues_enabled") {
		t.Fatalf("issue template config missing expected GitHub form config:\n%s", config)
	}
}

func TestDefaultGitHubIssueFormsHaveStructuredWorkflowContract(t *testing.T) {
	rendered, err := RenderTemplateTree("default", mustRepoRefForPortfolio("StatPan/example"), "2026-05-13")
	if err != nil {
		t.Fatalf("RenderTemplateTree error: %v", err)
	}

	forms := map[string]githubIssueForm{}
	for _, item := range rendered {
		if !strings.HasPrefix(item.Path, ".github/ISSUE_TEMPLATE/") || !strings.HasSuffix(item.Path, ".yml") || strings.HasSuffix(item.Path, "/config.yml") {
			continue
		}
		var form githubIssueForm
		if err := yaml.Unmarshal([]byte(item.Content), &form); err != nil {
			t.Fatalf("parse %s: %v\n%s", item.Path, err, item.Content)
		}
		if strings.TrimSpace(form.Name) == "" || strings.TrimSpace(form.Description) == "" || strings.TrimSpace(form.Title) == "" {
			t.Fatalf("%s missing name, description, or title: %+v", item.Path, form)
		}
		if len(form.Labels) == 0 || len(form.Body) == 0 {
			t.Fatalf("%s missing labels or body: %+v", item.Path, form)
		}
		forms[item.Path] = form
	}

	requiredFields := map[string][]string{
		".github/ISSUE_TEMPLATE/bug.yml":       {"impact", "actual", "expected", "reproduction", "priority"},
		".github/ISSUE_TEMPLATE/epic.yml":      {"goal", "scope", "acceptance", "priority"},
		".github/ISSUE_TEMPLATE/portfolio.yml": {"goal", "scope", "routing", "acceptance_criteria", "priority"},
		".github/ISSUE_TEMPLATE/spike.yml":     {"question", "scope", "output"},
		".github/ISSUE_TEMPLATE/story.yml":     {"problem", "scope", "acceptance", "priority"},
		".github/ISSUE_TEMPLATE/task.yml":      {"goal", "scope", "acceptance", "priority"},
	}
	for path, ids := range requiredFields {
		form, ok := forms[path]
		if !ok {
			t.Fatalf("expected rendered issue form %s; got paths=%v", path, mapKeys(forms))
		}
		fields := issueFieldsByID(form)
		for _, id := range ids {
			field, ok := fields[id]
			if !ok {
				t.Fatalf("%s missing field id %q; fields=%v", path, id, mapKeys(fields))
			}
			if !field.Validations.Required {
				t.Fatalf("%s field %q should be required: %+v", path, id, field)
			}
			if strings.TrimSpace(field.Attributes.Label) == "" {
				t.Fatalf("%s field %q missing label: %+v", path, id, field)
			}
		}
		if path != ".github/ISSUE_TEMPLATE/epic.yml" {
			if _, ok := fields["source_ticket"]; !ok {
				t.Fatalf("%s should expose optional source_ticket instead of a pre-created Gira ID", path)
			}
		}
		if _, ok := fields["gira_ticket_id"]; ok {
			t.Fatalf("%s should not ask for a Gira ticket ID before issue creation", path)
		}
	}
}

func TestDefaultGitHubIssueTemplateConfigIsParseable(t *testing.T) {
	rendered, err := RenderTemplateTree("default", mustRepoRefForPortfolio("StatPan/example"), "2026-05-13")
	if err != nil {
		t.Fatalf("RenderTemplateTree error: %v", err)
	}

	var content string
	for _, item := range rendered {
		if item.Path == ".github/ISSUE_TEMPLATE/config.yml" {
			content = item.Content
			break
		}
	}
	if content == "" {
		t.Fatal("issue template config was not rendered")
	}
	var config struct {
		BlankIssuesEnabled bool `yaml:"blank_issues_enabled"`
		ContactLinks       []struct {
			Name  string `yaml:"name"`
			URL   string `yaml:"url"`
			About string `yaml:"about"`
		} `yaml:"contact_links"`
	}
	if err := yaml.Unmarshal([]byte(content), &config); err != nil {
		t.Fatalf("parse issue template config: %v\n%s", err, content)
	}
	if !config.BlankIssuesEnabled || len(config.ContactLinks) != 1 || !strings.Contains(config.ContactLinks[0].URL, "gira.statpan.com") {
		t.Fatalf("unexpected issue template config: %+v", config)
	}
}

func TestRenderTemplateTreeIncludesProductionReadyPRTemplate(t *testing.T) {
	rendered, err := RenderTemplateTree("default", mustRepoRefForPortfolio("StatPan/example"), "2026-05-13")
	if err != nil {
		t.Fatalf("RenderTemplateTree error: %v", err)
	}

	var content string
	for _, item := range rendered {
		if item.Path == ".github/PULL_REQUEST_TEMPLATE.md" {
			content = item.Content
			break
		}
	}
	if content == "" {
		t.Fatalf("PR template was not rendered")
	}
	for _, want := range []string{
		"Closes #<issue-number>",
		"gira ticket start <id> --apply",
		"gira ticket finish <id> --apply",
		"Verification",
		"Schema, data, or migration",
		"Rollout and rollback",
		"Observability",
		"Security, permissions, or secret",
		"Documentation or runbook",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("PR template missing %q:\n%s", want, content)
		}
	}
}

func TestRenderTemplateTreeIncludesWorkspaceConfig(t *testing.T) {
	rendered, err := RenderTemplateTree("default", mustRepoRefForPortfolio("StatPan/example"), "2026-04-26")
	if err != nil {
		t.Fatalf("RenderTemplateTree error: %v", err)
	}

	var content string
	for _, item := range rendered {
		if item.Path == ".gira/config.yaml" {
			content = item.Content
			break
		}
	}
	if content == "" {
		t.Fatalf("workspace config was not rendered")
	}
	for _, want := range []string{
		"repo: StatPan/example",
		"start_mode: auto",
		"workspace:",
		"inbox_repo: StatPan/example",
		"- StatPan/example",
		"project:",
		"owner: StatPan",
		"title: example",
		"profiles:",
		"review_policy:",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("workspace config missing %q:\n%s", want, content)
		}
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write rendered config: %v", err)
	}
	resolved, err := ResolveWorkspaceConfig(path)
	if err != nil {
		t.Fatalf("ResolveWorkspaceConfig rendered template error: %v", err)
	}
	if resolved.Project.Owner != "StatPan" || resolved.Project.Title != "example" {
		t.Fatalf("resolved project = %+v, want template defaults", resolved.Project)
	}
}

func issueFieldsByID(form githubIssueForm) map[string]githubIssueField {
	fields := map[string]githubIssueField{}
	for _, field := range form.Body {
		if strings.TrimSpace(field.ID) == "" {
			continue
		}
		fields[field.ID] = field
	}
	return fields
}

func mapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}
