package gira

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	PMHarnessPolicyVersion        = "gira-pm-policy/v1"
	PMHarnessProtocolVersion      = "gira-pm-harness/v1"
	PMBootstrapSchemaVersion      = "pm-bootstrap/v1"
	PMConformanceRunSchemaVersion = "pm-conformance-run/v1"
	PMConformanceSchemaVersion    = "pm-conformance-report/v1"
)

var pmHarnessStages = []string{"hydrate", "compile", "discover", "decide", "plan", "execute", "observe", "replan", "validate", "report"}

type PMBootstrapInput struct {
	Repo      RepoRef
	Ticket    int
	Role      string
	Authority []string
	Budget    int
}

type PMBootstrapReport struct {
	Command          string                `json:"command"`
	SchemaVersion    string                `json:"schema_version"`
	PolicyVersion    string                `json:"policy_version"`
	ProtocolVersion  string                `json:"protocol_version"`
	ReadOnly         bool                  `json:"read_only"`
	Repo             string                `json:"repo"`
	Ticket           int                   `json:"ticket"`
	Role             string                `json:"role"`
	Authority        []string              `json:"authority"`
	SessionID        string                `json:"session_id"`
	Context          []PMHarnessContextRef `json:"context_refs"`
	CurrentPlan      PMHarnessCurrentPlan  `json:"current_plan"`
	Protocol         []PMHarnessTransition `json:"protocol"`
	RequiredReceipts []string              `json:"required_receipts"`
	Next             PMHarnessNextAction   `json:"next"`
	Budget           PMHarnessBudget       `json:"budget"`
}

type PMHarnessContextRef struct {
	Kind          string `json:"kind"`
	SchemaVersion string `json:"schema_version"`
	Digest        string `json:"digest,omitempty"`
	ExpandCommand string `json:"expand_command"`
}

type PMHarnessCurrentPlan struct {
	Compiled             bool   `json:"compiled"`
	CompileDigest        string `json:"compile_digest"`
	WorkGraphPlanID      string `json:"work_graph_plan_id,omitempty"`
	RecommendationDigest string `json:"recommendation_digest,omitempty"`
	AcceptanceID         string `json:"acceptance_id,omitempty"`
}

type PMHarnessTransition struct {
	Stage    string `json:"stage"`
	Contract string `json:"contract"`
	Mutation bool   `json:"mutation"`
	Gate     string `json:"gate"`
	Receipt  string `json:"receipt"`
}

type PMHarnessNextAction struct {
	Stage              string `json:"stage"`
	Action             string `json:"action"`
	Command            string `json:"command"`
	RequiredCapability string `json:"required_capability,omitempty"`
	AuthoritySatisfied bool   `json:"authority_satisfied"`
	ResumeCondition    string `json:"resume_condition,omitempty"`
}

type PMHarnessBudget struct {
	Limit      int `json:"limit"`
	Characters int `json:"characters"`
}

