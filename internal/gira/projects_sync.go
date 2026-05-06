package gira

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ProjectsSyncClient interface {
	Project(owner string, number int) (ProjectsSyncProject, error)
	Projects(owner string) ([]ProjectsSyncProject, error)
	LinkedProjects(repo RepoRef) ([]ProjectsSyncProject, error)
	StatusField(owner string, number int) (ProjectsSyncStatusField, error)
	ProjectFields(projectID string) ([]ProjectsSyncField, error)
	RepoLinked(owner string, number int, repo RepoRef) (bool, error)
	OpenIssues(repo RepoRef) ([]ProjectsSyncIssue, error)
	ProjectItems(owner string, number int) ([]ProjectsSyncItem, error)
	ProjectItemsGraphQL(projectID string) ([]ProjectsSyncItem, error)
	CreateProjectField(owner string, number int, field ProjectsSyncFieldDef) (string, error)
	LinkRepo(owner string, number int, repo RepoRef) error
	AddItem(owner string, number int, issue ProjectsSyncIssue) (string, error)
	UpdateItemStatus(projectID string, itemID string, fieldID string, optionID string) error
	UpdateItemDate(projectID string, itemID string, fieldID string, date string) error
}

type GHProjectsSyncClient struct {
	runner CommandRunner
}

func NewGHProjectsSyncClient(runner CommandRunner) GHProjectsSyncClient {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	return GHProjectsSyncClient{runner: runner}
}

