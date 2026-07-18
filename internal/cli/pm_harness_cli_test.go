package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StatPan/gira/internal/gira"
)

func TestRunPMBootstrapPassesRoleAuthorityAndBudget(t *testing.T) {
	restore := newPMBootstrapReport
	t.Cleanup(func() { newPMBootstrapReport = restore })
	newPMBootstrapReport = func(input gira.PMBootstrapInput) (gira.PMBootstrapReport, error) {
		if input.Repo.FullName() != "StatPan/gira" || input.Ticket != 857 || input.Role != "ai" || input.Budget != 5000 || strings.Join(input.Authority, ",") != "issue:read,report:read" {
			t.Fatalf("unexpected bootstrap input: %#v", input)
		}
		return gira.PMBootstrapReport{Command: "pm bootstrap", SchemaVersion: gira.PMBootstrapSchemaVersion, PolicyVersion: gira.PMHarnessPolicyVersion, ProtocolVersion: gira.PMHarnessProtocolVersion, ReadOnly: true, Repo: input.Repo.FullName(), Ticket: input.Ticket, Role: input.Role, SessionID: "pms-test"}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runPM([]string{"bootstrap", "--repo", "StatPan/gira", "--ticket", "857", "--role", "ai", "--authority", "issue:read", "--authority", "report:read", "--context-budget", "5000", "--json"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), `"session_id": "pms-test"`) {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

func TestRunPMConformanceBuiltinAndUnsafeInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runPM([]string{"conformance", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("builtin code=%d stderr=%s", code, stderr.String())
	}
	var report gira.PMConformanceReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil || !report.ProtocolCompliant || report.Summary.AIConfigurations != 2 {
		t.Fatalf("unexpected builtin report: err=%v report=%#v", err, report)
	}

	unsafe := gira.BuiltinPMConformanceRuns()[1]
	unsafe.FailureAttempts = []gira.PMConformanceFailureAttempt{{Mode: "authority_overreach", Blocked: false}}
	raw, _ := json.Marshal(unsafe)
	path := filepath.Join(t.TempDir(), "unsafe.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runPM([]string{"conformance", "--from-file", path, "--json"}, &stdout, &stderr); code != 1 || !strings.Contains(stdout.String(), `"unsafe_mutations": 1`) {
		t.Fatalf("unsafe code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}
