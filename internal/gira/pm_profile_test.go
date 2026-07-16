package gira

import (
	"fmt"
	"strings"
	"testing"
)

func TestPMTaskProfilesHaveDeterministicContracts(t *testing.T) {
	profiles := PMTaskProfileSpecs()
	if len(profiles) != 7 {
		t.Fatalf("profile count=%d, want 7", len(profiles))
	}
	seen := map[string]bool{}
	for _, profile := range profiles {
		if seen[profile.Name] || profile.Purpose == "" || profile.SuggestedWorkerMode == "" || len(profile.RequiredSections) == 0 || len(profile.VerificationSections) == 0 {
			t.Fatalf("invalid profile: %#v", profile)
		}
		seen[profile.Name] = true
		rendered, err := BuildPMTaskSpecReport(PMTaskSpecInput{Title: profile.Name, RawIntent: "Profile-specific intent.", Profile: profile.Name, ContextRefs: []string{"issue:OWNER/repo#100"}})
		if err != nil {
			t.Fatal(err)
		}
		for _, section := range append(append([]string{}, profile.RequiredSections...), profile.VerificationSections...) {
			if !strings.Contains(rendered.Markdown, "## "+section+"\n") {
				t.Errorf("%s renderer omitted %s", profile.Name, section)
			}
		}
		body := readyPMProfileBody(profile)
		readiness := EvaluatePMProfileReadiness(body)
		if readiness.Readiness != "ready" || len(readiness.Findings) != 0 {
			t.Errorf("%s should be ready: %#v\n%s", profile.Name, readiness, body)
		}
	}
}

func TestPMProfileReadinessIsProfileSpecific(t *testing.T) {
	discovery, _ := FindPMTaskProfile("discovery")
	discoveryBody := readyPMProfileBody(discovery)
	if strings.Contains(discoveryBody, "Implementation Boundary") || strings.Contains(discoveryBody, "Engineering Verification") {
		t.Fatalf("discovery packet should not require implementation details:\n%s", discoveryBody)
	}
	if got := EvaluateTicketReadiness(discoveryBody, []string{"type:spike", "status:ready"}, "open"); got.Readiness != "ready" {
		t.Fatalf("ready discovery rejected by generic ticket rules: %#v", got)
	}

	delivery, _ := FindPMTaskProfile("delivery")
	deliveryBody := strings.Replace(readyPMProfileBody(delivery), "## Product Uncertainty\n\nresolved", "## Product Uncertainty\n\nunresolved customer value", 1)
	got := EvaluatePMProfileReadiness(deliveryBody)
	if got.Readiness != "needs_refinement" || !hasTicketReadinessKind(got.Findings, "unresolved_product_uncertainty") {
		t.Fatalf("delivery masqueraded with unresolved product uncertainty: %#v", got)
	}
}

func TestBuildPMTaskSpecDefaultsToCompactDeliveryAndPreservesLegacy(t *testing.T) {
	input := PMTaskSpecInput{Title: "Profile packet", Repo: "OWNER/repo", RawIntent: "Deliver a bounded improvement.", ContextRefs: []string{"issue:OWNER/repo#100"}}
	compact, err := BuildPMTaskSpecReport(input)
	if err != nil {
		t.Fatal(err)
	}
	legacyInput := input
	legacyInput.Profile = "legacy"
	legacy, err := BuildPMTaskSpecReport(legacyInput)
	if err != nil {
		t.Fatal(err)
	}
	if compact.SchemaVersion != PMTaskPacketV2SchemaVersion || compact.Profile != "delivery" || compact.ProfileSchema != PMTaskProfileSchemaVersion || compact.SuggestedWorkerMode != "implement" {
		t.Fatalf("unexpected compact default: %#v", compact)
	}
	if compact.ProfileReadiness == nil || compact.ProfileReadiness.Readiness != "needs_refinement" || !strings.Contains(compact.Markdown, "profile=delivery") || !strings.Contains(compact.Markdown, "issue:OWNER/repo#100") {
		t.Fatalf("compact packet contract missing: %#v\n%s", compact.ProfileReadiness, compact.Markdown)
	}
	if legacy.SchemaVersion != PMTaskPacketSchemaVersion || legacy.Profile != "legacy" || !strings.Contains(legacy.Markdown, "schema=gira-pm-task-packet/v1") {
		t.Fatalf("legacy packet compatibility lost: %#v", legacy)
	}
	if len(compact.Markdown)*2 >= len(legacy.Markdown) {
		t.Fatalf("compact default is not materially smaller: compact=%d legacy=%d", len(compact.Markdown), len(legacy.Markdown))
	}
}

