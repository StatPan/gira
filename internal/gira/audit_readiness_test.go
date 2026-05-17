package gira

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildAuditReadinessReportWarnsForMissingLedger(t *testing.T) {
	repo := mustParseRepoRef(t, "StatPan/gira")
	report := BuildAuditReadinessReport(repo, filepath.Join(t.TempDir(), "*.jsonl"), readyDoctorRunner(), fixedAuditReadinessTime())

	if !report.Ready {
		t.Fatalf("expected missing ledger warning to remain ready, got %+v", report)
	}
	if report.Audit.Status != AuditReadinessStatusMissing {
		t.Fatalf("audit status = %q, want missing", report.Audit.Status)
	}
	if report.Mode != AuditReadinessModeDailyOperation {
		t.Fatalf("mode = %q, want daily_operation", report.Mode)
	}
	if report.Audit.Verify.Failure != "no_audit_files_found" {
		t.Fatalf("audit failure = %q, want no_audit_files_found", report.Audit.Verify.Failure)
	}
	if !strings.Contains(report.NextStep, "gira status --repo StatPan/gira") {
		t.Fatalf("next step should prefer Gira status, got %q", report.NextStep)
	}
}

func TestBuildAuditReadinessReportWarnsForEmptyLedger(t *testing.T) {
	dir := t.TempDir()
	repo := mustParseRepoRef(t, "StatPan/gira")
	path := filepath.Join(dir, "StatPan_gira.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write empty ledger: %v", err)
	}

	report := BuildAuditReadinessReport(repo, filepath.Join(dir, "*.jsonl"), readyDoctorRunner(), fixedAuditReadinessTime())

	if !report.Ready {
		t.Fatalf("expected empty ledger warning to remain ready, got %+v", report)
	}
	if report.Audit.Status != AuditReadinessStatusMissing {
		t.Fatalf("audit status = %q, want missing", report.Audit.Status)
	}
	if report.Audit.Verify.Failure != "no_audit_records" {
		t.Fatalf("audit failure = %q, want no_audit_records", report.Audit.Verify.Failure)
	}
	if !strings.Contains(report.Audit.Detail, "has no records yet") {
		t.Fatalf("audit detail = %q, want empty-history detail", report.Audit.Detail)
	}
}

func TestBuildAuditReadinessReportFailsForInvalidGlob(t *testing.T) {
	repo := mustParseRepoRef(t, "StatPan/gira")
	report := BuildAuditReadinessReport(repo, "[", readyDoctorRunner(), fixedAuditReadinessTime())

	if report.Ready {
		t.Fatalf("expected invalid glob to fail readiness")
	}
	if report.Audit.Status != AuditReadinessStatusFailed {
		t.Fatalf("audit status = %q, want failed", report.Audit.Status)
	}
	if report.Audit.Verify.Failure != "invalid_audit_glob" {
		t.Fatalf("audit failure = %q, want invalid_audit_glob", report.Audit.Verify.Failure)
	}
}

func TestBuildAuditReadinessReportFailsForInvalidLedger(t *testing.T) {
	dir := t.TempDir()
	repo := mustParseRepoRef(t, "StatPan/gira")
	path := filepath.Join(dir, "StatPan_gira.jsonl")
	if err := os.WriteFile(path, []byte("{not-json}\n"), 0o644); err != nil {
		t.Fatalf("write invalid ledger: %v", err)
	}

	report := BuildAuditReadinessReport(repo, filepath.Join(dir, "*.jsonl"), readyDoctorRunner(), fixedAuditReadinessTime())

	if report.Ready {
		t.Fatalf("expected invalid ledger to fail readiness")
	}
	if report.Audit.Status != AuditReadinessStatusFailed {
		t.Fatalf("audit status = %q, want failed", report.Audit.Status)
	}
	if report.Audit.Verify.Failure != "malformed_json" {
		t.Fatalf("audit failure = %q, want malformed_json", report.Audit.Verify.Failure)
	}
	if !strings.Contains(report.NextStep, "gira audit verify --repo StatPan/gira") {
		t.Fatalf("next step should point to audit verify, got %q", report.NextStep)
	}
}

func TestBuildAuditReadinessReportPassesForHealthyLedger(t *testing.T) {
	dir := t.TempDir()
	repo := mustParseRepoRef(t, "StatPan/gira")
	path := filepath.Join(dir, "StatPan_gira.jsonl")
	if err := AppendAuditRecords(path, []AuditRecord{
		NewAuditRecord("sync", "sha256:abc", "label:create", "issue#1", "ok", "", "allowed", fixedAuditReadinessTime()),
	}); err != nil {
		t.Fatalf("append audit: %v", err)
	}

	report := BuildAuditReadinessReport(repo, filepath.Join(dir, "*.jsonl"), readyDoctorRunner(), fixedAuditReadinessTime())

	if !report.Ready {
		t.Fatalf("expected healthy ledger readiness, got %+v", report)
	}
	if report.Audit.Status != AuditReadinessStatusOK {
		t.Fatalf("audit status = %q, want ok", report.Audit.Status)
	}
	if report.Audit.Verify.Records != 1 {
		t.Fatalf("records = %d, want 1", report.Audit.Verify.Records)
	}
	if report.NextStep != "gira status --repo StatPan/gira" {
		t.Fatalf("next step = %q, want gira status", report.NextStep)
	}
}