func BuildPMBootstrapReport(input PMBootstrapInput, runner CommandRunner) (PMBootstrapReport, error) {
	role := strings.ToLower(strings.TrimSpace(input.Role))
	if role == "" {
		role = "human"
	}
	if !containsPMValue([]string{"human", "ai"}, role) {
		return PMBootstrapReport{}, fmt.Errorf("role must be human or ai")
	}
	if input.Ticket <= 0 {
		return PMBootstrapReport{}, fmt.Errorf("ticket must be > 0")
	}
	if input.Budget == 0 {
		input.Budget = 6000
	}
	if input.Budget < 2000 || input.Budget > 20000 {
		return PMBootstrapReport{}, fmt.Errorf("budget must be between 2000 and 20000")
	}
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	runner = &pmObserveCachedRunner{base: runner, results: map[string]*pmObserveCachedResult{}}
	compile, err := BuildPMCompileReportFromRequest(PMCompileRequest{Repo: input.Repo.FullName(), Goal: input.Ticket}, runner)
	if err != nil {
		return PMBootstrapReport{}, err
	}
	observe, err := BuildPMObserveReport(PMObserveInput{Repo: input.Repo, Ticket: input.Ticket}, runner)
	if err != nil {
		return PMBootstrapReport{}, err
	}
	authority := uniqueSortedPMValues(input.Authority)
	report := PMBootstrapReport{
		Command: "pm bootstrap", SchemaVersion: PMBootstrapSchemaVersion,
		PolicyVersion: PMHarnessPolicyVersion, ProtocolVersion: PMHarnessProtocolVersion,
		ReadOnly: true, Repo: input.Repo.FullName(), Ticket: input.Ticket, Role: role, Authority: authority,
		Context:          pmHarnessContextRefs(input.Repo, input.Ticket, compile, observe),
		CurrentPlan:      PMHarnessCurrentPlan{Compiled: compile.Summary.Errors == 0, CompileDigest: compile.IR.SourceDigest, WorkGraphPlanID: observe.Snapshot.GraphPlanID, RecommendationDigest: observe.Change.CurrentDigest, AcceptanceID: observe.Snapshot.AcceptanceID},
		Protocol:         pmHarnessProtocol(),
		RequiredReceipts: []string{"source refs", "compile digest", "approved plan fingerprint", "authority evidence for mutations", "verification evidence", "outcome verdict separate from delivery", "report state digest"},
	}
	report.Next = pmHarnessNextAction(input, compile, observe, authority)
	report.SessionID = pmHarnessSessionID(report)
	encoded, _ := json.Marshal(report)
	report.Budget = PMHarnessBudget{Limit: input.Budget, Characters: len(encoded)}
	encoded, _ = json.Marshal(report)
	report.Budget.Characters = len(encoded)
	if report.Budget.Characters > input.Budget {
		return PMBootstrapReport{}, fmt.Errorf("bounded bootstrap exceeds budget: %d > %d", report.Budget.Characters, input.Budget)
	}
	return report, nil
}

func pmHarnessContextRefs(repo RepoRef, ticket int, compile PMCompileReport, observe PMObserveReport) []PMHarnessContextRef {
	base := fmt.Sprintf("--repo %s --ticket %d --json", repo.FullName(), ticket)
	return []PMHarnessContextRef{
		{Kind: "intent", SchemaVersion: PMIRSchemaVersion, Digest: compile.IR.SourceDigest, ExpandCommand: fmt.Sprintf("gira pm compile --repo %s --goal %d --json", repo.FullName(), ticket)},
		{Kind: "context", SchemaVersion: PMContextReportSchemaVersion, Digest: observe.Snapshot.ContextDigest, ExpandCommand: "gira pm context " + base},
		{Kind: "discovery", SchemaVersion: PMDiscoveryReportSchemaVersion, ExpandCommand: "gira pm discovery " + base},
		{Kind: "measurement", SchemaVersion: PMMeasurementReportSchemaVersion, ExpandCommand: "gira pm measure " + base},
		{Kind: "work_graph", SchemaVersion: PMWorkGraphReportSchemaVersion, Digest: observe.Snapshot.GraphPlanID, ExpandCommand: fmt.Sprintf("gira goal graph %d --repo %s --compact-json", ticket, repo.FullName())},
		{Kind: "observation", SchemaVersion: PMObserveSchemaVersion, Digest: observe.Change.CurrentDigest, ExpandCommand: "gira pm observe " + base},
		{Kind: "report", SchemaVersion: GoalPMViewSchemaVersion, ExpandCommand: fmt.Sprintf("gira goal report %d --repo %s --view ai --json", ticket, repo.FullName())},
	}
}

