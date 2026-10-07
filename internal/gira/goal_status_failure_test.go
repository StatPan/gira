package gira

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestGoalStatusRunnerFailureClassifiesKnownGraphQLSyntaxWithoutLeakingError(t *testing.T) {
	err := goalStatusRunnerFailure(goalStatusFailureStageSnapshotTransport, fmt.Errorf(`gh api failed: Expected NAME, actual: (none) ("") at [1, 13207]; GH_TOKEN=ghp_private`))
	stage, code := goalStatusFailureClass(err)
	if stage != goalStatusFailureStageSnapshotGraphQL || code != "query_syntax" {
		t.Fatalf("failure class = %s/%s, want snapshot_graphql/query_syntax", stage, code)
	}
	serialized, marshalErr := json.Marshal(goalStatusFailure("StatPan/gira", "repository_snapshot", stage, code, []goalChildRef{{Repo: RepoRef{Owner: "StatPan", Name: "gira"}, Number: 655}}, true))
	if marshalErr != nil {
		t.Fatalf("marshal safe failure: %v", marshalErr)
	}
	if strings.Contains(string(serialized), "ghp_private") || strings.Contains(string(serialized), "13207") || strings.Contains(err.Error(), "GH_TOKEN") {
		t.Fatalf("failure output exposed provider details: %s / %s", err, serialized)
	}
}

func TestGoalStatusFailureClassDefaultsUnknownErrorsToSanitizedStatusFailure(t *testing.T) {
	err := fmt.Errorf("query body and authorization token should never appear in JSON")
	stage, code := goalStatusFailureClass(err)
	if stage != goalStatusFailureStageChildStatus || code != "status_derivation_failed" {
		t.Fatalf("unknown error class = %s/%s", stage, code)
	}
	failure := goalStatusFailure("StatPan/gira", "child_status", stage, code, []goalChildRef{{Repo: RepoRef{Owner: "StatPan", Name: "gira"}, Number: 101}}, true)
	serialized, marshalErr := json.Marshal(failure)
	if marshalErr != nil {
		t.Fatalf("marshal failure: %v", marshalErr)
	}
	if strings.Contains(string(serialized), "authorization token") || strings.Contains(string(serialized), "query body") {
		t.Fatalf("raw error leaked: %s", serialized)
	}
}
