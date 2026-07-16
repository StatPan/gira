package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPMOperatingPolicyIsCanonicalAndReferenced(t *testing.T) {
	root := filepath.Join("..", "..")
	policyPath := filepath.Join(root, "docs", "pm-operating-policy.md")
	policy := readPMPolicyDoc(t, policyPath)

	for _, want := range []string{
		"<!-- gira:pm-policy canonical=v1 -->",
		"## Normative Invariants",
		"## Human And AI PM Roles",
		"## Causal Resolution Before Human Escalation",
		"## Current Command And Schema Coverage",
		"Decomposition before escalation",
		"MCP is a transport",
	} {
		if !strings.Contains(policy, want) {
			t.Fatalf("canonical PM policy missing %q", want)
		}
	}

	references := []string{
		filepath.Join(root, "AGENTS.md"),
		filepath.Join(root, "README.md"),
		filepath.Join(root, "docs", "pm-skill.md"),
		filepath.Join(root, "docs", "goal-operating-model.md"),
		filepath.Join(root, "docs", "skills", "gira-agent-operator.md"),
		filepath.Join(root, "docs-site", "pm-skill.md"),
	}
	for _, path := range references {
		text := readPMPolicyDoc(t, path)
		if !strings.Contains(text, "pm-operating-policy.md") {
			t.Errorf("%s does not reference the canonical PM policy", path)
		}
		if strings.Contains(text, "<!-- gira:pm-policy canonical=v1 -->") {
			t.Errorf("%s duplicates the canonical PM policy marker", path)
		}
	}
}

func TestPMOperatingPolicyCoverageMapNamesCurrentContracts(t *testing.T) {
	policy := readPMPolicyDoc(t, filepath.Join("..", "..", "docs", "pm-operating-policy.md"))
	for _, want := range []string{
		"`pm-ir/v1`",
		"`pm-compile-report/v1`",
		"`gira-pm-task-packet/v1`",
		"`decision-policy/v1`",
		"`goal-plan-compact/v1`",
		"`workspace-queues/v1`",
		"`gira-pm-qa/v1`",
		"tool access does not activate or prove PM protocol conformance",
		"does not infer an actor, problem, or outcome from free prose",
	} {
		if !strings.Contains(policy, want) {
			t.Errorf("PM command/schema coverage map missing %q", want)
		}
	}
}

func TestGoalOperatingModelUsesDecompositionFirstEscalation(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "goal-operating-model.md")
	text := readPMPolicyDoc(t, path)
	if strings.Contains(text, "An agent must stop and ask a human when") {
		t.Fatalf("%s retains the superseded undifferentiated human-stop rule", path)
	}
	for _, want := range []string{
		"decomposition-first causal resolution policy",
		"Independent safe work may",
		"Only the residual authority or product-direction decision",
		"do not prove causal decomposition has already happened",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("%s missing reconciled Goal rule %q", path, want)
		}
	}
}

func readPMPolicyDoc(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
