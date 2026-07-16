package gira

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const PMDiscoveryReportSchemaVersion = "pm-discovery-graph/v1"

const (
	PMDiscoveryDiagnosticMissingLink       = "PMD001_MISSING_LINK"
	PMDiscoveryDiagnosticMissingTest       = "PMD002_MISSING_TEST"
	PMDiscoveryDiagnosticInvalidRisk       = "PMD003_INVALID_RISK"
	PMDiscoveryDiagnosticInvalidExperiment = "PMD004_INVALID_EXPERIMENT"
	PMDiscoveryDiagnosticFalseValidation   = "PMD005_FALSE_VALIDATION"
	PMDiscoveryDiagnosticMissingEvidence   = "PMD006_MISSING_EVIDENCE"
	PMDiscoveryDiagnosticUnknownTarget     = "PMD007_UNKNOWN_TARGET"
	PMDiscoveryDiagnosticInvalidRelation   = "PMD008_INVALID_RELATION"
)

type PMDiscoveryInput struct {
	Repo   RepoRef
	Ticket int
}

type PMDiscoveryNode struct {
	ID               string   `json:"id"`
	Kind             string   `json:"kind"`
	Text             string   `json:"text"`
	Current          bool     `json:"current"`
	Status           string   `json:"status"`
	OutcomeState     string   `json:"outcome_state,omitempty"`
	ExperimentState  string   `json:"experiment_state,omitempty"`
	Conclusion       string   `json:"conclusion,omitempty"`
	RiskType         string   `json:"risk_type,omitempty"`
	EvidenceStrength string   `json:"evidence_strength,omitempty"`
	Confidence       string   `json:"confidence,omitempty"`
	SourceRefs       []string `json:"source_refs"`
	GoalRefs         []string `json:"goal_refs,omitempty"`
	TaskProfiles     []string `json:"task_profiles,omitempty"`
	CommentURL       string   `json:"comment_url,omitempty"`
}

type PMDiscoveryEdge struct {
	From     string `json:"from"`
	Relation string `json:"relation"`
	To       string `json:"to"`
	Current  bool   `json:"current"`
}

type PMDiscoveryTrace struct {
	OutcomeID string   `json:"outcome_id"`
	Path      []string `json:"path"`
}

type PMDiscoverySummary struct {
	Nodes               int            `json:"nodes"`
	CurrentNodes        int            `json:"current_nodes"`
	Edges               int            `json:"edges"`
	Traces              int            `json:"traces"`
	ByKind              map[string]int `json:"by_kind"`
	OutcomeStates       map[string]int `json:"outcome_states"`
	ExperimentStates    map[string]int `json:"experiment_states"`
	LearningConclusions map[string]int `json:"learning_conclusions"`
}

type PMDiscoveryReport struct {
	Command       string               `json:"command"`
	SchemaVersion string               `json:"schema_version"`
	ReadOnly      bool                 `json:"read_only"`
	Repo          string               `json:"repo"`
	Issue         PMContextIssue       `json:"issue"`
	Nodes         []PMDiscoveryNode    `json:"nodes"`
	Edges         []PMDiscoveryEdge    `json:"edges"`
	Traces        []PMDiscoveryTrace   `json:"traces"`
	Diagnostics   []PMLedgerDiagnostic `json:"diagnostics"`
	Summary       PMDiscoverySummary   `json:"summary"`
	DetailCommand string               `json:"detail_command"`
}

