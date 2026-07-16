package gira

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const PMIRSchemaVersion = "pm-ir/v1"
const PMCompileReportSchemaVersion = "pm-compile-report/v1"

const (
	PMProvenanceSupplied   = "supplied"
	PMProvenanceInferred   = "inferred"
	PMProvenanceAssumed    = "assumed"
	PMProvenanceUnresolved = "unresolved"
)

const (
	PMDiagnosticMissingActor        = "PM001_MISSING_ACTOR"
	PMDiagnosticMissingProblem      = "PM002_MISSING_PROBLEM"
	PMDiagnosticMissingOutcome      = "PM003_MISSING_OUTCOME"
	PMDiagnosticAmbiguousField      = "PM004_AMBIGUOUS_FIELD"
	PMDiagnosticLowEvidence         = "PM005_LOW_EVIDENCE"
	PMDiagnosticConflictingState    = "PM006_CONFLICTING_STATE"
	PMDiagnosticAuthorityBound      = "PM007_AUTHORITY_BOUND"
	PMDiagnosticUnstructuredIntent  = "PM008_UNSTRUCTURED_INTENT"
	PMDiagnosticMissingSuccess      = "PM009_MISSING_SUCCESS_CONDITION"
	PMDiagnosticUnrecognizedSection = "PM010_UNRECOGNIZED_SECTION"
)

type PMCompileInput struct {
	RawIntent string         `json:"raw_intent"`
	Repo      string         `json:"repo,omitempty"`
	Goal      *PMCompileGoal `json:"goal,omitempty"`
}

type PMCompileRequest struct {
	RawIntent string
	Repo      string
	Goal      int
}

type PMCompileGoal struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	URL    string `json:"url"`
}

type PMCompileReport struct {
	Command       string                `json:"command"`
	SchemaVersion string                `json:"schema_version"`
	ReadOnly      bool                  `json:"read_only"`
	IR            PMIR                  `json:"ir"`
	Diagnostics   []PMCompileDiagnostic `json:"diagnostics"`
	Summary       PMCompileSummary      `json:"summary"`
	DetailCommand string                `json:"detail_command"`
}

type PMCompileSummary struct {
	Errors      int `json:"errors"`
	Warnings    int `json:"warnings"`
	Info        int `json:"info"`
	Resolved    int `json:"resolved_fields"`
	Unresolved  int `json:"unresolved_fields"`
	SourceCount int `json:"source_count"`
}

type PMIR struct {
	SchemaVersion     string                 `json:"schema_version"`
	SourceDigest      string                 `json:"source_digest"`
	Sources           []PMIRSource           `json:"sources"`
	Premise           PMIRField              `json:"premise"`
	Actor             PMIRField              `json:"actor"`
	Problem           PMIRField              `json:"problem"`
	DesiredOutcome    PMIRField              `json:"desired_outcome"`
	Constraints       PMIRCollection         `json:"constraints"`
	NonGoals          PMIRCollection         `json:"non_goals"`
	Authority         PMIRCollection         `json:"authority"`
	Evidence          PMIRCollection         `json:"evidence"`
	Assumptions       PMIRCollection         `json:"assumptions"`
	DecisionDebt      PMIRCollection         `json:"decision_debt"`
	SuccessConditions PMIRCollection         `json:"success_conditions"`
	CandidateWork     PMIRCollection         `json:"candidate_work"`
	Repository        *PMIRRepositoryContext `json:"repository,omitempty"`
	GoalContext       *PMIRGoalContext       `json:"goal_context,omitempty"`
}

type PMIRRepositoryContext struct {
	FullName PMIRField `json:"full_name"`
}

type PMIRGoalContext struct {
	Number    int       `json:"number"`
	Title     PMIRField `json:"title"`
	Objective PMIRField `json:"objective"`
	Direction PMIRField `json:"direction"`
	Scope     PMIRField `json:"scope"`
	URL       string    `json:"url"`
}

type PMIRField struct {
	Value      string        `json:"value,omitempty"`
	Provenance string        `json:"provenance"`
	Sources    []PMSourceRef `json:"sources"`
}

type PMIRCollection struct {
	Items      []PMIRItem    `json:"items"`
	Provenance string        `json:"provenance"`
	Sources    []PMSourceRef `json:"sources"`
}

