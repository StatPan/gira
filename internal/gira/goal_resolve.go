package gira

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type GoalCandidate struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state,omitempty"`
	URL    string `json:"url,omitempty"`
	Source string `json:"source,omitempty"`
}

func ResolveGoalNumber(repo RepoRef, explicitGoal int, runner CommandRunner) (int, []GoalCandidate, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if explicitGoal > 0 {
		return explicitGoal, nil, nil
	}
	if branchIssue := inferIssueNumberFromLocalContext(repo, runner); branchIssue > 0 {
		issue, err := fetchDevIssue(repo, branchIssue, runner)
		if err == nil {
			if isGoalIssueLabels(issue.Labels) {
				return issue.Number, []GoalCandidate{goalCandidateFromDevIssue(issue, "current_issue")}, nil
			}
			if parent := parentGoalRefFromBody(repo, issue.Body); parent.Number > 0 && parent.Repo.FullName() == repo.FullName() {
				parentIssue, parentErr := fetchDevIssue(repo, parent.Number, runner)
				if parentErr == nil && isGoalIssueLabels(parentIssue.Labels) {
					return parentIssue.Number, []GoalCandidate{goalCandidateFromDevIssue(parentIssue, "parent_goal")}, nil
				}
			}
		}
	}
	openGoals, err := fetchOpenGoalCandidates(repo, runner)
	if err != nil {
		return 0, nil, err
	}
	if len(openGoals) == 1 {
		return openGoals[0].Number, openGoals, nil
	}
	if len(openGoals) == 0 {
		return 0, nil, fmt.Errorf("goal context unavailable: no open goal or epic matched; pass --goal N, run from a goal/child branch, or create a goal with `gira goal new --dry-run`")
	}
	return 0, openGoals, fmt.Errorf("goal context ambiguous: pass --goal N or run from a goal/child branch; candidates: %s", FormatGoalCandidates(openGoals))
}

type parentGoalRef struct {
	Repo   RepoRef
	Number int
}

var parentGoalRefPattern = regexp.MustCompile(`(?i)\bparent\s*:\s*(?:([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+))?#([1-9][0-9]*)`)

func parentGoalRefFromBody(defaultRepo RepoRef, body string) parentGoalRef {
	match := parentGoalRefPattern.FindStringSubmatch(strings.TrimSpace(body))
	if len(match) == 0 {
		return parentGoalRef{}
	}
	repo := defaultRepo
	if strings.TrimSpace(match[1]) != "" {
		parsed, err := ParseRepoRef(match[1])
		if err != nil {
			return parentGoalRef{}
		}
		repo = parsed
	}
	number, err := strconv.Atoi(match[2])
	if err != nil || number <= 0 {
		return parentGoalRef{}
	}
	return parentGoalRef{Repo: repo, Number: number}
}

func fetchOpenGoalCandidates(repo RepoRef, runner CommandRunner) ([]GoalCandidate, error) {
	issues, err := fetchEpicIssues(repo, "open", runner)
	if err != nil {
		return nil, err
	}
	out := []GoalCandidate{}
	for _, issue := range issues {
		labels := rawLabels(issue)
		if !isGoalIssueLabels(labels) {
			continue
		}
		out = append(out, GoalCandidate{Number: issue.Number, Title: issue.Title, State: strings.ToLower(issue.State), URL: issue.HTMLURL, Source: "open_goal"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out, nil
}

func isGoalIssueLabels(labels []string) bool {
	return hasLabel(labels, "type:goal") || hasLabel(labels, "type:epic")
}

func goalCandidateFromDevIssue(issue devStartIssue, source string) GoalCandidate {
	return GoalCandidate{
		Number: issue.Number,
		Title:  issue.Title,
		State:  strings.ToLower(issue.State),
		URL:    "",
		Source: source,
	}
}

func FormatGoalCandidates(candidates []GoalCandidate) string {
	if len(candidates) == 0 {
		return ""
	}
	parts := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		label := fmt.Sprintf("#%d %s", candidate.Number, strings.TrimSpace(candidate.Title))
		if strings.TrimSpace(candidate.Source) != "" {
			label += " (" + strings.TrimSpace(candidate.Source) + ")"
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, ", ")
}
