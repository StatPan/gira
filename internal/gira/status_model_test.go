package gira

import (
	"reflect"
	"testing"
)

func TestDeriveStatusCompatibility(t *testing.T) {
	tests := []struct {
		labels       []string
		raw, managed string
	}{
		{nil, "open", ""},
		{[]string{"type:task"}, "open", ""},
		{[]string{"status:backlog"}, "backlog", "Backlog"},
		{[]string{"status:ready"}, "ready", "Ready"},
		{[]string{"status:in-progress"}, "in-progress", "In progress"},
		{[]string{"status:in-review"}, "in-review", "In review"},
		{[]string{"status:blocked"}, "blocked", "Blocked"},
		{[]string{"status:done"}, "done", "Done"},
		{[]string{"status:needs-review"}, "needs-review", ""},
		{[]string{"status:in_progress"}, "in_progress", "In progress"},
		{[]string{"STATUS:READY"}, "open", "Ready"},
		{[]string{"status:ready", "status:done", "status:blocked"}, "done", "Done"},
		{[]string{"status:done", "status:ready"}, "done", "Done"},
		{[]string{"status:blocked", "status:in-progress", "status:in-review"}, "in-review", "In review"},
		{[]string{"status:needs-review", "status:ready"}, "ready", "Ready"},
		{[]string{"status:ready", "STATUS:DONE"}, "done", "Done"},
	}
	for _, tt := range tests {
		got := deriveStatus(tt.labels)
		if got.raw != tt.raw || got.managed != tt.managed {
			t.Errorf("deriveStatus(%q) = %#v, want raw %q managed %q", tt.labels, got, tt.raw, tt.managed)
		}
	}
}

func TestSyncStatusLabelsUnchanged(t *testing.T) {
	var got []LabelDef
	for _, label := range DesiredLabels {
		if len(label.Name) >= len("status:") && label.Name[:len("status:")] == "status:" {
			got = append(got, label)
		}
	}
	want := []LabelDef{
		{Name: "status:ready", Color: "C2E0C6", Description: "Ready to start."},
		{Name: "status:in-progress", Color: "1D76DB", Description: "Work has started on a branch or active implementation."},
		{Name: "status:in-review", Color: "7057FF", Description: "A linked PR or review surface is active."},
		{Name: "status:blocked", Color: "E99695", Description: "Blocked by an external dependency or decision."},
		{Name: "status:done", Color: "0E8A16", Description: "Work is completed with merged, closed, or accepted completion evidence."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sync statuses = %#v, want %#v", got, want)
	}
}
