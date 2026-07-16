package gira

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const PMMeasurementReportSchemaVersion = "pm-measurement-report/v1"

const (
	PMMeasurementMissingPlan         = "PMM001_MISSING_PLAN"
	PMMeasurementDeliveryProxy       = "PMM002_DELIVERY_PROXY"
	PMMeasurementMissingBaseline     = "PMM003_MISSING_BASELINE"
	PMMeasurementUnboundedWindow     = "PMM004_UNBOUNDED_WINDOW"
	PMMeasurementIncomparable        = "PMM005_INCOMPARABLE_DEFINITION"
	PMMeasurementUnavailableSource   = "PMM006_UNAVAILABLE_SOURCE"
	PMMeasurementVanityMetric        = "PMM007_VANITY_METRIC"
	PMMeasurementQualitativeGap      = "PMM008_QUALITATIVE_GAP"
	PMMeasurementGuardrailRegression = "PMM009_GUARDRAIL_REGRESSION"
	PMMeasurementLimitationGap       = "PMM010_LIMITATION_FOLLOW_UP"
	PMMeasurementMissingDecision     = "PMM011_MISSING_DECISION_RULE"
)

type PMMeasurementPlan struct {
	Signal               string `json:"signal"`
	SignalKind           string `json:"signal_kind"`
	EvidenceType         string `json:"evidence_type"`
	Baseline             string `json:"baseline,omitempty"`
	BaselineDefinition   string `json:"baseline_definition,omitempty"`
	Target               string `json:"target,omitempty"`
	TargetDirection      string `json:"target_direction,omitempty"`
	ObservationWindow    string `json:"observation_window,omitempty"`
	DataSource           string `json:"data_source,omitempty"`
	SourceStatus         string `json:"source_status,omitempty"`
	Owner                string `json:"owner,omitempty"`
	DecisionRule         string `json:"decision_rule,omitempty"`
	Evaluation           string `json:"evaluation,omitempty"`
	PostChangeDefinition string `json:"post_change_definition,omitempty"`
	QualitativeMethod    string `json:"qualitative_method,omitempty"`
	QualitativeSample    string `json:"qualitative_sample,omitempty"`
	QualitativeLimits    string `json:"qualitative_limits,omitempty"`
	EvidenceLimitation   string `json:"evidence_limitation,omitempty"`
	FollowUpRef          string `json:"follow_up_ref,omitempty"`
}

type PMMeasurementInput struct {
	Repo   RepoRef
	Ticket int
}

type PMMeasurementOutcome struct {
	OutcomeID      string   `json:"outcome_id"`
	State          string   `json:"state"`
	MeasurementIDs []string `json:"measurement_ids"`
}

type PMMeasurementNode struct {
	ID           string            `json:"id"`
	Current      bool              `json:"current"`
	Status       string            `json:"status"`
	Text         string            `json:"text"`
	Links        []PMLedgerLink    `json:"links"`
	Plan         PMMeasurementPlan `json:"plan"`
	SourceRefs   []string          `json:"source_refs"`
	GoalRefs     []string          `json:"goal_refs,omitempty"`
	TaskProfiles []string          `json:"task_profiles,omitempty"`
	CommentURL   string            `json:"comment_url,omitempty"`
}

type PMMeasurementSummary struct {
	Outcomes     int `json:"outcomes"`
	Measurements int `json:"measurements"`
	Validated    int `json:"validated"`
	NotValidated int `json:"not_validated"`
	Inconclusive int `json:"inconclusive"`
	Limited      int `json:"limited"`
	Blocked      int `json:"blocked"`
}

type PMMeasurementReport struct {
	Command       string                 `json:"command"`
	SchemaVersion string                 `json:"schema_version"`
	ReadOnly      bool                   `json:"read_only"`
	Repo          string                 `json:"repo"`
	Issue         PMContextIssue         `json:"issue"`
	Outcomes      []PMMeasurementOutcome `json:"outcomes"`
	Measurements  []PMMeasurementNode    `json:"measurements"`
	Diagnostics   []PMLedgerDiagnostic   `json:"diagnostics"`
	Summary       PMMeasurementSummary   `json:"summary"`
	DetailCommand string                 `json:"detail_command"`
}

