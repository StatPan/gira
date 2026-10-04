package gira

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

type InitConfig struct {
	Repo               string                 `yaml:"repo" toml:"repo" json:"repo"`
	OperationMode      string                 `yaml:"operation_mode" toml:"operation_mode" json:"operation_mode,omitempty"`
	DeliveryPolicy     string                 `yaml:"delivery_policy" toml:"delivery_policy" json:"delivery_policy,omitempty"`
	BranchPolicy       *BranchPolicyConfig    `yaml:"branch_policy" toml:"branch_policy" json:"branch_policy,omitempty"`
	FinishReviewPolicy string                 `yaml:"finish_review_policy" toml:"finish_review_policy" json:"finish_review_policy,omitempty"`
	RecordedReview     *RecordedReviewConfig  `yaml:"recorded_review" toml:"recorded_review" json:"recorded_review,omitempty"`
	Review             ReviewConfig           `yaml:"review" toml:"review" json:"review,omitempty"`
	Workspace          WorkspaceConfig        `yaml:"workspace" toml:"workspace" json:"workspace"`
	Portfolio          PortfolioConfig        `yaml:"portfolio" toml:"portfolio" json:"portfolio"`
	Profiles           map[string]InitProfile `yaml:"profiles" toml:"profiles" json:"profiles"`
}

// RecordedReviewConfig declares which authenticated GitHub accounts may record
// an independent review receipt and the development target that may use it.
// The values are repository policy and must be read from the PR base commit.
type RecordedReviewConfig struct {
	AllowedRecorders    []string `yaml:"allowed_recorders" toml:"allowed_recorders" json:"allowed_recorders"`
	AllowedBaseBranches []string `yaml:"allowed_base_branches" toml:"allowed_base_branches" json:"allowed_base_branches"`
}

// ReviewConfig keeps optional, checkout-local review commands. Commands are
// argv arrays rather than shell strings so Gira can execute only the declared
// program and arguments.
type ReviewConfig struct {
	LocalChecks []LocalReviewCheck `yaml:"local_checks" toml:"local_checks" json:"local_checks,omitempty"`
}

type LocalReviewCheck struct {
	Name    string   `yaml:"name" toml:"name" json:"name"`
	Command []string `yaml:"command" toml:"command" json:"command"`
}

type WorkspaceConfig struct {
	Name      string        `yaml:"name" toml:"name" json:"name"`
	Owner     string        `yaml:"owner" toml:"owner" json:"owner"`
	InboxRepo string        `yaml:"inbox_repo" toml:"inbox_repo" json:"inbox_repo"`
	Repos     []string      `yaml:"repos" toml:"repos" json:"repos"`
	Project   ProjectConfig `yaml:"project" toml:"project" json:"project"`
}

type ProjectConfig struct {
	Owner  string `yaml:"owner" toml:"owner" json:"owner"`
	Number int    `yaml:"number" toml:"number" json:"number"`
	Title  string `yaml:"title" toml:"title" json:"title"`
}

type PortfolioConfig struct {
	Repo  string   `yaml:"repo" toml:"repo" json:"repo"`
	Repos []string `yaml:"repos" toml:"repos" json:"repos"`
}

type InitProfile struct {
	Labels         []string     `yaml:"labels" toml:"labels" json:"labels"`
	Milestones     []string     `yaml:"milestones" toml:"milestones" json:"milestones"`
	IssueTemplates []string     `yaml:"issue_templates" toml:"issue_templates" json:"issue_templates"`
	ReviewPolicy   ReviewPolicy `yaml:"review_policy" toml:"review_policy" json:"review_policy"`
}

type ReviewPolicy struct {
	RequiredApprovals int  `yaml:"required_approvals" toml:"required_approvals" json:"required_approvals"`
	RequireCodeOwners bool `yaml:"require_code_owners" toml:"require_code_owners" json:"require_code_owners"`
}

func LoadInitConfig(path string) (InitConfig, error) {
	if strings.TrimSpace(path) == "" {
		return InitConfig{}, fmt.Errorf("config path is required")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return InitConfig{}, fmt.Errorf("read init config %q: %w", path, err)
	}
	return parseInitConfigContent(path, content)
}

