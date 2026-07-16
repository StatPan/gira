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
		{"queue", "list"},
		{"queue", "next"},
		{"queue", "handoff"},
		{"queue", "take"},
		{"dispatch", "goal"},
		{"pm", "compile"},
		{"pm", "record"},
		{"pm", "context"},
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

func TestCommandCapabilitiesCoverAdapterClasses(t *testing.T) {
	report := BuildCommandCapabilityReport(CoreCommandSpecs())
	if report.SchemaVersion != CommandCapabilitySchemaVersion {
		t.Fatalf("schema version = %q, want %q", report.SchemaVersion, CommandCapabilitySchemaVersion)
	}
	classes := map[AdapterCapabilityClass]bool{}
	byCanonical := map[string]CommandCapabilityEntry{}
	for _, command := range report.Commands {
		if command.Capability == "" {
			t.Fatalf("%s missing capability class", command.Canonical)
		}
		if command.JSONSupport == "" {
			t.Fatalf("%s missing JSON support metadata", command.Canonical)
		}
		if len(command.Docs) == 0 {
			t.Fatalf("%s missing docs references", command.Canonical)
		}
		classes[command.Capability] = true
		byCanonical[command.Canonical] = command
	}
	for _, class := range []AdapterCapabilityClass{AdapterCapabilityRead, AdapterCapabilityDryRunMutation, AdapterCapabilityApplyMutation, AdapterCapabilityUnsupported} {
		if !classes[class] {
			t.Fatalf("capability report missing class %q", class)
		}
	}
	assertCapability := func(canonical string, want AdapterCapabilityClass) {
		t.Helper()
		got, ok := byCanonical[canonical]
		if !ok {
			t.Fatalf("missing capability entry for %s", canonical)
		}
		if got.Capability != want {
			t.Fatalf("%s capability = %q, want %q", canonical, got.Capability, want)
		}
	}
	assertCapability("gira ticket status", AdapterCapabilityRead)
	assertCapability("gira goal report", AdapterCapabilityRead)
	assertCapability("gira queue list", AdapterCapabilityRead)
	assertCapability("gira queue next", AdapterCapabilityRead)
	assertCapability("gira queue handoff", AdapterCapabilityRead)
	assertCapability("gira queue take", AdapterCapabilityApplyMutation)
	assertCapability("gira dispatch goal", AdapterCapabilityRead)
	assertCapability("gira completion", AdapterCapabilityRead)
	assertCapability("gira goal plan", AdapterCapabilityApplyMutation)
	assertCapability("gira ticket start", AdapterCapabilityApplyMutation)
	assertCapability("gira stats workspace", AdapterCapabilityUnsupported)
	if byCanonical["gira completion"].JSONSupport != JSONSupportNone {
		t.Fatalf("completion JSON support = %q, want %q", byCanonical["gira completion"].JSONSupport, JSONSupportNone)
	}
	if !containsString(byCanonical["gira goal report"].Aliases, "gira goal dossier") {
		t.Fatalf("goal report capability must expose goal dossier alias: %+v", byCanonical["gira goal report"])
	}
	if !containsString(byCanonical["gira ticket view"].Aliases, "gira ticket show") {
		t.Fatalf("ticket view capability must expose ticket show alias: %+v", byCanonical["gira ticket view"])
	}
}

func TestCommandCapabilitiesDocsSiteIsGeneratedFromRegistry(t *testing.T) {
	want := RenderCommandCapabilitiesMarkdown(CoreCommandSpecs())
	path := filepath.Join("..", "..", "docs-site", "command-capabilities.md")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read command capabilities: %v", err)
	}
	if string(got) != want {
		t.Fatalf("%s is out of sync with command registry; regenerate it from RenderCommandCapabilitiesMarkdown(CoreCommandSpecs())", path)
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