type PMIRItem struct {
	Value      string        `json:"value"`
	Provenance string        `json:"provenance"`
	Sources    []PMSourceRef `json:"sources"`
}

type PMIRSource struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Ref     string `json:"ref"`
	Digest  string `json:"digest"`
	Lines   int    `json:"lines"`
	Content string `json:"content"`
}

type PMSourceRef struct {
	SourceID  string `json:"source_id"`
	Section   string `json:"section,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}

type PMCompileDiagnostic struct {
	Severity string               `json:"severity"`
	Code     string               `json:"code"`
	Field    string               `json:"field"`
	Location PMDiagnosticLocation `json:"location"`
	Reason   string               `json:"reason"`
	Impact   string               `json:"impact"`
	Repair   string               `json:"repair"`
}

type PMDiagnosticLocation struct {
	SourceID  string `json:"source_id"`
	Section   string `json:"section"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}

type pmParsedDocument struct {
	sourceID string
	lines    []string
	preamble []pmDocumentLine
	sections map[string][]pmSectionOccurrence
}

type pmDocumentLine struct {
	number int
	text   string
}

type pmSectionOccurrence struct {
	heading   string
	headingAt int
	lines     []pmDocumentLine
}

func BuildPMCompileReportFromRequest(request PMCompileRequest, runner CommandRunner) (PMCompileReport, error) {
	input := PMCompileInput{RawIntent: request.RawIntent, Repo: strings.TrimSpace(request.Repo)}
	if input.Repo != "" {
		if _, err := ParseRepoRef(input.Repo); err != nil {
			return PMCompileReport{}, err
		}
	}
	if request.Goal > 0 {
		if input.Repo == "" {
			return PMCompileReport{}, fmt.Errorf("--goal requires --repo OWNER/REPO")
		}
		if runner == nil {
			runner = ExecCommandRunner{}
		}
		repo, _ := ParseRepoRef(input.Repo)
		issue, err := fetchDevIssue(repo, request.Goal, runner)
		if err != nil {
			return PMCompileReport{}, fmt.Errorf("hydrate PM goal context: %w", err)
		}
		if issue.IsPR {
			return PMCompileReport{}, fmt.Errorf("goal context #%d is a pull request", request.Goal)
		}
		if !isGoalIssueLabels(issue.Labels) {
			return PMCompileReport{}, fmt.Errorf("goal context #%d is not a Goal issue; expected type:goal or type:epic", request.Goal)
		}
		input.Goal = &PMCompileGoal{
			Number: issue.Number,
			Title:  issue.Title,
			Body:   issue.Body,
			URL:    fmt.Sprintf("https://github.com/%s/issues/%d", input.Repo, issue.Number),
		}
	}
	return BuildPMCompileReport(input)
}

