package gira

import (
	"strings"
	"testing"
)

func TestResolveBranchPolicyDefaultUsesGitHubDefaultBranch(t *testing.T) {
	policy, err := ResolveBranchPolicy(nil, "trunk")
	if err != nil {
		t.Fatalf("ResolveBranchPolicy returned error: %v", err)
	}
	if policy.Mode != BranchPolicyModeGitHubFlow || policy.DefaultBase != "trunk" || policy.Targets["default"] != "trunk" {
		t.Fatalf("unexpected default policy: %+v", policy)
	}
	if !policy.PreserveStartBase || !policy.ForbidImplicitCurrentBranchBase || policy.FinishSyncLocal {
		t.Fatalf("unexpected default safety policy: %+v", policy)
	}
	if policy.PRBaseSource != BranchPolicyPRBaseRecordedTicketBase || policy.Source != "default" {
		t.Fatalf("unexpected default policy source/base source: %+v", policy)
	}
}

func TestResolveBranchPolicyGitFlowPreset(t *testing.T) {
	policy, err := ResolveBranchPolicy(&BranchPolicyConfig{Mode: BranchPolicyModeGitFlow}, "main")
	if err != nil {
		t.Fatalf("ResolveBranchPolicy returned error: %v", err)
	}
	if policy.DefaultBase != "develop" || policy.DevelopmentBase != "develop" || policy.ProductionBase != "main" {
		t.Fatalf("unexpected git-flow bases: %+v", policy)
	}
	if policy.DefaultTarget != "dev" || policy.Targets["dev"] != "develop" || policy.ReleaseBranchPattern != "release/*" || policy.HotfixBranchPattern != "hotfix/*" {
		t.Fatalf("unexpected git-flow targets/patterns: %+v", policy)
	}
}

func TestResolveBranchPolicyBuiltInPresets(t *testing.T) {
	tests := []struct {
		mode           string
		defaultBase    string
		development    string
		production     string
		defaultTarget  string
		featurePattern string
		releasePattern string
		hotfixPattern  string
	}{
		{
			mode:           BranchPolicyModeGitHubFlow,
			defaultBase:    "trunk",
			development:    "trunk",
			production:     "trunk",
			defaultTarget:  "default",
			featurePattern: "issue/{number}-{slug}",
		},
		{
			mode:           BranchPolicyModeTrunk,
			defaultBase:    "trunk",
			development:    "trunk",
			production:     "trunk",
			defaultTarget:  "dev",
			featurePattern: "issue/{number}-{slug}",
		},
		{
			mode:           BranchPolicyModeGitFlow,
			defaultBase:    "develop",
			development:    "develop",
			production:     "main",
			defaultTarget:  "dev",
			featurePattern: "feature/{number}-{slug}",
			releasePattern: "release/*",
			hotfixPattern:  "hotfix/*",
		},
		{
			mode:           BranchPolicyModeReleaseTrain,
			defaultBase:    "trunk",
			development:    "trunk",
			production:     "trunk",
			defaultTarget:  "dev",
			featurePattern: "issue/{number}-{slug}",
			releasePattern: "release/*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			policy, err := ResolveBranchPolicy(&BranchPolicyConfig{Mode: tt.mode}, "trunk")
			if err != nil {
				t.Fatalf("ResolveBranchPolicy returned error: %v", err)
			}
			if policy.DefaultBase != tt.defaultBase || policy.DevelopmentBase != tt.development || policy.ProductionBase != tt.production {
				t.Fatalf("unexpected bases: %+v", policy)
			}
			if policy.DefaultTarget != tt.defaultTarget || policy.FeatureBranchPattern != tt.featurePattern {
				t.Fatalf("unexpected target/pattern: %+v", policy)
			}
			if policy.ReleaseBranchPattern != tt.releasePattern || policy.HotfixBranchPattern != tt.hotfixPattern {
				t.Fatalf("unexpected release/hotfix pattern: %+v", policy)
			}
			if policy.Targets[policy.DefaultTarget] == "" {
				t.Fatalf("default target does not resolve: %+v", policy)
			}
		})
	}
}

func TestResolveBranchPolicyCustomOverrides(t *testing.T) {
	no := false
	yes := true
	policy, err := ResolveBranchPolicy(&BranchPolicyConfig{
		Mode:                            BranchPolicyModeCustom,
		DefaultBase:                     "integration",
		DevelopmentBase:                 "dev",
		ProductionBase:                  "stable",
		DefaultTarget:                   "release",
		FeatureBranchPattern:            "work/{number}-{slug}",
		ReleaseBranchPattern:            "rel/*",
		HotfixBranchPattern:             "fix/*",
		PreserveStartBase:               &yes,
		ForbidImplicitCurrentBranchBase: &yes,
		PRBaseSource:                    BranchPolicyPRBaseRecordedTicketBase,
		FinishSyncLocal:                 &no,
		Targets:                         map[string]string{"release": "rel/2026.05"},
	}, "main")
	if err != nil {
		t.Fatalf("ResolveBranchPolicy returned error: %v", err)
	}
	if policy.DefaultBase != "integration" || policy.Targets["release"] != "rel/2026.05" || policy.Targets["dev"] != "dev" || policy.Targets["production"] != "stable" {
		t.Fatalf("unexpected custom policy: %+v", policy)
	}
	if policy.FeatureBranchPattern != "work/{number}-{slug}" || policy.ReleaseBranchPattern != "rel/*" || policy.HotfixBranchPattern != "fix/*" {
		t.Fatalf("custom patterns not preserved: %+v", policy)
	}
	if !policy.PreserveStartBase || !policy.ForbidImplicitCurrentBranchBase || policy.FinishSyncLocal {
		t.Fatalf("custom safety fields not preserved: %+v", policy)
	}
}

func TestResolveBranchPolicyRejectsUnknownMode(t *testing.T) {
	_, err := ResolveBranchPolicy(&BranchPolicyConfig{Mode: "svn-flow"}, "main")
	if err == nil || !strings.Contains(err.Error(), "unknown branch_policy mode") {
		t.Fatalf("error = %v, want unknown mode diagnostic", err)
	}
}

func TestResolveBranchPolicyRejectsUnknownPRBaseSource(t *testing.T) {
	_, err := ResolveBranchPolicy(&BranchPolicyConfig{Mode: BranchPolicyModeGitHubFlow, PRBaseSource: "github_default"}, "main")
	if err == nil || !strings.Contains(err.Error(), "pr_base_source") {
		t.Fatalf("error = %v, want pr_base_source diagnostic", err)
	}
}

func TestResolveBranchPolicyStartModeDefaultsAndValidates(t *testing.T) {
	policy, err := ResolveBranchPolicy(nil, "main")
	if err != nil || policy.StartMode != BranchStartModeLegacyCreate {
		t.Fatalf("default start mode = %+v err=%v", policy, err)
	}
	policy, err = ResolveBranchPolicy(&BranchPolicyConfig{StartMode: BranchStartModeExplicit}, "main")
	if err != nil || policy.StartMode != BranchStartModeExplicit {
		t.Fatalf("explicit start mode = %+v err=%v", policy, err)
	}
	if _, err := ResolveBranchPolicy(&BranchPolicyConfig{StartMode: "guess"}, "main"); err == nil || !strings.Contains(err.Error(), "start_mode") {
		t.Fatalf("invalid start mode error = %v", err)
	}
}
