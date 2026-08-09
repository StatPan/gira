package gira

import (
	"fmt"
	"sort"
	"strings"
)

const (
	BranchPolicyModeGitHubFlow   = "github-flow"
	BranchPolicyModeTrunk        = "trunk"
	BranchPolicyModeGitFlow      = "git-flow"
	BranchPolicyModeReleaseTrain = "release-train"
	BranchPolicyModeCustom       = "custom"

	BranchPolicyPRBaseRecordedTicketBase = "recorded_ticket_base"
	BranchStartModeLegacyCreate          = "legacy-create"
	BranchStartModeExplicit              = "explicit"
)

type BranchPolicyConfig struct {
	Mode                            string            `yaml:"mode,omitempty" toml:"mode,omitempty" json:"mode,omitempty"`
	DefaultBase                     string            `yaml:"default_base,omitempty" toml:"default_base,omitempty" json:"default_base,omitempty"`
	DevelopmentBase                 string            `yaml:"development_base,omitempty" toml:"development_base,omitempty" json:"development_base,omitempty"`
	ProductionBase                  string            `yaml:"production_base,omitempty" toml:"production_base,omitempty" json:"production_base,omitempty"`
	DefaultTarget                   string            `yaml:"default_target,omitempty" toml:"default_target,omitempty" json:"default_target,omitempty"`
	FeatureBranchPattern            string            `yaml:"feature_branch_pattern,omitempty" toml:"feature_branch_pattern,omitempty" json:"feature_branch_pattern,omitempty"`
	StartMode                       string            `yaml:"start_mode,omitempty" toml:"start_mode,omitempty" json:"start_mode,omitempty"`
	ReleaseBranchPattern            string            `yaml:"release_branch_pattern,omitempty" toml:"release_branch_pattern,omitempty" json:"release_branch_pattern,omitempty"`
	HotfixBranchPattern             string            `yaml:"hotfix_branch_pattern,omitempty" toml:"hotfix_branch_pattern,omitempty" json:"hotfix_branch_pattern,omitempty"`
	PreserveStartBase               *bool             `yaml:"preserve_start_base,omitempty" toml:"preserve_start_base,omitempty" json:"preserve_start_base,omitempty"`
	ForbidImplicitCurrentBranchBase *bool             `yaml:"forbid_implicit_current_branch_base,omitempty" toml:"forbid_implicit_current_branch_base,omitempty" json:"forbid_implicit_current_branch_base,omitempty"`
	PRBaseSource                    string            `yaml:"pr_base_source,omitempty" toml:"pr_base_source,omitempty" json:"pr_base_source,omitempty"`
	FinishSyncLocal                 *bool             `yaml:"finish_sync_local,omitempty" toml:"finish_sync_local,omitempty" json:"finish_sync_local,omitempty"`
	Targets                         map[string]string `yaml:"targets,omitempty" toml:"targets,omitempty" json:"targets,omitempty"`
}

type ResolvedBranchPolicy struct {
	Mode                            string            `json:"mode"`
	DefaultBase                     string            `json:"default_base"`
	DevelopmentBase                 string            `json:"development_base,omitempty"`
	ProductionBase                  string            `json:"production_base,omitempty"`
	DefaultTarget                   string            `json:"default_target"`
	FeatureBranchPattern            string            `json:"feature_branch_pattern,omitempty"`
	StartMode                       string            `json:"start_mode"`
	ReleaseBranchPattern            string            `json:"release_branch_pattern,omitempty"`
	HotfixBranchPattern             string            `json:"hotfix_branch_pattern,omitempty"`
	PreserveStartBase               bool              `json:"preserve_start_base"`
	ForbidImplicitCurrentBranchBase bool              `json:"forbid_implicit_current_branch_base"`
	PRBaseSource                    string            `json:"pr_base_source"`
	FinishSyncLocal                 bool              `json:"finish_sync_local"`
	Targets                         map[string]string `json:"targets,omitempty"`
	Source                          string            `json:"source"`
}

