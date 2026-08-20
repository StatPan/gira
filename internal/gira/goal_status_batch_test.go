package gira

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type goalStatusCountingRunner struct {
	mu        sync.Mutex
	responses map[string]string
	graphql   map[string]string
	calls     []string
}

func (r *goalStatusCountingRunner) Run(name string, args ...string) ([]byte, error) {
	key := strings.TrimSpace(strings.Join(append([]string{name}, args...), " "))
	r.mu.Lock()
	r.calls = append(r.calls, key)
	response, ok := r.responses[key]
	if !ok && strings.HasPrefix(key, "gh api graphql ") {
		owner := commandFieldForCountingRunner(key, "owner")
		name := commandFieldForCountingRunner(key, "name")
		response, ok = r.graphql[owner+"/"+name]
		if !ok {
			response, ok = r.responses["gh api graphql"]
		}
	}
	r.mu.Unlock()
	if !ok {
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
	if len(report.Children) != 0 || len(report.Blockers) != 2 || !containsString(report.Blockers, "child_201_status_unavailable") || !containsString(report.Blockers, "child_202_status_unavailable") {
		t.Fatalf("partial snapshot must fail closed for every child: %+v", report)
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
	if len(report.Children) != 0 || !containsString(report.Blockers, "child_201_status_unavailable") {
		t.Fatalf("provider error must remain unavailable: %+v", report)
	}
	if got := runner.countPrefix("gh api repos/StatPan/gira/issues/201"); got != 0 {
		t.Fatalf("provider error re-entered per-child reads: %d", got)
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
	if len(report.Children) != 0 || !containsString(report.Blockers, "child_201_status_unavailable") {
		t.Fatalf("invalid operation policy must remain unavailable: %+v", report)
	}
}

func TestGoalStatusIssueSnapshotRejectsTruncatedGraphQLConnections(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	base := `"number":201,"title":"Child","state":"OPEN","body":"Work","labels":{"nodes":[{"name":"status:ready"}],"pageInfo":{"hasNextPage":%s}},"timelineItems":{"totalCount":1,"pageInfo":{"hasNextPage":%s},"nodes":[{"source":{"number":301,"title":"PR","body":"Fixes #201","state":"OPEN","url":"u","isDraft":false,"mergeStateStatus":"CLEAN","reviewDecision":"APPROVED","headRefName":"issue-201","baseRefName":"main","headRefOid":"sha","statusCheckRollup":{"contexts":{"pageInfo":{"hasNextPage":%s},"nodes":[{"name":"ci","status":"COMPLETED","conclusion":"SUCCESS"}]}}}}]}}`
	for _, tc := range []struct {
		name             string
		labelsNext       string
		timelineNext     string
		checksNext       string
		wantErr          bool
		wantPRIncomplete bool
		wantUnknownCheck bool
	}{
		{name: "labels", labelsNext: "true", timelineNext: "false", checksNext: "false", wantErr: true},
		{name: "timeline", labelsNext: "false", timelineNext: "true", checksNext: "false", wantPRIncomplete: true},
		{name: "checks", labelsNext: "false", timelineNext: "false", checksNext: "true", wantUnknownCheck: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := `{"data":{"repository":{"issue201":{` + fmt.Sprintf(base, tc.labelsNext, tc.timelineNext, tc.checksNext) + `}}}`
			runner := &goalStatusCountingRunner{responses: map[string]string{"gh api graphql": payload}}
			_, prs, incomplete, _, _, attempted, err := goalStatusIssueSnapshot(repo, []int{201}, runner)
			if attempted != true {
				t.Fatalf("attempted = %v, want true", attempted)
			}
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr=%v", err, tc.wantErr)
			}
			if tc.wantPRIncomplete && !incomplete[201] {
				t.Fatalf("timeline truncation was not retained: %+v", incomplete)
			}
			if tc.wantPRIncomplete {
				status := goalStatusPRForIssue(repo, devStartIssue{Number: 201, Title: "Child"}, prs, true, nil)
				if !containsString(status.Blockers, "pr_snapshot_incomplete") {
					t.Fatalf("candidate plus truncated timeline must block: %+v", status)
				}
			}
			if tc.wantUnknownCheck && (len(prs) != 1 || len(prs[0].StatusRollup) != 2) {
				t.Fatalf("check truncation was not retained: %+v", prs)
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
	if !attempted || err == nil || !strings.Contains(err.Error(), "snapshot limit") {
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
	_, prs, _, reviews, reviewsIncomplete, _, err := goalStatusIssueSnapshot(repo, []int{201}, runner)
	if err != nil {
		t.Fatalf("snapshot error: %v", err)
	}
	if !reviewsIncomplete[301] || len(reviews[301]) != 1 {
		t.Fatalf("review page truncation was not retained: reviews=%+v incomplete=%+v", reviews, reviewsIncomplete)
	}
	issue := devStartIssue{Number: 201, Labels: []string{"status:in-review"}}
	status := goalStatusPRForIssue(repo, issue, prs, false, nil)
	evidence := goalStatusReviewEvidenceForPR(issue, status, goalStatusRepositorySnapshot{Reviews: reviews, ReviewsIncomplete: reviewsIncomplete}, FinishReviewPolicy{Value: FinishReviewPolicyRequired})
	if evidence == nil || evidence.Blocker != "review_evidence_unavailable" {
		t.Fatalf("truncated review evidence must block: %+v", evidence)
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
