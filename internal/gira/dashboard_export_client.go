package gira

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type DashboardExportClient interface {
	Repo() RepoRef
	FetchIssues() ([]DashboardRawIssue, error)
	FetchPullRequests() ([]DashboardRawPullRequest, error)
	FetchMilestones() ([]DashboardRawMilestone, error)
	FetchProjectSnapshot() (ProjectSyncSnapshot, error)
	FetchTransitionSnapshot() (ProjectTransitionSnapshot, error)
	FetchCapabilities() (ProjectCapabilityReport, error)
}

type GHDashboardExportClient struct {
	repo   RepoRef
	runner CommandRunner
}

func NewGHDashboardExportClient(repo RepoRef, runner CommandRunner) GHDashboardExportClient {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	return GHDashboardExportClient{repo: repo, runner: runner}
}

func (c GHDashboardExportClient) Repo() RepoRef {
	return c.repo
}

func (c GHDashboardExportClient) FetchIssues() ([]DashboardRawIssue, error) {
	output, err := c.runner.Run(
		"gh",
		"api",
		"repos/"+c.repo.FullName()+"/issues",
		"--paginate",
		"--slurp",
		"-X",
		"GET",
		"-f",
		"state=all",
		"-f",
		"per_page=100",
	)
	if err != nil {
		return nil, err
	}

	var pages json.RawMessage
	if err := json.Unmarshal(output, &pages); err != nil {
		return nil, fmt.Errorf("parse issue pages: %w", err)
	}
	rows, err := flattenPages(pages)
	if err != nil {
		return nil, err
	}

	issues := make([]DashboardRawIssue, 0, len(rows))
	for _, row := range rows {
		var raw struct {
			Number      int    `json:"number"`
			Title       string `json:"title"`
			State       string `json:"state"`
			UpdatedAt   string `json:"updated_at"`
			HTMLURL     string `json:"html_url"`
			URL         string `json:"url"`
			PullRequest *struct {
			} `json:"pull_request"`
			Milestone *struct {
				Title string `json:"title"`
			} `json:"milestone"`
			Labels []struct {
				Name string `json:"name"`
			} `json:"labels"`
		}
		if err := json.Unmarshal(row, &raw); err != nil {
			return nil, fmt.Errorf("parse issue row: %w", err)
		}
		if raw.PullRequest != nil {
			continue
		}

		labels := make([]string, 0, len(raw.Labels))
		for _, label := range raw.Labels {
			if strings.TrimSpace(label.Name) != "" {
				labels = append(labels, label.Name)
			}
		}
		sort.Strings(labels)

		milestone := ""
		if raw.Milestone != nil {
			milestone = raw.Milestone.Title
		}
		url := raw.HTMLURL
		if url == "" {
			url = raw.URL
		}

		issues = append(issues, DashboardRawIssue{
			IssueNumber: raw.Number,
			Title:       raw.Title,
			State:       raw.State,
			Labels:      labels,
			UpdatedAt:   raw.UpdatedAt,
			Milestone:   milestone,
			URL:         url,
		})
	}
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].IssueNumber == issues[j].IssueNumber {
			return issues[i].Title < issues[j].Title
		}
		return issues[i].IssueNumber < issues[j].IssueNumber
	})
	return issues, nil
}