func ResolveBranchPolicy(config *BranchPolicyConfig, githubDefaultBranch string) (ResolvedBranchPolicy, error) {
	defaultBranch := strings.TrimSpace(githubDefaultBranch)
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	source := "default"
	raw := BranchPolicyConfig{}
	if config != nil {
		raw = *config
		source = "config"
	}
	mode := strings.TrimSpace(raw.Mode)
	if mode == "" {
		mode = BranchPolicyModeGitHubFlow
	}
	policy, err := branchPolicyPreset(mode, defaultBranch)
	if err != nil {
		return ResolvedBranchPolicy{}, err
	}
	policy.Source = source
	overlayBranchPolicy(&policy, raw)
	if err := validateResolvedBranchPolicy(policy); err != nil {
		return ResolvedBranchPolicy{}, err
	}
	return policy, nil
}

func branchPolicyPreset(mode string, defaultBranch string) (ResolvedBranchPolicy, error) {
	switch strings.TrimSpace(mode) {
	case BranchPolicyModeGitHubFlow:
		return ResolvedBranchPolicy{
			Mode:                            BranchPolicyModeGitHubFlow,
			DefaultBase:                     defaultBranch,
			DevelopmentBase:                 defaultBranch,
			ProductionBase:                  defaultBranch,
			DefaultTarget:                   "default",
			FeatureBranchPattern:            "issue/{number}-{slug}",
			StartMode:                       BranchStartModeLegacyCreate,
			PreserveStartBase:               true,
			ForbidImplicitCurrentBranchBase: true,
			PRBaseSource:                    BranchPolicyPRBaseRecordedTicketBase,
			FinishSyncLocal:                 false,
			Targets:                         map[string]string{"default": defaultBranch, "dev": defaultBranch},
		}, nil
	case BranchPolicyModeTrunk:
		return ResolvedBranchPolicy{
			Mode:                            BranchPolicyModeTrunk,
			DefaultBase:                     defaultBranch,
			DevelopmentBase:                 defaultBranch,
			ProductionBase:                  defaultBranch,
			DefaultTarget:                   "dev",
			FeatureBranchPattern:            "issue/{number}-{slug}",
			StartMode:                       BranchStartModeLegacyCreate,
			PreserveStartBase:               true,
			ForbidImplicitCurrentBranchBase: true,
			PRBaseSource:                    BranchPolicyPRBaseRecordedTicketBase,
			FinishSyncLocal:                 false,
			Targets:                         map[string]string{"default": defaultBranch, "dev": defaultBranch},
		}, nil
	case BranchPolicyModeGitFlow:
		return ResolvedBranchPolicy{
			Mode:                            BranchPolicyModeGitFlow,
			DefaultBase:                     "develop",
			DevelopmentBase:                 "develop",
			ProductionBase:                  "main",
			DefaultTarget:                   "dev",
			FeatureBranchPattern:            "feature/{number}-{slug}",
			StartMode:                       BranchStartModeLegacyCreate,
			ReleaseBranchPattern:            "release/*",
			HotfixBranchPattern:             "hotfix/*",
			PreserveStartBase:               true,
			ForbidImplicitCurrentBranchBase: true,
			PRBaseSource:                    BranchPolicyPRBaseRecordedTicketBase,
			FinishSyncLocal:                 false,
			Targets:                         map[string]string{"default": "develop", "dev": "develop", "production": "main"},
		}, nil
	case BranchPolicyModeReleaseTrain:
		return ResolvedBranchPolicy{
			Mode:                            BranchPolicyModeReleaseTrain,
			DefaultBase:                     defaultBranch,
			DevelopmentBase:                 defaultBranch,
			ProductionBase:                  defaultBranch,
			DefaultTarget:                   "dev",
			FeatureBranchPattern:            "issue/{number}-{slug}",
			StartMode:                       BranchStartModeLegacyCreate,
			ReleaseBranchPattern:            "release/*",
			PreserveStartBase:               true,
			ForbidImplicitCurrentBranchBase: true,
			PRBaseSource:                    BranchPolicyPRBaseRecordedTicketBase,
			FinishSyncLocal:                 false,
			Targets:                         map[string]string{"default": defaultBranch, "dev": defaultBranch, "production": defaultBranch},
		}, nil
	case BranchPolicyModeCustom:
		return ResolvedBranchPolicy{
			Mode:                            BranchPolicyModeCustom,
			DefaultBase:                     defaultBranch,
			DevelopmentBase:                 defaultBranch,
			ProductionBase:                  defaultBranch,
			DefaultTarget:                   "default",
			FeatureBranchPattern:            "issue/{number}-{slug}",
			StartMode:                       BranchStartModeLegacyCreate,
			PreserveStartBase:               true,
			ForbidImplicitCurrentBranchBase: true,
			PRBaseSource:                    BranchPolicyPRBaseRecordedTicketBase,
			FinishSyncLocal:                 false,
			Targets:                         map[string]string{"default": defaultBranch},
		}, nil
	default:
		return ResolvedBranchPolicy{}, fmt.Errorf("unknown branch_policy mode %q; expected github-flow, trunk, git-flow, release-train, or custom", strings.TrimSpace(mode))
	}
}

