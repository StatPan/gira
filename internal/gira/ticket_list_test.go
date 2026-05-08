package gira

import (
	"fmt"
	"strings"
	"testing"
)

func TestBuildTicketListReportConstructsGHCommandAndNormalizesRows(t *testing.T) {
	repo := ParseRepoRefMust("StatPan/gira")
	runner := &ticketListRunner{outputs: map[string][]byte{
		"gh issue list --repo StatPan/gira --state all --limit 25 --json number,title,state,labels,assignees,milestone,url --label status:ready --label priority:p1 --assignee alice --milestone MVP": []byte(`[
			{"number":12,"title":"Ready work","state":"OPEN","labels":[{"name":"priority:p1"},{"name":"status:ready"},{"name":"type:story"}],"assignees":[{"login":"bob"},{"login":"alice"}],"milestone":{"title":"MVP"},"url":"https://github.com/StatPan/gira/issues/12"},
			{"number":9,"title":"Closed bug","state":"CLOSED","labels":[{"name":"area:backend"},{"name":"status:done"},{"name":"unmanaged"}],"assignees":[],"milestone":null,"url":"https://github.com/StatPan/gira/issues/9"}
		]`),
	}}

	report, err := BuildTicketListReport(TicketListOptions{
		Repo:      repo,
		State:     "all",
		Labels:    []string{"status:ready,priority:p1", "status:ready"},
		Assignee:  "alice",
		Milestone: "MVP",
		Limit:     25,
	}, runner)
	if err != nil {
		t.Fatalf("BuildTicketListReport error: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("calls = %v, want 1 call", runner.calls)
	}
	if report.Filters.State != "all" || strings.Join(report.Filters.Labels, ",") != "status:ready,priority:p1" || report.Counts.Tickets != 2 {
		t.Fatalf("unexpected report filters/counts: %+v", report)
	}
	if report.Tickets[0].Number != 12 || report.Tickets[0].State != "open" || strings.Join(report.Tickets[0].Labels, ",") != "priority:p1,status:ready,type:story" {
		t.Fatalf("tickets should preserve gh order and normalize labels: %+v", report.Tickets)
	}
	if strings.Join(report.Tickets[0].Assignees, ",") != "alice,bob" || report.Tickets[0].Status != "ready" {
		t.Fatalf("assignees/status not normalized: %+v", report.Tickets[0])
	}
}

func TestBuildTicketListReportValidatesInputs(t *testing.T) {
	repo := ParseRepoRefMust("StatPan/gira")
	if _, err := BuildTicketListReport(TicketListOptions{Repo: repo, State: "pending"}, &ticketListRunner{}); err == nil || !strings.Contains(err.Error(), "--state") {
		t.Fatalf("expected state validation error, got %v", err)
	}
	if _, err := BuildTicketListReport(TicketListOptions{Repo: repo, Limit: -1}, &ticketListRunner{}); err == nil || !strings.Contains(err.Error(), "--limit") {
		t.Fatalf("expected limit validation error, got %v", err)
	}
}

func TestFormatTicketListCompact(t *testing.T) {
	out := FormatTicketList(TicketListReport{
		Repo:    "StatPan/gira",
		Filters: TicketListFilters{State: "open"},
		Counts:  TicketListCounts{Tickets: 1},
		Tickets: []TicketListItem{{
			Number:    12,
			State:     "open",
			Title:     "Ready work",
			Labels:    []string{"status:ready", "type:story"},
			Assignees: []string{"alice"},
			Milestone: "MVP",
		}},
	})
	for _, want := range []string{"ticket list: StatPan/gira state=open count=1", "#12 open", "labels=status:ready,type:story", "assignees=alice", "milestone=MVP"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestFormatTicketListUsesCommandForEmptyEpicList(t *testing.T) {
	out := FormatTicketList(TicketListReport{
		Command: "epic list",
		Repo:    "StatPan/gira",
		Filters: TicketListFilters{State: "open"},
		Counts:  TicketListCounts{Tickets: 0},
	})
	for _, want := range []string{"epic list: StatPan/gira state=open count=0", "epics: none"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "tickets: none") {
		t.Fatalf("epic list should not use ticket empty noun:\n%s", out)
	}
}

type ticketListRunner struct {
	outputs map[string][]byte
	calls   []string
}

func (r *ticketListRunner) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, key)
	if out, ok := r.outputs[key]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected call: %s", key)
}
