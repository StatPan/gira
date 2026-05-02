package gira

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type InitConfig struct {
	Profiles map[string]InitProfile `yaml:"profiles" json:"profiles"`
}

type InitProfile struct {
	Labels         []string `yaml:"labels" json:"labels"`
	Milestones     []string `yaml:"milestones" json:"milestones"`
	IssueTemplates []string `yaml:"issue_templates" json:"issue_templates"`
	ReviewPolicy   struct {
		RequiredApprovals int  `yaml:"required_approvals" json:"required_approvals"`
		RequireCodeOwners bool `yaml:"require_code_owners" json:"require_code_owners"`
	} `yaml:"review_policy" json:"review_policy"`
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
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return InitConfig{}, fmt.Errorf("parse init config %q: %w", path, err)
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

func DefaultInitConfigPath() string {
	return filepath.Join(".gira", "config.yaml")
}
