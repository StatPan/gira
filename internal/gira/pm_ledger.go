package gira

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const PMLedgerRecordSchemaVersion = "pm-ledger-record/v1"
const PMRecordReportSchemaVersion = "pm-record-report/v1"
const PMContextReportSchemaVersion = "pm-context/v1"

const pmLedgerRecordMarker = "<!-- gira:pm-ledger-record/v1 -->"

const (
	PMLedgerDiagnosticConflictID          = "PML001_CONFLICTING_RECORD_ID"
	PMLedgerDiagnosticMissingSupersession = "PML002_MISSING_SUPERSESSION_TARGET"
	PMLedgerDiagnosticSupersessionCycle   = "PML003_SUPERSESSION_CYCLE"
	PMLedgerDiagnosticSensitiveContent    = "PML004_SENSITIVE_CONTENT"
	PMLedgerDiagnosticDivergentHistory    = "PML005_DIVERGENT_SUPERSESSION"
	PMLedgerDiagnosticInvalidRecord       = "PML006_INVALID_RECORD"
)

var pmLedgerIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

type PMLedgerRecord struct {
	SchemaVersion string   `json:"schema_version"`
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	Text          string   `json:"text"`
	SourceRefs    []string `json:"source_refs"`
	ActorKind     string   `json:"actor_kind"`
	RecordedAt    string   `json:"recorded_at"`
	Status        string   `json:"status"`
	Supersedes    string   `json:"supersedes,omitempty"`
}

type PMLedgerDiagnostic struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	RecordID string `json:"record_id,omitempty"`
	Reason   string `json:"reason"`
	Impact   string `json:"impact"`
	Repair   string `json:"repair"`
}

type PMRecordInput struct {
	Repo       RepoRef
	Ticket     int
	ID         string
	Kind       string
	Text       string
	SourceRefs []string
	ActorKind  string
	Status     string
	Supersedes string
	RecordedAt time.Time
	DryRun     bool
	Apply      bool
}

type PMRecordAction struct {
	Action string `json:"action"`
	Status string `json:"status"`
	Target string `json:"target"`
	Detail string `json:"detail,omitempty"`
}

type PMRecordReport struct {
	Command       string               `json:"command"`
	SchemaVersion string               `json:"schema_version"`
	Repo          string               `json:"repo"`
	Ticket        int                  `json:"ticket"`
	DryRun        bool                 `json:"dry_run"`
	Record        PMLedgerRecord       `json:"record"`
	Diagnostics   []PMLedgerDiagnostic `json:"diagnostics"`
	Actions       []PMRecordAction     `json:"actions"`
	Idempotent    bool                 `json:"idempotent"`
	NextStep      string               `json:"next_step"`
	Approval      *ApprovalEvidence    `json:"approval,omitempty"`
}

type PMContextInput struct {
	Repo   RepoRef
	Ticket int
}

type PMContextIssue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

type PMContextRecord struct {
	Record           PMLedgerRecord `json:"record"`
	Current          bool           `json:"current"`
	CommentURL       string         `json:"comment_url"`
	GitHubAuthor     string         `json:"github_author"`
	CommentCreatedAt string         `json:"comment_created_at"`
}

type PMLegacyEvidenceRef struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Summary   string `json:"summary"`
}

type PMContextSummary struct {
	Records        int            `json:"records"`
	Current        int            `json:"current"`
	Superseded     int            `json:"superseded"`
	LegacyEvidence int            `json:"legacy_evidence"`
	ByKind         map[string]int `json:"by_kind"`
}

type PMContextReport struct {
	Command        string                `json:"command"`
	SchemaVersion  string                `json:"schema_version"`
	ReadOnly       bool                  `json:"read_only"`
	Repo           string                `json:"repo"`
	Issue          PMContextIssue        `json:"issue"`
	Records        []PMContextRecord     `json:"records"`
	LegacyEvidence []PMLegacyEvidenceRef `json:"legacy_evidence"`
	Diagnostics    []PMLedgerDiagnostic  `json:"diagnostics"`
	Summary        PMContextSummary      `json:"summary"`
	DetailCommand  string                `json:"detail_command"`
}

