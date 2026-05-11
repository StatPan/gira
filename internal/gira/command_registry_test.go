package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoreCommandSpecsCoverHighValueCommands(t *testing.T) {
	required := [][]string{
		{"setup", "global"},
		{"workspace", "repos", "sync"},
		{"workspace", "status"},
		{"jira", "init"},
		{"jira", "mirror"},
		{"jira", "transition"},
		{"ticket", "new"},
		{"ticket", "start"},
		{"ticket", "pr"},
		{"ticket", "checks"},
		{"ticket", "wait"},
		{"ticket", "finish"},
		{"ticket", "status"},
	}
	for _, path := range required {
		spec, ok := FindCommandSpec(path...)
		if !ok {
			t.Fatalf("missing command metadata for %q", strings.Join(path, " "))
		}
		if strings.TrimSpace(spec.Summary) == "" || strings.TrimSpace(spec.Usage) == "" || len(spec.Examples) == 0 || len(spec.Docs) == 0 {
			t.Fatalf("incomplete metadata for %q: %+v", strings.Join(path, " "), spec)
		}
	}
}

func TestHighValueCommandDocsSurfaces(t *testing.T) {
	for _, path := range [][]string{{"ticket", "new"}, {"setup", "global"}, {"workspace", "repos", "sync"}} {
		spec, ok := FindCommandSpec(path...)
		if !ok {
			t.Fatalf("missing command metadata for %q", strings.Join(path, " "))
		}
		if len(spec.GuideTopics) == 0 {
			t.Fatalf("%q must declare guide coverage", strings.Join(path, " "))
		}
		if !containsString(spec.Docs, "README.md") && !containsDocsSite(spec.Docs) {
			t.Fatalf("%q must be visible in README or docs-site: %+v", strings.Join(path, " "), spec.Docs)
		}
	}
}

func TestCommandReferenceDocsSiteIsGeneratedFromRegistry(t *testing.T) {
	want := RenderCommandReferenceMarkdown(CoreCommandSpecs())
	path := filepath.Join("..", "..", "docs-site", "command-reference.md")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read command reference: %v", err)
	}
	if string(got) != want {
		t.Fatalf("%s is out of sync with command registry; regenerate it from RenderCommandReferenceMarkdown(CoreCommandSpecs())", path)
	}
}

func TestRenderGuideCommandSectionUsesRegistryExamples(t *testing.T) {
	out := RenderGuideCommandSection("ticket", []CommandSpec{
		{
			Path:        []string{"ticket", "new"},
			Summary:     "Create a ticket.",
			Usage:       "gira ticket new \"Title\" --dry-run",
			GuideTopics: []string{"ticket"},
			Examples:    []CommandExample{{Summary: "Preview", Command: "gira ticket new \"Title\" --dry-run"}},
		},
		{
			Path:        []string{"setup", "global"},
			Summary:     "Set up global config.",
			Usage:       "gira setup global --dry-run",
			GuideTopics: []string{"quickstart"},
			Examples:    []CommandExample{{Summary: "Preview", Command: "gira setup global --dry-run"}},
		},
	})
	if !strings.Contains(out, "Create a ticket.") || !strings.Contains(out, "Example: gira ticket new") {
		t.Fatalf("guide section missing ticket metadata:\n%s", out)
	}
	if strings.Contains(out, "setup global") {
		t.Fatalf("guide section included wrong topic:\n%s", out)
	}
}

func containsDocsSite(values []string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, "docs-site/") {
			return true
		}
	}
	return false
}
