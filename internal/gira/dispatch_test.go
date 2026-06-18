package gira

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDispatchPacketFromGoalHandoffWrapsWorkerContext(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "backlog"}
	selected := GoalNextCandidate{Repo: "StatPan/gira", Number: 573, Title: "Add dispatch", URL: "https://github.com/StatPan/gira/issues/573"}
	worker := TicketHandoffReport{
		Command:              "ticket handoff",
		SchemaVersion:        WorkerHandoffSchemaVersion,
		Role:                 AgentPromptRoleImplementer,
		Profile:              AgentPromptProfileDefault,
		Repo:                 "StatPan/gira",
		Issue:                573,
		Title:                "Add dispatch",
		RequiredChecks:       []string{"go test ./internal/gira"},
		EvidenceExpectations: []string{"PR links selected child ticket"},
		NextAction:           "implement",
		NextSafeCommand:      "gira ticket pr --repo StatPan/gira --ticket 573 --dry-run",
		LinkedPR:             &TicketHandoffLinkedPR{Available: true, Number: 71, URL: "https://github.com/StatPan/gira/pull/71"},
	}
	handoff := GoalHandoffReport{
		Command:         "goal handoff",
		SchemaVersion:   GoalHandoffSchemaVersion,
		Repo:            repo.FullName(),
		Role:            AgentPromptRoleImplementer,
		Profile:         AgentPromptProfileDefault,
		Goal:            GoalStatusIssue{Number: 521, Title: "Dispatch goal", State: "open", Status: "Ready", URL: "https://github.com/StatPan/backlog/issues/521"},
		GoalContext:     GoalHandoffContext{Objective: "Issue official AI work orders", StopConditions: []string{"unclear acceptance"}},
		SelectedTicket:  &selected,
		WorkerHandoff:   &worker,
		NextAction:      "handoff_child",
		NextSafeCommand: worker.NextSafeCommand,
	}

	packet := dispatchPacketFromGoalHandoff(repo, handoff)
	if packet.SchemaVersion != DispatchPacketSchemaVersion || packet.Command != "dispatch goal" || packet.Source.Kind != "goal" {
		t.Fatalf("unexpected packet metadata: %+v", packet)
	}
	if len(packet.Authority) != 2 || packet.Authority[1].Kind != "selected_ticket" || packet.Authority[1].Repo != "StatPan/gira" {
		t.Fatalf("unexpected authority: %+v", packet.Authority)
	}
	if packet.WorkerHandoff == nil || packet.WorkerHandoff.SchemaVersion != WorkerHandoffSchemaVersion {
		t.Fatalf("missing worker handoff: %+v", packet.WorkerHandoff)
	}
	if packet.Instruction.Objective != "Issue official AI work orders" || !strings.Contains(packet.Instruction.SelectedWork, "StatPan/gira#573") {
		t.Fatalf("unexpected instruction: %+v", packet.Instruction)
	}
	if !containsString(packet.Instruction.StopConditions, "unclear acceptance") || !containsString(packet.Instruction.EvidenceRequired, "go test ./internal/gira") {
		t.Fatalf("instruction missing stop/evidence: %+v", packet.Instruction)
	}
	if len(packet.References) != 3 || packet.References[2].Kind != "linked_pr" {
		t.Fatalf("unexpected references: %+v", packet.References)
	}
	text := FormatDispatchPacket(packet)
	for _, want := range []string{"dispatch: dispatch goal", "selected work: StatPan/gira#573", "next safe command:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted dispatch missing %q:\n%s", want, text)
		}
	}
}

func TestDispatchPacketFromGoalHandoffCarriesStopReasons(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "backlog"}
	handoff := GoalHandoffReport{
		Command:         "goal handoff",
		SchemaVersion:   GoalHandoffSchemaVersion,
		Repo:            repo.FullName(),
		Role:            AgentPromptRoleImplementer,
		Profile:         AgentPromptProfileDefault,
		Goal:            GoalStatusIssue{Number: 521, Title: "Dispatch goal", State: "open", Status: "Ready"},
		StopReasons:     []string{"no_child_tickets"},
		NextAction:      "plan_children",
		NextSafeCommand: "gira goal plan --repo StatPan/backlog --goal 521 --dry-run",
	}

	packet := dispatchPacketFromGoalHandoff(repo, handoff)
	if packet.WorkerHandoff != nil || !containsString(packet.StopReasons, "no_child_tickets") || packet.NextAction != "plan_children" {
		t.Fatalf("unexpected stop packet: %+v", packet)
	}
}

