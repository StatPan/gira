package gira

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type goalStatusCountingRunner struct {
	mu        sync.Mutex
	responses map[string]string
	errors    map[string]error
	graphql   map[string]string
	calls     []string
}

func (r *goalStatusCountingRunner) Run(name string, args ...string) ([]byte, error) {
	key := strings.TrimSpace(strings.Join(append([]string{name}, args...), " "))
	r.mu.Lock()
	r.calls = append(r.calls, key)
	response, ok := r.responses[key]
	runErr := r.errors[key]
	if !ok && strings.HasPrefix(key, "gh api graphql ") {
		owner := commandFieldForCountingRunner(key, "owner")
		name := commandFieldForCountingRunner(key, "name")
		response, ok = r.graphql[owner+"/"+name]
		if !ok {
			response, ok = r.responses["gh api graphql"]
		}
	}
	r.mu.Unlock()
	if runErr != nil {
		return nil, runErr
	}
	if !ok {
		if strings.HasPrefix(key, "gh api repos/") && strings.Contains(key, "/issues/") && strings.Contains(key, "/sub_issues -X GET ") {
			return []byte(`[]`), nil
		}
		return nil, fmt.Errorf("unexpected command: %s", key)
	}
	return []byte(response), nil
}

func commandFieldForCountingRunner(command string, field string) string {
	prefix := "-f " + field + "="
	start := strings.Index(command, prefix)
	if start < 0 {
		return ""
	}
	value := command[start+len(prefix):]
	if end := strings.IndexByte(value, ' '); end >= 0 {
		value = value[:end]
	}
	return value
}

func (r *goalStatusCountingRunner) countPrefix(prefix string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, call := range r.calls {
		if strings.HasPrefix(call, prefix) {
			count++
		}
	}
	return count
}

func (r *goalStatusCountingRunner) countExact(command string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, call := range r.calls {
		if call == command {
			count++
		}
	}
	return count
}

func TestBuildGoalStatusReportBatchesSameRepositoryReads(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	children := make([]string, 0, 7)
	for number := 101; number <= 107; number++ {
		children = append(children, fmt.Sprintf("<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=%d -->", number))
	}
	responses := map[string]string{
		"gh api repos/StatPan/gira/issues/100":                  `{"number":100,"title":"Goal","state":"open","body":"` + strings.Join(children, "\\n") + `","labels":[{"name":"type:epic"},{"name":"status:ready"}]}`,
		"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[]}`,
	}
	responses["gh api graphql"] = goalStatusGraphQLFixtureWithPRs(101, 107)
	runner := &goalStatusCountingRunner{responses: responses}

	report, err := BuildGoalStatusReport(GoalStatusInput{Repo: repo, Goal: 100}, runner)
	if err != nil {
		t.Fatalf("BuildGoalStatusReport error: %v", err)
	}
	if len(report.Children) != 7 || report.Counts["done"] != 7 || report.SchemaVersion != GoalStatusSchemaVersion {
		t.Fatalf("unexpected batched report: %+v", report)
	}
	for index, child := range report.Children {
		want := 101 + index
		if child.Number != want || child.PRNumber != want+100 || child.Category != "done" || child.RelationSource != GoalChildRelationSourceGiraGoalChildLink {
			t.Fatalf("child %d = %+v", index, child)
		}
	}
	if got := runner.countExact("gh api repos/StatPan/gira/issues/100"); got != 1 {
		t.Fatalf("goal issue API calls = %d, want 1", got)
	}
	for _, prefix := range []string{
		"gh api repos/StatPan/gira/issues/101/timeline",
		"gh api repos/StatPan/gira/issues/102/timeline",
		"gh api repos/StatPan/gira/pulls/201/reviews",
		"gh api repos/StatPan/gira/commits/sha-101/check-runs",
	} {
		if got := runner.countPrefix(prefix); got != 0 {
			t.Fatalf("unexpected per-child call %q: %d", prefix, got)
		}
	}
	if got := runner.countPrefix("gh api graphql"); got != 1 {
		t.Fatalf("issue snapshot calls = %d, want 1", got)
	}
	if got := runner.countPrefix("gh pr list --repo StatPan/gira --state all"); got != 0 {
		t.Fatalf("repository-wide PR snapshot calls = %d, want 0", got)
	}
	runner.mu.Lock()
	defer runner.mu.Unlock()
	for _, call := range runner.calls {
		if strings.Contains(call, "--paginate") {
			t.Fatalf("batched status must not use unbounded pagination: %s", call)
		}
	}
}