type ProjectsSyncProject struct {
	ID     string `json:"id"`
	Owner  string `json:"owner"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

type ProjectsSyncStatusField struct {
	ID      string            `json:"id"`
	Options map[string]string `json:"options"`
}

type ProjectsSyncFieldDef struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Options []string `json:"options,omitempty"`
}

type ProjectsSyncField struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Options map[string]string `json:"options,omitempty"`
}

type ProjectsSyncIssue struct {
	Repo             string   `json:"repo"`
	Number           int      `json:"number"`
	Title            string   `json:"title"`
	URL              string   `json:"url"`
	Labels           []string `json:"labels"`
	Milestone        string   `json:"milestone,omitempty"`
	MilestoneDueDate string   `json:"milestone_due_date,omitempty"`
}

type ProjectsSyncItem struct {
	ID         string `json:"id"`
	Repo       string `json:"repo"`
	Number     int    `json:"number"`
	Status     string `json:"status,omitempty"`
	TargetDate string `json:"target_date,omitempty"`
}

type ProjectsSyncReport struct {
	Command   string               `json:"command"`
	DryRun    bool                 `json:"dry_run"`
	Workspace WorkspaceSummary     `json:"workspace"`
	Project   ProjectsSyncProject  `json:"project"`
	Repos     []string             `json:"repos"`
	Counts    ProjectsSyncCounts   `json:"counts"`
	Actions   []ProjectsSyncAction `json:"actions"`
	FetchedAt string               `json:"fetched_at"`
	NextSteps []string             `json:"next_steps"`
}

type ProjectsSyncCounts struct {
	Repos             int  `json:"repos"`
	Issues            int  `json:"issues"`
	FieldsCreate      int  `json:"fields_create"`
	FieldsSkip        int  `json:"fields_skip"`
	ProjectLinksAdd   int  `json:"project_links_add"`
	ProjectItemsAdd   int  `json:"project_items_add"`
	ProjectItemsSkip  int  `json:"project_items_skip"`
	StatusUpdates     int  `json:"status_updates"`
	StatusUpdateSkips int  `json:"status_update_skips"`
	DateUpdates       int  `json:"date_updates"`
	DateUpdateSkips   int  `json:"date_update_skips"`
	ViewSetupRequired bool `json:"view_setup_required"`
}

type ProjectsSyncAction struct {
	Action        string `json:"action"`
	Repo          string `json:"repo,omitempty"`
	Issue         int    `json:"issue,omitempty"`
	ItemID        string `json:"item_id,omitempty"`
	FieldID       string `json:"field_id,omitempty"`
	FieldName     string `json:"field_name,omitempty"`
	FieldType     string `json:"field_type,omitempty"`
	FromStatus    string `json:"from_status,omitempty"`
	ToStatus      string `json:"to_status,omitempty"`
	FromDate      string `json:"from_date,omitempty"`
	ToDate        string `json:"to_date,omitempty"`
	Status        string `json:"status"`
	Reason        string `json:"reason,omitempty"`
	AppliedItemID string `json:"applied_item_id,omitempty"`
}

var projectsSyncCanonicalFields = []ProjectsSyncFieldDef{
	{Name: "Status", Type: "SINGLE_SELECT", Options: []string{"Todo", "In Progress", "Done"}},
	{Name: "Priority", Type: "SINGLE_SELECT", Options: []string{"P0", "P1", "P2", "P3"}},
	{Name: "Layer / workstream", Type: "SINGLE_SELECT", Options: []string{"Product", "Backend", "Infra", "Docs"}},
	{Name: "Owner / agent", Type: "TEXT"},
	{Name: "Start date", Type: "DATE"},
	{Name: "Target date", Type: "DATE"},
}

func (c GHProjectsSyncClient) Project(owner string, number int) (ProjectsSyncProject, error) {
	output, err := c.runner.Run("gh", "project", "view", strconv.Itoa(number), "--owner", owner, "--format", "json")
	if err != nil {
		return ProjectsSyncProject{}, err
	}
	var raw struct {
		ID     string `json:"id"`
		Number int    `json:"number"`
		Title  string `json:"title"`
		URL    string `json:"url"`
		Owner  struct {
			Login string `json:"login"`
		} `json:"owner"`
	}
	if err := json.Unmarshal(output, &raw); err != nil {
		return ProjectsSyncProject{}, fmt.Errorf("parse project view JSON: %w", err)
	}
	return ProjectsSyncProject{ID: raw.ID, Owner: raw.Owner.Login, Number: raw.Number, Title: raw.Title, URL: raw.URL}, nil
}

func (c GHProjectsSyncClient) Projects(owner string) ([]ProjectsSyncProject, error) {
	output, err := c.runner.Run("gh", "project", "list", "--owner", owner, "--format", "json", "--limit", "100")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Projects []struct {
			ID     string `json:"id"`
			Number int    `json:"number"`
			Title  string `json:"title"`
			URL    string `json:"url"`
			Owner  struct {
				Login string `json:"login"`
			} `json:"owner"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil, fmt.Errorf("parse project list JSON: %w", err)
	}
	projects := make([]ProjectsSyncProject, 0, len(payload.Projects))
	for _, raw := range payload.Projects {
		projects = append(projects, ProjectsSyncProject{ID: raw.ID, Owner: raw.Owner.Login, Number: raw.Number, Title: raw.Title, URL: raw.URL})
	}
	return projects, nil
}

