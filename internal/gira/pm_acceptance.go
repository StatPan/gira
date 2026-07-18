package gira

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	PMAcceptanceResultSchemaVersion = "pm-acceptance-result/v1"
	PMAcceptanceReportSchemaVersion = "pm-acceptance-report/v1"
)

const pmAcceptanceMarker = "<!-- gira:pm-acceptance-result/v1 -->"

type PMAcceptanceEvidenceMap struct {
	Subject         string   `json:"subject"`
	Status          string   `json:"status"`
	EvidenceRefs    []string `json:"evidence_refs"`
	SupportsOutcome bool     `json:"supports_outcome,omitempty"`
}

type PMAcceptanceResult struct {
	SchemaVersion string                    `json:"schema_version"`
	ID            string                    `json:"id"`
	Issue         int                       `json:"issue"`
	PullRequest   int                       `json:"pull_request,omitempty"`
	DeliveryState string                    `json:"delivery_state"`
	OutcomeState  string                    `json:"outcome_state"`
	Criteria      []PMAcceptanceEvidenceMap `json:"criteria"`
	Claims        []PMAcceptanceEvidenceMap `json:"claims"`
	Reason        string                    `json:"reason"`
	SourceRefs    []string                  `json:"source_refs"`
	RecordedAt    string                    `json:"recorded_at"`
	Supersedes    string                    `json:"supersedes,omitempty"`
}

type PMAcceptanceInput struct {
	Repo   RepoRef
	Ticket int
	Result PMAcceptanceResult
	DryRun bool
	Apply  bool
}

type PMAcceptanceDiagnostic struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Subject  string `json:"subject,omitempty"`
	Reason   string `json:"reason"`
	Repair   string `json:"repair"`
}

type PMAcceptanceTransition struct {
	Action  string `json:"action"`
	Profile string `json:"profile"`
	Reason  string `json:"reason"`
	Status  string `json:"status"`
}

type PMAcceptanceReport struct {
	Command       string                   `json:"command"`
	SchemaVersion string                   `json:"schema_version"`
	Repo          string                   `json:"repo"`
	Ticket        int                      `json:"ticket"`
	DryRun        bool                     `json:"dry_run"`
	Result        PMAcceptanceResult       `json:"result"`
	Diagnostics   []PMAcceptanceDiagnostic `json:"diagnostics"`
	Transitions   []PMAcceptanceTransition `json:"transitions"`
	Idempotent    bool                     `json:"idempotent"`
	NextStep      string                   `json:"next_step"`
	Approval      *ApprovalEvidence        `json:"approval,omitempty"`
}

