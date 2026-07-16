package gira

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const PMTaskPacketV2SchemaVersion = "gira-pm-task-packet/v2"
const PMTaskProfileSchemaVersion = "pm-task-profile/v1"
const PMProfilePromotionSchemaVersion = "pm-profile-promotion/v1"

var pmTaskProfileMarkerPattern = regexp.MustCompile(`<!--\s*gira:task-profile/v1\s+profile=([a-z_]+)\s*-->`)

type PMTaskProfileSpec struct {
	Name                 string   `json:"name"`
	Purpose              string   `json:"purpose"`
	RequiredSections     []string `json:"required_sections"`
	VerificationSections []string `json:"verification_sections"`
	SuggestedWorkerMode  string   `json:"suggested_worker_mode"`
}

type PMProfileReadinessReport struct {
	SchemaVersion string                   `json:"schema_version"`
	Profile       string                   `json:"profile"`
	Readiness     string                   `json:"readiness"`
	Findings      []TicketReadinessFinding `json:"findings"`
	NextAction    string                   `json:"next_action"`
}

type PMProfilePromotionReport struct {
	SchemaVersion string                   `json:"schema_version"`
	SourceProfile string                   `json:"source_profile"`
	TargetProfile string                   `json:"target_profile"`
	SourceRef     string                   `json:"source_ref"`
	Ready         bool                     `json:"ready"`
	Findings      []TicketReadinessFinding `json:"findings"`
}

func PMTaskProfileSpecs() []PMTaskProfileSpec {
	common := []string{"Actor", "Problem", "Desired Outcome", "Goal Alignment", "Parent Context", "Source References", "Non-goals"}
	profile := func(name, purpose, mode string, required, verification []string) PMTaskProfileSpec {
		return PMTaskProfileSpec{Name: name, Purpose: purpose, SuggestedWorkerMode: mode, RequiredSections: append(append([]string{}, common...), required...), VerificationSections: verification}
	}
	return []PMTaskProfileSpec{
		profile("discovery", "Reduce product uncertainty before selecting delivery work.", "research", []string{"Opportunity", "Evidence Gap", "Research Question"}, []string{"Learning Evidence"}),
		profile("decision", "Resolve one consequential choice with visible authority and alternatives.", "plan", []string{"Decision Question", "Options", "Decision Policy", "Authority"}, []string{"Decision Receipt"}),
		profile("experiment", "Test a risky assumption with a bounded learning loop.", "research", []string{"Hypothesis", "Assumption", "Experiment", "Success Conditions", "Stop Conditions"}, []string{"Experiment Evidence"}),
		profile("delivery", "Deliver a bounded outcome after material product uncertainty is resolved.", "implement", []string{"Product Uncertainty", "Acceptance Criteria", "Implementation Boundary", "Dependencies"}, []string{"Engineering Verification"}),
		profile("rollout", "Change exposure safely with guardrails and rollback.", "implement", []string{"Rollout Plan", "Reversibility", "Guardrails", "Rollback Plan"}, []string{"Rollout Evidence"}),
		profile("measurement", "Evaluate an outcome with a defined signal and observation window.", "research", []string{"Signal", "Baseline", "Target", "Measurement Window", "Data Source"}, []string{"Measurement Evidence"}),
		profile("documentation", "Close a knowledge gap for a named audience and source of truth.", "implement", []string{"Audience", "Knowledge Gap", "Documentation Boundary", "Source of Truth"}, []string{"Documentation Acceptance"}),
	}
}

func FindPMTaskProfile(name string) (PMTaskProfileSpec, bool) {
	name = normalizePMTaskProfile(name)
	for _, profile := range PMTaskProfileSpecs() {
		if profile.Name == name {
			return profile, true
		}
	}
	return PMTaskProfileSpec{}, false
}

func normalizePMTaskProfile(value string) string {
	value = strings.ReplaceAll(strings.ToLower(strings.TrimSpace(value)), "-", "_")
	if value == "" || value == "auto" {
		return "delivery"
	}
	return value
}

func PMTaskProfileFromBody(body string) string {
	match := pmTaskProfileMarkerPattern.FindStringSubmatch(body)
	if len(match) != 2 {
		return ""
	}
	return normalizePMTaskProfile(match[1])
}

func EvaluatePMProfileReadiness(body string) PMProfileReadinessReport {
	profileName := PMTaskProfileFromBody(body)
	report := PMProfileReadinessReport{SchemaVersion: PMTaskProfileSchemaVersion, Profile: profileName, Readiness: "ready", Findings: []TicketReadinessFinding{}, NextAction: "start_profile_work"}
	if profileName == "" {
		report.Readiness = "legacy"
		report.NextAction = "use_legacy_readiness"
		return report
	}
	profile, ok := FindPMTaskProfile(profileName)
	if !ok {
		report.Readiness = "needs_refinement"
		report.NextAction = "select_valid_profile"
		report.Findings = append(report.Findings, ticketReadinessFinding("error", "invalid_pm_profile", fmt.Sprintf("PM task profile %q is unsupported.", profileName), "Use discovery, decision, experiment, delivery, rollout, measurement, or documentation."))
		return report
	}
	for _, section := range append(append([]string{}, profile.RequiredSections...), profile.VerificationSections...) {
		if emptyPMProfileSection(markdownSection(body, section)) {
			report.Findings = append(report.Findings, ticketReadinessFinding("error", "missing_profile_field", fmt.Sprintf("%s profile requires %s.", profile.Name, section), fmt.Sprintf("Fill the %s section with task-specific evidence or a bounded contract.", section)))
		}
	}
	if profile.Name == "delivery" {
		uncertainty := strings.ToLower(strings.TrimSpace(markdownSection(body, "Product Uncertainty")))
		if uncertainty != "resolved" {
			report.Findings = append(report.Findings, ticketReadinessFinding("error", "unresolved_product_uncertainty", "Delivery profile requires Product Uncertainty to be exactly `resolved`.", "Move unresolved product work to discovery, decision, or experiment, then promote it back with evidence."))
		}
	}
	if hasTicketReadinessSeverity(report.Findings, "error") {
		report.Readiness = "needs_refinement"
		report.NextAction = "repair_profile_contract"
	}
	return report
}

