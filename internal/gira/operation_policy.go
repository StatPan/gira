package gira

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	OperationModeObservation = "observation"
	OperationModeManaged     = "managed"

	DeliveryPolicyNone     = "none"
	DeliveryPolicyAdvisory = "advisory"
	DeliveryPolicyRequired = "required"

	OperationPolicySourceUnconfigured = "unconfigured"

	OperationPolicyFallbackUnenrolledRepository = "unenrolled_repository"
	OperationPolicyFallbackConfiguredRepository = "configured_repository"
)

// OperationPolicyConfig is the repository policy fragment shared by a local
// contract, a global repository registry entry, and a workspace registry entry.
// An explicit managed mode must declare whether its delivery requirements are
// advisory or required so command consumers never infer enforcement from an
// installation alone.
type OperationPolicyConfig struct {
	OperationMode  string
	DeliveryPolicy string
}

// ResolvedOperationPolicy is safe to embed in command results. Source names
// identify the selected configuration scope; CompatibilityFallback is set only
// when an older configuration does not yet declare an explicit policy.
type ResolvedOperationPolicy struct {
	OperationMode         string `json:"operation_mode"`
	DeliveryPolicy        string `json:"delivery_policy"`
	Source                string `json:"source"`
	CompatibilityFallback string `json:"compatibility_fallback,omitempty"`
}

func (policy ResolvedOperationPolicy) IsObservation() bool {
	return policy.OperationMode == OperationModeObservation
}

func (policy ResolvedOperationPolicy) IsManaged() bool {
	return policy.OperationMode == OperationModeManaged
}

func (policy ResolvedOperationPolicy) RequiresManagedDelivery() bool {
	return policy.IsManaged() && policy.DeliveryPolicy == DeliveryPolicyRequired
}

// ResolveOperationPolicy resolves an explicit policy or supplies the bounded
// compatibility behavior used during migration. Existing Gira configuration
// remains managed/required; repositories with no Gira configuration are
// observation-only until they opt in.
func ResolveOperationPolicy(config OperationPolicyConfig, source string, configured bool) (ResolvedOperationPolicy, error) {
	mode := strings.ToLower(strings.TrimSpace(config.OperationMode))
	delivery := strings.ToLower(strings.TrimSpace(config.DeliveryPolicy))
	source = strings.TrimSpace(source)
	if mode == "" && delivery == "" {
		if configured {
			return ResolvedOperationPolicy{
				OperationMode:         OperationModeManaged,
				DeliveryPolicy:        DeliveryPolicyRequired,
				Source:                resolvedOperationPolicySource(source),
				CompatibilityFallback: OperationPolicyFallbackConfiguredRepository,
			}, nil
		}
		return ResolvedOperationPolicy{
			OperationMode:         OperationModeObservation,
			DeliveryPolicy:        DeliveryPolicyNone,
			Source:                OperationPolicySourceUnconfigured,
			CompatibilityFallback: OperationPolicyFallbackUnenrolledRepository,
		}, nil
	}
	if mode == "" {
		mode = OperationModeManaged
	}
	if mode == OperationModeObservation {
		if delivery == "" || delivery == DeliveryPolicyNone {
			return ResolvedOperationPolicy{OperationMode: OperationModeObservation, DeliveryPolicy: DeliveryPolicyNone, Source: resolvedOperationPolicySource(source)}, nil
		}
		return ResolvedOperationPolicy{}, fmt.Errorf("operation_mode %q cannot use delivery_policy %q", OperationModeObservation, delivery)
	}
	if mode != OperationModeManaged {
		return ResolvedOperationPolicy{}, fmt.Errorf("operation_mode must be %q or %q", OperationModeObservation, OperationModeManaged)
	}
	if delivery != DeliveryPolicyAdvisory && delivery != DeliveryPolicyRequired {
		return ResolvedOperationPolicy{}, fmt.Errorf("managed operation_mode requires delivery_policy %q or %q", DeliveryPolicyAdvisory, DeliveryPolicyRequired)
	}
	return ResolvedOperationPolicy{OperationMode: OperationModeManaged, DeliveryPolicy: delivery, Source: resolvedOperationPolicySource(source)}, nil
}

func resolvedOperationPolicySource(source string) string {
	if strings.TrimSpace(source) == "" {
		return OperationPolicySourceUnconfigured
	}
	return source
}

func validateOperationPolicyConfig(source string, modeField string, deliveryField string, config OperationPolicyConfig) error {
	_, err := ResolveOperationPolicy(config, source, false)
	if err == nil {
		return nil
	}
	return fmt.Errorf("invalid operation policy %q: %s/%s: %w", source, modeField, deliveryField, err)
}

// ResolveRepoOperationPolicy selects policy using the same precedence as the
// branch-policy resolver: repo-local contract, global repo registry, then the
// associated global workspace registry. It is intentionally not called by
// existing commands until they opt into this policy contract.
func ResolveRepoOperationPolicy(repo RepoRef, runner CommandRunner) (ResolvedOperationPolicy, error) {
	candidate, err := loadOperationPolicyCandidate(repo, runner)
	if err != nil {
		return ResolvedOperationPolicy{}, err
	}
	return ResolveOperationPolicy(candidate.Config, candidate.Source, candidate.Configured)
}

type operationPolicyCandidate struct {
	Config     OperationPolicyConfig
	Source     string
	Configured bool
}