func (c GHProjectsSyncClient) LinkedProjects(repo RepoRef) ([]ProjectsSyncProject, error) {
	query := `query($o:String!,$n:String!){repository(owner:$o,name:$n){projectsV2(first:50){nodes{id number title url owner{... on User{login} ... on Organization{login}}}}}}`
	output, err := c.runner.Run("gh", "api", "graphql", "-f", "query="+query, "-f", "o="+repo.Owner, "-f", "n="+repo.Name)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data struct {
			Repository struct {
				ProjectsV2 struct {
					Nodes []struct {
						ID     string `json:"id"`
						Number int    `json:"number"`
						Title  string `json:"title"`
						URL    string `json:"url"`
						Owner  struct {
							Login string `json:"login"`
						} `json:"owner"`
					} `json:"nodes"`
				} `json:"projectsV2"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil, fmt.Errorf("parse linked projects JSON: %w", err)
	}
	projects := make([]ProjectsSyncProject, 0, len(payload.Data.Repository.ProjectsV2.Nodes))
	for _, raw := range payload.Data.Repository.ProjectsV2.Nodes {
		projects = append(projects, ProjectsSyncProject{ID: raw.ID, Owner: raw.Owner.Login, Number: raw.Number, Title: raw.Title, URL: raw.URL})
	}
	return projects, nil
}

func (c GHProjectsSyncClient) StatusField(owner string, number int) (ProjectsSyncStatusField, error) {
	output, err := c.runner.Run("gh", "project", "field-list", strconv.Itoa(number), "--owner", owner, "--format", "json")
	if err != nil {
		return ProjectsSyncStatusField{}, err
	}
	var payload struct {
		Fields []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Options []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"options"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return ProjectsSyncStatusField{}, fmt.Errorf("parse project field-list JSON: %w", err)
	}
	for _, field := range payload.Fields {
		if field.Name != "Status" {
			continue
		}
		options := map[string]string{}
		for _, option := range field.Options {
			options[option.Name] = option.ID
		}
		return ProjectsSyncStatusField{ID: field.ID, Options: options}, nil
	}
	return ProjectsSyncStatusField{}, nil
}

func (c GHProjectsSyncClient) ProjectFields(projectID string) ([]ProjectsSyncField, error) {
	query := `query($id:ID!){node(id:$id){... on ProjectV2{fields(first:100){nodes{... on ProjectV2Field{id name dataType} ... on ProjectV2SingleSelectField{id name dataType options{id name}}}}}}}`
	output, err := c.runner.Run("gh", "api", "graphql", "-f", "query="+query, "-f", "id="+projectID)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data struct {
			Node struct {
				Fields struct {
					Nodes []struct {
						ID       string `json:"id"`
						Name     string `json:"name"`
						DataType string `json:"dataType"`
						Options  []struct {
							ID   string `json:"id"`
							Name string `json:"name"`
						} `json:"options"`
					} `json:"nodes"`
				} `json:"fields"`
			} `json:"node"`
		} `json:"data"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil, fmt.Errorf("parse project fields JSON: %w", err)
	}
	fields := make([]ProjectsSyncField, 0, len(payload.Data.Node.Fields.Nodes))
	for _, raw := range payload.Data.Node.Fields.Nodes {
		field := ProjectsSyncField{ID: raw.ID, Name: raw.Name, Type: raw.DataType}
		if len(raw.Options) > 0 {
			field.Options = map[string]string{}
			for _, option := range raw.Options {
				field.Options[option.Name] = option.ID
			}
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func (c GHProjectsSyncClient) RepoLinked(owner string, number int, repo RepoRef) (bool, error) {
	projects, err := c.LinkedProjects(repo)
	if err != nil {
		return false, err
	}
	for _, project := range projects {
		if project.Number == number && strings.EqualFold(project.Owner, owner) {
			return true, nil
		}
	}
	return false, nil
}

func (c GHProjectsSyncClient) OpenIssues(repo RepoRef) ([]ProjectsSyncIssue, error) {
	issues, err := FetchIssues(NewGHStatusClient(repo, c.runner))
	if err != nil {
		return nil, err
	}
	milestones, err := FetchMilestones(NewGHStatusClient(repo, c.runner))
	if err != nil {
		return nil, err
	}
	dueByMilestone := map[string]string{}
	for _, milestone := range milestones {
		if milestone.DueOn == nil {
			continue
		}
		if due, ok := normalizeDate(*milestone.DueOn); ok {
			dueByMilestone[milestone.Title] = due
		}
	}
	out := make([]ProjectsSyncIssue, 0, len(issues))
	for _, issue := range issues {
		if !strings.EqualFold(issue.State, "open") {
			continue
		}
		milestone := ""
		milestoneDue := ""
		if issue.Milestone != nil {
			milestone = *issue.Milestone
			milestoneDue = dueByMilestone[milestone]
		}
		out = append(out, ProjectsSyncIssue{Repo: repo.FullName(), Number: issue.Number, Title: issue.Title, URL: issue.URL, Labels: issue.Labels, Milestone: milestone, MilestoneDueDate: milestoneDue})
	}
	return out, nil
}

func (c GHProjectsSyncClient) ProjectItems(owner string, number int) ([]ProjectsSyncItem, error) {
	output, err := c.runner.Run("gh", "project", "item-list", strconv.Itoa(number), "--owner", owner, "--format", "json", "--limit", "500")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Items []struct {
			ID      string `json:"id"`
			Status  string `json:"status"`
			Content *struct {
				Number     int    `json:"number"`
				Repository string `json:"repository"`
			} `json:"content"`
		} `json:"items"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil, fmt.Errorf("parse project item-list JSON: %w", err)
	}
	items := make([]ProjectsSyncItem, 0, len(payload.Items))
	for _, item := range payload.Items {
		if item.Content == nil || item.Content.Number == 0 || item.Content.Repository == "" {
			continue
		}
		items = append(items, ProjectsSyncItem{ID: item.ID, Repo: item.Content.Repository, Number: item.Content.Number, Status: item.Status})
	}
	return items, nil
}

func (c GHProjectsSyncClient) ProjectItemsGraphQL(projectID string) ([]ProjectsSyncItem, error) {
	query := `query($id:ID!){node(id:$id){... on ProjectV2{items(first:100){nodes{id content{... on Issue{number repository{nameWithOwner}}} fieldValues(first:50){nodes{... on ProjectV2ItemFieldDateValue{date field{... on ProjectV2FieldCommon{name}}} ... on ProjectV2ItemFieldSingleSelectValue{name field{... on ProjectV2FieldCommon{name}}}}}}}}}}`
	output, err := c.runner.Run("gh", "api", "graphql", "-f", "query="+query, "-f", "id="+projectID)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data struct {
			Node struct {
				Items struct {
					Nodes []struct {
						ID      string `json:"id"`
						Content *struct {
							Number     int `json:"number"`
							Repository struct {
								NameWithOwner string `json:"nameWithOwner"`
							} `json:"repository"`
						} `json:"content"`
						FieldValues struct {
							Nodes []struct {
								Date  *string `json:"date"`
								Name  string  `json:"name"`
								Field *struct {
									Name string `json:"name"`
								} `json:"field"`
							} `json:"nodes"`
						} `json:"fieldValues"`
					} `json:"nodes"`
				} `json:"items"`
			} `json:"node"`
		} `json:"data"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil, fmt.Errorf("parse project items GraphQL JSON: %w", err)
	}
	items := make([]ProjectsSyncItem, 0, len(payload.Data.Node.Items.Nodes))
	for _, raw := range payload.Data.Node.Items.Nodes {
		if raw.Content == nil || raw.Content.Number == 0 || raw.Content.Repository.NameWithOwner == "" {
			continue
		}
		item := ProjectsSyncItem{ID: raw.ID, Repo: raw.Content.Repository.NameWithOwner, Number: raw.Content.Number}
		for _, value := range raw.FieldValues.Nodes {
			if value.Field == nil {
				continue
			}
			switch value.Field.Name {
			case "Status":
				item.Status = value.Name
			case "Target date":
				if value.Date != nil {
					if date, ok := normalizeDate(*value.Date); ok {
						item.TargetDate = date
					}
				}
			}
		}
		items = append(items, item)
	}
	return items, nil
}

func (c GHProjectsSyncClient) LinkRepo(owner string, number int, repo RepoRef) error {
	repoValue := repo.FullName()
	if strings.EqualFold(owner, repo.Owner) {
		repoValue = repo.Name
	}
	_, err := c.runner.Run("gh", "project", "link", strconv.Itoa(number), "--owner", owner, "--repo", repoValue)
	return err
}

func (c GHProjectsSyncClient) AddItem(owner string, number int, issue ProjectsSyncIssue) (string, error) {
	output, err := c.runner.Run("gh", "project", "item-add", strconv.Itoa(number), "--owner", owner, "--url", issue.URL, "--format", "json")
	if err != nil {
		return "", err
	}
	var raw struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(output, &raw); err != nil {
		return "", fmt.Errorf("parse project item-add JSON: %w", err)
	}
	return raw.ID, nil
}

func (c GHProjectsSyncClient) CreateProjectField(owner string, number int, field ProjectsSyncFieldDef) (string, error) {
	args := []string{"project", "field-create", strconv.Itoa(number), "--owner", owner, "--name", field.Name, "--data-type", field.Type, "--format", "json"}
	if field.Type == "SINGLE_SELECT" && len(field.Options) > 0 {
		args = append(args, "--single-select-options", strings.Join(field.Options, ","))
	}
	output, err := c.runner.Run("gh", args...)
	if err != nil {
		return "", err
	}
	var raw struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(output, &raw); err != nil {
		return "", fmt.Errorf("parse project field-create JSON: %w", err)
	}
	return raw.ID, nil
}

func (c GHProjectsSyncClient) UpdateItemStatus(projectID string, itemID string, fieldID string, optionID string) error {
	_, err := c.runner.Run("gh", "project", "item-edit", "--id", itemID, "--project-id", projectID, "--field-id", fieldID, "--single-select-option-id", optionID)
	return err
}

func (c GHProjectsSyncClient) UpdateItemDate(projectID string, itemID string, fieldID string, date string) error {
	_, err := c.runner.Run("gh", "project", "item-edit", "--id", itemID, "--project-id", projectID, "--field-id", fieldID, "--date", date)
	return err
}

func BuildProjectsSyncReport(config WorkspaceConfigResolved, client ProjectsSyncClient, dryRun bool, fetchedAt time.Time) (ProjectsSyncReport, error) {
	projectOwner := strings.TrimSpace(config.Project.Owner)
	if projectOwner == "" {
		projectOwner = config.Owner
	}
	project, err := resolveProjectsSyncProject(config, client, projectOwner)
	if err != nil {
		return ProjectsSyncReport{}, err
	}
	report := ProjectsSyncReport{
		Command:   "projects sync",
		DryRun:    dryRun,
		Workspace: WorkspaceSummary{Name: config.Name, Owner: config.Owner},
		Project:   project,
		Actions:   []ProjectsSyncAction{},
		FetchedAt: fetchedAt.UTC().Format(time.RFC3339),
	}

	fields, err := client.ProjectFields(project.ID)
	if err != nil {
		return ProjectsSyncReport{}, err
	}
	fieldsByName := map[string]ProjectsSyncField{}
	for _, field := range fields {
		fieldsByName[field.Name] = field
	}
	for _, desired := range projectsSyncCanonicalFields {
		existing, exists := fieldsByName[desired.Name]
		if exists {
			report.Counts.FieldsSkip++
			if existing.Type != "" && !strings.EqualFold(existing.Type, desired.Type) {
				report.Actions = append(report.Actions, ProjectsSyncAction{Action: "project_field:skip", FieldID: existing.ID, FieldName: desired.Name, FieldType: existing.Type, Status: "skipped", Reason: fmt.Sprintf("field exists with type %s; expected %s", existing.Type, desired.Type)})
			}
			continue
		}
		action := ProjectsSyncAction{Action: "project_field:create", FieldName: desired.Name, FieldType: desired.Type, Status: actionStatus(dryRun), Reason: "canonical field is missing"}
		if !dryRun {
			fieldID, err := client.CreateProjectField(project.Owner, project.Number, desired)
			if err != nil {
				return ProjectsSyncReport{}, err
			}
			action.Status = "applied"
			action.FieldID = fieldID
			fieldsByName[desired.Name] = ProjectsSyncField{ID: fieldID, Name: desired.Name, Type: desired.Type}
		}
		report.Actions = append(report.Actions, action)
		report.Counts.FieldsCreate++
	}
	targetDateField := fieldsByName["Target date"]

	statusField, err := client.StatusField(project.Owner, project.Number)
	if err != nil {
		return ProjectsSyncReport{}, err
	}
	items, err := client.ProjectItemsGraphQL(project.ID)
	if err != nil {
		items, err = client.ProjectItems(project.Owner, project.Number)
		if err != nil {
			return ProjectsSyncReport{}, err
		}
	}
	itemByIssue := map[string]ProjectsSyncItem{}
	for _, item := range items {
		itemByIssue[projectIssueKey(item.Repo, item.Number)] = item
	}
	repos := uniqueProjectRepos(config)
	for _, repo := range repos {
		report.Repos = append(report.Repos, repo.FullName())
		linked, err := client.RepoLinked(project.Owner, project.Number, repo)
		if err != nil {
			return ProjectsSyncReport{}, err
		}
		if !linked {
			action := ProjectsSyncAction{Action: "project_repo:link", Repo: repo.FullName(), Status: actionStatus(dryRun), Reason: "project is not linked to repository"}
			if !dryRun {
				if err := client.LinkRepo(project.Owner, project.Number, repo); err != nil {
					return ProjectsSyncReport{}, err
				}
				action.Status = "applied"
			}
			report.Actions = append(report.Actions, action)
			report.Counts.ProjectLinksAdd++
		}
		issues, err := client.OpenIssues(repo)
		if err != nil {
			return ProjectsSyncReport{}, err
		}
		sort.Slice(issues, func(i, j int) bool { return issues[i].Number < issues[j].Number })
		for _, issue := range issues {
			report.Counts.Issues++
			key := projectIssueKey(issue.Repo, issue.Number)
			item, exists := itemByIssue[key]
			if !exists {
				action := ProjectsSyncAction{Action: "project_item:add", Repo: issue.Repo, Issue: issue.Number, ToStatus: desiredProjectStatus(issue.Labels), Status: actionStatus(dryRun), Reason: "issue is not in project"}
				if !dryRun {
					itemID, err := client.AddItem(project.Owner, project.Number, issue)
					if err != nil {
						return ProjectsSyncReport{}, err
					}
					action.Status = "applied"
					action.AppliedItemID = itemID
					item = ProjectsSyncItem{ID: itemID, Repo: issue.Repo, Number: issue.Number, Status: "Todo"}
					itemByIssue[key] = item
				}
				report.Actions = append(report.Actions, action)
				report.Counts.ProjectItemsAdd++
			} else {
				report.Counts.ProjectItemsSkip++
			}
			desired := desiredProjectStatus(issue.Labels)
			current := item.Status
			if current == "" && !exists {
				current = "Todo"
			}
			if current != desired {
				optionID := statusField.Options[desired]
				if statusField.ID == "" || optionID == "" || (!dryRun && item.ID == "") {
					report.Actions = append(report.Actions, ProjectsSyncAction{Action: "project_status:update", Repo: issue.Repo, Issue: issue.Number, ItemID: item.ID, FromStatus: current, ToStatus: desired, Status: "skipped", Reason: "project Status field or option is unavailable"})
					report.Counts.StatusUpdateSkips++
				} else {
					action := ProjectsSyncAction{Action: "project_status:update", Repo: issue.Repo, Issue: issue.Number, ItemID: item.ID, FromStatus: current, ToStatus: desired, Status: actionStatus(dryRun), Reason: "project status differs from issue status label"}
					if !dryRun {
						if err := client.UpdateItemStatus(project.ID, item.ID, statusField.ID, optionID); err != nil {
							return ProjectsSyncReport{}, err
						}
						action.Status = "applied"
					}
					report.Actions = append(report.Actions, action)
					report.Counts.StatusUpdates++
					item.Status = desired
					itemByIssue[key] = item
				}
			}
			syncProjectTargetDate(&report, client, project, targetDateField, issue, item, dryRun)
		}
	}
	report.Counts.Repos = len(report.Repos)
	report.Counts.ViewSetupRequired = true
	if dryRun {
		report.NextSteps = []string{"gira projects sync --config .gira/config.yaml --apply", "In GitHub Project, create Board grouped by Status and Schedule using Start date / Target date"}
	} else {
		report.NextSteps = []string{"gira projects sync --config .gira/config.yaml --dry-run", "In GitHub Project, create Board grouped by Status and Schedule using Start date / Target date"}
	}
	return report, nil
}

func resolveProjectsSyncProject(config WorkspaceConfigResolved, client ProjectsSyncClient, owner string) (ProjectsSyncProject, error) {
	if strings.TrimSpace(owner) == "" {
		return ProjectsSyncProject{}, fmt.Errorf("workspace.project.owner is required when workspace owner cannot be inferred")
	}
	if config.Project.Number > 0 {
		project, err := client.Project(owner, config.Project.Number)
		if err != nil {
			return ProjectsSyncProject{}, err
		}
		if strings.TrimSpace(config.Project.Title) != "" && project.Title != "" && !strings.EqualFold(config.Project.Title, project.Title) {
			return ProjectsSyncProject{}, fmt.Errorf("workspace.project.title %q does not match GitHub project %q", config.Project.Title, project.Title)
		}
		return project, nil
	}
	if strings.TrimSpace(config.Project.Title) != "" {
		projects, err := client.Projects(owner)
		if err != nil {
			return ProjectsSyncProject{}, err
		}
		var matches []ProjectsSyncProject
		for _, project := range projects {
			if strings.EqualFold(project.Title, config.Project.Title) {
				matches = append(matches, project)
			}
		}
		if len(matches) == 1 {
			return matches[0], nil
		}
		if len(matches) > 1 {
			return ProjectsSyncProject{}, fmt.Errorf("workspace.project.title %q is ambiguous for owner %s", config.Project.Title, owner)
		}
		return ProjectsSyncProject{}, fmt.Errorf("workspace.project.title %q was not found for owner %s", config.Project.Title, owner)
	}
	return inferLinkedProjectsSyncProject(config, client, owner)
}

func inferLinkedProjectsSyncProject(config WorkspaceConfigResolved, client ProjectsSyncClient, owner string) (ProjectsSyncProject, error) {
	projectsByKey := map[string]ProjectsSyncProject{}
	for _, repo := range uniqueProjectRepos(config) {
		projects, err := client.LinkedProjects(repo)
		if err != nil {
			return ProjectsSyncProject{}, err
		}
		for _, project := range projects {
			if !strings.EqualFold(project.Owner, owner) {
				continue
			}
			key := strings.ToLower(project.Owner) + "#" + strconv.Itoa(project.Number)
			projectsByKey[key] = project
		}
	}
	if len(projectsByKey) == 1 {
		for _, project := range projectsByKey {
			return project, nil
		}
	}
	if len(projectsByKey) == 0 {
		return ProjectsSyncProject{}, fmt.Errorf("workspace.project.title is required because no linked GitHub Project was found")
	}
	return ProjectsSyncProject{}, fmt.Errorf("workspace.project.title is required because multiple linked GitHub Projects were found")
}

func syncProjectTargetDate(report *ProjectsSyncReport, client ProjectsSyncClient, project ProjectsSyncProject, targetDateField ProjectsSyncField, issue ProjectsSyncIssue, item ProjectsSyncItem, dryRun bool) {
	if issue.Milestone == "" {
		return
	}
	if issue.MilestoneDueDate == "" {
		report.Actions = append(report.Actions, ProjectsSyncAction{Action: "project_date:skip", Repo: issue.Repo, Issue: issue.Number, ItemID: item.ID, FieldName: "Target date", Status: "skipped", Reason: "schedule_missing_due_date"})
		report.Counts.DateUpdateSkips++
		return
	}
	if item.TargetDate == issue.MilestoneDueDate {
		report.Counts.DateUpdateSkips++
		return
	}
	if targetDateField.ID == "" || (!dryRun && item.ID == "") {
		report.Actions = append(report.Actions, ProjectsSyncAction{Action: "project_date:update", Repo: issue.Repo, Issue: issue.Number, ItemID: item.ID, FieldName: "Target date", FromDate: item.TargetDate, ToDate: issue.MilestoneDueDate, Status: "skipped", Reason: "project Target date field or item id is unavailable"})
		report.Counts.DateUpdateSkips++
		return
	}
	action := ProjectsSyncAction{Action: "project_date:update", Repo: issue.Repo, Issue: issue.Number, ItemID: item.ID, FieldID: targetDateField.ID, FieldName: "Target date", FromDate: item.TargetDate, ToDate: issue.MilestoneDueDate, Status: actionStatus(dryRun), Reason: "milestone due date differs from project Target date"}
	if !dryRun {
		if err := client.UpdateItemDate(project.ID, item.ID, targetDateField.ID, issue.MilestoneDueDate); err != nil {
			action.Status = "skipped"
			action.Reason = err.Error()
			report.Counts.DateUpdateSkips++
			report.Actions = append(report.Actions, action)
			return
		}
		action.Status = "applied"
	}
	report.Actions = append(report.Actions, action)
	report.Counts.DateUpdates++
}

func FormatProjectsSyncReport(report ProjectsSyncReport) string {
	var b strings.Builder
	mode := "apply"
	if report.DryRun {
		mode = "dry-run"
	}
	fmt.Fprintf(&b, "projects sync: %s\n", mode)
	fmt.Fprintf(&b, "project: %s #%d\n", report.Project.Title, report.Project.Number)
	fmt.Fprintf(&b, "repos: %d issues: %d fields-create: %d add-items: %d status-updates: %d date-updates: %d\n", report.Counts.Repos, report.Counts.Issues, report.Counts.FieldsCreate, report.Counts.ProjectItemsAdd, report.Counts.StatusUpdates, report.Counts.DateUpdates)
	for _, action := range report.Actions {
		target := action.Repo
		if action.Issue > 0 {
			target = fmt.Sprintf("%s#%d", action.Repo, action.Issue)
		}
		if target == "" {
			target = action.FieldName
		}
		fmt.Fprintf(&b, "  %s %s %s", action.Status, action.Action, target)
		if action.ToStatus != "" {
			fmt.Fprintf(&b, " -> %s", action.ToStatus)
		}
		if action.ToDate != "" {
			fmt.Fprintf(&b, " -> %s", action.ToDate)
		}
		if action.Reason != "" {
			fmt.Fprintf(&b, " (%s)", action.Reason)
		}
		b.WriteString("\n")
	}
	if report.Counts.ViewSetupRequired {
		b.WriteString("view setup: create Board grouped by Status and Schedule using Start date / Target date in GitHub Project UI\n")
	}
	if len(report.NextSteps) > 0 {
		fmt.Fprintf(&b, "next step: %s\n", report.NextSteps[0])
	}
	return b.String()
}

func uniqueProjectRepos(config WorkspaceConfigResolved) []RepoRef {
	out := make([]RepoRef, 0, len(config.Repos)+1)
	seen := map[string]struct{}{}
	for _, repo := range append([]RepoRef{config.InboxRepo}, config.Repos...) {
		key := strings.ToLower(repo.FullName())
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, repo)
	}
	return out
}

func projectIssueKey(repo string, issue int) string {
	return strings.ToLower(repo) + "#" + strconv.Itoa(issue)
}

func desiredProjectStatus(labels []string) string {
	for _, label := range labels {
		switch strings.ToLower(strings.TrimSpace(label)) {
		case "status:in-progress", "status:in_progress", "status:in-review", "status:in_review":
			return "In Progress"
		case "status:done":
			return "Done"
		}
	}
	return "Todo"
}

func actionStatus(dryRun bool) string {
	if dryRun {
		return "planned"
	}
	return "applied"
}