func BuildPMDiscoveryReport(input PMDiscoveryInput, runner CommandRunner) (PMDiscoveryReport, error) {
	context, err := BuildPMContextReport(PMContextInput{Repo: input.Repo, Ticket: input.Ticket}, runner)
	report := PMDiscoveryReport{
		Command: "pm discovery", SchemaVersion: PMDiscoveryReportSchemaVersion, ReadOnly: true,
		Repo: input.Repo.FullName(), Nodes: []PMDiscoveryNode{}, Edges: []PMDiscoveryEdge{}, Traces: []PMDiscoveryTrace{}, Diagnostics: []PMLedgerDiagnostic{},
		Summary:       PMDiscoverySummary{ByKind: map[string]int{}, OutcomeStates: map[string]int{}, ExperimentStates: map[string]int{}, LearningConclusions: map[string]int{}},
		DetailCommand: fmt.Sprintf("gira pm discovery --repo %s --ticket %d --json", input.Repo.FullName(), input.Ticket),
	}
	if err != nil {
		return report, err
	}
	report.Issue = context.Issue
	report.Diagnostics = append(report.Diagnostics, context.Diagnostics...)
	recordByID := map[string]PMContextRecord{}
	for _, item := range context.Records {
		recordByID[item.Record.ID] = item
		if !isPMDiscoveryGraphKind(item.Record.Kind) {
			continue
		}
		node := pmDiscoveryNodeFromContext(item)
		report.Nodes = append(report.Nodes, node)
		for _, link := range item.Record.Links {
			report.Edges = append(report.Edges, PMDiscoveryEdge{From: item.Record.ID, Relation: link.Relation, To: link.TargetID, Current: item.Current})
		}
	}
	report.Diagnostics = append(report.Diagnostics, validatePMDiscoveryGraph(recordByID)...)
	report.Traces = buildPMDiscoveryTraces(recordByID)
	sort.Slice(report.Nodes, func(i, j int) bool {
		if report.Nodes[i].Kind != report.Nodes[j].Kind {
			return report.Nodes[i].Kind < report.Nodes[j].Kind
		}
		return report.Nodes[i].ID < report.Nodes[j].ID
	})
	sort.Slice(report.Edges, func(i, j int) bool {
		if report.Edges[i].From != report.Edges[j].From {
			return report.Edges[i].From < report.Edges[j].From
		}
		if report.Edges[i].Relation != report.Edges[j].Relation {
			return report.Edges[i].Relation < report.Edges[j].Relation
		}
		return report.Edges[i].To < report.Edges[j].To
	})
	sort.Slice(report.Traces, func(i, j int) bool {
		return strings.Join(report.Traces[i].Path, "\x00") < strings.Join(report.Traces[j].Path, "\x00")
	})
	for _, node := range report.Nodes {
		report.Summary.Nodes++
		if node.Current {
			report.Summary.CurrentNodes++
			report.Summary.ByKind[node.Kind]++
			if node.OutcomeState != "" {
				report.Summary.OutcomeStates[node.OutcomeState]++
			}
			if node.ExperimentState != "" {
				report.Summary.ExperimentStates[node.ExperimentState]++
			}
			if node.Conclusion != "" {
				report.Summary.LearningConclusions[node.Conclusion]++
			}
		}
	}
	report.Summary.Edges = len(report.Edges)
	report.Summary.Traces = len(report.Traces)
	sortPMLedgerDiagnostics(report.Diagnostics)
	return report, nil
}

func validatePMDiscoveryRecord(record PMLedgerRecord) []PMLedgerDiagnostic {
	if !isPMDiscoveryGraphKind(record.Kind) {
		return nil
	}
	out := []PMLedgerDiagnostic{}
	requireLink := func(relation string) {
		if !pmRecordHasLink(record, relation) {
			out = append(out, pmLedgerDiagnostic("error", PMDiscoveryDiagnosticMissingLink, record.ID, fmt.Sprintf("%s record lacks %s relation", record.Kind, relation), "the opportunity-to-outcome trace is broken", fmt.Sprintf("add --link %s=TARGET_ID", relation)))
		}
	}
	switch record.Kind {
	case "outcome":
		if record.OutcomeState == "" {
			record.OutcomeState = "proposed"
		}
		if !containsPMValue([]string{"proposed", "observing", "achieved", "not_achieved", "unknown"}, record.OutcomeState) {
			out = append(out, pmLedgerDiagnostic("error", PMDiscoveryDiagnosticMissingEvidence, record.ID, "outcome_state is invalid", "product outcome cannot be reported separately from delivery", "use proposed, observing, achieved, not_achieved, or unknown"))
		}
	case "opportunity":
		requireLink("supports")
	case "hypothesis":
		requireLink("addresses")
		if record.FalsificationTest == "" && record.TestWaiver == "" {
			out = append(out, pmLedgerDiagnostic("error", PMDiscoveryDiagnosticMissingTest, record.ID, "hypothesis has neither a falsification test nor a proportionality waiver", "the solution claim cannot produce inspectable learning", "add --falsification-test or --test-waiver"))
		}
	case "risk":
		requireLink("risks")
		if !containsPMValue([]string{"value", "usability", "feasibility", "viability"}, record.RiskType) {
			out = append(out, pmLedgerDiagnostic("error", PMDiscoveryDiagnosticInvalidRisk, record.ID, "risk_type is missing or invalid", "the PM cannot route the uncertainty proportionately", "use value, usability, feasibility, or viability"))
		}
	case "experiment":
		requireLink("tests")
		if !containsPMValue([]string{"planned", "running", "success", "failure", "inconclusive", "invalid"}, record.ExperimentState) {
			out = append(out, pmLedgerDiagnostic("error", PMDiscoveryDiagnosticInvalidExperiment, record.ID, "experiment_state is missing or invalid", "experiment evidence has no deterministic lifecycle", "use planned, running, success, failure, inconclusive, or invalid"))
		}
		if containsPMValue([]string{"success", "failure", "inconclusive", "invalid"}, record.ExperimentState) {
			out = append(out, validatePMDiscoveryEvidence(record)...)
		}
	case "learning":
		if len(record.Links) > 0 {
			requireLink("learned_from")
			if !containsPMValue([]string{"validated", "invalidated", "inconclusive", "no_build"}, record.Conclusion) {
				out = append(out, pmLedgerDiagnostic("error", PMDiscoveryDiagnosticMissingEvidence, record.ID, "learning conclusion is missing or invalid", "learning cannot safely drive a decision", "use validated, invalidated, inconclusive, or no_build"))
			}
			out = append(out, validatePMDiscoveryEvidence(record)...)
		}
	case "decision":
		if len(record.Links) > 0 {
			requireLink("based_on")
		}
	}
	return out
}

