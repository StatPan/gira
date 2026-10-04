package gira

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type finishPolicyTestRunner struct {
	outputs map[string][]byte
	errs    map[string]error
	calls   []string
}

func (r *finishPolicyTestRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, key)
	if err, ok := r.errs[key]; ok {
		return nil, err
	}
	if output, ok := r.outputs[key]; ok {
		return output, nil
	}
	return nil, fmt.Errorf("unexpected call: %s", key)
}

func TestResolveFinishReviewPolicyUsesOnlyImmutablePRBase(t *testing.T) {
	const baseSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	remoteRequired := `repo: StatPan/gira
finish_review_policy: required
profiles:
  default:
    labels: []
`

	tests := []struct {
		name        string
		local       string
		baseMissing bool
		wantPolicy  string
		wantBlocker string
	}{
		{
			name:       "local none cannot weaken committed required",
			local:      "repo: StatPan/gira\nfinish_review_policy: none\nprofiles:\n  default:\n    labels: []\n",
			wantPolicy: FinishReviewPolicyRequired,
		},
		{
			name:       "candidate recorded opt-in cannot override committed required",
			local:      recordedPolicyYAML(),
			wantPolicy: FinishReviewPolicyRequired,
		},
		{
			name:        "local none cannot authorize missing committed policy",
			local:       "repo: StatPan/gira\nfinish_review_policy: none\nprofiles:\n  default:\n    labels: []\n",
			baseMissing: true,
			wantPolicy:  FinishReviewPolicyMissing,
			wantBlocker: "review_policy_not_configured",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, ".gira", "config.yaml"), tt.local)
			t.Chdir(root)
			yamlKey := "gh api repos/StatPan/gira/contents/.gira/config.yaml --method GET -f ref=" + baseSHA
			tomlKey := "gh api repos/StatPan/gira/contents/.gira/config.toml --method GET -f ref=" + baseSHA
			runner := &finishPolicyTestRunner{outputs: map[string][]byte{}, errs: map[string]error{}}
			if tt.baseMissing {
				runner.errs[yamlKey] = fmt.Errorf("HTTP 404: Not Found")
				runner.errs[tomlKey] = fmt.Errorf("HTTP 404: Not Found")
			} else {
				runner.outputs[yamlKey] = githubContentsFile(".gira/config.yaml", remoteRequired)
			}
			status := DevPRStatusResult{BaseSHA: baseSHA}
			policy := resolveFinishReviewPolicy(repo, status, runner)
			if policy.Value != tt.wantPolicy {
				t.Fatalf("policy=%q source=%q validation=%q, want %q", policy.Value, policy.Source, policy.ValidationError, tt.wantPolicy)
			}
			if policy.Source == "repo_config" || !strings.Contains(policy.Source, baseSHA) {
				t.Fatalf("policy source must identify exact immutable base: %+v", policy)
			}
			if tt.wantBlocker != "" {
				evidence := finishReviewEvidence(repo, status, policy, runner)
				if evidence.Blocker != tt.wantBlocker {
					t.Fatalf("blocker=%q, want %q (%+v)", evidence.Blocker, tt.wantBlocker, evidence)
				}
			}
			if len(runner.calls) == 0 || !strings.Contains(runner.calls[0], "--method GET") || !strings.Contains(runner.calls[0], "ref="+baseSHA) {
				t.Fatalf("base config must be read with GET pinned to the exact base SHA: %#v", runner.calls)
			}
		})
	}
}

func TestRecordedReviewMergeRevalidatesAndPreservesNativeApproval(t *testing.T) {
	const (
		reviewedHead = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		changedHead  = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		baseSHA      = "cccccccccccccccccccccccccccccccccccccccc"
	)
	repo := RepoRef{Owner: "StatPan", Name: "gira"}
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".gira", "config.yaml"), recordedPolicyYAML())
	t.Chdir(root)
	policy := FinishReviewPolicy{
		Value:  FinishReviewPolicyRecordedIndependent,
		Source: "base_commit:" + baseSHA + ":.gira/config.yaml",
		RecordedReview: &RecordedReviewConfig{
			AllowedRecorders:    []string{"gira-review-bot"},
			AllowedBaseBranches: []string{"dev"},
		},
	}
	approved := finishReview{
		ID:          301,
		State:       "APPROVED",
		CommitID:    reviewedHead,
		SubmittedAt: "2026-10-04T00:05:01Z",
	}
	approved.User.Login = "native-reviewer"

	t.Run("changed head prevents merge", func(t *testing.T) {
		runner := &recordedReviewLifecycleRunner{root: root, headSHA: changedHead, baseSHA: baseSHA, reviews: []finishReview{approved}}
		status := recordedReviewLifecycleStatus(reviewedHead, baseSHA)
		err := finishRecordedReviewMerge(repo, 219, status, policy, runner, &WorkFinishResult{})
		if err == nil || !strings.Contains(err.Error(), "changed since review") {
			t.Fatalf("changed PR head should block the pre-merge recheck, got %v", err)
		}
		if len(runner.mergeCalls) != 0 {
			t.Fatalf("recheck failure must not issue a merge: %v", runner.mergeCalls)
		}
	})

	t.Run("current native approval remains valid under recorded policy", func(t *testing.T) {
		runner := &recordedReviewLifecycleRunner{root: root, headSHA: reviewedHead, baseSHA: baseSHA, reviews: []finishReview{approved}}
		status := recordedReviewLifecycleStatus(reviewedHead, baseSHA)
		if err := finishRecordedReviewMerge(repo, 219, status, policy, runner, &WorkFinishResult{}); err != nil {
			t.Fatalf("current native approval should satisfy recorded policy: %v", err)
		}
		want := "gh pr merge 220 --repo StatPan/gira --squash --delete-branch --match-head-commit " + reviewedHead
		if len(runner.mergeCalls) != 1 || runner.mergeCalls[0] != want {
			t.Fatalf("merge must be pinned to the reviewed head: got %v want %q", runner.mergeCalls, want)
		}
	})
}