func normalizePMMeasurementPlan(plan *PMMeasurementPlan) *PMMeasurementPlan {
	if plan == nil {
		return nil
	}
	n := *plan
	n.Signal = strings.TrimSpace(n.Signal)
	n.SignalKind = normalizePMLedgerKind(n.SignalKind)
	n.EvidenceType = normalizePMLedgerKind(n.EvidenceType)
	n.Baseline = strings.TrimSpace(n.Baseline)
	n.BaselineDefinition = strings.TrimSpace(n.BaselineDefinition)
	n.Target = strings.TrimSpace(n.Target)
	n.TargetDirection = normalizePMLedgerKind(n.TargetDirection)
	n.ObservationWindow = strings.TrimSpace(n.ObservationWindow)
	n.DataSource = strings.TrimSpace(n.DataSource)
	n.SourceStatus = normalizePMLedgerKind(n.SourceStatus)
	n.Owner = strings.TrimSpace(n.Owner)
	n.DecisionRule = strings.TrimSpace(n.DecisionRule)
	n.Evaluation = normalizePMLedgerKind(n.Evaluation)
	n.PostChangeDefinition = strings.TrimSpace(n.PostChangeDefinition)
	n.QualitativeMethod = strings.TrimSpace(n.QualitativeMethod)
	n.QualitativeSample = strings.TrimSpace(n.QualitativeSample)
	n.QualitativeLimits = strings.TrimSpace(n.QualitativeLimits)
	n.EvidenceLimitation = strings.TrimSpace(n.EvidenceLimitation)
	n.FollowUpRef = strings.TrimSpace(n.FollowUpRef)
	return &n
}

func pmMeasurementSensitiveValues(plan *PMMeasurementPlan) []string {
	if plan == nil {
		return nil
	}
	return []string{plan.Signal, plan.Baseline, plan.BaselineDefinition, plan.Target, plan.ObservationWindow, plan.DataSource, plan.Owner, plan.DecisionRule, plan.PostChangeDefinition, plan.QualitativeMethod, plan.QualitativeSample, plan.QualitativeLimits, plan.EvidenceLimitation, plan.FollowUpRef}
}

