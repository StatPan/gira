package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StatPan/gira/internal/gira"
)

func TestPMAcceptanceCLIReadsVersionedResultAndRequiresMode(t *testing.T) {
	originalAccept, originalRepo := newPMAcceptanceReport, repoContextRunner
	t.Cleanup(func() { newPMAcceptanceReport, repoContextRunner = originalAccept, originalRepo })
	repoContextRunner = pmObserveCLIRunner{}
	newPMAcceptanceReport = func(input gira.PMAcceptanceInput) (gira.PMAcceptanceReport, error) {
		return gira.PMAcceptanceReport{Command: "pm accept", SchemaVersion: gira.PMAcceptanceReportSchemaVersion, Repo: input.Repo.FullName(), Ticket: input.Ticket, DryRun: input.DryRun, Result: input.Result}, nil
	}
	path := filepath.Join(t.TempDir(), "acceptance.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":"pm-acceptance-result/v1","pull_request":99,"delivery_state":"accepted","outcome_state":"not_evaluated"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runPM([]string{"accept", "--repo", "StatPan/gira", "--ticket", "42", "--from-file", path, "--json"}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "exactly one") {
		t.Fatalf("missing mode code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runPM([]string{"accept", "--repo", "StatPan/gira", "--ticket", "42", "--from-file", path, "--dry-run", "--json"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), `"schema_version": "pm-acceptance-report/v1"`) {
		t.Fatalf("dry run code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}
