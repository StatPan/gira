package gira

import (
	"fmt"
	"strings"
)

const (
	WorkflowCostModeOptimistic   = "optimistic"
	WorkflowCostModeConservative = "conservative"

	WorkflowCostProfileStatus           = "status"
	WorkflowCostProfileWorkspaceStatus  = "workspace-status"
	WorkflowCostProfileQueueNext        = "queue-next"
	WorkflowCostProfileQueueTake        = "queue-take"
	WorkflowCostProfileTicketStatusView = "ticket-status-view"
	WorkflowCostProfileTicketLifecycle  = "ticket-lifecycle"
	WorkflowCostProfileGoalStatusNext   = "goal-status-next"
)

type WorkflowCostProfile struct {
	Name         string                     `json:"name"`
	Summary      string                     `json:"summary"`
	Commands     []string                   `json:"commands"`
	DefaultMode  string                     `json:"default_mode"`
	Optimistic   WorkflowCostBucketEstimate `json:"optimistic"`
	Conservative WorkflowCostBucketEstimate `json:"conservative"`
	Notes        []string                   `json:"notes,omitempty"`
}

type WorkflowCostBucketEstimate struct {
	RESTCore     int `json:"rest_core"`
	GraphQL      int `json:"graphql"`
	Search       int `json:"search"`
	WriteContent int `json:"write_content"`
}

var staticWorkflowCostProfiles = []WorkflowCostProfile{
	{
		Name:        WorkflowCostProfileStatus,
		Summary:     "Compact single-repo status inspection.",
		Commands:    []string{"gira status", "gira stats repo"},
		DefaultMode: WorkflowCostModeConservative,
		Optimistic: WorkflowCostBucketEstimate{
			RESTCore: 3,
			GraphQL:  1,
		},
		Conservative: WorkflowCostBucketEstimate{
			RESTCore: 6,
			GraphQL:  2,
		},
	},
	{
		Name:        WorkflowCostProfileWorkspaceStatus,
		Summary:     "Bounded multi-repo workspace status pass over the configured repo allowlist.",
		Commands:    []string{"gira workspace status"},
		DefaultMode: WorkflowCostModeConservative,
		Optimistic: WorkflowCostBucketEstimate{
			RESTCore: 15,
			GraphQL:  3,
		},
		Conservative: WorkflowCostBucketEstimate{
			RESTCore: 40,
			GraphQL:  8,
		},
		Notes: []string{"Repo count and enabled views can change actual cost; this profile intentionally stays fixed for first-pass planning."},
	},
	{
		Name:        WorkflowCostProfileQueueNext,
		Summary:     "Read-only agent queue selection for the next handoff-safe ticket.",
		Commands:    []string{"gira queue next", "gira queue handoff"},
		DefaultMode: WorkflowCostModeConservative,
		Optimistic: WorkflowCostBucketEstimate{
			RESTCore: 10,
			GraphQL:  2,
			Search:   1,
		},
		Conservative: WorkflowCostBucketEstimate{
			RESTCore: 24,
			GraphQL:  4,
			Search:   1,
		},
	},
	{
		Name:        WorkflowCostProfileQueueTake,
		Summary:     "Queue selection plus ticket-start handoff mutation.",
		Commands:    []string{"gira queue take"},
		DefaultMode: WorkflowCostModeConservative,
		Optimistic: WorkflowCostBucketEstimate{
			RESTCore:     18,
			GraphQL:      3,
			Search:       1,
			WriteContent: 2,
		},
		Conservative: WorkflowCostBucketEstimate{
			RESTCore:     35,
			GraphQL:      6,
			Search:       1,
			WriteContent: 3,
		},
	},
	{
		Name:        WorkflowCostProfileTicketStatusView,
		Summary:     "Read-only ticket inspection, including linked PR and check context when available.",
		Commands:    []string{"gira ticket status", "gira ticket view"},
		DefaultMode: WorkflowCostModeConservative,
		Optimistic: WorkflowCostBucketEstimate{
			RESTCore: 8,
			GraphQL:  2,
		},
		Conservative: WorkflowCostBucketEstimate{
			RESTCore: 18,
			GraphQL:  4,
		},
	},
	{
		Name:        WorkflowCostProfileTicketLifecycle,
		Summary:     "Typical issue-to-branch-to-PR-to-finish flow using Gira lifecycle commands.",
		Commands:    []string{"gira ticket start", "gira ticket pr", "gira ticket self-review", "gira ticket checks", "gira ticket wait", "gira ticket finish"},
		DefaultMode: WorkflowCostModeConservative,
		Optimistic: WorkflowCostBucketEstimate{
			RESTCore:     60,
			GraphQL:      12,
			WriteContent: 7,
		},
		Conservative: WorkflowCostBucketEstimate{
			RESTCore:     110,
			GraphQL:      24,
			Search:       1,
			WriteContent: 12,
		},
		Notes: []string{"Includes lifecycle writes, PR creation, self-review note, merge/finish receipt, and repeated check polling pressure."},
	},
	{
		Name:        WorkflowCostProfileGoalStatusNext,
		Summary:     "Goal graph inspection and next child-ticket selection.",
		Commands:    []string{"gira goal status", "gira goal next"},
		DefaultMode: WorkflowCostModeConservative,
		Optimistic: WorkflowCostBucketEstimate{
			RESTCore: 18,
			GraphQL:  4,
		},
		Conservative: WorkflowCostBucketEstimate{
			RESTCore: 45,
			GraphQL:  8,
			Search:   1,
		},
	},
}

func StaticWorkflowCostProfiles() []WorkflowCostProfile {
	profiles := make([]WorkflowCostProfile, len(staticWorkflowCostProfiles))
	for i, profile := range staticWorkflowCostProfiles {
		profiles[i] = cloneWorkflowCostProfile(profile)
	}
	return profiles
}

func LookupWorkflowCostProfile(name string) (WorkflowCostProfile, bool) {
	normalized := normalizeWorkflowCostProfileName(name)
	for _, profile := range staticWorkflowCostProfiles {
		if profile.Name == normalized {
			return cloneWorkflowCostProfile(profile), true
		}
	}
	return WorkflowCostProfile{}, false
}

func DefaultWorkflowCostEstimate(profile WorkflowCostProfile) (WorkflowCostBucketEstimate, error) {
	return WorkflowCostEstimateForMode(profile, profile.DefaultMode)
}

func WorkflowCostEstimateForMode(profile WorkflowCostProfile, mode string) (WorkflowCostBucketEstimate, error) {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "", WorkflowCostModeConservative:
		return profile.Conservative, nil
	case WorkflowCostModeOptimistic:
		return profile.Optimistic, nil
	default:
		return WorkflowCostBucketEstimate{}, fmt.Errorf("unknown workflow cost mode %q", mode)
	}
}

func cloneWorkflowCostProfile(profile WorkflowCostProfile) WorkflowCostProfile {
	profile.Commands = append([]string(nil), profile.Commands...)
	profile.Notes = append([]string(nil), profile.Notes...)
	return profile
}

func normalizeWorkflowCostProfileName(name string) string {
	normalized := strings.TrimSpace(strings.ToLower(name))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	normalized = strings.ReplaceAll(normalized, "/", "-")
	normalized = strings.Join(strings.Fields(normalized), "-")
	return normalized
}
