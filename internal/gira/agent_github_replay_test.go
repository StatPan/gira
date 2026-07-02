package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type agentGitHubReplayFixture struct {
	SchemaVersion string                `yaml:"schema_version"`
	Name          string                `yaml:"name"`
	Repo          string                `yaml:"repo"`
	EvidenceLevel string                `yaml:"evidence_level"`
	Workflow      string                `yaml:"workflow"`
	Outcome       string                `yaml:"outcome"`
	RawGH         agentGitHubReplayPath `yaml:"raw_gh"`
	Gira          agentGitHubReplayPath `yaml:"gira"`
	Expected      struct {
		RawGH        agentGitHubReplayMetrics `yaml:"raw_gh"`
		Gira         agentGitHubReplayMetrics `yaml:"gira"`
		Improvements agentGitHubReplayMetrics `yaml:"improvements"`
	} `yaml:"expected"`
}

type agentGitHubReplayPath struct {
	Name  string                  `yaml:"name"`
	Steps []agentGitHubReplayStep `yaml:"steps"`
}

type agentGitHubReplayStep struct {
	Command         string   `yaml:"command"`
	Actions         []string `yaml:"actions"`
	RequiredArgs    []string `yaml:"required_args"`
	DecisionNodes   []string `yaml:"decision_nodes"`
	CognitiveNodes  []string `yaml:"cognitive_nodes"`
	ProviderCalls   int      `yaml:"provider_calls"`
	DiscoveryReads  int      `yaml:"discovery_reads"`
	FallbackEscapes int      `yaml:"fallback_escapes"`
	DirectLabels    []string `yaml:"direct_labels"`
	PRActions       []string `yaml:"pr_actions"`
	Comments        []string `yaml:"comments"`
	Verifications   []string `yaml:"verifications"`
	DurableEvidence []string `yaml:"durable_evidence"`
}

type agentGitHubReplayMetrics struct {
	CommandNodes        int `yaml:"command_nodes"`
	AgentSteps          int `yaml:"agent_steps"`
	ArgumentNodes       int `yaml:"argument_nodes"`
	DecisionNodes       int `yaml:"decision_nodes"`
	CognitiveNodes      int `yaml:"cognitive_nodes"`
	ProviderCalls       int `yaml:"provider_calls"`
	DiscoveryReads      int `yaml:"discovery_reads"`
	FallbackEscapes     int `yaml:"fallback_escapes"`
	DirectLabelsTouched int `yaml:"direct_labels_touched"`
	PRActions           int `yaml:"pr_actions"`
	Comments            int `yaml:"comments"`
	Verifications       int `yaml:"verifications"`
	WorkflowCostX2      int `yaml:"workflow_cost_x2"`
}

func TestAgentGitHubReplayFixtures(t *testing.T) {
	paths, err := filepath.Glob("testdata/agent_github_replay/*.yaml")
	if err != nil {
		t.Fatalf("glob GitHub replay fixtures: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("GitHub replay fixtures = 0, want at least 1")
	}
	for _, path := range paths {
		path := path
		t.Run(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), func(t *testing.T) {
			fixture := readAgentGitHubReplayFixture(t, path)
			raw := measureAgentGitHubReplayPath(t, "raw_gh", fixture.RawGH)
			gira := measureAgentGitHubReplayPath(t, "gira", fixture.Gira)
			if raw != fixture.Expected.RawGH {
				t.Fatalf("raw_gh metrics = %+v, want %+v", raw, fixture.Expected.RawGH)
			}
			if gira != fixture.Expected.Gira {
				t.Fatalf("gira metrics = %+v, want %+v", gira, fixture.Expected.Gira)
			}
			improvements := subtractAgentGitHubReplayMetrics(raw, gira)
			if improvements != fixture.Expected.Improvements {
				t.Fatalf("improvements = %+v, want %+v", improvements, fixture.Expected.Improvements)
			}
			assertGitHubReplayCoversLifecycle(t, fixture.RawGH)
			assertGitHubReplayCoversLifecycle(t, fixture.Gira)
			if improvements.WorkflowCostX2 <= 0 {
				t.Fatalf("workflow cost did not improve: raw=%d gira=%d", raw.WorkflowCostX2, gira.WorkflowCostX2)
			}
			if improvements.DecisionNodes <= 0 {
				t.Fatalf("decision burden did not improve: raw=%d gira=%d", raw.DecisionNodes, gira.DecisionNodes)
			}
			if improvements.DirectLabelsTouched <= 0 {
				t.Fatalf("direct label handling did not improve: raw=%d gira=%d", raw.DirectLabelsTouched, gira.DirectLabelsTouched)
			}
			if gira.FallbackEscapes != 0 {
				t.Fatalf("gira fallback escapes = %d, want 0", gira.FallbackEscapes)
			}
			t.Logf("result %s: workflow_cost_x2 raw=%d gira=%d delta=%d; decisions raw=%d gira=%d; provider_calls raw=%d gira=%d",
				fixture.Name,
				raw.WorkflowCostX2,
				gira.WorkflowCostX2,
				improvements.WorkflowCostX2,
				raw.DecisionNodes,
				gira.DecisionNodes,
				raw.ProviderCalls,
				gira.ProviderCalls,
			)
		})
	}
}