func TestGoalStatusBatchPreservesUnknownChecksAsBlocker(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	responses := map[string]string{
		"gh api repos/StatPan/gira/issues/100":                                           `{"number":100,"title":"Goal","state":"open","body":"<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=101 -->","labels":[{"name":"type:epic"}]}`,
		"gh issue view 100 --repo StatPan/gira --json comments":                          `{"comments":[]}`,
		"gh api repos/StatPan/gira/issues -X GET -f state=all -f per_page=100 -f page=1": `[{"number":101,"title":"Child","state":"open","body":"Work","labels":[{"name":"type:task"},{"name":"status:in-progress"}]}]`,
	}
	responses["gh api graphql"] = goalStatusGraphQLFixtureWithUnknownPR(101, 201)
	runner := &goalStatusCountingRunner{responses: responses}

	report, err := BuildGoalStatusReport(GoalStatusInput{Repo: repo, Goal: 100}, runner)
	if err != nil {
		t.Fatalf("BuildGoalStatusReport error: %v", err)
	}
	if len(report.Children) != 1 || !containsString(report.Children[0].Blockers, "checks") || report.Children[0].ChecksStatus != "unknown" {
		t.Fatalf("unknown checks must remain blocking/unknown: %+v", report.Children)
	}
}

func TestBuildGoalStatusReportGroupsCrossRepositoryReads(t *testing.T) {
	parent := RepoRef{Owner: "StatPan", Name: "backlog"}
	gira := RepoRef{Owner: "StatPan", Name: "gira"}
	agentree := RepoRef{Owner: "StatPan", Name: "agentree"}
	responses := map[string]string{
		"gh api repos/StatPan/backlog/issues/100":                  `{"number":100,"title":"Goal","state":"open","body":"<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=201 -->\n<!-- gira:goal-child-link/v1 repo=StatPan/agentree issue=301 -->","labels":[{"name":"type:epic"}]}`,
		"gh issue view 100 --repo StatPan/backlog --json comments": `{"comments":[]}`,
	}
	runner := &goalStatusCountingRunner{
		responses: responses,
		graphql: map[string]string{
			gira.FullName():     goalStatusGraphQLFixture(201, 201),
			agentree.FullName(): goalStatusGraphQLFixture(301, 301),
		},
	}
	report, err := BuildGoalStatusReport(GoalStatusInput{Repo: parent, Goal: 100}, runner)
	if err != nil {
		t.Fatalf("BuildGoalStatusReport error: %v", err)
	}
	if len(report.Children) != 2 || report.Children[0].Repo != agentree.FullName() || report.Children[1].Repo != gira.FullName() {
		t.Fatalf("cross-repository children = %+v", report.Children)
	}
	if got := runner.countPrefix("gh api graphql"); got != 2 {
		t.Fatalf("GraphQL snapshots = %d, want one per repository", got)
	}
	if got := runner.countPrefix("gh pr list --repo StatPan/gira --state all"); got != 0 {
		t.Fatalf("gira repository-wide PR snapshots = %d, want 0", got)
	}
	if got := runner.countPrefix("gh pr list --repo StatPan/agentree --state all"); got != 0 {
		t.Fatalf("agentree repository-wide PR snapshots = %d, want 0", got)
	}
}

