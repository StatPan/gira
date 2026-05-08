package gira

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type WorkspaceProjectAdoptInput struct {
	ConfigPath string
	Owner      string
	Title      string
	Number     int
	DryRun     bool
	Apply      bool
}

type WorkspaceProjectAdoptReport struct {
	Command    string               `json:"command"`
	DryRun     bool                 `json:"dry_run"`
	ConfigPath string               `json:"config_path"`
	Project    ProjectsSyncProject  `json:"project"`
	Action     WorkspaceAdoptAction `json:"action"`
	NextSteps  []string             `json:"next_steps"`
}

type WorkspaceAdoptAction struct {
	Action string `json:"action"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

func BuildWorkspaceProjectAdoptReport(input WorkspaceProjectAdoptInput, client ProjectsSyncClient) (WorkspaceProjectAdoptReport, error) {
	input.ConfigPath = strings.TrimSpace(input.ConfigPath)
	if input.ConfigPath == "" {
		input.ConfigPath = DefaultInitConfigPath(".")
	}
	input.Owner = strings.TrimSpace(input.Owner)
	input.Title = strings.TrimSpace(input.Title)
	if input.Owner == "" {
		return WorkspaceProjectAdoptReport{}, fmt.Errorf("--owner is required for workspace project adopt")
	}
	if (input.Title == "") == (input.Number == 0) {
		return WorkspaceProjectAdoptReport{}, fmt.Errorf("exactly one of --title or --number is required for workspace project adopt")
	}
	if input.Number < 0 {
		return WorkspaceProjectAdoptReport{}, fmt.Errorf("--number must be greater than 0 for workspace project adopt")
	}
	if input.DryRun == input.Apply {
		return WorkspaceProjectAdoptReport{}, fmt.Errorf("exactly one of --dry-run or --apply is required for workspace project adopt")
	}

	project, err := resolveWorkspaceAdoptProject(input, client)
	if err != nil {
		return WorkspaceProjectAdoptReport{}, err
	}
	resolved, err := ResolveWorkspaceConfig(input.ConfigPath)
	if err != nil {
		return WorkspaceProjectAdoptReport{}, fmt.Errorf("workspace project adopt requires an existing valid workspace config: %w", err)
	}
	if !workspaceConfigHasExecutionSurface(resolved) {
		return WorkspaceProjectAdoptReport{}, fmt.Errorf("workspace project adopt requires workspace.inbox_repo and workspace.repos in %s", input.ConfigPath)
	}
	cfg, err := loadWorkspaceConfig(input.ConfigPath)
	if err != nil {
		return WorkspaceProjectAdoptReport{}, err
	}
	desired := workspaceProjectAdoptConfig(input, project)
	existing := cfg.Workspace.Project
	report := WorkspaceProjectAdoptReport{
		Command:    "workspace project adopt",
		DryRun:     input.DryRun,
		ConfigPath: input.ConfigPath,
		Project:    project,
		Action:     WorkspaceAdoptAction{Action: "workspace.project:set", Status: actionStatus(input.DryRun), Reason: "register existing GitHub Project in workspace config"},
		NextSteps:  []string{fmt.Sprintf("gira projects sync --config %s --dry-run", input.ConfigPath), "Repo issues remain the execution source of truth; the Project is a visibility surface."},
	}
	if projectConfigSet(existing) {
		if workspaceProjectConfigEquivalent(existing, desired, project) {
			if workspaceProjectConfigNeedsSpecificity(existing, desired) {
				report.Action = WorkspaceAdoptAction{Action: "workspace.project:update", Status: actionStatus(input.DryRun), Reason: "selected Project matches but config needs the requested disambiguation"}
				if input.Apply {
					if err := writeWorkspaceProjectConfig(input.ConfigPath, mergeWorkspaceProjectConfig(existing, desired)); err != nil {
						return WorkspaceProjectAdoptReport{}, err
					}
				}
				return report, nil
			}
			report.Action = WorkspaceAdoptAction{Action: "workspace.project:skip", Status: "skipped", Reason: "workspace.project already matches the selected GitHub Project"}
			return report, nil
		}
		return WorkspaceProjectAdoptReport{}, fmt.Errorf("workspace.project already points to a different Project; replace is not supported")
	}
	if input.Apply {
		if err := writeWorkspaceProjectConfig(input.ConfigPath, desired); err != nil {
			return WorkspaceProjectAdoptReport{}, err
		}
	}
	return report, nil
}

func FormatWorkspaceProjectAdoptReport(report WorkspaceProjectAdoptReport) string {
	var b strings.Builder
	mode := "apply"
	if report.DryRun {
		mode = "dry-run"
	}
	fmt.Fprintf(&b, "workspace project adopt: %s\n", mode)
	fmt.Fprintf(&b, "project: %s/%d %s\n", report.Project.Owner, report.Project.Number, report.Project.Title)
	fmt.Fprintf(&b, "config:  %s\n", report.ConfigPath)
	fmt.Fprintf(&b, "action:  %s %s", report.Action.Action, report.Action.Status)
	if report.Action.Reason != "" {
		fmt.Fprintf(&b, " - %s", report.Action.Reason)
	}
	b.WriteString("\n")
	if len(report.NextSteps) > 0 {
		fmt.Fprintf(&b, "next step: %s\n", report.NextSteps[0])
	}
	b.WriteString("note: repo issues remain the execution source of truth; the Project is a visibility surface.\n")
	return b.String()
}

func resolveWorkspaceAdoptProject(input WorkspaceProjectAdoptInput, client ProjectsSyncClient) (ProjectsSyncProject, error) {
	if input.Number > 0 {
		return client.Project(input.Owner, input.Number)
	}
	projects, err := client.Projects(input.Owner)
	if err != nil {
		return ProjectsSyncProject{}, err
	}
	var matches []ProjectsSyncProject
	for _, project := range projects {
		if strings.EqualFold(project.Title, input.Title) {
			matches = append(matches, project)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return ProjectsSyncProject{}, fmt.Errorf("GitHub Project title %q is ambiguous for owner %s", input.Title, input.Owner)
	}
	return ProjectsSyncProject{}, fmt.Errorf("GitHub Project title %q was not found for owner %s", input.Title, input.Owner)
}

func workspaceProjectAdoptConfig(input WorkspaceProjectAdoptInput, project ProjectsSyncProject) ProjectConfig {
	cfg := ProjectConfig{Owner: project.Owner}
	if cfg.Owner == "" {
		cfg.Owner = input.Owner
	}
	if input.Number > 0 {
		cfg.Number = project.Number
		if cfg.Number == 0 {
			cfg.Number = input.Number
		}
		return cfg
	}
	cfg.Title = project.Title
	if cfg.Title == "" {
		cfg.Title = input.Title
	}
	return cfg
}

func projectConfigSet(cfg ProjectConfig) bool {
	return strings.TrimSpace(cfg.Owner) != "" || strings.TrimSpace(cfg.Title) != "" || cfg.Number != 0
}

func workspaceProjectConfigEquivalent(existing ProjectConfig, desired ProjectConfig, project ProjectsSyncProject) bool {
	if strings.TrimSpace(existing.Owner) != "" && strings.TrimSpace(desired.Owner) != "" && !strings.EqualFold(existing.Owner, desired.Owner) {
		return false
	}
	if existing.Number > 0 && desired.Number > 0 {
		return existing.Number == desired.Number
	}
	if strings.TrimSpace(existing.Title) != "" && strings.TrimSpace(desired.Title) != "" {
		return strings.EqualFold(existing.Title, desired.Title)
	}
	if existing.Number > 0 && project.Number > 0 {
		return existing.Number == project.Number
	}
	if strings.TrimSpace(existing.Title) != "" && strings.TrimSpace(project.Title) != "" {
		return strings.EqualFold(existing.Title, project.Title)
	}
	return false
}

func workspaceProjectConfigNeedsSpecificity(existing ProjectConfig, desired ProjectConfig) bool {
	if desired.Number > 0 && existing.Number != desired.Number {
		return true
	}
	if strings.TrimSpace(desired.Title) != "" && !strings.EqualFold(strings.TrimSpace(existing.Title), strings.TrimSpace(desired.Title)) {
		return true
	}
	if strings.TrimSpace(desired.Owner) != "" && !strings.EqualFold(strings.TrimSpace(existing.Owner), strings.TrimSpace(desired.Owner)) {
		return true
	}
	return false
}

func mergeWorkspaceProjectConfig(existing ProjectConfig, desired ProjectConfig) ProjectConfig {
	merged := existing
	if strings.TrimSpace(desired.Owner) != "" {
		merged.Owner = desired.Owner
	}
	if desired.Number > 0 {
		merged.Number = desired.Number
	}
	if strings.TrimSpace(desired.Title) != "" {
		merged.Title = desired.Title
	}
	return merged
}

func workspaceConfigHasExecutionSurface(config WorkspaceConfigResolved) bool {
	return strings.TrimSpace(config.InboxRepo.FullName()) != "" && len(config.Repos) > 0
}

func writeWorkspaceProjectConfig(path string, project ProjectConfig) error {
	if strings.EqualFold(filepath.Ext(path), ".toml") {
		return fmt.Errorf("workspace project adopt apply does not support TOML config yet; use YAML .gira/config.yaml")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read workspace config %q: %w", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat workspace config %q: %w", path, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return fmt.Errorf("parse workspace config %q: %w", path, err)
	}
	root := yamlDocumentMapping(&doc)
	workspace := yamlMappingChild(root, "workspace")
	if workspace == nil {
		workspace = &yaml.Node{Kind: yaml.MappingNode}
		yamlSetMappingChild(root, "workspace", workspace)
	}
	projectNode := yamlProjectConfigNode(project)
	yamlSetMappingChild(workspace, "project", projectNode)

	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(&doc); err != nil {
		return fmt.Errorf("encode workspace config %q: %w", path, err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("encode workspace config %q: %w", path, err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp workspace config %q: %w", path, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(out.Bytes()); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp workspace config %q: %w", path, err)
	}
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp workspace config %q: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp workspace config %q: %w", path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace workspace config %q: %w", path, err)
	}
	return nil
}

func yamlDocumentMapping(doc *yaml.Node) *yaml.Node {
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			doc.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
		}
		return doc.Content[0]
	}
	if doc.Kind == 0 {
		doc.Kind = yaml.DocumentNode
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
		return doc.Content[0]
	}
	return doc
}

func yamlMappingChild(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func yamlSetMappingChild(mapping *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = value
			return
		}
	}
	mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, value)
}

func yamlProjectConfigNode(project ProjectConfig) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode}
	yamlSetMappingChild(node, "owner", yamlScalar(project.Owner))
	if project.Number > 0 {
		yamlSetMappingChild(node, "number", yamlScalar(fmt.Sprintf("%d", project.Number)))
	}
	if strings.TrimSpace(project.Title) != "" {
		yamlSetMappingChild(node, "title", yamlScalar(project.Title))
	}
	return node
}

func yamlScalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: value}
}
