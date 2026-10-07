package gira

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	FinishReviewPolicyRequired            = "required"
	FinishReviewPolicyNone                = "none"
	FinishReviewPolicyRecordedIndependent = "required_or_recorded_independent"
	FinishReviewPolicyMissing             = "not_configured"
	FinishReviewPolicyNotEvaluated        = "not_evaluated"
	IndependentReviewReceiptSchema        = "gira-independent-review/v1"
	IndependentReviewReceiptMarker        = "<!-- gira:independent-review/v1 -->"
)

type FinishReviewPolicy struct {
	Value           string                `json:"value"`
	Source          string                `json:"source"`
	RecordedReview  *RecordedReviewConfig `json:"recorded_review,omitempty"`
	ValidationError string                `json:"validation_error,omitempty"`
}

type FinishReviewEvidence struct {
	Status      string `json:"status"`
	Decision    string `json:"decision,omitempty"`
	HeadSHA     string `json:"head_sha,omitempty"`
	BaseSHA     string `json:"base_sha,omitempty"`
	ApprovalSHA string `json:"approval_sha,omitempty"`
	Source      string `json:"source,omitempty"`
	Recorder    string `json:"recorder,omitempty"`
	ReviewerID  string `json:"reviewer_id,omitempty"`
	ReviewID    int64  `json:"review_id,omitempty"`
	Blocker     string `json:"blocker,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}

func loadFinishReviewPolicy(repo RepoRef) FinishReviewPolicy {
	path, cfg, err := loadLocalRepoInitConfig(repo)
	if err != nil {
		return FinishReviewPolicy{Value: FinishReviewPolicyMissing, Source: "invalid_repo_config", ValidationError: err.Error()}
	}
	if path == "" {
		return FinishReviewPolicy{Value: FinishReviewPolicyMissing, Source: "repo_config_missing"}
	}
	return finishReviewPolicyFromConfig(cfg, "repo_config:"+path)
}

func loadLocalRepoInitConfig(repo RepoRef) (string, InitConfig, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", InitConfig{}, fmt.Errorf("working directory unavailable: %w", err)
	}
	for {
		for _, path := range []string{filepath.Join(root, ".gira", "config.yaml"), filepath.Join(root, ".gira", "config.toml")} {
			if _, err := os.Stat(path); err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return path, InitConfig{}, fmt.Errorf("inspect config %q: %w", path, err)
			}
			cfg, err := LoadInitConfig(path)
			if err != nil {
				return path, InitConfig{}, err
			}
			if configuredRepo := strings.TrimSpace(cfg.Repo); configuredRepo != "" {
				parsed, parseErr := ParseRepoRef(configuredRepo)
				if parseErr != nil {
					return path, InitConfig{}, fmt.Errorf("invalid repository binding in %q", path)
				}
				if !sameRepoRef(parsed, repo) {
					continue
				}
			}
			return path, cfg, nil
		}
		parent := filepath.Dir(root)
		if parent == root {
			break
		}
		root = parent
	}
	return "", InitConfig{}, nil
}

func finishReviewPolicyFromConfig(cfg InitConfig, source string) FinishReviewPolicy {
	value := strings.ToLower(strings.TrimSpace(cfg.FinishReviewPolicy))
	if value == "" {
		value = FinishReviewPolicyMissing
	}
	policy := FinishReviewPolicy{Value: value, Source: source}
	if cfg.RecordedReview != nil {
		copy := *cfg.RecordedReview
		copy.AllowedRecorders = append([]string(nil), cfg.RecordedReview.AllowedRecorders...)
		copy.AllowedBaseBranches = append([]string(nil), cfg.RecordedReview.AllowedBaseBranches...)
		policy.RecordedReview = &copy
	}
	return policy
}

// resolveFinishReviewPolicy treats the immutable commit at the actual PR base
// as the sole policy authority. Local config is parsed only to fail closed on
// malformed checkout state; it never authorizes a finish or weakens the
// policy committed at the base.
func resolveFinishReviewPolicy(repo RepoRef, status DevPRStatusResult, runner CommandRunner) FinishReviewPolicy {
	local := loadFinishReviewPolicy(repo)
	if local.ValidationError != "" {
		return local
	}
	baseSHA := strings.TrimSpace(status.BaseSHA)
	if !isFullCommitSHA(baseSHA) {
		return FinishReviewPolicy{Value: FinishReviewPolicyMissing, Source: "base_commit_unavailable", ValidationError: "the pull request base commit SHA is unavailable; a weaker or alternate local policy cannot be trusted"}
	}
	base, err := loadFinishReviewPolicyAtCommit(repo, baseSHA, runner)
	if err != nil {
		return FinishReviewPolicy{Value: FinishReviewPolicyMissing, Source: "base_commit:" + baseSHA + ":config_unavailable", ValidationError: err.Error()}
	}
	if base.Value == FinishReviewPolicyMissing {
		base.Source += ":policy_missing"
	}
	return base
}

func loadFinishReviewPolicyAtCommit(repo RepoRef, commitSHA string, runner CommandRunner) (FinishReviewPolicy, error) {
	if runner == nil {
		return FinishReviewPolicy{}, fmt.Errorf("GitHub reader is unavailable")
	}
	commitSHA = strings.TrimSpace(commitSHA)
	if !isFullCommitSHA(commitSHA) {
		return FinishReviewPolicy{}, fmt.Errorf("base commit SHA is invalid")
	}
	for _, path := range []string{".gira/config.yaml", ".gira/config.toml"} {
		out, err := runner.Run("gh", "api", fmt.Sprintf("repos/%s/contents/%s", repo.FullName(), path), "--method", "GET", "-f", "ref="+commitSHA)
		if err != nil {
			if isGitHubNotFound(err) {
				continue
			}
			return FinishReviewPolicy{}, fmt.Errorf("read %s at PR base %s: %w", path, commitSHA, err)
		}
		var file struct {
			Type     string `json:"type"`
			Path     string `json:"path"`
			Encoding string `json:"encoding"`
			Content  string `json:"content"`
		}
		if err := json.Unmarshal(out, &file); err != nil {
			return FinishReviewPolicy{}, fmt.Errorf("decode base config metadata: %w", err)
		}
		if file.Type != "file" || file.Path != path || file.Encoding != "base64" || strings.TrimSpace(file.Content) == "" {
			return FinishReviewPolicy{}, fmt.Errorf("base config %s is not a complete file response", path)
		}
		content, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(file.Content), ""))
		if err != nil {
			return FinishReviewPolicy{}, fmt.Errorf("decode base config content: %w", err)
		}
		cfg, err := parseInitConfigContent(path, content)
		if err != nil {
			return FinishReviewPolicy{}, err
		}
		if configuredRepo := strings.TrimSpace(cfg.Repo); configuredRepo != "" {
			parsed, parseErr := ParseRepoRef(configuredRepo)
			if parseErr != nil || !sameRepoRef(parsed, repo) {
				return FinishReviewPolicy{}, fmt.Errorf("base config repository binding does not match %s", repo.FullName())
			}
		}
		return finishReviewPolicyFromConfig(cfg, "base_commit:"+commitSHA+":"+path), nil
	}
	return FinishReviewPolicy{Value: FinishReviewPolicyMissing, Source: "base_commit:" + commitSHA + ":repo_config_missing"}, nil
}

func isGitHubNotFound(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "http 404") || strings.Contains(message, "404 not found") || strings.Contains(message, "status: 404")
}

func finishReviewEvidence(repo RepoRef, status DevPRStatusResult, policy FinishReviewPolicy, runner CommandRunner) FinishReviewEvidence {
	evidence := FinishReviewEvidence{Decision: strings.ToUpper(strings.TrimSpace(status.ReviewDecision)), HeadSHA: strings.TrimSpace(status.HeadSHA), BaseSHA: strings.TrimSpace(status.BaseSHA)}
	if policy.ValidationError != "" {
		return blockedFinishReview(evidence, "review_policy_invalid", "Repair the repository finish-review configuration and retry.")
	}
	if policy.Value == FinishReviewPolicyNone {
		evidence.Status = "not_required"
		return evidence
	}
	if policy.Value == FinishReviewPolicyMissing {
		return blockedFinishReview(evidence, "review_policy_not_configured", "Set an explicit finish_review_policy in the repository config.")
	}
	if policy.Value == FinishReviewPolicyRecordedIndependent {
		return recordedIndependentReviewEvidence(repo, status, policy, runner, evidence)
	}
	if policy.Value != FinishReviewPolicyRequired {
		return blockedFinishReview(evidence, "review_policy_invalid", "Select a supported finish-review policy and retry.")
	}
	if evidence.Decision != "APPROVED" {
		return blockedFinishReview(evidence, "review_required_but_absent", "Request and record an approving review for the current PR head.")
	}
	if evidence.HeadSHA == "" {
		return blockedFinishReview(evidence, "review_evidence_unavailable", "Restore the current PR head SHA in GitHub metadata and rerun ticket finish.")
	}
	reviews, err := fetchFinishReviews(repo, status.PRNumber, runner)
	if err != nil {
		return blockedFinishReview(evidence, "review_evidence_unavailable", "Restore GitHub review-read access and rerun ticket finish.")
	}
	for _, review := range reviews {
		if strings.EqualFold(strings.TrimSpace(review.State), "APPROVED") && strings.EqualFold(strings.TrimSpace(review.CommitID), evidence.HeadSHA) {
			evidence.Status = "approved"
			evidence.Source = "github_approval"
			evidence.ApprovalSHA = strings.TrimSpace(review.CommitID)
			return evidence
		}
	}
	return blockedFinishReview(evidence, "review_approval_stale", "Request a new approving review after the current PR head change.")
}

func blockedFinishReview(evidence FinishReviewEvidence, blocker, remediation string) FinishReviewEvidence {
	evidence.Status = "blocked"
	evidence.Blocker = blocker
	evidence.Remediation = remediation
	return evidence
}

func fetchFinishReviews(repo RepoRef, prNumber int, runner CommandRunner) ([]finishReview, error) {
	if runner == nil || prNumber <= 0 {
		return nil, fmt.Errorf("review reader and pull request are required")
	}
	out, err := runner.Run("gh", "api", fmt.Sprintf("repos/%s/pulls/%d/reviews", repo.FullName(), prNumber), "--paginate", "--slurp")
	if err != nil {
		return nil, err
	}
	reviews, err := decodePaginatedReviews(out)
	if err != nil {
		return nil, err
	}
	return reviews, nil
}

func recordedIndependentReviewEvidence(repo RepoRef, status DevPRStatusResult, policy FinishReviewPolicy, runner CommandRunner, evidence FinishReviewEvidence) FinishReviewEvidence {
	if status.PRNumber <= 0 || !isFullCommitSHA(evidence.HeadSHA) || !isFullCommitSHA(evidence.BaseSHA) {
		return blockedFinishReview(evidence, "review_evidence_unavailable", "Restore exact PR head and base commit SHAs before evaluating review evidence.")
	}
	if policy.RecordedReview == nil || len(policy.RecordedReview.AllowedRecorders) == 0 {
		return blockedFinishReview(evidence, "review_policy_invalid", "Repair the recorded_review policy and retry.")
	}
	reviews, err := fetchFinishReviews(repo, status.PRNumber, runner)
	if err != nil {
		return blockedFinishReview(evidence, "review_evidence_unavailable", "Restore GitHub review-read access and retry.")
	}
	if err := sortAndValidateFinishReviews(reviews); err != nil {
		return blockedFinishReview(evidence, "review_evidence_unavailable", "Restore complete GitHub review IDs and submission timestamps before retrying.")
	}
	if hasActiveChangesRequestedReview(reviews) {
		return blockedFinishReview(evidence, "review_changes_requested", "Resolve every active CHANGES_REQUESTED review before finish.")
	}
	baseRef := strings.TrimSpace(status.Binding.BaseRef)
	if baseRef == "" || !containsExactTrimmed(policy.RecordedReview.AllowedBaseBranches, baseRef) {
		if evidence.Decision == "APPROVED" && hasCurrentHeadApproval(reviews, evidence.HeadSHA) {
			evidence.Status = "approved"
			evidence.Source = "github_approval"
			evidence.ApprovalSHA = evidence.HeadSHA
			return evidence
		}
		if evidence.Decision != "APPROVED" {
			return blockedFinishReview(evidence, "review_required_but_absent", "Request and record a native approving review for the current PR head.")
		}
		if !isFullCommitSHA(evidence.HeadSHA) {
			return blockedFinishReview(evidence, "review_evidence_unavailable", "Restore the current PR head SHA before verifying its native review.")
		}
		return blockedFinishReview(evidence, "review_approval_stale", "Request a native approving review after the current PR head change.")
	}
	resolved, stale, reviewID, recorder, reviewerID := evaluateIndependentReviewReceipts(repo, status, policy, reviews)
	if resolved.Blocker != "" {
		return blockedFinishReview(evidence, resolved.Blocker, resolved.Remediation)
	}
	if resolved.Status == "independent_recorded" {
		evidence.Status = resolved.Status
		evidence.ReviewID = resolved.ReviewID
		evidence.Recorder = resolved.Recorder
		evidence.ReviewerID = resolved.ReviewerID
		evidence.Source = "recorded_independent_review"
		return evidence
	}
	if evidence.Decision == "APPROVED" && hasCurrentHeadApproval(reviews, evidence.HeadSHA) {
		evidence.Status = "approved"
		evidence.Source = "github_approval"
		evidence.ApprovalSHA = evidence.HeadSHA
		return evidence
	}
	if stale {
		return blockedFinishReview(evidence, "recorded_review_stale", "Record a new independent review for the current PR head and base.")
	}
	if reviewID > 0 || recorder != "" || reviewerID != "" {
		evidence.ReviewID = reviewID
		evidence.Recorder = recorder
		evidence.ReviewerID = reviewerID
	}
	return blockedFinishReview(evidence, "review_required_but_absent", "Record a versioned independent review on the current PR head using an allowlisted GitHub recorder.")
}

func hasCurrentHeadApproval(reviews []finishReview, headSHA string) bool {
	for _, review := range reviews {
		if strings.EqualFold(strings.TrimSpace(review.State), "APPROVED") && strings.EqualFold(strings.TrimSpace(review.CommitID), headSHA) {
			return true
		}
	}
	return false
}

func hasActiveChangesRequestedReview(reviews []finishReview) bool {
	latest := map[string]finishReview{}
	for _, review := range reviews {
		state := strings.ToUpper(strings.TrimSpace(review.State))
		if state != "APPROVED" && state != "CHANGES_REQUESTED" && state != "DISMISSED" {
			continue
		}
		login := strings.ToLower(strings.TrimSpace(review.User.Login))
		if login == "" {
			if state == "CHANGES_REQUESTED" {
				return true
			}
			continue
		}
		latest[login] = review
	}
	for _, review := range latest {
		if strings.EqualFold(strings.TrimSpace(review.State), "CHANGES_REQUESTED") {
			return true
		}
	}
	return false
}

func sortAndValidateFinishReviews(reviews []finishReview) error {
	type orderedReview struct {
		review    finishReview
		submitted time.Time
		valid     bool
	}
	ordered := make([]orderedReview, len(reviews))
	ids := map[int64]struct{}{}
	for index, review := range reviews {
		ordered[index].review = review
		state := strings.ToUpper(strings.TrimSpace(review.State))
		relevant := state == "APPROVED" || state == "CHANGES_REQUESTED" || state == "DISMISSED" || strings.HasPrefix(strings.TrimSpace(review.Body), IndependentReviewReceiptMarker)
		if !relevant {
			continue
		}
		if review.ID <= 0 {
			return fmt.Errorf("relevant review has no positive ID")
		}
		if _, duplicate := ids[review.ID]; duplicate {
			return fmt.Errorf("relevant review ID is duplicated")
		}
		ids[review.ID] = struct{}{}
		submittedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(review.SubmittedAt))
		if err != nil {
			return fmt.Errorf("relevant review has invalid submitted_at")
		}
		ordered[index].submitted = submittedAt
		ordered[index].valid = true
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].valid != ordered[j].valid {
			return ordered[i].valid
		}
		if ordered[i].valid && !ordered[i].submitted.Equal(ordered[j].submitted) {
			return ordered[i].submitted.Before(ordered[j].submitted)
		}
		return ordered[i].review.ID < ordered[j].review.ID
	})
	for index := range ordered {
		reviews[index] = ordered[index].review
	}
	return nil
}

type IndependentReviewIdentity struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	RunRef string `json:"run_ref"`
}

type IndependentReviewFinding struct {
	ID           string   `json:"id"`
	Severity     string   `json:"severity"`
	Status       string   `json:"status"`
	Summary      string   `json:"summary"`
	EvidenceRefs []string `json:"evidence_refs"`
}

type IndependentReviewReceipt struct {
	SchemaVersion       string                     `json:"schema_version"`
	Repository          string                     `json:"repository"`
	PullRequest         int                        `json:"pull_request"`
	HeadSHA             string                     `json:"head_sha"`
	BaseRef             string                     `json:"base_ref"`
	BaseSHA             string                     `json:"base_sha"`
	Verdict             string                     `json:"verdict"`
	Reviewer            IndependentReviewIdentity  `json:"reviewer"`
	Implementer         IndependentReviewIdentity  `json:"implementer"`
	Recorder            string                     `json:"recorder"`
	Independent         bool                       `json:"independent"`
	EvidenceRefs        []string                   `json:"evidence_refs"`
	Findings            []IndependentReviewFinding `json:"findings"`
	SupersedesReviewIDs []int64                    `json:"supersedes_review_ids,omitempty"`
	Limitations         []string                   `json:"limitations"`
}

type independentReviewEvaluation struct {
	Status      string
	Blocker     string
	Remediation string
	ReviewID    int64
	Recorder    string
	ReviewerID  string
}

func evaluateIndependentReviewReceipts(repo RepoRef, status DevPRStatusResult, policy FinishReviewPolicy, reviews []finishReview) (independentReviewEvaluation, bool, int64, string, string) {
	if policy.RecordedReview == nil || len(policy.RecordedReview.AllowedRecorders) == 0 {
		return independentReviewEvaluation{Blocker: "review_policy_invalid", Remediation: "Repair the recorded_review policy and retry."}, false, 0, "", ""
	}
	allowlist := map[string]struct{}{}
	for _, login := range policy.RecordedReview.AllowedRecorders {
		allowlist[strings.ToLower(strings.TrimSpace(login))] = struct{}{}
	}
	type findingIdentity struct {
		recorder  string
		reviewer  string
		findingID string
	}
	unresolvedFindings := map[findingIdentity]struct{}{}
	unresolvedMalformed := map[int64]string{}
	latest := independentReviewEvaluation{}
	stale := false
	var lastReviewID int64
	var lastRecorder, lastReviewerID string
	for _, review := range reviews {
		receipt, marked, err := decodeIndependentReviewReceipt(review.Body)
		if !marked {
			continue
		}
		login := strings.TrimSpace(review.User.Login)
		if _, trusted := allowlist[strings.ToLower(login)]; !trusted {
			continue
		}
		if review.ID <= 0 {
			unresolvedMalformed[0] = strings.ToLower(login)
			latest = independentReviewEvaluation{Blocker: "recorded_review_invalid", Remediation: "GitHub did not return a stable review ID for the recorded review."}
			continue
		}
		lastReviewID, lastRecorder = review.ID, login
		if strings.ToUpper(strings.TrimSpace(review.State)) != "COMMENTED" || err != nil || validateIndependentReviewReceiptHistory(repo, status.PRNumber, policy, login, receipt) != nil || !strings.EqualFold(strings.TrimSpace(review.CommitID), strings.TrimSpace(receipt.HeadSHA)) {
			unresolvedMalformed[review.ID] = strings.ToLower(login)
			latest = independentReviewEvaluation{Blocker: "recorded_review_invalid", Remediation: "Replace the malformed recorded review with a valid versioned receipt that explicitly supersedes its review ID."}
			continue
		}
		lastReviewerID = strings.TrimSpace(receipt.Reviewer.ID)
		current := strings.EqualFold(strings.TrimSpace(receipt.HeadSHA), strings.TrimSpace(status.HeadSHA)) &&
			strings.EqualFold(strings.TrimSpace(receipt.BaseSHA), strings.TrimSpace(status.BaseSHA)) &&
			strings.TrimSpace(receipt.BaseRef) == strings.TrimSpace(status.Binding.BaseRef)
		if current && validateIndependentReviewReceipt(repo, status, policy, login, receipt) != nil {
			unresolvedMalformed[review.ID] = strings.ToLower(login)
			latest = independentReviewEvaluation{Blocker: "recorded_review_invalid", Remediation: "Replace the malformed recorded review with a valid versioned receipt that explicitly supersedes its review ID."}
			continue
		}
		if !current {
			stale = true
		}
		stagedMalformed := make(map[int64]string, len(unresolvedMalformed))
		for id, owner := range unresolvedMalformed {
			stagedMalformed[id] = owner
		}
		stagedFindings := make(map[findingIdentity]struct{}, len(unresolvedFindings))
		for key := range unresolvedFindings {
			stagedFindings[key] = struct{}{}
		}
		invalidSupersession := false
		for _, supersededID := range receipt.SupersedesReviewIDs {
			owner, exists := stagedMalformed[supersededID]
			if !exists || owner != strings.ToLower(login) {
				invalidSupersession = true
				break
			}
		}
		if invalidSupersession {
			unresolvedMalformed[review.ID] = strings.ToLower(login)
			latest = independentReviewEvaluation{Blocker: "recorded_review_invalid", Remediation: "A recorded review may supersede only an earlier malformed receipt from the same trusted recorder."}
			continue
		}
		for _, supersededID := range receipt.SupersedesReviewIDs {
			delete(stagedMalformed, supersededID)
		}
		for _, finding := range receipt.Findings {
			if finding.Severity != "blocking" {
				continue
			}
			findingKey := findingIdentity{
				recorder:  strings.ToLower(login),
				reviewer:  strings.ToLower(strings.TrimSpace(receipt.Reviewer.ID)),
				findingID: finding.ID,
			}
			if finding.Status == "open" {
				stagedFindings[findingKey] = struct{}{}
			} else if finding.Status == "resolved" {
				if _, exists := stagedFindings[findingKey]; !exists {
					unresolvedMalformed[review.ID] = strings.ToLower(login)
					latest = independentReviewEvaluation{Blocker: "recorded_review_invalid", Remediation: "A resolution must identify an existing open finding from the same trusted recorder and reviewer."}
					invalidSupersession = true
					break
				}
				delete(stagedFindings, findingKey)
			}
		}
		if invalidSupersession {
			continue
		}
		unresolvedMalformed = stagedMalformed
		unresolvedFindings = stagedFindings
		if strings.EqualFold(receipt.Verdict, "BLOCKED") {
			if current {
				latest = independentReviewEvaluation{Blocker: "recorded_review_blocked", Remediation: "Resolve the blocking findings in the latest recorded review, then submit a new review with evidence refs."}
			}
			continue
		}
		if current {
			latest = independentReviewEvaluation{Status: "independent_recorded", ReviewID: review.ID, Recorder: login, ReviewerID: receipt.Reviewer.ID}
		}
	}
	if len(unresolvedMalformed) > 0 {
		return independentReviewEvaluation{Blocker: "recorded_review_invalid", Remediation: "A trusted malformed review receipt remains unresolved; supersede it explicitly with a valid receipt."}, stale, lastReviewID, lastRecorder, lastReviewerID
	}
	if len(unresolvedFindings) > 0 {
		return independentReviewEvaluation{Blocker: "recorded_review_blocked", Remediation: "Resolve every blocking finding with evidence in a later recorded review."}, stale, lastReviewID, lastRecorder, lastReviewerID
	}
	return latest, stale, lastReviewID, lastRecorder, lastReviewerID
}

func decodeIndependentReviewReceipt(body string) (IndependentReviewReceipt, bool, error) {
	trimmed := strings.TrimSpace(body)
	if !strings.HasPrefix(trimmed, IndependentReviewReceiptMarker) {
		return IndependentReviewReceipt{}, false, nil
	}
	raw := strings.TrimSpace(strings.TrimPrefix(trimmed, IndependentReviewReceiptMarker))
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var receipt IndependentReviewReceipt
	if err := decoder.Decode(&receipt); err != nil {
		return receipt, true, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			return receipt, true, fmt.Errorf("receipt contains trailing JSON")
		}
		return receipt, true, err
	}
	return receipt, true, nil
}

func validateIndependentReviewReceiptHistory(repo RepoRef, prNumber int, policy FinishReviewPolicy, recorder string, receipt IndependentReviewReceipt) error {
	if receipt.SchemaVersion != IndependentReviewReceiptSchema {
		return fmt.Errorf("unsupported receipt schema")
	}
	receiptRepo, err := ParseRepoRef(strings.TrimSpace(receipt.Repository))
	if err != nil || !sameRepoRef(receiptRepo, repo) || receipt.PullRequest != prNumber {
		return fmt.Errorf("receipt repository or PR does not match")
	}
	if !isFullCommitSHA(strings.TrimSpace(receipt.HeadSHA)) || !isFullCommitSHA(strings.TrimSpace(receipt.BaseSHA)) || strings.TrimSpace(receipt.BaseRef) == "" {
		return fmt.Errorf("receipt head or base binding is incomplete")
	}
	if !strings.EqualFold(strings.TrimSpace(receipt.Recorder), recorder) || !containsFold(policy.RecordedReview.AllowedRecorders, recorder) {
		return fmt.Errorf("receipt recorder is not trusted")
	}
	if !receipt.Independent || strings.TrimSpace(receipt.Reviewer.ID) == "" || strings.TrimSpace(receipt.Implementer.ID) == "" || strings.EqualFold(strings.TrimSpace(receipt.Reviewer.ID), strings.TrimSpace(receipt.Implementer.ID)) {
		return fmt.Errorf("reviewer and implementer are not explicitly independent")
	}
	if (receipt.Reviewer.Kind != "human" && receipt.Reviewer.Kind != "ai") || (receipt.Implementer.Kind != "human" && receipt.Implementer.Kind != "ai") || strings.TrimSpace(receipt.Reviewer.RunRef) == "" || strings.TrimSpace(receipt.Implementer.RunRef) == "" || strings.EqualFold(strings.TrimSpace(receipt.Reviewer.RunRef), strings.TrimSpace(receipt.Implementer.RunRef)) {
		return fmt.Errorf("reviewer and implementer run references are missing or not distinct")
	}
	if receipt.Verdict != "GO" && receipt.Verdict != "BLOCKED" {
		return fmt.Errorf("review verdict is invalid")
	}
	if len(receipt.EvidenceRefs) == 0 || !validEvidenceRefs(receipt.EvidenceRefs) || !nonEmptyStrings(receipt.Limitations) {
		return fmt.Errorf("inspectable evidence refs and explicit limitations are required")
	}
	seen := map[string]struct{}{}
	hasOpenBlocking := false
	for _, finding := range receipt.Findings {
		id := strings.TrimSpace(finding.ID)
		if id == "" || strings.TrimSpace(finding.Summary) == "" || (finding.Severity != "blocking" && finding.Severity != "non_blocking") || (finding.Status != "open" && finding.Status != "resolved") {
			return fmt.Errorf("review finding is incomplete")
		}
		if _, duplicate := seen[id]; duplicate {
			return fmt.Errorf("review finding IDs must be unique")
		}
		seen[id] = struct{}{}
		if finding.Status == "resolved" && !validEvidenceRefs(finding.EvidenceRefs) {
			return fmt.Errorf("resolved findings require inspectable evidence refs")
		}
		if finding.Severity == "blocking" && finding.Status == "open" {
			hasOpenBlocking = true
		}
	}
	if receipt.Verdict == "GO" && hasOpenBlocking {
		return fmt.Errorf("GO receipt contains unresolved blocking findings")
	}
	if receipt.Verdict == "BLOCKED" && !hasOpenBlocking {
		return fmt.Errorf("BLOCKED receipt must include an open blocking finding")
	}
	seenSuperseded := map[int64]struct{}{}
	for _, id := range receipt.SupersedesReviewIDs {
		if id <= 0 {
			return fmt.Errorf("superseded review IDs must be positive")
		}
		if _, duplicate := seenSuperseded[id]; duplicate {
			return fmt.Errorf("superseded review IDs must be unique")
		}
		seenSuperseded[id] = struct{}{}
	}
	return nil
}

func validateIndependentReviewReceipt(repo RepoRef, status DevPRStatusResult, policy FinishReviewPolicy, recorder string, receipt IndependentReviewReceipt) error {
	if err := validateIndependentReviewReceiptHistory(repo, status.PRNumber, policy, recorder, receipt); err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(receipt.HeadSHA), strings.TrimSpace(status.HeadSHA)) || !strings.EqualFold(strings.TrimSpace(receipt.BaseSHA), strings.TrimSpace(status.BaseSHA)) || strings.TrimSpace(receipt.BaseRef) != strings.TrimSpace(status.Binding.BaseRef) {
		return fmt.Errorf("receipt head or base does not match current pull request")
	}
	return nil
}

func validEvidenceRefs(refs []string) bool {
	if len(refs) == 0 {
		return false
	}
	for _, ref := range refs {
		parsed, err := url.Parse(strings.TrimSpace(ref))
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
			return false
		}
	}
	return true
}

func nonEmptyStrings(values []string) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	return true
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func containsExactTrimmed(values []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func isFullCommitSHA(value string) bool {
	if len(value) != 40 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

// IsFullCommitSHA reports whether value is a complete 40-character commit ID.
// It intentionally does not trim whitespace or accept abbreviated object IDs.
func IsFullCommitSHA(value string) bool {
	return isFullCommitSHA(value)
}

type finishReview struct {
	ID          int64  `json:"id"`
	State       string `json:"state"`
	CommitID    string `json:"commit_id"`
	Body        string `json:"body"`
	SubmittedAt string `json:"submitted_at"`
	User        struct {
		Login string `json:"login"`
	} `json:"user"`
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
