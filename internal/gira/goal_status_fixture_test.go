package gira

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// goalStatusFixtureRunner supplies the explicit repository-scoped fixtures
// used by the pre-existing goal-status tests. It is intentionally separate
// from onboardFakeRunner so unrelated command tests still fail on unexpected
// calls.
type goalStatusFixtureRunner struct {
	responses map[string]string
	errors    map[string]error
}

func (r goalStatusFixtureRunner) Run(name string, args ...string) ([]byte, error) {
	key := strings.TrimSpace(name + " " + strings.Join(args, " "))
	if response, ok := r.responses[key]; ok {
		return []byte(response), nil
	}
	if strings.HasPrefix(key, "gh api graphql ") {
		return r.graphQLIssueSnapshot(key)
	}
	return onboardFakeRunner{responses: r.responses, errors: r.errors}.Run(name, args...)
}

func (r goalStatusFixtureRunner) graphQLIssueSnapshot(command string) ([]byte, error) {
	owner := commandField(command, "owner")
	name := commandField(command, "name")
	if owner == "" || name == "" {
		return nil, fmt.Errorf("missing GraphQL repository fixture")
	}
	pattern := regexp.MustCompile(`issue([0-9]+): issue\(number: ([0-9]+)\)`)
	query := commandField(command, "query")
	result := struct {
		Data struct {
			Repository map[string]any `json:"repository"`
		} `json:"data"`
	}{}
	result.Data.Repository = map[string]any{}
	for _, match := range pattern.FindAllStringSubmatch(query, -1) {
		alias, number := "issue"+match[1], match[2]
		raw, ok := r.responses["gh api repos/"+owner+"/"+name+"/issues/"+number]
		if !ok {
			continue
		}
		var issue struct {
			Number int     `json:"number"`
			Title  string  `json:"title"`
			State  string  `json:"state"`
			Body   *string `json:"body"`
			Labels []struct {
				Name string `json:"name"`
			} `json:"labels"`
			Milestone *struct {
				Title string `json:"title"`
			} `json:"milestone"`
		}
		if err := json.Unmarshal([]byte(raw), &issue); err != nil {
			return nil, err
		}
		labels := make([]map[string]string, 0, len(issue.Labels))
		for _, label := range issue.Labels {
			labels = append(labels, map[string]string{"name": label.Name})
		}
		issuePayload := map[string]any{
			"number": issue.Number, "title": issue.Title, "state": issue.State,
			"body": issue.Body, "labels": map[string]any{"nodes": labels}, "milestone": issue.Milestone,
			"timelineItems": map[string]any{"nodes": []any{}},
		}
		prKeyPrefix := "gh pr list --repo " + owner + "/" + name + " --state all --search repo:" + owner + "/" + name + " is:pr " + number + " "
		for key, prJSON := range r.responses {
			if !strings.HasPrefix(key, prKeyPrefix) {
				continue
			}
			var prs []prSummary
			if json.Unmarshal([]byte(prJSON), &prs) != nil || len(prs) == 0 {
				continue
			}
			pr := prs[0]
			contexts := make([]map[string]any, 0, len(pr.StatusRollup))
			for _, check := range pr.StatusRollup {
				contexts = append(contexts, map[string]any{"name": check.Name, "status": check.Status, "conclusion": check.Conclusion, "detailsUrl": check.URL})
			}
			prPayload := map[string]any{
				"number": pr.Number, "title": pr.Title, "body": pr.Body, "state": pr.State,
				"url": pr.URL, "isDraft": pr.IsDraft, "mergeStateStatus": pr.MergeState,
				"reviewDecision": pr.ReviewDecision, "headRefName": pr.HeadRefName,
				"baseRefName": pr.BaseRefName, "headRefOid": pr.HeadRefOID,
				"statusCheckRollup": map[string]any{"contexts": map[string]any{"nodes": contexts}},
			}
			if pr.MergeCommit != nil {
				prPayload["mergeCommit"] = map[string]any{"oid": pr.MergeCommit.OID}
			}
			issuePayload["timelineItems"] = map[string]any{"nodes": []any{map[string]any{"source": prPayload}}}
			break
		}
		result.Data.Repository[alias] = issuePayload
	}
	return json.Marshal(result)
}

func commandField(command string, field string) string {
	prefix := "-f " + field + "="
	start := strings.Index(command, prefix)
	if start < 0 {
		return ""
	}
	value := command[start+len(prefix):]
	if field == "query" {
		return value
	}
	if end := strings.IndexByte(value, ' '); end >= 0 {
		value = value[:end]
	}
	return value
}