func loadOperationPolicyCandidate(repo RepoRef, runner CommandRunner) (operationPolicyCandidate, error) {
	if candidate, found, err := loadLocalOperationPolicyCandidate(repo, runner); err != nil || found {
		return candidate, err
	}
	return loadRegisteredOperationPolicyCandidate(repo)
}

func loadLocalOperationPolicyCandidate(repo RepoRef, runner CommandRunner) (operationPolicyCandidate, bool, error) {
	paths := []string{DefaultInitConfigPath("."), ".gira/config.toml"}
	if runner != nil {
		if output, err := runner.Run("git", "rev-parse", "--show-toplevel"); err == nil {
			if root := strings.TrimSpace(string(output)); root != "" && root != "." {
				paths = append(paths, filepath.Join(root, ".gira", "config.yaml"), filepath.Join(root, ".gira", "config.toml"))
			}
		}
	}
	seen := map[string]struct{}{}
	for _, path := range paths {
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		if _, err := os.Stat(clean); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return operationPolicyCandidate{}, false, fmt.Errorf("inspect repo operation policy %q: %w", clean, err)
		}
		cfg, err := LoadInitConfig(clean)
		if err != nil {
			return operationPolicyCandidate{}, false, err
		}
		if configuredRepo := strings.TrimSpace(cfg.Repo); configuredRepo != "" {
			parsed, parseErr := ParseRepoRef(configuredRepo)
			if parseErr != nil || !sameRepoRef(parsed, repo) {
				continue
			}
		}
		return operationPolicyCandidate{Config: OperationPolicyConfig{OperationMode: cfg.OperationMode, DeliveryPolicy: cfg.DeliveryPolicy}, Source: "repo_local_contract", Configured: true}, true, nil
	}
	return operationPolicyCandidate{}, false, nil
}

func loadRegisteredOperationPolicyCandidate(repo RepoRef) (operationPolicyCandidate, error) {
	root, err := globalConfigRoot("")
	if err != nil {
		return operationPolicyCandidate{}, fmt.Errorf("resolve global operation policy registry: %w", err)
	}
	entry, err := LoadGlobalRepoRegistryEntry(root, repo)
	if err == nil {
		if hasExplicitOperationPolicy(entry.OperationMode, entry.DeliveryPolicy) {
			return operationPolicyCandidate{Config: OperationPolicyConfig{OperationMode: entry.OperationMode, DeliveryPolicy: entry.DeliveryPolicy}, Source: "global_repo_registry", Configured: true}, nil
		}
		if workspace := strings.TrimSpace(entry.Workspace.Name); workspace != "" {
			if candidate, found, err := loadWorkspaceOperationPolicyCandidate(root, repo, workspace, true); err != nil || found {
				return candidate, err
			}
		}
		return operationPolicyCandidate{Source: "global_repo_registry", Configured: true}, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return operationPolicyCandidate{}, fmt.Errorf("load global repo operation policy: %w", err)
	}
	global, err := LoadGlobalConfig(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return operationPolicyCandidate{}, nil
		}
		return operationPolicyCandidate{}, fmt.Errorf("load global operation policy config: %w", err)
	}
	workspace := strings.TrimSpace(global.DefaultWorkspace)
	if workspace == "" {
		return operationPolicyCandidate{}, nil
	}
	candidate, found, err := loadWorkspaceOperationPolicyCandidate(root, repo, workspace, false)
	if err != nil || found {
		return candidate, err
	}
	return operationPolicyCandidate{}, nil
}

func loadWorkspaceOperationPolicyCandidate(root string, repo RepoRef, workspace string, required bool) (operationPolicyCandidate, bool, error) {
	entry, err := LoadGlobalWorkspaceRegistryEntry(root, workspace)
	if err != nil {
		if !required && errors.Is(err, os.ErrNotExist) {
			return operationPolicyCandidate{}, false, nil
		}
		return operationPolicyCandidate{}, false, fmt.Errorf("load global workspace operation policy %q: %w", workspace, err)
	}
	if !workspaceConfigContainsRepo(entry.Workspace, repo) {
		if required {
			return operationPolicyCandidate{}, false, fmt.Errorf("global repo registry workspace %q does not contain %s", workspace, repo.FullName())
		}
		return operationPolicyCandidate{}, false, nil
	}
	return operationPolicyCandidate{Config: OperationPolicyConfig{OperationMode: entry.OperationMode, DeliveryPolicy: entry.DeliveryPolicy}, Source: "global_workspace_registry", Configured: true}, true, nil
}

func hasExplicitOperationPolicy(mode string, delivery string) bool {
	return strings.TrimSpace(mode) != "" || strings.TrimSpace(delivery) != ""
}

func renderOperationPolicyConfig(config OperationPolicyConfig) string {
	if !hasExplicitOperationPolicy(config.OperationMode, config.DeliveryPolicy) {
		return ""
	}
	var b strings.Builder
	if mode := strings.TrimSpace(config.OperationMode); mode != "" {
		fmt.Fprintf(&b, "operation_mode: %s\n", yamlQuotedString(mode))
	}
	if delivery := strings.TrimSpace(config.DeliveryPolicy); delivery != "" {
		fmt.Fprintf(&b, "delivery_policy: %s\n", yamlQuotedString(delivery))
	}
	return b.String()
}
