package gira

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const GoalPMViewSchemaVersion = "goal-pm-view/v1"

var goalPMViewKinds = []string{"operator", "human", "ai", "stakeholder", "audit"}

type GoalPMView struct {
	SchemaVersion string                 `json:"schema_version"`
	Kind          string                 `json:"kind"`
	StateDigest   string                 `json:"state_digest"`
	Sources       []GoalDossierSource    `json:"sources"`
	Premise       string                 `json:"premise,omitempty"`
	Delivery      GoalPMDeliverySummary  `json:"delivery"`
	Outcome       GoalPMOutcomeSummary   `json:"outcome"`
	Summaries     []string               `json:"summaries,omitempty"`
	Deltas        []GoalPMDelta          `json:"deltas,omitempty"`
	References    []GoalPMReference      `json:"references,omitempty"`
	Residual      []GoalPMResidual       `json:"residual_decisions,omitempty"`
	Audit         []GoalPMAuditRecord    `json:"audit,omitempty"`
	ExpandCommand string                 `json:"expand_command"`
	Budget        *GoalPMHydrationBudget `json:"budget,omitempty"`
}

type GoalPMDeliverySummary struct {
	Total    int `json:"total"`
	Ready    int `json:"ready"`
	Active   int `json:"active"`
	InReview int `json:"in_review"`
	Blocked  int `json:"blocked"`
	Done     int `json:"done"`
}

type GoalPMOutcomeSummary struct {
	Outcomes      int    `json:"outcomes"`
	Validated     int    `json:"validated"`
	NotValidated  int    `json:"not_validated"`
	Inconclusive  int    `json:"inconclusive"`
	Limited       int    `json:"limited"`
	LatestVerdict string `json:"latest_verdict,omitempty"`
}

type GoalPMDelta struct {
	Kind   string `json:"kind"`
	From   string `json:"from,omitempty"`
	To     string `json:"to"`
	Reason string `json:"reason"`
	Ref    string `json:"ref"`
}

type GoalPMReference struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Status  string `json:"status,omitempty"`
	Summary string `json:"summary,omitempty"`
	Ref     string `json:"ref"`
}

type GoalPMResidual struct {
	Target          string   `json:"target"`
	Reason          string   `json:"reason"`
	Capability      string   `json:"capability"`
	Options         []string `json:"options"`
	Authority       string   `json:"authority"`
	Impact          string   `json:"impact"`
	ResumeCondition string   `json:"resume_condition"`
}

type GoalPMAuditRecord struct {
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	Current       bool     `json:"current"`
	Status        string   `json:"status"`
	Supersedes    string   `json:"supersedes,omitempty"`
	RecordedAt    string   `json:"recorded_at"`
	SourceRefs    []string `json:"source_refs"`
	CommentRef    string   `json:"comment_ref,omitempty"`
	ContentDigest string   `json:"content_digest"`
}

type GoalPMHydrationBudget struct {
	Characters int `json:"characters"`
	References int `json:"references"`
}

func BuildGoalPMView(kind string, compile PMCompileReport, observe PMObserveReport) (GoalPMView, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		kind = "operator"
	}
	if !containsPMValue(goalPMViewKinds, kind) {
		return GoalPMView{}, fmt.Errorf("view must be operator, human, ai, stakeholder, or audit")
	}
	view := GoalPMView{SchemaVersion: GoalPMViewSchemaVersion, Kind: kind, Premise: compactGoalPMText(compile.IR.Premise.Value), Sources: goalPMViewSources(observe), Delivery: goalPMDelivery(observe), Outcome: goalPMOutcome(observe), ExpandCommand: fmt.Sprintf("gira goal report %d --repo %s --view audit --json", observe.Ticket, observe.Repo)}
	view.References = goalPMReferences(observe)
	view.Deltas = goalPMDeltas(observe)
	view.Residual = goalPMResiduals(observe)
	view.StateDigest = goalPMStateDigest(compile.IR.SourceDigest, observe, view.References, view.Deltas)
	switch kind {
	case "operator":
		view.Summaries = goalPMOperatorSummary(observe)
		view.References = limitGoalPMReferences(view.References, 12)
	case "human":
		view.Summaries = goalPMHumanSummary(observe)
		view.References = limitGoalPMReferences(view.References, 20)
	case "ai":
		view.Summaries = goalPMAISummary(observe)
		view.References = limitGoalPMReferences(view.References, 24)
		view.Deltas = limitGoalPMDeltas(view.Deltas, 16)
		view.Residual = limitGoalPMResiduals(view.Residual, 8)
		view = fitGoalPMAIView(view, 6000)
	case "stakeholder":
		view.Summaries = goalPMStakeholderSummary(observe)
		view.References = goalPMStakeholderReferences(view.References)
		view.Premise = compactGoalPMText(compile.IR.DesiredOutcome.Value)
	case "audit":
		view.Summaries = []string{"Immutable PM records and supersession metadata; expand references for source content."}
		view.Audit = goalPMAudit(observe)
		view.References = nil
	}
	return view, nil
}

