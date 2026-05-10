package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderAgentGuideUsesCommandRegistry(t *testing.T) {
	spec := CoreAgentGuidanceSpec()
	out := RenderAgentGuide(spec, []CommandSpec{
		{
			Path:        []string{"ticket", "start"},
			Summary:     "Start from registry metadata.",
			Usage:       "gira ticket start 42 --dry-run",
			GuideTopics: []string{"agent"},
			GuideOrder:  20,
		},
		{
			Path:        []string{"ticket", "finish"},
			Summary:     "Finish from registry metadata.",
			Usage:       "gira ticket finish --dry-run",
			GuideTopics: []string{"agent"},
			GuideOrder:  60,
		},
	})
	if !strings.Contains(out, "Registry-backed lifecycle commands:") || !strings.Contains(out, "Start from registry metadata.") {
		t.Fatalf("agent guide did not render registry metadata:\n%s", out)
	}
	if strings.Index(out, "gira ticket start") > strings.Index(out, "gira ticket finish") {
		t.Fatalf("agent guide ignored guide order:\n%s", out)
	}
}

func TestRenderAgentsManagedBlockUsesRegistry(t *testing.T) {
	block := RenderAgentsManagedBlock(CoreAgentGuidanceSpec(), []CommandSpec{
		{
			Path:        []string{"ticket", "pr"},
			Summary:     "Open the linked PR.",
			Usage:       "gira ticket pr --dry-run",
			GuideTopics: []string{"agent"},
		},
	})
	for _, want := range []string{AgentsManagedBlockStart, AgentsManagedBlockEnd, "`gira ticket pr --dry-run`: Open the linked PR.", "Closes #N"} {
		if !strings.Contains(block, want) {
			t.Fatalf("managed block missing %q:\n%s", want, block)
		}
	}
}

func TestUpsertAgentsManagedBlockReplacesOnlyManagedRegion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")
	original := "# Custom\n\nKeep before.\n\n" + AgentsManagedBlockStart + "\nold generated text\n" + AgentsManagedBlockEnd + "\n\nKeep after.\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write AGENTS: %v", err)
	}
	if err := upsertAgentsManagedBlock(path); err != nil {
		t.Fatalf("upsertAgentsManagedBlock error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read AGENTS: %v", err)
	}
	text := string(got)
	for _, want := range []string{"# Custom", "Keep before.", "Keep after.", "`gira ticket start", "`gira ticket finish"} {
		if !strings.Contains(text, want) {
			t.Fatalf("updated AGENTS missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "old generated text") {
		t.Fatalf("old managed text was not replaced:\n%s", text)
	}
}

func TestAgentOperatorDocsSiteIsGeneratedFromRegistry(t *testing.T) {
	want := RenderAgentOperatorDocsSiteMarkdown(CoreAgentGuidanceSpec(), CoreCommandSpecs())
	path := filepath.Join("..", "..", "docs-site", "agent-operator-skill.md")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read agent operator docs-site page: %v", err)
	}
	if string(got) != want {
		t.Fatalf("%s is out of sync with agent guidance registry", path)
	}
}