func validatePMMeasurementRecord(record PMLedgerRecord) []PMLedgerDiagnostic {
	if record.Kind != "measurement" {
		return nil
	}
	out := []PMLedgerDiagnostic{}
	if !pmRecordHasLink(record, "measures") {
		out = append(out, pmLedgerDiagnostic("error", PMMeasurementMissingPlan, record.ID, "measurement lacks a measures relation", "the signal is not attached to a product outcome", "add --link measures=OUTCOME_ID"))
	}
	p := record.Measurement
	if p == nil {
		return append(out, pmLedgerDiagnostic("error", PMMeasurementMissingPlan, record.ID, "measurement plan is missing", "the outcome cannot be evaluated", "supply measurement fields"))
	}
	if p.Signal == "" || !containsPMValue([]string{"leading", "lagging", "delivery", "health", "guardrail"}, p.SignalKind) || !containsPMValue([]string{"quantitative", "qualitative", "limitation"}, p.EvidenceType) {
		out = append(out, pmLedgerDiagnostic("error", PMMeasurementMissingPlan, record.ID, "signal, signal_kind, or evidence_type is missing or invalid", "the evidence contract is ambiguous", "supply a named signal and documented kinds"))
	}
	if p.EvidenceType != "limitation" && (p.Baseline == "" || p.BaselineDefinition == "") {
		out = append(out, pmLedgerDiagnostic("error", PMMeasurementMissingBaseline, record.ID, "baseline or its definition is missing", "change cannot be compared to prior state", "add --baseline and --baseline-definition"))
	}
	if p.EvidenceType != "limitation" && (p.Target == "" || !containsPMValue([]string{"increase", "decrease", "maintain", "threshold", "qualitative"}, p.TargetDirection)) {
		out = append(out, pmLedgerDiagnostic("error", PMMeasurementMissingPlan, record.ID, "target or target_direction is missing", "success cannot be evaluated", "add --target and --target-direction"))
	}
	window := strings.ToLower(p.ObservationWindow)
	if p.EvidenceType != "limitation" && (p.ObservationWindow == "" || containsPMValue([]string{"forever", "ongoing", "unbounded", "tbd"}, window)) {
		out = append(out, pmLedgerDiagnostic("error", PMMeasurementUnboundedWindow, record.ID, "observation window is missing", "validation can be claimed at an arbitrary time", "add --observation-window"))
	}
	if p.EvidenceType != "limitation" && p.DataSource == "" {
		out = append(out, pmLedgerDiagnostic("error", PMMeasurementUnavailableSource, record.ID, "data source is missing", "evidence cannot be inspected", "add --data-source or record a limitation"))
	}
	if p.SourceStatus == "" || !containsPMValue([]string{"available", "unavailable"}, p.SourceStatus) {
		out = append(out, pmLedgerDiagnostic("error", PMMeasurementUnavailableSource, record.ID, "source_status is missing or invalid", "source availability cannot be evaluated", "use available or unavailable"))
	}
	if p.Owner == "" || p.DecisionRule == "" {
		out = append(out, pmLedgerDiagnostic("error", PMMeasurementMissingDecision, record.ID, "owner or decision rule is missing", "evidence cannot deterministically drive action", "add --owner and --decision-rule"))
	}
	if p.SourceStatus == "unavailable" {
		out = append(out, pmLedgerDiagnostic("warning", PMMeasurementUnavailableSource, record.ID, "data source is unavailable", "planned evidence cannot currently be collected", "record the limitation and a follow-up task"))
	}
	if p.BaselineDefinition != "" && p.PostChangeDefinition != "" && !strings.EqualFold(p.BaselineDefinition, p.PostChangeDefinition) {
		out = append(out, pmLedgerDiagnostic("warning", PMMeasurementIncomparable, record.ID, "baseline and post-change definitions differ", "the apparent change may be a measurement artifact", "align definitions or document a comparability analysis"))
	}
	if p.EvidenceType == "qualitative" && (p.QualitativeMethod == "" || p.QualitativeSample == "" || p.QualitativeLimits == "") {
		out = append(out, pmLedgerDiagnostic("error", PMMeasurementQualitativeGap, record.ID, "qualitative method, sample/context, or limits are missing", "qualitative evidence cannot be interpreted proportionately", "add qualitative method, sample, and limits"))
	}
	if p.EvidenceType == "limitation" && (p.EvidenceLimitation == "" || p.FollowUpRef == "") {
		out = append(out, pmLedgerDiagnostic("error", PMMeasurementLimitationGap, record.ID, "evidence limitation lacks explanation or follow-up", "the outcome would remain permanently unmeasurable", "add --evidence-limitation and --follow-up-ref"))
	}
	if containsPMVanitySignal(p.Signal) {
		out = append(out, pmLedgerDiagnostic("warning", PMMeasurementVanityMetric, record.ID, "signal resembles a vanity metric", "movement may not represent user or product value", "tie it to a decision rule and a user/product outcome signal"))
	}
	if p.SignalKind == "guardrail" && p.Evaluation == "regressed" {
		out = append(out, pmLedgerDiagnostic("error", PMMeasurementGuardrailRegression, record.ID, "guardrail regressed", "target improvement cannot be validated safely", "stop validation and follow the rollback or mitigation rule"))
	}
	if p.Evaluation != "" && !containsPMValue([]string{"met", "not_met", "inconclusive", "unavailable", "stable", "regressed"}, p.Evaluation) {
		out = append(out, pmLedgerDiagnostic("error", PMMeasurementMissingPlan, record.ID, "evaluation state is invalid", "validation state is ambiguous", "use met, not_met, inconclusive, unavailable, stable, or regressed"))
	}
	return out
}

func containsPMVanitySignal(signal string) bool {
	v := strings.ToLower(signal)
	for _, term := range []string{"page views", "impressions", "download count", "followers", "raw signups"} {
		if strings.Contains(v, term) {
			return true
		}
	}
	return false
}