func validatePMDiscoveryEvidence(record PMLedgerRecord) []PMLedgerDiagnostic {
	out := []PMLedgerDiagnostic{}
	if !containsPMValue([]string{"anecdotal", "qualitative", "quantitative", "replicated"}, record.EvidenceStrength) {
		out = append(out, pmLedgerDiagnostic("error", PMDiscoveryDiagnosticMissingEvidence, record.ID, "evidence_strength is missing or invalid", "evidence cannot be compared without exposing its basis", "use anecdotal, qualitative, quantitative, or replicated"))
	}
	if !containsPMValue([]string{"low", "medium", "high"}, record.Confidence) {
		out = append(out, pmLedgerDiagnostic("error", PMDiscoveryDiagnosticMissingEvidence, record.ID, "confidence is missing or invalid", "uncertainty remains opaque", "use low, medium, or high without aggregating it into a score"))
	}
	return out
}

func validatePMDiscoveryGraph(records map[string]PMContextRecord) []PMLedgerDiagnostic {
	out := []PMLedgerDiagnostic{}
	expected := map[string]map[string]string{
		"opportunity": {"supports": "outcome"},
		"hypothesis":  {"addresses": "opportunity"},
		"risk":        {"risks": "hypothesis"},
		"experiment":  {"tests": "hypothesis"},
		"learning":    {"learned_from": "experiment"},
		"decision":    {"based_on": "learning"},
	}
	for _, item := range records {
		if !item.Current {
			continue
		}
		record := item.Record
		if !isPMDiscoveryGraphKind(record.Kind) {
			continue
		}
		for _, link := range record.Links {
			target, ok := records[link.TargetID]
			if !ok || !target.Current {
				out = append(out, pmLedgerDiagnostic("error", PMDiscoveryDiagnosticUnknownTarget, record.ID, fmt.Sprintf("current link target %s does not exist", link.TargetID), "the current discovery graph cannot be traced", "record a current target or supersede this link"))
				continue
			}
			wantKind, allowed := expected[record.Kind][link.Relation]
			if !allowed || target.Record.Kind != wantKind {
				out = append(out, pmLedgerDiagnostic("error", PMDiscoveryDiagnosticInvalidRelation, record.ID, fmt.Sprintf("%s %s cannot target %s", record.Kind, link.Relation, target.Record.Kind), "the graph does not preserve outcome-to-learning semantics", "use the documented relation and target kind"))
			}
			if record.Kind == "learning" && record.Conclusion == "validated" && target.Record.ExperimentState == "inconclusive" {
				out = append(out, pmLedgerDiagnostic("error", PMDiscoveryDiagnosticFalseValidation, record.ID, "learning reports validation from an inconclusive experiment", "inconclusive evidence would be presented as product validation", "set conclusion to inconclusive or append stronger experiment evidence"))
			}
		}
	}
	return out
}

func isPMDiscoveryGraphKind(kind string) bool {
	return containsPMValue([]string{"outcome", "opportunity", "hypothesis", "risk", "experiment", "learning", "decision"}, kind)
}