func BuildPMCompileReport(input PMCompileInput) (PMCompileReport, error) {
	rawIntent := strings.TrimSpace(input.RawIntent)
	if rawIntent == "" {
		return PMCompileReport{}, fmt.Errorf("raw intent is required")
	}
	input.RawIntent = rawIntent
	input.Repo = strings.TrimSpace(input.Repo)
	if input.Repo != "" {
		if _, err := ParseRepoRef(input.Repo); err != nil {
			return PMCompileReport{}, err
		}
	}
	intent := parsePMDocument("intent", rawIntent)
	sources := []PMIRSource{newPMIRSource("intent", "intent", "inline-or-file", rawIntent)}
	diagnostics := []PMCompileDiagnostic{}

	premise, premiseDiagnostics := pmScalarField(intent, "premise", []string{"premise", "product premise", "product context"})
	diagnostics = append(diagnostics, premiseDiagnostics...)
	if premise.Provenance == PMProvenanceUnresolved {
		if preamble := pmLinesText(intent.preamble); preamble != "" {
			premise = pmFieldFromLines(preamble, "supplied", "intent", "preamble", intent.preamble)
		} else if len(intent.sections) == 0 {
			premise = PMIRField{Value: rawIntent, Provenance: PMProvenanceSupplied, Sources: []PMSourceRef{{SourceID: "intent", Section: "unstructured", StartLine: 1, EndLine: len(intent.lines)}}}
		}
	}
	actor, actorDiagnostics := pmScalarField(intent, "actor", []string{"actor", "user", "customer", "affected user"})
	problem, problemDiagnostics := pmScalarField(intent, "problem", []string{"problem"})
	outcome, outcomeDiagnostics := pmScalarField(intent, "desired_outcome", []string{"desired outcome", "outcome", "customer outcome", "user outcome"})
	diagnostics = append(diagnostics, actorDiagnostics...)
	diagnostics = append(diagnostics, problemDiagnostics...)
	diagnostics = append(diagnostics, outcomeDiagnostics...)

	ir := PMIR{
		SchemaVersion:     PMIRSchemaVersion,
		Sources:           sources,
		Premise:           premise,
		Actor:             actor,
		Problem:           problem,
		DesiredOutcome:    outcome,
		Constraints:       pmCollectionField(intent, []string{"constraints", "constraint"}),
		NonGoals:          pmCollectionField(intent, []string{"non-goals", "non goals", "no-gos", "no gos"}),
		Authority:         pmCollectionField(intent, []string{"authority", "authority boundaries", "approval boundaries"}),
		Evidence:          pmCollectionField(intent, []string{"evidence", "evidence refs", "sources"}),
		Assumptions:       pmCollectionField(intent, []string{"assumptions", "assumption"}),
		DecisionDebt:      pmCollectionField(intent, []string{"decision debt", "open decisions", "decision required"}),
		SuccessConditions: pmCollectionField(intent, []string{"success conditions", "success criteria", "acceptance criteria", "signals", "metrics"}),
		CandidateWork:     pmCollectionField(intent, []string{"candidate work", "work", "decomposition", "child work"}),
	}
	if input.Repo != "" {
		repoRef := PMSourceRef{SourceID: "repository", Section: "full_name", StartLine: 1, EndLine: 1}
		ir.Repository = &PMIRRepositoryContext{FullName: PMIRField{Value: input.Repo, Provenance: PMProvenanceSupplied, Sources: []PMSourceRef{repoRef}}}
		ir.Sources = append(ir.Sources, newPMIRSource("repository", "repository", input.Repo, input.Repo))
	}

	if input.Goal != nil {
		goalContext, goalSource, goalDiagnostics := compilePMGoalContext(*input.Goal)
		ir.GoalContext = &goalContext
		ir.Sources = append(ir.Sources, goalSource)
		diagnostics = append(diagnostics, goalDiagnostics...)
		if ir.DesiredOutcome.Provenance == PMProvenanceUnresolved && strings.TrimSpace(goalContext.Objective.Value) != "" {
			ir.DesiredOutcome = goalContext.Objective
			ir.DesiredOutcome.Provenance = PMProvenanceInferred
		}
	}

	if !hasRecognizedPMSections(intent) {
		diagnostics = append(diagnostics, pmDiagnostic("info", PMDiagnosticUnstructuredIntent, "intent", "intent", "unstructured", 1, len(intent.lines),
			"intent has no recognized PM headings", "semantic fields remain unresolved instead of being guessed", "add Markdown sections such as Actor, Problem, Desired Outcome, Evidence, and Success Conditions"))
	}
	diagnostics = append(diagnostics, unrecognizedPMSectionDiagnostics(intent)...)
	diagnostics = append(diagnostics, requiredPMFieldDiagnostics(ir)...)
	diagnostics = append(diagnostics, conflictingPMCollectionDiagnostics("constraints", ir.Constraints)...)
	diagnostics = append(diagnostics, conflictingPMCollectionDiagnostics("non_goals", ir.NonGoals)...)
	if len(ir.Authority.Items) > 0 {
		ref := firstPMCollectionRef(ir.Authority)
		diagnostics = append(diagnostics, pmDiagnostic("warning", PMDiagnosticAuthorityBound, "authority", ref.SourceID, ref.Section, ref.StartLine, ref.EndLine,
			"the intent declares an authority boundary", "candidate work crossing this boundary cannot be authorized by compilation", "keep authority-bound work isolated and require an explicit grant before apply"))
	}

	sortPMDiagnostics(diagnostics)
	ir.SourceDigest = digestPMIRSources(ir.Sources)
	report := PMCompileReport{
		Command:       "pm compile",
		SchemaVersion: PMCompileReportSchemaVersion,
		ReadOnly:      true,
		IR:            ir,
		Diagnostics:   diagnostics,
		DetailCommand: pmCompileDetailCommand(input),
	}
	report.Summary = summarizePMCompile(report)
	return report, nil
}