func validatePMMeasurementGraph(records map[string]PMContextRecord) []PMLedgerDiagnostic {
	out := []PMLedgerDiagnostic{}
	for _, item := range records {
		if !item.Current || item.Record.Kind != "measurement" {
			continue
		}
		for _, link := range item.Record.Links {
			if link.Relation != "measures" {
				out = append(out, pmLedgerDiagnostic("error", PMDiscoveryDiagnosticInvalidRelation, item.Record.ID, "measurement uses an unsupported relation", "measurement provenance is ambiguous", "use measures=OUTCOME_ID"))
				continue
			}
			target, ok := records[link.TargetID]
			if !ok || !target.Current || target.Record.Kind != "outcome" {
				out = append(out, pmLedgerDiagnostic("error", PMDiscoveryDiagnosticInvalidRelation, item.Record.ID, "measurement target is not a current outcome", "measurement cannot validate product state", "link to a current outcome record"))
			}
		}
	}
	return out
}

func BuildPMMeasurementReport(input PMMeasurementInput, runner CommandRunner) (PMMeasurementReport, error) {
	context, err := BuildPMContextReport(PMContextInput{Repo: input.Repo, Ticket: input.Ticket}, runner)
	report := PMMeasurementReport{Command: "pm measure", SchemaVersion: PMMeasurementReportSchemaVersion, ReadOnly: true, Repo: input.Repo.FullName(), Outcomes: []PMMeasurementOutcome{}, Measurements: []PMMeasurementNode{}, Diagnostics: []PMLedgerDiagnostic{}, DetailCommand: fmt.Sprintf("gira pm measure --repo %s --ticket %d --json", input.Repo.FullName(), input.Ticket)}
	if err != nil {
		return report, err
	}
	report.Issue = context.Issue
	report.Diagnostics = append(report.Diagnostics, context.Diagnostics...)
	byID := map[string]PMContextRecord{}
	measurementsByOutcome := map[string][]PMContextRecord{}
	for _, item := range context.Records {
		byID[item.Record.ID] = item
	}
	report.Diagnostics = append(report.Diagnostics, validatePMMeasurementGraph(byID)...)
	for _, item := range context.Records {
		if !item.Current || item.Record.Kind != "measurement" {
			continue
		}
		plan := PMMeasurementPlan{}
		if item.Record.Measurement != nil {
			plan = *item.Record.Measurement
		}
		report.Measurements = append(report.Measurements, PMMeasurementNode{ID: item.Record.ID, Current: item.Current, Status: item.Record.Status, Text: item.Record.Text, Links: item.Record.Links, Plan: plan, SourceRefs: item.Record.SourceRefs, GoalRefs: item.Record.GoalRefs, TaskProfiles: item.Record.TaskProfiles, CommentURL: item.CommentURL})
		for _, link := range item.Record.Links {
			if link.Relation != "measures" {
				continue
			}
			target, ok := byID[link.TargetID]
			if !ok || !target.Current || target.Record.Kind != "outcome" {
				continue
			}
			measurementsByOutcome[link.TargetID] = append(measurementsByOutcome[link.TargetID], item)
		}
	}
	for _, item := range context.Records {
		if !item.Current || item.Record.Kind != "outcome" {
			continue
		}
		linked := measurementsByOutcome[item.Record.ID]
		outcome := PMMeasurementOutcome{OutcomeID: item.Record.ID, State: "inconclusive", MeasurementIDs: []string{}}
		if len(linked) == 0 {
			report.Diagnostics = append(report.Diagnostics, pmLedgerDiagnostic("error", PMMeasurementMissingPlan, item.Record.ID, "outcome has no measurement plan", "delivery may be mistaken for product success", "append a measurement or explicit limitation record"))
		}
		hasProduct, met, notMet, limited, guardrailRegressed := false, false, false, false, false
		linkedError := false
		for _, m := range linked {
			outcome.MeasurementIDs = append(outcome.MeasurementIDs, m.Record.ID)
			p := m.Record.Measurement
			if p == nil {
				continue
			}
			if p.SignalKind != "delivery" && p.SignalKind != "guardrail" {
				hasProduct = true
				met = met || p.Evaluation == "met"
				notMet = notMet || p.Evaluation == "not_met"
			}
			limited = limited || p.EvidenceType == "limitation" || p.Evaluation == "unavailable" || p.SourceStatus == "unavailable"
			guardrailRegressed = guardrailRegressed || (p.SignalKind == "guardrail" && p.Evaluation == "regressed")
			for _, diagnostic := range report.Diagnostics {
				if diagnostic.RecordID == m.Record.ID && diagnostic.Severity == "error" {
					linkedError = true
				}
			}
		}
		if len(linked) > 0 && !hasProduct && !limited {
			report.Diagnostics = append(report.Diagnostics, pmLedgerDiagnostic("error", PMMeasurementDeliveryProxy, item.Record.ID, "outcome is measured only by delivery or guardrail signals", "shipping could be reported as product success", "add a leading, lagging, or health outcome signal"))
			linkedError = true
		}
		if met && notMet {
			report.Diagnostics = append(report.Diagnostics, pmLedgerDiagnostic("error", PMMeasurementMissingDecision, item.Record.ID, "product signal evaluations conflict", "the outcome has no deterministic aggregate decision", "record a resolving decision rule or supersede conflicting evidence"))
			linkedError = true
		}
		switch {
		case guardrailRegressed:
			outcome.State = "blocked"
		case linkedError:
			outcome.State = "blocked"
		case limited:
			outcome.State = "limited"
		case notMet:
			outcome.State = "not_validated"
		case met:
			outcome.State = "validated"
		}
		sort.Strings(outcome.MeasurementIDs)
		report.Outcomes = append(report.Outcomes, outcome)
	}
	if len(report.Outcomes) == 0 {
		report.Diagnostics = append(report.Diagnostics, pmLedgerDiagnostic("error", PMMeasurementMissingPlan, "", "ledger has no typed outcome record", "measurability cannot be evaluated for the current phase", "record an outcome before its measurement plan"))
	}
	sort.Slice(report.Outcomes, func(i, j int) bool { return report.Outcomes[i].OutcomeID < report.Outcomes[j].OutcomeID })
	report.Summary.Outcomes = len(report.Outcomes)
	report.Summary.Measurements = len(report.Measurements)
	for _, o := range report.Outcomes {
		switch o.State {
		case "validated":
			report.Summary.Validated++
		case "not_validated":
			report.Summary.NotValidated++
		case "limited":
			report.Summary.Limited++
		case "blocked":
			report.Summary.Blocked++
		default:
			report.Summary.Inconclusive++
		}
	}
	sortPMLedgerDiagnostics(report.Diagnostics)
	return report, nil
}