func TestBuildGoalStatusReportFailsClosedOnPartialRepositorySnapshot(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &goalStatusCountingRunner{
		responses: map[string]string{
			"gh api repos/StatPan/gira/issues/100":                  `{"number":100,"title":"Goal","state":"open","body":"<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=201 -->\n<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=202 -->","labels":[{"name":"type:epic"}]}`,
			"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[]}`,
		},
		graphql: map[string]string{repo.FullName(): goalStatusGraphQLFixture(201, 201)},
	}

	report, err := BuildGoalStatusReport(GoalStatusInput{Repo: repo, Goal: 100}, runner)
	if err != nil {
		t.Fatalf("BuildGoalStatusReport error: %v", err)
	}
	if len(report.Children) != 2 || report.Counts["total"] != 2 || report.Counts["unknown"] != 2 || report.RemainingAutonomousWork != nil || report.NextAction != "resolve_blockers" || !containsString(report.Blockers, "child_201_status_unavailable") || !containsString(report.Blockers, "child_202_status_unavailable") {
		t.Fatalf("partial snapshot must preserve every child identity and fail closed: %+v", report)
	}
	for _, child := range report.Children {
		if child.StatusAvailable || child.Category != "unknown" || child.URL == "" {
			t.Fatalf("partial snapshot fabricated child status or lost identity: %+v", child)
		}
	}
	if got := runner.countPrefix("gh api repos/StatPan/gira/issues/201"); got != 0 {
		t.Fatalf("partial snapshot re-entered per-child issue reads: %d", got)
	}
}

func TestBuildGoalStatusReportFailsClosedOnGraphQLProviderError(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &goalStatusCountingRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/100":                  `{"number":100,"title":"Goal","state":"open","body":"<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=201 -->","labels":[{"name":"type:epic"}]}`,
		"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[]}`,
	}}
	report, err := BuildGoalStatusReport(GoalStatusInput{Repo: repo, Goal: 100}, runner)
	if err != nil {
		t.Fatalf("BuildGoalStatusReport error: %v", err)
	}
	if len(report.Children) != 1 || report.Counts["total"] != 1 || report.Counts["unknown"] != 1 || report.RemainingAutonomousWork != nil || report.NextAction != "resolve_blockers" || !containsString(report.Blockers, "child_201_status_unavailable") {
		t.Fatalf("provider error must preserve identity and remain unavailable: %+v", report)
	}
	if got := runner.countPrefix("gh api repos/StatPan/gira/issues/201"); got != 0 {
		t.Fatalf("provider error re-entered per-child reads: %d", got)
	}
}

