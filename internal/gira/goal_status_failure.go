package gira

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	goalStatusFailureStageChildDiscovery     = "child_discovery"
	goalStatusFailureStageSnapshotTransport  = "snapshot_transport"
	goalStatusFailureStageSnapshotParse      = "snapshot_parse"
	goalStatusFailureStageSnapshotGraphQL    = "snapshot_graphql"
	goalStatusFailureStageSnapshotIncomplete = "snapshot_incomplete"
	goalStatusFailureStageSnapshotLimit      = "snapshot_limit"
	goalStatusFailureStageOperationPolicy    = "operation_policy"
	goalStatusFailureStageChildStatus        = "child_status"
)

type GoalStatusFailure struct {
	Repository                string                    `json:"repository"`
	Source                    string                    `json:"source"`
	Stage                     string                    `json:"stage"`
	Code                      string                    `json:"code"`
	AffectedCount             int                       `json:"affected_count"`
	AffectedCountComplete     bool                      `json:"affected_count_complete"`
	AffectedChildren          []GoalStatusChildIdentity `json:"affected_children,omitempty"`
	AffectedChildrenTruncated bool                      `json:"affected_children_truncated,omitempty"`
}

type GoalStatusChildIdentity struct {
	Repository string `json:"repository"`
	Number     int    `json:"number"`
	URL        string `json:"url"`
}

type goalStatusAcquisitionError struct {
	stage string
	code  string
	cause error
}

func (e *goalStatusAcquisitionError) Error() string {
	return "goal status acquisition failed at " + e.stage + "/" + e.code
}

func (e *goalStatusAcquisitionError) Unwrap() error { return e.cause }

func newGoalStatusAcquisitionError(stage, code string, cause error) error {
	return &goalStatusAcquisitionError{stage: stage, code: code, cause: cause}
}

func goalStatusFailureForError(repo, source string, refs []goalChildRef, err error) GoalStatusFailure {
	stage, code := goalStatusFailureClass(err)
	return goalStatusFailure(repo, source, stage, code, refs, true)
}

func goalStatusFailure(repo, source, stage, code string, refs []goalChildRef, countComplete bool) GoalStatusFailure {
	failure := GoalStatusFailure{
		Repository:            repo,
		Source:                source,
		Stage:                 stage,
		Code:                  code,
		AffectedCount:         len(refs),
		AffectedCountComplete: countComplete,
	}
	limit := goalStatusRepositoryChildLimit
	for _, ref := range refs {
		if len(failure.AffectedChildren) == limit {
			failure.AffectedChildrenTruncated = true
			break
		}
		failure.AffectedChildren = append(failure.AffectedChildren, GoalStatusChildIdentity{
			Repository: ref.Repo.FullName(),
			Number:     ref.Number,
			URL:        githubIssueURL(ref.Repo, ref.Number),
		})
	}
	return failure
}

func goalStatusFailureClass(err error) (string, string) {
	if err == nil {
		return goalStatusFailureStageChildStatus, "status_derivation_failed"
	}
	var acquisition *goalStatusAcquisitionError
	if asGoalStatusAcquisitionError(err, &acquisition) {
		return acquisition.stage, acquisition.code
	}
	return goalStatusFailureStageChildStatus, "status_derivation_failed"
}

func asGoalStatusAcquisitionError(err error, target **goalStatusAcquisitionError) bool {
	for err != nil {
		if acquisition, ok := err.(*goalStatusAcquisitionError); ok {
			*target = acquisition
			return true
		}
		err = unwrapGoalStatusError(err)
	}
	return false
}

func unwrapGoalStatusError(err error) error {
	type wrapped interface{ Unwrap() error }
	if current, ok := err.(wrapped); ok {
		return current.Unwrap()
	}
	return nil
}

func goalStatusRunnerFailure(stage string, err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "expected name, actual:") {
		return newGoalStatusAcquisitionError(goalStatusFailureStageSnapshotGraphQL, "query_syntax", err)
	}
	return newGoalStatusAcquisitionError(stage, "transport_unknown", err)
}

func goalStatusGraphQLFailure(message string) error {
	code := "graphql_error"
	lower := strings.ToLower(message)
	if strings.Contains(lower, "expected name, actual:") {
		code = "query_syntax"
	}
	return newGoalStatusAcquisitionError(goalStatusFailureStageSnapshotGraphQL, code, nil)
}

func goalStatusDiscoveryFailureCode(err error) (string, string) {
	var acquisition *goalStatusAcquisitionError
	if asGoalStatusAcquisitionError(err, &acquisition) {
		switch acquisition.code {
		case "invalid_json", "source_limit", "transport_unknown", "source_unavailable":
			return goalStatusFailureStageChildDiscovery, acquisition.code
		}
		return goalStatusFailureStageChildDiscovery, "transport_unknown"
	}
	return goalStatusFailureStageChildDiscovery, "source_unavailable"
}

func goalStatusIdentityRefs(refs []goalChildRef) []GoalStatusChildIdentity {
	out := make([]GoalStatusChildIdentity, 0, len(refs))
	for _, ref := range refs {
		out = append(out, GoalStatusChildIdentity{
			Repository: ref.Repo.FullName(),
			Number:     ref.Number,
			URL:        githubIssueURL(ref.Repo, ref.Number),
		})
	}
	return out
}

func goalStatusRefsForRepo(refs []goalChildRef, repo string) []goalChildRef {
	out := make([]goalChildRef, 0, len(refs))
	for _, ref := range refs {
		if ref.Repo.FullName() == repo {
			out = append(out, ref)
		}
	}
	return out
}

func goalStatusUnknownChildIdentities(children []GoalStatusChild) []GoalStatusChildIdentity {
	out := []GoalStatusChildIdentity{}
	for _, child := range children {
		if child.StatusAvailable && goalStatusKnownCategory(child.Category) {
			continue
		}
		out = append(out, GoalStatusChildIdentity{Repository: child.Repo, Number: child.Number, URL: child.URL})
	}
	return out
}

func goalStatusSortFailures(failures []GoalStatusFailure) {
	sort.Slice(failures, func(i, j int) bool {
		if failures[i].Repository != failures[j].Repository {
			return failures[i].Repository < failures[j].Repository
		}
		if failures[i].Source != failures[j].Source {
			return failures[i].Source < failures[j].Source
		}
		if failures[i].Stage != failures[j].Stage {
			return failures[i].Stage < failures[j].Stage
		}
		return failures[i].Code < failures[j].Code
	})
}

func goalStatusUnavailableChild(ref goalChildRef, descriptor *devStartIssue) GoalStatusChild {
	child := GoalStatusChild{
		Repo:            ref.Repo.FullName(),
		Number:          ref.Number,
		State:           "unknown",
		Status:          "unknown",
		Category:        "unknown",
		RelationSource:  ref.RelationSource,
		StatusAvailable: false,
		NextAction:      "inspect_child",
		NextStep:        fmt.Sprintf("gira ticket status --repo %s --ticket %d --json", ref.Repo.FullName(), ref.Number),
		URL:             githubIssueURL(ref.Repo, ref.Number),
		Blockers:        []string{"status_unavailable"},
	}
	if descriptor != nil {
		child.Title = descriptor.Title
		child.State = descriptor.State
		child.Labels = append([]string(nil), descriptor.Labels...)
	}
	return child
}

func goalStatusFailureSummary(failure GoalStatusFailure) string {
	return failure.Repository + ":" + failure.Stage + ":" + failure.Code + ":" + strconv.Itoa(failure.AffectedCount)
}
