package gira

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiktoken-go/tokenizer"
)

const agentContextTokenEncoding = "o200k_base"

type agentContextBudgetArtifact struct {
	Name      string
	Content   string
	MaxBytes  int
	MaxTokens int
}

func TestAgentContextBudgetBaseline(t *testing.T) {
	encoder, err := tokenizer.Get(tokenizer.O200kBase)
	if err != nil {
		t.Fatalf("load %s tokenizer: %v", agentContextTokenEncoding, err)
	}
	artifacts := []agentContextBudgetArtifact{
		{
			Name:      "agents_adapter",
			Content:   readAgentContextBudgetDocument(t, "AGENTS.md"),
			MaxBytes:  1801,
			MaxTokens: 410,
		},
		{
			Name:      "operator_skill",
			Content:   readAgentContextBudgetDocument(t, "docs", "skills", "gira-agent-operator.md"),
			MaxBytes:  13886,
			MaxTokens: 3512,
		},
		{
			Name:      "pm_skill",
			Content:   readAgentContextBudgetDocument(t, "docs", "pm-skill.md"),
			MaxBytes:  3790,
			MaxTokens: 804,
		},
		{
			Name:      "goal_plan_v1_dry_run",
			Content:   goalPlanV1BudgetFixture(t),
			MaxBytes:  4318,
			MaxTokens: 1123,
		},
	}
	for _, artifact := range artifacts {
		artifact := artifact
		t.Run(artifact.Name, func(t *testing.T) {
			bytes := len([]byte(artifact.Content))
			tokens, err := encoder.Count(artifact.Content)
			if err != nil {
				t.Fatalf("count %s tokens: %v", artifact.Name, err)
			}
			t.Logf("artifact=%s bytes=%d %s_tokens=%d", artifact.Name, bytes, agentContextTokenEncoding, tokens)
			if bytes > artifact.MaxBytes {
				t.Fatalf("%s bytes=%d exceeds budget=%d", artifact.Name, bytes, artifact.MaxBytes)
			}
			if tokens > artifact.MaxTokens {
				t.Fatalf("%s tokens=%d exceeds budget=%d", artifact.Name, tokens, artifact.MaxTokens)
			}
		})
	}
}

func readAgentContextBudgetDocument(t *testing.T, path ...string) string {
	t.Helper()
	root := filepath.Join("..", "..")
	content, err := os.ReadFile(filepath.Join(append([]string{root}, path...)...))
	if err != nil {
		t.Fatalf("read budget document %s: %v", filepath.Join(path...), err)
	}
	return string(content)
}

func goalPlanV1BudgetFixture(t *testing.T) string {
	t.Helper()
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := goalPlanRunner(
		goalPlanGoalJSON(100, goalPlanBody("## Goal\nShip goal mode\n\n## Scope\nCLI goal planning\n\n## Goal Plan\n- Add API\n- Add CLI\n", ""), []string{"type:epic", "priority:p1", "area:backend", "status:ready"}),
		`[]`,
		`{"comments":[]}`,
		nil,
	)
	report, err := BuildGoalPlanReport(GoalPlanInput{Repo: repo, Goal: 100, DryRun: true}, runner)
	if err != nil {
		t.Fatalf("build goal plan fixture: %v", err)
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("encode goal plan fixture: %v", err)
	}
	return string(encoded) + "\n"
}