func TestGoalStatusPreservesAllDiscoveredChildrenOnSnapshotFailure(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	markers := make([]string, 0, 18)
	for _, number := range []int{651, 652, 656, 657, 658, 659, 673, 677, 680, 683, 687, 688, 689, 693, 696, 699, 700, 703} {
		markers = append(markers, fmt.Sprintf("<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=%d -->", number))
	}
	runner := &goalStatusCountingRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/100":                  `{"number":100,"title":"Goal","state":"open","body":"` + strings.Join(markers, `\n`) + `","labels":[{"name":"type:epic"}]}`,
		"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[]}`,
	}}

	report, err := BuildGoalStatusReport(GoalStatusInput{Repo: repo, Goal: 100}, runner)
	if err != nil {
		t.Fatalf("BuildGoalStatusReport: %v", err)
	}
	if report.Counts["total"] != 18 || report.Counts["known"] != 0 || report.Counts["unknown"] != 18 || len(report.Children) != 18 {
		t.Fatalf("failed snapshot lost child identities: %+v", report)
	}
	if report.RemainingAutonomousWork != nil || report.NextAction != "resolve_blockers" || report.NextAction == "plan_children" || !report.DiscoveryComplete || report.StatusComplete {
		t.Fatalf("failed snapshot was presented as an empty/actionable goal: %+v", report)
	}
	if len(report.AcquisitionFailures) != 1 || report.AcquisitionFailures[0].Stage != goalStatusFailureStageSnapshotTransport || report.AcquisitionFailures[0].Code != "transport_unknown" || report.AcquisitionFailures[0].AffectedCount != 18 || !report.AcquisitionFailures[0].AffectedCountComplete || len(report.AcquisitionFailures[0].AffectedChildren) != 18 {
		t.Fatalf("snapshot failure metadata is incomplete or unbounded: %+v", report.AcquisitionFailures)
	}
	for i, child := range report.Children {
		if child.Number != []int{651, 652, 656, 657, 658, 659, 673, 677, 680, 683, 687, 688, 689, 693, 696, 699, 700, 703}[i] || child.StatusAvailable || child.Category != "unknown" || child.URL == "" {
			t.Fatalf("unknown child row %d lost identity or gained status: %+v", i, child)
		}
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal incomplete status: %v", err)
	}
	if !strings.Contains(string(encoded), `"remaining_autonomous_work":null`) || !strings.Contains(string(encoded), `"status_available":false`) {
		t.Fatalf("JSON did not preserve nullable remaining or unknown status: %s", encoded)
	}
	formatted := FormatGoalStatus(report)
	for _, want := range []string{"children=18 known_remaining=0 remaining=unknown", "discovery_complete=true status_complete=false", "unknown=18", "next=resolve_blockers", "snapshot_transport:transport_unknown:18"} {
		if !strings.Contains(formatted, want) {
			t.Fatalf("text status missing %q:\n%s", want, formatted)
		}
	}
	if got := runner.countPrefix("gh api repos/StatPan/gira/issues/651"); got != 0 {
		t.Fatalf("snapshot failure triggered per-child fallback: %d calls", got)
	}
}

func TestGoalStatusRetainsKnownAndUnknownChildrenAcrossRepositories(t *testing.T) {
	parent := RepoRef{Owner: "StatPan", Name: "backlog"}
	giraRepo := RepoRef{Owner: "StatPan", Name: "gira"}
	markers := []string{
		"<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=201 -->",
		"<!-- gira:goal-child-link/v1 repo=StatPan/agentree issue=301 -->",
	}
	runner := &goalStatusCountingRunner{
		responses: map[string]string{
			"gh api repos/StatPan/backlog/issues/100":                  `{"number":100,"title":"Goal","state":"open","body":"` + strings.Join(markers, `\n`) + `","labels":[{"name":"type:epic"}]}`,
			"gh issue view 100 --repo StatPan/backlog --json comments": `{"comments":[]}`,
		},
		graphql: map[string]string{giraRepo.FullName(): goalStatusGraphQLFixture(201, 201)},
	}
	report, err := BuildGoalStatusReport(GoalStatusInput{Repo: parent, Goal: 100}, runner)
	if err != nil {
		t.Fatalf("BuildGoalStatusReport: %v", err)
	}
	if report.Counts["total"] != 2 || report.Counts["known"] != 1 || report.Counts["unknown"] != 1 || len(report.Children) != 2 {
		t.Fatalf("cross-repository partial state counts = %+v", report.Counts)
	}
	if report.Children[0].Repo != "StatPan/agentree" || report.Children[0].StatusAvailable || report.Children[1].Repo != giraRepo.FullName() || !report.Children[1].StatusAvailable {
		t.Fatalf("known repo state or unavailable identity was lost: %+v", report.Children)
	}
	if report.RemainingAutonomousWork != nil || report.NextAction != "resolve_blockers" {
		t.Fatalf("partial status should block selection and keep overall remaining unknown: %+v", report)
	}
}