func compilePMGoalContext(goal PMCompileGoal) (PMIRGoalContext, PMIRSource, []PMCompileDiagnostic) {
	sourceID := fmt.Sprintf("goal:%d", goal.Number)
	parsed := parsePMDocument(sourceID, goal.Body)
	objective, objectiveDiagnostics := pmScalarField(parsed, "goal.objective", []string{"goal", "objective"})
	direction, directionDiagnostics := pmScalarField(parsed, "goal.direction", []string{"direction", "strategy"})
	scope, scopeDiagnostics := pmScalarField(parsed, "goal.scope", []string{"scope"})
	return PMIRGoalContext{
		Number:    goal.Number,
		Title:     PMIRField{Value: strings.TrimSpace(goal.Title), Provenance: PMProvenanceSupplied, Sources: []PMSourceRef{{SourceID: sourceID, Section: "title"}}},
		Objective: objective,
		Direction: direction,
		Scope:     scope,
		URL:       strings.TrimSpace(goal.URL),
	}, newPMIRSource(sourceID, "github_goal", goal.URL, goal.Body), append(append(objectiveDiagnostics, directionDiagnostics...), scopeDiagnostics...)
}

func requiredPMFieldDiagnostics(ir PMIR) []PMCompileDiagnostic {
	out := []PMCompileDiagnostic{}
	if ir.Actor.Provenance == PMProvenanceUnresolved {
		out = append(out, pmMissingDiagnostic("warning", PMDiagnosticMissingActor, "actor", "the affected user or actor is not supplied", "the compiler cannot verify whose outcome matters", "add an Actor or User section"))
	}
	if ir.Problem.Provenance == PMProvenanceUnresolved {
		out = append(out, pmMissingDiagnostic("error", PMDiagnosticMissingProblem, "problem", "the product or operational problem is not supplied", "candidate work may optimize for a requested solution instead of the problem", "add a Problem section grounded in available evidence"))
	}
	if ir.DesiredOutcome.Provenance == PMProvenanceUnresolved {
		out = append(out, pmMissingDiagnostic("error", PMDiagnosticMissingOutcome, "desired_outcome", "the desired outcome is not supplied or inferable from explicit Goal context", "completion cannot be evaluated as a product result", "add a Desired Outcome section or provide --repo and --goal with an explicit Goal objective"))
	}
	if len(ir.Evidence.Items) == 0 {
		out = append(out, pmMissingDiagnostic("warning", PMDiagnosticLowEvidence, "evidence", "no evidence references are supplied", "assumptions and problem claims cannot be independently inspected", "add repository, issue, research, metric, log, or customer evidence references"))
	}
	if len(ir.SuccessConditions.Items) == 0 {
		out = append(out, pmMissingDiagnostic("error", PMDiagnosticMissingSuccess, "success_conditions", "no success condition is supplied", "the PM cannot distinguish delivered work from a successful outcome", "add pass/fail success conditions, signals, metrics, or proportionate qualitative evidence"))
	}
	return out
}

func unrecognizedPMSectionDiagnostics(document pmParsedDocument) []PMCompileDiagnostic {
	recognized := recognizedPMHeadings()
	out := []PMCompileDiagnostic{}
	for heading, occurrences := range document.sections {
		if recognized[heading] {
			continue
		}
		for _, occurrence := range occurrences {
			out = append(out, pmDiagnostic("info", PMDiagnosticUnrecognizedSection, "intent", document.sourceID, occurrence.heading, occurrence.headingAt, occurrence.headingAt,
				"the heading is not mapped by pm-ir/v1", "its prose remains in the source but is not assigned semantic meaning", "rename the heading to a recognized PM field or retain it for a later compiler version"))
		}
	}
	return out
}

