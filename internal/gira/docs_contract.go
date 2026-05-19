package gira

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type DocsContractRefreshReport struct {
	Root    string   `json:"root"`
	Updated []string `json:"updated"`
}

func RefreshDocsContract(root string) (DocsContractRefreshReport, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}
	report := DocsContractRefreshReport{Root: root}
	writes := map[string]string{
		filepath.Join(root, "docs-site", "command-reference.md"):    RenderCommandReferenceMarkdown(CoreCommandSpecs()),
		filepath.Join(root, "docs-site", "agent-operator-skill.md"): RenderAgentOperatorDocsSiteMarkdown(CoreAgentGuidanceSpec(), CoreCommandSpecs()),
	}
	for path, content := range writes {
		if err := writeDocsContractFile(path, content); err != nil {
			return report, err
		}
		report.Updated = append(report.Updated, filepath.ToSlash(strings.TrimPrefix(path, root+string(os.PathSeparator))))
	}
	skillPath := filepath.Join(root, "docs", "skills", "gira-agent-operator.md")
	if err := refreshAgentSkillManagedBlock(skillPath); err != nil {
		return report, err
	}
	report.Updated = append(report.Updated, filepath.ToSlash(filepath.Join("docs", "skills", "gira-agent-operator.md")))
	return report, nil
}

func refreshAgentSkillManagedBlock(path string) error {
	existing, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read agent skill: %w", err)
	}
	updated, err := ReplaceSingleManagedBlock(string(existing), AgentSkillBlockStart, AgentSkillBlockEnd, RenderAgentSkillManagedBlock(CoreCommandSpecs()))
	if err != nil {
		return fmt.Errorf("refresh agent skill managed block: %w", err)
	}
	return writeDocsContractFile(path, updated)
}

func ReplaceSingleManagedBlock(text string, start string, end string, block string) (string, error) {
	lines := splitLinesPreserve(text)
	starts := []int{}
	ends := []int{}
	for i, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\n"))
		switch trimmed {
		case start:
			starts = append(starts, i)
		case end:
			ends = append(ends, i)
		}
	}
	if len(starts) == 0 {
		return "", fmt.Errorf("missing managed block start %s", start)
	}
	if len(ends) == 0 {
		return "", fmt.Errorf("missing managed block end %s", end)
	}
	ranges := [][2]int{}
	for _, startLine := range starts {
		endLine := -1
		for _, candidate := range ends {
			if candidate > startLine {
				endLine = candidate
				break
			}
		}
		if endLine < 0 {
			return "", fmt.Errorf("managed block start without end %s", start)
		}
		ranges = append(ranges, [2]int{startLine, endLine})
	}
	resultLines := []string{}
	for i := 0; i < len(lines); i++ {
		if i == ranges[0][0] {
			resultLines = append(resultLines, strings.SplitAfter(strings.TrimRight(block, "\n")+"\n", "\n")...)
			i = ranges[0][1]
			continue
		}
		skip := false
		for _, duplicate := range ranges[1:] {
			if i == duplicate[0] {
				i = duplicate[1]
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		resultLines = append(resultLines, lines[i])
	}
	return strings.Join(resultLines, ""), nil
}

func splitLinesPreserve(text string) []string {
	if text == "" {
		return []string{}
	}
	lines := strings.SplitAfter(text, "\n")
	if lines[len(lines)-1] == "" {
		return lines[:len(lines)-1]
	}
	return lines
}

func writeDocsContractFile(path string, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