func pmHarnessProtocol() []PMHarnessTransition {
	return []PMHarnessTransition{
		{Stage: "hydrate", Contract: PMBootstrapSchemaVersion, Gate: "bounded source refs", Receipt: "session_id"},
		{Stage: "compile", Contract: PMCompileReportSchemaVersion, Gate: "zero compile errors before lowering", Receipt: "pm-ir source_digest"},
		{Stage: "discover", Contract: PMDiscoveryReportSchemaVersion, Gate: "typed evidence and causal links", Receipt: "source-linked discovery state"},
		{Stage: "decide", Contract: PMLedgerRecordSchemaVersion, Mutation: true, Gate: "explicit options, rationale, and decision authority", Receipt: "append-safe decision record"},
		{Stage: "plan", Contract: PMWorkGraphReportSchemaVersion, Mutation: true, Gate: "matching --expect-plan and issue:create authority", Receipt: "work graph apply receipt"},
		{Stage: "execute", Contract: PMTaskPacketV2SchemaVersion, Mutation: true, Gate: "ticket lifecycle approval plan and repository capability", Receipt: "branch, PR, checks, and finish receipt"},
		{Stage: "observe", Contract: PMObserveSchemaVersion, Gate: "read current canonical state", Receipt: "recommendation digest"},
		{Stage: "replan", Contract: PMReplanSchemaVersion, Mutation: true, Gate: "matching --expect-plan and per-action capability", Receipt: "replan receipt and reason delta"},
		{Stage: "validate", Contract: PMAcceptanceReportSchemaVersion, Mutation: true, Gate: "typed evidence and explicit apply", Receipt: "acceptance and learning transition"},
		{Stage: "report", Contract: GoalPMViewSchemaVersion, Gate: "same canonical state digest", Receipt: "derived view refs"},
	}
}

func pmHarnessNextAction(input PMBootstrapInput, compile PMCompileReport, observe PMObserveReport, authority []string) PMHarnessNextAction {
	if compile.Summary.Errors > 0 {
		return PMHarnessNextAction{Stage: "compile", Action: "repair_intent", Command: fmt.Sprintf("gira pm compile --repo %s --goal %d --json", input.Repo.FullName(), input.Ticket), AuthoritySatisfied: true, ResumeCondition: "compile diagnostics contain no errors"}
	}
	action := PMObserveAction{Kind: "report", Capability: "report:read"}
	if len(observe.Actions) > 0 {
		action = observe.Actions[0]
	}
	satisfied := containsPMValue(authority, action.Capability) || strings.HasSuffix(action.Capability, ":read")
	next := PMHarnessNextAction{Stage: action.Kind, Action: action.Kind, Command: observe.NextStep, RequiredCapability: action.Capability, AuthoritySatisfied: satisfied}
	if !satisfied {
		next.ResumeCondition = "record explicit " + action.Capability + " authority or choose a read-only/decomposed action"
	}
	return next
}

func pmHarnessSessionID(report PMBootstrapReport) string {
	encoded, _ := json.Marshal(struct {
		Policy    string
		Protocol  string
		Repo      string
		Ticket    int
		Role      string
		Authority []string
		Plan      PMHarnessCurrentPlan
	}{report.PolicyVersion, report.ProtocolVersion, report.Repo, report.Ticket, report.Role, report.Authority, report.CurrentPlan})
	sum := sha256.Sum256(encoded)
	return "pms-" + hex.EncodeToString(sum[:8])
}

