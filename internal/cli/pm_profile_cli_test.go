package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/StatPan/gira/internal/gira"
)

func TestRunPMSpecDefaultsToDeliveryProfileJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runPM([]string{"spec", "--repo", "OWNER/repo", "--intent", "Deliver a bounded result.", "--context-ref", "issue:OWNER/repo#100", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	var report gira.PMTaskSpecReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != gira.PMTaskPacketV2SchemaVersion || report.Profile != "delivery" || report.SuggestedWorkerMode != "implement" || len(report.ContextRefs) != 1 {
		t.Fatalf("unexpected default profile report: %#v", report)
	}
}

func TestRunPMSpecLegacyCompatibility(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runPM([]string{"spec", "--profile", "legacy", "--intent", "Keep legacy rendering."}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "schema=gira-pm-task-packet/v1") || strings.Contains(stdout.String(), "gira:task-profile/v1") {
		t.Fatalf("code=%d stderr=%s output=%s", code, stderr.String(), stdout.String())
	}
}