func TestFormatAuditReadinessReportSeparatesDoctorAndAuditSections(t *testing.T) {
	repo := mustParseRepoRef(t, "StatPan/gira")
	report := BuildAuditReadinessReport(repo, filepath.Join(t.TempDir(), "*.jsonl"), readyDoctorRunner(), fixedAuditReadinessTime())
	out := FormatAuditReadinessReport(report)

	for _, want := range []string{"mode: daily_operation", "readiness/doctor checks:", "audit ledger health:", "[warn] audit_ledger"} {
		if !strings.Contains(out, want) {
			t.Fatalf("formatted report missing %q:\n%s", want, out)
		}
	}
}

func TestBuildAuditReadinessReportNoOpenWorkModeIsReady(t *testing.T) {
	report := BuildAuditReadinessReport(
		ParseRepoRefMust("StatPan/gira"),
		filepath.Join(t.TempDir(), "*.jsonl"),
		noOpenWorkDoctorRunner(),
		fixedAuditReadinessTime(),
	)

	if !report.Ready || report.Audit.Status != AuditReadinessStatusMissing {
		t.Fatalf("report = %+v, want no-open-work readiness warning", report)
	}
	if report.Mode != AuditReadinessModeNoOpenWork || !report.Doctor.Ready {
		t.Fatalf("mode=%q doctor.ready=%t, want no_open_work ready", report.Mode, report.Doctor.Ready)
	}
	check := doctorCheckByID(report.Doctor, "onboard_readiness")
	if check == nil || check.Status != DoctorCheckWarn || check.Detail != "open issues=0" {
		t.Fatalf("onboard_readiness check = %+v, want no-open-work warning", check)
	}
	if !strings.Contains(report.NextStep, "no open work") || strings.Contains(report.NextStep, "gira ticket new") {
		t.Fatalf("next step = %q, want completion/idle guidance without hard start prompt", report.NextStep)
	}
	text := FormatAuditReadinessReport(report)
	for _, want := range []string{"audit readiness: READY", "mode: no_open_work", "[warn] onboard_readiness: open issues=0", "[warn] audit_ledger"} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted readiness missing %q:\n%s", want, text)
		}
	}
}

func TestBuildAuditReadinessReportInvalidGlobGuidesVerify(t *testing.T) {
	report := BuildAuditReadinessReport(
		ParseRepoRefMust("StatPan/gira"),
		"[",
		readyDoctorRunner(),
		fixedAuditReadinessTime(),
	)

	if report.Ready || report.Audit.Status != AuditReadinessStatusFailed {
		t.Fatalf("report = %+v, want failed audit readiness", report)
	}
	if !strings.Contains(report.NextStep, "gira audit verify --repo StatPan/gira --path [") {
		t.Fatalf("next step = %q, want audit verify remediation", report.NextStep)
	}
	if got := auditReadinessHumanStatus(AuditReadinessStatusOK); got != "pass" {
		t.Fatalf("ok human status = %q, want pass", got)
	}
}

func TestAuditReadinessNextStepUsesFirstDoctorFailureRemediation(t *testing.T) {
	report := AuditReadinessReport{
		Repo:  "StatPan/gira",
		Mode:  AuditReadinessModeDailyOperation,
		Ready: false,
		Doctor: DoctorReport{
			Ready: false,
			Checks: []DoctorCheck{{
				ID:          "gh_auth",
				Status:      DoctorCheckFail,
				Detail:      "not logged in",
				Remediation: "run `gh auth login`",
			}},
		},
		Audit: AuditReadinessHealth{Status: AuditReadinessStatusOK},
	}

	got := auditReadinessNextStep(report, ".gira/audit/*.jsonl")
	want := "fix gh_auth: run `gh auth login`; then run `gira audit readiness --repo StatPan/gira --path .gira/audit/*.jsonl`"
	if got != want {
		t.Fatalf("next step = %q, want %q", got, want)
	}
}

func fixedAuditReadinessTime() time.Time {
	return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
}

func noOpenWorkDoctorRunner() onboardFakeRunner {
	runner := readyDoctorRunner()
	runner.responses["gh api repos/StatPan/gira/issues --paginate --slurp -X GET -f state=all -f per_page=100"] = `[]`
	return runner
}