func hasRecognizedPMSections(document pmParsedDocument) bool {
	recognized := recognizedPMHeadings()
	for heading := range document.sections {
		if recognized[heading] {
			return true
		}
	}
	return false
}

func recognizedPMHeadings() map[string]bool {
	recognized := map[string]bool{}
	for _, aliases := range [][]string{
		{"premise", "product premise", "product context"},
		{"actor", "user", "customer", "affected user"},
		{"problem"},
		{"desired outcome", "outcome", "customer outcome", "user outcome"},
		{"constraints", "constraint"},
		{"non-goals", "non goals", "no-gos", "no gos"},
		{"authority", "authority boundaries", "approval boundaries"},
		{"evidence", "evidence refs", "sources"},
		{"assumptions", "assumption"},
		{"decision debt", "open decisions", "decision required"},
		{"success conditions", "success criteria", "acceptance criteria", "signals", "metrics"},
		{"candidate work", "work", "decomposition", "child work"},
	} {
		for _, alias := range aliases {
			recognized[normalizePMHeading(alias)] = true
		}
	}
	return recognized
}

func pmMissingDiagnostic(severity, code, field, reason, impact, repair string) PMCompileDiagnostic {
	return pmDiagnostic(severity, code, field, "intent", field, 0, 0, reason, impact, repair)
}

func pmDiagnostic(severity, code, field, source, section string, start, end int, reason, impact, repair string) PMCompileDiagnostic {
	return PMCompileDiagnostic{
		Severity: severity,
		Code:     code,
		Field:    field,
		Location: PMDiagnosticLocation{SourceID: source, Section: section, StartLine: start, EndLine: end},
		Reason:   reason,
		Impact:   impact,
		Repair:   repair,
	}
}

func pmScalarField(document pmParsedDocument, field string, aliases []string) (PMIRField, []PMCompileDiagnostic) {
	occurrences := pmSectionOccurrences(document, aliases)
	if len(occurrences) == 0 {
		return unresolvedPMField(), nil
	}
	values := []string{}
	refs := []PMSourceRef{}
	for _, occurrence := range occurrences {
		value := pmLinesText(occurrence.lines)
		if value == "" {
			continue
		}
		values = append(values, value)
		refs = append(refs, pmRefForLines(document.sourceID, occurrence.heading, occurrence.lines, occurrence.headingAt))
	}
	if len(values) == 0 {
		return unresolvedPMField(), nil
	}
	first := normalizePMComparison(values[0])
	for _, value := range values[1:] {
		if normalizePMComparison(value) != first {
			ref := refs[0]
			return PMIRField{Provenance: PMProvenanceUnresolved, Sources: refs}, []PMCompileDiagnostic{
				pmDiagnostic("error", PMDiagnosticAmbiguousField, field, ref.SourceID, ref.Section, ref.StartLine, ref.EndLine,
					"multiple sections provide different values for the same scalar field", "the compiler cannot select one value without product judgment", "merge the sections into one explicit value or record the choice as a decision"),
			}
		}
	}
	return PMIRField{Value: values[0], Provenance: PMProvenanceSupplied, Sources: refs}, nil
}

func pmCollectionField(document pmParsedDocument, aliases []string) PMIRCollection {
	occurrences := pmSectionOccurrences(document, aliases)
	items := []PMIRItem{}
	refs := []PMSourceRef{}
	for _, occurrence := range occurrences {
		for _, line := range occurrence.lines {
			value := strings.TrimSpace(line.text)
			value = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(value, "- [ ]"), "- [x]"), "-"))
			value = strings.TrimSpace(strings.TrimPrefix(value, "*"))
			if value == "" {
				continue
			}
			ref := PMSourceRef{SourceID: document.sourceID, Section: occurrence.heading, StartLine: line.number, EndLine: line.number}
			items = append(items, PMIRItem{Value: value, Provenance: PMProvenanceSupplied, Sources: []PMSourceRef{ref}})
			refs = append(refs, ref)
		}
	}
	provenance := PMProvenanceSupplied
	if len(items) == 0 {
		provenance = PMProvenanceUnresolved
	}
	return PMIRCollection{Items: items, Provenance: provenance, Sources: refs}
}

