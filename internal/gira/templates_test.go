package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
