package gira

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const (
	recordedFinishLifecycleHead = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	recordedFinishLifecycleBase = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

type recordedFinishLifecycleSnapshot struct {
	HeadSHA           string
	BaseSHA           string
	BaseRef           string
	HeadRef           string
	Draft             bool
	CheckRunStatus    string
	CheckConclusion   string
	ChecksUnavailable bool
	Reviews           []finishReview
}

// recordedFinishLifecycleRunner drives the real status and finish paths while
// keeping GitHub responses deterministic. Tests may change revalidation to
// model evidence changing between the initial read and the guarded merge.
type recordedFinishLifecycleRunner struct {
	root         string
	initial      recordedFinishLifecycleSnapshot
	revalidation recordedFinishLifecycleSnapshot
	issueState   string
	issueLabels  []string
	merged       bool
	pullReads    int
	policyReads  int
	rawReviewReads int
	receiptReads int
	calls        []string
	mergeCalls   []string
	mutations    []string
	mu           sync.Mutex
}

func newRecordedFinishLifecycleRunner(t *testing.T) *recordedFinishLifecycleRunner {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".gira", "config.yaml"), recordedPolicyYAML())
	t.Chdir(root)

	initial := recordedFinishLifecycleSnapshot{
		HeadSHA:         recordedFinishLifecycleHead,
		BaseSHA:         recordedFinishLifecycleBase,
		BaseRef:         "dev",
		HeadRef:         "issue-219-finish",
		CheckRunStatus:  "completed",
		CheckConclusion: "success",
	}
	initial.Reviews = []finishReview{recordedFinishLifecycleReceipt(initial)}
	revalidation := initial
	revalidation.Reviews = append([]finishReview(nil), initial.Reviews...)
	return &recordedFinishLifecycleRunner{
		root:         root,
		initial:      initial,
		revalidation: revalidation,
		issueState:   "open",
		issueLabels:  []string{"status:in-review"},
	}
}

func recordedFinishLifecycleReceipt(snapshot recordedFinishLifecycleSnapshot) finishReview {
	receipt := IndependentReviewReceipt{
		SchemaVersion: IndependentReviewReceiptSchema,
		Repository:    "StatPan/gira",
		PullRequest:   220,
		HeadSHA:       snapshot.HeadSHA,
		BaseRef:       snapshot.BaseRef,
		BaseSHA:       snapshot.BaseSHA,
		Verdict:       "GO",
		Reviewer:      IndependentReviewIdentity{ID: "reviewer-run-978", Kind: "ai", RunRef: "https://example.test/reviewer-run-978"},
		Implementer:   IndependentReviewIdentity{ID: "implementer-run-978", Kind: "ai", RunRef: "https://example.test/implementer-run-978"},
		Recorder:      "gira-review-bot",
		Independent:   true,
		EvidenceRefs:  []string{"https://github.com/StatPan/gira/pull/220"},
		Findings:      []IndependentReviewFinding{},
		Limitations:   []string{"Fake GitHub lifecycle fixture; no live repository review was performed."},
	}
	body, _ := json.Marshal(receipt)
	review := finishReview{
		ID:          9781,
		State:       "COMMENTED",
		CommitID:    snapshot.HeadSHA,
		Body:        IndependentReviewReceiptMarker + "\n" + string(body),
		SubmittedAt: "2026-10-04T00:00:00Z",
	}
	review.User.Login = "gira-review-bot"
	return review
}

func recordedFinishLifecycleApproval(snapshot recordedFinishLifecycleSnapshot) finishReview {
	review := finishReview{ID: 9782, State: "APPROVED", CommitID: snapshot.HeadSHA, SubmittedAt: "2026-10-04T00:01:00Z"}
	review.User.Login = "native-reviewer"
	return review
}