func EvaluatePMProfilePromotion(sourceBody, targetBody, sourceRef string) PMProfilePromotionReport {
	report := PMProfilePromotionReport{
		SchemaVersion: PMProfilePromotionSchemaVersion,
		SourceProfile: PMTaskProfileFromBody(sourceBody), TargetProfile: PMTaskProfileFromBody(targetBody),
		SourceRef: strings.TrimSpace(sourceRef), Findings: []TicketReadinessFinding{},
	}
	if !containsPMValue([]string{"discovery", "decision", "experiment"}, report.SourceProfile) {
		report.Findings = append(report.Findings, ticketReadinessFinding("error", "invalid_promotion_source", "Delivery promotion must originate from discovery, decision, or experiment work.", "Select the source packet that resolved the product uncertainty."))
	}
	if sourceReadiness := EvaluatePMProfileReadiness(sourceBody); sourceReadiness.Readiness != "ready" {
		report.Findings = append(report.Findings, ticketReadinessFinding("error", "source_profile_not_ready", "Source profile still has unresolved readiness findings.", "Finish the source learning or decision contract before promotion."))
	}
	if report.TargetProfile != "delivery" {
		report.Findings = append(report.Findings, ticketReadinessFinding("error", "invalid_promotion_target", "Promotion target is not a delivery profile.", "Render a delivery profile for implementation-ready work."))
	}
	if targetReadiness := EvaluatePMProfileReadiness(targetBody); targetReadiness.Readiness != "ready" {
		report.Findings = append(report.Findings, ticketReadinessFinding("error", "target_profile_not_ready", "Delivery target does not satisfy its profile contract.", "Resolve every delivery readiness finding before worker handoff."))
	}
	if report.SourceRef == "" || !strings.Contains(markdownSection(targetBody, "Parent Context"), report.SourceRef) || !strings.Contains(markdownSection(targetBody, "Source References"), report.SourceRef) {
		report.Findings = append(report.Findings, ticketReadinessFinding("error", "missing_promotion_reference", "Delivery target does not retain the source packet in Parent Context and Source References.", "Add the stable source reference to both sections."))
	}
	report.Ready = !hasTicketReadinessSeverity(report.Findings, "error")
	return report
}

func emptyPMProfileSection(value string) bool {
	trimmed := strings.TrimSpace(value)
	if emptyReadinessSection(trimmed) {
		return true
	}
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(lower, "_required:") || strings.HasPrefix(lower, "required:") || strings.HasPrefix(lower, "todo:")
}

func RenderPMTaskProfileMarkdown(report PMTaskSpecReport, profile PMTaskProfileSpec) string {
	var b strings.Builder
	b.WriteString(PMStateMarker + "\n")
	fmt.Fprintf(&b, "<!-- gira:task-packet schema=%s -->\n", PMTaskPacketV2SchemaVersion)
	fmt.Fprintf(&b, "<!-- gira:task-profile/v1 profile=%s -->\n\n", profile.Name)
	fmt.Fprintf(&b, "# %s\n\n", report.Title)
	fmt.Fprintf(&b, "Profile: `%s` — %s\n\n", profile.Name, profile.Purpose)
	if report.Repo != "" {
		fmt.Fprintf(&b, "Repository: `%s`\n\n", report.Repo)
	}
	b.WriteString("## Raw Intent\n\n" + report.RawIntent + "\n\n")
	for _, section := range profile.RequiredSections {
		fmt.Fprintf(&b, "## %s\n\n", section)
		if section == "Parent Context" && len(report.ContextRefs) > 0 {
			for _, ref := range report.ContextRefs {
				fmt.Fprintf(&b, "- %s\n", ref)
			}
			b.WriteString("\n")
			continue
		}
		if section == "Product Uncertainty" {
			b.WriteString("_Required: use exactly `resolved`, or choose discovery/decision/experiment._\n\n")
			continue
		}
		fmt.Fprintf(&b, "_Required: %s-specific content._\n\n", profile.Name)
	}
	for _, section := range profile.VerificationSections {
		fmt.Fprintf(&b, "## %s\n\n_Required: inspectable pass/fail evidence._\n\n", section)
	}
	b.WriteString("## Suggested Worker Mode\n\n" + report.SuggestedWorkerMode + "\n\n")
	b.WriteString("## Next Action\n\nFill only this profile's required fields, rerun readiness, and promote to delivery only after product uncertainty is resolved.\n")
	return strings.TrimSpace(b.String()) + "\n"
}

func PMTaskProfileNames() []string {
	values := []string{"legacy"}
	for _, profile := range PMTaskProfileSpecs() {
		values = append(values, profile.Name)
	}
	sort.Strings(values)
	return values
}