func overlayBranchPolicy(policy *ResolvedBranchPolicy, raw BranchPolicyConfig) {
	if strings.TrimSpace(raw.DefaultBase) != "" {
		policy.DefaultBase = strings.TrimSpace(raw.DefaultBase)
	}
	if strings.TrimSpace(raw.DevelopmentBase) != "" {
		policy.DevelopmentBase = strings.TrimSpace(raw.DevelopmentBase)
	}
	if strings.TrimSpace(raw.ProductionBase) != "" {
		policy.ProductionBase = strings.TrimSpace(raw.ProductionBase)
	}
	if strings.TrimSpace(raw.DefaultTarget) != "" {
		policy.DefaultTarget = strings.TrimSpace(raw.DefaultTarget)
	}
	if strings.TrimSpace(raw.FeatureBranchPattern) != "" {
		policy.FeatureBranchPattern = strings.TrimSpace(raw.FeatureBranchPattern)
	}
	if strings.TrimSpace(raw.StartMode) != "" {
		policy.StartMode = strings.TrimSpace(raw.StartMode)
	}
	if strings.TrimSpace(raw.ReleaseBranchPattern) != "" {
		policy.ReleaseBranchPattern = strings.TrimSpace(raw.ReleaseBranchPattern)
	}
	if strings.TrimSpace(raw.HotfixBranchPattern) != "" {
		policy.HotfixBranchPattern = strings.TrimSpace(raw.HotfixBranchPattern)
	}
	if raw.PreserveStartBase != nil {
		policy.PreserveStartBase = *raw.PreserveStartBase
	}
	if raw.ForbidImplicitCurrentBranchBase != nil {
		policy.ForbidImplicitCurrentBranchBase = *raw.ForbidImplicitCurrentBranchBase
	}
	if strings.TrimSpace(raw.PRBaseSource) != "" {
		policy.PRBaseSource = strings.TrimSpace(raw.PRBaseSource)
	}
	if raw.FinishSyncLocal != nil {
		policy.FinishSyncLocal = *raw.FinishSyncLocal
	}
	if raw.Targets != nil {
		policy.Targets = map[string]string{}
		for key, value := range raw.Targets {
			policy.Targets[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	ensureBranchPolicyTargets(policy)
}

func ensureBranchPolicyTargets(policy *ResolvedBranchPolicy) {
	if policy.Targets == nil {
		policy.Targets = map[string]string{}
	}
	if strings.TrimSpace(policy.DefaultBase) != "" {
		if strings.TrimSpace(policy.Targets["default"]) == "" {
			policy.Targets["default"] = policy.DefaultBase
		}
		if strings.TrimSpace(policy.DefaultTarget) != "" && strings.TrimSpace(policy.Targets[policy.DefaultTarget]) == "" {
			policy.Targets[policy.DefaultTarget] = policy.DefaultBase
		}
	}
	if strings.TrimSpace(policy.DevelopmentBase) != "" && strings.TrimSpace(policy.Targets["dev"]) == "" {
		policy.Targets["dev"] = policy.DevelopmentBase
	}
	if strings.TrimSpace(policy.ProductionBase) != "" && strings.TrimSpace(policy.Targets["production"]) == "" {
		policy.Targets["production"] = policy.ProductionBase
	}
}

func validateResolvedBranchPolicy(policy ResolvedBranchPolicy) error {
	if strings.TrimSpace(policy.Mode) == "" {
		return fmt.Errorf("branch_policy mode is required")
	}
	if strings.TrimSpace(policy.DefaultBase) == "" {
		return fmt.Errorf("branch_policy default_base is required")
	}
	if strings.TrimSpace(policy.DefaultTarget) == "" {
		return fmt.Errorf("branch_policy default_target is required")
	}
	if strings.TrimSpace(policy.PRBaseSource) != BranchPolicyPRBaseRecordedTicketBase {
		return fmt.Errorf("branch_policy pr_base_source must be %q", BranchPolicyPRBaseRecordedTicketBase)
	}
	if policy.StartMode != BranchStartModeLegacyCreate && policy.StartMode != BranchStartModeExplicit {
		return fmt.Errorf("branch_policy start_mode must be %q or %q", BranchStartModeLegacyCreate, BranchStartModeExplicit)
	}
	for key, value := range policy.Targets {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("branch_policy targets must not contain an empty target name")
		}
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("branch_policy target %q must not be empty", key)
		}
	}
	if strings.TrimSpace(policy.Targets[policy.DefaultTarget]) == "" {
		return fmt.Errorf("branch_policy default_target %q must resolve to a base branch", policy.DefaultTarget)
	}
	return nil
}

func validateBranchPolicyConfig(source string, field string, config *BranchPolicyConfig) error {
	if config == nil {
		return nil
	}
	if _, err := ResolveBranchPolicy(config, "main"); err != nil {
		return fmt.Errorf("invalid %s %q: %s: %w", field, source, field, err)
	}
	return nil
}

func renderBranchPolicyConfig(config *BranchPolicyConfig) string {
	if config == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("branch_policy:\n")
	writeBranchPolicyString(&b, "mode", config.Mode)
	writeBranchPolicyString(&b, "default_base", config.DefaultBase)
	writeBranchPolicyString(&b, "development_base", config.DevelopmentBase)
	writeBranchPolicyString(&b, "production_base", config.ProductionBase)
	writeBranchPolicyString(&b, "default_target", config.DefaultTarget)
	writeBranchPolicyString(&b, "feature_branch_pattern", config.FeatureBranchPattern)
	writeBranchPolicyString(&b, "start_mode", config.StartMode)
	writeBranchPolicyString(&b, "release_branch_pattern", config.ReleaseBranchPattern)
	writeBranchPolicyString(&b, "hotfix_branch_pattern", config.HotfixBranchPattern)
	writeBranchPolicyBool(&b, "preserve_start_base", config.PreserveStartBase)
	writeBranchPolicyBool(&b, "forbid_implicit_current_branch_base", config.ForbidImplicitCurrentBranchBase)
	writeBranchPolicyString(&b, "pr_base_source", config.PRBaseSource)
	writeBranchPolicyBool(&b, "finish_sync_local", config.FinishSyncLocal)
	if len(config.Targets) > 0 {
		b.WriteString("  targets:\n")
		keys := make([]string, 0, len(config.Targets))
		for key := range config.Targets {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&b, "    %s: %s\n", yamlQuotedString(key), yamlQuotedString(config.Targets[key]))
		}
	}
	return b.String()
}

func writeBranchPolicyString(b *strings.Builder, key string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	fmt.Fprintf(b, "  %s: %s\n", key, yamlQuotedString(strings.TrimSpace(value)))
}

func writeBranchPolicyBool(b *strings.Builder, key string, value *bool) {
	if value == nil {
		return
	}
	fmt.Fprintf(b, "  %s: %t\n", key, *value)
}