type pmContextGitHubIssue struct {
	Number   int    `json:"number"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	URL      string `json:"url"`
	Comments []struct {
		Body      string `json:"body"`
		CreatedAt string `json:"createdAt"`
		URL       string `json:"url"`
		Author    struct {
			Login string `json:"login"`
		} `json:"author"`
	} `json:"comments"`
}

func BuildPMRecordReport(input PMRecordInput, runner CommandRunner) (PMRecordReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	report := PMRecordReport{
		Command: "pm record", SchemaVersion: PMRecordReportSchemaVersion,
		Repo: input.Repo.FullName(), Ticket: input.Ticket, DryRun: input.DryRun,
		Diagnostics: []PMLedgerDiagnostic{}, Actions: []PMRecordAction{},
	}
	if input.DryRun == input.Apply {
		return report, fmt.Errorf("exactly one of dry_run/apply is required")
	}
	record, diagnostics := normalizePMLedgerRecord(input)
	report.Record = record
	report.Diagnostics = append(report.Diagnostics, diagnostics...)
	if hasPMLedgerDiagnosticCode(diagnostics, PMLedgerDiagnosticSensitiveContent) {
		report.Record = redactSensitivePMLedgerRecord(report.Record)
	}
	if input.Ticket <= 0 {
		return report, fmt.Errorf("ticket must be > 0")
	}
	if hasPMLedgerErrors(report.Diagnostics) {
		report.NextStep = "repair PM record diagnostics"
		return report, fmt.Errorf("PM record validation failed")
	}
	context, err := BuildPMContextReport(PMContextInput{Repo: input.Repo, Ticket: input.Ticket}, runner)
	if err != nil {
		return report, err
	}
	report.Diagnostics = append(report.Diagnostics, context.Diagnostics...)
	if hasPMLedgerErrors(context.Diagnostics) {
		report.Actions = append(report.Actions, PMRecordAction{Action: "record:append", Status: "blocked", Target: record.ID, Detail: "existing ledger diagnostics"})
		report.NextStep = "repair existing PM ledger diagnostics"
		return report, fmt.Errorf("existing PM ledger is not safe to extend")
	}
	existing, conflict := findPMLedgerRecord(context.Records, record)
	if conflict != nil {
		report.Diagnostics = append(report.Diagnostics, *conflict)
		report.Actions = append(report.Actions, PMRecordAction{Action: "record:append", Status: "blocked", Target: record.ID, Detail: conflict.Code})
		report.NextStep = "choose a new record ID or explicitly supersede the existing record"
		return report, fmt.Errorf("PM ledger record %q conflicts with existing history", record.ID)
	}
	if existing {
		report.Idempotent = true
		report.Actions = append(report.Actions, PMRecordAction{Action: "record:append", Status: "skipped", Target: record.ID, Detail: "identical semantic record already exists"})
		report.NextStep = fmt.Sprintf("gira pm context --repo %s --ticket %d", report.Repo, report.Ticket)
		if input.DryRun {
			report.Approval = PMRecordApprovalEvidence(report)
		}
		return report, nil
	}
	if record.Supersedes != "" && !pmContextHasRecordID(context.Records, record.Supersedes) {
		diagnostic := pmLedgerDiagnostic("error", PMLedgerDiagnosticMissingSupersession, record.ID, "supersession target does not exist", "history cannot be resolved without fabricating a predecessor", "record the predecessor first or correct --supersedes")
		report.Diagnostics = append(report.Diagnostics, diagnostic)
		report.Actions = append(report.Actions, PMRecordAction{Action: "record:append", Status: "blocked", Target: record.ID, Detail: diagnostic.Code})
		report.NextStep = "repair PM record diagnostics"
		return report, fmt.Errorf("PM ledger supersession target %q was not found", record.Supersedes)
	}
	prospective := append(append([]PMContextRecord{}, context.Records...), PMContextRecord{Record: record})
	resolutionDiagnostics := resolvePMLedgerHistory(prospective)
	report.Diagnostics = append(report.Diagnostics, resolutionDiagnostics...)
	if hasPMLedgerErrors(resolutionDiagnostics) {
		report.Actions = append(report.Actions, PMRecordAction{Action: "record:append", Status: "blocked", Target: record.ID, Detail: "history resolution failed"})
		report.NextStep = "repair PM record diagnostics"
		return report, fmt.Errorf("PM ledger history resolution failed")
	}
	status := "planned"
	if input.Apply {
		if _, err := runner.Run("gh", "issue", "comment", strconv.Itoa(input.Ticket), "--repo", input.Repo.FullName(), "--body", RenderPMLedgerRecordComment(record)); err != nil {
			return report, fmt.Errorf("append PM ledger record: %w", err)
		}
		status = "applied"
	}
	report.Actions = append(report.Actions, PMRecordAction{Action: "record:append", Status: status, Target: record.ID, Detail: fmt.Sprintf("issue #%d typed comment", input.Ticket)})
	report.NextStep = fmt.Sprintf("gira pm context --repo %s --ticket %d", report.Repo, report.Ticket)
	if input.DryRun {
		report.Approval = PMRecordApprovalEvidence(report)
	}
	return report, nil
}

func BuildPMContextReport(input PMContextInput, runner CommandRunner) (PMContextReport, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	report := PMContextReport{
		Command: "pm context", SchemaVersion: PMContextReportSchemaVersion, ReadOnly: true,
		Repo: input.Repo.FullName(), Records: []PMContextRecord{}, LegacyEvidence: []PMLegacyEvidenceRef{}, Diagnostics: []PMLedgerDiagnostic{},
		Summary: PMContextSummary{ByKind: map[string]int{}},
	}
	if input.Ticket <= 0 {
		return report, fmt.Errorf("ticket must be > 0")
	}
	raw, err := runner.Run("gh", "issue", "view", strconv.Itoa(input.Ticket), "--repo", input.Repo.FullName(), "--json", "number,title,body,url,comments")
	if err != nil {
		return report, fmt.Errorf("hydrate PM context: %w", err)
	}
	var issue pmContextGitHubIssue
	if err := json.Unmarshal(raw, &issue); err != nil {
		return report, fmt.Errorf("parse PM context: %w", err)
	}
	if issue.Number != input.Ticket {
		return report, fmt.Errorf("parse PM context: expected issue #%d, got #%d", input.Ticket, issue.Number)
	}
	report.Issue = PMContextIssue{Number: issue.Number, Title: issue.Title, URL: issue.URL}
	report.DetailCommand = fmt.Sprintf("gira pm context --repo %s --ticket %d --json", input.Repo.FullName(), input.Ticket)
	if legacy := legacyPMEvidence(issue.Body, fmt.Sprintf("issue:%s#%d", input.Repo.FullName(), input.Ticket)); legacy != nil {
		if containsSensitivePMContent([]string{issue.Body}) {
			report.Diagnostics = append(report.Diagnostics, pmLedgerDiagnostic("warning", PMLedgerDiagnosticSensitiveContent, "", "legacy issue evidence resembles restricted content and was omitted", "compact hydration could otherwise repeat a secret or private transcript", "replace it with a safe source reference or redacted summary"))
		} else {
			report.LegacyEvidence = append(report.LegacyEvidence, *legacy)
		}
	}
	for index, comment := range issue.Comments {
		record, found, parseErr := ParsePMLedgerRecordComment(comment.Body)
		if parseErr != nil {
			report.Diagnostics = append(report.Diagnostics, pmLedgerDiagnostic("error", PMLedgerDiagnosticInvalidRecord, "", "typed PM ledger comment cannot be parsed", "current PM state may be incomplete", "repair or supersede the malformed typed comment"))
			continue
		}
		if found {
			validation := validateStoredPMLedgerRecord(record)
			report.Diagnostics = append(report.Diagnostics, validation...)
			if hasPMLedgerDiagnosticCode(validation, PMLedgerDiagnosticSensitiveContent) {
				record = redactSensitivePMLedgerRecord(record)
			}
			report.Records = append(report.Records, PMContextRecord{
				Record: record, CommentURL: comment.URL, GitHubAuthor: comment.Author.Login, CommentCreatedAt: comment.CreatedAt,
			})
			continue
		}
		ref := comment.URL
		if strings.TrimSpace(ref) == "" {
			ref = fmt.Sprintf("issue:%s#%d:comment:%d", input.Repo.FullName(), input.Ticket, index+1)
		}
		if legacy := legacyPMEvidence(comment.Body, ref); legacy != nil {
			if containsSensitivePMContent([]string{comment.Body}) {
				report.Diagnostics = append(report.Diagnostics, pmLedgerDiagnostic("warning", PMLedgerDiagnosticSensitiveContent, "", "legacy comment evidence resembles restricted content and was omitted", "compact hydration could otherwise repeat a secret or private transcript", "replace it with a safe source reference or redacted summary"))
			} else {
				report.LegacyEvidence = append(report.LegacyEvidence, *legacy)
			}
		}
	}
	report.Diagnostics = append(report.Diagnostics, resolvePMLedgerHistory(report.Records)...)
	markCurrentPMLedgerRecords(report.Records)
	sort.SliceStable(report.Records, func(i, j int) bool {
		if report.Records[i].Record.RecordedAt != report.Records[j].Record.RecordedAt {
			return report.Records[i].Record.RecordedAt < report.Records[j].Record.RecordedAt
		}
		return report.Records[i].Record.ID < report.Records[j].Record.ID
	})
	report.Summary.Records = len(report.Records)
	report.Summary.LegacyEvidence = len(report.LegacyEvidence)
	for _, item := range report.Records {
		if item.Current {
			report.Summary.Current++
			report.Summary.ByKind[item.Record.Kind]++
		} else {
			report.Summary.Superseded++
		}
	}
	sortPMLedgerDiagnostics(report.Diagnostics)
	return report, nil
}

func validateStoredPMLedgerRecord(record PMLedgerRecord) []PMLedgerDiagnostic {
	diagnostics := []PMLedgerDiagnostic{}
	recordedAt, err := time.Parse(time.RFC3339, record.RecordedAt)
	if err != nil {
		diagnostics = append(diagnostics, pmLedgerDiagnostic("error", PMLedgerDiagnosticInvalidRecord, record.ID, "recorded_at is not RFC3339", "record ordering and freshness are ambiguous", "supersede the malformed record with a valid RFC3339 timestamp"))
	}
	_, normalizedDiagnostics := normalizePMLedgerRecord(PMRecordInput{
		ID: record.ID, Kind: record.Kind, Text: record.Text, SourceRefs: record.SourceRefs,
		ActorKind: record.ActorKind, Status: record.Status, Supersedes: record.Supersedes, RecordedAt: recordedAt,
	})
	return append(diagnostics, normalizedDiagnostics...)
}

func normalizePMLedgerRecord(input PMRecordInput) (PMLedgerRecord, []PMLedgerDiagnostic) {
	record := PMLedgerRecord{
		SchemaVersion: PMLedgerRecordSchemaVersion,
		ID:            strings.TrimSpace(input.ID), Kind: normalizePMLedgerKind(input.Kind), Text: strings.TrimSpace(input.Text),
		SourceRefs: normalizePMLedgerRefs(input.SourceRefs), ActorKind: strings.ToLower(strings.TrimSpace(input.ActorKind)),
		Status: strings.ToLower(strings.TrimSpace(input.Status)), Supersedes: strings.TrimSpace(input.Supersedes),
	}
	if input.RecordedAt.IsZero() {
		input.RecordedAt = time.Now().UTC()
	}
	record.RecordedAt = input.RecordedAt.UTC().Format(time.RFC3339)
	diagnostics := []PMLedgerDiagnostic{}
	if !pmLedgerIDPattern.MatchString(record.ID) {
		diagnostics = append(diagnostics, pmLedgerDiagnostic("error", PMLedgerDiagnosticInvalidRecord, record.ID, "record ID is missing or invalid", "the record cannot be referenced or superseded safely", "use 1-128 letters, numbers, dots, colons, underscores, or hyphens"))
	}
	if !validPMLedgerKind(record.Kind) {
		diagnostics = append(diagnostics, pmLedgerDiagnostic("error", PMLedgerDiagnosticInvalidRecord, record.ID, "record kind is unsupported", "the ledger cannot apply deterministic lifecycle rules", "use context_source, evidence, inference, assumption, decision, question, or learning"))
	}
	if record.Text == "" {
		diagnostics = append(diagnostics, pmLedgerDiagnostic("error", PMLedgerDiagnosticInvalidRecord, record.ID, "record text is empty", "the typed record has no inspectable claim", "supply --text or --from-file"))
	}
	if len(record.SourceRefs) == 0 {
		diagnostics = append(diagnostics, pmLedgerDiagnostic("error", PMLedgerDiagnosticInvalidRecord, record.ID, "record has no source reference", "the claim cannot be inspected independently", "add at least one --source reference"))
	}
	if record.ActorKind == "" {
		record.ActorKind = "human"
	}
	if !containsPMValue([]string{"human", "ai", "system", "integration"}, record.ActorKind) {
		diagnostics = append(diagnostics, pmLedgerDiagnostic("error", PMLedgerDiagnosticInvalidRecord, record.ID, "actor kind is unsupported", "record authorship cannot be interpreted consistently", "use human, ai, system, or integration"))
	}
	if record.Status == "" {
		record.Status = defaultPMLedgerStatus(record.Kind)
	}
	if !validPMLedgerStatus(record.Kind, record.Status) {
		diagnostics = append(diagnostics, pmLedgerDiagnostic("error", PMLedgerDiagnosticInvalidRecord, record.ID, "status is invalid for the record kind", "lifecycle resolution would be ambiguous", "use a documented status for this record kind"))
	}
	if record.Supersedes == record.ID && record.ID != "" {
		diagnostics = append(diagnostics, pmLedgerDiagnostic("error", PMLedgerDiagnosticSupersessionCycle, record.ID, "record supersedes itself", "history contains a cycle", "reference a distinct predecessor ID"))
	}
	if containsSensitivePMContent(append([]string{record.Text}, record.SourceRefs...)) {
		diagnostics = append(diagnostics, pmLedgerDiagnostic("error", PMLedgerDiagnosticSensitiveContent, record.ID, "record resembles a secret or private transcript", "durable GitHub comments could disclose restricted content", "store only a safe reference or redacted summary"))
	}
	sortPMLedgerDiagnostics(diagnostics)
	return record, diagnostics
}

func RenderPMLedgerRecordComment(record PMLedgerRecord) string {
	encoded, _ := json.MarshalIndent(record, "", "  ")
	return pmLedgerRecordMarker + "\n\n```json\n" + string(encoded) + "\n```\n"
}

func ParsePMLedgerRecordComment(body string) (PMLedgerRecord, bool, error) {
	if !strings.Contains(body, pmLedgerRecordMarker) {
		return PMLedgerRecord{}, false, nil
	}
	after := strings.SplitN(body, pmLedgerRecordMarker, 2)[1]
	start := strings.Index(after, "```json")
	if start < 0 {
		return PMLedgerRecord{}, true, fmt.Errorf("missing JSON fence")
	}
	jsonBody := after[start+len("```json"):]
	end := strings.Index(jsonBody, "```")
	if end < 0 {
		return PMLedgerRecord{}, true, fmt.Errorf("unterminated JSON fence")
	}
	var record PMLedgerRecord
	if err := json.Unmarshal([]byte(strings.TrimSpace(jsonBody[:end])), &record); err != nil {
		return PMLedgerRecord{}, true, err
	}
	if record.SchemaVersion != PMLedgerRecordSchemaVersion {
		return PMLedgerRecord{}, true, fmt.Errorf("unsupported schema %q", record.SchemaVersion)
	}
	return record, true, nil
}