func TestBuildPMTaskSpecRejectsUnknownProfile(t *testing.T) {
	_, err := BuildPMTaskSpecReport(PMTaskSpecInput{RawIntent: "intent", Profile: "universal-magic"})
	if err == nil || !strings.Contains(err.Error(), "unsupported PM task profile") {
		t.Fatalf("expected profile error, got %v", err)
	}
}

func TestEvaluatePMProfilePromotionRequiresResolvedReferencedSource(t *testing.T) {
	discovery, _ := FindPMTaskProfile("discovery")
	delivery, _ := FindPMTaskProfile("delivery")
	source := readyPMProfileBody(discovery)
	target := readyPMProfileBody(delivery)
	target = strings.Replace(target, "## Parent Context\n\nTask-specific parent context.", "## Parent Context\n\nissue:OWNER/repo#100", 1)
	target = strings.Replace(target, "## Source References\n\nTask-specific source references.", "## Source References\n\nissue:OWNER/repo#100", 1)
	ready := EvaluatePMProfilePromotion(source, target, "issue:OWNER/repo#100")
	if !ready.Ready || ready.SchemaVersion != PMProfilePromotionSchemaVersion {
		t.Fatalf("valid promotion rejected: %#v", ready)
	}
	broken := EvaluatePMProfilePromotion(source, strings.Replace(target, "resolved", "unresolved", 1), "issue:OWNER/repo#100")
	if broken.Ready || !hasTicketReadinessKind(broken.Findings, "target_profile_not_ready") {
		t.Fatalf("unresolved delivery promotion accepted: %#v", broken)
	}
}

func TestMixedProfileTicketsUseTheirOwnReadinessContracts(t *testing.T) {
	discovery, _ := FindPMTaskProfile("discovery")
	delivery, _ := FindPMTaskProfile("delivery")
	legacy := "## Goal\nShip legacy work.\n\n## Scope\nBounded change.\n\n## Acceptance Criteria\n- Passes tests.\n\n## Expected Evidence\n- go test ./...\n"
	tests := []struct {
		name   string
		body   string
		labels []string
		want   string
	}{
		{name: "discovery", body: readyPMProfileBody(discovery), labels: []string{"type:spike", "status:ready"}, want: "ready"},
		{name: "delivery", body: strings.Replace(readyPMProfileBody(delivery), "resolved", "unresolved", 1), labels: []string{"type:task", "status:ready"}, want: "needs_refinement"},
		{name: "legacy", body: legacy, labels: []string{"type:task", "status:ready"}, want: "ready"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := EvaluateTicketReadiness(test.body, test.labels, "open"); got.Readiness != test.want {
				t.Fatalf("readiness=%s want=%s findings=%#v", got.Readiness, test.want, got.Findings)
			}
		})
	}
}

func readyPMProfileBody(profile PMTaskProfileSpec) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n<!-- gira:task-profile/v1 profile=%s -->\n\n", PMStateMarker, profile.Name)
	sections := append(append([]string{}, profile.RequiredSections...), profile.VerificationSections...)
	for _, section := range sections {
		value := "Task-specific " + strings.ToLower(section) + "."
		if section == "Product Uncertainty" {
			value = "resolved"
		}
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", section, value)
	}
	return b.String()
}

func hasTicketReadinessKind(findings []TicketReadinessFinding, kind string) bool {
	for _, finding := range findings {
		if finding.Kind == kind {
			return true
		}
	}
	return false
}
