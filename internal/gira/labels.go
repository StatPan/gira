package gira

import (
	"encoding/json"
	"fmt"
	"strings"
)

func fetchRepoLabelNames(repo RepoRef, runner CommandRunner) ([]string, error) {
	labels, err := fetchRepoLabelNamesREST(repo, runner)
	if err == nil {
		return labels, nil
	}
	return fetchRepoLabelNamesGHList(repo, runner)
}

func fetchRepoLabelNamesREST(repo RepoRef, runner CommandRunner) ([]string, error) {
	output, err := runner.Run("gh", "api", "repos/"+repo.FullName()+"/labels", "--paginate", "--slurp", "-X", "GET", "-f", "per_page=100")
	if err != nil {
		return nil, err
	}
	var pages json.RawMessage
	if err := json.Unmarshal(output, &pages); err != nil {
		return nil, fmt.Errorf("parse label pages: %w", err)
	}
	rows, err := flattenPages(pages)
	if err != nil {
		return nil, err
	}
	labels := make([]string, 0, len(rows))
	for _, row := range rows {
		var raw struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(row, &raw); err != nil {
			return nil, fmt.Errorf("parse label row: %w", err)
		}
		if name := strings.TrimSpace(raw.Name); name != "" {
			labels = append(labels, name)
		}
	}
	return labels, nil
}

func fetchRepoLabelNamesGHList(repo RepoRef, runner CommandRunner) ([]string, error) {
	output, err := runner.Run("gh", "label", "list", "--repo", repo.FullName(), "--json", "name", "--limit", "1000")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(output, &rows); err != nil {
		return nil, fmt.Errorf("parse label list: %w", err)
	}
	labels := make([]string, 0, len(rows))
	for _, row := range rows {
		if name := strings.TrimSpace(row.Name); name != "" {
			labels = append(labels, name)
		}
	}
	return labels, nil
}
