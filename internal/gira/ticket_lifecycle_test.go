package gira

import "testing"

func TestTicketLifecycleStateRoundTripsStartMode(t *testing.T) {
	for _, mode := range []string{BranchStartModeAuto, BranchStartModeExplicit} {
		body := RenderTicketLifecycleBlock(TicketLifecycleState{BaseBranch: "main", BranchPolicyMode: BranchPolicyModeGitHubFlow, StartMode: mode, WorkBranch: "team/work"})
		state := ParseTicketLifecycleState(body)
		if state.StartMode != mode {
			t.Fatalf("start mode = %q, want %q; body=%s", state.StartMode, mode, body)
		}
	}
}

func TestRecordedLifecycleStartModeControlsRetries(t *testing.T) {
	issue := devStartIssue{Number: 956, Body: RenderTicketLifecycleBlock(TicketLifecycleState{
		BaseBranch: "main", BaseSource: "recorded_ticket_base", BranchPolicyMode: BranchPolicyModeGitHubFlow,
		StartMode: BranchStartModeExplicit, WorkBranch: "team/work", WorkBranchSource: "current",
	})}
	start, err := resolveTicketStartBase(RepoRef{Owner: "StatPan", Name: "gira"}, issue, "", nil)
	if err != nil || start.StartMode != BranchStartModeExplicit {
		t.Fatalf("recorded start mode lost on ticket start retry: %+v err=%v", start, err)
	}
	pr, err := resolveTicketPRBase(RepoRef{Owner: "StatPan", Name: "gira"}, issue, nil)
	if err != nil || pr.StartMode != BranchStartModeExplicit {
		t.Fatalf("recorded start mode lost on PR resolution: %+v err=%v", pr, err)
	}
}
