package gira

import (
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