func TestBuildDispatchCompactPacketDropsVerboseWorkerPayload(t *testing.T) {
	repo := RepoRef{Owner: "StatPan", Name: "backlog"}
	selected := GoalNextCandidate{Repo: "StatPan/gira", Number: 573, Title: "Add compact dispatch", URL: "https://github.com/StatPan/gira/issues/573"}
	worker := TicketHandoffReport{
		SchemaVersion: WorkerHandoffSchemaVersion,
		Role:          AgentPromptRoleImplementer,
		Profile:       AgentPromptProfileDefault,
		Repo:          "StatPan/gira",
		Issue:         573,
		Title:         "Add compact dispatch",
		Readiness:     TicketReadinessReport{SchemaVersion: TicketReadinessSchemaVersion, Readiness: "ready"},
		WorkOrder: TicketHandoffWorkOrder{
			Goal:       "Make dispatch compact",
			Scope:      strings.Repeat("scope ", 400),
			Acceptance: []string{"compact JSON omits full ticket body", "prompt stays within budget"},
			TicketBody: "FULL TICKET BODY SHOULD NOT APPEAR",
		},
		RequiredChecks:       []string{"go test ./internal/gira"},
		EvidenceExpectations: []string{"compact output has selected work"},
		BranchPolicy:         TicketHandoffBranchPolicy{Base: "main", WorkBranch: "issue-573-compact-dispatch"},
		LinkedPR:             &TicketHandoffLinkedPR{Available: true, Number: 71, URL: "https://github.com/StatPan/gira/pull/71", Ready: false, Checks: []DevPRCheck{{Conclusion: "success"}, {Status: "queued"}}},
		NextSafeCommand:      "gira ticket start --repo StatPan/gira --ticket 573 --apply",
	}
	handoff := GoalHandoffReport{
		Repo:            repo.FullName(),
		Role:            AgentPromptRoleImplementer,
		Profile:         AgentPromptProfileDefault,
		Goal:            GoalStatusIssue{Number: 521, Title: "Dispatch goal", State: "open", Status: "Ready"},
		GoalContext:     GoalHandoffContext{Objective: "Reduce token waste", Scope: strings.Repeat("goal scope ", 300), StopConditions: []string{"unclear selected ticket"}},
		GoalStatus:      GoalStatusReport{Counts: map[string]int{"ready": 1, "total": 1}, RemainingAutonomousWork: 1},
		SelectedTicket:  &selected,
		WorkerHandoff:   &worker,
		NextAction:      "handoff_child",
		NextSafeCommand: worker.NextSafeCommand,
	}
	packet := dispatchPacketFromGoalHandoff(repo, handoff)

	compact := BuildDispatchCompactPacket(packet, 1200)
	if compact.SchemaVersion != DispatchCompactSchemaVersion || compact.SelectedTicket == nil || compact.SelectedTicket.Readiness != "ready" {
		t.Fatalf("unexpected compact packet: %+v", compact)
	}
	if !compact.Truncated {
		t.Fatalf("expected compact packet to mark truncation: %+v", compact)
	}
	out, err := json.Marshal(compact)
	if err != nil {
		t.Fatalf("marshal compact: %v", err)
	}
	if strings.Contains(string(out), "FULL TICKET BODY SHOULD NOT APPEAR") || strings.Contains(string(out), "role_packet") || strings.Contains(string(out), "ticket_body") {
		t.Fatalf("compact output leaked verbose worker payload:\n%s", string(out))
	}
	if !strings.Contains(string(out), "compact JSON omits full ticket body") || !strings.Contains(string(out), "checks_status") {
		t.Fatalf("compact output missing core context:\n%s", string(out))
	}
}

func TestFormatDispatchPromptHonorsContextBudget(t *testing.T) {
	packet := DispatchPacket{
		Command:       "dispatch goal",
		SchemaVersion: DispatchPacketSchemaVersion,
		Source:        DispatchSource{Kind: "goal", Repo: "StatPan/backlog", Number: 521},
		Role:          AgentPromptRoleImplementer,
		Profile:       AgentPromptProfileDefault,
		Instruction: DispatchInstruction{
			Objective:        strings.Repeat("objective ", 200),
			SelectedWork:     "StatPan/gira#573 Compact dispatch",
			AllowedActions:   []string{"Execute only selected child ticket."},
			StopConditions:   []string{"unclear acceptance"},
			EvidenceRequired: []string{"go test ./..."},
		},
		GoalHandoff: &GoalHandoffReport{
			Repo:            "StatPan/backlog",
			Role:            AgentPromptRoleImplementer,
			Profile:         AgentPromptProfileDefault,
			Goal:            GoalStatusIssue{Number: 521, Title: "Dispatch goal"},
			GoalContext:     GoalHandoffContext{Objective: strings.Repeat("objective ", 200)},
			NextAction:      "handoff_child",
			NextSafeCommand: "gira ticket start --repo StatPan/gira --ticket 573 --apply",
		},
		NextAction:      "handoff_child",
		NextSafeCommand: "gira ticket start --repo StatPan/gira --ticket 573 --apply",
	}

	prompt := FormatDispatchPrompt(packet, 1200)
	if len(prompt) > 1200 {
		t.Fatalf("prompt length = %d, want <= 1200\n%s", len(prompt), prompt)
	}
	for _, want := range []string{"# Gira Dispatch", "Next Safe Command", "truncated:"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}