func BuildPMAcceptanceReport(input PMAcceptanceInput, runner CommandRunner) (PMAcceptanceReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	report := PMAcceptanceReport{Command: "pm accept", SchemaVersion: PMAcceptanceReportSchemaVersion, Repo: input.Repo.FullName(), Ticket: input.Ticket, DryRun: input.DryRun, Diagnostics: []PMAcceptanceDiagnostic{}, Transitions: []PMAcceptanceTransition{}}
	if input.DryRun == input.Apply {
		return report, fmt.Errorf("exactly one of dry_run/apply is required")
	}
	if input.Ticket <= 0 {
		return report, fmt.Errorf("ticket must be > 0")
	}
	history, err := LoadPMAcceptanceHistory(input.Repo, input.Ticket, runner)
	if err != nil {
		return report, err
	}
	if input.Result.SchemaVersion != PMAcceptanceResultSchemaVersion {
		report.Result = input.Result
		report.Diagnostics = append(report.Diagnostics, PMAcceptanceDiagnostic{Severity: "error", Code: "PMA000_INVALID_SCHEMA", Subject: "schema_version", Reason: "acceptance input schema is missing or unsupported", Repair: "use pm-acceptance-result/v1"})
		return report, fmt.Errorf("PM acceptance result is invalid")
	}
	if input.Result.Issue > 0 && input.Result.Issue != input.Ticket {
		report.Result = input.Result
		report.Diagnostics = append(report.Diagnostics, PMAcceptanceDiagnostic{Severity: "error", Code: "PMA010_ISSUE_MISMATCH", Subject: "issue", Reason: "acceptance input targets a different issue", Repair: "use the same issue in input and --ticket"})
		return report, fmt.Errorf("PM acceptance result is invalid")
	}
	if input.Result.PullRequest <= 0 {
		report.Result = input.Result
		report.Diagnostics = append(report.Diagnostics, PMAcceptanceDiagnostic{Severity: "error", Code: "PMA007_MISSING_PR_SOURCE", Subject: "pull_request", Reason: "PM delivery acceptance requires a reviewed PR", Repair: "set pull_request and its source reference"})
		return report, fmt.Errorf("PM acceptance result is invalid")
	}
	pr, err := fetchAgentPromptPR(input.Repo, input.Result.PullRequest, runner)
	if err != nil {
		return report, fmt.Errorf("validate acceptance PR: %w", err)
	}
	if !hasClosingKeyword(pr.Body, input.Ticket) {
		report.Result = input.Result
		report.Diagnostics = append(report.Diagnostics, PMAcceptanceDiagnostic{Severity: "error", Code: "PMA011_PR_ISSUE_MISMATCH", Subject: "pull_request", Reason: "reviewed PR does not close the acceptance issue", Repair: "use the PR linked to the ticket or add its closing reference"})
		return report, fmt.Errorf("PM acceptance result is invalid")
	}
	result := normalizePMAcceptanceResult(input.Result, input.Ticket, history)
	report.Result = result
	report.Diagnostics = validatePMAcceptanceResult(result)
	report.Transitions = pmAcceptanceTransitions(result)
	if hasPMAcceptanceErrors(report.Diagnostics) {
		return report, fmt.Errorf("PM acceptance result is invalid")
	}
	for _, prior := range history {
		if prior.ID == result.ID {
			if pmAcceptanceEquivalent(prior, result) {
				report.Result = prior
				report.Idempotent = true
				if input.Apply {
					if err := persistPMAcceptanceTransition(report, input.Repo, runner); err != nil {
						return report, err
					}
				}
				report.NextStep = fmt.Sprintf("gira pm observe --repo %s --ticket %d", report.Repo, report.Ticket)
				return report, nil
			}
			return report, fmt.Errorf("acceptance result ID conflicts with different evidence")
		}
	}
	if input.Apply {
		body := renderPMAcceptanceResult(result)
		if _, err := runner.Run("gh", "issue", "comment", strconv.Itoa(input.Ticket), "--repo", input.Repo.FullName(), "--body", body); err != nil {
			return report, fmt.Errorf("persist PM acceptance result: %w", err)
		}
		if err := persistPMAcceptanceTransition(report, input.Repo, runner); err != nil {
			return report, err
		}
		for i := range report.Transitions {
			report.Transitions[i].Status = "applied"
		}
	}
	report.NextStep = fmt.Sprintf("gira pm observe --repo %s --ticket %d", report.Repo, report.Ticket)
	if input.DryRun {
		report.Approval = pmAcceptanceApproval(report)
	}
	return report, nil
}

func pmAcceptanceApproval(report PMAcceptanceReport) *ApprovalEvidence {
	dry := fmt.Sprintf("gira pm accept --repo %s --ticket %d --from-file RESULT.json --dry-run", report.Repo, report.Ticket)
	apply := fmt.Sprintf("gira pm accept --repo %s --ticket %d --from-file RESULT.json --apply", report.Repo, report.Ticket)
	actions := []ApprovalPlannedAction{{Action: "acceptance:append", Target: report.Result.ID, Detail: "persist immutable PM acceptance result"}, {Action: "pm_ledger:append", Target: "transition." + report.Result.ID, Detail: "persist typed acceptance learning transition"}}
	return &ApprovalEvidence{SchemaVersion: ApprovalPlanSchemaVersion, Capability: AdapterCapabilityApplyMutation, CanonicalCommand: "gira pm accept", DryRunCommand: dry, ApplyCommand: apply, Repo: report.Repo, Issue: report.Ticket, OutputSchema: PMAcceptanceReportSchemaVersion, PlannedActions: actions, Blockers: []string{}, Warnings: []string{}, PostApplyVerification: fmt.Sprintf("gira pm observe --repo %s --ticket %d --json", report.Repo, report.Ticket)}
}

