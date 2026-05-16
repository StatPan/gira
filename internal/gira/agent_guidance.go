package gira

import (
	"fmt"
	"strings"
)

const (
	AgentsManagedBlockStart = "<!-- gira:start -->"
	AgentsManagedBlockEnd   = "<!-- gira:end -->"
	AgentSkillBlockStart    = "<!-- gira:agent-skill:start -->"
	AgentSkillBlockEnd      = "<!-- gira:agent-skill:end -->"
)

type AgentGuidanceSpec struct {
	CanonicalSource string
	OperatingModel  []string
	Rules           []string
	RawGHAllowed    []string
	DoNot           []string
}

func CoreAgentGuidanceSpec() AgentGuidanceSpec {
	return AgentGuidanceSpec{
		CanonicalSource: "docs/skills/gira-agent-operator.md",
		OperatingModel: []string{
			"GitHub Issues are executable work packets.",
			"Branches are work-start evidence.",
			"PRs are change units.",
			"Merged PR plus closed issue is completion evidence.",
		},
		Rules: []string{
			"Use --dry-run before --apply for mutating Gira operations.",
			"Prefer Gira commands over raw gh when a Gira command exists.",
			"PR bodies must contain Closes #N, Fixes #N, or Resolves #N.",
			"Keep changes bounded to the ticket.",
			"Route project-only items to repository issues before implementation.",
			"Do not start work missing status:ready until triaged or adopted.",
			"Reuse an existing branch or PR only when it clearly belongs to the ticket.",
			"Fix failed checks before finish unless explicitly instructed.",
			"Ask for clarification when acceptance criteria or repo/ticket context is ambiguous.",
		},
		RawGHAllowed: []string{
			"gh auth status.",
			"Extra read-only issue, PR, or workflow diagnostics not exposed by Gira.",
			"Operations where Gira has no lifecycle command for the needed action.",
		},
		DoNot: []string{
			"Do not bypass Gira start, PR, checks/wait, or finish when those commands apply.",
			"Do not merge without Gira finish unless explicitly instructed.",
			"Do not change unrelated files or revert user changes.",
		},
	}
}

func RenderAgentGuide(spec AgentGuidanceSpec, commands []CommandSpec) string {
	var b strings.Builder
	b.WriteString("Gira agent operator skill\n\n")
	fmt.Fprintf(&b, "Canonical source:\n  %s\n\n", spec.CanonicalSource)
	writeIndentedList(&b, "Operating model:", spec.OperatingModel)
	b.WriteString("\nRegistry-backed lifecycle commands:\n")
	b.WriteString(RenderGuideCommandSection("agent", commands))
	b.WriteString("\n\n")
	writeIndentedList(&b, "Rules:", spec.Rules)
	b.WriteString("\n")
	writeIndentedList(&b, "Raw gh is allowed:", spec.RawGHAllowed)
	b.WriteString("\n")
	writeIndentedList(&b, "Do not:", spec.DoNot)
	return b.String()
}

func RenderAgentsManagedBlock(spec AgentGuidanceSpec, commands []CommandSpec) string {
	var b strings.Builder
	agentCommands := filterCommandSpecsForGuide("agent", commands)
	sortGuideSpecs(agentCommands)
	b.WriteString(AgentsManagedBlockStart)
	b.WriteString("\nGira workflow:\n")
	b.WriteString("- Canonical rules: run `gira guide agent` from the installed Gira CLI.\n")
	for _, command := range agentCommands {
		fmt.Fprintf(&b, "- `%s`: %s\n", command.Usage, command.Summary)
	}
	for _, rule := range spec.Rules[:minInt(4, len(spec.Rules))] {
		fmt.Fprintf(&b, "- %s\n", rule)
	}
	b.WriteString(AgentsManagedBlockEnd)
	b.WriteString("\n")
	return b.String()
}

func RenderAgentSkillManagedBlock(commands []CommandSpec) string {
	var b strings.Builder
	agentCommands := filterCommandSpecsForGuide("agent", commands)
	sortGuideSpecs(agentCommands)
	b.WriteString(AgentSkillBlockStart)
	b.WriteString("\n## Registry-Backed Lifecycle Command Guidance\n\n")
	b.WriteString("This generated section contains command facts for the agent lifecycle. Update `internal/gira/command_registry.go` first, then refresh this block.\n\n")
	for _, command := range agentCommands {
		fmt.Fprintf(&b, "- `%s`: %s\n", command.Usage, command.Summary)
	}
	b.WriteString("\n")
	b.WriteString(AgentSkillBlockEnd)
	b.WriteString("\n")
	return b.String()
}

func RenderAgentOperatorDocsSiteMarkdown(spec AgentGuidanceSpec, commands []CommandSpec) string {
	var b strings.Builder
	agentCommands := filterCommandSpecsForGuide("agent", commands)
	sortGuideSpecs(agentCommands)
	b.WriteString("# Agent Operator Skill\n\n")
	fmt.Fprintf(&b, "The canonical Gira agent/operator skill lives in\n[`%s`](https://github.com/StatPan/gira/blob/main/%s).\n\n", spec.CanonicalSource, spec.CanonicalSource)
	b.WriteString("Use it as the source of truth for coding agents operating Gira-managed\nrepositories. Adapter files such as `AGENTS.md`, `CLAUDE.md`,\n`.github/copilot-instructions.md`, Cursor rules, and `gira guide agent` should\nsummarize that skill instead of redefining it.\n\n")
	writeMarkdownList(&b, "## Operating Model", spec.OperatingModel)
	b.WriteString("## Registry-Backed Lifecycle Commands\n\n")
	for _, command := range agentCommands {
		fmt.Fprintf(&b, "- `%s`: %s\n", command.Usage, command.Summary)
	}
	b.WriteString("\n")
	writeMarkdownList(&b, "## Rules", spec.Rules)
	writeMarkdownList(&b, "## Raw `gh`", spec.RawGHAllowed)
	b.WriteString("## Drift Prevention\n\n")
	b.WriteString("Keep the canonical skill as the source of truth. Keep adapter files short,\nrefresh generated managed blocks from the shared renderer, and update\nCLI/docs tests whenever lifecycle wording changes.\n")
	return b.String()
}

func writeIndentedList(b *strings.Builder, title string, values []string) {
	b.WriteString(title)
	b.WriteString("\n")
	for _, value := range values {
		fmt.Fprintf(b, "  %s\n", value)
	}
}

func writeMarkdownList(b *strings.Builder, title string, values []string) {
	b.WriteString(title)
	b.WriteString("\n\n")
	for _, value := range values {
		fmt.Fprintf(b, "- %s\n", value)
	}
	b.WriteString("\n")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
