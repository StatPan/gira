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
	Repo         string                 `yaml:"repo" toml:"repo" json:"repo"`
	BranchPolicy *BranchPolicyConfig    `yaml:"branch_policy" toml:"branch_policy" json:"branch_policy,omitempty"`
	Workspace    WorkspaceConfig        `yaml:"workspace" toml:"workspace" json:"workspace"`
	Portfolio    PortfolioConfig        `yaml:"portfolio" toml:"portfolio" json:"portfolio"`
	Profiles     map[string]InitProfile `yaml:"profiles" toml:"profiles" json:"profiles"`
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
	if err := validateBranchPolicyConfig(path, "branch_policy", cfg.BranchPolicy); err != nil {
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

func DefaultInitConfigPath(basePath string) string {
	if strings.TrimSpace(basePath) == "" {
		basePath = "."
	}
	return filepath.Join(basePath, ".gira", "config.yaml")
}