func TestGoalStatusDiscoveryFailureCannotMasqueradeAsNoChildren(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	nativeSubissues := "gh api repos/StatPan/gira/issues/100/sub_issues -X GET -H Accept: application/vnd.github+json -H X-GitHub-Api-Version: 2026-03-10 -f per_page=100"
	runner := &goalStatusCountingRunner{
		responses: map[string]string{
			"gh api repos/StatPan/gira/issues/100":                  `{"number":100,"title":"Goal","state":"open","body":"","labels":[{"name":"type:epic"}]}`,
			"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[]}`,
		},
		errors: map[string]error{nativeSubissues: errors.New("unknown transport detail: token ghp_do_not_publish")},
	}
	report, err := BuildGoalStatusReport(GoalStatusInput{Repo: repo, Goal: 100}, runner)
	if err != nil {
		t.Fatalf("BuildGoalStatusReport: %v", err)
	}
	if report.Counts["total"] != 0 || report.RemainingAutonomousWork != nil || report.DiscoveryComplete || !report.StatusComplete || report.NextAction != "resolve_blockers" {
		t.Fatalf("incomplete discovery was mistaken for an empty goal: %+v", report)
	}
	if len(report.AcquisitionFailures) != 1 || report.AcquisitionFailures[0].Stage != goalStatusFailureStageChildDiscovery || report.AcquisitionFailures[0].Code != "transport_unknown" || report.AcquisitionFailures[0].AffectedCountComplete {
		t.Fatalf("discovery failure receipt is not honest and bounded: %+v", report.AcquisitionFailures)
	}
	out, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if strings.Contains(string(out), "ghp_do_not_publish") || strings.Contains(string(out), "unknown transport detail") {
		t.Fatalf("raw discovery error leaked into report: %s", out)
	}
	if next := BuildGoalNextReportFromStatus(repo, report); next.SelectedTicket != nil || next.NextAction != "resolve_blockers" || !containsString(next.StopReasons, "child_discovery_incomplete") {
		t.Fatalf("goal next acted on incomplete discovery: %+v", next)
	}
}

func TestGoalStatusDiscoveryFailureRetainsKnownRowsAndBlocksReplan(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	nativeSubissues := "gh api repos/StatPan/gira/issues/100/sub_issues -X GET -H Accept: application/vnd.github+json -H X-GitHub-Api-Version: 2026-03-10 -f per_page=100"
	runner := &goalStatusCountingRunner{
		responses: map[string]string{
			"gh api repos/StatPan/gira/issues/100":                  `{"number":100,"title":"Goal","state":"open","body":"<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=201 -->","labels":[{"name":"type:epic"}]}`,
			"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[]}`,
			"gh api graphql": goalStatusGraphQLFixture(201, 201),
		},
		errors: map[string]error{nativeSubissues: errors.New("transport unavailable")},
	}
	report, err := BuildGoalStatusReport(GoalStatusInput{Repo: repo, Goal: 100}, runner)
	if err != nil {
		t.Fatalf("BuildGoalStatusReport: %v", err)
	}
	if len(report.Children) != 1 || !report.Children[0].StatusAvailable || report.Counts["known"] != 1 || !report.StatusComplete || report.DiscoveryComplete || report.RemainingAutonomousWork != nil || report.NextAction != "resolve_blockers" {
		t.Fatalf("known statuses must not hide incomplete discovery: %+v", report)
	}
	next := BuildGoalNextReportFromStatus(repo, report)
	if next.SelectedTicket != nil || next.NextAction != "resolve_blockers" {
		t.Fatalf("goal next selected from an incomplete identity graph: %+v", next)
	}
}

func TestBuildGoalStatusReportFailsClosedOnOperationPolicyResolutionError(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".gira", "config.yaml"), "repo: StatPan/gira\noperation_mode: observation\ndelivery_policy: required\nprofiles:\n  default:\n    labels: []\n")
	t.Chdir(root)
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &goalStatusCountingRunner{responses: map[string]string{
		"gh api repos/StatPan/gira/issues/100":                  `{"number":100,"title":"Goal","state":"open","body":"<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=201 -->","labels":[{"name":"type:epic"}]}`,
		"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[]}`,
		"gh api graphql": goalStatusGraphQLFixture(201, 201),
	}}
	report, err := BuildGoalStatusReport(GoalStatusInput{Repo: repo, Goal: 100}, runner)
	if err != nil {
		t.Fatalf("BuildGoalStatusReport error: %v", err)
	}
	if len(report.Children) != 1 || report.Children[0].Title != "Child" || report.Children[0].StatusAvailable || report.Counts["total"] != 1 || report.Counts["unknown"] != 1 || report.RemainingAutonomousWork != nil || !containsString(report.Blockers, "child_201_status_unavailable") {
		t.Fatalf("invalid operation policy must retain snapshot identity while blocking status: %+v", report)
	}
}

