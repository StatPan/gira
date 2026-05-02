package gira

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type GraphClient interface {
	Repo() RepoRef
	Issues() ([]GraphIssue, error)
}

type GHGraphClient struct {
	repo   RepoRef
	runner CommandRunner
}

func NewGHGraphClient(repo RepoRef, runner CommandRunner) GHGraphClient {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	return GHGraphClient{repo: repo, runner: runner}
}

func (c GHGraphClient) Repo() RepoRef { return c.repo }

type GraphIssue struct {
	Number int
	State  string
	Labels []string
	Body   string
}

func (c GHGraphClient) Issues() ([]GraphIssue, error) {
	output, err := c.runner.Run("gh", "api", "repos/"+c.repo.FullName()+"/issues", "--paginate", "--slurp", "-X", "GET", "-f", "state=all", "-f", "per_page=100")
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
	issues := make([]GraphIssue, 0, len(rows))
	for _, row := range rows {
		var raw struct {
			Number      int       `json:"number"`
			State       string    `json:"state"`
			Body        string    `json:"body"`
			PullRequest *struct{} `json:"pull_request"`
			Labels      []struct {
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
		for _, l := range raw.Labels {
			labels = append(labels, l.Name)
		}
		issues = append(issues, GraphIssue{Number: raw.Number, State: raw.State, Labels: labels, Body: raw.Body})
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].Number < issues[j].Number })
	return issues, nil
}

type GraphValidationReport struct {
	Repo        string            `json:"repo"`
	Diagnostics []GraphDiagnostic `json:"diagnostics"`
	Counts      GraphCounts       `json:"counts"`
}

type GraphCounts struct {
	Issues      int `json:"issues"`
	Diagnostics int `json:"diagnostics"`
}

type GraphDiagnostic struct {
	Issue  int    `json:"issue"`
	RuleID string `json:"rule_id"`
	Detail string `json:"detail"`
}

type issueLinks struct {
	Parent    int
	DependsOn []int
	Blocks    []int
}

var (
	linkParentRe  = regexp.MustCompile(`(?im)^\s*parent\s*:\s*#(\d+)\s*$`)
	linkDependsRe = regexp.MustCompile(`(?im)^\s*depends[_ ]on\s*:\s*([^\n]+)$`)
	linkBlocksRe  = regexp.MustCompile(`(?im)^\s*blocks\s*:\s*([^\n]+)$`)
	refRe         = regexp.MustCompile(`#(\d+)`)
)

func BuildGraphValidationReportForClient(client GraphClient) (GraphValidationReport, error) {
	issues, err := client.Issues()
	if err != nil {
		return GraphValidationReport{}, err
	}
	return BuildGraphValidationReport(client.Repo().FullName(), issues), nil
}

func BuildGraphValidationReport(repo string, issues []GraphIssue) GraphValidationReport {
	byNumber := map[int]GraphIssue{}
	for _, issue := range issues {
		byNumber[issue.Number] = issue
	}
	diags := make([]GraphDiagnostic, 0)
	adj := map[int][]int{}
	for _, issue := range issues {
		links := parseIssueLinks(issue.Body)
		if needsParent(issue.Labels) && links.Parent == 0 {
			diags = append(diags, GraphDiagnostic{Issue: issue.Number, RuleID: "missing_parent", Detail: "type:story/task requires parent linkage"})
		}
		if links.Parent > 0 {
			if _, ok := byNumber[links.Parent]; !ok {
				diags = append(diags, GraphDiagnostic{Issue: issue.Number, RuleID: "broken_parent", Detail: fmt.Sprintf("parent #%d not found", links.Parent)})
			}
		}
		for _, dep := range links.DependsOn {
			adj[issue.Number] = append(adj[issue.Number], dep)
			depIssue, ok := byNumber[dep]
			if !ok {
				diags = append(diags, GraphDiagnostic{Issue: issue.Number, RuleID: "broken_depends_on", Detail: fmt.Sprintf("depends_on #%d not found", dep)})
				continue
			}
			if strings.EqualFold(issue.State, "open") && strings.EqualFold(depIssue.State, "open") {
				diags = append(diags, GraphDiagnostic{Issue: issue.Number, RuleID: "unresolved_blocker", Detail: fmt.Sprintf("depends_on #%d still open", dep)})
			}
			if hasDoneStatus(issue.Labels) && strings.EqualFold(depIssue.State, "open") && !hasGraphOverride(issue.Body) {
				diags = append(diags, GraphDiagnostic{Issue: issue.Number, RuleID: "done_with_open_dependency", Detail: fmt.Sprintf("done blocked by open dependency #%d", dep)})
			}
		}
		for _, b := range links.Blocks {
			if _, ok := byNumber[b]; !ok {
				diags = append(diags, GraphDiagnostic{Issue: issue.Number, RuleID: "broken_blocks", Detail: fmt.Sprintf("blocks #%d not found", b)})
			}
		}
	}
	for _, cycleNode := range detectCycles(adj) {
		diags = append(diags, GraphDiagnostic{Issue: cycleNode, RuleID: "dependency_cycle", Detail: "cycle detected in depends_on graph"})
	}
	sort.Slice(diags, func(i, j int) bool {
		if diags[i].Issue == diags[j].Issue {
			return diags[i].RuleID < diags[j].RuleID
		}
		return diags[i].Issue < diags[j].Issue
	})
	return GraphValidationReport{Repo: repo, Diagnostics: diags, Counts: GraphCounts{Issues: len(issues), Diagnostics: len(diags)}}
}

func parseIssueLinks(body string) issueLinks {
	out := issueLinks{}
	if m := linkParentRe.FindStringSubmatch(body); len(m) == 2 {
		out.Parent, _ = strconv.Atoi(m[1])
	}
	if m := linkDependsRe.FindStringSubmatch(body); len(m) == 2 {
		out.DependsOn = parseRefs(m[1])
	}
	if m := linkBlocksRe.FindStringSubmatch(body); len(m) == 2 {
		out.Blocks = parseRefs(m[1])
	}
	return out
}

func parseRefs(s string) []int {
	seen := map[int]struct{}{}
	out := []int{}
	for _, m := range refRe.FindAllStringSubmatch(s, -1) {
		n, _ := strconv.Atoi(m[1])
		if n <= 0 {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	sort.Ints(out)
	return out
}

func needsParent(labels []string) bool {
	for _, l := range labels {
		if l == "type:story" || l == "type:task" {
			return true
		}
	}
	return false
}
func hasDoneStatus(labels []string) bool {
	for _, l := range labels {
		if l == "status:done" {
			return true
		}
	}
	return false
}
func hasGraphOverride(body string) bool {
	return strings.Contains(strings.ToLower(body), "graph-override")
}

func detectCycles(adj map[int][]int) []int {
	state := map[int]int{}
	inCycle := map[int]struct{}{}
	var dfs func(int)
	dfs = func(n int) {
		if state[n] == 1 {
			inCycle[n] = struct{}{}
			return
		}
		if state[n] == 2 {
			return
		}
		state[n] = 1
		for _, m := range adj[n] {
			dfs(m)
		}
		state[n] = 2
	}
	for n := range adj {
		dfs(n)
	}
	out := make([]int, 0, len(inCycle))
	for n := range inCycle {
		out = append(out, n)
	}
	sort.Ints(out)
	return out
}