func normalizePMAcceptanceResult(value PMAcceptanceResult, ticket int, history []PMAcceptanceResult) PMAcceptanceResult {
	value.SchemaVersion = PMAcceptanceResultSchemaVersion
	value.Issue = ticket
	value.DeliveryState = normalizePMLedgerKind(value.DeliveryState)
	value.OutcomeState = normalizePMLedgerKind(value.OutcomeState)
	value.Reason = strings.TrimSpace(value.Reason)
	value.SourceRefs = normalizePMLedgerRefs(value.SourceRefs)
	for i := range value.Criteria {
		normalizePMAcceptanceMap(&value.Criteria[i])
	}
	for i := range value.Claims {
		normalizePMAcceptanceMap(&value.Claims[i])
	}
	sort.Slice(value.Criteria, func(i, j int) bool { return value.Criteria[i].Subject < value.Criteria[j].Subject })
	sort.Slice(value.Claims, func(i, j int) bool { return value.Claims[i].Subject < value.Claims[j].Subject })
	value.RecordedAt = ""
	value.Supersedes = ""
	value.ID = ""
	value.ID = pmAcceptanceContentID(value)
	if prior, ok := currentPMAcceptance(history); ok && prior.ID != value.ID {
		value.Supersedes = prior.ID
	}
	value.RecordedAt = time.Now().UTC().Format(time.RFC3339)
	return value
}

func normalizePMAcceptanceMap(value *PMAcceptanceEvidenceMap) {
	value.Subject = strings.TrimSpace(value.Subject)
	value.Status = normalizePMLedgerKind(value.Status)
	value.EvidenceRefs = normalizePMLedgerRefs(value.EvidenceRefs)
}

func validatePMAcceptanceResult(value PMAcceptanceResult) []PMAcceptanceDiagnostic {
	out := []PMAcceptanceDiagnostic{}
	add := func(code, subject, reason, repair string) {
		out = append(out, PMAcceptanceDiagnostic{Severity: "error", Code: code, Subject: subject, Reason: reason, Repair: repair})
	}
	if !containsPMValue([]string{"accepted", "implementation_mismatch", "spec_repair", "follow_up", "risk_reduction", "inconclusive"}, value.DeliveryState) {
		add("PMA001_INVALID_DELIVERY_STATE", "delivery_state", "delivery acceptance state is invalid", "use an accepted PM delivery state")
	}
	if !containsPMValue([]string{"not_evaluated", "validated", "not_validated", "inconclusive"}, value.OutcomeState) {
		add("PMA002_INVALID_OUTCOME_STATE", "outcome_state", "product outcome state is invalid", "use not_evaluated, validated, not_validated, or inconclusive")
	}
	if value.Reason == "" || len(value.SourceRefs) == 0 {
		add("PMA003_MISSING_PROVENANCE", "result", "reason and source_refs are required", "link the verdict to inspectable PM and PR evidence")
	}
	if value.PullRequest <= 0 || !pmAcceptanceHasPRSource(value) {
		add("PMA007_MISSING_PR_SOURCE", "pull_request", "PM delivery acceptance is not tied to inspectable PR evidence", "set pull_request and add its PR or GitHub URL source reference")
	}
	if len(value.Criteria) == 0 {
		add("PMA004_MISSING_CRITERIA", "criteria", "no acceptance criterion mapping was supplied", "map every criterion to evidence")
	}
	seenSubjects := map[string]bool{}
	for _, item := range value.Criteria {
		if !containsPMValue([]string{"accepted", "mismatch", "missing", "inconclusive"}, item.Status) {
			add("PMA008_INVALID_EVIDENCE_STATUS", item.Subject, "criterion status is invalid", "use accepted, mismatch, missing, or inconclusive")
		}
	}
	for _, item := range value.Claims {
		if !containsPMValue([]string{"supported", "unsupported", "inconclusive"}, item.Status) {
			add("PMA008_INVALID_EVIDENCE_STATUS", item.Subject, "claim support status is invalid", "use supported, unsupported, or inconclusive")
		}
	}
	for _, item := range append(append([]PMAcceptanceEvidenceMap{}, value.Criteria...), value.Claims...) {
		if seenSubjects[item.Subject] {
			add("PMA012_DUPLICATE_SUBJECT", item.Subject, "criterion or claim subject is duplicated", "use one deterministic evidence mapping per subject")
		}
		seenSubjects[item.Subject] = true
		if item.Subject == "" || item.Status == "" || len(item.EvidenceRefs) == 0 {
			add("PMA005_UNSUPPORTED_CLAIM", item.Subject, "criterion or claim lacks status or evidence", "supply a source-linked evidence mapping")
		}
	}
	if value.DeliveryState == "accepted" {
		for _, item := range value.Criteria {
			if item.Status != "accepted" {
				add("PMA013_FALSE_DELIVERY_ACCEPTANCE", item.Subject, "delivery is accepted while a criterion is not accepted", "use mismatch, follow_up, risk_reduction, or inconclusive delivery state")
			}
		}
		for _, item := range value.Claims {
			if item.Status != "supported" {
				add("PMA013_FALSE_DELIVERY_ACCEPTANCE", item.Subject, "delivery is accepted while an implementation claim is unsupported", "use implementation_mismatch or an inconclusive delivery state")
			}
		}
	}
	if value.OutcomeState == "validated" && !pmAcceptanceHasOutcomeEvidence(value) {
		add("PMA006_DELIVERY_PROXY", "outcome_state", "merge, checks, tests, or implementation evidence alone cannot validate a product outcome", "link measurement, customer, research, experiment, metric, or data evidence that supports the outcome")
	}
	privacyValues := append([]string{value.Reason}, value.SourceRefs...)
	for _, item := range append(append([]PMAcceptanceEvidenceMap{}, value.Criteria...), value.Claims...) {
		privacyValues = append(privacyValues, item.Subject)
		privacyValues = append(privacyValues, item.EvidenceRefs...)
	}
	if containsSensitivePMContent(privacyValues) {
		add("PMA009_SENSITIVE_CONTENT", "result", "acceptance evidence resembles a secret or private transcript", "store only safe references or redacted summaries")
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Code != out[j].Code {
			return out[i].Code < out[j].Code
		}
		return out[i].Subject < out[j].Subject
	})
	return out
}

