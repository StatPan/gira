package gira

import "testing"

func TestWorkflowCostProfileTicketLifecycleDefaultsToConservative(t *testing.T) {
	profile, ok := LookupWorkflowCostProfile("ticket lifecycle")
	if !ok {
		t.Fatal("ticket lifecycle profile not found")
	}
	if profile.Name != WorkflowCostProfileTicketLifecycle {
		t.Fatalf("profile name = %q, want %q", profile.Name, WorkflowCostProfileTicketLifecycle)
	}
	estimate, err := DefaultWorkflowCostEstimate(profile)
	if err != nil {
		t.Fatalf("DefaultWorkflowCostEstimate error: %v", err)
	}
	want := WorkflowCostBucketEstimate{RESTCore: 110, GraphQL: 24, Search: 1, WriteContent: 12}
	if estimate != want {
		t.Fatalf("estimate = %+v, want %+v", estimate, want)
	}
}

func TestWorkflowCostProfileWorkspaceStatusBuckets(t *testing.T) {
	profile, ok := LookupWorkflowCostProfile(WorkflowCostProfileWorkspaceStatus)
	if !ok {
		t.Fatal("workspace status profile not found")
	}
	optimistic, err := WorkflowCostEstimateForMode(profile, WorkflowCostModeOptimistic)
	if err != nil {
		t.Fatalf("WorkflowCostEstimateForMode optimistic error: %v", err)
	}
	conservative, err := WorkflowCostEstimateForMode(profile, WorkflowCostModeConservative)
	if err != nil {
		t.Fatalf("WorkflowCostEstimateForMode conservative error: %v", err)
	}
	if optimistic != (WorkflowCostBucketEstimate{RESTCore: 15, GraphQL: 3}) {
		t.Fatalf("optimistic = %+v", optimistic)
	}
	if conservative != (WorkflowCostBucketEstimate{RESTCore: 40, GraphQL: 8}) {
		t.Fatalf("conservative = %+v", conservative)
	}
}

func TestStaticWorkflowCostProfilesReturnsCopy(t *testing.T) {
	profiles := StaticWorkflowCostProfiles()
	if len(profiles) == 0 {
		t.Fatal("expected profiles")
	}
	profiles[0].Commands[0] = "changed"
	again := StaticWorkflowCostProfiles()
	if again[0].Commands[0] == "changed" {
		t.Fatal("StaticWorkflowCostProfiles exposed mutable command slice")
	}
}
