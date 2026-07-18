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

func TestRunPMCompileCompactDefault(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runPM([]string{"compile", "--intent", "# Actor\nPM\n# Problem\nIntent is lost.\n# Desired Outcome\nIntent is retained.\n# Evidence\n- issue #859\n# Success Conditions\n- IR is deterministic."}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{"pm compile: errors=0", "ir: pm-ir/v1", "detail: gira pm compile"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("output missing %q:\n%s", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "Intent is retained.") {
		t.Fatalf("compact output leaked source prose:\n%s", stdout.String())
	}
}

func TestRunPMCompileFromFileJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "request.md")
	if err := os.WriteFile(path, []byte("# Actor\nPM\n# Problem\nIntent is lost.\n# Desired Outcome\nIntent is retained.\n# Evidence\n- issue #859\n# Success Conditions\n- IR is deterministic."), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runPM([]string{"compile", "--from-file", path, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	var report gira.PMCompileReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.PMCompileReportSchemaVersion || report.IR.Actor.Value != "PM" || !report.ReadOnly {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func TestRunPMCompileRejectsGoalWithoutRepo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runPM([]string{"compile", "--intent", "intent", "--goal", "857"}, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "--goal requires --repo") {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func TestRunPMCompileAllowsGoalAsOnlyIntentSource(t *testing.T) {
	restore := newPMCompileReport
	t.Cleanup(func() { newPMCompileReport = restore })
	newPMCompileReport = func(input gira.PMCompileRequest) (gira.PMCompileReport, error) {
		if input.RawIntent != "" || input.Repo != "StatPan/gira" || input.Goal != 857 {
			t.Fatalf("unexpected compile request: %#v", input)
		}
		return gira.PMCompileReport{Command: "pm compile", SchemaVersion: gira.PMCompileReportSchemaVersion, ReadOnly: true, IR: gira.PMIR{SchemaVersion: gira.PMIRSchemaVersion}}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runPM([]string{"compile", "--repo", "StatPan/gira", "--goal", "857", "--json"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), `"pm-compile-report/v1"`) {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}