func pmAcceptanceHasPRSource(value PMAcceptanceResult) bool {
	number := strconv.Itoa(value.PullRequest)
	for _, ref := range value.SourceRefs {
		lower := strings.ToLower(ref)
		if (strings.HasPrefix(lower, "pr:") || strings.Contains(lower, "/pull/")) && strings.Contains(lower, number) {
			return true
		}
	}
	return false
}

func pmAcceptanceHasOutcomeEvidence(value PMAcceptanceResult) bool {
	for _, item := range append(append([]PMAcceptanceEvidenceMap{}, value.Criteria...), value.Claims...) {
		if !item.SupportsOutcome {
			continue
		}
		for _, ref := range item.EvidenceRefs {
			lower := strings.ToLower(ref)
			if strings.HasPrefix(lower, "measurement:") || strings.HasPrefix(lower, "metric:") || strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "research:") || strings.HasPrefix(lower, "customer:") || strings.HasPrefix(lower, "experiment:") {
				return true
			}
		}
	}
	return false
}

func pmAcceptanceTransitions(value PMAcceptanceResult) []PMAcceptanceTransition {
	out := []PMAcceptanceTransition{}
	add := func(action, profile, reason string) {
		out = append(out, PMAcceptanceTransition{Action: action, Profile: profile, Reason: reason, Status: "planned"})
	}
	switch value.DeliveryState {
	case "implementation_mismatch":
		add("create", "delivery", "implementation evidence does not satisfy the accepted specification")
	case "spec_repair":
		add("create", "decision", "the specification must be repaired before delivery continues")
	case "follow_up":
		add("create", "delivery", "accepted delivery left bounded follow-up work")
	case "risk_reduction":
		add("create", "experiment", "acceptance found unresolved product risk")
	case "inconclusive":
		add("defer", "validation", "delivery acceptance evidence is inconclusive")
	}
	switch value.OutcomeState {
	case "not_validated":
		add("replan", "discovery", "product outcome evidence did not validate the intended result")
	case "inconclusive":
		add("create", "measurement", "schedule observation or research before an outcome verdict")
	case "not_evaluated":
		add("defer", "measurement", "delivery acceptance does not imply product outcome validation")
	}
	return out
}

func persistPMAcceptanceTransition(report PMAcceptanceReport, repo RepoRef, runner CommandRunner) error {
	kind, status, conclusion := "evidence", "active", ""
	if report.Result.DeliveryState == "spec_repair" {
		kind, status = "decision", "review_due"
	}
	if report.Result.DeliveryState == "implementation_mismatch" || report.Result.OutcomeState == "not_validated" {
		kind, status, conclusion = "learning", "active", "invalidated"
	}
	if report.Result.OutcomeState == "inconclusive" {
		kind, status, conclusion = "learning", "active", "inconclusive"
	}
	recordedAt, _ := time.Parse(time.RFC3339, report.Result.RecordedAt)
	transitionSupersedes := ""
	if report.Result.Supersedes != "" {
		transitionSupersedes = "transition." + report.Result.Supersedes
	}
	_, err := BuildPMRecordReport(PMRecordInput{
		Repo: repo, Ticket: report.Ticket, ID: "transition." + report.Result.ID,
		Kind: kind, Text: report.Result.Reason,
		SourceRefs: append([]string{"acceptance:" + report.Result.ID}, report.Result.SourceRefs...),
		ActorKind:  "human", Status: status, Conclusion: conclusion, Supersedes: transitionSupersedes,
		RecordedAt: recordedAt, Apply: true,
	}, runner)
	if err != nil {
		return fmt.Errorf("persist acceptance learning transition: %w", err)
	}
	return nil
}

