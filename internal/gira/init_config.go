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
	Profiles map[string]InitProfile `yaml:"profiles" toml:"profiles" json:"profiles"`
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
