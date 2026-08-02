package gira

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	FinishReviewPolicyRequired = "required"
	FinishReviewPolicyNone     = "none"
	FinishReviewPolicyMissing  = "not_configured"
)

type FinishReviewPolicy struct {
	Value  string `json:"value"`
	Source string `json:"source"`
}

type FinishReviewEvidence struct {
	Status      string `json:"status"`
	Decision    string `json:"decision,omitempty"`
	HeadSHA     string `json:"head_sha,omitempty"`
	ApprovalSHA string `json:"approval_sha,omitempty"`
	Blocker     string `json:"blocker,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}

func loadFinishReviewPolicy(repo RepoRef) FinishReviewPolicy {
	root, err := os.Getwd()
	if err != nil {
		return FinishReviewPolicy{Value: FinishReviewPolicyMissing, Source: "working_directory_unavailable"}
	}
	for {
		for _, path := range []string{filepath.Join(root, ".gira", "config.yaml"), filepath.Join(root, ".gira", "config.toml")} {
			if _, err := os.Stat(path); err != nil {
				continue
			}
			cfg, err := LoadInitConfig(path)
			if err != nil {
				continue
			}
			if configuredRepo := strings.TrimSpace(cfg.Repo); configuredRepo != "" {
				parsed, parseErr := ParseRepoRef(configuredRepo)
				if parseErr != nil || !sameRepoRef(parsed, repo) {
					continue
				}
			}
			if value := strings.ToLower(strings.TrimSpace(cfg.FinishReviewPolicy)); value != "" {
				return FinishReviewPolicy{Value: value, Source: "repo_config:" + path}
			}
			return FinishReviewPolicy{Value: FinishReviewPolicyMissing, Source: "repo_config:" + path}
		}
		parent := filepath.Dir(root)
		if parent == root {
			break
		}
		root = parent
	}
	return FinishReviewPolicy{Value: FinishReviewPolicyMissing, Source: "repo_config_missing"}
}

func finishReviewEvidence(repo RepoRef, status DevPRStatusResult, policy FinishReviewPolicy, runner CommandRunner) FinishReviewEvidence {
	evidence := FinishReviewEvidence{Decision: strings.ToUpper(strings.TrimSpace(status.ReviewDecision)), HeadSHA: strings.TrimSpace(status.HeadSHA)}
	if policy.Value == FinishReviewPolicyNone {
		evidence.Status = "not_required"
		return evidence
	}
	if policy.Value == FinishReviewPolicyMissing {
		evidence.Status = "blocked"
		evidence.Blocker = "review_policy_not_configured"
		evidence.Remediation = "Set finish_review_policy: required or none in .gira/config.yaml."
		return evidence
	}
	if evidence.Decision != "APPROVED" {
		evidence.Status = "blocked"
		evidence.Blocker = "review_required_but_absent"
		evidence.Remediation = "Request and record an approving review for the current PR head."
		return evidence
	}
	if evidence.HeadSHA == "" {
		evidence.Status = "blocked"
		evidence.Blocker = "review_evidence_unavailable"
		evidence.Remediation = "Restore the current PR head SHA in GitHub metadata and rerun ticket finish."
		return evidence
	}
	out, err := runner.Run("gh", "api", fmt.Sprintf("repos/%s/pulls/%d/reviews", repo.FullName(), status.PRNumber), "--paginate", "--slurp")
	if err != nil {
		evidence.Status = "blocked"
		evidence.Blocker = "review_evidence_unavailable"
		evidence.Remediation = "Restore GitHub review-read access and rerun ticket finish."
		return evidence
	}
	reviews, err := decodePaginatedReviews(out)
	if err != nil {
		evidence.Status = "blocked"
		evidence.Blocker = "review_evidence_unavailable"
		evidence.Remediation = "Restore readable GitHub review evidence and rerun ticket finish."
		return evidence
	}
	for _, review := range reviews {
		if strings.EqualFold(strings.TrimSpace(review.State), "APPROVED") && strings.EqualFold(strings.TrimSpace(review.CommitID), evidence.HeadSHA) {
			evidence.Status = "approved"
			evidence.ApprovalSHA = strings.TrimSpace(review.CommitID)
			return evidence
		}
	}
	evidence.Status = "blocked"
	evidence.Blocker = "review_approval_stale"
	evidence.Remediation = "Request a new approving review after the current PR head change."
	return evidence
}

type finishReview struct {
	State    string `json:"state"`
	CommitID string `json:"commit_id"`
}

func decodePaginatedReviews(out []byte) ([]finishReview, error) {
	var pages [][]finishReview
	if err := json.Unmarshal(out, &pages); err == nil {
		reviews := make([]finishReview, 0)
		for _, page := range pages {
			reviews = append(reviews, page...)
		}
		return reviews, nil
	}
	var reviews []finishReview
	if err := json.Unmarshal(out, &reviews); err != nil {
		return nil, err
	}
	return reviews, nil
}