func readAgentGitHubReplayFixture(t *testing.T, path string) agentGitHubReplayFixture {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replay fixture: %v", err)
	}
	var fixture agentGitHubReplayFixture
	if err := yaml.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode replay fixture: %v", err)
	}
	if fixture.SchemaVersion != "agent-github-replay/v1" {
		t.Fatalf("schema_version = %q", fixture.SchemaVersion)
	}
	if fixture.Repo == "" || fixture.Workflow == "" || fixture.Outcome == "" {
		t.Fatalf("fixture must declare repo, workflow, and outcome")
	}
	if fixture.EvidenceLevel != "replayed_transcript" {
		t.Fatalf("evidence_level = %q, want replayed_transcript", fixture.EvidenceLevel)
	}
	return fixture
}

func measureAgentGitHubReplayPath(t *testing.T, label string, path agentGitHubReplayPath) agentGitHubReplayMetrics {
	t.Helper()
	if path.Name == "" {
		t.Fatalf("%s path has empty name", label)
	}
	if len(path.Steps) == 0 {
		t.Fatalf("%s path has no steps", label)
	}
	metrics := agentGitHubReplayMetrics{
		CommandNodes: len(path.Steps),
		AgentSteps:   len(path.Steps),
	}
	for i, step := range path.Steps {
		if strings.TrimSpace(step.Command) == "" {
			t.Fatalf("%s step %d has empty command", label, i+1)
		}
		if len(step.Actions) == 0 {
			t.Fatalf("%s step %d has no action tags", label, i+1)
		}
		if step.ProviderCalls < 0 || step.DiscoveryReads < 0 || step.FallbackEscapes < 0 {
			t.Fatalf("%s step %d has negative provider metrics", label, i+1)
		}
		metrics.ArgumentNodes += len(step.RequiredArgs)
		metrics.DecisionNodes += len(step.DecisionNodes)
		metrics.CognitiveNodes += len(step.CognitiveNodes)
		metrics.ProviderCalls += step.ProviderCalls
		metrics.DiscoveryReads += step.DiscoveryReads
		metrics.FallbackEscapes += step.FallbackEscapes
		metrics.DirectLabelsTouched += len(step.DirectLabels)
		metrics.PRActions += len(step.PRActions)
		metrics.Comments += len(step.Comments)
		metrics.Verifications += len(step.Verifications)
	}
	metrics.WorkflowCostX2 = workflowCostX2(metrics)
	return metrics
}

func workflowCostX2(metrics agentGitHubReplayMetrics) int {
	return 2*metrics.CommandNodes +
		metrics.ArgumentNodes +
		4*metrics.DecisionNodes +
		4*metrics.ProviderCalls +
		6*metrics.FallbackEscapes +
		3*metrics.CognitiveNodes
}

func subtractAgentGitHubReplayMetrics(raw agentGitHubReplayMetrics, gira agentGitHubReplayMetrics) agentGitHubReplayMetrics {
	return agentGitHubReplayMetrics{
		CommandNodes:        raw.CommandNodes - gira.CommandNodes,
		AgentSteps:          raw.AgentSteps - gira.AgentSteps,
		ArgumentNodes:       raw.ArgumentNodes - gira.ArgumentNodes,
		DecisionNodes:       raw.DecisionNodes - gira.DecisionNodes,
		CognitiveNodes:      raw.CognitiveNodes - gira.CognitiveNodes,
		ProviderCalls:       raw.ProviderCalls - gira.ProviderCalls,
		DiscoveryReads:      raw.DiscoveryReads - gira.DiscoveryReads,
		FallbackEscapes:     raw.FallbackEscapes - gira.FallbackEscapes,
		DirectLabelsTouched: raw.DirectLabelsTouched - gira.DirectLabelsTouched,
		PRActions:           raw.PRActions - gira.PRActions,
		Comments:            raw.Comments - gira.Comments,
		Verifications:       raw.Verifications - gira.Verifications,
		WorkflowCostX2:      raw.WorkflowCostX2 - gira.WorkflowCostX2,
	}
}

func assertGitHubReplayCoversLifecycle(t *testing.T, path agentGitHubReplayPath) {
	t.Helper()
	actions := map[string]bool{}
	for _, step := range path.Steps {
		for _, action := range step.Actions {
			actions[action] = true
		}
	}
	for _, action := range []string{
		"ticket_discovery",
		"label_status_transition",
		"branch_binding",
		"pr_creation",
		"review_packet",
		"comments_receipts",
		"checks_review_state",
		"finish_merge",
	} {
		if !actions[action] {
			t.Fatalf("%s missing lifecycle action %q", path.Name, action)
		}
	}
}