func TestGoalStatusIssueSnapshotRejectsTruncatedGraphQLConnections(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	base := `"number":201,"title":"Child","state":"OPEN","body":"Work","labels":{"nodes":[{"name":"status:ready"}],"pageInfo":{"hasNextPage":%s}},"timelineItems":{"totalCount":1,"pageInfo":{"hasNextPage":%s},"nodes":[{"source":{"number":301,"title":"PR","body":"Fixes #201","state":"OPEN","url":"u","isDraft":false,"mergeStateStatus":"CLEAN","reviewDecision":"APPROVED","headRefName":"issue-201","baseRefName":"main","headRefOid":"sha","statusCheckRollup":{"contexts":{"pageInfo":{"hasNextPage":%s},"nodes":[{"name":"ci","status":"COMPLETED","conclusion":"SUCCESS"}]}}}}]}}`
	for _, tc := range []struct {
		name         string
		labelsNext   string
		timelineNext string
		checksNext   string
		wantCode     string
	}{
		{name: "labels", labelsNext: "true", timelineNext: "false", checksNext: "false", wantCode: "labels_truncated"},
		{name: "timeline", labelsNext: "false", timelineNext: "true", checksNext: "false", wantCode: "timeline_truncated"},
		{name: "checks", labelsNext: "false", timelineNext: "false", checksNext: "true", wantCode: "checks_truncated"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := `{"data":{"repository":{"issue201":{` + fmt.Sprintf(base, tc.labelsNext, tc.timelineNext, tc.checksNext) + `}}}`
			runner := &goalStatusCountingRunner{responses: map[string]string{"gh api graphql": payload}}
			_, _, _, _, _, attempted, err := goalStatusIssueSnapshot(repo, []int{201}, runner)
			if attempted != true {
				t.Fatalf("attempted = %v, want true", attempted)
			}
			stage, code := goalStatusFailureClass(err)
			if stage != goalStatusFailureStageSnapshotLimit || code != tc.wantCode {
				t.Fatalf("truncation failure = %s/%s, want %s/%s", stage, code, goalStatusFailureStageSnapshotLimit, tc.wantCode)
			}
		})
	}
}