func conflictingPMCollectionDiagnostics(field string, collection PMIRCollection) []PMCompileDiagnostic {
	type normalizedItem struct {
		positive string
		negative bool
		item     PMIRItem
	}
	items := []normalizedItem{}
	for _, item := range collection.Items {
		value := normalizePMComparison(item.Value)
		negative := false
		for _, prefix := range []string{"must not ", "do not ", "not "} {
			if strings.HasPrefix(value, prefix) {
				negative = true
				value = strings.TrimSpace(strings.TrimPrefix(value, prefix))
				break
			}
		}
		items = append(items, normalizedItem{positive: value, negative: negative, item: item})
	}
	out := []PMCompileDiagnostic{}
	seen := map[string]normalizedItem{}
	for _, item := range items {
		previous, ok := seen[item.positive]
		if ok && previous.negative != item.negative && item.positive != "" {
			ref := firstPMItemRef(item.item)
			out = append(out, pmDiagnostic("error", PMDiagnosticConflictingState, field, ref.SourceID, ref.Section, ref.StartLine, ref.EndLine,
				"the same state is both required and prohibited", "the compiler preserves both statements and refuses to choose a winner", "remove one statement or add an explicit ranked decision policy resolving the conflict"))
			continue
		}
		seen[item.positive] = item
	}
	return out
}

func parsePMDocument(sourceID, content string) pmParsedDocument {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	document := pmParsedDocument{sourceID: sourceID, lines: lines, sections: map[string][]pmSectionOccurrence{}}
	var current *pmSectionOccurrence
	flush := func() {
		if current == nil {
			return
		}
		key := normalizePMHeading(current.heading)
		document.sections[key] = append(document.sections[key], *current)
	}
	for index, raw := range lines {
		lineNumber := index + 1
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "#") {
			heading := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			if heading != "" {
				flush()
				current = &pmSectionOccurrence{heading: heading, headingAt: lineNumber}
				continue
			}
		}
		line := pmDocumentLine{number: lineNumber, text: raw}
		if current == nil {
			document.preamble = append(document.preamble, line)
		} else {
			current.lines = append(current.lines, line)
		}
	}
	flush()
	return document
}

func pmSectionOccurrences(document pmParsedDocument, aliases []string) []pmSectionOccurrence {
	out := []pmSectionOccurrence{}
	for _, alias := range aliases {
		out = append(out, document.sections[normalizePMHeading(alias)]...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].headingAt < out[j].headingAt })
	return out
}

func pmLinesText(lines []pmDocumentLine) string {
	values := make([]string, 0, len(lines))
	for _, line := range lines {
		values = append(values, strings.TrimRight(line.text, " \t"))
	}
	return strings.TrimSpace(strings.Join(values, "\n"))
}

func pmFieldFromLines(value, provenance, source, section string, lines []pmDocumentLine) PMIRField {
	return PMIRField{Value: strings.TrimSpace(value), Provenance: provenance, Sources: []PMSourceRef{pmRefForLines(source, section, lines, 0)}}
}

func pmRefForLines(source, section string, lines []pmDocumentLine, fallback int) PMSourceRef {
	start, end := 0, 0
	for _, line := range lines {
		if strings.TrimSpace(line.text) == "" {
			continue
		}
		if start == 0 {
			start = line.number
		}
		end = line.number
	}
	if start == 0 {
		start, end = fallback, fallback
	}
	return PMSourceRef{SourceID: source, Section: section, StartLine: start, EndLine: end}
}

func unresolvedPMField() PMIRField {
	return PMIRField{Provenance: PMProvenanceUnresolved, Sources: []PMSourceRef{}}
}

func firstPMCollectionRef(collection PMIRCollection) PMSourceRef {
	if len(collection.Sources) > 0 {
		return collection.Sources[0]
	}
	return PMSourceRef{SourceID: "intent"}
}

func firstPMItemRef(item PMIRItem) PMSourceRef {
	if len(item.Sources) > 0 {
		return item.Sources[0]
	}
	return PMSourceRef{SourceID: "intent"}
}

