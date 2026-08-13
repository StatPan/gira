package gira

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveOperationPolicy(t *testing.T) {
	tests := []struct {
		name       string
		config     OperationPolicyConfig
		source     string
		configured bool
		wantMode   string
		wantPolicy string
		wantSource string
		fallback   string
		wantErr    string
	}{
		{
			name:       "unenrolled repository is observed",
			wantMode:   OperationModeObservation,
			wantPolicy: DeliveryPolicyNone,
			wantSource: OperationPolicySourceUnconfigured,
			fallback:   OperationPolicyFallbackUnenrolledRepository,
		},
		{
			name:       "configured repository keeps compatibility behavior",
			source:     "repo_local_contract",
			configured: true,
			wantMode:   OperationModeManaged,
			wantPolicy: DeliveryPolicyRequired,
			wantSource: "repo_local_contract",
			fallback:   OperationPolicyFallbackConfiguredRepository,
		},
		{
			name:       "explicit observation",
			config:     OperationPolicyConfig{OperationMode: OperationModeObservation},
			source:     "repo_local_contract",
			configured: true,
			wantMode:   OperationModeObservation,
			wantPolicy: DeliveryPolicyNone,
			wantSource: "repo_local_contract",
		},
		{
			name:       "explicit managed advisory",
			config:     OperationPolicyConfig{OperationMode: OperationModeManaged, DeliveryPolicy: DeliveryPolicyAdvisory},
			source:     "global_workspace_registry",
			configured: true,
			wantMode:   OperationModeManaged,
			wantPolicy: DeliveryPolicyAdvisory,
			wantSource: "global_workspace_registry",
		},
		{
			name:       "delivery policy infers managed mode",
			config:     OperationPolicyConfig{DeliveryPolicy: DeliveryPolicyRequired},
			source:     "global_repo_registry",
			configured: true,
			wantMode:   OperationModeManaged,
			wantPolicy: DeliveryPolicyRequired,
			wantSource: "global_repo_registry",
		},
		{
			name:    "managed requires explicit delivery policy",
			config:  OperationPolicyConfig{OperationMode: OperationModeManaged},
			wantErr: "managed operation_mode requires delivery_policy",
		},
		{
			name:    "observation rejects managed delivery policy",
			config:  OperationPolicyConfig{OperationMode: OperationModeObservation, DeliveryPolicy: DeliveryPolicyRequired},
			wantErr: "cannot use delivery_policy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveOperationPolicy(tt.config, tt.source, tt.configured)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ResolveOperationPolicy error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveOperationPolicy returned error: %v", err)
			}
			if got.OperationMode != tt.wantMode || got.DeliveryPolicy != tt.wantPolicy || got.Source != tt.wantSource || got.CompatibilityFallback != tt.fallback {
				t.Fatalf("ResolveOperationPolicy = %+v", got)
			}
		})
	}
}

func TestResolveRepoOperationPolicyPrecedenceAndCompatibility(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	checkout := t.TempDir()
	t.Chdir(checkout)
	repo := ParseRepoRefMust("StatPan/gira")

	writeTestFile(t, filepath.Join(configHome, "gira", "config.yaml"), "default_workspace: team\n")
	writeTestFile(t, filepath.Join(configHome, "gira", "workspaces", "team.yaml"), `workspace:
  name: team
  inbox_repo: StatPan/backlog
  repos:
    - StatPan/gira
operation_mode: managed
delivery_policy: advisory
`)

	policy, err := ResolveRepoOperationPolicy(repo, &workRunner{})
	if err != nil {
		t.Fatalf("resolve workspace policy: %v", err)
	}
	if policy.OperationMode != OperationModeManaged || policy.DeliveryPolicy != DeliveryPolicyAdvisory || policy.Source != "global_workspace_registry" || policy.CompatibilityFallback != "" {
		t.Fatalf("workspace policy = %+v", policy)
	}

	writeTestFile(t, filepath.Join(configHome, "gira", "repos", "StatPan", "gira.yaml"), `repo: StatPan/gira
operation_mode: managed
delivery_policy: required
workspace:
  name: team
`)
	policy, err = ResolveRepoOperationPolicy(repo, &workRunner{})
	if err != nil {
		t.Fatalf("resolve global repo policy: %v", err)
	}
	if policy.OperationMode != OperationModeManaged || policy.DeliveryPolicy != DeliveryPolicyRequired || policy.Source != "global_repo_registry" || policy.CompatibilityFallback != "" {
		t.Fatalf("global repo policy = %+v", policy)
	}

	writeTestFile(t, filepath.Join(checkout, ".gira", "config.yaml"), `repo: StatPan/gira
operation_mode: observation
profiles:
  default:
    labels: []
`)
	policy, err = ResolveRepoOperationPolicy(repo, &workRunner{})
	if err != nil {
		t.Fatalf("resolve local policy: %v", err)
	}
	if policy.OperationMode != OperationModeObservation || policy.DeliveryPolicy != DeliveryPolicyNone || policy.Source != "repo_local_contract" || policy.CompatibilityFallback != "" {
		t.Fatalf("local policy = %+v", policy)
	}

	writeTestFile(t, filepath.Join(checkout, ".gira", "config.yaml"), `repo: StatPan/gira
profiles:
  default:
    labels: []
`)
	policy, err = ResolveRepoOperationPolicy(repo, &workRunner{})
	if err != nil {
		t.Fatalf("resolve compatibility policy: %v", err)
	}
	if policy.OperationMode != OperationModeManaged || policy.DeliveryPolicy != DeliveryPolicyRequired || policy.Source != "repo_local_contract" || policy.CompatibilityFallback != OperationPolicyFallbackConfiguredRepository {
		t.Fatalf("compatibility policy = %+v", policy)
	}

	policy, err = ResolveRepoOperationPolicy(ParseRepoRefMust("StatPan/other"), &workRunner{})
	if err != nil {
		t.Fatalf("resolve unenrolled policy: %v", err)
	}
	if policy.OperationMode != OperationModeObservation || policy.DeliveryPolicy != DeliveryPolicyNone || policy.Source != OperationPolicySourceUnconfigured || policy.CompatibilityFallback != OperationPolicyFallbackUnenrolledRepository {
		t.Fatalf("unenrolled policy = %+v", policy)
	}
}