func (r *recordedFinishLifecycleRunner) Run(name string, args ...string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, key)
	switch {
	case key == "gh issue view 219 --repo StatPan/gira --json number,title,body":
		return []byte(`{"number":219,"title":"Finish review lifecycle","body":""}`), nil
	case key == "gh repo view StatPan/gira --json nameWithOwner,viewerPermission,defaultBranchRef":
		return []byte(`{"nameWithOwner":"StatPan/gira","viewerPermission":"ADMIN","defaultBranchRef":{"name":"main"}}`), nil
	case key == "git rev-parse --show-toplevel":
		return []byte(r.root), nil
	case key == "gh api repos/StatPan/gira/issues/219/timeline --paginate":
		return []byte(`[{"source":{"issue":{"number":220,"body":"Closes #219","pull_request":{"url":"https://api.github.com/repos/StatPan/gira/pulls/220"}}}}]`), nil
	case key == "gh api repos/StatPan/gira/pulls/220":
		return r.pullResponse()
	case key == "gh api repos/StatPan/gira/pulls/220/reviews --paginate":
		r.rawReviewReads++
		return r.reviewsResponse(false)
	case key == "gh api repos/StatPan/gira/pulls/220/reviews --paginate --slurp":
		r.receiptReads++
		return r.reviewsResponse(true)
	case strings.HasPrefix(key, "gh api repos/StatPan/gira/branches/") && strings.HasSuffix(key, "/protection/required_pull_request_reviews"):
		return []byte(`{}`), nil
	case strings.Contains(key, "/check-runs -X GET "):
		return r.checkRunsResponse()
	case strings.HasPrefix(key, "gh api repos/StatPan/gira/commits/") && strings.HasSuffix(key, "/status"):
		return []byte(`{"statuses":[]}`), nil
	case strings.HasPrefix(key, "gh api repos/StatPan/gira/contents/.gira/config.yaml --method GET -f ref="):
		r.policyReads++
		return githubContentsFile(".gira/config.yaml", recordedPolicyYAML()), nil
	case key == "gh api repos/StatPan/gira/issues/219":
		return r.issueResponse(), nil
	case key == "gh api repos/StatPan/gira/labels --paginate --slurp -X GET -f per_page=100":
		return []byte(`[[{"name":"status:done"},{"name":"status:in-review"}]]`), nil
	case strings.HasPrefix(key, "gh pr merge 220 "):
		r.mergeCalls = append(r.mergeCalls, key)
		r.mutations = append(r.mutations, key)
		r.merged = true
		return nil, nil
	case key == "gh pr ready 220 --repo StatPan/gira":
		r.mutations = append(r.mutations, key)
		r.initial.Draft = false
		r.revalidation.Draft = false
		return nil, nil
	case key == "gh issue close 219 --repo StatPan/gira --reason completed":
		r.mutations = append(r.mutations, key)
		r.issueState = "closed"
		return nil, nil
	case strings.HasPrefix(key, "gh issue edit 219 --repo StatPan/gira"):
		r.mutations = append(r.mutations, key)
		r.issueLabels = []string{"status:done"}
		return nil, nil
	case strings.HasPrefix(key, "gh issue comment 219 --repo StatPan/gira --body "):
		r.mutations = append(r.mutations, "gh issue comment 219 --repo StatPan/gira --body <finish receipt>")
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected recorded finish lifecycle call: %s", key)
	}
}

func (r *recordedFinishLifecycleRunner) activeSnapshot() *recordedFinishLifecycleSnapshot {
	if r.pullReads <= 1 {
		return &r.initial
	}
	return &r.revalidation
}

func (r *recordedFinishLifecycleRunner) pullResponse() ([]byte, error) {
	if r.merged {
		mergedAt := "2026-10-04T00:05:00Z"
		return json.Marshal(map[string]any{
			"number": 220, "body": "Closes #219", "state": "closed", "merged_at": mergedAt,
			"merge_commit_sha": "cccccccccccccccccccccccccccccccccccccccc", "html_url": "https://github.com/StatPan/gira/pull/220",
			"mergeable_state": "unknown", "head": map[string]string{"ref": r.initial.HeadRef, "sha": r.initial.HeadSHA},
			"base": map[string]string{"ref": r.initial.BaseRef, "sha": r.initial.BaseSHA},
		}), nil
	}
	r.pullReads++
	snapshot := r.activeSnapshot()
	return json.Marshal(map[string]any{
		"number": 220, "body": "Closes #219", "state": "open", "html_url": "https://github.com/StatPan/gira/pull/220",
		"draft": snapshot.Draft, "mergeable_state": "clean",
		"head": map[string]string{"ref": snapshot.HeadRef, "sha": snapshot.HeadSHA},
		"base": map[string]string{"ref": snapshot.BaseRef, "sha": snapshot.BaseSHA},
	}), nil
}

func (r *recordedFinishLifecycleRunner) reviewsResponse(slurp bool) ([]byte, error) {
	reviews := r.activeSnapshot().Reviews
	if slurp {
		return json.Marshal([][]finishReview{reviews})
	}
	return json.Marshal(reviews)
}

func (r *recordedFinishLifecycleRunner) checkRunsResponse() ([]byte, error) {
	snapshot := r.activeSnapshot()
	if snapshot.ChecksUnavailable {
		return nil, fmt.Errorf("check-runs API unavailable")
	}
	check := map[string]string{"name": "CI", "status": snapshot.CheckRunStatus, "conclusion": snapshot.CheckConclusion}
	return json.Marshal([]map[string]any{{"check_runs": []map[string]string{check}}})
}

