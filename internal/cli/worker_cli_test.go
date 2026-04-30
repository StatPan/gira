package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestWorkerClaimConflictFailsClosed(t *testing.T) {
	var out1, err1 bytes.Buffer
	code := Run([]string{"worker", "claim", "--repo", "StatPan/gira", "--issue", "72", "--worker", "alice", "--lease-minutes", "60"}, &out1, &err1)
	if code != 0 {
		t.Fatalf("first claim code=%d stderr=%s", code, err1.String())
	}

	var out2, err2 bytes.Buffer
	code = Run([]string{"worker", "claim", "--repo", "StatPan/gira", "--issue", "72", "--worker", "bob", "--lease-minutes", "60"}, &out2, &err2)
	if code == 0 {
		t.Fatalf("expected conflict; stdout=%s", out2.String())
	}
	if !strings.Contains(err2.String(), "claim_conflict") {
		t.Fatalf("missing conflict error: %s", err2.String())
	}

	var out3, err3 bytes.Buffer
	code = Run([]string{"worker", "release", "--repo", "StatPan/gira", "--issue", "72", "--worker", "alice"}, &out3, &err3)
	if code != 0 {
		t.Fatalf("release code=%d stderr=%s", code, err3.String())
	}
}

func TestWorkerHandoffRequiresPayloadFields(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"worker", "handoff", "--repo", "StatPan/gira", "--issue", "72"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero for invalid payload")
	}
	if !strings.Contains(stderr.String(), "missing_goal") {
		t.Fatalf("expected missing_goal error, got: %s", stderr.String())
	}
}