func TestGoalStatusIssueSnapshotRejectsOversizedRepositoryBatch(t *testing.T) {
	numbers := make([]int, goalStatusRepositoryChildLimit+1)
	for index := range numbers {
		numbers[index] = index + 1
	}
	runner := &goalStatusCountingRunner{responses: map[string]string{}}
	_, _, _, _, _, attempted, err := goalStatusIssueSnapshot(RepoRef{Owner: "StatPan", Name: "gira"}, numbers, runner)
	stage, code := goalStatusFailureClass(err)
	if !attempted || stage != goalStatusFailureStageSnapshotLimit || code != "child_limit_exceeded" {
		t.Fatalf("oversized repository batch = attempted %v, err %v", attempted, err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("oversized repository batch must not issue an unbounded query: %v", runner.calls)
	}
}

func TestGoalStatusBatchPolicyFailurePreservesClosingReferenceContract(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	issue := devStartIssue{Number: 201, Title: "Child"}
	prs := []prSummary{{
		Number:      301,
		Body:        "Closes #201",
		State:       "OPEN",
		HeadRefName: "feature/201-child",
		BaseRefName: "main",
		HeadRefOID:  "sha-301",
		StatusRollup: []struct {
			Name       string `json:"name"`
			Workflow   string `json:"workflowName"`
			Conclusion string `json:"conclusion"`
			Status     string `json:"status"`
			URL        string `json:"detailsUrl"`
		}{{Name: "ci", Status: "COMPLETED", Conclusion: "SUCCESS"}},
	}}
	withoutPolicy := goalStatusPRForIssue(repo, issue, prs, false, nil)
	if !withoutPolicy.Ready || withoutPolicy.Binding.Source != "closing_reference" {
		t.Fatalf("policy lookup failure changed the existing closing-reference contract: %+v", withoutPolicy)
	}
	withPolicy := goalStatusPRForIssue(repo, issue, prs, false, &ResolvedBranchPolicy{Source: "config", FeatureBranchPattern: "feature/{number}-{slug}"})
	if !withPolicy.Ready || withPolicy.Binding.Source != "branch_policy.feature_branch_pattern" || containsString(withPolicy.Blockers, "pr_binding") {
		t.Fatalf("matching resolved policy should restore trusted binding: %+v", withPolicy)
	}
}

func TestBuildGoalStatusBatchesCurrentHeadReviewEvidence(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".gira", "config.yaml"), "repo: StatPan/gira\nfinish_review_policy: required\nprofiles:\n  default:\n    labels: []\n")
	t.Chdir(root)
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	children := make([]string, 0, 4)
	for number := 201; number <= 204; number++ {
		children = append(children, fmt.Sprintf("<!-- gira:goal-child-link/v1 repo=StatPan/gira issue=%d -->", number))
	}
	runner := &goalStatusCountingRunner{
		responses: map[string]string{
			"gh api repos/StatPan/gira/issues/100":                  `{"number":100,"title":"Goal","state":"open","body":"` + strings.Join(children, "\\n") + `","labels":[{"name":"type:epic"}]}`,
			"gh issue view 100 --repo StatPan/gira --json comments": `{"comments":[]}`,
		},
	}
	runner.responses["gh api graphql"] = goalStatusGraphQLFixtureWithInReviewPRs(201, 204)

	report, err := BuildGoalStatusReport(GoalStatusInput{Repo: repo, Goal: 100}, runner)
	if err != nil {
		t.Fatalf("BuildGoalStatusReport error: %v", err)
	}
	if len(report.Children) != 4 {
		t.Fatalf("in-review children = %d, want 4: %+v", len(report.Children), report)
	}
	if got := runner.countPrefix("gh api graphql"); got != 1 {
		t.Fatalf("GraphQL snapshots = %d, want 1", got)
	}
	if got := runner.countPrefix("gh api repos/StatPan/gira/pulls/"); got != 0 {
		t.Fatalf("review evidence re-entered per-child provider calls: %d", got)
	}
	for _, child := range report.Children {
		if containsString(child.Blockers, "review_evidence_unavailable") || containsString(child.Blockers, "review_approval_stale") {
			t.Fatalf("current-head approval should be retained for child %+v", child)
		}
	}
}

func TestGoalStatusReviewSnapshotTruncationFailsClosed(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	runner := &goalStatusCountingRunner{responses: map[string]string{
		"gh api graphql": goalStatusGraphQLFixtureWithReviewPage(201, 201, true),
	}}
	_, _, _, _, _, _, err := goalStatusIssueSnapshot(repo, []int{201}, runner)
	stage, code := goalStatusFailureClass(err)
	if stage != goalStatusFailureStageSnapshotLimit || code != "reviews_truncated" {
		t.Fatalf("review-page truncation = %s/%s, want snapshot_limit/reviews_truncated", stage, code)
	}
}

func goalStatusGraphQLFixture(first, last int) string {
	rows := make([]string, 0, last-first+1)
	for number := first; number <= last; number++ {
		status := "done"
		state := "closed"
		body := "Done"
		if first == last {
			status = "in-progress"
			state = "open"
			body = "Work"
		}
		rows = append(rows, fmt.Sprintf(`"issue%d":{"number":%d,"title":"Child","state":"%s","body":"%s","labels":{"nodes":[{"name":"type:task"},{"name":"status:%s"}]}}`, number, number, state, body, status))
	}
	return `{"data":{"repository":{` + strings.Join(rows, ",") + `}}}`
}