func findPMLedgerRecord(records []PMContextRecord, candidate PMLedgerRecord) (bool, *PMLedgerDiagnostic) {
	for _, item := range records {
		if item.Record.ID != candidate.ID {
			continue
		}
		if samePMLedgerSemantics(item.Record, candidate) {
			return true, nil
		}
		diagnostic := pmLedgerDiagnostic("error", PMLedgerDiagnosticConflictID, candidate.ID, "record ID already exists with different semantic content", "append-only history cannot silently overwrite the earlier claim", "use the identical record for retry or create a new ID with --supersedes")
		return false, &diagnostic
	}
	return false, nil
}

func samePMLedgerSemantics(a, b PMLedgerRecord) bool {
	return a.ID == b.ID && a.Kind == b.Kind && a.Text == b.Text && a.ActorKind == b.ActorKind && a.Status == b.Status && a.Supersedes == b.Supersedes && strings.Join(normalizePMLedgerRefs(a.SourceRefs), "\x00") == strings.Join(normalizePMLedgerRefs(b.SourceRefs), "\x00")
}

func resolvePMLedgerHistory(records []PMContextRecord) []PMLedgerDiagnostic {
	diagnostics := []PMLedgerDiagnostic{}
	byID := map[string]PMLedgerRecord{}
	children := map[string][]string{}
	for _, item := range records {
		record := item.Record
		if existing, ok := byID[record.ID]; ok && !samePMLedgerSemantics(existing, record) {
			diagnostics = append(diagnostics, pmLedgerDiagnostic("error", PMLedgerDiagnosticConflictID, record.ID, "record ID has conflicting append history", "current state cannot select a record deterministically", "append a new superseding record after repairing the duplicate ID"))
			continue
		}
		byID[record.ID] = record
		if record.Supersedes != "" {
			children[record.Supersedes] = append(children[record.Supersedes], record.ID)
		}
	}
	for predecessor, successors := range children {
		if _, ok := byID[predecessor]; !ok {
			diagnostics = append(diagnostics, pmLedgerDiagnostic("error", PMLedgerDiagnosticMissingSupersession, successors[0], "supersession target is absent", "history cannot resolve the predecessor", "restore the referenced record or append a corrected successor"))
		}
		unique := stableStringSlice(successors)
		if len(unique) > 1 {
			diagnostics = append(diagnostics, pmLedgerDiagnostic("error", PMLedgerDiagnosticDivergentHistory, predecessor, "multiple records supersede the same predecessor", "two current branches compete without an authorized winner", "append one record that explicitly supersedes the selected branch and resolve the other"))
		}
	}
	for id := range byID {
		seen := map[string]bool{}
		current := id
		for current != "" {
			if seen[current] {
				diagnostics = append(diagnostics, pmLedgerDiagnostic("error", PMLedgerDiagnosticSupersessionCycle, id, "supersession chain contains a cycle", "there is no deterministic current record", "append corrected records without cyclic predecessor references"))
				break
			}
			seen[current] = true
			current = byID[current].Supersedes
		}
	}
	sortPMLedgerDiagnostics(diagnostics)
	return diagnostics
}

