package gira

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	templatefs "github.com/StatPan/gira/templates"
)

type RenderedTemplate struct {
	Path    string
	Content string
}

func RenderTemplateTree(template string, repo RepoRef, createdAt string) ([]RenderedTemplate, error) {
	if strings.Contains(template, "/") || strings.Contains(template, "\\") || template == "" {
		return nil, fmt.Errorf("invalid template name: %s", template)
	}
	if template != "default" {
		return nil, fmt.Errorf("unknown template: %s", template)
	}

	root := "default"
	var paths []string
	if err := fs.WalkDir(templatefs.FS, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(paths)

	context := map[string]string{
		"repo_owner":     repo.Owner,
		"repo_name":      repo.Name,
		"repo_full_name": repo.FullName(),
		"created_at":     createdAt,
	}

	rendered := make([]RenderedTemplate, 0, len(paths))
	for _, path := range paths {
		contentBytes, err := templatefs.FS.ReadFile(path)
		if err != nil {
			return nil, err
		}
		rel := strings.TrimPrefix(path, root+"/")
		outRel := strings.TrimSuffix(rel, ".j2")
		content := renderSimpleTemplate(string(contentBytes), context)
		if strings.Contains(content, "{{") || strings.Contains(content, "{%") || strings.Contains(content, "{#") {
			return nil, fmt.Errorf("unsupported template expression in %s", rel)
		}
		rendered = append(rendered, RenderedTemplate{Path: outRel, Content: content})
	}

	return rendered, nil
}

func FormatDryRun(rendered []RenderedTemplate) string {
	var b strings.Builder
	for _, item := range rendered {
		b.WriteString("--- ")
		b.WriteString(item.Path)
		b.WriteString("\n")
		b.WriteString(strings.TrimRight(item.Content, "\n"))
		b.WriteString("\n\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func renderSimpleTemplate(input string, context map[string]string) string {
	output := input
	for key, value := range context {
		output = strings.ReplaceAll(output, "{{ "+key+" }}", value)
		output = strings.ReplaceAll(output, "{{"+key+"}}", value)
	}
	return output
}