func TestOperationPolicyConfigParsingAndValidation(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantMode   string
		wantPolicy string
		wantErr    string
	}{
		{
			name: "existing config remains readable",
			content: `repo: StatPan/gira
profiles:
  default:
    labels: []
`,
		},
		{
			name: "explicit advisory policy parses",
			content: `repo: StatPan/gira
operation_mode: managed
delivery_policy: advisory
profiles:
  default:
    labels: []
`,
			wantMode:   OperationModeManaged,
			wantPolicy: DeliveryPolicyAdvisory,
		},
		{
			name: "managed policy must be complete",
			content: `operation_mode: managed
profiles:
  default:
    labels: []
`,
			wantErr: "managed operation_mode requires delivery_policy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			writeTestFile(t, path, tt.content)
			cfg, err := LoadInitConfig(path)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("LoadInitConfig error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadInitConfig returned error: %v", err)
			}
			if cfg.OperationMode != tt.wantMode || cfg.DeliveryPolicy != tt.wantPolicy {
				t.Fatalf("parsed policy = operation_mode %q delivery_policy %q", cfg.OperationMode, cfg.DeliveryPolicy)
			}
		})
	}
}

func TestGlobalOperationPolicyConfigParsingAndValidation(t *testing.T) {
	root := t.TempDir()
	repo := ParseRepoRefMust("StatPan/gira")
	writeTestFile(t, filepath.Join(root, "repos", "StatPan", "gira.yaml"), `repo: StatPan/gira
operation_mode: managed
delivery_policy: advisory
`)
	entry, err := LoadGlobalRepoRegistryEntry(root, repo)
	if err != nil {
		t.Fatalf("LoadGlobalRepoRegistryEntry returned error: %v", err)
	}
	if entry.OperationMode != OperationModeManaged || entry.DeliveryPolicy != DeliveryPolicyAdvisory {
		t.Fatalf("global repo policy = %+v", entry)
	}

	writeTestFile(t, filepath.Join(root, "workspaces", "team.yaml"), `workspace:
  name: team
  inbox_repo: StatPan/backlog
  repos:
    - StatPan/gira
operation_mode: observation
`)
	workspace, err := LoadGlobalWorkspaceRegistryEntry(root, "team")
	if err != nil {
		t.Fatalf("LoadGlobalWorkspaceRegistryEntry returned error: %v", err)
	}
	if workspace.OperationMode != OperationModeObservation || workspace.DeliveryPolicy != "" {
		t.Fatalf("global workspace policy = %+v", workspace)
	}

	writeTestFile(t, filepath.Join(root, "repos", "StatPan", "gira.yaml"), `repo: StatPan/gira
operation_mode: observation
delivery_policy: required
`)
	_, err = LoadGlobalRepoRegistryEntry(root, repo)
	if err == nil || !strings.Contains(err.Error(), "cannot use delivery_policy") {
		t.Fatalf("global repo invalid policy error = %v", err)
	}

	writeTestFile(t, filepath.Join(root, "workspaces", "team.yaml"), `workspace:
  name: team
  inbox_repo: StatPan/backlog
  repos:
    - StatPan/gira
operation_mode: managed
`)
	_, err = LoadGlobalWorkspaceRegistryEntry(root, "team")
	if err == nil || !strings.Contains(err.Error(), "managed operation_mode requires delivery_policy") {
		t.Fatalf("global workspace invalid policy error = %v", err)
	}
}

func TestOperationPolicyRenderersPreserveExplicitPolicy(t *testing.T) {
	policy := OperationPolicyConfig{OperationMode: OperationModeManaged, DeliveryPolicy: DeliveryPolicyAdvisory}
	repoContent := renderSetupGlobalRepoEntry(GlobalRepoRegistryEntry{Repo: "StatPan/gira", OperationMode: policy.OperationMode, DeliveryPolicy: policy.DeliveryPolicy})
	if !strings.Contains(repoContent, `operation_mode: "managed"`) || !strings.Contains(repoContent, `delivery_policy: "advisory"`) {
		t.Fatalf("setup global repo content missing operation policy:\n%s", repoContent)
	}
	workspaceContent := renderGlobalWorkspaceRegistryEntry(GlobalWorkspaceRegistryEntry{
		Workspace:      WorkspaceConfig{Name: "team", InboxRepo: "StatPan/backlog", Repos: []string{"StatPan/gira"}},
		OperationMode:  policy.OperationMode,
		DeliveryPolicy: policy.DeliveryPolicy,
	})
	if !strings.Contains(workspaceContent, `operation_mode: "managed"`) || !strings.Contains(workspaceContent, `delivery_policy: "advisory"`) {
		t.Fatalf("workspace content missing operation policy:\n%s", workspaceContent)
	}
	encoded, err := marshalRepoRegistryEntry(GlobalRepoRegistryEntry{Repo: "StatPan/gira"})
	if err != nil {
		t.Fatalf("marshal repo registry: %v", err)
	}
	if strings.Contains(string(encoded), "operation_mode:") || strings.Contains(string(encoded), "delivery_policy:") {
		t.Fatalf("empty repo policy should be omitted:\n%s", encoded)
	}
}
