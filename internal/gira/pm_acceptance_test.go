package gira

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type pmAcceptanceRunner struct {
	comments           []string
	failTransitionOnce bool
}

func (r *pmAcceptanceRunner) Run(name string, args ...string) ([]byte, error) {
	call := name + " " + strings.Join(args, " ")
	items := []map[string]any{}
	for i, body := range r.comments {
		items = append(items, map[string]any{"body": body, "url": fmt.Sprintf("https://example/c/%d", i+1), "createdAt": fmt.Sprintf("2026-07-19T00:00:%02dZ", i), "author": map[string]string{"login": "pm"}})
	}
	switch {
	case call == "gh issue view 42 --repo OWNER/repo --json comments":
		out, _ := json.Marshal(map[string]any{"comments": items})
		return out, nil
	case call == "gh issue view 42 --repo OWNER/repo --json number,title,body,url,comments":
		out, _ := json.Marshal(map[string]any{"number": 42, "title": "Acceptance target", "body": PMStateMarker, "url": "https://example/42", "comments": items})
		return out, nil
	case call == "gh pr view 99 --repo OWNER/repo --json number,title,body,state,url,headRefName,baseRefName,reviewDecision,isDraft,mergeStateStatus,statusCheckRollup":
		return []byte(`{"number":99,"title":"Acceptance PR","body":"Closes #42","state":"OPEN","url":"https://example/pr/99","headRefName":"issue-42","baseRefName":"main","headRefOid":"head220","isDraft":false,"statusCheckRollup":[]}`), nil
	case call == "gh pr diff 99 --repo OWNER/repo --name-only":
		return []byte("internal/gira/change.go\n"), nil
	case strings.HasPrefix(call, "gh issue comment 42 --repo OWNER/repo --body "):
		if r.failTransitionOnce && strings.Contains(args[len(args)-1], pmLedgerRecordMarker) {
			r.failTransitionOnce = false
			return nil, fmt.Errorf("temporary transition failure")
		}
		r.comments = append(r.comments, args[len(args)-1])
		return []byte(`{}`), nil
	default:
		return nil, fmt.Errorf("unexpected command: %s", call)
	}
}

func TestPMAcceptanceRetryRepairsMissingTransition(t *testing.T) {
	runner := &pmAcceptanceRunner{failTransitionOnce: true}
	input := PMAcceptanceInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Ticket: 42, Result: validPMAcceptanceResult(), Apply: true}
	if _, err := BuildPMAcceptanceReport(input, runner); err == nil || len(runner.comments) != 1 || !strings.Contains(runner.comments[0], pmAcceptanceMarker) {
		t.Fatalf("partial apply did not preserve acceptance boundary: comments=%#v err=%v", runner.comments, err)
	}
	retry, err := BuildPMAcceptanceReport(input, runner)
	if err != nil || !retry.Idempotent || len(runner.comments) != 2 || !strings.Contains(runner.comments[1], pmLedgerRecordMarker) {
		t.Fatalf("retry failed to repair transition: %#v comments=%#v err=%v", retry, runner.comments, err)
	}
}

func TestPMAcceptanceRejectsDeliveryProxyOutcomeValidation(t *testing.T) {
	result := validPMAcceptanceResult()
	result.OutcomeState = "validated"
	result.Claims[0].SupportsOutcome = true
	result.Claims[0].EvidenceRefs = []string{"check:ci", "pr:OWNER/repo#99"}
	report, err := BuildPMAcceptanceReport(PMAcceptanceInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Ticket: 42, Result: result, DryRun: true}, &pmAcceptanceRunner{})
	if err == nil || !hasPMAcceptanceDiagnostic(report.Diagnostics, "PMA006_DELIVERY_PROXY") {
		t.Fatalf("delivery proxy validated outcome: %#v err=%v", report, err)
	}
}

func TestPMAcceptanceRejectsFalseDeliveryAcceptance(t *testing.T) {
	result := validPMAcceptanceResult()
	result.Criteria[0].Status = "mismatch"
	report, err := BuildPMAcceptanceReport(PMAcceptanceInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Ticket: 42, Result: result, DryRun: true}, &pmAcceptanceRunner{})
	if err == nil || !hasPMAcceptanceDiagnostic(report.Diagnostics, "PMA013_FALSE_DELIVERY_ACCEPTANCE") {
		t.Fatalf("false acceptance passed: %#v err=%v", report, err)
	}
}

func TestPMAcceptanceDryRunEmitsApprovalEvidence(t *testing.T) {
	report, err := BuildPMAcceptanceReport(PMAcceptanceInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Ticket: 42, Result: validPMAcceptanceResult(), DryRun: true}, &pmAcceptanceRunner{})
	if err != nil || report.Approval == nil || len(report.Approval.PlannedActions) != 2 || report.Approval.Capability != AdapterCapabilityApplyMutation {
		t.Fatalf("approval=%#v err=%v", report.Approval, err)
	}
}

