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
	if !strings.Contains(block, "gira guide agent") {
		t.Fatalf("managed block should point adopted repos at CLI guidance:\n%s", block)
	}
	if strings.Contains(block, "docs/skills/gira-agent-operator.md") {
		t.Fatalf("managed block should not point adopted repos at missing repo-local docs:\n%s", block)
	}
}

func TestRenderAgentSkillManagedBlockUsesRegistry(t *testing.T) {
	block := RenderAgentSkillManagedBlock([]CommandSpec{
		{
			Path:        []string{"ticket", "start"},
			Summary:     "Start the ticket.",
			Usage:       "gira ticket start 42 --dry-run",
			GuideTopics: []string{"agent"},
			GuideOrder:  20,
		},
		{
			Path:        []string{"ticket", "finish"},
			Summary:     "Finish the ticket.",
			Usage:       "gira ticket finish --dry-run",
			GuideTopics: []string{"agent"},
			GuideOrder:  60,
		},
	})
	for _, want := range []string{AgentSkillBlockStart, AgentSkillBlockEnd, "Registry-Backed Lifecycle Command Guidance", "`gira ticket start 42 --dry-run`: Start the ticket."} {
		if !strings.Contains(block, want) {
			t.Fatalf("skill managed block missing %q:\n%s", want, block)
		}
	}
	if strings.Index(block, "gira ticket start") > strings.Index(block, "gira ticket finish") {
		t.Fatalf("skill managed block ignored guide order:\n%s", block)
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

func TestAgentSkillManagedBlockIsGeneratedFromRegistry(t *testing.T) {
	want := RenderAgentSkillManagedBlock(CoreCommandSpecs())
	path := filepath.Join("..", "..", "docs", "skills", "gira-agent-operator.md")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read agent skill: %v", err)
	}
	block, ok := extractManagedBlock(string(got), AgentSkillBlockStart, AgentSkillBlockEnd)
	if !ok {
		t.Fatalf("%s is missing managed skill block markers", path)
	}
	if block != want {
		t.Fatalf("%s managed skill block is out of sync with registry", path)
	}
}

func TestAgentSkillManagedBlockIsUnique(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "skills", "gira-agent-operator.md")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read agent skill: %v", err)
	}
	text := string(got)
	if starts := countExactLines(text, AgentSkillBlockStart); starts != 1 {
		t.Fatalf("%s has %d managed skill block starts, want 1", path, starts)
	}
	if ends := countExactLines(text, AgentSkillBlockEnd); ends != 1 {
		t.Fatalf("%s has %d managed skill block ends, want 1", path, ends)
	}
	refreshed, err := ReplaceSingleManagedBlock(text, AgentSkillBlockStart, AgentSkillBlockEnd, RenderAgentSkillManagedBlock(CoreCommandSpecs()))
	if err != nil {
		t.Fatalf("refresh managed block: %v", err)
	}
	if refreshed != text {
		t.Fatalf("%s managed skill block is not idempotent", path)
	}
}

func countExactLines(text string, target string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == target {
			count++
		}
	}
	return count
}

func extractManagedBlock(text string, start string, end string) (string, bool) {
	startAt := strings.Index(text, start)
	endAt := strings.Index(text, end)
	if startAt < 0 || endAt < startAt {
		return "", false
	}
	endAt += len(end)
	if endAt < len(text) && text[endAt] == '\n' {
		endAt++
	}
	return text[startAt:endAt], true
}