func goalPMViewSources(observe PMObserveReport) []GoalDossierSource {
	values := []GoalDossierSource{{Name: "pm_ir", SchemaVersion: PMIRSchemaVersion}, {Name: "pm_context", SchemaVersion: PMContextReportSchemaVersion}, {Name: "pm_discovery", SchemaVersion: PMDiscoveryReportSchemaVersion}, {Name: "pm_measurement", SchemaVersion: PMMeasurementReportSchemaVersion}, {Name: "pm_work_graph", SchemaVersion: PMWorkGraphReportSchemaVersion}, {Name: "pm_observe", SchemaVersion: PMObserveSchemaVersion}, {Name: "pm_acceptance", SchemaVersion: PMAcceptanceResultSchemaVersion}, {Name: "goal_status", SchemaVersion: GoalStatusSchemaVersion}}
	return values
}

func goalPMDelivery(observe PMObserveReport) GoalPMDeliverySummary {
	out := GoalPMDeliverySummary{}
	if observe.GoalStatus == nil {
		return out
	}
	out.Total = len(observe.GoalStatus.Children)
	for _, child := range observe.GoalStatus.Children {
		switch child.Category {
		case "ready":
			out.Ready++
		case "in_progress":
			out.Active++
		case "in_review":
			out.InReview++
		case "blocked":
			out.Blocked++
		case "done":
			out.Done++
		}
	}
	return out
}

func goalPMOutcome(observe PMObserveReport) GoalPMOutcomeSummary {
	out := GoalPMOutcomeSummary{}
	if observe.Measurement != nil {
		s := observe.Measurement.Summary
		out.Outcomes = s.Outcomes
		out.Validated = s.Validated
		out.NotValidated = s.NotValidated
		out.Inconclusive = s.Inconclusive
		out.Limited = s.Limited
	}
	if observe.Acceptance != nil {
		out.LatestVerdict = observe.Acceptance.OutcomeState
	}
	return out
}