func markCurrentPMLedgerRecords(records []PMContextRecord) {
	superseded := map[string]bool{}
	for _, item := range records {
		if item.Record.Supersedes != "" {
			superseded[item.Record.Supersedes] = true
		}
	}
	for index := range records {
		records[index].Current = !superseded[records[index].Record.ID]
	}
}

func pmContextHasRecordID(records []PMContextRecord, id string) bool {
	for _, item := range records {
		if item.Record.ID == id {
			return true
		}
	}
	return false
}

func normalizePMLedgerKind(value string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(value)), "-", "_")
}

func validPMLedgerKind(value string) bool {
	return containsPMValue([]string{"context_source", "evidence", "inference", "assumption", "decision", "question", "learning"}, value)
}

func defaultPMLedgerStatus(kind string) string {
	switch kind {
	case "assumption":
		return "proposed"
	case "decision":
		return "proposed"
	case "question":
		return "open"
	default:
		return "active"
	}
}

func validPMLedgerStatus(kind, status string) bool {
	allowed := map[string][]string{
		"context_source": {"active", "superseded", "revoked"},
		"evidence":       {"active", "superseded", "revoked"},
		"inference":      {"active", "superseded", "revoked"},
		"learning":       {"active", "superseded", "revoked"},
		"question":       {"open", "resolved", "superseded"},
		"assumption":     {"proposed", "testing", "supported", "invalidated", "expired"},
		"decision":       {"proposed", "accepted", "superseded", "revoked", "review_due"},
	}
	return containsPMValue(allowed[kind], status)
}

