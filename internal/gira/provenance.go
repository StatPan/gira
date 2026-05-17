package gira

import (
	"sort"
	"strings"
)

const (
	ProvenanceBlockStart = "<!-- gira:provenance:start -->"
	ProvenanceBlockEnd   = "<!-- gira:provenance:end -->"
)

type ProvenanceNote struct {
	Planning       []string `json:"planning,omitempty"`
	Implementation []string `json:"implementation,omitempty"`
	Review         []string `json:"review,omitempty"`
}

type StatusProvenanceCounts struct {
	AgentExecuted int `json:"agent_executed"`
	HumanExecuted int `json:"human_executed"`
	HumanReviewed int `json:"human_reviewed"`
	AIReviewed    int `json:"ai_reviewed"`
	MixedHumanAI  int `json:"mixed_human_ai"`
	Unknown       int `json:"unknown"`
}

func ParseProvenanceNote(text string) ProvenanceNote {
	block := extractProvenanceBlock(text)
	if block == "" {
		return ProvenanceNote{}
	}
	note := ProvenanceNote{}
	for _, line := range strings.Split(block, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		actors := normalizeProvenanceActors(value)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "planning":
			note.Planning = actors
		case "implementation":
			note.Implementation = actors
		case "review":
			note.Review = actors
		}
	}
	return note
}

func extractProvenanceBlock(text string) string {
	start := strings.Index(text, ProvenanceBlockStart)
	if start < 0 {
		return ""
	}
	start += len(ProvenanceBlockStart)
	end := strings.Index(text[start:], ProvenanceBlockEnd)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(text[start : start+end])
}

func normalizeProvenanceActors(value string) []string {
	seen := map[string]struct{}{}
	actors := []string{}
	for _, raw := range strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '|' || r == '/'
	}) {
		actor := strings.ToLower(strings.TrimSpace(raw))
		switch actor {
		case "ai", "agent", "bot", "llm", "model", "codex":
			actor = "ai"
		case "human", "person", "maintainer", "operator":
			actor = "human"
		default:
			continue
		}
		if _, ok := seen[actor]; ok {
			continue
		}
		seen[actor] = struct{}{}
		actors = append(actors, actor)
	}
	sort.Strings(actors)
	return actors
}

func summarizeIssueProvenance(issues []normalizedIssue) StatusProvenanceCounts {
	counts := StatusProvenanceCounts{}
	for _, issue := range issues {
		note := ParseProvenanceNote(issue.Body)
		impl := note.Implementation
		if len(impl) == 0 {
			impl = provenanceActorsFromAgentLabels(issue.Labels)
		}
		hasHuman := provenanceHasActor(note.Planning, "human") || provenanceHasActor(note.Implementation, "human") || provenanceHasActor(note.Review, "human")
		hasAI := provenanceHasActor(note.Planning, "ai") || provenanceHasActor(note.Implementation, "ai") || provenanceHasActor(note.Review, "ai")
		if len(impl) == 0 && !hasHuman && !hasAI {
			counts.Unknown++
			continue
		}
		if provenanceHasActor(impl, "ai") {
			counts.AgentExecuted++
			hasAI = true
		}
		if provenanceHasActor(impl, "human") {
			counts.HumanExecuted++
			hasHuman = true
		}
		if provenanceHasActor(note.Review, "human") {
			counts.HumanReviewed++
			hasHuman = true
		}
		if provenanceHasActor(note.Review, "ai") {
			counts.AIReviewed++
			hasAI = true
		}
		if hasHuman && hasAI {
			counts.MixedHumanAI++
		}
	}
	return counts
}

func provenanceActorsFromAgentLabels(labels []string) []string {
	for _, label := range labels {
		lower := strings.ToLower(strings.TrimSpace(label))
		if !strings.HasPrefix(lower, "agent:") {
			continue
		}
		switch strings.TrimPrefix(lower, "agent:") {
		case "human":
			return []string{"human"}
		case "worker", "codex", "gira", "reviewer":
			return []string{"ai"}
		}
	}
	return nil
}

func provenanceHasActor(actors []string, actor string) bool {
	for _, value := range actors {
		if value == actor {
			return true
		}
	}
	return false
}