func goalStatusGraphQLFixtureWithPRs(first, last int) string {
	rows := make([]string, 0, last-first+1)
	for number := first; number <= last; number++ {
		rows = append(rows, fmt.Sprintf(`"issue%d":{"number":%d,"title":"Child","state":"CLOSED","body":"Done","labels":{"nodes":[{"name":"type:task"},{"name":"status:done"}]},"timelineItems":{"nodes":[{"source":{"number":%d,"title":"PR","body":"Closes #%d","state":"MERGED","url":"https://example.test/pull/%d","isDraft":false,"mergeStateStatus":"UNKNOWN","reviewDecision":null,"headRefName":"issue-%d-child","baseRefName":"main","headRefOid":"sha-%d","mergeCommit":{"oid":"merge-%d"},"statusCheckRollup":{"contexts":{"nodes":[{"name":"ci","status":"COMPLETED","conclusion":"SUCCESS","detailsUrl":"https://example.test/ci"}]}}}}]}}`, number, number, number+100, number, number+100, number, number, number))
	}
	return `{"data":{"repository":{` + strings.Join(rows, ",") + `}}}`
}

func goalStatusGraphQLFixtureWithUnknownPR(issueNumber, prNumber int) string {
	return fmt.Sprintf(`{"data":{"repository":{"issue%d":{"number":%d,"title":"Child","state":"OPEN","body":"Work","labels":{"nodes":[{"name":"type:task"},{"name":"status:in-progress"}]},"timelineItems":{"nodes":[{"source":{"number":%d,"title":"PR","body":"Fixes #%d","state":"OPEN","url":"https://example.test/pull/%d","isDraft":false,"mergeStateStatus":"CLEAN","reviewDecision":"APPROVED","headRefName":"issue-%d-child","baseRefName":"main","headRefOid":"sha-%d","statusCheckRollup":{"contexts":{"nodes":[{"name":"ci","status":"COMPLETED","conclusion":"","detailsUrl":"https://example.test/ci"}]}}}}]}}}}}`, issueNumber, issueNumber, prNumber, issueNumber, prNumber, issueNumber, issueNumber)
}

func goalStatusGraphQLFixtureWithInReviewPRs(first, last int) string {
	return goalStatusGraphQLFixtureWithReviewPage(first, last, false)
}

func goalStatusGraphQLFixtureWithReviewPage(first, last int, reviewNext bool) string {
	rows := make([]string, 0, last-first+1)
	for number := first; number <= last; number++ {
		rows = append(rows, fmt.Sprintf(`"issue%d":{"number":%d,"title":"Child","state":"OPEN","body":"Work","labels":{"nodes":[{"name":"type:task"},{"name":"status:in-review"}]},"timelineItems":{"totalCount":1,"pageInfo":{"hasNextPage":false},"nodes":[{"source":{"number":%d,"title":"PR","body":"Closes #%d","state":"OPEN","url":"https://example.test/pull/%d","isDraft":false,"mergeStateStatus":"CLEAN","reviewDecision":"APPROVED","headRefName":"issue-%d-child","baseRefName":"main","headRefOid":"sha-%d","reviews":{"nodes":[{"state":"APPROVED","commit":{"oid":"sha-%d"}}],"pageInfo":{"hasNextPage":%t}},"statusCheckRollup":{"contexts":{"nodes":[{"name":"ci","status":"COMPLETED","conclusion":"SUCCESS","detailsUrl":"https://example.test/ci"}],"pageInfo":{"hasNextPage":false}}}}}]}}`, number, number, number+100, number, number+100, number, number, number, reviewNext))
	}
	return `{"data":{"repository":{` + strings.Join(rows, ",") + `}}}`
}