func containsPMValue(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func normalizePMLedgerRefs(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" && !seen[part] {
				seen[part] = true
				out = append(out, part)
			}
		}
	}
	sort.Strings(out)
	return out
}

func containsSensitivePMContent(values []string) bool {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)-----BEGIN (RSA |EC |OPENSSH |)PRIVATE KEY-----`),
		regexp.MustCompile(`(?i)(github_pat_|gh[pousr]_|sk-[A-Za-z0-9]{16,})`),
		regexp.MustCompile(`(?i)(password|passwd|api[_-]?key|access[_-]?token|secret)\s*[:=]\s*\S+`),
		regexp.MustCompile(`(?i)(private|hidden)\s+(chat|conversation|transcript)\s*:`),
	}
	for _, value := range values {
		for _, pattern := range patterns {
			if pattern.MatchString(value) {
				return true
			}
		}
	}
	return false
}

func legacyPMEvidence(body, ref string) *PMLegacyEvidenceRef {
	trimmed := strings.TrimSpace(body)
	lower := strings.ToLower(trimmed)
	if trimmed == "" || (!strings.HasPrefix(lower, "## decision") && !strings.HasPrefix(lower, "## evidence") && !strings.Contains(lower, "decision:")) {
		return nil
	}
	summary := strings.Join(strings.Fields(trimmed), " ")
	if len(summary) > 160 {
		summary = summary[:157] + "..."
	}
	return &PMLegacyEvidenceRef{Kind: "legacy_evidence", Reference: ref, Summary: summary}
}

func pmLedgerDiagnostic(severity, code, id, reason, impact, repair string) PMLedgerDiagnostic {
	return PMLedgerDiagnostic{Severity: severity, Code: code, RecordID: id, Reason: reason, Impact: impact, Repair: repair}
}

func hasPMLedgerErrors(diagnostics []PMLedgerDiagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			return true
		}
	}
	return false
}

func hasPMLedgerDiagnosticCode(diagnostics []PMLedgerDiagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func redactSensitivePMLedgerRecord(record PMLedgerRecord) PMLedgerRecord {
	record.Text = "[redacted: sensitive PM ledger content]"
	record.SourceRefs = []string{}
	return record
}

func sortPMLedgerDiagnostics(diagnostics []PMLedgerDiagnostic) {
	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].Code != diagnostics[j].Code {
			return diagnostics[i].Code < diagnostics[j].Code
		}
		return diagnostics[i].RecordID < diagnostics[j].RecordID
	})
}

func FormatPMRecord(report PMRecordReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "pm record: #%d id=%s kind=%s status=%s dry_run=%t idempotent=%t\n", report.Ticket, report.Record.ID, report.Record.Kind, report.Record.Status, report.DryRun, report.Idempotent)
	for _, diagnostic := range report.Diagnostics {
		fmt.Fprintf(&b, "- %s %s %s: %s; fix: %s\n", diagnostic.Severity, diagnostic.Code, diagnostic.RecordID, diagnostic.Reason, diagnostic.Repair)
	}
	for _, action := range report.Actions {
		fmt.Fprintf(&b, "- %s %s %s\n", action.Action, action.Status, action.Target)
	}
	fmt.Fprintf(&b, "next step: %s\n", report.NextStep)
	return b.String()
}

func FormatPMContext(report PMContextReport, budget int) string {
	if budget <= 0 {
		budget = 6000
	}
	detail := "detail: " + report.DetailCommand + "\n"
	header := fmt.Sprintf("pm context: %s#%d records=%d current=%d superseded=%d legacy=%d diagnostics=%d\n", report.Repo, report.Issue.Number, report.Summary.Records, report.Summary.Current, report.Summary.Superseded, report.Summary.LegacyEvidence, len(report.Diagnostics))
	lines := []string{}
	for _, diagnostic := range report.Diagnostics {
		lines = append(lines, fmt.Sprintf("- %s %s %s: %s", diagnostic.Severity, diagnostic.Code, diagnostic.RecordID, diagnostic.Reason))
	}
	for _, item := range report.Records {
		if !item.Current {
			continue
		}
		text := strings.Join(strings.Fields(item.Record.Text), " ")
		if len(text) > 160 {
			text = text[:157] + "..."
		}
		line := fmt.Sprintf("- %s %s [%s]: %s", item.Record.Kind, item.Record.ID, item.Record.Status, text)
		if len(item.Record.SourceRefs) > 0 {
			line += " refs=" + strings.Join(item.Record.SourceRefs, ",")
		}
		lines = append(lines, line)
	}
	for _, legacy := range report.LegacyEvidence {
		lines = append(lines, fmt.Sprintf("- legacy %s: %s", legacy.Reference, legacy.Summary))
	}
	var b strings.Builder
	b.WriteString(header)
	omitted := 0
	for index, line := range lines {
		remaining := len(lines) - index - 1
		reserve := len(detail) + 64
		if b.Len()+len(line)+1+reserve > budget {
			omitted = remaining + 1
			break
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	if omitted > 0 {
		fmt.Fprintf(&b, "- %d entries omitted by context budget\n", omitted)
	}
	b.WriteString(detail)
	output := b.String()
	if len(output) <= budget {
		return output
	}
	fallback := fmt.Sprintf("pm context: #%d current=%d\n- output reduced by context budget\ndetail: gira pm context --json\n", report.Issue.Number, report.Summary.Current)
	if len(fallback) <= budget {
		return fallback
	}
	return fallback[:budget]
}

func FormatPMContextJSON(report PMContextReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}