func FormatPMMeasurement(report PMMeasurementReport, budget int) string {
	if budget <= 0 {
		budget = 6000
	}
	var b strings.Builder
	fmt.Fprintf(&b, "pm measure: %s#%d outcomes=%d measurements=%d validated=%d blocked=%d diagnostics=%d\n", report.Repo, report.Issue.Number, report.Summary.Outcomes, report.Summary.Measurements, report.Summary.Validated, report.Summary.Blocked, len(report.Diagnostics))
	for _, d := range report.Diagnostics {
		line := fmt.Sprintf("- %s %s %s: %s\n", d.Severity, d.Code, d.RecordID, d.Reason)
		if b.Len()+len(line)+len(report.DetailCommand)+20 > budget {
			b.WriteString("- entries omitted by context budget\n")
			break
		}
		b.WriteString(line)
	}
	for _, o := range report.Outcomes {
		line := fmt.Sprintf("- outcome %s state=%s measurements=%d\n", o.OutcomeID, o.State, len(o.MeasurementIDs))
		if b.Len()+len(line)+len(report.DetailCommand)+20 > budget {
			break
		}
		b.WriteString(line)
	}
	b.WriteString("detail: " + report.DetailCommand + "\n")
	if b.Len() <= budget {
		return b.String()
	}
	fallback := fmt.Sprintf("pm measure: #%d outcomes=%d\ndetail: gira pm measure --json\n", report.Issue.Number, report.Summary.Outcomes)
	if len(fallback) > budget {
		return fallback[:budget]
	}
	return fallback
}

func FormatPMMeasurementJSON(report PMMeasurementReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}