func TestPMAcceptancePersistsIdempotentlyAndPreservesSupersession(t *testing.T) {
	runner := &pmAcceptanceRunner{}
	input := PMAcceptanceInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Ticket: 42, Result: validPMAcceptanceResult(), Apply: true}
	first, err := BuildPMAcceptanceReport(input, runner)
	if err != nil {
		t.Fatal(err)
	}
	if len(runner.comments) != 2 || first.Result.ID == "" || first.Result.Supersedes != "" {
		t.Fatalf("first persistence=%#v comments=%d", first, len(runner.comments))
	}
	retry, err := BuildPMAcceptanceReport(input, runner)
	if err != nil || !retry.Idempotent || len(runner.comments) != 2 {
		t.Fatalf("retry duplicated verdict: %#v err=%v comments=%d", retry, err, len(runner.comments))
	}
	changed := validPMAcceptanceResult()
	changed.DeliveryState = "implementation_mismatch"
	changed.Reason = "The implementation contradicts criterion one and needs a bounded code fix."
	second, err := BuildPMAcceptanceReport(PMAcceptanceInput{Repo: input.Repo, Ticket: 42, Result: changed, Apply: true}, runner)
	if err != nil {
		t.Fatal(err)
	}
	if second.Result.Supersedes != first.Result.ID || second.Result.ID == first.Result.ID || len(runner.comments) != 4 {
		t.Fatalf("changed verdict lost history: first=%#v second=%#v comments=%d", first.Result, second.Result, len(runner.comments))
	}
	history, err := LoadPMAcceptanceHistory(input.Repo, 42, runner)
	if err != nil || len(history) != 2 {
		t.Fatalf("history=%#v err=%v", history, err)
	}
	context, err := BuildPMContextReport(PMContextInput{Repo: input.Repo, Ticket: 42}, runner)
	if err != nil {
		t.Fatal(err)
	}
	currentTransitions := 0
	for _, item := range context.Records {
		if item.Current && strings.HasPrefix(item.Record.ID, "transition.acceptance.") {
			currentTransitions++
		}
	}
	if currentTransitions != 1 {
		t.Fatalf("superseded acceptance learning remained current: %#v", context.Records)
	}
}

func TestPMAcceptanceSeparatesSpecImplementationAndInconclusiveTransitions(t *testing.T) {
	value := validPMAcceptanceResult()
	value.DeliveryState = "implementation_mismatch"
	if got := pmAcceptanceTransitions(value); len(got) < 1 || got[0].Profile != "delivery" {
		t.Fatalf("implementation mismatch transition=%#v", got)
	}
	value.DeliveryState = "spec_repair"
	if got := pmAcceptanceTransitions(value); len(got) < 1 || got[0].Profile != "decision" {
		t.Fatalf("spec repair transition=%#v", got)
	}
	value.DeliveryState = "accepted"
	value.OutcomeState = "inconclusive"
	got := pmAcceptanceTransitions(value)
	if len(got) != 1 || got[0].Profile != "measurement" || got[0].Action != "create" {
		t.Fatalf("inconclusive outcome transition=%#v", got)
	}
}

func TestPMObserveConsumesLatestAcceptanceResult(t *testing.T) {
	state := pmObserveFixtureState("supported")
	value := validPMAcceptanceResult()
	value.Issue = 100
	value.DeliveryState = "spec_repair"
	value.OutcomeState = "inconclusive"
	value.ID = pmAcceptanceContentID(value)
	state.Acceptance = []PMAcceptanceResult{value}
	report := BuildPMObserveFromState(PMObserveInput{Repo: RepoRef{Owner: "OWNER", Name: "repo"}, Ticket: 100}, state)
	if report.Snapshot.AcceptanceID != value.ID || !hasPMObserveDiagnosis(report.Diagnoses, "PMO014_SPEC_DEFECT") || !hasPMObserveDiagnosis(report.Diagnoses, "PMO015_OUTCOME_INCONCLUSIVE") || !hasPMObserveAction(report.Actions, "decide") || !hasPMObserveAction(report.Actions, "validate") {
		t.Fatalf("acceptance not consumed: %#v", report)
	}
}

func validPMAcceptanceResult() PMAcceptanceResult {
	return PMAcceptanceResult{SchemaVersion: PMAcceptanceResultSchemaVersion, PullRequest: 99, DeliveryState: "accepted", OutcomeState: "not_evaluated", Reason: "All delivery criteria have inspectable evidence; outcome observation remains separate.", SourceRefs: []string{"issue:OWNER/repo#42", "pr:OWNER/repo#99"}, Criteria: []PMAcceptanceEvidenceMap{{Subject: "criterion.1", Status: "accepted", EvidenceRefs: []string{"test:go-test"}}}, Claims: []PMAcceptanceEvidenceMap{{Subject: "claim.cli", Status: "supported", EvidenceRefs: []string{"test:go-test", "diff:internal/cli"}}}}
}
func hasPMAcceptanceDiagnostic(values []PMAcceptanceDiagnostic, code string) bool {
	for _, value := range values {
		if value.Code == code {
			return true
		}
	}
	return false
}