func goalPMReferences(observe PMObserveReport) []GoalPMReference {
	out := []GoalPMReference{}
	if observe.Context != nil {
		for _, item := range observe.Context.Records {
			if !item.Current {
				continue
			}
			r := item.Record
			out = append(out, GoalPMReference{Kind: r.Kind, ID: r.ID, Status: goalPMRecordStatus(r), Summary: compactGoalPMText(r.Text), Ref: emptyGoalPMRef(item.CommentURL, "pm:"+r.ID)})
		}
	}
	if observe.Acceptance != nil {
		out = append(out, GoalPMReference{Kind: "acceptance", ID: observe.Acceptance.ID, Status: observe.Acceptance.DeliveryState + "/" + observe.Acceptance.OutcomeState, Summary: compactGoalPMText(observe.Acceptance.Reason), Ref: "acceptance:" + observe.Acceptance.ID})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func goalPMDeltas(observe PMObserveReport) []GoalPMDelta {
	out := []GoalPMDelta{}
	if observe.Change.Changed {
		out = append(out, GoalPMDelta{Kind: "plan", From: observe.Change.PreviousPlanID, To: observe.Change.CurrentDigest, Reason: observe.Change.Reason, Ref: fmt.Sprintf("issue:%s#%d", observe.Repo, observe.Ticket)})
	}
	if observe.Acceptance != nil && observe.Acceptance.Supersedes != "" {
		out = append(out, GoalPMDelta{Kind: "acceptance", From: observe.Acceptance.Supersedes, To: observe.Acceptance.ID, Reason: observe.Acceptance.Reason, Ref: "acceptance:" + observe.Acceptance.ID})
	}
	if observe.Context != nil {
		for _, item := range observe.Context.Records {
			if item.Current && item.Record.Supersedes != "" {
				out = append(out, GoalPMDelta{Kind: item.Record.Kind, From: item.Record.Supersedes, To: item.Record.ID, Reason: "typed PM state supersession", Ref: emptyGoalPMRef(item.CommentURL, "pm:"+item.Record.ID)})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].To < out[j].To
	})
	return out
}

func goalPMResiduals(observe PMObserveReport) []GoalPMResidual {
	out := []GoalPMResidual{}
	for _, action := range observe.Actions {
		if !action.Residual {
			continue
		}
		out = append(out, GoalPMResidual{
			Target:     action.Target,
			Reason:     action.Reason,
			Capability: action.Capability,
			Options: []string{
				"approve with rationale and evidence",
				"request a safer decomposition or more evidence",
				"defer and record the cost of delay",
			},
			Authority:       "explicit accountable decision owner",
			Impact:          "safe independent work continues; authority-bound mutation remains deferred",
			ResumeCondition: "record an accepted decision with rationale and source evidence",
		})
	}
	return out
}

func goalPMAudit(observe PMObserveReport) []GoalPMAuditRecord {
	out := []GoalPMAuditRecord{}
	if observe.Context == nil {
		return out
	}
	for _, item := range observe.Context.Records {
		r := item.Record
		sum := sha256.Sum256([]byte(r.Text))
		out = append(out, GoalPMAuditRecord{ID: r.ID, Kind: r.Kind, Current: item.Current, Status: goalPMRecordStatus(r), Supersedes: r.Supersedes, RecordedAt: r.RecordedAt, SourceRefs: append([]string(nil), r.SourceRefs...), CommentRef: item.CommentURL, ContentDigest: "sha256:" + hex.EncodeToString(sum[:])})
	}
	return out
}

func goalPMOperatorSummary(observe PMObserveReport) []string {
	return []string{fmt.Sprintf("diagnoses=%d actions=%d plan_changed=%t", len(observe.Diagnoses), len(observe.Actions), observe.Change.Changed), goalPMNextJudgment(observe)}
}
func goalPMHumanSummary(observe PMObserveReport) []string {
	return []string{"Changed: " + emptyGoalPMText(observe.Change.Reason, "no prior plan delta"), "Why: " + goalPMDiagnosisSummary(observe), "Learned: " + goalPMLearningSummary(observe), "Next judgment: " + goalPMNextJudgment(observe)}
}
func goalPMAISummary(observe PMObserveReport) []string {
	return []string{"Hydrate references first; expand only the IDs needed for the next action.", goalPMDiagnosisSummary(observe), goalPMNextJudgment(observe)}
}
func goalPMStakeholderSummary(observe PMObserveReport) []string {
	return []string{goalPMOutcomeSentence(observe), goalPMRiskSentence(observe), goalPMDecisionSentence(observe)}
}

func goalPMDiagnosisSummary(observe PMObserveReport) string {
	if len(observe.Diagnoses) == 0 {
		return "no active PM diagnosis"
	}
	parts := []string{}
	for _, d := range observe.Diagnoses {
		parts = append(parts, d.Code+":"+d.Target)
	}
	return strings.Join(parts, ", ")
}
func goalPMLearningSummary(observe PMObserveReport) string {
	parts := []string{}
	if observe.Context != nil {
		for _, item := range observe.Context.Records {
			if item.Current && item.Record.Kind == "learning" {
				parts = append(parts, compactGoalPMText(item.Record.Text))
			}
		}
	}
	if len(parts) == 0 {
		return "no current learning receipt"
	}
	return strings.Join(parts, "; ")
}
func goalPMNextJudgment(observe PMObserveReport) string {
	for _, a := range observe.Actions {
		if a.Residual {
			return a.Kind + " " + a.Target + ": " + a.Reason
		}
	}
	if len(observe.Actions) > 0 {
		return observe.Actions[0].Kind + " " + observe.Actions[0].Target
	}
	return "report current state"
}
func goalPMOutcomeSentence(observe PMObserveReport) string {
	o := goalPMOutcome(observe)
	return fmt.Sprintf("Outcome evidence: validated=%d not_validated=%d inconclusive=%d limited=%d.", o.Validated, o.NotValidated, o.Inconclusive, o.Limited)
}
func goalPMRiskSentence(observe PMObserveReport) string {
	return fmt.Sprintf("Active product risks and diagnoses: %d; residual authority decisions: %d.", len(observe.Diagnoses), len(goalPMResiduals(observe)))
}
func goalPMDecisionSentence(observe PMObserveReport) string {
	count := 0
	if observe.Context != nil {
		for _, r := range observe.Context.Records {
			if r.Current && r.Record.Kind == "decision" {
				count++
			}
		}
	}
	return fmt.Sprintf("Current decisions: %d; plan changed=%t.", count, observe.Change.Changed)
}

func goalPMStakeholderReferences(values []GoalPMReference) []GoalPMReference {
	out := []GoalPMReference{}
	for _, v := range values {
		if containsPMValue([]string{"outcome", "decision", "risk", "acceptance", "measurement"}, v.Kind) {
			v.Summary = ""
			out = append(out, v)
		}
	}
	return out
}
func limitGoalPMReferences(values []GoalPMReference, n int) []GoalPMReference {
	if len(values) <= n {
		return values
	}
	return append([]GoalPMReference(nil), values[:n]...)
}
func limitGoalPMDeltas(values []GoalPMDelta, n int) []GoalPMDelta {
	if len(values) <= n {
		return values
	}
	return append([]GoalPMDelta(nil), values[:n]...)
}
func limitGoalPMResiduals(values []GoalPMResidual, n int) []GoalPMResidual {
	if len(values) <= n {
		return values
	}
	return append([]GoalPMResidual(nil), values[:n]...)
}
func fitGoalPMAIView(view GoalPMView, limit int) GoalPMView {
	view.Budget = &GoalPMHydrationBudget{}
	for {
		encoded, _ := json.Marshal(view)
		view.Budget.Characters = len(encoded)
		view.Budget.References = len(view.References)
		encoded, _ = json.Marshal(view)
		if len(encoded) <= limit {
			view.Budget.Characters = len(encoded)
			return view
		}
		switch {
		case len(view.References) > 0:
			view.References = view.References[:len(view.References)-1]
		case len(view.Deltas) > 0:
			view.Deltas = view.Deltas[:len(view.Deltas)-1]
		case len(view.Residual) > 0:
			view.Residual = view.Residual[:len(view.Residual)-1]
		default:
			return view
		}
	}
}
func goalPMStateDigest(ir string, observe PMObserveReport, refs []GoalPMReference, deltas []GoalPMDelta) string {
	encoded, _ := json.Marshal(struct {
		IR         string
		Context    string
		Plan       string
		Acceptance string
		Snapshot   PMObserveSnapshot
		Delivery   GoalPMDeliverySummary
		Outcome    GoalPMOutcomeSummary
		PMAction   string
		Refs       []GoalPMReference
		Deltas     []GoalPMDelta
	}{
		IR:         ir,
		Context:    observe.Snapshot.ContextDigest,
		Plan:       observe.Snapshot.GraphPlanID,
		Acceptance: observe.Snapshot.AcceptanceID,
		Snapshot:   observe.Snapshot,
		Delivery:   goalPMDelivery(observe),
		Outcome:    goalPMOutcome(observe),
		PMAction:   observe.Change.CurrentDigest,
		Refs:       refs,
		Deltas:     deltas,
	})
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func goalPMRecordStatus(r PMLedgerRecord) string {
	for _, v := range []string{r.OutcomeState, r.ExperimentState, r.Conclusion, r.Status} {
		if v != "" {
			return v
		}
	}
	return "unknown"
}
func compactGoalPMText(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 160 {
		return value[:157] + "..."
	}
	return value
}
func emptyGoalPMRef(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
func emptyGoalPMText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func FormatGoalPMView(view GoalPMView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "pm view: %s digest=%s delivery=%d/%d outcome_validated=%d/%d\n", view.Kind, view.StateDigest, view.Delivery.Done, view.Delivery.Total, view.Outcome.Validated, view.Outcome.Outcomes)
	for _, s := range view.Summaries {
		fmt.Fprintf(&b, "- %s\n", s)
	}
	for _, r := range view.Residual {
		fmt.Fprintf(&b, "- residual %s: %s resume=%s\n", r.Target, r.Reason, r.ResumeCondition)
	}
	return b.String()
}