func (r *recordedFinishLifecycleRunner) issueResponse() []byte {
	labelRows := make([]map[string]string, 0, len(r.issueLabels))
	for _, label := range r.issueLabels {
		labelRows = append(labelRows, map[string]string{"name": label})
	}
	body, _ := json.Marshal(map[string]any{
		"number": 219, "title": "Finish review lifecycle", "state": r.issueState,
		"body": "", "labels": labelRows,
	})
	return body
}

func TestFinishWorkWithOptionsNormalizesVerifiedReviewBlocker(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "gira"}

	t.Run("recorded GO agrees with status, dry-run and guarded apply", func(t *testing.T) {
		rawRunner := newRecordedFinishLifecycleRunner(t)
		raw, err := DevPRStatus(repo, 219, rawRunner)
		if err != nil {
			t.Fatalf("DevPRStatus error: %v", err)
		}
		if raw.ReviewDecision != "REVIEW_REQUIRED" || !containsString(raw.Blockers, "review") {
			t.Fatalf("fixture must expose the generic review blocker before evidence normalization: %+v", raw)
		}

		statusRunner := newRecordedFinishLifecycleRunner(t)
		status, err := GetWorkStatus(repo, 219, statusRunner)
		if err != nil {
			t.Fatalf("GetWorkStatus error: %v", err)
		}
		if status.ReviewStatus != "independent_recorded" || status.ReviewEvidence == nil || status.ReviewEvidence.Status != "independent_recorded" || containsString(status.Blockers, "review") {
			t.Fatalf("ticket status did not normalize verified review: %+v", status)
		}

		dryRunner := newRecordedFinishLifecycleRunner(t)
		dry, err := FinishWorkWithOptions(repo, 219, true, 0, WorkFinishOptions{}, dryRunner)
		if err != nil {
			t.Fatalf("FinishWorkWithOptions dry-run error: %v", err)
		}
		if dry.ReviewEvidence.Status != "independent_recorded" || dry.FinalStatus.ReviewStatus != status.ReviewStatus || containsString(dry.Blockers, "review") || !finishActionStatus(dry.Actions, "pr:merge", "planned") || !dry.Readiness.Ready || len(dry.Readiness.Blockers) != 0 {
			t.Fatalf("dry-run disagrees with ticket status or failed to plan merge: %+v", dry)
		}
		if len(dryRunner.mergeCalls) != 0 || len(dryRunner.mutations) != 0 {
			t.Fatalf("dry-run made mutations: merge=%v mutations=%v", dryRunner.mergeCalls, dryRunner.mutations)
		}

		applyRunner := newRecordedFinishLifecycleRunner(t)
		applied, err := FinishWorkWithOptions(repo, 219, false, 0, WorkFinishOptions{}, applyRunner)
		if err != nil {
			t.Fatalf("FinishWorkWithOptions apply error: %v", err)
		}
		wantMerge := "gh pr merge 220 --repo StatPan/gira --squash --delete-branch --match-head-commit " + recordedFinishLifecycleHead
		if !applied.Merged || applied.PRState != "MERGED" || len(applyRunner.mergeCalls) != 1 || applyRunner.mergeCalls[0] != wantMerge {
			t.Fatalf("apply did not perform one pinned verified merge: result=%+v mergeCalls=%v", applied, applyRunner.mergeCalls)
		}
		if !containsString(applyRunner.mutations, "gh issue close 219 --repo StatPan/gira --reason completed") || applyRunner.pullReads != 2 || applyRunner.policyReads != 2 || applyRunner.rawReviewReads != 2 || applyRunner.receiptReads != 2 || len(applyRunner.mutations) < 3 {
			t.Fatalf("apply did not revalidate committed policy, reviews and convergence: pulls=%d policies=%d rawReviews=%d receipts=%d mutations=%v", applyRunner.pullReads, applyRunner.policyReads, applyRunner.rawReviewReads, applyRunner.receiptReads, applyRunner.mutations)
		}
	})

	t.Run("native approval remains a full-finish success", func(t *testing.T) {
		runner := newRecordedFinishLifecycleRunner(t)
		approval := recordedFinishLifecycleApproval(runner.initial)
		runner.initial.Reviews = []finishReview{approval}
		runner.revalidation.Reviews = []finishReview{approval}
		result, err := FinishWorkWithOptions(repo, 219, false, 0, WorkFinishOptions{}, runner)
		if err != nil {
			t.Fatalf("FinishWorkWithOptions native approval error: %v", err)
		}
		wantMerge := "gh pr merge 220 --repo StatPan/gira --squash --delete-branch --match-head-commit " + recordedFinishLifecycleHead
		if !result.Merged || result.ReviewEvidence.Status != "approved" || len(runner.mergeCalls) != 1 || runner.mergeCalls[0] != wantMerge {
			t.Fatalf("native approval did not reach the guarded full finish: result=%+v mergeCalls=%v", result, runner.mergeCalls)
		}
	})
}