func (c GHDashboardExportClient) FetchPullRequests() ([]DashboardRawPullRequest, error) {
	output, err := c.runner.Run(
		"gh",
		"api",
		"repos/"+c.repo.FullName()+"/pulls",
		"--paginate",
		"--slurp",
		"-X",
		"GET",
		"-f",
		"state=all",
		"-f",
		"per_page=100",
	)
	if err != nil {
		return nil, err
	}

	var pages json.RawMessage
	if err := json.Unmarshal(output, &pages); err != nil {
		return nil, fmt.Errorf("parse pull pages: %w", err)
	}
	rows, err := flattenPages(pages)
	if err != nil {
		return nil, err
	}

	pulls := make([]DashboardRawPullRequest, 0, len(rows))
	for _, row := range rows {
		var raw struct {
			Number  int    `json:"number"`
			Title   string `json:"title"`
			State   string `json:"state"`
			Draft   bool   `json:"draft"`
			URL     string `json:"url"`
			HTMLURL string `json:"html_url"`
			Labels  []struct {
				Name string `json:"name"`
			} `json:"labels"`
		}
		if err := json.Unmarshal(row, &raw); err != nil {
			return nil, fmt.Errorf("parse pull row: %w", err)
		}
		labels := make([]string, 0, len(raw.Labels))
		for _, label := range raw.Labels {
			if strings.TrimSpace(label.Name) != "" {
				labels = append(labels, label.Name)
			}
		}
		sort.Strings(labels)

		url := raw.HTMLURL
		if url == "" {
			url = raw.URL
		}
		pulls = append(pulls, DashboardRawPullRequest{
			PullRequestNumber: raw.Number,
			Title:             raw.Title,
			State:             raw.State,
			Draft:             raw.Draft,
			Labels:            labels,
			URL:               url,
		})
	}
	sort.Slice(pulls, func(i, j int) bool {
		if pulls[i].PullRequestNumber == pulls[j].PullRequestNumber {
			return pulls[i].Title < pulls[j].Title
		}
		return pulls[i].PullRequestNumber < pulls[j].PullRequestNumber
	})
	return pulls, nil
}

func (c GHDashboardExportClient) FetchMilestones() ([]DashboardRawMilestone, error) {
	output, err := c.runner.Run(
		"gh",
		"api",
		"repos/"+c.repo.FullName()+"/milestones",
		"--paginate",
		"--slurp",
		"-X",
		"GET",
		"-f",
		"state=all",
		"-f",
		"per_page=100",
	)
	if err != nil {
		return nil, err
	}

	var pages json.RawMessage
	if err := json.Unmarshal(output, &pages); err != nil {
		return nil, fmt.Errorf("parse milestone pages: %w", err)
	}
	rows, err := flattenPages(pages)
	if err != nil {
		return nil, err
	}

	milestones := make([]DashboardRawMilestone, 0, len(rows))
	for _, row := range rows {
		var raw struct {
			Number       int     `json:"number"`
			Title        string  `json:"title"`
			State        string  `json:"state"`
			Description  string  `json:"description"`
			DueOn        *string `json:"due_on"`
			OpenIssues   int     `json:"open_issues"`
			ClosedIssues int     `json:"closed_issues"`
		}
		if err := json.Unmarshal(row, &raw); err != nil {
			return nil, fmt.Errorf("parse milestone row: %w", err)
		}
		milestones = append(milestones, DashboardRawMilestone{
			MilestoneNumber: raw.Number,
			Title:           raw.Title,
			State:           raw.State,
			Description:     raw.Description,
			DueOn:           raw.DueOn,
			OpenIssues:      raw.OpenIssues,
			ClosedIssues:    raw.ClosedIssues,
		})
	}
	sort.Slice(milestones, func(i, j int) bool {
		if milestones[i].MilestoneNumber == milestones[j].MilestoneNumber {
			return milestones[i].Title < milestones[j].Title
		}
		return milestones[i].MilestoneNumber < milestones[j].MilestoneNumber
	})
	return milestones, nil
}

func (c GHDashboardExportClient) FetchProjectSnapshot() (ProjectSyncSnapshot, error) {
	client := NewGHProjectSyncClient(c.repo, c.runner)
	return client.Snapshot(ProductOSProjectName)
}

func (c GHDashboardExportClient) FetchTransitionSnapshot() (ProjectTransitionSnapshot, error) {
	client := NewGHProjectTransitionsClient(c.repo, c.runner)
	return client.Snapshot()
}

func (c GHDashboardExportClient) FetchCapabilities() (ProjectCapabilityReport, error) {
	return BuildProjectCapabilityReport(c.repo, c.runner)
}