func newPMIRSource(id, kind, ref, content string) PMIRSource {
	return PMIRSource{ID: id, Kind: kind, Ref: strings.TrimSpace(ref), Digest: pmDigest(content), Lines: len(strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")), Content: content}
}

func digestPMIRSources(sources []PMIRSource) string {
	parts := make([]string, 0, len(sources))
	for _, source := range sources {
		parts = append(parts, source.ID+":"+source.Digest)
	}
	return pmDigest(strings.Join(parts, "\n"))
}

func pmDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func normalizePMHeading(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("/", " ", "_", " ", "-", " ").Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func normalizePMComparison(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func sortPMDiagnostics(diagnostics []PMCompileDiagnostic) {
	severityRank := map[string]int{"error": 0, "warning": 1, "info": 2}
	sort.SliceStable(diagnostics, func(i, j int) bool {
		if severityRank[diagnostics[i].Severity] != severityRank[diagnostics[j].Severity] {
			return severityRank[diagnostics[i].Severity] < severityRank[diagnostics[j].Severity]
		}
		if diagnostics[i].Code != diagnostics[j].Code {
			return diagnostics[i].Code < diagnostics[j].Code
		}
		return diagnostics[i].Field < diagnostics[j].Field
	})
}

func summarizePMCompile(report PMCompileReport) PMCompileSummary {
	summary := PMCompileSummary{SourceCount: len(report.IR.Sources)}
	for _, diagnostic := range report.Diagnostics {
		switch diagnostic.Severity {
		case "error":
			summary.Errors++
		case "warning":
			summary.Warnings++
		default:
			summary.Info++
		}
	}
	provenances := []string{
		report.IR.Premise.Provenance,
		report.IR.Actor.Provenance,
		report.IR.Problem.Provenance,
		report.IR.DesiredOutcome.Provenance,
		report.IR.Constraints.Provenance,
		report.IR.NonGoals.Provenance,
		report.IR.Authority.Provenance,
		report.IR.Evidence.Provenance,
		report.IR.Assumptions.Provenance,
		report.IR.DecisionDebt.Provenance,
		report.IR.SuccessConditions.Provenance,
		report.IR.CandidateWork.Provenance,
	}
	for _, provenance := range provenances {
		if provenance == PMProvenanceUnresolved {
			summary.Unresolved++
		} else {
			summary.Resolved++
		}
	}
	return summary
}

func pmCompileDetailCommand(input PMCompileInput) string {
	parts := []string{"gira pm compile"}
	if strings.TrimSpace(input.Repo) != "" {
		parts = append(parts, "--repo", QuoteShellArg(input.Repo))
	}
	if input.Goal != nil && input.Goal.Number > 0 {
		parts = append(parts, "--goal", fmt.Sprintf("%d", input.Goal.Number))
	}
	parts = append(parts, "--from-file request.md --json")
	return strings.Join(parts, " ")
}

func FormatPMCompile(report PMCompileReport) string {
	const maxCompactDiagnostics = 8
	var b strings.Builder
	fmt.Fprintf(&b, "pm compile: errors=%d warnings=%d info=%d resolved=%d unresolved=%d sources=%d\n", report.Summary.Errors, report.Summary.Warnings, report.Summary.Info, report.Summary.Resolved, report.Summary.Unresolved, report.Summary.SourceCount)
	fmt.Fprintf(&b, "ir: %s %s\n", report.IR.SchemaVersion, report.IR.SourceDigest)
	visible := report.Diagnostics
	if len(visible) > maxCompactDiagnostics {
		visible = visible[:maxCompactDiagnostics]
	}
	for _, diagnostic := range visible {
		location := diagnostic.Location.SourceID + ":" + diagnostic.Location.Section
		if diagnostic.Location.StartLine > 0 {
			location += fmt.Sprintf(":%d", diagnostic.Location.StartLine)
		}
		fmt.Fprintf(&b, "- %s %s %s@%s: %s; fix: %s\n", diagnostic.Severity, diagnostic.Code, diagnostic.Field, location, diagnostic.Reason, diagnostic.Repair)
	}
	if omitted := len(report.Diagnostics) - len(visible); omitted > 0 {
		fmt.Fprintf(&b, "- %d additional diagnostics omitted; use --json for the full report\n", omitted)
	}
	fmt.Fprintf(&b, "detail: %s\n", report.DetailCommand)
	return b.String()
}

func FormatPMCompileJSON(report PMCompileReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}
