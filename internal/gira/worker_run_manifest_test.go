package gira

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestWorkerRunFixtureOutputsContract(t *testing.T) {
	data, err := os.ReadFile("testdata/worker_run/gira_686_worker_run.json")
	if err != nil {
		t.Fatalf("read worker run fixture: %v", err)
	}

	var manifest struct {
		SchemaVersion string `json:"schema_version"`
		Outputs       struct {
			Commits      []map[string]any `json:"commits"`
			PullRequests []map[string]any `json:"pull_requests"`
			Comments     []map[string]any `json:"comments"`
			Checks       []map[string]any `json:"checks"`
		} `json:"outputs"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("fixture is not valid JSON: %v", err)
	}
	if manifest.SchemaVersion != "worker-run/v1" {
		t.Fatalf("schema_version = %q, want worker-run/v1", manifest.SchemaVersion)
	}

	requireNonEmptyWorkerRunOutputs(t, "commits", manifest.Outputs.Commits)
	requireNonEmptyWorkerRunOutputs(t, "pull_requests", manifest.Outputs.PullRequests)
	requireNonEmptyWorkerRunOutputs(t, "comments", manifest.Outputs.Comments)
	requireNonEmptyWorkerRunOutputs(t, "checks", manifest.Outputs.Checks)

	for i, item := range manifest.Outputs.Commits {
		validateWorkerRunCommitOutput(t, fmt.Sprintf("commits[%d]", i), item)
	}
	for i, item := range manifest.Outputs.PullRequests {
		validateWorkerRunPullRequestOutput(t, fmt.Sprintf("pull_requests[%d]", i), item)
	}
	for i, item := range manifest.Outputs.Comments {
		validateWorkerRunCommentOutput(t, fmt.Sprintf("comments[%d]", i), item)
	}
	for i, item := range manifest.Outputs.Checks {
		validateWorkerRunCheckOutput(t, fmt.Sprintf("checks[%d]", i), item)
	}
}

func validateWorkerRunCommitOutput(t *testing.T, path string, item map[string]any) {
	t.Helper()
	requireWorkerRunKeys(t, path, item, "sha", "repo", "branch", "subject", "author", "authored_at", "committer", "committed_at", "url", "source")

	sha := requireWorkerRunString(t, path, item, "sha")
	if len(sha) != 40 {
		t.Fatalf("%s.sha length = %d, want 40", path, len(sha))
	}
	for _, r := range sha {
		if !strings.ContainsRune("0123456789abcdef", r) {
			t.Fatalf("%s.sha contains non-lowercase-hex character %q", path, r)
		}
	}
	requireWorkerRunString(t, path, item, "repo")
	requireWorkerRunNullableString(t, path, item, "branch")
	requireWorkerRunString(t, path, item, "subject")
	validateWorkerRunActor(t, path+".author", requireWorkerRunObject(t, path, item, "author"))
	validateWorkerRunActor(t, path+".committer", requireWorkerRunObject(t, path, item, "committer"))
	validateWorkerRunNullableTimestamp(t, path, item, "authored_at")
	validateWorkerRunNullableTimestamp(t, path, item, "committed_at")
	commitURL := requireWorkerRunNullableString(t, path, item, "url")
	if commitURL != nil {
		validateWorkerRunGitHubURL(t, path+".url", *commitURL)
		if !strings.Contains(*commitURL, "/commit/"+sha) {
			t.Fatalf("%s.url = %q, want URL for commit %s", path, *commitURL, sha)
		}
	}
	requireWorkerRunEnum(t, path, "source", requireWorkerRunString(t, path, item, "source"), "git_local", "github", "gira_export", "worker_manifest", "unknown")
}

func validateWorkerRunPullRequestOutput(t *testing.T, path string, item map[string]any) {
	t.Helper()
	requireWorkerRunKeys(t, path, item, "repo", "number", "url", "state", "draft", "title", "author", "created_at", "updated_at", "merged_at", "closed_at", "head_branch", "base_branch", "closing_refs", "source")

	requireWorkerRunString(t, path, item, "repo")
	requirePositiveWorkerRunNumber(t, path, item, "number")
	validateWorkerRunGitHubURL(t, path+".url", requireWorkerRunString(t, path, item, "url"))
	requireWorkerRunEnum(t, path, "state", requireWorkerRunString(t, path, item, "state"), "open", "closed", "merged", "unknown")
	requireWorkerRunBool(t, path, item, "draft")
	requireWorkerRunString(t, path, item, "title")
	validateWorkerRunActor(t, path+".author", requireWorkerRunObject(t, path, item, "author"))
	validateWorkerRunNullableTimestamp(t, path, item, "created_at")
	validateWorkerRunNullableTimestamp(t, path, item, "updated_at")
	validateWorkerRunNullableTimestamp(t, path, item, "merged_at")
	validateWorkerRunNullableTimestamp(t, path, item, "closed_at")
	requireWorkerRunNullableString(t, path, item, "head_branch")
	requireWorkerRunNullableString(t, path, item, "base_branch")
	requireWorkerRunEnum(t, path, "source", requireWorkerRunString(t, path, item, "source"), "gira_ticket_pr", "github", "worker", "gira_export", "unknown")

	closingRefs := requireWorkerRunArray(t, path, item, "closing_refs")
	for i, ref := range closingRefs {
		refPath := fmt.Sprintf("%s.closing_refs[%d]", path, i)
		refObject, ok := ref.(map[string]any)
		if !ok {
			t.Fatalf("%s must be an object", refPath)
		}
		requireWorkerRunKeys(t, refPath, refObject, "repo", "number", "url", "keyword")
		requireWorkerRunString(t, refPath, refObject, "repo")
		requirePositiveWorkerRunNumber(t, refPath, refObject, "number")
		validateWorkerRunGitHubURL(t, refPath+".url", requireWorkerRunString(t, refPath, refObject, "url"))
		requireWorkerRunNullableString(t, refPath, refObject, "keyword")
	}
}

func validateWorkerRunCommentOutput(t *testing.T, path string, item map[string]any) {
	t.Helper()
	requireWorkerRunKeys(t, path, item, "kind", "repo", "target_type", "target_number", "url", "author", "source", "created_at", "updated_at", "purpose")

	requireWorkerRunEnum(t, path, "kind", requireWorkerRunString(t, path, item, "kind"), "issue_comment", "pr_comment", "pr_review", "commit_comment", "check_run_annotation")
	requireWorkerRunString(t, path, item, "repo")
	requireWorkerRunEnum(t, path, "target_type", requireWorkerRunString(t, path, item, "target_type"), "issue", "pull_request", "commit", "check_run")
	requirePositiveWorkerRunNumber(t, path, item, "target_number")
	validateWorkerRunGitHubURL(t, path+".url", requireWorkerRunString(t, path, item, "url"))
	validateWorkerRunActor(t, path+".author", requireWorkerRunObject(t, path, item, "author"))
	requireWorkerRunEnum(t, path, "source", requireWorkerRunString(t, path, item, "source"), "worker", "gira_lifecycle", "gira_self_review", "reviewer", "operator", "github", "unknown")
	validateWorkerRunNullableTimestamp(t, path, item, "created_at")
	validateWorkerRunNullableTimestamp(t, path, item, "updated_at")
	requireWorkerRunString(t, path, item, "purpose")
}

func validateWorkerRunCheckOutput(t *testing.T, path string, item map[string]any) {
	t.Helper()
	requireWorkerRunKeys(t, path, item, "repo", "name", "workflow", "status", "conclusion", "url", "source", "started_at", "completed_at")

	requireWorkerRunString(t, path, item, "repo")
	requireWorkerRunString(t, path, item, "name")
	requireWorkerRunNullableString(t, path, item, "workflow")
	status := requireWorkerRunString(t, path, item, "status")
	requireWorkerRunEnum(t, path, "status", status, "queued", "in_progress", "completed", "waiting", "requested", "pending", "unknown")
	conclusion := requireWorkerRunNullableString(t, path, item, "conclusion")
	if conclusion != nil {
		requireWorkerRunEnum(t, path, "conclusion", *conclusion, "success", "failure", "neutral", "cancelled", "skipped", "timed_out", "action_required", "startup_failure", "stale")
	}
	if status != "completed" && conclusion != nil {
		t.Fatalf("%s.conclusion must be null when status is %q", path, status)
	}
	checkURL := requireWorkerRunNullableString(t, path, item, "url")
	if checkURL != nil {
		validateWorkerRunGitHubURL(t, path+".url", *checkURL)
	}
	requireWorkerRunEnum(t, path, "source", requireWorkerRunString(t, path, item, "source"), "github_actions", "github_check_run", "external_ci", "gira_export", "unknown")
	validateWorkerRunNullableTimestamp(t, path, item, "started_at")
	validateWorkerRunNullableTimestamp(t, path, item, "completed_at")
}

func validateWorkerRunActor(t *testing.T, path string, actor map[string]any) {
	t.Helper()
	requireWorkerRunKeys(t, path, actor, "type", "login")
	requireWorkerRunEnum(t, path, "type", requireWorkerRunString(t, path, actor, "type"), "github_user", "github_bot", "gira", "worker", "operator", "unknown")
	requireWorkerRunNullableString(t, path, actor, "login")
}

func requireNonEmptyWorkerRunOutputs(t *testing.T, name string, values []map[string]any) {
	t.Helper()
	if len(values) == 0 {
		t.Fatalf("outputs.%s must contain at least one representative item", name)
	}
}

func requireWorkerRunKeys(t *testing.T, path string, item map[string]any, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if _, ok := item[key]; !ok {
			t.Fatalf("%s missing required key %q", path, key)
		}
	}
}

func requireWorkerRunString(t *testing.T, path string, item map[string]any, key string) string {
	t.Helper()
	value, ok := item[key]
	if !ok {
		t.Fatalf("%s missing required key %q", path, key)
	}
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		t.Fatalf("%s.%s must be a non-empty string", path, key)
	}
	return text
}

func requireWorkerRunNullableString(t *testing.T, path string, item map[string]any, key string) *string {
	t.Helper()
	value, ok := item[key]
	if !ok {
		t.Fatalf("%s missing required nullable key %q", path, key)
	}
	if value == nil {
		return nil
	}
	text, ok := value.(string)
	if !ok {
		t.Fatalf("%s.%s must be a string or null", path, key)
	}
	return &text
}

func requireWorkerRunObject(t *testing.T, path string, item map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := item[key]
	if !ok {
		t.Fatalf("%s missing required key %q", path, key)
	}
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s.%s must be an object", path, key)
	}
	return object
}

func requireWorkerRunArray(t *testing.T, path string, item map[string]any, key string) []any {
	t.Helper()
	value, ok := item[key]
	if !ok {
		t.Fatalf("%s missing required key %q", path, key)
	}
	array, ok := value.([]any)
	if !ok {
		t.Fatalf("%s.%s must be an array", path, key)
	}
	return array
}

func requirePositiveWorkerRunNumber(t *testing.T, path string, item map[string]any, key string) {
	t.Helper()
	value, ok := item[key]
	if !ok {
		t.Fatalf("%s missing required key %q", path, key)
	}
	number, ok := value.(float64)
	if !ok || number < 1 || number != float64(int(number)) {
		t.Fatalf("%s.%s must be a positive integer", path, key)
	}
}

func requireWorkerRunBool(t *testing.T, path string, item map[string]any, key string) {
	t.Helper()
	value, ok := item[key]
	if !ok {
		t.Fatalf("%s missing required key %q", path, key)
	}
	if _, ok := value.(bool); !ok {
		t.Fatalf("%s.%s must be a boolean", path, key)
	}
}

func validateWorkerRunNullableTimestamp(t *testing.T, path string, item map[string]any, key string) {
	t.Helper()
	value := requireWorkerRunNullableString(t, path, item, key)
	if value == nil {
		return
	}
	if _, err := time.Parse(time.RFC3339, *value); err != nil {
		t.Fatalf("%s.%s must be RFC3339: %v", path, key, err)
	}
}

func validateWorkerRunGitHubURL(t *testing.T, path string, value string) {
	t.Helper()
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatalf("%s must parse as URL: %v", path, err)
	}
	if parsed.Scheme != "https" || parsed.Host != "github.com" {
		t.Fatalf("%s = %q, want absolute https://github.com URL", path, value)
	}
}

func requireWorkerRunEnum(t *testing.T, path string, field string, value string, allowed ...string) {
	t.Helper()
	for _, candidate := range allowed {
		if value == candidate {
			return
		}
	}
	t.Fatalf("%s.%s = %q, want one of %s", path, field, value, strings.Join(allowed, ", "))
}