func uniqueSortedPMValues(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func FormatPMBootstrap(report PMBootstrapReport) string {
	return fmt.Sprintf("pm bootstrap: session=%s role=%s compiled=%t plan=%s\nnext: %s authority=%t\ncommand: %s\n", report.SessionID, report.Role, report.CurrentPlan.Compiled, report.CurrentPlan.WorkGraphPlanID, report.Next.Action, report.Next.AuthoritySatisfied, report.Next.Command)
}

type PMConformanceRun struct {
	SchemaVersion   string                        `json:"schema_version"`
	HostID          string                        `json:"host_id"`
	ActorKind       string                        `json:"actor_kind"`
	ModelConfig     string                        `json:"model_config,omitempty"`
	PolicyVersion   string                        `json:"policy_version"`
	ProtocolVersion string                        `json:"protocol_version"`
	Stages          []string                      `json:"stages"`
	Receipts        []string                      `json:"receipts"`
	Claims          []PMConformanceClaim          `json:"claims"`
	FailureAttempts []PMConformanceFailureAttempt `json:"failure_attempts"`
	Privacy         PMConformancePrivacy          `json:"privacy"`
	SemanticQuality string                        `json:"semantic_quality"`
}

type PMConformanceClaim struct {
	Claim      string   `json:"claim"`
	SourceRefs []string `json:"source_refs"`
}

type PMConformanceFailureAttempt struct {
	Mode    string `json:"mode"`
	Blocked bool   `json:"blocked"`
}

type PMConformancePrivacy struct {
	SecretsCaptured           bool `json:"secrets_captured"`
	PrivateTranscriptCaptured bool `json:"private_transcript_captured"`
	WorkerRanking             bool `json:"worker_ranking"`
	TokenProductivityScoring  bool `json:"token_productivity_scoring"`
}

type PMConformanceReport struct {
	Command           string                   `json:"command"`
	SchemaVersion     string                   `json:"schema_version"`
	PolicyVersion     string                   `json:"policy_version"`
	ProtocolVersion   string                   `json:"protocol_version"`
	ProtocolCompliant bool                     `json:"protocol_compliant"`
	SemanticQuality   string                   `json:"semantic_quality"`
	Runs              []PMConformanceRunResult `json:"runs"`
	Summary           PMConformanceSummary     `json:"summary"`
}

type PMConformanceRunResult struct {
	HostID            string   `json:"host_id"`
	ActorKind         string   `json:"actor_kind"`
	ModelConfig       string   `json:"model_config,omitempty"`
	ProtocolCompliant bool     `json:"protocol_compliant"`
	SemanticQuality   string   `json:"semantic_quality"`
	RecordedFailures  []string `json:"recorded_failures,omitempty"`
	Findings          []string `json:"findings,omitempty"`
}

type PMConformanceSummary struct {
	Runs             int `json:"runs"`
	HumanRuns        int `json:"human_runs"`
	AIConfigurations int `json:"ai_configurations"`
	Compliant        int `json:"compliant"`
	RecordedFailures int `json:"recorded_failures"`
	UnsafeMutations  int `json:"unsafe_mutations"`
}

func BuildPMConformanceReport(runs []PMConformanceRun) PMConformanceReport {
	if runs == nil {
		runs = BuiltinPMConformanceRuns()
	}
	report := PMConformanceReport{Command: "pm conformance", SchemaVersion: PMConformanceSchemaVersion, PolicyVersion: PMHarnessPolicyVersion, ProtocolVersion: PMHarnessProtocolVersion, ProtocolCompliant: true, SemanticQuality: "reported_separately", Runs: []PMConformanceRunResult{}}
	if len(runs) == 0 {
		report.ProtocolCompliant = false
		return report
	}
	aiConfigs := map[string]bool{}
	for _, run := range runs {
		result := evaluatePMConformanceRun(run)
		report.Runs = append(report.Runs, result)
		report.Summary.Runs++
		if run.ActorKind == "human" {
			report.Summary.HumanRuns++
		} else if run.ActorKind == "ai" {
			aiConfigs[run.ModelConfig] = true
		}
		if result.ProtocolCompliant {
			report.Summary.Compliant++
		} else {
			report.ProtocolCompliant = false
		}
		report.Summary.RecordedFailures += len(result.RecordedFailures)
		for _, finding := range result.Findings {
			if strings.HasPrefix(finding, "unsafe_mutation:") {
				report.Summary.UnsafeMutations++
			}
		}
	}
	report.Summary.AIConfigurations = len(aiConfigs)
	return report
}

func evaluatePMConformanceRun(run PMConformanceRun) PMConformanceRunResult {
	result := PMConformanceRunResult{HostID: run.HostID, ActorKind: run.ActorKind, ModelConfig: run.ModelConfig, ProtocolCompliant: true, SemanticQuality: run.SemanticQuality}
	if run.SchemaVersion != PMConformanceRunSchemaVersion || run.PolicyVersion != PMHarnessPolicyVersion || run.ProtocolVersion != PMHarnessProtocolVersion {
		result.Findings = append(result.Findings, "stale_or_unsupported_contract")
	}
	if !containsPMValue([]string{"human", "ai"}, run.ActorKind) || (run.ActorKind == "ai" && strings.TrimSpace(run.ModelConfig) == "") {
		result.Findings = append(result.Findings, "invalid_host_identity")
	}
	for _, stage := range pmHarnessStages {
		if !containsPMValue(run.Stages, stage) {
			result.Findings = append(result.Findings, "missing_stage:"+stage)
		}
	}
	for _, receipt := range []string{"compile_digest", "plan_fingerprint", "authority_evidence", "verification_evidence", "report_digest"} {
		if !containsPMValue(run.Receipts, receipt) {
			result.Findings = append(result.Findings, "missing_receipt:"+receipt)
		}
	}
	for _, claim := range run.Claims {
		if len(claim.SourceRefs) == 0 {
			result.Findings = append(result.Findings, "unsupported_claim:"+claim.Claim)
		}
	}
	for _, attempt := range run.FailureAttempts {
		result.RecordedFailures = append(result.RecordedFailures, attempt.Mode)
		if !attempt.Blocked {
			result.Findings = append(result.Findings, "unsafe_mutation:"+attempt.Mode)
		}
	}
	if run.Privacy.SecretsCaptured || run.Privacy.PrivateTranscriptCaptured || run.Privacy.WorkerRanking || run.Privacy.TokenProductivityScoring {
		result.Findings = append(result.Findings, "privacy_boundary_violation")
	}
	result.ProtocolCompliant = len(result.Findings) == 0
	return result
}

func BuiltinPMConformanceRuns() []PMConformanceRun {
	base := func(host, actor, model, quality string) PMConformanceRun {
		return PMConformanceRun{SchemaVersion: PMConformanceRunSchemaVersion, HostID: host, ActorKind: actor, ModelConfig: model, PolicyVersion: PMHarnessPolicyVersion, ProtocolVersion: PMHarnessProtocolVersion, Stages: append([]string(nil), pmHarnessStages...), Receipts: []string{"compile_digest", "plan_fingerprint", "authority_evidence", "verification_evidence", "report_digest"}, Claims: []PMConformanceClaim{{Claim: "delivery and outcome are separate", SourceRefs: []string{"acceptance:fixture", "measurement:fixture"}}}, SemanticQuality: quality}
	}
	human := base("human-cli", "human", "", "accepted")
	reference := base("ai-reference", "ai", "host-a/model-capable", "accepted")
	weak := base("ai-bounded", "ai", "host-b/model-weak", "limited")
	weak.FailureAttempts = []PMConformanceFailureAttempt{{Mode: "context_loss", Blocked: true}, {Mode: "premature_delivery", Blocked: true}, {Mode: "generic_human_escalation", Blocked: true}, {Mode: "unsupported_claim", Blocked: true}, {Mode: "authority_overreach", Blocked: true}}
	return []PMConformanceRun{human, reference, weak}
}

func FormatPMConformance(report PMConformanceReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "pm conformance: compliant=%t runs=%d ai_configs=%d unsafe_mutations=%d\n", report.ProtocolCompliant, report.Summary.Runs, report.Summary.AIConfigurations, report.Summary.UnsafeMutations)
	for _, run := range report.Runs {
		fmt.Fprintf(&b, "- %s actor=%s protocol=%t semantic=%s recorded_failures=%d\n", run.HostID, run.ActorKind, run.ProtocolCompliant, run.SemanticQuality, len(run.RecordedFailures))
	}
	return b.String()
}