func parseInitConfigContent(path string, content []byte) (InitConfig, error) {
	var cfg InitConfig
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".toml":
		if err := toml.Unmarshal(content, &cfg); err != nil {
			return InitConfig{}, fmt.Errorf("parse init config %q: %w", path, err)
		}
	default:
		if err := yaml.Unmarshal(content, &cfg); err != nil {
			return InitConfig{}, fmt.Errorf("parse init config %q: %w", path, err)
		}
	}
	if len(cfg.Profiles) == 0 {
		return InitConfig{}, fmt.Errorf("invalid init config %q: profiles must include at least one profile", path)
	}
	if strings.TrimSpace(cfg.Repo) != "" {
		if _, err := ParseRepoRef(cfg.Repo); err != nil {
			return InitConfig{}, fmt.Errorf("invalid init config %q: repo must be in OWNER/REPO format", path)
		}
	}
	if err := validateOperationPolicyConfig(path, "operation_mode", "delivery_policy", OperationPolicyConfig{OperationMode: cfg.OperationMode, DeliveryPolicy: cfg.DeliveryPolicy}); err != nil {
		return InitConfig{}, err
	}
	if err := validateBranchPolicyConfig(path, "branch_policy", cfg.BranchPolicy); err != nil {
		return InitConfig{}, err
	}
	if err := validateLocalReviewChecks(path, cfg.Review.LocalChecks); err != nil {
		return InitConfig{}, err
	}
	if err := validateFinishReviewPolicyConfig(path, cfg); err != nil {
		return InitConfig{}, err
	}
	if strings.TrimSpace(cfg.Workspace.InboxRepo) != "" {
		if _, err := ParseRepoRef(cfg.Workspace.InboxRepo); err != nil {
			return InitConfig{}, fmt.Errorf("invalid init config %q: workspace.inbox_repo must be in OWNER/REPO format", path)
		}
	}
	seenWorkspaceRepos := map[string]struct{}{}
	for i, repoValue := range cfg.Workspace.Repos {
		repoValue = strings.TrimSpace(repoValue)
		if repoValue == "" {
			return InitConfig{}, fmt.Errorf("invalid init config %q: workspace.repos[%d] cannot be empty", path, i)
		}
		if _, err := ParseRepoRef(repoValue); err != nil {
			return InitConfig{}, fmt.Errorf("invalid init config %q: workspace.repos[%d] must be in OWNER/REPO format", path, i)
		}
		if _, ok := seenWorkspaceRepos[repoValue]; ok {
			return InitConfig{}, fmt.Errorf("invalid init config %q: workspace.repos contains duplicate repo %q", path, repoValue)
		}
		seenWorkspaceRepos[repoValue] = struct{}{}
	}
	if cfg.Workspace.Project.Number < 0 {
		return InitConfig{}, fmt.Errorf("invalid init config %q: workspace.project.number must be >= 0 when workspace.project is set", path)
	}
	if strings.TrimSpace(cfg.Portfolio.Repo) != "" {
		if _, err := ParseRepoRef(cfg.Portfolio.Repo); err != nil {
			return InitConfig{}, fmt.Errorf("invalid init config %q: portfolio.repo must be in OWNER/REPO format", path)
		}
	}
	seenPortfolioRepos := map[string]struct{}{}
	for i, repoValue := range cfg.Portfolio.Repos {
		repoValue = strings.TrimSpace(repoValue)
		if repoValue == "" {
			return InitConfig{}, fmt.Errorf("invalid init config %q: portfolio.repos[%d] cannot be empty", path, i)
		}
		if _, err := ParseRepoRef(repoValue); err != nil {
			return InitConfig{}, fmt.Errorf("invalid init config %q: portfolio.repos[%d] must be in OWNER/REPO format", path, i)
		}
		if _, ok := seenPortfolioRepos[repoValue]; ok {
			return InitConfig{}, fmt.Errorf("invalid init config %q: portfolio.repos contains duplicate repo %q", path, repoValue)
		}
		seenPortfolioRepos[repoValue] = struct{}{}
	}
	for name, profile := range cfg.Profiles {
		if strings.TrimSpace(name) == "" {
			return InitConfig{}, fmt.Errorf("invalid init config %q: profile name cannot be empty", path)
		}
		if profile.ReviewPolicy.RequiredApprovals < 0 {
			return InitConfig{}, fmt.Errorf("invalid init config %q: profiles.%s.review_policy.required_approvals must be >= 0", path, name)
		}
	}
	return cfg, nil
}

