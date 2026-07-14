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
			MaxBytes:  13920,
			MaxTokens: 3522,
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
	report := goalPlanBudgetReport(t)
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("encode goal plan fixture: %v", err)
	}
	return string(encoded) + "\n"
}

func TestGoalPlanCompactBudget(t *testing.T) {
	report := goalPlanBudgetReport(t)
	legacy := goalPlanV1BudgetFixture(t)
	dry, err := json.Marshal(BuildGoalPlanCompactReport(report, "dry_run", ""))
	if err != nil {
		t.Fatal(err)
	}
	applyReport := report
	applyReport.CreatedChildren = []GoalPlanChild{{Number: 101, Title: "Created"}}
	applyReport.Actions = []GoalPlanAction{{Action: "child_ticket:create", Status: "applied", Issue: 101}}
	receipt, err := json.Marshal(BuildGoalPlanCompactReport(applyReport, "apply", BuildGoalPlanCompactReport(applyReport, "dry_run", "").PlanID))
	if err != nil {
		t.Fatal(err)
	}
	encoder, err := tokenizer.Get(tokenizer.O200kBase)
	if err != nil {
		t.Fatal(err)
	}
	legacyTokens, _ := encoder.Count(legacy)
	dryTokens, _ := encoder.Count(string(dry))
	receiptTokens, _ := encoder.Count(string(receipt))
	t.Logf("goal-plan compact bytes dry=%d/%d receipt=%d/%d; tokens dry=%d/%d receipt=%d/%d", len(dry), len(legacy), len(receipt), len(legacy), dryTokens, legacyTokens, receiptTokens, legacyTokens)
	if len(dry)*2 > len(legacy) || dryTokens*2 > legacyTokens {
		t.Fatalf("compact dry-run exceeds 50%% budget: bytes=%d/%d tokens=%d/%d", len(dry), len(legacy), dryTokens, legacyTokens)
	}
	if len(receipt)*4 > len(legacy) || receiptTokens*4 > legacyTokens {
		t.Fatalf("compact apply receipt exceeds 25%% budget: bytes=%d/%d tokens=%d/%d", len(receipt), len(legacy), receiptTokens, legacyTokens)
	}
}

func goalPlanBudgetReport(t *testing.T) GoalPlanReport {
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
	return report
}