func recordedReviewLifecycleStatus(headSHA, baseSHA string) DevPRStatusResult {
	return DevPRStatusResult{
		Repo:             "StatPan/gira",
		Issue:            219,
		PRNumber:         220,
		PRURL:            "https://github.com/StatPan/gira/pull/220",
		State:            "OPEN",
		Mergeable:        "CLEAN",
		ReviewDecision:   "APPROVED",
		Binding:          DevPRBinding{Trusted: true, Source: "legacy_issue_branch", HeadRef: "issue-219-finish", BaseRef: "dev"},
		HeadSHA:          headSHA,
		BaseSHA:          baseSHA,
		ClosingReference: true,
	}
}

type recordedReviewLifecycleRunner struct {
	root       string
	headSHA    string
	baseSHA    string
	reviews    []finishReview
	mergeCalls []string
}

func (r *recordedReviewLifecycleRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	switch {
	case key == "gh issue view 219 --repo StatPan/gira --json number,title,body":
		return []byte(`{"number":219,"title":"Finish","body":""}`), nil
	case key == "gh repo view StatPan/gira --json nameWithOwner,viewerPermission,defaultBranchRef":
		return []byte(`{"nameWithOwner":"StatPan/gira","viewerPermission":"ADMIN","defaultBranchRef":{"name":"main"}}`), nil
	case key == "git rev-parse --show-toplevel":
		return []byte(r.root), nil
	case key == "gh api repos/StatPan/gira/issues/219/timeline --paginate":
		return []byte(`[{"source":{"issue":{"number":220,"body":"Closes #219","pull_request":{"url":"https://api.github.com/repos/StatPan/gira/pulls/220"}}}}]`), nil
	case key == "gh api repos/StatPan/gira/pulls/220":
		pull, err := json.Marshal(map[string]any{
			"number": 220, "body": "Closes #219", "state": "open", "html_url": "https://github.com/StatPan/gira/pull/220",
			"mergeable_state": "clean", "head": map[string]string{"ref": "issue-219-finish", "sha": r.headSHA},
			"base": map[string]string{"ref": "dev", "sha": r.baseSHA},
		})
		return pull, err
	case key == "gh api repos/StatPan/gira/pulls/220/reviews --paginate":
		return json.Marshal(r.reviews)
	case key == "gh api repos/StatPan/gira/branches/dev/protection/required_pull_request_reviews":
		return nil, fmt.Errorf("HTTP 404: branch protection is not configured")
	case strings.HasPrefix(key, "gh api repos/StatPan/gira/commits/") && strings.Contains(key, "/check-runs "):
		return []byte(`{"check_runs":[{"status":"completed","conclusion":"success"}]}`), nil
	case strings.HasPrefix(key, "gh api repos/StatPan/gira/commits/") && strings.HasSuffix(key, "/status"):
		return []byte(`{"statuses":[{"state":"success"}]}`), nil
	case strings.HasPrefix(key, "gh api repos/StatPan/gira/contents/.gira/config.yaml --method GET -f ref="):
		return githubContentsFile(".gira/config.yaml", recordedPolicyYAML()), nil
	case key == "gh api repos/StatPan/gira/pulls/220/reviews --paginate --slurp":
		return json.Marshal([][]finishReview{r.reviews})
	case strings.HasPrefix(key, "gh pr merge 220 "):
		r.mergeCalls = append(r.mergeCalls, key)
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected lifecycle fixture call: %s", key)
	}
}

func githubContentsFile(path, content string) []byte {
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	body, _ := json.Marshal(map[string]string{"type": "file", "path": path, "encoding": "base64", "content": encoded})
	return body
}

// finishReviewPolicyFixtureResponse supplies a base-commit snapshot only for
// legacy tests whose local fixture stands in for an unchanged committed config.
// Security tests use explicit remote responses so a local proposal cannot
// accidentally become the authority under test.
func finishReviewPolicyFixtureResponse(key string) ([]byte, error, bool) {
	const prefix = "gh api repos/StatPan/gira/contents/"
	const suffix = " --method GET -f ref="
	if !strings.HasPrefix(key, prefix) || !strings.Contains(key, suffix) {
		return nil, nil, false
	}
	path := strings.TrimPrefix(strings.SplitN(key, suffix, 2)[0], prefix)
	if path != ".gira/config.yaml" && path != ".gira/config.toml" {
		return nil, fmt.Errorf("unexpected config path: %s", path), true
	}
	content, err := os.ReadFile(filepath.Join(".gira", strings.TrimPrefix(path, ".gira/")))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("HTTP 404: Not Found"), true
		}
		return nil, err, true
	}
	return githubContentsFile(path, string(content)), nil, true
}

func recordedPolicyYAML() string {
	return `repo: StatPan/gira
finish_review_policy: required_or_recorded_independent
branch_policy:
  mode: github-flow
  development_base: dev
  production_base: main
recorded_review:
  allowed_recorders: [gira-review-bot]
  allowed_base_branches: [dev]
profiles:
  default:
    labels: []
`
}
