package gira

import "testing"

func TestParseProvenanceNoteNormalizesActorClasses(t *testing.T) {
	note := ParseProvenanceNote(`before
<!-- gira:provenance:start -->
planning: human
implementation: codex, human
review: maintainer / ai
<!-- gira:provenance:end -->
after`)

	if !sameStringSet(note.Planning, []string{"human"}) {
		t.Fatalf("planning = %v", note.Planning)
	}
	if !sameStringSet(note.Implementation, []string{"ai", "human"}) {
		t.Fatalf("implementation = %v", note.Implementation)
	}
	if !sameStringSet(note.Review, []string{"ai", "human"}) {
		t.Fatalf("review = %v", note.Review)
	}
}

func TestSummarizeIssueProvenanceHandlesLabelsAndMixedNotes(t *testing.T) {
	counts := summarizeIssueProvenance([]normalizedIssue{
		{Number: 1, Labels: []string{"agent:codex"}},
		{Number: 2, Labels: []string{"lane:agent"}, Body: `<!-- gira:provenance:start -->
planning: human
implementation: ai
review: human
<!-- gira:provenance:end -->`},
		{Number: 3, Labels: []string{"agent:human"}},
		{Number: 4},
	})

	if counts.AgentExecuted != 2 || counts.HumanExecuted != 1 || counts.HumanReviewed != 1 || counts.MixedHumanAI != 1 || counts.Unknown != 1 {
		t.Fatalf("unexpected provenance counts: %+v", counts)
	}
}

func sameStringSet(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