func LoadPMAcceptanceHistory(repo RepoRef, ticket int, runner CommandRunner) ([]PMAcceptanceResult, error) {
	raw, err := runner.Run("gh", "issue", "view", strconv.Itoa(ticket), "--repo", repo.FullName(), "--json", "comments")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Comments []struct {
			Body string `json:"body"`
		} `json:"comments"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	out := []PMAcceptanceResult{}
	for _, comment := range payload.Comments {
		if value, ok := parsePMAcceptanceResult(comment.Body); ok {
			out = append(out, value)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RecordedAt != out[j].RecordedAt {
			return out[i].RecordedAt < out[j].RecordedAt
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func currentPMAcceptance(history []PMAcceptanceResult) (PMAcceptanceResult, bool) {
	superseded := map[string]bool{}
	for _, value := range history {
		if value.Supersedes != "" {
			superseded[value.Supersedes] = true
		}
	}
	candidates := []PMAcceptanceResult{}
	for _, value := range history {
		if !superseded[value.ID] && !hasPMAcceptanceErrors(validatePMAcceptanceResult(value)) && value.ID == pmAcceptanceContentID(value) {
			candidates = append(candidates, value)
		}
	}
	if len(candidates) == 0 {
		return PMAcceptanceResult{}, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].RecordedAt != candidates[j].RecordedAt {
			return candidates[i].RecordedAt < candidates[j].RecordedAt
		}
		return candidates[i].ID < candidates[j].ID
	})
	return candidates[len(candidates)-1], true
}

func pmAcceptanceContentID(value PMAcceptanceResult) string {
	value.ID, value.RecordedAt, value.Supersedes = "", "", ""
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return "acceptance." + strconv.Itoa(value.Issue) + "." + hex.EncodeToString(sum[:8])
}

func renderPMAcceptanceResult(value PMAcceptanceResult) string {
	encoded, _ := json.MarshalIndent(value, "", "  ")
	return pmAcceptanceMarker + "\n\n```json\n" + string(encoded) + "\n```\n"
}
func parsePMAcceptanceResult(body string) (PMAcceptanceResult, bool) {
	if !strings.Contains(body, pmAcceptanceMarker) {
		return PMAcceptanceResult{}, false
	}
	start, end := strings.Index(body, "```json\n"), strings.LastIndex(body, "\n```")
	if start < 0 || end <= start {
		return PMAcceptanceResult{}, false
	}
	var value PMAcceptanceResult
	if json.Unmarshal([]byte(body[start+8:end]), &value) != nil || value.SchemaVersion != PMAcceptanceResultSchemaVersion {
		return PMAcceptanceResult{}, false
	}
	return value, true
}
func hasPMAcceptanceErrors(values []PMAcceptanceDiagnostic) bool {
	for _, value := range values {
		if value.Severity == "error" {
			return true
		}
	}
	return false
}
func pmAcceptanceEquivalent(a, b PMAcceptanceResult) bool {
	a.RecordedAt, b.RecordedAt = "", ""
	a.Supersedes, b.Supersedes = "", ""
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func FormatPMAcceptance(report PMAcceptanceReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "pm accept: #%d delivery=%s outcome=%s diagnostics=%d transitions=%d idempotent=%t\n", report.Ticket, report.Result.DeliveryState, report.Result.OutcomeState, len(report.Diagnostics), len(report.Transitions), report.Idempotent)
	for _, d := range report.Diagnostics {
		fmt.Fprintf(&b, "- %s %s: %s\n", d.Code, d.Subject, d.Reason)
	}
	for _, a := range report.Transitions {
		fmt.Fprintf(&b, "- %s profile=%s status=%s reason=%s\n", a.Action, a.Profile, a.Status, a.Reason)
	}
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}