func pmRecordHasLink(record PMLedgerRecord, relation string) bool {
	for _, link := range record.Links {
		if link.Relation == relation && link.TargetID != "" {
			return true
		}
	}
	return false
}

func pmDiscoveryNodeFromContext(item PMContextRecord) PMDiscoveryNode {
	record := item.Record
	outcomeState := record.OutcomeState
	if record.Kind == "outcome" && outcomeState == "" {
		outcomeState = "proposed"
	}
	return PMDiscoveryNode{
		ID: record.ID, Kind: record.Kind, Text: record.Text, Current: item.Current, Status: record.Status,
		OutcomeState: outcomeState, ExperimentState: record.ExperimentState, Conclusion: record.Conclusion,
		RiskType: record.RiskType, EvidenceStrength: record.EvidenceStrength, Confidence: record.Confidence,
		SourceRefs: record.SourceRefs, GoalRefs: record.GoalRefs, TaskProfiles: record.TaskProfiles, CommentURL: item.CommentURL,
	}
}

func buildPMDiscoveryTraces(records map[string]PMContextRecord) []PMDiscoveryTrace {
	reverse := map[string][]string{}
	for _, item := range records {
		if !item.Current {
			continue
		}
		for _, link := range item.Record.Links {
			if target, ok := records[link.TargetID]; ok && target.Current {
				reverse[link.TargetID] = append(reverse[link.TargetID], item.Record.ID)
			}
		}
	}
	for id := range reverse {
		sort.Strings(reverse[id])
	}
	traces := []PMDiscoveryTrace{}
	var walk func(outcome string, current string, path []string, seen map[string]bool)
	walk = func(outcome string, current string, path []string, seen map[string]bool) {
		if seen[current] {
			return
		}
		seen[current] = true
		path = append(path, current)
		next := reverse[current]
		if len(next) == 0 {
			traces = append(traces, PMDiscoveryTrace{OutcomeID: outcome, Path: append([]string{}, path...)})
			return
		}
		for _, child := range next {
			copySeen := map[string]bool{}
			for key, value := range seen {
				copySeen[key] = value
			}
			walk(outcome, child, path, copySeen)
		}
	}
	for id, item := range records {
		if item.Current && item.Record.Kind == "outcome" {
			walk(id, id, nil, map[string]bool{})
		}
	}
	return traces
}

func FormatPMDiscovery(report PMDiscoveryReport, budget int) string {
	if budget <= 0 {
		budget = 6000
	}
	var b strings.Builder
	fmt.Fprintf(&b, "pm discovery: %s#%d nodes=%d current=%d edges=%d traces=%d diagnostics=%d\n", report.Repo, report.Issue.Number, report.Summary.Nodes, report.Summary.CurrentNodes, report.Summary.Edges, report.Summary.Traces, len(report.Diagnostics))
	lines := []string{}
	for _, diagnostic := range report.Diagnostics {
		lines = append(lines, fmt.Sprintf("- %s %s %s: %s", diagnostic.Severity, diagnostic.Code, diagnostic.RecordID, diagnostic.Reason))
	}
	for _, trace := range report.Traces {
		lines = append(lines, "- trace "+strings.Join(trace.Path, " -> "))
	}
	for _, node := range report.Nodes {
		if !node.Current || node.Kind == "outcome" {
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s %s evidence=%s confidence=%s", node.Kind, node.ID, emptyPMDiscoveryValue(node.EvidenceStrength), emptyPMDiscoveryValue(node.Confidence)))
	}
	detail := "detail: " + report.DetailCommand + "\n"
	omitted := 0
	for index, line := range lines {
		if b.Len()+len(line)+len(detail)+65 > budget {
			omitted = len(lines) - index
			break
		}
		b.WriteString(line + "\n")
	}
	if omitted > 0 {
		fmt.Fprintf(&b, "- %d entries omitted by context budget\n", omitted)
	}
	b.WriteString(detail)
	if b.Len() <= budget {
		return b.String()
	}
	fallback := fmt.Sprintf("pm discovery: #%d current=%d\n- output reduced by context budget\ndetail: gira pm discovery --json\n", report.Issue.Number, report.Summary.CurrentNodes)
	if len(fallback) <= budget {
		return fallback
	}
	return fallback[:budget]
}

func emptyPMDiscoveryValue(value string) string {
	if value == "" {
		return "n/a"
	}
	return value
}

func FormatPMDiscoveryJSON(report PMDiscoveryReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}