func validateFinishReviewPolicyConfig(path string, cfg InitConfig) error {
	value := strings.ToLower(strings.TrimSpace(cfg.FinishReviewPolicy))
	if value != "" && value != FinishReviewPolicyRequired && value != FinishReviewPolicyNone && value != FinishReviewPolicyRecordedIndependent {
		return fmt.Errorf("invalid init config %q: finish_review_policy must be %q, %q, or %q", path, FinishReviewPolicyRequired, FinishReviewPolicyNone, FinishReviewPolicyRecordedIndependent)
	}
	if value != FinishReviewPolicyRecordedIndependent {
		if cfg.RecordedReview != nil {
			return fmt.Errorf("invalid init config %q: recorded_review requires finish_review_policy: %s", path, FinishReviewPolicyRecordedIndependent)
		}
		return nil
	}
	if cfg.RecordedReview == nil {
		return fmt.Errorf("invalid init config %q: recorded_review is required when finish_review_policy is %s", path, FinishReviewPolicyRecordedIndependent)
	}
	if cfg.BranchPolicy == nil || strings.TrimSpace(cfg.BranchPolicy.DevelopmentBase) == "" || strings.TrimSpace(cfg.BranchPolicy.ProductionBase) == "" {
		return fmt.Errorf("invalid init config %q: recorded review requires explicit branch_policy.development_base and branch_policy.production_base", path)
	}
	developmentBase := strings.TrimSpace(cfg.BranchPolicy.DevelopmentBase)
	productionBase := strings.TrimSpace(cfg.BranchPolicy.ProductionBase)
	if strings.EqualFold(developmentBase, productionBase) {
		return fmt.Errorf("invalid init config %q: recorded review development and production bases must differ", path)
	}
	if len(cfg.RecordedReview.AllowedBaseBranches) != 1 || !strings.EqualFold(strings.TrimSpace(cfg.RecordedReview.AllowedBaseBranches[0]), developmentBase) {
		return fmt.Errorf("invalid init config %q: recorded_review.allowed_base_branches must contain only branch_policy.development_base %q", path, developmentBase)
	}
	seenRecorders := map[string]struct{}{}
	for index, raw := range cfg.RecordedReview.AllowedRecorders {
		recorder := strings.TrimSpace(raw)
		if !validGitHubLogin(recorder) {
			return fmt.Errorf("invalid init config %q: recorded_review.allowed_recorders[%d] must be a GitHub login", path, index)
		}
		key := strings.ToLower(recorder)
		if _, exists := seenRecorders[key]; exists {
			return fmt.Errorf("invalid init config %q: recorded_review.allowed_recorders contains duplicate login %q", path, recorder)
		}
		seenRecorders[key] = struct{}{}
	}
	if len(seenRecorders) == 0 {
		return fmt.Errorf("invalid init config %q: recorded_review.allowed_recorders must include at least one GitHub login", path)
	}
	return nil
}

func validGitHubLogin(login string) bool {
	if login == "" || len(login) > 39 || login[0] == '-' || login[len(login)-1] == '-' {
		return false
	}
	for _, char := range login {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' {
			continue
		}
		return false
	}
	return true
}

func validateLocalReviewChecks(path string, checks []LocalReviewCheck) error {
	for index, check := range checks {
		if strings.TrimSpace(check.Name) == "" {
			return fmt.Errorf("invalid init config %q: review.local_checks[%d].name is required", path, index)
		}
		if len(check.Command) == 0 || strings.TrimSpace(check.Command[0]) == "" {
			return fmt.Errorf("invalid init config %q: review.local_checks[%d].command must include a program", path, index)
		}
		for argIndex, arg := range check.Command {
			if strings.TrimSpace(arg) == "" {
				return fmt.Errorf("invalid init config %q: review.local_checks[%d].command[%d] cannot be empty", path, index, argIndex)
			}
		}
	}
	return nil
}

// LoadLocalReviewConfig accepts a review-only config fragment so a repository
// can declare local checks without also adopting the bootstrap profiles schema.
func LoadLocalReviewConfig(path string) (ReviewConfig, bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return ReviewConfig{}, false, fmt.Errorf("read local review config %q: %w", path, err)
	}
	var partial struct {
		Review ReviewConfig `yaml:"review" toml:"review"`
	}
	if strings.EqualFold(filepath.Ext(path), ".toml") {
		if err := toml.Unmarshal(content, &partial); err != nil {
			return ReviewConfig{}, false, fmt.Errorf("parse local review config %q: %w", path, err)
		}
	} else if err := yaml.Unmarshal(content, &partial); err != nil {
		return ReviewConfig{}, false, fmt.Errorf("parse local review config %q: %w", path, err)
	}
	if len(partial.Review.LocalChecks) == 0 {
		return ReviewConfig{}, false, nil
	}
	if err := validateLocalReviewChecks(path, partial.Review.LocalChecks); err != nil {
		return ReviewConfig{}, false, err
	}
	return partial.Review, true, nil
}

func DefaultInitConfigPath(basePath string) string {
	if strings.TrimSpace(basePath) == "" {
		basePath = "."
	}
	return filepath.Join(basePath, ".gira", "config.yaml")
}
